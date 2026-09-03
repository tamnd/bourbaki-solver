package benchmark

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The set that ships is the one the numbers in the scorecard came from, so a
// typo in it is a wrong number and not a build failure. This is what reads it.
func TestTheSetThatShipsIsASet(t *testing.T) {
	set, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if err := set.Check(); err != nil {
		t.Error(err)
	}
	for _, c := range set {
		if _, err := corpus.ParseLabel(c.Label); err != nil {
			t.Errorf("%s: %v", c.Name(), err)
		}
	}
}

func TestAVerdictThatIsNotOneIsRefused(t *testing.T) {
	for _, bad := range []struct {
		why string
		set Set
	}{
		{"a verdict that is neither", Set{{Label: "alg-viii-s1-ex-1", Variant: "as-solved",
			Expect: "probably", Why: "it reads well", Read: "tam, 2026-08-13"}}},
		{"no reading behind it", Set{{Label: "alg-viii-s1-ex-1", Variant: "as-solved",
			Expect: Accept, Read: "tam, 2026-08-13"}}},
		{"nobody behind it", Set{{Label: "alg-viii-s1-ex-1", Variant: "as-solved",
			Expect: Accept, Why: "it reads well"}}},
		{"no variant", Set{{Label: "alg-viii-s1-ex-1", Expect: Accept,
			Why: "it reads well", Read: "tam, 2026-08-13"}}},
		{"the same case twice", Set{
			{Label: "alg-viii-s1-ex-1", Variant: "as-solved", Expect: Accept,
				Why: "it reads well", Read: "tam, 2026-08-13"},
			{Label: "alg-viii-s1-ex-1", Variant: "as-solved", Expect: Reject,
				Why: "it does not", Read: "tam, 2026-08-13"}}},
	} {
		if err := bad.set.Check(); err == nil {
			t.Errorf("a set with %s passed", bad.why)
		}
	}
}

// A benchmark of nothing but right answers cannot measure the number the whole
// thing exists for, and a rate of zero over nothing would say the opposite.
func TestARateWithNothingBehindItIsNotZero(t *testing.T) {
	var s Score
	s.Add(Outcome{Case: Case{Expect: Accept}, Judged: true, Status: corpus.StatusVerified})
	if got := s.FalseAcceptRate(); got != -1 {
		t.Errorf("a false accept rate of %v over no wrong answers", got)
	}
	if _, _, measured := s.Met(); measured {
		t.Error("a run with nothing to get wrong reported a measurement")
	}
}

func TestTheTwoErrorsAreCountedApart(t *testing.T) {
	var s Score
	// A wrong answer accepted, which is the one that matters.
	s.Add(Outcome{Case: Case{Expect: Reject}, Judged: true, Status: corpus.StatusVerified})
	// A wrong answer rejected, which is the judges working.
	s.Add(Outcome{Case: Case{Expect: Reject}, Judged: true, Status: corpus.StatusUnverified})
	// A right answer rejected, which costs a rerun and nothing worse.
	s.Add(Outcome{Case: Case{Expect: Accept}, Judged: true, Status: corpus.StatusUnverified})
	// A right answer accepted.
	s.Add(Outcome{Case: Case{Expect: Accept}, Judged: true, Status: corpus.StatusVerified})

	if s.FalseAccepts != 1 || s.FalseRejects != 1 {
		t.Fatalf("false accepts %d and false rejects %d, both should be 1",
			s.FalseAccepts, s.FalseRejects)
	}
	if got := s.FalseAcceptRate(); got != 0.5 {
		t.Errorf("false accept rate %v, want 0.5", got)
	}
	accept, reject, measured := s.Met()
	if !measured || accept {
		t.Error("half the wrong answers accepted was not reported as over the target")
	}
	if reject {
		t.Error("half the right answers rejected was reported as inside a 30 per cent target")
	}
}

// Partial is a solution with a part still open. The corpus says so out loud, so
// a partial is not an answer that went into a book as done.
func TestOnlyVerifiedCountsAsAccepted(t *testing.T) {
	for _, status := range []string{corpus.StatusPartial, corpus.StatusUnverified,
		corpus.StatusBlocked, corpus.StatusOpen} {
		o := Outcome{Case: Case{Expect: Accept}, Judged: true, Status: status}
		if o.Accepted() {
			t.Errorf("%s was counted as accepted", status)
		}
	}
}

