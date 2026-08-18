package extract

import "strings"

// A matrix set inline has no rows in a text layer either. TeX sets a small
// matrix in a running line at the size a script is set at, its top row where an
// exponent goes and its bottom row where an index goes, so the four entries of
// a two by two matrix come back as a superscript, a subscript, a superscript
// and a subscript with no base between them. Written in that order the diagonal
// matrix of a and b is ^a_0^0_b, which is two superscripts against one base and
// TeX refuses it by name, the same Double superscript a flattened stack gets
// refused for.
//
// stack.go is careful to leave this alone, since a matrix is not a script that
// came apart and merging its rows would say something the page does not. What
// it leaves is a grid, and a grid is what this reads.
//
// A grid comes back in one of two shapes, and which one it is depends on
// nothing more than where the page happened to break its runs.
//
// The first is boxed by column. Every entry arrives in a run of its own, and
// the grid is told from the scripts of a base by its shape: a base carries one
// superscript and one subscript and they are set at the same place, one over
// the other, where a matrix carries as many of each as it has columns and the
// two of a column are set one over the other while the columns stand apart by
// the space the page sets between them. So a cluster is a grid when it holds as
// many superscripts as subscripts, two of each or more, and they pair off into
// columns that do not overlap and stand in the same order at both levels. Two
// is what makes it a grid: one of each is a base with an exponent and an index,
// which is most of the mathematics in the book.
//
// The second is boxed by row, and it is the commoner of the two. Page 166 of
// Topologie algébrique prints the matrix of 0, 1 over -1, 0 and nothing there
// is boxed by column at all: the whole top row arrives as the single run "0 1"
// and the bottom row as two, the minus out of the symbol font and then "1 0".
// There is no pairing to measure, so the rows are put back together out of the
// pieces they were drawn in and cut into cells at the spaces the page set, and
// what says it is a matrix is that both rows cut into the same number of cells
// and that there are two or more of them.
//
// That is weaker evidence than the columns give, and on its own it is not
// enough: x^{a b}_{c d} is the same two rows of two cells and is not a matrix.
// What tells them apart is the base. A script hangs off the term before it and
// is set against it, and a matrix hangs off nothing and stands clear, so the
// row shape is only read where the cluster stands clear of whatever is in front
// of it. A limit is no trouble either, since hoist has already moved the sign
// in front of the limit it belongs to and the cluster is then set against it.
//
// The third is the same matrix with scripts on its entries, which neither of
// the other two can read: they are written over the runs of one level and an
// entry that carries a script is written over two. Page 73 of Théories
// spectrales prints the matrix of alpha inverse less alpha nought inverse, 0
// over 0, 0, and the -1 of it arrives a level further in than the entries do.
// The entries are the runs at the level the matrix is set at and a run deeper
// than that is written on the entry in front of it, which is the same rule the
// rest of a line is read by. What is left is two rows of entries, and they are
// cut into columns at the white that runs down the whole cluster rather than at
// the white in either row alone, since a wide entry over a narrow one leaves no
// gap where the row above it has one: the top row here is alpha inverse less
// alpha nought inverse and the bottom row under it is a single 0.
//
// This is read only where an entry does carry a script, so that a cluster
// either of the other two reads comes out of them and not out of this, and only
// where the cluster hangs off nothing, which is the same guard the row shape is
// read under and for the same reason.
//
// Running before overset is what makes it readable. overset takes that -1 for a
// script drawn over the 0 beside it and writes the cluster back one level
// shallower, and read after that the page comes out as the matrix of
// \overset{-1}{0}, which is a plausible matrix of the wrong thing.

