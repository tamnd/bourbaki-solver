package assemble

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

func blocks(texts ...string) []block {
	out := make([]block, 0, len(texts))
	for _, t := range texts {
		out = append(out, block{text: t, page: 42, label: "A VIII.7"})
	}
	return out
}

var vii = corpus.Ref{Book: "alg", Chapter: "VIII", Section: 1}

func texts(bs []block) []string {
	out := make([]string, 0, len(bs))
	for _, b := range bs {
		out = append(out, b.text)
	}
	return out
}

func labels(ss []corpus.Statement) []string {
	out := make([]string, 0, len(ss))
	for _, s := range ss {
		out = append(out, s.Label())
	}
	return out
}

func same(t *testing.T, got, want []string) {
	t.Helper()
	if len(got) != len(want) {
		t.Fatalf("got %d\n  %v\nwant %d\n  %v", len(got), got, len(want), want)
	}
	for i := range got {
		if got[i] != want[i] {
			t.Errorf("[%d] = %q, want %q", i, got[i], want[i])
		}
	}
}

// The four Examples of § 1 no. 1 as page 19 sets them: a plural head carrying
// the first, then three paragraphs of their own, the third of them marked with
// the asterisk that says it draws on something the reader has not reached yet.
func TestStatementsReadsARun(t *testing.T) {
	in := blocks(
		"### 1. Artinian Modules and Noetherian Modules",
		"**Examples.** — 1) A finite-dimensional vector space over a field K is both Artinian and Noetherian.",
		"2) Let M be an A-module. If there exists an infinite family of nonzero submodules of M, then M is neither Artinian nor Noetherian.",
		`$*3)$ We will see further on that the $\mathbf{Z}$-module $\mathbf{Z}$ is Noetherian and not Artinian.`,
		"4) Let $p$ be a prime number and $M_p$ the $p$-primary component of the torsion module.",
	)
	_, got, err := statements(in, vii, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"alg-viii-s1-n1-exa-1",
		"alg-viii-s1-n1-exa-2",
		"alg-viii-s1-n1-exa-3",
		"alg-viii-s1-n1-exa-4",
	})
	if strings.HasPrefix(got[2].Body, "$*3)$") {
		t.Errorf("the marker is still on the body: %q", got[2].Body)
	}
}

// A run carries on across whatever stands between its members, and a heading
// closes it. No. 7 of § 16 is the case that made this a rule: it prints Remark
// 1, a Definition, "Remarks. — 2)", then "3)" on its own.
func TestStatementsCarriesARunAcrossAStatement(t *testing.T) {
	in := blocks(
		"### 7. The Brauer Group",
		"**Remark 1.** — The mapping is bijective.",
		"**Definition 4.** — A K-algebra A is said to be split by L.",
		"**Remarks.** — 2) The construction does not depend on the choice.",
		"3) The normal basis theorem is a specific case of this.",
		"### 8. Cyclic Algebras",
		"4) This paragraph opens on a number in a new no. and is not a remark.",
	)
	out, got, err := statements(in, corpus.Ref{Book: "alg", Chapter: "VIII", Section: 16}, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"alg-viii-s16-n7-rem-1",
		"alg-viii-s16-def-4",
		"alg-viii-s16-n7-rem-2",
		"alg-viii-s16-n7-rem-3",
	})
	if last := out[len(out)-1].text; !strings.HasPrefix(last, "4) This paragraph") {
		t.Errorf("the run was carried past its heading: %q", last)
	}
}

