package book

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// find returns the check with this name, which is how a test says which of the
// twenty one it means without depending on the order they run in.
func find(t *testing.T, a *Audit, name string) Check {
	t.Helper()
	for _, c := range a.Checks {
		if strings.Contains(c.Name, name) {
			return c
		}
	}
	t.Fatalf("no check named anything like %q; the audit ran %d of them", name, len(a.Checks))
	return Check{}
}

func TestStructurePassesOnAWholeVolume(t *testing.T) {
	a := &Audit{}
	a.structure(sample())
	for _, c := range a.Checks {
		if !c.OK {
			t.Errorf("%s failed on a volume with nothing wrong with it: %s %v", c.Name, c.Detail, c.Notes)
		}
	}
}

// A chapter missing from the middle of a volume is the fault this whole package
// exists to find. Nothing that reads one file at a time can see it: every file
// left is valid, every anchor resolves, and the book is simply missing a
// hundred pages.
func TestStructureFindsAMissingChapter(t *testing.T) {
	v := sample()
	v.Meta.Chapters = []string{"I", "II"}
	a := &Audit{}
	a.structure(v)
	c := find(t, a, "chapters the manifest names")
	if c.OK {
		t.Error("a volume built without chapter II passed the chapter check")
	}
	if !strings.Contains(strings.Join(c.Notes, " "), "II") {
		t.Errorf("the check does not name the missing chapter: %v", c.Notes)
	}
}

func TestStructureFindsAGapInTheSections(t *testing.T) {
	v := sample()
	first := v.Chapters[0].Sections[0]
	third := *first
	third.Number = 3
	third.Label = "alg-i-s3"
	third.Path = "content/en/alg/I/03_s3_something.md"
	v.Chapters[0].Sections = append(v.Chapters[0].Sections, &third)
	a := &Audit{}
	a.structure(v)
	if c := find(t, a, "§§ of every chapter"); c.OK {
		t.Error("a chapter that goes from § 1 to § 3 passed the gap check")
	}
}

// The subsection gap is what caught the Vietnamese Algebra shipping a § with
// two subsections both numbered 3, where the translator had put a number on a
// heading that the English leaves unnumbered.
func TestStructureFindsARepeatedSubsection(t *testing.T) {
	v := sample()
	s := v.Chapters[0].Sections[0]
	s.Body = strings.Replace(s.Body, "### 2. QUOTIENTS", "### 1. QUOTIENTS", 1)
	a := &Audit{}
	a.structure(v)
	c := find(t, a, "numbered subsections")
	if c.OK {
		t.Error("a § whose subsections go 1 then 1 passed the gap check")
	}
	if !strings.Contains(strings.Join(c.Notes, " "), s.Path) {
		t.Errorf("the check does not name the file: %v", c.Notes)
	}
}

// A translation that stopped part way leaves a file that is valid Markdown, has
// front matter that still claims the statements of the English it was made
// from, and is missing the last three of them. Counting the heads against the
// claim is what finds it.
func TestStructureFindsAStatementShortfall(t *testing.T) {
	v := sample()
	v.Chapters[0].Sections[0].Statements = 4
	a := &Audit{}
	a.structure(v)
	c := find(t, a, "every statement the front matter claims")
	if c.OK {
		t.Error("a § claiming four statements and setting one passed")
	}
	if !strings.Contains(strings.Join(c.Notes, " "), "claims 4") {
		t.Errorf("the check does not say what was claimed: %v", c.Notes)
	}
}

func TestPackedPassesOnAnEPUBThisPackageWrote(t *testing.T) {
	e, _ := writeSample(t)
	a := &Audit{}
	a.packed(e)
	for _, c := range a.Checks {
		if !c.OK {
			t.Errorf("%s failed on an EPUB this package wrote: %s %v", c.Name, c.Detail, c.Notes)
		}
	}
}

// This is the check that earned its keep the day it was written. epubcheck is
// not on this laptop, so the link check is written here, and the first thing it
// found was two documents pointing at ../style.css from a directory that has no
// parent inside the container.
func TestPackedFindsABrokenLink(t *testing.T) {
	e := breakSample(t, "EPUB/text/alg-i-s1.xhtml", "#alg-i-s1-prop-1", "#no-such-anchor")
	a := &Audit{}
	a.packed(e)
	c := find(t, a, "every link in the EPUB")
	if c.OK {
		t.Error("an EPUB with a link to an anchor nothing has passed the link check")
	}
}

func TestPackedFindsADocumentThatIsNotWellFormed(t *testing.T) {
	e := breakSample(t, "EPUB/text/alg-i-s1.xhtml", "</p>", "</p")
	a := &Audit{}
	a.packed(e)
	c := find(t, a, "well formed XML")
	if c.OK {
		t.Error("an EPUB with a document that does not parse passed")
	}
}

