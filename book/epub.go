package book

import (
	"archive/zip"
	"encoding/xml"
	"fmt"
	"html"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/tamnd/bourbaki-solver/katex"
	"github.com/tamnd/bourbaki-solver/mathtex"
)

// The EPUB is the second half of the same build, and it is not a convenience.
//
// The PDF answers whether the volume paginates: whether the chapters follow one
// another, whether a § assembled out of forty pages is the § the printing has,
// whether the whole comes to about the length of the book it was taken from. It
// cannot answer whether the text is readable on anything that is not a page of
// that exact size, and it cannot be read on a phone by somebody on a bus, which
// is where a Vietnamese reader of a book that has never been printed in
// Vietnamese is actually going to read it.
//
// So the same volume goes out twice, from the same load and the same masking of
// the same mathematics, into two formats that fail in different ways. A file
// that breaks the typesetter and a file that breaks a reading system are rarely
// the same file, and running both over every volume is how a fault gets found
// twice rather than shipped once.

// An EPUB is one written book and what the writing found.
type EPUB struct {
	Path string
	// Documents is the number of XHTML files in the book, which is one per
	// division: the introduction, each chapter opening, each § and appendix,
	// each chapter's exercises and each historical note.
	Documents int
	Bytes     int64
	// Refused is every math span KaTeX would not read. The PDF build does not
	// have this list, because TeX sets what the corpus wrote whether or not it
	// parses as anything, so an EPUB build is the only place a formula that is
	// not a formula is caught by a machine rather than by a reader.
	Refused []Finding
	// Math is the number of spans set as MathML, which is the number that says
	// whether the reading system has any work to do at all.
	Math int
}

// WriteEPUB writes one volume as an EPUB 3 file.
func WriteEPUB(file string, v *Volume, opt Options) (*EPUB, error) {
	eng, err := katex.New()
	if err != nil {
		return nil, err
	}
	e := &EPUB{Path: file}
	docs := layout(v)

	// Where every anchor of the volume lives, so that a cross reference becomes
	// a link a reader can follow rather than the bare words it was written as.
	// It has to be built before anything is rendered, for the reason the LaTeX
	// writer gives: a one pass writer cannot tell a reference that will resolve
	// from one that will not, and a book that quietly drops a cross reference
	// reads fine and is wrong.
	where := map[string]string{}
	for _, d := range docs {
		for _, a := range d.anchors() {
			where[a] = d.Name
		}
	}

	for i := range docs {
		if err := docs[i].render(v, eng, where, e); err != nil {
			return nil, err
		}
		e.Documents++
	}

	f, err := os.Create(file)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	z := &epubZip{w: zip.NewWriter(f), when: time.Unix(opt.Epoch, 0).UTC()}

	// The mimetype goes in first and goes in uncompressed. It is the one rule of
	// the container format that is about the bytes of the zip rather than about
	// their content, and it is what lets a reading system know what it has
	// without unpacking anything.
	if err := z.store("mimetype", "application/epub+zip"); err != nil {
		return nil, err
	}
	add := []struct{ name, body string }{
		{"META-INF/container.xml", container},
		{"EPUB/package.opf", packageOPF(v, docs, opt)},
		{"EPUB/nav.xhtml", navDoc(v, docs)},
		{"EPUB/cover.xhtml", coverDoc(v)},
		{"EPUB/cover.svg", CoverSVG(v)},
		{"EPUB/style.css", epubCSS},
	}
	for _, a := range add {
		if err := z.add(a.name, a.body); err != nil {
			return nil, err
		}
	}
	for _, d := range docs {
		if err := z.add("EPUB/"+d.Name, d.page(v)); err != nil {
			return nil, err
		}
	}
	if err := z.w.Close(); err != nil {
		return nil, err
	}
	st, err := f.Stat()
	if err != nil {
		return nil, err
	}
	e.Bytes = st.Size()
	return e, nil
}

// A doc is one XHTML file of the book.
type doc struct {
	Name  string // text/ch-i-s1.xhtml, relative to EPUB/
	Kind  string // intro, chapter, section, exercises, historical
	Title string // the heading, and the label in the navigation
	// Chapter is the chapter this belongs to, for the exercises document, which
	// gathers a whole chapter's exercises the way the printing does.
	chapter *Chapter
	section *Section
	// Sub is the numbered subsections of a §, which the printed contents lists
	// and which the navigation therefore lists too.
	Sub  []subnav
	Body string
	math bool
	// Label is Title with the mathematics turned into plain characters, for the
	// navigation and for the <title> of the page, which are the two places in
	// the book where a formula cannot be a formula.
	Label string
}

type subnav struct {
	Frag  string
	Label string
}

