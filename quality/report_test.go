package quality

import "testing"

// P04 quotes the formula it could not set, and the line gathering of the
// extractor sometimes runs a display line together with the one under it, so
// the quoted formula spans a line break. Written out as it stands that breaks
// the list item of the report in two and leaves the first half ending in the
// space in front of the break, and the hygiene rules of the corpus refuse a
// line that ends in white space. This is the message of
// content/en/lie/VIII/13_s13_classical_splittable_simple_lie_algebras.md:499.
func TestAQuotedFormulaIsReadOnOneLine(t *testing.T) {
	msg := "KaTeX will not set it: Double superscript at position 9: $\n1\\sum^r''$"
	want := "KaTeX will not set it: Double superscript at position 9: $ 1\\sum^r''$"
	if got := oneline(msg); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}
