package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/fleet"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/prompt"
	"github.com/tamnd/bourbaki-solver/queue"
	"github.com/tamnd/bourbaki-solver/render"
	"github.com/tamnd/bourbaki-solver/route"
)

// ocrLaneMemoryMB is how much free memory a host needs per OCR lane.
//
// It is a floor and not a measurement. A lane sampled per process on server2
// peaked at 275 MB, so memory is not what runs out here, the CPU is. What this
// number is for is refusing a box that has nothing left at all: server1 sat at
// 736 MB free the day this was written, which will start a browser and then be
// killed by the OOM reaper halfway through a batch.
const ocrLaneMemoryMB = 1500

// ocrFleetRecheck is how long to wait before asking a busy fleet again.
//
// Long enough that three ssh round trips are noise against it, short enough
// that a box which frees up is picked up in the same coffee break.
const ocrFleetRecheck = 2 * time.Minute

// setup is everything the two OCR commands both need.
type setup struct {
	root     string
	entry    *corpus.Book
	pmap     *pagemap.Map
	manifest render.Manifest
	queue    *queue.Queue
	// only, when set, is the pages a run may touch whatever else is rendered.
	// It is how the flagged pages of a born-digital volume are read without the
	// other four hundred and forty nine going anywhere near a model.
	only map[int]bool
	// ask is the prompt the pages of this run are read with, which is not the
	// same question for a photograph of a scan and for a flagged page of a
	// volume that already has a reading. See prompt.OCRNative.
	ask string
}

// ocrSetup finds the book, its render manifest and the queue.
//
// flagged is the door for a born-digital volume. Algebra VIII extracts from its
// text layer and has no business here, except for the pages the text layer
// could not carry: a commutative diagram is not in the text layer at all. Those
// pages are named in the extraction report, rendered by render -flagged, and
// read here, and nothing else of that volume is.
//
// contents is the door for the pages of a table of contents, which are asked a
// different question. See prompt.Contents.
func ocrSetup(book, queueRoot string, flagged, contents bool) (setup, error) {
	var out setup
	root, err := corpus.Root()
	if err != nil {
		return out, err
	}
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return out, err
	}
	entry, ok := books.Get(book)
	if !ok {
		return out, fmt.Errorf("no book %q in %s", book, corpus.BooksPath(root))
	}
	if entry.Extraction != "ocr" && !flagged {
		return out, fmt.Errorf("%s is %s and extracts by %s: use bourbaki extract, or -flagged for the pages it could not read",
			entry.ID, entry.Nature, entry.Extraction)
	}
	out.ask = prompt.OCRFor(entry.ID)
	if contents {
		out.ask = prompt.Contents()
	}
	var only map[int]bool
	if flagged {
		out.ask = prompt.OCRNative()
		pages, err := flaggedPages(root, entry.ID)
		if err != nil {
			return out, err
		}
		only = map[int]bool{}
		for _, page := range pages {
			only[page] = true
		}
	}
	manifest, err := render.ReadManifest(root, entry.ID)
	if err != nil {
		return out, fmt.Errorf("%s has not been rendered: %w", entry.ID, err)
	}
	q, err := queue.Open(queueRoot)
	if err != nil {
		return out, err
	}
	pmap, mapErr := pagemap.Load(root, entry.ID)
	if mapErr != nil {
		fmt.Fprintf(os.Stderr, "no page map for %s, so the running head and page label rules are skipped: %v\n", entry.ID, mapErr)
	}
	// out and not a fresh literal. It was written as one and the prompt field
	// added later was set on out and dropped on the way back, so every page
	// went to the fleet with an empty prompt until a run said "no prompt".
	out.root, out.entry, out.pmap = root, entry, pmap
	out.manifest, out.queue, out.only = manifest, q, only
	return out, nil
}

// contentsRange refuses a contents run that names no pages.
//
// The contents is a handful of pages and the prompt that reads it is wrong for
// every other page of the volume: it asks for plain text with the leaders and
// the page numbers kept, and a page of prose read that way loses its headings
// and its mathematics. Without a range the run would put four hundred pages of
// body text through it, at a page a minute, and overwrite the readings already
// on disk. So the range is required rather than defaulted.
func contentsRange(contents bool, first, last int) error {
	if !contents {
		return nil
	}
	if first <= 0 || last <= 0 {
		return fmt.Errorf("-contents reads a table of contents and needs -f and -l: " +
			"the prompt is wrong for a page of the body, and the default range is the whole volume")
	}
	if last < first {
		return fmt.Errorf("-contents: page %d comes before page %d", last, first)
	}
	return nil
}