// layout cuts the volume into documents.
//
// One file per division rather than one file per chapter, which is a choice
// about how a reading system behaves rather than about how the book is written:
// a chapter of Algebra is a hundred and twenty pages, and a reading system that
// has to lay out a hundred and twenty pages of MathML before it can show the
// first of them is a reading system that appears to have hung. A § is four to
// twenty pages and lays out at once.
func layout(v *Volume) []doc {
	var out []doc
	if v.Reader != nil {
		title := v.Reader.Title
		if title == "" {
			title = "To the Reader"
		}
		out = append(out, doc{Name: "text/reader.xhtml", Kind: "intro", Title: title, section: v.Reader})
	}
	if v.Intro != nil {
		title := v.Intro.Title
		if title == "" {
			title = "Introduction"
		}
		out = append(out, doc{Name: "text/intro.xhtml", Kind: "intro", Title: title, section: v.Intro})
	}
	for _, c := range v.Chapters {
		if c.Front == nil && len(c.Sections) == 0 {
			continue
		}
		num := strings.ToLower(c.Numeral)
		title := c.Title
		if title == "" {
			title = c.Numeral
		}
		out = append(out, doc{
			Name: "text/ch-" + num + ".xhtml", Kind: "chapter",
			Title: title, chapter: c, section: c.Front,
		})
		for _, s := range c.Sections {
			name := s.Label
			if name == "" {
				name = fmt.Sprintf("ch-%s-s%d", num, s.Number)
			}
			out = append(out, doc{
				Name: "text/" + name + ".xhtml", Kind: "section",
				Title: s.Heading() + ". " + s.Title, chapter: c, section: s,
			})
		}
		if chapterHasExercises(c) {
			out = append(out, doc{
				Name: "text/ch-" + num + "-ex.xhtml", Kind: "exercises",
				Title: bookWord(v.Lang, "exercises"), chapter: c,
			})
		}
		if c.Historical != nil {
			title := c.Historical.Title
			if title == "" {
				title = bookWord(v.Lang, "historical")
			}
			out = append(out, doc{
				Name: "text/ch-" + num + "-hist.xhtml", Kind: "historical",
				Title: title, chapter: c, section: c.Historical,
			})
		}
	}
	return out
}

func chapterHasExercises(c *Chapter) bool {
	for _, s := range c.Sections {
		if len(s.Exercises) > 0 {
			return true
		}
	}
	return false
}

// anchors is every name in this document something else can point at.
func (d *doc) anchors() []string {
	var out []string
	if d.Kind == "exercises" {
		for _, s := range d.chapter.Sections {
			for _, e := range s.Exercises {
				if e.Label != "" {
					out = append(out, e.Label)
				}
			}
		}
		return out
	}
	if d.section == nil {
		return out
	}
	if d.section.Label != "" {
		out = append(out, d.section.Label)
	}
	for _, m := range headingAnchorRE.FindAllStringSubmatch(d.section.Body, -1) {
		out = append(out, m[1])
	}
	return out
}

// render turns one document's corpus Markdown into the XHTML body of a page.
func (d *doc) render(v *Volume, eng *katex.Renderer, where map[string]string, e *EPUB) error {
	d.Label = plainMath(eng, plain(d.Title))
	var b strings.Builder
	switch d.Kind {
	case "chapter":
		fmt.Fprintf(&b, "<h1 class=\"chapter\"><span class=\"chapno\">%s %s</span>%s</h1>\n",
			esc(bookWord(v.Lang, "chapter")), esc(d.chapter.Numeral), tag("span", "chaptitle", esc(d.Title)))
	case "section":
		fmt.Fprintf(&b, "<h1 class=\"section\" id=\"%s\"><span class=\"secno\">%s</span>%s</h1>\n",
			esc(d.anchorOrName()), esc(d.section.Heading()), tag("span", "sectitle", esc(d.section.Title)))
	default:
		fmt.Fprintf(&b, "<h1 class=\"%s\">%s</h1>\n", d.Kind, esc(d.Title))
	}

	if d.Kind == "exercises" {
		if err := d.exercises(v, eng, where, e, &b); err != nil {
			return err
		}
		d.Body = b.String()
		return nil
	}
	if d.section == nil {
		d.Body = b.String()
		return nil
	}
	p := &pageRenderer{
		file: d.section.Path, line: d.section.Head, lang: v.Lang,
		contents: d.section.Contents, eng: eng, where: where, doc: d.Name, epub: e,
	}
	body, err := p.render(StripExercisePointer(StripTitle(d.section.Body)))
	if err != nil {
		return err
	}
	b.WriteString(body)
	d.Sub, d.math = p.sub, p.math
	d.Body = b.String()
	return nil
}

func (d *doc) anchorOrName() string {
	if d.section != nil && d.section.Label != "" {
		return d.section.Label
	}
	return strings.TrimSuffix(path.Base(d.Name), ".xhtml")
}

// exercises writes a whole chapter's exercises, gathered under one heading with
// the §§ named inside it, which is where the printing puts them.
func (d *doc) exercises(v *Volume, eng *katex.Renderer, where map[string]string, e *EPUB, b *strings.Builder) error {
	for _, s := range d.chapter.Sections {
		if len(s.Exercises) == 0 {
			continue
		}
		label := s.Label
		if label == "" {
			label = fmt.Sprintf("ch-%s-s%d", strings.ToLower(d.chapter.Numeral), s.Number)
		}
		frag := label + "-ex"
		fmt.Fprintf(b, "<h2 class=\"exfor\" id=\"%s\">%s %s</h2>\n", esc(frag),
			esc(bookWord(v.Lang, "exercisesFor")), esc(s.Heading()))
		d.Sub = append(d.Sub, subnav{Frag: frag,
			Label: bookWord(v.Lang, "exercisesFor") + " " + s.Heading()})
		for _, x := range s.Exercises {
			id := x.Label
			if id == "" {
				id = fmt.Sprintf("%s-ex-%d", label, x.Number)
			}
			p := &pageRenderer{
				file: x.Path, line: x.Head, lang: v.Lang,
				eng: eng, where: where, doc: d.Name, epub: e,
			}
			body, err := p.render(StripTitle(x.Body))
			if err != nil {
				return err
			}
			if p.math {
				d.math = true
			}
			star := ""
			if x.Starred {
				star = "*"
			}
			num := fmt.Sprintf("<span class=\"exnum\">%s%d)</span> ", star, x.Number)
			fmt.Fprintf(b, "<div class=\"exercise\" id=\"%s\">\n%s</div>\n", esc(id), lead(body, num))
		}
	}
	return nil
}

