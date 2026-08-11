package extract

// A text layer has no rows in it. TeX sets a superscript and a subscript at the
// same place on the line, one over the other, and pdftohtml reports the runs of
// a page left to right, so a stack comes back interleaved. The -1 of an inverse
// is set over the E of \theta_E, its minus starting a hair to the left of the E
// and its one ending a hair to the right, and the three runs arrive as minus,
// E, one. Written in that order they are ^-_E^1, which is not a formula: TeX
// gives a base one superscript and one subscript and refuses the second by
// name, Double superscript. The audit counts them in M09 and there were 189 in
// the Markdown when this was written.
//
// The pieces can be put back, because the page says which ones belong together.
// The two halves of the -1 touch, since they were one thing before the E was
// laid across them, and the E lies inside what they span, since it is what they
// were laid across. Neither is true of a matrix flattened the same way: the X
// and the 0 of a first row stand five pixels apart, which is the gap the page
// leaves between two columns, and the rows offset each other rather than one
// lying inside the other.

// restackGap is how far two pieces of one script may stand apart and still be
// read as one thing cut in two, in the pixel units of the page.
//
// Chapter VIII measures 0 or 1 across every inverse it prints, 2 at the widest
// where a piece of the script carries a script of its own, and 4 at the
// narrowest column gap of a matrix. Three would do as well and two is taken
// because nothing on the page needs it.
const restackGap = 2

// restack puts the clusters of a line back into their levels.
func restack(toks []token) []token {
	out := make([]token, 0, len(toks))
	for i := 0; i < len(toks); {
		if toks[i].depth == 0 {
			out = append(out, toks[i])
			i++
			continue
		}
		j := i
		for j < len(toks) && toks[j].depth > 0 {
			j++
		}
		out = append(out, levels(toks[i:j])...)
		i = j
	}
	return out
}

// unit is one script of a cluster with whatever hangs off it: the i of M_{i_1}
// and the 1 under it are one unit, since they go where the i goes.
type unit struct {
	lo, hi      int
	level       Level
	left, right int
}

// levels gathers a cluster into its two scripts, where the cluster is a stack
// that came apart, and leaves it alone where it is not.
func levels(c []token) []token {
	us := units(c)
	if !stacked(c, us) {
		return c
	}
	out := make([]token, 0, len(c))
	for _, want := range []Level{us[0].level, flip(us[0].level)} {
		for _, u := range us {
			if u.level == want {
				out = append(out, c[u.lo:u.hi]...)
			}
		}
	}
	// What is on either side of the cluster measures its distance from the
	// token at the end of it, and after the reordering that token is no longer
	// the one that reaches furthest. Page 140 read "$A^{(\mathfrak{a})}_s$ ,
	// hence" with a space before the comma, because the s the subscript now
	// ends on stands where the closing bracket used to and the comma looked a
	// word away from it. So the ends of the cluster keep the reach the cluster
	// had.
	left, right := c[0].left, c[0].right
	for _, t := range c {
		left, right = min(left, t.left), max(right, t.right)
	}
	out[0].left, out[len(out)-1].right = left, right
	return out
}

// units cuts a cluster at every token written at the level of the cluster
// itself, so that what is written below that level stays with the token it
// hangs off.
func units(c []token) []unit {
	top := c[0].depth
	for _, t := range c {
		if t.depth < top {
			top = t.depth
		}
	}
	var us []unit
	for i, t := range c {
		if t.depth > top && len(us) > 0 {
			u := &us[len(us)-1]
			u.hi = i + 1
			u.left, u.right = min(u.left, t.left), max(u.right, t.right)
			continue
		}
		level := t.level
		// A prime sits on the line of whatever it is written against and takes
		// the level of the group it is in, which is what emit does with it too.
		if level == Base && len(us) > 0 {
			level = us[len(us)-1].level
		}
		us = append(us, unit{lo: i, hi: i + 1, level: level, left: t.left, right: t.right})
	}
	return us
}

// stacked reports whether a cluster is one superscript and one subscript that
// the page handed back interleaved.
//
// Three things are asked, and each of them is what tells the inverse from the
// flattened matrix that arrives looking like it.
//
// One script has to be written in more than one piece, since a cluster whose
// levels each arrive whole is not a stack that came apart and wants nothing
// done to it.
//
// The pieces of a script have to touch, since they were one thing before the
// other script was laid across them. This is what refuses the matrix: its
// columns stand apart by the space the page sets between them.
//
// One script has to lie inside what the other spans, since that is what being
// laid across something is. This is what refuses the pages where the line
// gathering has pulled in the limits of the display above, which touch and
// interleave but sit beside each other rather than one within the other.
//
// A cluster that carries the same box at both levels is refused whatever else
// it does. Four pages of chapter VIII print a sum under a sum and the two
// limits arrive at the same place on the line, one read as above it and one as
// below, and merging them would write the limit out twice and say nothing about
// the two lines that were run together to make it.
func stacked(c []token, us []unit) bool {
	if len(us) < 3 {
		return false
	}
	var seq []Level
	for _, u := range us {
		if u.level != Sup && u.level != Sub {
			return false
		}
		if len(seq) == 0 || seq[len(seq)-1] != u.level {
			seq = append(seq, u.level)
		}
	}
	if len(seq) < 3 {
		return false
	}
	for _, level := range []Level{Sup, Sub} {
		if !touching(us, level) {
			return false
		}
	}
	if !within(span(us, Sup), span(us, Sub)) && !within(span(us, Sub), span(us, Sup)) {
		return false
	}
	for i, a := range c {
		for _, b := range c[i+1:] {
			if a.level != b.level && a.left == b.left && a.right == b.right {
				return false
			}
		}
	}
	return true
}

// touching reports whether the pieces one level is written in stand close
// enough together to have been one piece before the other level cut them.
//
// The gap is only asked for across a break in the level, since two units
// written one after the other at the same level were never apart, and the
// space a page sets inside a script is wider than the space it leaves where it
// laid one script over another.
func touching(us []unit, level Level) bool {
	prev, right := -1, 0
	for i, u := range us {
		if u.level != level {
			continue
		}
		if prev >= 0 && i > prev+1 && u.left-right > restackGap {
			return false
		}
		if prev < 0 || u.right > right {
			right = u.right
		}
		prev = i
	}
	return true
}

// span is what one level of a cluster reaches across.
func span(us []unit, level Level) [2]int {
	out, first := [2]int{}, true
	for _, u := range us {
		if u.level != level {
			continue
		}
		if first {
			out, first = [2]int{u.left, u.right}, false
			continue
		}
		out = [2]int{min(out[0], u.left), max(out[1], u.right)}
	}
	return out
}

// within reports whether the first span lies inside the second.
func within(a, b [2]int) bool { return a[0] >= b[0] && a[1] <= b[1] }

func flip(l Level) Level {
	if l == Sup {
		return Sub
	}
	return Sup
}
