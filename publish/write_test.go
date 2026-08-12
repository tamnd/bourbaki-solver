package publish

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// testSite is two languages of one § and a second § to point at, built without
// touching a corpus. Load is not exercised here on purpose: what these tests
// are about is what comes out of the templates, and a fixture on disk would
// only put the reader further from the thing being asserted.
func testSite(t *testing.T) *Site {
	t.Helper()
	meta := func(lang string, n int, title string) corpus.SectionFrontMatter {
		return corpus.SectionFrontMatter{
			Book: "alg", BookTitle: "Algebra", Chapter: "VIII",
			ChapterTitle: "Semisimple Modules and Rings",
			Section:      n, SectionTitle: title, Lang: lang, Statements: 2,
			SourceEdition: "2023, Springer Nature", Extraction: "native",
		}
	}
	body := strings.Join([]string{
		"## § 14. Reduced Algebras",
		"",
		"Prose that opens the §.",
		"",
		"#### Theorem 1 {#alg-viii-s14-thm-1 .statement tag=00GJ}",
		"",
		"Let $x^*\\otimes y$ be the image of $x^*\\in E^*$.",
		"",
		"#### Lemma 2 {#alg-viii-s14-lem-2 .statement tag=00GO}",
		"",
		"By Theorem 1.",
		"",
		"See the [exercises for § 14](exercises/s14/).",
	}, "\n")

	s := &Site{Root: t.TempDir(), Langs: []string{"en", "fr"},
		Draft: map[string]bool{}, byTag: map[string]*Statement{}, byLabel: map[string]*Section{},
		byExTag: map[string]*Exercise{}, byExLabel: map[string]*Exercise{},
		CitedBy: map[string][]*Edge{}, Cites: map[string][]*Edge{}}
	for _, lang := range s.Langs {
		for _, m := range []corpus.SectionFrontMatter{meta(lang, 14, "Reduced Algebras"), meta(lang, 15, "Brauer Groups")} {
			sec := &Section{Lang: lang, Meta: m, Body: body, Slug: slug(m),
				Path: filepath.Join("content", lang, "alg", "VIII", m.SectionTitle+".md")}
			sec.Units = Units(sec.Body)
			s.Sections = append(s.Sections, sec)
		}
		// Two exercises of § 14, one of them starred, which is the shape the
		// chapter has: 76 of its 317 exercises carry a mark and the rest do not.
		for _, n := range []int{1, 8} {
			ex := &Exercise{Lang: lang, Dir: "s14", Body: "Prove that $A$ is reduced.",
				Path: filepath.Join("content", lang, "alg", "VIII", "exercises", "s14", fmt.Sprintf("%02d.md", n)),
				Meta: corpus.ExerciseFrontMatter{Book: "alg", Chapter: "VIII", Section: 14,
					Exercise: n, Label: fmt.Sprintf("alg-viii-s14-ex-%d", n), Lang: lang,
					Tag: map[int]string{1: "00H1", 8: "00H8"}[n], BookPage: "A VIII.92",
					Starred: n == 8}}
			s.Exercises = append(s.Exercises, ex)
		}
	}
	if err := s.index(); err != nil {
		t.Fatal(err)
	}
	return s
}

