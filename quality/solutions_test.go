package quality

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/tags"
)

// No solution here is a solution. Each is the shape of a claim the front matter
// can make about the judges, and the rules are about whether the claim holds
// together, not about whether the mathematics under it is right. Nothing here
// can tell whether a proof is a proof, which is the whole reason the judges
// exist upstream of the audit.
func solution(m corpus.SolutionFrontMatter, body string) Doc {
	if m.Lang == "" {
		m.Lang = "en"
	}
	return Doc{Path: "content/solutions/en/alg/VIII/s1/01.md", Lang: "en",
		Kind: KindSolution, Body: body, head: 1, Solution: &m}
}

const proof = "The module is Noetherian, and so every submodule is finitely generated."

func TestX02VerifiedMeansBothJudgesPassed(t *testing.T) {
	good := solution(corpus.SolutionFrontMatter{Label: "alg-viii-s1-ex-1",
		Status: corpus.StatusVerified, TruthJudge: "pass", AuditJudge: "pass"}, proof)
	if got := run(t, x02, good); len(got) != 0 {
		t.Errorf("a verified solution with both judges passing was reported: %v", got)
	}

	// The audit judge never answered and the status was written anyway. This is
	// the failure the rule exists for: a run that fell over between the two
	// judges and saved what it had.
	half := solution(corpus.SolutionFrontMatter{Label: "alg-viii-s1-ex-1",
		Status: corpus.StatusVerified, TruthJudge: "pass"}, proof)
	got := run(t, x02, half)
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(got), got)
	}
	if !strings.Contains(got[0].Msg, "audit_judge") {
		t.Errorf("the finding does not name the judge that did not pass: %s", got[0].Msg)
	}
}

// partial is the status that says the least on its own, so it is the one with
// the most to check.
func TestX02PartialSaysWhichParts(t *testing.T) {
	cases := []struct {
		name  string
		parts []corpus.Part
		want  string // the words the finding has to carry, empty for no finding
	}{
		{"a genuine partial", []corpus.Part{
			{ID: "a", Status: corpus.StatusVerified},
			{ID: "b", Status: corpus.StatusUnverified, Reason: "the finiteness argument has a gap"},
		}, ""},
		{"no parts at all", nil, "enumerates no parts"},
		{"every part passed, which is verified", []corpus.Part{
			{ID: "a", Status: corpus.StatusVerified},
			{ID: "b", Status: corpus.StatusVerified},
		}, "which is not partial"},
		{"no part passed, which is unverified", []corpus.Part{
			{ID: "a", Status: corpus.StatusUnverified, Reason: "no base case"},
			{ID: "b", Status: corpus.StatusBlocked, Reason: "cites General Topology"},
		}, "which is not partial"},
		{"a part that failed and will not say why", []corpus.Part{
			{ID: "a", Status: corpus.StatusVerified},
			{ID: "b", Status: corpus.StatusUnverified},
		}, "gives no reason"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			d := solution(corpus.SolutionFrontMatter{Label: "alg-viii-s1-ex-1",
				Status: corpus.StatusPartial, Parts: c.parts}, proof)
			got := run(t, x02, d)
			if c.want == "" {
				if len(got) != 0 {
					t.Fatalf("a partial that says which parts was reported: %v", got)
				}
				return
			}
			if len(got) != 1 {
				t.Fatalf("got %d findings, want 1: %v", len(got), got)
			}
			if !strings.Contains(got[0].Msg, c.want) {
				t.Errorf("the finding is %q, want it to say %q", got[0].Msg, c.want)
			}
		})
	}
}

