package typography

import (
	"regexp"
	"strings"
)

// statementHead is the head of a statement as an English volume of the Éléments
// sets it, written by a reading that lost the small capitals: the kind, an
// optional number, an optional name in parentheses, and the full stop that
// closes the head.
//
// The four kinds are the four every English printing in the corpus sets in
// small capitals and none of them sets any other way. Lemma, Remark, Example and
// Scholium are left out on purpose: Lie 7 to 9 sets those four in italic, so
// "Lemma 2." at the head of a paragraph is how that volume prints it and the
// assembler already reads it, and putting them in capitals would be inventing a
// printing that does not exist.
//
// The bold form is left out for the same reason. Algebra VIII sets its heads in
// bold and a reading gives "**Proposition 6.**", which is the page as printed,
// so only a head that starts with the bare word is looked at.
var statementHead = regexp.MustCompile(
	`^(Definitions?|Propositions?|Theorems?|Corollary|Corollaries)( \d+)?(\s*\([^)]*\))?\.`)

// headDash is the same head followed by a dash, which is a different fault and
// not this one.
//
// Algebra VIII prints "Proposition 6. — Let ..." with the kind in bold, and a
// reading that loses the bold leaves the dash standing where it was. Putting the
// kind in capitals there would make the head readable and leave the dash at the
// front of the statement's body, which is worse than leaving the line alone: the
// dash belongs to the head and the head is what has to be put back. Those lines
// are counted and reported and nothing is done to them.
var headDash = regexp.MustCompile(
	`^(?:Definitions?|Propositions?|Theorems?|Corollary|Corollaries)( \d+)?(\s*\([^)]*\))?\.\s*[-–—]`)

// SmallCaps is the body with the kind of every statement head written in
// capitals, the lines it changed and the lines it left alone because a dash
// follows the head.
//
// The English volumes set DEFINITION, PROPOSITION, THEOREM and COROLLARY in
// small capitals and the corpus writes small capitals as capitals, which is what
// almost every page already does. A reading that writes "Proposition 1." instead
// is reading the same type and writing it as the nearest thing on a keyboard,
// the same as a straight apostrophe for a typographic one, and it costs the same
// thing: the assembler finds a statement by its head, so a head in the wrong case
// is not a statement at all, the corollaries under it hang from nothing, and the
// § it stands in does not assemble. 308 heads across five volumes are in that
// state and chapter II of Lie 1 to 3 is held back by one of them.
//
// Only a line that opens a paragraph is looked at. The pages set a paragraph on
// one long line, so a head is a line that starts with the word and has a blank
// line in front of it, and asking for the blank line keeps this off a line of a
// display and off prose that happens to be broken there.
func SmallCaps(body string) (string, int, int) {
	lines := strings.Split(body, "\n")
	changed, left := 0, 0
	for i, line := range lines {
		if i > 0 && strings.TrimSpace(lines[i-1]) != "" {
			continue
		}
		if headDash.MatchString(line) {
			left++
			continue
		}
		m := statementHead.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		lines[i] = strings.ToUpper(m[1]) + line[len(m[1]):]
		changed++
	}
	return strings.Join(lines, "\n"), changed, left
}
