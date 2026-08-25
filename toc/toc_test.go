package toc

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
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
		// Off the English Algebra, pdf page 16 line 41. Read as § 1 this run
		// takes the place of the real run of § 1, and both pages are inside the
		// chapter, so nothing else in the file looks wrong.
		{"Exercises for § I 0", Pilcrow, entry{kind: kindExercises, number: 10}},
		{"Exercises for § II", Pilcrow, entry{kind: kindExercises, number: 11}},
		// The run is the appendix's, and saying so is what puts it there. The
		// old reader gave it to whatever section was listed last, which was the
		// appendix only because the appendix happens to close the chapter.
		{"Exercise for the Appendix", Pilcrow,
			entry{kind: kindExercises, appendix: true}},
		{"Exercises for Appendix II", Pilcrow,
			entry{kind: kindExercises, number: 2, appendix: true}},
		{"Appendix II - A connectedness property", Pilcrow,
			entry{kind: kindAppendix, number: 2, title: "A connectedness property"}},
		{"SUMMARY OF RESULTS", Pilcrow,
			entry{kind: kindPart, title: "SUMMARY OF RESULTS"}},
		{"CONTENTS OF THE ELEMENTS OF MATHEMATICS SERIES", Pilcrow, entry{}},
		{"XVlll", Pilcrow, entry{}},
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
func testMap() *pagemap.Map { return testMapFor("VIII") }

