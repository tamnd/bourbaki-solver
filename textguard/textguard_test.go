package textguard

import (
	"strings"
	"testing"
)

func TestRefusalsAreCaught(t *testing.T) {
	for _, text := range []string{
		"I'm sorry, I can't help with that.",
		"I am unable to transcribe this image.",
		"As an AI language model, I cannot read images.",
		"I apologize, but the image is not clear enough.",
	} {
		leaks := Check(text)
		if len(leaks) == 0 {
			t.Errorf("not caught: %q", text)
			continue
		}
		if leaks[0].Kind != "refusal" {
			t.Errorf("%q came back as %s, want refusal", text, leaks[0].Kind)
		}
	}
}

func TestNarrationIsCaughtEvenWhenTheRestLooksRight(t *testing.T) {
	// This is the dangerous one. The page under the first line may be a
	// perfectly good transcription, or the model may have summarised it, and
	// from the text alone there is no telling which.
	text := strings.Join([]string{
		"Here is the transcription of the image:",
		"",
		"A I.24  ALGEBRAIC STRUCTURES  § 4",
		"",
		"**Proposition 4.** — Let $G$ be a group.",
	}, "\n")
	leaks := Check(text)
	if len(leaks) != 1 || leaks[0].Kind != "meta" {
		t.Fatalf("narration not caught: %+v", leaks)
	}
	if leaks[0].Line != 1 {
		t.Fatalf("narration reported on line %d, want 1", leaks[0].Line)
	}
}

func TestClosingRemarksAreCaught(t *testing.T) {
	text := "A I.24\n\nSome real content here.\n\nLet me know if you need anything else!"
	leaks := Check(text)
	if len(leaks) != 1 {
		t.Fatalf("closing remark not caught: %+v", leaks)
	}
	if leaks[0].Line != 5 {
		t.Fatalf("reported on line %d, want 5", leaks[0].Line)
	}
}

func TestThePromptComingBackIsCaught(t *testing.T) {
	text := "Transcribe the complete text and all mathematical content from this image"
	leaks := Check(text)
	if len(leaks) == 0 || leaks[0].Kind != "prompt" {
		t.Fatalf("the prompt echoed back was not caught: %+v", leaks)
	}
}

func TestEmptyIsALeak(t *testing.T) {
	for _, text := range []string{"", "   ", "\n\n\t\n"} {
		leaks := Check(text)
		if len(leaks) != 1 || leaks[0].Kind != "empty" {
			t.Fatalf("%q came back as %+v", text, leaks)
		}
	}
}

func TestRealBourbakiPassesClean(t *testing.T) {
	// Mathematics that contains words the checks look for. "I cannot" is a
	// refusal, but a corpus about groups is full of the word violates in the
	// mathematical sense, and of sentences that begin with I as a chapter
	// number. Anything here that fails is a false positive that would reject a
	// good page and cost 151 seconds to read again.
	pages := []string{
		`A I.24  ALGEBRAIC STRUCTURES  § 4

**Proposition 4.** — Let $G$ be a group and $H$ a subgroup of $G$. For $H$ to be
normal in $G$ it is necessary and sufficient that $xHx^{-1} = H$ for all $x \in G$.

The image of $\mathbf{Z}$ under $f$ is a subring of $\mathbf{Q}$ (I, p. 23, Proposition 4).

☡ The image of a submodule is not in general a submodule.`,
		`A IV.7  POLYNOMIALS AND RATIONAL FRACTIONS  § 1

EXERCISES

1. Let $A$ be a commutative ring. Show that the canonical image contains
$\mathbf{N}$, and that the canonical map violates no relation (Set Theory, III, § 3, no. 6).`,
	}
	for i, page := range pages {
		if leaks := Check(page); len(leaks) != 0 {
			t.Errorf("page %d was rejected: %+v", i+1, leaks)
		}
	}
}

func TestFencesAreStrippedNotRejected(t *testing.T) {
	text := "```markdown\nA I.24  ALGEBRAIC STRUCTURES\n\n**Proposition 4.** — Let $G$ be a group.\n```"
	got := Strip(text)
	if strings.Contains(got, "```") {
		t.Fatalf("the fence survived:\n%s", got)
	}
	if !strings.HasPrefix(got, "A I.24") {
		t.Fatalf("stripping ate the content:\n%s", got)
	}
	// A fence inside the page, around a diagram, is content and stays.
	inner := "A I.24\n\n```diagram\nA --> B\n```\n\nmore text"
	if Strip(inner) != inner {
		t.Fatalf("an inner fence was stripped:\n%s", Strip(inner))
	}
}

