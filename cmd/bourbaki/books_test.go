package main

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// Three volumes of the library carry no text at all: Commutative Algebra, the
// second half of General Topology, and Algèbre chapter X. They are scans like
// the other scans and they are a different order of work, because a scan with
// somebody else's OCR on it still prints a legible running head and can be
// paged before it is read.
func TestTextLayer(t *testing.T) {
	for _, c := range []struct {
		name    string
		nature  pdfsrc.Nature
		perPage int
		want    string
	}{
		{"born digital", pdfsrc.NatureBornDigital, 1900, "native"},
		{"born digital with a run of plates", pdfsrc.NatureBornDigital, 0, "native"},
		{"scan somebody has read", pdfsrc.NatureScanned, 1800, "ocr"},
		{"scan with nothing on it", pdfsrc.NatureScanned, 0, "none"},
		{"scan with a stray page number", pdfsrc.NatureScanned, 12, "none"},
		{"just over the line", pdfsrc.NatureScanned, 200, "ocr"},
	} {
		s := pdfsrc.TextSample{Pages: 5, Chars: c.perPage * 5}
		if got := textLayer(c.nature, s); got != c.want {
			t.Errorf("%s: textLayer = %q, want %q", c.name, got, c.want)
		}
	}
}

// Adding a volume again to give it a language must not undo its page map.
func TestKeepLearnedKeepsWhatOnlyPagemapKnows(t *testing.T) {
	old := corpus.Book{ID: "alg-viii", Pages: 505, Grammar: "head-label", Pagination: "per-chapter"}
	fresh := corpus.Book{ID: "alg-viii", Lang: "en", Pages: 505}
	got := keepLearned(fresh, old)
	if got.Grammar != "head-label" || got.Pagination != "per-chapter" {
		t.Errorf("got %+v, want the grammar and pagination carried over", got)
	}
	if got.Lang != "en" {
		t.Errorf("Lang = %q, want the fresh probe to win where it knows better", got.Lang)
	}
}
