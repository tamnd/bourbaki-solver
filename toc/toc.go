// Package toc reads a volume's table of contents into the chapter, §, and no.
// tree that everything downstream is cut along.
//
// The table of contents is the only place in a Bourbaki volume that states,
// in one list, where every § and every no. begins. Reading it is what turns a
// PDF into a corpus with parts, and reading it well is what stops extraction
// from having to guess where one § ends and the next starts.
//
// The three volumes lay it out three ways:
//
//	Algebra chapter 8, 2023      front matter, "1. Title ... 13", § at the margin
//	Algebra chapters 1 to 3      front matter, "§ 1. Title ... 13"
//	Algebra chapters 4 to 7      back matter, "§ 1. Title ... IV.13"
//
// and the two scans damage it the same way they damage the running heads, so
// the digit repairs from the page map apply here too: "§ I." is § 1, "l 03" is
// page 103, "V1. 10" is V.10 and "v11.6" is VII.6.
//
// Which pages to read is not guessed. The page map already says which pages
// belong to a chapter, so the table of contents is somewhere in what is left,
// and that is true whether the publisher put it at the front or the back.
package toc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
)

// SectionMark is how a line announces that it starts a §.
type SectionMark string

const (
	// Pilcrow is "§ 1. Title", used by both scanned volumes.
	Pilcrow SectionMark = "pilcrow"
	// Column is a bare "1. Title" set at the left margin, used by the 2023
	// volume, where a no. line carries the same shape but is indented.
	Column SectionMark = "column"
)

// PageForm is how the contents prints the page a line points at.
type PageForm string

const (
	// Bare is "13".
	Bare PageForm = "bare"
	// Label is "IV.13".
	Label PageForm = "label"
)

// Grammar is the pair, detected per volume.
type Grammar struct {
	Mark SectionMark `json:"mark"`
	Page PageForm    `json:"page"`
}

func (g Grammar) String() string { return string(g.Mark) + "/" + string(g.Page) }

// Options configure a parse.
type Options struct {
	Book     string
	Chapters []string
	// Grammar is detected when nil.
	Grammar *Grammar
}

// Result is what one volume's contents yielded.
type Result struct {
	Book     string
	Grammar  Grammar
	Chapters []corpus.Chapter
	Problems []Problem
}

// Problem is something the contents said that the page map or the contents
// itself contradicts.
type Problem struct {
	Chapter string `json:"chapter,omitempty"`
	Section int    `json:"section,omitempty"`
	Detail  string `json:"detail"`
}

func (p Problem) String() string {
	switch {
	case p.Chapter != "" && p.Section > 0:
		return fmt.Sprintf("%s § %d: %s", p.Chapter, p.Section, p.Detail)
	case p.Chapter != "":
		return fmt.Sprintf("chapter %s: %s", p.Chapter, p.Detail)
	}
	return p.Detail
}

// A line of the contents is a prefix that says what it is, a title, a run of
// leader dots, and a page. The leaders are what make the tail unambiguous:
// without them a title ending in a numeral would be read as a page.
//
// The 1998 scan sets its leaders as bullets rather than periods, so the leader
// class carries all three characters the three volumes use.
const leader = `(?:(?:[.·•]\s*){2,}|\s{3,})`

