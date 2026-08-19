package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/quality"
	"github.com/tamnd/bourbaki-solver/roundtrip"
)

// roundTripCorpus is two English sections and their Vietnamese, plus one
// Chinese section, built in memory. quality.Corpus is what the audit rules read
// and Pairs is how they pair a translation with its source, so a test that
// built the pairs its own way would be testing a pairing no rule makes.
func roundTripCorpus() *quality.Corpus {
	en1 := quality.Doc{Path: "content/en/alg/VIII/01_s1.md", Lang: "en", Kind: quality.KindSection,
		Body: "A ring here.", Section: &corpus.SectionFrontMatter{Chapter: "VIII"}}
	en2 := quality.Doc{Path: "content/en/ens/I/02_s2.md", Lang: "en", Kind: quality.KindSection,
		Body: "A set here.", Section: &corpus.SectionFrontMatter{Chapter: "I"}}
	vi1 := quality.Doc{Path: "content/vi/alg/VIII/01_s1.md", Lang: "vi", Kind: quality.KindSection,
		Body: "Mot vanh o day.", Section: &corpus.SectionFrontMatter{
			Chapter: "VIII", TranslatedFrom: en1.Path}}
	vi2 := quality.Doc{Path: "content/vi/ens/I/02_s2.md", Lang: "vi", Kind: quality.KindSection,
		Body: "Mot tap hop o day.", Section: &corpus.SectionFrontMatter{
			Chapter: "I", TranslatedFrom: en2.Path}}
	zh1 := quality.Doc{Path: "content/zh/alg/VIII/01_s1.md", Lang: "zh", Kind: quality.KindSection,
		Body: "huan.", Section: &corpus.SectionFrontMatter{
			Chapter: "VIII", TranslatedFrom: en1.Path}}
	return &quality.Corpus{
		Docs:    []quality.Doc{en1, en2, vi1, vi2, zh1},
		Sources: []quality.Doc{en1, en2},
	}
}

func TestTheSampleIsDrawnOverTheTranslationsTheRulesPair(t *testing.T) {
	got, _ := roundTripItems(roundTripCorpus(), "", "", "", "")
	if len(got) != 3 {
		t.Fatalf("%d translations, want the two Vietnamese and the one Chinese", len(got))
	}
	for _, it := range got {
		if it.Lang == "en" {
			t.Errorf("%s is English and is in the list", it.Path)
		}
		if it.English == "" {
			t.Errorf("%s names no English source", it.Path)
		}
		if it.Digest == "" {
			t.Errorf("%s carries no digest, so nothing could tell a stale verdict from a current one", it.Path)
		}
	}
}

func TestTheFlagsNarrowTheSample(t *testing.T) {
	c := roundTripCorpus()
	for _, tc := range []struct {
		name                      string
		lang, book, chapter, file string
		want                      int
	}{
		{name: "language", lang: "vi", want: 2},
		{name: "book", book: "ens", want: 1},
		{name: "chapter", chapter: "VIII", want: 2},
		{name: "file", file: "content/zh/alg/VIII/01_s1.md", want: 1},
		{name: "language and book", lang: "vi", book: "alg", want: 1},
		{name: "a book nobody has", book: "top", want: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			if got, _ := roundTripItems(c, tc.lang, tc.book, tc.chapter, tc.file); len(got) != tc.want {
				t.Errorf("got %d, want %d", len(got), tc.want)
			}
		})
	}
}

func TestBothHalvesAreReadBeforeAnyHostIsAsked(t *testing.T) {
	// Finding out an hour in, one file at a time, that the sample names a file
	// the corpus does not hold is finding it out too late.
	c := roundTripCorpus()
	items, _ := roundTripItems(c, "vi", "", "", "")
	bodies, err := roundTripBodies(c, items)
	if err != nil {
		t.Fatal(err)
	}
	if len(bodies) != 2 {
		t.Fatalf("read %d files, want 2", len(bodies))
	}
	b := bodies["content/vi/alg/VIII/01_s1.md"]
	if b.translation == "" || b.english == "" {
		t.Errorf("one half is missing: %+v", b)
	}
}

func TestASampleNamingAFileTheCorpusDoesNotHoldStopsTheRun(t *testing.T) {
	c := roundTripCorpus()
	_, err := roundTripBodies(c, []roundtrip.Item{
		{Path: "content/vi/alg/VIII/01_s1.md", English: "content/en/alg/VIII/nothing.md", Lang: "vi"},
	})
	if err == nil {
		t.Fatal("a missing English source did not stop the run")
	}
	if !strings.Contains(err.Error(), "nothing.md") {
		t.Errorf("the error does not name the file: %v", err)
	}
}