// lead puts the exercise number inside the first paragraph rather than above
// it, because that is where the printing has it: the number opens the run of
// text and does not stand on a line of its own.
func lead(body, num string) string {
	if i := strings.Index(body, "<p"); i >= 0 {
		if j := strings.Index(body[i:], ">"); j >= 0 {
			return body[:i+j+1] + num + body[i+j+1:]
		}
	}
	return "<p>" + num + "</p>\n" + body
}

// A pageRenderer turns one body of corpus Markdown into XHTML with MathML in it.
//
// It is a second renderer beside the LaTeX one and shares everything that reads
// the corpus with it: the same masking of the mathematics, the same cut into
// blocks, the same heading expression, the same reading of a statement head. All
// that differs is what comes out the far end, which is the way round it has to
// be. Two readers of the corpus would drift, and the day they drifted the PDF
// and the EPUB would be two different books with one name.
type pageRenderer struct {
	file     string
	line     int
	lang     string
	contents map[int]string
	eng      *katex.Renderer
	where    map[string]string
	doc      string
	epub     *EPUB

	sub  []subnav
	math bool
}

func (p *pageRenderer) at(line int) string {
	if p.file == "" {
		return fmt.Sprintf("line %d", line)
	}
	return fmt.Sprintf("%s:%d", p.file, p.line+line-1)
}

func (p *pageRenderer) render(body string) (string, error) {
	r := Renderer{File: p.file, Line: p.line}
	masked, spans, err := r.mask(body)
	if err != nil {
		return "", err
	}
	masked, tags := numbers(masked, spans)

	var b strings.Builder
	// state holds a statement head waiting for the paragraph it belongs to. The
	// corpus writes the head as a heading and the statement as the paragraph
	// under it, and the printing runs the two together on one line with the
	// statement in italic, so the head cannot be written out until the paragraph
	// after it has been seen.
	state, italic := "", false
	flushState := func() {
		if state != "" {
			b.WriteString("<p class=\"state\">" + state + "</p>\n")
			state, italic = "", false
		}
	}
	// There is no equivalent here of the LaTeX writer's joining of a block that
	// opens with a display to the paragraph before it. That join exists to keep
	// TeX out of vertical mode, which is a fact about TeX and about nothing
	// else: a reading system sets a div after a p with the margins the
	// stylesheet gives and no surprise space of its own.
	for _, block := range blocks(masked) {
		if strings.HasPrefix(block, "#") {
			flushState()
			head, opens, pending := p.heading(block, spans, tags)
			if pending != "" {
				state, italic = pending, opens
				continue
			}
			b.WriteString(head)
			continue
		}
		text, err := p.blockText(block, spans, tags)
		if err != nil {
			return "", err
		}
		if state != "" {
			if italic {
				text = "<em>" + text + "</em>"
			}
			b.WriteString("<p class=\"state\">" + state + text + "</p>\n")
			state, italic = "", false
			continue
		}
		if displayOnly(block) {
			b.WriteString(text)
			continue
		}
		b.WriteString("<p>" + text + "</p>\n")
	}
	flushState()
	return b.String(), nil
}

// heading renders a heading. It returns the XHTML for the ones that stand on
// their own, and for a statement head it returns the head as pending text
// instead, for the paragraph after it to be run on to.
func (p *pageRenderer) heading(line string, spans []mathtex.Span, tags map[int]string) (out string, italic bool, pending string) {
	m := headingRE.FindStringSubmatch(line)
	if m == nil {
		text, _ := p.blockText(line, spans, tags)
		return "<p>" + text + "</p>\n", false, ""
	}
	level, text, anchor, attrs := len(m[1]), strings.TrimSpace(m[2]), m[3], m[4]
	id := ""
	if anchor != "" {
		id = fmt.Sprintf(" id=\"%s\"", esc(anchor))
	}
	switch level {
	case 1, 2:
		return fmt.Sprintf("<h2 class=\"part\"%s>%s</h2>\n", id, p.inline(text, spans, tags)), false, ""
	case 3:
		if n := numberedRE.FindStringSubmatch(text); n != nil {
			frag := anchor
			if frag == "" {
				frag = "no-" + n[1]
				id = fmt.Sprintf(" id=\"%s\"", esc(frag))
			}
			// The navigation lists the sentence case title off manifests/toc/,
			// which is what the printed contents has, and falls back to the
			// heading itself for a language the volume was never printed in.
			label, listed := n[2], false
			if no, err := strconv.Atoi(n[1]); err == nil && p.contents[no] != "" {
				label, listed = p.contents[no], true
			}
			// A title off the manifest still has its formulae in dollars, since
			// nothing masked it, so navText would find no placeholder to fill
			// and the entry would read a literal "$\\tau$-Extensions". That is
			// what plainMath is for.
			nav := p.navText(label, spans)
			if listed {
				nav = plainMath(p.eng, plain(label))
			}
			p.sub = append(p.sub, subnav{Frag: frag, Label: n[1] + ". " + nav})
			return fmt.Sprintf("<h3 class=\"no\"%s><span class=\"nonum\">%s.</span> %s</h3>\n",
				id, esc(n[1]), p.inline(n[2], spans, tags)), false, ""
		}
		return fmt.Sprintf("<h3 class=\"named\"%s>%s</h3>\n", id, p.inline(text, spans, tags)), false, ""
	}
	kind, number, note := statement(text)
	var h strings.Builder
	if anchor != "" {
		fmt.Fprintf(&h, "<span class=\"stateanchor\" id=\"%s\"></span>", esc(anchor))
	}
	h.WriteString("<span class=\"statehead\">" + p.inline(kind, spans, tags))
	if number != "" {
		h.WriteString(" " + esc(number))
	}
	if note != "" {
		h.WriteString(" (" + p.inline(note, spans, tags) + ")")
	}
	h.WriteString("</span>")
	if t := tagAttrRE.FindStringSubmatch(attrs); t != nil {
		fmt.Fprintf(&h, "<span class=\"tag\">%s</span>", esc(t[1]))
	}
	// The dash is the printing's own: it sets the head, a full stop, a thin
	// space, a dash, a thin space, and then the statement, all on one line.
	h.WriteString(". — ")
	return "", !roman[kind], h.String()
}

