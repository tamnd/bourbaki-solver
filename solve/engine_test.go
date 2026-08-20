package solve

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The pipeline is run end to end here against answers written by hand. The
// judging is the part of this that has to be right, and a harness that can only
// be exercised by spending a fleet is a harness nobody exercises.

// asker is a model that answers from a script.
type asker struct {
	// by is what to answer, keyed by the stage in the call's id. A slice
	// answers a stage that is asked more than once, in order, and the last entry
	// is repeated once the slice runs out.
	by map[string][]string
	// asked is every question put, in order, with its id.
	asked []struct{ id, question string }
	err   map[string]error
	// fails is how many times a stage does not come back at all before it
	// answers, which is a host dropping the connection and not a model saying
	// something unreadable.
	fails map[string]int
}

func (a *asker) Ask(_ context.Context, id, question string) (Answer, error) {
	a.asked = append(a.asked, struct{ id, question string }{id, question})
	stage := stageOf(id)
	if err, ok := a.err[stage]; ok {
		return Answer{}, err
	}
	if n := a.fails[stage]; n > 0 {
		a.fails[stage] = n - 1
		return Answer{}, errNo
	}
	answers, ok := a.by[stage]
	if !ok {
		return Answer{}, fmt.Errorf("the script says nothing about %q", stage)
	}
	n := 0
	for _, prior := range a.asked[:len(a.asked)-1] {
		if stageOf(prior.id) == stage {
			n++
		}
	}
	if n >= len(answers) {
		n = len(answers) - 1
	}
	return Answer{Text: answers[n], Model: "gpt-5-6", Conversation: "https://example/" + id,
		Elapsed: time.Second}, nil
}

// stageOf takes the label off the front of a call id and the attempt off the
// back, leaving the stage.
func stageOf(id string) string {
	i := strings.Index(id, "-ex-")
	if i >= 0 {
		id = id[i+len("-ex-"):]
		if j := strings.Index(id, "-"); j >= 0 {
			id = id[j+1:]
		}
	}
	if j := strings.LastIndex(id, "-"); j >= 0 {
		id = id[:j]
	}
	return id
}

func (a *asker) question(t *testing.T, stage string) string {
	t.Helper()
	for _, q := range a.asked {
		if stageOf(q.id) == stage {
			return q.question
		}
	}
	t.Fatalf("%s was never asked. What was: %s", stage, strings.Join(a.stages(), ", "))
	return ""
}

func (a *asker) stages() []string {
	var out []string
	for _, q := range a.asked {
		out = append(out, stageOf(q.id))
	}
	return out
}

func exercise(text string) *Context {
	return &Context{Label: "alg-viii-s1-ex-1", Tag: "000X", Lang: "en",
		Options: Options{Depth: 2, MaxChars: 40000},
		Pieces: []Piece{
			{Kind: TheExercise, Label: "alg-viii-s1-ex-1", Tag: "000X", Text: text},
			// The § carries its statements with their tags, the way a § does, so
			// that a candidate here citing 0001 is citing something it was shown.
			{Kind: TheSection, Label: "alg-viii-s1", Text: "## § 1. ARTINIAN MODULES\n\n" +
				"#### Definition 1 {#alg-viii-s1-def-1 .statement tag=0001}\n\n" +
				"A module is Artinian when every descending chain stops.\n\n" +
				"#### Proposition 1 {#alg-viii-s1-prop-1 .statement tag=0002}\n\n" +
				"A module is Noetherian if and only if every submodule is finitely generated.\n"},
		}}
}

