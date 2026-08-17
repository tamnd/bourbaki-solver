package main

import (
	"github.com/tamnd/bourbaki-solver/assemble"
	"github.com/tamnd/bourbaki-solver/corpus"
)

// introFile is the Book's introduction as it goes to disk.
//
// It is a section file like any other and it is the only one that belongs to no
// chapter, which is what the empty Chapter says. Nothing is looked up for it:
// an introduction carries no numbered statement, so there is no label, no tag
// and no erratum to find, and the permanent tags and the errata are both found
// by label.
func introFile(b corpus.Book, lang string, pages map[int]corpus.PageFile) (corpus.SectionFile, assemble.Piece, error) {
	p, err := assemble.Introduction(*b.Introduction, pages)
	if err != nil {
		return corpus.SectionFile{}, p, err
	}
	m := corpus.SectionFrontMatter{
		Book:          b.Book,
		BookTitle:     corpus.BookTitle(b.Book),
		SectionTitle:  b.Introduction.Title,
		Kind:          corpus.KindIntroduction,
		Lang:          lang,
		Source:        b.ID,
		SourceEdition: b.Edition,
		BookPages:     bookPages(p.Runs),
		PDFPages:      pdfPages(p.Runs),
		Extraction:    p.Extraction(),
	}
	return corpus.SectionFile{Meta: m, Body: corpus.NormalizeBody(p.Body)}, p, nil
}
