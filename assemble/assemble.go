// Package assemble puts the pages of a chapter back into the sections the book
// is written in.
//
// Extraction reads one page at a time and stops at the edge of it, because a
// page is all it can see. A paragraph cut in half by the end of a page stays
// cut, a § that runs over twenty pages is twenty files, and a footnote is
// numbered from one on every page it appears on. This is where that is undone.
//
// Nothing here reads the PDF, and that is the contract rather than an accident:
// assembly is a pure function of pages/ and manifests/toc/, so it runs in CI
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
	"strconv"
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/mathtex"
	"github.com/tamnd/bourbaki-solver/textguard"
	"github.com/tamnd/bourbaki-solver/typography"
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
//
// The printed page arrives one of two ways and a volume uses one of them. A
// volume that prints a page label has it in FirstLabel and LastLabel, "A
// VIII.69". A volume that numbers its pages straight through the book and sets
// the number bare at the foot has nothing to put there and has the number in
// FirstFolio and LastFolio instead. Theory of Sets is the second kind, and
// until this was written its assembled sections said nothing at all about what
// pages of the book they were, so a reference to Set Theory III page 190 had
// nothing to land on.
type Run struct {
	First, Last           int
	FirstLabel, LastLabel string
	FirstFolio, LastFolio int
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
		parts, err := slice(pages, s, end, pr)
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
				FirstFolio: r[0].folio, LastFolio: r[len(r)-1].folio,
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
				p.Exercises[i].Body, notes = unstraddle(body), left
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
		p.Body = unstraddle(body)
		if err := p.Verify(); err != nil {
			return nil, fmt.Errorf("chapter %s: %w", ch.Numeral, err)
		}
		out[i] = p
	}
	return out, nil
}

