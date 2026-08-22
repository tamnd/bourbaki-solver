package ocr

import (
	"errors"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/prompt"
)

// goodPage is what an accepted page of alg-iv-vii looks like: a head label, a
// statement, display mathematics, a cross-reference. Long enough to clear rule
// 1 and shaped like the real thing.
const goodPage = `A IV.7  POLYNOMIALS AND RATIONAL FRACTIONS  § 1

**Proposition 4.** — Let $A$ be a commutative ring and $B$ an $A$-algebra. For
every family $(b_\lambda)_{\lambda \in L}$ of elements of $B$ there exists a
unique homomorphism $\varphi$ of $A[(X_\lambda)_{\lambda \in L}]$ into $B$ such
that $\varphi(X_\lambda) = b_\lambda$ for all $\lambda \in L$.

$$\varphi\left(\sum_{\nu} a_\nu X^\nu\right) = \sum_{\nu} a_\nu b^\nu.$$

The image of $\mathbf{Z}$ under $\varphi$ is the prime subring of $B$
(I, p. 23, Proposition 4). This applies in particular when $A = \mathbf{Q}$.

☡ The homomorphism $\varphi$ is not in general injective.`

func alg4(page int) Expect {
	return Expect{
		Book: "alg-iv-vii", PDFPage: page + 10, Grammar: pagemap.HeadLabel,
		Chapter: "IV", Page: page, Confidence: pagemap.FromHead, HasHead: true,
	}
}

func TestARealPageIsAccepted(t *testing.T) {
	if problems := Validate(goodPage, alg4(7), Options{}); len(problems) != 0 {
		t.Fatalf("a good page was rejected: %s", Reasons(problems))
	}
	if !OK(goodPage, alg4(7), Options{}) {
		t.Fatal("OK disagrees with Validate")
	}
}

func TestRule1ShortPages(t *testing.T) {
	short := "A IV.7  POLYNOMIALS AND RATIONAL FRACTIONS  § 1\n\nThe rest of this page is blank."
	problems := Validate(short, alg4(7), Options{})
	if !has(problems, RuleShort) {
		t.Fatalf("a short page was accepted: %s", Reasons(problems))
	}
	// A blank page is short on purpose and must not be rejected for it.
	expect := alg4(7)
	expect.Blank, expect.HasHead = true, false
	if has(Validate("", expect, Options{}), RuleShort) {
		t.Error("a blank page was rejected for being short")
	}
	// Nor is a page that is short because the book is short there. alg-iv-vii
	// page 3 is the Springer knight and nothing else, and alg-viii pages 1 to 3
	// are title pages of under fifty characters. Rejecting those costs three
	// calls each and then files a title page as a defect.
	expect = alg4(7)
	expect.Sparse, expect.HasHead, expect.Confidence = true, false, pagemap.Unknown
	if has(Validate("Algebra\n\nChapters 4 to 7", expect, Options{}), RuleShort) {
		t.Error("a title page was rejected for being short")
	}
	// A sparse page is still held to the other rules. A refusal on one is a
	// refusal.
	if !has(Validate("I'm sorry, I can't help with that.", expect, Options{}), RuleLeak) {
		t.Error("a refusal on a sparse page was accepted")
	}
}

func TestRule2MathDelimiters(t *testing.T) {
	cases := []struct {
		name string
		body string
		bad  bool
	}{
		{"balanced inline", `Let $G$ be a group and $H \subset G$.`, false},
		{"one dollar left open", `Let $G$ be a group and $H \subset G.`, true},
		{"balanced display", `The sum is $$\sum_{i} a_i = 0.$$`, false},
		{"one display left open", `The sum is $$\sum_{i} a_i = 0.`, true},
		{"display and inline together", `For $x$ we have $$x^2 = 1$$ and $y$ follows.`, false},
		// A naive count of dollars calls this unbalanced, because $$ is two of
		// them. Every display page in both volumes looks like this.
		{"display only, no inline", `$$a + b = c$$ and $$d + e = f$$`, false},
		// Bourbaki's historical notes quote prices, and a literal dollar is
		// escaped. Counting it would reject the page.
		{"escaped dollar", `The prize was \$100 and the ring $\mathbf{Z}$ is free.`, false},
		{"two escaped dollars", `From \$5 to \$10, with $x$ fixed.`, false},
	}
	for _, test := range cases {
		body := "A IV.7  POLYNOMIALS  § 1\n\n" + test.body + strings.Repeat(" filler text to clear the length rule.", 8)
		problems := Validate(body, alg4(7), Options{})
		if got := has(problems, RuleMath); got != test.bad {
			t.Errorf("%s: math rule fired %t, want %t: %s", test.name, got, test.bad, Reasons(problems))
		}
	}
}

