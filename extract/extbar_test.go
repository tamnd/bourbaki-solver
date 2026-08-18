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