func TestNormaliseFixesWhatIsUnambiguouslyWrong(t *testing.T) {
	got := Normalise(`The ring $\mathbb{Z}$ and the field $\mathbb{Q}$, with ⚠ in the margin.   `)
	for _, want := range []string{`\mathbf{Z}`, `\mathbf{Q}`, "☡"} {
		if !strings.Contains(got, want) {
			t.Errorf("normalise did not produce %q: %s", want, got)
		}
	}
	if strings.Contains(got, `\mathbb`) {
		t.Errorf("blackboard bold survived: %s", got)
	}
	if strings.HasSuffix(got, " ") {
		t.Errorf("trailing space survived: %q", got)
	}
}

func TestNormaliseLeavesMathematicsAlone(t *testing.T) {
	// The minus sign, the two dash lengths and the quotation marks all mean
	// something here. A normaliser that flattened them would be changing the
	// text of the book.
	text := `$-1$ and $x - y$, the range 1–3, the rule — and "quotes"`
	if got := Normalise(text); got != text {
		t.Fatalf("normalise changed mathematics:\n%s\n%s", text, got)
	}
}

func TestCleanAgreesWithCheck(t *testing.T) {
	if !Clean("A I.24  ALGEBRAIC STRUCTURES") {
		t.Error("a good page is not clean")
	}
	if Clean("I'm sorry, I can't do that") {
		t.Error("a refusal is clean")
	}
}

// The three answers below are verbatim from the first live batch on server3, 10
// August 2026. Every one of them was written into the corpus as a page of
// Algebra I, because the apostrophe was the typographic one and no phrase here
// was spelled with it.
func TestAnAnswerFromAModelThatNeverGotThePage(t *testing.T) {
	answers := []string{
		"I don’t see an image attached to this message. Please upload the page image, and I’ll transcribe it exactly according to your specifications.",
		"I don’t see the image attached. Please upload the page image, and I’ll transcribe it exactly according to your specifications.",
		"Please upload the image page you want transcribed.",
	}
	for _, answer := range answers {
		leaks := Check(answer)
		if len(leaks) == 0 {
			t.Fatalf("no leak found in:\n%s", answer)
		}
		if leaks[0].Kind != "no-image" {
			t.Errorf("kind = %q, want no-image, in:\n%s", leaks[0].Kind, answer)
		}
	}
}

// The apostrophe fix is not only about the new phrases. Every refusal in the
// list was spelled with the ASCII one and a model writes the other.
func TestARefusalWithATypographicApostropheIsStillARefusal(t *testing.T) {
	for _, answer := range []string{
		"I’m sorry, I can’t transcribe this page.",
		"I’m unable to help with that.",
		"I can’t help with copyrighted material.",
	} {
		leaks := Check(answer)
		if len(leaks) == 0 || leaks[0].Kind != "refusal" {
			t.Errorf("no refusal found in %q: %+v", answer, leaks)
		}
	}
}

// Mathematics that talks about images must survive. Bourbaki does: the image of
// a homomorphism is on most pages of chapter I.
func TestTheWordImageInMathematicsIsNotALeak(t *testing.T) {
	for _, page := range []string{
		"A I.24  ALGEBRAIC STRUCTURES\n\nThe image of $f$ is a subgroup of $H$.",
		"Let $N$ be the inverse image of the identity element under $f$.",
		"We do not see any reason to distinguish the two images here.",
		"The image is not attached to any particular choice of basis, as we show below.",
	} {
		if leaks := Check(page); len(leaks) > 0 && leaks[0].Kind == "no-image" {
			t.Errorf("mathematics read as a failed upload: %q in %q", leaks[0].Detail, page)
		}
	}
}

// A single letter argument needs no braces and a model does not always write
// them. The first live page of Algebra I came back with both forms in it.
func TestBlackboardBoldWithoutBracesIsNormalisedToo(t *testing.T) {
	cases := map[string]string{
		`In $\mathbb Z$ (and more generally, in any totally ordered set)`: `In $\mathbf{Z}$ (and more generally, in any totally ordered set)`,
		`$\mathbb{Z}$ and $\mathbb N$ and $\mathbb  Q$`:                   `$\mathbf{Z}$ and $\mathbf{N}$ and $\mathbf{Q}$`,
		`the set $\mathbb R^n$`:                                           `the set $\mathbf{R}^n$`,
	}
	for text, want := range cases {
		if got := Normalise(text); got != want {
			t.Errorf("normalise(%q)\n got %q\nwant %q", text, got, want)
		}
	}
}

// Only the single letters this corpus uses for its number sets. A macro whose
// name merely starts with one of them keeps its tail.
func TestNormaliseDoesNotEatALongerMacroName(t *testing.T) {
	for _, text := range []string{
		`$\mathbb Zeta$`,
		`$\mathbb{Zeta}$`,
		`$\mathbb X$`,
	} {
		if got := Normalise(text); got != text {
			t.Errorf("normalise(%q) = %q, want it left alone", text, got)
		}
	}
}
