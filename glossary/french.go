package glossary

import "strings"

// The French counterpart of the English word list, for the one direction the
// script tests cannot see.
//
// glossary.Untranslated asks whether a word carries the writing of a language,
// and for Vietnamese, Chinese and Japanese that is a question about the letters:
// a Vietnamese word has a mark on it, a Chinese one is Han. English out of
// French is two languages in the same alphabet, so there is nothing to look at,
// and the rule that watches the other three sees nothing at all. Seven files of
// content/en-mt carried whole paragraphs of French, one of them a note to the
// reader that came back in French from end to end, and the audit was silent
// about every one of them.
//
// So it is decided on the words instead, and on the same kind of words the
// English list holds: the articles, the pronouns, the prepositions and the
// conjunctions, which are what a sentence is made of and what a translation
// leaves behind when it leaves a sentence behind. A term of art is no use here.
// The mathematics of the two languages shares most of its vocabulary, so
// "dimension", "module" and "isomorphisme" say nothing about which language the
// sentence around them is in, while "soient", "dans" and "des" say it at once.
//
// A word that is also an English word is off the list, however French it looks:
// "on", "or", "plus", "son", "car" and "a" are each a word an English paragraph
// can hold, and one of them on the list would put an English paragraph on the
// report. Both spellings of a word that carries an accent are on it, because
// the corpus holds pages read with the accents and pages read without them.
var french = map[string]bool{}

func init() {
	for _, w := range strings.Fields(`le la les de des du un une et est sont dans pour que qui
avec aux cette elle ne pas par tout tous toute toutes soit soient donc comme sans entre leur
leurs ainsi alors quel quelle si il ce cet ces nous vous au sur dont lorsque toujours montrer
ou où etre être meme même déduire deduire`) {
		french[w] = true
	}
}

// FrenchWords counts the words of text that are on that list.
func FrenchWords(text string) int {
	n := 0
	for _, w := range strings.Fields(strings.ToLower(text)) {
		if french[trimToLetters(w)] {
			n++
		}
	}
	return n
}
