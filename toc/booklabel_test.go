package toc

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/pagemap"
)

// Most of the library prints the chapter alone in the labels of its table,
// "I. 1" and "IV. 110". The French Theorie des ensembles prints the Book in
// front of it, "E I.7", and there is no way to read EI.7 as a numeral and a
// page, so the volume yielded no contents line at all.
func TestALabelThatCarriesTheBookIsStillALabel(t *testing.T) {
	tests := []struct {
		tok     string
		in      string
		chapter string
		page    int
	}{
		{"E I.7", "", "I", 7},
		{"E I.14", "I", "I", 14},
		{"E II.30", "II", "II", 30},
		{"E IV.101", "IV", "IV", 101},
		// The Book is dropped whatever it is, since the corpus has ac, ta, evt
		// and the rest ahead of it, and the scan sets it in one to three
		// capitals or not at all.
		{"EVT III.4", "III", "III", 4},
		{"AC II.19", "II", "II", 19},
	}
	for _, tt := range tests {
		ch, p, ok := readLabel(tt.tok, tt.in)
		if !ok || ch != tt.chapter || p != tt.page {
			t.Errorf("readLabel(%q, %q) = %s.%d %v, want %s.%d",
				tt.tok, tt.in, ch, p, ok, tt.chapter, tt.page)
		}
	}
}

// The field the second pass drops is a field the rest of the library needs, so
// it only goes when it cannot be the numeral itself.
func TestTheChapterNumeralIsNeverTakenForABook(t *testing.T) {
	tests := []struct {
		tok     string
		in      string
		chapter string
		page    int
	}{
		{"I 5", "I", "I", 5},
		{"V 12", "V", "V", 12},
		{"IV. 11 0", "IV", "IV", 110},
		{"II.lO", "II", "II", 10},
	}
	for _, tt := range tests {
		ch, p, ok := readLabel(tt.tok, tt.in)
		if !ok || ch != tt.chapter || p != tt.page {
			t.Errorf("readLabel(%q, %q) = %s.%d %v, want %s.%d",
				tt.tok, tt.in, ch, p, ok, tt.chapter, tt.page)
		}
	}
	// A numeral that is written the way a numeral is written stays where it is,
	// and one that is not is a Book.
	for _, s := range []string{"I", "II", "V", "IX"} {
		if _, ok := withoutBook(s + " 5"); ok {
			t.Errorf("%q was taken for a Book in front of a label", s)
		}
	}
	for _, s := range []string{"E", "AC", "EVT", "TG"} {
		rest, ok := withoutBook(s + " I.7")
		if !ok || rest != "I.7" {
			t.Errorf("withoutBook(%q) = %q %v, want I.7", s+" I.7", rest, ok)
		}
	}
	// Nothing longer than three letters and nothing with a figure in it is a
	// Book, because both are how a scan misreads a numeral run into its page.
	for _, s := range []string{"ELEM I.7", "IIL13 4", "Y.25 8"} {
		if _, ok := withoutBook(s); ok {
			t.Errorf("%q was taken for a Book in front of a label", s)
		}
	}
}

// The tail pattern reads a label off the end of a line in up to three pieces,
// each of them four characters wide after the first. A whole label does not fit
// in four, so "E II.30" split as "E" and "II.30" matched nothing and every line
// from § 5 of chapter II of the French Theorie des ensembles onwards was
// dropped, chapters III and IV among them.
func TestTheTailOfAnEntryHoldsTheBookAsWell(t *testing.T) {
	tests := []struct {
		line    string
		chapter string
		page    int
	}{
		{"INTRODUCTION . . . . . . . . . . . . . . . . . . . . . . . E I.7", "I", 7},
		{"§ 5. Relations d'equivalence . . . . . . . . . . . . . . . E II.30", "II", 30},
		{"    3. Ensembles bien ordonnes . . . . . . . . . . . . . . E III.15", "III", 15},
		{"    2. Cardinaux . . . . . . . . . . . . . . . . . . . . . E IV.101", "IV", 101},
	}
	for _, tt := range tests {
		_, got, ok := splitTail(tt.line, Label)
		if !ok || got.chapter != tt.chapter || got.page != tt.page {
			t.Errorf("splitTail(%q) = %+v %v, want %s.%d",
				tt.line, got, ok, tt.chapter, tt.page)
		}
	}
}

