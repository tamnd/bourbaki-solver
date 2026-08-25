package book

import (
	"archive/zip"
	"encoding/xml"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/katex"
)

func engine(t *testing.T) *katex.Renderer {
	t.Helper()
	eng, err := katex.New()
	if err != nil {
		t.Fatalf("loading katex: %v", err)
	}
	return eng
}

func TestMathPlain(t *testing.T) {
	eng := engine(t)
	cases := []struct {
		tex, want string
	}{
		{`\Lambda`, "Λ"},
		{`p`, "p"},
		// MathML is a tree of elements and the text of it is the symbols alone,
		// with the spacing carried by the elements, so a label reads as the
		// symbols run together. That is what a contents line wants: it is a
		// label rather than a formula.
		{`E \otimes F`, "E⊗F"},
		// A variant is an attribute in MathML rather than a character, so the
		// Fraktur S comes back as a plain S. A contents entry that reads S_n
		// where the heading sets the Fraktur is a small loss and the honest one
		// to take, since the alternative is guessing at a Unicode block.
		{`\mathfrak{S}_n`, "Sn"},
		// KaTeX writes the TeX it was handed back into the MathML as an
		// annotation. Read along with everything else it would put the formula
		// in the label twice, once as symbols and once as backslashes, which is
		// what the next case would catch.
		{`\alpha`, "α"},
		// A span the engine refuses comes back as its own source, which is ugly
		// and true. Anything else would be a label that quietly says less than
		// the heading it was taken from.
		{`\thisisnotacommand`, `\thisisnotacommand`},
	}
	for _, c := range cases {
		if got := mathPlain(eng, c.tex, false); got != c.want {
			t.Errorf("mathPlain(%q) = %q, want %q", c.tex, got, c.want)
		}
	}
}

// The invisible operators are what MathML puts between a function and its
// argument so that a reader aloud says "Hom of". A reading system drawing the
// contents as plain text draws nothing for them, so they are bytes in a label
// that nobody can see and nobody can search for.
func TestMathPlainDropsTheInvisibleOperators(t *testing.T) {
	got := mathPlain(engine(t), `\operatorname{Hom}(E, F)`, false)
	if strings.ContainsAny(got, "⁡⁢⁣⁤") {
		t.Errorf("mathPlain kept an invisible operator: %q", got)
	}
	if !strings.HasPrefix(got, "Hom") {
		t.Errorf("mathPlain gave %q, want it to start Hom", got)
	}
}