var (
	// The trailing punctuation is what the scanner adds after the number, not
	// what the volume prints: the English Theory of Sets sets no. 6 of III § 7
	// at page 204 and the scan reads it as "204-", which without this drops the
	// entry and renumbers every no. after it.
	//
	// The g, the J and the dash inside the number are the 1995 General Topology
	// scan, which reads 190 as "IgO", 198 as "Ig8", 219 as "2Ig", 169 as "J 69"
	// and 225 as "22-5". Twelve contents lines of that volume end that way and
	// every one of them was dropped or misread, taking two §§ of chapter III
	// with it and putting § 1 of chapter II a hundred pages before the chapter.
	// All three are allowed only after the leader dots, where nothing but a page
	// is ever set, and not in the reader that works without them.
	//
	// The gap inside a split number is as wide as the scanner made it: the run
	// of exercises for chapter II § 2 comes out "20   7" for 207.
	bareTailRe = regexp.MustCompile(
		leader + `\s*([0-9IlOgJ|]{1,3}(?:[\s\p{Pd}]+[0-9IlOgJ|]{1,3})?)\s*[.,\-\p{Pd}']?\s*$`)

	// The 2003 scan splits and misreads the chapter numeral in the label as
	// readily as it does in a running head: "V1. 10" for V.10, "v11.6" for
	// VII.6, "V-9" for V.9.
	// The S in the page is the 1987 Topological Vector Spaces scan, which reads
	// IV.15 as "IV.lS". It is allowed here, where the chapter numeral and the
	// separator in front of it say that a page is what follows, and nowhere
	// else.
	labelTailRe = regexp.MustCompile(
		leader + `\s*([A-Za-z1l|]{1,5})\s*[.\-,oO]\s*([0-9IlOgS|]{1,4})\s*\.?\s*$`)

	// The 2003 scan does not always read the word CHAPTER itself: the line
	// that opens chapter IV comes out "CHAP-1 ER IV." So the word is matched
	// with room for junk where the scan put junk, and the numeral after it
	// still has to be a Roman numeral before the line counts as a chapter.
	//
	// The French prints CHAPITRE, and sets an em dash between the numeral and
	// the title: "CHAPITRE II. — GROUPES LOCALEMENT COMPACTS".
	chapterRe = regexp.MustCompile(
		`(?i)^\s*chap[^a-z]{0,4}(?:i[^a-z]{0,2})?(?:t[^a-z]{0,2})?(?:er|re)[^a-z]{0,2}\s*([IVXLCDM1l|]+)\s*[.,:]?\s*[\p{Pd}]?\s*(.*)$`)

	// "§ 11." and, where the scan split the number, "§ I 0." for § 10. The
	// period is what makes the split safe to allow, so the form with a period
	// is tried first and the form without one, which the 2003 scan also
	// produces ("§ 1 Ordered groups"), only takes a single token.
	pilcrowRe     = regexp.MustCompile(`^\s*§\s*([0-9IlOJ|](?:\s?[0-9IlOJ|])?)\s*[.,·•]\s*(.*)$`)
	pilcrowBareRe = regexp.MustCompile(`^\s*§\s*([0-9IlOJ|]{1,2})\s+(.*)$`)

	// A no. line, and in the 2023 volume a § line too, told apart by indent.
	//
	// The J and the middle dot are the 1995 General Topology scan again. It
	// reads the lining figure 1 at the head of a no. line as "J", five times,
	// and sets a middle dot after the number rather than a period: "9· Completion
	// of subspaces". A no. line that is not read is not one entry lost, it
	// renumbers every no. after it in the §, which is why both are here.
	numberRe = regexp.MustCompile(`^(\s*)([0-9IlOJ|](?:\s?[0-9IlOJ|])?)\s*[.,·•]\s*(.*)$`)

	// "Appendix", "Appendix 1.", "Appendix I - Polynomial maps", and the
	// French "Appendice". Bourbaki closes chapters II, III and VIII with
	// appendices that carry no. and exercises just as a § does, so they are
	// held as sections with the appendix flag set rather than as a fourth kind
	// of thing.
	appendixRe = regexp.MustCompile(`(?i)^\s*appendi[xc]e?\s*([0-9IlO|]{1,2})?\s*[.,:]?\s*[\p{Pd}]?\s*(.*)$`)

	// "Exercises", "Exercises for § 3", "Exercises on § 3", and the French
	// "Exercices" and "Exercices du § 3". The number takes a space in the middle
	// for the same reason the § lines do: the English Algebra sets the run for
	// chapter II § 10 as "Exercises for § I 0", and read as a single token that
	// is § 1, which quietly replaces the real run of § 1 with the run of § 10.
	exercisesRe = regexp.MustCompile(
		`(?i)^\s*exerci[cs]es?\b[^0-9§]*(?:§\s*([0-9IlO|](?:\s?[0-9IlO|])?))?`)

	// "Exercises for Appendix I". The appendices carry their own run, and the
	// line names one the same way the § lines name a §, so it has to be read
	// before the § form or it comes out as the run of whatever § was listed
	// last.
	exercisesAppendixRe = regexp.MustCompile(
		`(?i)^\s*exerci[cs]es?\b[^0-9§]*?appendi[xc]e?\s*([0-9IVXlO|]{1,4})?`)

	// "Historical Note", and the French "Note historique".
	historicalRe = regexp.MustCompile(`(?i)^\s*(?:historical\s+note|note\s+historique)\b`)
)

