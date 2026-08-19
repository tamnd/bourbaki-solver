package pagemap

import "fmt"

// Problem is something Validate could not reconcile. A map that passes with no
// problems is not proof that every number is right, but every one of these
// checks failed at least once while the fitter was being written, so they are
// not decoration either.
type Problem struct {
	PDFPage int    `json:"pdf_page,omitempty"`
	Chapter string `json:"chapter,omitempty"`
	Detail  string `json:"detail"`
}

func (p Problem) String() string {
	if p.PDFPage > 0 {
		return fmt.Sprintf("pdf %d: %s", p.PDFPage, p.Detail)
	}
	if p.Chapter != "" {
		return fmt.Sprintf("chapter %s: %s", p.Chapter, p.Detail)
	}
	return p.Detail
}

// Validate checks the map against itself. The point is that a printed page
// sequence is highly constrained, so most ways of getting it wrong show up as
// an arithmetic inconsistency rather than as a plausible wrong number.
func (m *Map) Validate() []Problem {
	var probs []Problem

	// A fit that put every page of a volume in the front matter found nothing.
	// It is worth saying because of how it comes out otherwise: none of the
	// checks below has an entry to fail on, so an empty map passes them all and
	// gets written, and from then on the volume counts as mapped. Nothing
	// downstream then treats it as work still to do. The Algebre chapitre 10
	// scan does exactly this, 222 pages of which 57 have been read and none
	// carry a number the fitter could use.
	if m.BodyPages() == 0 && len(m.Entries) > 0 {
		probs = append(probs, Problem{
			Detail: fmt.Sprintf("none of the %d pages was mapped, so the fit found nothing",
				len(m.Entries))})
	}

	stepAt := map[int]Step{}
	for _, s := range m.Steps {
		stepAt[s.AtPDFPage] = s
	}
	restartAt := map[int]bool{}
	for _, r := range m.Restarts {
		if r < 2 || r > len(m.Entries) {
			probs = append(probs, Problem{PDFPage: r,
				Detail: fmt.Sprintf("a restart is declared here, outside the %d pages of the volume",
					len(m.Entries))})
			continue
		}
		restartAt[r] = true
	}

	for i, e := range m.Entries {
		if e.PDFPage != i+1 {
			probs = append(probs, Problem{PDFPage: e.PDFPage,
				Detail: fmt.Sprintf("entry %d is out of order", i)})
		}
		if e.Confidence == Unknown && e.Page != 0 {
			probs = append(probs, Problem{PDFPage: e.PDFPage,
				Detail: "an unmapped page carries a page number"})
		}
		if e.Confidence != Unknown && e.Page <= 0 {
			probs = append(probs, Problem{PDFPage: e.PDFPage,
				Detail: "a mapped page carries no page number"})
		}
		if i == 0 {
			continue
		}
		prev := m.Entries[i-1]
		if prev.Page == 0 || e.Page == 0 || prev.Chapter != e.Chapter {
			continue
		}
		// A declared restart is where one fascicule ends and the next begins,
		// and the printed number goes back to the front of the new one. There
		// is no arithmetic to check it against, so what is checked is that it
		// goes back at all: a restart that runs on is one written against the
		// wrong page, and the page it was meant for is the one the fit gets
		// wrong.
		if restartAt[e.PDFPage] {
			if e.Page >= prev.Page {
				probs = append(probs, Problem{PDFPage: e.PDFPage,
					Detail: fmt.Sprintf("a restart is declared here, but page %d follows page %d and does not start over",
						e.Page, prev.Page)})
			}
			continue
		}
		// Printed pages advance by one, and the only licensed exception is a
		// leaf the file does not have, which has to be recorded as a step.
		want := prev.Page + 1
		if s, ok := stepAt[e.PDFPage]; ok {
			want = prev.Page + 1 + len(s.MissingPages)
		}
		if e.Page != want {
			probs = append(probs, Problem{PDFPage: e.PDFPage,
				Detail: fmt.Sprintf("page %d follows page %d with no step recorded", e.Page, prev.Page)})
		}
	}

	for _, sp := range m.Chapters {
		// A per-chapter volume numbers every chapter from 1. The one chapter
		// that can honestly start above it is the first, and only where the
		// scan does not have the front of the volume, which the volume has to
		// say in its manifest because nothing in the file gives it away.
		want := 1
		if sp.FirstPDF == 1 && m.FirstPage != 0 {
			want = m.FirstPage
		}
		if m.Pagination == PerChapter && sp.FirstPage != want {
			probs = append(probs, Problem{Chapter: sp.Chapter, PDFPage: sp.FirstPDF,
				Detail: fmt.Sprintf("chapter starts at printed page %d, not %d", sp.FirstPage, want)})
		}
		missing := 0
		for _, s := range m.Steps {
			if s.Chapter == sp.Chapter {
				missing += len(s.MissingPages)
			}
		}
		pdfPages := sp.LastPDF - sp.FirstPDF + 1
		printed := sp.LastPage - sp.FirstPage + 1
		if printed != pdfPages+missing {
			probs = append(probs, Problem{Chapter: sp.Chapter,
				Detail: fmt.Sprintf("%d printed pages over %d pdf pages with %d recorded missing",
					printed, pdfPages, missing)})
		}
	}

	// A conflict is only safe to overrule when the pages on both sides were
	// read cleanly and bracket the fitted number. Anything else means the
	// fitter, not the scan, may be the one that is wrong.
	for _, c := range m.Conflicts {
		before, okB := m.Lookup(c.PDFPage - 1)
		after, okA := m.Lookup(c.PDFPage + 1)
		ok := okB && okA &&
			before.Confidence.Printed() && after.Confidence.Printed() &&
			before.Page == c.Fitted-1 && after.Page == c.Fitted+1 &&
			before.Chapter == c.Chapter && after.Chapter == c.Chapter
		if !ok {
			read, fitted := c.Pages()
			probs = append(probs, Problem{PDFPage: c.PDFPage,
				Detail: fmt.Sprintf("reading %s was overruled by %s without both neighbours confirming it",
					read, fitted)})
		}
	}
	return probs
}
