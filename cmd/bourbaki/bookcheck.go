package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/book"
	"github.com/tamnd/bourbaki-solver/corpus"
)

// bourbaki book check is the whole shelf built at once and written down.
//
// bourbaki book builds one volume in one language and prints an audit, which is
// what somebody wants while they are fixing that volume. It is not what the
// project wants, because the question the milestone actually asks is whether
// the corpus assembles, and that is a question about forty volumes and a
// hundred builds rather than about any one of them. A check that is run by hand
// on the volume somebody happens to be looking at will never find the volume
// nobody is looking at, and the last full sweep found twenty one builds that
// set no file of content/ at all and had been quietly producing four page PDFs.
//
// So this runs the same build over every volume in every language that has
// enough of it, gathers the audits, and writes reports/books.md and
// reports/books.json into the corpus the way the extraction and the translation
// reports are written. It goes in the corpus and not here for the reason those
// two give: it is a fact about the books as they stand, and committing it is
// what turns a volume that lost a chapter into a line of a diff.
const bookCheckUsage = `usage: bourbaki book check [-lang en,fr,vi] [-book id] [-out work/books]
                          [-write] [-json] [-no-pdf] [-no-epub] [-floor 0.10]
                          [-bundle url] [-cached] [-epoch n] [-short 0.10]
                          [-max-overfull 200] [-max-stray 0] [-max-wide 0]

Builds every volume in every language that has enough of it, and writes the
report the milestone asks for.

This is bourbaki book run over the whole shelf. Each build is exactly the one
that command makes, with the same flags and the same audit, so a row of the
report and a run of bourbaki book on that volume are the same numbers.

A language that holds less of the printing than -floor is refused rather than
bound, and the refusal is a row of the report saying how much it holds and what
is missing, rather than a build that quietly did not happen. That is what the
floor is for: it is the one thing in the audit that says there is no book here,
as against the many things that say there is a book here and it is wrong.

  -book           only this volume, for a quick look
  -lang           only these languages, comma separated. Every language under
                  content/ that has anything of the volume by default
  -write          write reports/books.md and reports/books.json in the corpus
  -json           print the rows as JSON instead of the report
  -out            where the builds go, work/books under the corpus by default

Exits 1 if any build failed to run at all. A build that ran and failed checks is
a row of the report and not an error, because that is the backlog and the report
is how it is watched.
`

// A buildRow is one volume in one language, as the report records it. The field
// names are the ones reports/books.json already carries, so that a reader
// comparing this run against the one in the corpus is comparing like with like.
type buildRow struct {
	ID    string `json:"id"`
	Lang  string `json:"lang"`
	Title string `json:"title"`
	// Printed is the language of the printing the pages came out of, which is
	// what a build in any other language is measured against.
	Printed      string `json:"printed"`
	PrintedPages int    `json:"printedPages"`

	// Refused is set instead of everything below it when the language does not
	// hold enough of the printing to be bound. A refused build is a row and not
	// a gap, because a volume that dropped under the floor between two runs is
	// the thing this report exists to make visible.
	Refused string `json:"refused,omitempty"`

	// Broke is why the volume could not be assembled at all, and empty when it
	// could. It is not the same as a failed check and not the same as a refusal:
	// a refusal is this package deciding there is not enough here, and this is
	// the corpus being malformed in a way that stops the writer before it starts.
	//
	// It is a row for the reason the whole sweep exists. The first run of this
	// command over the shelf stopped on its fifth volume, on one unclosed math
	// span in content/vi/alg/IV/exercises/s2/12.md, and reported nothing about
	// the other thirty nine: no report was written, and a hundred builds that
	// were fine were as invisible as the one that was not. A sweep that dies on
	// the first bad file is a sweep that can only be run once the corpus is
	// already clean, which is the opposite of what it is for. So the bad volume
	// becomes a line of the report and the sweep goes on, and the command still
	// exits non zero at the end so that nobody mistakes a report with broken
	// builds in it for a clean one.
	Broke string `json:"broke,omitempty"`

	Sections  int `json:"sections"`   // of the printing, held in this language
	OfPrint   int `json:"ofPrinting"` // as a percentage
	Files     int `json:"files"`
	Exercises int `json:"exercises"`
	Anchors   int `json:"anchors"`
	Formulae  int `json:"formulae"`
	Documents int `json:"documents"`
	Pages     int `json:"pages"`

	// TypesetterFailed is what tectonic said when it stopped, and empty when it
	// ran to the end. It is a field of its own rather than one of the failed
	// checks because it is the failure that hides all the others.
	TypesetterFailed string `json:"typesetterFailed,omitempty"`
	CoverIsRaster    bool   `json:"coverFromPDF"`

	Ran    int      `json:"ran"`
	Passed int      `json:"passed"`
	Failed []string `json:"failed"`
}

