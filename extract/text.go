package extract

import (
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"golang.org/x/text/unicode/norm"

	"github.com/tamnd/bourbaki-solver/corpus"
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
	// The box the run it came from was drawn in. Across the page it says
	// where one token ends and the next begins; up and down it says which of
	// them an accent was drawn over.
	left   int
	right  int
	top    int
	bottom int
	math   bool
	// sign is set on a large operator, which is the one kind of token a page
	// can draw a limit across rather than beside.
	sign bool
}

// Render writes one line as Markdown with LaTeX mathematics.
func Render(l Line) string {
	toks := tokens(l)
	toks = hoist(toks, l.Top, l.Bottom)
	toks = barred(toks, l.Rules)
	toks = overset(toks)
	toks = restack(toks)
	toks = marked(toks)
	toks = extend(toks)
	return statementHead(emit(toks), capsOpen(toks))
}

// headWord is the word a statement opens with, for the statements Bourbaki sets
// in the italic of the statement itself rather than in small capitals. The ones
// set in small capitals are told by their font instead, which is capsOpen, and
// that is the only route the French printings have: they open on Corollaire,
// Théorème, Définition, Remarque and Exemple, and listing the words of every
// language the corpus carries is a losing game when the font has already said
// it.
var headRE = regexp.MustCompile(`^(Definition|Proposition|Theorem|Lemma|Corollary|Corollaries|Remarks?|Examples?|Scholium|Criterion)( [0-9]+)?\.( —| --)`)

// capsTailRE is what the volume prints between the name of a statement and the
// statement: its number, a full stop, and a dash.
var capsTailRE = regexp.MustCompile(`^( [0-9]+)?\.(\s*(—|--))`)

// capsOpen is the words a line opens with in the type Bourbaki sets the name of
// a statement in, which is small capitals for a Proposition and italic for the
// kinds the French volumes set plainly.
//
// The italic is only taken as a name when what follows it is roman and is the
// number and dash of a head, because a statement is set in italic all through
// and every line of one opens in italic too. The French chapter VIII sets
// "Lemme " in italic, then "2. — " in roman, then the statement back in italic,
// and the change of font at the dash is the whole of the evidence: no line of
// prose in either volume opens in italic and returns to roman on a dash.
func capsOpen(toks []token) string {
	if s := openIn(toks, ClassHead); s != "" {
		return s
	}
	s := openIn(toks, ClassEmph)
	if s == "" {
		return ""
	}
	i := 0
	for i < len(toks) && toks[i].class == ClassEmph {
		i++
	}
	var rest strings.Builder
	for _, t := range toks[i:] {
		rest.WriteString(t.text)
	}
	if headTailRE.MatchString(rest.String()) {
		return s
	}
	return ""
}

// headTailRE is what the printing sets in roman between the italic name of a
// statement and the statement itself.
var headTailRE = regexp.MustCompile(`^\s*[0-9]*\s*\.\s*(—|--)`)

// openIn is the run of text at the head of a line set in one class.
func openIn(toks []token, c Class) string {
	var b strings.Builder
	for _, t := range toks {
		if t.class != c {
			break
		}
		b.WriteString(t.text)
	}
	return strings.TrimSpace(b.String())
}

