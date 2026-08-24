package typography

import (
	"regexp"
	"strings"
)

// Two things a printing varies in a title without changing the title, and both
// of them defeat a comparison that works on the letters.
//
// Comparing a heading on a page against the entry the table of contents gives it
// is done in two places, once when a missing heading is put back and once when
// the pages are read into content, and the two flatten a title differently for
// reasons of their own. What they have in common is these two facts about how a
// title is set, so the facts live here rather than in either of them.

// footnote is the marker a printing hangs off the end of a heading, which is a
// run of mathematics with a caret and a digit in it and nothing else.
//
// The character in front is carried through the replacement because RE2 has no
// way to look behind, and it is there to keep this off a real exponent. Three
// lines in this corpus set one, "(sin($\pi x$)$/(\pi x)$)$^2$" and
// "Ker(ad $x$)$^2$" among them, and every one of the three raises a closing
// parenthesis to the power. A footnote marker never follows a parenthesis. It
// follows a letter or a stop or a comma, so the parenthesis is the whole of what
// is refused.
var footnote = regexp.MustCompile(`([^)])\s*\$\s*\^\s*\{?\s*\d{1,2}\s*\}?\s*\$`)

// Footless is a title with the footnote markers taken off it.
//
// Page 69 of Groupes et algebres de Lie IX heads § 7 with a marker after the
// last word of the title and page 263 of Espaces vectoriels topologiques I a V
// heads chapter V the same way. The marker is on the paper and belongs on the
// page, so it is set aside for the comparison and left in the heading, which is
// why this returns a string to compare rather than one to write.
//
// Flattening cannot do this on its own. It throws away the dollars and the caret
// and keeps the digit, so the two sides come out differing by a 1 that is no
// part of either title.
func Footless(s string) string { return footnote.ReplaceAllString(s, "$1") }

// Accentless is a string with the accents a printing of this corpus was found
// dropping folded away, and every other accent left standing.
//
// The table is one letter and it is meant to stay that size. Page 38 of Algebre
// I a III heads § 4 of chapter I "§ 4. GROUPES ET GROUPES A OPÉRATEURS" and its
// own table of contents lists the same § as "Groupes et groupes à opérateurs".
// The page image settles what happened: the press set the preposition as a bare
// capital A and kept the acute on OPERATEURS, on the same line, four words
// apart. So this is not a press that does not accent capitals and it is not a
// reading that lost an accent. It is one word that French sets without its grave
// when it sets it in capitals, and the E beside it is the proof.
//
// Folding every accent instead would be easier to write and wrong. It would take
// a page heading a section "DUAL OF A FRECHET SPACE" against a contents entry
// reading "Dual of a Fréchet space" and call them the same title, and that pair
// is a reading that dropped an accent, which is a defect in the page and a thing
// somebody should be told about. The corpus has 388 French pages with apostrophe
// damage already and no need for a rule that hides the next class of it.
//
// So the table grows only when a page image says it should, the same way the
// class of characters a section sign is misread as grew.
func Accentless(s string) string { return accents.Replace(s) }

var accents = strings.NewReplacer("à", "a", "À", "A")
