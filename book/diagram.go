package book

import "strings"

// Centring the vertical arrows of a commutative diagram under the objects they
// hang from.
//
// A diagram is read off the page as an array, and a cell of it comes out as
// \downarrow l_G \otimes i, the arrow and the name of the map together in one
// cell. An array centres the whole of a cell in its column, so the arrow lands
// left of the column's centre by half the width of its own label, and a row of
// three arrows carrying three labels of three widths lands in three different
// places. Page 18 of the English Commutative Algebra is the clearest case: the
// three arrows of the first diagram sit visibly out of line with the three
// tensor products above them, and the diagram reads as a table of symbols
// rather than as a square.
//
// The printing centres the arrow and hangs the label beside it. amscd and
// tikz-cd both do the same thing and both do it the same way, by setting the
// label in a box of no width so that it never enters the column's measurement.
// This does that, with the label at script size, which is the size \xrightarrow
// already sets the labels of the horizontal arrows at, so the two kinds of
// label in one diagram stop being two different sizes.
//
// A box of no width overlaps whatever is beside it, so the rewrite fires only
// where the neighbouring cell is empty and the label has somewhere to hang. A
// diagram written with a column between its objects for the horizontal arrows,
// which is how nearly every one of them is written, satisfies that at every
// arrow. One written without keeps the setting it had, which is out of line and
// legible, rather than gaining a label printed over the next column.

// verticalArrows are the arrows a diagram hangs down or up from a row. The
// horizontal ones are not here: \xrightarrow and \xleftarrow already carry
// their label above the arrow and are already centred on it.
var verticalArrows = []string{`\Downarrow`, `\Uparrow`, `\downarrow`, `\uparrow`}

// The longest thing that will be treated as the name of a map. A cell holding
// an arrow and then a sentence is not a labelled arrow, it is a row of the
// diagram that lost its tab, and hanging it in a box of no width would print it
// across the rest of the line.
const maxArrowLabel = 48

// diagrams rewrites the labelled vertical arrows of every array in one math
// span. Anything it does not recognise it leaves exactly as it found it.
func diagrams(tex string) string {
	rs := []rune(tex)
	var out strings.Builder
	for i := 0; i < len(rs); {
		name, after := beginName(rs, i)
		if name != "array" {
			out.WriteRune(rs[i])
			i++
			continue
		}
		open := skipOptional(rs, after)
		spec, body := group(rs, open)
		if body == open {
			out.WriteString(string(rs[i:after]))
			i = after
			continue
		}
		end, close := environmentEnd(rs, body, name)
		if end < 0 {
			out.WriteString(string(rs[i:after]))
			i = after
			continue
		}
		out.WriteString(`\begin{array}`)
		out.WriteString(string(rs[after:open]))
		out.WriteString(`{` + spec + `}`)
		// Innermost first, so a diagram drawn inside a cell of another one is
		// already done by the time this one is read.
		out.WriteString(hangLabels(diagrams(string(rs[body:end]))))
		out.WriteString(string(rs[end:close]))
		i = close
	}
	return out.String()
}

// A cell of an array body, as a pair of rune offsets into it and where in the
// array it sits.
type cellSpan struct {
	row, col   int
	start, end int
}

// usedBeyond is whether the array holds anything at or past a column, looking
// the way step points. Nothing past it means the edge of the array, and past
// the edge of a centred display is white paper.
func usedBeyond(used map[int]bool, col, step int) bool {
	for c := range used {
		if step > 0 && c >= col || step < 0 && c <= col {
			return true
		}
	}
	return false
}

