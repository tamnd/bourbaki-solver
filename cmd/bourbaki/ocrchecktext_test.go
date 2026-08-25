package main

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The running head is off the body of every page file on disk, whichever way
// the page was read. Native extraction parses it out of the text layer, and
// ocr.readHead cuts it out of the model's answer before the file is written.
// So the rules only ever see it if checkText puts it back, and for a long time
// checkText put it back for native pages alone. Rule 4 then asked 4320 OCR
// pages for a head that had been moved and rejected all of them.
func TestCheckTextPutsTheHeadBackWhateverReadThePage(t *testing.T) {
	for _, method := range []corpus.PageMethod{
		corpus.MethodNative, corpus.MethodOCR, corpus.MethodOCRRepair,
	} {
		file := corpus.PageFile{
			Meta: corpus.PageFrontMatter{
				Book:        "alg-iv-vii",
				PDFPage:     40,
				Method:      method,
				PageLabel:   "A IV.7",
				RunningHead: "POLYNOMIALS AND RATIONAL FRACTIONS",
				Locator:     &corpus.PageLocator{Section: 1},
			},
			Body: "the first paragraph of the page\n",
		}
		got := checkText(file)
		first := strings.SplitN(got, "\n", 2)[0]
		if !strings.Contains(first, "A IV.7") {
			t.Errorf("%s: first line %q has no page label", method, first)
		}
		if !strings.HasSuffix(strings.TrimSpace(got), "the first paragraph of the page") {
			t.Errorf("%s: the body did not survive: %q", method, got)
		}
	}
}

// A page whose front matter holds no head is a page readHead could not find one
// on, and that is the case rule 4 exists for. Its body has to be handed over
// bare so the rule reads the real first line and judges it. 583 of the OCR
// pages in the head-label volumes are in this state and they are the ones worth
// looking at.
func TestCheckTextLeavesAHeadlessPageAlone(t *testing.T) {
	file := corpus.PageFile{
		Meta: corpus.PageFrontMatter{Book: "alg-iv-vii", PDFPage: 40, Method: corpus.MethodOCR},
		Body: "the first paragraph of the page\n",
	}
	if got := checkText(file); got != file.Body {
		t.Errorf("checkText rewrote a page with no head in its front matter: %q", got)
	}
}
