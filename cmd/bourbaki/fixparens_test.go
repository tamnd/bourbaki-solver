package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// parensCorpus writes one English section, one Vietnamese translation of it, and
// one Vietnamese exercise, and hands back the root. The English carries the
// straddle, so the translation carries it too, which is how the corpus stands:
// the model copied the mathematics it was given.
func parensCorpus(t *testing.T, recorded string) string {
	t.Helper()
	root := t.TempDir()
	write := func(rest, text string) string {
		path := filepath.Join(root, filepath.FromSlash(rest))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
		return path
	}
	en := write("content/en/alg/VIII/01_s1.md", `---
book: alg
chapter: VIII
section: 1
kind: section
lang: en
content_sha256: `+corpus.ContentSHA256("\nso Card(I$_L)$ is finite.\n")+`
---

so Card(I$_L)$ is finite.
`)
	vi := write("content/vi/alg/VIII/01_s1.md", `---
book: alg
chapter: VIII
section: 1
kind: section
lang: vi
content_sha256: `+corpus.ContentSHA256("\nvay Card(I$_L)$ la huu han.\n")+`
translated_from: content/en/alg/VIII/01_s1.md
source_content_sha256: `+recorded+`
---

vay Card(I$_L)$ la huu han.
`)
	ex := write("content/vi/alg/VIII/exercises/s1/01.md", `---
book: alg
chapter: VIII
section: 1
exercise: 1
label: alg-viii-s1-ex-1
lang: vi
translated_from: content/en/alg/VIII/exercises/s1/01.md
---

Chung minh rang Card(I$_L)$ la huu han.
`)
	_, _, _ = en, vi, ex
	return root
}

func TestFixParensContentRepairsATranslationAndItsExercises(t *testing.T) {
	root := parensCorpus(t, corpus.ContentSHA256("\nso Card(I$_L)$ is finite.\n"))

	files, changed, followed, err := parensContent(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if files != 3 {
		t.Errorf("read %d content files, want 3", files)
	}
	if changed != 3 {
		t.Errorf("changed %d files, want 3", changed)
	}
	if followed != 1 {
		t.Errorf("moved %d translations on, want 1", followed)
	}

	for _, rest := range []string{
		"content/en/alg/VIII/01_s1.md",
		"content/vi/alg/VIII/01_s1.md",
		"content/vi/alg/VIII/exercises/s1/01.md",
	} {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rest)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), `Card(I$_L$)`) {
			t.Errorf("%s was not repaired:\n%s", rest, raw)
		}
	}

	// The translation records the English as it now stands, so it is current and
	// not stale, and nobody has to ask the fleet for it again.
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash("content/vi/alg/VIII/01_s1.md")))
	if err != nil {
		t.Fatal(err)
	}
	want := corpus.ContentSHA256("\nso Card(I$_L$) is finite.\n")
	if !strings.Contains(string(raw), "source_content_sha256: "+want) {
		t.Errorf("the translation was not moved on to %s:\n%s", want, raw)
	}
}

// A translation that was already stale before the repair stays stale. Moving it
// on would say the model had seen a body it never saw.
func TestFixParensContentLeavesAStaleTranslationStale(t *testing.T) {
	root := parensCorpus(t, corpus.ContentSHA256("something else entirely"))

	_, _, followed, err := parensContent(root, false)
	if err != nil {
		t.Fatal(err)
	}
	if followed != 0 {
		t.Errorf("moved %d translations on, want 0", followed)
	}
	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash("content/vi/alg/VIII/01_s1.md")))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), "source_content_sha256: "+corpus.ContentSHA256("something else entirely")) {
		t.Errorf("the recorded source moved:\n%s", raw)
	}
}

func TestFixParensContentCheckWritesNothing(t *testing.T) {
	root := parensCorpus(t, corpus.ContentSHA256("\nso Card(I$_L)$ is finite.\n"))
	path := filepath.Join(root, filepath.FromSlash("content/vi/alg/VIII/01_s1.md"))
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := parensContent(root, true); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Errorf("-check wrote to the file")
	}
}
