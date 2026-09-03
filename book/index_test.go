package book

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// TestAnIndexEntryBecomesAParagraphAndADisplayStaysOneBlock is the shape the
// English Algebra I to III index of notation actually has on disk. Its first
// half puts a blank line between entries, its second half puts none, and the
// displays are $$ fences either way. The build before this test ran the packed
// half into one paragraph and, worse, split a $$ fence off its formula, which
// tectonic reported as "! Missing $ inserted." at line 21733 of a 21734-line
// book.tex and refused to keep the PDF for.
func TestAnIndexEntryBecomesAParagraphAndADisplayStaysOneBlock(t *testing.T) {
	body := "$0, 1$: I, § 2, no. 1.\n" +
		"\n" +
		"$$\n-x, x - y : \\text{I, § 2, no. 8}.\n$$\n" +
		"\n" +
		"$nx$: I, § 2, no. 8.\n" +
		"$x^n$: I, § 2, no. 8.\n" +
		"$$\n\\frac{1}{x} : \\text{I, § 2, no. 8}.\n$$\n" +
		"$\\alpha.x$: I, § 3, no. 1.\n"

	want := "$0, 1$: I, § 2, no. 1.\n" +
		"\n" +
		"$$\n-x, x - y : \\text{I, § 2, no. 8}.\n$$\n" +
		"\n" +
		"$nx$: I, § 2, no. 8.\n" +
		"\n" +
		"$x^n$: I, § 2, no. 8.\n" +
		"\n" +
		"$$\n\\frac{1}{x} : \\text{I, § 2, no. 8}.\n$$\n" +
		"\n" +
		"$\\alpha.x$: I, § 3, no. 1."

	if got := oneEntryToALine(body); got != want {
		t.Errorf("oneEntryToALine gave\n%q\nwant\n%q", got, want)
	}
}

// TestADisplayThatOpensAnIndexKeepsItsFormula guards the one line of
// oneEntryToALine that has to look at what came before it. The index of
// notation of Algebra I opens on a display, so the closing fence arrives when
// nothing has been emitted yet by any rule but the opening fence's own.
func TestADisplayThatOpensAnIndexKeepsItsFormula(t *testing.T) {
	got := oneEntryToALine("$$\nx \\top y\n$$\n\n$0$: I, § 2.\n")
	want := "$$\nx \\top y\n$$\n\n$0$: I, § 2."
	if got != want {
		t.Errorf("oneEntryToALine gave %q, want %q", got, want)
	}
}

// TestAnIndexReferenceToAChapterTheVolumeHasNotGotIsRejected is the six
// findings the check made the first time it was run over the library, one line
// each, standing for the four printings they came from. Every one of them is a
// numeral that came back wrong from the scan, and every one of them is settled
// by the entries around it, because an index is in alphabetical order and a
// misread numeral does not move its entry.
func TestAnIndexReferenceToAChapterTheVolumeHasNotGotIsRejected(t *testing.T) {
	v := &Volume{
		Meta: corpus.Book{ID: "top-v-x", Chapters: []string{"V", "VI", "VII", "VIII", "IX", "X"}},
		Terminology: &Section{
			Kind: corpus.KindTerminology,
			Path: "content/en/top/index_of_terminology_v_x.md",
			Body: "Space, Polish : IX, 6, 1.\n" +
				"Space, pseudo-compact : XI, 1, Exercise 21.\n" +
				"Space, real-compact : X, 4, Exercise 17.\n",
		},
	}
	a := &Audit{}
	a.indexed(v)

	if len(a.Checks) != 1 {
		t.Fatalf("indexed made %d checks, want 1", len(a.Checks))
	}
	c := a.Checks[0]
	if c.OK {
		t.Errorf("a reference to chapter XI of a volume that ends at X passed: %s", c.Detail)
	}
	if c.Detail != "3 references, 1 wrong" {
		t.Errorf("detail is %q, want %q", c.Detail, "3 references, 1 wrong")
	}
	if len(c.Notes) != 1 || !strings.Contains(c.Notes[0], "pseudo-compact") {
		t.Errorf("notes are %q, want the pseudo-compact line", c.Notes)
	}
}

// TestAVolumeWithNoIndexPassesTheIndexCheck is four of the forty four printings
// the library builds: two of the three volumes of the historical notes and two
// French volumes whose printing binds no index at all. They have to come back a
// pass with nothing compared rather than a pass for a check that did not run,
// which is what the detail says.
func TestAVolumeWithNoIndexPassesTheIndexCheck(t *testing.T) {
	a := &Audit{}
	a.indexed(&Volume{Meta: corpus.Book{ID: "hist", Chapters: []string{"I"}}})

	if len(a.Checks) != 1 || !a.Checks[0].OK {
		t.Fatalf("a volume with no index did not pass: %+v", a.Checks)
	}
	if a.Checks[0].Detail != "0 references, 0 wrong" {
		t.Errorf("detail is %q, want %q", a.Checks[0].Detail, "0 references, 0 wrong")
	}
}
