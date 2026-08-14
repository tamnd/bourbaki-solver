package refs

import (
	"fmt"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// How says what settled a reference, and is written on the edge so that a
// reader of the manifest can tell a lookup that had to guess from one that did
// not.
const (
	// ByPage is the ordinary case: the page named the §, and the § had exactly
	// one statement of that kind and number.
	ByPage = "page-map"
	// ByPageAndNo is the same with the no. brought in, which is what a
	// Corollary, a Remark and an Example need, since those three are not
	// numbered straight through the §.
	ByPageAndNo = "page-map+no"
	// ByStatementPage is the page the statement itself is printed on, read back
	// out of pages/. It is what settles the ones the no. cannot: two statements
	// of one number in one no., or a page that starts a no. and carries the tail
	// of the one before it.
	ByStatementPage = "statement-page"
	// BySection is a reference to a page and nothing on it. It resolves to a §
	// and to no statement, because there is no statement in it to resolve to.
	BySection = "section"
	// ByNumbering is a citation of the other printing, settled by what it says
	// outright. "Chap. VII, §2, no. 3, Prop. 10" names the § and the no., so
	// there is no page to map and nothing to work backwards from, and where the §
	// holds more than one Prop. 10 the no. it also names is what decides.
	ByNumbering = "section-no"
	// ByContext is a statement named in running prose with nothing else, which
	// means the § the sentence is in.
	ByContext = "in-section"
	// ByParent is a corollary named by the statement it hangs from. It is the
	// only lookup here that is not a search: the label is built and either it is
	// in the corpus or it is not.
	ByParent = "parent-label"
	// ByNearest is a bare reference in running prose that the § holds more than
	// one of, settled by taking the one printed nearest above the sentence.
	ByNearest = "nearest-above"
	// ByExercise is Exercise n of the § the page falls in.
	ByExercise = "exercise"
	// OutOfCorpus is a Book, or a chapter of Algebra, that this corpus does not
	// hold. Not an error, and the count of them is the ingestion roadmap.
	OutOfCorpus = "out-of-corpus"
)

// Target is what a citation was found to point at.
type Target struct {
	Label string
	Tag   string
	How   string
	Book  string // the Book code, when the reference leaves the corpus
}

// Chapter is the chapter of Algebra this corpus holds, and is what a reference
// that names no chapter of its own is read against.
//
// A reference to any other chapter of the same Book is a reference out of the
// corpus, and is worth keeping in the report: it is the closest thing to ingest
// next.
const Chapter = "VIII"

// corpusBooks is the Book code as the Éléments write it against the Book as a
// label writes it, for the Books this corpus has any of.
//
// A citation names its Book the way the book prints it, "Algebra" or "Lie", and
// a label names it the way a path does. Which of them the corpus actually holds
// is not settled here but asked of the index, so that ingesting a volume is
// enough to make its references resolve and nothing here has to be remembered.
var corpusBooks = map[string]string{
	"A":   "alg",
	"LIE": "lie",
	"E":   "ens",
	"TG":  "top",
	"TS":  "ts",
}

// Site is where the sentence making a reference stands. The § is what a bare
// "Proposition 4" means. The file and the line are what settles a bare
// "Corollary 1" in a § that prints several, since the one meant is the one
// printed nearest above.
type Site struct {
	Section string
	File    string
	Line    int
}

// bookOf is the Book a citation is to be looked up in, and says instead that the
// reference leaves the corpus when that Book is one the corpus has none of.
//
// A sentence that names no Book means its own, which until Lie 7 to 9 was
// assembled was always Algebra and is now whatever Book the citing § belongs to.
// The Book that is named is tested on its code and not on its name, since a Book
// has more than one spelling and Algebra has two, Algebra and Alg. Comparing the
// name would take every reference a chapter makes to itself under the second of
// them out of the corpus.
func (ix *Index) bookOf(c Citation, at Site) (book string, out Target, leaves bool) {
	if c.Book == "" {
		if s := ix.Section(at.Section); s != nil {
			return s.Book, Target{}, false
		}
		return "alg", Target{}, false
	}
	code := Code(c.Book)
	book, ok := corpusBooks[code]
	if code == "" || !ok || !ix.Holds(book) {
		return "", Target{How: OutOfCorpus, Book: code}, true
	}
	return book, Target{}, false
}

// holdsChapter reports whether the corpus holds this chapter of this Book.
func (ix *Index) holdsChapter(book, chapter string) bool {
	for i := range ix.Sections {
		if ix.Sections[i].Book == book && ix.Sections[i].Chapter == chapter {
			return true
		}
	}
	return false
}

// codeFor is the letter the Éléments give a Book, from the name a label uses.
func codeFor(book string) string {
	for code, b := range corpusBooks {
		if b == book {
			return code
		}
	}
	return ""
}

// Resolve looks a citation up.
//
// A member of a list whose locator was written on the end has a second reading,
// and it is tried only when the reference as it stands answers nowhere. See
// under, in the parser, for why it is second and not first. When both readings
// fail it is the first that is reported, since that is the reference the page
// prints and the one a reader would go looking for.
func (ix *Index) Resolve(c Citation, at Site) (Target, error) {
	out, err := ix.resolve(c, at)
	if err == nil || c.Under == nil {
		return out, err
	}
	if alt, err := ix.resolve(*c.Under, at); err == nil {
		return alt, nil
	}
	return out, err
}

func (ix *Index) resolve(c Citation, at Site) (Target, error) {
	book, out, leaves := ix.bookOf(c, at)
	if leaves {
		return out, nil
	}
	from := at.Section
	switch c.Form {
	case FormFormula:
		return Target{}, fmt.Errorf("a numbered display carries no tag of its own")
	case FormLocCit:
		return Target{}, fmt.Errorf("loc. cit. points at the last work cited, which this reads no further back to find")
	case FormLocal:
		// An exercise is not in the statement index, so a bare "Exercise 27" has
		// to go the same way a page-based one does. Nearly all of these are one
		// exercise of a § pointing at another of the same §, which is how the
		// book cross-refers within a run of exercises, and there are 166 of them.
		if c.Kind == corpus.KindExercise {
			return ix.exercise(ix.Section(from), c.Number)
		}
		// The no. is passed on where the sentence wrote one. A bare statement has
		// none and is settled by what stands nearest above it; one written "no. 2,
		// Remark 1" has said which no. it means, and that is better than nearest
		// and is the only thing that can settle a kind the § numbers afresh under
		// every no.
		return ix.inSection(c, from, ByContext, c.Subsec, at)
	case FormAttached:
		return ix.attachedIn(c, book, from)
	case FormSection:
		s, out, leaves := ix.sectionCited(c, book, from)
		if leaves {
			return out, nil
		}
		if s == nil {
			return Target{}, fmt.Errorf("chapter %s of %s has no %s", ix.chapterOf(c, from), book, sectionSaid(c))
		}
		if c.Kind == "" {
			return Target{Label: s.Label, How: BySection}, nil
		}
		if c.Kind == corpus.KindExercise {
			return ix.exercise(s, c.Number)
		}
		return ix.inSection(c, s.Label, ByNumbering, c.Subsec, at)
	}
	if !ix.holdsChapter(book, c.Chapter) {
		return Target{How: OutOfCorpus, Book: codeFor(book)}, nil
	}
	on := ix.SectionAt(book, c.Chapter, c.Page)
	switch len(on) {
	case 0:
		return Target{}, fmt.Errorf("no § of chapter %s is printed on page %d", c.Chapter, c.Page)
	case 1:
	default:
		return Target{}, fmt.Errorf("page %d is in %d sections at once", c.Page, len(on))
	}
	s := on[0]
	if c.Kind == "" {
		return Target{Label: s.Label, How: BySection}, nil
	}
	if c.Kind == corpus.KindExercise {
		return ix.exercise(s, c.Number)
	}
	no := c.Subsec
	if no == 0 {
		no = s.SubsecAt(c.Page)
	}
	return ix.inSection(c, s.Label, ByPage, no, at)
}

// attachedIn decides which § a corollary named by its parent is to be looked up
// in, which until this was written was always the § the sentence stood in.
//
// That is right for "by the corollary of Proposition 2" and wrong for every
// attached citation that says where the parent is printed, which is most of
// them: 20 of the chapter's 37 unresolved references were this one fault. A
// corollary of a Proposition 12 that belongs to Book II was hunted for in
// chapter VIII, where the hunt could only fail, and one whose parent is printed
// on VIII, p. 83 was hunted for in the § doing the citing rather than the § the
// page falls in.
//
// The locator governs whenever the sentence gives one, and the § in hand is the
// fallback rather than the rule.
func (ix *Index) attachedIn(c Citation, book, from string) (Target, error) {
	// A locator with a § and no page is one of the other printing, which says
	// which § the parent stands in rather than which page it is printed on.
	if c.Page == 0 && c.Section != 0 {
		s, out, leaves := ix.sectionCited(c, book, from)
		if leaves {
			return out, nil
		}
		if s == nil {
			return Target{}, fmt.Errorf("chapter %s of %s has no %s", ix.chapterOf(c, from), book, sectionSaid(c))
		}
		return ix.attached(c, s.Label)
	}
	if c.Chapter == "" {
		return ix.attached(c, from)
	}
	if !ix.holdsChapter(book, c.Chapter) {
		return Target{How: OutOfCorpus, Book: codeFor(book)}, nil
	}
	on := ix.SectionAt(book, c.Chapter, c.Page)
	switch len(on) {
	case 0:
		return Target{}, fmt.Errorf("no § of chapter %s is printed on page %d", c.Chapter, c.Page)
	case 1:
	default:
		return Target{}, fmt.Errorf("page %d is in %d sections at once", c.Page, len(on))
	}
	return ix.attached(c, on[0].Label)
}

// sectionCited is the § a citation of the other printing names, and says instead
// that the reference leaves the corpus when the chapter is one the corpus has
// none of.
//
// A citation that names no chapter means the chapter it is standing in, which is
// what "§ 2, no. 3, Prop. 10" says: a volume of three chapters refers to its own
// as often as to the other two. This is why the § the sentence is in has to be
// known, and a sentence in no § can only be left alone.
func (ix *Index) sectionCited(c Citation, book, from string) (*Section, Target, bool) {
	chapter := ix.chapterOf(c, from)
	if chapter == "" {
		return nil, Target{}, false
	}
	if !ix.holdsChapter(book, chapter) {
		return nil, Target{How: OutOfCorpus, Book: codeFor(book)}, true
	}
	return ix.SectionIn(book, chapter, c.Section, c.Appendix), Target{}, false
}

// chapterOf is the chapter a citation names, or the one the sentence stands in
// when it names none.
func (ix *Index) chapterOf(c Citation, from string) string {
	if c.Chapter != "" {
		return c.Chapter
	}
	if s := ix.Section(from); s != nil {
		return s.Chapter
	}
	return ""
}

// sectionSaid writes back the § a citation named, for a message about one the
// chapter does not have.
func sectionSaid(c Citation) string {
	if c.Appendix {
		return fmt.Sprintf("Appendix %d", c.Section)
	}
	return fmt.Sprintf("§ %d", c.Section)
}

// attached finds a corollary from the statement it hangs from.
//
// A numbered corollary carries its parent in its own label, so the sentence
// holds the answer and there is nothing to search: "Corollary 1 of Proposition
// 4" in § 20 is alg-viii-s20-prop-4-cor-1 and no other statement. That is worth
// having, because a bare Corollary 1 is ambiguous in eighteen of the twenty-two
// sections and would otherwise resolve to nothing.
//
// An unnumbered one is named by its no. instead, deliberately, so its label
// says nothing about its parent and it has to be found by where it stands: the
// corollaries printed under Proposition 3 are the ones with no number of their
// own that Proposition 3 is the last numbered statement above. Twenty-five of
// the chapter's thirty-five attached citations are of this kind.
func (ix *Index) attached(c Citation, section string) (Target, error) {
	s := ix.Section(section)
	if s == nil {
		return Target{}, fmt.Errorf("the sentence is in no §")
	}
	if c.Number > 0 {
		label := fmt.Sprintf("%s-%s-%d-cor-%d", section, c.ParentKind, c.ParentNumber, c.Number)
		if st := ix.Statement(label); st != nil {
			return Target{Label: st.Label, Tag: st.Tag, How: ByParent}, nil
		}
	}
	var under []*Statement
	for _, st := range s.Statements {
		if st.Kind == corpus.KindCorollary && !st.Named &&
			st.FollowsKind == c.ParentKind && st.FollowsNumber == c.ParentNumber {
			under = append(under, st)
		}
	}
	n := max(c.Number, 1)
	if len(under) < n {
		// A reference that gave no number to a parent whose corollaries are all
		// numbered is not a corollary the corpus is missing, it is a reference
		// that did not say enough, and saying so is the difference between a
		// finding a reader can act on and one that sends them looking for a
		// statement that is printed.
		if c.Number == 0 {
			if k := ix.numbered(section, c); k > 0 {
				return Target{}, fmt.Errorf("%s prints %d numbered corollaries of %s %d and the reference does not say which",
					section, k, c.ParentKind.Heading(), c.ParentNumber)
			}
		}
		if t, ok := ix.overProof(s, c, n); ok {
			return t, nil
		}
		return Target{}, fmt.Errorf("%s has no corollary %d of %s %d",
			section, n, c.ParentKind.Heading(), c.ParentNumber)
	}
	return Target{Label: under[n-1].Label, Tag: under[n-1].Tag, How: ByParent}, nil
}

// numbered counts the corollaries the printing hangs from the statement a
// reference names and gives a number of their own. They are counted by asking
// for their labels, since a numbered corollary carries its parent in its own
// label and the count runs from one without a gap.
func (ix *Index) numbered(section string, c Citation) int {
	n := 0
	for ix.Statement(fmt.Sprintf("%s-%s-%d-cor-%d", section, c.ParentKind, c.ParentNumber, n+1)) != nil {
		n++
	}
	return n
}

// overProof finds a corollary the book hangs from a result over the lemma that
// was proved on the way to it.
//
// A statement is given the last numbered statement above it as its parent, which
// is what the printing shows and is right nearly everywhere. It is not right
// where the proof of a theorem borrows a lemma: § 7 of chapter IX states Theorem
// 3, says "let us accept provisionally the following lemma", states and proves
// Lemma 5, finishes the proof of the theorem and then prints COROLLARY 1 and
// COROLLARY 2. Nothing on the page says those hang from the theorem rather than
// from the lemma, but the volume says it itself, twice, in the words "§7, no. 5,
// Cor. 1 of Th. 3".
//
// The walk stops at the next statement of the kind that was cited, and that is
// what keeps it from papering over a disagreement. A reference to a corollary of
// Theorem 1 that is only found past Theorem 2 is not a theorem cited over its
// proof, it is the reference and the printing naming different theorems, and § 10
// of chapter VIII is exactly that. It is left unresolved and reported, which is
// what it is.
func (ix *Index) overProof(s *Section, c Citation, n int) (Target, bool) {
	var from *Statement
	for _, st := range s.Statements {
		if st.Kind != c.ParentKind || st.Number != c.ParentNumber {
			continue
		}
		if from != nil {
			return Target{}, false
		}
		from = st
	}
	if from == nil {
		return Target{}, false
	}
	for _, st := range s.Statements {
		if st.Path != from.Path || st.Line <= from.Line {
			continue
		}
		if st.Kind == c.ParentKind {
			break
		}
		if st.Kind == corpus.KindCorollary && st.Named && st.Number == n {
			return Target{Label: st.Label, Tag: st.Tag, How: ByParent}, true
		}
	}
	return Target{}, false
}

// exercise is Exercise n of a §. There is nothing to search: the exercises of a
// § are numbered straight through, so the count is enough to say whether the
// one asked for exists and the label follows from the number.
func (ix *Index) exercise(s *Section, n int) (Target, error) {
	if s == nil {
		return Target{}, fmt.Errorf("the sentence is in no §")
	}
	if n < 1 || n > s.Exercises {
		return Target{}, fmt.Errorf("%s has %d exercises and none numbered %d", s.Label, s.Exercises, n)
	}
	label := fmt.Sprintf("%s-ex-%d", s.Label, n)
	return Target{Label: label, Tag: ix.Tag(label), How: ByExercise}, nil
}

// inSection finds the statement of a kind and number in one §, bringing the
// page and then the no. in only when the § holds more than one candidate.
// Nothing is narrowed that does not need narrowing, so a wrong no. cannot turn
// a lookup that would have worked into one that does not.
func (ix *Index) inSection(c Citation, section, how string, no int, at Site) (Target, error) {
	if section == "" {
		return Target{}, fmt.Errorf("the sentence is in no §")
	}
	cand := ix.Statements(section, c.Kind, c.Number)
	switch len(cand) {
	case 0:
		return Target{}, fmt.Errorf("%s has no %s %d", section, c.Kind.Heading(), c.Number)
	case 1:
		return Target{Label: cand[0].Label, Tag: cand[0].Tag, How: how}, nil
	}
	if t, ok := onPage(cand, c.Page); ok {
		return t, nil
	}
	if no == 0 {
		// Only for a reference that carries no page. One that does has said
		// something about where to look, and taking the nearest instead would be
		// throwing that away: "Corollary 1 of VIII, p. 215" is the corollary on
		// page 215 whatever stands nearest, and the one case in chapter VIII
		// where the two rules disagree is exactly that shape. A page that does
		// not narrow to one statement is left unresolved and reported.
		if how == ByContext {
			if t, ok := nearest(cand, at); ok {
				return t, nil
			}
		}
		return Target{}, fmt.Errorf("%s has %d statements called %s %d and nothing says which",
			section, len(cand), c.Kind.Heading(), c.Number)
	}
	var hit []*Statement
	for _, st := range cand {
		if st.Subsec == no {
			hit = append(hit, st)
		}
	}
	if len(hit) != 1 {
		return Target{}, fmt.Errorf("%s has %d statements called %s %d and %d of them in no. %d",
			section, len(cand), c.Kind.Heading(), c.Number, len(hit), no)
	}
	return Target{Label: hit[0].Label, Tag: hit[0].Tag, How: withNo(how)}, nil
}

// withNo is what an edge records when the no. is what settled it. A page
// citation that came down to its no. is a different lookup from one the page
// settled by itself, and is worth telling apart. A citation that named its § and
// its no. and nothing else was that lookup from the start and has nothing to add.
func withNo(how string) string {
	if how == ByPage {
		return ByPageAndNo
	}
	return how
}

// onPage takes the candidate the book prints on the page the citation names.
//
// This is what a reader does, and it is more nearly what the citation says than
// the no. is: "Corollary 1 of VIII, p. 215" is the Corollary 1 printed on page
// 215, and whether page 215 belongs to no. 2 or to no. 3 is a question the
// sentence never asked. It settles the two shapes the no. cannot: a page that
// starts a no. and carries the tail of the one before it, and two statements of
// one number standing in one no.
//
// Every candidate has to have a known page for this to fire. A statement that
// could not be placed has page 0, and letting that stand as a page would let a
// gap in the placement quietly rule a candidate out and hand back a confident
// wrong answer. Better to fall through to the no. and, failing that, to report.
func onPage(cand []*Statement, page int) (Target, bool) {
	if page == 0 {
		return Target{}, false
	}
	var hit []*Statement
	for _, st := range cand {
		if st.Page == 0 {
			return Target{}, false
		}
		if st.Page == page {
			hit = append(hit, st)
		}
	}
	if len(hit) != 1 {
		return Target{}, false
	}
	return Target{Label: hit[0].Label, Tag: hit[0].Tag, How: ByStatementPage}, true
}

// nearest takes the candidate printed nearest above the sentence, and is what
// settles a bare "Corollary 1" in a § that has three of them.
//
// It only fires within one file, so a reference made from an exercise is left
// unresolved rather than pointed at whichever candidate happens to come first:
// the exercises are files of their own and there is no "above" to speak of.
//
// The rule is the reader's own. A corollary is numbered under the statement it
// hangs from, so a § restarts the count at every theorem, and prose that says
// "by Corollary 1" without saying more is standing inside the run it means.
// It fires 31 times in chapter VIII. Checked against the run in force at the
// citing line, which is the other thing "Corollary 1" could be read against,
// the two rules name the same statement 30 times and never disagree; the
// thirty-first is a sentence whose own run holds no Corollary 1, and reading it
// says the statement above is the one meant. So this is not the weaker of the
// two readings, and it resolves the cases the other one cannot see.
func nearest(cand []*Statement, at Site) (Target, bool) {
	var best *Statement
	for _, st := range cand {
		if st.Path != at.File || st.Line >= at.Line {
			continue
		}
		if best == nil || st.Line > best.Line {
			best = st
		}
	}
	if best == nil {
		return Target{}, false
	}
	return Target{Label: best.Label, Tag: best.Tag, How: ByNearest}, true
}
