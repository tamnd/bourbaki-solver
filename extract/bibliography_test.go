package extract

import (
	"strings"
	"testing"
)

// The bibliography is the one place in the volume set with a hanging indent.
// The entry opens at the margin with its number in square brackets and every
// line after it is indented under it, which is the opposite of the way the body
// of the book is set. Read by the ordinary rule every line of it becomes a
// paragraph, and the hyphens the typesetter left at the ends of those lines stay
// where they are, so page 493 came out with "of any group of linear sub-" as one
// paragraph and "stitutions", Proc. Lond. Math. Soc." as the next.
//
// Entries [2] and [3] of page 493, as poppler prints them. Both readings were
// checked against the printed page, the volume number of the journal included:
// it is set bold there, so it is bold here, and it shipped as $30$ for as long
// as every bold run was read as a symbol.
const bibliographyXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="493" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="1" size="13" family="FBFVYY+LMRoman9" color="#000000"/>
<fontspec id="2" size="13" family="DHNIXB+LMRomanCaps10" color="#000000"/>
<fontspec id="3" size="13" family="UZMNEB+LMRoman9" color="#000000"/>
<fontspec id="4" size="13" family="JPAJVR+LMRoman9" color="#000000"/>
<text top="418" left="89" width="15" height="12" font="1">[2]</text>
<text top="420" left="111" width="72" height="12" font="2">R. Brauer</text>
<text top="418" left="189" width="265" height="12" font="1">– “Über Systeme hypercomplexer Zahlen”,</text>
<text top="418" left="461" width="95" height="12" font="3"><i>Math. Zeitschr.</i></text>
<text top="418" left="562" width="16" height="12" font="4"><b>30</b></text>
<text top="438" left="109" width="468" height="12" font="1">(1929), p. 79–107; Collected papers, vol. I, Cambridge, Massachusetts (The</text>
<text top="457" left="111" width="168" height="12" font="1">MIT Press), 1980, p. 40–68.</text>
<text top="488" left="89" width="15" height="12" font="1">[3]</text>
<text top="490" left="111" width="86" height="12" font="2">W. Burnside</text>
<text top="488" left="202" width="378" height="12" font="1">– “On the condition of reducibility of any group of linear sub-</text>
<text top="507" left="111" width="67" height="12" font="1">stitutions”,</text>
<text top="507" left="183" width="140" height="12" font="3"><i>Proc. Lond. Math. Soc.</i></text>
<text top="507" left="328" width="8" height="12" font="4"><b>3</b></text>
<text top="507" left="340" width="115" height="12" font="1">(1905), p. 430–434.</text>
</page>
</pdf2xml>
`

func TestAnEntryOfTheBibliographyIsOneParagraph(t *testing.T) {
	got := blocks(parse(t, bibliographyXML))
	want := strings.Join([]string{
		`[2] R. Brauer – “Über Systeme hypercomplexer Zahlen”, Math. Zeitschr. **30** (1929), p. 79–107; Collected papers, vol. I, Cambridge, Massachusetts (The MIT Press), 1980, p. 40–68.`,
		`[3] W. Burnside – “On the condition of reducibility of any group of linear substitutions”, Proc. Lond. Math. Soc. **3** (1905), p. 430–434.`,
	}, "\n\n")
	if got != want {
		t.Errorf("blocks:\n got %s\nwant %s", got, want)
	}
}

// The rule turns over on the number and on nothing else. A block whose lines
// happen to hang, with no bracketed number at the margin, is read the ordinary
// way: the six enumerated properties of Proposition 4 on page 155 are indented
// under an unindented line each, and reading the shape of the block alone glued
// all six into one paragraph and cut a sentence of page 257 in half.
const hangingXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="155" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="1" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<text top="418" left="89" width="200" height="13" font="1">a) the first property holds</text>
<text top="438" left="109" width="200" height="13" font="1">and continues here</text>
<text top="457" left="89" width="200" height="13" font="1">b) the second property holds</text>
<text top="477" left="109" width="200" height="13" font="1">and continues here too</text>
<text top="496" left="109" width="200" height="13" font="1">and here as well</text>
</page>
</pdf2xml>
`

func TestABlockThatHangsWithoutNumbersIsReadTheOrdinaryWay(t *testing.T) {
	lines := parse(t, hangingXML)
	opens := opener(lines, 89)
	for _, l := range lines {
		if want := l.Left >= 89+indent; opens(l) != want {
			t.Errorf("opener says the line at top %d opens = %v, want %v", l.Top, opens(l), want)
		}
	}
}
