// Package refs reads the cross-references Bourbaki writes into his prose and
// turns them into a graph over the permanent tags.
//
// The prose is never rewritten. The graph is a derived layer, so the committed
// text stays a faithful transcription of the page and a reader who wants to
// know what a citation points at asks the manifest rather than a link somebody
// wove into the sentence.
package refs

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// Form is the shape a citation was written in. It is kept on the edge because
// the shapes do not resolve alike, and a resolution rate that did not say which
// shape it was talking about would hide the one that goes wrong.
type Form string

const (
	// FormPage is the shape almost every cross-reference in the chapter takes:
	// a chapter, a page, and usually a statement on it.
	//
	//	VIII, p. 2
	//	VIII, p. 5, Example 3
	//	II, §1, No. 10, p. 212, Proposition 16
	//	Set Theory, III, §2, No. 4, p. 155
	FormPage Form = "page"

	// FormNamed is the same reference with the statement said first, which is
	// what the prose does when the citation is the subject of the sentence
	// rather than an aside in brackets.
	//
	//	Proposition 3 of VIII, p. 30
	//	Proposition 25 of II, §1, No. 13, p. 222
	FormNamed Form = "named"

	// FormAttached is a corollary named by the statement it hangs from, which is
	// how the prose refers to one whenever saying the number alone would not be
	// enough. There are 35 of them in the chapter, 10 with a number and 25
	// without, and no kind other than Corollary is ever named this way.
	//
	//	the corollary of Proposition 2
	//	Corollary 1 of Proposition 4
	//
	// It is the one form that resolves without a lookup, because a corollary's
	// label is built from its parent's and the parent is right there in the
	// sentence.
	FormAttached Form = "attached"

	// FormLocal is a statement of the § being read, named with nothing else.
	//
	//	by Proposition 2
	FormLocal Form = "local"

	// FormFormula is a numbered display.
	//
	//	formula (4)
	FormFormula Form = "formula"

	// FormLocCit is the reference that points at whatever was cited last.
	//
	//	the same then holds for M/(M1 ∩ M2) (loc. cit.)
	//
	// There are 35 in the chapter and none of them resolves. What they point at
	// is the last work cited, which can be several sentences back and in another
	// paragraph, and a rule that guessed at it would produce edges that look
	// right and are not. They are read and counted anyway, because leaving them
	// out of the denominator would put the resolution rate up by a point and a
	// half for no better reason than that the parser could not see them.
	FormLocCit Form = "loc-cit"

	// FormSection is the shape the other printing writes, which says the § and
	// the no. outright where Algebra VIII says a page and leaves the § to be
	// worked out from it.
	//
	//	Chap. VII, §2, no. 3, Prop. 10
	//	§1, no. 1
	//	Algebra, Chap. VIII, §3, no. 2, Th. 1
	//	Th. 2 of §2
	//
	// Lie 7 to 9 cites this way 755 times and by a page 228, so it is not a
	// variant of the page form but the way that volume refers to itself. Algebra
	// VIII writes it not once, and writes none of the abbreviated kinds either,
	// which is what makes reading both printings safe rather than a trade.
	FormSection Form = "section"
)

// Citation is one reference as it was written, before anything is looked up.
// Zero means absent: a citation that names no § has Section 0, and that is
// different from § 0, which Bourbaki does not print.
type Citation struct {
	Raw  string
	Form Form

	Book     string // the English title of another Book of the Éléments, empty for this one
	Chapter  string // "VIII"
	Section  int
	Appendix bool
	Subsec   int // the no.
	Page     int // the page within the chapter, as the book prints it

	Kind   corpus.Kind
	Number int

	// ParentKind and ParentNumber are the statement a corollary hangs from, set
	// only by FormAttached.
	ParentKind   corpus.Kind
	ParentNumber int

	// Line is where in the file it was found, counting from the top of the
	// file, front matter and all.
	Line int

	// Under is the same reference read under the locator the list it stands in
	// wrote on its end, and it is set only where the reference itself wrote
	// none. It is the second reading and not the reading: the resolver looks up
	// the citation as it stands and comes here only when that fails. See under.
	Under *Citation `json:"-"`
}

// The Books of the Éléments as a citation can name them, with the code an edge
// records when the reference leaves the corpus, which is not an error: it is
// the list of what to ingest next.
//
// The book prints the table itself, in "To the Reader" on A VIII.v and A
// VIII.vi: the French title, the English title, the code, and in brackets the
// abbreviation the English printing uses when it cites. So both spellings are
// the book's own and not a guess about how it might write them.
//
//	Théorie des ensembles (Theory of Sets)          E    (Set Theory)
//	Algèbre (Algebra)                               A    (Alg.)
//	Topologie générale (General Topology)           TG   (Gen. Top.)
//	Fonctions d'une variable réelle
//	  (Functions in One Real Variable)              FVR  (FRV)
//	Espaces vectoriels topologiques
//	  (Topological Vector Spaces)                   EVT  (Top. Vect. Sp.)
//	Intégration (Integration)                       INT  (Int.)
//	Algèbre commutative (Commutative Algebra)       AC   (Comm. Alg.)
//	Variétés différentiables et analytiques         VAR
//	Groupes et algèbres de Lie
//	  (Lie Groups and Lie Algebras)                 LIE  (Lie)
//	Théories spectrales                             TS
//	Topologie Algébrique                            TA
//
// The abbreviations are not decoration. Chapter VIII cites another Book by its
// abbreviation 37 times, and without them the citation does not fail, it
// succeeds wrongly: "Gen. Top., III, §6, No. 5, p. 276" comes out as chapter
// III of Algebra, because the chapter in Roman is all the locator can see. So
// every one of those was an edge that named the wrong Book of the Éléments in
// the report that says what to read in next.
//
// The bare codes are here because the exercises use them, and the single
// letters A and E are not, because a letter followed by a Roman numeral is a
// shape that mathematics writes for its own reasons.
//
// A Book is under more than one title where the translations differ, and Lie 7
// to 9 supplies two of those on its own account. It says Spectral Theory six
// times against the seven that say Spectral Theories, and it leaves Algebra in
// French six times, "Exerc. 23 of Algèbre, Chap. X, p. 194", which is the volume
// citing a Book that had not been translated when it went to press. An unread
// title is not a title lost, it is the locator lost with it: the chapter in Roman
// is then all there is to go on, so every one of those pointed at a chapter of
// the Book doing the citing.
var books = []struct{ Name, Code string }{
	{"Set Theory", "E"},
	{"Theory of Sets", "E"},
	{"General Topology", "TG"},
	{"Gen. Top.", "TG"},
	{"Commutative Algebra", "AC"},
	{"Comm. Alg.", "AC"},
	{"AC", "AC"},
	{"Topological Vector Spaces", "EVT"},
	{"Top. Vect. Sp.", "EVT"},
	{"EVT", "EVT"},
	{"Functions of a Real Variable", "FVR"},
	{"Functions in One Real Variable", "FVR"},
	{"FRV", "FVR"},
	{"Differentiable and Analytic Manifolds", "VAR"},
	{"VAR", "VAR"},
	{"Lie Groups and Lie Algebras", "LIE"},
	{"Lie", "LIE"},
	{"Spectral Theories", "TS"},
	{"Spectral Theory", "TS"},
	{"TS", "TS"},
	{"Integration", "INT"},
	{"Int.", "INT"},
	{"Algebra", "A"},
	{"Algèbre", "A"},
	{"Alg.", "A"},
}

