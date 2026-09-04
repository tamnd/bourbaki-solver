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

// The work the audit judge did is counted so that an answer which did none can
// be told from one that did. The fifty one bytes in the second case are what
// exercise 2 of § 1 came back with, twice, on a solution the truth judge had
// just failed at four thousand characters.
func TestTheWorkBehindAnAuditIsCounted(t *testing.T) {
	full := `CHECKED: step 1, it would fail if $M$ were not finite length, ruled out by the hypothesis
CHECKED: step 2, it would fail if $s$ were not surjective, not ruled out here
**TRIED:** $A = \mathbf{Z}/4$, the argument still runs

VERDICT: FAIL
TRUTH: FALSE
`
	if d := Read(full); d.Checked != 2 || d.Tried != 1 {
		t.Errorf("read %d checked and %d tried, want 2 and 1", d.Checked, d.Tried)
	}
	if d := Read("PART a: PASS\nPART b: PASS\nVERDICT: PASS\nTRUTH: TRUE\n"); d.Checked != 0 || d.Tried != 0 {
		t.Errorf("an audit with no work in it read %d checked and %d tried", d.Checked, d.Tried)
	}
	// A line with nothing after the colon is the keyword and not the work.
	if d := Read("CHECKED:\nTRIED:   \n"); d.Checked != 0 || d.Tried != 0 {
		t.Errorf("two empty lines read as %d checked and %d tried", d.Checked, d.Tried)
	}
}

// Counting the work is for the guard, which asks again. The verdict itself is
// not touched by it: a judge that wrote its work in prose and reached a fail has
// still reached a fail.
func TestTheWorkIsNotPartOfTheVerdict(t *testing.T) {
	if ok, why := Read("VERDICT: PASS\nTRUTH: TRUE\n").Audited(); !ok {
		t.Errorf("an audit with no CHECKED line did not pass Audited: %s", why)
	}
}

// One line to an obligation, the number read off the line so that ten lines
// about obligation 1 are not ten obligations checked.
func TestTheObligationsAreReadByNumber(t *testing.T) {
	d := Read(`OBLIGATION 1: DISCHARGED, the second paragraph proves it
**OBLIGATION 2:** NOT DISCHARGED, the base case is missing
OBLIGATION 10: DISCHARGED, by the citation of Proposition 3
OBLIGATION 1: NOT DISCHARGED, on second reading it asserts it

VERDICT: FAIL
`)
	if len(d.Obligations) != 3 {
		t.Fatalf("read %d obligations, want 3: %+v", len(d.Obligations), d.Obligations)
	}
	// In order, and the last word on obligation 1 is the one that counts.
	if d.Obligations[0].N != 1 || d.Obligations[0].Discharged {
		t.Errorf("the first is %+v, and it was answered twice with the second a no",
			d.Obligations[0])
	}
	if d.Obligations[1].N != 2 || d.Obligations[1].Discharged {
		t.Errorf("the second is %+v", d.Obligations[1])
	}
	if d.Obligations[2].N != 10 || !d.Obligations[2].Discharged {
		t.Errorf("the third is %+v", d.Obligations[2])
	}
	if d.Obligations[1].Why != "the base case is missing" {
		t.Errorf("the reason read as %q", d.Obligations[1].Why)
	}
}

// The hundred and thirty two bytes exercise 4 of § 1 was filed verified on.
func TestAVerdictWithNoObligationsInIt(t *testing.T) {
	d := Read("PART a: PASS\nPART b: PASS\nVERDICT: PASS\nTRUTH: TRUE\nCOMPLETE: YES\n" +
		"SELF_CONTAINED: YES\nHUMAN_READABLE: YES\nVERIFIABLE: YES\nSCORE: 7/7\n")
	if len(d.Obligations) != 0 {
		t.Errorf("an answer with no obligation line in it read %+v", d.Obligations)
	}
	// Nothing about the verdict changes. It parses as the pass it says it is,
	// and what refuses it is the guard that knows how many obligations there
	// were.
	if ok, why := d.Passed(); !ok {
		t.Errorf("it did not read as the pass it says it is: %s", why)
	}
}

// The five lines below are the ones the truth judge prompt asks to carry a
// reason. Every one of them is taken from a review the eval threw away.

func TestAQualityFieldThatNamesWhatItComesFromIsStillRead(t *testing.T) {
	d := Read(`VERDICT: FAIL
TRUTH: FALSE, the final inference from one implication to an equivalence.
COMPLETE: NO, obligation 3 is not discharged and the reverse implication is missing.
SELF_CONTAINED: YES
HUMAN_READABLE: YES
VERIFIABLE: NO, the converse proof is absent so the step cannot be checked.
SCORE: 3/7`)
	if !d.HasQuality {
		t.Fatal("a review that answered all four fields with a reason on two of them read as not having answered them")
	}
	if !d.HasTruth || d.Truth {
		t.Fatal("TRUTH: FALSE with the step named after it did not read as false")
	}
	if d.Complete || d.Verifiable {
		t.Fatal("a NO carrying its reason read as a yes")
	}
	if !d.SelfContained || !d.HumanReadable {
		t.Fatal("a bare YES beside two reasoned NOs stopped reading")
	}
}

func TestAReasonedFieldDoesNotTurnANoIntoAYes(t *testing.T) {
	d := Read("COMPLETE: NO, every obligation is discharged and it reads well.")
	if d.Complete {
		t.Fatal("the prose after the comma was read instead of the answer before it")
	}
}

func TestADecisionMentionedInPassingIsStillNotADecision(t *testing.T) {
	d := Read("The solution is not COMPLETE: YES material, whatever it claims.")
	if d.HasQuality {
		t.Fatal("a field named in the middle of a sentence was read as an answer")
	}
}

func TestAValueThatIsTheStartOfALongerWordIsNotADecision(t *testing.T) {
	d := Read("TRUTH: TRUENESS is not what is being asked about here.")
	if d.HasTruth {
		t.Fatal("TRUENESS was read as TRUE")
	}
}

func TestTheDecoratedFormStillReadsWithAReason(t *testing.T) {
	d := Read("**COMPLETE**: **NO**, the induction has no base case.")
	if _, has := boolean(completeLine, "**COMPLETE**: **NO**, the induction has no base case.", "YES"); !has {
		t.Fatal("a bolded field with a reason after it did not read")
	}
	if d.Complete {
		t.Fatal("a bolded NO read as a yes")
	}
}
