package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/tamnd/bourbaki-solver/audit"
	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/fleet"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/repair"
)

// The audit pass is for the pages that passed.
//
// The seven rules decide whether an answer is a transcription. They catch a
// model that gave up, one that narrated, one that left a formula open, and they
// are the only thing standing between the fleet and 1194 pages of nonsense. What
// they cannot catch is a page that is a transcription and is wrong in one
// character, because there is nothing about it to notice.
//
// Page 51 of Algebra I was accepted by all seven and reads i in {1, n} where the
// scan prints i in [1, n]. It was found by a person looking at the image next to
// the text, which does not scale to 1194 pages.
//
// So the detectors in the audit package look for shapes a scan of this typeface
// gets wrong, and each one becomes a single question put back into the
// conversation that produced the page, where the image still is. Most come back
// confirmed. The ones that do not are corrected under a proof that only the
// flagged span moved.

const ocrAuditUsage = `usage: bourbaki ocr audit -book ID [flags]

Look for pages that passed every rule and may still be wrong, and ask about
them in the conversations that produced them.

  -book ID       book id from manifests/books.yaml
  -f N -l N      first and last pdf page, default the whole volume
  -limit N       stop after this many questions
  -hosts LIST    comma separated route names
  -routes PATH   route file
  -dry           list what would be asked, and where, then stop

A suspect is not a defect. The detectors look for shapes that this scan is known
to produce wrongly, they are wrong more often than they are right, and the whole
point of asking in the thread is that the model can see the page and the checker
cannot. Confirming costs one word and settles the spot.

A correction is only written when the answer is the same line with the flagged
span replaced, byte for byte on both sides of it, and the new span within four
runes of the old one. Anything else is thrown away and the page is left as it
was, which is the safe direction: the page already passed.
`

func ocrAudit(args []string) error {
	fs := flag.NewFlagSet("ocr audit", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	first := fs.Int("f", 0, "first pdf page")
	last := fs.Int("l", 0, "last pdf page")
	limit := fs.Int("limit", 0, "stop after this many questions")
	hostList := fs.String("hosts", "", "comma separated route names")
	routeFile := fs.String("routes", "", "route file")
	dry := fs.Bool("dry", false, "say what would be asked and stop")
	fs.Usage = func() { fmt.Fprint(os.Stderr, ocrAuditUsage) }
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" {
		fs.Usage()
		os.Exit(2)
	}

	state, err := ocrSetup(*book, defaultQueueRoot(), false, false)
	if err != nil {
		return err
	}
	pages, suspects, err := suspectPages(state, *first, *last)
	if err != nil {
		return err
	}
	if len(suspects) == 0 {
		fmt.Printf("%s: %d pages read, nothing to ask about\n", state.entry.ID, pages)
		return nil
	}
	fmt.Printf("%s: %d pages read, %d spans worth a question\n", state.entry.ID, pages, len(suspects))

	var ready []spot
	for _, value := range suspects {
		thread, err := ocr.ReadThread(state.root, state.entry.ID, value.page)
		if err != nil {
			fmt.Printf("  page %4d  no conversation was recorded, so nobody can look at the image\n", value.page)
			continue
		}
		if err := ocr.ValidConversation(thread.Conversation); err != nil {
			fmt.Printf("  page %4d  %v\n", value.page, err)
			continue
		}
		value.thread = thread
		ready = append(ready, value)
		fmt.Printf("  page %4d  line %d  %s  on %s\n", value.page, value.suspect.Line, value.suspect.Span, thread.Host)
		if *limit > 0 && len(ready) >= *limit {
			break
		}
	}
	if len(ready) == 0 {
		fmt.Println("no suspect can be asked about in its own thread")
		return nil
	}
	if *dry {
		return nil
	}

	hosts, err := ocrHosts(*routeFile, *hostList)
	if err != nil {
		return err
	}
	byName := map[string]ocr.Host{}
	for _, host := range hosts {
		byName[host.Name] = host
	}
	start := time.Now()
	logf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "["+time.Since(start).Round(time.Second).String()+"] "+format+"\n", args...)
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var confirmed, corrected, unresolved int
	for _, value := range ready {
		if ctx.Err() != nil {
			break
		}
		host, ok := byName[value.thread.Host]
		if !ok {
			logf("page %d: it was read on %s, which is not in this run", value.page, value.thread.Host)
			unresolved++
			continue
		}
		result, err := askAboutSpan(ctx, state, host, value)
		switch {
		case err != nil:
			logf("page %d: the question failed: %v", value.page, err)
			unresolved++
		case result.Confirmed:
			confirmed++
			fmt.Printf("page %d line %d: %s confirmed against the image\n", value.page, value.suspect.Line, value.suspect.Span)
		case result.Accepted:
			if err := writeAudited(state, value, result.Text); err != nil {
				logf("page %d: the correction was accepted and could not be written: %v", value.page, err)
				unresolved++
				continue
			}
			corrected++
			fmt.Printf("page %d line %d: %s corrected\n", value.page, value.suspect.Line, value.suspect.Span)
		default:
			unresolved++
			logf("page %d: the answer settled nothing, the page is left as it was: %s", value.page, result.Reason)
		}
	}
	fmt.Printf("%s: %d confirmed, %d corrected, %d unresolved\n", state.entry.ID, confirmed, corrected, unresolved)
	return ctx.Err()
}

