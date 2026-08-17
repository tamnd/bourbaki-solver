package translate

import "testing"

// A page number written the Vietnamese way is put back to the way the English
// prints it, because a citation is an address.
//
// This is chunk 4 of the historical note of chapter IV, refused eight times over
// four passes for tr. 185 and nothing else, each ask costing five minutes.
func TestAPageNumberWrittenInVietnameseIsPutBack(t *testing.T) {
	const en = "all true propositions ([12 b], vol. VII, p. 185; cf. [12 bis], p. 22)."
	const tr = "mọi mệnh đề đúng ([12 b], tập VII, tr. 185; cf. [12 bis], trang 22)."
	const want = "mọi mệnh đề đúng ([12 b], tập VII, p. 185; cf. [12 bis], p. 22)."
	if got := Readdress("vi", en, tr); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if problems := Audit("vi", en, Readdress("vi", en, tr)); len(problems) != 0 {
		t.Errorf("the repaired answer still has %v", problems)
	}
	// And it is the citation that moves and not the sentence: vol. VII stays as
	// the model wrote it, because RuleReference does not read it and this file
	// touches nothing the rule does not refuse.
}

// A number the English does not cite that way is not touched, so a page number
// the translation invented is refused as it was before.
func TestANumberTheEnglishNeverCitedStandsAsTheModelWroteIt(t *testing.T) {
	const en = "the empty concept, p. 377."
	const tr = "khái niệm rỗng, tr. 377, và trang 12 nữa."
	const want = "khái niệm rỗng, p. 377, và trang 12 nữa."
	if got := Readdress("vi", en, tr); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// The other abbreviation the rule reads is No., and it goes the same way.
func TestASectionNumberWrittenInVietnameseIsPutBack(t *testing.T) {
	const en = "see III, § 2, No. 4."
	const tr = "xem III, § 2, số 4."
	if got, want := Readdress("vi", en, tr), "xem III, § 2, No. 4."; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// An answer that is missing nothing is left alone, however many of the words
// are in it.
//
// Exercise 9 of chapter IV, § 2 is where this was measured. The English cites
// "No. 4" once and writes "no. 4" twice more in running prose, which the rule
// does not read, and the Vietnamese has the citation and writes the other two
// as "số 4". A repair led by the words rewrote both of those and handed the rule
// two citations where the English has one; a repair led by the rule leaves the
// file exactly as it stands, and it is the only one of the 240 written files
// that this was ever a question about.
func TestAnAnswerThatIsMissingNothingIsNotRepaired(t *testing.T) {
	const en = "in the sense of Example 2 of No. 4, and no. 4 of section 1 again, and no. 4 once more."
	const tr = "theo nghĩa của Ví dụ 2 của No. 4, và số 4 của mục 1, và số 4 lần nữa."
	if got := Readdress("vi", en, tr); got != tr {
		t.Errorf("an answer with the citation already in it was rewritten to %q", got)
	}
}

// And where two are missing, two go back and no more.
func TestAsManyGoBackAsAreMissing(t *testing.T) {
	const en = "p. 13 and p. 13 again, and p. 439."
	const tr = "tr. 13 và tr. 13 nữa, và p. 439."
	if got, want := Readdress("vi", en, tr), "p. 13 và p. 13 nữa, và p. 439."; got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}

// A language nothing is listed for is left alone, which is where Chinese and
// Japanese are, and an answer that already writes the address the English way is
// handed back the same string.
func TestARepairWithNothingToRepairChangesNothing(t *testing.T) {
	const en = "all true propositions, p. 185."
	const tr = "mọi mệnh đề đúng, p. 185."
	if got := Readdress("vi", en, tr); got != tr {
		t.Errorf("a correct citation was rewritten to %q", got)
	}
	if got := Readdress("zh", en, "所有真命题，tr. 185。"); got != "所有真命题，tr. 185。" {
		t.Errorf("a language with no words listed was rewritten to %q", got)
	}
}
