package extract

import "testing"

// The fixtures are page 172 of Algebra VIII, where the typesetter broke
// "two-sided" at its own hyphen at the end of a line, and page 237, where the
// typesetter broke "commutative" at a hyphen of its own making. Nothing in
// either line says which is which.
const (
	brokenCompound = "d) The radical of A is contained in the intersection of the maximal two-"
	afterCompound  = "sided ideals of A."
	brokenWord     = "3) If A is a semisimple ring and M is a right A-module, then M is commu-"
	afterWord      = "tative."
)

// inside is a line of the same volume, on which the compound is set whole. Page
// 172 alone has three of them.
const inside = "b) Every two-sided ideal of A that contains the radical of A is the intersection of the maximal two-sided ideals of A containing it."

func TestACompoundTheVolumeWritesElsewhereKeepsItsHyphen(t *testing.T) {
	c := Compounds{}
	c.Read(inside)
	got, ok := runOn(brokenCompound, afterCompound, c)
	if !ok {
		t.Fatal("runOn() did not join the line at all")
	}
	if want := brokenCompound + afterCompound; got != want {
		t.Errorf("runOn() = %q, want the hyphen kept", got)
	}
}

func TestAWordTheVolumeNeverHyphenatesLosesIt(t *testing.T) {
	c := Compounds{}
	c.Read(inside)
	got, ok := runOn(brokenWord, afterWord, c)
	if !ok {
		t.Fatal("runOn() did not join the line at all")
	}
	if want := brokenWord[:len(brokenWord)-1] + afterWord; got != want {
		t.Errorf("runOn() = %q, want the hyphen dropped", got)
	}
}

func TestWithoutTheVolumeTheHyphenIsDropped(t *testing.T) {
	// The first of the two passes reads the volume with no compounds in hand,
	// and it has to, since this is where they come from. It gets the compound
	// wrong, which is why there is a second pass.
	got, _ := runOn(brokenCompound, afterCompound, nil)
	if want := brokenCompound[:len(brokenCompound)-1] + afterCompound; got != want {
		t.Errorf("runOn() = %q, want the hyphen dropped", got)
	}
}

func TestAHyphenAtTheEndOfALineIsNotEvidence(t *testing.T) {
	// It is the question rather than the answer, so a body that has one teaches
	// nothing. Only a hyphen with a word on both sides of it counts.
	c := Compounds{}
	c.Read(brokenCompound + "\n" + afterCompound)
	if len(c) != 0 {
		t.Errorf("Read() = %v, want nothing learned from a broken line", c)
	}
}

func TestOnlyLowerCaseWordsAreCompounds(t *testing.T) {
	// A hyphen after a capital is not the same mark. The volume writes A-module
	// and O-algebra with the symbol in mathematics, and where a line breaks
	// after that hyphen it is the mathematics on either side that decides, not
	// this.
	c := Compounds{}
	c.Read("Let M be an A-module over the O-algebra A, and let N be a sub-module of M.")
	if c["a-module"] || c["o-algebra"] {
		t.Errorf("Read() = %v, want the symbols left out", c)
	}
	if !c["sub-module"] {
		t.Errorf("Read() = %v, want sub-module", c)
	}
}
