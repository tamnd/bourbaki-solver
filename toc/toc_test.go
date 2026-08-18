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
