package main

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
)

// folioCorpus is a corpus of one volume: a manifest, three pages, and the page
// map the pages are checked against. It is the smallest thing fix folio can be
// run against, since the command reads all three.
func folioCorpus(t *testing.T, b corpus.Book, pages map[int]string, entries []pagemap.Entry) string {
	t.Helper()
	root := t.TempDir()
	t.Setenv("BOURBAKI_CORPUS", root)
	m := &corpus.BooksManifest{Books: []corpus.Book{b}}
	if err := m.Save(root); err != nil {
		t.Fatal(err)
	}
	for page, body := range pages {
		writePage(t, root, b.ID, page, body)
	}
	pm := &pagemap.Map{
		Book: b.ID, Grammar: pagemap.Grammar(b.Grammar),
		Pagination: pagemap.Pagination(b.Pagination),
		PDFPages:   b.Pages, Entries: entries,
	}
	if err := pm.Save(root); err != nil {
		t.Fatal(err)
	}
	return root
}

func readPage(t *testing.T, root, book string, page int) corpus.PageFile {
	t.Helper()
	f, err := corpus.ReadFile[corpus.PageFrontMatter](corpus.PagePath(root, book, page))
	if err != nil {
		t.Fatal(err)
	}
	return f
}

var ens = corpus.Book{
	ID: "ens-i-iv", Book: "ens", Lang: "en", Pages: 3,
	Grammar: "foot-number", Pagination: "continuous",
}

// The number goes to folio and the body loses it. No page label is written,
// because Theory of Sets numbers its pages straight through the book: "E I.24"
// would name the twenty fourth page of chapter I, and page 24 of this volume is
// in chapter I only by accident of where the chapter happens to start.
func TestFixFolioMovesTheNumberAndWritesNoLabel(t *testing.T) {
	root := folioCorpus(t, ens, map[int]string{
		1: "Let $E$ be a set.\n\n21\n",
		2: "## § 1. DEFINITIONS\n",
		3: "The empty set.\n\n23\n",
	}, []pagemap.Entry{
		{PDFPage: 1, Chapter: "I", Page: 21, Confidence: "foot"},
		{PDFPage: 2, Chapter: "I", Page: 22, Confidence: "interpolated"},
		{PDFPage: 3, Chapter: "I", Page: 23, Confidence: "foot"},
	})
	if err := fixFolio(nil); err != nil {
		t.Fatal(err)
	}

	first := readPage(t, root, ens.ID, 1)
	if first.Meta.Folio != 21 {
		t.Errorf("folio = %d, want 21", first.Meta.Folio)
	}
	if first.Meta.PageLabel != "" {
		t.Errorf("page_label = %q, and a volume paginated straight through prints none", first.Meta.PageLabel)
	}
	if strings.Contains(first.Body, "21") {
		t.Errorf("the number is still in the body: %q", first.Body)
	}
	if !strings.Contains(first.Body, "Let $E$ be a set.") {
		t.Errorf("the text went with it: %q", first.Body)
	}
	if first.Meta.Lines != 1 {
		t.Errorf("lines = %d, want 1, the line that is left", first.Meta.Lines)
	}

	// A page that prints no number keeps its body and gets no folio. The map
	// interpolated one for it, and an interpolated number is not printed on the
	// page, so writing it here would put a number in the corpus that nobody can
	// find in the book.
	second := readPage(t, root, ens.ID, 2)
	if second.Meta.Folio != 0 || second.Body != "## § 1. DEFINITIONS\n" {
		t.Errorf("a page with no printed number came back folio %d, body %q", second.Meta.Folio, second.Body)
	}
}

// One of the two is wrong and there is no way to tell which, so the page is
// left exactly as it stands and named on stderr. Believing the foot would write
// a number the map disagrees with, and believing the map would overwrite what
// the page plainly prints.
func TestFixFolioLeavesADisagreementAlone(t *testing.T) {
	root := folioCorpus(t, ens, map[int]string{
		1: "Let $E$ be a set.\n\n21\n",
	}, []pagemap.Entry{
		{PDFPage: 1, Chapter: "I", Page: 121, Confidence: "foot"},
	})
	if err := fixFolio(nil); err != nil {
		t.Fatal(err)
	}
	f := readPage(t, root, ens.ID, 1)
	if f.Meta.Folio != 0 {
		t.Errorf("folio = %d, want none written on a disagreement", f.Meta.Folio)
	}
	if !strings.HasSuffix(f.Body, "21\n") {
		t.Errorf("the body was cut anyway: %q", f.Body)
	}
}

