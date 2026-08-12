package publish

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// Exercise is one file of content/<lang>/<book>/<chapter>/exercises/<dir>/NN.md.
//
// Bourbaki sets the exercises at the end of the § and numbers them from one, and
// the corpus keeps one file per exercise for the same reason it keeps one file
// per §: the unit somebody works on is the unit that gets a file, a label and a
// tag.
type Exercise struct {
	Lang string
	Meta corpus.ExerciseFrontMatter
	Body string
	Path string // repo-relative, so the page can link at GitHub
	// Dir is the § the exercises belong to, "s5" or "a1", which is the same
	// string the § itself is slugged as. The two have to agree: the exercises of
	// § 5 live under the URL of § 5.
	Dir  string
	Head int
	// Solution is the worked solution, when the corpus holds one. It is nil for
	// every exercise today, which is the state the disclosure is built to show:
	// M8 writes solutions and nothing about this page changes when it does.
	Solution *Solution
}

// Solution is one file of content/solutions/<lang>/.
type Solution struct {
	Meta corpus.SolutionFrontMatter
	Body string
	Path string
	Head int
}

// Number is the exercise as the book numbers it.
func (e *Exercise) Number() int { return e.Meta.Exercise }

// Name is how an exercise is written when it is named on another page. The
// number alone is no use, since every § has an Exercise 1.
func (e *Exercise) Name() string {
	in := fmt.Sprintf("§ %d", e.Meta.Section)
	if e.Meta.Appendix {
		in = fmt.Sprintf("Appendix %d", e.Meta.Section)
	}
	return fmt.Sprintf("Exercise %d, %s", e.Meta.Exercise, in)
}

// loadExercises reads one language's exercises. The walk is the exercises
// directory of every chapter, which loadLang skips for exactly this reason.
func loadExercises(root, lang string) ([]*Exercise, error) {
	dir := filepath.Join(root, "content", lang)
	var out []*Exercise
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		rel, err := filepath.Rel(dir, p)
		if err != nil {
			return err
		}
		parts := strings.Split(filepath.ToSlash(rel), "/")
		// <book>/<chapter>/exercises/<dir>/NN.md and nothing else. A file at any
		// other depth under exercises/ is not an exercise and is left to whatever
		// pass owns it rather than being guessed at here.
		if len(parts) != 5 || parts[2] != "exercises" {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		f, err := corpus.ParseFile[corpus.ExerciseFrontMatter](raw)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		repoRel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		out = append(out, &Exercise{Lang: lang, Meta: f.Meta, Body: f.Body,
			Path: filepath.ToSlash(repoRel), Dir: parts[3], Head: corpus.BodyStart(raw)})
		return nil
	})
	if err != nil {
		return nil, err
	}
	// By § in the order the chapter has them and then by number, which is the
	// order the book prints them in and the order a list page wants. The file
	// names sort 01, 02, 10, so the path is not that order once an § passes nine.
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Meta.Book != b.Meta.Book {
			return a.Meta.Book < b.Meta.Book
		}
		if a.Meta.Chapter != b.Meta.Chapter {
			return a.Meta.Chapter < b.Meta.Chapter
		}
		if a.Meta.Appendix != b.Meta.Appendix {
			return !a.Meta.Appendix
		}
		if a.Meta.Section != b.Meta.Section {
			return a.Meta.Section < b.Meta.Section
		}
		return a.Meta.Exercise < b.Meta.Exercise
	})
	return out, nil
}

// loadSolutions reads content/solutions/ and keys what it finds by language and
// label.
//
// The directory does not exist yet and that is not an error. Solutions are M8,
// and the point of building the disclosure now is that landing them is a content
// change and not a template change.
//
// The key is the label out of the front matter rather than the path. SolutionPath
// writes every solution under sN even when the exercise is an appendix's, so two
// exercises with the same number in § 1 and Appendix 1 would share a file name,
// and reading the label is both the right key and the one that does not inherit
// that.
func loadSolutions(root string) (map[string]*Solution, error) {
	dir := filepath.Join(root, "content", "solutions")
	out := map[string]*Solution{}
	if _, err := os.Stat(dir); os.IsNotExist(err) {
		return out, nil
	}
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || !strings.HasSuffix(p, ".md") {
			return nil
		}
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		f, err := corpus.ParseFile[corpus.SolutionFrontMatter](raw)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		key := f.Meta.Lang + "\x00" + f.Meta.Label
		if old := out[key]; old != nil {
			return fmt.Errorf("%s and %s are both the solution to %s in %s",
				old.Path, filepath.ToSlash(rel), f.Meta.Label, f.Meta.Lang)
		}
		out[key] = &Solution{Meta: f.Meta, Body: f.Body,
			Path: filepath.ToSlash(rel), Head: corpus.BodyStart(raw)}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return out, nil
}

