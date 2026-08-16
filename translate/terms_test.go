package translate

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/glossary"
)

func rows() *glossary.Glossary {
	return &glossary.Glossary{Version: 3, Terms: []glossary.Term{
		{EN: "square", VI: "bình phương"},
		{EN: "ring", VI: "vành"},
		{EN: "Hausdorff", VI: "Hausdorff"},
		{EN: "fortiori", VI: "a fortiori"},
	}}
}

// A chunk that leaves a glossary term in English is refused, and refused with
// the term in the complaint so the second ask can be about it.
//
// This is L10, which the audit already reports of the finished file. Reporting
// it there and not here is how "square" got into content/vi/ens/I/01: the file
// is written, the chunk's answer is cached, and nothing puts the question
// again.
func TestAChunkThatLeavesATermInEnglishIsRefused(t *testing.T) {
	const en = "The square of a ring element is defined."
	problems := AuditTerms("vi", rows(), en, "Bình phương của một phần tử vành được định nghĩa.")
	if len(problems) != 0 {
		t.Fatalf("a translation that used both terms was refused: %v", problems)
	}

	problems = AuditTerms("vi", rows(), en, "The square của một phần tử vành được định nghĩa.")
	if len(problems) != 1 {
		t.Fatalf("%d problems, want the one term left standing: %v", len(problems), problems)
	}
	if problems[0].Rule != RuleTerminology {
		t.Errorf("the rule is %q, want %q", problems[0].Rule, RuleTerminology)
	}
	for _, want := range []string{"square", "bình phương"} {
		if !strings.Contains(problems[0].Msg, want) {
			t.Errorf("the complaint reads %q and does not say %q, so the second ask cannot be about it",
				problems[0].Msg, want)
		}
	}
}

// A term inside a formula is not a term left in English.
//
// The translator is told to copy the mathematics character for character, so a
// word inside dollar signs is a word it was ordered to keep. Reading the answer
// without taking the mathematics out would refuse every correct translation of
// a line like this one and the chunk would die after three asks it could never
// have passed.
func TestAWordInsideTheMathematicsIsNotATermLeftInEnglish(t *testing.T) {
	const en = "Write $\\text{square}(x)$ for the square of $x$."
	const tr = "Viết $\\text{square}(x)$ cho bình phương của $x$."
	if problems := AuditTerms("vi", rows(), en, tr); len(problems) != 0 {
		t.Errorf("the copied formula was read as prose: %v", problems)
	}

	// A display opened and closed on one line is the common case in this
	// corpus, and it is where the rule went wrong first: $$ at each end
	// counted as two dollars, the switch flipped twice, and the formula was
	// read as prose. \square then answered for the English word square and
	// \left and \right for left and right, on ten of the twelve findings L10
	// had against the Vietnamese of Theory of Sets.
	const oneLine = "Form\n\n$$\\overline{\\tau \\vee \\neg \\in \\square} A'$$\n\nfrom the square of $x$."
	const oneLineVI = "Tạo\n\n$$\\overline{\\tau \\vee \\neg \\in \\square} A'$$\n\ntừ bình phương của $x$."
	if problems := AuditTerms("vi", rows(), oneLine, oneLineVI); len(problems) != 0 {
		t.Errorf("a display written on one line was read as prose: %v", problems)
	}

	// And a display block goes the same way, whole.
	const display = "Consider\n\n$$\n\\text{square}(x) = x^2\n$$\n\nfor every $x$."
	const displayVI = "Xét\n\n$$\n\\text{square}(x) = x^2\n$$\n\nvới mọi $x$."
	if problems := AuditTerms("vi", rows(), display, displayVI); len(problems) != 0 {
		t.Errorf("the display block was read as prose: %v", problems)
	}
}

// A name the two languages spell the same way is not a finding, and a term the
// English never used is not one either.
func TestOnlyATermThatWasAskedForIsReported(t *testing.T) {
	if problems := AuditTerms("vi", rows(), "A Hausdorff space.", "Một không gian Hausdorff."); len(problems) != 0 {
		t.Errorf("a term that is the same word in both languages was refused: %v", problems)
	}
	// "ring" is in the answer and was never in the English, which is a
	// different fault and not this one to report.
	if problems := AuditTerms("vi", rows(), "A square.", "Một bình phương của ring."); len(problems) != 0 {
		t.Errorf("a term the English never used was reported: %v", problems)
	}
	// A rendering that holds the English word inside it proves nothing when
	// the English word turns up in the answer. The glossary writes fortiori as
	// "a fortiori", so every correct Vietnamese sentence about it has the word
	// fortiori in it, and those were the last five false positives on the 769
	// answers this was measured against.
	if problems := AuditTerms("vi", rows(), "A fortiori it holds.", "A fortiori điều đó đúng."); len(problems) != 0 {
		t.Errorf("a rendering that holds the English word was refused: %v", problems)
	}
	// No glossary at all is no complaint, which is where Chinese and Japanese
	// are today.
	if problems := AuditTerms("vi", nil, "A square.", "A square."); len(problems) != 0 {
		t.Errorf("a run with no glossary reported %v", problems)
	}
}
