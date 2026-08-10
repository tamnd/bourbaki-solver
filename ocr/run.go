package ocr

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/queue"
	"github.com/tamnd/bourbaki-solver/textguard"
)

// The run loop is the part that spends money, so it is written around the two
// facts that cost the most. A page is minutes, so nothing is done twice that
// does not have to be: the queue is content addressed and a page already read
// and accepted is never sent again. And the fleet is three boxes of very
// different size, so the hosts run independently rather than in lockstep;
// server3 takes four pages at a time and server1 takes none, and a barrier
// between them would leave the big box idle waiting for the small one.

// DefaultBatch is how many pages go to a host in one batch.
//
// Twenty five, for two reasons that pull in opposite directions. A batch is one
// rsync out, one start, one rsync back and a few polls, so a bigger batch
// amortises those better. But a batch is also the unit of loss: a host that
// falls over mid batch has its remaining pages re-queued, and a lease is held
// for the whole batch. Twenty five pages on four lanes is around twenty
// minutes, which is short enough to lose and long enough to be worth starting.
const DefaultBatch = 25

// Source is one page image a run can read, as the render manifest describes it.
// The ocr package takes this rather than the manifest itself, so that reading
// pages does not depend on the package that rasterises them.
type Source struct {
	Page   int
	SHA256 string
	Blank  bool
}

// Runner reads a volume through the fleet.
type Runner struct {
	Book string
	// Root is the corpus checkout. Images are read from under it and page files
	// are written under it, and neither is committed.
	Root  string
	Queue *queue.Queue
	// Prompt is the OCR instruction, and its hash is part of every job id, so a
	// changed prompt is new work rather than a silent reuse of old answers.
	Prompt string
	Model  string
	Hosts  []Host
	Shell  Shell
	Copy   Copier
	// Batch is pages per batch. Zero means DefaultBatch.
	Batch int
	// Expect says what is already known about a page, which is what rules 4 and
	// 6 compare the answer against.
	Expect func(page int) Expect
	// Rerender puts a fresh image on disk at a higher resolution before a retry.
	// It is a function rather than a dependency on the render package, because
	// this package has no business knowing about pdftoppm.
	Rerender func(ctx context.Context, page, dpi int) error
	// RetryDPI is what Rerender is asked for on the second attempt and after.
	RetryDPI int
	Options  Options
	// Keep leaves the page images on the hosts.
	Keep bool
	// Limit stops the run after this many pages have been read, for a pilot. Zero
	// is the whole volume.
	Limit int
	Logf  func(string, ...any)
	Sleep func(ctx context.Context, d time.Duration) error
	Now   func() time.Time
}

func (r *Runner) logf(format string, args ...any) {
	if r.Logf != nil {
		r.Logf(format, args...)
	}
}

func (r *Runner) now() time.Time {
	if r.Now != nil {
		return r.Now().UTC()
	}
	return time.Now().UTC()
}

func (r *Runner) batch() int {
	if r.Batch > 0 {
		return r.Batch
	}
	return DefaultBatch
}

// ImagePath is where a page image sits. It matches what render writes, and it
// is spelled out here rather than imported so the two packages stay apart.
func ImagePath(root, book string, page int) string {
	return filepath.Join(root, "images", book, fmt.Sprintf("%04d.png", page))
}

// RawDir is where the model's answers are kept before they are accepted.
//
// They are kept rather than dropped. An accepted page is rewritten with front
// matter and a normalised body, so the file in the corpus is no longer what the
// model said, and when a page turns out to be wrong three milestones later the
// only way to tell a bad reading from a bad repair is the original answer.
func RawDir(root, book string) string {
	return filepath.Join(root, "work", "pages-raw", book)
}

// Target names a page in the queue.
func Target(book string, page int) string { return fmt.Sprintf("%s/%04d", book, page) }

