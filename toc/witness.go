package toc

import (
	"regexp"
	"strings"
	"unicode"
)

// WitnessSection says whether the text layer of a page prints the § heading the
// contents puts on that page, and gives back the words it prints it in.
//
// This is not the repair. The rest of this file puts a heading back in the
// page's own words, and it can, because the words are on the page and only the
// level was lost. Twenty six openings in this corpus are not on the page at all,
// and for those there is nothing to put a level on. What is left to ask is
// whether the heading is printed on the paper, and there is a second reading of
// the same paper that can answer it: the text layer the publisher's own scan
// left in the PDF.
//
// It is a worse reading than the one that made the corpus and the difference is
// not small. Page 213 of Algebre I a III has "5 2. MODULES D'APPLICATIONS
// LINGAIRES." where the book prints "§ 2. MODULES D'APPLICATIONS LINÉAIRES.",
// with the section sign read as a five and an accented E read as a G. Page 250
// breaks "ET" into "E T". So the words it gives are not words to write into a
// page, and this returns them for the report and for nothing else. The caller
// writes the title the contents gives, which is the same book's own statement of
// the same title, set in type on its contents pages.
//
// What the loose reading buys is the number, and the number is the whole of the
// test. A running head carries the title and no number, which is what turned
// page 177 of Topologie generale V a X away from ESPACES POLONAIS ; ESPACES
// SOUSLINIENS in the head and on to the real § 6 two lines further down. The
// first no. of a § carries a number and it is 1, so it is refused unless the §
// is § 1. A wrong match needs a line numbered as the § is numbered, titled as
// the § is titled, on the one page the contents opens that § on.
func WitnessSection(page string, number int, title string) (string, bool) {
	if flatten(title) == "" {
		return "", false
	}
	lines := strings.Split(page, "\n")
	best, got := 0.0, ""
	for i, line := range lines {
		m := witnessSection.FindStringSubmatch(strings.TrimSpace(line))
		if m == nil || !sectionNumber(m[1], number) {
			continue
		}
		// The press broke a long title at the measure and the scan kept it
		// broken, so the run of lines is tried and not only the first of them.
		// § 6 of chapter IX of Topologie generale is set over two lines and § 4
		// of chapter II of Algebre over two more.
		run := []string{m[2]}
		for j := i; ; j++ {
			if s := titleScore(title, strings.Join(run, " ")); s > best {
				best, got = s, strings.Join(run, " ")
			}
			if j+1 >= len(lines) || strings.TrimSpace(lines[j+1]) == "" {
				break
			}
			run = append(run, strings.TrimSpace(lines[j+1]))
		}
	}
	if best < titleFloor {
		return "", false
	}
	return got, true
}

// HistoricalNote is the line of a page that carries the words a chapter's
// historical note is headed by, with the level lost, or -1 where the page
// carries no such line. It gives back whatever the line carries after the words,
// which is a parenthetical where it is anything at all.
//
// The words have to be the whole of the line, bar that parenthetical. Page 679
// of Algebra I to III sets HISTORICAL NOTE and then "(Chapters II and III)"
// under it on a line of its own, which is a line of the note and not part of its
// head, and a rule that took any line beginning with the words would swallow
// whatever a printing sets after them.
func HistoricalNote(lines []string) (int, string) {
	for i, line := range lines {
		if !plainLine(line) {
			continue
		}
		if rest, ok := historicalHead(line); ok {
			return i, rest
		}
	}
	return -1, ""
}

// HistoricalNoteFromHead says whether the reading filed the words a historical
// note is headed by as the page's running head, and gives back whatever the
// head carries after them.
//
// It is asked only about the page the contents opens the note on, and that page
// carries no running head: the note begins there, the press sets its head in
// display type at the top of the leaf, and nothing else is printed above the
// text. So the words standing in that field are the head of the note and not a
// head at all. Every one of the six chapters of General Topology V to X is in
// this state, with running_head "HISTORICAL NOTE" and a body that opens on the
// note's first parenthetical.
//
// The second and later pages of a note do carry HISTORICAL NOTE as a genuine
// running head, and reading one of those as an opening would put a second head
// in the middle of the note. Nothing here can reach them. The contents gives one
// page per note and it is the first.
func HistoricalNoteFromHead(running string) (string, bool) {
	return historicalHead(strings.TrimSpace(running))
}

// WitnessHistorical says whether the text layer of a page prints the words a
// historical note is headed by, and gives back what the layer's line carries
// after them. It is the same second reading WitnessSection asks, and it is asked
// the same narrow question: not what the words are, which the printing already
// settles, but whether they are on the paper.
func WitnessHistorical(page string) (string, bool) {
	at, rest := HistoricalNote(strings.Split(page, "\n"))
	return rest, at >= 0
}