// A Corollary is numbered under the statement it hangs from, and its label says
// so. A Remark and an Example are numbered inside the no.
func TestStatementsNumbersByScope(t *testing.T) {
	in := blocks(
		"### 2. Wedderburn's Theorem",
		"**Theorem 1.** — Let A be a ring.",
		"**Corollary 1.** — Every simple module is isomorphic to a quotient of A.",
		"**Corollary 2.** — The ring A is left Artinian.",
		"**Proposition 4.** — Let M be an A-module.",
		"**Corollary.** — The module M is semisimple.",
		"**Remark.** — The converse is false.",
	)
	_, got, err := statements(in, corpus.Ref{Book: "alg", Chapter: "VIII", Section: 5}, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"alg-viii-s5-thm-1",
		"alg-viii-s5-thm-1-cor-1",
		"alg-viii-s5-thm-1-cor-2",
		"alg-viii-s5-prop-4",
		"alg-viii-s5-n2-cor-1",
		"alg-viii-s5-n2-rem-1",
	})
}

// Two heads of chapter VIII in every twenty reach assembly without their bold,
// and each one lost it to something printed inside it: an attribution, or the
// star that opens a passage set in small type. Read as prose they end up inside
// the paragraph above with no heading, no label and no tag, and the reference
// graph found 55 references pointing at statements that were not there.
func TestStatementsReadsAHeadThatLostItsBold(t *testing.T) {
	in := blocks(
		"### 1. Simple Rings",
		"**Proposition 1.** — Let A be a ring.",
		"Theorem 1 (Wedderburn). — A ring is simple if and only if it is a matrix ring.",
		"Corollary (Schur’s lemma). — The endomorphism ring of a simple module is a field.",
		`$*$Remark 1. — The morphisms define a complex of K-modules.$*$`,
	)
	out, got, err := statements(in, corpus.Ref{Book: "alg", Chapter: "VIII", Section: 7}, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"alg-viii-s7-prop-1",
		"alg-viii-s7-thm-1",
		"alg-viii-s7-n1-cor-1", // unnumbered, so named by its no. and not by Theorem 1
		"alg-viii-s7-n1-rem-1",
	})
	// The star that opens the small type is kept, because the one that closes it
	// is still at the far end of the sentence and a lone $ is broken maths.
	body := out[len(out)-1].text
	if !strings.HasPrefix(body, "$*$") {
		t.Errorf("the small-type mark was dropped: %q", body)
	}
}

// The dangerous bend marks a passage, and a marked passage often opens on a
// statement. Extraction writes the sign at the head of the paragraph, so it
// stands in front of the head, and reading the paragraph as prose would drop
// four permanent tags out of the French printing: Remarque 2 of § 7 no. 1, two
// of the Examples of § 9 no. 2 and the Remark of § 16 no. 8. A tag that
// disappears is a tag retired, which is the one thing this corpus promises not
// to do.
func TestStatementsReadsAHeadBehindTheDangerousBend(t *testing.T) {
	in := blocks(
		"### 1. Anneaux simples",
		"**Remarques.** — 1) Un anneau simple est artinien à gauche.",
		corpus.Bend+" 2) On dit parfois qu’un anneau A est quasi-simple s’il n’est pas réduit à 0.",
		"### 2. Algèbres galoisiennes",
		corpus.Bend+" **Remarque.** — Soit L une extension galoisienne de degré fini du corps K.",
	)
	out, got, err := statements(in, corpus.Ref{Book: "alg", Chapter: "VIII", Section: 7}, printings["fr"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"alg-viii-s7-n1-rem-1",
		"alg-viii-s7-n1-rem-2",
		"alg-viii-s7-n2-rem-1",
	})
	// The sign is a mark on the whole statement, so it goes back at the front of
	// the body once the head has been read off. It marks the passage either way,
	// and leaving it on the heading would put it in the table of contents.
	for _, s := range got[1:] {
		if !strings.HasPrefix(s.Body, corpus.Bend+" ") {
			t.Errorf("%s lost the sign that marks it: %q", s.Label(), s.Body)
		}
	}
	for _, b := range out {
		if strings.HasPrefix(b.text, "#") && strings.Contains(b.text, corpus.Bend) {
			t.Errorf("the sign ended up in a heading: %q", b.text)
		}
	}
}

