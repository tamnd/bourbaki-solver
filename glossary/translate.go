package glossary

import (
	"fmt"
	"regexp"
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

	// EN and FR are one paragraph of a section in both printings, and they turn
	// the question into a different one. Empty is the question this started as,
	// which asks what a term is called in a language. Set is the aligned
	// question, which puts the French Bourbaki printed in front of the model and
	// asks which of those words the term is, and refuses an answer that is not
	// in the passage. Only the French column can be asked this way, because it
	// is the only one there is a printing of.
	EN, FR string
}

// Aligned says whether this batch carries the passage a term is to be found in.
func (b Batch) Aligned() bool { return b.FR != "" }

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
	if b.Aligned() {
		return prompt.GlossaryAligned(b.EN, b.FR, list.String())
	}
	return prompt.Glossary(Language(b.Lang), languageNote(b.Lang), list.String())
}

// languageNote is what is true of one column and not of the other three.
//
// Only French has one. The other three are languages somebody is being asked to
// write Bourbaki in, and the question is what a mathematician writing in them
// would say. French is a language Bourbaki is already written in, and the
// question is not what a French mathematician would say but what these books
// say, which is a matter of fact and not of judgement. Without the note the
// model answers the wrong question, and it answers it plausibly.
func languageNote(lang string) string {
	if lang != "fr" {
		return ""
	}
	return "These books were written in French and the English is a translation" +
		" of them, so the French of a term is not a rendering to be chosen. It is" +
		" the word Bourbaki printed, and what is wanted is that word. Where the" +
		" French spells it as the English does, module or radical, write it as it" +
		" is spelled rather than reaching for something that differs."
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
		// The passage is the ground the aligned question stands on. A phrase that
		// is not in it is the model writing French rather than reading it, which
		// is the failure this whole way of asking exists to rule out.
		if b.Aligned() && !inPassage(b.FR, rendering) {
			reply.Rejects = append(reply.Rejects, Reject{EN: term, Line: line,
				Reason: fmt.Sprintf("%q is not in the French passage", rendering)})
			continue
		}
		switch reason := badRendering(b.Lang, term, rendering); reason {
		case "":
		case unknown:
			reply.Unknown = append(reply.Unknown, term)
			continue
		case suspect, shared:
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

// shared is a French rendering spelled the way the English is.
//
// Coming back with the English unchanged is a model failing to answer, and for
// the three target languages that is all it can be. French is the one column
// where it is often the truth: module is module, radical is radical, and the
// English of those volumes took the word over from the French in the first
// place. A rule that threw those away would empty the column of exactly the
// terms both printings agree about.
//
// So it is a flag and not a refusal, on the same ground as suspect: what this
// cannot tell apart is a French word that looks English from an English word
// left standing, and a person can.
const shared = "spelled as the English is, which a French term may or may not be"

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
		if lang == "fr" {
			return shared
		}
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

// inPassage asks whether a phrase is in the paragraph it was supposed to be
// read out of.
//
// Loosely, and deliberately. The model is asked for the singular of a phrase the
// passage may have in the plural, and French marks a plural with a letter on the
// end, so the last word is matched from its front rather than whole: anneau
// finds anneaux and morphisme finds morphismes. Case and the accents are left
// alone, since a French term keeps them and a rendering that lost them is a
// rendering somebody would have to repair.
func inPassage(passage, phrase string) bool {
	words := strings.Fields(strings.ToLower(strings.TrimSpace(phrase)))
	if len(words) == 0 {
		return false
	}
	text := strings.ToLower(passage)
	stem := strings.Join(words, " ")
	if strings.Contains(text, stem) {
		return true
	}
	// The other direction. A passage in the singular answered in the plural is
	// the same phrase with a letter on the end of the last word, so that letter
	// comes off and the passage is searched again. The singular answered to a
	// plural passage needs nothing: anneau is inside anneaux already.
	last := words[len(words)-1]
	if len(last) < 4 {
		return false
	}
	words[len(words)-1] = last[:len(last)-1]
	return strings.Contains(text, strings.Join(words, " "))
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
			case "", unknown, suspect, shared:
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
// possessiveRE is a possessive the miner mangled: it used to write an
// apostrophe as a hyphen, so "Schur's lemma" came out "schur-s lemma". Nine
// rows went into the glossary spelled that way and six of them are real terms.
// A term is found in a section by literal search, so not one of the nine could
// ever be found, could reach a translation prompt, or could be checked for.
// They come out here and go back in correctly spelled on the next mining pass.
var possessiveRE = regexp.MustCompile(`(^|[a-z])-s($| )`)

func badTerm(en string) string {
	term := strings.ToLower(strings.TrimSpace(en))
	switch {
	case term == "":
		return "an empty term"
	case Operators[term]:
		return "is one of Bourbaki's operators and belongs inside the mathematics, not in the glossary"
	case stop[term] || notTerms[term]:
		return "is an ordinary English word and not a term"
	case possessiveRE.MatchString(term):
		return "carries -s where the book prints an apostrophe, which is the miner's old reading of Schur's, and the string occurs nowhere in the corpus"
	}
	// A phrase breaks at an operator, so a phrase with one inside it is not a
	// phrase the miner would hand back today. The rows that carry one were
	// mined before the operator list was written and they are the worst rows in
	// the file: "end generated" is rendered "kết thúc sinh bởi", finish
	// generated by, and "tr tr tr" is rendered "vết vết vết". Fourteen of them
	// went in with the first Vietnamese run.
	//
	// This is asked after the whole term, so the message can say which word did
	// it, which is what a person reading the list needs in order to tell a row
	// that wants deleting from an operator that wants adding to the list.
	for w := range strings.FieldsSeq(term) {
		if Operators[w] {
			return fmt.Sprintf("has %q in it, which is one of Bourbaki's operators, and a phrase does not run through an operator", w)
		}
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
