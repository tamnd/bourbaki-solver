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
	// Body is not a mark at all. It is what a reading carries when it came off
	// the volume's own pages rather than off a contents page, which is the only
	// thing there is for a volume whose scan carries no contents to read. Three
	// volumes are in that position: ac-x-fr and lie-vii-viii-fr print none, and
	// alg-iv-vii-fr prints one whose middle leaf is not in the file.
	//
	// It is a named constant so that a caller can tell the two apart without
	// spelling the word twice. bourbaki toc build asks, because a volume already
	// read this way is not a volume it failed on.
	Body SectionMark = "body"
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
	// Title is what the manifest calls this volume. It is only used for a
	// volume of one chapter, whose contents names no chapter at all and whose
	// one chapter therefore has no title to read. See openImplied.
	Title string
	// Grammar is detected when nil.
	Grammar *Grammar
	// FrontMatterPDF is the last pdf page of what the manifest declares as the
	// note to the reader and the introduction, and is 0 for a volume that
	// declares neither. It is here for one check, on where a chapter opens, and
	// what it says there is that a span starting before this page is a span the
	// page map guessed. See validate.
	FrontMatterPDF int
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

	// Soft marks a problem that is not the contents' fault, the way audit
	// separates a rule that says the corpus is wrong from a rule that says
	// somebody should look. A contents entry landing on no pdf page because the
	// scan is short a leaf is the case: the reading is right, the volume prints
	// the page it names, and the file is what is incomplete. Reporting that in
	// the same words as a misread digit made a correct reading unwritable.
	Soft bool `json:"soft,omitempty"`
}

