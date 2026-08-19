package corpus

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"gopkg.in/yaml.v3"
)

// BooksManifest is manifests/books.yaml, the record of which PDFs the corpus
// was built from.
type BooksManifest struct {
	Books []Book `yaml:"books"`
}

// Book is one source volume.
type Book struct {
	ID   string `yaml:"id"`
	Book string `yaml:"book"`
	// Lang is the language of this printing. Most Books of the Éléments exist
	// in both, and four of them exist in French only, so the language is part
	// of what a volume is rather than something to read off the file name.
	Lang       string   `yaml:"lang"`
	Title      string   `yaml:"title"`
	Edition    string   `yaml:"edition"`
	Chapters   []string `yaml:"chapters"`
	PDF        string   `yaml:"pdf"`
	PDFSHA256  string   `yaml:"pdf_sha256"`
	PDFBytes   int64    `yaml:"pdf_bytes"`
	Pages      int      `yaml:"pages"`
	Nature     string   `yaml:"nature"`
	Producer   string   `yaml:"producer,omitempty"`
	Scan       *Scan    `yaml:"scan,omitempty"`
	Extraction string   `yaml:"extraction"`
	// TextLayer is what the file's own text is worth: "native" in a born-digital
	// volume, "ocr" in a scan somebody has already read, "none" in a scan that
	// carries no text at all. The last of those cannot be paged without vision
	// OCR first, which is a different order of work from the other two.
	TextLayer  string  `yaml:"text_layer,omitempty"`
	PageWidth  float64 `yaml:"page_width_pt,omitempty"`
	PageHeight float64 `yaml:"page_height_pt,omitempty"`
	// Grammar is how the volume prints its page number, "head-label" or
	// "foot-number", and Pagination is what that number counts, "per-chapter"
	// or "continuous". Both are filled in by pagemap build, which detects them
	// and records what it found rather than asking anyone to remember.
	Grammar    string `yaml:"grammar,omitempty"`
	Pagination string `yaml:"pagination,omitempty"`
	// FirstPage is the printed page the PDF's first page carries, for a scan
	// that does not begin at the beginning of the volume, and is empty for one
	// that does. Fonctions d'une variable reelle in French opens on the half
	// title with FVR I.3 on it: the two leaves before it were never scanned.
	// A per-chapter volume numbers each chapter from 1 and a chapter that does
	// not is normally a fit that has slipped, so the page map refuses one, and
	// this is how a volume says which of the two it is. It is written by hand
	// after reading the printing, not detected, because a missing leaf and a
	// wrong offset look the same in the file.
	FirstPage int `yaml:"first_page,omitempty"`
	// Introduction is the Book's own introduction, where it has one, and is
	// empty where it has not.
	Introduction *Introduction `yaml:"introduction,omitempty"`
}

// Introduction is the pages of a Book's introduction.
//
// It stands before chapter I and belongs to no chapter, which is why it is here
// and not in the table of contents. The table of contents is what assembly
// walks and every entry in it is a chapter with numbered sections; the seven
// pages Theory of Sets opens with are neither. They are also not front matter
// in the sense the half title and the table of contents are: it is Bourbaki
// writing about what a proof is and why the Elements is formalized the way it
// is, referred to from the body, and a corpus of this Book without it is a
// corpus missing the part that says what the rest is for.
//
// The pages are given rather than found. A heading reading INTRODUCTION is also
// the running head of every page after the first, so looking for one finds
// seven beginnings, and the end of it is the beginning of chapter I, which the
// table of contents already names. Two numbers said once are what the volume
// knows about itself.
type Introduction struct {
	// Title is the heading the printing gives it, which is INTRODUCTION in the
	// English of Theory of Sets and Introduction in a French volume that sets
	// it that way. It is written at the top of the assembled file, so it is
	// what a reader sees, and a translation renders it like any other heading.
	Title string `yaml:"title"`
	// Page is the printed number of the first page, for the record and for the
	// front matter of the file. The last is worked out from the pages.
	Page         int `yaml:"page"`
	FirstPDFPage int `yaml:"first_pdf_page"`
	LastPDFPage  int `yaml:"last_pdf_page"`
}

