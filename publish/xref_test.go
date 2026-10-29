package publish

import (
	"strings"
	"testing"
)

func TestASubsectionIsAnchoredByItsNumber(t *testing.T) {
	for words, want := range map[string]string{
		"3. Filters":       "fvr-i-s1-n3",
		"12 : Compactness": "fvr-i-s1-n12",
		"Filters":          "",
	} {
		if got := subsecID("fvr-i-s1", words); got != want {
			t.Errorf("subsecID(%q) = %q, want %q", words, got, want)
		}
	}
	if got := subsecID("", "3. Filters"); got != "" {
		t.Errorf("no § label gave %q", got)
	}
}

func TestACitationIsLinkedOutsideMathAndLinks(t *testing.T) {
	cases := []struct{ line, want string }{
		{"by Proposition 3, we have", "by [Proposition 3](#p3), we have"},
		{"$Proposition 3$ and Proposition 3", "$Proposition 3$ and [Proposition 3](#p3)"},
		{"[Proposition 3](x) and Proposition 3", "[Proposition 3](x) and [Proposition 3](#p3)"},
	}
	for _, c := range cases {
		got, ok := linkIn(c.line, "Proposition 3", "#p3")
		if !ok || got != c.want {
			t.Errorf("linkIn(%q) = %q, %v; want %q", c.line, got, ok, c.want)
		}
	}
	if _, ok := linkIn("nothing here", "Proposition 3", "#p3"); ok {
		t.Error("linked a citation that is not on the line")
	}
}

func TestASnippetWritesTheCommonerTeX(t *testing.T) {
	got := readable("Let $x \\in \\mathbb{R}$ and $\\alpha \\leq \\beta$.", 80)
	for _, want := range []string{"∈", "ℝ", "α", "≤", "β"} {
		if !strings.Contains(got, want) {
			t.Errorf("readable gave %q, missing %q", got, want)
		}
	}
	if long := readable(strings.Repeat("word ", 50), 20); !strings.HasSuffix(long, "…") {
		t.Errorf("a long snippet is not cut: %q", long)
	}
}

func TestADisplayFormulaIsLeftUnlinked(t *testing.T) {
	lines := []string{"By Lemma 6,", "$$", `x = y \tag{\text{Lemma 6}}`, "$$", "so Lemma 6 holds", "$$ a $$"}
	want := []bool{false, true, true, true, false, true}
	got := displayLines(lines)
	for i := range want {
		if got[i] != want[i] {
			t.Errorf("line %d %q: in display %v, want %v", i, lines[i], got[i], want[i])
		}
	}
}

func TestASnippetWritesScriptsAndFractions(t *testing.T) {
	for tex, want := range map[string]string{
		`x_0`:               "x₀",
		`\mathbf{R}^3`:      "R³",
		`\frac{f(x)}{x}`:    "f(x)/x",
		`\frac{1}{x - x_0}`: "1/(x - x₀)",
		`x_{\alpha}`:        "x_α",
	} {
		if got := texText(tex); got != want {
			t.Errorf("texText(%q) = %q, want %q", tex, got, want)
		}
	}
}
