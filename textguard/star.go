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

// Stars puts the corpus's star back wherever a model wrote an ornament for it,
// and returns the text and how many it put back.
//
// It works outside the math spans only, and that is not tidiness. U+2217 is the
// asterisk operator and inside the mathematics it is a binary operation, a dual
// or a pullback, so the same glyph means one thing on one side of a dollar and
// another on the other. Outside a span it can be nothing but the mark, since
// prose has no operators in it. Inside one it belongs to bourbaki fix math,
// which turns it into the TeX that prints it. The other three have no reading
// anywhere and are left inside a span all the same, so that this function and
// the rule that reads it cannot disagree about where the mark can be.
func Stars(text string) (string, int) {
	math := inMath(text)
	rs := []rune(text)
	var b strings.Builder
	n := 0
	for i, r := range rs {
		if _, bad := ornament[r]; bad && !math[i] {
			b.WriteString(Star)
			n++
			continue
		}
		b.WriteRune(r)
	}
	return b.String(), n
}

// Ornaments is the same reading with nothing given back, for the audit. It hands
// over each line that carries an ornament and says which glyph it found, so a
// finding can name it rather than leave somebody to look up a code point.
func Ornaments(text string) []Ornament {
	math := inMath(text)
	var out []Ornament
	at, line := 0, 1
	for _, l := range strings.Split(text, "\n") {
		for i, r := range []rune(l) {
			name, bad := ornament[r]
			if bad && !math[at+i] {
				out = append(out, Ornament{Line: line, Name: name, Text: l})
				break
			}
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
