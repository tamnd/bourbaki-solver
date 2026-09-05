package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/fleet"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/prompt"
	"github.com/tamnd/bourbaki-solver/quality"
	"github.com/tamnd/bourbaki-solver/solve"
)

const solveRunUsage = `usage: bourbaki solve run [flags]

Runs the pipeline of spec 07 §2 over exercises that have no solution yet, and
writes one file each under content/solutions/<lang>/.

Seven calls at most to an exercise and as few as one: the candidate-blind
reference, three candidates from three angles, the selector, the truth judge and
the reference-blind audit judge, with a bounded correction loop behind them. An
exercise the reference reads as out of corpus or as an exploration stops after
the first call and is filed as blocked or open, which is a fact about the corpus
and belongs in it.

One exercise to a lane. The seven calls of an exercise are in order and cannot
be spread, so the parallelism is across exercises, and a lane that dies costs
the exercise it was on and nothing else.

Every question and every answer is archived under work/solve/, including the
calls that are thrown away. A solution goes into a book and the run that made it
is the only evidence of how.

flags:
  -lang       which printing to solve, en by default
  -label      one exercise, by permanent label
  -book       only this book, by short name, ens for Theory of Sets
  -section    only this §, by number
  -limit      stop after this many exercises
  -status     only exercises standing at this status, unattempted included
  -force      solve exercises that already have a solution file
  -candidates how many candidates to write, 3 by default
  -fixes      how many correction rounds, 2 by default
  -depth      how far to follow the cross-references, 2 by default
  -max        the cap on the references, in characters, 40000 by default
  -ask        the most that goes in one question, 32000 characters by default
  -routes     the route file naming the hosts
  -hosts      only these hosts, by name, comma separated
  -wait       wait this long for a host to come up
  -keep       leave the scratch directories on the hosts
  -dry-run    assemble and plan, and ask nothing
`

type solveRunFlags struct {
	lang       string
	label      string
	book       string
	section    string
	limit      int
	status     string
	force      bool
	candidates int
	fixes      int
	depth      int
	max        int
	ask        int
	routes     string
	hosts      string
	wait       time.Duration
	keep       bool
	dry        bool
}

func runSolveRun(args []string) error {
	fs := flag.NewFlagSet("solve run", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, solveRunUsage) }
	var f solveRunFlags
	fs.StringVar(&f.lang, "lang", "en", "printing to solve")
	fs.StringVar(&f.label, "label", "", "one exercise")
	fs.StringVar(&f.book, "book", "", "only this book")
	fs.StringVar(&f.section, "section", "", "only this §")
	fs.IntVar(&f.limit, "limit", 0, "stop after this many exercises")
	fs.StringVar(&f.status, "status", "", "only solutions at this status")
	fs.BoolVar(&f.force, "force", false, "solve exercises that already have a solution")
	fs.IntVar(&f.candidates, "candidates", 3, "candidates to write")
	fs.IntVar(&f.fixes, "fixes", 2, "correction rounds")
	fs.IntVar(&f.depth, "depth", 2, "how far to follow the cross-references")
	fs.IntVar(&f.max, "max", 40000, "cap on the references, in characters")
	fs.IntVar(&f.ask, "ask", 32000, "the most that goes in one question, in characters")
	fs.StringVar(&f.routes, "routes", "", "route file")
	fs.StringVar(&f.hosts, "hosts", "", "only these hosts")
	fs.DurationVar(&f.wait, "wait", 0, "wait this long for a host")
	fs.BoolVar(&f.keep, "keep", false, "leave the scratch on the hosts")
	fs.BoolVar(&f.dry, "dry-run", false, "assemble and plan, and ask nothing")
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
	work, err := solvePlan(c, store, f)
	if err != nil {
		return err
	}
	if len(work) == 0 {
		fmt.Println("nothing to solve")
		return nil
	}

	ctx, cancel := signalContext()
	defer cancel()
	o := solve.Options{Depth: f.depth, MaxChars: f.max}
	start := time.Now()
	logf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "["+time.Since(start).Round(time.Second).String()+"] "+format+"\n", args...)
	}

	if f.dry {
		return solveDryRun(c, o, f.ask, work)
	}

	hosts, err := askHostsNow(ctx, f.routes, f.hosts, f.wait, logf)
	if err != nil {
		return err
	}
	if len(hosts) == 0 {
		return errors.New("no host is up, run bourbaki fleet up")
	}
	fmt.Printf("%d exercises over %s\n", len(work), strings.Join(hostNames(hosts), ", "))

	counts := overHosts(ctx, hosts, work, func(ctx context.Context, host ocr.Host, label string) (string, error) {
		return solveOne(ctx, c, store, root, host, o, f, label, logf)
	}, logf)
	printCounts(counts)
	return ctx.Err()
}

