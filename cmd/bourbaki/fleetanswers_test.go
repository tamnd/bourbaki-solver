package main

import (
	"testing"
	"time"

	"github.com/tamnd/bourbaki-solver/queue"
)

// The board is meant to say what arrived and not only what was asked for, so
// this is read back off the work that has already run.
func TestAnswersReadTheModelOffTheFinishedWork(t *testing.T) {
	q, err := queue.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	clock := time.Date(2026, 8, 12, 7, 0, 0, 0, time.UTC)
	q.Now = func() time.Time { clock = clock.Add(time.Minute); return clock }

	finish := func(stage queue.Stage, target, host, model string) {
		item := queue.New(stage, target, target, "prompt-v1")
		if _, err := q.Add(item); err != nil {
			t.Fatal(err)
		}
		leased, err := q.Lease(stage, host, queue.GroupOf(target), time.Minute)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := q.Finish(leased, true, model); err != nil {
			t.Fatal(err)
		}
	}
	finish(queue.StageTranslate, "vi-aaaaaa/001", "server3", "gpt-5")
	finish(queue.StageTranslate, "vi-aaaaaa/002", "server3", "gpt-5-6-mini")
	finish(queue.StageOCR, "alg-x-fr/0174", "server2", "gpt-5")

	seen, err := answers(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 2 {
		t.Fatalf("hosts = %v, want the two that answered", seen)
	}
	// The account was moved down partway through, and the board has to say the
	// one that answered last rather than the one it started on.
	if got := seen["server3"]; got.Model != "gpt-5-6-mini" || got.Jobs != 2 {
		t.Errorf("server3 last answered on %q over %d jobs", got.Model, got.Jobs)
	}
	if got := seen["server3"].Models; got["gpt-5"] != 1 || got["gpt-5-6-mini"] != 1 {
		t.Errorf("server3 models = %v, want both kept", got)
	}
	// Stages do not see each other anywhere else in the queue, but a host is a
	// host: what OCR found out about a box is true for translation too.
	if got := seen["server2"]; got.Model != "gpt-5" || got.Jobs != 1 {
		t.Errorf("server2 = %+v, want the OCR job counted", got)
	}
}

// A failed attempt records why it failed, not what answered it. Counting those
// would put a refusal in the model column.
func TestAFailedAttemptIsNotAModel(t *testing.T) {
	q, err := queue.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	item := queue.New(queue.StageTranslate, "vi-aaaaaa/001", "input", "prompt-v1")
	if _, err := q.Add(item); err != nil {
		t.Fatal(err)
	}
	leased, err := q.Lease(queue.StageTranslate, "server3", "vi-aaaaaa", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.Fail(leased, "the formula came back reflowed"); err != nil {
		t.Fatal(err)
	}
	again, err := q.Lease(queue.StageTranslate, "server3", "vi-aaaaaa", time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := q.Finish(again, true, "gpt-5"); err != nil {
		t.Fatal(err)
	}

	seen, err := answers(q)
	if err != nil {
		t.Fatal(err)
	}
	got := seen["server3"]
	if got == nil || got.Model != "gpt-5" || len(got.Models) != 1 {
		t.Fatalf("server3 = %+v, want only the answer that came back", got)
	}
}

// Nothing has run, so there is nothing to say. An empty column reads like a
// fact, and the fact here is that nobody knows yet.
func TestNoFinishedWorkSaysNothing(t *testing.T) {
	q, err := queue.Open(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	seen, err := answers(q)
	if err != nil {
		t.Fatal(err)
	}
	if len(seen) != 0 {
		t.Fatalf("seen = %v", seen)
	}
}
