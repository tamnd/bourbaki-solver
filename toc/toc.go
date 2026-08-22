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
//
// The third form, a single dot with a space on each side, is what a model
// writes when it reads a contents page off the image rather than off the text
// layer. It sees a run of leaders and gives back " . " for it, and the reading
// of the English Lie 1 to 3 does that on two lines of § 3 of chapter III, which
// dropped no. 1 and no. 4 of the § and renumbered every no. after them. A
// single dot alone would be too little to go on, since a title can end in an
// abbreviation, but a dot standing on its own between two spaces is not how a
// sentence ends, and the page still has to be a page for the tail to match.
const leader = `(?:(?:[.·•]\s*){2,}|\s{3,}|\s[.·•]\s)`

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
	//
	// The bracket is the 1989 Lie 1 to 3 scan, which sets a lining 1 with a
	// serif at each end and reads it as a square bracket. It puts the run of
	// exercises for chapter III § 7 at "39]" for 391, and the folio of printed
	// 392 at "I]!".
	bareTailRe = regexp.MustCompile(
		leader + `\s*([0-9IlOgJ|\]]{1,3}(?:[\s\p{Pd}]+[0-9IlOgJ|\]]{1,3})?)\s*[.,\-\p{Pd}']?\s*$`)

	// A label is taken off the line whole and cut in readLabel rather than by
	// the regexp, because the scanners break a label in every place there is to
	// break one and no one pattern reads all of them. What is matched here is a
	// short run of label characters at the end of the line, in up to three
	// pieces where the scan put spaces: the 2003 scan sets V.10 as "V1. 10" and
	// the 2004 Integration sets IV.110 as "IV. 11 0".
	labelTailRe = regexp.MustCompile(
		leader + `\s*([A-Za-z0-9.,\-|·]{1,8}(?:\s[A-Za-z0-9.,\-|·]{1,4}){0,2})\s*$`)

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
	// of thing. Chapter IX of the English Integration 7 to 9 closes with one it
	// calls an annex, "ANNEX: Complements on Hilbert spaces", and it carries
	// no. and exercises like the rest of them.
	appendixRe = regexp.MustCompile(
		`(?i)^\s*(?:appendi[xc]e?|annexe?)\s*([0-9IlO|]{1,2})?\s*[.,:]?\s*[\p{Pd}]?\s*(.*)$`)

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
		`(?i)^\s*exerci[cs]es?\b[^0-9§]*?(?:appendi[xc]e?|annexe?)\s*([0-9IVXlO|]{1,4})?`)

	// "Historical Note", and the French "Note historique".
	historicalRe = regexp.MustCompile(`(?i)^\s*(?:historical\s+note|note\s+historique)\b`)
)

