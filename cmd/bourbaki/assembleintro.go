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
	return frontFile(b, *b.Introduction, corpus.KindIntroduction, lang, pages)
}

// readerFile is the publisher's note to the reader as it goes to disk.
//
// The same thing as an introduction from the assembler's point of view: pages
// named in the manifest, standing before chapter I, under a heading repeated as
// the running head of every page after the first. What it is not is an
// introduction, and it is kept apart from one because a volume can have both,
// and because the note is the same text in nearly every volume of the series
// while the introduction is about this volume's mathematics.
func readerFile(b corpus.Book, lang string, pages map[int]corpus.PageFile) (corpus.SectionFile, assemble.Piece, error) {
	return frontFile(b, *b.ReaderNote, corpus.KindReader, lang, pages)
}

// notationFile and terminologyFile are the volume's two indexes as they go to
// disk. From the assembler's point of view they are introductions again: a run
// of pages named in the manifest, under a heading repeated as the running head
// of every page after the first, belonging to no chapter. The only difference
// is which end of the book they stand at, and nothing here depends on that.
func notationFile(b corpus.Book, lang string, pages map[int]corpus.PageFile) (corpus.SectionFile, assemble.Piece, error) {
	return frontFile(b, *b.NotationIndex, corpus.KindNotation, lang, pages)
}

func terminologyFile(b corpus.Book, lang string, pages map[int]corpus.PageFile) (corpus.SectionFile, assemble.Piece, error) {
	return frontFile(b, *b.TerminologyIndex, corpus.KindTerminology, lang, pages)
}

// frontFile is the half they have in common. Nothing is looked up for
// either: neither carries a numbered statement, so there is no label, no tag
// and no erratum to find, and the permanent tags and the errata are both found
// by label.
func frontFile(b corpus.Book, in corpus.Introduction, kind, lang string, pages map[int]corpus.PageFile) (corpus.SectionFile, assemble.Piece, error) {
	p, err := assemble.Introduction(in, pages)
	if err != nil {
		return corpus.SectionFile{}, p, err
	}
	m := corpus.SectionFrontMatter{
		Book:          b.Book,
		BookTitle:     corpus.BookTitle(b.Book),
		SectionTitle:  in.Title,
		Kind:          kind,
		Lang:          lang,
		Source:        b.ID,
		SourceEdition: b.Edition,
		BookPages:     bookPages(p.Runs),
		PDFPages:      pdfPages(p.Runs),
		Extraction:    p.Extraction(),
		Filename:      in.File,
	}
	return corpus.SectionFile{Meta: m, Body: corpus.NormalizeBody(p.Body)}, p, nil
}
