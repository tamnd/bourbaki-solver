package translate

import "testing"

// The index of notation of Algebra VIII is the file these are about. It opens
// with $A_M$, every attempt came back with $A\_M$, and the run died on line 3
// of a file that is symbols the whole way down.

func TestAMathSpanTheModelEscapedAsMarkdownIsPutBack(t *testing.T) {
	en := `The ring $A_M$ and the module $M^*$.`
	tr := `Vành $A\_M$ và môđun $M^*$.`
	got := Unescape(en, tr)
	if got != `Vành $A_M$ và môđun $M^*$.` {
		t.Fatalf("the escape was not taken out: %q", got)
	}
	if ps := auditMath(en, got); len(ps) != 0 {
		t.Fatalf("the repaired answer still fails the math rule: %v", ps)
	}
}

func TestTheEscapeIsOnlyTakenOutWhereItMakesTheSpanTheEnglish(t *testing.T) {
	// The model escaped it and changed it as well. Dropping the backslash does
	// not make this the English, so nothing is touched and RuleMath refuses it,
	// which is the right answer: a repair that fires on a span the model
	// altered is a repair that launders a wrong formula.
	en := `The ring $A_M$.`
	tr := `Vành $B\_M$.`
	if got := Unescape(en, tr); got != tr {
		t.Fatalf("a changed span was rewritten: %q", got)
	}
}

func TestALiteralUnderscoreTheEnglishItselfWritesIsLeftAlone(t *testing.T) {
	en := `The name $\mathrm{f\_g}$.`
	tr := `Tên $\mathrm{f\_g}$.`
	if got := Unescape(en, tr); got != tr {
		t.Fatalf("an escape the English has was taken out: %q", got)
	}
}

func TestTheBracesAndLineEndsOfARealFormulaAreNotTouched(t *testing.T) {
	// \{ \} \\ and \& are 8251 occurrences over content/en and every one of
	// them is TeX doing its job. A repair that took a backslash off those
	// would be silent damage to a formula that was already right.
	en := `$\{x \mid x \in A\}$ and $a \\ b \& c$`
	tr := `$\{x \mid x \in A\}$ và $a \\ b \& c$`
	if got := Unescape(en, tr); got != tr {
		t.Fatalf("a real control sequence was unescaped: %q", got)
	}
}

func TestNothingIsRepairedWhenTheTwoSidesDoNotLineUp(t *testing.T) {
	// A dropped span is RuleMath's finding to report, and a repair that walked
	// the two lists out of step would put a formula in the wrong place.
	en := `$a$ and $b\_c$ and $d$`
	tr := `$a$ và $d$`
	if got := Unescape(en, tr); got != tr {
		t.Fatalf("a short answer was rewritten: %q", got)
	}
}

func TestAnAnswerWithNoEscapeInItComesBackUntouched(t *testing.T) {
	en := `The ring $A_M$.`
	tr := `Vành $A_M$.`
	if got := Unescape(en, tr); got != tr {
		t.Fatalf("an answer that needed nothing was rebuilt: %q", got)
	}
}