func (s setup) expect(page int) ocr.Expect {
	return expectFor(s.entry, s.pmap, s.manifest, page)
}

// expectFor is what the pipeline already knows about a page, which is what the
// rules compare a model's answer against.
//
// run and check have to build this the same way or check would be measuring
// something other than the decision run makes, which is the one thing it is for.
func expectFor(entry *corpus.Book, pmap *pagemap.Map, manifest render.Manifest, page int) ocr.Expect {
	value := ocr.Expect{Book: entry.ID, PDFPage: page, Grammar: pagemap.Grammar(entry.Grammar)}
	if found, ok := manifest.Find(page); ok {
		value.Blank = found.Blank
		value.Sparse = found.Ink < ocr.SparseInk
	}
	if pmap != nil {
		if found, ok := pmap.Lookup(page); ok {
			value.Chapter, value.Page = found.Chapter, found.Page
			value.Confidence = found.Confidence
			// A page whose number was read off its own running head has one by
			// definition. Anywhere else the page map cannot say, and asking for
			// a head that a chapter opener does not print would fail one page
			// per chapter.
			value.HasHead = found.Confidence == pagemap.FromHead
		}
	}
	return value
}

// sources is the pages of a volume that a run may read, bounded by the range
// asked for.
//
// unread drops every page that already has a reading committed, whatever it was
// read with. Without it a page comes back into the work list when the prompt
// moves, which is right when the prompt moved because the model was misreading
// something, and wrong when the pages have since been read against their images
// by hand: the corrections are overwritten by a fresh answer, and the uploads
// that paid for it were the ones the unread pages were waiting for.
func (s setup) sources(first, last int, unread bool) []ocr.Source {
	var out []ocr.Source
	for _, page := range s.manifest.Pages {
		if s.only != nil && !s.only[page.Page] {
			continue
		}
		if first > 0 && page.Page < first {
			continue
		}
		if last > 0 && page.Page > last {
			continue
		}
		if unread && committed(s.root, s.entry.ID, page.Page) {
			continue
		}
		out = append(out, ocr.Source{Page: page.Page, SHA256: page.SHA256, Blank: page.Blank})
	}
	return out
}

func ocrFlags(fs *flag.FlagSet) (book, hosts, routes, queueRoot *string, first, last, batch, limit *int, keep, dry *bool) {
	book = fs.String("book", "", "book id")
	first = fs.Int("f", 0, "first pdf page")
	last = fs.Int("l", 0, "last pdf page")
	batch = fs.Int("batch", ocr.DefaultBatch, "pages per batch")
	limit = fs.Int("limit", 0, "stop after this many pages")
	hosts = fs.String("hosts", "", "comma separated route names")
	routes = fs.String("routes", "", "route file")
	queueRoot = fs.String("queue", defaultQueueRoot(), "queue directory")
	keep = fs.Bool("keep", false, "leave the page images on the hosts")
	dry = fs.Bool("dry", false, "say what would be read and stop")
	fs.Usage = func() { fmt.Fprint(os.Stderr, ocrUsage) }
	return
}

func ocrFill(args []string) error {
	fs := flag.NewFlagSet("ocr fill", flag.ExitOnError)
	book, _, _, queueRoot, first, last, _, _, _, _ := ocrFlags(fs)
	flagged := fs.Bool("flagged", false, "only the pages a native extraction could not read")
	contents := fs.Bool("contents", false, "read the pages as a table of contents")
	unread := fs.Bool("unread", false, "only the pages with no reading committed")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" {
		fs.Usage()
		os.Exit(2)
	}
	if err := contentsRange(*contents, *first, *last); err != nil {
		return err
	}
	state, err := ocrSetup(*book, *queueRoot, *flagged, *contents)
	if err != nil {
		return err
	}
	runner := &ocr.Runner{Book: state.entry.ID, Root: state.root, Queue: state.queue, Prompt: state.ask}
	sources := state.sources(*first, *last, *unread)
	added, err := runner.Fill(sources)
	if err != nil {
		return err
	}
	var blank int
	for _, source := range sources {
		if source.Blank {
			blank++
		}
	}
	fmt.Printf("%s: %d pages in range, %d blank, %d queued\n", state.entry.ID, len(sources), blank, added)
	if added == 0 {
		fmt.Println("nothing new to read, every page in range is queued or already accepted at this prompt")
	}
	return nil
}

