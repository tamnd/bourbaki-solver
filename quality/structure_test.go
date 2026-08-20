package quality

import (
	"strings"
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

// s12Corpus is one chapter of one book: two § files that the manifest names and
// one Vietnamese translation of the first, which it does not.
func s12Corpus(named []corpus.SectionRecord) *Corpus {
	body := "the body of a section\n"
	sec := func(path string, from string) Doc {
		return Doc{Path: path, Lang: "en", Kind: KindSection, Body: body, head: 1,
			Section: &corpus.SectionFrontMatter{Book: "ens", Chapter: "IV",
				ContentSHA256: corpus.ContentSHA256(body), TranslatedFrom: from}}
	}
	return &Corpus{
		Docs: []Doc{
			sec("content/en/ens/IV/01_s1.md", ""),
			sec("content/en/ens/IV/02_s2.md", ""),
			sec("content/vi/ens/IV/01_s1.md", "content/en/ens/IV/01_s1.md"),
		},
		Sections: &corpus.SectionsManifest{Books: []corpus.BookSections{{
			ID:       "ens",
			Chapters: []corpus.ChapterSections{{Chapter: "IV", Sections: named}},
		}}},
	}
}

func s12Record(path string) corpus.SectionRecord {
	return corpus.SectionRecord{Kind: corpus.KindSection, Path: path,
		ContentSHA256: corpus.ContentSHA256("the body of a section\n")}
}

func TestS12PassesWhenTheManifestNamesEveryFileAndDescribesIt(t *testing.T) {
	c := s12Corpus([]corpus.SectionRecord{
		s12Record("content/en/ens/IV/01_s1.md"),
		s12Record("content/en/ens/IV/02_s2.md"),
	})
	got, err := s12(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %+v, want nothing", got)
	}
}

// This is the fault it was written for. assemble -partial skipped a chapter
// that was not read through and took its entries out with it, and every file of
// that chapter kept its text while nothing that reads the manifest could reach
// it.
func TestS12ReportsAFileTheManifestDoesNotName(t *testing.T) {
	c := s12Corpus([]corpus.SectionRecord{s12Record("content/en/ens/IV/01_s1.md")})
	got, err := s12(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("reported %+v, want the one file that fell out", got)
	}
	if got[0].File != "content/en/ens/IV/02_s2.md" {
		t.Errorf("named %q", got[0].File)
	}
	if !strings.Contains(got[0].Msg, "no entry in manifests/sections.yaml") {
		t.Errorf("said %q", got[0].Msg)
	}
}

func TestS12ReportsAManifestHashThatIsNotTheBody(t *testing.T) {
	stale := s12Record("content/en/ens/IV/02_s2.md")
	stale.ContentSHA256 = corpus.ContentSHA256("a body this file used to have\n")
	c := s12Corpus([]corpus.SectionRecord{s12Record("content/en/ens/IV/01_s1.md"), stale})
	got, err := s12(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("reported %+v, want the one stale hash", got)
	}
	if !strings.Contains(got[0].Msg, "holds content_sha256") {
		t.Errorf("said %q", got[0].Msg)
	}
}

func TestS12ReportsAnEntryWithNoFileUnderIt(t *testing.T) {
	c := s12Corpus([]corpus.SectionRecord{
		s12Record("content/en/ens/IV/01_s1.md"),
		s12Record("content/en/ens/IV/02_s2.md"),
		s12Record("content/en/ens/IV/03_s3.md"),
	})
	got, err := s12(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].File != "content/en/ens/IV/03_s3.md" {
		t.Fatalf("reported %+v, want the one entry with nothing under it", got)
	}
}

// A translation has no page under it, is not rewritten by an assemble, and is
// not the manifest's to name. The test is that it names a source, and not the
// language, since French is assembled too and is in the manifest.
func TestS12LeavesATranslationOutOfIt(t *testing.T) {
	c := s12Corpus([]corpus.SectionRecord{
		s12Record("content/en/ens/IV/01_s1.md"),
		s12Record("content/en/ens/IV/02_s2.md"),
	})
	got, err := s12(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		if strings.HasPrefix(f.File, "content/vi/") {
			t.Errorf("the Vietnamese was reported: %+v", f)
		}
	}
}
