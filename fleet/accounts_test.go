package fleet

import (
	"strings"
	"testing"
	"time"
)

// The tables here are the shape chatgpt-tool accounts prints, with the
// addresses replaced. Nothing in this package keeps an address and no test
// should be the one place that does.

const server3Board = `  base  a@example.invalid                   ~  free [reg-incomplete]
  1     (no creds)                                       free [no-creds]
  1     b@example.invalid                  free [session.local]
  2     c@example.invalid                  free [session.local]
  10    d@example.invalid            ✓  stale-lock pid=223643 (dead)
  11    e@example.invalid            ✓  BANNED (until 12:51, 3h2m left)
  12    f@example.invalid            ✓  BANNED (until 10:11, 0h22m left)
  13    g@example.invalid            ✓  BANNED (until 12:58, 3h8m left)
  17    h@example.invalid                   free
  40    i@example.invalid                ~  free [reg-incomplete]

Total: 33 slots — 4 real verified  0 locked  3 banned  22 free`

// A slot with no tick is a slot with no session on it, and free next to one
// means nothing. Three of the ten lines here read free and none of them can
// answer a page.
func TestOnlyASignedInSlotIsCounted(t *testing.T) {
	board := ParseAccounts("server3", server3Board)
	if board.Verified != 4 {
		t.Errorf("%d verified, want 4", board.Verified)
	}
	if board.Banned != 3 {
		t.Errorf("%d banned, want 3", board.Banned)
	}
	if board.Stale != 1 {
		t.Errorf("%d stale, want 1", board.Stale)
	}
	if board.Ready != 0 {
		t.Errorf("%d ready, want 0", board.Ready)
	}
}

// The wall clock beside the ban belongs to the host and the host is five hours
// behind this one, so the duration is read off the "left" and not off the time.
func TestTheTimeLeftIsReadAndNotTheWallClock(t *testing.T) {
	board := ParseAccounts("server3", server3Board)
	if board.Soonest != 22*time.Minute {
		t.Errorf("first one back in %s, want 22m", board.Soonest)
	}
}

// A host with a slot free now is not waiting for anything, whatever the bans on
// the other slots say.
func TestASlotFreeNowMeansNoWait(t *testing.T) {
	const table = `  10    a@example.invalid   ✓  free [session.local]
  11    b@example.invalid   ✓  BANNED (until 12:51, 3h2m left)`

	board := ParseAccounts("server2", table)
	if board.Ready != 1 || board.Soonest != 0 {
		t.Errorf("%d ready with %s to wait, want 1 and none", board.Ready, board.Soonest)
	}
}

// A lock a live process holds is a slot in use, not a slot rate limited, and
// the two are worth telling apart: one comes back on its own and the other one
// comes back when --clean-stale is run.
func TestALockedSlotIsNotABannedOne(t *testing.T) {
	const table = `  10    a@example.invalid   ✓  locked pid=4471
  11    b@example.invalid   ✓  stale-lock pid=223643 (dead)`

	board := ParseAccounts("server2", table)
	if board.Locked != 1 || board.Stale != 1 || board.Banned != 0 {
		t.Errorf("%d locked, %d stale, %d banned, want 1, 1, 0",
			board.Locked, board.Stale, board.Banned)
	}
}

// A ban under an hour prints no hours at all.
func TestABanWithNoHoursOnIt(t *testing.T) {
	board := ParseAccounts("server2", `  17  a@example.invalid  ✓  BANNED (until 10:33, 43m left)`)
	if board.Soonest != 43*time.Minute {
		t.Errorf("first one back in %s, want 43m", board.Soonest)
	}
}

// A table nothing here recognises counts nothing rather than guessing, and the
// caller sees a host with no signed in profile.
func TestATableWithNothingOnItCountsNothing(t *testing.T) {
	board := ParseAccounts("server1", "Total: 33 slots — 0 real verified  0 locked  0 banned  33 free")
	if board.Verified != 0 || board.Ready != 0 {
		t.Errorf("%d verified and %d ready, want none of either", board.Verified, board.Ready)
	}
}

// The fleet waits for the first host to come back, not for all of them.
func TestTheFleetWaitsForTheFirstHostBack(t *testing.T) {
	boards := []Accounts{
		{Host: "server3", Verified: 11, Banned: 11, Soonest: 3 * time.Hour},
		{Host: "server2", Verified: 10, Banned: 10, Soonest: 43 * time.Minute},
	}
	if got := Wait(boards); got != 43*time.Minute {
		t.Errorf("waiting %s, want 43m", got)
	}
}

// One host ready is the fleet ready.
func TestOneHostReadyIsNoWait(t *testing.T) {
	boards := []Accounts{
		{Host: "server3", Verified: 11, Banned: 11, Soonest: 3 * time.Hour},
		{Host: "server2", Verified: 10, Ready: 2},
	}
	if got := Wait(boards); got != 0 {
		t.Errorf("waiting %s, want none", got)
	}
}

// A fleet nobody could reach is a fleet worth trying, because a sleep decided
// on an ssh failure is a sleep decided on nothing.
func TestAFleetThatCouldNotBeReachedIsNotWaitedOn(t *testing.T) {
	boards := []Accounts{{Host: "server3", Err: "ssh: connect: timed out"}}
	if got := Wait(boards); got != 0 {
		t.Errorf("waiting %s, want none", got)
	}
}

// The table names the host and the counts and nothing else. An address on it
// would end up in a log tail pasted into an issue.
func TestTheTablePrintsNoAddress(t *testing.T) {
	board := ParseAccounts("server3", server3Board)
	out := AccountsTable([]Accounts{board})
	if strings.Contains(out, "@") {
		t.Errorf("an address reached the table:\n%s", out)
	}
	if !strings.Contains(out, "22m") {
		t.Errorf("the wait is not on the table:\n%s", out)
	}
}
