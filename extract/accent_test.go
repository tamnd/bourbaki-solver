package extract

import "testing"

// The dual of a locally compact commutative group, out of Théories spectrales
// chapitres 1 et 2. The hat is drawn out of MSBM10 and the volume names no
// glyph, so poppler falls back on the code and hands back an opening bracket
// sitting where the hat was drawn. Page 240 shipped "G[/H" and page 217 shipped
// "\\circ", which is not mathematics at all but a line break inside a formula.
const wideHatPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="240" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="3" size="16" family="VxcftvXychsnNbbbxkJyhgbrLMRoman10" color="#131413"/>
<fontspec id="7" size="16" family="DmblwdPfyppxVvkxlkSwtntqMSBM10" color="#131413"/>
<text top="643" left="81" width="46" height="14" font="3">Soit </text>
<text top="643" left="194" width="13" height="14" font="3">G</text>
<text top="642" left="195" width="13" height="13" font="7">[</text>
</page>
</pdf2xml>
`

func TestAWideAccentDrawnInTheAMSFontIsAnAccent(t *testing.T) {
	lines := parse(t, wideHatPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `Soit $\widehat{G}$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// The row is what the embedded subset says it is, and it is written down whole
// because the volume that draws only the hat is not the last volume.
func TestTheWideAccentsOfTheAMSFontAreTheOnesItCarries(t *testing.T) {
	for code, want := range map[rune]string{
		'[': `\widehat`, '\\': `\widehat`,
		']': `\widetilde`, '^': `\widetilde`,
	} {
		if got := msbm[code]; got != want {
			t.Errorf("msbm[%q] = %q, want %q", code, got, want)
		}
	}
	if len(msbm) != 4 {
		t.Errorf("the AMS symbol table has %d entries, want the 4 the font carries", len(msbm))
	}
}
