package share

import (
	"strings"
	"testing"
)

// The seams below are taken off the four Theory of Sets share pages, shortened
// to the last block of one answer and the first of the next. They are the whole
// argument for the joining rule: every one was read by hand against what the
// book prints, and the want column is what the book does.
func TestJoinsTheSeamsTheBookPrints(t *testing.T) {
	cases := []struct {
		name string
		prev string
		next string
		want bool
	}{
		{"a sentence broken at the page turn",
			"faulty reasoning arising from, for example, incorrect use of intuition",
			"or argument by analogy. In practice, the mathematician who wishes",
			true},
		{"broken after a comma",
			"the assertions are indeed the purest truisms (comparable,",
			`for example, to the following: "if a bag of counters contains"`,
			true},
		{"a full stop and a new paragraph",
			"which describes the theory of integers and cardinal numbers.",
			"Thus, written in accordance with the axiomatic method and keeping",
			false},
		// A rule made of punctuation splits this one, because the left side ends
		// in a full stop. The full stop is inside a quotation and the sentence
		// carries on overleaf.
		{"a full stop inside a quotation",
			`to avoid using the expression "natural number $n$" in the definition of a formative construction.`,
			`division ring is a field", "the zeros of $\zeta(s)$ other than $-2,-4$ lie on the line`,
			true},
		// And these four are what a punctuation rule joins and should not. The
		// left side is a display, which is a block of its own, so the words
		// after it are already the next block and there is nothing to glue.
		{"display then a remark",
			"$$\n(B\\mid x')(C\\mid y')(x'\\mid x)(y'\\mid y)A.\n$$",
			"*Remark.* When an abbreviating symbol $\\Sigma$ is introduced",
			false},
		{"display then a heading",
			"$$\nM \\cap (P \\cup Q).\n$$",
			"## 2. CRITERIA OF SUBSTITUTION",
			false},
		{"display then a new sentence",
			"$$\nA=sA_1A_2\\cdots A_n.\n$$",
			"The assemblies of the first species are those which",
			false},
		// The one shape where a display is on the left and there is still
		// something to do: the sentence resumes in lower case, and adjacent
		// blocks are already the right Markdown for it.
		{"display then the rest of the sentence",
			"$$\n\\tau_u\\bigl((y\\mid x)A_j''\\bigr)\n$$",
			"is a term of $\\mathcal{T}$. But this term is identical with",
			false},
		{"emphasis does not begin a sentence",
			"$$\n(A\\text{ and }B)\\Leftrightarrow B\n$$",
			"*is a theorem in $\\mathcal T$.*",
			false},
		{"a numbered item never continues a paragraph",
			"three cases are possible:",
			"1. $A$ is preceded by an assembly $B$",
			false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got, why := joins(c.prev, c.next)
			if got != c.want {
				t.Errorf("joins reported %v, want %v, because %s", got, c.want, why)
			}
		})
	}
}

// The seam this is about is in § 1, no. 3. The page ends mid-sentence at "every
// finite, a footnote is printed under it, and the sentence finishes overleaf
// with "division ring is a field". Joining to the last block puts the sentence
// inside the footnote, where it is wrong and where nobody will look at it
// again.
func TestRunningJoinsPastAFootnote(t *testing.T) {
	out := []string{
		"represent terms. The symbols",
		"$$\n\\pi=\\sqrt{2}+\\sqrt{3},\\qquad 1\\in2,\n$$",
		`"every finite`,
		"(*) As was said above, it would be possible to limit our consideration to signs of weight $2$.",
	}
	at, why := running(out, 0)
	if at != 2 {
		t.Fatalf("running picked block %d, want 2, because %q", at, why)
	}
	if !strings.Contains(why, "footnote") {
		t.Errorf("the reason does not say why: %q", why)
	}
	if at, _ := running([]string{"(*) a footnote and nothing else"}, 0); at != -1 {
		t.Errorf("an answer of nothing but footnotes offered block %d to join to", at)
	}
}

func TestMathRewritesTheDelimiters(t *testing.T) {
	got := math(strings.Join([]string{
		// The second span carries the padding a model put inside it once in
		// 1069 spans. A dollar with a space after it does not open mathematics.
		`Let \(A\) and \( B \) be assemblies.`,
		``,
		`\[`,
		`(B\mid x)C.`,
		`\]`,
		``,
		// The one display close in 466 that is not alone on its line: a
		// quotation with a display inside it, indented, with the closing
		// quotation mark behind the delimiter.
		`   "the zeros lie on the line`,
		`   \[`,
		`   \operatorname{R}(s)=1/2.`,
		`   \]"`,
	}, "\n"))
	want := strings.Join([]string{
		`Let $A$ and $B$ be assemblies.`,
		``,
		`$$`,
		`(B\mid x)C.`,
		`$$`,
		``,
		`   "the zeros lie on the line`,
		`   $$`,
		`   \operatorname{R}(s)=1/2.`,
		`   $$`,
		`   "`,
	}, "\n")
	if got != want {
		t.Errorf("got\n%s\nwant\n%s", got, want)
	}
}

func TestMarkdownRefusesAnAnswerThatIsNotATranscription(t *testing.T) {
	for name, answer := range map[string]string{
		"a narration": "Sure! Here's the transcription of the page:\n\nLet $A$ be a ring.",
		"a refusal":   "I'm sorry, I can't help with that.",
		"lost upload": "I don't see an image attached. Could you upload the page again?",
		"markup":      ":::writing{variant=\"document\" id=\"58321\"}\nLet $A$ be a ring.\n:::",
	} {
		t.Run(name, func(t *testing.T) {
			_, err := Markdown(&Conversation{Turns: []Turn{{Text: "Let $A$ be a ring."}, {Text: answer}}})
			if err == nil {
				t.Fatal("the import was accepted")
			}
			if !strings.Contains(err.Error(), "answer 2 of 2") {
				t.Errorf("the error does not name the answer: %v", err)
			}
		})
	}
}

func TestMarkdownRendersAConversation(t *testing.T) {
	p, err := Markdown(&Conversation{Turns: []Turn{
		{Text: "## 1. TERMS AND RELATIONS\n\nLet \\(A\\) be an assembly. We shall say that", Model: "gpt-5-6-thinking"},
		{Text: "it is of the first species.\n\n\\[\nA=sA_1A_2.\n\\]", Model: "gpt-5-6-thinking"},
	}})
	if err != nil {
		t.Fatal(err)
	}
	want := "## 1. TERMS AND RELATIONS\n\nLet $A$ be an assembly. We shall say that it is of the first species.\n\n$$\nA=sA_1A_2.\n$$\n"
	if p.Body != want {
		t.Errorf("got\n%q\nwant\n%q", p.Body, want)
	}
	if len(p.Boundaries) != 1 || !p.Boundaries[0].Joined {
		t.Errorf("boundaries are %+v", p.Boundaries)
	}
	if len(p.Models) != 1 {
		t.Errorf("models are %v, want the one that wrote both answers, once", p.Models)
	}
}
