package publish

import (
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
			SourceEdition: "2023, Springer Nature",
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
	}, "\n")

	s := &Site{Root: t.TempDir(), Langs: []string{"en", "fr"},
		Draft: map[string]bool{}, byTag: map[string]*Statement{}, byLabel: map[string]*Section{},
		CitedBy: map[string][]*Edge{}, Cites: map[string][]*Edge{}}
	for _, lang := range s.Langs {
		for _, m := range []corpus.SectionFrontMatter{meta(lang, 14, "Reduced Algebras"), meta(lang, 15, "Brauer Groups")} {
			sec := &Section{Lang: lang, Meta: m, Body: body, Slug: slug(m),
				Path: filepath.Join("content", lang, "alg", "VIII", m.SectionTitle+".md")}
			sec.Units = Units(sec.Body)
			s.Sections = append(s.Sections, sec)
		}
	}
	s.index()
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

// An exercise is in the corpus and has no page yet. Saying it is not in the
// corpus is a lie the reader can check, since the exercises are in the same
// repository as the page saying so.
func TestAnExerciseIsNotCalledMissing(t *testing.T) {
	s := testSite(t)
	s.CitedBy["00GJ"] = []*Edge{{FromLabel: "alg-viii-s14-ex-8", ToTag: "00GJ", ToLabel: "alg-viii-s14-thm-1"}}

	got := s.mustPage(t, "tag/00GJ/index.html")
	if strings.Contains(got, "not in this corpus") {
		t.Errorf("an exercise is called missing:\n%s", got)
	}
	if !strings.Contains(got, "an exercise, no page yet") {
		t.Errorf("want the exercise noted in:\n%s", got)
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
