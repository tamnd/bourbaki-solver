// Package publish turns the committed Markdown into a static site.
//
// It reads nothing that is not committed. No PDF, no work/ directory, no
// network. That is what makes the site reproducible from a public clone, and it
// is the property the whole milestone rests on: a reader who does not trust the
// site can build it themselves out of the same files.
package publish

import (
	"fmt"
	"html"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"github.com/tamnd/bourbaki-solver/katex"
	"github.com/tamnd/bourbaki-solver/mathtex"
)

// The corpus is machine-written Markdown over a small and known vocabulary, and
// the renderer is written for that vocabulary rather than for CommonMark.
//
// Measured over content/en as committed: no lists, no tables, no block quotes,
// no code fences, no images, no raw HTML. What there is is headings with an
// attribute block, paragraphs, display formulae, 28 bold spans and 25 links.
//
// A general Markdown library would be the safe answer if the input were written
// by people. It is the dangerous answer here, because the input is full of TeX
// and TeX and Markdown fight over the same characters. `$x^*\otimes y$` and
// `$x^*\in E^*$` in one sentence give a Markdown parser two asterisks to pair
// up, and the paragraph comes out with an emphasis span cut through the middle
// of two formulae. The mathematics is masked before anything else looks at the
// line, which is the only way to be sure of that, and once it is masked there
// is very little Markdown left to read.

// mask replaces every math span with a placeholder and returns the spans in the
// order they were taken out.
//
// The placeholder is built from NUL, which cannot occur in the corpus: the
// files are UTF-8 text out of a PDF and a NUL in one would fail the audit long
// before it reached here. Nothing downstream escapes it or breaks a line at it.
//
// It is a method for the sake of the one error it returns. A span that opens
// and never closes used to be reported as a bare body line number, which is a
// line in nothing: the body starts under the front matter, so the number is not
// the file's, and twenty thousand files were candidates for it. The renderer
// already carries the path and the offset for exactly this, and the refusal
// path already uses them, so this uses them too and the message names a file
// somebody can open.
func (r Renderer) mask(body string) (string, []mathtex.Span, error) {
	spans, unclosed := mathtex.Split(body)
	if unclosed != nil {
		return "", nil, fmt.Errorf("%s: a math span opens and never closes", r.at(unclosed.Line))
	}
	rs := []rune(body)
	var b strings.Builder
	at := 0
	for i, s := range spans {
		d := 1
		if s.Display {
			d = 2
		}
		b.WriteString(string(rs[at : s.Start-d]))
		fmt.Fprintf(&b, "\x00m%d\x00", i)
		at = s.End + d
	}
	b.WriteString(string(rs[at:]))
	return b.String(), spans, nil
}

var placeholderRE = regexp.MustCompile("\x00m(\\d+)\x00")

// mathEngine is the one KaTeX engine the package uses.
//
// It is package state, which is worth a word. Loading katex.min.js costs about
// a fifth of a second and rendering a span costs a fraction of a millisecond,
// and the corpus has 20 thousand spans, so building an engine per Renderer
// would be a fifth of a second per section for nothing. The engine holds no
// corpus state, is safe to call from several goroutines, and answers the same
// TeX with the same bytes for the life of the process, so sharing it cannot
// make two builds differ.
var mathEngine = sync.OnceValues(katex.New)

// unmask puts the mathematics back, rendered.
//
// A span KaTeX refuses stops the build. There is a tempting fallback here, of
// writing the TeX source out and letting the reader see it, and it is the wrong
// one: the site would then publish a formula the extraction got wrong in a form
// that looks deliberate, and nobody would ever fix it. The error names the file,
// the line and the span, which is what somebody needs to open the page and
// compare it against the book.
func (r Renderer) unmask(s string, spans []mathtex.Span) (string, error) {
	if len(spans) == 0 {
		return s, nil
	}
	eng, err := mathEngine()
	if err != nil {
		return "", err
	}
	out := make([]string, len(spans))
	for i, sp := range spans {
		render := eng.Render
		if r.MathML {
			render = eng.MathML
		}
		m, err := render(sp.Text, sp.Display)
		if err != nil {
			ref := Refusal{At: r.at(sp.Line), Span: sp.Text, Why: err.Error(), Display: sp.Display}
			if r.OnRefusal == nil {
				return "", ref
			}
			out[i] = r.OnRefusal(ref)
			continue
		}
		// The wrapper stays even though KaTeX brings its own. It is what the
		// stylesheet of this site hangs the scroll on a display too wide for a
		// phone, and keeping it means the rest of the page does not change
		// shape when KaTeX does.
		if sp.Display {
			out[i] = `<div class="math display">` + m + `</div>`
			continue
		}
		out[i] = `<span class="math">` + m + `</span>`
	}
	return placeholderRE.ReplaceAllStringFunc(s, func(m string) string {
		var i int
		fmt.Sscanf(m, "\x00m%d\x00", &i)
		return out[i]
	}), nil
}

