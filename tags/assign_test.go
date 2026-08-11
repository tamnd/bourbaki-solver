package tags

import (
	"os"
	"strings"
	"testing"
)

func TestAssignAllocatesInOrderAndOnlyOnce(t *testing.T) {
	s := &Set{}
	made, err := s.Assign([]string{"alg-viii-s1-def-1", "alg-viii-s1-prop-1", "alg-viii-s1-ex-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(made) != 3 || made[0].Tag != "0001" || made[2].Tag != "0003" {
		t.Fatalf("assigned %+v", made)
	}
	// The second run over the same corpus allocates nothing, which is the whole
	// contract: a tag is handed out once and then it is that statement's.
	again, err := s.Assign([]string{"alg-viii-s1-def-1", "alg-viii-s1-prop-1", "alg-viii-s1-ex-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(again) != 0 {
		t.Errorf("a second run allocated %+v", again)
	}
	// A statement added to the middle of a § takes the next free tag, not the
	// tag of its neighbour. Order of allocation says nothing about order in the
	// book and is not meant to.
	made, err = s.Assign([]string{"alg-viii-s1-def-1", "alg-viii-s1-lem-1", "alg-viii-s1-prop-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(made) != 1 || made[0].Tag != "0004" || made[0].Label != "alg-viii-s1-lem-1" {
		t.Errorf("assigned %+v", made)
	}
}

// A retired tag is burned. Somewhere there is a citation pointing at it, and
// handing it to a different statement would silently make that citation wrong,
// which is worse than a dead link.
func TestAssignNeverReusesARetiredTag(t *testing.T) {
	s := &Set{
		Tags:     []Entry{{Tag: "0001", Label: "alg-viii-s1-def-1"}},
		Inactive: []Retired{{Tag: "0002", Label: "alg-viii-s1-rem-9", Reason: "a misread page", Date: "2026-08-11"}},
	}
	made, err := s.Assign([]string{"alg-viii-s1-def-1", "alg-viii-s1-prop-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(made) != 1 || made[0].Tag != "0003" {
		t.Errorf("assigned %+v, want 0003: 0002 is burned", made)
	}
}

// A renamed statement keeps its tag. Without this an alias would cost a second
// tag on the same statement, which is exactly what the file exists to prevent.
func TestAssignFollowsAnAlias(t *testing.T) {
	s := &Set{
		Tags:    []Entry{{Tag: "0001", Label: "alg-viii-s1-rem-1"}},
		Aliases: []Alias{{Old: "alg-viii-s1-rem-1", New: "alg-viii-s1-n1-rem-1"}},
	}
	made, err := s.Assign([]string{"alg-viii-s1-n1-rem-1"})
	if err != nil {
		t.Fatal(err)
	}
	if len(made) != 0 {
		t.Fatalf("the renamed statement was given a second tag: %+v", made)
	}
	if got := s.Lookup()["alg-viii-s1-n1-rem-1"]; got != "0001" {
		t.Errorf("the new label resolves to %q, want 0001", got)
	}
	if done := s.Migrate(); len(done) != 1 || s.Tags[0].Label != "alg-viii-s1-n1-rem-1" {
		t.Errorf("after migrate the record is %+v", s.Tags)
	}
}

func TestMergeMakesTagsPermanent(t *testing.T) {
	s := &Set{New: []Entry{{Tag: "0002", Label: "b"}, {Tag: "0001", Label: "a"}}}
	n, err := s.Merge()
	if err != nil {
		t.Fatal(err)
	}
	if n != 2 || len(s.New) != 0 || len(s.Tags) != 2 {
		t.Fatalf("after merge %d new, %+v", n, s)
	}
	// tags is sorted by tag, so an append to the corpus is an append to the
	// file and the diff of a release reads as the list of what it added.
	if s.Tags[0].Tag != "0001" || s.Tags[1].Tag != "0002" {
		t.Errorf("tags came out in the order %+v", s.Tags)
	}
	s.New = []Entry{{Tag: "0003", Label: "a"}}
	if _, err := s.Merge(); err == nil {
		t.Error("a second tag on a label that already has one was merged")
	}
}

func TestRetire(t *testing.T) {
	s := &Set{Tags: []Entry{{Tag: "0001", Label: "a"}, {Tag: "0002", Label: "b"}}}
	tag, err := s.Retire("b", "the page was misread and there is no such remark", "2026-08-11")
	if err != nil {
		t.Fatal(err)
	}
	if tag != "0002" || len(s.Tags) != 1 || len(s.Inactive) != 1 {
		t.Fatalf("after retire %+v", s)
	}
	if _, err := s.Retire("a", "", "2026-08-11"); err == nil {
		t.Error("a tag was retired with no reason given")
	}
	if _, err := s.Retire("a", "one, two", "2026-08-11"); err == nil {
		t.Error("a reason with a comma in it was written to a comma separated file")
	}
	if _, err := s.Retire("nothing", "gone", "2026-08-11"); err == nil {
		t.Error("a label no tag holds was retired")
	}
}

func TestSaveRoundTrip(t *testing.T) {
	root := t.TempDir()
	s := &Set{
		Tags:     []Entry{{Tag: "0002", Label: "b"}, {Tag: "0001", Label: "a"}},
		New:      []Entry{{Tag: "0004", Label: "d"}},
		Inactive: []Retired{{Tag: "0003", Label: "c", Reason: "a misread page", Date: "2026-08-11"}},
		Aliases:  []Alias{{Old: "a", New: "aa"}},
	}
	if err := s.Save(root); err != nil {
		t.Fatal(err)
	}
	back, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Tags) != 2 || back.Tags[0].Tag != "0001" || len(back.New) != 1 ||
		len(back.Inactive) != 1 || len(back.Aliases) != 1 {
		t.Fatalf("read back %+v", back)
	}
	b, err := os.ReadFile(Path(root, TagsFile))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(b), "# List of current tags") {
		t.Errorf("tags has no header:\n%s", b)
	}

	// All four files stay on disk whether or not there is anything in them, so
	// that a reader looking for where a retired tag goes finds the file rather
	// than being told it will appear one day.
	if _, err := back.Merge(); err != nil {
		t.Fatal(err)
	}
	if err := back.Save(root); err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{TagsFile, NewTagsFile, InactiveFile, AliasesFile} {
		if _, err := os.Stat(Path(root, name)); err != nil {
			t.Errorf("%s is gone: %v", name, err)
		}
	}
	if b, err := os.ReadFile(Path(root, NewTagsFile)); err != nil || len(b) != 0 {
		t.Errorf("new-tags holds %q after a merge emptied it", b)
	}
}

// The header of a committed file is prose somebody wrote for the next reader.
// Rewriting it on every run would also make tags look like a file that had
// lines taken out of it, which is the one thing it may never look like.
func TestSaveKeepsTheHeaderItFound(t *testing.T) {
	root := t.TempDir()
	head := "# A header of our own\n# on two lines\n"
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(root, TagsFile), []byte(head+"0001,a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	s, err := Load(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.Assign([]string{"a", "b"}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Merge(); err != nil {
		t.Fatal(err)
	}
	if err := s.Save(root); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(Path(root, TagsFile))
	if err != nil {
		t.Fatal(err)
	}
	if string(b) != head+"0001,a\n0002,b\n" {
		t.Errorf("tags came out as\n%s", b)
	}
}

func TestLoadRefusesAMalformedFile(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(Dir(root), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(Path(root, TagsFile), []byte("0001,a\nnot-a-line\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(root); err == nil {
		t.Fatal("a line that is not tag,full_label was accepted")
	}
}

// Two statements at one address is a mistake in assembly. Minting a permanent
// identifier over the top of it would make the mistake permanent as well.
func TestAssignRefusesARepeatedLabel(t *testing.T) {
	s := &Set{}
	if _, err := s.Assign([]string{"a", "b", "a"}); err == nil {
		t.Fatal("a label that is on two things in the corpus was assigned one tag")
	}
	if len(s.New) != 0 {
		t.Errorf("the failed run left %+v behind", s.New)
	}
}
