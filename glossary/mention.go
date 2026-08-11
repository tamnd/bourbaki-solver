package glossary

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

// Finding a term in a body of text, which two different parts of the pipeline
// need and had better agree about.
//
// The chunker asks it of the English, to decide which rows of the glossary go
// in a prompt. The audit asks it of the English to decide which rows a section
// was held to, and of the translation to decide whether the row was followed.
// If those two disagreed, the audit would refuse a section over a term the
// translator was never shown, which is the worst kind of finding: true on its
// own terms and impossible to act on.

// Mentions reports whether term appears in text as a term rather than as a run
// of letters inside a longer word.
//
// Both arguments must already be lower case. That is an awkward contract for a
// public function and it is the one that makes a glossary scan affordable: 814
// terms against a 35,000 character section is 28 million comparisons per file,
// and lowering the section inside the loop would do it 814 times over.
func Mentions(text, term string) bool { return FindMention(text, term, 0) >= 0 }

// FindMention is where term appears in text at or after from, or -1. Same lower
// case contract as Mentions.
func FindMention(text, term string, from int) int {
	if term == "" || from < 0 {
		return -1
	}
	for i := from; i <= len(text); {
		j := strings.Index(text[i:], term)
		if j < 0 {
			return -1
		}
		j += i
		if boundaryBefore(text, j) && boundaryAfter(text, j+len(term)) {
			return j
		}
		i = j + 1
	}
	return -1
}

// A boundary is anything that is not a letter or a digit, and the ends of the
// text count.
//
// Read as runes and not as bytes, because a byte test calls the second byte of
// "đ" a boundary and then finds "vành" inside "vànhđ". No such word turned up
// in the corpus, but a term test that is wrong on Vietnamese is a poor thing to
// hold a Vietnamese translation to. An apostrophe and a hyphen are boundaries
// too, which is what makes "ring" a mention in "ring-theoretic".
func boundaryBefore(text string, i int) bool {
	if i <= 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(text[:i])
	return !isWordRune(r)
}

func boundaryAfter(text string, i int) bool {
	if i >= len(text) {
		return true
	}
	r, _ := utf8.DecodeRuneInString(text[i:])
	return !isWordRune(r)
}

func isWordRune(r rune) bool { return unicode.IsLetter(r) || unicode.IsDigit(r) }

// Mentioned is every term of the glossary that appears in the text, longest
// first, with each stretch of the text claimed by at most one term.
//
// The masking is the whole of it. "ring" and "semisimple ring" are both terms,
// and a scan that reported both of them for one occurrence of "semisimple ring"
// would then hold the translation to a rendering of "ring" that the phrase does
// not contain. Longest first and then out of the running is what a reader does:
// the phrase is met as a phrase, and the word inside it is not a second term.
//
// text must already be lower case, and it should be prose with the mathematics
// taken out, because a term that only appears inside a formula is a term the
// translator was told to copy rather than to render.
func (g Glossary) Mentioned(lang, text string) []Term {
	masked := make([]bool, len(text))
	var out []Term
	for _, t := range g.Sorted() {
		if t.In(lang) == "" {
			continue
		}
		key := Key(t.EN)
		for i := 0; i >= 0; {
			j := FindMention(text, key, i)
			if j < 0 {
				break
			}
			if free(masked, j, j+len(key)) {
				claim(masked, j, j+len(key))
				out = append(out, t)
				break
			}
			i = j + 1
		}
	}
	return out
}

func free(masked []bool, from, to int) bool {
	for i := from; i < to && i < len(masked); i++ {
		if masked[i] {
			return false
		}
	}
	return true
}

func claim(masked []bool, from, to int) {
	for i := from; i < to && i < len(masked); i++ {
		masked[i] = true
	}
}

// Follows reports whether a translation renders a term the way the glossary
// says. text must already be lower case.
//
// The test is presence and not count. A section that says "vành" once and then
// carries the idea in a pronoun for three paragraphs is ordinary writing, and a
// rule that counted occurrences would report every well written page. What this
// catches is the term rendered some other way throughout, which is the failure
// the glossary exists to prevent.
//
// A rendering may itself be several words, and it is looked for whole. Some of
// the Vietnamese rows are the English word standing as it is, Nullstellensatz
// and Noether among them, and those are found by the same test.
//
// Chinese and Japanese are looked for as a plain substring, because they are
// written without spaces and every neighbour of a term is a letter. The word
// boundary test would refuse 环 inside 交换环 and so refuse every term of a
// correct page. Vietnamese keeps the boundary test, which it needs: "vành" is a
// word in a language with spaces between words.
func Follows(lang, text, rendering string) bool {
	r := strings.ToLower(strings.TrimSpace(rendering))
	if r == "" {
		return true
	}
	switch lang {
	case "zh", "ja":
		return strings.Contains(text, r)
	}
	return Mentions(text, r)
}
