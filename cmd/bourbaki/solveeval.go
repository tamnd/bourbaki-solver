package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/tamnd/bourbaki-solver/benchmark"
	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/solve"
)

const solveEvalUsage = `usage: bourbaki solve eval [flags]

Puts answers a person has already ruled on to the judges, and counts how often
they agree.

This is the only thing that says what a verdict in this corpus is worth. The
pipeline decides for itself whether a solution is right, so every "verified" in
content/solutions is the judges marking their own work, and the number that
tells you whether to believe them is how often they accepted an answer somebody
had already found wrong.

The two errors are not the same error. A wrong answer accepted goes into a book
and is read by somebody who trusts it, and spec 07 §6 puts the target under 5
per cent. A right answer rejected costs a rerun, and up to 30 per cent of that
is tolerated, because a cautious verifier is the right failure direction here.

Three calls to a case, the same three bourbaki solve review makes: the
candidate-blind reference, the truth judge and the reference-blind audit judge.
Nothing is written to content/. The set is labels and verdicts and lives in the
solver repository; the answers themselves are in the corpus, under benchmark/.

flags:
  -lang     which printing, en by default
  -set      a set file to read instead of the one built into this binary
  -label    only cases on this exercise, by permanent label
  -limit    stop after this many cases
  -depth    how far to follow the cross-references, 2 by default
  -max      the cap on the references, in characters, 40000 by default
  -ask      the most that goes in one question, 32000 characters by default
  -routes   the route file naming the hosts
  -hosts    only these hosts, by name, comma separated
  -wait     wait this long for a host to come up
  -keep     leave the scratch directories on the hosts
  -write    put the run in reports/eval.json, which the scorecard reads
  -json     the outcomes as JSON
  -dry-run  say what would be judged and ask nothing
`

type solveEvalFlags struct {
	lang   string
	set    string
	label  string
	limit  int
	depth  int
	max    int
	ask    int
	routes string
	hosts  string
	wait   time.Duration
	keep   bool
	write  bool
	asJSON bool
	dry    bool
}

func runSolveEval(args []string) error {
	fs := flag.NewFlagSet("solve eval", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, solveEvalUsage) }
	var f solveEvalFlags
	fs.StringVar(&f.lang, "lang", "en", "printing to read")
	fs.StringVar(&f.set, "set", "", "a set file to read instead of the built-in one")
	fs.StringVar(&f.label, "label", "", "only cases on this exercise")
	fs.IntVar(&f.limit, "limit", 0, "stop after this many cases")
	fs.IntVar(&f.depth, "depth", 2, "how far to follow the cross-references")
	fs.IntVar(&f.max, "max", 40000, "cap on the references, in characters")
	fs.IntVar(&f.ask, "ask", 32000, "the most that goes in one question, in characters")
	fs.StringVar(&f.routes, "routes", "", "route file")
	fs.StringVar(&f.hosts, "hosts", "", "only these hosts")
	fs.DurationVar(&f.wait, "wait", 0, "wait this long for a host")
	fs.BoolVar(&f.keep, "keep", false, "leave the scratch on the hosts")
	fs.BoolVar(&f.write, "write", false, "put the run in reports/eval.json")
	fs.BoolVar(&f.asJSON, "json", false, "the outcomes as JSON")
	fs.BoolVar(&f.dry, "dry-run", false, "say what would be judged and ask nothing")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}

	root, err := corpus.Root()
	if err != nil {
		return err
	}
	set, where, err := evalSet(f.set)
	if err != nil {
		return err
	}
	work := evalPlan(set, f)
	if len(work) == 0 {
		fmt.Printf("%s holds no case to judge\n", where)
		return nil
	}
	// Every answer is read off disk before a host is asked for anything. A set
	// that names a file the corpus does not hold is a set that will measure a
	// smaller thing than it says it does, and finding that out an hour in, one
	// case at a time, is finding it out too late.
	bodies, missing, err := evalBodies(work, root, f.lang)
	if err != nil {
		return err
	}
	if len(missing) > 0 {
		return fmt.Errorf("%s names %d answers the corpus does not hold, the first is %s",
			where, len(missing), missing[0])
	}

	c, err := solve.Read(root, f.lang)
	if err != nil {
		return err
	}
	accept, reject := work.Counts()
	if f.dry {
		fmt.Printf("%d cases from %s, asking nothing\n\n", len(work), where)
		for _, c := range work {
			fmt.Printf("%-32s %-6s %d chars\n", c.Name(), c.Expect, len(bodies[c.Name()]))
		}
		fmt.Printf("\nread     %d to accept, %d to reject\n", accept, reject)
		fmt.Printf("calls    up to %d, three to a case\n", len(work)*3)
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
	fmt.Printf("%d cases from %s over %s\n", len(work), where, strings.Join(hostNames(hosts), ", "))

	o := solve.Options{Depth: f.depth, MaxChars: f.max}
	names := make([]string, len(work))
	byName := map[string]benchmark.Case{}
	for i, c := range work {
		names[i], byName[c.Name()] = c.Name(), c
	}
	var mu sync.Mutex
	got := map[string]benchmark.Outcome{}
	counts := overHosts(ctx, hosts, names, func(ctx context.Context, host ocr.Host, name string) (string, error) {
		out, err := evalOne(ctx, c, root, host, o, f, byName[name], bodies[name], logf)
		if err != nil {
			return "", err
		}
		mu.Lock()
		got[name] = out
		mu.Unlock()
		if out.Agreed() {
			return "agreed", nil
		}
		return "differed", nil
	}, logf)

	var outcomes []benchmark.Outcome
	var score benchmark.Score
	for _, c := range work {
		out, ran := got[c.Name()]
		if !ran {
			continue
		}
		outcomes = append(outcomes, out)
		score.Add(out)
	}
	if f.write {
		run := benchmark.Run{Ran: time.Now().UTC().Format(time.RFC3339), Set: where,
			Outcomes: outcomes, Score: score}
		if err := run.Save(root); err != nil {
			return err
		}
	}
	if f.asJSON {
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "  ")
		if err := e.Encode(benchmark.Run{Set: where, Outcomes: outcomes, Score: score,
			Rates: evalRates(score)}); err != nil {
			return err
		}
		return ctx.Err()
	}
	printEval(outcomes, score)
	printCounts(counts, "agreed", "differed")
	return ctx.Err()
}