// gridded reads a matrix set inline as a matrix.
func gridded(toks []token) []token {
	out := make([]token, 0, len(toks))
	rest := toks
	for len(rest) > 0 {
		if rest[0].depth == 0 {
			out = append(out, rest[0])
			rest = rest[1:]
			continue
		}
		j := 0
		for j < len(rest) && rest[j].depth > 0 {
			j++
		}
		c := rest[:j]
		cells, ok := byColumn(c)
		if !ok && loose(out, c) {
			if cells, ok = byRow(c); !ok {
				cells, ok = byInk(c)
			}
		}
		if !ok {
			out = append(out, c...)
			rest = rest[j:]
			continue
		}
		m := matrix(c, cells)
		// A matrix the page prints between parentheses gets them back as the
		// delimiters of the matrix, and loses them where it stood, since
		// \begin{pmatrix} draws its own. Both have to be there and both have to
		// be against the grid: exercise 5 of § 11 prints the pair (m+n, M) with
		// the tall opening parenthesis of M missing from the text layer
		// altogether, and taking the closing one for the matrix would take the
		// parenthesis of the pair.
		if open, ok := gridOpen(out, m); ok {
			if cut, ok := gridClose(rest[j:], m); ok {
				out = append(open, m)
				rest = cut
				continue
			}
		}
		out = append(out, m)
		rest = rest[j:]
	}
	return out
}

// gridOpen takes the parenthesis a matrix was printed after off the end of what
// stands before it, if there is one standing against it. It is written at the
// end of a run rather than as a run of its own: page 226 hands back the whole of
// "(A) to the class of the matrix (" as one piece of body type.
func gridOpen(out []token, m token) ([]token, bool) {
	if len(out) == 0 {
		return nil, false
	}
	last := out[len(out)-1]
	if !strings.HasSuffix(last.text, "(") || m.left-last.right > gridReach {
		return nil, false
	}
	cut := append([]token(nil), out...)
	last.text = last.text[:len(last.text)-1]
	if last.text == "" {
		return cut[:len(cut)-1], true
	}
	cut[len(cut)-1] = last
	return cut, true
}

// gridClose takes the parenthesis a matrix was printed before off the front of
// what follows it, if there is one standing against it.
func gridClose(rest []token, m token) ([]token, bool) {
	if len(rest) == 0 || !strings.HasPrefix(rest[0].text, ")") || rest[0].left-m.right > gridReach {
		return nil, false
	}
	cut := append([]token(nil), rest...)
	cut[0].text = cut[0].text[1:]
	if cut[0].text == "" {
		cut = cut[1:]
	}
	return cut, true
}

// gridReach is how far a parenthesis may stand from the matrix it delimits, in
// the pixel units of the page. Three is what the six volumes measure, on either
// side and in both printings, and it is the same reach two pieces of one word
// are read at.
const gridReach = hoistGap

// twoRows cuts a cluster into the row drawn above and the row drawn below, and
// says so when it is not two rows at all.
func twoRows(c []token) (sup, sub []token, ok bool) {
	for _, t := range c {
		switch {
		case t.depth != 1:
			return nil, nil, false
		case t.level == Sup:
			sup = append(sup, t)
		case t.level == Sub:
			sub = append(sub, t)
		default:
			return nil, nil, false
		}
	}
	if len(sup) == 0 || len(sub) == 0 || !apart(sup, sub, c) {
		return nil, nil, false
	}
	return sup, sub, true
}

// apart is the measurement that says two rows are the two rows of one matrix.
func apart(sup, sub, c []token) bool {
	// The top row is drawn above the bottom row and clear of it, which is what
	// makes them two rows rather than one.
	//
	// Clear is measured against the depth of the type, not against a fixed
	// number of units, because the box a glyph is reported in is not the ink
	// inside it and the slack is the descent of the glyph. It is the same
	// matrix printed twice that says so. Page 225 of chapter VIII sets X, 0
	// over 0, Y out of LMRoman7 and LMRoman6 and reports every box 8 units
	// tall, and the two rows abut exactly. Page 208 of the French printing
	// sets the same four entries out of LMRoman8 and reports the letters 12
	// units tall and the digits 8, so the top row hangs 3 units into the top
	// of the bottom row while the printed page is the same page. A third of
	// the depth is the descent a glyph reserves under its baseline, and it
	// takes both: 0 of 8 on the English page and 3 of 12 on the French.
	if highestTop(sup) >= highestTop(sub) || lowest(sup)-highest(sub) > deep(c)/descent {
		return false
	}
	// The rows stand a script apart and not a line apart, which is what says
	// they are the two rows of one matrix rather than two lines of a display
	// that the line gathering ran together. It is the measurement stack.go
	// makes for the same reason and refuses a stack on.
	return highest(sub)-lowest(sup) <= deep(c)
}