func bookCheckCmd(args []string) error {
	fs := flag.NewFlagSet("book check", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, bookCheckUsage) }
	only := fs.String("book", "", "only this volume")
	langs := fs.String("lang", "", "only these languages")
	out := fs.String("out", "", "where the builds go")
	write := fs.Bool("write", false, "write reports/books.md and reports/books.json")
	asJSON := fs.Bool("json", false, "print the rows as JSON")
	noPDF := fs.Bool("no-pdf", false, "write the .tex and skip the typesetter")
	noEPUB := fs.Bool("no-epub", false, "skip the EPUB")
	bundle := fs.String("bundle", os.Getenv("BOURBAKI_TEX_BUNDLE"), "where tectonic fetches its packages")
	cached := fs.Bool("cached", false, "refuse to fetch anything")
	epoch := fs.Int64("epoch", 1735689600, "the timestamp everything is pinned to")
	short := fs.Float64("short", 0.10, "how far under the printing's own text the volume may sit")
	maxOverfull := fs.Int("max-overfull", 200, "the most lines that may run past the measure")
	maxStray := fs.Int("max-stray", 0, "the most TeX control sequences loose in the prose")
	maxWide := fs.Int("max-wide", 0, "the most arrays widened to hold their own rows")
	floor := fs.Float64("floor", 0.10, "how much of the printing a language must hold to be bound at all")
	noCover := fs.Bool("no-cover-check", false, "do not render the first page to look at the cover")
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
	want, err := checkLanguages(root, *langs)
	if err != nil {
		return err
	}
	dir := *out
	if dir == "" {
		dir = filepath.Join(root, "work", "books")
	}
	opt := book.Options{Epoch: *epoch, Bundle: *bundle, Cached: *cached}
	aopt := book.AuditOptions{
		Short: *short, Overfull: *maxOverfull,
		Stray: *maxStray, Wide: *maxWide,
		Floor: *floor,
		Cover: !*noCover && !*noPDF,
	}

	var rows []buildRow
	for _, meta := range books.Books {
		if *only != "" && meta.ID != *only {
			continue
		}
		for _, lang := range want {
			row, ok, err := checkOne(root, dir, meta, lang, opt, aopt, *noPDF, *noEPUB)
			if err != nil {
				// Recorded and carried on with, rather than returned. See
				// buildRow.Broke: one malformed file must not cost the report on the
				// other hundred and thirty builds.
				row.Broke = firstLineOf(err.Error())
				ok = true
			}
			if !ok {
				continue
			}
			// One line a build as it finishes, because the sweep is an hour of
			// tectonic and a run that says nothing for an hour is a run nobody
			// leaves going.
			fmt.Printf("%-20s %-3s %s\n", row.ID, row.Lang, row.line())
			rows = append(rows, row)
		}
	}
	if len(rows) == 0 {
		return fmt.Errorf("book check: nothing to build; no volume matched %q in any of %s",
			*only, strings.Join(want, ", "))
	}
	if *asJSON {
		return printJSON(rows)
	}
	doc := bookReport(rows, aopt)
	if !*write {
		fmt.Print(doc)
		return nil
	}
	if err := os.MkdirAll(filepath.Join(root, "reports"), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(rows, "", " ")
	if err != nil {
		return err
	}
	for _, f := range []struct {
		name string
		body []byte
	}{
		{"books.md", []byte(doc)},
		{"books.json", append(raw, '\n')},
	} {
		path := filepath.Join(root, "reports", f.name)
		if err := os.WriteFile(path, f.body, 0o644); err != nil {
			return err
		}
		fmt.Printf("wrote %s\n", path)
	}
	return broken(rows)
}

// broken is the sweep's exit status. The report is written first and then this
// is returned, because the report is the deliverable and it is wanted most on
// the run that had something wrong with it. What it names is only the volumes
// that could not be assembled at all: a build that came out and failed six
// checks is what this report is for and is not an error, and making it one
// would mean the command exited non zero on every run for the next year.
func broken(rows []buildRow) error {
	var bad []string
	for _, r := range rows {
		if r.Broke != "" {
			bad = append(bad, r.ID+"-"+r.Lang)
		}
	}
	if len(bad) == 0 {
		return nil
	}
	return fmt.Errorf("%d of %d builds could not be assembled: %s",
		len(bad), len(rows), strings.Join(bad, ", "))
}

