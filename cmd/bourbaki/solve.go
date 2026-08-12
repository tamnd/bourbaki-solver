package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/solve"
)

func runSolve(args []string) error {
	fs := flag.NewFlagSet("solve", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: bourbaki solve context [-lang en] [-label alg-viii-s1-ex-19] [-json]

Assembles what a model is shown when it is asked to do an exercise, out of the
committed Markdown and the reference graph: the exercise, every earlier
exercise of its §, the whole of its §, the closure of its cross-references to
depth 2 capped at 40 000 characters, and the citations that leave the corpus.

With -label it writes that one context to stdout, which is the thing the model
reads and is worth reading yourself before spending a fleet on it. Without one
it measures every exercise of the printing and prints the distribution, which
is how the caps were chosen.

Nothing here calls a model. It reads the corpus and nothing else.

  -lang    which printing to read, en by default
  -label   one exercise, printed in full
  -depth   how far to follow the cross-references, 2 by default
  -max     the cap on the references, in characters, 40000 by default
  -json    the measurement as JSON rather than a table
`)
	}
	lang := fs.String("lang", "en", "printing to read")
	label := fs.String("label", "", "one exercise, printed in full")
	depth := fs.Int("depth", 2, "how far to follow the cross-references")
	max := fs.Int("max", 40000, "cap on the references, in characters")
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
		fmt.Print(ctx.Render())
		return nil
	}
	return measure(c, o, *asJSON)
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
}

// measure builds every context in the printing and says what came out.
//
// The numbers are the point. 40 000 characters and depth 2 are spec 07 §3.1's,
// written before there was a corpus to try them on, and a cap nobody has
// measured is a guess with a number on it. This says how many exercises reach
// the cap, how much the median one carries, and what the tail looks like.
func measure(c *solve.Corpus, o solve.Options, asJSON bool) error {
	var rows []row
	for _, label := range c.Exercises() {
		ctx, err := c.Build(label, o)
		if err != nil {
			return err
		}
		r := row{Label: label, Chars: ctx.Chars(),
			Siblings: ctx.Count(solve.Sibling), Refs: ctx.Count(solve.Reference),
			Outside: ctx.Count(solve.Outside)}
		for _, p := range ctx.Named {
			switch p.Why {
			case solve.OverCap:
				r.OverCap++
			case solve.SectionOnly:
				r.Coarse++
			}
		}
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

	chars := make([]int, len(rows))
	var totalRefs, totalOutside, totalCoarse, capped, biggest int
	big := rows[0]
	for i, r := range rows {
		chars[i] = r.Chars
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
	if n < 10000 {
		return fmt.Sprintf("%d chars", n)
	}
	return fmt.Sprintf("%.1fk chars", float64(n)/1000)
}
