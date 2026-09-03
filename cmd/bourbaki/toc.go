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
	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/prompt"
	"github.com/tamnd/bourbaki-solver/toc"
)

func runTOC(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("toc: want one of build, body, show, verify")
	}
	switch args[0] {
	case "build":
		return tocBuild(args[1:])
	case "show":
		return tocShow(args[1:])
	case "body":
		return tocBody(args[1:])
	case "verify":
		return tocVerify(args[1:])
	default:
		return fmt.Errorf("toc: unknown subcommand %q", args[0])
	}
}

func tocBuild(args []string) error {
	fs := flag.NewFlagSet("toc build", flag.ExitOnError)
	book := fs.String("book", "", "book id, or empty for every book in the manifest")
	dry := fs.Bool("n", false, "print the result without writing anything")
	verbose := fs.Bool("v", false, "print every § and no. of every chapter")
	retitle := fs.Bool("retitle", false, "take the titles the volume now reads, over the ones the manifest carries")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: bourbaki toc build [-book <id>] [flags]\n\nReads each volume's table of contents into manifests/toc/.\n\nA title already in the manifest is kept, because for a scanned volume the\ncontents page is OCR and its titles get corrected against the printing by\nhand. Every one kept is printed with what the volume now reads beside it,\nand counted in the summary. -retitle takes the new readings instead.\n\n")
		fs.PrintDefaults()
	}
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
	man, err := corpus.LoadTOC(root)
	if err != nil {
		return err
	}
	errata, err := corpus.LoadErrata(root)
	if err != nil {
		return err
	}
	list := books.Books
	if *book != "" {
		b, ok := books.Get(*book)
		if !ok {
			return fmt.Errorf("no book %q in %s", *book, corpus.BooksPath(root))
		}
		list = []corpus.Book{*b}
	}
	if len(list) == 0 {
		return fmt.Errorf("no books registered in %s", corpus.BooksPath(root))
	}

	ctx := context.Background()
	failed, skipped, derived := 0, 0, 0
	for _, b := range list {
		pm, err := pagemap.Load(root, b.ID)
		if err != nil {
			// The contents is read off the pages the page map leaves out, so a
			// volume that has not been mapped cannot be read yet. Naming one
			// book is a request and is an error when it cannot be met; asking
			// for every book is a sweep over a library that is mapped a few
			// volumes at a time.
			if *book != "" {
				return fmt.Errorf("%s: %w (run bourbaki pagemap build first)", b.ID, err)
			}
			skipped++
			continue
		}
		// A volume already read off its own pages is not a volume this failed
		// on. Three of the forty-four carry no contents this can read, and
		// bourbaki toc body was written for exactly those; their manifests say
		// so in their own grammar. Counting them as failures made a sweep of a
		// fully read library end in an error every time, which teaches a reader
		// to ignore the one line that matters.
		//
		// The three fail in two different ways and both have to be caught here.
		// ac-x-fr and lie-vii-viii-fr have no contents page at all, so the parse
		// returns an error. alg-iv-vii-fr prints a contents that reads cleanly
		// and is short a leaf: the scan runs from chapter V § 13 at the foot of
		// pdf 423 to chapter VII § 1 at the head of pdf 424, so the leaf holding
		// the end of chapter V and the whole of chapter VI is not in the file,
		// and the parse comes back with three of the four chapters and a problem
		// saying so. Taking that reading would throw away a chapter the body
		// route already has.
		fromBody := false
		if was, ok := man.Get(b.ID); ok {
			fromBody = readOffItsOwnPages(was.Grammar)
		}
		res, err := readContents(ctx, root, &b, pm, errata)
		if err != nil {
			if fromBody {
				fmt.Printf("%s  read off its own pages by toc body, left as it stands\n", b.ID)
				derived++
				continue
			}
			// Everything else is reported against the volume it happened to and
			// the sweep carries on. It used to return, and a single stranded
			// contents erratum in int-i-iv-fr therefore stopped the run before
			// it reached the thirty-odd volumes after it in the manifest, which
			// read perfectly well. A volume named on the command line is the
			// only volume in the list, so it still ends in a non-zero exit
			// through the count below.
			fmt.Printf("%s  %v\n", b.ID, err)
			failed++
			continue
		}
		// The titles the manifest already carries go back before anything is
		// printed, so what the table shows is what would be written. For a
		// scanned volume the contents page is OCR and its titles get corrected
		// against the printing by hand, and a rebuild used to put every one of
		// those corrections back the way the scan had it without saying so.
		if was, ok := man.Get(b.ID); ok {
			chapters, kept := toc.KeepTitles(was.Chapters, res.Chapters, pm)
			if !*retitle {
				res.Chapters = chapters
			}
			for _, one := range kept {
				what := "kept"
				if *retitle {
					what = "took"
				}
				fmt.Printf("  %s %s\n", what, one)
			}
			if len(kept) > 0 && !*retitle {
				// "readings" and not "titles": a printed page off the contents
				// is kept on the same grounds and gets counted in the same line.
				readings := "readings already in the manifest were kept"
				if len(kept) == 1 {
					readings = "reading already in the manifest was kept"
				}
				fmt.Printf("  %d %s, pass -retitle to take the new readings\n", len(kept), readings)
			}
		}
		if fromBody && len(toc.Hard(res.Problems)) > 0 {
			fmt.Printf("%s  read off its own pages by toc body, left as it stands\n", b.ID)
			derived++
			continue
		}
		printTOC(res, *verbose)
		if len(toc.Hard(res.Problems)) > 0 {
			// A contents with a problem in it is not written. What the parser
			// reports as a problem is a chapter it lost, a § it doubled or a
			// page the volume does not have, and a manifest that carries those
			// is worse than a manifest that says nothing: the coverage table
			// would then count sections the volume never had, and every reader
			// downstream would take the missing chapters for chapters nobody
			// has extracted yet.
			//
			// A soft problem is none of those. It says the scan is short a leaf
			// the volume prints, which is true of the file and not of the
			// reading, and refusing to write over it would hold a correct
			// contents hostage to a page nobody can put back by editing it.
			failed++
			continue
		}
		if *dry {
			continue
		}
		man.Upsert(corpus.BookTOC{ID: b.ID, Grammar: res.Grammar.String(), Chapters: res.Chapters})
	}
	if skipped > 0 {
		fmt.Printf("%d volumes have no page map yet and were not read\n", skipped)
	}
	if derived > 0 {
		volumes := "volumes carry a contents read off their own pages"
		if derived == 1 {
			volumes = "volume carries a contents read off its own pages"
		}
		fmt.Printf("%d %s and were left alone\n", derived, volumes)
	}
	if !*dry {
		if err := man.Save(root); err != nil {
			return err
		}
		fmt.Printf("written to %s\n", corpus.TOCDir(root))
	}
	if failed > 0 {
		return fmt.Errorf("%d volumes have contents problems and were not written", failed)
	}
	return nil
}

