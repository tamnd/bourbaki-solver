package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/tamnd/bourbaki-solver/clip"
	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/fleet"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/pdfsrc"
	"github.com/tamnd/bourbaki-solver/prompt"
)

const clipUsage = `usage: bourbaki clip <command> [flags]

commands:
  cut      cut what a query names out of the PDF as pictures
  read     send the pictures to the fleet and pull the readings back
  audit    compare what came back with what the extractor reads

flags for cut:
  -book ID       book id from manifests/books.yaml
  -whole         cut whole pages rather than single lines
  -f N -l N      first and last pdf page, default the whole volume
  -pages LIST    only these pdf pages, comma separated
  -match RE      cut what the extractor renders matching this expression
  -limit N       stop after this many, default 40
  -every N       keep one match in every N, to spread a sample out
  -dpi N         resolution of the cut, default 600 by the line, 300 by the page
  -pad N         page units kept around the cut, default 4

flags for read:
  -book ID       book id
  -hosts LIST    comma separated route names
  -routes PATH   route file
  -batch N       clips per batch, default 12
  -lanes N       override how many clips a host reads at once
  -wait DUR      wait this long for a box with a spare core rather than fail
  -keep          leave the clips on the hosts
  -dry           say what would be read and stop

flags for audit:
  -book ID       book id
  -o PATH        where to write the Markdown report, default reports/clip-<book>.md
  -fresh         judge against what the extractor reads today rather than what
                 it read when the clips were cut
  -v             print every disagreement rather than the first few

The extractor reads a born-digital volume out of its text layer, which is exact
where the font says what it draws and guesswork where it does not. A picture
needs none of that: the model sees what a reader sees. So a clip is two things,
a reading that can be right where ours is wrong, and a second opinion that owes
nothing to our rules.

Cut whole pages. The first run of this cut single lines, and a line goes to the
model with nothing around it: it read an interior as a closure, turned a rho
into a q, and dropped the bar of a not-equals so that the answer said a vector
was zero where the page says it is not. A page carries its own context. It also
costs the same, one model call of three or four minutes, and it carries forty
lines rather than one.

The clips are scratch under work/clips and are never committed, the same as the
page images. The report is Markdown and is.
`

func runClip(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, clipUsage)
		os.Exit(2)
	}
	switch args[0] {
	case "cut":
		return clipCut(args[1:])
	case "read":
		return clipRead(args[1:])
	case "audit":
		return clipAudit(args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(os.Stderr, clipUsage)
		return nil
	default:
		return fmt.Errorf("clip: unknown subcommand %q", args[0])
	}
}

// clipLimit is how many lines a cut takes when nobody says.
//
// Low on purpose. Every clip is a model call on a rented box, and the natural
// mistake with a command that takes a regular expression is to match six hundred
// lines and find out on the fleet. Forty is a sample worth reading and about an
// hour of two boxes.
const clipLimit = 40

