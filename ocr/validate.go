// Package ocr reads the pages of a scanned volume through a model and decides
// whether what came back is a transcription.
//
// The decision is the expensive half. A call costs about 151 seconds, so an
// accepted page that is wrong is not caught by anything downstream until a
// person reads it, and there are 1194 of them. Everything here is written to
// reject cheaply and to say which rule rejected, because the retry that follows
// is chosen from the reason.
package ocr

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/textguard"
)

// MinChars is the shortest a real page can be.
//
// The thinnest page of either volume that carries text is the seven line close
// of chapter III, about 400 characters. Two hundred is under half of that and
// still well above what a truncated answer or a one line apology comes to.
const MinChars = 200

// MaxIllegible is how many unreadable spots a page may carry and still be
// accepted. Up to two is a damaged scan, which these are. More than that is a
// model that gave up, and re-reading it at 600 dpi is worth the call.
const MaxIllegible = 2

// Illegible is what the prompt asks for in place of a guess.
const Illegible = "⟪illegible⟫"

// SparseInk is the ink ratio below which a page is too thin to have a length
// expected of it. A full page of these volumes runs about 6.5 percent; the
// title pages and the publisher's device come in under half a percent. One
// percent sits between the two with room on both sides.
const SparseInk = 0.01

// Rule names the check that rejected a page. It goes in the queue history and
// in the failures report, and the retry policy reads it.
type Rule string

const (
	RuleShort     Rule = "short"     // 1, empty or under MinChars
	RuleMath      Rule = "math"      // 2, unbalanced $ or $$
	RuleLeak      Rule = "leak"      // 3, refusal, narration or the prompt back
	RuleHead      Rule = "head"      // 4, no plausible running head on the first line
	RuleIllegible Rule = "illegible" // 5, too many unreadable spots
	RuleLabel     Rule = "label"     // 6, the page label contradicts the page map
	RuleLaTeX     Rule = "latex"     // 7, the LaTeX does not compile
	RuleExercise  Rule = "exercise"  // 8, an exercise below the exercises head is set as a heading
)

// Problem is one reason a page was not accepted.
type Problem struct {
	Rule   Rule
	Detail string
	Line   int
}

func (p Problem) String() string {
	if p.Line > 0 {
		return fmt.Sprintf("%s: %s (line %d)", p.Rule, p.Detail, p.Line)
	}
	return fmt.Sprintf("%s: %s", p.Rule, p.Detail)
}

// Expect is what the rest of the pipeline already knows about a page, which is
// what makes rules 4 and 6 possible at all.
type Expect struct {
	Book string
	// PDFPage is the page of the file, one based.
	PDFPage int
	// Blank is what render measured. A blank page never reaches here, but
	// saying so explicitly keeps rule 1 from rejecting one if it ever does.
	Blank bool
	// Sparse is a page that has ink on it but very little: a title page, a part
	// title, the publisher's device. Rule 1 does not run on one.
	//
	// Without this the rule fires on pages that are short because the book is
	// short there. Measured: alg-iv-vii page 3 is the Springer knight and
	// nothing else at 0.47 percent ink, and alg-viii pages 1 to 3 are the half
	// title, the series title and the title at under fifty characters each.
	// Each would burn three calls at 151 seconds and then be filed as a defect
	// that is not one. The caller sets this from the render manifest, so a
	// volume that was never rendered leaves it false and the rule runs, which
	// is the safe way round.
	Sparse bool
	// Grammar is how this volume prints its running head. HeadLabel means a
	// full page label such as A IV.7, so rules 4 and 6 both apply. FootNumber
	// means the number is at the foot and the head carries at most a chapter
	// name, so rule 6 does not apply and rule 4 is weaker.
	Grammar pagemap.Grammar
	// Chapter and Page are what the page map says this page is. Page is 0 when
	// the page map does not know, which is the case for front matter, and then
	// rule 6 does not apply.
	Chapter string
	Page    int
	// Confidence is where the page map's number came from. An interpolated
	// number is a guess, and a guess must not reject a page that was read
	// correctly, so rule 6 only runs on a printed one.
	Confidence pagemap.Confidence
	// HasHead is false for the pages that print no running head at all: the
	// first page of a chapter, a part title, the pages of the front matter.
	// Rule 4 does not run on those.
	HasHead bool
}

