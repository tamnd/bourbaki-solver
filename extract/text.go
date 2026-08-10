package extract

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Rendering a line is deciding where the mathematics starts and stops and then
// writing it as LaTeX.
//
// Most of that is settled by the fonts. What is not is that Bourbaki sets its
// rings and its modules in upright roman, the same font the prose is set in, so
// a run can hold the end of a formula and the start of a sentence at once:
// pdftohtml gives "= 1. If M were not of finite" as one run. Such a run is cut
// at the first word that cannot be part of a formula, and the cut is made
// conservatively, since a word wrongly left inside dollar signs is a sentence
// set in italic and a symbol wrongly left outside is only a symbol.

// token is one piece of a rendered line.
type token struct {
	text  string
	class Class
	level Level
	depth int
	left  int
	right int
	math  bool
}

// Render writes one line as Markdown with LaTeX mathematics.
func Render(l Line) string {
	toks := tokens(l)
	toks = extend(toks)
	return statementHead(emit(toks))
}

// headWord is the word a statement opens with. The four Bourbaki sets in small
// capitals are told by their font; the rest are set in the italic of the
// statement itself and are told by the word and the dash after it.
var headRE = regexp.MustCompile(`^(Definition|Proposition|Theorem|Lemma|Corollary|Corollaries|Remarks?|Examples?|Scholium|Criterion)( [0-9]+)?\.( —| --)`)

// statementHead bolds the head of a statement, which is how the corpus marks
// where one begins. Bourbaki prints it in small capitals or in italic and
// follows it with a number and a dash, and the dash stays outside the bold, as
// it is punctuation of the page rather than part of the name.
func statementHead(s string) string {
	m := headRE.FindStringSubmatch(s)
	if m == nil {
		return s
	}
	name := m[1] + m[2] + "."
	return "**" + name + "**" + s[len(name):]
}

// tokens turns the runs of a line into pieces of text, with the glyphs each
// font could not spell out put back.
func tokens(l Line) []token {
	var toks []token
	var accents []Run
	for _, r := range l.Runs {
		if family(r.Spec) == "CMEX" {
			text, acc := cmexText(r)
			if acc {
				accents = append(accents, r)
				continue
			}
			if text == "" {
				continue
			}
			toks = append(toks, token{text: text, class: ClassMath, level: r.Level,
				left: r.Left, right: r.Right(), math: true})
			continue
		}
		text := runText(r)
		if text == "" {
			continue
		}
		if r.Level != Base && footnoteMark(r.Text) {
			// A footnote reference is a superscript and is not an exponent.
			toks = append(toks, token{text: "[^" + strings.Trim(r.Text, "()") + "]",
				class: ClassText, left: r.Left, right: r.Right()})
			continue
		}
		level, depth := r.Level, r.Depth
		if level == Sup && strings.Trim(text, "'") == "" {
			// A prime is drawn above the line and written on it. It comes back
			// down one level and not all the way to the line, since the L of
			// A_{(L')} is itself inside an index and a prime that dropped to
			// the line would close the index before the bracket did.
			level, depth = Base, max(depth-1, 0)
		}
		toks = append(toks, token{text: text, class: r.Class, level: level, depth: depth,
			left: r.Left, right: r.Right(), math: r.Class.Math() || r.Level != Base})
	}
	return place(toks, accents)
}

// footnoteMark reports whether a run is a reference to a footnote, which this
// volume sets as a parenthesised number raised above the line. Everything else
// raised above the line is an exponent.
func footnoteMark(s string) bool {
	s = strings.TrimSpace(s)
	if len(s) < 3 || s[0] != '(' || s[len(s)-1] != ')' {
		return false
	}
	for _, c := range s[1 : len(s)-1] {
		if c < '0' || c > '9' {
			return false
		}
	}
	return true
}

// place puts each accent over the token it was drawn over. An accent is a glyph
// of its own sitting at the position of the letter it decorates, so the letter
// is the token it overlaps.
func place(toks []token, accents []Run) []token {
	for _, a := range accents {
		latex, _, ok := CMEX(first(a.Text))
		if !ok {
			continue
		}
		for i := range toks {
			if toks[i].right < a.Left || toks[i].left > a.Right() {
				continue
			}
			toks = append(toks[:i], append(accent(toks[i], a, latex), toks[i+1:]...)...)
			break
		}
	}
	return toks
}

