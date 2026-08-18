package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/prompt"
	"github.com/tamnd/bourbaki-solver/toc"
)

func runTOC(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("toc: want one of build, show, verify")
	}
	switch args[0] {
	case "build":
		return tocBuild(args[1:])
	case "show":
		return tocShow(args[1:])
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
		fmt.Fprint(os.Stderr, "usage: bourbaki toc build [-book <id>] [flags]\n\nReads each volume's table of contents into manifests/toc.yaml.\n\nA title already in the manifest is kept, because for a scanned volume the\ncontents page is OCR and its titles get corrected against the printing by\nhand. Every one kept is printed with what the volume now reads beside it,\nand counted in the summary. -retitle takes the new readings instead.\n\n")
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
	failed, skipped := 0, 0
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
		pages, err := volumeText(ctx, root, &b)
		if err != nil {
			return err
		}
		read, err := contentsReadings(root, b.ID)
		if err != nil {
			return fmt.Errorf("%s: %w", b.ID, err)
		}
		pages = toc.Overlay(pages, read)
		if pages, err = correctContents(pages, errata.ContentsErrata(b.ID)); err != nil {
			return fmt.Errorf("%s: %w", b.ID, err)
		}
		res, err := toc.Parse(pages, pm, toc.Options{
			Book: b.ID, Chapters: b.Chapters})
		if err != nil {
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
			chapters, kept := toc.KeepTitles(was.Chapters, res.Chapters)
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
				titles := "titles already in the manifest were kept"
				if len(kept) == 1 {
					titles = "title already in the manifest was kept"
				}
				fmt.Printf("  %d %s, pass -retitle to take the new readings\n", len(kept), titles)
			}
		}
		printTOC(res, *verbose)
		if len(res.Problems) > 0 {
			// A contents with a problem in it is not written. What the parser
			// reports as a problem is a chapter it lost, a § it doubled or a
			// page the volume does not have, and a manifest that carries those
			// is worse than a manifest that says nothing: the coverage table
			// would then count sections the volume never had, and every reader
			// downstream would take the missing chapters for chapters nobody
			// has extracted yet.
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
	if !*dry {
		if err := man.Save(root); err != nil {
			return err
		}
		fmt.Printf("written to %s\n", corpus.TOCPath(root))
	}
	if failed > 0 {
		return fmt.Errorf("%d volumes have contents problems and were not written", failed)
	}
	return nil
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
	if n := len(r.Problems); n > 0 {
		fmt.Printf("  %d problems:\n", n)
		for _, p := range r.Problems {
			fmt.Printf("    %s\n", p)
		}
	}
}

func trim(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n-1] + "…"
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
			return fmt.Errorf("no chapter %q in %s", *chapter, corpus.TOCPath(root))
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
		fmt.Fprint(os.Stderr, "usage: bourbaki toc verify [-book <id>] [-a]\n\nOpens every page manifests/toc.yaml points at and looks for the heading\nthat is supposed to be printed there.\n\n")
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
			return fmt.Errorf("%s is in %s but not in %s", bt.ID, corpus.TOCPath(root), corpus.BooksPath(root))
		}
		pages, err := volumeText(ctx, root, b)
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
