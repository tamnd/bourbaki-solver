package gtranslate

import (
	"context"
	"strings"

	"github.com/tamnd/bourbaki-solver/glossary"
)

// Terms splits the glossary rows a passage mentions into the ones to pin and
// the ones to leave to Google and correct afterwards.
//
// A noun is pinned: it goes to Google as a token and comes back as the
// glossary's rendering, which is the only way to stop "ring" coming back as a
// circle and "Proposition" as a bill before parliament. An adjective or a
// phrase that ends in a preposition is not, because Vietnamese puts the
// adjective after the noun and Google can only reorder words it can see:
// pinning "commutative" turned "commutative group structure" into "giao hoán
// cấu trúc nhóm". Those go through as words and Fix puts the glossary's
// rendering back over Google's afterwards. Into a language that writes
// articles nothing is pinned at all; see below.
func Terms(g *glossary.Glossary, source, target, body string) (pins, loose []Pin) {
	lower := strings.ToLower(body)
	for _, t := range g.Terms {
		from, to := t.In(source), t.In(target)
		if from == "" || to == "" || strings.EqualFold(from, to) {
			continue
		}
		if !mentioned(lower, glossary.Key(from)) {
			continue
		}
		p := Pin{From: from, To: to}
		switch {
		case !articleless[target]:
			// Into English, Google needs the nouns to reorder the French
			// adjectives around them and to make "k-algèbres" plural and give
			// it an article; a pinned noun came back as "two k-algebra
			// formally smooth". Google's English for mathematics is good, so
			// nouns are only corrected afterwards, and the glossary rows that
			// are not terms, "montrer" as "now prove", are left alone.
			if nounPhrase(t.EN) {
				loose = append(loose, p)
			}
		case nounPhrase(t.EN):
			pins = append(pins, p)
		default:
			loose = append(loose, p)
		}
	}
	return pins, loose
}

func mentioned(lower, key string) bool {
	for _, k := range []string{key, key + "s", key + "es", key + "x"} {
		if glossary.Mentions(lower, k) {
			return true
		}
	}
	return false
}

var prepositions = map[string]bool{
	"to": true, "over": true, "of": true, "on": true, "in": true, "with": true,
	"by": true, "from": true, "at": true, "for": true, "into": true, "onto": true,
}

// adjectives are the single words the suffix test gets wrong, both ways.
var adjectives = map[string]bool{
	"free": true, "left": true, "right": true, "open": true, "closed": true,
	"finite": true, "infinite": true, "simple": true, "dense": true, "compact": true,
	"exact": true, "flat": true, "regular": true, "normal": true, "proper": true,
	"perfect": true, "complete": true, "discrete": true, "empty": true, "equal": true,
	"same": true, "distinct": true, "unique": true, "null": true, "real": true,
	"complex": true, "pure": true, "strict": true, "smooth": true, "upper": true,
	"lower": true, "inverse": true, "opposite": true, "direct": true, "maximal": true,
	"minimal": true, "dual": true, "trivial": true, "whole": true, "prime": true,
	"stable": true, "faithful": true, "absolute": true, "relative": true,
}

// openers are the words a glossary row starts with when it is a turn of
// phrase and not a term: "now prove", "let", "show that".
var openers = map[string]bool{
	"now": true, "let": true, "show": true, "prove": true, "suppose": true, "assume": true,
	"we": true, "then": true, "hence": true, "thus": true, "deduce": true, "recall": true,
}

var nounEndings = []string{"ment", "ness", "ity", "ism", "ion", "ure", "ance", "ence", "ing"}

var adjectiveEndings = []string{
	"ive", "ous", "al", "ic", "able", "ible", "ed", "ent", "ant", "ful",
	"less", "ary", "ory", "ar", "ile", "ish", "ly", "wise",
}

// nounPhrase says whether an English headword names a thing. An English noun
// phrase ends in its noun, so the last word is the one asked about.
func nounPhrase(en string) bool {
	words := strings.Fields(strings.ToLower(en))
	if len(words) == 0 {
		return false
	}
	if openers[words[0]] {
		return false
	}
	head := words[len(words)-1]
	if prepositions[head] || adjectives[head] {
		return false
	}
	for _, e := range nounEndings {
		if strings.HasSuffix(head, e) {
			return true
		}
	}
	for _, e := range adjectiveEndings {
		if strings.HasSuffix(head, e) && len(head) > len(e)+2 {
			return false
		}
	}
	return true
}

// Learned is what Google writes for each loose term when it sees it alone,
// keyed by the term as the source spells it.
type Learned map[string]string

// Learn asks Google for its own rendering of each loose term, all of them in
// one request, one term to a line. The client caches the answer, so a run asks
// this once per set of terms.
func (c *Client) Learn(ctx context.Context, sl, tl string, loose []Pin) (Learned, error) {
	out := Learned{}
	if len(loose) == 0 {
		return out, nil
	}
	var lines []string
	for _, p := range loose {
		lines = append(lines, p.From)
	}
	got, err := c.Translate(ctx, sl, tl, strings.Join(lines, "\n"))
	if err != nil {
		return nil, err
	}
	back := strings.Split(got, "\n")
	if len(back) != len(lines) {
		// Google merged or split a line, and a table that is off by one row
		// would put the wrong word in. Better to correct nothing.
		return out, nil
	}
	for i, p := range loose {
		if g := strings.TrimSpace(back[i]); g != "" {
			out[p.From] = g
		}
	}
	return out, nil
}

// Fix puts the glossary's rendering of each loose term over Google's, where
// the answer does not already have the glossary's.
func Fix(answer string, loose []Pin, learned Learned) string {
	lower := strings.ToLower(answer)
	for _, p := range loose {
		g := learned[p.From]
		if g == "" || strings.EqualFold(g, p.To) {
			continue
		}
		if strings.Contains(lower, strings.ToLower(p.To)) {
			continue
		}
		if !strings.Contains(lower, strings.ToLower(g)) {
			continue
		}
		answer = replaceFold(answer, g, p.To)
		lower = strings.ToLower(answer)
	}
	return answer
}

// replaceFold replaces every whole-word occurrence of old, in any case, keeping
// a capital where the occurrence had one.
func replaceFold(s, old, new string) string {
	lower, lold := strings.ToLower(s), strings.ToLower(old)
	if len(lower) != len(s) {
		return s
	}
	var b strings.Builder
	at := 0
	for {
		i := strings.Index(lower[at:], lold)
		if i < 0 {
			break
		}
		i += at
		end := i + len(lold)
		if !edge(s, i, end) {
			b.WriteString(s[at:end])
			at = end
			continue
		}
		b.WriteString(s[at:i])
		if startsUpper(s[i:]) {
			b.WriteString(upperFirst(new))
		} else {
			b.WriteString(new)
		}
		at = end
	}
	b.WriteString(s[at:])
	return b.String()
}
