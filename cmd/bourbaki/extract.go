package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/extract"
	"github.com/tamnd/bourbaki-solver/pagemap"
)

func runExtract(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("extract: want one of audit, fonts, page, prepare, run")
	}
	switch args[0] {
	case "audit":
		return extractAudit(args[1:])
	case "fonts":
		return extractFonts(args[1:])
	case "page":
		return extractPage(args[1:])
	case "prepare":
		return extractPrepare(args[1:])
	case "run":
		return extractRun(args[1:])
	default:
		return fmt.Errorf("extract: unknown subcommand %q", args[0])
	}
}

// extractFonts reports what fonts a born-digital volume was set in and which
// characters came out of each of them.
//
// This is the survey the extractor is written against. A born-digital volume
// carries its whole font stack, and the fonts say what pdftotext cannot: a run
// set in CMEX10 is a large operator whatever letter poppler printed for it, a
// run set in LMMathItalic is a variable and not a word, and a run set in
// LMRomanCaps is the small capitals a statement head opens with.
func extractFonts(args []string) error {
	fs := flag.NewFlagSet("extract fonts", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	family := fs.String("family", "", "show every distinct run set in families matching this substring")
	first := fs.Int("f", 0, "first pdf page")
	last := fs.Int("l", 0, "last pdf page")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: bourbaki extract fonts -book <id> [-family <substring>] [-f N] [-l N]\n\n")
		fs.PrintDefaults()
	}
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
	lay, err := src.XML(context.Background(), *first, *last)
	if err != nil {
		return err
	}

	type stat struct {
		base   string
		size   int
		spans  int
		runes  int
		pages  map[int]bool
		glyphs map[string]int
	}
	stats := map[string]*stat{}
	for _, p := range lay.Pages {
		for _, s := range p.Spans {
			f := lay.Font(s)
			key := fmt.Sprintf("%s/%d", f.Base(), f.Size)
			st := stats[key]
			if st == nil {
				st = &stat{base: f.Base(), size: f.Size, pages: map[int]bool{}, glyphs: map[string]int{}}
				stats[key] = st
			}
			st.spans++
			st.runes += len([]rune(strings.TrimSpace(s.Text)))
			st.pages[p.Number] = true
			if t := strings.TrimSpace(s.Text); t != "" {
				st.glyphs[t]++
			}
		}
	}
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return stats[keys[i]].runes > stats[keys[j]].runes })

	fmt.Printf("%s  %d pages, %d fonts, %d distinct family and size\n", b.ID, len(lay.Pages), len(lay.Fonts), len(keys))
	fmt.Printf("%-22s %5s %8s %8s %7s\n", "family", "size", "spans", "chars", "pages")
	for _, k := range keys {
		st := stats[k]
		fmt.Printf("%-22s %5d %8d %8d %7d\n", st.base, st.size, st.spans, st.runes, len(st.pages))
	}
	if *family == "" {
		return nil
	}
	for _, k := range keys {
		st := stats[k]
		if !strings.Contains(strings.ToLower(st.base), strings.ToLower(*family)) {
			continue
		}
		fmt.Printf("\n%s at %d, %d distinct runs\n", st.base, st.size, len(st.glyphs))
		runs := make([]string, 0, len(st.glyphs))
		for g := range st.glyphs {
			runs = append(runs, g)
		}
		sort.Slice(runs, func(i, j int) bool { return st.glyphs[runs[i]] > st.glyphs[runs[j]] })
		for _, g := range runs {
			fmt.Printf("  %6d  %-24q %s\n", st.glyphs[g], g, codes(g))
		}
	}
	return nil
}

