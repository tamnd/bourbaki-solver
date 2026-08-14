package clip

import (
	"strings"
	"testing"
)

// The page every one of these is built from is 302 of Théories spectrales II,
// cut down to the wreck and the two paragraphs around it. The text layer set
// the fraction's halves on two bands and nothing in the file says they belong
// together, so the numerator is stranded on a line of its own and the
// denominator is in a display below it.
const (
	broken = `b) Pour tout entier $N\geqslant 1$, notons $f_N$ la fonction sur $\mathbf{T}$ telle que

$$
_{2i\pi ht}|h|
$$

$$
f_N(t) =\sum\widehat{f}(h)e1-
$$

N

$h\in \mathbf{Z}$ $|h|\leqslant N$

pour $t\in \mathbf{T}$. Soit $X\subset \mathbf{T}$ un ensemble tel que la restriction de $f$ à X est continue.`

	pictured = `b) Pour tout entier $N\geqslant 1$, notons $f_N$ la fonction sur $\mathbf{T}$ telle que

$$
f_N(t)=\sum_{\substack{h\in\mathbf{Z}\\ |h|\leqslant N}}\widehat f(h)e^{2i\pi ht}\left(1-\frac{|h|}{N}\right)
$$

pour $t\in\mathbf{T}$. Soit $X\subset\mathbf{T}$ un ensemble tel que la restriction de $f$ à $X$ est continue.`

	// whole is the one line of the repaired page that has to be there.
	whole = `f_N(t)=\sum_{\substack{h\in\mathbf{Z}\\ |h|\leqslant N}}\widehat f(h)e^{2i\pi ht}\left(1-\frac{|h|}{N}\right)`
)

func TestFixPutsTheDisplayBack(t *testing.T) {
	body, changes, refused := Fix(broken, pictured)
	if len(refused) != 0 {
		t.Fatalf("refused %d: %v", len(refused), refused)
	}
	if len(changes) != 1 {
		t.Fatalf("changed %d, want 1", len(changes))
	}
	if !strings.Contains(body, whole) {
		t.Errorf("the display did not come back:\n%s", body)
	}
	// The prose either side is the extractor's, byte for byte. This is the
	// point of the whole design: the model reads X as $X$ and writes \sup for
	// sup and would quietly hand back its own spelling of a paragraph nobody
	// asked it about.
	for _, want := range []string{
		`b) Pour tout entier $N\geqslant 1$, notons $f_N$ la fonction sur $\mathbf{T}$ telle que`,
		`pour $t\in \mathbf{T}$. Soit $X\subset \mathbf{T}$ un ensemble tel que la restriction de $f$ à X est continue.`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("the extractor's paragraph was overwritten, wanted\n%s\nin\n%s", want, body)
		}
	}
	if strings.Contains(body, "\n\nN\n\n") {
		t.Error("the stranded line is still there")
	}
}

func TestFixLeavesAWholePageAlone(t *testing.T) {
	body, changes, refused := Fix(pictured, pictured)
	if body != pictured || changes != nil || refused != nil {
		t.Errorf("a page with no wreck on it was touched: %d changes, %d refusals", len(changes), len(refused))
	}
}

func TestFixRefusesInventedProse(t *testing.T) {
	// The failure this is here for is the quiet one. The model transcribes the
	// display correctly and adds a sentence saying what it means, and the
	// sentence is fluent, is in the right language, and is not on the page.
	said := strings.Replace(pictured, "$$\n\npour $t", "$$\n\nCette série converge normalement sur le tore.\n\npour $t", 1)
	body, changes, refused := Fix(broken, said)
	if len(changes) != 0 {
		t.Fatalf("took a replacement that invented a sentence: %v", changes)
	}
	if len(refused) != 1 {
		t.Fatalf("refused %d, want 1", len(refused))
	}
	if !strings.Contains(refused[0].Reason, "says words the page does not") {
		t.Errorf("refused for the wrong reason: %s", refused[0].Reason)
	}
	if body != broken {
		t.Error("the body changed on a refusal")
	}
}

