package quality

import (
	"fmt"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/tags"
)

// A solution is the one thing in this corpus that no human wrote and no book
// printed. Everything else is a transcription, and a transcription can be
// checked against the page; a solution can only be checked against itself and
// against the judges that passed it.
//
// All four rules are soft. A wrong solution is not a corruption of the corpus,
// it is a wrong solution, and it should be marked unverified and left in rather
// than turning the build red for the whole repository.
//
// Nothing is solved yet, so all four report not run. M8 is what fills them.

func init() {
	register(
		Check{ID: "X01", Group: Solutions, Hard: false,
			Title: "a solution's tag is a tag, and it is an exercise", Run: x01, Need: needSolutions},
		Check{ID: "X02", Group: Solutions, Hard: false,
			Title: "a status is a fact about the judges", Run: x02, Need: needSolutions},
		Check{ID: "X03", Group: Solutions, Hard: false,
			Title: "every tag a solution says it uses exists", Run: x03, Need: needUses},
		Check{ID: "X04", Group: Solutions, Hard: false,
			Title: "no provider leakage and no meta-commentary", Run: x04, Need: needSolutions},
	)
}

func needSolutions(c *Corpus) string {
	for _, d := range c.Docs {
		if d.Kind == KindSolution {
			return ""
		}
	}
	return "the corpus has no solutions yet"
}

// needUses says why X03 cannot run. Solutions may exist and none of them name a
// tag, which is not the same as the rule passing.
func needUses(c *Corpus) string {
	if why := needSolutions(c); why != "" {
		return why
	}
	for _, d := range c.Docs {
		if d.Kind == KindSolution && d.Solution != nil && len(d.Solution.Uses) > 0 {
			return ""
		}
	}
	return "no solution names a tag it uses, so there is nothing to resolve"
}

// X01. A solution's tag is a tag, and it is an exercise.
//
// A solution is filed under the exercise's permanent label so that it follows
// the exercise through a renumbering. One filed under a tag that names a
// proposition is a solution to something that was never set.
func x01(c *Corpus) ([]Finding, error) {
	byTag := map[string]string{}
	for _, e := range append(append([]tags.Entry(nil), c.Tags.Tags...), c.Tags.New...) {
		byTag[string(e.Tag)] = e.Label
	}
	var out []Finding
	for _, d := range c.Docs {
		if d.Kind != KindSolution || d.Solution == nil {
			continue
		}
		if d.Solution.Tag == "" {
			out = append(out, Finding{File: d.Path, Line: 1, Msg: "records no tag"})
			continue
		}
		label, ok := byTag[d.Solution.Tag]
		if !ok {
			out = append(out, Finding{File: d.Path, Line: 1,
				Msg: fmt.Sprintf("tag %s is in no line of tags", d.Solution.Tag)})
			continue
		}
		ref, err := corpus.ParseLabel(label)
		if err != nil {
			out = append(out, Finding{File: d.Path, Line: 1,
				Msg: fmt.Sprintf("tag %s names %q, which does not parse: %v", d.Solution.Tag, label, err)})
			continue
		}
		if ref.Kind != corpus.KindExercise {
			out = append(out, Finding{File: d.Path, Line: 1,
				Msg: fmt.Sprintf("tag %s names %s, which is a %s and not an exercise",
					d.Solution.Tag, label, ref.Kind)})
		}
	}
	return out, nil
}

