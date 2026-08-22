package toc

import "testing"

// Every line in these tests is off a page of the corpus, and every title is
// what manifests/toc.yaml gives for the chapter or the § it belongs to.

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
