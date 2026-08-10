package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/queue"
)

const queueUsage = `usage: bourbaki queue <command> [flags]

commands:
  stats    count the jobs in every stage, and say how many leases have expired
  reap     return jobs whose worker did not come back
  retry    move failed and dead jobs back to pending with their attempts cleared
  drain    remove pending jobs, leaving the record of what happened
  list     print the jobs in one state, with the reason each one failed

flags:
  -root PATH    queue directory (default $BOURBAKI_WORK/queue, else work/queue
                under the corpus named by $BOURBAKI_CORPUS)
  -stage NAME   ocr, repair, translate, solve or judge (default every stage)

The queue is on disk because at 151 seconds a call nothing finishes in one
process lifetime. It lives under work/, which is not committed: job state is
local and disposable, and the outputs are the durable thing.
`

// defaultQueueRoot is where the work list lives.
//
// It follows the corpus rather than the working directory. It used to be the
// plain relative path work/queue, and that made two queues: one under the
// corpus checkout from the runs started there, one under this repo from the
// runs started here. A page read into the corpus by one of them was still
// pending in the other, and the second queue is where ten pages ended up
// enqueued twice. The queue is state about a corpus, so it belongs beside it.
func defaultQueueRoot() string {
	if value := strings.TrimSpace(os.Getenv("BOURBAKI_WORK")); value != "" {
		return filepath.Join(value, "queue")
	}
	root, err := corpus.Root()
	if err != nil {
		return filepath.Join("work", "queue")
	}
	return filepath.Join(root, "work", "queue")
}

func runQueue(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, queueUsage)
		os.Exit(2)
	}
	command := args[0]
	fs := flag.NewFlagSet("queue "+command, flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, queueUsage) }
	root := fs.String("root", defaultQueueRoot(), "queue directory")
	stageName := fs.String("stage", "", "one stage, default every stage")
	stateName := fs.String("state", string(queue.Dead), "state, for list and retry")
	asJSON := fs.Bool("json", false, "print jobs as JSON, for list")
	if command == "help" || command == "-h" || command == "--help" {
		fmt.Fprint(os.Stderr, queueUsage)
		return nil
	}
	if err := fs.Parse(args[1:]); err != nil {
		return err
	}

	q, err := queue.Open(*root)
	if err != nil {
		return err
	}
	stages := queue.Stages
	if strings.TrimSpace(*stageName) != "" {
		stage, err := queue.ParseStage(*stageName)
		if err != nil {
			return err
		}
		stages = []queue.Stage{stage}
	}

	switch command {
	case "stats":
		rows := make([]queue.Stats, 0, len(stages))
		for _, stage := range stages {
			stats, err := q.Stats(stage)
			if err != nil {
				return err
			}
			rows = append(rows, stats)
		}
		fmt.Print(queue.Table(rows))
		// An expired lease is not work in flight, it is a worker that died.
		// Saying so here saves somebody watching a stuck number for an hour.
		for _, row := range rows {
			if row.Expired > 0 {
				fmt.Fprintf(os.Stderr, "%s: %d leases have expired, run bourbaki queue reap\n", row.Stage, row.Expired)
			}
		}
		return nil

	case "reap":
		for _, stage := range stages {
			reaped, err := q.Reap(stage)
			if err != nil {
				return err
			}
			if len(reaped) > 0 {
				fmt.Printf("%s: returned %d jobs to pending\n", stage, len(reaped))
			}
		}
		return nil

	case "retry":
		state, err := queue.ParseState(*stateName)
		if err != nil {
			return err
		}
		for _, stage := range stages {
			moved, err := q.Retry(stage, state)
			if err != nil {
				return err
			}
			if moved > 0 {
				fmt.Printf("%s: moved %d jobs from %s to pending\n", stage, moved, state)
			}
		}
		return nil

	case "drain":
		for _, stage := range stages {
			removed, err := q.Drain(stage)
			if err != nil {
				return err
			}
			if removed > 0 {
				fmt.Printf("%s: removed %d pending jobs\n", stage, removed)
			}
		}
		return nil

	case "list":
		state, err := queue.ParseState(*stateName)
		if err != nil {
			return err
		}
		for _, stage := range stages {
			jobs, err := q.List(stage, state)
			if err != nil {
				return err
			}
			for _, job := range jobs {
				if *asJSON {
					raw, err := json.Marshal(job)
					if err != nil {
						return err
					}
					fmt.Println(string(raw))
					continue
				}
				fmt.Printf("%s  %-8s  %s  attempts %d\n", job.ID, job.Stage, job.Target, job.Attempts)
				// Every attempt, not just the last. A page that failed on three
				// hosts for three different reasons is a different problem from
				// one that failed the same way three times.
				for _, event := range job.History {
					fmt.Printf("    %s  %-8s  ok=%t  %s\n",
						event.TS.Format("2006-01-02 15:04:05"), event.Host, event.OK, event.Reason)
				}
			}
		}
		return nil

	default:
		return fmt.Errorf("unknown queue command %q", command)
	}
}
