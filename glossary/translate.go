package glossary

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/prompt"
	"github.com/tamnd/bourbaki-solver/textguard"
)

// Asking a model for terminology, and refusing most of what it says.
//
// The glossary is the one artefact of this project that cannot be repaired
// afterwards. A formula that came out wrong is one file to fix. A term rendered
// wrong is every file that used it, in a language the person who ran the
// pipeline very likely cannot read, and nobody finds out until a reader
// searches for the right word and gets nothing.
//
// So the audit here is not a sanity check, it is the point. A batch goes out
// with the terms numbered, and every line that comes back has to say which
// number it answers, repeat the English character for character, and be one
// short phrase in the script of the language. Anything else is dropped and
// reported, and a dropped line is cheap: it is one term to ask about again or
// to write by hand.
//
// The model is given a way to say it does not know, in as many words, and told
// that saying so is the wanted answer. That is worth more than any check below.
// A model with no way out invents, and an invented rendering is exactly the
// failure that survives every mechanical test: it is one phrase, in the right
// script, that no mathematician uses.

// Language is the name of a language as a model should be told it.
//
// Spelled out rather than the code, because a code is a thing to guess at. It
// lives in the prompt package, which is where the other thing that has to say
// "Vietnamese" to a model lives, and this is the alias so that the two never
// come to disagree about what zh means.
func Language(lang string) string { return prompt.Language(lang) }

// DefaultBatch is how many terms go in one question.
//
// Forty because of two opposite failures. A long list is where a model starts
// summarising, dropping the middle, or answering in a table; a short one pays
// the fixed cost of a browser and a page load for a handful of terms, and the
// fleet has three boxes and a daily quota. Forty is one screen of answer.
const DefaultBatch = 40

// Batch is one question: a slice of terms, numbered from 1 within the batch.
type Batch struct {
	Lang  string
	Terms []string
}

// Batches cuts a list of terms into questions.
func Batches(lang string, terms []string, size int) []Batch {
	if size <= 0 {
		size = DefaultBatch
	}
	var out []Batch
	for i := 0; i < len(terms); i += size {
		out = append(out, Batch{Lang: lang, Terms: terms[i:min(i+size, len(terms))]})
	}
	return out
}

// Prompt is the question this batch asks.
func (b Batch) Prompt() string {
	var list strings.Builder
	for i, term := range b.Terms {
		fmt.Fprintf(&list, "%d | %s\n", i+1, term)
	}
	return prompt.Glossary(Language(b.Lang), list.String())
}

// Row is one accepted rendering.
type Row struct {
	EN string
	TR string
}

// Reject is one line that was not accepted, and why.
//
// The English is kept even when the line was unreadable, so that a rejected
// term can be asked about again without working out which of the forty it was.
type Reject struct {
	EN     string
	Line   string
	Reason string
}

// Reply is what one answer came to.
type Reply struct {
	Rows []Row
	// Unknown is the terms the model said it did not know. Not a failure: it
	// was asked to say so, and a term nobody can render is a term for a person
	// to write by hand rather than one to ask a second model about.
	Unknown []string
	// Suspect is the rows that were accepted and are worth a person's eye. Today
	// that is one thing: a Vietnamese rendering with no diacritic in it, which
	// is either a Vietnamese word that has none or an English word left where it
	// stood, and nothing here can tell those apart.
	Suspect []Row
	Rejects []Reject
	// Collisions are renderings that came back for more than one English term.
	// Soft: two notions really can share a word, and the pair is worth a look
	// rather than a refusal.
	Collisions []Collision
}

// Collision is one rendering that answered more than one term.
type Collision struct {
	TR string
	EN []string
}

// maxWords bounds a rendering.
//
// A term is a noun phrase. Six words is past anything a noun phrase needs in
// any of the three languages and well short of a sentence, which is what a
// model writes when it has decided to explain the notion instead of naming it.
const maxWords = 6

