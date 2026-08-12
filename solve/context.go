// Package solve builds what a model is shown when it is asked to do an
// exercise, and will hold the pipeline that asks.
//
// The context is assembled and not hand-waved, which is spec 07 §3.1. A
// Bourbaki exercise is not a self-contained problem. It says "with the notation
// of Exercise 18", it uses a definition made four pages earlier, and it cites a
// proposition of a Book that is not in the corpus. Handing the model the
// exercise alone and calling what comes back a solution would be measuring the
// model's memory of Bourbaki rather than its reading of this one, and the whole
// point of extracting the chapter was to have the text to read.
package solve

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/refs"
)

// Kind is what a piece of the context is. It is also the order the pieces are
// written in, nearest the exercise first.
type Kind string

const (
	TheExercise Kind = "exercise"
	Sibling     Kind = "sibling"
	TheSection  Kind = "section"
	Reference   Kind = "reference"
	Outside     Kind = "outside"
)

// Piece is one block of the context with where it came from kept beside it.
//
// The provenance is not decoration. A solution has to say which results it
// used, by tag, and a context that has been flattened into one string by the
// time it reaches the model cannot be asked for a tag afterwards.
type Piece struct {
	Kind  Kind
	Label string
	Tag   string
	File  string
	// Depth is 0 for the exercise, its siblings and its §, 1 for what the
	// exercise cites and 2 for what those cite.
	Depth int
	// Raw is the citation as the book prints it. It is what an out-of-corpus
	// piece has instead of a text, and it is all there is.
	Raw  string
	Text string
	// Why is why a piece was named rather than carried, and it is Carried on a
	// piece that was carried. Nothing is left out of a context silently: a
	// model reading one that has quietly lost its references reads it exactly
	// like a context that had none.
	Why Reason
}

// Reach is how the walk out from the exercise met something: how far out it was
// met, and how many of the citations in the context point at it.
//
// Both are needed and neither is decoration. Exercise 17 of § 16 cites Exercise
// 14 twice and Exercise 1 once, both at depth 1, both about 2 400 characters
// long, and when only one of them can be kept the number of times the exercise
// asked for it is the only thing that tells them apart.
type Reach struct {
	Depth, Times int
}

// Reason is why a reference is named rather than carried.
//
// The reasons want different work done about them, which is why they are told
// apart and counted apart. A reference the cap dropped is one somebody could
// raise the cap for. A page citation that narrowed no further than a § is one
// the resolver could be made to read better, and it is a fault in the reference
// graph rather than a limit of the context. One that did not fit in the whole
// question is neither: it is the service refusing to read a long message, and it
// is measured in RenderWithin and not here.
type Reason string

const (
	Carried     Reason = ""
	OverCap     Reason = "over-cap"
	SectionOnly Reason = "section-only"
)

// Sentence is the reason as the model is told it.
func (r Reason) Sentence(max int) string {
	switch r {
	case OverCap:
		return fmt.Sprintf("the context has a limit of %d characters on its references", max)
	case SectionOnly:
		return "the citation is to a page, and resolved no further than this §"
	case OverAsk:
		return "the whole question has a length limit and this did not fit in it"
	}
	return ""
}

// Options are the two numbers spec 07 §3.1 fixes.
type Options struct {
	// Depth is how far the closure of the cross-references is followed. 2 by
	// default: what the exercise cites, and what those cite.
	Depth int
	// MaxChars caps the references and nothing else. 40000 by default. The
	// exercise, its earlier siblings and its § are what the exercise is, and a
	// context that drops one of them to fit a cap is not a smaller context, it
	// is a different question.
	//
	// It is not the limit on what can be sent. That is a property of the service
	// rather than of the corpus, it is measured against the whole question
	// including the instructions and whatever answer is being judged, and it is
	// in RenderWithin. This one is a judgement about how much of the closure of
	// the cross-references is worth carrying at all.
	MaxChars int
}

func (o Options) withDefaults() Options {
	if o.Depth == 0 {
		o.Depth = 2
	}
	if o.MaxChars == 0 {
		o.MaxChars = 40000
	}
	return o
}

// Context is everything one exercise is shown with.
type Context struct {
	Label string
	Tag   string
	Lang  string
	// Options are the ones it was built under, with the defaults filled in, so
	// that a context can say what its own limits were.
	Options Options

	Pieces []Piece
	// Cites is every label the walk out from the exercise reached, whether or
	// not it was carried.
	//
	// It is the only record of some of what the exercise points at. A statement
	// of the exercise's own § is never carried as a reference, since the § is
	// there whole, and an earlier exercise it cites is carried as a sibling
	// rather than as a citation, so in both cases the piece itself does not know
	// the exercise asked for it.
	Cites map[string]Reach
	// Named are the references that are in the corpus and are not in the
	// context: the ones the cap left out, and the ones that resolved no
	// further than a whole §. Each carries the reason in its Why.
	Named []Piece
}

