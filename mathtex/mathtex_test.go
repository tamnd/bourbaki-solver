package mathtex

import (
	"strings"
	"testing"
)

// The bodies here are written for the test and none of them is Bourbaki. They
// are the shapes the volumes come out in, set with made up letters and made up
// claims, because the corpus this package repairs is under copyright and a test
// file is a bad place to keep a copy of it. Where a case came from a real page
// the comment says which fault it stands for and not what the page says.

func TestSplit(t *testing.T) {
	cases := []struct {
		name  string
		body  string
		spans []string
		open  bool
	}{
		{"none", "no mathematics here at all", nil, false},
		{"inline", "let $x$ be a thing", []string{"x"}, false},
		{"two inline on a line", "if $x$ then $y$", []string{"x", "y"}, false},
		{"display", "we have\n\n$$\nx = y\n$$\n\nas claimed", []string{"\nx = y\n"}, false},
		{
			// A single dollar inside a display is text, not a close.
			"lone dollar in a display",
			"$$\na $ b\n$$",
			[]string{"\na $ b\n"},
			false,
		},
		{
			// An escaped dollar is a dollar sign and opens nothing. Getting this
			// wrong turns one bad region into a whole bad file.
			"escaped dollar",
			`the price is \$5 and $x$ is not`,
			[]string{"x"},
			false,
		},
		{"unclosed inline", "let $x be a thing", nil, true},
		{"unclosed display", "we have $$x = y and that is that", nil, true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			spans, un := Split(c.body)
			if (un != nil) != c.open {
				t.Errorf("unclosed = %v, want %v", un != nil, c.open)
			}
			if len(spans) != len(c.spans) {
				t.Fatalf("got %d spans, want %d: %v", len(spans), len(c.spans), spans)
			}
			for i, want := range c.spans {
				if spans[i].Text != want {
					t.Errorf("span %d is %q, want %q", i, spans[i].Text, want)
				}
			}
		})
	}
}

// The offsets are what a repair puts the text back with, so a span that reports
// the wrong ones rewrites the page around it.
func TestSplitOffsets(t *testing.T) {
	body := "if $a$ and $a$ then"
	spans, un := Split(body)
	if un != nil {
		t.Fatalf("unclosed span in %q", body)
	}
	if len(spans) != 2 {
		t.Fatalf("got %d spans, want 2", len(spans))
	}
	if got := body[spans[0].Start:spans[0].End]; got != "a" {
		t.Errorf("first span is at %q", got)
	}
	if spans[1].Start == spans[0].Start {
		t.Errorf("both copies of the same span reported at %d", spans[0].Start)
	}
	if got := body[spans[1].Start:spans[1].End]; got != "a" {
		t.Errorf("second span is at %q", got)
	}
}

func TestSplitLines(t *testing.T) {
	body := "one\ntwo\nthree $x$ four"
	spans, _ := Split(body)
	if len(spans) != 1 {
		t.Fatalf("got %d spans, want 1", len(spans))
	}
	if spans[0].Line != 3 {
		t.Errorf("span reported on line %d, want 3", spans[0].Line)
	}
}

func TestRepair(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
		n    int
	}{
		{
			"a lower case letter inside the mathematics",
			"the map $α$ is one",
			`the map $\alpha$ is one`,
			1,
		},
		{
			// The fault the whole table exists for: Bourbaki sets a capital
			// upright, so the extractor reads its run as prose and the letter
			// survives to be laid out inside a pair of dollars.
			"a capital inside the mathematics",
			"the group $Γ$ acts",
			`the group $\Gamma$ acts`,
			1,
		},
		{
			// Two commands run together name a command that does not exist.
			"a letter against the letter after it",
			"we set $Γx$ here",
			`we set $\Gamma x$ here`,
			1,
		},
		{
			"a letter against something that is not a letter",
			"we set $Γ_1$ here",
			`we set $\Gamma_1$ here`,
			1,
		},
		{
			// Outside the mathematics the letter is the letter and stays as it
			// is: repairing prose is not this package's business.
			"a letter outside the mathematics",
			"the group Γ acts",
			"the group Γ acts",
			0,
		},
		{
			// The compatibility characters print as the letter and are not the
			// letter, which is why counting the Greek block missed them.
			"the micro sign",
			"the index $µ$ runs",
			`the index $\mu$ runs`,
			1,
		},
		{
			"the ohm sign",
			"the field $Ω$ is closed",
			`the field $\Omega$ is closed`,
			1,
		},
		{
			"the increment sign, which is not a capital delta",
			"the map $∆$ is one",
			`the map $\Delta$ is one`,
			1,
		},
		{
			"nothing to do",
			`the map $\alpha$ is one`,
			`the map $\alpha$ is one`,
			0,
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, n, _ := Repair(c.body)
			if got != c.want {
				t.Errorf("Repair(%q)\n = %q\nwant %q", c.body, got, c.want)
			}
			if n != c.n {
				t.Errorf("Repair(%q) replaced %d characters, want %d", c.body, n, c.n)
			}
		})
	}
}

