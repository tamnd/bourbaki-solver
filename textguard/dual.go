package textguard

import (
	"strings"

	"github.com/tamnd/bourbaki-solver/mathtex"
)

// strandedScript is the other thing a base-less superscript can be, and it is
// written with the same three characters ScriptStars reads at the head of a
// line. There the prose leaves nothing for the script to sit on and the span is
// the forward-reference mark; here the prose leaves a base, and the span is a
// dual, an adjoint or a conjugate whose base the text layer stranded outside the
// delimiters.
const strandedScript = "^*"

// StrandedBase pulls a base back into the mathematics that carries its script.
//
// Bourbaki sets the base of a dual upright -- S, A, E, G, D, a single roman
// capital -- and an upright capital is what a text layer reads as prose. So it
// hands back cl(S$^*$) where the page prints cl(S*), with the letter outside the
// mathematics and the star alone inside it. The two print differently: the S
// comes out in the body font and the star sits at the foot of a line of its own
// making, and nothing about the span says what it is a dual of. A translator
// asked to copy the formulae copies "^*", correctly, and the audit compares that
// against the English "^*" and finds them equal, so the fault survives every
// rule that reads mathematics -- because as far as those rules can see, there is
// no mathematics here to be wrong.
//
// The base is taken only where the prose leaves one letter and that letter
// stands on its own. QQ$^*$ and Tr(BA$^*$) have two, and taking the nearer one
// would write $Q^*$ for what is a dual of QQ and BA; those are left for somebody
// to read, along with every base that closes a bracket -- (($u$)$^*$) and
// (End(V))*$^*$ -- where what belongs inside the delimiters is a decision and
// not a rule. This moves one letter and one pair of delimiters and never a
// character of prose or of mathematics.
func StrandedBase(text string) (string, int) {
	spans, _ := mathtex.Split(text)
	rs := []rune(text)
	var b strings.Builder
	at, n := 0, 0
	for _, s := range spans {
		if s.Display || s.Text != strandedScript {
			continue
		}
		from, to := s.Start-1, s.End+1
		if from < 0 || to > len(rs) || rs[from] != '$' || rs[to-1] != '$' {
			continue // a delimiter this is not equipped to reason about
		}
		base := from - 1
		if base < at || !isLetter(rs[base]) {
			continue
		}
		// A letter, a digit, a brace or a backslash before it means the base is
		// longer than the one letter and this rule has nothing to say.
		if base > 0 {
			switch c := rs[base-1]; {
			case isLetter(c), isDigit(c), c == '\\', c == '}', c == '$':
				continue
			}
		}
		b.WriteString(string(rs[at:base]))
		b.WriteString("$" + string(rs[base]) + s.Text + "$")
		at, n = to, n+1
	}
	if n == 0 {
		return text, 0
	}
	b.WriteString(string(rs[at:]))
	return b.String(), n
}
