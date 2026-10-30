package publish

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// The frame every page is written into.
//
// One template, one stylesheet and one script, the last two embedded from
// assets/. The frame is three columns: the contents of the Book a page stands
// in, the page, and the page's own outline. The contents are one file a Book
// and language, fetched by the script, rather than a copy on every page. A page
// read without the script loses the two side columns and nothing else, since
// the crumbs and the previous and next links are plain HTML.
//
// It is the frame of tamnd/gtm-solver, which took its look from the Math 2.0
// wiki, so that the three sites read as one family.

//go:embed assets/style.css
var styleCSS string

//go:embed assets/site.js
var siteJS string

// The text is set in New Computer Modern, which is the design KaTeX sets the
// mathematics in, so a formula inside a sentence is in the same face as the
// sentence around it. It and Inter, the sans of the frame, carry every letter
// of Vietnamese and French. They ship with the site rather than being named
// and hoped for: a reader without them installed, which is nearly every
// reader, otherwise gets Georgia, whose stacked Vietnamese accents and whose
// italics both sit badly next to the mathematics.
//
//go:embed assets/fonts
var fontFiles embed.FS

type layout struct {
	Title string
	// Lang is what goes in the lang attribute of the document. It is the
	// language of the text on the page and not of the site, which matters here
	// more than on most sites: a screen reader that says a Vietnamese page in
	// English pronunciation reads it as noise.
	Lang string
	// UI is the language the frame is said in, which is Lang where the frame has
	// a table for it and English where it does not.
	UI      string
	Home    string
	Library string
	Search  string
	Fonts   string
	MathML  bool
	// SearchLang is the language the search box in the header searches, empty
	// on a page that has no language of its own.
	SearchLang string
	Tags       string
	Reports    string
	About      string
	CSS        string
	JS         string
	KaTeX      string
	// Canonical is set on a page that is a second copy of another one, which is
	// what an exercise's tag URL is. Empty everywhere else.
	Canonical string
	Crumbs    []crumb
	Content   template.HTML
	Langs     []langLink
	Note      string
	// Side is the Book a page stands in, which puts its contents in the left
	// column. Nil on a page that stands in no Book.
	Side *side
	// Prev and Next are the pages either side of this one in reading order,
	// which the [ and ] keys follow.
	Prev, Next *pageLink
	// Wide is a page that is a list rather than a text, which takes the width
	// the outline would have had.
	Wide bool
	// Here is which of the top bar's links this page is under.
	Here string
	// Desc is the description a search engine or a link preview shows.
	Desc string
	T    map[string]string
}

type side struct {
	Abbr  string
	Title string
	Sub   string
	URL   string
	Nav   string
}

type pageLink struct {
	Text string
	URL  string
}

type crumb struct {
	Text string
	URL  string
}

type langLink struct {
	Lang  string
	Name  string
	URL   string
	Here  bool
	Draft bool
}

func (s *Site) render(l layout) (string, error) {
	// A page in a language searches that language. A page with none of its
	// own, the front page or a report, leaves the choice to the search page.
	l.SearchLang = l.Lang
	if l.Lang == "" {
		l.Lang = "en"
	}
	if l.UI == "" {
		l.UI = uiLang(l.Lang)
	}
	l.T = uiText[l.UI]
	l.Home = s.url()
	l.Library = s.url(homeLang(l.Lang))
	l.Search = s.url("search")
	l.Tags = s.url("tags")
	l.Reports = s.url("reports")
	// Every page links the about page, because every page of this site shows
	// text a machine had a hand in and P07 is that such a page says so.
	l.About = s.url("about")
	l.CSS = s.url() + "style.css"
	l.JS = s.url() + "site.js"
	l.Fonts = s.url() + "fonts/"
	l.MathML = s.MathML
	// Every page has mathematics on it, so the KaTeX stylesheet is linked
	// unconditionally; the browser caches it once for the whole site.
	l.KaTeX = s.url() + "katex/katex.min.css"
	if l.Desc == "" {
		l.Desc = T(l.UI, "tagline")
	}
	var b bytes.Buffer
	if err := pageTmpl.Execute(&b, l); err != nil {
		return "", err
	}
	return b.String(), nil
}

// homeLang is the library a page's brand link goes to: en-mt readers are
// English readers.
func homeLang(lang string) string {
	if lang == "en-mt" {
		return "en"
	}
	return lang
}

