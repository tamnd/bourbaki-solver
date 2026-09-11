package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"gopkg.in/yaml.v3"
)

// SectionsManifest is manifests/sections/, one line of accounting for every
// file assembly wrote: which PDF pages went into it, how much came out, and
// what the bytes hash to.
//
// It is written for the reader rather than for the program, which is why it
// holds counts and not content. Assembly needs nothing from it, since it works
// from the pages and the table of contents. What it answers is the question
// nobody can answer by looking at a directory of 26 Markdown files: whether
// every page of the volume ended up in exactly one of them, and whether the
// statements and exercises the corpus claims are the ones the book prints.
//
// It is one file to a volume, for the reason manifests/toc/ is: it was one file
// to the corpus until it ran to 574 KB, over the size H03 holds a tracked file
// to, and its size is a linear function of how much of the Éléments has been
// assembled. A record arrives in it as a volume is assembled, and a single file
// would have gone on growing past any bound the rule was given. Per volume it
// does not grow; the largest of them is a few tens of KB.
//
// Splitting it also made the write narrow. Assembling one volume used to
// rewrite every volume's accounting, because the manifest was rendered whole,
// so a run that touched Algebra I to III showed up in the diff as a change to
// all forty. Now it writes the one file it has something to say about.
type SectionsManifest struct {
	Books []BookSections `yaml:"books"`
}

// BookSections is one volume, and one file under manifests/sections/.
type BookSections struct {
	ID string `yaml:"id"`
	// Introduction is the Book's own introduction, where the volume has one. It
	// is beside the chapters rather than in them because it is in no chapter,
	// and it is accounted for like any other file: a part of the book that is
	// written and not counted is a part nobody notices is missing.
	// ReaderNote is the publisher's note to the reader, where the volume has
	// one. It stands ahead of the introduction, which is where the printing
	// puts it.
	ReaderNote   *SectionRecord    `yaml:"reader_note,omitempty"`
	Introduction *SectionRecord    `yaml:"introduction,omitempty"`
	Chapters     []ChapterSections `yaml:"chapters"`
	// NotationIndex and TerminologyIndex are the two lists the printing sets
	// after the last chapter. They come after Chapters here because they come
	// after the chapters in the book, and they are counted like anything else:
	// the index of terminology of the English Algebra I to III is 33 pages, and
	// 33 pages of a volume that nothing accounts for is 33 pages nobody notices
	// are missing.
	NotationIndex    *SectionRecord `yaml:"notation_index,omitempty"`
	TerminologyIndex *SectionRecord `yaml:"terminology_index,omitempty"`
}

// ChapterSections is one chapter.
type ChapterSections struct {
	Chapter  string          `yaml:"chapter"`
	Title    string          `yaml:"title"`
	Sections []SectionRecord `yaml:"sections"`
}

// SectionRecord is one assembled file.
type SectionRecord struct {
	// Kind is front, section, appendix or historical. The chapter's opening
	// pages and its historical note are files like any other and are counted
	// like any other, but neither is a §, so neither has a number.
	Kind          string `yaml:"kind"`
	Section       int    `yaml:"section,omitempty"`
	Title         string `yaml:"title,omitempty"`
	Path          string `yaml:"path"`
	Label         string `yaml:"label,omitempty"`
	FirstPDFPage  int    `yaml:"first_pdf_page"`
	LastPDFPage   int    `yaml:"last_pdf_page"`
	BookPages     string `yaml:"book_pages,omitempty"`
	Subsections   int    `yaml:"subsections"`
	Statements    int    `yaml:"statements"`
	Exercises     int    `yaml:"exercises"`
	Extraction    string `yaml:"extraction"`
	ContentSHA256 string `yaml:"content_sha256"`
}

// Section kinds.
const (
	KindFront = "front"
	// KindIntroduction is the Book's own introduction, which stands before
	// chapter I and belongs to no chapter. It is the one kind whose file is not
	// in a chapter directory. See Book.Introduction.
	KindIntroduction = "introduction"
	// KindReader is the publisher's note to the reader, printed at the front of
	// nearly every volume of the series under a heading like TO THE READER or
	// MODE D'EMPLOI DE CE TRAITE. Like an introduction it belongs to no
	// chapter. See Book.ReaderNote.
	KindReader     = "reader"
	KindSection    = "section"
	KindAppendix   = "appendix"
	KindHistorical = "historical"
	// KindNotation and KindTerminology are the volume's two indexes, which
	// stand after the last chapter and belong to no chapter, the same way an
	// introduction stands before the first and belongs to none.
	//
	// They are worth having as text and not only as pictures of pages because
	// of how the printing writes them. "Abelian group: I, § 4, no. 2" names a
	// chapter, a § and a numbered subsection, not a page, so every line of both
	// indexes is a reference into a structure this corpus already has, and a
	// rebuilt book can carry the index through unchanged and check every line
	// of it against itself. An index of pages could not survive a rebuild,
	// since the rebuilt volume paginates its own way; an index of nos. is the
	// same index in any setting of the same text.
	KindNotation    = "notation"
	KindTerminology = "terminology"
)

