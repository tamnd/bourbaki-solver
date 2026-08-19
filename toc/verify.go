package toc

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// Verify opens every page the contents points at and looks for the heading that
// is supposed to be printed there.
//
// The parser and the page map can both be self-consistent and still be wrong
// together, so the only check worth much is against the body of the book. A
// heading is matched on its words rather than on the whole string, because the
// contents and the body are two separate pieces of OCR of two separate pieces
// of type: chapter II no. 14 is "Multimodules" in the contents and
// "MULTJMODULES" in the body, and that is a match, not a miss.
//
// When a heading is not on the page the contents named, the pages around it are
// searched too. That is what separates the two failures worth telling apart: a
// heading found three pages later means the page map or a digit is wrong, a
// heading found nowhere means the scan of one of the two lines is too damaged
// to read. Only the first kind is a defect in the corpus.

// Check is one heading looked for and not found where it was expected.
type Check struct {
	Chapter string
	Section int
	Kind    string // "chapter", "§", "no.", "exercises", "historical note"
	Title   string
	PDFPage int
	Found   []int // pages nearby that do carry the heading, if any
}

func (c Check) String() string {
	where := "nowhere near it"
	if len(c.Found) > 0 {
		where = fmt.Sprintf("found on pdf %v", c.Found)
	}
	title := c.Title
	if title == "" {
		title = c.Kind
	}
	return fmt.Sprintf("%s %s %q: not on pdf %d, %s", c.Chapter, c.Kind, title, c.PDFPage, where)
}

// Report is the result of verifying one volume.
type Report struct {
	Book    string
	Checked int
	Matched int
	Misses  []Check
}

// Moved are the misses that were found on another page, which are the ones that
// mean something is wrong with the corpus rather than with the scan.
func (r *Report) Moved() []Check {
	var out []Check
	for _, c := range r.Misses {
		if len(c.Found) > 0 {
			out = append(out, c)
		}
	}
	return out
}

// Rate is the share of headings found where the contents said they were.
func (r *Report) Rate() float64 {
	if r.Checked == 0 {
		return 0
	}
	return 100 * float64(r.Matched) / float64(r.Checked)
}

// near is how far either side of the named page a missing heading is looked
// for. Three pages is more than any real off-by-one and short enough that a
// heading found that far away is not a coincidence.
const near = 3

var wordRe = regexp.MustCompile(`[a-z]{4,}`)

// Verify checks one volume's contents against its pages.
func Verify(pages []string, b corpus.BookTOC) *Report {
	norm := make([]string, len(pages))
	for i, p := range pages {
		norm[i] = normalize(p)
	}
	r := &Report{Book: b.ID}
	for _, c := range b.Chapters {
		r.check(norm, Check{Chapter: c.Numeral, Kind: "chapter", Title: c.Title, PDFPage: c.PDFPage})
		// The nos of a chapter that prints no § are checked the same way, and
		// they carry no § number because there is none to carry.
		for _, sub := range c.Subsections {
			r.check(norm, Check{Chapter: c.Numeral, Kind: "no.",
				Title: sub.Title, PDFPage: sub.PDFPage})
		}
		for _, s := range c.Sections {
			kind := "§"
			if s.Appendix {
				kind = "appendix"
			}
			r.check(norm, Check{Chapter: c.Numeral, Section: s.Number, Kind: kind,
				Title: s.Title, PDFPage: s.PDFPage})
			for _, sub := range s.Subsections {
				r.check(norm, Check{Chapter: c.Numeral, Section: s.Number, Kind: "no.",
					Title: sub.Title, PDFPage: sub.PDFPage})
			}
			if s.Exercises != nil {
				r.exercises(pages, Check{Chapter: c.Numeral, Section: s.Number, Kind: "exercises",
					PDFPage: s.Exercises.PDFPage}, s.Appendix)
			}
		}
		if c.Exercises != nil {
			r.exercises(pages, Check{Chapter: c.Numeral, Kind: "exercises",
				PDFPage: c.Exercises.PDFPage}, false)
		}
		if c.Historical != nil {
			r.check(norm, Check{Chapter: c.Numeral, Kind: "historical note",
				Title: "historical note", PDFPage: c.Historical.PDFPage})
		}
	}
	return r
}

