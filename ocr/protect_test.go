package ocr

import (
	"fmt"
	"path/filepath"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/queue"
)

// A page that reads clean, so that what these tests turn on is the guard and
// not the rules. Long enough to clear the short rule, with a head and balanced
// mathematics.
const cleanPage = `## § 1. DISTRIBUTIVITY FORMULAE

COROLLARY. Let $(X_\iota)_{\iota \in I}$ and $(Y_\varkappa)_{\varkappa \in K}$ be two
families of sets with non-empty index sets $I, K$. Then the canonical bijection of
$\coprod_{\lambda \in L} J_\lambda$ onto $I \times K$ carries one to the other, and
this is what the corollary asserts about the two families and their index sets.

$$\left( \bigcap_{\iota \in I} X_\iota \right) \cup \left( \bigcap_{\varkappa \in K} Y_\varkappa \right) = \bigcap_{(\iota, \varkappa) \in I \times K} (X_\iota \cup Y_\varkappa).$$

The proof is the one given for Proposition 8 above, applied to the family of
families obtained by indexing the two given families over a two element set.
`

// write puts a reading on disk where accepted will look for it and hands back
// the runner that will look.
func write(t *testing.T, model, promptSHA, inputSHA string) (*Runner, Source) {
	t.Helper()
	root := t.TempDir()
	path := corpus.PagePath(root, "ens-i-iv", 116)
	if err := (corpus.PageFile{Meta: corpus.PageFrontMatter{
		Book: "ens-i-iv", PDFPage: 116, Method: corpus.MethodOCR,
		RunningHead: "DISTRIBUTIVITY FORMULAE", Model: model,
		PromptSHA256: promptSHA, InputSHA256: inputSHA, Lines: 12,
	}, Body: cleanPage}).Write(path); err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != "0116.md" {
		t.Fatalf("page path is %q, the test is looking in the wrong place", path)
	}
	return &Runner{Book: "ens-i-iv", Root: root}, Source{Page: 116, SHA256: inputSHA}
}

// The whole of what this guard is for. The prompt moved, so the page is stale
// by the old rule, and the reading on disk came off a model the card does not
// match, so reading it again is a downgrade and not a repair.
func TestAPromptChangeDoesNotThrowAwayAStrongerReading(t *testing.T) {
	for _, model := range []string{"claude-opus", "gpt-5", "gpt-4o", "o3-mini"} {
		t.Run(model, func(t *testing.T) {
			r, source := write(t, model, "old-prompt", "same-image")
			if r.state(source, "new-prompt") != alreadyRead {
				t.Error("the page went back into the queue and the card would read over it")
			}
		})
	}
}

// The other half. A prompt change is new work for a page the local readers
// read, because that is the case the prompt was changed for.
func TestAPromptChangeStillRereadsALocalReading(t *testing.T) {
	for _, model := range []string{"olmOCR-2-7B-1025-FP8", "MinerU2.5-2509-1.2B", ""} {
		t.Run(model, func(t *testing.T) {
			r, source := write(t, model, "old-prompt", "same-image")
			if r.state(source, "new-prompt") != unread {
				t.Error("a stale local reading was kept and the new prompt never ran")
			}
		})
	}
}

// A page that nothing has changed about is accepted whoever read it, which is
// the behaviour that was there before the guard and has to stay.
func TestAnUnchangedPageIsStillAccepted(t *testing.T) {
	r, source := write(t, "olmOCR-2-7B-1025-FP8", "same-prompt", "same-image")
	if r.state(source, "same-prompt") != alreadyRead {
		t.Error("a page that nothing changed about was queued again")
	}
}

// A changed image is guarded the same way a changed prompt is. This started out
// the other way round, on the reasoning that a page rendered again at 600 dpi is
// a better picture and deserves a fresh reading, and that let the card read over
// gpt-5 on sixteen pages of Theory of Sets. The re-render was not a resolution
// change at all, it was the same page at the same settings after the images
// directory was swept, and the bytes differ for reasons that have nothing to do
// with how well the page can be read.
func TestAFreshRenderDoesNotThrowAwayAStrongerReading(t *testing.T) {
	r, source := write(t, "claude-opus", "same-prompt", "the-old-render")
	source.SHA256 = "the-new-render"
	if r.state(source, "same-prompt") != alreadyRead {
		t.Error("a re-render sent a stronger reading back to the card")
	}
}