// Code is the letter Bourbaki gives a Book, which is what an edge records when
// the citation leaves the corpus. They are the French initials, because that is
// what the Éléments use in every language: the English printing of chapter VIII
// cites Commutative Algebra as AC and not as CA.
func Code(book string) string {
	for _, b := range books {
		if b.Name == book {
			return b.Code
		}
	}
	return ""
}

// bookAlt is the Books in the order the alternation tries them, longest
// spelling of a Book before its shortest, since at a given position the first
// alternative that matches wins. The names are quoted, so the dot in "Comm.
// Alg." is a dot and not any character, which is also what keeps that
// alternative off the front of "Commutative Algebra".
//
// Both locators let the name be marked as italic, because the printing sets the
// title of another Book that way and a page the model read keeps the marks:
// "*General Topology*, Chap. III, §4, no. 2" seven times against the same
// citation undecorated. Left unread the name is not what is lost, the whole
// locator is, and "by applying Prop. 9 of *Algebra*, Chap. IV, §2, no. 3" came
// out as a bare Prop. 9 hunted for in the § of Lie the sentence stands in.
var bookAlt = bookNames()

func bookNames() string {
	out := make([]string, len(books))
	for i, b := range books {
		out[i] = regexp.QuoteMeta(b.Name)
	}
	return strings.Join(out, "|")
}

// part is the piece of a statement being pointed at, which the book writes into
// the reference and which has to be read past rather than read: a part carries
// no tag of its own, and a reference that stops at the number comes apart into
// a bare statement and a page, of which the bare half is what the resolver
// guesses at. Both shapes the chapter writes are here, "Corollary 1, c)" and
// "Proposition 16, (ii)".
//
// The comma is optional because the other printing leaves it out: Lie 7 to 9
// writes "Chap. VII, §5, no. 2, Cor. 1 (ii) of Prop. 4" where Algebra VIII would
// write "Corollary 1, (ii)". Without it the reference stops at "Cor. 1", the
// "of Prop. 4" that says which corollary is meant is thrown away, and what is
// left is hunted for in the § doing the citing.
const part = `(?:,?\s*(?:[a-z]\)|\([ivx]+\)))?`

// run is a second statement written into a reference by its number alone,
// sharing the kind said in front of it and the locator said after: "Cor. 1 and 3
// of Th. 2", "Prop. 7 and 8 of §7", "By Cor. 3 and 4 of Prop. 8". The corpus
// writes four and they are read as two references each, which is what they are.
//
// It has to be read here rather than left to fall through, because what falls
// through is not half the reference, it is neither half: the "and 8 of §7" is
// invisible, and the "Prop. 7" in front of it, with the locator now out of its
// reach, is a bare statement hunted for in the § the sentence stands in. Of the
// four, one was reported as a Proposition 7 that § 9 does not have, one as a
// Corollary 1 that § 5 has four of, and the two that were not reported resolved
// against the wrong § in silence.
const run = `(?:\s+and\s+\d+)?`

// runMember picks the number of the second statement back out of a reference
// that was read whole. It is anchored on the kind, which every form that reads a
// run begins with, so that the numbers of a locator further along the reference,
// "§6, no. 2 and 3", cannot be taken for a run of statements.
var runMember = regexp.MustCompile(`^(?:` + kindAlt + `|[Cc]orollary)\s*(\d+)\s+and\s+(\d+)\b`)

const kindAlt = `Proposition|Theorem|Corollary|Lemma|Definition|Remark|Example|Exercise|Scholium|` +
	`Prop\.|Th\.|Cor\.|Def\.|Exerc\.`

// locCit is the phrase that stands for the work cited last, with the italic the
// printing sets it in where a page the model read kept the marks. One of the 108
// is written "*loc. cit.*", and unread the phrase is not what is lost: the
// statement after it, "*loc. cit.*, Cor. 2 of Th. 4", is left behind as a
// corollary of a Theorem 4 that the § doing the citing does not have.
const locCit = `\*?loc\. cit\.\*?`

