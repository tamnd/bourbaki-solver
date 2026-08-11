package refs

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The sentences are taken off the page rather than invented, because a grammar
// written against invented citations is a grammar for a book nobody printed.
// Each one is a line of chapter VIII with the file it came from named beside it.
func TestParse(t *testing.T) {
	for _, tc := range []struct {
		name string
		in   string
		want []Citation
	}{{
		name: "a page and nothing on it",
		in:   "the ring E is neither left nor right Artinian (VIII, p. 2).",
		want: []Citation{{Raw: "VIII, p. 2", Form: FormPage, Chapter: "VIII", Page: 2}},
	}, {
		name: "a page and a statement on it",
		in:   "(resp. Noetherian) (VIII, p. 2, Example 2). In particular,",
		want: []Citation{{Raw: "VIII, p. 2, Example 2", Form: FormPage,
			Chapter: "VIII", Page: 2, Kind: corpus.KindExample, Number: 2}},
	}, {
		name: "a § and a no. before the page",
		in:   "there exists a linear form (II, §7, No. 5, p. 300, Theorem 6). For every",
		want: []Citation{{Raw: "II, §7, No. 5, p. 300, Theorem 6", Form: FormPage,
			Chapter: "II", Section: 7, Subsec: 5, Page: 300, Kind: corpus.KindTheorem, Number: 6}},
	}, {
		name: "another Book",
		in:   "by the axiom of choice (Set Theory, III, §2, No. 4, p. 155).",
		want: []Citation{{Raw: "Set Theory, III, §2, No. 4, p. 155", Form: FormPage,
			Book: "Set Theory", Chapter: "III", Section: 2, Subsec: 4, Page: 155}},
	}, {
		// The whole point of trying the named form first. Read the other way
		// round this is two references, a Proposition of the § in hand and a
		// page of chapter II that nothing points at.
		name: "the statement said first",
		in:   "This follows from Proposition 25 of II, §1, No. 13, p. 222.",
		want: []Citation{{Raw: "Proposition 25 of II, §1, No. 13, p. 222", Form: FormNamed,
			Chapter: "II", Section: 1, Subsec: 13, Page: 222,
			Kind: corpus.KindProposition, Number: 25}},
	}, {
		name: "a statement of the § being read",
		in:   "It is generating by Proposition 2, and the result follows.",
		want: []Citation{{Raw: "Proposition 2", Form: FormLocal,
			Kind: corpus.KindProposition, Number: 2}},
	}, {
		name: "a corollary named by its parent",
		in:   "By Corollary 1 of Proposition 4, there exists an element b in A.",
		want: []Citation{{Raw: "Corollary 1 of Proposition 4", Form: FormAttached,
			Kind: corpus.KindCorollary, Number: 1,
			ParentKind: corpus.KindProposition, ParentNumber: 4}},
	}, {
		// Number stays 0. The book left it unnumbered and so does the citation,
		// which is what tells the resolver to look for it by where it stands.
		name: "an unnumbered corollary named by its parent",
		in:   "This follows from the corollary of Proposition 2.",
		want: []Citation{{Raw: "corollary of Proposition 2", Form: FormAttached,
			Kind: corpus.KindCorollary, ParentKind: corpus.KindProposition, ParentNumber: 2}},
	}, {
		name: "a numbered display",
		in:   "where property d) results from formula (4).",
		want: []Citation{{Raw: "formula (4)", Form: FormFormula,
			Kind: corpus.KindEquation, Number: 4}},
	}, {
		// Two of the four forms in one sentence, which is the ordinary case and
		// not a hard one: the page citation is in brackets and the local one is
		// the subject.
		name: "a local reference beside an out-of-chapter page",
		in:   "the left homothety of this ring (I, §8, No. 1, p. 97) sends Proposition 7 to",
		want: []Citation{
			{Raw: "I, §8, No. 1, p. 97", Form: FormPage, Chapter: "I", Section: 8, Subsec: 1, Page: 97},
			{Raw: "Proposition 7", Form: FormLocal, Kind: corpus.KindProposition, Number: 7},
		},
	}, {
		name: "an appendix",
		in:   "as shown in VIII, Appendix, p. 429, Proposition 3.",
		want: []Citation{{Raw: "VIII, Appendix, p. 429, Proposition 3", Form: FormPage,
			Chapter: "VIII", Appendix: true, Page: 429,
			Kind: corpus.KindProposition, Number: 3}},
	}, {
		// A heading is the statement, not a reference to it.
		name: "a heading is not a citation",
		in:   "#### Proposition 7 {#alg-viii-s1-prop-7 .statement tag=000X}",
		want: nil,
	}, {
		// Where a bold lead survives in a body, extraction failed to turn it
		// into a heading. It is still not a reference, and counting it as one
		// would have every swallowed statement cite itself.
		name: "a surviving bold lead is not a citation",
		in:   "The sequence is then stationary by the following lemma. **Lemma 2.** — Let M be an A-module",
		want: nil,
	}, {
		// Read and counted, never resolved. What it points at is several
		// sentences back and guessing would build an edge that looks right.
		name: "loc. cit.",
		in:   "the same then holds for $M/(M_1\\cap M_2)$ (loc. cit.).",
		want: []Citation{{Raw: "loc. cit.", Form: FormLocCit}},
	}, {
		name: "an exercise",
		in:   "as in VIII, p. 155, Exercise 9, the module is faithful.",
		want: []Citation{{Raw: "VIII, p. 155, Exercise 9", Form: FormPage,
			Chapter: "VIII", Page: 155, Kind: corpus.KindExercise, Number: 9}},
	}, {
		// The same as the named form with the locator bracketed instead of
		// following "of". Read as two references it would be a bare Remark 1 of
		// whatever § the sentence is in and a page nothing points at.
		name: "the statement said first with the page in brackets",
		in:   "follows from Proposition 4, a) and Remark 1 (VIII, p. 97). Assume",
		want: []Citation{
			{Raw: "Proposition 4", Form: FormLocal, Kind: corpus.KindProposition, Number: 4},
			{Raw: "Remark 1 (VIII, p. 97", Form: FormNamed,
				Chapter: "VIII", Page: 97, Kind: corpus.KindRemark, Number: 1},
		},
	}, {
		// The letter is the part of the statement meant. Nothing is done with
		// it, since a part carries no tag, but it has to be read past.
		name: "a part of a statement",
		in:   "therefore $\\mathfrak{R}\\subset N$ by Corollary 1, c) of VIII, p. 152.",
		want: []Citation{{Raw: "Corollary 1, c) of VIII, p. 152", Form: FormNamed,
			Chapter: "VIII", Page: 152, Kind: corpus.KindCorollary, Number: 1}},
	}, {
		// The second page in the bracket does not repeat the chapter.
		name: "a second page in the same bracket",
		in:   "have the same class in $R_K(G)$ (VIII, p. 190, Corollary and p. 401, Corollary 1).",
		want: []Citation{
			{Raw: "VIII, p. 190", Form: FormPage, Chapter: "VIII", Page: 190},
			{Raw: "p. 401, Corollary 1", Form: FormPage, Chapter: "VIII", Page: 401,
				Kind: corpus.KindCorollary, Number: 1},
		},
	}, {
		// Page 213 was run into the maths that follows it when the page was
		// transcribed. Without the dollar the locator fails and a bare Corollary
		// 2 is left behind, pointing at this § rather than at Algebra II.
		name: "a page number pulled into the maths",
		in:   "By Corollary 2 of II, §1, No. 10, p. $213,\\theta$ is bijective.",
		want: []Citation{{Raw: "Corollary 2 of II, §1, No. 10, p. $213", Form: FormNamed,
			Chapter: "II", Section: 1, Subsec: 10, Page: 213,
			Kind: corpus.KindCorollary, Number: 2}},
	}, {
		// Nothing on the line named a chapter, so there is nothing to carry and
		// the reference is dropped rather than pinned on the § being read.
		name: "a bare page with no chapter to carry",
		in:   "and p. 401, Corollary 1 gives the result.",
		want: nil,
	}} {
		t.Run(tc.name, func(t *testing.T) {
			got := Parse(tc.in, 1)
			if len(got) != len(tc.want) {
				t.Fatalf("read %d citations, want %d: %+v", len(got), len(tc.want), got)
			}
			for i, w := range tc.want {
				w.Line = 1
				if got[i] != w {
					t.Errorf("citation %d is\n\t%+v\nwant\n\t%+v", i, got[i], w)
				}
			}
		})
	}
}

// Commutative Algebra has to be tried before Algebra or the alternation takes
// the tail of it and reads the reference as one to the wrong Book.
func TestParsePrefersTheLongerBookTitle(t *testing.T) {
	got := Parse("as in Commutative Algebra, II, §2, No. 5, p. 71, Proposition 11.", 1)
	if len(got) != 1 {
		t.Fatalf("read %d citations: %+v", len(got), got)
	}
	if got[0].Book != "Commutative Algebra" {
		t.Errorf("the Book came out as %q", got[0].Book)
	}
	if Code(got[0].Book) != "AC" {
		t.Errorf("the code came out as %q", Code(got[0].Book))
	}
}

func TestParseCountsLines(t *testing.T) {
	body := "no citation here\nby Proposition 2\n\nand VIII, p. 3\n"
	got := Parse(body, 12)
	if len(got) != 2 {
		t.Fatalf("read %d citations: %+v", len(got), got)
	}
	if got[0].Line != 13 {
		t.Errorf("the first is at line %d, want 13", got[0].Line)
	}
	if got[1].Line != 15 {
		t.Errorf("the second is at line %d, want 15", got[1].Line)
	}
}
