package footnote

import (
	"strings"
	"testing"
)

// The bodies here are the pages of the Theory of Sets as the fleet read them,
// cut down to the lines that carry a mark. Every shape in this file was found
// in the twenty pages of ens-i-iv that print one.

// Page 15 prints the mark and the reading wrote the reference beside it, twice,
// with two notes under two different marks.
func TestAMarkPrintedBesideItsReferenceComesOut(t *testing.T) {
	const body = `The *signs* of a mathematical theory $\mathscr{T}$ (*)[^1] are the following :

(1) The *logical signs* (†)[^2] : $\square$, $\tau$, $\vee$, $\neg$.

[^1]: (\*) The meaning of this expression will become clear as the chapter progresses.
[^2]: (†) For the intuitive meanings of these signs, see no. 3, Remark.`

	out, moves := Normalize(body)
	want := `The *signs* of a mathematical theory $\mathscr{T}$ [^1] are the following :

(1) The *logical signs* [^2] : $\square$, $\tau$, $\vee$, $\neg$.

[^1]: The meaning of this expression will become clear as the chapter progresses.
[^2]: For the intuitive meanings of these signs, see no. 3, Remark.`
	if out != want {
		t.Fatalf("got\n%s\nwant\n%s", out, want)
	}
	if got := count(moves, KindBeside); got != 2 {
		t.Errorf("%d marks beside a reference, want 2", got)
	}
	if got := count(moves, KindDefinition); got != 2 {
		t.Errorf("%d marks on a definition, want 2", got)
	}
}

// Page 244 ends the sentence between the mark and the reference. Taking the
// mark out has to take the space in front of it too, or the sentence ends on a
// space and then a full stop.
func TestASentenceEndingBetweenTheMarkAndTheReference(t *testing.T) {
	const body = `such that $0 < \alpha \leqslant \varkappa$ (*).[^1]

[^1]: (*) At present it is not known whether or not there exist inaccessible ordinals other than $\omega$.`

	out, _ := Normalize(body)
	want := `such that $0 < \alpha \leqslant \varkappa$.[^1]

[^1]: At present it is not known whether or not there exist inaccessible ordinals other than $\omega$.`
	if out != want {
		t.Fatalf("got\n%s\nwant\n%s", out, want)
	}
}

// Page 72 prints the mark and no reference at all. The mark is what sends the
// reader to the note, so it becomes the reference.
func TestAMarkStandingAloneBecomesTheReference(t *testing.T) {
	const body = `and is called *the empty set* (\*); the relation $(\forall x)(x \notin X)$ is read as follows.

[^1]: (\*) The term denoted by $\emptyset$ is therefore $\tau \neg \neg \in$.`

	out, moves := Normalize(body)
	want := `and is called *the empty set* [^1]; the relation $(\forall x)(x \notin X)$ is read as follows.

[^1]: The term denoted by $\emptyset$ is therefore $\tau \neg \neg \in$.`
	if out != want {
		t.Fatalf("got\n%s\nwant\n%s", out, want)
	}
	if got := count(moves, KindAlone); got != 1 {
		t.Errorf("%d marks standing alone, want 1", got)
	}
}

// Page 372 of the Summary of Results prints two notes under two marks and joins
// neither of them to its note.
func TestTwoMarksStandingAloneGoToTheirOwnNotes(t *testing.T) {
	const body = `The set $\mathbf{N}$ of natural integers (*) is ordered by the relation.

These relations are read “$x$ is less than $y$” (†). The set is ordered.

[^1]: (*) In accordance with our point of view in this Summary of Results.
[^2]: (†) Thus “less than” does not exclude “equal to”.`

	out, _ := Normalize(body)
	if !strings.Contains(out, "natural integers [^1] is ordered") {
		t.Errorf("the asterisk did not become note 1:\n%s", out)
	}
	if !strings.Contains(out, "less than $y$” [^2]. The set") {
		t.Errorf("the dagger did not become note 2:\n%s", out)
	}
}

