package fleet

import (
	"context"
	"errors"
	"strings"
	"testing"
)

// The row that started this. The best reader in the fleet was told it could
// read nothing, on the grounds that it has no ChatGPT account, which it does not
// need and never will.
func TestAReaderIsNotJudgedOnAnAccountPoolItHasNot(t *testing.T) {
	runner := &fakeRunner{out: map[string]string{"reader-box": "reader_answers=yes\n"}}
	board := Board(context.Background(), runner, Target{Name: "reader", Host: "reader-box",
		Kind: Reader, ReaderURL: "http://127.0.0.1:8801/v1", ReaderModel: "reader-a"})

	if board.Err != "" {
		t.Fatalf("Err = %q, want a serving reader reported as fine", board.Err)
	}
	if !board.Answers {
		t.Fatal("a reader whose endpoint answered was not recorded as answering")
	}
	if board.Model != "reader-a" {
		t.Errorf("Model = %q, want the weights it is serving", board.Model)
	}
	// It must never have run the account table, which is where the wrong
	// sentence came from.
	if strings.Contains(runner.command, "accounts") {
		t.Errorf("a reader was asked for its account table: %s", runner.command)
	}
	if !strings.Contains(runner.command, "http://127.0.0.1:8801/v1/models") {
		t.Errorf("the endpoint was not asked: %s", runner.command)
	}

	row := AccountsTable([]Accounts{board})
	if !strings.Contains(row, "reader-a") {
		t.Errorf("the row does not say which model it serves:\n%s", row)
	}
	if strings.Contains(row, "can read nothing") {
		t.Errorf("the row still says the reader can read nothing:\n%s", row)
	}
}

// And the reader's own failure, which is the one that will actually happen: the
// box is up, ssh works, and the service behind the card is down.
func TestAReaderWhoseServerIsDownSaysSo(t *testing.T) {
	runner := &fakeRunner{out: map[string]string{"reader-box": "reader_answers=\n"}}
	board := Board(context.Background(), runner, Target{Name: "reader", Host: "reader-box",
		Kind: Reader, ReaderURL: "http://127.0.0.1:8801/v1", ReaderModel: "reader-a"})

	if board.Answers {
		t.Fatal("a reader whose endpoint said nothing was recorded as answering")
	}
	if !strings.Contains(board.Err, "8801") {
		t.Errorf("Err = %q, want it to name the endpoint that is down", board.Err)
	}
	if got := AccountsTable([]Accounts{board}); !strings.Contains(got, "not answering") {
		t.Errorf("the row does not say the server is down:\n%s", got)
	}
}

// A host that never answered is a third state, and the table used to print the
// transport error into a row of counts.
func TestAHostThatDidNotAnswerIsNotAHostWithNoAccounts(t *testing.T) {
	runner := &fakeRunner{err: map[string]error{"s": errors.New("s: context deadline exceeded after 1m0s")}}
	board := Board(context.Background(), runner, Target{Name: "s", Host: "s"})
	if !board.TimedOut {
		t.Fatal("a host that ran out its deadline was not recorded as having timed out")
	}
	got := AccountsTable([]Accounts{board})
	if !strings.Contains(got, "did not answer") {
		t.Errorf("the row does not say the host never answered:\n%s", got)
	}
	// A host with an empty pool is still its own sentence, and the two must not
	// read the same.
	empty := AccountsTable([]Accounts{{Host: "s", Kind: Browser, Err: "no signed in profile, so this host can read nothing"}})
	if got == empty {
		t.Error("a host that timed out and a host with no profiles print the same row")
	}
}

