package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/quality"
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
	case "coverage":
		return reportCoverageCmd(args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(os.Stderr, reportUsage)
		return nil
	}
	return fmt.Errorf("unknown report %q, try: usage, coverage", args[0])
}

const coverageUsage = `usage: bourbaki report coverage [-write-readme]

Says what the corpus holds against what the table of contents says it should:
one row per chapter of every volume, whether or not anything has been read in
yet. A chapter with no row would be a chapter nobody notices is missing, which
is why the empty ones are listed too.

The README carries this table between two markers, and H01 to H06 check that it
is the one the corpus has. -write-readme is what puts it there.

flags:
  -write-readme  write the table into README.md between its markers
`

func reportCoverageCmd(args []string) error {
	fs := flag.NewFlagSet("report coverage", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, coverageUsage) }
	write := fs.Bool("write-readme", false, "write the table into README.md")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	// The coverage table is a fact about the corpus and not about the rules, so
	// it loads the corpus and runs nothing. -skip on every group would be the
	// same thing said less clearly.
	c, err := quality.Load(quality.Options{Root: root})
	if err != nil {
		return err
	}
	block := quality.Coverage(c)
	if !*write {
		fmt.Print(block)
		return nil
	}
	changed, err := quality.WriteCoverage(root, block)
	if err != nil {
		return err
	}
	if !changed {
		fmt.Println("report coverage: README.md is already the table the corpus has")
		return nil
	}
	fmt.Println("report coverage: wrote the table into README.md")
	return nil
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
