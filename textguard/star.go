package textguard

import (
	"regexp"
	"strings"

	"github.com/tamnd/bourbaki-solver/mathtex"
)

// Star is how the corpus writes the mark Bourbaki sets at each end of a passage
// that leans on results proved in a later Book. The Elements print an asterisk,
// and the corpus writes it escaped, because a bare one at the head of a line
// opens a list and a bare pair in a sentence opens emphasis. 82 of them are
// written that way in pages/ and they are the form everything else follows.
const Star = `\*`

// ornament is the glyph a model hands back in the star's place, and there are
// four of them in this corpus.
//
// None of the OCR prompts said how to write the star, so the model chose, and it
// chose by what the glyph looked like rather than what it meant. Three of the
// four are dingbats with no mathematical reading at all and the fourth is a
// binary operation. Theory of Sets alone has 24 of them against 82 written
// properly on the same volume's pages, and § 1 of chapter IV has both forms in
// one paragraph.
//
// The fault is quiet in the way that matters. A star is a piece of Bourbaki's
// own apparatus: it says this passage uses a Book you have not read yet, and the
// reader is meant to be able to find them. Four spellings means a search for
// them finds a quarter, and nothing on the page looks wrong, because an ornament
// at the end of a sentence reads as an ornament at the end of a sentence. It
// went through translation untouched, and content/vi carries the same four
// glyphs in the same places, which is what a fault does when nothing catches it.
var ornament = map[rune]string{
	'∗': "an asterisk operator",
	'✻': "a teardrop spoked asterisk",
	'✳': "an eight spoked asterisk",
	'⁎': "a low asterisk",
}

// An Ornament is one glyph standing where the corpus's star belongs.
type Ornament struct {
	Line int    // the body line it sits on, counting from one
	Name string // what the glyph is
	Text string // the line it was found on
}

// bareStar is the plainest way of getting the star wrong, and the only one that
// changes what the reader sees: a bare ASCII asterisk where the corpus writes
// the escaped one.
//
// The test is the space around it, and it comes from the Elements themselves. To
// the Reader says the passages are "always placed between two asterisks: * . . .
// *", and that is how they are set, with the space on the inside. Markdown reads
// the same shape the other way round: an emphasis run has to open on a non-space
// and close on a non-space, so *signs* is emphasis and * signs * is not. An
// asterisk with a space or a line end on both sides is therefore neither one end
// of an emphasis run nor half of a bold pair nor an escaped star already, and in
// Bourbaki it is the mark and nothing else.
//
// Left alone it renders wrong, which is what separates this from the ornaments.
// "* (3) The set R of real numbers is totally ordered. *" opens a bullet list on
// the star and the reader gets an indented item with no list around it. Nothing
// in the corpus writes a bullet list: the Elements number their lists and the
// only Markdown bullets anywhere are under content/solutions, which this never
// runs on.
func bareStar(rs []rune, i int, math []bool) bool {
	if rs[i] != '*' || math[i] {
		return false
	}
	space := func(j int) bool {
		if j < 0 || j >= len(rs) {
			return true // a line end counts, and so does the start of the body
		}
		return rs[j] == ' ' || rs[j] == '\n' || rs[j] == '\t'
	}
	// A backslash, a second asterisk and a letter all fail this, so an escaped
	// star, a bold run and an emphasis run are all left alone without being
	// named separately.
	return space(i-1) && space(i+1)
}

// Stars puts the corpus's star back wherever a model wrote something else for
// it, and returns the text and how many it put back.
//
// It works outside the math spans only, and that is not tidiness. U+2217 is the
// asterisk operator and inside the mathematics it is a binary operation, a dual
// or a pullback, so the same glyph means one thing on one side of a dollar and
// another on the other. The ASCII asterisk is worse: inside a span it is a
// convolution, an adjoint, a dual basis or the units of a ring, and K^* runs
// through the volumes in their thousands. Outside a span neither can be anything
// but the mark, since prose has no operators in it. Inside one they belong to
// bourbaki fix math, which turns them into the TeX that prints them. The three
// dingbats have no reading anywhere and are left inside a span all the same, so
// that this function and the rule that reads it cannot disagree about where the
// mark can be.
func Stars(text string) (string, int) {
	// The spans first, since putting a mark back into the prose changes where
	// the spans are and the rune walk below is indexed by the body it read.
	text, scripts := ScriptStars(text)
	math := inMath(text)
	rs := []rune(text)
	var b strings.Builder
	n := 0
	for i, r := range rs {
		_, bad := ornament[r]
		if (bad && !math[i]) || bareStar(rs, i, math) {
			b.WriteString(Star)
			n++
			continue
		}
		b.WriteRune(r)
	}
	return b.String(), n + scripts
}

