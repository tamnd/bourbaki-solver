package extract

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// The boxes below are the ones pdftohtml reports for the pages named in each
// test, so what is asserted is what the volumes print rather than what would be
// convenient to assert. A line is cut short where the rest of it says nothing
// about the bar.

// frlay is what the French volumes declare: 16 for the 10 point body, 12 for
// the 8 point script beside it, then 15 for the 9 point of an exercise with 10
// and 7 for the two script sizes under that.
var frlay = &pdfsrc.Layout{Fonts: map[int]pdfsrc.FontSpec{
	0:  {ID: 0, Size: 16, Family: "LMRoman10"},
	1:  {ID: 1, Size: 16, Family: "LMMathItalic10"},
	2:  {ID: 2, Size: 16, Family: "LMMathSymbols10"},
	3:  {ID: 3, Size: 12, Family: "LMRoman8"},
	4:  {ID: 4, Size: 12, Family: "LMMathItalic8"},
	5:  {ID: 5, Size: 16, Family: "MSAM10"},
	6:  {ID: 6, Size: 16, Family: "rsfs10"},
	7:  {ID: 7, Size: 15, Family: "LMRoman10"},
	8:  {ID: 8, Size: 15, Family: "LMMathItalic10"},
	9:  {ID: 9, Size: 15, Family: "MSAM10"},
	10: {ID: 10, Size: 10, Family: "LMRoman7"},
	11: {ID: 11, Size: 10, Family: "LMMathItalic7"},
	12: {ID: 12, Size: 7, Family: "LMMathItalic5"},
	13: {ID: 13, Size: 12, Family: "LMMathSymbols8"},
	14: {ID: 14, Size: 9, Family: "LMRoman6"},
	15: {ID: 15, Size: 9, Family: "LMMathItalic6"},
	16: {ID: 16, Size: 9, Family: "LMMathSymbols6"},
}}

// enlay is what the English volumes declare. Algebra VIII is set in the Latin
// Modern fonts and Lie 7 to 9 in the Computer Modern ones it was scanned from,
// and both put the body at 15.
var enlay = &pdfsrc.Layout{Fonts: map[int]pdfsrc.FontSpec{
	0:  {ID: 0, Size: 15, Family: "LMRoman10"},
	1:  {ID: 1, Size: 13, Family: "LMRoman9"},
	2:  {ID: 2, Size: 10, Family: "LMRoman7"},
	3:  {ID: 3, Size: 15, Family: "CMR10"},
	4:  {ID: 4, Size: 15, Family: "CMMI10"},
	5:  {ID: 5, Size: 10, Family: "CMR7"},
	6:  {ID: 6, Size: 10, Family: "CMMI7"},
	7:  {ID: 7, Size: 10, Family: "CMSY7"},
	8:  {ID: 8, Size: 15, Family: "CMEX10"},
	9:  {ID: 9, Size: 10, Family: "EUFM7"},
	10: {ID: 10, Size: 7, Family: "LMMathSymbols5"},
}}

// sole renders the one line a fixture page holds.
func sole(t *testing.T, l *pdfsrc.Layout, p pdfsrc.Page) string {
	t.Helper()
	lines := Lines(l, p)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	return Render(lines[0])
}

// A fraction set inline arrives as two runs at the same left edge, one above the
// rule and one below it, and nothing tells the halves apart but the rule. This
// is page 99 of Théories spectrales 1 and 2, which sets f = P(z)/Q(z).
func TestFractionSetInline(t *testing.T) {
	p := pdfsrc.Page{
		Number: 99, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 476, Left: 81, Width: 134, Height: 14, Text: "en aucun point de"},
			{Top: 476, Left: 222, Width: 9, Height: 14, Text: "S"},
			{Top: 476, Left: 238, Width: 105, Height: 14, Text: "et que l’on ait"},
			{Top: 476, Left: 351, Width: 8, Height: 15, Font: 1, Text: "f"},
			{Top: 476, Left: 368, Width: 13, Height: 14, Text: "="},
			{Top: 471, Left: 390, Width: 14, Height: 11, Font: 3, Text: "P("},
			{Top: 484, Left: 390, Width: 15, Height: 11, Font: 3, Text: "Q("},
			{Top: 471, Left: 404, Width: 6, Height: 11, Font: 4, Text: "z"},
			{Top: 484, Left: 404, Width: 6, Height: 11, Font: 4, Text: "z"},
			{Top: 471, Left: 410, Width: 5, Height: 11, Font: 3, Text: ")"},
			{Top: 484, Left: 411, Width: 5, Height: 11, Font: 3, Text: ")"},
			{Top: 476, Left: 418, Width: 62, Height: 14, Text: ". Notons"},
		},
		Rules: []pdfsrc.Rule{{Top: 482, Left: 389, Width: 26, Thickness: 0.397, Length: 17.42, Size: 4.1}},
	}
	want := `en aucun point de S et que l’on ait $f=\frac{P(z)}{Q(z)}$. Notons`
	if got := sole(t, frlay, p); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// A bar with nothing above it is an overline, and it covers the base and the
