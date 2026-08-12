package textguard

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// A judge decides and a program has to read the decision, so the prompts ask
// for it on a line of its own in a fixed vocabulary. This is the reader for
// those lines.
//
// The alternative is to read the prose, and the prose is where a judge says "so
// the argument does establish the claim, although" and means the opposite of
// what the first clause says. Everything downstream of a judge is a fact about
// the corpus: whether a solution is published as believed, what the scorecard
// counts, whether the correction loop runs again. None of that can rest on a
// keyword search over an essay.

// line matches one decision line.
//
// The models are not equally obedient about "on a line of its own". A weaker one
// bolds it, bullets it, quotes it, or ends it with a full stop, and none of that
// changes what it decided. Throwing away a solve over a pair of asterisks is
// expensive: the judging is the cheap half and the generating is not.
//
// The decision still has to be the whole line. A gate that accepted a verdict
// mentioned in passing would not be a gate, and a judge that writes "the answer
// is not VERDICT: PASS material" has said something this must not read as a
// pass.
func line(key, values string, suffix ...string) *regexp.Regexp {
	tail := ""
	if len(suffix) > 0 {
		tail = suffix[0]
	}
	return regexp.MustCompile(`(?mi)^` + mark + key + mark + `:` + mark +
		`(` + values + `)` + tail + mark + `[.]?` + mark + `$`)
}

// mark is the decoration a model puts around a decision, and it holds no
// newline. It has to appear on both sides of the colon, because a model that
// bolds the key alone writes **VERDICT**: PASS as readily as **VERDICT: PASS**.
//
// The newline is the part that matters. With \s in here the class ran off the
// end of the line, and a part decision then took the whole of the next line as
// its reason, so a four part exercise read as two parts with each other's lines
// hanging off them.
const mark = "[ \t*_`>#-]*"

var (
	verdictLine       = line("VERDICT", `PASS|FAIL`)
	scoreLine         = line("SCORE", `[0-7]`, `\s*/\s*7`)
	truthLine         = line("TRUTH", `TRUE|FALSE`)
	completeLine      = line("COMPLETE", `YES|NO`)
	selfContainedLine = line("SELF_CONTAINED", `YES|NO`)
	humanReadableLine = line("HUMAN_READABLE", `YES|NO`)
	verifiableLine    = line("VERIFIABLE", `YES|NO`)
	selectedLine      = line("SELECTED", `[1-5]`)
	natureLine        = line("NATURE", `PROOF|EXPLORATION`)
	reachLine         = line("REACH", `IN_CORPUS|OUT_OF_CORPUS`)
	usesLine          = regexp.MustCompile(`(?mi)^` + mark + `USES` + mark + `:[ \t]*(.*)$`)
	// The two lines the audit judge writes its work on. They take anything after
	// the colon and only ask that there is something, because what a step is and
	// what breaking it would take are the judge's words and not a vocabulary this
	// package has any business fixing.
	checkedLine = regexp.MustCompile(`(?mi)^` + mark + `CHECKED` + mark + `:[ \t]*\S.*$`)
	triedLine   = regexp.MustCompile(`(?mi)^` + mark + `TRIED` + mark + `:[ \t]*\S.*$`)
	// A part decision names the part first, because that is how the exercise
	// names it and because a judge that has to write the letter out is a judge
	// that has to look at which part it is deciding.
	partLine = regexp.MustCompile(`(?mi)^` + mark + `PART[ \t]+([a-z]|[0-9]{1,2})[.)]?` +
		mark + `:` + mark + `(PASS|FAIL)` + mark + `[,.]?[ \t]*(.*)$`)
	tagWord = regexp.MustCompile(`[0-9A-Z]{4}`)
)

