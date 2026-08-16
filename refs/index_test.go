package refs

import "testing"

// A volume that labels its pages and one that numbers them straight through
// write their runs differently, and both shapes reach the index through the same
// front matter field. Theory of Sets has no page label at all: the printed page
// is a bare folio, so a § of it is written "15-23" where a § of Algebra VIII is
// written "A VIII.15-A VIII.23".
//
// The lone run at the end of the last case is the one worth having a test for.
// Read by a pattern over the whole string, "15-23, 56" gives four numbers and a
// pair of them is taken for one run, which drops page 56 out of every § of the
// corpus. The exercises of a § of Theory of Sets are often a single page.
func TestPageRunsReadsBothWaysAVolumeNumbersItsPages(t *testing.T) {
	for _, c := range []struct {
		pages string
		want  []Run
	}{
		{"A VIII.69-A VIII.84", []Run{{69, 84}}},
		{"A VIII.69-A VIII.84, A VIII.218-A VIII.226", []Run{{69, 84}, {218, 226}}},
		{"A VIII.15", []Run{{15, 15}}},
		{"15-23", []Run{{15, 23}}},
		{"15-23, 56", []Run{{15, 23}, {56, 56}}},
		{"", nil},
	} {
		got := pageRuns(c.pages)
		if len(got) != len(c.want) {
			t.Errorf("pageRuns(%q) = %v, want %v", c.pages, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("pageRuns(%q)[%d] = %v, want %v", c.pages, i, got[i], c.want[i])
			}
		}
	}
}
