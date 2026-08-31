package book

import (
	"strings"
	"testing"
)

// These are the two faults that stopped seventeen of the hundred and seven
// volume builds from producing a PDF at all. Both are the writer's own doing
// rather than the corpus's, both were invisible until a whole book was set,
// and both report themselves as a TeX error a long way from the cause.

func TestFencesCountsWhatALineLeavesOpen(t *testing.T) {
	cases := []struct {
		in           string
		fence, brace int
	}{
		{`a + b`, 0, 0},
		{`\left(`, 1, 0},
		{`\right)`, -1, 0},
		{`\left( x \right)`, 0, 0},
		// \leftarrow and \rightharpoonup start with the same letters as the two
		// words being counted and open nothing. Counting the text rather than
		// the word gets both of these wrong in opposite directions.
		{`x \leftarrow y`, 0, 0},
		{`x \rightarrow y`, 0, 0},
		{`x \rightharpoonup y`, 0, 0},
		{`\frac{1}{2}`, 0, 0},
		{`\frac{1}{`, 0, 1},
		// An escaped brace is a character in the formula rather than a group.
		{`\{ x \}`, 0, 0},
		{`\left\{ x`, 1, 0},
	}
	for _, c := range cases {
		f, b := fences(c.in)
		if f != c.fence || b != c.brace {
			t.Errorf("fences(%q) = %d, %d, want %d, %d", c.in, f, b, c.fence, c.brace)
		}
	}
}

// This is Set Theory III, § 5, exercise 5 as the corpus has it: one display
// written over nine lines, of which five are inside a single \left( \right).
// Broken a line to a row, the \left lands in one cell of the aligned and the
// \right in the next, and the book stops with "Extra }, or forgotten \right".
func TestDisplayRowsKeepsAFenceInOneRow(t *testing.T) {
	body := strings.Join([]string{
		`f(\sup (I))+`,
		`\sum_{2n\leq \operatorname{Card}(I)}`,
		`\left(`,
		`\sum_{\substack{H\subset I,\ \operatorname{Card}(H)=2n}}`,
		`f(\inf (H))`,
		`\right)`,
		`=`,
	}, "\n")
	rows := displayRows(body)
	// The head of the sum, the sum sign, the whole bracket, and the relation:
	// the four lines the printing has, out of the seven the corpus wrote.
	if len(rows) != 4 {
		t.Fatalf("displayRows gave %d rows, want 4:\n%q", len(rows), rows)
	}
	if !strings.Contains(rows[2], `\left(`) || !strings.Contains(rows[2], `\right)`) {
		t.Errorf("the fence was split across rows: %q", rows)
	}
	for i, r := range rows {
		if f, b := fences(r); f != 0 || b != 0 {
			t.Errorf("row %d is not closed: fence %d brace %d in %q", i, f, b, r)
		}
	}
}

func TestDisplayRowsStillBreaksAnOrdinaryCalculation(t *testing.T) {
	rows := displayRows("a = b + c\n= d + e\n= f")
	if len(rows) != 3 {
		t.Errorf("a calculation of three steps came out as %d rows: %q", len(rows), rows)
	}
}

// Every row an alignment goes out with has to be closed, or the book does not
// typeset. This holds it over the shapes the corpus actually writes.
func TestAlignedDisplayCloseseveryRowItWrites(t *testing.T) {
	body := "x = \\left(\n\\frac{1}{\n2}\n\\right)\n= y"
	got, ok := alignedDisplay(body)
	if !ok {
		t.Fatal("alignedDisplay refused a calculation of two steps")
	}
	for _, row := range strings.Split(got, `\\`) {
		row = strings.TrimSpace(row)
		row = strings.TrimPrefix(row, `\begin{aligned}`)
		row = strings.TrimSuffix(row, `\end{aligned}`)
		if f, b := fences(row); f != 0 || b != 0 {
			t.Errorf("a row of the alignment is not closed: fence %d brace %d in %q", f, b, row)
		}
	}
}

func TestLiftTagTakesTheNumberOut(t *testing.T) {
	tag, rest, ok := liftTag(`x = y \tag{10}`)
	if !ok || tag != "10" || rest != "x = y" {
		t.Errorf("liftTag gave %q, %q, %v", tag, rest, ok)
	}
	if _, _, ok := liftTag("x = y"); ok {
		t.Error("liftTag found a tag in a display that has none")
	}
}

// A tag is not always a number. Reading its argument to the first closing brace
// cuts this one in half and leaves the second half of the formula in the body
// as text, which is a wrong book rather than a book that will not build.
func TestLiftTagReadsAnArgumentWithBracesInIt(t *testing.T) {
	tag, rest, ok := liftTag(`x = y \tag{$A \in \mathcal{S}(X)$}`)
	if !ok {
		t.Fatal("liftTag refused a tag whose argument has a group in it")
	}
	if tag != `$A \in \mathcal{S}(X)$` {
		t.Errorf("liftTag gave the tag as %q", tag)
	}
	if rest != "x = y" {
		t.Errorf("liftTag left %q in the body", rest)
	}
}

