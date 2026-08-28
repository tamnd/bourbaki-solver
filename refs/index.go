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
	tagset "github.com/tamnd/bourbaki-solver/tags"
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

// Section is one § or Appendix, with the runs of printed pages it occupies.
type Section struct {
	Label   string
	Book    string // the Book as a label writes it, "alg" or "lie"
	Chapter string
	// Number is the § number, and Appendix says whether that number counts
	// appendices rather than §§. They are here because the other citation
	// printing names a § by its number and not by a page it is printed on.
	Number   int
	Appendix bool
	// Runs are the stretches of printed pages the § occupies, in order. Nearly
	// every § is one run. A § of Lie 7 to 9 is two, because that volume gathers
	// a chapter's exercises at its end: § 1 of chapter VIII is printed on pages
	// 69 to 84 and again on 218 to 226, with twelve other §§ in between. Holding
	// the outer bounds instead would put every page of the chapter in every § at
	// once, and no citation to a page would resolve.
	Runs    []Run
	Subsecs []Subsec
	// Exercises is how many the § has, which is all that is needed to say
	// whether Exercise 9 of this § exists.
	Exercises  int
	Statements []*Statement
}

// Run is a stretch of printed pages, both ends included.
type Run struct{ First, Last int }

// pageRuns reads the printed pages a § occupies out of its front matter.
//
// The front matter writes one range to a run and separates the runs with a
// comma: "A VIII.69-A VIII.84, A VIII.218-A VIII.226" for a volume that labels
// its pages, "15-23, 56" for one that numbers them straight through. A run of a
// single page is written once and not twice, "56" and not "56-56", and it is a
// real run: the exercises of a § of Theory of Sets are often one page, and
// dropping them would put a reference to that page in no § at all.
//
// The two shapes are read one at a time and per run rather than by one pattern
// over the whole string. A label carries a number inside it, so a pattern that
// also took bare numbers would read "A VIII.69" as pages 8 and 69 and put every
// § of the chapter on page 8.
func pageRuns(pages string) []Run {
	var out []Run
	for _, run := range strings.Split(pages, ",") {
		m := pageRangeRE.FindAllStringSubmatch(run, -1)
		col := 2
		if len(m) == 0 {
			m, col = folioRangeRE.FindAllStringSubmatch(run, -1), 1
		}
		switch len(m) {
		case 0:
		case 1:
			n, _ := strconv.Atoi(m[0][col])
			out = append(out, Run{First: n, Last: n})
		default:
			first, _ := strconv.Atoi(m[0][col])
			last, _ := strconv.Atoi(m[len(m)-1][col])
			out = append(out, Run{First: first, Last: last})
		}
	}
	return out
}

// Holds reports whether the page falls in any run of the section.
func (s *Section) Holds(page int) bool {
	for _, r := range s.Runs {
		if r.First <= page && page <= r.Last {
			return true
		}
	}
	return false
}

// First is the page the § opens on, and Last the page it ends on, which is the
// last page of its last run and not of its first.
func (s *Section) First() int {
	if len(s.Runs) == 0 {
		return 0
	}
	return s.Runs[0].First
}

