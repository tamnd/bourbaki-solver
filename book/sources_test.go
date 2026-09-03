package book

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// A TeX error names a line of book.tex, and book.tex is a file nobody wrote: it
// is made and deleted on every build and its line 2350 is not a place anybody
// can go. These are the tests that the line comes back as a file under content/
// that somebody can open.

func TestTheWriterMarksWhichCorpusFileEachStretchOfTheDocumentCameFrom(t *testing.T) {
	v := &Volume{
		Lang: "en",
		Meta: corpus.Book{ID: "alg-i-iii", Lang: "en", Title: "Algebra", Chapters: []string{"I"}},
		Chapters: []*Chapter{{
			Numeral: "I", Title: "Algebraic Structures",
			Sections: []*Section{
				{Path: "content/en/alg/I/01_s1_laws.md", Number: 1, Title: "Laws", Body: "Alpha.\n"},
				{Path: "content/en/alg/I/02_s2_groups.md", Number: 2, Title: "Groups", Body: "Beta.\n"},
			},
		}},
	}
	d, err := Write(v)
	if err != nil {
		t.Fatal(err)
	}
	if len(d.Sources) != 2 {
		t.Fatalf("recorded %d sources, want the two §§: %+v", len(d.Sources), d.Sources)
	}
	// The line each marker claims has to be the line the marker is actually on,
	// because that is the arithmetic the typesetter's own line numbers are
	// compared against. Counted here off the finished string, which is what gets
	// written to disk and is what TeX reads.
	lines := strings.Split(d.TeX, "\n")
	for _, s := range d.Sources {
		if got := lines[s.Line-1]; got != sourceMark+s.Path {
			t.Errorf("Sources says %s begins at line %d, and line %d is %q", s.Path, s.Line, s.Line, got)
		}
	}
	// A line inside the second § belongs to the second §, a line inside the
	// first to the first, and a line of the front matter to neither. The last is
	// the case that must not guess: the cover, the half title and the contents
	// come out of the class and there is no file to blame them on.
	if got := d.At(d.Sources[1].Line + 1); got != "content/en/alg/I/02_s2_groups.md" {
		t.Errorf("At(inside the second §) = %q", got)
	}
	if got := d.At(d.Sources[0].Line + 1); got != "content/en/alg/I/01_s1_laws.md" {
		t.Errorf("At(inside the first §) = %q", got)
	}
	if got := d.At(1); got != "" {
		t.Errorf("At(the \\documentclass line) = %q, want nothing rather than a guess", got)
	}
}

func TestATeXErrorIsReportedAgainstTheSectionAndNotAgainstTheGeneratedFile(t *testing.T) {
	d := &Document{TeX: strings.Join([]string{
		`\documentclass{bourbaki}`,                    // 1
		`\begin{document}`,                            // 2
		sourceMark + "content/en/alg/I/01_s1_laws.md", // 3
		`Alpha.`, // 4
		sourceMark + "content/en/alg/I/02_s2_groups.md", // 5
		`Beta.`,          // 6
		`\end{document}`, // 7
	}, "\n")}
	d.index()

	// The shape of a real log. TeX puts the message first and the line a couple
	// of lines under it, after whatever context it decided to print.
	log := `This is XeTeX, Version 3.141592653
! Double superscript.
<recently read> \bstate
l.6 ...an element of $K^*$ such that $x^-^
                                          1 does not belong to $A$. ...
! Emergency stop.
Output written on book.xdv (12 pages, 1377952 bytes).
`
	dir := t.TempDir()
	path := filepath.Join(dir, "book.log")
	if err := os.WriteFile(path, []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &Build{Dir: dir, Log: path, PDF: filepath.Join(dir, "book.pdf")}
	if err := b.readLog(d); err != nil {
		t.Fatal(err)
	}
	if len(b.Errors) != 2 {
		t.Fatalf("errors = %q, want the superscript and the stop", b.Errors)
	}
	if !strings.Contains(b.Errors[0], "content/en/alg/I/02_s2_groups.md") {
		t.Errorf("first error = %q, want it to name the § it came out of", b.Errors[0])
	}
	// The emergency stop is about the run and not about a line of it, so there is
	// nothing to attribute it to and it must not borrow the last file that was
	// mentioned. An error pinned to the wrong § is worse than one pinned to
	// nothing, because somebody goes and reads that §.
	if strings.Contains(b.Errors[1], "content/") {
		t.Errorf("second error = %q, want no file on an error that named no line", b.Errors[1])
	}
}

func TestAnErrorReadWithNoDocumentBesideItIsLeftAsTeXGaveIt(t *testing.T) {
	// readLog is also what a test over a canned log calls, and a log with no
	// document beside it is still worth reading. A nil document must not be a
	// panic and must not be a guess.
	dir := t.TempDir()
	path := filepath.Join(dir, "book.log")
	if err := os.WriteFile(path, []byte("! Missing } inserted.\nl.6 x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &Build{Dir: dir, Log: path, PDF: filepath.Join(dir, "book.pdf")}
	if err := b.readLog(nil); err != nil {
		t.Fatal(err)
	}
	if len(b.Errors) != 1 || b.Errors[0] != "! Missing } inserted." {
		t.Fatalf("errors = %q", b.Errors)
	}
}
