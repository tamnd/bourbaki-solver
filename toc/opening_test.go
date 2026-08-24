package toc

import (
	"strconv"
	"strings"
	"testing"
)

// Every line in these tests is off a page of the corpus, and every title is
// what manifests/toc/ gives for the chapter or the § it belongs to.

func TestAChapterTitleBrokenAcrossTwoLinesIsOneHeading(t *testing.T) {
	// Page 22 of Theory of Sets. The press broke the title at the measure and
	// the reading kept the break, so neither line says what the contents says
	// and both together do.
	body := []string{
		"",
		"Description",
		"of Formal Mathematics",
		"",
		"## 1. TERMS AND RELATIONS",
	}
	out, ok := ChapterOpening(body, "en", "I", "DESCRIPTION OF FORMAL MATHEMATICS")
	if !ok {
		t.Fatal("the contents gives this chapter that title and the page carries it")
	}
	want := []string{
		"",
		"## CHAPTER I",
		"",
		"# Description of Formal Mathematics",
		"",
		"## 1. TERMS AND RELATIONS",
	}
	if len(out) != len(want) {
		t.Fatalf("got %d lines, want %d: %q", len(out), len(want), out)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("line %d is %q, want %q", i, out[i], want[i])
		}
	}
}

func TestASubtitleUnderAChapterTitleIsNotPartOfIt(t *testing.T) {
	// Page 225 of Topology I to IV. The contents calls the chapter
	// "Topological Groups" and the page sets what it is a theory of under it,
	// so the join stops at the first line and the second stays prose.
	body := []string{
		"",
		"Topological Groups",
		"(Elementary Theory)",
		"",
		"I. TOPOLOGIES ON GROUPS",
	}
	out, ok := ChapterOpening(body, "en", "III", "Topological Groups")
	if !ok {
		t.Fatal("the first line is the title the contents gives")
	}
	if out[1] != "## CHAPTER III" || out[3] != "# Topological Groups" {
		t.Fatalf("got %q", out[:4])
	}
	if out[4] != "(Elementary Theory)" {
		t.Errorf("the subtitle is %q, want it left where it was", out[4])
	}
}

func TestAChapterWhosePageHasLostItsTitleIsRefused(t *testing.T) {
	// Nothing at the top of this page says which chapter it is, so there is
	// nothing for the contents to agree with and the heading is not written.
	body := []string{
		"",
		"### 1. GROUPS",
		"",
		"Recall the following definition (§ 2, no. 3, Definition 6).",
	}
	if _, ok := ChapterOpening(body, "en", "I", "ALGEBRAIC STRUCTURES"); ok {
		t.Fatal("the page does not carry the title and the heading was written anyway")
	}
}

func TestASectionHeadingKeepsTheSignThePagePrints(t *testing.T) {
	// Page 10 of Algebra IV to VII. That printing sets the sign and the
	// heading comes back with everything but the level.
	body := []string{
		"Throughout this chapter $ \\mathbf{A} $ denotes a commutative ring.",
		"",
		"§ 1. POLYNOMIALS",
		"",
		"### 1. Definition of polynomials",
	}
	from, to, head, ok := SectionOpening(body, 1, "Polynomials")
	if !ok {
		t.Fatal("the contents opens § 1 on this page under this title")
	}
	if from != 2 || to != 2 {
		t.Errorf("rewrote lines %d to %d, want line 2 alone", from, to)
	}
	if want := "## § 1. POLYNOMIALS"; head != want {
		t.Errorf("got %q, want %q", head, want)
	}
}

func TestASignRunIntoItsNumberIsStillASection(t *testing.T) {
	// Page 7 of Integration VII to IX, and the other twelve § openings of that
	// volume, which are all set the same way. Requiring a space after the sign
	// refused every one of them and the volume did not assemble at all.
	body := []string{
		"All locally convex spaces will be assumed to be Hausdorff.",
		"",
		"§1. CONSTRUCTION OF A HAAR MEASURE",
		"",
		"1. Definitions and notations",
	}
	from, to, head, ok := SectionOpening(body, 1, "Construction of a Haar measure")
	if !ok {
		t.Fatal("the contents opens § 1 on this page under this title")
	}
	if from != 2 || to != 2 {
		t.Errorf("rewrote lines %d to %d, want line 2 alone", from, to)
	}
	if want := "## § 1. CONSTRUCTION OF A HAAR MEASURE"; head != want {
		t.Errorf("got %q, want %q", head, want)
	}
}

