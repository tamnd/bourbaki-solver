package main

import (
	"bytes"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/assemble"
	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/tags"
)

// assemble is the stage that turns five hundred page files into twenty-six
// chapter files. It reads pages/ and manifests/toc.yaml and nothing else, so it
// runs anywhere the repository is checked out, PDFs or no PDFs, and it writes
// content/, manifests/sections.yaml, and a line per section to the terminal.
//
// -check is the same run with the writes taken out and a comparison put in its
// place. It is what CI runs: the whole stage is meant to be a pure function of
// its inputs, and the only way to know that it stays one is to run it against
// what is committed and diff.

func runAssemble(args []string) error {
	fs := flag.NewFlagSet("assemble", flag.ExitOnError)
	book := fs.String("book", "", "book id, as in manifests/books.yaml")
	lang := fs.String("lang", "en", "language of the pages being assembled")
	check := fs.Bool("check", false, "assemble but write nothing, and report what differs from what is committed")
	quiet := fs.Bool("q", false, "print only the totals")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: bourbaki assemble -book <id> [-lang en] [-check]\n\n")
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
	toc, err := corpus.LoadTOC(root)
	if err != nil {
		return err
	}
	bt, ok := toc.Get(*book)
	if !ok || len(bt.Chapters) == 0 {
		return fmt.Errorf("no table of contents for %q in %s: run bourbaki toc build first",
			*book, corpus.TOCPath(root))
	}
	pages, err := readPages(root, *book)
	if err != nil {
		return err
	}
	if len(pages) == 0 {
		return fmt.Errorf("no pages in %s: run bourbaki extract run first", corpus.PagesDir(root, *book))
	}

	// The permanent tags are read and written back out, never allocated here:
	// assembly runs on every push and minting an identifier that can never be
	// taken back is not something a build gets to do. bourbaki tags assign is.
	set, err := tags.Load(root)
	if err != nil {
		return err
	}
	tagOf := set.Lookup()

	rec := corpus.BookSections{ID: *book}
	exrec := corpus.BookExercises{ID: *book}
	files := map[string][]byte{}
	var statements, exercises int
	for _, ch := range bt.Chapters {
		pieces, err := assemble.Chapter(b.Book, ch, pages)
		if err != nil {
			return err
		}
		cr := corpus.ChapterSections{Chapter: ch.Numeral, Title: ch.Title}
		cx := corpus.ChapterExercises{Chapter: ch.Numeral, Title: ch.Title}
		for _, p := range pieces {
			if err := writeExercises(root, *lang, p, files, &cx, tagOf); err != nil {
				return err
			}
			f := sectionFile(*b, ch, p, *lang, tagOf)
			path := corpus.SectionPath(root, *lang, f.Meta)
			out, err := f.Bytes()
			if err != nil {
				return err
			}
			files[path] = out
			rel, _ := filepath.Rel(root, path)
			cr.Sections = append(cr.Sections, corpus.SectionRecord{
				Kind:          kindOf(p),
				Section:       f.Meta.Section,
				Title:         f.Meta.SectionTitle,
				Path:          filepath.ToSlash(rel),
				Label:         labelOf(b.Book, ch.Numeral, p),
				FirstPDFPage:  p.First,
				LastPDFPage:   p.Last,
				BookPages:     f.Meta.BookPages,
				Subsections:   len(p.Subsections),
				Statements:    len(p.Statements),
				Exercises:     len(p.Exercises),
				Extraction:    f.Meta.Extraction,
				ContentSHA256: corpus.ContentSHA256(f.Body),
			})
			statements += len(p.Statements)
			exercises += len(p.Exercises)
			if !*quiet {
				fmt.Printf("%-46s %4d-%-4d %3d no. %3d statements %3d exercises\n",
					filepath.Base(path), p.First, p.Last, len(p.Subsections),
					len(p.Statements), len(p.Exercises))
			}
		}
		rec.Chapters = append(rec.Chapters, cr)
		exrec.Chapters = append(exrec.Chapters, cx)
	}

	sections, err := corpus.LoadSections(root)
	if err != nil {
		return err
	}
	sections.Upsert(rec)
	manifest, err := sections.Bytes()
	if err != nil {
		return err
	}
	files[corpus.SectionsPath(root)] = manifest

	exm, err := corpus.LoadExercises(root)
	if err != nil {
		return err
	}
	exm.Upsert(exrec)
	exmanifest, err := exm.Bytes()
	if err != nil {
		return err
	}
	files[corpus.ExercisesPath(root)] = exmanifest

	stale, err := staleFiles(root, *lang, b.Book, bt.Chapters, files)
	if err != nil {
		return err
	}
	if *check {
		return checkFiles(root, files, stale)
	}
	for _, path := range sortedKeys(files) {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, files[path], 0o644); err != nil {
			return err
		}
	}
	for _, path := range stale {
		if err := os.Remove(path); err != nil {
			return err
		}
		fmt.Printf("removed %s\n", rel(root, path))
	}
	fmt.Printf("%s: %d files, %d statements, %d exercises\n",
		*book, len(files)-2, statements, exercises)
	return nil
}

