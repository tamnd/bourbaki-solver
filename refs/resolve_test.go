package refs

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// testIndex is two sections of chapter VIII with the shapes that make the
// lookups hard: a Corollary the book numbered and one it did not, both hanging
// off the same Proposition, a Remark repeated in two no.s of one §, and two
// statements called Corollary 1, which is the shape eighteen of the chapter's
// twenty-two sections have.
//
// The lines are the lines of the file the § is written in, because a bare
// "Corollary 1" is settled by which candidate is printed above the sentence.
func testIndex() *Index {
	const s1file = "content/en/alg/VIII/01_s1.md"
	s1 := Section{
		Label: "alg-viii-s1", Chapter: "VIII", First: 1, Last: 23, Exercises: 12,
		Subsecs: []Subsec{{No: 1, Page: 1}, {No: 2, Page: 9}, {No: 3, Page: 15}},
		Statements: []*Statement{
			{Label: "alg-viii-s1-prop-1", Tag: "0001", Kind: corpus.KindProposition, Number: 1, Subsec: 1,
				Path: s1file, Line: 10},
			{Label: "alg-viii-s1-n1-rem-1", Tag: "0002", Kind: corpus.KindRemark, Number: 0, Subsec: 1,
				FollowsKind: corpus.KindProposition, FollowsNumber: 1, Path: s1file, Line: 20},
			{Label: "alg-viii-s1-prop-2", Tag: "0003", Kind: corpus.KindProposition, Number: 2, Subsec: 2,
				Path: s1file, Line: 30},
			{Label: "alg-viii-s1-prop-2-cor-1", Tag: "000A", Kind: corpus.KindCorollary, Number: 1, Subsec: 2,
				Named: true, FollowsKind: corpus.KindProposition, FollowsNumber: 2, Path: s1file, Line: 34},
			{Label: "alg-viii-s1-n2-cor-1", Tag: "0004", Kind: corpus.KindCorollary, Subsec: 2,
				FollowsKind: corpus.KindProposition, FollowsNumber: 2, Path: s1file, Line: 40},
			{Label: "alg-viii-s1-n2-cor-2", Tag: "0005", Kind: corpus.KindCorollary, Subsec: 2,
				FollowsKind: corpus.KindProposition, FollowsNumber: 2, Path: s1file, Line: 44},
			{Label: "alg-viii-s1-thm-1", Tag: "0006", Kind: corpus.KindTheorem, Number: 1, Subsec: 3,
				Path: s1file, Line: 60},
			{Label: "alg-viii-s1-thm-1-cor-1", Tag: "0007", Kind: corpus.KindCorollary, Number: 1, Subsec: 3,
				Named: true, FollowsKind: corpus.KindTheorem, FollowsNumber: 1, Path: s1file, Line: 64},
		},
	}
	s2 := Section{
		Label: "alg-viii-s2", Chapter: "VIII", First: 24, Last: 40,
		Subsecs: []Subsec{{No: 1, Page: 24}, {No: 2, Page: 33}},
		Statements: []*Statement{
			{Label: "alg-viii-s2-n1-exa-1", Tag: "0008", Kind: corpus.KindExample, Number: 1, Subsec: 1},
			{Label: "alg-viii-s2-n2-exa-1", Tag: "0009", Kind: corpus.KindExample, Number: 1, Subsec: 2},
		},
	}
	ix := &Index{Sections: []Section{s1, s2}}
	ix.index()
	return ix
}