const (
	reference = "## Independent Derivation\n\nEvery submodule is finitely generated.\n\n" +
		"## Obligations\n\n1. [*] every submodule of $M$ is finitely generated\n\n" +
		"## Failure Modes\n\nAssuming $M$ is free.\n\n" +
		"## Falsification Checks\n\nTake $M = \\mathbf{Z}$.\n\n" +
		"## Reference Conclusion\n\nThe module is Noetherian.\n\n" +
		"NATURE: PROOF\nREACH: IN_CORPUS\n"
	// The reference above sets one obligation, so a truth judge's answer owes it
	// one line. Both of these carry it, because an answer that does not is
	// refused before its verdict is read.
	pass = "OBLIGATION 1: DISCHARGED, the second paragraph proves it\n\n" +
		"Every obligation is discharged.\n\nVERDICT: PASS\nTRUTH: TRUE\n" +
		"COMPLETE: YES\nSELF_CONTAINED: YES\nHUMAN_READABLE: YES\nVERIFIABLE: YES\nSCORE: 7/7\n"
	fail = "OBLIGATION 1: NOT DISCHARGED, the third step is asserted\n\n" +
		"The third step is asserted.\n\nVERDICT: FAIL\nTRUTH: FALSE\n" +
		"COMPLETE: NO\nSELF_CONTAINED: YES\nHUMAN_READABLE: YES\nVERIFIABLE: NO\nSCORE: 3/7\n"
	work = "CHECKED: the chain stops, it would fail if the chain were infinite, ruled out\n" +
		"TRIED: $M = \\mathbf{Z}$, the argument still runs\n\n"
	auditPass = work + "I could not break it.\n\nVERDICT: PASS\nTRUTH: TRUE\n"
	auditFail = work + "The case $n = 0$ is not covered.\n\nVERDICT: FAIL\nTRUTH: FALSE\n"
	answer    = "Let $N$ be a submodule of $M$. Then $N$ is finitely generated.\n\nUSES: 0001, 0002\n"
)

func engine(a *asker) Engine {
	return Engine{Ask: a, Now: func() time.Time { return time.Unix(1700000000, 0) },
		Pause: func(context.Context, time.Duration) error { return nil }}
}

func TestAnExerciseSolvedAndBelieved(t *testing.T) {
	a := &asker{by: map[string][]string{
		"reference":                {reference},
		"candidate-direct":         {"the direct answer\n\nUSES: 0001\n"},
		"candidate-contrapositive": {answer},
		"candidate-elementary":     {"the elementary answer\n"},
		"select":                   {"Candidate 2 covers the obligation.\n\nSELECTED: 2\n"},
		"truth":                    {pass},
		"audit":                    {auditPass},
	}}
	got, err := engine(a).Solve(context.Background(), exercise("Prove that $M$ is Noetherian."))
	if err != nil {
		t.Fatal(err)
	}
	m := got.Solution.Meta
	if m.Status != corpus.StatusVerified {
		t.Errorf("status %q", m.Status)
	}
	if m.TruthJudge != "pass" || m.AuditJudge != "pass" {
		t.Errorf("judges %q and %q", m.TruthJudge, m.AuditJudge)
	}
	if m.Label != "alg-viii-s1-ex-1" || m.Tag != "000X" || m.Lang != "en" {
		t.Errorf("front matter %+v", m)
	}
	if m.Corrections != 0 || m.Candidates != 3 {
		t.Errorf("corrections %d, candidates %d", m.Corrections, m.Candidates)
	}
	if m.Model != "gpt-5-6" || m.Generated == "" || m.PromptSHA256 == "" {
		t.Errorf("provenance %+v", m)
	}
	// The chosen candidate is the one the selector named, its tag line has been
	// lifted into the front matter, and the body is the mathematics alone.
	if got.Selected != 2 {
		t.Errorf("selected %d", got.Selected)
	}
	if len(m.Uses) != 2 || m.Uses[0] != "0001" {
		t.Errorf("uses %v", m.Uses)
	}
	if strings.Contains(got.Solution.Body, "USES") || !strings.Contains(got.Solution.Body, "submodule") {
		t.Errorf("body %q", got.Solution.Body)
	}
	// Six calls: the reference, three candidates, the selector, and two judges.
	if want := []string{"reference", "candidate-direct", "candidate-contrapositive",
		"candidate-elementary", "select", "truth", "audit"}; !equal(a.stages(), want) {
		t.Errorf("the pipeline ran %v", a.stages())
	}
	if err := Validate(got.Solution); err != nil {
		t.Errorf("the pipeline wrote a solution the store would refuse: %v", err)
	}
}

// The two ways an exercise is not the model's to fail. Both are read off the
// reference, and both stop the pipeline there rather than spending three
// candidates to find out what one call already said.
func TestTheExercisesThatAreNotTheModelsToFail(t *testing.T) {
	cases := []struct {
		name, swap, with, want string
	}{
		{"a volume the corpus does not hold", "REACH: IN_CORPUS",
			"REACH: OUT_OF_CORPUS", corpus.StatusBlocked},
		{"an exercise that asks the reader to investigate", "NATURE: PROOF",
			"NATURE: EXPLORATION", corpus.StatusOpen},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			a := &asker{by: map[string][]string{
				"reference": {strings.ReplaceAll(reference, c.swap, c.with)}}}
			got, err := engine(a).Solve(context.Background(), exercise("Investigate."))
			if err != nil {
				t.Fatal(err)
			}
			if got.Solution.Meta.Status != c.want {
				t.Errorf("status %q, want %q", got.Solution.Meta.Status, c.want)
			}
			if len(a.asked) != 1 {
				t.Errorf("it spent %d calls on it", len(a.asked))
			}
			// The reasoning that reached the verdict is under it, or the file
			// says an exercise cannot be done and offers nothing to disagree
			// with.
			if !strings.Contains(got.Solution.Body, "Reference Conclusion") {
				t.Errorf("body %q", got.Solution.Body)
			}
			if err := Validate(got.Solution); err != nil {
				t.Errorf("the store would refuse it: %v", err)
			}
		})
	}
}

