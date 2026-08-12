package main

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/katex"
)

// A hand repair freezes a page: extract run writes every page of a volume
// except the ones carrying manual: true, so a page repaired in March is still
// the March reading of it in August, and every improvement the extractor has
// made since goes past it. extract drift is what says so, and -fix is what
// takes back the paragraphs a read today sets and the repair does not. These
// are the two questions -fix has to get right: which paragraph of the fresh
// read stands against which paragraph of the repair, and what it costs to be
// wrong.

// The unit is the block and not the line. 390 page files carry a $$ display,
// and a display is one block written over several lines: cutting per line and
// rejoining with blank lines would reformat every one of them.
func TestParagraphsCutsAPageIntoBlocksAndNotLines(t *testing.T) {
	body := "One paragraph.\n\n$$\na = b \\\\\nc = d\n$$\n\nAnother paragraph.\n"
	got := paragraphs(body)
	want := []string{"One paragraph.", "$$\na = b \\\\\nc = d\n$$", "Another paragraph."}
	if len(got) != len(want) {
		t.Fatalf("paragraphs() gave %d blocks, want %d:\n%q", len(got), len(want), got)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("block %d:\n got %q\nwant %q", i, got[i], want[i])
		}
	}
}

// prose is what says two paragraphs are the same paragraph. The drift is inside
// the mathematics, so the words either side of it are the paragraph's name, and
// the delimiters are not part of it: a formula the repair left inline is the
// same formula a read today may set as a display.
func TestProseIsTheWordsWithoutTheMathematics(t *testing.T) {
	inline := "we have $u(x) =\\sum_i^n_{=1}a_i\\sigma_i(x)$ for every $x$ in V."
	display := "we have $$u(x) =\\sum_{i=1}^na_i\\sigma_i(x)$$ for every $x$ in V."
	if a, b := prose(inline), prose(display); a != b {
		t.Errorf("prose() reads the same words two ways:\n %q\n %q", a, b)
	}
	if got, want := prose(inline), "we have for every in V."; got != want {
		t.Errorf("prose():\n got %q\nwant %q", got, want)
	}
}

// Page 106 of the English printing, both paragraphs of it. The repair left the
// sum of the Corollary as \sum_i^n_{=1}, which KaTeX refuses and a read today
// sets. The paragraph after it is the other way about: the repair wrote \sum
// for the large operator and the fresh read writes \Sigma, which KaTeX sets
// perfectly well and is the wrong sign. So the page is neither the repair nor
// the fresh read, and taking either of them whole loses something.
func TestOvertakenTakesTheParagraphThatSetsAndLeavesTheOneThatDoesNot(t *testing.T) {
	eng, err := katex.New()
	if err != nil {
		t.Fatal(err)
	}
	repair := strings.Join([]string{
		"**Corollary.** — Let V be of finite dimension $n$ over K, so that we have $u(x) =\\sum_i^n_{=1}a_i\\sigma_i(x)$ for every $x$ in V.",
		"Denote by E the set of mappings of the form $x\\mapsto$ $\\sum_{\\sigma\\in\\Gamma}a_{\\sigma}\\sigma (x)$, where the family has finite support.",
	}, "\n\n")
	fresh := strings.Join([]string{
		"**Corollary.** — Let V be of finite dimension $n$ over K, so that we have $u(x) =\\sum_{i=1}^na_i\\sigma_i(x)$ for every $x$ in V.",
		"Denote by E the set of mappings of the form $x\\mapsto$ $\\Sigma_{\\sigma\\in\\Gamma}a_{\\sigma}\\sigma (x)$, where the family has finite support.",
	}, "\n\n")

	got, n := overtaken(eng, repair, fresh)
	if n != 1 {
		t.Fatalf("overtaken() took %d paragraphs, want 1:\n%s", n, got)
	}
	if !strings.Contains(got, "\\sum_{i=1}^na_i") {
		t.Errorf("the sum a read today sets was not taken:\n%s", got)
	}
	if !strings.Contains(got, "\\sum_{\\sigma\\in\\Gamma}") {
		t.Errorf("the large operator the repair wrote was lost to \\Sigma:\n%s", got)
	}
	if refused(eng, got) != 0 {
		t.Errorf("KaTeX still refuses %d spans of the page:\n%s", refused(eng, got), got)
	}
}

// Page 471 of the English printing. The repair rebuilt three matrices the text
// layer had flattened, and a read today still hands them back flattened, as
// more lines than the repair has and with none of the words in the same place.
// One paragraph of that page does want the fresh read, and the other three have
// to survive it.
func TestOvertakenKeepsWhatTheFreshReadWouldWreck(t *testing.T) {
	eng, err := katex.New()
	if err != nil {
		t.Fatal(err)
	}
	repair := strings.Join([]string{
		"3) For every invertible diagonal matrix we have $B_{ij}(d_i\\lambda d^-_j^1)$.",
		"(24) $A = \\begin{pmatrix} 1 & 0 \\\\ ca^{-1} & 1 \\end{pmatrix}$.",
	}, "\n\n")
	fresh := strings.Join([]string{
		"3) For every invertible diagonal matrix we have $B_{ij}(d_i\\lambda d^{-1}_j)$.",
		"$($ 1 $0)((a$ 0 $)((1a^{-1}b)$",
		"(24) A = $_--$.",
	}, "\n\n")

	got, n := overtaken(eng, repair, fresh)
	if n != 1 {
		t.Fatalf("overtaken() took %d paragraphs, want 1:\n%s", n, got)
	}
	if !strings.Contains(got, "d^{-1}_j") {
		t.Errorf("the inverse a read today sets was not taken:\n%s", got)
	}
	if !strings.Contains(got, "\\begin{pmatrix}") {
		t.Errorf("the matrix the repair rebuilt was lost:\n%s", got)
	}
}

// Nothing is written back unless the body comes out of its blocks as it went
// in. A page that does not survive that round trip is a page where -fix would
// be committing a reformatting with a repair somewhere inside it, which is not
// what anybody asked it for and not what the diff would show.
func TestOvertakenWritesNothingBackToAPageItCannotRebuild(t *testing.T) {
	eng, err := katex.New()
	if err != nil {
		t.Fatal(err)
	}
	// Three blank lines between the blocks, which is not how a page file is
	// written and not what joining the blocks again would give.
	repair := "One paragraph with $a^-_j^1$ in it.\n\n\n\nAnother paragraph."
	fresh := "One paragraph with $a^{-1}_j$ in it.\n\nAnother paragraph."
	got, n := overtaken(eng, repair, fresh)
	if n != 0 || got != repair {
		t.Errorf("overtaken() took %d paragraphs from a page it cannot rebuild:\n%s", n, got)
	}
}