// exerciseSet is the exercises of one § in one language, which is what the list
// page at <§>/ex/ shows.
type exerciseSet struct {
	Lang, Book, Chapter, Dir string
	Section                  *Section // the § itself, when that language holds it
	Items                    []*Exercise
}

// exerciseSets groups the exercises the way the pages are written, in the order
// the exercises are already in.
func (s *Site) exerciseSets() []*exerciseSet {
	var out []*exerciseSet
	at := map[string]*exerciseSet{}
	for _, ex := range s.Exercises {
		k := strings.Join([]string{ex.Lang, ex.Meta.Book, ex.Meta.Chapter, ex.Dir}, "\x00")
		set := at[k]
		if set == nil {
			set = &exerciseSet{Lang: ex.Lang, Book: ex.Meta.Book, Chapter: ex.Meta.Chapter, Dir: ex.Dir}
			set.Section = s.sectionAt(ex.Lang, ex.Meta.Book, ex.Meta.Chapter, ex.Dir)
			at[k] = set
			out = append(out, set)
		}
		set.Items = append(set.Items, ex)
	}
	return out
}

// sectionAt finds a § by where it is rather than by its label, since the label
// is English-only in the graph and this is asked for every language.
func (s *Site) sectionAt(lang, book, chapter, slug string) *Section {
	for _, sec := range s.Sections {
		if sec.Lang == lang && sec.Meta.Book == book && sec.Meta.Chapter == chapter && sec.Slug == slug {
			return sec
		}
	}
	return nil
}

// exerciseHref answers the renderer's question about a corpus link to the
// exercises of a §, given the language the linking page is written in.
//
// A language can hold a § and not its exercises: the Vietnamese has Appendix 4
// and not the twelve exercises under it, and the sentence at the foot of that
// page still offers them. The English set is the honest answer there. It is a
// language the reader did not ask for, and it is the exercises they were
// promised, which beats a link to nothing.
func (s *Site) exerciseHref(lang, book, chapter string) func(string, int) string {
	find := func(lang, dir string, n int) string {
		for _, ex := range s.Exercises {
			if ex.Lang != lang || ex.Meta.Book != book || ex.Meta.Chapter != chapter || ex.Dir != dir {
				continue
			}
			if n == 0 {
				return s.ExercisesURL(lang, book, chapter, dir)
			}
			if ex.Meta.Exercise == n {
				return s.ExerciseURL(ex)
			}
		}
		return ""
	}
	return func(dir string, n int) string {
		if to := find(lang, dir, n); to != "" {
			return to
		}
		return find("en", dir, n)
	}
}

// hasExercises says whether one language holds the exercises of one §, which is
// not the same question as whether a link to them can be answered.
func (s *Site) hasExercises(lang, book, chapter, dir string) bool {
	for _, ex := range s.Exercises {
		if ex.Lang == lang && ex.Meta.Book == book && ex.Meta.Chapter == chapter && ex.Dir == dir {
			return true
		}
	}
	return false
}

// exercisePage is one exercise, and is what both its own URL and its tag URL
// serve. Canonical is empty on the first and the exercise's own URL on the
// second, since two URLs holding the same bytes is a thing to declare rather
// than to leave a search engine to work out.
func (s *Site) exercisePage(ex *Exercise, canonical string) (string, error) {
	sec := s.sectionAt(ex.Lang, ex.Meta.Book, ex.Meta.Chapter, ex.Dir)
	var b strings.Builder
	fmt.Fprintf(&b, "<h1>%s</h1>\n", template.HTMLEscapeString(ex.Name()))
	if ex.Meta.Tag != "" {
		fmt.Fprintf(&b, "<p class=\"tagline\">Tag <code>%s</code>, permanent. "+
			"Cite it as <code>[Bourbaki, Tag %s]</code>.</p>\n", ex.Meta.Tag, ex.Meta.Tag)
	}
	fmt.Fprintf(&b, "<p class=\"where\">%s</p>\n", s.exerciseWhere(ex, sec))
	if note := marks(ex); note != "" {
		fmt.Fprintf(&b, "<p class=\"marks\">%s</p>\n", note)
	}

	r := s.renderer(sec)
	// The renderer was built for the §, so it carries the §'s path. The formula
	// that fails is in the exercise file, and a message naming the § would send
	// somebody to the wrong file with a line number that means nothing in it.
	r.File, r.Line = ex.Path, ex.Head
	if r.Line == 0 {
		r.Line = 1
	}
	body, err := r.HTML(ex.Body)
	if err != nil {
		return "", err
	}
	b.WriteString(`<div class="statement-body">` + body + "</div>\n")
	s.solution(&b, ex)

	if ex.Meta.Tag != "" {
		s.list(&b, "Cited by", s.CitedBy[ex.Meta.Tag], s.from)
		s.list(&b, "Cites", s.Cites[ex.Meta.Tag], s.to)
	}

	var langs []langLink
	for _, l := range s.Langs {
		other := s.exerciseIn(l, ex)
		if other == nil {
			continue
		}
		langs = append(langs, langLink{Lang: l, URL: s.ExerciseURL(other),
			Here: l == ex.Lang, Draft: s.Draft[l]})
	}
	return s.render(layout{Title: ex.Name(), Lang: ex.Lang, Canonical: canonical, Langs: langs,
		Crumbs: s.exerciseCrumbs(ex, sec), Note: exerciseProvenance(ex), Content: template.HTML(b.String())})
}

