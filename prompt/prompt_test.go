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
	got, err := Translate("en", "vi", "ring | vành\n", "", body)
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

// A question is archived beside its answer, and the archive is the only record
// of what a model was shown. Reading the passage back out of a finished ask is
// how a later run tells whether an answer it finds on disk was written about the
// text it is holding now.
func TestThePassageReadsBackOutOfTheQuestion(t *testing.T) {
	for _, body := range []string{
		"Denote by A the ring $K[X]$.",
		// Every part of the prompt that varies, so that nothing here depends on
		// the glossary block or the note staying the length they are today.
		"Let $E$ be a set.\n\nLet $F$ be another.\n\n\\tag{alg-i-3-1}",
		// A passage carrying a line the fence would match. The first fence opens
		// and the last one closes, so this comes back whole rather than cut.
		"The table is set out as\n\n==========\n\nand read across.",
	} {
		for _, note := range []string{"", "The formulas were reflowed."} {
			got, err := Translate("en", "vi", "ring | vành\n", note, body)
			if err != nil {
				t.Fatal(err)
			}
			had, ok := TranslatePassage(got)
			if !ok {
				t.Fatalf("no passage was found in the question for %q", body)
			}
			if had != strings.TrimSpace(body) {
				t.Errorf("the question carried %q, want %q", had, body)
			}
		}
	}
	// Something that is not a translation question at all. An OCR prompt is the
	// other thing under work/, and it has no passage in it.
	if _, ok := TranslatePassage(OCR()); ok {
		t.Error("a passage was read out of a question that has none")
	}
	if _, ok := TranslatePassage(""); ok {
		t.Error("a passage was read out of nothing")
	}
}

// The retry's complaint is an instruction, so it goes above the source. It was
// appended below it once, which contradicted the sentence saying everything
// between the fences is source and none of it is an instruction.
func TestTheNoteGoesAboveTheSource(t *testing.T) {
	body := "Denote by A the ring $K[X]$."
	note := "Your previous answer to this section was thrown away."
	got, err := Translate("en", "vi", "", note, body)
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
	plain, err := Translate("en", "vi", "", "", body)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(plain, "{{") || strings.Contains(plain, "\n\n\n") {
		t.Errorf("an empty note left a mark in the prompt: %q", plain)
	}
}

func TestEveryLanguageHasItsOwnRulesAndItsOwnHash(t *testing.T) {
	seen := map[string]string{}
	for _, lang := range []string{"vi", "zh", "ja", "en"} {
		sum, err := TranslateSHA256("en", lang)
		if err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		if other, ok := seen[sum]; ok {
			t.Errorf("%s and %s hash the same, so a change to one would not mark the other stale", lang, other)
		}
		seen[sum] = lang
		if _, err := Translate("en", lang, "", "", "body"); err != nil {
			t.Errorf("%s: %v", lang, err)
		}
	}
	if _, err := Translate("en", "de", "", "", "body"); err == nil {
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
	ens := OCRFor("ens-i-iv", "ens")
	if !strings.HasPrefix(ens, OCR()) {
		t.Error("the shared prompt is not the head of the volume prompt")
	}
	for _, want := range []string{
		`\mathscr{T}`,
		"Theory of Sets and not Algebra",
		"at the foot of the page and not in the running head",
	} {
		if !strings.Contains(ens, want) {
			t.Errorf("the Theory of Sets note does not say %q", want)
		}
	}
	if OCRForSHA256("ens-i-iv", "ens") == OCRSHA256() {
		t.Error("the note did not change the hash, so nothing would be re-read")
	}
	for _, volume := range []string{"alg-i-iii", "alg-x-fr", "top-i-iv", "lie-i-iii"} {
		book := strings.SplitN(volume, "-", 2)[0]
		if OCRFor(volume, book) != OCR() {
			t.Errorf("%s no longer reads the shared prompt", volume)
		}
		if OCRForSHA256(volume, book) != OCRSHA256() {
			t.Errorf("%s went stale for a note about another volume", volume)
		}
	}
}

// Both printings of a Book are read with the Book's notation. This is what the
// old key got wrong: the note was filed under ens-i-iv, so the French printing
// was read with the shared prompt and nothing else, and it wrote \mathcal{T}
// 738 times where the printing that had been told wrote it none.
func TestBothPrintingsOfABookAreReadWithItsNotation(t *testing.T) {
	for _, volume := range []string{"ens-i-iv", "ens-i-iv-fr"} {
		got := OCRFor(volume, "ens")
		for _, want := range []string{
			`\mathscr{T}`,
			"never \\mathcal",
			"pilcrow",
			"CF1 to CF8",
			`\tau_x(A)`,
		} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: the Book note does not say %q", volume, want)
			}
		}
	}
}

