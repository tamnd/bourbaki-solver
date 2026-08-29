package toc

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
)

// Reading a volume's contents off its own pages, for the volumes that have no
// contents page to read.
//
// Parse takes the list the publisher printed and is the right way round: the
// list states, in one place, where every § and every no. begins, and reading it
// needs no judgement about what a heading is. Three volumes in this corpus
// cannot be read that way and between them they hold 667 pages that nothing
// downstream can touch.
//
//	ac-x-fr           180 pages   the scan opens on the first page of the
//	                              chapter and carries no front or back matter
//	                              at all, so there is no contents page in the
//	                              file and none can be made to appear
//	lie-vii-viii-fr    61 pages   the same, and the file holds only chapter VII
//	                              although the manifest names two chapters
//	alg-iv-vii-fr     426 pages   the contents is in the file and is short two
//	                              leaves. It lists chapters IV, V and VII and
//	                              says nothing whatever about chapter VI, and
//	                              the parser refuses the whole volume for it
//
// So the headings are read off the body instead. Every one of them is on the
// paper: the press sets the § at the head of its first page and each no. at the
// head of its own, and the running head of a recto carries the § title on every
// page of the §. What the body cannot give is the printed page of an entry,
// and it does not have to, because the page map already knows which printed
// page every pdf page is.
//
// This is a weaker reading than Parse and it says so. The list a contents page
// gives is the press's own and is complete by construction; a heading swept out
// of a body is only as complete as the reading of that body, and a page that
// came back short takes its headings with it. Every entry here therefore
// carries where it was found, and the problems name what is missing rather than
// filling it in.

// BodyPage is one page of a volume as this corpus read it.
type BodyPage struct {
	PDFPage     int
	RunningHead string
	// Section is the § the running head names, from the locator the reading
	// files, and 0 for a page whose head names none. It is what says which §
	// a run of exercises belongs to.
	Section int
	Body    string
}

// bodySection is "§ 4. Title", the § heading as the press sets it at the head
// of the §'s first page.
//
// Two of the three volumes set it and one does not. Chapter VII of Lie writes
// "§ 1. Décomposition primaire des représentations linéaires" in lower case and
// chapters IV to VII of Algebre write "§ 1. POLYNÔMES" in capitals, and both
// come through here. Chapter X of Algebre commutative sets its § headings the
// same way on the paper, but the reading of that volume did not keep a single
// one of them, so for that volume the §§ are found another way. See restart.
var bodySection = regexp.MustCompile(`^(?:§|\\S)\s*(\d{1,2})\s*\.\s*(\S.*)$`)

// bodyNumber is "3. Décomposition des représentations", the no. heading.
//
// The shape is common enough in running prose that the pattern alone cannot be
// trusted, and it is not trusted: a line matching this is taken only when its
// number is the one that comes next in the §. That ordinal constraint is what
// keeps it off "2) Soit x un élément", off "3. cf. p. 51" in a note and off the
// enumerated case of a proof, all of which appear in these three volumes and
// none of which arrive in the right order to be mistaken for a heading.
//
// The title has to open with a capital because a no. heading always does and a
// line of prose broken after a number rarely does. The guillemet and the two
// quotes are there because chapter X of Algebre commutative opens two no. with
// a quoted term.
var bodyNumber = regexp.MustCompile(`^(\d{1,2})\s*\.\s*([A-ZÀ-ÝÉÈÊÎÔÛÇ«"'’].{2,110})$`)

// headNumber is the "N° 7 " a recto running head carries in front of the §
// title in the head-label printings. It comes off before the head is used as a
// title.
var headNumber = regexp.MustCompile(`^\s*(?:N|n)\s*[°ºo]\s*\d{1,2}\s+`)

// headAppendix is a running head that names an appendix rather than a §, in the
// two forms the printings use. Page 46 of chapter VII of Lie heads the verso
// "Ch. VII, App. 1" and page 45 heads the recto "APPENDICE I".
var headAppendix = regexp.MustCompile(`(?i)\bapp(?:endice)?\.?\s+([IVX]+|\d{1,2})\b`)

// bodyChapterLine is "CHAPITRE VII", the line naming the chapter rather than
// titling it, in the two languages this reads.
var bodyChapterLine = regexp.MustCompile(`(?i)^(?:chapitre|chapter)\s+[IVXLC\d]+\.?$`)