// The site is committed by a workflow and the check that it is up to date is a
// byte comparison, so a map iterated in place of a sorted list anywhere in here
// would show up as an unexplainable diff on an unrelated pull request.
func TestTwoBuildsAreByteIdentical(t *testing.T) {
	s := testSite(t)
	s.CitedBy["00GJ"] = []*Edge{
		{FromTag: "00GO", FromLabel: "alg-viii-s14-lem-2", ToTag: "00GJ", ToLabel: "alg-viii-s14-thm-1"},
		{FromLabel: "alg-viii-s15", ToTag: "00GJ", ToLabel: "alg-viii-s14-thm-1"},
	}
	s.Cites["00GO"] = []*Edge{
		{FromTag: "00GO", FromLabel: "alg-viii-s14-lem-2", ToTag: "00GJ",
			ToLabel: "alg-viii-s14-thm-1", Raw: "Theorem 1"},
	}

	one, two := t.TempDir(), t.TempDir()
	wrote, err := s.Build(one)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Build(two); err != nil {
		t.Fatal(err)
	}
	if len(wrote) == 0 {
		t.Fatal("wrote nothing")
	}
	for _, rel := range wrote {
		a, err := os.ReadFile(filepath.Join(one, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		b, err := os.ReadFile(filepath.Join(two, filepath.FromSlash(rel)))
		if err != nil {
			t.Fatal(err)
		}
		if string(a) != string(b) {
			t.Errorf("%s differs between two builds of the same corpus", rel)
		}
	}
}

// A reference from a whole § is a real citation with a real target, and a §
// has no tag to point at. Left alone it came out as "not in this corpus",
// which is a page telling a reader something false about a file it just read.
func TestAReferenceFromASectionIsLinkedAtTheSection(t *testing.T) {
	s := testSite(t)
	s.CitedBy["00GJ"] = []*Edge{{FromLabel: "alg-viii-s15", ToTag: "00GJ", ToLabel: "alg-viii-s14-thm-1"}}

	got := s.mustPage(t, "tag/00GJ/index.html")
	if !strings.Contains(got, `<a href="/en/alg/VIII/s15/" title="alg-viii-s15">§ 15. Brauer Groups</a>`) {
		t.Errorf("the § is not linked:\n%s", got)
	}
	if strings.Contains(got, "not in this corpus") {
		t.Errorf("the § is called missing:\n%s", got)
	}
}

// An exercise citing a theorem is a row on the theorem's page, and it is a link
// now that the exercise has a page. It reads as "Exercise 8, § 14" rather than
// as its label, because every § has an Exercise 8.
func TestAnExerciseCitingAStatementIsLinked(t *testing.T) {
	s := testSite(t)
	s.CitedBy["00GJ"] = []*Edge{{FromLabel: "alg-viii-s14-ex-8", ToTag: "00GJ", ToLabel: "alg-viii-s14-thm-1"}}

	got := s.mustPage(t, "tag/00GJ/index.html")
	if strings.Contains(got, "not in this corpus") || strings.Contains(got, "does not hold") {
		t.Errorf("an exercise is called missing:\n%s", got)
	}
	if !strings.Contains(got, `<a href="/en/alg/VIII/s14/ex/8/" title="alg-viii-s14-ex-8">Exercise 8, § 14</a>`) {
		t.Errorf("the exercise is not linked:\n%s", got)
	}
}

// An exercise the corpus does not hold says so, and says it differently from a
// citation of a Book nobody has transcribed. The two look the same on the page
// and are fixed by different work.
func TestAnExerciseTheCorpusLacksSaysWhichKindOfGapItIs(t *testing.T) {
	s := testSite(t)
	s.CitedBy["00GJ"] = []*Edge{{FromLabel: "alg-viii-s14-ex-99", ToTag: "00GJ", ToLabel: "alg-viii-s14-thm-1"}}

	got := s.mustPage(t, "tag/00GJ/index.html")
	if !strings.Contains(got, "an exercise the corpus does not hold") {
		t.Errorf("want the gap named in:\n%s", got)
	}
}

// The page the whole step is for. The disclosure is written whether or not there
// is a solution, so that landing the solutions of M8 is a content change and not
// a template change, and it is closed, so a reader who wants to think about the
// exercise first can.
func TestAnExerciseHasAPageWithAnEmptyDisclosure(t *testing.T) {
	s := testSite(t)
	got := s.mustPage(t, "en/alg/VIII/s14/ex/8/index.html")
	for _, want := range []string{
		"<h1>Exercise 8, § 14</h1>",
		`<p class="tagline">Tag <code>00H8</code>`,
		"Algebra, VIII, p. 92",
		"Bourbaki marks this one as harder than the rest.",
		"<summary>Solution</summary>",
		"No solution has been written yet.",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the exercise page does not carry %q:\n%s", want, got)
		}
	}
	// <details> without open, which is the whole point of using the element.
	if strings.Contains(got, "<details class=\"solution\" open") {
		t.Errorf("the solution is open on load:\n%s", got)
	}
	// An unstarred exercise says nothing rather than saying it is not starred.
	if plain := s.mustPage(t, "en/alg/VIII/s14/ex/1/index.html"); strings.Contains(plain, "harder than the rest") {
		t.Errorf("an unmarked exercise is marked:\n%s", plain)
	}
}

// The list a § points at. The marks are the short form here, because the
// sentence the exercise's own page carries is nineteen repetitions of itself on
// a page of nineteen rows.
func TestTheExerciseListNamesWhatIsInIt(t *testing.T) {
	s := testSite(t)
	got := s.mustPage(t, "en/alg/VIII/s14/ex/index.html")
	for _, want := range []string{
		"<h1>Exercises for § 14. Reduced Algebras</h1>",
		"2 exercises. No solution has been written for any of them yet.",
		`<li><a href="/en/alg/VIII/s14/ex/1/">Exercise 1</a></li>`,
		`<li><a href="/en/alg/VIII/s14/ex/8/">Exercise 8</a> <span class="count">harder</span></li>`,
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the list does not carry %q:\n%s", want, got)
		}
	}
}

