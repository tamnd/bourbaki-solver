package quality

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The library table is the answer to what this project is for, and its numbers
// are sums over manifests/books.yaml. Two of them are per language, because the
// interesting fact about the Éléments in this corpus is how much of it Springer
// never translated.
func TestTheLibraryTableCountsWhatIsRegistered(t *testing.T) {
	c := &Corpus{Books: &corpus.BooksManifest{Books: []corpus.Book{
		{ID: "alg-viii", Book: "alg", Lang: "en", Title: "Algebra, Chapter 8",
			Chapters: []string{"VIII"}, Pages: 505},
		{ID: "alg-viii-fr", Book: "alg", Lang: "fr", Title: "Algèbre, Chapitre 8",
			Chapters: []string{"VIII"}, Pages: 487},
		{ID: "alg-ix-fr", Book: "alg", Lang: "fr", Title: "Algèbre, Chapitre 9",
			Chapters: []string{"IX"}, Pages: 207},
	}}}
	got := Library(c)
	for _, want := range []string{
		"All one Books",              // one Book is registered, and it says so
		"3 volumes, 1199 pages",      // every volume, every page
		"1 volumes and 505 pages ar", // the English side
		"2 volumes and 694 pages ar", // the French side
		"| Algebra | VIII | VIII to IX | 3 | 1199 |",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the library table is missing %q:\n%s", want, got)
		}
	}
}

// A Book printed in one language and not the other says so, and it says it in
// the column that has nothing in it rather than by leaving the row out.
func TestTheLibraryTableSaysWhichPrintingIsMissing(t *testing.T) {
	c := &Corpus{Books: &corpus.BooksManifest{Books: []corpus.Book{
		{ID: "ta-i-iv-fr", Book: "ta", Lang: "fr", Title: "Topologie algébrique, Chapitres 1 à 4",
			Chapters: []string{"I", "II", "III", "IV"}, Pages: 512},
	}}}
	if got := Library(c); !strings.Contains(got, "| Topologie algébrique | none held | I to IV | 1 | 512 |") {
		t.Errorf("the row does not say the English is not held:\n%s", got)
	}
}

// The two volumes that list no chapters list none for opposite reasons, and the
// difference is in the title. The history is not divided into chapters and is
// the whole of its Book; the fascicule de résultats is the summary of a Book
// whose chapters were never published.
func TestAVolumeWithNoChaptersIsReadOffItsTitle(t *testing.T) {
	for _, c := range []struct{ title, want string }{
		{"Elements of the History of Mathematics", "whole"},
		{"Variétés différentielles et analytiques, fascicule de résultats", "fascicule de résultats"},
		{"Algebra, Chapter 8", "VIII"},
	} {
		b := corpus.Book{Title: c.title}
		if c.want == "VIII" {
			b.Chapters = []string{"VIII"}
		}
		if got := chapterCell(b); got != c.want {
			t.Errorf("chapterCell(%q) is %q, want %q", c.title, got, c.want)
		}
	}
}

// A Book is shelved as several volumes and the column is one cell, so the
// chapters are folded into runs. A hole in the run is the thing worth seeing,
// so it is not smoothed over into a range that claims chapters nobody holds.
func TestChapterRangesFoldIntoRunsAndKeepTheirHoles(t *testing.T) {
	for _, c := range []struct {
		cells []string
		want  string
	}{
		{nil, "none held"},
		{[]string{"VIII"}, "VIII"},
		{[]string{"I II III", "IV V VI VII", "VIII"}, "I to VIII"},
		{[]string{"I II III", "VIII"}, "I to III, VIII"},
		{[]string{"IX", "I", "II III"}, "I to III, IX"},
		{[]string{"fascicule de résultats"}, "fascicule de résultats"},
	} {
		if got := chapterRange(c.cells); got != c.want {
			t.Errorf("chapterRange(%v) is %q, want %q", c.cells, got, c.want)
		}
	}
}

// The text layer decides what a volume costs to read, so the block names the
// cheap ones and the expensive ones rather than only counting them. A volume is
// named as it is titled, because Algebra Chapter 8 and Algèbre Chapitre 8 are
// two files and a reader has to know which is meant.
func TestTheTextLayerBlockNamesTheVolumesAtEachEnd(t *testing.T) {
	c := &Corpus{Books: &corpus.BooksManifest{Books: []corpus.Book{
		{Book: "alg", Lang: "en", Title: "Algebra, Chapter 8", TextLayer: "native", Pages: 505},
		{Book: "alg", Lang: "fr", Title: "Algèbre, Chapitre 8", TextLayer: "native", Pages: 487},
		{Book: "alg", Lang: "fr", Title: "Algèbre, Chapitre 10", TextLayer: "none", Pages: 222},
		{Book: "top", Lang: "en", Title: "General Topology, Chapters 1-4", TextLayer: "ocr", Pages: 437},
	}}}
	got := TextLayer(c)
	for _, want := range []string{
		"| native | 2 |",
		"| ocr | 1 |",
		"| none | 1 |",
		"The two native volumes are *Algebra, Chapter 8* in English, and *Algèbre, Chapitre 8* in French.",
		"The one with no text at all is *Algèbre, Chapitre 10* at 222 pages, and it is the most expensive volume in the library.",
		"The other one is the ordinary case",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the text layer block is missing %q:\n%s", want, got)
		}
	}
}

