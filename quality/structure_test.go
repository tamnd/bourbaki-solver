package quality

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// content/en-mt holds English written by a model out of the French printing.
// The name of the tree is how the file was made, so lang is en in it, the same
// as under content/en, and method and translated_from are what tell the two
// apart.
func TestTheMachineEnglishTreeHoldsEnglish(t *testing.T) {
	d := Doc{
		Path: "content/en-mt/ts/I/00_frontmatter.md", Lang: "en-mt", Kind: KindSection,
		Section: &corpus.SectionFrontMatter{
			Book: "ts", Chapter: "I", Lang: "en", ContentSHA256: "0",
			TranslatedFrom: "content/fr/ts/I/00_frontmatter.md",
		},
	}
	if got := requireSection(d); len(got) != 0 {
		t.Errorf("reported %+v, want nothing", got)
	}
}

// The tree is still read: a French file that has wandered into it is a file in
// the wrong place whatever the tree is called.
func TestAFrenchFileUnderTheMachineEnglishTreeIsReported(t *testing.T) {
	d := Doc{
		Path: "content/en-mt/ts/I/00_frontmatter.md", Lang: "en-mt", Kind: KindSection,
		Section: &corpus.SectionFrontMatter{
			Book: "ts", Chapter: "I", Lang: "fr", ContentSHA256: "0",
		},
	}
	if got := requireSection(d); len(got) != 1 {
		t.Errorf("reported %+v, want the one file in the wrong tree", got)
	}
}
