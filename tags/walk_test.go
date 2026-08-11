package tags

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestApply(t *testing.T) {
	body := strings.Join([]string{
		"### 1. Artinian Modules",
		"",
		"#### Definition 1 {#alg-viii-s1-def-1 .statement}",
		"",
		"Let A be a ring.",
		"",
		"#### Proposition 1 {#alg-viii-s1-prop-1 .statement tag=0009}",
		"",
		"#### Remark {#alg-viii-s1-n1-rem-1 .statement}",
	}, "\n")
	got := Apply(body, map[string]Tag{
		"alg-viii-s1-def-1":  "0001",
		"alg-viii-s1-prop-1": "0002",
	})
	if !strings.Contains(got, "#### Definition 1 {#alg-viii-s1-def-1 .statement tag=0001}") {
		t.Errorf("the tag was not written in:\n%s", got)
	}
	// A tag already in the file is replaced by what tags/ says, not kept. The
	// files are written from the record and never the other way round, which is
	// what makes hand editing a tag a thing CI catches instead of a thing that
	// quietly becomes true.
	if !strings.Contains(got, "#### Proposition 1 {#alg-viii-s1-prop-1 .statement tag=0002}") {
		t.Errorf("a stale tag in the file was kept:\n%s", got)
	}
	// A statement the record does not know is left bare. That is the state of
	// every statement the moment before its first assignment, and assembly has
	// to work then too.
	if !strings.Contains(got, "#### Remark {#alg-viii-s1-n1-rem-1 .statement}") {
		t.Errorf("an unassigned statement was not left alone:\n%s", got)
	}
	// With nothing in the record every heading comes out bare, tags and all.
	// The record is the truth and the Markdown is a copy of it.
	if strings.Contains(Apply(body, nil), "tag=") {
		t.Errorf("a tag survived an empty record:\n%s", Apply(body, nil))
	}
	if strings.Count(Apply(body, nil), ".statement}") != 3 {
		t.Errorf("an empty record did more than take the tags off:\n%s", Apply(body, nil))
	}
}

const walkSection = `---
book: alg
book_title: Algebra
chapter: VIII
chapter_title: Semisimple Modules and Rings
section: %d
section_title: A Section
lang: en
source: alg-viii
statements: 1
exercises: %d
content_sha256: 0000000000000000000000000000000000000000000000000000000000000000
---

#### Proposition 1 {#alg-%s-prop-1 .statement}

Let A be a ring.
`

const walkExercise = `---
book: alg
chapter: VIII
section: 1
exercise: 1
label: alg-viii-s1-ex-1
lang: en
has_hint: false
starred: false
---

Show that A is a ring.
`

// The order tags are handed out in has to be one a person can predict, because
// it is the order of a permanent record. It is the book's own: chapter in Roman
// order, then the sections as the volume prints them, and within a section the
// statements before the exercises printed at its foot.
func TestWalkGoesInBookOrder(t *testing.T) {
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
	write("content/en/alg/VIII/01_s1_a.md", fmt.Sprintf(walkSection, 1, 1, "viii-s1"))
	write("content/en/alg/VIII/02_s2_a.md", fmt.Sprintf(walkSection, 2, 0, "viii-s2"))
	write("content/en/alg/VIII/A1_a1_a.md", fmt.Sprintf(walkSection, 1, 0, "viii-a1"))
	write("content/en/alg/VIII/exercises/s1/01.md", walkExercise)
	write("content/en/alg/III/01_s1_a.md", fmt.Sprintf(walkSection, 1, 0, "iii-s1"))

	items, err := Walk(root, "en")
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"alg-iii-s1-prop-1", // chapter III comes first, Roman order and not lexicographic
		"alg-viii-s1-prop-1",
		"alg-viii-s1-ex-1", // the exercises of § 1 follow its statements
		"alg-viii-s2-prop-1",
		"alg-viii-a1-prop-1", // the appendices come after the sections, as the volume prints them
	}
	if len(items) != len(want) {
		t.Fatalf("walk found %d items, want %d: %+v", len(items), len(want), items)
	}
	for i, w := range want {
		if items[i].Label != w {
			t.Errorf("item %d is %s, want %s", i, items[i].Label, w)
		}
	}
	if items[1].Line == 0 {
		t.Error("a statement was found with no line number")
	}
	if items[2].Line != 0 {
		t.Error("an exercise carries its tag in the front matter and has no line")
	}
}