// Chars is how long the context is, counting what will be written to the model.
func (c *Context) Chars() int {
	n := 0
	for _, p := range c.Pieces {
		n += len(p.Text) + len(p.Raw)
	}
	return n
}

// Count is how many pieces of one kind the context holds.
func (c *Context) Count(k Kind) int {
	n := 0
	for _, p := range c.Pieces {
		if p.Kind == k {
			n++
		}
	}
	return n
}

// Corpus is one language of the corpus read once.
//
// Reading it is not cheap: the reference graph takes a second and a half to
// build and the chapter is 26 section files. Per exercise that would be eight
// minutes of nothing over the 317 the chapter prints, so it is done once and
// the contexts are cut out of it.
type Corpus struct {
	Root string
	Lang string

	graph *refs.Result
	// out is the edges of the graph by the tag they leave from, built once. Built
	// per context it would be 2122 edges walked 317 times to answer a question
	// that does not change between exercises.
	out map[string][]refs.Edge
	// sections is by § label. Only the files that are a § are in it, since the
	// front matter and the historical note are not what an exercise belongs to.
	sections map[string]*sectionFile
	// unit is every tagged statement of every file, by label, including the
	// files that are not a §. This is what a reference resolves through.
	unit      map[string]*statement
	exercises map[string]*exerciseFile
	inSection map[string][]*exerciseFile
}

type sectionFile struct {
	label string
	path  string
	body  string
}

type statement struct {
	label string
	tag   string
	file  *sectionFile
	text  string
}

type exerciseFile struct {
	label   string
	tag     string
	path    string
	body    string
	section string // the § label, alg-viii-s1
	number  int
}

// statementRE is the heading refs.Index reads too. The corpus writes a
// statement as a heading carrying its permanent label and its tag, and a
// reference resolves to the label, so this is what turns a resolved edge back
// into the words of the thing it resolved to.
var statementRE = regexp.MustCompile(`(?m)^#+ .*\{#([a-z0-9-]+) \.statement(?: tag=([0-9A-Z]{4}))?\}[ \t]*$`)

// Read loads one printing of the corpus for solving.
func Read(root, lang string) (*Corpus, error) {
	g, err := refs.Build(root, lang)
	if err != nil {
		return nil, err
	}
	c := &Corpus{Root: root, Lang: lang, graph: g,
		out:       map[string][]refs.Edge{},
		sections:  map[string]*sectionFile{},
		unit:      map[string]*statement{},
		exercises: map[string]*exerciseFile{},
		inSection: map[string][]*exerciseFile{}}
	if err := c.readSections(); err != nil {
		return nil, err
	}
	if err := c.readExercises(); err != nil {
		return nil, err
	}
	for _, e := range g.Edges {
		c.out[e.From] = append(c.out[e.From], e)
	}
	for _, list := range c.inSection {
		sort.Slice(list, func(i, j int) bool { return list[i].number < list[j].number })
	}
	return c, nil
}

// readSections reads the assembled files of this printing.
//
// The manifest carries both printings and the path says which is which, so the
// printing is filtered on rather than derived. A file that is not a § is read
// too: the front matter of a chapter carries its conventions and the historical
// note carries statements nothing else does, and a reference into either has to
// resolve to the words and not to a shrug.
func (c *Corpus) readSections() error {
	m, err := corpus.LoadSections(c.Root)
	if err != nil {
		return err
	}
	prefix := "content/" + c.Lang + "/"
	for _, b := range m.Books {
		for _, ch := range b.Chapters {
			for _, rec := range ch.Sections {
				if !strings.HasPrefix(rec.Path, prefix) {
					continue
				}
				f, err := corpus.ReadFile[corpus.SectionFrontMatter](
					filepath.Join(c.Root, filepath.FromSlash(rec.Path)))
				if err != nil {
					return err
				}
				sf := &sectionFile{label: rec.Label, path: rec.Path, body: strings.TrimSpace(f.Body)}
				if rec.Label != "" {
					c.sections[rec.Label] = sf
				}
				c.cut(sf)
			}
		}
	}
	return nil
}

// cut divides a file into its tagged statements.
//
// A statement runs from its own heading to the next one, which takes in the
// proof and the remarks printed under it. That is more than the statement and
// it is what a reader following the reference would have read.
func (c *Corpus) cut(sf *sectionFile) {
	at := statementRE.FindAllStringSubmatchIndex(sf.body, -1)
	for i, m := range at {
		end := len(sf.body)
		if i+1 < len(at) {
			end = at[i+1][0]
		}
		s := &statement{label: sf.body[m[2]:m[3]], file: sf,
			text: strings.TrimSpace(sf.body[m[0]:end])}
		if m[4] >= 0 {
			s.tag = sf.body[m[4]:m[5]]
		}
		c.unit[s.label] = s
	}
}

