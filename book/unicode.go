package book

import (
	"maps"
	"strings"
)

// The corpus is Unicode text and the book is set with XeTeX, so most of it goes
// through untouched: Latin Modern covers Latin-1, Latin Extended and the whole
// of Vietnamese, which was the one thing worth checking before choosing it.
//
// What it does not cover is the mathematics that leaked into the prose. 577
// alphas, 1560 elements-of signs, 533 less-than-or-equals, and another two
// hundred symbols besides, all of them outside a math span, all of them written
// by an OCR that read a formula in a sentence and set it as text. Every one is a
// character XeTeX cannot find a glyph for, and the way that fails is a silent
// hole in the page with a warning buried in a log nobody reads.
//
// The right repair is in the corpus, and it is a large one: those characters
// should be inside dollars. Until somebody does that, this table sets them, and
// it sets them as mathematics rather than as text because that is what they are.
// A sentence that says "for all α in A" gets the alpha the formula would get.
//
// The table is deliberately not a general Unicode-to-TeX table. It is the
// characters this corpus actually holds, counted, so that a character nobody
// checked cannot be set to something nobody looked at.

// Text is the prose of the corpus with the characters Latin Modern cannot set
// turned into TeX. It runs on already-escaped text, so what it writes must be
// TeX and not more prose.
func Text(s string) string {
	if !hasNonASCII(s) {
		return s
	}
	var b strings.Builder
	for _, r := range s {
		if r < 128 {
			b.WriteRune(r)
			continue
		}
		if tex, ok := texRune[r]; ok {
			b.WriteString(tex)
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// Missing reports the characters in a string that neither the table nor Latin
// Modern can set, which is what the audit prints. The set is what the corpus has
// besides Latin and the mathematics: the Devanagari, Arabic, Cyrillic, Armenian,
// Hangul and CJK the history volume quotes, and a handful of OCR noise.
//
// It is a report and not a repair. Dropping the character would lose text and
// substituting one would invent it, so the build sets what it can, says what it
// could not, and leaves the decision to somebody with the page open.
func Missing(s string) []rune {
	var out []rune
	seen := map[rune]bool{}
	for _, r := range s {
		if r < 128 || seen[r] {
			continue
		}
		if _, ok := texRune[r]; ok {
			continue
		}
		if latin(r) {
			continue
		}
		seen[r] = true
		out = append(out, r)
	}
	return out
}

func hasNonASCII(s string) bool {
	for _, r := range s {
		if r > 127 {
			return true
		}
	}
	return false
}

// latin is the ranges Latin Modern sets: Latin-1, Latin Extended A and B, the
// combining marks Vietnamese is written with, the spacing modifiers, and General
// Punctuation. Latin Extended Additional is where the Vietnamese tone letters
// live and it is the range that decided the font.
func latin(r rune) bool {
	switch {
	case r >= 0x00A0 && r <= 0x024F: // Latin-1, Extended A, Extended B
		return true
	case r >= 0x02B0 && r <= 0x02FF: // spacing modifiers, the loose tilde and circumflex
		return true
	case r >= 0x0300 && r <= 0x036F: // combining marks
		return true
	case r >= 0x1E00 && r <= 0x1EFF: // Latin Extended Additional, the Vietnamese
		return true
	case r >= 0x2010 && r <= 0x2027: // dashes and quotes
		return true
	case r >= 0x20A0 && r <= 0x20BF: // currency, one euro sign in a modern reference
		return true
	}
	return false
}

// math wraps a symbol so it sets in a formula inside a sentence.
func math(tex string) string { return "$" + tex + "$" }

// textArgument is the commands inside a formula whose argument is prose. Their
// contents are left alone by Math, because a Greek letter inside \text is a
// Greek letter in a Vietnamese sentence and \lambda there is an error rather
// than a lambda.
var textArgument = map[string]bool{
	"text": true, "textbf": true, "textit": true, "textrm": true,
	"textsf": true, "mbox": true, "emph": true,
}

// Math is Text for the inside of a formula.
//
// The corpus has 26061 non-ASCII characters inside its math spans. Most are
// Vietnamese inside \text and belong there. The rest are the ones that made the
// typesetter say "Missing character: There is no lambda in cmmi10": a Greek
// letter or a set sign that was written as the character rather than as the
// command, which KaTeX draws from a web font and which the math fonts of a
// printed book do not have at all. The character is right and the encoding is
// wrong, so it is turned back into the command it means.
func Math(s string) string {
	if !hasNonASCII(s) {
		return s
	}
	rs := []rune(s)
	var b strings.Builder
	for i := 0; i < len(rs); i++ {
		r := rs[i]
		if r == '\\' {
			name, j := controlName(rs, i)
			if textArgument[name] {
				arg, k := group(rs, j)
				if k > j {
					b.WriteString(`\` + name + "{" + arg + "}")
					i = k - 1
					continue
				}
			}
			b.WriteString(string(rs[i:max(j, i+1)]))
			i = max(j, i+1) - 1
			continue
		}
		if r < 128 {
			b.WriteRune(r)
			continue
		}
		if r == '°' {
			b.WriteString(degree(b.String()))
			continue
		}
		if tex, ok := mathRune[r]; ok {
			b.WriteString(tex)
			// A command has to be told where it stops. The corpus writes the
			// letter next to whatever follows it, so "ωy" becomes \omega and
			// then a y, and \omegay is not a command anybody has heard of.
			if endsInLetter(tex) && i+1 < len(rs) && isLetter(rs[i+1]) {
				b.WriteByte(' ')
			}
			continue
		}
		b.WriteRune(r)
	}
	return b.String()
}

// degree is the TeX for a degree sign, given what has been written before it.
//
// The sign is a raised circle and the corpus writes it in both of the places a
// raised circle can go. The polar of a set is A°, where the superscript has to
// be opened. The bipolar is A^{°°} and the polar of a union is (A° \cup U°)^°,
// where one is open already. Written as ^\circ every time, those two come out
// as A^{^\circ^\circ} and )^^\circ, and TeX stops on a double superscript. It
// stopped both of the volumes of Topological Vector Spaces.
func degree(before string) string {
	t := strings.TrimRight(before, " ")
	if strings.HasSuffix(t, "^") || strings.HasSuffix(t, "^{") || strings.HasSuffix(t, `\circ`) {
		return `\circ`
	}
	return `^\circ`
}

func endsInLetter(s string) bool {
	return s != "" && isLetter(rune(s[len(s)-1]))
}

// mathRune is the same table as texRune with the dollars taken off, plus the
// few characters that mean one thing in a sentence and another in a formula.
var mathRune = func() map[rune]string {
	m := map[rune]string{}
	for _, t := range []map[rune]string{greek, operators, letters, scripts} {
		maps.Copy(m, t)
	}
	maps.Copy(m, map[rune]string{
		'’': `'`, '‘': `'`, '§': `\S`, '°': `^\circ`, ' ': ` `,
		'₀': `_0`, '₁': `_1`, '₂': `_2`, '₃': `_3`, '₄': `_4`,
		'₅': `_5`, '₆': `_6`, '₇': `_7`, '₈': `_8`, '₉': `_9`,
		'₊': `_+`, '₋': `_-`, '₌': `_=`, '₍': `_(`, '₎': `_)`,
		'⁰': `^0`, '¹': `^1`, '²': `^2`, '³': `^3`, '⁴': `^4`,
		'⁵': `^5`, '⁶': `^6`, '⁷': `^7`, '⁸': `^8`, '⁹': `^9`,
		'⁺': `^+`, '⁻': `^-`, '⁼': `^=`, '⁽': `^(`, '⁾': `^)`,
	})
	return m
}()

// texRune is the table. Every entry was counted in the corpus before it was
// written down, which is why it stops where it does.
var texRune = func() map[rune]string {
	m := map[rune]string{}
	for r, tex := range greek {
		m[r] = math(tex)
	}
	for r, tex := range operators {
		m[r] = math(tex)
	}
	for r, tex := range letters {
		m[r] = math(tex)
	}
	for r, tex := range scripts {
		m[r] = math(tex)
	}
	maps.Copy(m, marks)
	return m
}()

// greek is the Greek of the Elements. Both cases, because a capital Latin
// Modern happens to have is still the wrong shape beside a lower case one it
// does not: Gamma set as text and gamma set as mathematics in the same sentence
// do not look like the same alphabet.
var greek = map[rune]string{
	'α': `\alpha`, 'β': `\beta`, 'γ': `\gamma`, 'δ': `\delta`, 'ε': `\varepsilon`,
	'ζ': `\zeta`, 'η': `\eta`, 'θ': `\theta`, 'ι': `\iota`, 'κ': `\kappa`,
	'λ': `\lambda`, 'μ': `\mu`, 'µ': `\mu`, 'ν': `\nu`, 'ξ': `\xi`, 'ο': `o`,
	'π': `\pi`, 'ρ': `\rho`, 'ς': `\varsigma`, 'σ': `\sigma`, 'τ': `\tau`,
	'υ': `\upsilon`, 'φ': `\varphi`, 'ϕ': `\phi`, 'χ': `\chi`, 'ψ': `\psi`,
	'ω': `\omega`, 'ϰ': `\varkappa`, 'ϲ': `\varsigma`,
	'Γ': `\Gamma`, 'Δ': `\Delta`, 'Θ': `\Theta`, 'Λ': `\Lambda`, 'Ξ': `\Xi`,
	'Π': `\Pi`, 'Σ': `\Sigma`, 'Υ': `\Upsilon`, 'Φ': `\Phi`, 'Ψ': `\Psi`,
	'Ω': `\Omega`,
	// The accented Greek is Greek quoted as Greek rather than used as a symbol,
	// in the history volume. It is set as the letter without the accent, which
	// loses the breathing and keeps the word legible, and the audit says so.
	'ἀ': `\alpha`, 'ἰ': `\iota`, 'ὴ': `\eta`, 'ό': `o`, 'ί': `\iota`, 'ύ': `\upsilon`,
	'ϭ': `\sigma`, 'Ϭ': `\Sigma`,
}

// operators is the relations, the arrows and the set signs.
var operators = map[rune]string{
	'∈': `\in`, '∉': `\notin`, '⊂': `\subset`, '⊃': `\supset`,
	'∩': `\cap`, '∪': `\cup`, '⋂': `\bigcap`, '⋃': `\bigcup`,
	'≤': `\leqslant`, '≥': `\geqslant`, '≠': `\neq`, '≡': `\equiv`, '≰': `\nleqslant`,
	'×': `\times`, '−': `-`, '⊗': `\otimes`, '⊕': `\oplus`, '∘': `\circ`, '◦': `\circ`,
	'⋅': `\cdot`, '∅': `\emptyset`, '∞': `\infty`, '∂': `\partial`,
	'∑': `\sum`, '∏': `\prod`, '∫': `\int`, '∧': `\wedge`, '∨': `\vee`,
	'⊔': `\sqcup`, '⊓': `\sqcap`, '±': `\pm`,
	'→': `\to`, '←': `\leftarrow`, '↔': `\leftrightarrow`, '↦': `\mapsto`,
	'⇒': `\Rightarrow`, '⇔': `\Leftrightarrow`, '↑': `\uparrow`, '↓': `\downarrow`,
	'↼': `\leftharpoonup`, '⇌': `\rightleftharpoons`,
	'⟨': `\langle`, '⟩': `\rangle`, '‖': `\|`, '′': `{}'`, '″': `{}''`,
	'⋯': `\cdots`, '■': `\blacksquare`, '•': `\bullet`,
}

// letters is the sub and superscript figures, which an OCR writes when the
// printing sets an index and the reader did not put it in a formula.
var letters = map[rune]string{
	'₀': `_0`, '₁': `_1`, '₂': `_2`, '₃': `_3`, '₄': `_4`, '₆': `_6`, '₇': `_7`, '₈': `_8`,
	'₊': `_+`, '₋': `_-`,
	'⁰': `^0`, '¹': `^1`, '²': `^2`, '³': `^3`, '⁴': `^4`, '⁶': `^6`, '⁷': `^7`,
	'⁸': `^8`, '⁹': `^9`, '⁺': `^+`, '⁻': `^-`,
	'ₐ': `_a`, 'ₑ': `_e`, 'ᵢ': `_i`, 'ⱼ': `_j`, 'ₖ': `_k`, 'ₗ': `_l`, 'ₘ': `_m`,
	'ₙ': `_n`, 'ₚ': `_p`, 'ᵣ': `_r`, 'ₛ': `_s`, 'ᵥ': `_v`, 'ₓ': `_x`,
	'ⁿ': `^n`, 'ʳ': `^r`, 'ᵉ': `^e`,
}

// scripts is the blackboard, script and fraktur letters. They are a real part
// of the notation and not OCR noise: a set is written blackboard bold and a Lie
// algebra fraktur, and a reader who gets a plain R where the book has a
// blackboard one is reading a different sentence.
var scripts = map[rune]string{
	'ℝ': `\mathbf{R}`, 'ℕ': `\mathbf{N}`, 'ℤ': `\mathbf{Z}`, '𝔽': `\mathbf{F}`,
	'𝔾': `\mathbf{G}`, 'ℵ': `\aleph`, 'ℓ': `\ell`,
	'ℒ': `\mathcal{L}`, 'ℋ': `\mathcal{H}`,
	'𝒞': `\mathcal{C}`, '𝒟': `\mathcal{D}`, '𝒢': `\mathcal{G}`, '𝒥': `\mathcal{J}`,
	'𝒦': `\mathcal{K}`, '𝒫': `\mathcal{P}`, '𝒬': `\mathcal{Q}`, '𝒮': `\mathcal{S}`,
	'𝒯': `\mathcal{T}`, '𝓗': `\mathcal{H}`, '𝓛': `\mathcal{L}`, '𝓜': `\mathcal{M}`,
	'𝔅': `\mathfrak{B}`, '𝔉': `\mathfrak{F}`, '𝔖': `\mathfrak{S}`,
	'𝔤': `\mathfrak{g}`, '𝔥': `\mathfrak{h}`, '𝔰': `\mathfrak{s}`,
}

// marks is what stays in text: the sign the Elements warns with, the spaces an
// OCR writes that TeX has its own names for, and the numero sign.
//
// The dangerous bend is the one that matters. Bourbaki sets a road sign in the
// margin against a passage a reader can go badly wrong in, and the corpus holds
// 35 of them as U+2621. Almost no text font has that character, and Knuth's own
// manual font has the sign, which is why the class loads manfnt for one glyph.
var marks = map[rune]string{
	'☡': `\dbend{}`,
	'№': `\textnumero{}`,
	' ': `~`,
	' ': `\,`,
	' ': `\quad{}`,
	'　': `\quad{}`,
	'­': ``,
	'‑': `-`,
}