// The page that made rule 2 count paragraphs. This is page 53 of Algebra I as
// the fleet returned it, cut to the two paragraphs that matter. Each one ends a
// formula without closing it, and five plus five is ten, so parity over the
// page said the mathematics was balanced and the page went into the corpus with
// no flag on it.
const page53 = `DISTRIBUTIVITY OF ONE INTERNAL LAW WITH RESPECT TO ANOTHER

for every ordered sequence $(\alpha_\lambda)_{\lambda\in L}$ of elements of $\Omega$ and all $x\in E.

(4) In $\mathbf N$ addition and multiplication are distributive with respect to the laws $\sup$ and $\inf.
`

func TestAnEvenNumberOfDollarsIsNotABalancedPage(t *testing.T) {
	problems := Validate(page53, alg4(7), Options{})
	if !has(problems, RuleMath) {
		t.Fatalf("page 53 was accepted with two unclosed formulae: %s", Reasons(problems))
	}
	// The line is the point of doing it this way. Parity can only say the page
	// is wrong; the repair pass needs to know where.
	var line int
	for _, problem := range problems {
		if problem.Rule == RuleMath {
			line = problem.Line
		}
	}
	if line != 3 {
		t.Errorf("the unclosed dollar was reported on line %d, want 3: %s", line, Reasons(problems))
	}
}

func TestRule2ReadsParagraphByParagraph(t *testing.T) {
	head := "A IV.7  POLYNOMIALS  § 1\n\n"
	filler := strings.Repeat(" filler text to clear the length rule.", 8)
	cases := []struct {
		name string
		body string
		bad  bool
	}{
		// An inline formula wrapped across a line is ordinary and stays legal.
		// Only a blank line ends the run.
		{"inline across a line break", "Let $G\nbe$ a group." + filler, false},
		{"unclosed then closed in the next paragraph", "Let $G be a group.\n\nand $H$ too." + filler, true},
		// A display block with a blank line inside it is legal LaTeX and the
		// dollars inside it are not inline delimiters. The rule stands down for
		// as long as the block is open, or every aligned equation on a page
		// costs three calls at 151 seconds and lands in the report as a defect.
		{"display block spanning a blank line", "$$\n\\begin{aligned}\na &= b\n\n&= c\n\\end{aligned}\n$$" + filler, false},
		{"a run left open at the end of the page", "Let $G$ be a group and $H \\subset G." + filler, true},
	}
	for _, test := range cases {
		problems := Validate(head+test.body, alg4(7), Options{})
		if got := has(problems, RuleMath); got != test.bad {
			t.Errorf("%s: math rule fired %t, want %t: %s", test.name, got, test.bad, Reasons(problems))
		}
	}
}

func TestRule3Leaks(t *testing.T) {
	text := "Here is the transcription of the image:\n\n" + goodPage
	problems := Validate(text, alg4(7), Options{})
	if !has(problems, RuleLeak) {
		t.Fatalf("narration was accepted: %s", Reasons(problems))
	}
	// A leak is reported on its own. A refusal is short and headless and has no
	// mathematics, and listing those as three more problems would say the page
	// failed four rules when it failed one.
	refusal := Validate("I'm sorry, I can't transcribe this image.", alg4(7), Options{})
	if len(Rules(refusal)) != 1 || Rules(refusal)[0] != RuleLeak {
		t.Fatalf("a refusal reported as %v, want leak alone", Rules(refusal))
	}
}

