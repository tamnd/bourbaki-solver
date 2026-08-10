package toc

import (
	"slices"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

func TestNearWord(t *testing.T) {
	// The body of chapter II prints no. 14 as MULTJMODULES.
	if !nearWord("multjmodules", "multimodules") {
		t.Error("one changed letter was not forgiven")
	}
	if nearWord("multiplication", "multimodules") {
		t.Error("two different words were treated as one")
	}
	// Short words are left alone, or "rings" would match "kings".
	if nearWord("kings", "rings") {
		t.Error("a five letter word was matched with a letter changed")
	}
}

func TestExercisesOn(t *testing.T) {
	// The 2023 volume prints the word at the head of each run.
	perSection := "                    EXERCISES                    A VIII.467\n\n1) Let A be a ring.\n"
	// The 1998 and 2003 volumes gather the runs and cut them with a bare §.
	gathered := "II                LINEAR ALGEBRA\n\n                    §11\n\n1) Let M be a graded module.\n"
	// The 2003 scan reads the § 5 marker of chapter VII as §S.
	damaged := "A.VII.70        MODULES OVER PRINCIPAL IDEAL DOMAINS\n\n              §S\n\n1) Let u be an endomorphism.\n"
	// A page in the middle of a run carries no marker at all.
	middle := "A.IV.98        POLYNOMIALS AND RATIONAL FRACTIONS        §6\n\nb) Show that Ap has as image.\n"

	pages := []string{perSection, gathered, damaged, middle}
	tests := []struct {
		page, section int
		appendix, ok  bool
	}{
		{1, 3, false, true},
		{2, 11, false, true},
		{2, 4, false, false},
		{3, 5, false, true},
		{4, 6, false, false},
		{9, 1, false, false},
	}
	for _, tt := range tests {
		if got := exercisesOn(pages, tt.page, tt.section, tt.appendix); got != tt.ok {
			t.Errorf("exercisesOn(page %d, § %d) = %v, want %v", tt.page, tt.section, got, tt.ok)
		}
	}
}

func TestVerify(t *testing.T) {
	pages := []string{
		"CHAPTER VIII\n\nSEMISIMPLE MODULES AND RINGS\n",
		"§ 1. SIMPLE MODULES\n\n1. Simple Modules\n\nLet A be a ring.\n",
		"2. Simple Modules over a Ring\n\nLet A be a ring.\n",
		"                    EXERCISES\n\n1) Let E be a module.\n",
	}
	b := corpus.BookTOC{ID: "test", Chapters: []corpus.Chapter{{
		Numeral: "VIII", Title: "Semisimple Modules and Rings", PDFPage: 1,
		Sections: []corpus.Section{{
			Number: 1, Title: "Simple Modules", PDFPage: 2,
			Subsections: []corpus.Subsection{
				{Number: 1, Title: "Simple Modules", PDFPage: 2},
				{Number: 2, Title: "Simple Modules over a Ring", PDFPage: 3},
			},
			Exercises: &corpus.Locator{PDFPage: 4},
		}},
	}}}
	r := Verify(pages, b)
	if r.Checked != 5 || r.Matched != 5 {
		t.Fatalf("matched %d of %d, misses %v", r.Matched, r.Checked, r.Misses)
	}

	// Move one no. off its page and the miss has to say where it really is,
	// because that is the difference between a wrong page and a bad scan.
	b.Chapters[0].Sections[0].Subsections[1].PDFPage = 4
	r = Verify(pages, b)
	moved := r.Moved()
	if len(moved) != 1 || !slices.Contains(moved[0].Found, 3) {
		t.Fatalf("moved = %+v, want the heading found on pdf 3", moved)
	}
}
