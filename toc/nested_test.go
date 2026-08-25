package toc

import "testing"

// The cases are the five shapes a numbered contents line takes in this library,
// measured over the text the parser reads. A line sits at the run indent give or
// take the width of its own number; a nested list sits three or more past it;
// and a run whose § line the reading lost also sits past it, but opens at no. 1
// or goes on from the no. before.
func TestNested(t *testing.T) {
	line := func(indent int) string {
		s := ""
		for i := 0; i < indent; i++ {
			s += " "
		}
		return s + "2. Groupe affine .................. 80"
	}
	cases := []struct {
		name                 string
		indent, runIndent    int
		num, lastNo, nestNum int
		want                 bool
	}{
		// The English Integration 7 to 9 sets its examples fifteen columns past
		// the run. Nothing else in the library is set that far in, so the number
		// is not consulted.
		{"the English example list opens", 19, 4, 2, 3, 0, true},
		{"the English example list goes on", 19, 4, 3, 3, 2, true},

		// The French Integration 7 and 8 sets the same list four columns past.
		// There the number has to agree that the count has restarted.
		{"the French example list opens", 8, 4, 2, 3, 0, true},
		{"the French example list goes on", 8, 4, 3, 3, 2, true},
		// The line that broke the first attempt at this. Example 4 follows the
		// § own no. 3 by one, because filing the examples leaves lastNo where
		// the § had got to, so the test that opens a list would refuse it. Once
		// the list is open it is the list the line has to go on with.
		{"the French example list reaches example 4", 8, 4, 4, 3, 3, true},

		// The French General Topology 5 to 10 opens a run four columns past the
		// one above it, because the § line between them was not read as one. It
		// opens at no. 1, which no restarted list inside an entry does.
		{"a lost § opens a run four columns past", 8, 4, 1, 10, 0, false},
		{"and the run goes on from it", 8, 4, 2, 1, 0, false},

		// The French Algebra 1 to 3 does the same thing two columns past, which
		// is inside the width of a number and never reaches the question.
		{"a run two columns past is only its own number's width", 6, 4, 1, 13, 0, false},

		// The English Algebra 4 to 7 skips from no. 5 to no. 7 two columns past
		// the run. The gap is a no. the reading lost and is worth reporting, and
		// filing it as a list would bury it.
		{"a gap in a run is not a list", 6, 4, 7, 5, 0, false},

		// A list that runs onto the next page has nothing above it to be read
		// against, and the numbering carries it over on its own.
		{"the list carries over onto the next page", 8, -1, 5, 3, 4, true},
		{"and stops where the numbering stops", 8, -1, 9, 3, 4, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := nested(line(c.indent), c.runIndent, c.num, c.lastNo, c.nestNum)
			if got != c.want {
				t.Errorf("nested(indent %d, run %d, no. %d, last %d, nest %d) = %v, want %v",
					c.indent, c.runIndent, c.num, c.lastNo, c.nestNum, got, c.want)
			}
		})
	}
}