func TestRule4RunningHead(t *testing.T) {
	// A volume that prints a page label has to show one.
	noHead := "**Proposition 4.** — Let $A$ be a commutative ring." + strings.Repeat(" More text follows here.", 12)
	if !has(Validate(noHead, alg4(7), Options{}), RuleHead) {
		t.Error("a page with no running head was accepted for a head-label volume")
	}
	// Pages that print no head at all are not asked for one. A chapter opener
	// is the common case, and rejecting those would fail one page per chapter.
	expect := alg4(7)
	expect.HasHead, expect.Confidence = false, pagemap.Unknown
	if has(Validate(noHead, expect, Options{}), RuleHead) {
		t.Error("a chapter opener was asked for a running head")
	}
}

// TestRule4ParagraphThatCitesAPage is the branch that let the rule down.
//
// ParsePageLabel searches the line it is given, which is what its other callers
// want, so a paragraph carrying a citation answered yes to "is there a page
// label in the first line" and the rule accepted the page. 9 of the 200 pages of
// golden-dev open with a paragraph shaped like this, the longest 1425 runes,
// against a longest genuinely printed head on the same set of 64.
func TestRule4ParagraphThatCitesAPage(t *testing.T) {
	opening := "still hold for generalized formal power series, by the argument of A VIII.202, " +
		"and the corollary above applies to each of them without change in this case."
	body := opening + "\n\n**Theorem 1.** — Every group is isomorphic to a group of permutations." +
		strings.Repeat(" The proof follows from the preceding remarks.", 6)
	if !has(Validate(body, alg4(7), Options{}), RuleHead) {
		t.Error("a paragraph that cites a page was accepted as a running head")
	}
}

// TestRule4DisplayOpener is the other letterless line.
//
// looksLikeHead reads a line with no letters in it as a bare folio, and a reader
// that opens the page on a display writes \[ and nothing else.
func TestRule4DisplayOpener(t *testing.T) {
	body := "\\[\n\n**Theorem 1.** — Every group is isomorphic to a group of permutations." +
		strings.Repeat(" The proof follows from the preceding remarks.", 6)
	expect := Expect{
		Book: "alg-i-iii", Grammar: pagemap.FootNumber, Chapter: "I",
		Page: 24, Confidence: pagemap.FromFoot, HasHead: true,
	}
	if !has(Validate(body, expect, Options{}), RuleHead) {
		t.Error("a display opener was accepted as a running head")
	}
}

func TestRule4ForTheVolumeThatPrintsItsNumberAtTheFoot(t *testing.T) {
	// alg-i-iii prints no page label. The head carries the chapter title in
	// capitals on one side and the section locator on the other, so the rule
	// can only ask that the line is a head and not the opening of a paragraph.
	foot := func(head string) []Problem {
		body := head + "\n\n**Theorem 1.** — Every group is isomorphic to a group of permutations." +
			strings.Repeat(" The proof follows from the preceding remarks.", 6)
		return Validate(body, Expect{
			Book: "alg-i-iii", Grammar: pagemap.FootNumber, Chapter: "I",
			Page: 24, Confidence: pagemap.FromFoot, HasHead: true,
		}, Options{})
	}
	for _, head := range []string{
		"ALGEBRAIC STRUCTURES",
		"§ 4.5",
		"§ 4",
		"24",
		"MONOIDS, GROUPS",
	} {
		if problems := foot(head); has(problems, RuleHead) {
			t.Errorf("a real running head was rejected: %q: %s", head, Reasons(problems))
		}
	}
	for _, head := range []string{
		"Let $G$ be a group whose law of composition is written multiplicatively.",
		"In this section we develop the theory of groups acting on sets, following.",
	} {
		if problems := foot(head); !has(problems, RuleHead) {
			t.Errorf("a line of prose passed as a running head: %q", head)
		}
	}
}

