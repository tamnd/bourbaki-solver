package extract

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// The Fekete inequality at the foot of page 33 of Théories spectrales I, which
// bounds the limit superior of a_n over n by the infimum of a_m over m. The
// three fractions are built up, so the scan finds three bands where the page
// prints one line: the numerators at 611, the operators and the two inequality
// signs at 622, and the denominators mixed in with the bounds at 633.
var fekete = pdfsrc.Page{
	Number: 33, Width: 659, Height: 999,
	Spans: []pdfsrc.Span{
		{Top: 611, Left: 262, Width: 9, Height: 15, Font: 1, Text: "a"},
		{Top: 617, Left: 271, Width: 8, Height: 11, Font: 4, Text: "n"},
		{Top: 611, Left: 334, Width: 9, Height: 15, Font: 1, Text: "a"},
		{Top: 617, Left: 343, Width: 11, Height: 11, Font: 4, Text: "m"},
		{Top: 611, Left: 428, Width: 9, Height: 15, Font: 1, Text: "a"},
		{Top: 617, Left: 437, Width: 8, Height: 11, Font: 4, Text: "n"},

		{Top: 622, Left: 207, Width: 50, Height: 14, Text: "lim sup"},
		{Top: 622, Left: 285, Width: 13, Height: 17, Font: 5, Text: "⩽"},
		{Top: 622, Left: 306, Width: 19, Height: 14, Text: "inf"},
		{Top: 622, Left: 361, Width: 13, Height: 17, Font: 5, Text: "⩽"},
		{Top: 622, Left: 378, Width: 44, Height: 14, Text: "lim inf"},
		{Top: 622, Left: 447, Width: 5, Height: 15, Font: 1, Text: "."},

		{Top: 638, Left: 211, Width: 8, Height: 11, Font: 4, Text: "n"},
		{Top: 638, Left: 219, Width: 13, Height: 11, Font: 13, Text: "→"},
		{Top: 638, Left: 231, Width: 10, Height: 11, Font: 3, Text: "+"},
		{Top: 638, Left: 241, Width: 13, Height: 11, Font: 13, Text: "∞"},
		{Top: 633, Left: 266, Width: 10, Height: 15, Font: 1, Text: "n"},
		{Top: 635, Left: 303, Width: 11, Height: 11, Font: 4, Text: "m"},
		{Top: 636, Left: 314, Width: 9, Height: 12, Font: 13, Text: "⩾"},
		{Top: 636, Left: 323, Width: 6, Height: 11, Font: 3, Text: "1"},
		{Top: 633, Left: 337, Width: 14, Height: 15, Font: 1, Text: "m"},
		{Top: 635, Left: 380, Width: 8, Height: 11, Font: 4, Text: "n"},
		{Top: 634, Left: 387, Width: 13, Height: 11, Font: 13, Text: "→"},
		{Top: 635, Left: 400, Width: 10, Height: 11, Font: 3, Text: "+"},
		{Top: 634, Left: 410, Width: 13, Height: 11, Font: 13, Text: "∞"},
		{Top: 633, Left: 432, Width: 10, Height: 15, Font: 1, Text: "n"},
	},
	Rules: []pdfsrc.Rule{
		{Top: 628, Left: 262, Width: 17, Thickness: 0.397, Length: 11.39, Size: 4.1},
		{Top: 628, Left: 334, Width: 21, Thickness: 0.397, Length: 13.74, Size: 4.1},
		{Top: 628, Left: 428, Width: 17, Thickness: 0.397, Length: 11.39, Size: 4.1},
	},
}

// A fraction built up in a display is one line of the page.
//
// Neither half of it is a script: both are set at the size of the body, so the
// tests that put a stray back on its line refuse them, and the display came
// out as a sentence reading "lim sup $\leqslant$ inf $\leqslant$ lim inf."
// with the bounds and the denominators left over as
// "_{n\rightarrow+\infty}n_{m\geqslant 1}m_n^{\rightarrow}_+^{\infty}n",
// which is a double subscript KaTeX refuses and no fraction anywhere.
func TestAFractionBuiltUpInADisplayIsOneLine(t *testing.T) {
	lines := Lines(frlay, fekete)
	if len(lines) != 1 {
		for i, one := range lines {
			t.Logf("line %d: %s", i, Render(one))
		}
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	want := `lim sup$_{n\rightarrow+\infty}\frac{a_n}{n}\leqslant$ inf$_{m\geqslant 1}\frac{a_m}{m}\leqslant$ lim inf$_{n\rightarrow+\infty}\frac{a_n}{n}$.`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Two entries of the index of notation of Théories spectrales V, page 563. The
// first names the Fourier transform and the second the cotransformation, which
// is the same letter with a bar over it. The bar stands one unit under the box
// of the letter above and along the top of the box of its own, and both letters
// are as wide as the bar, so a fraction read off the bands alone joins the two
// entries and prints the letter over itself.
var notationIndex = pdfsrc.Page{
	Number: 563, Width: 659, Height: 999,
	Spans: []pdfsrc.Span{
		{Top: 304, Left: 81, Width: 12, Height: 17, Font: 1, Text: "F"},
		{Top: 308, Left: 99, Width: 450, Height: 12, Text: "(transformation de Fourier)"},
		{Top: 308, Left: 557, Width: 21, Height: 12, Text: "196"},

		{Top: 322, Left: 81, Width: 12, Height: 17, Font: 1, Text: "F"},
		{Top: 325, Left: 99, Width: 450, Height: 12, Text: "(cotransformation de Fourier)"},
		{Top: 325, Left: 557, Width: 21, Height: 12, Text: "196"},
	},
	Rules: []pdfsrc.Rule{
		{Top: 322, Left: 81, Width: 14, Thickness: 0.397, Length: 9.23, Size: 4.1},
	},
}

func TestABarOverOneLetterDoesNotJoinTheLineAboveIt(t *testing.T) {
	lines := Lines(frlay, notationIndex)
	if len(lines) != 2 {
		for i, one := range lines {
			t.Logf("line %d: %s", i, Render(one))
		}
		t.Fatalf("got %d lines, want the two entries kept apart", len(lines))
	}
	if got := Render(lines[1]); !strings.Contains(got, `\overline{`) {
		t.Errorf("the second entry reads %s, want the letter under a bar", got)
	}
	for i, one := range lines {
		if got := Render(one); strings.Contains(got, `\frac{`) {
			t.Errorf("line %d reads %s, want no fraction anywhere", i, got)
		}
	}
}