// spot is one span of one page, and the conversation to ask about it in.
type spot struct {
	page    int
	file    corpus.PageFile
	path    string
	text    string
	suspect repair.Suspect
	thread  ocr.Thread
}

// suspectPages runs the detectors over every page of a volume that a model read
// and that passes the rules.
//
// A page that fails a rule is left out. It has a defect on it, the repair pass
// is what that is for, and a page being fixed in one place is not a page to ask
// a delicate question about in another.
func suspectPages(state setup, first, last int) (int, []spot, error) {
	paths, err := filepath.Glob(filepath.Join(corpus.PagesDir(state.root, state.entry.ID), "*.md"))
	if err != nil {
		return 0, nil, err
	}
	sort.Strings(paths)
	var read int
	var out []spot
	for _, path := range paths {
		file, err := corpus.ReadFile[corpus.PageFrontMatter](path)
		if err != nil {
			return 0, nil, fmt.Errorf("%s: %w", path, err)
		}
		page := file.Meta.PDFPage
		if first > 0 && page < first {
			continue
		}
		if last > 0 && page > last {
			continue
		}
		if file.Meta.Method != corpus.MethodOCR {
			continue
		}
		read++
		text := checkText(file)
		if problems := ocr.Validate(text, state.expect(page), ocr.Options{}); len(problems) > 0 {
			continue
		}
		for _, suspect := range audit.Scan(text) {
			out = append(out, spot{page: page, file: file, path: path, text: text, suspect: suspect})
		}
	}
	return read, out, nil
}

// askAboutSpan puts one question and audits one answer.
func askAboutSpan(ctx context.Context, state setup, host ocr.Host, value spot) (repair.Result, error) {
	request := repair.Request{Book: state.entry.ID, Page: value.page, Text: value.text, Suspect: &value.suspect}
	if _, ok := request.Kind(); !ok {
		return repair.Result{Reason: "this span cannot be asked about under a proof"}, nil
	}
	prompt := request.Prompt()
	ask := ocr.Follow{
		Host: host, Shell: fleet.SSH{Timeout: 2 * time.Minute}, Copy: ocr.Rsync{Timeout: 5 * time.Minute},
		Conversation: value.thread.Conversation, Profile: value.thread.Profile,
		Prompt: prompt, ID: askID(state.entry.ID, value.page, prompt),
	}
	answer, err := ask.Ask(ctx)
	if err != nil {
		return repair.Result{}, err
	}
	if err := archive(state.root, state.entry.ID, value.page, prompt, answer); err != nil {
		return repair.Result{}, err
	}
	return repair.Audit(request, answer, state.expect(value.page)), nil
}

// writeAudited puts the corrected page back.
//
// Only the one line moved, and the audit proved it, so the front matter is kept
// as it was apart from the line count and a flag saying what was asked. The flag
// is the record: a page that has been through this is a page a person does not
// have to look at that span on again.
func writeAudited(state setup, value spot, text string) error {
	head, body := ocr.SplitHead(text)
	meta := value.file.Meta
	if head.Label != "" {
		meta.PageLabel = head.Label
	}
	if head.Title != "" {
		meta.RunningHead = head.Title
	}
	if head.Locator != nil {
		meta.Locator = head.Locator
	}
	meta.Lines = len(strings.Split(strings.TrimSpace(body), "\n"))
	meta.Flags = append(meta.Flags, fmt.Sprintf("audited in its own thread: %s on line %d", value.suspect.Span, value.suspect.Line))
	return corpus.PageFile{Meta: meta, Body: body}.Write(value.path)
}
