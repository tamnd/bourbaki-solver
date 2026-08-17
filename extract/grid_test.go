package extract

import "testing"

// Page 379 of Algebra VIII, the isomorphism of the matrix algebra. The book
// prints the diagonal matrix of a and b between parentheses, and TeX sets a two
// by two matrix set inline at the size and the heights a superscript and a
// subscript are set at: the top row where the exponent goes and the bottom row
// where the index goes. Read run by run that is a superscript, a subscript, a
// superscript and a subscript against one base, which TeX refuses by name, and
// the page shipped "(^a_0^0_b)".
const diagonalMatrixXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="379" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="12" size="15" family="ORQWKC+PygvnsBslxqcLMRoman10" color="#000000"/>
<fontspec id="13" size="15" family="AQKDGP+QwxclbXgkcwbLMMathItalic10" color="#000000"/>
<fontspec id="14" size="15" family="XAAOWT+BrgrhxHhwpyxLMMathSymbols10" color="#000000"/>
<fontspec id="15" size="10" family="AQKDGP+QwxclbXgkcwbLMMathItalic7" color="#000000"/>
<fontspec id="16" size="10" family="ORQWKC+PygvnsBslxqcLMRoman7" color="#000000"/>
<text top="633" left="346" width="133" height="13" font="12">= 1. The mapping (</text>
<text top="633" left="479" width="21" height="13" font="13"><i>a, b</i></text>
<text top="633" left="500" width="6" height="13" font="12">)</text>
<text top="632" left="510" width="15" height="14" font="14">7→</text>
<text top="633" left="529" width="6" height="13" font="12">(</text>
<text top="631" left="538" width="6" height="9" font="15"><i>a</i></text>
<text top="641" left="538" width="6" height="9" font="16">0</text>
<text top="631" left="548" width="6" height="9" font="16">0</text>
<text top="640" left="549" width="5" height="9" font="15"><i>b</i></text>
<text top="633" left="557" width="21" height="13" font="12">) is</text>
</page>
</pdf2xml>
`

func TestATwoByTwoMatrixSetInlineIsAMatrix(t *testing.T) {
	lines := parse(t, diagonalMatrixXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `= 1. The mapping $(a, b)\mapsto \begin{pmatrix} a & 0 \\ 0 & b \end{pmatrix}$ is`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 224 of the same volume, exercise 5 of § 11, which prints the same
// diagonal matrix of p and q. Here the opening parenthesis is one of the tall
// ones TeX draws for a matrix and it is not in the text layer at all, so the
// grid stands with a base token on neither side of it that is a delimiter. The
// grid is still a grid, and reading it as one is what the page says; the
// parenthesis that is there closes nothing and is left where it was printed.
const openMatrixXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="224" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="12" size="13" family="ORQWKC+PygvnsBslxqcLMRoman9" color="#000000"/>
<fontspec id="13" size="13" family="AQKDGP+QwxclbXgkcwbLMMathItalic9" color="#000000"/>
<fontspec id="15" size="9" family="AQKDGP+QwxclbXgkcwbLMMathItalic6" color="#000000"/>
<fontspec id="16" size="9" family="ORQWKC+PygvnsBslxqcLMRoman6" color="#000000"/>
<text top="593" left="158" width="29" height="12" font="12">) = (</text>
<text top="593" left="187" width="12" height="12" font="13"><i>m</i></text>
<text top="593" left="202" width="11" height="12" font="12">+</text>
<text top="593" left="216" width="12" height="12" font="13"><i>n,</i></text>
<text top="590" left="239" width="6" height="8" font="15"><i>p</i></text>
<text top="599" left="240" width="5" height="8" font="16">0</text>
<text top="590" left="249" width="5" height="8" font="16">0</text>
<text top="599" left="249" width="5" height="8" font="15"><i>q</i></text>
<text top="593" left="263" width="9" height="12" font="12">).</text>
</page>
</pdf2xml>
`

func TestAMatrixWhoseOpeningDelimiterWasNotDrawn(t *testing.T) {
	lines := parse(t, openMatrixXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `$) = (m+n,\begin{pmatrix} p & 0 \\ 0 & q \end{pmatrix}$ ).`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// A base carrying one superscript and one subscript is a base carrying one
// superscript and one subscript, and no number of them in a row makes a grid.
// x_i^2 y_j^3 has to come through as it always did.
const scriptsNotAGridXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="1" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="13" size="15" family="AQKDGP+QwxclbXgkcwbLMMathItalic10" color="#000000"/>
<fontspec id="15" size="10" family="AQKDGP+QwxclbXgkcwbLMMathItalic7" color="#000000"/>
<fontspec id="16" size="10" family="ORQWKC+PygvnsBslxqcLMRoman7" color="#000000"/>
<text top="100" left="100" width="8" height="13" font="13"><i>x</i></text>
<text top="106" left="108" width="5" height="9" font="15"><i>i</i></text>
<text top="96" left="108" width="5" height="9" font="16">2</text>
</page>
</pdf2xml>
`

func TestOneSuperscriptAndOneSubscriptAreNotAGrid(t *testing.T) {
	lines := parse(t, scriptsNotAGridXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `$x^2_i$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}
