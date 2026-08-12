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
	// BySection is a reference to a page and nothing on it. It resolves to a §
	// and to no statement, because there is no statement in it to resolve to.
	BySection = "section"
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

// Chapter is the chapter of Algebra this corpus holds. A reference to any other
// chapter is a reference out of the corpus even though it is the same Book,
// which is worth keeping in the report: it is the closest thing to ingest next.
const Chapter = "VIII"

// Site is where the sentence making a reference stands. The § is what a bare
// "Proposition 4" means. The file and the line are what settles a bare
// "Corollary 1" in a § that prints several, since the one meant is the one
// printed nearest above.
type Site struct {
	Section string
	File    string
	Line    int
}

// Resolve looks a citation up.
func (ix *Index) Resolve(c Citation, at Site) (Target, error) {
	if code := Code(c.Book); code != "" && c.Book != "Algebra" {
		return Target{How: OutOfCorpus, Book: code}, nil
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
		return ix.inSection(c, from, ByContext, 0, at)
	case FormAttached:
		return ix.attachedIn(c, from)
	}
	if c.Chapter != Chapter {
		return Target{How: OutOfCorpus, Book: "A"}, nil
	}
	on := ix.SectionAt(c.Chapter, c.Page)
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
func (ix *Index) attachedIn(c Citation, from string) (Target, error) {
	if c.Chapter == "" {
		return ix.attached(c, from)
	}
	if c.Chapter != Chapter {
		return Target{How: OutOfCorpus, Book: "A"}, nil
	}
	on := ix.SectionAt(c.Chapter, c.Page)
	switch len(on) {
	case 0:
		return Target{}, fmt.Errorf("no § of chapter %s is printed on page %d", c.Chapter, c.Page)
	case 1:
	default:
		return Target{}, fmt.Errorf("page %d is in %d sections at once", c.Page, len(on))
	}
	return ix.attached(c, on[0].Label)
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
		return Target{}, fmt.Errorf("%s has no corollary %d of %s %d",
			section, n, c.ParentKind.Heading(), c.ParentNumber)
	}
	return Target{Label: under[n-1].Label, Tag: under[n-1].Tag, How: ByParent}, nil
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

// inSection finds the statement of a kind and number in one §, bringing the no.
// in only when the § holds more than one candidate. Nothing is narrowed that
// does not need narrowing, so a wrong no. cannot turn a lookup that would have
// worked into one that does not.
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
	return Target{Label: hit[0].Label, Tag: hit[0].Tag, How: ByPageAndNo}, nil
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
