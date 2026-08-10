package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/bourbaki-solver/fleet"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/route"
)

// The three boxes as fleet doctor measured them. server1 is the interesting
// one: it answers model calls all day and cannot open a browser, so it must
// come back with no lanes rather than with one.
var (
	server3 = fleet.Facts{Name: "server3", Cores: 8, MemFreeMB: 15378, Xvfb: true, Rsync: true, Tool: "/root/chatgpt-tool/.venv/bin/chatgpt-tool"}
	server2 = fleet.Facts{Name: "server2", Cores: 6, MemFreeMB: 7363, Xvfb: true, Rsync: true, Tool: "/root/chatgpt-tool/.venv/bin/chatgpt-tool"}
	server1 = fleet.Facts{Name: "server1", Cores: 4, MemFreeMB: 1334, Xvfb: true, Rsync: true, Tool: "/home/tam/chatgpt-tool/.venv/bin/chatgpt-tool"}
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
		// work and returned a blank page every time, so a host in that state has
		// to be refused by name rather than handed four lanes.
		{"a box somebody else is already using", route.Route{Concurrency: 4},
			fleet.Facts{Name: "server3", Cores: 8, LoadX100: 827, MemFreeMB: 15378, Xvfb: true, Rsync: true, Tool: "t"}, 0, "load average 8.3"},
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