// statementHead bolds the head of a statement, which is how the corpus marks
// where one begins. Bourbaki prints it in small capitals or in italic and
// follows it with a number and a dash, and the dash stays outside the bold, as
// it is punctuation of the page rather than part of the name.
func statementHead(s, caps string) string {
	if caps != "" && strings.HasPrefix(s, caps) {
		if m := capsTailRE.FindStringSubmatch(s[len(caps):]); m != nil {
			name := caps + m[1] + "."
			return "**" + name + "**" + s[len(name):]
		}
	}
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
	runs := unhat(composed(l.Runs))
	for i, r := range runs {
		if section(runs, i) {
			// The sign is a mark and not a formula. See section.
			toks = append(toks, token{text: "§", class: ClassText,
				left: r.Left, right: r.Right(), top: r.Top, bottom: r.Bottom()})
			continue
		}
		if _, ok := Accent(r.Spec, r.Text); ok {
			accents = append(accents, r)
			continue
		}
		if Extension(r.Spec) {
			text, acc := cmexText(r)
			if acc {
				accents = append(accents, r)
				continue
			}
			if text == "" {
				continue
			}
			toks = append(toks, token{text: text, class: ClassMath, level: r.Level, sign: cmexOps[text],
				left: r.Left, right: r.Right(), top: r.Top, bottom: r.Bottom(), math: true})
			continue
		}
		if bend(r) {
			// The sign is a mark and not a formula. It went out as $\dbend$,
			// a command out of a LaTeX package no renderer here loads, and
			// KaTeX refused all seventeen of them, which is what kept the
			// site from being published. Unicode has the sign, so the corpus
			// writes the sign.
			toks = append(toks, token{text: corpus.Bend, class: ClassText,
				left: r.Left, right: r.Right(), top: r.Top, bottom: r.Bottom()})
			continue
		}
		text := runText(r)
		if text == "" {
			continue
		}
		if r.Level == Sup && footnoteMark(r.Text) {
			// A footnote reference is a superscript and is not an exponent.
			toks = append(toks, token{text: "[^" + strings.Trim(r.Text, "()") + "]",
				class: ClassText, left: r.Left, right: r.Right(), top: r.Top, bottom: r.Bottom()})
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
			left: r.Left, right: r.Right(), top: r.Top, bottom: r.Bottom(), math: r.Class.Math() || r.Level != Base})
	}
	return place(toks, accents)
}

// composed folds an accent into the letter it was drawn over, for the accents
// that are never mathematics. The acute, the grave and the cedilla are drawn
// out of the text faces and every one of the 76 in the six volumes is a French
// word in a bibliography, a name in a historical note, or the copyright page,
// which reads "Toute repr´esentation, reproduction int´egrale ou partielle
// faite par quelque proc´ed´e que ce soit" in the corpus today and reads as
// French after this. Writing them as accents over a letter would put a French
// sentence inside dollar signs, so they compose or they stay as they are, and
// there is no test of a word here because there is nothing they could be
// mistaken for.
//
// Which run the two arrive in is not part of it. The layer breaks a run at an
// accent's kern as readily as anywhere else, and Lie 7 to 9 hands back "Sur les
// repr´" with "esentations alg´" after it and "´" welded to the front of
// "etale" twenty-five times.
//
// The accents that are also mathematics do not come through here at all. unhat
// cuts each of those out into an accent of its own and place puts it over the
// letter it was drawn over, which is the only thing that tells the hat of
// "u ∈ ˆG is summable", drawn on top of the G before it, from the diaeresis of
// "Jordan-H¨older", drawn on top of the o after it. A draft that read the
// letters either side instead wrote "îs".
func composed(runs []Run) []Run {
	var text []rune
	var owner []int
	for i, r := range runs {
		for _, c := range r.Text {
			text = append(text, c)
			owner = append(owner, i)
		}
	}
	type glyph struct {
		r   rune
		own int
	}
	out := make([]glyph, 0, len(text))
	widen := map[int]int{}
	taken := false
	for i := 0; i < len(text); i++ {
		mark, ok := marks[text[i]]
		// A large operator out of the extension font is not an accent
		// whatever code it arrives under. Poppler hands the coprod of "X =
		// \\coprod_{i\\in I}X_i" on page 18 of Topologie algebrique back as a
		// grave, and the i of the subscript is a letter right after it, so
		// without this the sum of a family of B-spaces reads "X = $_{ì\\in
		// I}X_i$".
		if Extension(runs[owner[i]].Spec) {
			ok = false
		}
		if _, drawn := spacing[text[i]]; !ok || drawn {
			out = append(out, glyph{text[i], owner[i]})
			continue
		}
		if i+1 < len(text) && unicode.IsLetter(text[i+1]) {
			if c, ok := compose(text[i+1], mark); ok {
				// The letter goes into the run the word is being written in
				// rather than into its own. TeX sets the e of an italic
				// "Algèbre" out of the mathematics italic, because that is
				// where an accented letter of an italic word comes from, so
				// the run it arrives in is a mathematics run of one character
				// and the page said "Alg$è$bre".
				own := owner[i+1]
				if len(out) > 0 && unicode.IsLetter(out[len(out)-1].r) {
					own = out[len(out)-1].own
					// The word takes the ground the accent and the letter
					// were drawn on as well as the letter itself. Without it
					// the emitter reads the seven units the run of the lone e
					// stood in as a gap between words and the page says
					// "Algè bre".
					widen[own] = max(widen[own], through(runs, owner[i+1]))
				}
				out = append(out, glyph{c, own})
				i++
				taken = true
				continue
			}
		}
		// The accent drawn after the letter it belongs over rather than
		// before it, which is how the layer hands back the cedilla of
		// "contrefac¸on" and, on two lines of Lie 7 to 9 out of the four that
		// name the same chapter of Algebra, the grave of "Alge`bre".
		if len(out) > 0 && unicode.IsLetter(out[len(out)-1].r) {
			if c, ok := compose(out[len(out)-1].r, mark); ok {
				out[len(out)-1].r = c
				// The same move the branch above makes, for the same
				// reason. Two of the four lines that cite Chapter X of
				// Algebra draw the grave after the e, and the e is a
				// mathematics run of one character sitting between two
				// italic ones, so the word takes it and the ground it
				// stood in or the page says "Alg$è$bre".
				if last := &out[len(out)-1]; len(out) > 1 &&
					utf8.RuneCountInString(runs[last.own].Text) == 1 &&
					unicode.IsLetter(out[len(out)-2].r) &&
					out[len(out)-2].own != last.own {
					widen[out[len(out)-2].own] = max(widen[out[len(out)-2].own], runs[last.own].Right())
					last.own = out[len(out)-2].own
				}
				taken = true
				continue
			}
		}
		out = append(out, glyph{text[i], owner[i]})
	}
	if !taken {
		return runs
	}
	folded := make([][]rune, len(runs))
	for _, g := range out {
		folded[g.own] = append(folded[g.own], g.r)
	}
	rewritten := make([]Run, len(runs))
	copy(rewritten, runs)
	for i := range rewritten {
		rewritten[i].Text = string(folded[i])
		if right, ok := widen[i]; ok && right > rewritten[i].Right() {
			rewritten[i].Width = right - rewritten[i].Left
		}
	}
	return rewritten
}