// byColumn gathers a cluster boxed by column into the cells of the matrix it is,
// and says so when it is not one.
func byColumn(c []token) ([][]string, bool) {
	sup, sub, ok := twoRows(c)
	if !ok || len(sup) < 2 || len(sup) != len(sub) {
		return nil, false
	}
	for i := range sup {
		// The two entries of a column are set one over the other.
		if sup[i].left > sub[i].right || sub[i].left > sup[i].right {
			return nil, false
		}
		// The columns stand apart, in the same order at both levels, and the
		// gap is what separates a matrix from a script written in more than
		// one piece. \sum^{n+1}_{i=0} arrives as the two superscripts n and +1
		// over the two subscripts i and =0, which pair off into columns as
		// neatly as any matrix does; what says it is not one is that the pieces
		// of a script are set touching and the columns of a matrix are not.
		//
		// Touching is measured on the ink. The first column of the matrix on
		// page 208 of the French printing is the run "X ", and the space is
		// inside the box the run is reported in, so its right edge falls
		// exactly on the left edge of the 0 beside it and the matrix is refused
		// for a gap it has.
		if i > 0 && (inkRight(sup[i-1]) >= sup[i].left || inkRight(sub[i-1]) >= sub[i].left) {
			return nil, false
		}
	}
	cells := [][]string{make([]string, 0, len(sup)), make([]string, 0, len(sub))}
	for i, row := range [][]token{sup, sub} {
		for _, x := range row {
			cells[i] = append(cells[i], strings.TrimSpace(x.text))
		}
	}
	return cells, true
}

// byRow gathers a cluster boxed by row into the cells of the matrix it is, and
// says so when it is not one.
//
// A cell is what stands between two of the spaces the page set, so a row is put
// back together first and cut afterwards. Where a cell is written with a space
// inside it this reads a column that is not there, and the two rows then cut
// into different numbers of cells and the cluster is left alone: nothing in the
// six volumes sets an inline matrix with anything wider than a signed digit or
// a single letter in it.
func byRow(c []token) ([][]string, bool) {
	sup, sub, ok := twoRows(c)
	if !ok {
		return nil, false
	}
	top, ok := cells(sup)
	if !ok {
		return nil, false
	}
	bottom, ok := cells(sub)
	if !ok || len(top) < 2 || len(top) != len(bottom) {
		return nil, false
	}
	return [][]string{top, bottom}, true
}

// byInk gathers a cluster whose entries carry scripts of their own, and says so
// when it is not one.
//
// The cluster is cut into entries first, since the runs of one entry are not the
// runs of one column: the -1 of alpha inverse is drawn after the alpha and the
// 0 of the row below is drawn between that and the next entry, so the runs of
// the top row and the runs of the bottom row are interleaved on the page. An
// entry is a run at the level the matrix is set at together with whatever was
// written deeper on it, which is the rule the rest of a line is read by, and
// what it leaves is two rows of entries that stand where their entries stand.
func byInk(c []token) ([][]string, bool) {
	sup, sub, ok := deepRows(c)
	if !ok {
		return nil, false
	}
	top, ok := entries(sup)
	if !ok {
		return nil, false
	}
	bottom, ok := entries(sub)
	if !ok {
		return nil, false
	}
	a, b := cut(top), cut(bottom)
	if len(a) < 2 || len(a) != len(b) {
		return nil, false
	}
	return [][]string{a, b}, true
}