func TestTheSpaceAfterTheSignIsWrittenTheCorpusWay(t *testing.T) {
	// The page decides whether there is a sign and the corpus decides the
	// spacing. All 143 § headings in the corpus that carry a sign are set
	// "## § N.", so a page that ran the two together gets the space put in.
	for _, line := range []string{"§1. POLYNOMIALS", "§ 1. POLYNOMIALS", "§   1. POLYNOMIALS"} {
		_, _, head, ok := SectionOpening([]string{line}, 1, "Polynomials")
		if !ok {
			t.Fatalf("%q: the contents opens § 1 on that page", line)
		}
		if want := "## § 1. POLYNOMIALS"; head != want {
			t.Errorf("%q: got %q, want %q", line, head, want)
		}
	}
}

func TestANoStillRefusesASignRunIntoItsNumber(t *testing.T) {
	// The printings that set a sign set it over the § and never over a no., so
	// the sign is how the two are told apart where the number cannot do it.
	// Losing the space must not lose that.
	if _, _, _, ok := NumberOpening([]string{"§1. CONSTRUCTION OF A HAAR MEASURE"},
		1, "Construction of a Haar measure"); ok {
		t.Error("a line carrying the sign is a § and not a no.")
	}
}

func TestASectionInBoldIsToldFromItsOwnFirstNo(t *testing.T) {
	// Page 103 of Topology I to IV carries § 10 and no. 1 of § 10 under the
	// same title, both in bold. The number is what separates them.
	body := []string{
		"**10. PROPER MAPPINGS**",
		"",
		"**1. PROPER MAPPINGS**",
	}
	from, _, head, ok := SectionOpening(body, 10, "Proper mappings")
	if !ok {
		t.Fatal("§ 10 is on this page")
	}
	if from != 0 {
		t.Errorf("rewrote line %d, want the § and not the no. on line 2", from)
	}
	if want := "## 10. PROPER MAPPINGS"; head != want {
		t.Errorf("got %q, want %q", head, want)
	}
}

func TestBoldRoundTheTitleAloneDoesNotSurviveIntoTheHeading(t *testing.T) {
	// Page 137 of Algebre commutative chapitres 8 et 9 has the number outside
	// the bold and the title inside it, where page 103 of Topology I to IV has
	// the whole line in bold. Only the second was written down at first, so the
	// closing pair came off the end, the opening one stayed on the front of the
	// title, and the heading went back as "### 3. **Existence et unicité des
	// $ p $-anneaux" with one half of a pair of asterisks on it.
	body := []string{"3. **Existence et unicité des $ p $-anneaux**"}
	_, _, head, ok := NumberOpening(body, 3, "Existence et unicité des p-anneaux")
	if !ok {
		t.Fatal("the contents opens no. 3 on that page")
	}
	if want := "### 3. Existence et unicité des $ p $-anneaux"; head != want {
		t.Errorf("got %q, want %q", head, want)
	}
	if strings.Count(head, "**")%2 != 0 {
		t.Errorf("the heading carries half a pair of asterisks: %q", head)
	}
}

// Bold inside a title is the title's own and is left where it is. What comes
// off is a pair that wraps the whole of it.
func TestBoldInsideATitleIsLeftAlone(t *testing.T) {
	body := []string{"4. The **plat** condition"}
	_, _, head, ok := NumberOpening(body, 4, "The plat condition")
	if !ok {
		t.Fatal("the contents opens no. 4 on that page")
	}
	if want := "### 4. The **plat** condition"; head != want {
		t.Errorf("got %q, want %q", head, want)
	}
}

