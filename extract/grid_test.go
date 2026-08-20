package extract

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

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

// Page 166 of Topologie algébrique, exercise 5 c) of § 6, which prints the two
// generators of SL(2, Z) as matrices set inline. Nothing here is boxed by
// column: the top row of A arrives as the single run "0 1" and its bottom row
// as two, the minus out of the symbol font and then "1 0", and B is broken the
// other way about. Both matrices are drawn between the tall parentheses of the
// math extension font, which the text layer reports as boxes with nothing at
// all in them. The page shipped "$_{-1 0}^{0 1}$" and "$^1_{1 0}^{-1}$".
const rowMatrixXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="166" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="15" family="MBPDRS+QkwywvSbcbrxLMRoman10" color="#000000"/>
<fontspec id="3" size="15" family="IWFIMB+PygvnsBslxqcLMRoman10" color="#000000"/>
<fontspec id="5" size="15" family="IWFIMB+BmxwvhBwyvtgLMRoman10" color="#000000"/>
<fontspec id="7" size="10" family="PNVTZC+JggwxpCkwggdLMRoman7" color="#000000"/>
<fontspec id="8" size="15" family="XAAOWT+BrgrhxHhwpyxLMMathSymbols10" color="#000000"/>
<fontspec id="9" size="15" family="HMSDWV+VshhsnLqjqwtLMMathExtension10" color="#000000"/>
<fontspec id="12" size="10" family="GGUHCB+GgcvhhNgchdxLMMathSymbols7" color="#000000"/>
<fontspec id="13" size="15" family="MBPDRS+GjmylgCvgpxqLMRoman10" color="#000000"/>
<text top="541" left="81" width="7" height="13" font="2"><i>c</i></text>
<text top="537" left="88" width="77" height="19" font="3">) On pose</text>
<text top="541" left="173" width="11" height="13" font="13"><i>A</i></text>
<text top="537" left="192" width="12" height="19" font="5">=</text>
<text top="538" left="226" width="21" height="9" font="7">0 1</text>
<text top="546" left="221" width="9" height="10" font="12">−</text>
<text top="547" left="230" width="16" height="9" font="7">1 0</text>
<text top="538" left="249" width="7" height="7" font="9"></text>
<text top="537" left="263" width="12" height="19" font="3">et</text>
<text top="541" left="283" width="11" height="13" font="13"><i>B</i></text>
<text top="537" left="302" width="12" height="19" font="5">=</text>
<text top="538" left="331" width="6" height="9" font="7">1</text>
<text top="537" left="342" width="9" height="10" font="12">−</text>
<text top="538" left="351" width="6" height="9" font="7">1</text>
<text top="548" left="331" width="21" height="9" font="7">1 0</text>
<text top="538" left="359" width="7" height="7" font="9"></text>
<text top="537" left="366" width="90" height="19" font="3">. Vérifier que</text>
<text top="541" left="463" width="11" height="13" font="13"><i>A</i></text>
<text top="538" left="474" width="6" height="9" font="7">2</text>
<text top="537" left="489" width="12" height="19" font="5">=</text>
<text top="541" left="509" width="11" height="13" font="13"><i>B</i></text>
<text top="538" left="520" width="6" height="9" font="7">3</text>
<text top="537" left="535" width="12" height="19" font="5">=</text>
<text top="540" left="555" width="12" height="14" font="8">−</text>
<text top="541" left="566" width="6" height="13" font="13"><i>I</i></text>
<text top="537" left="574" width="4" height="19" font="3">.</text>
</page>
</pdf2xml>
`

func TestAMatrixBoxedByRowIsAMatrix(t *testing.T) {
	lines := parse(t, rowMatrixXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `c) On pose A = $\begin{pmatrix} 0 & 1 \\ -1 & 0 \end{pmatrix}$ et B = ` +
		`$\begin{pmatrix} 1 & -1 \\ 1 & 0 \end{pmatrix}$ . Vérifier que $A^2$ = $B^3=-I$.`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// The rows of a matrix hang off nothing and the scripts of a base hang off the
// base, and that is the whole of what tells the two apart once the page has
// stopped boxing by column. This line is made up rather than measured off a
// page, since what it has to hold is the shape the row reader would otherwise
// take: x^{a b}_{c d} is two rows of two cells drawn one over the other,
// exactly as the matrices of page 166 are, and the only thing it does that they
// do not is stand against the x. The boxes are the ones the French printing
// reports for type of that size, and the cluster is set against the base the
// way that printing sets an index, which is with nothing between them.
const scriptRowsNotAGridXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="1" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="3" size="15" family="IWFIMB+PygvnsBslxqcLMRoman10" color="#000000"/>
<fontspec id="13" size="15" family="AQKDGP+QwxclbXgkcwbLMMathItalic10" color="#000000"/>
<fontspec id="15" size="10" family="AQKDGP+QwxclbXgkcwbLMMathItalic7" color="#000000"/>
<text top="537" left="81" width="77" height="19" font="3">On pose</text>
<text top="541" left="173" width="8" height="13" font="13"><i>x</i></text>
<text top="538" left="181" width="16" height="9" font="15"><i>a b</i></text>
<text top="548" left="181" width="16" height="9" font="15"><i>c d</i></text>
<text top="537" left="205" width="90" height="19" font="3">, où a et b sont</text>
</page>
</pdf2xml>
`

