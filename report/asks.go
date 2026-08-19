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

// Reading pages is half the questions this project asks and it was all of the
// accounting. Translating a §, solving an exercise and putting the glossary to
// a model are the other half, and until reports/ask-usage.jsonl there was no
// record of them at all: the archive under work/ holds what came back, so a
// question that was refused left nothing behind, and a run that asked four
// hundred and got three hundred read exactly like a run that asked three
// hundred.
//
// The unit here is one question and not one batch, because that is what these
// stages have. A batch of pages is a batch because the upload cap makes it one;
// a chunk of a § goes over on its own.

// Ask is one question as the log keeps it. It mirrors ocr.Note, which is what
// writes the file, and it is spelled out again here so the report does not pull
// the whole OCR transport in behind it.
type Ask struct {
	When    time.Time `json:"when"`
	Stage   string    `json:"stage"`
	Host    string    `json:"host"`
	Target  string    `json:"target,omitempty"`
	Model   string    `json:"model,omitempty"`
	Chars   int       `json:"chars"`
	Elapsed string    `json:"elapsed"`
	OK      bool      `json:"ok"`
	Reason  string    `json:"reason,omitempty"`
}

// Took is the elapsed time as a duration.
func (a Ask) Took() time.Duration {
	value, err := time.ParseDuration(a.Elapsed)
	if err != nil {
		return 0
	}
	return value
}

// ReadAsks parses the log, on the same terms as ReadUsage: a line that will not
// parse is counted and skipped rather than failing the report.
func ReadAsks(input io.Reader) ([]Ask, int, error) {
	var out []Ask
	var bad int
	scanner := bufio.NewScanner(input)
	// A refusal can carry a page of remote log with it.
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)
	for scanner.Scan() {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		var ask Ask
		if err := json.Unmarshal([]byte(text), &ask); err != nil {
			bad++
			continue
		}
		out = append(out, ask)
	}
	return out, bad, scanner.Err()
}

// Work is what one stage did on one host.
type Work struct {
	Stage    string
	Host     string
	Asks     int
	Answered int
	Chars    int
	Took     time.Duration
	First    time.Time
	Last     time.Time
	// Reasons counts questions by what went wrong, not batches. A question is
	// one question and it fails once.
	Reasons map[string]int
}

// PerAnswer is the wall clock cost of a question that came back. Questions that
// failed are not in the divisor and their time is in the total, which is the
// same arithmetic Host.PerPage does and for the same reason.
func (w Work) PerAnswer() time.Duration {
	if w.Answered == 0 {
		return 0
	}
	return w.Took / time.Duration(w.Answered)
}

// Asked groups the log by stage and host, and totals it.
type Asked struct {
	Rows  []Work
	Total Work
}

// SummariseAsks adds the questions up per stage and host, keeping only the ones
// the filters let through. stage is matched as a prefix, so "translate" takes
// every language and "translate vi" takes one.
func SummariseAsks(asks []Ask, stage string, since time.Time) Asked {
	byKey := map[string]*Work{}
	total := &Work{Stage: "all", Reasons: map[string]int{}}
	for _, ask := range asks {
		if stage != "" && !strings.HasPrefix(ask.Stage, stage) {
			continue
		}
		if !since.IsZero() && ask.When.Before(since) {
			continue
		}
		key := ask.Stage + "\x00" + ask.Host
		row := byKey[key]
		if row == nil {
			row = &Work{Stage: ask.Stage, Host: ask.Host, Reasons: map[string]int{}}
			byKey[key] = row
		}
		for _, into := range []*Work{row, total} {
			into.Asks++
			into.Chars += ask.Chars
			into.Took += ask.Took()
			if ask.OK {
				into.Answered++
			} else {
				into.Reasons[AskReason(ask)]++
			}
			if into.First.IsZero() || ask.When.Before(into.First) {
				into.First = ask.When
			}
			if ask.When.After(into.Last) {
				into.Last = ask.When
			}
		}
	}

	out := Asked{Total: *total}
	for _, row := range byKey {
		out.Rows = append(out.Rows, *row)
	}
	sort.Slice(out.Rows, func(i, j int) bool {
		if out.Rows[i].Stage != out.Rows[j].Stage {
			return out.Rows[i].Stage < out.Rows[j].Stage
		}
		return out.Rows[i].Host < out.Rows[j].Host
	})
	return out
}

// AskReason says why a question went unanswered, in the fleet's own words read
// through the same table the batch reasons use.
//
// The phrases are the same because the failures are: an account out of uploads
// refuses a translation exactly as it refuses a page. A reason that matches
// nothing is kept as it stands rather than filed under "no reason in the log",
// since unlike a batch a question has the transport's error in hand and that
// error is worth reading even when this table has not learned it yet.
func AskReason(ask Ask) string {
	if ask.OK {
		return ""
	}
	lowered := strings.ToLower(ask.Reason)
	for _, mark := range marks {
		if strings.Contains(lowered, mark.phrase) {
			return mark.reason
		}
	}
	if strings.TrimSpace(ask.Reason) == "" {
		return "no reason in the log"
	}
	return firstLine(ask.Reason)
}

// firstLine keeps a reason to something a table can hold. A transport error can
// arrive with the whole remote log behind it.
func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(s)
	if len(s) > 60 {
		s = s[:57] + "..."
	}
	return s
}

// Table renders the summary the way report usage prints it.
func (a Asked) Table() string {
	if a.Total.Asks == 0 {
		return "no questions in the ask log for that stage and window\n"
	}
	width := len("stage")
	for _, row := range a.Rows {
		width = max(width, len(row.Stage))
	}
	var out strings.Builder
	fmt.Fprintf(&out, "%-*s  %-8s  %6s  %8s  %6s  %10s  %10s\n",
		width, "stage", "host", "asks", "answered", "failed", "total", "per answer")
	rows := append(append([]Work{}, a.Rows...), a.Total)
	for _, row := range rows {
		fmt.Fprintf(&out, "%-*s  %-8s  %6d  %8d  %6d  %10s  %10s\n",
			width, row.Stage, row.Host, row.Asks, row.Answered, row.Asks-row.Answered,
			row.Took.Round(time.Second), row.PerAnswer().Round(time.Second))
	}

	if len(a.Total.Reasons) > 0 {
		fmt.Fprintf(&out, "\nwhy questions went unanswered, counted once each\n")
		for _, reason := range sorted(a.Total.Reasons) {
			fmt.Fprintf(&out, "  %-60s %3d\n", reason, a.Total.Reasons[reason])
		}
	}
	if !a.Total.First.IsZero() {
		fmt.Fprintf(&out, "\nfrom %s to %s\n",
			a.Total.First.Local().Format("2 Jan 15:04"), a.Total.Last.Local().Format("2 Jan 15:04"))
	}
	return out.String()
}
