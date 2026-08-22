// Package typography puts back the marks a printing sets that a reading of the
// page image writes as the nearest thing on a keyboard.
//
// These are not misreadings of the words. The words are right and the mark
// between them is wrong, which makes them cheap to find and cheap to be sure
// of, and it makes them worth finding at all: two spellings of the same word in
// one volume break a search, they show up as a difference between two pages
// that say the same thing, and they reach the translations, where the French
// side of a glossary entry has to match the French in the page for the entry to
// be found.
package typography

import (
	"regexp"
	"strings"
)

// elision is a French word that drops its vowel before the next one and takes
// an apostrophe where the vowel was, followed by a straight apostrophe and a
// letter.
//
// The list is closed and short because that is the whole of what makes this
// safe. A straight apostrophe in the text of a French page is one of two
// things: an elision, which is a mark of the language, or a prime on a letter
// that got out of its dollars, which is mathematics. Counted over the corpus
// there are 10219 of them, and asking which word is in front of the apostrophe
// tells the two apart on all but a couple of hundred: l', d', qu', n', s' and
// c' are 9781 of the total between them, and what is left over reads x'_1,
// e'_i, Q'_\mathfrak{p}, Σ'_1, which are primes and subscripts and no part of
// any word.
//
// Turning a prime into a typographic apostrophe would be a worse fault than the
// one being repaired, because a prime is mathematics and this is a repair of
// prose. So the rule is written to refuse everything it is not sure of, and the
// ones it refuses stay as they are and are counted.
//
// aujourd'hui is here because it is an elision that fused into one word, and it
// is the only one of those Bourbaki writes.
var elision = regexp.MustCompile(`(?i)\b(l|d|n|s|c|j|m|t|qu|jusqu|lorsqu|puisqu|quoiqu|aujourd)'(\pL)`)

// straight is a straight apostrophe standing on the back of a letter or a
// digit, which is every one this package has an opinion about.
//
// What comes after it is not asked, because that is where the two cases part
// company and the count is taken before they are told apart. An elision is
// followed by a letter and a prime is followed by a subscript, a bracket or
// another letter, and a count that only looked at the elisions would report
// nothing left over on a page whose mathematics is out in the prose.
var straight = regexp.MustCompile(`(?:\pL|\pN)'`)

// Apostrophes is the text with the straight apostrophe of an elision written as
// the typographic one, the count it changed and the count it left alone.
//
// The corpus already answers the question of which mark is right. 5251 French
// pages set the typographic apostrophe and none set the straight one as their
// only form, so the reading that sets a straight one is the odd one out, and it
// is odd page by page rather than volume by volume: Algebra I to III in French
// has 506 pages with the typographic mark and 123 with the straight one, from
// the same model reading the same printing.
//
// Only prose is looked at. Everything between dollar signs is mathematics and
// an apostrophe there is a prime, so it is stepped over and not counted either
// way.
func Apostrophes(text string) (string, int, int) {
	var b strings.Builder
	changed, left := 0, 0
	// The odd pieces of a split on $$ are display math and the odd pieces of a
	// split of what is left on a single $ are inline math. Both are put back
	// exactly as they came, separators and all.
	for i, block := range strings.Split(text, "$$") {
		if i > 0 {
			b.WriteString("$$")
		}
		if i%2 == 1 {
			b.WriteString(block)
			continue
		}
		for j, part := range strings.Split(block, "$") {
			if j > 0 {
				b.WriteString("$")
			}
			if j%2 == 1 {
				b.WriteString(part)
				continue
			}
			was := len(straight.FindAllString(part, -1))
			now := elision.ReplaceAllString(part, "$1’$2")
			changed += was - len(straight.FindAllString(now, -1))
			left += len(straight.FindAllString(now, -1))
			b.WriteString(now)
		}
	}
	return b.String(), changed, left
}
