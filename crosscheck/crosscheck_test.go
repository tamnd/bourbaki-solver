package crosscheck

import (
	"strings"
	"testing"
)

// Every fixture here is real. The pages are from Algebra VIII, the second
// reading is what pdftotext -layout prints for that page, and where a fixture
// shows the extractor getting something wrong it is what the extractor really
// shipped, taken out of the corpus at the commit before the fix.

// Page 114 as pdftotext prints the paragraph the accents landed on, with the
// two paragraphs below it that spell the same words right.
const theirs114 = `Because of the assumptions, P is faithful as an A-module and as a B-module.
                                                      e ◦ σ = 1P∗ and σ ◦ σ
From relations (15) and (16), respectively, we deduce σ                    e=
1Pe . Relations (13) and (14) then follow from (12) and (9), respectively.

Remarks. — 1) Suppose that the mapping b 7→ bP from B to EndA (P) is
bijective. Then
     a) the B-module P can be identified with the countermodule of P; it is
therefore faithful and balanced;
     b) the mapping a 7→ aP from A to EndB (P) is bijective if and only if the
A-module P is faithful and balanced.
     2) Under the assumptions of Proposition 1, the A-module P is balanced;
                         0`

// The same passage as the extractor shipped it before the accents were fixed.
// Each accent is drawn over the letters of the line below rather than the ones
// it belongs to, so the page reads "the assumpti~ons" and "f~aithful".
const ours114Broken = `Because o$\widetilde{f}$ the assumpti$\widetilde{o}$ns, P is f$\widetilde{ai}$thful as a$\widetilde{n}$ $\widetilde{A}$-module and a$\widetilde{s}$ $\widetilde{a}$ B-module. From relations (15) and (16), respectively, we deduce $\widetilde{\sigma}\circ \sigma = 1_{P^*}$ and $\sigma \circ \widetilde{\sigma}=$ $1_{\widetilde{P}}$. Relations (13) and (14) then follow from (12) and (9), respectively.

**Remarks.** — 1) Suppose that the mapping $b\mapsto b_P$ from B to End$_A(P)$ is bijective. Then

a) the B-module P can be identified with the countermodule of P; it is therefore faithful and balanced;

b) the mapping $a\mapsto a_P$ from A to End$_B(P)$ is bijective if and only if the A-module P is faithful and balanced.

2) Under the assumptions of Proposition 1, the A-module P is balanced;`

// And as it reads now.
const ours114Fixed = `Because of the assumptions, P is faithful as an A-module and as a B-module. From relations (15) and (16), respectively, we deduce $\widetilde{\sigma}\circ \sigma = 1_{P^*}$ and $\sigma \circ \widetilde{\sigma}=$ $1_{\widetilde{P}}$. Relations (13) and (14) then follow from (12) and (9), respectively.

**Remarks.** — 1) Suppose that the mapping $b\mapsto b_P$ from B to End$_A(P)$ is bijective. Then

a) the B-module P can be identified with the countermodule of P; it is therefore faithful and balanced;

b) the mapping $a\mapsto a_P$ from A to End$_B(P)$ is bijective if and only if the A-module P is faithful and balanced.

2) Under the assumptions of Proposition 1, the A-module P is balanced;`

func TestAnAccentDrawnOverTheWrongLettersIsFound(t *testing.T) {
	// The page has "assumptions" and "faithful" further down, spelled right, so
	// asking which of pdftotext's words are missing finds nothing at all. This
	// is the whole reason the other direction exists.
	if lost := Page(ours114Broken, theirs114); len(lost) != 0 {
		t.Errorf("Page() = %v, want nothing: the right spellings are elsewhere on the page", lost)
	}
	got := map[string]bool{}
	for _, l := range Extra(ours114Broken, theirs114) {
		if !l.Ours {
			t.Errorf("Extra() returned %q as pdftotext's, want ours", l.Word)
		}
		got[l.Word] = true
	}
	for _, w := range []string{"assumpti", "aithful"} {
		if !got[w] {
			t.Errorf("Extra() did not report %q, got %v", w, got)
		}
	}
}