// digitFixer and romanFixer undo the substitutions the scanners make. As in the
// page map, nothing they produce is taken on trust: every number they recover
// has to sit in the range the page map already fixed for its chapter, and has
// to keep the contents in order, or it is published as a problem.
var (
	digitFixer = strings.NewReplacer("I", "1", "l", "1", "|", "1", "J", "1", "O", "0", "g", "9", "S", "5")
	// The N is two I's the scanner ran together: the 1987 Topological Vector
	// Spaces scan sets the label of chapter II page 12 as "n.12". No roman
	// numeral has an N in it, so nothing legitimate is being rewritten.
	romanFixer = strings.NewReplacer("1", "I", "L", "I", "|", "I", "0", "O", "N", "II")
)

// dashes are what the 1995 scan puts inside a page number it split, "22-5" for
// 225, and they go the way the space it puts there goes.
var dashes = regexp.MustCompile(`[\p{Pd}]`)

func readNumber(s string) (int, bool) {
	s = strings.Join(strings.Fields(dashes.ReplaceAllString(s, " ")), "")
	n, err := strconv.Atoi(digitFixer.Replace(s))
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

func readRoman(s string) (string, bool) {
	s = romanFixer.Replace(strings.ToUpper(strings.Join(strings.Fields(s), "")))
	if _, err := corpus.RomanOrder(s); err != nil {
		return "", false
	}
	return s, true
}

// readOrdinal reads a number that the volume may have set either way. The
// English Lie volume numbers its appendices in roman, "Appendix II", where
// Algebra numbers its own in arabic, "Appendix 1.", and the two forms overlap
// at 1, where both readings give the same answer.
func readOrdinal(s string) (int, bool) {
	if r, ok := readRoman(s); ok {
		if n, err := corpus.RomanOrder(r); err == nil {
			return n, true
		}
	}
	return readNumber(s)
}

// tail is the page a contents line points at.
type tail struct {
	chapter string // set only in the label form
	page    int
}

// splitTail cuts a line into its text and the page it points at. A line with no
// page is not an error: a long title wraps, and the page sits on the wrapped
// line.
func splitTail(line string, form PageForm) (text string, t tail, ok bool) {
	if form == Label {
		m := labelTailRe.FindStringSubmatchIndex(line)
		if m == nil {
			return line, tail{}, false
		}
		ch, chOK := readRoman(line[m[2]:m[3]])
		p, pOK := readNumber(line[m[4]:m[5]])
		if !chOK || !pOK {
			return line, tail{}, false
		}
		return strings.TrimRight(line[:m[0]], " \t"), tail{chapter: ch, page: p}, true
	}
	m := bareTailRe.FindStringSubmatchIndex(line)
	if m == nil {
		return line, tail{}, false
	}
	p, pOK := readNumber(line[m[2]:m[3]])
	if !pOK {
		return line, tail{}, false
	}
	return strings.TrimRight(line[:m[0]], " \t"), tail{page: p}, true
}

// tailNoLeaderRe is a page number at the end of a line with nothing but a space
// in front of it.
var tailNoLeaderRe = regexp.MustCompile(`\s([0-9IlO|]{1,3})\s*[.,\-\p{Pd}']?\s*$`)

// noLeader reads the page off a line whose title ran the whole width, so the
// leaders the reader relies on were never set. The French Topologie algebrique
// prints "4. Produit d'un espace par un espace simplement connexe 129" with a
// single space in front of the page, four times in its contents.
//
// Standing alone the pattern would read the last word of any title that ends in
// a numeral as a page. Two things hold it in. The line has to have announced
// itself as a chapter, a §, a no. or a run of exercises already, which a
// wrapped second line does not, and taking the number off the end must not
// change what it announced. What survives both is checked against the page map
// and against the order of the contents like every other page here, so a title
// that really does end in a numeral is caught downstream rather than believed.
func noLeader(line, text string, e entry, mark SectionMark) (string, tail, bool, entry) {
	m := tailNoLeaderRe.FindStringSubmatchIndex(line)
	if m == nil {
		return text, tail{}, false, e
	}
	p, ok := readNumber(line[m[2]:m[3]])
	if !ok {
		return text, tail{}, false, e
	}
	cut := strings.TrimRight(line[:m[0]], " \t")
	got := classify(cut, mark)
	if got.kind != e.kind || got.number != e.number || got.numeral != e.numeral {
		return text, tail{}, false, e
	}
	return cut, tail{page: p}, true, got
}

// kind is what a contents line announces.
type kind int

const (
	kindNone kind = iota
	kindChapter
	kindSection
	kindSubsection
	kindAppendix
	kindExercises
	kindHistorical
	kindPart
)

// classify reads the prefix. The 2023 volume marks a § with nothing but the
// left margin, so the indent is part of the grammar there and is ignored in the
// two volumes that print a pilcrow.
func classify(text string, mark SectionMark) entry {
	if m := chapterRe.FindStringSubmatch(text); m != nil {
		if r, ok := readRoman(m[1]); ok {
			return entry{kind: kindChapter, numeral: r, title: cleanTitle(m[2])}
		}
	}
	if historicalRe.MatchString(text) {
		return entry{kind: kindHistorical}
	}
	if m := exercisesAppendixRe.FindStringSubmatch(text); m != nil {
		n, _ := readOrdinal(m[1])
		return entry{kind: kindExercises, number: n, appendix: true}
	}
	if m := appendixRe.FindStringSubmatch(text); m != nil {
		n, _ := readOrdinal(m[1])
		return entry{kind: kindAppendix, number: n, title: cleanTitle(m[2])}
	}
	if m := exercisesRe.FindStringSubmatch(text); m != nil {
		n, _ := readNumber(m[1])
		return entry{kind: kindExercises, number: n}
	}
	for _, re := range []*regexp.Regexp{pilcrowRe, pilcrowBareRe} {
		if m := re.FindStringSubmatch(text); m != nil {
			if n, ok := readNumber(m[1]); ok {
				return entry{kind: kindSection, number: n, title: cleanTitle(m[2])}
			}
		}
	}
	if m := numberRe.FindStringSubmatch(text); m != nil {
		n, ok := readNumber(m[2])
		if !ok {
			return entry{}
		}
		if mark == Column && m[1] == "" {
			return entry{kind: kindSection, number: n, title: cleanTitle(m[3])}
		}
		return entry{kind: kindSubsection, number: n, title: cleanTitle(m[3])}
	}
	if isPart(text) {
		return entry{kind: kindPart, title: cleanTitle(text)}
	}
	return entry{}
}

// isPart reports whether the line is a heading set the way a chapter heading is
// set, flush left and in capitals, without being a chapter.
//
// The English Theory of Sets closes with "SUMMARY OF RESULTS ... 347", which
// carries eight §§ of its own. They are not chapter IV's, and nothing else in
// the line says so: the page map runs chapter IV to the last page of the volume
// because the back matter keeps the same foot numbering, and the §§ are
// numbered from 1 like any other. What marks it is the typography, which is the
// typography of a chapter line.
func isPart(text string) bool {
	if text == "" || text[0] == ' ' || text[0] == '\t' {
		return false
	}
	// The running head over the contents itself is set the same way, flush left
	// and in capitals, and closing the chapter on it would throw away every
	// entry on the page it heads. The French Algebra chapter 8 prints one over
	// each of its seven contents pages.
	if contentsHeadRe.MatchString(text) {
		return false
	}
	// So is the running head that carries the folio instead of the word. The
	// contents of Topological Vector Spaces is at the back and the scanner sets
	// its versos "360 TOPOLOGICAL VECTOR SPACES", flush left and in capitals,
	// which closed the chapter on the first line of every second page and threw
	// away the seven §§ of chapter II that are listed there. No heading of these
	// volumes opens with the number of the page it is printed on.
	if folioHeadRe.MatchString(text) {
		return false
	}
	upper, letters := letterCase(text)
	return letters >= 4 && upper*5 >= letters*4
}

// letterCase counts the capitals and the letters. The threshold everywhere here
// is four fifths rather than all of them, because the 1998 scan reads the small
// capitals Bourbaki sets its headings in as lowercase.
func letterCase(s string) (upper, letters int) {
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			upper++
			letters++
		case r >= 'a' && r <= 'z':
			letters++
		}
	}
	return upper, letters
}

