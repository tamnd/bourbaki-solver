package main

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// writePage puts one page file on disk the way the OCR run writes them, body
// and all, so the reader under test sees what it sees in the corpus.
func writePage(t *testing.T, root, book string, pdfPage int, body string) {
	t.Helper()
	file := corpus.PageFile{
		Meta: corpus.PageFrontMatter{Book: book, PDFPage: pdfPage},
		Body: body,
	}
	if err := file.Write(corpus.PagePath(root, book, pdfPage)); err != nil {
		t.Fatal(err)
	}
}

// A volume with no text layer has nothing for pdftotext to read, so the running
// heads the page map is built out of come off the pages the model wrote. That
// is the whole reason render and ocr run before pagemap build on those three
// volumes, and it is the one ordering in the pipeline that is the other way
// round from every other.
func TestAVolumeWithNoTextLayerTakesItsHeadsOffThePagesTheModelWrote(t *testing.T) {
	root := t.TempDir()
	book := corpus.Book{ID: "alg-x-fr", Pages: 3, TextLayer: "none"}
	writePage(t, root, book.ID, 1, "A X.5 ALGEBRE HOMOLOGIQUE § 1\n\nfirst\n")
	writePage(t, root, book.ID, 2, "second\n")
	writePage(t, root, book.ID, 3, "A X.7 ALGEBRE HOMOLOGIQUE § 1\n\nthird\n")

	pages, err := readPageFiles(root, book)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 3 {
		t.Fatalf("got %d pages, want 3, one slot per PDF page", len(pages))
	}
	if !strings.Contains(pages[0], "A X.5") || !strings.Contains(pages[2], "A X.7") {
		t.Fatalf("the heads did not come through: %q", pages)
	}
	if !strings.Contains(pages[1], "second") {
		t.Fatalf("page 2 = %q, want the body of the second page", pages[1])
	}
}

// Half a volume is the normal state of one of these while the fleet is still
// reading it, and the offsets of the half that is there are worth having. A gap
// is a slot with nothing in it and not a page shifted up by one, which is the
// mistake that would put every anchor after the gap on the wrong PDF page.
func TestAVolumeHalfReadStillGivesAMapOfTheHalfThatIsThere(t *testing.T) {
	root := t.TempDir()
	book := corpus.Book{ID: "alg-x-fr", Pages: 4, TextLayer: "none"}
	writePage(t, root, book.ID, 1, "A X.5\n")
	writePage(t, root, book.ID, 4, "A X.8\n")

	pages, err := readPageFiles(root, book)
	if err != nil {
		t.Fatal(err)
	}
	if len(pages) != 4 {
		t.Fatalf("got %d pages, want 4 even with two of them unread", len(pages))
	}
	if pages[1] != "" || pages[2] != "" {
		t.Fatalf("pages 2 and 3 are unread and came back as %q and %q, want empty", pages[1], pages[2])
	}
	if !strings.Contains(pages[3], "A X.8") {
		t.Fatalf("page 4 = %q, want it in slot 4 and not moved up into the gap", pages[3])
	}
}

// Nothing read at all is the one case that must not come back as a map. An
// empty map would validate as a volume with no anchors, which reads as a page
// map that failed rather than a volume nobody has rendered yet, and the error
// says which two commands to run.
func TestAVolumeNobodyHasReadSaysWhichCommandsToRun(t *testing.T) {
	root := t.TempDir()
	book := corpus.Book{ID: "top-v-x", Pages: 372, TextLayer: "none"}

	if _, err := readPageFiles(root, book); err == nil {
		t.Fatal("a volume with no pages read gave back a map and no error")
	} else if !strings.Contains(err.Error(), "render") || !strings.Contains(err.Error(), "ocr") {
		t.Fatalf("the error is %q, want it to name bourbaki render and bourbaki ocr", err)
	}
}
