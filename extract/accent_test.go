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

// Page 420 of Théories spectrales chapitres 3 à 5, the remark that defines the
// reflected function. The check is drawn out of TeX-mathx10 and that subset
// names its narrow check "q", which the glyph list resolves to the letter q, so
// nothing is reported lost anywhere and the page shipped "la fonction $fq$ sur
// G", which reads as a product of two functions that is not there.
const checkAccentPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="420" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="3" size="16" family="KVZJDY+LMRoman10" color="#131413"/>
<fontspec id="4" size="16" family="OSYQLC+LMMathItalic10" color="#131413"/>
<fontspec id="9" size="16" family="LPADZX+TeX-mathx10" color="#131413"/>
<text top="574" left="81" width="80" height="14" font="3">la fonction </text>
<text top="574" left="168" width="7" height="15" font="4">f</text>
<text top="584" left="168" width="9" height="7" font="9">q</text>
<text top="574" left="181" width="30" height="14" font="3"> sur G</text>
</page>
</pdf2xml>
`

func TestACheckIsNotTheLetterQ(t *testing.T) {
	lines := parse(t, checkAccentPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `la fonction $\check{f}$ sur G`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// A 4 out of the first AMS symbol font is a preceding relation the same way a 6
// out of it is a slanted inequality, and the whole short row is written down.
func TestTheAMSRelationsAreNotDigits(t *testing.T) {
	for code, want := range map[rune]string{
		'4': `\preccurlyeq `, '6': `\leqslant `, '<': `\succcurlyeq `,
		'>': `\geqslant `, '{': `\complement `,
	} {
		if got := msam[code]; got != want {
			t.Errorf("msam[%q] = %q, want %q", code, got, want)
		}
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
