package book

import (
	"strings"
	"testing"
)

// The corpus writes a great deal of its mathematics in prose with no dollars
// round it. x_i and S^{-1}A and C^r are all over the text layer, and before
// the rescue in control.go the underscore and the caret were escaped and the
// page printed them. These are the cases that were read off content/ while
// the rule was being written: the ones that must be picked up, and, which
// matters far more, the ones that must be left alone. A rule that is greedy in
// the wrong place turns a sentence into mathematics and the reader gets a page
// of italics where the prose was.

func TestAtomFindsMathematicsLeftLooseInProse(t *testing.T) {
	cases := []struct {
		in   string
		want string // the run atom should find at the first letter it can start on
	}{
		// The commonest shapes, in the order the census put them.
		{`S^{-1}`, `S^{-1}`},
		{`C^r`, `C^r`},
		{`x_i`, `x_i`},
		{`E_i`, `E_i`},
		{`x_0`, `x_0`},
		{`H_{i,j}`, `H_{i,j}`},
		// Multi-letter bases are operator names and they are mathematics too.
		{`Hom_A`, `Hom_A`},
		{`dim_K`, `dim_K`},
		{`Tor_1^A`, `Tor_1^A`},
		{`Ext^n_A`, `Ext^n_A`},
		// Greek is written as a character in the corpus rather than as a
		// control sequence, so the run has to admit it.
		{`Ω_K`, `Ω_K`},
		{`E_β`, `E_β`},
		// The run is greedy past the decoration on both sides on purpose.
		// S^{-1}A and C^nG and x_iy_j are single formulae in the source.
		{`S^{-1}A`, `S^{-1}A`},
		{`C^nG`, `C^nG`},
		{`x_iy_j`, `x_iy_j`},
		{`d_{L/K}x`, `d_{L/K}x`},
		// A bracket comes into the run only when a decoration hangs off it.
		{`(x_λ)_{λ ∈ Λ}`, `(x_λ)_{λ ∈ Λ}`},
		{`K[z_λ]_{λ ∈ Λ}`, `K[z_λ]_{λ ∈ Λ}`},
	}
	for _, c := range cases {
		rs := []rune(c.in)
		var got string
		for i := range rs {
			if end := atom(rs, i); end > i {
				got = string(rs[i:end])
				break
			}
		}
		if got != c.want {
			t.Errorf("atom over %q found %q, want %q", c.in, got, c.want)
		}
	}
}

// Nothing here is mathematics and the build must not touch any of it. The
// third and fourth cases are the ones that would do real damage: a rule that
// swallowed any parenthesis after a letter would eat the cross references,
// which the corpus writes thousands of times.
func TestAtomLeavesProseAlone(t *testing.T) {
	cases := []string{
		`Let E be a set and F a subset of it.`,
		`the topology of the space`,
		`(voir chap. II)`,
		`(see Chapter III, § 2, no. 4)`,
		`Proposition 5 of no. 3`,
		`A, B and C are three sets`,
		`E. Cartan and H. Weyl`,
		// An underscore that belongs to nothing stays an underscore. Markdown
		// emphasis in this corpus is written with asterisks, never with these.
		`a _ b`,
		`a ^ b`,
		`_ leading`,
	}
	for _, c := range cases {
		rs := []rune(c)
		for i := range rs {
			if end := atom(rs, i); end > i {
				t.Errorf("atom took %q out of %q", string(rs[i:end]), c)
				break
			}
		}
	}
}

// The one thing the rule gets wrong, written down so that it is a known limit
// and not a surprise. An identifier with an underscore in the middle of it,
// snake_case or a file name, has exactly the shape of Hom_A and there is no
// way to tell them apart from the characters alone. The corpus has none: the
// census over content/ read all 161 distinct multi-letter bases by hand and
// every one of them is an operator name. If a volume ever starts printing an
// identifier in italics, this is why, and the answer is to put the identifier
// in backticks in the source rather than to narrow the rule and lose Hom_A.
func TestAtomTakesAnIdentifierForMathematics(t *testing.T) {
	rs := []rune(`snake_case`)
	if end := atom(rs, 0); string(rs[0:end]) != `snake_case` {
		t.Errorf("atom found %q, and the comment above this test is now wrong",
			string(rs[0:end]))
	}
}

