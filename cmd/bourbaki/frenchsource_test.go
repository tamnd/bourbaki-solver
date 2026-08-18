package main

import "testing"

// A file read from the English says nothing about where it came from, because
// that is what every translation in the corpus already is and thousands of heads
// on disk would otherwise have to change. A file read from the French says so,
// because it is a model's reading of a volume Bourbaki never printed in English.
func TestOnlyTheFrenchReadingRecordsItsProvenance(t *testing.T) {
	if lang, method := provenance("en"); lang != "" || method != "" {
		t.Fatalf("english source recorded %q %q, want nothing", lang, method)
	}
	lang, method := provenance("fr")
	if lang != "fr" || method != "machine" {
		t.Fatalf("french source recorded %q %q, want fr machine", lang, method)
	}
}
