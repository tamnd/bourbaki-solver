package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"syscall"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/fleet"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/repair"
)

// The repair pass is the cheap half of reading a book badly.
//
// A page that came back with one unclosed formula is not a page the model
// failed to read, it is a page it read and typed wrong in one character, and
// the alternative to fixing it is sending the image again for another two
// minutes and hoping. So the fault goes back into the conversation that
// produced it, where the image is still in context, and what comes back is
// checked hard enough that a model which decides to improve Bourbaki instead
// gets its answer thrown away.
//
// Nothing here decides whether an answer is good. That is the repair package,
// which proves that only dollar signs moved. This file is the wiring: find the
// host the conversation lives on, ask, and write the page back if the audit
// passed.

// mender builds the repair function the run loop and the standalone command
// both use.
//
// It closes over the hosts because a conversation is not portable. It belongs
// to one account, which belongs to one Chrome profile, which is on one box, so
// the follow up has to go to the machine that read the page or it goes nowhere.
func mender(root string, hosts []ocr.Host, expect func(page int) ocr.Expect, logf func(string, ...any)) func(context.Context, ocr.Thread, int, string, []ocr.Problem) (string, bool) {
	byName := map[string]ocr.Host{}
	for _, host := range hosts {
		byName[host.Name] = host
	}
	return func(ctx context.Context, thread ocr.Thread, page int, text string, problems []ocr.Problem) (string, bool) {
		host, ok := byName[thread.Host]
		if !ok {
			logf("page %d: it was read on %s, which is not in this run, so it goes back to the image", page, thread.Host)
			return "", false
		}
		result, err := askForRepair(ctx, root, host, thread, page, text, problems, expect(page))
		if err != nil {
			logf("page %d: the follow up failed, so it goes back to the image: %v", page, err)
			return "", false
		}
		switch {
		case result.Accepted:
			return result.Text, true
		case result.Refused:
			logf("page %d: the model declined to repair it: %s", page, result.Reason)
		default:
			logf("page %d: the answer was not a repair: %s", page, result.Reason)
		}
		return "", false
	}
}

// askForRepair puts one question and audits one answer.
func askForRepair(ctx context.Context, root string, host ocr.Host, thread ocr.Thread, page int, text string, problems []ocr.Problem, expect ocr.Expect) (repair.Result, error) {
	request := repair.Request{Book: thread.Book, Page: page, Text: text, Problems: problems}
	if _, ok := request.Kind(); !ok {
		return repair.Result{Reason: "this page has no defect that can be repaired without re-reading it"}, nil
	}
	prompt := request.Prompt()
	ask := ocr.Follow{
		Host: host, Shell: fleet.SSH{Timeout: 2 * time.Minute}, Copy: ocr.Rsync{Timeout: 5 * time.Minute},
		Conversation: thread.Conversation, Profile: thread.Profile,
		Prompt: prompt, ID: askID(thread.Book, page, prompt),
	}
	answer, err := ask.Ask(ctx)
	if err != nil {
		return repair.Result{}, err
	}
	// Both halves are kept before either is judged. An accepted repair rewrites
	// the page, so without this there is no way afterwards to tell a model that
	// was asked the wrong thing from one that answered badly.
	if err := archive(root, thread.Book, page, prompt, answer); err != nil {
		return repair.Result{}, err
	}
	return repair.Audit(request, answer, expect), nil
}

// askID names the scratch directory on the host. The prompt is hashed into it
// so that two attempts at the same page, which is what happens when the rules
// change, do not share a directory.
func askID(book string, page int, prompt string) string {
	sum := sha256.Sum256([]byte(prompt))
	return fmt.Sprintf("%s-%04d-ask-%s", book, page, hex.EncodeToString(sum[:])[:6])
}

// RepairDir is where the questions and the answers are kept, under work/ and
// out of the corpus: the prompt quotes a whole page of a copyrighted book back
// at the model, and the answer is the model's own words until it is accepted.
func repairDir(root, book string, page int) string {
	return filepath.Join(root, "work", "repairs", book, fmt.Sprintf("%04d", page))
}

func archive(root, book string, page int, prompt, answer string) error {
	dir := repairDir(root, book, page)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	stamp := time.Now().UTC().Format("20060102T150405Z")
	if err := os.WriteFile(filepath.Join(dir, stamp+".ask.md"), []byte(prompt), 0o600); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, stamp+".answer.md"), []byte(answer), 0o600)
}

const repairUsage = `usage: bourbaki ocr repair -book ID [flags]

Ask the model to fix pages that are already written and no longer pass the
rules, in the conversations that produced them.

  -book ID       book id from manifests/books.yaml
  -f N -l N      first and last pdf page, default the whole volume
  -rule NAME     only repair this rule, default math, which is the only one
                 with a repair that can be proved
  -limit N       stop after this many pages
  -hosts LIST    comma separated route names
  -routes PATH   route file
  -dry           say which pages would be asked about, and where, then stop

A page is only offered when three things hold. It fails a rule that has a
provable repair, which today means one thing: an inline formula that is opened
and never closed. It was read in a conversation this fleet recorded, which
pages read before the tool reported conversation URLs were not. And the answer
that comes back is identical to the page with every dollar sign removed, which
is what stops a model from tidying the notation while it is in there.

Anything that fails those goes back to being read again from the image, which
costs a full page and is always available.
`

