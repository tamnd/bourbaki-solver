package extract

import "testing"

// Every fixture here is a line of Algebra VIII as pdftohtml reports it, cut to
// the runs the line needs and no further. The boxes are the volume's own, since
// what this file decides is decided on the boxes and a made up one would only
// prove that the rule agrees with whoever made it up.

// Page 481 sets the inverse of theta sub E. The minus of the -1 starts a hair
// to the left of the E and the one ends a hair to the right of it, so the three
// runs come back as minus, E, one.
const inversePage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="481" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="3" size="15" family="XJSZDJ+LMMathItalic10" color="#000000"/>
<fontspec id="8" size="10" family="GORNHQ+LMMathSymbols7" color="#000000"/>
<fontspec id="9" size="10" family="DCOIDG+LMRoman7" color="#000000"/>
<text top="578" left="203" width="260" height="13" font="2">By composing it with the isomorphism</text>
<text top="578" left="468" width="7" height="13" font="3"><i>&#952;</i></text>
<text top="574" left="475" width="9" height="10" font="8">&#8722;</text>
<text top="575" left="484" width="6" height="9" font="9">1</text>
<text top="586" left="475" width="8" height="9" font="9">E</text>
<text top="578" left="491" width="86" height="13" font="2">, we deduce a</text>
</page>
</pdf2xml>
`

func TestAnInverseIsPutBackOverItsIndex(t *testing.T) {
	lines := parse(t, inversePage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `By composing it with the isomorphism $\theta^{-1}_E$, we deduce a`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 352 sets the sum from r = 0 to m. The upper limit is one letter and it
// stands between the r and the = 0 of the lower one, so the lower limit is the
// one that comes back in two pieces.
const sumPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="352" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="10" size="15" family="XJSZDJ+LMMathItalic10" color="#000000"/>
<fontspec id="12" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="13" size="15" family="XHUWIC+LMMathSymbols10" color="#000000"/>
<fontspec id="14" size="10" family="DCOIDG+LMRoman7" color="#000000"/>
<fontspec id="15" size="10" family="CWSGRY+LMMathItalic7" color="#000000"/>
<fontspec id="16" size="15" family="XAEWAV+CMEX10" color="#000000"/>
<text top="872" left="229" width="9" height="13" font="10"><i>&#967;</i></text>
<text top="879" left="239" width="7" height="9" font="15"><i>u</i></text>
<text top="872" left="247" width="39" height="13" font="12">(X) =</text>
<text top="856" left="295" width="11" height="9" font="15"><i>m</i></text>
<text top="867" left="289" width="22" height="7" font="16">X</text>
<text top="892" left="290" width="6" height="9" font="15"><i>r</i></text>
<text top="892" left="296" width="15" height="9" font="14">=0</text>
<text top="872" left="311" width="6" height="13" font="12">(</text>
<text top="871" left="317" width="12" height="14" font="13">&#8722;</text>
<text top="872" left="328" width="13" height="13" font="12">1)</text>
<text top="869" left="342" width="6" height="9" font="15"><i>r</i></text>
</page>
</pdf2xml>
`

