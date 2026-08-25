package textguard

import (
	"strings"

	"github.com/tamnd/bourbaki-solver/mathtex"
)

// An inline span is written tight against its dollars. $K[[T]]$ and not
// $ K[[T]] $.
//
// TeX does not care. Whitespace is not significant in math mode, so both spell
// the same formula and KaTeX sets them identically, which is why the padded
// form got into the corpus in the first place and stayed: nothing on the site
// ever looked different because of it.
//
// Two things do care. The first is GitHub, which renders $...$ in a Markdown
// file by a rule borrowed from pandoc: the opening dollar must not be followed
// by whitespace and the closing dollar must not be preceded by it. A padded
// span fails that test, so every $ K[[T]] $ in the corpus shows up on
// github.com as four literal characters and a formula that was never set. The
// files are read there far more often than they are read on the site.
//
// The second is the corpus itself. It arrived by three routes, the OCR, the
// mender and the translator, and each pads differently: measured over the tree
// as committed, English is 72 per cent padded, French 50 per cent, Vietnamese
// 63 per cent and the solutions almost not at all. That is not a house style,
// it is the absence of one, and it means a grep for a formula has to be written
// twice and a diff between two languages of the same section is noise. One
// spelling, and this is where the other one is turned into it.
//
// Displays are left alone. A display is set on its own lines with the
// delimiters on lines of their own, so the whitespace inside it is the line
// break that puts it there and taking that out would run the formula into its
// dollars and change the shape of every display in the corpus for nothing.

// A Pad is one inline span written loose against its dollars, for the audit.
type Pad struct {
	Line int    // the body line the opening delimiter sits on, counting from one
	Text string // the span as the corpus has it, without its delimiters
}

// padding is the whitespace this takes off. Spaces and tabs, not newlines.
//
// An inline span that runs across a line break is rare and it is somebody
// else's fault to fix, but pulling the dollar down onto the next line or the
// text up onto the previous one would reflow the paragraph around it, and a
// repair that changes lines nobody asked about is a repair that cannot be
// reviewed. So a span that starts or ends at a newline keeps it.
const padding = " \t"

// Tighten writes the corpus's inline spans tight against their dollars and says
// how many it closed up.
//
// It is idempotent: a text it has been through has no padded span left in it
// for a second pass to find.
//
// It refuses a body with a span left open, and gives it back untouched. The
// offsets after an unclosed delimiter are not the spans anybody meant, so
// trimming by them would move dollars around in prose. M01 reports that body
// and it is repaired by hand; this is not the pass to guess at it.
func Tighten(text string) (string, int) {
	spans, unclosed := mathtex.Split(text)
	if unclosed != nil || len(spans) == 0 {
		return text, 0
	}
	rs := []rune(text)
	var b strings.Builder
	at, n := 0, 0
	for _, s := range spans {
		tight, ok := tighten(s)
		if !ok {
			continue
		}
		b.WriteString(string(rs[at:s.Start]))
		b.WriteString(tight)
		at, n = s.End, n+1
	}
	if n == 0 {
		return text, 0
	}
	b.WriteString(string(rs[at:]))
	return b.String(), n
}

// Padded is the same reading with nothing given back, for the audit.
func Padded(text string) []Pad {
	spans, unclosed := mathtex.Split(text)
	if unclosed != nil {
		return nil
	}
	var out []Pad
	for _, s := range spans {
		if _, ok := tighten(s); ok {
			out = append(out, Pad{Line: s.Line, Text: s.Text})
		}
	}
	return out
}

// tighten says what one span should read and whether it is loose to begin with.
//
// An empty span is left as it is. $ $ and $$ both mean nothing, and the second
// is not an inline span at all: it is the opening delimiter of a display, so
// closing that one up would turn a formula that is merely blank into a
// delimiter that swallows the rest of the paragraph.
func tighten(s mathtex.Span) (string, bool) {
	if s.Display {
		return "", false
	}
	tight := strings.Trim(s.Text, padding)
	if tight == "" || tight == s.Text {
		return "", false
	}
	return tight, true
}
