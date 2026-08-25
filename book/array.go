package book

import (
	"strconv"
	"strings"
)

// An array whose preamble is narrower than its widest row.
//
// KaTeX, which is what the site renders with, invents the missing columns and
// draws the table. LaTeX refuses: "Extra alignment tab has been changed to
// \cr", and the run stops. The corpus has 544 arrays and tabulars and 66 of
// them are in this state, spread over every language, which is what you would
// expect from a defect that the reader never sees and the site never reports.
//
// Most of them are commutative diagrams that were read off a scanned page with
// arrows in the margin, so the row genuinely has the cells it claims and it is
// the preamble that came out short. Two of them, the multiplication tables in
// alg III § 1, have a rule in the preamble and a header row that counts the
// stub column twice.
//
// So the preamble is widened here rather than the rows cut, and every widening
// is reported. The report matters more than the repair: a diagram that has lost
// a column has probably lost an arrow with it, and that is a thing to go and
// look at on the page, not something this package can work out.
type wideArray struct {
	Env    string
	Spec   string
	Cols   int
	Widest int
}

func (a wideArray) String() string {
	where := a.Env
	if a.Spec != "" {
		where += "{" + a.Spec + "}"
	}
	return where + " has " + strconv.Itoa(a.Cols) + " columns and a row with " +
		strconv.Itoa(a.Widest) + " cells"
}

var arrayEnvs = map[string]bool{"array": true, "tabular": true, "tabularx": true}

// cases has no preamble to widen: amsmath fixes it at two columns, the value
// and the condition. Four of the corpus's 180 have three, which is a row that
// was read off a page where the printing set three things side by side inside
// one brace. Those become an array with the brace put back by hand, which is
// what cases is underneath anyway.
var casesEnvs = map[string]bool{"cases": true, "dcases": true}

const casesCols = 2

// widen rewrites every array preamble in one math span that is narrower than
// its own widest row, innermost first, and says what it did.
func widen(tex string) (string, []wideArray) {
	var found []wideArray
	rs := []rune(tex)
	var out strings.Builder
	for i := 0; i < len(rs); {
		name, after := beginName(rs, i)
		if casesEnvs[name] {
			end, close := environmentEnd(rs, after, name)
			if end < 0 {
				out.WriteRune(rs[i])
				i++
				continue
			}
			inner, deeper := widen(string(rs[after:end]))
			found = append(found, deeper...)
			if widest := rows(inner); widest > casesCols {
				found = append(found, wideArray{Env: name, Cols: casesCols, Widest: widest})
				out.WriteString(`\left\{\begin{array}{@{}l` +
					strings.Repeat(`@{\quad}l`, widest-1) + `@{}}` + inner + `\end{array}\right.`)
			} else {
				out.WriteString(`\begin{` + name + `}` + inner + string(rs[end:close]))
			}
			i = close
			continue
		}
		if !arrayEnvs[name] {
			out.WriteRune(rs[i])
			i++
			continue
		}
		open := after
		// tabularx takes a width before the preamble, and any of them may take
		// a [t] or [b] position first.
		if name == "tabularx" {
			open = skipOptional(rs, open)
			_, open = group(rs, open)
		} else {
			open = skipOptional(rs, open)
		}
		spec, body := group(rs, open)
		if body == open {
			// no preamble where one has to be, which is a different bug and
			// one the typesetter reports on its own terms.
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
		inner, deeper := widen(string(rs[body:end]))
		found = append(found, deeper...)
		cols, widest := specColumns(spec), rows(inner)
		if widest > cols {
			found = append(found, wideArray{Env: name, Spec: spec, Cols: cols, Widest: widest})
			spec = pad(spec, widest-cols)
		}
		out.WriteString(`\begin{` + name + `}`)
		out.WriteString(string(rs[after:open]))
		out.WriteString(`{` + spec + `}`)
		out.WriteString(inner)
		out.WriteString(string(rs[end:close]))
		i = close
	}
	return out.String(), found
}

// beginName reads a \begin{name} at i and returns the name and the index just
// past the closing brace, or an empty name if there is no \begin there.
func beginName(rs []rune, i int) (string, int) {
	const b = `\begin{`
	if i+len(b) > len(rs) || string(rs[i:i+len(b)]) != b {
		return "", i
	}
	j := i + len(b)
	for j < len(rs) && rs[j] != '}' {
		if rs[j] == '\\' || rs[j] == '{' {
			return "", i
		}
		j++
	}
	if j >= len(rs) {
		return "", i
	}
	return string(rs[i+len(b) : j]), j + 1
}

// environmentEnd finds the \end{name} that closes the environment opened
// before i, counting the ones nested inside. It returns where the body stops
// and where the \end stops.
func environmentEnd(rs []rune, i int, name string) (int, int) {
	end := `\end{` + name + `}`
	depth := 0
	for j := i; j < len(rs); j++ {
		if rs[j] != '\\' {
			continue
		}
		if n, _ := beginName(rs, j); n == name {
			depth++
			continue
		}
		if j+len(end) <= len(rs) && string(rs[j:j+len(end)]) == end {
			if depth == 0 {
				return j, j + len(end)
			}
			depth--
		}
	}
	return -1, -1
}

// group reads a braced group starting at i and returns its contents and the
// index just past the closing brace. If i is not a brace it returns i unmoved,
// which is how the caller tells that the group was not there.
func group(rs []rune, i int) (string, int) {
	if i >= len(rs) || rs[i] != '{' {
		return "", i
	}
	depth, j := 0, i
	for ; j < len(rs); j++ {
		switch rs[j] {
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				return string(rs[i+1 : j]), j + 1
			}
		case '\\':
			j++
		}
	}
	return "", i
}

