package solve

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

func good() Solution {
	return Solution{Meta: corpus.SolutionFrontMatter{
		Label: "alg-viii-s1-ex-3", Tag: "0005", Lang: "en",
		Status: corpus.StatusVerified, TruthJudge: "pass", AuditJudge: "pass",
		Uses: []string{"0001"}}, Body: "Every submodule is finitely generated."}
}

func TestASolutionGoesToDiskAndComesBack(t *testing.T) {
	s := Store{Root: t.TempDir()}
	if err := s.Save(good()); err != nil {
		t.Fatal(err)
	}
	back, ok, err := s.Load("en", "alg-viii-s1-ex-3")
	if err != nil || !ok {
		t.Fatalf("loaded %v %v", ok, err)
	}
	if !reflect.DeepEqual(back.Meta, good().Meta) {
		t.Errorf("the front matter came back as %+v", back.Meta)
	}
	if strings.TrimSpace(back.Body) != good().Body {
		t.Errorf("the body came back as %q", back.Body)
	}
	if back.Path != "content/solutions/en/alg/VIII/s1/03.md" {
		t.Errorf("it was filed at %s", back.Path)
	}
}

// An exercise with no solution is the ordinary case, not an error. 317
// exercises of chapter VIII are in it.
func TestAnExerciseWithNoSolution(t *testing.T) {
	s := Store{Root: t.TempDir()}
	_, ok, err := s.Load("en", "alg-viii-s1-ex-3")
	if err != nil {
		t.Fatal(err)
	}
	if ok {
		t.Error("an empty corpus answered with a solution")
	}
}

// Appendix 1 of chapter VIII and its § 1 both number their exercises from one,
// so a store that files both under s1 holds one file for two exercises and can
// say nothing about which one it answers.
func TestAnAppendixExerciseDoesNotOverwriteTheSection(t *testing.T) {
	s := Store{Root: t.TempDir()}
	section := good()
	appendix := good()
	appendix.Meta.Label = "alg-viii-a1-ex-3"
	appendix.Meta.Tag = "0006"
	appendix.Body = "The trace of a projector is its rank."
	for _, sol := range []Solution{section, appendix} {
		if err := s.Save(sol); err != nil {
			t.Fatal(err)
		}
	}
	list, err := s.List("en")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 2 {
		t.Fatalf("the store holds %d solutions, want 2: %+v", len(list), list)
	}
	if list[0].Path == list[1].Path {
		t.Fatalf("both were filed at %s", list[0].Path)
	}
}

func TestListAndCounts(t *testing.T) {
	s := Store{Root: t.TempDir()}
	for i, status := range []string{corpus.StatusVerified, corpus.StatusVerified,
		corpus.StatusBlocked} {
		sol := good()
		sol.Meta.Label = "alg-viii-s1-ex-" + string(rune('1'+i))
		sol.Meta.Status = status
		if status != corpus.StatusVerified {
			sol.Meta.TruthJudge, sol.Meta.AuditJudge = "", ""
		}
		if err := s.Save(sol); err != nil {
			t.Fatal(err)
		}
	}
	counts, err := s.Counts("en")
	if err != nil {
		t.Fatal(err)
	}
	if counts[corpus.StatusVerified] != 2 || counts[corpus.StatusBlocked] != 1 {
		t.Errorf("the counts came out as %v", counts)
	}
	list, err := s.List("en")
	if err != nil {
		t.Fatal(err)
	}
	for i := 1; i < len(list); i++ {
		if list[i-1].Meta.Label > list[i].Meta.Label {
			t.Fatalf("the list is not in label order: %s before %s",
				list[i-1].Meta.Label, list[i].Meta.Label)
		}
	}
}

// A language nothing has been solved in yet is an empty answer and not a
// missing directory error. The Vietnamese will be in this state for a while.
func TestALanguageWithNothingSolvedInIt(t *testing.T) {
	s := Store{Root: t.TempDir()}
	list, err := s.List("vi")
	if err != nil {
		t.Fatal(err)
	}
	if len(list) != 0 {
		t.Errorf("an unsolved language answered with %d solutions", len(list))
	}
}

// What Validate refuses is a file that would be a lie on disk. It is not a
// check that the mathematics is right, and there is no such check anywhere in
// this package.
func TestWhatWillNotBeWritten(t *testing.T) {
	cases := []struct {
		name string
		edit func(*Solution)
		want string
	}{
		{"no label", func(s *Solution) { s.Meta.Label = "" }, "belongs to no exercise"},
		{"a label that names a proposition", func(s *Solution) {
			s.Meta.Label = "alg-viii-s1-prop-1"
		}, "answers an exercise"},
		{"no language", func(s *Solution) { s.Meta.Lang = "" }, "cannot be filed"},
		{"a status nothing reads", func(s *Solution) { s.Meta.Status = "done" }, "is not one of"},
		{"verified with a judge that did not pass", func(s *Solution) {
			s.Meta.AuditJudge = "fail"
		}, "the judges are"},
		{"verified with a part that did not", func(s *Solution) {
			s.Meta.Parts = []corpus.Part{{ID: "a", Status: corpus.StatusUnverified}}
		}, "part a is unverified"},
		{"partial with nothing that failed", func(s *Solution) {
			s.Meta.Status = corpus.StatusPartial
			s.Meta.Parts = []corpus.Part{{ID: "a", Status: corpus.StatusVerified}}
		}, "1 parts passing and 0 not"},
		{"a tag that is not one", func(s *Solution) { s.Meta.Tag = "5" }, "characters"},
		{"a use that is not a tag", func(s *Solution) {
			s.Meta.Uses = []string{"prop-1"}
		}, "not one"},
		{"a part with no id", func(s *Solution) {
			s.Meta.Parts = []corpus.Part{{Status: corpus.StatusVerified}}
		}, "no id"},
		{"nothing under a verdict", func(s *Solution) { s.Body = "  \n" }, "nothing under it"},
		{"a proof under unattempted", func(s *Solution) {
			s.Meta.Status = corpus.StatusUnattempted
		}, "there is a solution under it"},
	}
	s := Store{Root: t.TempDir()}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			sol := good()
			c.edit(&sol)
			err := Validate(sol)
			if err == nil {
				t.Fatal("it was accepted")
			}
			if !strings.Contains(err.Error(), c.want) {
				t.Errorf("the refusal is %q, want it to say %q", err, c.want)
			}
			if err := s.Save(sol); err == nil {
				t.Error("Save wrote it anyway")
			}
		})
	}
	// And nothing was written by any of them.
	if _, err := os.Stat(filepath.Join(s.Root, "content")); !os.IsNotExist(err) {
		t.Errorf("a refused solution left something on disk: %v", err)
	}
}
