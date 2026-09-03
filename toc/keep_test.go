package toc

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
)

// plainMap is a volume whose printed page is its pdf page plus a fixed offset
// the whole way down, which is the shape hist-fr actually has: 117 of its pages
// had their folio read and every one of them agrees on printed = pdf + 2.
func plainMap(off, pages int) *pagemap.Map {
	m := &pagemap.Map{}
	for pdf := 1; pdf <= pages; pdf++ {
		m.Entries = append(m.Entries, pagemap.Entry{
			PDFPage: pdf, Page: pdf + off, Confidence: pagemap.FromHead})
	}
	return m
}

// The titles here are the ones the Theory of Sets came out with and the ones
// they were corrected to, which is the case the whole file exists for.

func TestACorrectedTitleIsKeptAndReported(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "I", Title: "Description of formal mathematics",
		Sections: []corpus.Section{{Number: 1, Title: "Terms and relations",
			Subsections: []corpus.Subsection{{Number: 3, Title: "Methods of proof"}}}}}}
	fresh := []corpus.Chapter{{Numeral: "I", Title: "Description of formal mathematics",
		Sections: []corpus.Section{{Number: 1, Title: "Terms and relations",
			Subsections: []corpus.Subsection{{Number: 3, Title: "Met?ods .of proof"}}}}}}

	out, kept := KeepTitles(old, fresh, plainMap(2, 400))
	if got := out[0].Sections[0].Subsections[0].Title; got != "Methods of proof" {
		t.Errorf("title %q, want the corrected one", got)
	}
	if len(kept) != 1 {
		t.Fatalf("kept %v, want one", kept)
	}
	if kept[0].Where != "chapter I § 1 no. 3" {
		t.Errorf("named %q, want chapter I § 1 no. 3", kept[0].Where)
	}
	if kept[0].Was != "Methods of proof" || kept[0].Now != "Met?ods .of proof" {
		t.Errorf("reported %q against %q", kept[0].Was, kept[0].Now)
	}
}

// A rebuild that reads the volume the same way it read it last time has nothing
// to keep and nothing to say.
func TestARebuildThatAgreesReportsNothing(t *testing.T) {
	chapters := []corpus.Chapter{{Numeral: "I", Title: "Description of formal mathematics",
		Sections: []corpus.Section{{Number: 1, Title: "Terms and relations"}}}}

	out, kept := KeepTitles(chapters, chapters, plainMap(2, 400))
	if len(kept) != 0 {
		t.Errorf("kept %v, want nothing", kept)
	}
	if out[0].Sections[0].Title != "Terms and relations" {
		t.Errorf("title %q came back changed", out[0].Sections[0].Title)
	}
}

// A chapter, a § or a no. the manifest has never carried is new, and what the
// volume reads is all there is. Nothing is reported, because there is no
// correction there to have undone.
func TestAnEntryTheManifestDoesNotHaveIsNew(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "I", Title: "Description of formal mathematics"}}
	fresh := []corpus.Chapter{
		{Numeral: "I", Title: "Description of formal mathematics"},
		{Numeral: "II", Title: "Theory of sets",
			Sections: []corpus.Section{{Number: 1, Title: "Collectivizing relations"}}},
	}

	out, kept := KeepTitles(old, fresh, plainMap(2, 400))
	if len(kept) != 0 {
		t.Errorf("kept %v, want nothing", kept)
	}
	if out[1].Sections[0].Title != "Collectivizing relations" {
		t.Errorf("the new § lost its title: %q", out[1].Sections[0].Title)
	}
}

// An appendix and a § can carry the same number in the same chapter, so the
// two are told apart before a title is kept. Chapter VIII of Algebra has four
// numbered appendices.
func TestAnAppendixIsNotTheSectionOfTheSameNumber(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "VIII", Sections: []corpus.Section{
		{Number: 1, Title: "Simple modules"},
		{Number: 1, Appendix: true, Title: "The Jacobson radical"},
	}}}
	fresh := []corpus.Chapter{{Numeral: "VIII", Sections: []corpus.Section{
		{Number: 1, Title: "SimpIe modules"},
		{Number: 1, Appendix: true, Title: "The Jacobson radical"},
	}}}

	out, kept := KeepTitles(old, fresh, plainMap(2, 400))
	if len(kept) != 1 || kept[0].Where != "chapter VIII § 1" {
		t.Fatalf("kept %v, want the § alone", kept)
	}
	if out[0].Sections[1].Title != "The Jacobson radical" {
		t.Errorf("the appendix title moved: %q", out[0].Sections[1].Title)
	}
}

// A chapter that prints its nos with no § over it keeps them the same way.
// Chapter I of the English Integration is the one chapter of the library like
// that.
func TestTheNosOfAChapterWithNoSection(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "I", Subsections: []corpus.Subsection{
		{Number: 1, Title: "Convexity inequalities"}}}}
	fresh := []corpus.Chapter{{Numeral: "I", Subsections: []corpus.Subsection{
		{Number: 1, Title: "Convexity inequaIities"}}}}

	out, kept := KeepTitles(old, fresh, plainMap(2, 400))
	if len(kept) != 1 || kept[0].Where != "chapter I no. 1" {
		t.Fatalf("kept %v, want the no. of the chapter", kept)
	}
	if out[0].Subsections[0].Title != "Convexity inequalities" {
		t.Errorf("title %q, want the corrected one", out[0].Subsections[0].Title)
	}
}

