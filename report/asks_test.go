package report

import (
	"strings"
	"testing"
	"time"
)

const askLog = `
{"when":"2026-08-20T01:00:00Z","stage":"translate vi","host":"server3","target":"ens/II/01 chunk 1 of 3","chars":5820,"elapsed":"48s","ok":true}
{"when":"2026-08-20T01:01:00Z","stage":"translate vi","host":"server3","target":"ens/II/01 chunk 2 of 3","chars":5910,"elapsed":"52s","ok":true}
{"when":"2026-08-20T01:02:00Z","stage":"translate vi","host":"server2","target":"ens/II/01 chunk 3 of 3","chars":4400,"elapsed":"2m0s","ok":false,"reason":"no uploads left, wait 17 minutes to upload again"}
not json at all
{"when":"2026-08-20T01:03:00Z","stage":"solve","host":"server2","target":"03WE","chars":21000,"elapsed":"4m0s","ok":false,"reason":"ssh: connect to host server2 port 22: operation timed out"}
`

func TestSummariseAsksGroupsByStageAndHost(t *testing.T) {
	asks, bad, err := ReadAsks(strings.NewReader(askLog))
	if err != nil {
		t.Fatal(err)
	}
	if bad != 1 {
		t.Errorf("%d bad lines, want 1", bad)
	}
	if len(asks) != 4 {
		t.Fatalf("%d asks, want 4", len(asks))
	}

	got := SummariseAsks(asks, "", time.Time{})
	if len(got.Rows) != 3 {
		t.Fatalf("%d rows, want 3: %+v", len(got.Rows), got.Rows)
	}
	if got.Total.Asks != 4 || got.Total.Answered != 2 {
		t.Errorf("total %+v", got.Total)
	}
	// The two that came back took 48s and 52s, and the two that did not took six
	// minutes between them. All of it is wall clock the fleet spent.
	if want := 48*time.Second + 52*time.Second + 6*time.Minute; got.Total.Took != want {
		t.Errorf("took %s, want %s", got.Total.Took, want)
	}
	// Failures are in the total and not in the divisor: a run that loses half
	// its questions really does cost twice as much a question as one that does
	// not.
	if want := (48*time.Second + 52*time.Second + 6*time.Minute) / 2; got.Total.PerAnswer() != want {
		t.Errorf("per answer %s, want %s", got.Total.PerAnswer(), want)
	}

	first := got.Rows[0]
	if first.Stage != "solve" || first.Host != "server2" || first.Asks != 1 {
		t.Errorf("first row %+v", first)
	}
}

// The phrases are the fleet's own and they are the ones the batch table already
// knows, because an account out of uploads refuses a translation exactly as it
// refuses a page.
func TestAskReasonReadsTheFleetsOwnWords(t *testing.T) {
	quota := Ask{Reason: "no uploads left, wait 17 minutes to upload again"}
	if got := AskReason(quota); got != "no uploads left on the account" {
		t.Errorf("got %q", got)
	}
	// A reason this table has not learned is kept as it stands. Unlike a batch,
	// a question has the transport's error in hand and it is worth reading.
	ssh := Ask{Reason: "ssh: connect to host server2 port 22: operation timed out"}
	if got := AskReason(ssh); !strings.HasPrefix(got, "ssh: connect to host") {
		t.Errorf("got %q", got)
	}
	if got := AskReason(Ask{OK: true, Reason: "ignored"}); got != "" {
		t.Errorf("an answered question was given a reason: %q", got)
	}
	if got := AskReason(Ask{}); got != "no reason in the log" {
		t.Errorf("got %q", got)
	}
}

func TestSummariseAsksTakesAStagePrefix(t *testing.T) {
	asks, _, err := ReadAsks(strings.NewReader(askLog))
	if err != nil {
		t.Fatal(err)
	}
	if got := SummariseAsks(asks, "translate", time.Time{}); got.Total.Asks != 3 {
		t.Errorf("%d asks under translate, want 3", got.Total.Asks)
	}
	if got := SummariseAsks(asks, "translate vi", time.Time{}); got.Total.Asks != 3 {
		t.Errorf("%d asks under translate vi, want 3", got.Total.Asks)
	}
	if got := SummariseAsks(asks, "glossary", time.Time{}); got.Total.Asks != 0 {
		t.Errorf("%d asks under glossary, want none", got.Total.Asks)
	}
	if got := SummariseAsks(asks, "glossary", time.Time{}).Table(); !strings.Contains(got, "no questions") {
		t.Errorf("an empty summary printed %q", got)
	}
}

func TestAskedTablePrintsEveryRowAndTheTotal(t *testing.T) {
	asks, _, err := ReadAsks(strings.NewReader(askLog))
	if err != nil {
		t.Fatal(err)
	}
	table := SummariseAsks(asks, "", time.Time{}).Table()
	for _, want := range []string{"translate vi", "solve", "server3", "server2",
		"no uploads left on the account", "why questions went unanswered"} {
		if !strings.Contains(table, want) {
			t.Errorf("the table does not mention %q:\n%s", want, table)
		}
	}
}
