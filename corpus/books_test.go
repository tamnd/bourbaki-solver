package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestBooksRoundTrip(t *testing.T) {
	root := t.TempDir()

	m, err := LoadBooks(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Books) != 0 {
		t.Fatal("a fresh corpus should have an empty manifest, not an error")
	}

	// Added out of order on purpose: the manifest sorts by first chapter.
	m.Upsert(Book{ID: "alg-viii", Book: "alg", Chapters: []string{"VIII"},
		Pages: 505, Nature: "born-digital", Extraction: "native"})
	m.Upsert(Book{ID: "alg-i-iii", Book: "alg", Chapters: []string{"I", "II", "III"},
		Pages: 734, Nature: "scanned", Extraction: "ocr",
		Scan: &Scan{Format: "jbig2", Width: 3026, Height: 4713, DPI: 600, BPC: 1, Color: "gray"}})
	m.Upsert(Book{ID: "alg-iv-vii", Book: "alg", Chapters: []string{"IV", "V", "VI", "VII"},
		Pages: 460, Nature: "scanned", Extraction: "ocr"})

	if got := m.Pages(); got != 1699 {
		t.Errorf("Pages() = %d, want 1699", got)
	}
	if err := m.Save(root); err != nil {
		t.Fatal(err)
	}

	back, err := LoadBooks(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"alg-i-iii", "alg-iv-vii", "alg-viii"}
	for i, id := range want {
		if back.Books[i].ID != id {
			t.Errorf("book %d is %q, want %q, so the sort is not by chapter order", i, back.Books[i].ID, id)
		}
	}
	b, ok := back.Get("alg-i-iii")
	if !ok {
		t.Fatal("alg-i-iii went missing")
	}
	if b.Scan == nil || b.Scan.DPI != 600 || b.Scan.Format != "jbig2" {
		t.Errorf("scan geometry did not survive the round trip: %+v", b.Scan)
	}
	if _, ok := back.Get("alg-ix"); ok {
		t.Error("Get returned a book that is not there")
	}
}

// The manifest holds two printings of nine Books, so first chapter alone is no
// longer an order. A reader looking for Algèbre commutative chapter 10 expects
// it after chapters 1 to 4 of the same printing, and expects the whole of the
// Book Algebra before the whole of Commutative Algebra.
func TestSaveShelvesByBookThenLanguageThenChapter(t *testing.T) {
	root := t.TempDir()
	m := &BooksManifest{}
	for _, b := range []Book{
		{ID: "ac-x-fr", Book: "ac", Lang: "fr", Chapters: []string{"X"}},
		{ID: "hist", Book: "hist", Lang: "en"},
		{ID: "alg-i-iii-fr", Book: "alg", Lang: "fr", Chapters: []string{"I", "II", "III"}},
		{ID: "ac-i-iv-fr", Book: "ac", Lang: "fr", Chapters: []string{"I", "II", "III", "IV"}},
		{ID: "alg-viii", Book: "alg", Lang: "en", Chapters: []string{"VIII"}},
		{ID: "alg-i-iii", Book: "alg", Lang: "en", Chapters: []string{"I", "II", "III"}},
		{ID: "ens-i-iv", Book: "ens", Lang: "en", Chapters: []string{"I", "II", "III", "IV"}},
		{ID: "var-fr", Book: "var", Lang: "fr"},
	} {
		m.Upsert(b)
	}
	if err := m.Save(root); err != nil {
		t.Fatal(err)
	}
	back, err := LoadBooks(root)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"ens-i-iv", "alg-i-iii", "alg-viii", "alg-i-iii-fr",
		"ac-i-iv-fr", "ac-x-fr", "var-fr", "hist"}
	var got []string
	for _, b := range back.Books {
		got = append(got, b.ID)
	}
	if strings.Join(got, " ") != strings.Join(want, " ") {
		t.Errorf("shelved as\n %v\nwant\n %v", got, want)
	}
}

func TestBookTitleFallsBackToTheSlug(t *testing.T) {
	if got := BookTitle("ts"); got != "Théories spectrales" {
		t.Errorf("BookTitle(ts) = %q", got)
	}
	if got := BookTitle("nobody"); got != "nobody" {
		t.Errorf("BookTitle of an unnamed Book = %q, want the slug back", got)
	}
}

// The letters are the table the Éléments print on page vii of every recent
// volume, so this test is against the book and not against the map. The last
// two cases are the ones that cost something: a Book with no letter must give
// back nothing rather than a guess, and no two Books may share a letter, since
// a page label is the letter and the number and nothing else.
func TestBookLetterIsWhatTheElementsPrint(t *testing.T) {
	for book, want := range map[string]string{
		"ens":  "E",
		"alg":  "A",
		"top":  "TG",
		"lie":  "LIE",
		"ts":   "TS",
		"ta":   "TA",
		"hist": "",
		"":     "",
	} {
		if got := BookLetter(book); got != want {
			t.Errorf("BookLetter(%q) = %q, want %q", book, got, want)
		}
	}
	seen := map[string]string{}
	for book, letter := range bookLetters {
		if other, ok := seen[letter]; ok {
			t.Errorf("%s and %s are both cited %q, so their page labels collide", book, other, letter)
		}
		seen[letter] = book
	}
	// Every Book with a letter is a Book the corpus knows the name of, and
	// every abbreviated Book is shelved, so a new volume cannot arrive with a
	// letter and no place to stand.
	for book := range bookLetters {
		if _, ok := bookTitles[book]; !ok {
			t.Errorf("%s is cited by a letter and has no title", book)
		}
		if _, ok := bookOrder[book]; !ok {
			t.Errorf("%s is cited by a letter and is not shelved", book)
		}
	}
}

func TestUpsertReplaces(t *testing.T) {
	m := &BooksManifest{}
	m.Upsert(Book{ID: "alg-viii", Pages: 1})
	m.Upsert(Book{ID: "alg-viii", Pages: 505})
	if len(m.Books) != 1 {
		t.Fatalf("got %d books, want 1", len(m.Books))
	}
	if m.Books[0].Pages != 505 {
		t.Errorf("Pages = %d, want the second write to win", m.Books[0].Pages)
	}
}

func TestSaveWritesAGeneratedHeader(t *testing.T) {
	root := t.TempDir()
	m := &BooksManifest{Books: []Book{{ID: "alg-viii", Chapters: []string{"VIII"}}}}
	if err := m.Save(root); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(BooksPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "# Generated by bourbaki books add") {
		t.Errorf("manifest should say it is generated, got %q", string(b[:40]))
	}
}

func TestLoadBooksRejectsBadYAML(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "manifests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(BooksPath(root), []byte("books: [oops\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadBooks(root); err == nil {
		t.Error("a malformed manifest should be an error")
	}
}

func TestRootPrefersTheEnvironment(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("BOURBAKI_CORPUS", dir)
	got, err := Root()
	if err != nil {
		t.Fatal(err)
	}
	// t.TempDir can hand back a symlinked path on darwin, so compare the base.
	if filepath.Base(got) != filepath.Base(dir) {
		t.Errorf("Root() = %q, want %q", got, dir)
	}
}