// A title that runs the width of the line leaves the printing setting a period
// where it would set a leader, and then the label has no leader in front of it
// and is read a piece at a time off the end. One piece is the page and one more
// is the Book, and stopping at the page left the Book on the end of the title.
//
// It is no. 5 of chapter II § 6 of the French Theorie des ensembles.
func TestALabelWithNoLeadersCarriesItsBookToo(t *testing.T) {
	const line = "    5. Applications compatibles avec des relations d'equivalence. E II.44"
	text, tl, ok, got := noLeaderLabel(line, line, classify(line, Pilcrow), Pilcrow, "II")
	if !ok || tl.chapter != "II" || tl.page != 44 {
		t.Fatalf("noLeaderLabel(%q) = %+v %v, want II.44", line, tl, ok)
	}
	if got.number != 5 || got.kind != kindSubsection {
		t.Errorf("the line stopped being no. 5: %+v", got)
	}
	if !strings.HasSuffix(text, "d'equivalence.") {
		t.Errorf("the Book was left on the title: %q", text)
	}
}

// Taking one more piece only counts where the longer reading is the same
// chapter and the same page, so a title whose last word looks like a Book keeps
// it. "Modules" is not a Book and "E" in "Modules E" is not one either once the
// reading without it already gives II.60.
func TestOneMorePieceDoesNotEatAWordOfTheTitle(t *testing.T) {
	const line = "    1. Modules; applications lineaires sur E II.60"
	text, tl, ok, _ := noLeaderLabel(line, line, classify(line, Pilcrow), Pilcrow, "II")
	if !ok || tl.page != 60 {
		t.Fatalf("noLeaderLabel(%q) = %+v %v, want II.60", line, tl, ok)
	}
	// "sur E II.60" reads as chapter II page 60 with the Book dropped, the same
	// as "E II.60", so the piece is taken. What must not happen is the title
	// losing a word that changes what the label says.
	if strings.Contains(text, "II.60") {
		t.Errorf("the label was left on the title: %q", text)
	}
	if !strings.Contains(text, "applications lineaires") {
		t.Errorf("the title lost its own words: %q", text)
	}
}

// ensFrontMatter is a table of contents that points at page labels carrying the
// Book, lists the front matter above the chapter line, and numbers that front
// matter inside chapter I. It is the French Theorie des ensembles cut to one
// chapter and two §.
const ensFrontMatter = `                              TABLE DES MATIÈRES

INTRODUCTION . . . . . . . . . . . . . . . . . . . . . . . . . E I.7

CHAPITRE I. — DESCRIPTION DE LA MATHÉMATIQUE FORMELLE . . . . . E I.14

§ 1. Termes et relations . . . . . . . . . . . . . . . . . . . . E I.14
   1. Signes et assemblages . . . . . . . . . . . . . . . . . . E I.14
   2. Critères de substitution . . . . . . . . . . . . . . . . . E I.16
§ 2. Théorèmes . . . . . . . . . . . . . . . . . . . . . . . . . E I.20
   1. Les axiomes . . . . . . . . . . . . . . . . . . . . . . . E I.20
`

// ensFrontMatterMap is chapter I running from pdf 2, because the note to the
// reader and the introduction on pdf 2 to 13 print folios that say chapter I.
func ensFrontMatterMap() *pagemap.Map {
	m := &pagemap.Map{Book: "test", PDFPages: 40,
		Chapters: []pagemap.Span{{Chapter: "I", FirstPDF: 2, LastPDF: 40, FirstPage: 1, LastPage: 39}}}
	for i := 1; i <= 40; i++ {
		e := pagemap.Entry{PDFPage: i, Confidence: pagemap.Unknown}
		if i >= 2 {
			e.Chapter, e.Page, e.Confidence = "I", i-1, pagemap.FromLabel
		}
		m.Entries = append(m.Entries, e)
	}
	return m
}

// A volume that numbers its own front matter inside chapter I puts real folios
// reading chapter I on leaves that are not chapter I. The contents says chapter
// I opens at printed page 14 and the page map starts the span at printed page 1,
// and both are right. Told where the front matter stops, the check knows the
// difference; not told, it reports a chapter that is fine.
func TestFrontMatterInsideChapterOneIsNotADisagreement(t *testing.T) {
	pages, pm := []string{ensFrontMatter}, ensFrontMatterMap()
	opt := Options{Book: "test", Chapters: []string{"I"}}

	res, err := Parse(pages, pm, opt)
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) == 0 {
		t.Fatal("a span reaching back over the front matter was not reported at all")
	}

	opt.FrontMatterPDF = 13
	res, err = Parse(pages, pm, opt)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range res.Problems {
		if strings.Contains(p.Detail, "the page map at") {
			t.Errorf("the front matter was still read as a disagreement: %s", p.Detail)
		}
	}
	c, ok := res.Get("I")
	if !ok {
		t.Fatal("no chapter I")
	}
	if len(c.Sections) != 2 {
		t.Fatalf("chapter I has %d §, want 2", len(c.Sections))
	}
	if c.Page != 14 {
		t.Errorf("chapter I opens at printed page %d, want 14", c.Page)
	}
}
