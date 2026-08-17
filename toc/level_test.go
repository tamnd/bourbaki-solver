package toc

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The book is Theory of Sets, chapter III. § 1 and its no. 1 both begin on pdf
// page 137, and no. 12 is on 152, where the reading wrote it as a §.
func ensIII() corpus.BookTOC {
	return corpus.BookTOC{ID: "ens-i-iv", Chapters: []corpus.Chapter{{
		Numeral: "III",
		Sections: []corpus.Section{{
			Number: 1, Title: "Order relations. Ordered sets", PDFPage: 137,
			Subsections: []corpus.Subsection{
				{Number: 1, Title: "Definition of an order relation", PDFPage: 137},
				{Number: 12, Title: "Totally ordered sets", PDFPage: 152},
				{Number: 13, Title: "Intervals", PDFPage: 153},
			},
		}, {
			Number: 2, Title: "Well-ordered sets", PDFPage: 154,
			Subsections: []corpus.Subsection{
				{Number: 1, Title: "Segments of a well-ordered set", PDFPage: 154},
				{Number: 2, Title: "The principle of transfinite induction", PDFPage: 157},
			},
		}},
	}}}
}

func TestTheContentsSaysWhichOfTheTwoAHeadingIs(t *testing.T) {
	b := ensIII()
	cases := []struct {
		page  int
		num   int
		title string
		want  int
	}{
		// The § and its first no. are on the same page and both are numbered 1.
		// This is the pair a lookup by number alone gets wrong.
		{137, 1, "ORDER RELATIONS. ORDERED SETS", 2},
		{137, 1, "DEFINITION OF AN ORDER RELATION", 3},
		// The eight of Theory of Sets, two of them.
		{152, 12, "TOTALLY ORDERED SETS", 3},
		{157, 2, "THE PRINCIPLE OF TRANSFINITE INDUCTION", 3},
		// And a § that is on the same page as a no. of the § before it.
		{154, 2, "WELL-ORDERED SETS", 2},
		{154, 1, "SEGMENTS OF A WELL-ORDERED SET", 3},
		// A heading the contents does not put on this page. The reading is
		// ahead of the contents or behind it, and either way this is not the
		// repair that settles it.
		{153, 12, "TOTALLY ORDERED SETS", 0},
		{152, 12, "SOMETHING ELSE ENTIRELY", 0},
	}
	for _, c := range cases {
		if got := Level(b, c.page, c.num, c.title); got != c.want {
			t.Errorf("Level(%d, %d, %q) = %d, want %d", c.page, c.num, c.title, got, c.want)
		}
	}
}

// The contents flattens the mathematics of a title to plain letters, and the
// page keeps it. no. 7 of chapter III, § 5 is one: "Expansion to base b" in the
// contents and "EXPANSION TO BASE $b$" on the page.
func TestATitleWithMathematicsInItStillMatches(t *testing.T) {
	b := corpus.BookTOC{ID: "ens-i-iv", Chapters: []corpus.Chapter{{
		Numeral: "III",
		Sections: []corpus.Section{{
			Number: 5, Title: "Properties of integers", PDFPage: 178,
			Subsections: []corpus.Subsection{
				{Number: 7, Title: "Expansion to base b", PDFPage: 183},
			},
		}},
	}}}
	if got := Level(b, 183, 7, "EXPANSION TO BASE $b$"); got != 3 {
		t.Errorf("Level = %d, want 3", got)
	}
}

func TestAHeadingIsReadAtEitherLevelAndPutBackAtTheOther(t *testing.T) {
	h, ok := ParseHeading("## 12. TOTALLY ORDERED SETS")
	if !ok {
		t.Fatal("the line was not read as a heading")
	}
	if h.Level != 2 || h.Number != 12 || h.Title != "TOTALLY ORDERED SETS" {
		t.Fatalf("read %+v", h)
	}
	if got, want := h.Write(3), "### 12. TOTALLY ORDERED SETS"; got != want {
		t.Errorf("Write(3) = %q, want %q", got, want)
	}
}

// § 21 of chapter VIII has a no. the book sets as supplementary, and the star
// belongs to the heading rather than to the title.
func TestTheSupplementaryStarSurvivesTheRepair(t *testing.T) {
	h, ok := ParseHeading(`## \*7. THE BRAUER GROUP`)
	if !ok {
		t.Fatal("the line was not read as a heading")
	}
	if h.Number != 7 || h.Title != "THE BRAUER GROUP" {
		t.Fatalf("read %+v", h)
	}
	if got, want := h.Write(3), `### \*7. THE BRAUER GROUP`; got != want {
		t.Errorf("Write(3) = %q, want %q", got, want)
	}
}

func TestALineThatIsNotANumberedHeadingIsNotOne(t *testing.T) {
	for _, line := range []string{
		"## CHAPTER III Ordered Sets, Cardinals, Integers",
		"#### 1. TOO DEEP",
		"# 1. TOO SHALLOW",
		"### Exercises {#ens-iii-s1-exercises}",
		"12. Totally ordered sets",
	} {
		if _, ok := ParseHeading(line); ok {
			t.Errorf("%q was read as a numbered heading", line)
		}
	}
}
