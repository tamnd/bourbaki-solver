package benchmark

import "github.com/tamnd/bourbaki-solver/corpus"

// The targets of spec 07 §6, as fractions.
const (
	FalseAcceptTarget = 0.05
	FalseRejectTarget = 0.30
)

// Outcome is one case after the judges have had it.
type Outcome struct {
	Case
	// Status is what the pipeline made of the answer, in the corpus taxonomy.
	Status string `json:"status"`
	// Judged is false where the reference call stopped before the judges, which
	// is a blocked or an open exercise. Neither is an opinion about the answer,
	// and counting one as a rejection would make the corpus's own gaps look like
	// the verifier being strict.
	Judged bool   `json:"judged"`
	Truth  bool   `json:"truth_judge_passed"`
	Audit  bool   `json:"audit_judge_passed"`
	Why    string `json:"judge_said,omitempty"`
	Calls  int    `json:"calls"`
}

// Accepted is whether the pipeline would have let this answer into a book.
//
// Verified and nothing else. Partial is a solution with a part still open,
// which is a thing the corpus says out loud and does not present as done.
func (o Outcome) Accepted() bool { return o.Judged && o.Status == corpus.StatusVerified }

// Agreed is whether the judges and the person who read it came to the same
// place.
func (o Outcome) Agreed() bool {
	if !o.Judged {
		return false
	}
	return o.Accepted() == (o.Expect == Accept)
}

// Score is the whole run counted up.
type Score struct {
	// Accepts and Rejects are how many cases a person put each way, counting
	// only the ones the judges actually ruled on.
	Accepts int `json:"accepts"`
	Rejects int `json:"rejects"`
	// FalseAccepts are wrong answers the judges let through, and they are the
	// number that matters. FalseRejects are right answers the judges threw out.
	FalseAccepts int `json:"false_accepts"`
	FalseRejects int `json:"false_rejects"`
	// Unjudged are the cases the reference call stopped, which are counted
	// nowhere else and are reported so that a set that quietly shrank is
	// visible.
	Unjudged int `json:"unjudged"`
}

// Add counts one outcome.
func (s *Score) Add(o Outcome) {
	if !o.Judged {
		s.Unjudged++
		return
	}
	if o.Expect == Accept {
		s.Accepts++
		if !o.Accepted() {
			s.FalseRejects++
		}
		return
	}
	s.Rejects++
	if o.Accepted() {
		s.FalseAccepts++
	}
}

// FalseAcceptRate is wrong answers accepted over wrong answers put. It is -1
// when no wrong answer was put, which is not a rate of zero: a run with nothing
// to get wrong has measured nothing.
func (s Score) FalseAcceptRate() float64 { return rate(s.FalseAccepts, s.Rejects) }

// FalseRejectRate is right answers thrown out over right answers put.
func (s Score) FalseRejectRate() float64 { return rate(s.FalseRejects, s.Accepts) }

func rate(n, of int) float64 {
	if of == 0 {
		return -1
	}
	return float64(n) / float64(of)
}

// Met says whether both rates are inside the targets. A rate that was not
// measured does not meet anything, and does not fail either, which is why this
// returns the two separately from whether either was measured at all.
func (s Score) Met() (accept, reject, measured bool) {
	fa, fr := s.FalseAcceptRate(), s.FalseRejectRate()
	if fa < 0 || fr < 0 {
		return fa >= 0 && fa <= FalseAcceptTarget, fr >= 0 && fr <= FalseRejectTarget, false
	}
	return fa <= FalseAcceptTarget, fr <= FalseRejectTarget, true
}
