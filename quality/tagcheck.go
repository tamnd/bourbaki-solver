package quality

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/tags"
)

// The tag rules are not reimplemented here. tags.Verify is what assign, merge
// and migrate are all held to, and an audit with its own second opinion about
// what a tag is would eventually disagree with the code that hands them out.
// What this file does is run that verifier once and hand each of its failures
// to the rule it belongs to, so that a CI log says T04 and not "tags".
//
// T01 to T07 are the invariants of spec 01 §5.3. T08 of spec 08 §2.2 is T05
// read off a diff rather than off the file, which is the only way it can be
// read, so it is reported under T05 and does not appear twice. T09 and T10 are
// spec 08's own.

func init() {
	register(
		Check{ID: "T01", Group: Tags, Hard: true,
			Title: "a tag is well formed and appears once in tags", Run: tagRule(tags.T01)},
		Check{ID: "T02", Group: Tags, Hard: true,
			Title: "a label appears at most once across tags and inactive", Run: tagRule(tags.T02)},
		Check{ID: "T03", Group: Tags, Hard: true,
			Title: "every statement in the corpus has exactly one tag", Run: tagRule(tags.T03)},
		Check{ID: "T04", Group: Tags, Hard: true,
			Title: "every tag in tags names a statement that is in the corpus", Run: tagRule(tags.T04)},
		Check{ID: "T05", Group: Tags, Hard: true,
			Title: "tags is only ever appended to, and T08 is this read off a diff",
			Run:   t05, Need: needHistory},
		Check{ID: "T06", Group: Tags, Hard: true,
			Title: "no tag is in both tags and inactive", Run: tagRule(tags.T06)},
		Check{ID: "T07", Group: Tags, Hard: true,
			Title: "a translation reuses the English tag and never gets its own", Run: tagRule(tags.T07)},
		Check{ID: "T09", Group: Tags, Hard: true,
			Title: "every tag= in the Markdown is four of [0-9A-Z]", Run: tagRule(tags.T09)},
		Check{ID: "T10", Group: Tags, Hard: false,
			Title: "the tags of a file climb, as they did on the run that assigned them", Run: t10},
	)
}

// verified is tags.Verify over the whole corpus, run once and kept. Nine rules
// read it and the walk behind it is the second most expensive thing the audit
// does.
func (c *Corpus) verified() []tags.Failure {
	if c.tagFailures == nil {
		c.tagFailures = append([]tags.Failure{}, tags.Verify(c.Tags, c.Items, c.Books.Printings())...)
	}
	return c.tagFailures
}

// tagRule is one invariant, taken out of the verifier's answer.
func tagRule(rule string) func(*Corpus) ([]Finding, error) {
	return func(c *Corpus) ([]Finding, error) {
		var out []Finding
		for _, f := range c.verified() {
			if f.Rule == rule {
				out = append(out, findingAt(f.Msg))
			}
		}
		return out, nil
	}
}

// needHistory says why T05 did not run. A shallow checkout, which is what a CI
// job gets unless it asks otherwise, has no merge base to diff against, and
// that is a coverage gap and not a pass.
func needHistory(c *Corpus) string {
	if c.TagsDiffErr != nil {
		return c.TagsDiffErr.Error()
	}
	return ""
}

// T05. tags is only ever appended to.
//
// This is a fact about the history of the file and not about the file, so it is
// read off a diff against the base commit. A removal is allowed only when an
// alias explains it and the same tag comes back under the new label, which is
// what migrate does and is the one edit to tags that is not an append.
func t05(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, f := range tags.AppendOnly(c.TagsDiff, c.Tags.Aliases, c.Tags.Inactive) {
		out = append(out, Finding{File: "tags/" + tags.TagsFile, Msg: f.Msg})
	}
	return out, nil
}

// T10. The tags of a file climb.
//
// Soft, and it has to be. Tags are handed out in reading order, so a file's
// tags climb on the run that assigns them, and a file where they do not is
// worth a look: a heading hand-edited with somebody else's tag, or two
// statements that swapped places without their tags. But add a proposition to
// the middle of § 3 and it takes the next free tag, which is larger than
// everything after it in the file, and the corpus is then correct and this is
// broken. A rule a correct edit breaks is not a rule, so it is counted and
// reported and nothing is failed for it.
func t10(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, lang := range c.Langs {
		for _, f := range tags.Order(c.Items[lang], c.Tags.Runs) {
			out = append(out, findingAt(f.Msg))
		}
	}
	return out, nil
}

// atRE is the place a tag failure opens with, "content/en/alg/VIII/03_s3.md:214".
var atRE = regexp.MustCompile(`^(\S+\.md)(?::(\d+))?\s+`)

// findingAt pulls the place out of the front of a verifier message, so the
// finding carries a file and a line like every other one here and an editor can
// jump to it.
func findingAt(msg string) Finding {
	m := atRE.FindStringSubmatch(msg)
	if m == nil {
		return Finding{Msg: msg}
	}
	line, _ := strconv.Atoi(m[2])
	return Finding{File: m[1], Line: line, Msg: strings.TrimSpace(msg[len(m[0]):])}
}
