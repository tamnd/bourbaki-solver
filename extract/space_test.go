package extract

import "testing"

// A word space in this volume is five units across and the letters of a word
// touch, so three units was enough to tell one from the other until the
// operators. Bourbaki sets a thin space before the argument of an operator, and
// a thin space is two units.
//
// Page 145 is the fixture. "consisting of the subspaces Ker" ends at 376 and
// the u that Ker is applied to opens at 378, while the "where" after it stands
// six units off. Read at three the page said "Keru".
const thinSpaceXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="145" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="0" size="13" family="FBFVYY+LMRoman9" color="#000000"/>
<fontspec id="5" size="13" family="TDCNDA+LMMathItalic9" color="#000000"/>
<fontspec id="7" size="13" family="ILCVAI+EUFM10" color="#000000"/>
<text top="444" left="80" width="80" height="12" font="0">the subset of</text>
<text top="444" left="166" width="8" height="12" font="7">F</text>
<text top="444" left="181" width="195" height="12" font="0">consisting of the subspaces Ker</text>
<text top="444" left="378" width="8" height="12" font="5"><i>u</i></text>
<text top="444" left="392" width="36" height="12" font="0">where</text>
<text top="444" left="433" width="8" height="12" font="5"><i>u</i></text>
<text top="444" left="447" width="80" height="12" font="0">runs through</text>
</page>
</pdf2xml>
`

func TestTheThinSpaceBeforeAnArgumentIsASpace(t *testing.T) {
	lines := parse(t, thinSpaceXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `the subset of $\mathfrak{F}$ consisting of the subspaces Ker $u$ where $u$ runs through`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

func TestAnIndexIsWrittenAgainstWhatItIndexes(t *testing.T) {
	// The same two units are what an index is set at, and there the space would
	// be wrong: page 348 wants Inf^q. So the narrow measure is only taken for a
	// letter written on the line.
	base := token{text: "u", level: Base, math: true}
	if got := spaceGap(base); got != 2 {
		t.Errorf("spaceGap(%q on the line) = %d, want 2", base.text, got)
	}
	sup := token{text: "q", level: Sup, math: true}
	if got := spaceGap(sup); got != 3 {
		t.Errorf("spaceGap(%q above the line) = %d, want 3", sup.text, got)
	}
	if got := spaceGap(token{text: ")"}); got != 3 {
		t.Errorf("spaceGap(bracket) = %d, want 3", got)
	}
}