// entry is one recognised contents line before its page is attached.
type entry struct {
	kind     kind
	number   int
	numeral  string // set for a chapter line
	title    string
	appendix bool // set on an exercise line that names an appendix
}

var (
	leaderRun = regexp.MustCompile(`[.·•]{3,}`)
	spaceRun  = regexp.MustCompile(`\s+`)
)

// cleanTitle strips the leader dots a wrapped line drags in and collapses the
// whitespace that -layout inserts to hold the columns apart.
// shout puts a chapter title that is set in capitals back into capitals. The
// 1998 volume prints its chapter titles in large and small capitals, and the
// scan reads the small ones as lowercase: chapter III comes out "TENSOR
// ALGEBRAS, ExTERIOR ALGEBRAs, SYMMETRIC ALGEBRAS". A title that is already
// almost all capitals was set in capitals, so the few that are not are damage.
func shout(s string) string {
	upper, letters := letterCase(s)
	if letters < 4 || upper*5 < letters*4 {
		return s
	}
	return strings.ToUpper(s)
}

func cleanTitle(s string) string {
	s = leaderRun.ReplaceAllString(s, " ")
	s = spaceRun.ReplaceAllString(s, " ")
	// The dashes go because the French sets one between a chapter numeral and
	// its title and the English Lie volume sets one after "Appendix I", and in
	// both the dash is punctuation of the contents rather than part of what the
	// chapter is called.
	return strings.Trim(s, " .,:;-\u2010\u2011\u2012\u2013\u2014\u2015")
}

