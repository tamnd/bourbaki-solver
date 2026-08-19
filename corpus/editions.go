package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// EditionsManifest is manifests/editions.yaml, every place two printings of the
// same text differ and both of them are right.
//
// It is not errata.yaml. That file is for a printing that is wrong, and it
// carries the words the page has and the words to read instead. This is the
// other thing. The 2023 English Algebra chapter VIII prints twenty exercises in
// § 2 and the 2012 French prints nineteen, the nineteen are the same nineteen in
// the same order, and neither volume is at fault. There is nothing to correct,
// only something to know.
//
// Somewhere to say so is worth a file of its own because of what the difference
// looks like from outside. An exercise in one tree and not the other is exactly
// the shape of a page that never got read, so the count gets noticed, chased
// back through the extraction, and settled again against the PDF by whoever
// notices next. It was settled once by reading the French page after exercise
// 19, which opens § 3. This is where that reading is kept.
//
// A person writes this file; nothing generated writes it.
type EditionsManifest struct {
	Differences []Difference `yaml:"differences"`
}

// Difference is one exercise the printings do not agree about.
//
// It is written per exercise rather than per § because the § is not what
// differs. Nineteen of the twenty are in both books, and a record that said the
// two printings of § 2 disagree would stop anybody ever checking the nineteen
// again.
type Difference struct {
	Book     string `yaml:"book"`
	Chapter  string `yaml:"chapter"`
	Section  int    `yaml:"section"`
	Appendix bool   `yaml:"appendix,omitempty"`
	Exercise int    `yaml:"exercise"`

	// In are the languages of the printings that have it. The ones that do not
	// are every other printing of the chapter, which is not written down,
	// because a volume the corpus has not extracted yet has to be able to arrive
	// without every entry here needing a hand edit.
	In []string `yaml:"in"`

	// Why is what was read to settle it, in the words of whoever read it.
	Why string `yaml:"why"`
}

// Key is the exercise a difference is about, in the form the audit counts.
func (d Difference) Key() string {
	return fmt.Sprintf("%s/%s/%s/%d", d.Book, d.Chapter,
		ExerciseDir(d.Section, d.Appendix), d.Exercise)
}

// LoadEditions reads manifests/editions.yaml. A missing file is an empty
// manifest, since most corpora hold one printing of anything and have nothing to
// say here.
func LoadEditions(root string) (*EditionsManifest, error) {
	path := EditionsPath(root)
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &EditionsManifest{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m EditionsManifest
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
// An entry naming no printing accounts for nothing, and an entry with no reason
// on it is somebody having silenced a count they did not settle. Both of those
// are worse than the finding they were written to answer, so they stop the load
// rather than being tolerated.
func (m *EditionsManifest) check(path string) error {
	seen := map[string]bool{}
	for _, d := range m.Differences {
		switch {
		case d.Book == "":
			return fmt.Errorf("%s: an entry has no book", path)
		case d.Chapter == "":
			return fmt.Errorf("%s: an entry of %s has no chapter", path, d.Book)
		case d.Exercise == 0:
			return fmt.Errorf("%s: an entry of %s chapter %s has no exercise number",
				path, d.Book, d.Chapter)
		case len(d.In) == 0:
			return fmt.Errorf("%s: %s is in no printing, so it is not a difference", path, d.Key())
		case d.Why == "":
			return fmt.Errorf("%s: %s has no reason on it, and a count nobody settled is not accounted for",
				path, d.Key())
		case seen[d.Key()]:
			return fmt.Errorf("%s: %s is entered twice", path, d.Key())
		}
		seen[d.Key()] = true
	}
	return nil
}

// Printings is the set of printings that have an exercise, or nil when the
// manifest says nothing about it. The key is the one Difference.Key writes.
func (m *EditionsManifest) Printings(key string) map[string]bool {
	for _, d := range m.Differences {
		if d.Key() != key {
			continue
		}
		out := map[string]bool{}
		for _, l := range d.In {
			out[l] = true
		}
		return out
	}
	return nil
}

// Bytes renders the manifest, sorted, so that two people adding an entry on the
// same day do not conflict over where it went.
func (m *EditionsManifest) Bytes() ([]byte, error) {
	sorted := make([]Difference, len(m.Differences))
	copy(sorted, m.Differences)
	sort.Slice(sorted, func(i, j int) bool { return sorted[i].Key() < sorted[j].Key() })
	enc, err := yaml.Marshal(&EditionsManifest{Differences: sorted})
	if err != nil {
		return nil, err
	}
	head := "# Where two printings of the same text differ and both are right.\n" +
		"# Written by hand, read by bourbaki audit. A printing that is wrong goes\n" +
		"# in errata.yaml instead.\n"
	return append([]byte(head), enc...), nil
}

// EditionsPath is where the manifest lives inside a corpus checkout.
func EditionsPath(root string) string {
	return filepath.Join(root, "manifests", "editions.yaml")
}