// index together where it spans both. This is page 431 of Théories spectrales 5
// and the conjugate of f_1 under an integral.
func TestBarOverNothingIsAnOverline(t *testing.T) {
	p := pdfsrc.Page{
		Number: 431, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 624, Left: 202, Width: 8, Height: 15, Font: 1, Text: "f"},
			{Top: 629, Left: 210, Width: 6, Height: 11, Font: 3, Text: "1"},
			{Top: 624, Left: 223, Width: 10, Height: 15, Font: 1, Text: "γ", Bold: true},
			{Top: 631, Left: 233, Width: 10, Height: 11, Font: 3, Text: "G"},
			{Top: 631, Left: 243, Width: 11, Height: 11, Font: 4, Text: ",χ"},
			{Top: 624, Left: 256, Width: 6, Height: 14, Text: "("},
			{Top: 624, Left: 262, Width: 8, Height: 15, Font: 1, Text: "g"},
			{Top: 624, Left: 270, Width: 6, Height: 14, Text: ")"},
			{Top: 624, Left: 277, Width: 8, Height: 15, Font: 1, Text: "f"},
			{Top: 629, Left: 285, Width: 6, Height: 11, Font: 3, Text: "2"},
		},
		Rules: []pdfsrc.Rule{{Top: 621, Left: 202, Width: 15, Thickness: 0.397, Length: 10.06, Size: 4.1}},
	}
	want := `$\overline{f_1}\boldsymbol{\gamma }_{G,\chi}(g)f_2$`
	if got := sole(t, frlay, p); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// A bar over an index with the exponent of the same base beside it is not a
// fraction, however much it looks like one to the layer. TeX sets a superscript
// flush against the box it hangs off and leaves room on each side of a
// fraction, so the rule that starts at the end of the run in front of it is the
// bar over the index alone. This is page 431 again, where the space is the one
// of square integrable functions of type chi bar.
func TestBarUnderAnExponentIsNotAFraction(t *testing.T) {
	p := pdfsrc.Page{
		Number: 431, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 206, Left: 81, Width: 8, Height: 15, Font: 1, Text: "g"},
			{Top: 205, Left: 94, Width: 11, Height: 15, Font: 2, Text: "∈"},
			{Top: 206, Left: 109, Width: 13, Height: 14, Text: "G"},
			{Top: 206, Left: 128, Width: 86, Height: 14, Text: "appartient à", Italic: true},
			{Top: 202, Left: 220, Width: 14, Height: 21, Font: 6, Text: "L"},
			{Top: 214, Left: 235, Width: 8, Height: 11, Font: 4, Text: "χ"},
			{Top: 204, Left: 238, Width: 6, Height: 11, Font: 3, Text: "2"},
			{Top: 206, Left: 245, Width: 26, Height: 14, Text: "(G)"},
		},
		Rules: []pdfsrc.Rule{{Top: 215, Left: 234, Width: 9, Thickness: 0.397, Length: 5.75, Size: 4.1}},
	}
	want := `$g\in G$ appartient à $\mathscr{L}_{\overline{\chi}}^2(G)$`
	if got := sole(t, frlay, p); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// Both halves of a fraction are set off the line and each carries its own