func TestFixRefusesDroppedProse(t *testing.T) {
	// The other half of the same proof. Page 302 of Théories spectrales II has
	// two lines of prose inside its wreck, between the displays, and a model
	// that rebuilds the mathematics and loses one of them leaves a page that
	// reads as though the second display followed from the first.
	was := strings.Replace(broken, "\n\nN\n\n", "\n\nN\n\noù l'on pose\n\n", 1)
	_, changes, refused := Fix(was, pictured)
	if len(changes) != 0 {
		t.Fatalf("took a replacement that dropped a line of prose: %v", changes)
	}
	if len(refused) != 1 || !strings.Contains(refused[0].Reason, "drops words the page has") {
		t.Fatalf("refused for the wrong reason: %v", refused)
	}
}

func TestFixRefusesAWreckWithNothingToAnchorTo(t *testing.T) {
	// Page 42 of Théories spectrales III ends in the middle of a display, so
	// the wreck runs to the foot of the page and there is no paragraph after it
	// to pin the replacement against. What matters is that this is reported.
	// The first cut of this dropped such a wreck where it stood and counted the
	// page as whole, which is a repair pass lying about the one thing it is for.
	foot := broken[:strings.Index(broken, "\n\npour $t")]
	body, changes, refused := Fix(foot, pictured)
	if len(changes) != 0 {
		t.Fatalf("repaired a wreck with no anchor after it: %v", changes)
	}
	if len(refused) != 1 || !strings.Contains(refused[0].Reason, "foot of the page") {
		t.Fatalf("refused for the wrong reason: %v", refused)
	}
	if body != foot {
		t.Error("the body changed on a refusal")
	}
}

func TestFixRefusesAPageTheModelNeverRead(t *testing.T) {
	_, changes, refused := Fix(broken, "TS II.292 EXERCICES § 1\n\nquelque chose de tout à fait différent, et plus long que six mots.")
	if len(changes) != 0 {
		t.Fatalf("repaired from an answer about another page: %v", changes)
	}
	if len(refused) != 1 {
		t.Fatalf("refused %d, want 1", len(refused))
	}
}

func TestHouse(t *testing.T) {
	// Every one of these is a spelling the corpus has settled and a model has
	// not. See house for the counts they were settled by.
	for _, c := range []struct{ from, to string }{
		{`\mathrm P^{k-\delta}`, `P^{k-\delta}`},
		{`S_{\mathrm P}(x)`, `S_{P}(x)`},
		{`\mathrm{N}\to+\infty`, `N\rightarrow+\infty`},
		{`t\longmapsto f(t)`, `t\mapsto f(t)`},
		{`\mathit{I}(y)`, `I(y)`},
		// Left alone: an operator's name is set upright by everybody, both
		// signs of inequality are on the page, and both ellipses are.
		{`\mathrm{Homgr}(A,B)`, `\mathrm{Homgr}(A,B)`},
		{`a\leq b\leqslant c`, `a\leq b\leqslant c`},
		{`x_1\cdots x_n\ldots`, `x_1\cdots x_n\ldots`},
		// \top is not \to, and a table that matched substrings would say it was.
		{`\top`, `\top`},
	} {
		if got := house(c.from); got != c.to {
			t.Errorf("house(%q) = %q, want %q", c.from, got, c.to)
		}
	}
}

func TestSaidDropsWhatIsNotLanguage(t *testing.T) {
	// The two readings of page 424 of Théories spectrales III differ in exactly
	// one word of the paragraph the repair is anchored against, and it is the
	// one word that is not a word: the extractor reads the roman sup out of the
	// text layer as three letters and the model writes \sup.
	ours := `Si $p= 1$, soit $w$ la fonction constante sur G égale à sup$_{x\in G}u(x)$. Si $p >1$, définissons`
	theirs := `Si $p=1$, soit $w$ la fonction constante sur G égale à $\sup_{x\in G}u(x)$. Si $p>1$, définissons`
	a, b := said(ours), said(theirs)
	if !same(a, b) {
		t.Errorf("the same paragraph read two ways did not match:\n%v\n%v", a, b)
	}
}
