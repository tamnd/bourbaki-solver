package publish

import (
	"fmt"
	"html/template"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

// The reports, spec 12 §3: the generated reports of the corpus, rendered.
//
// They are the part of the corpus that says what is wrong with it. Which
// statements nothing cites, which references do not resolve, what the two
// printings disagree about, and the whole audit. A reader who has to clone two
// repositories and install Go to see any of that will not see any of it, which
// is the whole reason this milestone exists.
//
// Every committed reports/*.md is published rather than a list of four names
// written down here, because the day a fifth report lands it should appear
// without anybody remembering to add it.

// reportFile is one of them.
type reportFile struct {
	Name  string // the file name without .md, which is its URL
	Title string // its first heading
	Intro string // its first paragraph, for the index page
	Body  string
}

// reports reads the committed reports. A corpus with no reports directory is
// not an error: the site is buildable from a clone that has not run the tools.
func (s *Site) reports() ([]reportFile, error) {
	entries, err := os.ReadDir(filepath.Join(s.Root, "reports"))
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var out []reportFile
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".md") {
			continue
		}
		b, err := os.ReadFile(filepath.Join(s.Root, "reports", e.Name()))
		if err != nil {
			return nil, err
		}
		r := reportFile{Name: strings.TrimSuffix(e.Name(), ".md"), Body: string(b)}
		r.Title, r.Intro = titleAndIntro(r.Body)
		if r.Title == "" {
			r.Title = r.Name
		}
		out = append(out, r)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out, nil
}

// titleAndIntro is the first heading and the first paragraph under it, which is
// what every one of these files opens with and what the index page lists them
// by. Neither is guessed: a file that does not open that way contributes an
// empty string and the page prints the file name instead.
func titleAndIntro(md string) (string, string) {
	title, intro := "", ""
	for _, block := range strings.Split(md, "\n\n") {
		block = strings.TrimSpace(block)
		if block == "" {
			continue
		}
		if strings.HasPrefix(block, "# ") {
			title = strings.TrimSpace(strings.TrimPrefix(block, "# "))
			continue
		}
		if title != "" && !strings.HasPrefix(block, "#") {
			intro = strings.Join(strings.Split(block, "\n"), " ")
			break
		}
	}
	return title, intro
}

func (s *Site) reportIndex(rs []reportFile) (string, error) {
	var b strings.Builder
	b.WriteString("<h1>Reports</h1>\n")
	b.WriteString("<p>Generated out of the corpus by the tools and committed to it, so that each one is " +
		"regenerated and diffed on every change and none of them can quietly go stale. They are the part " +
		"of this that says what is wrong with it.</p>\n")
	if len(rs) == 0 {
		b.WriteString("<p class=\"none\">This clone holds no report. They are written by " +
			"<code>bourbaki refs build</code> and <code>bourbaki audit</code>.</p>\n")
	}
	b.WriteString("<ul class=\"toc\">\n")
	for _, r := range rs {
		fmt.Fprintf(&b, "<li><a href=%q>%s</a><br><span class=\"count\">%s</span></li>\n",
			s.url("reports", r.Name), template.HTMLEscapeString(r.Title),
			s.report(nil).inline(r.Intro))
	}
	b.WriteString("</ul>\n")
	return s.render(layout{Title: "Reports", Content: template.HTML(b.String())})
}

func (s *Site) reportPage(r reportFile, at map[string]string) (string, error) {
	body := s.report(at).html(r.Body)
	return s.render(layout{
		Title:   r.Title,
		Crumbs:  []crumb{{"Reports", s.url("reports")}},
		Content: template.HTML(body),
	})
}

// sitewide is where each tag and each label the reports name is on this site.
//
// It is what makes a report worth reading here rather than on GitHub: the
// orphans report is 297 tags and the point of it is to go and look at them, so
// every one of them is a link. Built once for all the reports, since the audit
// alone names a thousand labels.
func (s *Site) sitewide() map[string]string {
	at := map[string]string{}
	for _, st := range s.Statements {
		at[st.Tag] = s.TagURL(st.Tag)
		at[st.Label] = s.TagURL(st.Tag)
	}
	// English for preference, which is what byExLabel and byLabel already hold:
	// an exercise label is the same label in every language and the reports are
	// written in English.
	for label, ex := range s.byExLabel {
		at[label] = s.ExerciseURL(ex)
		if ex.Meta.Tag != "" {
			at[ex.Meta.Tag] = s.TagURL(ex.Meta.Tag)
		}
	}
	for label, sec := range s.byLabel {
		at[label] = s.SectionURL(sec)
	}
	return at
}

// report is the renderer for one report body.
//
// It is not the corpus renderer and it deliberately does not share code with
// it. The corpus is prose and mathematics and its renderer masks every formula
// before it reads a single Markdown construct, which is the only safe way to
// read a paragraph that is half TeX. A report is the other language: tables,
// bullets and file paths, no mathematics at all in any of the six committed
// today, and a renderer that reads tables would be a new way for a formula to
// be cut in half if it were ever pointed at the corpus. Two small readers, each
// certain of what it is reading, beat one that has to guess.
type report struct {
	site *Site
	// at is tag or label to URL on the site. Nil renders the same text with
	// nothing linked, which is what the index page wants.
	at map[string]string
}

func (s *Site) report(at map[string]string) report { return report{site: s, at: at} }

var (
	// A row of dashes under the head of a table, with or without the colons
	// that set the alignment.
	ruleCellRE = regexp.MustCompile(`^:?-{2,}:?$`)
	// A committed file, with the line the finding is on. The audit prints
	// hundreds of these and each one is a place somebody has to open.
	repoPathRE = regexp.MustCompile(`^([a-zA-Z0-9_./-]+\.(?:md|json|jsonl))(?::(\d+))?$`)
	bulletLine = regexp.MustCompile(`^\s*[-*+]\s+`)
)