// bodyRunMark is "§ 1", the line a printing marks a § off with inside a block of
// exercises it gathers. It is assemble.SectionMark written again here rather than
// called, because the assembler reads this package and not the other way round.
var bodyRunMark = regexp.MustCompile(`^§\s*(\d{1,2})\.?$`)

// bodyAppendixLine is "APPENDICE 1", the word standing alone over an appendix,
// which is how fix opening leaves the page it has repaired.
var bodyAppendixLine = regexp.MustCompile(
	`(?i)^(?:appendice|appendix|annexe)(?:\s+(?:[IVX]+|\d{1,2}))?\.?$`)

// FromBody derives a volume's contents from the pages themselves.
//
// The chapters and their spans come from the page map, which is the only thing
// that knows them for a volume with no contents page, and the § and no. come
// from the headings on the pages inside each span.
func FromBody(pages []BodyPage, pm *pagemap.Map, opt Options) *Result {
	res := &Result{Book: opt.Book, Grammar: Grammar{Mark: "body", Page: Bare}}
	byPDF := map[int]BodyPage{}
	for _, p := range pages {
		byPDF[p.PDFPage] = p
	}
	for _, span := range pm.Chapters {
		ch, probs := bodyChapter(span, byPDF, pm, opt)
		if ch == nil {
			continue
		}
		res.Chapters = append(res.Chapters, *ch)
		res.Problems = append(res.Problems, probs...)
	}
	if len(res.Chapters) == 0 {
		res.Problems = append(res.Problems, Problem{
			Detail: "the pages yielded no chapter with any heading in it"})
		return res
	}
	if len(opt.Chapters) > 0 && len(res.Chapters) != len(opt.Chapters) {
		res.Problems = append(res.Problems, Problem{Detail: fmt.Sprintf(
			"the pages hold %d chapters, the manifest names %d",
			len(res.Chapters), len(opt.Chapters))})
	}
	return res
}

// builder carries the one chapter being read and the § it is inside.
type builder struct {
	chapter  *corpus.Chapter
	section  *corpus.Section
	pm       *pagemap.Map
	numeral  string
	probs    []Problem
	pages    []BodyPage // the chapter's pages, in pdf order
	at       int        // the one being read
	openPDF  int        // the chapter's first pdf page
	openPage int        // and its printed page
	// inExercises latches once the chapter reaches the run of exercises the
	// press gathers at its end. Nothing after that point is a § or a no.
	inExercises bool
}

func bodyChapter(span pagemap.Span, byPDF map[int]BodyPage, pm *pagemap.Map,
	opt Options) (*corpus.Chapter, []Problem) {
	b := &builder{
		pm:       pm,
		numeral:  span.Chapter,
		openPDF:  span.FirstPDF,
		openPage: span.FirstPage,
		chapter: &corpus.Chapter{
			Numeral: span.Chapter,
			Page:    span.FirstPage,
			PDFPage: span.FirstPDF,
		},
	}
	for pdf := span.FirstPDF; pdf <= span.LastPDF; pdf++ {
		if page, ok := byPDF[pdf]; ok {
			b.pages = append(b.pages, page)
		}
	}
	if len(b.pages) == 0 {
		return nil, nil
	}
	b.chapter.Title = bodyChapterTitle(b.pages[0], opt)
	for i, page := range b.pages {
		b.at = i
		b.page(page, printedPage(pm, page.PDFPage))
	}
	if len(b.chapter.Sections) == 0 {
		return nil, nil
	}
	b.missingAppendices()
	return b.chapter, b.probs
}

// missingAppendices reports an appendix the running heads name and the pages
// never open.
//
// Chapter VII of Lie is the case. Page 46 heads the verso "Ch. VII, App. 1" and
// page 48 heads it "Ch. VII, App. II", so the volume prints two appendices and
// the second opens on page 47. The reading of page 47 came back with the body
// starting in the middle of a sentence and no heading on it at all, and the
// text layer for that page is empty, so there is nothing on either side to open
// the appendix with. That is a page to read again and not a thing to guess at,
// so it is said rather than filled in.
func (b *builder) missingAppendices() {
	have := 0
	named := map[string]bool{}
	var order []string
	add := func(key string) {
		if !named[key] {
			named[key] = true
			order = append(order, key)
		}
	}
	// The appendices the pages did open are counted in, and not only the heads.
	// fix opening takes the running head off the page it has moved the word down
	// into, so on a repaired volume the head that named the first appendix is
	// gone and the heads alone name one fewer than the volume prints.
	for _, s := range b.chapter.Sections {
		if s.Appendix {
			have++
			add(roman(s.Number))
		}
	}
	for _, page := range b.pages {
		m := headAppendix.FindStringSubmatch(page.RunningHead)
		if m == nil {
			continue
		}
		// The heads write the same numeral two ways, "App. 1" on one page and
		// "App. II" on the next, because a scan of this age reads a roman one
		// as an arabic one about as often as not. They are the same appendix.
		add(strings.ToUpper(strings.ReplaceAll(m[1], "1", "I")))
	}
	if len(order) <= have {
		return
	}
	b.short(0, fmt.Sprintf("the pages name %d appendices, %s, and open %d, so a "+
		"heading is missing from the reading",
		len(order), strings.Join(order, " and "), have))
}

