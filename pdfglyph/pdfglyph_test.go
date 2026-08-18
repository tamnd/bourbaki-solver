package pdfglyph

import (
	"strings"
	"testing"
)

// Every fixture below is real, cut out of Algèbre chapitre 8 (2012, Springer)
// with the object numbers and the byte layout it has on disk. Objects 194 and
// 198 are the extension font and its encoding, 189 and 193 the eight point
// symbol font and its own, and 193 is where the 355 lost primes of that size
// come from.
const algVIIIFR = `%PDF-1.4
189 0 obj
<<
/ToUnicode 190 0 R
/Subtype /Type1
/FontDescriptor 191 0 R
/BaseFont /LMMathSymbols8-Regular
/Encoding 193 0 R
/Widths [500 500]
>>
endobj
193 0 obj
<<
/Type /Encoding
/Differences [2 /element /prime /negationslash /asteriskmath /infinity /circlemultiply /openbullet /perpendicular /circleplus /logicalor 92 /backslash 123 /braceleft /bar /braceright 138 /minus 215 /multiply ]
>>
endobj
194 0 obj
<<
/ToUnicode 195 0 R
/Subtype /Type1
/FontDescriptor 196 0 R
/BaseFont /CMEX10
/Encoding 198 0 R
/Widths [500 500 833]
>>
endobj
198 0 obj
<<
/Type /Encoding
/Differences [2 /intersectiontext /summationtext /circleplustext /parenleftBig /parenrightBig /producttext /parenleftbigg /summationdisplay /parenrightbigg /circleplusdisplay /tildewide /productdisplay /parenlefttp /parenleftex /parenleftbt /parenrighttp /parenrightex /parenrightbt /intersectiondisplay /parenleftBigg /parenrightBigg /parenleftbig /parenrightbig /tildewider /bracelefttp /braceex /braceleftmid /braceleftbt /circlemultiplytext /hatwide /integraldisplay /hatwider ]
>>
endobj
trailer
<< /Root 1 0 R >>
`

func TestRewriteKeepsEveryOffset(t *testing.T) {
	out, _, err := Rewrite([]byte(algVIIIFR))
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != len(algVIIIFR) {
		t.Fatalf("length changed: %d bytes in, %d out", len(algVIIIFR), len(out))
	}
}

func TestRewriteNamesTheExtensionFont(t *testing.T) {
	out, res, err := Rewrite([]byte(algVIIIFR))
	if err != nil {
		t.Fatal(err)
	}
	if res.Encodings != 2 {
		t.Errorf("changed %d encodings, want 2", res.Encodings)
	}
	// The direct sum of page 64, the one that made this whole package: the
	// French names it circleplustext and prints nothing, the English leaves it
	// at code L and prints L, and extract's cmex table reads L as \bigoplus.
	for _, want := range []string{"/L ", "/M ", "/P ", "/X ", "/e ", "/c "} {
		if !strings.Contains(string(out), want) {
			t.Errorf("no %q in the rewritten encoding", want)
		}
	}
	// The extensible pieces keep their private use area codes so that a page
	// built out of them is flagged rather than read wrong.
	if !strings.Contains(string(out), "/uniF8EB ") {
		t.Error("the left paren top did not go to the private use area")
	}
	// Nothing of the original TeX names is left where a name was replaced.
	for _, gone := range []string{"circleplustext", "summationdisplay", "parenlefttp", "tildewide"} {
		if strings.Contains(string(out), gone) {
			t.Errorf("%s survived the rewrite", gone)
		}
	}
}

func TestRewriteNamesThePrime(t *testing.T) {
	out, res, err := Rewrite([]byte(algVIIIFR))
	if err != nil {
		t.Fatal(err)
	}
	if res.Names["prime"] != 1 {
		t.Fatalf("replaced prime %d times, want 1", res.Names["prime"])
	}
	if res.Names["negationslash"] != 1 {
		t.Errorf("replaced negationslash %d times, want 1", res.Names["negationslash"])
	}
	if !strings.Contains(string(out), "/element /zero  /six ") {
		t.Error("prime and negationslash did not land on zero and six")
	}
}

