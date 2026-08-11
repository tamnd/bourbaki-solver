// Package glossary is the terminology contract between the four languages.
//
// It is what makes 1200 pages read as one translation instead of 120 unrelated
// ones. Bourbaki's vocabulary is fixed and deliberate, and a translation that
// renders "Artinian ring" three different ways across one chapter is a
// translation nobody can search and nobody can trust, however good each of the
// three renderings is on its own.
//
// The file is manifests/glossary.yaml in the corpus, and it is committed, read
// by the audit and embedded in every translation prompt. The version number on
// it is what makes a translation stale: bump it and every file that recorded an
// earlier version is due to be done again.
package glossary

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// Glossary is manifests/glossary.yaml.
type Glossary struct {
	Version int    `yaml:"version"`
	Terms   []Term `yaml:"terms"`
}

// A Term is one English term and what it is called in each language.
//
// A language may be empty, and that is not an error: a term is curated one
// language at a time and the Vietnamese pass runs first, so a glossary with 800
// Vietnamese rows and no Chinese ones is what the middle of M7 looks like.
type Term struct {
	EN string `yaml:"en"`
	VI string `yaml:"vi,omitempty"`
	ZH string `yaml:"zh,omitempty"`
	JA string `yaml:"ja,omitempty"`

	// Note is for the term whose translation depends on something a translator
	// cannot see in the phrase, which in these volumes is usually a Bourbaki
	// convention: a ring is associative with a unit unless it says otherwise,
	// and a translator who does not know that will reach for the wrong word.
	Note string `yaml:"note,omitempty"`
}

// In is the term's rendering in one language, empty when it has none.
func (t Term) In(lang string) string {
	switch lang {
	case "vi":
		return t.VI
	case "zh":
		return t.ZH
	case "ja":
		return t.JA
	case "en":
		return t.EN
	}
	return ""
}

// Set writes the term's rendering in one language. A language this glossary
// does not carry is ignored rather than added, because the audit and the
// translation prompts both read the three fields by name and a fourth would go
// nowhere.
func (t *Term) Set(lang, value string) {
	switch lang {
	case "vi":
		t.VI = value
	case "zh":
		t.ZH = value
	case "ja":
		t.JA = value
	}
}

// Langs are the languages a glossary row carries, in the order M7 does them.
var Langs = []string{"vi", "zh", "ja"}

// Path is where the glossary lives.
func Path(root string) string { return filepath.Join(root, "manifests", "glossary.yaml") }

// CandidatesPath is where the mined candidates live. They are a working
// document rather than a contract, which is why they are a separate file: the
// glossary is what has been decided and this is what has been noticed.
func CandidatesPath(root string) string {
	return filepath.Join(root, "manifests", "glossary-candidates.yaml")
}

// Load reads the glossary. A corpus with no glossary yet is not an error, it is
// an empty glossary, because everything that reads one has to work before M7
// has written it.
func Load(root string) (*Glossary, error) {
	b, err := os.ReadFile(Path(root))
	if os.IsNotExist(err) {
		return &Glossary{}, nil
	}
	if err != nil {
		return nil, err
	}
	return Parse(b, Path(root))
}

// Parse reads a glossary from bytes. name is what an error is blamed on, since
// the bytes do not always come from a file: the audit reads an older revision
// of the glossary out of git to see whether the version moved when the terms
// did.
func Parse(b []byte, name string) (*Glossary, error) {
	var g Glossary
	dec := yaml.NewDecoder(strings.NewReader(string(b)))
	dec.KnownFields(true)
	if err := dec.Decode(&g); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	if err := g.Validate(); err != nil {
		return nil, fmt.Errorf("%s: %w", name, err)
	}
	return &g, nil
}

