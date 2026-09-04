package translate

import (
	"strings"

	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/mathtex"
)

// mostMerged is the most answer spans that will be put back into one. An entry
// of an index lists two or three notations for the same thing and the longest
// in content/en lists five, so eight is the shape with room over it, and a cap
// keeps the search off a pathological answer.
const mostMerged = 8

// Remerge puts back together the spans of an answer that broke one formula of
// the English into several, and leaves everything else exactly as the model
// wrote it.
//
// The indexes of notation are what this is about. An entry of one lists the
// notations that share a definition, and the English sets the list as a single
// span: $\langle f, z' \rangle, \langle z', f \rangle$ is one formula with a
// comma in it. A model reading that as prose sets each notation in its own
// dollars and writes the comma between them as text, which is the same line on
// the page and a different file, and RuleMath counts 51 spans where the English
// has 33. The index of notation of Integration I-VI was refused on that fifteen
// times, and the four indexes among the eight Vietnamese files that would not
// land are all this shape or its neighbours.
//
// The repair is on the terms Respace and Unescape set. It works against the
// English span it came from, and it fires only where the answer's spans and the
// text the model wrote between them assemble into the English span exactly. So
// it cannot merge two formulae that were separate in the English, and it cannot
// turn a formula the model altered into one that passes: the test is that the
// result is the English.
//
// Where the two sides do not line up all the way it does nothing at all. A
// partial alignment would be a repair that guessed which of the answer's spans
// went with which of the English's, and putting a formula in the wrong place is
// worse than the refusal it was trying to avoid.
func Remerge(en, tr string) string {
	want, unclosedEn := mathtex.Split(en)
	got, unclosedTr := mathtex.Split(tr)
	if unclosedEn != nil || unclosedTr != nil || len(got) <= len(want) {
		// Fewer spans than the English, or the same number, is not this fault.
		// A short answer is RuleMath's finding to report.
		return tr
	}
	rs := []rune(tr)
	var b strings.Builder
	at, j := 0, 0
	merged := false
	for _, w := range want {
		if j >= len(got) {
			return tr
		}
		if got[j].Display == w.Display && glossary.SameMath(w.Text, got[j].Text) {
			j++
			continue
		}
		k, ok := spansThatAssemble(rs, got, j, w)
		if !ok {
			return tr
		}
		b.WriteString(string(rs[at:got[j].Start]))
		b.WriteString(w.Text)
		at = got[k].End
		j, merged = k+1, true
	}
	if !merged || j != len(got) {
		// Spans of the answer left over past the last of the English is an
		// answer that invented one, which is a different finding and not one to
		// paper over by dropping it.
		return tr
	}
	b.WriteString(string(rs[at:]))
	return b.String()
}

// spansThatAssemble is the last of the answer's spans, starting at j, whose
// texts and the text the model wrote between them come to the English span w.
//
// The run between two spans is taken without the two dollars that close one and
// open the next, which is what the model added and what putting them back into
// one formula takes out again. Displays are left out: a display written $$ has
// two delimiters either side and no entry of an index is one.
func spansThatAssemble(rs []rune, got []mathtex.Span, j int, w mathtex.Span) (int, bool) {
	if w.Display || got[j].Display {
		return 0, false
	}
	var b strings.Builder
	b.WriteString(got[j].Text)
	for k := j + 1; k < len(got) && k-j < mostMerged; k++ {
		if got[k].Display || got[k].Start-1 < got[k-1].End+1 {
			return 0, false
		}
		b.WriteString(string(rs[got[k-1].End+1 : got[k].Start-1]))
		b.WriteString(got[k].Text)
		if glossary.SameMath(w.Text, b.String()) {
			return k, true
		}
	}
	return 0, false
}
