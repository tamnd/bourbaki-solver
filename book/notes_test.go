package book

import (
	"strings"
	"testing"
)

// The 905 markers.
//
// A footnote written the way Markdown writes one went through both builders as
// prose, so 38 of the 129 built volumes printed the source of a note instead of
// the note: "[^1]: Notably Democritus, ..." as a paragraph in the EPUB and
// "[\textasciicircum{}1]: Notably Democritus, ..." on the page of the pdf. These
// are the readings that stop that.

func TestAMarkdownFootnoteIsSetAsAFootnoteAndNotAsItsOwnSource(t *testing.T) {
	body := "The atomists held the contrary [^1].\n\n[^1]: Notably Democritus.\n"
	out, err := Renderer{}.TeX(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `\footnote{Notably Democritus.}`) {
		t.Errorf("the note was not set as a footnote:\n%s", out)
	}
	// The two spellings the corpus used to print. The caret is the one the
	// escaper produced, which is why the defect reads differently on the page
	// than it does in the file.
	for _, wrong := range []string{"[^1]", `textasciicircum`, "]:"} {
		if strings.Contains(out, wrong) {
			t.Errorf("the source of the note reached the page as %q:\n%s", wrong, out)
		}
	}
}

// Bourbaki sets the mark against the word, "contrary¹.", and the reading wrote a
// space in front of it because that is where the eye put it. A space left in
// sets the mark adrift from its word and against the full stop after it.
func TestTheSpaceBeforeAMarkDoesNotReachThePage(t *testing.T) {
	out, err := Renderer{}.TeX("held the contrary [^1].\n\n[^1]: Notably Democritus.\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `contrary\footnote{`) {
		t.Errorf("the mark did not go against the word it belongs to:\n%s", out)
	}
}

// A note written over more than one line is one note. The corpus has one, in the
// Summary of Results of Set Theory, and 53 definitions sitting directly under
// another definition which are not a continuation of anything.
func TestANoteWrittenOverSeveralLinesIsOneNote(t *testing.T) {
	body := "The set $\\mathbf{N}$ is ordered [^1].\n\n" +
		"[^1]: The integers are assumed known.\nIn our terminology, 0 belongs to it.\n"
	out, err := Renderer{}.TeX(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `\footnote{The integers are assumed known. In our terminology, 0 belongs to it.}`) {
		t.Errorf("the second line of the note did not travel with the first:\n%s", out)
	}
}

func TestTwoDefinitionsInARowAreTwoNotes(t *testing.T) {
	body := "First [^1] and second [^2].\n\n[^1]: One.\n[^2]: Two.\n"
	out, err := Renderer{}.TeX(body)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `First\footnote{One.} and second\footnote{Two.}`) {
		t.Errorf("the two notes did not go to their own calls:\n%s", out)
	}
}

// A note nothing calls goes to the end of the text it was printed under. There
// is one in the corpus. Dropping it would lose the sentence the note carried,
// which is the half of a footnote worth keeping.
func TestANoteNothingCallsGoesToTheEndOfTheTextAboveIt(t *testing.T) {
	out, err := Renderer{}.TeX("The atomists held the contrary.\n\n[^1]: Notably Democritus.\n")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out, `contrary.\footnote{Notably Democritus.}`) {
		t.Errorf("the uncalled note did not go under its own text:\n%s", out)
	}
}

// A call with no note behind it is not a footnote at all. There are 65 in the
// corpus and every one is a superscript the reading lost: a (p) over a letter in
// Topological Vector Spaces V came back as [^1] between two formulae. Inventing
// a note would put an empty one at the foot of the page and link the reader to
// it.
func TestACallWithNoNoteBehindItIsLeftWhereItStands(t *testing.T) {
	in := "the representation $\\gamma$[^1]$_G$ of G is unitary.\n"
	if got := footnotes(in); got != in {
		t.Errorf("footnotes(%q) = %q, want it untouched", in, got)
	}
}

