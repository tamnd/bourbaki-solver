package extract

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// lay is the four sizes Algebra VIII sets, as pdftohtml declares them: 15 for
// the 10 point body, 13 for the 9 point of an exercise, 10 for a 7 point script
// and 9 for a 6 point one.
var lay = &pdfsrc.Layout{Fonts: map[int]pdfsrc.FontSpec{
	0:  {ID: 0, Size: 15, Family: "LMRoman10"},
	3:  {ID: 3, Size: 15, Family: "LMRoman10"},
	6:  {ID: 6, Size: 9, Family: "LMRoman6"},
	7:  {ID: 7, Size: 15, Family: "LMMathItalic10"},
	8:  {ID: 8, Size: 9, Family: "LMMathSymbols6"},
	9:  {ID: 9, Size: 15, Family: "LMMathSymbols10"},
	10: {ID: 10, Size: 10, Family: "LMRoman7"},
}}

// The boxes here are the ones pdftohtml reports for page 50 of the English
// volume and page 266 of the French, so what is asserted is what the volumes
// print rather than what would be convenient to assert.

// In a subscript the two operands come out as one run with a space in it,
// because poppler reads the room TeX left for the rule as a space. The sign
// goes in place of the space: "I J" becomes "I-J", and M_{I J} becomes M_{I-J}.
func TestMinusInRun(t *testing.T) {
	p := pdfsrc.Page{
		Number: 50, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 109, Left: 281, Width: 14, Height: 13, Text: "M"},
			{Top: 116, Left: 295, Width: 17, Height: 9, Font: 10, Text: "I J"},
			{Top: 109, Left: 312, Width: 5, Height: 13, Text: "."},
		},
		Rules: []pdfsrc.Rule{{Top: 119, Left: 298, Width: 6, Thickness: 0.943, Length: 4.28, Size: 9.75}},
	}
	out, placed, lost := Minus(lay, p)
	if placed != 1 || lost != 0 {
		t.Fatalf("placed %d lost %d, want 1 and 0", placed, lost)
	}
	if out.Spans[1].Text != "I-J" {
		t.Errorf("the run is %q, want %q", out.Spans[1].Text, "I-J")
	}
	if p.Spans[1].Text != "I J" {
		t.Errorf("the page that went in was written to: the run is now %q", p.Spans[1].Text)
	}
}

// At body size the operands are separate runs with a gap between them, and the
// sign becomes a run of its own in the font of the operand to its left. This is
// "set J' = J - {i}", where the braces are set in the mathematics font and the
// rest is prose.
func TestMinusInGap(t *testing.T) {
	p := pdfsrc.Page{
		Number: 50, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 198, Left: 518, Width: 23, Height: 13, Font: 3, Text: "= J"},
			{Top: 197, Left: 556, Width: 7, Height: 14, Font: 9, Text: "{"},
			{Top: 198, Left: 564, Width: 5, Height: 13, Font: 7, Text: "i"},
		},
		Rules: []pdfsrc.Rule{{Top: 202, Left: 542, Width: 12, Thickness: 1.457, Length: 7.75, Size: 15.07}},
	}
	out, placed, lost := Minus(lay, p)
	if placed != 1 || lost != 0 {
		t.Fatalf("placed %d lost %d, want 1 and 0", placed, lost)
	}
	got := out.Spans[len(out.Spans)-1]
	want := pdfsrc.Span{Top: 198, Left: 542, Width: 12, Height: 13, Font: 3, Text: "-"}
	if got != want {
		t.Errorf("the run added is %+v, want %+v", got, want)
	}
}

// The boxes either side of a bar lap over it by a pixel about as often as they
// do not, because pdftohtml shares one box out between the runs it holds. So
// the question is where the middle of the bar is and not whether the bar fits.
// This is page 266 of the French volume, V_{i in I - {j}}, where the box of the
// I ends half a pixel inside the bar and the gap it leaves is a pixel narrower
// than the bar is wide.
func TestMinusInNarrowGap(t *testing.T) {
	p := pdfsrc.Page{
		Number: 266, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 488, Left: 128, Width: 7, Height: 12, Font: 8, Text: "∈"},
			{Top: 491, Left: 135, Width: 4, Height: 8, Font: 6, Text: "I"},
			{Top: 488, Left: 144, Width: 5, Height: 12, Font: 8, Text: "{"},
		},
		Rules: []pdfsrc.Rule{{Top: 493, Left: 138, Width: 6, Thickness: 0.96, Length: 4.15, Size: 9.93}},
	}
	_, placed, lost := Minus(lay, p)
	if placed != 1 || lost != 0 {
		t.Fatalf("placed %d lost %d, want 1 and 0", placed, lost)
	}
}