// A solution is machine-written and machine-judged, and the note saying so is
// inside the disclosure and above the solution, so it cannot be read past.
func TestASolutionCarriesTheStandingNote(t *testing.T) {
	s := testSite(t)
	for _, ex := range s.Exercises {
		if ex.Meta.Exercise == 8 && ex.Lang == "en" {
			ex.Solution = &Solution{Body: "Because $A$ has no nilpotents.",
				Path: "content/solutions/en/alg/VIII/s14/08.md",
				Meta: corpus.SolutionFrontMatter{Label: ex.Meta.Label, Lang: "en",
					Status: corpus.StatusVerified, Model: "gpt-5.6"}}
		}
	}
	got := s.mustPage(t, "en/alg/VIII/s14/ex/8/index.html")
	note := "This solution was written by gpt-5.6 and judged by a machine. " +
		"It is not Bourbaki&#39;s and it has not been checked by a person."
	if !strings.Contains(got, note) {
		t.Errorf("the standing note is not on the page:\n%s", got)
	}
	if strings.Index(got, note) > strings.Index(got, "no nilpotents") {
		t.Error("the note is under the solution rather than above it")
	}
	if strings.Contains(got, "No solution has been written yet") {
		t.Errorf("the page says both that there is a solution and that there is not:\n%s", got)
	}
}

// The corpus links its own exercises the way the repository holds them, so that
// the link works when the file is read on GitHub. The site hangs them off the §.
// Passing the corpus path through unchanged is where the 49 broken links on the
// first build came from.
func TestTheCorpusLinkToItsExercisesLandsOnTheSite(t *testing.T) {
	s := testSite(t)
	got := s.mustPage(t, "en/alg/VIII/s14/index.html")
	if !strings.Contains(got, `<a href="/en/alg/VIII/s14/ex/">exercises for § 14</a>`) {
		t.Errorf("the corpus link was not mapped:\n%s", got)
	}
	if strings.Contains(got, "/en/alg/VIII/exercises/") {
		t.Errorf("the repository path is on the page as a URL:\n%s", got)
	}
}

// An exercise carries a tag as a statement does, and a tag that resolves to
// nothing is the one thing the tag scheme promises will not happen. The page is
// the exercise's own, so the second URL says which one is canonical.
func TestAnExerciseTagResolves(t *testing.T) {
	s := testSite(t)
	got := s.mustPage(t, "tag/00H8/index.html")
	if !strings.Contains(got, "<h1>Exercise 8, § 14</h1>") {
		t.Errorf("the tag page is not the exercise:\n%s", got)
	}
	if !strings.Contains(got, `<link rel="canonical" href="/en/alg/VIII/s14/ex/8/">`) {
		t.Errorf("the copy does not name the original:\n%s", got)
	}
	// And the original does not name itself, which would be noise on every one
	// of the seven hundred pages that are not copies.
	if page := s.mustPage(t, "en/alg/VIII/s14/ex/8/index.html"); strings.Contains(page, "rel=\"canonical\"") {
		t.Errorf("the exercise page carries a canonical link to itself:\n%s", page)
	}
}

// Two things under one tag is the failure the whole scheme exists to prevent,
// and left alone it shows up as a page written twice with whichever came last
// winning, which is the quietest way for it to go wrong.
func TestATagOnAStatementAndAnExerciseIsRefused(t *testing.T) {
	s := testSite(t)
	s.Exercises[0].Meta.Tag = "00GJ"
	s.byExTag, s.byExLabel = map[string]*Exercise{}, map[string]*Exercise{}
	if err := s.index(); err == nil {
		t.Fatal("a tag on two different things was accepted")
	}
}

// GitHub Pages serves a project site under the repository name, and a link
// written for the root of a domain is a link to nothing there.
func TestABaseMovesEveryLink(t *testing.T) {
	s := testSite(t)
	s.Base = "/bourbaki"
	got := s.mustPage(t, "tag/00GJ/index.html")
	for _, line := range strings.Split(got, "\n") {
		for _, attr := range []string{`href="`, `src="`} {
			for _, part := range strings.Split(line, attr)[1:] {
				url := part[:strings.Index(part, `"`)]
				if strings.HasPrefix(url, "/") && !strings.HasPrefix(url, "/bourbaki/") {
					t.Errorf("%s is outside the base", url)
				}
			}
		}
	}
}

