package translate

import "testing"

// The historical note of chapter IV cites Curry in the middle of a sentence,
// with the publisher in brackets of its own inside the citation.
const curry = "(H. CURRY, *Outlines of a Formalist Philosophy of Mathematics*, " +
	"Amsterdam (North Holland Publ. Co.), 1951, p. 57)"

// A work cited by name stands as printed, and the language rule does not read
// it. It is the bibliography's rule in the one place the bibliography does not
// reach: an entry gathered under a heading is recognised by its number, and this
// is the same thing written into a sentence.
func TestAWorkCitedByNameIsNotAnUntranslatedRun(t *testing.T) {
	en := `insisted that it is an "objective science" ` + curry + ", and said so twice."
	tr := `khẳng định rằng nó là một “khoa học khách quan” ` + curry + ", và nói vậy hai lần."
	if problems := Audit("vi", en, tr); len(problems) != 0 {
		t.Errorf("an answer that kept the citation as printed was refused: %v", problems)
	}
}

// A parenthesis of ordinary prose left in English is refused as it was, because
// nobody is named in it.
func TestAParenthesisOfEnglishProseIsStillARun(t *testing.T) {
	const en = "the set is open (see the remark which follows this proposition), and it is not closed."
	const tr = "tập hợp là mở (see the remark which follows this proposition), và nó không đóng."
	problems := Audit("vi", en, tr)
	if len(problems) != 1 || problems[0].Rule != RuleLanguage {
		t.Errorf("%d problems, want the English parenthesis: %v", len(problems), problems)
	}
}

// And a citation the English does not have word for word is not a citation out
// of the book, so it is read like the rest of the answer.
func TestACitationTheEnglishNeverPrintedIsStillRead(t *testing.T) {
	en := "insisted that it is an objective science " + curry + ", and said so twice."
	tr := "khẳng định rằng nó là một khoa học khách quan (H. CURRY, *Outlines of a Formalist " +
		"Philosophy of Mathematics*, Amsterdam, 1951, p. 57), và nói vậy hai lần."
	problems := Audit("vi", en, tr)
	if len(problems) == 0 {
		t.Error("a citation the English does not print was taken out of the reading")
	}
}

// A chapter is capitals and is not a name, so the citations the corpus is full
// of are untouched by this.
func TestAChapterNumberIsNotAName(t *testing.T) {
	if names("(VIII, p. 252, Theorem 1)") {
		t.Error("a chapter in roman numerals was read as somebody's name")
	}
	if !names(curry) {
		t.Error("a surname in capitals was not read as a name")
	}
}
