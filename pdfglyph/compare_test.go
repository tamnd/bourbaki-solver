package pdfglyph

import (
	"strings"
	"testing"
)

func TestCompareCountsWhatChanged(t *testing.T) {
	// Page 442 of Algèbre chapitre 8, the line that made this necessary: the
	// wide hat read as an exclamation mark before the rewrite and reads as the
	// CMEX accent afterwards, and vi became vi'.
	d := Compare([]rune("π(λ...!) vi = vi"), []rune("π(λ...c) vi0 = vi"))
	if d.Changed[Change{'!', 'c'}] != 1 {
		t.Errorf("the wide hat was not counted as a change: %v", d.Changed)
	}
	if d.Added['0'] != 1 {
		t.Errorf("the prime was not counted as added: %v", d.Added)
	}
	if len(d.Lost) != 0 {
		t.Errorf("lost %v", d.Lost)
	}
}

func TestCompareOnPureInsertion(t *testing.T) {
	d := Compare([]rune("Mestunsous-modulede M"), []rune("M0estunsous-modulede M"))
	if d.Added['0'] != 1 || len(d.Changed) != 0 || len(d.Lost) != 0 {
		t.Errorf("added %v changed %v lost %v", d.Added, d.Changed, d.Lost)
	}
	if d.Kept != len("Mestunsous-modulede M") {
		t.Errorf("kept %d of %d", d.Kept, len("Mestunsous-modulede M"))
	}
}

func TestCompareReportsALoss(t *testing.T) {
	d := Compare([]rune("Soit A un anneau"), []rune("Soit un anneau"))
	if Total(d.Lost) == 0 {
		t.Errorf("a dropped word was not reported: %v", d)
	}
}

func TestCompareOnIdenticalPages(t *testing.T) {
	s := []rune("Soit A un anneau commutatif et M un A-module")
	d := Compare(s, s)
	if d.Kept != len(s) || Total(d.Added) != 0 || Total(d.Lost) != 0 || len(d.Changed) != 0 {
		t.Errorf("two readings of one page differ: %+v", d)
	}
}

// Two readings that have nothing to do with each other are reported as such
// rather than aligned into a thousand edits.
func TestCompareGivesUpOnRubbish(t *testing.T) {
	a := make([]rune, 0, 400)
	b := make([]rune, 0, 400)
	for i := 0; i < 400; i++ {
		a = append(a, 'a')
		b = append(b, 'b')
	}
	if d := Compare(a, b); !d.Hard {
		t.Error("two unrelated pages were aligned")
	}
}

// A volume has blank pages in it, and two readings of nothing at all crashed
// the first version of this on the front matter of every book.
func TestCompareOnBlankPages(t *testing.T) {
	d := Compare(nil, nil)
	if d.Kept != 0 || d.Hard || Total(d.Added) != 0 || Total(d.Lost) != 0 {
		t.Errorf("two blank pages differ: %+v", d)
	}
	if d := Compare(nil, []rune("0")); d.Added['0'] != 1 {
		t.Errorf("a character on a page that had none: %+v", d)
	}
	if d := Compare([]rune("0"), nil); d.Lost['0'] != 1 {
		t.Errorf("a page emptied: %+v", d)
	}
}

// The Myers difference is free to choose any shortest script, so where a
// substitution has the same character again a few places along it will keep that
// one and put the deletion and the insertion in two runs. Page 118 of Théorie
// des ensembles chapitres 1 et 2 does exactly this with four of its 34 wide
// tildes, and reading the two halves apart called four characters lost on a page
// where nothing was lost.
func TestComparePairsASplitSubstitution(t *testing.T) {
	old := []rune("the sum e of the terms e and e")
	now := []rune("the sum ̃ of the terms ̃ and ̃")
	d := Compare(old, now)
	if Total(d.Lost) != 0 {
		t.Errorf("a substitution was read as a loss: %v", d.Lost)
	}
	if Total(d.Added) != 0 {
		t.Errorf("a substitution was read as a gain: %v", d.Added)
	}
	if d.Changed[Change{'e', '̃'}] != 3 {
		t.Errorf("changed %v, want three e read as a combining tilde", d.Changed)
	}
}

// Nothing pairs across the page. A character lost at the top and another
// recovered at the bottom are two separate things, and the whole point of this
// comparison is that the first of them is a bug.
func TestCompareDoesNotPairAcrossThePage(t *testing.T) {
	filler := strings.Repeat("x", 200)
	old := []rune("e" + filler)
	now := []rune(filler + "̃")
	d := Compare(old, now)
	if d.Lost['e'] != 1 {
		t.Errorf("a lost character was paid for by one recovered elsewhere: %+v", d)
	}
	if d.Added['̃'] != 1 {
		t.Errorf("added %v, want the tilde on its own", d.Added)
	}
}