func TestTwoRowsAgainstABaseAreNotAGrid(t *testing.T) {
	lines := parse(t, scriptRowsNotAGridXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `On pose $x^{a b}_{c d}$, où a et b sont`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 208 of the French printing of Algebra VIII, exercise 5 b) of § 11, which
// is the same exercise page 225 of the English printing sets and the same matrix
// of X, 0 over 0, Y. The English page read as a matrix from the first and this
// one did not, and both things that stopped it are the type rather than the
// mathematics.
//
// The English printing sets the four entries out of LMRoman7 and LMRoman6 and
// reports every box 8 units tall, so the rows abut. This one sets them out of
// LMRoman8 and reports the letters 12 units tall and the digits 8, so the top
// row hangs 3 units into the top of the bottom row, which two units of slack
// refused. And the top row arrives as one element holding an italic "X " and an
// upright 0, so the first column is the run "X " with the space inside the box
// it is reported in, and its right edge falls exactly on the left edge of the 0
// beside it: the columns were refused for a gap the page has and the box does
// not show.
const columnMatrixXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="208" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="1" size="13" family="LMRoman10" color="#131413"/>
<fontspec id="2" size="9" family="LMMathItalic8" color="#131413"/>
<fontspec id="4" size="9" family="LMRoman8" color="#131413"/>
<fontspec id="7" size="13" family="LMRoman10" color="#131413"/>
<fontspec id="8" size="12" family="CMEX10" color="#131413"/>
<fontspec id="10" size="9" family="LMRoman8" color="#131413"/>
<text top="362" left="80" width="42" height="11" font="7"><i>XY </i>= <i>I</i></text>
<text top="367" left="125" width="5" height="8" font="2"><i>p</i></text>
<text top="362" left="136" width="58" height="11" font="1">et <i>YX </i>= <i>I</i></text>
<text top="367" left="196" width="4" height="8" font="2"><i>q</i></text>
<text top="362" left="201" width="180" height="11" font="1">. Prouver que la matrice carrée</text>
<text top="351" left="387" width="5" height="15" font="8"></text>
<text top="358" left="394" width="19" height="12" font="10"><i>X </i>0</text>
<text top="369" left="396" width="5" height="8" font="4">0</text>
<text top="367" left="406" width="7" height="12" font="10"><i>Y</i></text>
<text top="351" left="417" width="5" height="15" font="8"></text>
<text top="362" left="428" width="143" height="11" font="1">est l’image d’une matrice</text>
</page>
</pdf2xml>
`

func TestAMatrixTheFrenchPrintingSetsInTallerType(t *testing.T) {
	lines := parse(t, columnMatrixXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `XY = $I_p$ et YX = $I_q$. Prouver que la matrice carrée ` +
		`$\begin{pmatrix} X & 0 \\ 0 & Y \end{pmatrix}$ est l’image d’une matrice`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 186 of the same volume, the corollary of § 10. The limits of the sum are
// written in two pieces each, n and +1 above i and =0 below, and they pair off
// into columns as neatly as any matrix does. What says they are not a matrix is
// that the pieces of one script are set touching, where the columns of a matrix
// stand apart, and the limits of this sum are the case that measures it: the n
// ends at 277 and the +1 begins at 277.
//
// It is the whole guard. Reading a column as apart where it merely does not
// overlap turned nine pages of the six volumes into matrices of their own
// exponents, this sum among them, and it came out as the matrix of n, +1 over
// i, =0.
//
// The line is cut where the printed sentence is not, since the reference to (5)
// and (6) in front of it is set in the colour of a link and is nothing to do
// with this. The sum sign itself is a box with nothing in it here: the sigma is
// drawn out of CMEX10 and is recovered from the file rather than from the text
// layer, so a fixture written in the text layer alone cannot carry it, and the
// limits are what the test is about.
const sumLimitsNotAGridXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="186" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="14" family="LMRoman12" color="#131413"/>
<fontspec id="5" size="14" family="LMMathItalic12" color="#131413"/>
<fontspec id="6" size="9" family="LMMathItalic8" color="#131413"/>
<fontspec id="7" size="14" family="LMMathSymbols10" color="#131413"/>
<fontspec id="8" size="9" family="LMRoman8" color="#131413"/>
<fontspec id="10" size="12" family="CMEX10" color="#131413"/>
<text top="320" left="151" width="103" height="12" font="2">), on tire aussitôt</text>
<text top="310" left="259" width="12" height="15" font="10"></text>
<text top="318" left="271" width="6" height="8" font="6"><i>n</i></text>
<text top="318" left="277" width="12" height="8" font="8">+1</text>
<text top="327" left="271" width="3" height="8" font="6"><i>i</i></text>
<text top="327" left="274" width="12" height="8" font="8">=0</text>
<text top="320" left="290" width="5" height="12" font="2">(</text>
<text top="317" left="295" width="11" height="18" font="7"><i>−</i></text>
<text top="320" left="306" width="12" height="12" font="2">1)</text>
<text top="319" left="318" width="3" height="8" font="6"><i>i</i></text>
<text top="320" left="322" width="9" height="12" font="5"><i>ϕ</i></text>
<text top="320" left="331" width="14" height="12" font="2">(E</text>
<text top="327" left="345" width="3" height="8" font="6"><i>i</i></text>
<text top="320" left="349" width="184" height="12" font="2">) = 0 et le corollaire en résulte.</text>
</page>
</pdf2xml>
`

func TestTheLimitsOfASumAreNotAGrid(t *testing.T) {
	lines := parse(t, sumLimitsNotAGridXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `), on tire aussitôt $^{n+1}_{i=0}(-1)^i\varphi (E_i) = 0$ et le corollaire en résulte.`
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

// Page 73 of Théories spectrales, the three congruences of the proof of
// theorem 1. Each is a two by two matrix whose top left entry carries a script
// of its own, and that script is set a level further in than the entries are:
// the alpha and the zeros of the second matrix come out of LMRoman8 at 12 and
// the -1 and the 0 written on the alpha out of LMRoman6 at 9, which is under
// four fifths of it and so one level down again.
//
// Neither of the older shapes can read that. The columns are counted at one
// level and the rows are cut at one level, and there are two here. The page
// shipped the second matrix as ^{\alpha^-_0}_{0^1}^0_0, which is the exponent
// of the entry landing on the entry of the row below it, since the 1 is drawn
// after that entry and run order is not reading order once a script is
// involved.
const deepMatrixXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="73" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="4" size="16" family="KVZJDY+LMRoman10" color="#000000"/>
<fontspec id="5" size="16" family="OSYQLC+LMMathItalic10" color="#000000"/>
<fontspec id="6" size="12" family="KXVPXH+LMRoman8" color="#000000"/>
<fontspec id="8" size="15" family="DOQQMC+LMMathExtension10" color="#000000"/>
<fontspec id="9" size="12" family="JWXNWC+LMMathItalic8" color="#000000"/>
<fontspec id="10" size="9" family="HYXHKT+LMMathSymbols6" color="#000000"/>
<fontspec id="11" size="9" family="DVCVPT+LMRoman6" color="#000000"/>
<fontspec id="14" size="16" family="PTVHYM+LMMathSymbols10" color="#000000"/>
<text top="259" left="157" width="9" height="15" font="5"><i>u</i></text>
<text top="258" left="171" width="13" height="15" font="14">≡</text>
<text top="259" left="188" width="6" height="14" font="4">(</text>
<text top="256" left="197" width="8" height="11" font="9"><i>α</i></text>
<text top="256" left="210" width="6" height="11" font="6">0</text>
<text top="267" left="198" width="18" height="11" font="6">0 0</text>
<text top="259" left="219" width="6" height="14" font="4">)</text>
<text top="259" left="234" width="5" height="15" font="5"><i>,</i></text>
<text top="259" left="257" width="8" height="15" font="5"><i>v</i></text>
<text top="264" left="265" width="6" height="11" font="6">0</text>
<text top="258" left="277" width="13" height="15" font="14">≡</text>
<text top="252" left="294" width="9" height="7" font="8"></text>
<text top="256" left="306" width="8" height="11" font="9"><i>α</i></text>
<text top="252" left="314" width="9" height="8" font="10">−</text>
<text top="253" left="323" width="5" height="8" font="11">1</text>
<text top="262" left="314" width="5" height="8" font="11">0</text>
<text top="256" left="333" width="6" height="11" font="6">0</text>
<text top="270" left="314" width="6" height="11" font="6">0</text>
<text top="270" left="333" width="6" height="11" font="6">0</text>
<text top="252" left="342" width="9" height="7" font="8"></text>
<text top="259" left="360" width="5" height="15" font="5"><i>,</i></text>
<text top="259" left="383" width="11" height="15" font="5"><i>φ</i></text>
<text top="259" left="394" width="6" height="14" font="4">(</text>
<text top="259" left="400" width="9" height="15" font="5"><i>u</i></text>
<text top="259" left="410" width="6" height="14" font="4">)</text>
<text top="258" left="421" width="13" height="15" font="14">≡</text>
<text top="252" left="438" width="9" height="7" font="8"></text>
<text top="258" left="449" width="8" height="11" font="9"><i>α</i></text>
<text top="255" left="458" width="9" height="8" font="10">−</text>
<text top="255" left="466" width="5" height="8" font="11">1</text>
<text top="258" left="477" width="6" height="11" font="6">0</text>
<text top="268" left="458" width="6" height="11" font="6">0</text>
<text top="268" left="477" width="6" height="11" font="6">0</text>
<text top="252" left="486" width="9" height="7" font="8"></text>
<text top="259" left="498" width="5" height="15" font="5"><i>.</i></text>
</page>
</pdf2xml>
`

func TestAMatrixWhoseEntriesCarryTheirOwnScripts(t *testing.T) {
	lines := parse(t, deepMatrixXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `$u\equiv \begin{pmatrix} \alpha & 0 \\ 0 & 0 \end{pmatrix},` +
		`v_0\equiv \begin{pmatrix} \alpha_0^{-1} & 0 \\ 0 & 0 \end{pmatrix},` +
		`\varphi (u)\equiv \begin{pmatrix} \alpha^{-1} & 0 \\ 0 & 0 \end{pmatrix}$.`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 70 of the same volume, the matrix of u_1, 0 over 0, 0 in the paragraph
// after definition 2. This one is drawn in both shapes at once: the top row is
// boxed by column, the entry u with its index 1 and then the 0 beside it, and
// the bottom row is boxed whole as the single run "0 0". The columns cannot be
// paired, since there is one box below and two above, and the rows cannot be
// cut at one level, since the index is a level deeper than the entry carrying
// it. Cutting the rows at the white after the entries have been put back
// together reads both, and the page had shipped it as ^u_{0 0^1}^0.
//
// The line is cut after the où, which the printed sentence runs on past.
const mixedMatrixXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="70" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="0" size="16" family="KVZJDY+LMRoman10" color="#000000"/>
<fontspec id="8" size="12" family="KXVPXH+LMRoman8" color="#000000"/>
<fontspec id="9" size="15" family="DOQQMC+LMMathExtension10" color="#000000"/>
<fontspec id="10" size="12" family="JWXNWC+LMMathItalic8" color="#000000"/>
<fontspec id="11" size="9" family="DVCVPT+LMRoman6" color="#000000"/>
<text top="312" left="81" width="55" height="14" font="0">matrice</text>
<text top="308" left="151" width="7" height="11" font="10"><i>u</i></text>
<text top="312" left="158" width="5" height="8" font="11">1</text>
<text top="308" left="169" width="6" height="11" font="8">0</text>
<text top="320" left="155" width="21" height="11" font="8">0 0</text>
<text top="310" left="178" width="7" height="7" font="9"></text>
<text top="312" left="191" width="18" height="14" font="0">où</text>
</page>
</pdf2xml>
`

func TestAMatrixDrawnByColumnAboveAndWholeBelow(t *testing.T) {
	lines := parse(t, mixedMatrixXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `matrice $\begin{pmatrix} u_1 & 0 \\ 0 & 0 \end{pmatrix}$ où`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 362 of Algebra VIII, the matrix of the endomorphism u of F[X] with
// respect to the basis (1, j), which the book prints as X-x, -gamma y bar over
// -y, X-x bar.
//
// Two things had to be right for this one to come out. The top row is written
// out as \gamma y, and the space that ends the name of the command is not white
// the page set: cut it there and the top row is three cells against the two of
// the row below, and a matrix whose rows are different lengths is no matrix and
// is left as it was. And the bar over the y of that row is drawn across a part
// of the run rather than across the whole of it, so nothing in the row is one
// the bar covers; it was passed over for the x of the row below, which is the
// next thing to the right of it and a row further down, and that x took both
// bars of the matrix and came out with two.
//
// The bar over the y is still missing here. It is drawn over one glyph of a run
// of two and there is nothing in the layer that says where the run divides, so
// the rule is dropped rather than guessed at.
func TestAMatrixWrittenWithACommandInIt(t *testing.T) {
	p := pdfsrc.Page{
		Number: 379, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 239, Left: 80, Width: 188, Height: 13, Text: "with respect to the basis (1"},
			{Top: 239, Left: 267, Width: 13, Height: 13, Font: 11, Text: ", j"},
			{Top: 239, Left: 281, Width: 22, Height: 13, Text: ") is"},
			{Top: 235, Left: 321, Width: 9, Height: 9, Font: 2, Text: "X"},
			{Top: 246, Left: 326, Width: 9, Height: 10, Font: 12, Text: "−"},
			{Top: 235, Left: 330, Width: 9, Height: 10, Font: 12, Text: "−"},
			{Top: 247, Left: 335, Width: 6, Height: 9, Font: 13, Text: "y"},
			{Top: 235, Left: 339, Width: 7, Height: 9, Font: 13, Text: "x"},
			{Top: 247, Left: 350, Width: 9, Height: 9, Font: 2, Text: "X"},
			{Top: 235, Left: 351, Width: 9, Height: 10, Font: 12, Text: "−"},
			{Top: 246, Left: 359, Width: 9, Height: 10, Font: 12, Text: "−"},
			{Top: 235, Left: 361, Width: 13, Height: 9, Font: 13, Text: "γy"},
			{Top: 247, Left: 368, Width: 7, Height: 9, Font: 13, Text: "x"},
			{Top: 239, Left: 387, Width: 191, Height: 13, Text: "; its determinant is equal to"},
		},
		Rules: []pdfsrc.Rule{
			{Top: 236, Left: 367, Width: 6, Thickness: 0.339, Length: 4.29, Size: 3.5},
			{Top: 247, Left: 368, Width: 7, Thickness: 0.339, Length: 4.51, Size: 3.5},
		},
	}
	const want = `with respect to the basis $(1, j)$ is $\begin{pmatrix} X-x & -\gamma y \\ -y & X-\overline{x} \end{pmatrix}$ ; its determinant is equal to`
	if got := sole(t, enlay, p); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// The French printing of the same page, which is page 340 of Algebre VIII, and
// which draws the parentheses of the matrix that the English printing leaves
// out of the layer altogether. They arrive out of CMEX as the runs "0" and "1",
// which are the codes of the two halves of a tall bracket, and they are set
// flush against the grid: the left one ends two units before the grid begins.
// A cluster that close to the run before it is a script of that run everywhere
// else on a page, so the grid was left as it was and the page shipped a matrix
// written as scripts inside a pair of parentheses. What tells the two apart is
// that a bracket is not a term a script hangs off.
func TestAMatrixSetFlushAgainstItsParentheses(t *testing.T) {
	p := pdfsrc.Page{
		Number: 357, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 188, Left: 315, Width: 144, Height: 12, Font: 21, Text: "par rapport à la base (1"},
			{Top: 188, Left: 459, Width: 12, Height: 12, Font: 22, Text: ", j"},
			{Top: 188, Left: 471, Width: 27, Height: 12, Font: 21, Text: ") est"},
			{Top: 173, Left: 503, Width: 7, Height: 15, Font: 17, Text: "0"},
			{Top: 185, Left: 512, Width: 7, Height: 8, Font: 18, Text: "X"},
			{Top: 192, Left: 516, Width: 8, Height: 12, Font: 19, Text: "−"},
			{Top: 182, Left: 519, Width: 8, Height: 12, Font: 19, Text: "−"},
			{Top: 195, Left: 523, Width: 5, Height: 8, Font: 20, Text: "y"},
			{Top: 185, Left: 527, Width: 6, Height: 8, Font: 20, Text: "x"},
			{Top: 195, Left: 536, Width: 7, Height: 8, Font: 18, Text: "X"},
			{Top: 182, Left: 537, Width: 8, Height: 12, Font: 19, Text: "−"},
			{Top: 192, Left: 543, Width: 8, Height: 12, Font: 19, Text: "−"},
			{Top: 185, Left: 545, Width: 10, Height: 8, Font: 20, Text: "γy"},
			{Top: 195, Left: 551, Width: 6, Height: 8, Font: 20, Text: "x"},
			{Top: 173, Left: 559, Width: 7, Height: 15, Font: 17, Text: "1"},
			{Top: 188, Left: 568, Width: 4, Height: 12, Font: 21, Text: ";"},
		},
		Rules: []pdfsrc.Rule{
			{Top: 185, Left: 550, Width: 5, Thickness: 0.360, Length: 3.48, Size: 3.7},
			{Top: 195, Left: 551, Width: 6, Thickness: 0.360, Length: 3.72, Size: 3.7},
		},
	}
	const want = `par rapport à la base $(1, j)$ est $\begin{pmatrix} X-x & -\gamma y \\ -y & X-\overline{x} \end{pmatrix}$ ;`
	if got := sole(t, frlay, p); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}

// alg448lay is what page 448 of the French Algebra VIII declares. The body of
// that page is set at 12 point and comes back at 14, one size up from the 10
// point body frlay carries, and the two script sizes under it at 9 and 7. The
// sizes are what the levels are read from, so the page gets its own layout
// rather than borrowing one measured off another printing.
var alg448lay = &pdfsrc.Layout{Fonts: map[int]pdfsrc.FontSpec{
	0: {ID: 0, Size: 14, Family: "LMRoman12"},
	1: {ID: 1, Size: 14, Family: "LMMathSymbols10"},
	2: {ID: 2, Size: 12, Family: "CMEX10"},
	3: {ID: 3, Size: 9, Family: "LMRoman8"},
	4: {ID: 4, Size: 7, Family: "LMMathItalic6"},
	5: {ID: 5, Size: 7, Family: "LMMathSymbols6"},
	6: {ID: 6, Size: 7, Family: "LMRoman6"},
}}

// Page 448 of the French Algebra VIII sets the homomorphism that embeds a two
// by two matrix as the bottom right block of a larger one, and it is the same
// flush parenthesis shape as the page above with one thing more: an entry that
// carries scripts of its own. I_{n-2} is drawn as the I at the level the matrix
// is set at with the n, the minus and the 2 a level deeper, so neither the
// column shape nor the row shape can read it and it falls to byInk.
//
// That is what makes it worth its own fixture. The first two shapes are written
// over the runs of one level and this one is not, and the parentheses have to
// be found on a cluster two levels deep just as they are on a cluster of one.
// Read as scripts instead, the page shipped U\rightarrow^{I_n}_{0^{-2}U}^0,
// which is not a matrix, is not what the page says, and is two superscripts
// against one base, so KaTeX refuses it by name and publish stops on the file.
func TestAMatrixWhoseEntriesCarryScriptsOfTheirOwn(t *testing.T) {
	p := pdfsrc.Page{
		Number: 448, Width: 659, Height: 999,
		Spans: []pdfsrc.Span{
			{Top: 168, Left: 80, Width: 10, Height: 12, Font: 0, Text: "U", Italic: true},
			{Top: 165, Left: 97, Width: 14, Height: 18, Font: 1, Text: "→"},
			{Top: 154, Left: 115, Width: 7, Height: 15, Font: 2, Text: "0"},
			{Top: 166, Left: 124, Width: 4, Height: 8, Font: 3, Text: "I"},
			{Top: 169, Left: 128, Width: 5, Height: 6, Font: 4, Text: "n"},
			{Top: 177, Left: 132, Width: 5, Height: 8, Font: 3, Text: "0"},
			{Top: 167, Left: 133, Width: 7, Height: 9, Font: 5, Text: "−"},
			{Top: 167, Left: 140, Width: 4, Height: 9, Font: 6, Text: "2"},
			{Top: 174, Left: 148, Width: 7, Height: 12, Font: 3, Text: "U", Italic: true},
			{Top: 166, Left: 150, Width: 5, Height: 8, Font: 3, Text: "0"},
			{Top: 154, Left: 159, Width: 7, Height: 15, Font: 2, Text: "1"},
			{Top: 168, Left: 171, Width: 24, Height: 12, Font: 0, Text: "de "},
		},
	}
	const want = `$U\rightarrow \begin{pmatrix} I_{n-2} & 0 \\ 0 & U \end{pmatrix}$ de`
	if got := sole(t, alg448lay, p); got != want {
		t.Errorf("got  %s\nwant %s", got, want)
	}
}
