package refs

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// fixture is a corpus of one § and one exercise, small enough to read and wide
// enough to exercise the whole of Build: the front matter that has no label,
// the subsection headings a no. is read off, an unnumbered corollary, a
// reference in an exercise, and a reference that leaves the corpus.
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
            - kind: front
              path: content/en/alg/VIII/00_frontmatter.md
              book_pages: A VIII.1
            - kind: section
              section: 1
              path: content/en/alg/VIII/01_s1_a.md
              label: alg-viii-s1
              book_pages: A VIII.1 - A VIII.23
`)
	write("manifests/exercises.json", `{"books":[{"id":"alg-viii","chapters":[{"chapter":"VIII",
	  "sections":[{"section":1,"label":"alg-viii-s1","dir":"s1","count":2,"first":1,"last":2}]}]}]}`)
	write("content/en/alg/VIII/00_frontmatter.md", `---
book: alg
chapter: VIII
lang: en
---

The chapter opens here and cites nothing.
`)
	write("content/en/alg/VIII/01_s1_a.md", `---
book: alg
book_title: Algebra
chapter: VIII
chapter_title: Semisimple Modules and Rings
section: 1
section_title: A Section
lang: en
source: alg-viii
subsections:
    - "no": 1
      page: 1
    - "no": 2
      page: 9
statements: 3
exercises: 2
content_sha256: 0000000000000000000000000000000000000000000000000000000000000000
---

### 1. A first no.

#### Proposition 1 {#alg-viii-s1-prop-1 .statement tag=0001}

Let A be a ring. This is proved in Set Theory, III, §2, No. 4, p. 155.

### 2. A second no.

#### Proposition 2 {#alg-viii-s1-prop-2 .statement tag=0002}

This follows from Proposition 1 and from VIII, p. 3, Proposition 1.

#### Corollary {#alg-viii-s1-n2-cor-1 .statement tag=0003}

Immediate, by formula (2).
`)
	write("content/en/alg/VIII/exercises/s1/01.md", `---
book: alg
chapter: VIII
section: 1
exercise: 1
label: alg-viii-s1-ex-1
tag: 0004
lang: en
has_hint: false
starred: false
---