// blockText renders one paragraph or display block.
func (p *pageRenderer) blockText(block string, spans []mathtex.Span, tags map[int]string) (string, error) {
	if displayOnly(block) {
		var b strings.Builder
		for _, m := range placeholderRE.FindAllString(block, -1) {
			var n int
			fmt.Sscanf(m, "\x00m%d\x00", &n)
			if n >= len(spans) {
				continue
			}
			math := p.set(spans[n], true)
			if t := tags[n]; t != "" {
				fmt.Fprintf(&b, "<div class=\"display tagged\"><span class=\"eqno\">(%s)</span>%s</div>\n", esc(t), math)
				continue
			}
			fmt.Fprintf(&b, "<div class=\"display\">%s</div>\n", math)
		}
		return b.String(), nil
	}
	return p.inline(strings.ReplaceAll(block, "\n", " "), spans, tags), nil
}

// inline renders the inside of a paragraph or a heading: the Markdown escapes,
// the emphasis, the links, the control sequences the corpus left in its prose,
// and the mathematics.
func (p *pageRenderer) inline(s string, spans []mathtex.Span, tags map[int]string) string {
	s, ctl := p.controls(s)
	var escaped []string
	s = mdEscapeRE.ReplaceAllStringFunc(s, func(m string) string {
		escaped = append(escaped, mdEscapeRE.FindStringSubmatch(m)[1])
		return escOpen + itoa(len(escaped)-1) + escClose
	})
	s = esc(s)
	s = boldRE.ReplaceAllString(s, "<strong>$1</strong>")
	s = emRE.ReplaceAllString(s, "$1<em>$2</em>$3")
	s = imageRE.ReplaceAllString(s, `<span class="figure">$1</span>`)
	s = linkRE.ReplaceAllStringFunc(s, func(m string) string {
		q := linkRE.FindStringSubmatch(m)
		return p.link(q[2], q[1])
	})
	if len(escaped) > 0 {
		s = escRestoreRE.ReplaceAllStringFunc(s, func(m string) string {
			var i int
			fmt.Sscanf(m, "\x00e%d\x00", &i)
			return esc(escaped[i])
		})
	}
	if len(ctl) > 0 {
		s = ctlRE.ReplaceAllStringFunc(s, func(m string) string {
			var i int
			fmt.Sscanf(m, "\x00c%d\x00", &i)
			return ctl[i]
		})
	}
	// The mathematics goes back last, so that nothing above reads the markup it
	// wrote as prose and escapes it a second time.
	return placeholderRE.ReplaceAllStringFunc(s, func(m string) string {
		var n int
		fmt.Sscanf(m, "\x00m%d\x00", &n)
		if n >= len(spans) {
			return ""
		}
		if spans[n].Display {
			return "<div class=\"display\">" + p.set(spans[n], true) + "</div>"
		}
		return p.set(spans[n], false)
	})
}

// link turns a corpus link into something a reading system can follow. A target
// inside this book becomes a real link, a target outside it becomes the words
// alone, and a URL stays a URL.
func (p *pageRenderer) link(url, text string) string {
	if a := anchorOf(url); a != "" {
		if where, ok := p.where[a]; ok {
			href := esc(relativeHref(p.doc, where) + "#" + a)
			return "<a href=\"" + href + "\">" + esc(text) + "</a>"
		}
	}
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		return "<a href=\"" + esc(url) + "\">" + esc(text) + "</a>"
	}
	return esc(text)
}

// relativeHref is one document seen from another. Every document is in text/,
// so this is the base name, and it is written as a function anyway because the
// day a book grows a second directory a bare base name is a broken link in
// several thousand places.
func relativeHref(from, to string) string {
	if path.Dir(from) == path.Dir(to) {
		return path.Base(to)
	}
	return to
}

// set renders one math span as MathML.
//
// A span KaTeX refuses is written out as the TeX it was, in a span the
// stylesheet sets in a monospaced face. This is the opposite of what publish
// does, and deliberately: the site stops on a refusal because the site is the
// published thing and a formula that is wrong there is wrong in public. The
// EPUB is built to find out what the corpus is like, so it carries on and
// counts, and the count goes in the audit with the file and the line.
func (p *pageRenderer) set(sp mathtex.Span, display bool) string {
	out, err := p.eng.MathML(sp.Text, display)
	if err != nil {
		p.epub.Refused = append(p.epub.Refused,
			Finding{Where: p.at(sp.Line), What: err.Error(), Count: 1})
		return "<span class=\"rawtex\">" + esc(sp.Text) + "</span>"
	}
	p.math = true
	p.epub.Math++
	// KaTeX wraps its MathML in a span of its own for the sake of a stylesheet
	// this book does not use. The math element is what EPUB requires a reading
	// system to understand, and it is taken out of the wrapper so that a
	// display sits in the page as a block rather than inside an inline span.
	out = strings.TrimPrefix(out, `<span class="katex">`)
	out = strings.TrimSuffix(out, "</span>")
	return out
}

