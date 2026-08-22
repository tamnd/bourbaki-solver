package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/prompt"
	"github.com/tamnd/bourbaki-solver/render"
	"github.com/tamnd/bourbaki-solver/report"
)

const extractionUsage = `usage: bourbaki report extraction [flags]

Every volume in manifests/books.yaml against the page files in pages/: how much
of the PDF has been read, how it was read, and how much of what was read passes
the rules the extraction accepts a page on.

This is ocr check asked of the whole library at once, and with the pages that do
not exist in the denominator. A volume with 57 pages read out of 222 prints
100.0 % accepted from ocr check, because check only ever sees the files that are
there, and the 165 pages nobody has read are the number worth having.

Passing the rules is not the same as being right. A page can balance its
dollars, carry a plausible running head and still read an interval as a set,
which is what ocr audit is for. The rejected column is a work list.

flags:
  -book NAME     only this volume
  -json          print the report as JSON
  -write         write the report to reports/extraction-quality.md in the corpus
`

func reportExtractionCmd(args []string) error {
	fs := flag.NewFlagSet("report extraction", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, extractionUsage) }
	book := fs.String("book", "", "only this volume")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	write := fs.Bool("write", false, "write the report to reports/extraction-quality.md")
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

	var rows []report.Volume
	for _, entry := range books.Books {
		if *book != "" && entry.ID != *book {
			continue
		}
		row, err := extractionOf(root, entry)
		if err != nil {
			return err
		}
		// A volume nobody has read yet is still a row. Leaving it out would
		// make this report agree with whatever has been done so far, which is
		// the one thing it must never do.
		rows = append(rows, row)
	}
	if len(rows) == 0 {
		return fmt.Errorf("no volume %q in %s", *book, corpus.BooksPath(root))
	}

	out := report.SummariseExtraction(rows)
	if *asJSON {
		return printJSON(out)
	}
	if *write {
		// It goes in the corpus and not in the solver, for the same reason the
		// printings comparison does: it is a fact about the books as they have
		// been read, and committing it is what makes a volume that was half
		// read and is now whole show up as a line of a diff.
		path := filepath.Join(root, "reports", "extraction-quality.md")
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(out.Doc()), 0o644); err != nil {
			return err
		}
		fmt.Printf("report extraction: wrote %s\n", path)
		return nil
	}
	fmt.Print(out.Table())
	return nil
}

// extractionOf reads one volume's page files and runs the rules over them.
//
// The rules are run here rather than read out of anything, because there is
// nothing to read: a page file records how it was read and not whether it was
// read well, and the acceptance decision lives in ocr.Validate. This is the
// same call ocr check makes, built the same way through expectFor and
// checkText, so the two cannot drift into measuring different things.
func extractionOf(root string, entry corpus.Book) (report.Volume, error) {
	row := report.Volume{
		ID: entry.ID, Title: entry.Title, Lang: entry.Lang,
		TextLayer: entry.TextLayer, Pages: entry.Pages,
		Methods: map[string]int{}, Rules: map[string]int{},
	}

	paths, err := filepath.Glob(filepath.Join(corpus.PagesDir(root, entry.ID), "*.md"))
	if err != nil {
		return row, err
	}
	if len(paths) == 0 {
		return row, nil
	}
	sort.Strings(paths)

	pmap, mapErr := pagemap.Load(root, entry.ID)
	row.NoPageMap = mapErr != nil
	// The manifest is what says a page is blank or nearly blank, and the short
	// rule is relaxed on a page it calls sparse. It lives under images/, which
	// is not in git, so whether it is here at all changes the acceptance figure
	// by a fraction of a point between the machine that rendered the PDFs and a
	// clean checkout. Missing is the ordinary case rather than an error, and the
	// report says how many volumes had one so the number carries its own caveat.
	manifest, manifestErr := render.ReadManifest(root, entry.ID)
	row.NoManifest = manifestErr != nil

	for _, path := range paths {
		file, err := corpus.ReadFile[corpus.PageFrontMatter](path)
		if err != nil {
			return row, fmt.Errorf("%s: %w", path, err)
		}
		page := file.Meta.PDFPage
		row.Read++
		method := strings.TrimSpace(string(file.Meta.Method))
		if method == "" {
			// A page file with no method is not a page read by an unnamed
			// method, it is a page whose record is broken, and the table should
			// say so rather than leave a blank column.
			method = "not recorded"
		}
		row.Methods[method]++
		if method == "ocr-failed" {
			row.Failed = append(row.Failed, page)
		}
		if len(file.Meta.Flags) > 0 {
			row.Flagged = append(row.Flagged, page)
		}
		if file.Meta.Manual {
			row.Manual++
		}
		if file.Meta.Method == corpus.MethodBlank {
			continue
		}
		row.Checked++

		problems := ocr.Validate(checkText(file), expectFor(&entry, pmap, manifest, page), ocr.Options{Prompt: prompt.OCRAnything(entry.ID)})
		if len(problems) == 0 {
			continue
		}
		row.Rejected = append(row.Rejected, page)
		for _, rule := range ocr.Rules(problems) {
			row.Rules[string(rule)]++
		}
	}
	return row, nil
}