func clipCut(args []string) error {
	fs := flag.NewFlagSet("clip cut", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	first := fs.Int("f", 0, "first pdf page")
	last := fs.Int("l", 0, "last pdf page")
	pages := fs.String("pages", "", "only these pdf pages, comma separated")
	match := fs.String("match", "", "cut the lines matching this expression")
	limit := fs.Int("limit", clipLimit, "stop after this many lines")
	every := fs.Int("every", 0, "keep one matching line in every N")
	whole := fs.Bool("whole", false, "cut whole pages rather than lines")
	dpi := fs.Int("dpi", 0, "resolution of the cut")
	pad := fs.Int("pad", clip.Pad, "page units kept around the line")
	fs.Usage = func() { fmt.Fprint(os.Stderr, clipUsage) }
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" || (*match == "" && *pages == "") {
		fs.Usage()
		os.Exit(2)
	}

	root, entry, err := findBook(*book)
	if err != nil {
		return err
	}
	query := clip.Query{First: *first, Last: *last, Limit: *limit, Every: *every}
	if *match != "" {
		expression, err := regexp.Compile(*match)
		if err != nil {
			return fmt.Errorf("-match: %w", err)
		}
		query.Match = expression
	}
	if *pages != "" {
		query.Pages = map[int]bool{}
		for _, field := range strings.Split(*pages, ",") {
			page, err := strconv.Atoi(strings.TrimSpace(field))
			if err != nil {
				return fmt.Errorf("-pages: %q is not a page number", field)
			}
			query.Pages[page] = true
		}
	}

	source, err := openPDF(root, entry)
	if err != nil {
		return err
	}
	// The boxes are measured in the prepared copy and the pictures are cut
	// from the volume. clip.Options.Paper says why.
	paper, err := pdfsrc.Open(filepath.Join(root, entry.PDF))
	if err != nil {
		return err
	}
	ctx := context.Background()
	info, err := source.Info(ctx)
	if err != nil {
		return err
	}
	// The XML of the pages in range, not of the volume. A query over one
	// chapter should not wait for poppler to lay out six hundred pages.
	layout, err := source.XML(ctx, query.First, query.Last)
	if err != nil {
		return err
	}
	if len(layout.Pages) == 0 {
		return fmt.Errorf("%s has no pages in range", entry.ID)
	}
	zoom := clip.ZoomOf(layout.Pages[0], info.WidthPt)
	if zoom != clip.Zoom {
		fmt.Fprintf(os.Stderr, "poppler laid this volume out at a zoom of %.4f rather than %.2f, cutting at the measured one\n",
			zoom, clip.Zoom)
	}

	targets := clip.Find(layout, query)
	unit := "line"
	if *whole {
		unit = "page"
		targets = clip.FindPages(layout, query, pageBody(root, entry.ID))
	}
	if len(targets) == 0 {
		return fmt.Errorf("no %s matched", unit)
	}
	if *dpi == 0 {
		*dpi = clip.DefaultDPI
		if *whole {
			*dpi = clip.PageDPI
		}
	}
	// The volume, not the prepared copy: the index says where the pictures
	// came from, and a reader who wants to look at one again wants the file
	// that was drawn.
	sum, err := paper.SHA256()
	if err != nil {
		return err
	}

	directory := clip.Dir(root, entry.ID)
	options := clip.Options{DPI: *dpi, Pad: *pad, Zoom: zoom, Paper: paper, Logf: func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}}
	if err := clip.Cut(ctx, source, directory, options, targets); err != nil {
		return err
	}
	index := clip.Index{
		Book: entry.ID, PDF: paper.Path, PDFSHA256: sum,
		DPI: *dpi, Zoom: zoom, Pad: *pad, Match: *match,
		Generated: time.Now().UTC(), Targets: targets,
	}
	if err := clip.WriteIndex(clip.IndexPath(root, entry.ID), index); err != nil {
		return err
	}

	fmt.Printf("%s: %d %s clips at %d dpi under %s\n", entry.ID, len(targets), unit, *dpi, directory)
	for _, target := range targets[:min(5, len(targets))] {
		where := fmt.Sprintf("page %d line %d", target.Page, target.Line)
		if target.Whole() {
			where = fmt.Sprintf("page %d", target.Page)
		}
		fmt.Printf("  %s  %s  %s\n", target.Name, where, condenseLine(target.Native))
	}
	if len(targets) > 5 {
		fmt.Printf("  and %d more, all of them in clips.json\n", len(targets)-5)
	}
	return nil
}

// pageBody reads what the extractor already has for a page, which for these
// volumes is the page in the corpus. It is the reading a page clip argues with,
// and it is the file a fix would be written back to, so taking it from anywhere
// else would be auditing something nobody ships.
//
// The running head and the folio come back beside the body. They are not part
// of the reading and the audit is going to throw them away, but it can only
// throw away what it has been told, and what it has been told is this.
func pageBody(root, book string) func(int) (string, string) {
	return func(page int) (string, string) {
		path := filepath.Join(corpus.PagesDir(root, book), fmt.Sprintf("%04d.md", page))
		file, err := corpus.ReadFile[corpus.PageFrontMatter](path)
		if err != nil {
			return "", ""
		}
		return file.Body, strings.TrimSpace(file.Meta.RunningHead + " " + file.Meta.PageLabel)
	}
}

// condenseLine is one line of a page shortened to fit in a terminal.
func condenseLine(text string) string {
	text = strings.Join(strings.Fields(text), " ")
	if runes := []rune(text); len(runes) > 60 {
		return string(runes[:60]) + "..."
	}
	return text
}

