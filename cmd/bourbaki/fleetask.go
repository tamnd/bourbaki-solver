package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tamnd/bourbaki-solver/fleet"
	"github.com/tamnd/bourbaki-solver/ocr"
)

const fleetAskUsage = `usage: bourbaki fleet ask [flags] [question]

Puts one question to one host and prints what came back, with the model that
answered it and how long it took.

This is the only honest check that a host will answer. fleet status reads the
route file and the finished jobs, and fleet probe -deep speaks the HTTP
completions API, which is a different transport from the one the work goes
over: a fleet that fails the deep probe answers ask perfectly well, and a
fleet whose accounts have been moved down to a cut down model passes both.

What it came to is added to the record of asks, so that a host checked by hand
counts towards the taking column of bourbaki fleet accounts like any other ask.

It is also how a limit gets measured rather than guessed. -fill pads the
question to a given length with filler that says it is filler, which is how the
character ceiling on a question was found after the pilot lost three calls to
it.

flags:
  -host    which host, by name. Required.
  -file    read the question from a file
  -fill    pad the question out to this many characters
  -routes  the route file naming the hosts
  -timeout give up after this long, 20m by default
  -keep    leave the scratch directory on the host
  -quiet   print the answer and nothing else
`

func runFleetAsk(args []string) error {
	fs := flag.NewFlagSet("fleet ask", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fleetAskUsage) }
	host := fs.String("host", "", "which host")
	file := fs.String("file", "", "read the question from a file")
	fill := fs.Int("fill", 0, "pad the question out to this many characters")
	routes := fs.String("routes", "", "route file")
	timeout := fs.Duration("timeout", 20*time.Minute, "give up after this long")
	keep := fs.Bool("keep", false, "leave the scratch on the host")
	quiet := fs.Bool("quiet", false, "print the answer and nothing else")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if *host == "" {
		return errors.New("say which host with -host")
	}

	question := strings.Join(rest, " ")
	if *file != "" {
		raw, err := os.ReadFile(*file)
		if err != nil {
			return err
		}
		question = string(raw)
	}
	if strings.TrimSpace(question) == "" {
		return errors.New("there is no question to ask")
	}
	if *fill > 0 {
		question = padTo(question, *fill)
	}

	hosts, err := askHosts(*routes, *host)
	if err != nil {
		return err
	}
	if len(hosts) != 1 {
		return fmt.Errorf("-host names %d hosts, and this asks one", len(hosts))
	}

	ctx, cancel := signalContext()
	defer cancel()
	if !*quiet {
		fmt.Fprintf(os.Stderr, "asking %s, %d characters\n", hosts[0].Name, len(question))
	}
	id := "ask-" + time.Now().UTC().Format("20060102-150405")
	// The branch is written out here rather than left to NewAsk because of
	// -timeout. A box is polled by this process and takes its deadline as an
	// argument; a gateway holds one HTTP call and takes its deadline from the
	// route, which is where a gateway's timeout belongs. Passing the flag to
	// something that cannot honour it would be worse than saying so.
	var call ocr.Asker
	if hosts[0].Client != nil {
		if *timeout > 0 {
			fmt.Fprintf(os.Stderr, "-timeout does not reach %s, a gateway waits as long as its route says\n", hosts[0].Name)
		}
		call = ocr.Gateway{Name: hosts[0].Name, Client: hosts[0].Client, Model: hosts[0].Model,
			Prompt: question, ID: id}
	} else {
		call = ocr.Ask{
			Host:     hosts[0],
			Shell:    fleet.SSH{Timeout: 2 * time.Minute},
			Copy:     ocr.Rsync{Timeout: 5 * time.Minute},
			Prompt:   question,
			ID:       id,
			Deadline: *timeout,
			Keep:     *keep,
		}
	}
	answer, err := call.Do(ctx)
	noteAsk(hosts[0].Name, err)
	if err != nil {
		return err
	}
	fmt.Print(strings.TrimRight(answer.Text, "\n") + "\n")
	if !*quiet {
		fmt.Fprintf(os.Stderr, "\n%s answered on %s in %s\n%s\n", hosts[0].Name,
			answer.Model, answer.Elapsed.Round(time.Second), answer.Conversation)
	}
	return nil
}

// noteAsk writes this ask into the record fleet accounts reads back.
//
// One command wrote that record, bourbaki translate, so the taking column spoke
// for the translate driver and for nothing else on the fleet. The usage above
// calls fleet ask the only honest check that a host will answer, and its verdict
// went to the terminal and nowhere else; solve kept ask-usage.jsonl, which is
// this run's questions in full and is nobody's idea of a live board.
//
// That column exists to catch a host whose profiles count as ready and will not
// take a prompt, and it failed to catch one. It read "8/10, first one back now"
// for a host at the same minute that four asks to that host in a row died with
// "ChatGPT never accepted the prompt", one of them a question thirty characters
// long. Every one of those four was this command, so none of them were in the
// record, and the last entries that were came from a translate run that had
// already finished. The board was reporting yesterday's driver.
//
// A failure to write is printed and not returned. The answer is the point of
// the command and the reader is standing at the terminal looking at it; losing
// the bookkeeping is not a reason to exit non-zero on an ask that worked.
//
// It is said once. This is called from the solve lanes as well as from the one
// ask, and a record file that cannot be opened cannot be opened for all of the
// thousands of questions a run makes: the second copy of that line onwards is
// noise over the log somebody needs to read.
func noteAsk(host string, err error) {
	led := fleet.NewLedger()
	led.Note(host, fleet.Classify(err))
	// Append re-reads and rewrites, so two lanes landing together would
	// otherwise each write what they read and the later one would drop the
	// other's ask.
	askRecord.Lock()
	defer askRecord.Unlock()
	if err := led.Append(fleet.LedgerPath()); err != nil {
		askRecordBad.Do(func() {
			fmt.Fprintf(os.Stderr, "asks could not be added to the record: %v\n", err)
		})
	}
}

var (
	askRecord    sync.Mutex
	askRecordBad sync.Once
)

// padTo pads a question out to a length with filler that says what it is.
//
// The filler is prose and not a repeated character, because a service that
// refuses a long question may be refusing a long question and may be refusing
// one that looks like an attack, and the two are told apart by padding that
// reads like the thing being measured.
func padTo(question string, n int) string {
	const filler = "This paragraph is padding, and it carries no question. It is " +
		"here so that the message reaches a set length, which is being measured. " +
		"Ignore it entirely and answer what is asked above. "
	if len(question) >= n {
		return question
	}
	var b strings.Builder
	b.WriteString(question)
	b.WriteString("\n\n")
	for b.Len() < n {
		b.WriteString(filler)
	}
	return b.String()[:n]
}
