package main

import (
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