// accent puts one accent over the part of a token it was drawn over, and gives
// back the token in the pieces that leaves.
//
// The token has to be cut because a run is not a word. Bourbaki writes "let Pe
// be the dual" and pdftohtml hands back "P be the dual" as one run with the
// tilde beside it, so an accent that takes the whole token takes the sentence
// with it, and the page comes out with eight words of English inside dollar
// signs. Which characters the accent covers is read off its box against the
// box of the token, which is what the accent is drawn to line up with.
func accent(t token, a Run, latex string) []token {
	rs := []rune(t.text)
	w := t.right - t.left
	// A token already rendered as LaTeX cannot be cut by character, since its
	// characters are no longer the ones on the page.
	if len(rs) < 2 || w <= 0 || strings.ContainsAny(t.text, `\{}`) {
		t.text = latex + "{" + strings.TrimSpace(t.text) + "}"
		t.math, t.class = true, ClassMath
		return []token{t}
	}
	at := func(x int) int { return min(max((x-t.left)*len(rs)/w, 0), len(rs)) }
	lo, hi := at(a.Left), at(a.Right())
	lo = min(lo, len(rs)-1)
	// An accent sits over a single letter. Anything wider than that is the
	// division of the box being off, not a tilde over a sentence.
	hi = min(max(hi, lo+1), min(lo+2, len(rs)))
	for lo < hi && rs[lo] == ' ' {
		lo++
	}
	for hi > lo && rs[hi-1] == ' ' {
		hi--
	}
	if hi <= lo {
		return []token{t}
	}
	cut := func(from, to int) token {
		p := t
		p.text = string(rs[from:to])
		p.left = t.left + w*from/len(rs)
		p.right = t.left + w*to/len(rs)
		return p
	}
	var out []token
	if lo > 0 {
		out = append(out, cut(0, lo))
	}
	mid := cut(lo, hi)
	mid.text = latex + "{" + strings.TrimSpace(mid.text) + "}"
	mid.math, mid.class = true, ClassMath
	out = append(out, mid)
	if hi < len(rs) {
		out = append(out, cut(hi, len(rs)))
	}
	return out
}

// cmexText renders a run of CMEX, which is one glyph in practice.
func cmexText(r Run) (text string, accent bool) {
	var b strings.Builder
	for _, c := range r.Text {
		if c == ' ' {
			// Position 0x20 of CMEX is a large left parenthesis, and poppler
			// prints it as the character its code stands for.
			b.WriteString("(")
			continue
		}
		latex, acc, ok := CMEX(c)
		if !ok {
			continue
		}
		if acc {
			return latex, true
		}
		b.WriteString(spaced(latex))
	}
	return strings.TrimSpace(b.String()), false
}

// runText renders a run that is not CMEX.
func runText(r Run) string {
	s := r.Text
	switch family(r.Spec) {
	case "rsfs":
		return wrapLetters(s, `\mathscr`)
	case "EUFM":
		return wrapLetters(s, `\mathfrak`)
	case "CMSSDC":
		return wrapLetters(s, `\mathsf`)
	case "LMMathSymbols":
		return symbols(s)
	}
	if r.Class == ClassBold {
		return wrapLetters(s, `\mathbf`)
	}
	if !r.Class.Math() && r.Level == Base {
		return s
	}
	if r.Class == ClassMath && r.Bold {
		// Bourbaki sets a few of its Greek letters bold, and a bold letter of
		// the mathematics italic is a different letter from the upright one.
		return `\boldsymbol{` + runes(s, nil) + `}`
	}
	return runes(s, nil)
}

