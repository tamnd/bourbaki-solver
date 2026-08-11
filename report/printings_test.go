package report

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// section is one record of the manifest, written short because a comparison
// reads nothing off it but the label, the kind and the two counts.
func section(label string, statements, exercises int) corpus.SectionRecord {
	return corpus.SectionRecord{
		Kind:       corpus.KindSection,
		Label:      label,
		Statements: statements,
		Exercises:  exercises,
	}
}

func chapter(name string, sections ...corpus.SectionRecord) corpus.ChapterSections {
	return corpus.ChapterSections{Chapter: name, Sections: sections}
}

// shelf is the two manifests a comparison needs, built together so that a
// volume in one is a volume in the other.
func shelf(t *testing.T, books []corpus.Book, sections []corpus.BookSections) (*corpus.BooksManifest, *corpus.SectionsManifest) {
	t.Helper()
	return &corpus.BooksManifest{Books: books}, &corpus.SectionsManifest{Books: sections}
}

func TestThePrintingsOfAChapterArePairedByChapterAndNotByVolume(t *testing.T) {
	// Integration I to VI is one volume in English and four in French, so
	// pairing volumes would pair nothing at all. Chapter V of the English is
	// chapter V of int-v-fr and that is the whole of what has to line up.
	bm, sm := shelf(t,
		[]corpus.Book{
			{ID: "int-i-vi", Book: "int", Lang: "en"},
			{ID: "int-i-iv-fr", Book: "int", Lang: "fr"},
			{ID: "int-v-fr", Book: "int", Lang: "fr"},
		},
		[]corpus.BookSections{
			{ID: "int-i-vi", Chapters: []corpus.ChapterSections{
				chapter("I", section("int-i-s1", 3, 1)),
				chapter("V", section("int-v-s1", 9, 4)),
			}},
			{ID: "int-i-iv-fr", Chapters: []corpus.ChapterSections{
				chapter("I", section("int-i-s1", 3, 1)),
			}},
			{ID: "int-v-fr", Chapters: []corpus.ChapterSections{
				chapter("V", section("int-v-s1", 9, 4)),
			}},
		})
	pairs := Pairs(bm, sm, "en", "fr")
	if len(pairs) != 2 {
		t.Fatalf("got %d pairs, want 2: %+v", len(pairs), pairs)
	}
	if pairs[0].Chapter != "I" || pairs[0].Right != "int-i-iv-fr" {
		t.Errorf("chapter I paired with %q", pairs[0].Right)
	}
	if pairs[1].Chapter != "V" || pairs[1].Right != "int-v-fr" {
		t.Errorf("chapter V paired with %q", pairs[1].Right)
	}
}

func TestAChapterOnlyOnePrintingHasReadIsNotAPair(t *testing.T) {
	// A French volume with nothing assembled yet is not a printing to compare
	// against, and saying so is the point: an empty side would read as a
	// chapter where every count disagrees.
	bm, sm := shelf(t,
		[]corpus.Book{
			{ID: "alg-viii", Book: "alg", Lang: "en"},
			{ID: "alg-ix-fr", Book: "alg", Lang: "fr"},
		},
		[]corpus.BookSections{
			{ID: "alg-viii", Chapters: []corpus.ChapterSections{
				chapter("VIII", section("alg-viii-s1", 36, 28)),
			}},
			{ID: "alg-ix-fr", Chapters: []corpus.ChapterSections{chapter("IX")}},
		})
	if pairs := Pairs(bm, sm, "en", "fr"); len(pairs) != 0 {
		t.Fatalf("got %+v, want no pair", pairs)
	}
}

