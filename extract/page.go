package extract

import (
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/mathtex"
	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// A page is read in four steps: gather the runs into lines, take the running
// head off the top, render each line as Markdown with LaTeX, and join the lines
// back into paragraphs. Nothing crosses a page boundary here. A paragraph cut
// in half by the end of a page stays cut until assembly, where the whole § is
// in front of the program at once.

// Flag is a reason a page cannot be trusted as it stands and has to be repaired
// against the printed image.
type Flag string

const (
	// FlagDiagram is a commutative diagram. It is drawn with xypic, as a set
	// of arrow fragments placed by coordinate, and no amount of geometry puts
	// those back into a diagram.
	FlagDiagram Flag = "diagram"
	// FlagTallDelimiter is a bracket spanning several lines, which arrives as
	// its separate pieces in the Private Use Area.
	FlagTallDelimiter Flag = "tall-delimiter"
	// FlagDroppedGlyph is a glyph poppler could not name. The large brackets
	// of CMEX sit at codes that are control characters, and poppler drops them
	// rather than printing them, so a formula loses a bracket without saying
	// so. The empty run it leaves behind is the only sign, and it is enough to
	// know the page needs looking at.
	FlagDroppedGlyph Flag = "dropped-glyph"
	// FlagUnbalanced is an odd number of dollar signs in the page.
	FlagUnbalanced Flag = "unbalanced-math"
	// FlagStrayDelimiter is a page that balanced only after a delimiter which
	// opened nothing was taken out of it. It is what a numbered display flattened
	// into one line of prose leaves behind, and taking the delimiter out makes
	// the page read correctly while losing the one sign that the display was
	// ever a display. So the flag stays on the page after the repair: the
	// mathematics is right and the setting is not, and only the printed page can
	// say what the setting was.
	FlagStrayDelimiter Flag = "stray-delimiter"
	// FlagWordInMath is an English word left inside dollar signs, which means
	// a formula was cut in the wrong place.
	FlagWordInMath Flag = "word-in-math"
	// FlagEmpty is a page with no text on it at all.
	FlagEmpty Flag = "empty"
)

// Page is one page read.
type Page struct {
	PDFPage int
	Head    string // the running head, as printed
	Title   string // the running head with the label and the locator taken out
	Label   string // the printed page label, as the head gives it
	Foot    int    // the page number printed at the foot, on a page with no head
	Section int    // the § the head names, zero if it names none
	Subsec  int    // the no. the head names, zero if it names none
	Body    string
	Lines   int
	Columns bool // the page is set in two columns, as the index at the back is
	Flags   []Flag

	// Continues says the body opens in the middle of the paragraph the page
	// before it ended in. Nothing on this page can be assembled without it, and
	// nothing but this page can work it out: see continues below.
	Continues bool
}

// Flagged reports whether the page needs repair.
func (p *Page) Flagged() bool { return len(p.Flags) > 0 }

func (p *Page) flag(f Flag) {
	if slices.Contains(p.Flags, f) {
		return
	}
	p.Flags = append(p.Flags, f)
}

// ReadPage reads one page of a born-digital volume, measuring the volume around
// it as it goes.
//
// It is the first of the two passes: it knows nothing of the compound words the
// volume writes, so a compound broken at the end of a line loses its hyphen.
// The bodies it returns are what ReadPageWith needs to do better.
func ReadPage(l *pdfsrc.Layout, p pdfsrc.Page) *Page { return ReadPageWith(l, p, Measure(l)) }

// ReadPageWith reads one page with the volume it belongs to in hand.
func ReadPageWith(l *pdfsrc.Layout, p pdfsrc.Page, v Volume) *Page {
	out := &Page{PDFPage: p.Number}
	lines, columns := LinesColumns(l, p)
	out.Columns = columns
	if len(lines) == 0 {
		out.flag(FlagEmpty)
		return out
	}
	if lines[0].Top < v.HeadBand {
		out.Head = plain(lines[0])
		out.readHead()
		lines = lines[1:]
	}
	lines = out.readFoot(lines, p.Height)
	lines, notes := splitNotes(lines)
	out.Lines = len(lines) + len(notes)
	for _, ln := range append(append([]Line{}, lines...), notes...) {
		for _, r := range ln.Runs {
			if r.Class == ClassDiagram {
				out.flag(FlagDiagram)
			}
			if strings.TrimSpace(r.Text) == "" && r.Class == ClassMath {
				out.flag(FlagDroppedGlyph)
			}
			for _, c := range r.Text {
				if PUA(c) {
					out.flag(FlagTallDelimiter)
				}
			}
		}
	}
	out.Continues = continues(lines)
	out.Body = blocks(lines, v)
	if len(notes) > 0 {
		out.Body = strings.TrimRight(out.Body+"\n\n"+footnotes(notes, v), "\n")
	}
	// A glyph is mapped to its TeX while the run's font is known, and a Greek
	// capital is not in a font that says it is mathematics: Bourbaki sets those
	// upright, in the text face, so the run is prose as far as Classify is
	// concerned and comes through as the letter. It is only mathematics once
	// the line is laid out and the letter is inside a pair of dollars, which is
	// here. 360 characters of chapter VIII arrived this way.
	//
	// The refusals are not reported from here. They are two characters in the
	// whole volume and M03 has them from the other side, with the file and the
	// line, which is where somebody deciding between a sigma and a sum wants to
	// be told about it.
	//
	// The stray delimiter goes first, because everything after one is a math
	// span as far as the splitter is concerned and Repair will not touch it.
	// Eight pages of chapter VIII need both, in that order.
	//
	// It is not run on a page that is already flagged. The flags at this point
	// are the ones raised while the runs were being read, and every one of them
	// means a formula arrived in pieces: on a page like that a delimiter is as
	// likely to be the surviving half of a display as it is to be a leftover,
	// and DropStray has no way to tell which.
	if body, ok := mathtex.DropStray(out.Body); ok && len(out.Flags) == 0 {
		out.Body = body
		out.flag(FlagStrayDelimiter)
	}
	// Then the bracket a name in prose left inside its own argument, which is
	// 138 spans of chapter VIII and reads correctly on the page while saying
	// something else in the Markdown. It goes before Repair so that Repair sees
	// the spans as they will be rather than as they arrived.
	out.Body, _ = mathtex.Unstraddle(out.Body)
	out.Body, _, _ = mathtex.Repair(out.Body)
	if strings.Count(out.Body, "$")%2 != 0 {
		out.flag(FlagUnbalanced)
	}
	if wordInMath(out.Body) {
		out.flag(FlagWordInMath)
	}
	return out
}

// There is no test here for a glyph poppler dropped without saying so, and the
// reason is worth writing down, because the test is the obvious thing to try.
//
// Some glyphs come out as nothing at all. The set difference of P and S is
// printed as P \ S and arrives as two runs with a hole between them where the
// backslash was, and neither pdftotext nor pdftohtml says a word about it. The
// hole is about one em wide, which is what a dropped operator is worth, so
// measuring the space between two runs looks like it would find them.
//
// It does not. pdftohtml reports one box per <text> element, and an element
// holds several runs when the fonts change inside it, so the box of a run in
// the middle of one is worked out by dividing the element up. That division is
// by character count, which is close but not exact, and the error in it is the
// same size as the hole being looked for. Run over the whole volume the test
// flags 243 pages of 505, nearly all of them a comma in a formula or a large
// delimiter whose width pdftohtml gets wrong, and a flag that fires on half the
// book tells the repair pass nothing. What is left is caught by the audit,
// which reads the printed page against the extracted one.

// The running head of this volume carries three things and never all three at
// once. The outer edge carries the page label, A VIII.4. The inner edge carries
// the § on a left-hand page and the no. on a right-hand one. Between them sits
// the title of the § or of the chapter, in capitals.
var (
	labelRE  = regexp.MustCompile(`A\s?([IVX]+)\.\s?(\d+)`)
	sectRE   = regexp.MustCompile(`§\s?(\d+)`)
	subsecRE = regexp.MustCompile(`\bNo\s?(\d+)`)
)

// readHead takes the head apart into the page label, the locator and the title.
func (p *Page) readHead() {
	rest := p.Head
	cut := func(re *regexp.Regexp) []string {
		m := re.FindStringSubmatch(rest)
		if m == nil {
			return nil
		}
		rest = strings.Replace(rest, m[0], " ", 1)
		return m
	}
	if m := cut(labelRE); m != nil {
		p.Label = "A " + m[1] + "." + m[2]
	}
	if m := cut(sectRE); m != nil {
		p.Section, _ = strconv.Atoi(m[1])
	}
	if m := cut(subsecRE); m != nil {
		p.Subsec, _ = strconv.Atoi(m[1])
	}
	p.Title = strings.Join(strings.Fields(rest), " ")
}

// footBand is how far down the page the folio has to be to be the folio. The
// volume sets it in the last tenth.
const footBand = 9

// readFoot takes the printed page number off the foot of a page.
//
// A page that opens a § carries no running head, because the head would sit
// where the title of the § does, so the volume prints the number at the foot
// instead. It is the same number the head carries elsewhere, and it belongs in
// the front matter of the page file rather than in the last paragraph of it.
func (p *Page) readFoot(lines []Line, height int) []Line {
	if len(lines) == 0 || height == 0 {
		return lines
	}
	last := lines[len(lines)-1]
	if last.Top*10 < height*footBand || len(last.Runs) != 1 {
		return lines
	}
	n := strings.TrimSpace(last.Runs[0].Text)
	if n == "" || strings.TrimLeft(n, "0123456789") != "" {
		return lines
	}
	if p.Label == "" {
		p.Foot, _ = strconv.Atoi(n)
	}
	return lines[:len(lines)-1]
}

// plain renders a line with no mathematics in it, which is what a running head
// is.
func plain(l Line) string {
	var b strings.Builder
	for i, r := range l.Runs {
		if i > 0 && r.Left-l.Runs[i-1].Right() > 8 {
			b.WriteString("   ")
		}
		b.WriteString(r.Text)
	}
	return strings.TrimSpace(b.String())
}

// indent is how far past the margin a line has to start to be an indented one.
// Bourbaki indents a paragraph by about 30 units and a display by more.
const indent = 12

// size is the size of the body font of a line, which is the size of the largest
// run sitting on the baseline. Bourbaki sets a remark, an example and an
// exercise in a smaller type than the text around it, and that change of size is
// where one block of the page ends and the next begins.
//
// A sign drawn out of proportion to the type beside it says nothing about the
// size that type is set in. The dangerous bend of French page 19 stands at 29
// against a remark set in 13, and taking the size from it cut the first line of
// the remark off from the rest of it.
func size(l Line) int {
	best := 0
	for _, r := range l.Runs {
		if r.Level != Base || offband(r) {
			continue
		}
		if r.Spec.Size > best {
			best = r.Spec.Size
		}
	}
	if best == 0 && len(l.Runs) > 0 {
		best = l.Runs[0].Spec.Size
	}
	return best
}

// blocks reads the lines of a page as a sequence of blocks, a block being a run
// of lines set in one size, and writes each block out as paragraphs.
//
// The margin has to be taken per block rather than per page. A remark set in
// small type is indented as a whole, so measuring its lines against the margin
// of the page would make every one of them the start of a paragraph.
func blocks(lines []Line, v Volume) string {
	var out []string
	for i := 0; i < len(lines); {
		j := i + 1
		for j < len(lines) && size(lines[j]) == size(lines[i]) {
			j++
		}
		if s := join(lines[i:j], v); s != "" {
			out = append(out, s)
		}
		i = j
	}
	return strings.Join(out, "\n\n")
}

// join puts the lines of one block back into paragraphs.
//
// A line is joined to the one before it unless it starts a paragraph, and it
// starts a paragraph when it is indented past the margin of its block. A word
// broken across the break is put back together, which is what the trailing
// hyphen means. A heading stands on its own.
func join(lines []Line, v Volume) string {
	left := lines[0].Left
	for _, l := range lines {
		if l.Left < left {
			left = l.Left
		}
	}
	lead := leading(lines)
	right := measure(lines)
	opens := opener(lines, left)
	var out []string
	var cur strings.Builder
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			out = append(out, s)
		}
		cur.Reset()
	}
	head := -1 // the line the last heading came off, for its continuation
	for i, l := range lines {
		if h, ok := heading(l, v.BodySize); ok {
			// A title too long for the measure is set on two lines and is still
			// one title. Page 42 sets "§ 2. THE STRUCTURE OF MODULES OF FINITE"
			// and "LENGTH" under it, page 112 breaks § 6 the same way, and the
			// four appendices print their number on one line and their name on
			// the next.
			if head == i-1 && len(out) > 0 && !opensHead(h) &&
				l.Top-lines[i-1].Top <= headLead {
				out[len(out)-1] += " " + strings.TrimLeft(h, "# ")
				head = i
				continue
			}
			flush()
			out = append(out, h)
			head = i
			continue
		}
		text := Render(l)
		if strings.TrimSpace(text) == "" {
			continue
		}
		// Bourbaki opens a statement at the margin and not on an indent, so
		// what says a new one has begun is the head itself and the air above
		// it. Both are read: the head where it is bold, the air where a line
		// sits further below the one before it than the leading of the block,
		// and the line above where it stops short of the measure.
		apart := i > 0 && l.Top-lines[i-1].Top > lead+6
		apart = apart || (i > 0 && right > 0 && lines[i-1].Right < right-short)
		if apart || opens(l) {
			if d, ok := display(text); ok {
				flush()
				out = append(out, d)
				continue
			}
		}
		// A word broken at a line end is not the end of a paragraph, whatever
		// the line below it is indented to. The bibliography hangs its indent
		// the other way round from the rest of the book and page 497 broke
		// "com-" onto "plexen"; page 237 sets each of the conditions (i), (ii),
		// (iii) on its own indent and broke "commu-" onto "tative"; page 377
		// broke "homoge-" onto "neous". All three shipped with the hyphen still
		// in them and the two halves in paragraphs of their own.
		if cur.Len() > 0 {
			if joined, ok := runOn(cur.String(), text, v.Compounds); ok {
				cur.Reset()
				cur.WriteString(joined)
				continue
			}
		}
		if apart || boldOpen(text) {
			flush()
			cur.WriteString(text)
			continue
		}
		if opens(l) || cur.Len() == 0 {
			flush()
			cur.WriteString(text)
			continue
		}
		cur.WriteString(" " + strings.TrimLeft(text, " "))
	}
	flush()
	return strings.Join(out, "\n\n")
}

