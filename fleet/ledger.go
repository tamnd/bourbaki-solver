package fleet

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// Ledger is what the last few hours of asking a host actually came to.
//
// The account board says what a host has and not what it can do with it, and
// the two came apart badly enough to be worth recording. A board reading 21
// verified profiles with 10 of them ready and idle looks like spare capacity, so
// three more translate lanes went at the six ready profiles on one host and
// wrote nothing at all. The reason was in the run logs and nowhere else: 995
// asks across the fleet had ended "stopped without writing an answer: browser:
// ... ERROR ChatGPT never accepted the prompt", 720 of them on that host and 262
// on another. The session loads, the profile counts as verified and ready, the
// prompt is composed, and the composer never takes it. Each one costs the whole
// deadline before the lane gives up, so a profile in that state is worse than an
// absent one: it looks available, gets leased, and burns twenty minutes.
//
// So the outcome of each ask is written down against the host it went to, and
// the board reads it back. Two failures that turn up beside it are honest and
// are counted apart from it: "All verified slots busy", which is a box that
// really is full, and a usage limit, which is a quota that comes back on its
// own. Neither says the host cannot take a prompt.
//
// It is per host and not per profile, and it has to be. The account table
// prints the email address of every slot and none of it is kept, on purpose;
// see Accounts. So there is no key here to hang a profile on and no honest way
// to say which of the ten ready slots is the one that will not compose. What
// this can say is that asking this host has not been working, which is the fact
// that decides where to send the next page.
//
// Nothing here gates. The board reports it and the run level breaker in the
// translate driver is what actually stops asking a host that has stopped
// answering, because that one is measured inside the run it is protecting and
// this is a record of runs already over. A ledger that refused a host outright
// would also have no way back: the only evidence that a host is composing again
// is an ask that was answered, and refusing to ask means never getting one.
type Ledger struct {
	Written time.Time          `json:"written"`
	Hosts   map[string][]Asked `json:"hosts,omitempty"`

	mu sync.Mutex
}

// Asked is one ask, and how it ended.
type Asked struct {
	At      time.Time `json:"at"`
	Outcome Outcome   `json:"outcome"`
}

// Outcome is what one ask came to, sorted into the kinds that mean different
// things about the host.
type Outcome string

const (
	// Answered is an ask that came back with something in it.
	Answered Outcome = "answered"
	// NoComposer is the one this exists for: the session was there and the
	// prompt was never taken.
	NoComposer Outcome = "no composer"
	// OutOfTurns is a quota, which returns on its own and is already counted
	// by the ban board.
	OutOfTurns Outcome = "out of turns"
	// Busy is every verified slot in use, which is a host doing work rather
	// than a host that cannot.
	Busy Outcome = "busy"
	// Failed is everything else, most of it transport.
	Failed Outcome = "failed"
)

const (
	// LedgerWindow is how far back the board looks. It is shorter than the
	// eight hour cooldown a rate limited profile sits out, so a host cannot be
	// judged on a ban that has since lifted, and long enough to hold a whole
	// pass of a section at twenty minutes an ask.
	LedgerWindow = 2 * time.Hour
	// LedgerDepth is how many asks are kept per host. Enough to cover the
	// window at every lane count the fleet has run at, and small enough that
	// the file stays a thing you can read.
	LedgerDepth = 400
	// enoughAsks is the fewest asks worth drawing a conclusion from. Two
	// failures in a row is a bad minute and every host has those.
	enoughAsks = 5
)

// LedgerPath is where the record lives, beside the fleet state and for the same
// reason: it is discovered fact about a running fleet and not configuration.
func LedgerPath() string {
	if value := strings.TrimSpace(os.Getenv("BOURBAKI_FLEET_ASKS")); value != "" {
		return value
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "asks.json"
	}
	return filepath.Join(home, ".config", "bourbaki", "asks.json")
}

// NewLedger is an empty ledger for one run to fill in.
func NewLedger() *Ledger { return &Ledger{Hosts: map[string][]Asked{}} }

// LoadLedger reads the record. A missing file is not an error: it means nothing
// has asked anything yet in this configuration.
func LoadLedger(path string) (*Ledger, error) {
	raw, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return NewLedger(), nil
	}
	if err != nil {
		return nil, err
	}
	value := NewLedger()
	if err := json.Unmarshal(raw, value); err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	if value.Hosts == nil {
		value.Hosts = map[string][]Asked{}
	}
	return value, nil
}

