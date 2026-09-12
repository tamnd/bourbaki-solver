package footnote

import (
	"regexp"
	"strings"
	"unicode"
)

// An Orphan is a note printed at the foot of a page with nothing in the page
// pointing at it.
type Orphan struct {
	Line  int    // the body line it sits on, counting from one
	Digit string // the number the printing gave it
	Text  string // the line, as the file writes it
}

var (
	// orphanRE is the shape a reading gives a note when it does not know it is
	// reading a note: the printed number, a space, and the note. The number is
	// not followed by a full stop or a bracket, which are what a numbered list
	// and a lettered item are written with.
	//
	// One digit, and that is a limit and not an oversight. The notes of a page
	// restart at 1 and rarely reach ten, and a numbered bibliography entry is
	// written the same way and does reach ten: allowing two digits takes this
	// from 68 findings to 135, 64 of them the reference list at the foot of the
	// historical notes, which is that page's actual layout and not a fault. The
	// nine notes lost to the limit are worth the sixty-four not raised.
	orphanRE = regexp.MustCompile(`^([1-9])[ \t]+(\S.*)$`)

	// supRE and htmlSupRE are the two ways a reading writes a superscript that
	// is not Markdown's own reference. Neither is a footnote to the assembler,
	// and both mean somebody wrote the mark down, so a page with one is a page
	// where the note is attached to something and this rule has nothing to say.
	supRE     = regexp.MustCompile(`\^[0-9a-zA-Z]+\^`)
	htmlSupRE = regexp.MustCompile(`(?i)<sup>`)
)

// tailLines is how far up from the foot of a page a note is looked for. The
// notes are set under a rule at the bottom and the reading keeps that order, so
// what is not in the last few lines is body prose that happens to open with a
// numeral -- the second term of a displayed series, a year, a page number
// carried into the text.
const tailLines = 4

// Orphans reads the foot of a page and reports a note with no mark anywhere
// above it.
//
// The other half of this is S10, which asks whether a note that is attached
// carries the printing's mark as well as Markdown's. Both halves have to be
// there for a footnote to be a footnote, and nothing asked about this one: a
// body that begins with a numeral assembles straight through into content/,
// gets translated, and prints in the middle of the running text as a paragraph
// that starts with a digit.
//
// The test is deliberately narrow, because the cost of a false positive here is
// somebody going to look at a page that is right. The line has to open with a
// single digit and a space, to hold more than thirty characters, to begin its
// text with a capital, a quotation mark or a formula, and to stand in a run
// that ends the page inside its last four lines. And the page has to carry no
// mark of any kind: no [^1] reference, no definition, no ^1^, no <sup>, and
// none of the symbols the volumes print. Where anything at all points at a
// note, the note is attached as far as this is concerned and is S10's business
// or nobody's.
//
// It reports 68 notes over 60 pages of 20 books, and every one of the 60 pages
// carries no marker of any form. It is not that most of them are attached and a
// few slipped: not one of them is.
//
// It is not a repair. Where the marker belongs in the sentence above is not
// recoverable from a page that never recorded it, so the honest fix is to read
// the printing and put the reference back, one note at a time.
func Orphans(body string) []Orphan {
	if marked(body) {
		return nil
	}
	lines := strings.Split(strings.TrimRight(body, "\n"), "\n")
	// A run that ends the page, blank lines apart. The notes are set under a
	// rule at the bottom and nothing is printed below them, so a line of prose
	// after the numeral means the numeral belongs to that prose: the exact
	// sequence "1 \longrightarrow T_Q \longrightarrow ... 1" on page 115 of Lie
	// VII opens on the trivial group and is followed by "is exact.", and the
	// three cases of exercise 16 on page 62 of FVR I are a list inside the
	// exercise with the brackets of "1)" lost.
	seen := 0
	var out []Orphan
	for i := len(lines) - 1; i >= 0 && seen < tailLines; i-- {
		line := strings.TrimRight(lines[i], " \t")
		if line == "" {
			continue
		}
		seen++
		m := orphanRE.FindStringSubmatch(line)
		if m == nil || len(line) <= 30 || !opensANote(m[2]) {
			break
		}
		out = append(out, Orphan{Line: i + 1, Digit: m[1], Text: line})
	}
	// Up the page, so the findings of one page read in the order they print.
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out
}

// marked says whether anything on the page points at a note. refRE covers the
// definitions as well as the references, since "[^1]:" holds "[^1]".
func marked(body string) bool {
	return refRE.MatchString(body) || supRE.MatchString(body) ||
		htmlSupRE.MatchString(body) || markRE.MatchString(body)
}

// opensANote says whether what follows the number reads as the beginning of a
// note rather than the rest of a sentence. A note is a sentence of its own and
// starts like one.
func opensANote(rest string) bool {
	r := []rune(rest)[0]
	switch r {
	case '$', '"', '\'', '«', '“', '‘', '*', '_':
		return true
	}
	return unicode.IsUpper(r)
}