// displayOrder is the order languages are offered in: the original first,
// then the printed translation, then the rest, and the machine English last.
func (s *Site) displayOrder() []string {
	rank := map[string]int{"fr": 0, "en": 1, "vi": 2, "en-mt": 9}
	out := append([]string(nil), s.Langs...)
	sort.SliceStable(out, func(i, j int) bool {
		ri, ok := rank[out[i]]
		if !ok {
			ri = 5
		}
		rj, ok := rank[out[j]]
		if !ok {
			rj = 5
		}
		return ri < rj
	})
	return out
}

// langLinks is the switcher for one page, given a way to say where each
// language holds the same thing. A language that does not hold the page is
// left off rather than linked at a blank.
func (s *Site) langLinks(here string, at func(lang string) string) []langLink {
	var out []langLink
	for _, lang := range s.displayOrder() {
		if lang != here && s.Draft[lang] && !s.Drafts {
			continue
		}
		url := at(lang)
		if url == "" {
			continue
		}
		out = append(out, langLink{Lang: lang, Name: native(lang), URL: url, Here: lang == here, Draft: s.Draft[lang]})
	}
	// One entry is the page saying what it already says in its lang attribute,
	// so there is nothing to switch to and the switcher is not written at all.
	if len(out) < 2 {
		return nil
	}
	return out
}

// ---------------------------------------------------------------- books ---

// shelf is one Book in one language: its pages in reading order.
type shelf struct {
	Lang  string
	Book  string
	Front []*Section // introductions and notes to the reader, before chapter I
	Chaps []*chap
	Back  []*Section // the indexes, after the last chapter
	Order []*Section // every page, in the order a reader goes through them
}

type chap struct {
	Name     string
	Title    string
	Sections []*Section
}

func (s *Site) shelves() map[string]*shelf {
	if s.shelf != nil {
		return s.shelf
	}
	s.shelf = map[string]*shelf{}
	chaps := map[string]*chap{}
	for _, sec := range s.Sections {
		k := sec.Lang + "/" + sec.Meta.Book
		sh := s.shelf[k]
		if sh == nil {
			sh = &shelf{Lang: sec.Lang, Book: sec.Meta.Book}
			s.shelf[k] = sh
		}
		if sec.Meta.Chapter == "" {
			switch sec.Meta.Kind {
			case "notation", "terminology":
				sh.Back = append(sh.Back, sec)
			default:
				sh.Front = append(sh.Front, sec)
			}
			continue
		}
		ck := k + "/" + sec.Meta.Chapter
		c := chaps[ck]
		if c == nil {
			c = &chap{Name: sec.Meta.Chapter}
			chaps[ck] = c
			sh.Chaps = append(sh.Chaps, c)
		}
		c.Sections = append(c.Sections, sec)
	}
	for _, sh := range s.shelf {
		sort.SliceStable(sh.Chaps, func(i, j int) bool {
			return romanValue(sh.Chaps[i].Name) < romanValue(sh.Chaps[j].Name)
		})
		order := func(list []*Section) {
			sort.SliceStable(list, func(i, j int) bool { return list[i].Path < list[j].Path })
		}
		order(sh.Front)
		order(sh.Back)
		sh.Order = append(sh.Order, sh.Front...)
		for _, c := range sh.Chaps {
			order(c.Sections)
			c.Title = s.chapterTitle(sh.Lang, c.Sections)
			sh.Order = append(sh.Order, c.Sections...)
		}
		sh.Order = append(sh.Order, sh.Back...)
	}
	return s.shelf
}

func (s *Site) shelfOf(lang, book string) *shelf { return s.shelves()[lang+"/"+book] }

// books is the Books a language holds, in Bourbaki's order.
func (s *Site) books(lang string) []*shelf {
	var out []*shelf
	for _, sh := range s.shelves() {
		if sh.Lang == lang {
			out = append(out, sh)
		}
	}
	sort.Slice(out, func(i, j int) bool { return bookRank(out[i].Book) < bookRank(out[j].Book) })
	return out
}

var h1RE = regexp.MustCompile(`(?m)^# +(.+?)\s*$`)
var h2RE = regexp.MustCompile(`(?m)^## +(.+?)\s*(\{[^}]*\})?\s*$`)
var sectionSignRE = regexp.MustCompile(`(?i)^(§|Appendix|Appendice|Annexe|Phụ lục)\s*[0-9IVX]*\s*[.:]?\s*`)

// numberRE is a number a heading opens with. A translation often drops the §
// and keeps the number, "## 1. BIỂU ĐỒ VÀ DÃY KHỚP", and the page then says
// "§ 1. 1. Biểu đồ". It is taken off only when it is the §'s own number, since
// a title can begin with a numeral that is part of it.
var numberRE = regexp.MustCompile(`^([0-9]+)\s*[.:]\s+`)

