package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
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

	hosts, err := ocrHosts(*routes, *host)
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
	answer, err := (ocr.Ask{
		Host:     hosts[0],
		Shell:    fleet.SSH{Timeout: 2 * time.Minute},
		Copy:     ocr.Rsync{Timeout: 5 * time.Minute},
		Prompt:   question,
		ID:       "ask-" + time.Now().UTC().Format("20060102-150405"),
		Deadline: *timeout,
		Keep:     *keep,
	}).Do(ctx)
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
