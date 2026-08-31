package book

import (
	"strings"
	"testing"
)

// A Greek word quoted as Greek and a Greek letter used as a symbol are the
// same characters and want opposite treatment, and the whole rule that tells
// them apart is how many of them stand next to each other. That is cheap
// enough to be worth stating and cheap enough to get wrong, so it is written
// down here with the two sentences out of the history volume that it was read
// off.
func TestGreekWordsAreSetAsWords(t *testing.T) {
	// Euclid's fourth common notion, in the historical note on Haar measure.
	// The first letter is an epsilon with a smooth breathing and Latin Modern
	// has no such character, so before this rule the word came out as one
	// letter in the fallback face followed by ten inline formulas.
	got := Text("coincident (ἐφαρμοζονται) sont")
	want := `coincident (\bgreek{ἐφαρμοζονται}) sont`
	if got != want {
		t.Errorf("Euclid:\n got %q\nwant %q", got, want)
	}

	// Pappus, in the historical note on polynomials. Two words and a space, so
	// two groups: the space between them is ordinary text and belongs in the
	// text face.
	got = Text("sous le nom de « τὸποι γραμμικοί ».")
	if !strings.Contains(got, `\bgreek{τὸποι}`) || !strings.Contains(got, `\bgreek{γραμμικοί}`) {
		t.Errorf("Pappus: got %q", got)
	}
	if strings.Contains(got, `$\tau$`) {
		t.Errorf("Pappus: a letter of the word was set as mathematics: %q", got)
	}
}

// One Greek letter on its own in a French sentence is the name of something in
// the mathematics and has to keep going through the symbol table, or every
// "soit epsilon > 0" in the corpus turns into a word in the fallback face.
func TestOneGreekLetterIsStillASymbol(t *testing.T) {
	got := Text("pour tout ε > 0 et tout λ dans K")
	want := `pour tout $\varepsilon$ > 0 et tout $\lambda$ dans K`
	if got != want {
		t.Errorf("\n got %q\nwant %q", got, want)
	}
}

// Missing is what the audit prints, and it prints things for somebody to open
// a page and look at. A Greek word set in the fallback face on purpose is not
// one of those, and reporting it is how a check that means something turns
// into a check people learn to skip.
func TestGreekWordsAreNotReportedAsMissing(t *testing.T) {
	if runes := Missing("(ἐφαρμοζονται)"); len(runes) != 0 {
		t.Errorf("a Greek quotation was reported missing: %q", string(runes))
	}
	// The Devanagari of the Rhind papyrus still is, because nobody has read it.
	if runes := Missing("तथ्य"); len(runes) == 0 {
		t.Error("the Devanagari stopped being reported")
	}
}
