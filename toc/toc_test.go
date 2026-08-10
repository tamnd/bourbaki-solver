package toc

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/pagemap"
)

// Every fixture in this file is a line copied out of pdftotext -layout run over
// one of the three volumes, damage included. A parser written against clean
// input is worth nothing here: the 1998 and 2003 scans read § 10 as "§ I 0",
// § 11 as "§ II", and the word CHAPTER as "CHAP-1 ER", and those are the lines
// it has to get right.

func TestClassify(t *testing.T) {
	tests := []struct {
		line string
		mark SectionMark
		want entry
	}{
		{"CHAPTER I", Pilcrow, entry{kind: kindChapter, numeral: "I"}},
		{"CHAP-1 ER IV.", Pilcrow, entry{kind: kindChapter, numeral: "IV"}},
		{"CHAPTER VIII Semisimple Modules and Rings", Column,
			entry{kind: kindChapter, numeral: "VIII", title: "Semisimple Modules and Rings"}},
		{"§ 1. Laws of composition", Pilcrow,
			entry{kind: kindSection, number: 1, title: "Laws of composition"}},
		{"§ I 0. Derivations", Pilcrow,
			entry{kind: kindSection, number: 10, title: "Derivations"}},
		{"§ II. Graded modules and rings", Pilcrow,
			entry{kind: kindSection, number: 11, title: "Graded modules and rings"}},
		{"§ 1 Ordered groups", Pilcrow,
			entry{kind: kindSection, number: 1, title: "Ordered groups"}},
		{"Appendix. Pseudomodules", Pilcrow,
			entry{kind: kindAppendix, title: "Pseudomodules"}},
		{"Appendix 1. Algebras without Unit Element", Column,
			entry{kind: kindAppendix, number: 1, title: "Algebras without Unit Element"}},
		{"Exercises for § 1", Pilcrow, entry{kind: kindExercises, number: 1}},
		{"Exercises on § 5", Pilcrow, entry{kind: kindExercises, number: 5}},
		{"Exercise for the Appendix", Pilcrow, entry{kind: kindExercises}},
		{"Historical Note", Pilcrow, entry{kind: kindHistorical}},
		{"   1. Regular Ideals", Column,
			entry{kind: kindSubsection, number: 1, title: "Regular Ideals"}},
		{"   IO. Total algebra of a monoid", Pilcrow,
			entry{kind: kindSubsection, number: 10, title: "Total algebra of a monoid"}},
		// At column 0 the 2023 volume means a §, not a no.
		{"21. Linear Representations of Finite Groups", Column,
			entry{kind: kindSection, number: 21, title: "Linear Representations of Finite Groups"}},
		{"Bibliography", Column, entry{}},
		{"and the reader is assumed to know", Pilcrow, entry{}},
	}
	for _, tt := range tests {
		got := classify(tt.line, tt.mark)
		if got != tt.want {
			t.Errorf("classify(%q, %s) = %+v, want %+v", tt.line, tt.mark, got, tt.want)
		}
	}
}

func TestSplitTail(t *testing.T) {
	tests := []struct {
		line string
		form PageForm
		text string
		want tail
		ok   bool
	}{
		{"§ 1. Laws of composition . . . . . . 1", Bare, "§ 1. Laws of composition", tail{page: 1}, true},
		{"11. Index and Exponent. . . . . . 322", Bare, "11. Index and Exponent", tail{page: 322}, true},
		// The scan splits three-figure numbers and reads 1 as l.
		{"6. Cogebras . . . . . . . . l 03", Bare, "6. Cogebras", tail{page: 103}, true},
		{"9. Modules . . . . . . . . 45 7", Bare, "9. Modules", tail{page: 457}, true},
		{"§ 1. Polynomials . . . . . IV.1", Label, "§ 1. Polynomials", tail{chapter: "IV", page: 1}, true},
		{"5. Roots . . . . . . . . . V1. 10", Label, "5. Roots", tail{chapter: "VI", page: 10}, true},
		// The separator between chapter and page came out as a letter.
		{"3. Complements . . . . V o137", Label, "3. Complements", tail{chapter: "V", page: 137}, true},
		{"a line with no page at the end", Bare, "", tail{}, false},
		{"1. A no. line with no leader 12", Bare, "", tail{}, false},
	}
	for _, tt := range tests {
		text, got, ok := splitTail(tt.line, tt.form)
		if ok != tt.ok {
			t.Errorf("splitTail(%q) ok = %v, want %v", tt.line, ok, tt.ok)
			continue
		}
		if !ok {
			continue
		}
		if text != tt.text || got != tt.want {
			t.Errorf("splitTail(%q) = %q %+v, want %q %+v", tt.line, text, got, tt.text, tt.want)
		}
	}
}

