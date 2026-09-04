package translate

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/glossary"
)

func rows() *glossary.Glossary {
	return &glossary.Glossary{Version: 3, Terms: []glossary.Term{
		{EN: "square", VI: "bình phương"},
		{EN: "calculus", VI: "phép tính"},
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

// A name in quotation marks that the book prints in Latin is kept, and keeping
// it is not a term left in English.
//
// This is chunk 4 of the historical note of chapter IV, where Leibniz calls his
// logic a "Calculus ratiocinator". Every model that was asked for the paragraph
// kept the name, which is what a reader wants, and the terminology rule read the
// word calculus in it and refused the chunk over "phép tính".
func TestALatinNameInQuotationMarksIsNotATermLeftInEnglish(t *testing.T) {
	const en = `he called it a "Calculus ratiocinator", and the square of it.`
	const tr = `ông gọi nó là một “Calculus ratiocinator”, và bình phương của nó.`
	if problems := AuditTerms("vi", rows(), en, tr); len(problems) != 0 {
		t.Errorf("a Latin name the book prints in quotation marks was refused: %v", problems)
	}

	// The term outside the quotation is still read, so the exclusion covers the
	// name and nothing around it.
	const left = `ông gọi nó là một “Calculus ratiocinator”, và the square của nó.`
	if problems := AuditTerms("vi", rows(), en, left); len(problems) != 1 {
		t.Errorf("%d problems, want the term outside the quotation: %v", len(problems), problems)
	}

	// An English sentence in quotation marks is English prose, whoever it is
	// quoted from, and a model that copies it rather than rendering it is
	// reported as before.
	const said = `he wrote that "the square of every element is the same", and said so twice.`
	const copied = `ông viết rằng “the square of every element is the same”, và nói vậy hai lần.`
	if problems := AuditTerms("vi", rows(), said, copied); len(problems) != 1 {
		t.Errorf("%d problems, want the copied English quotation: %v", len(problems), problems)
	}

	// A quotation the English does not have is not a quotation from the book,
	// so it stands to be read like the rest of the answer.
	const own = `ông gọi nó là một “the calculus of it”, và bình phương của nó.`
	if problems := AuditTerms("vi", rows(), en, own); len(problems) != 1 {
		t.Errorf("%d problems, want the quotation the English never printed: %v", len(problems), problems)
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

// A display whose fences are welded to the prose either side of them is still a
// display.
//
// § 3 of Topology IV, chunk 3, which the run refused five times over. The
// English fences its display on its own lines and the answer did not: it wrote
// the opening $$ at the end of the sentence that introduces the formula and the
// closing one at the start of the sentence that carries on from it, which is
// perfectly good Markdown and reads the same way in a browser. The middle line
// then has no dollar on it at all, so nothing took \left and \right out, the
// glossary has a row for each of those two English words, and the rule asked for
// them to be translated inside a formula. No model was going to do that and none
// did.
func TestADisplayWeldedToTheProseIsStillADisplay(t *testing.T) {
	g := &glossary.Glossary{Version: 3, Terms: []glossary.Term{
		{EN: "left", VI: "trái"},
		{EN: "right", VI: "phải"},
		{EN: "uniformly continuous", VI: "liên tục đều"},
	}}
	const en = "We shall establish that $1/x$ is *uniformly continuous*. Precisely, we have $$\n" +
		"\\left| \\frac{1}{x} - \\frac{1}{y} \\right| = \\frac{|x-y|}{xy}\n" +
		"$$; there is an integer $m > 0$ such that $|x| \\geq 1/m$."
	const vi = "Chúng ta sẽ thiết lập rằng $1/x$ là *liên tục đều*. Cụ thể, ta có $$\n" +
		"\\left| \\frac{1}{x} - \\frac{1}{y} \\right| = \\frac{|x-y|}{xy}\n" +
		"$$; tồn tại một số nguyên $m > 0$ sao cho $|x| \\geq 1/m$."
	if problems := AuditTerms("vi", g, en, vi); len(problems) != 0 {
		t.Errorf("the welded display was read as prose: %v", problems)
	}
	// And the rule still works on the prose around it. The term that is really
	// there in English is still reported.
	const kept = "Chúng ta sẽ thiết lập rằng $1/x$ is uniformly continuous. Cụ thể, ta có $$\n" +
		"\\left| \\frac{1}{x} - \\frac{1}{y} \\right| = \\frac{|x-y|}{xy}\n" +
		"$$; tồn tại một số nguyên $m > 0$ sao cho $|x| \\geq 1/m$."
	if problems := AuditTerms("vi", g, en, kept); len(problems) != 1 {
		t.Errorf("%d problems, want the one term left standing: %v", len(problems), problems)
	}
}

// citing is the rows plus the term whose only occurrence in the file that
// prompted this was inside the name of a cited paper.
func citing() *glossary.Glossary {
	return &glossary.Glossary{Version: 3, Terms: []glossary.Term{
		{EN: "space", VI: "không gian"},
		{EN: "ring", VI: "vành"},
	}}
}

// The title of a cited work stands as printed, so the English words in it are
// not English left in the answer.
//
// Exercise 15 of § 2 of Topological Vector Spaces I is one line of prose and one
// footnote, and the footnote is a citation. The word space is in the file once,
// inside "The space of p-adic norms", which is the name of a paper and is not
// translated by anybody. The rule asked for "không gian" in it, the chunk was
// refused all three times it was asked for, and the file was one of fifteen the
// run could not land.
func TestATermInsideTheTitleOfACitedPaperIsNotATermLeftInEnglish(t *testing.T) {
	const en = "1 For the exercises 12 and 13, see O. Goldman and N. Iwahori, " +
		"The space of p-adic norms, Acta math., 109 (1963), pp. 137-177.\n"
	const vi = "1 Đối với các bài tập 12 và 13, xem O. Goldman và N. Iwahori, " +
		"The space of p-adic norms, Acta math., 109 (1963), pp. 137-177.\n"
	if problems := AuditTerms("vi", citing(), en, vi); len(problems) != 0 {
		t.Fatalf("a citation kept as printed was refused: %v", problems)
	}
}

// The bracket-numbered references of the historical notes have the same fault
// for the same reason, and there are 180 of those lines against 88 the older
// rule already covered.
func TestATermInsideABracketNumberedReferenceIsNotATermLeftInEnglish(t *testing.T) {
	const en = "[4] E. Heine, On the space of trigonometric series, Crelle's Journal, 71 (1870), pp. 353-365.\n"
	if problems := AuditTerms("vi", citing(), en, en); len(problems) != 0 {
		t.Fatalf("a reference kept as printed was refused: %v", problems)
	}
}

// The guard on the above. A reference is read by its tail, and no running
// sentence has one, so an ordinary line that leaves a term in English is
// refused exactly as it was before.
func TestOrdinaryProseIsStillReadForTermsLeftInEnglish(t *testing.T) {
	const en = "Every vector space over a ring is considered.\n"
	const vi = "Mọi vector space trên một vành đều được xét.\n"
	problems := AuditTerms("vi", citing(), en, vi)
	if len(problems) != 1 {
		t.Fatalf("%d problems, want the one term left standing: %v", len(problems), problems)
	}
	if !strings.Contains(problems[0].Msg, "space") {
		t.Errorf("the complaint reads %q and does not say %q", problems[0].Msg, "space")
	}
}

// A TeX control word is not the English word its letters spell.
//
// An index of notation writes its entries as bare LaTeX with no dollar signs
// anywhere, so mathtex.Strip has nothing to take out and the line arrives whole.
// The glossary then read sum in \sum and asked for "tổng" in a list of symbols.
func TestATeXControlWordIsNotATermLeftInEnglish(t *testing.T) {
	g := &glossary.Glossary{Version: 3, Terms: []glossary.Term{
		{EN: "sum", VI: "tổng"},
		{EN: "map", VI: "ánh xạ"},
	}}
	const en = `\sum_{i=p}^q \sum_{j=r}^s x_{ij}, \prod_{i<j} x_{ij} : \text{I, § 1, no. 5}.` + "\n"
	if problems := AuditTerms("vi", g, en, en); len(problems) != 0 {
		t.Fatalf("an index of notation entry was read as prose: %v", problems)
	}
}

// The guard. Taking the control words out must not take the prose with them:
// a line that really does leave a term in English is refused as before.
func TestTakingOutControlWordsLeavesTheProseBehind(t *testing.T) {
	g := &glossary.Glossary{Version: 3, Terms: []glossary.Term{{EN: "sum", VI: "tổng"}}}
	const en = "The sum of the family is written \\sum x_i here.\n"
	const vi = "The sum của họ được viết \\sum x_i ở đây.\n"
	problems := AuditTerms("vi", g, en, vi)
	if len(problems) != 1 {
		t.Fatalf("%d problems, want the one term left standing: %v", len(problems), problems)
	}
}
