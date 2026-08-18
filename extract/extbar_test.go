package extract

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// The inequality at the head of page 122 of Théories spectrales I, which bounds
// the norm of the integral of f against a vector measure. The norm is drawn
// around a display an integral tall, so each of its two bars is three pieces of
// code 13 of the extension font stacked at one left edge, and the volume
// shipped it as \|\|\|\int ...\|\|\|.
const stackedBarXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="122" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="2" size="15" family="DGLKJH+CMR10" color="#000000"/>
<fontspec id="3" size="15" family="DGLMMN+CMMI10" color="#000000"/>
<fontspec id="4" size="15" family="DGLKKM+CMSY10" color="#000000"/>
<fontspec id="9" size="15" family="DGMCMP+CMEX10" color="#000000"/>
<text top="180" left="120" width="8" height="12" font="9">=</text>
<text top="191" left="120" width="8" height="12" font="9">=</text>
<text top="202" left="120" width="8" height="12" font="9">=</text>
<text top="188" left="132" width="11" height="24" font="9">Z</text>
<text top="191" left="150" width="10" height="13" font="3"><i>f</i></text>
<text top="191" left="164" width="10" height="13" font="3"><i>ω</i></text>
<text top="180" left="180" width="8" height="12" font="9">=</text>
<text top="191" left="180" width="8" height="12" font="9">=</text>
<text top="202" left="180" width="8" height="12" font="9">=</text>
<text top="191" left="196" width="14" height="13" font="4"><i>≤</i></text>
</page>
</pdf2xml>
`

// A norm drawn taller than one glyph comes back as one norm.
//
// The bars either side of it are the whole of the notation, and a reader that
// counted the pieces read the display as the norm of the norm of the norm. The
// page carries no mark of it either: every piece is a character poppler names,
// so nothing is lost and nothing is flagged, and the reading is simply wrong.
func TestANormDrawnTallerThanOneGlyphIsOneNorm(t *testing.T) {
	lay, err := pdfsrc.ParseXML(strings.NewReader(stackedBarXML))
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}
	p := ReadPage(lay, lay.Pages[0])
	if p.Pieces != 4 {
		t.Errorf("Pieces = %d, want 4", p.Pieces)
	}
	if strings.Contains(p.Body, `\|\|`) {
		t.Errorf("the page reads %q", p.Body)
	}
	if strings.Count(p.Body, `\|`) != 2 {
		t.Errorf("the page draws two bars and reads %q", p.Body)
	}
}

// Three bars set side by side is the operator norm and stays three bars.
//
// This is the reason the test is the left edge and not the number of pieces.
// Théories spectrales writes the norm of an endomorphism with three bars to a
// side, at three left edges on one line, and a rule that gathered by count
// would take the notation of the volume for a drawing accident.
const tripleBarXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="1" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="3" size="15" family="DGLMMN+CMMI10" color="#000000"/>
<fontspec id="9" size="15" family="DGMCMP+CMEX10" color="#000000"/>
<text top="191" left="120" width="8" height="12" font="9">&lt;</text>
<text top="191" left="128" width="8" height="12" font="9">&lt;</text>
<text top="191" left="136" width="8" height="12" font="9">&lt;</text>
<text top="191" left="148" width="10" height="13" font="3"><i>u</i></text>
<text top="191" left="162" width="8" height="12" font="9">&lt;</text>
<text top="191" left="170" width="8" height="12" font="9">&lt;</text>
<text top="191" left="178" width="8" height="12" font="9">&lt;</text>
</page>
</pdf2xml>
`

func TestTheOperatorNormKeepsItsThreeBars(t *testing.T) {
	lay, err := pdfsrc.ParseXML(strings.NewReader(tripleBarXML))
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}
	p := ReadPage(lay, lay.Pages[0])
	if p.Pieces != 0 {
		t.Errorf("Pieces = %d, want 0", p.Pieces)
	}
	if strings.Count(p.Body, "|") != 6 {
		t.Errorf("the page draws six bars and reads %q", p.Body)
	}
}

// Equation (28) of § 21 of Algebra VIII, the inner product of two class
// functions written as an average over the group. TeX centres the bound under
// the summation sign, and the bound is wider than the sign, so the g of "g in
// G" is drawn one unit to the left of it.
const limitLeftOfSignXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="427" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="2" size="15" family="ABCDEF+LMRoman10" color="#000000"/>
<fontspec id="3" size="15" family="ABCDEG+LMMathItalic10" color="#000000"/>
<fontspec id="4" size="15" family="ABCDEH+LMMathSymbols10" color="#000000"/>
<fontspec id="5" size="10" family="ABCDEI+LMMathSymbols7" color="#000000"/>
<fontspec id="6" size="10" family="ABCDEJ+LMRoman7" color="#000000"/>
<fontspec id="7" size="10" family="ABCDEK+LMMathItalic7" color="#000000"/>
<fontspec id="9" size="15" family="ABCDEL+CMEX10" color="#000000"/>
<text top="625" left="310" width="4" height="14" font="4"><i>|</i></text>
<text top="626" left="314" width="12" height="13" font="2">G</text>
<text top="625" left="326" width="4" height="14" font="4"><i>|</i></text>
<text top="623" left="330" width="9" height="10" font="5"><i>−</i></text>
<text top="623" left="340" width="6" height="9" font="6">1</text>
<text top="647" left="349" width="6" height="9" font="7"><i>g</i></text>
<text top="622" left="350" width="22" height="7" font="9">X</text>
<text top="647" left="355" width="8" height="10" font="5"><i>∈</i></text>
<text top="647" left="363" width="9" height="9" font="6">G</text>
<text top="626" left="375" width="7" height="13" font="3"><i>f</i></text>
</page>
</pdf2xml>
`

// A limit drawn to the left of its sign is still a limit of that sign.
//
// A line is read left to right and TeX centres a limit under the sign it
// belongs to, so a bound wider than the sign hangs out on both sides and its
// first character comes before it. The g of the sum over g in G was read as an
// index of the order of the group standing before the sign, and the display
// shipped as "|G|^{-1}_g\sum_{\in G}": a subscript the page does not print, a
// bound with its variable gone out of it, and a line KaTeX refuses.
func TestALimitDrawnLeftOfItsSignBelongsToTheSign(t *testing.T) {
	lay, err := pdfsrc.ParseXML(strings.NewReader(limitLeftOfSignXML))
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}
	p := ReadPage(lay, lay.Pages[0])
	if strings.Contains(p.Body, `_g\sum`) {
		t.Errorf("the bound was read as an index of what stands before the sign: %q", p.Body)
	}
	if !strings.Contains(p.Body, `\sum_{g\in G}`) {
		t.Errorf("the page reads %q, want the sum bounded by g in G", p.Body)
	}
}
