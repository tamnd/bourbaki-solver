package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/quality"
)

// What a translation report has to answer is how much of the English is in a
// language, how much of that is still the English it was made from, and how
// closely the agreed vocabulary is being kept. Three questions, and the corpus
// already knows all three: the coverage is a walk over content/en against the
// translated files that name each one, the staleness is the recorded hash
// against the English hash, which is L05, and the adherence is L06 counted
// rather than reported one file at a time.
//
// The last of those is why this exists rather than being read out of the audit.
// L06 says "this file misses 1 of 57 terms", which is the right thing to say
// about a file and the wrong thing to say about a language: the question worth
// asking of a whole tree is which term is being missed, across how many files,
// because a term missed once is a sentence and a term missed in thirty files is
// a bad row in the glossary. The first run of L06 found a bad row, and it found
// it because somebody read the finding rather than because the rule said so.
// Counting per term is what makes that the report's answer instead of a
// reader's.
//
// Percentages here are of what the corpus holds and not of the Éléments. A
// language at 100 per cent is a language that has every English file the corpus
// has today, which is four chapters of a library of forty three volumes.

// A Translation is one target language against the English.
type Translation struct {
	Lang string
	Rows []TranslationRow
	TranslationCounts
}

// A TranslationRow is one chapter of one Book in that language.
type TranslationRow struct {
	Book    string // alg
	Chapter string // VIII
	TranslationCounts
}

// TranslationCounts are the numbers, which are the same shape for a chapter and
// for a whole language.
type TranslationCounts struct {
	// Sections and Exercises are how many English files there are. Done is how
	// many carry a translation, Stale how many of those were made from an
	// English that has since changed.
	Sections       int
	SectionsDone   int
	SectionsStale  int
	Exercises      int
	ExercisesDone  int
	ExercisesStale int

	// Mentions is how many glossary terms the English of the translated files
	// mentions, counted once per term per file, and Followed is how many of
	// those the translation renders the way the glossary says. Both are zero
	// for a language with no glossary rows, which is not the same as adherence
	// of zero, and Adherence says so.
	Mentions int
	Followed int
}

func (c TranslationCounts) add(o TranslationCounts) TranslationCounts {
	c.Sections += o.Sections
	c.SectionsDone += o.SectionsDone
	c.SectionsStale += o.SectionsStale
	c.Exercises += o.Exercises
	c.ExercisesDone += o.ExercisesDone
	c.ExercisesStale += o.ExercisesStale
	c.Mentions += o.Mentions
	c.Followed += o.Followed
	return c
}

// Files is the English this language could hold, sections and exercises
// together, and Done is what it does hold.
func (c TranslationCounts) Files() int { return c.Sections + c.Exercises }

// Done is the translated files, stale ones included: a stale file is a file
// that is there.
func (c TranslationCounts) Done() int { return c.SectionsDone + c.ExercisesDone }

// Stale is the translated files whose English has changed since.
func (c TranslationCounts) Stale() int { return c.SectionsStale + c.ExercisesStale }

// Coverage is the share of the English that is in this language, as a
// percentage, and -1 when there is no English to translate.
func (c TranslationCounts) Coverage() float64 {
	if c.Files() == 0 {
		return -1
	}
	return 100 * float64(c.Done()) / float64(c.Files())
}

// Adherence is the share of the glossary terms that are rendered as agreed, and
// -1 when no term was mentioned at all, which is what a language with an empty
// glossary looks like. Reporting that as zero would read as a translation that
// keeps none of the vocabulary, which is the opposite of what it means.
func (c TranslationCounts) Adherence() float64 {
	if c.Mentions == 0 {
		return -1
	}
	return 100 * float64(c.Followed) / float64(c.Mentions)
}

// A TermRow is one glossary term across a whole language: how many translated
// files were shown it, and which of them do not carry it as the glossary writes
// it.
type TermRow struct {
	EN        string
	Rendering string
	Mentions  int
	Missed    int
	// Files are the translations that miss it, in path order. All of them: a
	// term missed in thirty files is the finding, and a list cut to three would
	// hide which thirty.
	Files []string
}

