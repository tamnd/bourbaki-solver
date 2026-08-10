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

func TestOCRPromptEndsWithOneNewline(t *testing.T) {
	text := OCR()
	if !strings.HasSuffix(text, "\n") || strings.HasSuffix(text, "\n\n") {
		t.Fatalf("the prompt should end with exactly one newline, got %q", text[len(text)-3:])
	}
}