// A calculation that justifies each of its steps carries a \tag on every row,
// and there is nowhere beside the display for the second and third of them to
// go. Left where they are they end up inside the \begin{aligned} the display is
// wrapped in, amsmath stops with "\tag not allowed here", and the volume does
// not typeset at all. Both builds of Lie I to III stopped on the display in III,
// § 3 that justifies three of its four steps this way.
func TestSeveralTagsAreSetBesideTheirOwnRows(t *testing.T) {
	body := "a = b \\tag{prop. 38} \\\\\n= c \\\\\n= d \\tag{VAR, R, 5.5.6} \\\\\n= e \\tag{prop. 38}"
	if n := countTags(body); n != 3 {
		t.Fatalf("countTags gave %d for a display with three tags", n)
	}
	got, n := inlineTags(body)
	if n != 3 {
		t.Errorf("inlineTags moved %d tags, want 3", n)
	}
	if strings.Contains(got, `\tag`) {
		t.Errorf("inlineTags left a tag in the body: %q", got)
	}
	for _, want := range []string{`\qquad(\text{prop. 38})`, `\qquad(\text{VAR, R, 5.5.6})`} {
		if !strings.Contains(got, want) {
			t.Errorf("inlineTags did not set %q, gave %q", want, got)
		}
	}
	if countTags("a = b") != 0 {
		t.Error("countTags found a tag in a display that has none")
	}
}

// The degree sign reached the writer through the mathematics and came out as a
// superscript circle, which is right in a formula and an error in the argument
// of a \tag, where amsmath sets text.
func TestTagTextSetsTheDegreeSignAsText(t *testing.T) {
	got := tagText(`cf. chap. I, \S 2, n^\circ 6`)
	if strings.Contains(got, `^\circ`) {
		t.Errorf("tagText left a superscript in text mode: %q", got)
	}
	if !strings.Contains(got, `\textdegree`) {
		t.Errorf("tagText lost the degree sign: %q", got)
	}
}

// The degree sign is a raised circle and the corpus writes it in both of the
// places a raised circle can go. Written as a superscript every time, the two
// polars of A come out as A^{^\circ^\circ}, and TeX stops on a double
// superscript. It stopped both volumes of Topological Vector Spaces.
func TestDegreeSignOpensASuperscriptOnlyWhenThereIsNotOne(t *testing.T) {
	cases := []struct{ in, want string }{
		{"A°", `A^\circ`},
		{"A^{°°}", `A^{\circ\circ}`},
		{"(A° \\cup U°)^°", `(A^\circ \cup U^\circ)^\circ`},
	}
	for _, c := range cases {
		got := Math(c.in)
		if got != c.want {
			t.Errorf("Math(%q) = %q, want %q", c.in, got, c.want)
		}
		if strings.Contains(got, "^^") {
			t.Errorf("Math(%q) wrote a double superscript", c.in)
		}
	}
}

func TestLiftNumberTakesTheNumberOffTheFirstLine(t *testing.T) {
	num, rest, ok := liftNumber("(3)\n" + `\begin{align*}` + "\nx &= y\n" + `\end{align*}`)
	if !ok || num != "3" {
		t.Fatalf("liftNumber gave %q, %v", num, ok)
	}
	if !strings.HasPrefix(rest, `\begin{align*}`) {
		t.Errorf("liftNumber left %q in the body", rest)
	}
	if _, _, ok := liftNumber("x = y\n= z"); ok {
		t.Error("liftNumber found a number in a display that has none")
	}
}

// An align inside \[ \] is what amsmath reports as "Erroneous nesting of
// equation structures", and it is what a numbered display that opens an
// environment of its own would come out as. There is one of these in the corpus
// and it stopped both builds of Lie IV to VI.
func TestInlineEnvironmentTakesTheDisplayOutOfTheEnvironment(t *testing.T) {
	cases := []struct{ in, want string }{
		{`\begin{align*}` + "\nx &= y\n" + `\end{align*}`,
			`\begin{aligned}` + "\nx &= y\n" + `\end{aligned}`},
		{`\begin{gather}` + "\nx\n" + `\end{gather}`,
			`\begin{gathered}` + "\nx\n" + `\end{gathered}`},
		{`\begin{alignat*}{2}` + "\nx &= y\n" + `\end{alignat*}`,
			`\begin{alignedat}{2}` + "\nx &= y\n" + `\end{alignedat}`},
	}
	for _, c := range cases {
		got, ok := inlineEnvironment(c.in)
		if !ok || got != c.want {
			t.Errorf("inlineEnvironment(%q) = %q, %v, want %q", c.in, got, ok, c.want)
		}
	}
	// An environment that opens and does not close, and a body that is not an
	// environment at all, are both shapes with no reading, and guessing at one
	// would move rows of a formula around on no evidence.
	if _, ok := inlineEnvironment(`\begin{align*}` + "\nx &= y"); ok {
		t.Error("inlineEnvironment rewrote an environment that never closes")
	}
	if _, ok := inlineEnvironment("x = y"); ok {
		t.Error("inlineEnvironment rewrote a display with no environment in it")
	}
}

