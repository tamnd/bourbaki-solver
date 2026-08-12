package publish

import (
	"bytes"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/katex"
	"github.com/tamnd/bourbaki-solver/quality"
)

// Build writes the whole site under out and returns the paths it wrote,
// sorted.
//
// It does not remove what is already there, so a page left behind by an earlier
// build under a name nothing writes any more survives. That is what the -clean
// flag is for, and it is why the workflow builds into an empty directory.
func (s *Site) Build(out string) ([]string, error) {
	var wrote []string
	write := func(rel, body string) error {
		p := filepath.Join(out, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
			return err
		}
		wrote = append(wrote, rel)
		return nil
	}
	if err := write("style.css", styleCSS); err != nil {
		return nil, err
	}
	// KaTeX writes markup that means nothing without its stylesheet and its
	// fonts, so they are part of the site rather than something a deployment
	// step is trusted to remember. They go under katex/ with the stylesheet at
	// the top of it, because the stylesheet asks for fonts/ relative to itself.
	err := fs.WalkDir(katex.Assets(), ".", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(katex.Assets(), p)
		if err != nil {
			return err
		}
		return write(path.Join("katex", p), string(b))
	})
	if err != nil {
		return nil, err
	}

	page, err := s.front()
	if err != nil {
		return nil, err
	}
	if err := write("index.html", page); err != nil {
		return nil, err
	}

	about, err := s.about()
	if err != nil {
		return nil, err
	}
	if err := write("about/index.html", about); err != nil {
		return nil, err
	}

	for _, sec := range s.Sections {
		body, err := s.section(sec)
		if err != nil {
			// Not wrapped with the path. The renderer names the file and the
			// line itself, since it is the only thing here that knows the line,
			// and a wrap would print the path twice on the one message anybody
			// ever sees from this.
			return nil, err
		}
		rel := filepath.ToSlash(filepath.Join(sec.Lang, sec.Meta.Book, sec.Meta.Chapter, sec.Slug, "index.html"))
		if err := write(rel, body); err != nil {
			return nil, err
		}
	}

	for _, ch := range s.chapters() {
		body, err := s.chapter(ch)
		if err != nil {
			return nil, err
		}
		rel := filepath.ToSlash(filepath.Join(ch.Lang, ch.Book, ch.Chapter, "index.html"))
		if err := write(rel, body); err != nil {
			return nil, err
		}
	}

	for _, set := range s.exerciseSets() {
		body, err := s.exerciseList(set)
		if err != nil {
			return nil, err
		}
		rel := filepath.ToSlash(filepath.Join(set.Lang, set.Book, set.Chapter, set.Dir, "ex", "index.html"))
		if err := write(rel, body); err != nil {
			return nil, err
		}
	}

	for _, ex := range s.Exercises {
		body, err := s.exercisePage(ex, "")
		if err != nil {
			return nil, err
		}
		rel := filepath.ToSlash(filepath.Join(ex.Lang, ex.Meta.Book, ex.Meta.Chapter, ex.Dir,
			"ex", strconv.Itoa(ex.Meta.Exercise), "index.html"))
		if err := write(rel, body); err != nil {
			return nil, err
		}
	}

	for _, st := range s.Statements {
		body, err := s.tag(st)
		if err != nil {
			return nil, fmt.Errorf("tag %s: %w", st.Tag, err)
		}
		if err := write(filepath.ToSlash(filepath.Join("tag", st.Tag, "index.html")), body); err != nil {
			return nil, err
		}
	}

	// An exercise carries a tag as a statement does, and a tag that resolves to
	// nothing is the one thing the tag scheme promises will not happen. The page
	// is the exercise's own page, served at the second URL and declaring the
	// first as canonical, because the exercise page is already everything a tag
	// page would hold and a second version of it would be a second thing to keep
	// right.
	for _, ex := range s.Exercises {
		if ex.Meta.Tag == "" || s.byExTag[ex.Meta.Tag] != ex {
			continue
		}
		body, err := s.exercisePage(ex, s.ExerciseURL(ex))
		if err != nil {
			return nil, err
		}
		if err := write(filepath.ToSlash(filepath.Join("tag", ex.Meta.Tag, "index.html")), body); err != nil {
			return nil, err
		}
	}

	// The reports of the corpus, rendered. They are committed Markdown like the
	// rest and they are the part of it that says what is wrong with it.
	rs, err := s.reports()
	if err != nil {
		return nil, err
	}
	at := s.sitewide()
	for _, r := range rs {
		body, err := s.reportPage(r, at)
		if err != nil {
			return nil, err
		}
		if err := write(path.Join("reports", r.Name, "index.html"), body); err != nil {
			return nil, err
		}
	}
	index, err := s.reportIndex(rs)
	if err != nil {
		return nil, err
	}
	if err := write("reports/index.html", index); err != nil {
		return nil, err
	}

	table, err := s.tagTable()
	if err != nil {
		return nil, err
	}
	if err := write("tags/index.html", table); err != nil {
		return nil, err
	}

	// One index per language, fetched by the search page and by nothing else.
	// They are the only files of the site that are not HTML a browser renders,
	// and they are built here rather than by a step of the workflow so that the
	// site in a directory is the whole site.
	for _, lang := range s.Langs {
		body, err := s.searchJSON(lang)
		if err != nil {
			return nil, err
		}
		if err := write(path.Join("search", lang+".json"), body); err != nil {
			return nil, err
		}
	}
	search, err := s.searchPage()
	if err != nil {
		return nil, err
	}
	if err := write("search/index.html", search); err != nil {
		return nil, err
	}
	sort.Strings(wrote)
	return wrote, nil
}