// readExercises walks the exercise files of this printing.
//
// The tree is walked rather than the manifest read, because the manifest counts
// exercises per volume and a volume is a printing, so following it would mean
// mapping a language onto a volume id to find files whose path already says
// which language they are.
func (c *Corpus) readExercises() error {
	root := filepath.Join(c.Root, "content", c.Lang)
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if os.IsNotExist(err) && path == root {
				return nil // a language the corpus does not carry yet
			}
			return err
		}
		if d.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		if !strings.Contains(filepath.ToSlash(path), "/exercises/") {
			return nil
		}
		b, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		f, err := corpus.ParseFile[corpus.ExerciseFrontMatter](b)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}
		rel, err := filepath.Rel(c.Root, path)
		if err != nil {
			return err
		}
		ex := &exerciseFile{label: f.Meta.Label, tag: f.Meta.Tag,
			path: filepath.ToSlash(rel), body: strings.TrimSpace(f.Body),
			number: f.Meta.Exercise, section: sectionOf(f.Meta.Label)}
		c.exercises[ex.label] = ex
		c.inSection[ex.section] = append(c.inSection[ex.section], ex)
		return nil
	})
}

// sectionOf is the § label an exercise label hangs off, alg-viii-s1 out of
// alg-viii-s1-ex-19.
func sectionOf(label string) string {
	i := strings.Index(label, "-ex-")
	if i < 0 {
		return ""
	}
	return label[:i]
}

// Exercises is every exercise this printing carries, in label order.
func (c *Corpus) Exercises() []string {
	out := make([]string, 0, len(c.exercises))
	for label := range c.exercises {
		out = append(out, label)
	}
	sort.Strings(out)
	return out
}

// Build assembles the context for one exercise.
func (c *Corpus) Build(label string, o Options) (*Context, error) {
	o = o.withDefaults()
	ex, ok := c.exercises[label]
	if !ok {
		return nil, fmt.Errorf("%s: no such exercise in the %s printing", label, c.Lang)
	}
	sf, ok := c.sections[ex.section]
	if !ok {
		return nil, fmt.Errorf("%s: §%s is not assembled in the %s printing",
			label, ex.section, c.Lang)
	}

	out := &Context{Label: label, Tag: ex.tag, Lang: c.Lang, Options: o}
	out.Pieces = append(out.Pieces, Piece{Kind: TheExercise, Label: ex.label,
		Tag: ex.tag, File: ex.path, Text: ex.body})

	// Every earlier exercise of the same §, in the order the book prints them.
	// "With the notation of Exercise 18" is how Bourbaki writes an exercise
	// that continues another, and the notation is rarely restated. The later
	// ones are left out: they are not what this one was written to follow, and
	// carrying them would put the answer in front of a model being asked for
	// it, since an exercise that builds on this one tends to say what this one
	// establishes.
	for _, sib := range c.inSection[ex.section] {
		if sib.number >= ex.number {
			continue
		}
		out.Pieces = append(out.Pieces, Piece{Kind: Sibling, Label: sib.label,
			Tag: sib.tag, File: sib.path, Text: sib.body})
	}

	out.Pieces = append(out.Pieces, Piece{Kind: TheSection, Label: sf.label,
		File: sf.path, Text: sf.body})

	in, named, outside, reach := c.closure(ex, o.Depth)
	out.Cites = reach
	kept, dropped := fit(in, o.MaxChars)
	out.Pieces = append(out.Pieces, kept...)
	out.Pieces = append(out.Pieces, outside...)
	out.Named = append(named, dropped...)
	return out, nil
}

