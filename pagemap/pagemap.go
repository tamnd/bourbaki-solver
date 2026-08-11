// Package pagemap builds the map from a PDF's own page numbering to the page
// numbering Bourbaki printed. Cross-references in the Éléments point at printed
// pages, so without this map nothing in the corpus can resolve a reference.
//
// The library prints its page numbers three different ways, and most of it is
// scans whose embedded text layer is somebody else's OCR:
//
//	Algebra chapter 8, 2023      "A VIII.13" in the running head, per chapter
//	Algebra chapters 4 to 7      "A.IV.3" in the running head, per chapter
//	Theory of Sets, French       "TS I.77" in the running head, per chapter
//	Integration, French          "INT IV.43" in the running head, per chapter
//	Algebra chapters 1 to 3      a bare number at the foot, continuous 1 to 710
//	Functions of a Real Variable a bare number at the outer edge of the head,
//	                             continuous, with "Ch. I" at the inner edge
//
// The prefix is a property of the printing and not of the Book: the English
// Topological Vector Spaces prints TVS where the French prints EVT. So it is
// measured off the volume rather than kept in a table, and a run of heads that
// disagrees with the volume's own dominant prefix is thrown away.
//
// So the reader is configurable and the fitter is not allowed to trust any one
// reading. Printed numbers run in step with PDF pages, which means the offset
// between them is constant over long stretches and steps only where the binding
// carries an unnumbered leaf. Fitting that piecewise-constant offset is what
// separates a genuine step in the printing from an OCR misread, and there are
// plenty of the latter: the 2003 scan yields "A. V. 33 4" for A.V.134 and
// "A. V. 3 02" for A.V.102.
package pagemap

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// Grammar is how a volume prints its page number.
type Grammar string

const (
	// HeadLabel is a full Bourbaki page label in the running head, "A VIII.13".
	HeadLabel Grammar = "head-label"
	// FootNumber is a bare arabic number at the foot of the page, with the
	// chapter numeral in the head on verso pages only.
	FootNumber Grammar = "foot-number"
	// HeadNumber is a bare arabic number at the outer edge of the running head,
	// with no Book prefix and no chapter numeral on it. The English Functions
	// of a Real Variable prints this: "§ 4.  EXERCISES  45" on the recto and
	// "46  DERIVATIVES  Ch. I" on the verso. It is FootNumber's pagination in
	// HeadLabel's position, so neither of the other two readers finds it.
	HeadNumber Grammar = "head-number"
)

// Pagination is what the printed number counts.
type Pagination string

const (
	// PerChapter restarts at 1 in every chapter, so a page needs its chapter to
	// be identified. This is the convention every modern printing uses.
	PerChapter Pagination = "per-chapter"
	// Continuous runs across the whole volume. The 1998 printing of chapters I
	// to III does this, and correspondingly cites by section and no. rather
	// than by page.
	Continuous Pagination = "continuous"
)

// Confidence says where an entry's page number came from. Anything other than
// head or foot was worked out rather than read, and the audit treats it as
// weaker evidence.
type Confidence string

const (
	// FromHead was read off the running head.
	FromHead Confidence = "head"
	// FromFoot was read off the foot of the page.
	FromFoot Confidence = "foot"
	// Interpolated was not printed on the page, or was misread, and follows
	// from the fitted offset of the pages around it.
	Interpolated Confidence = "interpolated"
	// Unknown means the page sits outside every fitted stretch. Front matter
	// lands here, which is correct: it carries no Bourbaki page number.
	Unknown Confidence = "none"
)

// Printed reports whether the number was read off the page rather than worked
// out from its neighbours.
func (c Confidence) Printed() bool { return c == FromHead || c == FromFoot }

// Entry is one PDF page.
type Entry struct {
	PDFPage    int
	Chapter    string
	Page       int // the printed page number, 0 when unknown
	Confidence Confidence
	Raw        string // the line the number was read from, empty otherwise
}

// Conflict is a page whose printed number was legible but disagrees with the
// offset fitted from its neighbours. Every one of these is either an OCR
// misread or a mistake in the fit, and both are worth looking at, so they are
// published rather than silently dropped.
type Conflict struct {
	PDFPage     int    `json:"pdf_page"`
	ReadChapter string `json:"read_chapter,omitempty"`
	Read        int    `json:"read"`
	Chapter     string `json:"chapter,omitempty"`
	Fitted      int    `json:"fitted"`
	Raw         string `json:"raw"`
}