// bareTitle is a heading with the § sign and the §'s own number taken off.
func bareTitle(t string, n int) string {
	t = strings.TrimSpace(sectionSignRE.ReplaceAllString(t, ""))
	if m := numberRE.FindStringSubmatch(t); m != nil && m[1] == strconv.Itoa(n) {
		t = strings.TrimSpace(t[len(m[0]):])
	}
	return t
}

// chapterTitle is a chapter's name in the language of its pages. The front
// matter carries the title the source printed, which for a translation is the
// English, so a translated chapter is named from the heading its own front
// page opens with.
func (s *Site) chapterTitle(lang string, secs []*Section) string {
	if len(secs) == 0 {
		return ""
	}
	meta := secs[0].Meta.ChapterTitle
	if secs[0].Meta.TranslatedFrom != "" {
		for _, sec := range secs {
			if sec.Meta.Kind == "front" {
				if m := h1RE.FindStringSubmatch(sec.Body); m != nil {
					return sentenceIn(lang, stripMath(m[1]))
				}
			}
		}
	}
	return sentenceIn(lang, meta)
}

// stripMath leaves a heading's mathematics alone; it is here so a title with a
// formula in it is not cased inside the formula.
func stripMath(s string) string { return s }

// title is a §'s title in the language of its page, without its number.
func title(sec *Section) string {
	if sec.Meta.TranslatedFrom != "" {
		if m := h2RE.FindStringSubmatch(sec.Body); m != nil {
			t := bareTitle(m[1], sec.Meta.Section)
			if t != "" {
				return sentenceIn(sec.Lang, t)
			}
		}
		if m := h1RE.FindStringSubmatch(sec.Body); m != nil && sec.Meta.Kind != "" {
			return sentenceIn(sec.Lang, m[1])
		}
	}
	return sentenceIn(sec.Lang, sec.Meta.SectionTitle)
}

// navName is how a page is named in the contents column and in prev and next.
func navName(sec *Section) string {
	t := title(sec)
	switch {
	case sec.Meta.Kind != "":
		return t
	case sec.Meta.Appendix:
		return fmt.Sprintf("App. %d. %s", sec.Meta.Section, t)
	}
	return fmt.Sprintf("§ %d. %s", sec.Meta.Section, t)
}

// BookURL is the contents of one Book in one language.
func (s *Site) BookURL(lang, book string) string { return s.url(lang, book) }

func (s *Site) navURL(lang, book string) string { return s.url(lang, book) + "nav.html" }

func (s *Site) sideFor(lang, book string) *side {
	if s.shelfOf(lang, book) == nil {
		return nil
	}
	abbr := ""
	for _, b := range bookList {
		if b.ID == book {
			abbr = b.Abbr
		}
	}
	sub := T(lang, "book")
	if lang == "en-mt" {
		sub = T(lang, "mt")
	}
	return &side{Abbr: abbr, Title: bookName(lang, book, book), Sub: sub,
		URL: s.BookURL(lang, book), Nav: s.navURL(lang, book)}
}

// around is the pages either side of a § in its Book.
func (s *Site) around(sec *Section) (*pageLink, *pageLink) {
	sh := s.shelfOf(sec.Lang, sec.Meta.Book)
	if sh == nil {
		return nil, nil
	}
	for i, o := range sh.Order {
		if o != sec {
			continue
		}
		var prev, next *pageLink
		if i > 0 {
			prev = &pageLink{navName(sh.Order[i-1]), s.SectionURL(sh.Order[i-1])}
		}
		if i+1 < len(sh.Order) {
			next = &pageLink{navName(sh.Order[i+1]), s.SectionURL(sh.Order[i+1])}
		}
		return prev, next
	}
	return nil, nil
}

func esc(s string) string { return template.HTMLEscapeString(s) }