func TestTheCorrectionLoopIsBounded(t *testing.T) {
	a := &asker{by: map[string][]string{
		"reference": {reference}, "candidate-direct": {answer},
		"candidate-contrapositive": {answer}, "candidate-elementary": {answer},
		"select": {"SELECTED: 1"},
		"truth":  {fail}, "truth-1": {fail}, "truth-2": {fail},
		"audit": {auditFail}, "audit-1": {auditFail}, "audit-2": {auditFail},
		"correct-1": {answer}, "correct-2": {answer},
	}}
	got, err := engine(a).Solve(context.Background(), exercise("Prove it."))
	if err != nil {
		t.Fatal(err)
	}
	if got.Solution.Meta.Status != corpus.StatusUnverified {
		t.Errorf("status %q", got.Solution.Meta.Status)
	}
	if got.Solution.Meta.Corrections != 2 {
		t.Errorf("corrections %d, and the budget is 2", got.Solution.Meta.Corrections)
	}
	if n := strings.Count(strings.Join(a.stages(), " "), "correct"); n != 2 {
		t.Errorf("it corrected %d times", n)
	}
	// The correction call is given both judgements whole, and only the ones that
	// failed. A summary of a complaint is a second reading of the solution by
	// something that has not read the solution.
	q := a.question(t, "correct-1")
	if !strings.Contains(q, "The third step is asserted") ||
		!strings.Contains(q, "The case $n = 0$ is not covered") {
		t.Errorf("the correction call was not told what was wrong: %s", first(q, 400))
	}
	if err := Validate(got.Solution); err != nil {
		t.Errorf("the store would refuse it: %v", err)
	}
}

// A correction that works. The budget is spent only as far as it has to be.
func TestACorrectionThatLands(t *testing.T) {
	a := &asker{by: map[string][]string{
		"reference": {reference}, "candidate-direct": {answer},
		"candidate-contrapositive": {answer}, "candidate-elementary": {answer},
		"select":    {"SELECTED: 1"},
		"truth":     {fail},
		"audit":     {auditFail},
		"correct-1": {"The repaired answer.\n\nUSES: 0001\n"},
		"truth-1":   {pass}, "audit-1": {auditPass},
	}}
	got, err := engine(a).Solve(context.Background(), exercise("Prove it."))
	if err != nil {
		t.Fatal(err)
	}
	if got.Solution.Meta.Status != corpus.StatusVerified {
		t.Errorf("status %q", got.Solution.Meta.Status)
	}
	if got.Solution.Meta.Corrections != 1 {
		t.Errorf("corrections %d", got.Solution.Meta.Corrections)
	}
	if !strings.Contains(got.Solution.Body, "repaired") {
		t.Errorf("the corrected text is not what was written: %q", got.Solution.Body)
	}
}

// Only the judge that failed is complained about. Sending the judgement that
// passed invites the model to fix what nobody objected to.
func TestOnlyTheJudgeThatFailedIsQuoted(t *testing.T) {
	a := &asker{by: map[string][]string{
		"reference": {reference}, "candidate-direct": {answer},
		"candidate-contrapositive": {answer}, "candidate-elementary": {answer},
		"select": {"SELECTED: 1"},
		"truth":  {pass}, "audit": {auditFail},
		"correct-1": {answer}, "truth-1": {pass}, "audit-1": {auditPass},
	}}
	if _, err := engine(a).Solve(context.Background(), exercise("Prove it.")); err != nil {
		t.Fatal(err)
	}
	q := a.question(t, "correct-1")
	if strings.Contains(q, "Every obligation is discharged") {
		t.Error("the correction call was sent the judgement that passed")
	}
	if !strings.Contains(q, "The case $n = 0$ is not covered") {
		t.Error("the correction call was not sent the judgement that failed")
	}
}

