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

	// runHead is a head that opens a run of statements and carries none of them:
	// the kind in the plural, alone on a line, with the members under it. Empty
	// for a printing that does not set one. See walk.
	runHead *regexp.Regexp

	// runLead is the other way a run is opened: the kind in the plural with the
	// first member on the same line. It is not a head of nothing, so the block it
	// matches is still read as the first member, and it is matched here only so
	// that a no. printing two runs of one kind can be told apart. See walk.
	runLead *regexp.Regexp

	// resume is a paragraph that hands the reader back to the proof of a
	// statement printed further up, which puts that statement in force again.
	// See resumed.
	resume *regexp.Regexp

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
				`|\*\*(` + enKinds + `)(?: (\d+))?(?:\.\*\*|\*\* \([^)]*\)\.)\s+` +
				`|\*\*(` + enCapKinds + `)(?: (\d+))?\.\*\*\s*` +
				`|\*(` + enPlainKinds + `)(?: (\d+))?\.\*\s+` +
				`|` + smallTypeSup + `(` + enPlainKinds + `)(?: (\d+))?\.?\s+)`),
		runHead: regexp.MustCompile(`^\*(` + enRunKinds + `)(?: [a-z][^*]*)?\*(?:\..*:)?\s*$`),
		runLead: regexp.MustCompile(`^\*{0,2}(` + enRunKinds + `)\.\*{0,2}\s+\*{0,2}1\)`),
		resume: regexp.MustCompile(`(?i)^(?:¶\s*)?[^.]{0,80}?\b(?:takes? up|comes? now|concludes?|` +
			`finish(?:es)?|completes?|resumes?|returns? to)\b[^.]{0,40}?\bproof of (?:the )?(` +
			enResumeKinds + `) (\d+)\b`),
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
				smallType + `(` + frKinds + `)(?: (\d+))?\.|(` + frKinds + `)(?: (\d+))?\.` +
				`|\*(` + frKinds + `)(?: (\d+))?\.\*)\s*—\s*`),
		resume: regexp.MustCompile(`(?i)^(?:¶\s*)?[^.]{0,80}?\b(?:terminons|terminer|achevons|achever|` +
			`reprenons|reprendre|concluons|conclure)\b[^.]{0,40}?\b(?:d[ée]monstration|preuve) ` +
			`(?:du|de la|de l['’])\s*(` + frResumeKinds + `) (\d+)\b`),
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

// enKinds are the kinds an English printing states its results in, written the
// way a reading of the page writes them rather than the way the page sets them.
//
// Two printings reach the grammar in this shape and they reach it for different
// reasons. Algebra VIII sets its heads in bold and follows them with an em dash,
// and that is what is on the page. Theory of Sets sets its heads in small
// capitals and follows them with nothing: page 46 prints "THEOREM 1. x = x."
// and page 104 prints "PROPOSITION 4. Let (X_i) be a family of sets", both of
// them small capitals, no dash anywhere on either page. A model reading those
// pages writes the small capitals as bold and lowers the word to ordinary
// capitals, so what arrives is "**Proposition 4.** *Let*". Not one head of the
// 357 pages read of that volume arrives in full capitals, which is why the
// capitals branch below never fires for it.
//
// The dash is the part worth being careful about. 152 of the volume's 257 bold
// heads arrive with an em dash after them and the page under them has no dash on
// it: both models write one, so it is not a habit of one reader, and the two
// images above settle what the page does. The assembler drops the dash with the
// rest of the head, so the invented ones cost nothing downstream, but a branch
// that leans on the dash would read 152 heads of this volume and lose the other
// 105. This branch leans on the bold instead: the period is inside it, and no
// sentence of any volume opens on a bold word with a period in it. Measured over
// every page of the corpus, the shape appears 101 times in Theory of Sets, three
// times in Algebra I to III, where all three are heads too, and nowhere else.
//
// The second half of the branch is the same head with the name of the result
// outside the bold, which is how four of them are written: "**Theorem 1**
// (Zermelo). *Every set* E *can be well-ordered.*". The name is picked out of the
// matched head by headName, exactly as it is for the dashed printings.
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