func TestResolve(t *testing.T) {
	ix := testIndex()
	for _, tc := range []struct {
		name  string
		in    string
		from  string
		label string
		how   string
		book  string
		err   string
	}{{
		name: "a page and a statement on it", in: "VIII, p. 3, Proposition 1",
		label: "alg-viii-s1-prop-1", how: ByPage,
	}, {
		name: "a page and nothing on it", in: "VIII, p. 3",
		label: "alg-viii-s1", how: BySection,
	}, {
		// Example 1 is printed twice in § 2, once in each no., so the page has
		// to say which. Page 33 is where no. 2 starts.
		name: "a kind that repeats inside the §", in: "VIII, p. 35, Example 1",
		label: "alg-viii-s2-n2-exa-1", how: ByPageAndNo,
	}, {
		name: "the same kind on the other side of the §", in: "VIII, p. 25, Example 1",
		label: "alg-viii-s2-n1-exa-1", how: ByPageAndNo,
	}, {
		name: "an exercise", in: "VIII, p. 20, Exercise 9",
		label: "alg-viii-s1-ex-9", how: ByExercise,
	}, {
		name: "an exercise the § does not have", in: "VIII, p. 20, Exercise 90",
		err: "12 exercises and none numbered 90",
	}, {
		name: "the statement said first", in: "Proposition 2 of VIII, p. 10",
		label: "alg-viii-s1-prop-2", how: ByPage,
	}, {
		name: "a statement of the § being read", in: "by Proposition 2",
		from: "alg-viii-s1", label: "alg-viii-s1-prop-2", how: ByContext,
	}, {
		// The corollary the book numbered carries its parent in its own label,
		// so this needs no search at all.
		name: "a numbered corollary named by its parent", in: "Corollary 1 of Theorem 1",
		from: "alg-viii-s1", label: "alg-viii-s1-thm-1-cor-1", how: ByParent,
	}, {
		// This one does not. It is found by standing under Proposition 2.
		name: "an unnumbered corollary named by its parent", in: "the corollary of Proposition 2",
		from: "alg-viii-s1", label: "alg-viii-s1-n2-cor-1", how: ByParent,
	}, {
		name: "the second unnumbered corollary of a proposition", in: "Corollary 2 of Proposition 2",
		from: "alg-viii-s1", label: "alg-viii-s1-n2-cor-2", how: ByParent,
	}, {
		name: "a corollary of a statement that has none", in: "the corollary of Proposition 1",
		from: "alg-viii-s1", err: "no corollary 1 of Proposition 1",
	}, {
		// The page says which § the parent stands in, so the § doing the citing
		// has nothing to do with it. Read without the page this is a Proposition
		// 2 of § 2, which § 2 does not print.
		name: "a corollary whose parent's page names another §", in: "Corollary 1 of Proposition 2 (VIII, p. 10)",
		from: "alg-viii-s2", label: "alg-viii-s1-prop-2-cor-1", how: ByParent,
	}, {
		name: "an unnumbered corollary whose parent's page names another §", in: "the corollary of Proposition 2 of VIII, p. 10",
		from: "alg-viii-s2", label: "alg-viii-s1-n2-cor-1", how: ByParent,
	}, {
		// Another chapter of Algebra, and there is nothing here to look it up
		// in. Hunting for it in the § doing the citing is how this used to fail.
		name: "a corollary of a statement in another chapter", in: "the corollary of Proposition 12 of II, §1, No. 8, p. 209",
		from: "alg-viii-s1", how: OutOfCorpus, book: "A",
	}, {
		name: "a corollary whose parent's page is in no §", in: "Corollary 1 of Proposition 2 (VIII, p. 900)",
		from: "alg-viii-s1", err: "no § of chapter VIII is printed on page 900",
	}, {
		name: "another Book", in: "Set Theory, III, §2, No. 4, p. 155",
		how: OutOfCorpus, book: "E",
	}, {
		// Same Book, different chapter. Not an error, and the count of these is
		// what says which chapter to ingest next.
		name: "another chapter of Algebra", in: "II, §7, No. 5, p. 300, Theorem 6",
		how: OutOfCorpus, book: "A",
	}, {
		name: "a numbered display", in: "formula (4)",
		err: "carries no tag of its own",
	}, {
		name: "a page no § is printed on", in: "VIII, p. 900",
		err: "no § of chapter VIII is printed on page 900",
	}, {
		name: "a statement the § does not have", in: "by Proposition 9",
		from: "alg-viii-s1", err: "alg-viii-s1 has no Proposition 9",
	}, {
		name: "a local reference in no §", in: "by Proposition 2",
		err: "the sentence is in no §",
	}, {
		// The exercises are not in the statement index, and one exercise citing
		// another of the same § is how the book cross-refers within a run of
		// them.
		name: "an exercise cited in prose", in: "as in Exercise 9",
		from: "alg-viii-s1", label: "alg-viii-s1-ex-9", how: ByExercise,
	}, {
		name: "an exercise cited in prose that the § does not have", in: "as in Exercise 90",
		from: "alg-viii-s1", err: "12 exercises and none numbered 90",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			cs := Parse(tc.in, 1)
			if len(cs) != 1 {
				t.Fatalf("%q read as %d citations: %+v", tc.in, len(cs), cs)
			}
			got, err := ix.Resolve(cs[0], Site{Section: tc.from})
			if tc.err != "" {
				if err == nil {
					t.Fatalf("resolved to %+v, want an error about %q", got, tc.err)
				}
				if !strings.Contains(err.Error(), tc.err) {
					t.Errorf("the error is %q, want it to mention %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("did not resolve: %v", err)
			}
			if got.Label != tc.label {
				t.Errorf("resolved to %q, want %q", got.Label, tc.label)
			}
			if got.How != tc.how {
				t.Errorf("resolved by %q, want %q", got.How, tc.how)
			}
			if got.Book != tc.book {
				t.Errorf("the book is %q, want %q", got.Book, tc.book)
			}
			if tc.how != OutOfCorpus && tc.how != BySection && got.Tag == "" && tc.how != ByExercise {
				t.Errorf("%s resolved with no tag", got.Label)
			}
		})
	}
}

// A citation that names a no. is narrowed by it, but a citation that does not
// must still resolve when there is nothing to narrow. Narrowing something that
// needs no narrowing is how a wrong no. turns a lookup that would have worked
// into one that does not.
func TestResolveNarrowsOnlyWhenItHasTo(t *testing.T) {
	ix := testIndex()
	// § 1 has one Proposition 1 and the page falls in no. 1, so the no. agrees.
	c := Parse("VIII, §1, No. 3, p. 3, Proposition 1", 1)
	if len(c) != 1 {
		t.Fatalf("read %d citations: %+v", len(c), c)
	}
	got, err := ix.Resolve(c[0], Site{})
	if err != nil {
		t.Fatalf("a no. that disagrees with the page broke a lookup with one candidate: %v", err)
	}
	if got.Label != "alg-viii-s1-prop-1" || got.How != ByPage {
		t.Errorf("resolved to %+v", got)
	}
}

// § 1 has two statements called Corollary 1, one under Proposition 2 and one
// under Theorem 1, which is the ordinary shape: a § restarts the corollary
// count at every statement that has corollaries. Prose that says "by Corollary
// 1" and nothing else means the one it is standing under.
func TestResolveTakesTheNearestCorollaryAbove(t *testing.T) {
	ix := testIndex()
	const file = "content/en/alg/VIII/01_s1.md"
	for _, tc := range []struct {
		name  string
		line  int
		label string
		how   string
		err   string
	}{{
		name: "in the proof under Proposition 2", line: 50,
		label: "alg-viii-s1-prop-2-cor-1", how: ByNearest,
	}, {
		name: "in the proof under Theorem 1", line: 70,
		label: "alg-viii-s1-thm-1-cor-1", how: ByNearest,
	}, {
		// Nothing of that name is printed above the sentence, so there is
		// nothing to be nearest to and the reference is reported instead.
		name: "above both of them", line: 12,
		err: "2 statements called Corollary 1 and nothing says which",
	}, {
		// An exercise is a file of its own, so "above" says nothing about the §
		// and the reference is left alone rather than pointed at the last one.
		name: "from an exercise", line: 70,
		err: "2 statements called Corollary 1 and nothing says which",
	}} {
		t.Run(tc.name, func(t *testing.T) {
			at := Site{Section: "alg-viii-s1", File: file, Line: tc.line}
			if tc.name == "from an exercise" {
				at.File = "content/en/alg/VIII/exercises/s1/03.md"
			}
			c := Parse("by Corollary 1", tc.line)
			if len(c) != 1 {
				t.Fatalf("read %d citations: %+v", len(c), c)
			}
			got, err := ix.Resolve(c[0], at)
			if tc.err != "" {
				if err == nil {
					t.Fatalf("resolved to %+v, want an error about %q", got, tc.err)
				}
				if !strings.Contains(err.Error(), tc.err) {
					t.Errorf("the error is %q, want it to mention %q", err, tc.err)
				}
				return
			}
			if err != nil {
				t.Fatalf("did not resolve: %v", err)
			}
			if got.Label != tc.label || got.How != tc.how {
				t.Errorf("resolved to %q by %q, want %q by %q", got.Label, got.How, tc.label, tc.how)
			}
		})
	}
}

// A reference that carries a page has said where to look, and taking whatever
// is nearest instead would throw that away. § 1 prints two Corollary 1s and
// page 3 is in no. 1, which holds neither, so this is left unresolved even
// though a Corollary 1 stands above the sentence.
func TestResolveDoesNotTakeTheNearestWhenAPageSaidWhere(t *testing.T) {
	ix := testIndex()
	c := Parse("Corollary 1 of VIII, p. 3", 70)
	if len(c) != 1 {
		t.Fatalf("read %d citations: %+v", len(c), c)
	}
	got, err := ix.Resolve(c[0], Site{Section: "alg-viii-s1", File: "content/en/alg/VIII/01_s1.md", Line: 70})
	if err == nil {
		t.Fatalf("resolved to %+v, want the page to be reported rather than guessed past", got)
	}
	if !strings.Contains(err.Error(), "0 of them in no. 1") {
		t.Errorf("the error is %q", err)
	}
}

func TestSubsecAt(t *testing.T) {
	s := testIndex().Section("alg-viii-s1")
	for _, tc := range []struct{ page, no int }{{1, 1}, {8, 1}, {9, 2}, {14, 2}, {15, 3}, {23, 3}, {0, 0}} {
		if got := s.SubsecAt(tc.page); got != tc.no {
			t.Errorf("page %d is in no. %d, want %d", tc.page, got, tc.no)
		}
	}
}

func TestSectionAt(t *testing.T) {
	ix := testIndex()
	if got := ix.SectionAt("VIII", 30); len(got) != 1 || got[0].Label != "alg-viii-s2" {
		t.Errorf("page 30 is in %+v", got)
	}
	if got := ix.SectionAt("VIII", 900); len(got) != 0 {
		t.Errorf("page 900 is in %+v", got)
	}
	// A page of another chapter is not a page of this one, however well the
	// number happens to land.
	if got := ix.SectionAt("II", 3); len(got) != 0 {
		t.Errorf("page 3 of chapter II is in %+v", got)
	}
}