// Both inputs moving at once is the case the sweep actually produced, since the
// prompt had changed as well, and it has to be guarded like either one alone.
func TestANewRenderAndANewPromptTogetherAreStillGuarded(t *testing.T) {
	r, source := write(t, "gpt-5", "old-prompt", "the-old-render")
	source.SHA256 = "the-new-render"
	if r.state(source, "new-prompt") != alreadyRead {
		t.Error("two changed inputs got past a guard that either one alone would hit")
	}
}

// The other half again. A re-render is a fresh reading for a page the local
// readers read, because there is nothing there worth keeping.
func TestAFreshRenderStillRereadsALocalReading(t *testing.T) {
	r, source := write(t, "olmOCR-2-7B-1025-FP8", "same-prompt", "the-old-render")
	source.SHA256 = "the-new-render"
	if r.state(source, "same-prompt") != unread {
		t.Error("a local reading of an image that is gone was kept")
	}
}

// After a deliberate re-render at a higher resolution, this is the flag that
// says so, and it has to reach the image hash and not only the prompt.
func TestRereadProtectedAlsoCoversAReRender(t *testing.T) {
	r, source := write(t, "claude-opus", "same-prompt", "the-300-dpi-image")
	source.SHA256 = "the-600-dpi-image"
	r.RereadProtected = true
	if r.state(source, "same-prompt") != unread {
		t.Error("the guard held on a re-render with RereadProtected set")
	}
}

// rejected puts a reading on disk that the rules will not pass, so that what
// these tests turn on is who reads it next.
func rejected(t *testing.T, model string) (string, Source) {
	t.Helper()
	root := t.TempDir()
	if err := (corpus.PageFile{Meta: corpus.PageFrontMatter{
		Book: "ens-i-iv", PDFPage: 116, Method: corpus.MethodOCR,
		RunningHead: "DISTRIBUTIVITY FORMULAE", Model: model,
		PromptSHA256: "old-prompt", InputSHA256: "same-image", Lines: 1,
	}, Body: "short"}).Write(corpus.PagePath(root, "ens-i-iv", 116)); err != nil {
		t.Fatal(err)
	}
	return root, Source{Page: 116, SHA256: "same-image"}
}

// The rules are still the last word for a run that can do something about
// them. A protected reading that does not pass them is work still to do,
// exactly as an unprotected one is, because the point of the rules is that
// written is not the same as read.
func TestAProtectedReadingStillHasToPassTheRules(t *testing.T) {
	root, source := rejected(t, "claude-opus")
	r := &Runner{Book: "ens-i-iv", Root: root, Model: "gpt-5"}
	if r.state(source, "new-prompt") != unread {
		t.Error("a reading that fails the rules was kept from a reader that could improve it")
	}
}

// The 142 pages. A run holding only the local card found a rejected page that
// claude-opus had read, queued it because it was rejected, and wrote olmOCR
// over the top. The page is still work and this run is still not the one to do
// it.
func TestARejectedReadingIsLeftToAReaderThatCanBeatIt(t *testing.T) {
	for _, model := range []string{"claude-opus", "gpt-5", "gpt-4o", "o3-mini"} {
		t.Run(model, func(t *testing.T) {
			root, source := rejected(t, model)
			r := &Runner{Book: "ens-i-iv", Root: root, Model: "gpt-5",
				Hosts: []Host{{Name: "gamingpc", Model: "olmOCR-2-7B-1025-FP8"}}}
			if r.state(source, "new-prompt") != needsABetterReader {
				t.Error("the card was sent a page it could only make worse")
			}
		})
	}
}

// One protected host in the run is enough to queue the page, because the run
// has somebody who can beat the reading on it. Which of the two hosts actually
// picks it up is decided in lease, by strongEnough, and the tests for that are
// at the bottom of this file.
func TestOneProtectedHostIsEnoughToQueueARejectedPage(t *testing.T) {
	root, source := rejected(t, "claude-opus")
	r := &Runner{Book: "ens-i-iv", Root: root, Model: "gpt-5",
		Hosts: []Host{{Name: "gamingpc", Model: "olmOCR-2-7B-1025-FP8"}, {Name: "server3"}}}
	if r.state(source, "new-prompt") != unread {
		t.Error("a run holding a protected host walked away from a page it could have read")
	}
}

