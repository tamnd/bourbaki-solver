// Package pdfglyph puts back the mathematics a printing hides in its glyph
// names.
//
// The English Algebra embeds its fonts as subsets with no Unicode of any kind,
// so poppler falls back on the code the glyph sits at and prints the character
// that code stands for: a direct sum set in CMEX10 arrives as "L", a prime set
// in the symbol font arrives as "0", and extract/font.go reads the code back as
// the operator, because in a TeX font the code is the encoding.
//
// The French printings do the opposite. They name every glyph, in a
// /Differences array in plain sight in the file, and the names are the TeX ones:
// circleplusdisplay, summationtext, prime, negationslash. Poppler looks those up
// in the Adobe glyph list, does not find them, and prints nothing at all. The
// run arrives with its box, its font and its width, and with no text in it.
//
// Measured over Algèbre chapitre 8: 4831 runs come out empty, on 348 of its 487
// pages. 2927 of them are the prime of M', 1017 are large operators, 168 are
// large delimiters, 421 are wide accents, and the rest are the angle brackets,
// the ell and the negation stroke. None of it is flagged by anything, because
// nothing is missing as far as poppler is concerned, and the page reads as
// perfectly good French with the mathematics quietly thinned out.
//
// So the names are rewritten to names poppler does resolve, and they are
// rewritten to the ones that land on exactly the characters the English
// printing delivers, so that the tables in extract/font.go, which were checked
// glyph by glyph against the printed page, do the work for both printings and
// there is no second table to keep honest.
//
// No byte of the file is ever moved. A name is replaced where it stands and
// padded with spaces, which a PDF name array reads as nothing, so every offset
// in the cross reference still points where it did. What the rewrite cannot
// reach in place it appends: an encoding that lives inside a compressed object
// stream, and a /ToUnicode CMap that has to be told about the codes it skips,
// are written to the end of the file as an incremental update, which is a
// stretch of new objects and a new cross reference carrying /Prev. Nothing
// outside a /Differences array and the CMaps of the fonts that use one is
// touched. The prepared copy goes in work/, which is gitignored, and the volume
// itself is never written to.
//
// Two of the six volumes hide their encodings further than plain sight. Lie
// chapters 7 to 9 keeps all 35 /Differences arrays and all 39 font dictionaries
// inside four /ObjStm, so the string does not occur in the file at all, and it
// ships a /ToUnicode CMap per font that maps every code except the ones whose
// TeX names the glyph list has never heard of. A CMap outranks the name it is
// attached to, so a rewrite that stops at the name changes nothing.
package pdfglyph

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Result is what one rewrite did.
type Result struct {
	// Encodings is the number of /Differences arrays that were changed.
	Encodings int
	// Names counts the replacements, by the TeX name that was replaced.
	Names map[string]int
	// Unicode is how many /ToUnicode CMaps were given codes they were missing,
	// each one a CMap that would otherwise have kept the old reading.
	Unicode int
	// Coded counts the names that were left where they were and given a CMap
	// entry, by name.
	Coded map[string]int
	// Unknown counts the names that look like TeX and have no replacement,
	// so that a volume with a glyph this package has never seen says so
	// rather than dropping it quietly.
	Unknown map[string]int
}

// Total is how many names were replaced. It counts names in the encodings and
// not glyphs on the page: one name replaced once can be a thousand primes.
func (r Result) Total() int {
	n := 0
	for _, c := range r.Names {
		n += c
	}
	return n
}

// Sorted lists the replacements, most frequent first, for a report.
func (r Result) Sorted() []string {
	keys := make([]string, 0, len(r.Names))
	for k := range r.Names {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if r.Names[keys[i]] != r.Names[keys[j]] {
			return r.Names[keys[i]] > r.Names[keys[j]]
		}
		return keys[i] < keys[j]
	})
	return keys
}

