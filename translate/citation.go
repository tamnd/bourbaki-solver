package translate

import (
	"regexp"
	"strings"
)

var (
	// parens is a parenthesis, and one parenthesis inside it. Bourbaki cites a
	// book with its publisher in brackets of its own, "(H. CURRY, *Outlines of a
	// Formalist Philosophy of Mathematics*, Amsterdam (North Holland Publ. Co.),
	// 1951, p. 57)", and one level is all the corpus has.
	parens = regexp.MustCompile(`\((?:[^()]|\([^()]*\))*\)`)
	// caps is a word the printing sets in capitals, which is how the book prints
	// the surname of an author it cites in the middle of a sentence.
	caps = regexp.MustCompile(`\b\p{Lu}{3,}\b`)
	// roman is what caps finds the rest of the time: a chapter number.
	roman = regexp.MustCompile(`^[IVXLCDM]+$`)
)

// WithoutCitations is a translation with the works it cites by name taken out of
// it, and it takes out nothing the English does not have word for word.
//
// A work cited in the middle of a sentence stands as printed, for the reason a
// numbered bibliography entry does: the title is the name of a book on a shelf,
// and a title in another language leads nowhere. The rule beside this one then
// reads the citation as a run of sixteen words with nothing Vietnamese in it,
// which is what it is, and refuses the only answer there is. That is the same
// contradiction the bibliography had, in the one place it survives: the entries
// are gathered under a heading and these are not.
//
// Two conditions, and both are needed. The English has to have the citation word
// for word, which is what says it was copied rather than composed, and it has to
// name somebody in capitals, which is how the book prints an author. The whole
// English corpus holds two of them, H. CURRY in the historical note of chapter
// IV and H. WEYL in Algebra, so this covers what it is written for and no more:
// a parenthesis of ordinary prose left in English is refused as it was.
func WithoutCitations(en, tr string) string {
	if !strings.Contains(tr, "(") {
		return tr
	}
	return parens.ReplaceAllStringFunc(tr, func(run string) string {
		if !names(run) || !strings.Contains(en, run) {
			return run
		}
		return " "
	})
}

// names says whether a parenthesis holds a surname printed in capitals. A
// chapter is capitals too and is not a name.
func names(run string) bool {
	for _, w := range caps.FindAllString(run, -1) {
		if !roman.MatchString(w) {
			return true
		}
	}
	return false
}
