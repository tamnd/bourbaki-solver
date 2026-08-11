// Package report reads the run logs back and says what the fleet did.
//
// Every OCR batch appends a line to reports/ocr-usage.jsonl and that file is
// never replaced, so it is the only record of how these boxes behave over a
// week rather than during the last run. The question it has to answer is what
// a page costs and where the failures come from, because the answer decides how
// the remaining volumes are scheduled.
package report

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
)

// Line is one batch as writeOCRReport recorded it.
type Line struct {
	Book  string    `json:"book"`
	When  time.Time `json:"when"`
	Host  string    `json:"host"`
	ID    string    `json:"id"`
	Pages int       `json:"pages"`
	Wrote int       `json:"wrote"`
	// Lanes is how many pages the host was reading at once. Batches from
	// before that was written down have nothing here, which is not the same
	// as one lane and must not be read as it.
	Lanes   int      `json:"lanes,omitempty"`
	Missing []string `json:"missing,omitempty"`
	Elapsed string   `json:"elapsed"`
	Log     string   `json:"log,omitempty"`
}

// Took is the elapsed time as a duration. It is written as a string, "22m8s",
// so a person reading the file can check it against a wall clock.
func (l Line) Took() time.Duration {
	value, err := time.ParseDuration(l.Elapsed)
	if err != nil {
		return 0
	}
	return value
}

// ReadUsage parses the log. A line that will not parse is counted and skipped
// rather than failing the whole report: this file is appended to by every run
// and one bad line should not hide the fifty good ones.
func ReadUsage(input io.Reader) ([]Line, int, error) {
	var out []Line
	var bad int
	scanner := bufio.NewScanner(input)
	// Batch logs are kept whole when something fails and they run long.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var line Line
		if err := json.Unmarshal([]byte(text), &line); err != nil {
			bad++
			continue
		}
		out = append(out, line)
	}
	return out, bad, scanner.Err()
}

// Host is what one box did.
type Host struct {
	Name    string
	Batches int
	Pages   int
	Wrote   int
	Took    time.Duration
	First   time.Time
	Last    time.Time
	// Reasons counts batches by what went wrong on them, not occurrences. A
	// batch that banned six slots had one problem, not six.
	Reasons map[string]int
}

// PerPage is the wall clock cost of a page that came back. Pages that failed
// are not in the divisor because they did not produce anything, but their time
// is in the total, which is the honest way round: a run that fails half its
// pages really does cost twice as much per page as one that does not.
func (h Host) PerPage() time.Duration {
	if h.Wrote == 0 {
		return 0
	}
	return h.Took / time.Duration(h.Wrote)
}

// Summary groups the log by host.
type Summary struct {
	Hosts []Host
	Total Host
}

// Summarise adds the lines up per host, oldest first, keeping only the ones the
// filters let through.
func Summarise(lines []Line, book string, since time.Time) Summary {
	byHost := map[string]*Host{}
	total := &Host{Name: "all", Reasons: map[string]int{}}
	for _, line := range lines {
		if book != "" && line.Book != book {
			continue
		}
		if !since.IsZero() && line.When.Before(since) {
			continue
		}
		host := byHost[line.Host]
		if host == nil {
			host = &Host{Name: line.Host, Reasons: map[string]int{}}
			byHost[line.Host] = host
		}
		for _, into := range []*Host{host, total} {
			into.Batches++
			into.Pages += line.Pages
			into.Wrote += line.Wrote
			into.Took += line.Took()
			if into.First.IsZero() || line.When.Before(into.First) {
				into.First = line.When
			}
			if line.When.After(into.Last) {
				into.Last = line.When
			}
			for _, reason := range Reasons(line) {
				into.Reasons[reason]++
			}
		}
	}

	out := Summary{Total: *total}
	for _, host := range byHost {
		out.Hosts = append(out.Hosts, *host)
	}
	sort.Slice(out.Hosts, func(i, j int) bool { return out.Hosts[i].Name < out.Hosts[j].Name })
	return out
}

// marks are the phrases a failing batch leaves in its log, in the order they
// are worth reporting. They are what the fleet actually says, and they were
// each read off a real run: the upload quota and the signed out slot were both
// silent failures until the tool learned to name them.
var marks = []struct {
	phrase string
	reason string
}{
	{"to upload again", "no uploads left on the account"},
	{"for more uploads", "no uploads left on the account"},
	{"signed out", "the slot is signed out"},
	{"ip-level block", "the address is blocked"},
	{"circuit-breaker", "the pool paused itself"},
	{"nothing came back", "no answer on the page"},
	{"no ocr response", "no answer on the page"},
	{"launch_persistent_context", "the browser would not start"},
	{"rate-limited", "a slot was banned"},
	{"[ban]", "a slot was banned"},
}