func (s *Section) Last() int {
	if len(s.Runs) == 0 {
		return 0
	}
	return s.Runs[len(s.Runs)-1].Last
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
	// Page is the page of the chapter the book prints it on, 0 when it could
	// not be placed. It is read back out of pages/ rather than written down in
	// content/, which is what pages.go is for.
	Page int
	Line int
	// Path is the file this statement was read out of, relative to the corpus
	// root and in the language the index was loaded for.
	Path string

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
	// folioRangeRE is the same thing for a volume that numbers its pages
	// straight through and prints the number bare at the foot, where the front
	// matter reads "15-56, 190-201" and there is no label to anchor on.
	folioRangeRE = regexp.MustCompile(`\b(\d+)\b`)
	statementRE  = regexp.MustCompile(`\{#([a-z0-9-]+) \.statement(?: tag=([0-9A-Z]{4}))?\}`)
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
	source, swap := printings(sections, lang)
	counts := map[string]int{}
	for _, b := range ex.Books {
		if !source[b.ID] {
			continue
		}
		for _, ch := range b.Chapters {
			for _, s := range ch.Section {
				counts[s.Label] = s.Count
			}
		}
	}
	ix := &Index{byLabel: map[string]*Section{}, stmt: map[stmtKey][]*Statement{},
		byStmt: map[string]*Statement{}, tagOf: map[string]string{}}
	// A statement carries its tag in the heading the resolver is already reading,
	// but an exercise carries it in the front matter of a file of its own, and
	// opening 317 of them to learn 317 tags is not worth it when tags/tags says
	// the same thing in one file. tags verify is what holds the two together.
	set, err := tagset.Load(root)
	if err != nil {
		return nil, err
	}
	for label, tag := range set.Lookup() {
		if strings.Contains(label, "-ex-") {
			ix.tagOf[label] = string(tag)
		}
	}
	for _, b := range sections.Books {
		if !source[b.ID] {
			continue
		}
		for _, ch := range b.Chapters {
			for _, rec := range ch.Sections {
				if rec.Label == "" {
					continue // front matter, the historical note, the index
				}
				s, err := section(root, lang, rec, swap)
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

// printings is the volumes the index is built over, by id, and whether their
// paths have to have the language swapped into them.
//
// The graph is over one printing, and indexing two is what has to be avoided.
// The corpus holds Algebra VIII twice, in English and in French, and the two
// share their labels and their printed page numbers on purpose, because they
// are the same chapter. Indexing both puts every page of the chapter in two
// sections at once, and a citation to a page then resolves to nothing:
// measured on this corpus, 979 references that resolved stopped resolving the
// day the French was assembled, and the rate fell from 91.7 per cent to 48.9.
//
// Which printing a record belongs to is in its path, which is where assembly
// put it, and not in books.yaml, because the fixtures the tests build have
// sections and no volumes.
//
// There are two ways a language gets into this corpus and they want opposite
// things here. A translation is the English file with the language swapped in
// its path: content/vi holds the same file names as content/en, carrying the
// same labels and the same page ranges, so it is indexed off the English
// records with the path rewritten. A second printing is a volume of its own
// that assembly read out of its own PDF, with its own file names in the
// manifest, and rewriting an English path towards it names a file that does
// not exist.
//
// So the manifest is asked rather than assumed. Where it holds records under
// content/<lang>, those are the printing and their paths stand. Where it holds
// none, the language is a translation and the English records are rewritten.
// This was the second: -lang fr had opened
// content/fr/alg/VIII/01_s1_artinian_modules_and_noetherian_modules.md and
// failed on it since the day the French was assembled, and nothing caught it
// because the audit only ever built the graph over the English.
func printings(sections *corpus.SectionsManifest, lang string) (source map[string]bool, swap bool) {
	own, english := map[string]bool{}, map[string]bool{}
	for _, b := range sections.Books {
		for _, ch := range b.Chapters {
			for _, rec := range ch.Sections {
				if strings.HasPrefix(rec.Path, "content/"+lang+"/") {
					own[b.ID] = true
				}
				if strings.HasPrefix(rec.Path, "content/en/") {
					english[b.ID] = true
				}
			}
		}
	}
	if len(own) > 0 {
		return own, false
	}
	return english, true
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
// A statement records the file it was read out of and not the file the
// manifest named, which for a translation are two different files. The two are
// compared: the rule that reads a bare "Corollary 1" as the nearest one above
// the citing line asks whether the statement and the citation stand in the same
// file, and the citation carries the path the walk gave it, which is the
// translated one. Recording the English path there meant the rule could not
// fire in any language but English.
func section(root, lang string, rec corpus.SectionRecord, swap bool) (*Section, error) {
	rel := rec.Path
	if swap {
		rel = strings.Replace(rel, "content/en/", "content/"+lang+"/", 1)
	}
	b, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rel)))
	if err != nil {
		return nil, err
	}
	f, err := corpus.ParseFile[corpus.SectionFrontMatter](b)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", rel, err)
	}
	s := &Section{Label: rec.Label, Book: f.Meta.Book, Chapter: f.Meta.Chapter,
		Number: f.Meta.Section, Appendix: f.Meta.Appendix}
	s.Runs = pageRuns(rec.BookPages)
	for _, sub := range f.Meta.Subsections {
		s.Subsecs = append(s.Subsecs, Subsec{No: sub.Number, Page: sub.Page})
	}
	sort.Slice(s.Subsecs, func(i, j int) bool { return s.Subsecs[i].No < s.Subsecs[j].No })

	offset := strings.Count(strings.TrimSuffix(string(b), f.Body), "\n")
	no := 0
	var follows corpus.Ref
	var headings []lead
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
			Subsec: no, Line: offset + i + 1, Path: rel,
			Named:       r.ParentNumber != 0,
			FollowsKind: follows.Kind, FollowsNumber: follows.Number,
		}
		s.Statements = append(s.Statements, st)
		headings = append(headings, printedAs(line, r))
		if fathers(r) {
			follows = r
		}
	}
	var leads []lead
	for _, r := range pdfRuns(f.Meta.PDFPages) {
		leads = append(leads, readLeads(root, f.Meta.Source, r.First, r.Last)...)
	}
	if len(leads) > 0 {
		placePages(s, headings, leads)
	}
	return s, nil
}

