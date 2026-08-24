package main

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// A heading put back from the contents has no page line to copy the sign or the
// case from, so it takes both off the § headings the volume already carries.
func TestSectionStyleOfReadsTheVolume(t *testing.T) {
	root := t.TempDir()
	write := func(book string, page int, body string) {
		t.Helper()
		path := corpus.PagePath(root, book, page)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		f := &corpus.PageFile{Meta: corpus.PageFrontMatter{Book: book, PDFPage: page}, Body: body}
		if err := f.Write(path); err != nil {
			t.Fatal(err)
		}
	}
	// Algebre I a III sets a sign and capitals.
	write("alg-i-iii-fr", 10, "## § 1. STRUCTURES ALGÉBRIQUES\n")
	write("alg-i-iii-fr", 20, "## § 2. GROUPES\n")
	// Groupes et algebres de Lie IV a VI sets a sign and no capitals.
	write("lie-iv-vi-fr", 10, "## § 1. Systèmes de racines\n")
	// Topologie generale I a IV sets no sign at all.
	write("top-i-iv", 10, "## 1. OPEN SETS\n")

	for _, c := range []struct {
		book        string
		sign, upper bool
	}{
		{"alg-i-iii-fr", true, true},
		{"lie-iv-vi-fr", true, false},
		{"top-i-iv", false, true},
	} {
		got := sectionStyleOf(root, c.book)
		if !got.known || got.sign != c.sign || got.upper != c.upper {
			t.Errorf("sectionStyleOf(%s) = %+v, want sign %v upper %v", c.book, got, c.sign, c.upper)
		}
	}
	// A volume with no § heading in it has nothing to be consistent with, and
	// saying so is what keeps a heading from being written in one of the two
	// styles at random.
	if got := sectionStyleOf(root, "alg-ix-fr"); got.known {
		t.Errorf("sectionStyleOf() on a volume with no § heading = %+v, want unknown", got)
	}
}

// The mathematics in a title is not upper cased, because \mathbf is a command
// and \MATHBF is not. § 3 of chapter VII of Topologie generale V a X is the
// title this came from.
func TestSectionStyleLeavesTheMathematicsAlone(t *testing.T) {
	st := sectionStyle{sign: true, upper: true, known: true}
	got := st.heading(3, `Sommes infinies dans les groupes $ \mathbf{R}^n $`)
	want := `## § 3. SOMMES INFINIES DANS LES GROUPES $ \mathbf{R}^n $`
	if got != want {
		t.Errorf("heading() = %q, want %q", got, want)
	}
	if got, want := (sectionStyle{known: true}).heading(1, "Réflexions"), "## 1. Réflexions"; got != want {
		t.Errorf("heading() = %q, want %q", got, want)
	}
}

// And a title that is mostly mathematics does not count as lower case when the
// volume is being read for its habit.
func TestCapitalsIgnoresTheMathematics(t *testing.T) {
	for _, c := range []struct {
		title string
		want  bool
	}{
		{`SOMMES INFINIES DANS LES GROUPES $ \mathbf{R}^n $`, true},
		{`Sommes infinies dans les groupes $ \mathbf{R}^n $`, false},
		{`$ \mathbf{R}^n $`, false},
		{"ANNEAUX", true},
		{"Anneaux", false},
	} {
		if got := capitals(c.title); got != c.want {
			t.Errorf("capitals(%q) = %v, want %v", c.title, got, c.want)
		}
	}
}
