package main

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/tamnd/bourbaki-solver/ocr"
)

// ocr-batch on this machine is the same tool the rented boxes run, in the sense
// that matters: it takes a directory of page images and a prompt and mirrors it
// into a directory of Markdown, one file per image with the extension swapped.
// The batch protocol in ocr/batch.go was written against chatgpt-tool's
// ocr-batch and this answers to the same command line, so a run that reads
// pages here needs no branch above the transport.
//
// What it reads with is the Claude Code CLI, which is on this machine and is
// not on a browser account with two uploads left in the day.

const ocrBatchUsage = `usage: bourbaki ocr-batch <images> <answers> [flags]

reads every page image in the images directory with the Claude Code CLI and
writes one Markdown file per image into the answers directory, named for the
image with the extension swapped: 0022.png becomes 0022.md.

flags:
  -j N             pages read at once, default 1
  -model NAME      model the CLI is asked for, default ` + defaultLocalModel + `
  -cli PATH        the Claude Code binary, default claude on PATH
  -prompt TEXT     the transcription instruction, required
  -ext EXT         image extension to read, default png
  -timeout N       seconds one page is allowed, default 900
  -rate-delay F    seconds between starting one page and the next
  -skip-existing   leave a page alone when its answer is already written
  -recursive       read the images directory tree and not just its top

This is the local half of bourbaki ocr run. It is named ocr-batch and takes
these flags because that is the command line the batch protocol builds, and the
protocol should not have to know which kind of host it is talking to.

A page that comes back empty, or that the CLI refuses, is not written. The run
above counts the answers that are there, puts the missing pages back in the
queue and reads them again later, which is the same thing it does for a page a
rented box did not manage.
`

// defaultLocalModel is what a page is read with here.
//
// The full model rather than a small one. This is the reading the whole corpus
// is built on, and a page misread is a page nothing downstream can recover: the
// picture is not in the repository and the sections, the tags and the
// translations are all made from the Markdown. A cheaper model would be read
// again by hand at more cost than it saved.
const defaultLocalModel = "opus"

// limitPhrases are how the CLI says the account is out of turns for now.
//
// It says it on standard error and exits non zero, which on its own is
// indistinguishable from a page that failed. The difference matters: one page
// failing is a page to read again, and the account being out means every page
// after it fails in a second and burns an attempt each. So the batch stops on
// this and lets the queue keep the pages.
var limitPhrases = []string{"session limit", "usage limit", "rate limit", "limit reached"}

func runOCRBatch(args []string) error {
	if len(args) < 2 || strings.HasPrefix(args[0], "-") {
		fmt.Fprint(os.Stderr, ocrBatchUsage)
		os.Exit(2)
	}
	in, out := args[0], args[1]

	fs := flag.NewFlagSet("ocr-batch", flag.ExitOnError)
	jobs := fs.Int("j", 1, "pages read at once")
	model := fs.String("model", defaultLocalModel, "model the CLI is asked for")
	binary := fs.String("cli", "claude", "the Claude Code binary")
	ask := fs.String("prompt", "", "the transcription instruction")
	ext := fs.String("ext", "png", "image extension to read")
	timeout := fs.Int("timeout", 900, "seconds one page is allowed")
	rate := fs.Float64("rate-delay", 0, "seconds between starting one page and the next")
	skip := fs.Bool("skip-existing", false, "leave a page alone when its answer is written")
	recursive := fs.Bool("recursive", false, "read the tree and not just the top")
	fs.Usage = func() { fmt.Fprint(os.Stderr, ocrBatchUsage) }
	if err := fs.Parse(args[2:]); err != nil {
		return err
	}
	if strings.TrimSpace(*ask) == "" {
		return fmt.Errorf("ocr-batch: no prompt, so there is nothing to ask about the pages")
	}

	// Its own process group, so that the kill the run sends when it gives up
	// reaches the readers under this and not only this. On a rented box setsid
	// does that from the shell; here there is no setsid and the tool does it
	// for itself.
	_ = syscall.Setpgid(0, 0)

	images, err := pageImages(in, *ext, *recursive)
	if err != nil {
		return err
	}
	if len(images) == 0 {
		return fmt.Errorf("ocr-batch: no %s images under %s", *ext, in)
	}
	if err := os.MkdirAll(out, 0o755); err != nil {
		return err
	}

	reader := localReader{
		Binary: *binary, Model: *model, Prompt: *ask,
		Timeout: time.Duration(*timeout) * time.Second,
	}
	fmt.Printf("reading %d pages with %s, %d at a time\n", len(images), *model, max(*jobs, 1))
	return reader.all(context.Background(), images, out, max(*jobs, 1), *rate, *skip)
}