// Decision is what a judge decided, as it wrote it down.
//
// Every field has a Has beside it or is reported as missing, because a judge
// that did not answer and a judge that answered no are different states and only
// one of them is a reason to ask again. A missing field read as a no would fail
// good solutions on a formatting slip; read as a yes it would pass anything that
// stayed quiet.
type Decision struct {
	Verdict string // PASS, FAIL, or UNKNOWN when no line said
	Score   int    // 0 to 7, and -1 when no line said

	Truth         bool
	Complete      bool
	SelfContained bool
	HumanReadable bool
	Verifiable    bool

	HasTruth   bool
	HasQuality bool // all four of the publication fields were answered

	// Checked and Tried are how many steps the audit judge says it went through
	// and how many things it says it substituted. They are counts and not
	// content: nothing here reads what was written on those lines, and what they
	// are for is telling a judgement from an assertion. Both audits of exercise 2
	// of § 1 came back as four lines and fifty one bytes, PART a PASS, PART b
	// PASS, VERDICT PASS, TRUTH TRUE, on a solution the truth judge had just
	// written four thousand characters against, while exercise 1 an hour earlier
	// went through eight steps and found a false citation in the sixth. A judge
	// that shows no work cannot be told from a judge that did none.
	Checked, Tried int

	// Parts is the per-part verdict of a multi-part exercise, keyed by the
	// letter the book prints. It is empty for an exercise that has no parts,
	// which is not the same as an exercise whose parts went unjudged: the
	// prompts ask for a part line only where the exercise has parts.
	Parts []PartDecision
}

// PartDecision is one lettered part of an exercise, judged alone.
type PartDecision struct {
	ID     string
	Pass   bool
	Reason string
}

// Read parses the decision lines out of a judge's answer.
//
// The last of each line wins. A judge that writes its working out and then its
// decision has written the same key twice, and what it concluded is what it said
// last, not what it first considered.
func Read(review string) Decision {
	d := Decision{Verdict: "UNKNOWN", Score: -1}
	if m := last(verdictLine, review); m != nil {
		d.Verdict = strings.ToUpper(m[1])
	}
	if m := last(scoreLine, review); m != nil {
		d.Score = int(m[1][0] - '0')
	}
	truth, hasTruth := boolean(truthLine, review, "TRUE")
	complete, hasComplete := boolean(completeLine, review, "YES")
	selfContained, hasSelfContained := boolean(selfContainedLine, review, "YES")
	humanReadable, hasHumanReadable := boolean(humanReadableLine, review, "YES")
	verifiable, hasVerifiable := boolean(verifiableLine, review, "YES")
	d.Truth, d.HasTruth = truth, hasTruth
	d.Complete, d.SelfContained = complete, selfContained
	d.HumanReadable, d.Verifiable = humanReadable, verifiable
	d.HasQuality = hasComplete && hasSelfContained && hasHumanReadable && hasVerifiable
	d.Checked = len(checkedLine.FindAllString(review, -1))
	d.Tried = len(triedLine.FindAllString(review, -1))
	d.Parts = parts(review)
	return d
}

// parts reads the per-part decisions, last one to a part winning.
func parts(review string) []PartDecision {
	byID := map[string]PartDecision{}
	for _, m := range partLine.FindAllStringSubmatch(review, -1) {
		id := strings.ToLower(m[1])
		byID[id] = PartDecision{ID: id, Pass: strings.EqualFold(m[2], "PASS"),
			Reason: strings.TrimSpace(strings.Trim(strings.TrimSpace(m[3]), "*_`-"))}
	}
	out := make([]PartDecision, 0, len(byID))
	for _, p := range byID {
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ID < out[j].ID })
	return out
}