// Every page ends in a slash and every link is absolute, so a link from a tag
// page three deep and the same link from a section page four deep are the same
// string.
func TestURLsAreAbsoluteAndEndInASlash(t *testing.T) {
	s := testSite(t)
	for _, url := range []string{s.url(), s.TagURL("00GJ"), s.ChapterURL("en", "alg", "VIII"),
		s.SectionURL(s.Sections[0])} {
		if !strings.HasPrefix(url, "/") || !strings.HasSuffix(url, "/") {
			t.Errorf("%q is not an absolute directory URL", url)
		}
	}
	if got := s.url(); got != "/" {
		t.Errorf("the root of a site served at the root of a domain is %q", got)
	}
}

// The statement and the § it is in are both named, because the corpus has 51
// headings that say only "Corollary" and a page of them all called Corollary
// is a page nobody can use.
func TestAStatementIsNamedWithItsSection(t *testing.T) {
	s := testSite(t)
	if got := s.name(s.Tag("00GO")); got != "Lemma 2, § 14" {
		t.Errorf("a statement is named %q", got)
	}
}

// The line saying who wrote the text is the one thing on the page a reader
// needs before deciding whether to trust the mathematics.
func TestEveryPageSaysWhoWroteIt(t *testing.T) {
	s := testSite(t)
	got := s.mustPage(t, "en/alg/VIII/s14/index.html")
	if !strings.Contains(got, "Transcribed from the printed edition, 2023, Springer Nature. Not translated.") {
		t.Errorf("no provenance on a section page:\n%s", got)
	}

	tr := *s.Sections[0]
	tr.Meta.TranslatedFrom = "content/en/alg/VIII/14.md"
	tr.Meta.TranslationModel = "gpt-5.6"
	tr.Meta.GlossaryVersion = 3
	want := "Machine translation of the English, by gpt-5.6, against glossary version 3. Not checked by a person."
	if got := provenance(&tr); got != want {
		t.Errorf("a translated page says\n%s\nwant\n%s", got, want)
	}
}

