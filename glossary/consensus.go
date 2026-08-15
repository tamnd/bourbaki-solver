package glossary

import (
	"sort"
	"strings"
)

// Asking the same question more than once, and keeping only what agrees.
//
// The checks in translate.go catch a line that is malformed, in the wrong
// script, or answering the wrong number. None of them can catch the failure
// that matters most here: one short phrase, in the right script, that is simply
// not the word mathematicians use. Every mechanical test passes it.
//
// The size of that gap was measured rather than guessed. The same 337 English
// terms went to the same model twice, an hour apart, and the two runs disagreed
// about 103 of them, so a little under seventy per cent agreed. Most of the
// disagreements were harmless, 无限集 against 无限集合 for an infinite set. Some
// were not: character came back as 特征 once and 特征标 once, and only the second
// is the character of a representation; Fitting was transliterated two ways in
// one run and left in Latin in the other, so one corpus would have spelled one
// name three ways.
//
// A model asked once has no way to tell those apart and neither does a reader
// of the log. Asked twice it does: where the answers agree the rendering is at
// least stable, and where they differ the term is exactly the one a person
// should write. That is the whole idea. It is affordable because the questions
// go to a gateway that costs nothing, so the price of asking three times is
// three times nothing and some minutes.

// Split is one term the rounds did not agree about.
type Split struct {
	EN string
	// TR is the distinct renderings, the most often answered first, and ties
	// broken by the rendering itself so a report does not shuffle between runs.
	TR []string
}

// Consensus keeps the rows that a majority of the rounds agreed about, and
// reports the rest.
//
// A term is kept when its most common rendering was answered by strictly more
// than half of the rounds that answered the term at all. Rounds that failed
// outright do not count against a term: a box that fell over is not a vote, and
// requiring a majority of the questions asked rather than of the answers
// received would throw away good work every time a host went down.
//
// A term nobody answered is not here at all. It keeps its place in the queue
// and the next run asks about it, which is the same policy as one round.
func Consensus(rounds [][]Row) (agreed []Row, split []Split) {
	type tally struct {
		en     string
		counts map[string]int
		total  int
	}
	byTerm := map[string]*tally{}
	var order []string
	for _, round := range rounds {
		// One round may answer the same term twice, if the same batch went out
		// twice in it. Its second answer is not a second opinion, so a term
		// votes once per round.
		seen := map[string]bool{}
		for _, row := range round {
			key := Key(row.EN)
			if seen[key] {
				continue
			}
			seen[key] = true
			t, ok := byTerm[key]
			if !ok {
				t = &tally{en: row.EN, counts: map[string]int{}}
				byTerm[key] = t
				order = append(order, key)
			}
			t.counts[strings.TrimSpace(row.TR)]++
			t.total++
		}
	}

	for _, key := range order {
		t := byTerm[key]
		variants := make([]string, 0, len(t.counts))
		for tr := range t.counts {
			variants = append(variants, tr)
		}
		sort.Slice(variants, func(i, j int) bool {
			if t.counts[variants[i]] != t.counts[variants[j]] {
				return t.counts[variants[i]] > t.counts[variants[j]]
			}
			return variants[i] < variants[j]
		})
		if best := variants[0]; 2*t.counts[best] > t.total {
			agreed = append(agreed, Row{EN: t.en, TR: best})
			continue
		}
		split = append(split, Split{EN: t.en, TR: variants})
	}
	return agreed, split
}
