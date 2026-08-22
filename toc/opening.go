package toc

import (
	"regexp"
	"strconv"
	"strings"
)

// A chapter opens under two headings and a § opens under one, and they are the
// largest type on their pages. Level is about the two smaller ones, the § and
// the no., which are printed alike and are told apart by the contents. These
// are not printed like anything else in the volume and the reading still loses
// them, in three ways: it keeps the words and drops the level, it keeps the
// words and misreads the number, or it drops the line altogether.
//
// The first two are repaired here and the third is not. What the two have in
// common is that the words are still on the page, so the contents has something
// to agree with and the repair is the same one Level makes: the page keeps its
// own words and the contents settles what level they are at. Where the line is
// gone there is nothing to agree with, and putting the heading back would mean
// writing onto a page a line the reading never saw. That is a re-reading of the
// page image and not a repair of the Markdown.

// ChapterWord is the word a printing sets over the number of a chapter, which
// is the one thing assemble looks for before a volume begins.
//
// It is here rather than taken from assemble's printings because a repair of
// pages/ cannot depend on the package that reads pages/ into content/, and
// because this is the whole of what a printing contributes: the number is the
// contents' and the title is the page's.
func ChapterWord(lang string) string {
	if lang == "fr" {
		return "CHAPITRE"
	}
	return "CHAPTER"
}

// ChapterOpening puts back the heading over a chapter whose page kept the title
// and lost everything above it.
//
// Page 22 of Theory of Sets opens the first chapter and came back as
//
//	Description
//	of Formal Mathematics
//
//	## 1. TERMS AND RELATIONS
//
// where the page prints CHAPTER I over a title set in the largest type in the
// volume. assemble finds a chapter by "## CHAPTER" and nothing else is one, so
// the volume does not begin and no part of it assembles.
//
// The title is what makes this safe. The contents gives the chapter a title and
// the page still carries it, in its own words and broken across the lines the
// press broke it across, so the two are put side by side and only a page whose
// opening lines say what the contents says is written to. The run has to start
// at the first line of the page, which is where a chapter title is and where
// nothing else is.
//
// The lines are joined with a space and set as one heading. The break between
// "Description" and "of Formal Mathematics" is the width of the measure and not
// part of the title, which is exactly what the contents is being asked. A line
// the join does not need is left where it stands: page 225 of Topology I to IV
// sets "Topological Groups" and "(Elementary Theory)" under it, the contents
// calls that chapter "Topological Groups", so the subtitle stays a line of the
// page rather than being taken into a heading the book does not give.
func ChapterOpening(body []string, lang, numeral, title string) ([]string, bool) {
	want := flatten(title)
	if want == "" || numeral == "" {
		return body, false
	}
	i := 0
	for i < len(body) && strings.TrimSpace(body[i]) == "" {
		i++
	}
	var run []string
	for j := i; j < len(body) && strings.TrimSpace(body[j]) != "" && plainLine(body[j]); j++ {
		run = append(run, strings.TrimSpace(body[j]))
		if flatten(strings.Join(run, " ")) != want {
			continue
		}
		out := make([]string, 0, len(body)+2)
		out = append(out, body[:i]...)
		out = append(out, "## "+ChapterWord(lang)+" "+numeral, "", "# "+strings.Join(run, " "))
		out = append(out, body[j+1:]...)
		return out, true
	}
	return body, false
}

// plainLine is a line with no markup at the front of it, which is what a
// heading the reading took for prose looks like. A line that is already a
// heading is not one of these and neither is a line that opens in mathematics
// or in bold, since both of those carry a decision the reading made and this
// is for the lines where it made none.
func plainLine(s string) bool {
	return !strings.HasPrefix(s, "#") && !strings.HasPrefix(s, "$") && !strings.HasPrefix(s, ">")
}

// lostSection is a § heading with no level on it: the number, a full stop and
// the title, with the section sign in front of it where the printing sets one
// and the bold the reading sometimes puts round the whole line.
//
// The number is taken as characters and not as a number, because the character
// is the thing that was misread. See sectionNumber.
var lostSection = regexp.MustCompile(`^(\*\*)?(§ )?([0-9A-Za-z]{1,4})\. +(.+?)(\*\*)?$`)