// kindAbbrev is the short spelling of a kind against the word it stands for.
//
// Lie 7 to 9 abbreviates where Algebra VIII writes the word out: 840 Prop., 315
// Th., 311 Cor., 273 Exerc. and 52 Def., against none of any of them in Algebra
// VIII. Lemma, Remark and Example it writes in full, which is why they are not
// here. So this is a second printing of the same citations rather than a second
// meaning, and reading it cannot change what Algebra VIII's citations point at.
var kindAbbrev = map[string]string{
	"Prop.":  "Proposition",
	"Th.":    "Theorem",
	"Cor.":   "Corollary",
	"Def.":   "Definition",
	"Exerc.": "Exercise",
}

// locator is the part of a citation that finds the page: an optional Book, a
// chapter in Roman, and then any of § no. Appendix before the page itself.
// Every one of the middle parts is optional, because the chapter shows all the
// combinations in use, and the page is not, because it is the anchor the whole
// scheme is built on.
//
// The no. is written either way round. "No. 4" is the English printing and
// `n$^o4$` is the French, which the printing keeps whenever it cites a volume
// that has not been translated: Algebra X, Spectral Theories, Topological
// Vector Spaces, and chapter VIII of Commutative Algebra. There are 22 of them
// and the kind that follows is in French too, "p. 9, exemple 1". The kind is
// left unread, because every one of these leaves the corpus and a reference
// that leaves the corpus is answered by its locator alone; the locator is not,
// because without it the whole citation is invisible. That is worse than
// unresolved: "Proposition 6 of X, §8, n$^o4$, p. 140" came apart into a bare
// Proposition 6, which § 8 has, and the edge pointed at it.
//
// The optional dollar is a transcription artefact and not a form the book has.
// Six pages of chapter VIII run the page number into the maths that follows it,
// so "p. 213, θ is bijective" comes out as `p. $213,\theta$`. Six references is
// not much, but a locator that fails leaves a bare "Corollary 2" behind, and a
// bare kind and number is the one thing the resolver is willing to guess at, so
// each of these turns into a confident edge pointing at the wrong statement of
// the wrong Book. Tolerating the dollar is cheaper than the guess.
// The word Chap. in front of the chapter is the other printing's, and it writes
// the whole locator no other way: Algebra VIII says "VIII, p. 30" and Lie 7 to 9
// says "Chap. X, p. 194". Twelve references name a chapter and a page that way,
// all of them to a Book outside the corpus, and unread the chapter is not what is
// lost. The name in front of it goes too, since the Book is only read as part of
// a locator, so "Exerc. 23 of Algèbre, Chap. X, p. 194" came out as an Exercise
// 23 of the § of chapter IX the sentence stands in, which has twelve.
// chapWord is how a printing writes the word in front of a chapter numeral.
// Lie 7 to 9 and the French volumes write "Chap. VII" and Theory of Sets writes
// it out, "Chapter II, § 4, no. 6, Proposition 7". Read one way only, 121
// references of the corpus lost the chapter they name and were resolved against
// the § doing the citing, which is how § 1 of chapter III came to cite a
// Proposition 7 of a § that has three.
const chapWord = `(?:Chap\.|Chapter)\s*`

var locator = `(?:\*?(` + bookAlt + `)\*?,\s*)?(?:` + chapWord + `)?\b([IVX]+)(?:,\s*§\s*(\d+))?(,\s*Appendix)?` +
	`(?:,\s*(?:[Nn]o\.\s*|n\$\^o)(\d+)\$?)?,\s*p\.\s*\$?(\d+)`

// sectionLocator is the locator of the other printing, which finds the § itself
// instead of finding the page the § happens to be printed on: an optional Book,
// an optional chapter written out as "Chap. VII", the § and then the no.
//
// The chapter is optional because a volume of three chapters refers to its own
// as often as to the others, and "§ 2, no. 3" means the § of the chapter the
// sentence is standing in. The no. is optional because a reference to a whole no.
// is common and a reference to a whole § is not rare.
//
// It carries no page, and that is the point: nothing in this locator has to be
// matched against a page map, so a § of a chapter the corpus has not read still
// says exactly which § it means, and a citation to Lie chapter VI is out of the
// corpus rather than unresolved.
//
// The dollar after the § is the same transcription artefact the page locator
// tolerates, and it is here for the same reason: five references run the § number
// into the maths beside it, "by Prop. 5 of §$7, \mathfrak{g}$ is nilpotent", and a
// locator that stops at the § leaves a bare Prop. 5 behind for the resolver to
// hunt for in the § doing the citing.
// stray is a span of maths the extractor has glued to the word in front of it,
// which is stepped over rather than read.
//
// Eight references have a piece of a display standing in the middle of them,
// "Chap. III,$^{\alpha\in B}$ §3, no. 11, Cor. 3 of Prop. 41", "Chap. VI,$\sum$
// §1, no. 10" and "by Prop. 2 (v) of$\sum$ §3, no. 1": the sum sign, or the limit
// under one, has landed on the line of prose beside the display it belongs to.
// None of it is part of the reference, so nothing is lost by skipping it, and a
// locator that stops in front of it leaves the § to be read as a § of the chapter
// the sentence stands in, which had chapter VIII reported as missing a
// Proposition 41 of chapter III and a Proposition 29 of chapter VI.
//
// After an "of" the span is only read as a stray one when it is glued to the
// word, since that is what tells it from maths the sentence is made of: prose
// sets a space in front of an inline span, "the Weyl group of $W_G(T) ($§4, no.
// 2)", and ten of the eleven spans standing between an "of" and a § are that.
// Inside the locator the position does the same work, because nothing belonging
// to the sentence stands between a chapter and its §. The span is held short
// either way, so that a reference cannot reach across a formula into a § named
// further down the line.
const stray = `\$[^$]{1,20}\$`

var sectionLocator = `(?:\*?(` + bookAlt + `)\*?,\s*)?(?:` + chapWord + `([IVX]+),\s*(?:` + stray + `\s*)?)?§\s*\$?\s*(\d+)` +
	`(?:,\s*[Nn]os?\.\s*(\d+))?`