// chapterKey is one chapter of one Book in one language.
type chapterKey struct{ Lang, Book, BookTitle, Chapter, Title string }

func (s *Site) chapters() []chapterKey {
	seen := map[chapterKey]bool{}
	var out []chapterKey
	for _, sec := range s.Sections {
		k := chapterKey{sec.Lang, sec.Meta.Book, sec.Meta.BookTitle, sec.Meta.Chapter, sec.Meta.ChapterTitle}
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Lang != b.Lang {
			return a.Lang < b.Lang
		}
		if a.Book != b.Book {
			return a.Book < b.Book
		}
		return a.Chapter < b.Chapter
	})
	return out
}

// renderer is how a body is turned into HTML, with the tag links pointed back
// into this site and the body's own relative links resolved against the chapter
// the body was written in.
func (s *Site) renderer(sec *Section) Renderer {
	r := Renderer{Link: s.TagURL, Line: 1}
	if sec != nil {
		r.Dir = s.ChapterURL(sec.Lang, sec.Meta.Book, sec.Meta.Chapter)
		r.Exercises = s.exerciseHref(sec.Lang, sec.Meta.Book, sec.Meta.Chapter)
		r.File, r.Line = sec.Path, sec.Head
		if r.Line == 0 {
			r.Line = 1
		}
	}
	if s.AllowBrokenMath {
		r.OnRefusal = s.marked
	}
	return r
}

// marked is the page a refused formula gets under -allow-broken-math: the TeX
// as the corpus has it, in a form that says on its face that it is broken. The
// same span is rendered on more than one page, so the list this keeps is the
// pages it appears on and not the count of distinct faults; the audit's P04 is
// what counts those.
func (s *Site) marked(ref Refusal) string {
	s.Broken = append(s.Broken, ref)
	el, class := "span", "math broken"
	if ref.Display {
		el, class = "div", "math display broken"
	}
	// The title carries KaTeX's message, which carries the TeX, which is full
	// of backslashes. %q would escape them the way Go writes a string literal
	// and the browser would print the escapes, so the attribute is escaped as
	// HTML and not as Go.
	return fmt.Sprintf(`<%s class="%s" title="%s">%s</%s>`, el, class,
		template.HTMLEscapeString("this formula is broken in the corpus and could not be set: "+ref.Why),
		template.HTMLEscapeString(delim(ref)), el)
}

// delim writes a span back the way the Markdown has it, delimiters and all,
// since what is being shown is the source rather than the mathematics.
func delim(ref Refusal) string {
	if ref.Display {
		return "$$" + ref.Span + "$$"
	}
	return "$" + ref.Span + "$"
}

type layout struct {
	Title string
	// Lang is what goes in the lang attribute of the document. It is the
	// language of the text on the page and not of the site, which matters here
	// more than on most sites: a screen reader that says a Vietnamese page in
	// English pronunciation reads it as noise.
	Lang   string
	Home   string
	Search string
	About  string
	CSS    string
	KaTeX  string
	// Canonical is set on a page that is a second copy of another one, which is
	// what an exercise's tag URL is. Empty everywhere else.
	Canonical string
	Crumbs    []crumb
	Content   template.HTML
	Langs     []langLink
	Note      string
}

