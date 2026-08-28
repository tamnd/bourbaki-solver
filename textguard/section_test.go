package textguard

import "testing"

func TestSectionSignWritesTheCorpusSign(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
		n    int
	}{
		{
			"an escaped dollar set open, which is what the reading of Integration sends",
			`ce qui prouve la proposition (\$ 1, n° 8).`,
			`ce qui prouve la proposition (§ 1, n° 8).`,
			1,
		},
		{
			"an escaped dollar set close",
			`est dense dans $\mathcal{K}(X;E)$ (\$1, n° 2, prop. 5)`,
			`est dense dans $\mathcal{K}(X;E)$ (§ 1, n° 2, prop. 5)`,
			1,
		},
		{
			"the LaTeX spelling",
			`en passant à la limite (\S 1, n° 3, th. 3)`,
			`en passant à la limite (§ 1, n° 3, th. 3)`,
			1,
		},
		{
			"two references in one sentence, which is the common shape",
			`(\S 5, n° 6, th. 5) et (\$ 3, n° 4, déf. 2)`,
			`(§ 5, n° 6, th. 5) et (§ 3, n° 4, déf. 2)`,
			2,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, n := SectionSign(tc.body)
			if got != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
			if n != tc.n {
				t.Errorf("count = %d, want %d", n, tc.n)
			}
			again, m := SectionSign(got)
			if again != got || m != 0 {
				t.Errorf("a second pass changed %q to %q, %d signs", got, again, m)
			}
		})
	}
}

func TestSectionSignLeavesWhatIsNotASectionSign(t *testing.T) {
	// A backslash in front of a letter is the start of a longer command, and a
	// backslash in front of a dollar that no numeral follows is a dollar
	// somebody meant to print.
	for _, body := range []string{
		`$\Sigma_n$ est le groupe symétrique`,
		`$A \Subset B$`,
		`\Sigma 1 is a command and a numeral, not a sign`,
		`the escape stands for itself here: \$ and nothing after it`,
	} {
		got, n := SectionSign(body)
		if got != body || n != 0 {
			t.Errorf("%q became %q, %d signs", body, got, n)
		}
	}
}

func TestSectionsReportsOneFindingPerLine(t *testing.T) {
	body := "d'où (\\S 1, n° 3)\nrien ici\nla proposition (\\$ 4, exerc. 5)"
	got := Sections(body)
	if len(got) != 2 {
		t.Fatalf("findings = %v, want two", got)
	}
	if got[0].Line != 1 || got[1].Line != 3 {
		t.Errorf("lines = %d and %d, want 1 and 3", got[0].Line, got[1].Line)
	}
	if got[0].Name != "a section sign written as a LaTeX command" {
		t.Errorf("first name = %q", got[0].Name)
	}
	if got[1].Name != "a section sign written as an escaped dollar" {
		t.Errorf("second name = %q", got[1].Name)
	}
}