// scripts, so each is measured from its own shallowest piece and written back
// at the level it was set in. This is page 120 of Théories spectrales 5, where
// the numerator holds 2k+1 and the denominator 2 to the n.
func TestHalvesAreLevelledFromTheirOwnDepth(t *testing.T) {
	p := pdfsrc.Page{
		Number: 120, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 433, Left: 289, Width: 7, Height: 13, Font: 7, Text: "0"},
			{Top: 433, Left: 317, Width: 10, Height: 13, Font: 7, Text: "si"},
			{Top: 430, Left: 334, Width: 6, Height: 9, Font: 10, Text: "2"},
			{Top: 430, Left: 339, Width: 6, Height: 9, Font: 11, Text: "k"},
			{Top: 441, Left: 341, Width: 6, Height: 9, Font: 10, Text: "2"},
			{Top: 430, Left: 346, Width: 15, Height: 9, Font: 10, Text: "+1"},
			{Top: 440, Left: 347, Width: 7, Height: 7, Font: 12, Text: "n"},
			{Top: 433, Left: 367, Width: 23, Height: 13, Font: 8, Text: "< y"},
			{Top: 433, Left: 395, Width: 12, Height: 16, Font: 9, Text: "⩽"},
			{Top: 430, Left: 413, Width: 6, Height: 9, Font: 10, Text: "2"},
			{Top: 430, Left: 418, Width: 6, Height: 9, Font: 11, Text: "k"},
			{Top: 441, Left: 420, Width: 6, Height: 9, Font: 10, Text: "2"},
			{Top: 430, Left: 425, Width: 15, Height: 9, Font: 10, Text: "+1"},
			{Top: 440, Left: 426, Width: 7, Height: 7, Font: 12, Text: "n"},
			{Top: 433, Left: 442, Width: 4, Height: 13, Font: 8, Text: "."},
		},
		Rules: []pdfsrc.Rule{
			{Top: 439, Left: 333, Width: 28, Thickness: 0.397, Length: 18.43, Size: 4.1},
			{Top: 439, Left: 412, Width: 28, Thickness: 0.397, Length: 18.44, Size: 4.1},
		},
	}
	want := `0 si $\frac{2k+1}{2^n}< y\leqslant \frac{2k+1}{2^n}$.`
	if got := sole(t, frlay, p); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// What the fraction reaches is what stands over the rule and what stands under
// it, and the run that starts clear of the rule stays on the line. This is page
// 31 of Théories spectrales 3 to 5, where the denominator 2nM ends one unit
// past the rule and the = 1 after it belongs to the formula and not to the
// fraction.
func TestFractionKeepsWhatTheBarDoesNotReach(t *testing.T) {
	p := pdfsrc.Page{
		Number: 31, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 705, Left: 231, Width: 13, Height: 17, Font: 5, Text: "⩽"},
			{Top: 701, Left: 252, Width: 6, Height: 11, Font: 3, Text: "1"},
			{Top: 714, Left: 252, Width: 6, Height: 11, Font: 3, Text: "2"},
			{Top: 705, Left: 265, Width: 13, Height: 14, Text: "+"},
			{Top: 714, Left: 284, Width: 6, Height: 11, Font: 3, Text: "2"},
			{Top: 701, Left: 287, Width: 8, Height: 11, Font: 4, Text: "n"},
			{Top: 713, Left: 290, Width: 8, Height: 11, Font: 4, Text: "n"},
			{Top: 701, Left: 295, Width: 12, Height: 11, Font: 3, Text: "M"},
			{Top: 714, Left: 298, Width: 12, Height: 11, Font: 3, Text: "M"},
			{Top: 705, Left: 318, Width: 125, Height: 14, Text: "= 1, et par suite"},
		},
		Rules: []pdfsrc.Rule{
			{Top: 711, Left: 252, Width: 6, Thickness: 0.397, Length: 4.23, Size: 4.1},
			{Top: 711, Left: 283, Width: 26, Thickness: 0.397, Length: 17.10, Size: 4.1},
		},
	}
	want := `$\leqslant \frac{1}{2}+\frac{nM}{2nM}= 1$, et par suite`
	if got := sole(t, frlay, p); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// A word under a bar keeps the spaces the line was set with. Two runs of
