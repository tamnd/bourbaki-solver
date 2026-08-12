package solve

import (
	"fmt"
	"strings"
)

// Render writes the context as the model will read it.
//
// Every piece is headed with what it is and where it came from, and the tag is
// on the heading. That is not bookkeeping shown to the model by accident: the
// solution has to name the results it used by tag, and it cannot name what it
// was never told the name of. A context assembled into one undifferentiated
// wall of Markdown would leave the model to guess which paragraph was the
// exercise.
//
// Nothing is summarised or paraphrased on the way through. What the model reads
// is the corpus, character for character, so that a fault in a solution is
// either the model's or the extraction's and never the assembler's.
func (c *Context) Render() string {
	var b strings.Builder
	for _, p := range c.Pieces {
		if p.Kind == Outside {
			continue
		}
		fmt.Fprintf(&b, "%s %s\n\n", openMark, head(p))
		b.WriteString(p.Text)
		fmt.Fprintf(&b, "\n\n%s\n\n", closeMark)
	}
	c.renderOutside(&b)
	c.renderNamed(&b)
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// openMark and closeMark bracket a piece.
//
// A heading will not do it. The § is carried whole and it carries its own
// headings, "## § 1. ARTINIAN MODULES AND NOETHERIAN MODULES" and the subsection
// headings under it, so a context that labelled its pieces with headings would
// be handing the model a document in which the boundary between one piece and
// the next is a heading among two hundred others. These two lines cannot occur
// in the corpus, which is what makes them worth the ugliness.
const (
	openMark  = ">>>"
	closeMark = "<<<"
)

func head(p Piece) string {
	var s string
	switch p.Kind {
	case TheExercise:
		s = "the exercise to solve"
	case Sibling:
		s = "an earlier exercise of the same section"
	case TheSection:
		s = "the section this exercise belongs to"
	case Reference:
		s = "cited by the material above"
	}
	if p.Label != "" {
		s += ": " + p.Label
		if p.Tag != "" {
			s += ", tag " + p.Tag
		}
	}
	return s
}

// outsideInstruction is spec 07 §3.1's rule for a citation the corpus cannot
// answer.
//
// The corpus is one chapter of a nine chapter Book inside a series of ten, so
// most of what Bourbaki cites is not here and will not be for a long time.
// There are three things to do about that and only one of them is honest.
// Refusing the exercise wastes the ones where the citation is a standard fact
// about sets. Saying nothing lets the model quote a numbered proposition it has
// only half remembered, in the Éléments' own voice, in a corpus whose whole
// claim is that the text is the text. Naming the citation and asking the model
// to say out loud that it is working from the standard content is the third,
// and it is what makes the difference between a solution and a citation of a
// solution visible to a reader afterwards.
const outsideInstruction = `These are cited by the material above and are not in the corpus, so their
statements are not available to you. Where you use one, use the standard
content of the result and say so explicitly in the solution, in the form "by
the standard <result> of <reference>". Do not quote a numbered statement of a
volume you have not been shown as though you had read it.`

func (c *Context) renderOutside(b *strings.Builder) {
	var out []Piece
	for _, p := range c.Pieces {
		if p.Kind == Outside {
			out = append(out, p)
		}
	}
	if len(out) == 0 {
		return
	}
	b.WriteString("References that leave the corpus\n\n")
	b.WriteString(outsideInstruction)
	b.WriteString("\n\n")
	for _, p := range out {
		fmt.Fprintf(b, "- %s\n", p.Raw)
	}
	b.WriteString("\n")
}

// renderNamed tells the model what is in the corpus and is not in front of it.
//
// A context that silently drops the tail of its references reads to the model
// exactly like a context that had no tail, and a solution that goes wrong for
// want of a definition it was never shown looks like a solution that went
// wrong. This is what makes the difference legible, to the model while it works
// and to whoever reads the run afterwards. The reason is printed with each one
// because the two reasons want different things: a reference the cap dropped is
// a reference somebody could raise the cap for, and a page citation that
// narrowed only to a § is one the resolver could be made to read better.
func (c *Context) renderNamed(b *strings.Builder) {
	if len(c.Named) == 0 {
		return
	}
	b.WriteString("Cited, in the corpus, and not included here\n\n")
	b.WriteString("These are in the corpus and are not in front of you, for the " +
		"reason given against each. If the solution turns on one of them, say so " +
		"rather than guessing at what it says.\n\n")
	for _, p := range c.Named {
		name := p.Label
		if p.Tag != "" {
			name += ", tag " + p.Tag
		}
		fmt.Fprintf(b, "- %s: %s\n", name, p.Why.Sentence(c.Options.MaxChars))
	}
	b.WriteString("\n")
}
