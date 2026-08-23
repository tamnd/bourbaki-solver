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
//
// The space after the sign is optional because the printing does not always set
// one and the reading does not always keep one. Integration VII to IX has
// "§1. CONSTRUCTION OF A HAAR MEASURE" on page 7 and every one of its thirteen
// § openings is set that way, so requiring the space refused the lot of them and
// left the volume unassembled. What goes back is normalised, see sectionSign.
//
// The bold is allowed in two places because the reading puts it in two places.
// Page 103 of Topology I to IV has "**10. PROPER MAPPINGS**", the whole line in
// bold, and page 137 of Algebre commutative chapitres 8 et 9 has
// "3. **Existence et unicité des $ p $-anneaux**", the title alone. Only the
// outer pair was written down at first, and since the comparison against the
// contents throws away everything that is not a letter or a digit, the second
// shape matched anyway: the closing pair was taken off the end and the opening
// one stayed on the front of the title, and the heading that went back carried
// one half of a pair of asterisks. Both are stripped now.
var lostSection = regexp.MustCompile(`^(\*\*)?(§ *)?([0-9A-Za-z]{1,4})\. +(?:\*\*)?(.+?)(\*\*)?$`)

// sectionSign is what goes in front of the number of a § heading, given what the
// page had in front of it.
//
// The page decides whether there is a sign at all, since Algebra I to III prints
// one and Topology I to IV does not, and the assembler reads either. It does not
// decide the spacing: the corpus sets "## § 1." in all 143 § headings that carry
// a sign, so a page that ran the sign into the number gets the space put in
// rather than carried through.
func sectionSign(had string) string {
	if had == "" {
		return ""
	}
	return "§ "
}

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
	return opening(body, number, title, "## ", true)
}

// NumberOpening puts back the heading over a no. whose page kept the title and
// lost the level. It is the same repair as SectionOpening one level down, and
// it is the same fault: page 32 of Theory of Sets sets PROOFS as its running
// head and "2. PROOFS" under it, and the reading kept one of the two.
//
// A no. is told from the § it belongs to by the sign. The printings that set a
// sign set it over the § and never over a no., so a line that carries one is
// refused here, and a printing that sets no sign at all leaves the number and
// the title to do the telling, which is what they do everywhere else in this
// file. Where the § has already been put back it is a heading by then and no
// heading is looked at twice.
func NumberOpening(body []string, number int, title string) (int, int, string, bool) {
	return opening(body, number, title, "### ", false)
}

// opening is the run of lines a heading was set on and the heading that goes
// back over them. level is the hashes the assembler reads it at, and sign is
// whether the printing is allowed to set § in front of the number.
func opening(body []string, number int, title, level string, sign bool) (int, int, string, bool) {
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
		if m[2] != "" && !sign {
			continue
		}
		run := []string{m[4]}
		for j := i; ; j++ {
			if flatten(strings.Join(run, " ")) == want {
				return i, j, level + sectionSign(m[2]) + strconv.Itoa(number) + ". " + strings.Join(run, " "), true
			}
			if j+1 >= len(body) || strings.TrimSpace(body[j+1]) == "" || !plainLine(body[j+1]) {
				break
			}
			run = append(run, strings.TrimSpace(body[j+1]))
		}
	}
	return 0, 0, "", false
}

// RunningHeadOpening is the heading a reading filed as the running head of the
// page, for a page where the two carry the same words.
//
// A recto sets the title of the current no. at the head of the page and the
// heading of the no. right under it, so a page where a no. begins prints those
// words twice, once without the number and once with it. Page 32 of Theory of
// Sets heads the page PROOFS and opens no. 2 under "2. PROOFS", and page 69 of
// Algebra I to III does the same with PRODUCTS AND FIBRE PRODUCTS. The reading
// keeps one line of the two, files it as the running head, and the body loses
// its heading.
//
// What makes this a repair rather than a re-reading is that the line is still
// in the file. It is in the front matter instead of the body, and the number on
// it is the tell: a running head carries no number and a heading does, so a
// running head that reads as "2. Proofs" where the contents opens no. 2 under
// that title is the heading and not the running head. Both go back, the heading
// to the top of the page where the no. begins, and the running head to the
// words without the number, which is what the page prints over them.
//
// A page whose running head is anything else is left alone and reported. The
// chapter title, the title with no number on it, or a paragraph the reading
// swallowed whole are all cases where the heading is not in the file at all.
func RunningHeadOpening(runningHead string, number int, title string) (string, string, bool) {
	m := lostSection.FindStringSubmatch(strings.TrimSpace(runningHead))
	if m == nil || m[2] != "" || !sectionNumber(m[3], number) {
		return "", "", false
	}
	if flatten(m[4]) != flatten(title) {
		return "", "", false
	}
	return "### " + strconv.Itoa(number) + ". " + m[4], m[4], true
}

// numbered is a heading that opens on a number, with whatever the printing
// sets between the hashes and the number.
var numbered = regexp.MustCompile(`^(#{2,4}) +(?:\*\*)?(?:\\?\*)?(?:§ *)?(\d+)\.`)

// Numbered is whether a page already carries a heading at this many hashes
// under this number, so that the repair leaves it alone.
//
// What comes between the hashes and the number is not part of either. § 21 no.
// 13 of Algebra VIII is a starred no., which the reading writes "### \*13.",
// and a heading the reading put bold round is "### **13.". Reading those as
// missing headings would send somebody to a page image over a heading that is
// on the page and correct.
func Numbered(body []string, hashes, number int) bool {
	for _, line := range body {
		m := numbered.FindStringSubmatch(line)
		if m == nil || len(m[1]) != hashes {
			continue
		}
		if n, err := strconv.Atoi(m[2]); err == nil && n == number {
			return true
		}
	}
	return false
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