func TestSomePartsRightAndSomeNot(t *testing.T) {
	a := &asker{by: map[string][]string{
		"reference": {reference}, "candidate-direct": {answer},
		"candidate-contrapositive": {answer}, "candidate-elementary": {answer},
		"select": {"SELECTED: 1"},
		"truth": {"PART a: PASS\nPART b: FAIL, the finiteness argument has a gap\n" +
			"PART c: PASS\n" + fail},
		"audit":     {"PART a: PASS\nPART b: FAIL, same gap\nPART c: PASS\n" + auditFail},
		"correct-1": {answer}, "correct-2": {answer},
		"truth-1": {"PART a: PASS\nPART b: FAIL, the finiteness argument has a gap\nPART c: PASS\n" + fail},
		"audit-1": {"PART a: PASS\nPART b: FAIL, same gap\nPART c: PASS\n" + auditFail},
		"truth-2": {"PART a: PASS\nPART b: FAIL, the finiteness argument has a gap\nPART c: PASS\n" + fail},
		"audit-2": {"PART a: PASS\nPART b: FAIL, same gap\nPART c: PASS\n" + auditFail},
	}}
	got, err := engine(a).Solve(context.Background(),
		exercise("a) Prove the first thing. b) Prove the second. c) Deduce the third."))
	if err != nil {
		t.Fatal(err)
	}
	m := got.Solution.Meta
	if m.Status != corpus.StatusPartial {
		t.Fatalf("status %q", m.Status)
	}
	if len(m.Parts) != 3 {
		t.Fatalf("parts %+v", m.Parts)
	}
	for i, want := range []corpus.Part{
		{ID: "a", Status: corpus.StatusVerified},
		{ID: "b", Status: corpus.StatusUnverified, Reason: "the finiteness argument has a gap"},
		{ID: "c", Status: corpus.StatusVerified},
	} {
		if m.Parts[i] != want {
			t.Errorf("part %d is %+v, want %+v", i, m.Parts[i], want)
		}
	}
	// The judges were asked about the parts by name, which is the only way the
	// per-part lines come back at all.
	if !strings.Contains(a.question(t, "truth"), "lettered parts: a, b and c") {
		t.Error("the truth judge was not told the exercise has parts")
	}
	if err := Validate(got.Solution); err != nil {
		t.Errorf("the store would refuse it: %v", err)
	}
}

// A part one judge passed and the other failed is not verified. Two judges is
// two judges, and a part that survives only one of them has survived the easier
// reading.
func TestAPartTheJudgesDisagreeAbout(t *testing.T) {
	a := &asker{by: map[string][]string{
		"reference": {reference}, "candidate-direct": {answer},
		"candidate-contrapositive": {answer}, "candidate-elementary": {answer},
		"select":    {"SELECTED: 1"},
		"truth":     {"PART a: PASS\nPART b: PASS\n" + pass},
		"audit":     {"PART a: PASS\nPART b: FAIL, the second case is missing\n" + auditFail},
		"correct-1": {answer}, "correct-2": {answer},
		"truth-1": {"PART a: PASS\nPART b: PASS\n" + pass},
		"audit-1": {"PART a: PASS\nPART b: FAIL, the second case is missing\n" + auditFail},
		"truth-2": {"PART a: PASS\nPART b: PASS\n" + pass},
		"audit-2": {"PART a: PASS\nPART b: FAIL, the second case is missing\n" + auditFail},
	}}
	got, err := engine(a).Solve(context.Background(), exercise("a) One. b) Two."))
	if err != nil {
		t.Fatal(err)
	}
	m := got.Solution.Meta
	if m.Status != corpus.StatusPartial {
		t.Fatalf("status %q, parts %+v", m.Status, m.Parts)
	}
	if m.Parts[1].Status != corpus.StatusUnverified ||
		!strings.Contains(m.Parts[1].Reason, "second case") {
		t.Errorf("part b is %+v", m.Parts[1])
	}
}

