package pdfsrc

import (
	"math"
	"strings"
	"testing"
)

// page is the shape pdftocairo writes: a viewBox in points, a <defs> holding one
// filled path per glyph the page uses, and the page's own drawing after it. The
// numbers are the ones a page of Algebra VIII prints, so that what the tests
// assert is what the volume does.
const page = `<?xml version="1.0" encoding="UTF-8"?>
<svg xmlns="http://www.w3.org/2000/svg" width="439.371pt" height="666.142pt" viewBox="0 0 439.371 666.142">
<defs>
<g>
<g id="glyph-0-0">
<path d="M 6.546875 -5.84375 L 6.546875 -6.109375 C 6.0625 -6.09375 5.9375 -6.09375 5.5 -6.09375 Z"/>
</g>
</g>
</defs>
<g fill="rgb(0%, 0%, 0%)" fill-opacity="1">
<use xlink:href="#glyph-0-0" x="53.41949" y="43.253737"/>
</g>
<path fill="none" stroke-width="0.944" stroke-linecap="butt" stroke="rgb(0%, 0%, 0%)" d="M 0.00184338 -0.00103772 L 4.289222 -0.00103772 " transform="matrix(0.99857, 0, 0, -0.99857, 199.013784, 79.94037)"/>
<path fill-rule="nonzero" fill="rgb(0%, 0%, 0%)" fill-opacity="1" d="M 361.632812 136.0625 L 369.386719 136.0625 L 369.386719 134.605469 L 361.632812 134.605469 Z M 361.632812 136.0625 "/>
<path fill="none" stroke-width="0.398" stroke-linecap="butt" stroke="rgb(0%, 0%, 0%)" d="M 0 0 L 21.66 0 " transform="matrix(1, 0, 0, -1, 100, 200)"/>
<path fill="none" stroke-width="0.944" stroke-linecap="butt" stroke="rgb(0%, 0%, 0%)" d="M 0 0 L 0 4.28 " transform="matrix(1, 0, 0, -1, 300, 400)"/>
<path fill-rule="nonzero" fill="rgb(0%, 0%, 0%)" d="M 10 10 C 12 12 14 14 16 16 Z"/>
</svg>
`

func TestParseSVG(t *testing.T) {
	rules, err := ParseSVG(strings.NewReader(page), 659)
	if err != nil {
		t.Fatal(err)
	}
	// Five paths are drawn and three of them are horizontal rules. The glyph in
	// <defs> is not one, because it is a glyph; the vertical segment is not one;
	// and the curve is not one.
	want := []Rule{
		{Top: 119, Left: 298, Width: 6, Thickness: 0.94265, Length: 4.28129},
		{Top: 202, Left: 542, Width: 12, Thickness: 1.45703, Length: 7.75391},
		{Top: 300, Left: 150, Width: 32, Thickness: 0.398, Length: 21.66},
	}
	if len(rules) != len(want) {
		t.Fatalf("got %d rules, want %d: %+v", len(rules), len(want), rules)
	}
	for i, w := range want {
		got := rules[i]
		if got.Top != w.Top || got.Left != w.Left || got.Width != w.Width {
			t.Errorf("rule %d box is %d %d %d, want %d %d %d",
				i, got.Top, got.Left, got.Width, w.Top, w.Left, w.Width)
		}
		if math.Abs(got.Thickness-w.Thickness) > 0.001 {
			t.Errorf("rule %d thickness is %.5f, want %.5f", i, got.Thickness, w.Thickness)
		}
		if math.Abs(got.Length-w.Length) > 0.001 {
			t.Errorf("rule %d length is %.5f, want %.5f", i, got.Length, w.Length)
		}
	}
}

// A page whose width is not asked for cannot be scaled, and a rule in the wrong
// units is worse than no rule at all, so nothing comes back.
func TestParseSVGNoWidth(t *testing.T) {
	rules, err := ParseSVG(strings.NewReader(page), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(rules) != 0 {
		t.Fatalf("got %d rules with no page width, want none", len(rules))
	}
}

func TestPoints(t *testing.T) {
	for _, tc := range []struct {
		in   string
		want int
		ok   bool
	}{
		{"M 1 2 L 3 4 ", 2, true},
		{"M 1 2 L 3 4 L 5 6 L 7 8 Z M 1 2 ", 5, true},
		{"M 1 2 C 3 4 5 6 7 8 Z", 0, false},
		{"M 1 2 L 3", 0, false},
		{"", 0, true},
	} {
		got, ok := points(tc.in)
		if ok != tc.ok {
			t.Errorf("points(%q) ok is %v, want %v", tc.in, ok, tc.ok)
			continue
		}
		if ok && len(got) != tc.want {
			t.Errorf("points(%q) gave %d points, want %d", tc.in, len(got), tc.want)
		}
	}
}

// A rotated or sheared transform is refused rather than approximated: a rule
// under one is not a horizontal line whatever the two coordinates in its data
// say about each other.
func TestMatrix(t *testing.T) {
	if _, _, _, _, ok := matrix("matrix(1, 0.5, 0, 1, 10, 20)"); ok {
		t.Error("a sheared transform was read")
	}
	if _, _, _, _, ok := matrix("translate(10, 20)"); ok {
		t.Error("a transform that is not a matrix was read")
	}
	a, d, e, f, ok := matrix("")
	if !ok || a != 1 || d != 1 || e != 0 || f != 0 {
		t.Errorf("no transform gave %v %v %v %v %v, want the identity", a, d, e, f, ok)
	}
}