// TestRule4TheVolumeThatPrintsAFullStopInItsHead is the second branch that let
// the rule down, and the more expensive of the two.
//
// The rule used to veto any first line ending in a full stop, on the reading
// that a sentence ends in one and a head does not. hist prints its heads with
// the stop, so the veto rejected the head the page really carries, three times
// each, and pages of that volume went dead on it. Across the 4809 raw readings
// on disk there are 112 first lines of 90 characters or fewer that end in a
// stop. 97 read as a head under the rule as it stands now, over 60 distinct
// pages, 56 of those in hist. The other 15 are mixed case sentences.
func TestRule4TheVolumeThatPrintsAFullStopInItsHead(t *testing.T) {
	head := func(first string) []Problem {
		body := first + "\n\n**Theorem 1.** — Every group is isomorphic to a group of permutations." +
			strings.Repeat(" The proof follows from the preceding remarks.", 6)
		return Validate(body, Expect{
			Book: "hist", Grammar: pagemap.HeadNumber, Chapter: "I",
			Page: 234, Confidence: pagemap.FromHead, HasHead: true,
		}, Options{})
	}
	for _, first := range []string{
		"234  23. HAAR MEASURE. CONVOLUTION.",
		"17. INFINITESIMAL CALCULUS.",
		"148  12. REAL NUMBERS.",
		"PREFACE.",
		"TABLE OF CONTENTS.",
	} {
		if problems := head(first); has(problems, RuleHead) {
			t.Errorf("a printed running head was rejected: %q: %s", first, Reasons(problems))
		}
	}
	// The capitals test is what keeps prose out, and it still does. Both of
	// these end in a full stop too, and neither is set in capitals.
	for _, first := range []string{
		"The theory of Haar measure is developed in the following section.",
		"We now turn to the convolution of two measures on a group.",
	} {
		if problems := head(first); !has(problems, RuleHead) {
			t.Errorf("a line of prose passed as a running head: %q", first)
		}
	}
}

func TestRule5Illegible(t *testing.T) {
	body := func(n int) string {
		return "A IV.7  POLYNOMIALS  § 1\n\nThe element " +
			strings.Repeat("x "+Illegible+" is not readable here. ", n) +
			strings.Repeat("Ordinary text of the page continues. ", 8)
	}
	// A damaged scan with one or two bad spots is still the best reading of
	// that page, and asking for it again gets the same answer.
	for _, n := range []int{0, 1, 2} {
		if has(Validate(body(n), alg4(7), Options{}), RuleIllegible) {
			t.Errorf("%d unreadable spots was rejected", n)
		}
	}
	for _, n := range []int{3, 6} {
		if !has(Validate(body(n), alg4(7), Options{}), RuleIllegible) {
			t.Errorf("%d unreadable spots was accepted", n)
		}
	}
}

func TestRule6PageLabelAgainstThePageMap(t *testing.T) {
	page := func(label string) []Problem {
		return Validate(strings.Replace(goodPage, "A IV.7", label, 1), alg4(7), Options{})
	}
	// One page of slack: a verso head and the facing recto differ by one, and
	// these scans are known to be off by a page in places.
	for _, label := range []string{"A IV.6", "A IV.7", "A IV.8"} {
		if problems := page(label); has(problems, RuleLabel) {
			t.Errorf("%s was rejected against a page map that says A IV.7: %s", label, Reasons(problems))
		}
	}
	for _, label := range []string{"A IV.9", "A IV.70", "A IV.5"} {
		if !has(page(label), RuleLabel) {
			t.Errorf("%s was accepted against a page map that says A IV.7", label)
		}
	}
	// The wrong chapter is always wrong, however close the number.
	if !has(page("A V.7"), RuleLabel) {
		t.Error("a page from the wrong chapter was accepted")
	}
}

func TestRule6DoesNotRunOnAGuess(t *testing.T) {
	// The page map interpolates a number where the page did not print a legible
	// one. Rejecting a page because a guess disagrees with it would throw away
	// the only reading that was actually made.
	expect := alg4(7)
	expect.Confidence = pagemap.Interpolated
	body := strings.Replace(goodPage, "A IV.7", "A IV.40", 1)
	if has(Validate(body, expect, Options{}), RuleLabel) {
		t.Error("an interpolated page number rejected a page")
	}
	// Nor where the volume prints no label at all.
	expect = Expect{Book: "alg-i-iii", Grammar: pagemap.FootNumber, Page: 3, Confidence: pagemap.FromFoot, HasHead: true}
	if has(Validate(body, expect, Options{}), RuleLabel) {
		t.Error("the label rule ran on a volume that prints no label")
	}
}

