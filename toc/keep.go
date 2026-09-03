package toc

import (
	"fmt"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pagemap"
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
// The printed page on a line is kept for the same reason as the title on it,
// because it is the same reading off the same OCR of the same line. hist-fr is
// the case. Its contents page prints "La fonction gamma ... 253", and the
// heading is on pdf 253, which the page map, uniform at printed = pdf + 2 over
// 117 pages whose folio was actually read, puts at printed 255. Four more
// entries of the same volume are wrong the same way. The manifest had all five
// corrected and verify was 27 of 27; a rebuild put all five back the way the
// scan had them and verify fell to 25, with two headings reported as printed on
// a page the contents does not name. That is the exact failure this file was
// written for, arriving through the number rather than through the title.
//
// The pdf page is not kept, because it is not a reading. It is the printed page
// carried through the page map, so it is re-resolved from the map whenever a
// printed page is kept, and a page map corrected tomorrow still moves every pdf
// page it should: that was the reason pages were not kept at all, and it
// survives intact. Only when the map cannot resolve the printed page does the
// number the manifest had stand, and that is the other half of this.
//
// hist § 19 is that other half. It is printed on page 203, the page map runs
// pdf 204 to printed 202 and pdf 205 to printed 204, and printed 203 is the
// missing leaf in the step between them, so the rebuild resolves it to nothing
// and would write 0 over the 205 that was there. Zero is not a page that
// changed, it is a page the reader failed to get. It carries no information the
// previous number did not, it is not a correction and it is not a reading, and
// toc verify then has one fewer heading it can check. So zero alone is held
// against, and any number the map did resolve is taken however far it is from
// the one before it.

// Retitle is one reading in a rebuilt contents that differs from the reading
// the manifest already carries.
type Retitle struct {
	// Where names the entry the way the book cites it, "chapter I § 3 no. 2".
	Where string
	// What the reading is, "title" or "printed page". Both come off the same
	// line of the same contents page and both are kept, and the two are told
	// apart here so that what is printed about them can say which it was.
	What string
	Was  string
	Now  string
}

func (r Retitle) String() string {
	if r.What == "" || r.What == "title" {
		return fmt.Sprintf("%s: %q, the volume now reads %q", r.Where, r.Was, r.Now)
	}
	return fmt.Sprintf("%s %s %s, the volume now reads %s", r.Where, r.What, r.Was, r.Now)
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
func KeepTitles(old, fresh []corpus.Chapter, pm *pagemap.Map) ([]corpus.Chapter, []Retitle) {
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
		chapter.Title, kept = keep(where, was.Title, chapter.Title, kept)
		chapter.Page, chapter.PDFPage, kept = keepPage(where, was.Page, was.PDFPage, chapter.Page, chapter.PDFPage, pm, kept)
		chapter.Sections, kept = keepSections(where, chapter.Numeral, pm, was.Sections, chapter.Sections, kept)
		chapter.Subsections, kept = keepSubsections(where, chapter.Numeral, pm, was.Subsections, chapter.Subsections, kept)
		out[i] = chapter
	}
	return out, kept
}

func keepSections(where, numeral string, pm *pagemap.Map, old, fresh []corpus.Section, kept []Retitle) ([]corpus.Section, []Retitle) {
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
		section.Page, section.PDFPage, kept = keepPage(name, was.Page, was.PDFPage, section.Page, section.PDFPage, pm, kept)
		section.Subsections, kept = keepSubsections(name, numeral, pm, was.Subsections, section.Subsections, kept)
		out[i] = section
	}
	return out, kept
}

func keepSubsections(where, numeral string, pm *pagemap.Map, old, fresh []corpus.Subsection, kept []Retitle) ([]corpus.Subsection, []Retitle) {
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
		name := fmt.Sprintf("%s no. %d", where, sub.Number)
		sub.Title, kept = keep(name, was.Title, sub.Title, kept)
		sub.Page, sub.PDFPage, kept = keepPage(name, was.Page, was.PDFPage, sub.Page, sub.PDFPage, pm, kept)
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

// keepPage decides one entry's pair of numbers, and reports a printed page it
// kept. It takes the four numbers rather than the entries because a chapter, a
// section and a subsection are three unrelated structs in corpus that happen to
// carry the same two fields, and one rule covers all three. See the file
// comment for both halves of the rule and for the volumes that produced them.
func keepPage(where string, wasPage, wasPDF, page, pdf int, pm *pagemap.Map, kept []Retitle) (int, int, []Retitle) {
	switch {
	case page == 0:
		// The rebuild came back with nothing. Both numbers stand as they were,
		// because there is no printed page here to carry through the map.
		return wasPage, wasPDF, kept
	case wasPage == 0 || wasPage == page:
		// Nothing to have corrected, or the rebuild agrees. The one case where
		// the pdf page is still worth holding is a printed page the map cannot
		// place, which is hist § 19.
		if pdf == 0 {
			pdf = wasPDF
		}
		return page, pdf, kept
	}
	// The readings differ, so the manifest's stands and is reported, and its
	// pdf page is re-resolved rather than restored: the map is the current one
	// and this is the number that has to follow it.
	pdf, ok := pm.PDFPageOf("", wasPage)
	if !ok {
		pdf = wasPDF
	}
	return wasPage, pdf, append(kept, Retitle{Where: where, What: "printed page",
		Was: fmt.Sprint(wasPage), Now: fmt.Sprint(page)})
}
