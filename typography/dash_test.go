package typography

import "testing"

func TestStatementDashPutsBackTheMarkThePressSets(t *testing.T) {
	// The six heads of chapter X of Algebre commutative that came back short,
	// as the pages carry them.
	cases := []struct{ in, want string }{
		{"Proposition 7. - Soit $A$ un anneau noethérien.",
			"Proposition 7. — Soit $A$ un anneau noethérien."},
		{"Proposition 9. – Soient ρ : A → B un homomorphisme.",
			"Proposition 9. — Soient ρ : A → B un homomorphisme."},
		{"Proposition 1.- Soient A un anneau, M un A-module.",
			"Proposition 1.— Soient A un anneau, M un A-module."},
		{"COROLLAIRE 4. - Soient $A$ un anneau noethérien.",
			"COROLLAIRE 4. — Soient $A$ un anneau noethérien."},
		{"COROLLAIRE 2.- *Soient k un corps, B une k-algèbre.*",
			"COROLLAIRE 2.— *Soient k un corps, B une k-algèbre.*"},
		{"**Théorème 3.** - Soit $k$ un anneau.",
			"**Théorème 3.** — Soit $k$ un anneau."},
	}
	for _, c := range cases {
		got, n := StatementDash(c.in)
		if got != c.want || n != 1 {
			t.Errorf("StatementDash(%q) = %q, %d, want %q, 1", c.in, got, n, c.want)
		}
	}
}

// The mark the press sets is left alone, so a second run over a volume that has
// been through this changes nothing.
func TestStatementDashLeavesTheMarkThatIsAlreadyRight(t *testing.T) {
	body := "PROPOSITION 8.— Soit $A$ un anneau.\n\nCOROLLAIRE 1. — Soit $B$ une algèbre."
	got, n := StatementDash(body)
	if n != 0 || got != body {
		t.Errorf("StatementDash changed %d heads that were already right", n)
	}
}

// A hyphen anywhere but at the head of a statement is a hyphen. The corpus sets
// intervals and compound words with one and neither is a lost em dash.
func TestStatementDashLeavesTheProseAlone(t *testing.T) {
	body := "Soit $A$ un anneau noethérien - au sens de I, p. 3 - et $M$ un module.\n" +
		"\n" +
		"Les pages 7 - 9 traitent le cas macaulayen.\n" +
		"\n" +
		"Proposition 7. - Soit $A$ un anneau."
	got, n := StatementDash(body)
	if n != 1 {
		t.Fatalf("changed %d lines, want the one head", n)
	}
	if want := "Proposition 7. — Soit $A$ un anneau."; got[len(got)-len(want):] != want {
		t.Errorf("the head reads %q", got[len(got)-len(want):])
	}
}

// A head in the middle of a paragraph is not a head. The pages set a paragraph
// on one long line and a line that follows prose is the continuation of it.
func TestStatementDashWantsTheLineToOpenAParagraph(t *testing.T) {
	body := "Soit $A$ un anneau.\nProposition 7. - Soit $B$ une algèbre."
	if got, n := StatementDash(body); n != 0 || got != body {
		t.Errorf("changed %d lines in the middle of a paragraph", n)
	}
}
