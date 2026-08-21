package ocr

import (
	"path/filepath"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
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
			if !r.accepted(source, "new-prompt") {
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
			if r.accepted(source, "new-prompt") {
				t.Error("a stale local reading was kept and the new prompt never ran")
			}
		})
	}
}

// A page that nothing has changed about is accepted whoever read it, which is
// the behaviour that was there before the guard and has to stay.
func TestAnUnchangedPageIsStillAccepted(t *testing.T) {
	r, source := write(t, "olmOCR-2-7B-1025-FP8", "same-prompt", "same-image")
	if !r.accepted(source, "same-prompt") {
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
	if !r.accepted(source, "same-prompt") {
		t.Error("a re-render sent a stronger reading back to the card")
	}
}

// Both inputs moving at once is the case the sweep actually produced, since the
// prompt had changed as well, and it has to be guarded like either one alone.
func TestANewRenderAndANewPromptTogetherAreStillGuarded(t *testing.T) {
	r, source := write(t, "gpt-5", "old-prompt", "the-old-render")
	source.SHA256 = "the-new-render"
	if !r.accepted(source, "new-prompt") {
		t.Error("two changed inputs got past a guard that either one alone would hit")
	}
}

// The other half again. A re-render is a fresh reading for a page the local
// readers read, because there is nothing there worth keeping.
func TestAFreshRenderStillRereadsALocalReading(t *testing.T) {
	r, source := write(t, "olmOCR-2-7B-1025-FP8", "same-prompt", "the-old-render")
	source.SHA256 = "the-new-render"
	if r.accepted(source, "same-prompt") {
		t.Error("a local reading of an image that is gone was kept")
	}
}

// After a deliberate re-render at a higher resolution, this is the flag that
// says so, and it has to reach the image hash and not only the prompt.
func TestRereadProtectedAlsoCoversAReRender(t *testing.T) {
	r, source := write(t, "claude-opus", "same-prompt", "the-300-dpi-image")
	source.SHA256 = "the-600-dpi-image"
	r.RereadProtected = true
	if r.accepted(source, "same-prompt") {
		t.Error("the guard held on a re-render with RereadProtected set")
	}
}

// The rules are still the last word. A protected reading that does not pass
// them is work still to do, exactly as an unprotected one is, because the point
// of the rules is that written is not the same as read.
func TestAProtectedReadingStillHasToPassTheRules(t *testing.T) {
	root := t.TempDir()
	if err := (corpus.PageFile{Meta: corpus.PageFrontMatter{
		Book: "ens-i-iv", PDFPage: 116, Method: corpus.MethodOCR,
		RunningHead: "DISTRIBUTIVITY FORMULAE", Model: "claude-opus",
		PromptSHA256: "old-prompt", InputSHA256: "same-image", Lines: 1,
	}, Body: "short"}).Write(corpus.PagePath(root, "ens-i-iv", 116)); err != nil {
		t.Fatal(err)
	}
	r := &Runner{Book: "ens-i-iv", Root: root}
	if r.accepted(Source{Page: 116, SHA256: "same-image"}, "new-prompt") {
		t.Error("a reading that fails the rules was kept because of who wrote it")
	}
}

// The escape hatch, for the case where the prompt changed because the old
// readings were wrong and walking over them is the whole point.
func TestRereadProtectedTurnsTheGuardOff(t *testing.T) {
	r, source := write(t, "claude-opus", "old-prompt", "same-image")
	r.RereadProtected = true
	if r.accepted(source, "new-prompt") {
		t.Error("the guard held with RereadProtected set")
	}
}
