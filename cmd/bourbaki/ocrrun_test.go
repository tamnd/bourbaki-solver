package main

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/extract"
	"github.com/tamnd/bourbaki-solver/fleet"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/render"
	"github.com/tamnd/bourbaki-solver/route"
	"gopkg.in/yaml.v3"
)

// The three boxes as fleet doctor measured them. server1 is the interesting
// one: it answers model calls all day and cannot open a browser, so it must
// come back with no lanes rather than with one.
var (
	server3 = fleet.Facts{Name: "server3", Cores: 8, MemFreeMB: 15378, DiskFreeMB: 12024, Xvfb: true, Rsync: true, Tool: "/root/chatgpt-tool/.venv/bin/chatgpt-tool"}
	server2 = fleet.Facts{Name: "server2", Cores: 6, MemFreeMB: 7363, DiskFreeMB: 44256, Xvfb: true, Rsync: true, Tool: "/root/chatgpt-tool/.venv/bin/chatgpt-tool"}
	server1 = fleet.Facts{Name: "server1", Cores: 4, MemFreeMB: 1334, DiskFreeMB: 169547, Xvfb: true, Rsync: true, Tool: "/home/tam/chatgpt-tool/.venv/bin/chatgpt-tool"}
)

func TestLanesFollowWhatTheBoxCanCarry(t *testing.T) {
	cases := []struct {
		name  string
		route route.Route
		facts fleet.Facts
		lanes int
		why   string
	}{
		{"the big box takes what the route asks for", route.Route{Concurrency: 4}, server3, 4, ""},
		{"the middle box too", route.Route{Concurrency: 3}, server2, 3, ""},
		{"the small box reads nothing", route.Route{Concurrency: 1}, server1, 0, "1334 MB free"},
		// Six cores carry three browsers. A route file asking for sixteen is
		// asking for a load average of forty and sixteen blank pages.
		{"the box caps an optimistic route file", route.Route{Concurrency: 16}, server2, 3, ""},
		// These are rented boxes and the cores are not all ours. server3 spent an
		// evening at a load of 8.27 across its eight cores on somebody else's
		// work, and it read pages the whole time, slowly. One lane, not the four
		// the route file asks for.
		{"a box somebody else is already using", route.Route{Concurrency: 4},
			fleet.Facts{Name: "server3", Cores: 8, LoadX100: 827, MemFreeMB: 15378, DiskFreeMB: 12024, Xvfb: true, Rsync: true, Tool: "t"}, 1, ""},
		// server1 on the morning of the eleventh: four cores at 39, running
		// another tenant's Kubernetes and a Harbor registry. Two runnable things
		// per core is crowded, nine is stuck, and a stuck box is refused by name.
		{"a box that is thrashing", route.Route{Concurrency: 4},
			fleet.Facts{Name: "server1", Cores: 4, LoadX100: 3914, MemFreeMB: 15378, DiskFreeMB: 169547, Xvfb: true, Rsync: true, Tool: "t"}, 0, "load average 39.1"},
		// server2 on the morning of the eleventh, with 10 GB of RAM free, a load
		// of 2.31 across six cores, and not one megabyte left on the disk. It
		// kept its lane and took the first chunk of the first Vietnamese
		// section, which came back as a failed mkdir.
		{"a box with a full disk", route.Route{Concurrency: 3},
			fleet.Facts{Name: "server2", Cores: 6, LoadX100: 231, MemFreeMB: 10898, DiskFreeMB: 0, Xvfb: true, Rsync: true, Tool: "t"}, 0, "free on disk"},
		{"no xvfb is no browser", route.Route{Concurrency: 4}, fleet.Facts{MemFreeMB: 16000, Rsync: true}, 0, "xvfb"},
		{"no rsync is no images", route.Route{Concurrency: 4}, fleet.Facts{MemFreeMB: 16000, Xvfb: true}, 0, "rsync"},
		{"an unmeasured box is taken at its word", route.Route{Concurrency: 2}, fleet.Facts{Xvfb: true, Rsync: true}, 2, ""},
		{"a route with no number still reads one page at a time", route.Route{}, server3, 1, ""},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			lanes, why := ocrLanes(test.route, test.facts)
			if lanes != test.lanes {
				t.Fatalf("lanes = %d, want %d (%s)", lanes, test.lanes, why)
			}
			if test.why == "" && why != "" {
				t.Fatalf("a host that reads pages should not come with a reason: %s", why)
			}
			if test.why != "" && !strings.Contains(why, test.why) {
				t.Fatalf("reason = %q, want it to mention %q", why, test.why)
			}
		})
	}
}