// sectionTitle is what to call a § whose own heading the reading did not keep.
//
// The running head of a recto in the head-label printings is the § title with
// "N° 7 " in front of it, and it is on every recto of the §, so the § can be
// named even where its heading page is not in the file at all. The verso head
// is the chapter title and names nothing, which is why the search is for a head
// that carries the no. marker rather than for the nearest head of any kind:
// asking the nearest gave § 1 of chapter X of Algebre commutative the title
// "PROFONDEUR, RÉGULARITÉ, DUALITÉ", which is the chapter.
//
// The search stops at the next § so that a § whose rectos all came back without
// a head is left with an empty title rather than borrowed the next §'s. An
// empty title is visible and a wrong one is not.
func (b *builder) sectionTitle() string {
	for i := b.at; i < len(b.pages); i++ {
		head := b.pages[i].RunningHead
		if headNumber.MatchString(head) {
			return headTitle(head)
		}
		if i > b.at && b.opensAnySection(b.pages[i]) {
			break
		}
	}
	return ""
}

// sectionOnePage is where to open a § 1 whose own heading the reading did not
// keep, as a pdf page and the printed page on it.
//
// The chapter's own first page is the answer nearly always, since a § 1 headed
// on the same page as the chapter is what these printings do and the reading
// lost the heading off it. Chapter X of Algebre commutative is the exception.
// It sets the chapter title and the conventions paragraph on printed page 1 and
// heads § 1 over on printed page 2, and printed page 2 is a leaf the scan does
// not have. Opening § 1 at page 1 there puts the chapter's own front matter
// inside the §, which is not where the book puts it, and the assembler then has
// nothing left to call front matter.
//
// The page map is what says so, and nothing else is guessed from. It records
// the step at pdf page 2 with printed page 2 missing, so the leaf between the
// chapter's first page and its second is known to be absent rather than
// suspected. Where the map records no such step the chapter's first page is
// returned and this changes nothing.
func (b *builder) sectionOnePage() (pdf, printed int) {
	if len(b.pages) > 1 && b.pages[0].PDFPage == b.openPDF {
		if next := b.pages[1].PDFPage; b.pm.AbsentBefore(next) {
			b.short(1, fmt.Sprintf("§ 1 is headed on a leaf the scan does not "+
				"have, so the contents opens it at pdf page %d, the first page "+
				"of it the file carries", next))
			return next, printedPage(b.pm, next)
		}
	}
	return b.openPDF, b.openPage
}

// opensAnySection says the page carries a § heading of its own.
func (b *builder) opensAnySection(page BodyPage) bool {
	for raw := range strings.SplitSeq(page.Body, "\n") {
		if bodySection.MatchString(plainHeading(raw)) {
			return true
		}
	}
	return false
}

// bodyChapterTitle is what the chapter's first page calls it.
//
// A Bourbaki chapter sets its title on its own first page under the chapter
// numeral, as the first line of the body, and the reading keeps it there: page
// 1 of chapter X of Algebre commutative reads "Profondeur, régularité, dualité"
// and nothing else before the conventions paragraph. Chapter VII of Lie sets it
// over two lines, "SOUS-ALGÈBRES DE CARTAN" and "ÉLÉMENTS RÉGULIERS", and both
// are taken, because a title the press broke across two lines is one title.
//
// The volume title is the fallback and not the first choice. For a fragment of
// one chapter the two are usually the same thing, but they are the same thing
// by accident, and a chapter that prints its own title should be called what it
// prints.
//
// The line naming the chapter is skipped. On a page as the reading left it there
// is no such line, because the press sets the numeral in the largest type on the
// page and the reading loses it, which is what fix opening exists to put back.
// Once it has been put back the page opens "## CHAPITRE VII" and the title is
// the line under it, and this has to give the same answer on the page either
// way or a second run over a repaired volume renames every chapter in it.
func bodyChapterTitle(page BodyPage, opt Options) string {
	var lines []string
	for raw := range strings.SplitSeq(page.Body, "\n") {
		line := plainHeading(raw)
		if line == "" {
			if len(lines) > 0 {
				break
			}
			continue
		}
		if len(lines) == 0 && bodyChapterLine.MatchString(line) {
			continue
		}
		// A title is short and carries no full stop in the middle of it. The
		// conventions paragraph that follows it is neither, and it is what
		// stops the walk when the page prints no title at all.
		if len(line) > 80 || strings.Contains(line, ". ") {
			break
		}
		lines = append(lines, line)
	}
	if len(lines) == 0 {
		return opt.Title
	}
	return strings.Join(lines, " ")
}

