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
func ocrSetup(book, queueRoot string, flagged bool) (setup, error) {
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
func (s setup) sources(first, last int) []ocr.Source {
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
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" {
		fs.Usage()
		os.Exit(2)
	}
	state, err := ocrSetup(*book, *queueRoot, *flagged)
	if err != nil {
		return err
	}
	runner := &ocr.Runner{Book: state.entry.ID, Root: state.root, Queue: state.queue, Prompt: state.ask}
	sources := state.sources(*first, *last)
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
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" {
		fs.Usage()
		os.Exit(2)
	}
	state, err := ocrSetup(*book, *queueRoot, *flagged)
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

	hosts, err := ocrHostsNow(ctx, *routeFile, *hostList, *wait, logf)
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
		Prompt: state.ask, Model: route.DefaultModel,
		Hosts: hosts, Shell: fleet.SSH{Timeout: 2 * time.Minute}, Copy: ocr.Rsync{Timeout: 30 * time.Minute},
		Batch: *batch, Limit: *limit, Keep: *keep,
		Expect: state.expect, RetryDPI: render.RetryDPI,
		Rerender: rerender(state),
		Logf:     logf,
	}
	// A page that fails on a delimiter is asked about in its own thread before
	// it is sent back to the queue for another full reading. The queue is the
	// fallback, not the first move: a follow up costs one turn and a re-read
	// costs a page.
	if !*noRepair {
		runner.Repair = mender(state.root, hosts, state.expect, logf)
	}

	added, err := runner.Fill(state.sources(*first, *last))
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

// ocrHosts turns the route file and what fleet doctor found into the boxes that
// can read pages.
//
// The lane count is not the route's concurrency. That number is model calls
// over HTTP, which cost a socket; a lane here is a Chrome profile under Xvfb,
// which costs a gigabyte. A host that has the memory for the first and not the
// second gets zero lanes and is skipped, which is exactly server1.
func ocrHosts(routeFile, names string) ([]ocr.Host, error) {
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

	var out []ocr.Host
	var refused []string
	for _, value := range registry.Enabled() {
		if strings.TrimSpace(value.Host) == "" {
			refused = append(refused, value.Name+": no ssh host in "+path)
			continue
		}
		tool, ok := state.Tool(value.Name)
		if !ok {
			refused = append(refused, value.Name+": no chatgpt-tool path, run bourbaki doctor")
			continue
		}
		lanes, why := ocrLanes(value, state.Hosts[value.Name])
		if lanes <= 0 {
			refused = append(refused, value.Name+": "+why)
			continue
		}
		out = append(out, ocr.Host{Name: value.Host, Tool: tool, Lanes: lanes})
	}
	for _, line := range refused {
		fmt.Fprintf(os.Stderr, "skipping %s\n", line)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no host can read page images, run bourbaki doctor")
	}
	return out, nil
}

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
		state.Hosts[target.Host] = rows[index]
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
	switch {
	case !facts.Xvfb:
		return 0, "no xvfb-run, it cannot open a browser"
	case !facts.Rsync:
		return 0, "no rsync, the page images cannot get there"
	case facts.MemFreeMB > 0 && facts.MemFreeMB < ocrLaneMemoryMB:
		return 0, fmt.Sprintf("%d MB free, one lane needs %d", facts.MemFreeMB, ocrLaneMemoryMB)
	// Cores is the marker of a box that was measured, and it is used here
	// rather than a nonzero disk figure because zero free disk is the case
	// this check exists for. server2 read 0 and kept its lane.
	case facts.Cores > 0 && facts.DiskFreeMB < fleet.OCRDiskMB:
		return 0, fmt.Sprintf("%d MB free on disk, one lane needs %d", facts.DiskFreeMB, fleet.OCRDiskMB)
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