// SectionOpening puts back the heading over a § whose page kept the title and
// lost the level, the number, or both. It gives the run of lines it replaces,
// from and to inclusive, and the one line that replaces them, and false where
// the page has nothing the contents can agree with.
//
// Four shapes turn up and the contents settles all four the same way. Page 10
// of Algebra IV to VII has "§ 1. POLYNOMIALS", which is the heading with the
// hashes gone. Page 103 of Topology I to IV has "**10. PROPER MAPPINGS**",
// which is the heading in bold, and that page also carries "**1. PROPER
// MAPPINGS**" twelve lines down, the first no. of the same §, under the same
// title. Page 23 of the same volume has "I. OPEN SETS, NEIGHBOURHOODS, CLOSED
// SETS", which is § 1 with the digit read as a letter, and page 113 has "II.
// CONNECTEDNESS", which is § 11 read the same way twice. Page 267 has
//
//  5. INFINITE SUMS
//     IN COMMUTATIVE GROUPS
//
// which is one heading the press broke at the measure and the reading kept
// broken, so it is joined the way a chapter title is joined.
//
// The number and the title both have to agree with the contents, which is what
// tells the § from its own first no. on page 103 and what keeps this off the
// lines of "I. THEORY OF SETS" and "II. ALGEBRA" that page 16 of the same
// volume sets when it lists the Books of the Éléments. Those are on a page the
// contents does not open a § on, so they are never looked at.
//
// The number is written back as the contents gives it and the title as the page
// gives it. The sign is kept where the page has one: Algebra I to III and
// Algebra IV to VII print it, Theory of Sets and Topology I to IV set the
// number alone, and the assembler reads either.
func SectionOpening(body []string, number int, title string) (int, int, string, bool) {
	want := flatten(title)
	if want == "" {
		return 0, 0, "", false
	}
	for i, line := range body {
		if !plainLine(line) {
			continue
		}
		m := lostSection.FindStringSubmatch(line)
		if m == nil || !sectionNumber(m[3], number) {
			continue
		}
		run := []string{m[4]}
		for j := i; ; j++ {
			if flatten(strings.Join(run, " ")) == want {
				return i, j, "## " + m[2] + strconv.Itoa(number) + ". " + strings.Join(run, " "), true
			}
			if j+1 >= len(body) || strings.TrimSpace(body[j+1]) == "" || !plainLine(body[j+1]) {
				break
			}
			run = append(run, strings.TrimSpace(body[j+1]))
		}
	}
	return 0, 0, "", false
}

// SectionTitle is what a page calls the § the contents numbers this way, for a
// page where the two do not agree.
//
// It is a report and not a repair. Page 36 of Algebra I to III heads § 2
// "IDENTITY ELEMENT; CANCELABLE ELEMENTS; INVERTIBLE ELEMENTS" and the contents
// spells the middle word with two l. One of the two is a misreading and the
// heading cannot be put back until somebody says which, so the two are printed
// side by side. That is a different piece of work from a heading the reading
// dropped, and telling them apart saves reading the page image for a fault that
// is not in the page image.
func SectionTitle(body []string, number int) (string, bool) {
	for _, line := range body {
		if !plainLine(line) {
			continue
		}
		if m := lostSection.FindStringSubmatch(line); m != nil && sectionNumber(m[3], number) {
			return m[4], true
		}
	}
	return "", false
}

// sectionNumber is whether what the page has in front of a § title is the
// number the contents gives it.
//
// A reading of a page image confuses the digit 1 with the letters I and l, and
// the digit 0 with the letter O, and it does it in the largest type as readily
// as anywhere else. § 1 of chapter I of Topology I to IV came back as "I." and
// § 11 of the same chapter as "II.", which is the same confusion twice in the
// same number. Reading those as roman numerals would make the second of them 2,
// so they are not roman numerals here: they are digits that were read as the
// letters they are shaped like, and turning the letters back is what the page
// means.
//
// Nothing is decided by this on its own. The title has to agree with the
// contents as well, and the page has to be the page the contents opens the §
// on, so a line that arrives at a number this way arrives at the right one.
func sectionNumber(s string, n int) bool {
	digits := strings.Map(func(r rune) rune {
		switch r {
		case 'I', 'l':
			return '1'
		case 'O':
			return '0'
		}
		return r
	}, s)
	got, err := strconv.Atoi(digits)
	return err == nil && got == n
}