// pending is a contents line whose title wrapped, held until the line carrying
// its page arrives.
type pending struct {
	entry
	lines int
}

// maxWrap is how many lines a wrapped title may run to. Bourbaki's longest
// contents entries wrap twice; anything past that is not a wrap, it is a line
// the parser misread, and holding on to it would let it swallow the page of a
// later entry.
const maxWrap = 3

// Parse reads the contents out of the pages the page map leaves outside every
// chapter, which is where a table of contents always is, front or back.
func Parse(pages []string, pm *pagemap.Map, opt Options) (*Result, error) {
	if pm == nil {
		return nil, fmt.Errorf("toc: a page map is required, the contents is read from the pages it leaves out")
	}
	cand := candidatePages(pages, pm)
	if len(cand) == 0 {
		return nil, fmt.Errorf("toc: %s has no pages outside its chapters, so no contents to read", opt.Book)
	}

	var g Grammar
	if opt.Grammar != nil {
		g = *opt.Grammar
	} else {
		g = Detect(cand)
	}
	cand = contentsRun(pages, pm, g)
	if len(cand) == 0 {
		return nil, fmt.Errorf("toc: %s has no page that looks like a table of contents", opt.Book)
	}

	res := &Result{Book: opt.Book, Grammar: g}
	var cur *corpus.Chapter
	var curSec *corpus.Section
	var pend *pending

	commit := func(e entry, t tail) {
		switch e.kind {
		case kindChapter:
			res.Chapters = append(res.Chapters, corpus.Chapter{
				Book: opt.Book, Numeral: e.numeral, Title: shout(e.title), Page: t.page})
			cur = &res.Chapters[len(res.Chapters)-1]
			curSec = nil
		case kindSection, kindAppendix:
			if cur == nil {
				return
			}
			if cur.Page == 0 {
				cur.Page = t.page
			}
			cur.Sections = append(cur.Sections, corpus.Section{
				Number: e.number, Title: e.title, Page: t.page,
				Appendix: e.kind == kindAppendix})
			curSec = &cur.Sections[len(cur.Sections)-1]
		case kindSubsection:
			if curSec == nil {
				return
			}
			curSec.Subsections = append(curSec.Subsections, corpus.Subsection{
				Number: e.number, Title: e.title, Page: t.page})
		case kindExercises:
			if cur == nil {
				return
			}
			// Chapter VIII prints "Exercises" at the end of each §, so the
			// current § owns them. Chapters I to VII gather them after the last
			// § as "Exercises for § 3", which names the § instead.
			target := curSec
			switch {
			case e.appendix:
				target = appendixOf(cur, e.number)
			case e.number > 0:
				if s, ok := cur.Get(e.number); ok {
					target = s
				} else {
					target = nil
				}
			}
			if target != nil {
				target.Exercises = &corpus.Locator{Page: t.page}
			}
		case kindHistorical:
			if cur != nil {
				cur.Historical = &corpus.Locator{Page: t.page}
			}
			// The French Theory of Spectra lists the parts of its historical
			// note the way it lists the no. of a §, numbered from 1. They are
			// the note's, not the last §'s, so the § is closed here.
			curSec = nil
		case kindPart:
			// Whatever follows belongs to the part, not to the chapter that
			// happened to be open, so nothing is collected again until the next
			// chapter line.
			cur, curSec = nil, nil
		}
	}

	for _, pg := range cand {
		// A wrapped title never crosses a page break in these volumes, and
		// letting one try is how a stray line picks up somebody else's page.
		pend = nil
		for _, line := range strings.Split(pg, "\n") {
			if strings.TrimSpace(line) == "" {
				continue
			}
			text, t, hasPage := splitTail(line, g.Page)
			e := classify(text, g.Mark)
			if !hasPage && e.kind != kindNone && g.Page == Bare {
				text, t, hasPage, e = noLeader(line, text, e, g.Mark)
			}

			if e.kind == kindNone {
				if pend == nil {
					continue
				}
				pend.title = strings.TrimSpace(pend.title + " " + cleanTitle(text))
				pend.lines++
				if hasPage {
					commit(pend.entry, t)
					pend = nil
				} else if pend.lines >= maxWrap {
					pend = nil
				}
				continue
			}

			// A chapter whose line carries no page is not an error. The
			// English Lie volume prints "CHAPTER VIII SPLIT SEMI-SIMPLE LIE
			// ALGEBRAS" with no leaders and no number, because the chapter
			// begins where its first § begins, and that page is on the next
			// line. The page is filled in from that § below.
			if pend != nil && pend.kind == kindChapter && e.kind == kindSection {
				commit(pend.entry, tail{})
				pend = nil
			}
			// A new prefix means the held line never found its page.
			if pend != nil {
				res.Problems = append(res.Problems, Problem{
					Chapter: chapterOf(cur),
					Detail:  fmt.Sprintf("contents entry %q has no page", pend.title)})
				pend = nil
			}
			// A part heading is committed whether or not its page could be
			// read. It contributes nothing but the fact that the chapter before
			// it has ended, and the front matter of the English Theory of Sets
			// numbers its pages in roman, "TO THE READER V", which the tail
			// reader does not read and does not need to.
			if hasPage || e.kind == kindPart {
				commit(e, t)
				continue
			}
			pend = &pending{entry: e}
		}
		if pend != nil {
			res.Problems = append(res.Problems, Problem{
				Chapter: chapterOf(cur),
				Detail:  fmt.Sprintf("contents entry %q has no page", pend.title)})
			pend = nil
		}
	}

	res.Chapters = mergeChapters(res.Chapters)
	chapterExercises(res)
	resolve(res, pm)
	res.Problems = append(res.Problems, res.validate(pm, opt)...)
	return res, nil
}

