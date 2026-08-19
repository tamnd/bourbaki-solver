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

// Two printed lines of page 253 of Topologie algebrique I to IV, the second of
// which sets 1 over the norm of x in the middle of a sentence. The fraction is
// set inline, so both halves are scripts and both arrive on the line the
// sentence was scanned into, and the bar at 298 has nothing to join.
//
// The bold B of the line above stands across the same bar, thirteen units up.
// It is the last thing over the bar and it was read as the numerator, so the
// two lines were joined and the Exemple lost its heading.
var inlineNorm = pdfsrc.Page{
	Number: 253, Width: 659, Height: 999,
	Spans: []pdfsrc.Span{
		{Top: 271, Left: 81, Width: 60, Height: 14, Text: "Exemple"},
		{Top: 266, Left: 141, Width: 64, Height: 21, Text: ". — Soit"},
		{Top: 266, Left: 211, Width: 12, Height: 21, Text: "X"},
		{Top: 266, Left: 229, Width: 255, Height: 21, Text: "le complémentaire de l’origine dans"},
		{Top: 270, Left: 490, Width: 13, Height: 15, Text: "B"},
		{Top: 276, Left: 504, Width: 8, Height: 11, Font: 4, Text: "n"},
		{Top: 266, Left: 512, Width: 66, Height: 21, Text: ". L’appli-"},

		{Top: 288, Left: 81, Width: 66, Height: 21, Text: "cation de"},
		{Top: 288, Left: 153, Width: 12, Height: 21, Text: "X"},
		{Top: 291, Left: 169, Width: 13, Height: 15, Font: 2, Text: "×"},
		{Top: 292, Left: 185, Width: 7, Height: 15, Text: "I"},
		{Top: 288, Left: 198, Width: 33, Height: 21, Text: "dans"},
		{Top: 288, Left: 237, Width: 12, Height: 21, Text: "X"},
		{Top: 288, Left: 255, Width: 79, Height: 21, Text: "donnée par"},
		{Top: 288, Left: 340, Width: 6, Height: 21, Text: "("},
		{Top: 292, Left: 347, Width: 23, Height: 15, Font: 1, Text: "x, t"},
		{Top: 288, Left: 369, Width: 6, Height: 21, Text: ")"},
		{Top: 291, Left: 381, Width: 16, Height: 15, Font: 2, Text: "7→"},
		{Top: 288, Left: 402, Width: 21, Height: 21, Text: "((1"},
		{Top: 291, Left: 427, Width: 13, Height: 15, Font: 2, Text: "−"},
		{Top: 292, Left: 444, Width: 6, Height: 15, Font: 1, Text: "t"},
		{Top: 288, Left: 449, Width: 23, Height: 21, Text: ") +"},
		{Top: 292, Left: 476, Width: 6, Height: 15, Font: 1, Text: "t"},
		{Top: 300, Left: 484, Width: 6, Height: 11, Font: 13, Text: "k"},
		{Top: 301, Left: 490, Width: 7, Height: 11, Font: 4, Text: "x"},
		{Top: 289, Left: 491, Width: 6, Height: 11, Font: 3, Text: "1"},
		{Top: 300, Left: 497, Width: 6, Height: 11, Font: 13, Text: "k"},
		{Top: 288, Left: 506, Width: 6, Height: 21, Text: ")"},
		{Top: 292, Left: 512, Width: 9, Height: 15, Font: 1, Text: "x"},
		{Top: 288, Left: 527, Width: 51, Height: 21, Text: "est une"},
	},
	Rules: []pdfsrc.Rule{
		{Top: 298, Left: 483, Width: 20, Thickness: 0.397, Length: 13.22, Size: 4.1},
	},
}