// nav is the contents fragment of one Book, the file the script loads into the
// left column.
func (s *Site) nav(sh *shelf) string {
	var b strings.Builder
	lang := sh.Lang
	if len(sh.Front) > 0 {
		fmt.Fprintf(&b, "<div class=\"part\">%s</div>\n<ol>\n", esc(T(lang, "front")))
		for _, sec := range sh.Front {
			fmt.Fprintf(&b, "<li><a href=%q>%s</a></li>\n", s.SectionURL(sec), esc(navName(sec)))
		}
		b.WriteString("</ol>\n")
	}
	for _, c := range sh.Chaps {
		fmt.Fprintf(&b, "<details><summary><a href=%q>%s</a></summary>\n<ol>\n",
			s.ChapterURL(lang, sh.Book, c.Name), esc(chapLabel(lang, c)))
		for _, sec := range c.Sections {
			fmt.Fprintf(&b, "<li><a href=%q>%s</a></li>\n", s.SectionURL(sec), esc(navName(sec)))
		}
		b.WriteString("</ol></details>\n")
	}
	if len(sh.Back) > 0 {
		fmt.Fprintf(&b, "<div class=\"part\">%s</div>\n<ol>\n", esc(T(lang, "back")))
		for _, sec := range sh.Back {
			fmt.Fprintf(&b, "<li><a href=%q>%s</a></li>\n", s.SectionURL(sec), esc(navName(sec)))
		}
		b.WriteString("</ol>\n")
	}
	return b.String()
}

func chapLabel(lang string, c *chap) string {
	if c.Title == "" {
		return chapterWord(lang, c.Name)
	}
	return c.Name + ". " + c.Title
}

// ---------------------------------------------------------------- pages ---

// front is the landing page: what this is, the languages, the twelve Books.
func (s *Site) front() (string, error) {
	var b strings.Builder
	b.WriteString(`<section class="hero">
<p class="eyebrow">Nicolas Bourbaki</p>
<h1 class="display">Éléments de mathématique</h1>
`)
	for _, l := range []string{"en", "fr", "vi"} {
		fmt.Fprintf(&b, "<p class=\"lede\" lang=%q>%s</p>\n", l, esc(T(l, "tagline")))
	}
	b.WriteString("</section>\n")

	nums := s.numbers()
	byLang := map[string]langNumbers{}
	for _, n := range nums {
		byLang[n.Lang] = n
	}
	b.WriteString("<ul class=\"langpick\">\n")
	for _, lang := range s.displayOrder() {
		n := byLang[lang]
		fmt.Fprintf(&b, "<li><a class=\"pick\" href=%q lang=%q><span class=\"code\">%s</span><span class=\"n\">%s</span>"+
			"<span class=\"s\">%d %s · %d %s</span><span class=\"go\">%s →</span></a></li>\n",
			s.url(lang), lang, esc(strings.ToUpper(lang)), esc(native(lang)),
			len(s.books(lang)), esc(T(lang, "booksN")), n.Sections, esc(T(lang, "sections")),
			esc(T(lang, "read")))
	}
	b.WriteString("</ul>\n")

	set, err := s.TagSet()
	if err != nil {
		return "", err
	}
	fmt.Fprintf(&b, "<div class=\"stats\"><div class=\"stat\"><b>%d</b><span>Books</span></div>"+
		"<div class=\"stat\"><b>%d</b><span>sections</span></div>"+
		"<div class=\"stat\"><b>%d</b><span>permanent tags</span></div>"+
		"<div class=\"stat\"><b>%d</b><span>exercises</span></div>"+
		"<div class=\"stat\"><b>%d</b><span>languages</span></div></div>\n",
		len(s.allBooks()), len(s.Sections), len(set.Tags), len(s.Exercises), len(s.Langs))

	b.WriteString("<h2 class=\"part-head\">Les Livres · The Books · Các Quyển</h2>\n<ul class=\"cards books\">\n")
	for _, id := range s.allBooks() {
		info := infoOf(id)
		fmt.Fprintf(&b, "<li><div class=\"card\"><span class=\"num\">%s</span><span class=\"t\" lang=\"fr\">%s</span>"+
			"<span class=\"a\">%s · %s</span><span class=\"s\">", esc(info.Abbr), esc(info.Names["fr"]),
			esc(info.Names["en"]), esc(info.Names["vi"]))
		for _, lang := range s.displayOrder() {
			if s.shelfOf(lang, id) != nil {
				fmt.Fprintf(&b, "<a class=\"lc\" href=%q>%s</a>", s.BookURL(lang, id), esc(lang))
			}
		}
		b.WriteString("</span></div></li>\n")
	}
	b.WriteString("</ul>\n")
	fmt.Fprintf(&b, "<p class=\"subtitle\">%s "+
		"<a href=%q>About</a> · <a href=%q>Tags</a> · <a href=%q>Reports</a></p>\n",
		esc(T("en", "personal")), s.url("about"), s.url("tags"), s.url("reports"))
	langs := s.langLinks("", func(l string) string { return s.url(l) })
	return s.render(layout{Title: "Bourbaki · Éléments de mathématique", Lang: "fr", UI: "en", Wide: true,
		Here: "home", Langs: langs, Content: template.HTML(b.String())})
}