// exerciseIn finds the same exercise in another language.
func (s *Site) exerciseIn(lang string, ex *Exercise) *Exercise {
	for _, o := range s.Exercises {
		if o.Lang == lang && o.Meta.Label == ex.Meta.Label {
			return o
		}
	}
	return nil
}

func (s *Site) exerciseCrumbs(ex *Exercise, sec *Section) []crumb {
	out := []crumb{{ex.Meta.Chapter, s.ChapterURL(ex.Lang, ex.Meta.Book, ex.Meta.Chapter)}}
	if sec != nil {
		out[0].Text = sec.Meta.BookTitle + ", Chapter " + ex.Meta.Chapter
		out = append(out, crumb{heading(sec), s.SectionURL(sec)})
	}
	return append(out, crumb{"Exercises",
		s.ExercisesURL(ex.Lang, ex.Meta.Book, ex.Meta.Chapter, ex.Dir)})
}

// exerciseWhere is the page of the volume the exercise is printed on, written
// the way the book cites itself.
func (s *Site) exerciseWhere(ex *Exercise, sec *Section) string {
	book := ex.Meta.Book
	if sec != nil {
		book = sec.Meta.BookTitle
	}
	if p, ok := corpus.ParsePageLabel(ex.Meta.BookPage); ok {
		return template.HTMLEscapeString(fmt.Sprintf("%s, %s, p. %d", book, ex.Meta.Chapter, p.Page))
	}
	// The front matter carries the running head as extraction read it, so a
	// page it could not read is shown as it stands rather than dropped.
	if ex.Meta.BookPage != "" {
		return template.HTMLEscapeString(fmt.Sprintf("%s, %s, %s", book, ex.Meta.Chapter, ex.Meta.BookPage))
	}
	return template.HTMLEscapeString(book + ", " + ex.Meta.Chapter)
}

// marks is what the book prints about an exercise beside the exercise itself.
// The corpus records three of these and 79 exercises carry one, and they are
// the reader's first clue about what they are in for.
func marks(ex *Exercise) string {
	var out []string
	if ex.Meta.Starred {
		out = append(out, "Bourbaki marks this one as harder than the rest")
	}
	if ex.Meta.Supplementary {
		out = append(out, "it is set in small type as supplementary")
	}
	if ex.Meta.HasHint {
		out = append(out, "a hint is printed with it")
	}
	if len(out) == 0 {
		return ""
	}
	s := strings.Join(out, ", ")
	return strings.ToUpper(s[:1]) + s[1:] + "."
}

// markWords is the same three facts in the form a list of nineteen rows wants.
// The sentence marks writes is right on the exercise's own page and is nineteen
// repetitions of itself on the page above.
func markWords(ex *Exercise) string {
	var out []string
	if ex.Meta.Starred {
		out = append(out, "harder")
	}
	if ex.Meta.Supplementary {
		out = append(out, "supplementary")
	}
	if ex.Meta.HasHint {
		out = append(out, "hint printed")
	}
	return strings.Join(out, ", ")
}

// solution writes the disclosure.
//
// <details> and no JavaScript. It is the element for exactly this, it works with
// the browser's own find-in-page once it is opened, and a reader who wants to
// think about the exercise first is not shown the answer by a script that has
// not loaded yet.
//
// The disclosure is written whether or not there is a solution, which is the
// point of building it now: M8 writes solutions and this template does not
// change when it does.
func (s *Site) solution(b *strings.Builder, ex *Exercise) {
	b.WriteString("<details class=\"solution\">\n<summary>Solution</summary>\n")
	if ex.Solution == nil {
		b.WriteString("<p class=\"none\">No solution has been written yet.</p>\n</details>\n")
		return
	}
	// The note is inside the disclosure and above the solution, so that it
	// cannot be read past, and it is not dismissible.
	fmt.Fprintf(b, "<p class=\"warn\">%s</p>\n", template.HTMLEscapeString(solutionNote(ex.Solution)))
	r := s.renderer(nil)
	r.File, r.Line = ex.Solution.Path, ex.Solution.Head
	body, err := r.HTML(ex.Solution.Body)
	if err != nil {
		// A solution is machine-written and a formula in one that will not set is
		// not a fault in the printed book, so it is shown as it stands rather than
		// stopping a build of the whole corpus over it.
		fmt.Fprintf(b, "<p class=\"warn\">This solution could not be rendered: %s</p>\n",
			template.HTMLEscapeString(err.Error()))
		b.WriteString("</details>\n")
		return
	}
	b.WriteString(body)
	b.WriteString("</details>\n")
}

