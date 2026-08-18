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
	// Soonest is how long until the first banned slot returns, and zero when a
	// slot is ready now. It is read off the "left" the host prints rather than
	// off the wall clock time beside it, because these boxes do not keep the
	// same clock as this one: server2 said 09:49 while it was 14:49 here.
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
		switch {
		case strings.Contains(line, "BANNED"):
			board.Banned++
			if left, ok := parseLeft(line); ok && (soonest < 0 || left < soonest) {
				soonest = left
			}
		case strings.Contains(line, "stale-lock"):
			board.Stale++
		case strings.Contains(line, "lock"):
			board.Locked++
		default:
			board.Ready++
		}
	}
	if board.Ready == 0 && soonest > 0 {
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
		if board.Ready > 0 {
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
		if board.Ready == 0 {
			back = "not for a while"
			if board.Soonest > 0 {
				back = board.Soonest.Round(time.Minute).String()
			}
		}
		fmt.Fprintf(&out, "%-8s  %8d  %5d  %6d  %6d  %5d  %s\n",
			board.Host, board.Verified, board.Ready, board.Banned, board.Locked, board.Stale, back)
	}
	return out.String()
}