// String describes a conflict the way it reads in the book.
func (c Conflict) String() string {
	read, fitted := strconv.Itoa(c.Read), strconv.Itoa(c.Fitted)
	if c.ReadChapter != "" {
		read = c.ReadChapter + "." + read
	}
	if c.Chapter != "" {
		fitted = c.Chapter + "." + fitted
	}
	return fmt.Sprintf("pdf %d read %s, fitted %s: %s", c.PDFPage, read, fitted, c.Raw)
}

// Gap is a run of consecutive PDF pages that carried no legible page number.
type Gap struct {
	From       int        `json:"from"`
	To         int        `json:"to"`
	Pages      int        `json:"pages"`
	Confidence Confidence `json:"confidence"`
}

// Step is a place where the offset between PDF page and printed page changes.
// It means the printing carries a page the PDF does not, normally a blank leaf
// dropped in production. Chapter 8 of the 2023 volume prints pages 1 to 490 on
// 488 PDF pages for exactly this reason, and knowing which two numbers are
// absent is the difference between a consistent map and an off-by-two nobody
// can explain.
//
// The unnumbered page immediately before a step is the one place the map cannot
// be certain of. Chapter 8 prints 467 on PDF page 484 and 470 on 486, and the
// page between them carries no head, so it is either 468 or 469 and nothing in
// the text says which. It is recorded as the lower of the two and left at
// confidence interpolated.
type Step struct {
	AtPDFPage    int    `json:"at_pdf_page"`
	Chapter      string `json:"chapter,omitempty"`
	FromOffset   int    `json:"from_offset"`
	ToOffset     int    `json:"to_offset"`
	MissingPages []int  `json:"missing_pages"`
}

// Span is the stretch of PDF pages one chapter occupies.
type Span struct {
	Chapter   string `json:"chapter"`
	FirstPDF  int    `json:"first_pdf_page"`
	LastPDF   int    `json:"last_pdf_page"`
	FirstPage int    `json:"first_page"`
	LastPage  int    `json:"last_page"`
}

// Map is the finished map for one volume.
type Map struct {
	Book       string
	Grammar    Grammar
	Pagination Pagination
	// Prefix is the Book prefix this volume prints in its page labels, "A" in
	// Algebra and "INT" in Integration, read off the volume and empty for the
	// grammars that print no label at all.
	Prefix    string
	PDFPages  int
	Entries   []Entry // one per PDF page, Entries[i].PDFPage == i+1
	Chapters  []Span
	Steps     []Step
	Conflicts []Conflict
	Gaps      []Gap
}

// Options configure a build.
type Options struct {
	Book       string
	Chapters   []string // the chapters this volume contains, in printed order
	Grammar    Grammar
	Pagination Pagination
	// MinRun is how many consecutive anchors have to agree on a new offset
	// before the fitter believes the printing stepped rather than the OCR
	// slipped. Zero means the default of 3.
	MinRun int
}

// DefaultMinRun is the run length that separates a step in the printing from a
// misread. Two is not enough: the 2003 scan produces adjacent misreads.
const DefaultMinRun = 3

// NativeMinRun is the run length for a volume whose text layer was written by
// the typesetter rather than by somebody's OCR. There is nothing to misread in
// such a volume, so two anchors that agree are two anchors that agree, and
// demanding three throws away the short stretches at the back of the book: the
// English Lie groups chapters 7 to 9 steps twice in its indexes, each time with
// only two numbered pages between the steps.
const NativeMinRun = 2

// SplitPages splits a whole-document pdftotext dump into pages. pdftotext ends
// every page with a form feed, including the last, so a plain split leaves a
// final empty element that is not a page.
func SplitPages(text string) []string {
	pages := strings.Split(text, "\f")
	if n := len(pages); n > 0 && strings.TrimSpace(pages[n-1]) == "" {
		pages = pages[:n-1]
	}
	return pages
}