// readOffItsOwnPages says whether a contents already in the manifest came from
// the volume's body rather than from a contents page.
//
// The grammar a reading records is the pair the contents page was set in,
// pilcrow or column and bare or label, and a body reading has no contents page
// to have a grammar of, so it writes the word body in the first half. That is
// the whole of the mark: it is written by one command, bourbaki toc body, and
// there is nothing else in the corpus that can produce it.
func readOffItsOwnPages(grammar string) bool {
	return strings.HasPrefix(grammar, string(toc.Body)+"/")
}

// readContents is one volume's table of contents, from the pdf text through the
// re-readings and the errata to the parse.
//
// It is a function of its own so that the sweep has one place to decide what a
// volume it cannot read means. Each of the four steps used to return straight
// out of the loop, so whichever volume failed first ended the run for every
// volume after it in the manifest.
func readContents(ctx context.Context, root string, b *corpus.Book, pm *pagemap.Map,
	errata *corpus.ErrataManifest) (*toc.Result, error) {
	pages, err := volumeText(ctx, root, b)
	if err != nil {
		return nil, err
	}
	read, err := contentsReadings(root, b.ID)
	if err != nil {
		return nil, err
	}
	pages = toc.Overlay(pages, read, pm)
	if pages, err = correctContents(pages, errata.ContentsErrata(b.ID)); err != nil {
		return nil, err
	}
	return toc.Parse(pages, pm, toc.Options{
		Book: b.ID, Chapters: b.Chapters, Title: b.Title,
		FrontMatterPDF: frontMatterPDF(b)})
}

// contentsReadings is what the model read off the pages of the table of
// contents, by pdf page.
//
// The pages of a volume are all in one directory and are read with two different
// prompts, and what tells them apart is the hash the reading carries in its own
// front matter. That is exact and it needs no second list to be kept in step: a
// page read as a table of contents says so, and a page read as a page of the
// body says something else. It also means that re-reading the contents under an
// edited prompt takes the old readings out of this on its own, rather than
// leaving a stale one in place with nothing to say it is stale.
//
// A volume with no such page has none of this happen to it.
// frontMatterPDF is the last pdf page the manifest gives to the note to the
// reader and the introduction, and 0 for a volume that declares neither.
//
// The contents reader wants it for one check, on where a chapter opens against
// where the page map starts it. A volume that numbers its front matter inside
// chapter I puts real folios reading chapter I on leaves that are not chapter I,
// and this is the only thing in the corpus that knows where those leaves stop.
func frontMatterPDF(b *corpus.Book) int {
	last := 0
	for _, part := range []*corpus.Introduction{b.ReaderNote, b.Introduction} {
		if part != nil && part.LastPDFPage > last {
			last = part.LastPDFPage
		}
	}
	return last
}

