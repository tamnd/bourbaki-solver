package textguard

import (
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
	return b.String(), n
}

// Ornaments is the same reading with nothing given back, for the audit. It hands
// over each line that carries one and says what it found, so a finding can name
// it rather than leave somebody to look up a code point.
func Ornaments(text string) []Ornament {
	math := inMath(text)
	rs := []rune(text)
	var out []Ornament
	at, line := 0, 1
	for _, l := range strings.Split(text, "\n") {
		for i, r := range []rune(l) {
			name, bad := ornament[r]
			if !bad || math[at+i] {
				if !bareStar(rs, at+i, math) {
					continue
				}
				name = "a bare asterisk"
			}
			out = append(out, Ornament{Line: line, Name: name, Text: l})
			break
		}
		at += len([]rune(l)) + 1
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
