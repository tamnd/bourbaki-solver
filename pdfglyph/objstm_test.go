package pdfglyph

import (
	"bytes"
	"compress/zlib"
	"fmt"
	"strings"
	"testing"
)

// objStmPDF is a file built the way Lie chapters 7 to 9 is built: the font
// dictionary and its encoding are inside a compressed object stream, so the
// string /Differences does not occur anywhere in the file, and the file ends on
// a cross reference stream rather than a table.
//
// Object 12 is the /ToUnicode CMap, which is a stream and so is top level
// wherever else the file puts its dictionaries. It maps code 2 and says nothing
// about code 3, which is the shape the real volume has and the reason a page of
// it prints 4592 fewer glyphs than it was set with.
func objStmPDF(t *testing.T) []byte {
	t.Helper()
	inner := []struct {
		num  int
		body string
	}{
		{5, "<</Type/Font/Subtype/Type1/BaseFont/PPQRST+CMSY7/Encoding 6 0 R/ToUnicode 12 0 R>>"},
		{6, "<</Type/Encoding/Differences[2 /prime /negationslash]>>"},
	}
	var head, body bytes.Buffer
	for _, o := range inner {
		fmt.Fprintf(&head, "%d %d ", o.num, body.Len())
		body.WriteString(o.body + "\n")
	}
	var packed bytes.Buffer
	packed.Write(head.Bytes())
	packed.Write(body.Bytes())

	var z bytes.Buffer
	w := zlib.NewWriter(&z)
	if _, err := w.Write(packed.Bytes()); err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}

	cmap := "/CIDInit /ProcSet findresource begin\n12 dict begin\nbegincmap\n1 begincodespacerange\n<00> <FF>\nendcodespacerange\n1 beginbfchar\n<02> <2032>\nendbfchar\nendcmap\nend\nend\n"

	var out bytes.Buffer
	out.WriteString("%PDF-1.5\n")
	fmt.Fprintf(&out, "10 0 obj\n<</Type/ObjStm/N %d/First %d/Filter/FlateDecode/Length %d>>\nstream\n",
		len(inner), head.Len(), z.Len())
	out.Write(z.Bytes())
	out.WriteString("\nendstream\nendobj\n")
	fmt.Fprintf(&out, "12 0 obj\n<</Length %d>>\nstream\n%s\nendstream\nendobj\n", len(cmap), cmap)
	xrefAt := out.Len()
	out.WriteString("13 0 obj\n<</Type/XRef/W[1 4 2]/Size 14/Root 1 0 R/Length 0>>\nstream\n\nendstream\nendobj\n")
	fmt.Fprintf(&out, "startxref\n%d\n%%%%EOF\n", xrefAt)
	return out.Bytes()
}

// The names inside an object stream are rewritten, and the rewrite arrives as an
// object appended to the end of the file rather than as an edit to the stream,
// which cannot be repacked to the size it came in at.
func TestRewriteReadsAnObjectStream(t *testing.T) {
	in := objStmPDF(t)
	out, res, err := Rewrite(in)
	if err != nil {
		t.Fatal(err)
	}
	if res.Encodings != 1 {
		t.Fatalf("changed %d encodings, want 1", res.Encodings)
	}
	if res.Names["prime"] != 1 || res.Names["negationslash"] != 1 {
		t.Fatalf("rewrote %v, want one prime and one negation stroke", res.Names)
	}
	if !bytes.HasPrefix(out, in) {
		t.Fatal("the file the update was written onto was changed")
	}
	added := string(out[len(in):])
	if !strings.Contains(added, "6 0 obj") {
		t.Error("the encoding was not appended under its own number")
	}
	if !strings.Contains(added, "/zero") || !strings.Contains(added, "/six") {
		t.Errorf("the appended encoding is not the rewritten one:\n%s", added)
	}
	if !strings.Contains(added, "/Prev") || !strings.Contains(added, "/Type/XRef") {
		t.Error("the update did not write a cross reference stream carrying /Prev")
	}
	if !strings.HasSuffix(added, "%%EOF\n") {
		t.Error("the update does not end the file")
	}
}

// The CMap is given the code it was missing and left alone at the code it
// already had, since it is the only good reading of every code this package has
// nothing to say about.
func TestRewritePatchesOnlyTheMissingCodes(t *testing.T) {
	in := objStmPDF(t)
	out, res, err := Rewrite(in)
	if err != nil {
		t.Fatal(err)
	}
	if res.Unicode != 1 {
		t.Fatalf("patched %d CMaps, want 1", res.Unicode)
	}
	added := string(out[len(in):])
	if !strings.Contains(added, "<03> <0036>") {
		t.Errorf("the negation stroke was not added at code 3:\n%s", added)
	}
	if strings.Contains(added, "<02> <0030>") {
		t.Error("code 2 was patched and the CMap already mapped it")
	}
	if !strings.Contains(added, "<02> <2032>") {
		t.Error("the mapping the CMap already had was not carried over")
	}
}