// An answer that cannot be read is asked again with a note saying what was
// missing. This is for an answer that did not carry its decision, not for one
// that decided something unwelcome.
func TestAnUnreadableAnswerIsAskedAgain(t *testing.T) {
	a := &asker{by: map[string][]string{
		"reference": {reference}, "candidate-direct": {answer},
		"candidate-contrapositive": {answer}, "candidate-elementary": {answer},
		"select": {"SELECTED: 1"},
		"truth":  {"It is quite good.", pass},
		"audit":  {auditPass},
	}}
	got, err := engine(a).Solve(context.Background(), exercise("Prove it."))
	if err != nil {
		t.Fatal(err)
	}
	if got.Solution.Meta.Status != corpus.StatusVerified {
		t.Errorf("status %q", got.Solution.Meta.Status)
	}
	var second string
	for _, q := range a.asked {
		if strings.HasSuffix(q.id, "truth-2") {
			second = q.question
		}
	}
	if second == "" {
		t.Fatal("the judge was not asked a second time")
	}
	// The note goes at the top. Every one of these prompts ends in the material,
	// and a complaint written under forty thousand characters of Bourbaki is a
	// complaint after the sentence saying everything below is source.
	if !strings.HasPrefix(second, "Your previous answer") {
		t.Errorf("the note is not at the top: %s", first(second, 120))
	}
	if !strings.Contains(first(second, 300), "no VERDICT line") {
		t.Errorf("the note does not say what was missing: %s", first(second, 300))
	}
}

// A candidate that came back as the model apologising is not a candidate. It is
// dropped before the selector, which is asked which of these is most nearly a
// solution and cannot answer that about an apology.
func TestACandidateThatIsNotOne(t *testing.T) {
	a := &asker{by: map[string][]string{
		"reference":                {reference},
		"candidate-direct":         {"I'm sorry, I cannot help with that."},
		"candidate-contrapositive": {answer},
		"candidate-elementary":     {"As an AI language model, I would say"},
		"select":                   {"SELECTED: 1"},
		"truth":                    {pass}, "audit": {auditPass},
	}}
	got, err := engine(a).Solve(context.Background(), exercise("Prove it."))
	if err != nil {
		t.Fatal(err)
	}
	if got.Solution.Meta.Status != corpus.StatusVerified {
		t.Errorf("status %q", got.Solution.Meta.Status)
	}
	// One candidate survived, so there was nothing to select between and the
	// call was not made.
	for _, s := range a.stages() {
		if s == "select" {
			t.Error("the selector was asked to choose the best of one")
		}
	}
	if got.Selected != 1 {
		t.Errorf("selected %d", got.Selected)
	}
}

func TestNothingCameBackAtAll(t *testing.T) {
	a := &asker{by: map[string][]string{"reference": {reference}},
		err: map[string]error{"candidate-direct": errNo, "candidate-contrapositive": errNo,
			"candidate-elementary": errNo}}
	_, err := engine(a).Solve(context.Background(), exercise("Prove it."))
	if err == nil || !strings.Contains(err.Error(), "no candidate") {
		t.Errorf("err %v", err)
	}
}

var errNo = fmt.Errorf("the host would not answer")

// Every question and every answer is archived, including the calls that were
// thrown away. A solution goes into a book and the run is the only evidence of
// how it got there.
func TestEveryCallIsArchived(t *testing.T) {
	a := &asker{by: map[string][]string{
		"reference": {reference}, "candidate-direct": {answer},
		"candidate-contrapositive": {answer}, "candidate-elementary": {answer},
		"select": {"SELECTED: 1"}, "truth": {pass}, "audit": {auditPass},
	}}
	e := engine(a)
	var kept []string
	e.Archive = func(id, question, answer string) error {
		if question == "" || answer == "" {
			return fmt.Errorf("%s was archived with a half of it missing", id)
		}
		kept = append(kept, id)
		return nil
	}
	got, err := e.Solve(context.Background(), exercise("Prove it."))
	if err != nil {
		t.Fatal(err)
	}
	if len(kept) != len(a.asked) {
		t.Errorf("%d calls and %d archived", len(a.asked), len(kept))
	}
	if len(got.Calls) != len(a.asked) {
		t.Errorf("%d calls and %d in the log", len(a.asked), len(got.Calls))
	}
	for _, c := range got.Calls {
		if c.Model != "gpt-5-6" || c.Conversation == "" || c.Question == 0 || c.Answer == 0 {
			t.Errorf("the log says %+v", c)
		}
	}
}