// exercises checks a page that a run of exercises is supposed to start on.
//
// The word is matched on its stem because the library is printed in two
// languages and the French sets EXERCICES where the English sets EXERCISES.
//
// The word EXERCISES is not the marker to look for. The 2023 volume prints it
// at the head of each §'s run, but the 1998 and 2003 volumes gather every run
// at the end of the chapter and separate them with a line carrying nothing but
// "§ 11", or the word APPENDIX for the run that belongs to the appendix. A page
// in the middle of such a run carries none of the three, which is why the run
// has to be looked for by its own marker rather than by a word.
func (r *Report) exercises(pages []string, c Check, appendix bool) {
	r.Checked++
	if exercisesOn(pages, c.PDFPage, c.Section, appendix) {
		r.Matched++
		return
	}
	for p := c.PDFPage - near; p <= c.PDFPage+near; p++ {
		if p != c.PDFPage && exercisesStart(pages, p, c.Section, appendix, p < c.PDFPage) {
			c.Found = append(c.Found, p)
		}
	}
	c.Title = fmt.Sprintf("exercises for § %d", c.Section)
	switch {
	case appendix:
		c.Title = "exercises for the appendix"
	case c.Section == 0:
		c.Title = "exercises for the chapter"
	}
	r.Misses = append(r.Misses, c)
}

func exercisesOn(pages []string, page, section int, appendix bool) bool {
	if page < 1 || page > len(pages) {
		return false
	}
	for _, l := range strings.Split(pages[page-1], "\n") {
		l = strings.TrimSpace(l)
		if strings.Contains(strings.ToLower(l), "exerci") {
			return true
		}
		if appendix && appendixMarkRe.MatchString(l) {
			return true
		}
		if m := runMarkRe.FindStringSubmatch(l); m != nil {
			if n, ok := readNumber(runFixer.Replace(m[1])); ok && n == section {
				return true
			}
		}
	}
	return false
}

// exercisesStart is the same question asked of a page the contents did not
// name, and it is a harder question than exercisesOn answers.
//
// The word EXERCISES is printed in the running head of every page of a run and
// not only the first, so a page three along from the one the contents named
// carries it too, and taking that for evidence says the run starts on four
// consecutive pages. That is what the search either side of a miss was doing:
// all six of the misses in the library were reported as printed on another
// page, and every one of the pages offered was the middle of the same run,
// found on the head alone. The marker the contents named is not on another
// page in any of the six. It was eaten by the scan, which is a damaged page
// and not a wrong page, and only the second is a defect in the corpus.
//
// So a running head counts here only where it names a §, which the 1998 and
// 2003 printings set to the left of the word: a head reading § 3 EXERCISES on
// the page before the one the contents gave for § 3 does say the corpus has
// the page wrong. Where the head names no § there is nothing to go on, and a
// bare word is not offered as a place the run might have started.
//
// Only a page before the named one is heard on its head. A head naming § 3 on
// the page after is what a run that starts exactly where the contents says it
// does looks like on its second page, so offering it as somewhere else the run
// might have started says nothing: page 256 of the Lie volume heads § 4 and
// page 255 is where § 4 begins.
//
// The run mark is heard on either side, since a bare § on a line of its own is
// where a run starts and nowhere else.
func exercisesStart(pages []string, page, section int, appendix, head bool) bool {
	if page < 1 || page > len(pages) {
		return false
	}
	for _, l := range strings.Split(pages[page-1], "\n") {
		l = strings.TrimSpace(l)
		if appendix && appendixMarkRe.MatchString(l) {
			return true
		}
		if m := runMarkRe.FindStringSubmatch(l); m != nil {
			if n, ok := readNumber(runFixer.Replace(m[1])); ok && n == section {
				return true
			}
		}
	}
	if !head {
		return false
	}
	n, ok := headSection(firstLine(pages[page-1]))
	return ok && n == section
}

