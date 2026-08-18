// Package footnote is the mark a book prints beside a footnote, and what to do
// with it once the note is Markdown.
//
// The volumes mark their notes with symbols and restart on every page: an
// asterisk for the first, a dagger for the second, two asterisks for the third.
// Markdown numbers its notes itself and prints the number it chose, so a
// reading that keeps the printed symbol ends up carrying two marks for one
// note, "(*)[^1]" in the body and "[^1]: (*) ..." at the foot, and that is what
// the reader sees. The printed mark is furniture of that page's typesetting and
// the reference is the corpus's own, so the mark comes out.
//
// It does not come out blind. The mark is the only thing that says which note a
// reference belongs to, and it earns its keep on the pages where the model
// wrote the symbol and no reference at all: page 15 of the Theory of Sets sets
// "the empty set (*)" with the note under it and nothing joining the two. So
// the marks are read off the definitions first, and a symbol standing on its
// own becomes the reference whose definition carries that symbol.
//
// A symbol that neither of those two describes is left where it is and named.
// A section assembled out of twenty pages carries twenty renumbered notes and
// the same asterisk over and over, so "the asterisk means note 1" holds on a
// page and does not hold in a file, and guessing there would move a reference
// to the wrong note. Being wrong about which note a reader is sent to is worse
// than leaving the printing's mark on the page.
package footnote

import (
	"regexp"
	"strings"

	"github.com/tamnd/bourbaki-solver/mathtex"
)

// Kind is what was done with one printed mark.
type Kind string

const (
	// KindDefinition is the mark at the head of the definition itself, which
	// always comes out: the definition already says which note it is.
	KindDefinition Kind = "definition"
	// KindBeside is a mark printed next to a reference that is already there.
	// The reference stays and the mark goes.
	KindBeside Kind = "beside"
	// KindAlone is a mark standing where the reference should be, with no
	// reference anywhere in the body. It becomes the reference its definition
	// names.
	KindAlone Kind = "alone"
	// KindLeft is a mark that neither of those describes, left as the page
	// prints it. Either the body carries more than one note under that mark, or
	// the note it would point at is already pointed at from somewhere else.
	KindLeft Kind = "left"
)

// A Move is one printed mark and what became of it.
type Move struct {
	Line  int    // the body line it sits on, counting from one
	Mark  string // as the file writes it, escape and all
	Label string // the note it belongs to, empty when nothing said which
	Kind  Kind
}

var (
	// markRE is a footnote mark as the volumes print it and as both readings
	// write it. The asterisk arrives escaped or bare, because a bare one opens
	// emphasis in Markdown and only some of the readings knew that. Two
	// asterisks are looked for before one, or the third note of a page would
	// read as the first note twice.
	markRE = regexp.MustCompile(`\((?:\\?\*\\?\*|\\?\*|†|‡)\)`)

	// defRE is a footnote definition, which Markdown puts at the head of its
	// own line.
	defRE = regexp.MustCompile(`^\[\^([0-9a-zA-Z]+)\]:[ \t]*`)

	// refRE is a reference to a note from the body.
	refRE = regexp.MustCompile(`\[\^([0-9a-zA-Z]+)\]`)

	// besideRE is a reference standing right after a mark, with at most the
	// punctuation the sentence ended on between them. Page 244 of the Theory of
	// Sets sets "(*).[^1]" and page 15 sets "(*)[^1]", and both are one note
	// marked twice.
	besideRE = regexp.MustCompile(`^[.,;:!?]{0,2}[ \t]*\[\^([0-9a-zA-Z]+)\]`)
)

// Normalize takes the printed marks out of a body and returns it with the moves
// it made. A body with no marked definition in it is returned untouched, which
// is every page of the volumes that were read off a text layer.
func Normalize(body string) (string, []Move) {
	byLabel, byMark := definitions(body)
	if len(byLabel) == 0 {
		return body, nil
	}
	out, moves := references(body, byLabel, byMark)
	out, dropped := strip(out)
	return out, append(moves, dropped...)
}

// definitions reads the marks off the definitions: the mark of each note, and
// the notes carrying each mark. The second is a list and not a single note
// because an assembled § holds the notes of every page it was made of, and the
// first note of each of those pages is an asterisk.
func definitions(body string) (byLabel map[string]string, byMark map[string][]string) {
	byLabel, byMark = map[string]string{}, map[string][]string{}
	for _, line := range strings.Split(body, "\n") {
		d := defRE.FindStringSubmatch(line)
		if d == nil {
			continue
		}
		m := markRE.FindString(line[len(d[0]):])
		if m == "" || !strings.HasPrefix(line[len(d[0]):], m) {
			continue
		}
		byLabel[d[1]] = bare(m)
		byMark[bare(m)] = append(byMark[bare(m)], d[1])
	}
	return byLabel, byMark
}

