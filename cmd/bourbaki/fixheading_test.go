package main

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// headingCorpus is a corpus of one volume with a contents to look headings up
// in. The pages are chapter III of Theory of Sets as the reading wrote them:
// § 1 and its no. 1 on pdf 137, no. 12 on 152 written a level too high, and
// § 2 on 154 written correctly.
func headingCorpus(t *testing.T, pages map[int]string) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("BOURBAKI_CORPUS", root)
	books := &corpus.BooksManifest{Books: []corpus.Book{{
		ID: "ens-i-iv", Book: "ens", Lang: "en", Pages: 418,
	}}}
	if err := books.Save(root); err != nil {
		t.Fatal(err)
	}
	man := &corpus.TOCManifest{Books: []corpus.BookTOC{{
		ID: "ens-i-iv", Chapters: []corpus.Chapter{{
			Numeral: "III", PDFPage: 137,
			Sections: []corpus.Section{{
				Number: 1, Title: "Order relations. Ordered sets", PDFPage: 137,
				Subsections: []corpus.Subsection{
					{Number: 1, Title: "Definition of an order relation", PDFPage: 137},
					{Number: 12, Title: "Totally ordered sets", PDFPage: 152},
				},
			}, {
				Number: 2, Title: "Well-ordered sets", PDFPage: 154,
				Subsections: []corpus.Subsection{
					{Number: 1, Title: "Segments of a well-ordered set", PDFPage: 154},
				},
			}},
		}},
	}}}
	if err := man.Save(root); err != nil {
		t.Fatal(err)
	}
	for page, body := range pages {
		writePage(t, root, "ens-i-iv", page, body)
	}
	return root
}

func TestFixHeadingPutsANoBackUnderItsSection(t *testing.T) {
	root := headingCorpus(t, map[int]string{
		137: "## 1. ORDER RELATIONS. ORDERED SETS\n\n### 1. DEFINITION OF AN ORDER RELATION\n\nLet $E$ be a set.\n",
		152: "## 12. TOTALLY ORDERED SETS\n\nA set is totally ordered when.\n",
		154: "## 2. WELL-ORDERED SETS\n\n### 1. SEGMENTS OF A WELL-ORDERED SET\n",
	})
	if err := fixHeading(nil); err != nil {
		t.Fatal(err)
	}

	// The one the reading got wrong.
	moved := readPage(t, root, "ens-i-iv", 152)
	if !strings.HasPrefix(moved.Body, "### 12. TOTALLY ORDERED SETS\n") {
		t.Errorf("page 152 reads %q", moved.Body)
	}
	if !strings.Contains(moved.Body, "A set is totally ordered when.") {
		t.Errorf("the rest of the page went with it: %q", moved.Body)
	}

	// The pair that a lookup by number alone would have got wrong: the § and
	// its first no. are both numbered 1 and both begin on page 137.
	pair := readPage(t, root, "ens-i-iv", 137)
	for _, want := range []string{"## 1. ORDER RELATIONS. ORDERED SETS", "### 1. DEFINITION OF AN ORDER RELATION"} {
		if !strings.Contains(pair.Body, want+"\n") {
			t.Errorf("page 137 lost %q: %q", want, pair.Body)
		}
	}

	// And a page that was already right is left as it stands.
	right := readPage(t, root, "ens-i-iv", 154)
	if !strings.HasPrefix(right.Body, "## 2. WELL-ORDERED SETS\n") {
		t.Errorf("page 154 reads %q", right.Body)
	}
}

// The contents pages of a volume are set as a list of numbered titles, so they
// read as a page full of headings on pages the contents itself does not name.
// Nothing there is moved.
func TestFixHeadingLeavesAHeadingTheContentsDoesNotHaveThere(t *testing.T) {
	root := headingCorpus(t, map[int]string{
		12: "## 12. TOTALLY ORDERED SETS ......... 146\n",
	})
	if err := fixHeading(nil); err != nil {
		t.Fatal(err)
	}
	if got := readPage(t, root, "ens-i-iv", 12).Body; !strings.HasPrefix(got, "## 12.") {
		t.Errorf("the contents page was rewritten: %q", got)
	}
}

func TestFixHeadingCheckWritesNothing(t *testing.T) {
	root := headingCorpus(t, map[int]string{
		152: "## 12. TOTALLY ORDERED SETS\n",
	})
	if err := fixHeading([]string{"-check"}); err != nil {
		t.Fatal(err)
	}
	if got := readPage(t, root, "ens-i-iv", 152).Body; !strings.HasPrefix(got, "## 12.") {
		t.Errorf("-check wrote the page: %q", got)
	}
}