// A capital sigma is the letter or it is a sum, and the only thing in the
// Markdown that tells them apart is what follows it. Substituting either way
// without looking would be an invention, so the shape decides and the rest is
// handed back.
func TestRepairRefusesAnOperator(t *testing.T) {
	body := "we form $Σ_{i}a_i$ over I"
	got, n, refused := Repair(body)
	if got != body {
		t.Errorf("Repair rewrote %q to %q", body, got)
	}
	if n != 0 {
		t.Errorf("Repair replaced %d characters, want 0", n)
	}
	if len(refused) != 1 {
		t.Fatalf("got %d refusals, want 1", len(refused))
	}
	if !strings.Contains(refused[0].Why, `\sum`) {
		t.Errorf("the refusal does not say what it thinks the character is: %s", refused[0].Why)
	}
	// The same letter with nothing under it is the letter.
	if got, n, _ := Repair("the set $Σ$ is finite"); n != 1 || got != `the set $\Sigma$ is finite` {
		t.Errorf("a bare capital sigma came out as %q with %d replacements", got, n)
	}
}

// Everything before an unclosed delimiter is closed and is repaired as usual.
// Refusing the whole page would leave forty good lines alone for the sake of
// three bad ones.
func TestRepairPastAnUnclosedSpan(t *testing.T) {
	body := "the map $α$ is one and then $β is lost"
	got, n, refused := Repair(body)
	if want := `the map $\alpha$ is one and then $β is lost`; got != want {
		t.Errorf("Repair\n = %q\nwant %q", got, want)
	}
	if n != 1 {
		t.Errorf("Repair replaced %d characters, want 1", n)
	}
	if len(refused) != 1 {
		t.Fatalf("got %d refusals, want 1", len(refused))
	}
	if refused[0].Rune != 0 {
		t.Errorf("a whole span refusal names the character %q", refused[0].Rune)
	}
	if strings.Contains(refused[0].String(), `'\x00'`) {
		t.Errorf("the refusal prints an empty character: %s", refused[0].String())
	}
}

func TestDropStray(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string // the body itself when nothing should be dropped
	}{
		{
			// The fault: a display flattened into a line of prose, with the
			// delimiter that closed it carried along to the full stop.
			"a display delimiter against a full stop",
			"and so we have\n\n(4) long$_A(M) =$ long(B) $$.\n\nwhich proves it",
			"and so we have\n\n(4) long$_A(M) =$ long(B).\n\nwhich proves it",
		},
		{
			"a display delimiter against a comma",
			"we have\n\n[F : K] = (G : H)$$,\n\nand that is all",
			"we have\n\n[F : K] = (G : H),\n\nand that is all",
		},
		{
			"the delimiter with no space in front of it",
			"we have Card(G)$$.\n\nand that is all",
			"we have Card(G).\n\nand that is all",
		},
		{
			// A single stray dollar is a mangled inline formula far more often
			// than it is a leftover. On one page of chapter VIII taking it out
			// broke a formula that read correctly.
			"an inline delimiter is left alone",
			"we have $f=a, and therefore this",
			"we have $f=a, and therefore this",
		},
		{
			// A $$ on a line of its own is a display delimiter doing its job.
			// The page it stands on lost the opening one, which is a different
			// fault and not one that can be repaired by counting.
			"a display delimiter on its own line is left alone",
			"we have\n\n$$\nx = y\n\nand that is all",
			"we have\n\n$$\nx = y\n\nand that is all",
		},
		{
			"a delimiter with prose after it is left alone",
			"we have $$ x = y and that is all",
			"we have $$ x = y and that is all",
		},
		{
			// The check: if the page does not balance without it, the damage is
			// somewhere else and this is not the repair for it.
			"a page that does not balance without it is left alone",
			"we have x$$.\n\nand then $z is lost",
			"we have x$$.\n\nand then $z is lost",
		},
		{
			"nothing to do",
			"we have $x = y$ and that is all",
			"we have $x = y$ and that is all",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, ok := DropStray(c.body)
			if want := c.body != c.want; ok != want {
				t.Errorf("DropStray(%q) reported %v, want %v", c.body, ok, want)
			}
			if got != c.want {
				t.Errorf("DropStray(%q)\n = %q\nwant %q", c.body, got, c.want)
			}
		})
	}
}

// Both repairs run over every page and most pages need neither, so the one
// thing they must never do is come back with a body that is not the one they
// were given.
func TestNothingToDoChangesNothing(t *testing.T) {
	body := "a paragraph with $x$ in it and a display\n\n$$\ny = z\n$$\n\nand nothing wrong."
	if got, n, refused := Repair(body); got != body || n != 0 || len(refused) != 0 {
		t.Errorf("Repair changed a body it had nothing to do to")
	}
	if got, ok := DropStray(body); got != body || ok {
		t.Errorf("DropStray changed a body it had nothing to do to")
	}
}
