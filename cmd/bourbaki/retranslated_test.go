package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The case the command exists for, and the one fix reseal refuses. The English
// gained a clause, the Vietnamese was given it by hand, and there is now nothing
// stale about the pair except the field that says so.
func TestRetranslatedRecordsASourceWhoseWordsMoved(t *testing.T) {
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
	carried := "Cho $A$ là một vành noether địa phương chiều một.\n"
	writeAll(t, root, map[string]string{
		enName: sealFile(rewritten, corpus.ContentSHA256(rewritten)),
		viName: sealFile(carried, corpus.ContentSHA256(carried),
			"translated_from: "+enName,
			"source_content_sha256: "+corpus.ContentSHA256(en)),
	})

	if err := fixRetranslated([]string{filepath.Join(root, filepath.FromSlash(viName))}); err != nil {
		t.Fatal(err)
	}
	if got, want := readSection(t, root, viName).Meta.SourceSHA256, corpus.ContentSHA256(rewritten); got != want {
		t.Errorf("the record is %s, want it moved on to %s", got, want)
	}
	// fix reseal must still refuse the same pair, since nothing it can prove has
	// changed: the two commands answer different questions about one field.
	writeAll(t, root, map[string]string{
		viName: sealFile(carried, corpus.ContentSHA256(carried),
			"translated_from: "+enName,
			"source_content_sha256: "+corpus.ContentSHA256(en)),
	})
	if err := fixReseal(nil); err != nil {
		t.Fatal(err)
	}
	if got, want := readSection(t, root, viName).Meta.SourceSHA256, corpus.ContentSHA256(en); got != want {
		t.Errorf("fix reseal moved a record whose words moved: %s, want it left at %s", got, want)
	}
}

// The guard that makes this safe to have at all. There is no sweep: a command
// that records an assertion rather than a proof has to cost one argument per
// file, or blessing a hundred disagreements is one careless line.
func TestRetranslatedRefusesToRunWithNoPathsNamed(t *testing.T) {
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

	err := fixRetranslated(nil)
	if err == nil {
		t.Fatal("with no paths named it ran, want a refusal")
	}
	if !strings.Contains(err.Error(), "no sweep") {
		t.Errorf("it refused with %q, want it to say why", err)
	}
	if got, want := readSection(t, root, viName).Meta.SourceSHA256, corpus.ContentSHA256(en); got != want {
		t.Errorf("it refused and wrote anyway: %s, want %s", got, want)
	}
}

// Only the file named. The other translation of the same source is just as
// stale and nobody said anything about it.
func TestRetranslatedRecordsOnlyTheFileItWasGiven(t *testing.T) {
	en := "Let $A$ be a noetherian ring.\n"
	vi := "Cho $A$ là một vành noether.\n"
	fr := "Soit $A$ un anneau noethérien.\n"
	enName := "content/en/alg/VIII/01_s1_simple_rings.md"
	viName := "content/vi/alg/VIII/01_s1_vanh_don.md"
	frName := "content/fr/alg/VIII/01_s1_anneaux_simples.md"
	root := resealCorpus(t, map[string]string{
		enName: sealFile(en, corpus.ContentSHA256(en)),
		viName: sealFile(vi, corpus.ContentSHA256(vi),
			"translated_from: "+enName,
			"source_content_sha256: "+corpus.ContentSHA256(en)),
		frName: sealFile(fr, corpus.ContentSHA256(fr),
			"translated_from: "+enName,
			"source_content_sha256: "+corpus.ContentSHA256(en)),
	})
	rewritten := "Let $A$ be a noetherian local ring of dimension one.\n"
	writeAll(t, root, map[string]string{enName: sealFile(rewritten, corpus.ContentSHA256(rewritten))})

	if err := fixRetranslated([]string{filepath.Join(root, filepath.FromSlash(viName))}); err != nil {
		t.Fatal(err)
	}
	if got, want := readSection(t, root, frName).Meta.SourceSHA256, corpus.ContentSHA256(en); got != want {
		t.Errorf("the French was recorded without being named: %s, want %s", got, want)
	}
}

