package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/benchmark"
	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/solve"
)

const solutionsUsage = `usage: bourbaki report solutions [flags]

The scorecard: what the corpus holds an answer to, and how well it believes it.

An exercise with no file is unattempted, and it is counted here rather than left
out. A scorecard that only counts what was run says 100 per cent verified on the
first exercise anybody solves, which is the one number this must never print.

The second table is by §, so a § where the pipeline is doing badly can be found
without reading three hundred files. The third is the parts, which is where a
partial says what it is partial about.

The verifier line is what the last bourbaki solve eval made of the judges, read
out of reports/eval.json and quoted rather than recomputed. Believed means
nothing without it: a corpus that says three hundred exercises are verified and
cannot say how often its judges accept an answer somebody has already found
wrong has reported a number and not a measurement.

flags:
  -lang     which printing, en by default
  -sections the per-§ table as well
  -parts    the parts of every partial, with the reason each failed
  -json     the scorecard as JSON
`

// scorecard is the whole of what this reports, and it is the JSON shape too.
type scorecard struct {
	Lang      string         `json:"lang"`
	Exercises int            `json:"exercises"`
	Status    map[string]int `json:"status"`
	// Answered is everything that is not unattempted, which is what coverage is
	// of. Blocked and open are answers: somebody asked and the corpus now says
	// why there is no proof under it.
	Answered int `json:"answered"`
	Believed int `json:"believed"` // verified, whole
	Parts    struct {
		Total    int `json:"total"`
		Verified int `json:"verified"`
	} `json:"parts"`
	// HandRead is how many solutions a person has read against their exercise,
	// and Disputed is how many of those readings found something the status does
	// not say. Believed counts what the judges decided; these two are the only
	// numbers on this card that somebody checked.
	HandRead int `json:"hand_read"`
	Disputed int `json:"disputed"`
	// Contested is how many solutions stand at verified with a reader's finding
	// against them, and it is the one number on this card that names a file the
	// corpus is currently getting wrong.
	//
	// Disputed already counted these, but it counted them next to hand read,
	// where they read as a note on the reading rather than on the status.
	// Believed went on counting all of them, so the card could say believed 37
	// and disputed 81% of the readings in the same breath and never say that 15
	// of the 37 were solutions somebody had read and objected to. Two of those
	// objections are that the proof assumes the result of the preceding
	// exercise, which is the failure a fluent wrong proof actually looks like
	// and is exactly what the judges cannot see.
	//
	// It is reported and not subtracted. A finding is a reader's note and not a
	// verdict, some of them are about the delimiters rather than the argument,
	// and moving a status is a decision about the mathematics that belongs in
	// the file rather than in a count. The card's job is to stop the number
	// being quoted without it.
	Contested   int            `json:"contested"`
	Corrections map[int]int    `json:"corrections"`
	Models      map[string]int `json:"models"`
	Sections    []sectionScore `json:"sections,omitempty"`
	// Verifier is the last bourbaki solve eval run, or nil where nobody has
	// ever measured what a verdict here is worth. It is quoted and not
	// recomputed: this command reads the corpus and asks no host anything.
	Verifier *benchmark.Run `json:"verifier,omitempty"`
}

type sectionScore struct {
	Section   int            `json:"section"`
	Exercises int            `json:"exercises"`
	Status    map[string]int `json:"status"`
}

func reportSolutionsCmd(args []string) error {
	fs := flag.NewFlagSet("report solutions", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, solutionsUsage) }
	lang := fs.String("lang", "en", "printing to read")
	sections := fs.Bool("sections", false, "the per-§ table as well")
	parts := fs.Bool("parts", false, "the parts of every partial")
	asJSON := fs.Bool("json", false, "the scorecard as JSON")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	c, err := solve.Read(root, *lang)
	if err != nil {
		return err
	}
	store := solve.Store{Root: root}
	held := map[string]solve.Solution{}
	list, err := store.List(*lang)
	if err != nil {
		return err
	}
	for _, sol := range list {
		held[sol.Meta.Label] = sol
	}

	card := score(c.Exercises(), held, *lang)
	last, measured, err := benchmark.LastRun(root)
	if err != nil {
		return err
	}
	if measured {
		card.Verifier = &last
	}
	if *asJSON {
		if !*sections {
			card.Sections = nil
		}
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "  ")
		return e.Encode(card)
	}
	printScorecard(card, *sections)
	if *parts {
		printParts(c.Exercises(), held)
	}
	return nil
}

