package typography

import "testing"

// Every line here is one this corpus prints. The first two are the headings that
// stopped a volume, the rest are the footnote markers as the reading writes
// them, which is with the spacing set four ways.
func TestFootlessTakesTheMarkerOffAHeading(t *testing.T) {
	for _, c := range []struct{ in, want string }{
		{"Espaces hilbertiens $ ^1 $", "Espaces hilbertiens"},
		{"REPRÉSENTATIONS IRRÉDUCTIBLES DES GROUPES DE LIE COMPACTS CONNEXES $ ^1 $",
			"REPRÉSENTATIONS IRRÉDUCTIBLES DES GROUPES DE LIE COMPACTS CONNEXES"},
		{"appelle trace (resp. norme,$^1$", "appelle trace (resp. norme,"},
		{"isomorphe à Q \\times^H H/G $^{1}$", "isomorphe à Q \\times^H H/G"},
		{"the idea of this extension,$^8$", "the idea of this extension,"},
		{"de Lie simples (complexes).$^3$", "de Lie simples (complexes)."},
	} {
		if got := Footless(c.in); got != c.want {
			t.Errorf("Footless(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// A superscript on a closing parenthesis is an exponent and not a marker, and
// the three lines of this corpus that set one are all of that shape. A title
// with no marker on it comes back as it was.
func TestFootlessKeepsAnExponent(t *testing.T) {
	for _, in := range []string{
		"=$ (sin($\\pi x$)$/(\\pi x)$)$^2$",
		"$\\mathfrak{n}=$ Ker(ad $x$)$^2$",
		"Espaces hilbertiens",
		"Groupes de Coxeter et systèmes de Tits",
	} {
		if got := Footless(in); got != in {
			t.Errorf("Footless(%q) = %q, want it left alone", in, got)
		}
	}
}

// The grave on the preposition and nothing else. Page 38 of Algebre I a III
// prints the A bare and the E accented on the same line, which is the whole of
// the evidence and the whole of the table.
func TestAccentlessFoldsTheGraveOnA(t *testing.T) {
	if got, want := Accentless("Groupes et groupes à opérateurs"), "Groupes et groupes a opérateurs"; got != want {
		t.Errorf("Accentless() = %q, want %q", got, want)
	}
	if got, want := Accentless("À PARTIR DU N° 3"), "A PARTIR DU N° 3"; got != want {
		t.Errorf("Accentless() = %q, want %q", got, want)
	}
}

// An accent the press keeps is an accent a reading has to keep too, so nothing
// else folds. Fréchet against Frechet is a page that lost one, and this is what
// leaves it visible.
func TestAccentlessLeavesEveryOtherAccent(t *testing.T) {
	for _, in := range []string{
		"Dual of a Fréchet space",
		"REPRÉSENTATION GÉOMÉTRIQUE",
		"SYSTÈMES DE TITS",
		"théorie élémentaire",
		"Šafarevič",
		"Espaces $ \\tau $-adiques",
	} {
		if got := Accentless(in); got != in {
			t.Errorf("Accentless(%q) = %q, want it left alone", in, got)
		}
	}
}
