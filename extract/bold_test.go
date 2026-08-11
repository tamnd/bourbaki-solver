package extract

import (
	"strings"
	"testing"
)

// Bourbaki sets two different things in bold roman: the names of the standard
// rings and groups, and every heading in the volume. Reading all of it as
// mathematics put the title of page 213 on the page one letter at a time,
// $7. \mathbf{M}\mathbf{u}\mathbf{l}\mathbf{t}\mathbf{i}$ and so on to the end
// of the line, on nineteen pages and on all seven pages of the table of
// contents.
//
// The heading of page 213, as poppler prints it. The bold run is one run and
// the K(C) after it is not bold at all, which is why the line was not read as
// a heading either: it was left to the renderer, and the renderer had nothing
// to say about it but dollar signs.
const boldHeadingXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="213" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="3" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="5" size="15" family="EBBNLB+rsfs10" color="#000000"/>
<fontspec id="11" size="15" family="GSFMFK+LMRoman10" color="#000000"/>
<text top="191" left="80" width="228" height="13" font="11"><b>7. Multiplicative Structure on</b></text>
<text top="191" left="314" width="17" height="13" font="3">K(</text>
<text top="187" left="332" width="10" height="19" font="5">C</text>
<text top="191" left="344" width="6" height="13" font="3">)</text>
</page>
</pdf2xml>
`

func TestAHeadingWithMathematicsInItIsAHeading(t *testing.T) {
	lines := parse(t, boldHeadingXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	got, ok := heading(lines[0])
	if !ok {
		t.Fatalf("the title of page 213 was not read as a heading, it rendered as %s", Render(lines[0]))
	}
	if want := `### 7. Multiplicative Structure on $K(\mathscr{C})$`; got != want {
		t.Errorf("heading:\n got %s\nwant %s", got, want)
	}
}

// The other half of the rule. A bold run of three letters or fewer is one of
// the names, and a heading it is not: the volume sets GL, SL, PGL, Sp, Z, M, N,
// Q, A, C and R in bold and nothing else.
func TestABoldNameIsStillMathematics(t *testing.T) {
	for _, s := range []string{"GL", "SL", "PGL", "Sp", "Z", "M", "N", "Q", "A", "C", "R", "α"} {
		if strong(s) {
			t.Errorf("%q was read as prose, it is one of the names Bourbaki sets bold", s)
		}
	}
	for _, s := range []string{"Exercises", "§ 16.", "1.", "40", "Historical Note", "-Extension"} {
		if !strong(s) {
			t.Errorf("%q was read as a name, it is a heading, a number of one, or a citation", s)
		}
	}
}

// The bibliography sets the volume number of a journal in bold, which is the
// one place in the volume where a bold run that is not a name stands in the
// middle of a sentence. Entry [27] of page 495 turns over onto its number, so
// the line under it opens on bold and is not the start of anything.
const boldTurnoverXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="495" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="5" size="13" family="FBFVYY+LMRoman9" color="#000000"/>
<fontspec id="6" size="13" family="DHNIXB+LMRomanCaps10" color="#000000"/>
<fontspec id="7" size="13" family="UZMNEB+LMRoman9" color="#000000"/>
<fontspec id="8" size="13" family="JPAJVR+LMRoman9" color="#000000"/>
<text top="277" left="82" width="21" height="12" font="5">[27]</text>
<text top="279" left="111" width="74" height="12" font="6">C. Hopkins</text>
<text top="277" left="190" width="299" height="12" font="5">– “Rings with minimal conditions for left ideals”,</text>
<text top="277" left="494" width="86" height="12" font="7"><i>Ann. of Math.</i></text>
<text top="297" left="110" width="16" height="12" font="8"><b>40</b></text>
<text top="297" left="131" width="115" height="12" font="5">(1939), p. 712–730.</text>
<text top="324" left="82" width="21" height="12" font="5">[28]</text>
<text top="326" left="111" width="69" height="12" font="6">W. Krull</text>
<text top="324" left="187" width="348" height="12" font="5">– “Über verallgemeinerte endliche Abelsche Gruppen”,</text>
<text top="324" left="544" width="36" height="12" font="7"><i>Math.</i></text>
<text top="344" left="110" width="53" height="12" font="7"><i>Zeitschr.</i></text>
<text top="344" left="169" width="16" height="12" font="8"><b>23</b></text>
<text top="344" left="191" width="386" height="12" font="5">(1925), p. 161–196; Gesammelte Abhandlungen, vol. 1, Berlin</text>
<text top="363" left="109" width="190" height="12" font="5">(de Gruyter), 1999, p. 263–298.</text>
</page>
</pdf2xml>
`

func TestAnEntryThatTurnsOverOntoItsVolumeNumberStaysOneParagraph(t *testing.T) {
	got := blocks(parse(t, boldTurnoverXML), nil)
	want := strings.Join([]string{
		`[27] C. Hopkins – “Rings with minimal conditions for left ideals”, Ann. of Math. **40** (1939), p. 712–730.`,
		`[28] W. Krull – “Über verallgemeinerte endliche Abelsche Gruppen”, Math. Zeitschr. **23** (1925), p. 161–196; Gesammelte Abhandlungen, vol. 1, Berlin (de Gruyter), 1999, p. 263–298.`,
	}, "\n\n")
	if got != want {
		t.Errorf("blocks:\n got %s\nwant %s", got, want)
	}
}
