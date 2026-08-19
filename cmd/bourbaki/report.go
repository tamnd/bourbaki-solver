package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/quality"
	"github.com/tamnd/bourbaki-solver/report"
)

const reportUsage = `usage: bourbaki report usage [flags]

Reads the two run logs, which every run appends to and nothing ever replaces,
and says what the fleet did.

reports/ocr-usage.jsonl is the pages: batches, pages sent, pages that came back,
wall clock, and the cost of a page that worked. The table under it is why
batches lost pages, counted once per batch. It is read out of the remote log, so
it names the failures the site does not report as errors: an account with no
image uploads left, a slot that is signed out, a pool that paused itself against
an address block.

reports/ask-usage.jsonl is everything else that asks: translating a §, solving
an exercise, putting the glossary to a model. The unit there is one question,
because that is what those stages have, and the failures are counted with the
answers. A question that was refused leaves nothing in the archive, so this is
the only place a run that asked four hundred and got three hundred can be told
from a run that asked three hundred.

flags:
  -book NAME     only this book, for the pages
  -stage NAME    only this stage, for the questions, as translate or translate vi
  -since DUR     only the last 24h, 7d and so on
  -json          print the summary as JSON
`

const reportHelp = `usage: bourbaki report <what>

  usage        what the fleet did: pages, questions, wall clock, and what failed
  coverage     what the corpus holds against what the table of contents says
  printings    where the two printings of a chapter disagree
  translation  what each language holds, what is stale, and which terms it misses
  solutions    the scorecard: what has an answer, and how well it is believed

Run bourbaki report <what> -h for the flags.
`

func runReport(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, reportHelp)
		os.Exit(2)
	}
	switch args[0] {
	case "usage":
		return reportUsageCmd(args[1:])
	case "coverage":
		return reportCoverageCmd(args[1:])
	case "printings":
		return reportPrintingsCmd(args[1:])
	case "translation", "translations":
		return reportTranslationCmd(args[1:])
	case "solutions", "solution":
		return reportSolutionsCmd(args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(os.Stderr, reportHelp)
		return nil
	}
	return fmt.Errorf("unknown report %q, try: usage, coverage, printings, translation, solutions", args[0])
}

const translationUsage = `usage: bourbaki report translation [flags]

What each target language holds against the English: how many sections and how
many exercises are translated, how many of those were made from an English that
has changed since, and how closely the glossary is followed.

The coverage is of what the corpus holds and not of the Éléments. A language at
100 per cent has every English file there is today.

-terms is the other half, and the one worth running before a translation pass:
the glossary term by term, worst first, with the files that miss each one. A
term missed in one file is a sentence somebody wrote differently, and a term
missed in thirty is a bad row in the glossary. The first run of this check found
a bad row: respect was pinned as the verb, and all 33 uses in the corpus are
"with respect to", which is a preposition.

flags:
  -lang CODE     only this language, vi zh or ja
  -terms         the per term adherence report rather than the coverage table
  -all           with -terms, every term and not only the ones something missed
  -json          print the report as JSON
`

func reportTranslationCmd(args []string) error {
	fs := flag.NewFlagSet("report translation", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, translationUsage) }
	lang := fs.String("lang", "", "only this language")
	terms := fs.Bool("terms", false, "the per term adherence report")
	all := fs.Bool("all", false, "every term, not only the ones something missed")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	c, err := quality.Load(quality.Options{Root: root})
	if err != nil {
		return err
	}
	g, err := glossary.Load(root)
	if err != nil {
		return err
	}

	if *terms {
		if *lang == "" {
			return fmt.Errorf("report translation -terms wants a -lang: the terms are per language")
		}
		rows := report.Terms(c, g, *lang, report.TermOptions{All: *all})
		if *asJSON {
			return printJSON(rows)
		}
		fmt.Print(report.TermTable(*lang, rows))
		return nil
	}

	out := report.Translations(c, g)
	if *lang != "" {
		var only []*report.Translation
		for _, t := range out {
			if t.Lang == *lang {
				only = append(only, t)
			}
		}
		out = only
	}
	if len(out) == 0 {
		// Nothing translated and nothing said would read as a clean report.
		fmt.Println("nothing is translated yet")
		return nil
	}
	if *asJSON {
		return printJSON(out)
	}
	for i, t := range out {
		if i > 0 {
			fmt.Println()
		}
		fmt.Print(t.Table())
	}
	return nil
}

func printJSON(v any) error {
	raw, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	fmt.Println(string(raw))
	return nil
}

const printingsUsage = `usage: bourbaki report printings [flags]

Sets the two printings of a chapter against each other and says where they
disagree: one row per section whose statement or exercise count is not the same
in both. Chapters neither printing has assembled are left out, and so are the
ones where the two agree throughout, unless -all is given.

A chapter is what the two are paired on, since the volumes do not pair up:
Integration I to VI is one book in English and four in French. Sections are
matched on their label, which both printings share, and not on their file name,
which is in the language of the printing.

A row here is a place to look and not a verdict. Most of them are one printing
being read wrongly, and the fix is in extraction or assembly. Some of them are
real: the 2023 English Algebra VIII prints a twentieth exercise in section 2
that the 2012 French edition does not.

flags:
  -book NAME     only this book, alg or top or int
  -chapter NUM   only this chapter, VIII
  -left LANG     the printing on the left, en by default
  -right LANG    the printing on the right, fr by default
  -all           every section, not only the ones that disagree
  -json          print the comparison as JSON
  -write         write the comparison to reports/printings.md in the corpus
`