// Ornaments is the same reading with nothing given back, for the audit. It hands
// over each line that carries one and says what it found, so a finding can name
// it rather than leave somebody to look up a code point.
func Ornaments(text string) []Ornament {
	math := inMath(text)
	rs := []rune(text)
	scripts := scriptStarLines(text)
	var out []Ornament
	at, line := 0, 1
	for _, l := range strings.Split(text, "\n") {
		found := false
		for i, r := range []rune(l) {
			name, bad := ornament[r]
			if !bad || math[at+i] {
				if !bareStar(rs, at+i, math) {
					continue
				}
				name = "a bare asterisk"
			}
			out = append(out, Ornament{Line: line, Name: name, Text: l})
			found = true
			break
		}
		if !found && scripts[line] {
			out = append(out, Ornament{Line: line,
				Name: "a star hung on nothing inside the mathematics", Text: l})
		}
		at += len([]rune(l)) + 1
		line++
	}
	return out
}

// scriptStarLines is the body lines ScriptStars would rewrite, by line number.
// The rule that reads Ornaments and the command that repairs the text have to
// agree about where the mark can be, and the cheapest way to be sure they do is
// for the report to ask the repair.
func scriptStarLines(text string) map[int]bool {
	out := map[int]bool{}
	if _, n := ScriptStars(text); n == 0 {
		return out
	}
	line := 1
	for _, l := range strings.Split(text, "\n") {
		if _, n := ScriptStars(l); n > 0 {
			out[line] = true
		}
		line++
	}
	return out
}

// inMath marks every rune of a body that sits inside a math span.
//
// A span that never closes takes the rest of the body with it. That is not what
// the page means, it is a file that M01 is already reporting, and until somebody
// reads the printed page there is no telling where the mathematics was supposed
// to stop. Marking the tail leaves it alone, which is the right thing to do with
// text whose reading is not known.
func inMath(text string) []bool {
	spans, open := mathtex.Split(text)
	mark := make([]bool, len([]rune(text)))
	set := func(from, to int) {
		if from < 0 {
			from = 0
		}
		if to > len(mark) {
			to = len(mark)
		}
		for i := from; i < to; i++ {
			mark[i] = true
		}
	}
	for _, s := range spans {
		set(s.Start, s.End)
	}
	if open != nil {
		set(open.Start, len(mark))
	}
	return mark
}

// scriptStar is a math span whose whole content is a star hung on nothing: a
// subscript or a superscript with no base, which is not something TeX has a
// reading for. KaTeX sets it as a lone star on the subscript's baseline, close
// enough to the printed mark that nothing looks wrong on the page.
//
// It is the same fault as the four ornaments and it hides in the one place
// Stars was told not to look. A model that read the mark as a script put it
// inside the dollars, and the span it made is the mark and nothing else:
//
//	$^*$(iv) G has a riemannian metric invariant under left and right translations.$_*$
//
// was one starred passage with both of its marks turned into scripts, and
// pages/ens-i-iv/0146.md had the opening mark escaped and correct with the
// closing one inside a span, on two adjacent lines.
//
// A trailing token is taken with it, because a model that swallowed the star
// swallowed what the star was set against: the sentence's full stop, the
// bracket that closed the parenthesis it ended, the number of the lemma being
// cited. Those come back out, and a letter comes back out into a span of its
// own, since "a subtorus of $G._*$" means the group G and not the letter G.
var scriptStar = []struct {
	content string // what the span holds, exactly
	prose   string // what goes back outside it, before the mark
}{
	{content: "_*"},
	{content: "._*", prose: "."},
	{content: ".)_*", prose: ".)"},
}

// openingScriptStar is the same fault at the other end of the passage, and it
// is the one that needs its surroundings read.
//
// A span holding ^* is the mark when it opens a line and is a dual or an
// adjoint when it does not: S$^*$ is the dual of S with its base stranded in
// the prose, which is bourbaki fix math's fault to repair and not this one's.
// 6 of the 24 in pages/ opened a line and 18 are glued to a letter or a
// bracket, so the two populations separate cleanly and neither rule need guess.
const openingScriptStar = "^*"

// footnoteBeforeStar is a footnote reference standing where a sentence ends,
// which is the one superscript that can come between the full stop and the
// mark. A number with no punctuation in front of it is an exponent.
var footnoteBeforeStar = regexp.MustCompile(`[.)]\^\{\d+\}$`)

