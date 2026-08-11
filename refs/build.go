package refs

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// Edge is one reference, from the statement that makes it to the statement it
// points at. It is the record spec 01 §6 describes, with the file and line
// added, because a reference that does not resolve has to be findable and the
// raw text alone is not enough when the same sentence is written twice.
type Edge struct {
	From      string  `json:"from"` // the tag of the statement the sentence is in
	FromLabel string  `json:"from_label"`
	To        *string `json:"to"` // the tag it points at, null when it points out of the corpus
	ToLabel   string  `json:"to_label,omitempty"`
	Kind      string  `json:"kind"`
	Raw       string  `json:"raw"`
	Form      Form    `json:"form"`
	How       string  `json:"resolved_by"`
	Book      string  `json:"book,omitempty"`
	Chapter   string  `json:"chapter,omitempty"` // only when the reference leaves the corpus
	File      string  `json:"file"`
	Line      int     `json:"line"`
}

// Problem is a reference that names something and does not find it. Almost
// every one of these is a hole in the corpus rather than a fault in the lookup:
// the book cites a statement, the statement is not in the Markdown, and the
// reason is that extraction ran it into the paragraph above it.
type Problem struct {
	Raw    string `json:"raw"`
	Form   Form   `json:"form"`
	Reason string `json:"reason"`
	File   string `json:"file"`
	Line   int    `json:"line"`
	From   string `json:"from,omitempty"`
}

// Manifest is manifests/refs.json.
type Manifest struct {
	Edges []Edge `json:"edges"`
}

// Result is one run of the builder.
type Result struct {
	Edges      []Edge
	Unresolved []Problem
	// Counts is every value of How plus unresolved, so that a rate can be
	// quoted with the shape it was measured over rather than as one number.
	Counts map[string]int
	Forms  map[Form]int
	// Index is what the run resolved against, kept so that a report can name a
	// statement's tag without reading the corpus a second time.
	Index *Index
}

// Build reads every committed file of one language and resolves what it cites.
func Build(root, lang string) (*Result, error) {
	ix, err := Load(root, lang)
	if err != nil {
		return nil, err
	}
	files, err := contentFiles(root, lang)
	if err != nil {
		return nil, err
	}
	res := &Result{Counts: map[string]int{}, Forms: map[Form]int{}, Index: ix}
	for _, path := range files {
		if err := res.file(ix, root, path); err != nil {
			return nil, err
		}
	}
	sort.SliceStable(res.Edges, func(i, j int) bool {
		if res.Edges[i].File != res.Edges[j].File {
			return res.Edges[i].File < res.Edges[j].File
		}
		return res.Edges[i].Line < res.Edges[j].Line
	})
	return res, nil
}

// where is the statement a sentence is printed under.
type where struct {
	section string
	label   string
	tag     string
}

// file resolves the references of one file.
//
// A reference belongs to the statement it is printed under, which is almost
// always the proof of that statement, so the nearest heading above it is the
// tail of the edge. A reference in the prose before the first statement of a §
// belongs to the § and has no tag at the tail, which is worth keeping rather
// than a reason to drop the edge.
func (res *Result) file(ix *Index, root, path string) error {
	b, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	rel := filepath.ToSlash(strings.TrimPrefix(strings.TrimPrefix(path, root), string(filepath.Separator)))
	if isExercise(path) {
		f, err := corpus.ParseFile[corpus.ExerciseFrontMatter](b)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		ref, err := corpus.ParseLabel(f.Meta.Label)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		at := where{section: ref.SectionLabel(), label: f.Meta.Label, tag: f.Meta.Tag}
		res.lines(ix, rel, f.Body, headLines(b, f.Body), func(int) where { return at })
		return nil
	}
	f, err := corpus.ParseFile[corpus.SectionFrontMatter](b)
	if err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}
	sec := corpus.Ref{
		Book: f.Meta.Book, Chapter: f.Meta.Chapter, Section: f.Meta.Section, Appendix: f.Meta.Appendix,
	}.SectionLabel()
	at := where{section: sec}
	lines := strings.Split(f.Body, "\n")
	res.lines(ix, rel, f.Body, headLines(b, f.Body), func(i int) where {
		if m := statementRE.FindStringSubmatch(lines[i]); m != nil {
			at = where{section: sec, label: m[1], tag: m[2]}
		}
		return at
	})
	return nil
}

// lines walks a body and resolves what each line cites. at is asked which
// statement is in force, and a heading line puts its own statement in force
// from that line on, which costs nothing either way: Parse reads no citation
// out of a heading.
func (res *Result) lines(ix *Index, rel, body string, line0 int, at func(int) where) {
	for i, line := range strings.Split(body, "\n") {
		here := at(i)
		for _, c := range Parse(line, line0+i) {
			res.one(ix, c, here, rel)
		}
	}
}

func (res *Result) one(ix *Index, c Citation, at where, rel string) {
	res.Forms[c.Form]++
	t, err := ix.Resolve(c, Site{Section: at.section, File: rel, Line: c.Line})
	if err != nil {
		res.Counts["unresolved"]++
		res.Unresolved = append(res.Unresolved, Problem{
			Raw: c.Raw, Form: c.Form, Reason: err.Error(), File: rel, Line: c.Line, From: at.label,
		})
		return
	}
	res.Counts[t.How]++
	e := Edge{
		From: at.tag, FromLabel: at.label, ToLabel: t.Label, Kind: "cites",
		Raw: c.Raw, Form: c.Form, How: t.How, Book: t.Book, File: rel, Line: c.Line,
	}
	if at.label == "" {
		e.FromLabel = at.section
	}
	if t.How == OutOfCorpus {
		// Which chapter of which Book, so that the report can say what to ingest
		// next rather than only how much is missing.
		e.Chapter = c.Chapter
	}
	if t.Tag != "" {
		tag := t.Tag
		e.To = &tag
	}
	res.Edges = append(res.Edges, e)
}

// headLines is how many lines of a file come before its body, so that a line of
// the body can be reported as a line of the file.
func headLines(b []byte, body string) int {
	return strings.Count(strings.TrimSuffix(string(b), body), "\n") + 1
}

func contentFiles(root, lang string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(filepath.Join(root, "content", lang), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func isExercise(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/exercises/")
}

// Manifest is the edge list as it goes to manifests/refs.json.
func (res *Result) Manifest() *Manifest { return &Manifest{Edges: res.Edges} }

// Save writes manifests/refs.json.
func (m *Manifest) Save(root string) error {
	b, err := m.bytes()
	if err != nil {
		return err
	}
	return os.WriteFile(manifestPath(root), b, 0o644)
}

// Stale is whether the committed manifest differs from this one, which is the
// whole of the CI check: the graph is a pure function of the Markdown, so a
// checkout that has one and disagrees with it has had one of the two edited by
// hand.
func (m *Manifest) Stale(root string) (bool, error) {
	want, err := m.bytes()
	if err != nil {
		return false, err
	}
	got, err := os.ReadFile(manifestPath(root))
	if os.IsNotExist(err) {
		return true, nil
	}
	if err != nil {
		return false, err
	}
	return !bytes.Equal(got, want), nil
}

func (m *Manifest) bytes() ([]byte, error) {
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(b, '\n'), nil
}

func manifestPath(root string) string { return filepath.Join(root, "manifests", "refs.json") }
