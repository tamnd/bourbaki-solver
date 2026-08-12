package quality

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
)

// The structure rules ask whether the corpus has the shape the book has: every
// § the table of contents names, once, in order, over a run of pages that meets
// the § before it and the § after it, with its exercises numbered the way they
// are printed.
//
// All nine are scoped to what has been extracted. A chapter with no content
// file is not a failure of these rules, it is work not yet done, and reporting
// sixty-two absent sections of chapters that nobody has read yet would bury the
// one section that really did go missing. What has been built and what has not
// is bourbaki report coverage, which is a different question and gets a
// different answer.

func init() {
	register(
		Check{ID: "S01", Group: Structure, Hard: true,
			Title: "every § of an extracted chapter has a file", Run: s01},
		Check{ID: "S02", Group: Structure, Hard: true,
			Title: "front matter parses against its schema", Run: s02},
		Check{ID: "S03", Group: Structure, Hard: true,
			Title: "§ numbers are contiguous within a chapter", Run: s03},
		Check{ID: "S04", Group: Structure, Hard: true,
			Title: "pdf page ranges tile the chapter without overlap", Run: s04},
		Check{ID: "S05", Group: Structure, Hard: true,
			Title: "the page map validates and its conflicts are bracketed", Run: s05},
		Check{ID: "S06", Group: Structure, Hard: true,
			Title: "no page was left at method ocr-failed", Run: s06},
		Check{ID: "S07", Group: Structure, Hard: true,
			Title: "exercise numbers run from 1 without a gap", Run: s07},
		Check{ID: "S08", Group: Structure, Hard: true,
			Title: "content_sha256 describes the body under it", Run: s08},
		Check{ID: "S09", Group: Structure, Hard: true,
			Title: "assembly is deterministic and what is committed is what it writes",
			Run:   s09, Need: needAssembly},
	)
}

// extracted is the chapters that have at least one content file, keyed
// book/chapter, which is the scope of S01, S03 and S04.
func (c *Corpus) extracted(lang string) map[string]bool {
	out := map[string]bool{}
	for _, d := range c.Docs {
		if d.Kind != KindSection || d.Lang != lang || d.Section == nil {
			continue
		}
		out[d.Section.Book+"/"+d.Section.Chapter] = true
	}
	return out
}

// sectionDocs are the § files of one chapter, in path order.
func (c *Corpus) sectionDocs(lang, book, chapter string) []Doc {
	var out []Doc
	for _, d := range c.Docs {
		if d.Kind == KindSection && d.Lang == lang && d.Section != nil &&
			d.Section.Book == book && d.Section.Chapter == chapter {
			out = append(out, d)
		}
	}
	return out
}

// S01. Every § the table of contents names has a file.
//
// The match is on the chapter, the number and whether it is an appendix, and
// not on the path, because a section file is named for its title and a title
// corrected in the table of contents would then look like a missing section
// rather than a renamed one.
func s01(c *Corpus) ([]Finding, error) {
	var out []Finding
	have := c.extracted("en")
	for _, bt := range c.TOC.Books {
		for _, ch := range bt.Chapters {
			key := ch.Book + "/" + ch.Numeral
			if !have[key] {
				continue
			}
			seen := map[string]Doc{}
			for _, d := range c.sectionDocs("en", ch.Book, ch.Numeral) {
				seen[sectionKey(d.Section.Section, d.Section.Appendix)] = d
			}
			for _, s := range ch.Sections {
				k := sectionKey(s.Number, s.Appendix)
				if _, ok := seen[k]; !ok {
					out = append(out, Finding{
						File: fmt.Sprintf("content/en/%s/%s", ch.Book, ch.Numeral),
						Msg: fmt.Sprintf("the table of contents has %s %q and the corpus has no file for it",
							sectionName(s.Number, s.Appendix), s.Title),
					})
				}
			}
		}
	}
	return out, nil
}

func sectionKey(n int, appendix bool) string {
	if appendix {
		return "a" + strconv.Itoa(n)
	}
	return "s" + strconv.Itoa(n)
}

func sectionName(n int, appendix bool) string {
	if appendix {
		if n == 0 {
			return "the Appendix"
		}
		return fmt.Sprintf("Appendix %d", n)
	}
	return fmt.Sprintf("§ %d", n)
}

