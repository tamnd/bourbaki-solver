package extract

import "testing"

// Page 100 of Théories spectrales chapitres 1 et 2, the line that defines the
// spectral radius as a limit of norms. Two glyphs on it carry names TeX made up
// and the Adobe glyph list has never heard of: varrho at 0x25 of the
// mathematics italic, which poppler prints as a per cent sign, and bardbl at
// 0x6B of the symbol font, which it prints as the letter k.
const variantGreekPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="100" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="4" size="16" family="QnnxqjJsnccdPstrhwMdcrddLMMathItalic10" color="#131413"/>
<fontspec id="5" size="16" family="WsbxwfCqqsdxNcxwdlBjnmwmLMMathSymbols10" color="#131413"/>
<fontspec id="6" size="16" family="SnsrnnQjcxplMknsjpVdyrqvLMRoman10" color="#131413"/>
<text top="200" left="100" width="9" height="12" font="4">%</text>
<text top="200" left="110" width="4" height="12" font="6">(</text>
<text top="200" left="115" width="8" height="12" font="4">x</text>
<text top="200" left="124" width="4" height="12" font="6">)</text>
<text top="200" left="136" width="9" height="12" font="6">=</text>
<text top="200" left="150" width="5" height="12" font="5">k</text>
<text top="200" left="156" width="8" height="12" font="4">x</text>
<text top="200" left="165" width="5" height="12" font="5">k</text>
</page>
</pdf2xml>
`

func TestTheVariantGreekAndTheNormAreNotPunctuation(t *testing.T) {
	lines := parse(t, variantGreekPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `$\varrho (x)=\|x\|$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// A per cent sign inside mathematics comments the rest of a LaTeX line out and
// a dollar sign closes the mathematics it is inside of, so neither is a thing
// the corpus may carry out of a mathematics font. The row is the whole of the
// variant Greek and is written down as a row, since the code is the encoding
// and the volume that draws only two of the six is not the last volume.
func TestTheVariantGreekRowIsWholeAndIsNeverPunctuation(t *testing.T) {
	for code, want := range map[rune]string{
		'"': `\varepsilon `, '#': `\vartheta `, '$': `\varpi `,
		'%': `\varrho `, '&': `\varsigma `, '\'': `\varphi `,
	} {
		if got := cmmi[code]; got != want {
			t.Errorf("cmmi[%q] = %q, want %q", code, got, want)
		}
	}
	if len(cmmi) != 6 {
		t.Errorf("the mathematics italic table has %d entries, want the 6 of the row", len(cmmi))
	}
}

// The hook of a mapsto and the arrow after it are one symbol drawn as two
// glyphs, and the pair table reads them where they arrive in the same run. It
// cannot where they do not, and three of them across the six volumes do not, so
// the hook alone still has to be the hook and not the digit it is printed as.
func TestTheHookOfAMapstoIsNotADigit(t *testing.T) {
	if got := symbols("7"); got != `\mapstochar ` {
		t.Errorf("symbols(\"7\") = %q, want the hook", got)
	}
	if got := symbols("7→"); got != `\mapsto ` {
		t.Errorf("symbols(\"7→\") = %q, want the whole symbol", got)
	}
}