func ocrRun(args []string) error {
	fs := flag.NewFlagSet("ocr run", flag.ExitOnError)
	book, hostList, routeFile, queueRoot, first, last, batch, limit, keep, dry := ocrFlags(fs)
	// On by default, and this is the way to turn it off. A run that is
	// measuring how well the model reads a page wants the raw rate, not the
	// rate after the pages that nearly worked were mended.
	noRepair := fs.Bool("no-repair", false, "reject a failed page instead of asking about it in its own thread")
	// Half the cores is a rule of thumb, not a measurement of this page. A
	// browser rendering chatgpt.com in software wants more than one core and
	// less than two, and where it falls between them depends on the box. The
	// number is worth overriding by somebody who has watched a run and read the
	// load average, which is cheaper than making the probe guess.
	lanes := fs.Int("lanes", 0, "override how many pages a host reads at once")
	// These boxes are shared and the load moves. Rather than refuse a run
	// because somebody else's build is on the machine right now, sit and ask
	// again. Zero keeps the old behaviour of failing straight away.
	wait := fs.Duration("wait", 0, "how long to wait for a host with a spare core before giving up")
	// The same door as render -flagged, and it has to be asked for twice on
	// purpose: rendering the pages of a born-digital volume is cheap and local,
	// reading them costs rationed uploads.
	flagged := fs.Bool("flagged", false, "only the pages a native extraction could not read")
	// The contents of a volume is three to eight pages and is asked a different
	// question from the rest of it, so it is asked for by name and with a range.
	contents := fs.Bool("contents", false, "read the pages as a table of contents")
	// A run against a starved pool spends its uploads on the pages nothing has
	// read yet, and a re-read is then something asked for rather than something
	// a prompt edit causes.
	unread := fs.Bool("unread", false, "only the pages with no reading committed")
	// A reading by claude or gpt-5 stands even when the prompt or the render
	// moved under it, because the alternative is a weaker reader writing over
	// it, which has now happened twice. This is how to say that the old
	// readings are the thing being replaced on purpose: after a prompt change
	// made because they were wrong, or after a deliberate re-render at a higher
	// resolution. It is spelled out rather than implied by a re-render, because
	// a re-render happens by accident whenever the images directory is swept.
	reread := fs.Bool("reread-protected", false, "read over readings by a stronger model too")
	salvage := fs.Bool("salvage", false, "on the last attempt, write a page whose only faults a fix pass can put right")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" {
		fs.Usage()
		os.Exit(2)
	}
	if err := contentsRange(*contents, *first, *last); err != nil {
		return err
	}
	state, err := ocrSetup(*book, *queueRoot, *flagged, *contents)
	if err != nil {
		return err
	}

	start := time.Now()
	logf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "["+time.Since(start).Round(time.Second).String()+"] "+format+"\n", args...)
	}

	// Ctrl-C stops the run without losing the pages that are in flight. The
	// leases expire on their own and the next run reaps them, which is the
	// whole point of putting the work list on disk. It is set up before the
	// fleet is measured so that a wait can be interrupted too.
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var hosts []ocr.Host
	if hereOnly(*hostList) {
		hosts, err = localHosts(*lanes)
	} else {
		hosts, err = ocrHostsNow(ctx, *routeFile, *hostList, *wait, logf)
	}
	if err != nil {
		return err
	}
	if *lanes > 0 {
		for i := range hosts {
			hosts[i].Lanes = *lanes
		}
	}

	runner := &ocr.Runner{
		Book: state.entry.ID, Root: state.root, Queue: state.queue,
		Prompt: state.ask, Model: runModel(hosts),
		Hosts: hosts,
		Shell: ocr.LocalShell{Remote: fleet.SSH{Timeout: 2 * time.Minute}},
		Copy:  ocr.LocalCopy{Remote: ocr.Rsync{Timeout: 30 * time.Minute}},
		Batch: *batch, Limit: *limit, Keep: *keep,
		First: *first, Last: *last, RereadProtected: *reread, Salvage: *salvage,
		Expect: state.expect, RetryDPI: render.RetryDPI,
		Rerender: rerender(state),
		Logf:     logf,
	}
	// A page that fails on a delimiter is asked about in its own thread before
	// it is sent back to the queue for another full reading. The queue is the
	// fallback, not the first move: a follow up costs one turn and a re-read
	// costs a page.
	//
	// Not here. A page read on this machine is in no conversation to go back
	// to, and a re-read is fifteen seconds, so the queue is the whole repair.
	if !*noRepair && !hereOnly(*hostList) {
		runner.Repair = mender(state.root, hosts, state.expect, logf)
	}

	added, err := runner.Fill(state.sources(*first, *last, *unread))
	if err != nil {
		return err
	}
	stats, err := state.queue.Stats(queue.StageOCR)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %d pages queued now, %d pending in all\n", state.entry.ID, added, stats.Counts[queue.Pending])
	for _, host := range hosts {
		fmt.Printf("  %-8s %d lanes  %s\n", host.Name, host.Lanes, host.Tool)
	}
	if *dry {
		return nil
	}
	if stats.Counts[queue.Pending] == 0 {
		fmt.Println("nothing to do")
		return nil
	}

	report, runErr := runner.Do(ctx)
	fmt.Print(report.Summary())
	if err := writeOCRReport(state.root, report); err != nil {
		fmt.Fprintf(os.Stderr, "could not write the report: %v\n", err)
	}
	if runErr != nil {
		return runErr
	}
	if report.Dead > 0 {
		fmt.Printf("%d pages are dead after %d attempts, see bourbaki queue list -stage ocr -state dead\n",
			report.Dead, queue.DefaultMaxAttempts)
	}
	return nil
}

