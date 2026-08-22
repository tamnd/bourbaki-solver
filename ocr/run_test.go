package ocr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/queue"
)

// fleet is a set of rented boxes that answer whatever a test tells them to.
// Unlike box in batch_test.go it is shared between goroutines, because Do runs
// one per host, and it answers per page rather than uniformly, because the
// interesting runs are the ones where some pages come back wrong.
type fleet struct {
	mu sync.Mutex
	// answer returns what the model said for a page image. An empty string is a
	// page that never came back.
	answer func(image string) string
	// refuse returns why the reader would not return a page at all, or the empty
	// string for a page it was willing to read. A reader that refuses leaves a
	// sidecar next to the answers rather than an answer. See refused.go.
	refuse func(image string) string
	// stale is a sidecar an earlier run left behind, written next to a perfectly
	// good answer rather than instead of one. rsync -az does not delete, and a
	// refusal reuses the batch directory because it does not spend an attempt,
	// so a directory holding both is what a re-read after an outage looks like.
	stale func(image string) string
	// batches records every batch directory the fleet was asked to write into,
	// which is how a test sees that a retry did not reuse a directory.
	batches []string
	// starts records every ocr-batch command line, which is how a test sees
	// what the runner told the reader about the volume.
	starts []string
	// pushErr is what rsync says when the box is not there. A batch that cannot
	// be pushed never reaches a host.
	pushErr error
	pushes  int
	// pending is keyed by batch id rather than by remote path, because the
	// images arrive under in/<id> and are collected from out/<id>.
	pending map[string][]string
	started int
	// stopped is a batch that died before it had written every page, and log is
	// what the tail of its log file says about why.
	stopped bool
	wrote   int
	log     string
}

// batchOf takes the id out of a remote path such as bourbaki-ocr/out/alg-0001-ab12/.
func batchOf(remote string) string { return filepath.Base(strings.TrimSuffix(remote, "/")) }

func newFleet(answer func(image string) string) *fleet {
	return &fleet{answer: answer, pending: map[string][]string{}}
}

func (f *fleet) Run(ctx context.Context, host, command string) (string, error) {
	if err := ctx.Err(); err != nil {
		return "", err
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	switch {
	case strings.Contains(command, "ocr-batch"):
		f.started++
		f.starts = append(f.starts, command)
		return "1234\n", nil
	case strings.Contains(command, "kill -0"):
		if f.stopped {
			return fmt.Sprintf("%d\ngone\n", f.wrote), nil
		}
		// Everything finished between the start and the first poll, which is
		// what a fake fleet is for.
		return "1000000\nrunning\n", nil
	case strings.HasPrefix(strings.TrimSpace(command), "tail"):
		return f.log, nil
	}
	return "", nil
}

func (f *fleet) Push(ctx context.Context, host string, local []string, remote string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.pushes++
	if f.pushErr != nil {
		return f.pushErr
	}
	for _, file := range local {
		if strings.HasPrefix(filepath.Base(file), "prompt-") {
			continue
		}
		id := batchOf(remote)
		f.pending[id] = append(f.pending[id], file)
	}
	return nil
}

func (f *fleet) Pull(ctx context.Context, host, remote, local string) error {
	f.mu.Lock()
	images := f.pending[batchOf(remote)]
	f.batches = append(f.batches, filepath.Base(local))
	answer, refuse, stale := f.answer, f.refuse, f.stale
	f.mu.Unlock()

	if err := os.MkdirAll(local, 0o755); err != nil {
		return err
	}
	for _, image := range images {
		if stale != nil {
			if said := stale(image); said != "" {
				if err := WriteRefusal(local, image, said); err != nil {
					return err
				}
			}
		}
		if refuse != nil {
			if said := refuse(image); said != "" {
				if err := WriteRefusal(local, image, said); err != nil {
					return err
				}
				continue
			}
		}
		body := answer(image)
		if body == "" {
			continue
		}
		if err := os.WriteFile(filepath.Join(local, OutputName(filepath.Base(image))), []byte(body), 0o644); err != nil {
			return err
		}
	}
	return nil
}

// page is a transcription shaped like a real one: a head with a page label, a
// statement, display mathematics, long enough to clear the length rule.
func page(label string) string {
	return label + `  POLYNOMIALS AND RATIONAL FRACTIONS  § 1

**Proposition 4.** — Let $A$ be a commutative ring and $B$ an $A$-algebra. For
every family $(b_\lambda)$ of elements of $B$ there exists a unique homomorphism
$\varphi$ of $A[(X_\lambda)]$ into $B$ such that $\varphi(X_\lambda) = b_\lambda$.

$$\varphi\left(\sum_{\nu} a_\nu X^\nu\right) = \sum_{\nu} a_\nu b^\nu.$$

The image of $\mathbf{Z}$ under $\varphi$ is the prime subring of $B$
(I, p. 23, Proposition 4).`
}

// world is a corpus checkout with images on disk and a queue beside it.
type world struct {
	root  string
	queue *queue.Queue
	pages []Source
}

func newWorld(t *testing.T, pages int) *world {
	t.Helper()
	root := t.TempDir()
	directory := filepath.Join(root, "images", "alg-iv-vii")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		t.Fatal(err)
	}
	w := &world{root: root}
	for n := 1; n <= pages; n++ {
		path := filepath.Join(directory, fmt.Sprintf("%04d.png", n))
		if err := os.WriteFile(path, fmt.Appendf(nil, "page %d", n), 0o644); err != nil {
			t.Fatal(err)
		}
		sum, err := fileSHA256(path)
		if err != nil {
			t.Fatal(err)
		}
		w.pages = append(w.pages, Source{Page: n, SHA256: sum})
	}
	q, err := queue.Open(filepath.Join(root, "work", "queue"))
	if err != nil {
		t.Fatal(err)
	}
	w.queue = q
	return w
}

func (w *world) runner(t *testing.T, machine *fleet) *Runner {
	t.Helper()
	return &Runner{
		Book: "alg-iv-vii", Root: w.root, Queue: w.queue,
		Prompt: "Transcribe the page verbatim, keeping the running head.",
		Model:  "gpt-5",
		Hosts:  []Host{{Name: "server3", Tool: "/root/bin/chatgpt-tool", Lanes: 4}},
		Shell:  machine, Copy: machine, Batch: 4, Sleep: noSleep,
		Expect: func(n int) Expect {
			return Expect{Book: "alg-iv-vii", PDFPage: n, Grammar: pagemap.HeadLabel, HasHead: true}
		},
	}
}