// controls takes the TeX control sequences out of a line of prose and gives
// back the XHTML each one becomes. It walks the same table control.go counted
// out of this corpus, and differs only in what it emits: the mathematics goes
// to MathML rather than to dollars, and the text mode commands go to the markup
// that means what they mean.
func (p *pageRenderer) controls(s string) (string, []string) {
	if !strings.ContainsAny(s, `\_^`) {
		return s, nil
	}
	var out []string
	var b strings.Builder
	rs := []rune(s)
	for i := 0; i < len(rs); {
		if rs[i] != '\\' {
			// The mathematics the corpus wrote without dollars round it, read by
			// the same scanner the PDF uses and set as MathML rather than as
			// dollars. It has to be here as well as there: a reader with the EPUB
			// and a reader with the PDF are meant to be looking at the same book,
			// and before this the EPUB showed x_i with the underscore in every
			// one of the eighteen thousand places the PDF sets it properly.
			if end := atom(rs, i); end > i {
				// Through Math for the same reason the PDF does it: the run is
				// corpus text and its Greek and its operators are written as
				// characters, and KaTeX wants the TeX spelling of them.
				out = append(out, p.mathml(Math(string(rs[i:end]))))
				b.WriteString(ctlOpen + itoa(len(out)-1) + ctlClose)
				i = end
				continue
			}
			b.WriteRune(rs[i])
			i++
			continue
		}
		name, next := controlName(rs, i)
		if name == "" {
			b.WriteRune('\\')
			i++
			continue
		}
		if len(name) == 1 && strings.ContainsAny(name, `*_$#[]()~^\&%{}`) {
			b.WriteString(`\` + name)
			i = next
			continue
		}
		cmd, ok := control[name]
		if !ok {
			b.WriteString(`\` + name)
			i = next
			continue
		}
		args, end, complete := arguments(rs, next, cmd.args, cmd.bare)
		if !complete {
			b.WriteString(`\` + name)
			i = next
			continue
		}
		suffix := ""
		if cmd.math {
			if e := decorations(rs, end); e > end {
				suffix = string(rs[end:e])
				end = e
			}
		}
		out = append(out, p.emit(cmd, name, args, suffix))
		b.WriteString(ctlOpen + itoa(len(out)-1) + ctlClose)
		i = end
	}
	return b.String(), out
}

// mathml is one run of TeX as the markup a reading system can set, or as the
// TeX itself in a span the stylesheet marks when KaTeX will not read it.
func (p *pageRenderer) mathml(tex string) string {
	out, err := p.eng.MathML(tex, false)
	if err != nil {
		return "<span class=\"rawtex\">" + esc(tex) + "</span>"
	}
	p.epub.Math++
	out = strings.TrimPrefix(out, `<span class="katex">`)
	return strings.TrimSuffix(out, "</span>")
}

func (p *pageRenderer) emit(c cmd, name string, args []string, suffix string) string {
	if c.raw != "" {
		return escapes[c.raw]
	}
	if c.math {
		tex := `\` + name
		for _, a := range args {
			tex += "{" + a + "}"
		}
		return p.mathml(tex + suffix)
	}
	if len(args) == 0 {
		return words[name]
	}
	if m, ok := accent[name]; ok && len(args) == 1 {
		return esc(args[0]) + m
	}
	inner := esc(args[0])
	switch name {
	case "emph", "textit":
		return "<em>" + inner + "</em>"
	case "textbf":
		return "<strong>" + inner + "</strong>"
	case "textsuperscript":
		return "<sup>" + inner + "</sup>"
	case "footnote", "footnotetext":
		// A reading system paginates for itself, so there is no foot of a page
		// to put a note at. It goes where the printing's own footnote marker is,
		// set apart, which is what every EPUB of a mathematics book does.
		return "<span class=\"footnote\">" + inner + "</span>"
	case "hspace":
		return " "
	}
	return inner
}

// words is the text mode commands that take no argument, as the characters they
// stand for. \S is the commonest thing in the corpus's stray TeX by a factor of
// four and it is not a defect: it is how the corpus writes the section sign.
var words = map[string]string{
	"S":     "§",
	"P":     "¶",
	"quad":  " ",
	"hfill": " ",
}

// escapes is the three spellings control.go rewrites, keyed by the LaTeX it
// rewrites them to and holding the character LaTeX would have printed. There is
// nothing in an EPUB to read a control sequence, so the thin space is a real
// thin space and the section sign is the sign.
var escapes = map[string]string{
	" ":     " ",
	"\\,":   "\u2009",
	"\\S{}": "\u00a7",
}

// accent is the three the corpus spells the old way, as the combining mark that
// goes after the letter. M\'eray is Méray and a book that printed it as M'eray
// would have a French name wrong in the historical notes.
var accent = map[string]string{
	"'": "́", // acute
	"v": "̌", // caron
	"c": "̧", // cedilla
}

// bookWord is the five words this build writes that are not in the corpus. The
// class has the same five in TeX, in \blanguage, and they are here again because
// a .cls file cannot be read from Go and a table of five words in four languages
// does not want a message catalogue over it. They are checked against each other
// by a test.
var bookWords = map[string]map[string]string{
	"en": {
		"chapter": "CHAPTER", "exercises": "EXERCISES", "exercisesFor": "Exercises for",
		"historical": "HISTORICAL NOTE", "contents": "CONTENTS",
	},
	"fr": {
		"chapter": "CHAPITRE", "exercises": "EXERCICES", "exercisesFor": "Exercices du",
		"historical": "NOTE HISTORIQUE", "contents": "TABLE DES MATIÈRES",
	},
	"vi": {
		"chapter": "CHƯƠNG", "exercises": "BÀI TẬP", "exercisesFor": "Bài tập cho",
		"historical": "GHI CHÚ LỊCH SỬ", "contents": "MỤC LỤC",
	},
}

func bookWord(lang, key string) string {
	if m, ok := bookWords[lang]; ok {
		if w := m[key]; w != "" {
			return w
		}
	}
	return bookWords["en"][key]
}

// plain is a title with its markup taken off, for a navigation label. The
// navigation is text and nothing else: a reading system shows it in a list and
// an element in it is either ignored or shown as its own angle brackets.
func plain(s string) string {
	s = boldRE.ReplaceAllString(s, "$1")
	s = emRE.ReplaceAllString(s, "$1$2$3")
	s = imageRE.ReplaceAllString(s, "$1")
	s = linkRE.ReplaceAllString(s, "$1")
	return strings.TrimSpace(s)
}

// navText is a heading with its mathematics turned into plain characters, for
// the one place in the book where a formula cannot be a formula.
//
// A reading system draws the table of contents in its own furniture, usually a
// list in a sidebar that none of our stylesheet ever reaches, and the format
// asks that the content of a navigation link resolve to text. So the formulae
// in a subsection title have to come out as characters or not at all, and not
// at all is worse than it sounds: dropping them turns a Vietnamese Algebra
// entry into "5. -NHOM", which is a contents line that lies about the page it
// points at.
//
// KaTeX already knows what the characters are. Its MathML is a tree of mi, mo
// and mn whose text is exactly the symbols the formula sets, so rendering the
// span and keeping the text gives the Greek letter for \Lambda and the sign for
// \otimes, out of the same engine that sets the same span on the page. What it
// does not give is a variant, which MathML carries as an attribute rather than
// as a character, so \mathfrak{S} comes back as a plain S. That is a small loss
// and the honest one to take, since the alternative is this program guessing at
// a Unicode block. A span the engine refuses falls back to its own source,
// which is ugly and true.
func (p *pageRenderer) navText(s string, spans []mathtex.Span) string {
	return placeholderRE.ReplaceAllStringFunc(plain(s), func(m string) string {
		var n int
		fmt.Sscanf(m, "\x00m%d\x00", &n)
		if n >= len(spans) {
			return ""
		}
		return mathPlain(p.eng, spans[n].Text, spans[n].Display)
	})
}

// plainMath is navText for a string that was never masked, which is every title
// that came off the front matter rather than out of a body.
func plainMath(eng *katex.Renderer, body string) string {
	spans, unclosed := mathtex.Split(body)
	if len(spans) == 0 || unclosed != nil {
		return body
	}
	rs := []rune(body)
	var b strings.Builder
	at := 0
	for _, sp := range spans {
		d := 1
		if sp.Display {
			d = 2
		}
		if sp.Start-d < at || sp.End+d > len(rs) {
			return body
		}
		b.WriteString(string(rs[at : sp.Start-d]))
		b.WriteString(mathPlain(eng, sp.Text, sp.Display))
		at = sp.End + d
	}
	b.WriteString(string(rs[at:]))
	return b.String()
}

// mathPlain is one span of TeX as the characters it sets.
func mathPlain(eng *katex.Renderer, tex string, display bool) string {
	out, err := eng.MathML(tex, display)
	if err != nil {
		return tex
	}
	dec := xml.NewDecoder(strings.NewReader(out))
	dec.Strict = false
	dec.Entity = xml.HTMLEntity
	var b strings.Builder
	skip := 0
	for {
		tok, err := dec.Token()
		if err != nil {
			break
		}
		switch t := tok.(type) {
		case xml.StartElement:
			// KaTeX puts the TeX it was given back into the MathML as an
			// annotation, for a tool that wants to recover the source. Read as
			// text along with everything else it would print the formula twice,
			// once as symbols and once as backslashes.
			if skip > 0 || t.Name.Local == "annotation" {
				skip++
			}
		case xml.EndElement:
			if skip > 0 {
				skip--
			}
		case xml.CharData:
			if skip == 0 {
				b.WriteString(string(t))
			}
		}
	}
	// The invisible operators go. MathML writes U+2061 between a function and
	// its argument so that a reader aloud says "Hom of", and a reading system
	// that draws the contents as plain text draws nothing for them, so they are
	// bytes in a label that no one can see and no one can search for.
	return strings.Join(strings.Fields(invisible.Replace(b.String())), " ")
}

// invisible is the four MathML operators that are meant to have no glyph.
var invisible = strings.NewReplacer(
	"\u2061", "", "\u2062", "", "\u2063", "", "\u2064", "",
)

func esc(s string) string { return html.EscapeString(s) }

func tag(el, class, body string) string {
	return "<" + el + " class=\"" + class + "\">" + body + "</" + el + ">"
}

// page wraps a rendered body in the XHTML a reading system opens.
func (d *doc) page(v *Volume) string {
	return xhtmlHead(v.Lang, d.Label, "../style.css") + d.Body + xhtmlFoot
}

// xhtmlHead opens a document. sheet is where the stylesheet is from this
// document, which is not the same for all of them: the chapters live in text/
// and the navigation and the cover live beside the stylesheet at the top. A
// single spelling would leave two of the files pointing at a stylesheet that is
// not there, which is a book that opens unstyled on the strict readers and
// unstyled everywhere on the rest.
func xhtmlHead(lang, title, sheet string) string {
	return `<?xml version="1.0" encoding="utf-8"?>` + "\n" +
		`<!DOCTYPE html>` + "\n" +
		`<html xmlns="http://www.w3.org/1999/xhtml" xmlns:epub="http://www.idpf.org/2007/ops" xml:lang="` +
		esc(lang) + `" lang="` + esc(lang) + `">` + "\n<head>\n" +
		"<meta charset=\"utf-8\"/>\n<title>" + esc(title) + "</title>\n" +
		"<link rel=\"stylesheet\" type=\"text/css\" href=\"" + sheet + "\"/>\n</head>\n<body>\n"
}

const xhtmlFoot = "</body>\n</html>\n"

// ---------------------------------------------------------------------------
// The container.
// ---------------------------------------------------------------------------

type epubZip struct {
	w    *zip.Writer
	when time.Time
}

func (z *epubZip) store(name, body string) error {
	w, err := z.w.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Store, Modified: z.when})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(body))
	return err
}

