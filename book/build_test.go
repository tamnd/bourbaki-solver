package book

import (
	"os"
	"path/filepath"
	"testing"
)

// The log this reads is the shape of a real one. The Double superscript came
// out of Algebre commutative VIII SS 2, where x inverse had been read as a
// unicode superscript with an unclosed dollar in front of it, and the Missing }
// out of Algebre IX SS 2, where the opening of a subscript had been read twice.
// Both shipped, both were reported as 20 of 22 checks passed, and neither was
// seen because the only error this looked for was an undefined control
// sequence.
const sampleLog = `This is XeTeX, Version 3.141592653
Overfull \hbox (12.0pt too wide) in paragraph at lines 40--42
! Double superscript.
l.2350 ...$ be an element of $K^*$ such that $x^-^
                                                  1 does not belong to $A$. ...
! Undefined control sequence.
l.900 \sqleftarrow 
                   u
Missing character: There is no ` + "đ" + ` in font cmr10!
! Missing } inserted.
l.965 ...E over A and $(a_j)_{a_j)_{j=1,\ldots,q}$
! Missing } inserted.
l.965 ...E over A and $(a_j)_{a_j)_{j=1,\ldots,q}$
LaTeX Warning: There were undefined references.
Output written on book.xdv (62 pages, 1377952 bytes).
`

func TestReadLogSeesEveryClassOfTeXError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "book.log")
	if err := os.WriteFile(path, []byte(sampleLog), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &Build{Dir: dir, Log: path, PDF: filepath.Join(dir, "book.pdf")}
	if err := b.readLog(); err != nil {
		t.Fatal(err)
	}

	// Three distinct errors and not four: the Missing } is printed twice, once
	// for each pass over the chapter, and that is one fault.
	if len(b.Errors) != 3 {
		t.Fatalf("errors = %d, want 3: %q", len(b.Errors), b.Errors)
	}
	want := []string{"! Double superscript.", "! Undefined control sequence.", "! Missing } inserted."}
	for i, w := range want {
		if b.Errors[i] != w {
			t.Errorf("error %d = %q, want %q", i, b.Errors[i], w)
		}
	}

	// The undefined control sequence is still picked up by name, because the
	// audit reports which command the class is missing and not just that one is.
	if len(b.Undefined) != 1 || b.Undefined[0] != `\sqleftarrow` {
		t.Errorf("undefined = %q, want [\\sqleftarrow]", b.Undefined)
	}
	if b.Overfull != 1 {
		t.Errorf("overfull = %d, want 1", b.Overfull)
	}
	if b.Unresolved != 1 {
		t.Errorf("unresolved = %d, want 1", b.Unresolved)
	}
	if len(b.MissingGlyphs) != 1 {
		t.Errorf("missing glyphs = %q, want one", b.MissingGlyphs)
	}
	if b.Pages != 62 {
		t.Errorf("pages = %d, want 62", b.Pages)
	}
}

func TestReadLogOnACleanLogFindsNoErrors(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "book.log")
	clean := "This is XeTeX, Version 3.141592653\n" +
		"Overfull \\hbox (12.0pt too wide) in paragraph at lines 40--42\n" +
		"Output written on book.xdv (126 pages, 631892 bytes).\n"
	if err := os.WriteFile(path, []byte(clean), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &Build{Dir: dir, Log: path, PDF: filepath.Join(dir, "book.pdf")}
	if err := b.readLog(); err != nil {
		t.Fatal(err)
	}
	if len(b.Errors) != 0 {
		t.Errorf("errors = %q, want none", b.Errors)
	}
	if b.Pages != 126 {
		t.Errorf("pages = %d, want 126", b.Pages)
	}
}