// evalSet reads the set, from a file if one was named and from the binary
// otherwise, and says where it came from.
func evalSet(path string) (benchmark.Set, string, error) {
	if path != "" {
		set, err := benchmark.LoadFile(path)
		return set, path, err
	}
	set, err := benchmark.Load()
	return set, "the built-in set", err
}

// evalPlan is the cases this run will put.
func evalPlan(set benchmark.Set, f solveEvalFlags) benchmark.Set {
	var out benchmark.Set
	for _, c := range set {
		if f.label != "" && c.Label != f.label {
			continue
		}
		out = append(out, c)
		if f.limit > 0 && len(out) == f.limit {
			break
		}
	}
	return out
}

// evalBodies reads every answer, and names the ones that are not there.
func evalBodies(set benchmark.Set, root, lang string) (map[string]string, []string, error) {
	bodies, missing := map[string]string{}, []string(nil)
	for _, c := range set {
		path := c.Body(root, lang)
		raw, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			missing = append(missing, path)
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		if strings.TrimSpace(string(raw)) == "" {
			missing = append(missing, path)
			continue
		}
		bodies[c.Name()] = string(raw)
	}
	return bodies, missing, nil
}

// evalOne puts one case to the judges.
func evalOne(ctx context.Context, c *solve.Corpus, root string, host ocr.Host,
	o solve.Options, f solveEvalFlags, this benchmark.Case, body string,
	logf func(string, ...any)) (benchmark.Outcome, error) {
	cx, err := c.Build(this.Label, o)
	if err != nil {
		return benchmark.Outcome{}, err
	}
	started := time.Now()
	engine := solve.Engine{
		Ask:   fleetAsker{host: host, keep: f.keep},
		Limit: f.ask,
		// Filed under the case and not under the exercise, because two answers to
		// one exercise are two runs and the second would otherwise overwrite the
		// evidence of the first.
		Archive: solveArchive(root, "eval", f.lang, this.Label+"."+this.Variant),
		Logf:    logf,
	}
	rv, err := engine.Review(ctx, cx, body)
	if err != nil {
		return benchmark.Outcome{}, err
	}
	out := benchmark.Outcome{Case: this, Status: rv.Status, Judged: rv.Judged,
		Truth: rv.TruthPassed, Audit: rv.AuditPassed, Calls: len(rv.Calls)}
	if rv.Judged {
		switch {
		case !rv.TruthPassed:
			out.Why = rv.WhyTruth
		case !rv.AuditPassed:
			out.Why = rv.WhyAudit
		}
	}
	mark := "agreed"
	if !out.Agreed() {
		mark = "DIFFERED"
	}
	logf("%s on %s: read as %s, judged %s, %s, %d calls in %s", this.Name(), host.Name,
		this.Expect, rv.Status, mark, len(rv.Calls), time.Since(started).Round(time.Second))
	return out, nil
}

func printEval(outcomes []benchmark.Outcome, score benchmark.Score) {
	fmt.Printf("\n%-32s %-7s %-12s %-6s %-6s %s\n",
		"case", "read", "judged", "truth", "audit", "")
	for _, o := range outcomes {
		mark := ""
		if !o.Agreed() {
			mark = "differed"
		}
		fmt.Printf("%-32s %-7s %-12s %-6s %-6s %s\n", o.Name(), o.Expect, o.Status,
			passFail(o.Truth), passFail(o.Audit), mark)
		if !o.Agreed() && o.Why != "" {
			fmt.Printf("%-32s %s\n", "", o.Why)
		}
	}
	fmt.Println()
	fmt.Println(evalLine("false accept", score.FalseAccepts, score.Rejects,
		score.FalseAcceptRate(), benchmark.FalseAcceptTarget,
		"wrong answers the judges let through"))
	fmt.Println(evalLine("false reject", score.FalseRejects, score.Accepts,
		score.FalseRejectRate(), benchmark.FalseRejectTarget,
		"right answers the judges threw out"))
	if score.Unjudged > 0 {
		fmt.Printf("unjudged      %d cases stopped at the reference, and are counted in "+
			"neither rate\n", score.Unjudged)
	}
}

// evalLine is one rate, said in words, with what it was measured against.
func evalLine(name string, n, of int, rate, target float64, what string) string {
	if of == 0 {
		return fmt.Sprintf("%-13s not measured, the set held no case to get wrong that way", name)
	}
	verdict := "over the"
	if rate <= target {
		verdict = "inside the"
	}
	return fmt.Sprintf("%-13s %d of %d, %.1f %%, %s %.0f %% this is held to, %s",
		name, n, of, rate*100, verdict, target*100, what)
}

func evalRates(s benchmark.Score) map[string]float64 {
	return map[string]float64{
		"false_accept": s.FalseAcceptRate(),
		"false_reject": s.FalseRejectRate(),
	}
}
