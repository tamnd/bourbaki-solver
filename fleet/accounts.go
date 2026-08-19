package fleet

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Accounts is the ban board of one host, counted and nothing else.
//
// A browser route reads pages through a signed in ChatGPT profile, and a
// profile that has answered enough of them is rate limited and sits out a
// cooldown of eight hours. chatgpt-tool accounts prints one line per profile
// slot with that state on it. What the fleet needs from that table is four
// numbers and one duration: how many slots are really signed in, how many can
// take work this minute, how many are sitting out, and how long until the first
// of them is back.
//
// The table also prints the email address of every account, and none of it is
// kept here. The parser reads the state off each line and drops the rest, so
// nothing downstream can print an address it never received. That is deliberate
// and it is the reason this returns counts rather than rows.
//
// The counting is worth doing because the alternative is what the sweep did for
// two cycles: send fifty pages to two hosts, wait nineteen minutes for a
// composer that never comes, and get back "the model is out of turns for now".
// The board says the same thing in one ssh round trip, and it says when to come
// back, which the failed run does not.
type Accounts struct {
	Host string
	// Verified is the slots that hold a real signed in session. The rest of the
	// thirty odd slots on these boxes are empty or half registered and they
	// cannot answer anything.
	Verified int
	// Ready is the verified slots that are neither banned nor held.
	Ready  int
	Banned int
	// Locked is a slot a live process holds. Stale is a lock left by a process
	// that is gone, which is a slot nothing is using and no work can reach.
	// chatgpt-tool accounts --clean-stale is what gets it back.
	Locked int
	Stale  int
	// Soonest is how long until the first banned slot returns. Zero is a slot
	// that is ready now or one whose cooldown has run out to the minute, and
	// less than zero is a host with nothing ready and no time on it to read. It
	// is read off the "left" the host prints rather than off the wall clock time
	// beside it, because these boxes do not keep the same clock as this one:
	// server2 said 09:49 while it was 14:49 here.
	//
	// The difference between zero and less than zero is worth the trouble. The
	// host counts a cooldown down to "0m left" and holds it there for the last
	// seconds of it, and a board that read that as no time at all sent the sweep
	// to sleep on the other host instead: the log has server2 at 2m0s, a sleep
	// of 120s, and then twenty minutes of waiting on server3 while server2 sat
	// there with its cooldown run out.
	Soonest time.Duration
	Err     string
}

// accountsScript finds chatgpt-tool the way the probe finds it, because the
// path differs per host, and prints the table. A host without the tool prints
// nothing rather than failing, so one missing box does not lose the others.
//
// The table comes out on standard error, not standard output: it is drawn by
// rich, which writes to the console it opened. Reading only stdout gets an
// empty string and a fleet that looks like it has no accounts at all, which is
// what the first run of this said about all three boxes.
const accountsScript = `
tool=$(command -v chatgpt-tool || ls "$HOME"/chatgpt-tool/.venv/bin/chatgpt-tool 2>/dev/null || true)
if [ -n "$tool" ]; then "$tool" accounts 2>&1; fi
`

var (
	verifiedMark = "✓"
	leftRE       = regexp.MustCompile(`(?:(\d+)h)?(\d+)m left`)
)

// ParseAccounts reads the table. Anything it does not recognise is a line it
// skips, which is the right failure: a new column on the far side costs a host
// its counts for one run and cannot invent slots that are not there.
func ParseAccounts(host, out string) Accounts {
	board := Accounts{Host: host}
	soonest := time.Duration(-1)
	for line := range strings.SplitSeq(out, "\n") {
		if !strings.Contains(line, verifiedMark) {
			continue
		}
		board.Verified++
		// The state is matched without regard to case, because the host does not
		// keep one: it prints BANNED and LOCKED in capitals and stale-lock in
		// lower case, on the same table. Reading the lock in lower case alone
		// found neither of the two locked slots the fleet really had, counted
		// each of them ready instead, and that one miscount was enough to keep
		// the sweep awake: Wait returns nothing to sleep on the moment a host
		// reads ready, so the loop sent a batch every few minutes to two hosts
		// whose only unbanned slot was held by another process, and got back
		// "the model is out of turns for now" forty five seconds to eight
		// minutes later, eleven cycles and 0 pages in a row.
		state := strings.ToUpper(line)
		switch {
		case strings.Contains(state, "BANNED"):
			board.Banned++
			if left, ok := parseLeft(line); ok && (soonest < 0 || left < soonest) {
				soonest = left
			}
		case strings.Contains(state, "STALE-LOCK"):
			board.Stale++
		case strings.Contains(state, "LOCK"):
			board.Locked++
		default:
			board.Ready++
		}
	}
	if board.Ready == 0 {
		board.Soonest = soonest
	}
	return board
}

func parseLeft(line string) (time.Duration, bool) {
	match := leftRE.FindStringSubmatch(line)
	if match == nil {
		return 0, false
	}
	hours, _ := strconv.Atoi(match[1])
	minutes, _ := strconv.Atoi(match[2])
	return time.Duration(hours)*time.Hour + time.Duration(minutes)*time.Minute, true
}

// Board asks one host for its accounts.
func Board(ctx context.Context, runner Runner, target Target) Accounts {
	out, err := runner.Run(ctx, target.Host, accountsScript)
	name := target.Name
	if name == "" {
		name = target.Host
	}
	if err != nil {
		return Accounts{Host: name, Err: err.Error()}
	}
	board := ParseAccounts(name, out)
	if board.Verified == 0 {
		board.Err = "no signed in profile, so this host can read nothing"
	}
	return board
}

// BoardAll asks every host at once, in the order given.
func BoardAll(ctx context.Context, runner Runner, targets []Target) []Accounts {
	out := make([]Accounts, len(targets))
	var group sync.WaitGroup
	for index, target := range targets {
		group.Go(func() { out[index] = Board(ctx, runner, target) })
	}
	group.Wait()
	return out
}

// Wait is how long to sit before asking again, across the whole fleet. Zero
// means a host can take work now. A fleet where every board failed also returns
// zero, because a run that goes and finds out is better than a sleep decided on
// no information.
func Wait(boards []Accounts) time.Duration {
	soonest := time.Duration(-1)
	for _, board := range boards {
		if board.Err != "" && board.Verified == 0 {
			continue
		}
		if board.Ready > 0 || board.Soonest == 0 {
			return 0
		}
		if board.Soonest > 0 && (soonest < 0 || board.Soonest < soonest) {
			soonest = board.Soonest
		}
	}
	if soonest < 0 {
		return 0
	}
	return soonest
}

// AccountsTable renders the boards the way fleet accounts prints them.
func AccountsTable(boards []Accounts) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%-8s  %8s  %5s  %6s  %6s  %5s  %s\n",
		"host", "verified", "ready", "banned", "locked", "stale", "first one back")
	for _, board := range boards {
		if board.Err != "" && board.Verified == 0 {
			fmt.Fprintf(&out, "%-8s  %s\n", board.Host, board.Err)
			continue
		}
		back := "now"
		switch {
		case board.Ready > 0:
		case board.Soonest < 0:
			back = "not for a while"
		case board.Soonest > 0:
			back = board.Soonest.Round(time.Minute).String()
		default:
			back = "any minute"
		}
		fmt.Fprintf(&out, "%-8s  %8d  %5d  %6d  %6d  %5d  %s\n",
			board.Host, board.Verified, board.Ready, board.Banned, board.Locked, board.Stale, back)
	}
	return out.String()
}