// contents is chapter VIII's opening as the 2023 volume prints it, cut down to
// two § and the first appendix.
const contents = `                                    Contents

CHAPTER VIII SEMISIMPLE MODULES AND RINGS . . . . . . . . . . . . . 1

1. Simple Modules . . . . . . . . . . . . . . . . . . . . . . . . . . . . 1
   1. Simple Modules . . . . . . . . . . . . . . . . . . . . . . . . . . 1
   2. Simple Modules over a Ring . . . . . . . . . . . . . . . . . . . . 3
   Exercises. . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . 5
2. Semisimple Modules . . . . . . . . . . . . . . . . . . . . . . . . . . 7
   1. Direct Sums . . . . . . . . . . . . . . . . . . . . . . . . . . . . 7
   Exercises. . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . 9
Appendix 1. Algebras without Unit Element . . . . . . . . . . . . . . . 11
   1. Regular Ideals . . . . . . . . . . . . . . . . . . . . . . . . . . 11
   Exercises. . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . 13
Historical Note. . . . . . . . . . . . . . . . . . . . . . . . . . . . . 15
`

// testMap is a page map for a single chapter of 20 printed pages sitting at a
// constant offset of 2, which is all the parser asks of one.
func testMap() *pagemap.Map {
	m := &pagemap.Map{Book: "test", PDFPages: 22,
		Chapters: []pagemap.Span{{Chapter: "VIII", FirstPDF: 3, LastPDF: 22, FirstPage: 1, LastPage: 20}}}
	for i := 1; i <= 22; i++ {
		e := pagemap.Entry{PDFPage: i, Confidence: pagemap.Unknown}
		if i >= 3 {
			e.Chapter, e.Page, e.Confidence = "VIII", i-2, pagemap.FromHead
		}
		m.Entries = append(m.Entries, e)
	}
	return m
}