// unstraddle normalises a joined body and moves back the brackets the printing
// swept into the mathematics: "Card(W$\varpi_1)$", where the bracket the
// sentence opened closes inside a span and the span then says something the page
// does not.
//
// It belongs here and not only in fix parens because this side of the join is
// where some of them are first visible. Extraction reads one page and stops at
// the edge of it, so a sentence that opens a bracket at the foot of one page and
// closes it at the head of the next holds nothing open as far as either page can
// see, and the repair that reads pages cannot make it. Conclude by induction on
// n.) at the end of A VIII § 1 Exercise 15 is one of ten across the corpus that
// only the assembled section shows.
//
// The other reason is that assembly is what writes these files. A repair made to
// an assembled section by anything else is thrown away the next time the book is
// assembled, and assemble -check would report it as drift for as long as it sat
// there. So the section comes out of the join already repaired and the two
// agree.
func unstraddle(body string) string {
	out, _ := mathtex.Unstraddle(body)
	// Tight, because the repair is the one thing in the pipeline that writes a
	// delimiter of its own rather than copying one off a page. A span it cuts in
	// two keeps the spaces that stood either side of the bracket it moved, so
	// "$... u_2 ) u_1$)" comes back as "$... u_2$ ) $ u_1$)", padded, and the
	// corpus writes an inline span tight. Nothing else here needs it: the pages
	// are tight, and a body copied off a tight page is tight already.
	//
	// It cannot go in Unstraddle itself. That function refuses its own output
	// unless everything but the dollars is unchanged, which is what keeps a
	// bracket repair from quietly editing the text, and taking a space out
	// would trip exactly that guard.
	out, _ = textguard.Tighten(out)
	// After, because the repair moves the space at the head of what is left of a
	// span outside the delimiter, and that space can land at the end of a line.
	return corpus.NormalizeBody(out)
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
	// A chapter the printing does not have gets no marker looked for and no
	// front matter opened, since the page the contents points at opens on the
	// first section. See corpus.Chapter.Nominal.
	if !ch.Nominal {
		off, _, err := find(pages, ch.PDFPage, pr.chapter)
		if err != nil {
			return nil, nil, fmt.Errorf("chapter %s: %w", ch.Numeral, err)
		}
		open(Piece{Front: true}, ch.PDFPage, off)
	}
	for _, s := range ch.Sections {
		want := sectionHeads(s.Number)
		if s.Appendix {
			want = pr.appendixHeads(s.Number)
		}
		page := s.PDFPage
		off, prefix, err := find(pages, page, want...)
		if err != nil && firstNoIsWhereTheContentsPoints(s) {
			if o, p, e := find(pages, page-1, want...); e == nil {
				page, off, prefix, err = page-1, o, p, nil
			}
		}
		if err != nil {
			return nil, nil, fmt.Errorf("chapter %s %s: %w", ch.Numeral, name(s), err)
		}
		// The heading and the contents entry are compared on what survives
		// both of them. See flat.
		head := headingText(pages[page].Body, off)
		title := strings.TrimPrefix(head, prefix)
		if flat(title) == "" && flat(s.Title) != "" {
			// An appendix whose heading is the word and the numeral alone,
			// with the title set under it. See titleUnder.
			//
			// Only where the contents says there is a title to find. Chapter IX
			// of Algebre commutative chapitres 8 et 9 closes on an appendix that
			// has none: the page prints APPENDICE centred and alone, the next
			// thing on it is the heading of no. 1, and the contents entry is the
			// word with nothing after it. Looking under the word there picks up
			// "1. Limite inductive d'anneaux locaux" and refuses the volume for
			// disagreeing with a contents entry that agreed already.
			title = titleUnder(pages[page].Body, off)
		}
		if !sameTitle(title, s.Title) && !Differs(pages[page].Meta.Book, page) {
			return nil, nil, fmt.Errorf("chapter %s %s: pdf page %d titles it %q, the table of contents calls it %q",
				ch.Numeral, name(s), page, title, s.Title)
		}
		open(Piece{Section: s}, page, off)
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
	spans, err := gathered(ch, out, body, pages, pr)
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
//
// A third case is the same block listed a different way. See chapterRuns.
func gathered(ch corpus.Chapter, pieces []Piece, body []span, pages map[int]corpus.PageFile, pr printing) ([]span, error) {
	out := slices.Clone(body)
	// heads is the gathered heading of a page, kept aside until the runs are in
	// printed order, because which run it belongs to is not known until then.
	heads := map[int]int{}
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
		// The case of the gathered heading is the press's and not the
		// printing's, the same way the case of an appendix mark is. Lie 7 to 9
		// sets it in capitals and the three recent French volumes set it
		// "Exercices", and both are the one heading over a chapter's worth of
		// exercises.
		if at, err := findLine(pages, page, func(l string) bool {
			return strings.EqualFold(l, pr.gathered)
		}); err == nil && at < off {
			if have, ok := heads[page]; !ok || at < have {
				heads[page] = at
			}
		}
		out = append(out, span{page: page, off: off, piece: b.piece,
			head: pr.exercises, mark: headingText(pages[page].Body, off)})
	}
	out = append(out, chapterRuns(ch, pieces, pages, pr, heads)...)
	slices.SortFunc(out, func(a, b span) int {
		if a.page != b.page {
			return a.page - b.page
		}
		return a.off - b.off
	})
	// The chapter's own note on its exercises stands above the first mark, under
	// the gathered heading: chapter VII of Lie 7 to 9 says there that its Lie
	// algebras are finite dimensional and that k has characteristic zero from
	// § 3 on. It is printed where it is printed, immediately before the
	// exercises of § 1, and it is kept there rather than moved or copied to each
	// §, so that the file reads as the page does.
	//
	// It belongs to the first run of the chapter's exercises and to no other.
	// The heading is a heading once and a running head after that: Theory of
	// Sets sets EXERCISES at the top of every verso page of the block, so pages
	// 130, 132 and 134 of the volume each carry one. Read as a heading every
	// time, a § marked at the top of such a page takes its run up over the tail
	// of the § before it. Exercises 7 to 11 of § 3 of chapter II stand on page
	// 132 above the mark for § 4, and they went into § 4's run, where 7 is not
	// the number that run is up to, so the volume lost five exercises it prints.
	//
	// Taking it once also settles the other way it goes wrong. Chapter I of
	// Theory of Sets marks § 1 and § 2 on the one page, its exercises for § 1
	// being five, and pulling both up to the heading opened two runs on the same
	// line.
	for i, s := range out {
		at, ok := heads[s.page]
		if !ok || at >= s.off {
			continue
		}
		out[i].off = at
		break
	}
	for i := 1; i < len(out); i++ {
		if a, b := out[i-1], out[i]; a.page == b.page && a.off == b.off {
			return nil, fmt.Errorf("%s and %s both open at the same place on pdf page %d",
				pieces[a.piece].Name(), pieces[b.piece].Name(), a.page)
		}
	}
	return out, nil
}

// chapterRuns is a run for every § marked inside a block of exercises the
// contents gives to the chapter rather than to a §.
//
// A volume that gathers its exercises can be listed either way. Lie 7 to 9
// prints one block at the end of a chapter and its contents names every § in
// it, "Exercises for § 3", so each § carries a locator of its own and the loop
// above has everything it needs. Topologie algebrique prints its four blocks
// the same way and its contents gives one line for each, "Exercices" and a page
// and nothing more. Read literally that line belongs to no §, and the four
// chapters came out with 0 of the 500 exercises the volume prints.
//
// The pages say what the contents leaves out. The block runs from the page the
// contents names to the end of the chapter, the §§ inside are marked as they
// are in the volumes above, and each mark opens the run of the § it names. A §
// the block does not mark gets nothing, which is the honest reading: it has no
// exercises.
//
// The locator is written back onto the § because it is what the rest of the
// assembler asks. Verify refuses a piece whose pages carry exercises the
// contents gives none of, and it is right to: a mark read by mistake would
// otherwise go by in silence. The contents does give these, once for the whole
// chapter, and this is the reading of that line.
func chapterRuns(ch corpus.Chapter, pieces []Piece, pages map[int]corpus.PageFile,
	pr printing, heads map[int]int) []span {
	if ch.Exercises == nil {
		return nil
	}
	var out []span
	for page := ch.Exercises.PDFPage; page <= chapterEnd(ch, pages, pr); page++ {
		f, ok := pages[page]
		if !ok {
			continue
		}
		for i := range pieces {
			s := pieces[i].Section
			if pieces[i].Front || pieces[i].Historical || s.Exercises != nil {
				continue
			}
			off, err := findLine(pages, page, runMark(s).MatchString)
			if err != nil {
				continue
			}
			pieces[i].Section.Exercises = &corpus.Locator{Page: printedNumber(f), PDFPage: page}
			out = append(out, span{page: page, off: off, piece: i,
				head: pr.exercises, mark: headingText(f.Body, off)})
		}
		if at, err := findLine(pages, page, func(l string) bool {
			return strings.EqualFold(l, pr.gathered)
		}); err == nil {
			if have, ok := heads[page]; !ok || at < have {
				heads[page] = at
			}
		}
	}
	return out
}

// printedNumber is the page number the volume prints on a page, which is the
// folio where it prints one and the tail of the label where it prints a label
// instead: Topologie algebrique heads its pages "A I.139" and the number of
// that page is 139.
func printedNumber(f corpus.PageFile) int {
	if f.Meta.Folio > 0 {
		return f.Meta.Folio
	}
	label := f.Meta.PageLabel
	if i := strings.LastIndexByte(label, '.'); i >= 0 {
		label = label[i+1:]
	}
	n, err := strconv.Atoi(strings.TrimSpace(label))
	if err != nil {
		return 0
	}
	return n
}

// runMark is the mark a chapter that gathers its exercises puts at the head of
// the block belonging to one § or appendix.
//
// A § is marked with the sign and the number and nothing else, and the gap after
// the sign is as wide as the press left it: of the 27 blocks of Lie 7 to 9, 25
// came out "§**1**" and 2 came out "§ **4**" off the same press, which is the
// same gap that the § headings themselves carry.
//
// What the mark is set in is the press's business too. Lie 7 to 9 sets it in
// bold at the size of the text and it is read as text; the three recent French
// volumes set it in the face and at the size they set a § heading in, so it is
// read as a heading and reaches here "## § 1". Neither is a heading of the
// corpus, since the § it names already has one a hundred pages back, so the
// level and the bold are both left out of the reading, exactly as the level and
// the case of an appendix mark are, and what is asked for is the sign and the
// number.
//
// An appendix is marked with its name, at whatever heading level extraction read
// off the size of the type. That is not one level: chapter VII of Lie 7 to 9
// marks its two "### Appendix I" and "### Appendix II" and chapter IX marks its
// one "## APPENDIX I", in the same volume off the same press, so the level and
// the case are both left out of the reading and only the word and the numeral
// are asked for.
//
// Nor is the name one word. Integration 7 to 9 calls its appendix an ANNEX
// throughout, in the running head, over the opener and over the block of
// exercises at the end of chapter IX, and the French volumes have ANNEXE beside
// APPENDICE for the same reason. Both words are taken here so that the page can
// keep the word it prints.
// appendixWord is the words a volume of the Éléments heads an appendix with, as
// an alternation for a pattern that is already case folded. See runMark.
const appendixWord = `(?:appendi[xc]e?|annexe?)`

func runMark(s corpus.Section) *regexp.Regexp {
	if s.Appendix {
		// A chapter with one appendix does not number it, and the page prints
		// the word by itself. Chapter I of Theory of Sets does, and the table of
		// contents gives it no numeral either, so a mark that insists on one
		// finds nothing on the page that carries it and the whole volume stops
		// assembling on a page that is perfectly well read.
		if s.Number == 0 {
			return regexp.MustCompile(`(?i)^#{1,4} +` + appendixWord + `\.?$`)
		}
		return regexp.MustCompile(fmt.Sprintf(`(?i)^#{1,4} +`+appendixWord+` +(?:%d|%s)\.?$`,
			s.Number, roman(s.Number)))
	}
	// The mark is display type and the reading sets display type bold often
	// enough that the bold has to be allowed for. It was already allowed for
	// around the number and not around the whole mark, which is the other place
	// it lands: page 583 of Algebre I a III writes "**§ 9**" and "**§ 10**", and
	// those two lines are the only ones in the corpus the narrower pattern
	// refuses. Refusing them stopped chapter III of that volume on a page that
	// carries both marks plainly enough for a person to read at a glance.
	return regexp.MustCompile(fmt.Sprintf(`^(?:#{1,4} +)?(?:\*\*)?§\s*(?:\*\*)?%d(?:\*\*)?\.?(?:\*\*)?$`, s.Number))
}

// SectionMark is the line a printing marks off a § with inside a block of
// exercises it gathers, and it is exported for the same reason
// HistoricalNoteHead is: the repair that puts a dropped mark back has to ask
// what the assembler looks for rather than keep its own copy of the answer.
func SectionMark(s corpus.Section) *regexp.Regexp { return runMark(s) }

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

// sectionHeads is the heading over § n, both ways a volume prints it.
//
// Five of the six volumes print the sign: "§ 1. PRIMARY DECOMPOSITION OF LINEAR
// REPRESENTATIONS". Theory of Sets prints the number alone, "2. THEOREMS",
// centred and set at the size the others set a § heading at, and its table of
// contents lists the same section as "§ 2. Theorems". The sign is the press's
// business and not the structure's, so both forms are offered and the page says
// which it is, the same way the numeral over an appendix is asked of the page.
//
// The bare form cannot be read as a no. by mistake. A no. is set smaller and
// reaches here at "### 3.", one level down, so the two are told apart by the
// level before the number is ever looked at.
func sectionHeads(n int) []string {
	return []string{fmt.Sprintf("## § %d.", n), fmt.Sprintf("## %d.", n)}
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

// firstNoIsWhereTheContentsPoints says whether a section and its first no. are
// listed on the same page, which is what an entry looks like when the contents
// has given the page the section's text starts on rather than the page its
// heading is set on.
//
// Section 2 of chapter VI of Lie 4 to 6 is listed at page 186 and so is its
// no. 1, and page 186 opens on the no. 1 heading with the § heading and the
// paragraph that introduces the § over on 185. Both entries are pointing at the
// same thing, and only one of them is a heading, so the § has to be looked for
// on the page before as well. A section whose first no. is listed on an earlier
// page has an entry that is about the section itself and is left alone.
func firstNoIsWhereTheContentsPoints(s corpus.Section) bool {
	return len(s.Subsections) > 0 && s.Subsections[0].PDFPage == s.PDFPage
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
// The one accent a printing of this corpus is known to drop comes off first.
// Upper casing does not take it off, since an accented small letter upper cases
// to an accented capital, and page 38 of Algebre I a III prints "GROUPES ET
// GROUPES A OPERATEURS" for a § its own table of contents lists with a grave on
// the a. Every other accent is still compared as it stands, which is what keeps
// a heading that lost one from passing for the heading it came from. See
// typography.Accentless.
func flat(s string) string {
	var b strings.Builder
	for _, r := range typography.Accentless(texWord.ReplaceAllString(s, "")) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(unicode.ToUpper(r))
		}
	}
	return b.String()
}

// texWord is the name of a TeX command, which is a word in the source and no
// part of the title: \mathfrak sets the letters after it, it does not add an M.
var texWord = regexp.MustCompile(`\\[a-zA-Z]+`)

// sameTitle says whether a heading and a contents entry name the same piece.
//
// The two are compared on what flat leaves of them, and failing that on what is
// left when each has dropped a leading article. Chapter V of Lie 4 to 6 heads
// its fourth section "§ 4. GEOMETRIC REPRESENTATION OF A COXETER GROUP" and its
// table of contents lists the same section as "The geometric representation of a
// Coxeter group". Both are printed and neither is a misreading, so the article
// is a matter of setting rather than of naming, the same way the capitals are.
// differs are the openings a volume gives two titles, written as the volume and
// the pdf page the heading is on.
//
// A heading that does not say what the contents says is normally a misreading of
// one of the two and is meant to stop the volume, since the page and the entry
// come off different pages of the same book and the reading of either can go
// wrong. sameTitle above is the whole of what a printing is allowed to vary
// without an entry here, and every line of it is a habit of the press rather
// than a difference in the words.
//
// This is for the words. A printing that heads a section one way and lists it
// another in its own table of contents is disagreeing with itself, no rule can
// tell which of the two it meant, and the corpus keeps both: the page keeps its
// own heading and the manifest keeps the contents entry. So the entry here is
// only the page, and it is one somebody has looked at twice, once at the heading
// and once at the contents line.
//
// lie-i-iii is the one entry so far. Page 337 of Lie Groups and Lie Algebras
// chapters 1 to 3, which is pdf page 355, heads its eighth section "§ 8. LIE
// GROUPS OVER R AND Q_p", and page xv of the same volume lists that section as
// "§ 8. Lie groups over R or Q_p". Both page images are plain and both readings
// are right.
var differs = map[string][]int{
	"alg-x-fr":  {67},
	"lie-i-iii": {355},
	"var-fr":    {29},
}

// Differs is whether the opening on this page of this volume is one the volume
// gives two titles, recorded above.
//
// It is asked outside the assembler by fix opening, which refuses to write a
// heading the contents does not agree with and is right to: a disagreement is a
// misreading of one side or the other far more often than it is the printing.
// Where the disagreement is recorded, the question has been settled already,
// and the repair is then the ordinary one. The page keeps its own words, since
// that is the whole of what the record says.
func Differs(book string, page int) bool {
	return slices.Contains(differs[book], page)
}

// The footnote markers come off both sides before anything else. A printing
// hangs one off the end of a heading where the § or the chapter has a note on
// it, and the contents entry never carries it: page 69 of Groupes et algebres de
// Lie IX heads § 7 that way and page 263 of Espaces vectoriels topologiques I a
// V heads chapter V that way. flat keeps the digit out of the marker, so the two
// sides came out differing by a 1 and the volume stopped. See
// typography.Footless.
func sameTitle(head, entry string) bool {
	head, entry = typography.Footless(head), typography.Footless(entry)
	if flat(head) == flat(entry) {
		return true
	}
	return flat(leadArticle.ReplaceAllString(head, "")) ==
		flat(leadArticle.ReplaceAllString(entry, ""))
}

// leadArticle is an article standing at the head of a title, in either of the
// two languages the corpus holds. It is only ever dropped from both sides at
// once, and only after the titles have already failed to agree as they stand,
// so a title that really begins with one of these words is not shortened by it.
var leadArticle = regexp.MustCompile(`^[\s*_]*(?i:the|an|a|les|le|la|l['’]|une|un)\b['’]?[\s*_]*`)

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
	folio     int
	method    corpus.PageMethod
	body      string
	continues bool
}

