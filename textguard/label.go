package textguard

import (
	"strings"

	"github.com/tamnd/bourbaki-solver/mathtex"
)

// The item labels of an exercise, put back outside the mathematics.
//
// Bourbaki sets the parts of an exercise as a), b), c) and so on, in italic, and
// the parts of a long proof the same way with capitals. Italic on a printed page
// is also how a variable is set, so a model reading the page sees the same slope
// on the a of "a) Show that" as on the a of "for all a in A", and writes the
// label as mathematics: $a)$ at the head of the line, or $a$) where it put the
// bracket outside instead. 1557 lines of pages/ carry the first shape and 23 the
// second, across 361 files and every volume that has exercises in it.
//
// It is wrong twice over. A lone bracket with nothing opening it is not TeX at
// all, so $a)$ is a span that no renderer can read and KaTeX prints it in the
// error colour. And a label is not a variable: the a of "a) Show that" names a
// part of a question, it is not quantified over anything, and setting it as
// mathematics says it is. The translators copied the shape through, so
// content/vi carries it too, and one of them lost the closing dollar on the way
// and left a file with an odd number of them.
//
// A span holding one letter and a closing bracket is a label wherever it stands,
// mid sentence as well as at the head of a line, because the bracket is inside
// the mathematics there and unmatched brackets are not mathematics. References
// to a label read the same way: "deduce from $b)$ that" is the prose pointing at
// part b, and it is set in text like the label it points at.
//
// The second shape has to be read more carefully, and only the head of a line
// will do. $x$) mid sentence is nearly always a variable at the end of a prose
// bracket, "compatible (in $x$)" and "with respect to the function $f$)", and
// there are 2056 of those against 23 labels. At the head of a line there is no
// bracket open to close, so there the same shape can only be a label.

// isLabel says whether the text of a span is a label with its bracket inside.
func isLabel(text string) bool {
	rs := []rune(text)
	return len(rs) == 2 && rs[1] == ')' && letter(rs[0])
}

// letter is ASCII only. The labels are a) to k) and A) to C) in this corpus, and
// a Greek or accented letter standing alone before a bracket is likelier to be
// mathematics somebody wrote badly than a label, so it is left for a person.
func letter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// Labels takes the item labels out of the mathematics and returns the text and
// how many it moved.
//
// Display spans are left alone. A label is never set on a line of its own in the
// middle of a paragraph, and $$a)$$ in this corpus would be something else.
func Labels(text string) (string, int) {
	spans, _ := mathtex.Split(text)
	rs := []rune(text)
	var b strings.Builder
	at, n := 0, 0
	for _, s := range spans {
		if s.Display {
			continue
		}
		// Start is the first rune of the content and End is the closing dollar,
		// so the span with its delimiters runs from Start-1 to End+1.
		open, close := s.Start-1, s.End+1
		if open < at || close > len(rs) {
			continue // an earlier span already covered this, or the body ended
		}
		switch {
		case isLabel(s.Text):
		case len(s.Text) == 1 && letter(rune(s.Text[0])) &&
			close < len(rs) && rs[close] == ')' && startsLine(rs, open):
		default:
			continue
		}
		b.WriteString(string(rs[at:open]))
		b.WriteString(s.Text)
		at, n = close, n+1
	}
	b.WriteString(string(rs[at:]))
	return b.String(), n
}

// startsLine says whether i is the first rune of a line of the body.
func startsLine(rs []rune, i int) bool {
	return i == 0 || rs[i-1] == '\n'
}
