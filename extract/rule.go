package extract

import (
	"math"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// Bourbaki writes the difference of two sets with a minus sign, A − B, and not
// with a backslash: Theory of Sets II, §1, no. 5. Algebra VIII follows it, and
// chapter VIII prints the operator 70 times in the English volume and 69 in the
// French, mostly in a subscript, as M_{I−J}, and mostly in the proofs that take
// an index out of an index set.
//
// It does not arrive. The sign is drawn as a rule and not set as a glyph, so it
// is not in the text layer, and pdftohtml reports the two operands with nothing
// between them. Read as it stands the page says M_{I J}, which is a product of
// two index sets, and there is no mark anywhere in the output to say a sign went
// missing. That is the worst kind of extraction defect: it does not look like
// one. Before this the corpus carried 14 of them, the ones somebody happened to
// catch by eye, out of 139.
//
// page.go used to carry a note saying this operator was a backslash that
// poppler dropped, and that the hole it left could not be told from the error in
// pdftohtml's own boxes. The first half was wrong and the second half was right,
// and both are now beside the point. The bar is drawn, pdftocairo reports every
// drawn path with its geometry, and there is nothing to measure a hole against
// because the sign says where it is.

// minus is the bar Bourbaki sets between the operands of a set difference,
// told from every other horizontal line on the page by its weight and its
// length.
//
// Measured over the 505 pages of the English volume, all 527 horizontal rules
// fall into one of eleven weights. 453 of them are between 0.29 and 0.50
// points: those are the fraction bars and the overlines, and a fraction bar is
// as long as the fraction over it, up to 325 points. 70 are between 0.85 and
// 1.46 points and between 3.86 and 7.75 points long, in four groups that are
// the same bar in the four type sizes the book uses, from a second level
// subscript at 6 point to the body at 10. Those 70 are the operator.
//
// The last 4 are why length is asked about as well. The one table in the book,
// on page 124, draws its rules at 0.50 and at 0.80, and the four at 0.80 are as
// heavy as the operator at 6 point. They are 325 points long and the operator
// is never more than eight, so length tells them apart with room to spare, and
// the two tests together are exact on this volume.
func minus(r pdfsrc.Rule) bool { return r.Thickness >= 0.8 && r.Length <= 12 }

// Minus puts the drawn set difference signs back into a page's runs, and
// reports how many it placed and how many it could not.
//
// A bar lands in one of two places and the page decides which. In a subscript
// the operands are one run, "I J", because poppler reads the space TeX left for
// the rule as a space and keeps it inside the run; the sign goes in place of
// that space. At body size the operands are separate runs with a gap between
// them, and the sign goes into the gap as a run of its own, in the font of the
// operand to its left so that it is prose or mathematics exactly as they are.
//
// A bar that is neither inside a run with a space in it nor in a gap between
// two runs is not placed. There is nowhere to put it that is not a guess, and a
// guess here writes a minus sign into the middle of a formula that did not have
// one, which is worse than the hole it would be filling. Those pages are
// flagged instead and go to the repair pass.
func Minus(l *pdfsrc.Layout, p pdfsrc.Page) (out pdfsrc.Page, placed, lost int) {
	out = p
	out.Spans = append([]pdfsrc.Span(nil), p.Spans...)
	var added []pdfsrc.Span
	for _, r := range p.Rules {
		if !minus(r) {
			continue
		}
		band := onBand(l, out.Spans, r)
		if i, at, ok := inRun(out.Spans, band, r); ok {
			s := out.Spans[i]
			out.Spans[i].Text = s.Text[:at] + "-" + s.Text[at+1:]
			placed++
			continue
		}
		if s, ok := inGap(out.Spans, band, r); ok {
			added = append(added, s)
			placed++
			continue
		}
		lost++
	}
	out.Spans = append(out.Spans, added...)
	return out, placed, lost
}

// onBand is the runs a rule could be an operator of: the ones it sits
// vertically inside, at the size it was drawn at.
//
// The rule's own top is the vertical test, because a rule is a point or two
// tall and its top is inside the run it belongs to whichever way it is rounded.
//
// The size test is what keeps a sign at the level it was set at. A line that
// reads L^* − {1} has the star raised and the minus at the foot of it, so the
// star is vertically inside the bar's band and is the nearest thing to its
// left, and a reader that took the nearest thing would set the minus inside the
// superscript and write L^{*-}{1}. It did, on page 374 of the English volume,
// until this was put in. The star is set at 7 point and the bar is drawn at 10,
// and that is the whole of the difference.
func onBand(l *pdfsrc.Layout, spans []pdfsrc.Span, r pdfsrc.Rule) []int {
	var out []int
	for i, s := range spans {
		if s.Top > r.Top || r.Top >= s.Bottom() {
			continue
		}
		if r.Size > 0 && math.Abs(float64(l.Font(s).Size)-r.Size) > 2 {
			continue
		}
		out = append(out, i)
	}
	return out
}

// inRun finds the run a rule sits inside and the byte the sign replaces.
//
// Inside means the middle of the rule is inside, not the whole of it. The two
// are not the same thing: pdftohtml gives one box per run and the boxes of a
// subscript butt up against each other, so a bar between two of them overlaps
// one by a fraction of a pixel about as often as it does not. Asking where the
// middle of the sign is asks the question the page answers.
//
// The run has to hold a space, and the space has to be under the bar. Where a
// character sits inside a run is worked out by sharing the box out evenly,
// which is exact for a subscript in a single font at a single size and is the
// only case this is asked about. The nearest space is taken, and only if it is
// within a character of the bar: a run whose only space is at the other end of
// it is a run the bar is not inside, whatever its box says.
func inRun(spans []pdfsrc.Span, band []int, r pdfsrc.Rule) (int, int, bool) {
	best, at, dist := -1, -1, math.MaxFloat64
	for _, i := range band {
		s := spans[i]
		if s.Left > r.Mid() || r.Mid() >= s.Right() || s.Width == 0 {
			continue
		}
		n := len([]rune(s.Text))
		if n == 0 {
			continue
		}
		em := float64(s.Width) / float64(n)
		for j, c := range s.Text {
			if c != ' ' {
				continue
			}
			k := len([]rune(s.Text[:j]))
			mid := float64(s.Left) + float64(s.Width)*(float64(k)+0.5)/float64(n)
			d := math.Abs(mid - float64(r.Mid()))
			if d < dist && d <= em {
				best, at, dist = i, j, d
			}
		}
	}
	return best, at, best >= 0
}

// inGap makes the run a rule becomes when it sits between two runs rather than
// inside one. There has to be a run ending before the middle of the rule and a
// run starting after it, and nothing straddling it, or the rule is over
// something and this is not the case it is.
//
// The middle again, and not the ends. The gap a bar sits in is narrower than
// the bar about half the time, because the boxes either side of it are worked
// out by sharing a box out and they lap over it by a pixel. Measuring the gap
// and asking whether the bar fits refuses those, and they are the same bar in
// the same place as the ones it accepts.
func inGap(spans []pdfsrc.Span, band []int, r pdfsrc.Rule) (pdfsrc.Span, bool) {
	left, right := -1, -1
	for _, i := range band {
		s := spans[i]
		switch {
		case s.Right() <= r.Mid():
			if left < 0 || s.Right() > spans[left].Right() {
				left = i
			}
		case s.Left > r.Mid():
			if right < 0 || s.Left < spans[right].Left {
				right = i
			}
		default:
			return pdfsrc.Span{}, false
		}
	}
	if left < 0 || right < 0 {
		return pdfsrc.Span{}, false
	}
	s := spans[left]
	return pdfsrc.Span{
		Top:    s.Top,
		Left:   r.Left,
		Width:  r.Width,
		Height: s.Height,
		Font:   s.Font,
		Text:   "-",
	}, true
}

// MinusCount is how many set difference signs a page draws, whether or not any
// of them can be placed. It is what a caller reports against.
func MinusCount(p pdfsrc.Page) int {
	n := 0
	for _, r := range p.Rules {
		if minus(r) {
			n++
		}
	}
	return n
}