func infoOf(id string) bookInfo {
	for _, b := range bookList {
		if b.ID == id {
			return b
		}
	}
	return bookInfo{ID: id, Abbr: strings.ToUpper(id), Names: map[string]string{"en": id, "fr": id, "vi": id}}
}

// allBooks is every Book any language holds, in Bourbaki's order.
func (s *Site) allBooks() []string {
	seen := map[string]bool{}
	var out []string
	for _, sh := range s.shelves() {
		if !seen[sh.Book] {
			seen[sh.Book] = true
			out = append(out, sh.Book)
		}
	}
	sort.Slice(out, func(i, j int) bool { return bookRank(out[i]) < bookRank(out[j]) })
	return out
}

// library is /<lang>/: the Books that language holds.
func (s *Site) library(lang string) (string, error) {
	var b strings.Builder
	fmt.Fprintf(&b, "<header class=\"doc-head\"><p class=\"eyebrow\">%s · %s</p><h1>%s</h1>"+
		"<p class=\"lede\">%s</p></header>\n", esc(T(lang, "library")), esc(native(lang)),
		esc(T(lang, "books")), esc(T(lang, "tagline")))
	if lang == "en-mt" {
		fmt.Fprintf(&b, "<p class=\"tombstone\">These Books and chapters were only ever printed in French. The English "+
			"here is %s with machine help and has not yet been reviewed by a person.</p>\n", esc(T(lang, "mt")))
	}
	fmt.Fprintf(&b, "<p class=\"chips\"><a class=\"chip\" href=%q><b>%s →</b> %s</a></p>\n",
		s.url(lang, "statements"), esc(T(lang, "ix.title")), esc(T(lang, "ix.most")))
	b.WriteString("<ul class=\"cards books\">\n")
	held := map[string]bool{}
	for _, sh := range s.books(lang) {
		held[sh.Book] = true
		st, ex := 0, 0
		for _, sec := range sh.Order {
			st += sec.Meta.Statements
			ex += sec.Meta.Exercises
		}
		info := infoOf(sh.Book)
		fmt.Fprintf(&b, "<li><a class=\"card\" href=%q><span class=\"num\">%s</span><span class=\"t\">%s</span>"+
			"<span class=\"a\">%s</span><span class=\"s\"><span>%d %s</span><span>%d %s</span><span>%d %s</span></span></a></li>\n",
			s.BookURL(lang, sh.Book), esc(info.Abbr), esc(bookName(lang, sh.Book, sh.Book)), esc(chapterSpan(lang, sh)),
			len(sh.Order), esc(T(lang, "sections")), st, esc(T(lang, "statements")), ex, esc(T(lang, "exercisesN")))
	}
	b.WriteString("</ul>\n")
	// The Books this language does not hold, and where they can be read.
	var missing []string
	for _, id := range s.allBooks() {
		if !held[id] {
			missing = append(missing, id)
		}
	}
	if len(missing) > 0 && lang != "en-mt" {
		fmt.Fprintf(&b, "<h2 class=\"part-head\">%s</h2>\n<ul class=\"cards books\">\n", esc(T(lang, "notheld")))
		for _, id := range missing {
			info := infoOf(id)
			fmt.Fprintf(&b, "<li><div class=\"card off\"><span class=\"num\">%s</span><span class=\"t\">%s</span><span class=\"s\">%s ",
				esc(info.Abbr), esc(bookName(lang, id, id)), esc(T(lang, "alsoin")))
			for _, l := range s.displayOrder() {
				if s.shelfOf(l, id) != nil {
					fmt.Fprintf(&b, "<a class=\"lc\" href=%q>%s</a>", s.BookURL(l, id), esc(l))
				}
			}
			b.WriteString("</span></div></li>\n")
		}
		b.WriteString("</ul>\n")
	}
	langs := s.langLinks(lang, func(l string) string { return s.url(l) })
	return s.render(layout{Title: T(lang, "books") + " · Bourbaki", Lang: lang, Wide: true, Here: "library",
		Langs: langs, Content: template.HTML(b.String())})
}

func chapterSpan(lang string, sh *shelf) string {
	if len(sh.Chaps) == 0 {
		return ""
	}
	first, last := sh.Chaps[0].Name, sh.Chaps[len(sh.Chaps)-1].Name
	word := T(lang, "chapters")
	if first == last {
		return chapterWord(lang, first)
	}
	return fmt.Sprintf("%s %s–%s", upperFirst(word), first, last)
}