// A name the extension font shares with the symbol font must be read as the
// font it is in. circleplus is a symbol in one and circleplustext an operator
// in the other, and reading either by the wrong table puts the wrong operator
// on the page.
func TestRewriteLeavesTheSymbolFontAlone(t *testing.T) {
	out, _, err := Rewrite([]byte(algVIIIFR))
	if err != nil {
		t.Fatal(err)
	}
	for _, keep := range []string{"/element", "/circleplus /logicalor", "/asteriskmath", "/multiply"} {
		if !strings.Contains(string(out), keep) {
			t.Errorf("%s was rewritten and should not have been", keep)
		}
	}
}

// The English printing has no /Differences to rewrite, so preparing it has to
// be a no-op rather than an error.
func TestRewriteWithoutEncodings(t *testing.T) {
	const en = "%PDF-1.6\n1 0 obj\n<< /Type /Font /BaseFont /AAAAAA+CMEX10 /Subtype /Type1 >>\nendobj\n"
	out, res, err := Rewrite([]byte(en))
	if err != nil {
		t.Fatal(err)
	}
	if res.Total() != 0 || res.Encodings != 0 {
		t.Errorf("changed %d names in %d encodings, want none", res.Total(), res.Encodings)
	}
	if string(out) != en {
		t.Error("the bytes changed")
	}
}

// The ten point symbol font of Lie groups and Lie algebras, chapters 7 to 9,
// cut out of the object stream it is compressed into. The double bar of the
// Hilbert norm of page 360 is the bardbl at the end of the run of names.
const lieVIIIX = `%PDF-1.5
41 0 obj
<<
/Subtype /Type1
/BaseFont /BXFHUL+LMMathSymbols10
/FontDescriptor 42 0 R
/Encoding 43 0 R
>>
endobj
43 0 obj
<<
/Type /Encoding
/Differences [1/greaterequal/arrowright/element/propersubset/negationslash/circlemultiply/arrowdblleft/arrowdblright/mapsto/circleplus/intersection/lessequal/propersuperset/openbullet/infinity/logicaland/union/angbracketleft/angbracketright/bardbl/equivalence/aleph/radical/lessmuch/greatermuch/asteriskmath 123/braceleft/bar/braceright 138/minus 167/section 177/plusminus 182/paragraph/periodcentered 215/multiply]
>>
endobj
`

// The norm bars of Lie 9 are drawn out of code 0x6B of the symbol font and the
// French style printing names that code bardbl, which poppler cannot resolve:
// every one of them arrived as an empty run, so page 360 said A_u^2_2 where it
// prints the norm of A_u squared and wrote the norm the sentence names as
// nothing at all. The replacement is k because 0x6B is where the double bar
// sits, and extract's cmsy table already reads k back as the norm.
func TestRewriteNamesTheDoubleBar(t *testing.T) {
	out, res, err := Rewrite([]byte(lieVIIIX))
	if err != nil {
		t.Fatal(err)
	}
	if res.Names["bardbl"] != 1 {
		t.Fatalf("replaced bardbl %d times, want 1", res.Names["bardbl"])
	}
	if !strings.Contains(string(out), "/k     /equivalence") {
		t.Error("the double bar did not land on k")
	}
	if len(res.Unknown) != 0 {
		t.Errorf("reported %v, and every name of this encoding is known", res.Unknown)
	}
}

// A glyph name that only TeX uses and that this package has no replacement for
// is reported, because the way it fails otherwise is a page that reads well
// with a symbol missing out of the middle of a formula.
func TestRewriteReportsUnknownNames(t *testing.T) {
	const odd = `%PDF-1.4
1 0 obj
<< /Subtype /Type1 /BaseFont /XXXXXX+CMEX10 /Encoding 2 0 R >>
endobj
2 0 obj
<< /Type /Encoding /Differences [2 /summationtext /contourintegraldisplay ] >>
endobj
`
	_, res, err := Rewrite([]byte(odd))
	if err != nil {
		t.Fatal(err)
	}
	if res.Unknown["CMEX10 contourintegraldisplay"] != 1 {
		t.Errorf("unknown names are %v, want the contour integral in CMEX10", res.Unknown)
	}
	if len(res.Unknown) != 1 {
		t.Errorf("reported %v, and the summation is known", res.Unknown)
	}
}

