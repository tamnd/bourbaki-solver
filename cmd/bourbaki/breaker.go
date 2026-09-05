package main

import (
	"sync"
	"time"
)

// breaker takes a host that has stopped answering out of the run.
//
// A section of twenty six chunks cost 377 asks. All twenty six were answered by
// one host inside nineteen minutes, and thirty five minutes after that the file
// still had not been written: the rows that were not the good ones were two
// chunks, over and over, all to one host and all refused. That host was not
// broken. Nine of its ten accounts were in a rate limit cooldown with between
// forty minutes and an hour and three quarters left, and the tenth was locked by
// a live solve, so it had nothing free to answer with and said so at once. The
// router picked it anyway, because the host list is a preference order and a
// host that refuses is still a host.
//
// The lane already retires itself when the provider never reads the question.
// That is not enough, and the reason is the shape of the run rather than the
// shape of the failure: lanes are started per file, so a host retired while one
// file was being translated is started again for the next one, and the next. Two
// wasted asks a file over a run of many files is where a number like 377 comes
// from. So the breaker is held for the run and not for the file, which is the
// only level at which it can remember anything.
//
// What it costs when it is wrong is one ask every Hold, because a tripped host
// is let back to try once rather than written off. A host whose cooldowns have
// expired answers that probe and is back in the run with its count cleared; one
// whose have not refuses it and is put away again. Cooldowns here run forty
// minutes to an hour and three quarters, so a hold in the tens of minutes probes
// a few times across one and is not worth tuning more finely than that.
//
// The count is of consecutive refusals and any answer clears it, because a host
// that is answering is not the thing this is looking for. It counts only the
// refusals where the question was never read: an answer that came back and was
// refused by the rules is the model being wrong, which is what the chunk's three
// attempts are for, and has nothing to do with whether the host is up.
type breaker struct {
	// After is how many consecutive refusals trip it. Small on purpose: the
	// evidence a host has nothing free is that it said so, immediately, and
	// three of those in a row is not a coincidence worth waiting through.
	After int
	// Hold is how long a tripped host is left out before it is let back to try
	// once.
	Hold time.Duration

	mu      sync.Mutex
	refused map[string]int
	until   map[string]time.Time
}

const (
	// breakAfter is small because the evidence is not statistical. A host with
	// nothing free says so in under a second, so three of those in a row is a
	// state of the host and not a run of bad luck, and waiting for a tenth costs
	// seven more messages against a quota that is the thing being conserved.
	breakAfter = 3
	// breakHold is under the shortest cooldown seen rather than over the longest,
	// because being early costs one probe and being late costs a whole file's
	// worth of lanes going to a host the others could have had.
	breakHold = 20 * time.Minute
)

func newBreaker(after int, hold time.Duration) *breaker {
	return &breaker{After: after, Hold: hold,
		refused: map[string]int{}, until: map[string]time.Time{}}
}

// Answered clears a host's count. Any answer at all is enough: the question is
// whether the host is serving, not how well.
func (b *breaker) Answered(host string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	delete(b.refused, host)
	delete(b.until, host)
}

// Refused counts one refusal and says whether that tripped the breaker, so the
// caller can say so once rather than on every lane.
func (b *breaker) Refused(host string) (tripped bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.refused[host]++
	if b.refused[host] < b.After {
		return false
	}
	// A host already inside its hold refused its probe. It goes away again for
	// another Hold, and the count goes back to the edge so that the next probe
	// is one refusal away rather than After of them.
	b.refused[host] = b.After - 1
	b.until[host] = time.Now().Add(b.Hold)
	return true
}

// Out says a host is not to be asked right now. A host inside its hold whose
// hold has run out is let through, which is the probe.
func (b *breaker) Out(host string) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	until, ok := b.until[host]
	return ok && time.Now().Before(until)
}

// Live is the hosts that are worth starting lanes on, in the order they were
// given. An empty answer means every host is inside its hold, and the caller has
// to say that rather than start nothing and report a section nobody asked for.
func (b *breaker) Live(hosts []string) []string {
	var out []string
	for _, h := range hosts {
		if !b.Out(h) {
			out = append(out, h)
		}
	}
	return out
}