func TestThePageIsQuietOnceTheAccentsAreRight(t *testing.T) {
	if lost := Extra(ours114Fixed, theirs114); len(lost) != 0 {
		t.Errorf("Extra() = %v, want nothing", lost)
	}
}

// TestAWordTheAccentBrokeIsNotExcusedAsGlue is the reason glued asks pdftotext
// about the tail as well as asking ourselves. "faithful" less its first three
// characters is "thful", which the broken line leaves standing on the page, so
// without that second question the rule excuses the defect it sits beside.
func TestAWordTheAccentBrokeIsNotExcusedAsGlue(t *testing.T) {
	const theirs = `Because of the assumptions, P is faithful as an A-module and as a B-module.`
	const ours = `Because o$\widetilde{f}$ the assumpti$\widetilde{o}$ns, P is f$\widetilde{ai}$thful as a$\widetilde{n}$ $\widetilde{A}$-module and a$\widetilde{s}$ $\widetilde{a}$ B-module.`
	got := map[string]bool{}
	for _, l := range Page(ours, theirs) {
		got[l.Word] = true
	}
	for _, w := range []string{"faithful", "assumptions"} {
		if !got[w] {
			t.Errorf("Page() did not report %q, got %v", w, got)
		}
	}
}

// Page 14, the table of contents, as it shipped before a bold heading was read
// as prose rather than as a symbol. Every letter of the entry went inside
// dollar signs on its own, and nothing downstream could tell.
func TestAHeadingSetOneLetterAtATimeIsFound(t *testing.T) {
	const theirs = `Appendix 1. Algebras without Unit Element . . . . . . . . . . . . . . . . .                                                      435`
	const ours = `$\mathbf{A}\mathbf{p}\mathbf{p}\mathbf{e}\mathbf{n}\mathbf{d}\mathbf{i}\mathbf{x} 1. \mathbf{A}\mathbf{l}\mathbf{g}\mathbf{e}\mathbf{b}\mathbf{r}\mathbf{a}\mathbf{s} \mathbf{w}\mathbf{i}\mathbf{t}\mathbf{h}\mathbf{o}\mathbf{u}\mathbf{t} \mathbf{U}\mathbf{n}\mathbf{i}\mathbf{t} \mathbf{E}\mathbf{l}\mathbf{e}\mathbf{m}\mathbf{e}\mathbf{n}\mathbf{t}$. . . . . . . . . . . . . . . . . 435`
	lost := Page(ours, theirs)
	if len(lost) != 1 || lost[0].Word != "without" {
		t.Errorf("Page() = %v, want the one word of the entry that is lower case", lost)
	}
}

// Page 120. The volume sets "k-algebras" with the ring in mathematics and the
// typesetter breaks the line at that hyphen, so putting pdftotext's two halves
// back together leaves "kalgebras" and no "algebras" anywhere on the page.
func TestACompoundBrokenAtItsOwnHyphenIsNotAReport(t *testing.T) {
	const theirs = `     In this subsection, the letters A and B denote Morita equivalent k-
algebras and P an invertible (A, B)k -bimodule. Choose a (B, A)k -bimodule Q
inverse to P and isomorphisms`
	const ours = `In this subsection, the letters A and B denote Morita equivalent $k$-algebras and P an invertible $(A,B)_k$-bimodule. Choose a $(B,A)_k$-bimodule Q inverse to P and isomorphisms`
	if lost := Extra(ours, theirs); len(lost) != 0 {
		t.Errorf("Extra() = %v, want nothing", lost)
	}
	if lost := Page(ours, theirs); len(lost) != 0 {
		t.Errorf("Page() = %v, want nothing", lost)
	}
}