// pageImages is every image the batch was given, in page order.
func pageImages(dir, ext string, recursive bool) ([]string, error) {
	want := "." + strings.TrimPrefix(strings.ToLower(ext), ".")
	var out []string
	err := filepath.WalkDir(dir, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if !recursive && path != dir {
				return filepath.SkipDir
			}
			return nil
		}
		if strings.ToLower(filepath.Ext(path)) == want {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

// localReader reads one page image at a time with the Claude Code CLI.
type localReader struct {
	Binary  string
	Model   string
	Prompt  string
	Timeout time.Duration
}

// all reads the pages, up to jobs of them at once, and stops early when the
// account is out of turns.
func (r localReader) all(ctx context.Context, images []string, out string, jobs int, rate float64, skip bool) error {
	ctx, stop := context.WithCancel(ctx)
	defer stop()

	var (
		mu     sync.Mutex
		limit  string // what the CLI said when it refused, empty until it does
		wrote  int
		failed int
	)
	work := make(chan string)
	var wg sync.WaitGroup
	for range jobs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for image := range work {
				name := filepath.Base(image)
				answer := filepath.Join(out, strings.TrimSuffix(name, filepath.Ext(name))+".md")
				if skip {
					if info, err := os.Stat(answer); err == nil && info.Size() > 0 {
						fmt.Printf("%s: already read\n", name)
						continue
					}
				}
				started := time.Now()
				text, err := r.one(ctx, image)
				switch {
				case err != nil && isLimit(err.Error()):
					mu.Lock()
					if limit == "" {
						limit = err.Error()
						// In the words the run above matches on, so that the
						// pages this batch did not reach go back with their
						// attempts intact rather than as four failures.
						fmt.Printf("%s: %s for now, stopping the batch: %s\n",
							name, ocr.OutOfTurnsMark, condenseLine(err.Error()))
						stop()
					}
					mu.Unlock()
				case err != nil:
					mu.Lock()
					failed++
					mu.Unlock()
					fmt.Printf("%s: %v\n", name, err)
				default:
					if err := writeAnswer(answer, text); err != nil {
						mu.Lock()
						failed++
						mu.Unlock()
						fmt.Printf("%s: could not write the answer: %v\n", name, err)
						continue
					}
					mu.Lock()
					wrote++
					mu.Unlock()
					fmt.Printf("%s: %d bytes in %s\n", name, len(text), time.Since(started).Round(time.Second))
				}
			}
		}()
	}
	for _, image := range images {
		select {
		case <-ctx.Done():
		case work <- image:
			if rate > 0 {
				time.Sleep(time.Duration(rate * float64(time.Second)))
			}
			continue
		}
		break
	}
	close(work)
	wg.Wait()

	fmt.Printf("%d pages written, %d failed, %d not attempted\n", wrote, failed, len(images)-wrote-failed)
	if limit != "" {
		return fmt.Errorf("ocr-batch: %s", condenseLine(limit))
	}
	return nil
}

// one reads a single page.
//
// The prompt goes in on standard input rather than on the command line, because
// it is several thousand bytes of LaTeX and shells have opinions about both of
// those. The image is named in the last line of it and the CLI is allowed the
// Read tool and nothing else, which is the whole of what reading a page needs.
func (r localReader) one(ctx context.Context, image string) (string, error) {
	path, err := filepath.Abs(image)
	if err != nil {
		return "", err
	}
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()

	ask := strings.TrimSpace(r.Prompt) + "\n\n" +
		"The page image is the file " + path + ". Read it and transcribe it. " +
		"Output only the transcription, starting with the running head line.\n"

	cmd := exec.CommandContext(ctx, r.Binary, "-p", "--model", r.Model, "--allowed-tools", "Read")
	// The timeout has to reach whatever the CLI started as well as the CLI.
	// Killing the process alone leaves its children holding the pipe this reads
	// the answer through, and Wait does not return while anything still holds
	// it: a page that hung was given up on after fifteen minutes and then held
	// its lane for the full half hour the child slept. So the reader gets a
	// process group of its own, the timeout kills the group, and the wait gives
	// up on the pipe shortly after either way.
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
	cmd.WaitDelay = 5 * time.Second
	cmd.Stdin = strings.NewReader(ask)
	var answer, problem bytes.Buffer
	cmd.Stdout = &answer
	cmd.Stderr = &problem
	if err := cmd.Run(); err != nil {
		said := strings.TrimSpace(problem.String())
		if said == "" {
			said = strings.TrimSpace(answer.String())
		}
		if said == "" {
			said = err.Error()
		}
		return "", fmt.Errorf("%s", said)
	}
	text := fence(strings.TrimSpace(answer.String()))
	if text == "" {
		return "", fmt.Errorf("the model returned nothing")
	}
	return text + "\n", nil
}

// fence takes off a code fence wrapping the whole answer.
//
// The prompt asks for the transcription and nothing else, and it is usually
// obeyed, but Markdown asked for as an answer comes back in a fence often
// enough that leaving it in would put ``` at the head of a page file and a
// stray one at the foot of the next.
func fence(text string) string {
	if !strings.HasPrefix(text, "```") {
		return text
	}
	first, rest, ok := strings.Cut(text, "\n")
	if !ok || strings.Contains(strings.TrimPrefix(first, "```"), "`") {
		return text
	}
	end := strings.LastIndex(rest, "```")
	if end < 0 {
		return text
	}
	return strings.TrimSpace(rest[:end])
}

// writeAnswer renames the finished page into place.
//
// The run above counts the Markdown files in this directory to know how far the
// batch has got, so a file that exists and is half written is an answer it will
// take. The temporary name ends in .part rather than .md for the same reason:
// the count is of names ending in .md.
func writeAnswer(path, text string) error {
	tmp := path + ".part"
	if err := os.WriteFile(tmp, []byte(text), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func isLimit(said string) bool {
	lower := strings.ToLower(said)
	for _, phrase := range limitPhrases {
		if strings.Contains(lower, phrase) {
			return true
		}
	}
	return false
}
