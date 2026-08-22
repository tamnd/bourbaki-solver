package typography

import "testing"

func TestAnElisionTakesTheTypographicApostrophe(t *testing.T) {
	// Off page 13 of Elements of Lie IV to VI in French, which the reader gave
	// back with the straight mark on the same day it gave back the page beside
	// it with the typographic one.
	was := "Comme $p$ est d'ordre $m$, les éléments sont distincts et l'on a $t_{j+m} = t_j$."
	want := "Comme $p$ est d’ordre $m$, les éléments sont distincts et l’on a $t_{j+m} = t_j$."
	got, changed, left := Apostrophes(was)
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if changed != 2 || left != 0 {
		t.Errorf("changed %d and left %d, want 2 and 0", changed, left)
	}
}

func TestEveryElisionBourbakiWrites(t *testing.T) {
	for _, c := range []struct{ was, want string }{
		{"d'un", "d’un"}, {"l'ensemble", "l’ensemble"}, {"qu'il", "qu’il"},
		{"n'est", "n’est"}, {"s'il", "s’il"}, {"c'est", "c’est"},
		{"jusqu'à", "jusqu’à"}, {"lorsqu'on", "lorsqu’on"},
		{"puisqu'elle", "puisqu’elle"}, {"quoiqu'aucun", "quoiqu’aucun"},
		{"aujourd'hui", "aujourd’hui"}, {"j'ai", "j’ai"},
		{"L'Algèbre", "L’Algèbre"}, {"D'après", "D’après"},
	} {
		got, changed, _ := Apostrophes(c.was)
		if got != c.want || changed != 1 {
			t.Errorf("%q became %q with %d changed, want %q and 1", c.was, got, changed, c.want)
		}
	}
}

func TestAPrimeOutsideItsDollarsIsLeftAlone(t *testing.T) {
	// These are all off French pages, where the reading let a prime and its
	// subscript out of the dollars. They are mathematics and the mark on them
	// is a prime, so the repair refuses them and says how many it refused.
	for _, was := range []string{
		"la famille $(e_i)$ et la famille (e'_i)",
		"les sommes Σ'_1 et Q'_\\mathfrak{p}",
		"on pose x'_1 = x'_2",
		"le produit h'h",
		"la suite f'_j",
	} {
		got, changed, left := Apostrophes(was)
		if got != was {
			t.Errorf("%q became %q", was, got)
		}
		if changed != 0 || left == 0 {
			t.Errorf("%q reported %d changed and %d left, want 0 changed and some left", was, changed, left)
		}
	}
}

func TestMathematicsIsNotLookedAtAtAll(t *testing.T) {
	// A prime inside the dollars is not counted in either direction, and the
	// dollars come back where they were. The display is one Bourbaki sets in
	// French and the letters in it are the ones the page has.
	was := "Soit $w'^{-1}s_q w'$ l'élément.\n\n$$\nB' = w B w^{-1}, \\quad l'\n$$\n\nOn a d'où.\n"
	want := "Soit $w'^{-1}s_q w'$ l’élément.\n\n$$\nB' = w B w^{-1}, \\quad l'\n$$\n\nOn a d’où.\n"
	got, changed, left := Apostrophes(was)
	if got != want {
		t.Errorf("got  %q\nwant %q", got, want)
	}
	if changed != 2 || left != 0 {
		t.Errorf("changed %d and left %d, want 2 and 0", changed, left)
	}
}

func TestAPageAlreadyRightIsNotTouched(t *testing.T) {
	was := "Comme $p$ est d’ordre $m$, l’on a d’où la formule (14)."
	got, changed, left := Apostrophes(was)
	if got != was || changed != 0 || left != 0 {
		t.Errorf("got %q with %d changed and %d left, want it unchanged", got, changed, left)
	}
}

func TestAnEnglishPossessiveIsNotAnElision(t *testing.T) {
	// The same fault is on the English pages, 218 straight against 380
	// typographic, and it is not this rule's. Hilbert's is not an elision and
	// nothing in the list in front of an apostrophe tells a possessive from a
	// prime, so those pages are counted and left for their own repair.
	was := "Hilbert's paradox and Zorn's lemma"
	got, changed, left := Apostrophes(was)
	if got != was || changed != 0 || left != 2 {
		t.Errorf("got %q with %d changed and %d left, want it unchanged with 2 left", got, changed, left)
	}
}
