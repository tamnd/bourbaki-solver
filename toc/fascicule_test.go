package toc

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/pagemap"
)

// The French Varietes differentielles et analytiques is one scan holding two
// fascicules de resultats. Paragraphes 1 a 7 are bound first, with their table
// of contents at the front, and paragraphes 8 a 15 follow with theirs at the
// back, and the printed numbering starts over between them: pdf 95 is printed
// page 97 and pdf 96 is printed page 6.
//
// Neither fascicule names a chapter, because neither has one, so every § came
// out of the contents with nothing open to hang it on and the volume reported
// that the contents yielded no chapters. openImplied is what opens a chapter a
// volume never names, and it used to want exactly one span in the page map,
// which a volume in two fascicules does not have.
const fascicule1 = `TABLE DES MATIÈRES
(paragraphes 1 à 7)

§ 1. Fonctions différentiables ...... 11
§ 2. Variétés différentielles ...... 20
§ 3. Espaces tangents ...... 31
`

const fascicule2 = `TABLE DES MATIÈRES
(paragraphes 8 à 15)

§ 8. Formes différentielles ...... 6
§ 9. Intégration ...... 19
§ 10. Cohomologie ...... 27
`

// fasciculeMap is the shape of the Varietes page map, cut down to the two runs
// and the pages the contents of each sits on. Fascicule 1 prints its contents
// at the front of itself and fascicule 2 at the back of itself, which is why
// the span is decided by where the page sits and not by what it lists.
func fasciculeMap() *pagemap.Map {
	m := &pagemap.Map{Book: "var-fr", Pagination: pagemap.Continuous, PDFPages: 10,
		Chapters: []pagemap.Span{
			{Chapter: "1", FirstPDF: 1, LastPDF: 5, FirstPage: 93, LastPage: 97},
			{Chapter: "2", FirstPDF: 6, LastPDF: 10, FirstPage: 6, LastPage: 10},
		}}
	for i := 1; i <= 10; i++ {
		e := pagemap.Entry{PDFPage: i, Confidence: pagemap.FromHead}
		if i <= 5 {
			e.Chapter, e.Page = "1", 92+i
		} else {
			e.Chapter, e.Page = "2", i-5
		}
		m.Entries = append(m.Entries, e)
	}
	return m
}

func TestEachFasciculeGetsItsOwnChapter(t *testing.T) {
	pages := make([]string, 10)
	// The contents of fascicule 1 is on pdf 3, inside fascicule 1, and the
	// contents of fascicule 2 on pdf 9, inside fascicule 2.
	pages[2] = fascicule1
	pages[8] = fascicule2

	res, err := Parse(pages, fasciculeMap(),
		Options{Book: "var-fr", Title: "Variétés différentielles et analytiques"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Chapters) != 2 {
		t.Fatalf("%d chapters, want one per fascicule: %+v", len(res.Chapters), res.Chapters)
	}
	one, ok := res.Get("1")
	if !ok {
		t.Fatal("fascicule 1 was never opened")
	}
	two, ok := res.Get("2")
	if !ok {
		t.Fatal("fascicule 2 was never opened")
	}
	// The §§ are split at the fascicule they are printed in and not pooled into
	// whichever chapter happened to be open when the second contents came round.
	if len(one.Sections) != 3 || len(two.Sections) != 3 {
		t.Fatalf("fascicule 1 has %d §, fascicule 2 has %d, want 3 each",
			len(one.Sections), len(two.Sections))
	}
	if one.Sections[0].Number != 1 || two.Sections[0].Number != 8 {
		t.Errorf("the §§ open at %d and %d, want 1 and 8",
			one.Sections[0].Number, two.Sections[0].Number)
	}
	// Each chapter takes the page of its first §, which for the second fascicule
	// is a printed page smaller than anything in the first.
	if one.Page != 11 || two.Page != 6 {
		t.Errorf("the chapters start at printed %d and %d, want 11 and 6", one.Page, two.Page)
	}
}

// The span is settled by where the contents page sits, because a fascicule
// prints its own contents inside itself. Reading it off the pages the contents
// lists would not work: the two fascicules print overlapping printed pages, so
// a § on printed page 20 is in both of them.
func TestTheFasciculeIsTakenFromWhereTheContentsIsPrinted(t *testing.T) {
	pm := fasciculeMap()
	for _, tc := range []struct {
		pdf  int
		want string
	}{{1, "1"}, {3, "1"}, {5, "1"}, {6, "2"}, {9, "2"}, {10, "2"}} {
		if got := spanAt(pm, tc.pdf); got != tc.want {
			t.Errorf("spanAt(%d) = %q, want %q", tc.pdf, got, tc.want)
		}
	}
	if got := spanAt(pm, 0); got != "" {
		t.Errorf("spanAt(0) = %q, want nothing before the first span", got)
	}
}

// Only a volume pagemap cut itself is read this way. Every volume that declares
// its own chapters declares them in roman, so a numeral that parses as arabic
// is the mark of a span we made, and a volume with two real chapters is left
// exactly where it was: the contents alone cannot say where one ends and the
// next starts, so the § is dropped and the problem is reported.
func TestAVolumeWithRealChaptersIsNotReadAsFascicules(t *testing.T) {
	roman := &pagemap.Map{Book: "lie-i-iii", PDFPages: 10,
		Chapters: []pagemap.Span{
			{Chapter: "I", FirstPDF: 1, LastPDF: 5},
			{Chapter: "II", FirstPDF: 6, LastPDF: 10},
		}}
	if fascicules(roman) {
		t.Error("a volume that prints chapters I and II was read as fascicules")
	}
	one := &pagemap.Map{Book: "hist-fr", PDFPages: 10,
		Chapters: []pagemap.Span{{Chapter: pagemap.WholeVolume, FirstPDF: 1, LastPDF: 10}}}
	if fascicules(one) {
		t.Error("a volume of one span was read as fascicules")
	}
	if fascicules(nil) {
		t.Error("no page map was read as fascicules")
	}
}