// A rejected local reading is work for anybody, which is the behaviour that was
// there before and has to stay: the guard is about not going backwards, not
// about leaving weak readings alone.
func TestARejectedLocalReadingIsStillReadAgain(t *testing.T) {
	root, source := rejected(t, "olmOCR-2-7B-1025-FP8")
	r := &Runner{Book: "ens-i-iv", Root: root, Model: "gpt-5",
		Hosts: []Host{{Name: "gamingpc", Model: "olmOCR-2-7B-1025-FP8"}}}
	if r.state(source, "new-prompt") != unread {
		t.Error("a weak reading was left alone by a run that could redo it")
	}
}

// RereadProtected reaches this guard too, since it is the operator saying the
// old readings are the thing that is wrong.
func TestRereadProtectedAlsoQueuesARejectedProtectedReading(t *testing.T) {
	root, source := rejected(t, "claude-opus")
	r := &Runner{Book: "ens-i-iv", Root: root, Model: "gpt-5",
		Hosts:           []Host{{Name: "gamingpc", Model: "olmOCR-2-7B-1025-FP8"}},
		RereadProtected: true}
	if r.state(source, "new-prompt") != unread {
		t.Error("the guard held with RereadProtected set")
	}
}

// The escape hatch, for the case where the prompt changed because the old
// readings were wrong and walking over them is the whole point.
func TestRereadProtectedTurnsTheGuardOff(t *testing.T) {
	r, source := write(t, "claude-opus", "old-prompt", "same-image")
	r.RereadProtected = true
	if r.state(source, "new-prompt") != unread {
		t.Error("the guard held with RereadProtected set")
	}
}

// The other half of the guard, the one that decides who writes.
//
// state runs once while the queue is being filled and asks about the run.
// strongEnough runs once per host per lease and asks about the host, which is
// the question the write actually turns on. The two tests above leave a
// rejected gpt-5 page queued for a run holding gamingpc and server3 together;
// these say which of them is allowed to have it.

// gamingpc is the weak host in all of this: a card serving olmOCR, which is
// what wrote over Theory of Sets.
var weakHost = Host{Name: "gamingpc", Model: "olmOCR-2-7B-1025-FP8"}

// server3 carries no model of its own and takes the run's, which is gpt-5.
var strongHost = Host{Name: "server3"}

// The whole point of the change. The page is queued because the run has a
// reader that can beat it, and the card is not that reader.
func TestTheWeakHostIsNotOfferedAPageAStrongReaderWrote(t *testing.T) {
	for _, model := range []string{"claude-opus", "gpt-5", "gpt-4o", "o3-mini"} {
		t.Run(model, func(t *testing.T) {
			root, _ := rejected(t, model)
			r := &Runner{Book: "ens-i-iv", Root: root, Model: "gpt-5",
				Hosts: []Host{weakHost, strongHost}}
			if r.strongEnough(weakHost, "ens-i-iv/0116") {
				t.Error("the card was offered a page it could only make worse")
			}
		})
	}
}

// And the page is not stranded by that. The host the queue was filled for gets
// it, which is why refusing the card is safe: LeasePart leaves a target its
// predicate turns down in pending for the next host to ask.
func TestTheStrongHostIsOfferedThatSamePage(t *testing.T) {
	root, _ := rejected(t, "claude-opus")
	r := &Runner{Book: "ens-i-iv", Root: root, Model: "gpt-5",
		Hosts: []Host{weakHost, strongHost}}
	if !r.strongEnough(strongHost, "ens-i-iv/0116") {
		t.Error("the page was kept from the reader the run queued it for")
	}
}

// A weak reading is work for anybody, the same way it is in state. The guard is
// about not going backwards, not about leaving pages alone.
func TestAWeakReadingIsOfferedToTheWeakHost(t *testing.T) {
	root, _ := rejected(t, "olmOCR-2-7B-1025-FP8")
	r := &Runner{Book: "ens-i-iv", Root: root, Model: "gpt-5",
		Hosts: []Host{weakHost, strongHost}}
	if !r.strongEnough(weakHost, "ens-i-iv/0116") {
		t.Error("the card was refused a page nothing better had read")
	}
}

// A page with no file behind it is a page nobody has read, and reading it
// cannot be a downgrade. This is most of what a sweep does and it must not slow
// down for the guard.
func TestAPageWithNoFileIsOfferedToTheWeakHost(t *testing.T) {
	root, _ := rejected(t, "claude-opus")
	r := &Runner{Book: "ens-i-iv", Root: root, Model: "gpt-5",
		Hosts: []Host{weakHost, strongHost}}
	if !r.strongEnough(weakHost, "ens-i-iv/0400") {
		t.Error("an unread page was held back from the only host reading anything")
	}
}