// solutionNote is the standing note every solution carries.
func solutionNote(sol *Solution) string {
	who := "a machine"
	if sol.Meta.Model != "" {
		who = sol.Meta.Model
	}
	note := fmt.Sprintf("This solution was written by %s and judged by a machine. "+
		"It is not Bourbaki's and it has not been checked by a person.", who)
	if sol.Meta.Status != "" && sol.Meta.Status != corpus.StatusVerified {
		note += fmt.Sprintf(" It did not pass that judgement: %s.", sol.Meta.Status)
	}
	return note
}

// exerciseProvenance is the footer line, which says who wrote the text of the
// exercise and not who wrote the solution.
func exerciseProvenance(ex *Exercise) string {
	if ex.Meta.TranslatedFrom == "" {
		return "Transcribed from the printed volume. Not translated."
	}
	note := "Machine translation of the English"
	if ex.Meta.TranslationModel != "" {
		note += ", by " + ex.Meta.TranslationModel
	}
	return note + ". Not checked by a person."
}

// exerciseList is the page at <§>/ex/.
func (s *Site) exerciseList(set *exerciseSet) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "<h1>%s</h1>\n", template.HTMLEscapeString(set.title()))
	if set.Section != nil {
		fmt.Fprintf(&b, "<p class=\"where\">Bourbaki sets these at the end of <a href=%q>%s</a>.</p>\n",
			s.SectionURL(set.Section), template.HTMLEscapeString(heading(set.Section)))
	}
	solved := 0
	for _, ex := range set.Items {
		if ex.Solution != nil {
			solved++
		}
	}
	switch {
	case solved == 0:
		fmt.Fprintf(&b, "<p class=\"none\">%d exercises. No solution has been written for any of them yet.</p>\n",
			len(set.Items))
	default:
		fmt.Fprintf(&b, "<p class=\"none\">%d exercises, %d of them with a solution. Every solution here is "+
			"machine-written and machine-judged and is not Bourbaki's.</p>\n", len(set.Items), solved)
	}
	b.WriteString("<ul class=\"toc\">\n")
	for _, ex := range set.Items {
		fmt.Fprintf(&b, "<li><a href=%q>Exercise %d</a>", s.ExerciseURL(ex), ex.Meta.Exercise)
		if note := markWords(ex); note != "" {
			fmt.Fprintf(&b, " <span class=\"count\">%s</span>", template.HTMLEscapeString(note))
		}
		if ex.Solution != nil {
			b.WriteString(` <span class="count">solution</span>`)
		}
		b.WriteString("</li>\n")
	}
	b.WriteString("</ul>\n")

	var langs []langLink
	for _, l := range s.Langs {
		if !s.hasExercises(l, set.Book, set.Chapter, set.Dir) {
			continue
		}
		langs = append(langs, langLink{Lang: l, URL: s.ExercisesURL(l, set.Book, set.Chapter, set.Dir),
			Here: l == set.Lang, Draft: s.Draft[l]})
	}
	crumbs := []crumb{{set.Chapter, s.ChapterURL(set.Lang, set.Book, set.Chapter)}}
	if set.Section != nil {
		crumbs[0].Text = set.Section.Meta.BookTitle + ", Chapter " + set.Chapter
		crumbs = append(crumbs, crumb{heading(set.Section), s.SectionURL(set.Section)})
	}
	return s.render(layout{Title: set.title(), Lang: set.Lang, Langs: langs, Crumbs: crumbs,
		Content: template.HTML(b.String())})
}

// title is how the list page names itself.
func (set *exerciseSet) title() string {
	if set.Section != nil {
		return "Exercises for " + heading(set.Section)
	}
	// The § is not held in this language and the exercises are, which happens
	// while a translation is part way through. The name is built from the path
	// rather than left blank.
	if strings.HasPrefix(set.Dir, "a") {
		return "Exercises for Appendix " + strings.TrimPrefix(set.Dir, "a")
	}
	return "Exercises for § " + strings.TrimPrefix(set.Dir, "s")
}