// The mathematics is rendered here, so the fonts it is set in have to be part
// of the site. A page that links a stylesheet the deployment does not carry
// falls back to the browser's serif at the wrong size, which looks like a
// rendering bug and is a packaging one.
func TestTheFontsShipWithTheSite(t *testing.T) {
	s := testSite(t)
	dir := t.TempDir()
	wrote, err := s.Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	fonts := 0
	css := false
	for _, rel := range wrote {
		switch {
		case rel == "katex/katex.min.css":
			css = true
		case strings.HasPrefix(rel, "katex/fonts/"):
			fonts++
		}
	}
	if !css || fonts == 0 {
		t.Fatalf("the site carries the stylesheet %v and %d fonts", css, fonts)
	}
	page := s.mustPage(t, "en/alg/VIII/s14/index.html")
	if !strings.Contains(page, `<link rel="stylesheet" href="/katex/katex.min.css">`) {
		t.Errorf("a page does not link the stylesheet:\n%s", page)
	}
	// The stylesheet asks for its fonts relative to itself, so the two have to
	// stay at these depths relative to each other.
	b, err := os.ReadFile(filepath.Join(dir, "katex", "katex.min.css"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "url(fonts/") {
		t.Error("the stylesheet does not reach its fonts from where it was written")
	}
}

// The escape hatch, and the two things it has to be: off by default, and loud
// when it is on. A build that quietly printed the TeX of a formula the
// extraction lost characters out of would put the fault in front of a reader in
// a form that looks like a decision somebody took.
func TestBrokenMathIsMarkedAndOnlyWhenAskedFor(t *testing.T) {
	s := testSite(t)
	broken := `Then $\frac{1$ and the rest.`
	for _, sec := range s.Sections {
		sec.Body += "\n\n" + broken
		sec.Units = Units(sec.Body)
	}

	if _, err := s.Build(t.TempDir()); err == nil {
		t.Fatal("a broken formula built without complaint")
	}
	if len(s.Broken) != 0 {
		t.Errorf("%d formulae were marked without being asked for", len(s.Broken))
	}

	s.AllowBrokenMath = true
	got := s.mustPage(t, "en/alg/VIII/s14/index.html")
	if !strings.Contains(got, `class="math broken"`) {
		t.Errorf("the formula is not marked:\n%s", got)
	}
	if !strings.Contains(got, `$\frac{1$`) {
		t.Errorf("the source of the formula is not shown:\n%s", got)
	}
	if len(s.Broken) == 0 {
		t.Error("the build did not say what it marked")
	}
}

// Spec 12 §6. The index is what the search is, and the one thing that is easy
// to get wrong in it is the mathematics: dropped, and a great many statements
// have nothing searchable left in them.
func TestTheSearchIndexKeepsTheMathematicsAsItsSource(t *testing.T) {
	for _, c := range []struct{ md, want string }{
		{"Let $x^*\\otimes y$ be the image.", "Let x^*\\otimes y be the image."},
		{"A **reduced** algebra over $k$.", "A reduced algebra over k ."},
		{"See the [exercises for § 14](exercises/s14/).", "See the exercises for § 14."},
		{"#### Theorem 1 {#alg-viii-s14-thm-1 .statement tag=00GJ}", "Theorem 1"},
		{"$$\\sum_{i\\in I} a_i e_i$$", "\\sum_{i\\in I} a_i e_i"},
		{"- one\n- two", "one two"},
	} {
		if got := plain(c.md); got != c.want {
			t.Errorf("plain(%q) is\n%q\nwant\n%q", c.md, got, c.want)
		}
	}
}

// One entry per tagged statement and one per exercise, since the exercises are
// a third of the tags and a search that could not find them would be a search of
// two thirds of the site.
func TestTheSearchIndexHoldsTheStatementsAndTheExercises(t *testing.T) {
	s := testSite(t)
	en := s.searchIndex("en")
	if len(en) != 4 {
		t.Fatalf("%d entries, want two statements and two exercises: %+v", len(en), en)
	}
	var stmt, ex *Entry
	for i := range en {
		switch en[i].Tag {
		case "00GJ":
			stmt = &en[i]
		case "00H8":
			ex = &en[i]
		}
	}
	if stmt == nil || ex == nil {
		t.Fatalf("the statement or the exercise is not in the index: %+v", en)
	}
	// A statement is found at its tag, which is the URL the tag scheme promises.
	if stmt.URL != "/tag/00GJ/" || stmt.Heading != "Theorem 1" || stmt.Section != "§ 14. Reduced Algebras" {
		t.Errorf("the statement entry is %+v", *stmt)
	}
	// An exercise is found at its own page rather than at the tag page, which is
	// the same bytes and is the copy that declares the other canonical.
	if ex.URL != "/en/alg/VIII/s14/ex/8/" || ex.Heading != "Exercise 8, § 14" {
		t.Errorf("the exercise entry is %+v", *ex)
	}
	if !strings.Contains(stmt.Text, "x^*\\otimes y") {
		t.Errorf("the mathematics is not in the text: %q", stmt.Text)
	}

	// One file per language and not one for the site, because three languages in
	// one file is three megabytes fetched to search one of them.
	dir := t.TempDir()
	wrote, err := s.Build(dir)
	if err != nil {
		t.Fatal(err)
	}
	var jsons []string
	for _, w := range wrote {
		if strings.HasSuffix(w, ".json") {
			jsons = append(jsons, w)
		}
	}
	if strings.Join(jsons, " ") != "search/en.json search/fr.json" {
		t.Errorf("the indexes written are %v", jsons)
	}
	var back []Entry
	b, err := os.ReadFile(filepath.Join(dir, "search", "en.json"))
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, &back); err != nil {
		t.Fatalf("the index is not JSON: %v", err)
	}
	if len(back) != len(en) {
		t.Errorf("%d entries were written and %d were built", len(back), len(en))
	}
}

// The search page is reachable from everywhere, and it is the only page that
// needs a script. A reader with scripts off is told that and is not left with a
// box that does nothing.
func TestSearchIsLinkedFromEveryPageAndSaysItNeedsAScript(t *testing.T) {
	s := testSite(t)
	for _, rel := range []string{"index.html", "en/alg/VIII/s14/index.html", "tag/00GJ/index.html",
		"en/alg/VIII/s14/ex/8/index.html", "tags/index.html"} {
		if got := s.mustPage(t, rel); !strings.Contains(got, `<a href="/search/">Search</a>`) {
			t.Errorf("%s does not link the search page:\n%s", rel, got)
		}
	}
	got := s.mustPage(t, "search/index.html")
	if !strings.Contains(got, "<noscript>") {
		t.Errorf("the search page does not say it needs a script:\n%s", got)
	}
	if !strings.Contains(got, `<script>`) || !strings.Contains(got, `const BASE = "/search/";`) {
		t.Errorf("the search page does not know where its indexes are:\n%s", got)
	}
	// And nowhere else. Everything the site is for is plain HTML.
	for _, rel := range []string{"en/alg/VIII/s14/index.html", "tag/00GJ/index.html",
		"en/alg/VIII/s14/ex/8/index.html"} {
		if strings.Contains(s.mustPage(t, rel), "<script") {
			t.Errorf("%s carries a script", rel)
		}
	}
}

