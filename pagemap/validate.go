package pagemap

import (
	"fmt"
	"strings"
)

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

// MinReadShare and MaxUnreadShare are what a fit has to be standing on before
// it counts as a fit. The first is the share of body pages whose number was
// read off the page; the second caps the longest single run of pages carried by
// arithmetic alone, as a share of the body.
//
// Both numbers were picked from the corpus rather than chosen. Over the 44 maps
// the share read runs 0.5 per cent for top-v-x and then nothing until 15.3 for
// lie-i-fr and var-fr, and up from there, so a floor anywhere in that gap costs
// nothing and 5 is the round number in the middle of it. The longest unread run
// is 73 per cent of the body for top-v-x and then nothing until 26.5 for
// alg-ix-fr, 24.3 for lie-i-fr and 12 for var-fr, so half the volume is again
// well clear of everything the corpus actually has.
//
// They are deliberately not tight. This is the check that stops a map being
// published that is not a fit at all, not a measure of how good a fit is; the
// coverage table is where a map that is thin but honest gets reported.
// MinBodyPages is the size below which neither share is asked, because a share
// over a handful of pages is not a measurement: two unread pages out of three
// is 67 per cent and says nothing at all. The smallest volume in the corpus is
// the 61 body pages of lie-vii-viii-fr, a fascicule, so this is well under
// anything real and well over the fixtures that exercise the other checks.
const (
	MinReadShare   = 0.05
	MaxUnreadShare = 0.50
	MinBodyPages   = 20
)

// standing refuses a map that no reading supports, which Validate cannot
// otherwise catch because there is nothing wrong with it.
//
// Every other check here is for a map that contradicts itself. One anchor
// extrapolated across a volume contradicts nothing: top-v-x is 372 pages,
// foot-number, with no text layer, and the reading kept the foot number on one
// page of them. The offset from that one page carried across the other 365
// validates, is written, and from then on the volume reads as mapped, the
// coverage table counts its pages, and every page the fitter got wrong is
// quoted back by everything that asks. A volume that steps anywhere at all,
// which most of them do, is then wrong from the step to the end with nothing
// saying so.
//
// So the refusal is not that the map is inconsistent, it is that it is not
// evidence. The file was being left out of the corpus by hand, which is not a
// thing that should need doing by hand.
func (m *Map) standing() []Problem {
	body := m.BodyPages()
	if body < MinBodyPages {
		return nil // The empty fit has its own problem, above.
	}
	var probs []Problem
	read := m.ReadPages()
	if share := float64(read) / float64(body); share < MinReadShare {
		probs = append(probs, Problem{
			Detail: fmt.Sprintf("%d of %d body pages carry a number that was read, %.1f%%, under the %.0f%% a fit is held to: this is one reading extrapolated and not a fit",
				read, body, share*100, MinReadShare*100)})
	}
	if run, from, to := m.LongestUnread(); float64(run) > MaxUnreadShare*float64(body) {
		probs = append(probs, Problem{PDFPage: from,
			Detail: fmt.Sprintf("pdf %d to %d, %d pages, carry no number that was read, over the %.0f%% of the volume a single stretch is held to",
				from, to, run, MaxUnreadShare*100)})
	}
	return probs
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

	probs = append(probs, m.standing()...)

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
	}

	// The printed numbers run in sequence in the order the volume reads, which
	// is the file's order except where the binder got two leaves the wrong way
	// round. A transposition the volume did not declare shows up here as two
	// pages that swap places, which is what it is; one it declared and does not
	// have shows up the same way, because the check is run over the pages as
	// declared and the declaration is what put them out of sequence.
	order, err := printingOrder(len(m.Entries), m.Transposed)
	if err != nil {
		return append(probs, Problem{Detail: strings.TrimPrefix(err.Error(), "pagemap: ")})
	}
	for i, p := range order {
		e := m.Entries[p-1]
		if i == 0 {
			continue
		}
		prev := m.Entries[order[i-1]-1]
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
		// A leaf inside the chapter that the book never numbered is the mirror
		// of a printed page the file does not have, and it has to come off the
		// count for the same reason the missing ones go on. The bibliography
		// leaf between IV.89 and IV.90 of the French Topologie generale is one:
		// the file has 96 leaves there and the printing numbered 95 of them,
		// which is not an error in the fit and reads as one without this.
		unnumbered := 0
		for _, e := range m.Entries {
			if e.PDFPage >= sp.FirstPDF && e.PDFPage <= sp.LastPDF && e.Page == 0 {
				unnumbered++
			}
		}
		pdfPages := sp.LastPDF - sp.FirstPDF + 1
		printed := sp.LastPage - sp.FirstPage + 1
		if printed != pdfPages+missing-unnumbered {
			probs = append(probs, Problem{Chapter: sp.Chapter,
				Detail: fmt.Sprintf("%d printed pages over %d pdf pages with %d recorded missing and %d unnumbered",
					printed, pdfPages, missing, unnumbered)})
		}
	}

	// A conflict is only safe to overrule when the pages on both sides were
	// read cleanly and bracket the fitted number. Anything else means the
	// fitter, not the scan, may be the one that is wrong.
	//
	// The pages on both sides are the ones the volume reads either side of it,
	// which is not the file's own neighbours where two leaves are the wrong way
	// round.
	at := make([]int, len(m.Entries)+1)
	for i, p := range order {
		at[p] = i
	}
	nextTo := func(pdfPage, dir int) (Entry, bool) {
		if pdfPage < 1 || pdfPage > len(m.Entries) {
			return Entry{}, false
		}
		i := at[pdfPage] + dir
		if i < 0 || i >= len(order) {
			return Entry{}, false
		}
		return m.Entries[order[i]-1], true
	}
	for _, c := range m.Conflicts {
		before, okB := nextTo(c.PDFPage, -1)
		after, okA := nextTo(c.PDFPage, +1)
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
