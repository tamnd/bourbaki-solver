package extract

import (
	"regexp"
	"slices"
	"sort"
	"strconv"
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/corpus"
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
	// FlagPlainHead is a page that came back with the kind of a statement head
	// in plain mixed case where the printing sets it in small capitals, and had
	// the case put back. Rule 9 rejects a reading like that now, so no new page
	// arrives in the state, but the pages read before the rule existed are
	// committed and the assembler reads no statement on any of them.
	//
	// The flag stays on the page after the repair, for the same reason the stray
	// delimiter flag does. Putting the capitals back is done from what the
	// printing is known to set and not from the page image, so the page has been
	// made to read correctly without having been read again, and it is still a
	// page one reading got wrong. That is worth knowing when the queue has room
	// to read it properly.
	FlagPlainHead Flag = "plain-statement-head"
	// FlagShortHeadDash is a page that came back with a hyphen or an en dash
	// after the head of a statement where the French printing sets an em dash,
	// and had the mark put back. It is the small capitals fault on the other
	// half of the same head and it stays on the page for the same reason: the
	// mark was put back from what the printing is known to set and not from the
	// page image, so the page reads correctly now without having been read
	// again.
	FlagShortHeadDash Flag = "short-statement-head-dash"
	// FlagStackedMatrix is a matrix the text layer flattened. A matrix is set
	// as rows one above the other, and the layer reports the top row raised and
	// the bottom row lowered, so a 2 by 2 arrives as a superscript holding one
	// row and a subscript holding the other: the page prints the matrix with
	// entries a b over c d and the Markdown says $(^{a b}_{c d})$.
	//
	// The geometry that would put it back is not in the layer. A raised run and
	// a lowered run is what a matrix looks like and it is also what x^2_i looks
	// like, and the only thing telling them apart is that the rows of a matrix
	// line up, which is a fact about the boxes that pdftohtml divides by
	// character count and gets wrong by about the width being measured. That is
	// the same wall the dropped glyph test hit.
	//
	// So the page goes to the model with the image, which is the one way a
	// born-digital volume is read through a model. Measured on Algebra VIII:
	// 19 pages of the 992 in the two printings, 31 matrices in each.
	FlagStackedMatrix Flag = "stacked-matrix"
	// FlagStackedRows is a display whose rows the gathering ran together. See
	// stackedRows below for what says so and what it costs to be wrong.
	FlagStackedRows Flag = "stacked-rows"
	// FlagEmpty is a page with no text on it at all.
	FlagEmpty Flag = "empty"
	// FlagDrawnRule is a rule the page draws that could not be put back into
	// the text. Bourbaki's set difference sign is drawn rather than set, so it
	// is not in the text layer at all, and rule.go finds it in the page's own
	// paths and writes it back where the geometry says it goes. Where the
	// geometry does not say, the page is flagged: a sign written into the wrong
	// place reads as mathematics the book does not contain, and the hole it
	// would be filling at least leaves the operands right.
	FlagDrawnRule Flag = "drawn-rule"
	// FlagUnknownFont is a run in a font the tables in font.go have never been
	// shown. It is read as prose, which is what a text face wants and what a
	// mathematics font cannot survive, and the name alone does not say which
	// of the two it is.
	//
	// This flag exists because of what its absence cost. Lie chapters 7 to 9 is
	// set in Knuth's Computer Modern where the volumes read before it are set
	// in Latin Modern, so its mathematics italic is called CMMI10 and not
	// LMMathItalic10, and 31496 runs of variables came through as English
	// words. The run reported the volume 100% clean. Nothing is missing from a
	// page read that way, which is exactly why nothing found it.
	FlagUnknownFont Flag = "unknown-font"
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

	// Minus is how many drawn set difference signs were put back into the
	// runs, and MinusLost how many the page draws that could not be placed.
	// The two are counted rather than inferred from the body, because the sign
	// is an ASCII hyphen once it is in and the page is full of those.
	Minus     int
	MinusLost int

	// Pieces is how many repeats of an extensible bar were taken out of the
	// runs, one bar having been drawn as a stack of them. See extbar.go.
	Pieces int

	// Lost is how many glyphs the page draws that poppler could not name. See
	// Lost below.
	Lost int

	// Continues says the body opens in the middle of the paragraph the page
	// before it ended in. Nothing on this page can be assembled without it, and
	// nothing but this page can work it out: see continues below.
	Continues bool
}

