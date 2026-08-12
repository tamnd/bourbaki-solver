// Package benchmark is the fixed set the verifier is measured against.
//
// Spec 07 §6. The pipeline decides for itself whether a solution is right, and
// a pipeline that grades its own work is worth exactly as much as the grading
// is. The only way to know what the grading is worth is to put answers in front
// of it whose worth a person has already settled, some of them right and some
// of them wrong on purpose, and count how often the judges agree.
//
// The two errors are not the same error. A wrong answer the judges accept goes
// into a book as verified and is read by somebody who trusts it, and that is the
// number spec 07 §6 sets a target for: under 5 per cent. A right answer the
// judges reject costs a rerun, and up to 30 per cent of that is tolerated,
// because a cautious verifier is the right failure direction for a corpus that
// nobody is going to read line by line.
//
// What lives here is labels, verdicts and one line of why, and no Bourbaki. The
// answers themselves are in the corpus, which is where text belongs, and this
// repository is public.
package benchmark

import (
	_ "embed"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

//go:embed set.json
var setJSON []byte

// The two verdicts a person can record. There is no third: a case nobody could
// decide about is not a case, it is a question, and it does not belong in a set
// that other numbers are measured against.
const (
	Accept = "accept" // a person read it and it is right
	Reject = "reject" // a person read it and it is wrong
)

// Case is one answer put to the judges, with what a person who read it says.
type Case struct {
	// Label is the exercise, by permanent label.
	Label string `json:"label"`
	// Variant tells two answers to the same exercise apart. "as-solved" is what
	// the pipeline wrote and a person then read; the rest are hand written, and
	// the ones that are wrong say what is wrong with them in their name.
	Variant string `json:"variant"`
	// Expect is Accept or Reject, and it is a person's reading and not a
	// judge's.
	Expect string `json:"expect"`
	// Why is what the person saw, in one line, in their own words. It says
	// nothing a reader of this repository could reassemble the exercise from.
	Why string `json:"why"`
	// Read is who settled it and when, because a verdict with nobody behind it
	// is a guess that has been written down.
	Read string `json:"read"`
}

// Name is how one case is written in a report.
func (c Case) Name() string { return c.Label + " " + c.Variant }

// Body is where the answer itself lives, which is the corpus and not here.
//
// One directory to the language, one file to the case. It is beside content/
// rather than in it because content/ is Bourbaki and the audit walks it, and an
// answer that is wrong on purpose has no business being read as part of the
// book.
func (c Case) Body(root, lang string) string {
	return filepath.Join(root, "benchmark", lang, c.Label+"."+c.Variant+".md")
}

// Set is the whole benchmark, in the order it is run and reported.
type Set []Case

// Load reads the set that ships with this binary.
func Load() (Set, error) { return parse(setJSON, "set.json") }

// LoadFile reads a set from disk, which is how a set is tried before it is
// shipped.
func LoadFile(path string) (Set, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return parse(raw, path)
}

func parse(raw []byte, from string) (Set, error) {
	var out Set
	if err := json.Unmarshal(raw, &out); err != nil {
		return nil, fmt.Errorf("%s: %w", from, err)
	}
	if err := out.Check(); err != nil {
		return nil, fmt.Errorf("%s: %w", from, err)
	}
	return out, nil
}

// Check says whether a set is one, and it is strict about it. A benchmark with a
// typo in a verdict does not fail, it quietly measures something else.
func (s Set) Check() error {
	seen := map[string]bool{}
	for i, c := range s {
		switch {
		case c.Label == "":
			return fmt.Errorf("case %d has no label", i+1)
		case c.Variant == "":
			return fmt.Errorf("%s has no variant", c.Label)
		case c.Expect != Accept && c.Expect != Reject:
			return fmt.Errorf("%s: %q is not %s or %s", c.Name(), c.Expect, Accept, Reject)
		case strings.TrimSpace(c.Why) == "":
			return fmt.Errorf("%s: a verdict with no reading behind it", c.Name())
		case strings.TrimSpace(c.Read) == "":
			return fmt.Errorf("%s: nobody is recorded as having read it", c.Name())
		case seen[c.Name()]:
			return fmt.Errorf("%s is in the set twice", c.Name())
		}
		seen[c.Name()] = true
	}
	return nil
}

// Counts is how many cases fall each way.
func (s Set) Counts() (accept, reject int) {
	for _, c := range s {
		if c.Expect == Accept {
			accept++
		} else {
			reject++
		}
	}
	return accept, reject
}

// Labels is the exercises the set covers, each once and in order.
func (s Set) Labels() []string {
	var out []string
	for _, c := range s {
		if !slices.Contains(out, c.Label) {
			out = append(out, c.Label)
		}
	}
	return out
}
