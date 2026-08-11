package share

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/textguard"
)

// Boundary is what happened where one answer met the next.
//
// Every boundary is recorded, joined or not, and the record goes in the
// manifest. The joining rule below is a judgement about somebody else's prose
// and it will eventually be wrong about a line; when that happens the way to
// find it is to have written down where the decisions were, not to re-derive
// them from the output, where a join and a paragraph that was always one
// paragraph look the same.
type Boundary struct {
	// After is the index of the answer on the left of the boundary.
	After int
	// Joined says whether the last paragraph of that answer and the first of
	// the next were run together into one.
	Joined bool
	// Why is the reason, in the words of the rule that decided.
	Why string
}

// Page is one imported conversation, rendered.
type Page struct {
	Body       string
	Boundaries []Boundary
	// Models are the model slugs that wrote the answers, deduplicated in the
	// order they first appear. Normally one. More than one means the
	// conversation was picked up again later, possibly by something smaller,
	// and the manifest says so rather than averaging it away.
	Models []string
}

// Markdown renders a conversation as the corpus writes Markdown.
//
// It refuses rather than repairs. An answer carrying a refusal, a narration or
// the provider's own markup is not a transcription with a blemish on it, it is
// an answer to a different question, and the import is abandoned with the turn
// named. That is the same bargain the OCR path makes, and the reason it is
// worth making here too is that a share page is somebody's old conversation:
// nobody is watching it go by, so nothing else will notice.
func Markdown(c *Conversation) (*Page, error) {
	p := &Page{}
	seen := map[string]bool{}
	var turns [][]string
	for i, t := range c.Turns {
		text := textguard.Strip(t.Text)
		if leaks := textguard.Check(text); len(leaks) > 0 {
			return nil, fmt.Errorf("answer %d of %d is not a transcription: %s on line %d",
				i+1, len(c.Turns), leaks[0].Detail, leaks[0].Line)
		}
		turns = append(turns, blocks(math(textguard.Normalise(text))))
		if t.Model != "" && !seen[t.Model] {
			seen[t.Model] = true
			p.Models = append(p.Models, t.Model)
		}
	}
	var out []string
	prevStart := 0
	for i, bs := range turns {
		if i == 0 || len(out) == 0 || len(bs) == 0 {
			prevStart = len(out)
			out = append(out, bs...)
			continue
		}
		at, why := running(out, prevStart)
		if at >= 0 {
			ok, w := joins(out[at], bs[0])
			why += w
			if !ok {
				at = -1
			}
		}
		p.Boundaries = append(p.Boundaries, Boundary{After: i - 1, Joined: at >= 0, Why: why})
		if at >= 0 {
			out[at] = out[at] + " " + bs[0]
			bs = bs[1:]
		}
		prevStart = len(out)
		out = append(out, bs...)
	}
	p.Body = strings.Join(out, "\n\n") + "\n"
	return p, nil
}

// running finds the block in the previous answer that the next answer might be
// continuing, which is the last one that is not a footnote.
//
// A footnote is printed at the foot of the page, so it comes last in the answer
// and yet the running text above it is what carries on overleaf. Joining to the
// last block outright gets that backwards, and it did: the seam in § 1 no. 3
// where the page ends "every finite and the next begins division ring is a
// field" has a footnote sitting between the two halves, and the sentence came
// back glued onto the end of the footnote, where it is both wrong and out of
// sight. Skipping back over the footnotes puts it where the book prints it.
//
// Measured on the four Theory of Sets share pages: 4 blocks open as footnotes,
// all of them "(*)", and 3 of the 4 are the last block of their answer. Of the
// 31 seams exactly one has a footnote on its left, and this is what that one
// turns from wrong into right.
func running(out []string, from int) (int, string) {
	notes := 0
	for i := len(out) - 1; i >= from; i-- {
		if footnote(out[i]) {
			notes++
			continue
		}
		if notes == 0 {
			return i, ""
		}
		return i, fmt.Sprintf("past %d footnote(s) at the foot of the page, ", notes)
	}
	return -1, "the answer is nothing but footnotes"
}

// footnote is a block that opens with a reference mark. Bourbaki sets these as
// (*), (**) and so on; a numbered form is allowed for because other volumes use
// it, and a paragraph of prose does not begin "(1) " by accident.
//
// The dollars are not decoration. Of the 5 footnotes in these four sections one
// came back as $(*)$, with the mark set as mathematics, which is the same mark
// and would not have matched. That is a model being inconsistent with itself
// across two answers in the same conversation, which is the normal case and not
// a fault worth rejecting a page over.
var footnoteMark = regexp.MustCompile(`^\$?\(\s*(\*{1,4}|\d{1,3})\s*\)\$?\s`)

func footnote(block string) bool {
	return footnoteMark.MatchString(strings.TrimSpace(block))
}

