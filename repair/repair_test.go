package repair

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/pagemap"
)

// page50 is the tail of page 50 of Algebra I as the fleet returned it. The
// paragraph ends "and $x\in E." with no closing dollar. Everything else on the
// page is right, which is the case for repair: re-reading it costs 151 seconds
// to get back the same 500 words with one character different.
const page50 = `SUBSETS STABLE UNDER AN ACTION

Let $\Omega$, $\Xi$, $E$, $F$ be sets, $\alpha\mapsto f_\alpha$ an action of $\Omega$ on $E$, $\beta\mapsto g_\beta$ an action of $\Xi$ on $F$ and $\phi$ a mapping of $\Omega$ into $\Omega'$. A $\phi$-morphism of $E$ into $F$ is a mapping $h$ of $E$ into $F$ such that

$$
g_{\phi(\alpha)}(h(x))=h(f_\alpha(x))
$$

for all $\alpha\in\Omega$ and $x\in E.

The set of such mappings is stable under composition, and the identity is one of them, so they form a monoid under composition of mappings in the usual way.`

// fixed is the repair that ought to be accepted: one dollar, in one place.
var fixed = strings.Replace(page50, `and $x\in E.`, `and $x\in E$.`, 1)

func expect() ocr.Expect {
	return ocr.Expect{
		Book: "alg-i-iii", PDFPage: 50,
		Grammar: pagemap.FootNumber, HasHead: true,
		Confidence: pagemap.Unknown,
	}
}

func request(t *testing.T) Request {
	t.Helper()
	problems := ocr.Validate(page50, expect(), ocr.Options{})
	if len(problems) == 0 {
		t.Fatal("the fixture is supposed to be a page that fails rule 2")
	}
	return Request{Book: "alg-i-iii", Page: 50, Text: page50, Problems: problems}
}

func TestTheRepairThatOnlyMovesADollarIsAccepted(t *testing.T) {
	result := Audit(request(t), fixed, expect())
	if !result.Accepted {
		t.Fatalf("a correct repair was rejected: %s", result.Reason)
	}
	if result.Text != fixed {
		t.Error("the accepted text is not what came back")
	}
}

// The whole reason this package exists. Each of these is a plausible, fluent
// answer from a model asked to fix a formula, and each one changes the page.
func TestEveryWayAModelRewritesThePageIsCaught(t *testing.T) {
	cases := []struct {
		name   string
		answer string
	}{
		{
			// The one that matters. It closed the formula and it also decided
			// the sentence was missing its end, which Bourbaki did not write.
			"it completed the mathematics",
			strings.Replace(page50, `and $x\in E.`, `and $x\in E$. Thus $h$ is a $\phi$-morphism.`, 1),
		},
		{
			// Tidying the notation. Every symbol is still a symbol, the page is
			// still fluent, and it is not the page.
			"it tidied a symbol",
			strings.Replace(fixed, `$\Omega'$`, `$\Omega^{\prime}$`, 1),
		},
		{
			"it corrected what it took to be a typo",
			strings.Replace(fixed, "stable under composition", "stable under composition of mappings", 1),
		},
		{
			"it dropped a paragraph it thought was redundant",
			strings.Split(fixed, "\n\nThe set of such")[0],
		},
		{
			"it re-transcribed the page and got a word different",
			strings.Replace(fixed, "an action of", "a action of", 1),
		},
		{
			// Cheapest possible way to make the count even, and it is nonsense.
			"it wrapped the whole page to balance the count",
			"$" + fixed + "$",
		},
		{
			"it changed a space",
			strings.Replace(fixed, "for all $\\alpha", "for all  $\\alpha", 1),
		},
	}
	for _, test := range cases {
		result := Audit(request(t), test.answer, expect())
		if result.Accepted {
			t.Errorf("%s: accepted, and it should not have been", test.name)
		}
	}
}

func TestNarrationInARepairIsStillNarration(t *testing.T) {
	result := Audit(request(t), "Here is the transcription with the fix applied:\n\n"+fixed, expect())
	if result.Accepted {
		t.Fatal("a repair that narrates what it repaired was accepted")
	}
}

