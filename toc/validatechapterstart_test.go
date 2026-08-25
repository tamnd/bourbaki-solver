package toc

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
)

func startProblems(probs []Problem) []Problem {
	var out []Problem
	for _, p := range probs {
		if strings.Contains(p.Detail, "the contents starts it at printed page") {
			out = append(out, p)
		}
	}
	return out
}

// Nothing marks the end of the front matter, so the fit hands every leaf before
// chapter II to chapter I and the span starts on the first leaf of the file.
// That is a default, not a reading, and it must not be used to call the
// contents wrong. ac-i-iv-fr is the case: the map runs chapter I from pdf 1 and
// calls it printed page 3, the contents says printed 13 at pdf 11, and the two
// agree on every other chapter and on the offset itself.
func TestAChapterSpanningFromTheFirstLeafDoesNotContradictTheContents(t *testing.T) {
	r := &Result{Book: "ac-i-iv-fr", Chapters: []corpus.Chapter{
		{Numeral: "I", Title: "Modules plats", Page: 13, PDFPage: 11},
		{Numeral: "II", Title: "Localisation", Page: 69, PDFPage: 67},
	}}
	pm := &pagemap.Map{Chapters: []pagemap.Span{
		{Chapter: "I", FirstPDF: 1, LastPDF: 66, FirstPage: 3, LastPage: 68},
		{Chapter: "II", FirstPDF: 67, LastPDF: 180, FirstPage: 69, LastPage: 182},
	}}
	if got := startProblems(r.validate(pm, Options{})); len(got) > 0 {
		t.Errorf("a span starting on pdf page 1 was treated as evidence: %v", got)
	}
}

// The rule still has to fire everywhere it is worth firing. A chapter that
// starts past the front matter has a span that was read rather than filled in,
// so a disagreement there is a real one and has to be reported.
func TestAChapterPastTheFrontMatterStillHasToAgree(t *testing.T) {
	r := &Result{Book: "alg-i-iii-fr", Chapters: []corpus.Chapter{
		{Numeral: "I", Title: "Structures algebriques", Page: 40, PDFPage: 30},
	}}
	pm := &pagemap.Map{Chapters: []pagemap.Span{
		{Chapter: "I", FirstPDF: 30, LastPDF: 200, FirstPage: 11, LastPage: 181},
	}}
	got := startProblems(r.validate(pm, Options{}))
	if len(got) != 1 {
		t.Fatalf("a real disagreement past the front matter was not reported: %v", got)
	}
	if !strings.Contains(got[0].Detail, "printed page 40") {
		t.Errorf("the problem does not say what the contents said: %q", got[0].Detail)
	}
}
