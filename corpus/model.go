package corpus

import "fmt"

// The Éléments are organised Book, then Chapter, then §, then no., then the
// statements inside a no. Bourbaki numbers the § within a chapter, the no.
// within a §, and most statements consecutively within a § rather than within a
// no., which is why a Ref carries a section and not a subsection.
//
// The types here are the shape of that hierarchy as the table of contents gives
// it. They carry both numberings: Page is what the book prints, PDFPage is
// where it sits in the file. The second comes from the page map, and it is what
// lets extraction cut the volume into sections without anybody counting pages
// by hand.

// Chapter is one chapter of a volume.
type Chapter struct {
	Book     string    `yaml:"book"`
	Numeral  string    `yaml:"chapter"`
	Title    string    `yaml:"title"`
	Page     int       `yaml:"page"`
	PDFPage  int       `yaml:"pdf_page"`
	Sections []Section `yaml:"sections"`

	// Historical is where the chapter's Historical Note starts. Chapters I to
	// III each carry one; chapter VIII carries none, so it stays nil.
	Historical *Locator `yaml:"historical_note,omitempty"`

	// Exercises is where the chapter's exercises begin, for the volumes that
	// print one run for the whole chapter rather than one per §. Topologie
	// algebrique does this: six §§ and then a single "Exercices" at page 139.
	// Where the volume prints them per §, this stays nil and each Section
	// carries its own.
	Exercises *Locator `yaml:"exercises,omitempty"`
}

// Section is a § of a chapter.
type Section struct {
	Number      int          `yaml:"section"`
	Title       string       `yaml:"title"`
	Page        int          `yaml:"page"`
	PDFPage     int          `yaml:"pdf_page"`
	Subsections []Subsection `yaml:"subsections,omitempty"`

	// Appendix is set when this is an Appendix rather than a §. Chapters II
	// and III each close with one unnumbered Appendix, and chapter VIII with
	// four numbered ones, all of which carry no. and exercises exactly as a §
	// does. Number is the appendix's own number, or 0 when it has none.
	Appendix bool `yaml:"appendix,omitempty"`

	// Exercises is where this §'s exercises begin. The three volumes put them
	// in three places: chapter VIII prints them at the end of each §, chapters
	// I to III gather them after § 10 as "Exercises for § 1" and so on, and
	// chapters IV to VII gather them as "Exercises on § 1". All three end up
	// here.
	Exercises *Locator `yaml:"exercises,omitempty"`
}

// Subsection is a no. of a §.
type Subsection struct {
	Number  int    `yaml:"no"`
	Title   string `yaml:"title"`
	Page    int    `yaml:"page"`
	PDFPage int    `yaml:"pdf_page"`
}

// Locator is a place in a volume, in both numberings.
type Locator struct {
	Page    int `yaml:"page"`
	PDFPage int `yaml:"pdf_page"`
}

// Statement is one numbered or unnumbered assertion in the text. The body is
// held as Markdown with LaTeX mathematics, exactly as it is committed.
type Statement struct {
	Ref     Ref
	Tag     string
	Page    int
	PDFPage int
	Body    string
}

// Label is the permanent label this statement's tag points at.
func (s Statement) Label() string { return s.Ref.Label() }

// Exercise is one exercise. Bourbaki gives no difficulty rating, unlike TAOCP,
// but it does mark the harder ones with a pilcrow and sets some in small type
// as supplementary, so those two facts are what there is to record.
type Exercise struct {
	Meta ExerciseFrontMatter
	Body string
	// Pages is every PDF page the exercise is printed on, in order. Meta
	// records only the first, which is the one a reader cites; this is what
	// assembly needs to know which of the volume's footnotes were printed
	// under the exercise. It is not written to the file.
	Pages []int
}

// Ref of an exercise, so that it labels and tags like any other statement.
func (e Exercise) Ref() Ref {
	return Ref{
		Book:     e.Meta.Book,
		Chapter:  e.Meta.Chapter,
		Section:  e.Meta.Section,
		Appendix: e.Meta.Appendix,
		Kind:     KindExercise,
		Number:   e.Meta.Exercise,
	}
}

// SectionRange is the span of pages a § occupies, worked out from where the
// next § starts. The table of contents gives only starts, so ends are derived.
func (c Chapter) SectionRange(section int) (first, last int, err error) {
	for i, s := range c.Sections {
		if s.Number != section {
			continue
		}
		first = s.PDFPage
		if i+1 < len(c.Sections) {
			return first, c.Sections[i+1].PDFPage - 1, nil
		}
		// The last § of a chapter runs to whatever comes after it, which is
		// the exercises when they are gathered at the end, or the historical
		// note, or the end of the chapter. The caller knows the chapter's last
		// page; here we can only say the § starts and does not end before it.
		if c.Historical != nil && c.Historical.PDFPage > first {
			return first, c.Historical.PDFPage - 1, nil
		}
		if s.Exercises != nil && s.Exercises.PDFPage > first {
			return first, s.Exercises.PDFPage - 1, nil
		}
		return first, 0, nil
	}
	return 0, 0, fmt.Errorf("chapter %s has no § %d", c.Numeral, section)
}

// Get returns the § with this number. An appendix is never returned, even when
// its number matches, because "Exercises for § 1" means the §.
func (c *Chapter) Get(section int) (*Section, bool) {
	for i := range c.Sections {
		if c.Sections[i].Appendix {
			continue
		}
		if c.Sections[i].Number == section {
			return &c.Sections[i], true
		}
	}
	return nil, false
}