// The cap only ever takes lanes away. A box with room for four that is asked
// for three reads three, because the route file is also allowed to be the
// cautious one.
func TestTheCapNeverRaisesTheLaneCount(t *testing.T) {
	lanes, _ := ocrLanes(route.Route{Concurrency: 3}, server3)
	if lanes != 3 {
		t.Fatalf("lanes = %d, want the route file's 3 and not the box's 4", lanes)
	}
}

// A busy fleet is a wait, not a failure, when the caller asked to wait. The
// boxes are shared and the load moves: server2 read 0.55 one evening and 7.67
// an hour later on somebody else's rust build, so a run left overnight should
// sit through that rather than give up on it.
func TestARunCanWaitForABoxToComeFree(t *testing.T) {
	tries := 0
	var slept []time.Duration
	sleep := func(_ context.Context, pause time.Duration) error {
		slept = append(slept, pause)
		return nil
	}
	pick := func() ([]ocr.Host, error) {
		tries++
		if tries < 3 {
			return nil, errors.New("no host can read page images")
		}
		return []ocr.Host{{Name: "server2", Tool: "t", Lanes: 2}}, nil
	}

	hosts, err := hostsWithin(context.Background(), time.Hour, func(string, ...any) {}, sleep, pick)
	if err != nil {
		t.Fatalf("the loop gave up on a fleet that came free: %v", err)
	}
	if len(hosts) != 1 || hosts[0].Name != "server2" {
		t.Fatalf("hosts = %+v", hosts)
	}
	if tries != 3 || len(slept) != 2 {
		t.Fatalf("asked %d times and slept %d times, want 3 and 2", tries, len(slept))
	}
	for _, pause := range slept {
		if pause != ocrFleetRecheck {
			t.Errorf("slept %s between tries, want %s", pause, ocrFleetRecheck)
		}
	}
}

// Without -wait the old behaviour stands: say so at once rather than sit on a
// fleet that is not coming back.
func TestARunWithNoWaitFailsAtOnce(t *testing.T) {
	tries := 0
	pick := func() ([]ocr.Host, error) {
		tries++
		return nil, errors.New("no host can read page images")
	}
	sleep := func(context.Context, time.Duration) error {
		t.Fatal("a run with no wait should not sleep")
		return nil
	}
	if _, err := hostsWithin(context.Background(), 0, func(string, ...any) {}, sleep, pick); err == nil {
		t.Fatal("a busy fleet with no wait should be an error")
	}
	if tries != 1 {
		t.Fatalf("asked %d times, want once", tries)
	}
}

// The pause never runs past the deadline. Asking to wait thirty seconds and
// being kept two minutes is worse than not waiting at all.
func TestTheLastWaitStopsAtTheDeadline(t *testing.T) {
	var slept []time.Duration
	sleep := func(_ context.Context, pause time.Duration) error {
		slept = append(slept, pause)
		// The sleep is what spends the wait, so it has to really spend it.
		time.Sleep(pause)
		return nil
	}
	pick := func() ([]ocr.Host, error) { return nil, errors.New("busy") }

	if _, err := hostsWithin(context.Background(), 30*time.Millisecond, func(string, ...any) {}, sleep, pick); err == nil {
		t.Fatal("a fleet that stays busy past the deadline should be an error")
	}
	if len(slept) != 1 {
		t.Fatalf("slept %d times, want once", len(slept))
	}
	if slept[0] > 30*time.Millisecond {
		t.Errorf("slept %s, which is past the deadline the caller gave", slept[0])
	}
}