var (
	// The opener prints the words on a line of their own. The table of contents
	// prints "CHAPTER I. ALGEBRAIC STRUCTURES ..... 1" on one line, so
	// anchoring the end of the line is what keeps the contents out.
	//
	// The French volumes print "CHAPITRE II", and the 2015 Theories spectrales
	// prints it in lower case as "chapitre ii", so the match is case insensitive
	// and the numeral is upper cased afterwards.
	chapterOpenerRe = regexp.MustCompile(`(?i)^\s*chap(?:ter|itre)\s+([ivxlcdm]+)\s*$`)

	// The 1998 scan reads the digit 1 as a capital I or a lower case l often
	// enough to matter: page 111 comes out "Ill", 251 comes out "25I" and 616
	// comes out "6I6". The foot of a page in that volume is never a Roman
	// numeral, so letting those characters stand for 1 costs nothing there, and
	// it is worth 8 pages of the map. A trailing speck of dirt is common enough
	// to allow too, since page 384 reads "384·".
	bareNumberRe = regexp.MustCompile(`^\s*([0-9IlO|]{1,4})\s*[.,·]?\s*$`)

	// headLabelRe reads a page label out of a running head. It is looser than
	// corpus.ParsePageLabel on purpose: the 2003 scan's text layer splits the
	// label, giving "A.IV.3 8" for A.IV.38 and "A.I V. 4 7" for A.IV.47, so
	// whitespace inside the chapter numeral and inside the number is stripped
	// before either is believed. The prose parser must not do that, because in
	// prose the next token after a page reference is another reference.
	//
	// The Book prefix is one to three letters. It was one letter for as long as
	// every volume in scope was Algebra and printed "A", and that made INT IV.43,
	// TS I.77, TA I.104 and TVS III.8 invisible: five of the twelve Books print
	// a multi-letter prefix, so the single letter was not a simplification, it
	// was a volume of Integration silently failing to map.
	//
	// The chapter numeral admits the digits the scanners produce for it. The
	// French Fonctions d'une variable réelle prints "FVR III.10" and its text
	// layer gives "FVR 111.10", so a numeral that is all ones is the ordinary
	// case rather than a curiosity.
	//
	// Anything this recovers still has to agree with the offset fitted from the
	// pages around it, so a loose read cannot put a wrong number in the map, it
	// can only turn an interpolation into a reading or into a published
	// conflict.
	headLabelRe = regexp.MustCompile(
		`(?:^|\s)(?P<book>[A-Z]{1,3})\s*[.\s]\s*(?P<ch>[IVXLCDM1l|][IVXLCDM1l|\s]{0,4})[.,]\s*(?P<p>\d[\d\s]{0,4})[.,]?(?:\s|$)`)

	// headNumberRe reads a bare page number at the outer edge of a running head,
	// which is where the English Functions of a Real Variable prints it. The
	// number is at the start of the line on a verso and at the end on a recto,
	// and nothing else on the line is a bare integer, so anchoring both ends is
	// what keeps a section locator or a formula out.
	headNumberRe = regexp.MustCompile(`^\s*(?P<lead>\d{1,4})\s|\s(?P<trail>\d{1,4})\s*$`)

	// headChapterRe reads the chapter numeral a head prints in words, "Ch. I".
	// Only Functions of a Real Variable does this, on its verso pages, and it is
	// worth reading because a continuously paginated volume otherwise has to
	// infer its chapter boundaries from the openers alone.
	headChapterRe = regexp.MustCompile(`\bCh\.?\s*([IVXLCDM]+)\b`)
)

// romanFixer undoes the substitutions the scanners make on a chapter numeral.
// It is the mirror of digitFixer: in a page number a Roman I is really the digit
// 1, and in a chapter numeral the digit 1 is really a Roman I. Applying either
// to the other field would be exactly wrong, which is why they are two.
var romanFixer = strings.NewReplacer("1", "I", "l", "I", "|", "I")

// digitFixer undoes the substitutions the scanners make on a page number.
// Nothing it produces is taken on trust: a reading has to agree with the offset
// fitted from the pages around it before it counts, so the worst a wrong fix
// can do is turn an interpolated page into a published conflict.
var digitFixer = strings.NewReplacer("I", "1", "l", "1", "|", "1", "O", "0")

// readNumber strips the spaces the scan inserted inside a number and reads it,
// repairing letters that stand for digits.
func readNumber(s string) (int, bool) {
	s = strings.Join(strings.Fields(s), "")
	n, err := strconv.Atoi(digitFixer.Replace(s))
	if err != nil || n <= 0 {
		return 0, false
	}
	return n, true
}

// readHeadLabel finds a page label in one running head line, and reports which
// Book printed it. The prefix is returned rather than checked here because no
// single line can say what this volume's prefix is; readAnchors decides that
// from the volume as a whole.
func readHeadLabel(line string) (prefix, chapter string, page int, ok bool) {
	m := headLabelRe.FindStringSubmatch(line)
	if m == nil {
		return "", "", 0, false
	}
	chapter = romanFixer.Replace(strings.Join(strings.Fields(m[2]), ""))
	p, ok := readNumber(m[3])
	if !ok {
		return "", "", 0, false
	}
	if _, err := corpus.RomanOrder(chapter); err != nil {
		return "", "", 0, false
	}
	return m[1], chapter, p, true
}

