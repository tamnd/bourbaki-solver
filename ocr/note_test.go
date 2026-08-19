package ocr

import (
	"context"
	"errors"
	"testing"
	"time"
)

type answers struct {
	answer Answer
	err    error
}

func (a answers) Do(context.Context) (Answer, error) { return a.answer, a.err }

func TestRecordedKeepsTheAnswerAndItsCost(t *testing.T) {
	var got Note
	call := Recorded{
		Asker: answers{answer: Answer{Text: "done", Model: "gpt-5", Elapsed: 48 * time.Second}},
		Stage: "translate", Host: "server3", Target: "content/en/ens/II/01.md", Chars: 5820,
		Note: func(n Note) { got = n },
	}
	if _, err := call.Do(context.Background()); err != nil {
		t.Fatal(err)
	}
	if !got.OK || got.Stage != "translate" || got.Host != "server3" {
		t.Errorf("got %+v", got)
	}
	// The transport's own elapsed time is the one to believe: it knows when the
	// box started rather than when this process asked.
	if got.Elapsed != "48s" || got.Took() != 48*time.Second {
		t.Errorf("elapsed %q, want 48s", got.Elapsed)
	}
	if got.Chars != 5820 || got.Model != "gpt-5" {
		t.Errorf("got %+v", got)
	}
}

// A question that fails is the one worth recording. The archive of answers
// cannot hold it, so without this a run that asked four hundred questions and
// got three hundred back looks exactly like a run that asked three hundred.
func TestRecordedKeepsTheQuestionsThatFailed(t *testing.T) {
	var got Note
	call := Recorded{
		Asker: answers{err: errors.New("no uploads left, wait 17 minutes")},
		Stage: "solve", Host: "server2",
		Note: func(n Note) { got = n },
	}
	if _, err := call.Do(context.Background()); err == nil {
		t.Fatal("the error was swallowed")
	}
	if got.OK {
		t.Error("a failed question was recorded as answered")
	}
	if got.Reason != "no uploads left, wait 17 minutes" {
		t.Errorf("reason %q", got.Reason)
	}
	if got.Stage != "solve" || got.Host != "server2" {
		t.Errorf("got %+v", got)
	}
}

// Nowhere to write is not a reason for the caller to branch.
func TestRecordedWithNoRecorderIsAPassThrough(t *testing.T) {
	call := Recorded{Asker: answers{answer: Answer{Text: "done"}}}
	answer, err := call.Do(context.Background())
	if err != nil || answer.Text != "done" {
		t.Errorf("got %q, %v", answer.Text, err)
	}
}