// firstLine is the running head, where a page has one.
func firstLine(page string) string {
	for _, l := range strings.Split(page, "\n") {
		if l = strings.TrimSpace(l); l != "" {
			return l
		}
	}
	return ""
}

// headSection reads the § a running head names out of whatever the printing
// set to the left of the word, which is where all three of the grammars put
// it. The § sign itself is not looked for: the 2003 scan of the Lie volume
// hands its heads back as "~ 1." and "s3.", so the sign is whatever the scan
// made of it and the digit beside it is the only part worth reading.
//
// The head and not the page. Page 252 of the Lie volume carries a footnote
// opening "2 This exercise, hitherto unpublished", and read anywhere on the
// page that line says the page heads the exercises for § 2, which it does not.
func headSection(line string) (int, bool) {
	i := strings.Index(strings.ToLower(line), "exerci")
	if i < 0 {
		return 0, false
	}
	m := headNumRe.FindString(line[:i])
	if m == "" {
		return 0, false
	}
	return readNumber(m)
}

var headNumRe = regexp.MustCompile(`[0-9]{1,2}`)

// runMarkRe is a line that carries a § and nothing else, which is how the
// gathered exercises are cut into runs. The 2003 scan reads the marker that
// opens the exercises for § 5 of chapter VII as "§S", and a letter standing
// where a § number belongs can only be a misread digit, so a few of them are
// put back. This is done here and not in the parser because a § number in the
// contents sits in a line of words where the same substitution would do harm.
var (
	runMarkRe = regexp.MustCompile(`^§\s*([0-9IlOSZ|]{1,2})\.?$`)
	runFixer  = strings.NewReplacer("S", "5", "Z", "2")

	// The marker that opens an appendix's run of exercises carries the
	// appendix's numeral where the chapter has more than one: the English Lie
	// volume sets "Appendix I" on printed page 66 and "Appendix II" on 67.
	appendixMarkRe = regexp.MustCompile(`(?i)^appendi[xc]e?\s*[0-9IVXL]{0,4}\.?$`)
)

// check looks for one heading and records it.
func (r *Report) check(norm []string, c Check) {
	words := wordRe.FindAllString(strings.ToLower(c.Title), -1)
	if len(words) == 0 {
		return
	}
	r.Checked++
	if onPage(norm, c.PDFPage, words) {
		r.Matched++
		return
	}
	for p := c.PDFPage - near; p <= c.PDFPage+near; p++ {
		if p != c.PDFPage && onPage(norm, p, words) {
			c.Found = append(c.Found, p)
		}
	}
	r.Misses = append(r.Misses, c)
}

// onPage reports whether half the words of a heading are on a page. Half is the
// threshold because a scan drops or mangles a letter here and there and a whole
// word with it, but it does not mangle half a line.
func onPage(norm []string, page int, words []string) bool {
	if page < 1 || page > len(norm) {
		return false
	}
	got := 0
	for _, w := range words {
		if strings.Contains(norm[page-1], w) || nearWord(norm[page-1], w) {
			got++
		}
	}
	return got*2 >= len(words)
}

// nearWord reports whether a page carries a word with one letter changed. What
// the scan does to a heading is swap a letter for a lookalike, not rewrite it:
// no. 14 of chapter II is "Multimodules" in the contents and "MULTJMODULES" in
// the body, which is the same heading and has to count as one. Only words long
// enough for a single change to leave no doubt are treated this way.
func nearWord(page, w string) bool {
	if len(w) < 6 {
		return false
	}
	for i := 0; i+len(w) <= len(page); i++ {
		diff := 0
		for j := 0; j < len(w) && diff < 2; j++ {
			if page[i+j] != w[j] {
				diff++
			}
		}
		if diff < 2 {
			return true
		}
	}
	return false
}

// normalize strips a page down to lowercase letters and digits, which throws
// away the line breaks, the leader dots and the spacing that the two pieces of
// OCR disagree about and keeps what they agree on.
func normalize(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	for _, r := range strings.ToLower(s) {
		if (r >= 'a' && r <= 'z') || (r >= '0' && r <= '9') {
			b.WriteRune(r)
		}
	}
	return b.String()
}
