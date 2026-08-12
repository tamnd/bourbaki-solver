package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/prompt"
	"github.com/tamnd/bourbaki-solver/solve"
)

func runSolve(args []string) error {
	if len(args) > 0 {
		switch args[0] {
		case "run":
			return runSolveRun(args[1:])
		case "review":
			return runSolveReview(args[1:])
		case "eval":
			return runSolveEval(args[1:])
		}
	}
	fs := flag.NewFlagSet("solve", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: bourbaki solve <context|run|review|eval> [flags]

  context  what a model is shown when it is asked to do an exercise
  run      ask, judge, and write the answers down
  review   judge a solution the corpus already holds, without rewriting it
  eval     put answers a person has ruled on to the judges, and count the misses

usage: bourbaki solve context [-lang en] [-label alg-viii-s1-ex-19] [-json]

Assembles what a model is shown when it is asked to do an exercise, out of the
committed Markdown and the reference graph: the exercise, every earlier
exercise of its §, the whole of its §, the closure of its cross-references to
depth 2 capped at 40 000 characters, and the citations that leave the corpus.

With -label it writes that one context to stdout, which is the thing the model
reads and is worth reading yourself before spending a fleet on it. Without one
it measures every exercise of the printing and prints the distribution, which
is how the caps were chosen.

Nothing here calls a model. It reads the corpus and nothing else. That is
bourbaki solve run, which is where the seven calls of the pipeline are made.

  -lang    which printing to read, en by default
  -label   one exercise, printed in full
  -depth   how far to follow the cross-references, 2 by default
  -max     the cap on the references, in characters, 40000 by default
  -ask     the most that goes in one question, 32000 characters by default,
           which is what -label is rendered inside of and what the measurement
           counts the contexts against
  -json    the measurement as JSON rather than a table
`)
	}
	lang := fs.String("lang", "en", "printing to read")
	label := fs.String("label", "", "one exercise, printed in full")
	depth := fs.Int("depth", 2, "how far to follow the cross-references")
	max := fs.Int("max", 40000, "cap on the references, in characters")
	ask := fs.Int("ask", 32000, "the most that goes in one question, in characters")
	asJSON := fs.Bool("json", false, "write the measurement as JSON")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || pos[0] != "context" {
		fs.Usage()
		os.Exit(2)
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	c, err := solve.Read(root, *lang)
	if err != nil {
		return err
	}
	o := solve.Options{Depth: *depth, MaxChars: *max}

	if *label != "" {
		ctx, err := c.Build(*label, o)
		if err != nil {
			return err
		}
		// What the model is shown, and not what the assembler put together, which
		// on this corpus are different things for 250 of the 317 exercises. The
		// room is the limit less the instructions the reference call wraps the
		// context in, since that call is the one that reads the context alone.
		fmt.Print(ctx.RenderWithin(*ask-len(prompt.SolveReference("")), ""))
		return nil
	}
	return measure(c, o, *ask, *asJSON)
}

// row is one exercise's context, measured.
type row struct {
	Label    string `json:"label"`
	Chars    int    `json:"chars"`
	Siblings int    `json:"siblings"`
	Refs     int    `json:"references"`
	Outside  int    `json:"outside"`
	OverCap  int    `json:"over_cap"`
	Coarse   int    `json:"section_only"`
	// By is the characters each kind of piece contributes, which is what a cap
	// on the whole question is chosen from. Knowing a context is 130 000
	// characters says it will not be sent; knowing that 96 000 of them are
	// references says what to drop.
	By map[string]int `json:"by_kind"`
	// Ask is how long the context is once it has been trimmed to fit a
	// question, which is the number that says whether the exercise can be put
	// to a model at all. Chars is what the corpus holds for it and Ask is what
	// the model gets to read.
	Ask int `json:"ask_chars"`
}

// measure builds every context in the printing and says what came out.
//
// The numbers are the point. 40 000 characters and depth 2 are spec 07 §3.1's,
// written before there was a corpus to try them on, and a cap nobody has
// measured is a guess with a number on it. This says how many exercises reach
// the cap, how much the median one carries, and what the tail looks like.
func measure(c *solve.Corpus, o solve.Options, ask int, asJSON bool) error {
	// The room the reference call leaves the context, which is the tightest of
	// the five calls to fit and so the one worth measuring.
	room := ask - len(prompt.SolveReference(""))
	var rows []row
	for _, label := range c.Exercises() {
		ctx, err := c.Build(label, o)
		if err != nil {
			return err
		}
		r := row{Label: label, Chars: ctx.Chars(), By: map[string]int{},
			Siblings: ctx.Count(solve.Sibling), Refs: ctx.Count(solve.Reference),
			Outside: ctx.Count(solve.Outside)}
		for _, p := range ctx.Pieces {
			r.By[string(p.Kind)] += len(p.Text) + len(p.Raw)
		}
		for _, p := range ctx.Named {
			switch p.Why {
			case solve.OverCap:
				r.OverCap++
			case solve.SectionOnly:
				r.Coarse++
			}
		}
		r.Ask = len(ctx.RenderWithin(room, ""))
		rows = append(rows, r)
	}
	if asJSON {
		e := json.NewEncoder(os.Stdout)
		e.SetIndent("", "  ")
		return e.Encode(rows)
	}
	if len(rows) == 0 {
		fmt.Println("no exercises")
		return nil
	}

	chars, asked := make([]int, len(rows)), make([]int, len(rows))
	by := map[string]int{}
	var totalRefs, totalOutside, totalCoarse, capped, biggest, overAsk, overRoom int
	big := rows[0]
	for i, r := range rows {
		chars[i], asked[i] = r.Chars, r.Ask
		if r.Ask > room {
			overRoom++
		}
		for k, n := range r.By {
			by[k] += n
		}
		if r.Chars > ask {
			overAsk++
		}
		totalRefs += r.Refs
		totalOutside += r.Outside
		totalCoarse += r.Coarse
		if r.OverCap > 0 {
			capped++
		}
		if r.Chars > biggest {
			biggest, big = r.Chars, r
		}
	}
	sort.Ints(chars)
	sort.Ints(asked)
	fmt.Printf("%d exercises\n", len(rows))
	fmt.Printf("context  median %s, mean %s, largest %s (%s)\n",
		kb(chars[len(chars)/2]), kb(mean(chars)), kb(biggest), big.Label)
	fmt.Printf("         p90 %s, smallest %s\n", kb(chars[len(chars)*9/10]), kb(chars[0]))
	fmt.Printf("cited    %d in the corpus and carried, %.1f an exercise\n",
		totalRefs, float64(totalRefs)/float64(len(rows)))
	fmt.Printf("         %d leaving the corpus, %.1f an exercise\n",
		totalOutside, float64(totalOutside)/float64(len(rows)))
	fmt.Printf("         %d named and not carried, a page citation that "+
		"narrowed only to a §\n", totalCoarse)
	fmt.Printf("capped   %d exercises reach the %s limit on references\n", capped, kb(o.MaxChars))
	for _, k := range []solve.Kind{solve.TheExercise, solve.Sibling, solve.TheSection, solve.Reference} {
		fmt.Printf("%-10s%s an exercise on average\n", k, kb(by[string(k)]/len(rows)))
	}
	fmt.Printf("over ask %d of %d contexts are over %s on their own, before the "+
		"instructions\n", overAsk, len(rows), kb(ask))
	fmt.Printf("trimmed  median %s, largest %s, %d still over the %s a question "+
		"leaves them\n", kb(asked[len(asked)/2]), kb(asked[len(asked)-1]), overRoom, kb(room))
	return nil
}

func mean(xs []int) int {
	n := 0
	for _, x := range xs {
		n += x
	}
	return n / len(xs)
}

func kb(n int) string {
	switch {
	case n < 10000:
		return fmt.Sprintf("%d chars", n)
	case n < 1000000:
		return fmt.Sprintf("%.1fk chars", float64(n)/1000)
	}
	return fmt.Sprintf("%.1fM chars", float64(n)/1000000)
}