// A Corollary standing under nothing is a section boundary read wrong, not a
// statement to guess a number for.
func TestStatementsRefusesAnOrphanCorollary(t *testing.T) {
	in := blocks("### 1. Modules", "**Corollary 1.** — Every module is a module.")
	if _, _, err := statements(in, vii, printings["en"]); err == nil {
		t.Fatal("a Corollary with no parent should be an error")
	}
}

func TestStatementsRefusesTwoStatementsAtOneLabel(t *testing.T) {
	in := blocks(
		"### 1. Modules",
		"**Proposition 3.** — The first one.",
		"**Proposition 3.** — The second one.",
	)
	if _, _, err := statements(in, vii, printings["en"]); err == nil {
		t.Fatal("two statements at one label should be an error")
	}
}

func TestHeadingCarriesTheLabel(t *testing.T) {
	r := corpus.Ref{Book: "alg", Chapter: "VIII", Section: 1, Kind: corpus.KindProposition, Number: 6}
	if got, want := heading(r, r.Label(), printings["en"]),
		"#### Proposition 6 {#alg-viii-s1-prop-6 .statement}"; got != want {
		t.Errorf("heading() = %q, want %q", got, want)
	}
	u := corpus.Ref{Book: "alg", Chapter: "VIII", Section: 1, Kind: corpus.KindRemark, Subsec: 3, Occurrence: 2}
	if got, want := heading(u, u.Label(), printings["en"]),
		"#### Remark {#alg-viii-s1-n3-rem-2 .statement}"; got != want {
		t.Errorf("heading() = %q, want %q", got, want)
	}
}

// The marks the book sets on an exercise are written as the mathematics the
// volume's text layer holds them as, and every one of these is a line out of
// chapter VIII.
func TestItemOpenOnRealMarkers(t *testing.T) {
	for _, s := range []string{
		"1) Let A be a ring and M an A-module.",
		`$\P 15)$ Let K be a commutative field.`,
		`$*19)$ Let A be an Artinian ring.`,
		`$*\P 12)$ Let G be a finite group.`,
		"12)$ Show that the ring A is simple.",
		"**7)** Let E be a finite dimensional s-module.",
		`$\P$ 13) Assume that $k=\mathbf{C}$.`,
		"¶ **9)** Consider the operation of s on its enveloping algebra.",
		"**¶5)** Assume that g is semi-simple.",
		`$7)^*$Let $d_1, . . . , d_l$ be the characteristic degrees.`,
	} {
		if !itemOpen(s) {
			t.Errorf("itemOpen(%q) = false", s)
		}
	}
	for _, s := range []string{
		"a) Let A be a ring.",
		"(i) The module M is simple.",
		"Let A be a ring and 15) is not how a sentence opens.",
		"$$ 5) $$",
	} {
		if itemOpen(s) {
			t.Errorf("itemOpen(%q) = true", s)
		}
	}
}

// Two of the 317 exercises of chapter VIII are set straight after the end of
// the one before them with no break the extractor can see. These are both of
// them, cut out of the pages as they stand.
func TestItemStartOnRunTogetherExercises(t *testing.T) {
	s11 := "12) Let A be a ring. Show that the module is projective (argue as in Exercise 11, c)). 13) Denote by $\\mathscr{C}$ the category of A-modules."
	i, m := itemStart(s11, 13)
	if i < 0 {
		t.Fatal("exercise 13 of § 11 was not found")
	}
	if got := strings.TrimSpace(s11[i+len(m[0]):]); !strings.HasPrefix(got, "Denote by") {
		t.Errorf("exercise 13 begins %q", first(got, 40))
	}
	s16 := "14) Show that the mappings are bijective.$*$ $15)*$a) Let A be a regular integral domain."
	i, m = itemStart(s16, 15)
	if i < 0 {
		t.Fatal("exercise 15 of § 16 was not found")
	}
	if got := strings.TrimSpace(s16[i+markerLen(m):]); !strings.HasPrefix(got, "a) Let A") {
		t.Errorf("exercise 15 begins %q", first(got, 40))
	}
	// The asterisk after the number opens a starred passage and is not a mark
	// on the exercise. The book puts its marks in front of the number, and all
	// nine asterisks of the chapter that mark something are set "$*19)$"; the
	// six set "$15)*$" are the other thing.
	if m[1] != "" {
		t.Errorf("the asterisk after the number of exercise 15 of § 16 was read as a mark: %q", m[1])
	}
}

