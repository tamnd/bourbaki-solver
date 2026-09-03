package main

import (
	"sync"
	"testing"
	"time"
)

func TestAHostIsOutOnlyAfterItHasRefusedEnoughTimes(t *testing.T) {
	b := newBreaker(3, time.Hour)
	for i := 1; i < 3; i++ {
		if b.Refused("a") {
			t.Fatalf("refusal %d tripped a breaker set to 3", i)
		}
		if b.Out("a") {
			t.Fatalf("the host was taken out after %d of 3 refusals", i)
		}
	}
	if !b.Refused("a") {
		t.Fatal("the third refusal did not trip it")
	}
	if !b.Out("a") {
		t.Fatal("the host is not out after the breaker tripped")
	}
	// The count is per host. This is the whole reason it is a map: the run that
	// cost 377 asks had one host refusing and another answering every chunk it
	// was given, and taking both out would have translated nothing.
	if b.Out("b") {
		t.Fatal("one host refusing took a different host out")
	}
}

func TestAnAnswerClearsTheRefusalsBeforeIt(t *testing.T) {
	b := newBreaker(3, time.Hour)
	b.Refused("a")
	b.Refused("a")
	b.Answered("a")
	// Two more is four refusals in all and still not three in a row.
	if b.Refused("a") || b.Refused("a") {
		t.Fatal("refusals from either side of an answer were counted as consecutive")
	}
	if !b.Refused("a") {
		t.Fatal("three refusals after the answer did not trip it")
	}
}

func TestAHostIsLetBackWhenItsHoldRunsOut(t *testing.T) {
	b := newBreaker(1, time.Millisecond)
	if !b.Refused("a") {
		t.Fatal("a breaker set to 1 did not trip on the first refusal")
	}
	if !b.Out("a") {
		t.Fatal("the host is not out immediately after tripping")
	}
	time.Sleep(5 * time.Millisecond)
	if b.Out("a") {
		t.Fatal("the host is still out after its hold ran out, so nothing ever probes it")
	}
	// And a probe that is refused puts it away again, rather than the host
	// coming back free every time the hold expires and being asked in a loop.
	if !b.Refused("a") {
		t.Fatal("the refused probe did not put the host away again")
	}
	if !b.Out("a") {
		t.Fatal("the host is not out after refusing its probe")
	}
}

func TestAProbeThatIsAnsweredPutsTheHostBackForGood(t *testing.T) {
	b := newBreaker(2, time.Millisecond)
	b.Refused("a")
	b.Refused("a")
	time.Sleep(5 * time.Millisecond)
	b.Answered("a")
	if b.Out("a") {
		t.Fatal("a host that answered its probe is still out")
	}
	// The count went with the hold: one refusal after this must not trip a
	// breaker set to two.
	if b.Refused("a") {
		t.Fatal("the answer left the count where the hold had put it")
	}
}

func TestLiveKeepsThePreferenceOrder(t *testing.T) {
	b := newBreaker(1, time.Hour)
	b.Refused("second")
	got := b.Live([]string{"first", "second", "third"})
	if len(got) != 2 || got[0] != "first" || got[1] != "third" {
		t.Fatalf("live hosts are %v, want the other two in the order they were given", got)
	}
	b.Refused("first")
	b.Refused("third")
	if got := b.Live([]string{"first", "second", "third"}); len(got) != 0 {
		t.Fatalf("live hosts are %v with every host out, want none", got)
	}
}

// The lanes of one host trip it from several goroutines at once, which is the
// case it is built for: nine accounts in cooldown means nine lanes refusing in
// the same second.
func TestTheBreakerHoldsUpUnderTheLanesThatTripIt(t *testing.T) {
	b := newBreaker(3, time.Hour)
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			b.Refused("a")
			b.Out("a")
			b.Live([]string{"a", "b"})
		}()
	}
	wg.Wait()
	if !b.Out("a") {
		t.Fatal("twenty refusals from twenty lanes did not take the host out")
	}
}