// H06 has to say which block went stale, because they move for different
// reasons and the fix is the same command either way but the surprise is not.
func TestStaleREADMENamesTheBlockThatMoved(t *testing.T) {
	c := &Corpus{
		Books: &corpus.BooksManifest{Books: []corpus.Book{
			{ID: "alg-viii", Book: "alg", Lang: "en", Title: "Algebra, Chapter 8",
				Chapters: []string{"VIII"}, TextLayer: "native", Pages: 505},
		}},
		TOC: &corpus.TOCManifest{Books: []corpus.BookTOC{{
			ID: "alg-viii", Chapters: []corpus.Chapter{{Numeral: "VIII"}},
		}}},
	}
	// Everything present and correct except the library, which says two
	// volumes where the manifest holds one.
	var readme strings.Builder
	for _, b := range READMEBlocks() {
		block := b.Render(c)
		if b.Name == "LIBRARY" {
			block = strings.Replace(block, "1 volumes", "2 volumes", 1)
		}
		readme.WriteString(BeginMarker(b.Name) + block + EndMarker(b.Name) + "\n")
	}
	stale, missing := StaleREADME(c, readme.String())
	if len(missing) != 0 {
		t.Errorf("blocks reported missing that are there: %v", missing)
	}
	if len(stale) != 1 || stale[0] != "LIBRARY" {
		t.Errorf("stale blocks are %v, want just LIBRARY", stale)
	}
}

// A block whose markers are gone is not a block that passes. It is a number
// that has quietly gone back to being typed in by hand, which is the state the
// markers exist to get out of.
func TestAMissingBlockIsAFindingAndNotAPass(t *testing.T) {
	c := &Corpus{
		Books: &corpus.BooksManifest{},
		TOC:   &corpus.TOCManifest{},
	}
	stale, missing := StaleREADME(c, "# bourbaki\n\nnothing generated in here at all\n")
	if len(stale) != 0 {
		t.Errorf("blocks reported stale that are not in the file: %v", stale)
	}
	if len(missing) != len(READMEBlocks()) {
		t.Errorf("missing blocks are %v, want all %d of them", missing, len(READMEBlocks()))
	}
}

// Round trip. Writing the blocks into a README and reading it back has to leave
// nothing stale, or the check and the writer disagree and the build is red with
// no way to make it green.
func TestWritingTheBlocksLeavesNothingStale(t *testing.T) {
	c := &Corpus{
		Books: &corpus.BooksManifest{Books: []corpus.Book{
			{ID: "alg-viii", Book: "alg", Lang: "en", Title: "Algebra, Chapter 8",
				Chapters: []string{"VIII"}, TextLayer: "native", Pages: 505},
		}},
		TOC: &corpus.TOCManifest{Books: []corpus.BookTOC{{
			ID: "alg-viii", Chapters: []corpus.Chapter{{Numeral: "VIII"}},
		}}},
	}
	root := t.TempDir()
	var readme strings.Builder
	readme.WriteString("# bourbaki\n\nprose that is not generated and must survive\n\n")
	for _, b := range READMEBlocks() {
		readme.WriteString(BeginMarker(b.Name) + "\n" + EndMarker(b.Name) + "\n\nmore prose\n\n")
	}
	path := filepath.Join(root, "README.md")
	if err := os.WriteFile(path, []byte(readme.String()), 0o644); err != nil {
		t.Fatal(err)
	}

	changed, err := WriteREADME(root, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != len(READMEBlocks()) {
		t.Errorf("wrote %v, want every block", changed)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	out := string(raw)
	if !strings.Contains(out, "prose that is not generated and must survive") {
		t.Error("the writer ate the prose around the blocks")
	}
	if stale, missing := StaleREADME(c, out); len(stale) != 0 || len(missing) != 0 {
		t.Errorf("after writing, stale %v missing %v", stale, missing)
	}
	// And again, with nothing to do.
	changed, err = WriteREADME(root, c)
	if err != nil {
		t.Fatal(err)
	}
	if len(changed) != 0 {
		t.Errorf("the second run rewrote %v, and nothing had moved", changed)
	}
}

func TestNumbersAreSpeltOutWhileTheyReadBetterAsWords(t *testing.T) {
	for _, c := range []struct {
		n    int
		want string
	}{{0, "zero"}, {1, "one"}, {12, "twelve"}, {20, "twenty"}, {21, "21"}, {43, "43"}} {
		if got := numberWord(c.n); got != c.want {
			t.Errorf("numberWord(%d) is %q, want %q", c.n, got, c.want)
		}
	}
}

func TestListsAreWrittenTheWayASentenceWritesThem(t *testing.T) {
	for _, c := range []struct {
		in   []string
		want string
	}{
		{nil, ""},
		{[]string{"a"}, "a"},
		{[]string{"a", "b"}, "a and b"},
		{[]string{"a", "b", "c"}, "a, b and c"},
	} {
		if got := joinList(c.in); got != c.want {
			t.Errorf("joinList(%v) is %q, want %q", c.in, got, c.want)
		}
	}
}