// S02. Front matter parses, with unknown fields refused.
//
// The decoder is the schema. corpus.ParseFile sets KnownFields, so a key that
// is not in the struct is an error rather than something silently dropped, and
// that is the check: a file whose front matter carries translation_mdoel has a
// field nothing will ever read and a translation nothing will ever mark stale.
func s02(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		if d.Err != nil {
			out = append(out, Finding{File: d.Path, Line: 1, Msg: trimErr(d.Err)})
			continue
		}
		switch {
		case d.Section != nil:
			out = append(out, requireSection(d)...)
		case d.Exercise != nil:
			out = append(out, requireExercise(d)...)
		case d.Solution != nil:
			out = append(out, requireSolution(d)...)
		}
	}
	return out, nil
}

// trimErr drops the path the parser prefixes, since the finding carries it.
func trimErr(err error) string {
	s := err.Error()
	if i := strings.Index(s, ".md: "); i >= 0 {
		s = s[i+len(".md: "):]
	}
	return strings.ReplaceAll(s, "\n", "; ")
}

func requireSection(d Doc) []Finding {
	var out []Finding
	m := d.Section
	miss := func(field string) {
		out = append(out, Finding{File: d.Path, Line: 1, Msg: field + " is empty"})
	}
	if m.Book == "" {
		miss("book")
	}
	if m.Chapter == "" {
		miss("chapter")
	}
	if m.Lang == "" {
		miss("lang")
	}
	if m.Lang != "" && m.Lang != d.Lang {
		out = append(out, Finding{File: d.Path, Line: 1,
			Msg: fmt.Sprintf("lang is %q in a file under content/%s", m.Lang, d.Lang)})
	}
	if m.ContentSHA256 == "" {
		miss("content_sha256")
	}
	return out
}

func requireExercise(d Doc) []Finding {
	var out []Finding
	m := d.Exercise
	if m.Label == "" {
		out = append(out, Finding{File: d.Path, Line: 1, Msg: "label is empty"})
	} else if _, err := corpus.ParseLabel(m.Label); err != nil {
		out = append(out, Finding{File: d.Path, Line: 1,
			Msg: fmt.Sprintf("label %q does not parse: %v", m.Label, err)})
	}
	if m.Exercise == 0 {
		out = append(out, Finding{File: d.Path, Line: 1, Msg: "exercise number is 0"})
	}
	if m.Lang != "" && m.Lang != d.Lang {
		out = append(out, Finding{File: d.Path, Line: 1,
			Msg: fmt.Sprintf("lang is %q in a file under content/%s", m.Lang, d.Lang)})
	}
	return out
}

func requireSolution(d Doc) []Finding {
	var out []Finding
	m := d.Solution
	switch {
	case m.Status == "":
		out = append(out, Finding{File: d.Path, Line: 1, Msg: "status is empty"})
	case !corpus.ValidStatus(m.Status):
		out = append(out, Finding{File: d.Path, Line: 1,
			Msg: fmt.Sprintf("status %q is not one of %s", m.Status,
				strings.Join(corpus.Statuses, ", "))})
	}
	if m.Label == "" {
		out = append(out, Finding{File: d.Path, Line: 1, Msg: "label is empty"})
	}
	for i, p := range m.Parts {
		switch {
		case p.ID == "":
			out = append(out, Finding{File: d.Path, Line: 1,
				Msg: fmt.Sprintf("part %d has no id", i+1)})
		case !corpus.ValidStatus(p.Status):
			out = append(out, Finding{File: d.Path, Line: 1,
				Msg: fmt.Sprintf("part %s has status %q, which is not one of %s",
					p.ID, p.Status, strings.Join(corpus.Statuses, ", "))})
		}
	}
	return out
}