// Ctrl-C during a wait stops the run then and there.
func TestAWaitCanBeInterrupted(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	pick := func() ([]ocr.Host, error) { return nil, errors.New("busy") }
	_, err := hostsWithin(ctx, time.Hour, func(string, ...any) {}, sleepFor, pick)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want the cancellation", err)
	}
}

func TestOnlyTheHostsThatCanReadPagesComeBack(t *testing.T) {
	home := t.TempDir()
	routes := filepath.Join(home, "routes.json")
	write(t, routes, `{"routes":[
		{"name":"server3","wire":"chat","base_url":"http://127.0.0.1:18773/v1","model":"gpt-5","host":"server3","concurrency":4},
		{"name":"server2","wire":"chat","base_url":"http://127.0.0.1:18772/v1","model":"gpt-5","host":"server2","concurrency":3},
		{"name":"server1","wire":"chat","base_url":"http://127.0.0.1:18771/v1","model":"gpt-5","host":"server1","concurrency":1}
	]}`)

	state := filepath.Join(home, "fleet.json")
	raw, err := json.Marshal(fleet.State{
		Written: time.Now(),
		Hosts:   map[string]fleet.Facts{"server3": server3, "server2": server2, "server1": server1},
	})
	if err != nil {
		t.Fatal(err)
	}
	write(t, state, string(raw))
	t.Setenv("BOURBAKI_FLEET_STATE", state)

	hosts, err := ocrHosts(routes, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 2 {
		t.Fatalf("got %d hosts, want server3 and server2 only: %+v", len(hosts), hosts)
	}
	want := []ocr.Host{
		{Name: "server3", Tool: server3.Tool, Lanes: 4},
		{Name: "server2", Tool: server2.Tool, Lanes: 3},
	}
	for i, host := range hosts {
		if host != want[i] {
			t.Errorf("host %d = %+v, want %+v", i, host, want[i])
		}
	}

	// The tool path is per host and is not guessable. Sending server1's path to
	// server3 would fail every batch on a box that works.
	if hosts[0].Tool == "" {
		t.Error("a host with no chatgpt-tool path should have been refused, not sent an empty command")
	}
}

func TestARunWithNoUsableHostSaysSo(t *testing.T) {
	home := t.TempDir()
	routes := filepath.Join(home, "routes.json")
	write(t, routes, `{"routes":[{"name":"server1","wire":"chat","base_url":"http://127.0.0.1:18771/v1","model":"gpt-5","host":"server1","concurrency":1}]}`)
	state := filepath.Join(home, "fleet.json")
	raw, _ := json.Marshal(fleet.State{Written: time.Now(), Hosts: map[string]fleet.Facts{"server1": server1}})
	write(t, state, string(raw))
	t.Setenv("BOURBAKI_FLEET_STATE", state)

	if _, err := ocrHosts(routes, ""); err == nil {
		t.Fatal("a fleet where nothing can read pages should be an error, not an empty run that reports success")
	}
}

// A route that fleet doctor has never seen has no tool path, and the path is
// not the same on every box, so guessing one would fail at the first batch.
func TestAHostFleetDoctorHasNotSeenIsRefused(t *testing.T) {
	home := t.TempDir()
	routes := filepath.Join(home, "routes.json")
	write(t, routes, `{"routes":[
		{"name":"server3","wire":"chat","base_url":"http://127.0.0.1:18773/v1","model":"gpt-5","host":"server3","concurrency":4},
		{"name":"server4","wire":"chat","base_url":"http://127.0.0.1:18774/v1","model":"gpt-5","host":"server4","concurrency":4}
	]}`)
	state := filepath.Join(home, "fleet.json")
	raw, _ := json.Marshal(fleet.State{Written: time.Now(), Hosts: map[string]fleet.Facts{"server3": server3}})
	write(t, state, string(raw))
	t.Setenv("BOURBAKI_FLEET_STATE", state)

	hosts, err := ocrHosts(routes, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(hosts) != 1 || hosts[0].Name != "server3" {
		t.Fatalf("got %+v, want server3 alone", hosts)
	}
}

func TestTheReportAndTheUsageLogAreWritten(t *testing.T) {
	root := t.TempDir()
	report := ocr.Report{
		Book: "alg-i", Started: time.Now().Add(-time.Hour), Finished: time.Now(),
		Accepted: 18, Rejected: 2,
		Batches: []ocr.Result{
			{Host: "server3", ID: "alg-i-0001-aa", Pages: 10, Wrote: 10, Elapsed: ocr.Duration(20 * time.Minute)},
			{Host: "server2", ID: "alg-i-0011-bb", Pages: 10, Wrote: 9, Elapsed: ocr.Duration(25 * time.Minute)},
		},
	}
	if err := writeOCRReport(root, report); err != nil {
		t.Fatal(err)
	}

	var read ocr.Report
	decode(t, filepath.Join(root, "reports", "ocr-alg-i.json"), &read)
	if read.Accepted != 18 || len(read.Batches) != 2 {
		t.Fatalf("report came back as %+v", read)
	}

	usage := filepath.Join(root, "reports", "ocr-usage.jsonl")
	if lines := countLines(t, usage); lines != 2 {
		t.Fatalf("usage log has %d lines, want one per batch", lines)
	}
	// A second run appends. The number worth having is how the fleet behaved
	// over a week, not during the last run, so this file is never replaced.
	if err := writeOCRReport(root, report); err != nil {
		t.Fatal(err)
	}
	if lines := countLines(t, usage); lines != 4 {
		t.Fatalf("usage log has %d lines after a second run, want 4: it was replaced rather than appended to", lines)
	}
	if left, _ := filepath.Glob(filepath.Join(root, "reports", "*.tmp")); len(left) != 0 {
		t.Fatalf("a temporary file was left behind: %v", left)
	}

	// Elapsed reads as a duration in the file, not as a pile of nanoseconds
	// that nobody can check against a wall clock.
	body, err := os.ReadFile(usage)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), `"20m0s"`) {
		t.Fatalf("usage line does not carry a readable elapsed time:\n%s", body)
	}
}

