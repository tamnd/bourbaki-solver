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

// The front matter and the back matter are the two parts of the build with no
// other check watching them, which is the whole reason matter exists. A volume
// can lose its title page and pass every other check in the file: the chapters
// the manifest names are still there, the §§ still run without a gap, and four
// leaves are half of one per cent of the text length.
func TestTheFrontAndBackMatterAreBoundIntoAVolumeThisPackageWrote(t *testing.T) {
	v := sample()
	d, err := Write(v)
	if err != nil {
		t.Fatal(err)
	}
	a := &Audit{}
	a.matter(v, d)
	for _, c := range a.Checks {
		if !c.OK {
			t.Errorf("%s failed on a document this package just wrote: %s %v", c.Name, c.Detail, c.Notes)
		}
	}
}

// The check has to name the part that went, because "the front matter is not
// bound in" sends whoever reads it back through document.go to work out which
// of the five it means.
func TestTheFrontMatterCheckNamesThePartThatIsNotThere(t *testing.T) {
	v := sample()
	d, err := Write(v)
	if err != nil {
		t.Fatal(err)
	}
	d.TeX = strings.Replace(d.TeX, "\\btitlepage\n", "", 1)
	a := &Audit{}
	a.matter(v, d)
	c := find(t, a, "front matter is bound in")
	if c.OK {
		t.Error("a document with no title page passed the front matter check")
	}
	if !strings.Contains(strings.Join(c.Notes, " "), `\btitlepage`) {
		t.Errorf("the check does not name the part that went: %v", c.Notes)
	}
}

// The edition line is asked for only where the manifest carries one, so a
// printing that does not say which edition it is passes rather than failing for
// a line it has no text for.
func TestTheEditionLineIsOnlyAskedForWhereThePrintingHasOne(t *testing.T) {
	v := sample()
	d, err := Write(v)
	if err != nil {
		t.Fatal(err)
	}
	a := &Audit{}
	a.matter(v, d)
	if c := find(t, a, "front matter is bound in"); !c.OK {
		t.Errorf("a volume with no edition in its manifest failed: %s %v", c.Detail, c.Notes)
	}
	v.Meta.Edition = "Second edition"
	d, err = Write(v)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(d.TeX, `\bedition{Second edition}`) {
		t.Fatal("the writer did not set the edition line the manifest carries")
	}
	d.TeX = strings.Replace(d.TeX, `\bedition{Second edition}`, "", 1)
	a = &Audit{}
	a.matter(v, d)
	if c := find(t, a, "front matter is bound in"); c.OK {
		t.Error("a volume whose manifest names an edition passed without the edition line")
	}
}

// The back matter is conditional on what the volume holds and not on a fixed
// list. Some printings set the Historical Note as a Book of its own and some as
// an appendix to a chapter, and not every one carries both indexes, so a volume
// with none of the three has nothing to bind and passes.
func TestAVolumeWithNoIndexesAndNoHistoricalNoteHasNoBackMatterToBind(t *testing.T) {
	v := sample()
	v.Chapters[0].Historical = nil
	d, err := Write(v)
	if err != nil {
		t.Fatal(err)
	}
	a := &Audit{}
	a.matter(v, d)
	c := find(t, a, "back matter is bound in")
	if !c.OK {
		t.Errorf("a volume with nothing to bind at the back failed: %s %v", c.Detail, c.Notes)
	}
	if !strings.Contains(c.Detail, "0 of 0") {
		t.Errorf("detail = %q, want it to say there was nothing to bind", c.Detail)
	}
}

// An index that is dropped on the way out is the fault this half of the check
// is for. The two indexes are the last thing in the document, so a writer that
// stops early loses them and leaves everything above them intact.
func TestTheBackMatterCheckFindsAnIndexThatDidNotReachTheDocument(t *testing.T) {
	v := sample()
	v.Notation = &Section{
		Kind: corpus.KindNotation, Title: "Index of Notation",
		Body: "$x$, I, § 1, no. 1\n", Path: "content/en/alg/I/notation.md", Head: 1, Lang: "en",
	}
	d, err := Write(v)
	if err != nil {
		t.Fatal(err)
	}
	a := &Audit{}
	a.matter(v, d)
	if c := find(t, a, "back matter is bound in"); !c.OK {
		t.Fatalf("a volume whose index was written failed: %s %v", c.Detail, c.Notes)
	}
	d.TeX = strings.Replace(d.TeX, "{notation}", "{dropped}", 1)
	a = &Audit{}
	a.matter(v, d)
	c := find(t, a, "back matter is bound in")
	if c.OK {
		t.Error("a document with no index of notation in it passed the back matter check")
	}
	if !strings.Contains(strings.Join(c.Notes, " "), "notation") {
		t.Errorf("the check does not name the index that went: %v", c.Notes)
	}
}