// Audit reads an answer against the batch that produced it.
//
// Every check here is on one line in isolation except the last, and every one
// of them is a failure that has been seen from a browser: an answer that
// renumbered itself, an answer that quietly corrected the English, an answer
// that gave two renderings with a slash between them, an answer that added a
// closing paragraph about how it had translated the terms.
func (b Batch) Audit(answer string) Reply {
	var reply Reply
	seen := map[int]bool{}
	byTR := map[string][]string{}

	for _, raw := range strings.Split(textguard.Strip(answer), "\n") {
		line := strings.TrimSpace(raw)
		if line == "" {
			continue
		}
		number, english, rendering, ok := parseRow(line)
		if !ok {
			// Not a row at all. A model that wrote a sentence is a model that
			// ignored the instruction, and the sentence is worth reporting once
			// rather than once per line, so only the first is kept.
			if len(reply.Rejects) == 0 || reply.Rejects[len(reply.Rejects)-1].Reason != "not a row" {
				reply.Rejects = append(reply.Rejects, Reject{Line: line, Reason: "not a row"})
			}
			continue
		}
		if number < 1 || number > len(b.Terms) {
			reply.Rejects = append(reply.Rejects, Reject{EN: english, Line: line,
				Reason: fmt.Sprintf("line %d, but this batch has %d terms", number, len(b.Terms))})
			continue
		}
		term := b.Terms[number-1]
		if seen[number] {
			reply.Rejects = append(reply.Rejects, Reject{EN: term, Line: line, Reason: "answered twice"})
			continue
		}
		seen[number] = true

		// The English column is what ties a rendering to a term. A model that
		// renumbered its answer, or that dropped a term and shifted everything
		// after it up by one, produces lines that are individually plausible
		// and attached to the wrong words; this is the check that catches it.
		if Key(english) != Key(term) {
			reply.Rejects = append(reply.Rejects, Reject{EN: term, Line: line,
				Reason: fmt.Sprintf("line %d answers %q, not %q", number, english, term)})
			continue
		}
		switch reason := badRendering(b.Lang, term, rendering); reason {
		case "":
		case unknown:
			reply.Unknown = append(reply.Unknown, term)
			continue
		case suspect:
			reply.Suspect = append(reply.Suspect, Row{EN: term, TR: rendering})
		default:
			reply.Rejects = append(reply.Rejects, Reject{EN: term, Line: line, Reason: reason})
			continue
		}
		reply.Rows = append(reply.Rows, Row{EN: term, TR: rendering})
		byTR[rendering] = append(byTR[rendering], term)
	}

	for i, term := range b.Terms {
		if !seen[i+1] {
			reply.Rejects = append(reply.Rejects, Reject{EN: term,
				Reason: fmt.Sprintf("term %d was not answered", i+1)})
		}
	}
	for _, row := range reply.Rows {
		ens := byTR[row.TR]
		if len(ens) > 1 && ens[0] == row.EN {
			reply.Collisions = append(reply.Collisions, Collision{TR: row.TR, EN: ens})
		}
	}
	return reply
}

// unknown is the reason that is not a rejection.
const unknown = "the model said it does not know"

// suspect is a Vietnamese rendering with no diacritic in it.
//
// The script test is right for Chinese and Japanese, where a rendering with no
// Han and no kana in it is English that came back untouched, and it is right
// for a Vietnamese paragraph, where the diacritics turn up in almost every
// sentence. It is wrong for a Vietnamese term. The first real run of this,
// forty terms into Vietnamese on 11 August 2026, produced exactly one: the
// model rendered "generated" as "sinh", which is the correct word and carries
// no diacritic, and a rule that threw it away would have been throwing away
// correct terminology at about one term in forty.
//
// So it is a flag and not a refusal. The row is kept and the term is listed for
// a person, because the thing this cannot tell apart is a diacritic-free
// Vietnamese word from an English one left standing.
const suspect = "no diacritic, which a Vietnamese term may or may not have"

