package extract

import "testing"

// The label in a running head opens with the letter of its Book, which is one
// letter in Algebra and two in Théories spectrales and Topologie algébrique.
// Read as a single A, the label of "TA I.144 EXERCICES" came out as "A I.144"
// and left the T of the Book behind in the title, and the label of "TS III.5"
// was not found at all, since there is no A in it to match.
func TestAHeadLabelCarriesTheLetterOfItsBook(t *testing.T) {
	cases := []struct {
		head  string
		label string
		title string
		foot  int
	}{{
		head:  "TA I.144 EXERCICES",
		label: "TA I.144",
		title: "EXERCICES",
	}, {
		head:  "EXERCICES TA I.144",
		label: "TA I.144",
		title: "EXERCICES",
	}, {
		head:  "APPLICATIONS LINÉAIRES COMPACTES TS III.5",
		label: "TS III.5",
		title: "APPLICATIONS LINÉAIRES COMPACTES",
	}, {
		head:  "A VIII.4 ARTINIAN MODULES AND NOETHERIAN MODULES",
		label: "A VIII.4",
		title: "ARTINIAN MODULES AND NOETHERIAN MODULES",
	}, {
		// Sixty pages of Algebra VIII head a title that runs the whole
		// measure, and the title and the label arrive with no space between
		// them.
		head:  "CRITÈRES POUR QU’UNE ALGÈBRE DE QUATERNIONS SOIT UN CORPSA VIII.357",
		label: "A VIII.357",
		title: "CRITÈRES POUR QU’UNE ALGÈBRE DE QUATERNIONS SOIT UN CORPS",
	}, {
		// Lie 7 to 9 prints no label anywhere and carries its page number at
		// the outer edge of the head instead. Nothing here is a label, and the
		// chapter at the end of the line is not one either.
		head:  "100 SPLIT SEMI-SIMPLE LIE ALGEBRAS Ch. VIII",
		label: "",
		title: "100 SPLIT SEMI-SIMPLE LIE ALGEBRAS Ch. VIII",
		foot:  100,
	}}
	for _, c := range cases {
		p := &Page{Head: c.head}
		p.readHead()
		if p.Label != c.label {
			t.Errorf("%q: label %q, want %q", c.head, p.Label, c.label)
		}
		if p.Title != c.title {
			t.Errorf("%q: title %q, want %q", c.head, p.Title, c.title)
		}
		if p.Foot != c.foot {
			t.Errorf("%q: foot %d, want %d", c.head, p.Foot, c.foot)
		}
	}
}
