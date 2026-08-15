package assemble

import (
	"fmt"
	"regexp"
	"strings"
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
	appendix   string // the word alone, the number is put after it
	historical string
	exercises  string

	// gathered is the heading over a whole chapter's worth of exercises, in the
	// volumes that print them that way rather than one run per §. It is set
	// larger than the per-§ heading because it stands for more, and what is
	// under it is divided by a mark rather than by a heading. See gathered.
	gathered string

	// head is a statement as the printing sets it and extraction writes it.
	head *regexp.Regexp

	// swallowed is a head whose number was drawn into the maths that follows it,
	// which is a thing that happens to a printing that sets the number hard
	// against the first word. Empty for a printing it does not happen to. See
	// unswallow.
	swallowed *regexp.Regexp
}

// unswallow puts back a head whose number the maths took, and returns the line
// unchanged when nothing took it.
//
// Lie 7 to 9 sets a statement in small capitals and puts nothing between the
// number and what comes after it, so extraction reads the pair as one run and
// the dollar lands before the number instead of after it: "PROPOSITION $5.a)$
// Every conjugacy class of G meets T." and "COROLLARY $1.\mathfrak{g}$ is
// simple". The number and its period are the head and the rest is the body, so
// the dollar is moved to the join. What follows really is maths in most of them,
// which is why it is reopened rather than dropped.
//
// A footnote on the head does the same thing for the same reason. The mark is
// set as a superscript, so the number and the mark come out as one span, either
// side of the period: "Lemma $4.^2$ Let A be a graded k-algebra" and "Lemma
// $2^2$. (i) Let W be a vector subspace of V". The first of those is already the
// shape above. The second closes the span before the period rather than after
// the number, so the mark is carried over to the front of the body and reopened
// there, which is what the star that brackets a passage in small type gets too.
// Dropping it would leave the footnote at the foot of the page with nothing
// pointing at it.
//
// Nine lines of that volume are like this and no line of any other is. Left
// alone each one is a statement that was never read, and § 2 of chapter IX loses
// its Propositions 5, 6 and 7 together with the nine citations to them.
func (p printing) unswallow(text string) string {
	if p.swallowed == nil {
		return text
	}
	m := p.swallowed.FindStringSubmatch(text)
	if m == nil {
		return text
	}
	rest := strings.TrimLeft(text[len(m[0]):], " ")
	if m[3] != "" {
		return m[1] + " " + m[2] + ". $" + m[3] + "$ " + rest
	}
	return m[1] + " " + m[2] + ". $" + rest
}

// appendixHeads is the heading over Appendix n, both ways it is numbered.
//
// The two English volumes disagree and neither is wrong: Algebra VIII heads its
// four APPENDIX 1 to APPENDIX 4, Lie 7 to 9 heads its two APPENDIX I and
// APPENDIX II. It is not a difference in language, so it is not one printing
// against another, and it is not worth a field per volume for a choice of
// numeral. The page is asked which it uses.
// A chapter that closes with one appendix does not always number it. Chapter I
// of Theory of Sets heads its own APPENDIX and sets CHARACTERIZATION OF TERMS
// AND RELATIONS under it, and chapters II and III of Algebra I to III do the
// same, so the contents gives all three the number 0 and the heading is the word
// alone.
func (p printing) appendixHeads(n int) []string {
	if n == 0 {
		return []string{p.appendix}
	}
	return []string{fmt.Sprintf("%s %d", p.appendix, n), fmt.Sprintf("%s %s", p.appendix, roman(n))}
}

// roman writes a small number the way a volume that numbers its appendices that
// way writes it. Four is as far as any chapter of the Éléments goes.
func roman(n int) string {
	if n < 1 || n > len(romans) {
		return fmt.Sprint(n)
	}
	return romans[n-1]
}

var romans = []string{"I", "II", "III", "IV", "V", "VI", "VII", "VIII", "IX", "X"}

