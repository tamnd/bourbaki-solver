package roundtrip

import (
	"fmt"
	"testing"
)

// items builds n Vietnamese files with predictable paths.
func items(lang string, n int) []Item {
	out := make([]Item, 0, n)
	for i := 0; i < n; i++ {
		out = append(out, Item{
			Path:    fmt.Sprintf("content/%s/alg/VIII/%02d_section.md", lang, i),
			English: fmt.Sprintf("content/en/alg/VIII/%02d_section.md", i),
			Lang:    lang,
			Digest:  fmt.Sprintf("digest%02d", i),
		})
	}
	return out
}

func TestTheDrawIsTheSameEveryTime(t *testing.T) {
	in := items("vi", 400)
	first := Draw(in, Rate)
	for i := 0; i < 5; i++ {
		again := Draw(in, Rate)
		if len(again) != len(first) {
			t.Fatalf("draw %d took %d files where the first took %d", i, len(again), len(first))
		}
		for j := range first {
			if again[j].Path != first[j].Path {
				t.Fatalf("draw %d differs at %d: %s against %s", i, j, again[j].Path, first[j].Path)
			}
		}
	}
}

func TestTheDrawDoesNotMoveWhenAFileIsTranslatedAgain(t *testing.T) {
	in := items("vi", 200)
	before := Draw(in, Rate)
	// Every body changes, which is what a retranslation of the whole tree looks
	// like. Membership must not follow the work around, or two runs report on
	// two different populations and neither number can be compared with the
	// other.
	for i := range in {
		in[i].Digest = "changed"
	}
	after := Draw(in, Rate)
	if len(before) != len(after) {
		t.Fatalf("the sample went from %d files to %d because the bodies changed", len(before), len(after))
	}
	for i := range before {
		if before[i].Path != after[i].Path {
			t.Fatalf("the sample moved at %d: %s became %s", i, before[i].Path, after[i].Path)
		}
	}
}

func TestTheDrawIsAboutTheRateAsked(t *testing.T) {
	in := items("vi", 2000)
	got := len(Draw(in, 0.05))
	if got < 80 || got > 120 {
		t.Errorf("5%% of 2000 came out as %d, which is not about a hundred", got)
	}
}

func TestTheLanguagesDrawIndependently(t *testing.T) {
	var in []Item
	for _, l := range []string{"vi", "zh", "ja"} {
		in = append(in, items(l, 400)...)
	}
	pick := map[string]map[string]bool{}
	for _, it := range Draw(in, Rate) {
		if pick[it.Lang] == nil {
			pick[it.Lang] = map[string]bool{}
		}
		// The path with the language taken out, so the three can be compared.
		pick[it.Lang][it.Path[len("content/vi"):]] = true
	}
	if len(pick) != 3 {
		t.Fatalf("the draw covered %d languages and not 3", len(pick))
	}
	same := 0
	for p := range pick["vi"] {
		if pick["zh"][p] {
			same++
		}
	}
	if same == len(pick["vi"]) && len(pick["vi"]) > 1 {
		t.Error("Vietnamese and Chinese drew exactly the same sections, which is one draw counted twice")
	}
}

func TestASmallLanguageStillGetsAFile(t *testing.T) {
	// Five per cent of three files is nought point one five. A rate that rounds
	// to nothing would print a sampling rate over a sample that measured
	// nothing, which reads like a pass.
	in := items("ja", 3)
	got := Draw(in, Rate)
	if len(got) != 1 {
		t.Fatalf("3 files at 5%% gave %d, and the floor is 1", len(got))
	}
	again := Draw(in, Rate)
	if again[0].Path != got[0].Path {
		t.Errorf("the floor picked %s and then %s, so it is not the same file every time", got[0].Path, again[0].Path)
	}
}

func TestOneFileIsSampled(t *testing.T) {
	got := Draw(items("vi", 1), Rate)
	if len(got) != 1 {
		t.Fatalf("a corpus of one translation sampled %d files", len(got))
	}
}

func TestNoTranslationsDrawsNothing(t *testing.T) {
	if got := Draw(nil, Rate); got != nil {
		t.Errorf("an empty corpus drew %v", got)
	}
}

func TestARateOfNoughtDrawsNothing(t *testing.T) {
	// Nought is a person asking for no sample, which is different from a small
	// sample, so the floor must not fire.
	if got := Draw(items("vi", 100), 0); len(got) != 0 {
		t.Errorf("a rate of 0 drew %d files", len(got))
	}
}

func TestARateOfOneDrawsEverything(t *testing.T) {
	in := items("vi", 50)
	if got := Draw(in, 1); len(got) != len(in) {
		t.Errorf("a rate of 1 drew %d of %d", len(got), len(in))
	}
}

func TestTheSampleComesBackSorted(t *testing.T) {
	var in []Item
	for _, l := range []string{"zh", "vi"} {
		in = append(in, items(l, 200)...)
	}
	got := Draw(in, 0.5)
	for i := 1; i < len(got); i++ {
		a, b := got[i-1], got[i]
		if a.Lang > b.Lang || (a.Lang == b.Lang && a.Path >= b.Path) {
			t.Fatalf("out of order at %d: %s %s then %s %s", i, a.Lang, a.Path, b.Lang, b.Path)
		}
	}
}