// cmexNames is the mathematics extension font, and every replacement here is
// the name of the character poppler prints for that glyph in the English
// printing. The large operators are the pairs of extract/font.go's cmex table,
// the delimiters are its cmexDelim block read at the largest of the four sizes,
// and the accents are its cmexAccent table.
//
// The delimiters take the fourth size for all four, and the size is not a loss:
// a bracket is a bracket in LaTeX and the size of it is TeX's business. What the
// fourth size buys is that its codes are 0x30 to 0x3F, printable every one of
// them, where the first size sits at 0x00 and poppler drops it as a control
// character. The French printing sets 168 delimiters this way and the English
// loses the smaller two sizes of its own.
var cmexNames = map[string]string{
	// The large operators, text size then display size.
	"squareuniontext": "F", "squareuniondisplay": "G",
	"circledottext": "J", "circledotdisplay": "K",
	"circleplustext": "L", "circleplusdisplay": "M",
	"circlemultiplytext": "N", "circlemultiplydisplay": "O",
	"summationtext": "P", "summationdisplay": "X",
	"producttext": "Q", "productdisplay": "Y",
	"integraltext": "R", "integraldisplay": "Z",
	"uniontext": "S", "uniondisplay": "bracketleft",
	"intersectiontext": "T", "intersectiondisplay": "backslash",
	"unionmultitext": "U", "unionmultidisplay": "bracketright",
	"logicalandtext": "V", "logicalanddisplay": "asciicircum",
	"logicalortext": "W", "logicalordisplay": "underscore",
	"coproducttext": "grave", "coproductdisplay": "a",

	// The wide accents, in the three widths CMEX carries of each.
	"hatwide": "b", "hatwider": "c", "hatwidest": "d",
	"tildewide": "e", "tildewider": "f", "tildewidest": "g",

	// The delimiters, all four sizes onto the fourth.
	"parenleftbig": "zero", "parenleftBig": "zero", "parenleftbigg": "zero", "parenleftBigg": "zero",
	"parenrightbig": "one", "parenrightBig": "one", "parenrightbigg": "one", "parenrightBigg": "one",
	"bracketleftbig": "two", "bracketleftBig": "two", "bracketleftbigg": "two", "bracketleftBigg": "two",
	"bracketrightbig": "three", "bracketrightBig": "three", "bracketrightbigg": "three", "bracketrightBigg": "three",
	"floorleftbig": "four", "floorleftBig": "four", "floorleftbigg": "four", "floorleftBigg": "four",
	"floorrightbig": "five", "floorrightBig": "five", "floorrightbigg": "five", "floorrightBigg": "five",
	"ceilingleftbig": "six", "ceilingleftBig": "six", "ceilingleftbigg": "six", "ceilingleftBigg": "six",
	"ceilingrightbig": "seven", "ceilingrightBig": "seven", "ceilingrightbigg": "seven", "ceilingrightBigg": "seven",
	"braceleftbig": "eight", "braceleftBig": "eight", "braceleftbigg": "eight", "braceleftBigg": "eight",
	"bracerightbig": "nine", "bracerightBig": "nine", "bracerightbigg": "nine", "bracerightBigg": "nine",
	"angbracketleftbig": "colon", "angbracketleftBig": "colon", "angbracketleftbigg": "colon", "angbracketleftBigg": "colon",
	"angbracketrightbig": "semicolon", "angbracketrightBig": "semicolon", "angbracketrightbigg": "semicolon", "angbracketrightBigg": "semicolon",
	"vextendsinglebig": "less", "vextendsingleBig": "less", "vextendsinglebigg": "less", "vextendsingleBigg": "less",
	"vextenddoublebig": "equal", "vextenddoubleBig": "equal", "vextenddoublebigg": "equal", "vextenddoubleBigg": "equal",
	"slashbig": "greater", "slashBig": "greater", "slashbigg": "greater", "slashBigg": "greater",
	"backslashbig": "question", "backslashBig": "question", "backslashbigg": "question", "backslashBigg": "question",

	// The pieces of a delimiter drawn taller than any single glyph. They go to
	// the private use area poppler puts them in when it does know them, which
	// is what extract.PUA reads to flag the page: a top, a stack of extensions
	// and a bottom arrive as runs on separate lines and cannot be put back
	// together from any one of them.
	"parenlefttp": "uniF8EB", "parenleftex": "uniF8EC", "parenleftbt": "uniF8ED",
	"parenrighttp": "uniF8F6", "parenrightex": "uniF8F7", "parenrightbt": "uniF8F8",
	"bracketlefttp": "uniF8EE", "bracketleftex": "uniF8EF", "bracketleftbt": "uniF8F0",
	"bracketrighttp": "uniF8F9", "bracketrightex": "uniF8FA", "bracketrightbt": "uniF8FB",
	"bracelefttp": "uniF8F1", "braceleftmid": "uniF8F2", "braceleftbt": "uniF8F3",
	"bracerighttp": "uniF8FC", "bracerightmid": "uniF8FD", "bracerightbt": "uniF8FE",
	"braceex": "uniF8F4", "arrowvertex": "uniF8F5", "arrowvertexdbl": "uniF8FF",

	// The two ends of a vertical arrow, which go where its shaft goes. Lie
	// chapters 7 to 9 draws 18 of them, six on the diagram of page 119 that
	// puts Aut(g, h) over Aut_0(g, h) and twelve more on the diagrams of pages
	// 375 and 376, and every one of them was an empty run.
	//
	// U+23D0 and not the up and down arrows, though an arrow of one row is
	// drawn out of the head alone and really is one. An arrow of a diagram
	// stands between two rows and is read on the row it was drawn beside, so
	// the up arrow of page 375 lands in the middle of the term above it and the
	// page reads Z C ↑ (G). What the reader needs off that page is not the
	// arrowhead but the diagram flag, and extract raises it on this character:
	// see arrowExtension, and the shaft this now agrees with.
	//
	// The radical of the extension font is the one TeX draws over a term too
	// wide for the sign of the mathematics italic, and poppler was falling back
	// on the code and printing an opening parenthesis for it. Lie chapter 9
	// writes the orthonormal basis of the anti-invariant elements as J(lambda
	// rho) over the root of w(G) and shipped it as "/(w(G)", and page 331 opens
	// the section on the norms of the coefficients with the same parenthesis in
	// front of the root of d(u). It is a surd here rather than a sqrt because
	// the sign and what it covers are drawn apart and arrive apart: the bar is
	// its own run and is already read as an overline.
	"arrowtp": "uni23D0", "arrowbt": "uni23D0", "radicalbig": "uni221A",
}

