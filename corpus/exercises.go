package corpus

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// ExercisesManifest is manifests/exercises.json, the inventory of every
// exercise the corpus holds.
//
// It is JSON and not YAML because it is the one manifest meant to be read by
// something other than a person: the solver picks its work off it, and a solver
// run that has to parse twenty-five directories to find out what there is to
// solve is a solver run that gets it wrong the first time the directories move.
//
// Gaps are what it is really for. Bourbaki numbers the exercises of a § from
// one straight through, so a § that reports 1 to 12 and then 14 has lost a page
// or lost a split, and the number nobody can spot by eye across 317 exercises
// is exactly the one this records.
type ExercisesManifest struct {
	Books []BookExercises `json:"books"`
}

// BookExercises is one volume.
type BookExercises struct {
	ID       string             `json:"id"`
	Chapters []ChapterExercises `json:"chapters"`
}

// ChapterExercises is one chapter.
type ChapterExercises struct {
	Chapter string            `json:"chapter"`
	Title   string            `json:"title,omitempty"`
	Total   int               `json:"total"`
	Section []SectionExercise `json:"sections"`
}

// SectionExercise is the exercises of one § or appendix.
type SectionExercise struct {
	Section  int    `json:"section"`
	Appendix bool   `json:"appendix,omitempty"`
	Label    string `json:"label"`
	Dir      string `json:"dir"`
	Count    int    `json:"count"`
	// First and Last are the numbers the book prints at either end. Count is
	// not Last minus First plus one when something is missing, which is the
	// whole point of recording all three.
	First int `json:"first"`
	Last  int `json:"last"`
	// Gaps are the numbers between First and Last that no file carries. An
	// audit failure, never a fact about the book.
	Gaps []int `json:"gaps,omitempty"`
	// Starred and Supplementary are the two marks Bourbaki puts on an
	// exercise: an asterisk for one that draws on what the reader has not
	// reached, a pilcrow for one of the harder ones.
	Starred       int `json:"starred,omitempty"`
	Supplementary int `json:"supplementary,omitempty"`
}

// Gaps works out which numbers are missing from a § whose exercises are these.
func Gaps(numbers []int) []int {
	if len(numbers) == 0 {
		return nil
	}
	have := make(map[int]bool, len(numbers))
	lo, hi := numbers[0], numbers[0]
	for _, n := range numbers {
		have[n] = true
		lo, hi = min(lo, n), max(hi, n)
	}
	var out []int
	for n := lo; n <= hi; n++ {
		if !have[n] {
			out = append(out, n)
		}
	}
	return out
}

// LoadExercises reads manifests/exercises.json. A missing file is an empty
// manifest, so the first assemble works on a fresh repo.
func LoadExercises(root string) (*ExercisesManifest, error) {
	path := ExercisesPath(root)
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &ExercisesManifest{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m ExercisesManifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &m, nil
}

// Upsert replaces a volume's record, or appends it, leaving the order of the
// other volumes alone.
func (m *ExercisesManifest) Upsert(b BookExercises) {
	for i := range m.Books {
		if m.Books[i].ID == b.ID {
			m.Books[i] = b
			return
		}
	}
	m.Books = append(m.Books, b)
}

// Bytes renders the manifest, indented and with a newline at the end, so that a
// diff of two runs reads as a diff of the corpus and not of one long line.
func (m *ExercisesManifest) Bytes() ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	if err := enc.Encode(m); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

// Save writes the manifest back.
func (m *ExercisesManifest) Save(root string) error {
	b, err := m.Bytes()
	if err != nil {
		return err
	}
	path := ExercisesPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// ExercisesPath is where the manifest lives inside a corpus checkout.
func ExercisesPath(root string) string {
	return filepath.Join(root, "manifests", "exercises.json")
}