func contentsReadings(root, book string) (map[int]string, error) {
	paths, err := filepath.Glob(filepath.Join(corpus.PagesDir(root, book), "*.md"))
	if err != nil {
		return nil, err
	}
	want := prompt.ContentsSHA256()
	out := map[int]string{}
	for _, path := range paths {
		file, err := corpus.ReadFile[corpus.PageFrontMatter](path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if file.Meta.PromptSHA256 != want || file.Meta.PDFPage <= 0 {
			continue
		}
		// The head goes back on the front of the body, because reading a page
		// took it off. The contents reader wants it where the page printed it:
		// it is what tells a page of the table of contents from a page of the
		// index in a volume that prints its contents at the back, where the page
		// map has already given those pages to the last chapter.
		body := file.Body
		if head := headOf(file.Meta); head != "" {
			body = head + "\n\n" + body
		}
		out[file.Meta.PDFPage] = body
	}
	return out, nil
}

// correctContents puts the errata of a volume's own table of contents into the
// pages before the contents is read off them.
//
// The correction is applied here rather than to the manifest because the
// manifest is generated and says so at the top of itself. A line that is not
// found, or is found more than once, is an error and not a warning: an erratum
// nobody applied is a person having written down a correction in the belief that
// it was in force, and the whole point of the check it is exempting is that a
// contents which disagrees with the pages usually means something worse.
func correctContents(pages []string, errata []corpus.Erratum) ([]string, error) {
	for _, e := range errata {
		n := 0
		for _, p := range pages {
			n += strings.Count(p, e.Says)
		}
		if n != 1 {
			return nil, fmt.Errorf("the contents erratum %q is on %d pages of the volume, want exactly one", e.Says, n)
		}
		for i, p := range pages {
			pages[i] = strings.Replace(p, e.Says, e.Read, 1)
			if pages[i] != p {
				break
			}
		}
	}
	return pages, nil
}

func printTOC(r *toc.Result, verbose bool) {
	ch, sec, sub, ex := r.Counts()
	fmt.Printf("%s  %s\n", r.Book, r.Grammar)
	fmt.Printf("  %d chapters, %d §, %d no., %d exercise runs\n", ch, sec, sub, ex)
	for _, c := range r.Chapters {
		held := fmt.Sprintf("%d §", len(c.Sections))
		if len(c.Subsections) > 0 {
			held = fmt.Sprintf("no §, %d no.", len(c.Subsections))
		}
		fmt.Printf("  chapter %-4s %-46s printed %d, pdf %d, %s\n",
			c.Numeral, trim(c.Title, 46), c.Page, c.PDFPage, held)
		if !verbose {
			continue
		}
		for _, sub := range c.Subsections {
			fmt.Printf("    no. %-3d %-42s printed %d, pdf %d\n",
				sub.Number, trim(sub.Title, 42), sub.Page, sub.PDFPage)
		}
		if len(c.Subsections) > 0 && c.Exercises != nil {
			fmt.Printf("    exercises %40s printed %d, pdf %d\n",
				"", c.Exercises.Page, c.Exercises.PDFPage)
		}
		for _, s := range c.Sections {
			kind := "§"
			if s.Appendix {
				kind = "appendix"
			}
			fmt.Printf("    %s %-3d %-42s printed %d, pdf %d\n",
				kind, s.Number, trim(s.Title, 42), s.Page, s.PDFPage)
			for _, sub := range s.Subsections {
				fmt.Printf("      no. %-3d %-40s printed %d, pdf %d\n",
					sub.Number, trim(sub.Title, 40), sub.Page, sub.PDFPage)
			}
			if s.Exercises != nil {
				fmt.Printf("      exercises %38s printed %d, pdf %d\n",
					"", s.Exercises.Page, s.Exercises.PDFPage)
			}
		}
	}
	hard := toc.Hard(r.Problems)
	if len(hard) > 0 {
		fmt.Printf("  %d problems:\n", len(hard))
		for _, p := range hard {
			fmt.Printf("    %s\n", p)
		}
	}
	// The soft ones are printed apart and named for what they are, so that a
	// volume written with one of them still says out loud what it is short.
	if n := len(r.Problems) - len(hard); n > 0 {
		fmt.Printf("  %d short of the scan:\n", n)
		for _, p := range r.Problems {
			if p.Soft {
				fmt.Printf("    %s\n", p)
			}
		}
	}
}

// trim is a title cut to fit a column of the table this prints.
//
// It counts runes and not bytes. Every title in the French volumes carries
// accents and half the titles in Lie carry a typographic apostrophe, all of
// which are two or three bytes, so a cut made on a byte count lands in the
// middle of one and prints a replacement character: "Décomposition des
// représentations d\ufffd" is what chapter VII of Lie used to show.
func trim(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n-1]) + "…"
}

