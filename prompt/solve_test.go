package prompt

import (
	"regexp"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/tags"
	"github.com/tamnd/bourbaki-solver/textguard"
)

// The prompts ask for decisions on fixed lines and textguard reads those lines.
// Nothing in the compiler ties the two together, and the failure when they come
// apart is expensive and quiet: a prompt that starts asking for CONCLUSION where
// the parser reads VERDICT produces judgements that parse as no verdict at all,
// on every exercise, until somebody reads a run by hand.
//
// So the contract is tested from the prompts themselves. Every example decision
// line printed in a prompt is fed to the parser that will read the answer, and
// every key any prompt asks for has to be a key the parser knows.

const someContext = ">>> the exercise to solve: alg-viii-s1-ex-1, tag 00QM\n\n" +
	"Show that every submodule of a Noetherian module is finitely generated.\n\n<<<\n"

// built is the six prompts as they go over the wire, keyed by name.
func built() map[string]string {
	parts := []string{"a", "b"}
	return map[string]string{
		"reference": SolveReference(someContext),
		"candidate": SolveCandidateFor(someContext, Angles()[0], parts),
		"select":    SolveSelect("the exercise", "1. [*] the module is Noetherian", []string{"one", "two"}),
		"truth":     SolveTruth(someContext, "the reference", "the solution", parts),
		"audit":     SolveAudit(someContext, "the solution", parts),
		"correct":   SolveCorrect(someContext, "the solution", "the complaints", parts),
	}
}

// keyLine is a decision line as a prompt prints it as an example. A PART line is
// not of this shape, since the part is lower case and varies with the exercise,
// so it has its own.
var (
	keyLine  = regexp.MustCompile(`(?m)^([A-Z][A-Z_]*): ?(.*)$`)
	partLine = regexp.MustCompile(`(?m)^PART [a-z0-9]+: .*$`)
)

// decisions is every example decision line one prompt prints, joined back into
// something that looks like the end of an answer.
func decisions(prompt string) string {
	var out []string
	out = append(out, partLine.FindAllString(prompt, -1)...)
	for _, m := range keyLine.FindAllStringSubmatch(prompt, -1) {
		out = append(out, m[0])
	}
	return strings.Join(out, "\n")
}

func TestTheReferenceAsksForNatureAndReach(t *testing.T) {
	got := decisions(built()["reference"])
	nature, ok := textguard.Nature(got)
	if !ok || nature != "PROOF" {
		t.Errorf("the reference prompt's own example reads as nature %q, ok %v", nature, ok)
	}
	reach, ok := textguard.Reach(got)
	if !ok || reach != "IN_CORPUS" {
		t.Errorf("the reference prompt's own example reads as reach %q, ok %v", reach, ok)
	}
}

func TestTheTruthJudgeAsksForEverythingThatPassingNeeds(t *testing.T) {
	d := textguard.Read(decisions(built()["truth"]))
	if d.Verdict != "PASS" {
		t.Errorf("verdict %q", d.Verdict)
	}
	if d.Score != 7 {
		t.Errorf("score %d, and the prompt prints 7/7", d.Score)
	}
	if !d.HasTruth || !d.HasQuality {
		t.Errorf("the prompt does not ask for every field Passed needs: %+v", d)
	}
	// The example is a pass, and the parts in it are the two the prompt was
	// built with. The second is the FAIL of the example, so the whole thing does
	// not pass, and that is the right way round: the example shows both verdicts
	// because a prompt that only ever shows PASS teaches the wrong lesson.
	if len(d.Parts) != 2 {
		t.Fatalf("got %d part lines from the prompt's example, want 2: %+v", len(d.Parts), d.Parts)
	}
	if d.Parts[0].ID != "a" || !d.Parts[0].Pass {
		t.Errorf("the first example part is %+v", d.Parts[0])
	}
	if d.Parts[1].Pass || !strings.Contains(d.Parts[1].Reason, "base case") {
		t.Errorf("the second example part is %+v", d.Parts[1])
	}
	// With the parts taken out it is a clean pass, which is what the prompt says
	// the seven lines mean.
	d.Parts = nil
	if ok, why := d.Passed(); !ok {
		t.Errorf("the truth prompt's own example does not pass: %s", why)
	}
}