// The forms, in the order they are tried. Order is the whole of the
// difference between reading "Proposition 25 of II, §1, No. 13, p. 222" as one
// reference to another chapter and reading it as two, a Proposition of the § in
// hand and a page of chapter II that nothing points at. It is also what keeps
// "Corollary 1 of Proposition 4" from coming out as two local references, one
// of them to a number the § has five of.
var (
	// The bracket is the same form with the locator in parentheses instead of
	// after "of": "Remark 1 (VIII, p. 97)" against "Remark 1 of VIII, p. 97".
	// The chapter writes it both ways, 346 times with "of" and 22 with the
	// bracket, and the bracket has to be here rather than left to fall through,
	// because falling through reads it as two references, a bare Remark 1 of the
	// § in hand and a page of chapter VIII that nothing points at.
	//
	// "Corollary 1, c) of VIII, p. 152" points at part c) of that Corollary and
	// the chapter writes 29 of them, which is what part is for.
	namedRE = regexp.MustCompile(`\b(` + kindAlt + `)\s+(\d+)` + run + part + `\s+(?:of\s+|\()` + locator)
	// The locator on the end is what says which § the parent stands in, and
	// leaving it out was the single largest fault in the graph. "the corollary
	// of Proposition 12 of II, §1, No. 8, p. 209" came apart into a corollary
	// hunted for in chapter VIII, where there is no Proposition 12 to hang it
	// on, and a page of chapter II that nothing pointed at. Read whole it is one
	// reference and it leaves the corpus. The same shape with this chapter's own
	// pages, "Corollary 1 of Proposition 4 (VIII, p. 83)", is a reference from
	// § 11 to a statement of § 5, and resolving it in § 11 could only ever fail.
	//
	// The abbreviation is read here too. Lie 7 to 9 writes "Cor. 1 of Prop. 4"
	// 233 times and "Corollary 1 of Proposition 4" not once, so without it every
	// one of those came apart into a bare Cor. 1 and a bare Prop. 4, both hunted
	// for in the § doing the citing. That is the reading that fails quietly: a §
	// with a Corollary 1 of its own answers it, and the answer is wrong.
	attachedRE = regexp.MustCompile(`\b(?:[Cc]orollary|Cor\.)\s*(\d*)` + run + part +
		`\s+(?:of|to)\s+(` + kindAlt + `)\s+(\d+)(?:\s*(?:of\s+|,\s*|\()` + locator + `\)?)?`)
	// The same reference with the locator written first, which is what the
	// brackets do: "(II, §1, No. 9, p. 210, Corollary of Proposition 13)" and
	// "(VIII, p. 82, Corollary 1 of Theorem 2)". This has to be tried before the
	// page form, which would take the head of it and hand the tail to the
	// resolver as a bare corollary: the second of those is a Corollary 1 that
	// § 5 prints two of, and dropping "of Theorem 2" throws away the one thing
	// in the sentence that says which.
	leadRE = regexp.MustCompile(locator + `,\s*[Cc]orollary\s*(\d*)` + part + `\s+(?:of|to)\s+(` + kindAlt + `)\s+(\d+)`)
	// The three shapes of the other printing, which have to be tried before the
	// page form and before the local one for the same reason every other form
	// does: a locator that is not read whole leaves a bare kind and number
	// behind, and a bare kind and number is the one thing the resolver will
	// guess at. "Chap. II, §6, no. 1, Remark 4" read as a bare Remark 4 is
	// hunted for in the § doing the citing, which is a different chapter of a
	// different volume, and there it either fails or, worse, succeeds.
	//
	// The attached one comes before the plain one because both begin at the §
	// and the plain one would stop at "Cor. 1" and throw away the "of Prop. 4"
	// that says which corollary is meant.
	sectionAttachedRE = regexp.MustCompile(sectionLocator +
		`,\s*(?:[Cc]orollary|Cor\.)\s*(\d*)` + part + `\s+(?:of|to)\s+(` + kindAlt + `)\s*(\d+)`)
	// The same attached reference with the § written last instead of first, "Cor.
	// 3 of Th. 2 of Chap. VII, §2, no. 4", which the other printing writes 16
	// times. It goes before the attached form, whose locator is a page and which
	// would otherwise take the head of this and leave the § unread, and before
	// the named one, which would take "Th. 2 of Chap. VII, §2, no. 4" and drop
	// the corollary that is what the sentence is pointing at.
	//
	// The § is put in brackets as well as after "of", "by Cor. 1 of Prop. 4 (§1,
	// no. 4)", which is the same difference the page form makes between "of VIII,
	// p. 30" and "(VIII, p. 30)". The volume writes it twice.
	sectionParentRE = regexp.MustCompile(`\b(?:[Cc]orollary|Cor\.)\s*(\d*)` + part +
		`\s+(?:of|to)\s+(` + kindAlt + `)\s*(\d+)(?:\s+of\s+|\s*\()` + sectionLocator + `\)?`)
	// The kind said first, "Th. 2 of §2" and "Prop. 7 of §2", which the prose
	// writes 60 times.
	//
	// The bracket is the same reference with the § in parentheses instead of after
	// "of", "Th. 1 (§6, no. 2)" and "Prop. 4 (§1, no. 4)", which is the difference
	// the page form already makes between "of VIII, p. 30" and "(VIII, p. 30)" and
	// the attached form between "of Prop. 4 (§1, no. 4)" and "of Prop. 4 of §1, no.
	// 4". Four places write it, and left to fall through each came apart into a
	// bare statement hunted for in the § doing the citing and a bare § that nothing
	// pointed at.
	sectionNamedRE = regexp.MustCompile(`\b(` + kindAlt + `)\s*(\d+)` + run + part +
		`(?:\s+of(?:\s+|` + stray + `\s*)|\s*\()` + sectionLocator + `\)?`)
	// The statement on the end is sometimes not the thing being pointed at.
	// "General Topology, Chap. VII, §1, no. 3, text preceding Prop. 8" points at a
	// passage the printing gives no number to, and names the Proposition only to
	// say where in the no. to look. So the phrase is read and the kind and number
	// are then dropped, since carrying them would anchor the edge on a statement
	// the sentence is deliberately pointing away from. Read no further than the
	// no., the "Prop. 8" left over is a bare local citation hunted for in the §
	// doing the citing, which is a different chapter of a different Book. Two
	// references are written this way.
	sectionRE = regexp.MustCompile(sectionLocator +
		`(?:,\s*(text\s+(?:preceding|following)\s+)?(` + kindAlt + `)\s*(\d+))?`)

	pageRE    = regexp.MustCompile(locator + `(?:,\s*(` + kindAlt + `)\s*(\d+))?`)
	formulaRE = regexp.MustCompile(`formula\s*\((\d+)\)`)
	// A statement of whatever was cited last: "follow the proof of Proposition
	// 14 of loc. cit.". This has to be read whole and before the local form,
	// which takes the head of it and hands the resolver a bare Proposition 14 to
	// look for in the § the sentence stands in. The chapter writes it twice and
	// neither reading survives being checked. One is an exercise of § 2 whose
	// loc. cit. is Algebra II, and § 2 has no Proposition 14, so it was reported
	// as a hole in the corpus that is not there. The other did resolve, and only
	// because the work cited last happens to be the appendix the sentence is
	// already standing in, which is luck and not a rule.
	namedLocCitRE = regexp.MustCompile(`(\b(?:` + kindAlt + `)\s+\d+` + part + `\s+of\s+` + locCit + `)`)
	// The same reference with the statement said after instead of before, which
	// is how the other printing writes it: "loc. cit., Cor. 6", "loc. cit., no. 7,
	// Prop. 25", "cf. loc. cit., Prop. 49". Twenty are written this way. Stopping
	// at the period leaves the tail behind as a bare Prop. 25, and a bare kind and
	// number is hunted for in the § the sentence stands in, which is not the work
	// the loc. cit. names. What the whole of it points at is still unknown, since
	// the resolver does not carry the last work cited, so it is a loc. cit. and is
	// counted as one rather than reported as a statement the corpus is missing.
	//
	// The statement is taken with its parent where it has one, "loc. cit., Cor. 1
	// of Prop. 3", because half of an attached reference is worse than none: the
	// corollary would go with the loc. cit. and the Proposition would be left
	// behind on its own, and § 10 of chapter VIII, which has no Proposition at
	// all, was reported as missing the Proposition 3 of chapter VII the sentence
	// is pointing at.
	locCitRE = regexp.MustCompile(`(` + locCit + `)(?:,\s*[Nn]os?\.\s*\d+)?` +
		`(?:,\s*(?:` + kindAlt + `)\s*\d+(?:` + part + `\s+(?:of|to)\s+(?:` + kindAlt + `)\s*\d+)?)?`)
	// A second page in the same bracket does not repeat the chapter: the book
	// writes "(VIII, p. 190, Corollary and p. 401, Corollary 1)" and means
	// chapter VIII both times. Five references of the chapter are written this
	// way. The kind and number are required here, unlike in the full page form,
	// so that a bare "p. 12" in prose is not read as a citation on the strength
	// of a chapter named a sentence earlier.
	contRE = regexp.MustCompile(`p\.\s*\$?(\d+),\s*(` + kindAlt + `)\s*(\d+)`)
	// The other printing does the same thing with a no. instead of a page:
	// "(Chap. VIII, §2, no. 2, Prop. 1 and no. 4, Lemma 3)" and "By Chap. III, §3,
	// no. 8, Cor. 2 of Prop. 29, and no. 10, Prop. 36" name the § once and the no.
	// twice. Five references are written this way. The "and" is required, where the
	// page form makes do with the kind and number, because "no. 4" is a thing the
	// prose says on its own account far more often than it says "p. 4", and the
	// word is what marks a second item in the same bracket rather than a fresh
	// mention.
	sectionContRE = regexp.MustCompile(`\band\s+[Nn]os?\.\s*(\d+),\s*(` + kindAlt + `)\s*(\d+)`)
	// A no. with no § in front of it, which is how a § refers to itself: "(no. 2,
	// Remark 2)", "Part (i) follows from no. 1, Lemma 1", "the elements defined in
	// no. 6, Def. 1". It is the section form with everything that would have been
	// repeated left out, and the volume writes it 39 times.
	//
	// The no. is the whole of what it adds, and it is what these references need:
	// 19 of the 39 name a Remark or a Corollary, and a § numbers both of those
	// afresh under every no., so the number alone does not say which one is meant.
	// Read no further than the kind, "(cf. no. 2, Remark 1)" is a bare Remark 1 of
	// a § that prints six of them, and six candidates with nothing to choose
	// between them is what was reported.
	//
	// The chapter is read where one is written, because the no. of another chapter
	// is not a no. of this §. The volume writes one, "(Chap. I, no. 3, Prop.
	// $4d))$", and it is not read at all, since its number went into the maths
	// beside it; the clause earns its place by making sure that a chapter can
	// never be dropped and the rest of the reference resolved quietly against the §
	// doing the citing.
	noRE = regexp.MustCompile(`(?:\*?(` + bookAlt + `)\*?,\s*)?(?:` + chapWord + `([IVX]+),\s*)?` +
		`\b[Nn]os?\.\s*(\d+),\s*(` + kindAlt + `)\s*(\d+)`)
	// A statement of a work that is not one of the Éléments, standing where that
	// work's own pages are given: "(B. KOSTANT, Lie group representations on
	// polynomial rings, Amer. J. Math., Vol. LXXXV (1963), pp. 327-404, Th. 10 and
	// 15)". It is read so that it is not read as anything else, and nothing is
	// made of it.
	//
	// The page range is what marks the work as another author's. The Éléments cite
	// themselves by a single page and never by a span, and all 22 spans the corpus
	// writes are in a footnote or an exercise naming a paper. One of them carries a
	// statement, and the § doing the citing was reported as missing a Theorem 10
	// that belongs to Kostant.
	outsideRE = regexp.MustCompile(`\bpp\.\s*\d+-\d+,\s*(?:` + kindAlt + `)\s*\d+` + run)
	localRE   = regexp.MustCompile(`\b(` + kindAlt + `)\s+(\d+)\b`)

	// A statement heading is not a citation of itself, and neither is the bold
	// lead a statement is printed with. The lead should not be in a body at all,
	// since assembly turns it into the heading, and where one survives it is an
	// extraction fault rather than a reference.
	headingRE = regexp.MustCompile(`^#`)
	boldLead  = regexp.MustCompile(`\*\*(` + kindAlt + `)\s*\d*\.?\*\*`)
)

