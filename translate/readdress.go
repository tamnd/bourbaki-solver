package translate

import (
	"regexp"
	"strings"
)

// marks is what a translation writes where the English writes p. and No., by
// language. Nothing is listed for a language until a run has seen a model write
// it, because a word listed here is a word this file moves.
//
// Vietnamese writes a page as trang and abbreviates it tr., and a number as so,
// with or without its accent depending on the model. Every one of those is the
// right word for the thing and the wrong thing to write in a citation.
var marks = map[string][]string{
	"vi": {"tr.", "trang", "Tr.", "Trang", "số", "Số", "so", "So"},
}

// Readdress puts back a citation the answer wrote in its own words, and leaves
// everything else exactly as the model wrote it.
//
// A citation is an address. "p. 185" is where the sentence sends a reader, and
// the corpus writes it the same way in all four languages, which is what lets a
// reference be compared across them and what RuleReference refuses an answer
// for. A model translating into Vietnamese writes "tr. 185" instead, because
// that is how a Vietnamese book prints a page number, and it is not being
// careless: it is writing good Vietnamese.
//
// Refusing is the wrong end to fix it at, for the reason Respace gives. Chunk 4
// of the historical note of chapter IV is five thousand characters of Leibniz
// and it was refused eight times over four passes for tr. 185 and nothing else,
// each ask costing five minutes, with a note in front of it naming the
// abbreviation. The number is right, the order is right and the citation stands
// where the English put it, so the letters in front of it go back and the answer
// goes to the audit with nothing left for RuleReference to find.
//
// It is a repair and not a normaliser, and it is led by the rule rather than by
// the words: it only puts back an address the English cites and the answer does
// not have, and it puts back as many as are missing and no more. An answer that
// writes "số 4" where the English also has a "no. 4" the rule does not read
// keeps it, because nothing is missing there; one exercise of chapter IV, § 2 is
// exactly that, and a repair by words alone rewrote it and gave the rule a
// citation the English does not have.
func Readdress(lang, en, tr string) string {
	words, ok := marks[lang]
	if !ok {
		return tr
	}
	missing := count(refs(en))
	for k, n := range count(refs(tr)) {
		missing[k] -= n
	}
	for address, need := range missing {
		if need <= 0 {
			continue
		}
		mark, number, ok := split(address)
		if !ok {
			continue
		}
		for _, w := range words {
			if need <= 0 {
				break
			}
			re, err := regexp.Compile(`\b` + regexp.QuoteMeta(w) + `\s*` + regexp.QuoteMeta(number) + `\b`)
			if err != nil {
				continue
			}
			tr, need = replaceUpTo(re, tr, mark+" "+number, need)
		}
	}
	return tr
}

// split is the abbreviation and the number of a tightened address, and says no
// to an address that is neither a page nor a numbered heading. A chapter and a §
// are cited in numerals the two languages share and no model has written either
// of them in words.
func split(address string) (mark, number string, ok bool) {
	for _, mark := range []string{"p.", "No."} {
		if !strings.HasPrefix(address, mark) {
			continue
		}
		number := strings.TrimPrefix(address, mark)
		if number == "" {
			return "", "", false
		}
		return mark, number, true
	}
	return "", "", false
}

// replaceUpTo rewrites at most n matches, and says how many are still wanted.
func replaceUpTo(re *regexp.Regexp, body, put string, n int) (string, int) {
	for n > 0 {
		loc := re.FindStringIndex(body)
		if loc == nil {
			break
		}
		body = body[:loc[0]] + put + body[loc[1]:]
		n--
	}
	return body, n
}

func count(addresses []string) map[string]int {
	out := map[string]int{}
	for _, a := range addresses {
		out[a]++
	}
	return out
}