// badRendering is why a rendering cannot be used, or "".
func badRendering(lang, term, tr string) string {
	if tr == "" {
		return "no rendering"
	}
	if strings.EqualFold(tr, "unknown") || strings.EqualFold(tr, "n/a") {
		return unknown
	}
	if leaks := textguard.Check(tr); len(leaks) > 0 {
		return "the rendering is not a rendering: " + leaks[0].Kind
	}
	// Operators come first, before anything looks at the rendering, because
	// what is wrong with them is the question and not the answer. "end" was
	// answered "kết thúc", which is the right Vietnamese for the word and the
	// wrong thing entirely for $\operatorname{End}$. See Operators.
	if Operators[strings.ToLower(strings.TrimSpace(term))] {
		return "is one of Bourbaki's operators and belongs inside the mathematics, not in the glossary"
	}
	// Mathematics the English term does not have. The second Vietnamese run
	// answered "b-algebra" with "$b$-đại số" and "bx" with "$bx$": the term was
	// mined out of prose that had lost its delimiters, and the model put them
	// back where it thought they went. A glossary row carrying a formula the
	// term does not carry would put that formula into a translation prompt as a
	// thing to write, and invariant 1 says the formulae come from the English.
	if len(mathParts(tr)) > len(mathParts(term)) {
		return "the rendering has mathematics in it that the term does not"
	}
	if n := len(strings.Fields(tr)); n > maxWords {
		return fmt.Sprintf("%d words, which is an explanation and not a term", n)
	}
	// One rendering was asked for. A slash, a semicolon or a bracket is the
	// shape of two, and picking one of them here would be this program guessing
	// at terminology, which is the one thing it must not do.
	for _, bad := range []string{"/", ";", "(", ")", "[", "]", " or ", "、", "，"} {
		if strings.Contains(tr, bad) {
			return fmt.Sprintf("more than one rendering, or a gloss: %q", bad)
		}
	}
	if Key(tr) == Key(term) {
		return "the English came back unchanged"
	}
	if !WrittenIn(lang, tr) {
		if lang == "vi" {
			return suspect
		}
		return "nothing of " + lang + " in it"
	}
	// The mathematics in a term is not translated and not moved. "$A$-module"
	// has to come back with the $A$ still in it, whatever happened to the word.
	for _, math := range mathParts(term) {
		if !strings.Contains(tr, math) {
			return fmt.Sprintf("the mathematics %s is not in the rendering", math)
		}
	}
	return ""
}

// mathParts is the inline formulas of a term.
func mathParts(term string) []string {
	var out []string
	rest := term
	for {
		i := strings.Index(rest, "$")
		if i < 0 {
			return out
		}
		j := strings.Index(rest[i+1:], "$")
		if j < 0 {
			return out
		}
		out = append(out, rest[i:i+j+2])
		rest = rest[i+j+2:]
	}
}

// WrittenIn asks whether text carries the script of a language.
//
// Chinese and Japanese prose contains Han characters or kana. Vietnamese is
// written in the Latin alphabet, so the test there is the diacritics, which
// Vietnamese uses in almost every sentence and English uses in none.
//
// This lives here rather than in the audit rules because the audit will read
// the glossary and not the other way round. A language nobody has written a
// test for is not failed for it.
func WrittenIn(lang, text string) bool {
	switch lang {
	case "zh", "ja":
		for _, r := range text {
			if unicode.Is(unicode.Han, r) || unicode.Is(unicode.Hiragana, r) || unicode.Is(unicode.Katakana, r) {
				return true
			}
		}
		return false
	case "vi":
		for _, r := range text {
			if r > unicode.MaxASCII && unicode.IsLetter(r) {
				return true
			}
		}
		return false
	}
	return true
}

// parseRow reads one answer line.
//
// The form asked for is "7 | left ideal | iđêan trái". What comes back is that,
// mostly, wrapped in whatever the model felt was tidy: a markdown table row
// with a leading and trailing bar, bold on one column, "7." for the number.
// Those are all the same answer and it would be a poor reason to throw a term
// away, so they are read. What is not read is a line with the columns in
// another order or with a separator that was not asked for: a line that has to
// be guessed at is a line that is dropped.
func parseRow(line string) (number int, english, rendering string, ok bool) {
	line = strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "|"))
	parts := strings.Split(line, "|")
	if len(parts) != 3 {
		return 0, "", "", false
	}
	head := strings.TrimSpace(parts[0])
	head = strings.TrimRight(head, ".)")
	n, err := strconv.Atoi(head)
	if err != nil {
		return 0, "", "", false
	}
	return n, clean(parts[1]), clean(parts[2]), true
}

