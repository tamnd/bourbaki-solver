package quality

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

// L09 is the rule that catches a glossary edited by hand.
//
// Save bumps the version when the renderings move, which closes half the hole.
// The other half is somebody opening manifests/glossary.yaml and typing, which
// goes round Save entirely and which happened while this was being written. The
// version is what every translated file records to say what it was written
// against, so if it does not move, 344 files go on reporting themselves current
// against terminology they have never seen.
func TestL09NoticesAHandEditedGlossary(t *testing.T) {
	cases := []struct {
		name    string
		version int
		terms   string
		want    bool
	}{
		{"nothing changed", 4, `- en: ring
  vi: vành
`, false},
		{"a rendering changed and the version did not", 4, `- en: ring
  vi: vòng
`, true},
		{"a rendering changed and the version did", 5, `- en: ring
  vi: vòng
`, false},
		{"a row was added and the version did not move", 4, `- en: ring
  vi: vành
- en: field
  vi: trường
`, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := committedGlossary(t, "version: 4\nterms:\n"+`- en: ring
  vi: vành
`)
			write(t, root, c.terms, c.version)
			got, err := l09(&Corpus{Root: root, Opt: Options{Base: "HEAD"}})
			if err != nil {
				t.Fatal(err)
			}
			if (len(got) > 0) != c.want {
				t.Errorf("l09 reported %v, want %v: %v", len(got) > 0, c.want, got)
			}
		})
	}
}

// Without a base revision there is nothing to compare against, and a rule with
// nothing to look at is not run rather than passed.
func TestL09NeedsARevisionToCompareAgainst(t *testing.T) {
	root := committedGlossary(t, "version: 4\nterms:\n- en: ring\n  vi: vành\n")
	if why := needGlossaryBase(&Corpus{Root: root}); why == "" {
		t.Error("the rule claimed it could run with no base revision")
	}
	if why := needGlossaryBase(&Corpus{Root: root, Opt: Options{Base: "HEAD"}}); why != "" {
		t.Errorf("the rule refused to run: %s", why)
	}
	if why := needGlossaryBase(&Corpus{Root: t.TempDir(), Opt: Options{Base: "HEAD"}}); why == "" {
		t.Error("the rule claimed it could run with no glossary")
	}
}

// A base revision with no glossary in it is the first commit of the file, and
// the rule has to survive it rather than fail the build on the day the file is
// introduced.
func TestL09SurvivesARevisionWithNoGlossaryInIt(t *testing.T) {
	root := t.TempDir()
	git(t, root, "init")
	git(t, root, "commit", "--allow-empty", "-m", "nothing yet")
	write(t, root, "- en: ring\n  vi: vành\n", 1)
	got, err := l09(&Corpus{Root: root, Opt: Options{Base: "HEAD"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("the first commit of the glossary was reported: %v", got)
	}
}

func committedGlossary(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	git(t, root, "init")
	if err := os.MkdirAll(filepath.Join(root, "manifests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifests", "glossary.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	git(t, root, "add", "manifests/glossary.yaml")
	git(t, root, "commit", "-m", "the glossary")
	return root
}

func write(t *testing.T, root, terms string, version int) {
	t.Helper()
	body := "version: " + itoa(version) + "\nterms:\n" + terms
	if err := os.MkdirAll(filepath.Join(root, "manifests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifests", "glossary.yaml"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var b []byte
	for n > 0 {
		b = append([]byte{byte('0' + n%10)}, b...)
		n /= 10
	}
	return string(b)
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=t", "GIT_AUTHOR_EMAIL=t@example.com",
		"GIT_COMMITTER_NAME=t", "GIT_COMMITTER_EMAIL=t@example.com")
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, out)
	}
}
