package extract

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
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

// The pages below are given as boxes rather than as a document, in the style of
// bar_test.go, and they use the layouts declared there. The glyph at 0x30 of a
// symbol font is the prime and not the digit, so a page hands a prime back as a
// zero here too.

// A prime and the index of the base it marks start at the same place, so the
// layer hands the index back first and the prime lands inside it. This is page
// 114 of Lie 7 to 9, which prints the bracket of X prime alpha against X prime
// minus alpha. The second of those came back as a base carrying two subscripts,
// which is not TeX at all, and the first set the prime on the far side of the
// index from where the volume prints it.
func TestAPrimeDrawnOverAnIndexGoesInFrontOfIt(t *testing.T) {
	p := pdfsrc.Page{
		Number: 114, Width: 612, Height: 792,
		Spans: []pdfsrc.Span{
			{Top: 292, Left: 172, Width: 4, Height: 13, Font: 3, Text: "["},
			{Top: 292, Left: 177, Width: 12, Height: 13, Font: 4, Text: "X"},
			{Top: 299, Left: 189, Width: 8, Height: 9, Font: 6, Text: "α"},
			{Top: 286, Left: 190, Width: 3, Height: 14, Font: 7, Text: "0"},
			{Top: 292, Left: 198, Width: 19, Height: 13, Font: 4, Text: ", X"},
			{Top: 296, Left: 217, Width: 9, Height: 14, Font: 7, Text: "−"},
			{Top: 286, Left: 218, Width: 3, Height: 14, Font: 7, Text: "0"},
			{Top: 299, Left: 226, Width: 8, Height: 9, Font: 6, Text: "α"},
			{Top: 292, Left: 234, Width: 28, Height: 13, Font: 3, Text: "] = ["},
		},
	}
	want := `$[X'_{\alpha}, X'_{-\alpha}] = [$`
	if got := sole(t, enlay, p); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// A prime the author wrote inside an index is set after the box of the letter
// it marks and stands clear of that letter, so it is left where it is. This is
// page 246 of Algebra VIII, where the L of A_(L') carries the prime and the
// bracket of the index closes after the prime rather than before it.
func TestAPrimeClearOfAnIndexStaysInside(t *testing.T) {
	p := pdfsrc.Page{
		Number: 246, Width: 612, Height: 792,
		Spans: []pdfsrc.Span{
			{Top: 777, Left: 80, Width: 163, Height: 13, Text: "extension of L. As an A"},
			{Top: 784, Left: 243, Width: 12, Height: 9, Font: 2, Text: "(L"},
			{Top: 783, Left: 255, Width: 3, Height: 7, Font: 10, Text: "0"},
			{Top: 784, Left: 259, Width: 5, Height: 9, Font: 2, Text: ")"},
		},
	}
	want := `extension of L. As an $A_{(L')}$`
	if got := sole(t, enlay, p); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// The prime of an inverse image is drawn between the two halves of the -1 that
// is written over the symbol, so the layer hands back the minus, the symbol,
// the one and the prime in that order. Reading the prime before the halves are
// put back together takes the inverse image apart, which is why the prime is
// read after it and not before. This is page 37 of Topologie algébrique 1 to 4.
func TestAPrimeInsideAnInverseImageLeavesItStanding(t *testing.T) {
	p := pdfsrc.Page{
		Number: 37, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 726, Left: 81, Width: 29, Height: 21, Text: "(20)"},
			{Top: 730, Left: 247, Width: 8, Height: 15, Font: 1, Text: "p"},
			{Top: 725, Left: 255, Width: 3, Height: 11, Font: 13, Text: "0"},
			{Top: 726, Left: 259, Width: 6, Height: 21, Text: "("},
			{Top: 726, Left: 266, Width: 6, Height: 21, Text: "("},
			{Top: 716, Left: 271, Width: 10, Height: 11, Font: 13, Text: "−"},
			{Top: 730, Left: 272, Width: 8, Height: 15, Font: 1, Text: "f"},
			{Top: 716, Left: 281, Width: 6, Height: 11, Font: 3, Text: "1"},
			{Top: 725, Left: 282, Width: 3, Height: 11, Font: 13, Text: "0"},
			{Top: 726, Left: 286, Width: 55, Height: 21, Text: ")(A)) ="},
			{Top: 718, Left: 346, Width: 10, Height: 11, Font: 13, Text: "−"},
			{Top: 730, Left: 349, Width: 8, Height: 15, Font: 1, Text: "f"},
			{Top: 718, Left: 356, Width: 6, Height: 11, Font: 3, Text: "1"},
			{Top: 726, Left: 362, Width: 6, Height: 21, Text: "("},
			{Top: 730, Left: 368, Width: 8, Height: 15, Font: 1, Text: "p"},
			{Top: 726, Left: 376, Width: 31, Height: 21, Text: "(A))"},
			{Top: 730, Left: 408, Width: 5, Height: 15, Font: 1, Text: "."},
		},
	}
	want := `(20) $p'((\overset{-1}{f}')(A)) =\overset{-1}{f}(p(A))$.`
	if got := sole(t, frlay, p); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// A prime deep inside a cluster goes to the letter it was drawn against.
//
// TeX runs out of script sizes at the second level, so a script of a script and
// a script of the other script of the same base come back at one size and so at
// one depth, and the depth is all that says what a run hangs off. Page 45 of
// Théories spectrales prints the script L with p' above it and L^{p'}(X) below,
// and the prime of the exponent inside the index went to the exponent above:
// the page said \mathscr{L}_L^{p'_{p'}}_{(X)}, an index in two pieces with the
// (X) written as a second index of the base, which KaTeX refuses by name.
//
// The page says which letter each prime marks by where it drew it, against the
// right edge of the letter and above the middle of it. The first prime is set
// at 505 where the p above ends at 504, the second at 509 where the p below
// ends at 509, and each is 3 units above the letter it is drawn against.
func TestAPrimeInsideAClusterMarksTheLetterItWasDrawnAgainst(t *testing.T) {
	p := pdfsrc.Page{
		Number: 45, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 674, Left: 383, Width: 91, Height: 14, Text: "appartient à"},
			{Top: 670, Left: 481, Width: 14, Height: 21, Font: 6, Text: "L"},
			{Top: 685, Left: 495, Width: 8, Height: 11, Font: 3, Text: "L"},
			{Top: 669, Left: 498, Width: 6, Height: 11, Font: 4, Text: "p"},
			{Top: 683, Left: 503, Width: 6, Height: 8, Font: 15, Text: "p"},
			{Top: 666, Left: 505, Width: 3, Height: 8, Font: 16, Text: "′"},
			{Top: 680, Left: 509, Width: 3, Height: 8, Font: 16, Text: "′"},
			{Top: 685, Left: 514, Width: 19, Height: 11, Font: 3, Text: "(X)"},
			{Top: 674, Left: 534, Width: 19, Height: 14, Text: "(Y"},
		},
	}
	const want = `appartient à $\mathscr{L}_{L^{p'}(X)}^{p'}(Y$`
	if got := sole(t, frlay, p); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}