// A Refusal is one span KaTeX would not set, and where it is.
type Refusal struct {
	At      string // file:line, or line N when the body came from nowhere
	Span    string // the TeX as the corpus has it
	Why     string // KaTeX's own message, which names the character it stopped at
	Display bool
}

func (r Refusal) Error() string {
	return fmt.Sprintf("%s: %s: %s", r.At, r.Why, oneLine(r.Span))
}

// at is where a body line is in the corpus, in the file:line form an editor and
// a CI annotation both take.
func (r Renderer) at(line int) string {
	if r.File == "" {
		return fmt.Sprintf("line %d", line)
	}
	return fmt.Sprintf("%s:%d", r.File, r.Line+line-1)
}

// oneLine puts a span on one line and cuts it, so that a display of fifteen
// lines does not bury the sentence that names it.
func oneLine(s string) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) > 60 {
		s = string([]rune(s)[:60]) + "..."
	}
	return s
}

// headingRE reads a heading and the attribute block assembly writes on it:
// the label, the classes, and tag=XXXX.
var headingRE = regexp.MustCompile(`^(#{1,6})\s+(.*?)(?:\s*\{#([a-z0-9-]+)((?:\s+[^}\s]+)*)\})?\s*$`)

// tagAttrRE is tag=XXXX inside that block.
var tagAttrRE = regexp.MustCompile(`\btag=([0-9A-Z]{4})\b`)

// Heading is one heading of a body.
type Heading struct {
	Level int
	Text  string // the heading as written, mathematics and all
	Label string // the {#label} anchor, empty when the heading has none
	Tag   string
	// Statement is whether the heading carries .statement, which is what says
	// this is one of the units that gets a permanent tag and a page.
	Statement bool
	Line      int // counting from zero, within the body
}

// Headings reads the headings of a body without rendering it.
func Headings(body string) []Heading {
	var out []Heading
	for i, line := range strings.Split(body, "\n") {
		if !strings.HasPrefix(line, "#") {
			continue
		}
		m := headingRE.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		h := Heading{Level: len(m[1]), Text: strings.TrimSpace(m[2]), Label: m[3], Line: i}
		h.Statement = strings.Contains(m[4], ".statement")
		if t := tagAttrRE.FindStringSubmatch(m[4]); t != nil {
			h.Tag = t[1]
		}
		out = append(out, h)
	}
	return out
}

// Unit is one statement of a section: its heading and the body under it, up to
// the next heading of any level.
//
// This is the cut a tag page needs. Bourbaki sets the proof directly after the
// statement with nothing between them, so the body of a unit is the statement
// and its proof together, and the corpus does not separate the two. Saying so
// on the page is better than pretending a separation that is not in the files.
type Unit struct {
	Heading Heading
	Body    string
}

// Units cuts a body at its headings and returns the statements.
func Units(body string) []Unit {
	lines := strings.Split(body, "\n")
	heads := Headings(body)
	var out []Unit
	for i, h := range heads {
		if !h.Statement {
			continue
		}
		end := len(lines)
		if i+1 < len(heads) {
			end = heads[i+1].Line
		}
		out = append(out, Unit{Heading: h, Body: strings.Trim(strings.Join(lines[h.Line+1:end], "\n"), "\n")})
	}
	return out
}