// mathNames is every other mathematics font: the symbol font, the AMS fonts and
// the math italic. These are read by extract's cmsy and unicodeMath tables
// rather than by position, so the replacement is the Unicode of the symbol
// itself where the corpus has a rendering for it, and the character the English
// printing prints where that is what the tables key on.
var mathNames = map[string]string{
	// The prime of M'. In the English printing this glyph is code 0x30 of the
	// symbol font and arrives as "0", which is why the text layer of that
	// volume calls a submodule M' by the name Mi0, and extract's cmsy table
	// reads "0" back as a prime. 2927 of them in Algèbre chapitre 8, every one
	// of them silently gone before this.
	"prime": "zero",
	// The stroke TeX draws over a relation to negate it. Code 0x36 of the
	// symbol font, so "6", which is what pdftotext prints for a 6= and what
	// extract's symbolPairs read as one symbol.
	"negationslash": "six",
	// The angle brackets, codes 0x68 and 0x69 of the symbol font.
	"angbracketleft": "h", "angbracketright": "i",
	// The double bar TeX draws a norm with, code 0x6B of the symbol font, so
	// "k", which is what extract's cmsy table reads back as \|. Page 360 of
	// Lie 7 to 9 defines the Hilbert norm on L^2 of the dual of G and writes
	// it with these; every one of them arrived as an empty run, so the page
	// said A_u^2_2 where it prints the norm of A_u squared, and the norm the
	// sentence names came through as nothing at all.
	"bardbl": "k",
	// These have no place in the symbol font's printable range, so they go to
	// their own Unicode and extract's unicodeMath table renders them.
	// The AMS symbol fonts carry the wide accents too, and this printing draws
	// its "omit this factor" hat out of MSBM10 rather than out of the extension
	// font: 24 of them, 22 in the appendix on determinants over a
	// noncommutative field where the whole argument turns on p(v1), . . . ,
	// widehat{p(vi)}, . . . , p(vn). They go to the combining accents rather
	// than to the codes CMEX reads them at, since a letter of the extension
	// font in another font is a letter and these have to be read as an accent
	// whatever they were set in.
	"hatwide": "uni0302", "hatwider": "uni0302", "hatwidest": "uni0302",
	"tildewide": "uni0303", "tildewider": "uni0303", "tildewidest": "uni0303",

	"lscript":        "uni2113", // the script ell
	"natural":        "uni266E",
	"squaremultiply": "uni22A0",
	"complement":     "uni2201",
	"Rfractur":       "uni211C",
	"Ifractur":       "uni2111",

	// The four below came out of the dropped glyph flag rather than out of the
	// report this package writes, which reads only the names texName matches
	// and matches none of them. They are the relations of the symbol font and
	// the AMS fonts whose codes are control characters, so poppler has nothing
	// to fall back on either: it prints nothing and prints nothing quietly.
	//
	// Lie chapters 7 to 9 defines the canonical pairing on the dual of a
	// compact group and writes it with the double angle brackets, so page 314
	// printed "a bilinear form   ,   on" and the display under it "= 2 pi i
	// a, b ." with the brackets gone. Page 370 states the domination relation
	// between two positive functions on G and lost the sign 14 times in the
	// paragraph that introduces it. Page 343 writes the Weyl integration
	// formula and lost the corner off omega_G.
	"lessmuch":        "uni226A",
	"greatermuch":     "uni226B",
	"precedesorcurly": "uni227C",
	"rightanglesw":    "uni231E",
}