// A case the reference call stopped is a fact about the corpus and not an
// opinion about the answer, so it belongs in neither rate.
func TestWhatTheReferenceStoppedIsCountedApart(t *testing.T) {
	var s Score
	s.Add(Outcome{Case: Case{Expect: Accept}, Judged: false, Status: corpus.StatusBlocked})
	if s.Accepts != 0 || s.FalseRejects != 0 {
		t.Error("a blocked exercise was counted as the verifier being strict")
	}
	if s.Unjudged != 1 {
		t.Error("a blocked exercise was not counted at all")
	}
}

// The scorecard quotes the last run rather than recomputing it, so what is
// written has to come back the way it went in.
func TestARunComesBackTheWayItWentIn(t *testing.T) {
	root := t.TempDir()
	if _, measured, err := LastRun(root); err != nil || measured {
		t.Fatalf("a corpus that has measured nothing reported a run, err %v", err)
	}
	var score Score
	score.Add(Outcome{Case: Case{Expect: Reject}, Judged: true, Status: corpus.StatusVerified})
	score.Add(Outcome{Case: Case{Expect: Accept}, Judged: true, Status: corpus.StatusVerified})
	in := Run{Ran: "2026-08-13T02:00:00Z", Set: "the built-in set", Score: score}
	if err := in.Save(root); err != nil {
		t.Fatal(err)
	}
	out, measured, err := LastRun(root)
	if err != nil || !measured {
		t.Fatalf("the run did not come back, err %v", err)
	}
	if out.Score != in.Score || out.Ran != in.Ran {
		t.Errorf("got %+v, want %+v", out, in)
	}
	// The rates are filled in on the way out, so that a reader who is not this
	// package does not have to know how they are worked out.
	if out.Rates["false_accept"] != 1 {
		t.Errorf("the saved rate was %v, want 1", out.Rates["false_accept"])
	}
}

// A run that was stopped has fewer answers than it has cases, and the gap is
// the only thing in the file that says so. A five hour run that was stopped
// wrote 4 outcomes over a committed 20 and read as a clean eval.
func TestARunSaysHowMuchOfTheSetItAnswered(t *testing.T) {
	for _, c := range []struct {
		name    string
		run     Run
		partial bool
	}{
		{"answered every case", Run{Of: 20, Outcomes: make([]Outcome, 20)}, false},
		{"stopped a fifth of the way in", Run{Of: 20, Outcomes: make([]Outcome, 4)}, true},
		{"answered nothing", Run{Of: 20}, true},
		// Written before the field existed, which is every committed run, and
		// those are the complete ones.
		{"from before the field", Run{Outcomes: make([]Outcome, 20)}, false},
	} {
		t.Run(c.name, func(t *testing.T) {
			if got := c.run.Partial(); got != c.partial {
				t.Errorf("Partial() = %v, want %v", got, c.partial)
			}
		})
	}
}

// Saving goes through a temporary file so a run killed while it writes leaves
// the last good result rather than half of a new one.
func TestSavingLeavesNoTemporaryFileBehind(t *testing.T) {
	root := t.TempDir()
	in := Run{Ran: "2026-09-04T02:00:00Z", Set: "the built-in set", Of: 2,
		Outcomes: make([]Outcome, 2)}
	if err := in.Save(root); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(RunPath(root) + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("the temporary file is still there, err %v", err)
	}
	out, measured, err := LastRun(root)
	if err != nil || !measured {
		t.Fatalf("the run did not come back, err %v", err)
	}
	if out.Of != 2 || out.Partial() {
		t.Errorf("got of=%d partial=%v, want of=2 and whole", out.Of, out.Partial())
	}
}

// The answers live in the corpus and this repository is public.
func TestTheAnswerIsLookedForInTheCorpus(t *testing.T) {
	c := Case{Label: "alg-viii-s1-ex-1", Variant: "flawed-induction"}
	got := c.Body("/corpus", "en")
	want := filepath.Join("/corpus", "benchmark", "en", "alg-viii-s1-ex-1.flawed-induction.md")
	if got != want {
		t.Errorf("the answer was looked for at %s, want %s", got, want)
	}
	if strings.Contains(got, "content") {
		t.Error("a benchmark answer was looked for inside content/, which the audit walks")
	}
}
