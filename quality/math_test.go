package quality

import (
	"strings"
	"testing"
)

// The bodies here are written for the test. None of them is Bourbaki: the
// corpus these rules audit is under copyright and a test file is a bad place to
// keep a copy of it, so each case is the shape of a fault set in invented text.
//
// Every case is a fault the corpus actually had. The comments say which, and
// what the rule was measured against, because a rule whose numbers came from
// nowhere is a rule nobody can argue with.

// doc is one content file as the rules see it. head is 1 because a body line
// and a file line only differ by the front matter, which is what BodyLine is
// for and is tested separately.
func doc(path, body string) Doc {
	return Doc{Path: path, Lang: "en", Kind: KindSection, Body: body, head: 1}
}

func run(t *testing.T, rule func(*Corpus) ([]Finding, error), docs ...Doc) []Finding {
	t.Helper()
	out, err := rule(&Corpus{Docs: docs})
	if err != nil {
		t.Fatalf("the rule returned an error: %v", err)
	}
	return out
}

func TestM01(t *testing.T) {
	clean := doc("a.md", "a paragraph with $x$ in it\n\n$$\ny = z\n$$\n\nand nothing else")
	if got := run(t, m01, clean); len(got) != 0 {
		t.Errorf("a balanced file was reported: %v", got)
	}

	// A span left open runs to the end of the file, which is why one finding
	// per file is right and a finding per line would be noise.
	open := doc("b.md", "one\ntwo\nthree $x = y\nfour\nfive")
	got := run(t, m01, open)
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(got), got)
	}
	if got[0].Line != 3 {
		t.Errorf("the finding is on line %d, want 3, the line the delimiter is on", got[0].Line)
	}
	if !strings.Contains(got[0].Msg, "inline") {
		t.Errorf("the finding does not say which kind of span: %s", got[0].Msg)
	}

	display := doc("c.md", "one\n$$\nx = y\n\nand on it goes")
	got = run(t, m01, display)
	if len(got) != 1 || !strings.Contains(got[0].Msg, "display") {
		t.Errorf("an unclosed display came out as %v", got)
	}
}

// M02 is the rule that is checked rather than trusted: a model asked to
// transcribe mathematics writes \mathbb because almost every other book does,
// and Bourbaki sets its number fields \mathbf.
func TestM02(t *testing.T) {
	if got := run(t, m02, doc("a.md", `the field $\mathbf{Q}$ is prime`)); len(got) != 0 {
		t.Errorf("\\mathbf was reported: %v", got)
	}
	got := run(t, m02, doc("a.md", `the field $\mathbb{Q}$ is prime`))
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(got), got)
	}
	if !strings.Contains(got[0].Msg, `\mathbf`) {
		t.Errorf("the finding does not say what belongs there: %s", got[0].Msg)
	}
}

func TestM03(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string // a phrase the finding has to carry, empty for no finding
	}{
		{
			// The fault the rule was written for: Bourbaki sets its capital
			// Greek upright, so the extractor reads the run as prose and the
			// letter survives inside a pair of dollars.
			"a capital inside the mathematics",
			`we have $\lambda \in Λ$ here`,
			"the letter",
		},
		{
			// Outside the mathematics the letter reads correctly, and putting
			// it into TeX would be a rewrite of the transcription rather than a
			// repair of it.
			"a capital outside the mathematics",
			`we have Λ and $\lambda$ here`,
			"",
		},
		{
			// The compatibility characters. These print correctly and read
			// correctly and the only way to see one is to count.
			"the micro sign",
			"the index $µ$ runs",
			"the letter",
		},
		{
			"the ohm sign",
			"the field $Ω$ is closed",
			"the letter",
		},
		{
			"the increment sign, which is not a capital delta",
			"the map $∆$ is one",
			"the operator",
		},
		{
			// A replacement glyph means a character was lost between the page
			// and the file, wherever it turns up.
			"a replacement glyph in the prose",
			"the map is one� here",
			"lost",
		},
		{
			// An accent standing on its own is the hat of a \widehat that came
			// away from what it was over. It is reported and not repaired,
			// because putting it back means deciding which symbol it covered.
			"a spacing accent with no letter under it",
			"the map Gˆ is one",
			"widehat",
		},
		{
			// The remains of a commutative diagram flattened into prose. One
			// line of the volume is a bare capital and it is exactly this.
			"a line that is one capital letter",
			"the diagram is\n\nH\n\nand the rest",
			"nothing on it",
		},
		{
			"nothing wrong",
			`we have $\lambda \in \Lambda$ here`,
			"",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := run(t, m03, doc("a.md", c.body))
			if c.want == "" {
				if len(got) != 0 {
					t.Fatalf("a clean body was reported: %v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("got %d findings, want 1: %v", len(got), got)
			}
			if !strings.Contains(got[0].Msg, c.want) {
				t.Errorf("the finding does not mention %q: %s", c.want, got[0].Msg)
			}
		})
	}
}

