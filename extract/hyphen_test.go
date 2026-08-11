package extract

import "testing"

// Bourbaki sets the A_M of "A_M-module" in the mathematics font and the word
// after it in roman, so the hyphen comes back from poppler on the mathematics
// side of that boundary. Left there the page reads $A_M-$ module, which is
// wrong twice over: the hyphen is typeset as a minus sign, and the compound
// word is broken by a space that is not on the page. There were 64 of them
// across 50 of the 505 pages of Algebra VIII.
//
// The trouble is that the volume also writes real subtractions the same way,
// and the two are told apart only by what comes after the hyphen. Both fixtures
// below are what poppler prints for the volume, and both readings were checked
// against the printed page.

// Page 100, the line that carries the module. The hyphen and the word after it
// are one run, "-module", set in roman against the closing bracket of M_n(A).
const compoundXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="100" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="3" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="4" size="15" family="AYUTJX+LMRoman10" color="#000000"/>
<fontspec id="5" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="8" size="15" family="GSFMFK+LMRoman10" color="#000000"/>
<fontspec id="9" size="10" family="CWSGRY+LMMathItalic7" color="#000000"/>
<text top="131" left="79" width="28" height="13" font="4"><i>view</i></text>
<text top="131" left="112" width="11" height="13" font="5">A</text>
<text top="128" left="123" width="7" height="9" font="9"><i>n</i></text>
<text top="131" left="136" width="51" height="13" font="4"><i>as a left</i></text>
<text top="131" left="191" width="16" height="13" font="8"><b>M</b></text>
<text top="138" left="207" width="7" height="9" font="9"><i>n</i></text>
<text top="131" left="216" width="23" height="13" font="5">(A)</text>
<text top="131" left="238" width="50" height="13" font="4"><i>-module</i></text>
<text top="131" left="294" width="143" height="13" font="3">(II, §10, No. 7, p. 349)</text>
<text top="131" left="437" width="141" height="13" font="4"><i>. The endomorphisms</i></text>
</page>
</pdf2xml>
`

func TestCompoundHyphenLeavesTheFormula(t *testing.T) {
	lines := parse(t, compoundXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `view $A^n$ as a left $\mathbf{M}_n(A)$-module (II, §10, No. 7, p. 349). The endomorphisms`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 29, the line that sets R. The minus is a real minus, drawn from the
// symbol font, and the operand after it is the product XQ. Read as a compound
// hyphen this comes out as $R = P$-XQ, which says the opposite of the page.
const minusXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="29" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="15" family="XJSZDJ+LMMathItalic10" color="#000000"/>
<fontspec id="3" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="4" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="5" size="15" family="ILCVAI+EUFM10" color="#000000"/>
<fontspec id="6" size="10" family="CWSGRY+LMMathItalic7" color="#000000"/>
<fontspec id="7" size="10" family="DCOIDG+LMRoman7" color="#000000"/>
<fontspec id="9" size="15" family="XHUWIC+LMMathSymbols10" color="#000000"/>
<text top="401" left="80" width="88" height="13" font="3">element Q of</text>
<text top="401" left="175" width="8" height="13" font="5">b</text>
<text top="400" left="188" width="10" height="14" font="9">∩</text>
<text top="401" left="202" width="11" height="13" font="4">B</text>
<text top="408" left="213" width="7" height="9" font="6"><i>n</i></text>
<text top="401" left="228" width="63" height="13" font="3">such that</text>
<text top="401" left="298" width="10" height="13" font="2"><i>ϕ</i></text>
<text top="408" left="308" width="7" height="9" font="6"><i>n</i></text>
<text top="408" left="315" width="15" height="9" font="7">+1</text>
<text top="401" left="331" width="41" height="13" font="4">(P) =</text>
<text top="401" left="379" width="9" height="13" font="2"><i>σ</i></text>
<text top="401" left="388" width="6" height="13" font="4">(</text>
<text top="401" left="394" width="10" height="13" font="2"><i>ϕ</i></text>
<text top="408" left="404" width="7" height="9" font="6"><i>n</i></text>
<text top="401" left="412" width="121" height="13" font="4">(Q)). Set R = P</text>
<text top="400" left="538" width="12" height="14" font="9">−</text>
<text top="401" left="554" width="27" height="13" font="4">XQ.</text>
</page>
</pdf2xml>
`

func TestAMinusStaysInTheFormula(t *testing.T) {
	lines := parse(t, minusXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	got := Render(lines[0])
	if want := "Set $R = P-$ XQ."; got[len(got)-len(want):] != want {
		t.Errorf("Render:\n got %s\nwant a line ending %s", got, want)
	}
}

// A hyphen at the end of a line is decided the same way, except that the word
// after it is on the next line and join is what holds both.
//
// Every left hand side here is a line of Algebra VIII as the renderer writes
// it, and every right hand side is the line that follows it in the volume.
func TestWhatRunsOnAtTheEndOfALine(t *testing.T) {
	for _, c := range []struct {
		name string
		line string
		next string
		want string
	}{
		{"a word broken in two", "the commu-", "tative ring A", "the commutative ring A"},
		{"a module", `Let P be an $(A,B)_k-$`, "bimodule and let", `Let P be an $(A,B)_k$-bimodule and let`},
		{"a word already carrying a hyphen", `Prove that $V'$ is a $(D,A)-$`, "sub-bimodule of", `Prove that $V'$ is a $(D,A)$-sub-bimodule of`},
		{"a module at the end of an aside", `$M/\mathfrak{R}(A)M$ is an $A/\mathfrak{R}(A)-$`, "module). b) Deduce", `$M/\mathfrak{R}(A)M$ is an $A/\mathfrak{R}(A)$-module). b) Deduce`},
		{"a space left where the formula ended", `is a $\tau -$`, "extension; we call it", `is a $\tau$-extension; we call it`},
		{"an operand in capitals", `Set $R = P-$`, "XQ. Because of", ""},
		{"a function applied to something", `the element cl(E) $-$ cl(E$')-$`, "cl(E$'')$ of the group", ""},
		{"a module whose ring is a plain letter", "Let M be a left A-", "module. The countermodule", "Let M be a left A-module. The countermodule"},
		{"an algebra over a field", "There exists a K-", "algebra homomorphism", "There exists a K-algebra homomorphism"},
		{"a line that does not run on", "the center of A", "is the set of", ""},
	} {
		t.Run(c.name, func(t *testing.T) {
			got, ok := runOn(c.line, c.next, nil)
			if ok != (c.want != "") {
				t.Fatalf("runOn(%q, %q) joined = %v, want %v", c.line, c.next, ok, c.want != "")
			}
			if ok && got != c.want {
				t.Errorf("runOn:\n got %s\nwant %s", got, c.want)
			}
		})
	}
}
