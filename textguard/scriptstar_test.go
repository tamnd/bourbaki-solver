package textguard

import "testing"

// The case the rule exists for, and the shape it comes in: one starred passage
// with both of its marks turned into scripts, from pages/lie-vii-ix/0291.md.
func TestScriptStarsReadsBothEndsOfAPassage(t *testing.T) {
	in := "$^*$(iv) G has a riemannian metric invariant under left and right translations.$_*$\n"
	want := "\\*(iv) G has a riemannian metric invariant under left and right translations.\\*\n"
	got, n := ScriptStars(in)
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if n != 2 {
		t.Errorf("it counted %d, want both marks", n)
	}
}

// The thing it must never reach. A subscript star on a base is a pushforward and
// a superscript star on a base is a dual, and the corpus runs on thousands of
// them.
func TestScriptStarsLeavesMathematicsWithABaseAlone(t *testing.T) {
	for _, in := range []string{
		"the map $f_*$ is induced by $f$\n",
		"the dual $K^*$ of $K$\n",
		"$(g \\circ f)_* = g_* \\circ f_*$\n",
		"$\\varphi_*$ and $(p_1)_*$\n",
		"$X^c_*$ is the trace\n",
	} {
		got, n := ScriptStars(in)
		if got != in || n != 0 {
			t.Errorf("%q was rewritten to %q (%d)", in, got, n)
		}
	}
}

// A bare superscript star glued to a letter or a bracket is a dual whose base is
// on the wrong side of the dollar. That is bourbaki fix math's fault to repair,
// and a rule that took it would turn S^* into a forward-reference mark.
func TestScriptStarsLeavesAStrandedBaseToFixMath(t *testing.T) {
	for _, in := range []string{
		"the injection of $k$[P] into S($\\mathfrak{h}$)$^*$\n",
		"let $G^*$ be an open subgroup and A$^*$ its dual\n",
		"the space E$^*$\n",
	} {
		got, n := ScriptStars(in)
		if got != in || n != 0 {
			t.Errorf("%q was rewritten to %q (%d)", in, got, n)
		}
	}
}

// What the model swallowed with the mark comes back out. A full stop and a
// closing bracket are prose and are written as prose.
func TestScriptStarsGivesBackThePunctuationItSwallowed(t *testing.T) {
	cases := map[string]string{
		"and use Remark 2 of no. $3.$)$_*$\n": "and use Remark 2 of no. $3.$)\\*\n",
		"an integral domain$._*$\n":           "an integral domain.\\*\n",
		"a maximal ideal of A$.)_*$\n":        "a maximal ideal of A.)\\*\n",
	}
	for in, want := range cases {
		got, n := ScriptStars(in)
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if n != 1 {
			t.Errorf("%q counted %d, want 1", in, n)
		}
	}
}

// A run of digits glued in is the number of a lemma or a no. and is prose. A
// single letter is a set or a group and wants the span the model took from it.
func TestScriptStarsTellsACitedNumberFromAGroup(t *testing.T) {
	cases := map[string]string{
		"this follows from Chap. III, §3, no. $13._*$\n": "this follows from Chap. III, §3, no. 13.\\*\n",
		"and Chap. V, §5, no. 5, Lemma $5._*$\n":         "and Chap. V, §5, no. 5, Lemma 5.\\*\n",
		"the closure is a subtorus of $G._*$\n":          "the closure is a subtorus of $G$.\\*\n",
		"$k$-points of $A._*$\n":                         "$k$-points of $A$.\\*\n",
		"by induction on $n.)_*$\n":                      "by induction on $n$.)\\*\n",
	}
	for in, want := range cases {
		got, n := ScriptStars(in)
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
		if n != 1 {
			t.Errorf("%q counted %d, want 1", in, n)
		}
	}
}

