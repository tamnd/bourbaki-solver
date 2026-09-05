package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/fleet"
	"github.com/tamnd/bourbaki-solver/report"
)

const benchUsage = `usage: bourbaki fleet bench [flags]

Says what concurrency each host should carry, and with -write puts it in the
route file.

Two numbers go into the answer. The ceiling is measured on the box now: cores
that nobody else is using, over the two a lane needs to draw a page through
software. The rate comes out of reports/ocr-usage.jsonl, which every OCR run
appends to, grouped by the lane count each batch actually ran at.

This does not read six pages to find out. Image uploads on these accounts are
rationed by the day and every page spent on a benchmark is a page of Bourbaki
not read, so the measurement is taken from the work rather than instead of it.
A lane count with one batch behind it is not reported as a measurement: one
batch on a shared box measures what the other tenants were doing.

flags:
  -book NAME     only count batches from this book
  -since DUR     only count batches this recent, 24h, 7d
  -min N         batches needed before a rate counts as measured (default 2)
  -write         write the recommendation into the route file
`

func runFleetBench(args []string) error {
	var flags fleetFlags
	fs := flag.NewFlagSet("fleet bench", flag.ExitOnError)
	flags.bind(fs)
	fs.Usage = func() { fmt.Fprint(os.Stderr, benchUsage) }
	book := fs.String("book", "", "only this book")
	since := fs.Duration("since", 0, "only batches this recent")
	minBatches := fs.Int("min", 2, "batches needed before a rate counts")
	write := fs.Bool("write", false, "write the recommendation into the route file")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}

	registry, source, err := flags.registry()
	if err != nil {
		return err
	}
	rates, err := benchRates(*book, *since)
	if err != nil {
		return err
	}

	// The ceiling is what the box can carry now, so it is measured now rather
	// than read out of whatever doctor left behind.
	var targets []fleet.Target
	for _, value := range registry.Enabled() {
		if value.Host == "" {
			continue
		}
		targets = append(targets, fleetTarget(value))
	}
	if len(targets) == 0 {
		return flags.noSSHHost(source, "enabled ")
	}
	ctx, cancel := signalContext()
	defer cancel()
	facts := fleet.ProbeAll(ctx, fleet.SSH{Timeout: 30 * time.Second}, targets)
	byName := map[string]fleet.Facts{}
	for index, target := range targets {
		byName[target.Name] = facts[index]
	}

	fmt.Printf("%-8s  %5s  %5s  %7s  %9s  %8s  %11s\n",
		"host", "cores", "load", "ceiling", "measured", "pages/h", "recommended")
	changed := map[string]int{}
	for _, value := range registry.Enabled() {
		if value.Host == "" {
			continue
		}
		fact := byName[value.Name]
		ceiling := quietLanes(fact)
		// A host that cannot open a browser at all is not a host with a lane
		// count. Its concurrency is HTTP calls, which is a different thing
		// measured a different way, and this command has nothing to say about
		// it. server1 is that host and always will be.
		if ceiling == 0 {
			_, why := fact.CanOCR()
			fmt.Printf("%-8s  %5d  %5.2f  %7s  %9s  %8s  %11s  %s\n",
				value.Name, fact.Cores, float64(fact.LoadX100)/100, "-", "-", "", "-", why)
			continue
		}

		best, ok := report.Best(rates, value.Host, *minBatches)
		measured, rate := "none", ""
		recommend := ceiling
		if ok {
			measured = fmt.Sprintf("%d lanes", best.Lanes)
			rate = fmt.Sprintf("%.1f", best.PagesPerHour())
			recommend = max(1, min(ceiling, best.Lanes))
		}
		fmt.Printf("%-8s  %5d  %5.2f  %7d  %9s  %8s  %11d\n",
			value.Name, fact.Cores, float64(fact.LoadX100)/100, ceiling, measured, rate, recommend)
		if recommend != value.Concurrency {
			changed[value.Name] = recommend
		}
	}

	if len(changed) == 0 {
		fmt.Println("\nthe route file already says what the fleet can carry")
		return nil
	}
	if !*write {
		fmt.Println("\nrun again with -write to put those in the route file")
		return nil
	}
	if source == "built-in" {
		return fmt.Errorf("there is no route file to write, run bourbaki fleet probe first or pass -routes")
	}
	for index := range registry.Routes {
		if value, ok := changed[registry.Routes[index].Name]; ok {
			registry.Routes[index].Concurrency = value
		}
	}
	if err := registry.Write(source); err != nil {
		return err
	}
	fmt.Printf("\nwrote %d route(s) to %s\n", len(changed), source)
	return nil
}

// quietLanes is what the box could carry with nobody else on it.
//
// Facts.Lanes answers a different question, which is what it can carry right
// now, and that is the right number for scheduling a run this minute and the
// wrong one to write into a file. A route file recording that server2 can take
// one lane, because somebody was compiling rust at the moment it was measured,
// would hold the fleet down for a week.
func quietLanes(facts fleet.Facts) int {
	if ok, _ := facts.CanOCR(); !ok {
		return 0
	}
	quiet := facts
	quiet.LoadX100 = 0
	return quiet.Lanes()
}

func benchRates(book string, since time.Duration) ([]report.Rate, error) {
	root, err := corpus.Root()
	if err != nil {
		return nil, err
	}
	file, err := os.Open(filepath.Join(root, "reports", "ocr-usage.jsonl"))
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	defer file.Close()

	lines, bad, err := report.ReadUsage(file)
	if err != nil {
		return nil, err
	}
	if bad > 0 {
		fmt.Fprintf(os.Stderr, "%d line(s) of the usage log did not parse and were skipped\n", bad)
	}
	var from time.Time
	if since > 0 {
		from = time.Now().Add(-since)
	}
	return report.Rates(lines, book, from), nil
}