// slice cuts the pages from the head of one run to the head of the next into the
// parts of one piece.
func slice(pages map[int]corpus.PageFile, from, to span, pr printing) ([]part, error) {
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
		lo, hi := 0, len(body)
		if p == to.page {
			hi = to.off
		}
		if p == from.page && from.off > 0 {
			// The piece opens partway down the page, so it opens on its own
			// heading and carries on nothing.
			lo, cont = from.off, false
		}
		if lo > 0 || hi < len(body) {
			body = cutPage(body, lo, hi)
		}
		if from.head != "" && p != from.page {
			body = dropRunningHead(body, pr.gathered)
		}
		if strings.TrimSpace(body) == "" {
			continue
		}
		out = append(out, part{page: p, label: f.Meta.PageLabel, folio: f.Meta.Folio,
			method: f.Meta.Method, body: body, continues: cont})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("pdf pages %d to %d are empty", from.page, to.page)
	}
	fillFolios(out)
	if from.head != "" {
		out[0].body = openRun(out[0].body, from.head, from.mark)
	}
	return out, nil
}

// fillFolios gives a page that prints no number the number it stands at.
//
// A page that opens a chapter or a block of exercises carries no running head,
// and a volume that prints its page number in the head therefore says nothing
// on such a page about which page of the book it is. Lie 7 to 9 opens the
// exercises of three § that way. Taken as it stands, § 1 of each of its three
// chapters came out printed on no page of the book at all, because a run with
// no number at one end is written as no range, and the eleven exercises printed
// on those three pages came out the same way.
//
// The number is not in doubt. The pages of a piece are consecutive in the
// printing as much as in the file, so a page with no number of its own is a
// numbered page of the same piece counted along to it. Nothing is written back
// on to the page by this. What is being asked for is which page of the book a
// piece of text is printed on, and not what any one page prints.
func fillFolios(parts []part) {
	k := -1
	for i, p := range parts {
		if p.folio > 0 {
			k = i
			break
		}
	}
	if k < 0 {
		return // a volume that prints a page label instead, or none at all
	}
	for i := range parts {
		if parts[i].folio == 0 {
			parts[i].folio = parts[k].folio + parts[i].page - parts[k].page
		}
	}
}