// The number of a cross-reference is not the number of an exercise, and the two
// look the same. What tells them apart is that one of them is the number the §
// is up to and comes after the end of a sentence.
func TestItemStartIgnoresACrossReference(t *testing.T) {
	for _, s := range []string{
		"Show that the ring is simple (VIII, p. 210, Exercise 13).",
		"By Exercise 13) of § 11, the module M is projective.",
	} {
		if i, _ := itemStart(s, 13); i >= 0 {
			t.Errorf("itemStart(%q) found an exercise at %d", first(s, 40), i)
		}
	}
}

func TestSentenceEnd(t *testing.T) {
	for _, s := range []string{"the module is projective.", "are bijective.$*$", "as in Exercise 11, c))", "M is simple. $"} {
		if !sentenceEnd(s) {
			t.Errorf("sentenceEnd(%q) = false", s)
		}
	}
	for _, s := range []string{"By Exercise", "the ring A", "VIII, p. 210, Exercise"} {
		if sentenceEnd(s) {
			t.Errorf("sentenceEnd(%q) = true", s)
		}
	}
}

func TestExercisesReadsTheMarks(t *testing.T) {
	in := blocks(
		"### Exercises",
		"1) Let A be a ring. Show that A is simple.",
		"a) The first part. b) The second part.",
		`$\P 2)$ Let K be a commutative field.`,
		`$*3)$ Let G be a finite group.`,
	)
	got, err := exercises(in, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d exercises, want 3", len(got))
	}
	if !strings.Contains(got[0].Body, "a) The first part") {
		t.Errorf("the lettered parts left exercise 1: %q", got[0].Body)
	}
	if got[1].Meta.Starred != true || got[1].Meta.Supplementary {
		t.Errorf("exercise 2 carries the pilcrow alone, got starred=%v supplementary=%v",
			got[1].Meta.Starred, got[1].Meta.Supplementary)
	}
	if got[2].Meta.Supplementary != true || got[2].Meta.Starred {
		t.Errorf("exercise 3 carries the asterisk alone, got starred=%v supplementary=%v",
			got[2].Meta.Starred, got[2].Meta.Supplementary)
	}
	if got[0].Meta.PDFPage != 42 || got[0].Meta.BookPage != "A VIII.7" {
		t.Errorf("exercise 1 is on %d, %q", got[0].Meta.PDFPage, got[0].Meta.BookPage)
	}
}

// What stands between the heading and exercise 1 is the preamble, and it stays
// in the section rather than going into the first exercise. Lie 7 to 9 prints
// one over most of its runs and Algebra VIII prints none.
func TestThePreambleIsNotPartOfExerciseOne(t *testing.T) {
	in := blocks("### Exercises", "The notations are those of nos. 1, 2, 3 of § 4.", "1) Let A be a ring.")
	got, err := exercises(in, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("read %d exercises, want 1", len(got))
	}
	if strings.Contains(got[0].Body, "The notations") {
		t.Errorf("the preamble went into exercise 1: %q", got[0].Body)
	}
	out := cutExercises(in, 1, false, printings["en"])
	if len(out) != 3 || !strings.HasPrefix(out[1].text, "The notations") {
		t.Errorf("the preamble did not stay in the section: %v", texts(out))
	}
}

// A run the reader never gets past the preamble of is a run whose first marker
// was misread, and a wrong cut is not something to write out. The heading alone
// satisfies the table of contents, so the count is what has to be checked.
func TestARunWithNoExerciseReadOutOfItIsRefused(t *testing.T) {
	p := Piece{Section: corpus.Section{Number: 1, Exercises: &corpus.Locator{PDFPage: 60}}, HasExercise: true}
	if err := p.Verify(); err == nil {
		t.Fatal("a § whose exercises were all read as preamble should be an error")
	}
}

