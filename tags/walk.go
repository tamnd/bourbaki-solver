package tags

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// HeadingRE reads a statement heading. The tag is optional because a corpus
// that has never been assigned has none, and that is the state assign is for.
//
//	#### Proposition 6 {#alg-viii-s1-prop-6 .statement}
//	#### Proposition 6 {#alg-viii-s1-prop-6 .statement tag=0A3F}
//
// What the tag may look like is deliberately loose here. A heading somebody
// hand-edited into tag=0a3f is a thing that has to be found and named, and a
// pattern that only matches the correct form would not find it: the heading
// would read as one carrying no tag at all, and the report would say something
// true but unhelpful. Parse is what decides, and the malformed text is kept so
// the message can quote it.
var HeadingRE = regexp.MustCompile(`^#{3,4} .*\{#([a-z0-9-]+) \.statement(?: tag=([^} ]*))?\}$`)

// Item is one thing in the corpus that carries a tag.
type Item struct {
	Path  string // the file it is in, relative to the corpus root
	Line  int    // the line of that file, 0 for an exercise, whose tag is in the front matter
	Label string
	Tag   Tag    // empty when nothing has been assigned yet
	Bad   string // the tag text as written, when it is not a tag at all
}

// Walk lists everything in one language of the corpus that carries a tag, in
// the order tags are allocated in.
//
// That order is the book's own: book, then chapter in Roman order, then the
// sections as the volume prints them, and within a section the statements in
// the order they are read and then the exercises printed at its foot. It has to
// be an order somebody can predict, because it is the order the permanent
// identifiers are handed out in and it is the only thing that makes a run of
// assign reproducible.
//
// An exercise is tagged as well as a statement. The book does not call an
// exercise a result, but this corpus cites them, translates them and solves
// them, and every one of those needs a name that survives a renumbering exactly
// as a proposition's does.
func Walk(root, lang string) ([]Item, error) {
	var out []Item
	books, err := dirs(filepath.Join(root, "content", lang))
	if err != nil {
		return nil, err
	}
	for _, book := range books {
		chapters, err := dirs(filepath.Join(root, "content", lang, book))
		if err != nil {
			return nil, err
		}
		if err := romanOrder(chapters); err != nil {
			return nil, fmt.Errorf("content/%s/%s: %w", lang, book, err)
		}
		for _, ch := range chapters {
			dir := filepath.Join(root, "content", lang, book, ch)
			items, err := walkChapter(root, dir)
			if err != nil {
				return nil, err
			}
			out = append(out, items...)
		}
	}
	return out, nil
}

func walkChapter(root, dir string) ([]Item, error) {
	names, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	var out []Item
	for _, name := range names {
		f, err := corpus.ReadFile[corpus.SectionFrontMatter](name)
		if err != nil {
			return nil, err
		}
		out = append(out, statements(root, name, f.Body)...)
		ex, err := exercises(root, dir, f.Meta)
		if err != nil {
			return nil, err
		}
		out = append(out, ex...)
	}
	return out, nil
}

// statements are the tagged headings of one section file, in the order they are
// printed. The line counts from the top of the file, front matter and all, so
// that an error names something an editor can jump to.
func statements(root, path, body string) []Item {
	offset := 0
	if b, err := os.ReadFile(path); err == nil {
		offset = strings.Count(strings.TrimSuffix(string(b), body), "\n")
	}
	var out []Item
	for i, line := range strings.Split(body, "\n") {
		m := HeadingRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		out = append(out, item(rel(root, path), offset+i+1, m[1], m[2]))
	}
	return out
}

// exercises are the tagged exercises of one section, in the order the book
// numbers them.
func exercises(root, dir string, m corpus.SectionFrontMatter) ([]Item, error) {
	if m.Exercises == 0 {
		return nil, nil
	}
	sub := filepath.Join(dir, "exercises", corpus.ExerciseDir(m.Section, m.Appendix))
	names, err := filepath.Glob(filepath.Join(sub, "*.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(names)
	out := make([]Item, 0, len(names))
	for _, name := range names {
		f, err := corpus.ReadFile[corpus.ExerciseFrontMatter](name)
		if err != nil {
			return nil, err
		}
		if f.Meta.Label == "" {
			return nil, fmt.Errorf("%s: no label in the front matter", rel(root, name))
		}
		out = append(out, item(rel(root, name), 0, f.Meta.Label, f.Meta.Tag))
	}
	return out, nil
}

// item sorts the tag text into a tag or into the thing that was written where
// one should have been. Empty is neither: it is a statement waiting for its
// first assign run, and T03 has something to say about it later on.
func item(path string, line int, label, text string) Item {
	it := Item{Path: path, Line: line, Label: label}
	switch t, err := Parse(text); {
	case text == "":
	case err != nil:
		it.Bad = text
	default:
		it.Tag = t
	}
	return it
}

// Labels is the list Assign takes.
func Labels(items []Item) []string {
	out := make([]string, 0, len(items))
	for _, it := range items {
		out = append(out, it.Label)
	}
	return out
}

// dirs are the subdirectories of a path, sorted, and none at all if the path
// does not exist: a corpus with no translations yet is not an error.
func dirs(path string) ([]string, error) {
	entries, err := os.ReadDir(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if e.IsDir() {
			out = append(out, e.Name())
		}
	}
	sort.Strings(out)
	return out, nil
}

// romanOrder sorts chapter directories the way the volume does, so that VIII
// comes after III and not before it. A volume divided into no chapters is
// bound in the order pagemap numbered its spans. See corpus.ChapterOrder.
func romanOrder(chapters []string) error {
	value := make(map[string]int, len(chapters))
	for _, ch := range chapters {
		n, err := corpus.ChapterOrder(ch)
		if err != nil {
			return err
		}
		value[ch] = n
	}
	sort.Slice(chapters, func(i, j int) bool { return value[chapters[i]] < value[chapters[j]] })
	return nil
}

func rel(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(r)
	}
	return path
}