func TestADigitReadAsALetterIsStillTheNumber(t *testing.T) {
	// Page 23 and page 113 of Topology I to IV. The reading gave § 1 as "I."
	// and § 11 as "II.", which is the same confusion once and twice. Read as
	// roman numerals the second would be 2 and the heading would be wrong.
	for _, c := range []struct {
		line   string
		number int
		title  string
		want   string
	}{
		{"I. OPEN SETS, NEIGHBOURHOODS, CLOSED SETS", 1, "Open sets, neighbourhoods, closed sets",
			"## 1. OPEN SETS, NEIGHBOURHOODS, CLOSED SETS"},
		{"II. CONNECTEDNESS", 11, "Connectedness", "## 11. CONNECTEDNESS"},
	} {
		_, _, head, ok := SectionOpening([]string{c.line}, c.number, c.title)
		if !ok {
			t.Fatalf("%q: the contents opens § %d on that page", c.line, c.number)
		}
		if head != c.want {
			t.Errorf("%q: got %q, want %q", c.line, head, c.want)
		}
	}
}

func TestTheListOfTheBooksIsNotASectionHeading(t *testing.T) {
	// Page 16 of Topology I to IV sets the Éléments as a numbered list, and
	// those numerals really are roman. The contents opens no § on that page,
	// so the repair never looks at it, and it would refuse the lines anyway
	// because no title agrees.
	body := []string{
		"I. THEORY OF SETS",
		"",
		"II. ALGEBRA",
		"",
		"III. GENERAL TOPOLOGY",
	}
	if _, _, _, ok := SectionOpening(body, 1, "Open sets, neighbourhoods, closed sets"); ok {
		t.Fatal("a line of that list was taken for a § heading")
	}
}

func TestASectionThatIsNotOnItsPageIsRefused(t *testing.T) {
	// Page 54 of Algebra I to III. The reading dropped the § 4 heading
	// altogether and the page opens at the first no. of it.
	body := []string{
		"### 1. GROUPS",
		"",
		"Recall the following definition (§ 2, no. 3, Definition 6).",
	}
	if _, _, _, ok := SectionOpening(body, 4, "Groups and groups with operators"); ok {
		t.Fatal("the heading is not on the page and one was written anyway")
	}
}

func TestAFrenchChapterOpensOnItsOwnWord(t *testing.T) {
	body := []string{"", "Structures algébriques", "", "§ 1. LOIS DE COMPOSITION"}
	out, ok := ChapterOpening(body, "fr", "I", "Structures algébriques")
	if !ok {
		t.Fatal("the page carries the title the contents gives")
	}
	if want := "## CHAPITRE I"; out[1] != want {
		t.Errorf("got %q, want %q", out[1], want)
	}
}

func TestASectionTitleBrokenAtTheMeasureIsOneHeading(t *testing.T) {
	// Page 267 of Topology I to IV. The press broke the heading and the
	// reading kept the break, so the two lines are joined the way a chapter
	// title is joined.
	body := []string{
		"5. INFINITE SUMS",
		"IN COMMUTATIVE GROUPS",
		"",
		"### 1. SUMMABLE FAMILIES IN A COMMUTATIVE GROUP",
	}
	from, to, head, ok := SectionOpening(body, 5, "Infinite sums in commutative groups")
	if !ok {
		t.Fatal("the two lines together are the title the contents gives")
	}
	if from != 0 || to != 1 {
		t.Errorf("rewrote lines %d to %d, want lines 0 to 1", from, to)
	}
	if want := "## 5. INFINITE SUMS IN COMMUTATIVE GROUPS"; head != want {
		t.Errorf("got %q, want %q", head, want)
	}
}