type texFails struct{ err error }

func (t texFails) Check(string) error { return t.err }

func TestRule7IsOptIn(t *testing.T) {
	if has(Validate(goodPage, alg4(7), Options{}), RuleLaTeX) {
		t.Fatal("the LaTeX rule ran without being asked for")
	}
	problems := Validate(goodPage, alg4(7), Options{LaTeX: texFails{errors.New("undefined control sequence \\varphi")}})
	if !has(problems, RuleLaTeX) {
		t.Fatalf("the LaTeX rule did not fire: %s", Reasons(problems))
	}
	if !strings.Contains(Reasons(problems), "undefined control sequence") {
		t.Fatalf("the compiler's message was lost: %s", Reasons(problems))
	}
	if has(Validate(goodPage, alg4(7), Options{LaTeX: texFails{nil}}), RuleLaTeX) {
		t.Error("the LaTeX rule fired on a fragment that compiled")
	}
}

func TestEveryFailingRuleIsReportedNotJustTheFirst(t *testing.T) {
	// Short, unbalanced, and no running head. The retry differs depending on
	// which of those it is, so all three have to come back.
	body := "Let $G$ be a group and let $H \\subset G."
	problems := Validate(body, alg4(7), Options{})
	rules := Rules(problems)
	for _, want := range []Rule{RuleShort, RuleMath, RuleHead} {
		if !has(problems, want) {
			t.Errorf("rule %s was not reported: %v", want, rules)
		}
	}
}

func TestReasonsReadsAsOneLine(t *testing.T) {
	if Reasons(nil) != "" {
		t.Error("no problems should read as nothing")
	}
	got := Reasons(Validate("Let $G$ be a group.", alg4(7), Options{}))
	if !strings.Contains(got, "short:") || !strings.Contains(got, "head:") {
		t.Fatalf("the reason line does not name its rules: %s", got)
	}
	if strings.Contains(got, "\n") {
		t.Fatalf("the reason line has a newline in it: %q", got)
	}
}

func has(problems []Problem, rule Rule) bool {
	for _, problem := range problems {
		if problem.Rule == rule {
			return true
		}
	}
	return false
}

// ens returns what the page map knows about a page of Theory of Sets, which
// prints its number at the foot and carries a chapter name up top.
func ens(page int) Expect {
	return Expect{
		Book: "ens-i-iv", PDFPage: page + 5, Grammar: pagemap.FootNumber,
		Chapter: "IV", Page: page, Confidence: pagemap.FromFoot, HasHead: false,
	}
}

// Page 289 of Theory of Sets prints EXERCISES, then § 1, then its first
// exercise as an ordinary paragraph, and the reading came back with that
// paragraph as a heading. Nothing of a § comes after its exercises, so the
// heading is the reading and not the page. See checkAfterExercises.
func TestRule8AnExerciseSetAsAHeading(t *testing.T) {
	page := `# EXERCISES

## § 1

### 1. Let $S$ be the set of signs $P$, $X$, $x_1, \ldots, x_n$, the letters
$x_i$ being of weight 0, $P$ of weight 1, and $X$ of weight 2. Such a word will
be called an *echelon type* on $x_1, \ldots, x_n$.

Let $E_1, \ldots, E_n$ be $n$ terms in a theory stronger than the theory of
sets. For each echelon type $T$ define a term $T(E_1, \ldots, E_n)$ as follows.`
	problems := Validate(page, ens(289), Options{})
	if !has(problems, RuleExercise) {
		t.Fatalf("an exercise set as a heading was accepted: %s", Reasons(problems))
	}
	for _, problem := range problems {
		if problem.Rule == RuleExercise && problem.Line != 5 {
			t.Errorf("the heading is on line 5, the rule says line %d", problem.Line)
		}
	}
}

