package corpus

import "testing"

// The bodies here are the shapes real pages of Theory of Sets came back in. The
// folio stands alone on the last line because that is where the volume prints
// it, and the point of the test is that nothing else on a last line is mistaken
// for one.
func TestCutFolio(t *testing.T) {
	for _, c := range []struct {
		name string
		body string
		want string
		n    int
	}{
		{
			name: "the ordinary page",
			body: "A relation is said to be *transitive* if\n\n$$x < y \\text{ and } y < z \\implies x < z$$\n\n289\n",
			want: "A relation is said to be *transitive* if\n\n$$x < y \\text{ and } y < z \\implies x < z$$\n",
			n:    289,
		},
		{
			name: "nothing but the number",
			body: "294\n",
			want: "",
			n:    294,
		},
		{
			name: "the volume prints no number on this page",
			body: "## § 1. THE THEORY OF SETS\n",
			want: "## § 1. THE THEORY OF SETS\n",
		},
		{
			name: "a number that is part of a sentence is not a folio",
			body: "and this is theorem 3\n",
			want: "and this is theorem 3\n",
		},
		{
			name: "a display that ends in a number is not a folio either",
			body: "$$n = 12$$\n",
			want: "$$n = 12$$\n",
		},
		{
			name: "the model dropped the number, so the page keeps none",
			body: "the set of such $x$ is empty.\n",
			want: "the set of such $x$ is empty.\n",
		},
		{
			name: "a page numbered zero is a reading error and is left alone",
			body: "text\n\n0\n",
			want: "text\n\n0\n",
		},
		{
			name: "trailing blank lines do not hide the folio",
			body: "text\n\n17\n\n\n",
			want: "text\n",
			n:    17,
		},
		{
			name: "five digits is not a page number of a book",
			body: "text\n\n12345\n",
			want: "text\n\n12345\n",
		},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, n := CutFolio(c.body)
			if n != c.n {
				t.Errorf("read folio %d, want %d", n, c.n)
			}
			if got != c.want {
				t.Errorf("body came back %q, want %q", got, c.want)
			}
		})
	}
}

// A body the folio has already been taken off must come back untouched, because
// fix folio is run more than once over a volume and the second run has to be a
// no-op rather than a second cut.
func TestCutFolioTwiceCutsOnce(t *testing.T) {
	body := "The axiom of choice.\n\n41\n"
	once, n := CutFolio(body)
	if n != 41 {
		t.Fatalf("first cut read %d, want 41", n)
	}
	twice, n := CutFolio(once)
	if n != 0 || twice != once {
		t.Errorf("second cut took %d off and left %q", n, twice)
	}
}
