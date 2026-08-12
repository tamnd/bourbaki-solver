package textguard

import (
	"strings"
	"testing"
)

// The pass block as a prompt asks for it, which is the case that has to work
// before any of the awkward ones matter.
const clean = `VERDICT: PASS
TRUTH: TRUE
COMPLETE: YES
SELF_CONTAINED: YES
HUMAN_READABLE: YES
VERIFIABLE: YES
SCORE: 7/7`

func TestAJudgeThatAnsweredCleanly(t *testing.T) {
	d := Read("The argument is sound throughout.\n\n" + clean + "\n")
	if ok, why := d.Passed(); !ok {
		t.Fatalf("a clean pass did not pass: %s", why)
	}
	if d.Score != 7 || !d.Truth || !d.Complete || !d.SelfContained ||
		!d.HumanReadable || !d.Verifiable {
		t.Errorf("read as %+v", d)
	}
}

// A weaker model bolds the decision, bullets it, quotes it, or puts a full stop
// on the end, and none of that changes what it decided. Throwing the solve away
// over a pair of asterisks costs a generation, and the generation is the
// expensive half.
func TestTheDecorationAModelPutsOnADecision(t *testing.T) {
	for _, form := range []string{
		"VERDICT: PASS",
		"**VERDICT: PASS**",
		"**VERDICT:** PASS",
		"- VERDICT: PASS",
		"* **VERDICT**: PASS",
		"> VERDICT: PASS.",
		"`VERDICT: PASS`",
		"### VERDICT: PASS",
		"  verdict: pass  ",
	} {
		if got := Read(form).Verdict; got != "PASS" {
			t.Errorf("%q read as %q", form, got)
		}
	}
	for _, form := range []string{"SCORE: 7/7", "**SCORE: 6 / 7**", "score: 5/7"} {
		if got := Read(form).Score; got < 5 || got > 7 {
			t.Errorf("%q read as %d", form, got)
		}
	}
}

// The other half of the same rule. A decision has to be the whole line, or a
// judge that mentions a verdict in a sentence has cast one.
func TestAVerdictMentionedInPassingIsNotAVerdict(t *testing.T) {
	for _, form := range []string{
		"This is not VERDICT: PASS material.",
		"I would write VERDICT: PASS if the third step held, and it does not.",
		"the rubric says to write VERDICT: PASS or VERDICT: FAIL at the end",
	} {
		if got := Read(form).Verdict; got == "PASS" {
			t.Errorf("%q was read as a pass", form)
		}
	}
}

// A judge that reasons out loud writes the key more than once. What it decided
// is what it said last.
func TestTheLastAnswerWins(t *testing.T) {
	review := "At first reading, VERDICT: PASS\n\nBut the case $n = 0$ is not " +
		"covered.\n\nVERDICT: FAIL\nTRUTH: FALSE\n"
	d := Read(review)
	if d.Verdict != "FAIL" || d.Truth {
		t.Errorf("read as %+v", d)
	}
}

// Missing and negative are different states and only one of them is worth
// asking again about. Read as no, a formatting slip fails a correct solution;
// read as yes, silence passes anything.
func TestSilenceIsNotAnAnswer(t *testing.T) {
	d := Read("The solution is correct in every particular.")
	if d.Verdict != "UNKNOWN" || d.Score != -1 || d.HasTruth || d.HasQuality {
		t.Errorf("an answer with no decision lines read as %+v", d)
	}
	ok, why := d.Passed()
	if ok || !strings.Contains(why, "no verdict") {
		t.Errorf("ok %v, why %q", ok, why)
	}
	// One field short of the set is still short. This is the run that fell over
	// between two lines and it must not be read as a pass.
	partial := strings.ReplaceAll(clean, "VERIFIABLE: YES\n", "")
	ok, why = Read(partial).Passed()
	if ok || !strings.Contains(why, "four publication fields") {
		t.Errorf("ok %v, why %q", ok, why)
	}
}

// Every way a truth judge can withhold a pass, and the words it gets back. The
// message is what goes into the correction prompt, so a wrong one sends the
// model to fix something nobody complained about.
func TestEveryWayATruthJudgeSaysNo(t *testing.T) {
	cases := []struct {
		swap, with, want string
	}{
		{"VERDICT: PASS", "VERDICT: FAIL", "verdict is fail"},
		{"TRUTH: TRUE", "TRUTH: FALSE", "not true"},
		{"COMPLETE: YES", "COMPLETE: NO", "not complete"},
		{"SELF_CONTAINED: YES", "SELF_CONTAINED: NO", "not self contained"},
		{"HUMAN_READABLE: YES", "HUMAN_READABLE: NO", "not readable"},
		{"VERIFIABLE: YES", "VERIFIABLE: NO", "cannot be checked"},
		{"SCORE: 7/7", "SCORE: 5/7", "scores it 5 of 7"},
	}
	for _, c := range cases {
		ok, why := Read(strings.ReplaceAll(clean, c.swap, c.with)).Passed()
		if ok {
			t.Errorf("%s passed", c.with)
			continue
		}
		if !strings.Contains(why, c.want) {
			t.Errorf("%s gave %q, want it to say %q", c.with, why, c.want)
		}
	}
	// 6 of 7 passes and 5 does not. The floor is a decision and not a
	// measurement: a judge that marked a solution down has found something, and
	// one point of slack is what is allowed for a judge being hard to please.
	if ok, why := Read(strings.ReplaceAll(clean, "7/7", "6/7")).Passed(); !ok {
		t.Errorf("6 of 7 did not pass: %s", why)
	}
}

