package assemble

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

func page(n int, label string, continues bool, body string) corpus.PageFile {
	return corpus.PageFile{
		Meta: corpus.PageFrontMatter{
			Book: "alg-viii", PDFPage: n, PageLabel: label,
			Continues: continues, Method: corpus.MethodNative,
		},
		Body: body,
	}
}

// A chapter cut down to three pages, with one of everything that assembly has
// to put back: a title page carrying the publisher's line, a § heading checked
// against the contents, a no. heading, statements set in bold, a paragraph
// broken across the page break, two footnotes both printed as 1, and the
// exercises at the end.
func smallChapter() (corpus.Chapter, map[int]corpus.PageFile) {
	ch := corpus.Chapter{
		Book: "alg-viii", Numeral: "VIII", Title: "Semisimple Modules and Rings", Page: 1, PDFPage: 18,
		Sections: []corpus.Section{{
			Number: 1, Title: "Artinian Modules and Noetherian Modules", Page: 1, PDFPage: 18,
			Subsections: []corpus.Subsection{
				{Number: 1, Title: "Artinian Modules and Noetherian Modules", Page: 1, PDFPage: 18},
				{Number: 2, Title: "Artinian Rings and Noetherian Rings", Page: 2, PDFPage: 19},
			},
			Exercises: &corpus.Locator{Page: 3, PDFPage: 20},
		}},
	}
	pages := map[int]corpus.PageFile{
		18: page(18, "A VIII.1", false, strings.Join([]string{
			"## CHAPTER VIII SEMISIMPLE MODULES AND RINGS",
			"© N. Bourbaki 2023 N. Bourbaki, Algebra, https://doi.org/10.1007/978-3-031-19291-7_1",
			"## § 1. ARTINIAN MODULES AND NOETHERIAN MODULES",
			"### 1. Artinian Modules and Noetherian Modules",
			"**Definition 1.** — An A-module M is said to be Artinian if every nonempty set of submodules of M has a minimal element[^1].",
			"**Examples.** — 1) A finite-dimensional vector space is Artinian.",
			"2) Let M be an A-module with an infinite family of submodules. Then M is neither Artinian nor",
			"[^1]: The empty set has no minimal element.",
		}, "\n\n")),
		19: page(19, "A VIII.2", true, strings.Join([]string{
			"Noetherian.",
			"### 2. Artinian Rings and Noetherian Rings",
			"**Proposition 1.** — Let A be a ring. The ring A is left Artinian if and only if the A-module A is Artinian[^1].",
			"**Corollary.** — Every quotient of a left Artinian ring is left Artinian.",
			"[^1]: This is the definition used throughout.",
		}, "\n\n")),
		20: page(20, "A VIII.3", false, strings.Join([]string{
			"### Exercises",
			"1) Let A be a ring. Show that A is left Artinian[^1].",
			`$\P 2)$ Let K be a field and V a K-vector space.`,
			"[^1]: A ring here is a ring with unit element.",
		}, "\n\n")),
		21: page(21, "", false, "# BIBLIOGRAPHY"),
	}
	return ch, pages
}

func TestChapter(t *testing.T) {
	ch, pages := smallChapter()
	got, err := Chapter("alg", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("got %d pieces, want the front matter and § 1", len(got))
	}
	front, sec := got[0], got[1]
	if !front.Front || front.Name() != "front matter" {
		t.Errorf("the first piece is %+v", front)
	}
	if front.First != 18 || front.Last != 18 {
		t.Errorf("the front matter runs %d-%d, want 18-18", front.First, front.Last)
	}
	if sec.First != 18 || sec.Last != 20 {
		t.Errorf("§ 1 runs %d-%d, want 18-20", sec.First, sec.Last)
	}
	if sec.FirstLabel != "A VIII.1" || sec.LastLabel != "A VIII.3" {
		t.Errorf("§ 1 runs %s-%s", sec.FirstLabel, sec.LastLabel)
	}
	if got, want := len(sec.Subsections), 2; got != want {
		t.Errorf("got %d no., want %d", got, want)
	}
	// The Corollary carries no number, so it is named by where it stands. See
	// statementAt.
	same(t, labels(sec.Statements), []string{
		"alg-viii-s1-def-1",
		"alg-viii-s1-n1-exa-1",
		"alg-viii-s1-n1-exa-2",
		"alg-viii-s1-prop-1",
		"alg-viii-s1-n2-cor-1",
	})
	if len(sec.Exercises) != 2 {
		t.Fatalf("got %d exercises, want 2", len(sec.Exercises))
	}
	if e := sec.Exercises[1]; e.Meta.Label != "alg-viii-s1-ex-2" || !e.Meta.Starred {
		t.Errorf("exercise 2 = %+v", e.Meta)
	}
	if !sec.HasExercise {
		t.Error("§ 1 has exercises and does not say so")
	}
}

