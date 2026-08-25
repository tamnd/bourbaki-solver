package book

import (
	"archive/zip"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
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