// Options turn the opt-in rules on.
type Options struct {
	// LaTeX runs rule 7. It is opt-in because it needs a TeX installation and
	// costs a subprocess per page, and the other six catch almost everything.
	LaTeX TeXChecker

	// Prompt is what the model was asked, so rule 3 can tell whether it handed
	// the question back instead of answering it. Empty leaves that half of the
	// rule off, which is what a caller validating a page it did not read has to
	// do, since the answer depends on which prompt read it.
	Prompt string
}

// TeXChecker compiles a fragment and reports what went wrong. Behind an
// interface so the rule can be tested without a TeX installation.
type TeXChecker interface {
	Check(fragment string) error
}

// Validate applies the rules in order and returns everything that failed.
//
// All of them run rather than stopping at the first. A page that is short and
// has no running head is a different failure from one that is only short: the
// first is a truncated answer and the second is a page whose head the scan ate,
// and the retry differs.
func Validate(text string, expect Expect, options Options) []Problem {
	var problems []Problem

	// Rule 3 first, even though it is numbered third. A refusal is short and
	// has no running head and no mathematics, so checking it first means the
	// report says what actually happened rather than listing four symptoms.
	for _, leak := range textguard.Check(text) {
		problems = append(problems, Problem{Rule: RuleLeak, Detail: leak.Kind + ": " + leak.Detail, Line: leak.Line})
	}
	// The other half of rule 3, and it needs the prompt in hand rather than a
	// list of phrases off an older one. See textguard.Echo.
	for _, leak := range textguard.Echo(text, options.Prompt) {
		problems = append(problems, Problem{Rule: RuleLeak, Detail: leak.Kind + ": " + leak.Detail, Line: leak.Line})
	}
	if len(problems) > 0 {
		return problems
	}

	body := strings.TrimSpace(text)

	// Rule 1.
	if !expect.Blank && !expect.Sparse && len([]rune(body)) < MinChars {
		problems = append(problems, Problem{
			Rule:   RuleShort,
			Detail: fmt.Sprintf("%d characters, want at least %d", len([]rune(body)), MinChars),
		})
	}

	// Rule 2.
	if problem, ok := checkMath(body); !ok {
		problems = append(problems, problem)
	}

	// Rule 5.
	if n := strings.Count(body, Illegible); n > MaxIllegible {
		problems = append(problems, Problem{
			Rule:   RuleIllegible,
			Detail: fmt.Sprintf("%d unreadable spots, want at most %d", n, MaxIllegible),
		})
	}

	// Rules 4 and 6 both read the first line.
	head := firstLine(body)
	if expect.HasHead {
		if problem, ok := checkHead(head, expect); !ok {
			problems = append(problems, problem)
		}
	}
	if problem, ok := checkLabel(head, expect); !ok {
		problems = append(problems, problem)
	}

	// Rule 8.
	if problem, ok := checkAfterExercises(body); !ok {
		problems = append(problems, problem)
	}

	// Rule 7.
	if options.LaTeX != nil {
		if err := options.LaTeX.Check(body); err != nil {
			problems = append(problems, Problem{Rule: RuleLaTeX, Detail: err.Error()})
		}
	}
	return problems
}

// OK says whether a page is accepted.
func OK(text string, expect Expect, options Options) bool {
	return len(Validate(text, expect, options)) == 0
}

// checkMath is rule 2. It counts the delimiters over the page and then again
// paragraph by paragraph, because the first count on its own is too weak to
// catch what the fleet actually returns.
// The paragraph check runs first even though the page count is the cruder and
// more obvious of the two. Both fire on a single unclosed formula, and only one
// of them can say which line it is on. That matters beyond the report: the
// repair prompt names the line so the model is not sent looking through five
// hundred words for something it cannot be told the shape of, and a problem
// with no line in it turns a targeted fix into a re-read.
func checkMath(text string) (Problem, bool) {
	if problem, ok := checkInlineRuns(text); !ok {
		return problem, ok
	}
	return checkMathTotals(text)
}