// page reads one page of the chapter.
func (b *builder) page(page BodyPage, printed int) {
	// An exercise page carries no heading this reads. It carries the § its
	// exercises belong to, in the locator the reading files off the running
	// head, and that is what the contents would have said.
	//
	// The run is latched rather than tested page by page. Both of these
	// printings gather the exercises at the end of the chapter under one
	// "Exercices" line and then head each page with the § it is exercising, but
	// the reading kept that head on only nine of the thirty one exercise pages
	// of chapter X of Algebre commutative. Testing each page in isolation
	// therefore let twenty two pages of exercises back into the body, where the
	// numbered exercises on them read as no. headings: "3) Soit A un anneau
	// local de Macaulay" is not one, but "1) Soient A un anneau, J un ideal"
	// arrives in the right order to be taken for one.
	if !b.inExercises && b.opensExercises(page) {
		b.inExercises = true
	}
	if b.inExercises {
		b.exercises(page, printed)
		return
	}
	// The page that opens an appendix is read on past the opening, because an
	// appendix carries no. of its own and sets the first of them under its
	// title on the same page. The appendix to chapter VII of Lie has two, and
	// the English printing of that chapter lists both.
	if n, title, ok := b.appendixOpening(page); ok {
		b.open(corpus.Section{Number: n, Title: title, Page: printed,
			PDFPage: page.PDFPage, Appendix: true})
	}
	for raw := range strings.SplitSeq(page.Body, "\n") {
		line := plainHeading(raw)
		// A contents line that leaked into the body carries the leaders the
		// press sets between the title and the page, and it is not a heading.
		if line == "" || len(line) > 120 || strings.Contains(line, "....") {
			continue
		}
		if m := bodySection.FindStringSubmatch(line); m != nil && b.opensSection(m[1]) {
			n, _ := strconv.Atoi(m[1])
			b.open(corpus.Section{Number: n, Title: strings.TrimSpace(m[2]),
				Page: printed, PDFPage: page.PDFPage})
			continue
		}
		m := bodyNumber.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		n, _ := strconv.Atoi(m[1])
		b.number(n, strings.TrimSpace(m[2]), page, printed)
	}
}

// opensSection says whether a "§ n." line is the next § of this chapter.
//
// A § heading is also a citation, and these volumes cite each other constantly:
// "(§ 2, prop. 4)" and "(A, IV, § 5, n° 10)" are on nearly every page. What
// tells the heading from the citation is that the heading is the whole line and
// its number is the one that comes next, and both are required here.
func (b *builder) opensSection(digits string) bool {
	n, err := strconv.Atoi(digits)
	if err != nil {
		return false
	}
	if b.section == nil {
		return n == 1
	}
	return n == b.section.Number+1 && !b.section.Appendix
}

