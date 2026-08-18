package pdfglyph

import (
	"fmt"
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

// The mathematics italic of the same volume, with the CMap that skips the one
// code it could not name. varpi is code 0x17 and the CMap lists 0x16 and 0x18
// on either side of it.
const lieVIIIXMath = `%PDF-1.5
51 0 obj
<<
/Type /Font
/Subtype /Type1
/BaseFont /DGLMMN+CMMI10
/Encoding 52 0 R
/ToUnicode 53 0 R
>>
endobj
52 0 obj
<<
/Type /Encoding
/Differences [1/lambda/rho/nu/gamma/alpha/beta/pi/phi1/psi/Omega/theta/sigma/tau/chi/epsilon/Lambda/Delta/Phi/delta/Sigma/iota/omega/pi1/Gamma/kappa/Pi/zeta/xi/eta/Psi/partialdiff/Theta/theta1 44/comma 181/mu]
>>
endobj
53 0 obj
<< /Length 260 >>
stream
/CIDInit /ProcSet findresource begin 12 dict begin begincmap
/CMapName /AAAAAA+F9+0 def
/CMapType 2 def
1 begincodespacerange <01> <b5> endcodespacerange
3 beginbfchar
<16> <03C9>
<18> <0393>
<b5> <00B5>
endbfchar
endcmap CMapName currentdict /CMap defineresource pop end end
endstream
endobj
`

// lieMath is that font with a cross reference under it, which an incremental
// update needs and the fixtures that append nothing do without.
func lieMath() string {
	return lieVIIIXMath + fmt.Sprintf(
		"xref\n0 1\n0000000000 65535 f \ntrailer\n<</Size 54/Root 1 0 R>>\nstartxref\n%d\n%%%%EOF\n",
		len(lieVIIIXMath))
}

// varpi has no name outside TeX and no replacement that fits in three bytes, so
// the encoding is left alone and the CMap is told what the code means. Lie 7 to
// 9 prints the fundamental weights with it, and Corollary 1 of § 7 no. 3 came
// out as ( )_{alpha in B} with the letter gone.
func TestRewriteCodesTheVariantPi(t *testing.T) {
	out, res, err := Rewrite([]byte(lieMath()))
	if err != nil {
		t.Fatal(err)
	}
	if res.Coded["pi1"] != 1 {
		t.Fatalf("coded %v, want one pi1", res.Coded)
	}
	if res.Total() != 0 || res.Encodings != 0 {
		t.Errorf("rewrote %d names in %d encodings, and this one is left alone", res.Total(), res.Encodings)
	}
	if !strings.Contains(string(out), "/omega/pi1/Gamma") {
		t.Error("the encoding was rewritten")
	}
	if res.Unicode != 1 {
		t.Fatalf("patched %d CMaps, want 1", res.Unicode)
	}
	// The codes the CMap already had are kept, since the CMap is the only good
	// reading of every name that is in the Adobe list.
	if !strings.Contains(string(out), "<17> <03D6>") {
		t.Error("the CMap was not told what code 0x17 is")
	}
	for _, keep := range []string{"<16> <03C9>", "<18> <0393>"} {
		if !strings.Contains(string(out), keep) {
			t.Errorf("%s was lost out of the CMap", keep)
		}
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
// So are the two ends of a vertical arrow, which are two ends of one arrow and
// go where its shaft goes.
func TestReplacementsDoNotCollide(t *testing.T) {
	seen := map[string]string{}
	for name, want := range cmexNames {
		if strings.HasSuffix(name, "big") || strings.HasSuffix(name, "Big") ||
			strings.HasSuffix(name, "bigg") || strings.HasSuffix(name, "Bigg") {
			continue
		}
		if name == "arrowtp" || name == "arrowbt" {
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

// The double angle brackets of the pairing on the dual of a compact group.
//
// These sit at codes 0x1C and 0x1D of the symbol font, which are control
// characters, so poppler has no name to read them by and no code to fall back
// on either. Page 314 of Lie chapters 7 to 9 printed "a bilinear form   ,   on"
// and the display under it "= 2 pi i a, b .", and nothing said so: texName
// matches neither name, so the report this package writes never mentioned them,
// and the run was empty rather than wrong so no rule of the audit could either.
func TestRewriteNamesTheDoubleAngleBrackets(t *testing.T) {
	out, res, err := Rewrite([]byte(lieVIIIX))
	if err != nil {
		t.Fatal(err)
	}
	if res.Names["lessmuch"] != 1 || res.Names["greatermuch"] != 1 {
		t.Fatalf("replaced lessmuch %d and greatermuch %d times, want 1 each",
			res.Names["lessmuch"], res.Names["greatermuch"])
	}
	if !strings.Contains(string(out), "/uni226A /uni226B    /asteriskmath") {
		t.Error("the brackets did not land on U+226A and U+226B")
	}
}

// The AMS symbol font of the same volume. The domination relation between two
// positive functions on a compact group is drawn out of it, and so is the
// corner the Weyl integration formula writes on a form induced from a subgroup.
// Neither name is in the Adobe list: page 370 lost the relation 14 times in the
// paragraph that introduces it, and page 343 wrote omega_G with the corner gone
// off it.
const lieVIIIXAMS = `%PDF-1.5
61 0 obj
<<
/Type /Font
/Subtype /Type1
/BaseFont /DHHBJN+MSAM10
/Encoding 62 0 R
>>
endobj
62 0 obj
<<
/Type /Encoding
/Differences [52/precedesorcurly 65/rightanglesw 123/complement]
>>
endobj
`

func TestRewriteNamesTheAMSRelations(t *testing.T) {
	out, res, err := Rewrite([]byte(lieVIIIXAMS))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"precedesorcurly", "rightanglesw", "complement"} {
		if res.Names[name] != 1 {
			t.Errorf("replaced %s %d times, want 1", name, res.Names[name])
		}
	}
	for _, want := range []string{"/uni227C", "/uni231E", "/uni2201"} {
		if !strings.Contains(string(out), want) {
			t.Errorf("the encoding does not carry %s", want)
		}
	}
}

// The head of a vertical arrow of a commutative diagram goes where its shaft
// goes, and not to an up arrow.
//
// An arrow of one row is drawn out of the head alone and really is one, so the
// up arrow reads correctly and lands in the middle of the term the diagram
// drew it beside: page 375 came out as Z C ↑ (G). What the page needs is the
// diagram flag, and extract raises that on U+23D0.
func TestRewriteNamesTheArrowheadsOfADiagram(t *testing.T) {
	const cmex = `%PDF-1.5
71 0 obj
<<
/Type /Font
/Subtype /Type1
/BaseFont /DGMCMP+CMEX10
/Encoding 72 0 R
>>
endobj
72 0 obj
<<
/Type /Encoding
/Differences [1/arrowtp/arrowvertex/arrowbt/radicalbig]
>>
endobj
`
	out, res, err := Rewrite([]byte(cmex))
	if err != nil {
		t.Fatal(err)
	}
	if res.Names["arrowtp"] != 1 || res.Names["arrowbt"] != 1 {
		t.Fatalf("replaced arrowtp %d and arrowbt %d times, want 1 each",
			res.Names["arrowtp"], res.Names["arrowbt"])
	}
	if !strings.Contains(string(out), "/uni23D0/uniF8F5    /uni23D0/uni221A   ") {
		t.Errorf("the ends of the arrow did not land on the shaft:\n%s", out)
	}
	if len(res.Unknown) != 0 {
		t.Errorf("reported %v, and every name of this encoding is known", res.Unknown)
	}
}

// The sharp of the convolution of Lie chapter 9, which no name poppler reads is
// short enough to replace.
//
// TeX draws the three musical signs out of the mathematics italic. A natural
// becomes uni266E and a flat uni266D, both of which fit where the printing
// wrote the name; a sharp is five letters and uni266F is seven, so the code
// goes in the CMap instead. 31 of them over six pages arrived as an empty run,
// and page 350 alone says "Denote by s   the vector integral" twice in two
// lines.
func TestRewriteCodesTheSharp(t *testing.T) {
	const cmmi = `%PDF-1.5
81 0 obj
<<
/Type /Font
/Subtype /Type1
/BaseFont /DGLMOM+CMMI7
/Encoding 82 0 R
/ToUnicode 83 0 R
>>
endobj
82 0 obj
<<
/Type /Encoding
/Differences [21/natural 27/sharp]
>>
endobj
83 0 obj
<< /Length 150 >>
stream
/CIDInit /ProcSet findresource begin 12 dict begin begincmap
1 begincodespacerange
<00> <ff>
endcodespacerange
1 beginbfchar
<15> <266E>
endbfchar
endcmap
end end
endstream
endobj
`
	body := cmmi + "xref\n0 1\n0000000000 65535 f \ntrailer\n<</Size 84/Root 1 0 R>>\nstartxref\n" +
		fmt.Sprint(len(cmmi)) + "\n%%EOF\n"
	out, res, err := Rewrite([]byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if res.Coded["sharp"] != 1 {
		t.Fatalf("coded sharp %d times, want 1", res.Coded["sharp"])
	}
	// The natural is rewritten in place beside it, and the encoding is written
	// back for that. The sharp is left as the printing wrote it either way.
	if !strings.Contains(string(out), "/uni266E 27/sharp") {
		t.Error("the natural did not land on U+266E with the sharp left beside it")
	}
	if res.Unicode != 1 {
		t.Fatalf("patched %d CMaps, want 1", res.Unicode)
	}
	if !strings.Contains(string(out), "<1B> <266F>") {
		t.Errorf("the CMap was not given the sharp:\n%s", out)
	}
}

// A font that keeps its encoding to itself, in the shape the English Algebra
// writes it: the /Differences array is an inline dictionary inside the font
// object rather than an object the font points at.
//
// This is the extension font of Algebra chapter 8 with its array cut down.
const algVIIIInline = `%PDF-1.5
5276 0 obj
<</BaseFont/XAEWAV+CMEX10/Encoding<</BaseEncoding/WinAnsiEncoding/Differences[0/parenleftbig/parenrightbig 80/summationtext/producttext 98/hatwide 101/tildewide]/Type/Encoding>>/FirstChar 0/FontDescriptor 5277 0 R/LastChar 103/Subtype/Type1/Type/Font>>
endobj
`

// An encoding written inside its font is read as that font's.
//
// Getting this wrong is not one name missed, it is the wrong table picked for a
// whole font. baseFonts only ever learned the name of a font that pointed at an
// encoding of its own, so a font that kept its encoding inside itself had no
// name, and a nameless font falls through to the mathematics table. The two
// tables share the wide accents and nothing else of the extension font, so
// Algebra chapter 8 had its hats and tildes rewritten and every delimiter, sum
// and product of the volume passed over, and none of them were reported either,
// since a name is only reported against a font that has one.
func TestRewriteNamesAnEncodingWrittenInsideItsFont(t *testing.T) {
	out, res, err := Rewrite([]byte(algVIIIInline))
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range []string{"parenleftbig", "parenrightbig", "summationtext", "producttext"} {
		if res.Names[name] != 1 {
			t.Errorf("replaced %s %d times, want 1", name, res.Names[name])
		}
	}
	// The accents are the two the mathematics table would also have caught, and
	// they have to land where the extension font puts them and not where the
	// AMS fonts do.
	if !strings.Contains(string(out), "98/b       101/e") {
		t.Errorf("the accents were not read as the extension font's:\n%s", out)
	}
	if !strings.Contains(string(out), "[0/zero        /one           80/P ") {
		t.Errorf("the delimiters were not rewritten:\n%s", out)
	}
}
