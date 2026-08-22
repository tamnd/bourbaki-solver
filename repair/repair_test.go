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

// page51 is the real defect this kind was written for, cut down to the
// paragraph it is on. The scan reads i in [1, n], an interval of integers. The
// transcription has a set with two elements in it. Every rule passes.
const page51 = `DISTRIBUTIVITY

**Definition 5.** Let $E_1,\ldots,E_n$ and $F$ be sets and $u$ a mapping of $E_1\times\cdots\times E_n$ into $F$. Let $i\in\{1,n\}$. Suppose $E_i$ and $F$ are given the structures of magmas. $u$ is said to be distributive relative to the index variable $i$ if the partial mapping

$$
x_i\longmapsto u(a_1,\ldots,a_{i-1},x_i,a_{i+1},\ldots,a_n)
$$

is a homomorphism of $E_i$ into $F$ for all fixed $a_j$ in $E_j$ and $j\ne i$. We leave to the reader the statement and proof of the analogues of Propositions 6, 7 and 8 of the previous paragraph, which are proved the same way and are used in the same places later on.`

func suspect() *Suspect {
	line := strings.Split(page51, "\n")[2]
	return &Suspect{Line: 3, Text: line, Span: `\{1,n\}`,
		Why: "braces here may be an interval printed [1, n] rather than a set with two elements in it"}
}

func glyphRequest() Request {
	return Request{Book: "alg-i-iii", Page: 51, Text: page51, Suspect: suspect()}
}

func glyphExpect() ocr.Expect {
	return ocr.Expect{Book: "alg-i-iii", PDFPage: 51, Grammar: pagemap.FootNumber, HasHead: true, Confidence: pagemap.Unknown}
}

// The expected answer. Most of what a checker flags is right, and confirming
// has to leave the page alone rather than counting as a failed repair.
func TestAConfirmedSpanLeavesThePageAsItIs(t *testing.T) {
	for _, answer := range []string{Unchanged, "unchanged.", "**UNCHANGED**", " UNCHANGED\n"} {
		result := Audit(glyphRequest(), answer, glyphExpect())
		if !result.Confirmed {
			t.Errorf("%q did not read as a confirmation: %s", answer, result.Reason)
		}
		if result.Accepted || result.Text != "" {
			t.Errorf("%q changed the page", answer)
		}
	}
}

func TestACorrectionInsideTheSpanIsAcceptedAndOnlyThatLineMoves(t *testing.T) {
	answer := strings.Replace(suspect().Text, `\{1,n\}`, `[1,n]`, 1)
	result := Audit(glyphRequest(), answer, glyphExpect())
	if !result.Accepted {
		t.Fatalf("the correction was rejected: %s", result.Reason)
	}
	want := strings.Replace(page51, `\{1,n\}`, `[1,n]`, 1)
	if result.Text != want {
		t.Errorf("the page came back as\n%s\nwant\n%s", result.Text, want)
	}
}

// A model asked about one span will happily improve the rest of the line while
// it is there, and the page it hands back reads perfectly well. Each of these
// is an answer that has to be thrown away.
func TestEveryWayAModelWandersOffTheSpanIsCaught(t *testing.T) {
	line := suspect().Text
	cases := []struct {
		name   string
		answer string
	}{
		{"a word tidied elsewhere on the line",
			strings.Replace(strings.Replace(line, `\{1,n\}`, `[1,n]`, 1), "magmas", "magmas (see above)", 1)},
		{"the span rewritten into different mathematics",
			strings.Replace(line, `\{1,n\}`, `\{1,2,\ldots,n\}`, 1)},
		{"the whole page handed back", strings.Replace(page51, `\{1,n\}`, `[1,n]`, 1)},
		{"the span unchanged, dressed as a correction", line},
		{"a narrated correction", "I changed the braces to brackets:\n" + strings.Replace(line, `\{1,n\}`, `[1,n]`, 1)},
		{"an empty answer", "   "},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			result := Audit(glyphRequest(), c.answer, glyphExpect())
			if result.Accepted {
				t.Fatalf("accepted: %q", result.Text)
			}
			if result.Reason == "" {
				t.Error("rejected without saying why")
			}
		})
	}
}

