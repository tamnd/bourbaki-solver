package main

import (
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/report"
)

// What is written has to be what report usage reads back, since the two are in
// different packages and nothing but this holds them to the same shape.
func TestNoteAsksWritesWhatTheReportReads(t *testing.T) {
	root := t.TempDir()
	note := noteAsks(root, nil)
	note(ocr.Note{When: time.Now(), Stage: "translate vi", Host: "server3",
		Target: "content/en/ens/II/01.md chunk 1 of 3", Chars: 5820, Elapsed: "48s", OK: true})
	note(ocr.Note{When: time.Now(), Stage: "translate vi", Host: "server2",
		Chars: 4400, Elapsed: "2m0s", Reason: "no uploads left, wait 17 minutes to upload again"})

	file, err := os.Open(filepath.Join(root, "reports", "ask-usage.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	asks, bad, err := report.ReadAsks(file)
	if err != nil || bad != 0 {
		t.Fatalf("%d bad lines, %v", bad, err)
	}
	got := report.SummariseAsks(asks, "", time.Time{})
	if got.Total.Asks != 2 || got.Total.Answered != 1 {
		t.Errorf("total %+v", got.Total)
	}
	if got.Total.Took != 48*time.Second+2*time.Minute {
		t.Errorf("took %s", got.Total.Took)
	}
	if n := got.Total.Reasons["no uploads left on the account"]; n != 1 {
		t.Errorf("the refusal was filed under %v", got.Total.Reasons)
	}
}

// The lanes ask in parallel and solve builds a recorder an exercise, so two
// recorders over one file is the ordinary case and not the odd one.
func TestNoteAsksIsSafeAcrossRecorders(t *testing.T) {
	root := t.TempDir()
	var wg sync.WaitGroup
	for i := range 8 {
		wg.Go(func() {
			note := noteAsks(root, nil)
			for range 8 {
				note(ocr.Note{When: time.Now(), Stage: "solve", Host: "server3",
					Target: strings.Repeat("x", i+1), Elapsed: "1s", OK: true})
			}
		})
	}
	wg.Wait()

	file, err := os.Open(filepath.Join(root, "reports", "ask-usage.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	defer file.Close()
	asks, bad, err := report.ReadAsks(file)
	if err != nil {
		t.Fatal(err)
	}
	if bad != 0 {
		t.Errorf("%d lines came out torn", bad)
	}
	if len(asks) != 64 {
		t.Errorf("%d questions recorded, want 64", len(asks))
	}
}
