package corpus

import (
	"os"
	"path/filepath"
	"testing"
)

func writeEditions(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "manifests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(EditionsPath(root), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// Most corpora hold one printing of anything and have nothing to say here, so a
// missing file is an empty manifest and not an error.
func TestAMissingEditionsManifestIsEmpty(t *testing.T) {
	m, err := LoadEditions(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Differences) != 0 {
		t.Errorf("read %v out of nothing", m.Differences)
	}
}

func TestADifferenceReadsBackAndAnswersForItsExercise(t *testing.T) {
	root := writeEditions(t, `differences:
  - book: alg
    chapter: VIII
    section: 2
    exercise: 20
    in: [en]
    why: the French printing of 2012 stops at nineteen
`)
	m, err := LoadEditions(root)
	if err != nil {
		t.Fatal(err)
	}
	in := m.Printings("alg/VIII/s2/20")
	if len(in) != 1 || !in["en"] {
		t.Errorf("the printings are %v, want the English alone", in)
	}
	if m.Printings("alg/VIII/s2/19") != nil {
		t.Error("an exercise the manifest says nothing about came back with an answer")
	}
}

// An entry with no reason on it is somebody having silenced a count they did
// not settle, which is worse than the finding it was written to answer. Saying
// so at load time costs nothing.
func TestAnEntryWithNoReasonIsRefused(t *testing.T) {
	root := writeEditions(t, `differences:
  - book: alg
    chapter: VIII
    section: 2
    exercise: 20
    in: [en]
`)
	if _, err := LoadEditions(root); err == nil {
		t.Error("an entry that settles nothing was loaded")
	}
}

// An entry in no printing at all is not a difference between printings, and an
// exercise entered twice is two readings of the same count with no saying which
// one is meant.
func TestAnEntryThatCannotBeReadIsRefused(t *testing.T) {
	for name, body := range map[string]string{
		"in no printing": `differences:
  - book: alg
    chapter: VIII
    section: 2
    exercise: 20
    in: []
    why: nowhere
`,
		"entered twice": `differences:
  - book: alg
    chapter: VIII
    section: 2
    exercise: 20
    in: [en]
    why: once
  - book: alg
    chapter: VIII
    section: 2
    exercise: 20
    in: [fr]
    why: and again
`,
	} {
		if _, err := LoadEditions(writeEditions(t, body)); err == nil {
			t.Errorf("%s was loaded", name)
		}
	}
}