// dropRunningHead takes the gathered exercises heading off a page in the middle
// of a gathered run, where it is the running head of the page and not a heading
// of anything.
//
// A volume that gathers its exercises prints the heading once over the block and
// then prints it again at the top of every page of the block, or of every verso
// page of it: Theory of Sets sets EXERCISES on pages 130, 132, 134 and on down
// the chapter. The first is the heading the run opens on and the rest say only
// what page the reader is on, so they are worth nothing to a file that is one §
// from its head to its foot, and left in they end a run early.
func dropRunningHead(body, head string) string {
	lines := strings.Split(body, "\n")
	out := lines[:0]
	for _, l := range lines {
		if strings.EqualFold(l, head) {
			continue
		}
		out = append(out, l)
	}
	return strings.Join(out, "\n")
}

// cutPage is the part of a page between two offsets, with the footnote
// definitions of the whole page put on the side of the cut that marks them.
//
// A footnote is printed at the foot of the page, below everything, and a piece
// boundary falls in the middle of the page. So the definition of a note marked
// in the text above the boundary sits physically below it, and a plain cut hands
// it to the piece that follows. Page 67 of Theory of Sets is exactly that: the
// last exercise of § 4 ends "(Take R to be the relation x = x ...) (*)", the
// APPENDIX heading opens two lines later, and the note the asterisk points at is
// at the foot of the page under both of them. Cut plainly, § 4 has a mark with
// nothing behind it and the appendix has a note nothing points at, which is what
// Chapter reports.
//
// So the definitions are read off the whole page and given to the side that
// carries the mark, whichever side of the cut they were printed on. The mark
// counts either as the reference extraction was asked for or as the mark the
// book printed, because at this point in the run the reference has not been put
// back yet: see markNotes.
func cutPage(page string, lo, hi int) string {
	cut := page[lo:hi]
	_, defs := cutNotes(page)
	// The mark is looked for in the text of the cut and not in the definitions
	// standing at the foot of it, since every definition opens on the mark it
	// defines and would otherwise be found marking itself.
	text, _ := cutNotes(cut)
	var keep []string
	for _, d := range defs {
		if noteRE.FindStringSubmatch(d) == nil {
			continue // a note that runs to a second line, which stays with its first
		}
		here, mine := strings.Contains(cut, d), marksNote(text, d)
		switch {
		case here && !mine:
			cut = strings.Replace(cut, d+"\n", "", 1)
			cut = strings.Replace(cut, d, "", 1)
		case !here && mine:
			keep = append(keep, d)
		}
	}
	cut = strings.TrimRight(cut, "\n ")
	if len(keep) > 0 {
		cut += "\n\n" + strings.Join(keep, "\n")
	}
	return cut
}