// readHeadNumber finds a bare page number at either edge of a running head, and
// the chapter numeral if the head prints one.
func readHeadNumber(line string) (chapter string, page int, ok bool) {
	if c := headChapterRe.FindStringSubmatch(line); c != nil {
		chapter = strings.ToUpper(c[1])
	}
	m := headNumberRe.FindStringSubmatch(line)
	if m == nil {
		return "", 0, false
	}
	// The alternation means exactly one of the two groups is set, and which one
	// says whether this is a verso or a recto, which nothing downstream needs.
	raw := m[1]
	if raw == "" {
		raw = m[2]
	}
	p, ok := readNumber(raw)
	if !ok {
		return "", 0, false
	}
	return chapter, p, true
}

// headLines returns the first n non-blank lines of a page.
func headLines(page string, n int) []string {
	var out []string
	for _, l := range strings.Split(page, "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		out = append(out, l)
		if len(out) == n {
			break
		}
	}
	return out
}

// footLine returns the last non-blank line of a page. Only the last one: a
// display formula can leave a bare integer on the second to last line, and page
// 350 of the 1998 scan ends "1" then "326", so looking any further up reads the
// tail of an equation as the page number.
func footLine(page string) string {
	lines := strings.Split(page, "\n")
	for i := len(lines) - 1; i >= 0; i-- {
		if strings.TrimSpace(lines[i]) != "" {
			return lines[i]
		}
	}
	return ""
}

// anchor is a page number read off a page, before any of it is believed.
type anchor struct {
	pdfPage int
	chapter string // empty when the grammar does not print one on the page
	page    int
	src     Confidence
	raw     string
	prefix  string // the Book prefix on the label, empty for the bare-number grammars
}

// readAnchors pulls one candidate page number per page. chapters restricts
// which Roman numerals count, so that a cross-reference to another volume in a
// running head does not become an anchor.
func readAnchors(pages []string, g Grammar, chapters []string) []anchor {
	as, _ := readAnchorsPrefix(pages, g, chapters)
	return as
}

// readAnchorsPrefix is readAnchors, and also reports the Book prefix the volume
// prints, which is empty for the two grammars that print no prefix.
//
// The prefix is measured rather than configured. A table of Book to prefix
// would have to say that the English Topological Vector Spaces prints TVS where
// the French Espaces vectoriels topologiques prints EVT, which makes it a table
// of printings and not of Books, and it would still be a guess for the thirty
// volumes nobody has opened. Reading it off the volume costs nothing extra, and
// it is what lets a head citing another Book be rejected: the alternative is
// accepting any letters, and then "TG I.4" in a cross-reference inside a running
// head of Algebra is an anchor.
func readAnchorsPrefix(pages []string, g Grammar, chapters []string) ([]anchor, string) {
	want := map[string]bool{}
	for _, c := range chapters {
		want[strings.ToUpper(c)] = true
	}
	var as []anchor
	prefixes := map[string]int{}
	for i, pg := range pages {
		n := i + 1
		switch g {
		case HeadLabel:
			for _, l := range headLines(pg, 2) {
				pre, ch, p, ok := readHeadLabel(l)
				if !ok || !want[ch] {
					continue
				}
				prefixes[pre]++
				as = append(as, anchor{pdfPage: n, chapter: ch, page: p,
					src: FromHead, raw: strings.TrimSpace(l), prefix: pre})
				break
			}
		case HeadNumber:
			hl := headLines(pg, 1)
			if len(hl) == 0 {
				continue
			}
			ch, p, ok := readHeadNumber(hl[0])
			if !ok {
				continue
			}
			if ch != "" && !want[ch] {
				ch = ""
			}
			as = append(as, anchor{pdfPage: n, chapter: ch, page: p,
				src: FromHead, raw: strings.TrimSpace(hl[0])})
		case FootNumber:
			l := footLine(pg)
			m := bareNumberRe.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			p, ok := readNumber(m[1])
			if !ok {
				continue
			}
			as = append(as, anchor{pdfPage: n, page: p, src: FromFoot, raw: strings.TrimSpace(l)})
		}
	}
	if g != HeadLabel {
		return as, ""
	}
	prefix := dominant(prefixes)
	kept := as[:0]
	for _, a := range as {
		if a.prefix == prefix {
			kept = append(kept, a)
		}
	}
	return kept, prefix
}

// dominant is the most frequent key, ties broken alphabetically so that the
// answer does not depend on map order.
func dominant(counts map[string]int) string {
	best, bestN := "", 0
	for k, n := range counts {
		if n > bestN || (n == bestN && k < best) {
			best, bestN = k, n
		}
	}
	return best
}