// codedNames are names this package does not replace and gives a CMap entry
// instead.
//
// A replacement has to fit in the bytes the old name took, since the rewrite is
// made in place, and there is no name three letters long that poppler reads as
// a variant Greek. pi1 is the one that matters: the printing of Lie chapters 7
// to 9 names code 0x17 of its mathematics italic pi1, which is what dvips calls
// varpi, and the Adobe list calls that character omega1 and has never heard of
// pi1. The /ToUnicode CMap of the font lists every other code of the encoding
// and skips this one, so poppler had nothing to read it by and handed back an
// empty run: Corollary 1 of § 7, no. 3 names the family of fundamental weights
// and the page printed "( )_{alpha in B}" with the letter gone out of it, and
// the same corollary lost the weight itself twice more in the proof below.
//
// The CMap is appended rather than rewritten, so nothing here has to fit
// anywhere, and the name in the encoding is left as the printing wrote it.
// The first three are the variant Greek of the mathematics italic that the
// Adobe list has no name for. Its theta1, phi1 and sigma1 are names of the
// symbol font and poppler reads all three, which is why they are not here.
//
// The sharp is here for the same arithmetic. TeX draws the three musical signs
// out of the mathematics italic and the Adobe list knows none of the three by
// those names, so a natural becomes uni266E and a flat uni266D, both of which
// fit. A sharp is five letters and uni266F is seven, and there is nothing
// shorter that reads as one. The convolution of Lie chapter 9 is written s
// sharp all through the section on the Weyl integration formula, at the size
// the italic sets a superscript, and 31 of them over six pages arrived as an
// empty run: page 350 alone says "Denote by s   the vector integral" twice in
// two lines.
var codedNames = map[string]rune{
	"pi1":      'ϖ', // varpi
	"rho1":     'ϱ', // varrho
	"epsilon1": 'ϵ', // the lunate epsilon
	"sharp":    '♯',
}

// texName is a glyph name that only a TeX font uses, so that a volume carrying
// one this package has no replacement for is reported rather than left to be
// found by a reader.
//
// The endings are the ones TeX builds its families of sizes from, and they are
// anchored to a stem rather than taken on their own, because a bare ex at the
// end of a name is also how the Adobe list ends circumflex, and a first pass at
// this reported eleven accented capitals as lost mathematics.
var texName = regexp.MustCompile(`^(?:[a-z]+(?:text|display|wide|wider|widest|big|Big|bigg|Bigg|tp|bt|mid)|[a-z]+(?:left|right|vert)ex|braceex|arrowvertexdbl)$`)

// Rewrite returns the volume with its glyph names replaced, and what it did.
//
// The bytes come back the same length. A replacement longer than the name it
// replaces is refused rather than made room for, since making room means moving
// every byte after it and rewriting the cross reference table, and every name
// this package replaces is shorter than the TeX name it stands for.
func Rewrite(pdf []byte) ([]byte, Result, error) {
	res := Result{Names: map[string]int{}, Coded: map[string]int{}, Unknown: map[string]int{}}
	out := make([]byte, len(pdf))
	copy(out, pdf)
	srcs, err := sources(out)
	if err != nil {
		return nil, res, err
	}
	fonts := baseFonts(srcs)
	rewritten := map[int]map[int]rune{}
	for _, s := range srcs {
		if err := rewriteNames(s, fonts, rewritten, &res); err != nil {
			return nil, res, err
		}
	}
	patches, err := patchToUnicode(newUnicodeMaps(srcs[0]), cmapNeeds(srcs, rewritten))
	if err != nil {
		return nil, res, err
	}
	res.Unicode = len(patches)
	out, err = appendUpdate(out, append(updates(srcs), patches...))
	if err != nil {
		return nil, res, err
	}
	return out, res, nil
}

