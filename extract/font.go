package extract

import (
	"sort"
	"strings"
	"unicode"

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
	// ClassStrong is bold roman set as prose, which in this volume is a
	// heading, a citation number or the volume number of a journal. It comes
	// last so that the values above it keep the numbers they had.
	ClassStrong
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
	case ClassStrong:
		return "strong"
	}
	return "text"
}

// Math reports whether a class belongs inside dollar signs.
func (c Class) Math() bool { return c == ClassMath || c == ClassBold }

// families is every font family the corpus is set in and what a run in one of
// them is. Where the class is ClassText the run takes its class from the bold
// and italic flags instead, since a text face carries prose, emphasis and
// headings all three.
//
// It is a table and not a switch because of what a missing entry used to do.
// Anything not named here fell through to prose, silently, and the volumes set
// in Knuth's original Computer Modern rather than in Latin Modern are named
// CMMI and CMSY where the others are named LMMathItalic and LMMathSymbols. Lie
// chapters 7 to 9 has 31496 runs of mathematics italic and 14475 of the symbol
// font, and every one of them was being read as an English word: the extractor
// called the volume 100% clean and nothing anywhere said otherwise. A family
// that is not in this table now flags its page. See FlagUnknownFont.
//
// The two names for one design are both here, because both printings exist and
// the encodings are the same font: CM and LM are the same shapes at the same
// codes, drawn again in an outline format. Only the names differ.
var families = map[string]Class{
	// The text faces. Roman, italic, bold and the typewriter, in both the
	// Computer Modern and the Latin Modern namings, plus the faces a publisher
	// dropped in for a running head or a chapter title.
	"LMRoman": ClassText, "CMR": ClassText, "CMTI": ClassText, "CMBX": ClassText,
	"CMTT": ClassText, "CMSL": ClassText, "CMCSC": ClassText, "CMB": ClassText,
	"Times": ClassText, "TimesNewRoman": ClassText, "Times New Roman": ClassText,
	"TimesNewRomanPS": ClassText, "TimesNewRomanPSMT": ClassText,
	"SMinionPlus": ClassText, "Springnew": ClassText, "SFXC": ClassText,

	// Small capitals, which is what a statement head is set in.
	"LMRomanCaps": ClassHead,

	// The mathematics. The italic carries the variables, the symbol font the
	// relations and the operators, the extension font the large operators and
	// the large delimiters, and the rest are the alphabets a formula reaches
	// for: fraktur, script, blackboard bold and the sans of a functor.
	"LMMathItalic": ClassMath, "CMMI": ClassMath, "CMMIB": ClassMath,
	"LMMathSymbols": ClassMath, "CMSY": ClassMath, "CMBSY": ClassMath,
	"LMMathExtension": ClassMath, "CMEX": ClassMath,
	"MSAM": ClassMath, "MSBM": ClassMath,
	"rsfs": ClassMath, "EUFM": ClassMath, "EUSM": ClassMath, "EUEX": ClassMath,
	"BOUR": ClassMath, "CMSSDC": ClassMath, "LMSans": ClassMath,
	"TeX-mathx": ClassMath,

	// The pieces of a commutative diagram, drawn by xypic.
	"XYCMAT-Medium": ClassDiagram, "XYCMBT-Medium": ClassDiagram,
	"XYDASH-Medium": ClassDiagram, "XYATIP-Medium": ClassDiagram,
	"XYBTIP-Medium": ClassDiagram, "XYBSQL-Medium": ClassDiagram,
}

