package tags

import (
	"os"
	"strings"
	"testing"
)

// A merge opens a run at the lowest tag it made permanent, and the file it goes
// to reads back as it was written.
func TestARunIsRecordedAndReadBack(t *testing.T) {
	root := t.TempDir()
	s := &Set{}
	s.Open("0001", "2026-08-11", "the English of Algebra chapter VIII")
	s.Open("00QV", "2026-08-11", "the 57 statements the reference graph found missing")
	if err := s.Save(root); err != nil {
		t.Fatal(err)
	}
	back, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Runs) != 2 || back.Runs[0].First != "0001" || back.Runs[1].First != "00QV" {
		t.Fatalf("the runs read back as %v", back.Runs)
	}
	if back.Runs[1].Note != "the 57 statements the reference graph found missing" {
		t.Errorf("the note reads %q", back.Runs[1].Note)
	}
	b, err := os.ReadFile(Path(root, RunsFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(b), "FIRST_TAG,DATE") {
		t.Errorf("the file went out without its header:\n%s", b)
	}
}

// A second merge of the same reading is the same run. It happens when somebody
// assigns, merges, finds one statement missed and assigns again, and the two of
// them read the file from the top once between them.
func TestTheSameBoundaryIsNotRecordedTwice(t *testing.T) {
	s := &Set{}
	s.Open("0001", "2026-08-11", "the volume")
	s.Open("0001", "2026-08-12", "the volume again")
	if len(s.Runs) != 1 || s.Runs[0].Date != "2026-08-11" {
		t.Errorf("the runs are %v, want the first one alone", s.Runs)
	}
}

// A boundary inside a run already recorded is a second reading and gets a line.
// This is how the corpus is: five of its eight merges went over a volume from
// the top and then went over it again, and until both readings were written
// down here T10 read the two of them as one file whose tags do not climb.
func TestAReadingInsideARecordedRunIsARunOfItsOwn(t *testing.T) {
	s := &Set{}
	s.Open("03G1", "2026-08-16", "Theory of Sets chapters I to III")
	s.Open("03V4", "2026-08-16", "Theory of Sets chapter IV")
	s.Open("03P6", "2026-08-16", "Theory of Sets read from the top a second time")
	if len(s.Runs) != 3 || s.Runs[1].First != "03P6" {
		t.Fatalf("the runs are %v, want the second reading between the two merges", s.Runs)
	}
	// The two sides of the new boundary answer differently, which is the whole
	// of what recording it buys.
	if RunAt(s.Runs, "03P0") == RunAt(s.Runs, "03PG") {
		t.Error("a tag either side of the second reading still reads as one run")
	}
}

// The file is comma separated and a note is prose somebody types, so a comma in
// it would take the line apart on the way back in.
func TestACommaInANoteDoesNotSplitTheLine(t *testing.T) {
	root := t.TempDir()
	s := &Set{}
	s.Open("0001", "2026-08-11", "Algebra VIII, both printings")
	if err := s.Save(root); err != nil {
		t.Fatal(err)
	}
	back, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Runs) != 1 || back.Runs[0].Note != "Algebra VIII both printings" {
		t.Fatalf("the runs read back as %v", back.Runs)
	}
}

// A line that is not three fields is a file somebody edited by hand into
// something this cannot read, and it stops rather than guessing at a boundary.
func TestARunLineOfTheWrongShapeIsAnError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(root, RunsFile), []byte("0001,2026-08-11\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Error("a line with no note was read as a run")
	}
}