// Fill adds a job for every page that still has to be read, and returns how
// many it added.
//
// Blank pages are skipped, which is what the ink measurement was for. Pages
// already accepted at this prompt are skipped too, because the job id is a hash
// of the image and the prompt and the queue refuses to add a job it has already
// done. A page accepted under an older prompt gets a new id and is read again,
// which is the intended cost of changing the prompt.
func (r *Runner) Fill(sources []Source) (int, error) {
	promptSHA := sha256Hex(r.Prompt)
	var added int
	for _, source := range sources {
		if source.Blank {
			continue
		}
		if r.accepted(source, promptSHA) {
			continue
		}
		job := queue.New(queue.StageOCR, Target(r.Book, source.Page), source.SHA256, promptSHA)
		ok, err := r.Queue.Add(job)
		if err != nil {
			return added, err
		}
		if ok {
			added++
		}
	}
	return added, nil
}

// accepted says whether a page is already on disk, read from this image with
// this prompt.
func (r *Runner) accepted(source Source, promptSHA string) bool {
	file, err := corpus.ReadFile[corpus.PageFrontMatter](corpus.PagePath(r.Root, r.Book, source.Page))
	if err != nil {
		return false
	}
	if file.Meta.Method != corpus.MethodOCR {
		return false
	}
	// The image hash is compared as well as the prompt. A page re-rendered at
	// 600 dpi is a different image and deserves a fresh reading, and comparing
	// only the prompt would keep the old answer.
	return file.Meta.PromptSHA256 == promptSHA && file.Meta.InputSHA256 == source.SHA256
}

// Failure is a page that did not come back clean, kept so the audit can name it
// rather than report a percentage.
type Failure struct {
	Page     int         `json:"page"`
	Attempts int         `json:"attempts"`
	State    queue.State `json:"state"`
	Reason   string      `json:"reason"`
}

// Report is what a run did.
type Report struct {
	Book      string              `json:"book"`
	Started   time.Time           `json:"started"`
	Finished  time.Time           `json:"finished"`
	Batches   []Result            `json:"batches"`
	Accepted  int                 `json:"accepted"`
	Rejected  int                 `json:"rejected"`
	Dead      int                 `json:"dead"`
	Rules     map[Rule]int        `json:"rules,omitempty"`
	Failures  []Failure           `json:"failures,omitempty"`
	Reaped    int                 `json:"reaped,omitempty"`
	PerHost   map[string]int      `json:"per_host,omitempty"`
	HostTimes map[string]Duration `json:"host_times,omitempty"`
}

// Rate is the share of pages read that were accepted, which is the number M3 is
// judged on.
func (r Report) Rate() float64 {
	total := r.Accepted + r.Rejected
	if total == 0 {
		return 0
	}
	return float64(r.Accepted) * 100 / float64(total)
}

// Summary is what a run prints when it stops.
func (r Report) Summary() string {
	var out strings.Builder
	elapsed := r.Finished.Sub(r.Started).Round(time.Second)
	fmt.Fprintf(&out, "%s: %d accepted, %d rejected, %d dead in %s, %.1f %% accepted\n",
		r.Book, r.Accepted, r.Rejected, r.Dead, elapsed, r.Rate())
	hosts := make([]string, 0, len(r.PerHost))
	for host := range r.PerHost {
		hosts = append(hosts, host)
	}
	sort.Strings(hosts)
	for _, host := range hosts {
		pages := r.PerHost[host]
		line := fmt.Sprintf("  %-8s %4d pages", host, pages)
		if spent := time.Duration(r.HostTimes[host]); spent > 0 && pages > 0 {
			line += fmt.Sprintf(" in %s, %s a page, %.0f pages an hour",
				spent.Round(time.Second), (spent / time.Duration(pages)).Round(time.Second),
				float64(pages)/spent.Hours())
		}
		out.WriteString(line + "\n")
	}
	for _, rule := range sortedRules(r.Rules) {
		fmt.Fprintf(&out, "  %-10s %4d\n", rule, r.Rules[rule])
	}
	return out.String()
}