// bookPage is /<lang>/<book>/: the contents of one Book.
func (s *Site) bookPage(sh *shelf) (string, error) {
	lang := sh.Lang
	var b strings.Builder
	info := infoOf(sh.Book)
	fmt.Fprintf(&b, "<header class=\"doc-head\"><p class=\"eyebrow\">%s %s · %s</p><h1>%s</h1>",
		esc(T(lang, "book")), esc(info.Abbr), esc(native(lang)), esc(bookName(lang, sh.Book, sh.Book)))
	if lang != "fr" {
		fmt.Fprintf(&b, "<p class=\"lede\" lang=\"fr\">%s</p>", esc(info.Names["fr"]))
	}
	b.WriteString("</header>\n")
	b.WriteString("<p class=\"chips\">")
	if first := firstOf(sh); first != nil {
		fmt.Fprintf(&b, "<a class=\"chip\" href=%q><b>%s →</b> %s</a> ", s.SectionURL(first),
			esc(T(lang, "read")), esc(navName(first)))
	}
	fmt.Fprintf(&b, "<a class=\"chip\" href=%q><b>%s</b></a>", s.StatementsURL(lang, sh.Book), esc(T(lang, "ix.title")))
	b.WriteString("</p>\n")
	list := func(secs []*Section) {
		b.WriteString("<ol>\n")
		for _, sec := range secs {
			fmt.Fprintf(&b, "<li><a href=%q>%s</a>", s.SectionURL(sec), esc(navName(sec)))
			if sec.Meta.Statements > 0 {
				fmt.Fprintf(&b, "<span class=\"x\">%d</span>", sec.Meta.Statements)
			}
			b.WriteString("</li>\n")
		}
		b.WriteString("</ol>\n")
	}
	b.WriteString("<ol class=\"contents\">\n")
	if len(sh.Front) > 0 {
		fmt.Fprintf(&b, "<li><div class=\"ch\"><span class=\"n\"></span>%s</div>\n", esc(T(lang, "front")))
		list(sh.Front)
		b.WriteString("</li>\n")
	}
	for _, c := range sh.Chaps {
		fmt.Fprintf(&b, "<li><div class=\"ch\"><span class=\"n\">%s</span><a href=%q>%s</a></div>\n",
			esc(c.Name), s.ChapterURL(lang, sh.Book, c.Name), esc(orWord(lang, c)))
		list(c.Sections)
		b.WriteString("</li>\n")
	}
	if len(sh.Back) > 0 {
		fmt.Fprintf(&b, "<li><div class=\"ch\"><span class=\"n\"></span>%s</div>\n", esc(T(lang, "back")))
		list(sh.Back)
		b.WriteString("</li>\n")
	}
	b.WriteString("</ol>\n")
	langs := s.langLinks(lang, func(l string) string {
		if s.shelfOf(l, sh.Book) == nil {
			return ""
		}
		return s.BookURL(l, sh.Book)
	})
	return s.render(layout{Title: bookName(lang, sh.Book, sh.Book) + " · Bourbaki", Lang: lang, Here: "library",
		Side: s.sideFor(lang, sh.Book), Wide: true, Langs: langs,
		Crumbs:  []crumb{{T(lang, "library"), s.url(homeLang(lang))}},
		Content: template.HTML(b.String())})
}

func orWord(lang string, c *chap) string {
	if c.Title == "" {
		return chapterWord(lang, c.Name)
	}
	return c.Title
}

func firstOf(sh *shelf) *Section {
	if len(sh.Order) == 0 {
		return nil
	}
	// A reader starts at chapter I, not at the note to the reader.
	if len(sh.Chaps) > 0 && len(sh.Chaps[0].Sections) > 0 {
		return sh.Chaps[0].Sections[0]
	}
	return sh.Order[0]
}

