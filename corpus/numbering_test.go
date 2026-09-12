package corpus

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func writeNumbering(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "manifests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(NumberingPath(root), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

// A printing with a hole in its exercise numbers is rare, so a missing file is
// an empty manifest and not an error.
func TestAMissingNumberingManifestIsEmpty(t *testing.T) {
	m, err := LoadNumbering(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Skipped) != 0 {
		t.Errorf("read %v out of nothing", m.Skipped)
	}
	if got := m.Skips(Ref{Book: "alg", Chapter: "III", Section: 2}); got != nil {
		t.Errorf("an empty manifest skips %v", got)
	}
}

func TestASkippedNumberReadsBackForItsSection(t *testing.T) {
	root := writeNumbering(t, `skipped:
  - book: alg
    chapter: III
    section: 2
    numbers: [4]
    why: the English of 1998 and the French of 2007 both run 1, 2, 3 a) to e), 5
`)
	m, err := LoadNumbering(root)
	if err != nil {
		t.Fatal(err)
	}
	if got := m.Skips(Ref{Book: "alg", Chapter: "III", Section: 2}); !slices.Equal(got, []int{4}) {
		t.Errorf("§ 2 of chapter III skips %v, want [4]", got)
	}
	if got := m.Skips(Ref{Book: "alg", Chapter: "III", Section: 3}); got != nil {
		t.Errorf("§ 3 of chapter III skips %v, and nothing was written about it", got)
	}
	if got := m.Skips(Ref{Book: "alg", Chapter: "III", Section: 2, Appendix: true}); got != nil {
		t.Errorf("the appendix skips %v, and the entry is about the §", got)
	}
}

// An entry nobody can act on is worse than the run-on it was written to answer,
// so the load stops rather than tolerating one.
func TestANumberingManifestRefusesAnEntryThatWouldGoUnread(t *testing.T) {
	for _, s := range []struct{ name, body, want string }{
		{"no book", "skipped:\n  - chapter: III\n    numbers: [4]\n    why: read\n", "no book"},
		{"no chapter", "skipped:\n  - book: alg\n    numbers: [4]\n    why: read\n", "no chapter"},
		{"no numbers", "skipped:\n  - book: alg\n    chapter: III\n    section: 2\n    why: read\n", "not a hole"},
		{"no reason", "skipped:\n  - book: alg\n    chapter: III\n    section: 2\n    numbers: [4]\n", "no reason on it"},
		{"number below one", "skipped:\n  - book: alg\n    chapter: III\n    section: 2\n    numbers: [0]\n    why: read\n", "counts from there"},
		{"entered twice", "skipped:\n  - book: alg\n    chapter: III\n    section: 2\n    numbers: [4]\n    why: read\n" +
			"  - book: alg\n    chapter: III\n    section: 2\n    numbers: [7]\n    why: read\n", "entered twice"},
	} {
		t.Run(s.name, func(t *testing.T) {
			_, err := LoadNumbering(writeNumbering(t, s.body))
			if err == nil {
				t.Fatalf("%s was let through", s.name)
			}
			if !strings.Contains(err.Error(), s.want) {
				t.Errorf("%s said %q, want it to mention %q", s.name, err, s.want)
			}
		})
	}
}

func TestNumberingBytesSortsSoTwoPeopleDoNotCollide(t *testing.T) {
	m := &NumberingManifest{Skipped: []Skipped{
		{Book: "top", Chapter: "I", Section: 9, Numbers: []int{3}, Why: "read"},
		{Book: "alg", Chapter: "III", Section: 2, Numbers: []int{4}, Why: "read"},
	}}
	b, err := m.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	got := string(b)
	if !strings.HasPrefix(got, "# The exercise numbers") {
		t.Errorf("the file opens %q", got[:min(len(got), 40)])
	}
	if strings.Index(got, "alg") > strings.Index(got, "top") {
		t.Errorf("the entries came out unsorted:\n%s", got)
	}
}
