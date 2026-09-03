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
		swaps, err := transpositions(b)
		if err != nil {
			return err
		}
		folios, err := pageFolios(root, b)
		if err != nil {
			return err
		}
		labels, err := pageLabels(root, b)
		if err != nil {
			return err
		}
		if err := correctHeads(pages, labels, errata.HeadErrata(b.ID)); err != nil {
			return fmt.Errorf("%s: %w", b.ID, err)
		}
		// What the manifest already says the volume does, unless the flag
		// overrules it. Detection is a guess made from the reading, and the
		// reading changes: Commutative Algebra is a foot-number volume whose
		// reader kept no foot numbers, so it detects as labelled off its
		// section titles and every rebuild of its map would quietly overturn
		// the line in books.yaml that says otherwise. The manifest is where a
		// settled answer lives, so a settled answer is used, and -grammar is
		// how it gets settled the first time or changed after.
		g, p := *grammar, *pagination
		if g == "" {
			g = b.Grammar
		}
		if p == "" {
			p = b.Pagination
		}
		pm, err := pagemap.Build(pages, pagemap.Options{
			Book:       b.ID,
			Chapters:   b.Chapters,
			Folios:     folios,
			Labels:     labels,
			Grammar:    pagemap.Grammar(g),
			Pagination: pagemap.Pagination(p),
			MinRun:     *minRun,
			FirstPage:  b.FirstPage,
			Restarts:   b.Restarts,
			Transposed: swaps,
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
//
// An empty says supplies a head the printing never set. Every house in the
// series suppresses the running head on the page a chapter opens on, and that
// page is numbered all the same, so a volume whose scan is missing a leaf right
// after its opener has no anchor at all in front of the gap and the fit slides
// the whole chapter down by the width of it. Algebre commutative chapitre 10 is
// that volume: its pdf 1 is the opener, printed AC X.1, its pdf 2 heads AC X.3,
// and the leaf carrying SS 1 and the opening of no. 1 was never scanned. With
// no anchor on pdf 1 the fit reads the volume as printed 2 to 180 and the map
// refuses to be written, because a per-chapter volume whose chapter starts at
// printed 2 is normally a fit that has slipped. Naming the page the opener
// carries turns the slide into the missing leaf it is, which the loader then
// reads back off the rows as a step.
// The label the reader wrote into the front matter is corrected here too, and
// it has to be, because it is the anchor the fit actually uses. readAnchors
// takes the label over the head wherever there is one, so a mangled label is
// not overruled by a sound head on the same page: it is preferred to it, and
// then thrown away by the filter that keeps only the volume's own Book prefix,
// which leaves the page with no anchor at all. Algebre commutative chapitre 10
// is that case seven times over. The reader lost the A off AC on pdf 41, 63,
// 73, 125, 131, 139 and 171, writing "C X.42" and its like, and the running
// head on each of those pages is the chapter title and carries no number to
// fall back to. Each of the seven fitted correctly all the same, because 119
// neighbours agree on the offset, so what the loss cost was not the number but
// the evidence for it: seven pages the volume prints a number on that the map
// had to call interpolated.
//
// The same erratum serves both, since which of the two a correction lands on is
// a fact about where the reader put the number rather than about the printing.
// An erratum is applied where its says is found and it is an error where it is
// found in neither place, on the same reasoning as below: a correction nobody
// applied is a person having believed it was in force.
func correctHeads(pages, labels []string, errata []corpus.PageErratum) error {
	for _, e := range errata {
		if e.PDFPage > len(pages) {
			return fmt.Errorf("the head erratum for pdf page %d is past the end of a %d page volume",
				e.PDFPage, len(pages))
		}
		pg := pages[e.PDFPage-1]
		if e.Says == "" {
			if strings.TrimSpace(e.Read) == "" {
				return fmt.Errorf("the head erratum for pdf page %d says nothing and reads nothing",
					e.PDFPage)
			}
			pages[e.PDFPage-1] = e.Read + "\n" + pg
			continue
		}
		onLabel := e.PDFPage <= len(labels) &&
			strings.TrimSpace(labels[e.PDFPage-1]) == strings.TrimSpace(e.Says)
		if onLabel {
			labels[e.PDFPage-1] = e.Read
		}
		// A says that is on the page more than once is still an error even when
		// the label carried it too, because the head it means is then not
		// decided and the wrong one would be rewritten.
		switch n := strings.Count(pg, e.Says); {
		case n == 1:
			pages[e.PDFPage-1] = strings.Replace(pg, e.Says, e.Read, 1)
		case n == 0 && onLabel:
		default:
			return fmt.Errorf("the head erratum %q is on pdf page %d %d times, want exactly one",
				e.Says, e.PDFPage, n)
		}
	}
	return nil
}

// transpositions is a volume's out of order leaves, checked here rather than in
// the fitter because what the manifest can get wrong is the shape of the entry
// and what the fitter can get wrong is the pages.
func transpositions(b corpus.Book) ([][2]int, error) {
	var out [][2]int
	for _, t := range b.Transposed {
		if len(t.Pages) != 2 {
			return nil, fmt.Errorf("%s: a transposition names %d pdf pages, want 2", b.ID, len(t.Pages))
		}
		if strings.TrimSpace(t.Why) == "" {
			return nil, fmt.Errorf("%s: the transposition of pdf %d and %d says no reason",
				b.ID, t.Pages[0], t.Pages[1])
		}
		out = append(out, [2]int{t.Pages[0], t.Pages[1]})
	}
	return out, nil
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
	for _, s := range r.Transposed {
		fmt.Printf("  pdf %d and %d are bound the wrong way round and are read in that order\n",
			s[0], s[1])
	}
	for _, p := range r.Restarts {
		e, ok := pm.Lookup(p)
		if !ok {
			fmt.Printf("  the numbering restarts at pdf %d, which is not in the file\n", p)
			continue
		}
		fmt.Printf("  the numbering restarts at pdf %d, printed page %d\n", p, e.Page)
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

// bestPageText is the best reading of every page of a volume: the page file
// where the volume has been read, and the PDF's own text layer where it has
// not.
//
// It exists because pageText answers a different question than the one a check
// on the printed page wants answered. pageText goes to the PDF for any volume
// whose manifest says it has a text layer, and for a scanned volume that layer
// is whatever the scanner left behind twenty years ago, which is not what read
// these pages. The page files under pages/ were read afterwards off the images
// by this project, and they are better by a wide margin.
//
// How wide is measurable. toc verify reported 140 headings as not printed on
// the page the contents names, and 108 of those were the start of a run of
// exercises. Every one of the 21 volumes holding them has text_layer: ocr, and
// not one of the 6 native or 4 unlayered volumes has a single one. Page 23 of
// top-v-x-fr is the shape of all of them: the run mark the check looks for is
// a line reading "§ 2", pages/top-v-x-fr/0023.md carries it, and the scanner's
// layer has a blank line where it was. The § is the character that layer loses
// most, and it is the one character the check is built on.
//
// The fall back is per page and not per volume, because a volume half read is
// the normal state of most of the shelf and the half that has been read should
// still be checked against the good reading.
//
// What it does not do is touch a native layer, and that line was measured too.
// A native layer is not a reading of anything: it is the publisher's own text,
// carried in the file since it was typeset, and it is exact in a way no OCR of
// a rendered image can be. Preferring the page files everywhere cost the three
// born-digital volumes 201 of 201 down to 176, 98 of 98 down to 93 and 165 of
// 165 down to 160. So the rule is only that this project's OCR beats a
// scanner's, which is the claim the evidence actually supports.
func bestPageText(ctx context.Context, root string, b corpus.Book) ([]string, error) {
	if b.TextLayer == "none" {
		return readPageFiles(root, b)
	}
	layer, err := volumeText(ctx, root, &b)
	if err != nil {
		return nil, err
	}
	if b.TextLayer != "ocr" {
		return layer, nil
	}
	read := 0
	for page := 1; page <= b.Pages && page <= len(layer); page++ {
		file, err := corpus.ReadFile[corpus.PageFrontMatter](corpus.PagePath(root, b.ID, page))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		// An empty body is a page that was read and found to hold nothing, a
		// plate or a blank verso. Taking it would replace whatever the layer has
		// with nothing, and a blank page is the one case where the two readings
		// cannot disagree about a heading anyway.
		if strings.TrimSpace(file.Body) == "" {
			continue
		}
		layer[page-1] = file.Body
		read++
	}
	if read < b.Pages {
		fmt.Printf("%s: %d of %d pages have been read, the rest are checked against the PDF's own text layer\n",
			b.ID, read, b.Pages)
	}
	return layer, nil
}

// pageFolios is the folio every page file of a volume records, in page order,
// with a zero where the page has not been read or does not say.
//
// It reads the page files whether or not the volume has a text layer, because
// the folio lives in the front matter either way and pageText goes to the PDF
// for a volume that has one.
func pageFolios(root string, b corpus.Book) ([]int, error) {
	folios := make([]int, b.Pages)
	for page := 1; page <= b.Pages; page++ {
		file, err := corpus.ReadFile[corpus.PageFrontMatter](corpus.PagePath(root, b.ID, page))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		folios[page-1] = file.Meta.Folio
	}
	return folios, nil
}

// pageLabels is the page label every page file of a volume records, in page
// order, with an empty string where the page has not been read or does not say.
//
// It reads the page files for the same reason pageFolios does, and the reason
// bites harder here. pageText goes to the PDF for any volume that has a text
// layer, and for a scan that layer is whatever the scanner left behind, which
// is not what read the pages: the page files were read afterwards off the image
// and the label went into the front matter rather than back into the body. So
// on the scanned labelled volumes the two sources are far apart, and the front
// matter is the better one.
func pageLabels(root string, b corpus.Book) ([]string, error) {
	labels := make([]string, b.Pages)
	for page := 1; page <= b.Pages; page++ {
		file, err := corpus.ReadFile[corpus.PageFrontMatter](corpus.PagePath(root, b.ID, page))
		if err != nil {
			if os.IsNotExist(err) {
				continue
			}
			return nil, err
		}
		labels[page-1] = file.Meta.PageLabel
	}
	return labels, nil
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
