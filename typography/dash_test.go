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

// The mark is written where the reading dropped it altogether, which is the
// commoner half of the fault: thirty six heads against ten. There is no spacing
// to preserve in this case, so the printing's own is written.
func TestStatementDashWritesTheMarkThatIsNotThereAtAll(t *testing.T) {
	cases := []struct{ in, want string }{
		{"COROLLAIRE 1. Pour toute suite $M$-régulière, les propriétés suivantes.",
			"COROLLAIRE 1. — Pour toute suite $M$-régulière, les propriétés suivantes."},
		{"**Proposition 10.** *Soient $\\rho : A \\to B$ un homomorphisme local.*",
			"**Proposition 10.** — *Soient $\\rho : A \\to B$ un homomorphisme local.*"},
		{"Corollaire. Soit $G_+$ l'ensemble des éléments positifs de $G$.",
			"Corollaire. — Soit $G_+$ l'ensemble des éléments positifs de $G$."},
		{"Définition 1. Soient $A$ un anneau, $J$ un idéal de $A$.",
			"Définition 1. — Soient $A$ un anneau, $J$ un idéal de $A$."},
		{"PROPOSITION 6. Soient $A$ un anneau noethérien, $N$ un $A$-module.",
			"PROPOSITION 6. — Soient $A$ un anneau noethérien, $N$ un $A$-module."},
	}
	for _, c := range cases {
		got, n := StatementDash(c.in)
		if got != c.want || n != 1 {
			t.Errorf("StatementDash(%q) = %q, %d, want %q, 1", c.in, got, n, c.want)
		}
	}
}

// The mark is moved out of the emphasis where the reading closed it one
// character late. The three shapes are tried in the order that matters: a head
// carrying a hyphen inside the emphasis has to have the mark moved and not
// merely lengthened in place, or the line comes out repaired and still wrong.
func TestStatementDashMovesTheMarkOutOfTheEmphasis(t *testing.T) {
	cases := []struct{ in, want string }{
		{"**Corollaire 1.—** *Toute algèbre finie et plate sur un anneau de Macaulay.*",
			"**Corollaire 1.** — *Toute algèbre finie et plate sur un anneau de Macaulay.*"},
		{"**Proposition 10.-** *Soit $\\rho : A \\to B$ un homomorphisme.*",
			"**Proposition 10.** — *Soit $\\rho : A \\to B$ un homomorphisme.*"},
		{"**THÉORÈME 3. —** *Supposons les $A$-modules de type fini.*",
			"**THÉORÈME 3.** — *Supposons les $A$-modules de type fini.*"},
		{"**Remarque. —** On notera que deux modules dualisants sont isomorphes.",
			"**Remarque.** — On notera que deux modules dualisants sont isomorphes."},
	}
	for _, c := range cases {
		got, n := StatementDash(c.in)
		if got != c.want || n != 1 {
			t.Errorf("StatementDash(%q) = %q, %d, want %q, 1", c.in, got, n, c.want)
		}
	}
}

// A head alone on its line is left alone. Its statement begins on the line
// under it and there is nowhere on this line for the mark to go, so writing one
// would leave the line ending in a dash and the statement still unmarked.
func TestStatementDashLeavesAHeadThatStandsAlone(t *testing.T) {
	body := "PROPOSITION 8.\n\n$$\nA \\otimes_B C = 0\n$$"
	if got, n := StatementDash(body); n != 0 || got != body {
		t.Errorf("changed %d lines, want the head left as it stands", n)
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
