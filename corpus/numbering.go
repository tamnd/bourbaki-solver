package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// NumberingManifest is manifests/numbering.yaml, the exercise numbers a § is
// printed without.
//
// The assembler numbers the exercises of a § from 1 and asks the page for the
// number it is up to, because a number it cannot find is nearly always a marker
// the reading lost, and taking the next one that happens to be legible would
// bury the loss instead of reporting it. Nearly always, but not always: § 2 of
// chapter III of Algebra is printed 1, 2, 3 a) to e), 5, in the English of 1998
// and in the French of 2007 alike. There is no exercise 4 in either book.
//
// Without somewhere to say that, the assembler waits for a 4 that is never
// coming and every exercise from 5 to 26 ends up inside the body of 3. That is
// what this file is for, and it is not errata.yaml: nothing here is wrong and
// there are no words to read instead. It is not editions.yaml either, because
// the two printings agree.
//
// A person writes this file; nothing generated writes it.
type NumberingManifest struct {
	Skipped []Skipped `yaml:"skipped"`
}

// Skipped is one § and the numbers its printing does not use.
//
// Per § rather than per exercise, because a hole is a fact about the run of
// numbers and not about any exercise: there is no exercise 4 of § 2 of chapter
// III of Algebra to hang it on.
type Skipped struct {
	Book     string `yaml:"book"`
	Chapter  string `yaml:"chapter"`
	Section  int    `yaml:"section"`
	Appendix bool   `yaml:"appendix,omitempty"`

	// Numbers are the numbers the printing sets no exercise against, in the
	// order they would have come.
	Numbers []int `yaml:"numbers"`

	// Why is what was read to settle it, in the words of whoever read it.
	Why string `yaml:"why"`
}

// Key is the § an entry is about, in the form the manifest is sorted by.
func (s Skipped) Key() string {
	return fmt.Sprintf("%s/%s/%s", s.Book, s.Chapter, ExerciseDir(s.Section, s.Appendix))
}

// LoadNumbering reads manifests/numbering.yaml. A missing file is an empty
// manifest, since a printing with a hole in its exercise numbers is rare enough
// that most corpora have nothing to say here.
func LoadNumbering(root string) (*NumberingManifest, error) {
	path := NumberingPath(root)
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &NumberingManifest{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m NumberingManifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	if err := m.check(path); err != nil {
		return nil, err
	}
	return &m, nil
}

// check refuses a manifest that would go unread.
//
// An entry with no reason on it is somebody having silenced a run-on they did
// not settle, which is worse than the run-on, so it stops the load. So does a
// number below 1, which no printing counts from, and a § entered twice, where
// whichever entry lost would be read by nobody.
func (m *NumberingManifest) check(path string) error {
	seen := map[string]bool{}
	for _, s := range m.Skipped {
		switch {
		case s.Book == "":
			return fmt.Errorf("%s: an entry has no book", path)
		case s.Chapter == "":
			return fmt.Errorf("%s: an entry of %s has no chapter", path, s.Book)
		case len(s.Numbers) == 0:
			return fmt.Errorf("%s: %s skips no number, so it is not a hole", path, s.Key())
		case s.Why == "":
			return fmt.Errorf("%s: %s has no reason on it, and a run-on nobody settled is not accounted for",
				path, s.Key())
		case seen[s.Key()]:
			return fmt.Errorf("%s: %s is entered twice", path, s.Key())
		}
		for _, n := range s.Numbers {
			if n < 1 {
				return fmt.Errorf("%s: %s skips exercise %d, and no printing counts from there",
					path, s.Key(), n)
			}
		}
		seen[s.Key()] = true
	}
	return nil
}

// Skips is the numbers a § is printed without, or nil when the manifest says
// nothing about it.
func (m *NumberingManifest) Skips(r Ref) []int {
	if m == nil {
		return nil
	}
	key := fmt.Sprintf("%s/%s/%s", r.Book, r.Chapter, ExerciseDir(r.Section, r.Appendix))
	for _, s := range m.Skipped {
		if s.Key() == key {
			return s.Numbers
		}
	}
	return nil
}

// Bytes renders the manifest, sorted, so that two people adding an entry on the
// same day do not conflict over where it went.
func (m *NumberingManifest) Bytes() ([]byte, error) {
	sorted := make([]Skipped, len(m.Skipped))
	copy(sorted, m.Skipped)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key() < sorted[j].Key() })
	enc, err := yaml.Marshal(&NumberingManifest{Skipped: sorted})
	if err != nil {
		return nil, err
	}
	head := "# The exercise numbers a § is printed without.\n" +
		"# Written by hand, read by bourbaki assemble and bourbaki audit. A printing\n" +
		"# that is wrong goes in errata.yaml, and two printings that differ and are\n" +
		"# both right go in editions.yaml.\n"
	return append([]byte(head), enc...), nil
}

// NumberingPath is where the manifest lives inside a corpus checkout.
func NumberingPath(root string) string {
	return filepath.Join(root, "manifests", "numbering.yaml")
}