// marksNote says a text carries the mark of a footnote definition.
func marksNote(text, def string) bool {
	n := noteRE.FindStringSubmatch(def)
	if n == nil {
		return false
	}
	if strings.Contains(text, "[^"+n[1]+"]") {
		return true
	}
	mark := headMarkRE.FindString(strings.TrimPrefix(def, n[0]))
	if mark == "" {
		return false
	}
	for _, m := range printedMarkRE.FindAllString(text, -1) {
		if markKey(m) == markKey(mark) {
			return true
		}
	}
	return false
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
	// folio is the number printed on the page this block starts on, for the
	// volumes that print a number and no label. See bookPage.
	folio int
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
// indent of the first line, and mends, which reads the text either side of the
// break where the indent did not carry. A word broken across the break is
// joined with it. A footnote is renumbered, because the book numbers them from
// one on each page and § 2 of chapter VIII has two notes both called 1, on
// pages 48 and 53.
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
		body = markNotes(body, defs)
		body, defs = renumber(body, defs, len(notes)+1)
		for _, d := range defs {
			notes = append(notes, note{def: d, page: p.page})
		}
		bs := split(body)
		if len(bs) == 0 {
			continue
		}
		if len(blocks) > 0 && (p.continues || mends(blocks[len(blocks)-1].text, bs[0])) &&
			joinable(blocks[len(blocks)-1].text, bs[0], pr) {
			blocks[len(blocks)-1].text = glue(blocks[len(blocks)-1].text, bs[0])
			blocks[len(blocks)-1].last = p.page
			bs = bs[1:]
		}
		for _, b := range bs {
			blocks = append(blocks, block{text: b, page: p.page, last: p.page, label: p.label, folio: p.folio})
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
				out = append(out, block{text: head, page: b.page, last: b.last, label: b.label, folio: b.folio})
			}
			break
		}
		dir := corpus.ExerciseDir(section, appendix)
		name := fmt.Sprintf("§ %d", section)
		if appendix {
			name = fmt.Sprintf("Appendix %d", section)
		}
		return append(out, block{text: fmt.Sprintf("See the [exercises for %s](exercises/%s/).", name, dir),
			page: b.page, label: b.label, folio: b.folio})
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

