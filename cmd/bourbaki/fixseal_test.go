package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// sealCorpus is a corpus of content files and nothing else. fix seal reads no
// manifest and no page, so a directory of sections is the whole of what it
// needs, and writing one by hand is how a body and its hash are made to
// disagree in the first place.
func sealCorpus(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("BOURBAKI_CORPUS", root)
	for name, text := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// sealFile is a section file with the hash it is given, which is the hash of
// some other body when the test wants an unsealed file.
func sealFile(body, sha string, extra ...string) string {
	head := []string{
		"book: alg", "book_title: Algebra", "chapter: VIII",
		"chapter_title: Rings", "section: 1", "section_title: Simple rings",
		"lang: en", "source: alg-viii.pdf", "statements: 0", "exercises: 0",
		"content_sha256: " + sha,
	}
	head = append(head, extra...)
	return "---\n" + strings.Join(head, "\n") + "\n---\n\n" + body
}

func readSection(t *testing.T, root, name string) corpus.File[corpus.SectionFrontMatter] {
	t.Helper()
	f, err := corpus.ReadFile[corpus.SectionFrontMatter](filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

func TestSealWritesTheHashOfTheBodyThatIsThere(t *testing.T) {
	body := "The centre of a simple ring is a field.\n"
	root := sealCorpus(t, map[string]string{
		"content/en/alg/VIII/01_s1_simple_rings.md": sealFile(body, corpus.ContentSHA256("something else")),
	})
	if err := fixSeal(nil); err != nil {
		t.Fatal(err)
	}
	f := readSection(t, root, "content/en/alg/VIII/01_s1_simple_rings.md")
	if got, want := f.Meta.ContentSHA256, corpus.ContentSHA256(body); got != want {
		t.Errorf("hash is %s, want %s", got, want)
	}
	if f.Body != body {
		t.Errorf("body is %q, want %q", f.Body, body)
	}
}

// A file that is already sealed is not rewritten. The command runs over the
// whole of content/ and a corpus of a hundred and fifty sections must not come
// back as a hundred and fifty modified files.
func TestSealLeavesASealedFileAlone(t *testing.T) {
	body := "Every field is a simple ring.\n"
	name := "content/en/alg/VIII/01_s1_simple_rings.md"
	root := sealCorpus(t, map[string]string{name: sealFile(body, corpus.ContentSHA256(body))})
	before, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	if err := fixSeal(nil); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("a sealed file was rewritten:\n%s", after)
	}
}

func TestSealCheckChangesNothing(t *testing.T) {
	body := "A ring with no two-sided ideal but itself and zero.\n"
	name := "content/en/alg/VIII/01_s1_simple_rings.md"
	stale := corpus.ContentSHA256("the body it used to have")
	root := sealCorpus(t, map[string]string{name: sealFile(body, stale)})
	if err := fixSeal([]string{"-check"}); err != nil {
		t.Fatal(err)
	}
	if got := readSection(t, root, name).Meta.ContentSHA256; got != stale {
		t.Errorf("-check wrote %s, want the file left at %s", got, stale)
	}
}

// The exercises are files of another schema, under the same tree. Reading one
// as a section fails on front matter it does not have, so the walk has to skip
// the directory rather than meet it.
func TestSealDoesNotReadAnExercise(t *testing.T) {
	body := "The centre is a field.\n"
	root := sealCorpus(t, map[string]string{
		"content/en/alg/VIII/01_s1_simple_rings.md": sealFile(body, corpus.ContentSHA256("stale")),
		"content/en/alg/VIII/exercises/s1/01.md":    "---\nbook: alg\nnumber: 1\n---\n\nShow that.\n",
		"content/solutions/en/alg/VIII/s1/01.md":    "---\nbook: alg\nnumber: 1\n---\n\nBecause.\n",
	})
	if err := fixSeal(nil); err != nil {
		t.Fatalf("the walk read a file of another schema: %v", err)
	}
	if got, want := readSection(t, root, "content/en/alg/VIII/01_s1_simple_rings.md").Meta.ContentSHA256,
		corpus.ContentSHA256(body); got != want {
		t.Errorf("hash is %s, want %s", got, want)
	}
}

// -lang is the whole point of the flag: the French moved and the Vietnamese
// is not to be touched in the same run.
func TestSealTakesOneLanguageAtATime(t *testing.T) {
	fr := "Le centre d’un anneau simple est un corps.\n"
	vi := "Tâm của một vành đơn là một trường.\n"
	root := sealCorpus(t, map[string]string{
		"content/fr/alg/VIII/01_s1_anneaux_simples.md": sealFile(fr, corpus.ContentSHA256("vieux")),
		"content/vi/alg/VIII/01_s1_vanh_don.md":        sealFile(vi, corpus.ContentSHA256("cũ")),
	})
	if err := fixSeal([]string{"-lang", "fr"}); err != nil {
		t.Fatal(err)
	}
	if got, want := readSection(t, root, "content/fr/alg/VIII/01_s1_anneaux_simples.md").Meta.ContentSHA256,
		corpus.ContentSHA256(fr); got != want {
		t.Errorf("French hash is %s, want %s", got, want)
	}
	if got, want := readSection(t, root, "content/vi/alg/VIII/01_s1_vanh_don.md").Meta.ContentSHA256,
		corpus.ContentSHA256("cũ"); got != want {
		t.Errorf("Vietnamese hash is %s, want the run to have left it at %s", got, want)
	}
}

// Sealing the English restales the translation made from it, and the run says
// which one. It does not touch the translation: the section really is stale and
// writing the new hash into it would be a lie about work nobody has done.
func TestSealNamesTheTranslationItStales(t *testing.T) {
	en := "The centre of a simple ring is a field.\n"
	old := corpus.ContentSHA256("the body before the correction")
	viName := "content/vi/alg/VIII/01_s1_vanh_don.md"
	root := sealCorpus(t, map[string]string{
		"content/en/alg/VIII/01_s1_simple_rings.md": sealFile(en, old),
		viName: sealFile("Tâm của một vành đơn là một trường.\n",
			corpus.ContentSHA256("Tâm của một vành đơn là một trường.\n"),
			"translated_from: content/en/alg/VIII/01_s1_simple_rings.md",
			"source_content_sha256: "+old),
	})
	if err := fixSeal(nil); err != nil {
		t.Fatal(err)
	}
	if got := readSection(t, root, viName).Meta.SourceSHA256; got != old {
		t.Errorf("the translation records %s, want it left at the old English hash %s", got, old)
	}
}
