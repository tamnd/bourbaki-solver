package roundtrip

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestAJudgeThatFoundNothingSaysSo(t *testing.T) {
	same, diffs, err := ParseJudgement(`{"same": true, "differences": []}`)
	if err != nil {
		t.Fatal(err)
	}
	if !same || len(diffs) != 0 {
		t.Errorf("same=%v with %d differences", same, len(diffs))
	}
}

func TestAJudgeAnswerInAFenceIsRead(t *testing.T) {
	// A model asked for JSON hands back a fence about half the time, and
	// refusing that would throw away good answers over punctuation.
	answer := "```json\n{\"same\": false, \"differences\": [{\"kind\": \"hypothesis\", \"english\": \"Let A be a commutative ring\", \"back\": \"Let A be a ring\", \"why\": \"commutative is gone\"}]}\n```"
	same, diffs, err := ParseJudgement(answer)
	if err != nil {
		t.Fatal(err)
	}
	if same {
		t.Error("a dropped hypothesis came back as the same mathematics")
	}
	if len(diffs) != 1 || diffs[0].Kind != KindHypothesis {
		t.Fatalf("got %+v", diffs)
	}
}

func TestTheDifferencesBeatTheSummary(t *testing.T) {
	// A judge that lists a dropped hypothesis and then says the two are the
	// same has answered the summary carelessly. Taking the summary would throw
	// away the evidence in favour of the sentence about it.
	same, diffs, err := ParseJudgement(
		`{"same": true, "differences": [{"kind": "quantifier", "english": "for every x", "back": "for some x", "why": "every became some"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if same {
		t.Error("same stayed true with a difference under it")
	}
	if len(diffs) != 1 {
		t.Fatalf("got %d differences", len(diffs))
	}
}

func TestAKindNobodyKnowsKeepsTheFinding(t *testing.T) {
	_, diffs, err := ParseJudgement(
		`{"same": false, "differences": [{"kind": "vibes", "english": "a", "back": "b", "why": "c"}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if len(diffs) != 1 {
		t.Fatalf("the finding was dropped with its label, %d left", len(diffs))
	}
	if diffs[0].Kind != KindOther {
		t.Errorf("kind is %q and not other", diffs[0].Kind)
	}
	if diffs[0].Why != "c" {
		t.Errorf("the reason was lost: %q", diffs[0].Why)
	}
}

func TestAnEmptyDifferenceIsNotCounted(t *testing.T) {
	// The model filling the shape of the answer rather than reporting anything.
	// Counting it would put a file in the differing column over an empty object.
	same, diffs, err := ParseJudgement(
		`{"same": true, "differences": [{"kind": "", "english": "", "back": "", "why": ""}]}`)
	if err != nil {
		t.Fatal(err)
	}
	if !same || len(diffs) != 0 {
		t.Errorf("same=%v with %d differences", same, len(diffs))
	}
}

func TestAJudgeThatWroteProseIsAnError(t *testing.T) {
	if _, _, err := ParseJudgement("The two passages agree closely, though the second is briefer."); err == nil {
		t.Error("prose was read as a verdict")
	}
}

func TestAnAnswerWithNoVerdictIsAnError(t *testing.T) {
	if _, _, err := ParseJudgement(`{"differences": []}`); err == nil {
		t.Error("an answer holding no same field was read as a verdict")
	}
}

func TestASilentJudgeIsAnError(t *testing.T) {
	if _, _, err := ParseJudgement("   \n  "); err == nil {
		t.Error("an empty answer was read as a verdict")
	}
}

func TestAVerdictSurvivesTheRoundTripToDisk(t *testing.T) {
	root := t.TempDir()
	r := &Results{Rate: Rate}
	r.Put(Verdict{Path: "content/vi/alg/VIII/01.md", English: "content/en/alg/VIII/01.md",
		Lang: "vi", Digest: "abc", Same: false,
		Differences: []Difference{{Kind: KindNumber, English: "§ 3", Back: "§ 2", Why: "the number moved"}}})
	if err := r.Save(root); err != nil {
		t.Fatal(err)
	}
	back, err := LoadResults(root)
	if err != nil {
		t.Fatal(err)
	}
	v := back.Find("vi", "content/vi/alg/VIII/01.md")
	if v == nil {
		t.Fatal("the verdict is not there")
	}
	if v.Same || len(v.Differences) != 1 || v.Differences[0].Kind != KindNumber {
		t.Errorf("came back as %+v", v)
	}
}

func TestNoResultsFileIsNotAnError(t *testing.T) {
	r, err := LoadResults(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if len(r.Verdicts) != 0 {
		t.Errorf("a checkout that has judged nothing produced %d verdicts", len(r.Verdicts))
	}
}

func TestABrokenResultsFileIsAnError(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "reports"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(ResultsPath(root), []byte("{not json"), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadResults(root); err == nil {
		t.Error("a file that will not parse was read as no verdicts")
	}
}

func TestPutReplacesTheVerdictOnTheSameFile(t *testing.T) {
	r := &Results{}
	r.Put(Verdict{Lang: "vi", Path: "a.md", Digest: "one", Same: true})
	r.Put(Verdict{Lang: "vi", Path: "a.md", Digest: "two", Same: false})
	if len(r.Verdicts) != 1 {
		t.Fatalf("%d verdicts on one file", len(r.Verdicts))
	}
	if r.Verdicts[0].Digest != "two" {
		t.Errorf("the old verdict stayed: %+v", r.Verdicts[0])
	}
}

func TestAFileTranslatedAgainIsStale(t *testing.T) {
	r := &Results{}
	it := Item{Lang: "vi", Path: "a.md", Digest: "one"}
	if !r.Stale(it) {
		t.Error("a file nobody has judged is not stale")
	}
	r.Put(Verdict{Lang: "vi", Path: "a.md", Digest: "one", Same: true})
	if r.Stale(it) {
		t.Error("a judged file is stale")
	}
	it.Digest = "two"
	if !r.Stale(it) {
		t.Error("a file translated again since it was judged is not stale")
	}
}

func TestTheTallyKeepsJudgedAndWaitingApart(t *testing.T) {
	// A run that judged two of forty and a run that judged forty of forty can
	// print the same percentage and they are not the same claim.
	sample := []Item{
		{Lang: "vi", Path: "a.md", Digest: "1"},
		{Lang: "vi", Path: "b.md", Digest: "2"},
		{Lang: "vi", Path: "c.md", Digest: "3"},
		{Lang: "zh", Path: "d.md", Digest: "4"},
	}
	r := &Results{}
	r.Put(Verdict{Lang: "vi", Path: "a.md", Digest: "1", Same: true})
	r.Put(Verdict{Lang: "vi", Path: "b.md", Digest: "1", Same: true}) // stale
	r.Put(Verdict{Lang: "vi", Path: "c.md", Digest: "3", Same: false,
		Differences: []Difference{{Kind: KindStatement}, {Kind: KindOmission}}})
	got := Tally(sample, r)
	if len(got) != 2 {
		t.Fatalf("%d languages in the tally", len(got))
	}
	vi := got[0]
	if vi.Lang != "vi" || vi.Sampled != 3 || vi.Judged != 2 || vi.Stale != 1 ||
		vi.Same != 1 || vi.Differing != 1 || vi.Differences != 2 {
		t.Errorf("vi came out as %+v", vi)
	}
	zh := got[1]
	if zh.Judged != 0 || zh.Stale != 1 {
		t.Errorf("zh came out as %+v", zh)
	}
	if line := zh.Line(); !strings.Contains(line, "nothing measured yet") {
		t.Errorf("a language with no verdicts reads %q", line)
	}
}
