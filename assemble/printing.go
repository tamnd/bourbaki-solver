package assemble

import (
	"fmt"
	"regexp"
)

// A printing of the Éléments is laid out the same way in either language and
// says so in its own words. The French chapter VIII opens on CHAPITRE VIII
// where the English opens on CHAPTER 8, it heads its exercises EXERCICES where
// the English heads them Exercises, and it states a Lemme where the English
// states a Lemma. None of that is a difference in structure, and none of it can
// be guessed at: it is written down here, one entry per printing, and the
// assembler is handed the entry for the language it is assembling.
//
// What is deliberately not in here is the label of a statement. Proposition 6
// of § 1 is one statement of the Éléments printed twice, so both printings give
// it the label alg-viii-s1-prop-6 and both carry the same permanent tag. The
// printing decides what a reader sees, never what a thing is called.
type printing struct {
	lang string

	// chapter, appendix, historical and exercises are the headings extraction
	// writes over the four pieces that are not a §, exactly as they appear in
	// pages/. The level is part of them: the French volume sets its appendix
	// headings smaller than its § headings and the English one does not, so
	// they come out of extraction at different depths.
	chapter    string
	appendix   string // takes the number
	historical string
	exercises  string

	// head is a statement as the printing sets it and extraction writes it.
	head *regexp.Regexp
}

// appendixHead is the heading over Appendix n.
func (p printing) appendixHead(n int) string { return fmt.Sprintf(p.appendix, n) }

// printings is every printing the assembler can read.
var printings = map[string]printing{
	"en": {
		lang:       "en",
		chapter:    "## CHAPTER",
		appendix:   "## APPENDIX %d",
		historical: "# HISTORICAL NOTE",
		exercises:  "### Exercises",
		head: regexp.MustCompile(
			`^(?:\*\*(` + enKinds + `)(?: (\d+))?\.\*\*|(` + enKinds + `)(?: (\d+))? \([^)]*\)\.|` +
				smallType + `(` + enKinds + `)(?: (\d+))?\.)\s*—\s*`),
	},
	"fr": {
		lang:       "fr",
		chapter:    "## CHAPITRE",
		appendix:   "### APPENDICE %d",
		historical: "# NOTE HISTORIQUE",
		exercises:  "## EXERCICES",
		head: regexp.MustCompile(
			`^(?:\*\*(` + frKinds + `)(?: (\d+))?\.\*\*|(` + frKinds + `)(?: (\d+))? \([^)]*\)\.|` +
				smallType + `(` + frKinds + `)(?: (\d+))?\.|(` + frPlainKinds + `)(?: (\d+))?\.)\s*—\s*`),
	},
}

// printingOf is the entry for a language, and an error rather than a guess when
// there is none: assembling a printing nobody has described would read every
// statement of it as prose and report a chapter with no statements in it.
func printingOf(lang string) (printing, error) {
	p, ok := printings[lang]
	if !ok {
		return printing{}, fmt.Errorf("nothing describes how the %q printing is laid out", lang)
	}
	return p, nil
}

// smallType is the mark that opens a passage set in small type.
const smallType = `\$\*\$`

const enKinds = `Definitions?|Propositions?|Theorems?|Lemmas?|Corollary|Corollaries|Remarks?|Examples?|Scholium`

// frKinds are the words the French printing states its results in. The accents
// are the volume's own and the plurals are too: it sets "Remarques. —" over a
// run of remarks exactly as the English sets "Remarks. —".
const frKinds = `D[ée]finitions?|Propositions?|Th[ée]or[èe]mes?|Lemmes?|Corollaires?|Remarques?|Exemples?|Scholie`

// frPlainKinds are the kinds the French printing sets in italic rather than in
// small capitals, and so the ones that reach here with no bold on them.
//
// The English printing sets every head in bold, which is why its grammar refuses
// an undecorated head: "Theorem 1. —" is a shape those pages never produce and
// reading one as a head would put the grammar at the mercy of a sentence that
// happens to open on a kind. The French printing does produce it, for these four
// kinds and for no others: all 19 lemmas of chapter VIII are set this way, and so
// are its 56 remarks and its 31 examples, while every Proposition, Théorème,
// Définition and Corollaire is in small capitals. The em dash is what keeps the
// rule honest, since a sentence opening on the word Exemple does not carry one.
const frPlainKinds = `Lemmes?|Remarques?|Exemples?|Scholie`
