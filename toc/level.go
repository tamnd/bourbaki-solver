package toc

import (
	"regexp"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// A § and a no. are printed the same way: a number, a full stop and a title in
// caps, alone on a line. Nothing on the page says which of the two it is, and
// the reading has to decide by the size of the type. It mostly does, and on
// Theory of Sets it got eight of them wrong, all in the same direction: a no.
// written as a §.
//
// The contents knows. It gives every § and every no. of every chapter with the
// page it starts on, so a heading that carries a number and a title can be
// looked up rather than guessed at, and the answer is a fact about the printed
// book rather than a judgement about type. That is what Level is.

// HeadingRE is a numbered heading as extraction writes it, at either level. The
// star marks a subsection the book sets as supplementary.
var HeadingRE = regexp.MustCompile(`^(#{2,3}) (\\\*)?(\d+)\. (.+)$`)

// Heading is one such line, taken apart.
type Heading struct {
	Level  int    // 2 or 3, as written
	Star   string // the supplementary marker, kept as it stands
	Number int
	Title  string
}

// ParseHeading reads a numbered heading. A line that is not one is not a
// heading, and the second return says so.
func ParseHeading(line string) (Heading, bool) {
	m := HeadingRE.FindStringSubmatch(line)
	if m == nil {
		return Heading{}, false
	}
	n, err := strconv.Atoi(m[3])
	if err != nil {
		return Heading{}, false
	}
	return Heading{Level: len(m[1]), Star: m[2], Number: n, Title: m[4]}, true
}

// Write puts a heading back as a line, at the level given.
func (h Heading) Write(level int) string {
	return strings.Repeat("#", level) + " " + h.Star + strconv.Itoa(h.Number) + ". " + h.Title
}

// Level is what the contents says a numbered heading printed on this PDF page
// is: 2 for a §, 3 for a no., and 0 for a heading the contents does not have
// there.
//
// Both the number and the title have to agree, and not the number alone. A § and
// its first no. begin on the same page nine times out of ten and both are
// numbered 1, so a lookup by number would call "### 1. SIGNS AND ASSEMBLIES" a §
// on the strength of "§ 1. TERMS AND RELATIONS" being on that page. The titles
// are what separate them, and they are matched the way toc.Verify matches them,
// stripped to letters and digits, because the contents and the body are two
// separate readings of two separate pieces of type.
//
// A heading the contents gives as both is 0 rather than either. That cannot
// happen in this corpus and it is not the business of a repair to decide it if
// it ever does.
func Level(b corpus.BookTOC, pdfPage, number int, title string) int {
	want := normalize(title)
	section, subsection := false, false
	for _, c := range b.Chapters {
		for _, s := range c.Sections {
			if s.PDFPage == pdfPage && s.Number == number && normalize(s.Title) == want {
				section = true
			}
			for _, ss := range s.Subsections {
				if ss.PDFPage == pdfPage && ss.Number == number && normalize(ss.Title) == want {
					subsection = true
				}
			}
		}
	}
	switch {
	case section && !subsection:
		return 2
	case subsection && !section:
		return 3
	}
	return 0
}