// mends says whether the text either side of a page break is one sentence cut
// in half, for a break the indent said was a paragraph boundary.
//
// The indent is the only thing extraction has to go on and it is not always
// there to be read. A page whose first line is a display, or is set flush left
// because the line before it ended full, or whose first line the reader took a
// running head off, comes out continues: false with a sentence running straight
// through it. Measured over the seven assembled books there are 2959 page
// junctions and 108 of them are of this kind, which is 3.7 %, and the text says
// what the indent did not.
//
// Two things have to hold at once and neither is enough alone. The page before
// has to end on no full stop, which for prose set by Bourbaki is already
// unusual: a paragraph ends on one. And the page after has to open on nothing
// that starts something of its own, which rules out the lettered parts of an
// exercise, a dash opening a list, a line of capitals such as TABLE 2, and any
// word beginning with a capital.
//
// The capital is the part that costs something and it is kept anyway. It turns
// down 46 junctions, and joinable turns 25 of those down as well on a heading or
// a display, so 21 are its own. Seven of the 21 are sentences broken at a word
// set as mathematics, "be the relations of the form" carrying on into
// "$S_{i_1}$", and leaving them broken is a real loss. The other fourteen open a
// lettered part of an exercise, a dash list, a line of capitals such as TABLE 2,
// a statement set in bold, a commutative diagram, the tail of the Springer
// imprint, a row of a table read out of order, and a citation whose full stop is
// inside the emphasis: joining any of those runs two unrelated things together
// in the middle of a paragraph, where nothing downstream can see it and no rule
// can catch it. A junction left broken is visible to a reader and repairable
// later; a junction wrongly joined is not.
//
// What is left after this is 20 junctions of the 2959, which is 0.7 %, and every
// one of the 20 is a page that opens on a display. A display is a paragraph of
// its own whatever the page before it ended on, so those are left where they are
// and the 3 % the milestone asks for is met with room to spare.
func mends(prev, next string) bool { return unstopped(prev) && !opener(next) }

