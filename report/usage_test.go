package report

import (
	"strings"
	"testing"
	"time"
)

// Four lines out of the real usage log, shortened. The second is the one worth
// keeping: a batch that lost all four pages, with the tool's own log on it
// naming the bans and the address block that caused it.
const log = `{"book":"alg-i-iii","when":"2026-08-10T14:15:51Z","host":"server3","id":"alg-i-iii-0042-c9d938","pages":4,"wrote":0,"pid":1364594,"elapsed":"40s"}
{"book":"alg-i-iii","when":"2026-08-10T14:17:57Z","host":"server3","id":"alg-i-iii-0042-c9d938","pages":4,"wrote":0,"pid":1364678,"elapsed":"39s","log":"  [ban] Profile slot chatgpt-profile-15 rate-limited until 16:28:11\n  [circuit-breaker] 4+ slots banned in 120s\n  [ip-block] chatgpt.com IP-level block detected"}
{"book":"alg-i-iii","when":"2026-08-10T14:21:11Z","host":"server3","id":"alg-i-iii-0042-c9d938","pages":4,"wrote":4,"pid":1364947,"elapsed":"22m8s"}
{"book":"alg-viii","when":"2026-08-10T15:00:00Z","host":"server2","id":"alg-viii-0001-aa","pages":2,"wrote":1,"pid":1,"elapsed":"10m","log":"No OCR response: nothing came back after 300s"}
`

func read(t *testing.T, text string) []Line {
	t.Helper()
	lines, bad, err := ReadUsage(strings.NewReader(text))
	if err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Fatalf("%d lines did not parse", bad)
	}
	return lines
}

func TestTheUsageLogAddsUpPerHost(t *testing.T) {
	summary := Summarise(read(t, log), "", time.Time{})

	if len(summary.Hosts) != 2 {
		t.Fatalf("hosts = %+v", summary.Hosts)
	}
	server3 := summary.Hosts[1]
	if server3.Name != "server3" {
		t.Fatalf("hosts came back in the wrong order: %+v", summary.Hosts)
	}
	if server3.Batches != 3 || server3.Pages != 12 || server3.Wrote != 4 {
		t.Errorf("server3 = %+v", server3)
	}
	// Two dead batches and one that worked: the page cost is the whole wall
	// clock over the pages that came back, because the failures really were
	// paid for.
	if want := 40*time.Second + 39*time.Second + 22*time.Minute + 8*time.Second; server3.Took != want {
		t.Errorf("Took = %s, want %s", server3.Took, want)
	}
	if want := server3.Took / 4; server3.PerPage() != want {
		t.Errorf("PerPage = %s, want %s", server3.PerPage(), want)
	}
	if summary.Total.Pages != 14 || summary.Total.Wrote != 5 {
		t.Errorf("total = %+v", summary.Total)
	}
}

func TestOneBookAtATime(t *testing.T) {
	summary := Summarise(read(t, log), "alg-viii", time.Time{})
	if len(summary.Hosts) != 1 || summary.Hosts[0].Name != "server2" {
		t.Fatalf("hosts = %+v", summary.Hosts)
	}
	if summary.Total.Batches != 1 {
		t.Errorf("batches = %d, want the one alg-viii line", summary.Total.Batches)
	}
}

func TestAWindowLeavesOutTheOlderRuns(t *testing.T) {
	since := time.Date(2026, 8, 10, 14, 30, 0, 0, time.UTC)
	summary := Summarise(read(t, log), "", since)
	if summary.Total.Batches != 1 || summary.Hosts[0].Name != "server2" {
		t.Fatalf("summary = %+v", summary)
	}
}

// The reasons are read out of the tool's own log, which is the only place the
// silent failures show up at all.
func TestTheReasonsComeOffTheRemoteLog(t *testing.T) {
	for _, c := range []struct {
		name string
		line Line
		want []string
	}{
		{"a quota with nothing else wrong", Line{Pages: 4, Log: "Or wait 17 hours to upload again"},
			[]string{"no uploads left on the account"}},
		{"a signed out slot", Line{Pages: 1, Log: "chatgpt-profile-9 is signed out"},
			[]string{"the slot is signed out"}},
		{"bans and the block behind them", Line{Pages: 4, Log: "[ban] rate-limited\n[circuit-breaker] paused\n[ip-block] IP-level block detected"},
			[]string{"the address is blocked", "the pool paused itself", "a slot was banned"}},
		{"a batch that failed silently", Line{Pages: 2, Log: ""},
			[]string{"no reason in the log"}},
		// A finished batch is not a failure whatever its log says. Slots are
		// banned and rotated on runs that read every page, and counting those
		// would report a working fleet as a broken one.
		{"a batch that finished", Line{Pages: 4, Wrote: 4, Log: "[ban] rate-limited"}, nil},
	} {
		got := Reasons(c.line)
		if len(got) != len(c.want) {
			t.Errorf("%s: reasons = %v, want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: reasons = %v, want %v", c.name, got, c.want)
				break
			}
		}
	}
}