// enRunKinds are the kinds Theory of Sets gathers into a run under a head that
// carries none of them, which it sets in italic, in the plural, with no period
// and on a line of its own. Page 16 prints "Examples" that way and then "(1)"
// and "(2)" as paragraphs under it, and the volume does this 34 times, 30 of
// them Examples and 4 Remarks. No other volume of the corpus sets a line this
// way at all, so nothing else is being read differently for it.
//
// The head is the whole line, which is what keeps a sentence out: a paragraph
// opening on the word Examples in italic and going on is not this. And the head
// gives no statement of its own, because the book gives it none: it is a lead,
// the way "Remarks. —" is a lead in Algebra VIII, except that this printing
// puts nothing after it on the line.
//
// The head may say what the run is of. Page 89 sets "*Examples of functions*"
// over the (1) and (2) that follow it, and § 3 of chapter II cites the first of
// them as "§ 3, no. 4, Example 1". One line of the volume is written that way
// and the other 34 are the bare word. What follows the kind is held to a
// lower-case word so that a title in italic, which opens on a capital, is not
// read as a run of examples with a name.
//
// The head may also announce the run in a sentence, and page 143 is the one
// place in the corpus that does: "*Examples*. The relations induced by the
// inclusion relation X ⊂ Y on various sets of subsets are of considerable
// importance. Here are some examples :", and then the (1), (2) and (3) of no. 4
// of § 1 of chapter III. Chapter IV cites the third of them, "in accordance with
// the definition given in Chapter III, § 1, no. 4, Example 3", which is the
// preordering finer than another, and read as prose the § had no Example there
// for that sentence to point at.
//
// The colon is what lets the sentence in without letting prose in with it. A
// lead that announces a list ends in one, and a paragraph that merely opens on
// the word Examples and goes on to say something does not, so the line still has
// to be a head and not a statement. Where a printing announces a run without the
// colon the run goes unread, and that fails loudly in R01 rather than quietly in
// the numbering, which is the way round to be wrong.
// The same kinds are what a run lead is of, which is why the one constant
// serves both. Topology I to IV, Topology V to X and Lie 7 to 9 open a run the
// other way, with the kind in the plural and the first member hard after it on
// the same line: page 27 of Topology I to IV sets "Examples. 1) In a discrete
// space (no. 1) the set {x} alone constitutes a fundamental system of
// neighbourhoods of the point x." That line is read as the first member and
// always was, by the branch of pr.head that takes a plural kind and the exNumRE
// that follows it, so runLead changes nothing about how one run is numbered. It
// exists for the no. that prints two.
const enRunKinds = `Examples|Remarks|Lemmas|Scholia`

// enResumeKinds and frResumeKinds are the kinds a paragraph can hand the reader
// back to, which are the kinds a Corollary can be numbered under. A run of
// Remarks is not something a proof is taken up again for.
const enResumeKinds = `Definition|Proposition|Theorem|Lemma|Scholium`

const frResumeKinds = `d[ée]finition|proposition|th[ée]or[èe]me|lemme|scholie`

// frKinds are the words the French printing states its results in. The accents
// are the volume's own and the plurals are too: it sets "Remarques. —" over a
// run of remarks exactly as the English sets "Remarks. —".
const frKinds = `D[ée]finitions?|Propositions?|Th[ée]or[èe]mes?|Lemmes?|Corollaires?|Remarques?|Exemples?|Scholie`

// The last branch of the French grammar is a head with no bold on it, and it
// takes any of the kinds.
//
// The French printing sets Lemme, Remarque, Exemple and Scholie in italic and
// the other four in small capitals, so for a long time this branch listed the
// first four only. The other four reached it in bold, which the branch above
// reads, and an undecorated Proposition was a shape the pages did not produce.
//
// They produce it now. The small capitals survive a reading of the text layer
// and do not always survive a reading of the page image, and 1581 heads across
// 18 French volumes have come back with the bold gone: 247 in Algebra I to III,
// 218 in the French Topology I to IV, 167 in Lie II and III, and 23 in the
// French Algebra VIII, where six § were re-read from the image and the heads on
// them went from "**Proposition 1.**" to "Proposition 1.". Left out, those 1581
// results are prose, they carry no permanent tag, and nothing can cite them.
//
// The em dash is what keeps this honest and it always was. It is the reason the
// four italic kinds could be read undecorated in the first place, since a
// sentence opening on the word Exemple does not carry one, and it holds the
// other four down for the same reason: the printing puts the dash after the
// number of a result and nowhere else, so a paragraph of prose cannot arrive at
// this shape by accident. I read all 1581 rather than trusting that, and every
// one of them states a result.
//
// The English grammar still refuses an undecorated head. That printing sets
// every head in bold and its pages have 8 lines of this shape in the whole
// corpus, which is not enough to say what the shape means there.
//
// The branch after it is the same head with the italic marked rather than lost,
// "*Lemme 1.* — ", which is the other thing a reading of the image does with a
// head it can see the shape of. The English grammar has had this branch for two
// heads of Lie 7 to 9; the French pages carry 248 of them across 17 volumes, and
// six are in the French Algebra VIII, where they are the last six statements
// missing from it once the bold is read.
