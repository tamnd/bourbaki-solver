package corpus

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// SectionsManifest is manifests/sections.yaml, one line of accounting for every
// file assembly wrote: which PDF pages went into it, how much came out, and
// what the bytes hash to.
//
// It is written for the reader rather than for the program, which is why it
// holds counts and not content. Assembly needs nothing from it, since it works
// from the pages and the table of contents. What it answers is the question
// nobody can answer by looking at a directory of 26 Markdown files: whether
// every page of the volume ended up in exactly one of them, and whether the
// statements and exercises the corpus claims are the ones the book prints.
type SectionsManifest struct {
	Books []BookSections `yaml:"books"`
}

// BookSections is one volume.
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

// LoadSections reads manifests/sections.yaml. A missing file is an empty
// manifest, so the first assemble works on a fresh repo.
func LoadSections(root string) (*SectionsManifest, error) {
	path := SectionsPath(root)
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &SectionsManifest{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m SectionsManifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &m, nil
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

// Bytes renders the manifest.
func (m *SectionsManifest) Bytes() ([]byte, error) {
	enc, err := yaml.Marshal(m)
	if err != nil {
		return nil, err
	}
	return append([]byte("# Generated by bourbaki assemble. Do not edit.\n"), enc...), nil
}

// Save writes the manifest back.
func (m *SectionsManifest) Save(root string) error {
	b, err := m.Bytes()
	if err != nil {
		return err
	}
	path := SectionsPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, b, 0o644)
}

// SectionsPath is where the manifest lives inside a corpus checkout.
func SectionsPath(root string) string {
	return filepath.Join(root, "manifests", "sections.yaml")
}
