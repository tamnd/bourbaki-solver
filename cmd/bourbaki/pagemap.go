package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
)

func runPagemap(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("pagemap: want one of build, show, gaps")
	}
	switch args[0] {
	case "build":
		return pagemapBuild(args[1:])
	case "show":
		return pagemapShow(args[1:])
	case "gaps":
		return pagemapGaps(args[1:])
	default:
		return fmt.Errorf("pagemap: unknown subcommand %q", args[0])
	}
}

func pagemapBuild(args []string) error {
	fs := flag.NewFlagSet("pagemap build", flag.ExitOnError)
	book := fs.String("book", "", "book id, or empty for every book in the manifest")
	grammar := fs.String("grammar", "", "override the detected grammar: head-label, head-number or foot-number")
	pagination := fs.String("pagination", "", "override the detected pagination: per-chapter or continuous")
	minRun := fs.Int("min-run", 0, "how many anchors must agree before a change of offset is believed, 0 for the default")
	dry := fs.Bool("n", false, "print the result without writing anything")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: bourbaki pagemap build [-book <id>] [flags]\n\nMaps PDF pages to the page numbers Bourbaki printed.\n\n")
		fs.PrintDefaults()
	}
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}

	root, err := corpus.Root()
	if err != nil {
		return err
	}
	m, err := corpus.LoadBooks(root)
	if err != nil {
		return err
	}
	books := m.Books
	if *book != "" {
		b, ok := m.Get(*book)
		if !ok {
			return fmt.Errorf("no book %q in %s", *book, corpus.BooksPath(root))
		}
		books = []corpus.Book{*b}
	}
	if len(books) == 0 {
		return fmt.Errorf("no books registered in %s", corpus.BooksPath(root))
	}

	errata, err := corpus.LoadErrata(root)
	if err != nil {
		return err
	}

	ctx := context.Background()
	failed := 0
	for _, b := range books {
		pages, err := pageText(ctx, root, b)
		if err != nil {
			return err
		}
		if len(pages) != b.Pages {
			fmt.Printf("%s: got %d pages, the manifest says %d\n", b.ID, len(pages), b.Pages)
		}
		if err := correctHeads(pages, errata.HeadErrata(b.ID)); err != nil {
			return fmt.Errorf("%s: %w", b.ID, err)
		}
		pm, err := pagemap.Build(pages, pagemap.Options{
			Book:       b.ID,
			Chapters:   b.Chapters,
			Grammar:    pagemap.Grammar(*grammar),
			Pagination: pagemap.Pagination(*pagination),
			MinRun:     *minRun,
			FirstPage:  b.FirstPage,
		})
		if err != nil {
			return err
		}
		printPagemap(pm)
		if probs := pm.Validate(); len(probs) > 0 {
			// A map with a problem in it is not written, which is what the
			// line at the end of the run has always said and what this did
			// not do. Every reader downstream takes the map for the truth
			// about where a printed page is, and a map that contradicts
			// itself is worse than no map: the volume counts as done, the
			// coverage table counts its pages, and the page the fitter got
			// wrong is quoted back by everything that asks.
			fmt.Printf("  %d problems, not written:\n", len(probs))
			for _, pr := range probs {
				fmt.Printf("    %s\n", pr)
			}
			failed++
			continue
		}
		if *dry {
			continue
		}
		if err := pm.Save(root); err != nil {
			return err
		}
		b.Grammar = string(pm.Grammar)
		b.Pagination = string(pm.Pagination)
		m.Upsert(b)
		fmt.Printf("  written to %s\n", pagemap.Path(root, b.ID))
	}
	if !*dry {
		if err := m.Save(root); err != nil {
			return err
		}
	}
	if failed > 0 {
		return fmt.Errorf("%d volumes did not map", failed)
	}
	return nil
}

// correctHeads puts the errata of a volume's running heads into the pages
// before the page numbers are read off them.
//
// It is applied here and not to the finished map because the map is generated
// and says so at the top of itself, and because a head that reads correctly is
// an anchor the fit uses rather than a conflict it has to talk itself out of.
//
// A says that is not on the page it names, or is on it more than once, is an
// error and not a warning. An erratum nobody applied is a person having written
// down a correction in the belief that it was in force, and the page it names
// is exactly the page the fit was going to get wrong.
func correctHeads(pages []string, errata []corpus.PageErratum) error {
	for _, e := range errata {
		if e.PDFPage > len(pages) {
			return fmt.Errorf("the head erratum for pdf page %d is past the end of a %d page volume",
				e.PDFPage, len(pages))
		}
		pg := pages[e.PDFPage-1]
		if n := strings.Count(pg, e.Says); n != 1 {
			return fmt.Errorf("the head erratum %q is on pdf page %d %d times, want exactly one",
				e.Says, e.PDFPage, n)
		}
		pages[e.PDFPage-1] = strings.Replace(pg, e.Says, e.Read, 1)
	}
	return nil
}