// A stream object whose dictionary happens to carry the word Encoding must not
// be read as an encoding, and the compressed bytes after it must not be read as
// names. This is the bug that made a first pass at this report Filter and
// Length as glyphs.
func TestRewriteStopsAtStreams(t *testing.T) {
	const withStream = `%PDF-1.4
1 0 obj
<< /Type /Encoding /Filter /FlateDecode /Length 9 >>
stream
/prime /summationtext ab
endstream
endobj
`
	out, res, err := Rewrite([]byte(withStream))
	if err != nil {
		t.Fatal(err)
	}
	if res.Total() != 0 {
		t.Errorf("rewrote %d names inside a stream", res.Total())
	}
	if string(out) != withStream {
		t.Error("stream bytes changed")
	}
}

func TestBaseStripsTheSubsetTag(t *testing.T) {
	for in, want := range map[string]string{
		"XAEWAV+CMEX10":          "CMEX10",
		"CMEX10":                 "CMEX10",
		"LMMathSymbols8-Regular": "LMMathSymbols8-Regular",
	} {
		if got := base(in); got != want {
			t.Errorf("base(%q) = %q, want %q", in, got, want)
		}
	}
}

// No replacement may be longer than the shortest TeX name it stands in for,
// since the file is rewritten in place and a longer name would have to push
// every byte after it along.
func TestEveryReplacementFits(t *testing.T) {
	for _, table := range []map[string]string{cmexNames, mathNames} {
		for name, want := range table {
			if len(want) > len(name) {
				t.Errorf("/%s does not fit where /%s was", want, name)
			}
		}
	}
}

// Two glyphs of one font landing on one code is a silent wrong reading, so the
// tables have to be injective inside a font. The delimiter sizes are the
// deliberate exception: four sizes of one bracket are one bracket in LaTeX.
func TestReplacementsDoNotCollide(t *testing.T) {
	seen := map[string]string{}
	for name, want := range cmexNames {
		if strings.HasSuffix(name, "big") || strings.HasSuffix(name, "Big") ||
			strings.HasSuffix(name, "bigg") || strings.HasSuffix(name, "Bigg") {
			continue
		}
		if old, ok := seen[want]; ok {
			t.Errorf("%s and %s both go to /%s", old, name, want)
		}
		seen[want] = name
	}
	seen = map[string]string{}
	for name, want := range mathNames {
		if strings.HasSuffix(name, "wide") || strings.HasSuffix(name, "wider") ||
			strings.HasSuffix(name, "widest") {
			continue
		}
		if old, ok := seen[want]; ok {
			t.Errorf("%s and %s both go to /%s", old, name, want)
		}
		seen[want] = name
	}
}

// A glyph name from the Adobe list is not a TeX name however it ends. The first
// pass at this reported eleven accented capitals as lost mathematics, because
// circumflex ends the way the extensible pieces of a big parenthesis do.
func TestTexNameLeavesTheAdobeListAlone(t *testing.T) {
	for _, name := range []string{
		"circumflex", "Acircumflex", "ecircumflex", "ocircumflex",
		"element", "emptyset", "backslash", "multiply", "asteriskmath",
	} {
		if texName.MatchString(name) {
			t.Errorf("%s was read as a TeX name", name)
		}
	}
	for _, name := range []string{
		"summationtext", "circleplusdisplay", "tildewide", "hatwider",
		"parenleftBigg", "parenlefttp", "parenleftex", "braceex",
		"braceleftmid", "bracerightbt", "arrowvertex", "arrowvertexdbl",
	} {
		if !texName.MatchString(name) {
			t.Errorf("%s was not read as a TeX name", name)
		}
	}
}