// printings is every printing the assembler can read.
var printings = map[string]printing{
	"en": {
		lang:       "en",
		chapter:    "## CHAPTER",
		appendix:   "## APPENDIX",
		historical: "# HISTORICAL NOTE",
		exercises:  "### Exercises",
		gathered:   "# EXERCISES",
		head: regexp.MustCompile(
			`^(?:(?:\*\*(` + enKinds + `)(?: (\d+))?\.\*\*|(` + enKinds + `)(?: (\d+))? \([^)]*\)\.|` +
				smallType + `(` + enKinds + `)(?: (\d+))?\.)\s*—\s*` +
				`|(` + enCapKinds + `)(?: (\d+))?(?: \([^)]*\))?\.\s*` +
				`|(` + enPlainKinds + `)(?: (\d+))?\.\s+` +
				`|\*\*(` + enCapKinds + `)(?: (\d+))?\.\*\*\s*` +
				`|\*(` + enPlainKinds + `)(?: (\d+))?\.\*\s+` +
				`|` + smallTypeSup + `(` + enPlainKinds + `)(?: (\d+))?\.?\s+)`),
		swallowed: regexp.MustCompile(`^(` + enCapKinds + `|` + enPlainKinds + `) \$(\d+)(?:(\^\d+)\$)?\.`),
	},
	"fr": {
		lang:       "fr",
		chapter:    "## CHAPITRE",
		appendix:   "### APPENDICE",
		historical: "# NOTE HISTORIQUE",
		exercises:  "## EXERCICES",
		gathered:   "# EXERCICES",
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

// smallTypeSup is the same mark where the printing sets it as a superscript,
// which Lie 7 to 9 does and Algebra VIII does not.
const smallTypeSup = `\$\^\*\$`

// smallTypeOpen is either of them at the front of a head, which is how the mark
// is found again once the head has been read, so that it can be put back on the
// body it belongs to.
var smallTypeOpen = regexp.MustCompile(`^(?:` + smallType + `|` + smallTypeSup + `)`)

// headName is the name a printing gives a result, in parentheses between the
// kind and the dash: "Theorem 1 (Wedderburn). —", "Théorème 2 (« lemme de
// Nakayama »). —". Both grammars have a branch for it and neither sets anything
// else in a head between parentheses, so the parentheses alone find it once the
// head has been matched.
//
// It is searched for in the head and not in the line, or the parenthesis of a
// head that has none would be found in the statement under it.
var headName = regexp.MustCompile(`\(([^)]*)\)`)

const enKinds = `Definitions?|Propositions?|Theorems?|Lemmas?|Corollary|Corollaries|Remarks?|Examples?|Scholium`

// enCapKinds and enPlainKinds are the two ways the other English printing sets a
// head, and it sets no head any other way.
//
// Algebra VIII prints "Proposition 6. — Let ...", bold and then an em dash. Lie 7
// to 9 prints no dash at all and divides its kinds: Definition, Proposition,
// Theorem and Corollary go in small capitals, and Lemma, Remark, Example and
// Scholium go in italic and reach here undecorated, exactly the division the
// French printing makes. That is a difference between two printings of the same
// language, like the numeral over an appendix, so both are read rather than one
// being chosen by the volume.
//
// What keeps the second of these honest, with no dash to lean on, is that the
// period has to follow the number immediately. Chapter I says "Lemma 4 is now
// immediate." and cites "Lemma 2)." and neither is a head. The four other English
// volumes between them open no paragraph in either of these shapes.
// Twelve heads of Lie 7 to 9 reach here in bold as well as in capitals, which is
// the last branch of the second group. They are all on pages that were read by
// the model rather than out of the type, and the model marks the small capitals
// it sees. The dash the first group leans on is still not there, so they cannot
// go in with Algebra VIII's bold heads; and unmarked capitals are already read,
// so the choice is between reading the marked ones too and losing a Proposition
// 5, 6 and 7 of § 2 of chapter IX along with every citation to them. Algebra VIII
// prints no head in this shape.
//
// A head in small capitals is followed by whatever space the volume set, and by
// none at all where the volume set none: chapter IX prints "PROPOSITION 18.The
// group Aut(G)/Int(G)". The kinds in italic are held to a space, since those are
// words a sentence can open on and the period after the number is all that stands
// between a head and a sentence. Nothing in small capitals is a word a sentence
// opens on, so nothing is being traded away there.
const enCapKinds = `DEFINITIONS?|PROPOSITIONS?|THEOREMS?|LEMMAS?|COROLLARY|COROLLARIES|REMARKS?|EXAMPLES?|SCHOLIUM`

// enPlainKinds are the kinds Lie 7 to 9 sets in italic, and the last branch of
// the grammar is the same head with the italic marked rather than lost. A page
// the model read gives "*Lemma 2.* For 1 <= r <= l" where a page out of the type
// gives "Lemma 2." undecorated, and the marked one is not read by the branch
// above it because the asterisks are still there. Two heads of that volume are
// written this way and none of any other is.
//
// The last branch is one of these opening a passage in small type, where the
// period after the kind is the volume's to leave out and it leaves it out: page
// 147 prints "*Remarks 1) Let P_1, ..., P_l be algebraically independent", with
// the star superscript, the word in italic and nothing between it and the first
// member of the run. The period is what the branch above leans on, so this one
// leans on the star instead, and the star is a mark no sentence opens on. One
// line of the volume is written this way, and left unread it costs the two
// Remarks of § 8 and the three citations to them.
const enPlainKinds = `Lemmas?|Remarks?|Examples?|Scholium`

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
