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

// The second half of formula (2) in the historical note of Théories spectrales
// V, page 531, which sums the reciprocal of an eigenvalue times an integral.
// The n of the eigenvalue stands five units to the left of the integral sign
// after it, which is near enough for the walk that puts a sum in front of its
// limit to take the n as a limit of the integral.
var eigenSum = pdfsrc.Page{
	Number: 531, Width: 659, Height: 999,
	Spans: []pdfsrc.Span{
		{Top: 406, Left: 79, Width: 21, Height: 14, Text: "(2)"},
		{Top: 396, Left: 173, Width: 8, Height: 7, Font: 23, Text: "Z"},
		{Top: 422, Left: 181, Width: 5, Height: 11, Font: 3, Text: "I"},
		{Top: 406, Left: 189, Width: 19, Height: 14, Text: "K("},
		{Top: 406, Left: 208, Width: 21, Height: 15, Font: 1, Text: "s, t"},
		{Top: 406, Left: 229, Width: 6, Height: 14, Text: ")"},
		{Top: 406, Left: 235, Width: 9, Height: 15, Font: 1, Text: "x"},
		{Top: 406, Left: 245, Width: 6, Height: 14, Text: "("},
		{Top: 406, Left: 251, Width: 8, Height: 15, Font: 1, Text: "s"},
		{Top: 406, Left: 259, Width: 6, Height: 14, Text: ")"},
		{Top: 406, Left: 265, Width: 9, Height: 15, Font: 1, Text: "x"},
		{Top: 406, Left: 274, Width: 6, Height: 14, Text: "("},
		{Top: 406, Left: 281, Width: 6, Height: 15, Font: 1, Text: "t"},
		{Top: 406, Left: 287, Width: 6, Height: 14, Text: ")"},
		{Top: 406, Left: 293, Width: 14, Height: 15, Font: 1, Text: "dt"},
		{Top: 406, Left: 312, Width: 13, Height: 14, Text: "="},
		{Top: 427, Left: 329, Width: 8, Height: 11, Font: 4, Text: "n"},
		{Top: 402, Left: 331, Width: 22, Height: 7, Font: 23, Text: "X"},
		{Top: 389, Left: 335, Width: 13, Height: 11, Font: 13, Text: "∞"},
		{Top: 427, Left: 337, Width: 16, Height: 11, Font: 3, Text: "=1"},
		{Top: 417, Left: 358, Width: 10, Height: 15, Font: 1, Text: "λ"},
		{Top: 395, Left: 363, Width: 8, Height: 14, Text: "1"},
		{Top: 422, Left: 367, Width: 8, Height: 11, Font: 4, Text: "n"},
		{Top: 396, Left: 380, Width: 8, Height: 7, Font: 23, Text: "Z"},
		{Top: 422, Left: 389, Width: 5, Height: 11, Font: 3, Text: "I"},
		{Top: 406, Left: 397, Width: 11, Height: 15, Font: 1, Text: "φ"},
		{Top: 411, Left: 407, Width: 8, Height: 11, Font: 4, Text: "n"},
		{Top: 406, Left: 416, Width: 6, Height: 14, Text: "("},
		{Top: 406, Left: 422, Width: 8, Height: 15, Font: 1, Text: "s"},
		{Top: 406, Left: 430, Width: 6, Height: 14, Text: ")"},
		{Top: 406, Left: 436, Width: 9, Height: 15, Font: 1, Text: "x"},
		{Top: 406, Left: 446, Width: 6, Height: 14, Text: "("},
		{Top: 406, Left: 452, Width: 8, Height: 15, Font: 1, Text: "s"},
		{Top: 406, Left: 460, Width: 6, Height: 14, Text: ")"},
		{Top: 406, Left: 466, Width: 21, Height: 15, Font: 1, Text: "ds,"},
	},
	Rules: []pdfsrc.Rule{
		{Top: 412, Left: 357, Width: 18, Thickness: 0.397, Length: 11.98, Size: 4.1},
	},
}