func reportPrintingsCmd(args []string) error {
	fs := flag.NewFlagSet("report printings", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, printingsUsage) }
	book := fs.String("book", "", "only this book")
	chapter := fs.String("chapter", "", "only this chapter")
	left := fs.String("left", "en", "the printing on the left")
	right := fs.String("right", "fr", "the printing on the right")
	all := fs.Bool("all", false, "every section, not only the ones that disagree")
	asJSON := fs.Bool("json", false, "print the comparison as JSON")
	write := fs.Bool("write", false, "write the comparison to reports/printings.md")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return err
	}
	sections, err := corpus.LoadSections(root)
	if err != nil {
		return err
	}
	pairs := report.Pairs(books, sections, *left, *right)
	var want []report.Pair
	for _, p := range pairs {
		if *book != "" && p.Book != *book {
			continue
		}
		if *chapter != "" && !strings.EqualFold(p.Chapter, *chapter) {
			continue
		}
		want = append(want, p)
	}
	if len(want) == 0 {
		// Saying nothing would read as agreement, which is the one thing this
		// report must never be mistaken for.
		fmt.Printf("no chapter is assembled in both %s and %s yet\n", *left, *right)
		return nil
	}
	var out []*report.Printings
	for _, p := range want {
		c, err := report.Compare(sections, p)
		if err != nil {
			return err
		}
		out = append(out, c)
	}
	if *asJSON {
		raw, err := json.MarshalIndent(out, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	if *write {
		// The comparison goes into the corpus and not into the solver, because
		// it is a fact about the books and it changes whenever a volume is read
		// again. Committing it is what makes a section that used to disagree
		// and now agrees show up as a line of a diff.
		path := filepath.Join(root, "reports", "printings.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(printingsDoc(out)), 0o644); err != nil {
			return err
		}
		fmt.Printf("report printings: wrote %s\n", path)
		return nil
	}
	for _, c := range out {
		if c.Disagreements() == 0 && !*all {
			fmt.Println(c.Line())
			continue
		}
		fmt.Print("\n" + c.Table(*all))
	}
	return nil
}

// printingsDoc is the whole comparison as one page of Markdown, every chapter
// in full, since a file nobody has to run a command to read is worth more than
// the few lines the terminal saves.
func printingsDoc(out []*report.Printings) string {
	var b strings.Builder
	b.WriteString("# The two printings against each other\n\n")
	b.WriteString("Generated by bourbaki report printings -write. Do not edit.\n\n")
	b.WriteString("A chapter of the Éléments printed twice is one chapter, so the two printings should hold the same statements and the same exercises section by section. Where they do not, either the printings differ or one of them is being read wrongly, and this table is where to look. The counts are what the corpus holds, not what anybody claims the book holds.\n\n")
	for _, c := range out {
		b.WriteString(c.Table(true))
		b.WriteString("\n")
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
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
	stage := fs.String("stage", "", "only this stage")
	since := fs.Duration("since", 0, "only batches this recent")
	asJSON := fs.Bool("json", false, "print the summary as JSON")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}

	root, err := corpus.Root()
	if err != nil {
		return err
	}
	var from time.Time
	if *since > 0 {
		from = time.Now().Add(-*since)
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
	summary := report.Summarise(lines, *book, from)

	asked, err := readAsks(root, *stage, from)
	if err != nil {
		return err
	}

	if *asJSON {
		raw, err := json.MarshalIndent(struct {
			Pages     report.Summary `json:"pages"`
			Questions report.Asked   `json:"questions"`
		}{summary, asked}, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	fmt.Print(summary.Table())
	// The ask log arrived after the OCR one and a corpus that predates it has
	// none. That is a corpus with nothing to say here, not an error.
	if asked.Total.Asks > 0 {
		fmt.Println()
		fmt.Print(asked.Table())
	}
	return nil
}

// readAsks reads reports/ask-usage.jsonl, and treats a missing file as an empty
// one.
func readAsks(root, stage string, from time.Time) (report.Asked, error) {
	path := filepath.Join(root, "reports", "ask-usage.jsonl")
	file, err := os.Open(path)
	if errors.Is(err, os.ErrNotExist) {
		return report.Asked{}, nil
	}
	if err != nil {
		return report.Asked{}, err
	}
	defer file.Close()

	asks, bad, err := report.ReadAsks(file)
	if err != nil {
		return report.Asked{}, err
	}
	if bad > 0 {
		fmt.Fprintf(os.Stderr, "%d line(s) in %s did not parse and were skipped\n", bad, path)
	}
	return report.SummariseAsks(asks, stage, from), nil
}
