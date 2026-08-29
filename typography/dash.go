package typography

import (
	"regexp"
	"strings"
)

// frenchKinds are the kinds of statement a French volume of the Éléments sets a
// head for, in the two spellings the pages come back in. It is assemble's
// frAnyKinds written again here rather than called, because the assembler reads
// this package and not the other way round.
const frenchKinds = `D[ée]finitions?|Propositions?|Th[ée]or[èe]mes?|Lemmes?|Corollaires?|Remarques?|Exemples?|Scholie` +
	`|D[ÉE]FINITIONS?|PROPOSITIONS?|TH[ÉE]OR[ÈE]MES?|LEMMES?|COROLLAIRES?|REMARQUES?|EXEMPLES?|SCHOLIE`

// shortDash is a French statement head followed by a dash shorter than the one
// the printing sets. The head itself is left to the first group and only the
// mark is replaced, so whatever spacing the reading put on either side of it
// stays as the reading left it.
var shortDash = regexp.MustCompile(
	`^(\*{0,2}(?:` + frenchKinds + `)(?: \d+)?(?:\s*\([^)]*\))?\.\*{0,2}\s*)[-–](\s)`)

// StatementDash is the body with the em dash put back after every statement
// head that came back with a shorter mark, and the number of them.
//
// The French printing sets an em dash between the head of a statement and the
// statement itself, "PROPOSITION 8. — Soit A un anneau", and 18200 heads in
// this corpus carry it. Six carry a hyphen or an en dash instead, all of them
// in the tenth chapter of Algebre commutative, which is the volume read most
// recently. It is the same fault as a straight apostrophe for a typographic
// one: the reading is looking at the mark the press set and writing the nearest
// thing on a keyboard.
//
// It costs what the apostrophe does not. The assembler finds a French statement
// by its head and the dash is part of the head it looks for, so a head with a
// hyphen in it is not a statement, and a corollary printed under it is numbered
// under whatever statement came before instead. Chapter X of Algebre
// commutative gave two corollaries the same label that way, one under
// proposition 8 on page 32 and one that belongs under proposition 9 on page 34,
// and the volume did not assemble.
//
// Only a line that opens a paragraph is looked at, which is the rule SmallCaps
// works by and for the same reason. The pages set a paragraph on one long line,
// so a head is a line that starts with the word and has a blank line in front of
// it, and asking for the blank line keeps this off the lines of a display and
// off prose that happens to be broken there.
func StatementDash(body string) (string, int) {
	lines := strings.Split(body, "\n")
	changed := 0
	for i, line := range lines {
		if i > 0 && strings.TrimSpace(lines[i-1]) != "" {
			continue
		}
		out := shortDash.ReplaceAllString(line, "$1—$2")
		if out == line {
			continue
		}
		lines[i] = out
		changed++
	}
	return strings.Join(lines, "\n"), changed
}
