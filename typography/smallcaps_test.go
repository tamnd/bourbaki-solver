package typography

import "testing"

func TestAStatementHeadIsSetInCapitals(t *testing.T) {
	// Off pdf page 154 of Lie 1 to 3, the head of Theorem 1 of chapter II § 3.
	// The corollary under it is on the next page and is written COROLLARY 1, so
	// the volume is being read two ways on two facing pages, and the corollary
	// is the one that fails: it is numbered under the theorem and the theorem
	// in this case is not a statement at all.
	was := "Theorem 1. *Let* $ \\alpha : L(X) \\to A(X) $ *be the unique homomorphism*"
	want := "THEOREM 1. *Let* $ \\alpha : L(X) \\to A(X) $ *be the unique homomorphism*"
	got, changed, left := SmallCaps(was)
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if changed != 1 || left != 0 {
		t.Errorf("changed %d and left %d, want 1 and 0", changed, left)
	}
}

func TestEveryKindTheEnglishPrintingSetsInSmallCapitals(t *testing.T) {
	for _, c := range []struct{ was, want string }{
		{"Definition 1. *A Lie algebra*", "DEFINITION 1. *A Lie algebra*"},
		{"Definitions. *Two mappings*", "DEFINITIONS. *Two mappings*"},
		{"Proposition 12. *Let g be*", "PROPOSITION 12. *Let g be*"},
		{"Theorem 2. *Suppose that*", "THEOREM 2. *Suppose that*"},
		{"Corollary. *The module* $ L(X) $", "COROLLARY. *The module* $ L(X) $"},
		{"Corollary 3. *If g is*", "COROLLARY 3. *If g is*"},
		{"Corollaries. *The two assertions*", "COROLLARIES. *The two assertions*"},
		{"Theorem 1 (Wedderburn). *Let A be*", "THEOREM 1 (Wedderburn). *Let A be*"},
	} {
		got, changed, _ := SmallCaps(c.was)
		if got != c.want || changed != 1 {
			t.Errorf("%q became %q with %d changed, want %q and 1", c.was, got, changed, c.want)
		}
	}
}

func TestTheKindsAVolumeSetsInItalicAreLeftAlone(t *testing.T) {
	// Lie 7 to 9 sets Lemma, Remark, Example and Scholium in italic and the
	// assembler reads them as they are. Putting them in capitals would be
	// writing a printing that no volume of the corpus has.
	for _, was := range []string{
		"Lemma 2. For $ 1 \\leq r \\leq l $, the element",
		"Remark 3. The hypothesis is not needed here.",
		"Example 1. Take for E the field of rationals.",
		"Scholium. The proof gives more than the statement.",
		// A sentence that opens on the word and does not close the head with a
		// full stop is prose about a statement and not a head.
		"Corollaries 1 and 2 follow from the theorem.",
		"Proposition 3 of § 2 gives the other inclusion.",
	} {
		got, changed, _ := SmallCaps(was)
		if got != was || changed != 0 {
			t.Errorf("%q became %q, want it left alone", was, got)
		}
	}
}

func TestABoldHeadIsLeftAlone(t *testing.T) {
	// Algebra VIII sets its heads in bold, which is the page as printed.
	was := "**Proposition 6.** — Let $ A $ be a ring."
	got, changed, _ := SmallCaps(was)
	if got != was || changed != 0 {
		t.Errorf("%q became %q, want it left alone", was, got)
	}
}

func TestAHeadFollowedByADashIsCountedAndLeftAlone(t *testing.T) {
	// The bold is gone and the dash is still there, so the fault is the bold and
	// not the case. Capitalising the kind would leave the dash at the head of the
	// statement's body.
	was := "Proposition 6. — Let $ A $ be a ring."
	got, changed, left := SmallCaps(was)
	if got != was || changed != 0 {
		t.Errorf("%q became %q, want it left alone", was, got)
	}
	if left != 1 {
		t.Errorf("left %d, want 1", left)
	}
}

func TestOnlyTheHeadOfAParagraphIsLookedAt(t *testing.T) {
	// A page sets a paragraph on one line, so a line with text in front of it is
	// a line of a display or a paragraph the reading broke, and neither of them
	// opens a statement.
	was := "$$\nProposition 1. \\text{ is not a head here}\n$$"
	got, changed, _ := SmallCaps(was)
	if got != was || changed != 0 {
		t.Errorf("got %q with %d changed, want it left alone", got, changed)
	}
}

func TestTheHeadKeepsTheRestOfItsLine(t *testing.T) {
	was := "Proposition 1. *Let g be a Lie algebra.*\n\nThis follows from Lemma 2.\n"
	want := "PROPOSITION 1. *Let g be a Lie algebra.*\n\nThis follows from Lemma 2.\n"
	got, changed, _ := SmallCaps(was)
	if got != want || changed != 1 {
		t.Errorf("got %q with %d changed, want %q and 1", got, changed, want)
	}
}
