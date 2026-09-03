package book

import "testing"

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