// rerender is what escalates a page to 600 dpi before a second attempt, and
// reports what it managed: a scan that holds 260 dpi is re-rendered at 260.
func rerender(state setup) func(context.Context, int, int) (int, error) {
	return func(ctx context.Context, page, dpi int) (int, error) {
		manifest, err := render.Render(ctx, render.Options{
			Book: state.entry.ID, PDF: filepath.Join(state.root, state.entry.PDF), Corpus: state.root,
			DPI: dpi, SourceDPI: sourceDPI(state.entry), Gray: true,
			First: page, Last: page, Batch: 1, Overwrite: true,
		})
		if err != nil {
			return 0, err
		}
		return manifest.DPI, nil
	}
}

// hereOnly says whether -hosts named this machine and nothing else.
//
// Nothing else, on purpose. A run that mixed the two would write one model name
// into the front matter of pages two different models read, and the front
// matter is where a page says what read it. Two runs cost one more command and
// keep that honest, and the queue is shared anyway, so they can be started
// together against the same volume and neither will read the other's pages.
func hereOnly(names string) bool {
	fields := strings.Split(strings.TrimSpace(names), ",")
	if len(fields) != 1 {
		return false
	}
	return strings.TrimSpace(fields[0]) == ocr.LocalHost
}

// localHosts is this machine as a host, which needs no route file and no probe.
//
// The tool is this binary. The batch protocol starts <tool> ocr-batch on the
// host, and the local half of that is a subcommand here, so the reader a run
// starts is the build the run itself came from rather than whatever is on PATH
// under that name.
func localHosts(lanes int) ([]ocr.Host, error) {
	self, err := os.Executable()
	if err != nil {
		return nil, fmt.Errorf("cannot find this binary, which is what reads the pages here: %w", err)
	}
	if lanes <= 0 {
		lanes = localLanes
	}
	return []ocr.Host{{
		Name: ocr.LocalHost, Tool: self, Lanes: lanes,
		RateDelay: localRateDelay, Model: localModelName,
	}}, nil
}

// localLanes is how many pages this machine reads at once by default.
//
// Six. Each lane is one CLI process waiting on a network call, so this is not
// bounded by the cores here; it is bounded at the other end, and six pages in
// thirty six seconds is what the pilot measured. The number is worth raising
// when the account is fresh and lowering when it is not, which is what -lanes
// is for.
const localLanes = 6

// localRateDelay is the gap between starting one page and the next. Small, but
// not nothing: six processes started in the same instant all open a connection
// in the same instant.
const localRateDelay = 1.0

// runModel is what a page of this run says read it when its host names nothing.
//
// It used to answer for the whole run by looking at hosts[0], which was wrong
// in both directions and got worse the moment the fleet stopped being three
// boxes running the same thing. A run holding this machine and server2 stamped
// claude-opus on the pages server2 read; a run holding server2 and this machine
// stamped gpt-5 on the pages read here. The front matter is the only record of
// who read a page, so a wrong answer there is not cosmetic, and it was about to
// stamp gpt-5 on every page a local card reads. The host answers for itself now
// and this is only the fallback. See ocr.Runner.modelFor.
func runModel(hosts []ocr.Host) string {
	for _, host := range hosts {
		if !host.Local() {
			return route.DefaultModel
		}
	}
	return localModelName
}

