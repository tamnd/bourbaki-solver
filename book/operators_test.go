package book

import (
	"strings"
	"testing"
)

// The names in this table are the ones a reading produces out of the French
// printing and no TeX engine defines, so the class defines them. They were
// found by walking every math span under pages/ and asking the typesetter,
// through \ifcsname, which of the names in them exist. The answer was sixteen
// that do not, over 150 occurrences and 28 pages of nine printings.
//
// The test is here because the failure it guards against is silent in the only
// place anybody would look. A volume whose pages carry \tg builds and ships
// today, so nothing about deleting a line below would show up until somebody
// assembles the nine printings that do not yet build, at which point the error
// is a stopped compile in a file that has not been touched in months.
//
// Two of the sixteen are not here and should not be. \oeth was n\oethérien
// read as mathematics, which is a word and not a sign, and \omicron was a
// Greek letter that is written o in every font there has ever been. Both were
// repaired on the page.
var operatorNames = []string{
	// The French names for the circular and hyperbolic functions, out of TG
	// VIII SS 2 and FVR VI. \th takes a name LaTeX already uses for the thorn.
	`\tg`, `\cotg`, `\ch`, `\sh`, `\th`, `\cosec`,
	// Commutative algebra, out of AC VIII SS 2 and AC X.
	`\codim`, `\dimgr`, `\htgr`, `\prof`, `\Ass`,
	// Leibniz writing omn. for omnia, quoted in the historical note to FVR.
	`\omn`,
}

func TestClassDefinesTheOperatorNamesThePrintingUses(t *testing.T) {
	for _, name := range operatorNames {
		if !strings.Contains(Class, `\DeclareMathOperator{`+name+`}`) {
			t.Errorf("the class does not define %s, and %s is on a page", name, name)
		}
	}
}

// \th is the one that has to be freed before it can be declared, since
// \DeclareMathOperator refuses a name that is already taken and LaTeX takes
// this one at load time. Without the \let the class does not compile at all,
// which is loud, but it is loud in a place where somebody tidying up the block
// above would be tempted to drop the line as noise.
func TestClassReleasesThornBeforeTakingItsName(t *testing.T) {
	let := strings.Index(Class, `\let\th\relax`)
	op := strings.Index(Class, `\DeclareMathOperator{\th}`)
	switch {
	case let < 0:
		t.Fatal("the class declares \\th without releasing the thorn first")
	case op < 0:
		t.Fatal("the class releases the thorn and then does not declare \\th")
	case let > op:
		t.Fatal("the class releases the thorn after declaring \\th, which is too late")
	}
}

// The arc over an inverse image, which the corpus asks for under either of the
// two package names. Both have to reach \widearc, which is the one newtxmath
// has, and neither may reach \overgroup, which draws a bar rather than a curve.
func TestClassGivesBothNamesForTheInverseImageArc(t *testing.T) {
	for _, name := range []string{`\overparen`, `\wideparen`} {
		want := `\providecommand{` + name + `}[1]{\widearc{#1}}`
		if !strings.Contains(Class, want) {
			t.Errorf("the class does not point %s at \\widearc", name)
		}
	}
}
