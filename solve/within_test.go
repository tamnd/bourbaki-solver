package solve

import (
	"strings"
	"testing"
)

// filler is a piece of text of a given size, which is what these tests are
// about. The words do not matter and the lengths do.
func filler(n int, what string) string {
	s := strings.Repeat(what+" ", n/(len(what)+1)+1)
	return s[:n]
}

// wide is a context with one exercise, two siblings, two references at either
// depth, and a § of four statements, sized and cited so that every rung of the
// ranking can be reached by asking for a different limit.
//
// The exercise cites the second sibling and the third statement of its §, which
// is what makes those two worth more than the longer things printed beside them.
func wide() *Context {
	section := "## § 1. THE FIRST SECTION\n\nSome words before the first statement.\n\n" +
		"#### Proposition 1 {#alg-viii-s1-prop-1 .statement tag=0001}\n\n" + filler(3000, "one") + "\n\n" +
		"#### Proposition 2 {#alg-viii-s1-prop-2 .statement tag=0002}\n\n" + filler(2000, "two") + "\n\n" +
		"#### Proposition 3 {#alg-viii-s1-prop-3 .statement tag=0003}\n\n" + filler(1000, "three") + "\n\n" +
		"#### Proposition 4 {#alg-viii-s1-prop-4 .statement tag=0004}\n\n" + filler(500, "four") + "\n"
	return &Context{Label: "alg-viii-s1-ex-3", Tag: "0005", Lang: "en",
		Options: Options{Depth: 2, MaxChars: 40000},
		Cites: map[string]Reach{
			"alg-viii-s1-prop-3": {Depth: 1, Times: 1}, "alg-viii-s1-ex-2": {Depth: 1, Times: 1},
			"alg-viii-s2-prop-1": {Depth: 1, Times: 1}, "alg-viii-s2-prop-9": {Depth: 2, Times: 1}},
		Pieces: []Piece{
			{Kind: TheExercise, Label: "alg-viii-s1-ex-3", Tag: "0005", Text: filler(400, "solve this")},
			{Kind: Sibling, Label: "alg-viii-s1-ex-1", Text: filler(800, "earlier-sibling")},
			{Kind: Sibling, Label: "alg-viii-s1-ex-2", Text: filler(800, "later-sibling")},
			{Kind: TheSection, Label: "alg-viii-s1", Text: section},
			{Kind: Reference, Label: "alg-viii-s2-prop-1", Depth: 1, Text: filler(600, "cited once")},
			{Kind: Reference, Label: "alg-viii-s2-prop-9", Depth: 2, Text: filler(600, "cited twice")},
			{Kind: Outside, Depth: 1, Raw: "Set Theory, III, §3, No. 6, p. 155, Proposition 13"},
		}}
}

func TestAContextThatFitsIsSentWhole(t *testing.T) {
	c := wide()
	if got, want := c.RenderWithin(len(c.Render()), ""), c.Render(); got != want {
		t.Error("a context inside the limit was trimmed")
	}
	if got, want := c.RenderWithin(0, ""), c.Render(); got != want {
		t.Error("a limit of zero trimmed the context")
	}
}

// The longest thing nothing points at goes first, whatever kind of thing it is.
// Dropping a piece takes its text and its heading out and puts a line into the
// block that names it, so the room won is a few dozen characters short of its
// length, which is why the margin here is not one of the sizes in the fixture.
func TestWhatNothingPointsAtGoesFirst(t *testing.T) {
	c := wide()
	out := c.RenderWithin(len(c.Render())-200, "")

	if strings.Contains(out, filler(3000, "one")[:100]) {
		t.Error("the longest statement nothing points at was kept")
	}
	for _, still := range []string{"cited twice", "earlier-sibling", filler(2000, "two")[:100]} {
		if !strings.Contains(out, still) {
			t.Errorf("more than the longest one went: %q is gone", still[:15])
		}
	}
}

// The rule the whole thing turns on. An earlier exercise of the § that the
// exercise never mentions is worth less than one it cites, and a statement of
// the § it cites outlives a longer statement of the same § it does not.
func TestWhatTheExerciseCitesOutlivesWhatItDoesNot(t *testing.T) {
	c := wide()
	out := c.RenderWithin(5000, "")

	if !strings.Contains(out, "later-sibling") {
		t.Error("the sibling the exercise cites was dropped")
	}
	if !strings.Contains(out, filler(1000, "three")[:100]) {
		t.Error("the statement the exercise cites was dropped")
	}
	if strings.Contains(out, "earlier-sibling") {
		t.Error("the sibling nothing points at was kept")
	}
	for _, went := range []string{filler(3000, "one")[:100], filler(2000, "two")[:100]} {
		if strings.Contains(out, went) {
			t.Errorf("a longer statement nothing points at was kept: %q", went[:15])
		}
	}
	if !strings.Contains(out, "## § 1. THE FIRST SECTION") {
		t.Error("thinning the § lost its heading")
	}
}

// The judge's own work outranks everything. What the solution in front of it
// turns on is the last thing to go, over what the exercise cites and over
// anything longer.
func TestWhatTheWorkNamesIsKeptLast(t *testing.T) {
	c := wide()
	out := c.RenderWithin(5500, "the solution turns on alg-viii-s1-prop-1 throughout")

	if !strings.Contains(out, filler(3000, "one")[:100]) {
		t.Error("the statement the solution names was dropped")
	}
	if strings.Contains(out, filler(2000, "two")[:100]) {
		t.Error("a statement nothing names was kept while the named one stood")
	}
}

// A tag is a name too. A solution is asked to cite by tag and a reference
// written from the headings tends to cite by whatever the heading printed.
func TestAStatementNamedByItsTagIsKept(t *testing.T) {
	c := wide()
	out := c.RenderWithin(5500, "by the proposition of tag 0002")

	if !strings.Contains(out, filler(2000, "two")[:100]) {
		t.Error("the statement named by its tag was dropped")
	}
}

// A context is never quietly smaller than it looks. A whole piece that went is
// named in the block that already exists for saying what is in the corpus and
// is not in front of you, and a statement that went leaves its name where it
// stood.
func TestWhatWasDroppedIsNamed(t *testing.T) {
	c := wide()
	out := c.RenderWithin(5000, "")

	if !strings.Contains(out, "alg-viii-s1-prop-1, tag 0001, is a statement of this §") {
		t.Errorf("the statement that went left nothing where it stood:\n%s", tail(out))
	}
	if !strings.Contains(out, "alg-viii-s1-ex-1") {
		t.Error("the dropped sibling was not named")
	}
	if !strings.Contains(out, OverAsk.Sentence(40000)) {
		t.Errorf("the reason was not given:\n%s", tail(out))
	}
}

// The exercise itself is not negotiable. Asked for a limit nothing will fit in,
// the trimming gives back the smallest honest question it can rather than an
// exercise with its own statement cut out of it.
func TestTheExerciseIsNeverDropped(t *testing.T) {
	c := wide()
	out := c.RenderWithin(10, "")

	if !strings.Contains(out, "solve this") {
		t.Errorf("the exercise went:\n%s", out)
	}
	if strings.Contains(out, filler(3000, "one")[:100]) {
		t.Error("a statement of the § survived a limit of ten characters")
	}
	if strings.Contains(out, "later-sibling") {
		t.Error("a sibling survived a limit of ten characters")
	}
}

// tail is the end of a render, for a failure message that would otherwise be
// forty thousand characters of filler.
func tail(s string) string {
	if len(s) > 1200 {
		return "..." + s[len(s)-1200:]
	}
	return s
}