// The audit is asked for less, so it must not fail a solution for want of a
// score line it was never asked to write.
func TestTheAuditIsAskedForLess(t *testing.T) {
	d := Read("VERDICT: PASS\nTRUTH: TRUE\n")
	if ok, why := d.Audited(); !ok {
		t.Fatalf("the audit block did not pass the audit: %s", why)
	}
	if ok, _ := d.Passed(); ok {
		t.Error("the audit block passed the truth judge's bar, which asks for more")
	}
	if ok, why := Read("VERDICT: PASS\n").Audited(); ok || !strings.Contains(why, "truth") {
		t.Errorf("an audit with no truth line: ok %v, why %q", ok, why)
	}
}

func TestPartByPart(t *testing.T) {
	review := "PART a: PASS\nPART b: FAIL, the induction has no base case\n" +
		"**PART c:** PASS\nPART d: FAIL, cites General Topology, which is not here\n" + clean
	d := Read(review)
	if len(d.Parts) != 4 {
		t.Fatalf("got %d parts: %+v", len(d.Parts), d.Parts)
	}
	for i, want := range []struct {
		id     string
		pass   bool
		reason string
	}{
		{"a", true, ""},
		{"b", false, "the induction has no base case"},
		{"c", true, ""},
		{"d", false, "cites General Topology, which is not here"},
	} {
		got := d.Parts[i]
		if got.ID != want.id || got.Pass != want.pass || got.Reason != want.reason {
			t.Errorf("part %d read as %+v, want %+v", i, got, want)
		}
	}
	// One part failing sinks the whole, and the reason names the part, because
	// the next thing that reads this is a person deciding what to rerun.
	ok, why := d.Passed()
	if ok || !strings.Contains(why, "part b") {
		t.Errorf("ok %v, why %q", ok, why)
	}
	// The parts come back in the book's order however the judge wrote them, and
	// a judge that revisits a part has revised it.
	d = Read("PART c: PASS\nPART a: FAIL, no\nPART a: PASS\n")
	if len(d.Parts) != 2 || d.Parts[0].ID != "a" || !d.Parts[0].Pass {
		t.Errorf("read as %+v", d.Parts)
	}
	// Numbered parts happen too, and an exercise with no parts has none.
	if d := Read("PART 1: PASS\nPART 12: FAIL, wrong\n"); len(d.Parts) != 2 {
		t.Errorf("numbered parts read as %+v", d.Parts)
	}
	if d := Read(clean); len(d.Parts) != 0 {
		t.Errorf("an exercise with no part lines read as %+v", d.Parts)
	}
}

func TestWhichCandidateWasChosen(t *testing.T) {
	for text, want := range map[string]int{
		"SELECTED: 2":                          2,
		"**SELECTED: 3**":                      3,
		"SELECTED: 1.":                         1,
		"nothing was selected":                 0,
		"SELECTED: 2\n\nSELECTED: 1":           1,
		"I would have written SELECTED: 2 but": 0,
	} {
		if got := Selected(text); got != want {
			t.Errorf("%q selected %d, want %d", text, got, want)
		}
	}
}

func TestWhatTheExerciseIs(t *testing.T) {
	n, ok := Nature("NATURE: EXPLORATION\nREACH: OUT_OF_CORPUS\n")
	if !ok || n != "EXPLORATION" {
		t.Errorf("nature %q, ok %v", n, ok)
	}
	r, ok := Reach("NATURE: EXPLORATION\nREACH: OUT_OF_CORPUS\n")
	if !ok || r != "OUT_OF_CORPUS" {
		t.Errorf("reach %q, ok %v", r, ok)
	}
	if _, ok := Nature("the exercise asks for a proof"); ok {
		t.Error("prose was read as a nature line")
	}
}

// The tag line is bookkeeping and the body is mathematics, so the line comes out
// of the body and goes to the front matter.
func TestTheTagsASolutionSaysItUsed(t *testing.T) {
	body := "The module is Noetherian, so every submodule is finitely generated.\n\n" +
		"USES: 00QM, 00QN\n"
	uses, rest := Uses(body)
	if len(uses) != 2 || uses[0] != "00QM" || uses[1] != "00QN" {
		t.Fatalf("read as %v", uses)
	}
	if strings.Contains(rest, "USES") {
		t.Errorf("the line was left in the body: %q", rest)
	}
	if !strings.HasSuffix(rest, "finitely generated.\n") {
		t.Errorf("the body came back as %q", rest)
	}

	// A model that answers in two vocabularies at once. The four characters are
	// the half that resolves and the rest is dropped here rather than refused
	// three functions later.
	uses, _ = Uses("proof\n\n**USES:** 00QM, Proposition 1 of § 2, 00QM, 00A2\n")
	if len(uses) != 2 || uses[0] != "00QM" || uses[1] != "00A2" {
		t.Errorf("read as %v", uses)
	}

	// No line at all, and a line with nothing on it. Neither is an error: a
	// solution that leaned on nothing in the corpus is a solution, and one that
	// forgot to say is caught by the audit rather than here.
	if uses, rest := Uses("proof\n"); uses != nil || rest != "proof\n" {
		t.Errorf("read as %v, %q", uses, rest)
	}
	if uses, rest := Uses("proof\n\nUSES:\n"); uses != nil || strings.Contains(rest, "USES") {
		t.Errorf("read as %v, %q", uses, rest)
	}
}
