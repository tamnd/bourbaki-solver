package toc

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The titles here are the ones the Theory of Sets came out with and the ones
// they were corrected to, which is the case the whole file exists for.

func TestACorrectedTitleIsKeptAndReported(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "I", Title: "Description of formal mathematics",
		Sections: []corpus.Section{{Number: 1, Title: "Terms and relations",
			Subsections: []corpus.Subsection{{Number: 3, Title: "Methods of proof"}}}}}}
	fresh := []corpus.Chapter{{Numeral: "I", Title: "Description of formal mathematics",
		Sections: []corpus.Section{{Number: 1, Title: "Terms and relations",
			Subsections: []corpus.Subsection{{Number: 3, Title: "Met?ods .of proof"}}}}}}

	out, kept := KeepTitles(old, fresh)
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

	out, kept := KeepTitles(chapters, chapters)
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

	out, kept := KeepTitles(old, fresh)
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

	out, kept := KeepTitles(old, fresh)
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

	out, kept := KeepTitles(old, fresh)
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

	out, kept := KeepTitles(old, fresh)
	if len(kept) != 0 {
		t.Errorf("kept %v, want nothing", kept)
	}
	if out[0].Sections[0].Title != "Terms and relations" {
		t.Errorf("title %q, want the one the volume reads", out[0].Sections[0].Title)
	}
}

// Everything but the title comes from the rebuild. A page that moved is a page
// map or an erratum doing its job, and toc verify holds pages against the body
// of the book, which is a check nobody does by hand.
func TestOnlyTheTitleIsKept(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "I", Title: "Description of formal mathematics",
		Page: 15, PDFPage: 20, Sections: []corpus.Section{
			{Number: 1, Title: "Terms and relations", Page: 15, PDFPage: 20}}}}
	fresh := []corpus.Chapter{{Numeral: "I", Title: "Description of formaI mathematics",
		Page: 16, PDFPage: 21, Sections: []corpus.Section{
			{Number: 1, Title: "Terms and relations", Page: 16, PDFPage: 21}}}}

	out, _ := KeepTitles(old, fresh)
	if out[0].Page != 16 || out[0].PDFPage != 21 {
		t.Errorf("chapter at printed %d pdf %d, want the rebuilt pages", out[0].Page, out[0].PDFPage)
	}
	if out[0].Sections[0].Page != 16 {
		t.Errorf("§ at printed %d, want the rebuilt page", out[0].Sections[0].Page)
	}
	if out[0].Title != "Description of formal mathematics" {
		t.Errorf("title %q, want the corrected one", out[0].Title)
	}
}

// The rebuilt chapters are not written over, so a caller that wants the new
// readings still has them.
func TestTheRebuiltChaptersAreNotWrittenOver(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "I", Sections: []corpus.Section{
		{Number: 1, Title: "Terms and relations"}}}}
	fresh := []corpus.Chapter{{Numeral: "I", Sections: []corpus.Section{
		{Number: 1, Title: "Terms and reIations"}}}}

	KeepTitles(old, fresh)
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

	out, kept := KeepTitles(old, fresh)
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

	out, _ := KeepTitles(old, fresh)
	if out[0].Nominal {
		t.Error("chapter I of Algebre came back nominal")
	}
}

// A page the rebuild could not resolve. hist § 19 is printed on 203, the page
// map steps from pdf 204 (printed 202) to pdf 205 (printed 204), and 203 is in
// the gap, so the rebuild comes back with nothing where the manifest had 205.
func TestAPageTheRebuildCouldNotResolveKeepsTheOneTheManifestHad(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "1", Page: 1, PDFPage: 9,
		Sections: []corpus.Section{{Number: 19, Page: 203, PDFPage: 205,
			Subsections: []corpus.Subsection{{Number: 1, Page: 203, PDFPage: 205}}}}}}
	fresh := []corpus.Chapter{{Numeral: "1", Page: 1, PDFPage: 9,
		Sections: []corpus.Section{{Number: 19, Page: 203, PDFPage: 0,
			Subsections: []corpus.Subsection{{Number: 1, Page: 203, PDFPage: 0}}}}}}

	out, kept := KeepTitles(old, fresh)
	if got := out[0].Sections[0].PDFPage; got != 205 {
		t.Errorf("§ 19 is on pdf page %d, want the 205 the manifest had", got)
	}
	if got := out[0].Sections[0].Subsections[0].PDFPage; got != 205 {
		t.Errorf("§ 19 no. 1 is on pdf page %d, want the 205 the manifest had", got)
	}
	// Only titles are reported. A page that was held on to is not a reading
	// somebody made and then had undone, it is a reading nobody got.
	if len(kept) != 0 {
		t.Errorf("reported %v, want nothing", kept)
	}
}

// The other half of the rule, and the half the file was written for: a page the
// rebuild did resolve is taken however far it is from the one before it, since
// that is the page map or an erratum doing its job.
func TestAPageTheRebuildDidResolveReplacesTheOneTheManifestHad(t *testing.T) {
	old := []corpus.Chapter{{Numeral: "I", Page: 1, PDFPage: 9,
		Sections: []corpus.Section{{Number: 1, Page: 3, PDFPage: 11}}}}
	fresh := []corpus.Chapter{{Numeral: "I", Page: 1, PDFPage: 13,
		Sections: []corpus.Section{{Number: 1, Page: 5, PDFPage: 17}}}}

	out, _ := KeepTitles(old, fresh)
	if out[0].PDFPage != 13 || out[0].Page != 1 {
		t.Errorf("chapter I is on page %d, pdf %d, want 1 and the new 13",
			out[0].Page, out[0].PDFPage)
	}
	if s := out[0].Sections[0]; s.Page != 5 || s.PDFPage != 17 {
		t.Errorf("§ 1 is on page %d, pdf %d, want the new 5 and 17", s.Page, s.PDFPage)
	}
}
