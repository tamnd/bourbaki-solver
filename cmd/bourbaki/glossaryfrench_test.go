package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/glossary"
)

// The two trees name a section after its own title, so the pairing is on the
// numbering in front of the title and on nothing else.
func TestASectionPairsWithItsFrenchPrintingWhateverItIsCalled(t *testing.T) {
	root := t.TempDir()
	writePair(t, root, "content/en/alg/VIII/03_s3_simple_modules.md", "the English of section 3")
	writePair(t, root, "content/fr/alg/VIII/03_s3_modules_simples.md", "le francais du paragraphe 3")
	writePair(t, root, "content/en/alg/VIII/04_s4_semisimple_modules.md", "no french for this one")
	pairs, err := alignedPairs(root, "", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(pairs) != 1 {
		t.Fatalf("paired %d sections, want the one that is in both", len(pairs))
	}
	if !strings.Contains(pairs[0].fr, "paragraphe 3") {
		t.Fatalf("paired with %q", pairs[0].fr)
	}
}

// A term is asked about once. The second question would be the same word out of
// a different sentence, and the questions are what the run costs.
func TestATermIsAskedAboutOnceHoweverOftenItIsSaid(t *testing.T) {
	g := &glossary.Glossary{Terms: []glossary.Term{{EN: "ring"}, {EN: "module", FR: "module"}}}
	pairs := []pair{{
		en:     "a ring and a module\n\nanother ring here",
		fr:     "un anneau et un module\n\nencore un anneau",
		source: "content/en/alg/VIII/03_s3.md",
	}}
	batches := alignedBatches(g, pairs, 12)
	if len(batches) != 1 {
		t.Fatalf("built %d questions, want one: %+v", len(batches), batches)
	}
	if len(batches[0].Terms) != 1 || batches[0].Terms[0] != "ring" {
		t.Fatalf("asked about %v, want ring alone, since module already has its French", batches[0].Terms)
	}
	if !strings.Contains(batches[0].FR, "anneau") {
		t.Fatalf("the question carries no French to read the answer out of: %q", batches[0].FR)
	}
}

// The whole point of asking this way is that the answer is in the book. A
// phrase that is not in the passage is the model writing French rather than
// reading it.
func TestAnAnswerThatIsNotInTheFrenchPassageIsRefused(t *testing.T) {
	b := glossary.Batch{Lang: "fr", Terms: []string{"ring", "module"},
		EN: "a ring and a module", FR: "un anneau et un module"}
	reply := b.Audit("1 | ring | anneau\n2 | module | corps\n")
	if len(reply.Rows) != 1 || reply.Rows[0].TR != "anneau" {
		t.Fatalf("kept %+v, want anneau alone", reply.Rows)
	}
	if len(reply.Rejects) != 1 || !strings.Contains(reply.Rejects[0].Reason, "not in the French passage") {
		t.Fatalf("refused %+v, want the one that is not in the passage", reply.Rejects)
	}
}

func writePair(t *testing.T, root, path, body string) {
	t.Helper()
	full := filepath.Join(root, filepath.FromSlash(path))
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	text := "---\nbook: alg\nlang: en\n---\n\n" + body + "\n"
	if err := os.WriteFile(full, []byte(text), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestTheFrenchIsFoundByItsMathematicsAfterTheCountSlips(t *testing.T) {
	// The French runs two paragraphs together early on, so from there down the
	// same paragraph is one lower than where counting would put it.
	en := []string{
		"A ring $A$ is simple when $\\mathfrak{m} = 0$.",
		"Let $E$ be a module over $A$.",
		"Every submodule of $E$ is a direct factor.",
		"The radical $\\mathfrak{r}(A)$ is nilpotent.",
	}
	fr := []string{
		"Un anneau $A$ est simple lorsque $\\mathfrak{m} = 0$.",
		"Soit $E$ un module sur $A$. Tout sous-module de $E$ est facteur direct.",
		"Le radical $\\mathfrak{r}(A)$ est nilpotent.",
	}
	at := align(en, fr)
	if at[0] != 0 {
		t.Errorf("the first paragraph went to %d, want 0", at[0])
	}
	if at[3] != 2 {
		t.Errorf("the paragraph after the slip went to %d, want 2", at[3])
	}
}

func TestAParagraphWithNoMathematicsIsLeftAlone(t *testing.T) {
	// Once the counts are far apart, a paragraph with no formula in it has
	// nothing to be found by, and a guess would be a question spent on a
	// passage that does not say what the English says.
	en := []string{"There is a converse, proved in chapter three.",
		"A second.", "A third.", "A fourth.", "A fifth.", "A sixth."}
	fr := []string{"Il y a une reciproque, demontree au chapitre trois."}
	if at := align(en, fr); at[0] != -1 {
		t.Errorf("a paragraph with nothing to anchor on went to %d, want -1", at[0])
	}
}

func TestOneBareLetterIsNotEnoughToAnchorOn(t *testing.T) {
	// The counts are far enough apart that counting is not trusted, so a
	// paragraph is placed by its mathematics or not at all. A single $n$ is in
	// half the paragraphs of the book and settles nothing.
	en := []string{"Let $n$ be an integer.", "Let $n$ be an integer again.",
		"A third.", "A fourth.", "A fifth.", "A sixth."}
	fr := []string{"Soit $n$ un entier."}
	at := align(en, fr)
	if at[0] != -1 || at[1] != -1 {
		t.Errorf("a single letter anchored: %v, want both -1", at)
	}
}

func TestCountingIsTrustedWhereTheTwoPrintingsAgreeOnTheCount(t *testing.T) {
	en := []string{"There is a converse.", "It is proved in chapter three."}
	fr := []string{"Il y a une reciproque.", "Elle est demontree au chapitre trois."}
	if at := align(en, fr); at[1] != 1 {
		t.Errorf("the second paragraph went to %d, want 1", at[1])
	}
}
