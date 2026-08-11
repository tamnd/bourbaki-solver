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
// The rewrite is byte for byte the same length as the file it came from. A name
// is replaced in place and padded with spaces, which a PDF name array reads as
// nothing, so every offset in the cross reference table still points where it
// did and no object has to be rewritten. Nothing outside a /Differences array
// of an /Type /Encoding object is touched. The prepared copy goes in work/,
// which is gitignored, and the volume itself is never written to.
package pdfglyph

import (
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
	res := Result{Names: map[string]int{}, Unknown: map[string]int{}}
	out := make([]byte, len(pdf))
	copy(out, pdf)
	fonts := baseFonts(pdf)
	for _, o := range objects(pdf) {
		dict := o.dict(pdf)
		if !encodingRe.MatchString(dict) {
			continue
		}
		s, e, ok := differences(pdf, o)
		if !ok {
			continue
		}
		table := cmexNames
		font := fonts[o.num]
		if !strings.Contains(strings.ToUpper(font), "CMEX") {
			table = mathNames
		}
		n := 0
		for _, m := range nameRe.FindAllSubmatchIndex(pdf[s:e], -1) {
			name := string(pdf[s+m[2] : s+m[3]])
			want, ok := table[name]
			if !ok {
				if texName.MatchString(name) && font != "" {
					res.Unknown[font+" "+name]++
				}
				continue
			}
			if len(want)+1 > m[1]-m[0] {
				return nil, res, fmt.Errorf("/%s does not fit where /%s was", want, name)
			}
			copy(out[s+m[0]:], "/"+want)
			for i := s + m[0] + len(want) + 1; i < s+m[1]; i++ {
				out[i] = ' '
			}
			res.Names[name]++
			n++
		}
		if n > 0 {
			res.Encodings++
		}
	}
	return out, res, nil
}

// nameRe is a PDF name as a font encoding writes one.
var nameRe = regexp.MustCompile(`/([A-Za-z][A-Za-z0-9]*)`)

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
	encodingRe = regexp.MustCompile(`/Type\s*/Encoding`)
	baseFontRe = regexp.MustCompile(`/BaseFont\s*/([A-Za-z0-9+\-]+)`)
	encRefRe   = regexp.MustCompile(`/Encoding\s+(\d+)\s+0\s+R`)
	diffRe     = regexp.MustCompile(`/Differences\s*\[`)
)

// baseFonts maps an encoding object to the font that uses it, since the same
// TeX name means different things in different fonts: hatwide is an accent to
// draw over a letter in the extension font and a glyph of its own in the AMS
// fonts. An encoding two fonts of different designs share is left out, so that
// it is skipped rather than read as one of them.
func baseFonts(pdf []byte) map[int]string {
	out := map[int]string{}
	for _, o := range objects(pdf) {
		dict := o.dict(pdf)
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
	if res.Total() == 0 {
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
	sum := sha256.Sum256([]byte(b.String()))
	return hex.EncodeToString(sum[:])[:6]
}
