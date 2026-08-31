package quality

import (
	"errors"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// atPath is a content file at a path, which is all this block reads.
func atPath(path string) Doc {
	lang, _, _ := contentPath(path)
	return Doc{Path: path, Lang: lang}
}

// transCorpus is a corpus made of nothing but paths, with the Books registered
// so the rows come out in shelf order.
//
// A French volume is registered along with the English ones and it has to be.
// SourceLangs reads the languages the library is printed in off this manifest,
// and with no French volume in it content/fr stops being a source and turns up
// as a language this project translates into, with a column of its own and
// every French section counted as an untranslated English one.
func transCorpus(paths ...string) *Corpus {
	c := &Corpus{
		Books: &corpus.BooksManifest{Books: []corpus.Book{
			{ID: "alg-viii", Book: "alg", Lang: "en", Chapters: []string{"VIII"}},
			{ID: "alg-viii-fr", Book: "alg", Lang: "fr", Chapters: []string{"VIII"}},
			{ID: "ac-i-vii", Book: "ac", Lang: "en", Chapters: []string{"I"}},
		}},
		Langs: []string{"en", "en-mt", "fr", "vi"},
	}
	for _, p := range paths {
		c.Docs = append(c.Docs, atPath(p))
	}
	return c
}

// The question this table exists to answer is how many exercises a Book has and
// how many of them are translated, so that is what it has to get right.
func TestTheTranslationTableCountsExercisesAgainstTheirTranslations(t *testing.T) {
	c := transCorpus(
		"content/en/alg/VIII/01_s1_rings.md",
		"content/en/alg/VIII/exercises/s1/01.md",
		"content/en/alg/VIII/exercises/s1/02.md",
		"content/en/alg/VIII/exercises/s1/03.md",
		"content/vi/alg/VIII/01_s1_rings.md",
		"content/vi/alg/VIII/exercises/s1/01.md",
	)
	got := Translated(c)
	if want := "| Algebra | 1 | 3 | 1 | 1 | 50% |"; !strings.Contains(got, want) {
		t.Errorf("the Algebra row is not %q:\n%s", want, got)
	}
	if want := "Vietnamese has 1 of the 1 sections and 1 of the 3 exercises"; !strings.Contains(got, want) {
		t.Errorf("the summary does not say %q:\n%s", want, got)
	}
}

// The English is two directories and a file in either of them is a file to
// translate. Counting only content/en would call every section of the chapters
// Springer never printed untranslatable rather than untranslated, which is the
// opposite of what the reader of this table wants to know.
func TestTheMachineEnglishCountsAsSomethingToTranslate(t *testing.T) {
	c := transCorpus(
		"content/en-mt/alg/IX/01_s1_forms.md",
		"content/en-mt/alg/IX/exercises/s1/01.md",
		"content/vi/alg/IX/01_s1_forms.md",
	)
	got := Translated(c)
	if want := "| Algebra | 1 | 1 | 1 | 0 | 50% |"; !strings.Contains(got, want) {
		t.Errorf("the machine English is not counted as work:\n%s", got)
	}
}

// A path held in both English trees is one section under two readings, not two
// sections. Counting it twice would inflate the denominator and make a Book
// look further behind than it is.
func TestASectionInBothEnglishTreesIsCountedOnce(t *testing.T) {
	c := transCorpus(
		"content/en/alg/VIII/01_s1_rings.md",
		"content/en-mt/alg/VIII/01_s1_rings.md",
		"content/vi/alg/VIII/01_s1_rings.md",
	)
	if want := "| Algebra | 1 | 0 | 1 | 0 | 100% |"; !strings.Contains(Translated(c), want) {
		t.Errorf("the same section in both English trees is counted twice:\n%s", Translated(c))
	}
}

// The French is counted and never matched against the English by path. A file
// name carries a slug of its own title, so the same section is
// 01_s1_ideaux_premiers.md in one tree and 01_s1_prime_ideals.md in the other.
// Matching them by path is what once reported 289 French sections with no
// English when the answer was 41, so the French must not reach the table.
func TestTheFrenchIsNotMatchedAgainstTheEnglishByPath(t *testing.T) {
	c := transCorpus(
		"content/fr/ac/I/01_s1_ideaux_premiers.md",
		"content/en/ac/I/01_s1_prime_ideals.md",
		"content/vi/ac/I/01_s1_prime_ideals.md",
	)
	got := Translated(c)
	if want := "| Commutative Algebra | 1 | 0 | 1 | 0 | 100% |"; !strings.Contains(got, want) {
		t.Errorf("the French slug has been taken for an untranslated section:\n%s", got)
	}
	if want := "The French originals are 1 sections and 0 exercises"; !strings.Contains(got, want) {
		t.Errorf("the French is not reported as a holding:\n%s", got)
	}
}

// Solutions live under content/ and are not a language. A language directory is
// what content/ lists and solutions is a sibling of them, so nothing about the
// solutions belongs in a table about translations.
func TestSolutionsAreNotATranslation(t *testing.T) {
	c := transCorpus(
		"content/en/alg/VIII/exercises/s1/01.md",
		"content/solutions/en/alg/VIII/s1/01.md",
	)
	if want := "| Algebra | 0 | 1 | 0 | 0 | 0% |"; !strings.Contains(Translated(c), want) {
		t.Errorf("a solution has been counted as a translation:\n%s", Translated(c))
	}
}

// A Book with nothing in the English holds no row rather than a row of zeroes
// divided by zero. The per cent of nothing is not nought, and printing it as
// nought would say a Book is untranslated when there is nothing to translate.
func TestABookWithNoEnglishIsNotAZeroPerCentRow(t *testing.T) {
	if got := percent(0, 0); got != "n/a" {
		t.Errorf("percent(0, 0) is %q, want n/a", got)
	}
	c := transCorpus("content/fr/ts/I/01_s1_algebres.md")
	if got := Translated(c); strings.Contains(got, "Théories spectrales") {
		t.Errorf("a Book with nothing in the English has a row:\n%s", got)
	}
}

// The rows go in the order the Éléments shelve their Books, which is the order
// manifests/books.yaml registers them in, and not alphabetically. Algebra comes
// before Commutative Algebra in the Éléments and after it in the alphabet.
func TestTheRowsAreInShelfOrderAndNotAlphabetical(t *testing.T) {
	c := transCorpus(
		"content/en/ac/I/01_s1_prime_ideals.md",
		"content/en/alg/VIII/01_s1_rings.md",
	)
	got := Translated(c)
	alg, ac := strings.Index(got, "| Algebra |"), strings.Index(got, "| Commutative Algebra |")
	if alg < 0 || ac < 0 {
		t.Fatalf("both Books should have a row:\n%s", got)
	}
	if alg > ac {
		t.Errorf("the rows are alphabetical rather than in shelf order:\n%s", got)
	}
}

// A Book whose only English is this project's own reading of the French is a
// Book where every translation is a translation of a translation, and the table
// has to say so. A hundred per cent for such a Book is not the claim a hundred
// per cent makes for one Springer printed, and the two sit in the same column.
func TestABookWithNoPrintedEnglishSaysSoInTheMachineColumn(t *testing.T) {
	c := transCorpus(
		"content/en-mt/ta/I/01_s1_revetements.md",
		"content/en-mt/ta/I/exercises/s1/01.md",
		"content/vi/ta/I/01_s1_revetements.md",
		"content/vi/ta/I/exercises/s1/01.md",
		"content/en/alg/VIII/01_s1_rings.md",
		"content/vi/alg/VIII/01_s1_rings.md",
	)
	got := Translated(c)
	if want := "| 1 | 1 | 100% | 2, all of it |"; !strings.Contains(got, want) {
		t.Errorf("the wholly machine English Book does not end %q:\n%s", want, got)
	}
	if want := "| Algebra | 1 | 0 | 1 | 0 | 100% | 0 |"; !strings.Contains(got, want) {
		t.Errorf("the Springer translated Book is not marked as having none:\n%s", got)
	}
	if want := "2 of the 3 files in Vietnamese"; !strings.Contains(got, want) {
		t.Errorf("the prose does not count the two-hop files: %q\n%s", want, got)
	}
}

// A Book part printed and part not gets the count and not the words, because
// "all of it" is a different warning and saying it wrongly is worse than a bare
// number.
func TestAPartlyPrintedBookGetsACountAndNotTheWords(t *testing.T) {
	c := transCorpus(
		"content/en/ac/I/01_s1_prime_ideals.md",
		"content/en-mt/ac/VIII/01_s1_dimension.md",
	)
	got := Translated(c)
	if want := "| Commutative Algebra | 2 | 0 | 0 | 0 | 0% | 1 |"; !strings.Contains(got, want) {
		t.Errorf("the partly printed Book is not counted plainly:\n%s", got)
	}
	if strings.Contains(got, "1, all of it") {
		t.Errorf("a partly printed Book is described as wholly machine English:\n%s", got)
	}
}

// Front matter that will not parse is still a file somebody has to translate,
// so the kind is read off the path and not off the fields. A file that lost its
// front matter should not quietly stop being counted as work.
func TestAFileWithBrokenFrontMatterIsStillCounted(t *testing.T) {
	c := transCorpus()
	c.Docs = append(c.Docs,
		Doc{Path: "content/en/alg/VIII/exercises/s1/01.md", Lang: "en", Err: errors.New("front matter will not parse")},
		Doc{Path: "content/en/alg/VIII/01_s1_rings.md", Lang: "en", Err: errors.New("front matter will not parse")},
	)
	if want := "| Algebra | 1 | 1 | 0 | 0 | 0% |"; !strings.Contains(Translated(c), want) {
		t.Errorf("a file with broken front matter has dropped out of the count:\n%s", Translated(c))
	}
}