// checkMathTotals counts the delimiters over the whole page.
//
// The two forms have to be counted together and in one pass, because $$ is two
// dollars and a naive count of $ says every display page is unbalanced. Escaped
// dollars are literal and do not count, and Bourbaki uses them: the volumes
// print prices in the historical notes.
func checkMathTotals(text string) (Problem, bool) {
	var inline, display int
	runes := []rune(text)
	for i := 0; i < len(runes); i++ {
		if runes[i] == '\\' {
			i++ // whatever follows a backslash is literal, including a dollar
			continue
		}
		if runes[i] != '$' {
			continue
		}
		if i+1 < len(runes) && runes[i+1] == '$' {
			display++
			i++
			continue
		}
		inline++
	}
	if display%2 != 0 {
		return Problem{Rule: RuleMath, Detail: fmt.Sprintf("%d display delimiters, an odd number", display)}, false
	}
	if inline%2 != 0 {
		return Problem{Rule: RuleMath, Detail: fmt.Sprintf("%d inline delimiters, an odd number", inline)}, false
	}
	return Problem{}, true
}

// checkInlineRuns takes the same count paragraph by paragraph.
//
// Parity over the whole page passes a page with two unclosed dollars as readily
// as one with none, and that is not a corner case. Page 53 of Algebra I came
// back with "and all $x\in E." at the end of one paragraph and "$\sup$ and
// $\inf." at the end of another, both missing their closing dollar, and the two
// odd counts added to an even one. The page was accepted with two broken
// formulae on it and no flag.
//
// An inline formula does not run across a blank line anywhere in these volumes,
// so a dollar still open when a paragraph ends is an unclosed one. That is a
// stronger rule than parity and it can name the line, which parity cannot.
//
// Dollars inside a display block are not inline delimiters and are not counted,
// which is what the display parity is tracked for here: a $$ block that spans a
// blank line leaves the count odd, and while it is odd the rule stands down.
func checkInlineRuns(text string) (Problem, bool) {
	openLine, display := 0, 0
	for i, raw := range strings.Split(text, "\n") {
		if strings.TrimSpace(raw) == "" {
			if openLine > 0 && display%2 == 0 {
				return unclosed(openLine), false
			}
			continue
		}
		runes := []rune(raw)
		for j := 0; j < len(runes); j++ {
			if runes[j] == '\\' {
				j++
				continue
			}
			if runes[j] != '$' {
				continue
			}
			if j+1 < len(runes) && runes[j+1] == '$' {
				display++
				j++
				continue
			}
			if display%2 != 0 {
				continue // inside a display block, not an inline delimiter
			}
			if openLine > 0 {
				openLine = 0
			} else {
				openLine = i + 1
			}
		}
	}
	if openLine > 0 && display%2 == 0 {
		return unclosed(openLine), false
	}
	return Problem{}, true
}

func unclosed(line int) Problem {
	return Problem{
		Rule:   RuleMath,
		Detail: "an inline $ opened on this line is never closed before the paragraph ends",
		Line:   line,
	}
}

// exerciseHeadRE is the head a volume prints over the exercises of a chapter,
// English and French, at whatever level the reading gave it.
var exerciseHeadRE = regexp.MustCompile(`^#+\s*(EXERCISES|EXERCICES)\s*$`)

// numberedHeadRE is a no. or a statement set as a heading, which is what the
// body of a § is written with and what nothing below the exercises head can be.
// The § head itself is left out of it: a chapter gathers the exercises of all
// its sections under one head and divides them by §, so "## § 3" below the
// exercises head is the printing and not a defect.
var numberedHeadRE = regexp.MustCompile(`^#{3,}\s`)