type crumb struct {
	Text string
	URL  string
}

type langLink struct {
	Lang  string
	URL   string
	Here  bool
	Draft bool
}

func (s *Site) render(l layout) (string, error) {
	l.Home = s.url()
	l.Search = s.url("search")
	// Every page links the about page, because every page of this site shows
	// text a machine had a hand in and P07 is that such a page says so. The line
	// at the foot says it for that page and the about page says it for the
	// corpus, and a reader who lands on a tag page from somebody else's citation
	// has to be one click from both.
	l.About = s.url("about")
	l.CSS = s.url() + "style.css"
	// Every page of this site has mathematics on it, including the ones that
	// only list statements, so the stylesheet is linked unconditionally rather
	// than page by page. It is 25 KB and it is the same 25 KB every time, which
	// the browser caches once for the whole site.
	l.KaTeX = s.url() + "katex/katex.min.css"
	if l.Lang == "" {
		l.Lang = "en"
	}
	var b bytes.Buffer
	if err := pageTmpl.Execute(&b, l); err != nil {
		return "", err
	}
	return b.String(), nil
}

func (s *Site) section(sec *Section) (string, error) {
	body, err := s.renderer(sec).HTML(sec.Body)
	if err != nil {
		return "", err
	}
	langs := s.langLinks(sec.Lang, func(lang string) string {
		if other := s.sectionIn(lang, sec); other != nil {
			return s.SectionURL(other)
		}
		return ""
	})
	return s.render(layout{
		Title: heading(sec),
		Lang:  sec.Lang,
		Crumbs: []crumb{
			{sec.Meta.BookTitle + ", Chapter " + sec.Meta.Chapter,
				s.ChapterURL(sec.Lang, sec.Meta.Book, sec.Meta.Chapter)},
		},
		Langs:   langs,
		Note:    provenance(sec),
		Content: template.HTML(body),
	})
}

// hasChapter says whether a language holds any of a chapter.
func (s *Site) hasChapter(lang, book, chapter string) bool {
	for _, sec := range s.Sections {
		if sec.Lang == lang && sec.Meta.Book == book && sec.Meta.Chapter == chapter {
			return true
		}
	}
	return false
}

// sectionIn finds the same § in another language, by Book, chapter and slug.
func (s *Site) sectionIn(lang string, sec *Section) *Section {
	for _, o := range s.Sections {
		if o.Lang == lang && o.Meta.Book == sec.Meta.Book && o.Meta.Chapter == sec.Meta.Chapter && o.Slug == sec.Slug {
			return o
		}
	}
	return nil
}

// provenance is the line every page carries saying who wrote the text on it. A
// reader deciding whether to trust a sentence has more right to this than the
// audit does.
func provenance(sec *Section) string {
	if sec.Meta.TranslatedFrom == "" {
		// The French volume carries no edition in the books manifest, so the
		// line says what is known and does not invent the rest.
		if sec.Meta.SourceEdition == "" {
			return "Transcribed from the printed volume. Not translated."
		}
		return fmt.Sprintf("Transcribed from the printed edition, %s. Not translated.", sec.Meta.SourceEdition)
	}
	note := fmt.Sprintf("Machine translation of the English, by %s", sec.Meta.TranslationModel)
	if sec.Meta.GlossaryVersion > 0 {
		note += fmt.Sprintf(", against glossary version %d", sec.Meta.GlossaryVersion)
	}
	return note + ". Not checked by a person." + cutDown(sec.Meta.TranslationModel)
}

// cutDown is the extra sentence a page written by a smaller model carries.
//
// The rule is quality.SmallModel, which is L08's rule, because two answers to
// "is this a cut down model" is one answer too many. The audit knows this and
// records it as a soft finding, and the reader of the page has more right to it
// than the audit does: nobody chose the model, the account was moved down
// between one section and the next, and the only place that fact reaches
// somebody reading the Vietnamese is here.
func cutDown(model string) string {
	if !quality.SmallModel(model) {
		return ""
	}
	return " That model is a cut down version of the one the rest of this language was written by."
}