// continues reports whether the body of a page opens in the middle of the
// paragraph the page before it ended in.
//
// Assembly cannot tell from the text. A page that ends "…is nonzero." and one
// that opens "For any λ ∈ S…" reads the same whether the second is a new
// paragraph or the rest of the one above, and the volume breaks both ways: page
// 363 opens "of scalars)" halfway through a sentence and page 159 opens a
// paragraph of its own. Guessing at the capital letter after the full stop gets
// the second right and the first wrong.
//
// The page itself knows, because Bourbaki indents the first line of a paragraph
// and sets the rest at the margin, which is the same fact join already reads
// inside a page. So it is read here too and written into the front matter, and
// assembly, which never sees the PDF, is told rather than left to guess.
//
// Measured on Algebra VIII: 275 of the 475 pages of the chapter open at the
// margin, and every one of the pages whose first character is lower case is
// among them.
//
// What this reports is the indent, and the indent is not the whole answer in
// the exercises, which the volume sets with no paragraph indent at all: 20 of
// the 21 pages that open on a new exercise open at the margin and are counted
// here as carrying on. Nothing on the page distinguishes them, so the rule is
// left as the measurement it is, and assembly, which can see that a paragraph
// opening "5) " is an item and not the tail of a sentence, declines to join
// them. See joinable.
func continues(lines []Line) bool {
	if len(lines) == 0 {
		return false
	}
	if _, ok := heading(lines[0], 0); ok {
		return false
	}
	// The margin is the margin of the block, as it is in blocks and join. A
	// remark in small type is indented as a whole, and the page that opens in
	// the middle of one opens at the margin of the remark.
	j := 1
	for j < len(lines) && size(lines[j]) == size(lines[0]) {
		j++
	}
	first := lines[:j]
	left := first[0].Left
	for _, l := range first {
		if l.Left < left {
			left = l.Left
		}
	}
	return !opener(first, left)(lines[0])
}