func TestPlainMath(t *testing.T) {
	eng := engine(t)
	cases := []struct{ in, want string }{
		{`$p$-NHÓM`, "p-NHÓM"},
		{"no mathematics here", "no mathematics here"},
		// A span that opens and never closes is left exactly as it was found. A
		// title is not the place to be clever about a file that is broken, and
		// the audit reports the same file for the same reason.
		{`$p`, `$p`},
	}
	for _, c := range cases {
		if got := plainMath(eng, c.in); got != c.want {
			t.Errorf("plainMath(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

// sample is a volume small enough to write in a test and shaped like a real
// one: a chapter, a § with a numbered subsection whose title carries a formula,
// a statement, an exercise and a historical note.
func sample() *Volume {
	body := strings.Join([]string{
		"### 1. $p$-GROUPS {#alg-i-s1-no-1}",
		"",
		"A paragraph with $x \\in E$ in it.",
		"",
		"#### Proposition 1 {#alg-i-s1-prop-1 .statement}",
		"",
		"*Every $p$-group is nilpotent.*",
		"",
		"### 2. QUOTIENTS {#alg-i-s1-no-2}",
		"",
		"Another paragraph, which points at [Proposition 1](#alg-i-s1-prop-1).",
		"",
	}, "\n")
	sec := &Section{
		Kind: corpus.KindSection, Number: 1, Title: "Groups", Label: "alg-i-s1",
		Body: body, Path: "content/en/alg/I/01_s1_groups.md", Head: 1, Lang: "en",
		Statements: 1,
		Exercises: []*Exercise{{
			Number: 1, Label: "alg-i-s1-ex-1",
			Body: "Show that $x \\in E$.\n", Path: "content/en/alg/I/exercises/s1/01.md", Head: 1,
		}},
	}
	note := &Section{
		Kind: corpus.KindHistorical, Title: "Historical Note",
		Body: "A paragraph of history.\n", Path: "content/en/alg/I/historical_note.md",
		Head: 1, Lang: "en",
	}
	return &Volume{
		Meta: corpus.Book{
			ID: "test-i", Book: "alg", Lang: "en",
			Chapters: []string{"I"}, Pages: 100, PageWidth: 363.12, PageHeight: 565.56,
		},
		Lang:  "en",
		Title: "Algebra",
		Chapters: []*Chapter{{
			Numeral: "I", Title: "Algebraic Structures",
			Sections: []*Section{sec}, Historical: note,
		}},
	}
}

func writeSample(t *testing.T) (*EPUB, *zip.Reader) {
	t.Helper()
	file := filepath.Join(t.TempDir(), "book.epub")
	e, err := WriteEPUB(file, sample(), Options{Epoch: 1735689600})
	if err != nil {
		t.Fatalf("WriteEPUB: %v", err)
	}
	z, err := zip.OpenReader(file)
	if err != nil {
		t.Fatalf("reopening the epub: %v", err)
	}
	t.Cleanup(func() { z.Close() })
	return e, &z.Reader
}

func read(t *testing.T, z *zip.Reader, name string) string {
	t.Helper()
	for _, f := range z.File {
		if f.Name != name {
			continue
		}
		r, err := f.Open()
		if err != nil {
			t.Fatalf("opening %s: %v", name, err)
		}
		defer r.Close()
		b, err := io.ReadAll(r)
		if err != nil {
			t.Fatalf("reading %s: %v", name, err)
		}
		return string(b)
	}
	t.Fatalf("the container has no %s", name)
	return ""
}

// The mimetype is the one rule of the container format that is about the bytes
// of the zip rather than about their content: first entry, stored, no
// compression, so that a reading system knows what it has without unpacking
// anything.
func TestEPUBMimetypeIsFirstAndStored(t *testing.T) {
	_, z := writeSample(t)
	if len(z.File) == 0 {
		t.Fatal("the container is empty")
	}
	first := z.File[0]
	if first.Name != "mimetype" {
		t.Errorf("the first entry is %q, want mimetype", first.Name)
	}
	if first.Method != zip.Store {
		t.Errorf("the mimetype is compressed, want stored")
	}
	if got := read(t, z, "mimetype"); got != "application/epub+zip" {
		t.Errorf("the mimetype reads %q", got)
	}
}

// This is the regression for a real fault. The navigation label of a numbered
// subsection falls back to the heading for a language the volume was never
// printed in, and the heading arrives with its mathematics masked out by a
// placeholder built from NUL. Left there, the Vietnamese Algebra shipped a
// table of contents with NUL bytes in it, which is not well formed XML and
// which no reading system would open.
func TestNavigationHasNoPlaceholderLeftInIt(t *testing.T) {
	_, z := writeSample(t)
	nav := read(t, z, "EPUB/nav.xhtml")
	if strings.Contains(nav, "\x00") {
		t.Error("the navigation carries a NUL, so a math placeholder was never put back")
	}
	if !strings.Contains(nav, "1. p-GROUPS") {
		t.Errorf("the navigation has no plain text for the formula in a subsection title:\n%s", nav)
	}
}

func TestEveryDocumentIsWellFormedXML(t *testing.T) {
	_, z := writeSample(t)
	for _, f := range z.File {
		if !strings.HasSuffix(f.Name, ".xhtml") && !strings.HasSuffix(f.Name, ".opf") &&
			!strings.HasSuffix(f.Name, ".xml") && !strings.HasSuffix(f.Name, ".svg") {
			continue
		}
		dec := xml.NewDecoder(strings.NewReader(read(t, z, f.Name)))
		dec.Strict = true
		dec.Entity = xml.HTMLEntity
		for {
			_, err := dec.Token()
			if err == io.EOF {
				break
			}
			if err != nil {
				t.Errorf("%s: %v", f.Name, err)
				break
			}
		}
	}
}

// A cross reference that resolves inside the book has to become a link a reader
// can follow, and one that does not has to become the words alone rather than a
// link to nowhere.
func TestCrossReferenceBecomesALink(t *testing.T) {
	_, z := writeSample(t)
	page := read(t, z, "EPUB/text/alg-i-s1.xhtml")
	if !strings.Contains(page, `href="alg-i-s1.xhtml#alg-i-s1-prop-1"`) {
		t.Errorf("the reference to Proposition 1 is not a link:\n%s", page)
	}
}

// Every span of TeX in the volume goes out as MathML, which is what EPUB 3
// requires a reading system to support and what the site's own KaTeX HTML is
// not: the HTML is a pile of positioned spans that a reading system ignoring
// our stylesheet would draw as every symbol in a row at the same size.
func TestFormulaeGoOutAsMathML(t *testing.T) {
	e, z := writeSample(t)
	if e.Math == 0 {
		t.Fatal("the volume has formulae and none were counted")
	}
	if len(e.Refused) != 0 {
		t.Errorf("katex refused %d spans of the sample: %v", len(e.Refused), e.Refused)
	}
	page := read(t, z, "EPUB/text/alg-i-s1.xhtml")
	if !strings.Contains(page, "http://www.w3.org/1998/Math/MathML") {
		t.Error("the page has no MathML in it")
	}
	if strings.Contains(page, `class="katex-html"`) {
		t.Error("the page carries the KaTeX HTML, which needs a stylesheet no reading system has")
	}
}

func TestCoverSVGIsWellFormed(t *testing.T) {
	svg := CoverSVG(sample())
	dec := xml.NewDecoder(strings.NewReader(svg))
	dec.Strict = true
	for {
		_, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			t.Fatalf("the cover is not well formed: %v", err)
		}
	}
	// The yellow and the blue are measured off a photograph of the printing and
	// the audit looks for both in the rendered first page, so a change to either
	// here has to be a deliberate one.
	if !strings.Contains(svg, "#FEC746") {
		t.Error("the cover is not the printing's yellow")
	}
	if !strings.Contains(svg, "#0077B4") {
		t.Error("the cover has none of the title blue")
	}
	if !strings.Contains(svg, Author) || !strings.Contains(svg, Series) {
		t.Error("the cover is missing the wordmark or the series")
	}
}

// Two builds of the same content have to come out as the same bytes, which is
// what makes comparing this build against the last one worth doing at all.
func TestTwoBuildsAgree(t *testing.T) {
	dir := t.TempDir()
	one := filepath.Join(dir, "one.epub")
	two := filepath.Join(dir, "two.epub")
	for _, f := range []string{one, two} {
		if _, err := WriteEPUB(f, sample(), Options{Epoch: 1735689600}); err != nil {
			t.Fatalf("WriteEPUB: %v", err)
		}
	}
	a, b := readFile(t, one), readFile(t, two)
	if a != b {
		t.Errorf("two builds of the same volume came out as %d and %d bytes", len(a), len(b))
	}
}

func readFile(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(b)
}