// scanner is the alternation of them all, tried in that order. Go's regexp
// prefers the earliest match and, among matches at the same place, the first
// alternative that matches, which is exactly the precedence wanted here.
var forms = []*regexp.Regexp{namedRE, sectionParentRE, attachedRE, leadRE, sectionNamedRE, sectionAttachedRE,
	sectionRE, pageRE, formulaRE, namedLocCitRE, locCitRE, contRE, sectionContRE, noRE, outsideRE, localRE}

var scanner = regexp.MustCompile(alternation(forms))

func alternation(res []*regexp.Regexp) string {
	parts := make([]string, len(res))
	for i, re := range res {
		parts[i] = `(?:` + re.String() + `)`
	}
	return strings.Join(parts, `|`)
}

// The group each form's submatches start at, counted from the forms themselves
// rather than written down. They were written down once, and every one of them
// had to be recounted by hand the day a form grew a group, which is the kind of
// arithmetic that is wrong long before anybody notices: a citation would come
// back as the wrong form with the wrong fields and still look like a citation.
var (
	atNamed       = 1
	atSecParent   = atNamed + namedRE.NumSubexp()
	atAttached    = atSecParent + sectionParentRE.NumSubexp()
	atLead        = atAttached + attachedRE.NumSubexp()
	atSecNamed    = atLead + leadRE.NumSubexp()
	atSecAttached = atSecNamed + sectionNamedRE.NumSubexp()
	atSection     = atSecAttached + sectionAttachedRE.NumSubexp()
	atPage        = atSection + sectionRE.NumSubexp()
	atFormula     = atPage + pageRE.NumSubexp()
	atNamedLoc    = atFormula + formulaRE.NumSubexp()
	atLocCit      = atNamedLoc + namedLocCitRE.NumSubexp()
	atCont        = atLocCit + locCitRE.NumSubexp()
	atSecCont     = atCont + contRE.NumSubexp()
	atNo          = atSecCont + sectionContRE.NumSubexp()
	atOutside     = atNo + noRE.NumSubexp()
	atLocal       = atOutside + outsideRE.NumSubexp()
)