// A bar with a run under its middle and no space in that run has nowhere to go.
// Writing it anywhere near would put an operator into a formula that has none,
// so it is counted as lost and the page goes to the repair pass.
func TestMinusLost(t *testing.T) {
	p := pdfsrc.Page{
		Number: 1, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 100, Left: 100, Width: 40, Height: 12, Font: 3, Text: "abcdef"},
		},
		Rules: []pdfsrc.Rule{{Top: 104, Left: 115, Width: 8, Thickness: 1.457, Length: 7.75, Size: 15.07}},
	}
	_, placed, lost := Minus(lay, p)
	if placed != 0 || lost != 1 {
		t.Fatalf("placed %d lost %d, want 0 and 1", placed, lost)
	}
}

// A run whose only space is at the far end of it is a run the bar is not
// inside, whatever its box says, so the space is left alone.
func TestMinusFarSpace(t *testing.T) {
	p := pdfsrc.Page{
		Number: 1, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 100, Left: 100, Width: 60, Height: 12, Font: 3, Text: "abcde fg"},
		},
		Rules: []pdfsrc.Rule{{Top: 104, Left: 105, Width: 6, Thickness: 1.457, Length: 7.75, Size: 15.07}},
	}
	out, placed, lost := Minus(lay, p)
	if placed != 0 || lost != 1 {
		t.Fatalf("placed %d lost %d, want 0 and 1", placed, lost)
	}
	if out.Spans[0].Text != "abcde fg" {
		t.Errorf("the run is %q, want it untouched", out.Spans[0].Text)
	}
}

// A fraction bar is a horizontal rule too, and it is not this. It is a third of
// the weight and it is as long as the fraction over it, and both tests have to
// hold for a rule to be read as an operator.
func TestMinusIgnoresOtherRules(t *testing.T) {
	for _, r := range []pdfsrc.Rule{
		{Top: 104, Left: 100, Width: 30, Thickness: 0.4, Length: 20},   // a fraction bar
		{Top: 104, Left: 100, Width: 487, Thickness: 0.8, Length: 325}, // a table rule
	} {
		p := pdfsrc.Page{
			Number: 1, Width: 659, Height: 999,
			Spans: []pdfsrc.Span{{Top: 100, Left: 100, Width: 40, Height: 12, Text: "a b"}},
			Rules: []pdfsrc.Rule{r},
		}
		out, placed, lost := Minus(lay, p)
		if placed != 0 || lost != 0 {
			t.Errorf("rule %+v gave placed %d lost %d, want 0 and 0", r, placed, lost)
		}
		if out.Spans[0].Text != "a b" {
			t.Errorf("rule %+v changed the run to %q", r, out.Spans[0].Text)
		}
	}
}

// A superscript is vertically inside the band of a bar set at body size and is
// not one of its operands. This is page 374 of the English volume, L^* - {1},
// where the star is the nearest run to the left of the bar and setting the sign
// beside it writes L^{*-}{1}, which is not what the page prints. The star is set
// at 7 point and the bar is drawn at 10.
func TestMinusIgnoresSuperscript(t *testing.T) {
	p := pdfsrc.Page{
		Number: 374, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 793, Left: 136, Width: 9, Height: 13, Font: 3, Text: "L"},
			{Top: 790, Left: 145, Width: 6, Height: 10, Font: 10, Text: "∗"},
			{Top: 792, Left: 168, Width: 7, Height: 14, Font: 9, Text: "{"},
			{Top: 793, Left: 176, Width: 7, Height: 13, Font: 3, Text: "1"},
		},
		Rules: []pdfsrc.Rule{{Top: 796, Left: 154, Width: 12, Thickness: 1.457, Length: 7.75, Size: 15.07}},
	}
	out, placed, lost := Minus(lay, p)
	if placed != 1 || lost != 0 {
		t.Fatalf("placed %d lost %d, want 1 and 0", placed, lost)
	}
	got := out.Spans[len(out.Spans)-1]
	if got.Font != 3 || got.Top != 793 || got.Height != 13 {
		t.Errorf("the sign was set beside the superscript: %+v", got)
	}
}