func TestTheLimitsOfASumArePutBackUnderIt(t *testing.T) {
	lines := parse(t, sumPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `$\chi_u(X) =\sum_{r=0}^m(-1)^r$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 140 is where the reordering was first seen to cost something. The
// subscript s of A_s^{(a)} is written under the opening bracket, so once the
// stack is put back the run the line ends on is the one that starts furthest
// left, and the comma after it read a word away.
const commaPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="140" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="10" size="10" family="CWSGRY+LMMathItalic7" color="#000000"/>
<fontspec id="11" size="10" family="DCOIDG+LMRoman7" color="#000000"/>
<fontspec id="14" size="10" family="XANRJU+EUFM7" color="#000000"/>
<text top="666" left="80" width="112" height="13" font="2">same length as A</text>
<text top="662" left="193" width="5" height="9" font="11">(</text>
<text top="662" left="197" width="6" height="9" font="14">a</text>
<text top="662" left="204" width="5" height="9" font="11">)</text>
<text top="671" left="193" width="6" height="9" font="10"><i>s</i></text>
<text top="666" left="209" width="270" height="13" font="2">, hence is isomorphic to it by Proposition</text>
</page>
</pdf2xml>
`

func TestAClusterKeepsItsReachAfterItIsPutBack(t *testing.T) {
	lines := parse(t, commaPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `same length as $A^{(\mathfrak{a})}_s$, hence is isomorphic to it by Proposition`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 226 prints a two by two matrix inline and pdftohtml flattens it the same
// way it flattens a stack: X, 0, 0, I, one row read into the superscript and
// one into the subscript. It must be left alone, and what says so is the gap
// the page leaves between the two columns: the X ends at 384 and the 0 of the
// second column opens at 389, where the two halves of an inverse touch.
const matrixPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="226" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="3" size="13" family="FBFVYY+LMRoman9" color="#000000"/>
<fontspec id="8" size="9" family="WUCILW+LMRoman6" color="#000000"/>
<fontspec id="17" size="9" family="NGQYRZ+LMRoman7" color="#000000"/>
<text top="656" left="179" width="195" height="12" font="3">(A) to the class of the matrix (</text>
<text top="655" left="376" width="8" height="8" font="17"><i>X</i></text>
<text top="655" left="389" width="5" height="8" font="8">0</text>
<text top="663" left="378" width="5" height="8" font="8">0</text>
<text top="663" left="389" width="4" height="8" font="17"><i>I</i></text>
<text top="656" left="397" width="181" height="12" font="3">). Prove that if the ring A is</text>
</page>
</pdf2xml>
`

func TestAFlattenedMatrixIsLeftAlone(t *testing.T) {
	lines := parse(t, matrixPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `(A) to the class of the matrix $(^X_0^0_I)$. Prove that if the ring A is`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 318 sets a product over x in H\G across two lines of one display, and
// the line gathering hands the lower limit of the first line to the second,
// where it is read as a superscript. The two limits touch and they interleave,
// so only the third question refuses them: they stand thirteen units apart,
// which is the offset between the two products, and neither lies inside what
// the other spans.
const stolenLimitPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="318" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="3" size="15" family="XJSZDJ+LMMathItalic10" color="#000000"/>
<fontspec id="4" size="15" family="XHUWIC+LMMathSymbols10" color="#000000"/>
<fontspec id="5" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="6" size="15" family="XAEWAV+CMEX10" color="#000000"/>
<fontspec id="7" size="10" family="CWSGRY+LMMathItalic7" color="#000000"/>
<fontspec id="8" size="10" family="DCOIDG+LMRoman7" color="#000000"/>
<fontspec id="9" size="10" family="GORNHQ+LMMathSymbols7" color="#000000"/>
<fontspec id="10" size="7" family="ODOCWD+LMMathSymbols5" color="#000000"/>
<fontspec id="11" size="7" family="GFWRXJ+LMRoman5" color="#000000"/>
<text top="152" left="168" width="7" height="9" font="7"><i>x</i></text>
<text top="151" left="175" width="8" height="10" font="9">&#8712;</text>
<text top="152" left="183" width="9" height="9" font="8">H</text>
<text top="151" left="192" width="6" height="10" font="9">\</text>
<text top="152" left="198" width="9" height="9" font="8">G</text>
<text top="174" left="167" width="12" height="14" font="4">&#215;</text>
<text top="171" left="191" width="19" height="7" font="6">Y</text>
<text top="197" left="181" width="7" height="9" font="7"><i>x</i></text>
<text top="196" left="188" width="8" height="10" font="9">&#8712;</text>
<text top="197" left="196" width="9" height="9" font="8">H</text>
<text top="196" left="205" width="6" height="10" font="9">\</text>
<text top="197" left="211" width="9" height="9" font="8">G</text>
<text top="172" left="223" width="6" height="9" font="7"><i>s</i></text>
<text top="172" left="229" width="5" height="9" font="8">(</text>
<text top="172" left="233" width="7" height="9" font="7"><i>x</i></text>
<text top="172" left="240" width="5" height="9" font="8">)</text>
<text top="169" left="245" width="8" height="7" font="10">&#8722;</text>
<text top="170" left="253" width="5" height="7" font="11">1</text>
<text top="175" left="266" width="5" height="13" font="3"><i>t</i></text>
<text top="175" left="272" width="6" height="13" font="5">(</text>
<text top="175" left="277" width="7" height="13" font="3"><i>s</i></text>
<text top="175" left="284" width="6" height="13" font="5">(</text>
<text top="175" left="290" width="9" height="13" font="3"><i>x</i></text>
<text top="175" left="299" width="6" height="13" font="5">)</text>
</page>
</pdf2xml>
`

func TestLimitsOffsetFromEachOtherAreLeftAlone(t *testing.T) {
	lines := parse(t, stolenLimitPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `$\times^{x\in}_x^H_{\in}\prod^{\backslash}_H^G_{\backslash G}^{s(x)^{-1}}t(s(x)$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 319 sets the same product over the same two lines, and there the two
// limits stand at the same place across the page rather than offset. Every box
// of the one is a box of the other, which is what says the line gathering has
// run two lines together: put back, the limit would be written out twice and
// nothing would say why.
const doubledLimitPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="319" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="3" size="15" family="XAEWAV+CMEX10" color="#000000"/>
<fontspec id="4" size="10" family="CWSGRY+LMMathItalic7" color="#000000"/>
<fontspec id="5" size="10" family="GORNHQ+LMMathSymbols7" color="#000000"/>
<fontspec id="6" size="10" family="DCOIDG+LMRoman7" color="#000000"/>
<fontspec id="7" size="7" family="ODOCWD+LMMathSymbols5" color="#000000"/>
<fontspec id="8" size="7" family="GFWRXJ+LMRoman5" color="#000000"/>
<fontspec id="9" size="15" family="XJSZDJ+LMMathItalic10" color="#000000"/>
<fontspec id="10" size="15" family="XHUWIC+LMMathSymbols10" color="#000000"/>
<text top="155" left="151" width="7" height="9" font="4"><i>x</i></text>
<text top="155" left="157" width="8" height="10" font="5">&#8712;</text>
<text top="155" left="165" width="9" height="9" font="6">H</text>
<text top="155" left="174" width="6" height="10" font="5">\</text>
<text top="155" left="180" width="9" height="9" font="6">G</text>
<text top="178" left="136" width="12" height="14" font="10">&#215;</text>
<text top="174" left="161" width="19" height="7" font="3">Y</text>
<text top="200" left="151" width="7" height="9" font="4"><i>x</i></text>
<text top="200" left="157" width="8" height="10" font="5">&#8712;</text>
<text top="200" left="165" width="9" height="9" font="6">H</text>
<text top="200" left="174" width="6" height="10" font="5">\</text>
<text top="200" left="180" width="9" height="9" font="6">G</text>
<text top="176" left="192" width="6" height="9" font="4"><i>g</i></text>
<text top="179" left="198" width="5" height="7" font="8">1</text>
<text top="176" left="204" width="6" height="9" font="4"><i>s</i></text>
<text top="176" left="209" width="5" height="9" font="6">(</text>
<text top="176" left="214" width="7" height="9" font="4"><i>x</i></text>
<text top="176" left="221" width="5" height="9" font="6">)</text>
<text top="173" left="225" width="8" height="7" font="7">&#8722;</text>
<text top="173" left="234" width="5" height="7" font="8">1</text>
<text top="179" left="240" width="22" height="13" font="9"><i>c h</i></text>
<text top="179" left="262" width="6" height="13" font="2">(</text>
<text top="179" left="268" width="9" height="13" font="9"><i>x</i></text>
<text top="179" left="276" width="6" height="13" font="2">)</text>
</page>
</pdf2xml>
`

func TestALimitAtTheSamePlaceAtBothLevelsIsLeftAlone(t *testing.T) {
	lines := parse(t, doubledLimitPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `$\times^x_x^{\in}_{\in}\prod^H_H^{\backslash}_{\backslash}^G_G^{g_1s(x)^{-1}}c h(x)$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// A base with one of each is what a formula looks like, and nothing is done to
// it. The boxes are page 481's, with the minus taken away.
const plainScriptPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="481" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="3" size="15" family="XJSZDJ+LMMathItalic10" color="#000000"/>
<fontspec id="9" size="10" family="DCOIDG+LMRoman7" color="#000000"/>
<text top="578" left="468" width="7" height="13" font="3"><i>&#952;</i></text>
<text top="575" left="475" width="6" height="9" font="9">1</text>
<text top="586" left="475" width="8" height="9" font="9">E</text>
<text top="578" left="491" width="86" height="13" font="2">, we deduce a</text>
</page>
</pdf2xml>
`

func TestOneOfEachScriptIsLeftAlone(t *testing.T) {
	lines := parse(t, plainScriptPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `$\theta^1_E$, we deduce a`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}
