package ocr

import (
	"fmt"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/render"
)

// This is how a page that is already committed is put back into the shape the
// rules judge, so that the check a reading passed at read time can be run again
// over the corpus as it stands.
//
// It matters that there is one copy of it. The rules run when a page comes back
// from a model and the page is kept or asked again on what they say; nothing ran
// them afterwards until bourbaki audit did, and a second reconstruction written
// beside the first would drift from it and report pages as rejected that no run
// would have rejected. tamnd/bourbaki#383 is the count these produce.

// CheckText reconstructs what a model would have returned for a page.
//
// Both extraction paths file the running head in the front matter, and neither
// body starts with it. Native extraction parses it out of the text layer.
// Vision OCR is asked for it on the first line of the answer, and readHead takes
// that line, splits it into the label, the title and the locator, and cuts it
// out of the body before the page file is written. So the head is off the body
// by the time a page file exists, whichever way the page was read, and putting
// it back is what lets rule 4 judge the line it is about.
func CheckText(file corpus.PageFile) string {
	head := HeadOf(file.Meta)
	if head == "" {
		return file.Body
	}
	return head + "\n\n" + file.Body
}

// HeadOf is the running head of a page put back into the one line it was
// printed on, out of the parts the front matter holds it in.
func HeadOf(meta corpus.PageFrontMatter) string {
	return strings.TrimSpace(strings.Join([]string{
		meta.PageLabel, meta.RunningHead, locatorOf(meta),
	}, "  "))
}

func locatorOf(meta corpus.PageFrontMatter) string {
	if meta.Locator == nil || meta.Locator.Section == 0 {
		return ""
	}
	if meta.Locator.Subsec > 0 {
		return fmt.Sprintf("§ %d.%d", meta.Locator.Section, meta.Locator.Subsec)
	}
	return fmt.Sprintf("§ %d", meta.Locator.Section)
}

// ExpectFor is what the rules are told to expect of one page of one volume.
//
// The render manifest says whether the page is blank or too thin to have a
// length expected of it, and the page map says which chapter and printed page
// it is. Without a page map the running head and page label rules have nothing
// to compare against, and they are skipped rather than guessed at.
func ExpectFor(entry *corpus.Book, pmap *pagemap.Map, manifest render.Manifest, page int) Expect {
	value := Expect{Book: entry.ID, PDFPage: page, Grammar: pagemap.Grammar(entry.Grammar)}
	if found, ok := manifest.Find(page); ok {
		value.Blank = found.Blank
		value.Sparse = found.Ink < SparseInk
	}
	if pmap != nil {
		if found, ok := pmap.Lookup(page); ok {
			value.Chapter, value.Page = found.Chapter, found.Page
			value.Confidence = found.Confidence
			// A page whose number was read off its own running head has one by
			// definition. Anywhere else the page map cannot say, and asking for
			// a head that a chapter opener does not print would fail one page
			// per chapter.
			//
			// Except that head does not always mean read off this page. An
			// erratum supplies the head an opener never printed, so that the
			// fit has an anchor where the printing gives it none, and the map
			// records that at confidence head like any other. The opener is
			// then asked for the very line the erratum exists because it is
			// missing. Every house suppresses the head on an opener, so the map
			// settles it and the confidence does not.
			value.HasHead = found.Confidence == pagemap.FromHead && !pmap.OpensChapter(page)
		}
	}
	return value
}
