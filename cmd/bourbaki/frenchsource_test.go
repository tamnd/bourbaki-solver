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

// English is not a column a glossary pass may fill, and it is a target here all
// the same, for the volumes that have no English printing. The first run of the
// French direction was refused by the flag check for exactly that reason.
func TestEnglishIsRefusedAsATargetOnlyWhenItIsNotFromTheFrench(t *testing.T) {
	if err := runTranslate([]string{"-from", "fr", "-lang", "vi", "-dry"}); err == nil {
		t.Error("reading the french into vietnamese was allowed, and it skips the english the glossary is built on")
	}
	if err := runTranslate([]string{"-lang", "en", "-dry"}); err == nil {
		t.Error("reading the english into english was allowed, and that is rewriting Springer")
	}
}
