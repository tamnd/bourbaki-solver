package publish

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/refs"
	tagset "github.com/tamnd/bourbaki-solver/tags"
)

// Site is the whole corpus in memory, in every language it is held in.
type Site struct {
	Root string
	Base string // the path the site is served under, "" or "/bourbaki"

	// Langs are the languages that have any content at all, English first and
	// the rest in name order. Which of them the navigation offers is a separate
	// question and is answered by Draft.
	Langs []string
	// Draft is a language under the coverage floor. It is built and it is
	// reachable, and it is kept out of the language switcher, because a
	// switcher that offers a language and then gives a reader a dead end
	// twenty five times out of twenty seven is worse than one that does not
	// offer it.
	Draft map[string]bool
	// Drafts offers the draft languages in the switcher anyway. It is the -drafts
	// flag and it is for looking at a translation in place while it is being
	// written. The deploy never sets it.
	Drafts bool

	Sections   []*Section
	Statements []*Statement
	Exercises  []*Exercise

	byTag map[string]*Statement
	// byExTag and byExLabel are the exercises, which carry tags of their own and
	// are not statements. 317 of the corpus's tags are on an exercise, so a tag
	// table that could not follow them was a third of the table saying "no page
	// yet" about files that were sitting in the repository.
	byExTag   map[string]*Exercise
	byExLabel map[string]*Exercise
	// byLabel is the § a section label names, English for preference. A
	// reference is as often to a whole § as to a statement in one, and a § has
	// no tag, so this is what those rows are linked through.
	byLabel map[string]*Section

	CitedBy map[string][]*Edge
	Cites   map[string][]*Edge

	// Unresolved is how many references the graph could not follow, which the
	// about page publishes rather than hides.
	Unresolved int

	// AllowBrokenMath builds a site with the formulae KaTeX refuses marked on
	// the page instead of failing. It is off by default and the deploy never
	// turns it on: spec 12 §5 makes a refused span a hard failure, because a
	// site that quietly prints the TeX of a formula the extraction got wrong
	// puts the fault in front of a reader in a form that looks deliberate.
	// Marked is not quiet, and it is what makes the other twenty thousand
	// formulae reviewable while the broken ones wait on the printed page.
	AllowBrokenMath bool
	// Broken is what that produced, in the order it was found.
	Broken []Refusal
}

// Section is one file of content/<lang>/.
type Section struct {
	Lang string
	Meta corpus.SectionFrontMatter
	Body string
	Path string // repo-relative, so the page can link at GitHub
	Slug string // front, historical-note, s1, a4
	// Head is the file line the body's first line sits on, which is the front
	// matter plus the blank line under it. It is carried so that a formula the
	// renderer refuses is reported at a line somebody can open.
	Head  int
	Units []Unit
}

// Statement is one tagged unit of the book, in every language that holds it.
type Statement struct {
	Tag   string
	Label string
	// Page is the page of the chapter the book prints it on, 0 when the index
	// could not place it. It is what makes a tag page citable against paper:
	// the reader who has the volume open wants VIII, p. 378 and not a URL.
	Page int
	// Sections and Units are keyed by language. A statement is in the corpus in
	// English and in whatever else has reached it, and the tag is the statement
	// rather than any one printing of it.
	Sections map[string]*Section
	Units    map[string]Unit
}

// Edge is one reference, from the graph, kept only in the shape the pages need.
type Edge struct {
	FromTag   string
	FromLabel string
	ToTag     string
	ToLabel   string
	Raw       string
	How       string
	Book      string
	Chapter   string
}

// Load reads everything the site is built from. Nothing here opens a PDF or
// touches work/, which is what lets the site be built from a public clone.
func Load(root string) (*Site, error) { return LoadOnly(root, nil) }