func ocrRepair(args []string) error {
	fs := flag.NewFlagSet("ocr repair", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	first := fs.Int("f", 0, "first pdf page")
	last := fs.Int("l", 0, "last pdf page")
	only := fs.String("rule", string(ocr.RuleMath), "only repair this rule")
	limit := fs.Int("limit", 0, "stop after this many pages")
	hostList := fs.String("hosts", "", "comma separated route names")
	routeFile := fs.String("routes", "", "route file")
	dry := fs.Bool("dry", false, "say what would be asked and stop")
	fs.Usage = func() { fmt.Fprint(os.Stderr, repairUsage) }
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" {
		fs.Usage()
		os.Exit(2)
	}

	state, err := ocrSetup(*book, defaultQueueRoot(), false)
	if err != nil {
		return err
	}
	broken, err := brokenPages(state, *first, *last, ocr.Rule(*only))
	if err != nil {
		return err
	}
	if len(broken) == 0 {
		fmt.Printf("%s: no page in range fails %s\n", state.entry.ID, *only)
		return nil
	}
	fmt.Printf("%s: %d pages fail %s\n", state.entry.ID, len(broken), *only)

	var ready []candidate
	for _, value := range broken {
		thread, err := ocr.ReadThread(state.root, state.entry.ID, value.page)
		if err != nil {
			fmt.Printf("  page %4d  no conversation was recorded, it has to be read again\n", value.page)
			continue
		}
		if err := ocr.ValidConversation(thread.Conversation); err != nil {
			fmt.Printf("  page %4d  %v\n", value.page, err)
			continue
		}
		value.thread = thread
		ready = append(ready, value)
		fmt.Printf("  page %4d  %s on %s\n", value.page, ocr.Reasons(value.problems), thread.Host)
		if *limit > 0 && len(ready) >= *limit {
			break
		}
	}
	if len(ready) == 0 {
		fmt.Println("no page can be repaired in its own thread, they all have to be read again")
		return nil
	}
	if *dry {
		return nil
	}

	hosts, err := ocrHosts(*routeFile, *hostList)
	if err != nil {
		return err
	}
	start := time.Now()
	logf := func(format string, args ...any) {
		fmt.Fprintf(os.Stderr, "["+time.Since(start).Round(time.Second).String()+"] "+format+"\n", args...)
	}
	fix := mender(state.root, hosts, state.expect, logf)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	var repaired, refused int
	for _, value := range ready {
		if ctx.Err() != nil {
			break
		}
		fixed, ok := fix(ctx, value.thread, value.page, value.text, value.problems)
		if !ok {
			refused++
			continue
		}
		if err := writeRepaired(state, value, fixed); err != nil {
			logf("page %d: the repair was accepted and could not be written: %v", value.page, err)
			refused++
			continue
		}
		repaired++
		fmt.Printf("page %d repaired\n", value.page)
	}
	fmt.Printf("%s: %d repaired, %d still have to be read again\n", state.entry.ID, repaired, refused)
	return ctx.Err()
}

// candidate is one page that fails a rule and might be fixable in its thread.
type candidate struct {
	page     int
	file     corpus.PageFile
	path     string
	text     string
	problems []ocr.Problem
	thread   ocr.Thread
}

// brokenPages is every page on disk that fails the rule asked for.
//
// Only pages that were read by a model. A natively extracted page has no
// conversation and never had one, and a page marked blank has no text to argue
// about.
func brokenPages(state setup, first, last int, rule ocr.Rule) ([]candidate, error) {
	paths, err := filepath.Glob(filepath.Join(corpus.PagesDir(state.root, state.entry.ID), "*.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	var out []candidate
	for _, path := range paths {
		file, err := corpus.ReadFile[corpus.PageFrontMatter](path)
		if err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
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
		text := checkText(file)
		problems := ocr.Validate(text, state.expect(page), ocr.Options{})
		if rule != "" {
			problems = filterRule(problems, rule)
		}
		if len(problems) == 0 {
			continue
		}
		out = append(out, candidate{page: page, file: file, path: path, text: text, problems: problems})
	}
	return out, nil
}

// writeRepaired puts the fixed page back where the broken one was.
//
// The head is taken apart again rather than kept, because the repaired text is
// what was audited and what is written has to be that text and nothing else. A
// repair that somehow changed the running head would be caught here, since only
// dollars can have moved and no running head in these volumes has one.
func writeRepaired(state setup, value candidate, fixed string) error {
	head, body := ocr.SplitHead(fixed)
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
	meta.Flags = append(meta.Flags, "repaired in its own thread: "+ocr.Reasons(value.problems))
	return corpus.PageFile{Meta: meta, Body: body}.Write(value.path)
}
