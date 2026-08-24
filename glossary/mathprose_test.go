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

// The display that killed § 21 of Algebra VIII. Both arms of one cases block
// are prose, and the rules have to want the same thing done to both of them.
//
// Before otherwise went on the list they wanted opposite things: if was prose
// and had to be translated, otherwise was read as a name and had to be copied,
// and the only answer that satisfied both was a cases block translated on one
// side and not the other. The chunk was asked six times and refused six times,
// three for translating otherwise and one for leaving if, and the file died at
// 100 chunks of 101 answered.
func TestBothArmsOfACasesBlockAreProse(t *testing.T) {
	en := `\iota (s'\otimes m)(s^{-1}) =\begin{cases} m & \text{if } s=s',\\ 0 & \text{otherwise.}\end{cases}`
	vi := `\iota (s'\otimes m)(s^{-1}) =\begin{cases} m & \text{nếu } s=s',\\ 0 & \text{nếu không.}\end{cases}`
	if !SameMath(en, vi) {
		t.Error("a cases block with both arms translated was refused as tampering")
	}
	if got := UntranslatedMathProse(en, vi); len(got) != 0 {
		t.Errorf("reported %v, and both arms were translated", got)
	}
	// And the other half, so that this does not pass by the rules having gone
	// quiet. An arm left in English is still an arm left in English.
	half := `\iota (s'\otimes m)(s^{-1}) =\begin{cases} m & \text{nếu } s=s',\\ 0 & \text{otherwise.}\end{cases}`
	if got := UntranslatedMathProse(en, half); len(got) != 1 {
		t.Errorf("reported %v, want the arm that stayed in English", got)
	}
}

// Exercise 9 of § 1 of Lie VIII, which is the same trap and had not been
// sprung only because nobody had asked for that file yet.
func TestTheParityWordsAreProse(t *testing.T) {
	en := `n\text{ odd}`
	if !SameMath(en, `n\text{ lẻ}`) {
		t.Error("a translated parity word was refused as tampering")
	}
	if got := UntranslatedMathProse(en, en); len(got) != 1 {
		t.Errorf("reported %v, want the run left in English", got)
	}
}

// The other side of the same measurement. What the corpus genuinely sets
// upright inside a formula stays off the list, and stays copied.
func TestTheNamesTheCorpusSetsUprightStayNames(t *testing.T) {
	for _, name := range []string{`\text{resp.}`, `\text{Card}`, `\text{i.e.,}`} {
		if SameMath(name, `\text{gì đó}`) {
			t.Errorf("%s was treated as prose and allowed to change", name)
		}
	}
}

// The display that killed § 7 of Lie chapter I. Ado's theorem sets a paragraph
// of its proof inside one formula, and among its runs are "be the kernel of",
// which the english list knows, and "generated by", which it does not. Read as a
// test of prose that list wanted the first translated and the second copied, in
// the same sentence of the same formula, and no model does that: the chunk was
// asked three times and refused three times and took the file with it.
func TestAParagraphSetInsideAFormulaIsAllProse(t *testing.T) {
	en := `\text{Let } I \subset U' \text{ be the kernel of } \rho' \text{. Let } S ` +
		`\text{ be the sub-} g \text{-module of } U'^* \text{ generated by } C(\rho').`
	vi := `\text{Đặt } I \subset U' \text{ là hạt nhân của } \rho' \text{. Đặt } S ` +
		`\text{ là } g \text{-module con của } U'^* \text{ sinh bởi } C(\rho').`
	if !SameMath(en, vi) {
		t.Error("a paragraph translated inside a formula was refused as tampering")
	}
	if got := UntranslatedMathProse(en, vi); len(got) != 0 {
		t.Errorf("reported %v, and the whole paragraph was translated", got)
	}
}

// in is not on the english list and never will be, because Vietnamese spells it
// the same way when it means to print. It is still prose, 37 times over in the
// English corpus, and a translation that writes trong is not tampering.
func TestAWordVietnameseSpellsTheSameWayIsStillProse(t *testing.T) {
	en := `x \text{ in } E`
	if !SameMath(en, `x \text{ trong } E`) {
		t.Error("a translated preposition was refused as tampering")
	}
	// Nothing requires it, though. The list is what says a run must move, and it
	// does not know this one, so the run may stand as printed.
	if got := UntranslatedMathProse(en, en); len(got) != 0 {
		t.Errorf("reported %v, and no word of that run is on the list", got)
	}
}

func TestWhatIsAName(t *testing.T) {
	for _, name := range []string{
		"Tr", "res", "sym", "dis", "resp.", "(resp.", "mod.", "Card", "tr . deg",
		"tr. deg", "Arc sin", "II", "III", "Ia", "Ib", "N", "i.e.,", ")", ")).",
		"---", "``", "''", "“", "«", "(22)", "(vii)", "", "  ",
	} {
		if !IsMathName(name) {
			t.Errorf("IsMathName(%q) = false, want a name", name)
		}
	}
	for _, prose := range []string{
		"in", "with", "to", "onto", "whence", "vertices", "an integer", "terms",
		"factors", "finite", "scalars", "compact", "with respect to",
		"rational integers", "generated by", "be the kernel of", "in E",
		"dimension 3:", "-module", "(l + 1 vertices)",
	} {
		if IsMathName(prose) {
			t.Errorf("IsMathName(%q) = true, want prose", prose)
		}
	}
}