// symbols renders a run of the mathematics symbol font, where the letters are
// not letters: an "i" is a closing angle bracket and a "0" is a prime.
//
// The scan is left to right and takes the longest match, since replacing the
// pairs first and the single glyphs afterwards would go on to read the
// backslashes of its own output.
func symbols(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); {
		matched := false
		for _, p := range symbolPairs {
			if strings.HasPrefix(s[i:], p.from) {
				b.WriteString(p.to)
				i += len(p.from)
				matched = true
				break
			}
		}
		if matched {
			continue
		}
		r, n := utf8.DecodeRuneInString(s[i:])
		i += n
		if latex, ok := cmsy[r]; ok {
			b.WriteString(latex)
			continue
		}
		if latex, ok := unicodeMath[r]; ok {
			b.WriteString(spaced(latex))
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// runes maps a run character by character, taking the font's own table first
// where it has one.
func runes(s string, own map[rune]string) string {
	var b strings.Builder
	for _, c := range s {
		if latex, ok := own[c]; ok {
			b.WriteString(latex)
			continue
		}
		if latex, ok := unicodeMath[c]; ok {
			b.WriteString(spaced(latex))
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}

// spaced separates a LaTeX command from whatever follows it. Without this the
// n of "\leqslant n" runs into the command and names one that does not exist.
func spaced(s string) string {
	if len(s) > 1 && s[0] == '\\' && isLetter(rune(s[len(s)-1])) {
		return s + " "
	}
	return s
}

func isLetter(c rune) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// wrapLetters puts every letter of a run inside a font command, which is how
// the script, fraktur, sans serif and bold alphabets are written in LaTeX.
func wrapLetters(s, cmd string) string {
	var b strings.Builder
	for _, c := range s {
		if unicode.IsLetter(c) {
			b.WriteString(cmd + "{" + string(c) + "}")
			continue
		}
		b.WriteRune(c)
	}
	return b.String()
}

func first(s string) rune {
	for _, c := range s {
		return c
	}
	return 0
}

// extend grows each stretch of mathematics over the runs of upright roman next
// to it that can only be mathematics: the digits, the operators and the single
// letters Bourbaki names its rings and modules with.
func extend(toks []token) []token {
	out := make([]token, 0, len(toks))
	for i, t := range toks {
		if t.math || t.class == ClassHead {
			out = append(out, t)
			continue
		}
		before := i > 0 && toks[i-1].math && near(toks[i-1], t)
		after := i+1 < len(toks) && toks[i+1].math && near(t, toks[i+1])
		if !before && !after {
			out = append(out, t)
			continue
		}
		out = append(out, split(t, before, after)...)
	}
	return out
}

// near reports whether two tokens are close enough to be part of one formula.
// A gap wider than a word space means the formula ended and a sentence carried
// on.
func near(a, b token) bool { return b.left-a.right <= 8 }

// split cuts a run of upright roman into the part that belongs to the formula
// beside it and the part that is prose. before and after say which side the
// formula is on.
func split(t token, before, after bool) []token {
	words := strings.Fields(t.text)
	if len(words) == 0 {
		return []token{t}
	}
	lo, hi := 0, len(words)
	if before {
		for lo < hi && mathWord(words[lo]) {
			lo++
			// A full stop ends the sentence and the formula with it. What
			// follows is prose, whatever it looks like.
			if strings.HasSuffix(words[lo-1], ".") {
				break
			}
		}
		// The full stop itself is read and not typeset, so it goes out of the
		// formula, or "n = 1. If M" sets a full stop in mathematics italic.
		if lo > 0 && lo < hi && strings.HasSuffix(words[lo-1], ".") {
			words[lo-1] = strings.TrimSuffix(words[lo-1], ".")
			words = append(words[:lo:lo], append([]string{"."}, words[lo:]...)...)
			hi++
		}
	}
	if after {
		for hi > lo && mathWord(words[hi-1]) {
			hi--
		}
	}
	if lo == 0 && hi == len(words) {
		return []token{t}
	}
	var out []token
	add := func(ws []string, math bool) {
		if len(ws) == 0 {
			return
		}
		n := token{text: strings.Join(ws, " "), class: t.class, level: t.level,
			depth: t.depth, left: t.left, right: t.right, math: math}
		if math {
			n.class = ClassMath
		}
		out = append(out, n)
	}
	add(words[:lo], true)
	add(words[lo:hi], false)
	add(words[hi:], true)
	// The pieces were words of one run with spaces between them, and the
	// spaces have to survive the cut: the formula and the sentence beside it
	// are not written against each other.
	for i := 1; i < len(out); i++ {
		if strings.ContainsRune(`.,;:)]`, rune(out[i].text[0])) {
			continue
		}
		out[i].text = " " + out[i].text
	}
	// The spaces at the ends of the original run are what separate it from its
	// neighbours, so they are kept.
	if len(out) > 0 {
		if strings.HasPrefix(t.text, " ") {
			out[0].text = " " + out[0].text
		}
		if strings.HasSuffix(t.text, " ") {
			out[len(out)-1].text += " "
		}
	}
	return out
}

// mathWord reports whether a word of upright roman can only be part of a
// formula. Anything holding two letters in a row is a word of English and stops
// the formula, which is what keeps "If" and "for" outside the dollar signs.
// Only a capital stands as a letter on its own. Bourbaki names its rings and
// its modules with capitals, and a lowercase letter alone in upright roman is
// the article "a", not a variable: the variables are set in the mathematics
// italic and never reach here.
func mathWord(w string) bool {
	if w == "" {
		return false
	}
	// A reference opens on a parenthesis and a chapter number and reads as a
	// formula to everything below: (I, §4, No. 6, p. 41, Theorem 4). The comma
	// inside the parenthesis is what gives it away, since a formula that opens
	// a parenthesis closes it before it comes to one.
	if w[0] == '(' && strings.HasSuffix(w, ",") {
		return false
	}
	letters := 0
	for _, c := range w {
		switch {
		case unicode.IsUpper(c):
			letters++
			if letters > 1 {
				return false
			}
		case unicode.IsDigit(c), strings.ContainsRune(`+-−=<>()[]{}|/*,.;:'`, c):
			letters = 0
		default:
			return false
		}
	}
	return true
}

// emit writes the tokens out, opening and closing the dollar signs and putting
// the superscripts and subscripts back where they belong.
//
// Two things are put back that pdftohtml does not report. A space between two
// runs that stand apart on the page, since poppler gives the gap and not the
// space that made it. And a space after a LaTeX command, without which the n of
// "\leqslant n" runs into the command and names one that does not exist.
func emit(toks []token) string {
	var out string
	inMath := false
	prev := token{right: -1}
	// open holds the mark of every index group still open, so that the index of
	// an index closes in the right order and M_{i_1} does not come out as Mi1.
	var open []Level
	var at []int // where each group's brace was written
	closeGroups := func(to int) {
		for len(open) > to {
			out = strings.TrimRight(out, " ")
			// An index of one character needs no braces, and M_i reads better
			// than M_{i} in a file people are meant to read.
			if body := out[at[len(at)-1]+1:]; len([]rune(body)) == 1 {
				out = out[:at[len(at)-1]] + body
			} else {
				out += "}"
			}
			open, at = open[:len(open)-1], at[:len(at)-1]
		}
	}
	closeMath := func() {
		if !inMath {
			return
		}
		closeGroups(0)
		out = strings.TrimRight(out, " ")
		// A formula that ends in the punctuation of the sentence around it
		// leaves the punctuation outside, where it is read and not typeset.
		tail := ""
		for len(out) > 0 && strings.ContainsRune(",.;", rune(out[len(out)-1])) {
			tail = out[len(out)-1:] + tail
			out = out[:len(out)-1]
		}
		out = strings.TrimRight(out, " ") + "$" + tail
		inMath = false
	}
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		text := t.text
		gap := prev.right >= 0 && t.left-prev.right >= 3
		if !t.math {
			closeMath()
			if gap {
				text = " " + strings.TrimLeft(text, " ")
			}
			out += text
			prev = t
			continue
		}
		if !inMath {
			if gap || strings.HasPrefix(text, " ") {
				out = strings.TrimRight(out, " ") + " "
			}
			text = strings.TrimLeft(text, " ")
			out += "$"
			inMath = true
		}
		// An index goes inside braces, and stays open for as long as the runs
		// keep coming at that size: i, ∈ and I side by side are one index and
		// not three.
		closeGroups(t.depth)
		// A prime sits on the line of whatever it is written against, so it
		// takes the level of the group it is in rather than opening one of its
		// own: the prime of A_{(L')} is neither above nor below the L.
		if t.depth > 0 && t.level == Base && len(open) >= t.depth {
			t.level = open[t.depth-1]
		}
		if t.depth > len(open) {
			for len(open) < t.depth {
				out = strings.TrimRight(out, " ") + mark(t.level) + "{"
				open, at = append(open, t.level), append(at, len(out)-1)
			}
		} else if t.depth > 0 && open[len(open)-1] != t.level {
			closeGroups(t.depth - 1)
			out += mark(t.level) + "{"
			open, at = append(open, t.level), append(at, len(out)-1)
		}
		if t.depth > 0 {
			text = strings.TrimSpace(text)
		}
		if openMacro(out) && startsWord(text) {
			out += " "
		}
		out += text
		prev = t
	}
	closeMath()
	return strings.TrimRight(out, " ")
}

// mark is how a level is written in LaTeX.
func mark(l Level) string {
	if l == Sub {
		return "_"
	}
	return "^"
}

// openMacro reports whether a string ends in a LaTeX command still taking
// letters.
func openMacro(s string) bool {
	for i := len(s) - 1; i >= 0; i-- {
		switch {
		case s[i] == '\\':
			return i < len(s)-1
		case isLetter(rune(s[i])):
		default:
			return false
		}
	}
	return false
}

func startsWord(s string) bool {
	if s == "" {
		return false
	}
	c := rune(s[0])
	return isLetter(c) || c >= '0' && c <= '9'
}