// A display block is never this. The mark is set in the prose and a $$ block
// holds a formula, so a script inside one is mathematics whatever it looks like.
func TestScriptStarsLeavesADisplayBlockAlone(t *testing.T) {
	in := "before\n\n$$\n_*\n$$\n\nafter\n"
	got, n := ScriptStars(in)
	if got != in || n != 0 {
		t.Errorf("a display block was rewritten to %q (%d)", got, n)
	}
}

// Stars is the one entry point the command and the audit both go through, so
// the script form has to come back out of it along with the four ornaments.
func TestStarsCarriesTheScriptFormToo(t *testing.T) {
	in := "the set is empty.$_*$ and ∗ here\n"
	got, n := Stars(in)
	want := "the set is empty.\\* and \\* here\n"
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if n != 2 {
		t.Errorf("it counted %d, want the script and the ornament", n)
	}
}

// The audit has to name the same lines the repair would rewrite, or the report
// says a file is clean that fix star is about to change.
func TestOrnamentsNamesTheScriptForm(t *testing.T) {
	in := "a line with nothing wrong\nthe set is empty.$_*$\n"
	got := Ornaments(in)
	if len(got) != 1 {
		t.Fatalf("it found %d, want the one line", len(got))
	}
	if got[0].Line != 2 {
		t.Errorf("it names line %d, want 2", got[0].Line)
	}
	if got[0].Name == "" {
		t.Errorf("it named the line without saying what is on it")
	}
}

// A bracket swept into the span along with the mark is prose when the span has
// no opening one to pair it with, and is the map's own when it has. Commutative
// Algebra V, chapter V, exercise 12 of section 1 is the first; the pushforwards
// scattered through Topology and Differentiable Varieties are the second.
func TestScriptStarsTellsAnAsideApartFromAPushforward(t *testing.T) {
	cases := map[string]string{
		"la fonction m\u00e9romorphe $1/(\\sin \\pi z))_*$\n": "la fonction m\u00e9romorphe $1/(\\sin \\pi z)$)\\*\n",
		"le morphisme $(p_1)_*$ est surjectif\n":              "le morphisme $(p_1)_*$ est surjectif\n",
		"on a $\\mathrm{Tor}_1^R(R/R_+, E)_*$\n":              "on a $\\mathrm{Tor}_1^R(R/R_+, E)_*$\n",
		"et $\\Gamma(g)_* \\circ \\delta(g)_*$\n":             "et $\\Gamma(g)_* \\circ \\delta(g)_*$\n",
	}
	for in, want := range cases {
		got, n := ScriptStars(in)
		if got != want {
			t.Errorf("ScriptStars(%q) = %q, want %q", in, got, want)
		}
		if wantN := 0; got != in {
			wantN = 1
			if n != wantN {
				t.Errorf("%q counted %d, want %d", in, n, wantN)
			}
		} else if n != 0 {
			t.Errorf("%q counted %d, want 0", in, n)
		}
	}
}

// A footnote reference sits between the end of the sentence and the mark, and
// stays in the mathematics where the volume put it. An exponent with no
// punctuation in front of it is an exponent and the star after it is a script
// with a base, so neither moves.
func TestScriptStarsKeepsAFootnoteAndGivesBackTheMark(t *testing.T) {
	cases := map[string]string{
		"the dual of the group $Q/(Q\\cap 2P).^{13}_*$\n": "the dual of the group $Q/(Q\\cap 2P).^{13}$\\*\n",
		"is isomorphic to $F.$)^{10}_*$\n":                "is isomorphic to $F.$)^{10}_*$\n",
		"and the exponent $X^{13}_*$ is a map\n":          "and the exponent $X^{13}_*$ is a map\n",
	}
	for in_, want := range cases {
		got, n := ScriptStars(in_)
		if got != want {
			t.Errorf("ScriptStars(%q) = %q, want %q", in_, got, want)
		}
		if want != in_ && n != 1 {
			t.Errorf("%q counted %d, want 1", in_, n)
		}
		if want == in_ && n != 0 {
			t.Errorf("%q counted %d, want 0", in_, n)
		}
	}
}
