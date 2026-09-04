package translate

import "testing"

// The index of notation of Integration I-VI is the file these are about. An
// entry of it sets the notations that share a definition as one span with a
// comma in it, the answer set each notation in its own dollars, and RuleMath
// counted 51 spans where the English has 33, fifteen times running.

func TestAnEntryTheAnswerBrokeIntoSeveralSpansIsPutBack(t *testing.T) {
	en := `Entry $\langle f, z \rangle, \langle z, f \rangle$ : I, p. 4`
	tr := `Mục $\langle f, z \rangle$, $\langle z, f \rangle$ : I, p. 4`
	got := Remerge(en, tr)
	if got != `Mục $\langle f, z \rangle, \langle z, f \rangle$ : I, p. 4` {
		t.Fatalf("the entry was not put back together: %q", got)
	}
	if ps := auditMath(en, got); len(ps) != 0 {
		t.Fatalf("the repaired answer still fails the math rule: %v", ps)
	}
}

func TestThreeSpansGoBackIntoOne(t *testing.T) {
	en := `$A, B, C$ and $D$`
	tr := `$A$, $B$, $C$ và $D$`
	if got := Remerge(en, tr); got != `$A, B, C$ và $D$` {
		t.Fatalf("three were not put back into one: %q", got)
	}
}

func TestTwoFormulaeTheEnglishKeepsApartAreNotWeldedTogether(t *testing.T) {
	// The whole risk of this repair. The English has two spans and the answer
	// has two, so there is nothing to merge, and merging them would make one
	// formula out of two that the book sets separately.
	en := `$A$ and $B$`
	tr := `$A$ và $B$`
	if got := Remerge(en, tr); got != tr {
		t.Fatalf("two separate formulae were welded: %q", got)
	}
}

func TestSpansThatDoNotAssembleIntoTheEnglishAreLeftAlone(t *testing.T) {
	// The model split the entry and changed it as well. The pieces do not come
	// to the English span, so nothing is touched and RuleMath refuses it, which
	// is the right answer: a repair that fires here launders a wrong formula.
	en := `$A, B$ and $C$`
	tr := `$A$, $X$ và $C$`
	if got := Remerge(en, tr); got != tr {
		t.Fatalf("a changed entry was rewritten: %q", got)
	}
}

func TestAnAnswerShortOfSpansIsNotThisFault(t *testing.T) {
	en := `$A$ and $B$ and $C$`
	tr := `$A$ và $C$`
	if got := Remerge(en, tr); got != tr {
		t.Fatalf("a short answer was rewritten: %q", got)
	}
}

func TestASpanTheAnswerInventedIsNotMergedAway(t *testing.T) {
	// One extra span past the last of the English. Consuming it would hide an
	// invented formula behind a repair.
	en := `$A$`
	tr := `$A$ và $B$`
	if got := Remerge(en, tr); got != tr {
		t.Fatalf("an invented span was swallowed: %q", got)
	}
}

func TestADisplayIsNeverMerged(t *testing.T) {
	en := "$$A, B$$"
	tr := "$$A$$, $$B$$"
	if got := Remerge(en, tr); got != tr {
		t.Fatalf("a display was merged: %q", got)
	}
}