// solvePlan is the exercises this run will attempt, in the order it will take
// them.
func solvePlan(c *solve.Corpus, store solve.Store, f solveRunFlags) ([]string, error) {
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
	// A book nothing is labelled with is a misspelling, and a misspelling that
	// went through would print "nothing to solve", which is also what a book
	// already solved through prints.
	if f.book != "" {
		found := false
		for _, label := range all {
			if inBook(label, f.book) {
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("%s: the %s printing has no exercises of that book, it has %s",
				f.book, f.lang, strings.Join(booksOf(all), ", "))
		}
	}
	var out []string
	for _, label := range all {
		if f.book != "" && !inBook(label, f.book) {
			continue
		}
		if f.section != "" && !inSection(label, f.section) {
			continue
		}
		sol, have, err := store.Load(f.lang, label)
		if err != nil {
			return nil, err
		}
		// An exercise with no file stands at unattempted, here as in the
		// scorecard. Reading the absence as a status is what makes -status
		// unattempted mean the ordinary run and -status partial mean going back
		// over what was left half done.
		status := corpus.StatusUnattempted
		if have {
			status = sol.Meta.Status
		}
		switch {
		case f.status != "":
			if status != f.status {
				continue
			}
		case have && !f.force:
			continue
		}
		out = append(out, label)
		if f.limit > 0 && len(out) == f.limit {
			break
		}
	}
	return out, nil
}

// inSection says whether a label belongs to the § the flag names.
//
// The flag is written the way the label is, so 1 is § 1 and a1 is the first
// appendix. The two are different sections with the same number, and a run that
// asked for § 1 and got the appendix would be a run whose report names 43
// exercises of a § that has 43 different ones.
// inBook says whether an exercise belongs to the book named, by the short name
// the labels are built on.
//
// The run takes the exercises of every printing it can read, and a corpus of
// six books is 858 of them in one queue, taken a chapter at a time across all
// of them at once. A book is the unit a person works in and the unit a
// milestone is written about, so it is the unit the run has to be able to take.
func inBook(label, book string) bool {
	r, err := corpus.ParseLabel(label)
	if err != nil {
		return false
	}
	return r.Book == book
}

// booksOf is the books the labels are drawn from, in the order they first
// appear, which is what a misspelled -book is answered with.
func booksOf(labels []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, label := range labels {
		r, err := corpus.ParseLabel(label)
		if err != nil || seen[r.Book] {
			continue
		}
		seen[r.Book] = true
		out = append(out, r.Book)
	}
	sort.Strings(out)
	return out
}

func inSection(label, section string) bool {
	r, err := corpus.ParseLabel(label)
	if err != nil {
		return false
	}
	want, appendix := strings.TrimPrefix(section, "s"), false
	if rest, cut := strings.CutPrefix(want, "a"); cut {
		want, appendix = rest, true
	}
	return r.Appendix == appendix && strconv.Itoa(r.Section) == want
}

// inOrder is the order a person reads the exercises in, which is not the order
// the labels sort in. Lexically, exercise 10 of § 1 comes second, and a pilot
// asked for the first nine exercises would get 1, 10, 11, 12, 13, 14, 15, 2, 3.
func inOrder(labels []string) {
	sort.SliceStable(labels, func(i, j int) bool {
		a, errA := corpus.ParseLabel(labels[i])
		b, errB := corpus.ParseLabel(labels[j])
		if errA != nil || errB != nil {
			return labels[i] < labels[j]
		}
		if a.Appendix != b.Appendix {
			return !a.Appendix
		}
		if a.Section != b.Section {
			return a.Section < b.Section
		}
		return a.Number < b.Number
	})
}

// solveDryRun says what would be asked for and asks nothing.
//
// It is not a rehearsal for its own sake. Every context is built, which is the
// half of the run that can fail without a fleet, and the sizes are what a person
// reads before spending an afternoon of somebody's account on a prompt that
// turns out to carry four hundred thousand characters.
func solveDryRun(c *solve.Corpus, o solve.Options, ask int, work []string) error {
	// What the reference call leaves the context, which is the tightest of the
	// five calls to fit and so the one the sent column is measured on.
	room := ask - len(prompt.SolveReference(""))
	fmt.Printf("%d exercises, asking nothing\n\n", len(work))
	fmt.Printf("%-24s %12s %12s  %5s  %s\n", "exercise", "context", "sent", "parts", "calls")
	total, most, trimmed := 0, 0, 0
	for _, label := range work {
		cx, err := c.Build(label, o)
		if err != nil {
			return err
		}
		parts := cx.PartsOf()
		chars := cx.Chars()
		sent := len(cx.RenderWithin(room, ""))
		total += chars
		if chars > most {
			most = chars
		}
		if sent < len(cx.Render()) {
			trimmed++
		}
		fmt.Printf("%-24s %12s %12s  %5d  %d\n", label, kb(chars), kb(sent), len(parts), 7)
	}
	fmt.Printf("\ncontext  mean %s, largest %s, %s in all\n",
		kb(total/len(work)), kb(most), kb(total))
	fmt.Printf("sent     %d of %d trimmed to fit the %s a question leaves them\n",
		trimmed, len(work), kb(room))
	fmt.Printf("calls    up to %d, and as few as %d if every one stops at the reference\n",
		len(work)*7, len(work))
	return nil
}

// solveOne runs the pipeline over one exercise and writes what came of it.
func solveOne(ctx context.Context, c *solve.Corpus, store solve.Store, root string,
	host ocr.Host, o solve.Options, f solveRunFlags, label string,
	logf func(string, ...any)) (string, error) {
	cx, err := c.Build(label, o)
	if err != nil {
		return "", err
	}
	started := time.Now()
	engine := solve.Engine{
		Ask:         fleetAsker{host: host, keep: f.keep, note: noteAsks(root, logf)},
		Candidates:  f.candidates,
		Corrections: f.fixes,
		Limit:       f.ask,
		Archive:     solveArchive(root, "solve", f.lang, label),
		Logf:        logf,
	}
	result, err := engine.Solve(ctx, cx)
	if err != nil {
		return "", err
	}
	if err := store.Save(result.Solution); err != nil {
		return "", err
	}
	if err := writeRunLog(root, f.lang, label, result); err != nil {
		return "", err
	}
	note := ""
	for _, call := range result.Calls {
		if quality.SmallModel(call.Model) {
			note = ", and the " + call.Stage + " call came back on " + call.Model +
				", which is a cut down model"
			break
		}
	}
	logf("%s on %s: %s after %d calls in %s%s", label, host.Name,
		result.Solution.Meta.Status, len(result.Calls),
		time.Since(started).Round(time.Second), note)
	return result.Solution.Meta.Status, nil
}

// fleetAsker is the engine's one method, over one host.
//
// One host and not the pool. The seven calls of an exercise are one line of
// reasoning and they are billed to one account, and spreading them over three
// accounts to save minutes buys a run whose failures cannot be read: an exercise
// that came back wrong is then wrong on three models at once.
type fleetAsker struct {
	host ocr.Host
	keep bool
	// note records the question in reports/ask-usage.jsonl. Nil is a run that
	// keeps no record, which is what the tests are.
	note func(ocr.Note)
}

func (a fleetAsker) Ask(ctx context.Context, id, question string) (solve.Answer, error) {
	call := ocr.NewAsk(a.host, fleet.SSH{Timeout: 2 * time.Minute}, ocr.Rsync{Timeout: 5 * time.Minute},
		question, "solve-"+strings.ReplaceAll(id, "/", "_"), a.keep)
	answer, err := ocr.Recorded{Asker: call, Stage: "solve", Host: a.host.Name,
		Target: id, Chars: len(question), Note: a.note}.Do(ctx)
	// Both records, because they answer different questions. ask-usage.jsonl is
	// this run's questions in full and is read afterwards; the ask record is the
	// last two hours across every run and is what fleet accounts prints before
	// anybody points more lanes at a host. Solving was in the first and not the
	// second, so a board drawn during a solve run could say a host was taking
	// prompts on the strength of a translate run that had already finished.
	//
	// Written per ask rather than merged at the end. A run that is killed is the
	// ordinary case here, and it is the run whose record is worth the most.
	noteAsk(a.host.Name, err)
	if err != nil {
		return solve.Answer{}, err
	}
	return solve.Answer{Text: answer.Text, Model: answer.Model,
		Conversation: answer.Conversation, Elapsed: answer.Elapsed}, nil
}

// solveArchive keeps both halves of every call under work/, which is not
// committed.
//
// Every call, including the six an exercise throws away. The candidate that
// lost is the evidence that the selector had something to choose between, and a
// judgement that failed is the only place the reason a solution is marked
// unverified is written out in full.
//
// The kind is solve or review, and it is a directory of its own because the two
// name their calls the same. A re-judging filed over the run it re-judged would
// destroy the evidence it was called to weigh.
func solveArchive(root, kind, lang, label string) func(id, question, answer string) error {
	dir := filepath.Join(root, "work", kind, lang, label)
	return func(id, question, answer string) error {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
		// The label is already the directory, so it comes off the file name.
		stem := filepath.Join(dir, strings.TrimPrefix(id, label+"-"))
		if err := os.WriteFile(stem+".ask.md", []byte(question), 0o644); err != nil {
			return err
		}
		return os.WriteFile(stem+".answer.md", []byte(answer), 0o644)
	}
}

// writeRunLog puts the reference and the call list beside the archive.
//
// The reference is the obligations and the falsification checks, and it is the
// thing to read when a judge said no and the reason is not obvious. It is not
// written to content/, because it is not believed and it is not mathematics the
// corpus stands behind; it is working.
func writeRunLog(root, lang, label string, result solve.Result) error {
	dir := filepath.Join(root, "work", "solve", lang, label)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "# %s\n\n", label)
	fmt.Fprintf(&b, "status %s, candidate %d selected, %d calls\n\n",
		result.Solution.Meta.Status, result.Selected, len(result.Calls))
	fmt.Fprintf(&b, "%-16s %7s %9s %9s %8s  %s\n",
		"stage", "attempt", "question", "answer", "elapsed", "model")
	for _, call := range result.Calls {
		fmt.Fprintf(&b, "%-16s %7d %9d %9d %8s  %s\n", call.Stage, call.Attempt,
			call.Question, call.Answer, call.Elapsed.Round(time.Second), call.Model)
	}
	if strings.TrimSpace(result.Reference) != "" {
		fmt.Fprintf(&b, "\n## The reference reading\n\n%s\n", strings.TrimSpace(result.Reference))
	}
	return os.WriteFile(filepath.Join(dir, "run.md"), []byte(b.String()), 0o644)
}
