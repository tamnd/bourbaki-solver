package quality

import (
	"fmt"

	"github.com/tamnd/bourbaki-solver/katex"
)

// The publication rules are spec 12 §9: what has to be true of the corpus
// before the static site built out of it is worth reading.
//
// They are here rather than in package publish because the audit is where a
// number that has to be watched over months belongs. publish fails on the first
// formula it cannot set, which is right for a build and useless for planning:
// the question a repair pass needs answered is how many there are and where,
// and one rule that lists all of them answers it.

func init() {
	register(
		Check{ID: "P04", Group: Publication, Hard: false,
			Title: "every math span parses under KaTeX", Run: p04},
	)
}

// P04. Every math span parses under KaTeX.
//
// M04 next door is the structural reading of a span: unbalanced braces, a
// backslash with nothing after it, a script with nothing under it. It was
// written that way because there was no KaTeX to call, and it says so in its
// own comment. There is one now, embedded and running in-process with nothing
// to install, so this rule asks the renderer that will actually set the page
// rather than a model of it.
//
// It is soft, for now, and that is a measurement and not a preference. The
// corpus has failures in it today, all of them lost characters from the text
// layer, and they are M2 repair work. Spec 12 §9 says the publication rules go
// hard once the site is built; this one goes hard when the count is zero, and
// the count is in the report until then. publish itself is already hard about
// it, so nothing ships with a formula in this list.
func p04(c *Corpus) ([]Finding, error) {
	eng, err := katex.New()
	if err != nil {
		return nil, err
	}
	var out []Finding
	for _, d := range c.Docs {
		spans, _ := Math(d.Body)
		for _, s := range spans {
			// A span that never closes is M01's finding and is not this rule's.
			// Split returns the closed ones here, so what is left is a span that
			// KaTeX read to the end and would not accept.
			if _, err := eng.Render(s.Text, s.Display); err != nil {
				out = append(out, Finding{File: d.Path, Line: d.BodyLine(s.Line),
					Msg: fmt.Sprintf("KaTeX will not set it: %s: %s", err, ellipsis(s.Text, 50))})
			}
		}
	}
	return out, nil
}
