package textguard

import (
	"strings"
	"testing"
)

func TestStarsPutsTheCorpusStarBackForEveryOrnament(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
		n    int
	}{
		{
			"an asterisk operator closing a passage",
			`of the field of scalars to $\mathbf{R}$"). ∗`,
			`of the field of scalars to $\mathbf{R}$"). \*`,
			1,
		},
		{
			"an asterisk operator opening one with no space after it",
			`richer than the species of order structures. ∗The species of commutative`,
			`richer than the species of order structures. \*The species of commutative`,
			1,
		},
		{
			"a teardrop spoked asterisk",
			`contains a cofinal subset isomorphic to E). ✻`,
			`contains a cofinal subset isomorphic to E). \*`,
			1,
		},
		{
			"an eight spoked asterisk",
			`$\varphi_\mathrm{E}$ is not injective. ✳`,
			`$\varphi_\mathrm{E}$ is not injective. \*`,
			1,
		},
		{
			"a low asterisk closing what an escaped one opened",
			`\* The name "power of the continuum" arises. ⁎ The hypothesis`,
			`\* The name "power of the continuum" arises. \* The hypothesis`,
			1,
		},
		{
			"a pair on one line",
			`no least element). ✻ Give an example. ✻`,
			`no least element). \* Give an example. \*`,
			2,
		},
		{
			"one at the head of a line, where a bare asterisk would open a list",
			"∗ (4) When each of $\\Sigma$ and $\\Theta$",
			"\\* (4) When each of $\\Sigma$ and $\\Theta$",
			1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, n := Stars(tc.body)
			if got != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
			if n != tc.n {
				t.Errorf("counted %d, want %d", n, tc.n)
			}
		})
	}
}

func TestStarsLeavesTheMathematicsAlone(t *testing.T) {
	for _, tc := range []string{
		`the law $x ∗ y$ of the magma`,
		`the dual $E^{∗}$ of a vector space`,
		`$$f ∗ g = \int f(t) g(x - t)\,dt$$`,
		`already written as \* by hand`,
		`no star here at all`,
	} {
		t.Run(tc, func(t *testing.T) {
			got, n := Stars(tc)
			if got != tc {
				t.Errorf("got %q, want it unchanged", got)
			}
			if n != 0 {
				t.Errorf("counted %d, want 0", n)
			}
		})
	}
}

// A span that never closes runs to the end of the file, so everything after it
// reads as mathematics and nothing there is touched. M01 reports the file and
// somebody reads the printed page; guessing where the span meant to stop would
// put a star in the middle of a formula.
func TestStarsLeavesTheTailOfAnUnclosedSpanAlone(t *testing.T) {
	body := "an opening $x + y that never closes. ∗"
	got, n := Stars(body)
	if got != body {
		t.Errorf("got %q, want it unchanged", got)
	}
	if n != 0 {
		t.Errorf("counted %d, want 0", n)
	}
}

func TestStarsKeepsItsPlaceInAFrenchSentence(t *testing.T) {
	body := "les théorèmes de « la théorie » sont démontrés plus loin. ✻"
	want := `les théorèmes de « la théorie » sont démontrés plus loin. \*`
	got, n := Stars(body)
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if n != 1 {
		t.Errorf("counted %d, want 1", n)
	}
}

func TestOrnamentsReportsTheLineAndNamesTheGlyph(t *testing.T) {
	body := strings.Join([]string{
		"the law $x ∗ y$ of the magma, which is no ornament",
		"of the field of scalars. ∗",
		"nothing on this line",
		"no least element). ✻ Give an example. ✻",
	}, "\n")
	got := Ornaments(body)
	if len(got) != 2 {
		t.Fatalf("got %d ornaments, want 2: %+v", len(got), got)
	}
	if got[0].Line != 2 || got[0].Name != "an asterisk operator" {
		t.Errorf("first is %+v", got[0])
	}
	if got[1].Line != 4 || got[1].Name != "a teardrop spoked asterisk" {
		t.Errorf("second is %+v", got[1])
	}
	if !strings.Contains(got[1].Text, "Give an example") {
		t.Errorf("second carries the wrong line: %q", got[1].Text)
	}
}

// Normalise is the seam the OCR writes through, so a page read from now on
// needs nothing done to it.
func TestNormaliseWritesTheCorpusStar(t *testing.T) {
	got := Normalise("is not injective. ✳")
	if got != `is not injective. \*` {
		t.Errorf("got %q", got)
	}
}
