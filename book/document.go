package book

import (
	_ "embed"
	"fmt"
	"path"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// Class is bourbaki.cls, carried in the binary rather than looked for on disk.
// The class and the writer below have to agree about every macro name and every
// argument order, and a class file sitting in a directory somewhere is a class
// file that can be a version behind the program that writes for it.
//
//go:embed assets/bourbaki.cls
var Class string

// A Document is one volume written out as LaTeX, with everything the build
// wants to say about it.
type Document struct {
	TeX string
	// Missing and Stray are what the renderer could not set: the characters no
	// font in the build covers, and the control sequences in prose that the
	// table does not know. Both are counted per file, because a report that says
	// there are 239 Devanagari characters somewhere in eight thousand files is a
	// report nobody can act on.
	Missing []Finding
	Stray   []Finding
	// Rescued is the mathematics the corpus left loose in its prose, x_i and
	// S^{-1}A and the rest, which the build wrapped in dollars so the page would
	// read. It is a count of a defect in content/ and not of one in the build.
	Rescued []Finding
	// Wide is every array whose preamble was narrower than its widest row. The
	// build widens it and carries on, and this is the list of pages somebody
	// should go and look at, because a diagram that lost a column in the reading
	// has usually lost an arrow with it.
	Wide []Finding
	// Anchors is every label the document defines, and Dangling every reference
	// it makes to one it does not. A book that quietly drops a cross reference
	// reads fine and is wrong, so both are counted.
	Anchors  int
	Dangling []Finding
	// Files and Exercises are what went in, for the audit to check against the
	// manifest.
	Files, Exercises int
	// Aligned is every display the corpus wrote over several lines and the build
	// set as a calculation aligned on the relation. It is a decision the build
	// made and not a fault, and it is listed so that somebody comparing a page
	// against the printing knows which formulas to look at first.
	Aligned []Finding
}

// A Finding is one thing the build could not do, and where.
type Finding struct {
	Where string // content/en/alg/I/03_s3_actions.md:118
	What  string // the character, or the control sequence
	Count int
}

// Write turns a volume into a LaTeX document.
//
// It reads the volume twice. The first pass collects every anchor the corpus
// defines, so that the second can tell a cross reference that will resolve from
// one that will not, and print a page number for the first and plain text for
// the second. A one-pass writer would have to emit \ref for everything and let
// LaTeX print a row of question marks, which is how a book ships with two
// hundred broken references and nobody notices.
func Write(v *Volume) (*Document, error) {
	d := &Document{}
	anchors := collect(v)
	d.Anchors = len(anchors)

	var b strings.Builder
	fmt.Fprintf(&b, "%% built from content/%s by bourbaki book. Do not edit.\n", v.Lang)
	b.WriteString("\\documentclass{bourbaki}\n")
	// One page for the whole shelf. \bpaper used to take the trim the manifest
	// records for this volume's printing; the class sets Springer's monograph
	// page instead and the reason is written beside it there.
	b.WriteString("\\bpaper\n")
	fmt.Fprintf(&b, "\\blanguage{%s}\n", v.Lang)
	fmt.Fprintf(&b, "\\btitle{%s}\n", escapeTeX(v.Title))
	fmt.Fprintf(&b, "\\bspan{%s}\n", escapeTeX(v.ChapterSpan()))
	if v.Meta.Edition != "" {
		fmt.Fprintf(&b, "\\bedition{%s}\n", escapeTeX(v.Meta.Edition))
	}
	b.WriteString("\\begin{document}\n\\bcover\n\\btitlepage\n\\frontmatter\n\\bcontents\n")

	// The note to the reader and the Book's own introduction stand before
	// chapter I and belong to no chapter, which is where the printing puts them
	// and why they are written into the front matter here rather than being
	// made into chapters of their own. The note comes first, as it is printed.
	for _, f := range []struct {
		sec      *Section
		fallback string
		anchor   string
	}{
		{v.Reader, "To the Reader", "reader"},
		{v.Intro, "Introduction", "intro"},
	} {
		if f.sec == nil {
			continue
		}
		title := f.sec.Title
		if title == "" {
			title = f.fallback
		}
		fmt.Fprintf(&b, "\n\\bunnumbered{%s}{%s}{%s}\n", escapeTeX(title), escapeTeX(listed(title, v.Lang)), f.anchor)
		tex, err := d.body(v, f.sec, anchors)
		if err != nil {
			return nil, err
		}
		b.WriteString(tex)
	}

	b.WriteString("\\mainmatter\n")
	for _, c := range v.Chapters {
		if c.Front == nil && len(c.Sections) == 0 {
			continue
		}
		if err := d.chapter(&b, v, c, anchors); err != nil {
			return nil, err
		}
	}
	b.WriteString("\\end{document}\n")
	d.TeX = b.String()
	return d, nil
}

// collect is every anchor the volume defines, which is what a cross reference
// can point at.
func collect(v *Volume) map[string]bool {
	out := map[string]bool{}
	for _, s := range v.Pieces() {
		if s.Label != "" {
			out[s.Label] = true
		}
		for _, m := range headingAnchorRE.FindAllStringSubmatch(s.Body, -1) {
			out[m[1]] = true
		}
		for _, e := range s.Exercises {
			if e.Label != "" {
				out[e.Label] = true
			}
		}
	}
	return out
}

var headingAnchorRE = regexp.MustCompile(`(?m)^#{1,6}[^\n]*\{#([a-z0-9-]+)`)

func (d *Document) chapter(b *strings.Builder, v *Volume, c *Chapter, anchors map[string]bool) error {
	title := c.Title
	if title == "" {
		title = c.Numeral
	}
	fmt.Fprintf(b, "\n\\bchapter{%s}{%s}{%s}{ch-%s}\n", escapeTeX(c.Numeral), escapeTeX(title),
		escapeTeX(listed(title, v.Lang)), strings.ToLower(c.Numeral))
	if c.Front != nil {
		tex, err := d.body(v, c.Front, anchors)
		if err != nil {
			return err
		}
		b.WriteString(tex)
	}
	for _, s := range c.Sections {
		label := s.Label
		if label == "" {
			label = fmt.Sprintf("ch-%s-s%d", strings.ToLower(c.Numeral), s.Number)
		}
		fmt.Fprintf(b, "\n\\bsection{%s}{%s}{%s}\n", escapeTeX(s.Heading()), escapeTeX(s.Title), label)
		tex, err := d.body(v, s, anchors)
		if err != nil {
			return err
		}
		b.WriteString(tex)
	}
	if err := d.exercises(b, v, c, anchors); err != nil {
		return err
	}
	if c.Historical != nil {
		title, list := c.Historical.Title, ""
		if title == "" {
			title, list = `\bhistoricalname`, `\bhistoricalname`
		} else {
			title, list = escapeTeX(title), escapeTeX(listed(title, v.Lang))
		}
		fmt.Fprintf(b, "\n\\bunnumbered{%s}{%s}{hist-%s}\n", title, list, strings.ToLower(c.Numeral))
		tex, err := d.body(v, c.Historical, anchors)
		if err != nil {
			return err
		}
		b.WriteString(tex)
	}
	return nil
}

// exercises writes the exercises of a whole chapter, gathered at its end under
// one head with the §§ named inside it.
//
// The corpus files them one to an exercise under the § they belong to, which
// leaves the arrangement open, and the printing settles it: page 123 of the
// English Algebra I to III ends § 10 of chapter I, page 124 opens EXERCISES, and
// the runs for § 1 to § 10 follow one another under it. A § with no exercises in
// the corpus is skipped rather than given an empty head, so a partly translated
// volume says nothing about the §§ it has not reached.
func (d *Document) exercises(b *strings.Builder, v *Volume, c *Chapter, anchors map[string]bool) error {
	any := false
	for _, s := range c.Sections {
		if len(s.Exercises) == 0 {
			continue
		}
		if !any {
			b.WriteString("\n\\bexercises\n")
			any = true
		}
		label := s.Label
		if label == "" {
			label = fmt.Sprintf("ch-%s-s%d", strings.ToLower(c.Numeral), s.Number)
		}
		fmt.Fprintf(b, "\n\\bexercisesfor{%s}{%s-ex}\n", escapeTeX(s.Heading()), label)
		for _, e := range s.Exercises {
			star := ""
			if e.Starred {
				star = "*"
			}
			exLabel := e.Label
			if exLabel == "" {
				exLabel = fmt.Sprintf("%s-ex-%d", label, e.Number)
			}
			fmt.Fprintf(b, "\\bexercise{%d}{%s}{%s}\n", e.Number, star, exLabel)
			r := d.renderer(v, e.Path, e.Head, anchors)
			tex, err := r.TeX(StripTitle(e.Body))
			if err != nil {
				return err
			}
			b.WriteString(tex)
			d.Exercises++
		}
	}
	return nil
}

func (d *Document) body(v *Volume, s *Section, anchors map[string]bool) (string, error) {
	r := d.renderer(v, s.Path, s.Head, anchors)
	r.Contents = s.Contents
	tex, err := r.TeX(StripExercisePointer(StripTitle(s.Body)))
	if err != nil {
		return "", err
	}
	d.Files++
	return tex, nil
}

// renderer wires one file's renderer to the document's counters.
func (d *Document) renderer(v *Volume, file string, head int, anchors map[string]bool) Renderer {
	return Renderer{
		File: file, Line: head, Lang: v.Lang,
		Label: func(anchor string) string { return anchor },
		Ref: func(url, text string) string {
			return reference(url, text, anchors, func(f Finding) {
				d.Dangling = append(d.Dangling, f)
			}, file)
		},
		Missing: func(where string, rs []rune) {
			for _, x := range rs {
				d.Missing = append(d.Missing, Finding{Where: where, What: string(x), Count: 1})
			}
		},
		Stray: func(where string, cs []string) {
			for _, c := range cs {
				d.Stray = append(d.Stray, Finding{Where: where, What: c, Count: 1})
			}
		},
		Rescued: func(where string, as []string) {
			for _, a := range as {
				d.Rescued = append(d.Rescued, Finding{Where: where, What: a, Count: 1})
			}
		},
		Aligned: func(where string) {
			d.Aligned = append(d.Aligned, Finding{Where: where, What: "display", Count: 1})
		},
		Wide: func(where string, a wideArray) {
			d.Wide = append(d.Wide, Finding{Where: where, What: a.String(), Count: 1})
		},
	}
}

// reference turns a corpus link into something a printed page can use.
//
// The corpus writes its links as site paths, ../s2/ and ../../II/03_s3.md and
// exercises/s1/, because the site is what they were written for. In a book a
// path is no use to anybody, so a link whose target is in this volume becomes
// the text and the page it is on, and a link whose target is not becomes the
// text alone. Neither one is a row of question marks, which is what a \ref to a
// label that was never defined prints.
func reference(url, text string, anchors map[string]bool, note func(Finding), file string) string {
	anchor := anchorOf(url)
	if anchor != "" && anchors[anchor] {
		return `\hyperref[` + anchor + `]{` + text + `} (p.~\pageref{` + anchor + `})`
	}
	if strings.HasPrefix(url, "http://") || strings.HasPrefix(url, "https://") {
		// A link out of the corpus stays a link, since a printed URL is at least
		// something a reader can type.
		return `\href{` + url + `}{` + text + `}`
	}
	note(Finding{Where: file, What: url, Count: 1})
	return text
}

// anchorOf reads the anchor out of a corpus link. The corpus writes three
// shapes, "#alg-i-s1-def-1" on its own, a path with one on the end, and a bare
// path to a file, and only the first two name something a book can point at.
func anchorOf(url string) string {
	if i := strings.IndexByte(url, '#'); i >= 0 {
		return url[i+1:]
	}
	return ""
}

// Summary is what the build says about the volume when it is done, one line per
// thing that went wrong and a count of what went right. It is written to the
// terminal and to the audit report, and it is deliberately the same text in
// both, so that a person reading the report and a person watching the build are
// arguing about one set of numbers.
func (d *Document) Summary(v *Volume) string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s: %d files, %d exercises, %d anchors, %d bytes of TeX\n",
		v.ID(), d.Files, d.Exercises, d.Anchors, len(d.TeX))
	report := func(name string, f []Finding) {
		if len(f) == 0 {
			return
		}
		by := map[string]int{}
		at := map[string]string{}
		for _, x := range f {
			by[x.What] += x.Count
			if at[x.What] == "" {
				at[x.What] = x.Where
			}
		}
		keys := make([]string, 0, len(by))
		for k := range by {
			keys = append(keys, k)
		}
		sort.Slice(keys, func(i, j int) bool {
			if by[keys[i]] != by[keys[j]] {
				return by[keys[i]] > by[keys[j]]
			}
			return keys[i] < keys[j]
		})
		fmt.Fprintf(&b, "  %s: %d distinct, %d in all\n", name, len(keys), len(f))
		for i, k := range keys {
			if i >= 12 {
				fmt.Fprintf(&b, "    and %d more\n", len(keys)-12)
				break
			}
			fmt.Fprintf(&b, "    %6d  %-12s %s\n", by[k], quoteWhat(k), at[k])
		}
	}
	report("characters no font can set", d.Missing)
	report("control sequences in prose", d.Stray)
	report("mathematics rescued from prose", d.Rescued)
	report("references with no target", d.Dangling)
	report("arrays widened to fit their own rows", d.Wide)
	return b.String()
}