// rewriteNames replaces the TeX glyph names in one source's /Differences
// arrays, and records which encoding objects it touched.
func rewriteNames(src *source, fonts map[int]string, rewritten map[int]map[int]rune, res *Result) error {
	buf := src.buf
	for _, o := range src.objs {
		dict := o.dict(buf)
		if !encodingRe.MatchString(dict) {
			continue
		}
		s, e, ok := differences(buf, o)
		if !ok {
			continue
		}
		table := cmexNames
		font := fonts[o.num]
		if !strings.Contains(strings.ToUpper(font), "CMEX") {
			table = mathNames
		}
		codes := map[int]rune{}
		named := false
		code := 0
		for _, m := range codeNameRe.FindAllSubmatchIndex(buf[s:e], -1) {
			if m[2] >= 0 {
				at, err := strconv.Atoi(string(buf[s+m[2] : s+m[3]]))
				if err != nil {
					return err
				}
				code = at
				continue
			}
			at := code
			code++
			name := string(buf[s+m[4] : s+m[5]])
			want, ok := table[name]
			if !ok {
				if r, ok := codedNames[name]; ok {
					codes[at] = r
					res.Coded[name]++
					continue
				}
				if texName.MatchString(name) && font != "" {
					res.Unknown[font+" "+name]++
				}
				continue
			}
			named = true
			if len(want) > m[5]-m[4] {
				return fmt.Errorf("/%s does not fit where /%s was", want, name)
			}
			copy(buf[s+m[4]-1:], "/"+want)
			for i := s + m[4] + len(want); i < s+m[5]; i++ {
				buf[i] = ' '
			}
			r, ok := glyphRune(want)
			if !ok {
				return fmt.Errorf("/%s is not a name this knows the character of", want)
			}
			res.Names[name]++
			codes[at] = r
		}
		if len(codes) > 0 {
			rewritten[o.num] = codes
		}
		// An encoding only a coded name was found in still reads as it did, and
		// the object it lives in is written back untouched. What that name
		// needed was the CMap entry above it, and the encoding is here to say
		// which code to put it at.
		if named {
			res.Encodings++
			src.edited(o.num)
		}
	}
	return nil
}

// cmapNeeds is what each /ToUnicode CMap in a file is missing, gathered from
// the fonts that point at it: the codes whose glyph name was just rewritten,
// and the character that name now stands for.
//
// A CMap is read before the glyph name and wins over it, so a font that carries
// one and a rewritten encoding would come back exactly as it went in unless the
// CMap is dealt with. Several fonts of a family share one CMap, so what they
// need is merged rather than fought over.
func cmapNeeds(srcs []*source, rewritten map[int]map[int]rune) map[int]map[int]rune {
	out := map[int]map[int]rune{}
	for _, src := range srcs {
		for _, o := range src.objs {
			dict := o.dict(src.buf)
			if !fontRe.MatchString(dict) {
				continue
			}
			enc := encRefRe.FindStringSubmatch(dict)
			cmap := toUnicodeRe.FindStringSubmatch(dict)
			if enc == nil || cmap == nil {
				continue
			}
			e, err := strconv.Atoi(enc[1])
			if err != nil {
				continue
			}
			want := rewritten[e]
			if len(want) == 0 {
				continue
			}
			num, err := strconv.Atoi(cmap[1])
			if err != nil {
				continue
			}
			if out[num] == nil {
				out[num] = map[int]rune{}
			}
			for code, r := range want {
				out[num][code] = r
			}
		}
	}
	return out
}

// patchToUnicode gives every CMap the codes it is missing, as objects to append
// to the file, and leaves alone every code it already has.
//
// The missing codes are not a corner case. Lie chapters 7 to 9 ships a CMap for
// every font, and each one lists every code except the ones whose TeX name is
// not in the Adobe glyph list: the CMap of the symbol font at 7 point maps 21
// codes and skips the prime, the negation stroke and the two angle brackets,
// which are precisely the four this package has replacements for. Poppler reads
// the CMap, finds nothing at those codes, and prints nothing, 4592 times over
// the volume.
//
// The first version of this threw the whole CMap away instead, which is one
// line of code and wrong. A CMap is a font wide table, and a font whose encoding
// this package touched at four codes has a hundred others the CMap is the only
// good reading of: throwing it away cost the French Algèbre chapitre 8 310 mus,
// which its CMap called U+03BC and whose glyph name is the micro sign, and 195
// pieces of tall parenthesis, which its CMap called U+239B and whose names are
// the private use codes a font means by them. Adding to the table takes the
// codes that were missing and keeps the ones that were not.
//
// A patched CMap is written out uncompressed. It is a few kilobytes in a copy
// that lives in work/ and is thrown away whenever the tables change, and it is
// worth being able to read the thing when a reading goes wrong.
func patchToUnicode(maps *unicodeMaps, needs map[int]map[int]rune) ([]update, error) {
	nums := make([]int, 0, len(needs))
	for num := range needs {
		nums = append(nums, num)
	}
	sort.Ints(nums)

	var out []update
	for _, num := range nums {
		body, ok := maps.body(num)
		if !ok {
			continue
		}
		have := cmapCodes(body)
		var codes []int
		for code := range needs[num] {
			if !have[code] {
				codes = append(codes, code)
			}
		}
		if len(codes) == 0 {
			continue
		}
		sort.Ints(codes)
		patched, err := addBFChars(body, codes, needs[num], codeWidth(body))
		if err != nil {
			return nil, fmt.Errorf("object %d: %w", num, err)
		}
		out = append(out, update{num: num, body: streamObject(patched)})
	}
	return out, nil
}