// writeExercises turns the exercises of one piece into one file each and
// records what went out.
//
// A gap in the numbering stops the run. Bourbaki numbers the exercises of a §
// from one straight through, so 1 to 12 and then 14 is not something the book
// does; it is a page that never got read or a split that came apart, and
// writing the manifest and carrying on would leave the corpus quietly short of
// an exercise.
func writeExercises(root, lang string, p assemble.Piece, files map[string][]byte,
	cx *corpus.ChapterExercises, tagOf map[string]tags.Tag) error {
	if len(p.Exercises) == 0 {
		return nil
	}
	nums := make([]int, 0, len(p.Exercises))
	sx := corpus.SectionExercise{
		Section:  p.Number,
		Appendix: p.Appendix,
		Label:    p.Exercises[0].Ref().SectionLabel(),
		Dir:      corpus.ExerciseDir(p.Number, p.Appendix),
		Count:    len(p.Exercises),
	}
	for _, e := range p.Exercises {
		e.Meta.Tag = string(tagOf[e.Meta.Label])
		f := corpus.ExerciseFile{Meta: e.Meta, Body: e.Body}
		out, err := f.Bytes()
		if err != nil {
			return err
		}
		files[corpus.ExercisePath(root, lang, e.Meta)] = out
		nums = append(nums, e.Meta.Exercise)
		if e.Meta.Starred {
			sx.Starred++
		}
		if e.Meta.Supplementary {
			sx.Supplementary++
		}
	}
	sx.First, sx.Last = nums[0], nums[len(nums)-1]
	sx.Gaps = corpus.Gaps(nums)
	if len(sx.Gaps) > 0 {
		return fmt.Errorf("%s %s: the exercises run %d to %d and %v are missing",
			cx.Chapter, p.Name(), sx.First, sx.Last, sx.Gaps)
	}
	cx.Total += len(p.Exercises)
	cx.Section = append(cx.Section, sx)
	return nil
}

// sectionFile is one assembled piece as it goes to disk.
func sectionFile(b corpus.Book, ch corpus.Chapter, p assemble.Piece, lang string,
	tagOf map[string]tags.Tag) corpus.SectionFile {
	m := corpus.SectionFrontMatter{
		Book:          b.Book,
		BookTitle:     corpus.BookTitle(b.Book),
		Chapter:       ch.Numeral,
		ChapterTitle:  ch.Title,
		Section:       p.Number,
		SectionTitle:  p.Title,
		Appendix:      p.Appendix,
		Lang:          lang,
		Source:        b.ID,
		SourceEdition: b.Edition,
		BookPages:     pageRange(p.FirstLabel, p.LastLabel),
		PDFPages:      fmt.Sprintf("%04d-%04d", p.First, p.Last),
		Extraction:    p.Extraction(),
		Subsections:   p.Subsections,
		Statements:    len(p.Statements),
		Exercises:     len(p.Exercises),
	}
	switch {
	case p.Front:
		m.Kind, m.SectionTitle = corpus.KindFront, ch.Title
	case p.Historical:
		m.Kind, m.SectionTitle = corpus.KindHistorical, "Historical Note"
	}
	return corpus.SectionFile{Meta: m, Body: tags.Apply(p.Body, tagOf)}
}

