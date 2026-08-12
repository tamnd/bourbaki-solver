package main

import (
	"context"
	"fmt"
	"sync"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/ocr"
)

// overHosts runs a list of exercises over the fleet, one exercise to a lane and
// one lane to a host, and counts what each came to.
//
// One lane to a host whatever the host says it can take. A lane here is several
// serial calls to one account over an hour or more, and the lane count the OCR
// path uses is a count of how many images a box can hold at once, which is a
// different question with the same name.
//
// The parallelism is across exercises and not within one. The calls of an
// exercise are in order and cannot be spread, so a lane that dies costs the
// exercise it was on and nothing else.
func overHosts(ctx context.Context, hosts []ocr.Host, work []string,
	do func(context.Context, ocr.Host, string) (string, error),
	logf func(string, ...any)) map[string]int {
	var mu sync.Mutex
	next := 0
	counts := map[string]int{}
	var wg sync.WaitGroup
	for _, host := range hosts {
		wg.Add(1)
		go func(host ocr.Host) {
			defer wg.Done()
			for ctx.Err() == nil {
				mu.Lock()
				if next >= len(work) {
					mu.Unlock()
					return
				}
				label := work[next]
				next++
				mu.Unlock()

				outcome, err := do(ctx, host, label)
				mu.Lock()
				if err != nil {
					logf("%s on %s: %v", label, host.Name, err)
					counts["failed"]++
				} else {
					counts[outcome]++
				}
				mu.Unlock()
			}
		}(host)
	}
	wg.Wait()
	return counts
}

func hostNames(hosts []ocr.Host) []string {
	out := make([]string, len(hosts))
	for i, h := range hosts {
		out[i] = h.Name
	}
	return out
}

// printCounts writes the tally at the foot of a run, statuses in the order the
// corpus lists them and anything else after.
func printCounts(counts map[string]int, extra ...string) {
	fmt.Println()
	for _, key := range append(append([]string{}, corpus.Statuses...), append(extra, "failed")...) {
		if counts[key] > 0 {
			fmt.Printf("%-12s %d\n", key, counts[key])
		}
	}
}
