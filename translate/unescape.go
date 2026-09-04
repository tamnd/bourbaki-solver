package translate

import (
	"regexp"
	"strings"

	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/mathtex"
)

// mdEscapeRE is a Markdown escape that has no business inside a formula: a
// backslash in front of one of the two characters Markdown gives a meaning to
// and mathematics mode does not.
//
// The brace, the ampersand, the caret, the tilde and the backslash itself are
// all left out, because every one of them is TeX a formula in this corpus
// really writes and taking a backslash off one would be silent damage. Counted
// over the 316374 math spans of content/en, \{ occurs 3252 times, \} 3228, \\
// 1739 and \& 32, and those are braces, line ends and alignment marks doing
// their job. The underscore and the star are the two where the escaped form is
// Markdown's idea and the mathematics never wants it: \_ occurs 0 times in
// those spans and \* 0 times.
var mdEscapeRE = regexp.MustCompile(`\\([_*])`)

// Unescape takes the Markdown escaping back out of a math span the model
// escaped as though the formula were prose, and leaves everything else exactly
// as the model wrote it.
//
// The index of notation of Algebra VIII was refused on this and could not get
// past it: the English opens with $A_M$ and the answer came back $A\_M$, twice
// running on two sittings. In TeX those are different things, "\_" being a
// literal underscore and "_" a subscript, so RuleMath is right to refuse it and
// the model is doing something quite reasonable one level up — an underscore in
// Markdown prose does want escaping, and a translator working through a page of
// index entries escapes the ones it is writing out. The whole file is symbols,
// so it happens on the first span and every attempt dies on line 3.
//
// Asking again is the wrong end to fix it at, for the reason Respace gives: the
// prose came back translated correctly, the question is expensive, and the next
// answer escapes it the same way. This is a repair on the same terms as Respace
// and with the same one-sidedness. It works span by span against the English it
// came from, it fires only where taking the escape off makes the span the
// English span, and where the two sides do not line up it does nothing at all.
//
// Because the test is that the result is the English, this cannot turn a wrong
// formula into an accepted one. A span the model actually changed does not
// become the English by dropping a backslash, and one whose English has a
// literal "\_" in it stays refused, since unescaping moves it away from the
// English rather than towards it.
func Unescape(en, tr string) string {
	want, unclosedEn := mathtex.Split(en)
	got, unclosedTr := mathtex.Split(tr)
	if unclosedEn != nil || unclosedTr != nil || len(want) != len(got) {
		// Not the same shape, which is RuleMath's finding to report and not
		// this function's to paper over.
		return tr
	}
	rs := []rune(tr)
	var b strings.Builder
	at := 0
	changed := false
	for i, g := range got {
		w := want[i]
		if g.Display != w.Display || !strings.Contains(g.Text, `\`) {
			continue
		}
		put := mdEscapeRE.ReplaceAllString(g.Text, "$1")
		if put == g.Text || !glossary.SameMath(w.Text, put) {
			continue
		}
		b.WriteString(string(rs[at:g.Start]))
		b.WriteString(put)
		at = g.End
		changed = true
	}
	if !changed {
		return tr
	}
	b.WriteString(string(rs[at:]))
	return b.String()
}
