package translate

import (
	"strings"

	"github.com/tamnd/bourbaki-solver/mathtex"
)

// Respace puts the English spacing back into an answer that re-laid out a
// formula, and leaves everything else exactly as the model wrote it.
//
// A model asked to translate a paragraph reflows the source of the mathematics
// in it as a matter of course. It writes $M \cap N$ where the book has
// $M\cap N$, it breaks a display over three lines, it re-indents an aligned
// block. None of that changes what is set: in mathematics mode the whitespace
// TeX ignores is whitespace TeX ignores. The corpus still wants the English
// bytes between the dollars, so that a span can be compared across four
// languages without a normaliser standing in the middle, and RuleMath refuses
// an answer that does not have them.
//
// Refusing is the wrong end to fix it at. The answer is a correct translation
// of the prose, the question took five minutes, and asking it again gets the
// same formula laid out some third way. So the span is put back to the English
// with the translated words kept in it, and the answer goes to the audit with
// nothing left for RuleMath to find. A span that differs anywhere else is not
// touched and is refused as before.
//
// It is a repair and not a normaliser: it works span by span against the
// English it came from, it moves nothing but whitespace outside a \text, and
// where the two sides do not line up it does nothing at all.
func Respace(en, tr string) string {
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
		if g.Text == w.Text || g.Display != w.Display || !sameButForSpace(w.Text, g.Text) {
			continue
		}
		put, ok := graft(w.Text, g.Text)
		if !ok {
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

// sameButForSpace says whether two spans are the same mathematics laid out
// differently, with the words inside them left out of the question.
func sameButForSpace(en, tr string) bool {
	e, t := mathtex.MaskText(en), mathtex.MaskText(tr)
	return e != t && mathtex.SqueezeSpace(e) == mathtex.SqueezeSpace(t)
}

// graft is the English span with the translation's words in it.
//
// The runs are walked backwards so that replacing one does not move the next,
// since their positions are counted in the string being cut up. The two sides
// hold the same number of runs, because the masks agree once the spacing is
// out and each run is one character of a mask.
func graft(en, tr string) (string, bool) {
	e, t := mathtex.TextRuns(en), mathtex.TextRuns(tr)
	if len(e) != len(t) {
		return "", false
	}
	rs := []rune(en)
	for i := len(e) - 1; i >= 0; i-- {
		rs = append(rs[:e[i].Start:e[i].Start], append([]rune(t[i].Text), rs[e[i].End:]...)...)
	}
	return string(rs), true
}
