package refs

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// Index is what a citation is looked up in: the sections of the corpus with the
// pages the book prints them on, and every statement and exercise in them.
//
// It is built from the committed Markdown and the manifests and never from a
// PDF, for the same reason assembly is: this has to run in CI, where there are
// no PDFs.
type Index struct {
	Sections []Section
	byLabel  map[string]*Section
	stmt     map[stmtKey][]*Statement
	byStmt   map[string]*Statement
	tagOf    map[string]string
}

// Section is one § or Appendix, with the run of printed pages it occupies.
type Section struct {
	Label   string
	Chapter string
	First   int // the first page the book prints it on, within the chapter
	Last    int
	Subsecs []Subsec
	// Exercises is how many the § has, which is all that is needed to say
	// whether Exercise 9 of this § exists.
	Exercises  int
	Statements []*Statement
}

// Subsec is a no. and the page it starts on.
type Subsec struct {
	No   int
	Page int
}

// Statement is one tagged unit.
type Statement struct {
	Label  string
	Tag    string
	Kind   corpus.Kind
	Number int
	Subsec int // the no. it stands in, 0 when the label does not say
	Line   int
	Path   string

	// Named is whether the label carries the statement this one hangs from,
	// which for a corollary it does when the book numbered it. An unnumbered
	// corollary is named by its no. instead, on purpose, so that the second
	// unnumbered corollary of a theorem and its Corollary 2 do not collide.
	Named bool
	// FollowsKind and FollowsNumber are the last numbered statement printed
	// above this one, which is what "the corollary of Proposition 3" means when
	// the corollary itself has no number to be looked up by.
	FollowsKind   corpus.Kind
	FollowsNumber int
}

type stmtKey struct {
	section string
	kind    corpus.Kind
	number  int
}

var (
	pageRangeRE = regexp.MustCompile(`\bA\s+([IVX]+)\.(\d+)`)
	statementRE = regexp.MustCompile(`\{#([a-z0-9-]+) \.statement(?: tag=([0-9A-Z]{4}))?\}`)
)

// Load builds the index for one language of the corpus.
func Load(root, lang string) (*Index, error) {
	sections, err := corpus.LoadSections(root)
	if err != nil {
		return nil, err
	}
	ex, err := corpus.LoadExercises(root)
	if err != nil {
		return nil, err
	}
	counts := map[string]int{}
	for _, b := range ex.Books {
		for _, ch := range b.Chapters {
			for _, s := range ch.Section {
				counts[s.Label] = s.Count
			}
		}
	}
	ix := &Index{byLabel: map[string]*Section{}, stmt: map[stmtKey][]*Statement{},
		byStmt: map[string]*Statement{}, tagOf: map[string]string{}}
	for _, b := range sections.Books {
		for _, ch := range b.Chapters {
			for _, rec := range ch.Sections {
				if rec.Label == "" {
					continue // front matter, the historical note, the index
				}
				s, err := section(root, lang, rec)
				if err != nil {
					return nil, err
				}
				s.Exercises = counts[rec.Label]
				ix.Sections = append(ix.Sections, *s)
			}
		}
	}
	ix.index()
	return ix, nil
}

// index wires the lookups up from the sections. It is separate from Load
// because a test builds its sections by hand and needs the same wiring.
func (ix *Index) index() {
	if ix.byLabel == nil {
		ix.byLabel, ix.stmt = map[string]*Section{}, map[stmtKey][]*Statement{}
		ix.byStmt, ix.tagOf = map[string]*Statement{}, map[string]string{}
	}
	for i := range ix.Sections {
		s := &ix.Sections[i]
		ix.byLabel[s.Label] = s
		for _, st := range s.Statements {
			ix.stmt[stmtKey{s.Label, st.Kind, st.Number}] = append(ix.stmt[stmtKey{s.Label, st.Kind, st.Number}], st)
			ix.byStmt[st.Label] = st
			ix.tagOf[st.Label] = st.Tag
		}
	}
}

