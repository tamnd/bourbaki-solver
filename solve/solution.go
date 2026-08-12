package solve

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/tags"
)

// Solution is one answer as it sits on disk: the front matter, and the
// mathematics under it.
type Solution struct {
	Meta corpus.SolutionFrontMatter
	Body string
	// Path is where it was read from or where it will be written, relative to
	// the corpus root. It is not part of the file.
	Path string
}

// Store is the solutions of the corpus, under content/solutions/<lang>/.
//
// Solutions are filed by the exercise's permanent label rather than by its
// number, so a solution follows its exercise through a renumbering, and one
// file to an exercise rather than a log of runs, because the corpus is what is
// believed now and the run that produced it is reports/ business.
type Store struct{ Root string }

// Path is where the solution to one exercise belongs.
func (s Store) Path(lang, label string) (string, error) {
	return corpus.SolutionPath(s.Root, lang, label)
}

// Load reads one solution. The second return is false when the corpus does not
// hold one, which is the ordinary case and not an error: 317 exercises of
// chapter VIII have no solution and will have none until they are run.
func (s Store) Load(lang, label string) (Solution, bool, error) {
	path, err := s.Path(lang, label)
	if err != nil {
		return Solution{}, false, err
	}
	f, err := corpus.ReadFile[corpus.SolutionFrontMatter](path)
	if os.IsNotExist(err) {
		return Solution{}, false, nil
	}
	if err != nil {
		return Solution{}, false, err
	}
	return Solution{Meta: f.Meta, Body: f.Body, Path: s.rel(path)}, true, nil
}

// Save writes one solution, after checking it is one.
//
// The check is here and not only in the audit because the audit runs over what
// was committed. A run that writes eight hundred files and then finds out from
// the audit that every one of them names a status nothing reads has wasted the
// fleet, and the fleet is the scarce thing.
func (s Store) Save(sol Solution) error {
	if err := Validate(sol); err != nil {
		return err
	}
	path, err := s.Path(sol.Meta.Lang, sol.Meta.Label)
	if err != nil {
		return err
	}
	f := corpus.File[corpus.SolutionFrontMatter]{Meta: sol.Meta, Body: sol.Body}
	return f.Write(path)
}

// List reads every solution of one language, in label order.
func (s Store) List(lang string) ([]Solution, error) {
	dir := filepath.Join(s.Root, "content", "solutions", lang)
	var out []Solution
	err := filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == dir {
				return nil // nothing solved in this language yet
			}
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		f, err := corpus.ReadFile[corpus.SolutionFrontMatter](path)
		if err != nil {
			return err
		}
		out = append(out, Solution{Meta: f.Meta, Body: f.Body, Path: s.rel(path)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Meta.Label < out[j].Meta.Label })
	return out, nil
}

// Counts is how many solutions of one language stand at each status, keyed by
// status. It is what a scorecard is built out of and what --status selects on.
func (s Store) Counts(lang string) (map[string]int, error) {
	list, err := s.List(lang)
	if err != nil {
		return nil, err
	}
	out := map[string]int{}
	for _, sol := range list {
		out[sol.Meta.Status]++
	}
	return out, nil
}

func (s Store) rel(path string) string {
	r, err := filepath.Rel(s.Root, path)
	if err != nil {
		return filepath.ToSlash(path)
	}
	return filepath.ToSlash(r)
}

// Validate says whether a solution is one, and it is stricter than the audit
// rules of the same name on purpose.
//
// The audit reports; this refuses. What it refuses is a file that would be a
// lie on disk: a status that is not a status, a verified that no judge passed,
// a partial that says nothing about its parts. A wrong solution is allowed
// through, because a wrong solution honestly marked is the thing this whole
// milestone is built to produce, and there is no test here that a proof is a
// proof.
func Validate(sol Solution) error {
	m := sol.Meta
	if m.Label == "" {
		return fmt.Errorf("a solution with no label belongs to no exercise")
	}
	r, err := corpus.ParseLabel(m.Label)
	if err != nil {
		return fmt.Errorf("label %q: %w", m.Label, err)
	}
	if r.Kind != corpus.KindExercise {
		return fmt.Errorf("label %q names a %s, and a solution answers an exercise",
			m.Label, r.Kind)
	}
	if m.Lang == "" {
		return fmt.Errorf("%s: a solution with no language cannot be filed", m.Label)
	}
	if !corpus.ValidStatus(m.Status) {
		return fmt.Errorf("%s: status %q is not one of %s", m.Label, m.Status,
			strings.Join(corpus.Statuses, ", "))
	}
	if m.Tag != "" {
		if _, err := tags.Parse(m.Tag); err != nil {
			return fmt.Errorf("%s: %w", m.Label, err)
		}
	}
	for _, use := range m.Uses {
		if _, err := tags.Parse(use); err != nil {
			return fmt.Errorf("%s: uses a tag that is not one: %w", m.Label, err)
		}
	}
	for i, p := range m.Parts {
		if p.ID == "" {
			return fmt.Errorf("%s: part %d has no id", m.Label, i+1)
		}
		if !corpus.ValidStatus(p.Status) {
			return fmt.Errorf("%s: part %s has status %q, which is not one of %s",
				m.Label, p.ID, p.Status, strings.Join(corpus.Statuses, ", "))
		}
	}
	return validateStatus(m, sol.Body)
}

// validateStatus is what each status commits the file to.
func validateStatus(m corpus.SolutionFrontMatter, body string) error {
	empty := strings.TrimSpace(body) == ""
	if m.Status == corpus.StatusUnattempted {
		if !empty {
			return fmt.Errorf("%s: unattempted, and there is a solution under it", m.Label)
		}
		return nil
	}
	if empty {
		return fmt.Errorf("%s: %s, and there is nothing under it", m.Label, m.Status)
	}
	switch m.Status {
	case corpus.StatusVerified:
		if m.TruthJudge != "pass" || m.AuditJudge != "pass" {
			return fmt.Errorf("%s: verified, and the judges are %q and %q",
				m.Label, m.TruthJudge, m.AuditJudge)
		}
		for _, p := range m.Parts {
			if p.Status != corpus.StatusVerified {
				return fmt.Errorf("%s: verified, and part %s is %s", m.Label, p.ID, p.Status)
			}
		}
	case corpus.StatusPartial:
		good, bad := 0, 0
		for _, p := range m.Parts {
			if p.Status == corpus.StatusVerified {
				good++
			} else {
				bad++
			}
		}
		if good == 0 || bad == 0 {
			return fmt.Errorf("%s: partial, with %d parts passing and %d not", m.Label, good, bad)
		}
	}
	return nil
}
