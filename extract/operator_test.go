package extract

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// Page 113 of Théories spectrales 1 and 2 fails twice over, and the two
// failings are the same two lines, so they are asserted off one fixture. The
// page sets the norm of an example as a sup over x in X, and the involution of
// the same example on the line under it as f maps to the conjugate of f. The
// sup is a large operator TeX sets in roman rather than out of CMEX, and the
// bar over the second f is drawn between the two lines.
var supAndBarPage = pdfsrc.Page{
	Number: 113, Width: 659, Height: 999,
	Spans: []pdfsrc.Span{
		{Top: 478, Left: 81, Width: 60, Height: 14, Text: "nées sur"},
		{Top: 478, Left: 149, Width: 12, Height: 14, Text: "X"},
		{Top: 478, Left: 161, Width: 153, Height: 14, Text: ", munie de la norme"},
		{Top: 477, Left: 322, Width: 8, Height: 15, Font: 2, Text: "k"},
		{Top: 478, Left: 330, Width: 8, Height: 15, Font: 1, Text: "f"},
		{Top: 477, Left: 340, Width: 8, Height: 15, Font: 2, Text: "k"},
		{Top: 478, Left: 357, Width: 46, Height: 14, Text: "= sup"},
		{Top: 495, Left: 378, Width: 7, Height: 11, Font: 4, Text: "x"},
		{Top: 494, Left: 385, Width: 8, Height: 11, Font: 13, Text: "∈"},
		{Top: 495, Left: 394, Width: 10, Height: 11, Font: 3, Text: "X"},
		{Top: 477, Left: 403, Width: 5, Height: 15, Font: 2, Text: "|"},
		{Top: 478, Left: 408, Width: 8, Height: 15, Font: 1, Text: "f"},
		{Top: 478, Left: 417, Width: 6, Height: 14, Text: "("},
		{Top: 478, Left: 424, Width: 9, Height: 15, Font: 1, Text: "x"},
		{Top: 478, Left: 433, Width: 6, Height: 14, Text: ")"},
		{Top: 477, Left: 440, Width: 5, Height: 15, Font: 2, Text: "|"},
		{Top: 478, Left: 452, Width: 126, Height: 14, Text: "et de l’involution"},
		{Top: 510, Left: 81, Width: 8, Height: 15, Font: 1, Text: "f"},
		{Top: 509, Left: 97, Width: 16, Height: 15, Font: 2, Text: "7→"},
		{Top: 510, Left: 119, Width: 8, Height: 15, Font: 1, Text: "f"},
		{Top: 510, Left: 129, Width: 400, Height: 14, Text: ", est une algèbre de Banach involutive. La sous-algèbre"},
		{Top: 505, Left: 535, Width: 11, Height: 21, Font: 6, Text: "C"},
		{Top: 515, Left: 546, Width: 6, Height: 11, Font: 3, Text: "0"},
		{Top: 510, Left: 553, Width: 25, Height: 14, Text: "(X)"},
	},
	Rules: []pdfsrc.Rule{{Top: 506, Left: 119, Width: 10, Thickness: 0.397, Length: 6.50, Size: 4.1}},
}

// pair renders the two lines a fixture page holds.
func pair(t *testing.T, l *pdfsrc.Layout, p pdfsrc.Page) (string, string) {
	t.Helper()
	lines := Lines(l, p)
	if len(lines) != 2 {
		for i, one := range lines {
			t.Logf("line %d: %s", i, Render(one))
		}
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	return Render(lines[0]), Render(lines[1])
}

// The limit of a sup belongs under the sup. A sum carries its limit because it
// is drawn out of CMEX and the size of the glyph says so, and sup and inf and
// lim are drawn in roman at the size of the body, so nothing about the type
// says they take one. The limit here is set at 494 between a line that ends at
// 492 and a line that opens at 505, and with the sup counting for nothing the
// nearer band took it and the page read x in X as an exponent of the first
// French word of the sentence under it.
func TestALimitUnderASupGoesToTheSup(t *testing.T) {
	got, _ := pair(t, frlay, supAndBarPage)
	want := `nées sur X, munie de la norme $\|f\|$ = sup$_{x\in X}|f(x)|$ et de l’involution`
	if got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// A bar goes to the line it was drawn over, not to the line nearest it. The
// line above has taken the limit of its sup down to 506, which is where this
// bar is drawn, so it holds the bar inside its extent and is offered first,
// and it has nothing at all under the bar to bar. The conjugate on the line
// below is what the bar belongs to and the page read f maps to f without it.
func TestABarGoesToTheLineItWasDrawnOver(t *testing.T) {
	_, got := pair(t, frlay, supAndBarPage)
	want := `$f\mapsto \overline{f}$, est une algèbre de Banach involutive. La sous-algèbre $\mathscr{C}_0(X)$`
	if got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}