func TestTheReportPutsTheTwoPassagesInFrontOfAReader(t *testing.T) {
	root := t.TempDir()
	sample := []roundtrip.Item{{Lang: "vi", Path: "content/vi/alg/VIII/01_s1.md", Digest: "d1"}}
	res := &roundtrip.Results{Rate: roundtrip.Rate}
	res.Put(roundtrip.Verdict{
		Lang: "vi", Path: "content/vi/alg/VIII/01_s1.md", English: "content/en/alg/VIII/01_s1.md",
		Digest: "d1", BackModel: "gpt-5-6-thinking", Same: false,
		Differences: []roundtrip.Difference{{
			Kind: roundtrip.KindHypothesis, English: "Let A be a\ncommutative ring",
			Back: "Let A be a ring", Why: "commutative is gone"}},
	})
	if err := writeRoundTripReport(root, "", false, sample, 40, res); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "reports", "roundtrip.md"))
	if err != nil {
		t.Fatal(err)
	}
	text := string(b)
	for _, want := range []string{
		"content/vi/alg/VIII/01_s1.md",
		"hypothesis",
		"commutative is gone",
		"> Let A be a commutative ring", // folded onto one line, or it renders as two quotes
		"> Let A be a ring",
		"40 translations, 1 sampled at 5 per cent",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("the report does not carry %q\n%s", want, text)
		}
	}
}

func TestTheReportSaysWhatNothingFoundMeans(t *testing.T) {
	root := t.TempDir()
	sample := []roundtrip.Item{{Lang: "vi", Path: "a.md", Digest: "d1"}}
	res := &roundtrip.Results{Rate: roundtrip.Rate}
	res.Put(roundtrip.Verdict{Lang: "vi", Path: "a.md", Digest: "d1", Same: true})
	if err := writeRoundTripReport(root, "", false, sample, 40, res); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "reports", "roundtrip.md"))
	if err != nil {
		t.Fatal(err)
	}
	// A clean sample is a fact about the draw and the day, not about the corpus,
	// and the report has to say so where somebody reads it.
	if !strings.Contains(string(b), "on the files the draw picked, on the day it ran") {
		t.Errorf("a clean report overclaims:\n%s", b)
	}
}

func TestNoReportWritesNothing(t *testing.T) {
	root := t.TempDir()
	if err := writeRoundTripReport(root, "", true, nil, 0, &roundtrip.Results{}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "reports", "roundtrip.md")); !os.IsNotExist(err) {
		t.Error("-no-report wrote a report")
	}
}

func TestAnAskIdCanBeADirectoryName(t *testing.T) {
	// The id becomes a directory on the host, so a slash in it makes a tree
	// nobody asked for.
	got := roundTripID("content/vi/alg/VIII/01_s1.md")
	if strings.ContainsAny(got, "/ ") {
		t.Errorf("%q cannot be a directory name", got)
	}
	if !strings.Contains(got, "01_s1") {
		t.Errorf("%q does not say which file it came from", got)
	}
}

func TestATranslationMadeFromFrenchIsLeftOutAndCounted(t *testing.T) {
	// content/en-mt is the French volumes read into English. Asking for it to be
	// put back into English and then comparing the two would be comparing a text
	// with a paraphrase of itself: every verdict would come back the same and
	// none of them would mean anything.
	c := roundTripCorpus()
	fr := quality.Doc{Path: "content/fr/ts/I/01_s1.md", Lang: "fr", Kind: quality.KindSection,
		Body: "Un anneau ici.", Section: &corpus.SectionFrontMatter{Chapter: "I"}}
	mt := quality.Doc{Path: "content/en-mt/ts/I/01_s1.md", Lang: "en-mt", Kind: quality.KindSection,
		Body: "A ring here.", Section: &corpus.SectionFrontMatter{
			Chapter: "I", TranslatedFrom: fr.Path}}
	c.Docs = append(c.Docs, fr, mt)
	c.Sources = append(c.Sources, fr)

	got, left := roundTripItems(c, "", "", "", "")
	if left != 1 {
		t.Errorf("%d translations reported as left out, want 1", left)
	}
	for _, it := range got {
		if it.Lang == "en-mt" {
			t.Errorf("%s went into the sample", it.Path)
		}
	}
	// Asking for it by name must not get round the rule either, or the one
	// invocation somebody reaches for when they want to check a file is the one
	// that measures nothing.
	if got, _ := roundTripItems(c, "en-mt", "", "", ""); len(got) != 0 {
		t.Errorf("-lang en-mt drew %d files", len(got))
	}
}