// An entry the manifest carries with no title at all is one nobody has
// corrected, so the volume's reading is taken rather than an empty string being
// treated as a correction to keep.
func TestAnEmptyTitleIsNotACorrection(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "I", Sections: []corpus.Section{{Number: 1}}}}
	fresh := []corpus.Chapter{{Numeral: "I", Sections: []corpus.Section{
		{Number: 1, Title: "Terms and relations"}}}}

	out, kept := KeepTitles(old, fresh, plainMap(2, 400))
	if len(kept) != 0 {
		t.Errorf("kept %v, want nothing", kept)
	}
	if out[0].Sections[0].Title != "Terms and relations" {
		t.Errorf("title %q, want the one the volume reads", out[0].Sections[0].Title)
	}
}

// The title and the printed page on one line of a contents page are one
// reading, and a scan that got the title wrong is a scan. This used to assert
// that only the title was kept; hist-fr is the volume that showed the page on
// the same line needs keeping for the same reason, and both are checked here
// now. The pdf page still comes from the map, which is the part that never
// changed.
func TestTheTitleAndThePrintedPageOnALineAreKeptTogether(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "I", Title: "Description of formal mathematics",
		Page: 15, PDFPage: 13, Sections: []corpus.Section{
			{Number: 1, Title: "Terms and relations", Page: 15, PDFPage: 13}}}}
	fresh := []corpus.Chapter{{Numeral: "I", Title: "Description of formaI mathematics",
		Page: 16, PDFPage: 14, Sections: []corpus.Section{
			{Number: 1, Title: "Terms and relations", Page: 16, PDFPage: 14}}}}

	out, kept := KeepTitles(old, fresh, plainMap(2, 400))
	if out[0].Title != "Description of formal mathematics" {
		t.Errorf("title %q, want the corrected one", out[0].Title)
	}
	if out[0].Page != 15 || out[0].PDFPage != 13 {
		t.Errorf("chapter at printed %d pdf %d, want the corrected 15 and the map's 13",
			out[0].Page, out[0].PDFPage)
	}
	if s := out[0].Sections[0]; s.Page != 15 || s.PDFPage != 13 {
		t.Errorf("§ at printed %d pdf %d, want 15 and 13", s.Page, s.PDFPage)
	}
	// One line, two readings, and each is reported on its own: the title is a
	// correction somebody made and so is the page, and a summary that counted
	// them as one would understate what a -retitle would throw away.
	if len(kept) != 3 {
		t.Fatalf("reported %v, want the chapter title, its page and the § page", kept)
	}
}

// The rebuilt chapters are not written over, so a caller that wants the new
// readings still has them.
func TestTheRebuiltChaptersAreNotWrittenOver(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "I", Sections: []corpus.Section{
		{Number: 1, Title: "Terms and relations"}}}}
	fresh := []corpus.Chapter{{Numeral: "I", Sections: []corpus.Section{
		{Number: 1, Title: "Terms and reIations"}}}}

	KeepTitles(old, fresh, plainMap(2, 400))
	if got := fresh[0].Sections[0].Title; got != "Terms and reIations" {
		t.Errorf("the rebuild now reads %q, want what the volume read", got)
	}
}

// A volume that is not divided into chapters carries the flag that says so in
// its manifest and nowhere else, since its contents page has no way to say it.
// Elements d'histoire des mathematiques and Varietes differentielles et
// analytiques are both like this, and a rebuild that dropped the flag would
// send the assembler looking for a chapter heading neither printing sets.
func TestTheNominalFlagSurvivesARebuild(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "1", Nominal: true,
		Title: "VARIÉTÉS DIFFÉRENTIELLES ET ANALYTIQUES, FASCICULE DE RÉSULTATS"}}
	fresh := []corpus.Chapter{{Numeral: "1", Page: 11, PDFPage: 9,
		Title: "VARIÉTÉS DIFFÉRENTIELLES ET ANALYTIQUES, FASCICULE DE RÉSULTATS"}}

	out, kept := KeepTitles(old, fresh, plainMap(2, 400))
	if !out[0].Nominal {
		t.Error("the rebuild dropped the nominal flag")
	}
	if len(kept) != 0 {
		t.Errorf("kept %v, want nothing: the flag is not a reading", kept)
	}
}

// A chapter the manifest does not call nominal stays that way. The flag is one
// the corpus adds and never one it takes away, but nothing here invents it.
func TestAChapterThePrintingHasIsNotMadeNominal(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "I", Title: "Structures algébriques"}}
	fresh := []corpus.Chapter{{Numeral: "I", Title: "Structures algébriques"}}

	out, _ := KeepTitles(old, fresh, plainMap(2, 400))
	if out[0].Nominal {
		t.Error("chapter I of Algebre came back nominal")
	}
}

