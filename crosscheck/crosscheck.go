// Package crosscheck reads a page twice and reports the words only one reading
// has.
//
// The extractor builds a page out of poppler's boxes and fonts, which is the
// only way to get the mathematics back, and every rule it applies to those boxes
// is a place it can be wrong. Nothing downstream can tell: a page that has lost
// a word or glued two together is the right length, its dollars balance, its
// running head is right, and it reads perfectly.
//
// pdftotext is the second reading. It comes out of the same PDF and the same
// library, it knows nothing about mathematics and throws all of it away, and
// that is what makes it useful here: it has no idea what our rules are, so where
// the two disagree about a word of prose, one of us is wrong about the prose.
//
// It is not a rule and it does not fail a page. It is a list of places for a
// person to look, and it has to be quiet enough to be worth reading. Both of
// those were measured on Algebra VIII, and both are why the rules below are
// written the way they are.
//
// Quiet: 494 pages of the volume as it stands come to 4 pages and 6 words. Two
// are the two-column index, which the two readings take apart differently; one
// is pdftotext's debris; one is a footnote set in two columns; one is a display
// formula; and one is the PDF itself, which writes "fromn" and "groupnK" on page
// 203 and is the only line in the volume that does. Every one of the six is
// explainable by reading the page.
//
// Worth reading: run against the same volume three commits back, it names pages
// 113, 114, 354, 424, 429, 431 and 466, which is every page an accent was drawn
// on the wrong letters, pages 14, 306 and 456, where a heading came out one
// letter at a time inside dollar signs, and pages 237, 377 and 497, where a word
// was left broken in half at the end of a line. None of those changed the length
// of a page, unbalanced a dollar or broke the front matter, and nothing else in
// the pipeline could see any of them.
//
// The two directions are Page and Extra, and both are needed. Page asks which
// of pdftotext's words we lost; Extra asks which of ours pdftotext has never
// heard of. An accent that breaks "assumptions" into "assumpti" and "ons"
// passes the first, because that page says "assumptions" correctly further
// down, and fails the second.
package crosscheck

import (
	"strings"
	"unicode"
)

// MinWord is how long a word has to be before its absence is worth printing.
//
// Short words are noise. The two readings split and rejoin around inline
// mathematics differently, and the fragments that fall out of that are two and
// three letters long. Five is where the list stops being mostly fragments.
const MinWord = 5

// Lost is a word one reading has and the other has not.
type Lost struct {
	Word string // the word, as the reading that has it printed it
	Line string // the line it sits on, for finding it on the page
	Ours bool   // the word is ours and pdftotext has not got it
}

// Page compares one page read two ways and returns what the first reading is
// missing.
//
// ours is a page body as the extractor wrote it, Markdown with LaTeX in it.
// theirs is the same page as pdftotext prints it.
//
// The running head and the page number are not passed in and do not have to be.
// We lift them out of the body into the front matter and pdftotext leaves them
// in the text, so every page would report its own header as lost, except that
// this volume sets its running heads in capitals and writes its page numbers as
// A VIII.85, and neither a capital nor a digit is a word this asks about.
func Page(ours, theirs string) []Lost {
	have := set(strip(ours))
	said := set(theirs)
	var out []Lost
	seen := map[string]bool{}
	// The hyphen a word was broken on is kept here rather than dropped, because
	// which of the two it is, the typesetter's break or the hyphen of a compound
	// word, is the question the extractor has to answer, and the two answers are
	// compared against ours below.
	lines := flow(theirs, true)
	// The break falls at the top and the foot of the page, and the top of the
	// page is the running head, so the first line of the body is the second
	// line with letters on it.
	top, body := next(lines, -1), -1
	if top >= 0 {
		body = next(lines, top)
	}
	for li, line := range lines {
		ws := words(line)
		for i, w := range ws {
			if have[strings.ToLower(w)] {
				continue
			}
			// The same word with the hyphen taken out. pdftotext keeps the
			// hyphen wherever it fell and we decide: "commu-tative" of page 237
			// is ours as "commutative", and "finite-dimensional" of page 104 is
			// ours with the hyphen, which the line above has already passed.
			if have[strings.ToLower(strings.ReplaceAll(w, "-", ""))] {
				continue
			}
			for _, p := range strings.Split(w, "-") {
				key := strings.ToLower(p)
				if !candidate(p) || have[key] || seen[key] {
					continue
				}
				if joined(have, ws, i) || glued(have, said, key) {
					continue
				}
				if (li == top || li == body || next(lines, li) < 0) && half(have, key) {
					continue
				}
				seen[key] = true
				out = append(out, Lost{Word: p, Line: strings.TrimSpace(line)})
			}
		}
	}
	return out
}

