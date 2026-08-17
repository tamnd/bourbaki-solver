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
// A matrix whose entries carry scripts of their own is not read at all, and
// running before overset is what leaves it unread. Page 73 of Théories
// spectrales prints the matrix of alpha inverse, 0 over 0, 0, and the -1 of it
// arrives a level further in than the entries, which is a depth this refuses.
// overset takes that -1 for a script drawn over the 0 beside it and writes the
// cluster back one level shallower, and read after that the page comes out as
// the matrix of \overset{-1}{0}, which is a plausible matrix of the wrong
// thing. Left where it is, it stays the double superscript it was and the audit
// goes on counting it in M09, which is what should happen to a formula nothing
// here can read.

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
			cells, ok = byRow(c)
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
	if len(sup) == 0 || len(sub) == 0 {
		return nil, nil, false
	}
	// The top row is drawn above the bottom row and clear of it, which is what
	// makes them two rows rather than one. Clear is measured with the same
	// overhang a stack is, since a row is set out of glyphs of different
	// depths and the box a glyph is reported in is not the ink inside it: the b
	// of the diagonal matrix on page 379 is drawn a unit higher than the 0
	// beside it, so the two rows touch.
	if highestTop(sup) >= highestTop(sub) || lowest(sup) > highest(sub)+overhang {
		return nil, nil, false
	}
	// The rows stand a script apart and not a line apart, which is what says
	// they are the two rows of one matrix rather than two lines of a display
	// that the line gathering ran together. It is the measurement stack.go
	// makes for the same reason and refuses a stack on.
	if highest(sub)-lowest(sup) > deep(c) {
		return nil, nil, false
	}
	return sup, sub, true
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
		// The columns stand apart, in the same order at both levels.
		if i > 0 && (sup[i-1].right >= sup[i].left || sub[i-1].right >= sub[i].left) {
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

// cells puts one row back together out of the pieces it was drawn in and cuts
// it at the spaces the page set.
func cells(ts []token) ([]string, bool) {
	var w strings.Builder
	for i, t := range ts {
		if i > 0 {
			// The pieces of a row stand in the order the row is read in and do
			// not overlap, since they are pieces of one row.
			if t.left < ts[i-1].right {
				return nil, false
			}
			if t.left-ts[i-1].right >= cellGap {
				w.WriteByte(' ')
			}
		}
		abut(&w, t.text)
	}
	return strings.Fields(w.String()), true
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
func loose(out []token, c []token) bool {
	return len(out) == 0 || c[0].left-out[len(out)-1].right > restackGap
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

// deep is the tallest box in a cluster, which is the measure of what a line
// apart means for the material in it.
func deep(ts []token) int {
	n := 0
	for _, t := range ts {
		n = max(n, t.bottom-t.top)
	}
	return n
}
