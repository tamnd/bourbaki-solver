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

// The manifest records the same hash a second time. assemble -check compares
// the manifest it would write against the committed one, so a section sealed
// without its row is a corpus that cannot pass its own check, and the volumes
// this command is for are exactly the ones assemble refuses to run on.
func TestSealWritesTheManifestRowToo(t *testing.T) {
	body := "Le centre d’un anneau simple est un corps.\n"
	name := "content/fr/alg/VIII/01_s1_anneaux_simples.md"
	old := corpus.ContentSHA256("le corps d’avant")
	root := sealCorpus(t, map[string]string{name: sealFile(body, old)})
	m := &corpus.SectionsManifest{Books: []corpus.BookSections{{
		ID: "alg-viii-fr",
		Chapters: []corpus.ChapterSections{{
			Chapter: "VIII",
			Sections: []corpus.SectionRecord{
				{Kind: corpus.KindSection, Section: 1, Path: name, ContentSHA256: old},
			},
		}},
	}}}
	if err := m.Save(root); err != nil {
		t.Fatal(err)
	}
	if err := fixSeal(nil); err != nil {
		t.Fatal(err)
	}
	got, err := corpus.LoadSections(root)
	if err != nil {
		t.Fatal(err)
	}
	if row := got.Books[0].Chapters[0].Sections[0].ContentSHA256; row != corpus.ContentSHA256(body) {
		t.Errorf("the manifest row is %s, want %s", row, corpus.ContentSHA256(body))
	}
}

