package ocr

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// A repaired page is read from a picture, and a picture does not carry a font
// name. The PDF does.
//
// The repair pass exists for what the text layer could not carry: a commutative
// diagram whose arrows poppler hands back as empty elements, a matrix whose
// rows gathered onto one line, a delimiter that lost its partner. On all of
// that the model looking at the page is the better witness. On typeface it is
// the worse one, and three rounds of prompt writing over the twenty three
// flagged pages of Lie 7 to 9 did not close the gap: each round fixed the
// defect the round before had found and produced a new one. The model wrote
// \lambda_g for \lambda_{\mathfrak{g}}, then \mathrm{SU} for a group the book
// sets in bold, then bolded twelve italic capitals so that the module
// Z(\lambda-\rho) became the ring of integers, then dropped the bold off the
// symmetric algebra S(\mathfrak{h}) on two pages.
//
// So the run reports it instead. Every letter whose face changed between the
// reading being replaced and the one replacing it is listed, per page, and a
// human decides. It is not a rewrite: the extractor is wrong about a face often
// enough that automatically restoring the old one would trade a known error
// rate for an unknown one.

// FaceChange is one letter that is written in a different face than it was.
type FaceChange struct {
	Page int `json:"page"`
	// Face is the LaTeX command without its backslash: mathbf, mathfrak,
	// mathscr. Plain is the letter in no face at all.
	Face   string `json:"face"`
	Letter string `json:"letter"`
	// Was and Now are how many times the letter appears in that face in the
	// reading being replaced and in the one replacing it.
	Was int `json:"was"`
	Now int `json:"now"`
}

func (c FaceChange) String() string {
	verb := "gained"
	if c.Now < c.Was {
		verb = "lost"
	}
	return fmt.Sprintf("page %d: %s %s %d times, was %d, now %d",
		c.Page, verb, faceName(c.Face, c.Letter), abs(c.Now-c.Was), c.Was, c.Now)
}

func abs(n int) int {
	if n < 0 {
		return -n
	}
	return n
}

