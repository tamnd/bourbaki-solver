package main

import (
	"flag"
	"fmt"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/solve"
)

const solveReadUsage = `usage: bourbaki solve read [-label ID] [flags]

Writes down that a person read a solution against its exercise, and what they
found.

Every status in content/solutions is the pipeline's own verdict on its own
work. When somebody reads one of those files and disagrees, the disagreement is
the most valuable thing anybody has produced about this corpus, and it has
nowhere to go: an edit to the status would say the pipeline decided something it
did not, and a note in a commit message is gone by the next one.

So it goes in the front matter beside the verdict, and the two sit there
disagreeing. hand_read is when it was read, found is what was found, one line to
a finding, and neither of them moves status. A file with a verified status and a
finding under it is not a contradiction the corpus needs to resolve, it is the
corpus saying plainly that its judges passed something a reader did not.

With no -label it lists every reading instead, which is the short table of what
anybody has actually checked.

flags:
  -label  the exercise, by permanent label
  -lang   which printing, en by default
  -found  what the reading found, once per line, repeat for more
  -on     the date of the reading, today by default
  -clear  take the reading off again, for one recorded by mistake
  -replace put these findings in place of the ones there, for a line put down wrong
`

// foundList is what a reading found, one line to a finding.
type foundList []string

func (f *foundList) String() string { return strings.Join(*f, "; ") }

func (f *foundList) Set(s string) error {
	if strings.TrimSpace(s) == "" {
		return fmt.Errorf("an empty finding")
	}
	*f = append(*f, strings.TrimSpace(s))
	return nil
}

func runSolveRead(args []string) error {
	fs := flag.NewFlagSet("solve read", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, solveReadUsage) }
	label := fs.String("label", "", "the exercise, by permanent label")
	lang := fs.String("lang", "en", "printing to read")
	on := fs.String("on", "", "the date of the reading, today by default")
	clear := fs.Bool("clear", false, "take the reading off again")
	replace := fs.Bool("replace", false, "put these findings in place of the ones already there")
	var found foundList
	fs.Var(&found, "found", "what the reading found, once per line")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	store := solve.Store{Root: root}
	if *label == "" {
		if len(found) > 0 || *clear {
			return fmt.Errorf("a reading needs an exercise, name one with -label")
		}
		return listReadings(store, *lang)
	}

	sol, have, err := store.Load(*lang, *label)
	if err != nil {
		return err
	}
	if !have {
		return fmt.Errorf("%s: the %s corpus holds no solution to read", *label, *lang)
	}
	if *clear {
		if sol.Meta.HandRead == "" && len(sol.Meta.Found) == 0 {
			fmt.Printf("%s carries no reading\n", *label)
			return nil
		}
		sol.Meta.HandRead, sol.Meta.Found = "", nil
		if err := store.Save(sol); err != nil {
			return err
		}
		fmt.Printf("%s: the reading is off, status is still %s\n", *label, sol.Meta.Status)
		return nil
	}

	day := *on
	if day == "" {
		day = time.Now().UTC().Format(time.DateOnly)
	}
	if _, err := time.Parse(time.DateOnly, day); err != nil {
		return fmt.Errorf("-on %q is not a date, write it as 2006-01-02", day)
	}
	sol.Meta.HandRead = day
	// Appended and not replaced. A second reader of a file somebody has already
	// read has more to say than the first did, not instead of it, and a flag
	// whose ordinary use silently drops the earlier reading is a flag that will
	// drop one. -replace is there for a line put down wrong.
	for _, line := range found {
		if !slices.Contains(sol.Meta.Found, line) {
			sol.Meta.Found = append(sol.Meta.Found, line)
		}
	}
	if *replace {
		sol.Meta.Found = found
	}
	if err := store.Save(sol); err != nil {
		return err
	}
	fmt.Printf("%s read on %s, status is still %s and this did not touch it\n",
		*label, day, sol.Meta.Status)
	for _, line := range sol.Meta.Found {
		fmt.Printf("  %s\n", line)
	}
	return nil
}

// listReadings is every solution somebody has read, and what they found.
func listReadings(store solve.Store, lang string) error {
	list, err := store.List(lang)
	if err != nil {
		return err
	}
	read, disputed := 0, 0
	for _, sol := range list {
		if sol.Meta.HandRead == "" {
			continue
		}
		read++
		mark := ""
		if len(sol.Meta.Found) > 0 {
			disputed++
			mark = "  found something"
		}
		fmt.Printf("%-20s %-11s read %s%s\n", sol.Meta.Label, sol.Meta.Status,
			sol.Meta.HandRead, mark)
		for _, line := range sol.Meta.Found {
			fmt.Printf("  %s\n", line)
		}
	}
	if read == 0 {
		fmt.Printf("nobody has read any of the %d solutions of the %s printing\n",
			len(list), lang)
		return nil
	}
	fmt.Printf("\n%d of %d solutions read, %d of the readings found something the "+
		"status does not say\n", read, len(list), disputed)
	return nil
}