// addBFChars puts a block of single character mappings into a CMap, in front of
// the endcmap that closes it, which is where a mapping is still read and where
// nothing already in the file has to move to make room. The block is broken at
// a hundred entries, which is the most the specification allows in one.
func addBFChars(body []byte, codes []int, want map[int]rune, width int) ([]byte, error) {
	i := bytes.LastIndex(body, []byte("endcmap"))
	if i < 0 {
		return nil, fmt.Errorf("the CMap does not end with endcmap")
	}
	var b bytes.Buffer
	b.Write(body[:i])
	for len(codes) > 0 {
		n := min(len(codes), 100)
		fmt.Fprintf(&b, "%d beginbfchar\n", n)
		for _, code := range codes[:n] {
			fmt.Fprintf(&b, "<%0*X> <%04X>\n", width, code, want[code])
		}
		b.WriteString("endbfchar\n")
		codes = codes[n:]
	}
	b.Write(body[i:])
	return b.Bytes(), nil
}

// streamObject is the body of an indirect object holding a stream, as an
// incremental update writes one.
func streamObject(body []byte) []byte {
	var b bytes.Buffer
	fmt.Fprintf(&b, "<</Length %d>>\nstream\n", len(body))
	b.Write(body)
	b.WriteString("\nendstream")
	return b.Bytes()
}

// codeWidth is how many hexadecimal digits a CMap writes a code in, taken from
// the entries already in it, since a code written to another width than its
// codespace says is not read at all. Two is what a simple font's CMap uses and
// what is assumed for a CMap with nothing in it to go on.
func codeWidth(body []byte) int {
	for _, b := range bfcharRe.FindAllSubmatch(body, -1) {
		if h := hexRe.FindSubmatch(b[1]); h != nil {
			return len(h[1])
		}
	}
	for _, b := range bfrangeRe.FindAllSubmatch(body, -1) {
		if h := hexRe.FindSubmatch(b[1]); h != nil {
			return len(h[1])
		}
	}
	return 2
}

// glyphRune is the character poppler prints for one of this package's
// replacement names, which is what a CMap entry for that code has to say if the
// two are to agree. Every value of the tables above is either a character of
// its own, a name of the Adobe list this spells out, or a uniXXXX.
func glyphRune(name string) (rune, bool) {
	if r, ok := glyphRunes[name]; ok {
		return r, true
	}
	if rest, ok := strings.CutPrefix(name, "uni"); ok && len(rest) == 4 {
		n, err := strconv.ParseInt(rest, 16, 32)
		if err != nil {
			return 0, false
		}
		return rune(n), true
	}
	r := []rune(name)
	if len(r) == 1 && r[0] < 0x80 {
		return r[0], true
	}
	return 0, false
}

// glyphRunes is the Adobe names this package replaces a TeX name with, and it
// is short because the replacements were chosen to be the characters the
// English printing delivers and those are nearly all printable on their own.
var glyphRunes = map[string]rune{
	"zero": '0', "one": '1', "two": '2', "three": '3', "four": '4',
	"five": '5', "six": '6', "seven": '7', "eight": '8', "nine": '9',
	"bracketleft": '[', "backslash": '\\', "bracketright": ']',
	"asciicircum": '^', "underscore": '_', "grave": '`',
	"colon": ':', "semicolon": ';', "less": '<', "equal": '=',
	"greater": '>', "question": '?',
}

// unicodeMaps reads the /ToUnicode CMaps of a file. A CMap is a stream, and a
// stream is always a top level object however much of the rest of the file is
// packed into object streams, so only the file itself is indexed. Several fonts
// of one family point at one CMap, so each is inflated once.
type unicodeMaps struct {
	file *source
	at   map[int]object
	seen map[int][]byte
}

func newUnicodeMaps(file *source) *unicodeMaps {
	m := &unicodeMaps{file: file, at: map[int]object{}, seen: map[int][]byte{}}
	for _, o := range file.objs {
		m.at[o.num] = o
	}
	return m
}

