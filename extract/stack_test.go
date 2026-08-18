package extract

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

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
// one into the subscript. restack must leave it alone, and what says so is the
// gap the page leaves between the two columns: the X ends at 384 and the 0 of
// the second column opens at 389, where the two halves of an inverse touch.
// What it is instead is a grid, which grid.go reads as the matrix it is.
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

func TestAFlattenedMatrixIsNotAStack(t *testing.T) {
	lines := parse(t, matrixPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `(A) to the class of the matrix $\begin{pmatrix} X & 0 \\ 0 & I \end{pmatrix}$. Prove that if the ring A is`
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
//
// The sign leads what is left, because hoist takes every script that clears the
// band and stands across a sign to be its limit and there is nothing on the
// line to say that two of these came off two different products. The levels
// themselves are what is left alone, and they are still written out interleaved.
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
	const want = `$\times \prod^{x\in}_x^H_{\in}^{\backslash}_H^G_{\backslash G}^{s(x)^{-1}}t(s(x)$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 319 sets the same product over the same two lines, and there the two
// limits stand at the same place across the page rather than offset. Every box
// of the one is a box of the other, which is what says the line gathering has
// run two lines together: put back, the limit would be written out twice and
// nothing would say why. Here too the sign leads what hoist read as its limit,
// and the levels are left interleaved.
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
	const want = `$\times \prod^x_x^{\in}_{\in}^H_H^{\backslash}_{\backslash}^G_G^{g_1s(x)^{-1}}c h(x)$`
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

// French page 399 sets a sum over G against a sum over the dual of G, and the
// bound of the first is written between the two signs. Walking left from the
// second sign reaches it before it reaches anything that would stop the walk,
// so what says to stop is that the bound stands eleven units from the first
// sign and forty one from the second, and a limit belongs to the sign it is
// centred on. The line is carried whole, since the sizes on it are what tell a
// bound from the body of the formula.
const doubleSumPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="399" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="2" size="14" family="LMRoman12" color="#131413"/>
<fontspec id="3" size="14" family="LMMathItalic12" color="#131413"/>
<fontspec id="4" size="14" family="LMMathSymbols10" color="#131413"/>
<fontspec id="5" size="12" family="CMEX10" color="#131413"/>
<fontspec id="6" size="9" family="LMRoman8" color="#131413"/>
<fontspec id="7" size="9" family="LMMathItalic8" color="#131413"/>
<fontspec id="8" size="9" family="LMMathSymbols8" color="#131413"/>
<text top="706" left="80" width="24" height="12" font="2">(13)</text>
<text top="706" left="200" width="7" height="12" font="3"><i>a</i></text>
<text top="706" left="211" width="11" height="12" font="2">=</text>
<text top="702" left="225" width="4" height="18" font="4"><i>|</i></text>
<text top="706" left="229" width="11" height="12" font="2">G</text>
<text top="702" left="240" width="4" height="18" font="4"><i>|</i></text>
<text top="701" left="244" width="8" height="12" font="8"><i>&#8722;</i></text>
<text top="703" left="251" width="5" height="8" font="6">1</text>
<text top="693" left="265" width="17" height="15" font="5">X</text>
<text top="723" left="264" width="5" height="8" font="7"><i>g</i></text>
<text top="721" left="269" width="7" height="12" font="8"><i>&#8712;</i></text>
<text top="723" left="275" width="8" height="8" font="6">G</text>
<text top="693" left="291" width="17" height="15" font="5">X</text>
<text top="726" left="290" width="6" height="8" font="7"><i>&#955;</i></text>
<text top="724" left="295" width="7" height="12" font="8"><i>&#8712;</i></text>
<text top="720" left="303" width="6" height="15" font="5">b</text>
<text top="726" left="302" width="8" height="8" font="6">G</text>
<text top="706" left="316" width="7" height="12" font="3"><i>d</i></text>
<text top="712" left="323" width="6" height="8" font="7"><i>&#955;</i></text>
<text top="706" left="336" width="19" height="12" font="2">Tr(</text>
<text top="706" left="356" width="8" height="12" font="3"><i>&#960;</i></text>
<text top="712" left="363" width="6" height="8" font="7"><i>&#955;</i></text>
<text top="706" left="370" width="5" height="12" font="2">(</text>
<text top="706" left="375" width="7" height="12" font="3"><i>a</i></text>
<text top="706" left="382" width="5" height="12" font="2">)</text>
<text top="706" left="387" width="8" height="12" font="3"><i>&#960;</i></text>
<text top="712" left="395" width="6" height="8" font="7"><i>&#955;</i></text>
<text top="706" left="401" width="5" height="12" font="2">(</text>
<text top="706" left="406" width="6" height="12" font="3"><i>g</i></text>
<text top="701" left="413" width="8" height="12" font="8"><i>&#8722;</i></text>
<text top="703" left="421" width="5" height="8" font="6">1</text>
<text top="706" left="426" width="10" height="12" font="2">))</text>
<text top="706" left="441" width="6" height="12" font="3"><i>g</i></text>
<text top="706" left="448" width="4" height="12" font="2">;</text>
</page>
</pdf2xml>
`

func TestEachSumOfADoubleSumKeepsItsOwnBound(t *testing.T) {
	lines := parse(t, doubleSumPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `(13) $a=|G|^{-1}\sum_{g\in G}\sum_{\lambda\in\widehat{G}}d_{\lambda}$ Tr($\pi_{\lambda}(a)\pi_{\lambda}(g^{-1}))g$;`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 18 of Topologie algébrique sets the fibre of a B-space as the inverse
// image of a point, and Bourbaki writes the inverse image with the -1 over the
// letter rather than after it. TeX draws the -1 first and the p across the
// middle of it, so the runs come back as minus, p, one, and the page said
// "$^-p^1(b)$". The minus is drawn from 131 to 141, the one from 141 to 147 and
// the p from 135 to 143, so every piece of the script is drawn across the
// letter, the two pieces touch, and the letter lies inside what they span.
const inverseImagePage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="18" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="16" family="PYGVNS+LMRoman10" color="#000000"/>
<fontspec id="3" size="16" family="QWXCLB+LMMathItalic10" color="#000000"/>
<fontspec id="8" size="12" family="BXXFTB+LMMathSymbols8" color="#000000"/>
<fontspec id="9" size="12" family="GTNDLC+LMRoman8" color="#000000"/>
<text top="278" left="81" width="46" height="21" font="2">espace</text>
<text top="274" left="131" width="10" height="11" font="8">&#8722;</text>
<text top="282" left="135" width="8" height="15" font="3"><i>p</i></text>
<text top="275" left="141" width="6" height="11" font="9">1</text>
<text top="278" left="147" width="6" height="21" font="2">(</text>
<text top="282" left="154" width="7" height="15" font="3"><i>b</i></text>
<text top="278" left="161" width="6" height="21" font="2">)</text>
<text top="278" left="172" width="16" height="21" font="2">de</text>
<text top="278" left="193" width="12" height="21" font="2">X</text>
<text top="278" left="210" width="88" height="21" font="2">est appelé la</text>
</page>
</pdf2xml>
`

func TestAnInverseImageIsPutBackOverItsLetter(t *testing.T) {
	lines := parse(t, inverseImagePage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `espace $\overset{-1}{p}(b)$ de X est appelé la`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// The same line with the -1 written where an exponent goes. The minus now
// begins where the p ends, so neither piece of the script is drawn across the
// letter, and an exponent that says what it means is left saying it.
const exponentPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="18" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="16" family="PYGVNS+LMRoman10" color="#000000"/>
<fontspec id="3" size="16" family="QWXCLB+LMMathItalic10" color="#000000"/>
<fontspec id="8" size="12" family="BXXFTB+LMMathSymbols8" color="#000000"/>
<fontspec id="9" size="12" family="GTNDLC+LMRoman8" color="#000000"/>
<text top="278" left="81" width="46" height="21" font="2">espace</text>
<text top="282" left="131" width="8" height="15" font="3"><i>p</i></text>
<text top="274" left="139" width="10" height="11" font="8">&#8722;</text>
<text top="275" left="149" width="6" height="11" font="9">1</text>
<text top="278" left="155" width="6" height="21" font="2">(</text>
<text top="282" left="162" width="7" height="15" font="3"><i>b</i></text>
<text top="278" left="169" width="6" height="21" font="2">)</text>
</page>
</pdf2xml>
`

func TestAnExponentAfterItsLetterIsLeftAlone(t *testing.T) {
	lines := parse(t, exponentPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `espace $p^{-1}(b)$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Two letters each carrying an exponent of its own, which is the shape that
// would be taken by a rule that asked only that the letter lie between two
// superscripts. The i ends at 139 and the j begins at 147, so the two stand
// eight units apart, which is a letter's width and not the nothing the halves
// of one script stand apart by.
const twoExponentsPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="18" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="3" size="16" family="QWXCLB+LMMathItalic10" color="#000000"/>
<fontspec id="9" size="12" family="GTNDLC+LMRoman8" color="#000000"/>
<text top="282" left="123" width="8" height="15" font="3"><i>a</i></text>
<text top="275" left="133" width="6" height="11" font="9">2</text>
<text top="282" left="139" width="8" height="15" font="3"><i>b</i></text>
<text top="275" left="147" width="6" height="11" font="9">3</text>
</page>
</pdf2xml>
`

func TestTwoLettersEachWithAnExponentAreLeftAlone(t *testing.T) {
	lines := parse(t, twoExponentsPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `$a^2b^3$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 379 of Topologie algébrique prints the inverse image of p sub n, and the
// -1 is drawn across the pair: the minus starts at 290 where the p starts, the
// one ends at 306 where the n ends, and the n sits at 298 to 306 squarely under
// the one. An index the script covers belongs inside the script with the letter.
const indexUnderPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="379" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="16" family="PYGVNS+LMRoman10" color="#000000"/>
<fontspec id="3" size="16" family="QWXCLB+LMMathItalic10" color="#000000"/>
<fontspec id="7" size="12" family="SQGJWG+LMMathItalic8" color="#000000"/>
<fontspec id="8" size="12" family="BXXFTB+LMMathSymbols8" color="#000000"/>
<fontspec id="9" size="12" family="GTNDLC+LMRoman8" color="#000000"/>
<text top="149" left="206" width="78" height="21" font="2">parcourant</text>
<text top="146" left="290" width="10" height="11" font="8">&#8722;</text>
<text top="154" left="290" width="8" height="15" font="3"><i>p</i></text>
<text top="159" left="298" width="8" height="11" font="7"><i>n</i></text>
<text top="147" left="300" width="6" height="11" font="9">1</text>
<text top="149" left="307" width="6" height="21" font="2">(</text>
<text top="154" left="313" width="7" height="15" font="3"><i>c</i></text>
<text top="149" left="320" width="6" height="21" font="2">)</text>
</page>
</pdf2xml>
`

func TestAnIndexTheScriptIsDrawnOverGoesUnderIt(t *testing.T) {
	lines := parse(t, indexUnderPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `parcourant $\overset{-1}{p_{n}}(c)$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 260 of the same volume prints the inverse image of phi sub 1, and the -1
// is drawn across the phi alone: the minus starts at 95 and the one ends at 111,
// where the phi ends at 109, and the index sits at 111 to 117, clear of the end
// of the script. An index the script stops short of goes after it, which is
// where the page drew it.
const indexAfterPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="260" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="16" family="PYGVNS+LMRoman10" color="#000000"/>
<fontspec id="3" size="16" family="QWXCLB+LMMathItalic10" color="#000000"/>
<fontspec id="8" size="12" family="BXXFTB+LMMathSymbols8" color="#000000"/>
<fontspec id="9" size="12" family="GTNDLC+LMRoman8" color="#000000"/>
<text top="653" left="81" width="8" height="21" font="2">a</text>
<text top="650" left="95" width="10" height="11" font="8">&#8722;</text>
<text top="657" left="98" width="11" height="15" font="3"><i>&#981;</i></text>
<text top="651" left="105" width="6" height="11" font="9">1</text>
<text top="665" left="111" width="6" height="11" font="9">1</text>
<text top="653" left="119" width="57" height="21" font="2">(0) = A</text>
</page>
</pdf2xml>
`

func TestAnIndexTheScriptStopsShortOfGoesAfterIt(t *testing.T) {
	lines := parse(t, indexAfterPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `a $\overset{-1}{\varphi}_{1}(0) = A$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// An inverse image written inside a subscript. Page 77 of Topologie algébrique
// sets the restriction map f from U to the inverse image of V as a subscript of
// f, so the u is at a level and the -1 written over it is at a level inside
// that: the minus spans 304 to 313, the u spans 307 to 314 and the one spans 312
// to 317. Nothing about that is different from the shape on the line except how
// far in it is, so the reading is the same reading one level out.
const nestedInverseImagePage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="77" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="16" family="BMXWVH+LMRoman10" color="#000000"/>
<fontspec id="3" size="16" family="QWXCLB+LMMathItalic10" color="#000000"/>
<fontspec id="4" size="16" family="BRGRHX+LMMathSymbols10" color="#000000"/>
<fontspec id="7" size="12" family="SQGJWG+LMMathItalic8" color="#000000"/>
<fontspec id="8" size="12" family="BXXFTB+LMMathSymbols8" color="#000000"/>
<fontspec id="9" size="12" family="GTNDLC+LMRoman8" color="#000000"/>
<fontspec id="10" size="9" family="GMKRRQ+LMMathSymbols6" color="#000000"/>
<fontspec id="11" size="9" family="NXVBWG+LMRoman6" color="#000000"/>
<text top="326" left="269" width="13" height="21" font="2">=</text>
<text top="330" left="286" width="8" height="15" font="3"><i>f</i></text>
<text top="343" left="294" width="10" height="11" font="9">U</text>
<text top="336" left="304" width="9" height="8" font="10">&#8722;</text>
<text top="343" left="307" width="7" height="11" font="7"><i>u</i></text>
<text top="337" left="312" width="5" height="8" font="11">1</text>
<text top="343" left="318" width="19" height="11" font="9">(V)</text>
<text top="326" left="338" width="6" height="21" font="2">(</text>
<text top="330" left="389" width="6" height="21" font="2">(</text>
<text top="330" left="395" width="6" height="15" font="3"><i>t</i></text>
<text top="329" left="405" width="8" height="15" font="4">&#8728;</text>
<text top="330" left="417" width="9" height="15" font="3"><i>u</i></text>
<text top="329" left="430" width="5" height="15" font="4">|</text>
<text top="322" left="438" width="10" height="11" font="8">&#8722;</text>
<text top="330" left="441" width="9" height="15" font="3"><i>u</i></text>
<text top="323" left="448" width="6" height="11" font="9">1</text>
<text top="326" left="454" width="38" height="21" font="2">(V)))</text>
</page>
</pdf2xml>
`

func TestAnInverseImageInsideASubscriptIsPutBackToo(t *testing.T) {
	lines := parse(t, nestedInverseImagePage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `$=f_{U\overset{-1}{u}(V)}((t∘u|\overset{-1}{u}(V)))$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// The same line of page 77 sets the same inverse image again a few characters
// along, this time straight after a psi rather than after an index. The two
// minuses measure the same to the unit, 336 to 344 down the page against a
// letter that runs 330 to 345, and the first came out a superscript and the
// second a subscript, because the level of a token is read against the token
// before it and a psi has a descender where an index has none. The near half of
// a script cut in two stands before the symbol it belongs to, so what is before
// it is never the thing it should be measured against.
const nestedAfterADescenderPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="77" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="16" family="BMXWVH+LMRoman10" color="#000000"/>
<fontspec id="3" size="16" family="QWXCLB+LMMathItalic10" color="#000000"/>
<fontspec id="4" size="16" family="BRGRHX+LMMathSymbols10" color="#000000"/>
<fontspec id="7" size="12" family="SQGJWG+LMMathItalic8" color="#000000"/>
<fontspec id="8" size="12" family="BXXFTB+LMMathSymbols8" color="#000000"/>
<fontspec id="9" size="12" family="GTNDLC+LMRoman8" color="#000000"/>
<fontspec id="10" size="9" family="GMKRRQ+LMMathSymbols6" color="#000000"/>
<fontspec id="11" size="9" family="NXVBWG+LMRoman6" color="#000000"/>
<text top="326" left="338" width="6" height="21" font="2">(</text>
<text top="330" left="344" width="11" height="15" font="3"><i>&#968;</i></text>
<text top="336" left="355" width="9" height="8" font="10">&#8722;</text>
<text top="343" left="358" width="7" height="11" font="7"><i>u</i></text>
<text top="337" left="364" width="5" height="8" font="11">1</text>
<text top="343" left="369" width="19" height="11" font="9">(V)</text>
<text top="326" left="389" width="6" height="21" font="2">(</text>
<text top="330" left="395" width="6" height="15" font="3"><i>t</i></text>
<text top="329" left="405" width="8" height="15" font="4">&#8728;</text>
<text top="330" left="417" width="9" height="15" font="3"><i>u</i></text>
<text top="329" left="430" width="5" height="15" font="4">|</text>
<text top="322" left="438" width="10" height="11" font="8">&#8722;</text>
<text top="330" left="441" width="9" height="15" font="3"><i>u</i></text>
<text top="323" left="448" width="6" height="11" font="9">1</text>
<text top="326" left="454" width="38" height="21" font="2">(V)))</text>
</page>
</pdf2xml>
`

func TestTheNearHalfOfAScriptIsReadByWhereItIsDrawnAndNotByItsLevel(t *testing.T) {
	lines := parse(t, nestedAfterADescenderPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `$(\psi_{\overset{-1}{u}(V)}(t∘u|\overset{-1}{u}(V)))$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 120 of Théories spectrales I and II writes the positive elements of A of
// norm under one as A with < 1 above and + below. The < of the exponent and the
// + of the index are both ten units wide and both start at 450, so the two
// boxes are the same box, and the cluster was refused for it: the volume
// shipped A^<_+^1 nine times over. The two scripts overlap by a unit, which is
// what TeX does with the two sides of one base.
const oneWidthStackPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="133" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="16" family="SNSRNN+LMRoman10" color="#000000"/>
<fontspec id="3" size="12" family="WKQCXM+LMMathItalic8" color="#000000"/>
<fontspec id="4" size="12" family="XKDTCH+LMRoman8" color="#000000"/>
<text top="502" left="393" width="40" height="14" font="2">(resp.</text>
<text top="502" left="438" width="12" height="14" font="2">A</text>
<text top="499" left="450" width="10" height="11" font="3"><i>&#60;</i></text>
<text top="509" left="450" width="10" height="11" font="4">+</text>
<text top="499" left="460" width="6" height="11" font="4">1</text>
<text top="502" left="467" width="111" height="14" font="2">) l&#8217;ensemble des</text>
</page>
</pdf2xml>
`

func TestTwoScriptsOfOneWidthAreStillAStack(t *testing.T) {
	lines := parse(t, oneWidthStackPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `(resp. $A^{<1}_+)$ l’ensemble des`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 20 of the same volume sets the spectral radius as the limit of the norm
// of x to the n, raised to one over n, in a norm that carries an index of its
// own. The 1 of the exponent and the 1 of the index are the same glyph at the
// same place, two units of clear space apart, and the line shipped as
// \|x^n\|^1_1^{/n}.
const normIndexPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="33" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="16" family="VXCFTV+LMRoman10" color="#000000"/>
<fontspec id="3" size="16" family="QNNXQJ+LMMathItalic10" color="#000000"/>
<fontspec id="4" size="16" family="WSBXWF+LMMathSymbols10" color="#000000"/>
<fontspec id="5" size="12" family="WKQCXM+LMMathItalic8" color="#000000"/>
<fontspec id="6" size="12" family="XKDTCH+LMRoman8" color="#000000"/>
<text top="805" left="264" width="23" height="14" font="2">lim</text>
<text top="804" left="297" width="8" height="15" font="4"><i>k</i></text>
<text top="805" left="305" width="9" height="15" font="3"><i>x</i></text>
<text top="802" left="315" width="8" height="11" font="5"><i>n</i></text>
<text top="804" left="323" width="8" height="15" font="4"><i>k</i></text>
<text top="800" left="331" width="6" height="11" font="6">1</text>
<text top="813" left="331" width="6" height="11" font="6">1</text>
<text top="800" left="338" width="14" height="11" font="5"><i>/n</i></text>
</page>
</pdf2xml>
`

func TestAnExponentAndAnIndexOfOneGlyphAreStillAStack(t *testing.T) {
	lines := parse(t, normIndexPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `lim $\|x^n\|^{1/n}_1$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 493 of Topologie algébrique I to IV writes x indexed by s to the minus
// one sub n, times s sub one. The index is a stack in its own right, three
// levels deep in all, and a stack inside a stack was only ever put back at the
// outer level: the line shipped as x_{s^-_n^1s_1}. What puts it right is asking
// the same question of what stands inside a unit that was asked of the cluster,
// which is what nested does before the units of the outer level are cut.
const nestedStackPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="493" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="3" size="15" family="IWFIMB+LMRoman10" color="#000000"/>
<fontspec id="4" size="15" family="AQKDGP+LMMathItalic10" color="#000000"/>
<fontspec id="5" size="15" family="IWFIMB+LMRoman10" color="#000000"/>
<fontspec id="7" size="10" family="UNPVBI+LMMathItalic7" color="#000000"/>
<fontspec id="16" size="7" family="LZFOFQ+LMMathSymbols5" color="#000000"/>
<fontspec id="17" size="7" family="HPCJLQ+LMRoman5" color="#000000"/>
<fontspec id="18" size="7" family="JDOPVZ+LMMathItalic5" color="#000000"/>
<text top="612" left="246" width="37" height="19" font="3">, on a</text>
<text top="616" left="289" width="7" height="13" font="4"><i>g</i></text>
<text top="612" left="296" width="6" height="19" font="5">(</text>
<text top="616" left="302" width="6" height="13" font="4"><i>c</i></text>
<text top="612" left="309" width="22" height="19" font="5">) =</text>
<text top="616" left="334" width="9" height="13" font="4"><i>x</i></text>
<text top="624" left="343" width="6" height="9" font="7"><i>s</i></text>
<text top="620" left="349" width="8" height="7" font="16">&#8722;</text>
<text top="621" left="357" width="5" height="7" font="17">1</text>
<text top="628" left="349" width="7" height="7" font="18"><i>n</i></text>
<text top="624" left="363" width="6" height="9" font="7"><i>s</i></text>
<text top="627" left="368" width="5" height="7" font="17">1</text>
</page>
</pdf2xml>
`

func TestAStackInsideAStackIsPutBackToo(t *testing.T) {
	lines := parse(t, nestedStackPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `, on a $g(c) =x_{s^{-1}_ns_1}$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 488 of the same volume writes x with w sub i above and s sub i,2 below.
// The i of the exponent is drawn after the exponent and before the index, and
// the order alone hangs it on the index, which is how the line shipped as
// x^w_{s_{i,}^i_2}. The two scripts are set at the same left edge, 230 both, and
// there the order says nothing about which of them a run to the right was
// written inside; the i sits 4 units from the middle of the w and 14 from the
// middle of the s, so it is the exponent's. Nothing else on the line moves,
// since the i, and the 2 are nearer the s than the w.
const scriptOfTheFarScriptPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="488" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="15" family="IWFIMB+LMRoman10" color="#000000"/>
<fontspec id="3" size="15" family="AQKDGP+LMMathItalic10" color="#000000"/>
<fontspec id="5" size="15" family="IWFIMB+LMRoman10" color="#000000"/>
<fontspec id="6" size="10" family="UNPVBI+LMMathItalic7" color="#000000"/>
<fontspec id="13" size="7" family="JDOPVZ+LMMathItalic5" color="#000000"/>
<fontspec id="14" size="7" family="HPCJLQ+LMRoman5" color="#000000"/>
<text top="640" left="206" width="9" height="13" font="3"><i>&#181;</i></text>
<text top="636" left="215" width="6" height="19" font="5">(</text>
<text top="640" left="221" width="9" height="13" font="3"><i>x</i></text>
<text top="638" left="230" width="9" height="9" font="6"><i>w</i></text>
<text top="641" left="238" width="4" height="7" font="13"><i>i</i></text>
<text top="647" left="230" width="6" height="9" font="6"><i>s</i></text>
<text top="651" left="235" width="7" height="7" font="13"><i>i,</i></text>
<text top="651" left="243" width="5" height="7" font="14">2</text>
<text top="636" left="249" width="6" height="19" font="5">)</text>
<text top="636" left="260" width="12" height="19" font="2">et</text>
</page>
</pdf2xml>
`

func TestAScriptOfTheFarScriptIsReadByHowFarApartTheyAreSet(t *testing.T) {
	lines := parse(t, scriptOfTheFarScriptPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	// The µ is the micro sign the volume sets and mathtex is what turns it into
	// \mu later, so it stands here as the page draws it.
	const want = `$µ(x^{w_i}_{s_{i,2}})$ et`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// The two scripts of a slanted base are not set at one left edge. TeX moves the
// exponent out by the italic correction of the glyph the scripts hang off, so
// the index of the index of a base is drawn after both of them and the order
// says nothing about which of the two it was written inside, exactly as it says
// nothing where the two edges agree to the unit.
//
// This is page 223 of Topologie algébrique, which prints the inverse of delta
// sub phi nought of j sub k. The delta carries phi at 329 and its exponent at
// 330, one unit apart, and the nought at 338 went to the exponent: the page
// said \delta_{\varphi}^{-_01}_{(j_k)}, which KaTeX refuses by name. The nought
// is set 4 units from the middle of the phi and 12 from the middle of the
// minus, so the height answers where the order cannot.
func TestTheTwoScriptsOfASlantedBaseArePartners(t *testing.T) {
	p := pdfsrc.Page{
		Number: 223, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 682, Left: 290, Width: 7, Height: 15, Font: 1, Text: "j"},
			{Top: 688, Left: 296, Width: 7, Height: 11, Font: 4, Text: "k"},
			{Top: 678, Left: 304, Width: 6, Height: 21, Text: ")"},
			{Top: 681, Left: 314, Width: 5, Height: 15, Font: 2, Text: "·"},
			{Top: 682, Left: 322, Width: 7, Height: 15, Font: 1, Text: "δ"},
			{Top: 691, Left: 329, Width: 8, Height: 11, Font: 4, Text: "ϕ"},
			{Top: 678, Left: 330, Width: 10, Height: 11, Font: 13, Text: "−"},
			{Top: 695, Left: 338, Width: 5, Height: 8, Font: 14, Text: "0"},
			{Top: 679, Left: 340, Width: 6, Height: 11, Font: 3, Text: "1"},
			{Top: 691, Left: 344, Width: 5, Height: 11, Font: 3, Text: "("},
			{Top: 691, Left: 349, Width: 5, Height: 11, Font: 4, Text: "j"},
			{Top: 695, Left: 354, Width: 6, Height: 8, Font: 15, Text: "k"},
			{Top: 691, Left: 361, Width: 5, Height: 11, Font: 3, Text: ")"},
			{Top: 681, Left: 370, Width: 5, Height: 15, Font: 2, Text: "·"},
		},
	}
	const want = `$j_k)\cdot \delta_{\varphi_0(j_k)}^{-1}\cdot$`
	if got := sole(t, frlay, p); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}