Show that A is a ring, using the corollary of Proposition 2.
`)
	return root
}

func TestBuild(t *testing.T) {
	root := fixture(t)
	res, err := Build(root, "en")
	if err != nil {
		t.Fatal(err)
	}
	// One formula, which never resolves and is counted anyway.
	if len(res.Unresolved) != 1 || res.Unresolved[0].Form != FormFormula {
		t.Errorf("unresolved came out as %+v", res.Unresolved)
	}
	want := map[string]int{
		OutOfCorpus:  1, // Set Theory
		ByContext:    1, // Proposition 1, named in the prose of Proposition 2
		ByPage:       1, // VIII, p. 3, Proposition 1
		ByParent:     1, // the corollary of Proposition 2, cited from the exercise
		"unresolved": 1,
	}
	for k, n := range want {
		if res.Counts[k] != n {
			t.Errorf("%s came out %d, want %d (all: %+v)", k, res.Counts[k], n, res.Counts)
		}
	}
	if len(res.Counts) != len(want) {
		t.Errorf("the counts are %+v, want only %v", res.Counts, want)
	}

	// The exercise's reference is an edge out of the exercise, not out of the §
	// it is printed under, and the tag on the tail is the exercise's own.
	var fromEx *Edge
	for i := range res.Edges {
		if strings.Contains(res.Edges[i].File, "exercises/") {
			fromEx = &res.Edges[i]
		}
	}
	if fromEx == nil {
		t.Fatal("the exercise cited nothing")
	}
	if fromEx.FromLabel != "alg-viii-s1-ex-1" || fromEx.From != "0004" {
		t.Errorf("the exercise's edge starts at %s (%q)", fromEx.FromLabel, fromEx.From)
	}
	if fromEx.ToLabel != "alg-viii-s1-n2-cor-1" || fromEx.To == nil || *fromEx.To != "0003" {
		t.Errorf("the exercise's edge ends at %s (%v)", fromEx.ToLabel, fromEx.To)
	}

	// A reference out of the corpus has no tag at the head, and says which
	// chapter of which Book, since that is the ingestion order.
	for _, e := range res.Edges {
		if e.How != OutOfCorpus {
			continue
		}
		if e.To != nil {
			t.Errorf("a reference out of the corpus was given the tag %q", *e.To)
		}
		if e.Book != "E" || e.Chapter != "III" {
			t.Errorf("the outside reference is book %q chapter %q", e.Book, e.Chapter)
		}
	}
}

// The manifest and the reports are a pure function of the Markdown, so a second
// run over an unchanged corpus has nothing to say. That is what lets CI check
// them with -check instead of rebuilding them.
func TestBuildIsStable(t *testing.T) {
	root := fixture(t)
	if err := os.MkdirAll(filepath.Join(root, "reports"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Build(root, "en")
	if err != nil {
		t.Fatal(err)
	}
	stale, err := res.Manifest().Stale(root)
	if err != nil {
		t.Fatal(err)
	}
	if !stale {
		t.Error("a corpus with no manifest was not reported stale")
	}
	if err := res.Manifest().Save(root); err != nil {
		t.Fatal(err)
	}
	wrote, err := res.Reports(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(wrote) != 4 {
		t.Errorf("wrote %v", wrote)
	}
	for _, w := range wrote {
		b, err := os.ReadFile(filepath.Join(root, w))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasPrefix(string(b), "# ") {
			t.Errorf("%s does not open with a heading", w)
		}
		if !strings.Contains(string(b), "bourbaki refs build") {
			t.Errorf("%s does not say it is generated", w)
		}
	}

	again, err := Build(root, "en")
	if err != nil {
		t.Fatal(err)
	}
	stale, err = again.Manifest().Stale(root)
	if err != nil {
		t.Fatal(err)
	}
	if stale {
		t.Error("a second run over an unchanged corpus disagreed with the first")
	}
}

// Proposition 1 is cited twice and nothing cites Proposition 2, so one belongs
// in each report and neither in both. The corollary is in neither: the exercise
// cites it, and an exercise is a citation like any other.
func TestReportsNameTheRightStatements(t *testing.T) {
	root := fixture(t)
	if err := os.MkdirAll(filepath.Join(root, "reports"), 0o755); err != nil {
		t.Fatal(err)
	}
	res, err := Build(root, "en")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := res.Reports(root); err != nil {
		t.Fatal(err)
	}
	read := func(name string) string {
		b, err := os.ReadFile(filepath.Join(root, "reports", name))
		if err != nil {
			t.Fatal(err)
		}
		return string(b)
	}
	cited := read(ReportMostCited)
	if !strings.Contains(cited, "| 2 | 0001 | alg-viii-s1-prop-1 |") {
		t.Errorf("most-cited does not have Proposition 1 at two references:\n%s", cited)
	}
	orphans := read(ReportOrphans)
	if strings.Contains(orphans, "alg-viii-s1-prop-1") {
		t.Errorf("a cited statement is in orphans:\n%s", orphans)
	}
	if !strings.Contains(orphans, "alg-viii-s1-prop-2") {
		t.Errorf("the statement nothing cites is not in orphans:\n%s", orphans)
	}
	if strings.Contains(orphans, "alg-viii-s1-n2-cor-1") {
		t.Errorf("a statement an exercise cites is in orphans:\n%s", orphans)
	}
	out := read(ReportOutOfCorpus)
	if !strings.Contains(out, "| 1 | E | III |") {
		t.Errorf("out-of-corpus does not name Set Theory III:\n%s", out)
	}
	un := read(ReportUnresolved)
	if !strings.Contains(un, "a numbered display carries no tag of its own") {
		t.Errorf("the formula is not in the unresolved report:\n%s", un)
	}
}

// An unnumbered corollary hangs from the last Theorem or Proposition above it,
// and not from whatever numbered statement happens to be nearest.
//
// The chapter prints both interpositions. § 20 no. 6 has Proposition 6, then a
// Remark, then the corollary of Proposition 6. § 1 no. 1 has Proposition 3,
// then Lemma 2, then the corollary of Proposition 3, the lemma being what the
// corollary's proof uses rather than what it follows from. Taking any numbered
// statement left both of those corollaries fathered by the interposed one, and
// the three references to them resolved to nothing.
func TestAnInterposedStatementDoesNotFatherACorollary(t *testing.T) {
	root := fixture(t)
	body := `---
book: alg
chapter: VIII
section: 1
lang: en
subsections:
    - "no": 1
      page: 1
statements: 6
exercises: 2
---

### 1. A first no.

#### Proposition 3 {#alg-viii-s1-prop-3 .statement tag=0001}

Let A be a ring.

#### Lemma 2 {#alg-viii-s1-lem-2 .statement tag=0002}

Used in the proof below.

#### Corollary {#alg-viii-s1-n1-cor-1 .statement tag=0003}

This is the corollary of Proposition 3.

#### Proposition 6 {#alg-viii-s1-prop-6 .statement tag=0004}

Let B be a ring.

