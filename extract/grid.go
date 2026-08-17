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
// The grid is told from the scripts of a base by its shape. A base carries one
// superscript and one subscript and they are set at the same place, one over
// the other; a matrix carries as many of each as it has columns, and the two of
// a column are set one over the other while the columns stand apart by the
// space the page sets between them. So a cluster is a grid when it holds as
// many superscripts as subscripts, two of each or more, and they pair off into
// columns that do not overlap and stand in the same order at both levels. Two
// is what makes it a grid: one of each is a base with an exponent and an index,
// which is most of the mathematics in the book.

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
		m, ok := grid(rest[:j])
		if !ok {
			out = append(out, rest[:j]...)
			rest = rest[j:]
			continue
		}
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

// grid gathers a cluster into the matrix it is, and says so when it is not one.
func grid(c []token) (token, bool) {
	var sup, sub []token
	for _, t := range c {
		switch {
		case t.depth != 1:
			return token{}, false
		case t.level == Sup:
			sup = append(sup, t)
		case t.level == Sub:
			sub = append(sub, t)
		default:
			return token{}, false
		}
	}
	if len(sup) < 2 || len(sup) != len(sub) {
		return token{}, false
	}
	// The top row is drawn above the bottom row and clear of it, which is what
	// makes them two rows rather than one. Clear is measured with the same
	// overhang a stack is, since a row is set out of glyphs of different
	// depths and the box a glyph is reported in is not the ink inside it: the b
	// of the diagonal matrix on page 379 is drawn a unit higher than the 0
	// beside it, so the two rows touch.
	if highestTop(sup) >= highestTop(sub) || lowest(sup) > highest(sub)+overhang {
		return token{}, false
	}
	for i := range sup {
		// The two entries of a column are set one over the other.
		if sup[i].left > sub[i].right || sub[i].left > sup[i].right {
			return token{}, false
		}
		// The columns stand apart, in the same order at both levels.
		if i > 0 && (sup[i-1].right >= sup[i].left || sub[i-1].right >= sub[i].left) {
			return token{}, false
		}
	}
	return matrix(c, sup, sub), true
}

// matrix writes the grid out as one token, standing where the whole cluster
// stood.
func matrix(c []token, sup, sub []token) token {
	rows := make([]string, 2)
	for i, row := range [][]token{sup, sub} {
		cells := make([]string, 0, len(row))
		for _, x := range row {
			cells = append(cells, cell(x))
		}
		rows[i] = strings.Join(cells, " & ")
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

// cell is what one entry of the matrix says.
func cell(t token) string { return strings.TrimSpace(t.text) }

// lowest, highest and highestTop are the edges a pair of rows is told apart by.
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