// The same page read as the volume prints it, which is what page 290 came back
// as. The exercise is prose and the § head above it is a head, because a chapter
// gathers the exercises of all its sections under one EXERCISES and divides them
// by §.
func TestRule8ASectionHeadBelowTheExercisesHeadIsThePrinting(t *testing.T) {
	page := `# EXERCISES

## § 1

1. Let $S$ be the set of signs $P$, $X$, $x_1, \ldots, x_n$, the letters $x_i$
being of weight 0, $P$ of weight 1, and $X$ of weight 2. Such a word will be
called an *echelon type* on $x_1, \ldots, x_n$.

2. Let $E_1, \ldots, E_n$ be $n$ terms in a theory stronger than the theory of
sets. For each echelon type $T$ define a term $T(E_1, \ldots, E_n)$ as follows.`
	if problems := Validate(page, ens(289), Options{}); has(problems, RuleExercise) {
		t.Fatalf("the printing was rejected: %s", Reasons(problems))
	}
}

// A no. heading is how the body of a § is written, and above the exercises head
// there is nothing wrong with it.
func TestRule8ANoHeadingAboveTheExercisesHeadStands(t *testing.T) {
	page := `### 4. Echelon types

Let $S$ be the set of signs $P$, $X$, $x_1, \ldots, x_n$, the letters $x_i$
being of weight 0, $P$ of weight 1, and $X$ of weight 2. Such a word will be
called an *echelon type* on $x_1, \ldots, x_n$.

# EXERCISES

1. Show that the relation given above holds for every echelon type on the
letters $x_1, \ldots, x_n$, and that no other relation holds for all of them.`
	if problems := Validate(page, ens(289), Options{}); has(problems, RuleExercise) {
		t.Fatalf("a no. heading above the exercises head was rejected: %s", Reasons(problems))
	}
}

// top returns what the page map knows about a page of Topology I to IV, which
// prints its number at the foot like Theory of Sets does.
func top(page int) Expect {
	return Expect{
		Book: "top-i-iv", PDFPage: page + 6, Grammar: pagemap.FootNumber,
		Chapter: "I", Page: page, Confidence: pagemap.FromFoot, HasHead: false,
	}
}

// Page 44 of Topology I to IV, as the local reader returned it. Every word is
// right and the mathematics balances. The printing sets PROPOSITION 1 and
// COROLLARY 1 in small capitals with the statements in italic, and neither the
// case nor the emphasis survived, so the assembler reads two paragraphs of prose
// where the page has two statements. See checkStatementHead.
func TestRule9AStatementHeadInPlainMixedCase(t *testing.T) {
	page := `If $I$ is a finite set, the construction of the product topology from the
topologies of the factors $X_\iota$ is simpler: the elementary sets are just
products $\prod_{\iota \in I} A_\iota$, where $A_\iota$ is any open subset of
$X_\iota$, for each $\iota \in I$ (cf. Exercise 9).

Proposition 1. Let $f = (f_\iota)$ be a mapping of a topological space $Y$ into
a product space $X = \prod_{\iota \in I} X_\iota$. Then $f$ is continuous at a
point $a \in Y$ if and only if $f_\iota$ is continuous at $a$ for each $\iota$.

Since $f_\iota = \mathrm{pr}_\iota \circ f$, this is just a particular case of
Proposition 4 of § 2, no. 3.`
	problems := Validate(page, top(44), Options{})
	if !has(problems, RuleStatement) {
		t.Fatalf("a lost statement head was accepted: %s", Reasons(problems))
	}
	for _, problem := range problems {
		if problem.Rule == RuleStatement && problem.Line != 6 {
			t.Errorf("the head is on line 6, the rule says line %d", problem.Line)
		}
	}
}