func TestSectionsAreMatchedOnTheirLabelAndNotTheirTitle(t *testing.T) {
	// § 13 is "Absolutely semisimple algebras" in one printing and "Algèbres
	// absolument semi-simples" in the other, and the files are named for the
	// titles. Only the label is the same in both.
	bm, sm := shelf(t,
		[]corpus.Book{
			{ID: "alg-viii", Book: "alg", Lang: "en"},
			{ID: "alg-viii-fr", Book: "alg", Lang: "fr"},
		},
		[]corpus.BookSections{
			{ID: "alg-viii", Chapters: []corpus.ChapterSections{chapter("VIII",
				corpus.SectionRecord{Kind: corpus.KindSection, Label: "alg-viii-s13",
					Title: "Absolutely semisimple algebras", Statements: 33, Exercises: 12},
			)}},
			{ID: "alg-viii-fr", Chapters: []corpus.ChapterSections{chapter("VIII",
				corpus.SectionRecord{Kind: corpus.KindSection, Label: "alg-viii-s13",
					Title: "Algèbres absolument semi-simples", Statements: 33, Exercises: 12},
			)}},
		})
	p, err := Compare(sm, Pairs(bm, sm, "en", "fr")[0])
	if err != nil {
		t.Fatal(err)
	}
	if len(p.Rows) != 1 {
		t.Fatalf("got %d rows, want the two titles on one row: %+v", len(p.Rows), p.Rows)
	}
	if p.Disagreements() != 0 {
		t.Errorf("§ 13 reads the same in both printings, got %s", p.Table(false))
	}
	if p.Rows[0].Title != "Absolutely semisimple algebras" {
		t.Errorf("the row is titled %q, want the left printing's title", p.Rows[0].Title)
	}
}

func TestOnlyTheSectionsThePrintingsDisagreeOnAreListed(t *testing.T) {
	bm, sm := shelf(t,
		[]corpus.Book{
			{ID: "alg-viii", Book: "alg", Lang: "en"},
			{ID: "alg-viii-fr", Book: "alg", Lang: "fr"},
		},
		[]corpus.BookSections{
			{ID: "alg-viii", Chapters: []corpus.ChapterSections{chapter("VIII",
				section("alg-viii-s1", 36, 28),
				section("alg-viii-s2", 36, 20),
			)}},
			{ID: "alg-viii-fr", Chapters: []corpus.ChapterSections{chapter("VIII",
				section("alg-viii-s1", 36, 28),
				section("alg-viii-s2", 36, 19),
			)}},
		})
	p, err := Compare(sm, Pairs(bm, sm, "en", "fr")[0])
	if err != nil {
		t.Fatal(err)
	}
	if p.Disagreements() != 1 {
		t.Fatalf("got %d disagreements, want the one exercise § 2 differs by", p.Disagreements())
	}
	table := p.Table(false)
	if strings.Contains(table, "alg-viii-s1 |") {
		t.Errorf("§ 1 agrees and should not be in the table:\n%s", table)
	}
	if !strings.Contains(table, "| alg-viii-s2 | 36 | 36 | 20 | 19 |") {
		t.Errorf("§ 2 is not in the table as it stands in the two printings:\n%s", table)
	}
	if !strings.Contains(table, "1 of 2 sections agree") {
		t.Errorf("the table does not count the sections that agree:\n%s", table)
	}
	if !strings.Contains(p.Table(true), "alg-viii-s1 |") {
		t.Errorf("-all should list § 1 too:\n%s", p.Table(true))
	}
}

func TestATableWithNothingInItSaysSo(t *testing.T) {
	// A table of no rows would look like a table that failed to be written,
	// which is the reading a report of disagreements can least afford.
	p := &Printings{Pair: Pair{Book: "alg", Chapter: "VIII", LeftLang: "en", RightLang: "fr"}}
	if !strings.Contains(p.Table(false), "| none |") {
		t.Errorf("an empty comparison prints:\n%s", p.Table(false))
	}
}

func TestSectionsComeOutInTheOrderTheChapterPrintsThem(t *testing.T) {
	// Sorted as strings, § 10 comes before § 2 and the frontmatter comes after
	// both. A reader checking a chapter against the book reads it in the book's
	// order or not at all.
	rows := []string{"historical", "alg-viii-a2", "alg-viii-s10", "alg-viii-a1", "front", "alg-viii-s2"}
	want := []string{"front", "alg-viii-s2", "alg-viii-s10", "alg-viii-a1", "alg-viii-a2", "historical"}
	for i := range rows {
		for j := i + 1; j < len(rows); j++ {
			if labelLess(rows[j], rows[i]) {
				rows[i], rows[j] = rows[j], rows[i]
			}
		}
	}
	for i := range want {
		if rows[i] != want[i] {
			t.Fatalf("got %v, want %v", rows, want)
		}
	}
}