func (s *Site) chapter(k chapterKey) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "<h1>%s, Chapter %s</h1>\n<p class=\"subtitle\">%s</p>\n<ul class=\"toc\">\n",
		template.HTMLEscapeString(k.BookTitle), template.HTMLEscapeString(k.Chapter),
		template.HTMLEscapeString(k.Title))
	for _, sec := range s.Sections {
		if sec.Lang != k.Lang || sec.Meta.Book != k.Book || sec.Meta.Chapter != k.Chapter {
			continue
		}
		fmt.Fprintf(&b, "<li><a href=%q>%s</a> <span class=\"count\">%d statements, ",
			s.SectionURL(sec), template.HTMLEscapeString(heading(sec)), sec.Meta.Statements)
		// The exercise count is a link where the exercises are held and plain
		// text where they are not, so the contents page says which §§ a reader
		// can actually go to the exercises of.
		if s.hasExercises(sec.Lang, sec.Meta.Book, sec.Meta.Chapter, sec.Slug) {
			fmt.Fprintf(&b, "<a href=%q>%d exercises</a>",
				s.ExercisesURL(sec.Lang, sec.Meta.Book, sec.Meta.Chapter, sec.Slug), sec.Meta.Exercises)
		} else {
			fmt.Fprintf(&b, "%d exercises", sec.Meta.Exercises)
		}
		b.WriteString("</span></li>\n")
	}
	b.WriteString("</ul>\n")
	// A language holds a chapter or it does not, and the switcher on a contents
	// page was the one place that assumed every language holds every chapter.
	// That is true of the corpus today, with one chapter in it, and it stops
	// being true the first time a second chapter is transcribed in one language
	// and not another.
	langs := s.langLinks(k.Lang, func(lang string) string {
		if !s.hasChapter(lang, k.Book, k.Chapter) {
			return ""
		}
		return s.ChapterURL(lang, k.Book, k.Chapter)
	})
	return s.render(layout{Title: k.BookTitle + ", Chapter " + k.Chapter, Lang: k.Lang,
		Langs: langs, Content: template.HTML(b.String())})
}

// printed is where the book puts a statement, written the way the book writes
// it. Bourbaki cites itself as "VIII, p. 378", so a page that wants to be cited
// alongside the volume says the same thing rather than inventing a form.
func printed(sec *Section, st *Statement) string {
	if st.Page == 0 {
		return sec.Meta.BookTitle + ", chapter " + sec.Meta.Chapter
	}
	return fmt.Sprintf("%s, %s, p. %d", sec.Meta.BookTitle, sec.Meta.Chapter, st.Page)
}

// heading is how a § is named in a contents list.
//
// It comes out of the front matter, which the translations leave in English on
// purpose so that the manifests join across languages. So a Vietnamese page is
// headed in English here and in Vietnamese in the body below, where the printed
// title is. That is the corpus as it stands rather than a decision taken here,
// and it is the sort of thing the language switcher makes visible.
func heading(sec *Section) string {
	switch sec.Meta.Kind {
	case "front", "historical":
		return sec.Meta.SectionTitle
	}
	if sec.Meta.Appendix {
		return fmt.Sprintf("Appendix %d. %s", sec.Meta.Section, sec.Meta.SectionTitle)
	}
	return fmt.Sprintf("§ %d. %s", sec.Meta.Section, sec.Meta.SectionTitle)
}

// short is the same name with the title left off.
func short(sec *Section) string {
	switch sec.Meta.Kind {
	case "front", "historical":
		return sec.Meta.SectionTitle
	}
	if sec.Meta.Appendix {
		return fmt.Sprintf("Appendix %d", sec.Meta.Section)
	}
	return fmt.Sprintf("§ %d", sec.Meta.Section)
}