// bookTitles is what the Éléments call their Books, as against what a publisher
// called a volume. Algebra is one Book printed as three volumes, and a section
// belongs to the Book, so this is what its front matter records. There is no
// field in books.yaml for it because a volume knows its own title and not the
// Book's, and inventing one would mean writing "Algebra" into the manifest
// three times and hoping the three stayed the same.
//
// The names are English where there is an English printing to take a name from.
// Variétés, Théories spectrales and Topologie algébrique were never translated,
// so French is their name of record and not a placeholder for one.
var bookTitles = map[string]string{
	"ens":  "Theory of Sets",
	"alg":  "Algebra",
	"top":  "General Topology",
	"fvr":  "Functions of a Real Variable",
	"evt":  "Topological Vector Spaces",
	"int":  "Integration",
	"ac":   "Commutative Algebra",
	"var":  "Variétés différentielles et analytiques",
	"lie":  "Lie Groups and Lie Algebras",
	"ts":   "Théories spectrales",
	"ta":   "Topologie algébrique",
	"hist": "Elements of the History of Mathematics",
}

// bookLetters is the abbreviation each Book of the Éléments is cited by, and it
// is not invented here. Page vii of every recent volume prints the table under
// item 3 of the Mode d'emploi, "Théorie des ensembles désigné par E", "Algèbre —
// A", and so on down to "Topologie algébrique — TA", and this is that table.
//
// It is what stands in front of a page label. The label of the twenty fourth
// page of chapter I is "A I.24" in Algebra and "E I.24" in Theory of Sets, and
// the two are different pages of different Books, so a label that spelled both
// "A I.24" would be a collision rather than a shorthand. Elements of the History
// of Mathematics is not one of the Books and is cited by name, so it has no
// letter here and gets none.
var bookLetters = map[string]string{
	"ens": "E",
	"alg": "A",
	"top": "TG",
	"fvr": "FVR",
	"evt": "EVT",
	"int": "INT",
	"ac":  "AC",
	"var": "VAR",
	"lie": "LIE",
	"ts":  "TS",
	"ta":  "TA",
}

// BookLetter is the abbreviation a Book is cited by, or "" for something the
// Éléments do not abbreviate. A caller with no letter has no page label to
// build and should leave the label empty rather than make one up.
func BookLetter(book string) string { return bookLetters[book] }

// BookLetters is every one of those abbreviations, longest first. It is what
// says a run of capitals in a running head opens a page label rather than a
// title: the head of Topologie algébrique reads "TA I.144 EXERCICES", and the
// two letters in front of the numeral are the Book and not the last word of the
// title.
//
// Longest first because a reader matching them in turn has to be offered TA
// before A, or the head of Topologie algébrique reads as Algebra.
func BookLetters() []string {
	out := make([]string, 0, len(bookLetters))
	for _, l := range bookLetters {
		out = append(out, l)
	}
	sort.Slice(out, func(i, j int) bool {
		if len(out[i]) != len(out[j]) {
			return len(out[i]) > len(out[j])
		}
		return out[i] < out[j]
	})
	return out
}

// bookOrder is the order the Éléments print their Books in, which is the order
// a reader expects to find them shelved and is not alphabetical. A slug nobody
// has numbered here sorts after the ones that are.
var bookOrder = map[string]int{
	"ens": 1, "alg": 2, "top": 3, "fvr": 4, "evt": 5, "int": 6,
	"ac": 7, "var": 8, "lie": 9, "ts": 10, "ta": 11, "hist": 12,
}

// BookTitle is the printed name of a Book of the Éléments. An id nobody has
// named yet is returned as it stands, so adding a volume of a new Book does not
// have to wait on this map.
func BookTitle(book string) string {
	if t, ok := bookTitles[book]; ok {
		return t
	}
	return book
}

// Scan describes the page images of a scanned volume.
type Scan struct {
	Format string `yaml:"format"`
	Width  int    `yaml:"width"`
	Height int    `yaml:"height"`
	DPI    int    `yaml:"dpi"`
	BPC    int    `yaml:"bpc"`
	Color  string `yaml:"color"`
}

// LoadBooks reads manifests/books.yaml under root. A missing file is not an
// error, it is an empty manifest, so the first books add works on a fresh repo.
func LoadBooks(root string) (*BooksManifest, error) {
	path := BooksPath(root)
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return &BooksManifest{}, nil
	}
	if err != nil {
		return nil, err
	}
	var m BooksManifest
	if err := yaml.Unmarshal(b, &m); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	return &m, nil
}