func TestFillSkipsBlanksAndWorkAlreadyDone(t *testing.T) {
	w := newWorld(t, 5)
	runner := w.runner(t, newFleet(nil))
	w.pages[2].Blank = true

	added, err := runner.Fill(w.pages)
	if err != nil {
		t.Fatal(err)
	}
	if added != 4 {
		t.Fatalf("filled %d jobs, want 4 with one blank page skipped", added)
	}
	// Filling again adds nothing. The id is the hash of the image and the
	// prompt, so the queue already has every one of them.
	again, err := runner.Fill(w.pages)
	if err != nil {
		t.Fatal(err)
	}
	if again != 0 {
		t.Errorf("a second fill added %d jobs, want 0", again)
	}
	// A different prompt is different work, but these pages are already in the
	// queue waiting and a batch reads them with whatever prompt the run holds
	// now, not the one that was current when the job was made. Queueing them a
	// second time would put one page in a batch twice, which is the thing that
	// killed twenty one pages of Algebra I.
	runner.Prompt += " Keep the exercises."
	third, err := runner.Fill(w.pages)
	if err != nil {
		t.Fatal(err)
	}
	if third != 0 {
		t.Errorf("a changed prompt added %d jobs on top of the ones already waiting, want 0", third)
	}

	// Once they have run, a changed prompt does queue them again: nothing is
	// waiting any more and what is on disk was read from the old prompt.
	drained, err := runner.Queue.Drain(queue.StageOCR)
	if err != nil || drained == 0 {
		t.Fatalf("drained %d jobs: %v", drained, err)
	}
	fourth, err := runner.Fill(w.pages)
	if err != nil {
		t.Fatal(err)
	}
	if fourth != 4 {
		t.Errorf("a changed prompt on an empty queue added %d jobs, want 4", fourth)
	}
}

func TestFillSkipsAPageAlreadyRead(t *testing.T) {
	w := newWorld(t, 3)
	runner := w.runner(t, newFleet(nil))
	// Page 2 is on disk, read from this image with this prompt, and good.
	writePage(t, w.root, 2, w.pages[1].SHA256, sha256Hex(runner.Prompt), page("A IV.2"))

	added, err := runner.Fill(w.pages)
	if err != nil {
		t.Fatal(err)
	}
	if added != 2 {
		t.Fatalf("filled %d jobs, want 2 with page 2 already read", added)
	}
}

// Written is not the same as read. Pages 50 and 53 of Algebra I sat in the
// corpus failing the math rule from the pilot onwards and no run would touch
// them, because a file existed with the right hashes on it. A page nobody would
// accept is work still to do.
func TestFillReadsARejectedPageAgain(t *testing.T) {
	w := newWorld(t, 3)
	runner := w.runner(t, newFleet(nil))
	// The right image, the right prompt, and a formula nobody closed.
	writePage(t, w.root, 2, w.pages[1].SHA256, sha256Hex(runner.Prompt),
		page("A IV.2")+"\n\nand so $x \\in E, which is where the dollar was lost.\n")

	added, err := runner.Fill(w.pages)
	if err != nil {
		t.Fatal(err)
	}
	if added != 3 {
		t.Fatalf("filled %d jobs, want all 3 with page 2 rejected", added)
	}
}

func writePage(t *testing.T, root string, number int, imageSHA, promptSHA, body string) {
	t.Helper()
	file := corpus.PageFile{Meta: corpus.PageFrontMatter{
		Book: "alg-iv-vii", PDFPage: number, Method: corpus.MethodOCR,
		InputSHA256: imageSHA, PromptSHA256: promptSHA,
	}, Body: body}
	path := corpus.PagePath(root, "alg-iv-vii", number)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := file.Write(path); err != nil {
		t.Fatal(err)
	}
}

func TestARunReadsAVolumeAndWritesIt(t *testing.T) {
	w := newWorld(t, 6)
	machine := newFleet(func(image string) string {
		return page("A IV." + strings.TrimLeft(strings.TrimSuffix(filepath.Base(image), ".png"), "0"))
	})
	runner := w.runner(t, machine)
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}

	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Accepted != 6 || report.Rejected != 0 {
		t.Fatalf("accepted %d rejected %d, want 6 and 0: %s", report.Accepted, report.Rejected, report.Summary())
	}
	if report.Rate() != 100 {
		t.Errorf("rate %.1f, want 100", report.Rate())
	}
	if report.PerHost["server3"] != 6 {
		t.Errorf("server3 read %d pages", report.PerHost["server3"])
	}

	// The head is filed in the front matter and taken out of the body, so an
	// OCR page has the same shape as a natively extracted one.
	file, err := corpus.ReadFile[corpus.PageFrontMatter](corpus.PagePath(w.root, "alg-iv-vii", 3))
	if err != nil {
		t.Fatal(err)
	}
	if file.Meta.PageLabel != "A IV.3" {
		t.Errorf("page label %q, want A IV.3", file.Meta.PageLabel)
	}
	if file.Meta.RunningHead != "POLYNOMIALS AND RATIONAL FRACTIONS" {
		t.Errorf("running head %q", file.Meta.RunningHead)
	}
	if file.Meta.Locator == nil || file.Meta.Locator.Section != 1 {
		t.Errorf("locator %+v, want § 1", file.Meta.Locator)
	}
	if file.Meta.Method != corpus.MethodOCR || file.Meta.Model != "gpt-5" {
		t.Errorf("front matter says %s by %s", file.Meta.Method, file.Meta.Model)
	}
	if file.Meta.InputSHA256 != w.pages[2].SHA256 {
		t.Errorf("the page records the wrong image hash")
	}
	if strings.HasPrefix(file.Body, "A IV.3") {
		t.Errorf("the running head was left in the body:\n%s", file.Body)
	}
	if !strings.Contains(file.Body, "Proposition 4") {
		t.Errorf("the body lost its statement:\n%s", file.Body)
	}

	// The queue agrees. Every page is done, nothing is left leased.
	stats, err := w.queue.Stats(queue.StageOCR)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Counts[queue.Done] != 6 || stats.Counts[queue.Leased] != 0 || stats.Counts[queue.Pending] != 0 {
		t.Errorf("queue after the run: %+v", stats.Counts)
	}
}

func TestARejectedPageIsTriedAgainAtAHigherResolution(t *testing.T) {
	w := newWorld(t, 2)
	// Page 1 comes back as a refusal every time. Page 2 is fine.
	machine := newFleet(func(image string) string {
		if strings.HasSuffix(image, "0001.png") {
			return "I'm sorry, I can't transcribe this image."
		}
		return page("A IV.2")
	})
	runner := w.runner(t, machine)
	var rerendered []int
	runner.RetryDPI = 600
	runner.Rerender = func(ctx context.Context, page, dpi int) (int, error) {
		if dpi != 600 {
			t.Errorf("re-rendered at %d dpi, want 600", dpi)
		}
		rerendered = append(rerendered, page)
		return dpi, nil
	}
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}

	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Three attempts at page 1, one accepted page 2.
	if report.Accepted != 1 {
		t.Errorf("accepted %d, want 1: %s", report.Accepted, report.Summary())
	}
	if report.Rejected != 3 {
		t.Errorf("rejected %d, want 3 attempts at the one bad page", report.Rejected)
	}
	if report.Dead != 1 {
		t.Errorf("dead %d, want 1", report.Dead)
	}
	if report.Rules[RuleLeak] != 3 {
		t.Errorf("the leak rule fired %d times, want 3", report.Rules[RuleLeak])
	}
	// The first attempt uses the image that is already there. Attempts two and
	// three escalate.
	if fmt.Sprint(rerendered) != "[1 1]" {
		t.Errorf("re-rendered %v, want page 1 twice", rerendered)
	}
	// The audit has to be able to name it.
	if len(report.Failures) == 0 || report.Failures[0].Page != 1 {
		t.Fatalf("failures %+v", report.Failures)
	}
	if report.Failures[len(report.Failures)-1].State != queue.Dead {
		t.Errorf("the last attempt left the job in %s", report.Failures[len(report.Failures)-1].State)
	}

	// Every attempt got its own directory on the host. ocr-batch is run with
	// --skip-existing, so a retry that reused a directory would find the
	// rejected answer sitting there and skip the page it was sent to re-read.
	seen := map[string]bool{}
	for _, name := range machine.batches {
		if seen[name] {
			t.Errorf("two batches shared the directory %s", name)
		}
		seen[name] = true
	}
}