// deepRows cuts a cluster into its entries and sorts them into the row drawn
// above and the row drawn below.
//
// Which entry a deeper run belongs to is read off the page and not off the order
// the runs arrive in, which is the one place this differs from the rest of the
// extractor. Everywhere else the run before is the term a script hangs off, and
// inside a matrix it is not: the page draws a line left to right across both
// rows at once, so the second matrix of page 73 hands back the alpha, the minus
// of its exponent, the 0 of its index, then the 0 of the row below, then the 1
// of the exponent it began three runs ago. Read in order the 1 lands on the
// entry underneath and the page says 0 to the 1.
//
// A row is a band of the page and a script is written inside the band of the row
// it belongs to, so the band is asked instead. It answers here because the two
// rows of a matrix stand clear of one another, which apart has already required,
// and a script of an entry is set within the height of its own row rather than
// out beyond it.
//
// Nothing deeper than one script is read: two levels inside a matrix is a shape
// no printing here sets, and reading it by guess would be reading it wrong. A
// cluster with no deep run at all is left to the other two readers, which are
// written over exactly that case and have the six volumes behind them.
func deepRows(c []token) (sup, sub [][]token, ok bool) {
	var carried bool
	var supAt, subAt []int
	for i, t := range c {
		switch {
		case t.depth == 1 && t.level == Sup:
			supAt = append(supAt, i)
		case t.depth == 1 && t.level == Sub:
			subAt = append(subAt, i)
		case t.depth == 2:
			carried = true
		default:
			return nil, nil, false
		}
	}
	if !carried || len(supAt) == 0 || len(subAt) == 0 {
		return nil, nil, false
	}
	// The rows are measured on the entries and not on what is written on them,
	// since a script of an entry sits above or below the row it is written in
	// and would put that row on both sides of the other.
	if !apart(at(c, supAt), at(c, subAt), c) {
		return nil, nil, false
	}
	owner := make([]int, len(c))
	for i, t := range c {
		if t.depth != 2 {
			owner[i] = i
			continue
		}
		row := supAt
		near, far := offRow(t, at(c, supAt)), offRow(t, at(c, subAt))
		switch {
		case far < near:
			row, near, far = subAt, far, near
		case far == near:
			// A script that stands as near one row as the other says nothing
			// about which it belongs to, and a matrix read on a coin is worse
			// than a formula left as it was.
			return nil, nil, false
		}
		j := -1
		for _, k := range row {
			if c[k].left <= t.left {
				j = k
			}
		}
		if j < 0 {
			return nil, nil, false
		}
		owner[i] = j
	}
	group := map[int][]token{}
	for i, t := range c {
		if i != owner[i] {
			// The level a run was given is the level it stands at against the
			// run before it, and the entry it belongs to is not always that run.
			t.level = side(c[owner[i]], t)
		}
		group[owner[i]] = append(group[owner[i]], t)
	}
	for _, i := range supAt {
		sup = append(sup, group[i])
	}
	for _, i := range subAt {
		sub = append(sub, group[i])
	}
	return sup, sub, true
}

// offRow is how far a run stands from a row of entries, and 0 where it stands
// within it.
func offRow(t token, row []token) int {
	top, bottom := highest(row), lowest(row)
	mid := (t.top + t.bottom) / 2
	switch {
	case mid < top:
		return top - mid
	case mid > bottom:
		return mid - bottom
	}
	return 0
}

// side is which side of an entry a script written on it stands, which is the
// measurement the line was read by in the first place, made again against the
// entry the page says it belongs to.
func side(p, t token) Level {
	if t.top < p.top+((p.bottom-p.top)-(t.bottom-t.top))/2 {
		return Sup
	}
	return Sub
}

// at is the runs at these places in a cluster.
func at(c []token, idx []int) []token {
	out := make([]token, 0, len(idx))
	for _, i := range idx {
		out = append(out, c[i])
	}
	return out
}

// slot is an entry of the matrix, written out, standing across the whole of
// what was drawn for it.
type slot struct {
	text        string
	left, right int
}

// entries writes each entry of a row as the mathematics it says.
//
// The scripts of an entry are gathered by level rather than in the order they
// were drawn, because the two are not the same order: alpha nought inverse
// hands back the minus of the exponent, then the 0 of the index, then the 1 of
// the exponent, since the page draws them left to right and the exponent is
// wider than the index it stands over.
func entries(rows [][]token) ([]slot, bool) {
	out := make([]slot, 0, len(rows))
	for _, g := range rows {
		var sup, sub strings.Builder
		for _, t := range g[1:] {
			switch t.level {
			case Sup:
				abut(&sup, t.text)
			case Sub:
				abut(&sub, t.text)
			default:
				return nil, false
			}
		}
		p := slot{text: strings.TrimSpace(g[0].text), left: g[0].left, right: g[0].right}
		if p.text == "" {
			return nil, false
		}
		p.text += script("_", sub.String()) + script("^", sup.String())
		for _, t := range g {
			p.left, p.right = min(p.left, t.left), max(p.right, t.right)
		}
		out = append(out, p)
	}
	return out, true
}