// boldOpen reports whether a line opens on bold words, which is where a
// statement begins and where an entry of the table of contents does.
//
// A line opening on a bold number is neither. The bibliography sets the volume
// number of a journal in bold, page 496 turned over onto one, and reading that
// as an opening cut the entry for C. Hopkins in two and left "40 (1939), p.
// 712-730." standing on its own.
func boldOpen(text string) bool {
	rest, ok := strings.CutPrefix(text, "**")
	if !ok {
		return false
	}
	i := strings.Index(rest, "**")
	return i > 0 && letters(rest[:i]) > 0
}

// runOn joins a line that runs on into the next one, and says whether it did.
//
// A hyphen at a line break usually means a word was broken across it, and
// putting the word back means dropping the hyphen: "commu-" and "tative" are
// one word.
//
// A hyphen inside the mathematics at the end of a line is the other case. It is
// the hyphen of A_M-module, where the typesetter set the A_M in mathematics and
// broke the line after the hyphen, and it comes back on the mathematics side of
// that boundary. Left there it prints as $A_M-$ module, which is wrong twice
// over: the hyphen is typeset as a minus sign and the compound word is broken by
// a space that is not on the page. So it moves outside the dollars and the word
// joins on to it.
//
// What tells that from a subtraction broken across a line is the word after it,
// under the same reading emit uses on a hyphen in the middle of a line.
//
// The third case is a compound word whose own hyphen fell at the end of a line.
// It is told from a broken word by the rest of the volume, which is what
// Compounds carries: see compound.go.
func runOn(s, next string, c Compounds) (string, bool) {
	next = strings.TrimLeft(next, " ")
	switch {
	case strings.HasSuffix(s, "-$") && compoundWord(next):
		return strings.TrimRight(strings.TrimSuffix(s, "-$"), " ") + "$-" + next, true
	case strings.HasSuffix(s, "$-"):
		// The same compound with the letter set as mathematics, "$(K,G)$-" and
		// "$K(\mathbf{T})$-". There is no word before the hyphen to have been
		// broken, only a close of mathematics, so the hyphen is the word's own
		// and stays. Measured on Algebra VIII: four lines end this way, and
		// dropping the hyphen gave $(K,G)$algebras on pages 324, 325 and 329
		// and $K(\mathbf{T})$algebra on page 362.
		return s + next, true
	case strings.HasSuffix(s, "-") && oneLetter(s):
		return s + next, true
	case strings.HasSuffix(s, "-") && c.Keeps(s, next):
		return s + next, true
	case strings.HasSuffix(s, "-"):
		return strings.TrimSuffix(s, "-") + next, true
	}
	return "", false
}

