// Package assemble puts the pages of a chapter back into the sections the book
// is written in.
//
// Extraction reads one page at a time and stops at the edge of it, because a
// page is all it can see. A paragraph cut in half by the end of a page stays
// cut, a § that runs over twenty pages is twenty files, and a footnote is
// numbered from one on every page it appears on. This is where that is undone.
//
// Nothing here reads the PDF, and that is the contract rather than an accident:
// assembly is a pure function of pages/ and manifests/toc.yaml, so it runs in CI
// where the PDFs are not and cannot be, and running it twice over the same pages
// gives the same bytes. Everything it needs that only the PDF knows is written
// into the page front matter by extraction, which is why a page carries
// "continues".
//
// The one thing it will not do is guess. The table of contents says § 12 opens
// on PDF page 228 and the page itself carries the heading; if the two disagree
// the run stops and says so. A disagreement means either the map or the reading
// is wrong, and quietly picking one of them is how a corpus ends up with a
// section missing its first three pages and nobody the wiser.
package assemble

import (
	"fmt"
	"regexp"
	"slices"
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// Piece is one assembled file: the opening of a chapter, one § or appendix, or
// the historical note.
type Piece struct {
	corpus.Section // the table of contents entry, empty for the front and the note

	Front      bool // what stands before § 1: the chapter title and its preamble
	Historical bool // the historical note at the end of the chapter

	// Runs are the stretches of pages the piece is made of, in printed order.
	// Nearly every piece is one run, and a § whose exercises the chapter gathered
	// at its end is two. See span.
	Runs    []Run
	Methods []corpus.PageMethod

	Body        string
	Subsections []corpus.Subsection
	Statements  []corpus.Statement
	Exercises   []corpus.Exercise
	HasExercise bool // the § carries a block of exercises
}

// Run is one stretch of pages a piece is made of, by PDF page and by the page
// the book prints on it.
type Run struct {
	First, Last           int
	FirstLabel, LastLabel string
}

// First and Last are the PDF pages the piece opens and ends on. Between them
// can lie pages of another piece, which is why they are not the piece.
func (p Piece) First() int {
	if len(p.Runs) == 0 {
		return 0
	}
	return p.Runs[0].First
}

func (p Piece) Last() int {
	if len(p.Runs) == 0 {
		return 0
	}
	return p.Runs[len(p.Runs)-1].Last
}

// Name is what the piece is called in a message.
func (p Piece) Name() string {
	switch {
	case p.Front:
		return "front matter"
	case p.Historical:
		return "historical note"
	case p.Appendix:
		return fmt.Sprintf("Appendix %d", p.Number)
	}
	return fmt.Sprintf("§ %d", p.Number)
}

// Extraction is how the pages of this piece were read, which is what the front
// matter records so that a reader knows how far to trust the text.
//
// A piece of twenty pages can be read twenty different ways, and what matters
// is the weakest of them: one page repaired by a model in an otherwise native
// section makes the section native+repair, not native. So the distinct methods
// are gathered and joined, and a page with nothing on it counts as nothing.
func (p Piece) Extraction() string {
	var out []string
	for _, m := range p.Methods {
		if m == corpus.MethodBlank || m == "" {
			continue
		}
		s := string(m)
		if s == string(corpus.MethodOCRRepair) {
			s = "repair"
		}
		if !slices.Contains(out, s) {
			out = append(out, s)
		}
	}
	if len(out) == 0 {
		return string(corpus.MethodBlank)
	}
	slices.Sort(out)
	return strings.Join(out, "+")
}

// Chapter assembles one chapter out of the pages of its volume.
//
// book is the Book of the Éléments the chapter belongs to, "alg", which is what
// a label is built on. It is passed in rather than read off the chapter,
// because a chapter records the volume it was bound in, "alg-viii", and the two
// are not the same thing: chapters I to VIII are one Book printed in three
// volumes, and a label that named the volume would move if the binding did.
//
// pages is every page of the volume by PDF page number, since a chapter reaches
// beyond its own first and last page: the § that opens the chapter shares a page
// with the chapter title, and the historical note ends where the volume's back
// matter begins.
func Chapter(book, lang string, ch corpus.Chapter, pages map[int]corpus.PageFile) ([]Piece, error) {
	pr, err := printingOf(lang)
	if err != nil {
		return nil, err
	}
	out, spans, err := marks(ch, pages, pr)
	if err != nil {
		return nil, err
	}
	last := chapterEnd(ch, pages, pr)
	runs := make([][][]part, len(out))
	for i, s := range spans {
		end := span{page: last + 1}
		if i+1 < len(spans) {
			end = spans[i+1]
		}
		parts, err := slice(pages, s, end)
		if err != nil {
			return nil, fmt.Errorf("chapter %s %s: %w", ch.Numeral, out[s.piece].Name(), err)
		}
		runs[s.piece] = append(runs[s.piece], parts)
	}
	for i := range out {
		parts := slices.Concat(runs[i]...)
		p := out[i]
		for _, r := range runs[i] {
			p.Runs = append(p.Runs, Run{
				First: r[0].page, Last: r[len(r)-1].page,
				FirstLabel: r[0].label, LastLabel: r[len(r)-1].label,
			})
		}
		for _, q := range parts {
			p.Methods = append(p.Methods, q.method)
		}
		p.Subsections = subsections(parts)
		blocks, notes := join(parts, pr)
		if !p.Front && !p.Historical {
			id := corpus.Ref{Book: book, Chapter: ch.Numeral, Section: p.Number, Appendix: p.Appendix}
			if blocks, p.Statements, err = statements(blocks, id, pr); err != nil {
				return nil, fmt.Errorf("chapter %s %s: %w", ch.Numeral, p.Name(), err)
			}
			blocks, p.HasExercise = anchorExercises(blocks, id, pr)
			if p.Exercises, err = exercises(blocks, pr); err != nil {
				return nil, fmt.Errorf("chapter %s %s: %w", ch.Numeral, p.Name(), err)
			}
			for i := range p.Exercises {
				m := &p.Exercises[i].Meta
				m.Book, m.Chapter, m.Section, m.Lang = id.Book, id.Chapter, id.Section, pr.lang
				m.Appendix = id.Appendix
				m.Label = p.Exercises[i].Ref().Label()
				body, left := takeNotes(corpus.NormalizeBody(p.Exercises[i].Body),
					p.Exercises[i].Pages, notes)
				p.Exercises[i].Body, notes = corpus.NormalizeBody(body), left
			}
			blocks = cutExercises(blocks, p.Number, p.Appendix, pr)
		}
		body, left := takeNotes(render(blocks), pagesOf(blocks), notes)
		// A definition nothing points at is a note whose mark was lost in
		// extraction, and appending it to the section quietly would hide that.
		if len(left) > 0 {
			return nil, fmt.Errorf("chapter %s %s: pdf page %d defines the footnote %s and nothing marks it",
				ch.Numeral, p.Name(), left[0].page, first(left[0].def, 40))
		}
		p.Body = corpus.NormalizeBody(body)
		if err := p.Verify(); err != nil {
			return nil, fmt.Errorf("chapter %s: %w", ch.Numeral, err)
		}
		out[i] = p
	}
	return out, nil
}

// span is one run of pages a piece is made of: where it begins, and which piece
// it belongs to.
//
// Most pieces are one run. A § whose exercises the chapter gathered at its end
// is two, printed a hundred pages apart, and that is the whole reason a piece is
// not simply the pages from one heading to the next. See gathered.
type span struct {
	page  int
	off   int
	piece int // index into the chapter's pieces

	// head and mark are the two edits a gathered run of exercises needs, and
	// are empty on every other run. See openRun.
	head string
	mark string
}

// marks finds where every piece of the chapter begins.
//
// The table of contents gives the page and the page gives the heading, and both
// have to agree. The page is what the run works from, since it is the only one
// of the two that says where on the page the section starts, and the heading is
// what says the page is the right one.
func marks(ch corpus.Chapter, pages map[int]corpus.PageFile, pr printing) ([]Piece, []span, error) {
	var out []Piece
	var body []span
	open := func(p Piece, page, off int) {
		out = append(out, p)
		body = append(body, span{page: page, off: off, piece: len(out) - 1})
	}
	off, _, err := find(pages, ch.PDFPage, pr.chapter)
	if err != nil {
		return nil, nil, fmt.Errorf("chapter %s: %w", ch.Numeral, err)
	}
	open(Piece{Front: true}, ch.PDFPage, off)
	for _, s := range ch.Sections {
		want := []string{fmt.Sprintf("## § %d.", s.Number)}
		if s.Appendix {
			want = pr.appendixHeads(s.Number)
		}
		off, prefix, err := find(pages, s.PDFPage, want...)
		if err != nil {
			return nil, nil, fmt.Errorf("chapter %s %s: %w", ch.Numeral, name(s), err)
		}
		// The heading and the contents entry are compared on what survives
		// both of them. See flat.
		head := headingText(pages[s.PDFPage].Body, off)
		title := strings.TrimPrefix(head, prefix)
		if flat(title) == "" {
			// An appendix whose heading is the word and the numeral alone,
			// with the title set under it. See titleUnder.
			title = titleUnder(pages[s.PDFPage].Body, off)
		}
		if got, want := flat(title), flat(s.Title); got != want {
			return nil, nil, fmt.Errorf("chapter %s %s: pdf page %d titles it %q, the table of contents calls it %q",
				ch.Numeral, name(s), s.PDFPage, title, s.Title)
		}
		open(Piece{Section: s}, s.PDFPage, off)
	}
	if ch.Historical != nil {
		off, _, err := find(pages, ch.Historical.PDFPage, pr.historical)
		if err != nil {
			return nil, nil, fmt.Errorf("chapter %s historical note: %w", ch.Numeral, err)
		}
		open(Piece{Historical: true}, ch.Historical.PDFPage, off)
	}
	for i := 1; i < len(body); i++ {
		if a, b := body[i-1], body[i]; b.page < a.page || (b.page == a.page && b.off <= a.off) {
			return nil, nil, fmt.Errorf("chapter %s: %s opens on page %d, which is not after %s on page %d",
				ch.Numeral, out[b.piece].Name(), b.page, out[a.piece].Name(), a.page)
		}
	}
	spans, err := gathered(out, body, pages, pr)
	if err != nil {
		return nil, nil, fmt.Errorf("chapter %s: %w", ch.Numeral, err)
	}
	return out, spans, nil
}

// gathered adds a run for every block of exercises the chapter prints away from
// the § it belongs to, and returns every run of the chapter in printed order.
//
// Algebra VIII prints the exercises of a § at the end of that §, and there is
// nothing to add: the block is already inside the pages the § covers. Lie 7 to 9
// gathers all the exercises of a chapter at the end of it, under one EXERCISES
// heading, and marks off inside that run which § or appendix each block belongs
// to. There the § is two runs of pages a hundred and fifty pages apart.
//
// Which of the two a chapter does is asked of the page and not worked out from
// the numbers, the same way the numbering of an appendix is. The table of
// contents already says what page each block of exercises begins on; that page
// either carries the printing's own Exercises heading, and the block is the §'s
// own, or it does not, and the block was gathered.
func gathered(pieces []Piece, body []span, pages map[int]corpus.PageFile, pr printing) ([]span, error) {
	out := slices.Clone(body)
	for _, b := range body {
		s := pieces[b.piece].Section
		if s.Exercises == nil {
			continue
		}
		page := s.Exercises.PDFPage
		if _, err := findLine(pages, page, func(l string) bool { return l == pr.exercises }); err == nil {
			continue
		}
		mark := runMark(s)
		off, err := findLine(pages, page, mark.MatchString)
		if err != nil {
			return nil, fmt.Errorf("%s: pdf page %d carries neither the heading %q nor a mark %s: %w",
				name(s), page, pr.exercises, mark, err)
		}
		// The chapter's own note on its exercises stands above the first mark,
		// under the gathered heading: chapter VII of Lie 7 to 9 says there that
		// its Lie algebras are finite dimensional and that k has characteristic
		// zero from § 3 on. It is printed where it is printed, immediately
		// before the exercises of § 1, and it is kept there rather than moved or
		// copied to each §, so that the file reads as the page does.
		head := off
		if at, err := findLine(pages, page, func(l string) bool { return l == pr.gathered }); err == nil && at < off {
			head = at
		}
		out = append(out, span{page: page, off: head, piece: b.piece,
			head: pr.exercises, mark: headingText(pages[page].Body, off)})
	}
	slices.SortFunc(out, func(a, b span) int {
		if a.page != b.page {
			return a.page - b.page
		}
		return a.off - b.off
	})
	for i := 1; i < len(out); i++ {
		if a, b := out[i-1], out[i]; a.page == b.page && a.off == b.off {
			return nil, fmt.Errorf("%s and %s both open at the same place on pdf page %d",
				pieces[a.piece].Name(), pieces[b.piece].Name(), a.page)
		}
	}
	return out, nil
}

// runMark is the mark a chapter that gathers its exercises puts at the head of
// the block belonging to one § or appendix.
//
// A § is marked with the sign and the number in bold and nothing else, and the
// gap after the sign is as wide as the press left it: of the 27 blocks of Lie 7
// to 9, 25 came out "§**1**" and 2 came out "§ **4**" off the same press, which
// is the same gap that the § headings themselves carry.
//
// An appendix is marked with its name, at whatever heading level extraction read
// off the size of the type. That is not one level: chapter VII of Lie 7 to 9
// marks its two "### Appendix I" and "### Appendix II" and chapter IX marks its
// one "## APPENDIX I", in the same volume off the same press, so the level and
// the case are both left out of the reading and only the word and the numeral
// are asked for.
func runMark(s corpus.Section) *regexp.Regexp {
	if s.Appendix {
		return regexp.MustCompile(fmt.Sprintf(`(?i)^#{1,4} +appendi[xc]e? +(?:%d|%s)\.?$`,
			s.Number, roman(s.Number)))
	}
	return regexp.MustCompile(fmt.Sprintf(`^§\s*\*\*%d\*\*\.?$`, s.Number))
}

// openRun writes the corpus's own exercises heading at the head of a gathered
// block and takes the volume's mark out of it.
//
// Everything downstream of assembly looks for that heading: it is what anchors
// "VIII, p. 15, Exercise 9", what cutExercises leaves behind in the section, and
// what tells exercises where the run begins. A volume that gathers its exercises
// writes the same fact a different way, so the fact is written the one way here
// and the difference stops at this line.
func openRun(body, head, mark string) string {
	lines := strings.Split(body, "\n")
	if len(lines) == 0 {
		return body
	}
	first := lines[0]
	lines[0] = head
	if first != mark {
		// The run opens on the gathered heading and the § is marked a few lines
		// below it, with the chapter's note on its exercises in between.
		for i, l := range lines {
			if l == mark {
				lines = slices.Delete(lines, i, i+1)
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

func name(s corpus.Section) string {
	if s.Appendix {
		return fmt.Sprintf("Appendix %d", s.Number)
	}
	return fmt.Sprintf("§ %d", s.Number)
}

// find is where on a page a heading with one of these prefixes begins, and
// which of them it was.
//
// There is more than one because the printings do not all number an appendix
// the same way. Algebra VIII heads its four APPENDIX 1 to APPENDIX 4 and Lie 7
// to 9 heads its two APPENDIX I and APPENDIX II, in the same language and the
// same series, so the caller offers both and the page says which it is. What
// comes back is the one that matched, since the title is checked against what
// follows it.
func find(pages map[int]corpus.PageFile, page int, prefixes ...string) (int, string, error) {
	p, ok := pages[page]
	if !ok {
		return 0, "", fmt.Errorf("pdf page %d has not been read", page)
	}
	for off := 0; off < len(p.Body); {
		for _, prefix := range prefixes {
			if strings.HasPrefix(p.Body[off:], prefix) {
				return off, prefix, nil
			}
		}
		i := strings.IndexByte(p.Body[off:], '\n')
		if i < 0 {
			break
		}
		off += i + 1
	}
	if len(prefixes) == 1 {
		return 0, "", fmt.Errorf("pdf page %d carries no heading %q", page, prefixes[0])
	}
	return 0, "", fmt.Errorf("pdf page %d carries no heading, of %q", page, prefixes)
}

// findLine is where on a page the first line the reader accepts begins.
func findLine(pages map[int]corpus.PageFile, page int, ok func(string) bool) (int, error) {
	p, found := pages[page]
	if !found {
		return 0, fmt.Errorf("pdf page %d has not been read", page)
	}
	for off := 0; off < len(p.Body); {
		if ok(headingText(p.Body, off)) {
			return off, nil
		}
		i := strings.IndexByte(p.Body[off:], '\n')
		if i < 0 {
			break
		}
		off += i + 1
	}
	return 0, fmt.Errorf("no line of pdf page %d is the one being looked for", page)
}

// flat reduces a title to the letters and digits in it, which is as much of one
// as the page and the table of contents can be relied on to agree about.
//
// The contents lists a title on one line, in title case, with the mathematics
// flattened out of it; the page sets the same title in capitals with the
// mathematics set as mathematics. § 1 of chapter VIII of Lie 7 to 9 is listed
// "The Lie algebra sl(2, k) and its representations" and headed "§ 1. THE LIE
// ALGEBRA $\mathfrak{s}\mathfrak{l}$(2$\boldsymbol{, k}$) AND ITS
// REPRESENTATIONS". Neither is wrong and neither can be turned into the other,
// since the capitals stop at the formula and the flattening does not, so what is
// compared is what is left when both are taken away. It is still enough to say
// the reading landed on the right page, which is all this check is for.
func flat(s string) string {
	var b strings.Builder
	for _, r := range texWord.ReplaceAllString(s, "") {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}

// texWord is the name of a TeX command, which is a word in the source and no
// part of the title: \mathfrak sets the letters after it, it does not add an M.
var texWord = regexp.MustCompile(`\\[a-zA-Z]+`)

// headingText is the heading line beginning at off.
func headingText(body string, off int) string {
	line := body[off:]
	if i := strings.IndexByte(line, '\n'); i >= 0 {
		line = line[:i]
	}
	return line
}

// titleUnder is the heading that stands immediately under the one at off, and
// is empty when the next thing on the page is not a heading.
//
// The two appendices of chapter VII of Lie 7 to 9 are headed "APPENDIX I
// POLYNOMIAL MAPS AND ZARISKI TOPOLOGY", the word and the numeral and the title
// on one line. The two of chapter IX are headed "APPENDIX I" with "STRUCTURE OF
// COMPACT GROUPS" set under it, larger, on a line of its own. It is the same
// book and the same kind of piece, so the difference is in the setting rather
// than in the structure, and both are read the same way.
func titleUnder(body string, off int) string {
	rest := body[off:]
	i := strings.IndexByte(rest, '\n')
	if i < 0 {
		return ""
	}
	for _, l := range strings.Split(rest[i+1:], "\n") {
		if strings.TrimSpace(l) == "" {
			continue
		}
		if h := strings.TrimLeft(l, "#"); h != l {
			return strings.TrimSpace(h)
		}
		return ""
	}
	return ""
}

// chapterEnd is the last page of the chapter.
//
// Nothing says where the last piece stops, so the search starts at the last page
// the table of contents names for anything in the chapter and runs on until it
// meets something belonging to neither the chapter nor the body of the book.
//
// That is either the back matter of the volume, which opens on a heading of its
// own, or the next chapter. The note of chapter VIII of Algebra runs from page
// 485 to 492 and page 493 is headed BIBLIOGRAPHY; the gathered exercises of
// chapter VII of Lie 7 to 9 run to page 76 and page 77 opens chapter VIII. A
// volume of one chapter never meets the second case, which is why it took a
// volume of three to find it.
func chapterEnd(ch corpus.Chapter, pages map[int]corpus.PageFile, pr printing) int {
	from := ch.PDFPage
	for _, s := range ch.Sections {
		from = max(from, s.PDFPage)
		if s.Exercises != nil {
			from = max(from, s.Exercises.PDFPage)
		}
	}
	if ch.Exercises != nil {
		from = max(from, ch.Exercises.PDFPage)
	}
	if ch.Historical != nil {
		from = max(from, ch.Historical.PDFPage)
	}
	last := from
	for p := from + 1; ; p++ {
		f, ok := pages[p]
		if !ok || strings.HasPrefix(f.Body, "# ") || strings.HasPrefix(f.Body, pr.chapter) {
			return last
		}
		last = p
	}
}

// part is the run of one page that belongs to one piece.
type part struct {
	page      int
	label     string
	method    corpus.PageMethod
	body      string
	continues bool
}

// slice cuts the pages from the head of one run to the head of the next into the
// parts of one piece.
func slice(pages map[int]corpus.PageFile, from, to span) ([]part, error) {
	var out []part
	for p := from.page; p <= to.page; p++ {
		if p == to.page && to.off == 0 {
			// The next piece opens at the head of its page, so this one ends on
			// the page before. The page after the last of a chapter is not
			// always a page of the volume at all, so it is not looked up.
			break
		}
		f, ok := pages[p]
		if !ok {
			return nil, fmt.Errorf("pdf page %d has not been read", p)
		}
		body, cont := f.Body, f.Meta.Continues
		if p == to.page {
			body = body[:to.off]
		}
		if p == from.page && from.off > 0 {
			// The piece opens partway down the page, so it opens on its own
			// heading and carries on nothing.
			body, cont = body[from.off:], false
		}
		if strings.TrimSpace(body) == "" {
			continue
		}
		out = append(out, part{page: p, label: f.Meta.PageLabel,
			method: f.Meta.Method, body: body, continues: cont})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("pdf pages %d to %d are empty", from.page, to.page)
	}
	if from.head != "" {
		out[0].body = openRun(out[0].body, from.head, from.mark)
	}
	return out, nil
}

// block is one paragraph, heading or display of the assembled text, with the
// page it was printed on.
//
// The page is carried because the corpus is page-indexed. Bourbaki cites itself
// by page ("VIII, p. 3, Proposition 3"), the exercises record the page they are
// set on, and once the pages are joined into a section nothing else can say
// which page a paragraph came off.
type block struct {
	text string
	page int
	// last is the page the block ends on, which is page unless a paragraph
	// broken by the end of a page was joined back up here. A footnote is
	// printed at the foot of the page its mark is on, so telling the two apart
	// is what lets a note find the block that marks it.
	last  int
	label string
}

func render(blocks []block) string {
	out := make([]string, 0, len(blocks))
	for _, b := range blocks {
		out = append(out, b.text)
	}
	return strings.Join(out, "\n\n")
}

// noteRE is a footnote definition as extraction writes it, at the head of a
// line, one to a line, at the foot of the page it was printed on.
var noteRE = regexp.MustCompile(`^\[\^([0-9a-zA-Z]+)\]:\s`)

// join puts the parts of a piece back into one text, and returns the footnote
// definitions gathered off the feet of its pages.
//
// Three things are put back here that a page cannot put back on its own. A
// paragraph broken by the end of a page is joined up, on the word of the page
// that follows: see the continues field, which extraction fills in from the
// indent of the first line. A word broken across the break is joined with it. A
// footnote is renumbered, because the book numbers them from one on each page
// and § 2 of chapter VIII has two notes both called 1, on pages 48 and 53.
//
// The definitions come back as a slice rather than as text at the foot of the
// piece, because the piece is not one file: the exercises are split off into
// files of their own and eight notes of chapter VIII are marked inside an
// exercise. Each note has to end up in the file that refers to it, which is
// what takeNotes does once the pieces of the section are known.
func join(parts []part, pr printing) ([]block, []note) {
	var blocks []block
	var notes []note
	for _, p := range parts {
		body, defs := cutNotes(p.body)
		body, defs = renumber(body, defs, len(notes)+1)
		for _, d := range defs {
			notes = append(notes, note{def: d, page: p.page})
		}
		bs := split(body)
		if len(bs) == 0 {
			continue
		}
		if p.continues && len(blocks) > 0 && joinable(blocks[len(blocks)-1].text, bs[0], pr) {
			blocks[len(blocks)-1].text = glue(blocks[len(blocks)-1].text, bs[0])
			blocks[len(blocks)-1].last = p.page
			bs = bs[1:]
		}
		for _, b := range bs {
			blocks = append(blocks, block{text: b, page: p.page, last: p.page, label: p.label})
		}
	}
	return blocks, notes
}

// note is one footnote definition and the page it was printed at the foot of.
type note struct {
	def  string
	page int
}

// takeNotes moves the footnotes belonging to this body out of defs and on to
// the foot of it, numbered from one, and gives back the ones still to be
// placed.
//
// A footnote belongs in the file its mark is in. Most of them are marked in the
// prose of the §, but eight of chapter VIII are marked inside an exercise, in
// §§ 1, 5, 7, 8, 9 and 21, and once the exercises are files of their own a
// definition left behind in the section is a mark pointing at nothing.
//
// The mark is not enough on its own to say which file a note belongs to, and
// pages says the rest. The book prints a footnote at the foot of the page its
// mark is on, so a note can only go to a file that holds part of that page, and
// without that rule a mark misread somewhere else in the § can take the note
// away from the text that really carries it: a subscript on page 449 came out
// as [^1] and walked off with the note of § 21, which is printed and marked
// thirty-four pages earlier.
//
// Each file numbers its notes from one, which is also what the book does on
// each page. The numbers join gave them are only there to keep the notes of one
// page apart from the notes of the next while the pages are being joined.
func takeNotes(body string, pages []int, defs []note) (string, []note) {
	var mine []string
	var rest []note
	for _, n := range defs {
		m := noteRE.FindStringSubmatch(n.def)
		if m != nil && slices.Contains(pages, n.page) && strings.Contains(body, "[^"+m[1]+"]") {
			mine = append(mine, n.def)
			continue
		}
		rest = append(rest, n)
	}
	body, mine = renumber(body, mine, 1)
	return body + tail(mine), rest
}

// tail is the footnote definitions as they are set at the foot of a file.
func tail(defs []string) string {
	if len(defs) == 0 {
		return ""
	}
	return "\n\n" + strings.Join(defs, "\n")
}

// pagesOf is every page these blocks came off, which is what says whether a
// footnote printed at the foot of a page belongs to them.
func pagesOf(blocks []block) []int {
	var out []int
	for _, b := range blocks {
		out = spanning(out, b)
	}
	return out
}

// spanning adds every page a block covers to pages.
func spanning(pages []int, b block) []int {
	for p := b.page; p <= max(b.page, b.last); p++ {
		if !slices.Contains(pages, p) {
			pages = append(pages, p)
		}
	}
	return pages
}

// cutExercises takes the block of exercises out of the section body and leaves
// a link to it.
//
// The exercises are written as one file each, so keeping them in the section as
// well would put every exercise of the chapter in the corpus twice, and a
// corpus with two copies of a text has two things to translate, two to tag and
// two to keep in step. What stays behind is the anchored heading, which is what
// a cross-reference to "VIII, p. 15, Exercise 9" points at, the preamble the
// book prints under it, and one line saying where the exercises went.
func cutExercises(blocks []block, section int, appendix bool, pr printing) []block {
	for i, b := range blocks {
		if !strings.HasPrefix(b.text, pr.exercises) {
			continue
		}
		out := blocks[: i+1 : i+1]
		for _, b := range blocks[i+1:] {
			at, _ := itemStart(b.text, 1)
			if at < 0 {
				out = append(out, b)
				continue
			}
			// Exercise 1 begins partway down the block, so what is in front of
			// it is the last of the preamble and the rest is an exercise.
			if head := strings.TrimSpace(b.text[:at]); head != "" {
				out = append(out, block{text: head, page: b.page, last: b.last, label: b.label})
			}
			break
		}
		dir := corpus.ExerciseDir(section, appendix)
		name := fmt.Sprintf("§ %d", section)
		if appendix {
			name = fmt.Sprintf("Appendix %d", section)
		}
		return append(out, block{text: fmt.Sprintf("See the [exercises for %s](exercises/%s/).", name, dir),
			page: b.page, label: b.label})
	}
	return blocks
}

// split is the blocks of a body, a block being a paragraph, a heading or a
// display.
func split(body string) []string {
	var out []string
	for _, b := range strings.Split(body, "\n\n") {
		b = imprint(strings.Trim(b, "\n"))
		if strings.TrimSpace(b) != "" {
			out = append(out, b)
		}
	}
	return out
}

// imprintRE is the publisher's line, which the volume prints at the foot of the
// first page of every chapter and of every piece of back matter.
var imprintRE = regexp.MustCompile(`©\s*(?:N\. Bourbaki|Springer Nature)[^\n]*?https://doi\.org/\S*`)

// imprint takes the publisher's line off a block.
//
// It is not part of the book and it is not ours to republish. Two of them fall
// inside chapter VIII: page 18 carries "© N. Bourbaki 2022 1 N. Bourbaki,
// Algebra, https://doi.org/..." as a paragraph of its own under the chapter
// title, and page 485 has one glued onto the end of footnote 2 of the
// historical note, which is why this trims rather than drops. A block that was
// nothing but the imprint comes back empty and split leaves it out.
func imprint(b string) string {
	if !strings.Contains(b, "©") {
		return b
	}
	return strings.TrimSpace(imprintRE.ReplaceAllString(b, ""))
}

// joinable says whether the last block of one page and the first of the next
// are two halves of one paragraph.
//
// A heading is never half of anything. A display is set on lines of its own and
// stays there: page 123 ends on one and page 124 opens on the table that
// follows it, and gluing the two would run a formula into a sentence. A
// footnote definition stands on its own line too.
//
// A page opening on a numbered item is the last case and the one that matters
// most. The volume sets its exercises with no paragraph indent, so the indent
// the page carries says it continues the page before it when it plainly does
// not: 21 pages of chapter VIII open on an exercise and 20 of them say so. What
// the text says is unambiguous where the indent is not, since a paragraph
// opening "5) " is the fifth exercise and never the tail of a broken sentence,
// and taking the text's word for it is what keeps § 1 from reporting four
// exercises where the book prints twenty-eight.
func joinable(prev, next string, pr printing) bool {
	if itemOpen(next) {
		return false
	}
	// A page that opens on a statement does not continue the paragraph before
	// it, whatever the indent said. Bourbaki sets a statement flush left, so a
	// page beginning with one looks exactly like a page beginning mid-paragraph
	// and extraction reads continues: true off it. Joined, the statement ends up
	// as prose inside the paragraph above and never gets a heading, a label or a
	// tag, and nothing in the corpus can point at it. This was 36 statements of
	// chapter VIII, found by the reference graph rather than by eye: the book
	// cites Theorem 1 of § 7 thirteen times and there was no Theorem 1 of § 7.
	if pr.head.MatchString(next) {
		return false
	}
	for _, s := range []string{prev, next} {
		if strings.HasPrefix(s, "#") || strings.HasPrefix(s, "$$") ||
			strings.HasSuffix(s, "$$") || noteRE.MatchString(s) {
			return false
		}
	}
	return true
}

// glue joins two halves of a paragraph.
//
// A word broken across the page break is put back together, which is what a
// trailing hyphen means, under the same reading extraction uses at the end of a
// line: the hyphen of A-module stays and the typesetter's goes. Measured on
// Algebra VIII this never fires, since not one of its 494 pages with a body ends
// in a hyphen; the two scanned volumes are set tighter and will.
func glue(prev, next string) string {
	prev = strings.TrimRight(prev, " ")
	next = strings.TrimLeft(next, " ")
	if !strings.HasSuffix(prev, "-") {
		return prev + " " + next
	}
	if compound(prev) {
		return prev + next
	}
	return strings.TrimSuffix(prev, "-") + next
}

// compound reports whether the hyphen the text ends on is part of the word
// rather than a break TeX put there.
//
// Two shapes say it is. One is a letter standing on its own, the A of A-module
// and the K of K-algebra, because TeX will not break a word after its first
// letter. The other is the close of inline mathematics, "$K$-algebra" and
// "$\mathbf{Q}$-vector space", which is the same compound with the letter set as
// mathematics, and there is no word there to have been broken at all.
func compound(s string) bool {
	s = strings.TrimSuffix(s, "-")
	if s == "" {
		return false
	}
	if s[len(s)-1] == '$' {
		return true
	}
	if !letter(s[len(s)-1]) {
		return false
	}
	if len(s) == 1 {
		return true
	}
	c := s[len(s)-2]
	return !letter(c) && (c < '0' || c > '9')
}

func letter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// cutNotes takes the footnote definitions off the foot of a page.
func cutNotes(body string) (string, []string) {
	var keep, defs []string
	for _, b := range split(body) {
		if noteRE.MatchString(b) {
			defs = append(defs, strings.Split(b, "\n")...)
			continue
		}
		keep = append(keep, b)
	}
	return strings.Join(keep, "\n\n"), defs
}

// renumber gives the footnotes of one page the numbers they will carry in the
// assembled section, in the definition and in the mark that points at it.
//
// The new number is written with a NUL in front of it until every note of the
// page is done, so that renumbering 2 to 3 and then 3 to 4 does not carry the
// first note along with the second.
func renumber(body string, defs []string, from int) (string, []string) {
	out := make([]string, 0, len(defs))
	for i, def := range defs {
		m := noteRE.FindStringSubmatch(def)
		if m == nil {
			out = append(out, def)
			continue
		}
		old := "[^" + m[1] + "]"
		want := fmt.Sprintf("[^\x00%d]", from+i)
		body = strings.ReplaceAll(body, old, want)
		out = append(out, want+strings.TrimPrefix(def, old))
	}
	body = strings.ReplaceAll(body, "\x00", "")
	for i := range out {
		out[i] = strings.ReplaceAll(out[i], "\x00", "")
	}
	return body, out
}
