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
	"slices"
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

// Pages is the two readings as the book would write them, the one on the page
// and the one the fit worked out. The chapter goes on either only where the map
// knows one, and it has to go on both or neither: a page read as VI.37 where the
// fit says VII.37 disagrees about the chapter and not about the number, and
// printing the two bare numbers says "37 was overruled by 37".
func (c Conflict) Pages() (read, fitted string) {
	read, fitted = strconv.Itoa(c.Read), strconv.Itoa(c.Fitted)
	if c.ReadChapter == "" && c.Chapter == "" {
		return read, fitted
	}
	label := func(chapter, page string) string {
		if chapter == "" {
			chapter = "?"
		}
		return chapter + "." + page
	}
	return label(c.ReadChapter, read), label(c.Chapter, fitted)
}

// String describes a conflict the way it reads in the book.
func (c Conflict) String() string {
	read, fitted := c.Pages()
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
	Prefix string
	// FirstPage is the printed page the file opens on, where the scan does not
	// start at the beginning of the volume. See Options.FirstPage.
	FirstPage int
	// Restarts are the PDF pages where the printed numbering starts over. See
	// Options.Restarts.
	Restarts []int
	// Transposed are pairs of PDF pages the file carries the wrong way round.
	// See Options.Transposed.
	Transposed [][2]int
	PDFPages   int
	Entries    []Entry // one per PDF page, Entries[i].PDFPage == i+1
	Chapters   []Span
	Steps      []Step
	Conflicts  []Conflict
	Gaps       []Gap
}

// Options configure a build.
type Options struct {
	Book       string
	Chapters   []string // the chapters this volume contains, in printed order
	Grammar    Grammar
	Pagination Pagination
	// MinRun is how many consecutive anchors have to agree on a new offset
	// before the fitter believes the printing stepped rather than the OCR
	// slipped. Zero means DefaultMinRun.
	MinRun int
	// FirstPage is the printed page the file's first page carries, for a scan
	// that does not begin at the beginning of the volume. Zero means it does.
	//
	// A per-chapter volume numbers each chapter from 1, and a chapter that does
	// not start at 1 is normally a fit that has slipped, which is what Validate
	// says. The front of a volume is also where a leaf goes missing, and the
	// scan of Fonctions d'une variable reelle in French opens on the half title
	// with FVR I.3 fitted to it, because the two leaves before it were never
	// scanned. Saying so here is what tells the two apart, and the volume has
	// to say it: nothing in the file distinguishes a missing leaf from a wrong
	// offset.
	FirstPage int
	// Restarts are the PDF pages where the printed numbering starts over,
	// because the file holds more than one separately paginated fascicule.
	//
	// The fitter finds the change of offset on its own, since a fascicule that
	// starts over is a long stretch of pages agreeing on a new one, but it
	// cannot tell where the new fascicule begins: the pages between the last
	// number of the old and the first of the new carry no head, and the divider
	// and the front matter behind it are exactly those pages. Nor can anything
	// downstream tell a restart from a fit that slipped, which is the same
	// backwards jump. Both are what the page named here settles.
	Restarts []int
	// Transposed are pairs of PDF pages the file carries the wrong way round,
	// because the leaves were bound out of order and the scan followed them.
	//
	// The pages are read in the order given here before anything else happens
	// and put back in file order afterwards, so a map is still a thing you look
	// a PDF page up in. See printingOrder for the volume this is here for.
	Transposed [][2]int
}

// DefaultMinRun is the run length that separates a step in the printing from a
// misread.
//
// It was three for the scanned volumes, because a scan produces misreads and
// two of them in a row could agree on an offset that is not there, and two for
// the born-digital ones, where there is nothing to misread. It is two for both
// now. The reason it had to be three was that the head reader accepted lines
// that are not heads at all, so a volume had misreads in quantity and adjacent
// ones were not rare; with the gap rule, the split numerals, the two-edged
// heads and the table of contents all dealt with, they are.
//
// Three throws away the short stretches at the back of the book, and that is
// where the printing steps: Algebre commutative chapitres 5 a 7 numbers its
// index terminologique and its table des matieres with two numbered pages in
// each, and Groupes et algebres de Lie chapitres 4 a 6 does the same across its
// planches. Over the 41 volumes that can be mapped, dropping to two leaves
// every committed map byte for byte as it was and lets those two fit.
const DefaultMinRun = 2

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

