package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// readCorpus is one § of one volume with its pages read and one import of it.
//
// Page 24 is the page § 2 starts on, and the last paragraph of § 1 is printed
// at the head of it above the § 2 heading. That is how the books are set and it
// is the case the sheet has to get right, so the fixture prints it that way
// rather than ending § 1 tidily at the foot of page 23.
func readCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	m := corpus.TOCManifest{Books: []corpus.BookTOC{{
		ID: "ens-i-iv",
		Chapters: []corpus.Chapter{{
			Book: "ens", Numeral: "I", Title: "DESCRIPTION OF FORMAL MATHEMATICS", PDFPage: 20,
			Sections: []corpus.Section{
				{Number: 1, Title: "Terms and relations", PDFPage: 22,
					Subsections: []corpus.Subsection{{Number: 1, Title: "Signs and assemblies", PDFPage: 22}}},
				{Number: 2, Title: "Criteria of substitution", PDFPage: 24},
			},
		}},
	}}}
	if err := m.Save(root); err != nil {
		t.Fatal(err)
	}

	if err := os.MkdirAll(corpus.PagesDir(root, "ens-i-iv"), 0o755); err != nil {
		t.Fatal(err)
	}
	pages := map[int]string{
		22: "### 1. SIGNS AND ASSEMBLIES\n\nA sign of the theory is one of the letters which have been written down in advance.\n",
		23: "An assembly of signs is a word formed from the signs of the theory in the manner just described.\n",
		24: "The last paragraph of the first section is printed above the heading of the second one.\n\n" +
			"### 2. CRITERIA OF SUBSTITUTION\n\nA criterion is a rule which allows a term to be put in place of a letter.\n",
	}
	for n, body := range pages {
		f := corpus.PageFile{Meta: corpus.PageFrontMatter{
			Book: "ens-i-iv", PDFPage: n, Method: corpus.MethodNative,
		}, Body: body}
		out, err := f.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(corpus.PagePath(root, "ens-i-iv", n), out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// writeImport puts one file in the import tree, head and all.
func writeImport(t *testing.T, root, name string, chapter, section int, intro bool, body string) string {
	t.Helper()
	rel := filepath.Join("imports", name, "chapter_1", "1."+string(rune('0'+section))+".md")
	if intro {
		rel = filepath.Join("imports", name, "chapter_1", "1.0.md")
	}
	path := filepath.Join(root, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	head := "---\nbook: ens-i-iv\nchapter: " + string(rune('0'+chapter)) +
		"\nsection: " + string(rune('0'+section)) + "\nlang: en\nextraction: share\n"
	if intro {
		head += "intro: true\n"
	}
	head += "---\n"
	if err := os.WriteFile(path, []byte(head+body), 0o644); err != nil {
		t.Fatal(err)
	}
	return rel
}

const faithful = "### 1. SIGNS AND ASSEMBLIES\n\n" +
	"A sign of the theory is one of the letters which have been written down in advance.\n\n" +
	"An assembly of signs is a word formed from the signs of the theory in the manner just described.\n\n" +
	"The last paragraph of the first section is printed above the heading of the second one.\n"

func TestShareReadWritesASheetForOneSection(t *testing.T) {
	root := readCorpus(t)
	rel := writeImport(t, root, "sets", 1, 1, false, faithful)
	if err := shareRead([]string{"-corpus", root, "-book", "ens-i-iv", "-file", rel}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "reports", "share-read-ens-i-iv-1.1.md"))
	if err != nil {
		t.Fatalf("the sheet should be written where the command says it is: %v", err)
	}
	sheet := string(b)
	for _, want := range []string{"## pdf page 22", "## pdf page 23", "## In the import and on no page of the section"} {
		if !strings.Contains(sheet, want) {
			t.Errorf("want %q on the sheet:\n%s", want, sheet)
		}
	}
	if strings.Contains(sheet, "## pdf page 24") {
		t.Errorf("page 24 is where § 2 starts and is not this §'s to answer for:\n%s", sheet)
	}
	if strings.Contains(sheet, "printed above the heading") {
		t.Errorf("the closing paragraph is printed on page 24 and must not read as invented:\n%s", sheet)
	}
}

func TestShareReadReportsASentenceTheBookDoesNotPrint(t *testing.T) {
	root := readCorpus(t)
	invented := "Every assembly of the theory may therefore be reduced to a normal form by the rule stated above."
	rel := writeImport(t, root, "sets", 1, 1, false, faithful+"\n"+invented+"\n")
	out := filepath.Join(root, "sheet.md")
	if err := shareRead([]string{"-corpus", root, "-book", "ens-i-iv", "-file", rel, "-out", out}); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), invented) {
		t.Fatalf("a sentence on no page of the § is the one thing this command exists to show:\n%s", b)
	}
}