func (z *epubZip) add(name, body string) error {
	w, err := z.w.CreateHeader(&zip.FileHeader{Name: name, Method: zip.Deflate, Modified: z.when})
	if err != nil {
		return err
	}
	_, err = w.Write([]byte(body))
	return err
}

const container = `<?xml version="1.0" encoding="UTF-8"?>
<container version="1.0" xmlns="urn:oasis:names:tc:opendocument:xmlns:container">
  <rootfiles>
    <rootfile full-path="EPUB/package.opf" media-type="application/oebps-package+xml"/>
  </rootfiles>
</container>
`

// packageOPF is the manifest and the spine.
//
// The identifier is built out of the volume and the language rather than being
// a fresh UUID, because two builds of the same content have to come out as the
// same bytes. A random identifier would make every rebuild a different book as
// far as a library is concerned, and would make it impossible to tell a build
// that changed from a build that did not.
func packageOPF(v *Volume, docs []doc, opt Options) string {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	b.WriteString(`<package xmlns="http://www.idpf.org/2007/opf" version="3.0" unique-identifier="bookid" xml:lang="` + esc(v.Lang) + `">` + "\n")
	b.WriteString("<metadata xmlns:dc=\"http://purl.org/dc/elements/1.1/\">\n")
	fmt.Fprintf(&b, "<dc:identifier id=\"bookid\">urn:bourbaki:%s</dc:identifier>\n", esc(v.ID()))
	fmt.Fprintf(&b, "<dc:title>%s</dc:title>\n", esc(v.Title))
	fmt.Fprintf(&b, "<dc:language>%s</dc:language>\n", esc(v.Lang))
	fmt.Fprintf(&b, "<dc:creator>%s</dc:creator>\n", esc(Author))
	fmt.Fprintf(&b, "<dc:publisher>%s</dc:publisher>\n", esc(Series))
	if v.Meta.Edition != "" {
		fmt.Fprintf(&b, "<dc:source>%s</dc:source>\n", esc(v.Meta.Edition))
	}
	fmt.Fprintf(&b, "<meta property=\"dcterms:modified\">%s</meta>\n",
		time.Unix(opt.Epoch, 0).UTC().Format("2006-01-02T15:04:05Z"))
	b.WriteString("<meta name=\"cover\" content=\"cover-image\"/>\n")
	b.WriteString("</metadata>\n<manifest>\n")
	b.WriteString(`<item id="nav" href="nav.xhtml" media-type="application/xhtml+xml" properties="nav"/>` + "\n")
	b.WriteString(`<item id="cover" href="cover.xhtml" media-type="application/xhtml+xml" properties="svg"/>` + "\n")
	b.WriteString(`<item id="cover-image" href="cover.svg" media-type="image/svg+xml" properties="cover-image"/>` + "\n")
	b.WriteString(`<item id="css" href="style.css" media-type="text/css"/>` + "\n")
	for i, d := range docs {
		props := ""
		if d.math {
			props = ` properties="mathml"`
		}
		fmt.Fprintf(&b, "<item id=\"d%d\" href=\"%s\" media-type=\"application/xhtml+xml\"%s/>\n",
			i, esc(d.Name), props)
	}
	b.WriteString("</manifest>\n<spine>\n")
	b.WriteString(`<itemref idref="cover"/>` + "\n")
	b.WriteString(`<itemref idref="nav"/>` + "\n")
	for i := range docs {
		fmt.Fprintf(&b, "<itemref idref=\"d%d\"/>\n", i)
	}
	b.WriteString("</spine>\n</package>\n")
	return b.String()
}

