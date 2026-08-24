package toc

import (
	"regexp"
	"slices"
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
	if numeral == "" {
		return body, false
	}
	i := blank(body, 0)
	j, head, ok := titleRun(body, i, title)
	if !ok {
		return body, false
	}
	out := make([]string, 0, len(body)+2)
	out = append(out, body[:i]...)
	out = append(out, "## "+ChapterWord(lang)+" "+numeral, "", "# "+head)
	out = append(out, body[j+1:]...)
	return out, true
}

// blank is the first line at or after i that has anything on it, or the end of
// the body where nothing does.
func blank(body []string, i int) int {
	for i < len(body) && strings.TrimSpace(body[i]) == "" {
		i++
	}
	return i
}

// titleRun is the run of plain lines starting at i that together say what the
// contents says the title is. It gives the line the run ends on and the title
// as one line.
//
// The lines are joined with a space. A press breaks a title at the measure and
// the reading keeps the break, so the title of a chapter is often two lines and
// neither of them says what the contents says while the two together do. The
// comparison is flattened on both sides, which is what lets a page that sets
// the title in capitals agree with a contents that sets it in mixed case.
//
// A line the join does not need is left where it stands. Page 225 of Topology I
// to IV sets a subtitle under the chapter title and the contents does not have
// it, so the run stops at the title and the subtitle stays a line of the page
// rather than being taken into a heading the book does not give.
func titleRun(body []string, i int, title string) (int, string, bool) {
	want := flatten(title)
	if want == "" {
		return 0, "", false
	}
	var run []string
	for j := i; j < len(body) && strings.TrimSpace(body[j]) != "" && plainLine(body[j]); j++ {
		run = append(run, strings.TrimSpace(body[j]))
		if flatten(strings.Join(run, " ")) == want {
			return j, strings.Join(run, " "), true
		}
	}
	return 0, "", false
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

// appendixLine is the word a volume heads an appendix with, standing alone on a
// line, with the number after it where a chapter has more than one appendix and
// numbers them.
//
// The words are the four assemble reads, since the volume chooses. Integration
// VII to IX calls its appendix an ANNEX throughout and the French volumes set
// ANNEXE beside APPENDICE, and a repair that insisted on one word would leave
// the other five volumes unassembled.
//
// The bold is allowed because the reading puts it there, and the full stop
// because some printings set one. Nothing else is allowed on the line: the word
// alone is the whole of what a page prints over an appendix, and requiring that
// is what keeps this off the sentence in a preface that happens to mention one.
var appendixLine = regexp.MustCompile(`(?i)^ *(?:\*\*)? *(appendi[xc]e?|annexe?)\.? *(\d+|[ivxIVX]+)? *\.? *(?:\*\*)? *$`)

// AppendixOpening puts back the heading over an appendix whose page kept the
// word but lost the level, or whose reading filed the word as the running head.
// It gives the body with the heading in it and whether the running head is now
// spent and should be dropped.
//
// Thirty nine appendices are in the contents of this corpus and twenty four of
// them are marked. The other fifteen fail in two ways and this repairs both.
// Seven have the word standing as a plain line in the body, which is the same
// fault SectionOpening repairs one shape up: the reading kept the words and
// dropped the hashes, so the words are still there to agree with. Seven more
// have it only in the running head, which happens because the word is the whole
// of what the page prints over an appendix and a reading that files the top
// line of a page as its running head takes the heading with it. Page 402 of
// Algebra I to III is one: the front matter has APPENDIX and the body opens on
// the title of the appendix with nothing above it.
//
// The running head goes when it is used, and that is the part worth arguing
// for. A running head is what a page prints at the top of every page of a run,
// and a word printed once over the opening of an appendix is not that. Leaving
// it in both places would put the word on the page twice, once as the heading
// the assembler reads and once as a running head no other page of the appendix
// carries.
//
// The number has to agree with the contents where the contents gives one. Nine
// of the fifteen are the only appendix of their chapter and are unnumbered, and
// the assembler reads an unnumbered appendix by the word alone, so a number on
// the page where the contents gives none is a line this does not touch. Where
// there is a number the page may set it as a digit or as a roman numeral and
// both are read, since Topology V to X numbers its two appendices in one
// printing and Lie IX in another.
//
// An appendix whose word is nowhere on the page and nowhere in the front matter
// is not put back, for the reason the head of this file gives: there would be
// nothing for the contents to agree with, and writing the word in would mean
// putting a line on a page that no reading of it ever produced.
func AppendixOpening(body []string, running, title string, number int) ([]string, bool, bool) {
	for i, line := range body {
		if !plainLine(line) {
			continue
		}
		m := appendixLine.FindStringSubmatch(line)
		if m == nil || !appendixNumber(m[2], number) {
			continue
		}
		out := slices.Clone(body)
		out[i] = "## " + appendixHead(m[1], number)
		return under(out, i+1, title), false, true
	}
	m := appendixLine.FindStringSubmatch(strings.TrimSpace(running))
	if m == nil || !appendixNumber(m[2], number) {
		return body, false, false
	}
	i := blank(body, 0)
	out := make([]string, 0, len(body)+2)
	out = append(out, body[:i]...)
	out = append(out, "## "+appendixHead(m[1], number), "")
	out = append(out, body[i:]...)
	return under(out, i+2, title), true, true
}

// under puts the title of an appendix under the heading that was just written
// over it, where the page carries the title as a plain line and the contents
// agrees with it.
//
// An appendix is headed by its word alone and the title is set under the word
// in its own type, so the reading loses the level on two lines rather than one.
// The assembler reads the title from under the word and wants a heading there:
// page 402 of Algebra I to III has the word in the running head and
// PSEUDOMODULES as the first plain line of the body, and marking the word alone
// gets as far as the assembler saying the page titles the appendix nothing
// while the contents calls it Pseudomodules.
//
// Where there is no title to find the body comes back as it was, which is the
// case that has to keep working. Chapter IX of Algebre commutative chapitres 8
// et 9 closes on an appendix the contents gives no title at all: the page
// prints the word centred and alone and the next thing on it is the heading of
// its first no. Reading a title there would take that heading.
func under(body []string, i int, title string) []string {
	i = blank(body, i)
	j, head, ok := titleRun(body, i, title)
	if !ok {
		return body
	}
	out := make([]string, 0, len(body))
	out = append(out, body[:i]...)
	out = append(out, "# "+head)
	out = append(out, body[j+1:]...)
	return out
}

// appendixMark is the heading the assembler reads over an appendix, and it is
// laxer than appendixLine on purpose. The repair asks for the word standing
// alone because that is what an unmarked page prints and anything else on the
// line means the line is something other than a heading. This asks whether the
// page is already marked, and a page that is already marked has whatever the
// volume's own hand put there: Algebra VIII sets the word, the number and the
// title of the appendix all on one line, and four of its appendices came back
// as unmarked when this was written the strict way.
//
// The number is not checked here for the same reason the assembler does not
// check it: a chapter with one appendix has one heading over it and a heading
// with the word in it is that heading whatever follows the word.
var appendixMark = regexp.MustCompile(`(?i)^#{1,4} +(?:\*\*)? *` + appendixWord + `\b`)

// appendixWord is the four words a volume of this corpus heads an appendix
// with. It is the same list the assembler reads, since a repair that put back a
// word the assembler does not read would leave the volume unassembled.
const appendixWord = `(?:appendi[xc]e?|annexe?)`

// Appendix is whether the body already opens an appendix, which is the test
// that keeps the repair from writing a second heading onto a page that has one.
// Twenty four of the thirty nine appendices in this corpus are already marked
// and every one of them has to come through untouched.
func Appendix(body []string) bool {
	return slices.ContainsFunc(body, appendixMark.MatchString)
}

// appendixHead is the heading that goes back, which is the word the page prints
// and the number the contents gives.
//
// The word is upper cased because that is how the corpus sets the twenty four
// appendices that are already marked, and because the page prints it that way
// in every one of the fifteen this repairs. The number is the contents' own,
// written as a digit, since assemble reads either and one of the two has to be
// chosen for the corpus to be consistent with itself.
func appendixHead(word string, number int) string {
	head := strings.ToUpper(word)
	if number > 0 {
		head += " " + strconv.Itoa(number)
	}
	return head
}

// appendixNumber is whether what the page sets after the word is the number the
// contents gives the appendix.
//
// The empty case is the common one and it has to be exact in both directions. A
// chapter with one appendix does not number it and the contents gives it no
// number either, so nothing after the word is right and a number after it is a
// line that belongs to something else. A chapter with two numbers both and the
// page may set the number as a digit or as a roman numeral.
func appendixNumber(had string, number int) bool {
	if had == "" {
		return number == 0
	}
	if number == 0 {
		return false
	}
	if n, err := strconv.Atoi(had); err == nil {
		return n == number
	}
	return strings.EqualFold(had, roman(number))
}

// roman is a number written the way a printing sets the number of an appendix.
// Nothing in this corpus numbers more than two, so the table stops where the
// evidence does rather than where a general algorithm would.
func roman(n int) string {
	switch n {
	case 1:
		return "I"
	case 2:
		return "II"
	case 3:
		return "III"
	case 4:
		return "IV"
	}
	return ""
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
