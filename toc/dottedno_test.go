package toc

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/pagemap"
)

// The French Varietes differentielles et analytiques is a fascicule de
// resultats and numbers its nos § point no., so § 1 runs 1.1 to 1.7 where every
// other volume in the library runs 1 to 7. classify reads the first component
// as the number and leaves the second at the head of the title, so all seven
// nos of § 1 came out as no. 1 and the § was reported as holding seven of them.
//
// The second period is optional because the printing is not consistent about
// it: the fascicule sets no. 1 of § 1 with none and no. 2 with one, on
// consecutive lines of the same page.
func TestNosNumberedSectionPointNoAreReadAsNos(t *testing.T) {
	const pg = `§ 1. Cartes et atlas . . . . . . . . . . . . . . . . . . . . . . . . . 1
      1.1  Cartes . . . . . . . . . . . . . . . . . . . . . . . . . . . 1
      1.2. Atlas . . . . . . . . . . . . . . . . . . . . . . . . . . . . 2
      1.3. Changements de cartes . . . . . . . . . . . . . . . . . . . . 4
§ 2. Varietes . . . . . . . . . . . . . . . . . . . . . . . . . . . . . 6
      2.1. Definition . . . . . . . . . . . . . . . . . . . . . . . . . 6
      2.2. Exemples . . . . . . . . . . . . . . . . . . . . . . . . . . 8
`
	res, err := Parse([]string{pg}, testMapFor(pagemap.WholeVolume),
		Options{Book: "var-fr", Title: "Varietes differentielles et analytiques"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) > 0 {
		t.Errorf("problems: %v", res.Problems)
	}
	c, ok := res.Get(pagemap.WholeVolume)
	if !ok {
		t.Fatal("the volume opened no chapter over its §")
	}
	if len(c.Sections) != 2 {
		t.Fatalf("the volume has %d §, want 2", len(c.Sections))
	}
	for i, want := range [][]string{
		{"Cartes", "Atlas", "Changements de cartes"},
		{"Definition", "Exemples"},
	} {
		s := c.Sections[i]
		if len(s.Subsections) != len(want) {
			t.Errorf("§ %d holds %d no., want %d", s.Number, len(s.Subsections), len(want))
			continue
		}
		for j, title := range want {
			sub := s.Subsections[j]
			if sub.Number != j+1 {
				t.Errorf("§ %d no. %d is numbered %d", s.Number, j+1, sub.Number)
			}
			// The second component has to come off the head of the title as
			// well as be read as the number, or every no. carries its own
			// numbering twice over.
			if sub.Title != title {
				t.Errorf("§ %d no. %d is titled %q, want %q", s.Number, j+1, sub.Title, title)
			}
		}
	}
}

// The reading is gated on the number of the § that is already open, and that is
// the whole reason it is safe. A rule firing on any line that opens with two
// numbers separated by a period would have to be trusted against every contents
// in the library, where a title is free to open with a figure. Requiring the
// first component to be the § the line is already under means a title would have
// to open with the number of its own § to be mistaken for one.
func TestATitleOpeningWithAFigureIsNotReadAsSectionPointNo(t *testing.T) {
	const pg = `§ 3. Groupes classiques . . . . . . . . . . . . . . . . . . . . . . . . 1
      1. 2. 3. Comme suite d'entiers . . . . . . . . . . . . . . . . . . 1
      2. 4. Racines de l'unite . . . . . . . . . . . . . . . . . . . . . 3
`
	res, err := Parse([]string{pg}, testMapFor(pagemap.WholeVolume),
		Options{Book: "test", Title: "Test"})
	if err != nil {
		t.Fatal(err)
	}
	c, ok := res.Get(pagemap.WholeVolume)
	if !ok {
		t.Fatal("the volume opened no chapter over its §")
	}
	if len(c.Sections) != 1 {
		t.Fatalf("the volume has %d §, want 1", len(c.Sections))
	}
	subs := c.Sections[0].Subsections
	if len(subs) != 2 {
		t.Fatalf("§ 3 holds %d no., want 2", len(subs))
	}
	// Neither line is under § 1 or § 2, so neither is a § point no. and both
	// keep the number classify read and the title that follows it.
	if subs[0].Number != 1 || subs[0].Title != "2. 3. Comme suite d'entiers" {
		t.Errorf("no. 1 = %d %q, want 1 and the figure left on the title",
			subs[0].Number, subs[0].Title)
	}
	if subs[1].Number != 2 || subs[1].Title != "4. Racines de l'unite" {
		t.Errorf("no. 2 = %d %q, want 2 and the figure left on the title",
			subs[1].Number, subs[1].Title)
	}
}
