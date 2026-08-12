package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/solve"
)

const solveReviewUsage = `usage: bourbaki solve review [flags]

Reads solutions the corpus already holds and asks the judges again, without
rewriting a word of them.

Three calls to a solution rather than seven: the candidate-blind reference, the
truth judge, and the reference-blind audit judge. No candidates and no
correction loop, because the question here is not what the answer should have
been, it is whether the answer that was written stands.

This is what a prompt change is worth reading against. A solution verified in
March under a judge that did not ask for a line on every part is a solution
nobody has judged under the rules the corpus now claims to hold to, and the only
honest way to find out is to ask again. It is also the way a solution written on
a cut down model gets rechecked on a full one.

Nothing is written to content/ unless -write is given. Without it this is a
report: what the file says, what the judges say now, and where the two differ.
With it the front matter is brought up to what the judges said, the body is left
exactly as it was, and reviewed: carries the date the judging happened, which is
not the date the solution was written.

flags:
  -lang     which printing, en by default
  -label    one solution, by permanent label
  -section  only this §, by number
  -status   only solutions standing at this status
  -limit    stop after this many
  -depth    how far to follow the cross-references, 2 by default
  -max      the cap on the references, in characters, 40000 by default
  -routes   the route file naming the hosts
  -hosts    only these hosts, by name, comma separated
  -wait     wait this long for a host to come up
  -keep     leave the scratch directories on the hosts
  -write    bring the front matter up to what the judges said
  -json     the rows as JSON
  -dry-run  say what would be judged and ask nothing
`

type solveReviewFlags struct {
	lang    string
	label   string
	section string
	status  string
	limit   int
	depth   int
	max     int
	routes  string
	hosts   string
	wait    time.Duration
	keep    bool
	write   bool
	asJSON  bool
	dry     bool
}

// reviewRow is one solution judged again, and it is the JSON shape too.
type reviewRow struct {
	Label string `json:"label"`
	Was   string `json:"was"`
	Now   string `json:"now"`
	// Judged is false where the reference stopped it, which is a blocked or open
	// exercise rather than one both judges threw out.
	Judged bool   `json:"judged"`
	Truth  string `json:"truth_judge,omitempty"`
	Audit  string `json:"audit_judge,omitempty"`
	Why    string `json:"why,omitempty"`
	Calls  int    `json:"calls"`
}

func (r reviewRow) agreed() bool { return r.Was == r.Now }