func TestATitleThePageAndTheContentsSpellDifferentlyIsNamed(t *testing.T) {
	// Page 36 of Algebra I to III. The page has one l in the middle word and
	// the contents has two, so the heading is not written and the two
	// spellings are given back for a person to settle.
	body := []string{"§ 2. IDENTITY ELEMENT; CANCELABLE ELEMENTS; INVERTIBLE ELEMENTS"}
	if _, _, _, ok := SectionOpening(body, 2, "Identity element; cancellable elements; invertible elements"); ok {
		t.Fatal("the two titles do not agree and the heading was written anyway")
	}
	got, ok := SectionTitle(body, 2)
	if !ok {
		t.Fatal("the page does head § 2 and the report found nothing")
	}
	if want := "IDENTITY ELEMENT; CANCELABLE ELEMENTS; INVERTIBLE ELEMENTS"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestANumberedHeadingWithNoLevelIsPutBack(t *testing.T) {
	// Page 32 of Theory of Sets. The page sets PROOFS as its running head and
	// "2. PROOFS" in bold under it, and the reading kept one of the two.
	body := []string{
		"**2. PROOFS**",
		"",
		"A *demonstrative text* in a theory $ \\mathscr{T} $ comprises:",
	}
	from, to, head, ok := NumberOpening(body, 2, "Proofs")
	if !ok {
		t.Fatal("the contents opens no. 2 on this page under this title")
	}
	if from != 0 || to != 0 {
		t.Errorf("rewrote lines %d to %d, want line 0 alone", from, to)
	}
	if want := "### 2. PROOFS"; head != want {
		t.Errorf("got %q, want %q", head, want)
	}
}

func TestASectionIsNotReadAsOneOfItsOwnNo(t *testing.T) {
	// Page 10 of Algebra IV to VII prints the sign over the § and over
	// nothing else, so a line that carries one is not a no.
	if _, _, _, ok := NumberOpening([]string{"§ 1. POLYNOMIALS"}, 1, "Polynomials"); ok {
		t.Fatal("a § heading was read as the first no. of itself")
	}
}

func TestANoTitleBrokenAtTheMeasureIsOneHeading(t *testing.T) {
	// Page 379 of Topology I to IV, the second no. of § 8.
	body := []string{
		"2. EXPANSIONS OF REAL NUMBERS RELATIVE",
		"TO A BASE SEQUENCE",
		"",
		"We shall limit ourselves to studying the case where",
	}
	from, to, head, ok := NumberOpening(body, 2, "Expansions of real numbers relative to a base sequence")
	if !ok {
		t.Fatal("the two lines together are the title the contents gives")
	}
	if from != 0 || to != 1 {
		t.Errorf("rewrote lines %d to %d, want lines 0 to 1", from, to)
	}
	if want := "### 2. EXPANSIONS OF REAL NUMBERS RELATIVE TO A BASE SEQUENCE"; head != want {
		t.Errorf("got %q, want %q", head, want)
	}
}

func TestAStarredOrBoldNoIsAlreadyAHeading(t *testing.T) {
	// § 21 no. 13 of Algebra VIII is starred and the reading writes the star
	// with a backslash. None of these is a missing heading.
	for _, line := range []string{
		"### 13. Complex Linear Representations",
		"### \\*13. Complex Linear Representations",
		"### **13. Complex Linear Representations**",
	} {
		if !Numbered([]string{line}, 3, 13) {
			t.Errorf("%q was read as no heading at all", line)
		}
	}
	if Numbered([]string{"### 3. Complex Linear Representations"}, 3, 13) {
		t.Error("no. 3 was taken for no. 13")
	}
	if Numbered([]string{"## 13. Complex Linear Representations"}, 3, 13) {
		t.Error("a § heading was taken for a no. heading")
	}
	if !Numbered([]string{"## § 4. GROUPS AND GROUPS WITH OPERATORS"}, 2, 4) {
		t.Error("a § heading with the sign on it was read as no heading at all")
	}
}

func TestAHeadingFiledAsTheRunningHeadIsPutBack(t *testing.T) {
	// Page 32 of Theory of Sets and page 69 of Algebra I to III. Both set the
	// title of the no. at the head of the page and the heading under it, and
	// both readings kept one line of the two and called it the running head.
	for _, c := range []struct {
		running string
		number  int
		title   string
		head    string
		want    string
	}{
		{"2. PROOFS", 2, "Proofs", "### 2. PROOFS", "PROOFS"},
		{"8. PRODUCTS AND FIBRE PRODUCTS", 8, "Products and fibre products",
			"### 8. PRODUCTS AND FIBRE PRODUCTS", "PRODUCTS AND FIBRE PRODUCTS"},
	} {
		head, running, ok := RunningHeadOpening(c.running, c.number, c.title)
		if !ok {
			t.Fatalf("%q reads as the heading of no. %d", c.running, c.number)
		}
		if head != c.head {
			t.Errorf("got %q, want %q", head, c.head)
		}
		if running != c.want {
			t.Errorf("running head is %q, want %q", running, c.want)
		}
	}
}

func TestARunningHeadWithNoNumberIsNotAHeading(t *testing.T) {
	// Page 91 of Topology I to IV keeps a real running head and lost the
	// heading, and page 35 of Theory of Sets keeps the chapter title. Neither
	// is the line the body is missing.
	for _, c := range []struct {
		running string
		number  int
		title   string
	}{
		{"QUASI-COMPACTS SETS; COMPACT SETS; RELATIVELY COMPACT SETS", 3,
			"Quasi-compact sets; compact sets; relatively compact sets"},
		{"I DESCRIPTION OF FORMAL MATHEMATICS", 1, "Axioms"},
		{"3. INITIAL TOPOLOGIES", 4, "Initial topologies"},
	} {
		if _, _, ok := RunningHeadOpening(c.running, c.number, c.title); ok {
			t.Errorf("%q was written back as the heading of no. %d", c.running, c.number)
		}
	}
}

func TestAnAppendixWhoseWordIsInTheBodyIsMarked(t *testing.T) {
	// Page 635 of Algebra I to III. The word is on the page and the level is
	// gone, which is the same fault SectionOpening repairs one level up.
	body := []string{
		"When $ E = K^n $, we sometimes write $ G_{n,p}(K) $.",
		"",
		"APPENDIX",
	}
	out, drop, ok := AppendixOpening(body, "APPENDIX", "", 0)
	if !ok {
		t.Fatal("the word is on the page and no heading was put back")
	}
	if drop {
		t.Error("the body had the word, so the running head was not the source and stays")
	}
	if want := "## APPENDIX"; out[2] != want {
		t.Errorf("got %q, want %q", out[2], want)
	}
	if len(out) != len(body) {
		t.Errorf("got %d lines, want %d", len(out), len(body))
	}
}

func TestAnAppendixThatIsOnlyARunningHeadIsWrittenIntoTheBody(t *testing.T) {
	// Page 402 of Algebra I to III. The word is the whole of what the page
	// prints over the appendix, so the reading filed it as the running head
	// and the body opens on the title with nothing above it.
	body := []string{"", "PSEUDOMODULES", "", "### 1. ADJUNCTION OF A UNIT ELEMENT"}
	out, drop, ok := AppendixOpening(body, "APPENDIX", "", 0)
	if !ok {
		t.Fatal("the front matter has the word and no heading was put back")
	}
	if !drop {
		t.Error("the running head is spent and should go")
	}
	want := []string{"", "## APPENDIX", "", "PSEUDOMODULES", "", "### 1. ADJUNCTION OF A UNIT ELEMENT"}
	if len(out) != len(want) {
		t.Fatalf("got %d lines, want %d: %q", len(out), len(want), out)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("line %d is %q, want %q", i, out[i], want[i])
		}
	}
}