// readChapterStarts finds the pages that open a chapter. The opener prints
// "CHAPTER III" and nothing else identifies it, so this is the only exact
// boundary available for a continuously paginated volume.
func readChapterStarts(pages []string, chapters []string) map[int]string {
	want := map[string]bool{}
	for _, c := range chapters {
		want[strings.ToUpper(c)] = true
	}
	starts := map[int]string{}
	seen := map[string]bool{}
	for i, pg := range pages {
		for _, l := range headLines(pg, 3) {
			m := chapterOpenerRe.FindStringSubmatch(l)
			if m == nil {
				continue
			}
			c := strings.ToUpper(m[1])
			// A chapter opens once. If the words turn up again they belong to
			// a cross-reference, not to a new chapter.
			if !want[c] || seen[c] {
				continue
			}
			seen[c] = true
			starts[i+1] = c
		}
	}
	return starts
}

// Detect guesses the grammar by reading the volume every way and taking the one
// that yields most anchors. The counts are not close: the 1998 scan gives 695
// foot numbers and no head labels at all.
//
// HeadLabel does not have to win outright, only to come close, because it is
// the one grammar that reads a chapter off the page as well as a number and the
// other two have to infer chapter boundaries from the openers. The French
// Topologie algebrique is why: it prints "TA II.214" in the head and 214 at the
// foot as well, so the foot reader finds a few more pages and would have thrown
// away every chapter in the volume to get them.
//
// HeadNumber is the loosest of the three, a bare integer at either edge of one
// line, so it finds something on many pages of a volume that is really printed
// one of the other two ways. It only wins where the other two find almost
// nothing.
func Detect(pages []string, chapters []string) Grammar {
	head := len(readAnchors(pages, HeadLabel, chapters))
	foot := len(readAnchors(pages, FootNumber, chapters))
	headNum := len(readAnchors(pages, HeadNumber, chapters))
	best, n := FootNumber, foot
	if headNum > n {
		best, n = HeadNumber, headNum
	}
	if head*5 >= n*4 {
		return HeadLabel
	}
	return best
}

// detectPagination decides whether the printed number restarts at 1 in every
// chapter, which can only be asked of a grammar that prints the chapter.
//
// It is a question and not a constant because the twenty-first century volumes
// changed the answer. Algebra chapter 8 prints A VIII.1 to A VIII.490 and the
// number belongs to the chapter. Theories spectrales prints TS I.1 to TS I.197
// and then TS II.200, so the numeral in the label says which chapter the page is
// in while the number itself counts the volume. Taking the older convention for
// both put every page of chapter I of that volume in chapter II.
func detectPagination(as []anchor) Pagination {
	lowest := map[string]int{}
	var order []string
	for _, a := range as {
		if a.chapter == "" {
			continue
		}
		n, seen := lowest[a.chapter]
		if !seen {
			order = append(order, a.chapter)
			lowest[a.chapter] = a.page
			continue
		}
		if a.page < n {
			lowest[a.chapter] = a.page
		}
	}
	// One chapter cannot tell the two apart, and nothing downstream depends on
	// which it is called, so it keeps the older convention.
	if len(order) < 2 {
		return PerChapter
	}
	// A chapter opener carries no running head and neither does the first page
	// of its first section, so the lowest number read off a restarting chapter
	// is 2 or 3 as often as it is 1.
	restarts := 0
	for _, ch := range order[1:] {
		if lowest[ch] <= 3 {
			restarts++
		}
	}
	if 2*restarts >= len(order)-1 {
		return PerChapter
	}
	return Continuous
}

// chapterStartsFromAnchors is where each chapter begins, taken from the labels
// rather than from the openers. It is exact where readChapterStarts is a guess,
// because every labelled page says which chapter it is in, and it works on the
// French volumes without the opener having to be recognised at all.
//
// A chapter begins on the page after the last page of the one before it, not on
// its own first labelled page: the opener itself prints no head, and it belongs
// to the chapter it opens.
func chapterStartsFromAnchors(as []anchor) map[int]string {
	starts := map[int]string{}
	seen := map[string]bool{}
	prev := ""
	prevPage := 0
	for _, a := range as {
		if a.chapter == "" || a.chapter == prev {
			if a.chapter == prev {
				prevPage = a.pdfPage
			}
			continue
		}
		if seen[a.chapter] {
			continue
		}
		seen[a.chapter] = true
		// The first chapter reaches back over everything before its first
		// labelled page, which is the front matter and its own opener.
		at := 1
		if prev != "" {
			at = prevPage + 1
		}
		starts[at] = a.chapter
		prev, prevPage = a.chapter, a.pdfPage
	}
	return starts
}

