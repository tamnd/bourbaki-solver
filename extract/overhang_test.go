package extract

import "testing"

// English page 36, exercise 19 of § 1, cut to the saturation mapping and the
// word before it. Bourbaki writes the inverse of the canonical mapping as
// \varepsilon_M^{-1}, so the page draws the minus and the one of the exponent
// over the M of the index, and pdftohtml hands the three runs back left to
// right as minus, M, one.
//
// M is a wide letter and 1 is a narrow one. The M starts five units inside the
// minus and ends two units past the one, so the subscript does not lie inside
// what the superscript spans, and the cluster was left as it arrived:
// \varepsilon^-_M^1, which TeX refuses by name, Double superscript.
const overhangingIndexPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="36" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="4" size="13" family="LMRoman9" color="#131413"/>
<fontspec id="5" size="13" family="LMMathItalic9" color="#131413"/>
<fontspec id="7" size="9" family="LMMathSymbols6" color="#131413"/>
<fontspec id="8" size="9" family="LMRoman6" color="#131413"/>
<text top="425" left="245" width="236" height="12" font="4">The submodule</text>
<text top="425" left="485" width="6" height="12" font="5">&#949;</text>
<text top="420" left="487" width="9" height="8" font="7">&#8722;</text>
<text top="430" left="492" width="10" height="8" font="8">M</text>
<text top="420" left="495" width="5" height="8" font="8">1</text>
</page>
</pdf2xml>
`

func TestAnIndexWiderThanTheExponentOverItIsStillAStack(t *testing.T) {
	lines := parse(t, overhangingIndexPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `The submodule $\varepsilon^{-1}_M$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// The same page with the index widened by one unit, which nothing in chapter
// VIII prints. Two units is the whole of what a glyph box carries past its ink
// at this size, and anything past that is a script standing beside the other
// rather than under it, which is what a matrix and a run-together display line
// both look like. So the overhang is a measurement and not a licence: three
// units and the cluster comes back as it arrived.
const wideOverhangPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="36" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="4" size="13" family="LMRoman9" color="#131413"/>
<fontspec id="5" size="13" family="LMMathItalic9" color="#131413"/>
<fontspec id="7" size="9" family="LMMathSymbols6" color="#131413"/>
<fontspec id="8" size="9" family="LMRoman6" color="#131413"/>
<text top="425" left="245" width="236" height="12" font="4">The submodule</text>
<text top="425" left="485" width="6" height="12" font="5">&#949;</text>
<text top="420" left="487" width="9" height="8" font="7">&#8722;</text>
<text top="430" left="492" width="11" height="8" font="8">M</text>
<text top="420" left="495" width="5" height="8" font="8">1</text>
</page>
</pdf2xml>
`

func TestAnIndexThatStandsFurtherOutThanAGlyphBoxIsLeftAlone(t *testing.T) {
	lines := parse(t, wideOverhangPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `The submodule $\varepsilon^-_M^1$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}
