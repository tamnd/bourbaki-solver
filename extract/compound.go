package extract

import (
	"strings"
	"unicode"
)

// A hyphen at the end of a line is two different marks in one glyph. It is
// mostly the typesetter breaking a long word, and putting the word back means
// dropping it: "commu-" and "tative" are one word. Sometimes it is the hyphen
// of a compound word that happens to fall at the end of a line, and dropping it
// there writes a word the book does not contain: page 172 shipped "twosided",
// page 104 "finitedimensional", page 145 "infinitedimensional", page 137
// "quasisimple", page 288 "subbimodules" and page 459 "subpseudomodule".
//
// Nothing about the line says which it is, and the halves do not say either. A
// rule that keeps the hyphen whenever the second half is a word of its own
// keeps it on "sub-module", "homo-morphism", "semi-simple" and "more-over": 19
// wrong out of 41 kept over this volume, measured against the words the book
// really writes with a hyphen.
//
// What does say is the rest of the volume. A compound broken at its hyphen at
// the end of one line is set inside a line somewhere else, and the book is
// consistent: two-sided is written with the hyphen 221 times, finite-dimensional
// 171 times, sub-bimodule 13 times, sub-pseudomodule 9 times. The words that
// were merely broken never appear hyphenated anywhere. So the volume is read
// once to collect the compounds it writes inside a line, and read again to lay
// out the pages with that in hand.

// Compounds is the set of words a volume writes with a hyphen inside a line.
type Compounds map[string]bool

// Read adds the compound words of one rendered page body.
//
// Only a hyphen inside a line counts. A hyphen at the end of a line is the
// question this is here to answer and cannot answer itself.
func (c Compounds) Read(body string) {
	for _, line := range strings.Split(body, "\n") {
		for _, w := range strings.Fields(line) {
			w = strings.Trim(w, ".,;:()[]“”’")
			a, b, ok := strings.Cut(w, "-")
			if !ok || !lowerWord(a) || !lowerWord(b) {
				continue
			}
			c[a+"-"+b] = true
		}
	}
}

// Keeps reports whether a word broken at the end of a line keeps its hyphen.
// s is the text so far, ending in the hyphen, and next is the line that follows.
func (c Compounds) Keeps(s, next string) bool {
	if len(c) == 0 {
		return false
	}
	a := tailWord(strings.TrimSuffix(s, "-"))
	b := headWord(strings.TrimLeft(next, " "))
	return a != "" && b != "" && c[a+"-"+b]
}

// lowerWord reports whether s is a word of lower case letters, which is what
// both halves of a compound are. A capital is a name or a symbol, and the
// hyphen after a symbol is decided by the mathematics around it instead.
func lowerWord(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLower(r) {
			return false
		}
	}
	return true
}

// tailWord is the run of lower case letters at the end of s.
func tailWord(s string) string {
	r := []rune(s)
	i := len(r)
	for i > 0 && unicode.IsLower(r[i-1]) {
		i--
	}
	return string(r[i:])
}

// headWord is the run of lower case letters at the start of s.
func headWord(s string) string {
	for i, r := range s {
		if !unicode.IsLower(r) {
			return s[:i]
		}
	}
	return s
}
