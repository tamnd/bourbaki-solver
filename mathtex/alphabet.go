package mathtex

import "strings"

// Alphabet puts a Mathematical Alphanumeric Symbols character back into the TeX
// that prints it, and puts a math span around what it finds.
//
// These are the script and fraktur letters of the prose: 𝒫 for \mathscr{P},
// 𝔤 for \mathfrak{g}, 𝔅 for \mathfrak{B}. They arrive because the extractor
// meets a letter set in a display face it has no font table entry for and
// reaches for the Unicode codepoint that draws it, and they all land outside
// the mathematics, because a run the extractor reads as prose is written as
// prose. So no M rule sees them: M03 reads inside the spans and #377 looks for
// a backslash, and there is no backslash here.
//
// The corpus decides the spelling and there is nothing to weigh. It writes
// \mathscr 108584 times and \mathfrak 78814, and \mathcal and \mathbb not once.
// The double-struck letters go to \mathbf, which is M02's answer: Bourbaki sets
// Z, Q, R, C and N in bold and not in blackboard bold.
//
// A few of them land inside the mathematics instead, where a bare ℓ is not TeX
// at all and the run will not set. Those are simply spelled, with no delimiters
// to add and no argument to weigh.
//
// The span is the part that needs care. A bare \mathscr{P} dropped into a
// sentence is the fault of #377 rather than a repair of this one, so the
// delimiters go on with it, and the argument comes inside them when it is
// plainly the letter's: 𝒦(G) becomes $\mathscr{K}(G)$ and not $\mathscr{K}$(G).
// Plainly is a narrow word here. A parenthesis is taken only when it balances
// on the same line and holds nothing but the ASCII a formula is written with,
// so 𝓛(E_0, M) comes in whole and ℓ_∞^∞(G) does not, because pulling a bare ∞
// inside a span would hand M03 a character stranded in the mathematics and
// trade one fault for another.
//
// Two letters are refused. The double-struck F and G of this corpus are not a
// font question at all: every 𝔽 is a misread \mathscr{B}, which the Borel tribe
// of int-ix-fr 0045 sets twice on one page and spells both ways, and the 𝔾 and
// the 𝔖 of top-i-iv 0262 are one ultrafilter written two ways, one of which is
// wrong and only the printed page of General Topology III, § 4 says which. A
// repair that mapped them by their block would write the wrong letter in a
// correct font, which is worse than leaving them where a reader can find them.
//
// See tamnd/bourbaki#399.
func Alphabet(body string) (string, int, []Refusal) {
	spans, unclosed := Split(body)
	rs := []rune(body)
	var b strings.Builder
	var refused []Refusal
	n, at := 0, 0
	stop := len(rs)
	if unclosed != nil {
		// Repair's reason: past an unclosed delimiter there is no telling prose
		// from mathematics, so nothing after it is touched.
		stop = unclosed.Start
	}
	line := 1
	for i := 0; i < stop; i++ {
		if rs[i] == '\n' {
			line++
			continue
		}
		tex, ok := alphabetTeX(rs[i])
		if !ok {
			if why, held := alphabetHeld[rs[i]]; held {
				refused = append(refused, Refusal{Line: line, Rune: rs[i],
					Why: why, Span: lineAround(rs, i)})
			}
			continue
		}
		if inSpan(spans, i) {
			// Already mathematics, so the delimiters are there and the argument
			// needs no vouching: the glyph is simply spelled. A command that
			// ends in a letter rather than a brace runs on into a letter after
			// it, so one space keeps them apart.
			if plainLetter(rune(tex[len(tex)-1])) && i+1 < stop && plainLetter(rs[i+1]) {
				tex += " "
			}
			b.WriteString(string(rs[at:i]))
			b.WriteString(tex)
			at, n = i+1, n+1
			continue
		}
		end := alphabetArgument(rs, i+1, stop)
		b.WriteString(string(rs[at:i]))
		b.WriteString("$" + tex + string(rs[i+1:end]) + "$")
		at, n = end, n+1
		i = end - 1
	}
	if n == 0 {
		return body, 0, refused
	}
	b.WriteString(string(rs[at:]))
	return b.String(), n, refused
}

