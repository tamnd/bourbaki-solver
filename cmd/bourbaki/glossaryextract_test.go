package main

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/glossary"
)

// The miner reads content/en, and a volume that is still being scanned has
// nothing there. Its table of contents holds the same § and no. titles its
// section files will hold, so the terminology of a Book can be settled while
// the Book is being read, which is the order the work actually goes in: the
// glossary has to be fixed before the first translation, and the first
// translation runs the day the first chapter assembles.
func TestATableOfContentsStandsInForAVolumeNotYetAssembled(t *testing.T) {
	root := smallCorpus(t)
	docs, err := unassembledTOCDocs(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 1 {
		t.Fatalf("got %d documents, want one for the one chapter: %+v", len(docs), docs)
	}
	if docs[0].Body != "" {
		t.Errorf("the document carries a body, and a table of contents has none: %q", docs[0].Body)
	}
	want := []string{
		"Semisimple Modules and Rings",
		"Artinian Modules and Noetherian Modules",
		"Artinian Modules",
	}
	if len(docs[0].Titles) != len(want) {
		t.Fatalf("titles are %q, want %q", docs[0].Titles, want)
	}
	for i, w := range want {
		if docs[0].Titles[i] != w {
			t.Errorf("title %d is %q, want %q", i, docs[0].Titles[i], w)
		}
	}
}

// A Book that is assembled has its titles read off its section files already,
// and mining its table of contents on top of that would count every title
// twice. The bar is the Book and not the volume, because Algebra is three
// volumes under one directory and assembling any of them is enough.
func TestAnAssembledBookIsNotMinedTwice(t *testing.T) {
	root := smallCorpus(t)
	have := []glossary.Doc{{Path: "content/en/alg/VIII/01.md"}}
	docs, err := unassembledTOCDocs(root, have)
	if err != nil {
		t.Fatal(err)
	}
	if len(docs) != 0 {
		t.Errorf("the Book is assembled and its table of contents was mined anyway: %+v", docs)
	}
}

// The French volumes are left out. Their titles are French, and a French phrase
// in a list of English candidates is a rendering asked for in the wrong
// direction.
func TestAFrenchVolumeIsNotMinedIntoTheEnglishCandidates(t *testing.T) {
	root := smallCorpus(t)
	books, err := corpus.LoadBooks(root)
	if err != nil {
		t.Fatal(err)
	}
	books.Upsert(corpus.Book{
		ID: "alg-viii-fr", Book: "alg-fr", Lang: "fr", Title: "Algèbre, Chapitre 8",
		Edition: "2012, Springer", Chapters: []string{"VIII"}, Pages: 4,
		Nature: "digital", Extraction: "native",
	})
	if err := books.Save(root); err != nil {
		t.Fatal(err)
	}
	toc, err := corpus.LoadTOC(root)
	if err != nil {
		t.Fatal(err)
	}
	toc.Upsert(corpus.BookTOC{ID: "alg-viii-fr", Grammar: "head-label", Chapters: []corpus.Chapter{{
		Book: "alg-viii-fr", Numeral: "VIII", Title: "Modules et anneaux semi-simples", Page: 1, PDFPage: 18,
	}}})
	if err := toc.Save(root); err != nil {
		t.Fatal(err)
	}
	docs, err := unassembledTOCDocs(root, nil)
	if err != nil {
		t.Fatal(err)
	}
	for _, d := range docs {
		for _, title := range d.Titles {
			if title == "Modules et anneaux semi-simples" {
				t.Errorf("a French title was offered as an English candidate")
			}
		}
	}
	if len(docs) != 1 {
		t.Errorf("got %d documents, want the one English chapter", len(docs))
	}
}

// The candidates are ordered by how often the prose says a phrase, and a volume
// whose pages are not read yet has no prose at all. Every row Theory of Sets
// puts in is a title with a count of one or two, so it sits at the bottom of a
// file of two and a half thousand, and -add reaching it means asking about
// everything above it first. -only is the way to the good source directly.
func TestOnlyTakesTheCandidatesOfOneSource(t *testing.T) {
	root := t.TempDir()
	g := &glossary.Glossary{Version: 1, Terms: []glossary.Term{{EN: "simple", VI: "đơn"}}}
	if err := g.Write(glossary.Path(root)); err != nil {
		t.Fatal(err)
	}
	if err := glossary.WriteCandidates(glossary.CandidatesPath(root), glossary.Candidates{
		Version: 1, From: "content/en", Terms: []glossary.Candidate{
			{EN: "flat quiver", Count: 90, Sources: []string{glossary.SourceProse}},
			{EN: "wobbly quiver", Count: 40, Sources: []string{glossary.SourceProse}},
			{EN: "Ordered gribbles", Count: 1, Sources: []string{glossary.SourceTitle}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	got, err := needing(root, loaded(t, root), "vi", 10, 0, glossary.SourceTitle, nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0] != "Ordered gribbles" {
		t.Fatalf("asked about %q, want the one title", got)
	}
	// And without it the commonest come first, which is what -add has always
	// done and what the rest of the corpus is filled by.
	got, err = needing(root, loaded(t, root), "vi", 2, 0, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != "flat quiver" || got[1] != "wobbly quiver" {
		t.Errorf("asked about %q, want the two commonest", got)
	}
}

// A term somebody has read the corpus and found is hardly ever at the head of
// the frequency order, and the head is the only door -add opens. "lattice" is
// used 61 times across the series and the Vietnamese renders it two ways, dàn
// in most places and mạng in eleven, which is a defect a person finds by
// reading and not a phrase a miner ranks highly; reaching it by -add means
// asking about two hundred phrases above it, of which "prop", "chap" and
// "exerc" are three.
func TestATermIsAskedAboutByName(t *testing.T) {
	root := t.TempDir()
	g := &glossary.Glossary{Version: 1, Terms: []glossary.Term{
		{EN: "simple", VI: "đơn"},
		{EN: "quiver"},
	}}
	if err := g.Write(glossary.Path(root)); err != nil {
		t.Fatal(err)
	}
	if err := glossary.WriteCandidates(glossary.CandidatesPath(root), glossary.Candidates{
		Version: 1, From: "content/en", Terms: []glossary.Candidate{
			{EN: "flat quiver", Count: 90, Sources: []string{glossary.SourceProse}},
			{EN: "wobbly quiver", Count: 40, Sources: []string{glossary.SourceProse}},
			{EN: "lattice", Count: 12, Sources: []string{glossary.SourceProse}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	// The row with no Vietnamese comes first, as it always has, and the named
	// term after it, with nothing from the head of the order.
	got, err := needing(root, loaded(t, root), "vi", 0, 0, "", []string{"lattice"})
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"quiver", "lattice"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("asked about %q, want %q", got, want)
	}
	// Naming a term does not come out of the -add budget, since a person who
	// names one wants it as well as the pass and not instead of part of it.
	got, err = needing(root, loaded(t, root), "vi", 1, 0, "", []string{"lattice"})
	if err != nil {
		t.Fatal(err)
	}
	want = []string{"quiver", "lattice", "flat quiver"}
	if len(got) != len(want) {
		t.Fatalf("asked about %q, want %q", got, want)
	}
	for i, w := range want {
		if got[i] != w {
			t.Errorf("term %d is %q, want %q", i, got[i], w)
		}
	}
}

// The two ways of getting this wrong, both refused rather than written into the
// file where nobody would look at them again. A term nothing in the corpus says
// is a typo, and a rendering that is already there is glossary set's to correct
// and never this command's to overwrite.
func TestATermAskedAboutByNameIsOneTheCorpusSays(t *testing.T) {
	root := t.TempDir()
	g := &glossary.Glossary{Version: 1, Terms: []glossary.Term{{EN: "simple", VI: "đơn"}}}
	if err := g.Write(glossary.Path(root)); err != nil {
		t.Fatal(err)
	}
	if err := glossary.WriteCandidates(glossary.CandidatesPath(root), glossary.Candidates{
		Version: 1, From: "content/en", Terms: []glossary.Candidate{
			{EN: "lattice", Count: 12, Sources: []string{glossary.SourceProse}},
		},
	}); err != nil {
		t.Fatal(err)
	}
	if _, err := needing(root, loaded(t, root), "vi", 0, 0, "", []string{"latice"}); err == nil {
		t.Error("a term the miner never saw was accepted")
	}
	_, err := needing(root, loaded(t, root), "vi", 0, 0, "", []string{"simple"})
	if err == nil {
		t.Fatal("a term that already has its Vietnamese was accepted")
	}
	if !strings.Contains(err.Error(), "glossary set") {
		t.Errorf("the error is %q, and it should say what does correct a rendering", err)
	}
}
