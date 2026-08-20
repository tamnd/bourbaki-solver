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

// The pages either side of a miss, which is a harder question than the page
// the contents named. Every line here is what the scan of the 1998 Lie volume
// and the 2003 Topology volume actually hand back.
func TestExercisesStart(t *testing.T) {
	pages := []string{
		// 1: the head names § 1, which is the only shape that says anything.
		"~ 1.                                  EXERCISES\n\n10) Let o h<' th<' highest root\n",
		// 2: the head lost its § to the scan, and a footnote opens with a 2.
		"                    EXERCISES                    23!)\n\n<') Let p be a pri111P\n\n" +
			"2   This exercise, hitherto unpuhlislwcl. was com1111111icated to us\n",
		// 3: a verso in the middle of a run heads the chapter, not the run.
		"240                               HOOT SYSTEi\\IS                           Ch. VI\n\n(iii) (p, -y*) = 1.\n",
		// 4: the head names § 3.
		"s3.                                      EXERCISES                                      241\n\np - n E X.\n",
		// 5: the 2003 Topology volume heads the word alone, naming no §.
		"                                   EXER.CISES\n\n5) Show that the set is closed.\n",
	}
	tests := []struct {
		page, section int
		head, ok      bool
	}{
		// The head of page 1 names § 1 and page 1 comes before the miss.
		{1, 1, true, true},
		// The same head, read from a page after the miss, says nothing: that is
		// what the second page of a correctly placed run looks like.
		{1, 1, false, false},
		// A footnote opening with a 2 is not a head naming § 2.
		{2, 2, true, false},
		// A verso heads the chapter, and a head naming no § is no evidence.
		{3, 2, true, false},
		{5, 4, true, false},
		// A head that names a different § is not this run.
		{4, 2, true, false},
		{4, 3, true, true},
		{9, 1, true, false},
	}
	for _, tt := range tests {
		got := exercisesStart(pages, tt.page, tt.section, false, tt.head)
		if got != tt.ok {
			t.Errorf("exercisesStart(page %d, § %d, head %v) = %v, want %v",
				tt.page, tt.section, tt.head, got, tt.ok)
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

func TestAHeadingWhoseTitleIsMostlyMathematicsIsCheckedOnItsWords(t *testing.T) {
	// The page is a scan of type and carries the words of the title and the
	// formula as it was set. It does not carry the names of the macros the
	// corpus writes that formula with, and asking it for them puts a heading
	// that is printed exactly where the contents says it is into the misses.
	pages := []string{
		"CHAPTER III\n\nSPACES OF CONTINUOUS LINEAR MAPPINGS\n",
		"§ 3. SPACES OF CONTINUOUS LINEAR MAPPINGS\n\n3. The spaces L_S(E; F)\n\nLet E and F be locally convex spaces.\n",
	}
	b := corpus.BookTOC{ID: "test", Chapters: []corpus.Chapter{{
		Numeral: "III", Title: "Spaces of Continuous Linear Mappings", PDFPage: 1,
		Sections: []corpus.Section{{
			Number: 3, Title: "Spaces of Continuous Linear Mappings", PDFPage: 2,
			Subsections: []corpus.Subsection{
				{Number: 3, Title: `The spaces $\mathscr{L}_{\mathfrak{S}}(E; F)$`, PDFPage: 2},
			},
		}},
	}}}
	r := Verify(pages, b)
	if len(r.Misses) != 0 {
		t.Fatalf("misses = %+v, want the heading found on the page it is printed on", r.Misses)
	}
}

func TestATitleThatIsNothingButMathematicsIsPassedOver(t *testing.T) {
	// There is nothing to look for and nothing honest to say about it, so it
	// is left out of the count rather than counted as a heading that is there
	// or a heading that is not.
	pages := []string{"CHAPTER I\n\nGROUPS\n", "§ 1. GROUPS\n\n2. G_a\n\nLet G be a group.\n"}
	b := corpus.BookTOC{ID: "test", Chapters: []corpus.Chapter{{
		Numeral: "I", Title: "Groups", PDFPage: 1,
		Sections: []corpus.Section{{
			Number: 1, Title: "Groups", PDFPage: 2,
			Subsections: []corpus.Subsection{{Number: 2, Title: `$G_{a}$`, PDFPage: 2}},
		}},
	}}}
	r := Verify(pages, b)
	// The chapter and the § carry words, the no. does not.
	if r.Checked != 2 {
		t.Errorf("checked %d headings, want the two that carry words", r.Checked)
	}
	if len(r.Misses) != 0 {
		t.Errorf("misses = %+v", r.Misses)
	}
}
