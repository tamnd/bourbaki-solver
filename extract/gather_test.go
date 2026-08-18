package extract

import "testing"

// Every fixture here is three lines of Algebra VIII as pdftohtml reports them,
// whole, at the boxes the volume prints them at. What is at stake is which of
// two neighbours a stray line belongs to, so both neighbours are carried in
// full: the sizes on a line are what tell an index from the body of it, and a
// line cut down to a few runs is set in a size it is not set in.

// Page 320 sets the product over x in H\G on line after line. Each product
// writes its limit under its sign, so the limit falls between the line it
// belongs to and the line below, a line's width from either, and nothing about
// where it sits refuses either line. The sign of the first line stands at 97 and
// the sign of the second at 114, and the limit between them runs from 87 to 125,
// which is centred on the first and not on the second.
const stackedProductsPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="320" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="3" size="15" family="XAEWAV+CMEX10" color="#000000"/>
<fontspec id="4" size="10" family="CWSGRY+LMMathItalic7" color="#000000"/>
<fontspec id="5" size="10" family="GORNHQ+LMMathSymbols7" color="#000000"/>
<fontspec id="6" size="10" family="DCOIDG+LMRoman7" color="#000000"/>
<fontspec id="7" size="7" family="ODOCWD+LMMathSymbols5" color="#000000"/>
<fontspec id="8" size="7" family="GFWRXJ+LMRoman5" color="#000000"/>
<fontspec id="9" size="15" family="XJSZDJ+LMMathItalic10" color="#000000"/>
<fontspec id="10" size="15" family="XHUWIC+LMMathSymbols10" color="#000000"/>
<text top="631" left="97" width="19" height="7" font="3">Y</text>
<text top="657" left="87" width="7" height="9" font="4"><i>x</i></text>
<text top="657" left="93" width="8" height="10" font="5">&#8712;</text>
<text top="657" left="101" width="9" height="9" font="6">H</text>
<text top="657" left="110" width="6" height="10" font="5">\</text>
<text top="657" left="116" width="9" height="9" font="6">G</text>
<text top="633" left="128" width="6" height="9" font="4"><i>s</i></text>
<text top="633" left="134" width="5" height="9" font="6">(</text>
<text top="633" left="138" width="7" height="9" font="4"><i>x</i></text>
<text top="633" left="145" width="5" height="9" font="6">)</text>
<text top="630" left="150" width="8" height="7" font="7">&#8722;</text>
<text top="630" left="158" width="5" height="7" font="8">1</text>
<text top="636" left="164" width="6" height="13" font="9"><i>c</i></text>
<text top="636" left="171" width="6" height="13" font="2">(</text>
<text top="636" left="177" width="7" height="13" font="9"><i>s</i></text>
<text top="636" left="184" width="6" height="13" font="2">(</text>
<text top="636" left="190" width="9" height="13" font="9"><i>x</i></text>
<text top="636" left="198" width="6" height="13" font="2">)</text>
<text top="636" left="204" width="7" height="13" font="9"><i>g</i></text>
<text top="643" left="211" width="6" height="9" font="6">1</text>
<text top="636" left="218" width="7" height="13" font="9"><i>s</i></text>
<text top="636" left="225" width="6" height="13" font="2">(</text>
<text top="636" left="231" width="9" height="13" font="9"><i>x</i></text>
<text top="635" left="242" width="4" height="14" font="10">&#183;</text>
<text top="636" left="250" width="7" height="13" font="9"><i>g</i></text>
<text top="643" left="257" width="6" height="9" font="6">1</text>
<text top="636" left="264" width="6" height="13" font="2">)</text>
<text top="632" left="270" width="9" height="10" font="5">&#8722;</text>
<text top="633" left="279" width="6" height="9" font="6">1</text>
<text top="636" left="286" width="14" height="13" font="9"><i>, s</i></text>
<text top="636" left="299" width="6" height="13" font="2">(</text>
<text top="636" left="305" width="9" height="13" font="9"><i>x</i></text>
<text top="635" left="317" width="4" height="14" font="10">&#183;</text>
<text top="636" left="324" width="7" height="13" font="9"><i>g</i></text>
<text top="643" left="332" width="6" height="9" font="6">1</text>
<text top="636" left="338" width="6" height="13" font="2">)</text>
<text top="636" left="344" width="7" height="13" font="9"><i>g</i></text>
<text top="643" left="351" width="6" height="9" font="6">2</text>
<text top="636" left="358" width="7" height="13" font="9"><i>s</i></text>
<text top="636" left="365" width="6" height="13" font="2">(</text>
<text top="636" left="371" width="9" height="13" font="9"><i>x</i></text>
<text top="635" left="383" width="4" height="14" font="10">&#183;</text>
<text top="636" left="390" width="7" height="13" font="9"><i>g</i></text>
<text top="643" left="397" width="6" height="9" font="6">1</text>
<text top="636" left="404" width="7" height="13" font="9"><i>g</i></text>
<text top="643" left="411" width="6" height="9" font="6">2</text>
<text top="636" left="418" width="6" height="13" font="2">)</text>
<text top="632" left="424" width="9" height="10" font="5">&#8722;</text>
<text top="633" left="433" width="6" height="9" font="6">1</text>
<text top="636" left="440" width="6" height="13" font="2">)</text>
<text top="681" left="88" width="12" height="13" font="2">=</text>
<text top="677" left="114" width="19" height="7" font="3">Y</text>
<text top="703" left="104" width="7" height="9" font="4"><i>x</i></text>
<text top="702" left="111" width="8" height="10" font="5">&#8712;</text>
<text top="703" left="119" width="9" height="9" font="6">H</text>
<text top="702" left="128" width="6" height="10" font="5">\</text>
<text top="703" left="134" width="9" height="9" font="6">G</text>
<text top="681" left="145" width="6" height="13" font="9"><i>c</i></text>
<text top="681" left="152" width="6" height="13" font="2">(</text>
<text top="681" left="158" width="7" height="13" font="9"><i>g</i></text>
<text top="688" left="165" width="6" height="9" font="6">1</text>
<text top="681" left="172" width="7" height="13" font="9"><i>s</i></text>
<text top="681" left="179" width="6" height="13" font="2">(</text>
<text top="681" left="184" width="9" height="13" font="9"><i>x</i></text>
<text top="680" left="196" width="4" height="14" font="10">&#183;</text>
<text top="681" left="204" width="7" height="13" font="9"><i>g</i></text>
<text top="688" left="211" width="6" height="9" font="6">1</text>
<text top="681" left="218" width="6" height="13" font="2">)</text>
<text top="677" left="223" width="9" height="10" font="5">&#8722;</text>
<text top="678" left="233" width="6" height="9" font="6">1</text>
<text top="681" left="239" width="14" height="13" font="9"><i>, s</i></text>
<text top="681" left="253" width="6" height="13" font="2">(</text>
<text top="681" left="259" width="9" height="13" font="9"><i>x</i></text>
<text top="680" left="271" width="4" height="14" font="10">&#183;</text>
<text top="681" left="278" width="7" height="13" font="9"><i>g</i></text>
<text top="688" left="285" width="6" height="9" font="6">1</text>
<text top="681" left="292" width="6" height="13" font="2">)</text>
<text top="681" left="298" width="7" height="13" font="9"><i>g</i></text>
<text top="688" left="305" width="6" height="9" font="6">2</text>
<text top="681" left="312" width="7" height="13" font="9"><i>s</i></text>
<text top="681" left="319" width="6" height="13" font="2">(</text>
<text top="681" left="325" width="9" height="13" font="9"><i>x</i></text>
<text top="680" left="336" width="4" height="14" font="10">&#183;</text>
<text top="681" left="344" width="7" height="13" font="9"><i>g</i></text>
<text top="688" left="351" width="6" height="9" font="6">1</text>
<text top="681" left="358" width="7" height="13" font="9"><i>g</i></text>
<text top="688" left="365" width="6" height="9" font="6">2</text>
<text top="681" left="372" width="6" height="13" font="2">)</text>
<text top="677" left="377" width="9" height="10" font="5">&#8722;</text>
<text top="678" left="387" width="6" height="9" font="6">1</text>
<text top="681" left="393" width="6" height="13" font="2">)</text>
</page>
</pdf2xml>
`

func TestALimitGoesToTheSignItIsCentredOn(t *testing.T) {
	lines := parse(t, stackedProductsPage)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	want := []string{
		`$\prod_{x\in H\backslash G}^{s(x)^{-1}}c(s(x)g_1s(x\cdot g_1)^{-1}, s(x\cdot g_1)g_2s(x\cdot g_1g_2)^{-1})$`,
		`$=\prod_{x\in H\backslash G}c(g_1s(x\cdot g_1)^{-1}, s(x\cdot g_1)g_2s(x\cdot g_1g_2)^{-1})$`,
	}
	for i, w := range want {
		if got := Render(lines[i]); got != w {
			t.Errorf("Render line %d:\n got %s\nwant %s", i, got, w)
		}
	}
}

// French page 414 hangs the same index off two lines that overlap each other,
// and neither line sets an operator of its own: the only thing drawn in the
// font of a summation sign is the wide hat of G hat, which is an accent and has
// no limits. So there is no sign to measure against and the nearer band decides,
// which puts the index of A sub lambda back on the line A sub lambda is on.
const overlappingIndexPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="414" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="14" family="LMRoman12" color="#131413"/>
<fontspec id="3" size="14" family="LMMathItalic12" color="#131413"/>
<fontspec id="6" size="9" family="LMMathSymbols8" color="#131413"/>
<fontspec id="7" size="9" family="LMRoman8" color="#131413"/>
<fontspec id="8" size="12" family="CMEX10" color="#131413"/>
<fontspec id="9" size="9" family="LMMathItalic8" color="#131413"/>
<fontspec id="14" size="14" family="rsfs10" color="#131413"/>
<text top="773" left="80" width="42" height="12" font="2">phisme</text>
<text top="773" left="126" width="8" height="12" font="3"><i>u</i></text>
<text top="773" left="138" width="28" height="12" font="2">de V</text>
<text top="780" left="165" width="6" height="8" font="9"><i>&#955;</i></text>
<text top="773" left="171" width="92" height="12" font="2">. Soient A = (A</text>
<text top="780" left="263" width="6" height="8" font="9"><i>&#955;</i></text>
<text top="773" left="269" width="5" height="12" font="2">)</text>
<text top="782" left="275" width="6" height="8" font="9"><i>&#955;</i></text>
<text top="779" left="280" width="7" height="12" font="6"><i>&#8712;</i></text>
<text top="776" left="288" width="6" height="15" font="8">b</text>
<text top="782" left="287" width="8" height="8" font="7">G</text>
<text top="773" left="299" width="25" height="12" font="2">et A</text>
<text top="773" left="332" width="30" height="12" font="2">= (A</text>
<text top="780" left="361" width="6" height="8" font="9"><i>&#955;</i></text>
<text top="773" left="368" width="5" height="12" font="2">)</text>
<text top="782" left="373" width="6" height="8" font="9"><i>&#955;</i></text>
<text top="779" left="379" width="7" height="12" font="6"><i>&#8712;</i></text>
<text top="776" left="386" width="6" height="15" font="8">b</text>
<text top="782" left="385" width="8" height="8" font="7">G</text>
<text top="773" left="397" width="109" height="12" font="2">des éléments de F(</text>
<text top="773" left="506" width="65" height="12" font="2">G). Notons</text>
<text top="793" left="80" width="10" height="12" font="2">A</text>
<text top="789" left="91" width="5" height="12" font="6"><i>&#8727;</i></text>
<text top="793" left="100" width="30" height="12" font="2">= (A</text>
<text top="789" left="130" width="5" height="12" font="6"><i>&#8727;</i></text>
<text top="799" left="130" width="6" height="8" font="9"><i>&#955;</i></text>
<text top="793" left="136" width="5" height="12" font="2">)</text>
<text top="801" left="141" width="6" height="8" font="9"><i>&#955;</i></text>
<text top="798" left="147" width="7" height="12" font="6"><i>&#8712;</i></text>
<text top="795" left="154" width="6" height="15" font="8">b</text>
<text top="801" left="154" width="8" height="8" font="7">G</text>
<text top="793" left="162" width="38" height="12" font="2">. On a</text>
<text top="789" left="204" width="12" height="18" font="14"><i>F</i></text>
<text top="793" left="218" width="5" height="12" font="2">(</text>
<text top="793" left="223" width="7" height="12" font="3"><i>a</i></text>
<text top="789" left="230" width="5" height="12" font="6"><i>&#8727;</i></text>
<text top="793" left="236" width="29" height="12" font="2">) = (</text>
<text top="789" left="265" width="12" height="18" font="14"><i>F</i></text>
<text top="793" left="279" width="5" height="12" font="2">(</text>
<text top="793" left="284" width="7" height="12" font="3"><i>a</i></text>
<text top="793" left="291" width="10" height="12" font="2">))</text>
<text top="789" left="302" width="5" height="12" font="6"><i>&#8727;</i></text>
<text top="793" left="312" width="107" height="12" font="2">pour tout élément</text>
<text top="793" left="423" width="7" height="12" font="3"><i>a</i></text>
<text top="793" left="435" width="96" height="12" font="2">de <b>C</b>[G]. Posons</text>
</page>
</pdf2xml>
`