// One finding per span and not one per character. A display with a dozen
// stranded letters in it is one thing to go and fix.
func TestM03ReportsASpanOnce(t *testing.T) {
	got := run(t, m03, doc("a.md", "we have $ΓΛΞ$ here"))
	if len(got) != 1 {
		t.Errorf("got %d findings for one span, want 1: %v", len(got), got)
	}
}

func TestM05(t *testing.T) {
	if got := run(t, m05, doc("a.md", "a clean paragraph")); len(got) != 0 {
		t.Errorf("a clean file was reported: %v", got)
	}
	// The marker the OCR path writes when it cannot read something. A corpus
	// that ships one of these is a corpus that is lying about what it holds.
	got := run(t, m05, doc("a.md", "the map is ⟪illegible⟫ here"))
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(got), got)
	}
}

// M07 is the rule a translation found rather than a reader. The shapes here
// print alike and say different things, and what tells them apart is whether the
// prose of the line is holding a bracket open where the span starts.
func TestM07(t *testing.T) {
	// The prose opened nothing on this line, so the bracket in the span is the
	// mathematics' own and there is nothing to report.
	if got := run(t, m07, doc("a.md", `the item $\alpha$) is the first`)); len(got) != 0 {
		t.Errorf("an innocent straddle was reported: %v", got)
	}
	if got := run(t, m07, doc("a.md", `the sum is Tr($u$).`)); len(got) != 0 {
		t.Errorf("a repaired page was reported: %v", got)
	}

	// The bracket belongs to a name Bourbaki sets upright, and the closing one
	// was swept into the formula. The mathematics of that span is "u)".
	got := run(t, m07, doc("a.md", "the trace\nis Tr($u)$."))
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(got), got)
	}
	if got[0].Line != 2 {
		t.Errorf("the finding is on line %d, want 2", got[0].Line)
	}
	if !strings.Contains(got[0].Msg, "u)") {
		t.Errorf("the finding does not say what the span holds: %s", got[0].Msg)
	}
}

// The rules and the tool that writes the pages have to agree about where the
// mathematics is, which is why the splitter is one function under both. This is
// the test that says the alias still points at it.
func TestMathIsTheSharedSplitter(t *testing.T) {
	spans, un := Math(`the price is \$5 and $x$ is not`)
	if un != nil {
		t.Fatalf("an escaped dollar opened a span")
	}
	if len(spans) != 1 || spans[0].Text != "x" {
		t.Errorf("got %v, want one span holding x", spans)
	}
}

// A finding has to point at a line of the file and not at a line of the body,
// because the reader opens the file.
func TestBodyLine(t *testing.T) {
	d := Doc{Path: "a.md", Body: "one\ntwo\nthree $x = y", head: 14}
	got, err := m01(&Corpus{Docs: []Doc{d}})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1", len(got))
	}
	if got[0].Line != 16 {
		t.Errorf("the finding is on line %d, want 16: body line 3 under 13 lines of front matter", got[0].Line)
	}
}

