package main

import (
	"os"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// brokenMathCorpus is the small corpus with a formula the text layer damaged in
// it, which is what the whole chapter is full of: a prime read as an underscore,
// so M' came out as M_' and KaTeX will not set it.
//
// The page is rewritten and the section assembled from it, rather than the
// section edited, because a section edited by hand is a different failure that
// the assembler's own check would catch first.
func brokenMathCorpus(t *testing.T) string {
	t.Helper()
	root := smallCorpus(t)
	f := corpus.PageFile{
		Meta: corpus.PageFrontMatter{Book: "alg-viii", PDFPage: 19, Method: corpus.MethodNative},
		Body: "**Proposition 1.** — Let $M_'$ be a module and $N_'$ another one.",
	}
	out, err := f.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corpus.PagePath(root, "alg-viii", 19), out, 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BOURBAKI_CORPUS", root)
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	return root
}

// The deploy passes no flag and gets no site, which is the point: a formula the
// corpus got wrong does not reach a reader dressed as mathematics.
func TestABrokenFormulaStopsAPlainBuild(t *testing.T) {
	brokenMathCorpus(t)
	err := runPublish([]string{"-check"})
	if err == nil {
		t.Fatal("a corpus with a formula KaTeX refuses built without complaint")
	}
	if !strings.Contains(err.Error(), "01_s1_artinian_modules_and_noetherian_modules.md") {
		t.Errorf("the failure does not name the file it is in: %v", err)
	}
}

// The ceiling is the pull request gate while the chapter still carries the
// damage the text layer did to it. It is a promise that the count does not go
// up, so it has to be a count and not a first sighting: a build that stopped at
// the first refusal would report 1 on a corpus that has two hundred.
func TestTheCeilingCountsEveryBrokenFormulaRatherThanStoppingAtTheFirst(t *testing.T) {
	brokenMathCorpus(t)
	if err := runPublish([]string{"-check", "-max-broken", "100"}); err != nil {
		t.Fatalf("a build under the ceiling failed: %v", err)
	}
	err := runPublish([]string{"-check", "-max-broken", "0"})
	if err == nil {
		t.Fatal("a build over the ceiling passed")
	}
	if !strings.Contains(err.Error(), "over the ceiling of 0") {
		t.Errorf("the failure does not say what the ceiling was: %v", err)
	}
	// Both of them, not the first of them. The number in that message is what
	// the next pull request is measured against.
	if !strings.Contains(err.Error(), "2 formulae") {
		t.Errorf("the failure counted something other than the two broken spans: %v", err)
	}
}

// And a ceiling of zero on a corpus that has nothing wrong with it passes,
// which is what the gate becomes on the day the repair is finished.
func TestACeilingOfZeroPassesOnACorpusWithNothingBroken(t *testing.T) {
	root := smallCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	if err := runPublish([]string{"-check", "-max-broken", "0"}); err != nil {
		t.Fatalf("a corpus with no broken formula failed the ceiling: %v", err)
	}
}