// unstopped says the text ends on no full stop.
//
// Emphasis and the closing halves of brackets and quotes are taken off first,
// since the stop of "*uniquely determined by this condition.*" is inside the
// emphasis and the stop of "(VIII, p. 267, exerc. 11)" is inside the brackets.
// A display is stopped whatever it ends on, because a display is a paragraph.
func unstopped(s string) bool {
	s = strings.TrimRight(s, ` *_)]}"'”’`)
	if s == "" || strings.HasSuffix(s, "$$") {
		return false
	}
	// A stop set as mathematics is still a stop. Exercise 11 of § 7 of Lie VIII
	// ends its page on "in a variable T, with coefficients in $\mathbf{Q}[\Delta
	// ]$)$:$" and the page after opens on the sum the colon introduces, so the
	// dollar on the end hid the one thing that said not to join them.
	if t := strings.TrimSuffix(s, "$"); t != s && t != "" {
		if r := []rune(t); strings.ContainsRune(".?!:;", r[len(r)-1]) {
			return false
		}
	}
	r := []rune(s)
	return !strings.ContainsRune(".?!:;", r[len(r)-1])
}

// opener says the text begins something of its own rather than carrying a
// sentence on.
//
// joinable already turns down a heading, a statement, a numbered exercise, a
// display and a footnote. What is left to this are the three shapes that open
// something without being any of those: the lettered part of an exercise, a) or
// b), which the volumes set the same way they set the numbers; a dash opening
// an item of a list; and a line of capitals, which in these volumes is the head
// of a table.
//
// A capital letter is the fourth, and is only read this way where the indent
// has already said the paragraph is new. Inside a paragraph a capital after a
// full stop is the next sentence and says nothing at all.
func opener(s string) bool {
	if lettered.MatchString(s) || capitals.MatchString(s) {
		return true
	}
	for _, r := range s {
		if unicode.IsLetter(r) {
			return unicode.IsUpper(r)
		}
	}
	return false
}