// Renderer turns a body into HTML. Link is asked for the href of a tag, so that
// the section page can link a statement heading at its tag page and the tag
// page can link the same heading at itself.
type Renderer struct {
	Link func(tag string) string
	// Dir is the URL that a relative link in the body resolves against, ending
	// in a slash. Empty leaves relative links alone.
	Dir string
	// Exercises is where the exercises of a § of this chapter are on the site,
	// given "s5" or "a1" and an exercise number, or zero for the whole set. Nil
	// resolves such a link like any other relative one.
	Exercises func(dir string, n int) string
	// File and Line say where the body came from: the path as committed, and
	// the file line the body's first line sits on. They are for the one message
	// this renderer can produce, a formula KaTeX will not read, and a message
	// that cannot be turned back into a place in a file is a message that gets
	// ignored.
	File string
	Line int
	// OnRefusal is what to put on the page in place of a formula KaTeX will not
	// set. Nil, which is the default and what every deploy uses, makes a
	// refusal an error and stops the build. It exists so that somebody working
	// on the site can look at the other twenty thousand formulae while the
	// hundred and thirty five broken ones wait on a repair pass that needs the
	// printed page open.
	OnRefusal func(Refusal) string

	// Lang is the language of the body, which is what decides whether a word
	// spelled like a roman numeral is one when a heading printed in capitals is
	// brought down to a sentence.
	Lang string
	// MathML sets the mathematics as MathML alone, for a host that cannot
	// carry the size of KaTeX's positioned spans. See Site.MathML.
	MathML bool
	// Xref is the foot of a statement card, given its label: what it cites
	// and what cites it. Nil leaves the cards bare.
	Xref func(label string) string
	// Section is the label of the § the body is, alg-i-s1, which a numbered
	// subsection heading takes its anchor from: alg-i-s1-n3 for no. 3.
	Section string
}

// HTML renders one body.
func (r Renderer) HTML(body string) (string, error) {
	masked, spans, err := r.mask(body)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	bs := blocks(masked)
	for i := 0; i < len(bs); i++ {
		block := bs[i]
		if !strings.HasPrefix(block, "#") {
			b.WriteString(r.para(block))
			continue
		}
		m := headingRE.FindStringSubmatch(block)
		if m == nil || !strings.Contains(m[4], ".statement") {
			b.WriteString(r.heading(block))
			continue
		}
		// A statement is set as a card: its heading and the statement itself.
		// What follows it up to the next heading is its proof, or for a
		// definition, an example or a remark the rest of it.
		kind := labelKind(m[3])
		end := i + 1
		for end < len(bs) && !strings.HasPrefix(bs[end], "#") {
			end++
		}
		body := bs[i+1 : end]
		n := len(body)
		if proves(kind) || kind == "definition" {
			n = statementLen(body)
		}
		if m[3] != "" {
			fmt.Fprintf(&b, "<div class=\"stmt k-%s\" data-label=\"%s\">\n%s", kind, html.EscapeString(m[3]), r.heading(block))
		} else {
			fmt.Fprintf(&b, "<div class=\"stmt k-%s\">\n%s", kind, r.heading(block))
		}
		for _, p := range body[:n] {
			b.WriteString(r.para(strings.TrimSpace(trimItalic(p))))
		}
		if r.Xref != nil && m[3] != "" {
			b.WriteString(r.Xref(m[3]))
		}
		b.WriteString("</div>\n")
		switch {
		case n == len(body):
		case proves(kind):
			b.WriteString("<div class=\"proof\">\n")
			for _, p := range body[n:] {
				b.WriteString(r.para(p))
			}
			b.WriteString("</div>\n")
		default:
			// What follows a definition is the text going on, not a proof.
			for _, p := range body[n:] {
				b.WriteString(r.para(p))
			}
		}
		i = end - 1
	}
	return r.unmask(b.String(), spans)
}

// para is one block of prose, or a display formula standing alone.
func (r Renderer) para(block string) string {
	if placeholderRE.MatchString(block) && strings.TrimSpace(placeholderRE.ReplaceAllString(block, "")) == "" {
		return r.inline(block) + "\n"
	}
	if items := listItems(block); items != nil {
		var b strings.Builder
		b.WriteString("<ul>\n")
		for _, it := range items {
			b.WriteString("<li>" + r.inline(it) + "</li>\n")
		}
		b.WriteString("</ul>\n")
		return b.String()
	}
	return "<p>" + r.inline(strings.ReplaceAll(block, "\n", " ")) + "</p>\n"
}

