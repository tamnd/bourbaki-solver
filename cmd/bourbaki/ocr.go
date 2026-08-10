package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/render"
)

const ocrUsage = `usage: bourbaki ocr <command> [flags]

commands:
  check    run the seven validation rules over the page files already on disk

flags:
  -book ID     book id from manifests/books.yaml
  -f N -l N    first and last pdf page, default the whole volume
  -rule NAME   only report this rule: short, math, leak, head, illegible, label, latex
  -v           print every problem, not just the counts
  -json        print one JSON object per rejected page

check is the same code that decides whether to accept a page as it comes back
from a model, run against what is already written. It is how a change to the
rules is measured before it is turned loose on 1194 pages at 151 seconds each.
`

func runOCR(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, ocrUsage)
		os.Exit(2)
	}
	switch args[0] {
	case "check":
		return ocrCheck(args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(os.Stderr, ocrUsage)
		return nil
	default:
		return fmt.Errorf("ocr: unknown subcommand %q", args[0])
	}
}

func ocrCheck(args []string) error {
	fs := flag.NewFlagSet("ocr check", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	first := fs.Int("f", 0, "first pdf page")
	last := fs.Int("l", 0, "last pdf page")
	only := fs.String("rule", "", "only report this rule")
	verbose := fs.Bool("v", false, "print every problem")
	asJSON := fs.Bool("json", false, "print one JSON object per rejected page")
	fs.Usage = func() { fmt.Fprint(os.Stderr, ocrUsage) }
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" {
		fs.Usage()
		os.Exit(2)
	}

	root, err := corpus.Root()
	if err != nil {
		return err
	}
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return err
	}
	entry, ok := books.Get(*book)
	if !ok {
		return fmt.Errorf("no book %q in %s", *book, corpus.BooksPath(root))
	}

	// The page map is what rules 4 and 6 compare against. Without it the two
	// rules are skipped rather than guessed at, and the run says so.
	pmap, mapErr := pagemap.Load(root, entry.ID)
	if mapErr != nil {
		fmt.Fprintf(os.Stderr, "no page map for %s, so the running head and page label rules are skipped: %v\n", entry.ID, mapErr)
	}
	// Blank pages come from the render manifest when there is one. A volume
	// that was never rendered has none, and every page is treated as inked.
	manifest, _ := render.ReadManifest(root, entry.ID)

	paths, err := filepath.Glob(filepath.Join(corpus.PagesDir(root, entry.ID), "*.md"))
	if err != nil {
		return err
	}
	if len(paths) == 0 {
		return fmt.Errorf("no page files under %s", corpus.PagesDir(root, entry.ID))
	}
	sort.Strings(paths)

	var checked, rejected, blank, skipped int
	counts := map[ocr.Rule]int{}
	byRule := map[ocr.Rule][]int{}

	for _, path := range paths {
		file, err := corpus.ReadFile[corpus.PageFrontMatter](path)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		page := file.Meta.PDFPage
		if *first > 0 && page < *first {
			continue
		}
		if *last > 0 && page > *last {
			continue
		}
		if file.Meta.Method == corpus.MethodBlank {
			blank++
			continue
		}
		checked++

		expect := ocr.Expect{Book: entry.ID, PDFPage: page, Grammar: pagemap.Grammar(entry.Grammar)}
		if value, ok := manifest.Find(page); ok {
			expect.Blank = value.Blank
			expect.Sparse = value.Ink < ocr.SparseInk
		}
		if pmap != nil {
			if pageEntry, ok := pmap.Lookup(page); ok {
				expect.Chapter, expect.Page = pageEntry.Chapter, pageEntry.Page
				expect.Confidence = pageEntry.Confidence
				// A page whose number was read off its own running head has
				// one by definition. Anywhere else the page map cannot say,
				// and asking for a head that a chapter opener does not print
				// would fail one page per chapter.
				expect.HasHead = pageEntry.Confidence == pagemap.FromHead
			}
		}

		problems := ocr.Validate(checkText(file), expect, ocr.Options{})
		if *only != "" {
			problems = filterRule(problems, ocr.Rule(*only))
		}
		if len(problems) == 0 {
			continue
		}
		rejected++
		for _, rule := range ocr.Rules(problems) {
			counts[rule]++
			byRule[rule] = append(byRule[rule], page)
		}
		switch {
		case *asJSON:
			raw, err := json.Marshal(struct {
				Page     int           `json:"pdf_page"`
				Method   string        `json:"method"`
				Problems []ocr.Problem `json:"problems"`
			}{page, string(file.Meta.Method), problems})
			if err != nil {
				return err
			}
			fmt.Println(string(raw))
		case *verbose:
			fmt.Printf("%s page %d: %s\n", entry.ID, page, ocr.Reasons(problems))
		}
	}
	if pmap == nil {
		skipped = checked
	}

	fmt.Printf("%s: %d pages checked, %d blank skipped, %d rejected, %.1f %% accepted\n",
		entry.ID, checked, blank, rejected, percent(checked-rejected, checked))
	if skipped > 0 {
		fmt.Printf("the running head and page label rules did not run, there is no page map\n")
	}
	rules := make([]ocr.Rule, 0, len(counts))
	for rule := range counts {
		rules = append(rules, rule)
	}
	sort.Slice(rules, func(i, j int) bool { return counts[rules[i]] > counts[rules[j]] })
	for _, rule := range rules {
		pages := byRule[rule]
		fmt.Printf("  %-10s %4d  %s\n", rule, counts[rule], pageList(pages))
	}
	return nil
}

// checkText reconstructs what a model would have returned for a page.
//
// The two extraction paths put the running head in different places. Vision OCR
// is asked for it on the first line of the answer, and that is what the rules
// read. Native extraction parses it out of the text and files it in the front
// matter, so its body starts at the first paragraph. Running the head rule
// against a native body rejects every page of a born-digital volume, which is
// what happened the first time this was run: 448 of 494 pages of alg-viii, none
// of them a real defect. Putting the head back is the honest comparison.
func checkText(file corpus.PageFile) string {
	if file.Meta.Method != corpus.MethodNative {
		return file.Body
	}
	head := strings.TrimSpace(strings.Join([]string{
		file.Meta.PageLabel, file.Meta.RunningHead, locatorOf(file.Meta),
	}, "  "))
	if head == "" {
		return file.Body
	}
	return head + "\n\n" + file.Body
}

func locatorOf(meta corpus.PageFrontMatter) string {
	if meta.Locator == nil || meta.Locator.Section == 0 {
		return ""
	}
	if meta.Locator.Subsec > 0 {
		return fmt.Sprintf("§ %d.%d", meta.Locator.Section, meta.Locator.Subsec)
	}
	return fmt.Sprintf("§ %d", meta.Locator.Section)
}

func filterRule(problems []ocr.Problem, rule ocr.Rule) []ocr.Problem {
	var out []ocr.Problem
	for _, problem := range problems {
		if problem.Rule == rule {
			out = append(out, problem)
		}
	}
	return out
}

func percent(part, whole int) float64 {
	if whole == 0 {
		return 0
	}
	return float64(part) * 100 / float64(whole)
}

// pageList prints the first few page numbers. The whole list goes in -v, and a
// summary line that ran to four hundred numbers would be unreadable.
func pageList(pages []int) string {
	const show = 12
	parts := make([]string, 0, show)
	for i, page := range pages {
		if i == show {
			return strings.Join(parts, " ") + fmt.Sprintf(" and %d more", len(pages)-show)
		}
		parts = append(parts, fmt.Sprint(page))
	}
	return strings.Join(parts, " ")
}