// localModelName is what a page read here records in its front matter. The CLI
// is asked for opus and resolves it to whichever Opus is current, so the name
// kept here is the family and not a build.
const localModelName = "claude-opus"

// ocrHosts turns the route file and what fleet doctor found into the boxes that
// can read pages.
//
// The lane count is not the route's concurrency. That number is model calls
// over HTTP, which cost a socket; a lane here is a Chrome profile under Xvfb,
// which costs a gigabyte. A host that has the memory for the first and not the
// second gets zero lanes and is skipped, which is exactly server1.
func ocrHosts(routeFile, names string) ([]ocr.Host, error) {
	return boxes(routeFile, names, ocrLanes, "read page images", false)
}

// laneRule is how many lanes a box gets for one kind of work, and when the
// answer is none, why. ocrLanes and askLanes are the two.
type laneRule func(route.Route, fleet.Facts) (int, string)

// boxes is the body ocrHosts and askHosts share.
//
// work goes into the refusals and the error, since "no host can read page
// images" is the wrong thing to print at somebody whose translation went
// nowhere. taken says the caller has already picked up the gateways and the
// commands for itself, which askHosts has, and stops this refusing routes that
// are about to be used.
func boxes(routeFile, names string, lanesFor laneRule, work string, taken bool) ([]ocr.Host, error) {
	registry, path, err := route.LoadOrDefault(routeFile)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(names) != "" {
		registry, err = registry.Select(strings.Split(names, ","))
		if err != nil {
			return nil, err
		}
	}
	state, err := fleet.LoadState(fleet.StatePath())
	if err != nil {
		return nil, err
	}

	asked := strings.TrimSpace(names) != ""

	var out []ocr.Host
	var refused []string
	for _, value := range registry.Enabled() {
		if value.Command != "" {
			// A command on this machine has no box either, and the same
			// reasoning applies: it is only worth a line when somebody asked
			// for it by name and is waiting to hear why it is not being used.
			if asked && !taken {
				refused = append(refused, value.Name+": a command on this machine cannot "+work)
			}
			continue
		}
		if value.Gateway {
			// A gateway answers HTTP and has no box to run chatgpt-tool on, so
			// it was never a candidate to read a page. Saying so on every run
			// of every volume would be a line of stderr forever about a route
			// behaving exactly as it is meant to. It is said when it was asked
			// for by name, because then somebody is waiting to hear why.
			if asked && !taken {
				refused = append(refused, value.Name+": a gateway cannot "+work+", it has no box to run chatgpt-tool on")
			}
			continue
		}
		if strings.TrimSpace(value.Host) == "" {
			refused = append(refused, value.Name+": no ssh host in "+path)
			continue
		}
		tool, ok := state.Tool(value.Name)
		if !ok {
			refused = append(refused, value.Name+": no chatgpt-tool path, run bourbaki doctor")
			continue
		}
		lanes, why := lanesFor(value, state.Hosts[value.Name])
		if lanes <= 0 {
			refused = append(refused, value.Name+": "+why)
			continue
		}
		// Reader and Model are carried only when the route names them, which
		// today means only gamingpc. A box driving a browser names neither, and
		// stamping the route's model on one would put gpt-5 in the front matter
		// of a page whose host drew a different slug out of the pool that
		// morning. modelFor falls back to the run's default for exactly that
		// case, and this is what gives it something better to fall back from.
		host := ocr.Host{Name: value.Host, Tool: tool, Lanes: lanes}
		if reader := strings.TrimSpace(value.Reader); reader != "" {
			host.Reader = reader
			host.Model = value.Model
		}
		out = append(out, host)
	}
	for _, line := range refused {
		fmt.Fprintf(os.Stderr, "skipping %s\n", line)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no host can %s, run bourbaki doctor", work)
	}
	return out, nil
}