// hangLabels rewrites the cells of one array body that hold a labelled vertical
// arrow with room beside them for the label.
func hangLabels(body string) string {
	rs := []rune(body)
	cs := cellSpans(rs)
	// The text of every cell, read before anything is rewritten. Reading it
	// afterwards would read a cell through an offset that a rewrite earlier in
	// the loop has already moved.
	text := make([]string, len(cs))
	for k, c := range cs {
		text[k] = string(rs[c.start:c.end])
	}
	// Which columns of the array hold anything at all. A column that is empty in
	// every row has no width, so a label hung into it is a label hung over
	// whatever comes after it. The columns of a diagram that carry the
	// horizontal arrows are the wide ones and they are the ones the labels
	// hang into.
	used := map[int]bool{}
	for k, c := range cs {
		if strings.TrimSpace(text[k]) != "" {
			used[c.col] = true
		}
	}
	// Room is either the margin of the display, when the array holds nothing
	// that way at all, or a column that has a width of its own and holds
	// nothing in this row.
	room := func(k, step int) bool {
		col := cs[k].col + step
		if !usedBeyond(used, col, step) {
			return true
		}
		if !used[col] {
			return false
		}
		n := k + step
		if n < 0 || n >= len(cs) || cs[n].row != cs[k].row {
			return true
		}
		return strings.TrimSpace(text[n]) == ""
	}
	// Backwards, so that an offset taken before a rewrite is still an offset
	// into the text the rewrite has not reached yet.
	for k := len(cs) - 1; k >= 0; k-- {
		arrow, label, after, ok := arrowCell(text[k])
		if !ok {
			continue
		}
		var hung string
		switch {
		case after && room(k, +1):
			hung = `\bvarrowr{` + arrow + `}{` + label + `}`
		case !after && room(k, -1):
			hung = `\bvarrowl{` + arrow + `}{` + label + `}`
		default:
			continue
		}
		rs = append(rs[:cs[k].start], append([]rune(" "+hung+" "), rs[cs[k].end:]...)...)
	}
	return string(rs)
}

// arrowCell reads a cell that holds a vertical arrow and the name of the map
// and nothing else. after says the name is written to the right of the arrow,
// which is the common way round; a diagram that writes it to the left, as
// a \downarrow, is the other.
func arrowCell(cell string) (arrow, label string, after, ok bool) {
	s := strings.TrimSpace(cell)
	for _, a := range verticalArrows {
		if rest, cut := strings.CutPrefix(s, a); cut && !startsLetter(rest) {
			rest = strings.TrimSpace(rest)
			return a, rest, true, plainLabel(rest)
		}
		if rest, cut := strings.CutSuffix(s, a); cut {
			rest = strings.TrimSpace(rest)
			return a, rest, false, plainLabel(rest)
		}
	}
	return "", "", false, false
}

// startsLetter is how \downarrow is told from \downarrowfoo, since TeX reads a
// control word up to the first character that is not a letter.
func startsLetter(s string) bool {
	if s == "" {
		return false
	}
	c := s[0]
	return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
}

// plainLabel is whether what is left beside the arrow is the name of a map. An
// empty cell is an arrow with no name and wants no box; anything holding a tab,
// a row break or an environment of its own is not a name.
func plainLabel(s string) bool {
	if s == "" || len([]rune(s)) > maxArrowLabel {
		return false
	}
	return !strings.ContainsAny(s, "&") && !strings.Contains(s, `\\`) &&
		!strings.Contains(s, `\begin{`) && !strings.Contains(s, `\end{`)
}

// cellSpans finds every cell of an array body. A tab or a row break inside a
// braced group or inside a nested environment belongs to that one and not to
// this array, which is the same rule rows() counts by.
func cellSpans(rs []rune) []cellSpan {
	var out []cellSpan
	row, col, start, depth, env := 0, 0, 0, 0, 0
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '{':
			depth++
		case '}':
			depth--
		case '&':
			if depth == 0 && env == 0 {
				out = append(out, cellSpan{row, col, start, i})
				col++
				start = i + 1
			}
		case '\\':
			if i+1 < len(rs) && rs[i+1] == '\\' {
				if depth == 0 && env == 0 {
					out = append(out, cellSpan{row, col, start, i})
					row, col = row+1, 0
					start = i + 2
				}
				i++
				continue
			}
			if n, _ := beginName(rs, i); n != "" {
				env++
				continue
			}
			if strings.HasPrefix(string(rs[i:min(i+5, len(rs))]), `\end{`) {
				if env > 0 {
					env--
				}
				continue
			}
			i++ // whatever the command was, its first character is not a tab
		}
	}
	return append(out, cellSpan{row, col, start, len(rs)})
}
