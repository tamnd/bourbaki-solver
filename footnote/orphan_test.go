package footnote

import "testing"

const page = `Some prose that runs on for a while and ends the paragraph here.

PROPOSITION 2. Every ordered set has a cofinal well ordered subset.

`

func TestOrphansFindsTheNoteNothingPointsAt(t *testing.T) {
	body := page + "1 The terminology used in *loc. cit.* is *directed*.\n"
	got := Orphans(body)
	if len(got) != 1 {
		t.Fatalf("Orphans found %d, want 1: %+v", len(got), got)
	}
	if got[0].Digit != "1" || got[0].Line != 5 {
		t.Errorf("got note %q on line %d, want note 1 on line 5", got[0].Digit, got[0].Line)
	}
}

func TestOrphansSaysNothingWhereAnythingPointsAtANote(t *testing.T) {
	note := "1 The terminology used in *loc. cit.* is *directed*.\n"
	for _, mark := range []string{
		"a set directed[^1] with respect to",   // a reference
		"[^1]: the terminology of loc. cit.\n", // a definition
		"a set directed^1^ with respect to",    // a superscript a reading wrote
		"a set directed<sup>1</sup> with",      // the same in HTML
		"the empty set (*) has no element",     // the mark the volumes print
		"the empty set (†) has no element",
	} {
		body := page + mark + "\n\n" + note
		if got := Orphans(body); len(got) != 0 {
			t.Errorf("with %q on the page, Orphans found %+v; want nothing", mark, got)
		}
	}
}

func TestOrphansLeavesTheProseThatOpensOnANumeral(t *testing.T) {
	for _, tail := range []string{
		// The trivial group at the head of an exact sequence, with prose under
		// it. Page 115 of Lie VII.
		"1 $\\longrightarrow T_Q\\longrightarrow$ Aut($\\mathfrak{g}$) $\\longrightarrow$ 1\n\nis exact.\n",
		// A list inside an exercise whose brackets the reading lost, with the
		// rest of the exercise under it. Page 62 of FVR I.
		"2 $x^a f(x^{-b})$ is increasing, $a(b - a) \\geq 0;$\n\nUnder the same hypotheses show that $e^{x/2}$ is convex.\n",
		// Too short to be a note.
		"1 See below.\n",
		// A numbered list item, which is written with the bracket.
		"1) Let $f$ be a positive convex function on the half line.\n",
		// A numbered heading, which is written with the full stop.
		"1. Let $f$ be a positive convex function on the half line.\n",
		// Not a sentence: a note opens like one.
		"1 and so the second term of the series is bounded above by one.\n",
	} {
		if got := Orphans(page + tail); len(got) != 0 {
			t.Errorf("Orphans(%q) = %+v; want nothing", tail, got)
		}
	}
}

func TestOrphansReadsARunOfNotesDownThePage(t *testing.T) {
	body := page +
		"1 Il faut noter que des énoncés équivalents à ces règles se trouvent déjà.\n" +
		"2 Toutefois, la notion de produit cartésien de deux ensembles quelconques.\n" +
		"3 Pour chaque relation obtenue à partir d’une ou de plusieurs relations données.\n"
	got := Orphans(body)
	if len(got) != 3 {
		t.Fatalf("Orphans found %d, want 3: %+v", len(got), got)
	}
	for i, want := range []string{"1", "2", "3"} {
		if got[i].Digit != want {
			t.Errorf("note %d of the run is %q, want %q", i, got[i].Digit, want)
		}
	}
	if got[0].Line >= got[1].Line || got[1].Line >= got[2].Line {
		t.Errorf("the notes come back out of order: %d, %d, %d", got[0].Line, got[1].Line, got[2].Line)
	}
}

func TestOrphansDoesNotLookPastTheTenthNote(t *testing.T) {
	// A numbered bibliography entry is written the way a note is and does reach
	// ten, so the reference lists of the historical notes would come back whole.
	body := page + "10 Hamilton had incidentally proved this theorem for matrices of order 3.\n"
	if got := Orphans(body); len(got) != 0 {
		t.Errorf("Orphans found %+v; want nothing", got)
	}
}
