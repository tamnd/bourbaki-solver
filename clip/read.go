package clip

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/tamnd/bourbaki-solver/ocr"
)

// DefaultBatch is how many clips go to a host in one call of the tool.
//
// Smaller than a page batch on purpose. A clip is a minute and a page is
// several, so a batch of twenty five clips is about the same wall clock as a
// batch of pages, and a batch that dies loses less. Twelve leaves both boxes
// something to do on a run of two dozen, which is the size an audit of one
// fault comes to.
const DefaultBatch = 12

// Fleet reads a directory of clips on the rented boxes.
//
// It is deliberately not the OCR runner. That one is built around a durable
// queue, leases, seven validation rules and a repair conversation, because a
// page is expensive and a run of 734 of them has to survive a dropped laptop
// lid. A clip run is two dozen images that either come back or do not, and the
// answer is judged by comparing it with the extractor rather than by rules. The
// one thing worth borrowing is the transport, and that is what this borrows.
type Fleet struct {
	Hosts  []ocr.Host
	Shell  ocr.Shell
	Copy   ocr.Copier
	Prompt string
	// Dir is where the clips are, Dest where the Markdown is pulled back to.
	Dir  string
	Dest string
	// ID is the stem of the batch names on the hosts. Each batch gets a number
	// and a digest of its images after it, so no two batches of one run and no
	// two runs of one volume ever share a remote directory.
	ID    string
	Batch int
	Keep  bool
	Logf  func(string, ...any)
	// Sleep is handed on to each batch, so a test does not wait a real poll.
	Sleep func(ctx context.Context, d time.Duration) error
	Poll  time.Duration
}

func (f Fleet) batch() int {
	if f.Batch > 0 {
		return f.Batch
	}
	return DefaultBatch
}

func (f Fleet) logf(format string, args ...any) {
	if f.Logf != nil {
		f.Logf(format, args...)
	}
}

// Pending is the clips of an index that have no answer yet.
//
// A run that was interrupted, or a host that dropped half a batch, should cost
// only what is missing. The names are the index's, so a clip that was never cut
// is reported here rather than silently skipped: an answer for a picture that
// does not exist is not a thing this can wait for.
func Pending(index Index, dir, dest string) ([]string, error) {
	var out []string
	for _, target := range index.Targets {
		image := filepath.Join(dir, target.Name)
		if _, err := os.Stat(image); err != nil {
			return nil, fmt.Errorf("clip %s was never cut: %w", target.Name, err)
		}
		answer := filepath.Join(dest, answerName(target.Name))
		if info, err := os.Stat(answer); err == nil && info.Size() > 0 {
			continue
		}
		out = append(out, image)
	}
	return out, nil
}

// batchID is the name of one batch's directories on a host.
//
// The digest of the images is in it, and it has to be. The first version named
// them <book>-clip-000 and the second run of the same volume reused the name:
// the previous run's twelve answers were still sitting in that directory on the
// box, the poll counted them, decided a batch of four had finished before it
// had started, and pulled back twelve answers to lines nobody had asked about
// while all seven pages came home missing. That cost a run. The page runner has
// carried a digest in its batch names from the beginning for the same reason.
//
// It is not the whole of the fix and was never going to be. The same pictures
// hash the same, so the same seven pages sent again under a rewritten prompt
// got the same name and the same stale answers, and the second run cost as much
// as the first. What stops it is ocr.Batch emptying the answers directory
// before it starts, and what this does is keep two batches of one run apart.
func batchID(stem string, index int, images []string) string {
	sum := sha256.New()
	for _, image := range images {
		fmt.Fprintln(sum, filepath.Base(image))
	}
	return fmt.Sprintf("%s-%03d-%s", stem, index, hex.EncodeToString(sum.Sum(nil))[:6])
}

// Read sends the clips to the hosts and returns what each batch cost.
//
// One goroutine per host, each taking the next batch off the list when it has
// finished the last one. There is no scheduler, the same as the page runner: a
// box that is quick comes back for more sooner, and a box that dies stops
// taking work rather than holding any.
func (f Fleet) Read(ctx context.Context, images []string) ([]ocr.Result, error) {
	if len(images) == 0 {
		return nil, nil
	}
	if len(f.Hosts) == 0 {
		return nil, fmt.Errorf("no host can read clips")
	}
	if err := os.MkdirAll(f.Dest, 0o755); err != nil {
		return nil, err
	}

	var batches [][]string
	for start := 0; start < len(images); start += f.batch() {
		batches = append(batches, images[start:min(start+f.batch(), len(images))])
	}
	f.logf("%d clips in %d batches over %d hosts", len(images), len(batches), len(f.Hosts))

	var (
		lock    sync.Mutex
		group   sync.WaitGroup
		next    int
		results []ocr.Result
		failed  error
	)
	for _, host := range f.Hosts {
		if host.Lanes <= 0 {
			f.logf("%s takes no clips, skipping it", host.Name)
			continue
		}
		group.Add(1)
		go func(host ocr.Host) {
			defer group.Done()
			for {
				if ctx.Err() != nil {
					return
				}
				lock.Lock()
				if next >= len(batches) {
					lock.Unlock()
					return
				}
				mine, index := batches[next], next
				next++
				lock.Unlock()

				batch := ocr.Batch{
					Host: host, ID: batchID(f.ID, index, mine),
					Images: mine, Prompt: f.Prompt, Dest: f.Dest,
					Shell: f.Shell, Copy: f.Copy, Keep: f.Keep,
					Poll: f.Poll, Sleep: f.Sleep, Logf: f.Logf,
				}
				result, err := batch.Run(ctx)
				lock.Lock()
				results = append(results, result)
				if err != nil {
					f.logf("%s: %v", host.Name, err)
					// The first failure is the one reported. A run that is
					// half done is still worth auditing, so the other host
					// keeps going and the error comes back at the end.
					if failed == nil {
						failed = err
					}
				}
				lock.Unlock()
				f.logf("%s", result.Summary())
			}
		}(host)
	}
	group.Wait()
	return results, failed
}