// appendixOf is the chapter's nth appendix, or its only one where the line
// named none, or nil where the chapter has no such appendix to hang the run on.
func appendixOf(c *corpus.Chapter, n int) *corpus.Section {
	var only *corpus.Section
	seen := 0
	for i := range c.Sections {
		s := &c.Sections[i]
		if !s.Appendix {
			continue
		}
		seen++
		only = s
		if s.Number == n {
			return s
		}
	}
	if n == 0 && seen == 1 {
		return only
	}
	return nil
}

// mergeChapters keeps one listing per chapter where the volume prints two.
//
// The French Springer volumes print a short SOMMAIRE at the front, chapters and
// §§ and nothing else, and the full TABLE DES MATIÈRES at the back, every no.
// with its page. Both are the volume's own contents and both parse, so the
// chapters come out twice. The fuller listing wins, and anything it happens not
// to carry is taken from the other, which is how the chapter's exercises
// survive when only one of the two lists them.
func mergeChapters(cs []corpus.Chapter) []corpus.Chapter {
	var out []corpus.Chapter
	at := map[string]int{}
	for _, c := range cs {
		i, ok := at[c.Numeral]
		if !ok {
			at[c.Numeral] = len(out)
			out = append(out, c)
			continue
		}
		keep, drop := out[i], c
		if depth(drop) > depth(keep) {
			keep, drop = drop, keep
		}
		if keep.Historical == nil {
			keep.Historical = drop.Historical
		}
		if keep.Exercises == nil {
			keep.Exercises = drop.Exercises
		}
		for j := range keep.Sections {
			if keep.Sections[j].Exercises != nil {
				continue
			}
			if s, ok := drop.Get(keep.Sections[j].Number); ok {
				keep.Sections[j].Exercises = s.Exercises
			}
		}
		out[i] = keep
	}
	return out
}

