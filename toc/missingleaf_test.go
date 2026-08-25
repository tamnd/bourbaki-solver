package toc

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
)

// A contents entry lands on no pdf page for two quite different reasons. Either
// a digit was misread and the entry names a page the volume never printed, or
// the volume printed it and the scan is short that leaf. The first is the
// contents being wrong and a manifest must not be written over it. The second is
// the contents being right about a file that is incomplete, and refusing to
// write over that holds a correct reading hostage to a page nobody can put back
// by editing it.
//
// The English History is the case. Its scan is missing ten printed pages, 56,
// 68, 84, 116, 124, 144, 203, 230, 268 and 296, and note 19 is listed on 203.
// Everything else about its contents reads clean.
func missingLeafMap(steps []pagemap.Step) *pagemap.Map {
	m := &pagemap.Map{Book: "hist", Pagination: pagemap.Continuous, PDFPages: 12,
		Chapters: []pagemap.Span{{Chapter: "1", FirstPDF: 1, LastPDF: 12,
			FirstPage: 200, LastPage: 210}},
		Steps: steps}
	pdf := 0
	for p := 200; p <= 210; p++ {
		if p == 203 {
			continue
		}
		pdf++
		m.Entries = append(m.Entries, pagemap.Entry{PDFPage: pdf, Chapter: "1",
			Page: p, Confidence: pagemap.FromHead})
	}
	return m
}

func missingLeafResult() *Result {
	return &Result{Book: "hist", Chapters: []corpus.Chapter{{
		Numeral: "1", Title: "ELEMENTS OF THE HISTORY OF MATHEMATICS",
		Page: 200, PDFPage: 1,
		Sections: []corpus.Section{
			{Number: 1, Title: "Foundations", Page: 200, PDFPage: 1},
			{Number: 2, Title: "The Gamma Function", Page: 203, PDFPage: 0},
		},
	}}}
}

// The leaf the map records as missing is reported softly, so the manifest is
// still written and the volume still says out loud what it is short.
func TestAPageTheScanIsShortIsSoft(t *testing.T) {
	pm := missingLeafMap([]pagemap.Step{{AtPDFPage: 4, Chapter: "1",
		FromOffset: -199, ToOffset: -198, MissingPages: []int{203}}})
	probs := missingLeafResult().validate(pm, Options{})
	if len(probs) != 1 {
		t.Fatalf("problems = %v, want the one missing leaf", probs)
	}
	if !probs[0].Soft {
		t.Errorf("the missing leaf was reported hard: %q", probs[0].Detail)
	}
	if !strings.Contains(probs[0].Detail, "not in the scan") {
		t.Errorf("the problem does not say what it is: %q", probs[0].Detail)
	}
	if len(Hard(probs)) != 0 {
		t.Errorf("Hard kept %v, so the manifest would not be written", Hard(probs))
	}
}

// The same entry against a map that records no such leaf is a misread digit and
// stays hard, because nothing says the volume ever printed the page.
func TestAPageTheVolumeNeverPrintedStaysHard(t *testing.T) {
	probs := missingLeafResult().validate(missingLeafMap(nil), Options{})
	if len(probs) != 1 {
		t.Fatalf("problems = %v, want the one page", probs)
	}
	if probs[0].Soft {
		t.Errorf("a page nothing accounts for was excused: %q", probs[0].Detail)
	}
	if !strings.Contains(probs[0].Detail, "on no pdf page") {
		t.Errorf("the problem does not read as it used to: %q", probs[0].Detail)
	}
	if len(Hard(probs)) != 1 {
		t.Errorf("Hard dropped it, so the manifest would be written anyway")
	}
}

// A step belongs to the chapter it was read in, and printed pages repeat across
// chapters in a volume that numbers per chapter. A leaf missing from chapter I
// is no excuse for the same printed number under chapter II.
func TestAMissingLeafDoesNotExcuseAnotherChapter(t *testing.T) {
	pm := missingLeafMap([]pagemap.Step{{AtPDFPage: 4, Chapter: "II",
		FromOffset: -199, ToOffset: -198, MissingPages: []int{203}}})
	probs := missingLeafResult().validate(pm, Options{})
	if len(probs) != 1 || probs[0].Soft {
		t.Errorf("problems = %v, want the one page reported hard", probs)
	}
}