// The whole path, from a line of corpus prose to the LaTeX the volume sets.
// This is what the reader of the PDF is looking at.
func TestTeXWrapsLooseMathematicsInDollars(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`the module S^{-1}A over S^{-1}M`, `$S^{-1}A$`},
		{`f_p du K-module`, `$f_p$`},
		{`the family (x_λ)_{λ ∈ Λ}`, `$(x_\lambda)_{\lambda \in \Lambda}$`},
		{`the ring K[z_λ]_{λ ∈ Λ}`, `$K[z_\lambda]_{\lambda \in \Lambda}$`},
		// A control sequence in prose keeps its own decoration inside the same
		// pair of dollars. Without that the \alpha went into mathematics and
		// the _i was left outside it to be printed as an underscore.
		{`the root \alpha_i of the system`, `$\alpha_i$`},
		{`the module \mathbf{Z}^n`, `$\mathbf{Z}^n$`},
	}
	for _, c := range cases {
		out, err := Renderer{File: "x.md", Line: 1}.TeX(c.in)
		if err != nil {
			t.Fatalf("TeX(%q): %v", c.in, err)
		}
		if !strings.Contains(out, c.want) {
			t.Errorf("TeX(%q) = %q, want it to contain %q", c.in, out, c.want)
		}
	}
}

// Prose is not to come back with dollars in it. This is the check that would
// catch a widened rule the day somebody widens it.
func TestTeXLeavesProseWithoutDollars(t *testing.T) {
	cases := []string{
		`Let E be a set and F a subset of it (voir chap. II).`,
		`Proposition 5 of no. 3 applies to A, B and C.`,
	}
	for _, c := range cases {
		out, err := Renderer{File: "x.md", Line: 1}.TeX(c)
		if err != nil {
			t.Fatalf("TeX(%q): %v", c, err)
		}
		if strings.Contains(out, "$") {
			t.Errorf("TeX(%q) = %q, want no mathematics in it", c, out)
		}
	}
}

// The rescue reports itself. The count is the size of a job in content/ and
// the volume audit prints it, so a build that silently stopped counting would
// read as a corpus that had been repaired.
func TestRescuedIsReported(t *testing.T) {
	var got []string
	r := Renderer{File: "x.md", Line: 1}
	r.Rescued = func(_ string, atoms []string) { got = append(got, atoms...) }
	if _, err := r.TeX(`the module S^{-1}A and the root \alpha_i`); err != nil {
		t.Fatal(err)
	}
	want := []string{`S^{-1}A`, `\alpha_i`}
	if len(got) != len(want) {
		t.Fatalf("rescued %q, want %q", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("rescued[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// An index of two characters is one subscript, not two. The table maps each
// Unicode subscript on its own, so an OCR that wrote a_ij as aᵢⱼ came out of
// Math as a_i_j, which TeX reads as a double subscript and stops on. That one
// index in Algebra IX, § 10 was enough to leave the Vietnamese volume with no
// PDF, and the corpus has ninety five of these runs.
//
// The last three cases are the ones that must not change: a run of one is what
// the table already did, and the degree sign is a superscript in the table too
// but it sets \circ, which is not something to sweep into an index.
func TestMathCollapsesARunOfUnicodeScripts(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`aᵢⱼ`, `a_{ij}`},
		{`x₀₁`, `x_{01}`},
		{`Aᵢ₊₁`, `A_{i+1}`},
		{`Bₖ₋₁`, `B_{k-1}`},
		{`Cᵢₖ₋₁`, `C_{ik-1}`},
		{`x⁻ⁿ`, `x^{-n}`},
		{`xᵢ`, `x_i`},
		{`x²`, `x^2`},
		{`30°`, `30^\circ`},
	} {
		if got := Math(c.in); got != c.want {
			t.Errorf("Math(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// The rescue must never put dollars round something that has dollars in it
// already. mask has been over the line before this scanner sees it, so a math
// span the corpus wrote is a placeholder built from NUL by then, and a run that
// reached across one came back as $C($\rho$)^I$ after the placeholders went
// home. That is the error "Missing $ inserted" and a volume that stops on it.
// Five volumes in each of two languages were coming off the shelf with no PDF
// for the two lines this covers.
func TestTheRescueDoesNotReachAcrossAMathSpan(t *testing.T) {
	for _, s := range []string{
		`the injective homomorphism of E into C($\rho$)^I; for`,
		`hence that L($\Phi$)|_{B_{\lambda/2}} and L($\Phi$)^{-1}|_{B} converge`,
		`with at least one coefficient $\geq 2$ (we denote the root $a \alpha_1$ by $abcd$):`,
	} {
		out, err := Renderer{File: "x.md", Line: 1}.TeX(s)
		if err != nil {
			t.Fatalf("TeX(%q): %v", s, err)
		}
		if strings.Contains(out, "$$") {
			t.Errorf("TeX(%q) = %q, want no dollars inside dollars", s, out)
		}
	}
}
