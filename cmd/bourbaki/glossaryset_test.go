package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/bourbaki-solver/glossary"
)

func setCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	g := &glossary.Glossary{Version: 11, Terms: []glossary.Term{
		{EN: "free A-module", VI: "môđun tự do trên a", ZH: "自由a-模"},
		{EN: "simple", VI: "đơn", ZH: "单", Note: "read by hand after a run got it wrong"},
	}}
	if err := g.Write(glossary.Path(root)); err != nil {
		t.Fatal(err)
	}
	return root
}

func loaded(t *testing.T, root string) *glossary.Glossary {
	t.Helper()
	g, err := glossary.Load(root)
	if err != nil {
		t.Fatal(err)
	}
	return g
}

// The whole point: a person who has read a rendering and knows it is wrong can
// put it right without opening the YAML.
func TestSetWritesTheRenderingAndMovesTheVersion(t *testing.T) {
	root := setCorpus(t)
	if err := glossarySet([]string{"-corpus", root, "-lang", "vi", "-write",
		"-note", "The corpus writes the ring in front, A-môđun, and never môđun trên A.",
		"free A-module=A-môđun tự do"}); err != nil {
		t.Fatal(err)
	}
	g := loaded(t, root)
	if got := g.Terms[0].VI; got != "A-môđun tự do" {
		t.Fatalf("vi = %q", got)
	}
	if g.Version != 12 {
		t.Errorf("version = %d, want it moved, since a file written against the old rendering is stale", g.Version)
	}
	if g.Terms[0].Note == "" {
		t.Error("the reason was not kept, and a row nobody can see the reason for gets undone by the next pass")
	}
	// One language at a time. Setting the Vietnamese must not disturb what the
	// Chinese pass decided.
	if g.Terms[0].ZH != "自由a-模" {
		t.Errorf("zh = %q, want it left alone", g.Terms[0].ZH)
	}
}

// Nothing is written without -write, the way tidy behaves, because this is a
// file three languages of translation are held to.
func TestSetWritesNothingWithoutTheFlag(t *testing.T) {
	root := setCorpus(t)
	before, err := os.ReadFile(glossary.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if err := glossarySet([]string{"-corpus", root, "-lang", "vi", "free A-module=A-môđun tự do"}); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(glossary.Path(root))
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("the file changed without -write")
	}
}

// A term the glossary does not have is a typo, and a typo that lands as a row
// is a row nobody ever looks at again.
func TestSetRefusesATermThatIsNotThere(t *testing.T) {
	root := setCorpus(t)
	err := glossarySet([]string{"-corpus", root, "-lang", "vi", "-write", "fre A-module=A-môđun tự do"})
	if err == nil {
		t.Fatal("a term that is not in the glossary was accepted")
	}
	if len(loaded(t, root).Terms) != 2 {
		t.Error("a row was added")
	}
}

// A run that names one term wrong changes nothing at all. Half an editorial
// pass leaves the file holding part of a decision with no record of which part.
func TestOneBadTermLeavesTheWholeRunUnwritten(t *testing.T) {
	root := setCorpus(t)
	err := glossarySet([]string{"-corpus", root, "-lang", "vi", "-write",
		"free A-module=A-môđun tự do", "semisimple=nửa đơn"})
	if err == nil {
		t.Fatal("the run was accepted with a term that is not there")
	}
	if got := loaded(t, root).Terms[0].VI; got != "môđun tự do trên a" {
		t.Errorf("vi = %q, want the good half not written either", got)
	}
}

// Running the same correction twice is not a second version, and a version that
// moves for nothing restales every file that mentions the term.
func TestSettingWhatIsAlreadyThereIsNotAChange(t *testing.T) {
	root := setCorpus(t)
	args := []string{"-corpus", root, "-lang", "vi", "-write", "free A-module=A-môđun tự do"}
	if err := glossarySet(args); err != nil {
		t.Fatal(err)
	}
	if err := glossarySet(args); err != nil {
		t.Fatal(err)
	}
	if got := loaded(t, root).Version; got != 12 {
		t.Errorf("version = %d after setting the same thing twice", got)
	}
}

// The reason this exists at all. A note with a colon and a space in it is not a
// plain YAML scalar, and writing one by hand stopped the file parsing once
// already.
func TestANoteWithAColonInItStillParses(t *testing.T) {
	root := setCorpus(t)
	note := "Corrected by hand: the run answered 简单, uncomplicated, where the book means 单."
	if err := glossarySet([]string{"-corpus", root, "-lang", "zh", "-write", "-note", note,
		"free A-module=自由A-模"}); err != nil {
		t.Fatal(err)
	}
	// Load parses with KnownFields, which is what the audit and every run do.
	if got := loaded(t, root).Terms[0].Note; got != note {
		t.Errorf("note came back as %q", got)
	}
	raw, err := os.ReadFile(filepath.Join(root, "manifests", "glossary.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := glossary.Parse(raw, "glossary.yaml"); err != nil {
		t.Fatalf("the file does not parse: %v", err)
	}
}

// Writing down why a row reads the way it does is an edit in its own right, and
// the first version of this refused it, because the rendering was already what
// it was being set to. It is also not a new version: a note never reaches a
// model, so nothing translated against this file is stale because of one.
func TestANoteOnARowThatIsAlreadyRightIsStillAnEdit(t *testing.T) {
	root := setCorpus(t)
	args := []string{"-corpus", root, "-lang", "vi", "-write",
		"-note", "The ring is written in front and its letter is a capital.",
		"free A-module=môđun tự do trên a"}
	if err := glossarySet(args); err != nil {
		t.Fatal(err)
	}
	g := loaded(t, root)
	if g.Terms[0].Note == "" {
		t.Fatal("the note was refused because the rendering was already right")
	}
	if g.Version != 11 {
		t.Errorf("version = %d, want it left alone, since a note stales nothing", g.Version)
	}
	// And the same note twice is nothing at all.
	if err := glossarySet(args); err != nil {
		t.Fatal(err)
	}
	if got := loaded(t, root).Version; got != 11 {
		t.Errorf("version = %d after writing the same note twice", got)
	}
}