// body is a CMap decoded, or nothing if the file does not have it or this
// cannot read it, in which case the font keeps whatever it had and the glyph
// name rewrite underneath is all it gets.
func (m *unicodeMaps) body(num int) ([]byte, bool) {
	if b, ok := m.seen[num]; ok {
		return b, b != nil
	}
	b := m.read(num)
	m.seen[num] = b
	return b, b != nil
}

func (m *unicodeMaps) read(num int) []byte {
	o, ok := m.at[num]
	if !ok {
		return nil
	}
	start, end, ok := streamSpan(m.file.buf, o)
	if !ok {
		return nil
	}
	body := m.file.buf[start:end]
	if flateRe.MatchString(o.dict(m.file.buf)) {
		out, err := inflate(body)
		if err != nil {
			return nil
		}
		return out
	}
	return append([]byte{}, body...)
}

// cmapCodes is every code a CMap gives a Unicode for. A destination of nothing
// but zeros is how a tool writes a code it had no Unicode for, and poppler
// prints nothing for it, so it does not count as covered.
func cmapCodes(body []byte) map[int]bool {
	out := map[int]bool{}
	for _, b := range bfcharRe.FindAllSubmatch(body, -1) {
		h := hexRe.FindAllSubmatch(b[1], -1)
		for i := 0; i+1 < len(h); i += 2 {
			code, err := strconv.ParseInt(string(h[i][1]), 16, 32)
			if err != nil || zeros(h[i+1][1]) {
				continue
			}
			out[int(code)] = true
		}
	}
	for _, b := range bfrangeRe.FindAllSubmatch(body, -1) {
		for _, r := range bfrangeEntryRe.FindAllSubmatch(b[1], -1) {
			lo, err := strconv.ParseInt(string(r[1]), 16, 32)
			if err != nil {
				continue
			}
			hi, err := strconv.ParseInt(string(r[2]), 16, 32)
			if err != nil || hi < lo || hi-lo > 0xFFFF {
				continue
			}
			for c := lo; c <= hi; c++ {
				out[int(c)] = true
			}
		}
	}
	return out
}

func zeros(hex []byte) bool {
	for _, c := range hex {
		if c != '0' {
			return false
		}
	}
	return true
}

var (
	bfcharRe       = regexp.MustCompile(`(?s)beginbfchar(.*?)endbfchar`)
	bfrangeRe      = regexp.MustCompile(`(?s)beginbfrange(.*?)endbfrange`)
	bfrangeEntryRe = regexp.MustCompile(`(?s)<([0-9A-Fa-f]+)>\s*<([0-9A-Fa-f]+)>\s*(?:<[0-9A-Fa-f]*>|\[.*?\])`)
	hexRe          = regexp.MustCompile(`<([0-9A-Fa-f]+)>`)
)

// codeNameRe is one entry of a /Differences array: a number, which says what
// code the names after it start at, or a name, which takes the next code.
var codeNameRe = regexp.MustCompile(`(\d+)|/([A-Za-z][A-Za-z0-9]*)`)

// object is one indirect object, as far as this package needs one: a number and
// where its body starts and stops.
type object struct {
	num        int
	start, end int
}

// dict is the object's dictionary, which is everything before its stream if it
// has one. A font encoding never has a stream; cutting at one keeps this from
// reading a compressed stream as text.
func (o object) dict(pdf []byte) string {
	body := string(pdf[o.start:o.end])
	if i := strings.Index(body, "stream"); i >= 0 {
		body = body[:i]
	}
	return body
}

var objRe = regexp.MustCompile(`(\d+)\s+0\s+obj`)

// objects indexes the file. An object runs to the start of the next one, which
// is not what the specification says and is what every writer does; the only
// thing read out of the span is the dictionary at the front of it.
func objects(pdf []byte) []object {
	m := objRe.FindAllSubmatchIndex(pdf, -1)
	out := make([]object, 0, len(m))
	for i, x := range m {
		num, err := strconv.Atoi(string(pdf[x[2]:x[3]]))
		if err != nil {
			continue
		}
		end := len(pdf)
		if i+1 < len(m) {
			end = m[i+1][0]
		}
		out = append(out, object{num: num, start: x[1], end: end})
	}
	return out
}

var (
	encodingRe  = regexp.MustCompile(`/Type\s*/Encoding`)
	fontRe      = regexp.MustCompile(`/Type\s*/Font[^a-zA-Z]`)
	toUnicodeRe = regexp.MustCompile(`/ToUnicode\s+(\d+)\s+\d+\s+R`)
	baseFontRe  = regexp.MustCompile(`/BaseFont\s*/([A-Za-z0-9+\-]+)`)
	encRefRe    = regexp.MustCompile(`/Encoding\s+(\d+)\s+0\s+R`)
	diffRe      = regexp.MustCompile(`/Differences\s*\[`)
)