// askHosts is ocrHosts for the commands that ask a question rather than read a
// page: translate, solve, glossary translate and fleet ask.
//
// The difference is the gateway. A gateway cannot read a page image and is
// rightly missing from ocrHosts, and it can answer every one of these, which
// are text going out and text coming back. It is also the cheap way to answer
// them: a question put to the fleet spends an upload on an account that runs
// out of them for hours, and one put here spends nothing.
//
// A box still needs its chatgpt-tool and its lanes, so those checks are not
// repeated here: boxes does them under askLanes, which differs from ocrLanes
// only in what a thrashing box is worth, and this adds the gateways to what it
// found. When the fleet is spent, or when a run names the
// gateway on the command line, that leaves a list with only gateways in it,
// which is the point.
func askHosts(routeFile, names string) ([]ocr.Host, error) {
	registry, _, err := route.LoadOrDefault(routeFile)
	if err != nil {
		return nil, err
	}
	if strings.TrimSpace(names) != "" {
		if registry, err = registry.Select(strings.Split(names, ",")); err != nil {
			return nil, err
		}
	}

	var out []ocr.Host
	var anyBox bool
	for _, value := range registry.Enabled() {
		if value.Command != "" {
			lanes := value.Concurrency
			if lanes <= 0 {
				lanes = 1
			}
			out = append(out, ocr.Host{Name: value.Name, Lanes: lanes,
				Command: value.Command, Model: value.Model})
			continue
		}
		if !value.Gateway {
			anyBox = true
			continue
		}
		client, err := value.Client(0, gatewayRetries)
		if err != nil {
			fmt.Fprintf(os.Stderr, "skipping %s: %v\n", value.Name, err)
			continue
		}
		lanes := value.Concurrency
		if lanes <= 0 {
			lanes = 1
		}
		out = append(out, ocr.Host{Name: value.Name, Lanes: lanes, Client: client, Model: value.Model})
	}
	if !anyBox {
		if len(out) == 0 {
			return nil, fmt.Errorf("no route can answer a question, run bourbaki doctor")
		}
		return out, nil
	}
	// A fleet that cannot be reached is not fatal here when a gateway can be:
	// the question gets answered, which is what the caller asked for, and the
	// reason the fleet was skipped has already been printed.
	hosts, err := boxes(routeFile, names, askLanes, "answer a question", true)
	if err != nil {
		if len(out) == 0 {
			return nil, err
		}
		return out, nil
	}
	return append(hosts, out...), nil
}

// gatewayRetries is retries inside one call. A free endpoint answers 429 when
// its window is full, which passes on its own, and failing the whole question
// over it would put the work back on the fleet and spend an upload on it.
const gatewayRetries = 4

// refreshFleet re-measures the boxes named in the route file before a run.
//
// Load average is the one fact in there with a short shelf life. server2 read
// 0.55 one evening and 7.67 an hour later, compiling somebody else's rust, so a
// run that schedules against the state file as it was left by the last doctor
// is scheduling against a number that has already moved. Three ssh trips in
// parallel cost about a second, which is nothing against a batch.
func refreshFleet(ctx context.Context, routeFile, names string) error {
	registry, _, err := route.LoadOrDefault(routeFile)
	if err != nil {
		return err
	}
	if strings.TrimSpace(names) != "" {
		if registry, err = registry.Select(strings.Split(names, ",")); err != nil {
			return err
		}
	}
	var targets []fleet.Target
	for _, value := range registry.Enabled() {
		if strings.TrimSpace(value.Host) == "" {
			continue
		}
		targets = append(targets, fleet.Target{Name: value.Name, Host: value.Host, Port: value.RemotePort})
	}
	if len(targets) == 0 {
		return nil
	}

	rows := fleet.ProbeAll(ctx, fleet.SSH{Timeout: 30 * time.Second}, targets)
	path := fleet.StatePath()
	state, err := fleet.LoadState(path)
	if err != nil {
		return err
	}
	for index, target := range targets {
		// A host that did not answer keeps the facts it had. One refused ssh
		// connection is not a reason to forget where chatgpt-tool lives.
		if rows[index].Err != "" && rows[index].Hostname == "" {
			continue
		}
		state.Hosts[target.Name] = rows[index]
	}
	state.Written = time.Now().UTC()
	return state.Save(path)
}