func TestTheAuditJudgeAsksForWhatAuditingNeeds(t *testing.T) {
	d := textguard.Read(decisions(built()["audit"]))
	d.Parts = nil
	if ok, why := d.Audited(); !ok {
		t.Errorf("the audit prompt's own example does not pass: %s", why)
	}
	// The audit is asked for less on purpose. If it grows a score line, the
	// reason it is a second opinion rather than a second truth judge has gone.
	if d.Score != -1 {
		t.Errorf("the audit prompt asks for a score of %d, and it should not", d.Score)
	}
}

func TestTheSelectorAsksForANumber(t *testing.T) {
	if got := textguard.Selected(decisions(built()["select"])); got != 1 {
		t.Errorf("the selector prompt's own example reads as candidate %d", got)
	}
}

// A candidate ends with the tags it used, and the run lifts that line out of the
// body into the front matter. The prompt's example is the thing to test it on,
// since it is what the model is copying.
func TestACandidateEndsWithTheTagsItUsed(t *testing.T) {
	for _, name := range []string{"candidate", "correct"} {
		body := "The module is Noetherian.\n\nUSES: XXXX, YYYY\n"
		if !strings.Contains(built()[name], "USES: XXXX, YYYY") {
			t.Fatalf("%s does not show the tag line it asks for", name)
		}
		uses, rest := textguard.Uses(body)
		if len(uses) != 2 || uses[0] != "XXXX" || uses[1] != "YYYY" {
			t.Errorf("%s: the example line reads as %v", name, uses)
		}
		if strings.Contains(rest, "USES") {
			t.Errorf("%s: the line was left in the body: %q", name, rest)
		}
	}
}

// The closure. Every key a prompt asks for has to be one the parser reads, so
// adding a line to a prompt and forgetting the parser fails here.
func TestEveryKeyAPromptAsksForIsOneTheParserReads(t *testing.T) {
	known := map[string]bool{
		"VERDICT": true, "TRUTH": true, "COMPLETE": true, "SELF_CONTAINED": true,
		"HUMAN_READABLE": true, "VERIFIABLE": true, "SCORE": true,
		"SELECTED": true, "NATURE": true, "REACH": true, "USES": true,
		"CHECKED": true, "TRIED": true,
	}
	for name, text := range built() {
		for _, m := range keyLine.FindAllStringSubmatch(text, -1) {
			if !known[m[1]] {
				t.Errorf("%s asks for %s and nothing reads it: %q", name, m[1], m[0])
			}
		}
	}
}

// An exercise with no parts must not be asked for a part line, because a judge
// that invents PART a on a one part exercise gives the store a part nothing in
// the corpus corresponds to.
func TestAnExerciseWithNoPartsIsAskedForNoPartLine(t *testing.T) {
	for _, text := range []string{
		SolveTruth(someContext, "r", "s", nil),
		SolveAudit(someContext, "s", nil),
	} {
		if !strings.Contains(text, "do not write a PART line") {
			t.Error("a judge prompt for a part-less exercise does not say so")
		}
		if strings.Contains(text, "PART a:") {
			t.Error("a judge prompt for a part-less exercise shows a part line anyway")
		}
	}
	// A writer is told nothing at all rather than told there are no parts, and
	// the paragraph it would have been closes up behind it. A prompt with two
	// blank lines in the middle reads to a model like something that failed to
	// arrive.
	for _, text := range []string{
		SolveCandidateFor(someContext, Angles()[0], nil),
		SolveCorrect(someContext, "s", "c", nil),
	} {
		if strings.Contains(text, "lettered parts") {
			t.Error("a writing prompt for a part-less exercise talks about parts")
		}
		if strings.Contains(text[:strings.Index(text, ">>>")], "\n\n\n") {
			t.Error("a writing prompt for a part-less exercise has a hole in it")
		}
	}
}

func TestNoPlaceholderSurvives(t *testing.T) {
	for name, text := range built() {
		if strings.Contains(text, "{{") {
			t.Errorf("%s went out with a placeholder still in it", name)
		}
		if !strings.Contains(text, someContext[:40]) && name != "select" {
			t.Errorf("%s went out without the context in it", name)
		}
		if strings.Contains(text, "\n\n\n") {
			t.Errorf("%s has a hole in it where something empty was substituted", name)
		}
	}
}