func kindOf(p assemble.Piece) string {
	switch {
	case p.Front:
		return corpus.KindFront
	case p.Historical:
		return corpus.KindHistorical
	case p.Appendix:
		return corpus.KindAppendix
	}
	return corpus.KindSection
}

func labelOf(book, chapter string, p assemble.Piece) string {
	if p.Front || p.Historical {
		return ""
	}
	return corpus.Ref{Book: book, Chapter: chapter, Section: p.Number, Appendix: p.Appendix}.SectionLabel()
}

// pageRange is the span of printed pages a piece covers, "A VIII.1-A VIII.16".
// A volume that prints no page label leaves this empty rather than guessing.
func pageRange(first, last string) string {
	if first == "" || last == "" {
		return ""
	}
	if first == last {
		return first
	}
	return first + "-" + last
}

// readPages reads every page of a volume.
func readPages(root, book string) (map[int]corpus.PageFile, error) {
	dir := corpus.PagesDir(root, book)
	names, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}
	out := make(map[int]corpus.PageFile, len(names))
	for _, name := range names {
		f, err := corpus.ReadFile[corpus.PageFrontMatter](name)
		if err != nil {
			return nil, err
		}
		if f.Meta.PDFPage == 0 {
			return nil, fmt.Errorf("%s: no pdf_page in the front matter", name)
		}
		if _, dup := out[f.Meta.PDFPage]; dup {
			return nil, fmt.Errorf("%s: pdf page %d is read twice", name, f.Meta.PDFPage)
		}
		out[f.Meta.PDFPage] = f
	}
	return out, nil
}

// staleFiles are the files of these chapters that this run did not write.
//
// A section file is named for its title, so correcting a title renames the
// file, and the old one would sit there for ever looking like a section of the
// book. An exercise file is named for its number, so a § that loses an exercise
// to a corrected split leaves its last file behind, still numbered, still read
// by anything that walks the directory. Both are swept, which is what taocp's
// splitter calls --sync and has for the same reason: regenerating without it
// keeps deleted files around for ever.
func staleFiles(root, lang, book string, chapters []corpus.Chapter, written map[string][]byte) ([]string, error) {
	var out []string
	for _, ch := range chapters {
		dir := filepath.Dir(corpus.SectionPath(root, lang, corpus.SectionFrontMatter{
			Book: book, Chapter: ch.Numeral,
		}))
		names, err := filepath.Glob(filepath.Join(dir, "*.md"))
		if err != nil {
			return nil, err
		}
		ex, err := filepath.Glob(filepath.Join(dir, "exercises", "*", "*.md"))
		if err != nil {
			return nil, err
		}
		for _, name := range append(names, ex...) {
			if _, ok := written[name]; !ok {
				out = append(out, name)
			}
		}
	}
	sort.Strings(out)
	return out, nil
}

// checkFiles compares what this run would write against what is committed.
func checkFiles(root string, files map[string][]byte, stale []string) error {
	var bad []string
	for _, path := range sortedKeys(files) {
		have, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			bad = append(bad, "missing: "+rel(root, path))
			continue
		}
		if err != nil {
			return err
		}
		if !bytes.Equal(have, files[path]) {
			bad = append(bad, "differs: "+rel(root, path))
		}
	}
	for _, path := range stale {
		bad = append(bad, "not assembled from any page: "+rel(root, path))
	}
	if len(bad) > 0 {
		return fmt.Errorf("assemble -check: %d files out of date\n\t%s",
			len(bad), strings.Join(bad, "\n\t"))
	}
	fmt.Printf("assemble -check: %d files up to date\n", len(files))
	return nil
}

func sortedKeys(m map[string][]byte) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return r
	}
	return path
}