// LoadOnly is Load restricted to the languages named, in the order Load would
// have put them. An empty list is every language content/ holds.
//
// The restriction is for a person looking at one language while working on it,
// so it fails on a language the corpus does not have rather than quietly
// building the rest: a typed -lang vn is a build of English alone that looks
// like a build of Vietnamese that came out empty.
func LoadOnly(root string, only []string) (*Site, error) {
	s := &Site{Root: root, byTag: map[string]*Statement{}, byLabel: map[string]*Section{},
		byExTag: map[string]*Exercise{}, byExLabel: map[string]*Exercise{},
		CitedBy: map[string][]*Edge{}, Cites: map[string][]*Edge{}, Draft: map[string]bool{}}

	langs, err := languages(root)
	if err != nil {
		return nil, err
	}
	if len(only) > 0 {
		langs, err = keep(langs, only)
		if err != nil {
			return nil, err
		}
	}
	s.Langs = langs
	for _, lang := range langs {
		secs, err := loadLang(root, lang)
		if err != nil {
			return nil, err
		}
		s.Sections = append(s.Sections, secs...)
		exs, err := loadExercises(root, lang)
		if err != nil {
			return nil, err
		}
		s.Exercises = append(s.Exercises, exs...)
	}
	sols, err := loadSolutions(root)
	if err != nil {
		return nil, err
	}
	for _, ex := range s.Exercises {
		ex.Solution = sols[ex.Lang+"\x00"+ex.Meta.Label]
	}
	if err := s.index(); err != nil {
		return nil, err
	}

	if err := s.floor(); err != nil {
		return nil, err
	}
	if err := s.graph(); err != nil {
		return nil, err
	}
	return s, nil
}

// languages lists what content/ holds, English first. English is first because
// it is the language every other one is a translation of, and a list that put
// French first because f sorts before v would put the site's default in a
// different place on a day somebody adds a language.
func languages(root string) ([]string, error) {
	entries, err := os.ReadDir(filepath.Join(root, "content"))
	if err != nil {
		return nil, err
	}
	var out []string
	for _, e := range entries {
		if !e.IsDir() || e.Name() == "solutions" {
			continue
		}
		out = append(out, e.Name())
	}
	sort.Slice(out, func(i, j int) bool {
		if (out[i] == "en") != (out[j] == "en") {
			return out[i] == "en"
		}
		return out[i] < out[j]
	})
	return out, nil
}

// keep narrows a language list to the ones asked for, keeping the order of the
// list rather than the order of the ask.
func keep(langs, only []string) ([]string, error) {
	has := map[string]bool{}
	for _, l := range langs {
		has[l] = true
	}
	want := map[string]bool{}
	for _, l := range only {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		if !has[l] {
			return nil, fmt.Errorf("no content in %s, the corpus holds %s", l, strings.Join(langs, ", "))
		}
		want[l] = true
	}
	var out []string
	for _, l := range langs {
		if want[l] {
			out = append(out, l)
		}
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no language asked for")
	}
	return out, nil
}

