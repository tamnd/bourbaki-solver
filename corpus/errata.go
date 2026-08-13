package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// ErrataManifest is manifests/errata.yaml, every place a printing is wrong and
// the corpus keeps the printed words anyway.
//
// It is here and not in the content files because content/ is assembled. Every
// exercise file is a pure function of the pages and the tag set, CI diffs the
// assembly against what is committed on every push, and a correction written
// into a file by hand is gone the next time the assembler runs. So the erratum
// is recorded once, against the permanent label of the thing it corrects, and
// the assembler stamps it into the front matter the same way it stamps the tag.
// A person writes this file; nothing generated writes it.
//
// The label is used and not the tag because an erratum is about a printing and
// a tag is about a statement across all of them, and because an exercise can
// want a correction before anything has assigned it a tag.
type ErrataManifest struct {
	Entries []LabelErrata `yaml:"errata"`
}

// LabelErrata is what is wrong with one labelled piece of one printing.
type LabelErrata struct {
	Label string `yaml:"label"`
	// Lang is the printing the error is in. The English translation of 1974
	// and the French of 1981 are different books and an error in one is not an
	// error in the other; exercise 3 a) of VIII § 1 is exactly that case.
	Lang   string    `yaml:"lang"`
	Errata []Erratum `yaml:"errata"`
}

// LoadErrata reads manifests/errata.yaml. A missing file is an empty manifest,
// which is what most corpora will have most of the time.
func LoadErrata(root string) (*ErrataManifest, error) {
	path := ErrataPath(root)
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &ErrataManifest{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m ErrataManifest
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
// An erratum with no label attaches to nothing and an erratum with an empty
// says or read corrects nothing, and either one is a person having written
// something down in the belief it would be applied. Saying so at load time
// costs nothing and finding out by reading an assembled file costs a run.
func (m *ErrataManifest) check(path string) error {
	seen := map[string]bool{}
	for _, e := range m.Entries {
		switch {
		case e.Label == "":
			return fmt.Errorf("%s: an entry has no label", path)
		case e.Lang == "":
			return fmt.Errorf("%s: %s has no lang, and an erratum is against one printing", path, e.Label)
		case len(e.Errata) == 0:
			return fmt.Errorf("%s: %s lists no errata", path, e.Label)
		}
		key := e.Lang + " " + e.Label
		if seen[key] {
			return fmt.Errorf("%s: %s is entered twice for %s", path, e.Label, e.Lang)
		}
		seen[key] = true
		for i, x := range e.Errata {
			if x.Says == "" || x.Read == "" {
				return fmt.Errorf("%s: %s erratum %d has to say what the page says and what to read instead",
					path, e.Label, i+1)
			}
			if x.Why == "" {
				return fmt.Errorf("%s: %s erratum %d has no reason on it", path, e.Label, i+1)
			}
		}
	}
	return nil
}

// Lookup is the errata of one printing, by label. The slices are the
// manifest's own and are not to be appended to.
func (m *ErrataManifest) Lookup(lang string) map[string][]Erratum {
	out := map[string][]Erratum{}
	for _, e := range m.Entries {
		if e.Lang == lang {
			out[e.Label] = e.Errata
		}
	}
	return out
}

// Bytes renders the manifest, sorted, so that two people adding an entry on the
// same day do not conflict over where it went.
func (m *ErrataManifest) Bytes() ([]byte, error) {
	sorted := make([]LabelErrata, len(m.Entries))
	copy(sorted, m.Entries)
	sort.Slice(sorted, func(i, j int) bool {
		if sorted[i].Label != sorted[j].Label {
			return sorted[i].Label < sorted[j].Label
		}
		return sorted[i].Lang < sorted[j].Lang
	})
	enc, err := yaml.Marshal(&ErrataManifest{Entries: sorted})
	if err != nil {
		return nil, err
	}
	head := "# Where a printing is wrong. Written by hand, read by bourbaki assemble.\n" +
		"# The printed words stay as they are in content/ and the correction lives here.\n"
	return append([]byte(head), enc...), nil
}

// ErrataPath is where the manifest lives inside a corpus checkout.
func ErrataPath(root string) string {
	return filepath.Join(root, "manifests", "errata.yaml")
}
