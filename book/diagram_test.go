package book

import (
	"strings"
	"testing"
)

// The diagram this was written for, from page 18 of the English Commutative
// Algebra. Every arrow of it has an empty cell to its right, so every one of
// them gets its label hung.
func TestAnArrowWithAnEmptyCellBesideItHangsItsLabel(t *testing.T) {
	in := `\begin{array}{ccccc}
G \otimes R & \xrightarrow{v \otimes 1_R} & H \otimes R \\
\downarrow l_G \otimes i & & \downarrow l_H \otimes i \\
G \otimes L & \xrightarrow{v \otimes 1_L} & H \otimes L
\end{array}`
	got := diagrams(in)
	if want := `\bvarrowr{\downarrow}{l_G \otimes i}`; !strings.Contains(got, want) {
		t.Errorf("the first arrow did not hang its label as %q:\n%s", want, got)
	}
	if want := `\bvarrowr{\downarrow}{l_H \otimes i}`; !strings.Contains(got, want) {
		t.Errorf("the last arrow of the row did not hang its label:\n%s", got)
	}
	if strings.Contains(got, `\downarrow l_G`) {
		t.Errorf("the old spelling of the cell is still there:\n%s", got)
	}
}

// A label written to the left of its arrow hangs to the left, because that is
// the side the diagram put it on and moving it to the other side would rename
// nothing but would move every map in the picture.
func TestALabelWrittenBeforeTheArrowHangsToTheLeft(t *testing.T) {
	in := `\begin{array}{ccc}
A & \to & B \\
a \downarrow & & b \downarrow \\
A' & \to & B'
\end{array}`
	got := diagrams(in)
	if want := `\bvarrowl{\downarrow}{a}`; !strings.Contains(got, want) {
		t.Errorf("the label written first did not hang to the left:\n%s", got)
	}
	if want := `\bvarrowl{\downarrow}{b}`; !strings.Contains(got, want) {
		t.Errorf("the second label did not hang to the left:\n%s", got)
	}
}

// A box of no width prints over whatever is beside it, so an arrow with a
// filled cell to its right keeps the setting it had. Out of line and legible
// beats in line and printed on top of the next column.
//
// The second arrow of the same row is a different case and is allowed: nothing
// of the array is to the right of it, so the box hangs into the white paper
// beside a centred display.
func TestAnArrowWithAFilledCellBesideItIsLeftAlone(t *testing.T) {
	in := `\begin{array}{cc}
\downarrow f & \downarrow g
\end{array}`
	got := diagrams(in)
	if !strings.Contains(got, `\downarrow f &`) {
		t.Errorf("the arrow with something beside it was rewritten:\n%s", got)
	}
	if want := `\bvarrowr{\downarrow}{g}`; !strings.Contains(got, want) {
		t.Errorf("the arrow at the edge of the array was not rewritten:\n%s", got)
	}
}

// A column that is empty in every row has no width, so it is not room. The
// label would hang straight over the column after it.
func TestAColumnThatIsEmptyThroughoutIsNotRoom(t *testing.T) {
	in := `\begin{array}{ccc}
\downarrow f & & A \to B \\
\downarrow g & & C \to D
\end{array}`
	if got := diagrams(in); strings.Contains(got, `\bvarrow`) {
		t.Errorf("a label was hung into a column of no width:\n%s", got)
	}
}

// An arrow with no name has nothing to hang and is already centred, so there is
// nothing here to do.
func TestABareArrowIsLeftAlone(t *testing.T) {
	in := `\begin{array}{ccc}A & & B \\ \downarrow & & \uparrow\end{array}`
	if got := diagrams(in); got != in {
		t.Errorf("a bare arrow was wrapped:\n%s", got)
	}
}

// A row that lost its tab reads as one cell holding an arrow and then the rest
// of the row. Hanging that in a box of no width would print it across the page,
// so the length is capped and a long cell is left as it is.
func TestARowThatLostItsTabIsNotTreatedAsALabel(t *testing.T) {
	lost := `\downarrow f A \to B \to C \to D \to E \to F \to G \to H \to I \to J`
	if len([]rune(lost)) <= maxArrowLabel {
		t.Fatal("the fixture is too short to be testing what it is for")
	}
	in := `\begin{array}{ccc}` + lost + ` & \xrightarrow{u} & Z\end{array}`
	if got := diagrams(in); strings.Contains(got, `\bvarrow`) {
		t.Errorf("half a row was hung in a box of no width:\n%s", got)
	}
}

// A tabular is a table and not a diagram. The multiplication tables of Algebra
// III have arrows in them nowhere, but a rule in the preamble and a stub column
// is not a picture and nothing here should be reaching into one.
func TestATabularIsNotADiagram(t *testing.T) {
	in := `\begin{tabular}{ccc}\downarrow f & & x\end{tabular}`
	if got := diagrams(in); got != in {
		t.Errorf("a tabular was rewritten:\n%s", got)
	}
}

// A diagram drawn inside a cell of another one is done first, so the outer pass
// reads a cell that already holds the finished inner array rather than one it
// is about to change underneath itself.
func TestADiagramInsideACellIsDoneToo(t *testing.T) {
	in := `\begin{array}{cc}` +
		`\begin{array}{cc}\downarrow f & \end{array} & x \end{array}`
	got := diagrams(in)
	if want := `\bvarrowr{\downarrow}{f}`; !strings.Contains(got, want) {
		t.Errorf("the inner diagram was not read:\n%s", got)
	}
}

// The tab inside a nested environment belongs to that environment. Counting it
// here would cut a cell in half and leave the arrow in one piece and its name
// in another.
func TestATabInsideANestedEnvironmentIsNotACellBoundary(t *testing.T) {
	in := `\begin{array}{ccc}` +
		`\downarrow \begin{smallmatrix}a & b\end{smallmatrix} & & y` +
		`\end{array}`
	got := diagrams(in)
	if strings.Contains(got, `\bvarrow`) {
		t.Errorf("a label holding an environment was hung:\n%s", got)
	}
	if !strings.Contains(got, `\begin{smallmatrix}a & b\end{smallmatrix}`) {
		t.Errorf("the nested environment did not come through whole:\n%s", got)
	}
}

// An array whose preamble is short is widened before this runs, and the two
// passes have to leave the same body behind. This is the pair of them in the
// order latex.go runs them.
func TestWideningAndHangingRunTogether(t *testing.T) {
	in := `\begin{array}{cc}
A & \to & B \\
\downarrow f & & \downarrow g
\end{array}`
	widened, wide := widen(in)
	if len(wide) != 1 {
		t.Fatalf("the short preamble was not reported: %v", wide)
	}
	got := diagrams(widened)
	if want := `\bvarrowr{\downarrow}{f}`; !strings.Contains(got, want) {
		t.Errorf("the arrow did not hang after widening:\n%s", got)
	}
	if !strings.Contains(got, `{ccc}`) {
		t.Errorf("the widened preamble was lost:\n%s", got)
	}
}