func TestAFrenchAppendixOpensOnItsOwnWord(t *testing.T) {
	// Page 225 of Topological Vector Spaces in French, and page 94 of
	// Integration IX, which calls the same thing an ANNEXE. A repair that
	// insisted on one word would leave the other volume unassembled.
	for _, c := range []struct{ running, want string }{
		{"APPENDICE", "## APPENDICE"},
		{"ANNEXE", "## ANNEXE"},
	} {
		body := []string{"", "Points fixes", "des groupes de transformations affines"}
		out, drop, ok := AppendixOpening(body, c.running, "Points fixes des groupes de transformations affines", 0)
		if !ok || !drop {
			t.Fatalf("%q was not read as the opening of an appendix", c.running)
		}
		if out[1] != c.want {
			t.Errorf("got %q, want %q", out[1], c.want)
		}
	}
}

func TestANumberedAppendixKeepsTheNumberTheContentsGives(t *testing.T) {
	// Chapter IX of Topology V to X has two appendices and the contents
	// numbers both. The page may set the number as a digit or as a roman
	// numeral and the heading that goes back is written the corpus way.
	for _, had := range []string{"APPENDIX 2", "APPENDIX II", "**APPENDIX II.**"} {
		out, _, ok := AppendixOpening([]string{had, "", "Some text."}, "", "", 2)
		if !ok {
			t.Fatalf("%q is the opening of appendix 2 and was refused", had)
		}
		if want := "## APPENDIX 2"; out[0] != want {
			t.Errorf("%q gave %q, want %q", had, out[0], want)
		}
	}
}

