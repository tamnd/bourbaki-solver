package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/assemble"
	"github.com/tamnd/bourbaki-solver/corpus"
)

// A corpus of one volume, one chapter, one §, three pages. It is the smallest
// thing the stage can be run against end to end, and it is run against the same
// three files a real run is: manifests/books.yaml, manifests/toc.yaml and
// pages/.
func smallCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	books := &corpus.BooksManifest{}
	books.Upsert(corpus.Book{
		ID: "alg-viii", Book: "alg", Title: "Algebra, Chapter 8", Edition: "2023, Springer Nature",
		Chapters: []string{"VIII"}, Pages: 4, Nature: "digital", Extraction: "native",
	})
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
	}}})
	if err := toc.Save(root); err != nil {
		t.Fatal(err)
	}

	bodies := map[int]string{
		18: "## CHAPTER VIII SEMISIMPLE MODULES AND RINGS\n\n" +
			"## § 1. ARTINIAN MODULES AND NOETHERIAN MODULES\n\n" +
			"### 1. Artinian Modules\n\n" +
			"**Definition 1.** — An A-module M is said to be Artinian if every nonempty set of submodules has a minimal element.",
		19: "**Proposition 1.** — Let A be a ring. The ring A is left Artinian.",
		20: "### Exercises\n\n1) Let A be a ring. Show that A is left Artinian.",
		21: "# BIBLIOGRAPHY",
	}
	dir := corpus.PagesDir(root, "alg-viii")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for n, body := range bodies {
		f := corpus.PageFile{Meta: corpus.PageFrontMatter{
			Book: "alg-viii", PDFPage: n, Method: corpus.MethodNative,
		}, Body: body}
		if n < 21 {
			f.Meta.PageLabel = corpus.PageLabel{Book: "A", Chapter: "VIII", Page: n - 17}.String()
		}
		out, err := f.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(corpus.PagePath(root, "alg-viii", n), out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRunAssemble(t *testing.T) {
	root := smallCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "content", "en", "alg", "VIII")
	names, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("wrote %v, want the front matter and § 1", names)
	}
	sec, err := corpus.ReadFile[corpus.SectionFrontMatter](
		filepath.Join(dir, "01_s1_artinian_modules_and_noetherian_modules.md"))
	if err != nil {
		t.Fatal(err)
	}
	if sec.Meta.BookTitle != "Algebra" {
		t.Errorf("book_title = %q, want the name of the Book and not of the volume", sec.Meta.BookTitle)
	}
	if sec.Meta.Statements != 2 || sec.Meta.Exercises != 1 {
		t.Errorf("§ 1 reports %d statements and %d exercises", sec.Meta.Statements, sec.Meta.Exercises)
	}
	if sec.Meta.PDFPages != "0018-0020" || sec.Meta.BookPages != "A VIII.1-A VIII.3" {
		t.Errorf("§ 1 covers %q, %q", sec.Meta.PDFPages, sec.Meta.BookPages)
	}
	if sec.Meta.ContentSHA256 != corpus.ContentSHA256(sec.Body) {
		t.Error("the file does not hash to what its front matter says")
	}

	m, err := corpus.LoadSections(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Books) != 1 || len(m.Books[0].Chapters) != 1 {
		t.Fatalf("the manifest is %+v", m)
	}
	recs := m.Books[0].Chapters[0].Sections
	if len(recs) != 2 || recs[0].Kind != corpus.KindFront || recs[1].Kind != corpus.KindSection {
		t.Fatalf("the manifest records %+v", recs)
	}
	if recs[1].Label != "alg-viii-s1" || recs[1].Path != "content/en/alg/VIII/01_s1_artinian_modules_and_noetherian_modules.md" {
		t.Errorf("§ 1 is recorded as %+v", recs[1])
	}

	// The whole stage is meant to be a pure function of the pages and the
	// contents, and -check is how that is known rather than hoped.
	if err := runAssemble([]string{"-book", "alg-viii", "-check", "-q"}); err != nil {
		t.Fatalf("a second run differs from the first: %v", err)
	}
}