// withDraft adds a language holding one § of the two and one exercise of the
// two, which is the shape the Vietnamese has: a translation that has started
// and is a long way from the end of the chapter.
func withDraft(t *testing.T, s *Site) *Site {
	t.Helper()
	en := s.Sections[0]
	m := en.Meta
	m.Lang = "vi"
	m.TranslatedFrom = "content/en/alg/VIII/Reduced Algebras.md"
	m.TranslationModel = "gpt-5-6-mini"
	m.GlossaryVersion = 5
	// The § stops before Lemma 2, so that one statement of it is held in three
	// languages and the other in two. A part-translated file is not a shape the
	// corpus has, and a §§ of a chapter translated and the rest not is exactly
	// its shape; this is the small version of that.
	body, _, _ := strings.Cut(en.Body, "#### Lemma 2")
	sec := &Section{Lang: "vi", Meta: m, Body: body, Slug: slug(m),
		Path: filepath.Join("content", "vi", "alg", "VIII", m.SectionTitle+".md")}
	sec.Units = Units(sec.Body)
	s.Sections = append(s.Sections, sec)

	ex := *s.Exercises[0]
	ex.Lang = "vi"
	ex.Meta.Lang = "vi"
	ex.Meta.TranslatedFrom = "content/en/alg/VIII/exercises/s14/01.md"
	ex.Meta.TranslationModel = "gpt-5-6-mini"
	ex.Path = filepath.Join("content", "vi", "alg", "VIII", "exercises", "s14", "01.md")
	s.Exercises = append(s.Exercises, &ex)

	s.Langs = append(s.Langs, "vi")
	// The fixture is two §§ where the corpus is twenty seven, so the count that
	// puts Vietnamese under the floor is set rather than counted. What floor
	// itself does is tested on its own.
	s.Draft["vi"] = true
	if err := s.index(); err != nil {
		t.Fatal(err)
	}
	return s
}

// Spec 12 §7. Vietnamese has two §§ of twenty seven, so a switcher that offered
// it everywhere would be a dead end twenty five times out of twenty seven.
func TestALanguageUnderTheFloorIsNotOfferedUnlessAskedFor(t *testing.T) {
	s := withDraft(t, testSite(t))

	en := s.mustPage(t, "en/alg/VIII/s14/index.html")
	if strings.Contains(en, `href="/vi/alg/VIII/s14/"`) {
		t.Errorf("a language under the floor is in the switcher of an English page:\n%s", en)
	}
	// Built and reachable, which is the other half of the rule: the floor keeps
	// a language out of the navigation and does not keep it off the site.
	vi := s.mustPage(t, "vi/alg/VIII/s14/index.html")
	if !strings.Contains(vi, `<span class="here">vi</span>`) ||
		!strings.Contains(vi, `href="/en/alg/VIII/s14/"`) {
		t.Errorf("the page of a draft language does not say what it is or where the English is:\n%s", vi)
	}

	s.Drafts = true
	en = s.mustPage(t, "en/alg/VIII/s14/index.html")
	if !strings.Contains(en, `<a class="draft" href="/vi/alg/VIII/s14/"`) {
		t.Errorf("-drafts did not put the language back in the switcher:\n%s", en)
	}
}

// The other half of §7, which holds whether or not the language is a draft: a
// language is offered on a page only where it has that page.
func TestALanguageIsOfferedOnlyWhereItHoldsThePage(t *testing.T) {
	s := withDraft(t, testSite(t))
	s.Drafts = true

	for _, rel := range []string{
		"en/alg/VIII/s15/index.html",      // Vietnamese has § 14 and not § 15
		"en/alg/VIII/s14/ex/8/index.html", // and exercise 1 and not exercise 8
		"tag/00GO/index.html",             // and Theorem 1 and not Lemma 2
	} {
		if got := s.mustPage(t, rel); strings.Contains(got, `>vi</a>`) {
			t.Errorf("%s offers a language that does not hold it:\n%s", rel, got)
		}
	}
	for _, rel := range []string{"en/alg/VIII/s14/ex/1/index.html", "tag/00GJ/index.html"} {
		if got := s.mustPage(t, rel); !strings.Contains(got, `>vi</a>`) {
			t.Errorf("%s does not offer the language that holds it:\n%s", rel, got)
		}
	}
}

