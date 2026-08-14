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

// One number for the whole corpus was enough while the corpus was one volume.
// The moment a second went in it stopped being enough, because a total lets a
// volume that got worse hide behind a volume that got better, and a formula
// that used to set and no longer does is the only thing this gate exists to
// catch.
func TestAVolumeThatGotWorseCannotHideBehindOneThatGotBetter(t *testing.T) {
	// Thirty-one and twenty are what the two Algebra volumes carried on the day
	// the Lie volume went in, and sixty-eight is what the Lie volume brought
	// with it. Six of the Algebra ones are repaired and six of the Lie ones are
	// broken, which a total of 119 has nothing to say about.
	c, err := parseCeilings("en/alg=31,fr/alg=20,en/lie=68")
	if err != nil {
		t.Fatal(err)
	}
	moved := map[string]int{"en/alg": 25, "fr/alg": 20, "en/lie": 74}
	err = c.check(moved)
	if err == nil {
		t.Fatal("a volume six formulae worse than its ceiling was let through")
	}
	if !strings.Contains(err.Error(), "en/lie has 74, over its ceiling of 68") {
		t.Errorf("did not say which volume or by how much: %v", err)
	}
	if strings.Contains(err.Error(), "en/alg") {
		t.Errorf("named a volume that came down rather than went up: %v", err)
	}
}

// A volume nobody has measured is a volume this is not watching, and it would
// otherwise go in without a word under a rule whose whole promise is that
// nothing does.
func TestAVolumeWithNoCeilingOfItsOwnIsRefused(t *testing.T) {
	c, err := parseCeilings("en/alg=31")
	if err != nil {
		t.Fatal(err)
	}
	err = c.check(map[string]int{"en/alg": 31, "en/lie": 1})
	if err == nil {
		t.Fatal("a volume with no ceiling was let through")
	}
	if !strings.Contains(err.Error(), "en/lie has 1 and no ceiling of its own") {
		t.Errorf("did not name the volume nobody has measured: %v", err)
	}
}

// A volume at its ceiling passes. The ceiling is the most it may carry and not
// the most it may carry less one, and a run that clears the last formula of a
// volume still has to pass so that it can say the ceiling can come down.
func TestAVolumeAtItsCeilingPasses(t *testing.T) {
	c, err := parseCeilings("en/alg=31,en/lie=68")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.check(map[string]int{"en/alg": 31, "en/lie": 0}); err != nil {
		t.Fatalf("a volume at its ceiling and a volume clear of it failed: %v", err)
	}
}

// A bare number is what the flag used to take and what a hand run still says,
// so it goes on meaning the whole corpus at once.
func TestABareNumberIsStillTheWholeCorpus(t *testing.T) {
	c, err := parseCeilings("51")
	if err != nil {
		t.Fatal(err)
	}
	if err := c.check(map[string]int{"en/alg": 31, "fr/alg": 20}); err != nil {
		t.Fatalf("fifty-one under a ceiling of fifty-one failed: %v", err)
	}
	if err := c.check(map[string]int{"en/alg": 31, "fr/alg": 21}); err == nil {
		t.Fatal("fifty-two under a ceiling of fifty-one passed")
	}
}

func TestAVolumeIsNamedTheWayItsFilesAre(t *testing.T) {
	for file, want := range map[string]string{
		"content/en/lie/VIII/06_s6_modules_over_a_split_semi_simple_lie.md": "en/lie",
		"content/fr/alg/VIII/exercises/s2/07.md":                            "fr/alg",
		"tags/00QT.md":                                                      "tags/00QT.md",
	} {
		if got := volumeOf(file); got != want {
			t.Errorf("volumeOf(%q) = %q, want %q", file, got, want)
		}
	}
}

func TestACeilingThatIsNeitherANumberNorAListIsRefused(t *testing.T) {
	for _, arg := range []string{"en/lie", "en/lie=", "en/lie=-1", "-3"} {
		if _, err := parseCeilings(arg); err == nil {
			t.Errorf("parseCeilings(%q) was accepted", arg)
		}
	}
}