// The three shapes a printing of the corpus actually sets a statement head in,
// none of which the rule may touch.
func TestRule9TheHeadsThePrintingsSetAreAccepted(t *testing.T) {
	for _, head := range []string{
		"PROPOSITION 1. *Let $f$ be a mapping of a topological space $Y$ into $X$.*",
		"**Proposition 1.** *Let $f$ be a mapping of a topological space $Y$ into $X$.*",
		"PROPOSITION 12. — Soit $A$ un anneau et $M$ un $A$-module de type fini.",
		"Proposition 12. — Soit $A$ un anneau et $M$ un $A$-module de type fini.",
	} {
		page := head + `

Since $f_\iota = \mathrm{pr}_\iota \circ f$, this is just a particular case of
Proposition 4 of § 2, no. 3, and the proof is the one given there for the
product topology on a finite family of topological spaces $X_\iota$.`
		if problems := Validate(page, top(44), Options{}); has(problems, RuleStatement) {
			t.Errorf("%q was rejected: %s", head, Reasons(problems))
		}
	}
}

// The kinds a printing does set plain, which is why they are not in the rule.
// Lie 7 to 9 prints "Lemma 1." unemphasised 101 times and Topology I to IV opens
// a run with "Examples. 1)", and the assembler reads both as they stand.
func TestRule9TheKindsSetPlainOnPurposeStand(t *testing.T) {
	for _, head := range []string{
		"Lemma 1. Every infinite set contains a countable subset, and the proof of",
		"Remark 2. The topology induced on a subspace need not be discrete, since",
		"Examples. 1) In a discrete space the set $\\{x\\}$ alone constitutes a",
		"Scholium. The argument above uses the axiom of choice only through Zorn's",
	} {
		page := head + `

fundamental system of neighbourhoods of the point $x$, and the same argument
applies to any set which is open in the product topology on $\prod X_\iota$
for each index $\iota$ of the finite set $I$ under consideration here.`
		if problems := Validate(page, top(44), Options{}); has(problems, RuleStatement) {
			t.Errorf("%q was rejected: %s", head, Reasons(problems))
		}
	}
}

// Rule 3 against the prompt the page was read with, which is the half of the
// rule that pages 3 and 9 of Algebra IV to VII walked through.
func TestRuleThreeCatchesThePromptHandedBack(t *testing.T) {
	ask := prompt.OCR()
	var body strings.Builder
	body.WriteString("A IV.7 POLYNOMIALS AND RATIONAL FRACTIONS\n\n")
	// The long house rules and not the short bullets above them, because that
	// is the shape the failure has on disk. The model handed back the middle of
	// the prompt and left the top of it out, which is exactly why the phrase
	// list missed it: every phrase in that list is off the first eight lines.
	for _, line := range strings.Split(ask, "\n") {
		if strings.HasPrefix(line, "- ") && len([]rune(line)) >= 120 {
			body.WriteString(line + "\n")
		}
	}
	expect := alg4(7)

	problems := Validate(body.String(), expect, Options{Prompt: ask})
	if len(problems) == 0 {
		t.Fatal("the prompt handed back was accepted")
	}
	for _, problem := range problems {
		if problem.Rule != RuleLeak {
			t.Errorf("rule = %q, want leak", problem.Rule)
		}
	}

	// And without the prompt in hand it is what it was before: long, prose, no
	// unbalanced mathematics, a first line that reads like a running head.
	if problems := Validate(body.String(), expect, Options{}); len(problems) > 0 {
		t.Fatalf("without the prompt there is nothing to compare against, got %v", problems)
	}
}

func TestAPageIsNotItsPrompt(t *testing.T) {
	page := "A IV.7 POLYNOMIALS AND RATIONAL FRACTIONS\n\n" +
		"**Proposition 4.** — Let $A$ be a commutative ring and let $X$ be an indeterminate. " +
		"Every polynomial of $A[X]$ whose leading coefficient is invertible is regular, and the " +
		"quotient of the division is unique. The proof rests on the induction of no. 3 and on the " +
		"remark that the degree of a product is the sum of the degrees when one of the two leading " +
		"coefficients is not a divisor of zero.\n"
	expect := alg4(7)
	if problems := Validate(page, expect, Options{Prompt: prompt.OCR()}); len(problems) > 0 {
		t.Fatalf("a page of the book was rejected against its own prompt: %v", problems)
	}
}