func printPagemap(pm *pagemap.Map) {
	r := pm.Report()
	fmt.Printf("%s  %s  %s  %d pdf pages\n", pm.Book, pm.Grammar, pm.Pagination, pm.PDFPages)
	fmt.Printf("  %d body pages, %d front matter\n", r.BodyPages, r.FrontMatter)
	keys := make([]string, 0, len(r.Counts))
	for k := range r.Counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("  %-14s %4d\n", k, r.Counts[k])
	}
	fmt.Printf("  printed on the page: %.1f%% of body\n", r.PrintedPct)
	for _, s := range r.Chapters {
		fmt.Printf("  chapter %-4s pdf %d-%d, printed %d-%d\n",
			s.Chapter, s.FirstPDF, s.LastPDF, s.FirstPage, s.LastPage)
	}
	for _, s := range r.Steps {
		fmt.Printf("  offset steps at pdf %d, printed page %v is not in the file\n",
			s.AtPDFPage, s.MissingPages)
	}
	if n := len(r.Conflicts); n > 0 {
		fmt.Printf("  %d readings disagree with the fit, first few:\n", n)
		for i, c := range r.Conflicts {
			if i == 5 {
				break
			}
			fmt.Printf("    %s\n", c)
		}
	}
}

func pagemapShow(args []string) error {
	fs := flag.NewFlagSet("pagemap show", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: bourbaki pagemap show -book <id> [pdf page ...]\n")
	}
	pos, err := parseFlags(fs, args)
	if err != nil {
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
	pm, err := pagemap.Load(root, *book)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		printPagemap(pm)
		return nil
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "PDF\tCHAPTER\tPAGE\tCONFIDENCE")
	for _, p := range pos {
		n := 0
		if _, err := fmt.Sscanf(p, "%d", &n); err != nil {
			return fmt.Errorf("not a page number: %q", p)
		}
		e, ok := pm.Lookup(n)
		if !ok {
			return fmt.Errorf("pdf page %d is outside %s", n, *book)
		}
		fmt.Fprintf(w, "%d\t%s\t%d\t%s\n", e.PDFPage, e.Chapter, e.Page, e.Confidence)
	}
	return w.Flush()
}

func pagemapGaps(args []string) error {
	fs := flag.NewFlagSet("pagemap gaps", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	min := fs.Int("min", 1, "only show runs at least this long")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	m, err := corpus.LoadBooks(root)
	if err != nil {
		return err
	}
	for _, b := range m.Books {
		if *book != "" && b.ID != *book {
			continue
		}
		pm, err := pagemap.Load(root, b.ID)
		if err != nil {
			return err
		}
		for _, g := range pm.Gaps {
			if g.Pages < *min {
				continue
			}
			fmt.Printf("%-12s pdf %d-%d  %d pages  %s\n", b.ID, g.From, g.To, g.Pages, g.Confidence)
		}
	}
	return nil
}

// pageText is the text of every page of a volume, indexed from page 1, which is
// what the head parsers read.
//
// For nearly every volume that is pdftotext: one call for the whole document,
// because per page it is 1700 process launches over the three Algebra volumes
// and takes minutes where one call takes seconds.
//
// Three volumes have no text layer at all. Commutative Algebra, General Topology
// chapters 5 to 10 and the French Algebra chapter 10 are scans nobody has run
// OCR over, so pdftotext returns 642, 372 and 222 empty pages and every page map
// built from them is empty. Those read their pages back out of pages/<book>/,
// which is what render and ocr put there, and say so plainly when they are not
// there yet rather than reporting a volume with no anchors on it.
func pageText(ctx context.Context, root string, b corpus.Book) ([]string, error) {
	if b.TextLayer == "none" {
		return readPageFiles(root, b)
	}
	return volumeText(ctx, root, &b)
}

// readPageFiles is the body of every page file of a volume, in page order, with
// a gap where a page has not been read. A gap is not an error here: the page map
// wants a running head off as many pages as it can get, and a volume half read
// still yields the offsets of the half that is there.
func readPageFiles(root string, b corpus.Book) ([]string, error) {
	pages := make([]string, b.Pages)
	found := 0
	for page := 1; page <= b.Pages; page++ {
		file, err := corpus.ReadFile[corpus.PageFrontMatter](corpus.PagePath(root, b.ID, page))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		pages[page-1] = file.Body
		found++
	}
	if found == 0 {
		return nil, fmt.Errorf("%s carries no text layer and has no pages in %s: run bourbaki render -book %s and then bourbaki ocr -book %s first",
			b.ID, corpus.PagesDir(root, b.ID), b.ID, b.ID)
	}
	if found < b.Pages {
		fmt.Printf("%s: %d of %d pages have been read, the map covers those\n", b.ID, found, b.Pages)
	}
	return pages, nil
}