func sortedRules(counts map[Rule]int) []Rule {
	out := make([]Rule, 0, len(counts))
	for rule := range counts {
		out = append(out, rule)
	}
	sort.Slice(out, func(i, j int) bool {
		if counts[out[i]] != counts[out[j]] {
			return counts[out[i]] > counts[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}

// Do runs the volume through every host until the queue is empty.
//
// One goroutine per host, each leasing its own batches. There is no scheduler
// and no work stealing: the queue is the scheduler, a host that is fast simply
// comes back for another batch sooner, and a host that dies holds nothing
// because its leases expire.
func (r *Runner) Do(ctx context.Context) (Report, error) {
	if err := r.check(); err != nil {
		return Report{}, err
	}
	report := Report{Book: r.Book, Started: r.now(), Rules: map[Rule]int{},
		PerHost: map[string]int{}, HostTimes: map[string]Duration{}}

	// Anything a dead worker left behind comes back before this run starts, or
	// the pages it was holding sit in leased until the lease expires on its own
	// and this run reports them as done when they were never read.
	reaped, err := r.Queue.Reap(queue.StageOCR)
	if err != nil {
		return report, err
	}
	report.Reaped = len(reaped)
	if len(reaped) > 0 {
		r.logf("%d jobs came back from a worker that did not finish", len(reaped))
	}

	var lock sync.Mutex
	var group sync.WaitGroup
	var read int
	for _, host := range r.Hosts {
		// The field, not lanes(): that accessor floors at one so the arithmetic
		// below it never divides by zero, and reading it here would send a
		// batch to the box that cannot run a browser.
		if host.Lanes <= 0 {
			r.logf("%s takes no OCR, skipping it", host.Name)
			continue
		}
		group.Add(1)
		go func(host Host) {
			defer group.Done()
			for {
				if ctx.Err() != nil {
					return
				}
				lock.Lock()
				stop := r.Limit > 0 && read >= r.Limit
				remaining := r.batch()
				if r.Limit > 0 {
					remaining = min(remaining, r.Limit-read)
				}
				lock.Unlock()
				if stop {
					return
				}

				tasks, err := r.lease(host, remaining)
				if err != nil {
					r.logf("%s: %v", host.Name, err)
					return
				}
				if len(tasks) == 0 {
					return
				}
				lock.Lock()
				read += len(tasks)
				lock.Unlock()

				result, outcome := r.one(ctx, host, tasks)
				lock.Lock()
				report.Batches = append(report.Batches, result)
				report.Accepted += outcome.accepted
				report.Rejected += outcome.rejected
				report.Dead += outcome.dead
				report.Failures = append(report.Failures, outcome.failures...)
				for rule, count := range outcome.rules {
					report.Rules[rule] += count
				}
				report.PerHost[host.Name] += outcome.accepted
				report.HostTimes[host.Name] += result.Elapsed
				lock.Unlock()
				r.logf("%s", result.Summary())
			}
		}(host)
	}
	group.Wait()

	report.Finished = r.now()
	sort.Slice(report.Failures, func(i, j int) bool { return report.Failures[i].Page < report.Failures[j].Page })
	sort.Slice(report.Batches, func(i, j int) bool { return report.Batches[i].ID < report.Batches[j].ID })
	return report, ctx.Err()
}

func (r *Runner) check() error {
	if strings.TrimSpace(r.Book) == "" {
		return fmt.Errorf("no book")
	}
	if r.Queue == nil {
		return fmt.Errorf("no queue")
	}
	if strings.TrimSpace(r.Prompt) == "" {
		return fmt.Errorf("no prompt")
	}
	if len(r.Hosts) == 0 {
		return fmt.Errorf("no hosts")
	}
	if r.Shell == nil || r.Copy == nil {
		return fmt.Errorf("no transport")
	}
	return nil
}

// task is one page in flight.
type task struct {
	job   queue.Job
	page  int
	image string
	// sha is the hash of the image as it was actually sent, which is not the
	// job's input hash once a retry has re-rendered the page at 600 dpi.
	sha string
	dpi int
}

// lease claims up to n pages for a host.
func (r *Runner) lease(host Host, n int) ([]task, error) {
	if n <= 0 {
		return nil, nil
	}
	// The lease has to outlast the whole batch, because the batch is what one
	// worker is doing and a lease that expires halfway hands the same pages to
	// another host while this one is still reading them.
	expected := time.Duration(n) * host.pageTimeout() / time.Duration(host.lanes())
	var out []task
	for len(out) < n {
		job, err := r.Queue.Lease(queue.StageOCR, host.Name, expected)
		if errors.Is(err, queue.ErrEmpty) {
			break
		}
		if err != nil {
			return out, err
		}
		page, err := pageOf(job.Target)
		if err != nil {
			if _, failErr := r.Queue.Fail(job, err.Error()); failErr != nil {
				return out, failErr
			}
			continue
		}
		out = append(out, task{job: job, page: page, image: ImagePath(r.Root, r.Book, page)})
	}
	return out, nil
}

func pageOf(target string) (int, error) {
	_, digits, ok := strings.Cut(target, "/")
	if !ok {
		return 0, fmt.Errorf("target %q is not book/page", target)
	}
	var page int
	if _, err := fmt.Sscanf(digits, "%d", &page); err != nil || page <= 0 {
		return 0, fmt.Errorf("target %q has no page number in it", target)
	}
	return page, nil
}

// outcome is what one batch did to the queue.
type outcome struct {
	accepted, rejected, dead int
	rules                    map[Rule]int
	failures                 []Failure
}

// one runs a single batch and files everything it produced.
func (r *Runner) one(ctx context.Context, host Host, tasks []task) (Result, outcome) {
	out := outcome{rules: map[Rule]int{}}

	// Escalate before the images move. The dpi follows from the attempt count
	// rather than from anything written in the job, so there is no second piece
	// of state to keep in step with the queue.
	for i := range tasks {
		tasks[i].dpi = 0
		if tasks[i].job.Attempts > 1 && r.Rerender != nil && r.RetryDPI > 0 {
			if err := r.Rerender(ctx, tasks[i].page, r.RetryDPI); err != nil {
				r.logf("page %d: could not re-render at %d dpi: %v", tasks[i].page, r.RetryDPI, err)
			} else {
				tasks[i].dpi = r.RetryDPI
				r.logf("page %d: attempt %d, re-rendered at %d dpi", tasks[i].page, tasks[i].job.Attempts, r.RetryDPI)
			}
		}
		if sum, err := fileSHA256(tasks[i].image); err == nil {
			tasks[i].sha = sum
		}
	}

	id := batchID(r.Book, tasks)
	work := Batch{
		Host: host, ID: id, Prompt: r.Prompt,
		Dest:  filepath.Join(RawDir(r.Root, r.Book), id),
		Shell: r.Shell, Copy: r.Copy, Keep: r.Keep, Logf: r.Logf, Sleep: r.Sleep,
	}
	for _, value := range tasks {
		work.Images = append(work.Images, value.image)
	}

	result, err := work.Run(ctx)
	if err != nil {
		r.logf("%s: batch %s: %v", host.Name, id, err)
	}

	for _, value := range tasks {
		r.file(work.Dest, value, &out)
	}
	return result, out
}

// batchID names a batch on the host.
//
// The attempt counts go into the hash on purpose. ocr-batch is run with
// --skip-existing, so a retry that landed in the same output directory would
// find the rejected answer already there and skip the page it was sent to read
// again. Hashing the attempts gives every retry its own directory.
func batchID(book string, tasks []task) string {
	hash := sha256.New()
	for _, value := range tasks {
		fmt.Fprintf(hash, "%s\x00%d\x00%d\n", value.job.ID, value.job.Attempts, value.dpi)
	}
	first := 0
	if len(tasks) > 0 {
		first = tasks[0].page
	}
	return fmt.Sprintf("%s-%04d-%s", book, first, hex.EncodeToString(hash.Sum(nil))[:6])
}

// toolHeader is the block ocr-batch writes above every answer: the source path,
// the model slug it happened to draw from the pool, the time and the elapsed
// seconds, fenced in the same three dashes a Markdown front matter uses.
var toolHeader = regexp.MustCompile(`(?s)\A---\r?\n.*?\r?\n---\r?\n`)

// StripToolHeader removes that block.
//
// It has to go for two reasons. It is not part of the page, so leaving it in
// makes the body of every file open with four lines of machinery that no reader
// of Bourbaki wants and every later stage has to skip. And its source line
// carries the absolute path on the rented box, /root/bourbaki-ocr/in/..., which
// is somebody's home directory and has no business in a public corpus.
//
// It also made the length rule useless. A refusal of a hundred and thirty
// characters came to two hundred and fifty-seven with the header on top, which
// is over the minimum, so the pipeline wrote "I don't see an image attached" to
// the corpus as though it were page 42 of Algebra I.
func StripToolHeader(text string) string {
	trimmed := strings.TrimLeft(text, " \t\r\n")
	// Only the tool's own header, which has these four keys. A page that really
	// begins with a horizontal rule keeps it.
	match := toolHeader.FindString(trimmed)
	if match == "" || !strings.Contains(match, "\nsource:") || !strings.Contains(match, "\nelapsed:") {
		return text
	}
	return strings.TrimLeft(trimmed[len(match):], " \t\r\n")
}

// file decides what happened to one page and tells the queue.
func (r *Runner) file(dest string, value task, out *outcome) {
	raw, err := os.ReadFile(filepath.Join(dest, OutputName(filepath.Base(value.image))))
	if err != nil {
		r.reject(value, out, nil, "no answer came back for this page")
		return
	}
	text := textguard.Normalise(textguard.Strip(StripToolHeader(string(raw))))
	expect := Expect{Book: r.Book, PDFPage: value.page}
	if r.Expect != nil {
		expect = r.Expect(value.page)
	}
	if problems := Validate(text, expect, r.Options); len(problems) > 0 {
		r.reject(value, out, Rules(problems), Reasons(problems))
		return
	}
	if err := r.write(value, text); err != nil {
		r.reject(value, out, nil, "could not write the page: "+err.Error())
		return
	}
	if _, err := r.Queue.Finish(value.job, true, ""); err != nil {
		r.logf("page %d: could not mark the job done: %v", value.page, err)
	}
	out.accepted++
}

func (r *Runner) reject(value task, out *outcome, rules []Rule, reason string) {
	state, err := r.Queue.Fail(value.job, reason)
	if err != nil {
		r.logf("page %d: could not mark the job failed: %v", value.page, err)
	}
	out.rejected++
	for _, rule := range rules {
		out.rules[rule]++
	}
	if state == queue.Dead {
		out.dead++
	}
	out.failures = append(out.failures, Failure{
		Page: value.page, Attempts: value.job.Attempts, State: state, Reason: reason,
	})
	r.logf("page %d rejected on attempt %d: %s", value.page, value.job.Attempts, reason)
}

// write puts an accepted page in the corpus.
func (r *Runner) write(value task, text string) error {
	head, body := SplitHead(text)
	meta := corpus.PageFrontMatter{
		Book: r.Book, PDFPage: value.page, Method: corpus.MethodOCR, Model: r.Model,
		PageLabel: head.Label, RunningHead: head.Title, Locator: head.Locator,
		InputSHA256: value.sha, PromptSHA256: sha256Hex(r.Prompt),
		Generated: r.now().Format(time.RFC3339),
		Lines:     len(strings.Split(strings.TrimSpace(body), "\n")),
	}
	if meta.InputSHA256 == "" {
		meta.InputSHA256 = value.job.InputSHA256
	}
	if value.dpi > 0 {
		meta.Flags = append(meta.Flags, fmt.Sprintf("rendered at %d dpi on attempt %d", value.dpi, value.job.Attempts))
	}
	path := corpus.PagePath(r.Root, r.Book, value.page)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return corpus.PageFile{Meta: meta, Body: body}.Write(path)
}

// Head is a transcribed running head taken apart.
type Head struct {
	Label   string
	Title   string
	Locator *corpus.PageLocator
}

// SplitHead separates the running head from the body of a page.
//
// The head is filed in the front matter rather than left in the text, so that
// an OCR page and a natively extracted page have the same shape and assembly
// does not have to ask where a page came from.
//
// The first line is only taken as a head when it reads as one: a page label, a
// section locator, or a short line of capitals followed by a blank. A page that
// prints no head, which is every chapter opener, keeps its first line, because
// eating the opening words of a chapter is a far worse failure than leaving a
// head in the body where a person can see it.
func SplitHead(text string) (Head, string) {
	trimmed := strings.TrimLeft(text, "\n")
	line, rest, _ := strings.Cut(trimmed, "\n")
	line = strings.TrimSpace(line)

	// The very next line, not the next line with something on it. A head stands
	// alone above the text block, and a line of capitals with prose directly
	// under it is a section title, which belongs in the body.
	next, _, _ := strings.Cut(rest, "\n")
	head, ok := readHead(line, strings.TrimSpace(next) == "")
	if !ok {
		return Head{}, strings.TrimSpace(text)
	}
	return head, strings.TrimSpace(rest)
}

func readHead(line string, blankAfter bool) (Head, bool) {
	if line == "" {
		return Head{}, false
	}
	var head Head
	rest := line
	if label, ok := corpus.ParsePageLabel(line); ok {
		head.Label = label.String()
		rest = cutFirst(rest, pageLabelIn(line))
	}
	if locator, ok := corpus.ParseSectionLocator(line); ok {
		head.Locator = &corpus.PageLocator{Section: locator.Section, Subsec: locator.Subsec}
		rest = cutFirst(rest, sectionLocatorIn(line))
	}
	// What is left after the label and the locator have been taken out is the
	// title, less the punctuation that was holding them together.
	head.Title = strings.Trim(strings.Join(strings.Fields(rest), " "), " .,;:")

	// A page label on the first line is unambiguous. No sentence in these
	// volumes opens with A IV.7, so that line is a running head whatever
	// follows it.
	if head.Label != "" {
		return head, true
	}
	// A bare section locator is not. Bourbaki prints § 1. POLYNOMIAL ALGEBRAS
	// as the title of a section, in the text block, and taking that for a
	// running head would file the title of the section as page furniture and
	// drop it out of the body. So a locator, and capitals with nothing else to
	// go on, both have to stand alone above the text, which is what the blank
	// line under them says.
	if blankAfter && (head.Locator != nil || looksLikeHead(line)) {
		return head, true
	}
	return Head{}, false
}

func cutFirst(text, part string) string {
	if part == "" {
		return text
	}
	return strings.Replace(text, part, " ", 1)
}

// These match what corpus.ParsePageLabel and corpus.ParseSectionLocator accept.
// The corpus functions return the parsed value, and what is wanted here is the
// span of text they matched, so that the title is what is left after both have
// been taken out of the line.
var (
	pageLabelFinder      = regexp.MustCompile(`\b[A-Z]{1,3}[.\s]\s*[IVXLCDM]+\s*[.,]\s*\d{1,4}\b`)
	sectionLocatorFinder = regexp.MustCompile(`§\s*\d{1,2}(?:\s*\.\s*\d{1,2})?`)
)

func pageLabelIn(line string) string { return pageLabelFinder.FindString(line) }

func sectionLocatorIn(line string) string { return sectionLocatorFinder.FindString(line) }

func sha256Hex(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

func fileSHA256(path string) (string, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(raw)
	return hex.EncodeToString(sum[:]), nil
}