func TestAnchorExercises(t *testing.T) {
	in := blocks("### Exercises", "1) Let A be a ring.")
	out, found := anchorExercises(in, vii, printings["en"])
	if !found {
		t.Fatal("the exercises heading was not found")
	}
	if got, want := out[0].text, "### Exercises {#alg-viii-s1-exercises}"; got != want {
		t.Errorf("anchor = %q, want %q", got, want)
	}
}

// The contents flattens the mathematics out of a subsection title and the page
// does not, so the title is read off the page. This is no. 6 of § 11 as both
// print it.
func TestSubsectionsTakeTheirTitleFromThePage(t *testing.T) {
	got := subsections([]part{{
		page:  231,
		label: "A VIII.213",
		body:  "### 6. The Grothendieck Group $R_K(A)$\n\nLet K be a commutative field.",
	}})
	if len(got) != 1 {
		t.Fatalf("got %d subsections, want 1", len(got))
	}
	if got[0].Number != 6 || got[0].Title != `The Grothendieck Group $R_K(A)$` {
		t.Errorf("subsection = %+v", got[0])
	}
	if got[0].PDFPage != 231 || got[0].Page != 213 {
		t.Errorf("subsection pages = %d, %d", got[0].PDFPage, got[0].Page)
	}
}

func TestVerifyChecksTheContents(t *testing.T) {
	p := Piece{
		Section: corpus.Section{Number: 1, Title: "Modules", PDFPage: 18, Subsections: []corpus.Subsection{
			{Number: 1, Title: "Modules", PDFPage: 18},
			{Number: 2, Title: "Rings", PDFPage: 21},
		}},
		Subsections: []corpus.Subsection{
			{Number: 1, Title: "Modules", PDFPage: 18},
			{Number: 2, Title: "Rings", PDFPage: 21},
		},
	}
	if err := p.Verify(); err != nil {
		t.Fatalf("a piece that agrees with the contents should verify: %v", err)
	}
	short := p
	short.Subsections = p.Subsections[:1]
	if err := short.Verify(); err == nil {
		t.Error("a missing no. should be an error")
	}
	moved := p
	moved.Subsections = []corpus.Subsection{
		{Number: 1, Title: "Modules", PDFPage: 18},
		{Number: 2, Title: "Rings", PDFPage: 22},
	}
	if err := moved.Verify(); err == nil {
		t.Error("a no. on the wrong page should be an error")
	}
}

// The text layer sets the pilcrow and the number of an exercise as mathematics,
// and the run it puts them in does not always stop where the number does, so
// the dollar that closes it is a few letters into the prose. Taking only the
// marker off leaves a span that nothing closes and the rest of the exercise
// reads as a formula, which is what M01 was reporting against four exercise
// files of chapter VIII. The text here is invented; the shape is the volume's.
func TestAfterMarker(t *testing.T) {
	cases := []struct {
		name   string
		marker string
		rest   string
		want   string
	}{
		{
			"the marker closes its own span",
			`$\P 18)$ `,
			"A group G is called locally finite.",
			"A group G is called locally finite.",
		},
		{
			// exNumRE stops at the space after the parenthesis, so the letter
			// the closing dollar sits behind is the first thing in the rest.
			"the span runs on into the prose",
			`$\P 18) `,
			`A$ group G is called locally finite.`,
			"A group G is called locally finite.",
		},
		{
			"no mathematics in the marker at all",
			"18) ",
			"A group G is called locally finite.",
			"A group G is called locally finite.",
		},
		{
			"the prose after it has mathematics of its own",
			`$\P 18) `,
			`A$ group $G$ is called locally finite.`,
			`A group $G$ is called locally finite.`,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := afterMarker(c.marker, c.rest); got != c.want {
				t.Errorf("afterMarker(%q, %q)\n = %q\nwant %q", c.marker, c.rest, got, c.want)
			}
		})
	}
}