// InAlphabet reports whether r is one of the display-face letters Alphabet is
// about: the Mathematical Alphanumeric Symbols block, and the holes in it that
// Letterlike Symbols had first. The whole block is reported and not only the
// part that maps, because a bold italic x or a sans-serif digit is the same
// fault arriving in a different font and the extractor should emit none of them.
func InAlphabet(r rune) bool {
	if r >= 0x1D400 && r <= 0x1D7FF {
		return true
	}
	_, _, ok := alphabetHole(r)
	return ok
}

// AlphabetTeX is the TeX that prints r, when Alphabet can say what it is.
func AlphabetTeX(r rune) (string, bool) { return alphabetTeX(r) }

// alphabetArgument is where the letter's own argument stops, given that the
// letter ends at i. It takes the accents that sit over the letter, then one
// parenthesised group, then one subscript and one superscript, and stops at the
// first thing it cannot vouch for.
func alphabetArgument(rs []rune, i, stop int) int {
	for i < stop && combining(rs[i]) {
		i++
	}
	if i < stop && rs[i] == '(' {
		if end, ok := plainGroup(rs, i, stop, '(', ')'); ok {
			i = end
		}
	}
	// One subscript and one superscript, in whichever order they are written.
	// The corpus writes 𝒞^r_b and it writes 𝒟_{n}^{p}, and TeX reads both.
	taken := map[rune]bool{}
	for range 2 {
		if i+1 >= stop || rs[i] != '_' && rs[i] != '^' || taken[rs[i]] {
			break
		}
		mark := rs[i]
		switch {
		case rs[i+1] == '{':
			end, ok := plainGroup(rs, i+1, stop, '{', '}')
			if !ok {
				return i
			}
			i = end
		case plainRune(rs[i+1]):
			i += 2
		default:
			return i
		}
		taken[mark] = true
	}
	return i
}

// plainGroup is the end of a balanced group that opens at i, when the group
// closes on the same line and holds nothing but what a formula is written with.
func plainGroup(rs []rune, i, stop int, open, close rune) (int, bool) {
	depth := 0
	for j := i; j < stop; j++ {
		switch {
		case rs[j] == '\n':
			return 0, false
		case rs[j] == open:
			depth++
		case rs[j] == close:
			if depth--; depth == 0 {
				return j + 1, true
			}
		case !plainRune(rs[j]):
			return 0, false
		}
	}
	return 0, false
}

// plainRune is a character that can go inside a span without asking anything of
// anybody: the ASCII a formula is written with, and nothing else. A letter
// outside it may be Greek that belongs in TeX of its own (#377), or an accent,
// or another of the letters this file is about, and each of those is a reading
// rather than a substitution.
func plainRune(r rune) bool {
	switch {
	case r >= 'a' && r <= 'z', r >= 'A' && r <= 'Z', r >= '0' && r <= '9':
		return true
	case r == ' ', r == ',', r == '.', r == '\'', r == '+', r == '-', r == '*',
		r == '/', r == '=', r == '<', r == '>', r == ':', r == ';', r == '!',
		r == '|', r == '(', r == ')', r == '[', r == ']', r == '{', r == '}',
		r == '_', r == '^':
		return true
	}
	return false
}

// plainLetter is an ASCII letter, which a TeX command would run into.
func plainLetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// combining is an accent that sits over the letter before it. The reading
// writes 𝔖̃ for \tilde{\mathfrak{S}} and the tilde arrives as its own character,
// so it has to come inside the span or it is left standing over a dollar sign.
func combining(r rune) bool { return r >= 0x0300 && r <= 0x036F }

// lineAround is the line a rune sits on, for a refusal to print.
func lineAround(rs []rune, i int) string {
	a, b := i, i
	for a > 0 && rs[a-1] != '\n' {
		a--
	}
	for b < len(rs) && rs[b] != '\n' {
		b++
	}
	return strings.TrimSpace(string(rs[a:b]))
}

