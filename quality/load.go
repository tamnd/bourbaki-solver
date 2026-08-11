package quality

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/refs"
	"github.com/tamnd/bourbaki-solver/tags"
)

// Options is what the command passes in.
type Options struct {
	Root string

	// Langs are the languages to audit. Empty means every language the corpus
	// has, which is what a run with no flags should do.
	Langs []string

	Only []string
	Skip []string

	// ValidateTeX turns on M04. It is off by default because it is the one
	// rule here that is slow, and the only one whose answer depends on which
	// TeX is installed on the machine running it.
	ValidateTeX bool

	// Base is the commit T05 reads the history of tags/tags against.
	Base string

	// Assembled is what bourbaki assemble would write, keyed by absolute path,
	// with Stale the committed files no page produced. S09 is the comparison
	// of that against what is on disk.
	//
	// It is passed in rather than computed here because the assembler's driver
	// lives with the command, and because a caller that has just assembled
	// should not pay for it twice. AssembleErr says why it is absent, so S09
	// reports not run rather than passing on an empty map.
	Assembled   map[string][]byte
	Stale       []string
	AssembleErr string
}

// A Doc is one committed content file, read once and shared by every rule.
//
// Body is normalised, so a rule that counts or hashes it agrees with what
// assembly wrote. BodyLine turns a line of that body back into a line of the
// file, which is the number the reader needs and the number a CI annotation
// has to carry.
type Doc struct {
	Path string // relative to the corpus root, forward slashes
	Lang string
	Kind string // section, exercise or solution
	Body string
	Raw  []byte

	Section  *corpus.SectionFrontMatter
	Exercise *corpus.ExerciseFrontMatter
	Solution *corpus.SolutionFrontMatter

	// Err is front matter that would not parse. The file is kept in the list
	// with its raw bytes, because the rules that read bytes rather than fields
	// can still say something useful about it, and S02 is what reports the Err.
	Err error

	head int // the file line the body's first line sits on
}

// The three shapes of content file.
const (
	KindSection  = "section"
	KindExercise = "exercise"
	KindSolution = "solution"
)

// BodyLine is the file line of line n of the body, counting both from one.
func (d Doc) BodyLine(n int) int { return d.head + n - 1 }

// A Corpus is everything the rules read, loaded once.
type Corpus struct {
	Opt  Options
	Root string

	Books     *corpus.BooksManifest
	TOC       *corpus.TOCManifest
	Sections  *corpus.SectionsManifest
	Exercises *corpus.ExercisesManifest

	Tags  *tags.Set
	Items map[string][]tags.Item // per language, in the order tags are handed out

	Docs      []Doc                        // every content file of every language
	Pages     map[string][]corpus.PageFile // per book, in page order
	PagePaths map[string][]string          // the paths of those pages, same order

	Maps map[string]*pagemap.Map // per book, as committed

	Refs *refs.Result // the reference graph over English

	// Langs are the languages found under content/, English first. Solutions
	// live under content/solutions and are not a language.
	Langs []string

	// Tracked is git ls-files, and TrackedErr why it is not available. The
	// hygiene rules are about what is committed rather than about what is in
	// the directory, so outside a git checkout they do not run.
	Tracked    []string
	TrackedErr error

	// TagsDiff is what this checkout did to tags/tags since Base.
	TagsDiff    string
	TagsDiffErr error

	// Figures are the files under figures/, relative to the root.
	Figures []string

	tagFailures []tags.Failure
}

// Load reads the corpus. It reads every file the rules need exactly once,
// because forty rules each walking content/ is forty times the io for the same
// bytes, and because two rules that disagree about what a file says are worse
// than either being wrong.
func Load(opt Options) (*Corpus, error) {
	if opt.Root == "" {
		return nil, fmt.Errorf("no corpus root")
	}
	c := &Corpus{Opt: opt, Root: opt.Root}

	var err error
	if c.Books, err = corpus.LoadBooks(c.Root); err != nil {
		return nil, err
	}
	if c.TOC, err = corpus.LoadTOC(c.Root); err != nil {
		return nil, err
	}
	if c.Sections, err = corpus.LoadSections(c.Root); err != nil {
		return nil, err
	}
	if c.Exercises, err = corpus.LoadExercises(c.Root); err != nil {
		return nil, err
	}
	if c.Tags, err = tags.Load(c.Root); err != nil {
		return nil, err
	}

	if c.Langs, err = languages(c.Root, opt.Langs); err != nil {
		return nil, err
	}
	c.Items = map[string][]tags.Item{}
	for _, l := range c.Langs {
		items, err := tags.Walk(c.Root, l)
		if err != nil {
			return nil, err
		}
		c.Items[l] = items
	}

	if c.Docs, err = readDocs(c.Root, c.Langs); err != nil {
		return nil, err
	}
	if err := c.readPages(); err != nil {
		return nil, err
	}
	c.readMaps()

	// The graph is built rather than read back, so R01 to R03 answer for the
	// Markdown as it stands and not for a manifest somebody could have edited.
	// That the manifest agrees with the Markdown is refs build -check, which is
	// its own step and not this one.
	if hasLang(c.Langs, "en") {
		if c.Refs, err = refs.Build(c.Root, "en"); err != nil {
			return nil, err
		}
	}

	c.Tracked, c.TrackedErr = tracked(c.Root)
	c.TagsDiff, c.TagsDiffErr = tagsDiff(c.Root, opt.Base)
	if c.Figures, err = figures(c.Root); err != nil {
		return nil, err
	}
	return c, nil
}