func tocShow(args []string) error {
	fs := flag.NewFlagSet("toc show", flag.ExitOnError)
	chapter := fs.String("chapter", "", "chapter numeral, for example VIII")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: bourbaki toc show [-chapter <numeral>]\n")
	}
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	man, err := corpus.LoadTOC(root)
	if err != nil {
		return err
	}
	if *chapter != "" {
		c, ok := man.Chapter(*chapter)
		if !ok {
			return fmt.Errorf("no chapter %q in %s", *chapter, corpus.TOCDir(root))
		}
		printChapter(*c)
		return nil
	}
	for _, b := range man.Books {
		fmt.Printf("%s  %s\n", b.ID, b.Grammar)
		for _, c := range b.Chapters {
			printChapter(c)
		}
	}
	return nil
}

func printChapter(c corpus.Chapter) {
	fmt.Printf("chapter %s. %s  printed %d, pdf %d\n", c.Numeral, c.Title, c.Page, c.PDFPage)
	for _, sub := range c.Subsections {
		fmt.Printf("  %-5s %-52s printed %4d, pdf %4d\n",
			fmt.Sprintf("no. %d", sub.Number), trim(sub.Title, 52), sub.Page, sub.PDFPage)
	}
	if len(c.Subsections) > 0 && c.Exercises != nil {
		fmt.Printf("  exercises %52s printed %4d, pdf %4d\n", "", c.Exercises.Page, c.Exercises.PDFPage)
	}
	for _, s := range c.Sections {
		head := fmt.Sprintf("§ %d", s.Number)
		if s.Appendix {
			head = "appendix"
			if s.Number > 0 {
				head = fmt.Sprintf("appendix %d", s.Number)
			}
		}
		fmt.Printf("  %-5s %-52s printed %4d, pdf %4d\n", head, trim(s.Title, 52), s.Page, s.PDFPage)
		for _, sub := range s.Subsections {
			fmt.Printf("      %-3d %-52s printed %4d, pdf %4d\n", sub.Number, trim(sub.Title, 52), sub.Page, sub.PDFPage)
		}
		if s.Exercises != nil {
			fmt.Printf("      exercises %52s printed %4d, pdf %4d\n", "", s.Exercises.Page, s.Exercises.PDFPage)
		}
	}
	if c.Historical != nil {
		fmt.Printf("  historical note %49s printed %4d, pdf %4d\n", "", c.Historical.Page, c.Historical.PDFPage)
	}
}

func tocVerify(args []string) error {
	fs := flag.NewFlagSet("toc verify", flag.ExitOnError)
	book := fs.String("book", "", "book id, or empty for every book in the manifest")
	all := fs.Bool("a", false, "list every miss, not only the ones found on another page")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: bourbaki toc verify [-book <id>] [-a]\n\nOpens every page manifests/toc/ points at and looks for the heading\nthat is supposed to be printed there.\n\n")
		fs.PrintDefaults()
	}
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
	man, err := corpus.LoadTOC(root)
	if err != nil {
		return err
	}

	ctx := context.Background()
	moved := 0
	for _, bt := range man.Books {
		if *book != "" && bt.ID != *book {
			continue
		}
		b, ok := books.Get(bt.ID)
		if !ok {
			return fmt.Errorf("%s is in %s but not in %s", bt.ID, corpus.TOCPath(root, bt.ID), corpus.BooksPath(root))
		}
		// bestPageText and not volumeText, which is the whole of what this check
		// used to be able to see. volumeText is the layer the PDF carries and
		// three volumes carry none: alg-x-fr, top-v-x and ac-i-vii. For those it
		// hands back empty pages, every heading misses, and the report reads as
		// a page map that put nothing where the contents says it is. alg-x-fr
		// scored 0 of 68 the first time its contents was read, on a map whose
		// own numbers come off the pages and validate clean.
		//
		// And bestPageText rather than pageText, which went to the PDF's layer
		// for every volume that had one however bad it was. On a scanned volume
		// that layer is the scanner's own OCR and this project has read the same
		// pages far better since; the § that this check is built on is the
		// character the old layer drops most. It took the shelf from 6373 of
		// 6513 headings to 6499, and the 31 headings this check reported as
		// printed on a page the contents does not name to none: every one of the
		// 31 was the scanner's layer losing a mark, not the corpus putting a
		// heading in the wrong place. See bestPageText.
		pages, err := bestPageText(ctx, root, *b)
		if err != nil {
			return err
		}
		r := toc.Verify(pages, bt)
		fmt.Printf("%s  %d of %d headings on the page the contents names, %.1f%%\n",
			r.Book, r.Matched, r.Checked, r.Rate())
		list := r.Moved()
		if *all {
			list = r.Misses
		}
		fmt.Printf("  %d missed, %d of those printed on another page\n", len(r.Misses), len(r.Moved()))
		for _, c := range list {
			fmt.Printf("    %s\n", c)
		}
		moved += len(r.Moved())
	}
	if moved > 0 {
		return fmt.Errorf("%d headings are printed on a page the contents does not name", moved)
	}
	return nil
}

