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

// Page 85 of Lie 7 to 9, the corollary of § 2 no. 2, cut to the piece that
// carries the fault. Bourbaki sets the element (−1 0 / 0 −1) of SL(2, k) as a
// 2 by 2 matrix in the middle of a running sentence, and the minus sign of its
// top row is drawn at the size of the body but nineteen units deep against the
// thirteen of the type beside it. The line takes the deeper box, and the prime
// of the E′ at the end of the sentence is then measured against a box that
// reaches well below the letter it is written on, so it reads as an index.
//
// The volume is set in Computer Modern, where the prime of the seven point
// symbol font is reported fourteen units high against the thirteen of the ten
// point roman it hangs off, which is why the band rule of widen does not reach
// it: the prime is not what took the line down, the matrix is.
const primeUnderAMatrixPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="85" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="1" size="15" family="DGLKJH+CMR10" color="#000000"/>
<fontspec id="5" size="15" family="DGLKKM+CMSY10" color="#000000"/>
<fontspec id="7" size="15" family="DGMBID+CMTI10" color="#000000"/>
<fontspec id="8" size="10" family="DGLNOH+CMSY7" color="#000000"/>
<text top="180" left="82" width="191" height="13" font="7"><i>The element</i></text>
<text top="168" left="292" width="12" height="19" font="5"><i>&#8722;</i></text>
<text top="171" left="304" width="7" height="13" font="1">1</text>
<text top="180" left="441" width="72" height="13" font="7"><i>operates by</i></text>
<text top="180" left="519" width="19" height="13" font="1">+1</text>
<text top="180" left="543" width="16" height="13" font="7"><i>on</i></text>
<text top="180" left="565" width="10" height="13" font="1">E</text>
<text top="176" left="575" width="3" height="14" font="8">0</text>
</page>
</pdf2xml>
`

// Nothing is ever written below the line as a prime, so a prime measured to be
// there was mismeasured. It comes back to the line the same way a prime read as
// an exponent does, and E' is what the page prints.
func TestAPrimeIsNeverReadAsAnIndex(t *testing.T) {
	lines := parse(t, primeUnderAMatrixPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	got := Render(lines[0])
	if strings.Contains(got, "_") {
		t.Errorf("the prime came back as an index: %s", got)
	}
	if !strings.Contains(got, `E'`) {
		t.Errorf("Render:\n got %s\nwant a line ending in E'", got)
	}
}