func skipOptional(rs []rune, i int) int {
	if i >= len(rs) || rs[i] != '[' {
		return i
	}
	for j := i; j < len(rs); j++ {
		if rs[j] == ']' {
			return j + 1
		}
	}
	return i
}

// specColumns counts the columns a preamble declares. The rules and the @, >
// and < inserts do not count, p, m and b take a width, and *{n}{...} repeats.
func specColumns(spec string) int {
	rs := []rune(spec)
	n := 0
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case 'l', 'c', 'r', 'X':
			n++
		case 'p', 'm', 'b':
			n++
			_, j := group(rs, i+1)
			i = j - 1
		case '@', '>', '<', '!':
			_, j := group(rs, i+1)
			i = j - 1
		case '*':
			count, j := group(rs, i+1)
			sub, k := group(rs, j)
			times, err := strconv.Atoi(strings.TrimSpace(count))
			if err != nil || times < 0 {
				times = 1
			}
			n += times * specColumns(sub)
			i = k - 1
		}
	}
	return n
}

// rows counts the cells in the widest row of an array body. A & inside a nested
// environment or a braced group belongs to that one and not to this, and
// \multicolumn takes the number of columns it says it does.
func rows(body string) int {
	rs := []rune(body)
	widest, cells, depth, env := 0, 1, 0, 0
	stop := func() {
		if cells > widest {
			widest = cells
		}
		cells = 1
	}
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '{':
			depth++
		case '}':
			depth--
		case '&':
			if depth == 0 && env == 0 {
				cells++
			}
		case '\\':
			if i+1 < len(rs) && rs[i+1] == '\\' {
				if depth == 0 && env == 0 {
					stop()
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
			if depth == 0 && env == 0 && strings.HasPrefix(string(rs[i:min(i+12, len(rs))]), `\multicolumn`) {
				count, _ := group(rs, i+len(`\multicolumn`))
				if k, err := strconv.Atoi(strings.TrimSpace(count)); err == nil && k > 1 {
					cells += k - 1
				}
			}
			i++ // whatever the command was, its first character is not a tab
		}
	}
	stop()
	return widest
}

// pad adds n columns to a preamble, repeating whatever the last one was along
// with the rule in front of it if there was one, so that a ruled table stays
// ruled and a plain one stays plain.
func pad(spec string, n int) string {
	rs := []rune(spec)
	end := len(rs)
	for end > 0 && rs[end-1] == ' ' {
		end--
	}
	start := end
	for start > 0 && !strings.ContainsRune("lcrXpmb", rs[start-1]) {
		start--
	}
	if start == 0 {
		return spec + strings.Repeat("c", n)
	}
	start--
	for start > 0 && (rs[start-1] == '|' || rs[start-1] == ' ') {
		start--
	}
	return spec + strings.Repeat(string(rs[start:end]), n)
}
