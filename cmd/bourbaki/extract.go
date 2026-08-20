package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/extract"
	"github.com/tamnd/bourbaki-solver/katex"
	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/quality"
)

func runExtract(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("extract: want one of audit, drift, fonts, page, prepare, run")
	}
	switch args[0] {
	case "audit":
		return extractAudit(args[1:])
	case "drift":
		return extractDrift(args[1:])
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
		reads  string
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
				st = &stat{base: f.Base(), size: f.Size, reads: unnamed,
					pages: map[int]bool{}, glyphs: map[string]int{}}
				if extract.KnownFont(f) {
					st.reads = extract.Classify(f, s).String()
				}
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
	fmt.Printf("%-22s %5s %8s %8s %7s  %s\n", "family", "size", "spans", "chars", "pages", "read as")
	unknown := 0
	for _, k := range keys {
		st := stats[k]
		fmt.Printf("%-22s %5d %8d %8d %7d  %s\n", st.base, st.size, st.spans, st.runes, len(st.pages), st.reads)
		if st.reads == unnamed {
			unknown++
		}
	}
	if unknown > 0 {
		fmt.Printf("\n%d of the %d have no entry in the tables of extract/font.go and are read as\n", unknown, len(keys))
		fmt.Printf("prose, which is right for a text face and loses a mathematics font whole.\n")
		fmt.Printf("Every page carrying one is flagged unknown-font.\n")
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
	// The signs the book draws rather than sets are not in the text layer at
	// all, so they come from a second pass over the same pages. It costs about
	// sixty milliseconds a page: see pdfsrc/rule.go.
	if err := src.WithRules(context.Background(), lay); err != nil {
		return err
	}
	// The words are a third pass, and are the same text read one word at a
	// time rather than one run at a time. A bar drawn over one letter of a
	// sentence has nothing in the run reading to line up with: see pdfsrc/
	// word.go.
	if err := src.WithWords(context.Background(), lay); err != nil {
		return err
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
			PageLabel:   pageLabel(pm, b, p),
			Folio:       p.Foot,
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
		fmt.Printf("  %d pages were repaired by hand or read as pictures and were left alone: %v\n", len(kept), kept)
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
// pageLabel is the label of a page, "A VIII.13", built from the page map rather
// than from the head, because a page that opens a § carries no head at all.
//
// book is the Book of the Éléments and not the volume, since the letter belongs
// to the Book: chapters I to VIII of Algebra are printed in three volumes and
// all three label their pages "A". A Book the Éléments do not abbreviate has no
// label to build, and the page keeps whatever its own head said.
func pageLabel(pm *pagemap.Map, b *corpus.Book, p *extract.Page) string {
	letter := corpus.BookLetter(b.Book)
	// Whether there is a label to build is what the volume prints and not what
	// the number in it counts. A volume that prints a bare number at the foot
	// has no label anywhere, and gluing the chapter of a page to its running
	// number would make one up: "E IV.289" would read as page 289 of chapter
	// IV, and chapter IV of Theory of Sets is 60 pages long. Those volumes
	// carry the bare number in Folio and no label.
	//
	// Pagination was asked here instead, and it is the wrong question. Théories
	// spectrales prints TS I.1 to TS I.197 and then TS II.200, so the numeral
	// says which chapter the page is in while the number counts the volume:
	// head-label and continuous both, and there is nothing made up about the
	// label, since the volume prints it on the page. Asking pagination dropped
	// the label from every page of Théories spectrales and Topologie
	// algébrique that does not carry a running head, which is every page that
	// opens anything.
	if b.Grammar != "" && b.Grammar != string(pagemap.HeadLabel) {
		return p.Label
	}
	if pm != nil && letter != "" {
		if e, ok := pm.Lookup(p.PDFPage); ok {
			if e.Page > 0 {
				return fmt.Sprintf("%s %s.%d", letter, e.Chapter, e.Page)
			}
			// A page that opens a § prints its number at the foot and carries
			// no head, so the number is all there is and the chapter comes from
			// the map.
			if p.Label == "" && p.Foot > 0 && e.Chapter != "" {
				return fmt.Sprintf("%s %s.%d", letter, e.Chapter, p.Foot)
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
	rules := fs.Bool("rules", false, "print one line per drawn horizontal rule, with its geometry")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: bourbaki extract page -book <id> -p <pdf page> [-lines] [-runs] [-rules]\n\n")
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
	if err := src.WithRules(context.Background(), lay); err != nil {
		return err
	}
	// The words are a third pass, and are the same text read one word at a
	// time rather than one run at a time. A bar drawn over one letter of a
	// sentence has nothing in the run reading to line up with: see pdfsrc/
	// word.go.
	if err := src.WithWords(context.Background(), lay); err != nil {
		return err
	}
	// The rules are the part of a page that is drawn rather than set, so they
	// are in none of the other views here and are the only evidence for a
	// fraction, an overline or a set difference sign. Weight and length are
	// what say which of those a line is, so both are printed in points beside
	// the box.
	if *rules {
		for _, r := range lay.Pages[0].Rules {
			fmt.Printf("[%4d %4d %3d] thickness %.3f length %6.2f size %5.1f\n",
				r.Top, r.Left, r.Width, r.Thickness, r.Length, r.Size)
		}
		return nil
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

// extractDrift re-reads the pages a hand repair froze, and reports the ones the
// extractor would now write differently.
//
// A hand repair sets manual: true and extract run then leaves the page alone
// for good. The reason it gives is that the text layer has not changed since
// the repair, so re-reading the page would put back the same fault. That is
// true of the layer and not of the reader: every pass that taught the extractor
// something new made some of those repairs unnecessary, and a few of them
// wrong. Page 106 of the English printing is the one that showed it. Its sum
// was repaired for the prime and kept "\sum_i^n_{=1}", which KaTeX will not
// set, while a read today gives "\sum_{i=1}^n" with nothing done to it, and the
// page had been sitting frozen in front of the repair for three passes.
//
// So this is the drift between the two: what is committed against what the
// reader says today, on the pages nobody re-reads. It is a work list and not a
// rule. Every manual page differs here by construction, since the repair itself
// is a difference, and the number worth looking at is the one beside it: how
// many spans of each body KaTeX refuses. A page whose fresh read refuses fewer
// is a page whose repair has been overtaken.
//
// -unmarked runs the same comparison over the other pages, and there a
// difference means something else entirely. A page with no manual: true is a
// page the pipeline claims it wrote and could write again, so a fresh read of
// it ought to give back what is committed. When it does not, either somebody
// repaired the page and did not say so, or the extractor has changed under it.
// Both want looking at and only one of them is a bug, but the unmarked repair
// is the one that bites: extract run will happily write over a page that
// carries no mark, so the repair lasts exactly until the next run of the volume
// and nothing reports its loss. pages/ts-i-ii-fr/0283.md was found that way,
// with a closing bracket swept into a formula that somebody had pulled back out
// again, and it was found by rebuilding the corpus from the PDFs on a clean
// clone rather than by any rule. This is that check made cheap enough to run.
//
// It reads no model and costs nothing but the PDF, which is why it can be run
// over every native volume rather than the one somebody happens to suspect.
func extractDrift(args []string) error {
	fs := flag.NewFlagSet("extract drift", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	first := fs.Int("f", 0, "first pdf page")
	last := fs.Int("l", 0, "last pdf page")
	verbose := fs.Bool("v", false, "print the paragraphs that differ")
	fix := fs.Bool("fix", false, "take the paragraphs whose fresh read KaTeX sets and the committed one it does not")
	unmarked := fs.Bool("unmarked", false, "the other way round: pages that differ and carry no manual: true")
	setMark := fs.Bool("mark", false, "write manual: true on the pages -unmarked found")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: bourbaki extract drift -book <id> [-f N] [-l N] [-v] [-fix|-unmarked [-mark]]\n\n")
		fs.PrintDefaults()
	}
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" {
		fs.Usage()
		os.Exit(2)
	}
	// -fix takes the fresh read over the committed body, which is the right
	// trade on a page whose repair has been overtaken and the exact wrong one on
	// a page whose repair is unmarked: there the committed body is the repair
	// and the fresh read is the fault it was made to undo.
	if *fix && *unmarked {
		return errors.New("extract drift: -fix would undo the repair -unmarked just found, so the two do not go together")
	}
	if *setMark && !*unmarked {
		return errors.New("extract drift: -mark is what -unmarked does about what it finds, so it wants -unmarked")
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
	if err := src.WithRules(context.Background(), lay); err != nil {
		return err
	}
	// The words are a third pass, and are the same text read one word at a
	// time rather than one run at a time. A bar drawn over one letter of a
	// sentence has nothing in the run reading to line up with: see pdfsrc/
	// word.go.
	if err := src.WithWords(context.Background(), lay); err != nil {
		return err
	}
	// The same two passes extract run makes, because a page read on its own is
	// not the page a run of the volume writes: the head band and the compound
	// words are both measurements over the whole volume, and a drift report
	// that read them differently would report drift it had caused itself.
	vol := extract.Measure(lay)
	compounds := extract.Compounds{}
	for _, pg := range lay.Pages {
		compounds.Read(extract.ReadPageWith(lay, pg, vol).Body)
	}
	vol.Compounds = compounds

	eng, err := katex.New()
	if err != nil {
		return err
	}
	manual, moved, took := 0, 0, 0
	for _, pg := range lay.Pages {
		path := corpus.PagePath(root, b.ID, pg.Number)
		f, err := corpus.ReadFile[corpus.PageFrontMatter](path)
		if os.IsNotExist(err) {
			continue
		}
		if err != nil {
			return err
		}
		// The forward direction asks about hand repairs, so it wants the pages
		// carrying the mark and nothing else.
		//
		// -unmarked asks a different question: which pages would extract run
		// write over. That is not the same set. repairedByHand keeps a page for
		// three reasons, not one, and the other two matter here. A page read by
		// a model was read from the picture and not from the text layer, and a
		// page that is a picture was never text at all, so neither of them has
		// any business being measured against a fresh native extraction: they
		// differ from it by construction and every one of them would be reported
		// as an unmarked repair. Twelve pages of lie-vii-ix were reported that
		// way and all twelve are method: ocr. Asking exactly the question
		// extract run asks is what keeps the answer honest.
		if *unmarked {
			keep, err := repairedByHand(path)
			if err != nil {
				return err
			}
			if keep {
				continue
			}
		} else if !f.Meta.Manual {
			continue
		}
		manual++
		p := extract.ReadPageWith(lay, pg, vol)
		d := diffCount(paragraphs(f.Body), paragraphs(p.Body))
		if !drifted(*unmarked, d, f.Body, p.Body) {
			continue
		}
		moved++
		was, now := refused(eng, f.Body), refused(eng, p.Body)
		mark := "  "
		switch {
		case *unmarked:
			mark = "!!" // a page nobody marked, and it does not read back
		case now < was:
			mark = "<-" // the fresh read sets what the repair could not
		}
		fmt.Printf("%s %s p %d  %d paragraphs differ  KaTeX refuses %d, would refuse %d\n",
			mark, b.ID, pg.Number, d, was, now)
		if *verbose {
			for _, l := range paraDiff(paragraphs(f.Body), paragraphs(p.Body)) {
				fmt.Printf("    %s\n", l)
			}
		}
		// Marking is all that is done about an unmarked page. The committed body
		// is the repair and is kept exactly as it stands; what changes is that
		// extract run will now leave it alone. If the reader has since overtaken
		// the repair, the forward direction of this same command finds that and
		// -fix takes what KaTeX sets, which is the path a marked page is already
		// on. So the two directions compose and neither one guesses.
		if *setMark {
			f.Meta.Manual = true
			if err := f.Write(path); err != nil {
				return err
			}
			took++
			continue
		}
		if !*fix || now >= was {
			continue
		}
		body, n := overtaken(eng, f.Body, p.Body)
		if n == 0 {
			continue
		}
		f.Body = body
		if err := f.Write(path); err != nil {
			return err
		}
		took += n
		fmt.Printf("   took %d paragraphs from the fresh read of p %d\n", n, pg.Number)
	}
	if *unmarked {
		fmt.Printf("%s: %d pages carry no mark, %d of them do not read back\n", b.ID, manual, moved)
		if *setMark {
			fmt.Printf("%s: %d of them marked manual\n", b.ID, took)
		}
		return nil
	}
	fmt.Printf("%s: %d pages repaired by hand, %d of them read differently today\n", b.ID, manual, moved)
	if *fix {
		fmt.Printf("%s: %d paragraphs taken from the fresh read\n", b.ID, took)
	}
	return nil
}

// overtaken replaces the paragraphs of a repaired page that a read today sets
// and the repair does not, and returns the body and how many it took.
//
// Paragraph by paragraph and not page by page, because a page is rarely one or
// the other. Page 106 of the English printing wants the fresh read of its
// Corollary, whose sum the repair left as "\sum_i^n_{=1}", and wants to keep
// its next paragraph, where the repair wrote \sum for a large operator the
// reader still calls \Sigma. Page 471 wants the fresh d^{-1}_j and would lose
// three matrices the repair rebuilt if it took the whole page.
//
// A paragraph is taken only where the two readings write the same words around
// the mathematics and the fresh reading sets a formula the repair does not. The
// words are what says the two are the same paragraph: the drift is inside the
// mathematics, so prose that matches means both readings are looking at the
// same text and the only question left is which of them sets it. That is what
// keeps the rebuilt displays of page 471, which the fresh read hands back as
// three lines of wreckage whose words match nothing the repair wrote.
func overtaken(eng *katex.Renderer, oldBody, freshBody string) (string, int) {
	old, now := paragraphs(oldBody), paragraphs(freshBody)
	// The body has to come back out of its blocks as it went in, or what is
	// written back is a reformatting of the page with a repair somewhere in it.
	if strings.Join(old, "\n\n") != strings.TrimSuffix(corpus.NormalizeBody(oldBody), "\n") {
		return oldBody, 0
	}
	keep := common(old, now)
	out := append([]string(nil), old...)
	took, i, j := 0, 0, 0
	// The hunks between the paragraphs the two readings share. Inside a hunk a
	// paragraph is paired by its words and not by its position, and only where
	// the words pick out one paragraph and no more: a hunk is short, and two
	// paragraphs of one with the same words and different mathematics would be
	// a page saying the same sentence twice, which is a page to read and not to
	// rewrite.
	for _, k := range append(keep, [2]int{len(old), len(now)}) {
		for a := i; a < k[0]; a++ {
			b, n := 0, 0
			for c := j; c < k[1]; c++ {
				if prose(now[c]) == prose(old[a]) {
					b, n = c, n+1
				}
			}
			if n != 1 {
				continue
			}
			if refused(eng, now[b]) < refused(eng, old[a]) {
				out[a] = now[b]
				took++
			}
		}
		i, j = k[0]+1, k[1]+1
	}
	if took == 0 {
		return oldBody, 0
	}
	return strings.Join(out, "\n\n"), took
}

// prose is text with its mathematics taken out, its delimiters with it, and its
// spacing flattened. What is left is the words, in order.
//
// The delimiters go because the two readings are allowed to disagree about them:
// a formula the repair left inline in the middle of a sentence is the same
// formula the fresh read sets as a display of its own, and prose that counted
// the dollars would call those two different texts.
func prose(s string) string {
	spans, _ := quality.Math(s)
	r := []rune(s)
	out, at := make([]rune, 0, len(r)), 0
	for _, sp := range spans {
		if sp.Start < at || sp.End > len(r) {
			return s // an unclosed span, so nothing here is measured
		}
		out = append(out, r[at:sp.Start]...)
		at = sp.End
	}
	out = append(out, r[at:]...)
	kept := out[:0]
	for i, c := range out {
		if c == '$' && (i == 0 || out[i-1] != '\\') {
			continue
		}
		kept = append(kept, c)
	}
	return strings.Join(strings.Fields(string(kept)), " ")
}

// refused is how many math spans of a body KaTeX will not set, which is P04
// asked of one page rather than of the corpus.
func refused(eng *katex.Renderer, body string) int {
	n := 0
	spans, _ := quality.Math(body)
	for _, s := range spans {
		if _, err := eng.Render(s.Text, s.Display); err != nil {
			n++
		}
	}
	return n
}

// drifted says whether a page counts as having moved, and the two directions of
// extract drift want two different answers.
//
// On a marked page the byte is the right unit. The page is frozen, the question
// is whether a read today would write anything at all differently, and a change
// in whitespace is a change the run would make.
//
// On an unmarked page the byte is the wrong unit and badly so. Every page in
// this corpus has had bourbaki fix run over it since it was extracted, and the
// fresh read has not, so the two differ in whitespace and delimiters on nearly
// every page with not a word of the text having moved. Measured by byte, 338 of
// the 340 unmarked pages of ts-i-ii-fr had drifted, which is a report that says
// nothing. Measured by paragraph, which is the unit a repair is made in and
// which puts both sides through corpus.NormalizeBody first, four had, and all
// four were hand repairs nobody had marked.
func drifted(unmarked bool, paragraphs int, committed, fresh string) bool {
	if unmarked {
		return paragraphs > 0
	}
	return committed != fresh
}

// paragraphs cuts a page body into the blocks it is written in, which is the
// unit a hand repair works in and the unit a diff of one is readable in.
//
// A block is not a line. A paragraph is one long line, but a display is three,
// the fences and the mathematics between them, and 390 of the page files carry
// one. Cutting on the blank line rather than on the newline keeps a display
// whole, so it is compared and written back as the one thing it is.
func paragraphs(body string) []string {
	var out []string
	for _, s := range strings.Split(corpus.NormalizeBody(body), "\n\n") {
		if t := strings.Trim(s, "\n"); t != "" {
			out = append(out, t)
		}
	}
	return out
}

// paraDiff is the paragraphs of a page that changed, as - for the committed
// text and + for what a read today gives.
func paraDiff(old, now []string) []string {
	keep := common(old, now)
	var out []string
	i, j := 0, 0
	for _, k := range keep {
		for ; i < k[0]; i++ {
			out = append(out, "- "+old[i])
		}
		for ; j < k[1]; j++ {
			out = append(out, "+ "+now[j])
		}
		i, j = k[0]+1, k[1]+1
	}
	for ; i < len(old); i++ {
		out = append(out, "- "+old[i])
	}
	for ; j < len(now); j++ {
		out = append(out, "+ "+now[j])
	}
	return out
}

// diffCount is how many paragraphs paraDiff would print.
func diffCount(old, now []string) int { return len(paraDiff(old, now)) }

// common is the longest common subsequence of two pages, as the pairs of
// indices that match. A page is a few dozen paragraphs, so the square table is
// nothing, and pairing by index is not enough: a repair that split a display
// back out of a paragraph moves everything under it by one.
func common(a, b []string) [][2]int {
	n, m := len(a), len(b)
	t := make([][]int, n+1)
	for i := range t {
		t[i] = make([]int, m+1)
	}
	for i := n - 1; i >= 0; i-- {
		for j := m - 1; j >= 0; j-- {
			if a[i] == b[j] {
				t[i][j] = t[i+1][j+1] + 1
				continue
			}
			t[i][j] = max(t[i+1][j], t[i][j+1])
		}
	}
	var out [][2]int
	for i, j := 0, 0; i < n && j < m; {
		switch {
		case a[i] == b[j]:
			out = append(out, [2]int{i, j})
			i, j = i+1, j+1
		case t[i+1][j] >= t[i][j+1]:
			i++
		default:
			j++
		}
	}
	return out
}

// repairedByHand says whether the page already at path was repaired by hand and
// must be left alone.
//
// A page read through a model is left alone too, and it is not repaired by hand
// in any sense: it is a flagged page of a born-digital volume that the text
// layer could not carry, rendered and read as a picture and accepted against
// the rules. Extraction cannot produce that page. It produced the reading that
// was replaced, which is the one thing on disk it must not write back, and this
// is the second time that has had to be said in this repo. The first was
// render -blanks, which overwrote eleven of them with a note saying the page
// was blank.
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
		return old.Meta.Manual || old.Meta.Pictured || old.Meta.Method == corpus.MethodOCR, nil
	}
	if os.IsNotExist(err) {
		return false, nil // no page yet, which is every page of a first run
	}
	return false, fmt.Errorf("%s is on disk and will not parse, so this run "+
		"cannot tell whether it was repaired by hand: %w", path, err)
}

// unnamed is what the font survey prints for a family the tables have never
// been shown. It is not a class: a run in such a font is read as prose because
// there is nothing else to do with it, and the page it is on says so.
const unnamed = "UNNAMED"