// Parse reads every citation out of one file's body.
//
// line0 is the line the body starts on, so that a citation can be reported at a
// line of the file an editor will open rather than a line of a string.
func Parse(body string, line0 int) []Citation {
	var out []Citation
	for i, line := range strings.Split(body, "\n") {
		if headingRE.MatchString(line) {
			continue
		}
		line = boldLead.ReplaceAllStringFunc(line, blank)
		// last is the Book and chapter the line has named so far, which is what a
		// second page in the same bracket leaves out.
		var last Citation
		// prev is the citation just read and prevEnd is where it ended, which is
		// what a run of statements written under one locator continues.
		var prev Citation
		prevEnd := -1
		// bare is the members of the run in hand that were written with no
		// locator, waiting for one to be written on the end of the run.
		var bare []int
		for _, loc := range scanner.FindAllStringSubmatchIndex(line, -1) {
			m := groups(line, loc)
			c, ok := citation(m, last)
			if !ok {
				continue
			}
			c.Line = line0 + i
			if prevEnd >= 0 {
				carry(&c, prev, line[prevEnd:loc[0]])
			}
			if prevEnd < 0 || !runConnector.MatchString(line[prevEnd:loc[0]]) {
				bare = bare[:0]
			}
			if written(c) {
				under(out, bare, c)
				bare = bare[:0]
			} else {
				bare = append(bare, len(out))
			}
			out = append(out, c)
			// The second member of a run is the same reference with another
			// number, so it is made from the one that was read rather than
			// matched again. The whole of the run is its raw text, because that
			// is what a reader would have to look at to see it.
			if r := runMember.FindStringSubmatch(c.Raw); r != nil && atoi(r[1]) == c.Number {
				second := c
				second.Number = atoi(r[2])
				out = append(out, second)
			}
			prev, prevEnd = c, loc[1]
			if c.Chapter != "" {
				last = c
			}
		}
	}
	return out
}

// blank keeps the length of what it replaces, so that nothing downstream has to
// care that a span was taken out.
func blank(s string) string { return strings.Repeat(" ", len(s)) }

// groups is one match's submatches as strings, which is what citation reads.
// They are taken from the indices rather than by matching a second time, so that
// where the match begins and ends is known as well as what it said.
func groups(s string, loc []int) []string {
	m := make([]string, len(loc)/2)
	for i := range m {
		if loc[2*i] >= 0 {
			m[i] = s[loc[2*i]:loc[2*i+1]]
		}
	}
	return m
}

// runConnector is all that may stand between a citation and the one that
// continues it: the comma or the "and" a list is written with, and nothing else.
var runConnector = regexp.MustCompile(`^(?:,|\s+and\b|,\s+and\b)\s*$`)

