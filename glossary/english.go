package glossary

import (
	"regexp"
	"strings"
	"unicode"
)

// What is left of the English inside a translation.
//
// WrittenIn asks the question of a whole piece of text, and a whole paragraph
// is the unit the audit had when the only failure anybody had seen was a
// paragraph handed back unchanged. The appendix on the trace turned up the
// other one: fifteen chunks came back, fourteen of them Vietnamese, and in the
// middle of the eleventh sat two English sentences with Vietnamese either side
// of them on the same line. Every rule that reads a paragraph passed it, since
// the paragraph is written in Vietnamese, and the only thing that noticed was
// the glossary rule, which reported the same sentence eight times over as eight
// separate terms left standing.
//
// So the unit here is the run and not the paragraph: the longest stretch of
// consecutive words that carry none of the language the text is supposed to be
// in. Sentence splitting was the obvious way to get there and it is the wrong
// one, because Bourbaki writes "III, §7, no. 9, p. 520" and a splitter that
// reads a full stop as the end of a sentence shreds every citation in the book.
// A run needs no punctuation at all.

// english is the words that hold an English sentence together and carry no
// mathematics of their own. A run with two of them is English; what is left of a
// display when the dollars come off has none.
//
// Four words are missing from it that belong there on the face of it, and they
// are missing because Vietnamese has them too. "in" is to print, "to" is big,
// "an" is peace, "do" is the first half of "do đó", which is how a Vietnamese
// proof says therefore. "so", "may" and "can" are out for the same reason. A
// word that both languages spell the same way says nothing about which language
// a piece of writing is in, and this list is only worth having if every word on
// it does.
var english = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`the of is and for be we that it as if then every there
this which are has have let was were from but not all such where when its they one on or
thus hence therefore follows also only same each other into over under between because since
while what who whose whom does did shall will would could might must been being had here now
first second third these those`) {
		english[w] = true
	}
}

// EnglishWords counts the words of text that are on that list.
func EnglishWords(text string) int {
	n := 0
	for _, w := range strings.Fields(strings.ToLower(text)) {
		if english[trimToLetters(w)] {
			n++
		}
	}
	return n
}

// maths is what a model is asked to copy rather than translate, and what is
// left of it when the dollars come off is not writing in any language. Taken
// out here rather than by the caller, because both callers would have to do it
// and one of them, the run that asks the questions, holds the answer as it
// arrived and has nothing that strips it.
var (
	maths = regexp.MustCompile(`(?s)\$\$.*?\$\$|\$[^$]*\$`)
	attrs = regexp.MustCompile(`\{#[^}]*\}`)
)

// Untranslated finds the longest run of consecutive words in text that carry no
// writing of lang, and reports the run and how many English words are in it.
//
// Measured over the corpus as it stands, which is 27 sections and their
// exercises: exactly one run anywhere carries two English words or more, and it
// is the one this was written for, at fourteen. The next run down carries none,
// and is the residue of a display. Two is the same floor L07 measured for a
// paragraph and it is clear of everything real by the same margin, which is
// what a hard rule needs.
func Untranslated(lang, text string) (run string, words int) {
	switch lang {
	case "vi", "zh", "ja":
	default:
		// A language nobody has written a script test for cannot be asked
		// whether a word is missing one, and WrittenIn says yes to everything
		// for exactly that reason.
		return "", 0
	}
	var best, cur []string
	for _, w := range strings.Fields(attrs.ReplaceAllString(maths.ReplaceAllString(text, " "), " ")) {
		core := trimToLetters(w)
		switch {
		case core == "":
			// Punctuation, a bare number, a stranded bracket. It belongs to
			// neither language and breaks no run.
		case WrittenIn(lang, core):
			cur = nil
		default:
			cur = append(cur, w)
			if len(cur) > len(best) {
				best = append(best[:0:0], cur...)
			}
		}
	}
	run = strings.Join(best, " ")
	return run, EnglishWords(run)
}

func trimToLetters(w string) string {
	return strings.TrimFunc(strings.ToLower(w), func(r rune) bool { return !unicode.IsLetter(r) })
}
