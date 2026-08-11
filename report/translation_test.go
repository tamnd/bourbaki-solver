package report

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/quality"
)

// english is one English section or exercise on the shelf.
func english(path, book, chapter, kind, body string) quality.Doc {
	d := quality.Doc{Path: path, Lang: "en", Kind: kind, Body: body}
	if kind == quality.KindSection {
		d.Section = &corpus.SectionFrontMatter{Book: book, Chapter: chapter, Lang: "en",
			ContentSHA256: corpus.ContentSHA256(body)}
	} else {
		d.Exercise = &corpus.ExerciseFrontMatter{Book: book, Chapter: chapter, Lang: "en"}
	}
	return d
}

// translated is a translation of one of those, made from the English body given
// as from. Passing an old body is how a stale file is written.
func translated(path, lang, kind, body string, en quality.Doc, from string) quality.Doc {
	d := quality.Doc{Path: path, Lang: lang, Kind: kind, Body: body}
	sum := corpus.ContentSHA256(from)
	if kind == quality.KindSection {
		s := *en.Section
		s.Lang = lang
		s.TranslatedFrom = en.Path
		s.SourceSHA256 = sum
		d.Section = &s
	} else {
		e := *en.Exercise
		e.Lang = lang
		e.TranslatedFrom = en.Path
		e.SourceSHA256 = sum
		d.Exercise = &e
	}
	return d
}

func shelfOf(langs []string, docs ...quality.Doc) *quality.Corpus {
	return &quality.Corpus{Root: "/corpus", Langs: langs, Docs: docs}
}

func vi(terms ...glossary.Term) *glossary.Glossary {
	return &glossary.Glossary{Version: 1, Terms: terms}
}

// The denominator is the English, not the translations. A language that has
// translated everything it has translated is at 100 per cent of itself and says
// nothing, and the number worth printing is the share of the English that is
// there.
func TestALanguageIsCountedAgainstTheEnglishAndNotAgainstItself(t *testing.T) {
	s1 := english("content/en/alg/VIII/01.md", "alg", "VIII", quality.KindSection, "a ring\n")
	s2 := english("content/en/alg/VIII/02.md", "alg", "VIII", quality.KindSection, "a field\n")
	e1 := english("content/en/alg/VIII/exercises/s1/01.md", "alg", "VIII", quality.KindExercise, "an exercise\n")
	e2 := english("content/en/alg/VIII/exercises/s1/02.md", "alg", "VIII", quality.KindExercise, "another\n")
	c := shelfOf([]string{"en", "vi"}, s1, s2, e1, e2,
		translated("content/vi/alg/VIII/01.md", "vi", quality.KindSection, "một vành\n", s1, s1.Body),
		translated("content/vi/alg/VIII/exercises/s1/01.md", "vi", quality.KindExercise, "một bài tập\n", e1, e1.Body),
	)

	out := Translations(c, vi())
	if len(out) != 1 || out[0].Lang != "vi" {
		t.Fatalf("got %d languages, want one and vi", len(out))
	}
	got := out[0]
	if got.Sections != 2 || got.SectionsDone != 1 {
		t.Errorf("sections = %d of %d, want 1 of 2", got.SectionsDone, got.Sections)
	}
	if got.Exercises != 2 || got.ExercisesDone != 1 {
		t.Errorf("exercises = %d of %d, want 1 of 2", got.ExercisesDone, got.Exercises)
	}
	if got.Coverage() != 50 {
		t.Errorf("coverage = %v, want 50", got.Coverage())
	}
}

