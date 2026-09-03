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

	// Kind is what this host reads with, and it decides which of these fields
	// mean anything. On a reader every count above is zero and always will be,
	// because a reader has no account pool: it serves its own weights and reads
	// against them. A board that did not know this printed "no signed in
	// profile, so this host can read nothing" about the host that reads the most
	// pages in the fleet.
	Kind Kind
	// Model is what a reader is serving and Answers is whether its endpoint
	// replied. Those two are a reader's readiness, exactly as the counts are a
	// browser host's.
	Model   string
	Answers bool
	// TimedOut says the host did not answer inside the deadline, which is a
	// different state from a host that answered and has nothing ready. The table
	// used to print the transport error into a row of counts, so a box that was
	// merely slow read the same as a box with every profile banned.
	TimedOut bool

	// Asks, AnsweredAsks and NoComposer are what asking this host has recently
	// come to, filled in from the Ledger and zero where nothing has asked it.
	// The counts above say what the host has; these say whether any of it
	// works. See Ledger for the run that made the distinction necessary.
	Asks         int
	AnsweredAsks int
	NoComposer   int
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

// readerBoardScript asks a reader host the question the account table asks a
// browser host: is there anything here that can take a page right now.
//
// For a browser that is a pool of signed in profiles and a cooldown. For a
// reader it is one thing, whether the model server on the box is up, so the
// script is one line. See readerProbeScript, which asks the same question in
// the same way from the wider probe.
const readerBoardScript = `
echo "reader_answers=$(curl -fsS --max-time 5 -o /dev/null -w 'yes' 'READER_URL/models' 2>/dev/null || echo '')"
`

// Board asks one host what it has ready, and asks the question its kind
// answers.
func Board(ctx context.Context, runner Runner, target Target) Accounts {
	name := target.Name
	if name == "" {
		name = target.Host
	}
	if target.Kind.Or() == Reader {
		return readerBoard(ctx, runner, target, name)
	}
	out, err := runner.Run(ctx, target.Host, accountsScript)
	if err != nil {
		return Accounts{Host: name, Kind: Browser, Err: err.Error(), TimedOut: timedOut(err)}
	}
	board := ParseAccounts(name, out)
	board.Kind = Browser
	if board.Verified == 0 {
		board.Err = "no signed in profile, so this host can read nothing"
	}
	return board
}

// readerBoard is the same question put to a host with no accounts.
func readerBoard(ctx context.Context, runner Runner, target Target, name string) Accounts {
	board := Accounts{Host: name, Kind: Reader, Model: target.ReaderModel}
	if target.ReaderURL == "" {
		board.Err = "no reader url, so there is no model server to ask about"
		return board
	}
	script := strings.ReplaceAll(readerBoardScript, "READER_URL", strings.TrimSuffix(target.ReaderURL, "/"))
	out, err := runner.Run(ctx, target.Host, script)
	if err != nil {
		board.Err = err.Error()
		board.TimedOut = timedOut(err)
		return board
	}
	board.Answers = strings.Contains(out, "reader_answers=yes")
	if !board.Answers {
		board.Err = "its model server does not answer on " + target.ReaderURL
	}
	return board
}

// timedOut says the host never answered, as against answering something this
// could not read.
//
// It matches on the message rather than on a sentinel because the runner is an
// interface and the ssh one wraps whatever the command did into text. Both
// wordings are here: the deadline this run set, and the one the far end's own
// TCP stack reports when a box is up but not listening.
func timedOut(err error) bool {
	if err == nil {
		return false
	}
	s := strings.ToLower(err.Error())
	return strings.Contains(s, "deadline exceeded") ||
		strings.Contains(s, "context canceled") ||
		strings.Contains(s, "timed out") ||
		strings.Contains(s, "timeout")
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
		// A reader that is answering means the fleet has somewhere to send a
		// page now, and a reader has no cooldown to count down to, so it is
		// either zero or it is not part of this sum at all. Without this the
		// sweep would go to sleep for the length of a browser cooldown with the
		// fastest reader in the pool sitting idle.
		if board.Kind == Reader {
			if board.Answers {
				return 0
			}
			continue
		}
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
//
// Three shapes of row, because there are three things a row can be saying. A
// browser host has counts. A reader host has a model and whether it answers,
// and printing zeroes for it in the account columns would be true and
// misleading. A host that never answered has neither, and is not the same state
// as a host that answered and has nothing ready.
//
// The taking column is the one column here that is not read off the host. It is
// answered out of recently asked, from the Ledger, and it is here because the
// six counted columns can all be right about a host that will not take a
// prompt: 21 verified, 10 ready and idle, and 720 asks in a week that never
// reached a composer. Where the record says that, the last column says it
// instead of saying the host is free now, because the last column is what
// somebody deciding where to send a page reads.
func AccountsTable(boards []Accounts) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%-8s  %8s  %5s  %6s  %6s  %5s  %7s  %s\n",
		"host", "verified", "ready", "banned", "locked", "stale", "taking", "first one back")
	for _, board := range boards {
		if board.TimedOut {
			fmt.Fprintf(&out, "%-8s  did not answer inside the deadline, so nothing is known about it\n", board.Host)
			continue
		}
		if board.Kind == Reader {
			what := "serving " + board.Model
			if board.Model == "" {
				what = "a reader"
			}
			state := "not answering"
			if board.Answers {
				state = "answering, and has no accounts to run out of"
			}
			fmt.Fprintf(&out, "%-8s  %s, %s\n", board.Host, what, state)
			continue
		}
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
		// A host that will not compose overrules whatever the counts made of
		// it, including a cooldown: the profiles come back and the composer
		// still does not take the prompt, so saying when the ban lifts is
		// answering a question nobody asked.
		if board.NotTaking() {
			back = fmt.Sprintf("ready on paper, but %d of its last %d asks never reached a composer",
				board.NoComposer, board.Asks)
		}
		taking := "-"
		if board.Asks > 0 {
			taking = fmt.Sprintf("%d/%d", board.AnsweredAsks, board.Asks)
		}
		fmt.Fprintf(&out, "%-8s  %8d  %5d  %6d  %6d  %5d  %7s  %s\n",
			board.Host, board.Verified, board.Ready, board.Banned, board.Locked, board.Stale, taking, back)
	}
	return out.String()
}
