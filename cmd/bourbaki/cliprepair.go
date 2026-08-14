package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/clip"
	"github.com/tamnd/bourbaki-solver/corpus"
)

const clipRepairUsage = `usage: bourbaki clip repair -book ID [flags]

flags:
  -book ID       book id from manifests/books.yaml
  -pages LIST    only these pdf pages, comma separated
  -n             say what would change and write nothing
  -v             print the replacement as well as what it replaced

A display that came apart is the one fault the text layer cannot be argued out
of, and the page image is the one reading that does not have it. This takes the
display and leaves everything else alone.

It writes nothing it cannot prove. A repair has to sit between two paragraphs
that both readings write the same way, it has to be no longer in blocks than
what it replaces, and it may not say a word the page did not already say. Where
any of that fails the page is printed with the reason and left as it is.

Read the pages first, with clip cut -whole -pages and clip read.
`

func clipRepair(args []string) error {
	fs := flag.NewFlagSet("clip repair", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	pages := fs.String("pages", "", "only these pdf pages, comma separated")
	dry := fs.Bool("n", false, "say what would change and write nothing")
	verbose := fs.Bool("v", false, "print the replacement as well")
	fs.Usage = func() { fmt.Fprint(os.Stderr, clipRepairUsage) }
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" {
		fs.Usage()
		os.Exit(2)
	}
	root, entry, err := findBook(*book)
	if err != nil {
		return err
	}
	only := map[int]bool{}
	for _, field := range strings.Split(*pages, ",") {
		if field = strings.TrimSpace(field); field == "" {
			continue
		}
		page, err := strconv.Atoi(field)
		if err != nil {
			return fmt.Errorf("-pages: %q is not a page number", field)
		}
		only[page] = true
	}

	// The answers on disk and not the index. A page can be repaired from a clip
	// that was cut in an earlier run under a different query, and the index only
	// remembers the last one; what matters is whether there is a reading of the
	// picture of that page, which is a file with the page number on it.
	answers := clip.AnswersDir(root, entry.ID)
	names, err := filepath.Glob(filepath.Join(answers, "*.md"))
	if err != nil {
		return err
	}
	sort.Strings(names)
	if len(names) == 0 {
		return fmt.Errorf("%s has no clip answers under %s", entry.ID, answers)
	}

	var repaired, refused, untouched int
	for _, name := range names {
		page, err := strconv.Atoi(strings.TrimSuffix(filepath.Base(name), ".md"))
		if err != nil {
			continue // not a page clip, and a line clip has nothing to say here
		}
		if len(only) > 0 && !only[page] {
			continue
		}
		path := corpus.PagePath(root, entry.ID, page)
		file, err := corpus.ReadFile[corpus.PageFrontMatter](path)
		if err != nil {
			return err
		}
		model, err := clip.ReadAnswer(name)
		if err != nil {
			fmt.Printf("page %d: %v\n", page, err)
			continue
		}
		body, changes, why := clip.Fix(file.Body, model)
		if len(changes) == 0 && len(why) == 0 {
			untouched++
			continue
		}
		for _, refusal := range why {
			refused++
			fmt.Printf("page %d line %d: left alone, %s\n", page, refusal.Line, refusal.Reason)
			fmt.Printf("  %s\n", condenseLine(refusal.Was))
		}
		if len(changes) == 0 {
			continue
		}
		repaired++
		for _, change := range changes {
			fmt.Printf("page %d line %d: put back from the picture\n", page, change.Line)
			fmt.Printf("  was   %s\n", condenseLine(change.Was))
			if *verbose {
				fmt.Printf("  now   %s\n", indent(change.Now))
			} else {
				fmt.Printf("  now   %s\n", condenseLine(change.Now))
			}
		}
		if *dry {
			continue
		}
		file.Body = body
		file.Meta.Pictured = true
		if err := file.Write(path); err != nil {
			return err
		}
	}

	what := "repaired"
	if *dry {
		what = "would be repaired"
	}
	fmt.Printf("%s: %d pages %s, %d displays left alone, %d pages already whole\n",
		entry.ID, repaired, what, refused, untouched)
	return nil
}

// indent is a replacement printed over several lines, lined up under the label.
func indent(text string) string {
	return strings.ReplaceAll(strings.TrimSpace(text), "\n", "\n        ")
}