func TestAPageThatNeverCameBackIsRejectedRatherThanLost(t *testing.T) {
	w := newWorld(t, 3)
	machine := newFleet(func(image string) string {
		if strings.HasSuffix(image, "0002.png") {
			return "" // the tool wrote nothing for this one
		}
		return page("A IV.1")
	})
	runner := w.runner(t, machine)
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}
	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Accepted != 2 {
		t.Errorf("accepted %d, want 2", report.Accepted)
	}
	var found bool
	for _, failure := range report.Failures {
		if failure.Page == 2 && strings.Contains(failure.Reason, "no answer") {
			found = true
		}
	}
	if !found {
		t.Errorf("the missing page was not reported: %+v", report.Failures)
	}
}

func TestALimitStopsTheRunEarly(t *testing.T) {
	w := newWorld(t, 20)
	machine := newFleet(func(string) string { return page("A IV.9") })
	runner := w.runner(t, machine)
	runner.Batch = 3
	runner.Limit = 6
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}
	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Accepted + report.Rejected; got != 6 {
		t.Fatalf("read %d pages, want the 6 that were asked for", got)
	}
	stats, err := w.queue.Stats(queue.StageOCR)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Counts[queue.Pending] != 14 {
		t.Errorf("%d pages left pending, want 14", stats.Counts[queue.Pending])
	}
}

// A volume is read a chapter at a time, because a chapter is what assembly
// works in, and a run asked for pages 5 to 8 has to read those and not the four
// pages that happen to be at the head of the queue. Fill has always taken the
// range and leasing did not, so on a queue filled once already the range chose
// nothing and the run read the front matter instead.
func TestARunReadsOnlyThePagesItWasAskedFor(t *testing.T) {
	w := newWorld(t, 20)
	machine := newFleet(func(string) string { return page("A IV.9") })
	runner := w.runner(t, machine)
	runner.First, runner.Last = 5, 8
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}
	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got := report.Accepted + report.Rejected; got != 4 {
		t.Fatalf("read %d pages, want the 4 of the range", got)
	}
	stats, err := w.queue.Stats(queue.StageOCR)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Counts[queue.Pending] != 16 {
		t.Errorf("%d pages left pending, want the 16 outside the range", stats.Counts[queue.Pending])
	}
	for p := 5; p <= 8; p++ {
		if _, err := os.Stat(corpus.PagePath(w.root, "alg-iv-vii", p)); err != nil {
			t.Errorf("page %d of the range was not read: %v", p, err)
		}
	}
	if _, err := os.Stat(corpus.PagePath(w.root, "alg-iv-vii", 1)); !os.IsNotExist(err) {
		t.Errorf("page 1 was read and nothing asked for it: %v", err)
	}
}

func TestAHostWithNoLanesIsNotAskedToReadAnything(t *testing.T) {
	w := newWorld(t, 2)
	machine := newFleet(func(string) string { return page("A IV.1") })
	runner := w.runner(t, machine)
	// server1 has under a gigabyte free, which is not enough for a browser.
	runner.Hosts = []Host{
		{Name: "server1", Tool: "/home/tam/bin/chatgpt-tool", Lanes: 0},
		{Name: "server3", Tool: "/root/bin/chatgpt-tool", Lanes: 4},
	}
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}
	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.PerHost["server1"] != 0 {
		t.Errorf("server1 was given %d pages", report.PerHost["server1"])
	}
	if report.Accepted != 2 {
		t.Errorf("accepted %d, want 2", report.Accepted)
	}
}

func TestTwoHostsShareTheQueueWithoutReadingAPageTwice(t *testing.T) {
	w := newWorld(t, 12)
	machine := newFleet(func(string) string { return page("A IV.5") })
	runner := w.runner(t, machine)
	runner.Batch = 2
	runner.Hosts = []Host{
		{Name: "server3", Tool: "/root/bin/chatgpt-tool", Lanes: 4},
		{Name: "server2", Tool: "/root/bin/chatgpt-tool", Lanes: 3},
	}
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}
	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Accepted != 12 {
		t.Fatalf("accepted %d of 12: %s", report.Accepted, report.Summary())
	}
	if total := report.PerHost["server3"] + report.PerHost["server2"]; total != 12 {
		t.Errorf("the hosts read %d pages between them, want 12", total)
	}
	// The lease is the mutual exclusion, so no page is read twice however the
	// two goroutines interleave.
	if machine.started != len(machine.batches) {
		t.Errorf("%d batches started, %d pulled", machine.started, len(machine.batches))
	}
}