func TestAnAppendixNumberThatDoesNotAgreeIsRefused(t *testing.T) {
	// The number has to be the contents' own in both directions. A chapter
	// with one appendix does not number it, so a number on the page belongs to
	// something else, and a chapter with two will not take the other one.
	for _, c := range []struct {
		line   string
		number int
	}{
		{"APPENDIX 2", 0},
		{"APPENDIX", 1},
		{"APPENDIX 1", 2},
		{"APPENDIX I", 2},
	} {
		if _, _, ok := AppendixOpening([]string{c.line}, "", "", c.number); ok {
			t.Errorf("%q was taken as appendix %d", c.line, c.number)
		}
	}
}

func TestAnAppendixThatIsNotOnThePageIsRefused(t *testing.T) {
	// Page 106 of Lie IX, whose second appendix the contents gives no page at
	// all. Nothing on the page and nothing in the front matter says the word,
	// so there is nothing for the contents to agree with.
	body := []string{"### 1. THE HAAR MEASURE", "", "Let G be a Lie group."}
	if _, _, ok := AppendixOpening(body, "STRUCTURE DES GROUPES", "", 0); ok {
		t.Fatal("a heading was written onto a page that does not carry the word")
	}
}

func TestASentenceThatMentionsAnAppendixIsNotAHeading(t *testing.T) {
	// The word alone is the whole of what a page prints over an appendix, so
	// nothing else is allowed on the line. Otherwise a cross reference in the
	// body would be marked as the opening.
	body := []string{"These results are proved in the Appendix to this chapter.", "", "> see the appendix"}
	if _, _, ok := AppendixOpening(body, "", "", 0); ok {
		t.Fatal("a sentence about an appendix was marked as one")
	}
}

func TestAnAppendixThatIsAlreadyMarkedIsLeftAlone(t *testing.T) {
	// Algebra VIII sets the word, the number and the title of the appendix all
	// on one line, and Integration VII to IX calls the same thing an ANNEX.
	// Both are already headings and neither wants a second one written over
	// it. The last two are the shapes the repair is for and are not marked.
	for _, c := range []struct {
		line   string
		marked bool
	}{
		{"## APPENDIX 1 ALGEBRAS WITHOUT UNIT ELEMENT", true},
		{"## APPENDIX", true},
		{"### Appendice", true},
		{"## ANNEX 2.", true},
		{"APPENDIX", false},
		{"## PSEUDOMODULES", false},
		{"## APPENDED REMARKS", false},
	} {
		if got := Appendix([]string{"", c.line, ""}); got != c.marked {
			t.Errorf("Appendix(%q) is %v, want %v", c.line, got, c.marked)
		}
	}
}

func TestTheTitleUnderAnAppendixWordIsMadeAHeading(t *testing.T) {
	// Page 402 of Algebra I to III. The word is set over a title in its own
	// type, so the reading loses the level on two lines and not one, and the
	// assembler reads the title from under the word.
	body := []string{"", "PSEUDOMODULES", "", "### 1. ADJUNCTION OF A UNIT ELEMENT"}
	out, drop, ok := AppendixOpening(body, "APPENDIX", "Pseudomodules", 0)
	if !ok || !drop {
		t.Fatal("the front matter has the word and no heading was put back")
	}
	want := []string{"", "## APPENDIX", "", "# PSEUDOMODULES", "", "### 1. ADJUNCTION OF A UNIT ELEMENT"}
	if len(out) != len(want) {
		t.Fatalf("got %d lines, want %d: %q", len(out), len(want), out)
	}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("line %d is %q, want %q", i, out[i], want[i])
		}
	}
}