// lettered is the a) of an exercise, the (i) of a case, or the dash of a list,
// at the head of the text, emphasis and all.
var lettered = regexp.MustCompile(`^(\*{0,2}[a-z]\)|\(\s*[ivx]+\s*\)|[-–—•]\s)`)

// capitals is a line opening on a run of them, which is how these volumes head
// a table.
var capitals = regexp.MustCompile(`^[A-Z][A-Z ]{3,}`)

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

// printedMarkRE is the mark the book itself prints at a footnote. Bourbaki does
// not number its notes, it marks them with an asterisk, then two asterisks, then
// a dagger, then a double dagger, starting again on each page. The mark stands
// in the text and again at the head of the definition, and extraction writes it
// in both places as the page set it, sometimes as mathematics and sometimes not.
var printedMarkRE = regexp.MustCompile(`\$?\(\s*(?:(?:\\?\*)+|\\?†|\\?‡)\s*\)\$?`)

// headMarkRE is the same mark where a definition opens on it.
var headMarkRE = regexp.MustCompile(`^` + printedMarkRE.String())

// markNotes puts back the reference to a footnote that a page printed the mark
// for and never wrote the reference for.
//
// Extraction is asked for two things at a footnote: the mark in the text
// becomes "(*)[^1]", the printed mark kept as a transcription and the reference
// added so that the definition has something pointing at it. Pages that were
// read by a model do the first half more reliably than the second. Theory of
// Sets prints 51 notes and 30 of them came back with the mark alone, "an axiom
// of $\mathscr{T}$ (\*)." against a definition reading "[^1]: (\*) This scheme
// may be expressed", so the definition had nothing pointing at it and the
// chapter would not assemble.
//
// Nothing is guessed. The definition carries the printed mark at its head, that
// is the mark the page set, and the reference goes after the one place in the
// body where the same mark stands. A mark that stands nowhere, or in more than
// one place, is left exactly as it was for the error in Chapter to report,
// because a note put at the wrong mark is worse than a note with no mark.
//
// The printed mark stays where it is rather than being replaced, because that is
// what the pages that were read properly look like: page 22 gives "the following
// signs of a mathematical theory $\mathscr{T}$ (*)[^1] are", with both. The mark
// is the book's and the reference is this corpus's, and neither says the other.
func markNotes(body string, defs []string) string {
	for _, def := range defs {
		n := noteRE.FindStringSubmatch(def)
		if n == nil || strings.Contains(body, "[^"+n[1]+"]") {
			continue
		}
		mark := headMarkRE.FindString(strings.TrimPrefix(def, n[0]))
		if mark == "" {
			continue
		}
		at := -1
		for _, loc := range printedMarkRE.FindAllStringIndex(body, -1) {
			if markKey(body[loc[0]:loc[1]]) != markKey(mark) {
				continue
			}
			if at >= 0 {
				at = -1 // the page sets this mark twice, so no one place is the one
				break
			}
			at = loc[1]
		}
		if at < 0 {
			continue
		}
		body = body[:at] + "[^" + n[1] + "]" + body[at:]
	}
	return body
}

// markKey is a printed mark with the decoration the page happened to set it in
// taken off, so that the "(\*)" of a definition is the same mark as the "(*)"
// of the text it belongs to. Page 22 of Theory of Sets writes it both ways.
func markKey(s string) string {
	return strings.NewReplacer(`\`, "", "$", "", " ", "").Replace(s)
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