func runSolveReview(args []string) error {
	fs := flag.NewFlagSet("solve review", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, solveReviewUsage) }
	var f solveReviewFlags
	fs.StringVar(&f.lang, "lang", "en", "printing to read")
	fs.StringVar(&f.label, "label", "", "one solution")
	fs.StringVar(&f.section, "section", "", "only this §")
	fs.StringVar(&f.status, "status", "", "only solutions at this status")
	fs.IntVar(&f.limit, "limit", 0, "stop after this many")
	fs.IntVar(&f.depth, "depth", 2, "how far to follow the cross-references")
	fs.IntVar(&f.max, "max", 40000, "cap on the references, in characters")
	fs.StringVar(&f.routes, "routes", "", "route file")
	fs.StringVar(&f.hosts, "hosts", "", "only these hosts")
	fs.DurationVar(&f.wait, "wait", 0, "wait this long for a host")
	fs.BoolVar(&f.keep, "keep", false, "leave the scratch on the hosts")
	fs.BoolVar(&f.write, "write", false, "bring the front matter up to what the judges said")
	fs.BoolVar(&f.asJSON, "json", false, "the rows as JSON")
	fs.BoolVar(&f.dry, "dry-run", false, "say what would be judged and ask nothing")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}

	root, err := corpus.Root()
	if err != nil {
		return err
	}
	c, err := solve.Read(root, f.lang)
	if err != nil {
		return err
	}
	store := solve.Store{Root: root}
	work, err := reviewPlan(c, store, f)
	if err != nil {
		return err
	}
	if len(work) == 0 {
		fmt.Println("no solution to judge again")
		return nil
	}
	if f.dry {
		fmt.Printf("%d solutions, asking nothing\n\n", len(work))
		for _, label := range work {
			fmt.Println(label)
		}
		fmt.Printf("\ncalls    up to %d, three to a solution\n", len(work)*3)
		return nil
	}

	ctx, cancel := signalContext()
	defer cancel()
	start := time.Now()
	logf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "["+time.Since(start).Round(time.Second).String()+"] "+format+"\n", args...)
	}
	hosts, err := ocrHostsNow(ctx, f.routes, f.hosts, f.wait, logf)
	if err != nil {
		return err
	}
	if len(hosts) == 0 {
		return errors.New("no host is up, run bourbaki fleet up")
	}
	fmt.Printf("%d solutions over %s\n", len(work), strings.Join(hostNames(hosts), ", "))

	o := solve.Options{Depth: f.depth, MaxChars: f.max}
	var mu sync.Mutex
	var rows []reviewRow
	counts := overHosts(ctx, hosts, work, func(ctx context.Context, host ocr.Host, label string) (string, error) {
		row, err := reviewOne(ctx, c, store, root, host, o, f, label, logf)
		if err != nil {
			return "", err
		}
		mu.Lock()
		rows = append(rows, row)
		mu.Unlock()
		if row.agreed() {
			return "stood", nil
		}
		return "changed", nil
	}, logf)

	sortRows(rows)
	if f.asJSON {
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "  ")
		if err := e.Encode(rows); err != nil {
			return err
		}
		return ctx.Err()
	}
	printReview(rows)
	printCounts(counts, "stood", "changed")
	return ctx.Err()
}

// reviewPlan is the solutions this run will judge again.
//
// Only what has a file. An exercise nobody has attempted has nothing to judge,
// and the run that would attempt it is bourbaki solve run.
func reviewPlan(c *solve.Corpus, store solve.Store, f solveReviewFlags) ([]string, error) {
	all := c.Exercises()
	inOrder(all)
	if f.label != "" {
		found := false
		for _, label := range all {
			if label == f.label {
				found = true
			}
		}
		if !found {
			return nil, fmt.Errorf("%s: the %s printing has no such exercise", f.label, f.lang)
		}
		all = []string{f.label}
	}
	var out []string
	for _, label := range all {
		if f.section != "" && !inSection(label, f.section) {
			continue
		}
		sol, have, err := store.Load(f.lang, label)
		if err != nil {
			return nil, err
		}
		if !have {
			continue
		}
		if f.status != "" && sol.Meta.Status != f.status {
			continue
		}
		out = append(out, label)
		if f.limit > 0 && len(out) == f.limit {
			break
		}
	}
	return out, nil
}