// The floor is a share of the English and not a count, so that it means the
// same thing when the corpus is one chapter and when it is ten.
func TestTheFloorIsAShareOfTheEnglish(t *testing.T) {
	s := &Site{Draft: map[string]bool{}, Langs: []string{"en", "fr", "vi"}}
	for i := 0; i < 27; i++ {
		s.Sections = append(s.Sections, &Section{Lang: "en"})
	}
	for i := 0; i < 27; i++ {
		s.Sections = append(s.Sections, &Section{Lang: "fr"})
	}
	for i := 0; i < 2; i++ {
		s.Sections = append(s.Sections, &Section{Lang: "vi"})
	}
	if err := s.floor(); err != nil {
		t.Fatal(err)
	}
	if s.Draft["fr"] {
		t.Error("a language with all of the chapter is under the floor")
	}
	if !s.Draft["vi"] {
		t.Error("a language with two §§ of twenty seven is not under the floor")
	}
	if s.Draft["en"] {
		t.Error("English is measured against itself and came out short")
	}
}

// L08 knows a translation was written by a cut down model and says so as a soft
// finding. The reader of the page has more right to that than the audit does.
func TestACutDownModelIsSaidOnThePage(t *testing.T) {
	s := withDraft(t, testSite(t))
	want := "cut down version"
	if got := s.mustPage(t, "vi/alg/VIII/s14/index.html"); !strings.Contains(got, want) {
		t.Errorf("a section written by a small model does not say so:\n%s", got)
	}
	if got := s.mustPage(t, "vi/alg/VIII/s14/ex/1/index.html"); !strings.Contains(got, want) {
		t.Errorf("an exercise written by a small model does not say so:\n%s", got)
	}
	// And not on the pages it is not true of, which is the whole chapter but two
	// files.
	if got := s.mustPage(t, "en/alg/VIII/s14/index.html"); strings.Contains(got, want) {
		t.Errorf("a transcription is called a cut down translation:\n%s", got)
	}
}

// A typed -lang vn is a build of English that looks like a build of Vietnamese
// which came out empty, so it is an error instead.
func TestALanguageTheCorpusLacksIsAnError(t *testing.T) {
	if _, err := keep([]string{"en", "fr", "vi"}, []string{"vn"}); err == nil {
		t.Error("a language the corpus does not hold was accepted")
	}
	got, err := keep([]string{"en", "fr", "vi"}, []string{"vi", "en"})
	if err != nil {
		t.Fatal(err)
	}
	// English first, as the whole site orders languages, and not in the order it
	// was asked for.
	if strings.Join(got, ",") != "en,vi" {
		t.Errorf("-lang reordered the languages: %v", got)
	}
}

// Spec 12 §4.4: the coverage table is generated and not typed. The way to test
// that is to move the corpus and watch the number move, since a page with the
// right number hard coded in it passes every assertion about the right number.
func TestTheAboutPageCountsTheCorpusRatherThanClaimingIt(t *testing.T) {
	s := withDraft(t, testSite(t))
	got := s.mustPage(t, "about/index.html")
	// One § of the two the English holds.
	if !strings.Contains(got, "<td>50%</td>") {
		t.Errorf("the coverage table does not hold the share of the English:\n%s", got)
	}

	en := *s.Sections[0]
	en.Meta.Section = 16
	en.Slug = slug(en.Meta)
	s.Sections = append(s.Sections, &en)
	if err := s.index(); err != nil {
		t.Fatal(err)
	}
	got = s.mustPage(t, "about/index.html")
	if strings.Contains(got, "<td>50%</td>") || !strings.Contains(got, "<td>33%</td>") {
		t.Errorf("the share did not follow the corpus:\n%s", got)
	}
}

// Spec 12 §7: a language under the floor is listed on /about/ with its real
// numbers. It is kept out of the switcher, and a language kept out of the
// switcher and off the about page as well would be a language the site holds
// and nobody can find.
func TestALanguageUnderTheFloorIsListedOnAboutWithItsNumbers(t *testing.T) {
	s := withDraft(t, testSite(t))
	got := s.mustPage(t, "about/index.html")
	if !strings.Contains(got, `<a href="/vi/alg/VIII/">vi</a>`) {
		t.Errorf("a language under the floor is not linked from the about page:\n%s", got)
	}
	if !strings.Contains(got, "under the coverage floor") ||
		!strings.Contains(got, "20 per cent") {
		t.Errorf("the about page does not say what the floor is:\n%s", got)
	}
}

