// Package repair asks the model to fix a page it has already read, and proves
// that what came back is a fix rather than a rewrite.
//
// The cheap way to deal with a page that failed validation is to read it again
// from the image. That costs a full call, about 151 seconds at best, and it
// throws away a transcription that was right everywhere except one place. The
// cheaper way is to stay in the same conversation, where the page image is
// still in context, and ask for the one thing that is wrong.
//
// The reason that is not obviously a good idea is the reason this package is
// mostly audit. A model asked to fix a formula will fix a formula. It will also
// tidy the notation, expand an abbreviation, correct what it takes to be a
// typographical error in Bourbaki, and hand back something fluent and wrong,
// and none of that is visible in a diff nobody reads. A page read twice is
// merely expensive. A page repaired into something Bourbaki did not write is a
// corpus that cannot be trusted, and there are 1194 pages of it.
//
// So no repair is accepted on the strength of the model saying it repaired
// something. Every kind of defect declares what a fix for it is allowed to
// change, and the answer has to prove it changed nothing else. Where that proof
// is not possible the repair is not offered, and the page goes back to being
// read again from the image, which is slow and safe.
package repair

import (
	"fmt"
	"strings"

	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/textguard"
)

// Refusal is what the model is told to say when it cannot fix the page without
// changing what is on it.
//
// Without somewhere to go, a model that cannot do the task does something else
// and calls it the task. An explicit refusal is cheap to check and it is the
// only answer that is allowed to be short.
const Refusal = "CANNOT REPAIR"

// Kind is a class of defect and, with it, the proof a fix has to carry.
type Kind string

const (
	// KindMath is a page whose inline mathematics is not delimited properly:
	// a formula opened with a dollar and never closed. The only legal edit is
	// to insert or remove dollars, and that is provable exactly.
	KindMath Kind = "math"
)

// MaxDelimiterEdits is how many dollars a repair may add or remove per problem
// reported.
//
// Without a bound, a page with one unclosed formula can be "fixed" by wrapping
// the whole page in a pair, which satisfies every other check here and is
// nonsense. Two is what closing one formula costs at worst, when the model
// moves a delimiter rather than adding one.
const MaxDelimiterEdits = 2

// Request is one page to be repaired.
type Request struct {
	// Book and Page say which page this is, for the log and nothing else.
	Book string
	Page int
	// Text is the transcription that failed. It is what the model is asked to
	// correct and it is what the audit compares against.
	Text string
	// Problems is what Validate found. They are named in the prompt so the
	// model is not asked to go looking, and they are what the audit re-runs.
	Problems []ocr.Problem
}

// Kind says which defect this request is about, or false when it is one this
// package will not attempt.
//
// A request has to be about one thing. A page that is both unbalanced and
// truncated has no minimal repair, because the missing text is missing: there
// is nothing to prove an answer against, and a model asked to supply it will
// supply it. That page goes back to the image.
func (r Request) Kind() (Kind, bool) {
	if len(r.Problems) == 0 {
		return "", false
	}
	for _, problem := range r.Problems {
		if problem.Rule != ocr.RuleMath {
			return "", false
		}
	}
	return KindMath, true
}

// Prompt is the follow up sent in the same conversation as the transcription.
//
// The page image is above it in the thread, which is the whole point: a model
// asked to close a formula it cannot see has to guess where the formula ended,
// and a guess here is indistinguishable from a reading.
//
// The instructions are negative on purpose. "Fix the mathematics" invites a
// model to improve the mathematics. What is wanted is a copy of its own answer
// with some dollars moved, and saying so plainly, twice, in the words of the
// thing it is not to do, is what makes the audit below pass rather than fire.
func (r Request) Prompt() string {
	var b strings.Builder
	b.WriteString("The transcription you just gave has a delimiter error in it. Here is what you produced, between the markers:\n\n")
	b.WriteString("<<<BEGIN\n")
	b.WriteString(r.Text)
	b.WriteString("\nEND>>>\n\n")
	b.WriteString("The problem:\n")
	for _, problem := range r.Problems {
		if problem.Line > 0 {
			fmt.Fprintf(&b, "- line %d: %s\n", problem.Line, problem.Detail)
		} else {
			fmt.Fprintf(&b, "- %s\n", problem.Detail)
		}
	}
	b.WriteString(`
Look at the page image again and repeat the transcription with the dollar signs corrected.

Rules, and they are absolute:

1. Change nothing except dollar signs. Not one letter, digit, space, accent, backslash or punctuation mark. If the two versions were compared with every dollar sign deleted from both, they must be identical.
2. Do not re-transcribe the page. Do not re-read it for other mistakes. If you see something else you believe is wrong, leave it wrong.
3. Do not add mathematics that is missing. Do not complete a formula that looks incomplete. If the page prints an incomplete formula, transcribe an incomplete formula.
4. Output the corrected transcription and nothing else. No preamble, no explanation of what you changed, no code fence.
5. If you cannot correct it under these rules, reply with exactly ` + Refusal + ` and nothing else.
`)
	return b.String()
}

