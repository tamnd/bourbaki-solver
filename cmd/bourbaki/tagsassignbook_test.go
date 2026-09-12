package main

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// TestInVolume is the narrowing tags assign -book does: a volume is a printing
// of some chapters of a Book, and everything else in content/ belongs to
// another volume whose pages have not been argued about yet.
func TestInVolume(t *testing.T) {
	v := &corpus.Book{
		ID:       "ens-i-iv",
		Book:     "ens",
		Lang:     "en",
		Chapters: []string{"I", "II", "III", "IV"},
		Fascicules: []corpus.Fascicule{
			{Numeral: "ER", Title: "SUMMARY OF RESULTS"},
		},
	}
	cases := []struct {
		path string
		want bool
		why  string
	}{
		{"content/en/ens/I/01_s1_terms.md", true, "a chapter of the volume"},
		{"content/en/ens/IV/02_s2_structures.md", true, "the last chapter of it"},
		{"content/en/ens/ER/02_s2_functions.md", true, "the fascicule bound into it"},
		{"content/en/ens/I/exercises/s1/003.md", true, "an exercise printed at the foot of one"},
		{"content/fr/ens/I/01_s1_termes.md", false, "the other printing is its own volume"},
		{"content/vi/ens/I/01_s1_terms.md", false, "a translation is not a printing"},
		{"content/en/alg/I/08_s8_rings.md", false, "another Book"},
		{"content/en/ens/V/01.md", false, "a chapter this volume does not print"},
		{"content/en/ens", false, "the Book directory names no chapter"},
	}
	for _, c := range cases {
		if got := inVolume(c.path, v); got != c.want {
			t.Errorf("inVolume(%q) = %v, want %v: %s", c.path, got, c.want, c.why)
		}
	}
}

// TestInVolumeChapterlessVolume is a volume that prints no chapters at all.
// Éléments d'histoire des mathématiques is one, and pagemap gives it the span
// "1", so that is what its chapter list holds and what content/ is under.
func TestInVolumeChapterlessVolume(t *testing.T) {
	v := &corpus.Book{ID: "hist-fr", Book: "hist", Lang: "fr", Chapters: []string{"1"}}
	if !inVolume("content/fr/hist/1/03_note.md", v) {
		t.Error("the one span of a volume with no chapters is where all of it is")
	}
	if inVolume("content/fr/hist/2/03_note.md", v) {
		t.Error("there is no second span")
	}
}
