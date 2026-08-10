package extract

import (
	"strings"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// What a run of text is depends on the font it was set in, and the 2023 volume
// carries its whole font stack in the file. That turns the two questions this
// package has to answer into lookups. Is this run mathematics: yes if it is set
// in a mathematics font. What is this stranded capital P: a summation sign, if
// it is set in CMEX10.
//
// The tables below were built by dumping every distinct run of every font in
// the volume with bourbaki extract fonts, and each of the ones that could not
// be read off the encoding was checked against the printed page.

// Class is what a run of text is.
type Class int

const (
	// ClassText is upright roman prose.
	ClassText Class = iota
	// ClassEmph is italic roman, which in Bourbaki is the statement of a
	// theorem or a definition rather than emphasis.
	ClassEmph
	// ClassHead is small capitals, which is what a statement head is set in.
	ClassHead
	// ClassBold is bold roman, which is how Bourbaki sets N, Z, Q, R and C.
	ClassBold
	// ClassMath is a mathematics font.
	ClassMath
	// ClassDiagram is a piece of a commutative diagram drawn by xypic. The
	// pieces cannot be reassembled from their positions, so a page carrying
	// any of them is flagged rather than guessed at.
	ClassDiagram
)

func (c Class) String() string {
	switch c {
	case ClassEmph:
		return "emph"
	case ClassHead:
		return "head"
	case ClassBold:
		return "bold"
	case ClassMath:
		return "math"
	case ClassDiagram:
		return "diagram"
	}
	return "text"
}

// Math reports whether a class belongs inside dollar signs.
func (c Class) Math() bool { return c == ClassMath || c == ClassBold }

// family is a font family with its size stripped, so that LMRoman10 and
// LMRoman7 are both "LMRoman" and CMEX10 and CMEX7 are both "CMEX".
func family(f pdfsrc.FontSpec) string {
	return strings.TrimRight(f.Base(), "0123456789")
}

// Classify decides what a run is from its font and the italic and bold flags
// pdftohtml reports.
func Classify(f pdfsrc.FontSpec, s pdfsrc.Span) Class {
	switch family(f) {
	case "LMRomanCaps":
		return ClassHead
	case "LMMathItalic", "LMMathSymbols", "CMEX", "MSAM", "MSBM", "rsfs", "EUFM", "BOUR", "CMSSDC":
		return ClassMath
	case "XYCMAT-Medium", "XYCMBT-Medium":
		return ClassDiagram
	}
	switch {
	case s.Bold:
		return ClassBold
	case s.Italic:
		return ClassEmph
	}
	return ClassText
}

// cmex maps what poppler prints for a CMEX glyph to the operator it is.
//
// CMEX is the TeX mathematics extension font, and poppler has no Unicode for
// most of it, so it prints the character whose code the glyph sits at. That is
// not a loss: the code is the encoding, and the encoding is fixed. Position
// 0x50 is a summation sign and poppler prints "P", position 0x4C is a direct
// sum and poppler prints "L". Each of these was checked against the page it
// came from before it went in the table.
//
// The large operators come in two sizes, one for text and one for a display,
// which is why the entries come in pairs. The size is not recorded because the
// LaTeX is the same either way.
var cmex = map[rune]string{
	'F': `\bigsqcup`, 'G': `\bigsqcup`,
	'J': `\bigodot`, 'K': `\bigodot`,
	'L': `\bigoplus`, 'M': `\bigoplus`,
	'N': `\bigotimes`, 'O': `\bigotimes`,
	'P': `\sum`, 'X': `\sum`,
	'Q': `\prod`, 'Y': `\prod`,
	'R': `\int`, 'Z': `\int`,
	'S': `\bigcup`, '[': `\bigcup`,
	'T': `\bigcap`, '\\': `\bigcap`,
	'U': `\biguplus`, ']': `\biguplus`,
	'V': `\bigwedge`, '^': `\bigwedge`,
	'W': `\bigvee`, '_': `\bigvee`,
	'`': `\coprod`, 'a': `\coprod`,
}

// cmexAccent maps the accent glyphs, which are the three widths CMEX carries of
// each. An accent is drawn over the run it decorates rather than beside it, so
// it is not written where it is found.
var cmexAccent = map[rune]string{
	'b': `\widehat`, 'c': `\widehat`, 'd': `\widehat`,
	'e': `\widetilde`, 'f': `\widetilde`, 'g': `\widetilde`,
}

// cmexDelim is the delimiter block of CMEX, which repeats every 16 positions at
// four sizes. Only the two largest arrive: poppler drops the two smallest
// because their codes are control characters, which is what the dropped
// delimiter flag is for.
var cmexDelim = [16]string{`(`, `)`, `[`, `]`, `\lfloor`, `\rfloor`, `\lceil`, `\rceil`,
	`\{`, `\}`, `\langle`, `\rangle`, `|`, `\|`, `/`, `\backslash`}

// CMEX returns the LaTeX for one CMEX glyph. accent is set when the glyph is an
// accent, which the caller has to place over its base rather than beside it.
func CMEX(r rune) (latex string, accent, ok bool) {
	if s, k := cmexAccent[r]; k {
		return s, true, true
	}
	if s, k := cmex[r]; k {
		return s, false, true
	}
	if r >= 0x20 && r < 0x40 {
		return cmexDelim[r&0x0F], false, true
	}
	return "", false, false
}

// symbolPairs are the places where TeX draws one symbol out of two glyphs and
// poppler prints both. A hook and an arrow is a mapsto, a solidus and a
// relation is that relation negated, three centred dots are an ellipsis.
// pdftotext prints them as "7→" and "6=" and leaves the reader to work it out.
//
// They are applied in order and before the single glyphs, since "7→" has to be
// read before the arrow in it is.
var symbolPairs = []struct{ from, to string }{
	{"7−→", `\longmapsto `},
	{"7→", `\mapsto `},
	{"−−−→", `\longrightarrow `},
	{"−−→", `\longrightarrow `},
	{"−→", `\longrightarrow `},
	{"6=", `\neq `},
	{"6∈", `\notin `},
	{"6⊂", `\not\subset `},
	{"6⊃", `\not\supset `},
	{"6|", `\nmid `},
	{"· · ·", `\cdots `},
	{"·· ·", `\cdots `},
	{"· ··", `\cdots `},
}

// cmsy is the part of the mathematics symbol font poppler could not name. It
// prints the character whose code the glyph sits at, exactly as it does for
// CMEX, so the code is again the encoding. Position 0x30 is a prime and comes
// out as "0", which is why the text layer of this volume calls a submodule M′
// by the name Mi0.
var cmsy = map[rune]string{
	'{':  `\{`,
	'}':  `\}`,
	'0':  `'`,
	'6':  `\not`,
	'h':  `\langle `,
	'i':  `\rangle `,
	'\\': `\backslash `,
}

// unicodeMath maps the characters the mathematics fonts do come out as to
// LaTeX. Leaving them as Unicode would read well and would not compile, and the
// corpus is Markdown with LaTeX mathematics rather than Markdown with symbols
// in it.
var unicodeMath = map[rune]string{
	'∈': `\in`, '∋': `\ni`, '⊗': `\otimes`, '⊕': `\oplus`, '⊖': `\ominus`,
	'−': `-`, '→': `\rightarrow`, '←': `\leftarrow`, '↔': `\leftrightarrow`,
	'⇒': `\Rightarrow`, '⇐': `\Leftarrow`, '⇔': `\Leftrightarrow`,
	'◦': `\circ`, '·': `\cdot`, '×': `\times`, '±': `\pm`, '∓': `\mp`,
	'⊂': `\subset`, '⊃': `\supset`, '⊆': `\subseteq`, '⊇': `\supseteq`,
	'∩': `\cap`, '∪': `\cup`, '∧': `\wedge`, '∨': `\vee`, '∗': `*`,
	'⩽': `\leqslant`, '⩾': `\geqslant`, '≤': `\leq`, '≥': `\geq`, '≠': `\neq`,
	'≡': `\equiv`, '≃': `\simeq`, '∼': `\sim`, '≈': `\approx`, '∅': `\emptyset`,
	'∞': `\infty`, '∀': `\forall`, '∃': `\exists`, '¬': `\neg`, '∇': `\nabla`,
	'∂': `\partial`, '√': `\surd`, '∐': `\amalg`, '⊥': `\bot`, '⊤': `\top`,
	'⌊': `\lfloor`, '⌋': `\rfloor`, '⌈': `\lceil`, '⌉': `\rceil`,
	'⟨': `\langle`, '⟩': `\rangle`, '‖': `\|`, '†': `\dagger`, '¶': `\P`,
	'α': `\alpha`, 'β': `\beta`, 'γ': `\gamma`, 'δ': `\delta`, 'ε': `\varepsilon`,
	'ζ': `\zeta`, 'η': `\eta`, 'θ': `\theta`, 'ι': `\iota`, 'κ': `\kappa`,
	'λ': `\lambda`, 'μ': `\mu`, 'ν': `\nu`, 'ξ': `\xi`, 'π': `\pi`,
	'ρ': `\rho`, 'σ': `\sigma`, 'τ': `\tau`, 'υ': `\upsilon`, 'ϕ': `\varphi`,
	'φ': `\varphi`, 'χ': `\chi`, 'ψ': `\psi`, 'ω': `\omega`,
	'Γ': `\Gamma`, 'Δ': `\Delta`, 'Θ': `\Theta`, 'Λ': `\Lambda`, 'Ξ': `\Xi`,
	'Π': `\Pi`, 'Σ': `\Sigma`, 'Υ': `\Upsilon`, 'Φ': `\Phi`, 'Ψ': `\Psi`, 'Ω': `\Omega`,
	'′': `'`, '‵': `'`, '⋆': `\star`, '•': `\bullet`, '⊙': `\odot`, '⊘': `\oslash`,
	'□': `\square`, '△': `\triangle`, '≺': `\prec`, '≻': `\succ`, '≪': `\ll`, '≫': `\gg`,
	'⊔': `\sqcup`, '⊓': `\sqcap`, '⊢': `\vdash`, '⊣': `\dashv`, '∤': `\nmid`,
}

// PUA is the Private Use Area block poppler puts the pieces of a delimiter that
// spans several lines into. A left parenthesis three lines tall is drawn as a
// top, some number of extensions and a bottom, and the pieces arrive as three
// separate runs on three separate lines. They cannot be put back together from
// one line, so a page carrying any of them is flagged.
func PUA(r rune) bool { return r >= 0xF8EB && r <= 0xF8FF }
