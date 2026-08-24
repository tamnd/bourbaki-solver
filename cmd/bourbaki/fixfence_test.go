package main

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// fenceCorpus is two volumes, one of each printing, because the head is read by
// the grammar of the language the volume is in and the page does not carry the
// language.
func fenceCorpus(t *testing.T, pages map[int]string, french map[int]string) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("BOURBAKI_CORPUS", root)
	books := &corpus.BooksManifest{Books: []corpus.Book{
		{ID: "alg-i-iii", Book: "alg", Lang: "en", Pages: 720},
		{ID: "alg-i-iii-fr", Book: "alg", Lang: "fr", Pages: 640},
	}}
	if err := books.Save(root); err != nil {
		t.Fatal(err)
	}
	for page, body := range pages {
		writePage(t, root, "alg-i-iii", page, body)
	}
	for page, body := range french {
		writePage(t, root, "alg-i-iii-fr", page, body)
	}
	return root
}

// Page 314 of Algebra I to III, which is the page the rule was written for. The
// display closes and Proposition 7 opens on the very next line, so Markdown and
// the assembler both read the two as one block, the head is inside a display,
// and the chapter does not assemble.
func TestFixFencePartsAHeadFromTheDisplayAboveIt(t *testing.T) {
	root := fenceCorpus(t, map[int]string{
		314: "The mapping is defined by\n\n$$\n\\pi(x) = x \\otimes 1\n$$\n**Proposition 7.** *The $ \\mathbf{Z} $-linear mapping (7) is bijective.*\n",
	}, map[int]string{
		267: "on a donc\n\n$$\nv = f \\circ u\n$$\n**Corollaire.** — *Pour tout homomorphisme* $v : F \\to F'$ *de A-modules.*\n",
	})
	if err := fixFence(nil); err != nil {
		t.Fatal(err)
	}
	if got := readPage(t, root, "alg-i-iii", 314).Body; !strings.Contains(got, "$$\n\n**Proposition 7.**") {
		t.Errorf("page 314 still runs the head into the display: %q", got)
	}
	if got := readPage(t, root, "alg-i-iii-fr", 267).Body; !strings.Contains(got, "$$\n\n**Corollaire.**") {
		t.Errorf("page 267 still runs the head into the display: %q", got)
	}
}

// A line after a display that does not state a result is where the reading put
// it, and the book means it there: the sentence carrying a display on into the
// text around it is the ordinary case and the one this rule must not touch.
func TestFixFenceLeavesProseWhereItStands(t *testing.T) {
	root := fenceCorpus(t, map[int]string{
		200: "so that\n\n$$\nx = y\n$$\nwhere $ y $ is the element constructed above, by Proposition 2 of § 3.\n",
		201: "and therefore\n\n$$\nx = y\n$$\nTheorem 1 follows from this statement.\n",
	}, nil)
	if err := fixFence(nil); err != nil {
		t.Fatal(err)
	}
	for _, page := range []int{200, 201} {
		if got := readPage(t, root, "alg-i-iii", page).Body; strings.Contains(got, "$$\n\n") {
			t.Errorf("page %d was parted on a line that states nothing: %q", page, got)
		}
	}
}

func TestFixFenceCheckWritesNothing(t *testing.T) {
	root := fenceCorpus(t, map[int]string{
		314: "$$\nx = y\n$$\n**Proposition 7.** *It is bijective.*\n",
	}, nil)
	if err := fixFence([]string{"-check"}); err != nil {
		t.Fatal(err)
	}
	if got := readPage(t, root, "alg-i-iii", 314).Body; strings.Contains(got, "$$\n\n") {
		t.Errorf("-check wrote the page: %q", got)
	}
}