// score counts one printing.
func score(exercises []string, held map[string]solve.Solution, lang string) scorecard {
	card := scorecard{Lang: lang, Exercises: len(exercises),
		Status: map[string]int{}, Corrections: map[int]int{}, Models: map[string]int{}}
	bySection := map[int]*sectionScore{}
	for _, label := range exercises {
		status := corpus.StatusUnattempted
		sol, have := held[label]
		if have {
			status = sol.Meta.Status
		}
		card.Status[status]++
		if status != corpus.StatusUnattempted {
			card.Answered++
		}
		if status == corpus.StatusVerified {
			card.Believed++
		}
		if have {
			if sol.Meta.HandRead != "" {
				card.HandRead++
				if len(sol.Meta.Found) > 0 {
					card.Disputed++
					if status == corpus.StatusVerified {
						card.Contested++
					}
				}
			}
			card.Corrections[sol.Meta.Corrections]++
			for _, model := range strings.Split(sol.Meta.Model, ", ") {
				if model = strings.TrimSpace(model); model != "" {
					card.Models[model]++
				}
			}
			for _, p := range sol.Meta.Parts {
				card.Parts.Total++
				if p.Status == corpus.StatusVerified {
					card.Parts.Verified++
				}
			}
		}
		r, err := corpus.ParseLabel(label)
		if err != nil {
			continue
		}
		row, ok := bySection[r.Section]
		if !ok {
			row = &sectionScore{Section: r.Section, Status: map[string]int{}}
			bySection[r.Section] = row
		}
		row.Exercises++
		row.Status[status]++
	}
	for _, row := range bySection {
		card.Sections = append(card.Sections, *row)
	}
	sort.Slice(card.Sections, func(i, j int) bool {
		return card.Sections[i].Section < card.Sections[j].Section
	})
	return card
}

func printScorecard(card scorecard, sections bool) {
	fmt.Printf("%d exercises in the %s printing\n\n", card.Exercises, card.Lang)
	for _, status := range corpus.Statuses {
		n := card.Status[status]
		if n == 0 && status != corpus.StatusVerified {
			continue
		}
		fmt.Printf("%-12s %5d  %5.1f %%\n", status, n, percent(n, card.Exercises))
	}
	fmt.Printf("\nanswered     %5d  %5.1f %%\n", card.Answered, percent(card.Answered, card.Exercises))
	fmt.Printf("believed     %5d  %5.1f %%\n", card.Believed, percent(card.Believed, card.Exercises))
	if card.Contested > 0 {
		fmt.Printf("  contested  %5d  %5.1f %% of them a reader has objected to\n",
			card.Contested, percent(card.Contested, card.Believed))
	}
	if card.Parts.Total > 0 {
		fmt.Printf("parts        %5d  %5.1f %% of them verified\n",
			card.Parts.Total, percent(card.Parts.Verified, card.Parts.Total))
	}
	// Under believed, because that is what it qualifies. Believed is the judges
	// on their own work; this is how much of it anybody has checked, and how
	// much of what was checked came back disagreeing.
	fmt.Printf("hand read    %5d  %5.1f %% of what is answered\n",
		card.HandRead, percent(card.HandRead, card.Answered))
	if card.HandRead > 0 {
		fmt.Printf("  disputed   %5d  %5.1f %% of the readings found something the "+
			"status does not say\n", card.Disputed, percent(card.Disputed, card.HandRead))
	}

	if len(card.Corrections) > 0 {
		var rounds []int
		for n := range card.Corrections {
			rounds = append(rounds, n)
		}
		sort.Ints(rounds)
		fmt.Println("\ncorrections")
		for _, n := range rounds {
			fmt.Printf("  %d rounds %5d\n", n, card.Corrections[n])
		}
	}
	printVerifier(card.Verifier)
	if len(card.Models) > 0 {
		fmt.Println("\nanswered by")
		for _, m := range sortedByCount(card.Models) {
			fmt.Printf("  %-24s %5d\n", m, card.Models[m])
		}
	}
	if !sections || len(card.Sections) == 0 {
		return
	}
	fmt.Printf("\n%-4s %6s", "§", "total")
	for _, status := range corpus.Statuses {
		fmt.Printf(" %10s", status)
	}
	fmt.Println()
	for _, row := range card.Sections {
		fmt.Printf("%-4d %6d", row.Section, row.Exercises)
		for _, status := range corpus.Statuses {
			fmt.Printf(" %10d", row.Status[status])
		}
		fmt.Println()
	}
}