func TestShareReadNamesTheFilesWhenNoneIsChosen(t *testing.T) {
	root := readCorpus(t)
	rel := writeImport(t, root, "sets", 1, 1, false, faithful)
	err := shareRead([]string{"-corpus", root, "-book", "ens-i-iv", "-import", "sets"})
	if err == nil {
		t.Fatal("a sheet covers one section and the command should say so rather than pick one")
	}
	if !strings.Contains(err.Error(), rel) {
		t.Errorf("somebody who left -file off should be told what to put there, got %q", err)
	}
}

func TestShareReadRefusesTheIntroduction(t *testing.T) {
	root := readCorpus(t)
	rel := writeImport(t, root, "sets", 1, 0, true, "This introduction has no numbering of its own.\n")
	err := shareRead([]string{"-corpus", root, "-book", "ens-i-iv", "-file", rel})
	if err == nil || !strings.Contains(err.Error(), "introduction") {
		t.Fatalf("the introduction has no § and no pages to hold it against, got %v", err)
	}
}

func TestShareReadRefusesASectionWhosePagesAreUnread(t *testing.T) {
	root := readCorpus(t)
	for _, n := range []int{22, 23} {
		if err := os.Remove(corpus.PagePath(root, "ens-i-iv", n)); err != nil {
			t.Fatal(err)
		}
	}
	rel := writeImport(t, root, "sets", 1, 1, false, faithful)
	err := shareRead([]string{"-corpus", root, "-book", "ens-i-iv", "-file", rel})
	if err == nil || !strings.Contains(err.Error(), "has been read") {
		t.Fatalf("with no page read there is nothing to hold the import against, got %v", err)
	}
}

// printedSection hands the audit the pages of the § and hands the reading sheet
// one page more. The two callers want different things out of the same walk and
// the fields have to stay apart, because a page whose greater part is the next §
// cannot be demanded of this import.
func TestPrintedSectionKeepsTheBoundaryPageOutOfPages(t *testing.T) {
	root := readCorpus(t)
	man, err := corpus.LoadTOC(root)
	if err != nil {
		t.Fatal(err)
	}
	bt, _ := man.Get("ens-i-iv")
	p, unread, err := printedSection(root, "ens-i-iv", bt, 1, 1)
	if err != nil {
		t.Fatal(err)
	}
	if unread != 0 {
		t.Errorf("every page of the § is written, got %d unread", unread)
	}
	if len(p.Pages) != 2 || p.Pages[0].PDFPage != 22 || p.Pages[1].PDFPage != 23 {
		t.Fatalf("want pdf 22 and 23 as the §'s own pages, got %v", p.Pages)
	}
	if len(p.After) != 1 || p.After[0].PDFPage != 24 {
		t.Fatalf("want pdf 24 as the boundary page, got %v", p.After)
	}
}

func TestPrintedSectionSaysNothingOfABoundaryPageNobodyHasRead(t *testing.T) {
	root := readCorpus(t)
	if err := os.Remove(corpus.PagePath(root, "ens-i-iv", 24)); err != nil {
		t.Fatal(err)
	}
	man, err := corpus.LoadTOC(root)
	if err != nil {
		t.Fatal(err)
	}
	bt, _ := man.Get("ens-i-iv")
	p, _, err := printedSection(root, "ens-i-iv", bt, 1, 1)
	if err != nil {
		t.Fatalf("an unread boundary page is ordinary and must not be an error: %v", err)
	}
	if len(p.After) != 0 {
		t.Fatalf("want no boundary page, got %v", p.After)
	}
}
