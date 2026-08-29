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

// noDash is a French statement head with no mark after it at all. The head goes
// to the first group and the statement to the second, and the mark is written
// between them in the spacing the printing uses, since there is no spacing on
// the page to preserve here: the reading dropped the whole of it.
//
// The second group has to open on something, and on something that is not a
// dash. A head alone on its line is a head whose statement begins on the next
// line, and there is no place on that line for the mark; a head already carrying
// a mark is either right already or is shortDash's to fix.
var noDash = regexp.MustCompile(
	`^(\*{0,2}(?:` + frenchKinds + `)(?: \d+)?(?:\s*\([^)]*\))?\.\*{0,2}) +([^\s—–-].*)$`)

// insideDash is a head whose mark the reading put inside the emphasis instead
// of after it, "**Corollaire 1.—**" where the printing sets "**Corollaire 1.**
// —". The press sets the head bold and the mark in the ordinary face, and a
// reading that takes the mark for part of the head closes the emphasis one
// character late.
//
// It is the same fault as the two above in what it costs. The head the
// assembler looks for ends at the emphasis and the mark comes after it, so a
// head built this way is not one, and the statements printed under it are
// numbered under whatever came before. Proposition 10 of chapter X of Algebre
// commutative is one of these, and its three corollaries went to proposition 9.
var insideDash = regexp.MustCompile(
	`^(\*\*(?:` + frenchKinds + `)(?: \d+)?(?:\s*\([^)]*\))?\.)\s*[-–—]\*\*(\s)`)

// StatementDash is the body with the em dash put back after every statement
// head that came back without it, and the number of them.
//
// The French printing sets an em dash between the head of a statement and the
// statement itself, "PROPOSITION 8. — Soit A un anneau", and 18200 heads in
// this corpus carry it. Fifty nine do not, in three shapes: ten carry a hyphen
// or an en dash, thirty six carry nothing at all, and thirteen carry the mark
// but inside the emphasis the head is set in rather than after it. All but four
// of the fifty nine are in the tenth chapter of Algebre commutative, which is
// the volume read most recently.
//
// The short mark is the same fault as a straight apostrophe for a typographic
// one, the reading looking at what the press set and writing the nearest thing
// on a keyboard. The missing mark is the reading seeing a rule of that length
// as ornament and leaving it out, which is the same thing it does to the
// running head and to the chapter numeral. The mark inside the emphasis is the
// reading taking it for part of the head, which is the one of the three a
// person would also get wrong.
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
		// insideDash comes first. A head it matches carries a mark, so
		// noDash would not touch it, but the mark it carries may be a
		// hyphen, and shortDash reading that hyphen would write an em
		// dash still inside the emphasis and call the line repaired.
		out := insideDash.ReplaceAllString(line, "$1** —$2")
		if out == line {
			out = shortDash.ReplaceAllString(line, "$1—$2")
		}
		if out == line {
			out = noDash.ReplaceAllString(line, "$1 — $2")
		}
		if out == line {
			continue
		}
		lines[i] = out
		changed++
	}
	return strings.Join(lines, "\n"), changed
}