// bare is a mark without the backslash a reading may have escaped it with, so
// that (*) and (\*) are the one mark they are on the page.
func bare(mark string) string { return strings.ReplaceAll(mark, `\`, "") }

// references takes the marks out of the body, leaving the definitions alone.
func references(body string, byLabel map[string]string, byMark map[string][]string) (string, []Move) {
	math := mathMask(body)
	referenced := map[string]bool{}
	for _, line := range strings.Split(body, "\n") {
		if defRE.MatchString(line) {
			continue
		}
		for _, r := range refRE.FindAllStringSubmatch(line, -1) {
			referenced[r[1]] = true
		}
	}

	var b strings.Builder
	var moves []Move
	last := 0
	for _, at := range markRE.FindAllStringIndex(body, -1) {
		s, e := at[0], at[1]
		if math[s] || defRE.MatchString(lineAt(body, s)) {
			continue
		}
		mark := body[s:e]
		move := Move{Line: lineOf(body, s), Mark: mark, Kind: KindLeft}
		cut := s
		var put string
		switch label := besideRE.FindStringSubmatch(body[e:]); {
		case label != nil && byLabel[label[1]] == bare(mark):
			// The mark and the reference are both here. Take the mark out, and
			// the space in front of it with it when what follows is the
			// punctuation the sentence ended on, or the sentence would end on a
			// space and then a full stop.
			move.Kind, move.Label = KindBeside, label[1]
			if s > last && body[s-1] == ' ' && strings.ContainsRune(".,;:!?", rune(body[e])) {
				cut = s - 1
			}
		case len(byMark[bare(mark)]) == 1 && !referenced[byMark[bare(mark)][0]]:
			// Nothing joins the mark to its note, and one note on this page
			// carries it. That note is what the mark means, and writing it as a
			// reference is the only way a reader gets from one to the other.
			move.Kind, move.Label = KindAlone, byMark[bare(mark)][0]
			put = "[^" + move.Label + "]"
		default:
			moves = append(moves, move)
			continue
		}
		b.WriteString(body[last:cut])
		b.WriteString(put)
		last = e
		moves = append(moves, move)
	}
	b.WriteString(body[last:])
	return b.String(), moves
}

// strip takes the mark off the head of each definition.
func strip(body string) (string, []Move) {
	lines := strings.Split(body, "\n")
	var moves []Move
	for i, line := range lines {
		d := defRE.FindStringSubmatch(line)
		if d == nil {
			continue
		}
		rest := line[len(d[0]):]
		m := markRE.FindString(rest)
		if m == "" || !strings.HasPrefix(rest, m) {
			continue
		}
		lines[i] = d[0] + strings.TrimLeft(rest[len(m):], " \t")
		moves = append(moves, Move{Line: i + 1, Mark: m, Label: d[1], Kind: KindDefinition})
	}
	return strings.Join(lines, "\n"), moves
}

// mathMask says, for each byte of the body, whether it is inside mathematics. A
// display can print an asterisk in brackets and mean multiplication, and the
// marks are read off the prose only.
func mathMask(body string) []bool {
	mask := make([]bool, len(body)+1)
	spans, _ := mathtex.Split(body)
	if len(spans) == 0 {
		return mask
	}
	// Split counts in runes and the marks are found in bytes, so the one has to
	// be turned into the other.
	at := make([]int, 0, len(body)+1)
	for i := range body {
		at = append(at, i)
	}
	at = append(at, len(body))
	for _, sp := range spans {
		if sp.Start < 0 || sp.End > len(at)-1 {
			continue
		}
		for j := at[sp.Start]; j < at[sp.End]; j++ {
			mask[j] = true
		}
	}
	return mask
}

// lineAt is the whole line the byte at i sits on.
func lineAt(body string, i int) string {
	start := strings.LastIndexByte(body[:i], '\n') + 1
	end := strings.IndexByte(body[i:], '\n')
	if end < 0 {
		return body[start:]
	}
	return body[start : i+end]
}

// lineOf is the line number of the byte at i, counting from one.
func lineOf(body string, i int) int {
	return strings.Count(body[:i], "\n") + 1
}