// script writes one script of an entry, and writes nothing where there is none.
// A script of one character needs no braces, which is how emit writes them
// everywhere else in the corpus.
func script(mark, s string) string {
	s = strings.TrimSpace(s)
	switch {
	case s == "":
		return ""
	case len([]rune(s)) == 1:
		return mark + s
	}
	return mark + "{" + s + "}"
}

// cut puts one row of entries back together and cuts it into cells at the
// spaces the page set, which is what cells does to the pieces of a row and is
// the same measurement made on entries rather than on runs.
//
// An entry may reach over the one after it, and the two are still two entries.
// cells refuses that, since two pieces of one row cannot overlap, and here it
// is ordinary: the exponent of an entry is set beyond the width of the entry
// and the next entry begins under it. Line 3 of page 73 sets alpha inverse and
// then the minus of the difference, and the second alpha begins one unit inside
// the box of that minus.
//
// A space inside an entry cuts it too, which is what reads the row a page boxed
// whole. The matrix of page 70 arrives with its top row in two runs, u and 0,
// and its bottom row as the single run "0 0", and the two rows come out two
// cells each.
//
// Not every space in a row is white the page set. A LaTeX command name ends at
// the first character that could not be part of it, so an entry written as a
// command and a letter carries a space that is part of the name rather than a
// gap between two cells. The matrix of page 362 of Algebra VIII holds -gamma y
// bar in its top right corner and the top row was cut into three cells against
// the two of the row below it, which left the matrix unread and the page saying
// ^X_-^-_y^x_X^-_-^{\gamma y}_{\overline{x}}.
func cut(row []slot) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		out = append(out, apartWords(b.String())...)
		b.Reset()
	}
	for i, p := range row {
		if i > 0 && p.left-row[i-1].right >= cellGap {
			flush()
		}
		abut(&b, p.text)
	}
	flush()
	return out
}

// apartWords cuts a cell at the white inside it, keeping the space that ends
// the name of a command.
func apartWords(s string) []string {
	var out []string
	var b strings.Builder
	end := func() {
		if t := strings.TrimSpace(b.String()); t != "" {
			out = append(out, t)
		}
		b.Reset()
	}
	for _, c := range s {
		if c != ' ' {
			b.WriteRune(c)
			continue
		}
		t := b.String()
		if i := strings.LastIndexByte(t, '\\'); i >= 0 && word(t[i+1:]) {
			b.WriteRune(c)
			continue
		}
		end()
	}
	end()
	return out
}

// cells puts one row back together out of the pieces it was drawn in and cuts
// it at the spaces the page set.
func cells(ts []token) ([]string, bool) {
	row := make([]slot, 0, len(ts))
	for i, t := range ts {
		// The pieces of a row stand in the order the row is read in and do not
		// overlap, since they are pieces of one row.
		//
		// Touching is not overlapping. The box pdftohtml reports is rounded to
		// the pixel and the rounding falls either way, so two pieces set edge
		// to edge are reported a unit apart or a unit into one another: the
		// bottom row of the matrix on page 340 of Algebre VIII sets the minus
		// of -y as the run 516 to 524 and the y as 523 to 528, and the row was
		// refused for a unit. It is the same slack a script is read within.
		if i > 0 && t.left+overhang < ts[i-1].right {
			return nil, false
		}
		row = append(row, slot{text: t.text, left: t.left, right: t.right})
	}
	return cut(row), true
}