// Translations is every target language under content/, in the order the corpus
// lists them, with the source languages left out. A language with nothing in it
// is not listed, because a row of zeroes for Japanese would read as a failed
// run rather than as work not started.
func Translations(c *quality.Corpus, g *glossary.Glossary) []*Translation {
	source := c.SourceLangs()
	byLang := map[string]*Translation{}
	var order []string
	for _, lang := range c.Langs {
		if source[lang] || lang == "" {
			continue
		}
		byLang[lang] = &Translation{Lang: lang}
		order = append(order, lang)
	}
	if len(order) == 0 {
		return nil
	}

	// The denominator first: every English file, by chapter. It is the same
	// walk for each language, so it is done once and copied.
	type key struct{ book, chapter string }
	english := map[key]*TranslationCounts{}
	var keys []key
	for _, d := range c.Docs {
		if d.Lang != "en" {
			continue
		}
		k, ok := chapterKey(d)
		if !ok {
			continue
		}
		kk := key{k[0], k[1]}
		if english[kk] == nil {
			english[kk] = &TranslationCounts{}
			keys = append(keys, kk)
		}
		if d.Kind == quality.KindSection {
			english[kk].Sections++
		} else {
			english[kk].Exercises++
		}
	}
	sort.Slice(keys, func(i, j int) bool { return chapterLess(keys[i].book, keys[i].chapter, keys[j].book, keys[j].chapter) })

	rows := map[string]map[key]*TranslationRow{}
	for _, lang := range order {
		rows[lang] = map[key]*TranslationRow{}
		for _, k := range keys {
			rows[lang][k] = &TranslationRow{Book: k.book, Chapter: k.chapter, TranslationCounts: *english[k]}
		}
	}

	for _, p := range c.Pairs() {
		t := byLang[p.Translation.Lang]
		if t == nil {
			continue
		}
		k, ok := chapterKey(p.English)
		if !ok {
			continue
		}
		row := rows[p.Translation.Lang][key{k[0], k[1]}]
		if row == nil {
			continue
		}
		stale := staleAgainst(p)
		if p.Translation.Kind == quality.KindSection {
			row.SectionsDone++
			if stale {
				row.SectionsStale++
			}
		} else {
			row.ExercisesDone++
			if stale {
				row.ExercisesStale++
			}
		}
		if g == nil {
			continue
		}
		mentions, followed := adherence(g, p)
		row.Mentions += mentions
		row.Followed += followed
	}

	var out []*Translation
	for _, lang := range order {
		t := byLang[lang]
		for _, k := range keys {
			row := rows[lang][k]
			// A chapter this language has not started is not a row. The chapter
			// is still in the denominator through the totals, so nothing is
			// hidden by leaving it out of the table.
			if row.Done() == 0 {
				t.TranslationCounts = t.TranslationCounts.add(row.TranslationCounts)
				continue
			}
			t.Rows = append(t.Rows, *row)
			t.TranslationCounts = t.TranslationCounts.add(row.TranslationCounts)
		}
		if t.Done() == 0 {
			continue
		}
		out = append(out, t)
	}
	return out
}

// chapterKey is the book and chapter a content file belongs to, which is in the
// front matter of both shapes and in neither the path nor the body.
func chapterKey(d quality.Doc) ([2]string, bool) {
	switch {
	case d.Section != nil && d.Section.Book != "":
		return [2]string{d.Section.Book, d.Section.Chapter}, true
	case d.Exercise != nil && d.Exercise.Book != "":
		return [2]string{d.Exercise.Book, d.Exercise.Chapter}, true
	}
	return [2]string{}, false
}

func chapterLess(bookA, chapA, bookB, chapB string) bool {
	if bookA != bookB {
		return bookA < bookB
	}
	ca, _ := corpus.RomanOrder(chapA)
	cb, _ := corpus.RomanOrder(chapB)
	if ca != cb {
		return ca < cb
	}
	return chapA < chapB
}

// staleAgainst is L05's question asked again: was this made from the English
// that is there now. A file that records no hash counts as stale, because
// nothing can say that it is not.
func staleAgainst(p quality.Pair) bool {
	var recorded string
	switch {
	case p.Translation.Section != nil:
		recorded = p.Translation.Section.SourceSHA256
	case p.Translation.Exercise != nil:
		recorded = p.Translation.Exercise.SourceSHA256
	}
	return recorded != corpus.ContentSHA256(p.English.Body)
}

// adherence is L06's question asked again, as two numbers rather than as a
// sentence.
func adherence(g *glossary.Glossary, p quality.Pair) (mentions, followed int) {
	lang := p.Translation.Lang
	en := strings.ToLower(quality.Prose(p.English.Body))
	tr := strings.ToLower(quality.Prose(p.Translation.Body))
	for _, t := range g.Mentioned(lang, en) {
		mentions++
		if glossary.Follows(lang, tr, t.In(lang)) {
			followed++
		}
	}
	return mentions, followed
}