// depth is how much of a chapter a listing gives, no. lines first because that
// is what separates the full contents from the summary.
func depth(c corpus.Chapter) int {
	n := 0
	for _, s := range c.Sections {
		n += len(s.Subsections)
	}
	return n*1000 + len(c.Sections)
}

// chapterExercises moves a single unnumbered exercise run up to the chapter it
// belongs to.
//
// A contents line that says only "Exercices" does not say whose exercises they
// are, and the volumes mean two different things by it. Algebra chapter 8
// prints one after every § and means that §. Topologie algebrique prints one
// after the last § of a chapter and means the whole chapter. What tells them
// apart is the count: a volume that prints them per § prints more than one, so
// a chapter with several §§ and exactly one run, sitting on the last of them,
// is the second case.
func chapterExercises(res *Result) {
	for i := range res.Chapters {
		c := &res.Chapters[i]
		if len(c.Sections) < 2 {
			continue
		}
		runs, last := 0, -1
		for j := range c.Sections {
			if c.Sections[j].Exercises != nil {
				runs++
				last = j
			}
		}
		if runs != 1 || last != len(c.Sections)-1 {
			continue
		}
		c.Exercises = c.Sections[last].Exercises
		c.Sections[last].Exercises = nil
	}
}

func chapterOf(c *corpus.Chapter) string {
	if c == nil {
		return ""
	}
	return c.Numeral
}

// contentsHeadRe is the running head a table of contents carries, in the two
// languages the library is printed in.
// The French Theory of Spectra runs the two words together in its text layer,
// "334 CHAPITRETABLE DES MATIÈRES", so there is no word boundary in front of
// the French form and none is asked for.
var contentsHeadRe = regexp.MustCompile(`(?i)(?:\bcontents|table\s+des\s+mati)`)

// folioHeadRe is a running head with the page number in front of it, which is
// how a verso is set.
var folioHeadRe = regexp.MustCompile(`^[0-9]{1,4}\s+\S`)

// announcesContents reports whether the page says at the top that it is the
// table of contents. Only the head is read, so a body page that mentions the
// contents in a sentence does not qualify.
func announcesContents(pg string) bool {
	seen := 0
	for _, line := range strings.Split(pg, "\n") {
		if strings.TrimSpace(line) == "" {
			continue
		}
		if contentsHeadRe.MatchString(line) {
			return true
		}
		if seen++; seen >= 2 {
			return false
		}
	}
	return false
}

// candidatePages are the pages the page map assigns to no chapter, which is
// front matter and back matter, and one of those holds the contents.
//
// The French Algebra chapter 8 needs the second rule. It prints its contents at
// the back and keeps the chapter's running head over it, "A VIII.484", so the
// page map puts those seven pages inside chapter VIII along with the body and
// the first rule leaves nothing to read.
func candidatePages(pages []string, pm *pagemap.Map) []string {
	var out []string
	for i, pg := range pages {
		e, ok := pm.Lookup(i + 1)
		if !ok || e.Confidence == pagemap.Unknown || announcesContents(pg) {
			out = append(out, pg)
		}
	}
	return out
}

// minEntries is how many complete lines a page needs before it is read as
// contents. A page of the table of contents carries dozens; the pages it shares
// the front matter with carry none, because prose has no leader dots and no
// page number at the end of a line.
//
// The gate matters more than it looks. Without it the numbered paragraphs of
// "To the Reader" parse as no. lines, and a paragraph that happens to end near
// a numeral would put a fictitious no. in the tree.
const minEntries = 3

// contentsPages keeps the pages that carry a table of contents.
func contentsPages(pages []string, g Grammar) []string {
	var out []string
	for _, pg := range pages {
		if readsAsContents(pg, g) {
			out = append(out, pg)
		}
	}
	return out
}

// readsAsContents reports whether the page carries enough complete contents
// lines to be one.
func readsAsContents(pg string, g Grammar) bool {
	n := 0
	for _, line := range strings.Split(pg, "\n") {
		text, _, ok := splitTail(line, g.Page)
		if ok && classify(text, g.Mark).kind != kindNone {
			n++
		}
	}
	return n >= minEntries
}

