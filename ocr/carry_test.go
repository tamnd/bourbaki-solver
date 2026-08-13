package ocr

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// A reading out of a picture keeps the two things a picture cannot say.
//
// Both are real. Lie 7 to 9 prints its folio as a bare number, "340 COMPACT
// REAL LIE GROUPS Ch. IX", so no page label can be read off the page at all and
// the two flagged pages of the pilot went out without one. Whether a page
// carries on the paragraph before it is read off the indent of the first line
// against the page before, which is not in the picture either.
func TestAPageReadFromAPictureKeepsTheLabelAndTheIndentTheExtractorFound(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0347.md")
	before := corpus.PageFile{Meta: corpus.PageFrontMatter{
		Book: "lie-vii-ix", PDFPage: 347, PageLabel: "A IX.340",
		RunningHead: "340 COMPACT REAL LIE GROUPS Ch. IX",
		Continues:   true, Method: corpus.MethodNative, Lines: 35,
	}, Body: "the reading out of the text layer"}
	if err := before.Write(path); err != nil {
		t.Fatal(err)
	}

	meta := corpus.PageFrontMatter{
		Book: "lie-vii-ix", PDFPage: 347, Method: corpus.MethodOCR,
		RunningHead: "340 COMPACT REAL LIE GROUPS Ch. IX", Lines: 48,
	}
	carry(&meta, path)
	if meta.PageLabel != "A IX.340" {
		t.Errorf("PageLabel = %q, want %q", meta.PageLabel, "A IX.340")
	}
	if !meta.Continues {
		t.Error("the page stopped saying it carries on the one before it")
	}
}

// What the model read wins where the model read something. A page whose head
// carries its own label keeps that label, since the picture is the more direct
// evidence and a re-read is exactly the place a wrong label gets corrected.
func TestALabelReadOffThePageIsTheOneKept(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0268.md")
	before := corpus.PageFile{Meta: corpus.PageFrontMatter{
		Book: "alg-viii-fr", PDFPage: 268, PageLabel: "A VIII.264",
		Method: corpus.MethodNative, Lines: 33,
	}, Body: "the reading out of the text layer"}
	if err := before.Write(path); err != nil {
		t.Fatal(err)
	}

	meta := corpus.PageFrontMatter{
		Book: "alg-viii-fr", PDFPage: 268, Method: corpus.MethodOCR,
		PageLabel: "A VIII.265", Lines: 33,
	}
	carry(&meta, path)
	if meta.PageLabel != "A VIII.265" {
		t.Errorf("PageLabel = %q, want the one the model read", meta.PageLabel)
	}
}

// A page nothing has read yet is the ordinary case for a scanned volume, and
// there is nothing to carry over. It has to be quiet about that rather than
// refuse to write the page.
func TestAPageWithNothingBeforeItIsWrittenAsItWasRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0012.md")
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Fatalf("the fixture directory is not empty: %v", err)
	}
	meta := corpus.PageFrontMatter{
		Book: "alg-i-iii", PDFPage: 12, Method: corpus.MethodOCR,
		PageLabel: "A I.5", Lines: 30,
	}
	carry(&meta, path)
	if meta.PageLabel != "A I.5" || meta.Continues {
		t.Errorf("meta changed: %+v", meta)
	}
}

// A head the model split across three lines is a head the parser did not find,
// and an empty head takes the locator down with it. Page 111 of Algebra VIII
// prints "§ 5." at the left margin, the chapter title in the middle and "111"
// at the right, and the model wrote the three parts as three lines.
func TestAPageWhoseHeadCameBackEmptyKeepsTheOneTheExtractorRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0119.md")
	before := corpus.PageFile{Meta: corpus.PageFrontMatter{
		Book: "lie-vii-ix", PDFPage: 119, PageLabel: "A VIII.111",
		RunningHead: ". AUTOMORPHISMS OF A SEMI-SIMPLE LIE ALGEBRA 111",
		Locator:     &corpus.PageLocator{Section: 5},
		Method:      corpus.MethodNative, Lines: 34,
	}, Body: "the reading out of the text layer"}
	if err := before.Write(path); err != nil {
		t.Fatal(err)
	}

	meta := corpus.PageFrontMatter{
		Book: "lie-vii-ix", PDFPage: 119, Method: corpus.MethodOCR, Lines: 43,
	}
	carry(&meta, path)
	if meta.RunningHead != before.Meta.RunningHead {
		t.Errorf("RunningHead = %q, want the one the extractor read", meta.RunningHead)
	}
	if meta.Locator == nil || meta.Locator.Section != 5 {
		t.Errorf("Locator = %v, want § 5", meta.Locator)
	}
}

// A head the model did read is the one to keep, locator and all. The picture is
// the more direct evidence, and a re-read is where a head the text layer got
// wrong gets put right.
func TestAHeadReadOffThePageIsTheOneKept(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "0147.md")
	before := corpus.PageFile{Meta: corpus.PageFrontMatter{
		Book: "lie-vii-ix", PDFPage: 147, PageLabel: "A VIII.139",
		RunningHead: "139 SPLITTABLE",
		Locator:     &corpus.PageLocator{Section: 5},
		Method:      corpus.MethodNative, Lines: 30,
	}, Body: "the reading out of the text layer"}
	if err := before.Write(path); err != nil {
		t.Fatal(err)
	}

	meta := corpus.PageFrontMatter{
		Book: "lie-vii-ix", PDFPage: 147, Method: corpus.MethodOCR,
		RunningHead: "§ 5.7 SPLITTABLE SUBALGEBRAS 139",
		Locator:     &corpus.PageLocator{Section: 5, Subsec: 7},
		Lines:       31,
	}
	carry(&meta, path)
	if meta.RunningHead != "§ 5.7 SPLITTABLE SUBALGEBRAS 139" {
		t.Errorf("RunningHead = %q, want the one the model read", meta.RunningHead)
	}
	if meta.Locator == nil || meta.Locator.Subsec != 7 {
		t.Errorf("Locator = %v, want § 5.7", meta.Locator)
	}
}