func clipRead(args []string) error {
	fs := flag.NewFlagSet("clip read", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	hostList := fs.String("hosts", "", "comma separated route names")
	routeFile := fs.String("routes", "", "route file")
	batch := fs.Int("batch", clip.DefaultBatch, "clips per batch")
	lanes := fs.Int("lanes", 0, "override how many clips a host reads at once")
	wait := fs.Duration("wait", 0, "how long to wait for a host with a spare core")
	keep := fs.Bool("keep", false, "leave the clips on the hosts")
	dry := fs.Bool("dry", false, "say what would be read and stop")
	fs.Usage = func() { fmt.Fprint(os.Stderr, clipUsage) }
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
	index, err := clip.ReadIndex(clip.IndexPath(root, entry.ID))
	if err != nil {
		return fmt.Errorf("%s has no clips: %w", entry.ID, err)
	}
	directory, answers := clip.Dir(root, entry.ID), clip.AnswersDir(root, entry.ID)
	pending, err := clip.Pending(index, directory, answers)
	if err != nil {
		return err
	}
	fmt.Printf("%s: %d clips, %d already read, %d to read\n",
		entry.ID, len(index.Targets), len(index.Targets)-len(pending), len(pending))
	if len(pending) == 0 {
		fmt.Println("nothing to do")
		return nil
	}

	start := time.Now()
	logf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "["+time.Since(start).Round(time.Second).String()+"] "+format+"\n", args...)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	hosts, err := ocrHostsNow(ctx, *routeFile, *hostList, *wait, logf)
	if err != nil {
		return err
	}
	if *lanes > 0 {
		for i := range hosts {
			hosts[i].Lanes = *lanes
		}
	}
	for _, host := range hosts {
		fmt.Printf("  %-8s %d lanes  %s\n", host.Name, host.Lanes, host.Tool)
	}
	if *dry {
		return nil
	}

	// A page and a line are different questions and get different prompts. The
	// index says which this run is: the cut wrote it down, and a run that read
	// pages with the line prompt would spend an hour telling the model to ignore
	// what is sliced off at the edges of a page that has nothing sliced off.
	ask := prompt.ClipLine()
	if len(index.Targets) > 0 && index.Targets[0].Whole() {
		ask = prompt.ClipPage()
	}
	reader := clip.Fleet{
		Hosts: hosts, Shell: fleet.SSH{Timeout: 2 * time.Minute},
		Copy:   ocr.Rsync{Timeout: 30 * time.Minute},
		Prompt: ask, Dir: directory, Dest: answers,
		ID: entry.ID + "-clip", Batch: *batch, Keep: *keep, Logf: logf,
	}
	results, readErr := reader.Read(ctx, pending)
	var wrote, sent int
	for _, result := range results {
		wrote += result.Wrote
		sent += result.Pages
	}
	fmt.Printf("%s: %d of %d clips read in %s\n", entry.ID, wrote, sent, time.Since(start).Round(time.Second))
	if err := writeClipUsage(root, entry.ID, results); err != nil {
		fmt.Fprintf(os.Stderr, "could not write the usage log: %v\n", err)
	}
	return readErr
}

// writeClipUsage appends what the run cost to the same log the page runs write.
//
// The same file on purpose. The question it exists to answer is what the fleet
// costs an hour, and a clip run competes for the same two boxes as a page run,
// so keeping them apart would make both answers wrong.
func writeClipUsage(root, book string, results []ocr.Result) error {
	if len(results) == 0 {
		return nil
	}
	directory := filepath.Join(root, "reports")
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return err
	}
	file, err := os.OpenFile(filepath.Join(directory, "ocr-usage.jsonl"), os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	defer file.Close()
	when := time.Now().UTC().Format(time.RFC3339)
	for _, result := range results {
		line, err := json.Marshal(struct {
			Book string `json:"book"`
			When string `json:"when"`
			Kind string `json:"kind"`
			ocr.Result
		}{book, when, "clip", result})
		if err != nil {
			return err
		}
		if _, err := file.Write(append(line, '\n')); err != nil {
			return err
		}
	}
	return nil
}

func clipAudit(args []string) error {
	fs := flag.NewFlagSet("clip audit", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	out := fs.String("o", "", "where to write the Markdown report")
	verbose := fs.Bool("v", false, "print every disagreement")
	fresh := fs.Bool("fresh", false, "judge against what the extractor reads today rather than what it read when the clips were cut")
	fs.Usage = func() { fmt.Fprint(os.Stderr, clipUsage) }
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
	index, err := clip.ReadIndex(clip.IndexPath(root, entry.ID))
	if err != nil {
		return fmt.Errorf("%s has no clips: %w", entry.ID, err)
	}
	if *fresh {
		index = index.Refresh(pageBody(root, entry.ID))
	}
	report, err := clip.Compare(index, clip.AnswersDir(root, entry.ID))
	if err != nil {
		return err
	}
	fmt.Print(report.Summary())

	shown := 0
	for _, row := range report.Rows {
		if row.Verdict != clip.Differ {
			continue
		}
		if !*verbose && shown >= 10 {
			fmt.Printf("and %d more, all of them in the report\n", report.Differed-shown)
			break
		}
		shown++
		if row.Line < 0 {
			fmt.Printf("\npage %d\n", row.Page)
			if len(row.Lost) > 0 {
				fmt.Printf("  the model read and we have not  %s\n", condenseLine(strings.Join(row.Lost, " ")))
			}
			if len(row.Extra) > 0 {
				fmt.Printf("  we have and the model did not   %s\n", condenseLine(strings.Join(row.Extra, " ")))
			}
			continue
		}
		fmt.Printf("\npage %d line %d\n  ours  %s\n  model %s\n", row.Page, row.Line, row.Native, row.Model)
	}

	path := *out
	if path == "" {
		path = filepath.Join(root, "reports", "clip-"+entry.ID+".md")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(path, []byte(report.Markdown()), 0o644); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(strings.TrimSuffix(path, ".md")+".json", append(raw, '\n'), 0o644); err != nil {
		return err
	}
	fmt.Printf("\n%s\n", path)
	return nil
}

// findBook is the three lookups every one of these commands opens with.
func findBook(book string) (string, *corpus.Book, error) {
	root, err := corpus.Root()
	if err != nil {
		return "", nil, err
	}
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return "", nil, err
	}
	entry, ok := books.Get(book)
	if !ok {
		return "", nil, fmt.Errorf("no book %q in %s", book, corpus.BooksPath(root))
	}
	return root, entry, nil
}