// X02. A status is a fact about the judges.
//
// status is what everything downstream reads: the coverage table, the
// scorecard, and whether the site prints a solution as believed or as offered.
// So it has to be a fact rather than a hope. A solution marked verified with an
// empty judge field is a solution nothing checked, and a partial with no parts
// is the word doing no work at all.
func x02(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		if d.Kind != KindSolution || d.Solution == nil {
			continue
		}
		m := d.Solution
		switch m.Status {
		case corpus.StatusVerified:
			for _, judge := range []struct{ field, got string }{
				{"truth_judge", m.TruthJudge}, {"audit_judge", m.AuditJudge},
			} {
				if judge.got != "pass" {
					out = append(out, Finding{File: d.Path, Line: 1,
						Msg: fmt.Sprintf("is verified and %s is %q", judge.field, judge.got)})
				}
			}
			for _, p := range m.Parts {
				if p.Status != corpus.StatusVerified {
					out = append(out, Finding{File: d.Path, Line: 1,
						Msg: fmt.Sprintf("is verified and part %s is %s", p.ID, p.Status)})
				}
			}
		case corpus.StatusPartial:
			// A partial says which parts it means. Without them the reader is
			// told that some of an eight part exercise is right and left to work
			// out which, which is more work than the exercise.
			if len(m.Parts) == 0 {
				out = append(out, Finding{File: d.Path, Line: 1,
					Msg: "is partial and enumerates no parts"})
				break
			}
			good, bad := 0, 0
			for _, p := range m.Parts {
				if p.Status == corpus.StatusVerified {
					good++
					continue
				}
				bad++
				if p.Reason == "" {
					out = append(out, Finding{File: d.Path, Line: 1,
						Msg: fmt.Sprintf("part %s is %s and gives no reason", p.ID, p.Status)})
				}
			}
			if good == 0 || bad == 0 {
				out = append(out, Finding{File: d.Path, Line: 1,
					Msg: fmt.Sprintf("is partial with %d parts passing and %d not, "+
						"which is not partial", good, bad)})
			}
		case corpus.StatusUnattempted:
			if strings.TrimSpace(d.Body) != "" {
				out = append(out, Finding{File: d.Path, Line: 1,
					Msg: "is unattempted and has a body"})
			}
		}
		// Whatever the status, the body is what the file is for. blocked and
		// open carry the reasoning that reached that verdict, which is the only
		// evidence it was arrived at rather than assumed.
		if m.Status != corpus.StatusUnattempted && strings.TrimSpace(d.Body) == "" {
			out = append(out, Finding{File: d.Path, Line: 1,
				Msg: fmt.Sprintf("is %s and the body is empty", m.Status)})
		}
	}
	return out, nil
}

// X03. Every tag a solution says it uses exists.
//
// The uses list is how the corpus learns which of its results are load-bearing,
// and it is worth having only if it resolves. A tag that is in no line of tags
// is a result the model named and the book does not print, which is the exact
// failure the tagging contract exists to catch.
func x03(c *Corpus) ([]Finding, error) {
	known := map[string]bool{}
	for _, e := range append(append([]tags.Entry(nil), c.Tags.Tags...), c.Tags.New...) {
		known[string(e.Tag)] = true
	}
	var out []Finding
	for _, d := range c.Docs {
		if d.Kind != KindSolution || d.Solution == nil {
			continue
		}
		seen := map[string]bool{}
		for _, use := range d.Solution.Uses {
			switch {
			case !known[use]:
				out = append(out, Finding{File: d.Path, Line: 1,
					Msg: fmt.Sprintf("uses tag %q, which is in no line of tags", use)})
			case seen[use]:
				out = append(out, Finding{File: d.Path, Line: 1,
					Msg: fmt.Sprintf("names tag %s twice", use)})
			}
			seen[use] = true
		}
	}
	return out, nil
}

// leaks are the shapes a model's own voice takes when it gets into the answer.
//
// The first group is the provider talking about itself, which is a solution
// that never got past the boilerplate. The second is the assistant register: a
// solution that offers to help further is a chat reply that was filed as
// mathematics.
var leaks = []string{
	"as an ai", "language model", "i cannot", "i'm sorry", "i am sorry",
	"openai", "anthropic", "chatgpt", "claude", "gpt-4", "gpt-5",
	"let me know if", "feel free to ask", "i hope this helps",
	"here is the solution", "certainly!", "sure!",
	"as requested", "i'll help you",
}

// X04. No provider leakage and no meta-commentary.
//
// The corpus is meant to read as a book. A line that says which model wrote it
// belongs in the front matter, where it is recorded, and not in the prose.
func x04(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		if d.Kind != KindSolution {
			continue
		}
		for i, line := range strings.Split(d.Body, "\n") {
			low := strings.ToLower(line)
			for _, leak := range leaks {
				if strings.Contains(low, leak) {
					out = append(out, Finding{File: d.Path, Line: d.BodyLine(i + 1),
						Msg: fmt.Sprintf("%q is the model talking rather than the mathematics: %s",
							leak, ellipsis(line, 50))})
					break
				}
			}
		}
	}
	return out, nil
}
