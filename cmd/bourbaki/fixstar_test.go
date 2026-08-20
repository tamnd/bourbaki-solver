package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/textguard"
)

// starCorpus writes one English section and one Vietnamese translation of it,
// both carrying the ornament. That is how the corpus stands: the model asked for
// a translation was given the ornament and it handed it back.
func starCorpus(t *testing.T) string {
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
	en := "\n∗ (4) The species of group structures, as algebra defines it. ∗\n"
	write("content/en/ens/IV/01_s1.md", `---
book: ens
chapter: IV
section: 1
kind: section
lang: en
content_sha256: `+corpus.ContentSHA256(en)+`
---
`+en)
	vi := "\n∗ (4) Loai cau truc nhom, nhu dai so dinh nghia no. ∗\n"
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

func TestFixStarRepairsATranslationAndMovesItOn(t *testing.T) {
	root := starCorpus(t)

	files, changed, followed, err := repairContent(root, false, "stars", textguard.Stars)
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
		if strings.Count(string(raw), `\*`) != 2 {
			t.Errorf("%s does not carry the pair the passage is set between:\n%s", rest, raw)
		}
		if strings.ContainsRune(string(raw), '∗') {
			t.Errorf("%s still carries the ornament:\n%s", rest, raw)
		}
	}

	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash("content/vi/ens/IV/01_s1.md")))
	if err != nil {
		t.Fatal(err)
	}
	want := corpus.ContentSHA256("\n\\* (4) The species of group structures, as algebra defines it. \\*\n")
	if !strings.Contains(string(raw), "source_content_sha256: "+want) {
		t.Errorf("the translation was not moved on to the English as it now stands:\n%s", raw)
	}
}

func TestFixStarCheckWritesNothing(t *testing.T) {
	root := starCorpus(t)
	path := filepath.Join(root, filepath.FromSlash("content/vi/ens/IV/01_s1.md"))
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := repairContent(root, true, "stars", textguard.Stars); err != nil {
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