// A bracket the corpus escaped is four characters of prose that has no note
// behind it, and reading the label wide enough to swallow the closing escape
// turns one into a footnote call.
func TestAnEscapedBracketIsNotAFootnoteCall(t *testing.T) {
	in := "the interval \\[^1\\] is closed.\n\n[^1]: Not this one.\n"
	if got := footnotes(in); !strings.Contains(got, `\[^1\]`) {
		t.Errorf("the escaped bracket was read as a call: %q", got)
	}
}

// Both marks get the note. Six labels in the corpus are called twice: two are a
// mark the reading doubled and one is Algebra IV, where two sections that each
// printed a note 1 were assembled into one file and one definition survived.
// Setting the note at both is the way round that leaves nothing printing its own
// brackets.
func TestALabelCalledTwiceGetsTheNoteAtBothMarks(t *testing.T) {
	out, err := Renderer{}.TeX("Here [^1] and again there [^1].\n\n[^1]: One note.\n")
	if err != nil {
		t.Fatal(err)
	}
	if n := strings.Count(out, `\footnote{One note.}`); n != 2 {
		t.Errorf("the note was set %d times, want 2:\n%s", n, out)
	}
}

// A file with no footnote in it is not to be rewritten, which is nearly every
// file in the corpus.
func TestABodyWithNoFootnoteInItComesBackUnchanged(t *testing.T) {
	in := "An ordered set is a set with an order on it.\n\nThat is all.\n"
	if got := footnotes(in); got != in {
		t.Errorf("footnotes changed a body with no note in it:\n%s", got)
	}
}

// A note is written at the foot of the body it belongs to, and the assembly
// puts the foot of the body under the pointer to the exercises whenever the §
// has any. Cutting at the pointer took 94 notes in 68 files with it, 22 per cent
// of the corpus's footnotes, each one called from a mark in the prose above.
func TestANoteParkedUnderTheExercisePointerIsNotCutAwayWithIt(t *testing.T) {
	body := "Divisibility [^1] is the subject.\n\n" +
		"### Exercises {#alg-iv-s1-exercises}\n\n" +
		"See the [exercises for § 1](exercises/s1/).\n\n" +
		"[^1]: The analogy with the integers.\n"
	out := StripExercisePointer(body)
	if strings.Contains(out, "exercises for") {
		t.Errorf("the pointer survived the strip:\n%s", out)
	}
	if !strings.Contains(out, "[^1]: The analogy with the integers.") {
		t.Errorf("the note went with the pointer:\n%s", out)
	}
	tex, err := Renderer{}.TeX(out)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(tex, `Divisibility\footnote{The analogy with the integers.}`) {
		t.Errorf("the note did not reach its mark:\n%s", tex)
	}
}

func TestABodyWithNoNoteUnderThePointerIsCutWhereItAlwaysWas(t *testing.T) {
	body := "Divisibility is the subject.\n\n" +
		"### Exercises {#alg-iv-s1-exercises}\n\n" +
		"See the [exercises for § 1](exercises/s1/).\n"
	if got, want := StripExercisePointer(body), "Divisibility is the subject.\n"; got != want {
		t.Errorf("StripExercisePointer = %q, want %q", got, want)
	}
}

// Nothing in the audit said that 38 volumes were printing 905 footnote markers
// as their own source. A whole book had to be read to find it, which is what
// this file of checks exists to stop.
func TestAFootnotePrintedAsItsOwnSourceIsAFailedCheck(t *testing.T) {
	find := func(tex string) Check {
		t.Helper()
		a := &Audit{}
		a.written(&Document{TeX: tex}, DefaultAuditOptions())
		for _, c := range a.Checks {
			if c.Name == "no footnote is printed as its own source" {
				return c
			}
		}
		t.Fatal("the check is not in the audit")
		return Check{}
	}
	bad := find(`earlier mathematicians [\textasciicircum{}1]. But from the earliest texts`)
	if bad.OK {
		t.Errorf("a marker on the page passed the audit: %+v", bad)
	}
	if len(bad.Notes) != 1 {
		t.Errorf("the audit named %d places, want 1: %+v", len(bad.Notes), bad.Notes)
	}
	if good := find(`earlier mathematicians\footnote{Notably Democritus.}.`); !good.OK {
		t.Errorf("a volume that sets its notes failed the audit: %+v", good)
	}
}

// ---------------------------------------------------------------------------
// The EPUB.
// ---------------------------------------------------------------------------