// S03. The § numbers of a chapter run from 1 without a gap.
//
// Appendices are counted apart from the §§. Chapter VIII has four numbered
// appendices and twenty-one §§, and folding them together would make § 22 look
// missing for ever.
func s03(c *Corpus) ([]Finding, error) {
	var out []Finding
	for key := range c.extracted("en") {
		book, chapter, _ := strings.Cut(key, "/")
		var secs, apps []int
		var where string
		for _, d := range c.sectionDocs("en", book, chapter) {
			where = filepath.ToSlash(filepath.Dir(d.Path))
			switch {
			case d.Section.Kind == corpus.KindFront || d.Section.Kind == corpus.KindHistorical:
			case d.Section.Appendix:
				apps = append(apps, d.Section.Section)
			default:
				secs = append(secs, d.Section.Section)
			}
		}
		for _, g := range gapsFromOne(secs) {
			out = append(out, Finding{File: where,
				Msg: fmt.Sprintf("chapter %s has no § %d, and the §§ it has run 1 to %d", chapter, g, maxOf(secs))})
		}
		// An unnumbered appendix is recorded as 0, which chapters II and III
		// both have, so a run of appendices starting at 0 is not a gap.
		if len(apps) > 0 && minOf(apps) > 0 {
			for _, g := range gapsFromOne(apps) {
				out = append(out, Finding{File: where,
					Msg: fmt.Sprintf("chapter %s has no Appendix %d", chapter, g)})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Msg < out[j].Msg })
	return out, nil
}

// gapsFromOne is the numbers missing between 1 and the largest.
func gapsFromOne(ns []int) []int {
	if len(ns) == 0 {
		return nil
	}
	have := map[int]bool{}
	for _, n := range ns {
		have[n] = true
	}
	var out []int
	for i := 1; i <= maxOf(ns); i++ {
		if !have[i] {
			out = append(out, i)
		}
	}
	return out
}

func maxOf(ns []int) int {
	m := ns[0]
	for _, n := range ns {
		if n > m {
			m = n
		}
	}
	return m
}

func minOf(ns []int) int {
	m := ns[0]
	for _, n := range ns {
		if n < m {
			m = n
		}
	}
	return m
}

// S04. The pdf page ranges of a chapter tile it: each § begins where the last
// one ended, and no page is in two files.
//
// This is the rule that catches a lost page. A § that ends at 40 followed by
// one that begins at 42 means page 41 was extracted, committed, and assembled
// into nothing, and there is no other check in the corpus that would notice: the
// page file is there, the section files are there, and every one of them parses.
//
// Two things the printing does are not that, and both were reported as faults by
// the first draft of this rule before the corpus was looked at.
//
// A blank leaf is in no section because there is nothing on it to be in one.
// Chapter VIII opens every § on a recto, so eleven of its verso pages are blank,
// and every one of them fell in a gap. The gap is only a finding when something
// was printed on the missing pages.
//
// A § that begins part way down a page shares that page with what came before,
// so the two files really do both cite it. Page 18 carries the chapter opening
// and the first words of § 1. An overlap of the one boundary page is the book
// being laid out; an overlap that reaches further back is a split that came
// apart, and that is what is reported.
func s04(c *Corpus) ([]Finding, error) {
	type span struct {
		doc         Doc
		first, last int
	}
	var out []Finding
	for key := range c.extracted("en") {
		book, chapter, _ := strings.Cut(key, "/")
		var spans []span
		for _, d := range c.sectionDocs("en", book, chapter) {
			first, last, err := pdfRange(d.Section.PDFPages)
			if err != nil {
				out = append(out, Finding{File: d.Path, Line: 1,
					Msg: fmt.Sprintf("pdf_pages %q: %v", d.Section.PDFPages, err)})
				continue
			}
			if first > last {
				out = append(out, Finding{File: d.Path, Line: 1,
					Msg: fmt.Sprintf("pdf_pages %q runs backwards", d.Section.PDFPages)})
				continue
			}
			spans = append(spans, span{d, first, last})
		}
		sort.Slice(spans, func(i, j int) bool { return spans[i].first < spans[j].first })
		for i := 1; i < len(spans); i++ {
			prev, cur := spans[i-1], spans[i]
			switch {
			case cur.first < prev.last:
				out = append(out, Finding{File: cur.doc.Path, Line: 1,
					Msg: fmt.Sprintf("pdf pages %d to %d are also in %s",
						cur.first, minOf([]int{cur.last, prev.last}), prev.doc.Path)})
			case cur.first > prev.last+1:
				lost := c.printedPages(cur.doc.Section.Source, prev.last+1, cur.first-1)
				if len(lost) == 0 {
					continue
				}
				out = append(out, Finding{File: cur.doc.Path, Line: 1,
					Msg: fmt.Sprintf("pdf %s in no section file, between %s and this one",
						pageList(lost), prev.doc.Path)})
			}
		}
	}
	return out, nil
}

// printedPages are the pages of book between first and last, inclusive, that
// have something printed on them. A page that was never extracted counts: a
// section range that skips a page nobody has read is still a hole in the
// chapter, and saying so is the only way it gets read.
func (c *Corpus) printedPages(book string, first, last int) []int {
	blank := map[int]bool{}
	for _, p := range c.Pages[book] {
		if p.Meta.Method == corpus.MethodBlank || strings.TrimSpace(p.Body) == "" {
			blank[p.Meta.PDFPage] = true
		}
	}
	var out []int
	for p := first; p <= last; p++ {
		if !blank[p] {
			out = append(out, p)
		}
	}
	return out
}

// pageList writes a run of pages the way somebody would say it out loud.
func pageList(ps []int) string {
	if len(ps) == 1 {
		return fmt.Sprintf("page %d is", ps[0])
	}
	if ps[len(ps)-1]-ps[0]+1 == len(ps) {
		return fmt.Sprintf("pages %d to %d are", ps[0], ps[len(ps)-1])
	}
	var s []string
	for _, p := range ps {
		s = append(s, strconv.Itoa(p))
	}
	return "pages " + strings.Join(s, ", ") + " are"
}

// pdfRange reads the "0001-0016" that assembly writes.
func pdfRange(s string) (int, int, error) {
	if s == "" {
		return 0, 0, fmt.Errorf("empty")
	}
	a, b, ok := strings.Cut(s, "-")
	if !ok {
		return 0, 0, fmt.Errorf("not a range")
	}
	first, err := strconv.Atoi(a)
	if err != nil {
		return 0, 0, err
	}
	last, err := strconv.Atoi(b)
	if err != nil {
		return 0, 0, err
	}
	return first, last, nil
}

// S05. The page map validates, and every conflict it published is bracketed.
//
// pagemap.Validate is the arithmetic: a printed page sequence is constrained
// enough that most ways of getting it wrong show up as an inconsistency rather
// than as a plausible wrong number.
//
// The bracketing is the other half. A conflict is a page whose printed number
// was legible and disagrees with the offset fitted from its neighbours, and the
// map publishes every one rather than dropping it. One conflict between two
// pages that were read cleanly is an OCR misread of a single page number and
// the fit is right. A conflict whose neighbour on either side was not read
// cleanly is a conflict with nothing holding it, and then it is the fit that is
// in doubt and not the page.
func s05(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, b := range c.Books.Books {
		m, ok := c.Maps[b.ID]
		if !ok {
			continue
		}
		rel := relPath(c.Root, pagemap.Path(c.Root, b.ID))
		for _, p := range m.Validate() {
			out = append(out, Finding{File: rel, Msg: p.String()})
		}
		conflicted := map[int]bool{}
		for _, cf := range m.Conflicts {
			conflicted[cf.PDFPage] = true
		}
		for _, cf := range m.Conflicts {
			before, after := cleanNeighbour(m, cf.PDFPage, -1, conflicted), cleanNeighbour(m, cf.PDFPage, +1, conflicted)
			if before == 0 || after == 0 {
				side := "after"
				if before == 0 {
					side = "before"
				}
				out = append(out, Finding{File: rel,
					Msg: fmt.Sprintf("%s, and no page %s it was read cleanly, so nothing brackets it", cf, side)})
			}
		}
	}
	return out, nil
}

// cleanNeighbour is the nearest page in the given direction that was read off
// the printed page and is not itself in conflict, within the same chapter, or
// 0 when there is none.
func cleanNeighbour(m *pagemap.Map, at, step int, conflicted map[int]bool) int {
	var here pagemap.Entry
	for _, e := range m.Entries {
		if e.PDFPage == at {
			here = e
		}
	}
	for p := at + step; p >= 1 && p <= m.PDFPages; p += step {
		for _, e := range m.Entries {
			if e.PDFPage != p {
				continue
			}
			if e.Chapter != here.Chapter {
				return 0
			}
			if e.Confidence.Printed() && !conflicted[p] {
				return p
			}
		}
	}
	return 0
}

// S06. No page was left at ocr-failed.
//
// A page that the model could not read is meant to go back on the queue, not
// into the corpus. One that is committed with this method assembles into a
// section file like any other and reads as a gap in the mathematics that
// nothing else flags.
func s06(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, b := range c.Books.Books {
		for i, p := range c.Pages[b.ID] {
			if string(p.Meta.Method) == "ocr-failed" {
				out = append(out, Finding{File: c.PagePaths[b.ID][i], Line: 1,
					Msg: fmt.Sprintf("pdf page %d is committed as ocr-failed", p.Meta.PDFPage)})
			}
		}
	}
	return out, nil
}

// S07. The exercises of a § are numbered from 1 without a gap.
//
// Bourbaki numbers them straight through, so 1 to 12 and then 14 is not
// something the book does. It is a page that never got read or a split that
// came apart, and across three hundred exercises it is the one thing nobody
// spots by eye.
func s07(c *Corpus) ([]Finding, error) {
	bySection := map[string][]int{}
	where := map[string]string{}
	for _, d := range c.Docs {
		if d.Kind != KindExercise || d.Exercise == nil || d.Lang != "en" {
			continue
		}
		key := fmt.Sprintf("%s/%s/%s", d.Exercise.Book, d.Exercise.Chapter,
			corpus.ExerciseDir(d.Exercise.Section, d.Exercise.Appendix))
		bySection[key] = append(bySection[key], d.Exercise.Exercise)
		where[key] = filepath.ToSlash(filepath.Dir(d.Path))
	}
	var out []Finding
	for _, key := range sortedStrings(bySection) {
		ns := bySection[key]
		sort.Ints(ns)
		if ns[0] != 1 {
			out = append(out, Finding{File: where[key],
				Msg: fmt.Sprintf("the exercises start at %d rather than 1", ns[0])})
		}
		if gaps := corpus.Gaps(ns); len(gaps) > 0 {
			out = append(out, Finding{File: where[key],
				Msg: fmt.Sprintf("the exercises run %d to %d and %v are missing",
					ns[0], ns[len(ns)-1], gaps)})
		}
	}
	return out, nil
}

func sortedStrings[V any](m map[string]V) []string {
	out := make([]string, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// S08. content_sha256 describes the body under it.
//
// This is what makes the corpus maintainable rather than merely present. A
// translation records the English hash it was made from, so when the English
// changes the stale translations are exactly the ones whose recorded hash no
// longer matches. That inference is worth nothing if the English file's own
// hash does not describe the English file.
func s08(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		if d.Section == nil || d.Section.ContentSHA256 == "" {
			continue
		}
		if want := corpus.ContentSHA256(d.Body); want != d.Section.ContentSHA256 {
			out = append(out, Finding{File: d.Path, Line: 1,
				Msg: fmt.Sprintf("content_sha256 is %s and the body hashes to %s",
					short(d.Section.ContentSHA256), short(want))})
		}
	}
	return out, nil
}

func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	return sha
}

// needAssembly says why S09 cannot run.
func needAssembly(c *Corpus) string {
	if c.Opt.Assembled == nil {
		if c.Opt.AssembleErr != "" {
			return c.Opt.AssembleErr
		}
		return "the assembler was not run, so there is nothing to compare against"
	}
	return ""
}

// S09. Assembly is a pure function of the pages and the table of contents, and
// what is committed is what it writes.
//
// This is the rule the whole corpus rests on. Everything else here reads the
// Markdown and trusts it; this one asks whether the Markdown is what the stated
// inputs produce. A section file edited by hand passes every other rule in this
// package and fails this one, which is the point: the corpus is a build output
// and a build output somebody has touched is not reproducible.
func s09(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, path := range sortedStrings(c.Opt.Assembled) {
		want := c.Opt.Assembled[path]
		have, err := os.ReadFile(path)
		if os.IsNotExist(err) {
			out = append(out, Finding{File: relPath(c.Root, path),
				Msg: "assembly writes this file and the corpus does not have it"})
			continue
		}
		if err != nil {
			return nil, err
		}
		if !bytes.Equal(have, want) {
			out = append(out, Finding{File: relPath(c.Root, path),
				Msg: "what is committed is not what assembly writes"})
		}
	}
	for _, path := range c.Opt.Stale {
		out = append(out, Finding{File: relPath(c.Root, path),
			Msg: "no page assembles into this file, so it is left over from an earlier split"})
	}
	return out, nil
}

func relPath(root, path string) string {
	if r, err := filepath.Rel(root, path); err == nil {
		return filepath.ToSlash(r)
	}
	return filepath.ToSlash(path)
}
