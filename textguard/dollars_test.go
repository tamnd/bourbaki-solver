package textguard

import (
	"strings"
	"testing"
)

func TestDollarsWritesTheCorpusDelimiters(t *testing.T) {
	for _, tc := range []struct {
		name string
		body string
		want string
		n    int
	}{
		{
			"a display on one line",
			`\[ f(x) = \sum_{n \ge 0} a_n x^n \]`,
			`$$ f(x) = \sum_{n \ge 0} a_n x^n $$`,
			2,
		},
		{
			"a display over three lines, as a solution writes one",
			"\\[\n\\varphi(x) = 0\n\\]",
			"$$\n\\varphi(x) = 0\n$$",
			2,
		},
		{
			"an inline span",
			`for every $x$ in \(E\) the map is open`,
			`for every $x$ in $E$ the map is open`,
			2,
		},
		{
			"both spellings in one paragraph, which is what a model actually sends",
			"The set \\(A\\) is closed, and\n\\[ A = \\bigcap_n U_n \\]\nholds.",
			"The set $A$ is closed, and\n$$ A = \\bigcap_n U_n $$\nholds.",
			4,
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, n := Dollars(tc.body)
			if got != tc.want {
				t.Errorf("got  %q\nwant %q", got, tc.want)
			}
			if n != tc.n {
				t.Errorf("counted %d, want %d", n, tc.n)
			}
		})
	}
}

// The row break of a matrix is the one thing in the corpus that looks like a
// display and is not, and there are a good many of them. Every case here is a
// spelling some file of this corpus actually uses.
func TestDollarsLeavesARowBreakAlone(t *testing.T) {
	for _, tc := range []string{
		`a & b \\[2pt] c & d`,
		`a & b \\[2mm] c & d`,
		`a & b \\[4mm] c & d`,
		`a & b \\[2ex] c & d`,
		`a & b \\[-1ex] c & d`,
		`a & b \\[-1.5em] c & d`,
		`\begin{pmatrix} 1 & 0 \\ 0 & 1 \end{pmatrix}`,
	} {
		t.Run(tc, func(t *testing.T) {
			got, n := Dollars(tc)
			if got != tc {
				t.Errorf("got %q, want it unchanged", got)
			}
			if n != 0 {
				t.Errorf("counted %d, want 0", n)
			}
		})
	}
}

// Running it twice has to come out the same as running it once, or bourbaki fix
// dollars cannot be run over the corpus more than once.
func TestDollarsIsIdempotent(t *testing.T) {
	body := "The set \\(A\\) is closed and\n\\[ A = \\bigcap_n U_n \\]\nwith a row break a & b \\\\[2pt] c & d."
	once, n := Dollars(body)
	if n != 4 {
		t.Fatalf("counted %d on the first pass, want 4", n)
	}
	twice, n := Dollars(once)
	if n != 0 || twice != once {
		t.Errorf("a second pass counted %d and gave %q", n, twice)
	}
}

func TestBracketsReportsTheLineAndNamesTheDelimiter(t *testing.T) {
	body := strings.Join([]string{
		"Let $E$ be a set, which is written the corpus's way.",
		`\[ f(x) = 0 \]`,
		`a & b \\[2pt] c & d, which is a row break and no display`,
		`and \(x\) is an element`,
	}, "\n")
	got := Brackets(body)
	if len(got) != 2 {
		t.Fatalf("got %d findings, want 2: %+v", len(got), got)
	}
	if got[0].Line != 2 || got[0].Name != "a display opened with a bracket" {
		t.Errorf("first is %+v", got[0])
	}
	if got[1].Line != 4 || got[1].Name != "a span opened with a parenthesis" {
		t.Errorf("second is %+v", got[1])
	}
	if !strings.Contains(got[1].Text, "is an element") {
		t.Errorf("second carries the wrong line: %q", got[1].Text)
	}
}

// A solution is Markdown somebody wrote, and its bullet lists are lists. This is
// the whole reason the solver gets its own seam rather than calling Normalise.
func TestNormaliseProseLeavesABulletListAlone(t *testing.T) {
	body := "There are two cases.\n\n* $x$ lies in $A$.\n* $x$ does not.\n"
	if got := NormaliseProse(body); got != body {
		t.Errorf("got %q, want it unchanged", got)
	}
	if got := Normalise(body); got == body {
		t.Fatal("Normalise left the list alone, and the split is pointless")
	}
}

// Everything else Normalise does, a solution wants done to it.
func TestNormaliseProseDoesTheRest(t *testing.T) {
	body := "The ring \\(\\mathbb{Z}\\) is principal.   \n\\[ \\mathbb Q = \\operatorname{Frac}(\\mathbb{Z}) \\]\n⚠ Careful."
	want := "The ring $\\mathbf{Z}$ is principal.\n$$ \\mathbf{Q} = \\operatorname{Frac}(\\mathbf{Z}) $$\n☡ Careful."
	if got := NormaliseProse(body); got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
}
