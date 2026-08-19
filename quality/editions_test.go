package quality

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// ex is one exercise file of one printing as S11 sees it.
func ex(lang string, section, n int) Doc {
	return Doc{
		Path: "content/" + lang + "/alg/VIII/exercises/s2/00.md",
		Lang: lang, Kind: KindExercise,
		Exercise: &corpus.ExerciseFrontMatter{
			Book: "alg", Chapter: "VIII", Section: section, Exercise: n,
		},
	}
}

// printings is a corpus of two volumes, which is what S11 needs to say
// anything: one printing on its own is S07's business.
func printings(docs []Doc, diffs ...corpus.Difference) *Corpus {
	return &Corpus{
		Docs:     docs,
		Books:    &corpus.BooksManifest{Books: []corpus.Book{{ID: "alg-viii", Lang: "en"}, {ID: "alg-viii-fr", Lang: "fr"}}},
		Editions: &corpus.EditionsManifest{Differences: diffs},
	}
}

// The case this rule was written for. Chapter VIII § 2 is twenty exercises in
// the 2023 English and nineteen in the 2012 French, and until the pages were
// read there was no telling that from the last page of the French § going
// unread.
func TestAnExerciseInOnePrintingAndNotTheOtherIsReported(t *testing.T) {
	c := printings([]Doc{ex("en", 2, 1), ex("en", 2, 2), ex("fr", 2, 1)})
	got, err := s11(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("reported %v, want the one exercise the French does not have", got)
	}
	if got[0].File != "content/fr/alg/VIII/exercises/s2" {
		t.Errorf("the finding points at %q, want the printing that is missing it", got[0].File)
	}
}

// Once the pages have been read and both printings turn out to be right, the
// manifest says so and the rule goes quiet. This is the whole point of having
// the manifest: the count is settled once rather than chased every time.
func TestADifferenceTheManifestAccountsForIsNotReported(t *testing.T) {
	c := printings(
		[]Doc{ex("en", 2, 1), ex("en", 2, 2), ex("fr", 2, 1)},
		corpus.Difference{Book: "alg", Chapter: "VIII", Section: 2, Exercise: 2,
			In: []string{"en"}, Why: "the French printing stops at 1"},
	)
	got, err := s11(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %v, want nothing", got)
	}
}

// An entry that names the wrong printings does not account for anything. It is
// what a manifest written against one volume looks like after the other volume
// is extracted, and reading it as an excuse would hide the exercise it now gets
// wrong.
func TestAManifestEntryThatNamesTheWrongPrintingsStillReports(t *testing.T) {
	c := printings(
		[]Doc{ex("en", 2, 1), ex("en", 2, 2), ex("fr", 2, 1)},
		corpus.Difference{Book: "alg", Chapter: "VIII", Section: 2, Exercise: 2,
			In: []string{"fr"}, Why: "written the wrong way round"},
	)
	got, err := s11(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("reported %v, want the exercise the entry does not cover", got)
	}
}

// A § extracted in one printing and not in the other is work not yet done. The
// corpus has ten French volumes with no English beside them, and reporting every
// exercise of every one of them would be nine hundred findings that all say the
// same thing bourbaki report coverage already says.
func TestASectionOnlyOnePrintingHasExtractedIsNotReported(t *testing.T) {
	c := printings([]Doc{ex("en", 2, 1), ex("en", 2, 2)})
	got, err := s11(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %v, want nothing", got)
	}
}

// content/en-mt is English a model wrote out of the French, so it holds the
// French printing's exercises. Counted as a printing of its own it would say
// the English volume has an exercise the machine English is missing, which is a
// fact about the translation queue and not about either book.
func TestTheMachineEnglishTreeIsNotAPrinting(t *testing.T) {
	c := printings([]Doc{ex("en", 2, 1), ex("en", 2, 2), ex("en-mt", 2, 1)})
	got, err := s11(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %v, want nothing", got)
	}
}