// checkOne builds one volume in one language. The second return is false for a
// language the volume has nothing of at all, which is not a row: a French
// Vietnamese translation that has never been started is not a fact about the
// corpus, it is the absence of one. A language that has some and not enough is
// a row, and says so.
func checkOne(root, out string, meta corpus.Book, lang string, opt book.Options,
	aopt book.AuditOptions, noPDF, noEPUB bool) (buildRow, bool, error) {
	row := buildRow{ID: meta.ID, Lang: lang, Title: meta.Title,
		Printed: meta.Lang, PrintedPages: meta.Pages}

	v, err := book.Load(root, meta.ID, lang)
	if err != nil {
		// A language with no directory for this volume is nothing to report.
		if os.IsNotExist(err) {
			return row, false, nil
		}
		return row, false, err
	}
	have, wantN, missing, cerr := book.Coverage(root, v)
	if cerr == nil && wantN > 0 {
		row.Sections, row.OfPrint = have, int(100*float64(have)/float64(wantN))
		if have == 0 {
			return row, false, nil
		}
	}
	if err := book.BelowFloor(root, v, aopt.Floor); err != nil {
		row.Refused = fmt.Sprintf("holds %d of the printing's %d sections, under the %.0f%% floor; "+
			"the first of what is missing is %s", have, wantN, 100*aopt.Floor, firstOf(missing))
		return row, true, nil
	}

	a, err := buildOne(root, out, meta.ID, lang, "", opt, aopt, noPDF, noEPUB)
	if err != nil {
		return row, false, err
	}
	row.Ran = len(a.Checks)
	row.Passed = row.Ran - a.Failed()
	for _, c := range a.Checks {
		if !c.OK {
			row.Failed = append(row.Failed, c.Name)
		}
	}
	if a.Doc != nil {
		row.Files, row.Exercises, row.Anchors = a.Doc.Files, a.Doc.Exercises, a.Doc.Anchors
	}
	if a.Build != nil {
		row.Pages, row.TypesetterFailed = a.Build.Pages, firstLineOf(a.Build.Failed)
	}
	if a.EPUB != nil {
		row.Documents, row.Formulae, row.CoverIsRaster = a.EPUB.Documents, a.EPUB.Math, a.EPUB.CoverIsRaster
	}
	return row, true, nil
}

// line is the one line the sweep prints as a build finishes.
func (r buildRow) line() string {
	if r.Broke != "" {
		return "COULD NOT BE ASSEMBLED: " + r.Broke
	}
	if r.Refused != "" {
		return "refused: " + r.Refused
	}
	s := fmt.Sprintf("%3d%% of the printing, %d files, %d pages, %d of %d checks",
		r.OfPrint, r.Files, r.Pages, r.Passed, r.Ran)
	if r.TypesetterFailed != "" {
		s += "  THE TYPESETTER STOPPED: " + r.TypesetterFailed
	}
	return s
}