// ScriptStars puts the corpus's star back wherever a model wrote it as a script
// inside a math span, and returns the text and how many it put back.
//
// It is deliberately not part of the rune walk in Stars. That walk asks what a
// single glyph means and answers by where it sits; this asks what a whole span
// is, and the answer turns on the span having no mathematics in it at all. A
// span with a base is left alone however it is written, so $f_*$ and $K^*$ and
// $(g \circ f)_*$ are never touched, and the rule cannot reach a pushforward by
// accident.
func ScriptStars(text string) (string, int) {
	spans, _ := mathtex.Split(text)
	rs := []rune(text)
	type edit struct {
		from, to int // in runes, taking the delimiters with it
		with     string
	}
	var edits []edit
	for _, s := range spans {
		if s.Display {
			continue
		}
		from, to := s.Start-1, s.End+1
		if from < 0 || to > len(rs) || rs[from] != '$' || rs[to-1] != '$' {
			continue // a delimiter this is not equipped to reason about
		}
		before := ' '
		if from > 0 {
			before = rs[from-1]
		}
		if s.Text == openingScriptStar {
			// Only where the prose leaves no base for it. Anything else is a
			// dual whose base is on the wrong side of the dollar.
			if before == ' ' || before == '\n' || before == '\t' {
				edits = append(edits, edit{from, to, Star})
			}
			continue
		}
		if s.Text == "_*" && (isLetter(before) || isDigit(before)) {
			// A base in the prose, which is fix math's to move. The corpus has
			// none today -- every $_*$ in it follows a space, a full stop or a
			// bracket -- and this is what keeps it that way if one arrives.
			// The forms with punctuation in the span need no such guard: no
			// reading of "domain$._*$" makes domain the base of anything.
			continue
		}
		if e, ok := scriptStarEdit(s.Text, from, to); ok {
			edits = append(edits, e)
		}
	}
	if len(edits) == 0 {
		return text, 0
	}
	var b strings.Builder
	at := 0
	for _, e := range edits {
		b.WriteString(string(rs[at:e.from]))
		b.WriteString(e.with)
		at = e.to
	}
	b.WriteString(string(rs[at:]))
	return b.String(), len(edits)
}

// scriptStarEdit reads one span and says what belongs in its place, or that it
// is not this rule's business.
func scriptStarEdit(text string, from, to int) (struct {
	from, to int
	with     string
}, bool) {
	var e struct {
		from, to int
		with     string
	}
	for _, c := range scriptStar {
		if text == c.content {
			e.from, e.to, e.with = from, to, c.prose+Star
			return e, true
		}
	}
	// Something glued in with the mark. What settles it is the tail: a full
	// stop before the _* ends a sentence, and no subscript is written after a
	// sentence ends, so the star is the mark and the head is what it was hung
	// on. A run of digits is the number of a lemma or a no. and is prose;
	// anything else was already being set as mathematics and stays that way.
	rest, ok := strings.CutSuffix(text, "_*")
	if !ok || rest == "" {
		return e, false
	}
	// A footnote reference between the end of the sentence and the mark. The
	// volume sets the number of the note in the mathematics either way round,
	// "$X_{\\alpha}.^{10}$" and "le to $F.$)$^{10}$", so the one that has
	// swallowed the mark keeps everything else it holds and gives back the
	// mark alone. Chapter IX of Lie Groups cites its thirteenth note this way.
	if footnoteBeforeStar.MatchString(rest) {
		e.from, e.to, e.with = from, to, "$"+rest+"$"+Star
		return e, true
	}
	// Peel the punctuation off the end of what is left. A full stop is always
	// prose. A bracket is prose only when the span has no opening one to pair
	// it with: $(p_1)_*$ and $\mathrm{Tor}_1^R(R/R_+, E)_*$ are pushforwards
	// and close what they opened, while Commutative Algebra V writes
	// "(considerer la fonction meromorphe $1/(\sin \pi z))_*$" with the
	// bracket of the aside swept in along with the mark.
	head, tail := rest, ""
	for head != "" {
		if h, ok := strings.CutSuffix(head, "."); ok {
			head, tail = h, "."+tail
			continue
		}
		if h, ok := strings.CutSuffix(head, ")"); ok && strings.Count(head, ")") > strings.Count(head, "(") {
			head, tail = h, ")"+tail
			continue
		}
		break
	}
	if tail == "" || head == "" {
		return e, false
	}
	if allDigits(head) {
		e.from, e.to, e.with = from, to, head+tail+Star
		return e, true
	}
	e.from, e.to, e.with = from, to, "$"+head+"$"+tail+Star
	return e, true
}

func allDigits(s string) bool {
	for _, r := range s {
		if !isDigit(r) {
			return false
		}
	}
	return s != ""
}

func isDigit(r rune) bool  { return r >= '0' && r <= '9' }
func isLetter(r rune) bool { return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' }
