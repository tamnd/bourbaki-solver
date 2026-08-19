package main

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/solve"
)

func readSolutions() map[string]solve.Solution {
	return map[string]solve.Solution{
		"alg-viii-s1-ex-1": {Meta: corpus.SolutionFrontMatter{
			Label: "alg-viii-s1-ex-1", Lang: "en", Status: corpus.StatusVerified,
			TruthJudge: "pass", AuditJudge: "pass",
			HandRead: "2026-08-19"}},
		"alg-viii-s1-ex-2": {Meta: corpus.SolutionFrontMatter{
			Label: "alg-viii-s1-ex-2", Lang: "en", Status: corpus.StatusVerified,
			TruthJudge: "pass", AuditJudge: "pass",
			HandRead: "2026-08-19",
			Found:    []string{"the induction lets one letter stand for two things"}}},
		"alg-viii-s1-ex-3": {Meta: corpus.SolutionFrontMatter{
			Label: "alg-viii-s1-ex-3", Lang: "en", Status: corpus.StatusUnverified}},
	}
}

// The two numbers on the scorecard that somebody checked. A reading that
// disagrees with the status has to survive being counted, because the whole
// point of writing it in the front matter rather than in the status is that it
// stays visible.
func TestAReadingThatDisagreesIsCountedAndDoesNotMoveTheStatus(t *testing.T) {
	held := readSolutions()
	labels := []string{"alg-viii-s1-ex-1", "alg-viii-s1-ex-2", "alg-viii-s1-ex-3",
		"alg-viii-s1-ex-4"}
	card := score(labels, held, "en")

	if card.HandRead != 2 {
		t.Errorf("hand read %d of them, want 2", card.HandRead)
	}
	if card.Disputed != 1 {
		t.Errorf("%d readings found something, want 1", card.Disputed)
	}
	// The one with a finding under it is still verified, and still counted as
	// verified. The corpus says both things and resolves neither.
	if card.Believed != 2 {
		t.Errorf("believed %d, want 2: a finding must not take a status away",
			card.Believed)
	}
	if got := held["alg-viii-s1-ex-2"].Meta.Status; got != corpus.StatusVerified {
		t.Errorf("the disputed solution is now %s", got)
	}
	// The exercise with no file at all is unattempted and is not read.
	if card.Status[corpus.StatusUnattempted] != 1 {
		t.Errorf("unattempted %d, want 1", card.Status[corpus.StatusUnattempted])
	}
}

func TestAFindingWithNothingInItIsRefused(t *testing.T) {
	var found foundList
	if err := found.Set("  "); err == nil {
		t.Error("an empty finding was taken")
	}
	if err := found.Set(" the converse is asserted "); err != nil {
		t.Fatal(err)
	}
	if len(found) != 1 || found[0] != "the converse is asserted" {
		t.Errorf("found = %q, want the line trimmed", found)
	}
}
