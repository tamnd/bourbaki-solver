package quality

import (
	"fmt"
	"slices"
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
		Check{ID: "X05", Group: Solutions, Hard: false,
			Title: "a solution writes its mathematics in TeX", Run: x05, Need: needSolutions},
		Check{ID: "X06", Group: Solutions, Hard: false,
			Title: "no solution was written on the free gateway", Run: x06, Need: needSolutions},
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

// mathGlyph is a character that belongs inside a math span and nowhere else.
//
// The ranges are the ones a model reaches for when it writes mathematics as
// text instead of as TeX: the operators, the arrows, the two supplemental
// operator blocks, Greek, and the raised and lowered digits. What is
// deliberately out is everything the prose of this corpus does use, so §, é,
// the dashes and the quotation marks are not here, and neither is × on its own,
// which the extraction leaves in the running text of a page range.
func mathGlyph(r rune) bool {
	switch {
	case r >= 0x0370 && r <= 0x03FF: // Greek, which is only ever a variable here
		return true
	case r >= 0x2070 && r <= 0x209F: // raised and lowered digits and letters
		return true
	case r >= 0x2190 && r <= 0x21FF: // arrows
		return true
	case r >= 0x2200 && r <= 0x22FF: // mathematical operators
		return true
	case r >= 0x27C0 && r <= 0x27EF, r >= 0x2A00 && r <= 0x2AFF: // supplements
		return true
	case r == 0x2102, r == 0x2115, r == 0x211A, r == 0x211D, r == 0x2124: // C N Q R Z
		return true
	}
	return false
}

// inMath marks the runes of a body that a math span covers.
func inMath(body string) []bool {
	covered := make([]bool, len([]rune(body)))
	spans, _ := Math(body)
	for _, s := range spans {
		for i := s.Start; i < s.End && i < len(covered); i++ {
			if i >= 0 {
				covered[i] = true
			}
		}
	}
	return covered
}

// X05. A solution writes its mathematics in TeX.
//
// Every other document in this corpus was typed from a printed page, so its
// mathematics is TeX because that is the only way the page could be
// transcribed. A solution is written by a model, and a model asked for
// mathematics in a chat window will answer in the symbols a chat window can
// draw: Γ, ⊂, ⋂, ∀. The answer reads correctly and is unusable, because
// nothing downstream can see it. M01 finds no unclosed span in it, M04 and P04
// have nothing to parse, the reference graph finds no notation to follow, and a
// translation of it has no math spans to hold in place. One solution of the
// twenty five on this corpus came back that way, with 181 such characters and
// not one dollar sign, and every rule passed it.
//
// Soft, like the rest of this group. The solution is wrong in its writing and
// not in its mathematics, and the answer is to ask for it again rather than to
// turn the repository red.
func x05(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		if d.Kind != KindSolution {
			continue
		}
		covered := inMath(d.Body)
		line, seen := 1, map[rune]bool{}
		count, first := 0, 0
		for i, r := range []rune(d.Body) {
			if r == '\n' {
				line++
				continue
			}
			if covered[i] || !mathGlyph(r) {
				continue
			}
			if count == 0 {
				first = line
			}
			count++
			seen[r] = true
		}
		if count == 0 {
			continue
		}
		var glyphs []rune
		for r := range seen {
			glyphs = append(glyphs, r)
		}
		slices.Sort(glyphs)
		out = append(out, Finding{File: d.Path, Line: d.BodyLine(first),
			Msg: fmt.Sprintf("%d characters of mathematics stand outside any math span, as %s, "+
				"so the solution was written in symbols rather than in TeX", count, string(glyphs))})
	}
	return out, nil
}

// X06. No solution was written on the free gateway.
//
// L15 asks this of a translation and nothing asked it of a solution, which is
// the larger hole of the two: 39 of the 44 solutions in the corpus were written
// on the gateway, and the audit said nothing about any of them.
//
// The reason it matters is stronger here than for a translation. A translation
// has an English source sitting beside it that a reader can check a sentence
// against, and a wrong rendering is usually visible as bad Vietnamese. A
// solution has nothing beside it. It is the only text in the corpus that is not
// a reading of a printed page, so there is no original to hold it to, and a
// wrong proof written fluently reads exactly like a right one. The judges are
// the only thing standing between a wrong answer and a reader who trusts it,
// and solve eval, which is what says what a judge verdict is worth, has not
// been run against the benchmark yet.
//
// So a gateway solution is a claim with no page behind it, checked by judges of
// unmeasured accuracy, from the route the pipeline falls back to when the good
// ones are out of allowance. Three of them are already flagged by X05 for
// writing their mathematics in Unicode instead of TeX, and all three failed
// both judges and were hand read as unintelligible. That is what the class
// looks like.
//
// Soft, for L15's reason. The answer may well be right and the corpus should
// not go red because the good routes had no allowance on the day. What it
// should do is name them, so a later pass with allowance asks for these first
// rather than finding them by reading.
func x06(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		if d.Kind != KindSolution || d.Solution == nil {
			continue
		}
		if !FreeGatewayModel(d.Solution.Model) {
			continue
		}
		out = append(out, Finding{File: d.Path, Line: 1, Msg: fmt.Sprintf(
			"was written by %s, which is a free gateway model, and a solution has no printed page behind it, so it is worth asking for again",
			d.Solution.Model)})
	}
	return out, nil
}