var listRE = regexp.MustCompile(`^[-*] +\S`)

// listItems reads a block every line of which opens with a bullet as a list.
func listItems(block string) []string {
	lines := strings.Split(block, "\n")
	if len(lines) < 2 {
		return nil
	}
	var out []string
	for _, l := range lines {
		if !listRE.MatchString(l) {
			return nil
		}
		out = append(out, strings.TrimSpace(l[1:]))
	}
	return out
}

// labelKind is a statement's kind as its label names it: alg-i-s1-prop-2 is a
// proposition. The label is the same in every language, which the heading is
// not, so this is what colours the card.
func labelKind(label string) string {
	parts := strings.Split(label, "-")
	for i := len(parts) - 1; i >= 0; i-- {
		switch parts[i] {
		case "def":
			return "definition"
		case "thm":
			return "theorem"
		case "prop":
			return "proposition"
		case "lem":
			return "lemma"
		case "cor":
			return "corollary"
		case "exa":
			return "example"
		case "rem", "sch":
			return "remark"
		}
	}
	return "other"
}

// proves is a kind whose statement is followed by a proof.
func proves(kind string) bool {
	switch kind {
	case "theorem", "proposition", "lemma", "corollary":
		return true
	}
	return false
}

// statementLen is how many blocks under a heading are the statement. The book
// sets a statement in italic, which the corpus writes as a paragraph opened
// with an asterisk, so the statement runs to the block that closes it. With no
// marker it is the first paragraph and any display formula that belongs to it.
func statementLen(body []string) int {
	if len(body) == 0 {
		return 0
	}
	first := strings.TrimSpace(body[0])
	if strings.HasPrefix(first, "*") || strings.HasPrefix(first, "_") {
		for k := 0; k < len(body) && k < 8; k++ {
			t := strings.TrimSpace(body[k])
			if (k > 0 || len(t) > 1) && (strings.HasSuffix(t, "*") || strings.HasSuffix(t, "_")) {
				return k + 1
			}
		}
		return 1
	}
	n := 1
	for n < len(body) && isDisplay(body[n]) {
		n++
		// The sentence the formula sits inside goes on after it in lower case.
		if n < len(body) && startsLower(body[n]) && len(body[n]) < 200 {
			n++
		}
	}
	return n
}

func isDisplay(block string) bool {
	return placeholderRE.MatchString(block) && strings.TrimSpace(placeholderRE.ReplaceAllString(block, "")) == ""
}

func startsLower(s string) bool {
	for _, r := range strings.TrimSpace(s) {
		return r >= 'a' && r <= 'z'
	}
	return false
}

// trimItalic takes the italic marks off a statement's paragraph, since the card
// sets the statement in italic itself.
func trimItalic(p string) string {
	t := strings.TrimSpace(p)
	if strings.HasPrefix(t, "**") {
		return p
	}
	t = strings.TrimPrefix(strings.TrimPrefix(t, "*"), "_")
	if !strings.HasSuffix(t, "**") {
		t = strings.TrimSuffix(strings.TrimSuffix(t, "*"), "_")
	}
	return t
}

// blocks cuts a masked body into headings and paragraphs. A heading is its own
// block whether or not a blank line separates it from what follows, because the
// corpus is written with the blank line and a file that lost one should still
// come out as a heading rather than as a paragraph beginning with hashes.
func blocks(masked string) []string {
	var out []string
	var cur []string
	flush := func() {
		if len(cur) > 0 {
			out = append(out, strings.Join(cur, "\n"))
			cur = nil
		}
	}
	for _, line := range strings.Split(masked, "\n") {
		switch {
		case strings.TrimSpace(line) == "":
			flush()
		case strings.HasPrefix(line, "#"):
			flush()
			out = append(out, line)
		default:
			cur = append(cur, line)
		}
	}
	flush()
	return out
}