func loadLang(root, lang string) ([]*Section, error) {
	dir := filepath.Join(root, "content", lang)
	var out []*Section
	err := filepath.WalkDir(dir, func(p string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			// Exercises are files of their own and are read by their own pass.
			if d.Name() == "exercises" {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(p, ".md") {
			return nil
		}
		// The bytes are read here rather than by corpus.ReadFile because the
		// front matter has to be counted as well as parsed, and reading the
		// file twice to learn how long its head is would be silly.
		raw, err := os.ReadFile(p)
		if err != nil {
			return err
		}
		f, err := corpus.ParseFile[corpus.SectionFrontMatter](raw)
		if err != nil {
			return fmt.Errorf("%s: %w", p, err)
		}
		rel, err := filepath.Rel(root, p)
		if err != nil {
			return err
		}
		sec := &Section{Lang: lang, Meta: f.Meta, Body: f.Body,
			Path: filepath.ToSlash(rel), Slug: slug(f.Meta), Head: corpus.BodyStart(raw)}
		sec.Units = Units(f.Body)
		out = append(out, sec)
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out, nil
}

// slug is the last path element of a section URL. It is derived from the front
// matter and not from the file name, because the file name carries a slug of
// the title and a title that is corrected would move the URL.
func slug(m corpus.SectionFrontMatter) string {
	switch m.Kind {
	case "front":
		return "front"
	case "historical":
		return "historical-note"
	}
	if m.Appendix {
		return fmt.Sprintf("a%d", m.Section)
	}
	return fmt.Sprintf("s%d", m.Section)
}

// sectionLabel is what the reference graph knows a § by, alg-viii-s15. The
// front matter and the historical note are not §§ and have none.
func sectionLabel(sec *Section) string {
	if sec.Meta.Kind != "" {
		return ""
	}
	return corpus.Ref{Book: sec.Meta.Book, Chapter: sec.Meta.Chapter,
		Section: sec.Meta.Section, Appendix: sec.Meta.Appendix}.SectionLabel()
}

// Section looks a § up by its label, which is how a reference to a whole § is
// followed.
func (s *Site) Section(label string) *Section { return s.byLabel[label] }

// index collects the statements across languages. The tag is the key, since it
// is the one thing that is the same statement in every printing.
func (s *Site) index() error {
	for _, sec := range s.Sections {
		if l := sectionLabel(sec); l != "" {
			if _, ok := s.byLabel[l]; !ok || sec.Lang == "en" {
				s.byLabel[l] = sec
			}
		}
		for _, u := range sec.Units {
			if u.Heading.Tag == "" {
				continue
			}
			st := s.byTag[u.Heading.Tag]
			if st == nil {
				st = &Statement{Tag: u.Heading.Tag, Label: u.Heading.Label,
					Sections: map[string]*Section{}, Units: map[string]Unit{}}
				s.byTag[u.Heading.Tag] = st
				s.Statements = append(s.Statements, st)
			}
			if st.Sections[sec.Lang] == nil {
				st.Sections[sec.Lang] = sec
				st.Units[sec.Lang] = u
			}
		}
	}
	sort.Slice(s.Statements, func(i, j int) bool { return s.Statements[i].Tag < s.Statements[j].Tag })

	// English for preference again, for the same reason: the label is what the
	// reference graph is built in and the graph is built in English.
	for _, ex := range s.Exercises {
		if _, ok := s.byExLabel[ex.Meta.Label]; !ok || ex.Lang == "en" {
			s.byExLabel[ex.Meta.Label] = ex
		}
		if ex.Meta.Tag == "" {
			continue
		}
		if _, ok := s.byExTag[ex.Meta.Tag]; !ok || ex.Lang == "en" {
			s.byExTag[ex.Meta.Tag] = ex
		}
		// A tag names one thing. Two things under one tag is the failure the
		// whole scheme exists to prevent, and it would show up here as a page
		// written twice with whichever came last winning, which is the quietest
		// way for it to go wrong.
		if st := s.byTag[ex.Meta.Tag]; st != nil {
			return fmt.Errorf("tag %s is on the statement %s and on the exercise %s",
				ex.Meta.Tag, st.Label, ex.Meta.Label)
		}
	}
	return nil
}

// Exercise looks an exercise up by its label, which is how a reference to one is
// followed.
func (s *Site) Exercise(label string) *Exercise { return s.byExLabel[label] }

// ExerciseTag looks one up by its tag, which is what a tag URL resolves through
// when the tag is not a statement's.
func (s *Site) ExerciseTag(tag string) *Exercise { return s.byExTag[tag] }

// Tag looks a statement up. It is what a tag URL resolves through.
func (s *Site) Tag(tag string) *Statement { return s.byTag[tag] }

// coverageFloor is the share of the English section count a language has to
// reach before the navigation offers it.
const coverageFloor = 0.2

// drafted says whether any language is under the floor, which is what decides
// whether the front page explains the rule at all.
func (s *Site) drafted() bool {
	for _, lang := range s.Langs {
		if s.Draft[lang] {
			return true
		}
	}
	return false
}

func (s *Site) floor() error {
	count := map[string]int{}
	for _, sec := range s.Sections {
		count[sec.Lang]++
	}
	for _, lang := range s.Langs {
		if lang == "en" {
			continue
		}
		s.Draft[lang] = float64(count[lang]) < coverageFloor*float64(count["en"])
	}
	return nil
}

// langLinks is the switcher for one page. here is the language of the page, and
// at answers "where is this same thing in that language", empty when that
// language does not have it.
//
// Every page builds its switcher through this, because the rule it applies is
// the same everywhere and it is easy to get subtly wrong page by page: offer a
// language only where it holds the thing the reader is looking at, and keep the
// languages under the coverage floor out unless somebody asked for them. The
// language of the page itself is always in the list, even when it is a draft and
// even when nothing else is, since a reader who has landed on a Vietnamese page
// should be told the page is Vietnamese.
func (s *Site) langLinks(here string, at func(lang string) string) []langLink {
	var out []langLink
	for _, lang := range s.Langs {
		if lang != here && s.Draft[lang] && !s.Drafts {
			continue
		}
		url := at(lang)
		if url == "" {
			continue
		}
		out = append(out, langLink{Lang: lang, URL: url, Here: lang == here, Draft: s.Draft[lang]})
	}
	// One entry is the page saying what it already says in its lang attribute, so
	// there is nothing to switch to and the switcher is not written at all.
	if len(out) < 2 {
		return nil
	}
	return out
}

// graph loads the reference graph and turns it into the two lists a tag page
// shows. The filtering is the filtering the reports do, deliberately: a
// statement that cites itself is a proof naming what it proves and is not the
// chapter leaning on it, and a reference to a § is not a citation of any
// statement in it.
func (s *Site) graph() error {
	res, err := refs.Build(s.Root, "en")
	if err != nil {
		return err
	}
	s.Unresolved = len(res.Unresolved)
	for _, st := range s.Statements {
		if in := res.Index.Statement(st.Label); in != nil {
			st.Page = in.Page
		}
	}
	for i := range res.Edges {
		e := &res.Edges[i]
		if e.FromLabel == e.ToLabel {
			continue
		}
		to := ""
		if e.To != nil {
			to = *e.To
		}
		edge := &Edge{FromTag: e.From, FromLabel: e.FromLabel, ToTag: to, ToLabel: e.ToLabel,
			Raw: e.Raw, How: e.How, Book: e.Book, Chapter: e.Chapter}
		if e.From != "" {
			s.Cites[e.From] = append(s.Cites[e.From], edge)
		}
		if to != "" && e.How != refs.BySection && e.How != refs.OutOfCorpus {
			s.CitedBy[to] = append(s.CitedBy[to], edge)
		}
	}
	// Both lists are read off maps and written into files that are compared
	// byte for byte between two builds, so every order here has to be total.
	// Each list is deduplicated on what its page prints of it, since two rows
	// that read the same are a repeat whatever the graph says they came from.
	// A proof that names Proposition 2 twice is one dependency on the page and
	// two edges in the graph, and both are right for what they are for.
	for _, m := range []map[string][]*Edge{s.CitedBy, s.Cites} {
		for _, list := range m {
			sort.Slice(list, func(i, j int) bool {
				a, b := list[i], list[j]
				if a.FromLabel != b.FromLabel {
					return a.FromLabel < b.FromLabel
				}
				if a.ToLabel != b.ToLabel {
					return a.ToLabel < b.ToLabel
				}
				return a.Raw < b.Raw
			})
		}
	}
	for tag, list := range s.CitedBy {
		s.CitedBy[tag] = dedupe(list, func(e *Edge) string { return e.FromLabel })
	}
	for tag, list := range s.Cites {
		s.Cites[tag] = dedupe(list, func(e *Edge) string { return e.ToLabel + "\x00" + e.Raw })
	}
	return nil
}

// dedupe drops the repeats out of a sorted list of edges, keeping the first of
// each. It is not the sort key, so the scan is over the whole list.
func dedupe(list []*Edge, key func(*Edge) string) []*Edge {
	seen := map[string]bool{}
	out := list[:0]
	for _, e := range list {
		k := key(e)
		if seen[k] {
			continue
		}
		seen[k] = true
		out = append(out, e)
	}
	return out
}

// Tags is the tag file as committed, which the tag table page prints. It is
// read rather than rebuilt from the corpus so that a tag assigned to a
// statement the corpus has lost still shows, which is exactly the case somebody
// needs to see.
func (s *Site) Tags() ([]tagset.Entry, []tagset.Retired, error) {
	set, err := tagset.Load(s.Root)
	if err != nil {
		return nil, nil, err
	}
	return set.Tags, set.Inactive, nil
}

// URL builders. Every one is absolute and ends in a slash. Absolute because a
// tag page is three directories deep and the section it links at is four, and a
// relative link between the two is a counting exercise that is wrong the first
// time somebody adds a level. The slash at the end because the pages are
// index.html files and a link without it costs a redirect.

func (s *Site) url(parts ...string) string {
	p := strings.Trim(path.Join(append([]string{s.Base}, parts...)...), "/")
	if p == "" {
		return "/"
	}
	return "/" + p + "/"
}

// TagURL is the permanent one.
func (s *Site) TagURL(tag string) string { return s.url("tag", tag) }

// SectionURL is one § in one language.
func (s *Site) SectionURL(sec *Section) string {
	return s.url(sec.Lang, sec.Meta.Book, sec.Meta.Chapter, sec.Slug)
}

// ChapterURL is the contents of one chapter in one language.
func (s *Site) ChapterURL(lang, book, chapter string) string {
	return s.url(lang, book, chapter)
}

// ExerciseURL is one exercise. It hangs off the § rather than off an exercises/
// directory of its own, because an exercise belongs to its § and a reader who
// trims the last two elements off this should land on the § and not on a
// parallel tree of the chapter.
func (s *Site) ExerciseURL(ex *Exercise) string {
	return s.url(ex.Lang, ex.Meta.Book, ex.Meta.Chapter, ex.Dir, "ex", strconv.Itoa(ex.Meta.Exercise))
}

// ExercisesURL is the list of them for one §.
func (s *Site) ExercisesURL(lang, book, chapter, dir string) string {
	return s.url(lang, book, chapter, dir, "ex")
}