// baseFonts maps an encoding object to the font that uses it, since the same
// TeX name means different things in different fonts: hatwide is an accent to
// draw over a letter in the extension font and a glyph of its own in the AMS
// fonts. An encoding two fonts of different designs share is left out, so that
// it is skipped rather than read as one of them.
func baseFonts(srcs []*source) map[int]string {
	out := map[int]string{}
	for _, src := range srcs {
		for _, o := range src.objs {
			dict := o.dict(src.buf)
			bf := baseFontRe.FindStringSubmatch(dict)
			ref := encRefRe.FindStringSubmatch(dict)
			if bf == nil || ref == nil {
				continue
			}
			num, err := strconv.Atoi(ref[1])
			if err != nil {
				continue
			}
			name := base(bf[1])
			if old, ok := out[num]; ok && old != name {
				out[num] = ""
				continue
			}
			out[num] = name
		}
	}
	return out
}

// base strips the subset tag a typesetter's toolchain puts in front of a font
// name, so that XAEWAV+CMEX10 reads as CMEX10.
func base(s string) string {
	if _, rest, ok := strings.Cut(s, "+"); ok {
		return rest
	}
	return s
}

// differences is the span inside the brackets of an object's /Differences array.
func differences(pdf []byte, o object) (int, int, bool) {
	dict := o.dict(pdf)
	m := diffRe.FindStringIndex(dict)
	if m == nil {
		return 0, 0, false
	}
	start := o.start + m[1]
	for i := start; i < o.start+len(dict); i++ {
		if pdf[i] == ']' {
			return start, i, true
		}
	}
	return 0, 0, false
}

// Prepare returns the path of the copy of a volume to read from, building it if
// it is not there already, and the original path when the volume needs nothing
// done to it.
//
// The copy is named after the digest of the volume it came from, so a volume
// replaced on disk gets a new one rather than an old one that no longer matches
// it, and work/ is gitignored, so none of this is ever committed.
func Prepare(root, id, path, sum string) (string, Result, error) {
	pdf, err := os.ReadFile(path)
	if err != nil {
		return "", Result{}, err
	}
	out, res, err := Rewrite(pdf)
	if err != nil {
		return "", res, err
	}
	if res.Total() == 0 && res.Unicode == 0 {
		return path, res, nil
	}
	prepared := PreparedPath(root, id, sum)
	if _, err := os.Stat(prepared); err == nil {
		return prepared, res, nil
	}
	if err := os.MkdirAll(filepath.Dir(prepared), 0o755); err != nil {
		return "", res, err
	}
	tmp := prepared + ".tmp"
	if err := os.WriteFile(tmp, out, 0o644); err != nil {
		return "", res, err
	}
	if err := os.Rename(tmp, prepared); err != nil {
		return "", res, err
	}
	return prepared, res, nil
}

// PreparedPath is where the prepared copy of a volume lives.
//
// The name carries the digest of the volume and the digest of the tables above,
// so that a copy is rebuilt both when the volume is replaced and when a glyph
// is added to the tables. Naming it after the volume alone left a stale copy
// behind every time a name was added, and a stale copy is the worst kind of
// wrong: it reads fine and it is missing the character that was just fixed.
func PreparedPath(root, id, sum string) string {
	if len(sum) > 12 {
		sum = sum[:12]
	}
	return filepath.Join(root, "work", "prepared", id+"-"+sum+"-"+tableSum()+".pdf")
}

// tableSum is a digest of the replacements, short enough to read in a file name
// and long enough that two sets of tables do not share one.
func tableSum() string {
	var b strings.Builder
	// The rewrite itself is versioned along with the tables, since a prepared
	// copy is only as good as what made it and a change in what this package
	// does to a file has to invalidate the copies as surely as a new name does.
	b.WriteString("v3 tounicode;")
	for _, t := range []map[string]string{cmexNames, mathNames} {
		keys := make([]string, 0, len(t))
		for k := range t {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			b.WriteString(k)
			b.WriteString("=")
			b.WriteString(t[k])
			b.WriteString(";")
		}
	}
	coded := make([]string, 0, len(codedNames))
	for k := range codedNames {
		coded = append(coded, k)
	}
	sort.Strings(coded)
	for _, k := range coded {
		b.WriteString(k)
		b.WriteString("=")
		b.WriteRune(codedNames[k])
		b.WriteString(";")
	}
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])[:6]
}
