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

// A heading the reading wrote as a paragraph comes back at the level the
// contents gives it. This is § 11 of Algebra VIII, where the contents lists
// twelve no., page 218 wrote no. 10 with no hashes on it, and the assembler
// stopped at eleven.
func TestAHeadingReadAsAParagraphIsFoundByTheContents(t *testing.T) {
	b := corpus.BookTOC{ID: "alg-viii", Chapters: []corpus.Chapter{{
		Numeral: "VIII",
		Sections: []corpus.Section{{
			Number: 11, Title: "Grothendieck Groups", PDFPage: 200,
			Subsections: []corpus.Subsection{
				{Number: 9, Title: "The Grothendieck Group K0 (A) of an Artinian Ring", PDFPage: 217},
				{Number: 10, Title: "Change of Rings for K0 (A)", PDFPage: 218},
				{Number: 11, Title: "Frobenius Reciprocity", PDFPage: 219},
			},
		}},
	}}}
	h, ok := ParseLostHeading("10. Change of Rings for $ K_0(A) $")
	if !ok {
		t.Fatal("the line was not read as a heading that lost its level")
	}
	if h.Level != 0 {
		t.Errorf("Level = %d, want 0, since the page said nothing about it", h.Level)
	}
	if got := Level(b, 218, h.Number, h.Title); got != 3 {
		t.Fatalf("Level = %d, want 3", got)
	}
	if got, want := h.Write(3), "### 10. Change of Rings for $ K_0(A) $"; got != want {
		t.Errorf("Write(3) = %q, want %q", got, want)
	}
}

// The contents is the whole of the authority here. A numbered paragraph has the
// same shape as a heading that lost its level, and the corpus has thousands of
// them: every volume sets its "To the reader" as a numbered list, and page 5 of
// Algebra I to III opens four of them. Level says nothing about any of these,
// so nothing promotes them.
func TestANumberedParagraphIsNotPromoted(t *testing.T) {
	b := corpus.BookTOC{ID: "alg-i-iii", Chapters: []corpus.Chapter{{
		Numeral: "I",
		Sections: []corpus.Section{{
			Number: 1, Title: "Laws of composition", PDFPage: 25,
			Subsections: []corpus.Subsection{
				{Number: 1, Title: "Laws of composition", PDFPage: 25},
			},
		}},
	}}}
	for _, line := range []string{
		"1. This series of volumes, a list of which is given on pages ix and x",
		"2. The method of exposition we have chosen is axiomatic and abstract",
		"5. The logical framework of each chapter consists of the definitions",
	} {
		h, ok := ParseLostHeading(line)
		if !ok {
			t.Fatalf("%q was not taken apart", line)
		}
		if got := Level(b, 5, h.Number, h.Title); got != 0 {
			t.Errorf("Level(%q) = %d, want 0", line, got)
		}
	}
}

// A line with no number on it is not one of these at all, and neither is a line
// that already carries its level.
func TestALineWithNoLostHeadingInItIsNotOne(t *testing.T) {
	for _, line := range []string{
		"### 10. Change of Rings",
		"Let $E$ be a module and 10 its rank.",
		"10.Change of Rings",
		"10. ",
	} {
		if _, ok := ParseLostHeading(line); ok {
			t.Errorf("%q was read as a heading that lost its level", line)
		}
	}
}

// The contents and the body write the mathematics in a title two different
// ways, and the difference is enough to hide a heading. § 16 of the French
// Algebra VIII is the case: the contents reads the tau as the character the
// page prints and the body reads it as TeX, so the body carries the letters t,
// a and u that the contents does not, and no. 2 and no. 4 of that § stayed
// paragraphs while their nine neighbours were found. flatten takes the control
// word out of both sides and they agree.
func TestAGreekLetterWrittenTwoWaysIsTheSameHeading(t *testing.T) {
	b := corpus.BookTOC{ID: "alg-viii-fr", Chapters: []corpus.Chapter{{
		Numeral: "VIII",
		Sections: []corpus.Section{{
			Number: 16, Title: "Autres descriptions du groupe de Brauer", PDFPage: 284,
			Subsections: []corpus.Subsection{
				{Number: 1, Title: "τ -extensions de groupes", PDFPage: 284},
				{Number: 2, Title: "Image inverse d’une τ -extension", PDFPage: 286},
				{Number: 4, Title: "Loi de groupe sur les classes de τ -extensions", PDFPage: 292},
			},
		}},
	}}}
	for _, c := range []struct {
		page int
		line string
	}{
		{286, "2. Image inverse d’une $ \\tau $-extension"},
		{292, "4. Loi de groupe sur les classes de $ \\tau $-extensions"},
	} {
		h, ok := ParseLostHeading(c.line)
		if !ok {
			t.Fatalf("%q was not taken apart", c.line)
		}
		if got := Level(b, c.page, h.Number, h.Title); got != 3 {
			t.Errorf("Level(%q) = %d, want 3", c.line, got)
		}
	}
}