// SourceLangs are the languages the library is printed in, which is English
// plus every language a registered volume carries, so in practice en and fr.
//
// It is what separates an extraction from a translation. content/fr is read off
// the French volume the same way content/en is read off the English one, so it
// names no translated_from and there is no English file to compare it against.
// Without this the translation rules would take all of the French for a
// translation with its source missing and say so once per file.
func (c *Corpus) SourceLangs() map[string]bool {
	out := map[string]bool{"en": true}
	if c.Books == nil {
		return out
	}
	for _, b := range c.Books.Books {
		if b.Lang != "" {
			out[b.Lang] = true
		}
	}
	return out
}

// languages lists the languages under content/. content/solutions is a sibling
// of the languages and not one of them, which is the one thing a caller walking
// that directory always gets wrong.
func languages(root string, want []string) ([]string, error) {
	ents, err := os.ReadDir(filepath.Join(root, "content"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range ents {
		if !e.IsDir() || e.Name() == "solutions" {
			continue
		}
		if len(want) > 0 && !hasLang(want, e.Name()) {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i] == "en") != (out[j] == "en") {
			return out[i] == "en"
		}
		return out[i] < out[j]
	})
	return out, nil
}

func hasLang(langs []string, l string) bool {
	for _, x := range langs {
		if x == l {
			return true
		}
	}
	return false
}

// readDocs reads every content file of every language, and the solutions.
func readDocs(root string, langs []string) ([]Doc, error) {
	var out []Doc
	for _, lang := range langs {
		dir := filepath.Join(root, "content", lang)
		if err := walkMarkdown(dir, func(path string) error {
			d, err := readDoc(root, path, lang)
			if err != nil {
				return err
			}
			out = append(out, d)
			return nil
		}); err != nil {
			return nil, err
		}
	}
	if err := walkMarkdown(filepath.Join(root, "content", "solutions"), func(path string) error {
		rest, err := filepath.Rel(filepath.Join(root, "content", "solutions"), path)
		if err != nil {
			return err
		}
		lang, _, _ := strings.Cut(filepath.ToSlash(rest), "/")
		d, err := readDoc(root, path, lang)
		if err != nil {
			return err
		}
		out = append(out, d)
		return nil
	}); err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

func walkMarkdown(dir string, fn func(string) error) error {
	err := filepath.WalkDir(dir, func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		return fn(path)
	})
	if os.IsNotExist(err) {
		return nil
	}
	return err
}

// readDoc reads one content file and works out which of the three schemas it
// is from where it sits. Front matter that will not parse is kept as Err rather
// than returned, so that one bad file does not stop the audit that would have
// told you which file it was.
func readDoc(root, path, lang string) (Doc, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Doc{}, err
	}
	rel, err := filepath.Rel(root, path)
	if err != nil {
		return Doc{}, err
	}
	d := Doc{Path: filepath.ToSlash(rel), Lang: lang, Raw: raw, head: bodyStart(raw)}
	switch {
	case strings.Contains(d.Path, "/solutions/"):
		d.Kind = KindSolution
	case strings.Contains(d.Path, "/exercises/"):
		d.Kind = KindExercise
	default:
		d.Kind = KindSection
	}
	switch d.Kind {
	case KindSolution:
		f, err := corpus.ParseFile[corpus.SolutionFrontMatter](raw)
		d.Body, d.Err = f.Body, err
		if err == nil {
			d.Solution = &f.Meta
		}
	case KindExercise:
		f, err := corpus.ParseFile[corpus.ExerciseFrontMatter](raw)
		d.Body, d.Err = f.Body, err
		if err == nil {
			d.Exercise = &f.Meta
		}
	default:
		f, err := corpus.ParseFile[corpus.SectionFrontMatter](raw)
		d.Body, d.Err = f.Body, err
		if err == nil {
			d.Section = &f.Meta
		}
	}
	if d.Err != nil {
		// Nothing parsed, so the body is whatever sits under the fence, taken
		// raw. The byte rules can still read it.
		if _, body, splitErr := corpus.SplitFrontMatter(raw); splitErr == nil {
			d.Body = corpus.NormalizeBody(string(body))
		}
	}
	return d, nil
}