// The page on disk can have been rewritten between the audit that raised the
// suspect and the answer coming back, by a re-read of the same page. Writing
// the answer into it then would put a line back that a later read replaced.
func TestASuspectThatNoLongerMatchesThePageIsThrownAway(t *testing.T) {
	request := glyphRequest()
	request.Text = strings.Replace(page51, "Definition 5", "Definition 6", 1)
	answer := strings.Replace(suspect().Text, `\{1,n\}`, `[1,n]`, 1)
	if result := Audit(request, answer, glyphExpect()); result.Accepted {
		t.Fatal("an answer was written into a page that had moved under it")
	}
}

// A page with a defect on it is not a page to ask a delicate question about,
// and a span that is not on its line once has nothing to prove an answer
// against.
func TestWhatIsNotWorthOneQuestion(t *testing.T) {
	withProblem := glyphRequest()
	withProblem.Problems = ocr.Validate(page50, expect(), ocr.Options{})
	if kind, ok := withProblem.Kind(); ok {
		t.Errorf("a page with a real defect was offered as %s", kind)
	}

	twice := glyphRequest()
	twice.Suspect = &Suspect{Line: 3, Text: "a $x$ and a $x$ again", Span: "$x$"}
	if _, ok := twice.Kind(); ok {
		t.Error("a span that is on its line twice was accepted as a suspect")
	}
}

// The prompt has to make confirming the cheap answer. If the only way to reply
// is to hand back a line, a model told the line is suspect will find something
// on it to change.
func TestThePromptOffersTheConfirmingAnswerFirst(t *testing.T) {
	prompt := glyphRequest().Prompt()
	if !strings.Contains(prompt, Unchanged) || !strings.Contains(prompt, Refusal) {
		t.Fatal("the prompt does not give both ways out")
	}
	if strings.Index(prompt, Unchanged) > strings.Index(prompt, "reply with that one line") {
		t.Error("the correcting answer is offered before the confirming one")
	}
	if !strings.Contains(prompt, `\{1,n\}`) {
		t.Error("the prompt does not say which span is in question")
	}
}

// The repair path calls ocr.Validate with empty options, which leaves the echo
// half of rule 3 off, and the question that raises is whether an answer that
// hands the repair prompt back can reach the corpus through here.
//
// It cannot, and not because of rule 3. The two audits above are stricter than
// rule 3 is: the delimiter audit compares the two texts with every dollar
// removed and refuses anything that is not identical, and the glyph audit
// requires the answer to open with the prefix and close with the suffix of the
// line it was asked about, byte for byte. A prompt cannot get past either. The
// echo check would be a third opinion on a question already settled twice, and
// the prompt it would need is the one that read the page, which is in another
// process and another conversation.
//
// These are here so that stays true. If someone loosens either audit, this is
// the test that says what it cost.
func TestAnAnswerThatIsTheRepairPromptIsRefused(t *testing.T) {
	work := request(t)
	for _, c := range []struct {
		name   string
		answer string
	}{
		{"the prompt whole", work.Prompt()},
		{"the rules only", work.Prompt()[strings.Index(work.Prompt(), "Rules, and they are absolute"):]},
		{"the page with the rules after it",
			fixed + "\n\n" + work.Prompt()[strings.Index(work.Prompt(), "Rules, and they are absolute"):]},
		{"the page inside the markers it was quoted in",
			"<<<BEGIN\n" + fixed + "\nEND>>>"},
	} {
		t.Run(c.name, func(t *testing.T) {
			result := Audit(work, c.answer, expect())
			if result.Accepted {
				t.Fatalf("the prompt came back and was accepted as a page:\n%s", result.Text)
			}
			if result.Reason == "" {
				t.Error("rejected without saying why")
			}
		})
	}
}

func TestAGlyphAnswerThatIsTheRepairPromptIsRefused(t *testing.T) {
	work := glyphRequest()
	for _, c := range []struct {
		name   string
		answer string
	}{
		{"the prompt whole", work.Prompt()},
		{"one rule off it", "- Change nothing on the line outside the span quoted above."},
		{"the quoted line with the rules after it",
			suspect().Text + "\n" + work.Prompt()[strings.Index(work.Prompt(), "Rules for answer 2"):]},
	} {
		t.Run(c.name, func(t *testing.T) {
			result := Audit(work, c.answer, glyphExpect())
			if result.Accepted {
				t.Fatalf("the prompt came back and was accepted as a line:\n%s", result.Text)
			}
			if result.Confirmed {
				t.Fatal("the prompt was read as the model confirming the span")
			}
			if result.Reason == "" {
				t.Error("rejected without saying why")
			}
		})
	}
}
