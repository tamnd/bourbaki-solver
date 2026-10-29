package publish

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"html/template"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/katex"
	"github.com/tamnd/bourbaki-solver/mathtex"
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
	s.Unreadable = nil // what a build found, not what every build so far found
	write := func(rel, body string) error {
		p := filepath.Join(out, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			return err
		}
		if s.Gzip && !strings.HasSuffix(rel, ".woff2") {
			var err error
			if body, err = gzipped(body); err != nil {
				return err
			}
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
	if err := write("site.js", siteJS); err != nil {
		return nil, err
	}
	if err := write("robots.txt", "User-agent: *\nAllow: /\n"); err != nil {
		return nil, err
	}
	// The page a host serves for an address nothing was written at.
	missing, err := s.render(layout{Title: "Not found · Bourbaki", Lang: "en", Wide: true,
		Content: template.HTML(fmt.Sprintf("<section class=\"hero\"><p class=\"eyebrow\">404</p>"+
			"<h1 class=\"display\">Not found</h1><p class=\"lede\">Nothing is published at this address. "+
			"A section may have moved when its chapter was read again; its permanent tag has not.</p>"+
			"<p class=\"lede\"><a href=%q>Library</a> · <a href=%q>Search</a> · <a href=%q>Tags</a></p></section>",
			s.url(), s.url("search"), s.url("tags")))})
	if err != nil {
		return nil, err
	}
	if err := write("404.html", missing); err != nil {
		return nil, err
	}
	// KaTeX writes markup that means nothing without its stylesheet and its
	// fonts, so they are part of the site rather than something a deployment
	// step is trusted to remember. They go under katex/ with the stylesheet at
	// the top of it, because the stylesheet asks for fonts/ relative to itself.
	// The text fonts go under fonts/, which is where style.css asks for them.
	err = fs.WalkDir(fontFiles, "assets/fonts", func(p string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return err
		}
		b, err := fs.ReadFile(fontFiles, p)
		if err != nil {
			return err
		}
		return write(path.Join("fonts", path.Base(p)), string(b))
	})
	if err != nil {
		return nil, err
	}
	err = fs.WalkDir(katex.Assets(), ".", func(p string, d fs.DirEntry, err error) error {
		if s.MathML {
			// MathML needs neither the KaTeX stylesheet nor its fonts.
			return fs.SkipAll
		}
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

	for _, lang := range s.Langs {
		page, err := s.library(lang)
		if err != nil {
			return nil, err
		}
		if err := write(path.Join(lang, "index.html"), page); err != nil {
			return nil, err
		}
		for _, sh := range s.books(lang) {
			page, err := s.bookPage(sh)
			if err != nil {
				return nil, err
			}
			if err := write(path.Join(lang, sh.Book, "index.html"), page); err != nil {
				return nil, err
			}
			if err := write(path.Join(lang, sh.Book, "nav.html"), s.nav(sh)); err != nil {
				return nil, err
			}
		}
	}

	about, err := s.about()
	if err != nil {
		return nil, err
	}
	if err := write("about/index.html", about); err != nil {
		return nil, err
	}

	// keep records a page whose body the build could not set and says to carry
	// on to the next one. Everything below it renders a corpus body and can
	// meet a file the corpus has broken, and stopping on the first of those is
	// what made a ten minute check into one run per fault. A failure to write
	// is a different thing and still returns: the disk being full is not a
	// finding about the corpus.
	//
	// The error is not wrapped with the path. The renderer names the file and
	// the line itself, since it is the only thing here that knows the line, and
	// a wrap would print the path twice on the one message anybody reads.
	keep := func(rel string, err error) bool {
		if err != nil {
			s.Unreadable = append(s.Unreadable, Unreadable{Rel: rel, Err: err})
			return false
		}
		return true
	}

	if err := s.warm(); err != nil {
		return nil, err
	}
	for _, sec := range s.Sections {
		rel := strings.TrimPrefix(s.SectionURL(sec), s.url()) + "index.html"
		body, err := s.section(sec)
		if !keep(rel, err) {
			continue
		}
		if err := write(rel, body); err != nil {
			return nil, err
		}
	}

	for _, ch := range s.chapters() {
		if ch.Chapter == "" {
			continue // the Book's own page, written above
		}
		rel := filepath.ToSlash(filepath.Join(ch.Lang, ch.Book, ch.Chapter, "index.html"))
		body, err := s.chapter(ch)
		if !keep(rel, err) {
			continue
		}
		if err := write(rel, body); err != nil {
			return nil, err
		}
	}

	for _, set := range s.exerciseSets() {
		rel := filepath.ToSlash(filepath.Join(set.Lang, set.Book, set.Chapter, set.Dir, "ex", "index.html"))
		body, err := s.exerciseList(set)
		if !keep(rel, err) {
			continue
		}
		if err := write(rel, body); err != nil {
			return nil, err
		}
	}

	for _, ex := range s.Exercises {
		rel := filepath.ToSlash(filepath.Join(ex.Lang, ex.Meta.Book, ex.Meta.Chapter, ex.Dir,
			"ex", strconv.Itoa(ex.Meta.Exercise), "index.html"))
		body, err := s.exercisePage(ex, "")
		if !keep(rel, err) {
			continue
		}
		if err := write(rel, body); err != nil {
			return nil, err
		}
	}

	for _, st := range s.Statements {
		rel := filepath.ToSlash(filepath.Join("tag", st.Tag, "index.html"))
		body, err := s.tag(st)
		if !keep(rel, err) {
			continue
		}
		if err := write(rel, body); err != nil {
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
		rel := filepath.ToSlash(filepath.Join("tag", ex.Meta.Tag, "index.html"))
		body, err := s.exercisePage(ex, s.ExerciseURL(ex))
		if !keep(rel, err) {
			continue
		}
		if err := write(rel, body); err != nil {
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
		rel := path.Join("reports", r.Name, "index.html")
		body, err := s.reportPage(r, at)
		if !keep(rel, err) {
			continue
		}
		if err := write(rel, body); err != nil {
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
		r.Lang = sec.Lang
		if r.Line == 0 {
			r.Line = 1
		}
	}
	if s.AllowBrokenMath {
		r.OnRefusal = s.marked
	}
	r.MathML = s.MathML
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
	prev, next := s.around(sec)
	return s.render(layout{
		Title:   navName(sec) + " · " + bookName(sec.Lang, sec.Meta.Book, sec.Meta.BookTitle),
		Lang:    sec.Lang,
		Crumbs:  s.sectionCrumbs(sec),
		Langs:   langs,
		Side:    s.sideFor(sec.Lang, sec.Meta.Book),
		Here:    "library",
		Prev:    prev,
		Next:    next,
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
	lang := sec.Lang
	if sec.Meta.TranslatedFrom == "" {
		// The French volume carries no edition in the books manifest, so the
		// line says what is known and does not invent the rest.
		if sec.Meta.SourceEdition == "" {
			return T(lang, "p.printed")
		}
		return fmt.Sprintf(T(lang, "p.edition"), sec.Meta.SourceEdition)
	}
	note := T(lang, "p.mt") + fmt.Sprintf(T(lang, "p.by"), sec.Meta.TranslationModel)
	if sec.Meta.GlossaryVersion > 0 {
		note += fmt.Sprintf(T(lang, "p.gloss"), sec.Meta.GlossaryVersion)
	}
	return note + T(lang, "p.unchecked") + cutDown(lang, sec.Meta.TranslationModel)
}

// cutDown is the extra sentence a page written by a smaller model carries.
//
// The rule is quality.SmallModel, which is L08's rule, because two answers to
// "is this a cut down model" is one answer too many. The audit knows this and
// records it as a soft finding, and the reader of the page has more right to it
// than the audit does: nobody chose the model, the account was moved down
// between one section and the next, and the only place that fact reaches
// somebody reading the Vietnamese is here.
func cutDown(lang, model string) string {
	if !quality.SmallModel(model) {
		return ""
	}
	return T(lang, "p.cut")
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
	fmt.Fprintf(&b, "<p class=\"tagline\">%s</p>\n", fmt.Sprintf(T(en.Lang, "x.tagline"),
		"<code>"+st.Tag+"</code>", "<code>[Bourbaki, Tag "+st.Tag+"]</code>"))
	fmt.Fprintf(&b, "<p class=\"where\">%s, <a href=%q>%s</a></p>\n",
		template.HTMLEscapeString(printed(en, st)), s.SectionURL(en)+"#"+st.Label,
		template.HTMLEscapeString(heading(en)))

	body, err := r.HTML(u.Body)
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "<div class=\"stmt k-%s\">%s</div>\n", labelKind(st.Label), body)
	fmt.Fprintf(&b, "<p class=\"caveat\">%s</p>\n", T(en.Lang, "x.caveat"))

	s.list(&b, en.Lang, "x.citedby", s.CitedBy[st.Tag], s.from)
	s.list(&b, en.Lang, "x.cites", s.Cites[st.Tag], s.to)

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
		Lang: lang, Langs: langs, Note: provenance(en), Side: s.sideFor(lang, en.Meta.Book), Here: "tags",
		Crumbs: append(s.sectionCrumbs(en), crumb{short(en), s.SectionURL(en)}), Content: template.HTML(b.String())})
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
func (s *Site) list(b *strings.Builder, lang, key string, edges []*Edge, of func(*Edge) row) {
	title := T(lang, key)
	if len(edges) == 0 {
		fmt.Fprintf(b, "<h2>%s</h2>\n<p class=\"none\">%s</p>\n", title, T(lang, "x.nothing"))
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
	return s.render(layout{Title: "Tags", Here: "tags", Wide: true, Content: template.HTML(b.String())})
}

// gzipped is body compressed, with no name and no time in the header, so that
// the same page is the same bytes on every build and a sync uploads only what
// changed.
func gzipped(body string) (string, error) {
	var b bytes.Buffer
	z, err := gzip.NewWriterLevel(&b, gzip.BestCompression)
	if err != nil {
		return "", err
	}
	if _, err := z.Write([]byte(body)); err != nil {
		return "", err
	}
	if err := z.Close(); err != nil {
		return "", err
	}
	return b.String(), nil
}

// warm sets every formula of the corpus on every core before a page is
// written, so the pages, which are written one after another, find them set.
func (s *Site) warm() error {
	eng, err := mathEngine()
	if err != nil {
		return err
	}
	var spans []katex.Span
	add := func(body string) {
		found, _ := mathtex.Split(body)
		for _, sp := range found {
			spans = append(spans, katex.Span{TeX: sp.Text, Display: sp.Display, MathML: s.MathML})
		}
	}
	for _, sec := range s.Sections {
		add(sec.Body)
	}
	for _, ex := range s.Exercises {
		add(ex.Body)
		if ex.Solution != nil {
			add(ex.Solution.Body)
		}
	}
	return eng.Warm(spans, runtime.GOMAXPROCS(0))
}