// Lost is how many glyphs of a page poppler drew a box for and could not name.
//
// This is the one loss the reader cannot see from what it was given. A glyph
// read as the wrong character is on the page to be caught by a rule, and a
// glyph read as a code is punctuation in the middle of a formula that stands
// out a mile. A glyph read as nothing leaves a page that reads well with a
// letter gone out of the middle of it: Lie chapters 7 to 9 lost every one of
// its fundamental weights this way and Corollary 1 of § 7, no. 3 said "the
// family ( )_{alpha in B}" for a year.
//
// pdfglyph puts back the ones whose name it knows, and the count here is what
// it could not: a name no table has, or a piece of an extensible bracket that
// has no character to be. The page is flagged for it and goes to the model
// with its image, which is the reading of last resort and the right one when
// the text layer is missing a character outright.
func Lost(p pdfsrc.Page) int {
	n := 0
	for _, s := range p.Spans {
		if s.Text == "" {
			n++
		}
	}
	return n
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
	// The signs the page draws rather than sets go in before anything reads
	// the runs, so that the rest of extraction sees a page with its operators
	// on it. See rule.go.
	p, out.Minus, out.MinusLost = Minus(l, p)
	// A bar built out of repeated pieces comes back to one bar before any of
	// it is read, for the same reason: the runs the rest of extraction sees
	// are meant to be what the page draws. See extbar.go.
	p, out.Pieces = Extensible(l, p)
	out.Lost = Lost(p)
	if out.Lost > 0 {
		out.flag(FlagDroppedGlyph)
	}
	lines, columns := LinesColumns(l, p)
	out.Columns = columns
	if len(lines) == 0 {
		out.flag(FlagEmpty)
		return out
	}
	if out.MinusLost > 0 {
		out.flag(FlagDrawnRule)
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
			if !KnownFont(r.Spec) {
				out.flag(FlagUnknownFont)
			}
			for _, c := range r.Text {
				if PUA(c) {
					out.flag(FlagTallDelimiter)
				}
				if c == arrowExtension {
					out.flag(FlagDiagram)
				}
			}
		}
	}
	for _, ln := range append(append([]Line{}, lines...), notes...) {
		if stackedRows(ln) {
			out.flag(FlagStackedRows)
		}
	}
	out.Continues = continues(lines, v)
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
	// Then the stroke that negates a relation sign, which the text layer hands
	// back as a solidus beside the sign because it has no glyph for the struck
	// one. It goes after Repair rather than before it, so that a sign which
	// arrived as a bare glyph and became TeX a line ago is read here too. This
	// is the one fault on the page that says the opposite of what the page says
	// while looking like nothing is wrong, so it is repaired as the page is
	// written rather than left for anybody to notice.
	out.Body, _ = mathtex.Negation(out.Body)
	if strings.Count(out.Body, "$")%2 != 0 {
		out.flag(FlagUnbalanced)
	}
	if wordInMath(out.Body) {
		out.flag(FlagWordInMath)
	}
	if mathtex.StackedMatrices(out.Body) > 0 {
		out.flag(FlagStackedMatrix)
	}
	return out
}

// There is no test here for a glyph poppler dropped without saying so, and the
// reason is worth writing down, because the test is the obvious thing to try.
//
// Some operators come out as nothing at all, and the hole they leave is about
// one em wide, which is what an operator is worth, so measuring the space
// between two runs looks like it would find them.
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
//
// This note used to name Bourbaki's set difference as the example, and to say
// it was a backslash poppler had dropped. It is not a dropped glyph and it is
// not a backslash: it is a minus sign the book draws as a rule rather than
// setting in a font, so there is no glyph for poppler to drop. Nothing has to
// be measured to find it, because pdftocairo reports the rule with its own
// geometry. rule.go does that, and it is why the example had to go.