// blocked and open are verdicts and not shrugs, so they carry the reasoning
// that reached them. An empty one is a file that says an exercise cannot be
// done and offers nothing for a reader to disagree with.
func TestX02AVerdictWithNothingUnderIt(t *testing.T) {
	for _, status := range []string{corpus.StatusBlocked, corpus.StatusOpen,
		corpus.StatusUnverified} {
		d := solution(corpus.SolutionFrontMatter{Label: "alg-viii-s1-ex-1", Status: status}, "")
		got := run(t, x02, d)
		if len(got) != 1 {
			t.Fatalf("%s: got %d findings, want 1: %v", status, len(got), got)
		}
		if !strings.Contains(got[0].Msg, "body is empty") {
			t.Errorf("%s: the finding is %q", status, got[0].Msg)
		}
	}
	// The other way round. unattempted is the one status that means the file is
	// a placeholder, and a placeholder with a proof in it is a run that wrote
	// the mathematics and lost the verdict.
	d := solution(corpus.SolutionFrontMatter{Label: "alg-viii-s1-ex-1",
		Status: corpus.StatusUnattempted}, proof)
	got := run(t, x02, d)
	if len(got) != 1 || !strings.Contains(got[0].Msg, "has a body") {
		t.Fatalf("an unattempted solution with a body: %v", got)
	}
}

func TestX03EveryTagASolutionUsesExists(t *testing.T) {
	set := &tags.Set{Tags: []tags.Entry{
		{Tag: "0001", Label: "alg-viii-s1-prop-1"},
		{Tag: "0002", Label: "alg-viii-s1-thm-1"},
	}}
	solutionWith := func(uses ...string) *Corpus {
		return &Corpus{Tags: set, Docs: []Doc{solution(corpus.SolutionFrontMatter{
			Label: "alg-viii-s1-ex-1", Status: corpus.StatusVerified,
			TruthJudge: "pass", AuditJudge: "pass", Uses: uses}, proof)}}
	}
	if got, _ := x03(solutionWith("0001", "0002")); len(got) != 0 {
		t.Errorf("a solution using two tags that exist was reported: %v", got)
	}

	// A tag nothing in the corpus carries. The model named a result the book
	// does not print, or named the right result with the wrong four characters,
	// and either way the corpus cannot follow it.
	got, _ := x03(solutionWith("0001", "ZZZZ"))
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(got), got)
	}
	if !strings.Contains(got[0].Msg, "ZZZZ") {
		t.Errorf("the finding does not name the tag: %s", got[0].Msg)
	}

	// The same tag twice. Harmless in itself, and it means the list was
	// assembled rather than written, which is worth knowing before anything
	// counts a citation out of it.
	got, _ = x03(solutionWith("0001", "0001"))
	if len(got) != 1 || !strings.Contains(got[0].Msg, "twice") {
		t.Fatalf("a repeated tag: %v", got)
	}
}

// The rule cannot run before there is anything to run it on, and the two ways
// it cannot are different: no solutions at all, and solutions that name nothing.
func TestX03SaysWhyItCannotRun(t *testing.T) {
	if why := needUses(&Corpus{}); !strings.Contains(why, "no solutions") {
		t.Errorf("an empty corpus says %q", why)
	}
	c := &Corpus{Docs: []Doc{solution(corpus.SolutionFrontMatter{
		Label: "alg-viii-s1-ex-1", Status: corpus.StatusVerified}, proof)}}
	if why := needUses(c); !strings.Contains(why, "names a tag") {
		t.Errorf("a corpus whose solutions name no tag says %q", why)
	}
	c.Docs[0].Solution.Uses = []string{"0001"}
	if why := needUses(c); why != "" {
		t.Errorf("a corpus whose solution names a tag says %q", why)
	}
}

// The real one: exercise 1 of § 2 of chapter III came back from a free model
// with 181 characters of mathematics in it and not one dollar sign, and every
// rule in the audit passed it.
func TestX05FindsASolutionWrittenInSymbolsRatherThanTeX(t *testing.T) {
	body := "Let Γ be an order on E. If Γ' ⊂ Γ and x ≤ y then ∀i, x ≤_{Γ_i} y."
	d := solution(corpus.SolutionFrontMatter{Label: "alg-viii-s1-ex-1",
		Status: corpus.StatusVerified, TruthJudge: "pass", AuditJudge: "pass"}, body)
	got := run(t, x05, d)
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(got), got)
	}
	if !strings.Contains(got[0].Msg, "written in symbols rather than in TeX") {
		t.Errorf("the finding does not say what is wrong: %s", got[0].Msg)
	}
	if !strings.Contains(got[0].Msg, "Γ") || !strings.Contains(got[0].Msg, "⊂") {
		t.Errorf("the finding does not name the characters it found: %s", got[0].Msg)
	}
}