// through is the right edge of the cell the first character of a run was drawn
// in, which is where a word that has taken that character now reaches to.
func through(runs []Run, i int) int {
	n := utf8.RuneCountInString(runs[i].Text)
	if n <= 1 {
		return runs[i].Right()
	}
	return runs[i].Left + runs[i].Width/n
}

// compose writes a letter and a combining mark as the one letter Unicode has
// for the two, and says so when Unicode has none. What the two make is
// Unicode's business rather than this file's, so the pair goes through NFC and
// is taken only where NFC gives back one character. That is also what tells the
// two orders apart: the grave of "Alge`bre" is drawn after its letter and a b
// with a grave over it is not a letter anybody has ever printed, so the pair
// the mark is written in front of composes to nothing and the pair behind it
// composes to an e with a grave.
func compose(letter rune, mark rune) (rune, bool) {
	c := norm.NFC.String(string([]rune{letter, mark}))
	if utf8.RuneCountInString(c) != 1 {
		return 0, false
	}
	return []rune(c)[0], true
}

// unhat cuts a spacing accent out of the runs it arrived welded to, so that it
// can be placed over the letter it was drawn over like any other accent.
//
// The French printing sets the hat of the Fourier transform in the text face
// rather than in the extension font, and poppler hands it back inside the run
// beside it: "= ˆ" at 141 with the tau it covers at 156, or a run of one hat at
// 250 with the tau at 249. Neither is an accent to Accent, which reads the
// first character of a run, so both came through as themselves and the page
// said "$\tau$ˆ$(...)$" where the book prints the transform of tau. Six of them
// in § 21 of the French chapter, and they are the whole of what M03 has left to
// say about that volume.
//
// The box of the piece is worked out by counting characters across the run,
// which is the same estimate accent makes when it cuts a token, and it is good
// enough here for the same reason: what it has to decide is which letter of the
// line the hat sits over, and the letters are wider than the error.
func unhat(runs []Run) []Run {
	var out []Run
	for _, r := range runs {
		rs := []rune(r.Text)
		if len(rs) < 2 || !strings.ContainsFunc(r.Text, isSpacing) {
			out = append(out, r)
			continue
		}
		w := r.Width
		cut := func(from, to int) Run {
			p := r
			p.Text = string(rs[from:to])
			p.Left = r.Left + w*from/len(rs)
			p.Width = r.Left + w*to/len(rs) - p.Left
			return p
		}
		last := 0
		for i, c := range rs {
			if !isSpacing(c) {
				continue
			}
			if i > last {
				out = append(out, cut(last, i))
			}
			out = append(out, cut(i, i+1))
			last = i + 1
		}
		if last < len(rs) {
			out = append(out, cut(last, len(rs)))
		}
	}
	return out
}