func TestM09(t *testing.T) {
	clean := doc("a.md", "the map $\\theta_E^{-1}$ and the sum $\\sum_{i=0}^{p-1}x_i$ and a prime $f'_1$")
	if got := run(t, m09, clean); len(got) != 0 {
		t.Errorf("mathematics that sets was reported: %v", got)
	}

	// The three shapes the linearised text layer leaves behind, one file each,
	// written as the corpus writes them.
	for _, tc := range []struct {
		name, body, want string
	}{
		{"an inverse", "we deduce $\\theta^-_E^1$ from it", `^-_E^1`},
		{"a matrix", "the matrix $(^X_0^0_I)$ is", `^X_0^0_I`},
		{"a sum's bound", "the sum $\\sum^p_{i=0}^{-1}x_i$", `^p_{i=0}^{-1}`},
		{"a prime", "the map $\\Gamma '_1^{\\pi'_1}$ is", `'_1^{\pi'_1}`},
	} {
		got := run(t, m09, doc("b.md", tc.body))
		if len(got) != 1 {
			t.Errorf("%s gave %d findings, want 1: %v", tc.name, len(got), got)
			continue
		}
		if !strings.HasSuffix(got[0].Msg, tc.want) {
			t.Errorf("%s was reported as %q, want it to name %q", tc.name, got[0].Msg, tc.want)
		}
	}

	// A line carrying two of them is two findings. The repairs are separate and
	// a reader who fixed one would take a single finding for done.
	two := doc("c.md", "so that $\\varphi ^-_V^1$ and $\\psi ^-_W^1$ agree")
	if got := run(t, m09, two); len(got) != 2 {
		t.Errorf("got %d findings for two faults on one line, want 2: %v", len(got), got)
	}
}

// The control space is a command and not a lost one. This is what M04 was
// measured against: run over the corpus with -validate-tex it reported 358 hard
// findings, every one of them a formula that is right, and 448 control spaces
// between them. P04 parses the same spans with KaTeX and takes all of them,
// which is what said which of the two rules was wrong.
func TestM04TakesTheControlSpace(t *testing.T) {
	for _, span := range []string{
		`R_1,\ R_2,\ \ldots,\ R_n`,
		`(T_1|a_1)(T_2|a_2) \ \ldots \ (T_h|a_h)A_i`,
		`(\forall x)(\forall y)\ \mathrm{Coll}_z(x)`,
	} {
		if why := parseTeX(span); why != "" {
			t.Errorf("%s was refused: %s", span, why)
		}
	}
}