func (s *Site) tag(st *Statement) (string, error) {
	en := st.Sections["en"]
	var b strings.Builder

	lang := "en"
	if en == nil {
		// A tag with no English is not a shape the corpus has today. It is a
		// shape it can have, since French is an extraction of its own, so the
		// page picks whatever language holds it rather than failing the build.
		for _, l := range s.Langs {
			if st.Sections[l] != nil {
				lang, en = l, st.Sections[l]
				break
			}
		}
	}
	if en == nil {
		return "", fmt.Errorf("no language holds it")
	}
	u := st.Units[lang]
	// The renderer is built after the language is settled, so that a tag held
	// in French alone resolves its links against the French chapter and names
	// the French file when a formula in it will not render.
	r := s.renderer(en)
	// The unit is a slice of the section body starting under its heading, so
	// its line one is not the section's, and a formula reported against this
	// page has to name the line the file actually has it on.
	r.Line += u.Heading.Line + 1

	fmt.Fprintf(&b, "<h1>%s</h1>\n", template.HTMLEscapeString(u.Heading.Text))
	fmt.Fprintf(&b, "<p class=\"tagline\">Tag <code>%s</code>, permanent. "+
		"Cite it as <code>[Bourbaki, Tag %s]</code>.</p>\n", st.Tag, st.Tag)
	fmt.Fprintf(&b, "<p class=\"where\">%s, <a href=%q>%s</a></p>\n",
		template.HTMLEscapeString(printed(en, st)), s.SectionURL(en)+"#"+st.Label,
		template.HTMLEscapeString(heading(en)))

	body, err := r.HTML(u.Body)
	if err != nil {
		return "", err
	}
	b.WriteString(`<div class="statement-body">` + body + "</div>\n")
	b.WriteString("<p class=\"caveat\">Bourbaki sets the proof directly after the statement, and the corpus " +
		"does not separate them, so what is above is the statement and its proof together.</p>\n")

	s.list(&b, "Cited by", s.CitedBy[st.Tag], s.from)
	s.list(&b, "Cites", s.Cites[st.Tag], s.to)

	// Spec 12 §7: a tag is offered in a language only where a file in that
	// language holds it. Vietnamese has two §§ of twenty seven, so it is on the
	// switcher of the statements of those two and nowhere else.
	langs := s.langLinks(lang, func(l string) string {
		if st.Sections[l] == nil {
			return ""
		}
		return s.SectionURL(st.Sections[l]) + "#" + st.Label
	})
	// The corpus has 51 headings that read only "Corollary", so the title of the
	// window and of a search result carries the § as well as the tag.
	return s.render(layout{Title: fmt.Sprintf("%s, %s (%s)", u.Heading.Text, short(en), st.Tag),
		Lang: lang, Langs: langs, Note: provenance(en), Content: template.HTML(b.String())})
}

// row is one line of a reference list. Title is what a reader gets on hover and
// is the label, which is the string to paste into a search or a bug report.
type row struct {
	URL   string
	Text  string
	Title string
	Note  string
}

// from is the citing end of an edge, which is what the cited-by list shows.
func (s *Site) from(e *Edge) row {
	if st := s.Tag(e.FromTag); st != nil {
		return row{URL: s.TagURL(e.FromTag), Text: s.name(st), Title: e.FromLabel}
	}
	// A whole § can cite a statement, in prose that stands outside every
	// statement of it. That is a real citation and it gets a real link, to the
	// § rather than to a tag, because a § has no tag.
	if sec := s.Section(e.FromLabel); sec != nil {
		return row{URL: s.SectionURL(sec), Text: heading(sec), Title: e.FromLabel}
	}
	if ex := s.Exercise(e.FromLabel); ex != nil {
		return row{URL: s.ExerciseURL(ex), Text: ex.Name(), Title: e.FromLabel}
	}
	return row{Text: e.FromLabel, Note: note(e, e.FromLabel)}
}

// to is the cited end, which the cites list shows. The text is the reference as
// Bourbaki wrote it, since what a proof says is the thing worth reading, and
// where it lands is on the link.
func (s *Site) to(e *Edge) row {
	if st := s.Tag(e.ToTag); st != nil {
		return row{URL: s.TagURL(e.ToTag), Text: e.Raw, Title: s.name(st) + " (" + e.ToLabel + ")"}
	}
	if sec := s.Section(e.ToLabel); sec != nil {
		return row{URL: s.SectionURL(sec), Text: e.Raw, Title: heading(sec)}
	}
	if ex := s.Exercise(e.ToLabel); ex != nil {
		return row{URL: s.ExerciseURL(ex), Text: e.Raw, Title: ex.Name() + " (" + e.ToLabel + ")"}
	}
	return row{Text: e.Raw, Note: note(e, e.ToLabel)}
}