// chapter is /<lang>/<book>/<chapter>/: the §§ of one chapter.
func (s *Site) chapter(k chapterKey) (string, error) {
	sh := s.shelfOf(k.Lang, k.Book)
	var c *chap
	if sh != nil {
		for _, x := range sh.Chaps {
			if x.Name == k.Chapter {
				c = x
			}
		}
	}
	if c == nil {
		return "", fmt.Errorf("no chapter %s of %s in %s", k.Chapter, k.Book, k.Lang)
	}
	lang := k.Lang
	var b strings.Builder
	fmt.Fprintf(&b, "<header class=\"doc-head\"><p class=\"eyebrow\"><a href=%q>%s</a> · %s</p><h1>%s</h1></header>\n",
		s.BookURL(lang, k.Book), esc(bookName(lang, k.Book, k.BookTitle)), esc(chapterWord(lang, k.Chapter)),
		esc(orWord(lang, c)))
	b.WriteString(s.toc(lang, c.Sections, nil))
	langs := s.langLinks(lang, func(l string) string {
		if !s.hasChapter(l, k.Book, k.Chapter) {
			return ""
		}
		return s.ChapterURL(l, k.Book, k.Chapter)
	})
	var prev, next *pageLink
	if len(c.Sections) > 0 {
		next = &pageLink{navName(c.Sections[0]), s.SectionURL(c.Sections[0])}
	}
	return s.render(layout{Title: chapterWord(lang, k.Chapter) + " · " + bookName(lang, k.Book, k.BookTitle),
		Lang: lang, Langs: langs, Side: s.sideFor(lang, k.Book), Here: "library", Prev: prev, Next: next,
		Crumbs: []crumb{{T(lang, "library"), s.url(homeLang(lang))},
			{bookName(lang, k.Book, k.BookTitle), s.BookURL(lang, k.Book)}},
		Content: template.HTML(b.String())})
}

// toc lists the sections of a chapter with their counts, leaving out skip.
func (s *Site) toc(lang string, secs []*Section, skip *Section) string {
	var b strings.Builder
	b.WriteString("<ul class=\"toc\">\n")
	for _, sec := range secs {
		if sec == skip {
			continue
		}
		fmt.Fprintf(&b, "<li><a href=%q>%s</a> <span class=\"count\">%d %s",
			s.SectionURL(sec), esc(navName(sec)), sec.Meta.Statements, esc(T(lang, "statements")))
		if s.hasExercises(sec.Lang, sec.Meta.Book, sec.Meta.Chapter, sec.Slug) {
			fmt.Fprintf(&b, " · <a href=%q>%d %s</a>",
				s.ExercisesURL(sec.Lang, sec.Meta.Book, sec.Meta.Chapter, sec.Slug), sec.Meta.Exercises, esc(T(lang, "exercisesN")))
		} else if sec.Meta.Exercises > 0 {
			fmt.Fprintf(&b, " · %d %s", sec.Meta.Exercises, esc(T(lang, "exercisesN")))
		}
		b.WriteString("</span></li>\n")
	}
	b.WriteString("</ul>\n")
	return b.String()
}

// chapterOf is the chapter a section sits in, as the navigation holds it.
func (s *Site) chapterOf(sec *Section) *chap {
	if sh := s.shelfOf(sec.Lang, sec.Meta.Book); sh != nil {
		for _, c := range sh.Chaps {
			for _, x := range c.Sections {
				if x == sec {
					return c
				}
			}
		}
	}
	return nil
}

// sectionCrumbs is the trail above a § or an exercise.
func (s *Site) sectionCrumbs(sec *Section) []crumb {
	lang := sec.Lang
	out := []crumb{{T(lang, "library"), s.url(homeLang(lang))},
		{bookName(lang, sec.Meta.Book, sec.Meta.BookTitle), s.BookURL(lang, sec.Meta.Book)}}
	if sec.Meta.Chapter != "" {
		out = append(out, crumb{chapterWord(lang, sec.Meta.Chapter), s.ChapterURL(lang, sec.Meta.Book, sec.Meta.Chapter)})
	}
	return out
}

// slugFromPath is the slug a page with no § number takes from its file name,
// so that the three introductions of Algebra are three pages and not one.
func slugFromPath(p string) string {
	base := strings.TrimSuffix(filepath.Base(p), ".md")
	base = strings.TrimLeft(base, "0123456789_")
	return strings.ReplaceAll(base, "_", "-")
}