// Reasons says what went wrong on a batch, at most once each.
//
// A batch with nothing missing gets nothing, whatever its log says. Slots are
// banned and rotated on runs that finish every page, and counting that as a
// failure would report a healthy fleet as a broken one.
func Reasons(line Line) []string {
	if line.Wrote >= line.Pages {
		return nil
	}
	lowered := strings.ToLower(line.Log)
	var out []string
	seen := map[string]bool{}
	for _, mark := range marks {
		if strings.Contains(lowered, mark.phrase) && !seen[mark.reason] {
			seen[mark.reason] = true
			out = append(out, mark.reason)
		}
	}
	if len(out) == 0 {
		out = append(out, "no reason in the log")
	}
	return out
}

// Table renders the summary the way report usage prints it.
func (s Summary) Table() string {
	if s.Total.Batches == 0 {
		return "no batches in the usage log for that book and window\n"
	}
	var out strings.Builder
	fmt.Fprintf(&out, "%-8s  %8s  %6s  %6s  %6s  %10s  %10s\n",
		"host", "batches", "pages", "wrote", "failed", "total", "per page")
	rows := append(append([]Host{}, s.Hosts...), s.Total)
	for _, host := range rows {
		fmt.Fprintf(&out, "%-8s  %8d  %6d  %6d  %6d  %10s  %10s\n",
			host.Name, host.Batches, host.Pages, host.Wrote, host.Pages-host.Wrote,
			host.Took.Round(time.Second), host.PerPage().Round(time.Second))
	}

	if len(s.Total.Reasons) > 0 {
		fmt.Fprintf(&out, "\nwhy batches lost pages, counted once per batch\n")
		for _, reason := range sorted(s.Total.Reasons) {
			fmt.Fprintf(&out, "  %-34s %3d\n", reason, s.Total.Reasons[reason])
		}
	}
	if !s.Total.First.IsZero() {
		fmt.Fprintf(&out, "\nfrom %s to %s\n",
			s.Total.First.Local().Format("2 Jan 15:04"), s.Total.Last.Local().Format("2 Jan 15:04"))
	}
	return out.String()
}

// sorted puts the commonest reason first, and ties in alphabetical order so the
// same log prints the same way twice.
func sorted(counts map[string]int) []string {
	keys := make([]string, 0, len(counts))
	for key := range counts {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool {
		if counts[keys[i]] != counts[keys[j]] {
			return counts[keys[i]] > counts[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}

// Rate is what one host delivered at one lane count.
//
// The lane count is the whole point of grouping this way. A box reading two
// pages at once is not twice as fast as one reading a single page, and on these
// shared boxes it can easily be slower, so the only way to set the number in
// the route file honestly is to look at what each setting actually delivered.
type Rate struct {
	Host    string
	Lanes   int
	Batches int
	Pages   int
	Wrote   int
	Took    time.Duration
}

// PagesPerHour counts the pages that came back over the wall clock the host
// spent, failures included. It is what a schedule is made of.
func (r Rate) PagesPerHour() float64 {
	if r.Took <= 0 {
		return 0
	}
	return float64(r.Wrote) / r.Took.Hours()
}

// Rates groups the log by host and lane count, best first.
//
// Batches recorded before the lane count was written down come back under
// lanes 0, which is a thing the caller has to say out loud rather than treat
// as a measurement of one lane.
func Rates(lines []Line, book string, since time.Time) []Rate {
	type key struct {
		host  string
		lanes int
	}
	byKey := map[key]*Rate{}
	for _, line := range lines {
		if book != "" && line.Book != book {
			continue
		}
		if !since.IsZero() && line.When.Before(since) {
			continue
		}
		id := key{line.Host, line.Lanes}
		rate := byKey[id]
		if rate == nil {
			rate = &Rate{Host: line.Host, Lanes: line.Lanes}
			byKey[id] = rate
		}
		rate.Batches++
		rate.Pages += line.Pages
		rate.Wrote += line.Wrote
		rate.Took += line.Took()
	}

	out := make([]Rate, 0, len(byKey))
	for _, rate := range byKey {
		out = append(out, *rate)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Host != out[j].Host {
			return out[i].Host < out[j].Host
		}
		return out[i].Lanes < out[j].Lanes
	})
	return out
}

// Best is the lane count that delivered the most pages an hour on this host,
// and how many batches say so. It ignores lanes 0, which is not a measurement
// but a batch from before the lane count was recorded, and it ignores a lane
// count with only one batch behind it, because one batch on a shared box is a
// measurement of what the other tenants were doing.
func Best(rates []Rate, host string, minBatches int) (Rate, bool) {
	var best Rate
	var found bool
	for _, rate := range rates {
		if rate.Host != host || rate.Lanes == 0 || rate.Batches < minBatches || rate.Wrote == 0 {
			continue
		}
		if !found || rate.PagesPerHour() > best.PagesPerHour() {
			best, found = rate, true
		}
	}
	return best, found
}