func testMapFor(numeral string) *pagemap.Map {
	m := &pagemap.Map{Book: "test", PDFPages: 22,
		Chapters: []pagemap.Span{{Chapter: numeral, FirstPDF: 3, LastPDF: 22, FirstPage: 1, LastPage: 20}}}
	for i := 1; i <= 22; i++ {
		e := pagemap.Entry{PDFPage: i, Confidence: pagemap.Unknown}
		if i >= 3 {
			e.Chapter, e.Page, e.Confidence = numeral, i-2, pagemap.FromHead
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
	if got := contentsPages([]string{prose}, Grammar{Pilcrow, Bare}, false); len(got) != 0 {
		t.Errorf("%d pages of prose read as contents", len(got))
	}
	if got := contentsPages([]string{contents}, Grammar{Column, Bare}, false); len(got) != 1 {
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

// frenchContents is the head of the full table of contents the 2019 French
// Topologie algebrique prints at the back, copied off pdf pages 507 and 509.
// Every awkward thing in it is the volume's, not mine: the em dash after the
// chapter numeral, the title that runs the whole width and leaves a single
// space in front of its page, and the one run of exercises for the chapter.
const frenchContents = `                     TABLE DES MATIÈRES

CHAPITRE I. — REVÊTEMENTS . . . . . . . . . . . . . . . . . . . . . . 1
    § 1. Produits fibrés et carrés cartésiens. . . . . . . . . . . . . 1
          1. Structure de B-espace. . . . . . . . . . . . . . . . . . . 1
          2. Opérations sur les B-espaces . . . . . . . . . . . . . . . 2
    § 2. Applications étales. . . . . . . . . . . . . . . . . . . . . . 5
          1. Applications séparées. . . . . . . . . . . . . . . . . . . 5
          2. Produit d’un espace par un espace simplement connexe 8
    Exercices. . . . . . . . . . . . . . . . . . . . . . . . . . . . . 13
`

// A French volume reads at all, and the three things that stopped it reading
// are each checked: the chapter line, the page with no leaders in front of it,
// and the single run of exercises that belongs to the chapter and not to § 2.
func TestParseFrench(t *testing.T) {
	res, err := Parse([]string{frenchContents}, testMapFor("I"),
		Options{Book: "test", Chapters: []string{"I"}})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := res.Get("I")
	if !ok {
		t.Fatalf("no chapter I, problems %v", res.Problems)
	}
	if c.Title != "REVÊTEMENTS" || c.Page != 1 {
		t.Errorf("chapter = %q printed %d", c.Title, c.Page)
	}
	if len(c.Sections) != 2 {
		t.Fatalf("%d sections, want 2", len(c.Sections))
	}
	s, _ := c.Get(2)
	if len(s.Subsections) != 2 {
		t.Fatalf("§ 2 has %d no., want 2", len(s.Subsections))
	}
	if got := s.Subsections[1]; got.Page != 8 || got.Title != "Produit d’un espace par un espace simplement connexe" {
		t.Errorf("no. 2 = %+v, want page 8 and the whole title", got)
	}
	if s.Exercises != nil {
		t.Error("the exercises were left on § 2")
	}
	if c.Exercises == nil || c.Exercises.Page != 13 {
		t.Errorf("chapter exercises = %+v, want page 13", c.Exercises)
	}
}

// A title that ends in a numeral must not lose it to the page reader. The guard
// is that taking the number off has to leave the line saying the same thing,
// and here it does not: "2. Fonctions continues sur CΛ" is no. 2 either way,
// but the number is part of the title and the line carries no page at all, so
// the entry is held for the wrapped line that does.
func TestNoLeaderKeepsAWrappedTitle(t *testing.T) {
	const pg = `CHAPITRE I. — REVÊTEMENTS . . . . . . . . . . . . . . . . . . . . . . 1
    § 1. Produits fibrés . . . . . . . . . . . . . . . . . . . . . . . 1
          1. Structure de B-espace. . . . . . . . . . . . . . . . . . . 1
          2. Fonctions continues sur un sous-ensemble compact
             de CΛ . . . . . . . . . . . . . . . . . . . . . . . . . . 3
`
	res, err := Parse([]string{pg}, testMapFor("I"), Options{Book: "test", Chapters: []string{"I"}})
	if err != nil {
		t.Fatal(err)
	}
	c, _ := res.Get("I")
	s, ok := c.Get(1)
	if !ok || len(s.Subsections) != 2 {
		t.Fatalf("§ 1 = %+v", s)
	}
	if got := s.Subsections[1]; got.Page != 3 {
		t.Errorf("no. 2 starts at printed page %d, want 3", got.Page)
	}
}

// The English Lie volume prints its chapter line with no page, because the
// chapter begins where its first § begins.
func TestChapterWithNoPageTakesItsFirstSection(t *testing.T) {
	const pg = `CHAPTER I CARTAN SUBALGEBRAS AND REGULAR ELEMENTS
§ 1. Primary decomposition . . . . . . . . . . . . . . . . . . . . . . 1
      1. Decomposition of a family . . . . . . . . . . . . . . . . . . 1
      2. The case of a linear family . . . . . . . . . . . . . . . . . 6
`
	res, err := Parse([]string{pg}, testMapFor("I"), Options{Book: "test", Chapters: []string{"I"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) > 0 {
		t.Errorf("problems: %v", res.Problems)
	}
	c, ok := res.Get("I")
	if !ok || c.Page != 1 {
		t.Fatalf("chapter = %+v, want printed page 1", c)
	}
}

// The English Theory of Sets closes with a part that carries §§ of its own,
// numbered from 1. They are not the last chapter's.
func TestPartEndsTheChapter(t *testing.T) {
	const pg = `CHAPTER I STRUCTURES . . . . . . . . . . . . . . . . . . . . . . . . . 1
§ 1. Structures and isomorphisms . . . . . . . . . . . . . . . . . . . 1
§ 2. Morphisms . . . . . . . . . . . . . . . . . . . . . . . . . . . . 4
SUMMARY OF RESULTS . . . . . . . . . . . . . . . . . . . . . . . . . . 8
§ 1. Elements and subsets of a set . . . . . . . . . . . . . . . . . . 8
§ 2. Functions . . . . . . . . . . . . . . . . . . . . . . . . . . . . 9
`
	res, err := Parse([]string{pg}, testMapFor("I"), Options{Book: "test", Chapters: []string{"I"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) > 0 {
		t.Errorf("problems: %v", res.Problems)
	}
	c, _ := res.Get("I")
	if len(c.Sections) != 2 {
		t.Errorf("%d sections, want the chapter's 2 and none of the part's", len(c.Sections))
	}
}

// The running head over the contents is set flush left and in capitals, like a
// part, and closing the chapter on it would throw the page away.
func TestContentsRunningHeadIsNotAPart(t *testing.T) {
	for _, head := range []string{"CONTENTS", "TABLE DES MATIÈRES",
		"334                           CHAPITRETABLE DES MATIÈRES"} {
		if isPart(head) {
			t.Errorf("isPart(%q) = true", head)
		}
	}
	if !isPart("SUMMARY OF RESULTS") {
		t.Error(`isPart("SUMMARY OF RESULTS") = false`)
	}
	// The 1998 scan reads small capitals as lowercase, so a roman page number
	// at the foot of a contents page comes out "XVlll" and is four fifths
	// capitals by accident. It is too short to be a heading.
	if isPart("XVlll") {
		t.Error(`isPart("XVlll") = true, the foot page number ended a chapter`)
	}
}

// The two printings of Algebra chapter 8 number their appendices differently.
func TestReadOrdinalTakesRomanOrArabic(t *testing.T) {
	for _, tt := range []struct {
		in   string
		want int
	}{{"I", 1}, {"II", 2}, {"1", 1}, {"2", 2}, {"", 0}} {
		got, ok := readOrdinal(tt.in)
		if tt.want == 0 {
			if ok {
				t.Errorf("readOrdinal(%q) = %d, want no number", tt.in, got)
			}
			continue
		}
		if !ok || got != tt.want {
			t.Errorf("readOrdinal(%q) = %d %v, want %d", tt.in, got, ok, tt.want)
		}
	}
}

// The scan of the English Theory of Sets sets no. 6 of III § 7 at "204-", and
// dropping it renumbered every no. after it.
func TestTailSurvivesTheScannersPunctuation(t *testing.T) {
	line := "         6. Direct systems of mappings . . . . . . . . . . . . . . . . 204-"
	text, tl, ok := splitTail(line, Bare)
	if !ok || tl.page != 204 {
		t.Fatalf("splitTail = %q %+v %v", text, tl, ok)
	}
}

// A volume that prints a short summary at the front and the full contents at
// the back yields every chapter twice, and the fuller listing is the one to
// keep.
func TestMergeChaptersKeepsTheFullerListing(t *testing.T) {
	summary := corpus.Chapter{Numeral: "I", Page: 1,
		Sections:  []corpus.Section{{Number: 1, Page: 1}, {Number: 2, Page: 5}},
		Exercises: &corpus.Locator{Page: 13}}
	full := corpus.Chapter{Numeral: "I", Page: 1, Sections: []corpus.Section{
		{Number: 1, Page: 1, Subsections: []corpus.Subsection{{Number: 1, Page: 1}}},
		{Number: 2, Page: 5, Subsections: []corpus.Subsection{{Number: 1, Page: 5}}},
	}}
	got := mergeChapters([]corpus.Chapter{summary, full})
	if len(got) != 1 {
		t.Fatalf("%d chapters, want 1", len(got))
	}
	if n := len(got[0].Sections[0].Subsections); n != 1 {
		t.Errorf("§ 1 has %d no., want the full listing's 1", n)
	}
	if got[0].Exercises == nil || got[0].Exercises.Page != 13 {
		t.Errorf("exercises = %+v, want the summary's page 13", got[0].Exercises)
	}
}

// The 1995 General Topology scan reads a page number four ways the reader had
// no name for, and every one of them is a contents line thrown away.
//
// The volume prints 190, 191, 197, 198, 219, 169, 225 and 207, and the text
// layer gives IgO, IgI, Ig7, Ig8, 2Ig, "J 69", "22-5" and "20   7". Ten lines
// end that way. Two of them are §§ of chapter III, which came out with five §§
// where the book has seven, and one is § 1 of chapter II, which came out at
// printed page 69, a hundred pages before the chapter starts.
func TestThePageNumbersThe1995ScanMisreads(t *testing.T) {
	tests := []struct {
		line string
		want int
	}{
		{"           6. Extension of uniformly continuous functions . . . . . . . . .        IgO", 190},
		{"           7. The completion of a uniform space ................        IgI", 191},
		{"          I. Uniformity of compact spaces. . . . . . . . . . . . . . . . . . . .        Ig8", 198},
		{"    § I. Topologies on groups. . . . . . . . . . . . . . . . . . . . . . . . . . . . . . ..          2Ig", 219},
		{"     § I. Uniform spaces .....................................        J 69", 169},
		{"            homogeneous spaces, product groups .............          22-5", 225},
		{"    Exercises for § 2                                                     20   7", 207},
	}
	for _, tt := range tests {
		_, got, ok := splitTail(tt.line, Bare)
		if !ok || got.page != tt.want {
			t.Errorf("splitTail(%q) = %+v %v, want page %d", tt.line, got, ok, tt.want)
		}
	}
}

// The same scan reads the lining figure 1 at the head of a no. line as J, and
// sets a middle dot after the number where the volume prints a period.
//
// A no. line that is not read is not one entry lost. The no. after it takes its
// place, so § 3 of chapter I came out starting at page 37, which is where its
// no. 2 is, and every no. of the § was reported as the wrong one.
func TestTheNumbersThe1995ScanMisreadsAtTheHeadOfALine(t *testing.T) {
	tests := []struct {
		line string
		want entry
	}{
		{"          J. Subspaces of a topological space",
			entry{kind: kindSubsection, number: 1, title: "Subspaces of a topological space"}},
		{"          J. Hausdorff spaces",
			entry{kind: kindSubsection, number: 1, title: "Hausdorff spaces"}},
		{"           9· Completion of subspaces and product spaces",
			entry{kind: kindSubsection, number: 9, title: "Completion of subspaces and product spaces"}},
	}
	for _, tt := range tests {
		if got := classify(tt.line, Pilcrow); got != tt.want {
			t.Errorf("classify(%q) = %+v, want %+v", tt.line, got, tt.want)
		}
	}
}

// And a title that ends in a letter of that class is still a title, because
// nothing but the leader dots lets a page be read at all.
func TestAWordIsNotAPageNumber(t *testing.T) {
	if _, _, ok := splitTail("4. Ultrafilters", Bare); ok {
		t.Error("a line with no leaders yielded a page")
	}
}

// The 1987 Topological Vector Spaces scan reads the 5 of a page label as an S
// and the two I's of a chapter numeral as an N, and both are allowed only where
// the label says a page is what follows.
//
// Five lines of that volume end in one of the two, and each of them is a no. of
// chapter II or IV that the volume lists and the manifest did not have.
func TestThePageLabelsThe1987ScanMisreads(t *testing.T) {
	tests := []struct {
		line    string
		chapter string
		page    int
	}{
		{"           6. Separately continuous bilinear mappings. . . . . . . . . ..     IV.lS", "IV", 15},
		{"           5. Ordered vector spaces ................................ .        n.12", "II", 12},
	}
	for _, tt := range tests {
		_, got, ok := splitTail(tt.line, Label)
		if !ok || got.chapter != tt.chapter || got.page != tt.page {
			t.Errorf("splitTail(%q) = %+v %v, want %s.%d", tt.line, got, ok, tt.chapter, tt.page)
		}
	}
}

// The 2004 Integration scan runs the chapter numeral of a label into the page
// it labels, and misreads letters on the way through.
//
// Sixty three of the entries of that volume ended in one of these, which is
// most of chapter V, the whole of the run of exercises of chapters II and III,
// and chapter I entire. The chapter each line is read in is what tells "ILl5"
// that it is II.15 and not III.5.
func TestTheLabelsThe2004IntegrationScanRunsTogether(t *testing.T) {
	tests := []struct {
		tok     string
		in      string
		chapter string
		page    int
	}{
		{"Ll", "", "I", 1},          // chapter I, page 1
		{"104", "I", "I", 4},        // the period read as a nought
		{"1104", "II", "II", 4},     // the numeral read as two ones, and the same nought
		{"ILl", "II", "II", 1},      // the numeral run into the page with no period at all
		{"IIL13", "III", "III", 13}, // and the same, two figures
		{"IliA", "III", "III", 4},   // the period and the four read as one letter
		{"YA8", "V", "V", 48},       // the V read as a Y, and the same letter
		{"Y.25", "V", "V", 25},
		{"ILl5", "II", "II", 15},  // II.15, which is also a reading of III.5
		{"IIL5", "III", "III", 5}, // and III.5, which the chapter tells apart from it
		{"II.lO", "II", "II", 10}, // the 1987 scan, whose O is a nought and not a stray
		{"IV. 11 0", "IV", "IV", 110},
	}
	for _, tt := range tests {
		ch, p, ok := readLabel(tt.tok, tt.in)
		if !ok || ch != tt.chapter || p != tt.page {
			t.Errorf("readLabel(%q, %q) = %s.%d %v, want %s.%d",
				tt.tok, tt.in, ch, p, ok, tt.chapter, tt.page)
		}
	}
}

// A numeral that is not written the way its number is written is not a chapter
// numeral. RomanOrder reads IIII as four, and every label the 2004 scan runs
// together offers one of those as a reading.
func TestANumeralNobodyWritesIsNotAChapter(t *testing.T) {
	for _, s := range []string{"IIII", "VIIII", "IIIII"} {
		if isCanonicalRoman(s) {
			t.Errorf("%q was taken for a chapter numeral", s)
		}
	}
	for _, s := range []string{"I", "II", "III", "IV", "V", "VIII", "IX", "X"} {
		if !isCanonicalRoman(s) {
			t.Errorf("%q was refused as a chapter numeral", s)
		}
	}
}

// A running head with the folio in front of it is not a heading.
//
// It is set the way a chapter line is set, flush left and in capitals, and
// reading it as one closes the chapter and throws away every entry on the page
// it heads.
func TestARunningHeadWithAFolioIsNotAHeading(t *testing.T) {
	if isPart("360                       TOPOLOGICAL VECTOR SPACES") {
		t.Error("a running head was read as a heading")
	}
	if !isPart("SUMMARY OF RESULTS") {
		t.Error("a heading was not read as one")
	}
}

// A contents printed at the back of the volume runs over several pages, and the
// pages after the first carry the running head of the volume rather than the
// word Contents. They belong to the same contents.
//
// Topological Vector Spaces is where this was measured. Its contents is at the
// back, the scanner sets the versos "360 TOPOLOGICAL VECTOR SPACES", and
// chapter II came out with one § where the volume prints eight.
func TestAContentsThatRunsOverTheBackMatterKeepsItsVersos(t *testing.T) {
	const recto = `                                    Contents

CHAPTER VIII SEMISIMPLE MODULES AND RINGS . . . . . . . . . . . . . 1

1. Simple Modules . . . . . . . . . . . . . . . . . . . . . . . . . . . . 1
   1. Simple Modules . . . . . . . . . . . . . . . . . . . . . . . . . . 1
`
	const verso = `18                    SEMISIMPLE MODULES AND RINGS

   2. Simple Modules over a Ring . . . . . . . . . . . . . . . . . . . . 3
2. Semisimple Modules . . . . . . . . . . . . . . . . . . . . . . . . . . 7
   1. Direct Sums . . . . . . . . . . . . . . . . . . . . . . . . . . . . 7
`
	pages := make([]string, 22)
	pages[19], pages[20] = recto, verso
	res, err := Parse(pages, testMap(), Options{Book: "test", Chapters: []string{"VIII"}})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := res.Get("VIII")
	if !ok {
		t.Fatal("no chapter VIII")
	}
	if len(c.Sections) != 2 {
		t.Fatalf("chapter VIII has %d §, want 2", len(c.Sections))
	}
	if got := len(c.Sections[0].Subsections); got != 2 {
		t.Errorf("§ 1 has %d no., want 2, so the verso was not read", got)
	}
}

// A contents line the text layer sets on two lines, the number and the page on
// one and the title on the next, is read as one line.
//
// The 2003 Functions of a Real Variable does this fifteen times. The title line
// carries no page, so it was dropped, and the numbered line was dropped with it
// for having no title. Chapter V lost five of the nos of its first two §§ and
// three of the six of its appendix.
func TestATitleTheTextLayerSetOnItsOwnLine(t *testing.T) {
	lines := []string{
		"     6.                                                         ......... 188",
		"         Linear differential equations with constant coefficients",
		"    7.   Linear equations of order ii   ............................... 192",
	}
	got := mend(lines, Grammar{Pilcrow, Bare})
	text, tl, ok := splitTail(got[0], Bare)
	if !ok || tl.page != 188 {
		t.Fatalf("splitTail(%q) = %+v %v, want page 188", got[0], tl, ok)
	}
	want := entry{kind: kindSubsection, number: 6,
		title: "Linear differential equations with constant coefficients"}
	if e := classify(text, Pilcrow); e != want {
		t.Errorf("classify(%q) = %+v, want %+v", text, e, want)
	}
	if strings.TrimSpace(got[1]) != "" {
		t.Errorf("the title line was left behind as %q", got[1])
	}
	if got[2] != lines[2] {
		t.Errorf("a whole line was rewritten to %q", got[2])
	}
}

// And a line that carries a title of its own is not mended, whatever follows it.
func TestALineWithATitleIsLeftAlone(t *testing.T) {
	lines := []string{
		"    5.   Adjoint equation ........................................           186",
		"    6.   Linearity of the integrals ............................. 179",
	}
	if got := mend(lines, Grammar{Pilcrow, Bare}); got[0] != lines[0] || got[1] != lines[1] {
		t.Errorf("mend rewrote a pair of complete lines to %q", got)
	}
}

// A chapter that prints no § owns its nos itself.
//
// Chapter I of the English Integration is the one chapter of the library that
// does this: three nos straight under the chapter heading, then the run of
// exercises the volume names for the chapter, then the historical note, and
// never a § anywhere. The lines here are its contents entries with the leaders
// shortened. Nothing in the manifest used to be able to hold them, and a
// manifest that left them out would say the chapter has no content at all.
func TestAChapterWithNoSectionOwnsItsNos(t *testing.T) {
	const contents = `                                    Contents

CHAPTER VIII INEQUALITIES OF CONVEXITY . . . . . . . . . . . . . . . . 1
   1. The fundamental inequality of convexity . . . . . . . . . . . . . 1
   2. The inequalities of Holder and Minkowski . . . . . . . . . . . . . 3
   3. The semi-norms Np . . . . . . . . . . . . . . . . . . . . . . . . . 4
Exercises for Ch. VIII . . . . . . . . . . . . . . . . . . . . . . . . . 6
Historical note . . . . . . . . . . . . . . . . . . . . . . . . . . . . . 8
`
	res, err := Parse([]string{contents}, testMap(), Options{Book: "test", Chapters: []string{"VIII"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) > 0 {
		t.Fatalf("problems: %v", res.Problems)
	}
	c, ok := res.Get("VIII")
	if !ok {
		t.Fatal("no chapter VIII")
	}
	if len(c.Sections) != 0 {
		t.Errorf("%d §, want none: %+v", len(c.Sections), c.Sections)
	}
	if len(c.Subsections) != 3 {
		t.Fatalf("%d no. on the chapter, want 3: %+v", len(c.Subsections), c.Subsections)
	}
	want := []corpus.Subsection{
		{Number: 1, Title: "The fundamental inequality of convexity", Page: 1, PDFPage: 3},
		{Number: 2, Title: "The inequalities of Holder and Minkowski", Page: 3, PDFPage: 5},
		{Number: 3, Title: "The semi-norms Np", Page: 4, PDFPage: 6},
	}
	for i, sub := range c.Subsections {
		if sub != want[i] {
			t.Errorf("no. %d = %+v, want %+v", i+1, sub, want[i])
		}
	}
	// The run of exercises is named for the chapter, because there is no § to
	// name it for, so it goes where a chapter that gathers its exercises at the
	// end keeps its own.
	if c.Exercises == nil {
		t.Fatal("the chapter's exercises were dropped")
	}
	if c.Exercises.Page != 6 || c.Exercises.PDFPage != 8 {
		t.Errorf("exercises at printed %d, pdf %d, want 6 and 8", c.Exercises.Page, c.Exercises.PDFPage)
	}
	if c.Historical == nil || c.Historical.Page != 8 {
		t.Errorf("historical note = %+v, want printed page 8", c.Historical)
	}
	if _, _, sub, ex := res.Counts(); sub != 3 || ex != 1 {
		t.Errorf("counts say %d no. and %d exercise runs, want 3 and 1", sub, ex)
	}
}

// A chapter that ends up holding both is a § line misread as a no.
//
// The two cannot be told apart on the line, because the § that settles it comes
// further down the page, so the reader collects the nos and the refusal is made
// once the whole contents has been read.
func TestAChapterHoldingBothNosAndSectionsIsRefused(t *testing.T) {
	const contents = `                                    Contents

CHAPTER VIII INEQUALITIES OF CONVEXITY . . . . . . . . . . . . . . . . 1
   1. The fundamental inequality of convexity . . . . . . . . . . . . . 1
§ 1. Riesz spaces . . . . . . . . . . . . . . . . . . . . . . . . . . . . 3
   1. Definition of Riesz spaces . . . . . . . . . . . . . . . . . . . . 3
`
	res, err := Parse([]string{contents}, testMap(), Options{Book: "test", Chapters: []string{"VIII"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) != 1 {
		t.Fatalf("%d problems, want one: %v", len(res.Problems), res.Problems)
	}
	if p := res.Problems[0]; p.Chapter != "VIII" || !strings.Contains(p.Detail, "was read as a no.") {
		t.Errorf("problem = %v", p)
	}
}

// A numbered list inside an entry is not a run of nos, on the page it starts on
// or on the page it runs over onto.
//
// Chapter VII of the English Integration 7 to 9 lists no. 3 of its § 3 as
// "Examples: 1. General linear group" and sets the other seven examples on
// their own lines. Read as nos they take the run to eleven where the volume
// prints three, and the three problems that come of it kept the whole volume
// out of the manifest.
func TestAListInsideAnEntryIsNotARunOfNos(t *testing.T) {
	const first = `                                    Contents

CHAPTER VIII HAAR MEASURE . . . . . . . . . . . . . . . . . . . . . . . 1
1. Applications and examples . . . . . . . . . . . . . . . . . . . . . . 1
   1. Compact groups of linear mappings . . . . . . . . . . . . . . . . . 1
   2. Triviality of fibered spaces . . . . . . . . . . . . . . . . . . . 3
   3. Examples: 1. General linear group . . . . . . . . . . . . . . . . . 5
                2. Affine group . . . . . . . . . . . . . . . . . . . . . 6
                3. Strict triangular group . . . . . . . . . . . . . . . 7
                4. Large triangular group . . . . . . . . . . . . . . . . 8
                5. Special triangular group . . . . . . . . . . . . . . . 9
`
	const second = `18                              INTEGRATION

                6. Special linear group . . . . . . . . . . . . . . . . . 10
                7. Iwasawa decomposition . . . . . . . . . . . . . . . . . 10
2. The space of closed subgroups . . . . . . . . . . . . . . . . . . . . 11
   1. The space of Haar measures . . . . . . . . . . . . . . . . . . . . 11
`
	pages := make([]string, 22)
	pages[19], pages[20] = first, second
	res, err := Parse(pages, testMap(), Options{Book: "test", Chapters: []string{"VIII"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) > 0 {
		t.Errorf("problems: %v", res.Problems)
	}
	c, ok := res.Get("VIII")
	if !ok {
		t.Fatal("no chapter VIII")
	}
	if len(c.Sections) != 2 {
		t.Fatalf("chapter VIII has %d §, want 2", len(c.Sections))
	}
	if got := len(c.Sections[0].Subsections); got != 3 {
		t.Errorf("§ 1 has %d no., want 3, so the examples were read as nos", got)
	}
	if got := len(c.Sections[1].Subsections); got != 1 {
		t.Errorf("§ 2 has %d no., want 1", got)
	}
}

// A no. that carries on the numbering of its own run after a page break is a
// no. and not part of the list above it. The English Algebra I breaks the nos
// of chapter III § 8 between 9 and 10 and sets the second page nine columns
// further in.
func TestARunThatCarriesOnOverAPageBreakIsARun(t *testing.T) {
	const first = `                                    Contents

CHAPTER VIII HAAR MEASURE . . . . . . . . . . . . . . . . . . . . . . . 1
1. Projective spaces . . . . . . . . . . . . . . . . . . . . . . . . . . 1
     8. Projective completion of an affine space . . . . . . . . . . . . 3
     9. Extension of rational functions . . . . . . . . . . . . . . . . . 4
`
	const second = `                                   CONTENTS

              10. Projective linear mappings . . . . . . . . . . . . . . 5
              11. Projective space structure . . . . . . . . . . . . . . 7
`
	pages := make([]string, 22)
	pages[19], pages[20] = first, second
	res, err := Parse(pages, testMap(), Options{Book: "test", Chapters: []string{"VIII"}})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := res.Get("VIII")
	if !ok {
		t.Fatal("no chapter VIII")
	}
	if got := len(c.Sections[0].Subsections); got != 4 {
		t.Errorf("§ 1 has %d no., want 4, so the page break lost the rest of the run", got)
	}
}

// An appendix the volume calls an annex is an appendix, and so is the run of
// exercises that goes with it. Chapter IX of the English Integration 7 to 9 is
// the only one of them that does this.
func TestAnAnnexIsAnAppendix(t *testing.T) {
	e := classify("ANNEX: Complements on Hilbert spaces", Pilcrow)
	if e.kind != kindAppendix || e.title != "Complements on Hilbert spaces" {
		t.Errorf("classify of the annex = %+v", e)
	}
	x := classify("Exercises for the Annex", Pilcrow)
	if x.kind != kindExercises || !x.appendix {
		t.Errorf("classify of the exercises of the annex = %+v", x)
	}
}

// An appendix with no title is not missing anything. Chapter VII of the English
// Integration 7 to 9 prints "Appendix I" and "Appendix II" bare, in the
// contents and over the appendices themselves, and both were reported.
func TestAnAppendixNeedsNoTitle(t *testing.T) {
	const contents = `                                    Contents

CHAPTER VIII HAAR MEASURE . . . . . . . . . . . . . . . . . . . . . . . 1
1. Construction of a Haar measure . . . . . . . . . . . . . . . . . . . . 1
   1. Definitions and notations . . . . . . . . . . . . . . . . . . . . . 1
Appendix I . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . . 5
   1. Polynomial maps . . . . . . . . . . . . . . . . . . . . . . . . . . 5
`
	res, err := Parse([]string{contents}, testMap(), Options{Book: "test", Chapters: []string{"VIII"}})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range res.Problems {
		if strings.Contains(p.Detail, "no title") {
			t.Errorf("problem = %v", p)
		}
	}
}

// A line whose title runs the width of the page carries its label with nothing
// but a space in front of it, and the label still has to name the chapter the
// line is in.
//
// The 2004 Integration sets the title of chapter IX this way, and the label
// stayed on the end of the title all the way into the manifest.
func TestALabelWithNoLeadersIsStillALabel(t *testing.T) {
	const line = "CHAPTER IX. - MEASURES ON HAUSDORFF TOPOLOGICAL SPACES IX.l"
	text, tl, ok, _ := noLeaderLabel(line, line, classify(line, Pilcrow), Pilcrow, "IX")
	if !ok || tl.chapter != "IX" || tl.page != 1 {
		t.Fatalf("noLeaderLabel(%q) = %+v %v, want IX.1", line, tl, ok)
	}
	if strings.HasSuffix(text, "IX.l") {
		t.Errorf("the label was left on the title: %q", text)
	}
	// And a title that ends in something that is not the label of the chapter
	// it is in is left alone.
	const other = "     2. Bounded measures and linear forms on L2"
	if _, _, ok, _ := noLeaderLabel(other, other, classify(other, Pilcrow), Pilcrow, "IX"); ok {
		t.Errorf("the end of %q was read as a label", other)
	}
}

// The two fixtures below are the contents of the French Espaces vectoriels
// topologiques as pdftotext -layout returns it and as the model reads the same
// page image. The scan captured the titles and the leader dots and not one of
// the page numbers, which stand in a column of their own at the right margin.
const evtLayer = `                                      Table des matières


       § 1 . Espaces vectoriels topologiques . . . . . . . . . . . . . . . . . . . . . .
              1. Définition d'un espace vectoriel topologique . . . . . . . . . . . .
              2 . Espaces normés sur un corps valué . . . . . . . . . . . . . . . . . .`

const evtRead = `Table des matières

§ 1. Espaces vectoriels topologiques ....... II.1
    1. Définition d'un espace vectoriel topologique ....... II.1
    2. Espaces normés sur un corps valué ....... II.3`

func TestAReadingWithThePagesOnItBeatsALayerWithout(t *testing.T) {
	pages := []string{"front matter", evtLayer, "chapter I"}
	// nil for the page map. Espaces vectoriels topologiques prints chapters, so
	// there is nothing here the flat reading would change.
	out := Overlay(pages, map[int]string{2: evtRead}, nil)
	if out[1] != evtRead {
		t.Fatalf("the layer was kept over the reading:\n%s", out[1])
	}
	if out[0] != pages[0] || out[2] != pages[2] {
		t.Errorf("a page nobody read was replaced")
	}
	// The originals are not written through.
	if pages[1] != evtLayer {
		t.Errorf("Overlay wrote into the pages it was given")
	}
}

func TestAReadingThatCarriesLessThanTheLayerIsDropped(t *testing.T) {
	// What the ordinary page prompt returns for a contents page. It is a fair
	// reading of the words and it has thrown away every page number, which is
	// the whole of what the contents is for.
	const asHeadings = `## § 1. Espaces vectoriels topologiques
### 1. Définition d'un espace vectoriel topologique
### 2. Espaces normés sur un corps valué`
	pages := []string{evtRead}
	out := Overlay(pages, map[int]string{1: asHeadings}, nil)
	if out[0] != evtRead {
		t.Fatalf("a reading with no pages in it replaced one that had them:\n%s", out[0])
	}
}

// A scan that lost the last two words of the running head over the table of
// contents still has a table of contents. Groupes et algebres de Lie chapitres
// 4 a 6 in French and Integration chapitre 5 in French both come out of the
// text layer with "TABLE" and the folio and nothing else over every page of the
// contents at the back, and both were reported as volumes with no contents at
// all: one because every one of its pages carries a printed number and so the
// map leaves none out, the other because no page read as contents.
//
// What is not taken is a head that says what else it is the table of. The
// French Integration prints a TABLE DE CONCORDANCE between the two editions
// right after its contents, and that is a different thing with a different
// shape.
func TestAHeadThatIsNothingButTheWordTableAnnouncesTheContents(t *testing.T) {
	for _, c := range []struct {
		name string
		head string
		want bool
	}{
		{"recto", "                    TABLE                    287", true},
		{"verso", "286                                 TABLE", true},
		{"in full", "                    TABLE DES MATIÈRES", true},
		{"english", "                    CONTENTS", true},
		{"the concordance", "                    TABLE DE CONCORDANCE", false},
		{"the concordance on a verso", "154              TABLE DE CONCORDANCE", false},
		{"a table in the body", "  TABLE 1. The exceptional root systems", false},
	} {
		t.Run(c.name, func(t *testing.T) {
			pg := c.head + "\n\n§ 1. Hyperplans, chambres et facettes . . . 61\n"
			if got := announcesContents(pg); got != c.want {
				t.Errorf("announcesContents(%q) = %v, want %v", c.head, got, c.want)
			}
		})
	}
}

// The French Groupes et algebres de Lie chapitre 1 prints its contents over two
// pages and heads the second one ALGÈBRES DE LIE, which is set flush left and in
// capitals exactly as a part heading is. Reading it as a part closed chapter I
// on the first line of the page and threw away §§ 5, 6 and 7 along with all
// seven exercise runs, and the volume came out with four §§ where it prints
// seven, with nothing reported.
func TestContentsRunningHeadDoesNotCloseTheChapter(t *testing.T) {
	first := `TABLE DES MATIÈRES

CHAPITRE I. — Algèbres de Lie ............................ 1

§ 4. Algèbres de Lie nilpotentes ......................... 5
    1. Définition des algèbres de Lie nilpotentes ....... 5
    2. Le théorème d’Engel .............................. 7
`
	second := `ALGÈBRES DE LIE

3. Le plus grand idéal de nilpotence d’une représentation.. 8

§ 5. Algèbres de Lie résolubles ......................... 11
    1. Définition des algèbres de Lie résolubles ....... 12

Exercices du § 4......................................... 18
`
	res, err := Parse([]string{first, second}, testMapFor("I"),
		Options{Book: "test", Chapters: []string{"I"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Chapters) != 1 {
		t.Fatalf("got %d chapters, want 1", len(res.Chapters))
	}
	c := res.Chapters[0]
	if len(c.Sections) != 2 {
		t.Fatalf("chapter I got %d §, want 2", len(c.Sections))
	}
	if got := len(c.Sections[0].Subsections); got != 3 {
		t.Errorf("§ 4 got %d no., want 3", got)
	}
	if got := len(c.Sections[1].Subsections); got != 1 {
		t.Errorf("§ 5 got %d no., want 1", got)
	}
	if c.Sections[0].Exercises == nil || c.Sections[0].Exercises.Page != 18 {
		t.Errorf("§ 4 exercises = %v, want page 18", c.Sections[0].Exercises)
	}
}

// A volume of one chapter names no chapter in its contents, because the chapter
// is the volume. The French Integration chapter IX opens straight at § 1 and
// the numeral IX is nowhere on the page, and every § used to be dropped for
// want of a chapter to hang it on, which left the volume with no contents at
// all. The page map found the chapter, and when it found exactly one there is
// nothing to decide.
func TestASingleChapterVolumeNeedsNoChapterLine(t *testing.T) {
	headless := strings.Replace(contents,
		"CHAPTER VIII SEMISIMPLE MODULES AND RINGS . . . . . . . . . . . . . 1\n\n", "", 1)
	res, err := Parse([]string{headless}, testMap(),
		Options{Book: "test", Chapters: []string{"VIII"}, Title: "Semisimple Modules and Rings"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) > 0 {
		t.Errorf("problems: %v", res.Problems)
	}
	c, ok := res.Get("VIII")
	if !ok {
		t.Fatal("the chapter the page map found was not opened")
	}
	if c.Title != "SEMISIMPLE MODULES AND RINGS" {
		t.Errorf("title = %q, want the volume title in capitals", c.Title)
	}
	if c.Page != 1 || c.PDFPage != 3 {
		t.Errorf("chapter starts printed %d pdf %d, want 1 and 3", c.Page, c.PDFPage)
	}
	if len(c.Sections) != 3 {
		t.Errorf("%d sections, want the same 2 § and 1 appendix the chapter line gave", len(c.Sections))
	}
	if c.Historical == nil {
		t.Error("the historical note was lost")
	}
}

// With two chapters mapped the contents alone cannot say where the first ends,
// so a § with no chapter over it is still dropped and the volume is still
// reported as yielding nothing. Guessing here would put every § of the volume
// into whichever chapter happened to be first.
func TestTwoMappedChaptersImplyNothing(t *testing.T) {
	headless := strings.Replace(contents,
		"CHAPTER VIII SEMISIMPLE MODULES AND RINGS . . . . . . . . . . . . . 1\n\n", "", 1)
	pm := testMap()
	pm.Chapters = append(pm.Chapters, pagemap.Span{
		Chapter: "IX", FirstPDF: 23, LastPDF: 30, FirstPage: 21, LastPage: 28})
	res, err := Parse([]string{headless}, pm,
		Options{Book: "test", Chapters: []string{"VIII", "IX"}, Title: "Two Chapters"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Chapters) != 0 {
		t.Fatalf("%d chapters were invented", len(res.Chapters))
	}
	if len(res.Problems) == 0 {
		t.Error("a contents that yielded no chapters was not reported")
	}
}

// A label with no leaders in front of it also comes in pieces, because the
// scanners break a label wherever there is a place to break one. The old French
// Topologie generale sets no. 6 of chapter I § 8 with the title running the
// width of the line and the label in two pieces after it, and eleven more
// entries of that volume and of Topologie generale chapitres 5 a 10 go the same
// way. Read as one piece the pattern took the page on its own, which names no
// chapter, and the entry was lost.
func TestALabelWithNoLeadersComesInPieces(t *testing.T) {
	const line = "    6. Limites dans les espaces produits et les espaces quotients. I. 51"
	text, tl, ok, got := noLeaderLabel(line, line, classify(line, Pilcrow), Pilcrow, "I")
	if !ok || tl.chapter != "I" || tl.page != 51 {
		t.Fatalf("noLeaderLabel(%q) = %+v %v, want I.51", line, tl, ok)
	}
	if got.number != 6 || got.kind != kindSubsection {
		t.Errorf("the line stopped being no. 6: %+v", got)
	}
	if !strings.HasSuffix(text, "quotients.") {
		t.Errorf("the label was left on the title: %q", text)
	}
	// The same volume sets no. 4 of chapter I § 10 with no punctuation at all
	// between the title and the label.
	const bare = "    4. Image d'un espace compact par une application continue I. 62"
	if _, tl, ok, _ := noLeaderLabel(bare, bare, classify(bare, Pilcrow), Pilcrow, "I"); !ok || tl.page != 62 {
		t.Errorf("noLeaderLabel(%q) = %+v %v, want I.62", bare, tl, ok)
	}
	// A title whose last words could be pieced together into something is still
	// not a label, because what is pieced together has to name the chapter.
	const other = "    3. Sous-groupes et groupes quotients d'un groupe quotient"
	if _, _, ok, _ := noLeaderLabel(other, other, classify(other, Pilcrow), Pilcrow, "III"); ok {
		t.Errorf("the end of %q was read as a label", other)
	}
}

// wrappedLabel is a contents that numbers its pages by chapter and wraps one of
// its entries over three lines, setting the label on the last of them with no
// leaders in front of it. It is the French Algebre chapitres 1 a 3, no. 4 of
// chapter II § 3, cut down to one chapter and two §.
const wrappedLabel = `                              TABLE DES MATIÈRES

CHAPITRE II. — ALGÈBRE LINÉAIRE . . . . . . . . . . . . . . . . . II.1

§ 1. Modules . . . . . . . . . . . . . . . . . . . . . . . . . . . II.1
   1. Modules; applications linéaires . . . . . . . . . . . . . . II.1
§ 2. Produits tensoriels . . . . . . . . . . . . . . . . . . . . . II.60
   1. Produit tensoriel de deux modules . . . . . . . . . . . . . II.60
   2. Produit tensoriel de deux applications linéaires . . . . . . II.65
   3. Changement de l'anneau de base . . . . . . . . . . . . . . . II.72
   4. L'homomorphisme
      Hom_C (E_1, F_1) $ \otimes_C $ Hom_C (E_2, F_2)
      $ \to $ Hom_C (E_1 $ \otimes_C $ E_2, F_1 $ \otimes_C $ F_2) II.79
`

// wrappedLabelMap is chapter II of 90 printed pages at a constant offset of 2.
func wrappedLabelMap() *pagemap.Map {
	m := &pagemap.Map{Book: "test", PDFPages: 92,
		Chapters: []pagemap.Span{{Chapter: "II", FirstPDF: 3, LastPDF: 92, FirstPage: 1, LastPage: 90}}}
	for i := 1; i <= 92; i++ {
		e := pagemap.Entry{PDFPage: i, Confidence: pagemap.Unknown}
		if i >= 3 {
			e.Chapter, e.Page, e.Confidence = "II", i-2, pagemap.FromHead
		}
		m.Entries = append(m.Entries, e)
	}
	return m
}

func TestAWrappedEntryFindsALabelWithNoLeaders(t *testing.T) {
	res, err := Parse([]string{wrappedLabel}, wrappedLabelMap(),
		Options{Book: "test", Chapters: []string{"II"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) > 0 {
		t.Fatalf("problems: %v", res.Problems)
	}
	c, ok := res.Get("II")
	if !ok {
		t.Fatal("no chapter II")
	}
	var s *corpus.Section
	for i := range c.Sections {
		if c.Sections[i].Number == 2 {
			s = &c.Sections[i]
		}
	}
	if s == nil {
		t.Fatal("no § 2")
	}
	if len(s.Subsections) != 4 {
		t.Fatalf("§ 2 has %d no., want 4", len(s.Subsections))
	}
	last := s.Subsections[3]
	if last.Number != 4 || last.Page != 79 {
		t.Errorf("no. 4 = no. %d printed %d, want no. 4 printed 79", last.Number, last.Page)
	}
	if !strings.HasPrefix(last.Title, "L'homomorphisme") || strings.Contains(last.Title, "II.79") {
		t.Errorf("title = %q, want the wrapped title with the label off it", last.Title)
	}
}
