package extract

import (
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"

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
}

// Flagged reports whether the page needs repair.
func (p *Page) Flagged() bool { return len(p.Flags) > 0 }

func (p *Page) flag(f Flag) {
	if slices.Contains(p.Flags, f) {
		return
	}
	p.Flags = append(p.Flags, f)
}

// headBand is how far down the page the running head can be. The 2023 volume
// sets it at 56 and the first line of the body at 87.
const headBand = 70

// ReadPage reads one page of a born-digital volume.
func ReadPage(l *pdfsrc.Layout, p pdfsrc.Page) *Page {
	out := &Page{PDFPage: p.Number}
	lines, columns := LinesColumns(l, p)
	out.Columns = columns
	if len(lines) == 0 {
		out.flag(FlagEmpty)
		return out
	}
	if lines[0].Top < headBand {
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
	out.Body = blocks(lines)
	if len(notes) > 0 {
		out.Body = strings.TrimRight(out.Body+"\n\n"+footnotes(notes), "\n")
	}
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
func size(l Line) int {
	best := 0
	for _, r := range l.Runs {
		if r.Level != Base || tall(r) {
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
func blocks(lines []Line) string {
	var out []string
	for i := 0; i < len(lines); {
		j := i + 1
		for j < len(lines) && size(lines[j]) == size(lines[i]) {
			j++
		}
		if s := join(lines[i:j]); s != "" {
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
func join(lines []Line) string {
	left := lines[0].Left
	for _, l := range lines {
		if l.Left < left {
			left = l.Left
		}
	}
	lead := leading(lines)
	var out []string
	var cur strings.Builder
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			out = append(out, s)
		}
		cur.Reset()
	}
	for i, l := range lines {
		if h, ok := heading(l); ok {
			flush()
			out = append(out, h)
			continue
		}
		text := Render(l)
		if strings.TrimSpace(text) == "" {
			continue
		}
		// Bourbaki opens a statement at the margin and not on an indent, so
		// what says a new one has begun is the head itself and the air above
		// it. Both are read: the head where it is bold, the air where a line
		// sits further below the one before it than the leading of the block.
		apart := i > 0 && l.Top-lines[i-1].Top > lead+6
		if apart || l.Left >= left+indent {
			if d, ok := display(text); ok {
				flush()
				out = append(out, d)
				continue
			}
		}
		if apart || strings.HasPrefix(text, "**") {
			flush()
			cur.WriteString(text)
			continue
		}
		if l.Left >= left+indent || cur.Len() == 0 {
			flush()
			cur.WriteString(text)
			continue
		}
		if joined, ok := runOn(cur.String(), text); ok {
			cur.Reset()
			cur.WriteString(joined)
			continue
		}
		cur.WriteString(" " + strings.TrimLeft(text, " "))
	}
	flush()
	return strings.Join(out, "\n\n")
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
func runOn(s, next string) (string, bool) {
	next = strings.TrimLeft(next, " ")
	switch {
	case strings.HasSuffix(s, "-$") && compoundWord(next):
		return strings.TrimRight(strings.TrimSuffix(s, "-$"), " ") + "$-" + next, true
	case strings.HasSuffix(s, "-") && oneLetter(s):
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
func heading(l Line) (string, bool) {
	var b strings.Builder
	for i, r := range l.Runs {
		if r.Class != ClassBold {
			return "", false
		}
		if i > 0 && r.Left-l.Runs[i-1].Right() >= 3 {
			b.WriteString(" ")
		}
		b.WriteString(r.Text)
	}
	text := strings.Join(strings.Fields(b.String()), " ")
	if len([]rune(text)) < 3 {
		return "", false
	}
	switch {
	case size(l) >= 18:
		return "# " + text, true
	case strings.HasPrefix(text, "§"):
		return "## " + text, true
	case size(l) < 15:
		return "## " + text, true
	}
	return "### " + text, true
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
	if r.Level == Base || r.Class.Math() || !footnoteMark(r.Text) {
		return "", false
	}
	return strings.Trim(strings.TrimSpace(r.Text), "()"), true
}

// footnotes writes the notes of a page as Markdown footnote definitions, which
// is where the references written for them in the body point.
func footnotes(lines []Line) string {
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
		if joined, ok := runOn(strings.TrimRight(cur.String(), " "), text); ok {
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
