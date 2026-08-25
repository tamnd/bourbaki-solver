package textguard

import "testing"

func TestTightenClosesUpAnInlineSpan(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
		n    int
	}{
		{
			"the span that started this, from the formal power series",
			`the ring $ K[[T]] $ of formal power series`,
			`the ring $K[[T]]$ of formal power series`,
			1,
		},
		{
			"padded on the opening side only",
			`for every $ x$ in E`,
			`for every $x$ in E`,
			1,
		},
		{
			"padded on the closing side only",
			`for every $x $ in E`,
			`for every $x$ in E`,
			1,
		},
		{
			"several spans in one sentence, some loose and some not",
			`if $ f $ is continuous and $g$ is open then $ f \circ g $ is open`,
			`if $f$ is continuous and $g$ is open then $f \circ g$ is open`,
			2,
		},
		{
			"a tab, which the translator sends and the OCR does not",
			"the map $\tx\t$ is open",
			`the map $x$ is open`,
			1,
		},
		{
			"the space inside the formula stays, only the ends are read",
			`$ f(x) = \sum_{n \ge 0} a_n x^n $`,
			`$f(x) = \sum_{n \ge 0} a_n x^n$`,
			1,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, n := Tighten(tc.body)
			if got != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
			if n != tc.n {
				t.Errorf("counted %d, want %d", n, tc.n)
			}
		})
	}
}

// A display is set on lines of its own, so the whitespace inside it is the line
// break that puts it there. Closing that up would run every formula in the
// corpus into its delimiters and change the shape of the file for nothing.
func TestTightenLeavesADisplayAlone(t *testing.T) {
	for _, tc := range []string{
		"$$\n\\varphi(x) = 0\n$$",
		`$$ f(x) = \sum_{n \ge 0} a_n x^n $$`,
		"paragraph before\n\n$$\nA = \\bigcap_n U_n\n$$\n\nparagraph after",
	} {
		t.Run(tc, func(t *testing.T) {
			got, n := Tighten(tc)
			if got != tc {
				t.Errorf("got %q, want it unchanged", got)
			}
			if n != 0 {
				t.Errorf("counted %d, want 0", n)
			}
		})
	}
}

func TestTightenLeavesTheseAlone(t *testing.T) {
	for _, tc := range []struct{ name, body string }{
		{"already tight", `the ring $K[[T]]$ of formal power series`},
		{
			// $ $ and $$ both say nothing, and the second is not an inline span
			// at all: it opens a display. Closing up a blank span would turn a
			// formula that is merely empty into a delimiter that swallows the
			// rest of the paragraph.
			"a blank span",
			`the ring $ $ of formal power series`,
		},
		{
			// The offsets after a delimiter that never closes are not the spans
			// anybody meant. M01 reports the file and somebody opens the page.
			"a span left open, which is M01's to report and not this to guess at",
			`the ring $ K[[T]] of formal power series`,
		},
		{
			// \$ is a dollar sign and not a delimiter, which is LaTeX's rule and
			// mathtex's.
			"an escaped dollar in the prose",
			`costs \$ 5 \$ and no formula is opened`,
		},
		{
			// Pulling the dollar down onto the next line or the text up onto the
			// previous one reflows the paragraph around it, and a repair that
			// changes lines nobody asked about cannot be reviewed.
			"a span that runs across a line break keeps its newline",
			"the map $x\ny$ is open",
		},
		{"no mathematics at all", "an ordinary sentence with no dollars in it"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, n := Tighten(tc.body)
			if got != tc.body {
				t.Errorf("got %q, want it unchanged", got)
			}
			if n != 0 {
				t.Errorf("counted %d, want 0", n)
			}
		})
	}
}

// A span padded at one end and sitting against a newline at the other gets the
// end that can be closed up and keeps the end that cannot.
func TestTightenClosesUpTheEndItCan(t *testing.T) {
	body := "the map $ x\ny $ is open"
	want := "the map $x\ny$ is open"
	got, n := Tighten(body)
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if n != 1 {
		t.Errorf("counted %d, want 1", n)
	}
}

// Running it twice has to come out the same as running it once, or bourbaki fix
// padding cannot be run over the corpus more than once.
func TestTightenIsIdempotent(t *testing.T) {
	body := "if $ f $ is continuous then\n$$\nf \\circ g\n$$\nis open, and $ g $ is too."
	once, n := Tighten(body)
	if n != 2 {
		t.Fatalf("counted %d on the first pass, want 2", n)
	}
	twice, n := Tighten(once)
	if n != 0 {
		t.Errorf("counted %d on the second pass, want 0", n)
	}
	if twice != once {
		t.Errorf("the second pass moved it:\ngot  %q\nwant %q", twice, once)
	}
}

func TestPaddedReadsWithoutRepairing(t *testing.T) {
	body := "line one has $x$ tight\nline two has $ y $ loose\n$$\nz\n$$\nline six has $ w $ loose"
	got := Padded(body)
	if len(got) != 2 {
		t.Fatalf("found %d, want 2: %+v", len(got), got)
	}
	for i, want := range []Pad{{Line: 2, Text: " y "}, {Line: 6, Text: " w "}} {
		if got[i] != want {
			t.Errorf("finding %d is %+v, want %+v", i, got[i], want)
		}
	}
}

// The audit reads the same body the repair does, so a file the repair refuses is
// a file the audit says nothing about. M01 has that one.
func TestPaddedSaysNothingAboutABodyWithASpanLeftOpen(t *testing.T) {
	if got := Padded("the ring $ K[[T]] of formal power series"); got != nil {
		t.Errorf("got %+v, want nothing", got)
	}
}