// bodyStart is the file line the body's first line sits on.
//
// The body is normalised before any rule sees it, which strips the blank line
// between the fence and the text, so counting from the fence would put every
// line of every finding one out. Nobody checks a line number that is nearly
// right, they stop trusting the tool.
func bodyStart(raw []byte) int {
	lines := bytes.Split(raw, []byte("\n"))
	line, fences := 0, 0
	for i, l := range lines {
		if string(bytes.TrimRight(l, " \t\r")) == "---" {
			fences++
			if fences == 2 {
				line = i + 2 // the line after the closing fence, counting from one
				break
			}
		}
	}
	if fences < 2 {
		return 1
	}
	for line <= len(lines) && strings.TrimSpace(string(lines[line-1])) == "" {
		line++
	}
	return line
}

// readPages reads pages/<book>/*.md for every book the corpus has extracted.
func (c *Corpus) readPages() error {
	c.Pages = map[string][]corpus.PageFile{}
	c.PagePaths = map[string][]string{}
	for _, b := range c.Books.Books {
		dir := corpus.PagesDir(c.Root, b.ID)
		names, err := filepath.Glob(filepath.Join(dir, "*.md"))
		if err != nil {
			return err
		}
		sort.Strings(names)
		for _, name := range names {
			f, err := corpus.ReadFile[corpus.PageFrontMatter](name)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(c.Root, name)
			c.Pages[b.ID] = append(c.Pages[b.ID], f)
			c.PagePaths[b.ID] = append(c.PagePaths[b.ID], filepath.ToSlash(rel))
		}
	}
	return nil
}

// readMaps reads the committed page map of every book. A book with no map is
// left out rather than reported here; S05 is what says so.
func (c *Corpus) readMaps() {
	c.Maps = map[string]*pagemap.Map{}
	for _, b := range c.Books.Books {
		if m, err := pagemap.Load(c.Root, b.ID); err == nil {
			c.Maps[b.ID] = m
		}
	}
}

// tracked is git ls-files, which is the only honest answer to what is in the
// repository. Walking the directory would count the PDFs, the page images and
// everything else .gitignore keeps out, and would fail every hygiene rule on
// every developer's machine while passing in CI.
func tracked(root string) ([]string, error) {
	cmd := exec.Command("git", "ls-files", "-z")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("git ls-files: %w", err)
	}
	var names []string
	for _, n := range strings.Split(string(out), "\x00") {
		if n != "" {
			names = append(names, n)
		}
	}
	return names, nil
}

// tagsDiff asks git what this checkout did to tags/tags since base, for T05.
func tagsDiff(root, base string) (string, error) {
	if base == "" {
		return "", fmt.Errorf("no base commit given")
	}
	cmd := exec.Command("git", "diff", "--unified=0", base, "--", filepath.Join("tags", tags.TagsFile))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff %s: %s", base, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

// figures lists what is under figures/, relative to the root. .gitkeep is not a
// figure; it is how an empty directory is committed at all.
func figures(root string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(filepath.Join(root, "figures"), func(path string, e fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() || e.Name() == ".gitkeep" {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		out = append(out, filepath.ToSlash(rel))
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	sort.Strings(out)
	return out, err
}

// Sections, Exercises and Solutions are the docs of one shape.
func (c *Corpus) OfKind(kind string) []Doc {
	var out []Doc
	for _, d := range c.Docs {
		if d.Kind == kind {
			out = append(out, d)
		}
	}
	return out
}

// InLang is the docs of one language.
func (c *Corpus) InLang(lang string) []Doc {
	var out []Doc
	for _, d := range c.Docs {
		if d.Lang == lang {
			out = append(out, d)
		}
	}
	return out
}

// Translations are the languages other than English.
func (c *Corpus) Translations() []string {
	var out []string
	for _, l := range c.Langs {
		if l != "en" {
			out = append(out, l)
		}
	}
	return out
}

// isTracked answers whether git has this path.
func (c *Corpus) isTracked(rel string) bool {
	for _, t := range c.Tracked {
		if t == rel {
			return true
		}
	}
	return false
}
