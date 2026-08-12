package refs

import (
	"os"
	"path/filepath"
	"testing"
)

// pagesFixture is a § whose second no. runs over two printed pages and holds a
// Corollary 1 on each of them, which is the shape the no. cannot separate and
// the page can. § 5 of chapter VIII is this shape twice over and § 12 once.
//
// Its third no. is a run of remarks set under one lead, "Remarks. —", which the
// book does and the corpus does not: the corpus gives each remark of the run a
// statement and a tag of its own, since a tag is an address and a run needs
// several.
func pagesFixture(t *testing.T) string {
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
              book_pages: A VIII.1 - A VIII.4
`)
	write("manifests/exercises.json", `{"books":[{"id":"alg-viii","chapters":[{"chapter":"VIII",
	  "sections":[{"section":1,"label":"alg-viii-s1","dir":"s1","count":0,"first":0,"last":0}]}]}]}`)
	write("content/en/alg/VIII/01_s1_a.md", `---
book: alg
book_title: Algebra
chapter: VIII
chapter_title: Semisimple Modules and Rings
section: 1
section_title: A Section
lang: en
source: alg-viii
book_pages: A VIII.1-A VIII.4
pdf_pages: 0001-0004
subsections:
    - "no": 1
      page: 1
    - "no": 2
      page: 2
    - "no": 3
      page: 4
statements: 7
exercises: 0
content_sha256: 0000000000000000000000000000000000000000000000000000000000000000
---

### 1. A first no.

#### Theorem 1 {#alg-viii-s1-thm-1 .statement tag=0001}

Every module is a module.

By VIII, p. 3, Corollary 1 and by VIII, p. 2, Corollary 1 this is clear.

### 2. A second no.

#### Proposition 1 {#alg-viii-s1-prop-1 .statement tag=0002}

The first proposition.

#### Corollary 1 {#alg-viii-s1-prop-1-cor-1 .statement tag=0003}

The corollary of the first.

#### Proposition 2 {#alg-viii-s1-prop-2 .statement tag=0004}

The second proposition, printed on the next page.

#### Corollary 1 {#alg-viii-s1-prop-2-cor-1 .statement tag=0005}

The corollary of the second.

### 3. A third no.

#### Remark 1 {#alg-viii-s1-n3-rem-1 .statement tag=0006}

The first remark of the run.

#### Remark 2 {#alg-viii-s1-n3-rem-2 .statement tag=0007}

The second remark of the run.
`)
	write("pages/alg-viii/0001.md", `---
book: alg-viii
pdf_page: 1
page_label: A VIII.1
method: native
lines: 3
---

**Theorem 1.** — Every module is a module.
`)
	write("pages/alg-viii/0002.md", `---
book: alg-viii
pdf_page: 2
page_label: A VIII.2
method: native
lines: 5
---

**Proposition 1.** — The first proposition.

**Corollary 1.** — The corollary of the first.
`)
	write("pages/alg-viii/0003.md", `---
book: alg-viii
pdf_page: 3
page_label: A VIII.3
method: native
lines: 5
---

**Proposition 2.** — The second proposition, printed on the next page.

**Corollary 1.** — The corollary of the second.
`)
	write("pages/alg-viii/0004.md", `---
book: alg-viii
pdf_page: 4
page_label: A VIII.4
method: native
lines: 3
---

**Remarks.** — The first remark of the run. The second remark of the run.
`)
	return root
}

// The page a statement is printed on is read back out of pages/ by lining the
// statements of a § up against the leads printed on its pages.
func TestStatementsAreGivenThePageTheyArePrintedOn(t *testing.T) {
	ix, err := Load(pagesFixture(t), "en")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]int{
		"alg-viii-s1-thm-1":        1,
		"alg-viii-s1-prop-1":       2,
		"alg-viii-s1-prop-1-cor-1": 2,
		"alg-viii-s1-prop-2":       3,
		"alg-viii-s1-prop-2-cor-1": 3,
		// Both members of the run are placed where the one lead is printed.
		"alg-viii-s1-n3-rem-1": 4,
		"alg-viii-s1-n3-rem-2": 4,
	}
	for label, page := range want {
		st := ix.Statement(label)
		if st == nil {
			t.Errorf("%s is not in the index", label)
			continue
		}
		if st.Page != page {
			t.Errorf("%s is placed on page %d, want %d", label, st.Page, page)
		}
	}
}

// Two statements called Corollary 1 stand in one no., so the no. says nothing
// about which of them a citation means and the page says everything.
func TestAPageSeparatesTwoStatementsOfOneNumberInOneNo(t *testing.T) {
	root := pagesFixture(t)
	res, err := Build(root, "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Unresolved) != 0 {
		t.Fatalf("unresolved: %+v", res.Unresolved)
	}
	want := map[string]string{
		"VIII, p. 3, Corollary 1": "alg-viii-s1-prop-2-cor-1",
		"VIII, p. 2, Corollary 1": "alg-viii-s1-prop-1-cor-1",
	}
	for _, e := range res.Edges {
		label, ok := want[e.Raw]
		if !ok {
			t.Errorf("%q is a reference the fixture does not make", e.Raw)
			continue
		}
		if e.ToLabel != label {
			t.Errorf("%q points at %s, want %s", e.Raw, e.ToLabel, label)
		}
		if e.How != ByStatementPage {
			t.Errorf("%q was settled by %s, want %s", e.Raw, e.How, ByStatementPage)
		}
		delete(want, e.Raw)
	}
	if len(want) > 0 {
		t.Errorf("%d of the fixture's references were not read at all: %v", len(want), want)
	}
}

// A statement that could not be placed has page 0, and page 0 is not a page: it
// has to stop the page from narrowing rather than rule the statement out, or a
// gap in the placement turns into a confident wrong answer.
func TestAnUnplacedStatementIsNotRuledOutByThePage(t *testing.T) {
	cand := []*Statement{
		{Label: "a", Page: 2},
		{Label: "b", Page: 0},
	}
	if _, ok := onPage(cand, 2); ok {
		t.Error("a page narrowed a set holding a statement that was never placed")
	}
	cand[1].Page = 3
	got, ok := onPage(cand, 2)
	if !ok || got.Label != "a" || got.How != ByStatementPage {
		t.Errorf("onPage came out %+v, %v", got, ok)
	}
	if _, ok := onPage(cand, 0); ok {
		t.Error("a citation that names no page was narrowed by one")
	}
}