// oneLetter reports whether the hyphen a line ends on comes straight after a
// letter standing on its own, which is the other half of the compound word:
// the A of A-module, the K of K-algebra, the B of B-linear. Bourbaki sets these
// in roman when the letter is a plain one, so no dollar is anywhere near them
// and the line simply ends in "A-".
//
// The hyphen has to stay. TeX will not break a word after its first letter, so
// a one letter fragment before a hyphen is never a word cut in two, and
// dropping the hyphen the way the ordinary case does gives Amodule. Measured on
// Algebra VIII before this: 47 of them, Amodule, Kalgebra, Bmodule, Lalgebra,
// Alinear.
func oneLetter(s string) bool {
	s = strings.TrimSuffix(s, "-")
	if s == "" || !isLetter(rune(s[len(s)-1])) {
		return false
	}
	if len(s) == 1 {
		return true
	}
	c := rune(s[len(s)-2])
	return !isLetter(c) && (c < '0' || c > '9')
}

// entryRE is the number a bibliography entry opens with, in square brackets at
// the margin.
var entryRE = regexp.MustCompile(`^\[\d+\]`)

// opener says which lines of a block start a paragraph.
//
// Bourbaki indents the first line of a paragraph and sets the rest at the
// margin, so an indented line is where a paragraph begins. The bibliography is
// set the other way round: the entry opens at the margin with its number in
// square brackets and the lines after it are indented under it. Read by the
// ordinary rule every line of it becomes a paragraph of its own, which is what
// the six pages of the bibliography looked like, hyphens left at the ends of
// the lines and all. "of any group of linear sub-" was one paragraph and
// "stitutions", Proc. Lond. Math. Soc." was the next.
//
// The number is what turns the rule over, and nothing else does. Leaning on the
// geometry alone, on the grounds that a hanging indent has more indented lines
// than not, reads the six enumerated properties of Proposition 4 on page 155 as
// one paragraph and cuts a sentence of page 257 in half. The volume indents too
// many things for the shape of the block to say what the block is.
func opener(lines []Line, left int) func(Line) bool {
	ordinary := func(l Line) bool { return l.Left >= left+indent }
	labelled, in := 0, 0
	for _, l := range lines {
		if ordinary(l) {
			in++
			continue
		}
		if len(l.Runs) > 0 && entryRE.MatchString(strings.TrimSpace(l.Runs[0].Text)) {
			labelled++
		}
	}
	if labelled < 2 || in <= labelled {
		return ordinary
	}
	return func(l Line) bool { return !ordinary(l) }
}

