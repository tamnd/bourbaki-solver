package assemble

import (
	"slices"
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

// The same chapter as a volume paginated straight through prints it: no page
// label anywhere, a folio at the foot of each page, and the grammar of the
// volume saying so. Theory of Sets is this kind, and until the folio was carried
// through assembly a § of it had no printed pages at all: the runs came out with
// empty labels, book_pages was written empty, and the reference index put the
// volume on no page of the book.
func folioChapter() (corpus.Chapter, map[int]corpus.PageFile) {
	ch, pages := smallChapter()
	for n, p := range pages {
		p.Meta.PageLabel = ""
		p.Meta.Folio = n - 3
		pages[n] = p
	}
	return ch, pages
}

func TestAVolumeWithNoPageLabelIsPlacedByItsFolio(t *testing.T) {
	ch, pages := folioChapter()
	got, err := Chapter("alg", "en", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	sec := got[1]
	if want := []Run{{First: 18, Last: 20, FirstFolio: 15, LastFolio: 17}}; !slices.Equal(sec.Runs, want) {
		t.Errorf("§ 1 runs %+v, want %+v", sec.Runs, want)
	}
	// The no. is placed by the folio too, or a reference to no. 2 by page would
	// land in whichever no. was placed last.
	if len(sec.Subsections) != 2 || sec.Subsections[1].Page != 16 {
		t.Errorf("the no. of § 1 are %+v, want the second of them on page 16", sec.Subsections)
	}
}

// The page a § opens on prints no number, which is what a page that opens
// something does in a volume that carries the number in the running head. Taken
// as it stands the run has no number at one end, so it is written as no range
// at all, and § 1 of every chapter of Lie 7 to 9 came out printed on no page of
// the book.
func TestASectionOpeningOnAnUnnumberedPageStillHasItsPages(t *testing.T) {
	ch, pages := folioChapter()
	first := pages[18]
	first.Meta.Folio = 0
	pages[18] = first
	got, err := Chapter("alg", "en", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	sec := got[1]
	if want := []Run{{First: 18, Last: 20, FirstFolio: 15, LastFolio: 17}}; !slices.Equal(sec.Runs, want) {
		t.Errorf("§ 1 runs %+v, want %+v", sec.Runs, want)
	}
}

// A sentence opens a bracket at the foot of one page and closes it at the head
// of the next, and the closing bracket comes back from the text layer inside the
// mathematics. Neither page holds the fault on its own: the first closes nothing
// and the second opens nothing, so a repair that reads one page at a time has
// nothing to go on and the assembled section is the first place it can be seen.
// Ten spans across the corpus are this, among them the end of A VIII § 1
// Exercise 15.
func TestABracketTheJoinPutsBackComesOutOfTheMathematics(t *testing.T) {
	ch, pages := smallChapter()
	first := pages[18]
	first.Body = strings.Replace(first.Body,
		"2) Let M be an A-module with an infinite family of submodules. Then M is neither Artinian nor",
		"2) Let M be an A-module with an infinite family of submodules. (Reduce to the case when M is", 1)
	pages[18] = first
	second := pages[19]
	second.Body = strings.Replace(second.Body,
		"Noetherian.", `faithful and conclude by induction on $n.)$`, 1)
	pages[19] = second

	got, err := Chapter("alg", "en", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	if want := `by induction on $n.$)`; !strings.Contains(got[1].Body, want) {
		t.Errorf("§ 1 does not read %q:\n%s", want, got[1].Body)
	}
}

func TestChapter(t *testing.T) {
	ch, pages := smallChapter()
	got, err := Chapter("alg", "en", ch, pages)
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
	if want := []Run{{First: 18, Last: 18, FirstLabel: "A VIII.1", LastLabel: "A VIII.1"}}; !slices.Equal(front.Runs, want) {
		t.Errorf("the front matter runs %+v, want %+v", front.Runs, want)
	}
	if want := []Run{{First: 18, Last: 20, FirstLabel: "A VIII.1", LastLabel: "A VIII.3"}}; !slices.Equal(sec.Runs, want) {
		t.Errorf("§ 1 runs %+v, want %+v", sec.Runs, want)
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

// An exercise is cited by the page it is printed on, and in a volume with no
// page label that page is the folio. The 351 exercises of Lie 7 to 9 said
// nothing at all about where they are printed once the wrong label they used to
// carry was taken off them.
func TestAnExerciseWithNoPageLabelIsPlacedByItsFolio(t *testing.T) {
	ch, pages := smallChapter()
	got, err := Chapter("alg", "en", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	if p := got[1].Exercises[0].Meta.BookPage; p != "A VIII.3" {
		t.Errorf("exercise 1 is printed on %q, want the label of the page", p)
	}
	ch, pages = folioChapter()
	if got, err = Chapter("alg", "en", ch, pages); err != nil {
		t.Fatal(err)
	}
	if p := got[1].Exercises[0].Meta.BookPage; p != "17" {
		t.Errorf("exercise 1 is printed on %q, want the folio of the page", p)
	}
	// The page the exercises open on prints no head and so no number, which is
	// how all three chapters of Lie 7 to 9 open theirs.
	opening := pages[20]
	opening.Meta.Folio = 0
	pages[20] = opening
	if got, err = Chapter("alg", "en", ch, pages); err != nil {
		t.Fatal(err)
	}
	if p := got[1].Exercises[0].Meta.BookPage; p != "17" {
		t.Errorf("exercise 1 opening on an unnumbered page is printed on %q, want 17", p)
	}
}

// The exercises go out as one file each, so the section keeps the heading a
// cross-reference points at and a line saying where they went, and none of the
// text of them.
func TestChapterSplitsTheExercises(t *testing.T) {
	ch, pages := smallChapter()
	got, err := Chapter("alg", "en", ch, pages)
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
	got, err := Chapter("alg", "en", ch, pages)
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
	if _, err := Chapter("alg", "en", ch, pages); err == nil {
		t.Fatal("a footnote nothing marks should be an error")
	}
}

// The publisher's line is not part of the book and is not ours to republish.
func TestChapterDropsTheImprint(t *testing.T) {
	ch, pages := smallChapter()
	got, err := Chapter("alg", "en", ch, pages)
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
	got, err := Chapter("alg", "en", ch, pages)
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

	got, err := Chapter("alg", "en", ch, pages)
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
	if _, err := Chapter("alg", "en", ch, pages); err == nil {
		t.Fatal("a title the page disagrees with should be an error")
	}
	ch, pages = smallChapter()
	ch.Sections[0].PDFPage = 19
	if _, err := Chapter("alg", "en", ch, pages); err == nil {
		t.Fatal("a § on the wrong page should be an error")
	}
}

// Chapter VI of Lie 4 to 6 sets the heading of its § 2 near the foot of page
// 185 and its contents lists the § at 186, which is where the § 's first no. is
// listed as well. The contents is giving the page the § is read on rather than
// the page its heading is set on. Assembly has to take the heading where the
// printing put it, and the § and its first no. standing on the same page of the
// contents is what says the entry is that kind.
func TestChapterTakesASectionHeadingFromTheFootOfThePageBefore(t *testing.T) {
	ch, pages := smallChapter()
	ch.Sections[0].PDFPage = 19
	ch.Sections[0].Subsections[0].PDFPage = 19
	got, err := Chapter("alg", "en", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(got[1].Body, "## § 1. ARTINIAN MODULES AND NOETHERIAN MODULES") {
		t.Errorf("the § did not open on the page its heading is on:\n%.80s", got[1].Body)
	}

	// The same entry with the first no. listed earlier than the § is an entry
	// about the § itself, and a § the page it names does not carry is an error.
	ch, pages = smallChapter()
	ch.Sections[0].PDFPage = 19
	if _, err := Chapter("alg", "en", ch, pages); err == nil {
		t.Fatal("a § on the wrong page should be an error")
	}
}

// Chapter I of Theory of Sets, cut down to four pages. It heads its § by the
// number alone, "2. THEOREMS", where the other volumes print the sign, and it
// closes on an appendix that carries no number, the word on one line and the
// title under it. Both are the press's way of setting the same structure.
func setsChapter() (corpus.Chapter, map[int]corpus.PageFile) {
	ch := corpus.Chapter{
		Book: "ens-i-iv", Numeral: "I", Title: "DESCRIPTION OF FORMAL MATHEMATICS",
		Page: 15, PDFPage: 22,
		Sections: []corpus.Section{{
			Number: 2, Title: "Theorems", Page: 24, PDFPage: 22,
			Subsections: []corpus.Subsection{
				{Number: 1, Title: "THE AXIOMS", Page: 24, PDFPage: 22},
			},
		}, {
			Number: 0, Title: "Characterization of terms and relations", Page: 50, PDFPage: 23,
			Appendix: true,
			Subsections: []corpus.Subsection{
				{Number: 1, Title: "SIGNS AND WORDS", Page: 50, PDFPage: 23},
			},
		}},
	}
	pages := map[int]corpus.PageFile{
		22: page(22, "", false, strings.Join([]string{
			"## CHAPTER I Description of Formal Mathematics",
			"## 2. THEOREMS",
			"### 1. THE AXIOMS",
			"A theory is not the same thing as a proof.",
		}, "\n\n")),
		23: page(23, "", false, strings.Join([]string{
			"## APPENDIX",
			"## CHARACTERIZATION OF TERMS AND RELATIONS",
			"### 1. SIGNS AND WORDS",
			"Let the signs be given in some order.",
		}, "\n\n")),
		24: page(24, "", false, "# HISTORICAL NOTE"),
	}
	return ch, pages
}

func TestChapterReadsASectionHeadedByItsNumberAlone(t *testing.T) {
	ch, pages := setsChapter()
	got, err := Chapter("ens", "en", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d pieces, want the front matter, § 2 and the appendix", len(got))
	}
	if name := got[1].Name(); name != "§ 2" {
		t.Errorf("the second piece is %q, want § 2", name)
	}
	if !strings.Contains(got[1].Body, "A theory is not the same thing as a proof.") {
		t.Errorf("§ 2 does not carry its text: %q", got[1].Body)
	}
	if name := got[2].Name(); name != "Appendix 0" {
		t.Errorf("the third piece is %q, want the appendix", name)
	}
	if !strings.Contains(got[2].Body, "Let the signs be given in some order.") {
		t.Errorf("the appendix does not carry its text: %q", got[2].Body)
	}
}

// The bare form is still checked against the contents. A number that lands on
// the wrong title is the fault it was always there to catch, and taking the
// sign out of the heading does not take that check away.
func TestChapterRefusesABareSectionHeadingTheContentsDisagreesWith(t *testing.T) {
	ch, pages := setsChapter()
	ch.Sections[0].Title = "Proofs"
	if _, err := Chapter("ens", "en", ch, pages); err == nil {
		t.Fatal("a bare heading the contents disagrees with should be an error")
	}
}

// Not every appendix is given a title. Chapter IX of Algebre commutative
// chapitres 8 et 9 closes on one that prints APPENDICE centred and alone, with
// the heading of no. 1 as the next thing on the page and nothing in the contents
// after the word. Looking under the word for a title there picks up the no. and
// refuses a volume the contents already agreed with.
func TestChapterReadsAnAppendixWithNoTitle(t *testing.T) {
	ch, pages := setsChapter()
	ch.Sections[1].Title = ""
	pages[23] = page(23, "", false, strings.Join([]string{
		"## APPENDIX",
		"### 1. SIGNS AND WORDS",
		"Let the signs be given in some order.",
	}, "\n\n"))
	got, err := Chapter("ens", "en", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d pieces, want the front matter, § 2 and the appendix", len(got))
	}
	if name := got[2].Name(); name != "Appendix 0" {
		t.Errorf("the third piece is %q, want the appendix", name)
	}
	if !strings.Contains(got[2].Body, "Let the signs be given in some order.") {
		t.Errorf("the appendix does not carry its text: %q", got[2].Body)
	}
}

// Assembly reads the pages and never the PDF, so a page it has not been given
// is an error rather than a gap to write out.
func TestChapterRefusesAMissingPage(t *testing.T) {
	ch, pages := smallChapter()
	delete(pages, 19)
	if _, err := Chapter("alg", "en", ch, pages); err == nil {
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
		if got := joinable(c.prev, c.next, printings["en"]); got != c.want {
			t.Errorf("joinable(%q, %q) = %v, want %v", c.prev, c.next, got, c.want)
		}
	}
}

func TestMends(t *testing.T) {
	cases := []struct {
		prev, next string
		want       bool
	}{
		// The 88 junctions this places are all of this shape: the page before
		// stops in the middle of a sentence and the page after picks it up.
		{"for example undetermined letters. To", "simplify the exposition it is", true},
		{"then $S_1$ is weakly compa-", "tible in $z$ and $t$", true},
		// A full stop ends the paragraph, whatever the page after opens on.
		{"is neither Artinian nor Noetherian.", "the next thing", false},
		{"determined by this condition.*", "the next thing", false},
		{"(VIII, p. 267, exerc. 11).", "the next thing", false},
		{"$$x = y$$", "the next thing", false},
		// A stop set as mathematics is still a stop. Exercise 11 of § 7 of Lie
		// VIII ends its page on a colon inside the dollars and the page after
		// opens on the sum the colon introduces, and the dollar on the end hid
		// the one thing that said not to join them.
		{"with coefficients in $\\mathbf{Q}[\\Delta ]$)$:$", "$$P = \\sum a_i$$", false},
		{"in a variable T$:$", "the next thing", false},
		// A capital is where this stops, and it costs seven sentences broken at
		// a word set as mathematics to keep fourteen junctions from being run
		// together. See the note on mends.
		{"be the relations of the form", "$S_{i_1}$ and $S_{i_2}$", false},
		{"We have a commutative diagram", "ZL$^2(G)$", false},
		// The lettered parts of an exercise, and the head of a table.
		{"The set of minimal ideals of Z", "g) The set of maximal ideals", false},
		{"for every A-module M and", "– a B-linear homomorphism", false},
		{"the canonical mapping", "(i) *Let $(x^{(i)})$*", false},
		{"$\\varpi_8$ 1", "TABLE 2", false},
	}
	for _, c := range cases {
		if got := mends(c.prev, c.next); got != c.want {
			t.Errorf("mends(%q, %q) = %v, want %v", c.prev, c.next, got, c.want)
		}
	}
}

// Page 24 of Theory of Sets breaks a sentence at "To" and page 25 picks it up
// at "simplify", and neither page says so: the indent the reader would have
// taken continues from is not there to be read. The text is, and it is enough.
func TestJoinMendsAPageThatDoesNotSayItContinues(t *testing.T) {
	parts := []part{
		{page: 24, body: "the rules involve assemblies which are more or less undetermined, for example undetermined letters. To"},
		{page: 25, body: "simplify the exposition it is convenient to denote such assemblies by less cumbersome symbols."},
	}
	got, _ := join(parts, printings["en"])
	if len(got) != 1 {
		t.Fatalf("got %d blocks, want 1: %+v", len(got), got)
	}
	if !strings.Contains(got[0].text, "letters. To simplify the exposition") {
		t.Errorf("the sentence broken at the page break was not mended:\n%s", got[0].text)
	}
	if got[0].last != 25 {
		t.Errorf("the mended block ends on page %d, want 25", got[0].last)
	}
}

// The same two pages with a full stop where the break falls. Nothing says the
// paragraph runs on, so nothing joins it.
func TestJoinLeavesTwoParagraphsApart(t *testing.T) {
	parts := []part{
		{page: 24, body: "the rules involve assemblies which are more or less undetermined."},
		{page: 25, body: "we shall not enunciate strict general rules for the use of these symbols."},
	}
	if got, _ := join(parts, printings["en"]); len(got) != 2 {
		t.Fatalf("got %d blocks, want 2: %+v", len(got), got)
	}
}

// The same page with the same missing indent, opening on an exercise. Nothing
// here may join that: an exercise glued onto the page before it is an exercise
// the corpus has no file for.
func TestChapterDoesNotMendOntoAnExercise(t *testing.T) {
	ch, pages := smallChapter()
	p := pages[20]
	p.Body = strings.TrimPrefix(p.Body, "### Exercises\n\n")
	pages[20] = p
	ch.Sections[0].Exercises = nil

	got, err := Chapter("alg", "en", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got[1].Body, "\n\n1) Let A be a ring.") {
		t.Errorf("the exercise was mended onto the paragraph before it:\n%s", got[1].Body)
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
	got := cutExercises(in, 2, true, printings["en"])
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

// A chapter with one appendix does not number it. Chapter I of Theory of Sets
// prints the word alone, its table of contents gives it no numeral, and a mark
// that insisted on one stopped the whole volume assembling on a page that had
// been read perfectly well.
func TestTheMarkOfAnUnnumberedAppendix(t *testing.T) {
	mark := runMark(corpus.Section{Appendix: true})
	// ANNEX and ANNEXE stand beside the other two because the word is a choice
	// the volume makes and not one the language makes. Integration 7 to 9 heads
	// its one appendix ANNEX in English, in the running head, over the opener on
	// pdf page 284 and over the block of exercises on pdf page 300.
	for _, line := range []string{
		"## APPENDIX", "### Appendix", "#### appendix.", "## APPENDICE",
		"## ANNEX", "### Annex", "#### annex.", "## ANNEXE",
	} {
		if !mark.MatchString(line) {
			t.Errorf("%q is not read as the appendix mark", line)
		}
	}
	// And it is still a mark and not a word in a sentence.
	for _, line := range []string{"APPENDIX", "## Appendix I", "## Appendix to chapter I", "ANNEX", "## Annex to chapter IX"} {
		if mark.MatchString(line) {
			t.Errorf("%q is read as the mark of an unnumbered appendix", line)
		}
	}
	// A numbered one is unchanged, in either numeral and at any level.
	numbered := runMark(corpus.Section{Appendix: true, Number: 2})
	for _, line := range []string{"## Appendix 2", "### APPENDIX II", "## ANNEX 2", "### Annex II"} {
		if !numbered.MatchString(line) {
			t.Errorf("%q is not read as the mark of appendix 2", line)
		}
	}
	if numbered.MatchString("## Appendix") {
		t.Error("a chapter with two appendices reads a bare mark as the second one")
	}
}

// The mark of a § inside a block of gathered exercises is display type, and the
// reading sets display type bold. The bold was allowed for around the number and
// not around the whole mark, which is where page 583 of Algebre I a III puts it,
// and chapter III of that volume stopped assembling on a page carrying both of
// the marks it was said to be missing.
func TestTheMarkOfASectionIsReadThroughBold(t *testing.T) {
	mark := SectionMark(corpus.Section{Number: 9})
	for _, line := range []string{"§ 9", "§9", "§ 9.", "## § 9", "**§ 9**", "§ **9**", "#### § 9."} {
		if !mark.MatchString(line) {
			t.Errorf("%q is not read as the mark of § 9", line)
		}
	}
	// And it is still the sign and the number alone. The block is full of short
	// lines that are a number and nothing else.
	for _, line := range []string{"9", "§ 90", "**9**", "§ 9. ANNEAUX", "(§ 9)", "voir § 9"} {
		if mark.MatchString(line) {
			t.Errorf("%q is read as the mark of § 9", line)
		}
	}
	if SectionMark(corpus.Section{Number: 10}).MatchString("**§ 9**") {
		t.Error("the mark of § 10 reads the mark of § 9")
	}
}

// The heading over the opener is looked up by the words the printing knows and
// not by a pattern, so the two have to be kept in step. Integration 7 to 9 heads
// its appendix ANNEX and Algebra VIII heads its four APPENDIX, both in English
// off the same press, so a volume that prints either word has to be found.
func TestTheHeadsOverAnAppendixCoverBothWordsAndBothNumerals(t *testing.T) {
	for _, c := range []struct {
		lang string
		n    int
		want []string
	}{
		{"en", 0, []string{"APPENDIX", "ANNEX"}},
		{"en", 2, []string{"APPENDIX 2", "APPENDIX II", "ANNEX 2", "ANNEX II"}},
		{"fr", 0, []string{"APPENDICE", "ANNEXE"}},
		{"fr", 1, []string{"APPENDICE 1", "APPENDICE I", "ANNEXE 1", "ANNEXE I"}},
	} {
		// The level is crossed in as well as the word and the numeral. An
		// appendix is marked at whatever level extraction read off the size of
		// the type and that is not one level even inside a volume, so asking
		// for the word at the one level the printing was written down with
		// misses a page that read the type a size larger or smaller.
		var want []string
		for _, level := range []string{"#", "##", "###", "####"} {
			for _, w := range c.want {
				want = append(want, level+" "+w)
			}
		}
		got := printings[c.lang].appendixHeads(c.n)
		if len(got) != len(want) {
			t.Errorf("%s appendix %d has %d heads, want %d: %q", c.lang, c.n, len(got), len(want), got)
		}
		for _, w := range want {
			if !slices.Contains(got, w) {
				t.Errorf("%s appendix %d does not offer %q", c.lang, c.n, w)
			}
		}
	}
	// Every one of them is a mark the run knows, or the volume stops on a page
	// whose heading the contents was looked up by.
	for _, n := range []int{0, 1, 2} {
		for _, lang := range []string{"en", "fr"} {
			mark := runMark(corpus.Section{Appendix: true, Number: n})
			for _, head := range printings[lang].appendixHeads(n) {
				if !mark.MatchString(head) {
					t.Errorf("%s heads appendix %d %q and the run mark does not read it", lang, n, head)
				}
			}
		}
	}
}

// Bourbaki does not number its notes, it marks them with an asterisk, then two
// asterisks, then a dagger. The mark stands in the text and again at the head
// of the definition, and a reading that gave the mark and no reference used to
// stop the volume. Theory of Sets prints 51 notes and 30 came back that way.
func TestMarkNotesPutsTheReferenceBackWhereThePagePrintsTheMark(t *testing.T) {
	body := "The theory is contradictory (*) and nothing else follows."
	got := markNotes(body, []string{"[^1]: (*) This is the note."})
	if want := "The theory is contradictory (*)[^1] and nothing else follows."; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// The mark is written as the page set it, sometimes as mathematics and
// sometimes not, and the two are the same mark.
func TestMarkNotesReadsTheMarkAsMathematics(t *testing.T) {
	body := `A term of the theory $(\*)$ is an assembly.`
	got := markNotes(body, []string{`[^2]: (*) The note.`})
	if !strings.Contains(got, `$(\*)$[^2]`) {
		t.Errorf("the mathematical mark was not matched: %q", got)
	}
}

// Two things are left alone. A page that already marks the note is a page
// nothing has to be done to, and a page that sets the same mark twice gives no
// one place the reference belongs, so it goes to the check rather than to a
// guess.
func TestMarkNotesLeavesWhatItCannotPlace(t *testing.T) {
	marked := "already marked[^1] here (*) and there."
	if got := markNotes(marked, []string{"[^1]: (*) The note."}); got != marked {
		t.Errorf("a body that marks its note was changed: %q", got)
	}
	twice := "one (*) and two (*)."
	if got := markNotes(twice, []string{"[^1]: (*) The note."}); got != twice {
		t.Errorf("a mark the page sets twice was placed anyway: %q", got)
	}
}

// A piece can open partway down a page, and the note of the half above it is
// printed at the foot under both halves. Page 67 of Theory of Sets is the case:
// the last exercise of § 4 ends on the mark, APPENDIX opens two lines later,
// and the definition stands at the foot of the whole page.
func TestCutPageLeavesANoteWithTheHalfThatMarksIt(t *testing.T) {
	page := "The relation holds (*)\n\n## APPENDIX\n\nA characterization of terms.\n\n[^1]: (*) Take R to be the relation."
	top := cutPage(page, 0, strings.Index(page, "## APPENDIX"))
	if !strings.Contains(top, "[^1]: (*) Take R") {
		t.Errorf("the top of the page lost the note it marks:\n%s", top)
	}
	bottom := cutPage(page, strings.Index(page, "## APPENDIX"), len(page))
	if strings.Contains(bottom, "[^1]:") {
		t.Errorf("the bottom of the page kept a note nothing in it marks:\n%s", bottom)
	}
	if !strings.Contains(bottom, "A characterization of terms.") {
		t.Errorf("the bottom of the page lost its text:\n%s", bottom)
	}
}

// The definition opens on the mark it defines, so a note looked for in the
// whole cut finds itself and every half of every page keeps every note.
func TestCutPageDoesNotLetANoteMarkItself(t *testing.T) {
	page := "Nothing here refers to it.\n\n## APPENDIX\n\nThe text (*) of the appendix.\n\n[^1]: (*) The note."
	top := cutPage(page, 0, strings.Index(page, "## APPENDIX"))
	if strings.Contains(top, "[^1]:") {
		t.Errorf("the note followed the definition rather than the mark:\n%s", top)
	}
}

// A chapter of three §§ whose exercises are printed in one block at the end,
// with the block listed in the table of contents under the chapter and not
// under the §§. Two of the three are marked inside the block and the third is
// not, which is Topologie algebrique: § 4 of chapter II prints no exercises and
// the block says so by marking § 3 and then § 5.
func gatheredChapter() (corpus.Chapter, map[int]corpus.PageFile) {
	ch := corpus.Chapter{
		Book: "ta", Numeral: "I", Title: "Revetements", Page: 1, PDFPage: 18,
		Sections: []corpus.Section{
			{Number: 1, Title: "Produits Fibres", Page: 1, PDFPage: 18},
			{Number: 2, Title: "Applications Etales", Page: 2, PDFPage: 19},
			{Number: 3, Title: "Faisceaux", Page: 3, PDFPage: 20},
		},
		Exercises: &corpus.Locator{Page: 4, PDFPage: 21},
	}
	pages := map[int]corpus.PageFile{
		18: page(18, "A I.1", false, strings.Join([]string{
			"## CHAPTER I REVETEMENTS",
			"## § 1. PRODUITS FIBRES",
			"**Definition 1.** — A fibre product is what it is.",
		}, "\n\n")),
		19: page(19, "A I.2", false, strings.Join([]string{
			"## § 2. APPLICATIONS ETALES",
			"**Definition 2.** — An etale map is what it is.",
		}, "\n\n")),
		20: page(20, "A I.3", false, strings.Join([]string{
			"## § 3. FAISCEAUX",
			"**Definition 3.** — A sheaf is what it is.",
		}, "\n\n")),
		21: page(21, "A I.4", false, strings.Join([]string{
			"# EXERCISES",
			"§1",
			"1) Show that the fibre product is a limit.",
			"§3",
			"1) Show that a sheaf is a limit too.",
		}, "\n\n")),
		22: page(22, "", false, "# BIBLIOGRAPHY"),
	}
	return ch, pages
}

// The block is read from the page the contents names, each mark opens the run
// of the § it names, and a § the block does not mark gets nothing.
func TestChapterReadsAGatheredBlockTheContentsGivesToTheChapter(t *testing.T) {
	ch, pages := gatheredChapter()
	got, err := Chapter("ta", "en", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 4 {
		t.Fatalf("%d pieces, want the front matter and three §§", len(got))
	}
	for i, want := range []string{"Show that the fibre product is a limit.", "", "Show that a sheaf is a limit too."} {
		p := got[i+1]
		if want == "" {
			if len(p.Exercises) != 0 {
				t.Errorf("%s has %d exercises, and the block marks it nowhere", p.Name(), len(p.Exercises))
			}
			continue
		}
		if len(p.Exercises) != 1 {
			t.Errorf("%s has %d exercises, want 1", p.Name(), len(p.Exercises))
			continue
		}
		if body := p.Exercises[0].Body; !strings.HasPrefix(body, want) {
			t.Errorf("%s exercise 1 is %q, want %q", p.Name(), body, want)
		}
	}
}

// The § is given the locator the contents gave the chapter, on the page its own
// mark stands on, because that is what the rest of assembly asks for. The page
// prints its number in a label and not in a folio, so the number is the tail of
// the label.
func TestAGatheredBlockGivesEachSectionThePageItsMarkIsOn(t *testing.T) {
	ch, pages := gatheredChapter()
	got, err := Chapter("ta", "en", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range got[1:] {
		if p.Number == 2 {
			if p.Section.Exercises != nil {
				t.Errorf("§ 2 was given a locator: %+v", p.Section.Exercises)
			}
			continue
		}
		want := corpus.Locator{Page: 4, PDFPage: 21}
		if p.Section.Exercises == nil || *p.Section.Exercises != want {
			t.Errorf("%s has locator %+v, want %+v", p.Name(), p.Section.Exercises, want)
		}
	}
}

func TestPrintedNumber(t *testing.T) {
	if got := printedNumber(page(21, "A I.139", false, "")); got != 139 {
		t.Errorf("the number of A I.139 is %d, want 139", got)
	}
	folio := page(21, "", false, "")
	folio.Meta.Folio = 18
	if got := printedNumber(folio); got != 18 {
		t.Errorf("the number of a page with a folio is %d, want 18", got)
	}
	if got := printedNumber(page(21, "", false, "")); got != 0 {
		t.Errorf("a page that prints no number is %d, want 0", got)
	}
}

func TestSameTitle(t *testing.T) {
	same := []struct{ head, entry string }{
		{" DUAL OF A FRÉCHET SPACE", "Dual of a Fréchet space"},
		{" GEOMETRIC REPRESENTATION OF A COXETER GROUP", "The geometric representation of a Coxeter group"},
		{" FORM ASSOCIATED TO A COXETER GROUP", "The form associated to a Coxeter group"},
		{" The contragredient representation", "Contragredient representation"},
		{" REPRÉSENTATION GÉOMÉTRIQUE D'UN GROUPE DE COXETER", "La représentation géométrique d'un groupe de Coxeter"},
		{" REPRÉSENTATIONS LINÉAIRES", "Les représentations linéaires"},
		// Page 38 of Algebre I a III sets the preposition as a bare capital and
		// keeps the acute on the last word, so the grave is a matter of setting
		// the way the capitals are.
		{" GROUPES ET GROUPES A OPÉRATEURS", "Groupes et groupes à opérateurs"},
		// Page 69 of Groupes et algebres de Lie IX hangs a footnote marker off
		// the end of the heading and the contents entry does not carry one.
		{" REPRÉSENTATIONS IRRÉDUCTIBLES DES GROUPES DE LIE COMPACTS CONNEXES $ ^1 $",
			"Représentations irréductibles des groupes de Lie compacts connexes"},
		// Page 263 of Espaces vectoriels topologiques I a V does the same over
		// a chapter, with the marker inside the title rather than after it.
		{" Espaces hilbertiens $ ^1 $ (théorie élémentaire)", "ESPACES HILBERTIENS (THÉORIE ÉLÉMENTAIRE)"},
	}
	for _, c := range same {
		if !sameTitle(c.head, c.entry) {
			t.Errorf("sameTitle(%q, %q) is false, want true", c.head, c.entry)
		}
	}
	apart := []struct{ head, entry string }{
		{" DUAL OF A FRECHET SPACE", "Dual of a Fréchet space"},
		{" THEOREMS OF DENSITY", "Orems of density"},
		{" REFLECTIONS", "Pseudo-reflections"},
		{" A COXETER GROUP", "A Coxeter system"},
	}
	for _, c := range apart {
		if sameTitle(c.head, c.entry) {
			t.Errorf("sameTitle(%q, %q) is true, want false", c.head, c.entry)
		}
	}
}

// Three volumes are not divided into chapters. Elements d'histoire des
// mathematiques is a run of twenty seven historical notes and Varietes
// differentielles et analytiques is a fascicule de resultats, and neither sets
// a chapter heading anywhere, because neither has a chapter. The contents wraps
// each in one chapter all the same, since the assembler hangs everything off
// one, and that chapter is the manifest's own and not the printing's.
//
// Asked for a marker no page has, the assembler refused all three volumes and
// they came out of a build with nothing in them. Nominal says the chapter is
// not the printing's, and the marker and the front matter are then both what
// the printing says they are, which is absent.
func TestAChapterThePrintingDoesNotHaveOpensOnItsFirstSection(t *testing.T) {
	ch, pages := smallChapter()
	ch.Nominal = true
	first := pages[18]
	first.Body = strings.TrimPrefix(first.Body, "## CHAPTER VIII SEMISIMPLE MODULES AND RINGS\n\n")
	pages[18] = first

	got, err := Chapter("alg", "en", ch, pages)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d pieces, want § 1 alone", len(got))
	}
	if got[0].Front {
		t.Errorf("the one piece is the front matter: %+v", got[0])
	}
	if got, want := got[0].Name(), "§ 1"; got != want {
		t.Errorf("the one piece is %q, want %q", got, want)
	}
}

// The same chapter with the marker still on the page and nothing saying the
// chapter is nominal is the ordinary case, and it keeps its front matter.
func TestAChapterThePrintingDoesHaveStillWantsItsMarker(t *testing.T) {
	ch, pages := smallChapter()
	first := pages[18]
	first.Body = strings.TrimPrefix(first.Body, "## CHAPTER VIII SEMISIMPLE MODULES AND RINGS\n\n")
	pages[18] = first
	if _, err := Chapter("alg", "en", ch, pages); err == nil {
		t.Fatal("a chapter with no marker on its page assembled")
	}
}