// A newline or a tab after a backslash is still a lost command name. Nothing
// sets those on purpose, and a line break inside a formula is how an extraction
// loses the word after it.
func TestM04StillRefusesALostCommand(t *testing.T) {
	for _, span := range []string{"a \\\n b", "a \\\t b", `x + \`} {
		if parseTeX(span) == "" {
			t.Errorf("%q was taken", span)
		}
	}
}

func TestM04ReadsTheCorpusSpans(t *testing.T) {
	// One file with a control space, which is right, and one with a brace left
	// open, which is not.
	good := doc("good.md", "the relations $R_1,\\ R_2,\\ \\ldots,\\ R_n$ are equivalent")
	bad := doc("bad.md", "the field $\\mathbf{Q$ is prime")
	got := run(t, m04, good, bad)
	if len(got) != 1 || got[0].File != "bad.md" {
		t.Errorf("findings %v", got)
	}
}

func TestM10(t *testing.T) {
	// A quotient is a quotient. Nothing here divides by a relation sign, and the
	// last of these is the equivalence a set is quotiented by.
	clean := doc("a.md", `the group $\mathbf{Z}/n\mathbf{Z}$, the map $G \to G/H$, the space $X/\sim$`)
	if got := run(t, m10, clean); len(got) != 0 {
		t.Errorf("a quotient was reported as a struck sign: %v", got)
	}
	if got := run(t, m10, doc("a.md", `if $0\notin S$ then`)); len(got) != 0 {
		t.Errorf("a repaired page was reported: %v", got)
	}

	// Both sides, since the stroke falls on whichever the text layer met first.
	for _, tc := range []struct{ name, body, want string }{
		{"a stroke after the sign", "we have\n$0\\in /S$ here", `\in`},
		{"a stroke before the sign", "we have\n$\\lambda  /\\in$ Sp($u$)", `\in`},
		{"an inclusion", "we have\n$\\mathfrak{g}\\subset /\\mathfrak{h}$", `\subset`},
		{"a congruence", "we have\n$n\\equiv / p$ (mod. 3)", `\equiv`},
	} {
		got := run(t, m10, doc("b.md", tc.body))
		if len(got) != 1 {
			t.Errorf("%s gave %d findings, want 1: %v", tc.name, len(got), got)
			continue
		}
		if got[0].Line != 2 {
			t.Errorf("%s is on line %d, want 2", tc.name, got[0].Line)
		}
		if !strings.HasPrefix(got[0].Msg, tc.want+" has lost") {
			t.Errorf("%s does not name the sign: %s", tc.name, got[0].Msg)
		}
	}
}

func TestM11(t *testing.T) {
	// The corpus's own star, and the operator inside a span, which is a binary
	// law and a dual and belongs to M03.
	clean := doc("a.md", "\\* the passage runs from here to here. \\*\nthe law $x ∗ y$ and the dual $E^{∗}$")
	if got := run(t, m11, clean); len(got) != 0 {
		t.Errorf("the corpus's star or the operator was reported: %v", got)
	}

	for _, tc := range []struct{ name, body, want string }{
		{"an asterisk operator", "of the field of scalars.\nis richer than that. ∗", "an asterisk operator"},
		{"a teardrop spoked asterisk", "no least element).\nGive an example. ✻", "a teardrop spoked asterisk"},
		{"an eight spoked asterisk", "the mapping is\nnot injective. ✳", "an eight spoked asterisk"},
		{"a low asterisk", "the continuum is\nequipotent to it. ⁎", "a low asterisk"},
	} {
		got := run(t, m11, doc("b.md", tc.body))
		if len(got) != 1 {
			t.Errorf("%s gave %d findings, want 1: %v", tc.name, len(got), got)
			continue
		}
		if got[0].Line != 2 {
			t.Errorf("%s is on line %d, want 2", tc.name, got[0].Line)
		}
		if !strings.HasPrefix(got[0].Msg, tc.want+" where the corpus writes") {
			t.Errorf("%s does not name the glyph: %s", tc.name, got[0].Msg)
		}
	}

	// One finding to a line, so a pair set on one line is reported once and the
	// line is what somebody goes and looks at.
	pair := doc("c.md", "no least element). ✻ Give an example. ✻")
	if got := run(t, m11, pair); len(got) != 1 {
		t.Errorf("a pair on one line gave %d findings, want 1: %v", len(got), got)
	}

	// The bare asterisk, which is the spelling a reader can see, since one at the
	// head of a line opens a bullet list.
	bareLine := doc("d.md", "the ordering is total.\n* (3) The set $\\mathbf{R}$ is totally ordered. *")
	got := run(t, m11, bareLine)
	if len(got) != 1 {
		t.Fatalf("a bare pair gave %d findings, want 1: %v", len(got), got)
	}
	if got[0].Line != 2 {
		t.Errorf("the bare pair is on line %d, want 2", got[0].Line)
	}
	if !strings.HasPrefix(got[0].Msg, "a bare asterisk where the corpus writes") {
		t.Errorf("the bare pair is reported as %q", got[0].Msg)
	}

	// Emphasis, bold and the units of a ring are asterisks that are not the mark,
	// and all three are everywhere in the Elements.
	notTheMark := doc("e.md", "The *signs* of a theory.\n**Definition 1.** — the group $K^*$ of units")
	if got := run(t, m11, notTheMark); len(got) != 0 {
		t.Errorf("emphasis, bold or the units were reported: %v", got)
	}

	// A solution is the one place the corpus writes real bullet lists, and a
	// bullet has the shape of a star.
	bullets := Doc{Path: "content/solutions/en/ens/I/a0/01.md", Lang: "en", Kind: KindSolution, head: 1,
		Body: "Write $B$ as three pieces:\n* $F$ = the first segment\n* $D$ = the overlap"}
	if got := run(t, m11, bullets); len(got) != 0 {
		t.Errorf("a solution's bullet list was reported as the mark: %v", got)
	}
}