var stmtHeadingRE = regexp.MustCompile(`^#+ ([A-Za-zé]+)\s*(\d*)`)

// printedAs is a statement as its own Markdown heads it, which is what lines up
// against the leads printed on the pages.
//
// The number here is the one the book printed and not the one in the label. A
// Remark the book set with no number is labelled rem-1, because a no. can hold
// several remarks and each of them needs an address, and the lead on the page
// says only "Remark". Lining the labels up against the leads instead left 49 of
// the chapter's 709 statements unplaced, all of them remarks and examples.
func printedAs(line string, r corpus.Ref) lead {
	m := stmtHeadingRE.FindStringSubmatch(line)
	if m == nil {
		return lead{kind: r.Kind, number: r.Number}
	}
	n, _ := strconv.Atoi(m[2])
	return lead{kind: r.Kind, number: n}
}

var pdfRangeRE = regexp.MustCompile(`^(\d+)-(\d+)$`)

// pdfRuns are the runs of PDF pages a § was assembled from, which is where its
// statement leads are to be read. The front matter writes them as "0228-0245",
// or as "0077-0092, 0226-0234" for a § the volume printed in two places.
func pdfRuns(s string) []Run {
	var out []Run
	for _, part := range strings.Split(s, ",") {
		m := pdfRangeRE.FindStringSubmatch(strings.TrimSpace(part))
		if m == nil {
			continue
		}
		first, _ := strconv.Atoi(m[1])
		last, _ := strconv.Atoi(m[2])
		if first > 0 && last >= first {
			out = append(out, Run{First: first, Last: last})
		}
	}
	return out
}

// fathers says whether a statement is one an unnumbered corollary below it can
// be said to hang from.
//
// The chain used to take any numbered statement, and that is not how the book
// reads. § 20 no. 6 prints Proposition 6, then a Remark, then a Corollary, and
// the corollary is the corollary of Proposition 6: the remark stands between
// them on the page and hangs from nothing. § 1 no. 1 does the same with a Lemma
// 2 interposed, which is the lemma the corollary's proof uses rather than the
// statement it follows from.
//
// Only a Theorem or a Proposition may father one, and that is the corpus
// speaking rather than a guess: of the thirty-six attached citations in the
// chapter that name a parent, twenty-eight name a Proposition and eight a
// Theorem, and none names a Lemma, a Definition, a Remark, an Example or a
// Scholium. So nothing the book actually writes can be lost by this.
func fathers(r corpus.Ref) bool {
	return r.Number != 0 && (r.Kind == corpus.KindTheorem || r.Kind == corpus.KindProposition)
}

// The section number a fascicule de resultats prints in front of its no. is
// read and dropped here for assemble/structure.go's reason: the no. is what a
// citation names.
var subsecRE = regexp.MustCompile(`^### (?:\d+\.)?(\d+)\.? `)

func subsecHeading(line string) (int, bool) {
	m := subsecRE.FindStringSubmatch(line)
	if m == nil {
		return 0, false
	}
	n, _ := strconv.Atoi(m[1])
	return n, true
}

// SectionAt is the § the book prints on a page of a chapter.
//
// The Book is asked as well as the chapter, and has to be, because the corpus
// now holds two chapters VIII: Algebra VIII and Lie VIII. On the chapter alone,
// every page citation either chapter makes came back with two candidates and
// resolved to nothing, 606 of them.
//
// The page ranges within a chapter do not overlap: of the 467 pages the sections
// of Algebra VIII span, 456 fall in exactly one and 11 in none, those being the
// leaves between one § and the next. So this returns at most one section, and
// the ambiguity spec 05 §4 anticipated is not one these chapters have. A chapter
// that does have it will come back through here as two candidates and be
// reported.
func (ix *Index) SectionAt(book, chapter string, page int) []*Section {
	var out []*Section
	for i := range ix.Sections {
		s := &ix.Sections[i]
		if s.Book == book && s.Chapter == chapter && s.Holds(page) {
			out = append(out, s)
		}
	}
	return out
}

// SectionIn is a § of a chapter by its number, which is how the other citation
// printing names one. It is a plainer lookup than SectionAt and cannot be
// ambiguous: a chapter has one § 2, where a page can belong to two §§ at once
// because the second of them began part way down it.
func (ix *Index) SectionIn(book, chapter string, n int, appendix bool) *Section {
	for i := range ix.Sections {
		s := &ix.Sections[i]
		if s.Book == book && s.Chapter == chapter && s.Number == n && s.Appendix == appendix {
			return s
		}
	}
	return nil
}

// Holds reports whether the corpus has any of this Book in it.
func (ix *Index) Holds(book string) bool {
	for i := range ix.Sections {
		if ix.Sections[i].Book == book {
			return true
		}
	}
	return false
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