// ocrHostsNow measures the fleet and then picks the hosts that can read pages,
// waiting for one if the whole fleet is busy and the caller said to wait.
//
// Waiting is the useful behaviour on rented boxes. The fleet had no spare core
// at all one evening, all of it other tenants, and the honest choices are to
// refuse the run or to sit until a core comes back. A run left overnight should
// sit.
func ocrHostsNow(ctx context.Context, routeFile, names string, wait time.Duration, logf func(string, ...any)) ([]ocr.Host, error) {
	return hostsWithin(ctx, wait, logf, sleepFor, func() ([]ocr.Host, error) {
		if err := refreshFleet(ctx, routeFile, names); err != nil {
			logf("could not re-measure the fleet, going on what the state file says: %v", err)
		}
		return ocrHosts(routeFile, names)
	})
}

// askHostsNow is ocrHostsNow for the commands that ask a question, and it waits
// the same way for the same reason.
//
// The wait is shorter in practice than it looks, because a gateway is up or it
// is not and no amount of sitting frees a core on it. What the loop is really
// for is the fleet: when both boxes are busy the run goes out on the gateway
// alone, and the next call round finds the boxes back.
func askHostsNow(ctx context.Context, routeFile, names string, wait time.Duration, logf func(string, ...any)) ([]ocr.Host, error) {
	return hostsWithin(ctx, wait, logf, sleepFor, func() ([]ocr.Host, error) {
		if err := refreshFleet(ctx, routeFile, names); err != nil {
			logf("could not re-measure the fleet, going on what the state file says: %v", err)
		}
		return askHosts(routeFile, names)
	})
}

// sleepFor is the wait a run really does, interruptible by Ctrl-C.
func sleepFor(ctx context.Context, pause time.Duration) error {
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-time.After(pause):
		return nil
	}
}

// hostsWithin is the loop on its own, with the measuring and the sleeping
// handed in, so a test can run it without an ssh key or a two minute pause.
func hostsWithin(ctx context.Context, wait time.Duration, logf func(string, ...any),
	sleep func(context.Context, time.Duration) error, pick func() ([]ocr.Host, error)) ([]ocr.Host, error) {
	deadline := time.Now().Add(wait)
	for {
		hosts, err := pick()
		if err == nil {
			return hosts, nil
		}
		left := time.Until(deadline)
		if wait <= 0 || left <= 0 {
			return nil, err
		}
		// Never sleep past the deadline the caller gave. A -wait of thirty
		// seconds should come back in thirty seconds, not in two minutes.
		pause := min(ocrFleetRecheck, left)
		logf("%v, asking again in %s, giving up at %s",
			err, pause.Round(time.Second), deadline.Format(time.Kitchen))
		if err := sleep(ctx, pause); err != nil {
			return nil, err
		}
	}
}

// ocrLanes is how many page images a host reads at once, and when the answer is
// none, why.
//
// A route's concurrency is model calls over HTTP, which cost a socket. A lane
// here is a Chrome profile under Xvfb, which costs a core and a half. A box can
// have the room for the first and not the second, and server1 does.
//
// Memory is not what runs out. A lane measured on server2 peaks at 275 MB, and
// the box sat at 10 GB free through a page that took six minutes. What runs out
// is CPU: the browser draws through swiftshader, because these boxes have no
// GPU, and four of them on six cores took the load average to ten and left every
// page blank. So the ceiling here is Facts.Lanes, which counts cores, and the
// memory floor below is kept only to refuse a box that has nothing left at all.
func ocrLanes(value route.Route, facts fleet.Facts) (int, string) {
	if ok, why := laneFloor(facts); !ok {
		return 0, why
	}
	lanes := value.Concurrency
	if lanes <= 0 {
		lanes = 1
	}
	// Never more than the box itself says it can carry, whatever the route
	// file asks for. The route file is written by hand and the facts are
	// measured, so when they disagree the measurement wins.
	//
	// Cores is the marker of a box that was actually measured. A route with no
	// facts behind it is taken at its word, because refusing a host on the
	// strength of a struct nobody filled in helps nobody.
	if facts.Cores > 0 {
		capacity := facts.Lanes()
		if capacity <= 0 {
			return 0, fmt.Sprintf("load average %.1f across %d cores, thrashing, not slow",
				float64(facts.LoadX100)/100, facts.Cores)
		}
		lanes = min(lanes, capacity)
	}
	return lanes, ""
}