// The exercises go out as one file each, so the section keeps the heading a
// cross-reference points at and a line saying where they went, and none of the
// text of them.
func TestChapterSplitsTheExercises(t *testing.T) {
	ch, pages := smallChapter()
	got, err := Chapter("alg", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	sec := got[1]
	if !strings.Contains(sec.Body, "### Exercises {#alg-viii-s1-exercises}") {
		t.Error("the anchored heading is gone from the section")
	}
	if !strings.Contains(sec.Body, "See the [exercises for § 1](exercises/s1/).") {
		t.Errorf("the section does not say where its exercises went:\n%s", sec.Body)
	}
	for _, gone := range []string{"Show that A is left Artinian", "a K-vector space"} {
		if strings.Contains(sec.Body, gone) {
			t.Errorf("the section still carries the exercise text %q", gone)
		}
	}
	if got, want := sec.Exercises[0].Body, "Let A be a ring. Show that A is left Artinian[^1]."; !strings.HasPrefix(got, want) {
		t.Errorf("exercise 1 is %q", got)
	}
}

// A footnote belongs in the file its mark is in. The note of exercise 1 is
// marked in the exercise and printed at the foot of the page the exercise is
// on, so it leaves the section with it, and both files number from one.
func TestChapterPutsAFootnoteInTheFileThatMarksIt(t *testing.T) {
	ch, pages := smallChapter()
	got, err := Chapter("alg", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	sec := got[1]
	if strings.Contains(sec.Body, "a ring with unit element") {
		t.Error("the section kept a note that belongs to an exercise")
	}
	if strings.Count(sec.Body, "\n[^") != 2 || !strings.Contains(sec.Body, "[^1]: The empty set") {
		t.Errorf("the section's notes are wrong:\n%s", sec.Body)
	}
	ex := sec.Exercises[0].Body
	if !strings.Contains(ex, "[^1]: A ring here is a ring with unit element.") {
		t.Errorf("exercise 1 does not carry its note:\n%s", ex)
	}
	if !strings.Contains(ex, "left Artinian[^1].") {
		t.Errorf("the mark in exercise 1 was not renumbered to one:\n%s", ex)
	}
}

// A definition with nothing pointing at it is a mark extraction lost, and
// putting it at the foot of the section anyway would hide that.
func TestChapterRefusesAFootnoteNothingMarks(t *testing.T) {
	ch, pages := smallChapter()
	p := pages[19]
	p.Body = strings.Replace(p.Body, "Artinian[^1]", "Artinian", 1)
	pages[19] = p
	if _, err := Chapter("alg", ch, pages); err == nil {
		t.Fatal("a footnote nothing marks should be an error")
	}
}

// The publisher's line is not part of the book and is not ours to republish.
func TestChapterDropsTheImprint(t *testing.T) {
	ch, pages := smallChapter()
	got, err := Chapter("alg", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range got {
		if strings.Contains(p.Body, "©") || strings.Contains(p.Body, "doi.org") {
			t.Errorf("%s still carries the imprint", p.Name())
		}
	}
}

// The sentence broken by the end of page 18 has to come back as one sentence,
// and the two footnotes both printed as 1 have to come out 1 and 2, in the
// definition and in the mark alike.
func TestChapterJoinsAndRenumbers(t *testing.T) {
	ch, pages := smallChapter()
	got, err := Chapter("alg", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	body := got[1].Body
	if !strings.Contains(body, "neither Artinian nor Noetherian.") {
		t.Error("the paragraph broken at the page break was not joined")
	}
	for _, want := range []string{"[^1]: The empty set", "[^2]: This is the definition", "minimal element[^1]", "Artinian[^2]"} {
		if !strings.Contains(body, want) {
			t.Errorf("the body carries no %q", want)
		}
	}
}

// The exercises of § 1 open on page 20 with no paragraph indent, so the page
// says it carries on the page before it. It does not, and the text is what
// says so.
func TestChapterDoesNotGlueAnExerciseOntoThePageBefore(t *testing.T) {
	ch, pages := smallChapter()
	p := pages[20]
	p.Meta.Continues = true
	p.Body = strings.TrimPrefix(p.Body, "### Exercises\n\n")
	pages[20] = p
	ch.Sections[0].Exercises = nil

	got, err := Chapter("alg", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got[1].Body, "\n\n1) Let A be a ring.") {
		t.Errorf("the exercise was glued onto the paragraph before it:\n%s", got[1].Body)
	}
}

// A section heading the contents does not agree with is an error. Nothing here
// can tell which of the two is wrong, and guessing is how a chapter ends up
// silently missing a §.
func TestChapterRefusesAHeadingTheContentsDisagreesWith(t *testing.T) {
	ch, pages := smallChapter()
	ch.Sections[0].Title = "Artinian Modules"
	if _, err := Chapter("alg", ch, pages); err == nil {
		t.Fatal("a title the page disagrees with should be an error")
	}
	ch, pages = smallChapter()
	ch.Sections[0].PDFPage = 19
	if _, err := Chapter("alg", ch, pages); err == nil {
		t.Fatal("a § on the wrong page should be an error")
	}
}

// Assembly reads the pages and never the PDF, so a page it has not been given
// is an error rather than a gap to write out.
func TestChapterRefusesAMissingPage(t *testing.T) {
	ch, pages := smallChapter()
	delete(pages, 19)
	if _, err := Chapter("alg", ch, pages); err == nil {
		t.Fatal("a missing page should be an error")
	}
}

func TestPieceName(t *testing.T) {
	cases := []struct {
		piece Piece
		name  string
	}{
		{Piece{Front: true}, "front matter"},
		{Piece{Historical: true}, "historical note"},
		{Piece{Section: corpus.Section{Number: 3}}, "§ 3"},
		{Piece{Section: corpus.Section{Number: 2, Appendix: true}}, "Appendix 2"},
	}
	for _, c := range cases {
		if got := c.piece.Name(); got != c.name {
			t.Errorf("Name() = %q, want %q", got, c.name)
		}
	}
}

// A section is read one way or two, and the front matter of the file says
// which: one repaired page in an otherwise native section makes the section
// native+repair, so that nobody has to go back to the pages to find out. A
// blank page is read no way at all and counts as nothing.
func TestPieceExtraction(t *testing.T) {
	cases := []struct {
		methods []corpus.PageMethod
		want    string
	}{
		{[]corpus.PageMethod{corpus.MethodNative, corpus.MethodNative}, "native"},
		{[]corpus.PageMethod{corpus.MethodOCR, corpus.MethodOCR}, "ocr"},
		{[]corpus.PageMethod{corpus.MethodOCR, corpus.MethodOCRRepair}, "ocr+repair"},
		{[]corpus.PageMethod{corpus.MethodNative, corpus.MethodBlank}, "native"},
		{[]corpus.PageMethod{corpus.MethodBlank}, "blank"},
	}
	for _, c := range cases {
		if got := (Piece{Methods: c.methods}).Extraction(); got != c.want {
			t.Errorf("Extraction(%v) = %q, want %q", c.methods, got, c.want)
		}
	}
}

// A word broken across a page break comes back together, and the hyphen of
// A-module stays. TeX will not break a word after its first letter, which is
// what tells the two apart.
func TestGlue(t *testing.T) {
	cases := []struct{ prev, next, want string }{
		{"the module is", "Artinian", "the module is Artinian"},
		{"a semi-", "simple ring", "a semisimple ring"},
		{"an A-", "module M", "an A-module M"},
		{"a $K$-", "algebra", "a $K$-algebra"},
		{`a $\mathbf{Q}$-`, "vector space", `a $\mathbf{Q}$-vector space`},
	}
	for _, c := range cases {
		if got := glue(c.prev, c.next); got != c.want {
			t.Errorf("glue(%q, %q) = %q, want %q", c.prev, c.next, got, c.want)
		}
	}
}

func TestJoinable(t *testing.T) {
	cases := []struct {
		prev, next string
		want       bool
	}{
		{"the module is", "Artinian.", true},
		{"the module is", "1) Let A be a ring.", false},
		{"the module is", `$\P 15)$ Let K be a field.`, false},
		{"the module is", "## § 2. THE RADICAL", false},
		{"$$x = y$$", "the table that follows", false},
		{"the module is", "$$x = y$$", false},
		{"the module is", "[^1]: The empty set.", false},
		// Bourbaki sets a statement flush left, so a page that opens on one looks
		// exactly like a page that opens mid-paragraph and extraction reads
		// continues: true off the indent. Joined, the statement ends up as prose
		// inside the paragraph above and never gets a heading, a label or a tag.
		// That was 37 statements of chapter VIII, found by the reference graph
		// rather than by eye: the book cites Theorem 1 of § 7 thirteen times and
		// there was no Theorem 1 of § 7.
		{"the module is", "**Proposition 7.** — Let A be a ring.", false},
		{"the module is", "Theorem 1 (Wedderburn). — A ring is simple", false},
	}
	for _, c := range cases {
		if got := joinable(c.prev, c.next); got != c.want {
			t.Errorf("joinable(%q, %q) = %v, want %v", c.prev, c.next, got, c.want)
		}
	}
}

// Page 18 carries the imprint as a paragraph of its own and page 485 has one
// glued onto the end of a footnote, which is why this trims rather than drops.
func TestImprint(t *testing.T) {
	alone := "© N. Bourbaki 2023 N. Bourbaki, Algebra, https://doi.org/10.1007/978-3-031-19291-7_1"
	if got := imprint(alone); got != "" {
		t.Errorf("imprint(alone) = %q, want empty", got)
	}
	tail := "[^2]: See the note of chapter V. © Springer Nature Switzerland AG 2023 N. Bourbaki, Algebra, https://doi.org/10.1007/978-3-031-19291-7"
	if got, want := imprint(tail), "[^2]: See the note of chapter V."; got != want {
		t.Errorf("imprint(tail) = %q, want %q", got, want)
	}
	plain := "The mapping is bijective."
	if got := imprint(plain); got != plain {
		t.Errorf("imprint(plain) = %q", got)
	}
}

// The book numbers its footnotes from one on every page, so two notes of one §
// come out both called 1. Renumbering 2 to 3 and then 3 to 4 must not carry the
// first note along with the second.
func TestRenumber(t *testing.T) {
	body := "first[^1] second[^2] third[^3]"
	defs := []string{"[^1]: one", "[^2]: two", "[^3]: three"}
	got, out := renumber(body, defs, 2)
	if want := "first[^2] second[^3] third[^4]"; got != want {
		t.Errorf("body = %q, want %q", got, want)
	}
	same(t, out, []string{"[^2]: one", "[^3]: two", "[^4]: three"})
}

// A note is printed at the foot of the page its mark is on, so a body that
// carries the mark but not the page has no claim on it. Without that rule a
// subscript misread as a mark thirty pages away takes the note off the text
// that really carries it, which is what page 449 of Algebra VIII does to § 21.
func TestTakeNotesGoesByThePageAsWellAsTheMark(t *testing.T) {
	defs := []note{{def: "[^1]: the note", page: 415}}
	body, left := takeNotes("a stray mark[^1] on another page", []int{449}, defs)
	if len(left) != 1 {
		t.Errorf("the note went to a body that does not hold its page: %q", body)
	}
	body, left = takeNotes("the real mark[^1] here", []int{415}, defs)
	if len(left) != 0 {
		t.Fatal("the note did not go to the body that marks it on its own page")
	}
	if want := "the real mark[^1] here\n\n[^1]: the note"; body != want {
		t.Errorf("body = %q, want %q", body, want)
	}
}

func TestCutExercises(t *testing.T) {
	in := []block{
		{text: "some prose", page: 20, last: 20},
		{text: "### Exercises {#alg-viii-a2-exercises}", page: 20, last: 20},
		{text: "1) Let A be a ring.", page: 20, last: 21},
	}
	got := cutExercises(in, 2, true)
	if len(got) != 3 {
		t.Fatalf("got %d blocks, want the prose, the heading and the link", len(got))
	}
	if want := "See the [exercises for Appendix 2](exercises/a2/)."; got[2].text != want {
		t.Errorf("the link is %q, want %q", got[2].text, want)
	}
}

func TestCutNotes(t *testing.T) {
	body := "a paragraph[^1]\n\n[^1]: the note\n\nanother paragraph"
	keep, defs := cutNotes(body)
	if want := "a paragraph[^1]\n\nanother paragraph"; keep != want {
		t.Errorf("keep = %q, want %q", keep, want)
	}
	same(t, defs, []string{"[^1]: the note"})
}
