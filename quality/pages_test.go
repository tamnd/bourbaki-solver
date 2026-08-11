package quality

import (
	"fmt"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// pageTree writes n pages of a book and gives back the corpus they are in, with
// no answer from git in it yet.
func pageTree(t *testing.T, book string, n int) *Corpus {
	t.Helper()
	root := t.TempDir()
	for page := 1; page <= n; page++ {
		f := corpus.PageFile{
			Meta: corpus.PageFrontMatter{Book: book, PDFPage: page, Method: corpus.MethodOCR},
			Body: fmt.Sprintf("Page %d of the volume.\n", page),
		}
		if err := f.Write(corpus.PagePath(root, book, page)); err != nil {
			t.Fatal(err)
		}
	}
	return &Corpus{
		Root:  root,
		Books: &corpus.BooksManifest{Books: []corpus.Book{{ID: book, Book: "alg", Pages: n}}},
	}
}

// The audit report is regenerated in CI and diffed against the committed one,
// so what the audit counts has to be what the repository holds. An OCR run
// writes pages for hours and commits none of them, and the pages it has written
// so far are in the working tree and nowhere else.
func TestAPageNobodyCommittedIsNotAPageTheCorpusHas(t *testing.T) {
	c := pageTree(t, "alg-x-fr", 3)
	c.Tracked = []string{"pages/alg-x-fr/0001.md", "pages/alg-x-fr/0002.md"}
	if err := c.readPages(); err != nil {
		t.Fatal(err)
	}
	if got := len(c.Pages["alg-x-fr"]); got != 2 {
		t.Errorf("the corpus holds %d pages, want the 2 git has", got)
	}
	for _, p := range c.PagePaths["alg-x-fr"] {
		if p == "pages/alg-x-fr/0003.md" {
			t.Error("the uncommitted page is in the paths")
		}
	}
}

// Every test that builds a corpus in a temporary directory is in no repository
// at all, and so is a developer who has not run git init. Reading nothing there
// would be a worse answer than reading everything.
func TestWithNoAnswerFromGitEveryPageOnDiskCounts(t *testing.T) {
	c := pageTree(t, "alg-x-fr", 3)
	c.TrackedErr = fmt.Errorf("git ls-files: not a repository")
	if err := c.readPages(); err != nil {
		t.Fatal(err)
	}
	if got := len(c.Pages["alg-x-fr"]); got != 3 {
		t.Errorf("the corpus holds %d pages, want all 3 that are on disk", got)
	}
}

// The pages stay in reading order, which is what assembly and the S rules walk
// them in, whether or not the run that wrote them stopped partway.
func TestSkippingAPageDoesNotDisturbTheOrder(t *testing.T) {
	c := pageTree(t, "alg-x-fr", 4)
	c.Tracked = []string{"pages/alg-x-fr/0004.md", "pages/alg-x-fr/0001.md", "pages/alg-x-fr/0003.md"}
	if err := c.readPages(); err != nil {
		t.Fatal(err)
	}
	var got []int
	for _, p := range c.Pages["alg-x-fr"] {
		got = append(got, p.Meta.PDFPage)
	}
	if len(got) != 3 || got[0] != 1 || got[1] != 3 || got[2] != 4 {
		t.Errorf("the pages came out %v, want 1 3 4", got)
	}
}
