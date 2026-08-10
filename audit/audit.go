// Package audit reads an accepted page and points at the places worth one
// question.
//
// Everything else in the pipeline decides whether a page is a transcription.
// Seven rules do it, they are cheap, and they catch a model that gave up, one
// that narrated, one that left a formula open. What none of them catch is a page
// that is a transcription and is wrong in one character, because there is
// nothing about such a page to notice: it is the right length, it balances, the
// running head is right, and it reads perfectly.
//
// Page 51 of Algebra I is the measured case. The scan prints "i in [1, n]", an
// interval of integers. The transcription has "i in {1, n}", a set with two
// elements in it. Every rule passed and nothing downstream would ever have
// looked at it again.
//
// So the detectors here look for shapes that a scan of this typeface is known to
// produce wrongly, and each one they find becomes a question put back into the
// conversation the page came out of, where the image still is. They are
// suspicions and not defects. Most of them come back confirmed, and a confirmed
// suspicion costs one follow up call and settles a spot in the corpus for good.
//
// Every detector in here has to have a page behind it. A shape somebody thought
// might be a problem, added in advance of ever seeing one, spends a model call
// per occurrence across 1194 pages and buys nothing.
package audit

import (
	"regexp"
	"strings"

	"github.com/tamnd/bourbaki-solver/repair"
)

// Detector is one shape worth asking about.
type Detector struct {
	// Name goes in the report, so a week of these can say which detector is
	// worth its calls and which one only ever comes back confirmed.
	Name string
	// Find returns the spans of one line that this detector suspects.
	Find func(line string) []string
	// Why is shown to the model, and it has to describe what the checker
	// suspects rather than assert it. A model told a span is wrong will make it
	// different.
	Why string
}

// interval is braces where the page very likely prints an interval.
//
// Bourbaki writes the integers from 1 to n as [1, n] and does it constantly.
// The brackets are printed heavy and square in this edition and the scan is a
// 1974 offset print, so a bracket closing tight against a digit fills in and
// comes out as a brace. The two element set {1, n} does occur, which is why this
// is a question and not a rule.
//
// The shape is narrow on purpose, because every match spends a call: a digit,
// one comma, and a name. A set of three, a set of letters and a range written
// out with an ellipsis are all ordinary mathematics and are left alone. What is
// left is the form Bourbaki uses for an index range and almost never for a pair.
var interval = regexp.MustCompile(`\\in\s*\\\{\s*([0-9]{1,2}),\s*([A-Za-z][A-Za-z0-9+\-]{0,3})\s*\\\}`)

// Detectors is every shape currently worth a call, in the order they run.
var Detectors = []Detector{
	{
		Name: "interval",
		Why:  "the braces may be square brackets on the page, printed [1, n] and meaning the integers from 1 to n rather than a set with two elements in it",
		Find: func(line string) []string {
			var out []string
			for _, match := range interval.FindAllString(line, -1) {
				// The span handed on is the braces and what is between them.
				// Leaving \in out of it lets the model correct the brackets
				// without the audit having to allow the word before them to
				// change.
				at := strings.Index(match, `\{`)
				out = append(out, match[at:])
			}
			return out
		},
	},
}

// Scan is every suspect on a page, in reading order.
//
// A span that appears on its line more than once is dropped rather than
// reported. The audit that follows proves a correction by splitting the line at
// the span, and a span that is on the line twice cannot be split at.
func Scan(text string) []repair.Suspect {
	var out []repair.Suspect
	for i, line := range strings.Split(text, "\n") {
		for _, detector := range Detectors {
			for _, span := range detector.Find(line) {
				if strings.Count(line, span) != 1 {
					continue
				}
				out = append(out, repair.Suspect{Line: i + 1, Text: line, Span: span, Why: detector.Why})
			}
		}
	}
	return out
}
