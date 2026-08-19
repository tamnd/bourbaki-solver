package ocr

import (
	"context"
	"time"
)

// Keeping a record of every question the project puts to the fleet.
//
// Reading a page already writes one line a batch to reports/ocr-usage.jsonl,
// and that file is where the schedule for the remaining volumes comes from.
// Nothing was written down for the other three things that ask: translation,
// solving and the glossary. Those are most of the questions by count and all of
// the questions by length, and the only trace they left was the archive of what
// came back, which says nothing about what a question cost or how often one
// went unanswered.
//
// This is the same accounting for them, taken at the one place they have in
// common. All three go through an Asker, so a wrapper around an Asker sees
// every question exactly once, including the ones that fail, which the archive
// by its nature never holds.

// Note is one question and what it cost.
//
// Elapsed is written as a string, "48s", so a person reading the file can check
// it against a wall clock, which is the same choice the OCR log made.
type Note struct {
	When  time.Time `json:"when"`
	Stage string    `json:"stage"`
	Host  string    `json:"host"`
	// Target is what the question was about: a section for a translation, an
	// exercise for a solution, a batch of terms for the glossary. It is what
	// makes a line worth reading on its own rather than only in a total.
	Target  string `json:"target,omitempty"`
	Model   string `json:"model,omitempty"`
	Chars   int    `json:"chars"`
	Elapsed string `json:"elapsed"`
	OK      bool   `json:"ok"`
	// Reason is the transport's own words when the question failed. It is kept
	// unclassified, because the phrases the fleet uses change and a report can
	// be taught new ones long after the line was written.
	Reason string `json:"reason,omitempty"`
}

// Took is the elapsed time as a duration.
func (n Note) Took() time.Duration {
	value, err := time.ParseDuration(n.Elapsed)
	if err != nil {
		return 0
	}
	return value
}

// Recorded is an Asker that hands a Note to a recorder on every question.
//
// It records the failures as well as the answers, and the failures are the
// point: a run that asks four hundred questions and gets three hundred back
// looks in the archive exactly like a run that asked three hundred.
//
// Note is called with the record even when the ask fails, and a nil Note makes
// this a plain pass through, so a caller with nowhere to write is not a caller
// that has to branch.
type Recorded struct {
	Asker
	Stage  string
	Host   string
	Target string
	Chars  int
	Note   func(Note)
	Now    func() time.Time
}

func (r Recorded) Do(ctx context.Context) (Answer, error) {
	started := r.now()
	answer, err := r.Asker.Do(ctx)
	if r.Note == nil {
		return answer, err
	}
	note := Note{
		When: started.UTC(), Stage: r.Stage, Host: r.Host, Target: r.Target,
		Model: answer.Model, Chars: r.Chars, OK: err == nil,
		// The transport reports its own elapsed time and it is the one to
		// believe, since it knows when the box started rather than when this
		// process asked. A failed question has none, so the wall clock stands
		// in: the time a question that never answered took is still time spent.
		Elapsed: r.now().Sub(started).Round(time.Second).String(),
	}
	if err == nil && answer.Elapsed > 0 {
		note.Elapsed = answer.Elapsed.Round(time.Second).String()
	}
	if err != nil {
		note.Reason = err.Error()
	}
	r.Note(note)
	return answer, err
}

func (r Recorded) now() time.Time {
	if r.Now != nil {
		return r.Now()
	}
	return time.Now()
}
