package solve

import (
	"strings"
	"testing"
)

// The question exercise 2 of § 1 was actually sent, in the two lines of it that
// matter here.
const asked = `#### Lemma 2 {#alg-viii-s1-lem-2 .statement tag=00QV}

- alg-viii-s1-prop-9, tag 000T: the references came to more than 40000 characters
`

func TestATagTheQuestionNeverCarriedIsRefused(t *testing.T) {
	want := wantSolutionFrom(asked)
	for _, c := range []struct {
		why, solution string
		ok            bool
	}{
		{"a printed statement", "the work\n\nUSES: 00QV", true},
		{"one named and not printed", "the work\n\nUSES: 000T", true},
		{"both", "the work\n\nUSES: 00QV, 000T", true},
		{"no citation at all", "the work\n\nUSES:", true},
		// Theorem 1 of Appendix 3, the Nullstellensatz, which is a real tag of
		// this corpus and was nowhere in this question.
		{"a tag from somewhere else", "the work\n\nUSES: 00QM", false},
		{"one of each", "the work\n\nUSES: 00QV, 00QM", false},
	} {
		err := want(c.solution)
		if c.ok && err != nil {
			t.Errorf("%s was refused: %v", c.why, err)
		}
		if !c.ok && err == nil {
			t.Errorf("%s passed", c.why)
		}
	}
}

// The retry is worth a call only if the answer says which tag it was, since the
// model is being asked to write the line again and not to guess which of four
// characters offended.
func TestTheRefusalNamesTheTag(t *testing.T) {
	err := wantSolutionFrom(asked)("the work\n\nUSES: 00QM, 00QN")
	if err == nil {
		t.Fatal("two fabricated tags passed")
	}
	for _, want := range []string{"00QM", "00QN", "were"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("%q is not in %q", want, err)
		}
	}
}

// A solution that came back empty or came back as the model talking about itself
// is still refused first. A fabricated citation is the smaller complaint and
// naming it instead would send the model back to fix the wrong thing.
func TestTheOlderRefusalsStillCome(t *testing.T) {
	if err := wantSolutionFrom(asked)("   "); err == nil {
		t.Error("an empty answer passed")
	}
}