// The running head of this volume carries three things and never all three at
// once. The outer edge carries the page label, A VIII.4. The inner edge carries
// the § on a left-hand page and the no. on a right-hand one. Between them sits
// the title of the § or of the chapter, in capitals.
//
// The letter in front of the numeral is the Book, and it is one letter in
// Algebra and two or three in most of the rest: page vii of every recent volume
// prints the table, and Topologie algébrique heads its pages "TA I.144" and
// Théories spectrales "TS III.5". Reading the label as a single A left the
// other letters behind in the title, which is how the corpus shipped 468 pages
// of Topologie algébrique with a running head reading "T EXERCICES" and
// "EXERCICES T", and how every page of Théories spectrales kept its label
// inside its title, since the label there has no A in it to match at all.
//
// The letters are the eleven the Éléments print and not a run of capitals of
// the right length, because there is nothing between the title and the label to
// say where one ends and the other begins. Sixty pages of Algebra VIII head a
// title that runs the whole measure, and the two arrive with no space between
// them: "CRITÈRES POUR QU’UNE ALGÈBRE DE QUATERNIONS SOIT UN CORPSA VIII.357".
var (
	labelRE  = regexp.MustCompile(`(` + strings.Join(corpus.BookLetters(), "|") + `)\s?([IVX]+)\.\s?(\d+)`)
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
		p.Label = m[1] + " " + m[2] + "." + m[3]
	}
	if m := cut(sectRE); m != nil {
		p.Section, _ = strconv.Atoi(m[1])
	}
	if m := cut(subsecRE); m != nil {
		p.Subsec, _ = strconv.Atoi(m[1])
	}
	p.Title = strings.Join(strings.Fields(rest), " ")
	if p.Label == "" {
		p.Foot = headFolio(p.Title)
	}
}

