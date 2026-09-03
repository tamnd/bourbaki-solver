package toc

import (
	"fmt"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The titles in manifests/toc/ are read off the volume's own contents page,
// and for a scanned volume that page is OCR nobody should trust. Thirteen
// titles of the Theory of Sets came out as "Met?ods .of proof", "ConjunctIon",
// "Complement ofa set" and the like, and were corrected against the printed
// contents. Running the build again put all thirteen back, and said nothing
// about it: the summary line read "4 chapters, 22 §, 132 no." both times.
//
// So a rebuild keeps the title that is already there. What the volume now reads
// is printed beside what the manifest holds, and the count of titles kept goes
// in the summary, which is the part that was missing. Taking the new readings
// is a flag, because the case for it is real, a source that has been re-read
// and is better than it was, and it should be a thing somebody asks for rather
// than a thing that happens.
//
// Pages are not kept, with one exception. A page is a number the reader either
// got or did not, it is checked against the body by toc verify, and a page that
// changed is a page map or an erratum doing its job. It is the titles that get
// corrected by hand and the titles that got silently undone.
//
// The exception is zero, which is not a page that changed but a page the reader
// failed to get. hist § 19 is the case: it is printed on page 203, the page map
// runs pdf 204 to printed 202 and pdf 205 to printed 204, and printed 203 is in
// the step between them, so the rebuild resolves it to nothing and writes 0
// over the 205 that was there. Nobody gains a thing by that. Zero carries no
// information the previous number did not, it is not a correction and it is not
// a reading, and toc verify then has one fewer heading it can check. So a page
// the rebuild could not resolve keeps whatever the manifest had, and a page it
// did resolve replaces it however much it differs.

// Retitle is one title in a rebuilt contents that differs from the title the
// manifest already carries.
type Retitle struct {
	// Where names the entry the way the book cites it, "chapter I § 3 no. 2".
	Where string
	Was   string
	Now   string
}

func (r Retitle) String() string {
	return fmt.Sprintf("%s: %q, the volume now reads %q", r.Where, r.Was, r.Now)
}

// KeepTitles returns the rebuilt chapters with every title the manifest already
// carries put back, and says which ones it put back.
//
// Entries are matched by where they sit and not by what they say: a chapter by
// its numeral, a § by its number and whether it is an appendix, a no. by its
// number. An entry the manifest does not have is new and keeps what the volume
// reads, and is not reported, since there is nothing there to have corrected.
//
// The nominal flag is kept for the same reason the titles are. It says the
// chapter is the manifest's own and not the printing's, which is a thing about
// the volume that its contents page cannot say, so the build has no way to
// derive it and a rebuild that dropped it would refuse the volume for the
// absence of a chapter heading the printing never set. It is not reported,
// because unlike a title it is not a reading that could have gone either way.
func KeepTitles(old, fresh []corpus.Chapter) ([]corpus.Chapter, []Retitle) {
	var kept []Retitle
	out := make([]corpus.Chapter, len(fresh))
	byNumeral := map[string]*corpus.Chapter{}
	for i := range old {
		byNumeral[old[i].Numeral] = &old[i]
	}
	for i, chapter := range fresh {
		was, ok := byNumeral[chapter.Numeral]
		if !ok {
			out[i] = chapter
			continue
		}
		where := "chapter " + chapter.Numeral
		chapter.Nominal = chapter.Nominal || was.Nominal
		chapter.Page = keepPage(was.Page, chapter.Page)
		chapter.PDFPage = keepPage(was.PDFPage, chapter.PDFPage)
		chapter.Title, kept = keep(where, was.Title, chapter.Title, kept)
		chapter.Sections, kept = keepSections(where, was.Sections, chapter.Sections, kept)
		chapter.Subsections, kept = keepSubsections(where, was.Subsections, chapter.Subsections, kept)
		out[i] = chapter
	}
	return out, kept
}

func keepSections(where string, old, fresh []corpus.Section, kept []Retitle) ([]corpus.Section, []Retitle) {
	if len(fresh) == 0 {
		return fresh, kept
	}
	type key struct {
		number   int
		appendix bool
	}
	byNumber := map[key]*corpus.Section{}
	for i := range old {
		byNumber[key{old[i].Number, old[i].Appendix}] = &old[i]
	}
	out := make([]corpus.Section, len(fresh))
	for i, section := range fresh {
		was, ok := byNumber[key{section.Number, section.Appendix}]
		if !ok {
			out[i] = section
			continue
		}
		name := fmt.Sprintf("%s § %d", where, section.Number)
		if section.Appendix {
			name = fmt.Sprintf("%s appendix %d", where, section.Number)
		}
		section.Title, kept = keep(name, was.Title, section.Title, kept)
		section.Page = keepPage(was.Page, section.Page)
		section.PDFPage = keepPage(was.PDFPage, section.PDFPage)
		section.Subsections, kept = keepSubsections(name, was.Subsections, section.Subsections, kept)
		out[i] = section
	}
	return out, kept
}

func keepSubsections(where string, old, fresh []corpus.Subsection, kept []Retitle) ([]corpus.Subsection, []Retitle) {
	if len(fresh) == 0 {
		return fresh, kept
	}
	byNumber := map[int]*corpus.Subsection{}
	for i := range old {
		byNumber[old[i].Number] = &old[i]
	}
	out := make([]corpus.Subsection, len(fresh))
	for i, sub := range fresh {
		was, ok := byNumber[sub.Number]
		if !ok {
			out[i] = sub
			continue
		}
		sub.Title, kept = keep(fmt.Sprintf("%s no. %d", where, sub.Number), was.Title, sub.Title, kept)
		sub.Page = keepPage(was.Page, sub.Page)
		sub.PDFPage = keepPage(was.PDFPage, sub.PDFPage)
		out[i] = sub
	}
	return out, kept
}

// keep is the decision itself. An entry the manifest has no title for is one
// nobody has corrected, so the volume's reading is taken and nothing is
// reported: an empty title is the absence of a correction and not a correction
// to the empty string.
func keep(where, was, now string, kept []Retitle) (string, []Retitle) {
	if was == "" || was == now {
		return now, kept
	}
	return was, append(kept, Retitle{Where: where, Was: was, Now: now})
}

// keepPage holds on to a page the rebuild could not resolve. Zero is the
// reader coming back with nothing rather than with a different answer, and
// writing it over a number that was there loses a heading toc verify could
// otherwise check. Any number the rebuild did get is taken, however far it is
// from the one before it: that is the page map or an erratum doing its job.
func keepPage(was, now int) int {
	if now == 0 {
		return was
	}
	return now
}