// -check says and does not write, so the drift can be read before it is signed
// for.
func TestRetranslatedChecksWithoutWriting(t *testing.T) {
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

	if err := fixRetranslated([]string{"-check", filepath.Join(root, filepath.FromSlash(viName))}); err != nil {
		t.Fatal(err)
	}
	if got, want := readSection(t, root, viName).Meta.SourceSHA256, corpus.ContentSHA256(en); got != want {
		t.Errorf("-check wrote: %s, want it left at %s", got, want)
	}
}

// A translation that is already current is not rewritten and is said so. Naming
// a file that needed nothing is a mistake worth hearing about and not worth
// failing over.
func TestRetranslatedLeavesACurrentTranslationAlone(t *testing.T) {
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
	if err := fixRetranslated([]string{filepath.Join(root, filepath.FromSlash(viName))}); err != nil {
		t.Fatal(err)
	}
	if got, want := readSection(t, root, viName).Meta.SourceSHA256, corpus.ContentSHA256(en); got != want {
		t.Errorf("a current translation was rewritten: %s, want %s", got, want)
	}
}

// An exercise is the other schema carrying the field, and the one fix seal
// could not look at. 7875 of the 8657 files that carry it are exercises.
func TestRetranslatedRecordsAnExerciseToo(t *testing.T) {
	en := "Show that $A$ is noetherian.\n"
	vi := "Chứng minh rằng $A$ là noether.\n"
	enName := "content/en/alg/VIII/exercises/s1/07.md"
	viName := "content/vi/alg/VIII/exercises/s1/07.md"
	root := resealCorpus(t, map[string]string{
		enName: exerciseFile(en),
		viName: exerciseFile(vi,
			"translated_from: "+enName,
			"source_content_sha256: "+corpus.ContentSHA256(en)),
	})
	rewritten := "Show that $A$ is noetherian and local.\n"
	writeAll(t, root, map[string]string{enName: exerciseFile(rewritten)})

	if err := fixRetranslated([]string{filepath.Join(root, filepath.FromSlash(viName))}); err != nil {
		t.Fatal(err)
	}
	if got, want := readExercise(t, root, viName).Meta.SourceSHA256, corpus.ContentSHA256(rewritten); got != want {
		t.Errorf("the exercise records %s, want %s", got, want)
	}
}

// A path that is not a translation at all is a refusal and not a shrug, unlike
// fix reseal: there the argument narrows a sweep, here it is the whole of what
// the command was told, so a typo in it means nothing asked for was recorded.
func TestRetranslatedRefusesAPathThatIsNotATranslation(t *testing.T) {
	en := "Let $A$ be a noetherian ring.\n"
	enName := "content/en/alg/VIII/01_s1_simple_rings.md"
	root := resealCorpus(t, map[string]string{
		enName:               sealFile(en, corpus.ContentSHA256(en)),
		"manifests/toc/x.md": "not a section at all\n",
	})
	err := fixRetranslated([]string{filepath.Join(root, "manifests", "toc", "x.md")})
	if err == nil {
		t.Fatal("it accepted a path that is not a translation, want a refusal")
	}
	if !strings.Contains(err.Error(), "not a translation") {
		t.Errorf("it refused with %q, want it to name the trouble", err)
	}
}

// The drift is printed because the printing is the point: what is being signed
// for has to be visible next to the signature.
func TestProseDriftNamesWhatMovedAndIgnoresTheMathematics(t *testing.T) {
	was := "Let $\\Lambda$ be a noetherian ring.\n"
	now := "Let $A$ be a noetherian local ring.\n"
	lines := proseDrift(was, now)
	joined := strings.Join(lines, "\n")
	if !strings.Contains(joined, "-") || !strings.Contains(joined, "+") {
		t.Fatalf("the drift is %q, want a line gone and a line gained", joined)
	}
	if strings.Contains(joined, "Lambda") || strings.Contains(joined, "$A$") {
		t.Errorf("the drift is %q, want the mathematics left out of it", joined)
	}

	same := proseDrift("Let $\\Lambda$ be noetherian.\n", "Let $A$ be noetherian.\n")
	if len(same) != 1 || !strings.Contains(same[0], "fix reseal") {
		t.Errorf("a pair whose prose did not move reports %q, want it sent to fix reseal", same)
	}
}