// laneFloor is what a box has to have before how many lanes it can carry is
// worth asking, and it is the same for a page and for a question. What differs
// between the two is only what a busy box is worth, which is below.
func laneFloor(facts fleet.Facts) (bool, string) {
	switch {
	case !facts.Xvfb:
		return false, "no xvfb-run, it cannot open a browser"
	case !facts.Rsync:
		return false, "no rsync, the files cannot get there"
	case facts.MemFreeMB > 0 && facts.MemFreeMB < ocrLaneMemoryMB:
		return false, fmt.Sprintf("%d MB free, one lane needs %d", facts.MemFreeMB, ocrLaneMemoryMB)
	// Cores is the marker of a box that was measured, and it is used here
	// rather than a nonzero disk figure because zero free disk is the case
	// this check exists for. server2 read 0 and kept its lane.
	case facts.Cores > 0 && facts.DiskFreeMB < fleet.OCRDiskMB:
		return false, fmt.Sprintf("%d MB free on disk, one lane needs %d", facts.DiskFreeMB, fleet.OCRDiskMB)
	}
	return true, ""
}

// askLanes is ocrLanes for a question, which is text going out and text coming
// back rather than a page image going up.
//
// Everything the box needs it still needs. The question is rsynced over as a
// file, so rsync is not optional here either, and it is still a headed Chrome
// on chatgpt.com under Xvfb, so the memory and the disk are the same. The one
// thing that differs is what a thrashing box is worth, and the answer measured
// rather than assumed is: one lane.
//
// server3 sat at a load average between 49 and 59 on eight cores for a day,
// somebody else's build, six times past the mark where ocrLanes gives up. In
// that state it answered three questions in a row, in 1m48s, 2m25s and 4m10s,
// and healed thirteen profiles, each of which is a browser launched and a page
// loaded. So the box was slow and it was not stuck. On the same box in the same
// hour a page image batch failed, and it failed on a Cloudflare interstitial on
// the address, which is a thing the load has nothing to do with.
//
// The asymmetry is the work. Reading a page is a large upload and a long
// generation, and it comes back blank when the browser cannot get the CPU to
// finish rendering, which is where ocrLanes was written from and is still right
// about. A question is a paste and a wait. What it costs on a crowded box is
// minutes, and minutes are worth having when the alternative is a translate
// queue with 2299 pending and every route refusing it.
//
// One and not facts.Lanes(): a thrashing box gets a lane, not a share of a
// machine it does not have. When the box is not thrashing this is ocrLanes
// exactly, so nothing changes for a fleet that is behaving.
func askLanes(value route.Route, facts fleet.Facts) (int, string) {
	lanes, why := ocrLanes(value, facts)
	if lanes > 0 {
		return lanes, why
	}
	// Only the load is overturned, and the floor is asked again rather than
	// told apart from the sentence ocrLanes wrote, so a box that is both
	// thrashing and out of memory is still refused and still refused for the
	// memory. Reading the reason back out of the message is how the two drift
	// apart the first time one of them is reworded.
	if ok, cannot := laneFloor(facts); !ok {
		return 0, cannot
	}
	if facts.Thrashing() {
		return 1, ""
	}
	return lanes, why
}

// writeOCRReport publishes what the run cost.
//
// The pages are scratch and the images are scratch, but what it took to read
// them is the claim the milestone is judged on, so it is committed. The usage
// file is appended to rather than replaced, because the interesting number is
// how the fleet behaved over a week and not during the last run.
//
// A report with no book is one from a run cancelled before it read the volume
// out of the queue, and it has nothing in it to publish. It used to be written
// to reports/ocr-.json, one file that every such run overwrote.
func writeOCRReport(root string, report ocr.Report) error {
	if report.Book == "" {
		return nil
	}
	directory := filepath.Join(root, "reports")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	path := filepath.Join(directory, "ocr-"+report.Book+".json")
	if err := os.WriteFile(path+".tmp", append(raw, '\n'), 0o644); err != nil {
		return err
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		return err
	}

	usage, err := os.OpenFile(filepath.Join(directory, "ocr-usage.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer usage.Close()
	for _, value := range report.Batches {
		line, err := json.Marshal(struct {
			Book string `json:"book"`
			When string `json:"when"`
			ocr.Result
		}{report.Book, report.Started.Format(time.RFC3339), value})
		if err != nil {
			return err
		}
		if _, err := usage.Write(append(line, '\n')); err != nil {
			return err
		}
	}
	return nil
}

// committed says whether a page of this volume already has a reading in the
// corpus. It asks the file system and not the queue: what a re-read would
// overwrite is the file, and the file is the thing worth not overwriting.
func committed(root, book string, page int) bool {
	_, err := os.Stat(corpus.PagePath(root, book, page))
	return err == nil
}
