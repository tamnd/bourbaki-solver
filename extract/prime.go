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

// ticked writes a prime on to the run it marks.
//
// A prime is not a script. It is a mark on a letter, the way an accent is, and
// the only reason it arrives as a run of its own is that TeX sets it out of a
// smaller font and poppler cuts a line at every change of font. Everything that
// reads a cluster of scripts afterwards reads it as a script all the same, and
// a script is placed by its depth, so a prime deep inside a cluster is placed
// against the wrong thing whenever the depth cannot say what it hangs off.
//
// Page 45 of Théories spectrales is where this shows. It prints the script L
// with q' above it and L^{p'}(X) below, and TeX runs out of script sizes at the
// second level: the p of the exponent above and the p of the exponent inside
// the index below are both set at 9 point, so both arrive at the same depth and
// nothing about the depth says that the prime of the second belongs to the
// second. It was given to the first, which left the index of the script L in
// two pieces with the (X) written as a second index of it, and KaTeX refuses a
// base with two indices by name.
//
// What the page says is where it drew the prime: against the right edge of the
// letter it marks and above the middle of it. That is asked here and the mark
// is written on to the letter, so that what follows sees a letter with a prime
// on it rather than a script of its own. Where two runs end where the prime
// begins, which is the two scripts of one base set one over the other, the
// nearer of them in height is the one it was drawn against.
//
// A prime the page drew across the index rather than after it ends nowhere near
// its own left edge and is left alone here. That one is marked, further down
// the pipeline, and it has to be: the index it was drawn over has to be read
// whole before there is anything to write the mark on.
func ticked(toks []token) []token {
	out := make([]token, 0, len(toks))
	for _, t := range toks {
		// A prime written on the line is left where it is. It is placed by what
		// stands beside it rather than by any depth, so there is nothing here
		// for it to repair, and writing it on to the letter would take the
		// letter into the formula the prime opens and move where the formula
		// begins. Page 651 of Lie 7 to 9 has eleven of them in one paragraph.
		if !tick(t) || t.depth == 0 {
			out = append(out, t)
			continue
		}
		at := -1
		for j, o := range out {
			if tick(o) || o.depth == 0 || t.left-o.right > restackGap || o.right-t.left > restackGap {
				continue
			}
			// The run it marks stands under the mark, which is the whole of
			// what tells a prime from a run that happens to end where it
			// begins.
			if o.top+o.bottom <= t.top+t.bottom {
				continue
			}
			if at < 0 || apartBy(o, t) < apartBy(out[at], t) {
				at = j
			}
		}
		if at < 0 {
			out = append(out, t)
			continue
		}
		out[at].text = strings.TrimRight(out[at].text, " ") + strings.TrimSpace(t.text)
		out[at].right = max(out[at].right, t.right)
		out[at].top = min(out[at].top, t.top)
		// A prime is mathematics whatever the letter it marks was set in. The V
		// of page 289 of Topologie algebrique comes out of the roman, which is
		// what a base level letter of a French page is more often than not, and
		// the volume prints V prime rather than the letter V.
		out[at].math = out[at].math || t.math
	}
	return out
}

// tick reports whether a token is a prime and nothing else.
func tick(t token) bool {
	s := strings.TrimSpace(t.text)
	return s != "" && strings.Trim(s, "'") == ""
}
