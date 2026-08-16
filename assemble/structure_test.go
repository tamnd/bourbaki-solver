package assemble

import (
	"slices"
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

// No. 3 of § 1 of chapter II of Théories spectrales, as pages 227 and 228 set
// it: an unnumbered Remarque, then a run of Remarques opening at 1). Named by
// where it stands, the unnumbered one would be Remark 1 and so would the first
// member of the run, and the chapter stopped assembling on the pair. The number
// in the run is the book's own, so it keeps it and the unnumbered one steps over
// it.
func TestStatementsLeavesTheBookItsNumber(t *testing.T) {
	in := blocks(
		"### 3. Le théorème de Plancherel",
		"**Remarque.** — Soit $a$ un nombre réel $>0$.",
		"**Remarques.** — 1) Certaines des formules concernant la transformation de Fourier s’étendent.",
		"2) La transformation de Fourier est un isomorphisme d’espaces hilbertiens.",
	)
	_, got, err := statements(in, corpus.Ref{Book: "ts", Chapter: "II", Section: 1}, printings["fr"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"ts-ii-s1-n3-rem-3",
		"ts-ii-s1-n3-rem-1",
		"ts-ii-s1-n3-rem-2",
	})
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
	if got, want := heading(r, r.Label(), "", printings["en"]),
		"#### Proposition 6 {#alg-viii-s1-prop-6 .statement}"; got != want {
		t.Errorf("heading() = %q, want %q", got, want)
	}
	u := corpus.Ref{Book: "alg", Chapter: "VIII", Section: 1, Kind: corpus.KindRemark, Subsec: 3, Occurrence: 2}
	if got, want := heading(u, u.Label(), "", printings["en"]),
		"#### Remark {#alg-viii-s1-n3-rem-2 .statement}"; got != want {
		t.Errorf("heading() = %q, want %q", got, want)
	}
}

// The name the printing gives a result is kept, and the label is not touched by
// it: Theorem 1 of § 2 of chapter VIII is one statement of the Éléments whether
// the printing in hand calls it Wedderburn's or not, so both printings label it
// alg-viii-s2-thm-1 and both carry the same tag.
func TestHeadingKeepsTheNameThePrintingGives(t *testing.T) {
	r := corpus.Ref{Book: "alg", Chapter: "VIII", Section: 2, Kind: corpus.KindTheorem, Number: 1}
	if got, want := heading(r, r.Label(), "Wedderburn", printings["en"]),
		"#### Theorem 1 (Wedderburn) {#alg-viii-s2-thm-1 .statement}"; got != want {
		t.Errorf("heading() = %q, want %q", got, want)
	}
	if got, want := heading(r, r.Label(), "Wedderburn", printings["fr"]),
		"#### Théorème 1 (Wedderburn) {#alg-viii-s2-thm-1 .statement}"; got != want {
		t.Errorf("heading() = %q, want %q", got, want)
	}
}

// And the name is read off the head, footnote mark and all. Page 382 of
// Topologie algébrique IV states Shelah's theorem with a note on the name, and
// the note is printed at the foot of that page: dropping the mark with the head
// left the note pointing at nothing, which is what assembly of that chapter
// stopped on.
func TestANameCarriesItsFootnoteMark(t *testing.T) {
	text := `Théorème 2 (Shelah[^2]). — Soit X un espace polonais connexe et localement connexe par arcs.`
	occ := map[corpus.Ref]int{}
	id := corpus.Ref{Book: "ta", Chapter: "IV", Section: 2}
	r, name, body, ok, err := statementAt(text, id, 1, corpus.Ref{}, corpus.Ref{}, 0, occ, nil, printings["fr"])
	if err != nil || !ok {
		t.Fatalf("statementAt() = %v, %v, want a Théorème", ok, err)
	}
	if r.Kind != corpus.KindTheorem || r.Number != 2 {
		t.Errorf("statementAt() read %s, want Théorème 2", r.Label())
	}
	if want := "Shelah[^2]"; name != want {
		t.Errorf("name = %q, want %q", name, want)
	}
	if want := "Soit X un espace polonais connexe et localement connexe par arcs."; body != want {
		t.Errorf("body = %q, want %q", body, want)
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
	if star, _ := marksOf(m[1]); star != "" {
		t.Errorf("the asterisk after the number of exercise 15 of § 16 was read as a mark: %q", m[1])
	}
}

// Theory of Sets sets the star that brackets a passage in small type outside
// the mathematics, so it arrives as a bullet or as an escaped star, and it sets
// the pilcrow before the star as well as after it.
func TestItemStartReadsTheMarksTheoryOfSetsPrints(t *testing.T) {
	for _, c := range []struct {
		text          string
		n             int
		star, pilcrow bool
		body          string
	}{
		{"* 4. Let E be an ordered set, and let $(E_\\iota)$ be the partition of E.", 4, true, false, "Let E be an"},
		{"\\* 9. If E is a lattice, prove that", 9, true, false, "If E is a"},
		{"¶ * 18.  Let A be a set with at least three elements.", 18, true, true, "Let A be a"},
		{"¶ 17.  A lattice E which has a least element.", 17, false, true, "A lattice E"},
		{"19. An ordered set E is said to be *without gaps*.", 19, false, false, "An ordered set"},
	} {
		i, m := itemStart(c.text, c.n)
		if i < 0 {
			t.Errorf("exercise %d was not found in %q", c.n, c.text)
			continue
		}
		star, pilcrow := marksOf(m[1])
		if (star != "") != c.star || (pilcrow != "") != c.pilcrow {
			t.Errorf("%q: star %q pilcrow %q, want star %v pilcrow %v", c.text, star, pilcrow, c.star, c.pilcrow)
		}
		if got := strings.TrimSpace(c.text[i+markerLen(m):]); !strings.HasPrefix(got, c.body) {
			t.Errorf("%q: the exercise begins %q", c.text, first(got, 40))
		}
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
	for _, s := range []string{"By Exercise", "the ring A", "VIII, p. 210, Exercise",
		"with the preorder relation (Chapter II, § 6, no.", "see VIII, p.", "cf.", "as in fig."} {
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

// Theory of Sets closes the number of an exercise with a full stop where every
// other volume closes it with a parenthesis, and it letters the parts "(a)"
// rather than "a)", so the two shapes never meet on one page. Read the other
// way only, every § of the volume came back with a preamble and no exercises.
func TestExercisesReadsTheFullStopTheoryOfSetsPrints(t *testing.T) {
	in := blocks(
		"### Exercises",
		`1. Let $\mathscr{T}$ be a theory with no specific signs.`,
		"(a) Show that the relation is false. (b) Deduce that the theory is contradictory.",
		`$\P 2.$ Let $A$ be a term.`,
		`$*3.$ Let $R$ be a relation.`,
	)
	got, err := exercises(in, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d exercises, want 3", len(got))
	}
	if !strings.Contains(got[0].Body, "(a) Show that") {
		t.Errorf("the lettered parts left exercise 1: %q", got[0].Body)
	}
	if !got[1].Meta.Starred || got[1].Meta.Supplementary {
		t.Errorf("exercise 2 carries the pilcrow alone, got starred=%v supplementary=%v",
			got[1].Meta.Starred, got[1].Meta.Supplementary)
	}
	if !got[2].Meta.Supplementary || got[2].Meta.Starred {
		t.Errorf("exercise 3 carries the asterisk alone, got starred=%v supplementary=%v",
			got[2].Meta.Supplementary, got[2].Meta.Starred)
	}
}

// The full stop is only a marker where the number is the one the run is up to,
// which is what keeps a sentence opening on a numeral out of the run. No
// paragraph of the assembled corpus opens on a number and a full stop, and the
// count was taken before the shape was allowed.
func TestAParagraphOpeningOnANumberIsNotAnExercise(t *testing.T) {
	in := blocks(
		"### Exercises",
		"1. Let $A$ be a set.",
		"3. is the number of elements, and 2. of them are here.",
		"2. Let $B$ be a set.",
	)
	got, err := exercises(in, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d exercises, want 2", len(got))
	}
	if !strings.Contains(got[0].Body, "3. is the number of elements") {
		t.Errorf("the paragraph opening on 3. was taken for an exercise: %q", got[0].Body)
	}
}

// Theory of Sets sets its heads in small capitals and follows them with nothing:
// page 46 prints "THEOREM 1.  x = x." and page 104 prints "PROPOSITION 4.  Let
// (X_i) be a family of sets", and there is no dash on either page. A reading
// writes the small capitals as bold and the word in ordinary capitals, so what
// arrives is the shape below. Read as prose it cost the volume 101 of its 257
// heads, and § 5 of chapter I came out with none of its three theorems.
func TestStatementsReadsAHeadWithNoDashAfterIt(t *testing.T) {
	in := blocks(
		"### 1. The Axioms",
		"**Theorem 1.** $x = x$.",
		"**Theorem 2** (Zermelo). *Every set* E *can be well-ordered.*",
		"**Proposition 4.** *Let* $(X_\\iota)$ *be a family of sets.*",
		"**Remark.** There exists no set of which every object is an element.",
	)
	out, got, err := statements(in, corpus.Ref{Book: "ens", Chapter: "I", Section: 5}, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"ens-i-s5-thm-1",
		"ens-i-s5-thm-2",
		"ens-i-s5-prop-4",
		"ens-i-s5-n1-rem-1",
	})
	if !strings.Contains(strings.Join(texts(out), "\n"), "Theorem 2 (Zermelo)") {
		t.Errorf("the name outside the bold was dropped: %v", texts(out))
	}
	if !strings.HasPrefix(got[0].Body, "$x = x$") {
		t.Errorf("the head was not taken off the body: %q", got[0].Body)
	}
}

// The dash is what the other bold printing leans on, and this volume has none,
// so the bold is what the branch leans on instead. A sentence of the corpus
// opens on a bold word often enough to matter, and none of them closes it with a
// period inside the bold.
func TestASentenceOpeningOnABoldWordIsNotAHead(t *testing.T) {
	in := blocks(
		"### 1. The Axioms",
		"**Theorem 1.** $x = x$.",
		"**Proposition** 5 of § 2 is proved the same way.",
		"**Remarks** are gathered at the end of the no.",
	)
	out, got, err := statements(in, corpus.Ref{Book: "ens", Chapter: "I", Section: 5}, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{"ens-i-s5-thm-1"})
	for _, want := range []string{"**Proposition** 5 of § 2 is proved the same way.", "**Remarks** are gathered at the end of the no."} {
		if !slices.Contains(texts(out), want) {
			t.Errorf("the sentence was read as a head and taken apart: %q", want)
		}
	}
}

// Theory of Sets sets the kind in the plural alone on a line and puts the
// members under it, each opening on its own number in brackets.
func TestARunOpensOnItsKindAloneOnALine(t *testing.T) {
	in := blocks(
		"### 3. Order Relations",
		"*Examples*",
		"(1) The relations of equality and inclusion are not order relations.",
		"(2) Let E be a set such that $x \\in E$.",
		"*Remarks*",
		"(1) The empty set satisfies this condition.",
	)
	out, got, err := statements(in, corpus.Ref{Book: "ens", Chapter: "III", Section: 1}, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"ens-iii-s1-n3-exa-1",
		"ens-iii-s1-n3-exa-2",
		"ens-iii-s1-n3-rem-1",
	})
	if !strings.HasPrefix(got[0].Body, "The relations of equality") {
		t.Errorf("the number was not taken off the body: %q", got[0].Body)
	}
	if !slices.Contains(texts(out), "*Examples*") {
		t.Errorf("the lead carries no statement, so it stays where it is: %v", texts(out))
	}
}

// Page 16 of Theory of Sets is read with (2) on the line under (1), so the two
// members arrive as one block and the second would go into the body of the
// first.
func TestARunSplitsTwoMembersThePageRanTogether(t *testing.T) {
	in := blocks(
		"### 1. Terms and Relations",
		"*Examples*",
		"(1) The assembly $\\vee 1$ is represented by $\\Rightarrow$.\n(2) The following symbols represent assemblies :",
		"(3) The sign $\\square$ is not a letter.",
	)
	_, got, err := statements(in, corpus.Ref{Book: "ens", Chapter: "I", Section: 1}, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"ens-i-s1-n1-exa-1",
		"ens-i-s1-n1-exa-2",
		"ens-i-s1-n1-exa-3",
	})
	if strings.Contains(got[0].Body, "The following symbols") {
		t.Errorf("the second member went into the body of the first: %q", got[0].Body)
	}
	if !strings.HasPrefix(got[1].Body, "The following symbols") {
		t.Errorf("the second member was not read: %q", got[1].Body)
	}
}

// The second run of a kind in one no. is left as prose. Numbering it on from
// the first would put a number on a statement that the book does not give it,
// and its own numbers are already spoken for. See walk.
func TestASecondRunOfOneKindInANoIsLeftAlone(t *testing.T) {
	in := blocks(
		"### 1. Definition of an Order Relation",
		"*Examples*",
		"(1) The relation $x = x$ is not collectivizing.",
		"(2) An order relation on a set E.",
		"An *ordering* on a set E is a correspondence.",
		"*Examples*",
		"(1) The relations of equality and inclusion.",
		"(2) The order relation induced on E.",
		"(3) The relation g extends f.",
	)
	out, got, err := statements(in, corpus.Ref{Book: "ens", Chapter: "III", Section: 1}, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{"ens-iii-s1-n1-exa-1", "ens-iii-s1-n1-exa-2"})
	for _, want := range []string{
		"(1) The relations of equality and inclusion.",
		"(2) The order relation induced on E.",
		"(3) The relation g extends f.",
	} {
		if !slices.Contains(texts(out), want) {
			t.Errorf("the second run was taken apart: %q", want)
		}
	}
}