func TestAnAppendixTheContentsGivesNoTitleTakesNothingUnderIt(t *testing.T) {
	// Chapter IX of Algebre commutative chapitres 8 et 9 closes on an appendix
	// the contents gives no title. The page prints the word alone and the next
	// thing on it is the heading of its first no., which is not a title and
	// must not be taken as one.
	body := []string{"APPENDICE", "", "### 1. Limite inductive d'anneaux locaux"}
	out, _, ok := AppendixOpening(body, "", "", 0)
	if !ok {
		t.Fatal("the word is on the page and no heading was put back")
	}
	want := []string{"## APPENDICE", "", "### 1. Limite inductive d'anneaux locaux"}
	for i := range want {
		if out[i] != want[i] {
			t.Errorf("line %d is %q, want %q", i, out[i], want[i])
		}
	}
	if len(out) != len(want) {
		t.Errorf("got %d lines, want %d", len(out), len(want))
	}
}

// The thirteen no. of Algebra I to III that fix opening was refusing. Every one
// of them has its heading on its page and was turned away over the wording, and
// they are here as the four shapes that wording takes.
//
// The heading that goes back is in the page's own words in all four. The
// contents settles the number and the level and nothing else, which is the rule
// the rest of this file works by, so a loose comparison cannot put a word on a
// page that the page did not print.
func TestANoWhosePageWordsItDifferentlyIsStillRepaired(t *testing.T) {
	for _, c := range []struct {
		why      string
		number   int
		contents string
		page     string
	}{
		// Page 245. The contents has an article the page does not.
		{"a dropped article", 13, "Change of the ring of scalars", "CHANGE OF RING OF SCALARS"},
		// Page 330. The page sets the first word plural.
		{"a plural", 7, "Tensor product of vector spaces", "TENSOR PRODUCTS OF VECTOR SPACES"},
		// Page 453. The contents entry came off the contents page with the l of
		// subalgebras read as an i.
		{"a misread letter", 2, "Subaigebras. Ideals. Quotient algebras", "SUBALGEBRAS. IDEALS. QUOTIENT ALGEBRAS"},
		// Page 521. The page names the no. at more length than the contents.
		{"a longer heading", 1, "Symmetric algebra of a module", "DEFINITION OF THE SYMMETRIC ALGEBRA OF A MODULE"},
	} {
		body := []string{strconv.Itoa(c.number) + ". " + c.page}
		_, _, head, ok := NumberOpening(body, c.number, c.contents)
		if !ok {
			t.Errorf("%s: the heading is on the page and was refused", c.why)
			continue
		}
		if !strings.Contains(head, c.page) {
			t.Errorf("%s: the heading is %q, which is not what the page printed", c.why, head)
		}
	}
}

// The loose comparison is only loose about wording, and these are the two ways
// it has to stay strict.
//
// A different title under the same number is still refused, because that is the
// one case where a wrong match writes a wrong heading. And the § keeps the exact
// rule it had, because a § the page and the contents spell differently is
// reported by name for a person to settle and that report is worth more than the
// repair.
func TestTheLooseComparisonStaysStrictAboutTheThingsItHasTo(t *testing.T) {
	body := []string{"2. QUOTIENT GROUPS AND QUOTIENT RINGS"}
	if _, _, _, ok := NumberOpening(body, 2, "Tensor product of vector spaces"); ok {
		t.Error("a no. numbered the same but titled something else was taken for a match")
	}
	// Page 36 of Algebra I to III, the one letter the § test above pins.
	section := []string{"§ 2. IDENTITY ELEMENT; CANCELABLE ELEMENTS; INVERTIBLE ELEMENTS"}
	if _, _, _, ok := SectionOpening(section, 2, "Identity element; cancellable elements; invertible elements"); ok {
		t.Error("the § comparison went loose, and the differ report it feeds is gone")
	}
}
