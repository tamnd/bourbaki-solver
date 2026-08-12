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
		m, err := eng.Render(sp.Text, sp.Display)
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
	return r.unmask(b.String(), spans)
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
		// The URL is already escaped, since the whole line was escaped before
		// this ran, so it is written between quotes rather than through %q.
		// Escaping it twice would turn the & of a query string into &amp;amp;,
		// and %q would escape a backslash in it the way Go writes a string
		// literal, which is not what a browser reads.
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
