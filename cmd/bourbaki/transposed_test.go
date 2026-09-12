package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// boundCorpus is one chapter whose last two leaves are bound the wrong way
// round, which is the shape of the fault in Algebre chapitres 4 a 7 in French.
//
// The exercises run off the foot of pdf 20 and finish on pdf 22, and the
// historical note opens on pdf 21 and finishes on pdf 23, so the printing reads
// 20, 22, 21, 23 where the file holds 20, 21, 22, 23. Read in file order the
// exercise stops a sentence short and the note opens in the middle of it, and
// everything checks out: the pages are all there and all of them assemble.
//
// swap says whether the volume declares the transposition, so that one corpus
// serves both the fix and the fault it was written for.
func boundCorpus(t *testing.T, swap bool) string {
	t.Helper()
	root := t.TempDir()

	b := corpus.Book{
		ID: "alg-viii", Book: "alg", Lang: "en", Title: "Algebra, Chapter 8",
		Edition: "2023, Springer Nature", Chapters: []string{"VIII"}, Pages: 24,
		Nature: "digital", Extraction: "native",
	}
	if swap {
		b.Transposed = []corpus.Transposition{{
			Pages: []int{21, 22},
			Why: "pdf 22 heads A VIII.5 and ends the exercises of § 1, and pdf 21 opens the " +
				"historical note at printed 6 and runs straight into pdf 23",
		}}
	}
	books := &corpus.BooksManifest{}
	books.Upsert(b)
	if err := books.Save(root); err != nil {
		t.Fatal(err)
	}

	toc := &corpus.TOCManifest{}
	toc.Upsert(corpus.BookTOC{ID: "alg-viii", Grammar: "head-label", Chapters: []corpus.Chapter{{
		Book: "alg-viii", Numeral: "VIII", Title: "Semisimple Modules and Rings", Page: 1, PDFPage: 18,
		Sections: []corpus.Section{{
			Number: 1, Title: "Artinian Modules and Noetherian Modules", Page: 1, PDFPage: 18,
			Subsections: []corpus.Subsection{{Number: 1, Title: "Artinian Modules", Page: 1, PDFPage: 18}},
			Exercises:   &corpus.Locator{Page: 3, PDFPage: 20},
		}},
		// The note opens on the leaf the file numbers 21, which is where a
		// reader who turned to it would find it, and the fifth page of the
		// printing.
		Historical: &corpus.Locator{Page: 6, PDFPage: 21},
	}}})
	if err := toc.Save(root); err != nil {
		t.Fatal(err)
	}

	pages := []struct {
		n         int
		label     int
		body      string
		continues bool
	}{
		{n: 18, label: 1, body: "## CHAPTER VIII SEMISIMPLE MODULES AND RINGS\n\n" +
			"## § 1. ARTINIAN MODULES AND NOETHERIAN MODULES\n\n" +
			"### 1. Artinian Modules\n\n" +
			"**Definition 1.** — An A-module M is said to be Artinian if every nonempty set of submodules has a minimal element."},
		{n: 19, label: 2, body: "**Proposition 1.** — Let A be a ring. The ring A is left Artinian."},
		{n: 20, label: 3, body: "### Exercises\n\n" +
			"1) Let A be a ring. Show that A is left Artinian.\n\n" +
			"2) Let K be a field and V a K-vector space of infinite"},
		{n: 21, label: 6, body: "# HISTORICAL NOTE\n\n" +
			"The theory of semisimple rings begins with Molien and Wedderburn,"},
		{n: 22, label: 5, continues: true, body: "dimension. Show that V is neither Artinian nor Noetherian."},
		{n: 23, label: 7, continues: true, body: "and was completed by Artin."},
		{n: 24, body: "# BIBLIOGRAPHY"},
	}
	if err := os.MkdirAll(corpus.PagesDir(root, "alg-viii"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, p := range pages {
		f := corpus.PageFile{Meta: corpus.PageFrontMatter{
			Book: "alg-viii", PDFPage: p.n, Method: corpus.MethodNative, Continues: p.continues,
		}, Body: p.body}
		if p.label > 0 {
			f.Meta.PageLabel = corpus.PageLabel{Book: "A", Chapter: "VIII", Page: p.label}.String()
		}
		out, err := f.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(corpus.PagePath(root, "alg-viii", p.n), out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// A volume that declares its transposed leaves is read in printing order: the
// exercise finishes on the leaf that finishes it and the note opens on the leaf
// that opens it, and neither one has any of the other in it.
//
// The pdf pages written down are still the pdf pages. The note is printed on
// leaves 21 and 23 of the file and says so, which is what a reader turning to it
// needs and what fix opening renders from.
func TestAssembleReadsATransposedVolumeInPrintingOrder(t *testing.T) {
	root := boundCorpus(t, true)
	t.Setenv("BOURBAKI_CORPUS", root)
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "content", "en", "alg", "VIII")

	note, err := corpus.ReadFile[corpus.SectionFrontMatter](filepath.Join(dir, "historical_note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if note.Meta.Kind != corpus.KindHistorical {
		t.Errorf("the note is kind %q", note.Meta.Kind)
	}
	if !strings.Contains(note.Body, "Molien and Wedderburn, and was completed by Artin.") {
		t.Errorf("the note does not run on into the leaf that finishes it:\n%s", note.Body)
	}
	if strings.Contains(note.Body, "Artinian nor Noetherian") {
		t.Errorf("the note carries the end of the exercise:\n%s", note.Body)
	}
	if note.Meta.PDFPages != "0021-0023" {
		t.Errorf("the note covers %q, want the leaves it is printed on", note.Meta.PDFPages)
	}

	ex, err := corpus.ReadFile[corpus.ExerciseFrontMatter](filepath.Join(dir, "exercises", "s1", "02.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(ex.Body, "infinite dimension. Show that V is neither Artinian nor Noetherian.") {
		t.Errorf("exercise 2 stops short of the leaf that finishes it:\n%s", ex.Body)
	}
	if strings.Contains(ex.Body, "Molien") {
		t.Errorf("exercise 2 swallowed the historical note:\n%s", ex.Body)
	}
	if ex.Meta.PDFPage != 20 {
		t.Errorf("exercise 2 opens on pdf page %d, want the leaf it is printed on", ex.Meta.PDFPage)
	}
}

// The same volume with the transposition taken out of the manifest, which is
// what assembly did with it for as long as it read the pages in file order.
// The note opens in the middle of an exercise, the exercise stops a sentence
// short, and nothing anywhere says so.
func TestAssembleWithoutTheTranspositionRunsTheLeavesTogether(t *testing.T) {
	root := boundCorpus(t, false)
	t.Setenv("BOURBAKI_CORPUS", root)
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "content", "en", "alg", "VIII")
	note, err := corpus.ReadFile[corpus.SectionFrontMatter](filepath.Join(dir, "historical_note.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(note.Body, "Artinian nor Noetherian") {
		t.Fatalf("the fault this guards against did not reproduce:\n%s", note.Body)
	}
}

func TestBoundOrderRefusesAPageTransposedTwice(t *testing.T) {
	_, err := boundOrder(corpus.Book{ID: "mini", Pages: 30, Transposed: []corpus.Transposition{
		{Pages: []int{21, 22}, Why: "the leaves are bound the wrong way round"},
		{Pages: []int{22, 23}, Why: "and this one says so again"},
	}})
	if err == nil {
		t.Fatal("a page transposed twice was let through")
	}
}

func TestBoundOrderSendsALocatorBothWays(t *testing.T) {
	o, err := boundOrder(corpus.Book{ID: "mini", Pages: 30, Transposed: []corpus.Transposition{
		{Pages: []int{21, 22}, Why: "the leaves are bound the wrong way round"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	for _, c := range []struct{ in, want int }{{20, 20}, {21, 22}, {22, 21}, {23, 23}} {
		if got := o.at(c.in); got != c.want {
			t.Errorf("at(%d) = %d, want %d", c.in, got, c.want)
		}
		if got := o.at(o.at(c.in)); got != c.in {
			t.Errorf("at(at(%d)) = %d, want the page back: the permutation is its own inverse", c.in, got)
		}
	}
	in := []corpus.Chapter{{Numeral: "VIII", PDFPage: 18,
		Historical: &corpus.Locator{Page: 6, PDFPage: 21},
		Sections: []corpus.Section{{Number: 1, PDFPage: 18,
			Subsections: []corpus.Subsection{{Number: 1, PDFPage: 22}},
			Exercises:   &corpus.Locator{Page: 3, PDFPage: 20}}}}}
	out := o.chapters(in)
	if out[0].Historical.PDFPage != 22 || out[0].Sections[0].Subsections[0].PDFPage != 21 {
		t.Errorf("the contents came through as %+v", out[0])
	}
	if in[0].Historical.PDFPage != 21 || in[0].Sections[0].Subsections[0].PDFPage != 22 {
		t.Error("the contents the caller loaded was written through")
	}
}