// The same mathematics, written the way the corpus writes it. Nothing outside a
// span, so nothing to report, and the Greek inside one is not the rule's
// business.
func TestX05AcceptsTheSameMathematicsInTeX(t *testing.T) {
	body := `Let $\Gamma$ be an order on $E$. If $\Gamma' \subset \Gamma$ and $x \le y$ then $\forall i$, $x \le_{\Gamma_i} y$. And Γ.`
	d := solution(corpus.SolutionFrontMatter{Label: "alg-viii-s1-ex-1",
		Status: corpus.StatusVerified, TruthJudge: "pass", AuditJudge: "pass"}, body)
	got := run(t, x05, d)
	if len(got) != 1 {
		t.Fatalf("got %d findings, want the one stray Greek letter: %v", len(got), got)
	}
	if !strings.Contains(got[0].Msg, "1 characters") {
		t.Errorf("counted more than the stray: %s", got[0].Msg)
	}
}

// A solution with no mathematics outside its spans at all is the ordinary case,
// and the rule has to be silent on it or every solution carries a finding.
func TestX05IsSilentOnOrdinaryProse(t *testing.T) {
	d := solution(corpus.SolutionFrontMatter{Label: "alg-viii-s1-ex-1",
		Status: corpus.StatusVerified, TruthJudge: "pass", AuditJudge: "pass"}, proof)
	if got := run(t, x05, d); len(got) != 0 {
		t.Errorf("ordinary prose was reported: %v", got)
	}
}

func TestX06FindsASolutionWrittenOnTheFreeGateway(t *testing.T) {
	d := solution(corpus.SolutionFrontMatter{Label: "alg-viii-s1-ex-1",
		Status: corpus.StatusVerified, TruthJudge: "pass", AuditJudge: "pass",
		Model: "nemotron-3-ultra-free"}, proof)
	got := run(t, x06, d)
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(got), got)
	}
	if !strings.Contains(got[0].Msg, "nemotron-3-ultra-free") {
		t.Errorf("the finding does not name the model: %s", got[0].Msg)
	}
	if !strings.Contains(got[0].Msg, "no printed page behind it") {
		t.Errorf("the finding does not say why a solution is the worse case: %s", got[0].Msg)
	}
}

// A judge verdict of pass does not excuse the route. solve eval has not been
// run against the benchmark yet, so what a pass is worth is unmeasured, and a
// rule that let a self-declared pass silence it would be reporting the
// pipeline's opinion of itself.
func TestX06IsNotSilencedByAPassingJudge(t *testing.T) {
	d := solution(corpus.SolutionFrontMatter{Label: "alg-viii-s1-ex-1",
		Status: corpus.StatusVerified, TruthJudge: "pass", AuditJudge: "pass",
		Model: "laguna-s-2.1-free"}, proof)
	if got := run(t, x06, d); len(got) != 1 {
		t.Fatalf("a passing judge should not silence the route: %v", got)
	}
}

// One gateway answer anywhere in the file is a file worth asking for again,
// which is how L15 reads a translation written on two routes.
func TestX06FindsTheGatewayAmongSeveralModels(t *testing.T) {
	d := solution(corpus.SolutionFrontMatter{Label: "alg-viii-s1-ex-1",
		Status: corpus.StatusVerified, TruthJudge: "pass", AuditJudge: "pass",
		Model: "gpt-5-6-mini, hy3-free"}, proof)
	if got := run(t, x06, d); len(got) != 1 {
		t.Fatalf("want the gateway found among the models, got %v", got)
	}
}

func TestX06IsSilentOnASubscriptionModel(t *testing.T) {
	d := solution(corpus.SolutionFrontMatter{Label: "alg-viii-s1-ex-1",
		Status: corpus.StatusVerified, TruthJudge: "pass", AuditJudge: "pass",
		Model: "gpt-5-6-mini, gpt-5-6"}, proof)
	if got := run(t, x06, d); len(got) != 0 {
		t.Errorf("a subscription model was reported: %v", got)
	}
}
