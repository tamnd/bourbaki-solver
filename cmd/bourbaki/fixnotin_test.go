package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/mathtex"
)

// notinCorpus writes one English section and one Vietnamese translation of it,
// both carrying the loose solidus. That is how the corpus stands: the model was
// told to copy the mathematics it was given and it copied this too.
func notinCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rest, text string) {
		path := filepath.Join(root, filepath.FromSlash(rest))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	en := "\nif $0\\in /S$ then $S$ is not saturated.\n"
	write("content/en/ens/IV/01_s1.md", `---
book: ens
chapter: IV
section: 1
kind: section
lang: en
content_sha256: `+corpus.ContentSHA256(en)+`
---
`+en)
	vi := "\nneu $0\\in /S$ thi $S$ khong bao hoa.\n"
	write("content/vi/ens/IV/01_s1.md", `---
book: ens
chapter: IV
section: 1
kind: section
lang: vi
content_sha256: `+corpus.ContentSHA256(vi)+`
translated_from: content/en/ens/IV/01_s1.md
source_content_sha256: `+corpus.ContentSHA256(en)+`
---
`+vi)
	return root
}

// A translation is the only file here the next assemble will not rewrite, so it
// is the one the content pass exists for.
func TestFixNotinRepairsATranslationAndMovesItOn(t *testing.T) {
	root := notinCorpus(t)

	files, changed, followed, err := repairContent(root, false, "signs", mathtex.Negation)
	if err != nil {
		t.Fatal(err)
	}
	if files != 2 || changed != 2 || followed != 1 {
		t.Fatalf("read %d files, changed %d, moved %d on, want 2, 2, 1", files, changed, followed)
	}

	for _, rest := range []string{"content/en/ens/IV/01_s1.md", "content/vi/ens/IV/01_s1.md"} {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rest)))
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(string(raw), `$0\notin S$`) {
			t.Errorf("%s was not repaired:\n%s", rest, raw)
		}
		if strings.Contains(string(raw), `\in /`) {
			t.Errorf("%s still says the opposite of what the book prints:\n%s", rest, raw)
		}
	}

	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash("content/vi/ens/IV/01_s1.md")))
	if err != nil {
		t.Fatal(err)
	}
	want := corpus.ContentSHA256("\nif $0\\notin S$ then $S$ is not saturated.\n")
	if !strings.Contains(string(raw), "source_content_sha256: "+want) {
		t.Errorf("the translation was not moved on to the English as it now stands:\n%s", raw)
	}
}

func TestFixNotinCheckWritesNothing(t *testing.T) {
	root := notinCorpus(t)
	path := filepath.Join(root, filepath.FromSlash("content/vi/ens/IV/01_s1.md"))
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := repairContent(root, true, "signs", mathtex.Negation); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("-check wrote to the file")
	}
}
