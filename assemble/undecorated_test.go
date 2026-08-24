package assemble

import "testing"

// The English printing sets a statement head in small capitals and a reading of
// the page image gives it back bold most of the time and undecorated the rest,
// with nothing on the page to say which of the two it will be. The grammar used
// to read the undecorated form for four kinds only, so a Proposition that lost
// its mark was prose and everything hanging off it moved to the statement above.
// Page 312 of Algebra I to III is the case that found this. See the note on
// enCapKinds.
func TestStatementsReadsAnUndecoratedHead(t *testing.T) {
	in := blocks(
		"### 1. Modules",
		"Proposition 6. Let E be a module and F a submodule of E.",
		"Corollary 1. The quotient E/F is a module.",
		"Theorem 2. — Every module over a field is free.",
	)
	_, got, err := statements(in, vii, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"alg-viii-s1-prop-6",
		"alg-viii-s1-prop-6-cor-1",
		"alg-viii-s1-thm-2",
	})
}

// What keeps the undecorated branch off ordinary prose is the period following
// the number with nothing between them. A citation writes the number into a
// reference and prose writes it into a sentence, and neither of those is this
// shape.
func TestStatementsLeavesACitationAlone(t *testing.T) {
	for _, line := range []string{
		"Proposition 2 of § 3 gives the result at once.",
		"This follows from (§ 3, no. 1, Proposition 1).",
		"Theorem 1 follows from this statement.",
		"Corollary 2 of Proposition 7 will not be used here.",
	} {
		if StatesAResult("en", line) {
			t.Errorf("StatesAResult(%q) = true, want false", line)
		}
	}
}

// StatesAResult is the grammar itself and not a copy of it, and the repair that
// parts a head from the display above it asks it rather than deciding for
// itself what a head looks like. See fixFence.
func TestStatesAResult(t *testing.T) {
	// Every one of these is the line a display was run together with on one of
	// the eight pages the repair names.
	heads := []struct{ lang, line string }{
		{"en", "**Proposition 7.** *The $ \\mathbf{Z} $-linear mapping (7) is bijective.*"},
		{"en", "PROPOSITION 1."},
		{"en", "**Corollary.** *If G is a finite group of order g and H is a subgroup.*"},
		{"en", "**Proposition 10.** — Let $ m \\geq 1 $. There exists a unique polynomial."},
		{"en", "**Remark 2.** Let $ (g, x) \\mapsto gx $ be a law of left operation."},
		{"fr", "**Corollaire.** — *Pour tout homomorphisme* $v : F \\to F'$ *de A-modules.*"},
		{"fr", "Remarques. — 1) Il résulte des formules (6) et (7) que S est sécante pour M."},
		{"fr", "*Exemple.* — Supposons que G soit le groupe de Lie défini par un module."},
	}
	for _, h := range heads {
		if !StatesAResult(h.lang, h.line) {
			t.Errorf("StatesAResult(%q, %q) = false, want true", h.lang, h.line)
		}
	}
	// A language this file does not describe has no heads, which is the honest
	// answer and not an error: the repair walks every page of the corpus and
	// two of the volumes are in a printing nothing here describes yet.
	if StatesAResult("vi", "**Proposition 7.** *Anything at all.*") {
		t.Error("StatesAResult() on a language with no grammar should be false")
	}
}