// A fraction set inline does not join the line above it to its own.
//
// Both halves are scripts and both are on the one line already, so the bar has
// nothing to join, whatever else on the page happens to stand across it.
func TestAFractionSetInlineLeavesTheLineAboveAlone(t *testing.T) {
	lines := Lines(frlay, inlineNorm)
	if len(lines) != 2 {
		for i, one := range lines {
			t.Logf("line %d: %s", i, Render(one))
		}
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if got := Render(lines[0]); !strings.HasPrefix(got, "Exemple") || !strings.HasSuffix(got, "L’appli-") {
		t.Errorf("Render: %s, want the whole of the first printed line and no more", got)
	}
	if got := Render(lines[1]); !strings.HasPrefix(got, "cation de") || !strings.Contains(got, `\frac{1}{`) {
		t.Errorf("Render: %s, want the second printed line with its fraction", got)
	}
}

// Two printed lines of page 100 of Théories spectrales I, the second of which
// sets the conjugate of f*(1 ⊗ x). The overline of that conjugate runs from 370
// to 437, and the f* of the sentence above it stands at 416 to 431, inside the
// bar and well to the right of the middle of it.
//
// Nothing on the line above is the numerator of anything. The f* was read as
// one all the same, the two printed lines were joined, and the Lemme of I,
// paragraph 4, number 13 lost the heading that makes it a statement.
var conjugate = pdfsrc.Page{
	Number: 100, Width: 659, Height: 999,
	Spans: []pdfsrc.Span{
		{Top: 305, Left: 81, Width: 51, Height: 14, Text: "Lemme"},
		{Top: 305, Left: 132, Width: 28, Height: 14, Text: ". —"},
		{Top: 305, Left: 167, Width: 69, Height: 14, Text: "Pour tout"},
		{Top: 305, Left: 242, Width: 8, Height: 15, Font: 1, Text: "f"},
		{Top: 304, Left: 257, Width: 11, Height: 15, Font: 2, Text: "∈"},
		{Top: 300, Left: 272, Width: 12, Height: 21, Font: 6, Text: "O"},
		{Top: 305, Left: 286, Width: 25, Height: 14, Text: "(Sp"},
		{Top: 312, Left: 310, Width: 10, Height: 11, Font: 3, Text: "A"},
		{Top: 316, Left: 320, Width: 4, Height: 8, Font: 14, Text: "("},
		{Top: 316, Left: 324, Width: 9, Height: 8, Font: 14, Text: "C"},
		{Top: 316, Left: 333, Width: 4, Height: 8, Font: 14, Text: ")"},
		{Top: 305, Left: 339, Width: 6, Height: 14, Text: "("},
		{Top: 305, Left: 345, Width: 9, Height: 15, Font: 1, Text: "x"},
		{Top: 305, Left: 354, Width: 13, Height: 14, Text: "))"},
		{Top: 305, Left: 367, Width: 43, Height: 14, Text: ", on a"},
		{Top: 305, Left: 416, Width: 8, Height: 15, Font: 1, Text: "f"},
		{Top: 301, Left: 425, Width: 6, Height: 11, Font: 13, Text: "∗"},
		{Top: 305, Left: 432, Width: 15, Height: 14, Text: "(1"},
		{Top: 304, Left: 451, Width: 13, Height: 15, Font: 2, Text: "⊗"},
		{Top: 305, Left: 467, Width: 9, Height: 15, Font: 1, Text: "x"},
		{Top: 305, Left: 476, Width: 24, Height: 14, Text: ") ="},
		{Top: 305, Left: 505, Width: 8, Height: 15, Font: 1, Text: "f"},
		{Top: 305, Left: 514, Width: 15, Height: 14, Text: "(1"},
		{Top: 304, Left: 532, Width: 13, Height: 15, Font: 2, Text: "⊗"},
		{Top: 305, Left: 549, Width: 9, Height: 15, Font: 1, Text: "x"},
		{Top: 305, Left: 558, Width: 6, Height: 14, Text: ")"},
		{Top: 305, Left: 565, Width: 5, Height: 14, Text: "."},

		{Top: 331, Left: 99, Width: 115, Height: 14, Text: "Les applications"},
		{Top: 331, Left: 218, Width: 8, Height: 15, Font: 1, Text: "f"},
		{Top: 330, Left: 233, Width: 16, Height: 15, Font: 2, Text: "7→"},
		{Top: 331, Left: 254, Width: 8, Height: 15, Font: 1, Text: "f"},
		{Top: 331, Left: 263, Width: 15, Height: 14, Text: "(1"},
		{Top: 330, Left: 281, Width: 13, Height: 15, Font: 2, Text: "⊗"},
		{Top: 331, Left: 296, Width: 9, Height: 15, Font: 1, Text: "x"},
		{Top: 331, Left: 306, Width: 6, Height: 14, Text: ")"},
		{Top: 331, Left: 317, Width: 14, Height: 14, Text: "et"},
		{Top: 331, Left: 336, Width: 8, Height: 15, Font: 1, Text: "f"},
		{Top: 330, Left: 350, Width: 16, Height: 15, Font: 2, Text: "7→"},
		{Top: 331, Left: 371, Width: 8, Height: 15, Font: 1, Text: "f"},
		{Top: 328, Left: 381, Width: 6, Height: 11, Font: 13, Text: "∗"},
		{Top: 331, Left: 388, Width: 15, Height: 14, Text: "(1"},
		{Top: 330, Left: 406, Width: 13, Height: 15, Font: 2, Text: "⊗"},
		{Top: 331, Left: 422, Width: 9, Height: 15, Font: 1, Text: "x"},
		{Top: 331, Left: 432, Width: 6, Height: 14, Text: ")"},
		{Top: 331, Left: 443, Width: 135, Height: 14, Text: "sont des homomor-"},
	},
	Rules: []pdfsrc.Rule{
		{Top: 301, Left: 504, Width: 60, Thickness: 0.397, Length: 39.96, Size: 4.1},
		{Top: 327, Left: 370, Width: 67, Thickness: 0.397, Length: 44.69, Size: 4.1},
	},
}

// An overline does not take a numerator from the line above it.
//
// A numerator stands across the middle of its bar or starts where the bar
// starts, because TeX centres both halves of a fraction on a bar drawn to the
// width of the wider one. Something that merely laps over a bar does neither.
func TestAnOverlineDoesNotJoinTheLineAboveIt(t *testing.T) {
	lines := Lines(frlay, conjugate)
	if len(lines) != 2 {
		for i, one := range lines {
			t.Logf("line %d: %s", i, Render(one))
		}
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if got := Render(lines[0]); !strings.HasPrefix(got, "Lemme") || !strings.HasSuffix(got, `\otimes x)}$.`) {
		t.Errorf("Render: %s, want the whole of the first printed line and no more", got)
	}
	if got := Render(lines[1]); !strings.HasPrefix(got, "Les applications") || !strings.HasSuffix(got, "sont des homomor-") {
		t.Errorf("Render: %s, want the second printed line whole", got)
	}
}

// footnoted carries the foot of page 10 of Lie 7 to 9: the last line of a
// Proposition, the rule TeX draws to hang a note off it, and the first line of
// the note. The rule is 0.398 points thick and 85 units wide, which puts it
// inside the range that a fraction bar occupies on both counts, and prose stands
// across the middle of it with the note under it. What says it is not a bar is
// that it starts at the left margin of the type area, where the volumes never
// set a display.
var footnoted = pdfsrc.Page{
	Number: 20, Width: 659, Height: 999,
	Spans: []pdfsrc.Span{
		{Top: 812, Left: 83, Width: 66, Height: 13, Font: 3, Text: "conditions"},
		{Top: 812, Left: 155, Width: 19, Height: 13, Font: 3, Text: "(1)"},
		{Top: 812, Left: 179, Width: 24, Height: 13, Font: 3, Text: "and"},
		{Top: 812, Left: 208, Width: 19, Height: 13, Font: 3, Text: "(2)"},
		{Top: 812, Left: 232, Width: 78, Height: 13, Font: 3, Text: "of Lemma 2"},
		{Top: 812, Left: 315, Width: 255, Height: 13, Font: 3, Text: "; it is reductive."},
		{Top: 838, Left: 83, Width: 5, Height: 10, Font: 2, Text: "1"},
		{Top: 841, Left: 94, Width: 70, Height: 12, Font: 1, Text: "By Chap. I,"},
		{Top: 841, Left: 169, Width: 76, Height: 12, Font: 1, Text: "6, no. 3, Th. 3,"},
		{Top: 841, Left: 250, Width: 320, Height: 12, Font: 1, Text: "every element has a semi-simple part."},
	},
	Rules: []pdfsrc.Rule{
		{Top: 834, Left: 83, Width: 85, Thickness: 0.398, Length: 56.60, Size: 4.1},
	},
}

func TestAFootnoteRuleIsNotAFractionBar(t *testing.T) {
	lines := Lines(enlay, footnoted)
	if len(lines) != 2 {
		for i, one := range lines {
			t.Logf("line %d: %s", i, Render(one))
		}
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	if got := Render(lines[0]); !strings.HasPrefix(got, "conditions") || !strings.HasSuffix(got, "it is reductive.") {
		t.Errorf("Render: %s, want the last line of the Proposition and no more", got)
	}
	if got := Render(lines[1]); !strings.Contains(got, "By Chap. I,") {
		t.Errorf("Render: %s, want the note whole", got)
	}
}
