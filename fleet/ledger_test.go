package fleet

import (
	"errors"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

// The row that started it. Twenty one verified profiles, ten of them ready and
// idle, and three more lanes put against them wrote nothing at all, because the
// composer on that host was not taking the prompt. Every count in the table was
// right and the table was still telling somebody to send work there.
func TestAHostWithReadyProfilesThatWillNotComposeSaysSo(t *testing.T) {
	led := NewLedger()
	for range 34 {
		led.Note("box", NoComposer)
	}
	for range 6 {
		led.Note("box", Answered)
	}
	boards := led.Apply([]Accounts{{Host: "box", Kind: Browser, Verified: 21, Ready: 10}}, LedgerWindow)
	if !boards[0].NotTaking() {
		t.Fatal("a host that answered 6 of its last 40 asks was not called out")
	}
	row := AccountsTable(boards)
	if !strings.Contains(row, "6/40") {
		t.Errorf("the taking column does not carry the count:\n%s", row)
	}
	if !strings.Contains(row, "never reached a composer") {
		t.Errorf("the row still reads as a host with capacity:\n%s", row)
	}
	// The counts it does have are still printed. What changed is the sentence
	// about what to do with them, not the numbers themselves.
	if !strings.Contains(row, "21") || !strings.Contains(row, "10") {
		t.Errorf("the account counts were dropped:\n%s", row)
	}
}

// And a host that is merely busy or out of turns is not the same thing. Those
// are honest states, one is a box doing work and the other is a quota that
// comes back on its own, and calling either of them broken would send the fleet
// away from the hosts it has.
func TestABusyHostIsNotAHostThatWillNotCompose(t *testing.T) {
	for _, kind := range []Outcome{Busy, OutOfTurns, Failed} {
		led := NewLedger()
		for range 40 {
			led.Note("box", kind)
		}
		boards := led.Apply([]Accounts{{Host: "box", Kind: Browser, Verified: 21, Ready: 10}}, LedgerWindow)
		if boards[0].NotTaking() {
			t.Errorf("%s was read as a host that cannot take a prompt", kind)
		}
		if got := AccountsTable(boards); !strings.Contains(got, "0/40") {
			t.Errorf("%s: the taking column does not say none were answered:\n%s", kind, got)
		}
	}
}

// Two failures in a row is a bad minute and every host has those. A verdict
// wants a sample.
func TestOneBadMinuteIsNotAVerdict(t *testing.T) {
	led := NewLedger()
	for range enoughAsks - 1 {
		led.Note("box", NoComposer)
	}
	boards := led.Apply([]Accounts{{Host: "box", Kind: Browser, Verified: 21, Ready: 10}}, LedgerWindow)
	if boards[0].NotTaking() {
		t.Errorf("%d failures were enough to condemn a host", enoughAsks-1)
	}
	// A host nothing has asked at all says nothing rather than saying zero,
	// because zero out of zero reads as a host that failed everything.
	empty := led.Apply([]Accounts{{Host: "elsewhere", Kind: Browser, Verified: 4, Ready: 4}}, LedgerWindow)
	if got := AccountsTable(empty); !strings.Contains(got, "  -  ") {
		t.Errorf("a host with no record does not read as unknown:\n%s", got)
	}
}

func TestTheWordsTheRunnerLogsCarryAreSortedByWhatTheySayAboutTheHost(t *testing.T) {
	for _, c := range []struct {
		message string
		want    Outcome
	}{
		{"question tr-1 on a stopped without writing an answer: browser: asking 7523 chars ... ERROR ChatGPT never accepted the prompt", NoComposer},
		{"the model is out of turns for now, usage limit", OutOfTurns},
		{"429 too many requests", OutOfTurns},
		{"[router] All verified slots busy", Busy},
		{"browser: ERROR Cloudflare is holding this host on an interstitial", Failed},
	} {
		if got := ClassifyText(c.message); got != c.want {
			t.Errorf("%q sorted as %q, want %q", c.message, got, c.want)
		}
		if got := Classify(errors.New(c.message)); got != c.want {
			t.Errorf("as an error, %q sorted as %q, want %q", c.message, got, c.want)
		}
	}
	if got := Classify(nil); got != Answered {
		t.Errorf("no error sorted as %q, want an answer", got)
	}
}

// Only the window counts, because the point of the record is what the host is
// doing now. An eight hour cooldown is longer than the window on purpose, so a
// ban that has since lifted cannot be what a host is judged on.
func TestOnlyTheRecentAsksCount(t *testing.T) {
	led := NewLedger()
	old := time.Now().Add(-6 * time.Hour)
	for range 40 {
		led.Hosts["box"] = append(led.Hosts["box"], Asked{At: old, Outcome: NoComposer})
	}
	for range 5 {
		led.Note("box", Answered)
	}
	asks, byKind := led.Recent("box", LedgerWindow)
	if asks != 5 || byKind[Answered] != 5 {
		t.Errorf("Recent gave %d asks %v, want the 5 inside the window", asks, byKind)
	}
	if LedgerWindow >= 8*time.Hour {
		t.Error("the window is as long as a cooldown, so a lifted ban can still condemn a host")
	}
}

// Two runs on this laptop at once is the ordinary case, and the second one to
// finish must not drop what the first wrote down.
func TestTwoRunsWritingTheRecordKeepBothHalvesOfIt(t *testing.T) {
	path := filepath.Join(t.TempDir(), "asks.json")
	first, second := NewLedger(), NewLedger()
	for range 3 {
		first.Note("box", Answered)
		second.Note("box", NoComposer)
		second.Note("other", Answered)
	}
	if err := first.Append(path); err != nil {
		t.Fatal(err)
	}
	if err := second.Append(path); err != nil {
		t.Fatal(err)
	}
	back, err := LoadLedger(path)
	if err != nil {
		t.Fatal(err)
	}
	asks, byKind := back.Recent("box", LedgerWindow)
	if asks != 6 || byKind[Answered] != 3 || byKind[NoComposer] != 3 {
		t.Errorf("the record holds %d asks %v, want both runs", asks, byKind)
	}
	if asks, _ := back.Recent("other", LedgerWindow); asks != 3 {
		t.Errorf("the second host holds %d asks, want 3", asks)
	}
	// And a file that is not there yet is not an error, because before the
	// first run there is nothing to read.
	if _, err := LoadLedger(filepath.Join(t.TempDir(), "nothing.json")); err != nil {
		t.Errorf("a missing record was an error: %v", err)
	}
}

// The file cannot grow without bound, and what it drops is the oldest.
func TestTheRecordKeepsTheNewestAndDropsTheRest(t *testing.T) {
	led := NewLedger()
	for range LedgerDepth + 50 {
		led.Note("box", NoComposer)
	}
	led.Note("box", Answered)
	if got := len(led.Hosts["box"]); got != LedgerDepth {
		t.Errorf("the record holds %d asks, want it trimmed to %d", got, LedgerDepth)
	}
	if last := led.Hosts["box"][LedgerDepth-1]; last.Outcome != Answered {
		t.Errorf("the newest ask is %q, want the one just noted", last.Outcome)
	}
}

// A reader has no account pool and no composer, so none of this is about it and
// the row it gets is the one fleet gives a reader.
func TestTheRecordDoesNotChangeWhatAReaderRowSays(t *testing.T) {
	led := NewLedger()
	for range 40 {
		led.Note("card", NoComposer)
	}
	boards := led.Apply([]Accounts{{Host: "card", Kind: Reader, Model: "reader-a", Answers: true}}, LedgerWindow)
	got := AccountsTable(boards)
	if !strings.Contains(got, "reader-a") || !strings.Contains(got, "answering") {
		t.Errorf("the reader row was overwritten:\n%s", got)
	}
	if strings.Contains(got, "never reached a composer") {
		t.Errorf("a reader was judged on a composer it has not:\n%s", got)
	}
}
