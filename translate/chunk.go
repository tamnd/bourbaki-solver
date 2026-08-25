package translate

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/mathtex"
)

// A section does not go over in one piece, and the spec said it would.
//
// Spec 2115-06 §1 makes the section the unit of work, on the reasoning that a §
// is eight to twenty book pages and that is one coherent context. The reasoning
// is right and the unit is wrong, and the measurement is what settles it. The
// twenty-seven English bodies of chapter VIII run from 1,249 characters to
// 97,520, with a median of 35,072. The prompt without a body is about 5,000
// characters and the full Vietnamese glossary is another 11,405. So the largest
// section asks a browser composer to swallow 114,000 characters and asks for
// about as many back, in one answer, in one call that must not be interrupted
// for the twenty minutes it takes. The glossary run measured its prompts at
// 2,476 characters and they went through; nothing here has been measured at
// anything near a hundred thousand.
//
// So the unit that goes over is a chunk of a section, and the section is put
// back together from the chunks. Nothing is lost by it: every rule in Audit
// compares blocks, headings, math spans and references in order, and all four
// of those are the same whether the comparison is one chunk against its source
// or a whole section against its own. A chunk boundary is a blank line, which
// is a boundary the Markdown already has, so joining the answers with a blank
// line between them reproduces exactly the block structure of the English.
//
// What is lost is context: the model translating chunk four has not read chunk
// three. The glossary is what makes up for that, and it is the reason the
// glossary comes first in the order of work rather than after.

// ChunkChars is how much body goes in one question.
//
// Six thousand. A block is never split, so this is a target and not a limit: a
// chunk is blocks added until the next one would take it past this, and a
// single block longer than this goes over on its own. The number comes from the
// prompt arithmetic above rather than from a failure, and it wants revisiting
// with a measurement the first time a chunk is refused for length.
const ChunkChars = 6000

// ChunkSpans is how many pieces of mathematics go in one question.
//
// Characters were the only budget at first and they were the wrong one. The
// appendix on the Nullstellensatz is a 3,503 character body, one chunk with
// room to spare under ChunkChars, and it carries 45 math spans. It was refused
// four times running, every time for the mathematics: 48 spans against 45, then
// 41 against 45, then 46 against 45, then span 23 coming back as "\mathfrak{m}"
// where the English has "s". The prose was fine every time. What the model
// cannot do is copy forty-five formulas byte for byte while translating the
// sentences between them, and throwing away all forty-five because it fumbled
// one is a bad trade.
//
// Measured over the 344 English files: a chunk cut at ChunkChars alone carries a
// median of 20 spans, a tenth of them carry more than 108, and the worst carries
// 154. A block carries a median of 2, and 99 in 100 carry 27 or fewer. So the
// cap is enforceable at the boundary the chunker already cuts on, for all but a
// handful of blocks that go over on their own because a block is never split.
//
// Fifteen was the first answer. It put the appendix into four asks instead of
// one, each with a quarter of the mathematics, so a slip cost one ask rather
// than the section. Across the whole corpus it turned 479 chunks into 1,552, a
// median of 13 spans each and 19 at the ninetieth percentile, and the 58 span
// worst case was a single block that cannot be cut. Three times the asks was
// the price, and it was worth paying while a refusal cost a whole section.
//
// Sixty is the answer now, and the thing that changed is not the models. It is
// that an ask stopped being cheap. Of 17,123 translation asks recorded in
// reports/ask-usage.jsonl between 19 and 25 August, 33 to 47 percent a day were
// lost before the question was read at all: the account was out of quota, or
// answering 403, or signed out. That is a bill per message and not per
// character, so the arithmetic that priced three times the asks as worth paying
// no longer holds. Three times the asks is now three times the messages spent
// against a quota that is eating half of them.
//
// The measurement the paragraph above asked for is on the same file. Refusal
// against the number of spans in the chunk, over the 7,598 chunks with a
// question archived beside them:
//
//	spans     chunks   first ask refused   ever accepted
//	0 to 4       777               9.1%           99.5%
//	5 to 9     1,426              11.2%           99.3%
//	10 to 14   3,157              15.5%           99.7%
//	15 to 19   1,661              16.9%           99.5%
//	20 to 29     460              15.4%           99.6%
//	30 to 44     103              22.3%          100.0%
//
// It is flat. Refusal goes from 15 percent to 22 across the range where the
// sample is worth anything, and every band lands eventually. The Nullstellensatz
// appendix is in the last row and it is the hard end of it: a chunk over the cap
// today is a chunk the cutter could not split, which means one block carrying
// all of that mathematics at once. A chunk of sixty spans built out of twenty
// small blocks is an easier question than the appendix, not a harder one.
//
// What sixty buys, measured on the 102 English sections of Algebra: 4,459 chunks
// at a cap of fifteen, 2,362 at thirty, 1,164 at sixty, 765 at a hundred. The
// median chunk goes from 658 characters to 2,873, which with the 6,700
// characters of instructions every question carries is a 9,573 character ask.
// Measured on the same usage log, once the account failures are set aside, an
// ask under 10,000 characters is answered 91 percent of the time in a mean of 73
// seconds, and one over 11,000 falls to 70 percent and 232 seconds. So sixty
// sits inside the flat band and a hundred does not.
//
// The extrapolation is one band wide and it is the part to watch. There are 117
// archived chunks over thirty spans and all of them are single blocks, so
// nothing here has measured a sixty span chunk made of many. If refusal at sixty
// comes back worse than the high twenties, the number to try next is thirty,
// which is covered by the table above and still halves the asks.
const ChunkSpans = 60