// Passed is the whole of what a truth judge has to have said for a solution to
// be believed, and the reason when it has not.
//
// The floor on the score is 6 of 7 and it is the same floor taocp-solver runs.
// It is not a measurement of anything; it is a way of saying that a judge which
// went to the trouble of marking a solution down has been listened to. A judge
// that says TRUE and then scores it 3 has found something, and passing it
// because the truth line said TRUE would be reading only the field that agrees.
func (d Decision) Passed() (bool, string) {
	switch {
	case d.Verdict == "UNKNOWN":
		return false, "the review carries no verdict line"
	case !d.HasTruth:
		return false, "the review carries no truth line"
	case !d.HasQuality:
		return false, "the review does not answer all four publication fields"
	case d.Score < 0:
		return false, "the review carries no score line"
	case d.Verdict == "FAIL":
		return false, "the verdict is fail"
	case !d.Truth:
		return false, "the review says the solution is not true"
	case !d.Complete:
		return false, "the review says the solution is not complete"
	case !d.SelfContained:
		return false, "the review says the solution is not self contained"
	case !d.HumanReadable:
		return false, "the review says the solution is not readable"
	case !d.Verifiable:
		return false, "the review says the solution cannot be checked"
	case d.Score < 6:
		return false, fmt.Sprintf("the review scores it %d of 7", d.Score)
	}
	for _, p := range d.Parts {
		if !p.Pass {
			return false, fmt.Sprintf("part %s did not pass", p.ID)
		}
	}
	return true, ""
}

// Audited is the same question of the audit judge, which is asked for less: it
// never sees the reference and it is asked only whether it could falsify the
// solution. Asking it for a publication score as well would be asking it to do
// the truth judge's job with less to go on.
func (d Decision) Audited() (bool, string) {
	switch {
	case d.Verdict == "UNKNOWN":
		return false, "the audit carries no verdict line"
	case !d.HasTruth:
		return false, "the audit carries no truth line"
	case d.Verdict == "FAIL":
		return false, "the audit verdict is fail"
	case !d.Truth:
		return false, "the audit says the solution is not true"
	}
	for _, p := range d.Parts {
		if !p.Pass {
			return false, fmt.Sprintf("part %s did not survive the audit", p.ID)
		}
	}
	return true, ""
}

// Selected is the candidate the selector chose, and 0 when it did not say.
func Selected(review string) int {
	if m := last(selectedLine, review); m != nil {
		return int(m[1][0] - '0')
	}
	return 0
}

// Nature and Reach are the two things the reference call is asked about the
// exercise rather than about an answer, and they are what the statuses open and
// blocked are made of.
//
// They are asked of the reference and not of a judge because the reference is
// the one call that has read the exercise and has not read an answer. A model
// that has just written three pages of proof is the worst possible witness to
// whether the exercise could be proved.
func Nature(review string) (string, bool) {
	if m := last(natureLine, review); m != nil {
		return strings.ToUpper(m[1]), true
	}
	return "", false
}

// Reach says whether the exercise can be discharged from what the corpus holds.
func Reach(review string) (string, bool) {
	if m := last(reachLine, review); m != nil {
		return strings.ToUpper(strings.ReplaceAll(m[1], " ", "_")), true
	}
	return "", false
}

// Uses reads the tags a solution says it leaned on, and gives back the solution
// without that line.
//
// The line is taken out of the body because it is bookkeeping and the body is
// mathematics. It goes to the front matter, where the audit can resolve it, and
// a reader who wants to know what a proof rests on reads the sentences that say
// so rather than a list of four character codes under the last line.
//
// Anything that is not the shape of a tag is dropped here rather than passed on
// to be refused later. A model writing "USES: 0001, Proposition 1" has answered
// the question in two vocabularies, and the four characters are the half that
// resolves.
func Uses(solution string) ([]string, string) {
	m := usesLine.FindStringSubmatchIndex(solution)
	if m == nil {
		return nil, solution
	}
	var out []string
	seen := map[string]bool{}
	for _, tag := range tagWord.FindAllString(solution[m[2]:m[3]], -1) {
		if !seen[tag] {
			seen[tag] = true
			out = append(out, tag)
		}
	}
	body := solution[:m[0]] + solution[m[1]:]
	return out, strings.TrimRight(body, " \t\n") + "\n"
}

func last(re *regexp.Regexp, s string) []string {
	m := re.FindAllStringSubmatch(s, -1)
	if len(m) == 0 {
		return nil
	}
	return m[len(m)-1]
}

func boolean(re *regexp.Regexp, s, affirmative string) (bool, bool) {
	m := last(re, s)
	if m == nil {
		return false, false
	}
	return strings.EqualFold(m[1], affirmative), true
}
