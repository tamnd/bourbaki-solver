package glossary

import (
	"os"
	"path/filepath"
	"testing"
)

// The disk filled during a run of glossary translate, and manifests/glossary.yaml
// came out of it zero bytes long: os.WriteFile empties the file and then fills
// it, and the run died in between. A thousand renderings went with it and only a
// checkout brought them back.
//
// A directory nobody may write to is the same failure arriving earlier, and the
// thing to check is the same one: whatever happens, the file that was there is
// still there and still says what it said.
func TestAFailedWriteLeavesTheGlossaryAsItWas(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes to a read only directory, so there is no failure to see")
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "glossary.yaml")

	g := &Glossary{Terms: []Term{{EN: "ring", VI: "vành", ZH: "环"}}}
	if _, _, err := g.Save(path); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}

	if err := os.Chmod(dir, 0o500); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o700) })

	g.Terms = append(g.Terms, Term{EN: "field", VI: "trường"})
	if _, _, err := g.Save(path); err == nil {
		t.Fatal("a save into a directory nobody may write to reported success")
	}

	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("the glossary is gone after a failed save: %v", err)
	}
	if len(after) == 0 {
		t.Fatal("the glossary is empty after a failed save, which is the whole bug")
	}
	if string(after) != string(before) {
		t.Errorf("the glossary changed under a failed save:\n%s", after)
	}
}

// And the ordinary case still writes what it says, at the mode the repository
// keeps its manifests at, with nothing left beside it.
func TestWriteLeavesNoTemporaryFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "glossary.yaml")
	if err := (Glossary{Version: 3, Terms: []Term{{EN: "ring", VI: "vành"}}}).Write(path); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != "glossary.yaml" {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("the directory holds %v, want the glossary alone", names)
	}
	info, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm() != 0o644 {
		t.Errorf("mode = %v, want 0644 as the other manifests are", info.Mode().Perm())
	}
}