// A volume that prints its number in the running head is not touched here at
// all. SplitHead took the head off as the page was read, so anything at the
// foot of one of these bodies is text.
func TestFixFolioSkipsTheVolumesThatPrintTheNumberInTheHead(t *testing.T) {
	alg := corpus.Book{
		ID: "alg-viii", Book: "alg", Lang: "en", Pages: 1,
		Grammar: "head-label", Pagination: "per-chapter",
	}
	root := folioCorpus(t, alg, map[int]string{
		1: "the order of the group is\n\n60\n",
	}, []pagemap.Entry{{PDFPage: 1, Chapter: "VIII", Page: 60, Confidence: "head"}})
	if err := fixFolio(nil); err != nil {
		t.Fatal(err)
	}
	f := readPage(t, root, alg.ID, 1)
	if f.Meta.Folio != 0 || !strings.HasSuffix(f.Body, "60\n") {
		t.Errorf("a head-label volume was cut: folio %d, body %q", f.Meta.Folio, f.Body)
	}
}

// The page prints a number, the map has it off that same foot, and the reading
// dropped it. Page 32 of Theory of Sets is this: 25 at the foot of the image, no
// folio in the file. -fill writes it and says on the page where it came from.
func TestFixFolioFillsANumberTheReadingDropped(t *testing.T) {
	root := folioCorpus(t, ens, map[int]string{
		1: "Let $E$ be a set.\n",
	}, []pagemap.Entry{{PDFPage: 1, Chapter: "I", Page: 25, Confidence: "foot"}})
	if err := fixFolio([]string{"-fill"}); err != nil {
		t.Fatal(err)
	}
	f := readPage(t, root, ens.ID, 1)
	if f.Meta.Folio != 25 {
		t.Errorf("folio = %d, want 25 from the page map", f.Meta.Folio)
	}
	if f.Body != "Let $E$ be a set.\n" {
		t.Errorf("the body was touched, and there was nothing in it to take: %q", f.Body)
	}
	if len(f.Meta.Flags) != 1 || !strings.Contains(f.Meta.Flags[0], "page map") {
		t.Errorf("flags = %q, and a number from somewhere other than the page says so", f.Meta.Flags)
	}
}

// Without the flag nothing is filled, so no other volume changes under a run of
// fix folio that was meant to move numbers out of bodies.
func TestFixFolioFillsNothingUnasked(t *testing.T) {
	root := folioCorpus(t, ens, map[int]string{
		1: "Let $E$ be a set.\n",
	}, []pagemap.Entry{{PDFPage: 1, Chapter: "I", Page: 25, Confidence: "foot"}})
	if err := fixFolio(nil); err != nil {
		t.Fatal(err)
	}
	if f := readPage(t, root, ens.ID, 1); f.Meta.Folio != 0 || len(f.Meta.Flags) != 0 {
		t.Errorf("folio = %d, flags = %q, and neither was asked for", f.Meta.Folio, f.Meta.Flags)
	}
}

// An interpolated number is not printed on the page. Filling it would put a
// number in the corpus that a reader holding the book cannot find at the foot,
// so -fill leaves the page as it stands.
func TestFixFolioFillsNoNumberThePageDoesNotPrint(t *testing.T) {
	root := folioCorpus(t, ens, map[int]string{
		1: "## § 1. DEFINITIONS\n",
	}, []pagemap.Entry{{PDFPage: 1, Chapter: "I", Page: 22, Confidence: "interpolated"}})
	if err := fixFolio([]string{"-fill"}); err != nil {
		t.Fatal(err)
	}
	if f := readPage(t, root, ens.ID, 1); f.Meta.Folio != 0 {
		t.Errorf("folio = %d, and the page prints none", f.Meta.Folio)
	}
}

// A page repaired by an earlier run has no number left in its body and a folio
// in its front matter. Running again must not read that as a page that prints
// none and must not flag it as filled from the map.
func TestFixFolioLeavesAPageItAlreadyRepaired(t *testing.T) {
	root := folioCorpus(t, ens, map[int]string{
		1: "Let $E$ be a set.\n\n21\n",
	}, []pagemap.Entry{{PDFPage: 1, Chapter: "I", Page: 21, Confidence: "foot"}})
	if err := fixFolio([]string{"-fill"}); err != nil {
		t.Fatal(err)
	}
	if err := fixFolio([]string{"-fill"}); err != nil {
		t.Fatal(err)
	}
	f := readPage(t, root, ens.ID, 1)
	if f.Meta.Folio != 21 {
		t.Errorf("folio = %d, want the 21 the first run took off the foot", f.Meta.Folio)
	}
	if len(f.Meta.Flags) != 0 {
		t.Errorf("flags = %q, and the number was read off the page by the first run", f.Meta.Flags)
	}
}

// -check is the same reading with nothing written, which is what makes it safe
// to run over the whole corpus before running it for real.
func TestFixFolioCheckWritesNothing(t *testing.T) {
	root := folioCorpus(t, ens, map[int]string{
		1: "Let $E$ be a set.\n\n21\n",
	}, []pagemap.Entry{{PDFPage: 1, Chapter: "I", Page: 21, Confidence: "foot"}})
	if err := fixFolio([]string{"-check"}); err != nil {
		t.Fatal(err)
	}
	f := readPage(t, root, ens.ID, 1)
	if f.Meta.Folio != 0 || !strings.HasSuffix(f.Body, "21\n") {
		t.Errorf("-check wrote to the page: folio %d, body %q", f.Meta.Folio, f.Body)
	}
}