// The same fault with markup rather than a character. The body sets the reals
// in bold and the contents prints a plain R, and eleven no. of Topology I to IV
// are written this way.
func TestMarkupInATitleIsNotPartOfIt(t *testing.T) {
	b := corpus.BookTOC{ID: "top-i-iv", Chapters: []corpus.Chapter{{
		Numeral: "IV",
		Sections: []corpus.Section{{
			Number: 2, Title: "Topology of the rational line", PDFPage: 341,
			Subsections: []corpus.Subsection{
				{Number: 2, Title: "Compact subsets of R", PDFPage: 341},
				{Number: 3, Title: "Least upper bound of a subset of R", PDFPage: 341},
			},
		}},
	}}}
	for _, line := range []string{
		"2. COMPACT SUBSETS OF $ \\mathbf{R} $",
		"3. LEAST UPPER BOUND OF A SUBSET OF $ \\mathbf{R} $",
	} {
		h, ok := ParseLostHeading(line)
		if !ok {
			t.Fatalf("%q was not taken apart", line)
		}
		if got := Level(b, 341, h.Number, h.Title); got != 3 {
			t.Errorf("Level(%q) = %d, want 3", line, got)
		}
	}
}

// Dropping the control words and not the whole formula is what keeps no. 10 of
// § 11 of Algebra VIII apart from its neighbours, since the contents prints the
// plain part of the formula and that is most of what the title says.
func TestThePlainPartOfAFormulaIsPartOfTheTitle(t *testing.T) {
	b := corpus.BookTOC{ID: "alg-viii", Chapters: []corpus.Chapter{{
		Numeral: "VIII",
		Sections: []corpus.Section{{
			Number: 11, Title: "K-theory", PDFPage: 200,
			Subsections: []corpus.Subsection{
				{Number: 10, Title: "Change of Rings for K0 (A)", PDFPage: 218},
				{Number: 11, Title: "Change of Rings for K1 (A)", PDFPage: 219},
			},
		}},
	}}}
	if got := Level(b, 218, 10, "Change of Rings for $ K_0(A) $"); got != 3 {
		t.Errorf("Level = %d, want 3", got)
	}
	if got := Level(b, 219, 10, "Change of Rings for $ K_0(A) $"); got != 0 {
		t.Errorf("Level on the wrong page = %d, want 0", got)
	}
	if got := Level(b, 218, 10, "Change of Rings for $ K_1(A) $"); got != 0 {
		t.Errorf("the neighbouring title = %d, want 0, the two differ by a digit", got)
	}
}

// A title with nothing left in it agrees with everything, so it agrees with
// nothing. Two § of Integration VII to IX have an empty title in the contents.
func TestATitleThatFlattensToNothingMatchesNothing(t *testing.T) {
	b := corpus.BookTOC{ID: "int-vii-ix", Chapters: []corpus.Chapter{{
		Numeral:  "VII",
		Sections: []corpus.Section{{Number: 1, Title: "", PDFPage: 12}},
	}}}
	if got := Level(b, 12, 1, "$ \\alpha $"); got != 0 {
		t.Errorf("Level = %d, want 0", got)
	}
}

// A no. the book sets as supplementary carries an asterisk in front of its
// number. Extraction writes it escaped and six lines of the corpus write it
// bare, and no. 13 of § 21 of the French Algebra VIII is one of them. It is
// read either way and written back escaped, since the escaped form is the one
// the assembler reads.
func TestABareAsteriskIsTheSupplementaryMark(t *testing.T) {
	b := corpus.BookTOC{ID: "alg-viii-fr", Chapters: []corpus.Chapter{{
		Numeral: "VIII",
		Sections: []corpus.Section{{
			Number: 21, Title: "Représentations linéaires des groupes finis", PDFPage: 389,
			Subsections: []corpus.Subsection{
				{Number: 13, Title: "Représentations linéaires complexes", PDFPage: 413},
			},
		}},
	}}}
	h, ok := ParseLostHeading("*13. Représentations linéaires complexes")
	if !ok {
		t.Fatal("the line was not taken apart")
	}
	if got := Level(b, 413, h.Number, h.Title); got != 3 {
		t.Fatalf("Level = %d, want 3", got)
	}
	want := `### \*13. Représentations linéaires complexes`
	if got := h.Write(3); got != want {
		t.Errorf("Write(3) = %q, want %q", got, want)
	}
}

// The other two of the six are exercise statements, and nothing here decides
// they are headings. The contents refuses them the way it refuses a numbered
// paragraph.
func TestAStarredExerciseIsNotPromoted(t *testing.T) {
	b := corpus.BookTOC{ID: "ens-i-iv", Chapters: []corpus.Chapter{{
		Numeral: "III",
		Sections: []corpus.Section{{
			Number: 1, Title: "Order relations", PDFPage: 134,
			Subsections: []corpus.Subsection{
				{Number: 3, Title: "Increasing mappings", PDFPage: 134},
			},
		}},
	}}}
	h, ok := ParseLostHeading(`*3. Let $(\mathrm{X}_i)_{1 \leqslant i \leqslant n}$ be a finite family of sets. For each subset H of the index set $[1, n]$ let`)
	if !ok {
		t.Fatal("the line was not taken apart")
	}
	if got := Level(b, 134, h.Number, h.Title); got != 0 {
		t.Errorf("Level = %d, want 0", got)
	}
}
