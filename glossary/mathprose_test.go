package glossary

import "testing"

// The line of chapter I this was written for. The words move and nothing else
// does, which is what a translated formula looks like.
func TestAFormulaWhoseWordsWereTranslatedIsTheSameFormula(t *testing.T) {
	en := `((\text{not } A) \text{ or } B) \Rightarrow ((\text{not not } A) \text{ or } B)`
	vi := `((\text{không } A) \text{ hoặc } B) \Rightarrow ((\text{không không } A) \text{ hoặc } B)`
	if !SameMath(en, vi) {
		t.Error("the translated formula was read as a different formula")
	}
	if got := UntranslatedMathProse(en, vi); len(got) != 0 {
		t.Errorf("reported %v as left in English", got)
	}
}

// The failure this is for: the model treated the formula as mathematics and
// copied the words through with the symbols.
func TestAFormulaCopiedThroughIsReported(t *testing.T) {
	en := `(\text{not } A) \text{ or } B`
	if got := UntranslatedMathProse(en, en); len(got) != 2 {
		t.Errorf("reported %v, want both runs", got)
	}
	if !SameMath(en, en) {
		t.Error("a span copied through is not a span that changed")
	}
}

// Formula (5) of § 7 of chapter III, which is where whenever came from. The
// word is prose and a reader of the Vietnamese has to be able to read it, so
// the condition on the limit may be translated and stands reported if it is
// not.
func TestTheConditionOnALimitIsProse(t *testing.T) {
	en := `f_{\alpha\beta} \circ u_\beta = u_\alpha \qquad \textit{whenever } \alpha \leqslant \beta`
	vi := `f_{\alpha\beta} \circ u_\beta = u_\alpha \qquad \textit{mỗi khi } \alpha \leqslant \beta`
	if !SameMath(en, vi) {
		t.Error("the translated condition was read as a different formula")
	}
	if got := UntranslatedMathProse(en, vi); len(got) != 0 {
		t.Errorf("reported %v as left in English", got)
	}
	if got := UntranslatedMathProse(en, en); len(got) != 1 {
		t.Errorf("reported %v, want the run copied through", got)
	}
}

// A variable renamed inside a formula is still refused, which is the whole
// reason the mathematics is compared at all.
func TestASymbolThatMovedIsStillRefused(t *testing.T) {
	en := `(\text{not } A) \text{ or } B`
	vi := `(\text{không } A) \text{ hoặc } C`
	if SameMath(en, vi) {
		t.Error("a renamed variable came through the comparison")
	}
}

// A run that is not prose is a name the printing sets upright, and it has to
// come back as it went. Card is the one the corpus has, five times in chapter
// III, and no glossary rendering may be pushed into it.
func TestANameSetUprightMayNotBeTranslated(t *testing.T) {
	en := `\text{Card } A`
	if SameMath(en, `\text{Lực lượng } A`) {
		t.Error("a name that is not prose was allowed to change")
	}
	if got := UntranslatedMathProse(en, en); len(got) != 0 {
		t.Errorf("reported %v, and there is no prose in that span", got)
	}
}

// A word deleted rather than translated leaves a formula that sets and says
// less than the book does, and the mask cannot see it.
func TestAWordDeletedCountsAsUntranslated(t *testing.T) {
	en := `A \text{ and } B`
	vi := `A \text{} B`
	if got := UntranslatedMathProse(en, vi); len(got) != 1 {
		t.Errorf("reported %v, want the emptied run", got)
	}
}

// Nothing in Algebra changes. Its spans hold no words at all, so the two
// comparisons are the byte for byte one they always were.
func TestASpanWithNoWordsIsComparedAsItAlwaysWas(t *testing.T) {
	en := `\sum_{i\in I} x_i^2`
	if !SameMath(en, en) {
		t.Error("a span refused itself")
	}
	if SameMath(en, `\sum_{i\in J} x_i^2`) {
		t.Error("two different spans were read as one")
	}
}
