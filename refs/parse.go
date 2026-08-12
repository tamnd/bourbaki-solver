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
}

// The Books of the Éléments as the Springer translations title them. A citation
// that opens with one of these leaves the corpus, which is not an error: it is
// the list of what to ingest next.
var books = []string{
	"Set Theory",
	"Theory of Sets",
	"General Topology",
	"Commutative Algebra",
	"Topological Vector Spaces",
	"Functions of a Real Variable",
	"Differentiable and Analytic Manifolds",
	"Lie Groups and Lie Algebras",
	"Spectral Theories",
	"Integration",
	"Algebra",
}

// Code is the letter Bourbaki gives a Book, which is what an edge records when
// the citation leaves the corpus. They are the French initials, because that is
// what the Éléments use in every language.
func Code(book string) string {
	switch book {
	case "Set Theory", "Theory of Sets":
		return "E"
	case "Algebra":
		return "A"
	case "General Topology":
		return "TG"
	case "Functions of a Real Variable":
		return "FVR"
	case "Topological Vector Spaces":
		return "EVT"
	case "Integration":
		return "INT"
	case "Lie Groups and Lie Algebras":
		return "LIE"
	case "Commutative Algebra":
		return "AC"
	case "Spectral Theories":
		return "TS"
	case "Differentiable and Analytic Manifolds":
		return "VAR"
	}
	return ""
}

// bookAlt is the Books in the order the alternation tries them, which is why
// Algebra comes last: at a given position the first alternative that matches
// wins, and Algebra would take the tail of Commutative Algebra.
var bookAlt = strings.Join(books, "|")

const part = `(?:,\s*[a-z]\))?`

const kindAlt = `Proposition|Theorem|Corollary|Lemma|Definition|Remark|Example|Exercise|Scholium`

// locator is the part of a citation that finds the page: an optional Book, a
// chapter in Roman, and then any of § no. Appendix before the page itself.
// Every one of the middle parts is optional, because the chapter shows all the
// combinations in use, and the page is not, because it is the anchor the whole
// scheme is built on.
//
// The optional dollar is a transcription artefact and not a form the book has.
// Six pages of chapter VIII run the page number into the maths that follows it,
// so "p. 213, θ is bijective" comes out as `p. $213,\theta$`. Six references is
// not much, but a locator that fails leaves a bare "Corollary 2" behind, and a
// bare kind and number is the one thing the resolver is willing to guess at, so
// each of these turns into a confident edge pointing at the wrong statement of
// the wrong Book. Tolerating the dollar is cheaper than the guess.
var locator = `(?:(` + bookAlt + `),\s*)?\b([IVX]+)(?:,\s*§\s*(\d+))?(,\s*Appendix)?(?:,\s*[Nn]o\.\s*(\d+))?,\s*p\.\s*\$?(\d+)`

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
	// The optional letter is the part of a statement being pointed at, which the
	// book writes into the reference: "Corollary 1, c) of VIII, p. 152" means
	// part c) of that Corollary. There are 29 in the chapter. Nothing is done
	// with the letter, since a part of a statement carries no tag of its own, but
	// it has to be read past: left out, the reference comes apart into a bare
	// Corollary 1 and a page, and the bare half is what the resolver guesses at.
	namedRE = regexp.MustCompile(`\b(` + kindAlt + `)\s+(\d+)` + part + `\s+(?:of\s+|\()` + locator)
	// The locator on the end is what says which § the parent stands in, and
	// leaving it out was the single largest fault in the graph. "the corollary
	// of Proposition 12 of II, §1, No. 8, p. 209" came apart into a corollary
	// hunted for in chapter VIII, where there is no Proposition 12 to hang it
	// on, and a page of chapter II that nothing pointed at. Read whole it is one
	// reference and it leaves the corpus. The same shape with this chapter's own
	// pages, "Corollary 1 of Proposition 4 (VIII, p. 83)", is a reference from
	// § 11 to a statement of § 5, and resolving it in § 11 could only ever fail.
	attachedRE = regexp.MustCompile(`\b[Cc]orollary\s*(\d*)` + part +
		`\s+(?:of|to)\s+(` + kindAlt + `)\s+(\d+)(?:\s*(?:of\s+|,\s*|\()` + locator + `\)?)?`)
	// The same reference with the locator written first, which is what the
	// brackets do: "(II, §1, No. 9, p. 210, Corollary of Proposition 13)" and
	// "(VIII, p. 82, Corollary 1 of Theorem 2)". This has to be tried before the
	// page form, which would take the head of it and hand the tail to the
	// resolver as a bare corollary: the second of those is a Corollary 1 that
	// § 5 prints two of, and dropping "of Theorem 2" throws away the one thing
	// in the sentence that says which.
	leadRE    = regexp.MustCompile(locator + `,\s*[Cc]orollary\s*(\d*)` + part + `\s+(?:of|to)\s+(` + kindAlt + `)\s+(\d+)`)
	pageRE    = regexp.MustCompile(locator + `(?:,\s*(` + kindAlt + `)\s*(\d+))?`)
	formulaRE = regexp.MustCompile(`formula\s*\((\d+)\)`)
	locCitRE  = regexp.MustCompile(`(loc\. cit\.)`)
	// A second page in the same bracket does not repeat the chapter: the book
	// writes "(VIII, p. 190, Corollary and p. 401, Corollary 1)" and means
	// chapter VIII both times. Five references of the chapter are written this
	// way. The kind and number are required here, unlike in the full page form,
	// so that a bare "p. 12" in prose is not read as a citation on the strength
	// of a chapter named a sentence earlier.
	contRE  = regexp.MustCompile(`p\.\s*\$?(\d+),\s*(` + kindAlt + `)\s*(\d+)`)
	localRE = regexp.MustCompile(`\b(` + kindAlt + `)\s+(\d+)\b`)

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
var forms = []*regexp.Regexp{namedRE, attachedRE, leadRE, pageRE, formulaRE, locCitRE, contRE, localRE}

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
	atNamed    = 1
	atAttached = atNamed + namedRE.NumSubexp()
	atLead     = atAttached + attachedRE.NumSubexp()
	atPage     = atLead + leadRE.NumSubexp()
	atFormula  = atPage + pageRE.NumSubexp()
	atLocCit   = atFormula + formulaRE.NumSubexp()
	atCont     = atLocCit + locCitRE.NumSubexp()
	atLocal    = atCont + contRE.NumSubexp()
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
		for _, m := range scanner.FindAllStringSubmatch(line, -1) {
			c, ok := citation(m, last)
			if !ok {
				continue
			}
			c.Line = line0 + i
			out = append(out, c)
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
	page, formula, locCit, cont, local := atPage, atFormula, atLocCit, atCont, atLocal
	switch {
	case m[named] != "":
		c := Citation{Raw: m[0], Form: FormNamed}
		c.Kind, c.Number = kind(m[named]), atoi(m[named+1])
		locate(&c, m, named+2)
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
	case m[page+1] != "":
		c := Citation{Raw: m[0], Form: FormPage}
		locate(&c, m, page)
		if m[page+6] != "" {
			c.Kind, c.Number = kind(m[page+6]), atoi(m[page+7])
		}
		return c, true
	case m[formula] != "":
		return Citation{Raw: m[0], Form: FormFormula, Kind: corpus.KindEquation, Number: atoi(m[formula])}, true
	case m[locCit] != "":
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

func kind(s string) corpus.Kind {
	k, _ := corpus.KindFromHeading(s)
	return k
}

func atoi(s string) int {
	n, _ := strconv.Atoi(strings.TrimSpace(s))
	return n
}