// headGapMin is how many spaces stand between the page number and the title in
// a running head, at least. Two is enough to tell a head from prose, since
// prose puts one space between its words, and it is as low as it can go: the
// recto heads of Integration set the number three spaces clear of the title and
// the section mark one space clear of it, which is what tells the two apart on
// the pages whose section mark the scan turned into a digit.
const headGapMin = "2"

var (
	// The opener prints the words on a line of their own. The table of contents
	// prints "CHAPTER I. ALGEBRAIC STRUCTURES ..... 1" on one line, so
	// anchoring the end of the line is what keeps the contents out.
	//
	// The French volumes print "CHAPITRE II", and the 2015 Theories spectrales
	// prints it in lower case as "chapitre ii", so the match is case insensitive
	// and the numeral is upper cased afterwards.
	//
	// The numeral admits the spaces and the letter for digit swaps the scanners
	// put inside it, as headLabelRe and headChapterRe do. Algebre chapitre 9
	// opens with "CHAPITRE I X" letter spaced across the page, which a numeral
	// read as an unbroken run of Roman letters does not match at all, and that
	// one line is the whole chapter structure of the volume: the map came out
	// with 211 pages and no chapter on any of them.
	chapterOpenerRe = regexp.MustCompile(`(?i)^\s*chap(?:ter|itre)\s+([ivxlcdm1|][ivxlcdm1| ]*?)\s*$`)

	// A French volume whose first chapter is its own book writes the numeral
	// out: Groupes et algebres de Lie chapitre 1, Theories spectrales and
	// Topologie algebrique all open CHAPITRE PREMIER and none of them opens
	// CHAPITRE I. Without this the numeral pattern finds no opener anywhere in
	// those volumes and the map comes out with no chapter on any page.
	firstOpenerRe = regexp.MustCompile(`(?i)^\s*chap(?:ter|itre)\s+premier\s*$`)

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

	// headLeadRe and headTrailRe read a bare page number at either edge of a
	// running head, which is where the English Functions of a Real Variable
	// prints it. The number is at the start of the line on a verso and at the
	// end on a recto, so both edges have to be looked at and anchoring them is
	// what keeps a formula in the middle of the line out.
	//
	// They were one pattern with an alternation in it, which meant the lead won
	// wherever both edges carried a number and the trail was never even read.
	// That is the wrong way round on a recto whose section mark the scan turned
	// into a digit, and the French printings are full of them: "§ 1  EXERCICES
	// 69" arrives as "5 1  EXERCICES  69" and the page shipped as 5.
	// The number has to stand clear of the rest of the line, which is what makes
	// it a page number rather than a word of prose that happens to be a numeral.
	// Something has to say which lines are running heads at all, and for this
	// grammar nothing did: the reader took the first non-blank line of the page
	// and looked for an integer at either edge, which is a description of most
	// lines of French mathematical prose. "Reimpression inchangee de l'edition
	// originale de 1959" was read as page 1959, "(no 7). Lorsque @ est
	// bilineaire, Q, est l'image reciproque de 5" as page 5, and "§ 2.
	// Relevement des ideaux premiers." as page 5.
	//
	// The gap is what a typesetter puts there and prose does not: Bourbaki sets
	// the page number at the outer edge of the measure and the title in the
	// middle of it, so pdftotext -layout hands back "298     Index" and "8
	// SOUS-ALGEBRES DE CARTAN. ELEMENTS REGULIERS   Ch. VII, § 1" with runs of
	// spaces in them, where a line of prose has one space between its words.
	// Setting the title in capitals would have been the other way to ask, and it
	// is the wrong one: the Elements of the History of Mathematics heads its
	// index "Index" and its bibliography "BIBLIOGRAPHY", and four pages of that
	// volume have a page number nothing else in the file can supply.
	//
	// Both edges admit the spaces the scan puts inside the number, as
	// headLabelRe does, and it is the gap that keeps that from swallowing the
	// rest of the line: the digits have to run out before the two spaces begin.
	// The exercise heads of the French printings are where this matters. They
	// set the section at the inner edge and the page at the outer one, and the
	// outer one is the one the scan breaks: "54    EXERCICES    24 1" is page
	// 241 and was read as page 54, because 24 1 is not a number and 54 is.
	headLeadRe  = regexp.MustCompile(`^\s*(\d[\d ]{0,5}?)\s{` + headGapMin + `,}`)
	headTrailRe = regexp.MustCompile(`\s{` + headGapMin + `,}(\d[\d ]{0,5}?)\s*$`)

	// headChapterRe reads the chapter numeral a head prints in words. The English
	// Functions of a Real Variable prints "Ch. I" and this was written for that
	// alone, which left every French volume of the same grammar with no chapter
	// on any page: Integration prints "Chap. V" and "chap. VII", Lie prints
	// "Ch. II" with the numeral scanned as "11", and the first of those three
	// forms is the only one an anchored capital Ch with a Roman numeral straight
	// after it matches.
	//
	// The period is required rather than optional, and the numeral has to be
	// followed by a comma or by the end of the line. Without both of those the
	// pattern reads the ch of any French word that happens to be followed by
	// something the Roman class admits, and the head of a page is exactly where
	// such a word turns up: this is a running head, not a citation, so there is
	// nothing to gain by being loose and a whole chapter to lose by being wrong.
	//
	// The numeral admits the spaces the scanners put inside it, as headLabelRe
	// does. Algebre commutative prints "chap. II" and its text layer gives
	// "chap. I I", so a numeral read up to the first space is chapter I on every
	// page of chapter II, which is 43 pages of that volume claiming to be in a
	// chapter that ended sixty pages earlier.
	headChapterRe = regexp.MustCompile(`(?i)\bch(?:ap)?\.\s*([ivxlcdm1|][ivxlcdm1| ]{0,4}?)\s*(?:[,.;]|$)`)

	// leaderRe is the row of dots a table of contents runs between a heading and
	// the page it is on. It is the one thing in a volume that looks more like a
	// running head than a running head does: a title at one edge, a number at
	// the other, and the wide gap between them that tells the two apart from
	// prose.
	//
	// A volume's own table of contents sits in the back of the book in the
	// French printings, past the last page the volume numbers, so a number read
	// off one of its lines is a reference to somewhere else entirely. Groupes
	// et algebres de Lie chapitre 1 prints its table on pdf 141 and 142, and
	// the first line of 142 ends in 58, which was read as page 58 arriving
	// directly after page 143.
	//
	// entryRe is a whole contents entry, a leader with the page it points at on
	// the end of it, and is what counts a page as a table of contents. leaderRe
	// is just the dots, and is what disqualifies a line on a page already known
	// to be one. The two differ because the scans lose most of a leader often
	// enough that the strict form alone would miss the line that matters: that
	// first line of pdf 142 has two dots where the four under it have twenty.
	// Asking the loose question of every page in the volume is what is wrong,
	// since a badly scanned proof is full of dots, and it cost four pages of
	// Lie chapters 4 to 6 their running heads.
	leaderRe = regexp.MustCompile(`\.\s*\.`)
	entryRe  = regexp.MustCompile(`\.\s*\.\s*\.\s*\.[\s.]*\s\s*\d[\d ]*$`)
)

