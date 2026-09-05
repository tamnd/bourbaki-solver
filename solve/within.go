package solve

import (
	"fmt"
	"slices"
	"sort"
	"strings"
)

// There is a ceiling on how much can be put to the model in one message, and it
// is well under what an exercise of this corpus is worth showing.
//
// It was found by measurement and not in any documentation. On the live fleet,
// truth judge questions of 36 569 and 38 033 characters of Bourbaki were
// answered, one of 40 033 came back as the service's own error page, and 42 277
// did so every time it was sent, on two hosts, over an hour. Filler prose of
// 59 456 characters was answered in a minute, so the ceiling is not on
// characters: prose runs about 4.3 characters to the token and this material
// runs about 2.2 to 2.5, which puts every one of those observations on the right
// side of a cap near sixteen thousand tokens.
//
// That is a fact about the service and it will move. What does not move is the
// shape of the problem it leaves: of the 317 exercises of this printing the
// median context is 44 780 characters and the largest is 130 961, so this is not
// a tail to be handled, it is nearly every exercise. Something has to be left
// out, and the only question is whether the model is told what.

// OverAsk is a piece left out because the whole question would not have fit.
//
// It is not OverCap. OverCap is spec 07 §3.1's limit on the references, which is
// a judgement about how much of the closure is worth carrying and is the same
// for every call. This is the service refusing to read a long message, it is
// measured against the whole question and not against the references, and the
// same context reaches the candidate call whole and the truth judge thinned,
// because the truth judge's question also carries a reference and a solution.
const OverAsk Reason = "over-ask"

// RenderWithin writes the context as the model will read it, small enough to be
// read at all.
//
// cited is what the trimming is read against: the judge calls carry a solution
// and a worked reference, and whatever either of them leans on is what the judge
// cannot check the work without. It is empty on the calls that have not got an
// answer in front of them yet.
//
// What goes is what nothing points at, before what something does, and the kind
// of the piece does not come into it. That is the whole rule. An earlier
// exercise of the § that this exercise never mentions is worth less than a
// proposition it cites, which is worth less than a proposition the solution in
// front of the judge turns on, and whether a thing is an exercise, a proposition
// or a paragraph of the § it was printed in says nothing about which of those it
// is. Exercise 17 of § 16 is what taught this: it carries sixteen earlier
// exercises and a § of forty statements, all of it four times over the limit,
// and it says "use Exercise 14" in its own second line.
//
// The exercise itself is never dropped. Nothing is dropped silently: a whole
// piece that goes moves into the block that says what is in the corpus and is
// not in front of you, and a statement cut out of the § leaves its name behind
// where it stood.
// A limit of zero or below is a room of nothing and not an absence of a limit,
// and the two shared a branch here. Every caller works the room out by
// subtraction, limit less what the rest of the question already takes, so a
// call whose instructions, reference and candidate solution fill the limit on
// their own asks for a room of zero or below. That is the tightest a question
// is ever assembled and the one place trimming earns its keep, and the branch
// answered it by returning the whole context. Asking for a room of one
// character trimmed this context to 2491; asking for a room of none returned
// 10856 of it. The unlimited case never reaches here: the engine returns Render
// itself when its limit is negative, before any room is worked out.
func (c *Context) RenderWithin(limit int, cited string) string {
	out := c.Render()
	if len(out) <= limit {
		return out
	}
	cited += "\n" + c.Exercise()
	section := slices.IndexFunc(c.Pieces, func(p Piece) bool { return p.Kind == TheSection })
	var cuts []span
	if section >= 0 {
		cuts = statements(c.Pieces[section].Text)
	}

	gone, cut := map[int]bool{}, map[int]bool{}
	for _, g := range c.order(cuts, cited) {
		if g.span >= 0 {
			cut[g.span] = true
		} else {
			gone[g.piece] = true
		}
		if out = c.less(gone, cut, section, cuts); len(out) <= limit {
			break
		}
	}
	return out
}

// give is one thing that can be given up: a whole piece of the context, or one
// statement of the §.
type give struct {
	piece, span int
	keep, times int
	size        int
}

// order is everything that can go, in the order it goes.
//
// Least kept first, then least often pointed at, then longest first, because
// the point of dropping is to fit and giving up four short ones to keep one long
// one is four things lost for nothing.
func (c *Context) order(cuts []span, cited string) []give {
	var out []give
	for i, p := range c.Pieces {
		switch p.Kind {
		case TheExercise, TheSection, Outside:
			continue
		}
		r := c.Cites[p.Label]
		if r.Depth == 0 || (p.Depth > 0 && p.Depth < r.Depth) {
			r.Depth = p.Depth
		}
		out = append(out, give{piece: i, span: -1, size: len(p.Text), times: r.Times,
			keep: keep(r.Depth, p.Label != "" && strings.Contains(cited, p.Label))})
	}
	for i, s := range cuts {
		r := c.Cites[s.label]
		out = append(out, give{piece: -1, span: i, size: len(s.text), times: r.Times,
			keep: keep(r.Depth, named(s, cited))})
	}
	sort.SliceStable(out, func(i, j int) bool {
		switch {
		case out[i].keep != out[j].keep:
			return out[i].keep < out[j].keep
		case out[i].times != out[j].times:
			return out[i].times < out[j].times
		}
		return out[i].size > out[j].size
	})
	return out
}

