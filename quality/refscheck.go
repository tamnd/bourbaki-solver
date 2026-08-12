package quality

import (
	"fmt"

	"github.com/tamnd/bourbaki-solver/refs"
	"github.com/tamnd/bourbaki-solver/tags"
)

// The reference rules were soft while the corpus held one chapter of one Book.
// Most of what Bourbaki cites is not here yet: 553 of the 2271 references in
// chapter VIII leave the corpus, and every one of them points at a volume
// nobody has read in. Failing a build for that would be failing it for work not
// yet done.
//
// None of the three ever asked for that work. R01 counts only the references
// that stay inside the corpus, R02 asks of the ones that leave it no more than
// that they name where they went, and R03 asks that an edge that claims to
// resolve resolves to something. All three read 0 over both printings, so they
// are hard from M9 on, and what they now hold is the thing they were always
// good at: the graph is the best extraction audit this project has, and all
// three of the extraction faults repaired during M5 were found by the resolver
// rather than by reading.
//
// A chapter read in later will bring references that used to leave the corpus
// back inside it, and some of them will not resolve first time. That is the
// point of the gate. It fails on the pull request that reads the chapter in,
// where somebody is looking at the references anyway, rather than months later
// in a report nobody opens.

func init() {
	register(
		Check{ID: "R01", Group: References, Hard: true,
			Title: "every in-corpus reference resolves", Run: r01, Need: needRefs},
		Check{ID: "R02", Group: References, Hard: true,
			Title: "a reference that leaves the corpus names a Book of the Éléments",
			Run:   r02, Need: needRefs},
		Check{ID: "R03", Group: References, Hard: true,
			Title: "no reference resolves to a tag that is not in tags", Run: r03, Need: needRefs},
	)
}

func needRefs(c *Corpus) string {
	if c.Refs == nil {
		return "there is no English corpus to build a reference graph over"
	}
	return ""
}

// R01. Every in-corpus reference resolves.
//
// Two of the seven forms resolve to nothing and are meant to. A numbered
// display, "formula (35)", points at an equation, and an equation carries no
// tag because nothing in this corpus tags equations; a loc. cit. points at
// whatever the sentence before it pointed at, and the resolver does not carry
// that state. Counting either as a failure would put 150 permanent findings in
// the report and hide the 41 that are real.
func r01(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, p := range c.Refs.Unresolved {
		if p.Form == refs.FormFormula || p.Form == refs.FormLocCit {
			continue
		}
		out = append(out, Finding{File: p.File, Line: p.Line,
			Msg: fmt.Sprintf("%q does not resolve: %s", p.Raw, p.Reason)})
	}
	return out, nil
}

// R02. A reference that leaves the corpus names a Book of the Éléments.
//
// An edge with no Book is a citation the parser read as leaving the corpus and
// could not say where to, which means it read something that is not a citation
// at all. The count of the ones that do name a Book is not a fault: it is the
// list of what to ingest next.
func r02(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, e := range c.Refs.Edges {
		if e.How != refs.OutOfCorpus {
			continue
		}
		if e.Book == "" {
			out = append(out, Finding{File: e.File, Line: e.Line,
				Msg: fmt.Sprintf("%q leaves the corpus and names no Book", e.Raw)})
		}
	}
	return out, nil
}

// R03. No reference resolves to a tag that is not in tags.
//
// A dangling edge is worse than an unresolved one. Unresolved is honest; an
// edge that points at a tag nothing carries is a citation that looks like it
// works, and it would go on looking like it works through every translation and
// every solution built on top of it.
func r03(c *Corpus) ([]Finding, error) {
	live := map[string]bool{}
	for _, e := range append(append([]tags.Entry(nil), c.Tags.Tags...), c.Tags.New...) {
		live[string(e.Tag)] = true
	}
	var out []Finding
	for _, e := range c.Refs.Edges {
		if e.To == nil || *e.To == "" {
			continue
		}
		if !live[*e.To] {
			out = append(out, Finding{File: e.File, Line: e.Line,
				Msg: fmt.Sprintf("%q resolves to %s, which is in no line of tags", e.Raw, *e.To)})
		}
	}
	return out, nil
}