// Chapterless says whether a file of this kind belongs to no chapter.
//
// Four kinds do: the note to the reader and the Book's introduction at the
// front, and the index of notation and the index of terminology at the back.
// They sit beside the chapter directories rather than in one, and an empty
// chapter in their front matter is what says so rather than a field somebody
// forgot, so every rule that reads the chapter of a file has to know about
// them. Written once here because the list grew from two to four and the two
// places that had it spelled out inline would each have been a rule that
// quietly refused every index in the corpus.
func Chapterless(kind string) bool {
	switch kind {
	case KindIntroduction, KindReader, KindNotation, KindTerminology:
		return true
	}
	return false
}

// LoadSections reads manifests/sections/. A missing directory is an empty
// manifest, so the first assemble works on a fresh repo.
//
// The volumes come back in the order the file names sort, which is the order
// their ids sort, and not the order of manifests/books.yaml. Nothing reads the
// manifest in order, and a stable order is what keeps an assemble of one volume
// from rewriting the rest.
func LoadSections(root string) (*SectionsManifest, error) {
	dir := SectionsDir(root)
	ents, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return &SectionsManifest{}, nil
	}
	if err != nil {
		return nil, err
	}
	m := &SectionsManifest{}
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" {
			continue
		}
		path := filepath.Join(dir, e.Name())
		b, err := os.ReadFile(path)
		if err != nil {
			return nil, err
		}
		var bs BookSections
		if err := yaml.Unmarshal(b, &bs); err != nil {
			return nil, fmt.Errorf("%s: %w", path, err)
		}
		m.Books = append(m.Books, bs)
	}
	sort.Slice(m.Books, func(i, j int) bool { return m.Books[i].ID < m.Books[j].ID })
	return m, nil
}

// Get is the volume's committed record, and whether there is one.
//
// Assembly writes this manifest and does not read it to do its work, so for a
// long time nothing needed this. What needs it is a partial assemble, which has
// to know what the last full one recorded for a chapter it is not assembling
// this time, so that skipping a chapter does not take that chapter out of the
// manifest.
func (m *SectionsManifest) Get(id string) (BookSections, bool) {
	for _, b := range m.Books {
		if b.ID == id {
			return b, true
		}
	}
	return BookSections{}, false
}

// Upsert replaces a volume's record, or appends it, leaving the order of the
// other volumes alone.
func (m *SectionsManifest) Upsert(b BookSections) {
	for i := range m.Books {
		if m.Books[i].ID == b.ID {
			m.Books[i] = b
			return
		}
	}
	m.Books = append(m.Books, b)
}

// Bytes renders one volume's accounting, which is one file of the manifest.
func (b BookSections) Bytes() ([]byte, error) {
	enc, err := yaml.Marshal(b)
	if err != nil {
		return nil, err
	}
	return append([]byte("# Generated by bourbaki assemble. Do not edit.\n"), enc...), nil
}

// Save writes the manifest back, one file to a volume.
//
// A volume that is no longer in the manifest has its file removed, because the
// directory is the manifest and a file left behind in it would be read back on
// the next load as a volume that is still there.
func (m *SectionsManifest) Save(root string) error {
	dir := SectionsDir(root)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	keep := map[string]bool{}
	for _, b := range m.Books {
		buf, err := b.Bytes()
		if err != nil {
			return err
		}
		name := b.ID + ".yaml"
		keep[name] = true
		if err := writeIfChanged(filepath.Join(dir, name), buf); err != nil {
			return err
		}
	}
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}
	for _, e := range ents {
		if e.IsDir() || filepath.Ext(e.Name()) != ".yaml" || keep[e.Name()] {
			continue
		}
		if err := os.Remove(filepath.Join(dir, e.Name())); err != nil {
			return err
		}
	}
	return nil
}

// SectionsDir is where the manifest lives inside a corpus checkout.
func SectionsDir(root string) string { return filepath.Join(root, "manifests", "sections") }

// SectionsPath is the file one volume's accounting is written to.
func SectionsPath(root, id string) string { return filepath.Join(SectionsDir(root), id+".yaml") }