// segment is a stretch of anchors that agree on one offset.
type segment struct {
	first, last int // indices into the anchor slice
	offset      int
}

// fitOffsets fits a piecewise-constant offset to anchors that are already
// sorted by PDF page. A disagreement opens a new segment only when minRun
// consecutive anchors back it up; a lone disagreement is a misread and is
// returned as an outlier instead.
func fitOffsets(as []anchor, minRun int) (segs []segment, outliers []int) {
	if minRun < 1 {
		minRun = 1
	}
	i := 0
	for i < len(as) {
		off := as[i].pdfPage - as[i].page
		if !runAgrees(as, i, minRun, off) {
			outliers = append(outliers, i)
			i++
			continue
		}
		seg := segment{first: i, last: i, offset: off}
		for i < len(as) {
			if as[i].pdfPage-as[i].page == off {
				seg.last = i
				i++
				continue
			}
			if runAgrees(as, i, minRun, as[i].pdfPage-as[i].page) {
				break // the printing really did step here
			}
			outliers = append(outliers, i)
			i++
		}
		segs = append(segs, seg)
	}
	return segs, outliers
}

// runAgrees reports whether an offset has enough support at i to be believed as
// a real step in the printing.
//
// It counts agreeing anchors in a window rather than demanding n in a row,
// because a misread lands in the middle of a perfectly good stretch and would
// otherwise stop the fitter from ever opening a segment there. Near the end of
// the volume there may be fewer than n anchors left, and then all of them have
// to agree.
func runAgrees(as []anchor, i, n, off int) bool {
	left := len(as) - i
	if left < n {
		n = left
	}
	end := min(i+2*n, len(as))
	agree := 0
	for j := i; j < end; j++ {
		if as[j].pdfPage-as[j].page == off {
			agree++
		}
	}
	return agree >= n
}

// cover is a stretch of PDF pages whose printed number is pdfPage minus offset.
type cover struct {
	from, to int
	offset   int
	chapter  string
}

// Build reads the anchors, fits the offsets and fills in every PDF page.
func Build(pages []string, opt Options) (*Map, error) {
	if len(pages) == 0 {
		return nil, fmt.Errorf("pagemap: no pages")
	}
	if opt.Grammar == "" {
		opt.Grammar = Detect(pages, opt.Chapters)
	}
	if opt.MinRun == 0 {
		opt.MinRun = DefaultMinRun
	}

	as, prefix := readAnchorsPrefix(pages, opt.Grammar, opt.Chapters)
	if opt.Pagination == "" {
		// A grammar that prints no chapter on the page cannot be numbering by
		// chapter, because there would be no way to tell two page 40s apart. The
		// label grammar has to be asked.
		opt.Pagination = Continuous
		if opt.Grammar == HeadLabel {
			opt.Pagination = detectPagination(as)
		}
	}
	m := &Map{Book: opt.Book, Grammar: opt.Grammar, Pagination: opt.Pagination,
		Prefix: prefix, PDFPages: len(pages)}

	var covers []cover
	switch opt.Pagination {
	case Continuous:
		// Where the page says which chapter it is in, that is the boundary. The
		// openers are the fallback for the grammars that print no chapter, and
		// they are a fallback because a volume whose opener is set in a way the
		// pattern does not match comes out as one unnamed chapter.
		starts := readChapterStarts(pages, opt.Chapters)
		if opt.Grammar == HeadLabel {
			starts = chapterStartsFromAnchors(as)
		}
		covers = coverContinuous(as, len(pages), starts, opt)
	default:
		covers = coverPerChapter(as, opt)
	}

	byPDF := map[int]anchor{}
	for _, a := range as {
		byPDF[a.pdfPage] = a
	}
	m.Entries = make([]Entry, len(pages))
	for i := range m.Entries {
		n := i + 1
		e := Entry{PDFPage: n, Confidence: Unknown}
		if c, ok := findCover(covers, n); ok {
			e.Chapter = c.chapter
			e.Page = n - c.offset
			e.Confidence = Interpolated
			if a, ok := byPDF[n]; ok {
				// The reading counts only when it agrees with the fit on both
				// the chapter and the page. A reading of "A.I V.14" splits the
				// numeral and lands the page in the wrong chapter with the
				// right number, which is a disagreement even though the number
				// matches.
				if a.page == e.Page && (a.chapter == "" || a.chapter == e.Chapter) {
					e.Confidence = a.src
					e.Raw = a.raw
				} else {
					m.Conflicts = append(m.Conflicts, Conflict{
						PDFPage: n, ReadChapter: a.chapter, Read: a.page,
						Chapter: e.Chapter, Fitted: e.Page, Raw: a.raw})
				}
			}
		}
		m.Entries[i] = e
	}
	m.Chapters = chapterSpans(m.Entries, opt.Chapters)
	m.Steps = findSteps(covers)
	m.Gaps = findGaps(m.Entries)
	return m, nil
}

