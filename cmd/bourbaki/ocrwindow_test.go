package main

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/queue"
	"github.com/tamnd/bourbaki-solver/render"
)

// The 320 pages.
//
// The driver rendered a window, called ocr run over that range, and wrote the
// page after it into a cursor file. A window that died part way through advanced
// the cursor anyway, so a page it had never leased stayed pending in the queue
// with the cursor already past it and nothing coming back. hist was the worst of
// them: 130 pages still pending, between 1 and 299, behind a cursor at 300, in a
// volume that read as finished.
//
// Nothing here counts a cursor. A page is outstanding when the queue is waiting
// on it or when the corpus has no file for it, and that is a question with an
// answer whatever happened during the last run.
func TestAPageTheLastWindowNeverLeasedIsStillOutstanding(t *testing.T) {
	state, root := windowSetup(t, 10)

	// Page 3 was read and committed. Page 5 was queued and never leased, which
	// is the page the cursor used to lose. The rest were never touched.
	writePage(t, root, "hist", 3, "Un ensemble.\n")
	queuePage(t, state, 5)

	got, err := state.outstanding(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	want := []int{1, 2, 4, 5, 6, 7, 8, 9, 10}
	if !sameInts(got, want) {
		t.Errorf("outstanding = %v, want %v", got, want)
	}
}

// Dead is the queue's own answer that it has stopped trying, and a windowed run
// that took those back would render, ship and refuse the same pages until
// somebody killed it. bourbaki queue retry is the door back and it is a
// deliberate one.
func TestAPageTheQueueHasGivenUpOnIsNotWorkAWindowPicksUp(t *testing.T) {
	state, root := windowSetup(t, 4)
	for _, page := range []int{1, 2, 3, 4} {
		writePage(t, root, "hist", page, "Un ensemble.\n")
	}
	job := queuePage(t, state, 2)
	for range queue.DefaultMaxAttempts {
		leased, err := state.queue.Lease(queue.StageOCR, "box", "", 0)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := state.queue.Fail(leased, "the reader would not return it"); err != nil {
			t.Fatal(err)
		}
	}
	if _, found, err := state.queue.Find(queue.StageOCR, job.ID); err != nil {
		t.Fatal(err)
	} else if found != queue.Dead {
		t.Fatalf("the job is %s after %d failures, want dead", found, queue.DefaultMaxAttempts)
	}

	got, err := state.outstanding(0, 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("outstanding = %v, want nothing: every page is committed and the one job is dead", got)
	}
}

// The range still applies on top of the queue, because reading ten pages of a
// volume before turning the fleet loose on it is what -f and -l are for.
func TestAWindowedRunStaysInsideTheRangeItWasGiven(t *testing.T) {
	state, _ := windowSetup(t, 20)
	got, err := state.outstanding(5, 8)
	if err != nil {
		t.Fatal(err)
	}
	if !sameInts(got, []int{5, 6, 7, 8}) {
		t.Errorf("outstanding(5, 8) = %v, want 5 to 8", got)
	}
}

// A volume whose entry does not say how many pages it has cannot be cut into
// windows, and the message says which file to put the number in rather than
// reporting an empty volume.
func TestAVolumeThatDoesNotSayHowLongItIsCannotBeWindowed(t *testing.T) {
	state, _ := windowSetup(t, 0)
	_, err := state.outstanding(0, 0)
	if err == nil {
		t.Fatal("a volume with no page count was windowed anyway")
	}
	if !strings.Contains(err.Error(), "books.yaml") {
		t.Errorf("the refusal does not name the manifest to fix: %v", err)
	}
}

// windowSetup is a scanned volume of n pages with nothing read and nothing
// queued, which is where a windowed run starts.
func windowSetup(t *testing.T, pages int) (setup, string) {
	t.Helper()
	book := corpus.Book{ID: "hist", Nature: "scan", Extraction: "ocr", Pages: pages}
	root := setupCorpus(t, book, render.Manifest{Book: "hist"}, nil)
	state, err := ocrSetupFor("hist", filepath.Join(t.TempDir(), "queue"), false, false, false)
	if err != nil {
		t.Fatalf("ocrSetupFor: %v", err)
	}
	return state, root
}

func queuePage(t *testing.T, state setup, page int) queue.Job {
	t.Helper()
	job := queue.New(queue.StageOCR, ocr.Target(state.entry.ID, page), "image", "prompt")
	if ok, err := state.queue.Add(job); err != nil {
		t.Fatal(err)
	} else if !ok {
		t.Fatalf("page %d was not added to the queue", page)
	}
	return job
}

func sameInts(got, want []int) bool {
	if len(got) != len(want) {
		return false
	}
	for i := range got {
		if got[i] != want[i] {
			return false
		}
	}
	return true
}