// The selector is given the obligations and not the reference's own derivation.
// Handed the derivation it picks the candidate that reads most like it, which is
// a vote for one route rather than a judgement about which answer is right.
func TestTheSelectorIsGivenTheObligationsAndNoMore(t *testing.T) {
	a := &asker{by: map[string][]string{
		"reference": {reference}, "candidate-direct": {answer},
		"candidate-contrapositive": {answer}, "candidate-elementary": {answer},
		"select": {"SELECTED: 1"}, "truth": {pass}, "audit": {auditPass},
	}}
	if _, err := engine(a).Solve(context.Background(), exercise("Prove it.")); err != nil {
		t.Fatal(err)
	}
	q := a.question(t, "select")
	if !strings.Contains(q, "every submodule of $M$ is finitely generated") {
		t.Error("the selector was not given the obligations")
	}
	if strings.Contains(q, "Independent Derivation") || strings.Contains(q, "Falsification") {
		t.Error("the selector was given the rest of the reference")
	}
	// And the judges are, since that is what they hold the solution to.
	if !strings.Contains(a.question(t, "truth"), "Falsification Checks") {
		t.Error("the truth judge was not given the reference")
	}
	// The audit judge is not, which is the whole of why there are two.
	if strings.Contains(a.question(t, "audit"), "Falsification Checks") {
		t.Error("the audit judge was given the reference")
	}
}

// A call that did not come back at all is not an answer that could not be read.
// The service answers its own failures in the same place it answers questions,
// and an exercise with five calls already spent on it is not worth losing to a
// bad minute on somebody's account.
func TestACallThatDidNotComeBackIsTriedAgain(t *testing.T) {
	a := &asker{by: map[string][]string{
		"reference": {reference}, "candidate-direct": {answer},
		"candidate-contrapositive": {answer}, "candidate-elementary": {answer},
		"select": {"SELECTED: 1"}, "truth": {pass}, "audit": {auditPass},
	}, fails: map[string]int{"truth": 2}}
	got, err := engine(a).Solve(context.Background(), exercise("Prove it."))
	if err != nil {
		t.Fatal(err)
	}
	if got.Solution.Meta.Status != corpus.StatusVerified {
		t.Errorf("status %q", got.Solution.Meta.Status)
	}
	var truth []string
	for _, q := range a.asked {
		if stageOf(q.id) == "truth" {
			truth = append(truth, q.id)
			// No note. A host that dropped the connection was not told anything
			// and did not misread it, and a complaint about an answer nobody sent
			// is a complaint the next model has to work out is not about it.
			if strings.HasPrefix(q.question, "Your previous answer") {
				t.Errorf("%s was sent a note about a call that never landed", q.id)
			}
		}
	}
	// Three asks, numbered apart, so the archive keeps them apart too.
	if want := []string{"alg-viii-s1-ex-1-truth-1", "alg-viii-s1-ex-1-truth-2",
		"alg-viii-s1-ex-1-truth-3"}; !equal(truth, want) {
		t.Errorf("the truth judge was asked %v", truth)
	}
	// And the two that did not come back are not in the call log, because
	// nothing was said and nothing was charged.
	for _, c := range got.Calls {
		if c.Stage == "truth" && c.Attempt != 3 {
			t.Errorf("the log carries a call that never landed: %+v", c)
		}
	}
}

// Review is the judging half with the writing half taken out. It is three calls
// and not seven, and the solution it was given comes back the way it went in.
func TestASolutionJudgedAndNotRewritten(t *testing.T) {
	a := &asker{by: map[string][]string{
		"reference": {reference}, "truth": {pass}, "audit": {auditPass},
	}}
	got, err := engine(a).Review(context.Background(),
		exercise("Prove that $M$ is Noetherian."), answer)
	if err != nil {
		t.Fatal(err)
	}
	if !got.Judged || got.Status != corpus.StatusVerified {
		t.Errorf("judged %v, status %q", got.Judged, got.Status)
	}
	if !got.TruthPassed || !got.AuditPassed {
		t.Errorf("judges %v and %v", got.TruthPassed, got.AuditPassed)
	}
	if want := []string{"reference", "truth", "audit"}; !equal(a.stages(), want) {
		t.Errorf("it ran %v", a.stages())
	}
	// The judges were given the solution as it stands, and no candidate was
	// asked for. A review that wrote its own answer would be judging that one.
	if !strings.Contains(a.question(t, "truth"), "finitely generated") {
		t.Error("the truth judge was not shown the solution")
	}
}

// A solution the judges throw out. The verdict comes back whole, because the
// table says a solution changed and the judgement is the only thing that says
// why.
func TestAReviewThatOverturnsWhatTheFileSays(t *testing.T) {
	a := &asker{by: map[string][]string{
		"reference": {reference}, "truth": {fail}, "audit": {auditFail},
	}}
	got, err := engine(a).Review(context.Background(), exercise("Prove it."), answer)
	if err != nil {
		t.Fatal(err)
	}
	if got.Status != corpus.StatusUnverified {
		t.Errorf("status %q", got.Status)
	}
	if !strings.Contains(got.Truth, "The third step is asserted") ||
		!strings.Contains(got.Audit, "The case $n = 0$ is not covered") {
		t.Error("the judgements did not come back whole")
	}
	if got.WhyTruth == "" {
		t.Error("there is no line saying what the truth judge decided")
	}
	// Nothing was corrected. The question here is whether what was written
	// stands, not what it should have said.
	for _, s := range a.stages() {
		if strings.HasPrefix(s, "correct") {
			t.Errorf("it tried to fix the solution: %v", a.stages())
		}
	}
}

