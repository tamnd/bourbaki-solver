package quality

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/tags"
)

// P04 quotes the formula it could not set, and the line gathering of the
// extractor sometimes runs a display line together with the one under it, so
// the quoted formula spans a line break. Written out as it stands that breaks
// the list item of the report in two and leaves the first half ending in the
// space in front of the break, and the hygiene rules of the corpus refuse a
// line that ends in white space. This is the message of
// content/en/lie/VIII/13_s13_classical_splittable_simple_lie_algebras.md:499.
func TestAQuotedFormulaIsReadOnOneLine(t *testing.T) {
	msg := "KaTeX will not set it: Double superscript at position 9: $\n1\\sum^r''$"
	want := "KaTeX will not set it: Double superscript at position 9: $ 1\\sum^r''$"
	if got := oneline(msg); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// The introduction to Theory of Sets is in no chapter and its front matter says
// so with an empty chapter. Counted as it is written, the empty name is a name
// and the line says the corpus holds one more chapter than any book has.
func TestTheScopeLineCountsNoChapterForAFileInNone(t *testing.T) {
	docs := []Doc{
		{Path: "content/en/ens/I/01_s1.md", Lang: "en", Kind: KindSection,
			Section: &corpus.SectionFrontMatter{Book: "ens", Chapter: "I"}},
		{Path: "content/en/ens/II/01_s1.md", Lang: "en", Kind: KindSection,
			Section: &corpus.SectionFrontMatter{Book: "ens", Chapter: "II"}},
		{Path: "content/en/ens/00_introduction.md", Lang: "en", Kind: KindSection,
			Section: &corpus.SectionFrontMatter{Book: "ens", Kind: corpus.KindIntroduction}},
	}
	got := scope(&Corpus{Docs: docs, Tags: &tags.Set{}})
	if !strings.Contains(got, "2 chapters") {
		t.Errorf("the scope line reads %q", got)
	}
	if !strings.Contains(got, "3 sections") {
		t.Errorf("the introduction is a file of the corpus and is not counted: %q", got)
	}
}
