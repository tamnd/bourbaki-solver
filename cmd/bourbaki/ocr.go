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
	"github.com/tamnd/bourbaki-solver/prompt"
	"github.com/tamnd/bourbaki-solver/render"
)

const ocrUsage = `usage: bourbaki ocr <command> [flags]

commands:
  fill     put a job in the queue for every page of a volume that has to be read
  run      lease pages, read them on the fleet, validate what comes back
  check    run the nine validation rules over the page files already on disk
  repair   ask the model to fix a flagged page in the conversation that read it
  audit    ask about pages that passed the rules and may still be wrong

flags for fill and run:
  -book ID       book id from manifests/books.yaml
  -f N -l N      first and last pdf page, default the whole volume
  -batch N       pages per batch, default 25
  -limit N       stop after this many pages, for a pilot
  -hosts LIST    comma separated route names, default every route that does OCR.
                 -hosts local reads the pages on this machine with the Claude
                 Code CLI instead of the fleet, which needs no browser account
                 and reads a page in about fifteen seconds. It is not the path
                 for a volume: it spends a metered Claude session quota that
                 costs real money, and it will not return a page of continuous
                 prose at all. The fleet is what reads a book.
  -routes PATH   route file, default ~/.config/bourbaki/routes.json
  -queue PATH    queue directory
  -keep          leave the page images on the hosts, for debugging
  -no-repair     reject a failed page instead of asking about it in its thread
  -salvage       on the attempt that would kill a page, write it anyway when the
                 only things wrong with it are things a fix pass can put right.
                 Five of the nine rules are like that: a statement head in mixed
                 case is what fix smallcaps is for, an unclosed dollar is what
                 fix dollars is for, a first line that does not read as a running
                 head is what fix folio and fix heading are for, an exercise set
                 as a heading is a mark in the wrong place, and LaTeX that does
                 not compile is a fragment to mend. The other four say the answer
                 is not that page at all and no flag makes it one. A salvaged
                 page carries a flag naming what is still wrong with it, so the
                 corpus never claims it is clean. Page 128 of Algebra I to III
                 died on the statement rule and took chapter I § 8 out of the
                 assembly with it, which is the case this is for
  -lanes N       override how many pages a host reads at once
  -wait DUR      wait this long for a box with a spare core rather than fail
  -flagged       only the pages a native extraction could not read, which is the
                 one way a born-digital volume is read through a model
  -contents      read the pages named by -f and -l as a table of contents, which
                 is a different question: plain text with the indentation, the
                 leader dots and the page numbers kept, because the page numbers
                 are the whole content of a contents page. It needs a range, and
                 it is for the volumes whose text layer drops that column
  -unread        only the pages with no reading committed, so that editing the
                 prompt does not send the pages already read back to the fleet
  -again         (fill) queue pages that already pass the rules. The rules are
                 structural and a page can be well formed and wrong, so this is
                 how an operator says a set of readings should be made again
                 whatever the rules think of them. It needs -f and -l, because
                 the pages it puts back already passed. Pages a stronger reader
                 wrote are still held back for that reader, so pointing it at a
                 volume read half by gpt-5 and half by a weaker model queues the
                 weaker half and leaves the rest
  -reread-protected
                 read over pages that claude or gpt-5 already read. Those stand
                 by default even when the prompt or the render moved under them,
                 and now also when they fail the rules and no host in the run
                 reads better than whoever wrote them, because a weaker reader
                 writing over them has cost the corpus real mathematics three
                 times. Ask for this after a prompt change made because the old
                 readings were wrong, or after a deliberate re-render at a
                 higher resolution, and not otherwise
  -dry           say what would be read and stop

flags for check:
  -rule NAME     only report this rule: short, math, leak, head, illegible, label,
                 latex, exercise, statement
  -v             print every problem, not just the counts
  -json          print one JSON object per rejected page

run is the expensive command in this repo. It sends page images to rented boxes
over ssh, reads them with chatgpt-tool ocr-batch, pulls the Markdown back and
accepts or rejects each page against the nine rules. A page costs minutes, so
nothing is read twice: the queue is content addressed on the image and the
prompt, and a run that is interrupted picks up where it stopped.

check is the same code that decides whether to accept a page, run against what
is already written. It is how a change to the rules is measured before it is
turned loose on 1194 pages.

repair is for the pages check finds. A page with one unclosed formula does not
need reading again, it needs one question asked in the thread that produced it,
where the image is still in context. What comes back has to be the same page
with a dollar moved and nothing else, proved rather than assumed, or it is
thrown away and the page goes back to the image. See bourbaki ocr repair -h.

audit is for the pages check says are fine. Page 51 of Algebra I passed all
nine rules and reads i in {1, n} where the scan prints i in [1, n], which is a
set of two elements where the book has an interval. No rule can see that. The
detectors look for shapes this scan gets wrong, and each one becomes one
question in the thread that produced the page, where the image still is. See
bourbaki ocr audit -h.
`

func runOCR(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, ocrUsage)
		os.Exit(2)
	}
	switch args[0] {
	case "fill":
		return ocrFill(args[1:])
	case "run":
		return ocrRun(args[1:])
	case "check":
		return ocrCheck(args[1:])
	case "repair":
		return ocrRepair(args[1:])
	case "audit":
		return ocrAudit(args[1:])
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

		problems := ocr.Validate(checkText(file), expectFor(entry, pmap, manifest, page), ocr.Options{Prompt: prompt.OCRAnything(entry.ID, entry.Book)})
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
// Both extraction paths file the running head in the front matter, and neither
// body starts with it. Native extraction parses it out of the text layer.
// Vision OCR is asked for it on the first line of the answer, and ocr.readHead
// takes that line, splits it into the label, the title and the locator, and
// cuts it out of the body before the page file is written. So the head is off
// the body by the time a page file exists, whichever way the page was read.
//
// This used to put the head back for native pages only, on the reading that an
// OCR body still opens with it. It does not, and the cost of the mistake was
// the largest single number in the extraction report: of the 4903 OCR pages in
// the nineteen head-label volumes, 4320 carry a page label in the front matter
// and not one carried it on the first body line, so rule 4 asked every one of
// them for a head that had been moved and rejected all of them. 5162 rejections
// against 105 for the next rule, none of them a real defect.
//
// The other 583 have no head in the front matter, which means readHead did not
// recognise one in the answer. Those keep their bare body and rule 4 keeps
// judging them, which is the case the rule exists for.
func checkText(file corpus.PageFile) string {
	head := headOf(file.Meta)
	if head == "" {
		return file.Body
	}
	return head + "\n\n" + file.Body
}

// headOf is the running head of a page put back into the one line it was
// printed on, out of the parts the front matter holds it in.
func headOf(meta corpus.PageFrontMatter) string {
	return strings.TrimSpace(strings.Join([]string{
		meta.PageLabel, meta.RunningHead, locatorOf(meta),
	}, "  "))
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
