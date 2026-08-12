package extract

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// French page 257, whole, three lines of it and the sign beside them. The sign
// is drawn in the margin at 40, the paragraph is indented to 108 and set to the
// margin of 80, and the sign falls beside the second line rather than the first
// because that is where TeX drew it. The word "automorphisme" is broken across
// the first two lines, so this is also the fixture where the sign lands inside
// a word: the corpus shipped "auto$\dbend$ morphisme".
const bendInAWordPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="257" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="4" size="14" family="LMRoman12" color="#131413"/>
<fontspec id="9" size="29" family="BOUR17" color="#131413"/>
<text top="291" left="108" width="464" height="12" font="4">a) Soient D un corps et Z son centre. Si D est de degré fini sur Z, tout auto-</text>
<text top="310" left="80" width="491" height="12" font="4">morphisme de D qui laisse fixes les éléments de Z est intérieur. L’hypothèse que D</text>
<text top="316" left="40" width="26" height="37" font="9">Z</text>
<text top="329" left="80" width="268" height="12" font="4">est de degré fini sur Z est essentielle.</text>
</page>
</pdf2xml>
`

// The sign is a mark on the passage, so it goes at the head of the passage. It
// is not a formula, so it is not written as one: KaTeX refused every \dbend the
// corpus carried and the site could not be published while it did.
func TestTheDangerousBendGoesToTheHeadOfThePassageItMarks(t *testing.T) {
	lines := parse(t, bendInAWordPage)
	got := blocks(lines, Volume{BodySize: 14})
	want := corpus.Bend + " a) Soient D un corps et Z son centre. Si D est de degré fini sur Z, " +
		"tout automorphisme de D qui laisse fixes les éléments de Z est intérieur. " +
		"L’hypothèse que D est de degré fini sur Z est essentielle."
	if got != want {
		t.Errorf("blocks():\n got %q\nwant %q", got, want)
	}
}

// And the word it was drawn beside is one word again. This is the half of it a
// reader would notice: the sign at least looked like a mark, "auto☡morphisme"
// looks like a spelling nobody caught.
func TestTheSignDoesNotSplitTheWordItIsDrawnBeside(t *testing.T) {
	got := blocks(parse(t, bendInAWordPage), Volume{BodySize: 14})
	if !strings.Contains(got, "automorphisme") {
		t.Errorf("the word the sign was drawn beside is still broken:\n%s", got)
	}
	if strings.Contains(got, `\dbend`) {
		t.Errorf("the sign is still written as a command KaTeX does not know:\n%s", got)
	}
	if n := strings.Count(got, corpus.Bend); n != 1 {
		t.Errorf("the sign appears %d times, want 1:\n%s", n, got)
	}
}

// The foreword of the French printing prints the sign inside a sentence, to say
// what it means. That one is not a margin mark and does not move: it stands to
// the right of the words before it and the words continue after it.
const bendInASentencePage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="7" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="4" size="14" family="LMRoman12" color="#131413"/>
<fontspec id="9" size="20" family="BOUR17" color="#131413"/>
<text top="803" left="80" width="467" height="12" font="4">ces passages sont signalés en marge par le signe</text>
<text top="797" left="554" width="18" height="26" font="9">Z</text>
</page>
</pdf2xml>
`

func TestTheSignInsideASentenceStaysWhereItIsWritten(t *testing.T) {
	got := blocks(parse(t, bendInASentencePage), Volume{BodySize: 14})
	if want := "ces passages sont signalés en marge par le signe " + corpus.Bend; got != want {
		t.Errorf("blocks():\n got %q\nwant %q", got, want)
	}
}