// name is how a statement is written when it is named on another page. The
// label is machine-readable and unreadable, and there are 51 headings that say
// only "Corollary", so it is the heading and the § together.
func (s *Site) name(st *Statement) string {
	for _, lang := range []string{"en", "fr"} {
		if sec := st.Sections[lang]; sec != nil {
			return st.Units[lang].Heading.Text + ", " + short(sec)
		}
	}
	return st.Label
}

// list writes one of the two reference blocks of a tag page.
func (s *Site) list(b *strings.Builder, title string, edges []*Edge, of func(*Edge) row) {
	if len(edges) == 0 {
		fmt.Fprintf(b, "<h2>%s</h2>\n<p class=\"none\">Nothing.</p>\n", title)
		return
	}
	fmt.Fprintf(b, "<h2>%s</h2>\n<ul class=\"refs\">\n", title)
	for _, e := range edges {
		r := of(e)
		text := template.HTMLEscapeString(r.Text)
		if r.URL == "" {
			fmt.Fprintf(b, "<li>%s <span class=\"out\">%s</span></li>\n", text, r.Note)
			continue
		}
		if r.Title != "" {
			// The title is a heading out of the corpus and can hold a quote or
			// a backslash, so it is escaped as HTML rather than written with %q,
			// which escapes for Go and would end the attribute early.
			fmt.Fprintf(b, "<li><a href=%q title=\"%s\">%s</a></li>\n",
				r.URL, template.HTMLEscapeString(r.Title), text)
			continue
		}
		fmt.Fprintf(b, "<li><a href=%q>%s</a></li>\n", r.URL, text)
	}
	b.WriteString("</ul>\n")
}

// note says why a reference has no link, which is not always the same reason. A
// citation of a Book this corpus does not hold is a fact about the printed
// volume and stays visible.
func note(e *Edge, label string) string {
	if e.Book != "" {
		return template.HTMLEscapeString("not in this corpus: " + e.Book + " " + e.Chapter)
	}
	return missing(label)
}

// missing says why a label has no page of its own. An exercise gets its own
// wording, because an exercise reaching here is one the extraction did not pick
// up rather than a whole volume nobody has transcribed, and the two are fixed by
// different work.
func missing(label string) string {
	if ref, err := corpus.ParseLabel(label); err == nil && ref.Kind == corpus.KindExercise {
		return "an exercise the corpus does not hold"
	}
	return "not in this corpus"
}

func (s *Site) tagTable() (string, error) {
	entries, retired, err := s.Tags()
	if err != nil {
		return "", err
	}
	var b strings.Builder
	fmt.Fprintf(&b, "<h1>Tags</h1>\n<p>%d tags, %d retired. A tag is assigned once and never reused, "+
		"so a citation of one stays good across every renumbering of the text.</p>\n",
		len(entries), len(retired))
	b.WriteString("<table class=\"tags\">\n<tr><th>tag</th><th>statement</th></tr>\n")
	for _, e := range entries {
		label := template.HTMLEscapeString(e.Label)
		// A statement and an exercise both have a tag page, and the row is the
		// same row either way: the promise a tag makes is that the URL resolves,
		// not that what it resolves to is a theorem.
		if s.Tag(string(e.Tag)) != nil || s.ExerciseTag(string(e.Tag)) != nil {
			fmt.Fprintf(&b, "<tr><td><a href=%q>%s</a></td><td>%s</td></tr>\n",
				s.TagURL(string(e.Tag)), e.Tag, label)
			continue
		}
		// A tag with no page is printed and marked rather than left out, since a
		// tag table that quietly holds back a third of the tags is worse than no
		// table.
		fmt.Fprintf(&b, "<tr><td>%s</td><td>%s <span class=\"out\">%s</span></td></tr>\n",
			e.Tag, label, missing(e.Label))
	}
	b.WriteString("</table>\n")
	return s.render(layout{Title: "Tags", Content: template.HTML(b.String())})
}