// number takes a no. heading, opening a § first where the page carries no §
// heading of its own.
func (b *builder) number(n int, title string, page BodyPage, printed int) {
	switch {
	case b.section == nil:
		// The chapter's first § opens with no heading this reading kept. It
		// begins where the chapter begins, and the running head of a recto in
		// these printings is the § title, so that is what it is called.
		//
		// Chapter X of Algebre commutative is the case and it is worth stating
		// plainly. Its § 1 is headed on printed page 2, printed page 2 is a
		// leaf the scan does not have, and the page map records it as missing.
		// So the heading is not on any page of the file and no reading of the
		// file will ever produce it. What the file does have is page 3, whose
		// running head is "N° 1 PROFONDEUR", which names both the § and the no.
		pdf, at := b.sectionOnePage()
		b.open(corpus.Section{Number: 1, Title: b.sectionTitle(),
			Page: at, PDFPage: pdf})
		if n != 1 {
			b.short(1, fmt.Sprintf("§ 1 opens at no. %d, so the heading of "+
				"no. 1 is on a page the scan does not have", n))
		}
	case b.section.Appendix:
		// An appendix is the last thing in a chapter and no § follows it, so
		// the rule below does not apply inside one. A second no. 1 there is a
		// second appendix, and the number of an appendix is not a thing the
		// body counts, so it is left to missingAppendices to say so.
		if want := b.wantNumber(); n != want {
			return
		}
	case n == 1 && len(b.section.Subsections) > 0:
		// A no. numbered 1 inside a § that already has one is the next § of the
		// chapter, opening on a page whose § heading the reading did not keep.
		b.open(corpus.Section{Number: b.section.Number + 1,
			Title: b.sectionTitle(), Page: printed,
			PDFPage: page.PDFPage})
	default:
		if want := b.wantNumber(); n != want {
			return
		}
	}
	b.section.Subsections = append(b.section.Subsections, corpus.Subsection{
		Number: n, Title: title, Page: printed, PDFPage: page.PDFPage})
}

// wantNumber is the no. that comes next in the § being read.
func (b *builder) wantNumber() int {
	if len(b.section.Subsections) == 0 {
		return 1
	}
	return b.section.Subsections[len(b.section.Subsections)-1].Number + 1
}

// open starts a new § or appendix and makes it the one being read.
func (b *builder) open(s corpus.Section) {
	b.chapter.Sections = append(b.chapter.Sections, s)
	b.section = &b.chapter.Sections[len(b.chapter.Sections)-1]
}

// appendixOpening says whether this page opens an appendix, and what it is
// called.
//
// An appendix is not a § and does not carry a number the body counts, so the
// ordinal test that finds a § cannot find one. What names it is the running
// head, which reads "APPENDICE I" on the recto it opens on and "Ch. VII,
// App. 1" on the versos after it. So the head is what is asked, and only on a
// page whose body opens with a title rather than with prose, which is what
// tells the first page of the appendix from the rest of it.
//
// The appendices of a chapter are numbered from 1 over again and not on from
// the last §. That is what the rest of the corpus does: chapter VII in the
// English printing of the same book lists five §§ and then an appendix 1 and an
// appendix 2, and the assembler looks for "APPENDICE 1" or "APPENDICE I" on the
// page, which is what the French prints over it.
func (b *builder) appendixOpening(page BodyPage) (int, string, bool) {
	if b.section == nil {
		return 0, "", false
	}
	title, ok := appendixTitle(page)
	if !ok || (b.section.Appendix && b.section.Title == title) {
		return 0, "", false
	}
	n := 0
	for _, s := range b.chapter.Sections {
		if s.Appendix {
			n++
		}
	}
	return n + 1, title, true
}

// appendixTitle is what the appendix this page opens is called, in the two
// shapes a page of one comes in.
//
// As the reading left it the page carries the word in its running head and opens
// its body with the title. fix opening then rewrites that into the other shape,
// moving the word into the body as a heading of its own and taking the running
// head off the page, since the word is the whole of what the page prints above
// the appendix. Both are read, because a volume is read again after it has been
// repaired and the second reading has to say what the first one said. Reading
// only the first shape turned the appendix to chapter VII of Lie back into a
// § 6 with no title the moment the repair had run.
func appendixTitle(page BodyPage) (string, bool) {
	var lead []string
	for raw := range strings.SplitSeq(page.Body, "\n") {
		if line := plainHeading(raw); line != "" {
			lead = append(lead, line)
			if len(lead) == 2 {
				break
			}
		}
	}
	if len(lead) > 0 && bodyAppendixLine.MatchString(lead[0]) {
		if len(lead) < 2 || !titleShaped(lead[1]) {
			return "", false
		}
		return lead[1], true
	}
	if headAppendix.FindStringSubmatch(page.RunningHead) == nil {
		return "", false
	}
	title := bodyChapterTitle(BodyPage{Body: page.Body}, Options{})
	if !titleShaped(title) {
		return "", false
	}
	return title, true
}

// titleShaped says the line could be a title and not the opening of a paragraph.
//
// The head names an appendix on every page of it, so a page whose first line is
// prose is inside one already and does not open it. A title never runs to the end
// of a paragraph and never carries a full stop or a comma in the middle of it,
// and the appendix's own first line under its title is its conventions sentence,
// which does.
func titleShaped(s string) bool {
	return s != "" && len([]rune(s)) <= 80 &&
		!strings.HasSuffix(s, ".") && !strings.Contains(s, ", ")
}