// carry gives a statement that was written with no locator the locator of the
// citation it was written under.
//
// A list of statements says where they are once and then names them one after
// another: "General Topology, Chap. VIII, §1, no. 4, Prop. 3, Prop. 4 and Remark
// 4" and "(Use §2, Th. 2, Prop. 10 and Cor. 2 of Th. 1.)". Every member after the
// first is left with nothing to say where it is, and a statement with no locator
// is looked for in the § the sentence stands in, which is not where it is and in
// the first of those is not even the right Book.
//
// What makes this safe is that the run has to be unbroken. The continuation
// begins exactly where the citation before it ended, with nothing between them
// but the comma or the "and", so a statement named after a few words of prose is
// not one and is left alone. The locator is copied whole, so a third member
// continues the second the same way the second continued the first.
//
// The other half of the same habit is a second § named under the same Book: the
// work is written once and the §s after it are written bare, "Spectral Theory,
// Chap. II, §2, no. 2, Cor. 2 of Prop. 4 and §1, no. 9, Cor. 2 of Prop. 11" and
// "Algebra, Chap. III, §7, no. 4, Prop. 7 and §11, no. 10". A bare § is read as
// a § of the chapter the sentence stands in, so every one of those was resolved
// against the wrong chapter of the wrong Book, and two of them were reported as
// statements chapters VIII and IX are missing. Twelve places continue a citation
// with "and §", and the four whose first member names a Book or a chapter are
// these; the other eight name none, so there is nothing to carry and the reading
// is unchanged.
func carry(c *Citation, prev Citation, gap string) {
	if !runConnector.MatchString(gap) {
		return
	}
	if c.Book == "" && c.Chapter == "" && c.Section != 0 && c.Page == 0 &&
		(prev.Book != "" || prev.Chapter != "") {
		c.Book, c.Chapter = prev.Book, prev.Chapter
		return
	}
	lift(c, prev)
}

// lift gives a statement that was written with no locator the locator of
// another, and says whether there was one to give. It is the whole of what a
// member of a list takes from the member it is written beside, in either
// direction, and it is a copy and not a merge: a citation that named anything of
// its own is left exactly as it was written.
func lift(c *Citation, from Citation) bool {
	if c.Form != FormLocal && c.Form != FormAttached {
		return false
	}
	if c.Book != "" || c.Chapter != "" || c.Section != 0 || c.Subsec != 0 || c.Page != 0 {
		return false
	}
	if from.Section == 0 && from.Page == 0 {
		return false
	}
	c.Book, c.Chapter, c.Appendix = from.Book, from.Chapter, from.Appendix
	c.Section, c.Subsec, c.Page = from.Section, from.Subsec, from.Page
	// An attached citation reads its own locator and is still attached. A bare
	// statement was a local one only for want of a locator, and now that it has
	// one it is read the way the locator is written, by the § or by the page.
	if c.Form == FormLocal {
		if c.Page != 0 {
			c.Form = FormPage
		} else {
			c.Form = FormSection
		}
	}
	return true
}

// under records, against every member of an unbroken run that was written with
// no locator, the reading it would have if the locator on the end of the run
// governed the whole of it.
//
// A list writes its locator once, and the printing puts it at either end. "(Use
// §2, Th. 2, Prop. 10 and Cor. 2 of Th. 1.)" puts it first and carry answers
// that one. The other half write it last: "(Use Exerc. 4 and Exerc. 20 of §4.)",
// "(Use Prop. 12 and Prop. 1 (ii) of §6.)" and "Cor. 2 of Th. 1 and Prop. 4 (§6,
// no. 2 and 3)". Nine places do, and in every one of them the members in front
// of the locator are left bare and are hunted for in the § the sentence stands
// in.
//
// This is where the two ends differ, and it is why the second reading is
// recorded beside the first rather than put in its place. A locator written
// first governs what comes after it and there is nothing else it could be doing.
// A locator written last is ambiguous, and the volume means it both ways: "use
// Exerc. 10 and Exerc. 4 of Chap. VI, §4" stands in an exercise that has already
// cited Exerc. 10 twice as one of its own §, so there the first member is local
// and only the second is of chapter VI. Handing the locator back would be wrong
// in that one and right in the rest, and the sentence says nothing that tells
// them apart.
//
// So the reference as written is what is looked up, and the run's locator is
// tried only where that fails. A member that answers in the § doing the citing
// is left alone, and one that answers nowhere is read as the printing's other
// habit rather than reported as a statement the corpus is missing.
func under(out []Citation, bare []int, from Citation) {
	for _, i := range bare {
		u := out[i]
		if lift(&u, from) {
			out[i].Under = &u
		}
	}
}

// written says whether a citation named any part of a locator of its own, which
// is what makes it a member a run has nothing to give.
func written(c Citation) bool {
	return c.Book != "" || c.Chapter != "" || c.Section != 0 || c.Subsec != 0 || c.Page != 0
}