func (r Renderer) heading(line string) string {
	m := headingRE.FindStringSubmatch(line)
	if m == nil {
		return "<p>" + r.inline(line) + "</p>\n"
	}
	level := len(m[1])
	if level > 6 {
		level = 6
	}
	var attr string
	if m[3] != "" {
		attr = fmt.Sprintf(` id=%q`, m[3])
	} else if level == 3 {
		if id := subsecID(r.Section, m[2]); id != "" {
			attr = fmt.Sprintf(` id=%q`, id)
		}
	}
	if strings.Contains(m[4], ".statement") {
		attr += ` class="statement"`
	}
	words := strings.TrimSpace(m[2])
	if level <= 3 {
		words = sentenceIn(r.Lang, words)
	}
	text := r.inline(words)
	if tag := tagAttrRE.FindStringSubmatch(m[4]); tag != nil && r.Link != nil {
		text += fmt.Sprintf(` <a class="tag" href=%q title="permanent tag">%s</a>`,
			r.Link(tag[1]), tag[1])
	}
	return fmt.Sprintf("<h%d%s>%s</h%d>\n", level, attr, text, level)
}

var (
	boldRE = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	// italicRE is *text* or _text_, opened after a space or a bracket and with
	// no space inside either mark. The mathematics is masked by now, so an
	// underscore in x_1 is not in the text this sees.
	italicRE = regexp.MustCompile(`(^|[\s(\[{“"'«])[*_]([^\s*_](?:[^*_]*?[^\s*_])?)[*_]`)
	linkRE   = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
)

// inline renders the inside of a paragraph or a heading.
//
// Everything is escaped first and the three constructs the corpus uses are put
// back afterwards, which is the order that cannot leak markup: a stray < in the
// prose is a less-than sign and never the start of a tag. Emphasis with single
// asterisks is deliberately not read, since the mathematics is masked but the
// prose around it still writes * for a footnote in two places, and a construct
// the corpus does not use is a construct that can only misfire.
func (r Renderer) inline(s string) string {
	s = html.EscapeString(s)
	s = boldRE.ReplaceAllString(s, "<strong>$1</strong>")
	s = italicRE.ReplaceAllString(s, "$1<em>$2</em>")
	s = linkRE.ReplaceAllStringFunc(s, func(m string) string {
		p := linkRE.FindStringSubmatch(m)
		// The URL is already escaped, since the whole line was escaped before
		// this ran, so it is written between quotes rather than through %q.
		// Escaping it twice would turn the & of a query string into &amp;amp;,
		// and %q would escape a backslash in it the way Go writes a string
		// literal, which is not what a browser reads.
		// A link to a place on a page is a citation the graph resolved, and
		// it is set as one: quietly, so that the prose still reads as prose.
		if strings.Contains(p[2], "#") {
			return fmt.Sprintf(`<a class="ref" href="%s">%s</a>`, r.href(p[2]), p[1])
		}
		return fmt.Sprintf(`<a href="%s">%s</a>`, r.href(p[2]), p[1])
	})
	return s
}

// exerciseLinkRE is the corpus's own way of pointing at exercises, which is a
// path in the repository: exercises/s5/ for the set and exercises/s5/08.md for
// one of them.
var exerciseLinkRE = regexp.MustCompile(`^exercises/([sa]\d+)/(?:(\d+)\.md)?$`)

// href resolves a link written in the body.
//
// The corpus writes its own links relative to the directory the Markdown files
// sit in, which is the chapter: content/en/alg/VIII/01_s1.md links the
// exercises of § 1 as exercises/s1/. A section page is one level below the
// chapter on the site, so passing that through unchanged would point a level
// too deep, and it would still look like a link.
//
// The exercises are the one place where the repository layout and the site
// layout differ rather than nest. The corpus keeps them in a directory of the
// chapter, so that the link works when the file is read on GitHub, and the site
// hangs them off the § they belong to. Mapping the one to the other here is what
// lets both be true of the same line of Markdown.
func (r Renderer) href(url string) string {
	if r.Dir == "" || strings.HasPrefix(url, "/") || strings.Contains(url, "://") ||
		strings.HasPrefix(url, "#") {
		return url
	}
	if m := exerciseLinkRE.FindStringSubmatch(url); m != nil && r.Exercises != nil {
		n, _ := strconv.Atoi(m[2]) // empty means the whole set, which is zero
		if to := r.Exercises(m[1], n); to != "" {
			return to
		}
	}
	return r.Dir + url
}
