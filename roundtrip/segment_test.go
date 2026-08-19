package roundtrip

import (
	"strings"
	"testing"
)

func TestTheParagraphsArePairedInOrder(t *testing.T) {
	en := "First paragraph.\n\nSecond paragraph.\n\nThird paragraph."
	back := "One.\n\nTwo.\n\nThree."
	got, err := Segments(en, back, 10000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("%d questions for three short paragraphs", len(got))
	}
	if got[0].English != en || got[0].Back != back {
		t.Errorf("the whole thing did not come through: %+v", got[0])
	}
}

func TestALongFileIsCutAtParagraphs(t *testing.T) {
	// The cut has to fall between paragraphs. A cut by character count puts the
	// end of one paragraph beside the start of another and the judge reports
	// every seam as an omission.
	var eb, bb []string
	for i := 0; i < 20; i++ {
		eb = append(eb, strings.Repeat("a", 500))
		bb = append(bb, strings.Repeat("b", 500))
	}
	en, back := strings.Join(eb, "\n\n"), strings.Join(bb, "\n\n")
	got, err := Segments(en, back, 4000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) < 4 {
		t.Fatalf("20,000 characters at a budget of 4,000 came to %d questions", len(got))
	}
	blocks := 0
	for i, s := range got {
		if s.Index != i+1 || s.Of != len(got) {
			t.Errorf("question %d is numbered %d of %d", i, s.Index, s.Of)
		}
		e := strings.Split(s.English, "\n\n")
		b := strings.Split(s.Back, "\n\n")
		if len(e) != len(b) {
			t.Errorf("question %d has %d English paragraphs against %d", i, len(e), len(b))
		}
		for _, p := range e {
			if p != strings.Repeat("a", 500) {
				t.Fatalf("question %d cut inside a paragraph", i)
			}
		}
		blocks += len(e)
	}
	if blocks != 20 {
		t.Errorf("%d paragraphs went to the judge and there are 20", blocks)
	}
}

func TestAParagraphLongerThanTheBudgetGoesOnItsOwn(t *testing.T) {
	// The chunker lets a single long block go over on its own, and so does
	// this: splitting inside a paragraph to meet a number would give back the
	// alignment the whole function is for.
	en := "short.\n\n" + strings.Repeat("x", 9000)
	back := "brief.\n\n" + strings.Repeat("y", 9000)
	got, err := Segments(en, back, 1000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("%d questions, expected the short one then the long one alone", len(got))
	}
	if len(got[1].English) != 9000 {
		t.Errorf("the long paragraph was cut to %d characters", len(got[1].English))
	}
}

func TestABackTranslationWithTheWrongParagraphCountIsRefused(t *testing.T) {
	// Nothing sound can be done with it here. Judging the misaligned text would
	// produce a page of findings about paragraphs that were never compared with
	// their own counterparts, and a page of invented differences is worse than
	// an honest gap.
	en := "One.\n\nTwo.\n\nThree."
	back := "One and two together.\n\nThree."
	_, err := Segments(en, back, 10000)
	if err == nil {
		t.Fatal("a back translation that merged two paragraphs was judged anyway")
	}
	if !strings.Contains(err.Error(), "3 blocks") || !strings.Contains(err.Error(), "2") {
		t.Errorf("the error does not say what the counts were: %v", err)
	}
}

func TestAnEmptyPassageIsRefused(t *testing.T) {
	if _, err := Segments("", "", 10000); err == nil {
		t.Error("an empty passage was segmented")
	}
}

func TestNoBudgetTakesTheDefault(t *testing.T) {
	en := strings.Repeat("a", 100)
	got, err := Segments(en, strings.Repeat("b", 100), 0)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Errorf("%d questions for one short paragraph", len(got))
	}
}