func TestSplitHeadKeepsAPageThatPrintsNoHead(t *testing.T) {
	cases := []struct {
		name  string
		text  string
		label string
		title string
		sec   int
		body  string
	}{
		{
			name: "page label and locator", label: "A IV.7", title: "POLYNOMIALS", sec: 1,
			text: "A IV.7  POLYNOMIALS  § 1\n\nLet $A$ be a ring.",
			body: "Let $A$ be a ring.",
		},
		{
			name:  "locator only, which is how chapters I to III print it",
			text:  "§ 4.5  ALGEBRAIC STRUCTURES\n\nLet $G$ be a group.",
			title: "ALGEBRAIC STRUCTURES", sec: 4,
			body: "Let $G$ be a group.",
		},
		{
			// A chapter opener prints no head. Eating its first line would lose
			// the opening words of a chapter and nothing would say so.
			name: "chapter opener",
			text: "In this chapter every ring is commutative and has a unit.\n\nLet $A$ be such a ring.",
			body: "In this chapter every ring is commutative and has a unit.\n\nLet $A$ be such a ring.",
		},
		{
			// A locator with prose directly underneath is the title of a
			// section, printed in the text block. Taking it for a running head
			// would file the title of the section as page furniture and drop it
			// out of the body.
			name: "section title, no blank line under it",
			text: "§ 1. POLYNOMIAL ALGEBRAS\nLet $A$ be a ring.",
			body: "§ 1. POLYNOMIAL ALGEBRAS\nLet $A$ be a ring.",
		},
		{
			name:  "capitals standing alone",
			text:  "MONOIDS, GROUPS\n\nLet $G$ be a monoid.",
			title: "MONOIDS, GROUPS",
			body:  "Let $G$ be a monoid.",
		},
		{
			// The page that opens a chapter of Theory of Sets. The heading
			// stands alone above the title of the chapter exactly as a running
			// head stands above the text block, and reading it as one leaves
			// the chapter with nothing on it for the assembler to find.
			name: "a marked heading is body",
			text: "## CHAPTER I Description of Formal Mathematics\n\n## 1. TERMS AND RELATIONS",
			body: "## CHAPTER I Description of Formal Mathematics\n\n## 1. TERMS AND RELATIONS",
		},
		{
			name: "a marked heading of capitals is body too",
			text: "## APPENDIX\n\n## CHARACTERIZATION OF TERMS AND RELATIONS",
			body: "## APPENDIX\n\n## CHARACTERIZATION OF TERMS AND RELATIONS",
		},
		{
			// A page whose first line carries on a paragraph from the page
			// before, and cites another page while doing it. ParsePageLabel
			// searches the line rather than matching it, so this took the
			// unambiguous branch and the whole paragraph was cut out of the
			// body and filed as the title of a running head. 256 of the 4492
			// raw pages in the corpus open with a line of this shape.
			name: "a paragraph that cites a page is not a head",
			text: "still hold for generalized formal power series, by the argument of A VIII.202, " +
				"and the corollary above applies to each of them without change here.\n\n" +
				"Let $A$ be a ring.",
			body: "still hold for generalized formal power series, by the argument of A VIII.202, " +
				"and the corollary above applies to each of them without change here.\n\n" +
				"Let $A$ be a ring.",
		},
		{
			// The other letterless line. A reader that opens the page on a
			// display writes \[ and nothing else, and the capitals test reads a
			// line with no letters in it as a bare folio.
			name: "a display opener is not a bare folio",
			text: "\\[\n\n= \\prod_{x \\in H} g_1 c(s(x)^{-1})\n\\]",
			body: "\\[\n\n= \\prod_{x \\in H} g_1 c(s(x)^{-1})\n\\]",
		},
		{
			// The line the digit rule has to keep working for.
			name:  "a bare folio is still a head",
			text:  "496\n\nTABLE OF CONTENTS",
			title: "496",
			body:  "TABLE OF CONTENTS",
		},
	}
	for _, test := range cases {
		head, body := SplitHead(test.text)
		if head.Label != test.label {
			t.Errorf("%s: label %q, want %q", test.name, head.Label, test.label)
		}
		if head.Title != test.title {
			t.Errorf("%s: title %q, want %q", test.name, head.Title, test.title)
		}
		section := 0
		if head.Locator != nil {
			section = head.Locator.Section
		}
		if section != test.sec {
			t.Errorf("%s: section %d, want %d", test.name, section, test.sec)
		}
		if body != test.body {
			t.Errorf("%s: body\n%q\nwant\n%q", test.name, body, test.body)
		}
	}
}

func TestSplitHeadReadsTheSubsection(t *testing.T) {
	head, _ := SplitHead("A IV.7  POLYNOMIALS  § 2.5\n\nLet $A$ be a ring.")
	if head.Locator == nil || head.Locator.Section != 2 || head.Locator.Subsec != 5 {
		t.Fatalf("locator %+v, want § 2 no. 5", head.Locator)
	}
}

func TestARunNeedsSomewhereToSendWork(t *testing.T) {
	w := newWorld(t, 1)
	machine := newFleet(nil)
	for name, change := range map[string]func(*Runner){
		"no book":   func(r *Runner) { r.Book = "" },
		"no prompt": func(r *Runner) { r.Prompt = "" },
		"no hosts":  func(r *Runner) { r.Hosts = nil },
		"no shell":  func(r *Runner) { r.Shell = nil },
	} {
		runner := w.runner(t, machine)
		change(runner)
		if _, err := runner.Do(context.Background()); err == nil {
			t.Errorf("%s: a run with nowhere to go reported success", name)
		}
	}
}

func TestABadTargetDoesNotStallTheQueue(t *testing.T) {
	w := newWorld(t, 1)
	if _, err := w.queue.Add(queue.New(queue.StageOCR, "nonsense", "a", "b")); err != nil {
		t.Fatal(err)
	}
	machine := newFleet(func(string) string { return page("A IV.1") })
	runner := w.runner(t, machine)
	if _, err := runner.Do(context.Background()); err != nil {
		t.Fatal(err)
	}
	// It failed rather than hanging in leased forever.
	stats, err := w.queue.Stats(queue.StageOCR)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Counts[queue.Leased] != 0 {
		t.Errorf("%d jobs left leased", stats.Counts[queue.Leased])
	}
}

func TestTheReportSaysWhatEachHostCost(t *testing.T) {
	report := Report{
		Book: "alg-i-iii", Accepted: 700, Rejected: 34, Dead: 2,
		Started: time.Unix(0, 0).UTC(), Finished: time.Unix(0, 0).UTC().Add(6 * time.Hour),
		PerHost:   map[string]int{"server3": 400, "server2": 300},
		HostTimes: map[string]Duration{"server3": Duration(4 * time.Hour), "server2": Duration(5 * time.Hour)},
		Rules:     map[Rule]int{RuleHead: 20, RuleShort: 14},
	}
	got := report.Summary()
	for _, want := range []string{
		"700 accepted, 34 rejected, 2 dead",
		"95.4 % accepted",
		"server3   400 pages",
		"100 pages an hour",
		"head         20",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the summary has no %q in it:\n%s", want, got)
		}
	}
	// The rules are listed worst first, so the line that matters is the one at
	// the top rather than whichever the map iterated to.
	if strings.Index(got, "head") > strings.Index(got, "short") {
		t.Errorf("the rules are not in order:\n%s", got)
	}
	if (Report{}).Rate() != 0 {
		t.Error("an empty report claimed a rate")
	}
}

// The header below is verbatim from what ocr-batch wrote on server3. Its source
// line is an absolute path in somebody's home directory on a rented box, which
// is the part that must not reach a public corpus.
func TestTheToolsOwnHeaderIsNotPartOfThePage(t *testing.T) {
	raw := "---\n" +
		"source: /root/bourbaki-ocr/in/alg-i-iii-0042-c9d938/0054.png\n" +
		"model: gpt-5-6\n" +
		"fetched: 2026-08-10 16:42\n" +
		"elapsed: 149.9s\n" +
		"---\n\n" +
		"I ALGEBRAIC STRUCTURES\n\n§ 4. GROUPS AND GROUPS WITH OPERATORS\n"

	body := StripToolHeader(raw)
	if strings.Contains(body, "/root/") {
		t.Errorf("the path on the rented box survived into the page:\n%s", body)
	}
	if strings.Contains(body, "elapsed:") || strings.Contains(body, "gpt-5-6") {
		t.Errorf("the tool's own header survived:\n%s", body)
	}
	if !strings.HasPrefix(body, "I ALGEBRAIC STRUCTURES") {
		t.Errorf("the page does not start at its running head:\n%s", body)
	}
}