// reviewOne judges one solution again and says what came of it.
func reviewOne(ctx context.Context, c *solve.Corpus, store solve.Store, root string,
	host ocr.Host, o solve.Options, f solveReviewFlags, label string,
	logf func(string, ...any)) (reviewRow, error) {
	sol, have, err := store.Load(f.lang, label)
	if err != nil {
		return reviewRow{}, err
	}
	if !have {
		return reviewRow{}, fmt.Errorf("%s: there is no solution to judge", label)
	}
	cx, err := c.Build(label, o)
	if err != nil {
		return reviewRow{}, err
	}
	started := time.Now()
	engine := solve.Engine{
		Ask:     fleetAsker{host: host, keep: f.keep},
		Archive: solveArchive(root, "review", f.lang, label),
		Logf:    logf,
	}
	rv, err := engine.Review(ctx, cx, sol.Body)
	if err != nil {
		return reviewRow{}, err
	}
	row := reviewRow{Label: label, Was: sol.Meta.Status, Now: rv.Status,
		Judged: rv.Judged, Calls: len(rv.Calls)}
	if rv.Judged {
		row.Truth, row.Audit = passFail(rv.TruthPassed), passFail(rv.AuditPassed)
		// The reason is the failing judge's, and the truth judge's where both
		// failed. A row that quoted the judge that passed would read as though the
		// solution had been faulted for what nobody objected to.
		switch {
		case !rv.TruthPassed:
			row.Why = strings.TrimSpace(rv.WhyTruth)
		case !rv.AuditPassed:
			row.Why = strings.TrimSpace(rv.WhyAudit)
		}
	}
	if err := writeReviewLog(root, f.lang, label, sol, rv); err != nil {
		return reviewRow{}, err
	}
	if f.write && !row.agreed() {
		// The body is left exactly as it was. This command judges, and a command
		// that quietly rewrote what it was judging would leave nobody able to say
		// which text the verdict was about.
		sol.Meta.Status = rv.Status
		sol.Meta.Parts = rv.Parts
		if rv.Judged {
			sol.Meta.TruthJudge, sol.Meta.AuditJudge = passFail(rv.TruthPassed), passFail(rv.AuditPassed)
		}
		sol.Meta.Reviewed = time.Now().UTC().Format(time.RFC3339)
		if err := store.Save(sol); err != nil {
			return reviewRow{}, err
		}
	}
	logf("%s on %s: was %s, now %s, %d calls in %s", label, host.Name,
		row.Was, row.Now, row.Calls, time.Since(started).Round(time.Second))
	return row, nil
}

func passFail(ok bool) string {
	if ok {
		return "pass"
	}
	return "fail"
}

func sortRows(rows []reviewRow) {
	labels := make([]string, len(rows))
	for i, r := range rows {
		labels[i] = r.Label
	}
	inOrder(labels)
	at := map[string]int{}
	for i, label := range labels {
		at[label] = i
	}
	out := make([]reviewRow, len(rows))
	for _, r := range rows {
		out[at[r.Label]] = r
	}
	copy(rows, out)
}

func printReview(rows []reviewRow) {
	fmt.Printf("\n%-24s %-12s %-12s %-6s %-6s %s\n",
		"exercise", "was", "now", "truth", "audit", "")
	for _, r := range rows {
		mark := ""
		if !r.agreed() {
			mark = "changed"
		}
		fmt.Printf("%-24s %-12s %-12s %-6s %-6s %s\n",
			r.Label, r.Was, r.Now, r.Truth, r.Audit, mark)
		if !r.agreed() && r.Why != "" {
			fmt.Printf("%-24s %s\n", "", r.Why)
		}
	}
}

// writeReviewLog keeps the whole of both judgements beside the archive.
//
// The table says a verdict changed. This is the only place that says why, in
// the judges' own words and unsummarised, which is what a person needs when the
// question is whether the new verdict or the old one is the mistake.
func writeReviewLog(root, lang, label string, sol solve.Solution, rv solve.Review) error {
	dir := filepath.Join(root, "work", "review", lang, label)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s judged again\n\n", label)
	fmt.Fprintf(&b, "the file says %s, the judges now say %s, over %d calls\n\n",
		sol.Meta.Status, rv.Status, len(rv.Calls))
	if !rv.Judged {
		fmt.Fprintf(&b, "The judges were not asked. The reference reading stopped it.\n\n")
	}
	for _, call := range rv.Calls {
		fmt.Fprintf(&b, "%-16s %7d %9d %9d %8s  %s\n", call.Stage, call.Attempt,
			call.Question, call.Answer, call.Elapsed.Round(time.Second), call.Model)
	}
	if s := strings.TrimSpace(rv.Reference); s != "" {
		fmt.Fprintf(&b, "\n## The reference reading\n\n%s\n", s)
	}
	if s := strings.TrimSpace(rv.Truth); s != "" {
		fmt.Fprintf(&b, "\n## The truth judge\n\n%s\n", s)
	}
	if s := strings.TrimSpace(rv.Audit); s != "" {
		fmt.Fprintf(&b, "\n## The audit judge\n\n%s\n", s)
	}
	return os.WriteFile(filepath.Join(dir, "review.md"), []byte(b.String()), 0o644)
}