// CanOCR asked one question of every host, and a reader answers no to all of it.
func TestTheGateAsksAReaderAboutItsCardAndNotAboutABrowser(t *testing.T) {
	// Everything a browser needs is missing, which is how the reader really is:
	// no chatgpt-tool worth using, no Xvfb, and 900 MB free, which is under the
	// browser profile floor and irrelevant to a server already holding weights.
	reader := Facts{Kind: Reader, ReaderTool: "/opt/local-ocr", Rsync: true,
		MemFreeMB: 900, DiskFreeMB: 40000,
		ReaderURL: "http://127.0.0.1:8801/v1", ReaderAnswers: true}
	if ok, why := reader.CanOCR(); !ok {
		t.Fatalf("a serving reader was refused: %s", why)
	}
	if got := reader.Lanes(); got < 1 {
		t.Errorf("Lanes = %d, want a serving reader to get work", got)
	}

	// The same box with its model server down takes nothing, and that is the
	// clause that should fire.
	down := reader
	down.ReaderAnswers = false
	ok, why := down.CanOCR()
	if ok {
		t.Fatal("a reader with no model server behind it was called capable")
	}
	if !strings.Contains(why, "8801") {
		t.Errorf("reason = %q, want it to name the endpoint", why)
	}
	if got := down.Lanes(); got != 0 {
		t.Errorf("Lanes = %d, want 0", got)
	}

	// A reader still needs the images, and it still needs somewhere to put them.
	for _, c := range []struct {
		name  string
		facts Facts
		want  string
	}{
		{"no rsync", func() Facts { f := reader; f.Rsync = false; return f }(), "rsync"},
		{"no reader program", func() Facts { f := reader; f.ReaderTool = ""; return f }(), "reader program"},
		{"a full disk", func() Facts { f := reader; f.DiskFreeMB = 0; return f }(), "disk"},
	} {
		ok, why := c.facts.CanOCR()
		if ok || !strings.Contains(why, c.want) {
			t.Errorf("%s: CanOCR = %t %q, want a refusal mentioning %q", c.name, ok, why, c.want)
		}
	}
}

// And a browser host is judged exactly as it always was, because the zero value
// of the kind is the kind every host in the pool used to be.
func TestAHostWithNoKindIsStillJudgedAsABrowser(t *testing.T) {
	facts := Facts{Cores: 8, MemFreeMB: 13217, DiskFreeMB: 12024, Tool: "t", Rsync: true}
	ok, why := facts.CanOCR()
	if ok || !strings.Contains(why, "xvfb") {
		t.Errorf("CanOCR = %t %q, want the browser test applied to a host with no kind", ok, why)
	}
	if got := Kind("").Or(); got != Browser {
		t.Errorf("the zero kind is %q, want a browser", got)
	}
}

// The sweep sleeps on the account board, so a reader that is up has to end the
// sleep. Otherwise the fleet waits out a browser cooldown with the fastest
// reader in it doing nothing.
func TestAServingReaderMeansThereIsNoReasonToWait(t *testing.T) {
	banned := Accounts{Host: "b", Kind: Browser, Verified: 10, Banned: 10, Soonest: 90 * 60 * 1e9}
	if got := Wait([]Accounts{banned}); got == 0 {
		t.Fatal("a host with every profile banned gave nothing to wait for")
	}
	reader := Accounts{Host: "r", Kind: Reader, Model: "reader-a", Answers: true}
	if got := Wait([]Accounts{banned, reader}); got != 0 {
		t.Errorf("Wait = %s with a reader answering, want no wait at all", got)
	}
	// A reader that is down has no cooldown to offer either, so it must not be
	// counted as a reason to keep waiting or as a reason to stop.
	down := Accounts{Host: "r", Kind: Reader, Model: "reader-a"}
	if got := Wait([]Accounts{banned, down}); got != banned.Soonest {
		t.Errorf("Wait = %s with the reader down, want the browser cooldown %s", got, banned.Soonest)
	}
}

// The probe only asks the endpoint of a host that has one, because the other
// boxes have nothing at that address and a curl to nowhere is a wasted five
// seconds on every probe of every host.
func TestOnlyAReaderIsAskedAboutAnEndpoint(t *testing.T) {
	runner := &fakeRunner{out: map[string]string{"browser-box": "host=browser-box\ntool=t\n"}}
	Probe(context.Background(), runner, Target{Name: "browser", Host: "browser-box", Port: 8077})
	if strings.Contains(runner.command, "reader_answers") {
		t.Errorf("a browser box was asked about a model endpoint: %s", runner.command)
	}
}
