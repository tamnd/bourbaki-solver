package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/translate"
)

// Staleness is what decides whether a section is asked for again, so getting it
// wrong is expensive in both directions: too eager and a glossary edit buys a
// run of the whole corpus, too lazy and files go on claiming to be current
// against terminology they were never shown.
//
// The rule that shipped first was too eager. It compared glossary_version, which
// moves on any edit anywhere, so pinning "common zero", a phrase that occurs in
// one appendix of chapter VIII, marked all 27 sections stale. Measured on the
// real corpus, the version 2 to version 5 move changes what 14 of the 27 are
// shown, "common zero" reaches 1 of them and "algebraic over" reaches 3.
func TestAGlossaryRowOnlyStalesTheSectionsItReaches(t *testing.T) {
	root := t.TempDir()
	english := "Let A be a ring, and let M be a simple module over it."
	other := "Every finite division ring is a field."
	writeEnglish(t, root, 1, english)
	writeEnglish(t, root, 2, other)

	g := &glossary.Glossary{Version: 4, Terms: []glossary.Term{
		{EN: "ring", VI: "vành"},
		{EN: "simple module", VI: "môđun đơn"},
	}}
	writeVietnamese(t, root, g, 1, english)
	writeVietnamese(t, root, g, 2, other)

	// Nothing has moved, so nothing is stale.
	if jobs := stale(t, root, g); len(jobs) != 0 {
		t.Fatalf("a corpus nobody touched reported %d stale: %v", len(jobs), jobs)
	}

	// A row that reaches one section stales that one and not the other, even
	// though the version moves for both.
	g.Version = 5
	g.Terms[1].VI = "môđun giản đơn"
	jobs := stale(t, root, g)
	if len(jobs) != 1 || jobs[0].source != rel(root, englishPath(root, 1)) {
		t.Fatalf("got %v, want only the section that mentions the term", jobs)
	}
	if jobs[0].why != "the terminology it was shown has changed" {
		t.Errorf("the reason given is %q", jobs[0].why)
	}

	// A row in neither section stales neither, and it does move the version.
	g.Version = 6
	g.Terms = append(g.Terms, glossary.Term{EN: "quaternion algebra", VI: "đại số quaternion"})
	writeVietnamese(t, root, g, 1, english)
	if jobs := stale(t, root, g); len(jobs) != 0 {
		t.Fatalf("a row neither section mentions stale %d of them: %v", len(jobs), jobs)
	}
}

// A file written before the digest existed has nothing to compare, so it falls
// back to the version and is stale on any bump. That is where every file already
// was, and it is the only honest answer: the rows it was shown were not recorded
// and cannot be recovered from the file.
func TestAFileWithNoDigestFallsBackToTheVersion(t *testing.T) {
	root := t.TempDir()
	english := "Let A be a ring."
	writeEnglish(t, root, 1, english)
	g := &glossary.Glossary{Version: 4, Terms: []glossary.Term{{EN: "ring", VI: "vành"}}}
	writeVietnamese(t, root, g, 1, english)

	// Take the digest out, the way a file written a month ago has none.
	path := vietnamesePath(root, 1)
	f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
	if err != nil {
		t.Fatal(err)
	}
	f.Meta.GlossaryTerms = ""
	if err := f.Write(path); err != nil {
		t.Fatal(err)
	}

	if jobs := stale(t, root, g); len(jobs) != 0 {
		t.Fatalf("the same version was called stale: %v", jobs)
	}
	g.Version = 5
	jobs := stale(t, root, g)
	if len(jobs) != 1 {
		t.Fatalf("got %d stale, want 1", len(jobs))
	}
	if jobs[0].why != "it records no terminology and was made with glossary 4, which is now 5" {
		t.Errorf("the reason given is %q", jobs[0].why)
	}
}