// A page the rebuild could not resolve. hist § 19 is printed on 203, the page
// map steps from pdf 204 (printed 202) to pdf 205 (printed 204), and 203 is in
// the gap, so the rebuild comes back with nothing where the manifest had 205.
func TestAPageTheRebuildCouldNotResolveKeepsTheOneTheManifestHad(t *testing.T) {
	// The gap itself: printed 203 is on no pdf page of this map.
	pm := &pagemap.Map{Entries: []pagemap.Entry{
		{PDFPage: 204, Page: 202, Confidence: pagemap.FromHead},
		{PDFPage: 205, Page: 204, Confidence: pagemap.FromHead},
	}}
	old := []corpus.Chapter{{Numeral: "1", Page: 1, PDFPage: 9,
		Sections: []corpus.Section{{Number: 19, Page: 203, PDFPage: 205}}}}
	fresh := []corpus.Chapter{{Numeral: "1", Page: 1, PDFPage: 9,
		Sections: []corpus.Section{{Number: 19, Page: 203, PDFPage: 0}}}}

	out, kept := KeepTitles(old, fresh, pm)
	if got := out[0].Sections[0].PDFPage; got != 205 {
		t.Errorf("§ 19 is on pdf page %d, want the 205 the manifest had", got)
	}
	// The two readings agree, so there is no correction here to report. What
	// the map could not do is not a thing the contents page got wrong.
	if len(kept) != 0 {
		t.Errorf("reported %v, want nothing", kept)
	}
}

// The five entries of hist-fr. Its contents page prints "La fonction gamma
// ... 253" and the heading is on pdf 253, which the map puts at printed 255.
// Taking the contents at its word cost the volume two headings at toc verify.
func TestACorrectedPrintedPageIsKeptAndReported(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "1", Page: 3, PDFPage: 1,
		Sections: []corpus.Section{{Number: 19, Title: "La fonction gamma",
			Page: 255, PDFPage: 253}}}}
	fresh := []corpus.Chapter{{Numeral: "1", Page: 3, PDFPage: 1,
		Sections: []corpus.Section{{Number: 19, Title: "La fonction gamma",
			Page: 253, PDFPage: 251}}}}

	out, kept := KeepTitles(old, fresh, plainMap(2, 400))
	s := out[0].Sections[0]
	if s.Page != 255 || s.PDFPage != 253 {
		t.Errorf("§ 19 is on printed %d, pdf %d, want the corrected 255 and 253",
			s.Page, s.PDFPage)
	}
	if len(kept) != 1 {
		t.Fatalf("reported %v, want the one page it kept", kept)
	}
	if kept[0].What != "printed page" || kept[0].Was != "255" || kept[0].Now != "253" {
		t.Errorf("reported %+v, want the printed page 255 against 253", kept[0])
	}
}

// The reason pages were not kept at all, and the half that has to survive:
// a page map corrected tomorrow still moves every pdf page it should. Here the
// contents reading has not changed and the map has, and the new pdf page wins.
func TestAPageMapThatMovedStillMovesThePDFPage(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "I", Page: 3, PDFPage: 1,
		Sections: []corpus.Section{{Number: 1, Page: 13, PDFPage: 11}}}}
	fresh := []corpus.Chapter{{Numeral: "I", Page: 3, PDFPage: 5,
		Sections: []corpus.Section{{Number: 1, Page: 13, PDFPage: 15}}}}

	out, kept := KeepTitles(old, fresh, plainMap(-2, 400))
	if out[0].PDFPage != 5 {
		t.Errorf("chapter I is on pdf %d, want the map's new 5", out[0].PDFPage)
	}
	if s := out[0].Sections[0]; s.Page != 13 || s.PDFPage != 15 {
		t.Errorf("§ 1 is on printed %d, pdf %d, want 13 and the map's new 15",
			s.Page, s.PDFPage)
	}
	if len(kept) != 0 {
		t.Errorf("reported %v, want nothing: no reading changed", kept)
	}
}

// A printed page kept when the map cannot place it either. The manifest's pdf
// page is all there is, so it stands rather than becoming zero.
func TestAKeptPageThatTheMapCannotPlaceHoldsItsPDFPage(t *testing.T) {
	pm := &pagemap.Map{Entries: []pagemap.Entry{
		{PDFPage: 204, Page: 202, Confidence: pagemap.FromHead},
		{PDFPage: 205, Page: 204, Confidence: pagemap.FromHead},
	}}
	old := []corpus.Chapter{{Numeral: "1", Page: 202, PDFPage: 204,
		Sections: []corpus.Section{{Number: 19, Page: 203, PDFPage: 205}}}}
	fresh := []corpus.Chapter{{Numeral: "1", Page: 202, PDFPage: 204,
		Sections: []corpus.Section{{Number: 19, Page: 201, PDFPage: 203}}}}

	out, kept := KeepTitles(old, fresh, pm)
	if s := out[0].Sections[0]; s.Page != 203 || s.PDFPage != 205 {
		t.Errorf("§ 19 is on printed %d, pdf %d, want the manifest's 203 and 205",
			s.Page, s.PDFPage)
	}
	if len(kept) != 1 {
		t.Errorf("reported %v, want the one page it kept", kept)
	}
}
