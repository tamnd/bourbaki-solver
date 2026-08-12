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
	"strings"

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
func mask(body string) (string, []mathtex.Span, error) {
	spans, unclosed := mathtex.Split(body)
	if unclosed != nil {
		return "", nil, fmt.Errorf("line %d: a math span opens and never closes", unclosed.Line)
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

// unmask puts the mathematics back, as markup rather than as source.
//
// The TeX is written out with its delimiters, which is not decoration: KaTeX is
// not run yet, so what a reader sees today is the source, and the source is
// what a reader can check against the printed page. When KaTeX lands it
// replaces the contents of these elements and the elements themselves do not
// move.
func unmask(s string, spans []mathtex.Span) string {
	return placeholderRE.ReplaceAllStringFunc(s, func(m string) string {
		var i int
		fmt.Sscanf(m, "\x00m%d\x00", &i)
		sp := spans[i]
		if sp.Display {
			return `<div class="math display">$$` + html.EscapeString(sp.Text) + `$$</div>`
		}
		return `<span class="math">$` + html.EscapeString(sp.Text) + `$</span>`
	})
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
}

// HTML renders one body.
func (r Renderer) HTML(body string) (string, error) {
	masked, spans, err := mask(body)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, block := range blocks(masked) {
		if strings.HasPrefix(block, "#") {
			b.WriteString(r.heading(block))
			continue
		}
		// A paragraph that is nothing but a display is the display, not a
		// paragraph with a display inside it. Wrapping it in <p> would put a
		// block element inside an inline context and no browser agrees about
		// what that means.
		if placeholderRE.MatchString(block) && strings.TrimSpace(placeholderRE.ReplaceAllString(block, "")) == "" {
			b.WriteString(r.inline(block) + "\n")
			continue
		}
		b.WriteString("<p>" + r.inline(strings.ReplaceAll(block, "\n", " ")) + "</p>\n")
	}
	return unmask(b.String(), spans), nil
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
	}
	if strings.Contains(m[4], ".statement") {
		attr += ` class="statement"`
	}
	text := r.inline(strings.TrimSpace(m[2]))
	if tag := tagAttrRE.FindStringSubmatch(m[4]); tag != nil && r.Link != nil {
		text += fmt.Sprintf(` <a class="tag" href=%q title="permanent tag">%s</a>`,
			r.Link(tag[1]), tag[1])
	}
	return fmt.Sprintf("<h%d%s>%s</h%d>\n", level, attr, text, level)
}

var (
	boldRE = regexp.MustCompile(`\*\*([^*]+)\*\*`)
	linkRE = regexp.MustCompile(`\[([^\]]+)\]\(([^)\s]+)\)`)
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
	s = linkRE.ReplaceAllStringFunc(s, func(m string) string {
		p := linkRE.FindStringSubmatch(m)
		return fmt.Sprintf(`<a href=%q>%s</a>`, r.href(p[2]), p[1])
	})
	return s
}

// href resolves a link written in the body.
//
// The corpus writes its own links relative to the directory the Markdown files
// sit in, which is the chapter: content/en/alg/VIII/01_s1.md links the
// exercises of § 1 as exercises/s1/. A section page is one level below the
// chapter on the site, so passing that through unchanged would point a level
// too deep, and it would still look like a link.
func (r Renderer) href(url string) string {
	if r.Dir == "" || strings.HasPrefix(url, "/") || strings.Contains(url, "://") ||
		strings.HasPrefix(url, "#") {
		return url
	}
	return r.Dir + url
}