// The pages where only the definition carries the mark are the ones the reading
// got right in the body, and the body is left exactly as it was.
func TestOnlyTheDefinitionCarriesTheMark(t *testing.T) {
	const body = `To deduce C63 from C62 [^1], let $u$ be a letter.

[^1]: (*) It is also possible to give a direct proof of C63.`

	out, moves := Normalize(body)
	want := `To deduce C63 from C62 [^1], let $u$ be a letter.

[^1]: It is also possible to give a direct proof of C63.`
	if out != want {
		t.Fatalf("got\n%s\nwant\n%s", out, want)
	}
	if len(moves) != 1 || moves[0].Kind != KindDefinition {
		t.Errorf("moves %v, want the definition alone", moves)
	}
}

// A § assembled out of twenty pages carries the notes of all twenty, renumbered,
// and three of them can be the same asterisk. A mark with no reference beside it
// is then a mark nothing identifies, and guessing sends the reader to the wrong
// note.
func TestAMarkTwoNotesShareIsLeftAlone(t *testing.T) {
	const body = `a specific sign $s$ of weight $n$ (*) in $\mathscr{T}$.

[^3]: (*) In accordance with what was said in no. 1.
[^4]: (*) As was said above, it would be possible to limit our consideration.`

	out, moves := Normalize(body)
	if !strings.Contains(out, `of weight $n$ (*) in`) {
		t.Errorf("the mark was moved and nothing said where to:\n%s", out)
	}
	if got := count(moves, KindLeft); got != 1 {
		t.Errorf("%d marks left alone, want 1", got)
	}
	if got := count(moves, KindDefinition); got != 2 {
		t.Errorf("%d definitions stripped, want 2", got)
	}
}

// A note already pointed at from the body is not pointed at a second time. The
// second mark is the printing's and the reference is the corpus's, and adding
// another reference would put two marks in the text for one note again.
func TestAMarkIsNotAddedToANoteAlreadyReferenced(t *testing.T) {
	const body = `We remarked on it in the Introduction [^1]. The purpose is to give a simple example (*).

[^1]: (*) The results established in this Appendix will not be used anywhere else.`

	out, moves := Normalize(body)
	if !strings.Contains(out, `a simple example (*).`) {
		t.Errorf("a second reference to note 1 was written:\n%s", out)
	}
	if got := count(moves, KindLeft); got != 1 {
		t.Errorf("%d marks left alone, want 1", got)
	}
}

// The asterisks that open and close a starred passage are not footnote marks
// and no note carries them, and neither is an asterisk inside a formula.
func TestWhatIsNotAFootnoteMarkIsNotTouched(t *testing.T) {
	const body = `\*For example, in the Theory of Sets, $(*)$ denotes the product, and

is an assembly.\* The note follows [^1].

[^1]: (*) The meaning of this expression will become clear.`

	out, moves := Normalize(body)
	if !strings.Contains(out, `\*For example`) || !strings.Contains(out, `is an assembly.\*`) {
		t.Errorf("a starred passage was taken for a footnote mark:\n%s", out)
	}
	if !strings.Contains(out, `$(*)$ denotes`) {
		t.Errorf("a formula was taken for a footnote mark:\n%s", out)
	}
	if len(moves) != 1 || moves[0].Kind != KindDefinition {
		t.Errorf("moves %v, want the definition alone", moves)
	}
}

// A body with no marked definition is the usual case, and it comes back the
// same string rather than a rebuilt one.
func TestABodyWithNoPrintedMarkIsUntouched(t *testing.T) {
	const body = `The purpose of this Appendix is to give a simple example [^1].

[^1]: The results established here will not be used anywhere else.`

	out, moves := Normalize(body)
	if out != body || moves != nil {
		t.Errorf("got %q with %v, want the body back", out, moves)
	}
}

// Running it twice changes nothing the second time, which is what lets the OCR
// writer and the fix command both run it.
func TestNormalizeIsIdempotent(t *testing.T) {
	const body = `The *signs* of a theory (*)[^1] are these, and the empty set (†) is that.

[^1]: (\*) The meaning of this expression.
[^2]: (†) The intuitive meaning of these signs.`

	once, _ := Normalize(body)
	twice, moves := Normalize(once)
	if twice != once {
		t.Fatalf("second run changed it:\n%s\n%s", once, twice)
	}
	if len(moves) != 0 {
		t.Errorf("second run made %v, want nothing", moves)
	}
}

func count(moves []Move, k Kind) int {
	n := 0
	for _, m := range moves {
		if m.Kind == k {
			n++
		}
	}
	return n
}