// A page whose text came out of the PDF rather than off an image is not an OCR
// reading and has no model to rank, so the guard has nothing to say about it.
func TestANativePageIsOfferedToTheWeakHost(t *testing.T) {
	root := t.TempDir()
	if err := (corpus.PageFile{Meta: corpus.PageFrontMatter{
		Book: "ens-i-iv", PDFPage: 116, Method: corpus.MethodNative,
		RunningHead: "DISTRIBUTIVITY FORMULAE", Lines: 1,
	}, Body: "short"}).Write(corpus.PagePath(root, "ens-i-iv", 116)); err != nil {
		t.Fatal(err)
	}
	r := &Runner{Book: "ens-i-iv", Root: root, Model: "gpt-5", Hosts: []Host{weakHost}}
	if !r.strongEnough(weakHost, "ens-i-iv/0116") {
		t.Error("a native page was ranked as though a reader had written it")
	}
}

// RereadProtected reaches this guard like it reaches the other two, because it
// is the operator saying the old readings are the thing that is wrong.
func TestRereadProtectedOffersTheProtectedPageToTheWeakHost(t *testing.T) {
	root, _ := rejected(t, "claude-opus")
	r := &Runner{Book: "ens-i-iv", Root: root, Model: "gpt-5",
		Hosts: []Host{weakHost}, RereadProtected: true}
	if !r.strongEnough(weakHost, "ens-i-iv/0116") {
		t.Error("the guard held with RereadProtected set")
	}
}

// A target the guard cannot read is passed through rather than held, so that a
// malformed job fails where malformed jobs already fail, in one, with a reason
// written on it. Holding it here would leave it in pending with nothing ever
// looking at it again.
func TestAMalformedTargetIsNotHeldByTheGuard(t *testing.T) {
	root, _ := rejected(t, "claude-opus")
	r := &Runner{Book: "ens-i-iv", Root: root, Model: "gpt-5", Hosts: []Host{weakHost}}
	for _, target := range []string{"ens-i-iv", "ens-i-iv/", "ens-i-iv/cover", "ens-i-iv/0000"} {
		if !r.strongEnough(weakHost, target) {
			t.Errorf("target %q was left in the queue for nobody", target)
		}
	}
}

// The accounting. A page the card was refused is normally read by the stronger
// host in the same run, so it must not turn up in the report as work left
// undone. The run works that out by asking the queue at the end, and this is
// both halves of the answer: the page still pending is named, the page that
// went through is not.
func TestOnlyTheGuardedPagesStillInTheQueueAreReported(t *testing.T) {
	root, _ := rejected(t, "claude-opus")
	q, err := queue.Open(filepath.Join(root, "work", "queue"))
	if err != nil {
		t.Fatal(err)
	}
	r := &Runner{Book: "ens-i-iv", Root: root, Model: "gpt-5", Queue: q,
		Hosts: []Host{weakHost, strongHost}}

	// Page 116 is still waiting, page 200 was read by somebody and is gone.
	if _, err := q.Add(queue.New(queue.StageOCR, Target("ens-i-iv", 116), "same-image", "old-prompt")); err != nil {
		t.Fatal(err)
	}
	r.passOver(Target("ens-i-iv", 116))
	r.passOver(Target("ens-i-iv", 200))

	left := r.leftBehind()
	if len(left) != 1 || left[0] != 116 {
		t.Fatalf("the run reported %v left behind, wanted just page 116", left)
	}
}

// A run that guarded nothing says nothing, and does not go to the queue to find
// that out.
func TestARunThatGuardedNothingReportsNothing(t *testing.T) {
	r := &Runner{Book: "ens-i-iv", Root: t.TempDir(), Model: "gpt-5"}
	if left := r.leftBehind(); left != nil {
		t.Errorf("a run with no guarded pages reported %v", left)
	}
}