#### Remark {#alg-viii-s1-n1-rem-1 .statement tag=0005}

An aside that hangs from nothing.

#### Corollary {#alg-viii-s1-n1-cor-2 .statement tag=0006}

This is the corollary of Proposition 6.
`
	if err := os.WriteFile(filepath.Join(root, "content", "en", "alg", "VIII", "01_s1_a.md"),
		[]byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	ix, err := Load(root, "en")
	if err != nil {
		t.Fatal(err)
	}
	for _, tc := range []struct {
		label  string
		parent string
		number int
	}{
		{"alg-viii-s1-n1-cor-1", "prop", 3},
		{"alg-viii-s1-n1-cor-2", "prop", 6},
	} {
		st := ix.Statement(tc.label)
		if st == nil {
			t.Fatalf("%s is not in the index", tc.label)
		}
		if string(st.FollowsKind) != tc.parent || st.FollowsNumber != tc.number {
			t.Errorf("%s hangs from %s %d, want %s %d",
				tc.label, st.FollowsKind, st.FollowsNumber, tc.parent, tc.number)
		}
	}
	// And the references to them are what this is for.
	for _, tc := range []struct{ in, label string }{
		{"the corollary of Proposition 3", "alg-viii-s1-n1-cor-1"},
		{"the corollary of Proposition 6", "alg-viii-s1-n1-cor-2"},
	} {
		c := Parse(tc.in, 1)
		if len(c) != 1 {
			t.Fatalf("%q read as %d citations", tc.in, len(c))
		}
		got, err := ix.Resolve(c[0], Site{Section: "alg-viii-s1"})
		if err != nil {
			t.Fatalf("%q did not resolve: %v", tc.in, err)
		}
		if got.Label != tc.label {
			t.Errorf("%q resolved to %s, want %s", tc.in, got.Label, tc.label)
		}
	}
}

// A language the corpus does not hold is an error and not an empty graph, so
// that a typo in -lang does not read as a corpus with no references in it.
func TestBuildRejectsAMissingLanguage(t *testing.T) {
	if _, err := Build(fixture(t), "vi"); err == nil {
		t.Error("a language with no content built a graph")
	}
}

// The corpus holds Algebra VIII twice, in English and in French, and the two
// printings share their labels and their printed page numbers because they are
// the same chapter. The graph is over one of them.
//
// Indexing both is what broke on the day the French was assembled: a citation
// to VIII, p. 3 found the page in two sections at once and resolved to nothing,
// and 979 references went with it. The second printing has to be out of the
// index, not merely deduplicated, because its § 2 holds one exercise fewer and
// the counts would then depend on which one was read last.
func TestASecondPrintingIsNotASecondSection(t *testing.T) {
	root := fixture(t)
	french := `books:
    - id: alg-viii
      chapters:
        - chapter: VIII
          sections:
            - kind: front
              path: content/en/alg/VIII/00_frontmatter.md
              book_pages: A VIII.1
            - kind: section
              section: 1
              path: content/en/alg/VIII/01_s1_a.md
              label: alg-viii-s1
              book_pages: A VIII.1 - A VIII.23
    - id: alg-viii-fr
      chapters:
        - chapter: VIII
          sections:
            - kind: section
              section: 1
              path: content/fr/alg/VIII/01_s1_a.md
              label: alg-viii-s1
              book_pages: A VIII.1 - A VIII.23
`
	if err := os.WriteFile(filepath.Join(root, "manifests", "sections.yaml"), []byte(french), 0o644); err != nil {
		t.Fatal(err)
	}
	// The French printing is on disk, so that what this test catches is the
	// index holding two sections and not a file that could not be opened.
	en, err := os.ReadFile(filepath.Join(root, "content", "en", "alg", "VIII", "01_s1_a.md"))
	if err != nil {
		t.Fatal(err)
	}
	fr := filepath.Join(root, "content", "fr", "alg", "VIII", "01_s1_a.md")
	if err := os.MkdirAll(filepath.Dir(fr), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(fr, []byte(strings.Replace(string(en), "lang: en", "lang: fr", 1)), 0o644); err != nil {
		t.Fatal(err)
	}

	res, err := Build(root, "en")
	if err != nil {
		t.Fatal(err)
	}
	if res.Counts[ByPage] != 1 {
		t.Fatalf("references resolved by page = %d, want 1: %+v", res.Counts[ByPage], res.Counts)
	}
	for _, u := range res.Unresolved {
		if strings.Contains(u.Reason, "sections at once") {
			t.Fatalf("the two printings were indexed as two sections: %s", u.Reason)
		}
	}
}