// joins decides whether a block of one answer and the first block of the next
// are two halves of one paragraph.
//
// The assembler has this question already, for the seam between two PDF pages,
// and answers it with the indent: a page whose first line is not indented
// continues the paragraph above. There is no indent here. The pages went into
// the conversation as images and came back as prose, and whatever the model
// knew about the indent it did not write down.
//
// What is left is the text on either side, and it turns out to be enough,
// because English sentences do not begin in lower case. If the next answer
// opens with a lower case word then it is not opening a sentence, so it is
// finishing the one above. That is the whole rule.
//
// Measured on the four Theory of Sets share pages of 11 August 2026, which have
// 31 seams between them: the rule joins 6 and leaves 25, and reading all 31 by
// hand it is right about every one, given that running above has already found
// it the correct block to look at. Notably it is right about the two shapes a
// rule made of punctuation gets wrong. It does not join across a seam where the
// left side ends in display mathematics and the right side opens a new
// paragraph, which happened 4 times; and it does join the seam where the left
// side ends in a full stop inside a quotation and the right side carries on
// with "division ring is a field", which a punctuation rule would have split.
//
// The obvious way for it to be wrong is a paragraph that really does begin with
// a lower case symbol, which mathematics allows. There were none in the 31, and
// the cost if there is one is a run-on paragraph rather than a lost sentence,
// which is the right way round for a mistake to fall.
func joins(prev, next string) (bool, string) {
	if !paragraph(prev) {
		return false, "the answer ends with something that is not a paragraph"
	}
	if !paragraph(next) {
		return false, "the next answer opens with something that is not a paragraph"
	}
	if strings.HasSuffix(strings.TrimRight(prev, " "), "-") {
		// Never seen in the 31 seams, because these answers are prose the model
		// rewrapped rather than lines lifted off a page. Left separate rather
		// than glued, since gluing a hyphen wrongly buries the mistake inside a
		// word, where nothing will find it again.
		return false, "the answer ends in a hyphen, which is ambiguous, so the two are left apart"
	}
	r := opener(next)
	if !unicode.IsLower(r) {
		return false, fmt.Sprintf("the next answer opens with %q, which can begin a sentence", string(r))
	}
	return true, fmt.Sprintf("the next answer opens with %q, which cannot begin a sentence", string(r))
}

// opener is the first character of a block that carries meaning, which means
// looking past the emphasis a block may open with. Four of the answers here
// open with *is a theorem in ...* or *Remark.*, and the asterisk is markup and
// says nothing about whether a sentence is starting.
func opener(block string) rune {
	for _, r := range block {
		if r == '*' || r == '_' || unicode.IsSpace(r) {
			continue
		}
		return r
	}
	return 0
}

// listItem, and the rest of paragraph, are the block shapes that are not prose
// and so cannot be half of a sentence.
var listItem = regexp.MustCompile(`^([-*+]|\d+[.)])\s`)

func paragraph(block string) bool {
	block = strings.TrimSpace(block)
	switch {
	case block == "":
		return false
	case strings.HasPrefix(block, "#"), strings.HasPrefix(block, ">"),
		strings.HasPrefix(block, "|"), strings.HasPrefix(block, "$$"),
		strings.HasPrefix(block, "```"):
		return false
	case strings.HasSuffix(block, "$$"):
		// A block that ends in display mathematics can be continued by the
		// words after it, but not by gluing them on with a space: the display
		// is its own block and the words are the next one. Sitting adjacent is
		// already the right Markdown, so there is nothing to join.
		return false
	case listItem.MatchString(block), footnote(block):
		return false
	}
	return true
}

// math rewrites the provider's delimiters as the corpus writes them.
//
// The corpus is $ for inline and $$ on a line of its own for display; a share
// answer is \( \) and \[ \]. Measured on the four Theory of Sets pages: 1069
// inline spans, 233 display, and not one dollar sign anywhere in the source, so
// nothing has to be escaped and the rewrite cannot collide with text that was
// already mathematics.
//
// The display delimiters arrive alone on their line 465 times out of 466. The
// exception is a quotation carrying a display inside it, where the close is
// written \]" with the quotation mark behind it; that one is split onto two
// lines, which leaves the closing quotation mark on a line of its own. It is
// visible, it is one occurrence, and it is better than a $$ with something
// after it, which does not render as mathematics at all.
//
// The space inside an inline span is trimmed, which happened once in the 1069.
// \( (A)\operatorname{or}(B) \) is mathematics either way in TeX, but a dollar
// with a space after it does not open a span at all in Markdown, so the line
// would have rendered as literal dollars and backslashes in the middle of a
// sentence about the word "or".
func math(text string) string {
	text = inlineSpan.ReplaceAllStringFunc(text, func(s string) string {
		return "$" + strings.TrimSpace(s[2:len(s)-2]) + "$"
	})
	text = strings.ReplaceAll(text, `\[`, "$$")
	text = strings.ReplaceAll(text, `\]`, "$$")
	var out []string
	for _, line := range strings.Split(text, "\n") {
		indent := line[:len(line)-len(strings.TrimLeft(line, " \t"))]
		rest := strings.TrimSpace(line)
		if rest == "$$" || !strings.HasPrefix(rest, "$$") {
			out = append(out, line)
			continue
		}
		out = append(out, indent+"$$")
		if tail := strings.TrimSpace(rest[2:]); tail != "" {
			out = append(out, indent+tail)
		}
	}
	return strings.Join(out, "\n")
}

// blocks splits Markdown at blank lines. The pieces keep their own line breaks,
// so an indented quotation stays indented and a display stays on three lines.
func blocks(text string) []string {
	var out []string
	for _, b := range blankLine.Split(text, -1) {
		if strings.TrimSpace(b) != "" {
			out = append(out, strings.Trim(b, "\n"))
		}
	}
	return out
}

var blankLine = regexp.MustCompile(`\n[ \t]*\n`)

// inlineSpan is one \( ... \) pair. The backslash before the bracket is what
// makes it a delimiter and not \left( or \bigl(, where the character in front
// of the bracket is a letter.
var inlineSpan = regexp.MustCompile(`(?s)\\\(.*?\\\)`)