// Stripping it is what makes the length rule mean anything. The refusal below
// is 131 characters and the header is another 126, and with the header on top
// it cleared the 200 the rule asks for and was written to the corpus.
func TestTheHeaderNoLongerPadsARefusalPastTheLengthRule(t *testing.T) {
	raw := "---\n" +
		"source: /root/bourbaki-ocr/in/alg-i-iii-0042-c9d938/0042.png\n" +
		"model: gpt-5-6\n" +
		"fetched: 2026-08-10 16:42\n" +
		"elapsed: 188.4s\n" +
		"---\n\n" +
		"I don’t see an image attached to this message. Please upload the page image, and I’ll transcribe it exactly according to your specifications.\n"

	if len([]rune(raw)) < MinChars {
		t.Fatalf("this test is not testing what it says: the raw answer is only %d characters", len([]rune(raw)))
	}
	body := StripToolHeader(raw)
	problems := Validate(body, Expect{Book: "alg-i-iii", PDFPage: 42}, Options{})
	if len(problems) == 0 {
		t.Fatal("a model that never saw the page was accepted")
	}
	if problems[0].Rule != RuleLeak {
		t.Errorf("rejected as %s, want %s: %s", problems[0].Rule, RuleLeak, problems[0].Detail)
	}
}

// A page that really opens with a rule keeps it, and so does an answer with no
// header at all. Only the four keys the tool writes are taken for machinery.
func TestOnlyTheToolsHeaderIsStripped(t *testing.T) {
	for _, text := range []string{
		"A I.24  ALGEBRAIC STRUCTURES\n\nLet $G$ be a group.",
		"---\n\nA horizontal rule opens this page.",
		"---\ntitle: something else\nauthor: nobody\n---\n\nkept",
	} {
		if got := StripToolHeader(text); got != text {
			t.Errorf("stripped something it should not have:\n%q\nbecame\n%q", text, got)
		}
	}
}

// broken is the same page with one inline formula opened and never closed,
// which is what a real answer looked like on page 50 of Algebra I. Everything
// else about it is right, and reading it again costs a full page.
func broken(label string) string {
	return strings.Replace(page(label), "prime subring of $B$", "prime subring of $B", 1)
}

func TestAPageThatFailsOnADelimiterIsAskedAboutRatherThanReadAgain(t *testing.T) {
	w := newWorld(t, 2)
	machine := newFleet(func(image string) string {
		if strings.HasSuffix(image, "0001.png") {
			return "---\nsource: /root/x.png\nmodel: gpt-5\ngenerated: now\nelapsed: 63s\n" +
				"conversation: https://chatgpt.com/c/abc-1\nprofile: /root/.config/chatgpt-profile-3\n---\n\n" + broken("A IV.1")
		}
		return page("A IV.2")
	})
	runner := w.runner(t, machine)

	var asked Thread
	runner.Repair = func(ctx context.Context, thread Thread, page int, text string, problems []Problem) (string, bool) {
		asked = thread
		return strings.Replace(text, "prime subring of $B", "prime subring of $B$", 1), true
	}
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}
	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Accepted != 2 || report.Rejected != 0 {
		t.Fatalf("accepted %d rejected %d: %s", report.Accepted, report.Rejected, report.Summary())
	}
	if report.Repaired != 1 {
		t.Errorf("the report says %d pages were repaired, want 1", report.Repaired)
	}
	// The follow up has to go to the conversation that read the page and to the
	// box that conversation lives on, or it goes nowhere.
	if asked.Conversation != "https://chatgpt.com/c/abc-1" || asked.Host != "server3" {
		t.Errorf("the repair was offered %+v", asked)
	}
	file, err := corpus.ReadFile[corpus.PageFrontMatter](corpus.PagePath(w.root, "alg-iv-vii", 1))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(file.Body, "prime subring of $B$") {
		t.Errorf("the repaired text is not what was written:\n%s", file.Body)
	}
	// A mended page passes every rule now, so nothing else in the corpus would
	// ever say it was not the transcription the model first gave.
	if len(file.Meta.Flags) == 0 || !strings.Contains(file.Meta.Flags[0], "repaired in its own thread") {
		t.Errorf("the page does not record that it was repaired: %v", file.Meta.Flags)
	}
}

func TestARepairThatIsRefusedLeavesThePageForTheQueue(t *testing.T) {
	w := newWorld(t, 1)
	machine := newFleet(func(image string) string {
		return "---\nsource: /root/x.png\nmodel: gpt-5\ngenerated: now\nelapsed: 63s\n" +
			"conversation: https://chatgpt.com/c/abc-1\nprofile: /root/.config/chatgpt-profile-3\n---\n\n" + broken("A IV.1")
	})
	runner := w.runner(t, machine)
	runner.Repair = func(context.Context, Thread, int, string, []Problem) (string, bool) { return "", false }
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}
	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	// Back to the queue, which tries it twice more at a higher resolution and
	// then gives up on it. That is what a page with no provable repair costs,
	// and it is the price of not writing a guess into the corpus.
	if report.Accepted != 0 || report.Repaired != 0 || report.Dead != 1 {
		t.Fatalf("a refused repair did not send the page back: %s", report.Summary())
	}
	if _, err := os.Stat(corpus.PagePath(w.root, "alg-iv-vii", 1)); !os.IsNotExist(err) {
		t.Error("a page whose repair was refused was written anyway")
	}
}

// A page read before the tool reported conversation urls has nothing to ask in,
// and asking in the wrong thread is worse than reading the page again.
func TestAPageWithNoThreadIsNotOfferedForRepair(t *testing.T) {
	w := newWorld(t, 1)
	machine := newFleet(func(image string) string { return broken("A IV.1") })
	runner := w.runner(t, machine)
	var offered int
	runner.Repair = func(context.Context, Thread, int, string, []Problem) (string, bool) {
		offered++
		return "", false
	}
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Do(context.Background()); err != nil {
		t.Fatal(err)
	}
	if offered != 0 {
		t.Errorf("a page with no conversation was offered for repair %d times", offered)
	}
	if _, err := ReadThread(w.root, "alg-iv-vii", 1); err == nil {
		t.Error("a thread was recorded for a page whose answer carried no conversation")
	}
}

// A batch that fails on the way out, before chatgpt-tool ever runs, still has
// to say which box it was and how many pages it was carrying. Two lines in the
// real usage log have none of that on them, which makes them unreadable a week
// later.
func TestAFailedBatchStillNamesTheBox(t *testing.T) {
	result := named(Result{}, "server2", "alg-i-iii-0050-ab12cd", 4, errors.New("ssh: connect to host server2 port 22: connection refused"))
	if result.Host != "server2" || result.ID != "alg-i-iii-0050-ab12cd" {
		t.Fatalf("result = %+v", result)
	}
	if result.Pages != 4 {
		t.Errorf("Pages = %d, want the 4 it was carrying", result.Pages)
	}
	if !strings.Contains(result.Log, "connection refused") {
		t.Errorf("Log = %q, want the error that stopped it", result.Log)
	}
	if result.Wrote != 0 {
		t.Errorf("Wrote = %d, want none", result.Wrote)
	}
}

// What the tool reported is never overwritten by the fallback.
func TestAFinishedBatchKeepsWhatItReported(t *testing.T) {
	result := named(Result{Host: "server3", ID: "real", Pages: 6, Wrote: 6, Log: "kept"}, "server2", "guess", 4, nil)
	if result.Host != "server3" || result.ID != "real" || result.Pages != 6 || result.Log != "kept" {
		t.Fatalf("result = %+v", result)
	}
}