// displayRE is a line that is nothing but one formula, with the number the
// book gives the formula in front of it and the punctuation of the sentence
// around it behind.
var displayRE = regexp.MustCompile(`^(?:\((\d+)\)\s*)?\$([^$]+)\$[.,;]?$`)

// display writes a line that stands on its own as a display.
//
// The punctuation the book sets after a display is punctuation of the sentence
// the display is set into, and it is dropped: it reads as part of the formula
// once the formula is on a line of its own. The number is kept, since the text
// refers back to it.
func display(text string) (string, bool) {
	m := displayRE.FindStringSubmatch(text)
	if m == nil {
		return "", false
	}
	body := strings.TrimSpace(m[2])
	if m[1] != "" {
		body += ` \tag{` + m[1] + `}`
	}
	return "$$\n" + body + "\n$$", true
}

// leading is how far apart the lines of a block are set, taken as the median
// step from one line to the next. A step longer than that is white space the
// typesetter put there, and white space between two lines of prose is a
// paragraph.
// short is how far a line has to stop before the measure to have ended the
// paragraph it is in. A line of justified type ends on the measure to within a
// unit or two, so anything wider than a character of the body type is a line
// that was not filled, and the only line of a paragraph that is not filled is
// its last.
const short = 20

// measure is the width the block is set to, taken from its longest line, and
// zero when the block is too short to have a line that was filled.
//
// The measure has to be taken per block for the same reason the margin does: a
// remark set in small type is indented on both sides and its lines end well
// inside the measure of the page around it.
func measure(lines []Line) int {
	if len(lines) < 3 {
		return 0
	}
	right := 0
	for _, l := range lines {
		if l.Right > right {
			right = l.Right
		}
	}
	return right
}

