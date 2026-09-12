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
			// A Unicode subscript where the reading should have written a TeX
			// one. KaTeX takes it without complaint -- an unknown character is
			// an ordinary atom -- and then cmmi10 has no glyph for it and
			// nothing reaches the page. Integration V set the Lebesgue-Fubini
			// theorem this way (tamnd/bourbaki#414).
			"a subscript digit inside the mathematics",
			`the integral $\int f(t₁, t₂)$ is one`,
			"a Unicode script",
		},
		{
			"a superscript inside the mathematics",
			`the group $SO⁺(q)$ acts`,
			"a Unicode script",
		},
		{
			// The subscript letters Unicode left out of the block.
			"a subscript letter from Phonetic Extensions",
			`the tangent map $T₀kᵢ$ is one`,
			"a Unicode script",
		},
		{
			// In the prose the same characters are usually right: 1039 of the
			// corpus's superscript ones are footnote markers, and E₆ is the name
			// of a root system in a running head.
			"a footnote marker in the prose",
			"the order is 12.² The rest follows",
			"",
		},
		{
			"a root system named in the prose",
			"the diagram of E₆ is one",
			"",
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

// A capital alone on a line is a flattened diagram in the prose and ordinary
// TeX inside a display, and the rule has to tell them apart. The exercise of
// Algebre II, § 10 sets a product of three matrices over eleven lines and the Q
// of P D Q is one of them.
func TestM03ReadsACapitalInsideADisplayAsMathematics(t *testing.T) {
	body := "We have\n\n$$\nM = P\n\\begin{pmatrix}\na & 0 \\\\\n0 & b\n\\end{pmatrix}\nQ\n$$\n\nand so on."
	if got := run(t, m03, doc("a.md", body)); len(got) != 0 {
		t.Errorf("a capital on its own line inside a display was reported: %v", got)
	}
	// The same capital in the prose is the fault the check exists for.
	got := run(t, m03, doc("a.md", "the diagram is\n\nH\n\nand it commutes"))
	if len(got) != 1 {
		t.Fatalf("got %d findings for a capital in the prose, want 1: %v", len(got), got)
	}
}

// The spacing modifier block opens with letters and not with accents, and a
// French bibliography sets an ordinal with them.
func TestM03LeavesAModifierLetterAlone(t *testing.T) {
	got := run(t, m03, doc("a.md", "A. Cauchy, Cours d’Analyse, 1ʳᵉ partie, 1821."))
	if len(got) != 0 {
		t.Errorf("an ordinal set with modifier letters was reported: %v", got)
	}
	// The accent proper is still a lost \widehat.
	if got := run(t, m03, doc("a.md", "the completion Gˆ of G")); len(got) != 1 {
		t.Fatalf("got %d findings for a stranded circumflex, want 1: %v", len(got), got)
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

func TestM12(t *testing.T) {
	// The corpus's own delimiters, which are the whole of what the rest of this
	// group can read.
	clean := doc("a.md", "for every $x$ in $E$ we have\n$$\nf(x) = 0\n$$\nand nothing else")
	if got := run(t, m12, clean); len(got) != 0 {
		t.Errorf("a file written the corpus's way was reported: %v", got)
	}

	for _, tc := range []struct{ name, body, want string }{
		{"a display opened", "we have\n\\[ f(x) = 0 $$", "a display opened with a bracket"},
		{"a display closed", "we have\n$$ f(x) = 0 \\]", "a display closed with a bracket"},
		{"a span opened", "for every\n\\(x\\) in $E$", "a span opened with a parenthesis"},
		{"a span closed", "for every\n$x\\) in $E$", "a span closed with a parenthesis"},
	} {
		got := run(t, m12, doc("b.md", tc.body))
		if len(got) != 1 {
			t.Errorf("%s gave %d findings, want 1: %v", tc.name, len(got), got)
			continue
		}
		if got[0].Line != 2 {
			t.Errorf("%s is on line %d, want 2", tc.name, got[0].Line)
		}
		if !strings.HasPrefix(got[0].Msg, tc.want+" where the corpus writes") {
			t.Errorf("%s does not name the delimiter: %s", tc.name, got[0].Msg)
		}
	}

	// One finding to a line, so a display opened and closed on one line sends
	// somebody to that line once.
	pair := doc("c.md", `\[ f(x) = 0 \]`)
	if got := run(t, m12, pair); len(got) != 1 {
		t.Errorf("a display on one line gave %d findings, want 1: %v", len(got), got)
	}

	// The row break of a matrix is the one thing in the corpus with this shape
	// that is not a display, and both spellings here are ones the corpus uses.
	rows := doc("d.md", `$$\begin{pmatrix} a & b \\[2pt] c & d \end{pmatrix}$$`+"\n"+
		`$$\begin{pmatrix} a & b \\[-1.5em] c & d \end{pmatrix}$$`)
	if got := run(t, m12, rows); len(got) != 0 {
		t.Errorf("a row break was reported as a display: %v", got)
	}

	// The solutions are read, unlike M11, and they are the reason the rule is
	// here: this is what exercise 1 of § 5 of chapter II of Theory of Sets
	// shipped with, verified by both judges.
	sol := Doc{Path: "content/solutions/en/ens/II/s5/01.md", Lang: "en", Kind: KindSolution, head: 1,
		Body: "By induction on $n$:\n" + `\[ \operatorname{Card}(A) = n \]`}
	got := run(t, m12, sol)
	if len(got) != 1 {
		t.Fatalf("a solution gave %d findings, want 1: %v", len(got), got)
	}
	if got[0].Line != 2 {
		t.Errorf("the display is on line %d, want 2", got[0].Line)
	}
}

func TestM13(t *testing.T) {
	// A file written the corpus's way: the inline spans tight against their
	// dollars and the display on lines of its own.
	clean := doc("a.md", "for every $x$ in $E$ we have\n$$\nf(x) = 0\n$$\nand nothing else")
	if got := run(t, m13, clean); len(got) != 0 {
		t.Errorf("a file written the corpus's way was reported: %v", got)
	}

	loose := doc("b.md", "the ring $ K[[T]] $ of formal power series\nover $ k $")
	got := run(t, m13, loose)
	if len(got) != 2 {
		t.Fatalf("gave %d findings, want 2: %v", len(got), got)
	}
	if got[0].Line != 1 || got[1].Line != 2 {
		t.Errorf("the findings are on lines %d and %d, want 1 and 2", got[0].Line, got[1].Line)
	}
	if !strings.Contains(got[0].Msg, "$ K[[T]] $") {
		t.Errorf("the finding does not show the span: %s", got[0].Msg)
	}
	if !strings.Contains(got[0].Msg, "bourbaki fix padding") {
		t.Errorf("the finding does not name the repair: %s", got[0].Msg)
	}

	// A display keeps the whitespace that puts it on its own lines, so neither
	// spelling of one is a finding here.
	display := doc("c.md", "$$\nf(x) = 0\n$$\n"+`$$ f(x) = 0 $$`)
	if got := run(t, m13, display); len(got) != 0 {
		t.Errorf("a display was reported: %v", got)
	}

	// A blank span is not this rule's to close up. See tighten in textguard.
	if got := run(t, m13, doc("d.md", "the ring $ $ of series")); len(got) != 0 {
		t.Errorf("a blank span was reported: %v", got)
	}

	// A body with a span left open belongs to M01, and reading it by these
	// offsets would report spans nobody wrote.
	if got := run(t, m13, doc("e.md", "the ring $ K[[T]] of series")); len(got) != 0 {
		t.Errorf("a body with a span left open was reported: %v", got)
	}

	// The solutions are read, the way M12 reads them, because the solver is the
	// third thing that writes Markdown into the corpus.
	sol := Doc{Path: "content/solutions/en/ens/II/s5/01.md", Lang: "en", Kind: KindSolution, head: 1,
		Body: "By induction on $ n $:\n$$\n\\operatorname{Card}(A) = n\n$$"}
	if got := run(t, m13, sol); len(got) != 1 {
		t.Errorf("a solution gave %d findings, want 1: %v", len(got), got)
	}
}

func TestM14(t *testing.T) {
	// Mathematics that is inside a span is not a finding, however much of it
	// there is and whichever delimiters it uses.
	clean := doc("a.md", "for every $x \\in E$ we have\n$$\nf(x) \\otimes g \\leq 0\n$$\nand nothing else")
	if got := run(t, m14, clean); len(got) != 0 {
		t.Errorf("a file written the corpus's way was reported: %v", got)
	}

	// The shape from #377: a line of a table of contents where the reading
	// wrote the TeX and never opened a span.
	contents := doc("b.md", "1. A first no.\n6. Properties of E \\otimes_A F relative to exact sequences ... 251\n")
	got := run(t, m14, contents)
	if len(got) != 1 {
		t.Fatalf("gave %d findings, want 1: %v", len(got), got)
	}
	if got[0].Line != 2 {
		t.Errorf("the finding is on line %d, want 2", got[0].Line)
	}
	if !strings.Contains(got[0].Msg, "\\otimes") {
		t.Errorf("the finding does not name the control sequence: %s", got[0].Msg)
	}
}

// One finding a line and not one a control sequence. A contents line has nine
// of them and they are one mistake.
func TestM14ReportsALineOnce(t *testing.T) {
	d := doc("c.md", "Hom_B(E \\otimes_A F, G) \\to Hom_A(F, Hom_B(E, G))")
	if got := run(t, m14, d); len(got) != 1 {
		t.Errorf("gave %d findings, want 1: %v", len(got), got)
	}
}

// The markup macros are the prose's own and are left alone. Whether the corpus
// should write LaTeX markup in Markdown is a different question from this one.
func TestM14LeavesTheMarkupMacrosAlone(t *testing.T) {
	d := doc("d.md", "the theorem of Cauchy\\footnote{See the historical note.}\n*Bourbaki*, 2\\textsuperscript{e} \\'edition")
	got := run(t, m14, d)
	for _, f := range got {
		if strings.Contains(f.Msg, "\\footnote") || strings.Contains(f.Msg, "\\textsuperscript") {
			t.Errorf("a markup macro was reported: %s", f.Msg)
		}
	}
}

// An unclosed span makes everything after it look like prose, and M01 already
// reports the one fault that caused it. Reporting the rest here would bury M01
// under its own consequences.
func TestM14IsSilentAfterAnUnclosedSpan(t *testing.T) {
	d := doc("e.md", "we have $x \\in E and then\nf \\otimes g and then\nh \\leq k")
	if got := run(t, m14, d); len(got) != 0 {
		t.Errorf("the tail of a file with an unclosed span was reported: %v", got)
	}
}

func TestM15(t *testing.T) {
	// A volume that has settled on one spelling, whichever it is.
	clean := []Doc{
		doc("content/en/ens/01.md", `the topology $\mathscr{T}$ on $\mathscr{S}$`),
		doc("content/en/ens/02.md", `and $\mathscr{T}$ again`),
	}
	if got := run(t, m15, clean...); len(got) != 0 {
		t.Errorf("a volume with one spelling was reported: %v", got)
	}

	// The shape from tamnd/bourbaki#386: one letter, two commands, one volume.
	// The minority is what is reported, one finding a file.
	split := []Doc{
		doc("content/fr/ens/01.md", `la topologie $\mathcal{T}$`),
		doc("content/fr/ens/02.md", `la topologie $\mathcal{T}$ encore`+"\n"+`et $\mathcal{T}$ une fois de plus`),
		doc("content/fr/ens/03.md", `la topologie $\mathscr{T}$ ici`),
	}
	got := run(t, m15, split...)
	if len(got) != 1 {
		t.Fatalf("gave %d findings, want 1: %v", len(got), got)
	}
	if got[0].File != "content/fr/ens/03.md" || got[0].Line != 1 {
		t.Errorf("the finding is at %s, want content/fr/ens/03.md:1", got[0].At())
	}
	for _, want := range []string{`\mathscr{T}`, `\mathcal{T}`, "content/fr/ens"} {
		if !strings.Contains(got[0].Msg, want) {
			t.Errorf("the finding does not name %s: %s", want, got[0].Msg)
		}
	}

	// Two letters, one spelled each way, is a distinction this rule cannot see
	// and must not guess at. Only the same letter both ways is one symbol
	// printed two ways.
	letters := []Doc{
		doc("content/en/top/01.md", `the sheaf $\mathcal{F}$ and the filter $\mathscr{G}$`),
	}
	if got := run(t, m15, letters...); len(got) != 0 {
		t.Errorf("two different letters were reported: %v", got)
	}

	// Two volumes that each have one spelling are two volumes and not one
	// split. Disagreement between volumes is tamnd/bourbaki#392 and is not
	// something a reader holding either of them can see.
	volumes := []Doc{
		doc("content/en/alg/01.md", `the algebra $\mathscr{A}$`),
		doc("content/en/top/01.md", `the algebra $\mathcal{A}$`),
	}
	if got := run(t, m15, volumes...); len(got) != 0 {
		t.Errorf("two volumes with one spelling each were reported: %v", got)
	}

	// The brace is optional in TeX and the corpus writes it both ways.
	braceless := []Doc{
		doc("content/en/int/01.md", `the measure $\mathcal M$`),
		doc("content/en/int/02.md", `the measure $\mathscr{M}$`),
		doc("content/en/int/03.md", `the measure $\mathscr{M}$`),
	}
	if got := run(t, m15, braceless...); len(got) != 1 {
		t.Fatalf("the braceless spelling was not matched: %v", got)
	}
}

func TestM16(t *testing.T) {
	// The shape from tamnd/bourbaki#395: one definition, one ring, two letters.
	mixed := []Doc{
		doc("content/fr/ac/X/09.md",
			`Soit $A$ un anneau noethérien. On dit qu'un $\Lambda$-module $\Omega$ est`+"\n"+
				`dualisant si, pour tout idéal maximal $m$ de $\Lambda$, …`),
	}
	got := run(t, m16, mixed...)
	if len(got) != 1 {
		t.Fatalf("gave %d findings, want 1: %v", len(got), got)
	}
	if got[0].Line != 1 {
		t.Errorf("the finding is at line %d, want 1: %v", got[0].Line, got[0])
	}
	if !strings.Contains(got[0].Msg, `\Lambda`) {
		t.Errorf("the finding does not name the letter: %s", got[0].Msg)
	}

	// The subscript of an operator that takes a ring is a ring, on both sides:
	// prof_A(p; M) = dim_{Λ_p}(M_p) is the corollary that gave the issue away.
	subscripts := []Doc{
		doc("content/fr/ac/X/01.md", `on a $\mathrm{prof}_A(p; M) = \dim_{\Lambda_p}(M_p)$`),
	}
	if got := run(t, m16, subscripts...); len(got) != 1 {
		t.Errorf("the two letters inside one equation were not reported: %v", got)
	}

	// Λ is a good letter. A file that uses it and never names a ring A is a
	// file about an exterior algebra or an index set and is not this fault.
	lambda := []Doc{
		doc("content/en/alg/III/01.md", `the exterior algebra $\Lambda(L)$ and $\Lambda^n L$`),
		doc("content/fr/ens/III/01.md", `une famille $(x_\lambda)_{\lambda \in \Lambda}$`),
	}
	if got := run(t, m16, lambda...); len(got) != 0 {
		t.Errorf("an honest lambda was reported: %v", got)
	}

	// Two rings that really are two, as Lie VII § 1, Exercise 8 has them: Λ is
	// a subring of U_(K) and A is the base ring. On this much of it nothing
	// names A as a ring, so nothing says the two are the same one. The file it
	// is taken from does name A a ring further down and the rule reports it,
	// which is the false positive m16's comment admits to.
	twoRings := []Doc{
		doc("content/fr/lie/VII/exercises/s1/08.md",
			`Soit $d$ un élément non nul de $A$, et soit $\Lambda$ un sous-anneau de `+
				`$U_{(K)}$ tel que $U \subset \Lambda \subset d^{-1}U$.`),
	}
	if got := run(t, m16, twoRings...); len(got) != 0 {
		t.Errorf("two rings that really are two were reported: %v", got)
	}

	// A ring named A without its dollars still names it. The Springer English
	// writes "a finitely generated A-module" in the prose.
	bare := []Doc{
		doc("content/en/alg/VII/exercises/s4/09.md",
			`let M be a finitely generated A-module of rank $n$, and let P be a `+
				`finitely generated $\Lambda$-module of rank q.`),
	}
	if got := run(t, m16, bare...); len(got) != 1 {
		t.Errorf("the bare A-module was not read as a ring: %v", got)
	}

	// A letter that happens to end a word is not the ring. Without the guard
	// on the left, "Galois-module" or "gamma-module" would name one.
	word := []Doc{
		doc("content/en/alg/IV/01.md",
			`the $\Lambda$-module of a Gamma-module`),
	}
	if got := run(t, m16, word...); len(got) != 0 {
		t.Errorf("a word ending in A was read as the ring: %v", got)
	}
}

func TestM17(t *testing.T) {
	cases := []struct {
		name string
		body string
		want string
	}{
		{
			// The fault the rule was written for. The English reads
			// "Let $x \in X$ and $A$ be ...", and the dollar was put one
			// word too early (tamnd/bourbaki#412).
			"a Vietnamese word the dollar took with it",
			`$Gọi x \in X$ và $A$ là một tập hợp`,
			"ọ",
		},
		{
			// A word inside the mathematics on purpose. \text is set in the
			// text face and its letters reach the page.
			"a word inside \\text",
			`we have $\Phi_p(X) \quad \text{với} \quad r \geq 0$ here`,
			"",
		},
		{
			"a word inside \\operatorname",
			`the order $\operatorname{ord}_{\mathfrak{p}}(x)$ is one`,
			"",
		},
		{
			"a French accent inside a display",
			"$$\nf(x) = y \\text{ où } x \\in E, \\text{ donné}\n$$",
			"",
		},
		{
			"a French word the display swallowed",
			"$$\nf(x) = y \\quad où \\quad x \\in E\n$$",
			"ù",
		},
		{
			// The two letters of the Latin-1 block that are operators and
			// not letters at all.
			"a times sign",
			`the product $E × F$ is one`,
			"",
		},
		{
			// M03's, and M03 says it better.
			"the dotless i",
			`the index $\alpha_ı$ runs`,
			"",
		},
		{
			"nothing wrong",
			`we have $\lambda \in \Lambda$ here`,
			"",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := run(t, m17, doc("a.md", c.body))
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
				t.Errorf("the finding does not name %q: %s", c.want, got[0].Msg)
			}
		})
	}
}

// One finding per span. A clause of eight Vietnamese words inside a pair of
// dollars is one dollar in the wrong place and one thing to go and fix.
func TestM17ReportsASpanOnce(t *testing.T) {
	got := run(t, m17, doc("a.md", `$Gọi một tập hợp x$ here`))
	if len(got) != 1 {
		t.Errorf("got %d findings for one span, want 1: %v", len(got), got)
	}
}

// The stripper has to balance braces, or a \text{} with a group inside it
// hides the mathematics that follows and the rule goes blind.
func TestM17StripsTextArgumentsAndNoMore(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{`\text{où l'on a} x + y`, ` x + y`},
		{`\text{a \frac{1}{2} b} où`, ` où`},
		{`\begin{pmatrix} a & b \end{pmatrix}`, ` a & b `},
		{`\operatorname*{lim} \alpha`, ` \alpha`},
		{`\href{http://é}{lé} x`, ` x`},
		{`\lambda \in \Lambda`, `\lambda \in \Lambda`},
		{`\$ \{ a \}`, `\$ \{ a \}`},
	} {
		if got := mathProper(c.in); got != c.want {
			t.Errorf("mathProper(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestM18(t *testing.T) {
	// A II § 11 decides it inside one sentence: the same group is written both
	// ways eight words apart (tamnd/bourbaki#413).
	one := doc("content/fr/alg/II/11_s11_gradues.md",
		`on peut supposer de la forme $\mathbf{Q}^{(I)}$; l'ensemble $\mathbf{Q}^{(1)}$ est totalement ordonné`)
	got := run(t, m18, one)
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(got), got)
	}
	if !strings.Contains(got[0].Msg, "the digit 1") {
		t.Errorf("the finding does not say what was read: %s", got[0].Msg)
	}

	// The lowercase l is the other reading of the same glyph.
	l := doc("content/en/alg/IV/01_s1_polynomials.md",
		`for $\nu \in \mathbf{N}^{(I)}$ and $\mu \in \mathbf{N}^{(l)}$`)
	if got := run(t, m18, l); len(got) != 1 || !strings.Contains(got[0].Msg, "lowercase l") {
		t.Errorf("the lowercase l came out as %v", got)
	}
}

// The gate is per base. ^{(1)} is a good exponent and most of its uses are
// real: AC X sets two complexes C^{(1)} and C^{(2)} whose index set is {1, 2},
// and nothing anywhere writes C^{(I)}.
func TestM18LeavesAnUngatedBaseAlone(t *testing.T) {
	for _, body := range []string{
		`$C_n = \sum (C^{(1)})_{p_1} \otimes (C^{(2)})_{p_2}$`,
		`the derived series $l = l^{(0)} \supset l^{(1)} \supset b$`,
		`the Hilbert series $H_{M,F}^{(1)}$ of the filtration`,
		// The right form on a different base does not gate this one.
		`$\mathbf{N}^{(I)}$ and the commutator subgroup $G^{(1)}$`,
	} {
		if got := run(t, m18, doc("content/en/ac/X/01_s1_a.md", body)); len(got) != 0 {
			t.Errorf("an ungated base was reported for %q: %v", body, got)
		}
	}
}

// The twin section is the gate that reaches the sections which never write the
// right form at all. The Remark of A VII § 3 says "every submodule of $A^{(1)}$
// is isomorphic to a direct sum" and never writes A^{(I)}; the French facing
// page has $A^{(I)}$ with the set under the sum as well.
func TestM18ReadsTheOtherLanguage(t *testing.T) {
	en := doc("content/en/alg/VII/03_s3_free_modules.md",
		`every submodule of $A^{(1)}$ is isomorphic to a direct sum $\bigoplus a_i$`)
	if got := run(t, m18, en); len(got) != 0 {
		t.Fatalf("the English alone should not decide it: %v", got)
	}
	fr := doc("content/fr/alg/VII/03_s3_modules_libres.md",
		`tout sous-module de $A^{(I)}$ est isomorphe à une somme directe $\bigoplus_{i \in I} a_i$`)
	got := run(t, m18, en, fr)
	if len(got) != 1 {
		t.Fatalf("got %d findings with the twin, want 1: %v", len(got), got)
	}
	if got[0].File != en.Path {
		t.Errorf("the finding is against %s, want the English", got[0].File)
	}
}

func TestTwinKey(t *testing.T) {
	for _, c := range []struct{ path, want string }{
		{"content/en/alg/IV/01_s1_polynomials.md", "alg/IV/01"},
		{"content/fr/alg/IV/01_s1_polynomes.md", "alg/IV/01"},
		{"content/vi/alg/IV/01_s1_polynomials.md", "alg/IV/01"},
		{"content/en/top/III/exercises/s6/10.md", "top/III/exercises/s6/10"},
		{"pages/alg-iv-vii/0015.md", ""},
	} {
		if got := twinKey(c.path); got != c.want {
			t.Errorf("twinKey(%q) = %q, want %q", c.path, got, c.want)
		}
	}
}
