package book

import (
	"maps"
	"regexp"
	"strings"
)

// The corpus has TeX control sequences sitting in its prose, outside any math
// span, and there are 157 distinct ones over 9800 occurrences. They are the same
// problem unicode.go describes, written the other way round: an OCR read a
// formula in the middle of a sentence and set it as text, except that here it
// came out as \alpha rather than as α.
//
// They do not all mean the same thing, and that is the whole reason this file
// exists rather than one rule for all backslashes.
//
//   - 2155 of them are \S, which is not a mistake at all. It is text-mode LaTeX
//     for the section sign and it is correct where it stands, and a renderer
//     that escaped it would print a literal backslash-S in two thousand places
//     where the book says "see § 2".
//   - About 5900 are mathematics: \in, \otimes, \mathbf{Z}, \alpha, \frac. They
//     belong inside dollars, they are not there, and setting them as text is a
//     LaTeX error rather than a bad-looking page.
//   - About 250 are the rest of text-mode LaTeX: \footnote, \emph, \textit,
//     \quad, and the two accents \v and \c.
//   - 44 are fragments of a formula that was cut in half. \left with no \right,
//     \begin with no environment around it, \bigl on its own. Those cannot be
//     repaired from here, because the other half is not in the file.
//
// So: a counted table, the way unicode.go is a counted table. Text mode passes
// through, mathematics gets its dollars, and anything the table does not know
// is set as the literal characters it is made of and handed to the audit. The
// one thing that does not happen is a guess, because a control sequence guessed
// wrong does not look wrong on the page, it silently sets the wrong symbol.
//
// The real repair is in the corpus and it is the same repair unicode.go wants:
// these belong inside dollars. This is what the book does until somebody does
// that, and the audit counts them every build so that nobody can forget.

// A control sequence is TeX's own definition: a run of letters after a
// backslash, or a backslash and one character that is not a letter. The corpus
// has both, \alpha being a word and \' a symbol, and controlName below reads
// them rather than a regular expression, because reading the arguments after
// the name needs a brace counter anyway and one scanner is easier to trust than
// a scanner and an expression that have to agree.
const (
	ctlOpen  = "\x00c"
	ctlClose = "\x00"
)

var ctlRE = regexp.MustCompile("\x00c(\\d+)\x00")