// clean takes the decoration off a column.
func clean(text string) string {
	text = strings.TrimSpace(text)
	for _, mark := range []string{"**", "*", "`", "_"} {
		if len(text) > 2*len(mark) && strings.HasPrefix(text, mark) && strings.HasSuffix(text, mark) {
			text = strings.TrimSpace(text[len(mark) : len(text)-len(mark)])
		}
	}
	return strings.TrimSpace(strings.Trim(text, "\""))
}

// Merge puts accepted rows into a glossary, and says how many rows it changed.
//
// A rendering already in the glossary is left alone. Whatever is there was
// either curated by a person or accepted by an earlier run, and a later run
// silently overwriting it would make the file depend on the order the batches
// happened to finish in.
// Tidy applies the rules to the rows already in the file.
//
// It exists because the rules arrived after the rows did. The first two
// Vietnamese runs put 879 terms in the glossary and then reading them found two
// kinds of thing that should never have got in: Bourbaki's operators, which are
// notation and not vocabulary, and renderings carrying a formula their term
// does not have. Both were mined, both were rendered correctly as words, and
// both would do damage in a translation prompt. Writing the rules and leaving
// the rows would be writing a rule that only applies to what has not happened
// yet.
//
// A row whose English is refused goes entirely. A row whose English is fine and
// whose rendering in one language is refused loses that rendering and keeps the
// others, and goes only when it has none left. Nothing is corrected: this
// removes and it never writes terminology, because writing terminology is the
// thing this program is not allowed to do.
//
// A row with a note is left alone. Nothing in this program writes a note, so a
// note is a person's writing and the row it is on is a row a person has already
// judged. That is not a nicety. The Nullstellensatz row renders the term as the
// German word itself, because that is what Vietnamese mathematical writing does
// with it, and to the rendering rules that is the English coming back unchanged
// and a row to throw away. The rule is right in general and wrong here, and the
// note is how the file says so.
func (g *Glossary) Tidy() (dropped []Reject) {
	var keep []Term
	for _, t := range g.Terms {
		if strings.TrimSpace(t.Note) != "" {
			keep = append(keep, t)
			continue
		}
		if why := badTerm(t.EN); why != "" {
			dropped = append(dropped, Reject{EN: t.EN, Reason: why})
			continue
		}
		any := false
		for _, lang := range Langs {
			v := t.In(lang)
			if v == "" {
				continue
			}
			switch why := badRendering(lang, t.EN, v); why {
			case "", unknown, suspect:
				any = true
			default:
				dropped = append(dropped, Reject{EN: t.EN, Line: lang + ": " + v, Reason: why})
				t.Set(lang, "")
			}
		}
		if !any {
			continue
		}
		keep = append(keep, t)
	}
	g.Terms = keep
	return dropped
}

// badTerm is what is wrong with the English side of a row, if anything.
//
// Only the two things that have actually been measured coming out of the miner.
// A rule here that guessed at what a term looks like would throw away real
// vocabulary, and the miner's output is already a list for a person to read.
func badTerm(en string) string {
	term := strings.ToLower(strings.TrimSpace(en))
	switch {
	case term == "":
		return "an empty term"
	case Operators[term]:
		return "is one of Bourbaki's operators and belongs inside the mathematics, not in the glossary"
	case stop[term] || notTerms[term]:
		return "is an ordinary English word and not a term"
	}
	return ""
}

func (g *Glossary) Merge(lang string, rows []Row) (added, kept int) {
	at := map[string]int{}
	for i, t := range g.Terms {
		at[Key(t.EN)] = i
	}
	for _, row := range rows {
		i, ok := at[Key(row.EN)]
		if !ok {
			t := Term{EN: row.EN}
			t.Set(lang, row.TR)
			g.Terms = append(g.Terms, t)
			at[Key(row.EN)] = len(g.Terms) - 1
			added++
			continue
		}
		if g.Terms[i].In(lang) != "" {
			kept++
			continue
		}
		g.Terms[i].Set(lang, row.TR)
		added++
	}
	return added, kept
}
