package solve

import (
	"regexp"
	"strings"
)

// A Bourbaki exercise runs to a) through h) more often than not, and the parts
// are not decoration. They are separate problems printed under one number, and
// one of them being wrong says nothing about the others.
//
// This is what makes the partial status mean something. Without the parts, an
// exercise where four parts of five are right is either published whole, which
// is a lie about the fifth, or thrown away whole, which wastes the four. With
// them the judges answer a line each and the file says which is which.

// partHere is a part marker where a part marker can stand: at the start of the
// exercise, at the start of a line, or after a sentence has ended.
//
// That last case is what stops this being a line-start rule. The book prints
// markers inline. Exercise 12 of § 21 closes a part with "(calculate the order
// of L). f ) Let h be in H", and exercise 3 of § 1 opens "a) Give an example of
// a commutative field", runs a paragraph, and says "b) Give an example of a
// module of finite length" in the middle of it. An exercise judged on two of
// its three parts is worse than one judged on none.
//
// What the sentence has to have ended for is the back-references, which are the
// same three characters in the same order and are not parts. The chapter is full
// of "(use a) to calculate", "deduce from c) that L is", "apply Exercise 23, d)
// of I, §6, p. 145". Every one of them follows a word or a comma, and no marker
// the book sets does. Reading them as parts gave forty of chapter VIII's
// exercises exactly five parts, a) through e), which is not a thing a book does
// forty times, and it hid the longer exercises by stopping their letter walks
// early.
//
// The space inside the marker is optional because the printing has "f )" as
// often as "f)", and losing f would end the walk with two parts still to come.
// The dollar in the line-start class is the star this corpus sets on a starred
// part, "$*$c) Assume from now on", which is markup and not a word.
var partHere = regexp.MustCompile(`(?:\A|\n[ \t>*_$]*|[.!?][)$*_]*[ \t]+)\(?([a-z])[ \t]?\)[ \t]`)

// last is the last letter a part can be. It is h and not z because the book
// numbers a list of equivalent conditions (i), (ii), (iii), and an exercise
// with eight parts followed by such a list would otherwise read as having nine.
// No exercise in the corpus prints an i) part.
const last = 'h'

// Parts is the lettered parts of an exercise, in the order the book sets them,
// and nothing at all for an exercise that has none.
//
// The letters have to run a, b, c with no gaps and in order, and a marker out
// of turn is passed over rather than taken. That is the second half of the
// defence against a false reading and it is what lets a missed back-reference
// cost nothing: "Deduce from a) and b)" in the middle of part e is two letters
// the walk is long past wanting.
//
// What it costs is an exercise that prints a) and then c), which would be a
// misprint, and one that starts at b), which would be a hole in the extraction
// worth finding some other way. Neither happens in the chapter this was
// measured on, in either language.
func Parts(body string) []string {
	var out []string
	want := byte('a')
	for _, m := range partHere.FindAllStringSubmatchIndex(body, -1) {
		if body[m[2]] != want {
			continue
		}
		out = append(out, string(want))
		if want == last {
			break
		}
		want++
	}
	if len(out) < 2 {
		// One part is no parts. An exercise whose body happens to contain an a)
		// and nothing after it is a sentence with a bracket in it, not a
		// decomposition, and asking a judge for a line about part a of a one part
		// exercise gives the store a part the book never set.
		return nil
	}
	return out
}

// Exercise is the text of the exercise the context was built for.
func (c *Context) Exercise() string {
	for _, p := range c.Pieces {
		if p.Kind == TheExercise {
			return p.Text
		}
	}
	return ""
}

// PartsOf is the parts of the exercise this context was built for.
func (c *Context) PartsOf() []string { return Parts(strings.TrimSpace(c.Exercise())) }