// at is the last citation on the line that named a chapter, which is all the
// continuation form has to go on.
//
// citation reads one match of the scanner. The submatch groups are the five
// forms end to end, so the first non-empty run says which form matched. Each
// form is tested on a group it cannot match without, which is not always the
// first: a page citation need not name a Book and an attached citation need not
// number the corollary.
func citation(m []string, at Citation) (Citation, bool) {
	named, attached, lead := atNamed, atAttached, atLead
	secParent := atSecParent
	secNamed, secAttached, section := atSecNamed, atSecAttached, atSection
	page, formula, cont, local := atPage, atFormula, atCont, atLocal
	secCont, no := atSecCont, atNo
	namedLoc, locCit := atNamedLoc, atLocCit
	switch {
	case m[named] != "":
		c := Citation{Raw: m[0], Form: FormNamed}
		c.Kind, c.Number = kind(m[named]), atoi(m[named+1])
		locate(&c, m, named+2)
		return c, true
	case m[secParent+1] != "":
		c := Citation{Raw: m[0], Form: FormAttached, Kind: corpus.KindCorollary,
			Number:     atoi(m[secParent]),
			ParentKind: kind(m[secParent+1]), ParentNumber: atoi(m[secParent+2])}
		locateSection(&c, m, secParent+3)
		return c, true
	case m[attached+1] != "":
		// Number stays 0 when the prose did not write one, because a corollary
		// the book left unnumbered is not the same statement as its Corollary 1
		// and is not looked up the same way.
		c := Citation{Raw: m[0], Form: FormAttached, Kind: corpus.KindCorollary, Number: atoi(m[attached]),
			ParentKind: kind(m[attached+1]), ParentNumber: atoi(m[attached+2])}
		locate(&c, m, attached+3)
		return c, true
	case m[lead+1] != "":
		c := Citation{Raw: m[0], Form: FormAttached, Kind: corpus.KindCorollary, Number: atoi(m[lead+6]),
			ParentKind: kind(m[lead+7]), ParentNumber: atoi(m[lead+8])}
		locate(&c, m, lead)
		return c, true
	case m[secNamed] != "":
		c := Citation{Raw: m[0], Form: FormSection}
		c.Kind, c.Number = kind(m[secNamed]), atoi(m[secNamed+1])
		locateSection(&c, m, secNamed+2)
		return c, true
	case m[secAttached+5] != "":
		c := Citation{Raw: m[0], Form: FormAttached, Kind: corpus.KindCorollary,
			Number:     atoi(m[secAttached+4]),
			ParentKind: kind(m[secAttached+5]), ParentNumber: atoi(m[secAttached+6])}
		locateSection(&c, m, secAttached)
		return c, true
	case m[section+2] != "":
		// A § named with nothing else around it is not a citation. The prose says
		// "the results of § 2" often enough, and reading that as a reference would
		// put an edge on every mention of a § while saying nothing a reader of the
		// graph did not already have. What makes it one is any of the parts that
		// point somewhere: a Book, a chapter, a no., or a statement on the end.
		if m[section] == "" && m[section+1] == "" && m[section+3] == "" && m[section+5] == "" {
			return Citation{}, false
		}
		c := Citation{Raw: m[0], Form: FormSection}
		locateSection(&c, m, section)
		// The statement is taken only when the reference is pointing at it, and
		// not when it is pointing at the text either side of it.
		if m[section+4] == "" && m[section+5] != "" {
			c.Kind, c.Number = kind(m[section+5]), atoi(m[section+6])
		}
		return c, true
	case m[page+1] != "":
		c := Citation{Raw: m[0], Form: FormPage}
		locate(&c, m, page)
		if m[page+6] != "" {
			c.Kind, c.Number = kind(m[page+6]), atoi(m[page+7])
		}
		return c, true
	case m[formula] != "":
		return Citation{Raw: m[0], Form: FormFormula, Kind: corpus.KindEquation, Number: atoi(m[formula])}, true
	case m[namedLoc] != "", m[locCit] != "":
		return Citation{Raw: m[0], Form: FormLocCit}, true
	case m[cont] != "":
		// The chapter is whatever the last citation on this line named. Without
		// one there is nothing to resolve against and the reference is dropped
		// rather than guessed at, which in chapter VIII never happens.
		if at.Chapter == "" {
			return Citation{}, false
		}
		c := Citation{Raw: m[0], Form: FormPage, Book: at.Book, Chapter: at.Chapter, Page: atoi(m[cont])}
		c.Kind, c.Number = kind(m[cont+1]), atoi(m[cont+2])
		return c, true
	case m[secCont] != "":
		// The § is whatever the last citation on this line named, and a citation
		// that named no § is nothing to carry: a page reference has a chapter and
		// no §, and reading a no. against it would produce a § 0 that the book
		// does not print.
		if at.Chapter == "" || at.Section == 0 {
			return Citation{}, false
		}
		c := Citation{Raw: m[0], Form: FormSection, Book: at.Book, Chapter: at.Chapter,
			Section: at.Section, Subsec: atoi(m[secCont])}
		c.Kind, c.Number = kind(m[secCont+1]), atoi(m[secCont+2])
		return c, true
	case m[no+2] != "":
		// A no. and nothing else means a no. of the § the sentence stands in, and
		// that is a local reference with one more thing said about it. Where a
		// chapter is named it is a reference of the other printing that has left
		// the § out, and it is read as one so that the chapter is not thrown away.
		c := Citation{Raw: m[0], Form: FormLocal, Book: m[no], Chapter: m[no+1], Subsec: atoi(m[no+2]),
			Kind: kind(m[no+3]), Number: atoi(m[no+4])}
		if c.Book != "" || c.Chapter != "" {
			c.Form = FormSection
		}
		return c, true
	case m[local] != "":
		return Citation{Raw: m[0], Form: FormLocal, Kind: kind(m[local]), Number: atoi(m[local+1])}, true
	}
	return Citation{}, false
}

// locate reads the six groups of a locator into the citation. The order is the
// one the pattern writes them in, Book, chapter, §, Appendix, no., page, and it
// is read in one place so that a form which grows a locator does not also grow
// its own way of misreading one.
func locate(c *Citation, m []string, at int) {
	c.Book, c.Chapter = m[at], m[at+1]
	c.Appendix = m[at+3] != ""
	c.Section, c.Subsec, c.Page = atoi(m[at+2]), atoi(m[at+4]), atoi(m[at+5])
}

// locateSection reads the four groups of a section locator, Book, chapter, § and
// no., in the order the pattern writes them. The page is left at zero, and the
// resolver reads that zero: a citation that carries a § and no page is one of
// the other printing and is looked up by its numbering rather than by a page map.
func locateSection(c *Citation, m []string, at int) {
	c.Book, c.Chapter = m[at], m[at+1]
	c.Section, c.Subsec = atoi(m[at+2]), atoi(m[at+3])
}

func kind(s string) corpus.Kind {
	if full, ok := kindAbbrev[s]; ok {
		s = full
	}
	k, _ := corpus.KindFromHeading(s)
	return k
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