// candidate says whether a word of the second reading is worth checking at all.
//
// Lower case and five letters. A capital is either a proper name, which the two
// readings spell the same and lose the same way, or a variable, which we set in
// mathematics and pdftotext prints as a bare letter, and comparing those two is
// comparing a formula with the debris of one.
//
// A letter outside ASCII is thrown out for the same reason once removed. The
// mathematics fonts come through pdftotext as Greek and as arrows, and a word
// with any of that in it is a piece of a formula rather than a word: sigmaa,
// lambdax, phiy.
func candidate(w string) bool {
	if len([]rune(w)) < MinWord {
		return false
	}
	for _, r := range w {
		if !unicode.IsLower(r) || r > unicode.MaxASCII {
			return false
		}
	}
	return true
}

// glued says whether the word is one of pdftotext's own. It sets the
// mathematics down where it stood and runs it into the word beside it, and it
// joins what the typesetter broke at a line end without keeping the halves. It
// fires five times on this volume, on "kalgebra" and "kalgebras" for a
// k-algebra, on "rmodule" and "domodule" for an R-module and an O-module, and
// on "complexen" for the "com-" of page 497 that belongs to the line below.
// Three characters is as far as that goes here.
//
// The tail has to be a word of the page in both readings, and that second half
// of the test is not a nicety. Without it the rule swallows the defect it was
// written beside: page 114 read "f~aithful" with the accent over the wrong
// letters, which leaves "thful" standing in our text, and "faithful" less three
// characters is "thful", so the one page that showed the accents were wrong
// went quiet. pdftotext has no "thful" anywhere and does have "module"
// everywhere.
func glued(have, theirs map[string]bool, w string) bool {
	for k := 1; k <= 3 && k < len(w); k++ {
		if tail := w[k:]; len(tail) >= MinWord-1 && have[tail] && theirs[tail] {
			return true
		}
	}
	return false
}

// half says whether the word is one end of a word broken across the page break.
//
// The typesetter breaks a word across a page as readily as across a line, and
// the two readings put it back differently: page 102 opens on "scription" and
// pages 159, 363 and 480 open on "morphism". We have the whole word, since the
// half at the foot of the previous page is in the poppler XML of this one, and
// pdftotext prints the page as it stands.
//
// It is only asked at the top and the foot of the page, where that break is.
// Anywhere else "morphism" being the tail of some "homomorphism" on the page
// would excuse a real loss.
func half(have map[string]bool, w string) bool {
	for k := range have {
		if len(k) > len(w) && (strings.HasSuffix(k, w) || strings.HasPrefix(k, w)) {
			return true
		}
	}
	return false
}

// joined says whether the word is half of something we wrote as one word.
//
// Both readings undo the typesetter's hyphenation at the end of a line, and not
// always in the same place. Where pdftotext keeps "sub" and "bimodule" apart we
// may have written "sub-bimodule", and neither reading has lost anything. So a
// word that joins on to its neighbour, with or without the hyphen, is not a
// loss.
func joined(have map[string]bool, ws []string, i int) bool {
	var pairs []string
	if i > 0 {
		pairs = append(pairs, ws[i-1]+ws[i], ws[i-1]+"-"+ws[i])
	}
	if i+1 < len(ws) {
		pairs = append(pairs, ws[i]+ws[i+1], ws[i]+"-"+ws[i+1])
	}
	for _, p := range pairs {
		if have[strings.ToLower(p)] {
			return true
		}
	}
	return false
}

