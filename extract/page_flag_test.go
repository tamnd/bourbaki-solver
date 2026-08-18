package extract

import (
	"slices"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/mathtex"
	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// A page whose text layer flattened a matrix is flagged, and the flag is what
// render -flagged and ocr fill -flagged read to send the page to the model.
func TestAPageWithAStackedMatrixIsFlagged(t *testing.T) {
	p := &Page{Body: `Soit A $= (^{a b}_{c d})$ un élément.`}
	if mathtex.StackedMatrices(p.Body) == 0 {
		t.Fatal("the body was not read as carrying a matrix")
	}
	p.flag(FlagStackedMatrix)
	if !p.Flagged() {
		t.Error("the page was not flagged")
	}
}

// The top row of the commutative diagram on page 119 of Lie 7 to 9, with the
// three vertical arrows that hang off it.
//
// The page prints two exact sequences one above the other with an arrow from
// each term of the upper to the term below it. Every arrowhead is an empty
// element, which is poppler saying it has no character for the code and
// dropping it, so all that is left of an arrow is the two shaft pieces under
// it and there is nothing in the layer to say what they joined.
const diagramXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="119" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="1" size="15" family="DGLKJH+CMR10" color="#000000"/>
<fontspec id="3" size="15" family="DGLMMN+CMMI10" color="#000000"/>
<fontspec id="5" size="10" family="DGLOII+CMR7" color="#000000"/>
<fontspec id="7" size="15" family="DGLMFD+EUFM10" color="#000000"/>
<fontspec id="8" size="10" family="DGLMOM+CMMI7" color="#000000"/>
<fontspec id="9" size="15" family="DGLKKM+CMSY10" color="#000000"/>
<fontspec id="10" size="15" family="DGMCMP+CMEX10" color="#000000"/>
<text top="246" left="82" width="210" height="13" font="1">following commutative diagram:</text>
<text top="274" left="107" width="7" height="13" font="1">1</text>
<text top="271" left="130" width="24" height="19" font="9"><i>−→</i></text>
<text top="274" left="169" width="11" height="13" font="1">T</text>
<text top="280" left="180" width="9" height="9" font="5">Q</text>
<text top="268" left="213" width="6" height="9" font="8"><i>f</i></text>
<text top="271" left="204" width="24" height="19" font="9"><i>−→</i></text>
<text top="274" left="247" width="31" height="13" font="1">Aut(</text>
<text top="276" left="278" width="8" height="12" font="7">g</text>
<text top="274" left="285" width="4" height="13" font="3"><i>,</i></text>
<text top="276" left="292" width="8" height="12" font="7">h</text>
<text top="274" left="300" width="6" height="13" font="1">)</text>
<text top="269" left="333" width="6" height="9" font="8"><i>ε</i></text>
<text top="271" left="324" width="24" height="19" font="9"><i>−→</i></text>
<text top="274" left="365" width="34" height="13" font="1">A(R)</text>
<text top="271" left="416" width="24" height="19" font="9"><i>−→</i></text>
<text top="274" left="455" width="7" height="13" font="1">1</text>
<text top="276" left="174" width="10" height="19" font="10"></text>
<text top="285" left="174" width="10" height="19" font="10">⏐</text>
<text top="294" left="174" width="10" height="19" font="10">⏐</text>
<text top="276" left="271" width="10" height="19" font="10"></text>
<text top="285" left="271" width="10" height="19" font="10">⏐</text>
<text top="294" left="271" width="10" height="19" font="10">⏐</text>
<text top="276" left="377" width="10" height="19" font="10"></text>
<text top="285" left="377" width="10" height="19" font="10">⏐</text>
<text top="294" left="377" width="10" height="19" font="10">⏐</text>
</page>
</pdf2xml>
`

// A diagram whose vertical arrows came apart is flagged as a diagram.
//
// The page used to ship three exact sequences one after another with nothing
// between them and the label of one arrow attached to the term above it as an
// exponent, and it raised no flag at all, so the repair pass never saw it and
// the audit had nothing to compare against. Saying so is the whole of the fix:
// nothing puts a dropped arrowhead back.
func TestADiagramWhoseVerticalArrowsCameApartIsFlagged(t *testing.T) {
	lay, err := pdfsrc.ParseXML(strings.NewReader(diagramXML))
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}
	p := ReadPage(lay, lay.Pages[0])
	if !slices.Contains(p.Flags, FlagDiagram) {
		t.Errorf("Flags = %v, want one of them %s", p.Flags, FlagDiagram)
	}
}

// The display at the head of page 229 of Lie 7 to 9, which decomposes the
// square of V(m) as a direct sum and closes it with V(0) if m is even set over
// V(2) if m is odd behind a tall brace.
//
// The two rows of the brace stand a line apart, above and below the chain of
// direct sums they close, and it is the chain they gather onto: a display is
// one line as far as the layer is concerned however many rows are drawn in it.
const stackedRowsXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="229" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="2" size="15" family="DGLKJH+CMR10" color="#000000"/>
<fontspec id="3" size="15" family="DGLMMN+CMMI10" color="#000000"/>
<fontspec id="4" size="15" family="DGLKKM+CMSY10" color="#000000"/>
<fontspec id="9" size="15" family="DGMCMP+CMEX10" color="#000000"/>
<text top="200" left="105" width="24" height="13" font="2">V(2</text>
<text top="200" left="129" width="13" height="13" font="3"><i>m</i></text>
<text top="200" left="142" width="6" height="13" font="2">)</text>
<text top="197" left="152" width="12" height="19" font="4"><i>⊕</i></text>
<text top="200" left="167" width="24" height="13" font="2">V(2</text>
<text top="200" left="191" width="13" height="13" font="3"><i>m</i></text>
<text top="197" left="207" width="12" height="19" font="4"><i>−</i></text>
<text top="200" left="222" width="13" height="13" font="2">4)</text>
<text top="197" left="326" width="47" height="19" font="4"><i>⊕ · · · ⊕</i></text>
<text top="176" left="377" width="11" height="19" font="9">(</text>
<text top="192" left="391" width="30" height="13" font="2">V(0)</text>
<text top="192" left="436" width="9" height="13" font="2">if</text>
<text top="192" left="449" width="13" height="13" font="3"><i>m</i></text>
<text top="192" left="468" width="44" height="13" font="2">is even</text>
<text top="209" left="391" width="30" height="13" font="2">V(2)</text>
<text top="209" left="436" width="9" height="13" font="2">if</text>
<text top="209" left="449" width="13" height="13" font="3"><i>m</i></text>
<text top="209" left="468" width="39" height="13" font="2">is odd</text>
</page>
</pdf2xml>
`

// A display whose rows the gathering ran together is flagged.
//
// The reading is still wrong after this and is meant to be: the rows are placed
// by coordinate and there is no row in the layer to gather them by, so the page
// reads "V(0)V(2) ifif m is evenis odd" and wants the model. What the flag
// changes is that the page now says so.
func TestADisplayWhoseRowsRanTogetherIsFlagged(t *testing.T) {
	lay, err := pdfsrc.ParseXML(strings.NewReader(stackedRowsXML))
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}
	p := ReadPage(lay, lay.Pages[0])
	if !slices.Contains(p.Flags, FlagStackedRows) {
		t.Errorf("Flags = %v, want one of them %s", p.Flags, FlagStackedRows)
	}
}

