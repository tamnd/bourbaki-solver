package quality

import (
	"strings"
	"testing"
)

func TestP04(t *testing.T) {
	// Written the way the chapter is written, and one span that is not. The
	// broken one is the shape the corpus actually has 57 times: a prime that
	// came off its letter and landed after the subscript, leaving the subscript
	// with nothing under it.
	body := "The module $\\mathscr{H}(M)$ is semisimple, and $\\mathscr{H}(M_')$ is not."
	got := run(t, p04, doc("a.md", body))
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(got), got)
	}
	if got[0].Line != 1 {
		t.Errorf("the finding is on line %d", got[0].Line)
	}
	// KaTeX's own message, which names what it stopped at. A rule that said
	// only "will not parse" would send the reader back to the file to find out
	// what is wrong with a line they already cannot read.
	if !strings.Contains(got[0].Msg, "Expected group after") {
		t.Errorf("the finding does not say what is wrong: %s", got[0].Msg)
	}
	if !strings.Contains(got[0].Msg, `M_'`) {
		t.Errorf("the finding does not carry the span: %s", got[0].Msg)
	}
}

// The rule reads what the site will set, so a display is checked in display
// mode, and mathematics that is fine is left alone. The second half matters
// more than it looks: a rule with false positives over 20 thousand spans is a
// rule somebody turns off.
func TestP04LeavesGoodMathematicsAlone(t *testing.T) {
	body := strings.Join([]string{
		`Let $x^*\otimes y$ be the image of $x^*\in E^*$ under $\operatorname{End}_A(M)$.`,
		"",
		"$$",
		`\sum_{i\in I}\mathfrak{m}_i = \prod_{j=1}^{n}\mathbf{M}_{n_j}(D_j)`,
		"$$",
		"",
		`And $\mathscr{A}_{\mathrm{red}}$, and $\Gamma \backslash G$.`,
	}, "\n")
	if got := run(t, p04, doc("a.md", body)); len(got) != 0 {
		t.Errorf("good mathematics was reported: %v", got)
	}
}
