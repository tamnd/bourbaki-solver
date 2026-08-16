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
	}, {
		// The corollary and the page it is printed on are one reference. Read as
		// two, the corollary is hunted for in the § doing the citing, which is
		// § 8 and holds no Proposition 12, and the page belongs to a chapter this
		// corpus does not have.
		name: "a corollary of a statement in another Book",
		in:   "and the corollary of Proposition 12 of II, §1, No. 8, p. 209, the mapping",
		want: []Citation{{Raw: "corollary of Proposition 12 of II, §1, No. 8, p. 209", Form: FormAttached,
			Chapter: "II", Section: 1, Subsec: 8, Page: 209,
			Kind: corpus.KindCorollary, ParentKind: corpus.KindProposition, ParentNumber: 12}},
	}, {
		name: "a corollary whose parent's page is in brackets",
		in:   "By Corollary 1 of Proposition 4 (VIII, p. 83), there exists an element b in A.",
		want: []Citation{{Raw: "Corollary 1 of Proposition 4 (VIII, p. 83)", Form: FormAttached,
			Chapter: "VIII", Page: 83,
			Kind: corpus.KindCorollary, Number: 1, ParentKind: corpus.KindProposition, ParentNumber: 4}},
	}, {
		// The third way the chapter writes the same thing, with a comma and no
		// word between the parent and its page.
		name: "a corollary whose parent's page follows a comma",
		in:   "By the corollary of Proposition 9, VIII, p. 71, the mapping is bijective.",
		want: []Citation{{Raw: "corollary of Proposition 9, VIII, p. 71", Form: FormAttached,
			Chapter: "VIII", Page: 71,
			Kind: corpus.KindCorollary, ParentKind: corpus.KindProposition, ParentNumber: 9}},
	}, {
		// The locator written first, which is what a bracketed aside does. This
		// has to be tried before the page form: that form takes the head of it
		// and leaves a bare "of Theorem 2" behind, and § 5 prints two statements
		// called Corollary 1, so throwing away the parent throws away the one
		// thing in the sentence that says which.
		name: "a corollary with its parent's page said first",
		in:   "is equal to $(D_{\\lambda})_{V_{\\lambda}}$ (VIII, p. 82, Corollary 1 of Theorem 2).",
		want: []Citation{{Raw: "VIII, p. 82, Corollary 1 of Theorem 2", Form: FormAttached,
			Chapter: "VIII", Page: 82,
			Kind: corpus.KindCorollary, Number: 1, ParentKind: corpus.KindTheorem, ParentNumber: 2}},
	}, {
		name: "an unnumbered corollary with its parent's page said first",
		in:   "there exists (II, §1, No. 9, p. 210, Corollary of Proposition 13) an isomorphism",
		want: []Citation{{Raw: "II, §1, No. 9, p. 210, Corollary of Proposition 13", Form: FormAttached,
			Chapter: "II", Section: 1, Subsec: 9, Page: 210,
			Kind: corpus.KindCorollary, ParentKind: corpus.KindProposition, ParentNumber: 13}},
	}, {
		// Without the abbreviation this does not fail, it succeeds wrongly, as
		// chapter III of Algebra: the Roman numeral is all the locator can see.
		name: "a Book cited by its abbreviation",
		in:   "Suppose that the ring A is Hausdorff and complete (Gen. Top., III, §6, No. 5, p. 276) for this",
		want: []Citation{{Raw: "Gen. Top., III, §6, No. 5, p. 276", Form: FormPage,
			Book: "Gen. Top.", Chapter: "III", Section: 6, Subsec: 5, Page: 276}},
	}, {
		name: "a Book cited by its abbreviation with the statement said first",
		in:   "By Theorem 3 of Comm. Alg., II, §5, No. 4, p. 114, the following properties are equivalent:",
		want: []Citation{{Raw: "Theorem 3 of Comm. Alg., II, §5, No. 4, p. 114", Form: FormNamed,
			Book: "Comm. Alg.", Chapter: "II", Section: 5, Subsec: 4, Page: 114,
			Kind: corpus.KindTheorem, Number: 3}},
	}, {
		// The exercises write the code instead, and FRV is the English
		// printing's own spelling of the code of FVR.
		name: "a Book cited by its code",
		in:   "and therefore $\\sum_i|\\chi_i(g)|^2\\geqslant s($FRV, III, §1, No. 1, p. 93, Proposition 2).",
		want: []Citation{{Raw: "FRV, III, §1, No. 1, p. 93, Proposition 2", Form: FormPage,
			Book: "FRV", Chapter: "III", Section: 1, Subsec: 1, Page: 93,
			Kind: corpus.KindProposition, Number: 2}},
	}, {
		// The second shape a part takes. Left unread the reference comes apart
		// into a bare Proposition 16, which § 20 does not have, and a page of
		// chapter II that nothing points at.
		name: "a part written in roman numerals",
		in:   "By Proposition 16, (ii) of II, §7, No. 7, p. 308, the mapping $j_n$ is injective.",
		want: []Citation{{Raw: "Proposition 16, (ii) of II, §7, No. 7, p. 308", Form: FormNamed,
			Chapter: "II", Section: 7, Subsec: 7, Page: 308,
			Kind: corpus.KindProposition, Number: 16}},
	}, {
		// One reference to the work cited last, not a local Proposition 14 and a
		// loc. cit. beside it.
		name: "a statement of the work cited last",
		in:   "such that we have PXQ = diag(1$, . . . ,1, \\delta )$ (follow the proof of Proposition 14 of loc. cit.).",
		want: []Citation{{Raw: "Proposition 14 of loc. cit.", Form: FormLocCit}},
	}, {
		// The printing keeps the French no. whenever it cites a volume that has
		// not been translated. The kind that follows is French too and is left
		// unread, which costs nothing: chapter X is not in the corpus and the
		// locator alone answers the reference.
		name: "the no. written in French",
		in:   "hence flat (X, §1, n$^o3$, p. 9, exemple 1), the A-module Hom$_B(Q,V)$ is injective",
		want: []Citation{{Raw: "X, §1, n$^o3$, p. 9", Form: FormPage,
			Chapter: "X", Section: 1, Subsec: 3, Page: 9}},
	}, {
		// The same, with a statement said first. Read without the French no. the
		// locator fails and a bare Proposition 6 is left behind, which § 8 has,
		// so the edge pointed at a statement of this chapter.
		name: "the no. written in French with the statement said first",
		in:   "For other characterizations of semisimple rings, see Proposition 6 of X, §8, n$^o4$, p. $140.*$",
		want: []Citation{{Raw: "Proposition 6 of X, §8, n$^o4$, p. $140", Form: FormNamed,
			Chapter: "X", Section: 8, Subsec: 4, Page: 140,
			Kind: corpus.KindProposition, Number: 6}},
	}, {
		// Two corollaries of the same Theorem, the second written by its number
		// alone. Both come out of the one match, so the second gets the locator
		// the first was read with rather than being hunted for on its own.
		name: "a second statement written by its number alone",
		in: "(Reduce to the case where $k'$ is algebraically closed, and use $b)$ " +
			"as well as Cor. 1 and 3 of Th. 1.)",
		want: []Citation{
			{Raw: "Cor. 1 and 3 of Th. 1", Form: FormAttached, Kind: corpus.KindCorollary, Number: 1,
				ParentKind: corpus.KindTheorem, ParentNumber: 1},
			{Raw: "Cor. 1 and 3 of Th. 1", Form: FormAttached, Kind: corpus.KindCorollary, Number: 3,
				ParentKind: corpus.KindTheorem, ParentNumber: 1},
		},
	}, {
		// The Book and the chapter are written once and the second § is written
		// bare. Left alone the second is a § of the chapter of Lie the sentence
		// stands in, and what it means is a § of chapter III of Algebra.
		name: "a second § under the same Book",
		in: "be identified with an element $\\Gamma^*\\in^2V^*($Algebra, Chap. III, §7, no. 4, " +
			"Prop. 7 and §11, no. 10) and it is easy to",
		want: []Citation{
			{Raw: "Algebra, Chap. III, §7, no. 4, Prop. 7", Form: FormSection,
				Book: "Algebra", Chapter: "III", Section: 7, Subsec: 4,
				Kind: corpus.KindProposition, Number: 7},
			{Raw: "§11, no. 10", Form: FormSection, Book: "Algebra", Chapter: "III",
				Section: 11, Subsec: 10},
		},
	}, {
		// The superscript belongs to a display further down the page and has
		// landed between the chapter and its §. Read no further than the chapter,
		// the § is a § of the chapter doing the citing and the corollary is hunted
		// for in it, which had chapter VIII reported as missing a Proposition 41.
		name: "a piece of a display standing inside the locator",
		in: "the $\\mathfrak{g}$-module structure of X (Chap. III,$^{\\alpha\\in B}$ §3, no. 11, " +
			"Cor. 3 of Prop. 41). Let C be",
		want: []Citation{{Raw: "Chap. III,$^{\\alpha\\in B}$ §3, no. 11, Cor. 3 of Prop. 41",
			Form: FormAttached, Chapter: "III", Section: 3, Subsec: 11,
			Kind: corpus.KindCorollary, Number: 3,
			ParentKind: corpus.KindProposition, ParentNumber: 41}},
	}, {
		// The other printing's way of naming a chapter and a page, and the Book
		// left in French because Algebra X had not been translated. Unread, the
		// whole locator goes and an Exercise 23 is looked for in the § of chapter
		// IX the sentence stands in, which has twelve.
		name: "a chapter and a page written the other printing's way",
		in:   "$d)$ Recover the results of $b)$ and $c)$ by using Exerc. 23 of Algèbre, Chap. X, p. 194.",
		want: []Citation{{Raw: "Exerc. 23 of Algèbre, Chap. X, p. 194", Form: FormNamed,
			Book: "Algèbre", Chapter: "X", Page: 194,
			Kind: corpus.KindExercise, Number: 23}},
	}, {
		// A no. of the § the sentence stands in. The no. is the whole of what it
		// says, and § 7 of chapter VIII prints six Remarks, so without it there is
		// nothing to choose between them.
		name: "a no. of the § doing the citing",
		in: "(ii) If E is simple and has highest weight $\\omega ,E^*$ is simple and has " +
			"highest weight $-w_0(\\omega )$ (cf. no. 2, Remark 2).",
		want: []Citation{{Raw: "no. 2, Remark 2", Form: FormLocal, Subsec: 2,
			Kind: corpus.KindRemark, Number: 2}},
	}, {
		// A statement of another author's paper, standing where that paper's own
		// pages are given. It is read so that nothing else reads it, and § 8 is no
		// longer reported as missing a Theorem 10 that belongs to Kostant.
		name: "a statement of a work outside the Éléments",
		in: "$^3$ It can be shown (B. KOSTANT, Lie group representations on polynomial rings, " +
			"Amer. J. Math., Vol. LXXXV (1963), pp. 327-404, Th. 10 and 15) that",
		want: nil,
	}, {
		// The Proposition says where to look and is not what is being looked at,
		// so the reference stops at the no. and the statement is dropped.
		name: "the text in front of a statement",
		in: "the set $\\mathscr{T}$ is a noetherian ordered set (Theory of Sets, Chap. III, §6, " +
			"no. 5, text preceding Prop. 7).",
		want: []Citation{{Raw: "Theory of Sets, Chap. III, §6, no. 5, text preceding Prop. 7",
			Form: FormSection, Book: "Theory of Sets", Chapter: "III", Section: 6, Subsec: 5}},
	}, {
		// The corollary goes with the loc. cit. and so does its parent. Splitting
		// them leaves a Proposition 3 behind in § 10 of chapter VIII, which prints
		// no Proposition at all.
		name: "a corollary of a statement of the work cited last",
		in:   "it is a decomposable Lie algebra (loc. cit., Cor. 1 of Prop. 3).",
		want: []Citation{{Raw: "loc. cit., Cor. 1 of Prop. 3", Form: FormLocCit}},
	}, {
		// A second no. of the same §, which is what the other printing writes
		// where Algebra VIII writes a second page of the same chapter.
		name: "a second no. in the same bracket",
		in:   "By Chap. III, §3, no. 8, Cor. 2 of Prop. 29, and no. 10, Prop. 36, Aut($\\mathfrak{g}$)",
		want: []Citation{
			{Raw: "Chap. III, §3, no. 8, Cor. 2 of Prop. 29", Form: FormAttached,
				Chapter: "III", Section: 3, Subsec: 8,
				Kind: corpus.KindCorollary, Number: 2,
				ParentKind: corpus.KindProposition, ParentNumber: 29},
			{Raw: "and no. 10, Prop. 36", Form: FormSection,
				Chapter: "III", Section: 3, Subsec: 10,
				Kind: corpus.KindProposition, Number: 36},
		},
	}, {
		// Theory of Sets writes the word out where the other printings write it
		// short. Read only in the short form the chapter went unread and the § was
		// taken for a § of the chapter doing the citing, so § 1 of chapter III was
		// reported as citing a no. 6 of a § that has five.
		name: "a chapter written out in full",
		in: "The condition for the existence of a common extension of a family of mappings " +
			"belonging to $\\Phi(E, F)$ (Chapter II, § 4, no. 6, Proposition 7) shows that",
		want: []Citation{{Raw: "Chapter II, § 4, no. 6, Proposition 7", Form: FormSection,
			Chapter: "II", Section: 4, Subsec: 6,
			Kind: corpus.KindProposition, Number: 7}},
	}, {
		// The sign set as TeX, which is how page 178 of Theory of Sets writes it
		// four times over. Nothing here was read at all before, not even as an
		// unresolved reference, because the line carries no § for a locator to
		// stop on.
		name: "a § written as TeX with a thin space",
		in: "and its cardinal is that of the set of subsets of E ($\\S\\,3$, no. 5, Proposition 12), " +
			"so that we may write",
		want: []Citation{{Raw: "§ 3, no. 5, Proposition 12", Form: FormSection,
			Section: 3, Subsec: 5,
			Kind: corpus.KindProposition, Number: 12}},
	}, {
		// The same sign with a space instead of a thin space, and the dollar
		// closing between the § and what follows it. Read with the dollar left in,
		// the reference comes apart into a § 2 nothing points at and a bare
		// Exercise 13 hunted for in § 3, which prints six.
		name: "a § written as TeX in front of an exercise",
		in: "Let $(\\lambda_\\iota)_{\\iota \\in I}$ be a family of order-types ($\\S 2$, Exercise 13), " +
			"indexed by an ordered set I.",
		want: []Citation{{Raw: "§ 2, Exercise 13", Form: FormSection,
			Section: 2, Kind: corpus.KindExercise, Number: 13}},
	}, {
		// \Sigma opens the way \S does and the corpus is full of it, so the rule
		// has to tell them apart or every sum in the volume turns into a §.
		name: "a sum sign is not a §",
		in:   "where $\\Sigma$ is the sum, and by Prop. 3 (§2, no. 4) we have",
		want: []Citation{{Raw: "Prop. 3 (§2, no. 4)", Form: FormSection,
			Section: 2, Subsec: 4,
			Kind: corpus.KindProposition, Number: 3}},
	}, {
		// The § in brackets after the statement instead of after an "of", which
		// the page form has read since it was written. Left to fall through it is
		// a bare Prop. 3 hunted for in § 4, which prints four Propositions, and a
		// § 2 that nothing points at.
		name: "a § in brackets after the statement",
		in:   "The homomorphism $f(G,T)$ is surjective by Prop. 3 (§2, no. 4). We denote by",
		want: []Citation{{Raw: "Prop. 3 (§2, no. 4)", Form: FormSection,
			Section: 2, Subsec: 4, Kind: corpus.KindProposition, Number: 3}},
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

// Every spelling of a Book that is not this one leaves the corpus under the
// code the Éléments give it, which is what the report that says what to ingest
// next is counted on. Before the abbreviations were read, 37 references of the
// chapter were counted against a chapter of Algebra instead.
func TestABookCitedByAnythingButItsFullTitleStillLeavesTheCorpus(t *testing.T) {
	for _, tc := range []struct{ in, book, code string }{
		{"complete (Gen. Top., III, §6, No. 5, p. 276) for this topology", "Gen. Top.", "TG"},
		{"By Theorem 3 of Comm. Alg., II, §5, No. 4, p. 114, the following", "Comm. Alg.", "AC"},
		{"$\\geqslant s($FRV, III, §1, No. 1, p. 93, Proposition 2).", "FRV", "FVR"},
		{"the bracket (Lie, I, §2, No. 7, p. 21) is then", "Lie", "LIE"},
		{"the ring A is regular (AC, X, §4, n$^o2$, p. 55), then", "AC", "AC"},
		{"By TS, II, §3, n$^o2$, p. 252, the mapping", "TS", "TS"},
		{"is closed in D (EVT, I, §2, n$^o3$, p. 14), we obtain", "EVT", "EVT"},
	} {
		t.Run(tc.book, func(t *testing.T) {
			got := Parse(tc.in, 1)
			if len(got) != 1 {
				t.Fatalf("read %d citations: %+v", len(got), got)
			}
			if got[0].Book != tc.book {
				t.Fatalf("the Book came out as %q, want %q", got[0].Book, tc.book)
			}
			// The index is empty on purpose: a reference that leaves the corpus is
			// answered before anything is looked up in one.
			target, err := (&Index{}).Resolve(got[0], Site{})
			if err != nil {
				t.Fatal(err)
			}
			if target.How != OutOfCorpus || target.Book != tc.code {
				t.Errorf("resolved by %s to Book %q, want %s to %q",
					target.How, target.Book, OutOfCorpus, tc.code)
			}
		})
	}
}

// Algebra is this Book under either spelling, so neither of them may be sent
// out of the corpus the way the Books above are.
func TestThisBookIsNotSentOutOfTheCorpusByItsOwnName(t *testing.T) {
	for _, name := range []string{"Algebra", "Alg."} {
		if code := Code(name); code != "A" {
			t.Fatalf("%q has code %q, want A", name, code)
		}
		c := Citation{Book: name, Chapter: "III", Page: 100, Form: FormPage}
		target, err := (&Index{}).Resolve(c, Site{})
		if err != nil {
			t.Fatal(err)
		}
		// Chapter III of Algebra is out of the corpus because the corpus holds
		// one chapter, not because the Book is another Book.
		if target.Book != "A" {
			t.Errorf("%q resolved to Book %q, want A", name, target.Book)
		}
	}
}

// A run whose locator is written on its end leaves every member in front of it
// bare, and the second reading is recorded against those and against nothing
// else. The sentences are the two the volume writes either way: the first is the
// exercise whose "Cor. 2 of Th. 1" belongs to § 6 along with the Prop. 4 beside
// it, and the second is the exercise that has already cited Exerc. 10 twice as
// one of its own §, where only the member the locator is written on belongs to
// chapter VI.
func TestALocatorOnTheEndOfARunIsReadAsASecondReading(t *testing.T) {
	got := Parse("(use $c)$ as well as Cor. 2 of Th. 1 and Prop. 4 (§6, no. 2 and 3)).", 1)
	if len(got) != 2 {
		t.Fatalf("read %d citations: %+v", len(got), got)
	}
	// The reference as printed says no § and is still read that way.
	if got[0].Raw != "Cor. 2 of Th. 1" || got[0].Section != 0 {
		t.Fatalf("the first is %+v", got[0])
	}
	u := got[0].Under
	if u == nil {
		t.Fatal("the first has no second reading")
	}
	if u.Section != 6 || u.Subsec != 2 || u.Form != FormAttached ||
		u.Kind != corpus.KindCorollary || u.Number != 2 ||
		u.ParentKind != corpus.KindTheorem || u.ParentNumber != 1 {
		t.Errorf("the second reading is %+v", *u)
	}
	// The member the locator was written on has nothing to be given.
	if got[1].Under != nil {
		t.Errorf("the member that wrote the § has a second reading %+v", *got[1].Under)
	}

	// Each member with a § of its own is left alone, since there is no one
	// locator for the run to hand back.
	for _, c := range Parse("use Exerc. 4 of §3 and Exerc. 11 of §5.", 1) {
		if c.Under != nil {
			t.Errorf("%q has a second reading %+v", c.Raw, *c.Under)
		}
	}
	// Prose between the two breaks the run, so a statement named a few words
	// later is not a member of it.
	for _, c := range Parse("by Th. 1, and it follows from Prop. 6 of §3, no. 6 that", 1) {
		if c.Under != nil {
			t.Errorf("%q has a second reading %+v", c.Raw, *c.Under)
		}
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