// isContents reports whether a page is part of a volume's table of contents.
// Three entries on one page is a contents and one is a displayed formula that
// happens to end in a numeral.
func isContents(page string) bool {
	n := 0
	for _, l := range strings.Split(page, "\n") {
		if entryRe.MatchString(l) {
			n++
			if n == 3 {
				return true
			}
		}
	}
	return false
}

// headLineOf returns the line a page prints its running head on, and reports
// whether there is one to read.
//
// It is the first non-blank line, except on a table of contents, where the
// lines under the head are entries pointing at other pages and any of them can
// come first once the scan has eaten the leader. Integration chapitre 9 heads
// its own contents "TABLE DES MATIERES    133", which is a real running head on
// a real page and is kept.
func headLineOf(page string) (string, bool) {
	contents := isContents(page)
	for _, l := range headLines(page, 4) {
		if contents && leaderRe.MatchString(l) {
			continue
		}
		return l, true
	}
	return "", false
}

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

// readEdgeNumber reads the number at one edge of a running head. No volume in
// the library runs past four digits, and the edge patterns admit the spaces the
// scan puts inside a number, so the count is checked here rather than left to
// the pattern.
func readEdgeNumber(m []string) (int, bool) {
	if m == nil {
		return 0, false
	}
	s := strings.Join(strings.Fields(m[1]), "")
	if len(s) > 4 {
		return 0, false
	}
	return readNumber(s)
}