// Result is what came back and what was made of it.
type Result struct {
	// Text is the repaired page, set only when Accepted.
	Text string
	// Accepted says whether it may be written to the corpus.
	Accepted bool
	// Reason is why it was not, or why the model declined.
	Reason string
	// Refused is the model saying it could not do it, which is a different
	// thing from an answer that failed the audit and is worth counting apart.
	Refused bool
}

// Audit decides whether an answer is a repair.
//
// Nothing here trusts the model's account of what it did. Each check is a
// property of the two texts that can be computed, and an answer that has all of
// them is a repair whatever the model intended.
func Audit(request Request, answer string, expect ocr.Expect) Result {
	kind, ok := request.Kind()
	if !ok {
		return Result{Reason: "this page has no defect that can be repaired without re-reading it"}
	}
	body := textguard.Normalise(textguard.Strip(ocr.StripToolHeader(answer)))
	if strings.TrimSpace(body) == "" {
		return Result{Reason: "the answer is empty"}
	}
	if isRefusal(body) {
		return Result{Refused: true, Reason: "the model says it cannot repair this page under the rules"}
	}
	// Rule 3 of the OCR rules, unchanged. A repair that narrates what it
	// repaired is narration in the corpus, and it arrives here looking exactly
	// like a page.
	if leaks := textguard.Check(body); len(leaks) > 0 {
		return Result{Reason: "the answer is not a transcription: " + leaks[0].Kind + ": " + leaks[0].Detail}
	}

	switch kind {
	case KindMath:
		if reason := auditMath(request.Text, body, len(request.Problems)); reason != "" {
			return Result{Reason: reason}
		}
	default:
		return Result{Reason: fmt.Sprintf("no audit is defined for %q, so nothing is accepted", kind)}
	}

	// It has to have worked. A repair that leaves the page failing the rule it
	// was for is a call spent to write the same problem back.
	if problems := ocr.Validate(body, expect, ocr.Options{}); len(problems) > 0 {
		return Result{Reason: "the repaired page still fails: " + ocr.Reasons(problems)}
	}
	return Result{Text: body, Accepted: true}
}

// auditMath is the proof that only delimiters moved.
//
// Deleting every unescaped dollar from both versions and requiring what is left
// to match byte for byte is the whole argument. A model cannot change a symbol,
// a word, a number or a space and pass it. It is not a similarity score with a
// threshold somebody has to defend; it either holds or the answer is thrown
// away.
//
// What it does not prove is that the dollar went to the right place. Both
// $x\in E$. and $x\in E.$ survive it, because both have the same text with the
// dollars taken out. That is the honest limit of the method and it is a much
// smaller error than the one being ruled out: the class this eliminates is the
// model inventing mathematics, which is invisible in review, and what it lets
// through is a delimiter one character out, which is not.
func auditMath(before, after string, problems int) string {
	if bare(before) != bare(after) {
		return "the answer changes the page, not only its delimiters"
	}
	edits := abs(count(after) - count(before))
	if budget := problems * MaxDelimiterEdits; edits > budget {
		return fmt.Sprintf("the answer moves %d delimiters for %d problem(s), at most %d is a repair", edits, problems, budget)
	}
	if edits == 0 {
		return "the answer is the same page, no delimiter was changed"
	}
	return ""
}

// bare is the text with every unescaped dollar removed. A dollar behind a
// backslash is a printed dollar, and Bourbaki's historical notes quote prices,
// so removing one would make a real difference look like no difference.
func bare(text string) string {
	var b strings.Builder
	b.Grow(len(text))
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			b.WriteRune(runes[i])
			b.WriteRune(runes[i+1])
			i++
			continue
		}
		if runes[i] == '$' {
			continue
		}
		b.WriteRune(runes[i])
	}
	return b.String()
}

// count is how many unescaped dollars there are, counted the same way.
func count(text string) int {
	var n int
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' && i+1 < len(runes) {
			i++
			continue
		}
		if runes[i] == '$' {
			n++
		}
	}
	return n
}

// isRefusal reads the escape hatch loosely. A model told to say one thing and
// nothing else says it with a full stop after it about half the time, and
// treating that as a failed repair would send the page for a re-read it does
// not need. Loose in length, though: the refusal has to be the whole answer,
// or a page that happens to discuss what cannot be repaired is thrown away.
func isRefusal(text string) bool {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) > len(Refusal)+8 {
		return false
	}
	upper := strings.ToUpper(strings.Trim(trimmed, " \t\r\n.\"'`*"))
	return upper == Refusal
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}
