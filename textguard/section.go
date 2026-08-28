package textguard

import (
	"regexp"
	"strings"
)

// The books refer to their own parts by the section sign and a numeral, "(§ 1,
// n° 8)" or "(chap. III, § 2, n° 4, prop. 7)", and there is a reference of that
// shape on most pages of most volumes. The corpus writes the sign itself, and it
// writes it that way 5729 times against 918 for the LaTeX spelling and 191 for
// an escaped dollar.
//
// The escaped dollar is the one that does damage. A model that has been asked
// for Markdown with mathematics in it knows that a dollar has to be escaped to
// stand for itself, and it reaches for that escape when what it is looking at is
// a section sign it has no character for. What comes back is "(\$ 1, n° 8)",
// which reads as a section reference and counts as one more dollar on the line.
// Every rule that finds mathematics here finds it by counting dollars, so one
// stray escape moves the boundary of every span after it on the page: prose is
// read as a formula and the formula after it as prose, for the rest of the file.
// Integration chapitres 1 a 4 has 25 of them and 9 of its section files came out
// with an odd count.
//
// The LaTeX spelling does no damage. \S renders as the sign and always has, so
// this is the corpus preferring one spelling of a thing to another, for the
// reason it prefers one spelling of a delimiter: a text with two spellings in it
// has to be searched twice, and the second search is the one somebody forgets.
//
// Both are turned into the sign. Neither has a second reading in these books:
// nothing in Bourbaki is priced, and \S has no meaning in mathematics.
type Section struct {
	Line int    // the body line it sits on, counting from one
	Name string // which spelling it is
	Text string // the line it was found on
}

// The sign has to be in front of a numeral to be one. An escaped dollar in front
// of anything else is a dollar somebody meant, and \S in front of a letter is
// the start of a longer command: \Sigma and \Subset and \Supset are all in the
// corpus and all begin this way.
//
// The space between the sign and the numeral is the printing's and the corpus
// keeps it, so a reference set close is opened up and one set open is left as it
// is. The books set it open.
var (
	dollarSign  = regexp.MustCompile(`\\\$\s*(\d)`)
	latexSign   = regexp.MustCompile(`\\S\s*(\d)`)
	sectionName = []struct {
		re   *regexp.Regexp
		name string
	}{
		{dollarSign, "a section sign written as an escaped dollar"},
		{latexSign, "a section sign written as a LaTeX command"},
	}
)

// SectionSign writes the corpus's section sign and says how many it wrote.
//
// It is idempotent, since a text it has been through carries no backslash in
// front of its section signs for it to find on a second pass.
func SectionSign(text string) (string, int) {
	n := 0
	for _, s := range sectionName {
		n += len(s.re.FindAllStringIndex(text, -1))
	}
	if n == 0 {
		return text, 0
	}
	out := dollarSign.ReplaceAllString(text, "§ $1")
	out = latexSign.ReplaceAllString(out, "§ $1")
	return out, n
}

// Sections is the same reading with nothing given back, for the audit. One
// finding per line, naming the first spelling on it, which is enough to send a
// reader to the right place.
func Sections(text string) []Section {
	var out []Section
	for i, line := range strings.Split(text, "\n") {
		at, name := -1, ""
		for _, s := range sectionName {
			if m := s.re.FindStringIndex(line); m != nil && (at < 0 || m[0] < at) {
				at, name = m[0], s.name
			}
		}
		if at >= 0 {
			out = append(out, Section{Line: i + 1, Name: name, Text: line})
		}
	}
	return out
}
