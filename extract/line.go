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

	// Top and Bottom are the band of the tallest run, which is the body of
	// the line. Left and Right are the extent of everything in it.
	Top, Bottom, Left, Right int
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
	for _, r := range runs {
		if cur != nil && joins(*cur, r) {
			cur.Runs = append(cur.Runs, r)
			widen(cur, r)
			continue
		}
		if cur != nil {
			lines = append(lines, *cur)
		}
		cur = &Line{Runs: []Run{r}, Top: r.Top, Bottom: r.Bottom(), Left: r.Left, Right: r.Right()}
	}
	if cur != nil {
		lines = append(lines, *cur)
	}
	lines = gather(lines)
	for i := range lines {
		finish(&lines[i])
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
func gather(lines []Line) []Line {
	out := lines[:0:0]
	take := func(l *Line, from Line) {
		l.Runs = append(l.Runs, from.Runs...)
		l.Left = min(l.Left, from.Left)
		l.Right = max(l.Right, from.Right)
	}
	for i := 0; i < len(lines); i++ {
		if i+1 < len(lines) && stray(lines[i], lines[i+1]) {
			take(&lines[i+1], lines[i])
			continue
		}
		if n := len(out); n > 0 && stray(lines[i], out[n-1]) {
			take(&out[n-1], lines[i])
			continue
		}
		out = append(out, lines[i])
	}
	return out
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
		if !tall(r) {
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
func joins(l Line, r Run) bool {
	lo, hi := max(l.Top, r.Top), min(l.Bottom, r.Bottom())
	overlap := hi - lo
	if overlap <= 0 {
		return false
	}
	shorter := min(l.Height(), r.Height)
	return overlap*2 >= shorter
}

// widen grows a line to hold a run. The band follows the tallest run rather
// than the union of them all, so that a large operator, which is drawn far
// above and below the baseline, does not swallow the line above.
func widen(l *Line, r Run) {
	l.Left = min(l.Left, r.Left)
	l.Right = max(l.Right, r.Right())
	if !tall(r) && r.Height > l.Height() {
		l.Top, l.Bottom = r.Top, r.Bottom()
	}
}

// tall reports whether a run is one of the glyphs that is drawn out of
// proportion to the line it is on: the large operators and the pieces of a
// delimiter spanning several lines. pdftohtml reports a height of 7 for all of
// them whatever their real size, so they are recognised by their font.
func tall(r Run) bool { return family(r.Spec) == "CMEX" }

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
	depths := depths(l.Runs)
	base := Run{Span: pdfsrc.Span{Top: l.Top, Height: l.Height()}}
	parent := []Run{base}
	for i := range l.Runs {
		r := &l.Runs[i]
		if tall(*r) {
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
		if i > 0 && !tall(l.Runs[i-1]) && r.Left > l.Runs[i-1].Right()+4*r.Spec.Size {
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

// depths reads the sizes on a line as levels. The largest is the line itself.
// A size no more than four fifths of the one above it is a level below it, and
// a size close to it is the same level: the volume sets a line of small type at
// 13 where the text around it is 15, and that is a change of type and not an
// index.
func depths(runs []Run) map[int]int {
	sizes := make([]int, 0, 4)
	for _, r := range runs {
		if !tall(r) && !slices.Contains(sizes, r.Spec.Size) {
			sizes = append(sizes, r.Spec.Size)
		}
	}
	sort.Sort(sort.Reverse(sort.IntSlice(sizes)))
	out := map[int]int{}
	depth, step := 0, 0
	for i, s := range sizes {
		if i > 0 && s*5 <= step*4 {
			depth++
			step = s
		}
		if i == 0 {
			step = s
		}
		out[s] = depth
	}
	return out
}
