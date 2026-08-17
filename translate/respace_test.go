package translate

import "testing"

// The three shapes the Vietnamese run of Theory of Sets came back with, each of
// which cost a question, a five minute wait and an answer thrown away.
func TestRespacePutsTheEnglishLayoutBack(t *testing.T) {
	for _, c := range []struct{ name, en, tr, want string }{
		{
			"a space put in around an operator",
			`Let $M\cap N$ be the intersection.`,
			`Cho $M \cap N$ là giao.`,
			`Cho $M\cap N$ là giao.`,
		},
		{
			"a display wrapped over lines",
			"Ta có\n\n$$A' \\times B' \\subset A \\times B.$$\n",
			"Ta có\n\n$$A' \\times B'\n\\subset A \\times B.$$\n",
			"Ta có\n\n$$A' \\times B' \\subset A \\times B.$$\n",
		},
		{
			"words inside the formula, translated, in a formula re-spaced around them",
			`This is $(\text{not } A)\text{ or } B$ here.`,
			`Đây là $(\text{không } A) \text{ or } B$ đây.`,
			`Đây là $(\text{không } A)\text{ or } B$ đây.`,
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := Respace(c.en, c.tr); got != c.want {
				t.Errorf("Respace gave\n%q\nwant\n%q", got, c.want)
			}
		})
	}
}

// Everything the repair must not touch. A formula that says something else is
// RuleMath's to refuse, and a repair that quietly put the English back over a
// changed formula would hide exactly the failure the rule is there to catch.
func TestRespaceLeavesAnythingElseAlone(t *testing.T) {
	for _, c := range []struct{ name, en, tr string }{
		{"a symbol that moved", `Let $M\cap N$ be.`, `Cho $M\cup N$ là.`},
		{"a formula that is gone", `Let $M\cap N$ be $x$.`, `Cho $x$ là.`},
		{"an inline span set as a display", `Let $M\cap N$ be.`, `Cho $$M \cap N$$ là.`},
		{"a formula copied exactly, which is the normal case", `Let $M\cap N$ be.`, `Cho $M\cap N$ là.`},
		{"prose respaced, which is not mathematics", `Let  $x$  be.`, `Cho $x$ là.`},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := Respace(c.en, c.tr); got != c.tr {
				t.Errorf("Respace changed the answer to %q", got)
			}
		})
	}
}

// The point of the whole thing: what came back is refused, and what the repair
// hands over is accepted, with nothing else about the answer moved.
func TestARespacedAnswerPassesTheAudit(t *testing.T) {
	en := "Let $A/\\mathfrak{m}$ be a field.\n"
	tr := "Cho $A / \\mathfrak{m}$ là một trường.\n"
	if ps := Audit("vi", en, tr); len(ps) == 0 {
		t.Fatal("the answer as it came back was accepted, so there is nothing to repair")
	}
	if ps := Audit("vi", en, Respace(en, tr)); len(ps) != 0 {
		t.Fatalf("the repaired answer is still refused: %v", ps)
	}
}
