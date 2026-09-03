package main

import "testing"

// The three volumes whose contents cannot be read off a contents page are read
// off their own pages instead, and bourbaki toc build has to leave those alone
// rather than count them as volumes it failed on. What tells it apart is the
// grammar the manifest records, so these are the strings the forty-four
// manifests actually carry.

func TestAContentsReadOffTheVolumesOwnPagesIsRecognisedByItsGrammar(t *testing.T) {
	for _, grammar := range []string{"body/bare", "body/label"} {
		if !readOffItsOwnPages(grammar) {
			t.Errorf("readOffItsOwnPages(%q) = false, want true", grammar)
		}
	}
}

func TestAContentsReadOffAContentsPageIsNotTakenForABodyReading(t *testing.T) {
	// Every other grammar in manifests/toc, one of each mark and each page
	// form. None of them is a body reading and none must be read as one, or a
	// volume with a real contents would stop being rebuilt the moment it first
	// failed.
	for _, grammar := range []string{"pilcrow/bare", "pilcrow/label", "column/bare"} {
		if readOffItsOwnPages(grammar) {
			t.Errorf("readOffItsOwnPages(%q) = true, want false", grammar)
		}
	}
}

func TestAManifestWithNoGrammarAtAllIsNotABodyReading(t *testing.T) {
	// A manifest written before the grammar was recorded, and the zero value a
	// missing entry hands back. Neither says the volume was read off its body,
	// and reading either as one would leave a volume unbuilt with nothing said.
	if readOffItsOwnPages("") {
		t.Error(`readOffItsOwnPages("") = true, want false`)
	}
}

func TestTheWordBodyOnItsOwnIsNotAGrammar(t *testing.T) {
	// The mark is half of a pair and the slash is part of the test, so that a
	// grammar that merely starts with the letters cannot pass for one. There is
	// no "bodyless" mark today; this is what stops one being invented into a
	// body reading by accident.
	for _, grammar := range []string{"body", "bodyless/bare"} {
		if readOffItsOwnPages(grammar) {
			t.Errorf("readOffItsOwnPages(%q) = true, want false", grammar)
		}
	}
}