func quoteWhat(s string) string {
	if len(s) <= 8 {
		return strconv.Quote(s)
	}
	return strconv.Quote(s[:8]) + "..."
}

// Coverage says which of the volume's sections the language has not got yet,
// counted against the sections manifest of the printing.
//
// It is separate from Write because a build of a language that is half done is
// a useful thing to look at and a bad thing to call finished. The build makes
// the PDF either way and the number goes in the audit, so that a Vietnamese
// Algebra at eighty per cent is a Vietnamese Algebra at eighty per cent and not
// a Vietnamese Algebra.
func Coverage(root string, v *Volume) (have, want int, missing []string, err error) {
	m, err := corpus.LoadSections(root)
	if err != nil {
		return 0, 0, nil, err
	}
	// The sections manifest is keyed by printing, and the printing is the
	// English or French one the pages came out of. A Vietnamese build of the
	// same volume is counted against that same list, which is the point: the
	// question is how much of the book exists in this language, and the book is
	// what the printing has.
	bs, ok := m.Get(v.Meta.ID)
	if !ok {
		return 0, 0, nil, fmt.Errorf("no sections recorded for %s", v.Meta.ID)
	}
	got := map[string]bool{}
	for _, s := range v.Pieces() {
		got[path.Base(s.Path)] = true
	}
	count := func(r corpus.SectionRecord) {
		want++
		if got[path.Base(r.Path)] {
			have++
			return
		}
		missing = append(missing, r.Path)
	}
	if bs.ReaderNote != nil {
		count(*bs.ReaderNote)
	}
	if bs.Introduction != nil {
		count(*bs.Introduction)
	}
	for _, c := range bs.Chapters {
		for _, s := range c.Sections {
			count(s)
		}
	}
	return have, want, missing, nil
}