func (s *Site) front() (string, error) {
	var b strings.Builder
	b.WriteString("<h1>Bourbaki, Éléments de mathématique</h1>\n")
	b.WriteString("<p>A Markdown corpus of the <em>Éléments</em> with a permanent tag on every numbered " +
		"statement, in the manner of the Stacks Project. This is a study project and it is not Bourbaki's " +
		"and not Springer's.</p>\n")
	b.WriteString("<h2>What is here</h2>\n<p>One chapter of one Book, and not the forty three volumes the " +
		"library holds. The table is generated from the corpus and says what is actually in it.</p>\n")
	b.WriteString("<table class=\"coverage\">\n<tr><th>language</th><th>sections</th><th>statements</th>" +
		"<th>exercises</th><th></th></tr>\n")
	for _, n := range s.numbers() {
		note := n.made()
		if s.Draft[n.Lang] {
			note += ", below the coverage floor"
		}
		fmt.Fprintf(&b, "<tr><td><a href=%q>%s</a> <span class=\"count\">%s</span></td>"+
			"<td>%d</td><td>%d</td><td>%d</td><td>%s</td></tr>\n",
			s.langHome(n.Lang), n.Lang, langName(n.Lang), n.Sections, n.Statements, n.Exercises, note)
	}
	b.WriteString("</table>\n")
	// The floor is stated here because this table is the one place a language it
	// applies to is offered at all. A reader who wants the two Vietnamese §§ can
	// have them; what the floor stops is the rest of the site offering Vietnamese
	// on twenty five pages that do not have it. The rule itself is on the about
	// page, with the share of the English that each language actually holds.
	if s.drafted() {
		fmt.Fprintf(&b, "<p>A language holding under %d per cent of the English is linked here and is "+
			"left out of the switcher on the pages themselves, because a switcher that offers a language "+
			"and then has nothing to switch to is worse than one that does not offer it. <a href=%q>About</a> "+
			"has the real share each language holds.</p>\n", int(coverageFloor*100), s.url("about"))
	}
	// The tag count is the tag file's and not the number of statements. 317 of
	// the tags are on an exercise, so a front page that counted the statements
	// and called them tags would be out by a third.
	set, err := s.TagSet()
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "<p><a href=%q>All %d tags</a>, <a href=%q>search</a> over the statements and the "+
		"exercises, and <a href=%q>about</a> for how the text got here and what a machine wrote. Of the "+
		"%d references the chapter makes, %d do not resolve, and %d of the %d tagged statements are cited "+
		"by nothing. Those are the numbers worth watching and the <a href=%q>reports</a> name every one "+
		"of them.</p>\n",
		s.url("tags"), len(set.Tags), s.url("search"), s.url("about"),
		s.Edges, s.Unresolved, s.uncited(), len(s.Statements), s.url("reports"))
	return s.render(layout{Title: "Bourbaki", Content: template.HTML(b.String())})
}

// langHome is where the front page sends a language. One chapter is held, so it
// is that chapter; when there is more than one this becomes a contents page.
func (s *Site) langHome(lang string) string {
	for _, sec := range s.Sections {
		if sec.Lang == lang {
			return s.ChapterURL(lang, sec.Meta.Book, sec.Meta.Chapter)
		}
	}
	return s.url()
}