// A batch that cannot be sent has read nothing. The pages have to come back
// with their attempts unspent, or a local error costs the same as three bad
// readings: twenty one pages of Algebra I went from pending to dead in forty
// one seconds that way, without one image leaving the laptop.
func TestABatchThatNeverWentOutCostsNoAttempts(t *testing.T) {
	w := newWorld(t, 4)
	machine := newFleet(func(string) string { return page("42") })
	machine.pushErr = errors.New("ssh: connect to host server3 port 22: no route to host")
	runner := w.runner(t, machine)
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}

	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Rejected != 0 || report.Dead != 0 {
		t.Errorf("a batch that never went out rejected %d and killed %d pages", report.Rejected, report.Dead)
	}
	if report.Released != 4 {
		t.Errorf("released = %d, want the 4 pages handed back", report.Released)
	}
	// One batch, not one per page: a host that hands everything back is done
	// for this run, or the loop leases the same pages again at the speed of a
	// local error.
	if machine.pushes != 1 {
		t.Errorf("the run tried the dead host %d times, want once", machine.pushes)
	}

	stats, err := w.queue.Stats(queue.StageOCR)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Counts[queue.Pending] != 4 || stats.Counts[queue.Dead] != 0 {
		t.Fatalf("queue = %+v, want the four pages waiting", stats.Counts)
	}
	for _, job := range mustList(t, w.queue, queue.Pending) {
		if job.Attempts != 0 {
			t.Errorf("%s came back with %d attempts spent, want 0", job.Target, job.Attempts)
		}
		if len(job.History) != 1 || job.History[0].Reason == "" {
			t.Errorf("%s says nothing about being handed back: %+v", job.Target, job.History)
		}
	}
}

// The model being out of turns is a pause, not four bad pages.
//
// A reader whose account has no turns left refuses every page after the first
// in about a second each. Filing those as attempts spends a page's four tries
// on an account that will be back within the hour, and the fourth one kills the
// page for good. So the pages that were not reached go back untouched, the ones
// that were read are still filed, and nothing more is sent to that host in this
// run.
func TestAPauseCostsThePagesNoAttempts(t *testing.T) {
	w := newWorld(t, 4)
	machine := newFleet(func(image string) string {
		if strings.HasSuffix(image, "0001.png") {
			return page("A IV.1")
		}
		return ""
	})
	machine.stopped, machine.wrote = true, 1
	machine.log = "0002.png: " + OutOfTurnsMark + " for now, stopping the batch: You've hit your session limit"
	runner := w.runner(t, machine)
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}

	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Accepted != 1 {
		t.Errorf("accepted %d, want the one page that was read before the pause", report.Accepted)
	}
	if report.Rejected != 0 || report.Dead != 0 {
		t.Errorf("a pause rejected %d pages and killed %d", report.Rejected, report.Dead)
	}
	if report.Released != 3 {
		t.Errorf("released %d, want the 3 pages the reader never reached", report.Released)
	}
	if machine.started != 1 {
		t.Errorf("the run started %d batches against a reader that is out of turns, want 1", machine.started)
	}
	for _, job := range mustList(t, w.queue, queue.Pending) {
		if job.Attempts != 0 {
			t.Errorf("%s came back with %d attempts spent, want 0", job.Target, job.Attempts)
		}
	}
}

// The pause is a fleet thing before it is a local thing, and the fleet says it
// in its own words. Ten pages of Theory of Sets were killed in two minutes
// because these lines were in the log and nothing read them.
func TestAPauseIsRecognisedInEveryReadersWords(t *testing.T) {
	paused := []string{
		"0002.png: " + OutOfTurnsMark + " for now: You've hit your session limit",
		"fail  0394.png  0.0s  not attempted: every account on this host is out of uploads until 20:14:17",
		"no-response  0385.png  retry in 30s (attempt 1/4): chatgpt-profile-12 has no uploads left, the composer says 'wait 9 hours to upload again'",
	}
	for _, log := range paused {
		if !OutOfTurns(log) {
			t.Errorf("this reads as ten bad pages rather than a pause:\n%s", log)
		}
	}
	working := []string{
		"", "OCR done=10 fail=0",
		"fail  0042.png  151.2s  the model returned nothing",
		"[ban] Profile slot chatgpt-profile-12 rate-limited until 01:36:01 — will rotate to a fresh slot.",
	}
	for _, log := range working {
		if OutOfTurns(log) {
			t.Errorf("this reads as a pause, so the host is taken out of the run over one bad page:\n%s", log)
		}
	}
}

// The tool wraps its log to the width of the terminal it thinks it has, so the
// break falls inside the sentence and the phrase is not there to be found. This
// is server3's log as it came off the host, and it cost ten pages on the first
// run after the pause was supposedly understood.
func TestAPauseIsRecognisedThroughTheToolsLineWrapping(t *testing.T) {
	wrapped := []string{
		"  fail  0384.png  0.0s  not attempted: every account on this host is out of \nuploads until 20:14:17",
		"because no account here can upload: all 11 verified slot(s) are banned, the \nearliest lifts at 20:14:17, which is 25 minutes away",
		"  fail  0381.png  0.0s  upload cap: all 11 verified slot(s) are banned, the \nearliest lifts at 20:14:17, which is 25 minutes ",
	}
	for _, log := range wrapped {
		if !OutOfTurns(log) {
			t.Errorf("a line break inside the phrase hid the pause:\n%s", log)
		}
	}
}

// The whole batch failing in a tenth of a second each is what a spent account
// looks like, and every one of those pages has to come back with its attempts
// where they were.
func TestAFleetOutOfUploadsCostsThePagesNoAttempts(t *testing.T) {
	w := newWorld(t, 4)
	machine := newFleet(func(string) string { return "" })
	machine.stopped, machine.wrote = true, 0
	machine.log = "fail  0001.png  0.0s  not attempted: every account on this host is out of uploads until 20:14:17"
	runner := w.runner(t, machine)
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}

	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Rejected != 0 || report.Dead != 0 {
		t.Errorf("a spent account rejected %d pages and killed %d, want neither", report.Rejected, report.Dead)
	}
	if report.Released != 4 {
		t.Errorf("released %d, want all 4 pages back", report.Released)
	}
	if machine.started != 1 {
		t.Errorf("the run started %d batches against a host with nothing left, want 1", machine.started)
	}
	for _, job := range mustList(t, w.queue, queue.Pending) {
		if job.Attempts != 0 {
			t.Errorf("%s came back with %d attempts spent, want 0", job.Target, job.Attempts)
		}
	}
}

