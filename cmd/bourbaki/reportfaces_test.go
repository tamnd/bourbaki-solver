package main

import "testing"

// The argument is taken whole rather than a letter at a time. \mathfrak{su} is
// a thing the books set and splitting it would report two letters the printing
// does not have.
func TestFaceArgumentsAreTakenWhole(t *testing.T) {
	for _, c := range []struct {
		text string
		want []string
	}{
		{`$\mathfrak{su}(2)$`, []string{"mathfrak su"}},
		{`$\mathcal{G}$ and $\mathcal G$`, []string{"mathcal G", "mathcal G"}},
		{`$\mathcal{CV}$`, []string{"mathcal CV"}},
		// Neither of the other two faces is this report's business: the script
		// and bold ones are what faceChanges watches.
		{`$\mathscr{T}$ and $\mathbf{R}$`, nil},
		{`no mathematics here at all`, nil},
	} {
		var got []string
		for _, m := range faceArg.FindAllStringSubmatch(c.text, -1) {
			arg := m[2]
			if arg == "" {
				arg = m[3]
			}
			got = append(got, m[1]+" "+arg)
		}
		if len(got) != len(c.want) {
			t.Errorf("%s gave %v, want %v", c.text, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s gave %v, want %v", c.text, got, c.want)
				break
			}
		}
	}
}

// A face on every page of a volume is not the shape the report is looking for
// and its page list says nothing, so the list is cut off.
func TestThePageListIsCutOffWhereItStopsSayingAnything(t *testing.T) {
	pages := []int{16, 54, 160, 180, 181}
	if got, want := facePages(pages, 3), "0016 0054 0160 and 2 more"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if got, want := facePages(pages, 0), "0016 0054 0160 0180 0181"; got != want {
		t.Errorf("asking for all of them gave %q, want %q", got, want)
	}
	if got, want := facePages(pages, 5), "0016 0054 0160 0180 0181"; got != want {
		t.Errorf("a cut at exactly the length gave %q, want %q", got, want)
	}
}