// cellGap is how far two pieces of a row have to stand apart before the white
// between them is the space between two columns rather than the fit of the
// glyphs, in the pixel units of the page.
//
// The minus of a negative entry is drawn against the digit it signs and page
// 166 measures 0 between the two, in both matrices it prints and at both
// levels, where it measures 5 between one column and the next. Page 292 of
// chapter VIII is set smaller and measures 1 inside the entry sigma(z) and 4
// between it and the column before. Three is taken because it sits inside both
// windows and because that is the reach text.go already reads a space at.
const cellGap = 3

// loose reports whether a cluster hangs off nothing, which is what says the two
// rows in it are the rows of a matrix and not the scripts of the term before.
//
// An index is set against the thing it indexes with nothing between them, and
// the reach is the one restack reads two pieces of one script at. The matrices
// stand well clear of it: page 166 measures 17 units from the equals sign that
// introduces each of them, page 292 of chapter VIII measures 14 from the word
// before, page 72 of Théories spectrales 17. The narrowest is the 3 of page 73
// of the same volume, where what stands in front is not a term at all but the
// parenthesis TeX drew for the matrix.
// A matrix the page prints between parentheses hangs off nothing either, and
// it does not stand clear: TeX sets the grid flush against the delimiter it
// drew, so the two touch. The French printing of page 362 of Algebra VIII draws
// the parentheses of the matrix of u out of CMEX and the grid begins two units
// after the one on the left, where the English printing has no delimiters in
// the layer at all and the same grid stands eighteen units clear of the words
// before it. A delimiter is not a term a script hangs off, which is the test
// bar.go makes to tell a fraction from a bar over an index, and it is the same
// question here.
func loose(out []token, c []token) bool {
	if len(out) == 0 {
		return true
	}
	last := out[len(out)-1]
	return c[0].left-last.right > restackGap || !carries(last)
}

// matrix writes the grid out as one token, standing where the whole cluster
// stood.
func matrix(c []token, cells [][]string) token {
	rows := make([]string, 0, len(cells))
	for _, row := range cells {
		rows = append(rows, strings.Join(row, " & "))
	}
	t := token{
		text:   `\begin{pmatrix} ` + strings.Join(rows, ` \\ `) + ` \end{pmatrix}`,
		class:  c[0].class,
		level:  Base,
		math:   true,
		left:   c[0].left,
		right:  c[0].right,
		top:    c[0].top,
		bottom: c[0].bottom,
	}
	for _, x := range c {
		t.left, t.right = min(t.left, x.left), max(t.right, x.right)
		t.top, t.bottom = min(t.top, x.top), max(t.bottom, x.bottom)
	}
	return t
}

// lowest, highest, highestTop and deep are the edges a pair of rows is told
// apart by.
func lowest(ts []token) int {
	n := ts[0].bottom
	for _, t := range ts {
		n = max(n, t.bottom)
	}
	return n
}

func highest(ts []token) int {
	n := ts[0].top
	for _, t := range ts {
		n = min(n, t.top)
	}
	return n
}

// highestTop is where a row begins, which is what says which of two rows is the
// top one.
func highestTop(ts []token) int {
	n := ts[0].top
	for _, t := range ts {
		n = max(n, t.top)
	}
	return n
}

// inkRight is where a run stops printing, which is its right edge less whatever
// trailing space it carries. A box is measured over the whole of the run's text
// and a space is part of that text, so "X " is reported six units wider than the
// X in it.
//
// The width of a space is taken as the run's own width shared out over its
// characters, which is what there is to go on: the runs carry no per glyph
// metrics. It is an estimate, and it is only ever asked whether two columns
// touch, where being a unit or two out either way does not change the answer.
func inkRight(t token) int {
	trimmed := strings.TrimRight(t.text, " ")
	n := len([]rune(t.text))
	if n == 0 || trimmed == t.text {
		return t.right
	}
	return t.left + (t.right-t.left)*len([]rune(trimmed))/n
}

// descent is what fraction of its box a glyph reserves below the baseline, as a
// divisor. A third is what the printings measure and it is what type is set at.
const descent = 3

// deep is the tallest box in a cluster, which is the measure of what a line
// apart means for the material in it.
func deep(ts []token) int {
	n := 0
	for _, t := range ts {
		n = max(n, t.bottom-t.top)
	}
	return n
}