func leading(lines []Line) int {
	if len(lines) < 3 {
		return 1 << 20 // too few lines to tell; never break on the step
	}
	steps := make([]int, 0, len(lines)-1)
	for i := 1; i < len(lines); i++ {
		steps = append(steps, lines[i].Top-lines[i-1].Top)
	}
	sort.Ints(steps)
	return steps[len(steps)/2]
}

// heading reads a line set entirely in bold, which in this volume is a heading
// and nothing else: the chapter, the §, the numbered subsection, the word
// Exercises. A bold run inside a line of prose is one of the letters N, Z, Q, R
// and C, or an indeterminate X, and is left to the renderer.
//
// Both bold classes are taken. The words of a heading are strong and the number
// it opens on can be either, since § 16. carries punctuation and the 1. of a
// subsection is short enough to be read as a symbol on its own.
//
// A heading may also carry mathematics, and twelve of them in this volume do:
// "6. The Grothendieck Group R_K(A)" on page 211, "1. τ-Extensions of Groups"
// on page 302. Such a line is read by the renderer and its bold marks taken off
// again, so the mathematics in it comes out as mathematics.
//
// Which level a heading is set at is measured against the size the volume sets
// its body in, since the printings do not agree on a size: the English chapter
// is set in 15 and the French one in 14, and a rule reading anything under 15 as
// the word CHAPTER made every subsection head of the French volume a ## where
// the English has ###. body is that size, and zero means nobody measured, in
// which case the English size is assumed.
func heading(l Line, body int) (string, bool) {
	if !headed(l) {
		return "", false
	}
	text := headingText(l)
	if len([]rune(text)) < 3 {
		return "", false
	}
	if body == 0 {
		body = englishBodySize
	}
	switch {
	case size(l) >= body+3:
		return "# " + text, true
	case strings.HasPrefix(text, "§"), strings.HasPrefix(text, "APPENDIX"):
		// An appendix is a section of the chapter and is set like one, and the
		// table of contents lists the four of this volume beside the twenty-one
		// §§. Its number stands on a line of its own, which is the only thing
		// that tells it apart from a subsection head.
		return "## " + text, true
	case size(l) < body:
		// The word CHAPTER, which the volume sets small over the title of the
		// chapter itself.
		return "## " + text, true
	}
	return "### " + text, true
}

