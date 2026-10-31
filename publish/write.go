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
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/katex"
	"github.com/tamnd/bourbaki-solver/mathtex"
	tagset "github.com/tamnd/bourbaki-solver/tags"
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
			ix, err := s.statementsPage(sh)
			if err != nil {
				return nil, err
			}
			if err := write(path.Join(lang, sh.Book, "statements", "index.html"), ix); err != nil {
				return nil, err
			}
		}
		cited, err := s.citedPage(lang)
		if err != nil {
			return nil, err
		}
		if err := write(path.Join(lang, "statements", "index.html"), cited); err != nil {
			return nil, err
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
		// The Markdown the page was set from, beside it, which is what the copy
		// button on a statement card reads its statement out of.
		if err := write(strings.TrimSuffix(rel, "index.html")+"index.md", sec.Body); err != nil {
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
	entries, _, err := s.Tags()
	if err != nil {
		return nil, err
	}
	tbs, secOf, chapName := s.tagBooks(entries)
	for _, tb := range tbs {
		rel := path.Join("tags", tb.ID, "index.html")
		body, err := s.tagBookPage(tb, secOf, chapName)
		if !keep(rel, err) {
			continue
		}
		if err := write(rel, body); err != nil {
			return nil, err
		}
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
		template.HTMLEscapeString("this formula could not be typeset: "+ref.Why),
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
	r := s.renderer(sec)
	r.Section = sectionLabel(sec)
	r.Xref = func(label string) string { return s.xref(sec, label) }
	body, err := r.HTML(s.linked(sec))
	if err != nil {
		return "", err
	}
	// A page with nothing but its heading, the opening page of a chapter in
	// most Books, would be a dead end; it lists what the chapter holds instead.
	if headingsOnly(body) {
		if c := s.chapterOf(sec); c != nil && len(c.Sections) > 1 {
			body += fmt.Sprintf("<h2>%s</h2>\n", esc(T(sec.Lang, "contents"))) + s.toc(sec.Lang, c.Sections, sec)
		}
	}
	if sec.Meta.TranslationQuality == corpus.QualityLow {
		body = draftBanner(sec.Lang) + body
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

var (
	headingRe = regexp.MustCompile(`(?s)<h[1-6][^>]*>.*?</h[1-6]>`)
	tagRe     = regexp.MustCompile(`<[^>]*>`)
)

// headingsOnly says whether rendered HTML holds no text outside its headings.
func headingsOnly(body string) bool {
	return strings.TrimSpace(tagRe.ReplaceAllString(headingRe.ReplaceAllString(body, ""), "")) == ""
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

// draftBanner is the warning at the top of a page whose text is a Google
// Translate draft. The footer says it too, but a reader who stops at the
// first wrong sentence never gets to the footer.
func draftBanner(lang string) string {
	return fmt.Sprintf("<p class=\"draft\" role=\"note\">%s</p>\n", esc(T(lang, "p.gt")))
}

// provenance is the line every page carries saying where its text comes from.
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
	if sec.Meta.TranslationQuality == corpus.QualityLow {
		return T(lang, "p.gt")
	}
	return T(lang, "p.mt") + T(lang, "p.unchecked")
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
		return template.HTMLEscapeString("not yet on this site: " + e.Book + " " + e.Chapter)
	}
	return missing(label)
}

// missing says why a label has no page of its own. An exercise gets its own
// wording, because an exercise reaching here is one the extraction did not pick
// up rather than a whole volume nobody has transcribed, and the two are fixed by
// different work.
func missing(label string) string {
	if ref, err := corpus.ParseLabel(label); err == nil && ref.Kind == corpus.KindExercise {
		return "an exercise not yet on this site"
	}
	return "not yet on this site"
}

// tagKey is one § of one Book, whichever printing it is read in, which is what
// the tags of a Book are grouped under.
type tagKey struct {
	Book, Chapter string
	App           bool
	N             int
}

// tagRow is one tag on the page of its Book.
type tagRow struct {
	Tag, Label, Kind, Name string
	Key                    tagKey
	Out                    bool
}

// tagBook is a Book's tags, by § in the order the Book prints them.
type tagBook struct {
	ID         string
	Keys       []tagKey
	Rows       map[tagKey][]tagRow
	Statements int
	Exercises  int
	Missing    int
}

// tagBooks puts every tag under the Book and the § it names. The order of the
// §§ is the French printing's, which is the whole of the Éléments, with what
// only another language holds after it; a chapter is named the English way
// where the English has it.
func (s *Site) tagBooks(entries []tagset.Entry) ([]*tagBook, map[tagKey]*Section, map[string]string) {
	order := map[tagKey]int{}
	secOf := map[tagKey]*Section{}
	chapName := map[string]string{}
	langs := append([]string{"fr", "en"}, s.Langs...)
	for _, lang := range langs {
		for _, sh := range s.books(lang) {
			for _, c := range sh.Chaps {
				if _, ok := chapName[sh.Book+"/"+c.Name]; !ok || lang == "en" {
					chapName[sh.Book+"/"+c.Name] = chapLabel(lang, c)
				}
				for _, sec := range c.Sections {
					k := tagKey{sh.Book, sec.Meta.Chapter, sec.Meta.Appendix, sec.Meta.Section}
					if _, ok := order[k]; !ok {
						order[k] = len(order)
					}
					if secOf[k] == nil || lang == "en" {
						secOf[k] = sec
					}
				}
			}
		}
	}
	books := map[string]*tagBook{}
	for _, e := range entries {
		r := tagRow{Tag: string(e.Tag), Label: e.Label}
		switch st, ex := s.Tag(r.Tag), s.ExerciseTag(r.Tag); {
		case st != nil:
			r.Kind = labelKind(st.Label)
			for _, l := range langs {
				if sec := st.Sections[l]; sec != nil {
					r.Name = st.Units[l].Heading.Text
					r.Key = tagKey{sec.Meta.Book, sec.Meta.Chapter, sec.Meta.Appendix, sec.Meta.Section}
					break
				}
			}
		case ex != nil:
			r.Kind = "exercise"
			r.Name = ex.Name()
			r.Key = tagKey{ex.Meta.Book, ex.Meta.Chapter, ex.Meta.Appendix, ex.Meta.Section}
		default:
			r.Kind, r.Name, r.Out = labelKind(e.Label), e.Label, true
			if ref, err := corpus.ParseLabel(e.Label); err == nil && ref.Kind == corpus.KindExercise {
				r.Kind = "exercise"
			}
			r.Key.Book, _, _ = strings.Cut(e.Label, "-")
		}
		tb := books[r.Key.Book]
		if tb == nil {
			tb = &tagBook{ID: r.Key.Book, Rows: map[tagKey][]tagRow{}}
			books[r.Key.Book] = tb
		}
		if _, ok := tb.Rows[r.Key]; !ok {
			tb.Keys = append(tb.Keys, r.Key)
		}
		tb.Rows[r.Key] = append(tb.Rows[r.Key], r)
		switch {
		case r.Out:
			tb.Missing++
		case r.Kind == "exercise":
			tb.Exercises++
		default:
			tb.Statements++
		}
	}
	var out []*tagBook
	add := func(tb *tagBook) {
		sort.SliceStable(tb.Keys, func(i, j int) bool {
			oi, iok := order[tb.Keys[i]]
			oj, jok := order[tb.Keys[j]]
			if iok != jok {
				return iok
			}
			return oi < oj
		})
		out = append(out, tb)
	}
	for _, b := range bookList {
		if tb := books[b.ID]; tb != nil {
			add(tb)
			delete(books, b.ID)
		}
	}
	var rest []string
	for id := range books {
		rest = append(rest, id)
	}
	sort.Strings(rest)
	for _, id := range rest {
		add(books[id])
	}
	return out, secOf, chapName
}

// TagsURL is the tags of one Book, or the list of Books with no argument.
func (s *Site) TagsURL(book ...string) string { return s.url(append([]string{"tags"}, book...)...) }

// tagLookup is the box that takes a reader from a tag they were given to its
// page, which is the one thing a tag is for.
func (s *Site) tagLookup(b *strings.Builder) {
	fmt.Fprintf(b, "<form class=\"tag-go\" data-taggo action=%q><input name=\"t\" placeholder=\"0AZ1\" maxlength=\"4\" "+
		"autocomplete=\"off\" spellcheck=\"false\" aria-label=\"Tag\" pattern=\"[0-9A-Za-z]{4}\" required>"+
		"<button type=\"submit\">Go to tag</button></form>\n", s.url("tag"))
}

// tagTable is /tags/: what a tag is, the box that looks one up, and every Book
// with how many of its statements and exercises have one.
func (s *Site) tagTable() (string, error) {
	entries, retired, err := s.Tags()
	if err != nil {
		return "", err
	}
	books, _, _ := s.tagBooks(entries)
	var b strings.Builder
	fmt.Fprintf(&b, "<header class=\"doc-head\"><p class=\"eyebrow\">Bourbaki</p><h1>Tags</h1><p class=\"lede\">%d tags, %d retired. "+
		"Every definition, theorem, proposition, lemma, corollary, remark, example and exercise has a tag of four characters. "+
		"A tag is assigned once and never reused, so a citation of one, <code>[Bourbaki, Tag 0AZ1]</code>, stays good across every renumbering of the text.</p></header>\n",
		len(entries), len(retired))
	s.tagLookup(&b)
	b.WriteString("<table class=\"ix-books tag-books\"><thead><tr><th>Book</th><th>Statements</th><th>Exercises</th><th></th></tr></thead><tbody>\n")
	for _, tb := range books {
		fmt.Fprintf(&b, "<tr><th><a href=%q>%s</a></th><td>%d</td><td>%d</td><td>", s.TagsURL(tb.ID), esc(bookName("en", tb.ID, tb.ID)), tb.Statements, tb.Exercises)
		for _, l := range []string{"en", "fr"} {
			if s.shelfOf(l, tb.ID) != nil {
				fmt.Fprintf(&b, "<a href=%q>%s</a> ", s.StatementsURL(l, tb.ID), esc(T(l, "ix.title")))
				break
			}
		}
		b.WriteString("</td></tr>\n")
	}
	b.WriteString("</tbody></table>\n")
	return s.render(layout{Title: "Tags", Here: "tags", Wide: true, Content: template.HTML(b.String())})
}

// tagBookPage is /tags/<book>/: every tag of one Book under its chapter and §,
// with the chips and the filter of the statement index.
func (s *Site) tagBookPage(tb *tagBook, secOf map[tagKey]*Section, chapName map[string]string) (string, error) {
	var b strings.Builder
	name := bookName("en", tb.ID, tb.ID)
	fmt.Fprintf(&b, "<header class=\"doc-head\"><p class=\"eyebrow\"><a href=%q>Tags</a></p><h1>%s</h1>", s.TagsURL(), esc(name))
	fmt.Fprintf(&b, "<p class=\"lede\">The tags of %d statements and %d exercises, by chapter and §. Each tag is the permanent address of what it names.</p></header>\n",
		tb.Statements, tb.Exercises)
	s.tagLookup(&b)
	count := map[string]int{}
	total := 0
	for _, rs := range tb.Rows {
		for _, r := range rs {
			count[r.Kind]++
			total++
		}
	}
	b.WriteString("<div class=\"ix-tools\" data-ix>\n")
	fmt.Fprintf(&b, "<button type=\"button\" class=\"chip on\" data-k=\"\">All <span class=\"x\">%d</span></button>\n", total)
	for _, k := range append(kinds, "exercise") {
		if count[k] == 0 {
			continue
		}
		n := kindName("en", k)
		if k == "exercise" {
			n = "Exercises"
		}
		fmt.Fprintf(&b, "<button type=\"button\" class=\"chip k-%s\" data-k=\"%s\"><span class=\"dot\"></span>%s <span class=\"x\">%d</span></button>\n",
			k, k, esc(n), count[k])
	}
	b.WriteString("<input type=\"search\" class=\"ix-q\" placeholder=\"Filter by tag, name or label\" aria-label=\"Filter\">\n</div>\n<div class=\"ix\">\n")
	open := ""
	for _, k := range tb.Keys {
		if k.Chapter != open {
			if open != "" {
				b.WriteString("</section>\n")
			}
			open = k.Chapter
			title := chapName[tb.ID+"/"+k.Chapter]
			if title == "" {
				title = "Chapter " + k.Chapter
			}
			if k.Chapter == "" {
				title = "Not yet on this site"
			}
			fmt.Fprintf(&b, "<section class=\"ix-ch\"><h2 id=\"ch-%s\">%s</h2>\n", esc(k.Chapter), esc(title))
		}
		b.WriteString("<div class=\"ix-sec\">")
		if sec := secOf[k]; sec != nil {
			fmt.Fprintf(&b, "<h3><a href=%q>%s</a></h3>", s.SectionURL(sec), esc(navName(sec)))
		}
		b.WriteString("\n<ul class=\"ix-list\">\n")
		for _, r := range tb.Rows[k] {
			if r.Out {
				fmt.Fprintf(&b, "<li class=\"k-%s\" data-k=\"%s\"><span class=\"dot\"></span><b>%s</b> <span class=\"where\">%s</span> <span class=\"tag\">%s</span></li>\n",
					r.Kind, r.Kind, esc(r.Name), esc(missing(r.Label)), r.Tag)
				continue
			}
			fmt.Fprintf(&b, "<li class=\"k-%s\" data-k=\"%s\"><a href=%q><span class=\"dot\"></span><b>%s</b></a> <span class=\"where\">%s</span> <a class=\"tag\" href=%q>%s</a></li>\n",
				r.Kind, r.Kind, s.TagURL(r.Tag), esc(r.Name), esc(r.Label), s.TagURL(r.Tag), r.Tag)
		}
		b.WriteString("</ul></div>\n")
	}
	if open != "" {
		b.WriteString("</section>\n")
	}
	b.WriteString("</div>\n")
	return s.render(layout{Title: "Tags · " + name, Here: "tags", Wide: true,
		Crumbs:  []crumb{{"Tags", s.TagsURL()}},
		Content: template.HTML(b.String())})
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
