package main

import (
	"fmt"
	"os"
	"slices"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/bourbaki-solver/queue"
	"github.com/tamnd/bourbaki-solver/route"
)

// What the boxes actually answer on, as opposed to what they offer.
//
// The model column of the status board is the model the route file asks for,
// checked against the catalogue the host advertises. Both of those describe what
// is on offer. Neither can see an account being moved down at answer time, and
// that is not a hypothetical: a whole section of chapter VIII came back on a cut
// down model with the board reading gpt-5 the entire time, and nobody knew until
// the audit ran over the finished file.
//
// A deep probe would settle it, but the deep probe goes over HTTP and the fleet
// answers over ssh through chatgpt-tool, so it does not reach the same door.
// What does reach it is the work already done: every finished job records the
// host that answered and the model it answered on. This reads that back. It
// costs no model calls, it cannot be wrong about what happened, and its one
// limit is honest, which is that it knows nothing until some work has run.

// answer is what one host was last seen answering on.
type answer struct {
	Host  string
	Model string
	When  time.Time
	Jobs  int
	// Models is every model this host has been seen answering on, so a route
	// that was moved down partway through a run says so rather than showing
	// whichever call happened to be last.
	Models map[string]int
}

// answers reads the finished jobs and says, per host, what answered.
//
// Only successful events count. A failed attempt records why it failed rather
// than what answered it, and counting those would report a refusal as a model.
func answers(q *queue.Queue) (map[string]*answer, error) {
	out := map[string]*answer{}
	for _, stage := range queue.Stages {
		jobs, err := q.List(stage, queue.Done)
		if err != nil {
			return nil, err
		}
		for _, job := range jobs {
			for _, event := range job.History {
				model := strings.TrimSpace(event.Reason)
				if !event.OK || event.Host == "" || model == "" {
					continue
				}
				row := out[event.Host]
				if row == nil {
					row = &answer{Host: event.Host, Models: map[string]int{}}
					out[event.Host] = row
				}
				row.Jobs++
				row.Models[model]++
				if event.TS.After(row.When) {
					row.When, row.Model = event.TS, model
				}
			}
		}
	}
	return out, nil
}

// printAnswers writes the block under the status board, and writes nothing at
// all when no work has run: a board with an empty column reads like a fact, and
// this has no fact to report until a job has finished.
func printAnswers(root string, results []route.Health) {
	q, err := queue.Open(root)
	if err != nil {
		return
	}
	seen, err := answers(q)
	if err != nil || len(seen) == 0 {
		return
	}

	// The route names come first and in their own order, then anything the queue
	// knows about that the route file does not, since a host that answered work
	// is worth showing whether or not it is still configured.
	names := make([]string, 0, len(seen))
	for _, row := range results {
		if _, ok := seen[row.Route]; ok {
			names = append(names, row.Route)
		}
	}
	rest := make([]string, 0, len(seen))
	for name := range seen {
		if !slices.Contains(names, name) {
			rest = append(rest, name)
		}
	}
	sort.Strings(rest)
	names = append(names, rest...)

	fmt.Println()
	fmt.Printf("%-8s  %-16s  %-25s  %6s  %s\n", "host", "last answered on", "when", "jobs", "seen answering on")
	for _, name := range names {
		row := seen[name]
		kinds := make([]string, 0, len(row.Models))
		for model, count := range row.Models {
			kinds = append(kinds, fmt.Sprintf("%s %d", model, count))
		}
		sort.Strings(kinds)
		fmt.Printf("%-8s  %-16s  %-25s  %6d  %s\n", row.Host, row.Model,
			row.When.Local().Format(time.RFC3339), row.Jobs, strings.Join(kinds, ", "))
	}
	fmt.Fprintf(os.Stderr, "answers as recorded by finished jobs in %s\n", root)
}