// Spec 12 §7 again, gathered: the model that wrote a translation and the
// glossary it was held to. The pages say it one at a time and this says it for
// the corpus, which is the only place a reader sees that two models were used.
func TestTheAboutPageNamesEveryModelThatWroteAPage(t *testing.T) {
	s := withDraft(t, testSite(t))
	// A second Vietnamese §, written in a later run by the full model, which is
	// what the corpus itself holds.
	vi := *s.Sections[len(s.Sections)-1]
	vi.Meta.Section = 15
	vi.Meta.TranslationModel = "gpt-5-6"
	vi.Meta.GlossaryVersion = 14
	vi.Slug = slug(vi.Meta)
	s.Sections = append(s.Sections, &vi)
	if err := s.index(); err != nil {
		t.Fatal(err)
	}

	got := s.mustPage(t, "about/index.html")
	for _, want := range []string{"gpt-5-6 and gpt-5-6-mini", "versions 5 and 14", "cut down version"} {
		if !strings.Contains(got, want) {
			t.Errorf("the about page does not say %q:\n%s", want, got)
		}
	}
	// And the transcriptions are not described as anything a model wrote, which
	// is the distinction the whole section is for.
	if !strings.Contains(got, "No model wrote those sentences") {
		t.Errorf("the about page does not separate the transcription from the translation:\n%s", got)
	}
}

// The claim about where the text came from is counted out of the front matter
// rather than asserted, because the day a file is extracted some other way the
// sentence has to change with it.
func TestTheAboutPageDoesNotCallAnOCRedSectionNative(t *testing.T) {
	s := testSite(t)
	s.Sections[0].Meta.Extraction = "ocr"
	s.Sections[0].Meta.ExtractionModel = "gpt-5-6"
	got := s.mustPage(t, "about/index.html")
	if strings.Contains(got, "with no model in the path at all") {
		t.Errorf("a section a model read is counted as native:\n%s", got)
	}
	if !strings.Contains(got, "3 native and 1 ocr") {
		t.Errorf("the about page does not count the extraction methods:\n%s", got)
	}
}

// The unflattering numbers go up with the rest, which is the rule spec 12 §4.4
// takes from design principle 6 of spec 00 and points at the public.
func TestTheAboutPagePublishesTheNumbersThatDoNotFlatterIt(t *testing.T) {
	s := testSite(t)
	s.Edges, s.Unresolved = 2122, 149
	s.CitedBy["00GJ"] = []*Edge{{FromTag: "00GO", FromLabel: "alg-viii-s14-lem-2",
		ToTag: "00GJ", ToLabel: "alg-viii-s14-thm-1"}}

	got := s.mustPage(t, "about/index.html")
	for _, want := range []string{
		"<td>2122</td>",   // references found
		"<td>149</td>",    // and the ones that do not resolve
		"<td>1 of 2</td>", // cited by nothing: 00GJ is cited, 00GO is not
		"<td>2 of 2</td>", // and neither has a printed page in the fixture
	} {
		if !strings.Contains(got, want) {
			t.Errorf("the about page does not publish %q:\n%s", want, got)
		}
	}
	if got := s.mustPage(t, "index.html"); !strings.Contains(got, "149 do not resolve") {
		t.Errorf("the front page does not publish the references that do not resolve:\n%s", got)
	}
}

// P07 is that every page showing machine-written text says so. The line at the
// foot says it for the page and the about page says it for the corpus, so every
// page has to reach it in one click.
func TestTheAboutPageIsLinkedFromEveryPage(t *testing.T) {
	s := withDraft(t, testSite(t))
	for _, rel := range []string{"index.html", "about/index.html", "search/index.html",
		"tags/index.html", "en/alg/VIII/index.html", "en/alg/VIII/s14/index.html",
		"tag/00GJ/index.html", "en/alg/VIII/s14/ex/8/index.html", "vi/alg/VIII/s14/index.html"} {
		if got := s.mustPage(t, rel); !strings.Contains(got, `<a href="/about/">About</a>`) {
			t.Errorf("%s does not link the about page:\n%s", rel, got)
		}
	}
}

func (s *Site) mustPage(t *testing.T, rel string) string {
	t.Helper()
	dir := t.TempDir()
	if _, err := s.Build(dir); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}
