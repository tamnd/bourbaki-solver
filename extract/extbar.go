package extract

import (
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// extensionFamilies is the fonts that carry the pieces of a delimiter drawn
// taller than any one glyph. It is the same list preparing a volume uses to
// pick the extension table over the mathematics one.
var extensionFamilies = map[string]bool{
	"CMEX": true, "LMMathExtension": true, "EUEX": true,
}

// barPiece reports whether a run is one piece of an extensible bar: the single bar
// of an absolute value or the double bar of a norm, which stand at codes 12 and
// 13 of the extension font and which preparing a volume names after the
// delimiter they build so that cmexDelim reads them.
func barPiece(l *pdfsrc.Layout, s pdfsrc.Span) bool {
	t := strings.TrimSpace(s.Text)
	if t != "<" && t != "=" {
		return false
	}
	return extensionFamilies[family(l.Font(s))]
}

// Extensible collapses a bar drawn as a stack of pieces to the one bar it is,
// and reports how many pieces it took out.
//
// TeX draws a delimiter taller than its tallest single glyph by repeating one
// piece down the height of what it encloses, and poppler gives back a run for
// every piece. A parenthesis drawn that way arrives as a top, a stack and a
// bottom, three different characters, which is what the tall delimiter flag
// reads. A bar has no top and no bottom: every piece of it is the same
// character, so a norm around a display two lines tall came back as six bars,
// three to a side, and the page read \|\|\|X\|\|\| where the book prints \|X\|.
//
// The pieces of one delimiter stand at one left edge, each under the last, and
// two bars set side by side stand at two left edges. That is the whole of the
// test, and it is the left edge that is asked about rather than the count,
// because |||u||| is a norm Théories spectrales really does print: three bars
// at three left edges where a tall bar is three at one.
func Extensible(l *pdfsrc.Layout, p pdfsrc.Page) (out pdfsrc.Page, dropped int) {
	out = p
	var idx []int
	for i, s := range p.Spans {
		if barPiece(l, s) {
			idx = append(idx, i)
		}
	}
	if len(idx) < 2 {
		return out, 0
	}
	sort.SliceStable(idx, func(a, b int) bool {
		x, y := p.Spans[idx[a]], p.Spans[idx[b]]
		if x.Left != y.Left {
			return x.Left < y.Left
		}
		return x.Top < y.Top
	})
	drop := make(map[int]bool)
	for a := 1; a < len(idx); a++ {
		prev, cur := p.Spans[idx[a-1]], p.Spans[idx[a]]
		if prev.Left != cur.Left || strings.TrimSpace(prev.Text) != strings.TrimSpace(cur.Text) {
			continue
		}
		// No gap worth the name between the foot of the piece above and the
		// head of this one. TeX overlaps the pieces so that the bar comes out
		// solid, so this is really asking whether the two were drawn apart on
		// purpose: the same bar opening two displays one under the other is
		// two bars and not one six pieces tall.
		if cur.Top > prev.Bottom()+prev.Height/2 {
			continue
		}
		drop[idx[a]] = true
	}
	if len(drop) == 0 {
		return out, 0
	}
	spans := make([]pdfsrc.Span, 0, len(p.Spans)-len(drop))
	for i, s := range p.Spans {
		if !drop[i] {
			spans = append(spans, s)
		}
	}
	out.Spans = spans
	return out, len(drop)
}