// stems is the keys of families, longest first, so that LMRomanCaps is read
// before LMRoman and TimesNewRomanPSMT before Times.
var stems = func() []string {
	out := make([]string, 0, len(families))
	for k := range families {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}()

// family is the entry in families that a font is, or the empty string for a
// font nobody has named yet.
//
// It matches the end of the name rather than the whole of it, because a subset
// tag is not always written the way the specification says. The specification
// wants six capitals and a plus, DGLNOH+CMSY7, which pdfsrc.FontSpec.Base cuts
// off. The Théories spectrales and Topologie algébrique files were merged by a
// tool that ran the tag straight into the name instead, four times over, and
// arrive as SnsrnnQjcxplMknsjpVdyrqvLMRoman10. Matching the end reads both, and
// the design size comes off first so that CMSY7 and CMSY10 are one font.
func family(f pdfsrc.FontSpec) string {
	name := f.Base()
	// A publisher's font carries its style in the name, after a comma the way
	// the specification writes it or run straight on: the running head of
	// Topologie algébrique is set in TimesNewRomanItalic and the title page
	// beside it in TimesNewRoman,Italic. The style is already known from the
	// flags pdftohtml reports, so it comes off and the name is read again.
	if i := strings.IndexByte(name, ','); i > 0 {
		name = name[:i]
	}
	if s := stemOf(name); s != "" {
		return s
	}
	// The style is written after a hyphen as often as it is run straight on, and
	// LMMathSymbols8-Regular is the same design as LMMathSymbols8.
	for _, style := range []string{"BoldItalic", "Italic", "Bold", "Regular"} {
		if rest, ok := strings.CutSuffix(name, style); ok {
			return stemOf(strings.TrimSuffix(rest, "-"))
		}
	}
	return ""
}

// stemOf is the entry in families that a font name ends with, once the design
// size has come off it, or the empty string for a name nothing matches.
func stemOf(name string) string {
	name = strings.TrimRight(name, "0123456789")
	for _, s := range stems {
		if len(name) >= len(s) && strings.EqualFold(name[len(name)-len(s):], s) {
			return s
		}
	}
	return ""
}

// KnownFont reports whether the tables have anything to say about a font. A run
// in a font they do not know is read as prose, which is right for a text face
// nobody has listed and wrong for a mathematics font, and there is no way to
// tell which from the name alone. So the page says so instead.
func KnownFont(f pdfsrc.FontSpec) bool { return family(f) != "" }

// Classify decides what a run is from its font and the italic and bold flags
// pdftohtml reports.
func Classify(f pdfsrc.FontSpec, s pdfsrc.Span) Class {
	if c, ok := families[family(f)]; ok && c != ClassText {
		return c
	}
	switch {
	case s.Bold && strong(s.Text):
		return ClassStrong
	case s.Bold:
		return ClassBold
	case s.Italic:
		return ClassEmph
	}
	return ClassText
}

// strong reports whether a bold run is prose rather than a symbol.
//
// The volume sets N, Z, Q, R, C and the classical groups in bold roman, and it
// sets every heading in bold roman as well, so the font does not say which of
// the two a run is. Its shape does. All 345 distinct bold runs of the volume
// were dumped and read: every symbol among them is at most three letters long
// and is nothing but letters (Z 271 times, M 196, GL 71, SL 33, PGL 3, Sp, pq,
// pp, N, Q, A, C, R, α), and everything longer or carrying a digit or a mark of
// punctuation is a heading, a piece of one, a citation number like the 51 of
// ([51], p. 102) or the volume number of a journal in the bibliography.
//
// Getting this wrong is not a small matter of style. A heading classified as a
// symbol goes inside dollar signs a letter at a time, and page 213 shipped its
// title as \mathbf{M}\mathbf{u}\mathbf{l}\mathbf{t}\mathbf{i} and so on for the
// whole line.
func strong(s string) bool {
	n := 0
	for _, r := range strings.TrimSpace(s) {
		if !unicode.IsLetter(r) {
			return true
		}
		n++
	}
	return n > 3
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

// cmexOps is the LaTeX every large operator is written as, which is what tells
// an operator from a delimiter or an accent drawn out of the same font. TeX
// sets limits over and under an operator and nothing over or under a bracket,
// so this is the list of glyphs that can have a limit written across them.
var cmexOps = func() map[string]bool {
	m := make(map[string]bool, len(cmex))
	for _, s := range cmex {
		m[s] = true
	}
	return m
}()

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

// combining is the accents that arrive as a combining character rather than as
// a letter of the extension font. Some are drawn out of the AMS symbol fonts,
// where a letter of CMEX read in another font is only a letter, and some are
// drawn out of CMEX itself: preparing a volume names both of them after the
// accent they are, so a hat that reached this file as the letter b in the
// English printing and as nothing at all in the French now reaches it as
// U+0302 in either. See pdfglyph.
var combining = map[rune]string{
	'̂': `\widehat`,
	'̃': `\widetilde`,
	// U+02C6, the circumflex that takes a width of its own instead of
	// combining. It is what the French printing sets the hat of a Fourier
	// transform in, out of the text face rather than out of an extension font,
	// and it arrives welded to the run beside it. See unhat, which cuts it out
	// before this is asked about it.
	'ˆ': `\widehat`,
}

// Accent returns the LaTeX for a run that is an accent drawn over another run,
// whichever font it was set in. An accent is not written where it is found: the
// caller has to put it over the run it covers.
func Accent(f pdfsrc.FontSpec, text string) (string, bool) {
	r := first(text)
	// The accent itself is read before the font's own code page, since a
	// prepared volume says U+0302 where the font said b, and the b is only a
	// hat by the convention of a font nobody can see.
	if latex, ok := combining[r]; ok {
		return latex, true
	}
	if Extension(f) {
		latex, acc, ok := CMEX(r)
		return latex, ok && acc
	}
	return "", false
}

// Extension reports whether a font is the mathematics extension font, which
// TeX calls CMEX10 and its outline redrawing calls LMMathExtension10. It is the
// one font read entirely by position, so the two names have to answer alike.
func Extension(f pdfsrc.FontSpec) bool {
	switch family(f) {
	case "CMEX", "LMMathExtension":
		return true
	}
	return false
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
	// The last row arrives only from the French printings, and only once their
	// glyph names have been rewritten to names poppler can resolve. See
	// pdfglyph: the script ell of a lattice, the real and imaginary parts, the
	// boxed times of a tensor product of representations, the natural sign the
	// series uses for a canonical map and the complement of a subset were all
	// coming out as nothing at all.
	'ℓ': `\ell`, 'ℜ': `\Re`, 'ℑ': `\Im`,
	'⊠': `\boxtimes`, '♮': `\natural`, '∁': `\complement`,
}

// PUA is the Private Use Area block poppler puts the pieces of a delimiter that
// spans several lines into. A left parenthesis three lines tall is drawn as a
// top, some number of extensions and a bottom, and the pieces arrive as three
// separate runs on three separate lines. They cannot be put back together from
// one line, so a page carrying any of them is flagged.
func PUA(r rune) bool { return r >= 0xF8EB && r <= 0xF8FF }
