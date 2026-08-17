package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// introCorpus is smallCorpus with three pages of introduction put in front of
// it, set the way the printing sets them: the title on the first page, the
// running head in the front matter of the second, the running head written into
// the body of the third because a model read that one page differently, and the
// printed number bare at the foot of two of them.
func introCorpus(t *testing.T) string {
	t.Helper()
	root := smallCorpus(t)

	books, err := corpus.LoadBooks(root)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := books.Get("alg-viii")
	b.Introduction = &corpus.Introduction{
		Title: "INTRODUCTION", Page: 7, FirstPDFPage: 15, LastPDFPage: 17,
	}
	books.Upsert(*b)
	if err := books.Save(root); err != nil {
		t.Fatal(err)
	}

	pages := []struct {
		n    int
		head string
		body string
	}{
		{15, "", "# INTRODUCTION\n\nEver since the Greeks, mathematics has involved proof; and it is doubted by some whether\n\n7"},
		{16, "INTRODUCTION", "proof in that sense is to be found outside mathematics. A proof is a chain of syllogisms.\n\n8"},
		{17, "", "# INTRODUCTION\n\nThe axiomatic method is no more than this.\n"},
	}
	for _, p := range pages {
		f := corpus.PageFile{Meta: corpus.PageFrontMatter{
			Book: "alg-viii", PDFPage: p.n, RunningHead: p.head, Method: corpus.MethodOCR,
		}, Body: p.body}
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

// The introduction comes out as one file beside the chapter directories, with
// the title written once, the running heads gone and the printed numbers off the
// feet. It is the part of the book that says what the rest is for, and it was
// missing from Theory of Sets for the whole of the read because the assembler
// walks the table of contents and it is in no chapter.
func TestAssembleWritesTheIntroduction(t *testing.T) {
	root := introCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "content", "en", "alg", "00_introduction.md")
	f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Meta.Kind != corpus.KindIntroduction || f.Meta.Chapter != "" {
		t.Errorf("the introduction is kind %q of chapter %q", f.Meta.Kind, f.Meta.Chapter)
	}
	if f.Meta.BookTitle != "Algebra" || f.Meta.SectionTitle != "INTRODUCTION" {
		t.Errorf("the front matter reads %+v", f.Meta)
	}
	if f.Meta.PDFPages != "0015-0017" || f.Meta.BookPages != "7-9" {
		t.Errorf("the introduction covers %q, %q", f.Meta.PDFPages, f.Meta.BookPages)
	}
	if f.Meta.Extraction != "ocr" {
		t.Errorf("extraction = %q", f.Meta.Extraction)
	}
	if f.Meta.ContentSHA256 != corpus.ContentSHA256(f.Body) {
		t.Error("the file does not hash to what its front matter says")
	}

	if n := strings.Count(f.Body, "INTRODUCTION"); n != 1 {
		t.Errorf("the title is written %d times:\n%s", n, f.Body)
	}
	if !strings.HasPrefix(f.Body, "## INTRODUCTION\n") {
		t.Errorf("the introduction opens %q", f.Body)
	}
	// The sentence broken by the end of page 7 is put back together, and the new
	// paragraph opening page 9 is left where it is.
	if !strings.Contains(f.Body, "by some whether proof in that sense is to be found") {
		t.Errorf("the sentence broken across the page break was not joined:\n%s", f.Body)
	}
	if !strings.Contains(f.Body, "syllogisms.\n\nThe axiomatic method") {
		t.Errorf("a paragraph of its own was glued onto the one before it:\n%s", f.Body)
	}
	// The folio is furniture. Seven pages of it would leave six bare numbers
	// standing between the paragraphs of the assembled file.
	for _, folio := range []string{"\n7\n", "\n8\n"} {
		if strings.Contains(f.Body, folio) {
			t.Errorf("a printed page number is still in the body:\n%s", f.Body)
		}
	}

	m, err := corpus.LoadSections(root)
	if err != nil {
		t.Fatal(err)
	}
	rec := m.Books[0].Introduction
	if rec == nil {
		t.Fatal("the manifest does not account for the introduction")
	}
	if rec.Path != "content/en/alg/00_introduction.md" || rec.FirstPDFPage != 15 || rec.LastPDFPage != 17 {
		t.Errorf("the introduction is recorded as %+v", rec)
	}
	if rec.ContentSHA256 != corpus.ContentSHA256(f.Body) {
		t.Error("the manifest hash is not the file's")
	}

	if err := runAssemble([]string{"-book", "alg-viii", "-check", "-q"}); err != nil {
		t.Fatalf("a second run differs from the first: %v", err)
	}
}

// A volume with no introduction gets no file and no record, which is every
// volume of the corpus but this one.
func TestAssembleWritesNoIntroductionWhereThereIsNone(t *testing.T) {
	root := smallCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "content", "en", "alg", "00_introduction.md")); !os.IsNotExist(err) {
		t.Errorf("an introduction was invented: %v", err)
	}
	m, err := corpus.LoadSections(root)
	if err != nil {
		t.Fatal(err)
	}
	if m.Books[0].Introduction != nil {
		t.Errorf("the manifest records %+v", m.Books[0].Introduction)
	}
}

// An introduction half read stops the run, the same as a chapter half read, and
// -partial passes over it the same way. Assembling three pages of seven and
// saying nothing would leave a file that looks whole and is not.
func TestAssembleHoldsAnIntroductionWithAHoleInIt(t *testing.T) {
	root := introCorpus(t)
	if err := os.Remove(corpus.PagePath(root, "alg-viii", 16)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BOURBAKI_CORPUS", root)
	err := runAssemble([]string{"-book", "alg-viii", "-q"})
	if err == nil {
		t.Fatal("the run assembled an introduction it had not read")
	}
	if !strings.Contains(err.Error(), "page 16 is not read yet") {
		t.Errorf("the run stopped with %v", err)
	}
	if err := runAssemble([]string{"-book", "alg-viii", "-partial", "-q"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "content", "en", "alg", "00_introduction.md")); !os.IsNotExist(err) {
		t.Errorf("a partial run wrote a partial introduction: %v", err)
	}
}