// printVerifier is what the last eval run made of the judges.
//
// It is on the scorecard because believed is meaningless without it. A corpus
// that says three hundred exercises are verified, and cannot say how often its
// judges accept an answer somebody has already found wrong, has reported a
// number and not a measurement.
func printVerifier(run *benchmark.Run) {
	fmt.Println()
	if run == nil {
		fmt.Println("verifier     nothing has measured it, run bourbaki solve eval")
		return
	}
	s := run.Score
	// A partial run should not reach here, because solve eval -write refuses to
	// write one, but a file can be put there by hand and the scorecard is the
	// thing that quotes it, so it says so rather than reading it as whole.
	of := ""
	if run.Partial() {
		of = fmt.Sprintf(", a part of a set of %d and not the set", run.Of)
	}
	fmt.Printf("verifier     measured %s on %d cases%s\n", run.Ran, len(run.Outcomes), of)
	fmt.Printf("  false accept %s\n", verifierRate(s.FalseAccepts, s.Rejects,
		s.FalseAcceptRate(), benchmark.FalseAcceptTarget))
	fmt.Printf("  false reject %s\n", verifierRate(s.FalseRejects, s.Accepts,
		s.FalseRejectRate(), benchmark.FalseRejectTarget))
}

func verifierRate(n, of int, rate, target float64) string {
	if of == 0 {
		return "not measured, the set held no case to get wrong that way"
	}
	verdict := "over"
	if rate <= target {
		verdict = "inside"
	}
	return fmt.Sprintf("%d of %d, %.1f %%, %s the %.0f %% it is held to",
		n, of, rate*100, verdict, target*100)
}

// printParts is the partials, part by part, with what the judges said.
//
// This is the table that says whether partial is being used honestly. A run
// where every partial fails its last part is a run whose judges are tiring, not
// a book whose last parts are hard.
func printParts(exercises []string, held map[string]solve.Solution) {
	fmt.Println()
	shown := 0
	for _, label := range exercises {
		sol, have := held[label]
		if !have || len(sol.Meta.Parts) == 0 {
			continue
		}
		if sol.Meta.Status == corpus.StatusVerified {
			continue
		}
		shown++
		fmt.Printf("%s  %s\n", label, sol.Meta.Status)
		for _, p := range sol.Meta.Parts {
			reason := p.Reason
			if reason != "" {
				reason = "  " + reason
			}
			fmt.Printf("  %-3s %-11s%s\n", p.ID, p.Status, reason)
		}
	}
	if shown == 0 {
		fmt.Println("no solution carries a part that did not pass")
	}
}

func sortedByCount(counts map[string]int) []string {
	out := make([]string, 0, len(counts))
	for k := range counts {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if counts[out[i]] != counts[out[j]] {
			return counts[out[i]] > counts[out[j]]
		}
		return out[i] < out[j]
	})
	return out
}
