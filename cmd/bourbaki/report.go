package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/report"
)

const reportUsage = `usage: bourbaki report usage [flags]

Reads reports/ocr-usage.jsonl, which every OCR run appends to and nothing ever
replaces, and says what the fleet did: batches, pages sent, pages that came
back, wall clock, and the cost of a page that worked.

The second table is why batches lost pages, counted once per batch. It is read
out of the remote log, so it names the failures the site does not report as
errors: an account with no image uploads left, a slot that is signed out, a
pool that paused itself against an address block.

flags:
  -book NAME     only this book
  -since DUR     only the last 24h, 7d and so on
  -json          print the summary as JSON
`

func runReport(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, reportUsage)
		os.Exit(2)
	}
	switch args[0] {
	case "usage":
		return reportUsageCmd(args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(os.Stderr, reportUsage)
		return nil
	}
	return fmt.Errorf("unknown report %q, try: usage", args[0])
}

func reportUsageCmd(args []string) error {
	fs := flag.NewFlagSet("report usage", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, reportUsage) }
	book := fs.String("book", "", "only this book")
	since := fs.Duration("since", 0, "only batches this recent")
	asJSON := fs.Bool("json", false, "print the summary as JSON")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}

	root, err := corpus.Root()
	if err != nil {
		return err
	}
	path := filepath.Join(root, "reports", "ocr-usage.jsonl")
	file, err := os.Open(path)
	if err != nil {
		return fmt.Errorf("no usage log yet: %w", err)
	}
	defer file.Close()

	lines, bad, err := report.ReadUsage(file)
	if err != nil {
		return err
	}
	if bad > 0 {
		fmt.Fprintf(os.Stderr, "%d line(s) in %s did not parse and were skipped\n", bad, path)
	}

	var from time.Time
	if *since > 0 {
		from = time.Now().Add(-*since)
	}
	summary := report.Summarise(lines, *book, from)

	if *asJSON {
		raw, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	fmt.Print(summary.Table())
	return nil
}