func renderEPUBBody(t *testing.T, body string) string {
	t.Helper()
	p := &pageRenderer{lang: "en", eng: engine(t)}
	out, err := p.render(body)
	if err != nil {
		t.Fatalf("rendering: %v", err)
	}
	return out
}

// A reading system paginates for itself, so there is no foot of a page to put a
// note at. It goes to the end of the document with a link each way, which is
// what an EPUB can do and what the format has an epub:type for.
func TestAFootnoteInTheEPUBIsALinkToANoteAtTheEndOfTheDocument(t *testing.T) {
	out := renderEPUBBody(t, "The atomists held the contrary [^1].\n\n[^1]: Notably Democritus.\n")
	for _, want := range []string{
		`<sup class="fnmark" id="fnref-1">`,
		`<a epub:type="noteref" href="#fn-1">1</a>`,
		`<aside class="notes" epub:type="footnotes">`,
		`<div class="fn" id="fn-1" epub:type="footnote">`,
		`<a class="fnback" href="#fnref-1">1</a> Notably Democritus.`,
	} {
		if !strings.Contains(out, want) {
			t.Errorf("the note is missing %s:\n%s", want, out)
		}
	}
	if strings.Contains(out, "[^1]") {
		t.Errorf("the source of the note reached the page:\n%s", out)
	}
}

// The pdf has read an inline \footnote{...} since it was written and the EPUB
// left it in the sentence inside a span, so the note interrupted the words it
// was a note on. The corpus writes 118 of them.
func TestAnInlineFootnoteLeavesTheSentenceInTheEPUB(t *testing.T) {
	out := renderEPUBBody(t, "The atomists held the contrary\\footnote{Notably Democritus.}.\n")
	if strings.Contains(out, `class="footnote"`) {
		t.Errorf("the note is still set in the middle of the sentence:\n%s", out)
	}
	if !strings.Contains(out, `<aside class="notes" epub:type="footnotes">`) ||
		!strings.Contains(out, "Notably Democritus.</div>") {
		t.Errorf("the note did not go to the end of the document:\n%s", out)
	}
	if !strings.Contains(out, `contrary<sup class="fnmark"`) {
		t.Errorf("the mark did not stay where the note was called:\n%s", out)
	}
}

// A \footnotetext is a note the reading found at the foot of a page without
// finding the call, which is 33 places in the corpus. It gets no number, because
// a number says there is a mark to look for and there is not.
func TestAFootnoteTextIsANoteWithNoMark(t *testing.T) {
	out := renderEPUBBody(t, "The atomists held the contrary.\\footnotetext{Notably Democritus.}\n")
	if strings.Contains(out, "fnmark") || strings.Contains(out, "fnref") {
		t.Errorf("a mark was invented for a note that has no call:\n%s", out)
	}
	if !strings.Contains(out, `<div class="fn">Notably Democritus.</div>`) {
		t.Errorf("the note did not reach the end of the document:\n%s", out)
	}
}

// Two notes in one document are two numbers and two pairs of ids. An id that
// repeats inside a file sends every mark to the first note.
func TestTheNotesOfADocumentAreNumberedThroughIt(t *testing.T) {
	out := renderEPUBBody(t, "First [^1].\n\nSecond [^2].\n\n[^1]: One.\n[^2]: Two.\n")
	for _, want := range []string{`id="fnref-1"`, `id="fn-1"`, `id="fnref-2"`, `id="fn-2"`} {
		if strings.Count(out, want) != 1 {
			t.Errorf("%s appears %d times, want once:\n%s", want, strings.Count(out, want), out)
		}
	}
	if strings.Count(out, `<aside class="notes"`) != 1 {
		t.Errorf("the notes were not gathered into one run:\n%s", out)
	}
}

// A document with no note in it gets no rule and no empty aside at the end of
// it, which is nearly every document in the library.
func TestADocumentWithNoNoteInItEndsWithNothingExtra(t *testing.T) {
	out := renderEPUBBody(t, "An ordered set is a set with an order on it.\n")
	if strings.Contains(out, "notes") {
		t.Errorf("an empty note run was written:\n%s", out)
	}
}