// flow puts back the words pdftotext leaves broken at a line end.
//
// It prints the page line by line and leaves the typesetter's hyphens where
// they are, so "commu-" ends one line and "tative" opens the next. We join such
// a word up, so unless the two halves are put back together here every
// dehyphenated word of the volume is reported as lost twice over.
//
// The half words are not always on consecutive lines. pdftotext sets what it
// cannot place on a line of its own, so "Conse-" ends line 59 of page 100, the
// two primes of A'_M and A”_M have line 60 to themselves, and "quently," opens
// line 61.
//
// A line that does not open on a word is passed over, and asking only for a
// letter is not enough. Page 102 breaks "de-" and sets a lone subscript lambda
// on the line below it, which is a letter, so the reading joined "de" to that
// lambda and left "scription" standing on the line after as a word we had lost.
func flow(s string, hyphen bool) []string {
	lines := strings.Split(s, "\n")
	out := make([]string, 0, len(lines))
	for i := range lines {
		line := strings.TrimRight(lines[i], " ")
		if j := word(lines, i); strings.HasSuffix(line, "-") && j > 0 {
			head, rest, _ := strings.Cut(strings.TrimLeft(lines[j], " "), " ")
			if head != "" {
				if !hyphen {
					line = strings.TrimSuffix(line, "-")
				}
				line += head
				lines[j] = rest
			}
		}
		out = append(out, line)
	}
	return out
}

// word is the next line after i that opens on a word, or -1. A word here starts
// with two ASCII letters, which is what a word broken at a line end turns over
// onto and what the stranded pieces of a formula are not.
func word(lines []string, i int) int {
	for j := i + 1; j < len(lines); j++ {
		t := strings.TrimLeft(lines[j], " ")
		if len(t) >= 2 && isASCIILetter(t[0]) && isASCIILetter(t[1]) {
			return j
		}
	}
	return -1
}

// next is the line after i with a letter on it, or -1.
func next(lines []string, i int) int {
	for j := i + 1; j < len(lines); j++ {
		if strings.ContainsFunc(lines[j], unicode.IsLetter) {
			return j
		}
	}
	return -1
}

// strip takes the LaTeX off our own reading, leaving what a reader would read.
//
// A macro name is not a word and its letters are not letters of the page:
// \widetilde{n} would otherwise put "widetilde" into the page and, worse, hide
// nothing at all, since what we are asking is which of pdftotext's words we do
// not have. The braces and the dollars go with it.
func strip(s string) string {
	var b strings.Builder
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' {
			if s[i] != '$' && s[i] != '{' && s[i] != '}' {
				b.WriteByte(s[i])
			}
			continue
		}
		i++
		for i < len(s) && isASCIILetter(s[i]) {
			i++
		}
		i--
		b.WriteByte(' ')
	}
	return b.String()
}

func isASCIILetter(c byte) bool { return c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' }

// words is every maximal run of letters and hyphens in a string.
//
// The hyphen is kept inside a word because the volume is full of compound words
// carrying one, and dropping it would make "sub-bimodule" and "subbimodule" the
// same word, which is the difference this is here to see. It is trimmed off the
// ends, where it is the typesetter's break rather than part of the word.
func words(s string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		w := strings.Trim(b.String(), "-")
		b.Reset()
		if w != "" {
			out = append(out, w)
		}
	}
	for _, r := range s {
		if unicode.IsLetter(r) || r == '-' {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

// set is the words of a page, lower cased, for asking whether a word is in it.
// A compound word goes in whole and in pieces, since the other reading may have
// broken it where we did not.
func set(s string) map[string]bool {
	m := map[string]bool{}
	for _, w := range words(s) {
		m[strings.ToLower(w)] = true
		for _, p := range strings.Split(w, "-") {
			if p != "" {
				m[strings.ToLower(p)] = true
			}
		}
	}
	return m
}
