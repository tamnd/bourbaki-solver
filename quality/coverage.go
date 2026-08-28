package quality

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// Coverage is what the corpus holds against what it is meant to hold, written
// as the table the README carries between two markers.
//
// It is the answer to the question the structure rules deliberately do not ask.
// S01 is scoped to chapters that have been extracted, because reporting sixty
// absent sections of volumes nobody has read yet would bury the one section
// that really went missing. That leaves somebody having to say what has not
// been read, and this is where it is said, once, in the place people look.

// row is one chapter.
type coverageRow struct {
	book       string
	bookTitle  string
	chapter    string
	order      int
	want       int // §§ and appendices the table of contents names
	have       int // of those, the ones with a file
	statements int
	exercises  int
	tagged     int
	pages      int // page files committed for this chapter
}

// Coverage renders the block. It is a pure function of the manifests and the
// committed Markdown, like everything else here, so the README it writes can be
// checked by regenerating it.
func Coverage(c *Corpus) string {
	rows := coverageRows(c)
	var b strings.Builder
	b.WriteString("\n| Book | Chapter | Sections | Statements | Exercises | Tagged | Pages |\n")
	b.WriteString("| --- | --- | --- | --- | --- | --- | --- |\n")
	var want, have, statements, exercises, tagged int
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %d of %d | %d | %d | %d | %d |\n",
			r.bookTitle, r.chapter, r.have, r.want, r.statements, r.exercises, r.tagged, r.pages)
		want += r.want
		have += r.have
		statements += r.statements
		exercises += r.exercises
		tagged += r.tagged
	}
	pct := 0.0
	if want > 0 {
		pct = 100 * float64(have) / float64(want)
	}
	fmt.Fprintf(&b, "\n%d of %d sections are in the corpus, %.0f per cent. %d statements and %d exercises, %d of them carrying a permanent tag.\n",
		have, want, pct, statements, exercises, tagged)
	if n, pages := unread(c); n > 0 {
		fmt.Fprintf(&b, "\nThe table is one row per chapter of the volumes that have a table of contents. %d further volumes and %d pages are registered in `manifests/books.yaml` with no table of contents read off them yet, so none of their chapters are counted above.\n",
			n, pages)
	}
	return b.String()
}

// unread is how much of the registered library the table above says nothing
// about, in volumes and pages. Without it the percentage reads as a fraction of
// the Éléments when it is a fraction of the three volumes anybody has opened,
// which is the kind of number that looks like progress and is not.
func unread(c *Corpus) (volumes, pages int) {
	has := map[string]bool{}
	for _, bt := range c.TOC.Books {
		has[bt.ID] = true
	}
	for _, b := range c.Books.Books {
		if !has[b.ID] {
			volumes++
			pages += b.Pages
		}
	}
	return volumes, pages
}

// coverageRows is one row per chapter of every book in the table of contents,
// in reading order, whether or not anything has been extracted from it. A
// chapter that is missing entirely is the thing this table exists to show, so
// leaving it out would defeat the point.
func coverageRows(c *Corpus) []coverageRow {
	byKey := map[string]*coverageRow{}
	var order []string
	for _, bt := range c.TOC.Books {
		// The table of contents is keyed by source volume, alg-i-iii, and the
		// corpus by Book of the Éléments, alg. Algebra is one Book printed as
		// three volumes and a section belongs to the Book, so the volume is
		// translated here and not carried into the table, where a row per volume
		// would split chapters I to III from chapter VIII of the same work.
		work := bt.ID
		if b, ok := c.Books.Get(bt.ID); ok && b.Book != "" {
			work = b.Book
		}
		for _, ch := range bt.Chapters {
			key := work + "/" + ch.Numeral
			// A chapter can be registered twice, because the library holds two
			// printings of it: chapter VIII of Algebra is here in English and
			// in French, and both are read into the table of contents. It is
			// one chapter of one Book either way, so it gets one row, and the
			// fuller listing of the two says how many § it has.
			if r, ok := byKey[key]; ok {
				if n := coverageWant(ch); n > r.want {
					r.want = n
				}
				continue
			}
			n, _ := corpus.RomanOrder(ch.Numeral)
			byKey[key] = &coverageRow{
				book: work, bookTitle: corpus.BookTitle(work),
				chapter: ch.Numeral, order: n, want: coverageWant(ch),
			}
			order = append(order, key)
		}
	}
	for _, d := range c.Docs {
		if d.Lang != "en" {
			continue
		}
		switch {
		case d.Section != nil:
			r, ok := byKey[d.Section.Book+"/"+d.Section.Chapter]
			if !ok {
				continue
			}
			// front matter and the historical note are files like any other and
			// carry statements like any other, but neither is a § and neither is
			// in the table of contents' section list, so neither counts against
			// what the chapter is meant to hold.
			switch {
			case d.Section.Appendix, d.Section.Kind == "", d.Section.Kind == corpus.KindSection:
				r.have++
			}
			r.statements += d.Section.Statements
		case d.Exercise != nil:
			if r, ok := byKey[d.Exercise.Book+"/"+d.Exercise.Chapter]; ok {
				r.exercises++
			}
		}
	}
	for lang, items := range c.Items {
		if lang != "en" {
			continue
		}
		for _, it := range items {
			if it.Tag == "" {
				continue
			}
			ref, err := corpus.ParseLabel(it.Label)
			if err != nil {
				continue
			}
			if r, ok := byKey[ref.Book+"/"+strings.ToUpper(ref.Chapter)]; ok {
				r.tagged++
			}
		}
	}
	for _, b := range c.Books.Books {
		for _, p := range c.Pages[b.ID] {
			ch := chapterOfPage(c, b.ID, p.Meta.PDFPage)
			if r, ok := byKey[b.Book+"/"+ch]; ok && ch != "" {
				r.pages++
			}
		}
	}
	out := make([]coverageRow, 0, len(order))
	for _, k := range order {
		out = append(out, *byKey[k])
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].book != out[j].book {
			return out[i].book < out[j].book
		}
		return out[i].order < out[j].order
	})
	return out
}

// coverageWant is how many units of work a chapter is, which is its §§ and its
// appendices.
//
// Chapter I of the English Integration prints no § at all and owns its three
// nos itself, so counting its §§ makes it 0 of 0, which reads as a chapter with
// nothing in it and nothing to do. It is one chapter of real content, and it
// counts as one.
func coverageWant(ch corpus.Chapter) int {
	if len(ch.Sections) == 0 && len(ch.Subsections) > 0 {
		return 1
	}
	return len(ch.Sections)
}

// chapterOfPage is which chapter a PDF page falls in, off the page map.
func chapterOfPage(c *Corpus, book string, page int) string {
	m, ok := c.Maps[book]
	if !ok {
		return ""
	}
	if e, ok := m.Lookup(page); ok {
		return e.Chapter
	}
	return ""
}