// Page 179 of Lie 7 to 9 sets a sum in the middle of a sentence, and the box
// poppler reports for it is not the box it is drawn in. The sign is reported at
// top 200, which is the band of the line above, running 198 to 211, and the
// sentence it belongs to runs 217 to 230. Placing it by its top put the sum in
// front of the sl_2 of the line above and left its limit standing under a line
// with no sign on it, so the page shipped "there exists an$\sum$sl_2-triplet"
// and "the elements of $_{\alpha\in B}$".
//
// The foot is what places it. This printing reports the height of an extension
// glyph as 19 whatever size it is set at, which is junk, but top plus height
// lands at 219, two units inside the band of the line the sign is on. That is
// the whole of the measurement, and it holds for the other ten sums of the page.
const inlineSumPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="179" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="4" size="15" family="DGLKJH+CMR10" color="#000000"/>
<fontspec id="5" size="15" family="DGLMMN+CMMI10" color="#000000"/>
<fontspec id="6" size="10" family="DGLNOH+CMSY7" color="#000000"/>
<fontspec id="7" size="15" family="DGMBID+CMTI10" color="#000000"/>
<fontspec id="8" size="15" family="DGLMFD+EUFM10" color="#000000"/>
<fontspec id="10" size="10" family="DGLOII+CMR7" color="#000000"/>
<fontspec id="11" size="15" family="DGMCMP+CMEX10" color="#000000"/>
<fontspec id="12" size="10" family="DGLMOM+CMMI7" color="#000000"/>
<text top="198" left="105" width="20" height="13" font="4">(ii)</text>
<text top="198" left="131" width="87" height="13" font="7"><i>The elements</i></text>
<text top="198" left="224" width="9" height="13" font="5"><i>x</i></text>
<text top="198" left="238" width="12" height="13" font="7"><i>of</i></text>
<text top="200" left="257" width="8" height="12" font="8">g</text>
<text top="198" left="270" width="161" height="13" font="7"><i>such that there exists an</i></text>
<text top="200" left="437" width="11" height="12" font="8">sl</text>
<text top="204" left="448" width="6" height="9" font="10">2</text>
<text top="198" left="455" width="125" height="13" font="7"><i>-triplet of the form</i></text>
<text top="217" left="82" width="6" height="13" font="4">(</text>
<text top="217" left="88" width="24" height="13" font="5"><i>x, h</i></text>
<text top="215" left="112" width="6" height="9" font="10">0</text>
<text top="217" left="119" width="14" height="13" font="5"><i>, y</i></text>
<text top="217" left="133" width="6" height="13" font="4">)</text>
<text top="217" left="144" width="123" height="13" font="7"><i>are the elements of</i></text>
<text top="200" left="277" width="16" height="19" font="11">P</text>
<text top="231" left="273" width="8" height="9" font="12"><i>&#945;</i></text>
<text top="229" left="280" width="8" height="14" font="6"><i>&#8712;</i></text>
<text top="231" left="289" width="8" height="9" font="10">B</text>
<text top="219" left="299" width="8" height="12" font="8">g</text>
<text top="215" left="307" width="8" height="9" font="12"><i>&#945;</i></text>
<text top="217" left="321" width="259" height="13" font="7"><i>that have a non-zero component in each</i></text>
<text top="243" left="82" width="8" height="12" font="8">g</text>
<text top="239" left="90" width="8" height="9" font="12"><i>&#945;</i></text>
<text top="242" left="99" width="5" height="13" font="7"><i>.</i></text>
</page>
</pdf2xml>
`

func TestAnOperatorGoesToTheLineItsFootIsIn(t *testing.T) {
	lines := parse(t, inlineSumPage)
	if len(lines) != 3 {
		t.Fatalf("got %d lines, want 3", len(lines))
	}
	want := []string{
		`(ii) The elements $x$ of $\mathfrak{g}$ such that there exists an $\mathfrak{s}\mathfrak{l}_2$-triplet of the form`,
		`$(x, h^0, y)$ are the elements of $\sum_{\alpha\in B}\mathfrak{g}^{\alpha}$ that have a non-zero component in each`,
		`$\mathfrak{g}^{\alpha}$.`,
	}
	for i, w := range want {
		if got := Render(lines[i]); got != w {
			t.Errorf("Render line %d:\n got %s\nwant %s", i, got, w)
		}
	}
}

func TestAnIndexGoesToTheNearerLine(t *testing.T) {
	lines := parse(t, overlappingIndexPage)
	if len(lines) != 2 {
		t.Fatalf("got %d lines, want 2", len(lines))
	}
	want := []string{
		`phisme $u$ de $V_{\lambda}$. Soient $A = (A_{\lambda})_{\lambda\in\widehat{G}}$ et A $= (A_{\lambda})_{\lambda\in\widehat{G}}$ des éléments de F(G). Notons`,
		`$A^*= (A^*_{\lambda})_{\lambda\in\widehat{G}}$. On a $\mathscr{F}(a^*) = (\mathscr{F}(a))^*$ pour tout élément $a$ de $\mathbf{C}[G]$. Posons`,
	}
	for i, w := range want {
		if got := Render(lines[i]); got != w {
			t.Errorf("Render line %d:\n got %s\nwant %s", i, got, w)
		}
	}
}

// Page 431 of Théories spectrales V sets an integral over G/Z with the square
// of the norm of f(g) before it. The exponent of the closing bar of that norm
// stands at 735 and ends at 746, and the display it belongs to opens at 746, so
// the two touch and do not overlap. Asking for a unit of overlap left the
// exponent standing as a line of its own, and the sign of the integral, which
// is drawn well above the band, came away with it: the page was printed with
// the integral on a line by itself, and the G/Z written under a sign that was
// no longer there was read on to the display below as a second pair of scripts.
const touchingExponentPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="431" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="16" family="MJDJIW+LMMathSymbols10" color="#000000"/>
<fontspec id="3" size="16" family="XAEWAV+LMMathItalic10" color="#000000"/>
<fontspec id="4" size="16" family="CWSGRY+LMRoman10" color="#000000"/>
<fontspec id="5" size="12" family="GORNHQ+LMRoman8" color="#000000"/>
<fontspec id="6" size="16" family="DCOIDG+MSAM10" color="#000000"/>
<fontspec id="7" size="15" family="ODOCWD+LMMathExtension10" color="#000000"/>
<fontspec id="8" size="12" family="GFWRXJ+LMMathItalic8" color="#000000"/>
<text top="745" left="153" width="5" height="15" font="2">|</text>
<text top="746" left="157" width="8" height="15" font="3"><i>f</i></text>
<text top="746" left="167" width="6" height="14" font="4">(</text>
<text top="746" left="173" width="8" height="15" font="3"><i>g</i></text>
<text top="746" left="182" width="6" height="14" font="4">)</text>
<text top="745" left="188" width="5" height="15" font="2">|</text>
<text top="742" left="193" width="6" height="11" font="5">2</text>
<text top="746" left="204" width="13" height="17" font="6">&#10877;</text>
<text top="736" left="230" width="8" height="7" font="7">Z</text>
<text top="762" left="239" width="10" height="11" font="5">G</text>
<text top="762" left="249" width="6" height="11" font="8"><i>/</i></text>
<text top="762" left="255" width="8" height="11" font="5">Z</text>
<text top="745" left="264" width="5" height="15" font="2">|</text>
<text top="746" left="268" width="8" height="15" font="3"><i>f</i></text>
<text top="751" left="276" width="6" height="11" font="5">1</text>
<text top="745" left="283" width="5" height="15" font="2">|</text>
<text top="742" left="288" width="6" height="11" font="5">1</text>
<text top="742" left="294" width="6" height="11" font="8"><i>/</i></text>
<text top="742" left="300" width="6" height="11" font="5">2</text>
<text top="745" left="310" width="5" height="15" font="2">|</text>
<text top="746" left="315" width="8" height="15" font="3"><i>f</i></text>
<text top="751" left="323" width="6" height="11" font="5">3</text>
<text top="745" left="330" width="15" height="15" font="2">| |</text>
<text top="746" left="345" width="10" height="15" font="3"><i>&#947;</i></text>
<text top="753" left="355" width="10" height="11" font="5">G</text>
<text top="753" left="365" width="11" height="11" font="8"><i>,&#967;</i></text>
<text top="746" left="377" width="6" height="14" font="4">(</text>
<text top="746" left="384" width="8" height="15" font="3"><i>g</i></text>
<text top="746" left="392" width="6" height="14" font="4">)</text>
<text top="746" left="398" width="8" height="15" font="3"><i>f</i></text>
<text top="751" left="406" width="6" height="11" font="5">2</text>
<text top="745" left="414" width="5" height="15" font="2">|</text>
<text top="746" left="421" width="17" height="15" font="3"><i>d&#957;</i></text>
<text top="735" left="447" width="6" height="11" font="5">2</text>
</page>
</pdf2xml>
`

func TestAScriptThatTouchesTheBandIsPartOfTheLine(t *testing.T) {
	lines := parse(t, touchingExponentPage)
	if len(lines) != 1 {
		for i, l := range lines {
			t.Logf("line %d: %s", i, Render(l))
		}
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	want := `$|f(g)|^2\leqslant \int_{G/Z}|f_1|^{1/2}|f_3| |\gamma_{G,\chi}(g)f_2|d\nu^2$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}