// findSteps reports where the offset changes between two touching stretches,
// and which printed page numbers fall in the crack.
func findSteps(covers []cover) []Step {
	var steps []Step
	for i := 1; i < len(covers); i++ {
		prev, next := covers[i-1], covers[i]
		if prev.offset == next.offset || prev.to+1 != next.from {
			continue
		}
		// A chapter boundary is not a step: per-chapter numbering restarts, so
		// the offset is expected to change there.
		if prev.chapter != next.chapter {
			continue
		}
		s := Step{AtPDFPage: next.from, Chapter: next.chapter,
			FromOffset: prev.offset, ToOffset: next.offset}
		for p := prev.to - prev.offset + 1; p < next.from-next.offset; p++ {
			s.MissingPages = append(s.MissingPages, p)
		}
		steps = append(steps, s)
	}
	return steps
}

// coverPerChapter fits each chapter on its own, because the printed number
// restarts at 1 and so does the offset.
func coverPerChapter(as []anchor, opt Options) []cover {
	byChapter := map[string][]anchor{}
	for _, a := range as {
		byChapter[a.chapter] = append(byChapter[a.chapter], a)
	}
	order := chapterOrder(opt.Chapters)

	var covers []cover
	for _, ch := range order {
		list := byChapter[ch]
		if len(list) == 0 {
			continue
		}
		segs, _ := fitOffsets(list, opt.MinRun)
		for j, s := range segs {
			c := cover{from: list[s.first].pdfPage, to: list[s.last].pdfPage,
				offset: s.offset, chapter: ch}
			// The first printed page of a chapter is 1, and its opener carries
			// no running head, so the first stretch reaches back to it.
			if j == 0 {
				c.from = s.offset + 1
			}
			covers = append(covers, c)
		}
	}
	sort.Slice(covers, func(i, j int) bool { return covers[i].from < covers[j].from })
	return closeCracks(covers)
}

// closeCracks hands out the pages that fall between two fitted stretches. None
// of them carries a number, or it would have been an anchor, so the only
// question is which side of the crack they belong to.
//
// A chapter's last stretch runs up to the page before the next chapter starts,
// because that is where its exercises and its historical note sit, so a crack
// between two chapters goes to the left.
//
// Inside a chapter the direction of the step decides. When the offset drops the
// file is missing a page the book printed, and what the book drops is the blank
// verso in front of a division opener: the opener itself is in the file and is
// the first page of the new offset. Chapter VIII is the case in hand, where pdf
// 485 opens the historical note at printed 469 and the blank 468 is not in the
// file at all, so the crack goes to the right. When the offset rises the file
// carries pages the book never numbered, and those belong to the end of what
// came before, so the crack goes to the left.
func closeCracks(covers []cover) []cover {
	for i := 0; i+1 < len(covers); i++ {
		cur, next := &covers[i], &covers[i+1]
		if cur.to >= next.from-1 {
			continue
		}
		if cur.chapter == next.chapter && next.offset < cur.offset {
			next.from = cur.to + 1
			continue
		}
		cur.to = next.from - 1
	}
	return covers
}

// coverContinuous fits one offset across the whole volume and cuts it at the
// chapter boundaries its caller worked out, since the page number itself does
// not restart and so says nothing about where a chapter begins.
func coverContinuous(as []anchor, pdfPages int, starts map[int]string, opt Options) []cover {
	segs, _ := fitOffsets(as, opt.MinRun)
	var startPages []int
	for p := range starts {
		startPages = append(startPages, p)
	}
	sort.Ints(startPages)

	chapterAt := func(p int) string {
		ch := ""
		for _, s := range startPages {
			if s <= p {
				ch = starts[s]
			}
		}
		return ch
	}

	var covers []cover
	for j, s := range segs {
		from, to := as[s.first].pdfPage, as[s.last].pdfPage
		if j == 0 {
			from = s.offset + 1
		}
		if j == len(segs)-1 {
			to = pdfPages
		}
		// One stretch of pages can span a chapter boundary, so it is cut at
		// every opener and each piece is labelled with its own chapter.
		cuts := []int{from}
		for _, sp := range startPages {
			if sp > from && sp <= to {
				cuts = append(cuts, sp)
			}
		}
		for k, c := range cuts {
			end := to
			if k+1 < len(cuts) {
				end = cuts[k+1] - 1
			}
			covers = append(covers, cover{from: c, to: end, offset: s.offset, chapter: chapterAt(c)})
		}
	}
	return closeCracks(openersGoRight(covers, startPages))
}