// One malformed line must not hide the rest. This file is appended to by every
// run and a truncated write is a thing that happens.
func TestABadLineIsCountedAndSkipped(t *testing.T) {
	lines, bad, err := ReadUsage(strings.NewReader(log + "{not json\n\n"))
	if err != nil {
		t.Fatal(err)
	}
	if bad != 1 {
		t.Errorf("bad = %d, want 1", bad)
	}
	if len(lines) != 4 {
		t.Errorf("read %d good lines, want 4", len(lines))
	}
}

func TestAnEmptyLogSaysSoRatherThanPrintingAnEmptyTable(t *testing.T) {
	table := Summarise(nil, "", time.Time{}).Table()
	if !strings.Contains(table, "no batches") {
		t.Errorf("table = %q", table)
	}
}

func TestTheTableNamesTheHostsAndTheReasons(t *testing.T) {
	table := Summarise(read(t, log), "", time.Time{}).Table()
	for _, want := range []string{"server2", "server3", "all", "the address is blocked", "no answer on the page"} {
		if !strings.Contains(table, want) {
			t.Errorf("the table does not mention %q:\n%s", want, table)
		}
	}
}

// What a second lane buys is the question the route file's concurrency is an
// answer to, so the rate is grouped by the lane count the batch actually ran
// at rather than averaged over all of them.
const lanelog = `{"book":"alg-i-iii","when":"2026-08-11T01:00:00Z","host":"server2","id":"a","pages":2,"wrote":2,"lanes":1,"elapsed":"20m"}
{"book":"alg-i-iii","when":"2026-08-11T02:00:00Z","host":"server2","id":"b","pages":2,"wrote":2,"lanes":1,"elapsed":"20m"}
{"book":"alg-i-iii","when":"2026-08-11T03:00:00Z","host":"server2","id":"c","pages":4,"wrote":4,"lanes":2,"elapsed":"30m"}
{"book":"alg-i-iii","when":"2026-08-11T04:00:00Z","host":"server2","id":"d","pages":4,"wrote":4,"lanes":2,"elapsed":"30m"}
{"book":"alg-i-iii","when":"2026-08-11T05:00:00Z","host":"server2","id":"e","pages":4,"wrote":4,"lanes":3,"elapsed":"20m"}
{"book":"alg-i-iii","when":"2026-08-11T06:00:00Z","host":"server3","id":"f","pages":2,"wrote":0,"elapsed":"40m"}
`

func TestRatesAreGroupedByLaneCount(t *testing.T) {
	rates := Rates(read(t, lanelog), "", time.Time{})
	if len(rates) != 4 {
		t.Fatalf("rates = %+v", rates)
	}
	// server3 first by name, then server2 by lane count.
	one := rates[0]
	if one.Host != "server2" || one.Lanes != 1 || one.Batches != 2 || one.Wrote != 4 {
		t.Fatalf("one lane = %+v", one)
	}
	if got := one.PagesPerHour(); got != 6 {
		t.Errorf("one lane did %.1f pages an hour, want 6", got)
	}
	two := rates[1]
	if got := two.PagesPerHour(); got != 8 {
		t.Errorf("two lanes did %.1f pages an hour, want 8", got)
	}
}

func TestTheBestLaneCountNeedsMoreThanOneBatchBehindIt(t *testing.T) {
	rates := Rates(read(t, lanelog), "", time.Time{})

	// Three lanes did twelve pages an hour on one batch, which is the fastest
	// number in the log and not a measurement of anything.
	best, ok := Best(rates, "server2", 2)
	if !ok {
		t.Fatal("no rate came back for server2")
	}
	if best.Lanes != 2 {
		t.Errorf("best = %d lanes at %.1f an hour, want the two lanes with two batches behind it",
			best.Lanes, best.PagesPerHour())
	}

	// With the bar at one batch the single fast run wins, which is what that
	// flag is for.
	if best, _ := Best(rates, "server2", 1); best.Lanes != 3 {
		t.Errorf("best at min 1 = %d lanes, want 3", best.Lanes)
	}
}

// Batches from before the lane count was recorded are not one lane. There are
// twenty five of them in the real log and reading them as one lane would set
// the whole fleet's concurrency from a number nobody wrote down.
func TestBatchesWithNoLaneCountAreNotAMeasurement(t *testing.T) {
	rates := Rates(read(t, lanelog), "", time.Time{})
	if _, ok := Best(rates, "server3", 1); ok {
		t.Error("a host whose only batches predate the lane count came back with a recommendation")
	}
}
