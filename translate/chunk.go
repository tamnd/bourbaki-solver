package translate

import (
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
// Fifteen. It puts the appendix into four asks instead of one, each with a
// quarter of the mathematics, and a slip costs one ask rather than the section.
// Across the whole corpus it turns 479 chunks into 1,552, a median of 13 spans
// each and 19 at the ninetieth percentile, and the 58 span worst case is a
// single block that cannot be cut. Three times the asks is the price, and it is
// worth paying while a refusal costs a whole section.
const ChunkSpans = 15

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
	cur, curSpans := "", 0
	for _, b := range bs {
		n := spanCount(b)
		switch {
		case cur == "":
			cur, curSpans = b, n
		case len(cur)+2+len(b) <= ChunkChars && curSpans+n <= ChunkSpans:
			cur += "\n\n" + b
			curSpans += n
		default:
			out = append(out, Chunk{Body: cur})
			cur, curSpans = b, n
		}
	}
	out = append(out, Chunk{Body: cur})
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

// Join puts the answers back together in the order they were asked for.
func Join(answers []string) string {
	parts := make([]string, 0, len(answers))
	for _, a := range answers {
		if s := strings.TrimSpace(a); s != "" {
			parts = append(parts, s)
		}
	}
	return corpus.NormalizeBody(strings.Join(parts, "\n\n"))
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
// matching on a word boundary rather than a substring, and by the adherence
// report, which reads the finished files and asks the same question of the
// answer that this asks of the source.
func GlossaryBlock(g *glossary.Glossary, lang, body string) string {
	lower := strings.ToLower(body)
	type row struct{ en, tr string }
	var rows []row
	for _, t := range g.Terms {
		tr := t.In(lang)
		if tr == "" || !containsTerm(lower, strings.ToLower(t.EN)) {
			continue
		}
		rows = append(rows, row{t.EN, tr})
	}
	// Longest English first, so a reader of the list meets the phrase before
	// the word inside it.
	for i := 1; i < len(rows); i++ {
		for j := i; j > 0 && len(rows[j].en) > len(rows[j-1].en); j-- {
			rows[j], rows[j-1] = rows[j-1], rows[j]
		}
	}
	var b strings.Builder
	for _, r := range rows {
		b.WriteString(r.en)
		b.WriteString(" | ")
		b.WriteString(r.tr)
		b.WriteString("\n")
	}
	return b.String()
}

// containsTerm asks whether a term appears in the text as a term rather than as
// a run of letters inside another word. Both arguments are already lowered.
func containsTerm(text, term string) bool {
	if term == "" {
		return false
	}
	for i := 0; ; {
		j := strings.Index(text[i:], term)
		if j < 0 {
			return false
		}
		j += i
		if boundary(text, j-1) && boundary(text, j+len(term)) {
			return true
		}
		i = j + 1
	}
}

func boundary(text string, i int) bool {
	if i < 0 || i >= len(text) {
		return true
	}
	c := text[i]
	return !(c >= 'a' && c <= 'z' || c >= '0' && c <= '9')
}