// An integral sets its bounds beside the sign, so nothing to the left of one is
// a limit of it.
//
// Taken as a limit, the index of the eigenvalue moved inside the integral. That
// left the numerator and the denominator of the fraction with the integral sign
// between them, which is not one stretch of the line, so the bar was refused and
// the page printed "\lambda 1\int_{nI}" where it sets one over lambda sub n.
func TestAnIntegralDoesNotTakeTheIndexBeforeItAsALimit(t *testing.T) {
	lines := Lines(frlay, eigenSum)
	if len(lines) != 1 {
		for i, one := range lines {
			t.Logf("line %d: %s", i, Render(one))
		}
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	want := "(2) $\\int_IK(s, t)x(s)x(t)dt=\\sum_{n=1}^{\\infty}\\frac{1}{\\lambda_n}\\int_I\\varphi_n(s)x(s)ds$,"
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// The head of the table on page 124 of the English Algebra VIII, which draws
// its rules the width of the type area. The one under the column heads is
// light enough to pass for a fraction bar, and read as one it joined the heads
// to the first row and ran the two columns into each other.
var tableHead = pdfsrc.Page{
	Number: 124, Width: 659, Height: 999,
	Spans: []pdfsrc.Span{
		{Top: 90, Left: 100, Width: 120, Height: 14, Text: "Submodules of M"},
		{Top: 90, Left: 340, Width: 140, Height: 14, Text: "Ordered set D(M)"},

		{Top: 120, Left: 100, Width: 120, Height: 14, Text: "Zero submodule"},
		{Top: 120, Left: 340, Width: 190, Height: 14, Text: "Smallest element of D(M)"},
	},
	Rules: []pdfsrc.Rule{
		{Top: 83, Left: 85, Width: 487, Thickness: 0.796, Length: 324.58, Size: 8.2},
		{Top: 112, Left: 85, Width: 487, Thickness: 0.497, Length: 324.58, Size: 5.1},
	},
}

func TestATableRuleDoesNotJoinTheRowsItSeparates(t *testing.T) {
	lines := Lines(frlay, tableHead)
	if len(lines) != 2 {
		for i, one := range lines {
			t.Logf("line %d: %s", i, Render(one))
		}
		t.Fatalf("got %d lines, want the head and the row kept apart", len(lines))
	}
}

// The Fourier transform on page 230 of Théories spectrales V, which is printed
// with the domain of the integral set over and under the sign. The upper R^n
// begins six units to the left of the sign and the maps-to arrow in front of it
// ends where that R begins, so the two touch across the page and are told apart
// only by the band: the arrow is on the line and the limit is fifteen units
// above it.
var fourierLimits = pdfsrc.Page{
	Number: 230, Width: 659, Height: 999,
	Spans: []pdfsrc.Span{
		{Top: 756, Left: 220, Width: 8, Height: 15, Font: 1, Text: "y"},
		{Top: 756, Left: 233, Width: 16, Height: 15, Font: 2, Text: "7→"},
		{Top: 730, Left: 248, Width: 11, Height: 11, Font: 3, Text: "R"},
		{Top: 746, Left: 254, Width: 8, Height: 7, Font: 23, Text: "Z"},
		{Top: 728, Left: 260, Width: 7, Height: 8, Font: 15, Text: "n"},
		{Top: 773, Left: 262, Width: 11, Height: 11, Font: 3, Text: "R"},
		{Top: 771, Left: 273, Width: 7, Height: 8, Font: 15, Text: "n"},
		{Top: 756, Left: 284, Width: 11, Height: 15, Font: 1, Text: "φ"},
		{Top: 757, Left: 295, Width: 6, Height: 14, Text: "("},
		{Top: 756, Left: 301, Width: 9, Height: 15, Font: 1, Text: "x"},
		{Top: 757, Left: 311, Width: 6, Height: 14, Text: ")"},
	},
}

// An integral printed with its domain across the sign keeps it.
//
// The guard that stops an integral taking the index in front of it as a limit
// has to leave this alone, and it does, because the R of the domain shares no
// height with the arrow it touches.
func TestAnIntegralKeepsALimitDrawnAcrossTheSign(t *testing.T) {
	lines := Lines(frlay, fourierLimits)
	if len(lines) != 1 {
		for i, one := range lines {
			t.Logf("line %d: %s", i, Render(one))
		}
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	got := Render(lines[0])
	if strings.Contains(got, `\mapsto^`) || !strings.Contains(got, `\int^`) {
		t.Errorf("Render: %s, want the domain left on the integral", got)
	}
}
