package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/render"
)

// The page files are the only witness, so the walk has to read them the way the
// extraction wrote them: the method off the front matter, the blanks out of the
// checked count, and the rules run through the same expectFor and checkText the
// acceptance decision uses.
func TestExtractionOfCountsWhatIsOnDisk(t *testing.T) {
	book := corpus.Book{ID: "alg-viii", Lang: "en", TextLayer: "native", Pages: 6}
	root := setupCorpus(t, book, render.Manifest{Book: book.ID}, nil)

	dir := corpus.PagesDir(root, book.ID)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	write := func(name, body string) {
		t.Helper()
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	// A page long enough and balanced enough to pass, a blank, a page nobody
	// could read, and a page short enough for the short rule to reject.
	//
	// The last two sit past ocr.FrontLeaves on purpose. Rule 1 does not run on
	// the opening leaves of a volume, because a cover has no length to hold it
	// to, so a fixture that wants to be rejected for being short has to be a
	// page of the book.
	write("0001.md", "---\nbook: alg-viii\npdf_page: 1\nmethod: native\nlines: 40\n---\n\n"+
		"A VIII.1  STRUCTURE OF SEMISIMPLE RINGS  § 1\n\n"+longEnough)
	write("0002.md", "---\nbook: alg-viii\npdf_page: 2\nmethod: blank\nlines: 0\n---\n")
	write("0005.md", "---\nbook: alg-viii\npdf_page: 5\nmethod: ocr-failed\nlines: 0\n---\n\nnothing\n")
	write("0006.md", "---\nbook: alg-viii\npdf_page: 6\nmethod: ocr\nlines: 1\nflags:\n  - diagram\nmanual: true\n---\n\nshort\n")

	got, err := extractionOf(root, book)
	if err != nil {
		t.Fatal(err)
	}
	if got.Read != 4 {
		t.Errorf("read %d, want 4", got.Read)
	}
	// Two of the six pages of the PDF have no file at all, and that is the
	// number ocr check cannot see.
	if got.Unread() != 2 {
		t.Errorf("unread %d, want 2", got.Unread())
	}
	// The blank is read and not checked. A blank page that has been looked at
	// is done, and there is nothing on it for a rule to hold an opinion about.
	if got.Checked != 3 {
		t.Errorf("checked %d, want 3", got.Checked)
	}
	if got.Methods["native"] != 1 || got.Methods["blank"] != 1 || got.Methods["ocr"] != 1 {
		t.Errorf("methods %v", got.Methods)
	}
	if len(got.Failed) != 1 || got.Failed[0] != 5 {
		t.Errorf("ocr-failed pages %v, want [5]", got.Failed)
	}
	if len(got.Flagged) != 1 || got.Manual != 1 {
		t.Errorf("flagged %v, by hand %d", got.Flagged, got.Manual)
	}
	if len(got.Rejected) != 2 {
		t.Errorf("rejected %v, want pages 5 and 6", got.Rejected)
	}
	if got.Rules["short"] != 2 {
		t.Errorf("rules %v", got.Rules)
	}
	// There is no page map in this corpus, and two of the eight rules are worth
	// nothing without one. The report has to say so rather than count them.
	if !got.NoPageMap {
		t.Error("a volume with no page map did not say so")
	}
	if got.NoManifest {
		t.Error("the manifest this corpus was set up with was not found")
	}
}

// images/ is not in git, so the ordinary state of a clean checkout is no
// manifest at all, and the short rule is relaxed on a page the manifest calls
// sparse. Not finding one is not an error and it does change the acceptance
// figure, so it is recorded.
func TestExtractionOfNotesAMissingRenderManifest(t *testing.T) {
	book := corpus.Book{ID: "alg-x-fr", Lang: "fr", Pages: 222}
	root := setupCorpus(t, book, render.Manifest{Book: book.ID}, nil)
	writePage(t, root, book.ID, 1, "a page of prose long enough that the rules have something to run over\n")
	if err := os.RemoveAll(filepath.Dir(render.ManifestPath(root, book.ID))); err != nil {
		t.Fatal(err)
	}
	got, err := extractionOf(root, book)
	if err != nil {
		t.Fatal(err)
	}
	if !got.NoManifest {
		t.Error("a corpus with no render manifest did not say so")
	}
	if got.Read != 1 {
		t.Errorf("read %d, want 1: a missing manifest is not a missing page", got.Read)
	}
}

// A volume in the manifest with no page files is the row that matters most, and
// the walk has to return it rather than skipping to the volumes that exist.
func TestExtractionOfKeepsAVolumeNobodyHasRead(t *testing.T) {
	book := corpus.Book{ID: "top-i-iv", Lang: "en", Pages: 443}
	root := setupCorpus(t, book, render.Manifest{Book: book.ID}, nil)
	got, err := extractionOf(root, book)
	if err != nil {
		t.Fatal(err)
	}
	if got.Read != 0 || got.Unread() != 443 || got.Coverage() != 0 {
		t.Errorf("%+v", got)
	}
}

// Long enough that the short rule has no complaint, and with its dollars in
// pairs.
const longEnough = `Let $A$ be a ring and $M$ an $A$-module. We say that $M$ is
semisimple if it is the sum of a family of simple submodules. The purpose of
this paragraph is to show that this condition is equivalent to several others,
and to draw the consequences for the structure of the ring itself. Recall that a
module is simple when it is not reduced to zero and has no submodule other than
itself and zero. Every quotient of a semisimple module is semisimple, and so is
every submodule, which is the fact the whole theory rests on.
`