// An exercise the reference stops at is not a solution both judges threw out,
// and a review that reported it as unverified would be reporting a corpus gap
// as a wrong answer.
func TestAReviewOfAnExerciseTheReferenceStops(t *testing.T) {
	a := &asker{by: map[string][]string{
		"reference": {strings.ReplaceAll(reference, "REACH: IN_CORPUS", "REACH: OUT_OF_CORPUS")}}}
	got, err := engine(a).Review(context.Background(), exercise("Prove it."), answer)
	if err != nil {
		t.Fatal(err)
	}
	if got.Judged {
		t.Error("the judges were asked about an exercise that is out of corpus")
	}
	if got.Status != corpus.StatusBlocked {
		t.Errorf("status %q", got.Status)
	}
	if len(a.asked) != 1 {
		t.Errorf("it spent %d calls on it", len(a.asked))
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func first(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

// The audit judge's answer is asked for again when it carries a verdict and no
// work behind it. This is the fifty one bytes exercise 2 of § 1 came back with,
// twice, on a solution the truth judge had failed at four thousand characters.
func TestAnAuditWithNoWorkInItIsAskedAgain(t *testing.T) {
	bare := "PART a: PASS\nPART b: PASS\nVERDICT: PASS\nTRUTH: TRUE\n"
	a := &asker{by: map[string][]string{
		"reference": {reference}, "candidate-direct": {answer},
		"candidate-contrapositive": {answer}, "candidate-elementary": {answer},
		"select": {"SELECTED: 1"}, "truth": {pass},
		"audit": {bare, auditPass},
	}}
	got, err := engine(a).Solve(context.Background(), exercise("Prove it."))
	if err != nil {
		t.Fatal(err)
	}
	if got.Solution.Meta.Status != corpus.StatusVerified {
		t.Errorf("status %q, and the second audit was a pass", got.Solution.Meta.Status)
	}
	var second string
	for _, q := range a.asked {
		if strings.HasSuffix(q.id, "audit-2") {
			second = q.question
		}
	}
	if second == "" {
		t.Fatal("the audit was not asked a second time")
	}
	if !strings.Contains(first(second, 300), "no CHECKED line") {
		t.Errorf("the complaint is not what was wrong: %s", first(second, 200))
	}
}

// A judge that went through the steps and tried nothing is the other half of it.
func TestAnAuditThatSubstitutedNothingIsAskedAgain(t *testing.T) {
	if err := wantAudit("CHECKED: step 1, nothing breaks it, ruled out\n" +
		"VERDICT: PASS\nTRUTH: TRUE\n"); err == nil {
		t.Error("an audit that tried nothing passed the guard")
	}
	if err := wantAudit(auditPass); err != nil {
		t.Errorf("an audit with its work in it was refused: %v", err)
	}
}

// The truth judge's answer is asked for again when it reaches a verdict without
// having written down a reading of the obligations. This is the hundred and
// thirty two bytes exercise 4 of § 1 was filed verified on, against a reference
// that set ten.
func TestATruthJudgementWithNoObligationsInItIsAskedAgain(t *testing.T) {
	bare := "PART a: PASS\nPART b: PASS\nVERDICT: PASS\nTRUTH: TRUE\nCOMPLETE: YES\n" +
		"SELF_CONTAINED: YES\nHUMAN_READABLE: YES\nVERIFIABLE: YES\nSCORE: 7/7\n"
	a := &asker{by: map[string][]string{
		"reference": {reference}, "candidate-direct": {answer},
		"candidate-contrapositive": {answer}, "candidate-elementary": {answer},
		"select": {"SELECTED: 1"}, "truth": {bare, pass},
		"audit": {auditPass},
	}}
	got, err := engine(a).Solve(context.Background(), exercise("Prove it."))
	if err != nil {
		t.Fatal(err)
	}
	if got.Solution.Meta.Status != corpus.StatusVerified {
		t.Errorf("status %q, and the second judgement was a pass", got.Solution.Meta.Status)
	}
	var second string
	for _, q := range a.asked {
		if strings.HasSuffix(q.id, "truth-2") {
			second = q.question
		}
	}
	if second == "" {
		t.Fatal("the truth judge was not asked a second time")
	}
	if !strings.Contains(first(second, 300), "OBLIGATION line") {
		t.Errorf("the complaint is not what was wrong: %s", first(second, 200))
	}
}

// How many lines the judge owes is read off the reference it was given, so an
// exercise whose reference sets ten is not answered with one.
func TestTheObligationsAreCountedOffTheReference(t *testing.T) {
	ten := "## Obligations\n\n" +
		"1. the ascending chain condition holds\n" +
		"2. the module is finitely generated\n" +
		"3. every quotient is Artinian\n\n" +
		"For part b:\n\n" +
		"4. the endomorphism is nilpotent\n" +
		"5. the kernel filtration is finite\n\n" +
		"## Failure Modes\n\n1. assuming the module is free\n"
	if n := countObligations(ten); n != 5 {
		t.Errorf("counted %d obligations, want 5", n)
	}
	// The failure modes are a numbered list too, and they are not obligations.
	if err := wantTruthOf(ten)(pass); err == nil {
		t.Error("one line answered a reference that set five obligations")
	}
	// One line is owed even when the reference's own list cannot be read,
	// because the answer to refuse is the one with no reading in it at all.
	if n := countObligations("nothing here looks like a list"); n != 1 {
		t.Errorf("a reference with no readable obligations asks for %d lines", n)
	}
	if err := wantTruthOf(reference)(pass); err != nil {
		t.Errorf("the one line the fixture owes was refused: %v", err)
	}
}

// A model that writes its display the other way round has it turned into the
// corpus's spelling before anything reads it, so what the judges saw and what
// the file carries are the same text. Left to itself the pipeline shipped
// exercise 1 of § 5 of chapter II of Theory of Sets with three of these in it,
// verified by both judges, because nothing downstream can see inside a span the
// dollars do not mark.
func TestTheDelimitersAreTurnedRoundBeforeTheJudgesReadThem(t *testing.T) {
	brackets := "Let \\(N\\) be a submodule. Then\n\\[ N = \\sum_i A x_i \\]\nand the chain stops.\n\nUSES: 0001\n"
	a := &asker{by: map[string][]string{
		"reference":                {reference},
		"candidate-direct":         {brackets},
		"candidate-contrapositive": {brackets},
		"candidate-elementary":     {brackets},
		"select":                   {"Candidate 1 covers the obligation.\n\nSELECTED: 1\n"},
		"truth":                    {pass},
		"audit":                    {auditPass},
	}}
	got, err := engine(a).Solve(context.Background(), exercise("Prove that $M$ is Noetherian."))
	if err != nil {
		t.Fatal(err)
	}
	body := got.Solution.Body
	if strings.Contains(body, `\[`) || strings.Contains(body, `\(`) {
		t.Errorf("the file carries the other spelling: %q", body)
	}
	if !strings.Contains(body, "$$ N = \\sum_i A x_i $$") || !strings.Contains(body, "$N$") {
		t.Errorf("the display or the span did not come out as the corpus writes it: %q", body)
	}
	// And the judges read it that way, which is what makes the verdict about
	// the file rather than about a draft of it.
	for _, stage := range []string{"truth", "audit"} {
		if q := a.question(t, stage); strings.Contains(q, `\[`) {
			t.Errorf("the %s judge was shown the other spelling", stage)
		}
	}
}

// The solver writes Markdown somebody's own hand wrote, and a bullet list in it
// is a bullet list. The OCR's star repair would put a backslash at the head of
// every item, so the solver gets the rest of the typography and not that.
func TestABulletListInASolutionIsLeftAlone(t *testing.T) {
	list := "There are two cases.\n\n* $N$ is finitely generated.\n* $N$ is not.\n\nUSES: 0001\n"
	a := &asker{by: map[string][]string{
		"reference":                {reference},
		"candidate-direct":         {list},
		"candidate-contrapositive": {list},
		"candidate-elementary":     {list},
		"select":                   {"Candidate 1 covers the obligation.\n\nSELECTED: 1\n"},
		"truth":                    {pass},
		"audit":                    {auditPass},
	}}
	got, err := engine(a).Solve(context.Background(), exercise("Prove that $M$ is Noetherian."))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got.Solution.Body, `\*`) {
		t.Errorf("the bullet list was read as Bourbaki's star: %q", got.Solution.Body)
	}
}