// headFolio is the page number a running head carries at its outer edge, for
// the volumes that print it there rather than at the foot.
//
// Lie 7 to 9 is one: the head of a left-hand page reads "100 SPLIT SEMI-SIMPLE
// LIE ALGEBRAS Ch. VIII" and of a right-hand one ". ALGEBRA DEFINED BY A ROOT
// SYSTEM 97". The volume is paginated straight through, so it prints no page
// label anywhere and there is nothing else on the page that says which printed
// page it is. Without this the 413 pages of it that carry a head say nothing
// about their number, and a § of it assembles with an empty book_pages, which
// is how every reference made to the volume by page goes unresolved.
//
// Only at an edge, and only in arabic, because a number inside a title is part
// of the title: the head of chapter IX names the group SL 2 in the middle of a
// line. The front matter is numbered in roman and is left alone, which is what
// the corpus already does with it.
func headFolio(title string) int {
	f := strings.Fields(title)
	if len(f) < 2 {
		return 0
	}
	for _, s := range []string{f[0], f[len(f)-1]} {
		if n, err := strconv.Atoi(s); err == nil && n > 0 && n < 1000 {
			return n
		}
	}
	return 0
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
	if p.Label == "" && p.Foot == 0 {
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
//
// Mathematics is the same thing again and is more than one sign of it, so it is
// left out of the measure wherever there is text to measure instead. TeX does
// not set a formula at the size of the words around it: page 77 of Lie 7 to 9
// heads § 1 in 18-point bold and draws the sl of sl(2, k) inside that heading
// out of EUFM10 at 24, and heads the subsection under it in 15-point bold with
// the same sl at 19. Measured on the largest run of any kind, the § heading came
// out six points over the body and the subsection head four, so the subsection
// head was read as a chapter title, and the second line of the § heading, which
// carries no mathematics and so measured 18, landed in a block of its own and
// was never joined back on to the line it belongs to. A line that is nothing but
// mathematics has no text to measure and is taken as it stands.
func size(l Line) int {
	best, math := 0, 0
	for _, r := range l.Runs {
		if r.Level != Base || offband(r) {
			continue
		}
		if r.Spec.Size > math {
			math = r.Spec.Size
		}
		if !r.Class.Math() && r.Spec.Size > best {
			best = r.Spec.Size
		}
	}
	switch {
	case best > 0:
		return best
	case math > 0:
		return math
	case len(l.Runs) > 0:
		return l.Runs[0].Spec.Size
	}
	return 0
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

// carryHead puts the rest of a title back on the line it was broken off.
//
// A break falls between two words and the two are joined by the space that
// stood between them, except where the title breaks at a hyphen. TeX breaks a
// line at a hyphen the printing sets, and the hyphen stays in print, so the
// name carries on with no space: Lie 7 to 9 heads three no. that way and the
// corpus shipped "POINCARE-BIRKHOFF- WITT", "SEMI- SIMPLE" and "INFINITELY-
// DIFFERENTIABLE".
//
// TeX breaks a title at a hyphen of its own making too, and then the hyphen has
// to go: Théories spectrales heads no. 2 of chapter I "sur un espace locale-"
// and "ment compact" under it, and no. 13 "dans une algèbre nor-" and "mable
// complète". Which hyphen it is, is the question compound.go answers for the
// body of the volume, and a title is asked it the same way. Only where both
// halves are lower case, since the compounds of a volume are collected in lower
// case and a title set in capitals is not in that book.
func carryHead(head, rest string, c Compounds) string {
	if strings.HasSuffix(head, "-") {
		if lowerBreak(head, rest) && !c.Keeps(head, rest) {
			return strings.TrimSuffix(head, "-") + rest
		}
		return head + rest
	}
	return head + " " + rest
}

// lowerBreak reports whether a title broken at a hyphen has a lower case word
// on both sides of it, which is what the compounds of a volume are written in.
func lowerBreak(head, rest string) bool {
	return tailWord(strings.TrimSuffix(head, "-")) != "" &&
		headWord(strings.TrimLeft(rest, " ")) != ""
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
	// marked is the dangerous bend standing in the margin beside the paragraph
	// being gathered. The sign is drawn against whichever line of the paragraph
	// TeX put it against, which is not the first one and is not the start of a
	// sentence either: French page 257 draws it beside the line that opens
	// "morphisme de D", so the corpus shipped the word "automorphisme" with the
	// sign inside it. The head of the paragraph is where the sign means, and it
	// is where a reader looking for it will look.
	marked := false
	flush := func() {
		if s := strings.TrimSpace(cur.String()); s != "" {
			if marked {
				s = corpus.Bend + " " + s
			}
			out = append(out, s)
		}
		cur.Reset()
		marked = false
	}
	head := -1 // the line the last heading came off, for its continuation
	for i, l := range lines {
		if h, ok := heading(l, v); ok {
			// A title too long for the measure is set on two lines and is still
			// one title. Page 42 sets "§ 2. THE STRUCTURE OF MODULES OF FINITE"
			// and "LENGTH" under it, page 112 breaks § 6 the same way, and the
			// four appendices print their number on one line and their name on
			// the next.
			if head == i-1 && len(out) > 0 && !opensHead(h) &&
				!chapterLine(lines[i-1]) &&
				l.Top-lines[i-1].Top <= headLead(lines[i-1]) {
				out[len(out)-1] = carryHead(out[len(out)-1], strings.TrimLeft(h, "# "), v.Compounds)
				head = i
				continue
			}
			flush()
			out = append(out, h)
			head = i
			continue
		}
		// The rest of a title broken after a word, where what carries on is not
		// itself the shape of a heading. See headTail.
		if head == i-1 && len(out) > 0 && headTail(l) && l.Top-lines[i-1].Top <= headLead(lines[i-1]) {
			out[len(out)-1] = carryHead(out[len(out)-1], headingText(l), v.Compounds)
			head = i
			continue
		}
		text := Render(l)
		// A sign at the head of a line is the one in the margin, since margin
		// took it out of the extent of the line and everything else on the line
		// is set to the right of it. The sign inside a sentence stays where it
		// was written: the foreword says the passages are marked in the margin
		// by the sign, and prints it there in the sentence.
		mark := false
		if s, ok := strings.CutPrefix(text, corpus.Bend); ok {
			mark, text = true, strings.TrimLeft(s, " ")
		}
		if strings.TrimSpace(text) == "" {
			marked = marked || mark
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
				// A display cannot carry the sign at its head without ceasing
				// to be a display, so it goes to the paragraph after it.
				marked = mark
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
				marked = marked || mark
				continue
			}
		}
		if apart || boldOpen(text) {
			flush()
			cur.WriteString(text)
			marked = mark
			continue
		}
		if opens(l) || cur.Len() == 0 {
			flush()
			cur.WriteString(text)
			marked = mark
			continue
		}
		cur.WriteString(" " + strings.TrimLeft(text, " "))
		marked = marked || mark
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
func continues(lines []Line, v Volume) bool {
	if len(lines) == 0 {
		return false
	}
	if _, ok := heading(lines[0], v); ok {
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
		// An index that is nothing but the minus is a minus sign and not the
		// spelling of the word on the next line, which is the same reading
		// emit makes of a hyphen it finds at the end of a formula and for the
		// same reason: taking the sign out leaves the underscore that opened
		// the index with nothing after it, and KaTeX refuses the line. Page
		// 154 of Lie 7 to 9 breaks after the sum of n plus and n minus and
		// shipped as "$\mathfrak{n}_++\mathfrak{n}_$-in", and page 357 of
		// Theories spectrales III to V breaks after A minus and shipped as
		// "$A^$-des". Braces come off an index of one character, so an index
		// that is nothing but the sign is the one way a line can end in an
		// underscore and a hyphen.
		if indexSign(s) {
			return "", false
		}
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

// indexSign reports whether the hyphen a line ends its formula on is the whole
// of an index rather than the hyphen of a word broken across the line.
func indexSign(s string) bool {
	s = strings.TrimSuffix(s, "-$")
	return strings.HasSuffix(s, "_") || strings.HasSuffix(s, "^")
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
// book gives the formula on whichever side the volume sets it and the
// punctuation of the sentence around it behind.
//
// Algebra VIII sets the number at the left margin and Lie 7 to 9 sets it flush
// right, 7 of the one against 110 of the other, and neither volume sets one both
// ways. Read on the left only, every numbered display of Lie 7 to 9 stayed a line
// of inline mathematics with a stray "(6)" on the end of it. Worse, the number
// reaches the measure, so the line looked like a filled line of prose and the
// line under it was joined on: page 76 shipped "PROPOSITION 6. Let E be a finite
// dimensional sl(2, k)-module" on the end of formula (6), where it was not a head
// standing at the front of a paragraph and was never read as one.
var displayRE = regexp.MustCompile(`^(?:\((\d+)\)\s*)?\$([^$]+)\$[.,;]?(?:\s*\((\d+)\))?$`)

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
	// The number is kept and the side it was set on is not, since a display on a
	// line of its own is tagged rather than laid out.
	if n := m[1] + m[3]; n != "" {
		body += ` \tag{` + n + `}`
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
func heading(l Line, v Volume) (string, bool) {
	if h, ok := chapterOpen(l); ok {
		return h, true
	}
	if !headed(l, v) {
		return "", false
	}
	text := headingText(l)
	if len([]rune(text)) < 3 {
		return "", false
	}
	body := v.BodySize
	if body == 0 {
		body = englishBodySize
	}
	// The book prints "§ 2." and the gap it leaves between the sign and the
	// number is not always wide enough to read as a space: 20 of the 31 §
	// headings of Lie 7 to 9 came out "§2." and 11 came out "§ 2." off the same
	// press. The space is not a fact about the book, so it is written the one
	// way here rather than left to the measurement.
	text = sectionSpace.ReplaceAllString(text, "§ ")
	switch {
	case named(text):
		// A heading that says in words what it is takes its level from the
		// words and not from the size, because the printings do not agree on
		// the size and do agree on the words. Algebra VIII sets the word
		// CHAPTER small over the title and Lie 7 to 9 sets it three points over
		// the body, and both of them open a chapter; measured on the size they
		// came out ## and #, and assembly, which looks for the heading that
		// opens a chapter, found one volume and not the other.
		//
		// An appendix is a section of the chapter and is set like one, and the
		// table of contents lists the four of Algebra VIII beside its twenty-one
		// §§. Its number stands on a line of its own, which is the only thing
		// that tells it apart from a subsection head.
		return "## " + text, true
	case size(l) >= body+3:
		return "# " + text, true
	case size(l) < body:
		return "## " + text, true
	}
	return "### " + text, true
}

// sectionSpace is a section sign at the head of a heading with whatever the
// press left between it and the number.
var sectionSpace = regexp.MustCompile(`^§\s*`)

// chapterOpen reads the line a chapter opens on in the three recent French
// volumes, which say CHAPITRE in a face of their own rather than in bold.
//
// Théories spectrales and Topologie algébrique set that one line, and nothing
// else in either book, in a small capitals face the file names SFXC: 4 lines in
// Topologie algébrique for its 4 chapters, 2 and 3 in the two volumes of
// Théories spectrales for theirs. Nothing about it is bold and it is set at the
// size of the body, so neither half of what tells a heading from a sentence
// anywhere else in the corpus can see it, and the chapters would not assemble
// at all: assembly asks the page the table of contents names for the heading
// that opens a chapter, and that page carried a paragraph reading "chapitre
// premier".
//
// The words come back in lower case because that is how a small capitals font
// is encoded, a small A sitting at the code of a small a. The page prints
// CHAPITRE PREMIER, every other volume of the corpus writes that heading in
// capitals, and the reader of the corpus is owed the same word in the same
// shape, so the case is put back.
func chapterOpen(l Line) (string, bool) {
	if len(l.Runs) == 0 {
		return "", false
	}
	for _, r := range l.Runs {
		if family(r.Spec) != chapterFace {
			return "", false
		}
	}
	text := strings.ToUpper(strings.Join(strings.Fields(plain(l)), " "))
	if !named(text) {
		return "", false
	}
	return "## " + text, true
}

// chapterFace is the face those volumes keep for that one line.
const chapterFace = "SFXC"

// sized reports whether a line is a heading in a printing that marks its
// headings by size rather than by weight.
//
// The 2012 French Algebra VIII and both English volumes set every heading in
// bold, which is what headed reads. The three recent French volumes set almost
// none: the § headings of all three are drawn in the volume's own roman at 18
// against a body of 16, upright and unbolded, and so are the marks that divide
// a gathered run of exercises, and the chapter titles are the same roman at 45.
// Only the word Exercices and the headings of the back matter carry a bold flag.
// Read for bold alone, Topologie algébrique offered assembly 25 § headings and
// none of them was one, and the whole of "§ 1. PRODUITS FIBRÉS ET CARRÉS
// CARTÉSIENS" arrived as the first sentence of the paragraph under it.
//
// So the size is read, and it is read narrowly. The line has to be set in the
// face the volume sets its own text in, since a publisher's Times at 45 on a
// cover is not a heading of anything, and every run of it has to be prose,
// which keeps a display carrying a large operator or a tall delimiter out. What
// is left in these three volumes is exactly the headings: at 18 the §§ and
// their marks, at 22 the back matter, at 45 the chapter titles, and nothing
// else in the volume is set in that face above the size of the body.
//
// A row of dot leaders is refused here as it is in headed, for the same reason.
// Nothing else in a volume prints one, and a table of contents that set its
// entries a size up would otherwise come out as a hundred headings.
func sized(l Line, v Volume) bool {
	if len(l.Runs) == 0 || v.BodyFace == "" || v.BodySize == 0 || size(l) <= v.BodySize {
		return false
	}
	for _, r := range l.Runs {
		if r.Class != ClassText || family(r.Spec) != v.BodyFace {
			return false
		}
		if strings.Contains(r.Text, ". . .") || strings.Contains(r.Text, "...") {
			return false
		}
	}
	return true
}

// named reports whether a heading opens on the word for what it is.
//
// APPENDICE is deliberately not here beside APPENDIX. The French printing sets
// its appendix headings smaller than its §§ and the English one does not, so
// the two come out at different depths and the assembler is told which by the
// printing it was handed. That is a difference the volumes really have. The
// depth of a chapter is not: both printings open a chapter the same way and
// only the point size disagrees.
func named(text string) bool {
	for _, word := range []string{"§", "APPENDIX", "CHAPTER", "CHAPITRE"} {
		if strings.HasPrefix(text, word) {
			return true
		}
	}
	return false
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
	// A heading set all in bold is written out of the runs rather than
	// rendered, so the accents have to be folded here too. Page 182 heads a
	// subsection with the Poincaré-Birkhoff-Witt theorem and the layer hands
	// the acute back at the end of one run with the E at the head of the next.
	runs := composed(l.Runs)
	for i, r := range runs {
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
		if i > 0 && r.Left-runs[i-1].Right() >= 3 {
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
// series. That is the 32 below, and it holds for every heading of the corpus
// set at the size of a heading.
//
// A title set in display type breaks further apart, because the type is bigger
// and the leading goes with it. Chapter III of Théories spectrales is headed
// "Applications linéaires" with "compactes et perturbations" 59 units under it,
// in 39-unit lines, and read against a flat 32 the two halves of the title came
// out as two chapter titles. So the lead is measured against the line rather
// than fixed, and the flat figure stands as the floor: twice the band of a
// 13-unit heading is 26, which is less than the 29 an appendix already needs.
func headLead(l Line) int {
	return max(headLeadFlat, l.Height()*2)
}

const headLeadFlat = 32

// chapterLine reports whether a line is the CHAPITRE of a chapter opening in
// the face those volumes keep for it. It takes no continuation: the printing
// sets the title of the chapter under it in display type, on a line of its own,
// and that title is a heading in its own right and a larger one.
func chapterLine(l Line) bool {
	_, ok := chapterOpen(l)
	return ok
}

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

// headTail reports whether a line could be the rest of the heading above it.
//
// It is not itself a heading and headed will not have it: a heading of this
// volume opens on a word or on a single sign, and page 171 breaks the title of
// § 11 after the word AND and sets "sl_2-TRIPLETS" under it, which opens on the
// two letters of a fraktur sl. What says it belongs to the line above is that
// there is nothing on it but the type a heading is set in, capitals and
// mathematics and no running text, and that it is set in the same size as the
// heading, which is already true of every line the caller offers here.
func headTail(l Line) bool {
	strong := false
	for _, r := range l.Runs {
		if offband(r) {
			continue
		}
		switch r.Class {
		case ClassStrong, ClassBold:
			strong = true
		case ClassMath:
		default:
			return false
		}
	}
	return strong
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
// A printing that marks its headings by size and not by weight is read the
// other way, off the measurement of the volume. See sized.
//
// What separates them is where the bold is and what it says. A heading opens on
// its own words, so the line has to start on bold, and it has to carry a bold
// run of four letters or more somewhere, which a citation number never is. A
// line of the table of contents passes both and is not a heading, so the dot
// leaders are read as well: nothing else in the volume prints a row of dots.
//
// The last of it is that a heading is set in heading type all the way across.
// Page 402 of Lie 7 to 9 carries on a sentence of exercise 5 e) with a line that
// opens on the bold SO of SO(2r, R) and names the group Spin further along, so it
// begins on bold and carries a bold word and is not a heading; what it also
// carries, and no heading of the volume does, is forty characters of roman
// running text.
func headed(l Line, v Volume) bool {
	if len(l.Runs) == 0 {
		return false
	}
	if sized(l, v) {
		return true
	}
	runs := l.Runs
	if r := runs[0]; r.Class == ClassMath && len([]rune(strings.TrimSpace(r.Text))) == 1 {
		// The star that marks a subsection optional is drawn before its number,
		// and so is the sign over a §, which TeX keeps in the mathematics
		// symbol font. Neither is bold and neither is what the line is: the
		// heading opens on the word after it.
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
		if (r.Class == ClassText || r.Class == ClassEmph) && letters(r.Text) >= 4 {
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

// arrowExtension is the piece TeX draws a tall vertical arrow's shaft out of.
//
// A commutative diagram set without xypic is a table of terms with arrows
// between them, and the vertical ones are built the way a tall bracket is
// built, out of an arrowhead and as many shaft pieces as the gap needs. The
// head sits at a CMEX code poppler drops, and the shaft comes back as this
// character with nothing around it to say what it belonged to.
//
// Nothing puts that back. The arrows are placed by coordinate and the terms
// they join are on three separate lines by the time they are read, so the
// diagram of page 119 of Lie 7 to 9 came out as three exact sequences one
// after another with no arrows between them and the label q of the middle
// arrow attached to the term above it as an exponent. The page said nothing
// about it and raised no flag, which is worse than saying so.
const arrowExtension = '⏐'

// stackedRows reports whether the gathering ran two rows of a display together.
//
// A display that sets rows one above the other, a cases or a small array,
// hands its rows back interleaved: the layer has no rows in it, so sorting by
// the left edge takes the first cell of the top row, then the first cell of
// the bottom row, and page 229 of Lie 7 to 9 came out reading
// "$\{V(0)$V(2) ifif $mm$ is evenis odd" where the page prints V(0) if m is
// even over V(2) if m is odd.
//
// What says it is rows and not an exponent over an index is that the cells
// line up. TeX sets a superscript and a subscript of one base at the same left
// edge too, which is why one pair proves nothing, but it sets them smaller and
// off the baseline, and it never sets four of them in a column. So only
// full-size runs on the baseline are counted and two columns are wanted.
func stackedRows(l Line) bool {
	type cell struct{ left, top, bottom int }
	var cells []cell
	for _, r := range l.Runs {
		if r.Level != Base || r.Depth != 0 || strings.TrimSpace(r.Text) == "" {
			continue
		}
		cells = append(cells, cell{r.Left, r.Top, r.Bottom()})
	}
	columns := map[int]bool{}
	for i, a := range cells {
		for _, b := range cells[i+1:] {
			if a.left != b.left {
				continue
			}
			if a.bottom > b.top && b.bottom > a.top {
				continue
			}
			columns[a.left] = true
		}
	}
	return len(columns) >= 2
}
