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

// read returns a solution somebody has read, with the findings they wrote down.
func read(label, status string, found ...string) solve.Solution {
	s := sol(label, status)
	s.Meta.HandRead = "2026-08-19"
	s.Meta.Found = found
	return s
}

// A verified solution a reader objected to is the one thing on this card that
// names a file the corpus is getting wrong, and it used to be counted only
// beside hand read, where it read as a note on the reading and not on the
// status. Believed went on counting it either way.
func TestAVerifiedSolutionAReaderObjectedToIsCountedAgainstBelieved(t *testing.T) {
	exercises := []string{"alg-viii-s1-ex-1", "alg-viii-s1-ex-2", "alg-viii-s1-ex-3"}
	held := map[string]solve.Solution{
		"alg-viii-s1-ex-1": read("alg-viii-s1-ex-1", corpus.StatusVerified,
			"the proof assumes the result of the preceding exercise"),
		"alg-viii-s1-ex-2": read("alg-viii-s1-ex-2", corpus.StatusVerified),
		"alg-viii-s1-ex-3": sol("alg-viii-s1-ex-3", corpus.StatusVerified),
	}
	card := score(exercises, held, "en")
	if card.Believed != 3 {
		t.Errorf("believed %d, want 3: a finding is a reader's note and does not move a status", card.Believed)
	}
	if card.Contested != 1 {
		t.Errorf("contested %d, want 1", card.Contested)
	}
	if card.HandRead != 2 || card.Disputed != 1 {
		t.Errorf("hand read %d disputed %d, want 2 and 1", card.HandRead, card.Disputed)
	}
}

// A reader who found something against an answer that was never claimed to be
// right has not contested anything. The count is of the gap between what the
// judges said and what somebody read, so an unverified solution with a finding
// is the two agreeing.
func TestAFindingAgainstAnUnverifiedSolutionIsNotContested(t *testing.T) {
	exercises := []string{"alg-viii-s1-ex-1", "alg-viii-s1-ex-2"}
	held := map[string]solve.Solution{
		"alg-viii-s1-ex-1": read("alg-viii-s1-ex-1", corpus.StatusUnverified, "the second inclusion is asserted"),
		"alg-viii-s1-ex-2": read("alg-viii-s1-ex-2", corpus.StatusPartial, "part (b) is hand waved"),
	}
	card := score(exercises, held, "en")
	if card.Disputed != 2 {
		t.Errorf("disputed %d, want 2", card.Disputed)
	}
	if card.Contested != 0 {
		t.Errorf("contested %d, want 0", card.Contested)
	}
}