// readHeadNumber returns the number at the edge the line most likely prints the
// page at, the chapter numeral if the head prints one, and, where both edges
// carry a number, the other as an alternative for the fit to choose between.
func readHeadNumber(line string) (chapter string, page, alt int, ok bool) {
	if leaderRe.MatchString(line) {
		return "", 0, 0, false
	}
	if c := headChapterRe.FindStringSubmatch(line); c != nil {
		ch := strings.ToUpper(romanFixer.Replace(strings.Join(strings.Fields(c[1]), "")))
		if _, err := corpus.RomanOrder(ch); err == nil {
			chapter = ch
		}
	}
	lead, hasLead := readEdgeNumber(headLeadRe.FindStringSubmatch(line))
	trail, hasTrail := readEdgeNumber(headTrailRe.FindStringSubmatch(line))
	// A head that names its chapter is a verso, and a verso prints the page at
	// the inner edge and the locator at the outer one: "10  MESURES SUR LES
	// ESPACES TOPOLOGIQUES SEPARES  Ch. IX, § 1". The number at the end of such
	// a line is the § and never the page, and reading it as the page is how
	// Integration chapitre 9 came to say three separate pages were printed IX.1.
	// It happens on the pages the scan mangles: 10 arrives as "1O", which is not
	// a lead number, and the reader then took what it could find.
	if chapter != "" {
		hasTrail = false
	}
	switch {
	case hasLead && hasTrail:
		return chapter, lead, trail, true
	case hasLead:
		return chapter, lead, 0, true
	case hasTrail:
		return chapter, trail, 0, true
	}
	return "", 0, 0, false
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
	// alt is the other number on the line, where the line carries one at each
	// edge and nothing on the line says which of them is the page. It is zero
	// everywhere else. The fit decides between the two, which is the only thing
	// in the volume that can: on a recto whose section mark the scan turned into
	// a digit both readings are equally well formed, and the one that continues
	// the offset the rest of the chapter runs on is the page number.
	alt    int
	src    Confidence
	raw    string
	prefix string // the Book prefix on the label, empty for the bare-number grammars
}

// offsets is the offsets between PDF page and printed page this anchor allows,
// the reading at the edge the line most likely prints the page at first.
func (a anchor) offsets() []int {
	if a.alt == 0 {
		return []int{a.pdfPage - a.page}
	}
	return []int{a.pdfPage - a.page, a.pdfPage - a.alt}
}