// A file made from an English that has changed since is counted as translated,
// because it is there, and counted as stale, because what is there is not a
// translation of what the corpus now holds. Both numbers are needed: reporting
// only the first says the work is done and only the second says it was never
// started.
func TestAFileMadeFromAnEnglishThatHasChangedIsTranslatedAndStale(t *testing.T) {
	en := english("content/en/alg/VIII/01.md", "alg", "VIII", quality.KindSection, "a ring, reread\n")
	c := shelfOf([]string{"en", "vi"}, en,
		translated("content/vi/alg/VIII/01.md", "vi", quality.KindSection, "một vành\n", en, "a ring\n"),
	)

	out := Translations(c, vi())
	if len(out) != 1 {
		t.Fatalf("got %d languages, want one", len(out))
	}
	if out[0].Done() != 1 {
		t.Errorf("done = %d, want 1: a stale file is a file that is there", out[0].Done())
	}
	if out[0].Stale() != 1 {
		t.Errorf("stale = %d, want 1", out[0].Stale())
	}
}

// A translation that records no source hash cannot be shown to be current, and
// a report that called it current would be the report saying so on no evidence.
func TestATranslationThatRecordsNoSourceHashCountsAsStale(t *testing.T) {
	en := english("content/en/alg/VIII/01.md", "alg", "VIII", quality.KindSection, "a ring\n")
	tr := translated("content/vi/alg/VIII/01.md", "vi", quality.KindSection, "một vành\n", en, en.Body)
	tr.Section.SourceSHA256 = ""
	c := shelfOf([]string{"en", "vi"}, en, tr)

	out := Translations(c, vi())
	if len(out) != 1 || out[0].Stale() != 1 {
		t.Fatalf("stale = %v, want 1", out)
	}
}

// The whole point of counting per term rather than per file. One file missing a
// term is a sentence somebody wrote differently; every file missing the same
// term is a row that is wrong, and it has to come out at the top where somebody
// will read it.
func TestTheTermEveryFileMissesComesOutAboveTheOneMissedOnce(t *testing.T) {
	s1 := english("content/en/alg/VIII/01.md", "alg", "VIII", quality.KindSection, "a ring with respect to a basis\n")
	s2 := english("content/en/alg/VIII/02.md", "alg", "VIII", quality.KindSection, "a ring with respect to a basis\n")
	c := shelfOf([]string{"en", "vi"}, s1, s2,
		// Both keep vành for ring. Neither keeps the rendering pinned for
		// respect, and the second also writes the basis some other way.
		translated("content/vi/alg/VIII/01.md", "vi", quality.KindSection, "một vành với cơ sở\n", s1, s1.Body),
		translated("content/vi/alg/VIII/02.md", "vi", quality.KindSection, "một vành và một hệ\n", s2, s2.Body),
	)
	g := vi(
		glossary.Term{EN: "ring", VI: "vành"},
		glossary.Term{EN: "basis", VI: "cơ sở"},
		glossary.Term{EN: "respect", VI: "bảo toàn"},
	)

	rows := Terms(c, g, "vi", TermOptions{})
	if len(rows) != 2 {
		t.Fatalf("got %d rows, want 2: ring is kept and the other two are not", len(rows))
	}
	if rows[0].EN != "respect" || rows[0].Missed != 2 {
		t.Fatalf("the first row is %s missed %d times, want respect missed 2", rows[0].EN, rows[0].Missed)
	}
	if rows[1].EN != "basis" || rows[1].Missed != 1 {
		t.Fatalf("the second row is %s missed %d times, want basis missed 1", rows[1].EN, rows[1].Missed)
	}
	if len(rows[0].Files) != 2 {
		t.Errorf("respect names %v, want both files: a list that is cut hides which files", rows[0].Files)
	}
}

// The report is the misses by default. A reader who wants the whole vocabulary
// asks for it, and until then a term that is kept everywhere is not news.
func TestATermKeptEverywhereIsOnlyListedWhenAllIsAsked(t *testing.T) {
	en := english("content/en/alg/VIII/01.md", "alg", "VIII", quality.KindSection, "a ring\n")
	c := shelfOf([]string{"en", "vi"}, en,
		translated("content/vi/alg/VIII/01.md", "vi", quality.KindSection, "một vành\n", en, en.Body),
	)
	g := vi(glossary.Term{EN: "ring", VI: "vành"})

	if rows := Terms(c, g, "vi", TermOptions{}); len(rows) != 0 {
		t.Fatalf("got %v, want nothing: the term is kept", rows)
	}
	rows := Terms(c, g, "vi", TermOptions{All: true})
	if len(rows) != 1 || rows[0].Mentions != 1 || rows[0].Missed != 0 {
		t.Fatalf("got %v, want one row shown once and missed never", rows)
	}
}

