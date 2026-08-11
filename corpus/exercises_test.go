package corpus

import (
	"slices"
	"strings"
	"testing"
)

// A gap is the one thing this manifest exists to catch: Bourbaki numbers the
// exercises of a § from one straight through, so a number nothing carries means
// a page was lost or a split came apart.
func TestGaps(t *testing.T) {
	cases := []struct {
		numbers []int
		want    []int
	}{
		{nil, nil},
		{[]int{1, 2, 3}, nil},
		{[]int{1}, nil},
		{[]int{1, 2, 4, 5}, []int{3}},
		{[]int{1, 4}, []int{2, 3}},
	}
	for _, c := range cases {
		if got := Gaps(c.numbers); !slices.Equal(got, c.want) {
			t.Errorf("Gaps(%v) = %v, want %v", c.numbers, got, c.want)
		}
	}
}

func TestExercisesRoundTrip(t *testing.T) {
	root := t.TempDir()
	m, err := LoadExercises(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Books) != 0 {
		t.Fatalf("a fresh repo loaded %+v", m)
	}
	m.Upsert(BookExercises{ID: "alg-viii", Chapters: []ChapterExercises{{
		Chapter: "VIII", Title: "Semisimple Modules and Rings", Total: 30,
		Section: []SectionExercise{
			{Section: 1, Label: "alg-viii-s1", Dir: "s1", Count: 28, First: 1, Last: 28, Starred: 3},
			{Section: 1, Appendix: true, Label: "alg-viii-a1", Dir: "a1", Count: 2, First: 1, Last: 2},
		},
	}}})
	if err := m.Save(root); err != nil {
		t.Fatal(err)
	}
	back, err := LoadExercises(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(back.Books) != 1 || len(back.Books[0].Chapters) != 1 {
		t.Fatalf("read back %+v", back)
	}
	secs := back.Books[0].Chapters[0].Section
	if len(secs) != 2 || secs[0].Count != 28 || !secs[1].Appendix || secs[1].Dir != "a1" {
		t.Errorf("read back %+v", secs)
	}

	// Upsert replaces a volume rather than appending a second record of it.
	back.Upsert(BookExercises{ID: "alg-viii"})
	back.Upsert(BookExercises{ID: "alg-i-iii"})
	if len(back.Books) != 2 || len(back.Books[0].Chapters) != 0 {
		t.Errorf("after upsert the manifest is %+v", back.Books)
	}

	b, err := m.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasSuffix(string(b), "\n") || !strings.Contains(string(b), "\n  ") {
		t.Error("the manifest is written as one line and will diff as one line")
	}
}