// extractRun reads a whole born-digital volume and writes one page file per
// page under pages/<book>, plus a report of how the run went under
// reports/.
//
// The page files are scratch and work/ is not committed. The report is, because
// it is the claim: this many pages came out of the text layer with nothing
// flagged on them, and here is every page that did not, with the reason. A page
// on that list is a page the repair pass has to put in front of a model, and
// anybody can take the list, crop the printed page and check it.
func extractRun(args []string) error {
	fs := flag.NewFlagSet("extract run", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	first := fs.Int("f", 0, "first pdf page")
	last := fs.Int("l", 0, "last pdf page")
	dry := fs.Bool("dry-run", false, "count the pages without writing anything")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: bourbaki extract run -book <id> [-f N] [-l N] [-dry-run]\n\n")
		fs.PrintDefaults()
	}
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
	if b.Extraction != "native" {
		return fmt.Errorf("%s is extracted by %q, not native: this reads the text layer", b.ID, b.Extraction)
	}
	src, err := openPDF(root, b)
	if err != nil {
		return err
	}
	lay, err := src.XML(context.Background(), *first, *last)
	if err != nil {
		return err
	}
	if len(lay.Pages) == 0 {
		return fmt.Errorf("%s has no pages in %d-%d", b.ID, *first, *last)
	}
	// The page map is what turns a PDF page into the label the volume prints.
	// It is not required: a volume can be read before it has been mapped, and
	// the running head carries the label on most pages anyway.
	pm, err := pagemap.Load(root, b.ID)
	if err != nil && !os.IsNotExist(err) {
		return err
	}

	// The volume is read twice. A hyphen at the end of a line is the typesetter
	// breaking a word most of the time and the hyphen of a compound word the
	// rest of the time, and the line it stands on does not say which. The rest
	// of the book does: a compound broken at its hyphen here is set inside a
	// line somewhere else. So the first pass is only there to collect the words
	// the volume writes with a hyphen, and the second lays out the pages.
	vol := extract.Measure(lay)
	fmt.Printf("  running head band measured at %d\n", vol.HeadBand)
	compounds := extract.Compounds{}
	for _, pg := range lay.Pages {
		compounds.Read(extract.ReadPageWith(lay, pg, vol).Body)
	}
	vol.Compounds = compounds

	res := &extract.Result{Book: b.ID}
	var kept []int // the pages repaired by hand, which this run left alone
	for _, pg := range lay.Pages {
		p := extract.ReadPageWith(lay, pg, vol)
		res.Add(p)
		if *dry {
			continue
		}
		meta := corpus.PageFrontMatter{
			Book:        b.ID,
			PDFPage:     p.PDFPage,
			PageLabel:   pageLabel(pm, p),
			RunningHead: p.Title,
			Continues:   p.Continues,
			Method:      corpus.MethodNative,
			InputSHA256: extract.InputSHA256(lay, pg),
			Lines:       p.Lines,
		}
		if p.Section > 0 || p.Subsec > 0 {
			meta.Locator = &corpus.PageLocator{Section: p.Section, Subsec: p.Subsec}
		}
		if p.Lines == 0 {
			meta.Method = corpus.MethodBlank
		}
		if p.Columns {
			meta.Columns = 2
			res.Columns = append(res.Columns, p.PDFPage)
		}
		for _, f := range p.Flags {
			meta.Flags = append(meta.Flags, string(f))
		}
		path := corpus.PagePath(root, b.ID, p.PDFPage)
		// A page somebody has repaired by hand is left as it is. The text layer
		// has not changed since it was repaired, so re-reading it would put
		// back exactly the fault that was repaired, and the repair is the one
		// thing here that cost somebody reading the printed page.
		//
		// A page that will not parse is not a page that may be overwritten. It
		// used to be: the read and the manual check were one condition, so a
		// page the reader choked on fell through to the write as though it had
		// said nothing, and a hand repair went under it without a word. That is
		// how the generated stamp came out of the front matter and took seven
		// repaired pages of chapter VIII with it, because dropping the field
		// made every page written before it unreadable to a decoder that
		// refuses an unknown key, and the seven were only got back because they
		// were committed. So a page that is there and will not parse stops the
		// run and says which one.
		keep, err := repairedByHand(path)
		if err != nil {
			return err
		}
		if keep {
			kept = append(kept, p.PDFPage)
			continue
		}
		f := corpus.PageFile{Meta: meta, Body: p.Body}
		if err := f.Write(path); err != nil {
			return err
		}
	}
	fmt.Print(res)
	if *dry {
		return nil
	}
	if len(kept) > 0 {
		fmt.Printf("  %d pages were repaired by hand and were left alone: %v\n", len(kept), kept)
	}
	fmt.Printf("  pages written to %s\n", corpus.PagesDir(root, b.ID))
	out, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		return err
	}
	path := corpus.ExtractReportPath(root, b.ID)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, append(out, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("  report written to %s\n", path)
	return nil
}