// A term inside a formula was never asked for: the translator was told to copy
// the mathematics through. This is L06's rule and the report has to keep it, or
// it will report a miss on every section that mentions a ring only in a display.
func TestATermThatOnlyAppearsInAFormulaIsNotAMiss(t *testing.T) {
	en := english("content/en/alg/VIII/01.md", "alg", "VIII", quality.KindSection,
		"let $\\operatorname{ring} A$ be given\n")
	c := shelfOf([]string{"en", "vi"}, en,
		translated("content/vi/alg/VIII/01.md", "vi", quality.KindSection,
			"cho $\\operatorname{ring} A$\n", en, en.Body),
	)
	g := vi(glossary.Term{EN: "ring", VI: "vành"})

	if rows := Terms(c, g, "vi", TermOptions{All: true}); len(rows) != 0 {
		t.Fatalf("got %v, want nothing: the word is only inside the mathematics", rows)
	}
}

// A language whose glossary is empty has nothing to be held to, and nothing is
// not zero. Printing 0% adherence for Japanese on the day its rows are written
// would read as a translation that keeps none of the vocabulary.
func TestALanguageWithNoGlossaryRowsHasNoAdherenceRatherThanNone(t *testing.T) {
	en := english("content/en/alg/VIII/01.md", "alg", "VIII", quality.KindSection, "a ring\n")
	c := shelfOf([]string{"en", "ja"}, en,
		translated("content/ja/alg/VIII/01.md", "ja", quality.KindSection, "環\n", en, en.Body),
	)

	out := Translations(c, vi(glossary.Term{EN: "ring", VI: "vành"}))
	if len(out) != 1 {
		t.Fatalf("got %d languages, want one", len(out))
	}
	if out[0].Adherence() != -1 {
		t.Errorf("adherence = %v, want -1, which prints as a dash", out[0].Adherence())
	}
	if !strings.Contains(out[0].Table(), "-") {
		t.Errorf("the table is %q, want a dash where there is no percentage", out[0].Table())
	}
	if strings.Contains(out[0].Line(), "glossary") {
		t.Errorf("the line is %q, want no glossary claim at all", out[0].Line())
	}
}

// The check before a run has to cover what the run will cover, or it is a check
// of something else.
func TestACheckIsBoundedTheSameThreeWaysTheRunIs(t *testing.T) {
	a := english("content/en/alg/VIII/01.md", "alg", "VIII", quality.KindSection, "a ring\n")
	b := english("content/en/top/IV/01.md", "top", "IV", quality.KindSection, "a ring\n")
	c := shelfOf([]string{"en", "vi"}, a, b,
		translated("content/vi/alg/VIII/01.md", "vi", quality.KindSection, "một hệ\n", a, a.Body),
		translated("content/vi/top/IV/01.md", "vi", quality.KindSection, "một hệ\n", b, b.Body),
	)
	g := vi(glossary.Term{EN: "ring", VI: "vành"})

	if rows := Terms(c, g, "vi", TermOptions{}); len(rows) != 1 || rows[0].Missed != 2 {
		t.Fatalf("unbounded, ring is missed %v, want twice", rows)
	}
	rows := Terms(c, g, "vi", TermOptions{Book: "top"})
	if len(rows) != 1 || rows[0].Missed != 1 {
		t.Fatalf("bounded to top, ring is missed %v, want once", rows)
	}
	rows = Terms(c, g, "vi", TermOptions{Chapter: "viii"})
	if len(rows) != 1 || rows[0].Missed != 1 {
		t.Fatalf("bounded to chapter viii, ring is missed %v, want once, matched without regard to case", rows)
	}
	rows = Terms(c, g, "vi", TermOptions{File: "content/en/top/IV/01.md"})
	if len(rows) != 1 || rows[0].Missed != 1 {
		t.Fatalf("bounded to one English file, ring is missed %v, want once", rows)
	}
}