// englishBodySize is the size the 2023 English printing sets its text in, which
// is what the reading of that volume was audited against.
const englishBodySize = 15

// headingText writes the words of a heading.
//
// A line that is bold from end to end is read straight off the runs, with a
// space where the typesetter left one. The renderer is not asked, because it
// answers questions a heading does not have: the title page sets "Chapter 8"
// smaller and lower than "Algebra" above it, which is a subscript to the
// renderer and the second half of the title to a reader.
//
// A line carrying mathematics has to go through the renderer, since that is
// where the mathematics is written, and its bold marks come off afterwards.
func headingText(l Line) string {
	var b strings.Builder
	for i, r := range l.Runs {
		if r.Class != ClassBold && r.Class != ClassStrong {
			text := strings.Join(strings.Fields(strings.ReplaceAll(Render(l), "**", "")), " ")
			// The star that marks a subsection optional is a mark of the book
			// and not a formula, so it comes out of the dollars and is escaped
			// where it stands.
			if rest, ok := strings.CutPrefix(text, "$*$"); ok {
				text = `\*` + rest
			}
			return text
		}
		if i > 0 && r.Left-l.Runs[i-1].Right() >= 3 {
			b.WriteString(" ")
		}
		b.WriteString(r.Text)
	}
	return strings.Join(strings.Fields(b.String()), " ")
}

// headLead is how far under a heading the rest of that heading can be.
//
// The volume sets a heading in 13-unit lines and puts the second line of one 21
// units under the first, or 29 where the number stands on a line of its own as
// it does over an appendix. What is not a continuation is much further off: the
// chapter title of page 18 is 64 units under the word CHAPTER, in a larger font,
// and the title page sets the name of the book 93 units under the name of the
// series.
const headLead = 32

// opensHead reports whether a heading line opens a heading of its own rather
// than carrying on the one above it. Every heading of the volume that does opens
// on its number or on a word that names what it is.
func opensHead(h string) bool {
	t := strings.TrimLeft(h, "# ")
	if i := strings.IndexByte(t, '.'); i > 0 && i <= 3 && allDigits(t[:i]) {
		return true
	}
	for _, w := range []string{"§", "CHAPTER", "APPENDIX", "Exercises"} {
		if strings.HasPrefix(t, w) {
			return true
		}
	}
	return false
}

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return s != ""
}

