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
	if err := b.readLog(nil); err != nil {
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
	if err := b.readLog(nil); err != nil {
		t.Fatal(err)
	}
	if len(b.Errors) != 0 {
		t.Errorf("errors = %q, want none", b.Errors)
	}
	if b.Pages != 126 {
		t.Errorf("pages = %d, want 126", b.Pages)
	}
}

// XeTeX names the font it could not find a character in by file and by the whole
// OpenType feature string, that runs past the width the log is wrapped at, and
// the exclamation mark that ends the message lands on the next line. The pattern
// wanted the exclamation mark on the same line, so under the OpenType fonts this
// found nothing at all: the French history volume dropped four characters over
// six places, two of them the Greek of a quotation from Euclid, and the audit
// said no character had been lost.
//
// The wrap also cuts the character itself. It is by byte and not by character,
// so the three bytes of a CJK character arrive as one and a half and the rest is
// mojibake. The codepoint beside it is ASCII and survives, and the report is
// built from that.
func TestReadLogSeesAGlyphWhoseFontNameWrapped(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "book.log")
	log := "Missing character: There is no ὸ (U+1F78) in font [texgyretermes-regular.otf]/O\n" +
		"T:script=latn;language=dflt;mapping=tex-text;!\n" +
		"Missing character: There is no \xe5\xae (U+5B9E) in font [texgyretermes-regular.otf]/O\n" +
		"T:script=latn;language=dflt;mapping=tex-text;!\n" +
		"Output written on book.xdv (374 pages, 1377952 bytes).\n"
	if err := os.WriteFile(path, []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &Build{Dir: dir, Log: path, PDF: filepath.Join(dir, "book.pdf")}
	if err := b.readLog(nil); err != nil {
		t.Fatal(err)
	}
	if len(b.MissingGlyphs) != 2 {
		t.Fatalf("missing glyphs = %q, want two", b.MissingGlyphs)
	}
	for i, w := range []string{
		"ὸ (U+1F78) in [texgyretermes-regular.otf]/O",
		"实 (U+5B9E) in [texgyretermes-regular.otf]/O",
	} {
		if b.MissingGlyphs[i] != w {
			t.Errorf("glyph %d = %q, want %q", i, b.MissingGlyphs[i], w)
		}
	}
}

// The box TeX dumps under an overfull warning is the content of the box with
// every font change spelled out, wrapped at the log's width with no regard for
// what it cuts. An exclamation mark lands at the start of a line often enough:
// it is the slot the arrow sits in in cmsy, so a volume full of long exact
// sequences dumps line after line starting "! []". The French Algebre X failed
// the typesetter gate on one of those, on a warning the same log had already
// counted correctly one line above.
func TestReadLogDoesNotReadABoxDumpAsAnError(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "book.log")
	log := "This is XeTeX, Version 3.141592653\n" +
		"Overfull \\hbox (5.94576pt too wide) detected at line 7692\n" +
		"\\T1/lmr/m/n/10 0 \\OMS/cmsy/m/n/10 ! []\\T1/lmr/m/n/10 (\\OML/cmm/m/it/10 u\n" +
		"! [] \\OML/cmm/m/it/10 v \\OMS/cmsy/m/n/10 ! \\T1/lmr/m/n/10 0\\OML/cmm/m/it/10 :\n" +
		" []\n" +
		"\n" +
		"! Undefined control sequence.\n" +
		"l.900 \\sqleftarrow \n" +
		"Output written on book.xdv (267 pages, 1377952 bytes).\n"
	if err := os.WriteFile(path, []byte(log), 0o644); err != nil {
		t.Fatal(err)
	}
	b := &Build{Dir: dir, Log: path, PDF: filepath.Join(dir, "book.pdf")}
	if err := b.readLog(nil); err != nil {
		t.Fatal(err)
	}
	// The real error after the dump still counts, and the overfull is still one.
	if len(b.Errors) != 1 || b.Errors[0] != "! Undefined control sequence." {
		t.Errorf("errors = %q, want just the undefined control sequence", b.Errors)
	}
	if b.Overfull != 1 {
		t.Errorf("overfull = %d, want 1", b.Overfull)
	}
}
