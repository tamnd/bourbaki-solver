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
	bareTailRe = regexp.MustCompile(
		leader + `\s*([0-9IlO|]{1,3}(?:\s[0-9IlO|]{1,3})?)\s*\.?\s*$`)

	// The 2003 scan splits and misreads the chapter numeral in the label as
	// readily as it does in a running head: "V1. 10" for V.10, "v11.6" for
	// VII.6, "V-9" for V.9.
	labelTailRe = regexp.MustCompile(
		leader + `\s*([A-Za-z1l|]{1,5})\s*[.\-,oO]\s*([0-9IlO|]{1,4})\s*\.?\s*$`)

	// The 2003 scan does not always read the word CHAPTER itself: the line
	// that opens chapter IV comes out "CHAP-1 ER IV." So the word is matched
	// with room for junk where the scan put junk, and the numeral after it
	// still has to be a Roman numeral before the line counts as a chapter.
	chapterRe = regexp.MustCompile(
		`(?i)^\s*chap[^a-z]{0,4}(?:t[^a-z]{0,2})?er[^a-z]{0,2}\s*([IVXLCDM1l|]+)\s*[.,:]?\s*(.*)$`)

	// "§ 11." and, where the scan split the number, "§ I 0." for § 10. The
	// period is what makes the split safe to allow, so the form with a period
	// is tried first and the form without one, which the 2003 scan also
	// produces ("§ 1 Ordered groups"), only takes a single token.
	pilcrowRe     = regexp.MustCompile(`^\s*§\s*([0-9IlO|](?:\s?[0-9IlO|])?)\s*[.,]\s*(.*)$`)
	pilcrowBareRe = regexp.MustCompile(`^\s*§\s*([0-9IlO|]{1,2})\s+(.*)$`)

	// A no. line, and in the 2023 volume a § line too, told apart by indent.
	numberRe = regexp.MustCompile(`^(\s*)([0-9IlO|](?:\s?[0-9IlO|])?)\s*[.,]\s*(.*)$`)

	// "Appendix", "Appendix 1." Bourbaki closes chapters II, III and VIII with
	// appendices that carry no. and exercises just as a § does, so they are
	// held as sections with the appendix flag set rather than as a fourth kind
	// of thing.
	appendixRe = regexp.MustCompile(`(?i)^\s*appendix\s*([0-9IlO|]{1,2})?\s*[.,:]?\s*(.*)$`)

	// "Exercises", "Exercises for § 3", "Exercises on § 3".
	exercisesRe = regexp.MustCompile(`(?i)^\s*exercises?\b[^0-9§]*(?:§\s*([0-9IlO|]{1,2}))?`)

	historicalRe = regexp.MustCompile(`(?i)^\s*historical\s+note\b`)
)

// digitFixer and romanFixer undo the substitutions the scanners make. As in the
// page map, nothing they produce is taken on trust: every number they recover
// has to sit in the range the page map already fixed for its chapter, and has
// to keep the contents in order, or it is published as a problem.
var (
	digitFixer = strings.NewReplacer("I", "1", "l", "1", "|", "1", "O", "0")
	romanFixer = strings.NewReplacer("1", "I", "L", "I", "|", "I", "0", "O")
)

func readNumber(s string) (int, bool) {
	s = strings.Join(strings.Fields(s), "")
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
	if m := appendixRe.FindStringSubmatch(text); m != nil {
		n, _ := readNumber(m[1])
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
	return entry{}
}

// entry is one recognised contents line before its page is attached.
type entry struct {
	kind    kind
	number  int
	numeral string // set for a chapter line
	title   string
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
	upper, letters := 0, 0
	for _, r := range s {
		switch {
		case r >= 'A' && r <= 'Z':
			upper++
			letters++
		case r >= 'a' && r <= 'z':
			letters++
		}
	}
	if letters < 4 || upper*5 < letters*4 {
		return s
	}
	return strings.ToUpper(s)
}

func cleanTitle(s string) string {
	s = leaderRun.ReplaceAllString(s, " ")
	s = spaceRun.ReplaceAllString(s, " ")
	return strings.Trim(s, " .,:;")
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
	cand = contentsPages(cand, g)
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
			if e.number > 0 {
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

			// A new prefix means the held line never found its page.
			if pend != nil {
				res.Problems = append(res.Problems, Problem{
					Chapter: chapterOf(cur),
					Detail:  fmt.Sprintf("contents entry %q has no page", pend.title)})
				pend = nil
			}
			if hasPage {
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

	resolve(res, pm)
	res.Problems = append(res.Problems, res.validate(pm, opt)...)
	return res, nil
}

func chapterOf(c *corpus.Chapter) string {
	if c == nil {
		return ""
	}
	return c.Numeral
}

// candidatePages are the pages the page map assigns to no chapter, which is
// front matter and back matter, and one of those holds the contents.
func candidatePages(pages []string, pm *pagemap.Map) []string {
	var out []string
	for i, pg := range pages {
		e, ok := pm.Lookup(i + 1)
		if !ok || e.Confidence == pagemap.Unknown {
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
		n := 0
		for _, line := range strings.Split(pg, "\n") {
			text, _, ok := splitTail(line, g.Page)
			if ok && classify(text, g.Mark).kind != kindNone {
				n++
			}
		}
		if n >= minEntries {
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