// TestAConditionListWhoseGreekWasReadAsLatinIsFound is the finding that started
// this check. The English of ac III, § 3, exercise 23 came off the scan with
// its first two labels lost, and the French it was set from reads α) β) γ).
func TestAConditionListWhoseGreekWasReadAsLatinIsFound(t *testing.T) {
	body := "The following are equivalent:\n\n(a) every ideal is principal;\n(β) every ideal is free;\n(y) the ring is a field.\n"
	got := misreadLabels("x.md", body)
	if len(got) != 1 {
		t.Fatalf("misreadLabels found %d lists, want 1: %v", len(got), got)
	}
	if !strings.Contains(got[0], "(α) (β) (γ)") {
		t.Errorf("the finding does not say what the list should read: %s", got[0])
	}
}

// TestPartsOfAnExerciseAroundAGreekListAreNotAFinding is the ordinary shape and
// is far commoner than the misreadings. An exercise numbers its parts (a), (b),
// (c) in Latin and its conditions α, β, γ in Greek, so the two alphabets sit
// next to each other everywhere and a check that only looked for that would
// fail most of the library. Mapping (a) to α leaves α α β, which repeats, and a
// list of conditions does not repeat.
func TestPartsOfAnExerciseAroundAGreekListAreNotAFinding(t *testing.T) {
	body := "(a) Show that the following are equivalent:\n(α) the ring is local;\n(β) the ring is a field.\n\n(b) Deduce that the ring is Noetherian.\n"
	if got := misreadLabels("x.md", body); len(got) != 0 {
		t.Errorf("the ordinary shape of an exercise was reported as a finding: %v", got)
	}
}

// TestATrailingLatinPartAfterAGreekListIsNotAFinding is alg I, § 2, exercise
// 17, where two properties α and β are stated and then part (a) opens. Mapped,
// that is α β α, which goes backwards, so it is not a list of conditions.
func TestATrailingLatinPartAfterAGreekListIsNotAFinding(t *testing.T) {
	body := "(α) for all s in S there exist t and b;\n(β) for all a, b in E there exists t;\n(a) In E times S let the relation denote this.\n"
	if got := misreadLabels("x.md", body); len(got) != 0 {
		t.Errorf("a Latin part after a Greek list was reported as a finding: %v", got)
	}
}

// TestTheDigitEightIsReadAsDelta is ac II, § 4, exercise 18, whose fourth
// condition came off the scan as "(8) The ring C(Y; R) is absolutely flat".
func TestTheDigitEightIsReadAsDelta(t *testing.T) {
	body := "(α) every prime ideal is maximal;\n(β) every countable intersection is open;\n(y) every function is locally constant;\n(8) the ring is absolutely flat.\n"
	got := misreadLabels("x.md", body)
	if len(got) != 1 {
		t.Fatalf("misreadLabels found %d lists, want 1: %v", len(got), got)
	}
	if !strings.Contains(got[0], "(α) (β) (γ) (δ)") {
		t.Errorf("the digit 8 was not read back as delta: %s", got[0])
	}
}

// TestAListThatIsAllGreekOrAllLatinIsNotAFinding: the check needs one of each
// to have anything to say, since a list that is wholly in either alphabet is
// either right or wrong in a way this cannot see.
func TestAListThatIsAllGreekOrAllLatinIsNotAFinding(t *testing.T) {
	for _, body := range []string{
		"(α) the first;\n(β) the second;\n(γ) the third.\n",
		"(a) the first;\n(b) the second.\n",
	} {
		if got := misreadLabels("x.md", body); len(got) != 0 {
			t.Errorf("a list in one alphabet was reported as a finding: %v", got)
		}
	}
}

// TestAGapInTheRunIsNotAListOfConditions: two labels that do not follow each
// other are not the list this check is about, and reporting them would make it
// fire on prose that happens to open two lines with brackets.
func TestAGapInTheRunIsNotAListOfConditions(t *testing.T) {
	body := "(a) the first;\n(γ) the third.\n"
	if got := misreadLabels("x.md", body); len(got) != 0 {
		t.Errorf("a run that skips a letter was reported as a finding: %v", got)
	}
}

// TestTheGreekLabelCheckReadsTheExercisesAndNotOnlyTheSections is the mistake
// the first cut of this check made. Volume.Pieces returns the §§, the chapter
// fronts, the historical notes and the two indexes, and it does not return the
// exercises, which hang off each § instead. Every one of the fifteen misread
// lists in the corpus was in an exercise, so a check that walked Pieces alone
// passed the entire library while ac IV, § 2, exercise 10 still read
// (a) (b) (y) (δ). It was found by putting that file back as it was and
// watching the audit stay green.
func TestTheGreekLabelCheckReadsTheExercisesAndNotOnlyTheSections(t *testing.T) {
	v := sample()
	s := v.Chapters[0].Sections[0]
	s.Exercises = append(s.Exercises, &Exercise{
		Number: 1,
		Path:   "ex/1.md",
		Body:   "Show that the following are equivalent:\n\n(a) the ring is local;\n(β) the ring is a field.\n",
	})
	a := &Audit{}
	a.structure(v)
	c := find(t, a, "Greek label")
	if c.OK {
		t.Fatalf("the check passed a volume whose exercise reads (a) (β): %s", c.Detail)
	}
	if len(c.Notes) != 1 || !strings.Contains(c.Notes[0], "ex/1.md") {
		t.Errorf("the finding does not name the exercise it is in: %v", c.Notes)
	}
}