// The Historical Note of Theory of Sets is prose, and a reader whose provider
// will not return a long verbatim stretch of a published book returns nothing
// for those pages. That is not the page being wrong, and it must not be paid
// for out of the page's four attempts: those pages read on the fleet without
// complaint, and a page killed here is a page missing from the corpus for good.
func TestARefusedPageKeepsItsAttempts(t *testing.T) {
	w := newWorld(t, 4)
	machine := newFleet(func(string) string { return page("A IV.1") })
	machine.refuse = func(image string) string {
		if strings.HasSuffix(image, "0003.png") {
			return "API Error: 400 Output blocked by content filtering policy"
		}
		return ""
	}
	runner := w.runner(t, machine)
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}

	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Accepted != 3 {
		t.Errorf("accepted %d, want the 3 pages the reader was willing to read", report.Accepted)
	}
	if report.Rejected != 0 || report.Dead != 0 {
		t.Errorf("a refusal rejected %d pages and killed %d, want neither", report.Rejected, report.Dead)
	}
	if report.Refused != 1 {
		t.Errorf("refused %d, want the one page the reader would not return", report.Refused)
	}
	if len(report.Refusals) != 1 || report.Refusals[0] != 3 {
		t.Errorf("the refused pages are %v, want page 3 named", report.Refusals)
	}
	// A refused page is not evidence about how well the model reads, so it is
	// out of the rate rather than counted against it.
	if report.Rate() != 100 {
		t.Errorf("the rate is %.1f %%, want 100 with the refusal left out of it", report.Rate())
	}
	pending := mustList(t, w.queue, queue.Pending)
	if len(pending) != 1 || pending[0].Target != Target("alg-iv-vii", 3) {
		t.Fatalf("pending is %v, want page 3 still waiting for a host that will read it", pending)
	}
	if pending[0].Attempts != 0 {
		t.Errorf("the refused page came back with %d attempts spent, want 0", pending[0].Attempts)
	}
}

// The release puts the page back in pending, where the next lease finds it, and
// the answer is the same every time. Without the host remembering its own
// refusals a run reads the last twenty five pages of the volume for ever.
func TestARefusedPageIsNotOfferedToThatHostAgain(t *testing.T) {
	w := newWorld(t, 3)
	machine := newFleet(func(string) string { return page("A IV.1") })
	machine.refuse = func(string) string { return "API Error: 400 Output blocked by content filtering policy" }
	runner := w.runner(t, machine)
	runner.Batch = 3
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}

	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Refused != 3 {
		t.Errorf("refused %d of 3 pages", report.Refused)
	}
	if machine.started != 1 {
		t.Errorf("the run started %d batches, want 1: the refused pages must not come round again", machine.started)
	}
}

// gamingpc's reader was down for forty two minutes and every page offered to it
// in that window got a sidecar reading "ConnectError: All connection attempts
// failed". Those pages went back to pending, which is right, and were read again
// afterwards, which worked. Every one of the readings was then thrown away
// unopened, because the sidecar from the outage was still sitting beside it: a
// refusal does not spend an attempt, so the retry hashed to the same batch id
// and landed in the same directory, and rsync -az does not delete. Two whole
// volumes could not be finished by any number of retries. The answer wins.
func TestAnAnswerBeatsTheSidecarAnEarlierOutageLeft(t *testing.T) {
	w := newWorld(t, 3)
	machine := newFleet(func(string) string { return page("A IV.1") })
	machine.stale = func(string) string { return "ConnectError: All connection attempts failed" }
	runner := w.runner(t, machine)
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}

	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Accepted != 3 {
		t.Errorf("accepted %d of 3 pages that all came back read", report.Accepted)
	}
	if report.Refused != 0 {
		t.Errorf("refused %d pages that have an answer on disk, want 0", report.Refused)
	}
	if left := mustList(t, w.queue, queue.Pending); len(left) != 0 {
		t.Errorf("%d pages are still pending, want none: they were read", len(left))
	}
}

// The other half of the same rule. With no answer beside it the sidecar is the
// only account of what happened to the page, and it still has to be believed.
func TestAnEmptyAnswerIsNoAnswerAndTheSidecarSpeaks(t *testing.T) {
	w := newWorld(t, 1)
	machine := newFleet(func(string) string { return "   \n" })
	machine.stale = func(string) string { return "ConnectError: All connection attempts failed" }
	runner := w.runner(t, machine)
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}

	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Refused != 1 {
		t.Errorf("refused %d, want the one page whose only account is the sidecar", report.Refused)
	}
	if report.Rejected != 0 {
		t.Errorf("rejected %d, want 0: a page nobody read must keep its attempts", report.Rejected)
	}
}

// corpus.PageFrontMatter has said since the field was added that a page marked
// manual must not be written over, and extract has honoured it from the start.
// The OCR route never learned it. Running the flagged pages of Algebra VIII
// rewrote 36 pages, 19 of them marked, and seven came back with every \mathscr
// turned into \mathcal. Those pages carry the accents of § 21 and the summation
// sign of § 5, which is the work the field exists to protect.
func TestAPageRepairedByHandIsNotWrittenOver(t *testing.T) {
	w := newWorld(t, 3)
	repaired := "A IV.2  BY HAND  § 1\n\nSomething only a person reading the printed page could put here.\n"
	byHand(t, w.root, 2, repaired)

	machine := newFleet(func(string) string { return page("A IV.9") })
	runner := w.runner(t, machine)
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}

	report, err := runner.Do(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if report.Kept != 1 {
		t.Errorf("kept %d pages, want the one somebody had repaired by hand", report.Kept)
	}
	if report.Accepted != 2 {
		t.Errorf("accepted %d, want the 2 pages nobody had touched", report.Accepted)
	}
	got, err := corpus.ReadFile[corpus.PageFrontMatter](corpus.PagePath(w.root, "alg-iv-vii", 2))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got.Body, "only a person reading the printed page") {
		t.Errorf("the repair was written over, the page now reads %q", got.Body)
	}
	if !got.Meta.Manual {
		t.Error("the page lost its manual mark, so the next run would write over it too")
	}
	// A page nobody may write is not work left to do. Handing it back would put
	// it in front of the model again on every run for ever.
	for _, job := range mustList(t, w.queue, queue.Pending) {
		if job.Target == Target("alg-iv-vii", 2) {
			t.Error("the hand repaired page is still pending, so it will be read again next run")
		}
	}
}