// mathematics are left to touch wherever the layer put them, because the room
// between one symbol and the next is the setting and not the reading, but a
// word has no outside to be written to once it stands inside a fraction. This
// is page 346 of Lie 7 to 9 and the determinant of exp ad x - 1 over ad x.
func TestWordsUnderABarKeepTheirSpaces(t *testing.T) {
	p := pdfsrc.Page{
		Number: 346, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 852, Left: 371, Width: 47, Height: 13, Font: 3, Text: ") = det"},
			{Top: 848, Left: 419, Width: 33, Height: 9, Font: 5, Text: "exp ad"},
			{Top: 860, Left: 437, Width: 13, Height: 9, Font: 5, Text: "ad"},
			{Top: 860, Left: 454, Width: 7, Height: 9, Font: 6, Text: "x"},
			{Top: 848, Left: 456, Width: 7, Height: 9, Font: 6, Text: "x"},
			{Top: 846, Left: 463, Width: 9, Height: 14, Font: 7, Text: "−"},
			{Top: 848, Left: 472, Width: 6, Height: 9, Font: 5, Text: "1"},
			{Top: 852, Left: 485, Width: 71, Height: 13, Font: 3, Text: "(Chap. III,"},
		},
		Rules: []pdfsrc.Rule{{Top: 858, Left: 419, Width: 59, Thickness: 0.398, Length: 39.26, Size: 4.1}},
	}
	want := `) = det$\frac{exp ad x-1}{ad x}$ (Chap. III,`
	if got := sole(t, enlay, p); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// A fraction set as a display puts its halves on lines of their own, and the
// line the rule is nearest holds the brackets around it and nothing else. There
// is no run inside the span, so the rule is left where it is. This is page 346
// of Lie 7 to 9 again, the sum of ad x to the p over p+1 factorial.
func TestDisplayBarIsRefused(t *testing.T) {
	p := pdfsrc.Page{
		Number: 346, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 579, Left: 105, Width: 9, Height: 13, Font: 4, Text: "λ"},
			{Top: 582, Left: 114, Width: 7, Height: 12, Font: 9, Text: "h"},
			{Top: 579, Left: 121, Width: 6, Height: 13, Font: 3, Text: "("},
			{Top: 579, Left: 127, Width: 9, Height: 13, Font: 4, Text: "x"},
			{Top: 579, Left: 135, Width: 46, Height: 13, Font: 3, Text: ") = det"},
			{Top: 573, Left: 184, Width: 13, Height: 19, Font: 8, Text: "⎝"},
			{Top: 562, Left: 197, Width: 22, Height: 19, Font: 8, Text: "X"},
			{Top: 579, Left: 274, Width: 22, Height: 13, Font: 3, Text: "(ad"},
			{Top: 579, Left: 300, Width: 9, Height: 13, Font: 4, Text: "x"},
			{Top: 579, Left: 309, Width: 6, Height: 13, Font: 3, Text: ")"},
			{Top: 573, Left: 322, Width: 13, Height: 19, Font: 8, Text: "⎠"},
		},
		Rules: []pdfsrc.Rule{{Top: 586, Left: 223, Width: 49, Thickness: 0.398, Length: 32.66, Size: 4.1}},
	}
	want := `$\lambda_{\mathfrak{h}}(x) =$ det $\sum$ (ad $x)$`
	if got := sole(t, enlay, p); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// The rules that draw a table are read the same way and refused the same way,
// because the line they stand nearest is wholly above them. This is the table
// of page 124 of Algebra VIII, whose head rule runs the width of the text.
func TestTableRuleIsRefused(t *testing.T) {
	p := pdfsrc.Page{
		Number: 124, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 93, Left: 93, Width: 114, Height: 13, Text: "Submodules of M"},
			{Top: 93, Left: 292, Width: 118, Height: 13, Text: "Ordered set D(M)"},
		},
		Rules: []pdfsrc.Rule{{Top: 112, Left: 85, Width: 487, Thickness: 0.497, Length: 324.58, Size: 5.1}},
	}
	want := `Submodules of M Ordered set D(M)`
	if got := sole(t, enlay, p); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// The weight of the rule tells a bar from a sign. The fraction rules and the
// overlines of the six volumes are drawn between 0.29 and 0.50 point, the rules
// of a table at 0.50 and 0.80, and the set difference sign, which is read
// elsewhere, at 0.85 and above.
func TestWeightTellsABarFromASign(t *testing.T) {
	for _, c := range []struct {
		thick float64
		want  bool
		what  string
	}{
		{0.397, true, "a fraction rule of the French volumes"},
		{0.398, true, "a fraction rule of Lie 7 to 9"},
		{0.497, true, "a table rule of Algebra VIII"},
		{0.796, true, "the heavy table rule of Algebra VIII"},
		{0.943, false, "the set difference sign at body size"},
		{1.453, false, "the set difference sign in a script"},
		{0, false, "a rule with no weight at all"},
	} {
		if got := bar(pdfsrc.Rule{Thickness: c.thick}); got != c.want {
			t.Errorf("%s at %.3f: got %v, want %v", c.what, c.thick, got, c.want)
		}
	}
}