// The three angles have to actually differ, since three identical asks is one
// ask with the cost of three.
func TestTheAnglesAreThreeDifferentAsks(t *testing.T) {
	seen := map[string]bool{}
	for _, a := range Angles() {
		if a.Name == "" || len(a.Instruction) < 80 {
			t.Errorf("angle %q is not an instruction: %q", a.Name, a.Instruction)
		}
		if seen[a.Name] || seen[a.Instruction] {
			t.Errorf("angle %q is a repeat", a.Name)
		}
		seen[a.Name], seen[a.Instruction] = true, true
		if !strings.Contains(SolveCandidateFor(someContext, a, nil), a.Instruction) {
			t.Errorf("angle %q does not reach the prompt", a.Name)
		}
	}
}

// The hash is what marks a solution stale, so it has to move when any of the six
// prompts or any of the angles moves, and it has to be the same twice running.
func TestTheHashCoversTheWholePipeline(t *testing.T) {
	first := SolveSHA256()
	if len(first) != 64 {
		t.Fatalf("the hash is %q", first)
	}
	if first != SolveSHA256() {
		t.Error("two calls, two hashes")
	}
	if first == SHA256(strings.TrimSpace(solveTruth)+"\n") {
		t.Error("the hash covers only one of the six prompts")
	}
}

// list is the difference between "a, b and c" and "[a b c]" in front of a model.
func TestPartsAreNamedTheWayAPersonWouldNameThem(t *testing.T) {
	cases := map[string]string{"a": "a", "a,b": "a and b", "a,b,c": "a, b and c"}
	for in, want := range cases {
		if got := list(strings.Split(in, ",")); got != want {
			t.Errorf("list(%q) = %q, want %q", in, got, want)
		}
	}
	if got := list(nil); got != "" {
		t.Errorf("list(nil) = %q", got)
	}
}

// The audit judge is asked to write down what it checked and what it tried,
// and it is asked for those on lines because a prompt that asks for the work in
// prose gets a verdict and no work. The lines it is shown are the ones the
// parser counts.
func TestTheAuditIsShownTheWorkLinesTheParserCounts(t *testing.T) {
	d := textguard.Read(built()["audit"])
	if d.Checked == 0 || d.Tried == 0 {
		t.Errorf("the audit prompt's own examples read as %d checked and %d tried",
			d.Checked, d.Tried)
	}
}

// The truth judge is asked for one line to an obligation, for the same reason
// the audit judge is asked for its steps: exercise 4 of § 1 answered ten
// obligations with a hundred and thirty two bytes and scored it 7 of 7. The
// lines the prompt shows have to be lines the parser reads.
func TestTheTruthJudgeIsShownTheObligationLinesTheParserReads(t *testing.T) {
	d := textguard.Read(built()["truth"])
	if len(d.Obligations) != 2 {
		t.Fatalf("the truth prompt's own examples read as %d obligation lines, want 2",
			len(d.Obligations))
	}
	if !d.Obligations[0].Discharged || d.Obligations[1].Discharged {
		t.Errorf("the examples read as %+v, and the prompt shows one of each", d.Obligations)
	}
}

// What the prompt writes on its sample tag line is not a tag any statement can
// have. It used to be 00QM and 00QN, which are Theorem 1 and Corollary 2 of
// Appendix 3, and three of the five answers to exercise 1 of § 1 came back
// citing Hilbert's Nullstellensatz for a fact about Artinian modules over a
// principal ideal domain.
func TestTheSampleTagLineNamesNothingRealResultCanBe(t *testing.T) {
	for _, p := range []struct{ name, text string }{
		{"solve_candidate.md", SolveCandidateFor("the context", Angles()[0], nil)},
		{"solve_correct.md", SolveCorrect("the context", "the solution", "the complaints", nil)},
	} {
		line := usesSample(p.text)
		if line == "" {
			t.Errorf("%s no longer shows the shape of the tag line", p.name)
			continue
		}
		for _, w := range tagsIn(line) {
			if _, err := tags.Parse(w); err == nil {
				t.Errorf("%s writes %s on its sample line, and a statement can carry "+
					"that tag, so a model copying the line cites a real result", p.name, w)
			}
		}
	}
}