// closure walks the reference graph out from the exercise.
//
// A reference into the § the exercise stands in is not carried, because the
// whole of that § is in the context already and writing a proposition of it
// twice costs a thousand characters and buys nothing. A reference that leaves
// the corpus is carried as the citation itself, which is spec 07 §3.1's honest
// handling of one chapter of a nine chapter Book in a series of ten: the model
// is told what was cited, told that the corpus does not hold it, and told to
// say so if it uses it.
func (c *Corpus) closure(of *exerciseFile, depth int) (in, named, outside []Piece, reach map[string]Reach) {
	seen := map[string]bool{of.label: true}
	reach = map[string]Reach{}
	said := map[string]bool{}
	frontier := []string{of.tag}
	for d := 1; d <= depth && len(frontier) > 0; d++ {
		var next []string
		for _, tag := range frontier {
			for _, e := range c.out[tag] {
				if e.How == refs.OutOfCorpus {
					if !said[e.Raw] {
						said[e.Raw] = true
						outside = append(outside, Piece{Kind: Outside, Depth: d,
							Raw: e.Raw, File: e.File, Label: e.Book})
					}
					continue
				}
				// An edge is followed by its label and not by its tag. A
				// reference that resolved no further than a whole § has a label
				// and no tag, because nothing in this corpus tags a §, and
				// keying the walk on the tag threw every one of them away. It
				// showed up as the French printing carrying no in-corpus
				// references at all: its resolver reaches § granularity and no
				// further, so all 240 of its edges out of an exercise were
				// silently dropped and every French context was the exercise,
				// its siblings and its § with nothing else in it.
				var tagged string
				if e.To != nil {
					tagged = *e.To
				}
				if e.ToLabel == "" {
					continue
				}
				// Counted before the walk decides whether to follow it, since
				// the second citation of a thing is a fact about the exercise
				// and not a repeat of the first.
				r := reach[e.ToLabel]
				if r.Depth == 0 {
					r.Depth = d
				}
				r.Times++
				reach[e.ToLabel] = r
				if seen[e.ToLabel] {
					continue
				}
				seen[e.ToLabel] = true
				// The frontier grows whether or not the piece is carried. What
				// the § already in the context cites is a depth 2 reference of
				// this exercise like any other, and stopping the walk at it
				// would make the closure depend on which § the exercise happens
				// to sit in. It grows only through what has a tag, since the
				// edges are keyed by the tag they leave from.
				if tagged != "" {
					next = append(next, tagged)
				}
				p, ok := c.piece(e.ToLabel, tagged, of, d)
				switch {
				case !ok:
				case p.Why != Carried:
					named = append(named, p)
				default:
					in = append(in, p)
				}
			}
		}
		frontier = next
	}
	return in, named, outside, reach
}

// piece is the words of one thing a reference resolved to, statement or
// exercise, or nothing at all where it is in the context already.
//
// The two are looked up differently because they are indexed differently. A
// statement is a heading inside an assembled file and the reference index knows
// where it is; an exercise is a file of its own that no index holds, which is
// what the walk in readExercises is for.
func (c *Corpus) piece(label, tag string, of *exerciseFile, depth int) (Piece, bool) {
	if f, ok := c.exercises[label]; ok {
		// An exercise of the same § that this one follows is carried already,
		// as a sibling. One it precedes is not, and is carried here: an
		// exercise that cites forward means it, and refusing a citation the
		// book makes on a rule about ordering would be answering a question the
		// book did not ask. Only three exercises of chapter VIII do it.
		if f.label == of.label || f.section == of.section && f.number < of.number {
			return Piece{}, false
		}
		return Piece{Kind: Reference, Label: label, Tag: tag, File: f.path,
			Depth: depth, Text: f.body}, true
	}
	if s, ok := c.unit[label]; ok {
		if s.file.label == of.section {
			return Piece{}, false // inside the § that is carried whole
		}
		return Piece{Kind: Reference, Label: label, Tag: tag, File: s.file.path,
			Depth: depth, Text: s.text}, true
	}
	// A whole § is named and not carried. These are the bare page citations,
	// "VIII, p. 51", which the resolver could narrow to the § holding that page
	// and no further. What the sentence wants is the page; what carrying it
	// would put in front of the model is forty thousand characters of §, which
	// is the entire reference budget spent on one citation that was never
	// precise enough to spend it on. Chapter VIII makes 200 of them.
	if sf, ok := c.sections[label]; ok {
		if sf.label == of.section {
			return Piece{}, false
		}
		return Piece{Kind: Reference, Label: label, Tag: tag, File: sf.path, Depth: depth,
			Why: SectionOnly}, true
	}
	return Piece{}, false
}

// fit keeps the nearest references that will go in and names the rest.
//
// Nearest first is depth, and inside a depth the order the citations were met,
// which is the order of the sentences that make them. It is not a ranking of
// what matters, since nothing here knows that. It is the order a reader would
// have followed them in, and it is stable, which is what lets two runs over the
// same exercise be compared.
//
// A piece too large to fit does not stop the ones after it being tried. The
// long ones are the theorems with their proofs under them and the short ones
// are the definitions, and a definition that would have fitted is worth more
// per character than the theorem that displaced it.
func fit(in []Piece, max int) (kept, dropped []Piece) {
	sort.SliceStable(in, func(i, j int) bool { return in[i].Depth < in[j].Depth })
	n := 0
	for _, p := range in {
		if n+len(p.Text) > max {
			p.Why = OverCap
			p.Text = ""
			dropped = append(dropped, p)
			continue
		}
		n += len(p.Text)
		kept = append(kept, p)
	}
	return kept, dropped
}