// Pictured is the same promise about a narrower claim and is honoured the same
// way. Method: ocr is not: a page read by this route is what a retry replaces.
func TestOnlyAHumanClaimStopsAReread(t *testing.T) {
	for _, c := range []struct {
		name string
		set  func(*corpus.PageFrontMatter)
		want int
	}{
		{"manual", func(m *corpus.PageFrontMatter) { m.Manual = true }, 1},
		{"pictured", func(m *corpus.PageFrontMatter) { m.Pictured = true }, 1},
		{"an earlier ocr reading", func(m *corpus.PageFrontMatter) { m.Method = corpus.MethodOCR }, 0},
	} {
		t.Run(c.name, func(t *testing.T) {
			w := newWorld(t, 1)
			meta := corpus.PageFrontMatter{Book: "alg-iv-vii", PDFPage: 1, Method: corpus.MethodNative}
			c.set(&meta)
			path := corpus.PagePath(w.root, "alg-iv-vii", 1)
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := (corpus.PageFile{Meta: meta, Body: "old\n"}).Write(path); err != nil {
				t.Fatal(err)
			}

			runner := w.runner(t, newFleet(func(string) string { return page("A IV.1") }))
			if _, err := runner.Fill(w.pages); err != nil {
				t.Fatal(err)
			}
			report, err := runner.Do(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if report.Kept != c.want {
				t.Errorf("kept %d, want %d", report.Kept, c.want)
			}
		})
	}
}

// byHand writes a page the way a person who repaired one leaves it.
func byHand(t *testing.T, root string, number int, body string) {
	t.Helper()
	path := corpus.PagePath(root, "alg-iv-vii", number)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	file := corpus.PageFile{Meta: corpus.PageFrontMatter{
		Book: "alg-iv-vii", PDFPage: number, Method: corpus.MethodNative, Manual: true,
	}, Body: body}
	if err := file.Write(path); err != nil {
		t.Fatal(err)
	}
}

func TestAPageListReadsAsAPersonWouldWriteIt(t *testing.T) {
	for _, c := range []struct {
		pages []int
		want  string
	}{
		{nil, ""},
		{[]int{89}, "89"},
		{[]int{89, 90}, "89, 90"},
		{[]int{301, 302, 303}, "301 to 303"},
		{[]int{306, 301, 302, 303, 89, 147}, "89, 147, 301 to 303, 306"},
	} {
		if got := pageList(c.pages); got != c.want {
			t.Errorf("pageList(%v) = %q, want %q", c.pages, got, c.want)
		}
	}
}

// Two jobs for one page point at one image file, and the output is matched back
// to the input by name, so a batch holding both is refused whole. Fill will not
// make that pair any more and this makes sure a pair from anywhere else, an old
// queue or a hand written job, costs one page and not the batch.
func TestOnePageIsNotPutInABatchTwice(t *testing.T) {
	w := newWorld(t, 2)
	runner := w.runner(t, newFleet(func(string) string { return page("42") }))
	for _, sha := range []string{"aaaa", "bbbb"} {
		if _, err := w.queue.Add(queue.New(queue.StageOCR, Target("alg-iv-vii", 1), sha, "p")); err != nil {
			t.Fatal(err)
		}
	}

	tasks, err := runner.lease(runner.Hosts[0], 4)
	if err != nil {
		t.Fatal(err)
	}
	if len(tasks) != 1 {
		t.Fatalf("leased %d tasks for one page, want 1", len(tasks))
	}
	stats, err := w.queue.Stats(queue.StageOCR)
	if err != nil {
		t.Fatal(err)
	}
	if stats.Counts[queue.Pending] != 1 || stats.Counts[queue.Leased] != 1 {
		t.Errorf("queue = %+v, want the duplicate back in pending and the other leased", stats.Counts)
	}
}

func mustList(t *testing.T, q *queue.Queue, state queue.State) []queue.Job {
	t.Helper()
	jobs, err := q.List(queue.StageOCR, state)
	if err != nil {
		t.Fatal(err)
	}
	return jobs
}

// The bug this replaced stamped one model name on a whole run. It looked at the
// first host in the list and answered for all of them, so a run holding this
// machine and server2 recorded claude-opus on the pages server2 read, and a run
// holding server2 and this machine recorded gpt-5 on the pages read here. The
// front matter is the only record of who read a page.
func TestTheHostNamesTheModelAndTheRunIsOnlyTheFallback(t *testing.T) {
	runner := &Runner{Model: "fallback"}
	cases := []struct {
		what string
		host Host
		want string
	}{
		{"a box driving a browser names nothing", Host{Name: "server2"}, "fallback"},
		{"a gateway names what it calls", Host{Name: "zen", Model: "grok-4"}, "grok-4"},
		{"a card in the next room names what it serves", Host{Name: "gpc", Model: "olmocr-2-7b"}, "olmocr-2-7b"},
		{"whitespace is not a name", Host{Name: "server3", Model: "  "}, "fallback"},
	}
	for _, c := range cases {
		if got := runner.modelFor(c.host); got != c.want {
			t.Errorf("%s: recorded %q, want %q", c.what, got, c.want)
		}
	}
}

// The head pass in the reader has to be told what rule 4 will judge its answer
// by, or a page can be repaired against one standard and marked against
// another. So the grammar comes from Expect and not from the manifest.
func TestTheRunnerTellsTheReaderWhatTheVolumePrints(t *testing.T) {
	w := newWorld(t, 4)
	machine := newFleet(func(image string) string {
		return page("A IV." + strings.TrimLeft(strings.TrimSuffix(filepath.Base(image), ".png"), "0"))
	})
	runner := w.runner(t, machine)
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}
	if _, err := runner.Do(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(machine.starts) == 0 {
		t.Fatal("no batch was started")
	}
	for _, command := range machine.starts {
		if !strings.HasPrefix(command, "LOCAL_OCR_HEAD_LABEL=1 ") {
			t.Errorf("a head-label volume did not say so:\n%s", command)
		}
		if !strings.Contains(command, "LOCAL_OCR_HEAD_GRAMMAR='head-label' ") {
			t.Errorf("a head-label volume did not name its grammar:\n%s", command)
		}
	}
}

func TestAVolumeWithNoExpectTellsTheReaderNothing(t *testing.T) {
	w := newWorld(t, 4)
	machine := newFleet(func(image string) string {
		return page("A IV." + strings.TrimLeft(strings.TrimSuffix(filepath.Base(image), ".png"), "0"))
	})
	runner := w.runner(t, machine)
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}
	// Fill needs Expect and Do does not, so it is dropped between the two.
	runner.Expect = nil
	if _, err := runner.Do(context.Background()); err != nil {
		t.Fatal(err)
	}
	if len(machine.starts) == 0 {
		t.Fatal("no batch was started")
	}
	for _, command := range machine.starts {
		if strings.Contains(command, "LOCAL_OCR_HEAD_LABEL") {
			t.Errorf("a runner that knows nothing told the reader something:\n%s", command)
		}
		if strings.Contains(command, "LOCAL_OCR_HEAD_GRAMMAR") {
			t.Errorf("a runner that knows nothing named a grammar:\n%s", command)
		}
	}
}

func TestAFootNumberVolumeSaysSo(t *testing.T) {
	w := newWorld(t, 4)
	machine := newFleet(func(image string) string {
		return page("A IV." + strings.TrimLeft(strings.TrimSuffix(filepath.Base(image), ".png"), "0"))
	})
	runner := w.runner(t, machine)
	if _, err := runner.Fill(w.pages); err != nil {
		t.Fatal(err)
	}
	runner.Expect = func(n int) Expect {
		return Expect{Book: "ens-i-iv", PDFPage: n, Grammar: pagemap.FootNumber, HasHead: true}
	}
	if _, err := runner.Do(context.Background()); err != nil {
		t.Fatal(err)
	}
	for _, command := range machine.starts {
		if !strings.HasPrefix(command, "LOCAL_OCR_HEAD_LABEL=0 ") {
			t.Errorf("a foot-number volume did not say so:\n%s", command)
		}
		// The flag reads 0 for foot-number and for head-number both, so the
		// word is the only thing that tells the reader which one this is.
		if !strings.Contains(command, "LOCAL_OCR_HEAD_GRAMMAR='foot-number' ") {
			t.Errorf("a foot-number volume did not name its grammar:\n%s", command)
		}
	}
}