// breakSample writes the sample EPUB, unpacks it, does one replacement in one
// document and packs it again, so that a check can be shown failing on a
// container that is wrong in exactly one way.
func breakSample(t *testing.T, name, from, to string) *EPUB {
	t.Helper()
	dir := t.TempDir()
	good := filepath.Join(dir, "good.epub")
	e, err := WriteEPUB(good, sample(), Options{Epoch: 1735689600})
	if err != nil {
		t.Fatalf("WriteEPUB: %v", err)
	}
	in, err := zip.OpenReader(good)
	if err != nil {
		t.Fatalf("reopening: %v", err)
	}
	defer in.Close()

	bad := filepath.Join(dir, "bad.epub")
	f, err := os.Create(bad)
	if err != nil {
		t.Fatalf("creating: %v", err)
	}
	defer f.Close()
	w := zip.NewWriter(f)
	found := false
	for _, entry := range in.File {
		r, err := entry.Open()
		if err != nil {
			t.Fatalf("opening %s: %v", entry.Name, err)
		}
		body, err := io.ReadAll(r)
		r.Close()
		if err != nil {
			t.Fatalf("reading %s: %v", entry.Name, err)
		}
		s := string(body)
		if entry.Name == name {
			if !strings.Contains(s, from) {
				t.Fatalf("%s does not contain %q, so the test breaks nothing", name, from)
			}
			s = strings.Replace(s, from, to, 1)
			found = true
		}
		head := entry.FileHeader
		out, err := w.CreateHeader(&head)
		if err != nil {
			t.Fatalf("writing %s: %v", entry.Name, err)
		}
		if _, err := out.Write([]byte(s)); err != nil {
			t.Fatalf("writing %s: %v", entry.Name, err)
		}
	}
	if !found {
		t.Fatalf("the container has no %s", name)
	}
	if err := w.Close(); err != nil {
		t.Fatalf("closing: %v", err)
	}
	e.Path = bad
	return e
}

func TestWrittenHoldsTheCeilings(t *testing.T) {
	d := &Document{
		Stray: []Finding{{Where: "a.md:1", What: `\p`, Count: 2}},
		Wide:  []Finding{{Where: "b.md:2", What: "an array", Count: 1}},
	}
	// A ceiling of zero is what a corpus with nothing wrong with it would run
	// at, and both of these are wrong.
	a := &Audit{}
	a.written(d, AuditOptions{})
	if find(t, a, "TeX control sequence").OK {
		t.Error("a stray control sequence passed a ceiling of zero")
	}
	if find(t, a, "array had to be widened").OK {
		t.Error("a widened array passed a ceiling of zero")
	}
	// Raised to what is in the corpus today, both pass, which is what lets this
	// be a gate at all. A ceiling may be lowered and may never be raised.
	b := &Audit{}
	b.written(d, AuditOptions{Stray: 1, Wide: 1})
	if !find(t, b, "TeX control sequence").OK {
		t.Error("a stray control sequence failed a ceiling that covers it")
	}
	if !find(t, b, "array had to be widened").OK {
		t.Error("a widened array failed a ceiling that covers it")
	}
}

// The length check is the one that had to be rewritten, so it is the one worth
// pinning down. It used to compare the page count against the printing's, which
// measured the publisher's leading, and it now compares characters against
// pages/, which measures the book.

