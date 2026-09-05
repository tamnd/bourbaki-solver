package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/benchmark"
)

func benchSet() benchmark.Set {
	return benchmark.Set{
		{Label: "alg-viii-s1-ex-1", Variant: "as-solved", Expect: benchmark.Accept,
			Why: "reads correctly", Read: "tam, 2026-08-13"},
		{Label: "alg-viii-s1-ex-1", Variant: "flawed-converse", Expect: benchmark.Reject,
			Why: "the converse is asserted", Read: "tam, 2026-08-13"},
		{Label: "alg-viii-s1-ex-2", Variant: "as-solved", Expect: benchmark.Accept,
			Why: "reads correctly", Read: "tam, 2026-08-13"},
	}
}

func TestEvalRunsOnlyWhatWasAskedFor(t *testing.T) {
	set := benchSet()
	if got := evalPlan(set, solveEvalFlags{label: "alg-viii-s1-ex-1"}); len(got) != 2 {
		t.Errorf("-label took %d cases of the exercise, want 2", len(got))
	}
	if got := evalPlan(set, solveEvalFlags{limit: 1}); len(got) != 1 {
		t.Errorf("-limit took %d cases, want 1", len(got))
	}
	if got := evalPlan(set, solveEvalFlags{}); len(got) != len(set) {
		t.Errorf("with no flags %d cases of %d were taken", len(got), len(set))
	}
}

// An hour of somebody's account is not the place to find out that the set names
// a file nobody wrote.
func TestAnAnswerTheCorpusDoesNotHoldIsFoundBeforeAnyHostIsAsked(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "benchmark", "en")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("alg-viii-s1-ex-1.as-solved.md", "a solution")
	// There on disk and empty, which measures exactly as little as one that is
	// not there at all.
	write("alg-viii-s1-ex-1.flawed-converse.md", "   \n")

	bodies, missing, err := evalBodies(benchSet(), root, "en")
	if err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 1 || bodies["alg-viii-s1-ex-1 as-solved"] != "a solution" {
		t.Errorf("read %d answers, want the one that was written", len(bodies))
	}
	if len(missing) != 2 {
		t.Fatalf("%d answers reported missing, want 2: the empty one and the absent one", len(missing))
	}
	for _, m := range missing {
		if !strings.HasPrefix(m, root) {
			t.Errorf("%s is not in the corpus that was searched", m)
		}
	}
}

func TestARateIsReportedAgainstWhatItIsHeldTo(t *testing.T) {
	over := evalLine("false accept", 1, 8, 0.125, benchmark.FalseAcceptTarget, "wrong answers through")
	if !strings.Contains(over, "12.5 %") || !strings.Contains(over, "over the 5 %") {
		t.Errorf("a rate over the target read %q", over)
	}
	inside := evalLine("false accept", 0, 8, 0, benchmark.FalseAcceptTarget, "wrong answers through")
	if !strings.Contains(inside, "inside the 5 %") {
		t.Errorf("a rate inside the target read %q", inside)
	}
	none := evalLine("false accept", 0, 0, -1, benchmark.FalseAcceptTarget, "wrong answers through")
	if !strings.Contains(none, "not measured") {
		t.Errorf("a rate with nothing behind it read %q", none)
	}
}

// The wrong directory and a broken set both end the run before a host is asked,
// and they are not the same thing to go and fix, so they must not read the same.
func TestEveryAnswerMissingIsReportedAsTheWrongCorpusAndNotAsABrokenSet(t *testing.T) {
	const root = "/somewhere/that/is/not/the/corpus"
	err := evalMissing(3, "set.json", root, []string{
		filepath.Join(root, "benchmark", "en", "alg-viii-s1-ex-1.as-solved.md"),
		filepath.Join(root, "benchmark", "en", "alg-viii-s1-ex-1.flawed-converse.md"),
		filepath.Join(root, "benchmark", "en", "alg-viii-s1-ex-2.as-solved.md"),
	})
	if err == nil {
		t.Fatal("a set with nothing on disk ran on")
	}
	if !strings.Contains(err.Error(), root) {
		t.Errorf("the root that was searched is not in %q, and it is the thing that was wrong", err)
	}
	if !strings.Contains(err.Error(), "BOURBAKI_CORPUS") {
		t.Errorf("%q does not say how to point the run at a corpus", err)
	}
}

func TestSomeAnswersMissingStillNamesTheSetAndTheFirstFile(t *testing.T) {
	const root = "/a/corpus"
	first := filepath.Join(root, "benchmark", "en", "alg-viii-s1-ex-2.as-solved.md")
	err := evalMissing(3, "set.json", root, []string{first})
	if err == nil {
		t.Fatal("a set naming a file nobody wrote ran on")
	}
	for _, want := range []string{"set.json", first, root} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q does not name %s", err, want)
		}
	}
}

func TestNothingMissingIsNotAnError(t *testing.T) {
	if err := evalMissing(3, "set.json", "/a/corpus", nil); err != nil {
		t.Errorf("a set that is entirely on disk was refused: %v", err)
	}
}