func faceName(face, letter string) string {
	if face == plainFace {
		return "a plain " + letter
	}
	return `\` + face + "{" + letter + "}"
}

// plainFace is the bucket for a letter standing in no face command. It is
// counted because most of what goes wrong here is a letter moving between a
// face and no face, and a report that only counted the faces would show
// \mathbf{S} falling from seventeen to zero without saying where it went.
const plainFace = "plain"

// faces reads the faces of a page.
//
// A face command takes a group or a single letter, and both spellings are the
// same page: \mathfrak{s}\mathfrak{u} and \mathfrak{su} are two letters in
// fraktur either way. Counting groups rather than letters was the first way
// this was written and it reported three losses that were not there, because
// the model had joined a pair the extractor had split.
//
// Only single letters are counted in the plain bucket, and only inside math.
// An operator name is a run of letters and is not a variable that lost a face,
// and prose is not mathematics.
func faces(text string) map[[2]string]int {
	out := map[[2]string]int{}
	for _, span := range mathSpans(text) {
		rest := span
		for {
			loc := faceCommand.FindStringSubmatchIndex(rest)
			if loc == nil {
				break
			}
			plainLetters(out, rest[:loc[0]])
			name := rest[loc[2]:loc[3]]
			// One of the two alternatives matched and the other is -1: a group
			// for \mathbf{Sp}, a bare letter for \mathbf C.
			letters := ""
			if loc[4] >= 0 {
				letters = rest[loc[4]:loc[5]]
			} else if loc[6] >= 0 {
				letters = rest[loc[6]:loc[7]]
			}
			for _, r := range letters {
				if isLetter(r) {
					out[[2]string{name, string(r)}]++
				}
			}
			rest = rest[loc[1]:]
		}
		plainLetters(out, rest)
	}
	return out
}

// faceCommand matches \mathbf{Sp}, \mathbf S and the fraktur, script and
// calligraphic forms. The braced alternative comes first so that \mathbf{S} is
// read as a group rather than as a bare brace.
//
// mathcal is here so that a letter moving between the two script faces is
// reported as the move it is. Without it \mathscr{T} becoming \mathcal{T} read
// as thirty losses of \mathscr{T} with nothing gaining them, which is a report
// that says a face went and cannot say where.
var faceCommand = regexp.MustCompile(`\\(mathbf|mathfrak|mathscr|mathcal)\s*(?:\{([^{}]*)\}|([A-Za-z]))`)

// plainLetters counts the single letters of a stretch of mathematics that carry
// no face command. A letter is single when what sits either side of it is not a
// letter, which is what keeps \operatorname{Hom}, \sin and \alpha out.
func plainLetters(out map[[2]string]int, math string) {
	runes := []rune(math)
	for i, r := range runes {
		if !isLetter(r) {
			continue
		}
		if i > 0 && (isLetter(runes[i-1]) || runes[i-1] == '\\') {
			continue
		}
		if i+1 < len(runes) && isLetter(runes[i+1]) {
			continue
		}
		out[[2]string{plainFace, string(r)}]++
	}
}

func isLetter(r rune) bool {
	return r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z'
}

// mathSpans is the text between dollar signs, display and inline.
//
// It is deliberately crude. A face in prose is not a thing Bourbaki sets and
// not a thing this rule is looking for, and the cost of missing a span is a
// change that goes unreported rather than one reported wrongly.
func mathSpans(text string) []string {
	var spans []string
	// The odd pieces of a split on $$ are display math. What is left over on the
	// even pieces is prose with inline math in it, whose odd pieces on a split
	// by a single $ are the spans.
	for i, block := range strings.Split(text, "$$") {
		if i%2 == 1 {
			spans = append(spans, block)
			continue
		}
		for j, part := range strings.Split(block, "$") {
			if j%2 == 1 {
				spans = append(spans, part)
			}
		}
	}
	return spans
}

// restoreScript puts back the script capitals a reading from a picture spelled
// calligraphic, and it is the one face the run corrects rather than reports.
//
// The general rule is the right one and this is a narrower claim than the rule
// it stands beside. On a born digital volume the extractor reads the font name
// out of the PDF and the model guesses a typeface from pixels at 300 dpi, so
// the text layer is not merely right more often, it has evidence the model does
// not have at all. Page 76 of Algebra VIII is the shape of it: thirty
// \mathscr in the native reading, thirty \mathcal in the reading that replaced
// it, and nothing else about the mathematics changed. Corpus wide both
// spellings are in heavy use, so the two are not interchangeable and a page
// that ends up with one of each is a page spelling one printed glyph two ways.
//
// The claim is narrowed four ways and every one of them matters. Only against a
// native reading, which the caller decides. Only that pair of faces. Only that
// direction, calligraphic back to script and never the other way. And only for
// a letter the native reading set in script and did not also set calligraphic,
// so a page that really did distinguish the two is left exactly as it is.
//
// Whether such a page exists was the thing to check before writing this, and
// the corpus says no. 139 pages use both commands, and on 137 of them no letter
// takes both faces, which is two different letters and not a distinction. The
// two that remain, page 42 of int-ix-fr and page 258 of int-vii-ix, are both
// OCR readings spelling the same letter both ways within the page: \mathcal{C}
// nine times against \mathscr{C} four on one, \mathcal{F} four against
// \mathscr{F} one on the other. That is one reader being inconsistent and not
// a book printing two glyphs, and neither page is native, so neither is
// reachable from here anyway.
//
// The replacement is not limited to math spans because it does not need to be.
// A face command outside mathematics is not something these volumes set, and
// rule 2 reports any bare TeX outside a span as a defect, so a \mathcal in
// prose is already a finding rather than a thing to preserve.
func restoreScript(was, now string) string {
	before := faces(was)
	want := map[string]bool{}
	for key := range before {
		if key[0] == "mathscr" {
			want[key[1]] = true
		}
	}
	for key := range before {
		if key[0] == "mathcal" {
			delete(want, key[1])
		}
	}
	if len(want) == 0 {
		return now
	}
	return calLetter.ReplaceAllStringFunc(now, func(match string) string {
		m := calLetter.FindStringSubmatch(match)
		letter := m[1]
		if letter == "" {
			letter = m[2]
		}
		if !want[letter] {
			return match
		}
		return `\mathscr{` + letter + `}`
	})
}

// calLetter matches \mathcal{C} and \mathcal C, and only a single letter,
// because a script capital is one letter and a group of two is something else.
var calLetter = regexp.MustCompile(`\\mathcal\s*(?:\{([A-Za-z])\}|([A-Za-z]))`)

// faceChanges is every letter whose face count differs between two readings of
// the same page, worst first.
//
// A letter has to have moved in a face command to be here at all. Plain counts
// drift on every repaired page and drift is not the question: the pass exists to
// put back mathematics the text layer dropped, so a page that gains a diagram
// gains the letters in it, and over the eleven pages of lie-vii-ix that shipped
// first that drift was fifty of the seventy four lines this produced. Once a
// letter qualifies its plain count is printed too, which is the half of the
// movement that says where the face went.
func faceChanges(page int, was, now string) []FaceChange {
	before, after := faces(was), faces(now)
	seen := map[[2]string]bool{}
	moved := map[string]bool{}
	var out []FaceChange
	for key := range before {
		seen[key] = true
	}
	for key := range after {
		seen[key] = true
	}
	for key := range seen {
		if key[0] != plainFace && before[key] != after[key] {
			moved[key[1]] = true
		}
	}
	for key := range seen {
		a, b := before[key], after[key]
		if a == b || !moved[key[1]] {
			continue
		}
		out = append(out, FaceChange{Page: page, Face: key[0], Letter: key[1], Was: a, Now: b})
	}
	sort.Slice(out, func(i, j int) bool {
		x, y := out[i], out[j]
		if d := abs(x.Now-x.Was) - abs(y.Now-y.Was); d != 0 {
			return d > 0
		}
		if x.Face != y.Face {
			return x.Face < y.Face
		}
		return x.Letter < y.Letter
	})
	return out
}