// writePrinting lays out a corpus root holding a reading of a printing whose
// text is n copies of the volume's own, which is how a test says "the same
// length" and "twice the length" without counting anything by hand.
func writePrinting(t *testing.T, id string, text string, pages int) string {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "pages", id)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for i := 1; i <= pages; i++ {
		name := filepath.Join(dir, fmt.Sprintf("%04d.md", i))
		page := "---\nbook: " + id + "\npdf_page: " + fmt.Sprint(i) + "\n---\n\n" + text
		if err := os.WriteFile(name, []byte(page), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestLengthPassesWhenTheVolumeHoldsThePrinting(t *testing.T) {
	v := sample()
	root := writePrinting(t, v.Meta.ID, strings.Repeat("x", v.Chars()/2), 2)
	a := &Audit{}
	a.length(root, v, DefaultAuditOptions())
	if c := find(t, a, "holds the text the printing has"); !c.OK {
		t.Errorf("a volume the same length as its printing failed: %s", c.Detail)
	}
}

func TestLengthFindsAVolumeThatIsShortOfThePrinting(t *testing.T) {
	v := sample()
	root := writePrinting(t, v.Meta.ID, strings.Repeat("x", v.Chars()), 2)
	a := &Audit{}
	a.length(root, v, DefaultAuditOptions())
	c := find(t, a, "holds the text the printing has")
	if c.OK {
		t.Errorf("a volume holding half its printing passed: %s", c.Detail)
	}
	if !strings.Contains(c.Detail, "50% of it") {
		t.Errorf("the detail should say how short it is, got %q", c.Detail)
	}
}

// A language the volume was never printed in holds what has been translated,
// and half a translation should be half the text. Calling that short would be
// calling the translation's progress a defect, which is the same reason the
// coverage check scales.
func TestLengthScalesAPartialTranslation(t *testing.T) {
	v := sample()
	root := writePrinting(t, v.Meta.ID, strings.Repeat("x", v.Chars()), 2)
	a := &Audit{Have: 1, Want: 2}
	a.length(root, v, DefaultAuditOptions())
	if c := find(t, a, "holds the text the printing has"); !c.OK {
		t.Errorf("half a translation of a printing failed: %s", c.Detail)
	}
}

// A volume the corpus has no reading of cannot be measured, and a check that
// cannot be measured passes and says why rather than failing on a fact about
// the repository.
func TestLengthPassesWhenThereIsNoReadingToCompareAgainst(t *testing.T) {
	a := &Audit{}
	a.length(t.TempDir(), sample(), DefaultAuditOptions())
	c := find(t, a, "holds the text the printing has")
	if !c.OK || !strings.Contains(c.Detail, "no reading") {
		t.Errorf("want a pass that says there is nothing to compare against, got %v %q", c.OK, c.Detail)
	}
}

// The markup is why the comparison is one-sided. content/ writes \varphi where
// the printing sets one letter, so a faithful volume runs over the printing and
// running over is not a defect.
func TestLengthDoesNotMindAVolumeLongerThanThePrinting(t *testing.T) {
	v := sample()
	root := writePrinting(t, v.Meta.ID, strings.Repeat("x", v.Chars()/4), 2)
	a := &Audit{}
	a.length(root, v, DefaultAuditOptions())
	if c := find(t, a, "holds the text the printing has"); !c.OK {
		t.Errorf("a volume twice the length of its printing failed: %s", c.Detail)
	}
}

// The sections manifest holds the printing and a translation is counted against
// it, so the two sides name the same § in two languages. Matching them by
// filename made a Vietnamese Algebra IX read as 8 sections of 30 when it had
// all thirty, because the file is the title slugged and the title is
// translated. The chapter and the § number are the same in every language.
func TestCoverageCountsATranslationWhoseFilesAreNamedInItsOwnLanguage(t *testing.T) {
	root := t.TempDir()
	m := &corpus.SectionsManifest{}
	m.Upsert(corpus.BookSections{ID: "test-i", Chapters: []corpus.ChapterSections{{
		Chapter: "I",
		Sections: []corpus.SectionRecord{
			{Kind: corpus.KindSection, Section: 1, Path: "content/en/alg/I/01_s1_groups.md"},
			{Kind: corpus.KindHistorical, Path: "content/en/alg/I/historical_note.md"},
		},
	}}})
	if err := m.Save(root); err != nil {
		t.Fatal(err)
	}
	v := sample()
	v.Lang = "vi"
	v.Chapters[0].Sections[0].Path = "content/vi/alg/I/01_s1_cac_nhom.md"
	v.Chapters[0].Historical.Path = "content/vi/alg/I/ghi_chu_lich_su.md"
	have, want, missing, err := Coverage(root, v)
	if err != nil {
		t.Fatal(err)
	}
	if want != 2 || have != 2 {
		t.Errorf("counted %d of %d, missing %v; both sections are there under translated names", have, want, missing)
	}
}

// A § the language really has not got is still missing, and the message names
// the file of the printing so there is something to go and look for.
func TestCoverageStillReportsASectionThatIsNotThere(t *testing.T) {
	root := t.TempDir()
	m := &corpus.SectionsManifest{}
	m.Upsert(corpus.BookSections{ID: "test-i", Chapters: []corpus.ChapterSections{{
		Chapter: "I",
		Sections: []corpus.SectionRecord{
			{Kind: corpus.KindSection, Section: 1, Path: "content/en/alg/I/01_s1_groups.md"},
			{Kind: corpus.KindSection, Section: 2, Path: "content/en/alg/I/02_s2_rings.md"},
			{Kind: corpus.KindHistorical, Path: "content/en/alg/I/historical_note.md"},
		},
	}}})
	if err := m.Save(root); err != nil {
		t.Fatal(err)
	}
	have, want, missing, err := Coverage(root, sample())
	if err != nil {
		t.Fatal(err)
	}
	if have != 2 || want != 3 {
		t.Fatalf("counted %d of %d, want 2 of 3", have, want)
	}
	if len(missing) != 1 || missing[0] != "content/en/alg/I/02_s2_rings.md" {
		t.Errorf("missing is %v, want the § 2 of the printing", missing)
	}
}