// A chapter nobody has started is not a row, and it is still in the totals. A
// table of empty rows is a table nobody reads, and a total that leaves the
// chapter out is a coverage figure that flatters itself.
func TestAChapterNotStartedIsOutOfTheTableAndStillInTheTotal(t *testing.T) {
	a := english("content/en/alg/VIII/01.md", "alg", "VIII", quality.KindSection, "a ring\n")
	b := english("content/en/alg/IV/01.md", "alg", "IV", quality.KindSection, "a field\n")
	c := shelfOf([]string{"en", "vi"}, a, b,
		translated("content/vi/alg/VIII/01.md", "vi", quality.KindSection, "một vành\n", a, a.Body),
	)

	out := Translations(c, vi())
	if len(out) != 1 {
		t.Fatalf("got %d languages, want one", len(out))
	}
	if len(out[0].Rows) != 1 || out[0].Rows[0].Chapter != "VIII" {
		t.Fatalf("rows = %v, want chapter VIII alone", out[0].Rows)
	}
	if out[0].Sections != 2 || out[0].Coverage() != 50 {
		t.Errorf("coverage = %v over %d sections, want 50 over 2", out[0].Coverage(), out[0].Sections)
	}
}

// The chapters come out in the order the book prints them, which is the roman
// numeral and not the string. IV before VIII, and X after both.
func TestChaptersComeOutInTheOrderTheBookPrintsThem(t *testing.T) {
	var docs []quality.Doc
	for _, ch := range []string{"X", "IV", "VIII"} {
		en := english("content/en/alg/"+ch+"/01.md", "alg", ch, quality.KindSection, "a ring in "+ch+"\n")
		docs = append(docs, en,
			translated("content/vi/alg/"+ch+"/01.md", "vi", quality.KindSection, "một vành\n", en, en.Body))
	}
	out := Translations(shelfOf([]string{"en", "vi"}, docs...), vi())
	if len(out) != 1 || len(out[0].Rows) != 3 {
		t.Fatalf("got %v, want three chapters in one language", out)
	}
	want := []string{"IV", "VIII", "X"}
	for i, r := range out[0].Rows {
		if r.Chapter != want[i] {
			t.Fatalf("the chapters are %v, want %v", out[0].Rows, want)
		}
	}
}

// English is a source and so is French, and neither is a language this report is
// about. Listing content/en as nought per cent translated into English is the
// kind of row that makes a reader stop trusting the rest of the table.
func TestASourceLanguageIsNotATargetLanguage(t *testing.T) {
	en := english("content/en/alg/VIII/01.md", "alg", "VIII", quality.KindSection, "a ring\n")
	fr := english("content/fr/alg/VIII/01.md", "alg", "VIII", quality.KindSection, "un anneau\n")
	fr.Lang = "fr"
	fr.Section.Lang = "fr"
	c := shelfOf([]string{"en", "fr", "vi"}, en, fr,
		translated("content/vi/alg/VIII/01.md", "vi", quality.KindSection, "một vành\n", en, en.Body),
	)
	c.Books = &corpus.BooksManifest{Books: []corpus.Book{
		{ID: "alg-viii", Lang: "en"}, {ID: "alg-viii-fr", Lang: "fr"},
	}}

	out := Translations(c, vi())
	if len(out) != 1 || out[0].Lang != "vi" {
		t.Fatalf("got %v, want vi alone", out)
	}
	if out[0].Sections != 1 {
		t.Errorf("sections = %d, want 1: the French is not something to translate into", out[0].Sections)
	}
}
