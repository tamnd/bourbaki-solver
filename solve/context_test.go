package solve

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture is a corpus of two §, four exercises and a citation of every shape
// the assembler has to tell apart: one into the exercise's own §, one into the
// other §, one forward to a later exercise, one that resolves no further than a
// § because it names a page and nothing else, and one that leaves the corpus.
//
// It is small enough to read, which is the point. Every number the tests below
// assert can be counted by eye off this file.
func fixture(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(path, body string) {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("manifests/sections.yaml", `books:
    - id: alg-viii
      chapters:
        - chapter: VIII
          sections:
            - kind: section
              section: 1
              path: content/en/alg/VIII/01_s1_a.md
              label: alg-viii-s1
              book_pages: A VIII.1 - A VIII.8
            - kind: section
              section: 2
              path: content/en/alg/VIII/02_s2_b.md
              label: alg-viii-s2
              book_pages: A VIII.9 - A VIII.16
`)
	write("manifests/exercises.json", `{"books":[{"id":"alg-viii","chapters":[{"chapter":"VIII",
	  "sections":[{"section":1,"label":"alg-viii-s1","dir":"s1","count":4,"first":1,"last":4},
	              {"section":2,"label":"alg-viii-s2","dir":"s2","count":1,"first":1,"last":1}]}]}]}`)
	write("content/en/alg/VIII/01_s1_a.md", `---
book: alg
chapter: VIII
section: 1
lang: en
subsections:
    - "no": 1
      page: 1
statements: 1
exercises: 4
---

## § 1. THE FIRST SECTION

### 1. A first no.

#### Proposition 1 {#alg-viii-s1-prop-1 .statement tag=0001}

Every ring is a ring, by VIII, p. 9, Proposition 1.
`)
	write("content/en/alg/VIII/02_s2_b.md", `---
book: alg
chapter: VIII
section: 2
lang: en
subsections:
    - "no": 1
      page: 9
statements: 1
exercises: 1
---

## § 2. THE SECOND SECTION

### 1. A first no.

#### Proposition 1 {#alg-viii-s2-prop-1 .statement tag=0002}

This is the proposition the first section reaches at depth two. It cites Set
Theory, III, §3, No. 6, p. 155, Proposition 13, which is not in the corpus.
`)
	ex := func(n int, tag, body string) {
		write("content/en/alg/VIII/exercises/s1/0"+string(rune('0'+n))+".md", `---
book: alg
chapter: VIII
section: 1
exercise: `+string(rune('0'+n))+`
label: alg-viii-s1-ex-`+string(rune('0'+n))+`
tag: `+tag+`
lang: en
---

`+body+`
`)
	}
	ex(1, "0003", "The first exercise says nothing of interest.")
	ex(2, "0004", "The second exercise cites VIII, p. 12, which names a page and no statement.")
	ex(3, "0005", "The third cites Proposition 1 of this section, and VIII, p. 9, Proposition 1, "+
		"and Exercise 4 of this section, and Set Theory, III, §2, No. 4, p. 155.")
	ex(4, "0006", "The fourth exercise is printed after the third and is cited by it.")
	return root
}

func read(t *testing.T) *Corpus {
	t.Helper()
	c, err := Read(fixture(t), "en")
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func build(t *testing.T, c *Corpus, label string, o Options) *Context {
	t.Helper()
	ctx, err := c.Build(label, o)
	if err != nil {
		t.Fatal(err)
	}
	return ctx
}

func labels(ps []Piece, k Kind) []string {
	var out []string
	for _, p := range ps {
		if p.Kind == k {
			out = append(out, p.Label)
		}
	}
	return out
}

func TestAnExerciseIsShownItselfItsEarlierSiblingsAndItsSection(t *testing.T) {
	c := read(t)
	ctx := build(t, c, "alg-viii-s1-ex-3", Options{})

	if got := labels(ctx.Pieces, TheExercise); len(got) != 1 || got[0] != "alg-viii-s1-ex-3" {
		t.Errorf("the exercise came out as %v", got)
	}
	want := []string{"alg-viii-s1-ex-1", "alg-viii-s1-ex-2"}
	if got := labels(ctx.Pieces, Sibling); strings.Join(got, ",") != strings.Join(want, ",") {
		t.Errorf("the siblings came out as %v, want %v", got, want)
	}
	if got := labels(ctx.Pieces, TheSection); len(got) != 1 || got[0] != "alg-viii-s1" {
		t.Errorf("the § came out as %v", got)
	}
}

// The fourth exercise is printed after the third and the third cites it. It is
// not a sibling, since the siblings are the ones this exercise was written to
// follow, and it is carried anyway, since the book pointed at it.
func TestAnExerciseCitedForwardIsCarriedAndIsNotASibling(t *testing.T) {
	c := read(t)
	ctx := build(t, c, "alg-viii-s1-ex-3", Options{})

	for _, l := range labels(ctx.Pieces, Sibling) {
		if l == "alg-viii-s1-ex-4" {
			t.Fatal("the fourth exercise was carried as a sibling of the third")
		}
	}
	if !has(labels(ctx.Pieces, Reference), "alg-viii-s1-ex-4") {
		t.Errorf("the fourth exercise was not carried at all: %v", labels(ctx.Pieces, Reference))
	}
}

// Proposition 1 of § 1 is cited by exercise 3 of § 1, and § 1 is in the context
// whole, so carrying it again would be writing the same words twice.
func TestAStatementOfTheSectionAlreadyCarriedIsNotCarriedTwice(t *testing.T) {
	c := read(t)
	ctx := build(t, c, "alg-viii-s1-ex-3", Options{})

	if has(labels(ctx.Pieces, Reference), "alg-viii-s1-prop-1") {
		t.Error("Proposition 1 of § 1 was carried beside the § that contains it")
	}
	if !has(labels(ctx.Pieces, Reference), "alg-viii-s2-prop-1") {
		t.Errorf("Proposition 1 of § 2 was not carried: %v", labels(ctx.Pieces, Reference))
	}
}

// Set Theory is not in the corpus and neither is the volume the § 2 proposition
// cites. Both are named, and the instruction that goes with them is the one
// spec 07 §3.1 asks for.
func TestACitationThatLeavesTheCorpusIsNamedVerbatim(t *testing.T) {
	c := read(t)
	ctx := build(t, c, "alg-viii-s1-ex-3", Options{})

	var raw []string
	for _, p := range ctx.Pieces {
		if p.Kind == Outside {
			raw = append(raw, p.Raw)
		}
	}
	if len(raw) == 0 {
		t.Fatal("nothing left the corpus")
	}
	for _, r := range raw {
		if !strings.Contains(ctx.Render(), r) {
			t.Errorf("%q was collected and not written out", r)
		}
	}
	if !strings.Contains(ctx.Render(), "use the standard") {
		t.Error("the out-of-corpus instruction is missing")
	}
}

// Exercise 2 cites a bare page, which the resolver can narrow to the § holding
// that page and no further. Forty thousand characters of § is not what the
// sentence asked for, so it is named and not carried.
func TestAPageCitationThatNarrowsOnlyToASectionIsNamed(t *testing.T) {
	c := read(t)
	ctx := build(t, c, "alg-viii-s1-ex-2", Options{})

	var found bool
	for _, p := range ctx.Named {
		if p.Label == "alg-viii-s2" && p.Why == SectionOnly {
			found = true
		}
	}
	if !found {
		t.Fatalf("the page citation was not named: %+v", ctx.Named)
	}
	if has(labels(ctx.Pieces, Reference), "alg-viii-s2") {
		t.Error("a whole § was carried for a citation that named a page")
	}
	if !strings.Contains(ctx.Render(), "resolved no further than this §") {
		t.Error("the model is not told why the § is missing")
	}
}

// The § 2 proposition is cited by exercise 3 and cites Set Theory itself, so
// Set Theory is reached at depth 2 and only at depth 2.
func TestTheClosureReachesWhatTheCitedStatementCites(t *testing.T) {
	c := read(t)
	deep := build(t, c, "alg-viii-s1-ex-3", Options{Depth: 2})
	shallow := build(t, c, "alg-viii-s1-ex-3", Options{Depth: 1})

	want := "III, §3, No. 6, p. 155, Proposition 13"
	if !has(raws(deep.Pieces), want) {
		t.Errorf("depth 2 did not reach %q: %v", want, raws(deep.Pieces))
	}
	if has(raws(shallow.Pieces), want) {
		t.Errorf("depth 1 reached %q, which is two citations away", want)
	}
}

// raws is the citations that left the corpus. The words are asserted on rather
// than the rendering, because the § 2 proposition prints the citation in its own
// body and is itself carried, so the rendering holds the words at either depth.
func raws(ps []Piece) []string {
	var out []string
	for _, p := range ps {
		if p.Kind == Outside {
			out = append(out, p.Raw)
		}
	}
	return out
}

// A cap that drops a reference and says nothing reads to the model exactly like
// a context that never had it.
func TestTheCapNamesWhatItDrops(t *testing.T) {
	c := read(t)
	ctx := build(t, c, "alg-viii-s1-ex-3", Options{MaxChars: 1})

	if n := len(labels(ctx.Pieces, Reference)); n != 0 {
		t.Errorf("%d references were carried under a cap of one character", n)
	}
	var dropped int
	for _, p := range ctx.Named {
		if p.Why == OverCap {
			dropped++
		}
	}
	if dropped == 0 {
		t.Fatalf("the cap dropped nothing and named nothing: %+v", ctx.Named)
	}
	if !strings.Contains(ctx.Render(), "limit of 1 characters") {
		t.Error("the model is not told the cap is what took them")
	}
}

// The § carries its own headings and the exercises do not, so a piece boundary
// has to be something the corpus cannot print.
func TestEveryPieceIsBracketedByAMarkTheCorpusCannotContain(t *testing.T) {
	c := read(t)
	ctx := build(t, c, "alg-viii-s1-ex-3", Options{})
	out := ctx.Render()

	var carried int
	for _, p := range ctx.Pieces {
		if p.Kind != Outside {
			carried++
		}
	}
	if got := strings.Count(out, "\n"+openMark+" ") + boolToInt(strings.HasPrefix(out, openMark+" ")); got != carried {
		t.Errorf("%d pieces opened, want %d", got, carried)
	}
	if got := strings.Count(out, "\n"+closeMark+"\n"); got != carried {
		t.Errorf("%d pieces closed, want %d", got, carried)
	}
}

func TestAnExerciseTheCorpusDoesNotHold(t *testing.T) {
	c := read(t)
	if _, err := c.Build("alg-viii-s9-ex-1", Options{}); err == nil {
		t.Fatal("an exercise that is not in the corpus built a context")
	}
}

func has(xs []string, want string) bool {
	for _, x := range xs {
		if x == want {
			return true
		}
	}
	return false
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

// A printing with an error in it keeps the error and carries the correction
// beside it. The words a reader holds in their hands are not edited, and the
// model is not asked to solve an exercise that cannot be solved.
func TestAnExerciseCarriesTheCorrectionsToItsPrinting(t *testing.T) {
	root := fixture(t)
	if err := os.WriteFile(filepath.Join(root,
		"content/en/alg/VIII/exercises/s1/01.md"), []byte(`---
book: alg
chapter: VIII
section: 1
exercise: 1
label: alg-viii-s1-ex-1
tag: "0003"
lang: en
errata:
    - says: finite-dimensional over K'
      read: infinite-dimensional over K'
      why: the French of 1981 says de dimension infinie, and the deduction the
        exercise asks for holds only in the infinite case.
---

The first exercise asks for a field finite-dimensional over K'.
`), 0o644); err != nil {
		t.Fatal(err)
	}
	c, err := Read(root, "en")
	if err != nil {
		t.Fatal(err)
	}
	text := build(t, c, "alg-viii-s1-ex-1", Options{}).Pieces[0].Text
	if !strings.Contains(text, "The first exercise asks for a field finite-dimensional") {
		t.Error("the printed words were edited rather than kept")
	}
	for _, want := range []string{"has an error in it", `read "infinite-dimensional over K'"`,
		"de dimension infinie"} {
		if !strings.Contains(text, want) {
			t.Errorf("the correction does not say %q:\n%s", want, text)
		}
	}
	// And an exercise printed correctly gains nothing.
	if plain := build(t, c, "alg-viii-s1-ex-2", Options{}).Pieces[0].Text; strings.Contains(
		plain, "has an error in it") {
		t.Errorf("an exercise with no erratum on it was told there was one: %s", plain)
	}
	// The correction travels with the exercise when a later one is being
	// solved, because exercise 4 b) of § 1 is answered out of exercise 3 a).
	sib := build(t, c, "alg-viii-s1-ex-3", Options{})
	var carried bool
	for _, p := range sib.Pieces {
		if p.Kind == Sibling && p.Label == "alg-viii-s1-ex-1" {
			carried = strings.Contains(p.Text, "has an error in it")
		}
	}
	if !carried {
		t.Error("the correction was left behind when the exercise went in as a sibling")
	}
}
