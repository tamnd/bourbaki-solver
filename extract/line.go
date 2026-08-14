package extract

import (
	"math"
	"regexp"
	"slices"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// pdftohtml emits the runs of a page in the order the glyphs were drawn, which
// is not the order they are read in. Putting them back into lines is geometry,
// and the geometry is exact: every run comes with its box.
//
// A line is gathered around the tallest run in it, because that is the one
// sitting on the baseline. Everything shorter that overlaps it vertically
// belongs to the same line, and where it sits inside the line says what it is:
// higher than the middle and smaller is a superscript, lower is a subscript.
// That is how a page keeps its exponents and its indices, which is the whole
// difference between reading M_i and reading Mi.

// Level is where a run sits against the baseline of its line.
type Level int

const (
	Base Level = iota
	Sup
	Sub
)

// Run is one span of text with everything known about it.
type Run struct {
	pdfsrc.Span
	Spec  pdfsrc.FontSpec
	Class Class
	Level Level
	// Depth is how far the run is nested below the line. An index is one
	// deep, the index of an index is two. It is read off the size of the
	// font, since TeX sets each level smaller than the one above it.
	Depth int
}

// Line is one line of a page.
type Line struct {
	Runs []Run

	// Top and Bottom are the band of the body type of the line. Left and
	// Right are the extent of everything in it.
	Top, Bottom, Left, Right int

	// band is the font size the band was taken from. It is what says whether
	// the next run to arrive is body type or something hanging off it.
	band int
}

// Height of the line's band.
func (l Line) Height() int { return l.Bottom - l.Top }

// Mid of the line's band.
func (l Line) Mid() float64 { return float64(l.Top+l.Bottom) / 2 }

// Lines gathers the runs of one page into lines, in reading order.
func Lines(l *pdfsrc.Layout, p pdfsrc.Page) []Line {
	lines, _ := LinesColumns(l, p)
	return lines
}

// LinesColumns gathers the runs of one page into lines and says whether the
// page was set in two columns, which the index at the back of the volume is and
// which assembly has to know.
func LinesColumns(l *pdfsrc.Layout, p pdfsrc.Page) ([]Line, bool) {
	runs := make([]Run, 0, len(p.Spans))
	for _, s := range p.Spans {
		spec := l.Font(s)
		s.Text = Unligature(s.Text)
		runs = append(runs, Run{Span: s, Spec: spec, Class: Classify(spec, s)})
	}
	lines := rows(runs)
	x, ok := gutter(lines)
	if !ok {
		return lines, false
	}
	var left, right []Run
	for _, r := range runs {
		if r.Left < x {
			left = append(left, r)
		} else {
			right = append(right, r)
		}
	}
	return append(rows(left), rows(right)...), true
}

// ligatures are the single characters a font uses for two or three letters set
// together. The 2012 French printing carries them and the 2023 English one does
// not, so a French page arrives with suﬃt, ﬁni and déﬁnir in it, which reads
// correctly on screen and matches nothing anybody searches for.
var ligatures = strings.NewReplacer(
	"ﬀ", "ff", "ﬁ", "fi", "ﬂ", "fl",
	"ﬃ", "ffi", "ﬄ", "ffl", "ﬅ", "st", "ﬆ", "st",
)

// Unligature writes the letters a ligature stands for.
//
// The æ and œ of French are not here. They are letters of the alphabet the
// volume is set in rather than two letters drawn as one, and the volume spells
// œuvre and cæsius with them on purpose.
func Unligature(s string) string {
	for _, c := range s {
		if c >= 'ﬀ' && c <= 'ﬆ' {
			return ligatures.Replace(s)
		}
	}
	return s
}

// gutter finds the strip between two columns of type, and reports where it is.
//
// The index at the back of the volume is set in two columns, and a page set in
// two columns has no lines in the sense the rest of this file uses: an entry in
// one column and an entry in the other sit at the same height and are not read
// one after the other. Gathering them into one line puts the heading of the
// second column in the middle of an entry of the first, and the sizes on that
// line then read as an entry indexed by a heading.
//
// What is counted is lines with ink in the strip, not runs. A page of prose
// sets most of its lines the full width of the measure, so most of its lines
// have ink anywhere one looks; a page in columns has ink in the strip only
// where an entry has run long, and this volume has two such entries on its
// first index page and a title across the top. Counting runs instead makes a
// page of prose look empty in the middle, because a line of prose is often one
// run and one run is easy to set aside as a title.
func gutter(lines []Line) (int, bool) {
	if len(lines) < 12 {
		return 0, false
	}
	lo, hi := lines[0].Left, lines[0].Right
	for _, l := range lines {
		lo, hi = min(lo, l.Left), max(hi, l.Right)
	}
	width := hi - lo
	if width < 200 {
		return 0, false
	}
	cross := make([]int, width+1)
	for _, l := range lines {
		seen := make([]bool, width+1)
		for _, r := range l.Runs {
			for x := max(r.Left, lo); x <= r.Right() && x-lo < len(seen); x++ {
				seen[x-lo] = true
			}
		}
		for x, s := range seen {
			if s {
				cross[x]++
			}
		}
	}
	limit := max(1, len(lines)/10)
	best, bestAt := 0, 0
	for x := width / 4; x < 3*width/4; x++ {
		if cross[x] > limit {
			continue
		}
		n := 0
		for x+n < 3*width/4 && cross[x+n] <= limit {
			n++
		}
		if n > best {
			best, bestAt = n, x
		}
		x += n
	}
	// gutterWidth is how wide the strip has to be. The measure of this volume
	// is about 500 units across and the space between two words is five.
	const gutterWidth = 25
	if best < gutterWidth {
		return 0, false
	}
	at := lo + bestAt + best/2
	left, right := 0, 0
	for _, l := range lines {
		for _, r := range l.Runs {
			if r.Left < at {
				left++
			} else {
				right++
			}
		}
	}
	// Both sides have to carry a column's worth of type. One line of an
	// equation standing well to the right of the margin leaves a strip too, and
	// it is not a column.
	return at, left >= 8 && right >= 8
}

// rows gathers one column of runs into lines.
func rows(runs []Run) []Line {
	if len(runs) == 0 {
		return nil
	}
	sort.SliceStable(runs, func(i, j int) bool {
		if runs[i].Top != runs[j].Top {
			return runs[i].Top < runs[j].Top
		}
		return runs[i].Left < runs[j].Left
	})

	var lines []Line
	var cur *Line
	var held []Run
	for _, r := range runs {
		if outsize(r) {
			held = append(held, r)
			continue
		}
		if cur != nil && joins(*cur, r) {
			cur.Runs = append(cur.Runs, r)
			widen(cur, r)
			continue
		}
		if cur != nil {
			lines = append(lines, *cur)
		}
		cur = &Line{Runs: []Run{r}, Top: r.Top, Bottom: r.Bottom(),
			Left: r.Left, Right: r.Right(), band: bandSize(r)}
	}
	if cur != nil {
		lines = append(lines, *cur)
	}
	lines = stand(lines, held)
	lines = gather(lines)
	for i := range lines {
		finish(&lines[i])
	}
	return lines
}

// outsize reports whether a run is held out of the scan and placed once the
// lines are built. A large operator and a piece of a delimiter are drawn to a
// size the line they stand on does not account for, and they are reported at a
// box that is not the box they are drawn in, so they cannot be scanned into the
// lines with the type they are set among.
//
// An accent is drawn out of the same font and is none of this. It is drawn over
// a letter, it is reported inside the band of the line that letter is on, and
// what it belongs to is decided from where it sits rather than from what it is
// set against.
func outsize(r Run) bool {
	if _, ok := Accent(r.Spec, r.Text); ok {
		return false
	}
	return tall(r)
}

// stand puts the large operators and the delimiters back on the lines they
// belong to, once the lines have been gathered from the type around them.
//
// The height these are reported at is not their height. pdftohtml gives 7 for
// every one of them in the Latin Modern printings whatever size they are set
// at, and the Computer Modern printing of Lie 7 to 9 gives 19 for a summation
// sign drawn half as tall again as that. What is reported truly is where the
// glyph starts, and a large operator starts well above the line it is set on,
// so its box lies over the line above as readily as over its own and its middle
// often lies there too. The sum of "x ∈ ∑ 𝔤^α" on page 179 of Lie 7 to 9, on a
// line whose body sits at 300, is reported at 282 against a line above that
// ends at 293, and the sign was set in the middle of an English sentence three
// times on that page alone.
//
// The foot is the one edge of the box that lands where the line is, since the
// height that is too small for the glyph is measured down from a top that is
// right. It lands 1 to 5 units inside the band in both printings, and it is
// what the line is chosen by. Nothing is asked of the run in the scan, and
// nothing about where the line sits is taken from it afterwards: a sign is not
// body type and has nothing to say about the band.
//
// This is what keeps a large delimiter off the line above as well. It is drawn
// to span what it encloses, so its box laps over that line as readily as over
// the formula itself. The characteristic polynomial on French page 356 is
// printed (X² − T_F(q)X + N_F(q))², and its parentheses, drawn 15 units tall
// against a body of 12, clipped the band of the head above them by exactly half
// their neighbour's height. Both went to the head, which then read "Proposition
// $($ 1. — ... élément$)q$ de F", and the formula below lost the brackets that
// say what is squared.
func stand(lines []Line, held []Run) []Line {
	if len(held) == 0 {
		return lines
	}
	if len(lines) == 0 {
		// A column of nothing but signs is not a page, but nothing is thrown
		// away on the strength of that.
		l := Line{Runs: held, Top: held[0].Top, Bottom: held[0].Bottom(),
			Left: held[0].Left, Right: held[0].Right()}
		for _, r := range held {
			l.Left, l.Right = min(l.Left, r.Left), max(l.Right, r.Right())
		}
		return append(lines, l)
	}
	for _, r := range held {
		at, best := 0, 0
		for i, l := range lines {
			d := 0
			switch foot := r.Bottom(); {
			case foot < l.Top:
				d = l.Top - foot
			case foot > l.Bottom:
				d = foot - l.Bottom
			default:
				// Inside the band, and the further inside the
				// surer. Two lines overlap only where the lower
				// is the row of scripts that hangs under the
				// upper and gather has not put the two together
				// yet, so a foot can land in both, and the one
				// it is a hair inside is the one it is a hair
				// outside of. The subscripts of
				// "\sum_{i\in I}Y_ie_i of B_{(K(\mathbf{Y}))}"
				// on page 368 make a band that opens at 440
				// where the line itself runs from 429 to 447,
				// and the foot of the sign is at 440.
				d = -min(foot-l.Top, l.Bottom-foot)
			}
			if i == 0 || d < best {
				at, best = i, d
			}
		}
		l := &lines[at]
		l.Runs = append(l.Runs, r)
		if sign(r) {
			// An operator is set on its line and the line reaches to
			// it. A delimiter is not: it is drawn to span the rows it
			// encloses, it belongs to none of them, and the matrix of
			// page 365 is bracketed 21 units to the left of its first
			// column. Giving that edge to one row of it puts the row
			// outside the line whose scripts it carries, and the s-1
			// under the a of the second row was left standing as a
			// line of its own.
			l.Left, l.Right = min(l.Left, r.Left), max(l.Right, r.Right())
		}
	}
	return lines
}

// gather takes the exponents that were left behind and puts them back on the
// line they belong to.
//
// A display sets its large operators and the exponents beside them so far above
// the line that their boxes miss the band of the line entirely, and they arrive
// as a line of their own holding four characters and a summation sign. An index
// hanging under a line does the same the other way: the i of M_{n_i} sits low
// enough to miss the band and comes out as a line holding one letter. Such a
// line is not a line: it is math, it is short, and it starts and ends inside a
// line it overlaps. It goes back on whichever of its neighbours that is.
//
// An accent is asked about first and asked a different question, because the
// general rule offers the line below before the line above and an accent is
// drawn above the letter it belongs to.
func gather(lines []Line) []Line {
	out := lines[:0:0]
	take := func(l *Line, from Line) {
		l.Runs = append(l.Runs, from.Runs...)
		l.Left = min(l.Left, from.Left)
		l.Right = max(l.Right, from.Right)
	}
	for i := 0; i < len(lines); i++ {
		if n := len(out); n > 0 && accented(lines[i]) && over(lines[i], out[n-1]) {
			take(&out[n-1], lines[i])
			continue
		}
		below := i+1 < len(lines) && stray(lines[i], lines[i+1])
		above := len(out) > 0 && stray(lines[i], out[len(out)-1])
		if below && above && closest(lines[i], out[len(out)-1], lines[i+1]) {
			below = false
		}
		if below {
			take(&lines[i+1], lines[i])
			continue
		}
		if above {
			take(&out[len(out)-1], lines[i])
			continue
		}
		out = append(out, lines[i])
	}
	return out
}

// accented reports whether a line is nothing but accents. CMEX draws a wide
// tilde or a wide hat as a glyph of its own rather than as part of the letter,
// and a row of them across a display often clears the band of every line and
// arrives as a line of its own.
func accented(l Line) bool {
	for _, r := range l.Runs {
		if _, ok := Accent(r.Spec, r.Text); !ok {
			return false
		}
	}
	return len(l.Runs) > 0
}

// over reports whether a line of accents was drawn over the line above it.
//
// The nearness test that finds the limit of a sum is no use here. A row of
// tildes over a display sits within a few units of the prose on the line below
// as well as of the letters it belongs to, so both neighbours answer to it and
// the general rule takes the one it is offered first, which is the wrong one.
//
// Where poppler puts the accent settles it. An accent is reported inside the
// band of the line it decorates, about ten units down from the top of it, and
// well clear of the band of the next line: page 114 sets eight tildes at 387
// over the display at 376, and the prose they were read as belonging to starts
// at 406. That page came out with a tilde over eight letters of an English
// sentence and nothing to show for it, since a page whose accents have wandered
// still balances its dollars and reads as prose.
func over(l, other Line) bool {
	for _, r := range l.Runs {
		if r.Top < other.Top || r.Top >= other.Bottom {
			return false
		}
	}
	return true
}

// stray reports whether a line is a piece of the line beside it. It is either a
// script that missed the band, which overlaps the line it belongs to and starts
// and ends inside it, or the limit of a large operator, which misses the band by
// so much that it does not overlap at all.
func stray(l, other Line) bool {
	if !piece(l, other) {
		return false
	}
	// Inside the line it came off, give or take the size of what is on it. The
	// last index of a formula ends a little past the last letter of the line,
	// because the line ends with the letter and the index hangs off it.
	em := 0
	for _, r := range l.Runs {
		em = max(em, r.Spec.Size)
	}
	if l.Bottom > other.Top && l.Top < other.Bottom && l.Left >= other.Left-em && l.Right <= other.Right+em {
		return true
	}
	return limit(l, other)
}

// wordRE matches type that reads as a word rather than as a name.
var wordRE = regexp.MustCompile(`[A-Za-z]{3,}`)

// piece reports whether a line could have come off the line beside it. Whatever
// is on it is set smaller than the body of that line, since TeX sets a script
// and the limit of an operator smaller than the term they hang off, and there is
// too little of it to be a line. The lower limit of a sum is "i=1", of which the
// "=1" is set in roman and is not math by the classifier, so the test cannot ask
// for math and asks instead that nothing on the line reads as a word.
func piece(l, other Line) bool {
	body := 0
	for _, r := range other.Runs {
		if !offband(r) {
			body = max(body, r.Spec.Size)
		}
	}
	if body == 0 {
		// Nothing but large operators on it, which happens when the signs of a
		// display come away on a line of their own. There is no body to be
		// smaller than, so the size says nothing.
		body = math.MaxInt
	}
	n := 0
	for _, r := range l.Runs {
		// A large operator carries the size of the line it belongs to whatever
		// height it is drawn at, so it is not evidence either way.
		if !tall(r) && r.Spec.Size >= body {
			return false
		}
		t := strings.TrimSpace(r.Text)
		if !r.Class.Math() && wordRE.MatchString(t) {
			return false
		}
		n += len([]rune(t))
	}
	return n > 0 && n <= 20
}

// limit reports whether a short line is the upper or lower limit of a large
// operator on the line beside it.
//
// A display sets the limits of a sum above and below the sign, far enough out
// that their boxes clear the band of the line altogether, so the overlap test
// says nothing about them and they arrive as lines of their own holding "n" and
// "i=1". The sign is the evidence. A piece of math centred over or under a large
// operator, with no line of its own height between the two, is its limit and
// nothing else: ordinary type is never set that close above a display.
//
// Every run has to answer to a sign, not just the line as a whole. A display
// with three sums in it puts the three lower limits on one line between them,
// and that line is centred on nothing.
func limit(l, other Line) bool {
	// The two have to be within a line of each other. They may also overlap, and
	// a display of several sums often puts the signs on a line of their own that
	// laps over the body of the formula, so a negative gap is no objection.
	if max(other.Top-l.Bottom, l.Top-other.Bottom) > other.Height() {
		return false
	}
	// The sign can be on either line. An integral in a display is drawn tall
	// enough that its box reaches the upper limit, and it comes away with the
	// limit rather than with the body of the formula.
	signs := append(append([]Run{}, other.Runs...), l.Runs...)
	n := 0
	for _, r := range signs {
		if tall(r) {
			n++
		}
	}
	if n == 0 {
		return false
	}
	for _, r := range l.Runs {
		if !tall(r) && !under(r, signs) {
			return false
		}
	}
	return true
}

// closest reports whether a stray line that both of its neighbours answered for
// belongs to the one above it.
//
// Both answering is what two display lines do when each sets a large operator
// with a limit written under it. The limit between them is under the sign of the
// line above and over the sign of the line below, a line's width from either, so
// nothing about where it sits refuses either line, and gather offers the line
// below first. Pages 318 to 321 of the English printing and 299 to 302 of the
// French set the same product on line after line, and every one of them was
// handed the limit of the product above it and lost its own.
//
// A limit is centred on its sign, so the sign it is centred on is the one it
// belongs to. The measure is taken run by run and the worst run counts, since a
// display of two sums puts both lower limits on one line and the middle of that
// line is centred on nothing.
//
// Where the two signs stand at the same place across the page, which is what a
// display that repeats one product line after line does, the measure says
// nothing and the nearer band takes it. The nearer band takes it too where
// neither line sets a sign, since then the stray line is not the limit of
// anything but an index hanging off the end of a formula, and French page 414
// hangs one off each of two lines that overlap it.
//
// A stray line can carry the sign itself, and then the sign is evidence for
// both neighbours equally and for neither.
func closest(l, above, below Line) bool {
	if _, ok := reach(l, l); ok {
		// The stray line carries the operator itself, which happens when the
		// signs of a display come away on a line of their own. The sign then
		// answers for both neighbours equally and says nothing about either.
		return false
	}
	da, oka := reach(l, above)
	db, okb := reach(l, below)
	switch {
	case oka && !okb:
		// A limit belongs to an operator, so where only one of the two sets one
		// there is nothing to weigh.
		return true
	case okb && !oka:
		return false
	case oka && okb && da != db:
		return da < db
	}
	return l.Top-above.Bottom <= below.Top-l.Bottom
}

// reach is how far the runs of a stray line stand from the large operators of
// the line beside it, worst run first, and whether that line sets one at all.
// Distances are doubled, being between midpoints, which are halves.
func reach(l, other Line) (int, bool) {
	var signs []Run
	for _, r := range other.Runs {
		// A wide tilde is drawn in the same font as a summation sign and is not
		// an operator and has no limits. French page 333 sets det(u~) on the
		// line above the sum of item (6), and the tilde stood nearer the bound
		// of that sum than anything on the sum's own line did, so the bound went
		// up into the determinant and item (6) lost it.
		if _, ok := Accent(r.Spec, r.Text); ok {
			continue
		}
		if tall(r) {
			signs = append(signs, r)
		}
	}
	if len(signs) == 0 {
		return 0, false
	}
	worst := 0
	for _, r := range l.Runs {
		if tall(r) {
			continue
		}
		best := -1
		for _, s := range signs {
			d := r.Left + r.Right() - s.Left - s.Right()
			if d < 0 {
				d = -d
			}
			if best < 0 || d < best {
				best = d
			}
		}
		worst = max(worst, best)
	}
	return worst, true
}

// under reports whether a run stands against one of the large operators among
// the runs given. A sum is set with its limits centred over and under the sign
// and an integral with them up and down to the right of it, so the test is
// nearness rather than position: within a couple of ems of the sign, which is
// where a limit is and where the body of the formula is not.
func under(r Run, runs []Run) bool {
	slack := 2 * r.Spec.Size
	for _, s := range runs {
		if tall(s) && r.Left <= s.Right()+slack && r.Right() >= s.Left-slack {
			return true
		}
	}
	return false
}

// joins reports whether a run belongs to the line being gathered.
//
// The test is vertical overlap rather than a shared top, because a subscript
// and the letter it hangs off do not share a top and a superscript sits above
// both. Half of the shorter run has to be inside the taller one's band, which
// separates an index from the line below it without separating it from its own.
// A wide accent is asked for its middle instead. It is drawn out of the font
// the large operators come from and is reported as tall as one, so half of it
// inside a band is easily met by a band it has no business in, and a row of
// accents over a display sits within a few units of the prose below it. The
// operators themselves never come here at all: they are held out of the scan
// and placed by stand once the lines are built.
func joins(l Line, r Run) bool {
	lo, hi := max(l.Top, r.Top), min(l.Bottom, r.Bottom())
	overlap := hi - lo
	if overlap <= 0 {
		return false
	}
	if tall(r) {
		mid := r.Top + r.Height/2
		return l.Top <= mid && mid <= l.Bottom
	}
	shorter := min(l.Height(), r.Height)
	return overlap*2 >= shorter
}

// widen grows a line to hold a run. The band follows the body type of the line
// rather than the union of everything on it, so that a large operator, which is
// drawn far above and below the baseline, does not swallow the line above.
//
// Which run is the body is read off the font size and not off the box, because
// the boxes of the mathematics fonts are not to scale: the prime of the eight
// point symbol font is reported 12 units high, the same as the twelve point
// roman it hangs off, so a line that happens to begin with a prime kept the
// prime's band and everything else on the line then sat below it. That is how
// "N′ et donc isomorphes à M/N′" came out of the French chapter as "$N_'$ et
// donc isomorphes à $M/N_'$", with both primes read as indices.
func widen(l *Line, r Run) {
	l.Left = min(l.Left, r.Left)
	l.Right = max(l.Right, r.Right())
	if offband(r) || r.Spec.Size < l.band {
		return
	}
	if r.Spec.Size > l.band || r.Height > l.Height() {
		l.Top, l.Bottom, l.band = r.Top, r.Bottom(), r.Spec.Size
	}
}

// bandSize is the size a run offers a line as its body. A sign drawn out of
// proportion to the type beside it offers nothing.
func bandSize(r Run) int {
	if offband(r) {
		return 0
	}
	return r.Spec.Size
}

// tall reports whether a run is one of the glyphs that is drawn out of
// proportion to the line it is on: the large operators and the pieces of a
// delimiter spanning several lines. pdftohtml reports a height of 7 for all of
// them whatever their real size, so they are recognised by their font.
func tall(r Run) bool { return Extension(r.Spec) }

// sign reports whether a run is a large operator, which is the one kind of
// glyph a line can write a limit across rather than beside. A wide tilde and a
// wide hat are drawn out of the same font and are neither.
func sign(r Run) bool {
	if !tall(r) {
		return false
	}
	for _, c := range strings.TrimSpace(r.Text) {
		s, accent, ok := CMEX(c)
		return ok && !accent && cmexOps[s]
	}
	return false
}

// across reports whether a run stands over or under a large operator on this
// line, which is what a limit does and nothing else does.
func across(r Run, runs []Run) bool {
	for _, o := range runs {
		if sign(o) && r.Left < o.Right() && o.Left < r.Right() {
			return true
		}
	}
	return false
}

// bend reports whether a run is the dangerous bend, the sign Bourbaki sets in
// the margin against a passage the reader is to take slowly.
//
// The French printing draws it from its own font at twice the size of the body,
// and the only other thing that font sets is the brackets of a citation, which
// are drawn at the size of the type around them. The 2023 English printing
// carries no such character at all: its text layer has the brackets and nothing
// else, so the sign is drawn there rather than set.
func bend(r Run) bool { return family(r.Spec) == "BOUR" && strings.TrimSpace(r.Text) == "Z" }

// section reports whether a run is the section sign standing as the mark of the
// book rather than a glyph that came back wrong.
//
// TeX keeps \S in the mathematics symbol font, so a volume that prints §§ draws
// them out of a family this reads as mathematics, and Lie 7 to 9 shipped 1268
// cross references inside dollar signs on the strength of it: "$§1$", "$I,§6$",
// "$(§2$", and every § heading of the volume written "$§$" and so set at the
// wrong level. The sign is a mark of the book and never an operator.
//
// A § carrying an index is the exception, and there is one in the six volumes.
// Page 363 of Topologie 1 to 4 writes the fundamental group of the circle,
// \pi_1(S_1,1), and the S comes back from that page as a §. That one is inside
// a formula and taking it out of the dollars would cut the formula in two,
// where leaving it is a wrong letter in a formula that still parses. A citation
// never subscripts its §, so the index is what tells them apart.
func section(runs []Run, i int) bool {
	if strings.TrimSpace(runs[i].Text) != "§" {
		return false
	}
	return i+1 >= len(runs) || runs[i+1].Level == Base
}

// offband reports whether a run is drawn out of proportion to the line it
// stands on and so can say nothing about where that line sits or how deep
// anything beside it is nested.
//
// The dangerous bend of French page 19 is set at 29 against a body of 14 and
// stands in the margin beside a remark in small type. It came first on the
// line, so the two lines of the remark were gathered into its band and read as
// its indices, and the page shipped the remark as "$Z_{artinien) (VIII,
// p.}^{L\u2019anneau de polyn\u00f4mes}...$".
func offband(r Run) bool { return tall(r) || bend(r) }

// finish orders a line left to right and works out how deep each run is nested
// and which side of its parent it sits on.
//
// Depth is read off the font size rather than off the position. TeX sets an
// index in a smaller font than the term it hangs off and the index of an index
// smaller still, so the sizes on a line are its levels, and no measurement is
// needed to tell one from another. Position is then only asked the one question
// it answers reliably: above or below. That matters, because the boxes
// pdftohtml reports are coarse enough that a subscript and a letter on the
// baseline can come out at the same height, and a rule that reads depth off the
// position reads M_{i+1} as Mi+1 whenever they do.
func finish(l *Line) {
	sort.SliceStable(l.Runs, func(i, j int) bool { return l.Runs[i].Left < l.Runs[j].Left })
	margin(l)
	depths := depths(l.Runs)
	base := Run{Span: pdfsrc.Span{Top: l.Top, Height: l.Height()}}
	parent := []Run{base}
	for i := range l.Runs {
		r := &l.Runs[i]
		if offband(*r) {
			continue
		}
		r.Depth = depths[r.Spec.Size]
		if r.Depth == 0 {
			parent = append(parent[:1], *r)
			continue
		}
		for len(parent) <= r.Depth {
			parent = append(parent, parent[len(parent)-1])
		}
		p := parent[r.Depth-1]
		// An index is set against the thing it indexes, with no space between
		// them. A smaller run standing well clear of anything is not an index
		// at all, it is a change of type: the index at the back of the volume
		// sets its headings in large bold capitals and its entries in small
		// type beside them in the other column, and reading those entries as
		// exponents of the headings puts half the index inside dollar signs.
		// The run a limit stands clear of is the one before the display, so
		// the gap is not asked of it. Equation (18) of § 21 opens on the bound
		// of a sum, a hundred units past the equation number and directly under
		// the sign, and reading the gap there made the bound a change of type
		// and left the display as "\lambda \sum_{\in\widehat{G}}e_{\lambda}".
		// A limit is centred on its sign, so what says a run is one is that it
		// stands across a large operator, which is the one thing on a line that
		// can have something written over and under it.
		if i > 0 && !offband(l.Runs[i-1]) && r.Left > l.Runs[i-1].Right()+4*r.Spec.Size &&
			!across(*r, l.Runs) {
			r.Depth = 0
			parent = append(parent[:1], *r)
			continue
		}
		// A run set below its parent is an index and one set above is an
		// exponent. The two sit either side of the line the parent stands on,
		// which is where its box would end if the run were on the line with it.
		if r.Top < p.Top+(p.Height-r.Height)/2 {
			r.Level = Sup
		} else {
			r.Level = Sub
		}
		parent = append(parent[:r.Depth], *r)
	}
}

// margin takes the dangerous bend out of the extent of a line.
//
// The sign is set in the margin, which is where the reader is meant to catch
// sight of it and is a good half inch left of the type it marks. Counting it
// puts the left edge of the line out there with it, and the rest of the remark
// then reads as indented past a margin nothing else on the page is set to: the
// remark of French page 19 came out as three paragraphs of one line each.
func margin(l *Line) {
	left, right, n := 0, 0, 0
	for _, r := range l.Runs {
		if bend(r) {
			continue
		}
		if n == 0 {
			left, right = r.Left, r.Right()
		}
		left, right, n = min(left, r.Left), max(right, r.Right()), n+1
	}
	if n > 0 {
		l.Left, l.Right = left, right
	}
}

// depths reads the sizes on a line as levels. The size the line is set in is the
// line itself. A size no more than four fifths of the one above it is a level
// below it, and a size close to it is the same level: the volume sets a line of
// small type at 13 where the text around it is 15, and that is a change of type
// and not an index.
//
// Nothing above the size of the line is nested at all. A sign is often drawn
// larger than the type it is set among, and reading the type as hanging off the
// sign turns a sentence into an index: the French exercises set the exterior
// power of page 333 at 20 in a body of 14, and the lemma beside it shipped as
// "$_{Lemme1. \u2014Soitpun entier tel que0\leqslant p\leqslant m ...}$".
func depths(runs []Run) map[int]int {
	sizes := make([]int, 0, 4)
	for _, r := range runs {
		if !offband(r) && !slices.Contains(sizes, r.Spec.Size) {
			sizes = append(sizes, r.Spec.Size)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sizes)))
	body := bodySize(runs)
	out := map[int]int{}
	depth, step := 0, body
	for _, s := range sizes {
		if s >= body {
			out[s] = 0
			continue
		}
		if s*5 <= step*4 {
			depth++
			step = s
		}
		out[s] = depth
	}
	return out
}

// bodySize is the size a line is set in.
//
// It is the largest size carrying a fair share of the characters on the line,
// counted by character and not by run, because a display breaks into a run per
// symbol where a sentence arrives in two or three pieces. A size a single
// character wide is a sign drawn out of proportion rather than the type the
// line is set in, and the commonest size alone is not the answer either: a line
// of d^i_{n-1} \circ d^j_n carries more characters in its indices than on its
// baseline, and reading it at the size of its indices flattens them.
func bodySize(runs []Run) int {
	count := map[int]int{}
	for _, r := range runs {
		if offband(r) {
			continue
		}
		count[r.Spec.Size] += len([]rune(strings.TrimSpace(r.Text)))
	}
	most := 0
	for _, chars := range count {
		most = max(most, chars)
	}
	best := 0
	for size, chars := range count {
		// share is how much of the commonest size a size has to carry to
		// pass for type. A quarter keeps the baseline of a line of indices
		// and drops a sign standing alone beside a sentence.
		const share = 4
		if chars*share >= most && size > best {
			best = size
		}
	}
	return best
}