// The two lines that really are about the printing go the other way, and each
// one is wrong for the other printing rather than merely absent from it. The
// English heads a section with a bare number and the French heads it with a §,
// and the English is foot-number where the French is head-label.
func TestEachPrintingIsToldHowItsOwnPagesAreSet(t *testing.T) {
	en := OCRFor("ens-i-iv", "ens")
	fr := OCRFor("ens-i-iv-fr", "ens")
	if !strings.Contains(en, "carry no § sign") {
		t.Error("the English printing is not told its headings print no sign")
	}
	if !strings.Contains(fr, "do carry the § sign") {
		t.Error("the French printing is not told its headings print the sign")
	}
	if strings.Contains(fr, "Do not add a sign the page does not print") {
		t.Error("the French printing is told to drop a sign its pages print")
	}
	if !strings.Contains(en, "at the foot of the page and not in the running head") {
		t.Error("the English printing is not told where its folio sits")
	}
	if !strings.Contains(fr, "in the running head and not at the foot") {
		t.Error("the French printing is not told where its folio sits")
	}
	if strings.Contains(fr, "leave the folio at the foot where it is printed") {
		t.Error("the French printing is sent to the foot for a folio in the head")
	}
	if en == fr {
		t.Error("the two printings are read with the same prompt")
	}
}

// The two volumes that print the page number at the foot and have no text layer
// behind them are the two the page map cannot fit, because the shared prompt
// asks for the running head and never for the foot. Measured over what is read
// now, the number is on the last line of 1 of 640 pages of ac-i-vii and 0 of 367
// of top-v-x, against 42% of ac-i-iv-fr, which prints it in the head and maps.
func TestTheVolumesNumberedAtTheFootAreAskedForTheFoot(t *testing.T) {
	for _, volume := range []string{"ac-i-vii", "top-v-x"} {
		book := strings.SplitN(volume, "-", 2)[0]
		got := OCRFor(volume, book)
		if !strings.HasPrefix(got, OCR()) {
			t.Errorf("%s: the shared prompt is not the head of the volume prompt", book)
		}
		for _, want := range []string{
			"page number printed at the foot as the last line",
			"Where the page prints no number at the foot, write nothing",
			"no text layer",
		} {
			if !strings.Contains(got, want) {
				t.Errorf("%s: the foot note does not say %q", book, want)
			}
		}
		if OCRForSHA256(volume, book) == OCRSHA256() {
			t.Errorf("%s: the note did not change the hash, so nothing would be re-read", book)
		}
	}
	if OCRFor("ac-i-vii", "ac") != OCRFor("top-v-x", "top") {
		t.Error("the two foot-numbered volumes are read with different prompts")
	}
}

// ens-i-iv is foot-number too and is deliberately left on its own note. It has a
// text layer, so pdftotext reads the foot off that and the reading never has to,
// and moving its hash would put 416 read pages back in the queue for nothing.
func TestTheFootNoteIsKeptOffTheVolumeThatHasATextLayer(t *testing.T) {
	if strings.Contains(OCRFor("ens-i-iv", "ens"), "no text layer") {
		t.Error("ens-i-iv picked up the foot note and its pages would all go stale")
	}
	if OCRFor("ens-i-iv", "ens") == OCRFor("ac-i-vii", "ac") {
		t.Error("ens-i-iv is being read with the foot note rather than its own")
	}
}
