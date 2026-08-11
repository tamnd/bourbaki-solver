package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/crosscheck"
	"github.com/tamnd/bourbaki-solver/pagemap"
)

const extractAuditUsage = `usage: bourbaki extract audit -book ID [flags]

Read every extracted page against pdftotext and print the words only pdftotext
has.

  -book ID    book id from manifests/books.yaml
  -f N -l N   first and last pdf page, default the whole volume
  -v          print every page that has anything on it, not just the count

The extractor reads poppler's boxes and fonts, which is the only way to get the
mathematics back, and every rule it applies to them is a place it can be wrong.
Nothing downstream can tell: a page that lost a word or glued two together is
the right length, its dollars balance and it reads perfectly.

pdftotext is a second reading of the same file. It knows nothing about
mathematics and throws all of it away, and that is the point: it has no idea
what our rules are, so where the two disagree about a word of prose, one of us
is wrong about the prose.

This is not a rule and it fails nothing. It is a list of places to look.
`

func extractAudit(args []string) error {
	fs := flag.NewFlagSet("extract audit", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	first := fs.Int("f", 0, "first pdf page")
	last := fs.Int("l", 0, "last pdf page")
	verbose := fs.Bool("v", false, "print every page that has anything on it")
	fs.Usage = func() { fmt.Fprint(os.Stderr, extractAuditUsage) }
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" {
		fs.Usage()
		os.Exit(2)
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return err
	}
	b, ok := books.Get(*book)
	if !ok {
		return fmt.Errorf("no book %q in %s", *book, corpus.BooksPath(root))
	}
	src, err := openPDF(root, b)
	if err != nil {
		return err
	}
	// One pdftotext call for the whole document. Per page it is a process launch
	// each and takes minutes; in one call it takes seconds.
	text, err := src.Text(context.Background(), 0, 0, true)
	if err != nil {
		return err
	}
	theirs := pagemap.SplitPages(text)

	paths, err := filepath.Glob(filepath.Join(corpus.PagesDir(root, b.ID), "*.md"))
	if err != nil {
		return err
	}
	sort.Strings(paths)

	var read, flagged, lostWords int
	for _, path := range paths {
		file, err := corpus.ReadFile[corpus.PageFrontMatter](path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		page := file.Meta.PDFPage
		if *first > 0 && page < *first || *last > 0 && page > *last {
			continue
		}
		if page < 1 || page > len(theirs) {
			continue
		}
		if strings.TrimSpace(file.Body) == "" {
			continue
		}
		read++
		lost := crosscheck.Page(file.Body, theirs[page-1])
		lost = append(lost, crosscheck.Extra(file.Body, theirs[page-1])...)
		if len(lost) == 0 {
			continue
		}
		flagged++
		lostWords += len(lost)
		if !*verbose {
			continue
		}
		fmt.Printf("page %4d  %d\n", page, len(lost))
		for _, l := range lost {
			side := "pdftotext"
			if l.Ours {
				side = "ours"
			}
			fmt.Printf("    %-9s %-24s %s\n", side, l.Word, l.Line)
		}
	}
	fmt.Printf("%s: %d pages read against pdftotext, %d with something on them, %d words\n",
		b.ID, read, flagged, lostWords)
	return nil
}
