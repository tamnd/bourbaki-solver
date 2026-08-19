package pdfsrc

import (
	"strings"
	"testing"
)

const bbox = `<!DOCTYPE html PUBLIC "-//W3C//DTD XHTML 1.0 Transitional//EN" "http://www.w3.org/TR/xhtml1/DTD/xhtml1-transitional.dtd"><html xmlns="http://www.w3.org/1999/xhtml">
<head><title></title></head>
<body>
<doc>
  <page width="439.371000" height="666.142000">
    <word xMin="73.339000" yMin="489.400000" xMax="88.000000" yMax="498.000000">Let</word>
    <word xMin="92.000000" yMin="489.400000" xMax="99.333000" yMax="498.000000">P</word>
    <word xMin="260.000000" yMin="489.400000" xMax="266.667000" yMax="498.000000">P</word>
  </page>
</doc>
</body>
</html>
`

// The boxes come back in the pixels the run reading is in, so a word and a run
// can be compared without either being converted. This is page 177 of Algebra
// VIII, which pdftohtml reports 659 pixels wide against the 439.371 points
// pdftotext reports, and where the second P of the line stands at 390.
func TestParseBBoxScalesToThePixelsOfTheRuns(t *testing.T) {
	got, err := ParseBBox(strings.NewReader(bbox), 659)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d words, want 3", len(got))
	}
	if got[0].Left != 110 || got[0].Text != "Let" {
		t.Errorf("got %d %q, want 110 \"Let\"", got[0].Left, got[0].Text)
	}
	if got[2].Left != 390 || got[2].Right() != 400 {
		t.Errorf("got %d..%d, want 390..400", got[2].Left, got[2].Right())
	}
}

// A glyph the font maps to nothing arrives at whatever code the font gave it,
// and XML 1.0 has no way of carrying a control character. Algebra VIII has one
// at U+000F, and before this the reader stopped on the page rather than on the
// word and the whole volume went unread.
func TestParseBBoxReadsPastACharacterXMLForbids(t *testing.T) {
	got, err := ParseBBox(strings.NewReader(strings.Replace(bbox, ">P</word>", ">\x0fP</word>", 1)), 659)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d words, want 3", len(got))
	}
	if got[1].Text != "P" {
		t.Errorf("got %q, want \"P\"", got[1].Text)
	}
}

// One call asks for one page, so a caller that asked for a range gets the first
// page of it and not the words of every page piled together.
func TestParseBBoxTakesOnlyTheFirstPage(t *testing.T) {
	two := strings.Replace(bbox, "</doc>", `  <page width="439.371000" height="666.142000">
    <word xMin="73.339000" yMin="489.400000" xMax="88.000000" yMax="498.000000">Next</word>
  </page>
</doc>`, 1)
	got, err := ParseBBox(strings.NewReader(two), 659)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 3 {
		t.Fatalf("got %d words, want 3", len(got))
	}
}