// The other three reasons a file is stale, and each says which it is, because
// "stale" on its own does not tell you whether one section is going again or all
// of them are.
func TestEachKindOfStalenessSaysWhichItIs(t *testing.T) {
	g := &glossary.Glossary{Version: 4, Terms: []glossary.Term{{EN: "ring", VI: "vành"}}}
	english := "Let A be a ring."

	cases := []struct {
		name string
		bend func(root string, meta *corpus.SectionFrontMatter)
		want string
	}{
		{"no translation", nil, "there is no translation"},
		{"the English moved", func(_ string, m *corpus.SectionFrontMatter) {
			m.SourceSHA256 = "0000"
		}, "the English has changed since"},
		{"the instructions moved", func(_ string, m *corpus.SectionFrontMatter) {
			m.PromptSHA256 = "0000"
		}, "the instructions have changed since"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeEnglish(t, root, 1, english)
			if c.bend != nil {
				writeVietnamese(t, root, g, 1, english)
				path := vietnamesePath(root, 1)
				f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
				if err != nil {
					t.Fatal(err)
				}
				c.bend(root, &f.Meta)
				if err := f.Write(path); err != nil {
					t.Fatal(err)
				}
			}
			jobs := stale(t, root, g)
			if len(jobs) != 1 {
				t.Fatalf("got %d stale, want 1", len(jobs))
			}
			if jobs[0].why != c.want {
				t.Errorf("the reason given is %q, want %q", jobs[0].why, c.want)
			}
		})
	}
}

// The digest is of the rows the section is shown and not of the glossary, so two
// sections that mention the same terms carry the same digest and a term nobody
// mentions does not appear in either.
func TestTheDigestIsTheRowsAndNotTheGlossary(t *testing.T) {
	g := &glossary.Glossary{Version: 1, Terms: []glossary.Term{
		{EN: "ring", VI: "vành"},
		{EN: "quaternion algebra", VI: "đại số quaternion"},
	}}
	a := translate.GlossaryDigest(g, "vi", "Let A be a ring.")
	b := translate.GlossaryDigest(g, "vi", "Every ring has a unit.")
	if a != b {
		t.Error("two sections shown the same rows carry different digests")
	}
	if c := translate.GlossaryDigest(g, "vi", "Let A be a quaternion algebra."); c == a {
		t.Error("two sections shown different rows carry the same digest")
	}
	// A language with no rendering is shown nothing, which is where Chinese and
	// Japanese are until the glossary is filled in for them.
	if translate.GlossaryDigest(g, "zh", "Let A be a ring.") != translate.GlossaryDigest(g, "zh", "") {
		t.Error("a language with no renderings was shown something")
	}
}

// stale is what a run would do, without asking anything.
func stale(t *testing.T, root string, g *glossary.Glossary) []job {
	t.Helper()
	jobs, _, err := translateJobs(root, g, "vi", "", "", "", "prompt-hash", false)
	if err != nil {
		t.Fatal(err)
	}
	return jobs
}

func meta(n int) corpus.SectionFrontMatter {
	return corpus.SectionFrontMatter{
		Book: "alg", BookTitle: "Algebra", Chapter: "VIII", ChapterTitle: "Semisimple",
		Section: n, SectionTitle: "A Section", Lang: "en", Source: "alg-viii",
	}
}

func englishPath(root string, n int) string {
	return corpus.SectionPath(root, "en", meta(n))
}

func vietnamesePath(root string, n int) string {
	m := meta(n)
	m.Lang = "vi"
	return corpus.SectionPath(root, "vi", m)
}

func writeEnglish(t *testing.T, root string, n int, body string) {
	t.Helper()
	m := meta(n)
	m.ContentSHA256 = corpus.ContentSHA256(body)
	writeSection(t, corpus.SectionPath(root, "en", m), corpus.SectionFile{Meta: m, Body: body})
}

// writeVietnamese writes the file a run against this glossary would have left,
// which is the only way a test of staleness can be about staleness rather than
// about how the front matter is filled in.
func writeVietnamese(t *testing.T, root string, g *glossary.Glossary, n int, english string) {
	t.Helper()
	m := meta(n)
	m.Lang = "vi"
	m.TranslatedFrom = rel(root, englishPath(root, n))
	m.SourceSHA256 = corpus.ContentSHA256(english)
	m.ContentSHA256 = corpus.ContentSHA256("Cho A là một vành.")
	m.GlossaryVersion = g.Version
	m.GlossaryTerms = translate.GlossaryDigest(g, "vi", english)
	m.PromptSHA256 = "prompt-hash"
	writeSection(t, corpus.SectionPath(root, "vi", m), corpus.SectionFile{Meta: m, Body: "Cho A là một vành."})
}

func writeSection(t *testing.T, path string, f corpus.SectionFile) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := f.Write(path); err != nil {
		t.Fatal(err)
	}
}
