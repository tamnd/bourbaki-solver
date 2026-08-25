package toc

import "testing"

// A § whose line carries no page is not an error either, and giving it up costs
// more than the § itself. The nos under it do not go with it: they are
// committed to whatever § is still open, which is the one above, and that § then
// holds two runs of nos end to end. Every no. of the second run is then reported
// as missing or doubled, so one lost page number is reported once per no.
//
// The French Functions of a Real Variable prints "§ 2. Équations différentielles
// linéaires" with its leaders and no page at all, and chapter IV came out with
// one § holding sixteen nos rather than two holding seven and nine. The French
// General Topology loses the page off § 6 of chapter IV the same way.
func TestSectionWithNoPageTakesItsFirstSubsection(t *testing.T) {
	const pg = `CHAPTER I STRUCTURES TOPOLOGIQUES . . . . . . . . . . . . . . . . . . 1
§ 1. Existence theorems . . . . . . . . . . . . . . . . . . . . . . . . 1
      1. Decomposition of a family . . . . . . . . . . . . . . . . . . . 1
      2. The case of a linear family . . . . . . . . . . . . . . . . . . 6
§ 2. Linear differential equations . . . . . . . . . . . . . . . . . . .
      1. Existence of solutions . . . . . . . . . . . . . . . . . . . . 10
      2. Continuity of the solutions . . . . . . . . . . . . . . . . . . 12
`
	res, err := Parse([]string{pg}, testMapFor("I"), Options{Book: "test", Chapters: []string{"I"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Problems) > 0 {
		t.Errorf("problems: %v", res.Problems)
	}
	c, ok := res.Get("I")
	if !ok {
		t.Fatal("no chapter I")
	}
	if len(c.Sections) != 2 {
		t.Fatalf("chapter I has %d §, want 2", len(c.Sections))
	}
	// The § opens where its no. 1 opens, which is the fact validate leans on
	// everywhere else, so it is the fact the page is filled in from.
	if got := c.Sections[1]; got.Number != 2 || got.Page != 10 {
		t.Errorf("§ 2 = no. %d at printed page %d, want no. 2 at 10", got.Number, got.Page)
	}
	// The nos are the point. Two runs of two, not one run of four.
	for i, want := range []int{2, 2} {
		if got := len(c.Sections[i].Subsections); got != want {
			t.Errorf("§ %d holds %d no., want %d", c.Sections[i].Number, got, want)
		}
	}
}