// fits reports whether either reading of this anchor sits at the given offset.
//
// The fit is where a two-edged head is decided, and it has to be decided during
// the fit and not after it. Algebre commutative chapitres 5 a 7 sets its section
// mark at the inner edge of a recto and its page at the outer one, and the scan
// reads the mark as a 5, so page 305 comes back as "54  EXERCICES  305" with
// two well formed numbers on it and nothing on the line saying which is which.
// Fitting on the first reading alone put a step in the printing there that the
// printing does not have, and carried the whole back of the volume one page out.
func (a anchor) fits(off int) bool {
	for _, o := range a.offsets() {
		if o == off {
			return true
		}
	}
	return false
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
			hl, ok := headLineOf(pg)
			if !ok {
				continue
			}
			ch, p, alt, ok := readHeadNumber(hl)
			if !ok {
				continue
			}
			if ch != "" && !want[ch] {
				ch = ""
			}
			as = append(as, anchor{pdfPage: n, chapter: ch, page: p, alt: alt,
				src: FromHead, raw: strings.TrimSpace(hl)})
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
			c := ""
			switch m := chapterOpenerRe.FindStringSubmatch(l); {
			case m != nil:
				c = strings.ToUpper(romanFixer.Replace(strings.Join(strings.Fields(m[1]), "")))
			case firstOpenerRe.MatchString(l):
				c = "I"
			default:
				continue
			}
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
	lowest, highest := map[string]int{}, map[string]int{}
	var order []string
	for _, a := range as {
		if a.chapter == "" {
			continue
		}
		if _, seen := lowest[a.chapter]; !seen {
			order = append(order, a.chapter)
			lowest[a.chapter], highest[a.chapter] = a.page, a.page
			continue
		}
		if a.page < lowest[a.chapter] {
			lowest[a.chapter] = a.page
		}
		if a.page > highest[a.chapter] {
			highest[a.chapter] = a.page
		}
	}
	// One chapter cannot tell the two apart, and nothing downstream depends on
	// which it is called, so it keeps the older convention.
	if len(order) < 2 {
		return PerChapter
	}
	// A chapter that numbers straight on from the one before it starts above
	// where that one ended, and a chapter that restarts starts below it. That
	// is the question, and asking it this way needs nothing from the pages a
	// volume happens not to print a head on.
	//
	// Asking instead whether the lowest number read in a chapter is 1, 2 or 3
	// is what this used to do, and it is a question about the scan rather than
	// about the printing. Topologie generale sets the head of its first pages
	// of chapter III as "T G 111.2", with the book letters apart, which reads
	// as prefix G and is dropped with every other page that is not the volume's
	// own prefix, and the first four pages of chapter IV have no head in the
	// text layer at all. The lowest number the reader saw in those two chapters
	// was 5, so a volume whose four chapters run 1 to 127, 1 to 44, 1 to 88 and
	// 1 to 96 was called continuously paginated, which decides whether a page
	// of it is labelled TG III.5 or just 5.
	restarts := 0
	for i, ch := range order[1:] {
		if lowest[ch] < highest[order[i]] {
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

// anchorsNameChapters reports whether the pages of this volume say which
// chapter they are in.
func anchorsNameChapters(as []anchor) bool {
	for _, a := range as {
		if a.chapter != "" {
			return true
		}
	}
	return false
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
		off, ok := 0, false
		for _, cand := range as[i].offsets() {
			if runAgrees(as, i, minRun, cand) {
				off, ok = cand, true
				break
			}
		}
		if !ok {
			outliers = append(outliers, i)
			i++
			continue
		}
		seg := segment{first: i, last: i, offset: off}
		for i < len(as) {
			if as[i].fits(off) {
				seg.last = i
				i++
				continue
			}
			stepped := false
			for _, cand := range as[i].offsets() {
				if runAgrees(as, i, minRun, cand) {
					stepped = true
					break
				}
			}
			if stepped {
				break // the printing really did step here
			}
			outliers = append(outliers, i)
			i++
		}
		segs = append(segs, seg)
	}
	return believeLoneAnchors(as, segs, outliers)
}

// believeLoneAnchors takes back the outliers that are not misreads.
//
// The run rule is about misreads, and a misread is a page whose number came out
// wrong while the pages around it came out right. It is not the only way an
// anchor ends up alone. A stretch of pages that print no number at all, with
// one that does in the middle of it, gives a reading with nothing to agree with
// it however good the reading is, and throwing it away throws away the only
// evidence there is about that part of the volume.
//
// What tells the two apart is arithmetic. The fit already has an offset before
// the anchor and an offset after it, and between them the printing carries
// pages the file does not. A reading that lands strictly inside that step is
// consistent with everything the fit knows: it says the missing pages are not
// all in one place, which is what a step of three pages across twelve normally
// means. A misread lands anywhere, and the odds of it landing inside the step
// and nowhere else are what makes this worth doing.
//
// Groupes et algebres de Lie chapitres 4 a 6 is the volume it is here for. Its
// planches run from pdf 248 to pdf 270 and print a number on one page in
// eleven: pdf 247 heads 248, pdf 259 heads 262 and pdf 261 heads 265. The fit
// put the whole step of three at pdf 248, overruled the reading on 259 and
// refused the volume. Two pages of the printing are missing before pdf 259 and
// one after it, and the reading on 259 is what says so.
//
// Two outliers between the same pair of segments are two readings that disagree
// with each other, since two that agreed would have made a segment of their
// own, and neither is taken.
func believeLoneAnchors(as []anchor, segs []segment, outliers []int) ([]segment, []int) {
	var kept []int
	var lone []segment
	for n, i := range outliers {
		if (n > 0 && between(segs, outliers[n-1]) == between(segs, i)) ||
			(n+1 < len(outliers) && between(segs, outliers[n+1]) == between(segs, i)) {
			kept = append(kept, i)
			continue
		}
		b := between(segs, i)
		if b < 0 {
			kept = append(kept, i)
			continue
		}
		lo, hi := segs[b].offset, segs[b+1].offset
		off, ok := 0, false
		for _, cand := range as[i].offsets() {
			if (cand-lo)*(hi-cand) > 0 {
				off, ok = cand, true
				break
			}
		}
		if !ok {
			kept = append(kept, i)
			continue
		}
		lone = append(lone, segment{first: i, last: i, offset: off})
	}
	if len(lone) == 0 {
		return segs, outliers
	}
	segs = append(segs, lone...)
	sort.Slice(segs, func(i, j int) bool { return segs[i].first < segs[j].first })
	return segs, kept
}

// between is the index of the segment that ends before anchor i, where the one
// that begins after it is the very next segment. It is -1 where the anchor is
// in front of every segment, behind every segment, or inside one.
func between(segs []segment, i int) int {
	for k := 0; k+1 < len(segs); k++ {
		if segs[k].last < i && i < segs[k+1].first {
			return k
		}
	}
	return -1
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
		if as[j].fits(off) {
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
	// Everything from here to the end of the fit works in printing order, the
	// order the volume reads in, which is the file's own order except in a scan
	// whose leaves were bound the wrong way round. The offsets are what the fit
	// is about and they only mean anything in that order. The entries, the
	// steps and the conflicts are put back into file order at the end, because
	// a map is a thing you look a PDF page up in.
	order, err := printingOrder(len(pages), opt.Transposed)
	if err != nil {
		return nil, err
	}
	at := make([]int, len(pages)+1)
	for i, p := range order {
		at[p] = i + 1
	}
	read := make([]string, len(pages))
	for i, p := range order {
		read[i] = pages[p-1]
	}
	pages = read
	declared := opt.Restarts
	restarts := make([]int, 0, len(declared))
	for _, r := range declared {
		if r >= 1 && r < len(at) {
			r = at[r]
		}
		restarts = append(restarts, r)
	}
	opt.Restarts = restarts

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
		Prefix: prefix, FirstPage: opt.FirstPage, Restarts: declared,
		Transposed: opt.Transposed, PDFPages: len(pages)}

	var covers []cover
	switch opt.Pagination {
	case Continuous:
		// Where the page says which chapter it is in, that is the boundary. The
		// openers are the fallback for the grammars that print no chapter, and
		// they are a fallback because a volume whose opener is set in a way the
		// pattern does not match comes out as one unnamed chapter.
		starts := readChapterStarts(pages, opt.Chapters)
		// An opener is where the chapter begins and a head is only a bound on
		// it, so the openers are used whenever they account for every chapter
		// the volume has. What the heads give is the page after the last page
		// of the chapter before, and the pages in between are the opener, its
		// blank verso and whatever else carries no running head: Lie 7 to 9
		// puts chapter VIII at printed 69 and the heads put it at 67, and
		// Functions of a Real Variable is out by two at the front and by
		// thirty four in the middle.
		//
		// Where the openers fall short the heads are all there is, and they are
		// worth having. A French head reads "10 MESURES SUR LES ESPACES
		// TOPOLOGIQUES SEPARES Ch. IX, § 1" and names its chapter as plainly as
		// "INT IX.10" does, and on the scans whose opener line the OCR mangled
		// that is the difference between a volume with chapters and a volume
		// that is one unnamed run: Lie chapitres 7 et 8 found no opener at all.
		//
		// This holds for the label grammar as much as for the bare-number ones,
		// though it did not used to. A labelled page says which chapter it is
		// in and that looked exact enough to prefer, but what stands between
		// the last labelled page of a chapter and the opener of the next is the
		// back matter of the chapter that is ending: Topologie generale prints
		// a NOTE HISTORIQUE and a BIBLIOGRAPHIE there and neither carries a
		// page number. Reading the heads put those three pages of chapter II at
		// the front of chapter III and gave them printed pages -2, -1 and 0.
		if len(starts) < len(opt.Chapters) && anchorsNameChapters(as) {
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
				//
				// The chapter counts as agreeing where the fit has none. A
				// continuously paginated volume takes its chapter from the
				// openers, and where no opener was recognised the fit knows no
				// chapter at all; a head that names one is then adding to the
				// fit rather than contradicting it, and calling that a conflict
				// cost Lie chapitres 7 et 8 every one of its twenty readings.
				agrees := func(read int) bool {
					return read == e.Page && (a.chapter == "" || e.Chapter == "" || a.chapter == e.Chapter)
				}
				switch {
				case agrees(a.page):
					e.Confidence = a.src
					e.Raw = a.raw
				case a.alt != 0 && agrees(a.alt):
					e.Confidence = a.src
					e.Raw = a.raw
				default:
					m.Conflicts = append(m.Conflicts, Conflict{
						PDFPage: n, ReadChapter: a.chapter, Read: a.page,
						Chapter: e.Chapter, Fitted: e.Page, Raw: a.raw})
				}
			}
		}
		m.Entries[i] = e
	}
	m.Steps = findSteps(covers, opt.Restarts)

	// Back into file order. Every page keeps the number the fit gave it and
	// goes to the row a reader will look it up in.
	if len(opt.Transposed) > 0 {
		inFile := make([]Entry, len(m.Entries))
		for i, e := range m.Entries {
			e.PDFPage = order[i]
			inFile[order[i]-1] = e
		}
		m.Entries = inFile
		for i, c := range m.Conflicts {
			m.Conflicts[i].PDFPage = order[c.PDFPage-1]
		}
		for i, s := range m.Steps {
			m.Steps[i].AtPDFPage = order[s.AtPDFPage-1]
		}
	}
	m.Chapters = chapterSpans(m.Entries, opt.Chapters)
	m.Gaps = findGaps(m.Entries)
	return m, nil
}

// printingOrder is the PDF pages in the order the volume reads, so that the
// page standing at printing position i is order[i-1].
//
// It is the identity for every volume but the ones a binder got wrong. The
// French Algebre chapitres 4 a 7 is one: its pdf 274 heads A V.169 and carries
// the end of the exercises of chapter V, and its pdf 273 is the opener of the
// note historique, printed 170, whose text runs straight into pdf 275 at
// printed 171. The two leaves are the wrong way round in the scan.
//
// Reading them in the wrong order costs more than the two pages. The fit sees
// 167, 168, then 169 one page late and 171 one page early, so the run of the
// exercises and the run of the note disagree by one over two pages and neither
// is long enough to be believed; the whole volume was refused for it.
func printingOrder(n int, swaps [][2]int) ([]int, error) {
	order := make([]int, n)
	for i := range order {
		order[i] = i + 1
	}
	seen := map[int]bool{}
	for _, s := range swaps {
		if s[0] == s[1] {
			return nil, fmt.Errorf("pagemap: pdf page %d is transposed with itself", s[0])
		}
		for _, p := range s {
			if p < 1 || p > n {
				return nil, fmt.Errorf("pagemap: pdf page %d is transposed, outside the %d pages of the volume", p, n)
			}
			if seen[p] {
				return nil, fmt.Errorf("pagemap: pdf page %d is transposed twice", p)
			}
			seen[p] = true
		}
		order[s[0]-1], order[s[1]-1] = order[s[1]-1], order[s[0]-1]
	}
	return order, nil
}

// findSteps reports where the offset changes between two touching stretches,
// and which printed page numbers fall in the crack.
func findSteps(covers []cover, restarts []int) []Step {
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
		// Neither is the start of a new fascicule, for the same reason. A step
		// is the printing carrying a leaf the file does not, and reading one
		// off a restart would put every page of the fascicule that ended into
		// the missing list.
		if slices.Contains(restarts, next.from) {
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
	return closeCracks(restartsGoRight(openersGoRight(covers, startPages), opt.Restarts))
}

// restartsGoRight hands the front matter of a new fascicule to the fascicule it
// belongs to.
//
// The pages between the last numbered page of one fascicule and the first of
// the next are its divider, its half title and its notations, and none of them
// carries a running head. Left to closeCracks they would all go to the left,
// which is the rule for a crack where the offset rises and is right when the
// rise is unnumbered leaves at the end of a chapter. It is wrong here: those
// pages are printed pages 6 and 7 of the fascicule beginning, not 99 and 100 of
// the one that ended. Varietes differentielles et analytiques puts its
// DEUXIEME PARTIE divider on pdf 96 and the first head of the second fascicule
// on pdf 98, and the volume names 96.
func restartsGoRight(covers []cover, restarts []int) []cover {
	for i := 0; i+1 < len(covers); i++ {
		cur, next := &covers[i], &covers[i+1]
		for _, r := range restarts {
			if r > cur.to && r < next.from {
				next.from = r
				break
			}
		}
	}
	return covers
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