// checkLanguages is which languages to try. Every directory under content/ by
// default, because a language nobody named on the command line is exactly the
// language whose volumes stop being built.
func checkLanguages(root, named string) ([]string, error) {
	if named != "" {
		var out []string
		for _, l := range strings.Split(named, ",") {
			if l = strings.TrimSpace(l); l != "" {
				out = append(out, l)
			}
		}
		return out, nil
	}
	entries, err := os.ReadDir(filepath.Join(root, "content"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() && !strings.HasPrefix(e.Name(), ".") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

func firstOf(s []string) string {
	if len(s) == 0 {
		return "nothing the sections manifest names"
	}
	return s[0]
}

func firstLineOf(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		return s[:i]
	}
	return s
}

// bookReport is the Markdown the sweep writes.
//
// What it is for is the check counts, not the prose. A reader wants three
// things out of it: how many builds there are, which checks the corpus is
// failing and how often, and which individual builds are in trouble. Anything
// else somebody wants to say about a particular run belongs in the issue that
// run was made for, so this deliberately writes no narrative it cannot compute.
func bookReport(rows []buildRow, aopt book.AuditOptions) string {
	var b strings.Builder
	b.WriteString("# Assembling the books back out of the corpus\n\n")
	b.WriteString("Every volume the manifest names, built back out of `content/` into TeX, a PDF and an EPUB, " +
		"in every language that has enough of it. It is milestone 13, and the point is not that one book builds: " +
		"it is that the corpus as a whole still assembles, because nothing that reads one file at a time can see " +
		"a chapter missing from the middle of a volume or a § that goes from 1 to 3.\n\n")
	b.WriteString("Written by `bourbaki book check -write`. Every number here is one run of the same build " +
		"`bourbaki book` makes, so a row of this table and a run of that command on that volume are the same numbers.\n\n")

	built, refused, broke, noPDF, pages, formulae := 0, 0, 0, 0, 0, 0
	for _, r := range rows {
		if r.Broke != "" {
			broke++
			continue
		}
		if r.Refused != "" {
			refused++
			continue
		}
		built++
		pages += r.Pages
		formulae += r.Formulae
		if r.TypesetterFailed != "" || r.Pages == 0 {
			noPDF++
		}
	}
	vols := map[string]bool{}
	for _, r := range rows {
		vols[r.ID] = true
	}
	b.WriteString("## What it comes to\n\n| | |\n| --- | --- |\n")
	row := func(k string, v any) { fmt.Fprintf(&b, "| %s | %v |\n", k, v) }
	row("volumes", len(vols))
	row("builds", built)
	row("builds that reached a PDF", built-noPDF)
	row("pages set", pages)
	row("formulae set", formulae)
	fmt.Fprintf(&b, "| languages refused under the %.0f%% floor | %d |\n", 100*aopt.Floor, refused)
	row("builds that could not be assembled", broke)
	b.WriteString("\n")

	// This section first, above the checks, because a volume that will not
	// assemble is not a book with something wrong with it: there is no book, and
	// no check ran on it to be counted anywhere else in this report.
	if broke > 0 {
		b.WriteString("## Could not be assembled\n\nThe writer stopped before it produced a document. " +
			"These are faults in `content/`, one file at a time, and each line names the file and the line " +
			"of it: they are fixed in the corpus and not here.\n\n" +
			"| volume | language | what stopped it |\n| --- | --- | --- |\n")
		for _, r := range rows {
			if r.Broke != "" {
				fmt.Fprintf(&b, "| %s | %s | %s |\n", r.ID, r.Lang, r.Broke)
			}
		}
		b.WriteString("\n")
	}

	// The checks, worst first. This is the backlog, and it is the half of the
	// report anybody actually acts on.
	by := map[string]int{}
	for _, r := range rows {
		for _, f := range r.Failed {
			by[f]++
		}
	}
	if len(by) > 0 {
		names := make([]string, 0, len(by))
		for k := range by {
			names = append(names, k)
		}
		sort.Slice(names, func(i, j int) bool {
			if by[names[i]] != by[names[j]] {
				return by[names[i]] > by[names[j]]
			}
			return names[i] < names[j]
		})
		b.WriteString("## The checks, and how many builds each one fails\n\n| check | builds failing |\n| --- | --- |\n")
		for _, n := range names {
			fmt.Fprintf(&b, "| %s | %d |\n", n, by[n])
		}
		b.WriteString("\n")
	}

	if refused > 0 {
		fmt.Fprintf(&b, "## Refused under the floor\n\nA language that holds less than %.0f%% of the printing's "+
			"sections is not bound. The last sweep before the floor existed produced twenty one four page PDFs "+
			"this way, a cover and a title page and a contents of nothing, each of them passing eighteen of "+
			"twenty one checks because there was no text in them to be wrong about.\n\n"+
			"| volume | language | holds |\n| --- | --- | --- |\n", 100*aopt.Floor)
		for _, r := range rows {
			if r.Refused != "" {
				fmt.Fprintf(&b, "| %s | %s | %s |\n", r.ID, r.Lang, r.Refused)
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("## Every build\n\n")
	b.WriteString("| volume | language | of the printing | files | pages | printing's pages | checks passed |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	for _, r := range rows {
		if r.Broke != "" {
			fmt.Fprintf(&b, "| %s | %s | %d%% | | | %d | could not be assembled |\n",
				r.ID, r.Lang, r.OfPrint, r.PrintedPages)
			continue
		}
		if r.Refused != "" {
			fmt.Fprintf(&b, "| %s | %s | %d%% | | | %d | refused under the floor |\n",
				r.ID, r.Lang, r.OfPrint, r.PrintedPages)
			continue
		}
		note := ""
		if r.TypesetterFailed != "" {
			note = " (the typesetter stopped)"
		}
		fmt.Fprintf(&b, "| %s | %s | %d%% | %d | %d | %d | %d of %d%s |\n",
			r.ID, r.Lang, r.OfPrint, r.Files, r.Pages, r.PrintedPages, r.Passed, r.Ran, note)
	}
	return b.String()
}
