package assemble

import (
	"slices"
	"strconv"
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
	r, name, body, ok, err := statementAt(text, id, 1, corpus.Ref{}, corpus.Ref{}, 0, occ,
		map[corpus.Ref]int{}, map[corpus.Ref]int{}, nil, printings["fr"])
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
		// The section sign standing where the pilcrow is printed. All four are
		// cut out of the pages as they stand.
		"**§ 4.** Let $X$ be a set and $c$ a cardinal.",
		"§ 11. A topological space is said to be solvable.",
		"§ 9) Soient E un espace vectoriel de dimension $n$.",
		"**§ 27.** (a) Let A be a commutative ring.",
		// The pilcrow read as a letter. All four are cut out of the pages.
		"**T 17) Let A be the set of increasing maps.",
		"T 6) On suppose G simplement connexe.",
		"**Q 11) Soit (W, S) un système de Coxeter.",
		`**Π 13)** \* We keep the general hypothesis.`,
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
		// A cross reference to a subsection, set on a line of its own. The digit
		// after the full stop is what keeps it out, and chapters I and III of
		// Lie hold six of them.
		"§ 4.3",
		"§ 1.10",
		// A label is a capital against its number with no space, and the corpus
		// writes 307 of them. A is a case of an argument and not a mark, which
		// is why it is not one of the four letters read as a pilcrow.
		"C1. The space is Hausdorff.",
		"S2. Let A be a ring.",
		"E1) The module is free.",
		"A 2) *Cas général* ($p = 1$).",
	} {
		if itemOpen(s) {
			t.Errorf("itemOpen(%q) = true", s)
		}
	}
}

// The section sign is a misread pilcrow and has to mark what the pilcrow marks,
// or the exercise it opens comes out unstarred. § 7 of chapter I of Algebra is
// the case that found it: nothing could read "**§ 4.**", so the § stopped at the
// third of the forty exercises the volume prints and the other 37 were appended
// to the body of the third.
func TestSectionSignIsAPilcrow(t *testing.T) {
	m := exNumRE.FindStringSubmatch("**§ 4.** Let $X$ be a set and $c$ a cardinal.")
	if m == nil {
		t.Fatal(`"**§ 4.**" was not read as a marker`)
	}
	if m[2] != "4" {
		t.Errorf("the marker carries the number %q", m[2])
	}
	star, pilcrow := marksOf(m[1])
	if pilcrow == "" {
		t.Errorf("marksOf(%q) read no pilcrow off the marker", m[1])
	}
	if star != "" {
		t.Errorf("marksOf(%q) read a star %q that is not printed", m[1], star)
	}
}

