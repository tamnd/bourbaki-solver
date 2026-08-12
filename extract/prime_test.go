package extract

import (
	"strings"
	"testing"
)

// French page 287, the diagram of § 16 that carries the Brauer group of a
// subextension, cut to the first object of it and its arrow label. Bourbaki
// sets F', its index 1, and over the arrow the map iota' with its own index.
// TeX draws a prime as a raised glyph out of the symbol font, so it comes back
// as a superscript run, which is what the character "0" at the smaller size is
// here.
const primedBasePage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="287" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="4" size="14" family="LMRoman12" color="#131413"/>
<fontspec id="7" size="9" family="LMMathSymbols8" color="#131413"/>
<fontspec id="8" size="9" family="LMRoman8" color="#131413"/>
<fontspec id="9" size="9" family="LMMathItalic8" color="#131413"/>
<fontspec id="10" size="7" family="LMMathSymbols6" color="#131413"/>
<fontspec id="11" size="7" family="LMRoman6" color="#131413"/>
<text top="184" left="266" width="9" height="12" font="4">F</text>
<text top="180" left="275" width="3" height="12" font="7">0</text>
<text top="191" left="275" width="5" height="8" font="8">1</text>
<text top="176" left="295" width="3" height="8" font="9">&#953;</text>
<text top="173" left="298" width="3" height="9" font="10">0</text>
<text top="179" left="298" width="4" height="9" font="11">1</text>
</page>
</pdf2xml>
`

// A prime is a superscript to TeX, so a base that carries one and then takes a
// superscript of its own is two superscripts against one base, and TeX refuses
// it by name: Double superscript. It prints, and it reads, and KaTeX will not
// set it, so the site could not carry the page. The base and its index go
// inside braces.
func TestAPrimedBaseIsBracedBeforeItTakesASuperscript(t *testing.T) {
	lines := parse(t, primedBasePage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `${F'_1}^{\iota'_1}$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// The braces are only for the base that needs them. F'_1 with nothing over it
// is what the same page writes in its prose, and TeX sets that as it stands:
// bracing every prime would put a group around half the mathematics of the
// section for the sake of eleven spans.
const primedBaseAlonePage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="287" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="4" size="14" family="LMRoman12" color="#131413"/>
<fontspec id="7" size="9" family="LMMathSymbols8" color="#131413"/>
<fontspec id="8" size="9" family="LMRoman8" color="#131413"/>
<text top="130" left="164" width="9" height="12" font="4">F</text>
<text top="126" left="173" width="3" height="12" font="7">0</text>
<text top="136" left="173" width="5" height="8" font="8">1</text>
</page>
</pdf2xml>
`

func TestAPrimedBaseWithNothingOverItIsLeftAlone(t *testing.T) {
	lines := parse(t, primedBaseAlonePage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	got := Render(lines[0])
	if strings.ContainsAny(got, "{}") {
		t.Errorf("Render braced a base that needs no braces: %s", got)
	}
	if want := `$F'_1$`; got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}