// TermOptions bounds an adherence report to part of the corpus, the same three
// ways translate itself is bounded, so that the check before a pass covers what
// the pass will cover.
type TermOptions struct {
	Book    string // alg
	Chapter string // VIII
	File    string // one English file, relative to the corpus root
	// All keeps the terms nothing missed. Off, the report is the misses, which
	// is what somebody is looking for; on, it is the whole vocabulary of the
	// tree, which is what says how much was checked.
	All bool
}

func (o TermOptions) covers(p quality.Pair) bool {
	if o.File != "" && p.English.Path != o.File {
		return false
	}
	if o.Book == "" && o.Chapter == "" {
		return true
	}
	k, ok := chapterKey(p.English)
	if !ok {
		return false
	}
	if o.Book != "" && k[0] != o.Book {
		return false
	}
	return o.Chapter == "" || strings.EqualFold(k[1], o.Chapter)
}

// Terms is the adherence report per term for one language, worst first, with
// the terms nothing missed left out unless All is set. A term is counted once
// per file: the question is whether the file renders it as agreed, not how
// often, which is Follows's rule and has to stay Follows's rule here.
func Terms(c *quality.Corpus, g *glossary.Glossary, lang string, opt TermOptions) []TermRow {
	byTerm := map[string]*TermRow{}
	var order []string
	for _, p := range c.Pairs() {
		if p.Translation.Lang != lang || !opt.covers(p) {
			continue
		}
		en := strings.ToLower(quality.Prose(p.English.Body))
		tr := strings.ToLower(quality.Prose(p.Translation.Body))
		for _, t := range g.Mentioned(lang, en) {
			row := byTerm[t.EN]
			if row == nil {
				row = &TermRow{EN: t.EN, Rendering: t.In(lang)}
				byTerm[t.EN] = row
				order = append(order, t.EN)
			}
			row.Mentions++
			if glossary.Follows(lang, tr, t.In(lang)) {
				continue
			}
			row.Missed++
			row.Files = append(row.Files, p.Translation.Path)
		}
	}
	var out []TermRow
	for _, en := range order {
		row := byTerm[en]
		if row.Missed == 0 && !opt.All {
			continue
		}
		sort.Strings(row.Files)
		out = append(out, *row)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Missed != out[j].Missed {
			return out[i].Missed > out[j].Missed
		}
		if out[i].Mentions != out[j].Mentions {
			return out[i].Mentions > out[j].Mentions
		}
		return out[i].EN < out[j].EN
	})
	return out
}

// Line is one language in one line, for the languages a run does not want a
// table of.
func (t *Translation) Line() string {
	s := fmt.Sprintf("%s: %d of %d files, %s covered", t.Lang, t.Done(), t.Files(), pct(t.Coverage()))
	if t.Stale() > 0 {
		s += fmt.Sprintf(", %d stale", t.Stale())
	}
	if t.Mentions > 0 {
		s += fmt.Sprintf(", glossary %s", pct(t.Adherence()))
	}
	return s
}

// Table is one language chapter by chapter.
func (t *Translation) Table() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s\n\n", t.Line())
	fmt.Fprintf(&b, "%-12s %-16s %-16s %6s %9s\n", "chapter", "sections", "exercises", "stale", "glossary")
	for _, r := range t.Rows {
		fmt.Fprintf(&b, "%-12s %-16s %-16s %6d %9s\n",
			r.Book+" "+r.Chapter,
			fmt.Sprintf("%d of %d", r.SectionsDone, r.Sections),
			fmt.Sprintf("%d of %d", r.ExercisesDone, r.Exercises),
			r.Stale(),
			pct(r.Adherence()))
	}
	return b.String()
}

// TermTable is the adherence report per term.
func TermTable(lang string, rows []TermRow) string {
	if len(rows) == 0 {
		return fmt.Sprintf("%s: every glossary term the English mentions is rendered as the glossary writes it\n", lang)
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-28s %-24s %8s %8s\n", "term", lang, "shown", "missed")
	for _, r := range rows {
		fmt.Fprintf(&b, "%-28s %-24s %8d %8d\n", r.EN, r.Rendering, r.Mentions, r.Missed)
		for _, f := range r.Files {
			fmt.Fprintf(&b, "    %s\n", f)
		}
	}
	return b.String()
}

// pct prints a percentage, and prints a dash where there is nothing to take a
// percentage of. Zero and nothing are different answers and a report that
// writes both as 0% is a report that cannot be read.
func pct(v float64) string {
	if v < 0 {
		return "-"
	}
	return fmt.Sprintf("%.1f%%", v)
}
