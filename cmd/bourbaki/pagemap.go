package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"text/tabwriter"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/pdfsrc"
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
	grammar := fs.String("grammar", "", "override the detected grammar: head-label or foot-number")
	minRun := fs.Int("min-run", pagemap.DefaultMinRun, "how many anchors must agree before a change of offset is believed")
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

	ctx := context.Background()
	failed := 0
	for _, b := range books {
		src, err := pdfsrc.Open(filepath.Join(root, b.PDF))
		if err != nil {
			return err
		}
		// One pdftotext call for the whole document. Per page it is 1700 process
		// launches for the three volumes and takes minutes; in one call it takes
		// seconds.
		text, err := src.Text(ctx, 0, 0, true)
		if err != nil {
			return err
		}
		pages := pagemap.SplitPages(text)
		if len(pages) != b.Pages {
			fmt.Printf("%s: pdftotext gave %d pages, the manifest says %d\n", b.ID, len(pages), b.Pages)
		}
		pm, err := pagemap.Build(pages, pagemap.Options{
			Book:       b.ID,
			Chapters:   b.Chapters,
			Grammar:    pagemap.Grammar(*grammar),
			Pagination: pagemap.Pagination(b.Pagination),
			MinRun:     *minRun,
		})
		if err != nil {
			return err
		}
		printPagemap(pm)
		if probs := pm.Validate(); len(probs) > 0 {
			fmt.Printf("  %d problems:\n", len(probs))
			for _, pr := range probs {
				fmt.Printf("    %s\n", pr)
			}
			failed++
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
