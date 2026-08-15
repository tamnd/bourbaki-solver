package extract

import "strings"

// marked puts a prime back in front of the index it was drawn over.
//
// TeX sets a prime as a superscript of the base it marks, so the prime and the
// index of that base are stacked one over the other and start at the same
// place. poppler hands the runs of a line back in the order of their left edges
// and not in the order they were drawn, so the index opens to the left of the
// prime and the prime lands inside it. Page 114 of Lie 7 to 9 sets X prime with
// -alpha as its index and hands back the X, the minus, the prime and the alpha
// in that order, which is written X_-'_{\alpha}: a base carrying two subscripts,
// which KaTeX refuses. Where the index is one piece nothing fails and the page
// ships X_{\alpha}', which sets, and prints the prime after the index where the
// volume prints it before.
//
// What says the prime belongs to the base is that it was drawn over the index
// rather than after it. A prime the author wrote after the index is set after
// the box of the index and stands clear of it, and a prime the author wrote on
// the base is set over that box and overlaps it. The two do not meet, and this
// asks the page which it is rather than deciding by what the index holds.
//
// The prime of A_{(L')} is written inside the index and follows the L there, so
// it stands clear and is left where it is.
func marked(toks []token) []token {
	for i := 1; i < len(toks); i++ {
		if !tick(toks[i]) || toks[i-1].depth <= toks[i].depth {
			continue
		}
		// The index in front of the prime, which is one run of pieces at one
		// depth and one level, standing on a base one level out from it and at
		// the level the prime is written at.
		d, lv := toks[i-1].depth, toks[i-1].level
		j := i
		for j > 0 && toks[j-1].depth == d && toks[j-1].level == lv {
			j--
		}
		if j == 0 || toks[j-1].depth != d-1 || toks[j-1].depth != toks[i].depth {
			continue
		}
		if toks[i].left >= toks[i-1].right {
			continue
		}
		p := toks[i]
		p.depth, p.level = toks[j-1].depth, toks[j-1].level
		out := make([]token, 0, len(toks))
		out = append(out, toks[:j]...)
		out = append(out, p)
		out = append(out, toks[j:i]...)
		toks = append(out, toks[i+1:]...)
	}
	return toks
}

// tick reports whether a token is a prime and nothing else.
func tick(t token) bool {
	s := strings.TrimSpace(t.text)
	return s != "" && strings.Trim(s, "'") == ""
}