// salvagedPage puts a reading on disk that was kept with a fault on it, which
// is what the third attempt writes when Salvage is on.
func salvagedPage(t *testing.T, model string) string {
	t.Helper()
	root := t.TempDir()
	if err := (corpus.PageFile{Meta: corpus.PageFrontMatter{
		Book: "lie-i-iii", PDFPage: 252, Method: corpus.MethodOCR,
		RunningHead: "LIE GROUPS", Model: model,
		PromptSHA256: "old-prompt", InputSHA256: "same-image", Lines: 40,
		Flags: []string{
			"rendered at 300 dpi on attempt 3",
			"read 3 times and never came back clean, so it is kept with the faults rule statement names still on it, see bourbaki ocr check",
		},
	}, Body: "short"}).Write(corpus.PagePath(root, "lie-i-iii", 252)); err != nil {
		t.Fatal(err)
	}
	return root
}

// The pass that came twenty minutes after lie-i-iii was committed. The card
// salvaged 21 pages, the next run found them still failing the rules, read 18
// of them again on the same images and lost mathematics doing it.
func TestASalvagedPageIsNotReadAgainByTheSameClassOfReader(t *testing.T) {
	root := salvagedPage(t, "olmOCR-2-7B-1025-FP8")
	r := &Runner{Book: "lie-i-iii", Root: root, Model: "olmOCR-2-7B-1025-FP8",
		Hosts: []Host{weakHost}}
	if r.state(Source{Page: 252, SHA256: "same-image"}, "old-prompt") != needsABetterReader {
		t.Error("the card was sent back to a page it had already had three goes at")
	}
}

// And the batch guard says the same thing, so a mixed run does not hand it to
// the card either.
func TestASalvagedPageIsNotOfferedToTheWeakHost(t *testing.T) {
	root := salvagedPage(t, "olmOCR-2-7B-1025-FP8")
	r := &Runner{Book: "lie-i-iii", Root: root, Model: "gpt-5",
		Hosts: []Host{weakHost, strongHost}}
	if r.strongEnough(weakHost, "lie-i-iii/0252") {
		t.Error("a salvaged page was offered to the reader that salvaged it")
	}
	if !r.strongEnough(strongHost, "lie-i-iii/0252") {
		t.Error("a salvaged page was kept from the reader that could actually improve it")
	}
}

// A better reader is the whole point of leaving it alone, so a run that has one
// still queues the page.
func TestASalvagedPageIsQueuedForABetterReader(t *testing.T) {
	root := salvagedPage(t, "olmOCR-2-7B-1025-FP8")
	r := &Runner{Book: "lie-i-iii", Root: root, Model: "gpt-5",
		Hosts: []Host{weakHost, strongHost}}
	if r.state(Source{Page: 252, SHA256: "same-image"}, "old-prompt") != unread {
		t.Error("a run holding a stronger reader walked away from a salvaged page")
	}
}

// A page that was written because it read clean is not salvaged and carries no
// such flag, so nothing here applies to it.
func TestAnOrdinaryRejectedPageIsUnaffectedBySalvageGuard(t *testing.T) {
	root, source := rejected(t, "olmOCR-2-7B-1025-FP8")
	r := &Runner{Book: "ens-i-iv", Root: root, Model: "olmOCR-2-7B-1025-FP8",
		Hosts: []Host{weakHost}}
	if r.state(source, "old-prompt") != unread {
		t.Error("a plain rejected reading was treated as salvaged")
	}
}

// RereadProtected reaches this guard like it reaches the other three.
func TestRereadProtectedReadsASalvagedPageAgain(t *testing.T) {
	root := salvagedPage(t, "olmOCR-2-7B-1025-FP8")
	r := &Runner{Book: "lie-i-iii", Root: root, Model: "olmOCR-2-7B-1025-FP8",
		Hosts: []Host{weakHost}, RereadProtected: true}
	if r.state(Source{Page: 252, SHA256: "same-image"}, "old-prompt") != unread {
		t.Error("the guard held with RereadProtected set")
	}
}

// The flag written by write and the mark read by salvagedReading have to stay
// in step, so the wording that is actually emitted is checked against it rather
// than a copy of it.
func TestTheSalvageFlagCarriesTheMarkThatIsMatched(t *testing.T) {
	flag := fmt.Sprintf(
		"read %d times and never came back clean, so it is kept with the faults rule %s names still on it, see bourbaki ocr check",
		3, ruleList([]Rule{RuleStatement}))
	if !salvagedReading(corpus.PageFrontMatter{Flags: []string{flag}}) {
		t.Errorf("the flag write emits does not match the mark salvagedReading looks for: %q", flag)
	}
}
