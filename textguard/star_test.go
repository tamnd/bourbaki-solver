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

// The bare ASCII asterisk is the fifth spelling and the only one a reader sees,
// since one at the head of a line opens a bullet list. The test for it is the
// space, which is Bourbaki's own: To the Reader sets these passages "between two
// asterisks: * . . . *", and Markdown wants emphasis to open and close on a
// non-space, so the two agree.
func TestStarsPutsTheCorpusStarBackForABareAsterisk(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
		n    int
	}{
		{
			"a pair with the whole passage between them",
			`* (3) The set $\mathbf{R}$ of real numbers is totally ordered. *`,
			`\* (3) The set $\mathbf{R}$ of real numbers is totally ordered. \*`,
			2,
		},
		{
			"a pair inside a paragraph",
			`denoted by $A$. * For example, take a point $x$ on this line. * In order that`,
			`denoted by $A$. \* For example, take a point $x$ on this line. \* In order that`,
			2,
		},
		{
			"one closing at the end of a line",
			`défini dans X, p. 151). *`,
			`défini dans X, p. 151). \*`,
			1,
		},
		{
			"one opening an exercise",
			`* 29) On garde les notations de l’exercice précédent`,
			`\* 29) On garde les notations de l’exercice précédent`,
			1,
		},
		{
			"the sentence in To the Reader that describes the mark",
			`Such examples are always placed between two asterisks: * . . . *`,
			`Such examples are always placed between two asterisks: \* . . . \*`,
			2,
		},
		{
			"one on a line of its own",
			"*",
			`\*`,
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

// Emphasis and bold are asterisks too, and both are all over the Elements: the
// definitions and the axioms are set in italic and the statement heads are bold.
// Neither has a space where the star has one, which is the whole reason the
// space is the test.
func TestStarsLeavesEmphasisAndBoldAlone(t *testing.T) {
	for _, tc := range []string{
		`The *signs* of a mathematical theory are the following :`,
		`**Definition 1.** — An A-module M is said to be Artinian.`,
		`*Example.* *In the theory of sets, $\in$ is a relational sign.*`,
		`**Remark.** — A lattice is obviously both left and right directed.`,
		`a *term* and a *relation* in the same sentence`,
		`already written as \* by hand, with the backslash`,
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

// Running it twice has to come out the same as running it once, or every pass of
// bourbaki fix star over the corpus adds another backslash.
func TestStarsIsIdempotent(t *testing.T) {
	body := `* (3) The set $\mathbf{R}$ is totally ordered. * and $K^*$ is the units`
	once, n := Stars(body)
	if n != 2 {
		t.Fatalf("counted %d on the first pass, want 2", n)
	}
	twice, n := Stars(once)
	if n != 0 || twice != once {
		t.Errorf("a second pass counted %d and gave %q", n, twice)
	}
}

func TestStarsLeavesTheMathematicsAlone(t *testing.T) {
	for _, tc := range []string{
		`the law $x ∗ y$ of the magma`,
		`the dual $E^{∗}$ of a vector space`,
		`$$f ∗ g = \int f(t) g(x - t)\,dt$$`,
		`already written as \* by hand`,
		`no star here at all`,
		// The plain asterisk inside a span is a convolution, an adjoint, a dual
		// basis or the units of a ring, and it runs through the volumes in their
		// thousands. It is the reason the spans are read before anything else.
		`the group of units $K^*$ of a field`,
		`the convolution $\varepsilon_x * f$ of two measures`,
		`$$\langle (ga)^*, b\rangle = q(ga, b)$$`,
		`the adjoint $\mu^*$ and the lattice $\Lambda^*$`,
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