// checkAfterExercises is rule 8.
//
// Nothing of a § comes after its exercises, so a no. or a statement heading
// below the exercises head is the reading of the page and not the page. Page
// 289 of Theory of Sets prints EXERCISES, then § 1, then "1. Let S be the set
// of signs P, X, ..." as an ordinary paragraph, and the reading came back with
// that paragraph as "### 1. Let S be the set of signs". Every other exercise of
// the volume, on 32 other pages, came back as the prose it is, including the
// one on the very next page, which opens the same way word for word.
//
// The assembler is where this used to surface, as "the exercises are followed
// by the heading", and by then the page has been accepted, committed and paid
// for. Here it costs one re-read from an image that is still on disk.
//
// Measured over every page of every volume this project has read, 3301 of them
// in English and French: 33 carry an exercises head, 9 English and 24 French,
// one line below one of them is a § heading and one is a numbered heading, and
// the numbered one is page 289 of Theory of Sets. So the rule rejects exactly
// the page it was written for.
//
// It only sees the page it is given, and a volume prints EXERCISES once over a
// block that runs for pages. An exercise set as a heading on the second page of
// such a block goes past this rule, and the assembler is still the thing that
// catches it.
func checkAfterExercises(text string) (Problem, bool) {
	after := false
	for i, line := range strings.Split(text, "\n") {
		if exerciseHeadRE.MatchString(strings.TrimSpace(line)) {
			after = true
			continue
		}
		if after && numberedHeadRE.MatchString(line) {
			return Problem{
				Rule:   RuleExercise,
				Detail: fmt.Sprintf("an exercise below the exercises head is set as a heading: %q", clip(line)),
				Line:   i + 1,
			}, false
		}
	}
	return Problem{}, true
}

// longestHead is how long a running head is allowed to be, in runes.
//
// Bourbaki prints a chapter name and a page label and nothing else up there.
// Over the 200 pages of golden-dev the longest one printed is 64 runes, so 90
// is not a tight bound, it is room for a printing whose heads run longer than
// any of these do.
//
// It is a named constant because three places need it and two of them did not
// have it. ParsePageLabel and ParseSectionLocator both search the string they
// are given rather than matching it, which is what their other callers want,
// since the job there is to pull a locator out of the middle of a citation. Ask
// either of them whether a 473 rune paragraph is a running head, because it
// happens to cite II, § 4, and it says yes.
const longestHead = 90

// checkHead is rule 4.
//
// What counts as a plausible head depends on how the volume prints one. A
// volume with head labels has to show one. A volume that prints its number at
// the foot has only the chapter name or the section locator up there, and the
// most that can be asked is that the first line is a head and not the first
// sentence of a paragraph: short, and set in capitals.
func checkHead(head string, expect Expect) (Problem, bool) {
	if strings.TrimSpace(head) == "" {
		return Problem{Rule: RuleHead, Detail: "the first line is empty, the page map says this page has a running head", Line: 1}, false
	}
	// Before the grammar, because it is true under both of them and because the
	// label branch below is the one that let it through. 9 of the 200 readings
	// on golden-dev open with a paragraph of body text carrying a citation, and
	// this rule called every one of them a page with a running head.
	if len([]rune(strings.TrimSpace(head))) > longestHead {
		return Problem{Rule: RuleHead, Detail: fmt.Sprintf("the first line reads as prose, not a running head: %q", clip(head)), Line: 1}, false
	}
	if expect.Grammar == pagemap.HeadLabel {
		if _, ok := corpus.ParsePageLabel(head); !ok {
			return Problem{Rule: RuleHead, Detail: fmt.Sprintf("no page label in the first line: %q", clip(head)), Line: 1}, false
		}
		return Problem{}, true
	}
	// FootNumber. A running head here is a chapter name in capitals, a section
	// locator, or a bare number. A line of prose is none of those.
	if _, ok := corpus.ParseSectionLocator(head); ok {
		return Problem{}, true
	}
	if looksLikeHead(head) {
		return Problem{}, true
	}
	return Problem{Rule: RuleHead, Detail: fmt.Sprintf("the first line reads as prose, not a running head: %q", clip(head)), Line: 1}, false
}