// The heading of a § spells a section sign and a number as well, so the sign is
// taken only where the number is the one the § is up to. This is the guard that
// lets the sign be read at all.
func TestSectionSignNeedsTheNumberTheSectionIsUpTo(t *testing.T) {
	const line = "§ 11. A topological space is said to be solvable."
	if i, _ := itemStart(line, 11); i != 0 {
		t.Errorf("exercise 11 was not found at the head of the block, i = %d", i)
	}
	if i, _ := itemStart(line, 4); i >= 0 {
		t.Errorf("the line was read as exercise 4 at %d", i)
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
	for _, s := range []string{"the module is projective.", "are bijective.$*$", "as in Exercise 11, c))", "M is simple. $",
		"les deux structures sont distinctes.\\*", "the two structures are distinct.\\*\n"} {
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

// A starred exercise closes on a star and the exercises printed after it are
// still exercises. Page 366 of Algebre 1 a 3 in French is the case: SS 3 of
// chapter II opens its second exercise with a star and closes it with one at the
// end of part b, and its third and fourth are set on the lines under that with
// no blank line between them, so all three land in one block. The closing star
// is written escaped, since Markdown reads a bare one as emphasis, and the
// backslash of it used to be left standing where the number was looked at, so
// the sentence did not read as ended and exercises 3 and 4 were swallowed into
// the body of exercise 2.
func TestExercisesFindsTheOnesAfterAStarredOne(t *testing.T) {
	in := blocks(
		"### Exercises",
		"1) Let A be a ring, E a right A-module, F a left A-module.",
		`\*2) Consider the field C of complex numbers as a vector space over R.
a) Show that the canonical map is not injective.
b) Show that the two C-module structures are distinct.\*
3) a) Let F be a left A-module. Show that the canonical map is surjective.
b) Give an example of a commutative ring A and an ideal m of A.
4) Let E be a right A-module and F a free left A-module.`,
	)
	got, err := exercises(in, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("got %d exercises, want 4", len(got))
	}
	if !got[1].Meta.Supplementary {
		t.Error("exercise 2 opens with a star and is supplementary")
	}
	if got[2].Meta.Supplementary || got[3].Meta.Supplementary {
		t.Error("the star closes on exercise 2 and does not carry on to 3 and 4")
	}
	if strings.Contains(got[1].Body, "surjective") {
		t.Errorf("exercise 3 was swallowed into exercise 2: %q", got[1].Body)
	}
	if !strings.Contains(got[2].Body, "b) Give an example") {
		t.Errorf("the lettered parts left exercise 3: %q", got[2].Body)
	}
	if !strings.Contains(got[3].Body, "free left A-module") {
		t.Errorf("exercise 4 is missing its text: %q", got[3].Body)
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

// The preamble and the first exercise land in the one block when the page was
// read with no blank line between them, which is how the French Lie chapter I
// prints the exercises of its § 4. The full stop that ends the preamble then has
// a newline after it, and the reader used to trim spaces off the end and nothing
// else, so the sentence did not read as ended and exercise 1 was never found.
// Losing 1 loses the whole run, since the reader looks for one number and one
// only, and that § prints 27 exercises.
func TestThePreambleAndExerciseOneInOneBlock(t *testing.T) {
	in := blocks("### Exercises",
		"Les conventions du § 4 restent valables, sauf mention contraire.\n"+
			"1) Soient g une algèbre de Lie nilpotente, p le plus petit entier tel que $C^p g = 0$.\n"+
			"2) Soit g un produit semi-direct d’une algèbre h de dimension 1.")
	got, err := exercises(in, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("read %d exercises, want 2", len(got))
	}
	if strings.Contains(got[0].Body, "Les conventions") {
		t.Errorf("the preamble went into exercise 1: %q", got[0].Body)
	}
	if !strings.Contains(got[1].Body, "produit semi-direct") {
		t.Errorf("exercise 2 is not the second exercise: %q", got[1].Body)
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
	// A heading set at the foot of the page before the one the contents gives.
	// See the note in Verify.
	early := p
	early.Subsections = []corpus.Subsection{
		{Number: 1, Title: "Modules", PDFPage: 18},
		{Number: 2, Title: "Rings", PDFPage: 20},
	}
	if err := early.Verify(); err != nil {
		t.Errorf("a no. whose heading is a page before the contents gives should verify: %v", err)
	}
	earlier := p
	earlier.Subsections = []corpus.Subsection{
		{Number: 1, Title: "Modules", PDFPage: 18},
		{Number: 2, Title: "Rings", PDFPage: 19},
	}
	if err := earlier.Verify(); err == nil {
		t.Error("a no. two pages before the contents gives should be an error")
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

// Where a no. prints two runs of one kind the numbers belong to the last, which
// is the run the volume cites by, and the earlier one is left as the prose it is
// printed as. See walk.
func TestTheLastRunOfAKindInANoCarriesTheNumbering(t *testing.T) {
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
	same(t, labels(got), []string{"ens-iii-s1-n1-exa-1", "ens-iii-s1-n1-exa-2", "ens-iii-s1-n1-exa-3"})
	if want := "(3) The relation g extends f."; slices.Contains(texts(out), want) {
		t.Errorf("the last run was left as prose: %q", want)
	}
	for _, want := range []string{
		"(1) The relation $x = x$ is not collectivizing.",
		"(2) An order relation on a set E.",
	} {
		if !slices.Contains(texts(out), want) {
			t.Errorf("the earlier run was taken apart: %q", want)
		}
	}
}

// The same rule where the printing sets the lead with its first member on the
// line instead of the word alone. No. 3 of § 1 of chapter I of Topology I to IV
// prints "Examples. 1)" twice on page 27, once for fundamental systems of
// neighbourhoods and once for bases of a topology, and read without the rule
// both run number from 1 and the § has two statements called Example 1, which is
// what stopped that volume assembling at all. See runLead.
func TestTheLastInlineRunOfAKindInANoCarriesTheNumbering(t *testing.T) {
	in := blocks(
		"### 3. Fundamental Systems of Neighbourhoods; Bases of a Topology",
		"DEFINITION 5. In a topological space X, a fundamental system of neighbourhoods of a point x is any set of neighbourhoods.",
		"Examples. 1) In a discrete space (no. 1) the set $\\{x\\}$ alone constitutes a fundamental system of neighbourhoods of the point x.",
		"2) In a topological space X the set of open neighbourhoods of a point x is a fundamental system of neighbourhoods of x.",
		"DEFINITION 6. A base of the topology of a topological space X is any set of open sets.",
		"Examples. 1) The discrete topology has as a base the set of subsets of X which consist of a single point.",
		"2) The set of open intervals is a base of the topology of the rational line.",
	)
	out, got, err := statements(in, corpus.Ref{Book: "top-i", Chapter: "I", Section: 1}, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"top-i-i-s1-def-5",
		"top-i-i-s1-def-6",
		"top-i-i-s1-n3-exa-1",
		"top-i-i-s1-n3-exa-2",
	})
	if !strings.HasPrefix(got[2].Body, "The discrete topology") {
		t.Errorf("the numbers went to the first run, not the last: %q", got[2].Body)
	}
	for _, want := range []string{in[2].text, in[3].text} {
		if !slices.Contains(texts(out), want) {
			t.Errorf("the earlier run was taken apart: %q", want)
		}
	}
}

// A no. that prints one inline lead is read exactly as it was before the rule
// above, which is what keeps Lie 7 to 9 where it is: 24 pages of that volume
// open a run this way and every one of them is the only run of its kind in its
// no. The lead is not a head of nothing, so the block it stands on is the first
// member and not prose.
func TestOneInlineRunIsStillNumberedFromItsLead(t *testing.T) {
	in := blocks(
		"### 2. Bases of a Root System",
		"Remarks. 1) The set of roots of a base is linearly independent.",
		"2) A base is contained in a chamber.",
	)
	out, got, err := statements(in, corpus.Ref{Book: "lie", Chapter: "VI", Section: 1}, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{"lie-vi-s1-n2-rem-1", "lie-vi-s1-n2-rem-2"})
	if slices.Contains(texts(out), in[1].text) {
		t.Errorf("the lead was passed through as prose: %v", texts(out))
	}
}

// No. 4 of § 1 of chapter III announces its run in a sentence instead of setting
// the word alone on a line, and chapter IV cites the third member of it. See
// enRunKinds.
func TestARunOpensOnAKindThatAnnouncesItself(t *testing.T) {
	in := blocks(
		"### 4. Ordered Subsets. Product of Ordered Sets",
		"*Examples*. The relations induced by the inclusion relation $X \\subset Y$ on various sets of subsets are of considerable importance. Here are some examples :",
		"(1) Let E, F be two sets, and let $\\Phi(E, F)$ be the set of all mappings.",
		"(2) For each partition of a set E, let $\\tilde{\\varpi}$ be the graph.",
		"(3) Let E be a set and let $\\Omega$ be the set of graphs of preorderings on E.",
	)
	out, got, err := statements(in, corpus.Ref{Book: "ens", Chapter: "III", Section: 1}, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"ens-iii-s1-n4-exa-1",
		"ens-iii-s1-n4-exa-2",
		"ens-iii-s1-n4-exa-3",
	})
	if !slices.Contains(texts(out), in[1].text) {
		t.Errorf("the lead carries no statement, so it stays where it is: %v", texts(out))
	}
}

// The colon is what tells the lead from a paragraph that opens on the word and
// goes on to say something, which is not a head and states nothing.
func TestAParagraphOpeningOnTheWordExamplesIsNotARun(t *testing.T) {
	in := blocks(
		"### 4. Ordered Subsets. Product of Ordered Sets",
		"*Examples* of this are given in the exercises. The reader will supply them.",
		"(1) Let E, F be two sets, and let $\\Phi(E, F)$ be the set of all mappings.",
	)
	_, got, err := statements(in, corpus.Ref{Book: "ens", Chapter: "III", Section: 1}, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("a sentence was read as the head of a run: %v", labels(got))
	}
}

// A proof taken up again puts the statement it proves back in force, so the
// corollaries printed after it hang from that statement and not from the last
// lemma the proof needed. See resumed.
func TestAProofTakenUpAgainCarriesTheCorollaries(t *testing.T) {
	in := blocks(
		"### 3. Properties of Infinite Cardinals",
		"**Theorem 2.** *Let* $\\mathfrak{a}$ *be an infinite cardinal.*",
		"**Lemma 1.** *Every infinite set contains a countable subset.*",
		"The proof is by Zorn's lemma.",
		"**Lemma 2.** *There is a bijection of* D *onto* D $\\times$ D.",
		"¶ We come now to the proof of Theorem 2. Let E be a set.",
		"**Corollary 1.** $\\mathfrak{a}^n = \\mathfrak{a}$ *for every integer* $n \\geqslant 1$.",
		"**Corollary 2.** *The product of a finite family of non-zero cardinals.*",
	)
	_, got, err := statements(in, corpus.Ref{Book: "ens", Chapter: "III", Section: 6}, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"ens-iii-s6-thm-2",
		"ens-iii-s6-lem-1",
		"ens-iii-s6-lem-2",
		"ens-iii-s6-thm-2-cor-1",
		"ens-iii-s6-thm-2-cor-2",
	})
}

// A paragraph naming a statement the § has not printed yet hands the reader back
// to nothing, and the corollary under it stays where the printing put it.
func TestAProofNotYetReachedLeavesTheParentAlone(t *testing.T) {
	in := blocks(
		"### 1. Compact Linear Mappings",
		"**Lemma 1.** *Let* E *be a locally convex space.*",
		"We shall complete the proof of Theorem 9 in the next no.",
		"**Corollary 1.** *Keep the hypotheses of Lemma 1.*",
	)
	_, got, err := statements(in, corpus.Ref{Book: "ts", Chapter: "III", Section: 1}, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{"ts-iii-s1-lem-1", "ts-iii-s1-lem-1-cor-1"})
}

// A pilcrow that came back from the page image as a section sign still marks the
// exercise it marks. Page 447 of Algebra VIII is where this cost something: the
// marker on exercise 22 was read that way, § 21 stopped at 21, and the nine the
// volume prints after it were lost.
func TestASectionSignIsReadAsAMisreadPilcrow(t *testing.T) {
	const line = `$ \S 22) $ Let $ n $ be a natural number.`
	m := exNumRE.FindStringSubmatch(line)
	if m == nil {
		t.Fatal("the marker was not read")
	}
	if m[2] != "22" {
		t.Errorf("number = %q, want 22", m[2])
	}
	star, pilcrow := marksOf(m[1])
	if pilcrow == "" {
		t.Error("the exercise did not come out marked as one of the harder ones")
	}
	if star != "" {
		t.Errorf("star = %q, want none", star)
	}
}

// A dagger is the same swap, and page 95 of Algebra 4 to 7 is where it cost
// something: the printing opens exercises 1, 3 and 4 of § 1 with a pilcrow, the
// reading gave all three as a dagger, and the § came out as a preamble with no
// exercises in it at all, which stopped the whole of chapter IV assembling.
func TestADaggerIsReadAsAMisreadPilcrow(t *testing.T) {
	for _, line := range []string{
		`† 1) \* When M is a subgroup of the additive group R of real numbers,`,
		`†24) Soit $ i : A \to I $ une enveloppe injective.`,
	} {
		m := exNumRE.FindStringSubmatch(line)
		if m == nil {
			t.Errorf("%q: the marker was not read", line)
			continue
		}
		_, pilcrow := marksOf(m[1])
		if pilcrow == "" {
			t.Errorf("%q: the exercise did not come out marked as one of the harder ones", line)
		}
	}
}

// And a dagger anywhere else is a footnote mark, which is what the other 66 of
// the corpus's 130 daggers are. A footnote mark sits inside a sentence or hard
// against the word it hangs off, never in front of a number and a bracket.
func TestADaggerInAFootnoteMarkOpensNothing(t *testing.T) {
	for _, line := range []string{
		`to confusion†) will denote the set of elements $ x $`,
		`† For more details on this exercise, see Bourbaki.`,
		`the length of $m.$†`,
	} {
		if exNumRE.MatchString(line) {
			t.Errorf("%q was read as an exercise marker", line)
		}
	}
}

// And a section sign in front of a number is otherwise a citation, which is what
// 1209 lines of the corpus use it for. None of these opens an exercise.
func TestASectionSignInACitationOpensNothing(t *testing.T) {
	for _, line := range []string{
		`$\S 8$, p. 639, Exercise 5`,
		`(cf. III, $\S 1$, no. 2)`,
		`\S 22, and the rest follows`,
	} {
		if exNumRE.MatchString(line) {
			t.Errorf("%q was read as an exercise marker", line)
		}
	}
}

// The French printing sets Proposition, Théorème, Définition and Corollaire in
// small capitals, and a reading of the page image does not always keep them.
// 1584 heads across 18 French volumes have come back with the bold gone, and
// left out they are prose with no tag and nothing can cite them. § 11 of the
// French Algebra VIII is where it showed: six of its § were re-read from the
// image and the volume came out 29 statements short.
//
// The em dash is what makes this safe to read. The printing puts one after the
// number of a result and after nothing else, so a paragraph of prose does not
// arrive at this shape.
func TestStatementsReadsAFrenchHeadWithNoBoldOnIt(t *testing.T) {
	in := blocks(
		"### 1. Le groupe de Grothendieck",
		"Proposition 1. — Soient A un anneau et E un A-module de longueur finie.",
		"Définition 2. — On appelle groupe de Grothendieck de A le groupe abélien.",
		"Corollaire 1. — Le groupe $ K_0(A) $ est engendré par les classes des modules simples.",
		"Une proposition qui n’en est pas une, sans tiret cadratin.",
	)
	_, got, err := statements(in, corpus.Ref{Book: "alg", Chapter: "VIII", Section: 11}, printings["fr"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"alg-viii-s11-prop-1",
		"alg-viii-s11-def-2",
		"alg-viii-s11-def-2-cor-1",
	})
}

// The same head with the italic marked rather than lost, which is the other
// thing a reading of the image does with one. The French pages carry 248 of
// these and the six in the French Algebra VIII are the last statements missing
// from it. The line off page 209 is the Lemme.
func TestStatementsReadsAFrenchHeadWithTheItalicMarked(t *testing.T) {
	in := blocks(
		"### 1. Le groupe de Grothendieck",
		"*Lemme 1.* — *Soient E, E' et E'' des A-modules de longueur finie et*",
		"*Remarque.* — Notons $ \\Sigma $ l’espèce de structure de $ \\tau' $-extension.",
	)
	_, got, err := statements(in, corpus.Ref{Book: "alg", Chapter: "VIII", Section: 11}, printings["fr"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"alg-viii-s11-lem-1",
		"alg-viii-s11-n1-rem-1",
	})
}

// § 3 of chapter III of Groupes et algebres de Lie numbers a statement twice.
// It prints Definition 7 at no. 7 and DEFINITION 7 again at no. 12, and both
// page images say so. The later one takes the bis mark, so the first keeps the
// label every citation of def. 7 means, and Definition 8 after it is untouched.
//
// The page is part of the key, so a collision anywhere else still fails, which
// is what the second half of this checks: the same two statements one page over
// from the one that was verified are a fault and are reported as one.
func TestStatementsMarksANumberThePrintingGaveTwice(t *testing.T) {
	in := []block{
		{text: "### 7. Algèbre de Lie d’un groupe de Lie", page: 137, label: "LIE III.139"},
		{text: "Définition 7. — Soient X une variété de classe $ C^r $, g une algèbre de Lie normable.", page: 137, label: "LIE III.139"},
		{text: "### 12. Représentation adjointe", page: 151, label: "LIE III.153"},
		{text: "DÉFINITION 7. — La représentation Ad de G dans L(G) s’appelle la représentation adjointe de G.", page: 151, label: "LIE III.153"},
		{text: "Définition 8. — Soient G un groupe de Lie, M une variété de classe $ C^r $.", page: 160, label: "LIE III.162"},
	}
	id := corpus.Ref{Book: "lie", Chapter: "III", Section: 3}
	_, got, err := statements(in, id, printings["fr"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"lie-iii-s3-def-7",
		"lie-iii-s3-def-7-bis",
		"lie-iii-s3-def-8",
	})

	in[3].page, in[3].label = 152, "LIE III.154"
	if _, _, err := statements(in, id, printings["fr"]); err == nil {
		t.Error("a collision on a page nobody verified was let through")
	}
}

// § 6 of chapter II of Espaces vectoriels topologiques numbers a Definition
// twice and both printings of the volume do it, once at no. 2 under the weak
// topologies and again at no. 3 under the polar sets. A label names the book and
// not the volume, so the English and the French arrive at the same label on
// different pages, 81 and 82, and the exception carries a page for each.
func TestStatementsTakesARepeatFromEitherPrinting(t *testing.T) {
	id := corpus.Ref{Book: "evt", Chapter: "II", Section: 6}
	want := []string{"evt-ii-s6-def-2", "evt-ii-s6-def-2-bis"}

	english := []block{
		{text: "### 2. Weak topologies", page: 79, label: "TVS II.42"},
		{text: "DEFINITION 2. — Let F and G be two vector spaces put in duality by the bilinear form B.", page: 79, label: "TVS II.42"},
		{text: "### 3. Polar sets and orthogonal subspaces", page: 81, label: "TVS II.44"},
		{text: "Definition 2. — Let F and G be two (real) vector spaces in duality.", page: 81, label: "TVS II.44"},
	}
	_, got, err := statements(english, id, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), want)

	french := []block{
		{text: "### 2. Topologies faibles", page: 80, label: "EVT II.42"},
		{text: "DÉFINITION 2. — Soient F et G deux espaces vectoriels mis en dualité par la forme bilinéaire B.", page: 80, label: "EVT II.42"},
		{text: "### 3. Ensembles polaires et sous-espaces orthogonaux", page: 82, label: "EVT II.44"},
		{text: "Définition 2. — Soient F et G deux espaces vectoriels en dualité.", page: 82, label: "EVT II.44"},
	}
	_, got, err = statements(french, id, printings["fr"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), want)

	english[3].page, english[3].label = 83, "TVS II.46"
	if _, _, err := statements(english, id, printings["en"]); err == nil {
		t.Error("a collision on a page nobody verified was let through")
	}
}

// § 3 of chapter III of Commutative Algebra numbers a Definition twice, once at
// no. 1 where it says what an m-good filtration is and again at no. 3 where it
// says what a defining ideal of a topology is. Only the English volume is in the
// corpus, so the exception carries the one page, and a collision anywhere else
// in the § is still a fault.
func TestStatementsTakesTheCommutativeAlgebraRepeat(t *testing.T) {
	id := corpus.Ref{Book: "ac", Chapter: "III", Section: 3}
	in := []block{
		{text: "### 1. Good filtrations", page: 215, label: "195"},
		{text: "DEFINITION 1. Let $ A $ be a commutative ring and $ m $ an ideal of $ A $.", page: 216, label: "196"},
		{text: "### 3. Zariski rings", page: 221, label: "201"},
		{text: "DEFINITION 1. Let $ A $ be a topological ring.", page: 221, label: "201"},
	}
	_, got, err := statements(in, id, printings["en"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{"ac-iii-s3-def-1", "ac-iii-s3-def-1-bis"})

	in[3].page, in[3].label = 226, "206"
	if _, _, err := statements(in, id, printings["en"]); err == nil {
		t.Error("a collision on a page nobody verified was let through")
	}
}

// The same head again with the small capitals read as capitals rather than as
// bold, which is what 2033 heads across the French volumes came back as. Read as
// prose they take the whole chapter down with them and not only themselves: this
// is § 6 of chapter I of the French Groupes et algebres de Lie, where THEOREME 4
// and DEFINITION 5 went unread, so the parent of a corollary never advanced past
// Proposition 5 and the two corollaries under two different results were both
// labelled lie-i-s6-prop-5-cor-1.
func TestStatementsReadsAFrenchHeadInCapitals(t *testing.T) {
	in := blocks(
		"### 3. Représentations semi-simples",
		"PROPOSITION 5. — Soient $ g $ une algèbre de Lie, $ r $ son radical.",
		"COROLLAIRE 1. — Soient $ g $ une algèbre de Lie, $ a $ un idéal de $ g $.",
		"THÉORÈME 4. — *Soient g une algèbre de Lie, r son radical, $\\rho$ une représentation de g*.",
		"COROLLAIRE. — *Soient g, g' des algèbres de Lie, s le radical nilpotent de g*.",
		"DÉFINITION 5. — Soient $ g $ une algèbre de Lie, $ h $ une sous-algèbre de Lie de $ g $.",
		"**PROPOSITION 6.** — Soient $ g $ une algèbre de Lie, $ h $ une sous-algèbre réductive.",
		"THÉORÈME 5 (Levi). — *Soit g une algèbre de Lie de dimension finie*.",
		"REMARQUES. — 1) La sous-algèbre $ h $ n’est pas unique.",
	)
	_, got, err := statements(in, corpus.Ref{Book: "lie", Chapter: "I", Section: 6}, printings["fr"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"lie-i-s6-prop-5",
		"lie-i-s6-prop-5-cor-1",
		"lie-i-s6-thm-4",
		"lie-i-s6-n3-cor-1",
		"lie-i-s6-def-5",
		"lie-i-s6-prop-6",
		"lie-i-s6-thm-5",
		"lie-i-s6-n3-rem-1",
	})
}

// Page 41 of Algebre commutative chapitres 5 a 7 sets the number of a displayed
// formula on a line of its own, "(3)" with the display under it, while the run
// of Remarques opened on page 40 is still open and up to 3. Read as a member,
// that number takes the label of the remark on page 42 that really is the third,
// two statements come out labelled ac-v-s2-n2-rem-3, and the chapter does not
// assemble at all.
func TestTheNumberOfADisplayedFormulaIsNotAMemberOfAnOpenRun(t *testing.T) {
	in := blocks(
		"### 2. Groupe de décomposition et groupe d’inertie",
		"Remarques. — 1) On notera que $k'$ peut être de degré infini sur $k$.",
		"2) Il est clair que $k'$ est une extension galoisienne de $k$.",
		"(3)\n$$\nB = A + \\mathfrak{p}(B)\n$$",
		"Remarques. — 3) Si $A'$ est intègre et noethérien.",
		"4) Lorsque $\\mathfrak{p}$ n’est pas un idéal maximal de $A$.",
	)
	out, got, err := statements(in, corpus.Ref{Book: "ac", Chapter: "V", Section: 2}, printings["fr"])
	if err != nil {
		t.Fatal(err)
	}
	same(t, labels(got), []string{
		"ac-v-s2-n2-rem-1",
		"ac-v-s2-n2-rem-2",
		"ac-v-s2-n2-rem-3",
		"ac-v-s2-n2-rem-4",
	})
	if !slices.Contains(texts(out), "(3)\n$$\nB = A + \\mathfrak{p}(B)\n$$") {
		t.Errorf("the numbered display did not stay in the prose: %v", texts(out))
	}
}

// The exercise before this one ends on a display, and a display closes on two
// dollars. Reading the second of the two as the dollar that opens the marker's
// span cuts the delimiter in half: the exercise before is left with one dollar
// and a formula that never closes, and afterMarker then takes the marker for a
// span left open and deletes the next real dollar out of this exercise to close
// it. Exercises 30, 31 and 32 of § 1 of chapter III of Functions of a Real
// Variable are the case that found it.
func TestItemStartDoesNotCutADisplayDelimiterInHalf(t *testing.T) {
	text := "a) Show that\n$$\n\\int_0^{+\\infty} f = \\pi.\n$$\n31) If $ l_{m,n} $ is a primitive of $ g $, show that"
	i, m := itemStart(text, 31)
	if i < 0 {
		t.Fatal("exercise 31 was not found")
	}
	if before := text[:i]; !strings.HasSuffix(before, "$$") {
		t.Errorf("the exercise before ends %q, so its display no longer closes", first(before[len(before)-8:], 8))
	}
	if got := afterMarker(m[0], text[i+markerLen(m):]); !strings.HasPrefix(got, "If $ l_{m,n} $") {
		t.Errorf("exercise 31 begins %q", first(got, 40))
	}
}

// A block that ends on what reads as a marker gave the caller a marker one
// byte longer than the block, because the pattern is matched against the block
// with a space on the end of it and the space stayed in the match. Page 112 of
// Topologie generale V a X in French is the case and the audit panicked on it.
func TestItemStartOnAMarkerThatEndsTheBlock(t *testing.T) {
	for _, s := range []string{
		"On munit E de la topologie de la convergence simple (cf. VII, p. 16, prop. 2)",
		"Soit X un espace compact. 2)",
	} {
		i, m := itemStart(s, 2)
		if i < 0 {
			continue
		}
		if n := i + markerLen(m); n > len(s) {
			t.Errorf("itemStart(%q) marks %d bytes of a block of %d", first(s, 40), n, len(s))
		}
	}
}

func TestANoIsReadWhenTheFascicleNumbersItByItsSection(t *testing.T) {
	// Varietes differentielles et analytiques, fascicule de resultats, sets the
	// fourth no. of its first section "1.4. Produit de fonctions derivables" and
	// lists it in its own contents as no. 4 of § 1. Three of its no. carry no
	// full stop after the number.
	for _, tc := range []struct {
		line  string
		no    int
		title string
	}{
		{"### 3. Simple Modules", 3, "Simple Modules"},
		{"### 1.4. Produit de fonctions derivables", 4, "Produit de fonctions derivables"},
		{"### 1.1 Ordre de contact de deux fonctions en un point", 1, "Ordre de contact de deux fonctions en un point"},
		{"### 5.11. Produits fibres et images reciproques", 11, "Produits fibres et images reciproques"},
		{`### \*10. Subimmersions`, 10, "Subimmersions"},
	} {
		m := subsecRE.FindStringSubmatch(tc.line)
		if m == nil {
			t.Errorf("%q read as no heading", tc.line)
			continue
		}
		if m[1] != strconv.Itoa(tc.no) || m[2] != tc.title {
			t.Errorf("%q gives no. %q titled %q, want %d and %q", tc.line, m[1], m[2], tc.no, tc.title)
		}
	}
	// A title that opens with a numeral is still a title. Chapter I of Algebre
	// has "### 3. 2-groupes" nowhere, but the pattern has to leave the digit to
	// the title or a volume that does would lose its heading.
	m := subsecRE.FindStringSubmatch("### 3. 2-groupes")
	if m == nil || m[1] != "3" || m[2] != "2-groupes" {
		t.Errorf("### 3. 2-groupes gives %v", m)
	}
}