// headed reports whether a line is a heading.
//
// A line set in bold from end to end is one, and that is most of them. The rest
// carry the mathematics of their own title, so the test cannot be that every
// run is bold, and it cannot be that any run is either: the historical note
// sets its citation numbers bold and the bibliography sets the volume number of
// a journal bold, so "an arbitrary base field ([51], p. 102)" has a bold run in
// the middle of a sentence.
//
// What separates them is where the bold is and what it says. A heading opens on
// its own words, so the line has to start on bold, and it has to carry a bold
// run of four letters or more somewhere, which a citation number never is. A
// line of the table of contents passes both and is not a heading, so the dot
// leaders are read as well: nothing else in the volume prints a row of dots.
func headed(l Line) bool {
	if len(l.Runs) == 0 {
		return false
	}
	runs := l.Runs
	if r := runs[0]; r.Class == ClassMath && len([]rune(strings.TrimSpace(r.Text))) == 1 {
		// The star that marks a subsection optional is drawn before its number.
		runs = runs[1:]
	}
	if len(runs) == 0 || runs[0].Class != ClassBold && runs[0].Class != ClassStrong {
		return false
	}
	words := false
	for _, r := range l.Runs {
		if strings.Contains(r.Text, ". . .") || strings.Contains(r.Text, "...") {
			return false
		}
		if r.Class == ClassStrong && letters(r.Text) >= 4 {
			words = true
		}
	}
	return words
}

// letters counts the letters of a run.
func letters(s string) int {
	n := 0
	for _, r := range s {
		if unicode.IsLetter(r) {
			n++
		}
	}
	return n
}

// splitNotes takes the footnotes off the bottom of a page. A footnote opens on
// its number raised above the line and at the margin, and everything below the
// first of them is footnote.
func splitNotes(lines []Line) (body, notes []Line) {
	for i, l := range lines {
		if _, ok := noteNumber(l); ok {
			return lines[:i], lines[i:]
		}
	}
	return lines, nil
}

// noteNumber reads the number a footnote opens with.
func noteNumber(l Line) (string, bool) {
	if len(l.Runs) == 0 {
		return "", false
	}
	r := l.Runs[0]
	if r.Level != Sup || r.Class.Math() || !footnoteMark(r.Text) {
		return "", false
	}
	return strings.Trim(strings.TrimSpace(r.Text), "()"), true
}

// footnotes writes the notes of a page as Markdown footnote definitions, which
// is where the references written for them in the body point.
func footnotes(lines []Line, v Volume) string {
	var out []string
	var cur strings.Builder
	num := ""
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			out = append(out, "[^"+num+"]: "+s)
		}
		cur.Reset()
	}
	for _, l := range lines {
		if n, ok := noteNumber(l); ok {
			flush()
			num = n
			cur.WriteString(strings.TrimSpace(Render(Line{Runs: l.Runs[1:],
				Top: l.Top, Bottom: l.Bottom, Left: l.Left, Right: l.Right})))
			continue
		}
		text := strings.TrimSpace(Render(l))
		if text == "" {
			continue
		}
		if joined, ok := runOn(strings.TrimRight(cur.String(), " "), text, v.Compounds); ok {
			cur.Reset()
			cur.WriteString(joined)
			continue
		}
		cur.WriteString(" " + text)
	}
	flush()
	return strings.Join(out, "\n")
}

// macroRE is a LaTeX command with its name.
var macroRE = regexp.MustCompile(`\\[a-zA-Z]+`)

// proseRE is two words of three letters or more with a single space between
// them, which is prose and not a formula.
var proseRE = regexp.MustCompile(`[A-Za-z]{3,} [A-Za-z]{3,}`)

// wordInMath reports whether any formula holds prose, which is the sign that a
// formula was cut in the wrong place.
//
// What says a formula holds prose is not the length of a word but the space
// beside it. A run of letters on its own is as likely to be a product of
// variables as a word: the book writes xaxa and nnnq and axbc, and a rule that
// reads four letters in a row as English calls all three of them English. Two
// words with a space between them is prose, because a formula does not put a
// space between two products.
func wordInMath(body string) bool {
	parts := strings.Split(body, "$")
	for i := 1; i < len(parts); i += 2 {
		// The name of a LaTeX command is not a word of the book: \subset is
		// six letters in a row and reads as one.
		if proseRE.MatchString(macroRE.ReplaceAllString(parts[i], " ")) {
			return true
		}
	}
	return false
}