// html renders a whole report.
func (r report) html(md string) string {
	var b strings.Builder
	for _, block := range strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n\n") {
		lines := trimBlank(strings.Split(block, "\n"))
		if len(lines) == 0 {
			continue
		}
		switch {
		case strings.HasPrefix(lines[0], "#"):
			b.WriteString(r.heading(lines[0]))
			// A heading and the paragraph under it with no blank line between
			// them is one block here and two on the page.
			if rest := trimBlank(lines[1:]); len(rest) > 0 {
				b.WriteString(r.block(rest))
			}
		default:
			b.WriteString(r.block(lines))
		}
	}
	return b.String()
}

func (r report) block(lines []string) string {
	switch {
	case strings.HasPrefix(lines[0], "|"):
		return r.table(lines)
	case bulletLine.MatchString(lines[0]):
		return r.list(lines)
	}
	return "<p>" + r.inline(strings.Join(lines, " ")) + "</p>\n"
}

func (r report) heading(line string) string {
	level := len(line) - len(strings.TrimLeft(line, "#"))
	if level > 6 {
		level = 6
	}
	text := strings.TrimSpace(strings.TrimLeft(line, "#"))
	// The h1 of the file is the h1 of the page and the rest step down with it.
	return fmt.Sprintf("<h%d>%s</h%d>\n", level, r.inline(text), level)
}

func (r report) list(lines []string) string {
	var b strings.Builder
	b.WriteString("<ul>\n")
	// A wrapped line belongs to the item above it. The reports do not wrap
	// today and a file that starts to should not come out as one item per line.
	var item []string
	flush := func() {
		if len(item) > 0 {
			fmt.Fprintf(&b, "<li>%s</li>\n", r.inline(strings.Join(item, " ")))
			item = nil
		}
	}
	for _, line := range lines {
		if bulletLine.MatchString(line) {
			flush()
			line = bulletLine.ReplaceAllString(line, "")
		}
		item = append(item, strings.TrimSpace(line))
	}
	flush()
	b.WriteString("</ul>\n")
	return b.String()
}

func (r report) table(lines []string) string {
	var rows [][]string
	var align []string
	for i, line := range lines {
		cells := splitRow(line)
		if i == 1 && isRule(cells) {
			align = alignment(cells)
			continue
		}
		rows = append(rows, cells)
	}
	var b strings.Builder
	b.WriteString("<table class=\"report\">\n")
	for i, row := range rows {
		b.WriteString("<tr>")
		for j, cell := range row {
			tag, right := "td", ""
			if i == 0 {
				tag = "th"
			}
			if j < len(align) && align[j] == "right" {
				right = ` class="num"`
			}
			fmt.Fprintf(&b, "<%s%s>%s</%s>", tag, right, r.cell(cell), tag)
		}
		b.WriteString("</tr>\n")
	}
	b.WriteString("</table>\n")
	return b.String()
}

// cell is one table cell. A cell that is nothing but a tag or a label the site
// holds becomes a link to it, which is what the reports are for: a list of 297
// tags nothing cites is a list to go and look at.
func (r report) cell(text string) string {
	body := r.inline(text)
	if url := r.at[strings.TrimSpace(text)]; url != "" {
		return fmt.Sprintf("<a href=%q>%s</a>", url, body)
	}
	return body
}

// inline renders the inside of a paragraph, a cell or an item.
//
// Escaped first and the constructs put back after, which is the order that
// cannot leak markup. Code spans are taken out before anything else looks at
// the text, so that a path or a fragment of TeX inside backticks cannot be read
// as emphasis.
func (r report) inline(text string) string {
	var b strings.Builder
	for i, part := range strings.Split(text, "`") {
		if i%2 == 1 {
			b.WriteString(r.code(part))
			continue
		}
		b.WriteString(boldRE.ReplaceAllString(template.HTMLEscapeString(part), "<strong>$1</strong>"))
	}
	return b.String()
}

// code is one backticked span. A committed file inside one becomes a link to
// that file at that line in the corpus repository, since an audit finding whose
// place a reader cannot open is a finding nobody acts on.
func (r report) code(text string) string {
	body := "<code>" + template.HTMLEscapeString(text) + "</code>"
	m := repoPathRE.FindStringSubmatch(strings.TrimSpace(text))
	if m == nil {
		return body
	}
	url := corpusRepo + "/blob/main/" + m[1]
	if m[2] != "" {
		url += "#L" + m[2]
	}
	return fmt.Sprintf("<a href=%q>%s</a>", url, body)
}

func splitRow(line string) []string {
	line = strings.TrimSpace(line)
	line = strings.TrimPrefix(line, "|")
	line = strings.TrimSuffix(line, "|")
	var out []string
	for _, cell := range strings.Split(line, "|") {
		out = append(out, strings.TrimSpace(cell))
	}
	return out
}

func isRule(cells []string) bool {
	for _, c := range cells {
		if !ruleCellRE.MatchString(c) {
			return false
		}
	}
	return len(cells) > 0
}

// alignment reads the colons off the rule row. Only the right hand one is used:
// every one of these tables is words on the left and counts on the right, and a
// count that is not right aligned is a column a reader cannot compare down.
func alignment(cells []string) []string {
	out := make([]string, len(cells))
	for i, c := range cells {
		if strings.HasSuffix(c, ":") && !strings.HasPrefix(c, ":") {
			out[i] = "right"
		}
	}
	return out
}

func trimBlank(lines []string) []string {
	var out []string
	for _, l := range lines {
		if strings.TrimSpace(l) != "" {
			out = append(out, l)
		}
	}
	return out
}
