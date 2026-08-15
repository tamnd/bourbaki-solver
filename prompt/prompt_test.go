package prompt

import (
	"strings"
	"testing"
)

func TestOCRPromptSaysWhatBourbakiNeeds(t *testing.T) {
	text := OCR()
	// Each of these is a rule the corpus depends on somewhere else. Losing one
	// in an edit costs a full re-read of 1194 pages at 151 seconds a page, so
	// they are pinned here rather than trusted to review.
	for _, want := range []string{
		"$...$",   // inline math, what the assembler parses
		"$$...$$", // display math
		`\mathbf{Z}`,
		`Never write \mathbb`,
		"running head", // what the page map reads
		"**Definition 3.** —",
		"☡",           // the dangerous bend, a real part of the text
		"⟪illegible⟫", // the honest failure, rather than an invention
		"no. ",        // the subdivision mark, kept as printed
		"EXERCISES",   // the heading the solver keys off
		"[^1]:",       // the footnote definition assembly reads a note out of
		"do not translate",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the OCR prompt no longer says %q", want)
		}
	}
	if strings.Contains(text, `\mathbb{Z}`) && !strings.Contains(text, `Never write \mathbb{Z}`) {
		t.Error("the prompt shows \\mathbb{Z} without saying not to use it")
	}
}

func TestPromptHashIsStableAndSpecific(t *testing.T) {
	first, second := OCRSHA256(), OCRSHA256()
	if first != second {
		t.Fatalf("the same prompt hashed two ways: %s and %s", first, second)
	}
	if len(first) != 64 {
		t.Fatalf("hash is %d characters, want 64", len(first))
	}
	// A page's front matter carries this hash so that a prompt change makes the
	// page stale. If a change did not move the hash, nothing would be re-read.
	if SHA256(OCR()) == SHA256(OCR()+"one more rule\n") {
		t.Fatal("changing the prompt did not change its hash")
	}
}

// The source is fenced on both sides and the body is not the last thing the
// model reads. Five asks against a prompt that opened the fence and never
// closed it came back with the last sentence or two written twice, so the
// closing fence is a rule and not a tidiness.
func TestTheSourceIsFencedOnBothSides(t *testing.T) {
	body := "Denote by A the ring $K[X]$."
	got, err := Translate("vi", "ring | vành\n", "", body)
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(got, "\n==========\n"); n != 2 {
		t.Fatalf("the source is fenced by %d lines of equals signs, want 2", n)
	}
	before, after, ok := strings.Cut(got, body)
	if !ok {
		t.Fatal("the body is not in the prompt")
	}
	if !strings.Contains(before, "==========") {
		t.Error("nothing opens the source")
	}
	if !strings.Contains(after, "==========") {
		t.Error("nothing closes the source, so the model has no place to stop")
	}
	if strings.TrimSpace(after) == "==========" {
		t.Error("the fence closes and says nothing, so it reads as more source")
	}
}

// The retry's complaint is an instruction, so it goes above the source. It was
// appended below it once, which contradicted the sentence saying everything
// between the fences is source and none of it is an instruction.
func TestTheNoteGoesAboveTheSource(t *testing.T) {
	body := "Denote by A the ring $K[X]$."
	note := "Your previous answer to this section was thrown away."
	got, err := Translate("vi", "", note, body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(got, note) {
		t.Fatal("the note is not in the prompt")
	}
	if strings.Index(got, note) > strings.Index(got, "==========") {
		t.Error("the note is inside the source, where the prompt says nothing is an instruction")
	}
	// A first attempt carries no note and must not carry a blank hole either.
	plain, err := Translate("vi", "", "", body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, "{{") || strings.Contains(plain, "\n\n\n") {
		t.Errorf("an empty note left a mark in the prompt: %q", plain)
	}
}

func TestEveryLanguageHasItsOwnRulesAndItsOwnHash(t *testing.T) {
	seen := map[string]string{}
	for _, lang := range []string{"vi", "zh", "ja"} {
		sum, err := TranslateSHA256(lang)
		if err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		if other, ok := seen[sum]; ok {
			t.Errorf("%s and %s hash the same, so a change to one would not mark the other stale", lang, other)
		}
		seen[sum] = lang
		if _, err := Translate(lang, "", "", "body"); err != nil {
			t.Errorf("%s: %v", lang, err)
		}
	}
	if _, err := Translate("fr", "", "", "body"); err == nil {
		t.Error("a language with no rules was translated anyway")
	}
}

func TestOCRPromptEndsWithOneNewline(t *testing.T) {
	text := OCR()
	if !strings.HasSuffix(text, "\n") || strings.HasSuffix(text, "\n\n") {
		t.Fatalf("the prompt should end with exactly one newline, got %q", text[len(text)-3:])
	}
}

// The born-digital prompt carries the rules the pilot on Lie 7 to 9 bought.
// Each one is a defect that shipped before it was written down.
func TestTheBornDigitalPromptSaysWhatThePilotFound(t *testing.T) {
	text := OCRNative()
	for _, want := range []string{
		`\mathfrak{g}`,   // the Lie algebra came back as a plain g
		`\mathscr`,       // the script C came back as \mathcal
		`never \mathcal`, // and it has to be told that in those words
		"### 4. CENTRAL FUNCTIONS ON G AND FUNCTIONS ON T", // a heading came back bold
		"**PROPOSITION 3.** —",                             // and a statement head is bold
		`\mathbf{Z}`,
		"never \\mathbb",
		`\lambda_{\mathfrak{g}}`, // it flattened the fraktur in a subscript
		`\mathbf{SU}`,            // and wrote the classical groups upright
		`Z(\lambda-\rho)`,        // and bolded a Verma module into the integers
		"running head",
		"never as two or three lines", // it wrote the head as three and lost it
		"LaTeX array",                 // a diagram is an array and not three loose rows
		`\tag{11}`,                    // a numbered display keeps its number
		"⟪illegible⟫",                 // the honest failure
		"no. ",
		"translating nothing",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the born-digital prompt no longer says %q", want)
		}
	}
	if OCRNativeSHA256() == OCRSHA256() {
		t.Error("the two prompts hash the same, so a page cannot say which read it")
	}
}

// A volume with a notation of its own reads the shared prompt and its own note,
// and a volume with nothing to add reads the shared prompt byte for byte. The
// second half is the one that matters: a note about the sign for a theory must
// not put a page of Algebra back in the queue.
func TestAVolumeNoteIsAddedAndChangesNoOtherVolume(t *testing.T) {
	ens := OCRFor("ens-i-iv")
	if !strings.HasPrefix(ens, OCR()) {
		t.Error("the shared prompt is not the head of the volume prompt")
	}
	for _, want := range []string{
		`\mathscr{T}`,
		"Theory of Sets and not Algebra",
		"printed at the foot of the page",
	} {
		if !strings.Contains(ens, want) {
			t.Errorf("the Theory of Sets note does not say %q", want)
		}
	}
	if OCRForSHA256("ens-i-iv") == OCRSHA256() {
		t.Error("the note did not change the hash, so nothing would be re-read")
	}
	for _, book := range []string{"alg-i-iii", "alg-x-fr", "top-v-x", "ac-i-vii"} {
		if OCRFor(book) != OCR() {
			t.Errorf("%s no longer reads the shared prompt", book)
		}
		if OCRForSHA256(book) != OCRSHA256() {
			t.Errorf("%s went stale for a note about another volume", book)
		}
	}
}
