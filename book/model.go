// Package book builds a printed volume back out of the corpus.
//
// Everything else in this repository reads in one direction. Forty three PDFs
// went in and 14989 pages of Markdown came out, and no gate has ever asked
// whether what came out is a book. The audit reads a body, publish -check reads
// a span, the anchor check reads an identifier. None of them asks whether
// chapter III follows chapter II, whether the § that was assembled out of forty
// pages is the § the printing has, or whether the whole of it comes to roughly
// the length of the volume it was taken from. A build that paginates answers all
// three at once, which is why this exists.
//
// It reads content/ and manifests/ and nothing else. No PDF, no work/, no
// network, the same rule publish keeps, and for the same reason: a reader who
// does not trust the book should be able to build it from a public clone.
package book

import (
	"fmt"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// A Volume is one printed volume of the Elements, in one language.
//
// The volume is the unit because it is what the printing sells and what the
// manifest records: Algebra I is chapters I to III bound together, with one
// trim size, one edition line and one page count to check against. A Book of
// the Elements can run to four volumes and a chapter is never bound alone, so
// neither is a thing to build.
//
// Lang is not the language of the manifest entry. alg-i-iii is an English
// printing, and -lang vi builds the Vietnamese of the same chapters out of
// content/vi, which is the whole point of doing this: a Vietnamese reader
// should get the volume, with the same furniture, and not a directory of files.
type Volume struct {
	Meta corpus.Book
	Lang string

	// Title is the volume's title in Lang, off the table in titles.go. It is
	// not read out of the content: a translated file keeps the book_title of
	// the file it was translated from, so content/vi says Algebra.
	Title string
	// Chapters is the span the manifest names, in printed order, and never
	// more: a build of Algebra I is chapters I to III even though content/
	// holds I to X under the same book.
	Chapters []*Chapter
	// Intro is the Book's own introduction, which stands before chapter I and
	// belongs to no chapter. Four Books have one; the rest leave it nil.
	Intro *Section
	// Reader is the publisher's note to the reader, which stands ahead of the
	// introduction because that is where the printing puts it.
	Reader *Section
}

// Chapter is one chapter of the volume, in the order the volume prints it.
type Chapter struct {
	Numeral string // I, II, VIII
	Title   string
	// Listed is the chapter's title as the printed contents sets it, off
	// manifests/toc/. See Section.Contents, which is the same thing one level
	// down and which was already read; this is the two levels above it, and it
	// is empty for a language the volume was not printed in.
	Listed string
	// Front is the chapter's opening page, which carries the chapter number and
	// the title and sometimes a paragraph under them. It is a file like any
	// other and is nil when the corpus has not got it.
	Front *Section
	// Sections are the §§ and the appendices, in printed order. Bourbaki prints
	// the appendices of a chapter after its last §, and the corpus numbers them
	// in the same field, so an appendix sorts after a § of the same number and
	// the two are told apart by Appendix rather than by the number.
	Sections []*Section
	// Historical is the chapter's Historical Note. Most chapters have one, some
	// do not, and a Book that gathers all of them at the end of the last volume
	// puts it on that chapter.
	Historical *Section
}

// Section is one file of content/, whatever kind of file it is.
type Section struct {
	// Kind is one of corpus.KindFront, KindIntroduction, KindSection,
	// KindAppendix or KindHistorical, the same vocabulary the sections manifest
	// uses.
	Kind   string
	Number int    // the § number, 0 for anything that is not a §
	Title  string // the § title as the corpus has it
	// Listed is the § title as the printed contents sets it, off
	// manifests/toc/, and empty for a language the volume was not printed in.
	Listed  string
	Label   string // alg-i-s1, the permanent name of the §, empty for the rest
	Body    string
	Path    string // repo-relative, for a message that names a file
	Head    int    // the file line the body's first line sits on
	Lang    string
	Subsecs []corpus.Subsection
	// Contents is the title each numbered subsection takes in the table of
	// contents, keyed by its number, off manifests/toc/. It is a second set of
	// titles because the printing has two: page 52 of the English Algebra I to
	// III heads a subsection "1. MONOID OPERATING ON A SET" and page xi lists it
	// as "1. Monoid operating on a set", and only the first of those is in
	// content/. Lower casing the first is not a way to get the second. Doing it
	// naively over the 4240 subsection titles in manifests/toc/ and comparing
	// with what is written there gets 1030 of them wrong, a quarter, because of
	// Hensel and Zariski and Noetherian and "Applications: II." and every
	// mathematical symbol that is a capital letter.
	//
	// It is empty for a language the volume was not printed in, since the
	// manifest holds the contents of the printing and there is no Vietnamese
	// printing to hold a contents of. Those fall back to the heading, which is
	// in capitals, and a Vietnamese contents in capitals is a plain thing that
	// is true rather than a quarter of a page of mangled proper names.
	Contents map[int]string
	// BookTitle and ChapterTitle are the volume's and the chapter's titles as
	// this file has them, which is how a build in a language gets its titles in
	// that language. They are on every section file, translated with the rest,
	// so nothing has to be kept beside the manifest and remembered.
	BookTitle    string
	ChapterTitle string
	// Exercises are the §'s, in number order. They are printed after the § the
	// way the volume prints them, gathered or not, because a corpus that keeps
	// one file per exercise has already lost where the printing put the run and
	// the § is the only place that is right for all three layouts.
	Exercises []*Exercise
	// Statements is what the front matter claims, which the audit counts the
	// labels against.
	Statements int
}

// Exercise is one file of content/<lang>/<book>/<ch>/exercises/<dir>/NN.md.
type Exercise struct {
	Number int
	Label  string
	Body   string
	Path   string
	Head   int
	// Starred marks the ones the printing sets as supplementary, and Hint says
	// the file carries one. Both are front matter and both change how the
	// exercise is set.
	Starred bool
	Hint    bool
}

// IsSection says whether this file is a § or an appendix, which is to say one
// of the numbered divisions that carry statements and exercises. The front
// page, the introduction and the historical note are not.
func (s *Section) IsSection() bool {
	return s.Kind == "" || s.Kind == corpus.KindSection || s.Kind == corpus.KindAppendix
}

// Heading is how the § names itself at the head of its own text, "§ 4." or
// "Appendix 2." or the bare title of a note.
//
// It is worked out here rather than read off the body because the body's own
// first heading is written in the language of the file and in the case of the
// appendices is written in six different ways across the corpus, and a table of
// contents that says "APPEND II" because a Roman numeral fell out of an OCR is
// a table of contents nobody can use.
func (s *Section) Heading() string {
	switch s.Kind {
	case corpus.KindAppendix:
		name := appendixWord[s.Lang]
		if name == "" {
			name = appendixWord["en"]
		}
		if s.Number == 0 {
			return name
		}
		return fmt.Sprintf("%s %d", name, s.Number)
	case corpus.KindHistorical, corpus.KindFront, corpus.KindIntroduction, corpus.KindReader:
		return s.Title
	}
	return fmt.Sprintf("§ %d", s.Number)
}

// appendixWord is the word the printing heads an appendix with, per language.
// It sits beside spanWords for the same reason: it is one of the few words on
// the page that came out of this program rather than out of the corpus, and a
// Vietnamese volume that heads a division "Appendix 2" has an English word in
// it that nothing will ever translate.
var appendixWord = map[string]string{
	"en": "Appendix",
	"fr": "Appendice",
	"vi": "Phụ lục",
}

// Pieces is every file of the volume in printed order, which is the order the
// book sets them and the order the audit counts them in.
//
// One list rather than a walk of the tree, because almost everything that
// happens to a volume happens to every file of it in order: numbering the
// sections, writing the contents, counting the labels, cutting the EPUB into
// documents. Each of those written as its own nested loop over chapters and
// then sections and then the two that are neither is four chances to forget the
// historical note, which is exactly the part of the book that gets forgotten.
func (v *Volume) Pieces() []*Section {
	var out []*Section
	if v.Reader != nil {
		out = append(out, v.Reader)
	}
	if v.Intro != nil {
		out = append(out, v.Intro)
	}
	for _, c := range v.Chapters {
		if c.Front != nil {
			out = append(out, c.Front)
		}
		out = append(out, c.Sections...)
		if c.Historical != nil {
			out = append(out, c.Historical)
		}
	}
	return out
}

// Exercises is every exercise of the volume, in the order its § is printed in.
func (v *Volume) Exercises() []*Exercise {
	var out []*Exercise
	for _, s := range v.Pieces() {
		out = append(out, s.Exercises...)
	}
	return out
}

// ID is what a built file is named, alg-i-iii-vi, the manifest id and the
// language. The language is in it because the same volume is built in three of
// them into one directory and a name that left it out would have them overwrite
// each other.
func (v *Volume) ID() string { return v.Meta.ID + "-" + v.Lang }

// Chapter finds one by numeral.
func (v *Volume) Chapter(numeral string) (*Chapter, bool) {
	for _, c := range v.Chapters {
		if c.Numeral == numeral {
			return c, true
		}
	}
	return nil, false
}

// TrimMM is the trim size in millimetres, which is what a TeX class wants. The
// manifest holds points, because that is what a PDF holds.
func (v *Volume) TrimMM() (w, h float64) {
	return v.Meta.PageWidth * 25.4 / 72, v.Meta.PageHeight * 25.4 / 72
}

// Series is the name of the series as the cover sets it. It stays in French in
// every language, because Elements de mathematique is the name of the thing and
// not a phrase to translate, the same way nobody translates Principia.
const Series = "ÉLÉMENTS DE MATHÉMATIQUE"

// Author is the name on the cover, set the way the cover sets it.
const Author = "N. BOURBAKI"

// ChapterSpan is the chapter line of the cover, "Chapters 1 to 3", in the
// language of the build. The manifest holds the numerals and the printing sets
// them as arabic numerals on the cover and as Roman inside, which is a thing
// worth doing right rather than approximately: a cover that reads "Chapters I
// to III" is not the cover of any Bourbaki anybody has held.
func (v *Volume) ChapterSpan() string {
	if len(v.Chapters) == 0 {
		return ""
	}
	first := arabic(v.Chapters[0].Numeral)
	last := arabic(v.Chapters[len(v.Chapters)-1].Numeral)
	words := spanWords[v.Lang]
	if words == [3]string{} {
		words = spanWords["en"]
	}
	if first == last {
		return words[0] + " " + first
	}
	return words[1] + " " + first + " " + words[2] + " " + last
}

// spanWords is the singular, the plural and the joining word, per language. It
// is a table and not a message catalogue because it is the only sentence the
// build writes that is not in the corpus, and three words in four languages do
// not want a translation framework over them.
var spanWords = map[string][3]string{
	"en": {"Chapter", "Chapters", "to"},
	"fr": {"Chapitre", "Chapitres", "à"},
	"vi": {"Chương", "Chương", "đến"},
}

// romanValue is the numerals the Elements uses, which stop well short of what a
// general converter would need: the longest Book is Integration at nine
// chapters and the deepest numeral anywhere is X.
var romanValue = map[byte]int{'I': 1, 'V': 5, 'X': 10, 'L': 50, 'C': 100}

// arabic turns a chapter numeral into the figure the cover prints. A numeral it
// cannot read comes back unchanged, since a cover with III on it is better than
// a cover with an error on it.
func arabic(numeral string) string {
	s := strings.ToUpper(strings.TrimSpace(numeral))
	if s == "" {
		return numeral
	}
	total := 0
	for i := 0; i < len(s); i++ {
		v, ok := romanValue[s[i]]
		if !ok {
			return numeral
		}
		if i+1 < len(s) {
			if next, ok := romanValue[s[i+1]]; ok && next > v {
				total -= v
				continue
			}
		}
		total += v
	}
	if total == 0 {
		return numeral
	}
	return fmt.Sprint(total)
}