// Page 237. The two propositions are listed as (i), (ii), (iii), and the entry
// that ends in a hyphen is followed by a blank line, so the word was never put
// back together. This one is still in the corpus.
func TestAWordWeLeftBrokenAtALineEndIsFound(t *testing.T) {
	const theirs = `           (ii) The ring Z is isomorphic to the product of a family of commu-
           tative fields.`
	const ours = `(ii) The ring Z is isomorphic to the product of a family of commu-

tative fields.`
	got := map[string]bool{}
	for _, l := range Extra(ours, theirs) {
		got[l.Word] = true
	}
	if !got["commu"] || !got["tative"] {
		t.Errorf("Extra() = %v, want both halves of the broken word", got)
	}
}

// Page 102. The typesetter breaks "de-" at the end of a line and pdftotext sets
// the subscript lambda of the formula above it on a line of its own in between,
// so the second reading has "de-", then a lambda, then "scription". Putting the
// word back together has to step over that lambda.
func TestAWordBrokenAroundAStrandedSubscriptIsPutBack(t *testing.T) {
	const theirs = `     The mapping λ∈SM αλ from λ∈SM Vλ ⊗Do Sλ to M provides a de-
                                                      λ
scription (VIII, p. 69, Definition 5) of the semisimple B-module M. By
VIII, p. 70, Proposition 8, b), for every λ ∈ SM , the mapping from Sλ to
HomB (Vλ , M) described in e) is bijective and Dλ -linear.`
	const ours = `The mapping $\\sum_{\\lambda\\in\\mathscr{S}_M}\\alpha_{\\lambda}$ from $\\bigoplus_{\\lambda\\in\\mathscr{S}_M}V_{\\lambda}\\otimes_{D^o_{\\lambda}}S_{\\lambda}$ to M provides a description (VIII, p. 69, Definition 5) of the semisimple B-module M. By VIII, p. 70, Proposition 8, b), for every $\\lambda \\in \\mathscr{S}_M$, the mapping from $S_{\\lambda}$ to Hom$_B(V_{\\lambda}, M)$ described in e) is bijective and $D_{\\lambda}$-linear.`
	if lost := Page(ours, theirs); len(lost) != 0 {
		t.Errorf("Page() = %v, want nothing", lost)
	}
}

// TestShortWordsAreNotWorthPrinting records where the floor comes from. The two
// readings split around inline mathematics differently and the pieces that fall
// out are two and three letters long.
func TestShortWordsAreNotWorthPrinting(t *testing.T) {
	for _, w := range []string{"the", "and", "ring", "id"} {
		if candidate(w) {
			t.Errorf("candidate(%q) = true, want false", w)
		}
	}
	if !candidate("module") {
		t.Error(`candidate("module") = false, want true`)
	}
}

// TestACapitalIsNotComparable records the other half of that. A capital in
// pdftotext is either a proper name, which both readings spell the same, or a
// variable we set in mathematics, and comparing those two is comparing a
// formula with the debris of one.
func TestACapitalIsNotComparable(t *testing.T) {
	for _, w := range []string{"Wedderburn", "Artinian", "sigmaα"} {
		if candidate(w) {
			t.Errorf("candidate(%q) = true, want false", w)
		}
	}
}

// TestTheMacrosAreNotWordsOfThePage. \widetilde{n} would otherwise put
// "widetilde" on the page, and worse, hide nothing, since what we are asking is
// which of pdftotext's words we do not have.
func TestTheMacrosAreNotWordsOfThePage(t *testing.T) {
	got := strip(`the $\widetilde{A}$-module $\mathscr{C}$ is semisimple`)
	if strings.Contains(got, "widetilde") || strings.Contains(got, "mathscr") {
		t.Errorf("strip() = %q, want the macro names gone", got)
	}
	if !strings.Contains(got, "semisimple") {
		t.Errorf("strip() = %q, want the words kept", got)
	}
}
