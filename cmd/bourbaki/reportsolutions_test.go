package main

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/solve"
)

func sol(label, status string, parts ...corpus.Part) solve.Solution {
	return solve.Solution{Meta: corpus.SolutionFrontMatter{
		Label: label, Lang: "en", Status: status, Parts: parts, Model: "gpt-5"}}
}

// The exercises with no file are the whole point of the count. A scorecard over
// what was run says a hundred per cent verified on the first exercise anybody
// solves.
func TestAnExerciseWithNoFileIsUnattempted(t *testing.T) {
	exercises := []string{
		"alg-viii-s1-ex-1", "alg-viii-s1-ex-2", "alg-viii-s1-ex-3", "alg-viii-s2-ex-1",
	}
	held := map[string]solve.Solution{
		"alg-viii-s1-ex-1": sol("alg-viii-s1-ex-1", corpus.StatusVerified),
	}
	card := score(exercises, held, "en")
	if card.Exercises != 4 {
		t.Errorf("counted %d exercises", card.Exercises)
	}
	if card.Status[corpus.StatusUnattempted] != 3 {
		t.Errorf("unattempted %d, want 3", card.Status[corpus.StatusUnattempted])
	}
	if card.Believed != 1 || card.Answered != 1 {
		t.Errorf("believed %d answered %d, want 1 and 1", card.Believed, card.Answered)
	}
}

// Blocked and open are answers. Somebody asked, and the corpus now says why
// there is no proof under the exercise, which is worth more than a blank.
func TestBlockedAndOpenAreAnsweredAndNotBelieved(t *testing.T) {
	exercises := []string{"alg-viii-s1-ex-1", "alg-viii-s1-ex-2"}
	held := map[string]solve.Solution{
		"alg-viii-s1-ex-1": sol("alg-viii-s1-ex-1", corpus.StatusBlocked),
		"alg-viii-s1-ex-2": sol("alg-viii-s1-ex-2", corpus.StatusOpen),
	}
	card := score(exercises, held, "en")
	if card.Answered != 2 {
		t.Errorf("answered %d, want 2", card.Answered)
	}
	if card.Believed != 0 {
		t.Errorf("believed %d, want 0", card.Believed)
	}
}

func TestThePartsAreCountedAcrossTheWholePrinting(t *testing.T) {
	exercises := []string{"alg-viii-s1-ex-1", "alg-viii-s21-ex-12"}
	held := map[string]solve.Solution{
		"alg-viii-s1-ex-1": sol("alg-viii-s1-ex-1", corpus.StatusPartial,
			corpus.Part{ID: "a", Status: corpus.StatusVerified},
			corpus.Part{ID: "b", Status: corpus.StatusUnverified, Reason: "the base case is missing"}),
		"alg-viii-s21-ex-12": sol("alg-viii-s21-ex-12", corpus.StatusVerified,
			corpus.Part{ID: "a", Status: corpus.StatusVerified},
			corpus.Part{ID: "b", Status: corpus.StatusVerified}),
	}
	card := score(exercises, held, "en")
	if card.Parts.Total != 4 || card.Parts.Verified != 3 {
		t.Errorf("parts %d of which %d verified, want 4 and 3",
			card.Parts.Total, card.Parts.Verified)
	}
	// The two exercises are in different sections and the table has to say so,
	// because a § where the pipeline is doing badly is the thing this is read to
	// find.
	if len(card.Sections) != 2 {
		t.Fatalf("%d sections in the table", len(card.Sections))
	}
	if card.Sections[0].Section != 1 || card.Sections[1].Section != 21 {
		t.Errorf("the sections came out as %d and %d",
			card.Sections[0].Section, card.Sections[1].Section)
	}
}
