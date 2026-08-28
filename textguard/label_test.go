package textguard

import "testing"

func TestLabelsTakesTheBracketedFormOutOfTheMathematics(t *testing.T) {
	const in = "$a)$ Show that $E$ is complete.\n\n$b)$ Deduce from $a)$ that $E$ is closed.\n"
	const want = "a) Show that $E$ is complete.\n\nb) Deduce from a) that $E$ is closed.\n"
	got, n := Labels(in)
	if got != want {
		t.Errorf("Labels() wrote\n%q\nwant\n%q", got, want)
	}
	if n != 3 {
		t.Errorf("Labels() moved %d labels, want 3", n)
	}
}

func TestLabelsTakesTheHeadOfALineOnly(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want string
		n    int
	}{
		{"a label at the head of a line", "$a$) Show that $E$ is complete.\n", "a) Show that $E$ is complete.\n", 1},
		{"a label on the first line of the body", "$c$) Give an example.", "c) Give an example.", 1},
		{"a variable closing a prose bracket", "It is compatible (in $x$) with the law.\n", "It is compatible (in $x$) with the law.\n", 0},
		{"a variable at the end of a parenthesis", "the completion (with respect to $f$)\n", "the completion (with respect to $f$)\n", 0},
		{"a label mid sentence with its bracket inside", "Deduce from $d)$ that it holds.\n", "Deduce from d) that it holds.\n", 1},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, n := Labels(c.in)
			if got != c.want {
				t.Errorf("Labels(%q) = %q, want %q", c.in, got, c.want)
			}
			if n != c.n {
				t.Errorf("Labels(%q) moved %d, want %d", c.in, n, c.n)
			}
		})
	}
}

func TestLabelsLeavesTheMathematicsAlone(t *testing.T) {
	cases := []string{
		"Let $f(a)$ be the value at $a$.\n",
		"The interval $[0, a)$ is half open.\n",
		"$$a)$$\n",
		"For every $x$ in $E$ there is a $y$ with $x + y = 0$.\n",
		"Apply the result of $a), b), d)$ of Def. 1.\n",
		"Take $G$ to be $\\mathfrak{g}$ (use $f$)$)$.\n",
		"An escaped \\$a)\\$ is not a span.\n",
	}
	for _, in := range cases {
		got, n := Labels(in)
		if got != in || n != 0 {
			t.Errorf("Labels(%q) = %q with %d moved, want it unchanged", in, got, n)
		}
	}
}

func TestLabelsCountsWhatItWrote(t *testing.T) {
	const in = "$a)$ one\n\n$b)$ two\n\n$c$) three\n\n$d)$ four, and see $a)$ again.\n"
	got, n := Labels(in)
	if n != 5 {
		t.Errorf("Labels() moved %d labels, want 5", n)
	}
	again, m := Labels(got)
	if m != 0 || again != got {
		t.Errorf("Labels() is not idempotent: a second pass moved %d and wrote %q", m, again)
	}
}