// navDoc is the table of contents, at the three levels the printing has: the
// chapter, the § inside it, and the numbered subsections inside that.
func navDoc(v *Volume, docs []doc) string {
	var b strings.Builder
	b.WriteString(xhtmlHead(v.Lang, bookWord(v.Lang, "contents"), "style.css"))
	b.WriteString(`<nav epub:type="toc" id="toc">` + "\n")
	fmt.Fprintf(&b, "<h1>%s</h1>\n", esc(bookWord(v.Lang, "contents")))
	b.WriteString("<ol>\n")
	open := false
	closeChapter := func() {
		if open {
			b.WriteString("</ol></li>\n")
			open = false
		}
	}
	for _, d := range docs {
		href := esc(d.Name)
		label := esc(d.Label)
		if d.Kind == "chapter" {
			closeChapter()
			fmt.Fprintf(&b, "<li><a href=\"%s\">%s %s. %s</a>\n<ol>\n",
				href, esc(bookWord(v.Lang, "chapter")), esc(d.chapter.Numeral), label)
			open = true
			continue
		}
		if !open {
			fmt.Fprintf(&b, "<li><a href=\"%s\">%s</a></li>\n", href, label)
			continue
		}
		if len(d.Sub) == 0 {
			fmt.Fprintf(&b, "<li><a href=\"%s\">%s</a></li>\n", href, label)
			continue
		}
		fmt.Fprintf(&b, "<li><a href=\"%s\">%s</a>\n<ol>\n", href, label)
		for _, s := range d.Sub {
			fmt.Fprintf(&b, "<li><a href=\"%s#%s\">%s</a></li>\n", href, esc(s.Frag), esc(s.Label))
		}
		b.WriteString("</ol></li>\n")
	}
	closeChapter()
	b.WriteString("</ol>\n</nav>\n")
	b.WriteString(xhtmlFoot)
	return b.String()
}

