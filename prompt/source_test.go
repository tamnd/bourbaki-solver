package prompt

import (
	"strings"
	"testing"
)

// Adding the source axis leaves every finished translation alone.
//
// prompt_sha256 is one of the four things that make a file stale, so a change
// to the wording of the common rules marks every section that prompt produced
// as due to be done again. Putting {{SOURCE}} where the file used to spell out
// English is a change to the wording, and taken naively it would have marked
// all 968 files stale for a change no model can even see.
//
// It does not, because the hash is taken after the substitution rather than
// before it, and an English source puts the same word back. These are the
// hashes as they stood before the placeholder existed, written out rather than
// computed so that a change to the common rules has to be faced here.
func TestAnEnglishSourceHashesToWhatItAlwaysDid(t *testing.T) {
	before := map[string]string{
		"vi": "83b0139ff219095b0a7a24ef29a6e4741b2ee7f67a6376a31a9f6aa03aebf5b5",
		"zh": "05b6c0db78fa3be910d42f79f149903e7b4e3a0b5bf2e55cfaa7ab2667486f27",
		"ja": "a7f0e149f6b2673dda466f0e3bd14f3acc73ce864d292d81df3cac748a448e5e",
	}
	for lang, want := range before {
		got, err := TranslateSHA256("en", lang)
		if err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		if got != want {
			t.Errorf("%s hashes to %s, and it used to hash to %s, so every %s file on disk just went stale",
				lang, got, want, lang)
		}
	}
}

// A French source is a different prompt and hashes differently.
//
// This is the other half of the same rule. The point of the axis is that the
// model is told which language is in front of it, so a passage read from the
// French has to carry a hash that says so, or a later run could not tell the
// two apart.
func TestAFrenchSourceHashesDifferently(t *testing.T) {
	fromEnglish, err := TranslateSHA256("en", "vi")
	if err != nil {
		t.Fatal(err)
	}
	fromFrench, err := TranslateSHA256("fr", "vi")
	if err != nil {
		t.Fatal(err)
	}
	if fromEnglish == fromFrench {
		t.Error("the source language does not reach the hash, so the two readings are indistinguishable")
	}
}

// The rules name the language actually in front of the model.
func TestTheSourceLanguageIsNamedInThePrompt(t *testing.T) {
	got, err := Translate("fr", "en", "anneau | ring\n", "", "Soit $A$ un anneau.")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(got, "{{SOURCE}}") {
		t.Error("the placeholder survived into the prompt")
	}
	if !strings.Contains(got, "wherever the French term appears") {
		t.Error("the glossary rule still asks for the English term")
	}
	if !strings.Contains(got, "Left is the French as the book spells it") {
		t.Error("the glossary heading still calls its left column English")
	}
	if !strings.Contains(got, "into English") {
		t.Error("the prompt does not say what to write")
	}
}
