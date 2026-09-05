package solve

import (
	"fmt"
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
	// This asserted the reverse, that a limit of zero was no limit and sent the
	// context whole. Nothing asked for it that way: all four callers work the
	// room out as the limit less what the rest of the question already takes,
	// and the engine returns Render itself for an unlimited run before a room is
	// ever computed. What the branch actually served was the truth judge, whose
	// question carries a reference and a solution and so can leave a room of
	// nothing, being handed the whole context at the tightest moment there is.
	if got := c.RenderWithin(0, ""); len(got) >= len(c.Render()) {
		t.Error("a room of nothing was sent the context whole")
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

// A solution cites the way the book prints it, and the trimming has to hear
// that. Exercise 6 of § 1 is why: it proves d) out of Definition 1 and
// Definition 2, names neither by label nor by tag, and both were dropped.
func TestTheTrimmingHearsTheBooksOwnWayOfCiting(t *testing.T) {
	s := span{label: "alg-viii-s1-def-1", tag: "0001", name: "Definition 1"}
	for _, cited := range []string{
		"By Definition 2 of § 1 and Definition 1(ii), the sequence is stationary.",
		"the tag is alg-viii-s1-def-1",
		"cited as 0001 on the line",
	} {
		if !named(s, cited) {
			t.Errorf("%q did not count as naming Definition 1", cited)
		}
	}
	for _, quiet := range []string{
		"By Definition 12 of § 1 the module is Artinian.",
		"By Proposition 1 of § 1 the module is Artinian.",
		"nothing points at it at all",
	} {
		if named(s, quiet) {
			t.Errorf("%q was read as naming Definition 1", quiet)
		}
	}
	// A statement whose heading the corpus prints without a name is matched on
	// the label and the tag as before, and never on the empty string.
	if named(span{label: "alg-viii-s1-n1", tag: "0002"}, "anything at all") {
		t.Error("a statement with no printed name matched everything")
	}
}

// And the whole of it, on a § that will not fit: the definition the solution
// argues from stays and the one it never mentions goes.
func TestTheStatementTheSolutionArguesFromIsTheOneKept(t *testing.T) {
	c := &Context{Label: "alg-viii-s1-ex-6", Lang: "en", Cites: map[string]Reach{},
		Pieces: []Piece{
			{Kind: TheExercise, Label: "alg-viii-s1-ex-6", Text: "Let A be a ring and e an idempotent."},
			{Kind: TheSection, Label: "alg-viii-s1", Text: "## § 1. ARTINIAN MODULES\n\n" +
				"#### Definition 1 {#alg-viii-s1-def-1 .statement tag=0001}\n\n" +
				strings.Repeat("Every decreasing sequence of submodules is stationary. ", 40) + "\n\n" +
				"#### Example 3 {#alg-viii-s1-n2-exa-3 .statement tag=000G}\n\n" +
				strings.Repeat("A principal ideal domain is Noetherian. ", 40) + "\n"}}}
	whole := c.Render()
	got := c.RenderWithin(len(whole)-1200, "The sequence is stationary by Definition 1(ii).")
	if strings.Contains(got, "is not printed here") == false {
		t.Fatalf("nothing was dropped from a context over the limit:\n%s", got)
	}
	if !strings.Contains(got, "Every decreasing sequence of submodules is stationary") {
		t.Error("the definition the solution argues from was dropped")
	}
	if strings.Contains(got, "A principal ideal domain is Noetherian") {
		t.Error("the statement nothing points at was kept instead")
	}
}

// manyNamed is a context whose closure reached most of the corpus, which is the
// ordinary case for a book that cites across volumes and is what the cap is for.
func manyNamed(n int) *Context {
	c := wide()
	for i := range n {
		c.Named = append(c.Named, Piece{Kind: Reference,
			Label: fmt.Sprintf("top-iii-s%d", i), Why: SectionOnly})
	}
	return c
}

// The block that says what is in the corpus and is not in front of you is
// neither trimmed by RenderWithin nor counted by Chars, so before it was capped
// it was written out under a trimmed context at whatever length it came to and
// the question went out at that length. Exercise 1 of Commutative Algebra I § 1
// left the assembler at 447.9k characters against a 28k room, 422.3k of it this
// block, and the engine sent it because from where it stands an exercise that
// will not fit is a fact about the exercise.
func TestAContextThatCitedMostOfTheCorpusStillFitsTheRoomItWasGiven(t *testing.T) {
	c := manyNamed(5154)
	out := c.RenderWithin(28000, "")

	if len(out) > 28000 {
		t.Errorf("the question is %d characters against a room of 28000", len(out))
	}
	if strings.Count(out, "\n- top-iii-s") > mostNamed {
		t.Errorf("named %d of them, want at most %d",
			strings.Count(out, "\n- top-iii-s"), mostNamed)
	}
	if !strings.Contains(out, fmt.Sprintf("and %d more", 5154-mostNamed)) {
		t.Errorf("the tail was dropped without saying how much of it there was:\n%s", tail(out))
	}
}

// The cap is for the pathological case and the ordinary one has to come through
// it whole. A handful of references somebody could raise the cap for is worth
// naming one by one, which is what this block was written to do.
func TestAShortListOfWhatIsMissingIsPrintedWhole(t *testing.T) {
	c := manyNamed(3)
	out := c.RenderWithin(28000, "")

	if n := strings.Count(out, "\n- top-iii-s"); n != 3 {
		t.Errorf("named %d of the 3", n)
	}
	if strings.Contains(out, "more, not named here") {
		t.Errorf("a list of 3 was told it had a tail:\n%s", tail(out))
	}
}

// A room of nothing is not the same as no limit, and the two shared a branch.
//
// within computes the room as the limit less what the rest of the question
// already takes, so a call whose instructions, reference and candidate solution
// fill the limit on their own asks for a room of zero or below. That is the
// tightest a question is ever assembled and the one place trimming matters
// most. The unlimited case never reaches here: within returns Render itself
// when the engine's limit is negative, and every other caller computes the room
// by subtraction the same way.
//
// The branch read limit <= 0 and returned the whole context, so the assembler
// answered "there is no room at all" by sending everything it had.
func TestAContextGivenNoRoomAtAllGivesUpEverythingItCan(t *testing.T) {
	c := wide()
	whole := c.Render()

	for _, room := range []int{0, -1, -5000} {
		out := c.RenderWithin(room, "")
		if len(out) >= len(whole) {
			t.Errorf("a room of %d was rendered at %d characters, the whole of it being %d",
				room, len(out), len(whole))
		}
		// The floor and not nothing: the exercise is never given up, because a
		// question without it is not a question.
		if !strings.Contains(out, "the exercise to solve") {
			t.Errorf("a room of %d dropped the exercise itself", room)
		}
	}
}

// The floor a room of nothing reaches is the floor every tighter room reaches,
// because the same things are undroppable either way.
func TestNoRoomAndAVeryTightRoomComeToTheSameThing(t *testing.T) {
	c := wide()
	if got, want := c.RenderWithin(0, ""), c.RenderWithin(1, ""); got != want {
		t.Errorf("a room of 0 rendered %d characters and a room of 1 rendered %d",
			len(got), len(want))
	}
}

// manyOutside is a context citing n more things the corpus does not hold, on
// top of the one the fixture already carries.
func manyOutside(n int) *Context {
	c := wide()
	for i := range n {
		c.Pieces = append(c.Pieces, Piece{Kind: Outside, Depth: 1,
			Raw: fmt.Sprintf("Set Theory, III, §%d, No. 6, p. 155, Proposition 13", i)})
	}
	return c
}

// The out-of-corpus block is in the floor: order never offers Outside up, so a
// context trimmed until everything that could go has gone still carries it
// whole. Two exercises of Commutative Algebra § 1 carry 545 of these at 13723
// characters against a question limit of 32000, which is a trimmer that has
// given up every reference it had still handing over 13.7k it may not touch.
func TestTheReferencesThatLeaveTheCorpusDoNotBecomeTheFloor(t *testing.T) {
	c := manyOutside(545)
	out := c.RenderWithin(0, "")

	if len(out) > 6000 {
		t.Errorf("the floor is %d characters", len(out))
	}
	if n := strings.Count(out, "\n- Set Theory"); n > mostOutside {
		t.Errorf("named %d of them, the cap being %d", n, mostOutside)
	}
	// 545 added to the one the fixture already had, less the 40 named.
	if !strings.Contains(out, "and 506 more") {
		t.Error("the ones not named are not counted")
	}
	// The rule is the point of the block and it survives the cap.
	if !strings.Contains(out, "statements are not available to you") {
		t.Error("the instruction went with the list")
	}
}

// Under the cap nothing changes: a handful is named one by one, which is what
// the block was written to do.
func TestAShortListOfWhatLeavesTheCorpusIsPrintedWhole(t *testing.T) {
	c := manyOutside(3)
	out := c.Render()
	if n := strings.Count(out, "\n- Set Theory"); n != 4 {
		t.Errorf("named %d of the 4 the fixture comes to", n)
	}
	if strings.Contains(out, "more, which are not named here") {
		t.Error("a list that fits was given a tail")
	}
}