// tocBody reads a volume's contents off its own pages, for the volumes that
// have no contents page in the scan to read.
//
// It is a separate subcommand and not a fallback inside build, because the two
// readings are not of equal standing and the manifest should say which one it
// got. build reads the list the press printed and is complete by construction.
// This sweeps the headings out of a body and is only as complete as the reading
// of that body, so it is asked for by name, one volume at a time, and what it
// writes is looked at before it is committed.
func tocBody(args []string) error {
	fs := flag.NewFlagSet("toc body", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	dry := fs.Bool("n", false, "print the result without writing anything")
	verbose := fs.Bool("v", false, "print every § and no. of every chapter")
	retitle := fs.Bool("retitle", false, "take the titles the pages now read, over the ones the manifest carries")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: bourbaki toc body -book <id> [flags]\n\nReads a volume's § and no. headings off its pages into manifests/toc/,\nfor a volume whose scan carries no table of contents.\n\nThe chapters and their spans come from the page map. The § and no. come\nfrom the headings the press set on the pages, and the printed page of\neach comes from the page map rather than from the heading, because a\nheading does not carry one.\n\n")
		fs.PrintDefaults()
	}
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" {
		fs.Usage()
		return fmt.Errorf("toc body: -book is required")
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
	man, err := corpus.LoadTOC(root)
	if err != nil {
		return err
	}
	pm, err := pagemap.Load(root, b.ID)
	if err != nil {
		return fmt.Errorf("%s: %w (run bourbaki pagemap build first)", b.ID, err)
	}
	pages, err := bodyPages(root, b.ID)
	if err != nil {
		return err
	}
	if len(pages) == 0 {
		return fmt.Errorf("%s has no pages read yet", b.ID)
	}
	res := toc.FromBody(pages, pm, toc.Options{
		Book: b.ID, Chapters: b.Chapters, Title: b.Title})
	if was, ok := man.Get(b.ID); ok && !*retitle {
		chapters, kept := toc.KeepTitles(was.Chapters, res.Chapters, pm)
		res.Chapters = chapters
		for _, one := range kept {
			fmt.Printf("  kept %s\n", one)
		}
	}
	printTOC(res, *verbose)
	if len(toc.Hard(res.Problems)) > 0 {
		return fmt.Errorf("%s has contents problems and was not written", b.ID)
	}
	if *dry {
		return nil
	}
	man.Upsert(corpus.BookTOC{ID: b.ID, Grammar: res.Grammar.String(),
		Chapters: res.Chapters})
	if err := man.Save(root); err != nil {
		return err
	}
	fmt.Printf("written to %s\n", corpus.TOCDir(root))
	return nil
}

// bodyPages is every page of a volume as this corpus read it, in pdf order.
func bodyPages(root, book string) ([]toc.BodyPage, error) {
	paths, err := filepath.Glob(filepath.Join(corpus.PagesDir(root, book), "*.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []toc.BodyPage
	for _, path := range paths {
		file, err := corpus.ReadFile[corpus.PageFrontMatter](path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		if file.Meta.PDFPage <= 0 {
			continue
		}
		page := toc.BodyPage{PDFPage: file.Meta.PDFPage,
			RunningHead: file.Meta.RunningHead, Body: file.Body}
		if file.Meta.Locator != nil {
			page.Section = file.Meta.Locator.Section
		}
		out = append(out, page)
	}
	return out, nil
}
