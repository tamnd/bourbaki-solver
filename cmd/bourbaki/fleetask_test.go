package main

import (
	"errors"
	"path/filepath"
	"sync"
	"testing"

	"github.com/tamnd/bourbaki-solver/fleet"
)

// The record is what fleet accounts reads back, and a host checked by hand is
// the case it was least able to see: the check that exists to catch a host
// whose profiles will not take a prompt was the one check that told it nothing.
func TestAnAskThatDiedAtTheComposerIsInTheRecordAsSuch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asks.json")
	t.Setenv("BOURBAKI_FLEET_ASKS", path)

	noteAsk("server-x", errors.New("browser: asking 30 chars ERROR ChatGPT never accepted the prompt"))

	led, err := fleet.LoadLedger(path)
	if err != nil {
		t.Fatalf("read the record back: %v", err)
	}
	asks, byKind := led.Recent("server-x", fleet.LedgerWindow)
	if asks != 1 {
		t.Fatalf("recorded %d asks, want 1", asks)
	}
	if byKind[fleet.NoComposer] != 1 {
		t.Errorf("recorded it as %v, want one no composer", byKind)
	}
}

// An ask that worked has to count too. A record holding only the failures says
// every host is broken, which is the same lie the other way round.
func TestAnAskThatAnsweredIsInTheRecordAsAnAnswer(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asks.json")
	t.Setenv("BOURBAKI_FLEET_ASKS", path)

	noteAsk("server-x", nil)

	led, err := fleet.LoadLedger(path)
	if err != nil {
		t.Fatalf("read the record back: %v", err)
	}
	_, byKind := led.Recent("server-x", fleet.LedgerWindow)
	if byKind[fleet.Answered] != 1 {
		t.Errorf("recorded it as %v, want one answered", byKind)
	}
}

// Append merges rather than overwrites, and one ask at a time is how this
// command writes, so a hand check must not wipe the run that came before it.
func TestAskingByHandKeepsWhatTheRunsBeforeItRecorded(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asks.json")
	t.Setenv("BOURBAKI_FLEET_ASKS", path)

	before := fleet.NewLedger()
	for i := 0; i < 3; i++ {
		before.Note("server-x", fleet.Answered)
	}
	if err := before.Append(path); err != nil {
		t.Fatalf("write the earlier run: %v", err)
	}

	noteAsk("server-x", errors.New("ChatGPT never accepted the prompt"))

	led, err := fleet.LoadLedger(path)
	if err != nil {
		t.Fatalf("read the record back: %v", err)
	}
	asks, byKind := led.Recent("server-x", fleet.LedgerWindow)
	if asks != 4 {
		t.Fatalf("recorded %d asks, want the 3 already there and this one", asks)
	}
	if byKind[fleet.Answered] != 3 || byKind[fleet.NoComposer] != 1 {
		t.Errorf("recorded %v, want 3 answered and 1 no composer", byKind)
	}
}

// The solve lanes ask in parallel and every one of them appends, and Append
// re-reads the file it is adding to. Two landing together must not come to one
// ask recorded.
func TestTwoLanesRecordingAtOnceBothLandInTheRecord(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asks.json")
	t.Setenv("BOURBAKI_FLEET_ASKS", path)

	const lanes = 8
	var wg sync.WaitGroup
	for i := 0; i < lanes; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			noteAsk("server-x", nil)
		}()
	}
	wg.Wait()

	led, err := fleet.LoadLedger(path)
	if err != nil {
		t.Fatalf("read the record back: %v", err)
	}
	asks, _ := led.Recent("server-x", fleet.LedgerWindow)
	if asks != lanes {
		t.Errorf("recorded %d asks, want all %d", asks, lanes)
	}
}
