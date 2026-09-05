package main

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/ocr"
)

// -contents replaces the question a page is asked with one that keeps the
// leader dots and the printed page numbers where the printing sets them. A host
// whose program is not chatgpt-tool answers with a prompt of its own instead,
// takes the flag without complaint, and hands back an ordinary reading of a
// contents page. Nothing in the answer says the question was changed, so the
// run has to say it before it starts.

func silent(string, ...any) {}

func TestAContentsRunSkipsAHostThatReadsWithItsOwnPrompt(t *testing.T) {
	hosts := []ocr.Host{
		{Name: "reader", Reader: "local-ocr"},
		{Name: "browser"},
	}
	var said []string
	kept, err := contentsHosts(hosts, true, func(format string, args ...any) {
		said = append(said, format)
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 || kept[0].Name != "browser" {
		t.Fatalf("kept %+v, want only the host that can be asked a question", kept)
	}
	if len(said) != 1 || !strings.Contains(said[0], "skipping") {
		t.Errorf("said %q, want the run to say which host it left out", said)
	}
}

func TestAContentsRunWithNoHostThatCanBeAskedIsRefused(t *testing.T) {
	// The state the French Integration re-read was actually in: the two boxes
	// that can carry a prompt were under somebody else's load and the one left
	// was the card in the next room. It read the three pages in forty-four
	// seconds and gained nothing, which is worse than reading nothing, because
	// the readings went on to disk over the ones already there.
	hosts := []ocr.Host{{Name: "reader", Reader: "local-ocr"}}
	if _, err := contentsHosts(hosts, true, silent); err == nil {
		t.Fatal("no error, want a refusal rather than a run that reads with the wrong prompt")
	} else if !strings.Contains(err.Error(), "-contents") {
		t.Errorf("err = %v, want it to name the flag it cannot honour", err)
	}
}

func TestAnOrdinaryRunKeepsEveryHostItWasGiven(t *testing.T) {
	// The reader is the fastest thing in the pool for a page of body text, at
	// thirteen seconds against a rented box's four minutes, so this must cost
	// it nothing anywhere but a contents run.
	hosts := []ocr.Host{
		{Name: "reader", Reader: "local-ocr"},
		{Name: "browser"},
	}
	kept, err := contentsHosts(hosts, false, func(string, ...any) {
		t.Error("an ordinary run said something about the contents prompt")
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != len(hosts) {
		t.Fatalf("kept %d hosts of %d, want every one", len(kept), len(hosts))
	}
}

func TestAContentsRunOnHostsThatCanAllBeAskedSaysNothing(t *testing.T) {
	hosts := []ocr.Host{{Name: "one"}, {Name: "two"}}
	kept, err := contentsHosts(hosts, true, func(string, ...any) {
		t.Error("a run that left nobody out still printed a line about it")
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 2 {
		t.Fatalf("kept %d hosts of 2", len(kept))
	}
}

func TestAReaderNamedAsBlankSpaceIsStillChatgptTool(t *testing.T) {
	// Reader empty means chatgpt-tool, and a route file written by hand can put
	// a space there. Reading that as a reader would drop the one host in the
	// pool that can do the job.
	hosts := []ocr.Host{{Name: "browser", Reader: "  "}}
	kept, err := contentsHosts(hosts, true, silent)
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != 1 {
		t.Fatalf("kept %d hosts of 1, want the host a blank reader names no program for", len(kept))
	}
}