// contentsRun is the table of contents as the volume prints it, which is a run
// of consecutive pages and not a set of pages that each announce themselves.
//
// A page opens the run if the two rules of candidatePages let it through and it
// reads as contents. Every page after it that reads as contents belongs to the
// same run, whatever its running head says and wherever the page map put it.
//
// Topological Vector Spaces needs the second half. Its contents is at the back
// and runs over six pages, and the scanner set the running head of the versos
// as "360 TOPOLOGICAL VECTOR SPACES" and of the rectos as "CONTENTS". Only the
// rectos announce themselves, so the versos were dropped and chapter II came
// out with one § where the volume prints eight, with no problem reported
// because nothing tells the validator how many §§ a chapter should have.
func contentsRun(pages []string, pm *pagemap.Map, g Grammar) []string {
	keep := make([]bool, len(pages))
	for i, pg := range pages {
		if !readsAsContents(pg, g) {
			continue
		}
		e, ok := pm.Lookup(i + 1)
		keep[i] = !ok || e.Confidence == pagemap.Unknown || announcesContents(pg)
	}
	for i := range pages {
		if !keep[i] {
			continue
		}
		for j := i + 1; j < len(pages) && !keep[j] && readsAsContents(pages[j], g); j++ {
			keep[j] = true
		}
	}
	var out []string
	for i, pg := range pages {
		if keep[i] {
			out = append(out, pg)
		}
	}
	return out
}

// Detect works out the grammar by reading the candidate pages both ways and
// taking whichever reads more. The counts are not close: a volume that prints
// "IV.13" yields no bare tails at all, because a bare tail has to start with a
// digit and the label starts with a letter.
func Detect(pages []string) Grammar {
	var bare, label, pilcrow, column int
	for _, pg := range pages {
		for _, line := range strings.Split(pg, "\n") {
			if _, _, ok := splitTail(line, Bare); ok {
				bare++
			}
			text, _, ok := splitTail(line, Label)
			if ok {
				label++
			}
			if !ok {
				text, _, _ = splitTail(line, Bare)
			}
			if pilcrowRe.MatchString(text) {
				pilcrow++
			} else if m := numberRe.FindStringSubmatch(text); m != nil && m[1] == "" {
				column++
			}
		}
	}
	g := Grammar{Mark: Pilcrow, Page: Bare}
	if label > bare {
		g.Page = Label
	}
	if column > pilcrow {
		g.Mark = Column
	}
	return g
}

// resolve turns every printed page in the tree into a PDF page, using the map
// built from the running heads. This is what lets extraction open the file at
// the right leaf without anybody counting.
func resolve(res *Result, pm *pagemap.Map) {
	for i := range res.Chapters {
		c := &res.Chapters[i]
		c.PDFPage, _ = pm.PDFPageOf(c.Numeral, c.Page)
		for j := range c.Sections {
			s := &c.Sections[j]
			s.PDFPage, _ = pm.PDFPageOf(c.Numeral, s.Page)
			for k := range s.Subsections {
				sub := &s.Subsections[k]
				sub.PDFPage, _ = pm.PDFPageOf(c.Numeral, sub.Page)
			}
			if s.Exercises != nil {
				s.Exercises.PDFPage, _ = pm.PDFPageOf(c.Numeral, s.Exercises.Page)
			}
		}
		if c.Exercises != nil {
			c.Exercises.PDFPage, _ = pm.PDFPageOf(c.Numeral, c.Exercises.Page)
		}
		if c.Historical != nil {
			c.Historical.PDFPage, _ = pm.PDFPageOf(c.Numeral, c.Historical.Page)
		}
	}
}

// Counts is what the parse found, for the report.
func (r *Result) Counts() (chapters, sections, subsections, exercises int) {
	chapters = len(r.Chapters)
	for _, c := range r.Chapters {
		sections += len(c.Sections)
		if c.Exercises != nil {
			exercises++
		}
		for _, s := range c.Sections {
			subsections += len(s.Subsections)
			if s.Exercises != nil {
				exercises++
			}
		}
	}
	return
}

// Get returns the chapter with this numeral.
func (r *Result) Get(numeral string) (*corpus.Chapter, bool) {
	for i := range r.Chapters {
		if r.Chapters[i].Numeral == strings.ToUpper(numeral) {
			return &r.Chapters[i], true
		}
	}
	return nil, false
}