func TestParse(t *testing.T) {
	res, err := Parse([]string{contents}, testMap(), Options{Book: "test", Chapters: []string{"VIII"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) > 0 {
		t.Errorf("problems: %v", res.Problems)
	}
	if got := res.Grammar; got.Mark != Column || got.Page != Bare {
		t.Errorf("grammar = %v, want column/bare", got)
	}
	c, ok := res.Get("VIII")
	if !ok {
		t.Fatal("no chapter VIII")
	}
	if c.Title != "SEMISIMPLE MODULES AND RINGS" || c.Page != 1 || c.PDFPage != 3 {
		t.Errorf("chapter = %q printed %d pdf %d", c.Title, c.Page, c.PDFPage)
	}
	if len(c.Sections) != 3 {
		t.Fatalf("%d sections, want 2 § and 1 appendix", len(c.Sections))
	}
	app := c.Sections[2]
	if !app.Appendix || app.Number != 1 || app.Title != "Algebras without Unit Element" {
		t.Errorf("appendix = %+v", app)
	}
	if app.Page != 11 || app.PDFPage != 13 {
		t.Errorf("appendix printed %d pdf %d, want 11 and 13", app.Page, app.PDFPage)
	}
	// An appendix is not a §, so § 1 is still § 1.
	if s, ok := c.Get(1); !ok || len(s.Subsections) != 2 || s.Exercises == nil || s.Exercises.Page != 5 {
		t.Errorf("§ 1 = %+v", s)
	}
	if c.Historical == nil || c.Historical.Page != 15 || c.Historical.PDFPage != 17 {
		t.Errorf("historical note = %+v", c.Historical)
	}
	if ch, sec, sub, ex := res.Counts(); ch != 1 || sec != 3 || sub != 4 || ex != 3 {
		t.Errorf("counts = %d %d %d %d", ch, sec, sub, ex)
	}
}

// A § listed out of order, or on a page the chapter does not have, has to be
// reported rather than committed, because that is the whole reason the contents
// is checked against the page map at all.
func TestParseReportsDamage(t *testing.T) {
	bad := strings.Replace(contents, "2. Semisimple Modules . . . . . . . . . . . . . . . . . . . . . . . . . . 7",
		"3. Semisimple Modules . . . . . . . . . . . . . . . . . . . . . . . . . . 7", 1)
	res, err := Parse([]string{bad}, testMap(), Options{Book: "test", Chapters: []string{"VIII"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) == 0 {
		t.Fatal("a § numbered 3 in second place was accepted")
	}
}

// The numbered paragraphs of "To the Reader" look exactly like no. lines. What
// keeps them out is that a contents entry ends in a page number and a paragraph
// does not, so a page has to yield a few complete entries before it is read.
func TestContentsPagesRejectsProse(t *testing.T) {
	prose := `                            TO THE READER

1. The Elements of Mathematics Series takes up mathematics at the
beginning, and gives complete proofs.

2. The method of exposition we have chosen is axiomatic and abstract,
and normally proceeds from the general to the particular.

3. The Elements are divided into Books.
`
	if got := contentsPages([]string{prose}, Grammar{Pilcrow, Bare}); len(got) != 0 {
		t.Errorf("%d pages of prose read as contents", len(got))
	}
	if got := contentsPages([]string{contents}, Grammar{Column, Bare}); len(got) != 1 {
		t.Errorf("the contents was not recognised, got %d pages", len(got))
	}
}

func TestDetect(t *testing.T) {
	if g := Detect([]string{contents}); g.Mark != Column || g.Page != Bare {
		t.Errorf("Detect = %v, want column/bare", g)
	}
	pilcrow := `§ 1. Laws of composition . . . . . . . . . 1
§ 2. Identity element . . . . . . . . . . . 12
§ 3. Actions . . . . . . . . . . . . . . . . 22
`
	if g := Detect([]string{pilcrow}); g.Mark != Pilcrow || g.Page != Bare {
		t.Errorf("Detect = %v, want pilcrow/bare", g)
	}
	label := `§ 1. Polynomials . . . . . . . . . . . IV.1
§ 2. Rational fractions . . . . . . . . IV.20
§ 3. Symmetric functions . . . . . . . IV.55
`
	if g := Detect([]string{label}); g.Mark != Pilcrow || g.Page != Label {
		t.Errorf("Detect = %v, want pilcrow/label", g)
	}
}

func TestShout(t *testing.T) {
	tests := []struct{ in, want string }{
		{"TENSOR ALGEBRAS, ExTERIOR ALGEBRAs, SYMMETRIC ALGEBRAS",
			"TENSOR ALGEBRAS, EXTERIOR ALGEBRAS, SYMMETRIC ALGEBRAS"},
		{"Semisimple Modules and Rings", "Semisimple Modules and Rings"},
		{"The homomorphism E* 0A F", "The homomorphism E* 0A F"},
	}
	for _, tt := range tests {
		if got := shout(tt.in); got != tt.want {
			t.Errorf("shout(%q) = %q", tt.in, got)
		}
	}
}
