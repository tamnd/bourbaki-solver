package main

import (
	"fmt"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// A leaf bound the wrong way round is read where the printing sets it, not
// where the binder put it.
//
// The assembler walks a volume by counting pdf pages up from where the contents
// points, and joins the last paragraph of one page to the first of the next. A
// file whose leaves are out of order is then read out of order, and nothing in
// it says so: the pages are all there, they all assemble, and what comes out is
// prose that steps sideways once and back.
//
// Algebre chapitres 4 a 7 in French is the volume that has it. Its pdf 274
// heads A V.169 and carries the end of the exercices of chapter V; its pdf 273
// opens the NOTE HISTORIQUE of chapters IV and V at printed 170, and the last
// paragraph of that page runs straight into pdf 275, which heads A V.171. So
// the printing reads 272, 274, 273, 275 and the file holds 272, 273, 274, 275.
// books.yaml has said so since the page map was fitted, in the transposed:
// block, and assembly was the one thing that never read it.
//
// What that cost is one file. Exercise 5 of § 17 of chapter V came out 55572
// characters, the largest file in the corpus, because the exercise ran on to
// pdf 273, met the opening of the note historique in the middle of itself and
// swallowed it; and chapter V then had no historical note at all, because the
// note had already been eaten by an exercise before anything looked for one.
//
// The fix is a permutation, built once here and applied to two things: the
// pages map, which is re-keyed by printing position so that counting up through
// it walks the book, and every pdf page the contents manifest names, which is
// sent the same way so that a locator still points at the same leaf. Nothing is
// written through it. The pages keep the pdf_page they were read at, the
// contents manifest keeps true pdf pages, which is what fix opening renders
// from, and the assembled front matter names the leaf and not the position.

// bound is where a volume's leaves stand once they are read in printing order.
// It maps a pdf page to the position it should be read at, and a position to
// the pdf page that should be read there, which are the same map: transposing
// pairs of leaves is an involution, so the permutation is its own inverse.
type bound map[int]int

// boundOrder is the permutation the volume's transposed: block describes.
//
// A volume with no transposition gets an empty map, which is the identity and
// costs a map lookup per page.
func boundOrder(b corpus.Book) (bound, error) {
	swaps, err := transpositions(b)
	if err != nil {
		return nil, err
	}
	out := bound{}
	for _, s := range swaps {
		for _, p := range s {
			if b.Pages > 0 && (p < 1 || p > b.Pages) {
				return nil, fmt.Errorf("%s: pdf page %d is transposed, outside the %d pages of the volume",
					b.ID, p, b.Pages)
			}
			if _, dup := out[p]; dup {
				return nil, fmt.Errorf("%s: pdf page %d is transposed twice", b.ID, p)
			}
		}
		out[s[0]], out[s[1]] = s[1], s[0]
	}
	return out, nil
}

// at sends a pdf page to its printing position, and a printing position to the
// pdf page bound there. See bound.
func (o bound) at(p int) int {
	if q, ok := o[p]; ok {
		return q
	}
	return p
}

// pages re-keys the pages of a volume by printing position, so that counting up
// through them walks the book. Each page keeps the pdf_page it was read at.
func (o bound) pages(in map[int]corpus.PageFile) map[int]corpus.PageFile {
	if len(o) == 0 {
		return in
	}
	out := make(map[int]corpus.PageFile, len(in))
	for p, f := range in {
		out[o.at(p)] = f
	}
	return out
}

// book is the volume with the pdf pages of its front and back matter sent
// through the permutation, so that the assembler looks for them where the
// printing puts them.
func (o bound) book(b corpus.Book) corpus.Book {
	if len(o) == 0 {
		return b
	}
	for _, in := range []**corpus.Introduction{&b.ReaderNote, &b.Introduction, &b.NotationIndex, &b.TerminologyIndex} {
		if *in == nil {
			continue
		}
		c := **in
		c.FirstPDFPage, c.LastPDFPage = o.at(c.FirstPDFPage), o.at(c.LastPDFPage)
		*in = &c
	}
	return b
}

// chapters is the contents with every pdf page it names sent through the
// permutation. It copies down to each locator rather than writing through the
// ones it was given, so that what the caller loaded from the file still reads
// as the file has it. Nothing writes the contents manifest back from here.
func (o bound) chapters(in []corpus.Chapter) []corpus.Chapter {
	if len(o) == 0 {
		return in
	}
	out := make([]corpus.Chapter, len(in))
	for i, ch := range in {
		ch.PDFPage = o.at(ch.PDFPage)
		ch.Historical = o.locator(ch.Historical)
		ch.Exercises = o.locator(ch.Exercises)
		ch.Subsections = o.subsections(ch.Subsections)
		sections := make([]corpus.Section, len(ch.Sections))
		for j, s := range ch.Sections {
			s.PDFPage = o.at(s.PDFPage)
			s.Exercises = o.locator(s.Exercises)
			s.Subsections = o.subsections(s.Subsections)
			sections[j] = s
		}
		ch.Sections = sections
		out[i] = ch
	}
	return out
}

// subsections sends the pdf page of each no. heading through the permutation.
//
// It is used both ways round. The contents goes through it on the way in, so
// that the assembler checks a heading against the entry for it; the headings the
// assembler found go back through it on the way out, so that the no. written
// into a section's front matter names the leaf a reader would turn to. Being an
// involution, the same call does both. The no. page is the one number that is
// checked against the contents and also written to a file, which is why it is
// the one that has to make the round trip.
func (o bound) subsections(in []corpus.Subsection) []corpus.Subsection {
	if len(o) == 0 || in == nil {
		return in
	}
	out := make([]corpus.Subsection, len(in))
	for i, s := range in {
		s.PDFPage = o.at(s.PDFPage)
		out[i] = s
	}
	return out
}

func (o bound) locator(l *corpus.Locator) *corpus.Locator {
	if l == nil {
		return nil
	}
	c := *l
	c.PDFPage = o.at(c.PDFPage)
	return &c
}