// A file that keeps its cross reference as a table gets a table back. Algèbre
// chapitre 8 is such a file, and the first version of the update wrote it a
// cross reference stream, which poppler read as a file with no pages in it.
func TestAppendUpdateWritesTheKindOfCrossReferenceTheFileHas(t *testing.T) {
	const body = "%PDF-1.4\n1 0 obj\n<</Type/Catalog>>\nendobj\n"
	classic := body + fmt.Sprintf("xref\n0 1\n0000000000 65535 f \ntrailer\n<</Size 2/Root 1 0 R/ID[<AB> <AB>]>>\nstartxref\n%d\n%%%%EOF\n", len(body))
	out, err := appendUpdate([]byte(classic), []update{{num: 1, body: []byte("<</Type/Catalog/X 1>>")}})
	if err != nil {
		t.Fatal(err)
	}
	added := string(out[len(classic):])
	for _, want := range []string{"1 0 obj", "xref\n1 1\n", "trailer", "/Size 2", "/Root 1 0 R", fmt.Sprintf("/Prev %d", len(body)), "/ID[<AB> <AB>]"} {
		if !strings.Contains(added, want) {
			t.Errorf("the update has no %q:\n%s", want, added)
		}
	}
	if strings.Contains(added, "/Type/XRef") {
		t.Error("a file with a cross reference table was given a cross reference stream")
	}
}

// Consecutive object numbers go in one subsection and a gap starts another,
// which is what both kinds of cross reference are written in.
func TestSectionsGroupConsecutiveNumbers(t *testing.T) {
	got := sections(map[int]int{4: 0, 5: 0, 6: 0, 9: 0, 20: 0, 21: 0})
	want := [][]int{{4, 5, 6}, {9}, {20, 21}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Errorf("sections = %v, want %v", got, want)
	}
}

func TestCMapCodes(t *testing.T) {
	const cmap = "2 beginbfchar\n<41> <0041>\n<42> <0000>\nendbfchar\n1 beginbfrange\n<50> <52> <0050>\nendbfrange\n"
	got := cmapCodes([]byte(cmap))
	for _, code := range []int{0x41, 0x50, 0x51, 0x52} {
		if !got[code] {
			t.Errorf("code %#x is mapped and was not read", code)
		}
	}
	// A destination of nothing but zeros is a tool writing down that it had no
	// Unicode for the code, and poppler prints nothing for it.
	if got[0x42] {
		t.Error("a code mapped to zero was read as covered")
	}
	if len(got) != 4 {
		t.Errorf("read %d codes, want 4: %v", len(got), got)
	}
}

// A code written to another width than the CMap's codespace is not read at all,
// so an added entry is written to the width the entries already there use.
func TestCodeWidth(t *testing.T) {
	for in, want := range map[string]int{
		"1 beginbfchar\n<41> <0041>\nendbfchar\n":            2,
		"1 beginbfchar\n<0041> <0041>\nendbfchar\n":          4,
		"1 beginbfrange\n<0010> <0020> <0041>\nendbfrange\n": 4,
		"endcmap\n": 2,
	} {
		if got := codeWidth([]byte(in)); got != want {
			t.Errorf("codeWidth(%q) = %d, want %d", in, got, want)
		}
	}
}

// Every replacement the tables name has to have a character behind it, since
// that character is what a CMap entry for the same code has to say if the two
// are not to contradict each other.
func TestGlyphRuneKnowsEveryReplacement(t *testing.T) {
	for _, table := range []map[string]string{cmexNames, mathNames} {
		for name, want := range table {
			if _, ok := glyphRune(want); !ok {
				t.Errorf("%s goes to /%s and nothing knows what character that is", name, want)
			}
		}
	}
}

func TestGlyphRune(t *testing.T) {
	for in, want := range map[string]rune{
		"zero": '0', "six": '6', "bracketleft": '[', "backslash": '\\',
		"L": 'L', "a": 'a', "uniF8EB": '\uf8eb', "uni0302": '\u0302',
	} {
		got, ok := glyphRune(in)
		if !ok || got != want {
			t.Errorf("glyphRune(%q) = %q %v, want %q", in, got, ok, want)
		}
	}
	for _, in := range []string{"", "summationtext", "uniZZZZ", "uni12"} {
		if _, ok := glyphRune(in); ok {
			t.Errorf("glyphRune(%q) claimed to know a character", in)
		}
	}
}
