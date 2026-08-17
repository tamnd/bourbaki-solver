package main

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/glossary"
)

// A row that is right about the word and wrong about these books goes, and the
// version moves with it, so every file that was shown it is translated again.
func TestDropTakesTheRowOutAndMovesTheVersion(t *testing.T) {
	root := setCorpus(t)
	if err := glossaryDrop([]string{"-corpus", root, "-write", "simple"}); err != nil {
		t.Fatal(err)
	}
	g := loaded(t, root)
	if len(g.Terms) != 1 {
		t.Fatalf("%d terms left, want 1", len(g.Terms))
	}
	if g.Terms[0].EN != "free A-module" {
		t.Errorf("the row left is %q, want the one nobody asked to drop", g.Terms[0].EN)
	}
	if g.Version != 12 {
		t.Errorf("version = %d, want it moved, since a file shown the old row is stale", g.Version)
	}
}

// Nothing is written without -write. This is a file three languages of
// translation are held to and a removal is not undoable by reading it again.
func TestDropWritesNothingWithoutWrite(t *testing.T) {
	root := setCorpus(t)
	if err := glossaryDrop([]string{"-corpus", root, "simple"}); err != nil {
		t.Fatal(err)
	}
	g := loaded(t, root)
	if len(g.Terms) != 2 || g.Version != 11 {
		t.Errorf("%d terms at version %d, want the file untouched", len(g.Terms), g.Version)
	}
}

// A term that is not there is a typo, and a typo that reports success is a typo
// nobody finds. The rows named alongside it stay, since a half applied
// editorial decision is worse than none.
func TestDropRefusesATermThatIsNotThereAndTakesNothing(t *testing.T) {
	root := setCorpus(t)
	err := glossaryDrop([]string{"-corpus", root, "-write", "simple", "smiple"})
	if err == nil {
		t.Fatal("dropping a term that is not in the glossary was accepted")
	}
	if !strings.Contains(err.Error(), "smiple") {
		t.Errorf("the error does not say which term: %v", err)
	}
	g := loaded(t, root)
	if len(g.Terms) != 2 {
		t.Errorf("%d terms left, want both, since the run named one of them wrong", len(g.Terms))
	}
}

// The term is matched the way the glossary matches terms everywhere else, so a
// person types the word as they read it and not as the file happens to hold it.
func TestDropMatchesTheTermTheWayTheGlossaryDoes(t *testing.T) {
	root := setCorpus(t)
	if err := glossaryDrop([]string{"-corpus", root, "-write", "  Free A-Module  "}); err != nil {
		t.Fatal(err)
	}
	if g := loaded(t, root); len(g.Terms) != 1 {
		t.Errorf("%d terms left, want the row gone", len(g.Terms))
	}
}

func TestRenderingsSaysWhatIsGoingInEveryLanguage(t *testing.T) {
	got := renderings(glossary.Term{EN: "identity", VI: "đơn vị", ZH: "单位元"})
	if !strings.Contains(got, "vi đơn vị") || !strings.Contains(got, "zh 单位元") {
		t.Errorf("renderings = %q", got)
	}
	if got := renderings(glossary.Term{EN: "identity"}); got != "(no renderings)" {
		t.Errorf("a row with nothing in it reads %q", got)
	}
}