// Note records one ask. It is called from the lanes, so it locks.
func (l *Ledger) Note(host string, outcome Outcome) {
	if strings.TrimSpace(host) == "" {
		return
	}
	l.mu.Lock()
	defer l.mu.Unlock()
	if l.Hosts == nil {
		l.Hosts = map[string][]Asked{}
	}
	l.Hosts[host] = trim(append(l.Hosts[host], Asked{At: time.Now(), Outcome: outcome}))
}

// Classify sorts an ask's error into the kind of thing it says about the host.
// A nil error is an answer.
//
// It matches on the message because what it is reading is the far end's own
// words, relayed through however many layers wrapped them. The strings are the
// ones the runner logs actually carry, counted rather than guessed at: 995 of
// the first, 802 of the third and 1670 of the fourth in one week of runs.
func Classify(err error) Outcome {
	if err == nil {
		return Answered
	}
	return ClassifyText(err.Error())
}

// ClassifyText is Classify for a caller that carries the far end's words in a
// string rather than in an error, which the translate driver does: a refusal
// there is a list of rules and messages and the transport one is the message.
func ClassifyText(message string) Outcome {
	s := strings.ToLower(message)
	switch {
	case strings.Contains(s, "never accepted the prompt"):
		return NoComposer
	case strings.Contains(s, "usage limit"),
		strings.Contains(s, "rate limit"),
		strings.Contains(s, "too many requests"):
		return OutOfTurns
	case strings.Contains(s, "verified slots busy"):
		return Busy
	default:
		return Failed
	}
}

// Recent is what a host's asks came to inside the window, newest first out of
// the file and counted by kind.
func (l *Ledger) Recent(host string, within time.Duration) (asks int, byKind map[Outcome]int) {
	l.mu.Lock()
	defer l.mu.Unlock()
	byKind = map[Outcome]int{}
	if within <= 0 {
		within = LedgerWindow
	}
	cutoff := time.Now().Add(-within)
	for _, a := range l.Hosts[host] {
		if a.At.Before(cutoff) {
			continue
		}
		asks++
		byKind[a.Outcome]++
	}
	return asks, byKind
}

// Append merges this run's notes into whatever is on disk and writes the whole
// thing back atomically.
//
// It re-reads rather than writing what it loaded, because two runs against the
// same laptop is the ordinary case here and the second one to finish would
// otherwise drop everything the first recorded. Merging by time and trimming to
// the depth gives the same answer whichever order they land in.
func (l *Ledger) Append(path string) error {
	on, err := LoadLedger(path)
	if err != nil {
		return err
	}
	l.mu.Lock()
	for host, notes := range l.Hosts {
		on.Hosts[host] = trim(append(on.Hosts[host], notes...))
	}
	l.mu.Unlock()
	on.Written = time.Now()
	if directory := filepath.Dir(path); directory != "." {
		if err := os.MkdirAll(directory, 0o755); err != nil {
			return err
		}
	}
	raw, err := json.MarshalIndent(on, "", "  ")
	if err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, append(raw, '\n'), 0o600); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// trim puts the notes in time order and keeps the newest LedgerDepth of them.
func trim(notes []Asked) []Asked {
	sort.SliceStable(notes, func(i, j int) bool { return notes[i].At.Before(notes[j].At) })
	if len(notes) > LedgerDepth {
		notes = notes[len(notes)-LedgerDepth:]
	}
	return notes
}

// Apply puts the record onto the boards, so that a table of what each host has
// also says whether asking it has been working.
func (l *Ledger) Apply(boards []Accounts, within time.Duration) []Accounts {
	out := make([]Accounts, len(boards))
	copy(out, boards)
	for i := range out {
		asks, byKind := l.Recent(out[i].Host, within)
		out[i].Asks = asks
		out[i].AnsweredAsks = byKind[Answered]
		out[i].NoComposer = byKind[NoComposer]
	}
	return out
}

// NotTaking says the record is bad enough that the ready count is not to be
// believed. It wants a sample worth drawing on and it wants the composer to be
// the reason, because a host that is merely busy or out of turns is a host
// whose ready count was honest.
func (a Accounts) NotTaking() bool {
	return a.Asks >= enoughAsks && a.NoComposer*2 > a.Asks
}