var pageTmpl = template.Must(template.New("page").Parse(`<!DOCTYPE html>
<html lang="{{.Lang}}">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{.Title}}</title>
<meta name="description" content="{{.Desc}}">
<meta property="og:title" content="{{.Title}}">
<meta property="og:description" content="{{.Desc}}">
<meta property="og:site_name" content="Bourbaki · Éléments de mathématique">
<meta name="theme-color" content="#f6f7f9">
<link rel="icon" href="data:image/svg+xml,%3Csvg xmlns='http://www.w3.org/2000/svg' viewBox='0 0 32 32'%3E%3Crect width='32' height='32' fill='%23f2c94c'/%3E%3Ctext x='16' y='23' font-family='Georgia,serif' font-size='20' text-anchor='middle' fill='%2316181d'%3E%C3%89%3C/text%3E%3C/svg%3E">
{{if .Canonical}}<link rel="canonical" href="{{.Canonical}}">
{{end}}{{range .Langs}}{{if not .Here}}<link rel="alternate" hreflang="{{.Lang}}" href="{{.URL}}">
{{end}}{{end}}<script>try{var t=localStorage.getItem("theme");if(t)document.documentElement.dataset.theme=t}catch(e){}</script>
<link rel="preload" href="{{.Fonts}}newcm-book.woff2" as="font" type="font/woff2" crossorigin>
{{if not .MathML}}<link rel="stylesheet" href="{{.KaTeX}}">{{end}}
<link rel="stylesheet" href="{{.CSS}}">
<script defer src="{{.JS}}"></script>
</head>
<body{{if .MathML}} class="mathml"{{end}} data-ui="{{.UI}}" data-outline="{{index .T "onpage"}}" data-top="{{index .T "top"}}" data-copy="{{index .T "copy"}}" data-copied="{{index .T "copied"}}">
<a class="skip" href="#main">{{index .T "skip"}}</a>
<header class="topbar">
{{if .Side}}<button class="icon-btn menu" type="button" data-toggle="side" aria-label="{{index .T "contents"}}">&#9776;</button>
{{end}}<a class="brand" href="{{.Home}}"><span class="logo">É</span><span class="brand-text">Bourbaki</span></a>
<form class="find" action="{{.Search}}" method="get" role="search"><input name="q" type="search" placeholder="{{index .T "find"}}" aria-label="{{index .T "search"}}" autocomplete="off">{{if .SearchLang}}<input type="hidden" name="lang" value="{{.SearchLang}}">{{end}}<kbd>/</kbd></form>
<nav class="primary"><a href="{{.Library}}"{{if eq .Here "library"}} aria-current="page"{{end}}>{{index .T "library"}}</a><a href="{{.Tags}}"{{if eq .Here "tags"}} aria-current="page"{{end}}>{{index .T "tags"}}</a><a href="{{.Reports}}"{{if eq .Here "reports"}} aria-current="page"{{end}}>{{index .T "reports"}}</a><a href="{{.About}}"{{if eq .Here "about"}} aria-current="page"{{end}}>{{index .T "about"}}</a></nav>
{{if .Langs}}<nav class="langs" aria-label="{{index .T "lang"}}">{{range .Langs}}{{if .Here}}<span class="here" title="{{.Name}}">{{.Lang}}</span>{{else}}<a{{if .Draft}} class="draft"{{end}} href="{{.URL}}" hreflang="{{.Lang}}" title="{{.Name}}">{{.Lang}}</a>{{end}}{{end}}</nav>
{{end}}<button class="icon-btn" type="button" data-theme-toggle aria-label="{{index .T "theme"}}">&#9680;</button>
</header>
<div class="shell{{if .Side}} has-side{{end}}{{if .Wide}} wide{{end}}">
{{with .Side}}<aside class="side" id="side" aria-label="{{index $.T "contents"}}">
<a class="side-vol" href="{{.URL}}"><span class="num">{{.Abbr}}</span><span class="title">{{.Title}}</span>{{if .Sub}}<span class="by">{{.Sub}}</span>{{end}}</a>
<div class="side-links"><a href="{{.URL}}">{{index $.T "contents"}}</a></div>
<nav class="side-nav" data-src="{{.Nav}}"></nav>
</aside>
{{end}}<main id="main">
{{if .Crumbs}}<nav class="crumbs" aria-label="Breadcrumb">{{range $i, $c := .Crumbs}}{{if $i}}<span class="sep">/</span>{{end}}<a href="{{$c.URL}}">{{$c.Text}}</a>{{end}}</nav>
{{end}}<article class="doc">
{{.Content}}</article>
{{if or .Prev .Next}}<nav class="pager" aria-label="{{index .T "prev"}} / {{index .T "next"}}">{{with .Prev}}<a class="prev" href="{{.URL}}" rel="prev"><span class="l">&larr; {{index $.T "prev"}}</span><span class="t">{{.Text}}</span></a>{{end}}{{with .Next}}<a class="next" href="{{.URL}}" rel="next"><span class="l">{{index $.T "next"}} &rarr;</span><span class="t">{{.Text}}</span></a>{{end}}</nav>
{{end}}{{if .Note}}<footer class="note" data-label="{{index .T "provenance"}}">{{.Note}}</footer>
{{end}}</main>
{{if not .Wide}}<aside class="outline" aria-label="{{index .T "onpage"}}"></aside>
{{end}}</div>
<footer class="site-foot"><span>Bourbaki · Éléments de mathématique</span><span>{{index .T "personal"}}</span><a href="{{.About}}">{{index .T "about"}}</a></footer>
<div class="peek" hidden></div>
</body>
</html>
`))