// looksLikeHead is deliberately loose. Rejecting a good page costs 151 seconds
// and lands it in the failures report; letting a doubtful one through costs a
// line in the audit, which a person reads anyway.
func looksLikeHead(line string) bool {
	line = strings.TrimSpace(line)
	if len([]rune(line)) > longestHead {
		return false // a running head is not a paragraph
	}
	letters, upper := 0, 0
	for _, r := range line {
		switch {
		case r >= 'a' && r <= 'z':
			letters++
		case r >= 'A' && r <= 'Z':
			letters++
			upper++
		}
	}
	if letters == 0 {
		// A bare number or a locator, both of which carry a digit. Without the
		// digit the line is punctuation, and the one that turns up is a reader
		// opening the page on a display and writing \[ or \( on the first line.
		return strings.ContainsFunc(line, unicode.IsDigit)
	}
	// Bourbaki sets its running heads in capitals. Half is enough, because the
	// small capitals of a chapter title come back from OCR mixed.
	//
	// A terminal full stop used to veto the line before this test, on the
	// reading that a sentence ends in one and a head does not. That holds for
	// the volumes whose heads are a title and a locator and fails for hist,
	// which prints "23. HAAR MEASURE. CONVOLUTION.", "PREFACE." and "TABLE OF
	// CONTENTS." with the stop, and pages of it went dead after three attempts
	// on a head the page really prints. Across the 4809 raw readings on disk
	// there are 112 first lines of 90 characters or fewer that end in a stop.
	// 97 of them read as a head under this rule, over 60 distinct pages, 56 of
	// those in hist. The other 15 are mixed case sentences, and the capitals
	// test below rejects every one of them on its own. That is the whole of the
	// veto's work: a sentence of prose ending in a full stop is mixed case and
	// fails the capitals test anyway, so the veto only ever fired on heads.
	return upper*2 >= letters
}

// checkLabel is rule 6.
//
// It only runs where a page label is printed and where the page map read its
// number off a page rather than working it out, because two guesses disagreeing
// says nothing. One page of slack is allowed: the head of a verso and the head
// of the facing recto differ by one, and a scan that is off by a page in its
// own numbering is a known feature of these files.
func checkLabel(head string, expect Expect) (Problem, bool) {
	if expect.Grammar != pagemap.HeadLabel || expect.Page == 0 || !expect.Confidence.Printed() {
		return Problem{}, true
	}
	label, ok := corpus.ParsePageLabel(head)
	if !ok {
		return Problem{}, true // rule 4 has already said so, do not say it twice
	}
	if expect.Chapter != "" && label.Chapter != expect.Chapter {
		return Problem{
			Rule:   RuleLabel,
			Detail: fmt.Sprintf("the page says chapter %s, the page map says %s", label.Chapter, expect.Chapter),
			Line:   1,
		}, false
	}
	if diff := label.Page - expect.Page; diff > 1 || diff < -1 {
		return Problem{
			Rule:   RuleLabel,
			Detail: fmt.Sprintf("the page says %d, the page map says %d", label.Page, expect.Page),
			Line:   1,
		}, false
	}
	return Problem{}, true
}

func firstLine(text string) string {
	for line := range strings.SplitSeq(text, "\n") {
		if strings.TrimSpace(line) != "" {
			return strings.TrimSpace(line)
		}
	}
	return ""
}

func clip(s string) string {
	runes := []rune(s)
	if len(runes) <= 60 {
		return s
	}
	return string(runes[:60]) + "…"
}

// Reasons joins the problems into the one line that goes in the queue history.
func Reasons(problems []Problem) string {
	if len(problems) == 0 {
		return ""
	}
	parts := make([]string, 0, len(problems))
	for _, problem := range problems {
		parts = append(parts, problem.String())
	}
	return strings.Join(parts, "; ")
}

// Rules returns the distinct rules that rejected a page, which is what the
// retry policy switches on.
func Rules(problems []Problem) []Rule {
	seen := map[Rule]bool{}
	var out []Rule
	for _, problem := range problems {
		if !seen[problem.Rule] {
			seen[problem.Rule] = true
			out = append(out, problem.Rule)
		}
	}
	return out
}