// openersGoRight hands a chapter opener that fell between two fitted stretches
// to the chapter it opens.
//
// The stretch before a chapter boundary ends on the last page that printed a
// number and the one after begins on the first, and the opener is in between
// because Bourbaki prints no running head on it. Left to closeCracks it would
// go to the chapter before, which is right for a page of exercises and wrong
// for the page that says "chapitre iv" on it. The printed number changes side
// with it: in Theories spectrales chapters 1 and 2 that page is 199, the last
// number of chapter I is 197 and the first of chapter II is 200.
func openersGoRight(covers []cover, startPages []int) []cover {
	for i := 0; i+1 < len(covers); i++ {
		cur, next := &covers[i], &covers[i+1]
		if cur.chapter == next.chapter || cur.to >= next.from-1 {
			continue
		}
		for _, sp := range startPages {
			if sp > cur.to && sp < next.from {
				next.from = sp
				break
			}
		}
	}
	return covers
}

func findCover(covers []cover, p int) (cover, bool) {
	for _, c := range covers {
		if p >= c.from && p <= c.to {
			return c, true
		}
	}
	return cover{}, false
}

// chapterOrder sorts the volume's chapters by Roman value, so that a manifest
// listing them out of order still fits them in printed order.
func chapterOrder(chapters []string) []string {
	out := make([]string, 0, len(chapters))
	for _, c := range chapters {
		out = append(out, strings.ToUpper(c))
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, _ := corpus.RomanOrder(out[i])
		b, _ := corpus.RomanOrder(out[j])
		return a < b
	})
	return out
}

func chapterSpans(entries []Entry, chapters []string) []Span {
	byChapter := map[string]*Span{}
	for _, e := range entries {
		if e.Chapter == "" || e.Page == 0 {
			continue
		}
		s, ok := byChapter[e.Chapter]
		if !ok {
			byChapter[e.Chapter] = &Span{Chapter: e.Chapter, FirstPDF: e.PDFPage,
				LastPDF: e.PDFPage, FirstPage: e.Page, LastPage: e.Page}
			continue
		}
		s.LastPDF = e.PDFPage
		s.LastPage = e.Page
	}
	var out []Span
	for _, ch := range chapterOrder(chapters) {
		if s, ok := byChapter[ch]; ok {
			out = append(out, *s)
		}
	}
	return out
}

// findGaps collects the runs of pages whose number was not printed, so the
// stretches that were worked out rather than read can be inspected as a list
// instead of by scrolling the map.
func findGaps(entries []Entry) []Gap {
	var gaps []Gap
	var cur *Gap
	for _, e := range entries {
		if e.Confidence.Printed() {
			cur = nil
			continue
		}
		if cur != nil && cur.Confidence == e.Confidence && cur.To == e.PDFPage-1 {
			cur.To = e.PDFPage
			cur.Pages++
			continue
		}
		gaps = append(gaps, Gap{From: e.PDFPage, To: e.PDFPage, Pages: 1, Confidence: e.Confidence})
		cur = &gaps[len(gaps)-1]
	}
	return gaps
}

// Counts tallies the entries by confidence.
func (m *Map) Counts() map[Confidence]int {
	c := map[Confidence]int{}
	for _, e := range m.Entries {
		c[e.Confidence]++
	}
	return c
}

// BodyPages is the number of PDF pages that carry a Bourbaki page number at
// all. Coverage is reported against this rather than the whole file, because
// the front matter has no number to find and counting it as a miss would make
// every volume look worse than it is.
func (m *Map) BodyPages() int {
	n := 0
	for _, e := range m.Entries {
		if e.Confidence != Unknown {
			n++
		}
	}
	return n
}

// Lookup returns the entry for a PDF page.
func (m *Map) Lookup(pdfPage int) (Entry, bool) {
	if pdfPage < 1 || pdfPage > len(m.Entries) {
		return Entry{}, false
	}
	return m.Entries[pdfPage-1], true
}

// PDFPageOf is the inverse: which PDF page carries a printed page of a chapter.
func (m *Map) PDFPageOf(chapter string, page int) (int, bool) {
	chapter = strings.ToUpper(chapter)
	for _, e := range m.Entries {
		if e.Page == page && (chapter == "" || e.Chapter == chapter) {
			return e.PDFPage, true
		}
	}
	return 0, false
}