// digitFixer and romanFixer undo the substitutions the scanners make. As in the
// page map, nothing they produce is taken on trust: every number they recover
// has to sit in the range the page map already fixed for its chapter, and has
// to keep the contents in order, or it is published as a problem.
var (
	digitFixer = strings.NewReplacer("I", "1", "l", "1", "|", "1", "J", "1", "]", "1", "O", "0", "g", "9", "S", "5")
	// The N is two I's the scanner ran together: the 1987 Topological Vector
	// Spaces scan sets the label of chapter II page 12 as "n.12". No roman
	// numeral has an N in it, so nothing legitimate is being rewritten.
	// The Y is the 2004 Integration scan, which reads the V of a label as a Y
	// throughout chapter V: "Y.25", "Y.62". No roman numeral has a Y in it
	// either.
	romanFixer = strings.NewReplacer("1", "I", "L", "I", "|", "I", "0", "O", "N", "II", "Y", "V")
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

// labelSeparators are the characters a scan puts between the chapter numeral
// and the page of a label where the volume prints a period. The letters are the
// 2003 and 2004 scans reading the period as an o, and the zero is the 2004
// Integration reading it as a nought: II.4 comes out "1104".
const labelSeparators = ".,·•-oO"

// readLabel cuts a page label into the chapter it names and the page in that
// chapter, and is where the several ways a scan can break a label are undone.
//
// The label is read from the right rather than matched: the longest prefix that
// is a chapter numeral, written the way the number is written, and with a page
// after it, is the answer. Reading it any other way gets the 2004 Integration
// wrong. That scan runs the numeral into the page it labels, so III.13 comes
// out "IIL13" and II.1 comes out "ILl", and a reader that takes as much as it
// can for the numeral cuts "IIL13" into IIII and 3, both of which are a number
// and neither of which is right. A reader that takes as little as it can cuts
// "1104", which is II.4, into I and 104.
//
// The A is the same scan again, reading the period and the four that follows it
// as one letter: III.4 comes out "IliA", V.48 comes out "YA8". It is read only
// where a separator would stand, which is a place no chapter numeral and no
// page begins.
// Where two readings are both a numeral and a page, the chapter the line is
// being read in settles it, because a label names the chapter it stands in. The
// 2004 Integration sets the run of exercises of chapter II § 1 as "ILl5", which
// is II.15 and is also III.5, and the entry is read while chapter II is open.
func readLabel(tok, want string) (chapter string, page int, found bool) {
	// What is trimmed off the end is punctuation the scanner added, and the
	// letters the separator is misread as are not trimmed there: the 1987 scan
	// sets II.10 as "II.lO", and a trim that took the O for a stray separator
	// would leave II.1 and put no. 4 of chapter II § 2 nine pages before no. 3.
	s := strings.TrimRight(strings.Join(strings.Fields(tok), ""), ".,·•-'")
	for i := len(s); i > 0; i-- {
		ch, ok := readRoman(s[:i])
		if !ok || !isCanonicalRoman(ch) {
			continue
		}
		rest := strings.TrimLeft(s[i:], labelSeparators)
		if strings.HasPrefix(rest, "A") {
			rest = "4" + rest[1:]
		}
		if rest == "" {
			continue
		}
		p, ok := readNumber(rest)
		if !ok {
			continue
		}
		if ch == want {
			return ch, p, true
		}
		if !found {
			chapter, page, found = ch, p, true
		}
	}
	return chapter, page, found
}

// isCanonicalRoman says whether a numeral is the way its number is written.
// RomanOrder reads IIII as four, and a numeral that has run into the page it
// labels is exactly the kind of numeral that comes out that way. No volume
// numbers a chapter IIII.
func isCanonicalRoman(s string) bool {
	n, err := corpus.RomanOrder(s)
	if err != nil {
		return false
	}
	return writeRoman(n) == s
}

func writeRoman(n int) string {
	var b strings.Builder
	for _, r := range []struct {
		n int
		s string
	}{{1000, "M"}, {900, "CM"}, {500, "D"}, {400, "CD"}, {100, "C"}, {90, "XC"},
		{50, "L"}, {40, "XL"}, {10, "X"}, {9, "IX"}, {5, "V"}, {4, "IV"}, {1, "I"}} {
		for n >= r.n {
			b.WriteString(r.s)
			n -= r.n
		}
	}
	return b.String()
}

// indentOf is how far a line is pushed in from the left margin.
func indentOf(line string) int {
	return len(line) - len(strings.TrimLeft(line, " \t"))
}

// nested says a numbered line is a list inside an entry rather than a no. of
// its own. Chapter VII of the English Integration 7 to 9 lists no. 3 of § 3 as
// "Examples: 1. General linear group" and sets its other seven examples on
// their own lines, indented far past the nos around them. A no. is set at the
// same indent as the rest of its run, give or take the width of its own
// number, so anything pushed much further in belongs to the entry above it.
//
// The indent is only worth reading against the rest of the page it is on, and
// that list runs over onto the next one, where there is nothing above it to
// read it against. What carries it over is the numbering: examples 6, 7 and 8
// go on from example 5 rather than from no. 3, which is where the § itself had
// got to.
const nestIndent = 8

func nested(line string, runIndent, num, lastNo, nestNum int) bool {
	if runIndent >= 0 && indentOf(line) >= runIndent+nestIndent {
		return true
	}
	return nestNum > 0 && num == nestNum+1 && num != lastNo+1
}

// splitTail cuts a line into its text and the page it points at. A line with no
// page is not an error: a long title wraps, and the page sits on the wrapped
// line.
func splitTail(line string, form PageForm) (text string, t tail, ok bool) {
	return splitTailIn(line, form, "")
}

// splitTailIn is splitTail read inside a chapter, which is what tells a label
// that could be read two ways which of the two it is.
func splitTailIn(line string, form PageForm, want string) (text string, t tail, ok bool) {
	if form == Label {
		m := labelTailRe.FindStringSubmatchIndex(line)
		if m == nil {
			return line, tail{}, false
		}
		ch, p, ok := readLabel(line[m[2]:m[3]], want)
		if !ok {
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

// labelNoLeaderRe is a label at the end of a line with nothing but a space in
// front of it.
var labelNoLeaderRe = regexp.MustCompile(`\s([A-Za-z0-9.,\-|·]{2,8})\s*$`)

// noLeaderLabel is noLeader for the volumes that number their pages by chapter.
// The 2004 Integration sets three of its contents lines with no leaders at all,
// among them the title of chapter IX, which runs the width of the line and
// takes its label straight after the last word.
//
// It is held in tighter than noLeader is, because a label is a good deal easier
// to see where there is none: as well as having to announce itself and to go on
// announcing the same thing once the label is off, the label has to name the
// chapter the line is already known to be in.
func noLeaderLabel(line, text string, e entry, mark SectionMark, want string) (string, tail, bool, entry) {
	if want == "" {
		return text, tail{}, false, e
	}
	m := labelNoLeaderRe.FindStringSubmatchIndex(line)
	if m == nil {
		return text, tail{}, false, e
	}
	ch, p, ok := readLabel(line[m[2]:m[3]], want)
	if !ok || ch != want {
		return text, tail{}, false, e
	}
	cut := strings.TrimRight(line[:m[0]], " \t")
	got := classify(cut, mark)
	if got.kind != e.kind || got.number != e.number || got.numeral != e.numeral {
		return text, tail{}, false, e
	}
	return cut, tail{chapter: ch, page: p}, true, got
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
	// underNote says the lines being read belong to a historical note or to a
	// part rather than to a §. The French Theory of Spectra numbers the parts of
	// its historical note from 1 and they read exactly like a no., so they are
	// dropped on purpose and are not a chapter losing its content.
	var underNote bool
	var pend *pending
	// runIndent is where the nos of the § being read start on the line, or -1
	// between runs. It is what tells a numbered line that is really a list
	// inside an entry from a no. of its own. lastNo is the last no. taken into
	// the run, and nestNum the last item taken out of it, which is what reads
	// a list that runs over onto the next page.
	runIndent, lastNo, nestNum := -1, 0, 0

	commit := func(e entry, t tail) {
		switch e.kind {
		case kindChapter:
			res.Chapters = append(res.Chapters, corpus.Chapter{
				Book: opt.Book, Numeral: e.numeral, Title: shout(e.title), Page: t.page})
			cur = &res.Chapters[len(res.Chapters)-1]
			curSec, underNote = nil, false
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
			curSec, underNote = &cur.Sections[len(cur.Sections)-1], false
		case kindSubsection:
			if curSec == nil {
				// A no. with no § over it belongs to the chapter. Chapter I of
				// the English Integration prints three nos straight under the
				// chapter heading and never opens a §, and dropping them is a
				// chapter the corpus says has no content. Whether this chapter
				// is that one or a contents line misread is not decided here,
				// because the § that would settle it is further down the page:
				// validate refuses a chapter that ends up with both.
				if cur == nil || underNote {
					return
				}
				cur.Subsections = append(cur.Subsections, corpus.Subsection{
					Number: e.number, Title: e.title, Page: t.page})
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
				return
			}
			// A chapter with no § has nothing to hang a run on, and the volume
			// names it for the chapter: Integration prints "Exercises for Ch. I"
			// under the three nos of chapter I. It goes where the run of a
			// chapter that gathers its exercises at the end goes.
			if curSec == nil && len(cur.Sections) == 0 && !e.appendix && e.number == 0 {
				cur.Exercises = &corpus.Locator{Page: t.page}
			}
		case kindHistorical:
			if cur != nil {
				cur.Historical = &corpus.Locator{Page: t.page}
			}
			// The French Theory of Spectra lists the parts of its historical
			// note the way it lists the no. of a §, numbered from 1. They are
			// the note's, not the last §'s, so the § is closed here.
			curSec, underNote = nil, true
		case kindPart:
			// Whatever follows belongs to the part, not to the chapter that
			// happened to be open, so nothing is collected again until the next
			// chapter line.
			cur, curSec, underNote = nil, nil, true
		}
	}

	for _, pg := range cand {
		// A wrapped title never crosses a page break in these volumes, and
		// letting one try is how a stray line picks up somebody else's page.
		pend = nil
		// The indent of a run is read off the page it is on. A contents that
		// runs over sets the margin of the next page where it likes, and the
		// English Algebra I moves it nine columns for the second half of the
		// nos of chapter III § 8.
		runIndent = -1
		for _, line := range mend(strings.Split(pg, "\n"), g) {
			if strings.TrimSpace(line) == "" {
				continue
			}
			want := ""
			if cur != nil {
				want = cur.Numeral
			}
			text, t, hasPage := splitTailIn(line, g.Page, want)
			e := classify(text, g.Mark)
			if !hasPage && e.kind != kindNone && g.Page == Bare {
				text, t, hasPage, e = noLeader(line, text, e, g.Mark)
			}
			if !hasPage && e.kind != kindNone && g.Page == Label {
				// A chapter line carries its own numeral, and it is the
				// chapter about to open rather than the one still open that
				// its label names.
				exp := want
				if e.kind == kindChapter {
					exp = e.numeral
				}
				text, t, hasPage, e = noLeaderLabel(line, text, e, g.Mark, exp)
			}
			if e.kind == kindSubsection && nested(line, runIndent, e.number, lastNo, nestNum) {
				nestNum = e.number
				e = entry{}
			}
			if e.kind == kindSubsection {
				nestNum, lastNo = 0, e.number
				if i := indentOf(line); runIndent < 0 || i < runIndent {
					runIndent = i
				}
			} else if e.kind != kindNone {
				runIndent, lastNo, nestNum = -1, 0, 0
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

// bareTableHeadRe is a running head that is the word TABLE and the folio and
// nothing else.
//
// Groupes et algebres de Lie chapitres 4 a 6 in French is why it is here. Its
// table of contents runs from pdf 279 to pdf 282 at the back of the volume and
// the head over every one of those pages comes out of the scan as "TABLE", with
// the rest of "TABLE DES MATIÈRES" lost off the right of the line: pdf 281
// reads "TABLE 287" and pdf 280 reads "286 TABLE".
//
// It is asked to be the whole head rather than to appear in it, because half a
// word is weak evidence and a page whose head is nothing but that word is not
// something the body of a Bourbaki volume prints. The heads over the body are
// the chapter title or the volume title, and the tables of notation and the
// indexes say what they are the table of.
var bareTableHeadRe = regexp.MustCompile(`(?i)^\s*(?:[0-9]{1,4}\s+)?table\s*(?:\s[0-9]{1,4})?\s*$`)

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
		if contentsHeadRe.MatchString(line) || bareTableHeadRe.MatchString(line) {
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

// numberOnlyRe is a contents line whose text is nothing but the number of the
// entry, the page having been read off the end of it already.
var numberOnlyRe = regexp.MustCompile(`^(\s*)([0-9IlOJ|]{1,2})\s*[.,·•]?\s*$`)

// mend puts back the title of a contents line the text layer set on two lines.
//
// The 2003 Functions of a Real Variable is where this shows. Fifteen of its
// contents lines come out as the number, the leaders and the page with nothing
// between them, and the title alone on the line below, "6.  ......... 188" and
// then "Linear differential equations with constant coefficients". The title
// line carries no page, so it was dropped, and the numbered line was dropped
// with it for having no title. Chapter V lost five of the nos of § 1 and § 2
// and three of the six of its appendix.
//
// A line whose text is a bare number is broken however it got that way, and the
// line under it is its title as long as that line announces nothing of its own
// and carries no page of its own.
func mend(lines []string, g Grammar) []string {
	var out []string
	out = append(out, lines...)
	for i, line := range out {
		text, _, ok := splitTail(line, g.Page)
		if !ok {
			continue
		}
		m := numberOnlyRe.FindStringSubmatch(text)
		if m == nil {
			continue
		}
		j := i + 1
		for j < len(out) && strings.TrimSpace(out[j]) == "" {
			j++
		}
		if j >= len(out) {
			continue
		}
		title, _, hasPage := splitTail(out[j], g.Page)
		if hasPage || strings.TrimSpace(title) == "" || classify(title, g.Mark).kind != kindNone {
			continue
		}
		out[i] = m[1] + m[2] + ". " + strings.TrimSpace(title) + line[len(text):]
		out[j] = ""
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

// minRunEntries is what a page needs to go on with a run that is already open.
// The last page of the contents of the English Integration 7 to 9 lists two
// things, the exercises of the annex of chapter IX and the historical note,
// and then turns to the index. A page that follows a page of contents has much
// less to prove than one that has to open the run on its own.
const minRunEntries = 2

// contentsPages keeps the pages that carry a table of contents.
func contentsPages(pages []string, g Grammar) []string {
	var out []string
	for _, pg := range pages {
		if readsAsContents(pg, g, minEntries) {
			out = append(out, pg)
		}
	}
	return out
}

// readsAsContents reports whether the page carries enough complete contents
// lines to be one.
func readsAsContents(pg string, g Grammar, min int) bool {
	n := 0
	for _, line := range strings.Split(pg, "\n") {
		text, _, ok := splitTail(line, g.Page)
		if ok && classify(text, g.Mark).kind != kindNone {
			n++
		}
	}
	return n >= min
}

// Overlay puts a model's reading of a page in place of the text layer's,
// wherever the reading carries more of the contents than the layer does.
//
// It is what lets a volume be read at all when the layer has no contents in it.
// The scan of Espaces vectoriels topologiques prints all twenty two §§ of its
// contents with their titles, their leader dots and no page numbers whatever,
// because the column those numbers stand in was never captured; the scan of
// Groupes et algebres de Lie chapitre 9 captures that column for the last few
// lines of the page and drops it from the rest. Neither yields a chapter. What
// the reading of the page image carries, they cannot.
//
// The two are compared rather than one preferred, and the count is of complete
// contents lines, an entry with a page on the end of it. That is the measure
// that matters here, and it is what makes this safe to run over a volume that
// needs none of it: a page whose layer already reads is left alone, so a reading
// that came back short or refused cannot take a working contents away.
//
// read is keyed by pdf page, counting from one.
func Overlay(pages []string, read map[int]string) []string {
	out := make([]string, len(pages))
	copy(out, pages)
	for i := range out {
		text, ok := read[i+1]
		if !ok {
			continue
		}
		if contentsLines(text) > contentsLines(out[i]) {
			out[i] = text
		}
	}
	return out
}

// contentsLines counts the lines of a page that are a contents entry with a page
// number on the end.
//
// Both page forms are tried and the mark is not, because this runs before the
// grammar is detected and only has to compare one page against another reading
// of the same page. A bare "1. Title" at the margin is a § in the 2023 volumes
// and a no. everywhere else, and either way it is an entry and it counts.
func contentsLines(pg string) int {
	n := 0
	for _, line := range strings.Split(pg, "\n") {
		for _, form := range []PageForm{Bare, Label} {
			text, _, ok := splitTail(line, form)
			if ok && classify(text, Pilcrow).kind != kindNone {
				n++
				break
			}
		}
	}
	return n
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
		if !readsAsContents(pg, g, minEntries) {
			continue
		}
		e, ok := pm.Lookup(i + 1)
		keep[i] = !ok || e.Confidence == pagemap.Unknown || announcesContents(pg)
	}
	for i := range pages {
		if !keep[i] {
			continue
		}
		for j := i + 1; j < len(pages) && !keep[j] && readsAsContents(pages[j], g, minRunEntries); j++ {
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
//
// The mark is decided by a margin rather than by a majority, because the two
// counts are not the same kind of evidence. A volume that marks its §§ with a
// pilcrow prints one for every § and nothing else prints one, so a pilcrow is
// proof; a line at the margin that starts with a number is a § only in the 2023
// volumes and is otherwise a no. whose indent was lost. Reading a page image
// loses that indent all the time, and the reading of the English Lie 1 to 3
// puts twenty eight no. lines at the margin against twenty five pilcrows, which
// under a majority took every one of its §§ apart and turned three chapters
// into thirty seven. The volumes that really are set in columns print no
// pilcrow at all, or one, so the margin costs them nothing.
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
	if column > 2*pilcrow {
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
		for j := range c.Subsections {
			sub := &c.Subsections[j]
			sub.PDFPage, _ = pm.PDFPageOf(c.Numeral, sub.Page)
		}
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
		subsections += len(c.Subsections)
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
