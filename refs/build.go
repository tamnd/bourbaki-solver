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
	To        *string `json:"to,omitempty"` // the tag it points at, absent when it points out of the corpus
	ToLabel   string  `json:"to_label,omitempty"`
	Kind      string  `json:"kind"`
	Raw       string  `json:"raw"`
	Form      Form    `json:"form"`
	How       string  `json:"resolved_by"`
	Book      string  `json:"book,omitempty"`
	Chapter   string  `json:"chapter,omitempty"` // only when the reference leaves the corpus
	File      string  `json:"file"`
	Line      int     `json:"line"`
	// Section is the § the sentence stands in, which is which file of the
	// manifest the edge is written to. It is not written into the edge, since it
	// is the name of the file the edge is already in.
	Section string `json:"-"`
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

// Manifest is the graph as it goes to manifests/refs/.
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
		res.lines(ix, rel, corrected(f.Body, f.Meta.Errata), headLines(b, f.Body), func(int) where { return at })
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
	body := corrected(f.Body, f.Meta.Errata)
	lines := strings.Split(body, "\n")
	res.lines(ix, rel, body, headLines(b, f.Body), func(i int) where {
		switch m := statementRE.FindStringSubmatch(lines[i]); {
		case m != nil:
			at = where{section: sec, label: m[1], tag: m[2]}
		case headingRE.MatchString(lines[i]):
			// Any other heading ends the statement and puts the § back in force.
			// Bourbaki sets the proof directly after the statement with nothing
			// between them, so a statement holds until something closes it, and
			// the only thing that closes one is a heading: the next statement, or
			// the subsection the next statement is under. Without this a statement
			// held to the end of the § and the prose opening subsection 2 was
			// attributed to the last lemma of subsection 1, which made its "(Lemma
			// 1)" read as the lemma citing itself.
			at = where{section: sec}
		}
		return at
	})
	return nil
}

// corrected is the body a reference is read out of, which is the printed body
// with the errata of the file applied to it.
//
// The corpus transcribes a printing and keeps the printed words even where they
// are wrong, and the correction goes in the front matter beside them. That is
// right for a reader holding the book and wrong for a graph: § 10 of chapter
// VIII prints no corollary under its Theorem 1, both corollaries stand under
// Theorem 2, and four exercises cite "Cor. 2 of Th. 1". Read as printed those
// four point at nothing and are reported four times over as a corollary the
// corpus is missing, which it is not; the manifest already says what the
// reference has to say to point where it means, so it is what the graph reads.
//
// The substitution is textual and stays on the line it was made on, since an
// erratum corrects words inside one sentence and the corpus writes a paragraph
// on one line. So the line a finding is reported at is still the file's own.
func corrected(body string, errata []corpus.Erratum) string {
	for _, e := range errata {
		body = strings.ReplaceAll(body, e.Says, e.Read)
	}
	return body
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
		Section: at.section,
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

// Manifest is the edge list as it goes to manifests/refs/.
func (res *Result) Manifest() *Manifest { return &Manifest{Edges: res.Edges} }

// ManifestDir is where the graph is written, one file to a §.
//
// It was one file, manifests/refs.json, and chapter VIII alone brought it to
// 510 KB against the 512 KB H03 holds a tracked file to. Laying it out more
// tightly was the answer the first time it went over and it is not the answer
// twice: the size is linear in how much of the Éléments has been read in, and
// chapter VIII is one of eight chapters in scope.
//
// A § is the unit to break it on because it is the unit everything else is
// already broken on: one file of content/, one file here, 42 KB at the largest
// and not growing with the corpus. It reads better as a diff too. A change to
// one section showed as a thousand lines in the middle of a file shared with
// twenty-five others, and now shows as a change to that section's file. The
// name is the § own label, so which file an edge belongs in is a question the
// edge answers.
const ManifestDir = "manifests/refs"

// Save writes manifests/refs/, one file to a §.
//
// A file that was written by an earlier run and is not written by this one is
// removed, which is what makes the directory the manifest rather than a place
// manifests accumulate: a § that is renamed or dropped leaves nothing behind to
// go stale.
func (m *Manifest) Save(root string) error {
	want, err := m.shards()
	if err != nil {
		return err
	}
	dir := filepath.Join(root, filepath.FromSlash(ManifestDir))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	old, err := shardFiles(dir)
	if err != nil {
		return err
	}
	for _, name := range old {
		if _, ok := want[name]; ok {
			continue
		}
		if err := os.Remove(filepath.Join(dir, name)); err != nil {
			return err
		}
	}
	for name, b := range want {
		if err := os.WriteFile(filepath.Join(dir, name), b, 0o644); err != nil {
			return err
		}
	}
	return nil
}

// Stale is the files of the manifest that a rebuild would change, which is the
// whole of the CI check: the graph is a pure function of the Markdown, so a
// checkout that has one and disagrees with it has had one of the two edited by
// hand. A file the build no longer writes counts as stale, since leaving it in
// place is how a checkout comes to hold a graph of a § it does not have.
func (m *Manifest) Stale(root string) ([]string, error) {
	want, err := m.shards()
	if err != nil {
		return nil, err
	}
	dir := filepath.Join(root, filepath.FromSlash(ManifestDir))
	old, err := shardFiles(dir)
	if err != nil {
		return nil, err
	}
	var out []string
	for _, name := range old {
		if _, ok := want[name]; !ok {
			out = append(out, ManifestDir+"/"+name)
		}
	}
	for name, b := range want {
		got, err := os.ReadFile(filepath.Join(dir, name))
		if err != nil && !os.IsNotExist(err) {
			return nil, err
		}
		if !bytes.Equal(got, b) {
			out = append(out, ManifestDir+"/"+name)
		}
	}
	sort.Strings(out)
	return out, nil
}

// shards renders the manifest one edge to a line, one file to a §.
//
// json.MarshalIndent gives 25710 lines and 704 KB for chapter VIII alone, and
// it diffs by field: an edge whose line number moved shows as a dozen changed
// lines and a reference that was added shifts everything under it. One edge to
// a line diffs by edge, which is the unit anybody reading the change cares
// about, and it is what makes a per-§ file readable at all.
//
// Each file is a JSON document of the same shape the single manifest had, so
// nothing that consumes one has to know it is a shard.
func (m *Manifest) shards() (map[string][]byte, error) {
	by := map[string][]Edge{}
	for _, e := range m.Edges {
		s := e.Section
		if s == "" {
			// An edge the builder could not place in a §. There are none in the
			// corpus as it stands, and one would be a fault in the builder rather
			// than in the book, so it is kept and named rather than dropped.
			s = "unplaced"
		}
		by[s] = append(by[s], e)
	}
	out := make(map[string][]byte, len(by))
	for s, edges := range by {
		var buf bytes.Buffer
		buf.WriteString("{\"edges\": [\n")
		for i, e := range edges {
			b, err := json.Marshal(e)
			if err != nil {
				return nil, err
			}
			buf.Write(b)
			if i+1 < len(edges) {
				buf.WriteByte(',')
			}
			buf.WriteByte('\n')
		}
		buf.WriteString("]}\n")
		out[s+".json"] = buf.Bytes()
	}
	return out, nil
}

// shardFiles is what the manifest directory holds now. A directory that is not
// there yet is not an error: it is a checkout that has never built the graph.
func shardFiles(dir string) ([]string, error) {
	ents, err := os.ReadDir(dir)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}
