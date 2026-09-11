package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// resealCorpus is a corpus that is also a git checkout, because the whole of
// what fix reseal knows comes out of the history of the source file. The files
// are written and committed as given, so a test says what the source was; a
// second write and a second commit says what it became.
func resealCorpus(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("BOURBAKI_CORPUS", root)
	git := func(args ...string) {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", root}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.invalid",
			"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.invalid")
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
	}
	git("init", "-q", "-b", "main")
	writeAll(t, root, files)
	git("add", "-A")
	git("commit", "-q", "-m", "the corpus as the translator saw it")
	return root
}

func writeAll(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for name, text := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// exerciseFile is an exercise of the schema fix reseal has to read as well as
// a section, since 7875 of the 8657 files carrying a source_content_sha256 are
// exercises and fix seal could not look at one.
func exerciseFile(body string, extra ...string) string {
	head := []string{
		"book: alg", "chapter: VIII", "section: 1", "exercise: 7",
		"label: alg-viii-s1-ex-7", "lang: en", "has_hint: false", "starred: false",
	}
	head = append(head, extra...)
	return "---\n" + strings.Join(head, "\n") + "\n---\n\n" + body
}

func readExercise(t *testing.T, root, name string) corpus.File[corpus.ExerciseFrontMatter] {
	t.Helper()
	f, err := corpus.ReadFile[corpus.ExerciseFrontMatter](filepath.Join(root, filepath.FromSlash(name)))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

// The case the command exists for. The English said \Lambda where the printing
// has a roman A, somebody repaired it, and the repair is inside the dollars. The
// Vietnamese renders the words and copies the mathematics, so nothing it holds
// is out of date and there is no reason to ask for it again.
func TestResealMovesTheRecordWhenOnlyTheMathematicsMoved(t *testing.T) {
	en := "Let $\\Lambda$ be a noetherian ring and $M$ a $\\Lambda$-module.\n"
	vi := "Cho $\\Lambda$ là một vành noether và $M$ một $\\Lambda$-môđun.\n"
	enName := "content/en/alg/VIII/01_s1_simple_rings.md"
	viName := "content/vi/alg/VIII/01_s1_vanh_don.md"
	root := resealCorpus(t, map[string]string{
		enName: sealFile(en, corpus.ContentSHA256(en)),
		viName: sealFile(vi, corpus.ContentSHA256(vi),
			"translated_from: "+enName,
			"source_content_sha256: "+corpus.ContentSHA256(en)),
	})

	repaired := "Let $A$ be a noetherian ring and $M$ a $A$-module.\n"
	writeAll(t, root, map[string]string{enName: sealFile(repaired, corpus.ContentSHA256(repaired))})

	if err := fixReseal(nil); err != nil {
		t.Fatal(err)
	}
	if got, want := readSection(t, root, viName).Meta.SourceSHA256, corpus.ContentSHA256(repaired); got != want {
		t.Errorf("the record is %s, want it moved on to %s", got, want)
	}
}

// The danger this has to be proof against. A source whose words moved is a
// source the translation no longer renders, and a command that blessed it would
// be recording work nobody has done. L05 is the list of translations that need
// asking for again and a sweep must not shorten it by lying.
func TestResealLeavesARecordAloneWhenTheWordsMoved(t *testing.T) {
	en := "Let $A$ be a noetherian ring.\n"
	vi := "Cho $A$ là một vành noether.\n"
	enName := "content/en/alg/VIII/01_s1_simple_rings.md"
	viName := "content/vi/alg/VIII/01_s1_vanh_don.md"
	root := resealCorpus(t, map[string]string{
		enName: sealFile(en, corpus.ContentSHA256(en)),
		viName: sealFile(vi, corpus.ContentSHA256(vi),
			"translated_from: "+enName,
			"source_content_sha256: "+corpus.ContentSHA256(en)),
	})

	rewritten := "Let $A$ be a noetherian local ring of dimension one.\n"
	writeAll(t, root, map[string]string{enName: sealFile(rewritten, corpus.ContentSHA256(rewritten))})

	if err := fixReseal(nil); err != nil {
		t.Fatal(err)
	}
	if got, want := readSection(t, root, viName).Meta.SourceSHA256, corpus.ContentSHA256(en); got != want {
		t.Errorf("a translation whose source was rewritten was resealed: %s, want it left at %s", got, want)
	}
}

// A record no commit of its source ever hashed to is a record this command
// cannot account for, and that is exactly the one it must not move: something
// happened to that pair that nobody has an account of, and blessing it would
// bury the question. Twelve files of the corpus are in this state.
func TestResealLeavesARecordItCannotFindInTheHistory(t *testing.T) {
	en := "Let $A$ be a noetherian ring.\n"
	vi := "Cho $A$ là một vành noether.\n"
	enName := "content/en/alg/VIII/01_s1_simple_rings.md"
	viName := "content/vi/alg/VIII/01_s1_vanh_don.md"
	stranger := corpus.ContentSHA256("a body no commit of this file ever held\n")
	root := resealCorpus(t, map[string]string{
		enName: sealFile(en, corpus.ContentSHA256(en)),
		viName: sealFile(vi, corpus.ContentSHA256(vi),
			"translated_from: "+enName,
			"source_content_sha256: "+stranger),
	})

	repaired := "Let $B$ be a noetherian ring.\n"
	writeAll(t, root, map[string]string{enName: sealFile(repaired, corpus.ContentSHA256(repaired))})

	if err := fixReseal(nil); err != nil {
		t.Fatal(err)
	}
	if got := readSection(t, root, viName).Meta.SourceSHA256; got != stranger {
		t.Errorf("a record with no history behind it was moved to %s, want it left at %s", got, stranger)
	}
}

// 91 per cent of the files that carry a source_content_sha256 are exercises,
// and fix seal would not look at one. This command reads both schemas.
func TestResealMovesAnExerciseRecordToo(t *testing.T) {
	en := "Show that $\\Lambda$ is noetherian.\n"
	vi := "Chứng minh rằng $\\Lambda$ là noether.\n"
	enName := "content/en/alg/VIII/exercises/s1/07.md"
	viName := "content/vi/alg/VIII/exercises/s1/07.md"
	root := resealCorpus(t, map[string]string{
		enName: exerciseFile(en),
		viName: exerciseFile(vi, "translated_from: "+enName,
			"source_content_sha256: "+corpus.ContentSHA256(en)),
	})

	repaired := "Show that $A$ is noetherian.\n"
	writeAll(t, root, map[string]string{enName: exerciseFile(repaired)})

	if err := fixReseal(nil); err != nil {
		t.Fatal(err)
	}
	if got, want := readExercise(t, root, viName).Meta.SourceSHA256, corpus.ContentSHA256(repaired); got != want {
		t.Errorf("the exercise record is %s, want %s", got, want)
	}
}

func TestResealCheckMovesNothing(t *testing.T) {
	en := "Let $\\Lambda$ be a ring.\n"
	vi := "Cho $\\Lambda$ là một vành.\n"
	enName := "content/en/alg/VIII/01_s1_simple_rings.md"
	viName := "content/vi/alg/VIII/01_s1_vanh_don.md"
	root := resealCorpus(t, map[string]string{
		enName: sealFile(en, corpus.ContentSHA256(en)),
		viName: sealFile(vi, corpus.ContentSHA256(vi),
			"translated_from: "+enName,
			"source_content_sha256: "+corpus.ContentSHA256(en)),
	})
	repaired := "Let $A$ be a ring.\n"
	writeAll(t, root, map[string]string{enName: sealFile(repaired, corpus.ContentSHA256(repaired))})

	if err := fixReseal([]string{"-check"}); err != nil {
		t.Fatal(err)
	}
	if got, want := readSection(t, root, viName).Meta.SourceSHA256, corpus.ContentSHA256(en); got != want {
		t.Errorf("-check wrote %s, want the file left at %s", got, want)
	}
}

// The body is not touched. This command writes one field of the front matter
// and a translation that came out of it with a word changed would be a
// translation nobody asked for.
func TestResealLeavesTheBodyAlone(t *testing.T) {
	en := "Let $\\Lambda$ be a ring.\n"
	vi := "Cho $\\Lambda$ là một vành.\n"
	enName := "content/en/alg/VIII/01_s1_simple_rings.md"
	viName := "content/vi/alg/VIII/01_s1_vanh_don.md"
	root := resealCorpus(t, map[string]string{
		enName: sealFile(en, corpus.ContentSHA256(en)),
		viName: sealFile(vi, corpus.ContentSHA256(vi),
			"translated_from: "+enName,
			"source_content_sha256: "+corpus.ContentSHA256(en)),
	})
	repaired := "Let $A$ be a ring.\n"
	writeAll(t, root, map[string]string{enName: sealFile(repaired, corpus.ContentSHA256(repaired))})

	if err := fixReseal(nil); err != nil {
		t.Fatal(err)
	}
	if got := readSection(t, root, viName).Body; got != vi {
		t.Errorf("the body is %q, want %q", got, vi)
	}
}

// The paths bind, for fix seal's reason. A command that rewrites the field
// telling a stale translation from a current one is the last one that should do
// more than it was asked, and fix seal resealing 209 files under a run that
// named one is the incident that rule came out of.
func TestResealMovesOnlyTheFileItWasGiven(t *testing.T) {
	en := "Let $\\Lambda$ be a ring.\n"
	asked := "content/vi/alg/VIII/01_s1_vanh_don.md"
	other := "content/vi/alg/VIII/02_s2_vanh_nua_don.md"
	enName := "content/en/alg/VIII/01_s1_simple_rings.md"
	root := resealCorpus(t, map[string]string{
		enName: sealFile(en, corpus.ContentSHA256(en)),
		asked: sealFile("Cho $\\Lambda$ là một vành.\n", corpus.ContentSHA256("a"),
			"translated_from: "+enName, "source_content_sha256: "+corpus.ContentSHA256(en)),
		other: sealFile("Một sửa tay của người khác.\n", corpus.ContentSHA256("b"),
			"translated_from: "+enName, "source_content_sha256: "+corpus.ContentSHA256(en)),
	})
	repaired := "Let $A$ be a ring.\n"
	writeAll(t, root, map[string]string{enName: sealFile(repaired, corpus.ContentSHA256(repaired))})

	if err := fixReseal([]string{filepath.Join(root, filepath.FromSlash(asked))}); err != nil {
		t.Fatal(err)
	}
	if got, want := readSection(t, root, asked).Meta.SourceSHA256, corpus.ContentSHA256(repaired); got != want {
		t.Errorf("the file that was asked for records %s, want %s", got, want)
	}
	if got, want := readSection(t, root, other).Meta.SourceSHA256, corpus.ContentSHA256(en); got != want {
		t.Errorf("a file nobody asked about was resealed: %s, want %s", got, want)
	}
}

// A corpus that is not a git checkout has no history to read, and the command
// has to say nothing rather than fail: it is one of a chain of repairs and a
// corpus unpacked from a tarball is still a corpus.
func TestResealSaysNothingWithNoHistoryToRead(t *testing.T) {
	en := "Let $A$ be a ring.\n"
	enName := "content/en/alg/VIII/01_s1_simple_rings.md"
	viName := "content/vi/alg/VIII/01_s1_vanh_don.md"
	old := corpus.ContentSHA256("Let $\\Lambda$ be a ring.\n")
	root := sealCorpus(t, map[string]string{
		enName: sealFile(en, corpus.ContentSHA256(en)),
		viName: sealFile("Cho $A$ là một vành.\n", corpus.ContentSHA256("x"),
			"translated_from: "+enName, "source_content_sha256: "+old),
	})
	if err := fixReseal(nil); err != nil {
		t.Fatalf("a corpus with no git history failed the run: %v", err)
	}
	if got := readSection(t, root, viName).Meta.SourceSHA256; got != old {
		t.Errorf("a record was moved with no history to justify it: %s, want %s", got, old)
	}
}