// A section file is named for its title, so correcting a title renames the
// file. The old one has to go, or it sits there for ever looking like a section
// of the book.
func TestRunAssembleRemovesAFileItDidNotWrite(t *testing.T) {
	root := smallCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	dir := filepath.Join(root, "content", "en", "alg", "VIII")
	if err := os.MkdirAll(filepath.Join(dir, "exercises", "s1"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "01_s1_artinian_modules.md")
	if err := os.WriteFile(old, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// The exercises belong to the next stage and are not this one's to sweep.
	ex := filepath.Join(dir, "exercises", "s1", "01.md")
	if err := os.WriteFile(ex, []byte("exercise\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("the file with the old name is still there")
	}
	if _, err := os.Stat(ex); err != nil {
		t.Errorf("the exercise file was swept: %v", err)
	}
}

// -check writes nothing and says what differs. It is what CI runs, and CI has
// no PDFs and nothing to compare against but the repository.
func TestRunAssembleCheckReportsADifference(t *testing.T) {
	root := smallCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "content", "en", "alg", "VIII",
		"01_s1_artinian_modules_and_noetherian_modules.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(before, []byte("\nan edit nobody made upstream\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	err = runAssemble([]string{"-book", "alg-viii", "-check", "-q"})
	if err == nil {
		t.Fatal("an edited file should be reported")
	}
	if !strings.Contains(err.Error(), "01_s1_artinian_modules_and_noetherian_modules.md") {
		t.Errorf("the error does not name the file: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) == len(before) {
		t.Error("-check wrote the file back")
	}
}

// Two page files claiming the same PDF page means one of them is a leftover,
// and assembling both of them would put the page in twice.
func TestReadPagesRefusesTwoFilesForOnePage(t *testing.T) {
	root := smallCorpus(t)
	f := corpus.PageFile{Meta: corpus.PageFrontMatter{
		Book: "alg-viii", PDFPage: 19, Method: corpus.MethodNative,
	}, Body: "a second reading of page 19"}
	out, err := f.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corpus.PagesDir(root, "alg-viii"), "0019_old.md"), out, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPages(root, "alg-viii"); err == nil {
		t.Fatal("two files for one page should be an error")
	}
}

func TestPageRange(t *testing.T) {
	cases := []struct{ first, last, want string }{
		{"A VIII.1", "A VIII.23", "A VIII.1-A VIII.23"},
		{"A VIII.7", "A VIII.7", "A VIII.7"},
		{"", "A VIII.23", ""},
		{"A VIII.1", "", ""},
	}
	for _, c := range cases {
		if got := pageRange(c.first, c.last); got != c.want {
			t.Errorf("pageRange(%q, %q) = %q, want %q", c.first, c.last, got, c.want)
		}
	}
}

func TestKindOfAndLabelOf(t *testing.T) {
	cases := []struct {
		piece assemble.Piece
		kind  string
		label string
	}{
		{assemble.Piece{Front: true}, corpus.KindFront, ""},
		{assemble.Piece{Historical: true}, corpus.KindHistorical, ""},
		{assemble.Piece{Section: corpus.Section{Number: 3}}, corpus.KindSection, "alg-viii-s3"},
		{assemble.Piece{Section: corpus.Section{Number: 2, Appendix: true}}, corpus.KindAppendix, "alg-viii-a2"},
	}
	for _, c := range cases {
		if got := kindOf(c.piece); got != c.kind {
			t.Errorf("kindOf(%s) = %q, want %q", c.piece.Name(), got, c.kind)
		}
		if got := labelOf("alg", "VIII", c.piece); got != c.label {
			t.Errorf("labelOf(%s) = %q, want %q", c.piece.Name(), got, c.label)
		}
	}
}