func coverDoc(v *Volume) string {
	w, h := v.Meta.PageWidth, v.Meta.PageHeight
	if w == 0 || h == 0 {
		w, h = 363.12, 565.56
	}
	var b strings.Builder
	b.WriteString(xhtmlHead(v.Lang, v.Title, "style.css"))
	b.WriteString(`<div class="cover">` + "\n")
	fmt.Fprintf(&b, `<svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" version="1.1" width="100%%" height="100%%" viewBox="0 0 %.2f %.2f" preserveAspectRatio="xMidYMid meet"><image width="%.2f" height="%.2f" xlink:href="cover.svg"/></svg>`+"\n", w, h, w, h)
	b.WriteString("</div>\n")
	b.WriteString(xhtmlFoot)
	return b.String()
}

// CoverSVG draws the yellow cover.
//
// It is the same cover the class draws and it is drawn from the same measured
// numbers: the yellow is #FEC746 and the type is #0077B4, the wordmark sits at
// 0.050 of the trim height and the series line at 0.157, the title at 0.2745
// and the chapter line at 0.330, and the first two are set to a width of 0.78
// of the trim rather than to a size, because that is what the printed covers do
// on both of the volumes that were measured.
//
// textLength with lengthAdjust is the SVG spelling of \resizebox. Without it the
// two drawn lines would come out at whatever width the reading system's own
// serif face happens to give them, which is a different width on every device,
// and the cover of a Bourbaki is a piece of design rather than a paragraph.
func CoverSVG(v *Volume) string {
	w, h := v.Meta.PageWidth, v.Meta.PageHeight
	if w == 0 || h == 0 {
		w, h = 363.12, 565.56
	}
	line := func(top, size float64, textLen float64, s string) string {
		length := ""
		if textLen > 0 {
			length = fmt.Sprintf(" textLength=\"%.2f\" lengthAdjust=\"spacingAndGlyphs\"", textLen*w)
		}
		return fmt.Sprintf(
			"<text x=\"%.2f\" y=\"%.2f\" text-anchor=\"middle\" font-family=\"serif\" font-size=\"%.2f\" fill=\"#0077B4\"%s>%s</text>\n",
			w/2, top*h+size*0.72, size, length, esc(s))
	}
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="utf-8"?>` + "\n")
	fmt.Fprintf(&b, "<svg xmlns=\"http://www.w3.org/2000/svg\" version=\"1.1\" width=\"%.2f\" height=\"%.2f\" viewBox=\"0 0 %.2f %.2f\">\n", w, h, w, h)
	fmt.Fprintf(&b, "<title>%s. %s</title>\n", esc(v.Title), esc(v.ChapterSpan()))
	fmt.Fprintf(&b, "<rect x=\"0\" y=\"0\" width=\"%.2f\" height=\"%.2f\" fill=\"#FEC746\"/>\n", w, h)
	b.WriteString(line(0.050, 0.089*h, 0.78, Author))
	b.WriteString(line(0.157, 0.041*h, 0.78, Series))
	b.WriteString(line(0.2745, 22, 0, v.Title))
	b.WriteString(line(0.330, 13, 0, v.ChapterSpan()))
	b.WriteString("</svg>\n")
	return b.String()
}

// epubCSS is deliberately short. A reading system is a browser with opinions and
// a user with a font size, and a stylesheet that fights either of them makes the
// book worse. What is set here is what the corpus needs and nothing else: the
// displays, the numbered subsection heads, the statement heads, the permanent
// tags, and the one thing that must not happen, which is a wide formula pushing
// the page sideways.
const epubCSS = `html { font-size: 100%; }
body { margin: 0 5%; line-height: 1.45; text-align: justify; hyphens: auto; }
p { margin: 0; text-indent: 1.2em; }
p.state, p + div.display + p { text-indent: 0; }
h1 { font-size: 1.3em; text-align: center; margin: 1.5em 0 1.2em; font-weight: normal; }
h1 .chapno, h1 .secno { display: block; font-variant: small-caps; font-size: 0.8em; margin-bottom: 0.5em; }
h1 .chaptitle, h1 .sectitle { display: block; }
h2 { font-size: 1.05em; margin: 1.4em 0 0.6em; }
h2.exfor { font-style: italic; font-weight: normal; }
h3 { font-size: 1em; margin: 1.3em 0 0.4em; font-weight: bold; text-transform: none; }
h3.no .nonum { font-weight: bold; }
div.display { margin: 0.7em 0; text-align: center; overflow-x: auto; }
div.display.tagged { position: relative; }
div.display .eqno { float: left; }
.statehead { font-weight: bold; }
.tag { font-family: monospace; font-size: 0.7em; color: #777; margin-left: 0.4em; }
.exnum { font-weight: bold; }
div.exercise { margin: 0.6em 0; }
.footnote { font-size: 0.85em; }
.figure { display: block; text-align: center; font-style: italic; font-size: 0.9em; border: 1px solid #999; padding: 0.6em; margin: 0.9em 0; text-indent: 0; }
.rawtex { font-family: monospace; background: #fee; }
.cover { margin: 0; padding: 0; text-align: center; }
math { font-size: 1em; }
`
