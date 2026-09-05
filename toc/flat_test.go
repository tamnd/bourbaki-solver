package toc

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/pagemap"
)

// A volume that prints no chapters has nothing to number its entries by, so it
// numbers none of them. The two printings of the Elements of the History of
// Mathematics are the case, and the French one is the one that had to be read
// this way: it lists twenty six notes with a title, a run of leader dots and a
// page, and nothing whatever in front of the title. Every line of it was turned
// away by classify, the page counted zero contents lines, and the volume came
// out of contentsRun with nothing, so toc build refused it with "hist-fr has no
// page that looks like a table of contents" before a line had been looked at.
//
// The fixture is the page as the model read it, pages/hist-fr/0006.md, with the
// leader runs cut short so the lines fit. Nothing else is changed, and the last
// two lines are kept because they are the whole reason the reading needs a rule
// about what it will not take.
const flatContents = `TABLE DES MATIÈRES

Fondements des mathématiques; logique; théorie des ensembles ...... 9
Numération; analyse combinatoire ...... 65
L'évolution de l'algèbre ...... 68
Algèbre linéaire et algèbre multilinéaire ...... 78
Polynômes et corps commutatifs ...... 92
Divisibilité; corps ordonnés ...... 110
Bibliographie ...... 342
Index des noms cités ...... 366
`

// flatMap is a page map for a volume that prints no chapters, which is the one
// span pagemap gives such a volume and the state the reading is gated on.
func flatMap() *pagemap.Map {
	m := &pagemap.Map{Book: "hist-fr", Pagination: pagemap.Continuous, PDFPages: 400,
		Chapters: []pagemap.Span{{Chapter: pagemap.WholeVolume, FirstPDF: 7,
			LastPDF: 400, FirstPage: 9, LastPage: 402}}}
	for i := 1; i <= 400; i++ {
		e := pagemap.Entry{PDFPage: i, Confidence: pagemap.Unknown}
		if i >= 7 {
			e.Chapter, e.Page, e.Confidence = pagemap.WholeVolume, i+2, pagemap.FromHead
		}
		m.Entries = append(m.Entries, e)
	}
	return m
}

func TestAContentsOfUnnumberedNotesIsRead(t *testing.T) {
	res, err := Parse([]string{flatContents}, flatMap(),
		Options{Book: "hist-fr", Title: "Éléments d'histoire des mathématiques"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) > 0 {
		t.Errorf("problems: %v", res.Problems)
	}
	c, ok := res.Get(pagemap.WholeVolume)
	if !ok {
		t.Fatal("the implied chapter was never opened")
	}
	// Six notes and not eight. Bibliographie and the index of names are listed
	// the same way the notes are, with a page against each, and read as notes
	// they would put two things in the volume that are not part of its body.
	if len(c.Sections) != 6 {
		t.Fatalf("%d notes, want the 6 that are notes", len(c.Sections))
	}
	first := c.Sections[0]
	if first.Number != 1 || first.Page != 9 {
		t.Errorf("note 1 = %+v, want number 1 on printed page 9", first)
	}
	if first.Title != "Fondements des mathématiques; logique; théorie des ensembles" {
		t.Errorf("note 1 title = %q", first.Title)
	}
	// Numbered in the order they are printed, which is what makes the manifest
	// line up with the English printing note for note.
	for i, s := range c.Sections {
		if s.Number != i+1 {
			t.Errorf("note %d is numbered %d", i+1, s.Number)
		}
	}
	last := c.Sections[5]
	if last.Title != "Divisibilité; corps ordonnés" || last.Page != 110 {
		t.Errorf("last note = %+v, want Divisibilité on 110", last)
	}
	// The chapter takes the page of its first note, the way a chapter that
	// prints no page of its own takes the page of its first §.
	if c.Page != 9 {
		t.Errorf("the chapter starts at printed page %d, want 9", c.Page)
	}
}

// The reading is gated on the volume, not on the line, and this is what that
// buys. The same page against a volume that prints chapters yields nothing,
// because a title with a page on the end of it is only an entry where there is
// no numbering for it to be missing from.
func TestUnnumberedLinesAreNotReadWhereTheVolumeHasChapters(t *testing.T) {
	pm := testMapFor("VIII")
	if flatVolume(pm) {
		t.Fatal("a volume with chapter VIII was taken for a chapterless one")
	}
	if n := contentsLines(flatContents, false); n != 0 {
		t.Errorf("%d lines of an unnumbered contents counted where they should not", n)
	}
	if readsAsContents(flatContents, Grammar{Pilcrow, Bare}, minEntries, false) {
		t.Error("the page read as contents for a volume that prints chapters")
	}
}

// The prose of the front matter must not become notes even in a chapterless
// volume, and what keeps it out is the page number. A sentence has no leaders
// and does not end on a figure, so splitTail refuses it and the reading is
// never reached.
func TestProseInAChapterlessVolumeIsNotReadAsNotes(t *testing.T) {
	const prose = `This history is not a history of mathematics.

It follows the order of the Elements and normally proceeds from the
general to the particular, which is the order of Book I.
`
	if n := contentsLines(prose, true); n != 0 {
		t.Errorf("%d lines of prose read as contents entries", n)
	}
	if readsAsContents(prose, Grammar{Pilcrow, Bare}, minEntries, true) {
		t.Error("a page of prose read as a table of contents")
	}
}

// What the back matter rule turns away, and what it must not.
func TestBackMatterIsNotANote(t *testing.T) {
	for _, s := range []string{"Bibliographie", "Bibliography", "Index des noms cités",
		"Index", "TABLE DES MATIÈRES", "Contents"} {
		if _, ok := flatEntry(s); ok {
			t.Errorf("%q was read as a note", s)
		}
	}
	for _, s := range []string{"Nombres réels", "Espaces fonctionnels",
		"Indices et exposants", "Calcul infinitésimal"} {
		e, ok := flatEntry(s)
		if !ok {
			t.Errorf("%q was turned away", s)
			continue
		}
		if e.kind != kindSection || e.title != s {
			t.Errorf("flatEntry(%q) = %+v", s, e)
		}
	}
}

func TestTheChapterAFlatVolumeNeverPrintsIsMarkedNominal(t *testing.T) {
	res, err := Parse([]string{flatContents}, flatMap(),
		Options{Book: "hist-fr", Title: "Éléments d'histoire des mathématiques"})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := res.Get(pagemap.WholeVolume)
	if !ok {
		t.Fatalf("no chapter %q: %+v", pagemap.WholeVolume, res.Chapters)
	}
	if !c.Nominal {
		t.Errorf("the chapter is not marked nominal, so every consumer reads it as one the printing sets")
	}
}