func write(t *testing.T, path, body string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(body), 0o600); err != nil {
		t.Fatal(err)
	}
}

func decode(t *testing.T, path string, into any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, into); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
}

func countLines(t *testing.T, path string) int {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	body := strings.TrimSpace(string(raw))
	if body == "" {
		return 0
	}
	return strings.Count(body, "\n") + 1
}

// setupCorpus is a corpus with one book in it, enough for ocrSetup to run
// against without a checkout.
func setupCorpus(t *testing.T, book corpus.Book, manifest render.Manifest, flags []extract.PageFlags) string {
	t.Helper()
	root := t.TempDir()
	for _, dir := range []string{"manifests", "reports"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	raw, err := yaml.Marshal(corpus.BooksManifest{Books: []corpus.Book{book}})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corpus.BooksPath(root), raw, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := render.WriteManifest(root, book.ID, manifest); err != nil {
		t.Fatal(err)
	}
	report, err := json.Marshal(extract.Result{Book: book.ID, Flags: flags})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corpus.ExtractReportPath(root, book.ID), report, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BOURBAKI_CORPUS", root)
	return root
}

// Algebra VIII has a text layer and reading all 505 of its pages through a model
// would spend a day of upload quota on pages that are already perfect. The
// repair pass is the exception, and it is the flagged pages only: the door has
// to open exactly that far and no further.
func TestFlaggedIsTheOnlyWayIntoABornDigitalVolume(t *testing.T) {
	book := corpus.Book{ID: "alg-viii", Nature: "born-digital", Extraction: "native"}
	manifest := render.Manifest{Book: "alg-viii", Pages: []render.Page{
		{Page: 41, SHA256: "a"}, {Page: 76, SHA256: "b"}, {Page: 77, SHA256: "c"}, {Page: 78, SHA256: "d"},
	}}
	flags := []extract.PageFlags{
		{PDFPage: 76, Flags: []string{"diagram"}},
		{PDFPage: 77, Flags: []string{"diagram"}},
	}
	setupCorpus(t, book, manifest, flags)
	queueRoot := filepath.Join(t.TempDir(), "queue")

	if _, err := ocrSetup("alg-viii", queueRoot, false); err == nil {
		t.Fatal("a born-digital volume was let in without -flagged")
	} else if !strings.Contains(err.Error(), "-flagged") {
		t.Errorf("the refusal does not say what to do instead: %v", err)
	}

	s, err := ocrSetup("alg-viii", queueRoot, true)
	if err != nil {
		t.Fatalf("ocrSetup with -flagged: %v", err)
	}
	sources := s.sources(0, 0, false)
	if len(sources) != 2 || sources[0].Page != 76 || sources[1].Page != 77 {
		t.Fatalf("sources = %v, want only the flagged pages 76 and 77", sources)
	}

	// The range still applies on top of the flag list, because a repair pass of
	// forty five pages is still something you want to try ten of first.
	if got := s.sources(77, 0, false); len(got) != 1 || got[0].Page != 77 {
		t.Errorf("sources(77, 0) = %v, want page 77 alone", got)
	}
}

// A scanned volume reads every page it rendered, and nothing about the repair
// door may change that.
func TestWithoutFlaggedAScannedVolumeReadsEveryRenderedPage(t *testing.T) {
	book := corpus.Book{ID: "alg-i-iii", Nature: "scan", Extraction: "ocr"}
	manifest := render.Manifest{Book: "alg-i-iii", Pages: []render.Page{
		{Page: 50, SHA256: "a"}, {Page: 51, SHA256: "b"},
	}}
	setupCorpus(t, book, manifest, nil)

	s, err := ocrSetup("alg-i-iii", filepath.Join(t.TempDir(), "queue"), false)
	if err != nil {
		t.Fatalf("ocrSetup: %v", err)
	}
	if got := s.sources(0, 0, false); len(got) != 2 {
		t.Errorf("sources = %v, want both rendered pages", got)
	}
}

// A page already read is not read again under -unread, and one never read is.
//
// The prompt of Theory of Sets was edited after fifty five of its pages were in
// and the fifty five came straight back into the work list, which is the rule
// working as written: the prompt moved, so the readings were made under rules
// that no longer hold. It was still the wrong thing to do that afternoon. Those
// pages had been read against their images by hand, a re-read would have
// overwritten the corrections with a fresh answer, and the pool had two uploads
// left in it and three hundred and sixty three pages nobody had read at all.
func TestUnreadLeavesThePagesAlreadyReadAlone(t *testing.T) {
	book := corpus.Book{ID: "ens-i-iv", Nature: "scan", Extraction: "ocr"}
	manifest := render.Manifest{Book: "ens-i-iv", Pages: []render.Page{
		{Page: 22, SHA256: "a"}, {Page: 23, SHA256: "b"}, {Page: 24, SHA256: "c"},
	}}
	root := setupCorpus(t, book, manifest, nil)
	writePage(t, root, "ens-i-iv", 23, "Let $E$ be a set.\n")

	s, err := ocrSetup("ens-i-iv", filepath.Join(t.TempDir(), "queue"), false)
	if err != nil {
		t.Fatalf("ocrSetup: %v", err)
	}
	got := s.sources(0, 0, true)
	if len(got) != 2 || got[0].Page != 22 || got[1].Page != 24 {
		t.Errorf("sources = %v, want 22 and 24 and not the page that is read", got)
	}
	if all := s.sources(0, 0, false); len(all) != 3 {
		t.Errorf("without -unread sources = %v, want all three", all)
	}
}