func TestAnAnswerThatChangesNothingIsNotARepair(t *testing.T) {
	result := Audit(request(t), page50, expect())
	if result.Accepted {
		t.Fatal("the unchanged page was accepted as a repair")
	}
	if !strings.Contains(result.Reason, "no delimiter was changed") {
		t.Errorf("the reason does not say what happened: %s", result.Reason)
	}
}

// A model that cannot do the job has to have somewhere to go, or it does
// something else and calls it the job.
func TestTheModelIsAllowedToSayItCannot(t *testing.T) {
	for _, answer := range []string{Refusal, Refusal + ".", "  " + Refusal + "  \n", "**" + Refusal + "**"} {
		result := Audit(request(t), answer, expect())
		if result.Accepted {
			t.Errorf("%q was accepted as a repaired page", answer)
		}
		if !result.Refused {
			t.Errorf("%q was not read as a refusal", answer)
		}
	}
}

// A page that discusses what cannot be repaired is a page, not a refusal.
func TestAPageIsNotARefusalBecauseItSaysTheWords(t *testing.T) {
	long := strings.Replace(fixed, "The set of such mappings", "The relation CANNOT REPAIR is not one we shall need. The set of such mappings", 1)
	result := Audit(request(t), long, expect())
	if result.Refused {
		t.Error("a page was thrown away for containing the refusal phrase")
	}
}

// A dollar behind a backslash is a printed dollar. Bourbaki's historical notes
// quote prices, and if those were stripped too then adding or removing one
// would look like no change at all.
func TestAnEscapedDollarIsNotADelimiter(t *testing.T) {
	before := `A IV.7  POLYNOMIALS

The prize offered was \$100 for a proof that $G$ is finite.

It was never claimed, and the ring $\mathbf{Z}$ is free of rank one over itself, which is the only fact we need here at all.`
	// The model turned a printed price into a delimiter. Nothing about the
	// dollar count says so, and the text is a different text.
	after := strings.Replace(before, `\$100`, `$100$`, 1)
	req := Request{Text: before, Problems: []ocr.Problem{{Rule: ocr.RuleMath, Detail: "made up for the test"}}}
	if result := Audit(req, after, expect()); result.Accepted {
		t.Error("a printed dollar was turned into a delimiter and it passed")
	}
}

func TestAPageWithNothingToRepairIsNotOffered(t *testing.T) {
	req := Request{Text: page50, Problems: []ocr.Problem{{Rule: ocr.RuleShort, Detail: "45 characters"}}}
	if result := Audit(req, fixed, expect()); result.Accepted {
		t.Fatal("a truncated page was repaired instead of re-read, and the missing text was invented")
	}
	// Mixed is also refused. There is no minimal edit that fixes both, so
	// there is nothing to prove an answer against.
	req.Problems = append(req.Problems, ocr.Problem{Rule: ocr.RuleMath, Detail: "unbalanced"})
	if _, ok := req.Kind(); ok {
		t.Error("a page that is both truncated and unbalanced was offered for repair")
	}
}

func TestTheRepairStillHasToPassTheRules(t *testing.T) {
	// Balanced now, and rule 2 is satisfied, but the page lost its head, so
	// the answer is not a page that may be written.
	headless := strings.Replace(fixed, "SUBSETS STABLE UNDER AN ACTION\n\n", "", 1)
	if result := Audit(request(t), headless, expect()); result.Accepted {
		t.Fatal("a repair that broke another rule was accepted")
	}
}

func TestThePromptSaysTheThingsThatMakeTheAuditPass(t *testing.T) {
	prompt := request(t).Prompt()
	for _, want := range []string{
		page50,   // its own answer, so it is not working from memory
		"line 9", // where, so it is not asked to go looking
		"Change nothing except dollar signs",
		"Do not re-transcribe",
		"Do not add mathematics that is missing",
		Refusal,
	} {
		if !strings.Contains(prompt, want) {
			t.Errorf("the prompt does not contain %q", want)
		}
	}
}