// Validate catches the two ways a hand-edited glossary is wrong in a way that
// would be silent: a duplicated English term, where the second row is dead and
// nobody can see which one is being used, and an empty one.
func (g Glossary) Validate() error {
	seen := map[string]bool{}
	for i, t := range g.Terms {
		key := Key(t.EN)
		if key == "" {
			return fmt.Errorf("term %d has no en", i+1)
		}
		if seen[key] {
			return fmt.Errorf("term %q appears twice", t.EN)
		}
		seen[key] = true
	}
	return nil
}

// Key is the form a term is compared in: lower case, single spaced. Bourbaki
// writes "Artinian ring" at the head of a sentence and "artinian ring" inside
// one, and those are one term.
func Key(term string) string { return strings.Join(strings.Fields(strings.ToLower(term)), " ") }

// In is every term that has a rendering in this language, keyed by Key(en).
func (g Glossary) In(lang string) map[string]Term {
	out := map[string]Term{}
	for _, t := range g.Terms {
		if t.In(lang) != "" {
			out[Key(t.EN)] = t
		}
	}
	return out
}

// Sorted is the terms longest first, then alphabetically.
//
// Longest first is what a scanner needs. "semisimple ring" and "ring" are both
// terms, and a scan that met "ring" first would find it inside the longer one
// and report the longer term as absent from a translation that renders it
// perfectly well.
func (g Glossary) Sorted() []Term {
	out := append([]Term(nil), g.Terms...)
	sort.SliceStable(out, func(i, j int) bool {
		a, b := Key(out[i].EN), Key(out[j].EN)
		if len(a) != len(b) {
			return len(a) > len(b)
		}
		return a < b
	})
	return out
}

// Save writes the glossary and moves the version on if the terms changed.
//
// The version is what makes a translated file stale, and until now it was a
// number somebody had to remember to raise. It was raised by hand twice: once
// when the Vietnamese renderings landed and once when Nullstellensatz was pinned
// after a heading came back half translated. A number that has to be remembered
// is a number that will be forgotten, and the cost of forgetting it is a corpus
// where some files were translated against one glossary and some against
// another with nothing recording which.
//
// The comparison is on the renderings and not on the bytes, so re-ordering the
// file does not move it and neither does editing a note, which is written for a
// person and never reaches a model. A row added, removed or rendered differently
// does move it, because those are the three things that change what a prompt
// says.
//
// It returns the version as written and whether it moved, so a command can say
// so. A glossary that is not there yet starts at 1.
func (g *Glossary) Save(path string) (version int, bumped bool, err error) {
	old, err := os.ReadFile(path)
	switch {
	case os.IsNotExist(err):
		if g.Version == 0 {
			g.Version = 1
		}
	case err != nil:
		return 0, false, err
	default:
		var was Glossary
		if err := yaml.Unmarshal(old, &was); err != nil {
			return 0, false, fmt.Errorf("%s: %w", path, err)
		}
		if !SameTerms(was.Terms, g.Terms) {
			g.Version = max(was.Version, g.Version) + 1
			bumped = true
		}
	}
	if err := g.Write(path); err != nil {
		return 0, false, err
	}
	return g.Version, bumped, nil
}

// SameTerms says whether two glossaries hold the same renderings.
//
// Compared by key with the note cleared, so that reordering the file and
// writing down why a row is there do not count as a change. Only what a
// translation is actually held to counts, which is the renderings.
//
// Exported because Save is not the only way the file changes. Somebody editing
// the YAML by hand goes round Save entirely, and L09 is what catches that, by
// asking this the same question about the committed revision.
func SameTerms(a, b []Term) bool {
	if len(a) != len(b) {
		return false
	}
	was := make(map[string]Term, len(a))
	for _, t := range a {
		t.Note = ""
		was[Key(t.EN)] = t
	}
	for _, t := range b {
		t.Note = ""
		if other, ok := was[Key(t.EN)]; !ok || other != t {
			return false
		}
	}
	return true
}

// Write renders the glossary to a path, creating the directory.
func (g Glossary) Write(path string) error {
	b, err := yaml.Marshal(g)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}