// Save writes the manifest back, shelved rather than sorted by id, so the file
// is stable across runs and diffs stay readable.
func (m *BooksManifest) Save(root string) error {
	sort.SliceStable(m.Books, func(i, j int) bool {
		return m.Books[i].sortKey().less(m.Books[j].sortKey())
	})
	var buf []byte
	enc, err := yaml.Marshal(m)
	if err != nil {
		return err
	}
	buf = append([]byte("# Generated by bourbaki books add. Edit the PDFs, not this file.\n"), enc...)
	path := BooksPath(root)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	return os.WriteFile(path, buf, 0o644)
}

// shelf is where a volume stands: which Book of the Éléments, which printing of
// it, and which chapters. Sorting by id would put Algèbre commutative chapter 10
// before chapters 1 to 4, and the whole of Commutative Algebra before Algebra.
type shelf struct{ book, lang, chapter int }

func (s shelf) less(o shelf) bool {
	if s.book != o.book {
		return s.book < o.book
	}
	if s.lang != o.lang {
		return s.lang < o.lang
	}
	return s.chapter < o.chapter
}

// sortKey shelves a volume by Book in the order the Éléments print them,
// English before French because English is the edition of record wherever one
// exists, then by first chapter. A volume with no chapters, such as the history
// or the fascicule de résultats, stands at the end of its own Book.
func (b Book) sortKey() shelf {
	s := shelf{book: 1 << 30, lang: 2, chapter: 1 << 30}
	if n, ok := bookOrder[b.Book]; ok {
		s.book = n
	}
	switch b.Lang {
	case "en":
		s.lang = 0
	case "fr":
		s.lang = 1
	}
	if len(b.Chapters) > 0 {
		if n, err := RomanOrder(b.Chapters[0]); err == nil {
			s.chapter = n
		}
	}
	return s
}

// Get returns the book with the given id.
func (m *BooksManifest) Get(id string) (*Book, bool) {
	for i := range m.Books {
		if m.Books[i].ID == id {
			return &m.Books[i], true
		}
	}
	return nil, false
}

// Upsert replaces the book with the same id, or appends it.
func (m *BooksManifest) Upsert(b Book) {
	for i := range m.Books {
		if m.Books[i].ID == b.ID {
			m.Books[i] = b
			return
		}
	}
	m.Books = append(m.Books, b)
}

// Printings are the languages the corpus holds volumes in.
//
// That is not the same list as the languages there is content in. A printing is
// the book itself, set by Hermann or by Springer and read out of a PDF; every
// other language here is a translation this corpus made. The difference is what
// tagging turns on: a statement is tagged off a printing, because a printing is
// where a statement of the Éléments is to be found, and a translation reuses the
// tag of the printing it was made from. Théories spectrales and Topologie
// algébrique were never translated into English, so their statements are tagged
// off the French, which is the only printing of them there is.
//
// English comes first and the rest in alphabetical order. That is not a
// preference between two printings of one thing, it is the order the tags were
// handed out in and it has to stay fixed: the English Algebra VIII was read
// first and its tags are permanent, so it goes on being read first.
func (m *BooksManifest) Printings() []string {
	var out []string
	for _, b := range m.Books {
		if b.Lang != "" && !slices.Contains(out, b.Lang) {
			out = append(out, b.Lang)
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i] == "en") != (out[j] == "en") {
			return out[i] == "en"
		}
		return out[i] < out[j]
	})
	return out
}

// Pages is the total page count across all volumes.
func (m *BooksManifest) Pages() int {
	n := 0
	for _, b := range m.Books {
		n += b.Pages
	}
	return n
}

// BooksPath is where the manifest lives inside a corpus checkout.
func BooksPath(root string) string { return filepath.Join(root, "manifests", "books.yaml") }

// Root resolves the corpus checkout from BOURBAKI_CORPUS, falling back to the
// working directory.
func Root() (string, error) {
	if r := os.Getenv("BOURBAKI_CORPUS"); r != "" {
		return filepath.Abs(r)
	}
	return os.Getwd()
}