// section reads one section file into the index.
//
// A statement's no. is taken from the subsection heading above it and not from
// its label, because the label of a Corollary says which statement it hangs
// from and not which no. it stands in, and narrowing a Corollary down to a no.
// is exactly what a page-based citation to one needs.
func section(root, lang string, rec corpus.SectionRecord) (*Section, error) {
	path := filepath.Join(root, filepath.FromSlash(rec.Path))
	if lang != "en" {
		path = strings.Replace(path, "/content/en/", "/content/"+lang+"/", 1)
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	f, err := corpus.ParseFile[corpus.SectionFrontMatter](b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rec.Path, err)
	}
	s := &Section{Label: rec.Label, Chapter: f.Meta.Chapter}
	if m := pageRangeRE.FindAllStringSubmatch(rec.BookPages, -1); len(m) > 0 {
		s.First, _ = strconv.Atoi(m[0][2])
		s.Last, _ = strconv.Atoi(m[len(m)-1][2])
	}
	for _, sub := range f.Meta.Subsections {
		s.Subsecs = append(s.Subsecs, Subsec{No: sub.Number, Page: sub.Page})
	}
	sort.Slice(s.Subsecs, func(i, j int) bool { return s.Subsecs[i].No < s.Subsecs[j].No })

	offset := strings.Count(strings.TrimSuffix(string(b), f.Body), "\n")
	no := 0
	var follows corpus.Ref
	for i, line := range strings.Split(f.Body, "\n") {
		if n, ok := subsecHeading(line); ok {
			no = n
			continue
		}
		m := statementRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		r, err := corpus.ParseLabel(m[1])
		if err != nil {
			continue // the Exercises heading and anything else that is not a statement
		}
		st := &Statement{
			Label: m[1], Tag: m[2], Kind: r.Kind, Number: r.Number,
			Subsec: no, Line: offset + i + 1, Path: rec.Path,
			Named:       r.ParentNumber != 0,
			FollowsKind: follows.Kind, FollowsNumber: follows.Number,
		}
		s.Statements = append(s.Statements, st)
		if r.Number != 0 && r.Kind.Scope() != corpus.ScopeParent {
			follows = r
		}
	}
	return s, nil
}

var subsecRE = regexp.MustCompile(`^### (\d+)\.`)

func subsecHeading(line string) (int, bool) {
	m := subsecRE.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	n, _ := strconv.Atoi(m[1])
	return n, true
}

// SectionAt is the § the book prints on a page of the chapter.
//
// The page ranges of chapter VIII do not overlap: of the 467 pages the sections
// span, 456 fall in exactly one and 11 in none, those being the leaves between
// one § and the next. So this returns at most one section, and the ambiguity
// spec 05 §4 anticipated is not one this chapter has. A chapter that does have
// it will come back through here as two candidates and be reported.
func (ix *Index) SectionAt(chapter string, page int) []*Section {
	var out []*Section
	for i := range ix.Sections {
		s := &ix.Sections[i]
		if s.Chapter == chapter && s.First <= page && page <= s.Last {
			out = append(out, s)
		}
	}
	return out
}

// Statements is every statement of a § with this kind and number. More than one
// means the kind is not numbered straight through the § and the no. has to
// settle it.
func (ix *Index) Statements(section string, k corpus.Kind, n int) []*Statement {
	return ix.stmt[stmtKey{section, k, n}]
}

// Section looks one up by label.
func (ix *Index) Section(label string) *Section { return ix.byLabel[label] }

// Statement looks one up by label, which is what a citation that names a
// corollary by its parent can do without any search at all.
func (ix *Index) Statement(label string) *Statement { return ix.byStmt[label] }

// Tag is the permanent tag of a label, empty if it has none.
func (ix *Index) Tag(label string) string { return ix.tagOf[label] }

// SubsecAt is the no. a page of a § falls in. The subsections are listed with
// the page each starts on, so the last one that starts at or before the page is
// the one the page is in.
func (s *Section) SubsecAt(page int) int {
	no := 0
	for _, sub := range s.Subsecs {
		if sub.Page <= page {
			no = sub.No
		}
	}
	return no
}
