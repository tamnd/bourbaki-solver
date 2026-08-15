package glossary

import (
	"strings"
	"testing"
)

func TestConsensusKeepsWhatAgrees(t *testing.T) {
	agreed, split := Consensus([][]Row{
		{{EN: "infinite set", TR: "无限集"}, {EN: "character", TR: "特征"}, {EN: "root system", TR: "根系"}},
		{{EN: "infinite set", TR: "无限集"}, {EN: "character", TR: "特征标"}, {EN: "root system", TR: "根系"}},
		{{EN: "infinite set", TR: "无限集合"}, {EN: "character", TR: "特征标"}, {EN: "root system", TR: "根系"}},
	})
	got := map[string]string{}
	for _, row := range agreed {
		got[row.EN] = row.TR
	}
	// Two of three is a majority, so both of these are kept, and the reading
	// only one round gave is the one that goes.
	if got["infinite set"] != "无限集" || got["character"] != "特征标" || got["root system"] != "根系" {
		t.Fatalf("agreed = %v", got)
	}
	if len(split) != 0 {
		t.Errorf("split = %v, and every term had a majority", split)
	}
}

// Two rounds that disagree is the case this is for. Neither answer is a
// majority of two, and picking either would be picking at random, so the term
// goes to a person with both readings in front of them.
func TestConsensusReportsATermTheRoundsSplitOn(t *testing.T) {
	agreed, split := Consensus([][]Row{
		{{EN: "Fitting decomposition", TR: "菲丁分解"}},
		{{EN: "Fitting decomposition", TR: "Fitting 分解"}},
	})
	if len(agreed) != 0 {
		t.Fatalf("agreed = %v, and the two rounds said different things", agreed)
	}
	if len(split) != 1 || split[0].EN != "Fitting decomposition" {
		t.Fatalf("split = %v", split)
	}
	if strings.Join(split[0].TR, ",") != "Fitting 分解,菲丁分解" {
		t.Errorf("TR = %v, want both readings in a settled order", split[0].TR)
	}
}

// A question that failed is not a vote against the terms in it. Requiring a
// majority of the rounds asked rather than of the answers that arrived would
// throw away good work every time a box fell over, and the boxes fall over.
func TestConsensusCountsTheAnswersAndNotTheQuestions(t *testing.T) {
	agreed, split := Consensus([][]Row{
		{{EN: "ring", TR: "环"}},
		nil,
		nil,
	})
	if len(agreed) != 1 || agreed[0].TR != "环" {
		t.Fatalf("agreed = %v, split = %v", agreed, split)
	}
}

// The same batch can go out twice in one round, and its second answer is the
// same opinion arriving again rather than a second one.
func TestConsensusVotesOncePerRound(t *testing.T) {
	_, split := Consensus([][]Row{
		{{EN: "ring", TR: "环"}, {EN: "Ring", TR: "环"}},
		{{EN: "ring", TR: "圆环"}},
	})
	if len(split) != 1 {
		t.Fatalf("split = %v, and one round saying 环 twice is one vote for 环", split)
	}
}

// Whitespace is not a disagreement.
func TestConsensusIgnoresSpacingAndCase(t *testing.T) {
	agreed, split := Consensus([][]Row{
		{{EN: "root system", TR: "根系"}},
		{{EN: "ROOT SYSTEM", TR: " 根系 "}},
	})
	if len(agreed) != 1 || len(split) != 0 {
		t.Fatalf("agreed = %v, split = %v", agreed, split)
	}
}