// The whole of the fault, end to end: the shape the corpus has in Lie VI, § 3
// goes out as one display with its number beside it and no environment nested
// inside another.
func TestNumberedEnvironmentDisplayGoesOutAsOneDisplay(t *testing.T) {
	body := "$$\n(3)\n" + `\begin{align*}` + "\nd &= x \\\\\n&= y\n" + `\end{align*}` + "\n$$\n"
	out, err := Renderer{File: "x.md", Line: 1}.TeX(body)
	if err != nil {
		t.Fatalf("TeX: %v", err)
	}
	if strings.Contains(out, `\begin{align*}`) {
		t.Errorf("an align* went out inside a display:\n%s", out)
	}
	if !strings.Contains(out, `\tag{3}`) {
		t.Errorf("the number was not set beside the display:\n%s", out)
	}
	if !strings.Contains(out, `\begin{aligned}`) {
		t.Errorf("the rows of the alignment were lost:\n%s", out)
	}
}

// A number written under its display is the display's number. In the margin of
// a scan it sits beside the formula, so which side of it the number lands on in
// the text says nothing about what it numbers, and 366 of them in the corpus
// landed underneath. Page 18 of Commutative Algebra I is one: the diagram, then
// a line reading "(3)", then the next paragraph.
func TestANumberUnderItsDisplayIsStillItsNumber(t *testing.T) {
	body := "$$\nx = y\n$$\n\n(3)\n\nIt follows immediately that this is so.\n"
	out, err := Renderer{File: "x.md", Line: 1}.TeX(body)
	if err != nil {
		t.Fatalf("TeX: %v", err)
	}
	if !strings.Contains(out, `\tag{3}`) {
		t.Errorf("the number under the display was not taken:\n%s", out)
	}
	if strings.Contains(out, "\n(3)\n") {
		t.Errorf("the number is still set as a paragraph of its own:\n%s", out)
	}
}

// A number on the same line as its display, which is how 590 of them are
// written. soleDisplay used to refuse the line because the number counted as
// prose on it, and the number then set as a paragraph above an unnumbered
// formula, the same fault by a different route.
func TestANumberOnTheSameLineAsItsDisplayIsTaken(t *testing.T) {
	body := "Consider the sequence\n\n(4) $$ x \\to y $$\n\nThis being so, E is flat.\n"
	out, err := Renderer{File: "x.md", Line: 1}.TeX(body)
	if err != nil {
		t.Fatalf("TeX: %v", err)
	}
	if !strings.Contains(out, `\tag{4}`) {
		t.Errorf("the number beside the display was not taken:\n%s", out)
	}
	if strings.Contains(out, "(4) ") {
		t.Errorf("the number is still sitting in the text:\n%s", out)
	}
}

// An enumerated item is not a formula number, and the thing that tells them
// apart is that an item has its text on the same line. This is the case the
// backward look could have broken and did not.
func TestAnEnumeratedItemUnderADisplayIsLeftAlone(t *testing.T) {
	body := "$$\nx = y\n$$\n\n(3) In $\\mathbf{Z}$ and more generally in any ring.\n"
	out, err := Renderer{File: "x.md", Line: 1}.TeX(body)
	if err != nil {
		t.Fatalf("TeX: %v", err)
	}
	if strings.Contains(out, `\tag{3}`) {
		t.Errorf("an enumerated item was taken for a formula number:\n%s", out)
	}
	if !strings.Contains(out, "(3) In") {
		t.Errorf("the enumerated item lost its number:\n%s", out)
	}
}

// A display already carrying a number does not take a second one. Two numbers
// on one formula is worse than the paragraph this replaces, because the second
// would silently overwrite the first.
func TestADisplayDoesNotTakeTwoNumbers(t *testing.T) {
	body := "(5)\n\n$$\nx = y\n$$\n\n(6)\n\nAnd so on.\n"
	out, err := Renderer{File: "x.md", Line: 1}.TeX(body)
	if err != nil {
		t.Fatalf("TeX: %v", err)
	}
	if !strings.Contains(out, `\tag{5}`) {
		t.Errorf("the display lost the number written above it:\n%s", out)
	}
	if strings.Contains(out, `\tag{6}`) {
		t.Errorf("the display took a second number:\n%s", out)
	}
}