// A Chunk is one question's worth of a section body.
type Chunk struct {
	// Index is one based and Of is how many chunks the section came to. Both
	// go in the archive file name, so a run can be read back in order.
	Index, Of int
	// Body is the English, blocks joined by a blank line, with no leading or
	// trailing blank.
	Body string
}

// Chunks splits a body at block boundaries.
//
// The blocks are the ones Audit counts, from the same function, because a split
// that disagreed with the audit about where a block ends would produce chunks
// that each pass and a section that does not.
//
// Two budgets, and either one full ends the chunk. A block is never split, so
// both are targets rather than limits and a single block over either of them
// goes over on its own.
func Chunks(body string) []Chunk {
	bs := blocks(body)
	if len(bs) == 0 {
		return nil
	}
	var out []Chunk
	var cur []string
	curLen, curSpans := 0, 0
	for _, b := range bs {
		n := spanCount(b)
		switch {
		case len(cur) == 0:
			cur, curLen, curSpans = []string{b}, len(b), n
		case curLen+2+len(b) <= ChunkChars && curSpans+n <= ChunkSpans:
			cur = append(cur, b)
			curLen += 2 + len(b)
			curSpans += n
		default:
			out = append(out, Chunk{Body: joinBlocks(cur)})
			cur, curLen, curSpans = []string{b}, len(b), n
		}
	}
	out = append(out, Chunk{Body: joinBlocks(cur)})
	for i := range out {
		out[i].Index, out[i].Of = i+1, len(out)
	}
	return out
}

// spanCount is how many pieces of mathematics a block holds. It uses the same
// splitter Audit uses, because a chunker that counted spans differently from
// the rule that refuses them would budget for the wrong thing. An unclosed span
// is counted, since the audit will compare it too.
func spanCount(block string) int {
	spans, unclosed := mathtex.Split(block)
	n := len(spans)
	if unclosed != nil {
		n++
	}
	return n
}

// Blocks is the paragraphs of a body, by the same reckoning the chunker and the
// audit use. It is exported for the French pass, which pairs the paragraphs of a
// section with the paragraphs of its French printing and would pair them wrong
// if it split the text its own way.
func Blocks(body string) []string { return blocks(body) }

// Join puts the answers back together in the order they were asked for.
func Join(answers []string) string {
	parts := make([]string, 0, len(answers))
	for _, a := range answers {
		if s := strings.TrimSpace(a); s != "" {
			parts = append(parts, s)
		}
	}
	return corpus.NormalizeBody(joinBlocks(parts))
}

// GlossaryBlock is the terminology for one chunk, one term to a line.
//
// The spec asks for the whole glossary in every prompt, and at 457 terms that
// is 11,405 characters against a 6,000 character chunk: two thirds of the
// question would be terms the chunk does not contain. So it is the terms that
// appear, matched on the English as the book spells it, longest first so that
// "law of composition" is found before "composition".
//
// The cost of getting this wrong is a term rendered its own way in one chunk,
// which is what the glossary exists to prevent. It is guarded against by
// matching on a word boundary rather than a substring, and by L06, which reads
// the finished files and asks the same question of the answer that this asks of
// the source. The matcher is glossary.Mentions, one copy shared with L06, so
// that the rule cannot hold a section to a term the prompt never showed it.
//
// This list is not masked and L06's is. A prompt that carries both "ring" and
// "semisimple ring" tells the model something true twice; a rule that held the
// answer to both would want a rendering of "ring" inside a phrase that has no
// room for one. So the prompt is generous and the rule is strict, in that
// order, and the difference is deliberate.
// source is the language the body is written in, and it is what the left column
// is matched and printed in. For an English body that is the English headword,
// which is every row this has ever built. A French body is matched on the French
// the printing spells the term with, and the right column is still what to
// write, so the block reads the same way in both directions.
func GlossaryBlock(g *glossary.Glossary, source, lang, body string) string {
	lower := strings.ToLower(body)
	type row struct{ from, tr string }
	var rows []row
	for _, t := range g.Terms {
		from, tr := t.In(source), t.In(lang)
		if from == "" || tr == "" || !glossary.Mentions(lower, glossary.Key(from)) {
			continue
		}
		rows = append(rows, row{from, tr})
	}
	// Longest first, so a reader of the list meets the phrase before the word
	// inside it.
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && len(rows[j].from) > len(rows[j-1].from); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(r.from)
		b.WriteString(" | ")
		b.WriteString(r.tr)
		b.WriteString("\n")
	}
	return b.String()
}

// GlossaryDigest is the terminology this section was actually shown, hashed.
//
// glossary_version was the wrong question to ask of a translated file. It moves
// whenever any rendering anywhere moves, so pinning "common zero", a phrase that
// occurs in one appendix, marked all 27 sections of chapter VIII stale and would
// have bought 27 runs to fix 1 file. Measured on this corpus: the move from
// version 2 to version 5 changes what 14 of the 27 sections are shown and leaves
// the other 13 alone, and of the three edits in it, "common zero" reaches 1
// section and "algebraic over" reaches 3.
//
// So the file records what it was shown rather than which glossary it came from,
// and it is stale when what it would be shown today differs. A common word is
// still expensive, and should be: taking "ring" out changes the block of 26 of
// the 27 and "module" of 23, because those really are in nearly every section.
//
// The body hashed is the English, not the translation, because the question is
// what the prompt carried and the prompt is built from the English. It is the
// whole section rather than a chunk, so that re-cutting a section into different
// chunks does not restale it.
func GlossaryDigest(g *glossary.Glossary, source, lang, body string) string {
	sum := sha256.Sum256([]byte(GlossaryBlock(g, source, lang, body)))
	return hex.EncodeToString(sum[:])
}