// WitnessMark says whether the text layer of a page prints the mark a § is
// marked off by inside a chapter's gathered exercises, which is the sign and the
// number on a line of their own.
//
// It is the narrowest of the three witnesses and it has to be. The mark is two
// or three characters of display type standing alone in white space, so a bad
// layer loses it altogether more often than it mangles it, and what it does give
// back is the same shape as a hundred other short lines in a block of exercises:
// a formula number, a page number, a footnote marker. So the sign is required.
// Page 671 of Algebra I to III gives "§II" with no space and the 1s read as
// capital I, which sectionNumber already folds, and that is as far as this
// stretches.
func WitnessMark(page string, number int) bool {
	for _, line := range strings.Split(page, "\n") {
		m := witnessMark.FindStringSubmatch(strings.TrimSpace(line))
		if m != nil && sectionNumber(m[1], number) {
			return true
		}
	}
	return false
}

// witnessMark is the sign and the number alone on a line, with the sign read the
// way an old scan reads it and the spacing left open. The sign class is the one
// witnessSection uses less the characters that are digits or read as digits: a
// bare "5" or "9" on a line of a layer is a number, not a sign, and there is no
// title after it here to say otherwise.
var witnessMark = regexp.MustCompile(`^(?:\*\*)?[§$SsG]\s*([0-9IlO]{1,4})\s*\.?\s*(?:\*\*)?$`)

// historicalHead says whether a line is nothing but the words a historical note
// is headed by, and gives back the parenthetical the line carries after them.
//
// The head is set in display type and the older French volumes have no text
// layer of their own, only a scan of one, so the second reading comes back with
// the head mangled in four ways this corpus was found with and none of them a
// guess. Page 183 of Algebre IX gives "NOTE H I S T O R I Q U E", the
// letterspacing read as spaces between the letters. Page 100 of Integration VI
// gives "NOTE HISTOKIQUE" and page 111 of Integration IX gives "KOTE
// HISTORIQUE", one letter wrong in each. Page 233 of Groupes et algebres de Lie
// IV a VI gives "NOTE HISTORIQUE (chapitres IV, V et VI)." with the chapters the
// note covers set on the same line as the head and a stop after them.
//
// So the words are taken as letters with everything else thrown away, which
// answers the letterspacing, and one letter is allowed to be wrong, which
// answers the other two. A parenthetical and a stop are allowed at the end and
// handed back rather than swallowed, because on most pages the parenthetical is
// a line of its own that the page already carries and on this one page it is
// not, and that is a difference somebody should see rather than one this rule
// should decide.
func historicalHead(line string) (string, bool) {
	line = strings.TrimSpace(strings.Trim(strings.TrimSpace(line), "*"))
	rest := ""
	if m := historicalTail.FindStringSubmatch(line); m != nil {
		rest, line = m[1], line[:len(line)-len(m[0])]
	}
	got := letters(line)
	for _, want := range []string{"historicalnote", "notehistorique"} {
		if oneOff(got, want) {
			return rest, true
		}
	}
	return "", false
}

// historicalTail is the parenthetical a head is sometimes set with, and the full
// stop after it.
var historicalTail = regexp.MustCompile(`\s*(\([^()]*\))\s*\.?\s*$`)

// letters is a line with everything that is not a letter thrown away, folded to
// lower case.
func letters(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) {
			b.WriteRune(unicode.ToLower(r))
		}
	}
	return b.String()
}

// oneOff says whether one word can be turned into another by changing, dropping
// or adding a single letter. One is as far as this goes: the head is fourteen
// letters and a second wrong letter is a page that needs reading again rather
// than a page this can speak for.
func oneOff(got, want string) bool {
	switch {
	case got == want:
		return true
	case len(got) == len(want):
		return apart(got, want)
	case len(got) == len(want)+1:
		return dropped(got, want)
	case len(got)+1 == len(want):
		return dropped(want, got)
	}
	return false
}

// apart says whether two words of a length differ in one letter.
func apart(a, b string) bool {
	off := 0
	for i := range a {
		if a[i] != b[i] {
			off++
		}
	}
	return off == 1
}

// dropped says whether the longer word is the shorter one with a letter put into
// it.
func dropped(long, short string) bool {
	for i := 0; i < len(short); i++ {
		if long[i] != short[i] {
			return long[i+1:] == short[i:]
		}
	}
	return true
}

// witnessSection is lostSection with the section sign read the way an old scan
// reads it and the spacing left open.
//
// Every character in the sign class is one this corpus was found printing for a
// section sign, and none of them is a guess: page 102 of Algebre I a III has
// "5 8. ANNEAUX", page 340 has "$11. MODULES ET ANNEAUX GRADUÉS", page 40 of
// Topologie generale V a X has "$ 3 . ESPACES PROJECTIFS RÉELS" with the space
// on both sides of the stop, and page 531 of Algebra I to III has the sign
// itself. The sign is thrown away either way, since the caller writes the
// heading rather than carrying this line through, and it is admitted only so
// that the number after it can be read.
//
// A § whose number really is 5 is not swallowed by the class. "5. ANNEAUX" has
// nothing after the five for the number to match, so the sign goes unmatched and
// the five is the number, which is what the leftmost-first rule gives and what
// the test holds it to.
var witnessSection = regexp.MustCompile(`^(?:\*\*)?(?:[§$5SsG9]\s*)?([0-9IlO]{1,4})\s*\.\s*(.+?)\s*$`)
