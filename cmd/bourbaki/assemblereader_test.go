package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// readerCorpus is smallCorpus with three pages of the publisher's note to the
// reader in front of it and an introduction after them, which is how nearly
// every volume of the series is printed. The note carries its title on the
// first page and as the running head of the two after it.
func readerCorpus(t *testing.T) string {
	t.Helper()
	root := introCorpus(t)

	books, err := corpus.LoadBooks(root)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := books.Get("alg-viii")
	b.ReaderNote = &corpus.Introduction{
		Title: "TO THE READER", Page: 5, FirstPDFPage: 11, LastPDFPage: 13,
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
		{11, "", "# TO THE READER\n\n1. This series of volumes takes up mathematics at the beginning and gives\n\n5"},
		{12, "TO THE READER", "complete proofs. In principle it requires no particular knowledge of mathematics.\n\n6"},
		{13, "TO THE READER", "2. The method of exposition we have chosen is axiomatic and abstract.\n"},
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

// The note to the reader comes out as its own file beside the chapter
// directories, ahead of the introduction, with the running heads gone and the
// printed numbers off the feet. Ninety pages of it across twenty five volumes
// were read and then left out of the corpus, because the assembler walks the
// table of contents and the note is in no chapter.
func TestAssembleWritesTheNoteToTheReader(t *testing.T) {
	root := readerCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "content", "en", "alg", "00_to_the_reader.md")
	f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Meta.Kind != corpus.KindReader || f.Meta.Chapter != "" {
		t.Errorf("the note is kind %q of chapter %q", f.Meta.Kind, f.Meta.Chapter)
	}
	if f.Meta.SectionTitle != "TO THE READER" || f.Meta.PDFPages != "0011-0013" {
		t.Errorf("the front matter reads %+v", f.Meta)
	}
	if f.Meta.ContentSHA256 != corpus.ContentSHA256(f.Body) {
		t.Error("the file does not hash to what its front matter says")
	}
	if n := strings.Count(f.Body, "TO THE READER"); n != 1 {
		t.Errorf("the title is written %d times:\n%s", n, f.Body)
	}
	if !strings.Contains(f.Body, "at the beginning and gives complete proofs") {
		t.Errorf("the sentence broken across the page break was not joined:\n%s", f.Body)
	}
	for _, folio := range []string{"\n5\n", "\n6\n"} {
		if strings.Contains(f.Body, folio) {
			t.Errorf("a printed page number is still in the body:\n%s", f.Body)
		}
	}

	// The note and the introduction are two files and two entries, and the
	// note's entry comes first because the note is printed first.
	m, err := corpus.LoadSections(root)
	if err != nil {
		t.Fatal(err)
	}
	rec := m.Books[0].ReaderNote
	if rec == nil {
		t.Fatal("the manifest does not account for the note to the reader")
	}
	if rec.Path != "content/en/alg/00_to_the_reader.md" || rec.FirstPDFPage != 11 || rec.LastPDFPage != 13 {
		t.Errorf("the note is recorded as %+v", rec)
	}
	if m.Books[0].Introduction == nil {
		t.Error("writing the note to the reader dropped the introduction")
	}
}

// A Book printed as several volumes keeps them in one directory, so two volumes
// that each open with an introduction would both want 00_introduction.md.
// Theories spectrales is the case and books.yaml names the file for the second.
func TestAssembleHonoursTheNamedIntroductionFile(t *testing.T) {
	root := introCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)

	books, err := corpus.LoadBooks(root)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := books.Get("alg-viii")
	b.Introduction.File = "00_introduction_viii.md"
	books.Upsert(*b)
	if err := books.Save(root); err != nil {
		t.Fatal(err)
	}

	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	named := filepath.Join(root, "content", "en", "alg", "00_introduction_viii.md")
	if _, err := os.Stat(named); err != nil {
		t.Fatalf("the introduction is not under the name the manifest gives it: %v", err)
	}
	if _, err := os.Stat(filepath.Join(root, "content", "en", "alg", "00_introduction.md")); !os.IsNotExist(err) {
		t.Error("the introduction was also written under the default name")
	}
	m, err := corpus.LoadSections(root)
	if err != nil {
		t.Fatal(err)
	}
	if rec := m.Books[0].Introduction; rec == nil || rec.Path != "content/en/alg/00_introduction_viii.md" {
		t.Errorf("the manifest records %+v", rec)
	}
}