// isSpacing reports whether a character is one of the accents a text face draws
// at a width of its own rather than as a mark that combines with the letter
// before it.
func isSpacing(r rune) bool {
	_, ok := spacing[r]
	return ok
}

// footnoteMark reports whether a run is a reference to a footnote, which this
// volume sets as a parenthesised number raised above the line. Everything else
// raised above the line is an exponent.
//
// Above the line and not merely off it. A parenthesised number dropped below
// the line is an index, and reading one as a footnote reference cost three of
// them in Algebra VIII: X_{\sigma(1)} on page 446, v_{\sigma(1)} on 465 and
// \varepsilon_{\sigma(1)} on 471 came out as X_{\sigma}[^1] and so on, and each
// of the three then pointed at a real footnote of the § it stood in.
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
//
// Overlapping across the page is not enough on its own. A line gathers what is
// drawn beside it as well as what is on it, and a display carries its accents
// far enough above the letters that they can be gathered onto the wrong line;
// when that happened on page 114 every tilde of the display found a letter of
// the sentence below it at the same place across the measure and went over
// that. So the accent's band has to meet the token's, which it does on the line
// it belongs to and does not on the line below.
//
// Meeting the band and not starting inside it. The two printings draw the same
// accent differently: the English sets the glyph inside the band of the letters
// it covers, and the French sets it above them, so that the hat of the omitted
// factor of p(v1), . . . , widehat{p(vi)}, . . . , p(vn) on page 441 opens at
// 333 where the letters run from 340 to 352. Reading the top alone lost every
// one of the 24 hats of that appendix.
//
// Across the page it is the token the accent covers most, not the first one it
// touches. The boxes of a formula are set edge to edge, so the tilde over the
// sigma of Θ ∘ (σ ⊗ 1_P) starts at exactly the right edge of the bracket before
// it, and taking the first token that overlaps puts the tilde over the bracket.
func place(toks []token, accents []Run) []token {
	for _, a := range accents {
		latex, ok := Accent(a.Spec, a.Text)
		if !ok {
			continue
		}
		at, best := -1, 0
		for i, t := range toks {
			over := min(t.right, a.Right()) - max(t.left, a.Left)
			if over < 0 || a.Bottom() <= t.top || a.Top >= t.bottom {
				continue
			}
			if at < 0 || over > best {
				at, best = i, over
			}
		}
		if at < 0 {
			continue
		}
		toks = append(toks[:at], append(accent(toks[at], a, latex), toks[at+1:]...)...)
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
		t.text = latex + "{" + strings.TrimSpace(runes(t.text, nil)) + "}"
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
	if word, ok := letter(rs, lo, hi, a); ok {
		// The accent was drawn over a letter of a word rather than over a
		// letter of mathematics, so it is spelling and not notation. Lie 7 to
		// 9 breaks the run at the kern and hands back "Jordan-H¨" with "older"
		// after it, "K¨" with "ahler", "f¨" with "ur", and "ZASSENHAUS, ¨"
		// with "Uber", and all ten of them went out with a diaeresis over a
		// letter in mathematics inside an English sentence.
		t.text = word
		// The accent stood between the letter before it and the letter it
		// covers, and now it stands nowhere, so the word takes the space it
		// was drawn in. Without this the emitter reads the three units the
		// diaeresis of "a K¨" occupied as a gap between words and page 421
		// says "a K ähler structure".
		t.left = min(t.left, a.Left)
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
	// The letter under the accent is being written as mathematics whatever it
	// was read as, so it goes through the table on the way in. Bourbaki sets
	// its capital Greek in the text face and page 114 puts a tilde over one of
	// them, which came out as \widetilde{Λ} with the letter still Unicode.
	mid.text = latex + "{" + strings.TrimSpace(runes(mid.text, nil)) + "}"
	mid.math, mid.class = true, ClassMath
	out = append(out, mid)
	if hi < len(rs) {
		out = append(out, cut(hi, len(rs)))
	}
	return out
}

// letter reports whether the accent was drawn over a letter of a word, and
// gives back the token's text with the two written as the one letter the
// printer set.
//
// A letter of a word has a letter beside it and a letter of mathematics does
// not. "Jordan-H¨older" breaks into "Jordan-H" and "older" and the o the hat
// covers is followed by an l; "from ˜V to C" breaks the same way and the V the
// tilde covers is followed by a space. That is the whole of the difference, and
// nothing in the geometry or in the run boundaries carries it: the two are laid
// out alike, down to the accent overlapping the letter after it by the same few
// units in both.
//
// The accent has to be one the text faces draw, since a wide hat out of the AMS
// symbol font is notation wherever it is drawn, and the two have to compose to
// a letter Unicode has. The caution over the S of Šmulian is drawn as a breve
// rather than as a caron and so composes to nothing, which is right: the corpus
// cannot spell what the volume prints.
//
// What this does not reach is a word the layer cuts into single letters. The
// bibliography of Lie 7 to 9 names a Russian journal and the layer hands it
// back as "Obˇ", "sˇ" and "c", so each caron covers a token of one character
// with no letter beside it in the token and there is nothing here to read. It
// stays as the corpus has always carried it.
func letter(rs []rune, lo, hi int, a Run) (string, bool) {
	if hi != lo+1 || !unicode.IsLetter(rs[lo]) {
		return "", false
	}
	before := lo > 0 && unicode.IsLetter(rs[lo-1])
	after := hi < len(rs) && unicode.IsLetter(rs[hi])
	if !before && !after {
		return "", false
	}
	mark, ok := marks[[]rune(a.Text)[0]]
	if !ok {
		return "", false
	}
	c, ok := compose(rs[lo], mark)
	if !ok {
		return "", false
	}
	return string(rs[:lo]) + string(c) + string(rs[hi:]), true
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
		// A character the volume names properly is read as itself. Preparing
		// a volume turns the names CMEX gives its glyphs into the characters
		// they stand for where there is one, so a run of this font can carry
		// a real ℓ or ℜ, and the code page of the font says nothing about
		// those.
		if latex, ok := unicodeMath[c]; ok {
			b.WriteString(spaced(latex))
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
	case "rsfs", "EUSM":
		return wrapLetters(s, `\mathscr`)
	case "EUFM":
		return wrapLetters(s, `\mathfrak`)
	case "CMSSDC", "LMSans":
		return wrapLetters(s, `\mathsf`)
	case "LMMathSymbols", "CMSY", "CMBSY":
		return symbols(s)
	case "LMMathItalic", "CMMI", "CMMIB":
		if r.Bold {
			return `\boldsymbol{` + runes(s, cmmi) + `}`
		}
		return runes(s, cmmi)
	case "MSAM":
		return runes(s, msam)
	}
	if r.Class == ClassStrong {
		return bold(s)
	}
	if r.Class == ClassBold {
		return wrapLetters(s, `\mathbf`)
	}
	if !r.Class.Math() && r.Level == Base {
		return prose.Replace(s)
	}
	if r.Class == ClassMath && r.Bold {
		// Bourbaki sets a few of its Greek letters bold, and a bold letter of
		// the mathematics italic is a different letter from the upright one.
		return `\boldsymbol{` + runes(s, nil) + `}`
	}
	return runes(s, nil)
}

// bold marks a run as bold Markdown, keeping the space it was set with outside
// the asterisks. Markdown reads a pair of asterisks with a space after it as
// two literal asterisks, and a run of a heading often ends in one.
func bold(s string) string {
	t := strings.TrimSpace(s)
	if t == "" {
		return s
	}
	i := strings.Index(s, t)
	return s[:i] + "**" + t + "**" + s[i+len(t):]
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
		// A statement head and a heading are prose whatever they sit beside.
		// Page 438 opens its subsection on the star that marks it optional, and
		// reading the number after the star as part of a formula turned "∗13.
		// Complex Linear Representations" into $***13$ and left the asterisks
		// of the heading open.
		if t.math || t.class == ClassHead || t.class == ClassStrong {
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
			// The piece was read as prose and is being written as mathematics,
			// so the characters in it have to be written as mathematics too.
			// Bourbaki sets its capital Greek upright, in the text face, so the
			// sum of page 95 of the French chapter arrived as the letter Σ in a
			// run of prose and went inside the dollar signs as itself, where it
			// prints nothing and says nothing. The same letter in an index goes
			// through this table already, which is why the line read
			// "Σ_{\sigma\in\Gamma}" with one of its two letters in TeX.
			n.text = runes(n.text, nil)
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

// spaceGap is how far two runs have to stand apart before the white between
// them is a space of the sentence rather than the fit of the letters.
//
// A word space in this volume is five units and the letters of a word touch, so
// three was enough until the operators. Bourbaki sets a thin space before the
// argument of one, and a thin space is two units: page 145 ends "Ker" at 376 and
// opens "u" at 378, and read at three that page said "Keru". The same goes for
// det u, Tr u, Im u, Ann x and Nrd.
//
// The narrower measure is only taken when a letter follows, and only when it is
// written on the line. Two units apart is where the volume also sets a closing
// bracket against the word it closes and the leaders of the table of contents
// against their entry, and those want no space at all: 472 of them against 81
// that open on a letter. An index is written against the thing it indexes for
// the same reason, which is why page 348 wants Inf^q and not Inf ^q.
func spaceGap(t token) int {
	r, _ := utf8.DecodeRuneInString(strings.TrimLeft(t.text, " "))
	if unicode.IsLetter(r) && t.level == Base && t.depth == 0 {
		return 2
	}
	return 3
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
	// hyphenated says the formula just closed ended in the hyphen of a compound
	// word, so the word that follows joins on to it without a space.
	hyphenated := false
	// opened is where the dollar of the formula being written stands, so that a
	// formula that turns out to hold nothing can take it back.
	opened := 0
	// primed says the base being written carries a prime, and baseAt is where
	// that base begins. A prime is a superscript to TeX, so a base that carries
	// one and then takes a superscript of its own is two superscripts against
	// one base and TeX refuses it by name. The French chapter on the Brauer
	// group writes \Gamma '_1^{\pi'_1} for the group of a subextension and
	// KaTeX set none of them. The base and whatever index it has go inside
	// braces, which is what the printed page shows and what anybody writing it
	// out would do.
	primed, baseAt := false, 0
	brace := func() {
		// baseAt has to be inside the formula being written. A prime is a mark
		// on the base it is drawn against and nothing outside that formula, so
		// a stale one would put a brace around the sentence in between.
		if !primed || baseAt >= len(out) || baseAt <= opened {
			return
		}
		out = out[:baseAt] + "{" + strings.TrimRight(out[baseAt:], " ") + "}"
		primed = false
	}
	closeMath := func(compound bool) {
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
		// A hyphen at the end of a formula is not a minus sign, it is the
		// hyphen of a compound word. Bourbaki sets the A_M of "A_M-module" in
		// the mathematics font and the word after it in roman, and the hyphen
		// comes back on the mathematics side of that boundary. Left there the
		// page reads $A_M-$ module, which is wrong twice over: the hyphen is
		// typeset as a minus sign, and the compound word is broken by a space
		// that is not on the page. Measured on Algebra VIII before this: 64 of
		// them across 50 of its 505 pages.
		//
		// The hyphen has to be written on the line. One that is the whole of an
		// index is a minus sign, part of what the formula says rather than the
		// spelling of the word after it, and taking it out leaves the underscore
		// that opened the index with nothing after it. Page 207 of Lie 7 to 9
		// prints "X_+ and X_-, introduce a basis" and shipped as "$X_$-,introduce",
		// which KaTeX will not set and which puts a hyphen in front of a word the
		// page does not hyphenate. Braces are dropped from an index of one
		// character, so an index that is nothing but the minus is the one way a
		// formula can end in an underscore and a hyphen.
		indexed := len(out) > 1 && (out[len(out)-2] == '_' || out[len(out)-2] == '^')
		if compound && !indexed && strings.HasSuffix(out, "-") {
			out = strings.TrimRight(out[:len(out)-1], " ")
			tail = "-" + tail
			hyphenated = true
		}
		// A formula with nothing left in it is not a formula, and the dollar
		// that opened it has to come back off. Bourbaki sets the comma of
		// "sous-(A, A)-bimodule" in the mathematics italic, which makes a run
		// of one comma, and the rule above then takes the comma outside and
		// leaves the two dollars against each other. That is not an empty
		// formula to anything that reads the Markdown afterwards, it is the
		// opening of a display: page 385 of the French chapter had the rest of
		// its proof inside one, and the audit reported the display that never
		// closes rather than the comma that opened it. Five pages of the two
		// printings do this, all of them a comma or a full stop.
		if strings.TrimSpace(out[opened+1:]) == "" {
			out = strings.TrimRight(out[:opened], " ") + tail
			inMath = false
			return
		}
		out = strings.TrimRight(out, " ") + "$" + tail
		inMath = false
	}
	for i := 0; i < len(toks); i++ {
		t := toks[i]
		text := t.text
		gap := prev.right >= 0 && t.left-prev.right >= spaceGap(t)
		if !t.math {
			closeMath(!gap && compoundWord(text))
			primed = false
			switch {
			case hyphenated:
				text = strings.TrimLeft(text, " ")
				hyphenated = false
			case gap:
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
			opened = len(out)
			out += "$"
			inMath = true
		}
		// A prime never opens a group of its own. It is a mark on what is
		// already written rather than a script standing by itself, so where a
		// group is open at its depth it joins that one, and where there is
		// none it belongs on the line beside the letter it marks.
		//
		// Which of the two it is was decided by measurement, and the
		// measurement is against the box of the line. Page 85 of Lie 7 to 9
		// sets the matrix of an element of SL(2, k) in the middle of a running
		// sentence; the minus sign of its top row is drawn nineteen units deep
		// against the thirteen of the type beside it, so the line takes the
		// deeper box, and every prime on that line then sits above the letter
		// it marks and below the middle of the line. "operates by +1 on E′"
		// shipped as "operates by +1 on $E_'$", which KaTeX will not set. The
		// twenty-two of those in the volume are all one shape: a prime that
		// opened an index with nothing else in it.
		if t.depth > len(open) && strings.Trim(t.text, "'") == "" {
			t.depth, t.level = len(open), Base
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
				if len(open) == 0 && t.level == Sup {
					brace()
				}
				out = strings.TrimRight(out, " ") + mark(t.level) + "{"
				open, at = append(open, t.level), append(at, len(out)-1)
			}
		} else if t.depth > 0 && open[len(open)-1] != t.level {
			closeGroups(t.depth - 1)
			if len(open) == 0 && t.level == Sup {
				brace()
			}
			out += mark(t.level) + "{"
			open, at = append(open, t.level), append(at, len(out)-1)
		}
		if t.depth > 0 {
			text = strings.TrimSpace(text)
		}
		if openMacro(out) && startsWord(text) {
			out += " "
		}
		if t.depth == 0 {
			if strings.Trim(strings.TrimSpace(text), "'") == "" {
				primed = true
			} else {
				primed, baseAt = false, len(out)
			}
		}
		out += text
		prev = t
	}
	// Nothing is known about what follows the end of a line, so a hyphen there
	// stays where it is and join decides, where the next line is in hand.
	closeMath(false)
	return strings.TrimRight(out, " ")
}

// compoundWord reports whether the run after a hyphen at the end of a formula
// is the second half of a compound word rather than the operand of a
// subtraction. It is what tells the hyphen of A_M-module from the minus of
// R = P-XQ, and both shapes are common in this volume.
//
// The word has to be lower case, and it has to end where a word ends. Bourbaki
// writes A_M-module, A_M-linear, (A,B)_k-bimodule, k-algebra, and every one of
// them is a whole word of prose with a space or a full stop after it. The
// operand of a subtraction is either capital, as in P-XQ, or it is the name of
// a function with its argument after it, as in cl(E)-cl(E'), and the bracket
// is what gives that one away: no word of the book has a bracket welded to it.
// A second hyphen is a word too, since the book writes (D,A)-sub-bimodule, and
// so is a closing bracket, since the word can be the last thing in an aside.
func compoundWord(s string) bool {
	s = strings.TrimLeft(s, " ")
	n := 0
	for _, r := range s {
		if !unicode.IsLower(r) {
			break
		}
		n += utf8.RuneLen(r)
	}
	if n == 0 {
		return false
	}
	rest := s[n:]
	if rest == "" {
		return true
	}
	r, _ := utf8.DecodeRuneInString(rest)
	return r == ' ' || strings.ContainsRune(",.;:-)", r)
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