// pageLabel is the label the volume prints on a page. The map is believed first
// because it was fitted over the whole volume and knows what a page carries even
// when the page does not say, and the running head is the fallback for a run
// made before the volume was mapped.
func pageLabel(pm *pagemap.Map, p *extract.Page) string {
	if pm != nil {
		if e, ok := pm.Lookup(p.PDFPage); ok {
			if e.Page > 0 {
				return fmt.Sprintf("A %s.%d", e.Chapter, e.Page)
			}
			// A page that opens a § prints its number at the foot and carries
			// no head, so the number is all there is and the chapter comes from
			// the map.
			if p.Label == "" && p.Foot > 0 && e.Chapter != "" {
				return fmt.Sprintf("A %s.%d", e.Chapter, p.Foot)
			}
		}
	}
	return p.Label
}

// level names where a run sits against the baseline.
func level(l extract.Level) string {
	switch l {
	case extract.Sup:
		return "sup"
	case extract.Sub:
		return "sub"
	}
	return "base"
}

// codes spells out a run as code points, since the runs worth looking at are
// the ones whose characters do not print.
func codes(s string) string {
	var b strings.Builder
	for _, r := range s {
		fmt.Fprintf(&b, "U+%04X ", r)
	}
	return strings.TrimSpace(b.String())
}

// extractPage prints one page as it would be written, with the lines and the
// flags, which is how a reading is argued with rather than guessed at.
func extractPage(args []string) error {
	fs := flag.NewFlagSet("extract page", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	page := fs.Int("p", 0, "pdf page")
	raw := fs.Bool("lines", false, "print one line per line, with its geometry")
	runs := fs.Bool("runs", false, "print one line per run, with its font, class and level")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: bourbaki extract page -book <id> -p <pdf page> [-lines] [-runs]\n\n")
		fs.PrintDefaults()
	}
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" || *page == 0 {
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
	lay, err := src.XML(context.Background(), *page, *page)
	if err != nil {
		return err
	}
	if len(lay.Pages) == 0 {
		return fmt.Errorf("%s has no pdf page %d", b.ID, *page)
	}
	if *runs {
		for i, l := range extract.Lines(lay, lay.Pages[0]) {
			fmt.Printf("line %d  [top %d bottom %d left %d]\n", i, l.Top, l.Bottom, l.Left)
			for _, r := range l.Runs {
				fmt.Printf("    %-22s %2d %-8s %-5s [%4d %4d %3d %3d] %q\n",
					r.Spec.Base(), r.Spec.Size, r.Class, level(r.Level),
					r.Top, r.Left, r.Width, r.Height, r.Text)
			}
		}
		return nil
	}
	if *raw {
		for _, l := range extract.Lines(lay, lay.Pages[0]) {
			fmt.Printf("[%4d %4d %4d] %s\n", l.Top, l.Left, l.Height(), extract.Render(l))
		}
		return nil
	}
	// The head band is a measurement over the volume, and one page cannot
	// supply it, so a window either side of the page is read for it. Without
	// this the page prints here with its running head in the body and prints
	// correctly in a run of the whole volume, which is the wrong way round for
	// a command whose whole use is arguing with a reading.
	around, err := src.XML(context.Background(), max(1, *page-12), *page+12)
	if err != nil {
		return err
	}
	vol := extract.Measure(around)
	p := extract.ReadPageWith(lay, lay.Pages[0], vol)
	fmt.Printf("head: %s [band %d]\nlines: %d  flags: %v\n\n%s\n",
		p.Head, vol.HeadBand, p.Lines, p.Flags, p.Body)
	return nil
}

// repairedByHand says whether the page already at path was repaired by hand and
// must be left alone.
//
// A page that is there and will not parse is an error and not a false. It used
// to be a false, because the read and the check were one condition, so a page
// the reader choked on fell through to the write as though it had said nothing
// and a repair went under it without a word. That is how dropping the generated
// stamp from the front matter took seven repaired pages of chapter VIII with
// it: the decoder refuses an unknown key, so every page written before the
// change stopped parsing at once. They came back because they were committed,
// which is luck and not a plan.
func repairedByHand(path string) (bool, error) {
	old, err := corpus.ReadFile[corpus.PageFrontMatter](path)
	if err == nil {
		return old.Meta.Manual, nil
	}
	if os.IsNotExist(err) {
		return false, nil // no page yet, which is every page of a first run
	}
	return false, fmt.Errorf("%s is on disk and will not parse, so this run "+
		"cannot tell whether it was repaired by hand: %w", path, err)
}