// exercises files the runs of exercises that begin on this page.
//
// A chapter that gathers its exercises at the end of itself separates the runs
// with the sign and the number on a line of their own, "§ 1", and that line is
// what the assembler cuts the block on. So that line is what the contents has to
// point at, and not the first page of the run that happens to carry a running
// head. Page 51 of chapter VII of Lie is the third page of § 1's run and it is
// the first one the reading kept a head on; a contents that opened the run there
// cut off the eight exercises before it, and assemble said so.
//
// The locator stands in where the page carries no mark, which is what a page in
// the middle of a run looks like. It only ever files a § that has no run yet, so
// on a chapter whose marks all survived it never fires, and where a mark is lost
// it puts the run somewhere inside itself rather than nowhere.
func (b *builder) exercises(page BodyPage, printed int) {
	var marked []int
	for raw := range strings.SplitSeq(page.Body, "\n") {
		if m := bodyRunMark.FindStringSubmatch(plainHeading(raw)); m != nil {
			n, _ := strconv.Atoi(m[1])
			marked = append(marked, n)
		}
	}
	if len(marked) == 0 {
		if page.Section == 0 {
			return
		}
		marked = []int{page.Section}
	}
	for _, n := range marked {
		for i := range b.chapter.Sections {
			s := &b.chapter.Sections[i]
			if s.Number != n || s.Appendix || s.Exercises != nil {
				continue
			}
			s.Exercises = &corpus.Locator{Page: printed, PDFPage: page.PDFPage}
			break
		}
	}
}

// short records that the pages are missing a heading the volume prints.
//
// It is soft for the reason the missing leaf in validate is soft. The reading
// is not wrong and the manifest it produces is not wrong either, it is short,
// and refusing to write a contents that is right about nine §§ because the
// scan lost the tenth's heading would leave the whole volume unreadable.
func (b *builder) short(section int, detail string) {
	b.probs = append(b.probs, Problem{Chapter: b.numeral, Section: section,
		Soft: true, Detail: detail})
}

// headTitle is a running head used as a § title.
func headTitle(head string) string {
	s := headNumber.ReplaceAllString(head, "")
	s = headAppendix.ReplaceAllString(s, "")
	return strings.TrimSpace(strings.Trim(strings.TrimSpace(s), ",."))
}

// opensExercises says the chapter's run of exercises begins on this page.
//
// The press sets one "Exercices" line at the head of the run and then heads
// every page of it with the § being exercised, so either says the run has
// begun. The first is looked for as the page's own first line, because the word
// appears inside the exercises themselves as a cross reference and once, on
// page 49 of chapter VII of Lie, in the sentence that opens the run.
func (b *builder) opensExercises(page BodyPage) bool {
	if strings.HasPrefix(strings.ToUpper(strings.TrimSpace(page.RunningHead)), "EXERC") {
		return true
	}
	for raw := range strings.SplitSeq(page.Body, "\n") {
		line := plainHeading(raw)
		if line == "" {
			continue
		}
		return strings.EqualFold(line, "exercices") || strings.EqualFold(line, "exercises")
	}
	return false
}

// plainHeading is a body line with the emphasis a reading wrapped it in taken
// off.
//
// Some pages came back with the heading set as bold, "**3. Applications de la
// conjugaison**", because that is the weight the press sets it in and the
// reading kept it. Two no. of chapter VII of Lie are written that way and
// nothing else about them differs from the rest, so the markers come off here
// rather than a second pattern being written for them.
func plainHeading(raw string) string {
	s := strings.TrimSpace(strings.Trim(strings.TrimSpace(raw), "#"))
	switch {
	case strings.HasPrefix(s, "**") && strings.HasSuffix(s, "**") && len(s) > 4:
		s = s[2 : len(s)-2]
	case strings.HasPrefix(s, "*") && strings.HasSuffix(s, "*") && len(s) > 2:
		s = s[1 : len(s)-1]
	}
	return strings.TrimSpace(s)
}

// printedPage is what the volume prints on a pdf page, and 0 where the map has
// nothing for it.
func printedPage(pm *pagemap.Map, pdf int) int {
	if e, ok := pm.Lookup(pdf); ok {
		return e.Page
	}
	return 0
}
