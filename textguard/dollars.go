package textguard

import "strings"

// The corpus writes a display between two pairs of dollars and an inline span
// between one pair, and it writes them that way everywhere. LaTeX has a second
// spelling for both, \[ ... \] for the display and \( ... \) for the span, and
// the two mean exactly the same thing to a mathematician and nothing at all to
// this corpus.
//
// The reason it matters is not taste. Every rule that reads mathematics here
// reads it through mathtex.Split, which finds spans by their dollars, so a
// formula written with brackets is not a formula as far as the corpus is
// concerned: it is prose that happens to have backslashes in it. M02 does not
// check its braces, M05 does not check its operators, M10 does not look for a
// lost stroke, and Stars will happily rewrite an asterisk inside it because
// inMath says it is not inside anything. The site renders it the same way a
// reader would then see it, as literal backslashes and brackets sitting in the
// middle of a sentence.
//
// So it is one spelling, and this is where the other one is turned into it.
const (
	Display = "$$"
	Span    = "$"
)

// A Bracket is one line written with the other spelling, for the audit.
type Bracket struct {
	Line int    // the body line it sits on, counting from one
	Name string // which delimiter it is
	Text string // the line it was found on
}

// The row break of a matrix is \\, and \\[2pt] is a row break asking for space
// after it. Both start with a backslash that is not the delimiter's, so they are
// put aside before the delimiters are turned round and put back afterwards. The
// corpus has a good many of them, \\[2pt] and \\[2mm] and \\[-1.5em] among
// others, and every one is a legitimate row break rather than a display.
var (
	rowBreak     = strings.NewReplacer(`\\`, "\x00")
	rowBreakBack = strings.NewReplacer("\x00", `\\`)
	delimiters   = strings.NewReplacer(
		`\[`, Display,
		`\]`, Display,
		`\(`, Span,
		`\)`, Span,
	)
)

// bracketName says what each of the four is, so a finding can name it rather
// than print a backslash and leave somebody to work out which end it was.
var bracketName = []struct{ delim, name string }{
	{`\[`, "a display opened with a bracket"},
	{`\]`, "a display closed with a bracket"},
	{`\(`, "a span opened with a parenthesis"},
	{`\)`, "a span closed with a parenthesis"},
}

// Dollars writes the corpus's delimiters and says how many it turned round.
//
// It is idempotent, since a text it has been through carries no \[ for it to
// find on a second pass, which is what lets bourbaki fix dollars be run over the
// corpus as often as anyone likes.
func Dollars(text string) (string, int) {
	held := rowBreak.Replace(text)
	n := 0
	for _, b := range bracketName {
		n += strings.Count(held, b.delim)
	}
	if n == 0 {
		return text, 0
	}
	return rowBreakBack.Replace(delimiters.Replace(held)), n
}

// Brackets is the same reading with nothing given back, for the audit. One
// finding per line, naming the first delimiter on it, which is enough to send a
// reader to the right place.
func Brackets(text string) []Bracket {
	var out []Bracket
	for i, line := range strings.Split(text, "\n") {
		held := rowBreak.Replace(line)
		at, name := -1, ""
		for _, b := range bracketName {
			if j := strings.Index(held, b.delim); j >= 0 && (at < 0 || j < at) {
				at, name = j, b.name
			}
		}
		if at >= 0 {
			out = append(out, Bracket{Line: i + 1, Name: name, Text: line})
		}
	}
	return out
}