// Hard is the problems that are the contents' own fault. They are the ones a
// manifest must not be written over, and the soft ones are not, so the two have
// to be counted apart wherever that decision is made. See Problem.Soft.
func Hard(probs []Problem) []Problem {
	var out []Problem
	for _, p := range probs {
		if !p.Soft {
			out = append(out, p)
		}
	}
	return out
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
	//
	// The Book in front of it is its own piece and not part of the first one,
	// because the pieces after the first are four characters wide and a whole
	// label does not fit in four. The French Theorie des ensembles is the volume
	// that prints the Book in its own table, "INTRODUCTION ..... E I.7", and it
	// read as far as E II.9 and then stopped: "E II.30" is the Book, a space,
	// and six characters, so no split of it fitted the pattern and every line
	// from § 5 of chapter II onwards was dropped. That is two thirds of the
	// volume, chapters III and IV among them.
	//
	// It stays inside the capture because readLabel is where a label is cut, and
	// what it is given has to be the whole of what the line ends with.
	labelTailRe = regexp.MustCompile(
		leader + `\s*((?:[A-Z]{1,3}\s)?[A-Za-z0-9.,\-|·]{1,8}(?:\s[A-Za-z0-9.,\-|·]{1,4}){0,2})\s*$`)

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

	// A no. line of a volume that numbers its nos § point no. rather than
	// restarting the count at 1 under each §. The fascicule de résultats does
	// this and nothing else in the library does, so the reading is gated on the
	// § that is open rather than on the shape of the line alone. See dottedNo.
	//
	// The second period is optional because the reading does not print it
	// consistently: the fascicule sets no. 1 of § 1 with none and no. 2 with
	// one, on consecutive lines of the same page.
	dottedNoRe = regexp.MustCompile(`^\s*([0-9IlOJ|]{1,2})\s*[.·•]\s*([0-9IlOJ|]{1,2})\s*[.,·•]?\s+(.*)$`)

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

// dottedNo reads a no. line of a volume that numbers its nos § point no.
//
// The Varietes differentielles et analytiques is a fascicule de resultats and
// numbers its nos that way, so § 1 runs 1.1 to 1.7 where every other volume in
// the library runs 1 to 7. classify reads the first component as the number and
// leaves the second at the head of the title, so all seven nos of § 1 came out
// as no. 1 and the § was reported as holding seven of them.
//
// The reading is gated on the § that is open rather than on the shape of the
// line, and that is the point of doing it here instead of in classify. A rule
// that fired on any line opening with two numbers separated by a period would
// have to be trusted against every contents in the library, where a title is
// free to open with a figure. Requiring the first component to be the number of
// the § the line is already under costs nothing on the volume that wants it and
// cannot fire anywhere else, because a title would have to open with the number
// of its own § and a period to be mistaken for one.
func dottedNo(text string, sec int) (int, string, bool) {
	m := dottedNoRe.FindStringSubmatch(text)
	if m == nil {
		return 0, "", false
	}
	if s, ok := readNumber(m[1]); !ok || s != sec {
		return 0, "", false
	}
	n, ok := readNumber(m[2])
	if !ok {
		return 0, "", false
	}
	return n, cleanTitle(m[3]), true
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
	if ch, p, ok := readLabelBody(tok, want); ok {
		return ch, p, ok
	}
	// A contents that points at the Book as well as the chapter gets a second
	// reading with the Book taken off. Most of the library prints the chapter
	// alone in its table, "I. 1" and "IV. 110", and that is what this was
	// written for. The French Theorie des ensembles prints the whole label,
	// "INTRODUCTION ..... E I.7", and there is no way to read EI.7 as a numeral
	// and a page, so the volume yielded no contents line at all and the parser
	// reported it as having no table of contents.
	//
	// It is a second pass and not a looser first one because the field being
	// dropped is a field the other volumes need. Only a whole field goes, only
	// where it is one to three letters with no separator in it, and only where
	// it is not itself a numeral, which is what keeps "I 5" and "V 12" intact.
	if rest, ok := withoutBook(tok); ok {
		return readLabelBody(rest, want)
	}
	return "", 0, false
}

// withoutBook drops the Book prefix from a label, and reports whether there was
// one to drop.
func withoutBook(tok string) (string, bool) {
	fields := strings.Fields(tok)
	if len(fields) < 2 {
		return tok, false
	}
	head := fields[0]
	if len(head) > 3 {
		return tok, false
	}
	for _, r := range head {
		if r < 'A' || r > 'Z' {
			return tok, false
		}
	}
	if ch, ok := readRoman(head); ok && isCanonicalRoman(ch) {
		return tok, false
	}
	return strings.Join(fields[1:], " "), true
}

func readLabelBody(tok, want string) (chapter string, page int, found bool) {
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
// their own lines, indented past the nos around them. A no. is set at the same
// indent as the rest of its run, give or take the width of its own number, so
// anything pushed much further in belongs to the entry above it.
//
// The indent is only worth reading against the rest of the page it is on, and
// that list runs over onto the next one, where there is nothing above it to
// read it against. What carries it over is the numbering: examples 6, 7 and 8
// go on from example 5 rather than from no. 3, which is where the § itself had
// got to.
//
// How far past the run a nested line is set is a fact about the printing and
// not a constant. The French Integration 7 and 8 lists the same eight examples
// as the English volume, in the same place, and sets them four columns past the
// run where the English sets them fifteen. Measured over the text the parser
// reads, 2232 numbered contents lines in the library, 97.3 per cent sit between
// four columns short of the run indent and one past it, which is the width of
// their own numbers set against a column that is aligned on the right. Five sit
// two past, ten sit four or more past, and nothing at all sits three past. So
// the bar goes in that empty column.
//
// Three columns is not on its own enough to call a line nested, because a run
// whose § line the reading lost also opens further in than the run above it:
// the French General Topology 5 to 10 and the French Algebra 1 to 3 both do
// that, at four columns and two. The numbering separates them. A run either
// opens at no. 1 or goes on from the no. before it, and a nested list does
// neither, because it restarts the count inside an entry that has already been
// counted. Both example lists open on their example 2 after the § reached its
// no. 3, example 1 having been printed on no. 3's own line.
const (
	nestIndent = 8
	nearIndent = 3
)

func nested(line string, runIndent, num, lastNo, nestNum int) bool {
	if runIndent >= 0 {
		switch past := indentOf(line) - runIndent; {
		case past >= nestIndent:
			return true
		case past < nearIndent:
		case nestNum > 0:
			// The list is already open, so the only question is whether this
			// line goes on with it.
			return num == nestNum+1
		default:
			return num != 1 && num != lastNo+1
		}
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

// labelPieceRe is one piece of a label that was set with no leaders in front of
// it. A piece is short and holds nothing but the characters a label and the
// scanner's misreadings of one are made of.
var labelPieceRe = regexp.MustCompile(`^[A-Za-z0-9.,\-|·]{1,8}$`)

// labelPieces is how many space separated pieces a label with no leaders may be
// put back together from. The 2003 scan sets V.10 as "V1. 10" and the old
// French Topologie generale sets I.51 as "I. 51", so two is the common break
// and three is the most any volume produces.
const labelPieces = 3

// noLeaderLabel is noLeader for the volumes that number their pages by chapter.
// The 2004 Integration sets three of its contents lines with no leaders at all,
// among them the title of chapter IX, which runs the width of the line and
// takes its label straight after the last word.
//
// It is held in tighter than noLeader is, because a label is a good deal easier
// to see where there is none: as well as having to announce itself and to go on
// announcing the same thing once the label is off, the label has to name the
// chapter the line is already known to be in.
//
// The label is put back together from the right, one piece at a time, because
// the scanners break a label wherever there is a place to break one and the old
// French scans break it at the separator more often than not. Topologie
// generale chapitres 1 a 4 sets no. 6 of chapter I § 8 as "6. Limites dans les
// espaces produits et les espaces quotients. I. 51", with no leaders at all and
// the label in two pieces. Taking the pieces from the right is what keeps the
// last words of a long title out of the label: a pattern that reads the line
// from the left finds "SPACES IX.l" at the end of the chapter IX line of the
// English Integration before it finds "IX.l".
//
// A line that announced nothing gets a reading too, where a title is being held
// waiting for its page. noLeader refuses that case, and refuses it for a good
// reason: on the second line of a wrapped title there is nothing left saying the
// line is an entry, so a bare number at the end of it is as likely to be the
// last word of a title as a page. A label is not, because it has to name the
// chapter the entry is already known to be in, and a title whose last word is
// the open chapter's numeral and a page number joined by a period is not a thing
// the Éléments contains. The French Algèbre chapitres 1 à 3 wraps no. 4 of
// chapter II § 3 over three lines, the homomorphism Hom(E1,F1) tensor Hom(E2,F2)
// to Hom(E1 tensor E2, F1 tensor F2), and sets II.79 on the third of them with
// no leaders in front of it.
func noLeaderLabel(line, text string, e entry, mark SectionMark, want string) (string, tail, bool, entry) {
	if want == "" {
		return text, tail{}, false, e
	}
	rest, tok := strings.TrimRight(line, " \t"), ""
	for range labelPieces {
		i := strings.LastIndexAny(rest, " \t")
		if i < 0 {
			break
		}
		piece := strings.TrimLeft(rest[i:], " \t")
		if !labelPieceRe.MatchString(piece) {
			break
		}
		if tok == "" {
			tok = piece
		} else {
			tok = piece + " " + tok
		}
		rest = strings.TrimRight(rest[:i], " \t")
		ch, p, ok := readLabel(tok, want)
		if !ok || ch != want {
			continue
		}
		// The Book in front of the label is part of the label and not the last
		// word of the title. Taking one more piece only counts where it reads
		// as the same chapter and the same page, which is what a Book prefix
		// does and what a word of a title does not.
		//
		// The French Theorie des ensembles is the volume that needs it. No. 5
		// of chapter II § 6 runs the width of the line, so the printing sets a
		// period where it sets a leader: "5. Applications compatibles avec des
		// relations d'equivalence. E II.44". Stopping at II.44 left the E on the
		// end of the title.
		if head, ok := oneMorePiece(rest); ok {
			if ch2, p2, ok := readLabel(head.tok+" "+tok, want); ok && ch2 == ch && p2 == p {
				// Only the line matters from here. The loop classifies rest and
				// returns on the next statement either way, so the longer token
				// has nowhere left to go.
				rest = head.rest
			}
		}
		got := classify(rest, mark)
		if got.kind != e.kind || got.number != e.number || got.numeral != e.numeral {
			return text, tail{}, false, e
		}
		return rest, tail{chapter: ch, page: p}, true, got
	}
	return text, tail{}, false, e
}

// labelHead is a line with the last space separated piece taken off it.
type labelHead struct {
	rest string
	tok  string
}

// oneMorePiece takes the last space separated piece off a line, where that
// piece could be part of a label.
func oneMorePiece(line string) (labelHead, bool) {
	i := strings.LastIndexAny(line, " \t")
	if i < 0 {
		return labelHead{}, false
	}
	tok := strings.TrimLeft(line[i:], " \t")
	if !labelPieceRe.MatchString(tok) {
		return labelHead{}, false
	}
	return labelHead{rest: strings.TrimRight(line[:i], " \t"), tok: tok}, true
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

// flatVolume reports whether the volume prints no chapters over its body.
//
// It is the state pagemap.WholeVolume names, one span over the whole of a
// volume that declares chapters: [] and whose page map found no chapter to put
// a row under. Two volumes are in it, the two printings of the Elements of the
// History of Mathematics, which are a flat run of numbered notes with nothing
// above them.
//
// It is asked as a question about the page map rather than about the line,
// which is the whole reason the reading below is safe to do at all.
func flatVolume(pm *pagemap.Map) bool {
	return pm != nil && len(pm.Chapters) == 1 &&
		pm.Chapters[0].Chapter == pagemap.WholeVolume
}

// fascicules says the volume is bound from more than one fascicule, each with
// its own front matter, its own table of contents and its own page numbering.
// The French Varietes is the one in the library: paragraphes 1 a 7 are bound
// ahead of paragraphes 8 a 15 and the printed numbering starts over between
// them, so pagemap gives it a span each rather than one span running backwards.
//
// It is the numerals that identify those spans, because pagemap named them and
// named them with arabic ones. A chapter Bourbaki prints is always roman, so
// nothing a volume declares for itself can be taken for one of these.
func fascicules(pm *pagemap.Map) bool {
	if pm == nil || len(pm.Chapters) < 2 {
		return false
	}
	for _, sp := range pm.Chapters {
		if _, err := strconv.Atoi(sp.Chapter); err != nil {
			return false
		}
	}
	return true
}

// spanAt is the numeral of the span a pdf page falls in, taken as the last span
// that opens at or before it. A fascicule prints its contents inside itself,
// at the front in the Varietes' first and at the back in its second, so the
// span already open at that page is the one the contents describes.
func spanAt(pm *pagemap.Map, pdf int) string {
	out := ""
	for _, sp := range pm.Chapters {
		if sp.FirstPDF <= pdf {
			out = sp.Chapter
		}
	}
	return out
}

// backMatterRe matches what a table of contents lists after the body.
//
// A flat contents has no numbers to tell a note from the bibliography, so the
// only thing separating them is what they are called. The French History lists
// Bibliographie and Index des noms cites under its twenty six notes with a page
// against each, exactly as it lists the notes, and read as notes they would
// make the volume twenty eight. The English printing lists the same two and
// prints no page against either, which is why it never had to be said there.
//
// It is a word list and it is written down as one, in the two languages the
// corpus is in, the same way contentsHeadRe and bareTableHeadRe are targeted.
var backMatterRe = regexp.MustCompile(
	`(?i)^\s*(?:bibliographie|bibliography|index\b|table\s+des\s+mati|contents\b)`)

// flatEntry reads a contents line that carries a title and a page and no number
// of any kind.
//
// Every caller gates this on flatVolume, and nothing else may use it. The
// comment on minEntries says why: a rule that read any line ending near a
// numeral as an entry would take the numbered paragraphs of To the Reader and
// any prose line that happened to end on a figure. What makes it safe here is
// not the shape of the line but the state of the volume. A volume with one span
// over the whole of it prints no chapter lines, no § lines and no no. lines, so
// there is nothing for an unnumbered entry to be confused with.
//
// The number is left at zero. The notes are numbered where they are read, in
// the order the contents prints them, which is the order the English printing
// numbers the same twenty six notes in and is what makes the two manifests
// comparable.
func flatEntry(text string) (entry, bool) {
	if backMatterRe.MatchString(text) {
		return entry{}, false
	}
	title := cleanTitle(text)
	if title == "" {
		return entry{}, false
	}
	return entry{kind: kindSection, title: title}, true
}

// flatLine reports whether the line reads as an entry of a flat contents, which
// is what the two counters ask so that a page of them is seen at all.
func flatLine(text string, flat bool) bool {
	if !flat {
		return false
	}
	_, ok := flatEntry(text)
	return ok
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
	cand, candAt := contentsRun(pages, pm, g)
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
	// flat says the volume prints no chapters, which is what lets an unnumbered
	// line be read as an entry at all. flatNo numbers those entries in the order
	// they are printed, since the volume numbers none of them itself.
	flat, flatNo := flatVolume(pm), 0
	// curPDF is where in the pdf the contents page being read sits, which is
	// what says which fascicule of a volume bound from two the page describes.
	curPDF := 0

	// openImplied opens the chapter a one chapter volume never names.
	//
	// Such a volume prints no chapter line in its contents, because the chapter
	// is the volume and the cover has already said which one it is: the French
	// Integration chapter IX opens its contents at "§ 1. Prémesures et mesures
	// sur un espace topologique" and the numeral IX appears nowhere on the page.
	// Every § then arrives with no chapter open and is dropped, and the volume
	// comes out with no chapters at all.
	//
	// The page map is what settles it. When it found exactly one chapter there
	// is nothing to decide, every § in the contents belongs to that one, so the
	// chapter is opened here on the § that would otherwise be lost.
	//
	// A volume bound from fascicules is the same case twice over. Each fascicule
	// names no chapter either, and pagemap has already cut the volume at the
	// restart, so the fascicule the contents page is printed in is the one its
	// §§ belong to. With two or more chapters the volume declared for itself the
	// contents alone cannot say where one ends and the next starts, so the § is
	// still dropped and validate still reports a contents that yielded no
	// chapters, which is the honest answer.
	//
	// The title is the volume's, because the contents page does not carry one.
	// It is a starting point and not a reading: the chapter opening page prints
	// the real title, and once that page is read the title in the manifest is
	// corrected by hand and KeepTitles holds on to the correction across every
	// later rebuild. Both fascicules therefore open under the volume title and
	// are told apart by hand, which is the same one line correction.
	openImplied := func(page int) bool {
		num := ""
		switch {
		case len(pm.Chapters) == 1:
			num = pm.Chapters[0].Chapter
		case fascicules(pm):
			num = spanAt(pm, curPDF)
		}
		if num == "" {
			return false
		}
		// Whether the printing has this chapter or only the manifest does. See
		// corpus.Chapter.Nominal, which three consumers read and which until now
		// nothing ever set: keep.go carried it across a rebuild, so it could only
		// survive a value that was never written in the first place.
		//
		// The two cases here are not the same. A one chapter volume whose
		// contents never names it is a real chapter with a real numeral: the
		// French Integration chapter IX is printed as IX on every page of it and
		// is simply absent from its own contents page. A flat volume and a
		// fascicule span are the opposite. The printing sets no chapter line
		// anywhere, pagemap named the span itself, and it named it in arabic
		// precisely because a chapter Bourbaki prints is always roman. Those are
		// the manifest's own and are marked so, which is what stops the
		// assembler asking for front matter under a heading that is on no page.
		nominal := flatVolume(pm) || fascicules(pm)
		res.Chapters = append(res.Chapters, corpus.Chapter{
			Book: opt.Book, Numeral: num, Nominal: nominal,
			Title: strings.ToUpper(opt.Title), Page: page})
		cur = &res.Chapters[len(res.Chapters)-1]
		curSec, underNote = nil, false
		return true
	}

	commit := func(e entry, t tail) {
		switch e.kind {
		case kindChapter:
			res.Chapters = append(res.Chapters, corpus.Chapter{
				Book: opt.Book, Numeral: e.numeral, Title: shout(e.title), Page: t.page})
			cur = &res.Chapters[len(res.Chapters)-1]
			curSec, underNote = nil, false
		case kindSection, kindAppendix:
			if cur == nil && !openImplied(t.page) {
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
			// A § committed without a page takes the page of its first no., the
			// way a chapter takes the page of its first §.
			if curSec.Page == 0 {
				curSec.Page = t.page
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

	for ci, pg := range cand {
		curPDF = candAt[ci]
		// The second fascicule of a volume bound from two starts a contents of
		// its own, so whatever the first left open is closed here. Without this
		// the §§ of paragraphes 8 a 15 would be filed under paragraphes 1 a 7
		// and the second fascicule would come out empty.
		if fascicules(pm) && cur != nil && spanAt(pm, curPDF) != cur.Numeral {
			cur, curSec, underNote = nil, nil, false
		}
		// A wrapped title never crosses a page break in these volumes, and
		// letting one try is how a stray line picks up somebody else's page.
		pend = nil
		// The indent of a run is read off the page it is on. A contents that
		// runs over sets the margin of the next page where it likes, and the
		// English Algebra I moves it nine columns for the second half of the
		// nos of chapter III § 8.
		runIndent = -1
		// atTop is set on the first line of the page that carries anything,
		// which is where the running head sits when the page came from a reading
		// of the image rather than from the text layer.
		atTop := true
		for _, line := range mend(strings.Split(pg, "\n"), g) {
			if strings.TrimSpace(line) == "" {
				continue
			}
			top := atTop
			atTop = false
			want := ""
			if cur != nil {
				want = cur.Numeral
			}
			text, t, hasPage := splitTailIn(line, g.Page, want)
			e := classify(text, g.Mark)
			if !hasPage && e.kind != kindNone && g.Page == Bare {
				text, t, hasPage, e = noLeader(line, text, e, g.Mark)
			}
			if !hasPage && g.Page == Label && (e.kind != kindNone || pend != nil) {
				// A chapter line carries its own numeral, and it is the
				// chapter about to open rather than the one still open that
				// its label names.
				exp := want
				if e.kind == kindChapter {
					exp = e.numeral
				}
				text, t, hasPage, e = noLeaderLabel(line, text, e, g.Mark, exp)
			}
			// The running head over a page of the table of contents is set flush
			// left and in capitals, which is exactly how a part heading is set,
			// and closing the chapter on it throws away every entry on the page
			// it heads. Two of those heads are known by their wording, the one
			// that says CONTENTS and the one that carries the folio, and both are
			// turned away in isPart. The French Groupes et algebres de Lie
			// chapitre 1 prints a third that neither test catches: ALGÈBRES DE
			// LIE over the second of its two contents pages, which took §§ 5, 6
			// and 7 of chapter I and all seven of its exercise runs out of the
			// volume and left nobody a message saying so.
			//
			// What separates a head from a part is where it sits and what it
			// carries. A part heading of a table of contents names a part that
			// begins somewhere and prints the page it begins on; a running head
			// prints none and stands alone at the top of the page.
			if e.kind == kindPart && top && !hasPage {
				continue
			}
			// A volume with no chapters lists its notes with no numbers at all,
			// so nothing above this reads a single line of its contents. The two
			// printings of the Elements of the History of Mathematics are it: the
			// French lists twenty six titles, each with leaders and a page and
			// nothing in front, and came out with no chapter and no §§.
			//
			// They are read as §§ because that is what the English manifest calls
			// them, and because commit's § case is the one that calls openImplied,
			// which opens the single chapter a chapterless volume never names in
			// its contents. Numbering is by the order they are printed in, which
			// is the order the English printing numbers the same notes in, so the
			// two manifests line up note for note.
			//
			// hasPage is required. A title with no page cannot be placed, and
			// without that gate every line of prose in the front matter would
			// become a note.
			if flat && e.kind == kindNone && hasPage {
				if fe, ok := flatEntry(text); ok {
					flatNo++
					fe.number = flatNo
					e = fe
				}
			}
			// Read § point no. before anything downstream looks at the number,
			// since nested and the run indent both compare one no. against the
			// one before it and both want the real one.
			if e.kind == kindSubsection && curSec != nil {
				if n, title, ok := dottedNo(text, curSec.Number); ok {
					e.number, e.title = n, title
				}
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
			// The same thing one level down, and for the same reason. A § whose
			// line carries no page still opens where its no. 1 opens, on that
			// page or the next one, which is the fact validate leans on to catch
			// a misread digit. So the page is there to be filled in from below
			// and the § does not have to be given up.
			//
			// Giving it up is expensive, because the nos underneath it do not go
			// with it. They are committed to whatever § is still open, which is
			// the § above, and that § then holds two runs of nos end to end. The
			// French Functions of a Real Variable prints "§ 2. Équations
			// différentielles linéaires" with its leaders and no page at all, and
			// chapter IV came out with one § holding sixteen nos rather than two
			// holding seven and nine. The French General Topology loses the page
			// off § 6 of chapter IV the same way and chapter IV came out with
			// seven § rather than eight. In both the nine or so "a no. is missing
			// or doubled" that follow are one lost page number reported once per
			// no., and both volumes were refused a contents manifest over it.
			if pend != nil && e.kind == kindSubsection &&
				(pend.kind == kindSection || pend.kind == kindAppendix) {
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
func contentsPages(pages []string, g Grammar, flat bool) []string {
	var out []string
	for _, pg := range pages {
		if readsAsContents(pg, g, minEntries, flat) {
			out = append(out, pg)
		}
	}
	return out
}

// readsAsContents reports whether the page carries enough complete contents
// lines to be one.
//
// flat is what the volume's page map said, and it has to be asked here and not
// only at the point the line is read. A contents of unnumbered notes has no
// line classify will take, so the count came out zero, the page was not a
// candidate, and the volume was turned away with "no page that looks like a
// table of contents" before a single line of it was looked at.
func readsAsContents(pg string, g Grammar, min int, flat bool) bool {
	n := 0
	for _, line := range strings.Split(pg, "\n") {
		text, _, ok := splitTail(line, g.Page)
		if !ok {
			continue
		}
		if classify(text, g.Mark).kind != kindNone || flatLine(text, flat) {
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
// read is keyed by pdf page, counting from one. pm is the volume's page map,
// which is asked only whether the volume prints chapters at all, and may be nil.
func Overlay(pages []string, read map[int]string, pm *pagemap.Map) []string {
	flat := flatVolume(pm)
	out := make([]string, len(pages))
	copy(out, pages)
	for i := range out {
		text, ok := read[i+1]
		if !ok {
			continue
		}
		if contentsLines(text, flat) > contentsLines(out[i], flat) {
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
func contentsLines(pg string, flat bool) int {
	n := 0
	for _, line := range strings.Split(pg, "\n") {
		for _, form := range []PageForm{Bare, Label} {
			text, _, ok := splitTail(line, form)
			if !ok {
				continue
			}
			if classify(text, Pilcrow).kind != kindNone || flatLine(text, flat) {
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
//
// The pdf page of each kept page comes back alongside it, because a volume
// bound from two fascicules prints two tables of contents and which fascicule
// each one describes is settled by where in the book it sits.
func contentsRun(pages []string, pm *pagemap.Map, g Grammar) ([]string, []int) {
	flat := flatVolume(pm)
	keep := make([]bool, len(pages))
	for i, pg := range pages {
		if !readsAsContents(pg, g, minEntries, flat) {
			continue
		}
		e, ok := pm.Lookup(i + 1)
		keep[i] = !ok || e.Confidence == pagemap.Unknown || announcesContents(pg)
	}
	for i := range pages {
		if !keep[i] {
			continue
		}
		for j := i + 1; j < len(pages) && !keep[j] && readsAsContents(pages[j], g, minRunEntries, flat); j++ {
			keep[j] = true
		}
	}
	var out []string
	var at []int
	for i, pg := range pages {
		if keep[i] {
			out = append(out, pg)
			at = append(at, i+1)
		}
	}
	return out, at
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