// A corpus in order comes out of a run with the manifest it went in with, down
// to the byte. The manifest is a thousand lines of generated YAML and a run
// that rewrites it for nothing is a diff nobody can read.
func TestSealLeavesTheManifestAloneWhenNothingMoved(t *testing.T) {
	body := "Every field is a simple ring.\n"
	name := "content/en/alg/VIII/01_s1_simple_rings.md"
	root := sealCorpus(t, map[string]string{name: sealFile(body, corpus.ContentSHA256(body))})
	m := &corpus.SectionsManifest{Books: []corpus.BookSections{{
		ID: "alg-viii",
		Chapters: []corpus.ChapterSections{{
			Chapter: "VIII",
			Sections: []corpus.SectionRecord{
				{Kind: corpus.KindSection, Section: 1, Path: name,
					ContentSHA256: corpus.ContentSHA256(body)},
			},
		}},
	}}}
	if err := m.Save(root); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(corpus.SectionsPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := fixSeal(nil); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(corpus.SectionsPath(root))
	if err != nil {
		t.Fatal(err)
	}
	if string(after) != string(before) {
		t.Errorf("the manifest was rewritten:\n%s", after)
	}
}

// A row can be stale on its own. Seal a section today and the row goes with it,
// but a section that was corrected and sealed before this command existed left
// a row behind that nothing has been through since, and the file it describes
// is in order. So the walk offers every section to the manifest and not only
// the ones it wrote.
func TestSealFixesARowUnderAFileThatIsAlreadySealed(t *testing.T) {
	body := "The centre of a simple ring is a field.\n"
	name := "content/en/alg/VIII/01_s1_simple_rings.md"
	root := sealCorpus(t, map[string]string{name: sealFile(body, corpus.ContentSHA256(body))})
	m := &corpus.SectionsManifest{Books: []corpus.BookSections{{
		ID: "alg-viii",
		Chapters: []corpus.ChapterSections{{
			Chapter: "VIII",
			Sections: []corpus.SectionRecord{
				{Kind: corpus.KindSection, Section: 1, Path: name,
					ContentSHA256: corpus.ContentSHA256("the body before the correction")},
			},
		}},
	}}}
	if err := m.Save(root); err != nil {
		t.Fatal(err)
	}
	if err := fixSeal(nil); err != nil {
		t.Fatal(err)
	}
	got, err := corpus.LoadSections(root)
	if err != nil {
		t.Fatal(err)
	}
	if row := got.Books[0].Chapters[0].Sections[0].ContentSHA256; row != corpus.ContentSHA256(body) {
		t.Errorf("the manifest row is %s, want %s", row, corpus.ContentSHA256(body))
	}
}

// The incident this command's paths were made to bind for. Two sections are
// unsealed, one is named, and the other must come back untouched. Before the
// paths bound, a run naming a single Vietnamese file resealed 209 of them and 4
// more in en-mt, because parseFlags returned the paths and the caller dropped
// them.
func TestSealSealsOnlyTheFileItWasGiven(t *testing.T) {
	asked := "content/vi/alg/VIII/01_s1_vanh_don.md"
	other := "content/vi/alg/VIII/02_s2_vanh_nua_don.md"
	mine, theirs := "Tâm của một vành đơn là một trường.\n", "Một sửa tay của người khác.\n"
	root := sealCorpus(t, map[string]string{
		asked: sealFile(mine, corpus.ContentSHA256("cũ")),
		other: sealFile(theirs, corpus.ContentSHA256("cũ")),
	})
	if err := fixSeal([]string{"-lang", "vi", filepath.Join(root, filepath.FromSlash(asked))}); err != nil {
		t.Fatal(err)
	}
	if got, want := readSection(t, root, asked).Meta.ContentSHA256, corpus.ContentSHA256(mine); got != want {
		t.Errorf("the file that was asked for has hash %s, want %s", got, want)
	}
	if got, want := readSection(t, root, other).Meta.ContentSHA256, corpus.ContentSHA256("cũ"); got != want {
		t.Errorf("a file nobody asked about was resealed: hash is %s, want %s", got, want)
	}
}

// A relative path is what anyone actually types, and the walk hands out
// absolute ones, so the two are made to meet before they are compared.
func TestSealTakesARelativePath(t *testing.T) {
	name := "content/vi/alg/VIII/01_s1_vanh_don.md"
	body := "Một vành đơn không có iđêan hai phía nào khác.\n"
	root := sealCorpus(t, map[string]string{name: sealFile(body, corpus.ContentSHA256("cũ"))})
	t.Chdir(root)
	if err := fixSeal([]string{name}); err != nil {
		t.Fatal(err)
	}
	if got, want := readSection(t, root, name).Meta.ContentSHA256, corpus.ContentSHA256(body); got != want {
		t.Errorf("hash is %s, want %s", got, want)
	}
}

// Exercises carry no hash of their own and eachSection does not descend into
// them, so naming one asks for nothing at all. Saying so beats reporting that
// zero sections were read and letting it pass for success.
func TestSealSaysSoWhenTheNamedFileIsNotASectionItCanSeal(t *testing.T) {
	name := "content/en/alg/VIII/exercises/s1/07.md"
	root := sealCorpus(t, map[string]string{name: sealFile("A body.\n", corpus.ContentSHA256("old"))})
	err := fixSeal([]string{filepath.Join(root, filepath.FromSlash(name))})
	if err == nil {
		t.Fatal("naming an exercise was accepted, want an error")
	}
	if !strings.Contains(err.Error(), "07.md") {
		t.Errorf("the error does not name the file: %v", err)
	}
}

// A path in one language under -lang another is a contradiction, and the file
// is filtered out of the walk before it is ever offered. That must read as the
// mistake it is rather than as a run that sealed nothing.
func TestSealSaysSoWhenThePathIsOutsideTheLanguageAsked(t *testing.T) {
	name := "content/vi/alg/VIII/01_s1_vanh_don.md"
	root := sealCorpus(t, map[string]string{name: sealFile("Thân bài.\n", corpus.ContentSHA256("cũ"))})
	err := fixSeal([]string{"-lang", "en", filepath.Join(root, filepath.FromSlash(name))})
	if err == nil {
		t.Fatal("a vi path under -lang en was accepted, want an error")
	}
	if got, want := readSection(t, root, name).Meta.ContentSHA256, corpus.ContentSHA256("cũ"); got != want {
		t.Errorf("the file was sealed anyway: hash is %s, want %s", got, want)
	}
}

// A path that is not there at all is a typo, and the run must not go on to
// reseal the corpus because the set of files to narrow to came out empty.
func TestSealRefusesAPathThatIsNotThere(t *testing.T) {
	name := "content/en/alg/VIII/01_s1_simple_rings.md"
	root := sealCorpus(t, map[string]string{name: sealFile("A body.\n", corpus.ContentSHA256("old"))})
	if err := fixSeal([]string{filepath.Join(root, "content/en/alg/VIII/99_s99_nothing.md")}); err == nil {
		t.Fatal("a path that is not there was accepted, want an error")
	}
	if got, want := readSection(t, root, name).Meta.ContentSHA256, corpus.ContentSHA256("old"); got != want {
		t.Errorf("the corpus was sealed after a bad path: hash is %s, want %s", got, want)
	}
}
