package fleet

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"
)

// Facts are what one ssh round trip found on a host.
//
// The point of collecting them in one trip is that every one of them decides
// something. Cores and free memory decide how many browser profiles a host can
// run without swapping; the tool path differs per host and has to be discovered
// rather than assumed; serve decides whether the tunnel is worth starting; and
// a host missing rsync or xvfb-run cannot take OCR work at all.
type Facts struct {
	Name       string `json:"name"`
	Hostname   string `json:"hostname"`
	Cores      int    `json:"cores"`
	MemTotalMB int    `json:"mem_total_mb"`
	MemFreeMB  int    `json:"mem_free_mb"`
	DiskFreeMB int    `json:"disk_free_mb"`
	// Tool is the absolute path of chatgpt-tool, which is under /home/tam on
	// server1 and /root on server2 and server3.
	Tool string `json:"tool"`
	// Serving reports whether something holds 127.0.0.1:8077.
	Serving bool `json:"serving"`
	Xvfb    bool `json:"xvfb"`
	Rsync   bool `json:"rsync"`
	Screen  bool `json:"screen"`

	CheckedAt time.Time `json:"checked_at"`
	Err       string    `json:"error,omitempty"`
}

// probeScript is one shell command, not ten, because the round trip to these
// boxes costs more than everything it runs. Every line is key=value so a new
// field on the far side cannot shift the meaning of an old one.
const probeScript = `
echo "host=$(hostname)"
echo "cores=$(nproc)"
echo "mem_total_mb=$(awk '/MemTotal/{print int($2/1024)}' /proc/meminfo)"
echo "mem_free_mb=$(awk '/MemAvailable/{print int($2/1024)}' /proc/meminfo)"
echo "disk_free_mb=$(df -Pm "$HOME" | awk 'NR==2{print $4}')"
echo "tool=$(command -v chatgpt-tool || ls "$HOME"/chatgpt-tool/.venv/bin/chatgpt-tool 2>/dev/null || echo '')"
echo "serve=$(ss -ltn 2>/dev/null | grep -c '127.0.0.1:PORT')"
echo "xvfb=$(command -v xvfb-run || echo '')"
echo "rsync=$(command -v rsync || echo '')"
echo "screen=$(command -v screen || echo '')"
`

// Target is one host to ask.
//
// Name and Host are two different things and conflating them is a bug waiting
// to happen: the route is called server3 in the route file and in every report,
// while the ssh destination is whatever ~/.ssh/config calls it, which is the
// user's business and not this repo's.
type Target struct {
	Name string
	Host string
	Port int
}

// Probe asks one host what it is.
func Probe(ctx context.Context, runner Runner, target Target) Facts {
	facts := Facts{Name: target.Name, CheckedAt: time.Now().UTC()}
	if facts.Name == "" {
		facts.Name = target.Host
	}
	script := strings.ReplaceAll(probeScript, "PORT", strconv.Itoa(target.Port))
	out, err := runner.Run(ctx, target.Host, script)
	if err != nil {
		facts.Err = err.Error()
		return facts
	}
	for line := range strings.SplitSeq(out, "\n") {
		key, value, ok := strings.Cut(strings.TrimSpace(line), "=")
		if !ok {
			continue
		}
		switch key {
		case "host":
			facts.Hostname = value
		case "cores":
			facts.Cores = number(value)
		case "mem_total_mb":
			facts.MemTotalMB = number(value)
		case "mem_free_mb":
			facts.MemFreeMB = number(value)
		case "disk_free_mb":
			facts.DiskFreeMB = number(value)
		case "tool":
			facts.Tool = value
		case "serve":
			facts.Serving = number(value) > 0
		case "xvfb":
			facts.Xvfb = value != ""
		case "rsync":
			facts.Rsync = value != ""
		case "screen":
			facts.Screen = value != ""
		}
	}
	if facts.Tool == "" {
		facts.Err = "chatgpt-tool is not installed"
	}
	return facts
}

func number(value string) int {
	n, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil {
		return 0
	}
	return n
}

// ProbeAll asks every host at once. They are independent boxes and the trip is
// most of the cost. Results come back in the order of the targets, so a report
// reads the same way twice.
func ProbeAll(ctx context.Context, runner Runner, targets []Target) []Facts {
	out := make([]Facts, len(targets))
	var group sync.WaitGroup
	for index, target := range targets {
		group.Go(func() { out[index] = Probe(ctx, runner, target) })
	}
	group.Wait()
	return out
}

// OCRMemoryMB is what one browser profile needs to transcribe a page without
// the box starting to swap. It is a working figure from the fleet as measured,
// not a published number, and fleet bench is what replaces it.
const OCRMemoryMB = 1500

// CanOCR reports whether a host can run chatgpt-tool ocr-batch.
//
// OCR is a headed Chrome under xvfb-run driving the ChatGPT page. A host
// without xvfb-run cannot start it, a host without rsync cannot be given the
// images, and a host with 553 MB free, which is what server1 had on the day
// this was written, will start it and then be killed by the OOM reaper halfway
// through a batch. Reporting that up front is the difference between a host
// marked incapable and a batch that dies at page eleven.
func (f Facts) CanOCR() (bool, string) {
	switch {
	case f.Err != "":
		return false, f.Err
	case f.Tool == "":
		return false, "chatgpt-tool is not installed"
	case !f.Xvfb:
		return false, "no xvfb-run, so a headed Chrome cannot start"
	case !f.Rsync:
		return false, "no rsync, so the page images cannot be sent"
	case f.MemFreeMB < OCRMemoryMB:
		return false, fmt.Sprintf("%d MB free, and one profile needs about %d", f.MemFreeMB, OCRMemoryMB)
	}
	return true, ""
}

// Lanes is how many OCR workers this host can carry: one per profile that fits
// in free memory, never more than half the cores, because each one is a browser
// and a browser is not a single thread.
func (f Facts) Lanes() int {
	if ok, _ := f.CanOCR(); !ok {
		return 0
	}
	byMemory := f.MemFreeMB / OCRMemoryMB
	byCPU := max(1, f.Cores/2)
	return max(1, min(byMemory, byCPU))
}

// Table renders probe results the way fleet probe prints them.
func Table(rows []Facts) string {
	var out strings.Builder
	fmt.Fprintf(&out, "%-8s  %-12s  %5s  %10s  %10s  %6s  %5s  %5s  %s\n",
		"host", "hostname", "cores", "free RAM", "free disk", "serve", "ocr", "lanes", "tool")
	for _, row := range rows {
		if row.Err != "" && row.Hostname == "" {
			fmt.Fprintf(&out, "%-8s  %s\n", row.Name, row.Err)
			continue
		}
		ocr, why := row.CanOCR()
		tool := row.Tool
		if !ocr && why != "" {
			tool = tool + "  (" + why + ")"
		}
		fmt.Fprintf(&out, "%-8s  %-12s  %5d  %7d MB  %7d MB  %6t  %5t  %5d  %s\n",
			row.Name, row.Hostname, row.Cores, row.MemFreeMB, row.DiskFreeMB,
			row.Serving, ocr, row.Lanes(), tool)
	}
	return out.String()
}