// controls takes the control sequences out of a line of prose and returns the
// line with placeholders in their place, plus the LaTeX each placeholder stands
// for. It runs before the prose is escaped, because after that there are no
// control sequences left to find, only backslashes that have been printed.
func (r Renderer) controls(s string) (string, []string) {
	if !strings.ContainsRune(s, '\\') {
		return s, nil
	}
	var out []string
	var b strings.Builder
	var stray []string
	rs := []rune(s)
	for i := 0; i < len(rs); {
		if rs[i] != '\\' {
			b.WriteRune(rs[i])
			i++
			continue
		}
		name, next := controlName(rs, i)
		if name == "" {
			// A backslash with nothing readable after it. It is prose as far as
			// anything here can tell, so it goes through the escaper and gets
			// printed, and the audit hears about it.
			stray = append(stray, `\`)
			b.WriteRune('\\')
			i++
			continue
		}
		// A Markdown escape, \* or \_, is not a control sequence. mdEscapeRE
		// handles those and it has to see them, so they are left alone here.
		if len(name) == 1 && strings.ContainsAny(name, `*_$#[]()~^\&%{}`) {
			b.WriteString(`\` + name)
			i = next
			continue
		}
		cmd, ok := control[name]
		if !ok {
			stray = append(stray, `\`+name)
			b.WriteString(`\` + name)
			i = next
			continue
		}
		args, end, complete := arguments(rs, next, cmd.args, cmd.bare)
		if !complete {
			// The command is known but its arguments are not all there, which
			// is a formula that was cut in half rather than one that was set as
			// text. Nothing here can put back what is not in the file.
			stray = append(stray, `\`+name)
			b.WriteString(`\` + name)
			i = next
			continue
		}
		out = append(out, r.emit(cmd, name, args))
		b.WriteString(ctlOpen + itoa(len(out)-1) + ctlClose)
		i = end
	}
	if len(stray) > 0 && r.Stray != nil {
		r.Stray(r.at(0), stray)
	}
	return b.String(), out
}

// emit is the LaTeX one control sequence and its arguments come out as.
func (r Renderer) emit(c cmd, name string, args []string) string {
	if c.raw != "" {
		return c.raw
	}
	if c.math {
		// The argument goes through Math for the same reason the text-mode
		// argument below goes through inline: what is being written here is a
		// formula, so what is inside it has to be read as one.
		//
		// Without this the wrapping happened and the reading did not, and the
		// characters inside a rescued formula went out as themselves. The
		// corpus writes \overline{F ∩ E_n} loose in an exercise of Espaces
		// vectoriels topologiques IV and \sqrt{12dδ} loose in chapter V, and
		// both came out of the build with the intersection sign and the delta
		// still spelled as characters, inside dollars, where no math font has
		// them. Two of the six volumes that were failing "every character
		// reached the page" were failing on nothing but that.
		var b strings.Builder
		b.WriteString(`$\` + name)
		for _, a := range args {
			b.WriteString("{" + Math(a) + "}")
		}
		b.WriteString(`$`)
		return b.String()
	}
	if len(args) == 0 {
		// The braces keep TeX from eating the space that follows. \S 2 without
		// them sets "§2", and the corpus writes \S 2 two thousand times meaning
		// "§ 2".
		return `\` + name + `{}`
	}
	// A text-mode argument is prose and is rendered as prose, so that a
	// \footnote{...} with an ampersand or a formula in it comes out right rather
	// than coming out raw.
	var b strings.Builder
	b.WriteString(`\` + name)
	for _, a := range args {
		b.WriteString("{" + r.inline(a) + "}")
	}
	return b.String()
}

// controlName reads the name of the control sequence starting at rs[i], which is
// a backslash, and says where it ends.
func controlName(rs []rune, i int) (string, int) {
	j := i + 1
	if j >= len(rs) {
		return "", i + 1
	}
	if !isLetter(rs[j]) {
		return string(rs[j]), j + 1
	}
	for j < len(rs) && isLetter(rs[j]) {
		j++
	}
	return string(rs[i+1 : j]), j
}

func isLetter(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }

// arguments reads n braced arguments after a control sequence, skipping the
// spaces TeX skips. complete is false when fewer than n are there, which is how
// a \frac with one argument is told from a \frac with two, and how the one
// \footnote in Algebra III that closes with a round bracket instead of a brace
// is told from the forty eight that close properly.
//
// bare allows the old accent spelling, one character with no braces round it.
// It is not allowed in general because \alpha x would then swallow the x.
func arguments(rs []rune, i, n int, bare bool) (args []string, end int, complete bool) {
	for len(args) < n {
		j := i
		for j < len(rs) && (rs[j] == ' ' || rs[j] == '\t') {
			j++
		}
		if j >= len(rs) {
			return args, i, false
		}
		if rs[j] != '{' {
			if !bare || rs[j] == '\\' {
				return args, i, false
			}
			args = append(args, string(rs[j]))
			i = j + 1
			continue
		}
		depth, k := 1, j+1
		for k < len(rs) && depth > 0 {
			switch rs[k] {
			case '\\':
				k++ // the escape takes the next character with it
			case '{':
				depth++
			case '}':
				depth--
			}
			k++
		}
		if depth != 0 {
			return args, i, false
		}
		args = append(args, string(rs[j+1:k-1]))
		i = k
	}
	return args, i, true
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var d [20]byte
	i := len(d)
	for n > 0 {
		i--
		d[i] = byte('0' + n%10)
		n /= 10
	}
	return string(d[i:])
}

type cmd struct {
	args int
	math bool
	// raw is what to write instead of the command itself, for the few whose
	// spelling in the corpus is not the spelling LaTeX wants.
	raw string
	// bare says the argument may be a single character with no braces round it,
	// which is how an accent is written and how the corpus writes the one in
	// M\'eray.
	bare bool
}

// control is the table. Every name in it was counted in this corpus before it
// was written down, which is why it holds \varpi and not \varrho: one of them
// is in the prose twice and the other is not in it at all. A table of everything
// LaTeX defines would be a table nobody had checked.
var control = func() map[string]cmd {
	m := map[string]cmd{}
	add := func(names []string, args int, math bool) {
		for _, n := range names {
			m[n] = cmd{args: args, math: math}
		}
	}
	add(mathWords, 0, true)
	add(mathArg1, 1, true)
	add(mathArg2, 2, true)
	add(textWords, 0, false)
	for _, n := range accents {
		m[n] = cmd{args: 1, bare: true}
	}
	add(textArg1, 1, false)
	maps.Copy(m, spellings)
	return m
}()

// spellings is the handful the corpus writes in a way LaTeX does not read, each
// counted and each looked at.
//
//	\  is TeX's control space and the corpus has 98, all of them in a formula
//	   that was set as text, where an ordinary space is what the printing has.
//	\, is a thin space and is legal in text mode as it stands.
//	\§ is somebody's \S with the S already turned into the sign it stands for,
//	   6 of them, all in one chapter of Integration, all reading "(\§1, No. 2".
var spellings = map[string]cmd{
	" ": {raw: " "},
	",": {raw: `\,`},
	"§": {raw: `\S{}`},
}

// mathWords is the mathematics that stands on its own: a relation, an arrow, a
// letter, or one of the operators that sets its own name upright. Each gets a
// pair of dollars and nothing else.
var mathWords = []string{
	// Greek, both cases.
	"alpha", "beta", "gamma", "delta", "epsilon", "varepsilon", "zeta", "eta",
	"theta", "iota", "kappa", "lambda", "mu", "nu", "xi", "pi", "varpi", "rho",
	"sigma", "tau", "upsilon", "phi", "varphi", "chi", "psi", "omega",
	"Gamma", "Delta", "Theta", "Lambda", "Xi", "Pi", "Sigma", "Upsilon", "Phi",
	"Psi", "Omega",
	// Set theory and the relations.
	"in", "notin", "subset", "supset", "supseteq", "cap", "cup", "bigcap",
	"bigcup", "setminus", "emptyset", "varnothing", "mid", "leq", "geq",
	"leqslant", "geqslant", "neq", "equiv", "sim", "approx", "asymp", "ll",
	"gg", "prec", "preceq", "preccurlyeq", "succeq", "perp",
	// The operations.
	"times", "otimes", "oplus", "bigotimes", "bigoplus", "circ", "cdot", "pm",
	"wedge", "vee", "sum", "prod", "int", "partial", "infty", "backslash",
	// The arrows.
	"to", "rightarrow", "leftarrow", "longrightarrow", "Rightarrow",
	"Leftrightarrow", "Longrightarrow", "implies", "mapsto", "uparrow",
	// The named operators, which set their own name upright and are why a
	// sentence saying "the lim of" has a control sequence in it at all.
	"lim", "varprojlim", "varinjlim", "liminf", "exp", "log", "sin", "cos",
	"tan", "sec", "cot", "csc", "dim", "det", "deg", "ker", "inf", "sup",
	"bmod",
	// The rest: dots, brackets, and the two marks.
	"ldots", "cdots", "langle", "rangle", "ell", "dagger", "bullet", "natural",
	// \| is the double bar of a norm and there are 276 of them, which makes it
	// the commonest piece of stranded mathematics in the corpus after \in. It is
	// a control symbol rather than a control word, so it sits at the end here
	// rather than among the letters.
	"|",
}

// mathArg1 is the mathematics that takes one braced argument, which is nearly
// always an alphabet command around a single letter: \mathbf{Z}, \mathfrak{g}.
var mathArg1 = []string{
	"text", "mathbf", "mathcal", "mathfrak", "mathrm", "mathscr", "mathbb",
	"overline", "bar", "hat", "tilde", "vec", "sqrt", "pmod", "operatorname",
	"boxed", "substack", "xrightarrow",
}

// mathArg2 is the two that take a numerator and a denominator.
var mathArg2 = []string{"frac", "binom"}

// textWords is text-mode LaTeX that is correct where it stands. \S is by far
// the commonest thing in this whole file and the only one of them that is not a
// defect: the corpus writes "\S 2" where the book prints "§ 2".
//
// \bfnmark is not in the corpus at all. It is put there by this package, in
// heading, where a \footnote in a numbered subsection title is split into a mark
// for the head and a \footnotetext after it. That mark then goes through inline
// like everything else, and with the name missing from this table the build was
// reporting its own output as a control sequence loose in the prose. One
// finding, on no. 10 of Integration VI, § 2, which is the one title in the
// corpus that carries a footnote. The class declares \bfnmark rather than the
// build writing \footnotemark, because the title is uppercased and \footnotemark
// does not survive that.
var textWords = []string{"S", "P", "quad", "hfill", "bfnmark"}

// textArg1 is text-mode LaTeX with an argument, which is rendered as prose
// rather than passed through, so that what is inside a \footnote gets the same
// escaping and the same character table as what is outside it.
var textArg1 = []string{
	"footnote", "footnotetext", "emph", "textit", "textbf", "textsuperscript",
	"hspace",
}

// accents are the ones the corpus spells the old way, with the letter after the
// command and no braces round it: M\'eray, and the hacek and the cedilla on two
// names in the historical notes. They are correct LaTeX and always have been,
// which is why they take an unbraced argument as well as a braced one.
var accents = []string{"'", "v", "c"}