// keep is how long a thing is held on to: 0 for what nothing points at, 1 for
// what is reached through something else, 2 for what the exercise cites itself,
// 3 for what the work in front of the judge names.
//
// The middle two are read off the reference graph, which is the only thing that
// knows them. A citation from an exercise into a proposition of its own § is
// never carried as a reference, because the § is in the context whole, and an
// earlier exercise it cites is carried as a sibling rather than as a citation,
// so in both cases the fact that the exercise pointed at it survives nowhere
// else.
func keep(depth int, named bool) int {
	switch {
	case named:
		return 3
	case depth == 1:
		return 2
	case depth > 1:
		return 1
	}
	return 0
}

// less renders the context with some of it given up.
//
// It is built afresh from the whole context every time rather than cut down in
// place, so that what comes out depends on which things were dropped and not on
// what order they were dropped in.
func (c *Context) less(gone, cut map[int]bool, section int, cuts []span) string {
	out := &Context{Label: c.Label, Tag: c.Tag, Lang: c.Lang, Options: c.Options,
		Cites: c.Cites, Named: slices.Clone(c.Named)}
	for i, p := range c.Pieces {
		switch {
		case gone[i]:
			p.Text, p.Why = "", OverAsk
			out.Named = append(out.Named, p)
		case i == section && len(cut) > 0:
			p.Text = rebuild(p.Text, cuts, cut)
			out.Pieces = append(out.Pieces, p)
		default:
			out.Pieces = append(out.Pieces, p)
		}
	}
	return out.Render()
}

// span is one statement of a § and where it sits in the file.
type span struct {
	label, tag string
	// name is how the book itself prints the statement, "Definition 1" or
	// "Proposition 3", off the heading and before the brace.
	name string
	from int
	text string
}

func statements(body string) []span {
	at := statementRE.FindAllStringSubmatchIndex(body, -1)
	out := make([]span, 0, len(at))
	for i, m := range at {
		end := len(body)
		if i+1 < len(at) {
			end = at[i+1][0]
		}
		s := span{label: body[m[4]:m[5]], from: m[0], text: body[m[0]:end],
			name: strings.TrimSpace(body[m[2]:m[3]])}
		if m[6] >= 0 {
			s.tag = body[m[6]:m[7]]
		}
		out = append(out, s)
	}
	return out
}

// named says whether the work in front of the judge names this statement: by its
// permanent label, by its tag, or the way the book itself prints it.
//
// The third of those was missing and it cost exercise 6 of § 1 a part. The
// solution proves d) out of Definition 1 and Definition 2, cites them as
// "Definition 2 of § 1" and "Definition 1(ii)", which is exactly what the
// candidate prompt asks for, and names neither the label nor the tag anywhere,
// because a solution is mathematics and a tag is bookkeeping. So both
// definitions ranked as things nothing pointed at, both were the first out when
// the question would not fit, and the truth judge failed part d) for relying on
// "numbered definitions not included in the supplied section". It was right to.
// Nothing had shown them to it.
//
// The match is on the printed name and is deliberately loose. "Proposition 3 of
// § 4" holds on to this §'s Proposition 3 as well, and a § that prints Example 5
// twice holds on to both. Keeping a statement the work merely mentions costs
// some room; dropping one it turns on costs the judgement.
func named(s span, cited string) bool {
	if s.label != "" && strings.Contains(cited, s.label) {
		return true
	}
	if s.tag != "" && strings.Contains(cited, s.tag) {
		return true
	}
	return s.name != "" && mentions(cited, s.name)
}

// mentions is Contains that does not let Proposition 1 be found inside
// Proposition 12. Bourbaki numbers from one and reaches two figures in a long §,
// so the digit after matters.
func mentions(text, name string) bool {
	for at := 0; ; {
		i := strings.Index(text[at:], name)
		if i < 0 {
			return false
		}
		at += i + len(name)
		if at == len(text) || text[at] < '0' || text[at] > '9' {
			return true
		}
	}
}

// rebuild writes the § back with the dropped statements replaced by their names.
//
// Cutting the § by the statement rather than by the character is what keeps it a
// text. A § truncated at twenty thousand characters ends in the middle of a
// proof and the model cannot tell a cut from an end, while a § with Proposition
// 4 lifted out of it and its name left in its place is a § with a proposition
// named and not shown, which is a thing this corpus already knows how to say.
func rebuild(whole string, cuts []span, cut map[int]bool) string {
	var b strings.Builder
	b.WriteString(whole[:cuts[0].from])
	for i, s := range cuts {
		if !cut[i] {
			b.WriteString(s.text)
			continue
		}
		name := s.label
		if s.tag != "" {
			name += ", tag " + s.tag
		}
		fmt.Fprintf(&b, "%s\n\n", left(name))
	}
	return b.String()
}

// left is what stands where a statement stood.
func left(name string) string {
	return fmt.Sprintf("[%s, is a statement of this § and is not printed here, "+
		"because the whole question has a length limit. It is in the corpus. If "+
		"the work turns on it, say so rather than guessing at what it says.]", name)
}