// Two scripts of one base are not two rows, which is what keeps the rule above
// off every subscripted term in the volume. TeX sets a superscript and the
// subscript under it at the same left edge, so the columns line up, but it sets
// them smaller and off the baseline and it never sets a column of four.
func TestAPairOfScriptsIsNotAPairOfRows(t *testing.T) {
	lay, err := pdfsrc.ParseXML(strings.NewReader(scriptPairXML))
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}
	p := ReadPage(lay, lay.Pages[0])
	if slices.Contains(p.Flags, FlagStackedRows) {
		t.Errorf("Flags = %v, want none of them %s", p.Flags, FlagStackedRows)
	}
}

// A sum over i in I of the module M sub i, set in display style so that the
// limits stand over and under the sign and the two of them share a left edge.
const scriptPairXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="1" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="2" size="15" family="DGLKJH+CMR10" color="#000000"/>
<fontspec id="3" size="15" family="DGLMMN+CMMI10" color="#000000"/>
<fontspec id="9" size="15" family="DGMCMP+CMEX10" color="#000000"/>
<fontspec id="10" size="10" family="DGLMOM+CMMI7" color="#000000"/>
<text top="192" left="240" width="19" height="24" font="9">X</text>
<text top="182" left="240" width="12" height="9" font="10"><i>n</i></text>
<text top="218" left="240" width="14" height="9" font="10"><i>i</i>=1</text>
<text top="192" left="264" width="16" height="13" font="3"><i>M</i></text>
<text top="198" left="280" width="5" height="9" font="10"><i>i</i></text>
</page>
</pdf2xml>
`

// The head of Corollary 1 of Lie 8, § 7, no. 3, which names the family of
// fundamental weights. The letter is the varpi at code 0x17 of the mathematics
// italic, and poppler had no name and no CMap entry for it, so the element
// carries a box and nothing else.
const lostGlyphXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="141" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="2" size="15" family="DGLKJH+CMR10" color="#000000"/>
<fontspec id="13" size="15" family="DGLMMN+CMMI10" color="#000000"/>
<fontspec id="14" size="10" family="DGLMOM+CMMI7" color="#000000"/>
<text top="260" left="82" width="112" height="13" font="2">COROLLARY 1.</text>
<text top="260" left="226" width="6" height="13" font="2">(</text>
<text top="260" left="232" width="12" height="13" font="13"><i></i></text>
<text top="266" left="244" width="8" height="9" font="14"><i>α</i></text>
<text top="260" left="253" width="6" height="13" font="2">)</text>
</page>
</pdf2xml>
`

// A page that lost a glyph outright says so.
//
// This is the loss nothing else catches. A glyph read as the wrong character
// is on the page for a rule to find, and a glyph read as its code is
// punctuation in the middle of a formula. A glyph read as nothing leaves a
// page that reads well with a letter gone out of the middle of it, and this
// corollary shipped as "the family ( )_{alpha in B}" for a year.
func TestAPageThatLostAGlyphIsFlagged(t *testing.T) {
	lay, err := pdfsrc.ParseXML(strings.NewReader(lostGlyphXML))
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}
	p := ReadPage(lay, lay.Pages[0])
	if p.Lost != 1 {
		t.Errorf("Lost = %d, want 1", p.Lost)
	}
	if !slices.Contains(p.Flags, FlagDroppedGlyph) {
		t.Errorf("Flags = %v, want one of them %s", p.Flags, FlagDroppedGlyph)
	}
	// The run itself is kept out of the reading. It has a box and no
	// characters, so it is a gap nothing can be read out of, and the words
	// around it are set as they were.
	if !strings.Contains(p.Body, "COROLLARY 1.") {
		t.Errorf("the page reads %q", p.Body)
	}
}