var sampleLine = regexp.MustCompile(`(?m)^USES: (.+)$`)
var sampleTag = regexp.MustCompile(`[0-9A-Z]{4}`)

func usesSample(prompt string) string {
	m := sampleLine.FindStringSubmatch(prompt)
	if m == nil {
		return ""
	}
	return m[1]
}

func tagsIn(line string) []string { return sampleTag.FindAllString(line, -1) }

// The truth judge failed 5 of the 10 right answers of the twenty-case set, and
// three of those reviews had cleared every obligation, every failure mode and
// every falsification check before writing the solution off. The summary block
// was not being read off the review. These hold the prompt to the two things
// that were changed about it, because both are one sentence away from being
// deleted by a later edit that reads them as padding.

func TestTheTruthJudgeIsToldToAnswerFromTheWorkItJustDid(t *testing.T) {
	p := built()["truth"]
	for _, want := range []string{
		"read off the work above them",
		"names what it comes from",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the truth judge is no longer told to answer from its own review: %q is gone", want)
		}
	}
}

func TestTheDefaultToFailIsScopedToAStepThatWasNotChecked(t *testing.T) {
	p := built()["truth"]
	if !strings.Contains(p, "Your default is to fail an unchecked step") {
		t.Error("the default to fail is unscoped again, which is what failed three right answers that had passed every check in the review")
	}
	if !strings.Contains(p, "not licence to fail a step you") {
		t.Error("nothing now stops a review that cleared every check from failing the solution anyway")
	}
}

func TestTheTruthJudgeStillFailsWhatItCouldNotSettle(t *testing.T) {
	// The false accept rate is the number that decides whether a verdict is
	// worth anything, it is at 0 of 10, and none of the above may loosen it.
	p := built()["truth"]
	for _, want := range []string{
		"If you are unsure, it is FAIL",
		"VERDICT is PASS only when TRUTH is TRUE",
		"score is 6 or 7",
		"Reading a step and thinking it is probably fine is failing the step",
	} {
		if !strings.Contains(p, want) {
			t.Errorf("the gate was loosened: %q is gone", want)
		}
	}
}

// TestTheTruthJudgeCanWriteTheReasonItIsAskedFor is the far side of the
// contract the file comment names, in the direction that actually broke.
//
// The prompt tells the judge that a field it answers NO "names what it comes
// from, in a few words after the line". Nothing asserted that the parser could
// read a line in that shape, and it could not: the decision lines were anchored
// at the end of the value, so every reasoned NO read as no answer at all,
// HasQuality went false on a review that had answered everything, and the case
// was thrown away. Six of the twenty cases of the built-in eval set were lost
// that way before anybody looked at an archived review.
//
// This asserts the two halves against each other rather than against a literal:
// what the prompt asks for is read out of the prompt, so a prompt that stops
// asking for reasons fails here rather than quietly making the test vacuous.
func TestTheTruthJudgeCanWriteTheReasonItIsAskedFor(t *testing.T) {
	truth := built()["truth"]
	if !strings.Contains(truth, "names what it comes from") {
		t.Fatal("the truth prompt no longer asks a NO to name what it comes from, " +
			"so this test is asserting nothing; delete it or follow the prompt")
	}
	d := textguard.Read(`VERDICT: FAIL
TRUTH: FALSE, the step from one implication to an equivalence.
COMPLETE: NO, obligation 3 is not discharged.
SELF_CONTAINED: YES
HUMAN_READABLE: YES
VERIFIABLE: NO, the converse is absent so the last step cannot be checked.
SCORE: 3/7`)
	if !d.HasQuality {
		t.Error("the judge answered all four fields the way the prompt asks and the parser read none of them")
	}
	if !d.HasTruth || d.Truth {
		t.Error("TRUTH: FALSE with its step named did not read as false")
	}
	if d.Complete || d.Verifiable {
		t.Error("a NO that named what it came from read as a yes")
	}
	if d.Verdict != "FAIL" || d.Score != 3 {
		t.Errorf("verdict %q score %d beside reasoned fields", d.Verdict, d.Score)
	}
}