var pageTmpl = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html lang="{{.Lang}}">
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
{{if .Canonical}}<link rel="canonical" href="{{.Canonical}}">
{{end}}<link rel="stylesheet" href="{{.KaTeX}}">
<link rel="stylesheet" href="{{.CSS}}">
<body>
<header>
<span class="site"><a class="home" href="{{.Home}}">Bourbaki</a> <a href="{{.Search}}">Search</a> <a href="{{.About}}">About</a></span>
{{if .Langs}}<nav class="langs">{{range .Langs}}{{if .Here}}<span class="here">{{.Lang}}</span>{{else if .Draft}}<a class="draft" href="{{.URL}}" title="below the coverage floor">{{.Lang}}</a>{{else}}<a href="{{.URL}}">{{.Lang}}</a>{{end}}{{end}}</nav>{{end}}
</header>
<main>
{{if .Crumbs}}<nav class="crumbs">{{range $i, $c := .Crumbs}}{{if $i}}<span class="sep">›</span>{{end}}<a href="{{$c.URL}}">{{$c.Text}}</a>{{end}}</nav>
{{end}}{{.Content}}
</main>
{{if .Note}}<footer class="note">{{.Note}}</footer>{{end}}
</body>
</html>
`))

const styleCSS = `:root { --ink: #111; --thin: #666; --rule: #ddd; }
* { box-sizing: border-box; }
body { margin: 0 auto; max-width: 44rem; padding: 1rem 1.2rem 4rem;
  font: 16px/1.6 Georgia, "Times New Roman", serif; color: var(--ink); }
header { display: flex; justify-content: space-between; align-items: baseline;
  border-bottom: 1px solid var(--rule); padding-bottom: .4rem; margin-bottom: 1.6rem;
  font-family: system-ui, sans-serif; font-size: .85rem; }
a { color: #10389a; }
h1 { font-size: 1.5rem; line-height: 1.3; }
h2 { font-size: 1.15rem; margin-top: 2rem; }
h3 { font-size: 1rem; }
h4 { font-size: 1rem; margin-top: 1.6rem; }
h4.statement { font-variant: small-caps; }
.tag { font-family: ui-monospace, monospace; font-size: .8em; text-decoration: none;
  border: 1px solid var(--rule); border-radius: 3px; padding: 0 .25em; }
/* KaTeX sets the mathematics itself. What is left here is the one thing it
   cannot know: a display wider than a phone has to scroll rather than push the
   page sideways. */
.math.display { margin: 1rem 0; overflow-x: auto; overflow-y: hidden; padding: .2rem 0; }
.katex { font-size: 1.05em; }
/* Only ever seen under -allow-broken-math, and loud on purpose: this is the
   source of a formula that did not survive the text layer, and it must not be
   mistaken for a formula somebody meant to write this way. */
.broken { font-family: ui-monospace, monospace; font-size: .85em; color: #a11;
  background: #fee; border: 1px solid #a11; border-radius: 3px; padding: 0 .2em; }
.site a { margin-right: .7rem; }
.langs a, .langs .here { margin-left: .5rem; }
.langs .here { font-weight: bold; }
.langs .draft { opacity: .55; }
.crumbs { font-family: system-ui, sans-serif; font-size: .8rem; margin-bottom: 1rem; }
/* The separator is its own element rather than a ::before on the link, so that
   it is not underlined with the link and is not part of what a click hits. */
.crumbs .sep { color: var(--thin); margin: 0 .4em; }
.tagline, .where, .caveat, .note, .subtitle, .marks { color: var(--thin); font-size: .88rem; }
/* The solution is behind a disclosure and nothing here opens it. A reader who
   wants to think about the exercise first should not have the answer on the
   screen, and a reader who wants the answer is one click away. */
details.solution { border: 1px solid var(--rule); border-radius: 3px; padding: .6rem .8rem; margin: 2rem 0; }
details.solution summary { cursor: pointer; font-family: system-ui, sans-serif; font-size: .9rem; }
details.solution[open] summary { border-bottom: 1px solid var(--rule); padding-bottom: .4rem; margin-bottom: .6rem; }
.warn { font-size: .85rem; color: #a11; }
.caveat, .note { border-top: 1px solid var(--rule); padding-top: .6rem; margin-top: 2rem; }
.statement-body { border-left: 3px solid var(--rule); padding-left: 1rem; }
ul.refs, ul.toc { padding-left: 1.2rem; }
ul.toc li { margin: .3rem 0; }
.count, .out, .none { color: var(--thin); font-size: .85rem; }
/* Search is the one page with a script on it, and the one page with a form. */
#f { display: flex; gap: .5rem; margin: 1.5rem 0 1rem; font-family: system-ui, sans-serif; }
#q { flex: 1; font: inherit; padding: .4rem .6rem; border: 1px solid var(--rule); border-radius: 3px; }
#lang, #f button { font: inherit; padding: .4rem .6rem; border: 1px solid var(--rule);
  border-radius: 3px; background: #fff; }
ol.results { padding-left: 1.4rem; }
ol.results li { margin: 1rem 0; }
.snippet { margin: .2rem 0 0; font-size: .9rem; color: var(--thin); }
table { border-collapse: collapse; width: 100%; font-size: .9rem; }
/* The language cell is a code and the name of the language and the two are one
   thing, so they do not break across two lines while the rest of the row is
   one. */
.coverage td:first-child { white-space: nowrap; }
/* A report table is words on the left and counts on the right, and a count that
   is not right aligned is a column nobody can compare down. */
table.report .num { text-align: right; white-space: nowrap; }
table.report td:first-child { white-space: nowrap; }
th, td { text-align: left; padding: .25rem .5rem; border-bottom: 1px solid var(--rule); }
code { font-family: ui-monospace, monospace; font-size: .9em; }
`