// alphabetHeld is the two letters that are the wrong letter rather than the
// wrong font. See Alphabet.
var alphabetHeld = map[rune]string{
	'𝔽': `a double-struck F, which in this corpus is a misread \mathscr{B} ` +
		`and wants the page rather than a font`,
	'𝔾': `a double-struck G, which is one half of an ultrafilter written two ` +
		`ways and wants the page rather than a font`,
}

// alphabetTeX is the TeX that prints one Mathematical Alphanumeric Symbols
// character, and whether there is one.
//
// The block lays each alphabet out as A to Z then a to z, so the letter is the
// offset from the start of the run. Eleven of its cells are empty, because the
// characters that would have gone there were already encoded in Letterlike
// Symbols years earlier, and those are listed separately. A reading that finds
// the script H writes U+210B and never U+1D4A5, so leaving them out would leave
// the commonest of them behind.
func alphabetTeX(r rune) (string, bool) {
	if cmd, letter, ok := alphabetHole(r); ok {
		if letter == 0 {
			return cmd, true // \ell takes no argument
		}
		return cmd + "{" + string(letter) + "}", true
	}
	if _, held := alphabetHeld[r]; held {
		return "", false
	}
	var cmd string
	var base rune
	switch {
	case r >= 0x1D49C && r <= 0x1D4CF: // script
		cmd, base = `\mathscr`, 0x1D49C
	case r >= 0x1D4D0 && r <= 0x1D503: // bold script
		cmd, base = `\mathscr`, 0x1D4D0
	case r >= 0x1D504 && r <= 0x1D537: // fraktur
		cmd, base = `\mathfrak`, 0x1D504
	case r >= 0x1D56C && r <= 0x1D59F: // bold fraktur
		cmd, base = `\mathfrak`, 0x1D56C
	case r >= 0x1D538 && r <= 0x1D56B: // double-struck
		cmd, base = `\mathbf`, 0x1D538
	default:
		return "", false
	}
	off := r - base
	letter := 'A' + off
	if off >= 26 {
		letter = 'a' + off - 26
	}
	return cmd + "{" + string(letter) + "}", true
}

// alphabetHole is a letter the Mathematical Alphanumeric Symbols block leaves
// empty because Letterlike Symbols already had it.
func alphabetHole(r rune) (string, rune, bool) {
	switch r {
	case 'ℬ':
		return `\mathscr`, 'B', true
	case 'ℰ':
		return `\mathscr`, 'E', true
	case 'ℱ':
		return `\mathscr`, 'F', true
	case 'ℋ':
		return `\mathscr`, 'H', true
	case 'ℐ':
		return `\mathscr`, 'I', true
	case 'ℒ':
		return `\mathscr`, 'L', true
	case 'ℳ':
		return `\mathscr`, 'M', true
	case 'ℛ':
		return `\mathscr`, 'R', true
	case 'ℯ':
		return `\mathscr`, 'e', true
	case 'ℊ':
		return `\mathscr`, 'g', true
	case 'ℴ':
		return `\mathscr`, 'o', true
	case 'ℓ':
		// The script small l of an l^p space. TeX has a command of its own for
		// it and every other \ell in the corpus is written that way.
		return `\ell`, 0, true
	case 'ℭ':
		return `\mathfrak`, 'C', true
	case 'ℌ':
		return `\mathfrak`, 'H', true
	case 'ℑ':
		return `\mathfrak`, 'I', true
	case 'ℜ':
		return `\mathfrak`, 'R', true
	case 'ℨ':
		return `\mathfrak`, 'Z', true
	case 'ℂ':
		return `\mathbf`, 'C', true
	case 'ℍ':
		return `\mathbf`, 'H', true
	case 'ℕ':
		return `\mathbf`, 'N', true
	case 'ℙ':
		return `\mathbf`, 'P', true
	case 'ℚ':
		return `\mathbf`, 'Q', true
	case 'ℝ':
		return `\mathbf`, 'R', true
	case 'ℤ':
		return `\mathbf`, 'Z', true
	}
	return "", 0, false
}
