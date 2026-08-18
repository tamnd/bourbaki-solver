package extract

import (
	"sort"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// A horizontal bar over a group of runs is not in the text layer at all. TeX
// draws the rule of a fraction and the bar of a closure, a conjugate and a mean
// as a path, the same way it draws the set difference sign rule.go puts back,
// and pdftohtml reports the runs either side of it and nothing between them.
//
// The two faults that leaves are not the same size. A fraction set inline comes
// back as its two halves interleaved, since TeX sets them at the size and the
// height a superscript and a subscript are set at and the layer reports a page
// left to right: P(z) over Q(z) on page 99 of Théories spectrales I arrives as
// "P(", "Q(", "z", "z", ")", ")", and written in that order it says
// ^{P(}_{Q(}^z_z^)_), which KaTeX refuses by name. That one is at least loud.
//
// A bar drawn over one thing is silent. The conjugate of f_1 comes back as f_1,
// the closure of A comes back as A, and there is no mark anywhere in the output
// to say a bar went missing. Page 431 of Théories spectrales V writes the inner
// product out as the integral of the conjugate of f_1 against gamma(g)f_2, and
// the corpus has been carrying it as f_1 with no bar since the volume was
// extracted. Nothing counts that, no gate sees it, and the page reads as a
// statement the book does not make.
//
// Both are read the same way, because the page draws the same thing in both
// cases: a thin rule with material under it, and for a fraction material over
// it as well. What is over and what is under is measured against the rule and
// not read off the levels, since a fraction set in the text style has both
// halves at full size and the levels then say nothing.

// bar reports whether a drawn rule is one TeX sets over material rather than
// beside it.
//
// Weight is the whole of the test. Measured over the English Algebra VIII, the
// 527 horizontal rules of the volume fall into two groups by thickness: 453 are
// between 0.29 and 0.50 points and are the fraction bars and the overlines, and
// 70 are between 0.85 and 1.46 and are the set difference sign. rule.go takes
// the heavy ones and this takes the light ones, and there is nothing in
// between.
//
// Length is not asked about here the way it is asked about for the sign. A
// fraction bar is as long as the wider of its two halves, which runs from four
// points for 1 over 2 up to the width of the measure for a display, so length
// says nothing about what a light rule is. What keeps the four table rules of
// page 124 out is what is drawn around them rather than the rule itself: see
// laden.
func bar(r pdfsrc.Rule) bool { return r.Thickness > 0 && r.Thickness < 0.8 }

// bars picks the rules of a page that belong to each line, and is what puts
// them within reach of a reading of the runs.
//
// A bar goes to the line whose material it is drawn over, which is the line it
// is nearest to vertically among the ones it stands across. Nearest and not
// inside: the bar of a closure is drawn above the top of the letter it covers,
// so it lies outside the extent of its own line by a unit or two and inside the
// extent of nothing else. Page 431 of Théories spectrales V draws the bar of
// the conjugate of f_1 at 621 where the f begins at 624 and the line above ends
// at 607, and three units beats fourteen.
//
// The extent is taken over the runs and not from the band, because a line
// carries scripts above and below the type it is set in and a fraction bar sits
// between two of them.
func bars(lines []Line, rules []pdfsrc.Rule) {
	for _, r := range rules {
		if !bar(r) {
			continue
		}
		best, dist, bestOver := -1, 0, false
		for i, l := range lines {
			if r.Right() <= l.Left || r.Left >= l.Right {
				continue
			}
			d, o := off(r.Top, l), drawnOver(r, l)
			// Two lines can both hold the bar inside their extent, and then the
			// one with type under the bar is the one the bar was drawn over. A
			// line that took a script off the line under it reaches down to the
			// top of that line, so the bar over the first letter of the next
			// line lies inside both of them, and the line above is offered
			// first. Page 113 of Theories spectrales sets sup with x in X under
			// it at 494, and the bar of the conjugate of f on the line at 505
			// went up to the sup, which has nothing under it to bar, and the
			// page came out reading f maps to f.
			if best < 0 || d < dist || (d == dist && o && !bestOver) {
				best, dist, bestOver = i, d, o
			}
		}
		// barReach is how far a bar may stand outside the extent of the line
		// it belongs to. A closure bar clears the tallest letter under it by a
		// unit or two and a fraction bar is inside its line by construction, so
		// this only has to cover the first of the two.
		const barReach = 6
		if best < 0 || dist > barReach {
			continue
		}
		lines[best].Rules = append(lines[best].Rules, r)
	}
}

// stackReach is how many lines a fraction may be scanned across. A fraction
// built up in a display is a numerator, a rule and a denominator, and what can
// come between them is the row the operators of the display sit on, which is
// one line. Nothing legitimate is further apart than that, and a rule this has
// mistaken for a fraction bar then joins two paragraphs.
const stackReach = 2

// builtUp joins the lines a built up fraction was scanned into.
//
// A fraction set in a display is taller than the type around it. TeX puts the
// numerator well above the axis of the line and the denominator well below it,
// far enough that the scan finds three bands where the page prints one line,
// and neither half is a script: both are set at the size of the body, so the
// tests that put a stray script back on its line refuse them, since what they
// ask is that the stray be smaller than what it came off.
//
// The rule is the evidence, and it is the same evidence laden reads. A bar with
// material above it and material below it is a fraction bar, so the line
// holding what is above and the line holding what is below are one line of the
// page and everything between them is on it too. Page 33 of Théories spectrales
// I sets the Fekete inequality as lim sup of a_n over n, and the three bands
// came out as a row of numerators, a row of operators with their bounds gone,
// and a row of denominators mixed in with the bounds: the volume shipped
// "lim sup $\leqslant$ inf $\leqslant$ lim inf." as a sentence, followed by
// "_{n\rightarrow+\infty}n_{m\geqslant 1}m_n^{\rightarrow}_+^{\infty}n" as a
// display, which is a double subscript KaTeX refuses and no fraction anywhere.
func builtUp(lines []Line, rules []pdfsrc.Rule) []Line {
	if len(lines) < 2 {
		return lines
	}
	// reach[i] is the last line the line at i has to be read together with.
	reach := make([]int, len(lines))
	for i := range reach {
		reach[i] = i
	}
	for _, r := range rules {
		if !bar(r) {
			continue
		}
		first, last := -1, -1
		for i, l := range lines {
			above, below := false, false
			for _, run := range l.Runs {
				if !halved(run, r) {
					continue
				}
				// Measured against the rule and not against the band,
				// for the reason laden gives: a display fraction sets
				// both halves at full size and the levels say nothing.
				switch {
				case run.Bottom() <= r.Top && r.Top-run.Bottom() <= run.Height:
					above = true
				case run.Top >= r.Top && run.Top-r.Top <= run.Height:
					below = true
				}
			}
			if above {
				first = i // the nearest line above, so the last one found
			}
			if below && last < 0 {
				last = i // the nearest line below, so the first one found
			}
		}
		if first < 0 || last < 0 || first >= last || last-first > stackReach {
			continue
		}
		reach[first] = max(reach[first], last)
	}
	out := lines[:0:0]
	for i := 0; i < len(lines); {
		end := reach[i]
		for j := i; j <= end && j < len(lines); j++ {
			end = max(end, reach[j])
		}
		if end == i {
			out = append(out, lines[i])
			i++
			continue
		}
		out = append(out, oneLine(lines[i:end+1]))
		i = end + 1
	}
	return out
}

// halved reports whether a run could be one half of the fraction a rule is the
// bar of, which is that the rule reaches across the whole of it.
//
// A fraction bar is drawn as long as the wider of the two halves, so neither
// half stands outside it, and asking for that is what keeps the bar of a
// conjugate from reading the prose above it as a numerator. Page 113 of
// Théories spectrales draws the bar of the conjugate of f at 506 over ten
// units, and the line above it opens with sixty units of French that lie
// across those ten and end fourteen units higher up.
func halved(run Run, r pdfsrc.Rule) bool {
	return run.Left >= r.Left && run.Right() <= r.Right()
}

// oneLine reads a run of lines as the one line they are.
//
// The band is taken from whichever of them carries the most body type rather
// than from the first, because the first is the numerator and the band is what
// says whether a run arriving later is body type or something hanging off it.
// The Fekete display puts three numerators on its first band and its lim sup,
// its two inequality signs and its lim inf on the second.
func oneLine(lines []Line) Line {
	at, best := 0, -1
	for i, l := range lines {
		size, n := 0, 0
		for _, r := range l.Runs {
			if r.Spec.Size > size {
				size, n = r.Spec.Size, 0
			}
			if r.Spec.Size == size {
				n++
			}
		}
		if n > best {
			at, best = i, n
		}
	}
	out := lines[at]
	out.Runs = nil
	for _, l := range lines {
		out.Runs = append(out.Runs, l.Runs...)
		out.Left, out.Right = min(out.Left, l.Left), max(out.Right, l.Right)
	}
	return out
}

// drawnOver reports whether a line sets type under a bar, across the width the
// bar is drawn. That is what a bar is for, and a line with nothing under it did
// not have this one drawn over it however near it stands.
func drawnOver(r pdfsrc.Rule, l Line) bool {
	for _, run := range l.Runs {
		if run.Left < r.Right() && r.Left < run.Right() && run.Top >= r.Top {
			return true
		}
	}
	return false
}

// off is how far a height stands outside the extent of a line's runs, and zero
// if it is inside.
func off(top int, l Line) int {
	lo, hi := l.Top, l.Bottom
	for _, r := range l.Runs {
		lo, hi = min(lo, r.Top), max(hi, r.Bottom())
	}
	switch {
	case top < lo:
		return lo - top
	case top > hi:
		return top - hi
	}
	return 0
}

// barred writes the fractions and the overlines of a line back into its tokens.
//
// The narrowest bar is taken first, so that a bar drawn over part of what
// another bar covers is written before the one that covers it and the outer bar
// then finds it as a single token. The mean of a conjugate is set that way and
// so is a fraction whose numerator carries a closure.
func barred(toks []token, rules []pdfsrc.Rule) []token {
	if len(rules) == 0 {
		return toks
	}
	rs := append([]pdfsrc.Rule(nil), rules...)
	sort.SliceStable(rs, func(i, j int) bool { return rs[i].Width < rs[j].Width })
	for _, r := range rs {
		if t, at, end, ok := laden(toks, r); ok {
			out := append([]token(nil), toks[:at]...)
			out = append(out, t)
			toks = append(out, toks[end:]...)
		}
	}
	return toks
}

// laden reads what a bar is drawn over and gives back the one token the two
// make, with the stretch of tokens it replaces.
//
// The tokens a bar covers are the ones inside what it spans, and they are
// contiguous, since a line is ordered left to right and a bar is drawn across
// one stretch of it. A token that starts inside the span and ends outside it is
// refused rather than cut: a bar whose edge falls in the middle of a run is a
// bar this has not understood, and half a closure is worse than none.
//
// Above and below are measured against the rule. A fraction set in the display
// style has both halves at full size, so their levels say nothing, and a
// closure over an index has its material below a rule that is itself well above
// the line, so the line's own band says nothing either. The rule is the axis
// the page drew, and both halves are placed against it.
//
// A bar with material above it and none below is not a bar of anything. TeX
// draws no such thing and a rule that measures that way is a rule this has
// mistaken for one, most likely the frame of a table: page 124 of Algebra VIII
// draws the only table in the volume and its rules run the width of the measure
// with a row of type above each of them. Those are refused here, by the same
// test that refuses a stray path, rather than by a threshold on the length.
func laden(toks []token, r pdfsrc.Rule) (token, int, int, bool) {
	span := [2]int{r.Left, r.Right()}
	at, end := -1, -1
	for i, t := range toks {
		if !within([2]int{t.left, t.right}, span) {
			if at >= 0 && end < 0 {
				end = i
			}
			continue
		}
		if end >= 0 {
			// A second stretch under one bar, with something outside the span
			// between them. Nothing on a page is drawn that way.
			return token{}, 0, 0, false
		}
		if at < 0 {
			at = i
		}
	}
	if at < 0 {
		return token{}, 0, 0, false
	}
	if end < 0 {
		end = len(toks)
	}
	// barSlack is how far a piece of type may cross the height of the rule and
	// still be read as standing wholly on one side of it, in the pixel units of
	// the page. The two are set edge to edge, so the boxes meet and the
	// rounding pdftohtml does on a box puts them a unit either way.
	const barSlack = 2
	var over, under []token
	for _, t := range toks[at:end] {
		switch {
		case t.bottom <= r.Top+barSlack:
			over = append(over, t)
		case t.top >= r.Top-barSlack:
			under = append(under, t)
		default:
			// Drawn through the rule, which neither half of a fraction is and
			// nothing under a closure is.
			return token{}, 0, 0, false
		}
	}
	if len(under) == 0 {
		return token{}, 0, 0, false
	}
	// A bar is drawn against the top of what it covers, so the tallest thing
	// under it begins at the rule and not a row further down. The two bars of
	// the matrix on page 362 of Algebra VIII are drawn at 236 and at 247, one
	// over the y of the top row and one over the x of the row below, and the
	// top row is written in a run the first of them covers only a part of.
	// Nothing in that row is a run the bar spans, so the bar was given to the
	// x of the row below, which is the next thing to the right of it, and that
	// x came out with two bars on it in a matrix that then read a row short.
	//
	// The measurement is made on the whole of what the bar covers and not run
	// by run, since an index under a closure is set below the letter it hangs
	// off and is no less under the bar for that. Depth is what says a row: the
	// tallest piece of type under a bar reaches the rule within its own height,
	// and a row further down begins beyond it.
	if highest(under)-r.Top > deep(under) {
		return token{}, 0, 0, false
	}
	// What stands above a bar is the numerator of a fraction only if the bar was
	// drawn beside the type before it rather than under it.
	//
	// TeX leaves a nulldelimiterspace on each side of a fraction that carries no
	// delimiters, and leaves nothing at all beside a script: a superscript is set
	// flush against the box of the base it hangs off. So a rule that starts at or
	// before the end of the run in front of it is the bar over an index, with the
	// exponent of that same base standing beside it, and a rule that starts clear
	// of it is a fraction. Measured over the six volumes the two do not meet:
	// every one of the fractions the page really sets stands at least one unit
	// clear, and the eleven readings that are not fractions all touch.
	//
	// The reading this refuses is one shape and it is a common one. Page 431 of
	// Théories spectrales V sets the space L^2 with a barred chi as its index,
	// which the layer hands back as a superscript and a subscript at the same
	// left edge with a rule between them; read as a fraction it says two over chi
	// and the page then states a lemma about a space that is not in the book.
	// Page 268 does the same with the multiplication operator by g bar to the
	// ell, which the volume itself writes out in full on the other side of the
	// equals sign.
	//
	// The run in front has to be one a script could hang off. A fraction after an
	// opening bracket is a fraction whatever the rounding of the boxes says, and
	// the bracket carries no exponent.
	//
	// What is above the bar is dropped in that case and the bar is read as what
	// it is, a bar over the index alone. Dropping is only safe where the
	// superscript stands at one end of what the bar spans, since the tokens this
	// gives back have to be one stretch of the line.
	if len(over) > 0 && at > 0 && carries(toks[at-1]) && r.Left <= toks[at-1].right {
		lo, hi := at, end
		for lo < end && toks[lo].bottom <= r.Top+barSlack {
			lo++
		}
		for hi > lo && toks[hi-1].bottom <= r.Top+barSlack {
			hi--
		}
		for _, t := range toks[lo:hi] {
			if t.bottom <= r.Top+barSlack {
				return token{}, 0, 0, false
			}
		}
		at, end, over = lo, hi, nil
	}
	if !notation(toks[at:end]) {
		return token{}, 0, 0, false
	}
	out := token{class: ClassMath, math: true,
		left: r.Left, right: r.Right(), top: toks[at].top, bottom: toks[end-1].bottom}
	held := [2]int{toks[at].left, toks[at].right}
	for _, t := range toks[at:end] {
		out.top, out.bottom = min(out.top, t.top), max(out.bottom, t.bottom)
		held = [2]int{min(held[0], t.left), max(held[1], t.right)}
	}
	// The bar has to reach across what it is drawn over and no further. TeX
	// draws the bar of a closure at exactly the width of the box under it and
	// the rule of a fraction at the width of the wider half, so the two ends
	// meet to within the overhang one glyph box carries past another, and a
	// rule that only happens to pass over a stretch of a line does not meet
	// them at all.
	//
	// This is what keeps a display off a line of its own text. Page 346 of Lie
	// 7 to 9 sets the exponential series as a display and its fraction bar,
	// drawn at 586 from 223 to 272, lands inside the extent of the line above
	// rather than of the numerator it belongs to, because the numerator was
	// gathered as a line of its own. Nothing on that line is inside the span,
	// so it is refused here; a display whose two halves land on two lines is
	// out of reach of this and stays as it arrived.
	if !within(span, held) {
		return token{}, 0, 0, false
	}
	// The cluster keeps the reach it had, which is the bar and the type under
	// it together. Page 31 of Théories spectrales III sets 1 over 2 plus nM
	// over 2nM and ends the line "= 1, et par suite": the M of the denominator
	// reaches a unit past the end of the bar, and a cluster measured by the bar
	// alone left a gap of nine where the reader's word space is eight, so the
	// formula closed and the page said "= 1" outside it.
	out.left, out.right = min(out.left, held[0]), max(out.right, held[1])
	if len(over) == 0 {
		// A bar over one thing is the closure, the conjugate or the mean, and
		// it stands where the thing it covers stood.
		out.depth, out.level = shallowest(under)
		out.text = `\overline{` + inner(relevel(under), out.depth) + `}`
		return out, at, end, true
	}
	// A fraction stands on the line whatever its halves were set at. The two
	// are one level in from it, or at its own level where TeX set the fraction
	// in the text style and gave both halves full size, and anything deeper
	// than that is a fraction inside a script, which has a level this cannot
	// read off the page and is left as it arrived.
	//
	// Each half is measured from its own shallowest piece, because the two are
	// separate groups and neither is written inside the other. Page 164 of Lie 7
	// to 9 sets a table of roots the gathering has already run together, and its
	// three halves and one halves arrive one level apart; measured against the
	// pair they came out as a subscript with nothing in front of it.
	n, _ := shallowest(over)
	u, _ := shallowest(under)
	if min(n, u) > 1 {
		return token{}, 0, 0, false
	}
	out.text = `\frac{` + inner(relevel(over), n) + `}{` + inner(relevel(under), u) + `}`
	return out, at, end, true
}

// carries reports whether a token is one a script could hang off, which is
// anything that ends in a letter, a digit, a prime or a closing bracket. An
// opening bracket, an operator and a comma carry nothing.
func carries(t token) bool {
	r, _ := utf8.DecodeLastRuneInString(strings.TrimRight(t.text, " "))
	return unicode.IsLetter(r) || unicode.IsDigit(r) || strings.ContainsRune(`}])'`, r)
}

// relevel works out again which side of its parent each piece of a group sits
// on, now that the group is known to be one.
//
// The levels a line arrives with cannot be right inside a fraction. finish
// reads them by walking the line left to right and hanging each run off the
// last shallower run before it, which is the term it indexes everywhere else on
// a page; under a bar it is the wrong half. The exponent of the denominator of
// (2k+1) over 2^n on page 120 of Théories spectrales III sits at 440 against a
// denominator at 441 and a numerator at 430, and hung off the numerator it read
// as an index and the page said 2 sub n where it prints 2 to the n.
//
// Which half a piece belongs to is settled by then, since the bar has said so,
// so the walk is the same walk over one half of it and the answer comes out
// right for the same reason it comes out right on an ordinary line.
func relevel(toks []token) []token {
	base, _ := shallowest(toks)
	out := append([]token(nil), toks...)
	var parent []token
	for i := range out {
		t := &out[i]
		d := t.depth - base
		if d <= 0 || len(parent) == 0 {
			parent = append(parent[:0], *t)
			continue
		}
		for len(parent) < d {
			parent = append(parent, parent[len(parent)-1])
		}
		p := parent[d-1]
		if t.top < p.top+((p.bottom-p.top)-(t.bottom-t.top))/2 {
			t.level = Sup
		} else {
			t.level = Sub
		}
		parent = append(parent[:d], *t)
	}
	return out
}

// shallowest is the depth and the level of the outermost token of a group,
// which is where the group as a whole stands on its line.
func shallowest(toks []token) (int, Level) {
	d, l := toks[0].depth, toks[0].level
	for _, t := range toks[1:] {
		if t.depth < d {
			d, l = t.depth, t.level
		}
	}
	return d, l
}

// notation reports whether a group is one a bar could have been drawn over, which
// is to say that none of it is a word of the sentence around it.
//
// A light rule with a line of type under it is a rule of a table and not a bar,
// and a table is the one thing in these volumes that draws one. Asking what is
// under it asks the question directly, where a threshold on the length would
// only stand in for it: mathWord is what the extractor already uses to decide
// where a formula stops, and a bar is drawn over mathematics or it is drawn
// over nothing.
func notation(toks []token) bool {
	for _, t := range toks {
		if t.math || t.class.Math() {
			continue
		}
		for _, w := range strings.Fields(t.text) {
			if !mathWord(w) {
				return false
			}
		}
	}
	return true
}

// inner writes a group of tokens as the mathematics they say, with each script
// nested inside the one it hangs off.
//
// This is emit's work done over again on a stretch of a line, and it is done
// again rather than reused because emit writes a line: it opens and closes the
// dollar signs, it cuts the prose out of the formula and it puts back the
// spaces the layer did not report. None of that belongs inside a fraction, and
// what does belong is only the nesting.
func inner(toks []token, depth int) string {
	var b strings.Builder
	var prev *token
	for i := 0; i < len(toks); i++ {
		if toks[i].depth <= depth {
			if prev != nil && parted(*prev, toks[i]) {
				b.WriteByte(' ')
			}
			abut(&b, toks[i].text)
			prev = &toks[i]
			continue
		}
		j := i
		for j < len(toks) && toks[j].depth > depth {
			j++
		}
		mark := "^"
		if toks[i].level == Sub {
			mark = "_"
		}
		// An index of one character needs no braces, and f_1 reads better than
		// f_{1} in a file people are meant to read. emit writes them that way
		// everywhere else and this is the same corpus.
		if s := inner(toks[i:j], depth+1); len([]rune(s)) == 1 {
			b.WriteString(mark + s)
		} else {
			b.WriteString(mark + "{" + s + "}")
		}
		prev = &toks[j-1]
		i = j - 1
	}
	return strings.TrimSpace(b.String())
}

// parted reports whether the white between two runs of a group is a space of
// the sentence rather than the fit of the letters.
//
// Only the white beside a word counts. emit lets two runs of mathematics touch
// wherever the layer put them, because the space between one symbol and the
// next is the setting and not the reading, and a fraction is written the same
// way. A word is different, because emit writes a word outside the dollar
// signs where the space beside it survives, and inside a fraction there is no
// outside to write it to. Page 346 of Lie 7 to 9 sets the determinant of
// exp ad x - 1 over ad x, with the exp ad of the numerator ending at 452 and
// the x opening at 456, and run together it reads exp adx-1 over adx.
func parted(a, b token) bool {
	if symbolic(a) && symbolic(b) {
		return false
	}
	return b.left-a.right >= spaceGap(b)
}

// symbolic reports whether a token is mathematics rather than a word, which
// here is the face it came back in and nothing else.
//
// The math field of a token is not the test. It is set for anything written
// off the line as well as for anything in a mathematical face, and both halves
// of a fraction are written off the line, so every token under a bar has it
// set and it would say the exp ad of page 346 is a symbol.
func symbolic(t token) bool { return t.class.Math() }
