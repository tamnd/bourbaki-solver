package toc

import (
	"fmt"

	"github.com/tamnd/bourbaki-solver/pagemap"
)

// validate checks the contents against the page map and against itself.
//
// A table of contents is heavily constrained: chapters, § and no. are numbered
// from 1 without gaps, pages only move forward, no. 1 of a § starts on the same
// page as the § itself, and every page named has to be a page the volume has.
// Almost every way an OCR misread can corrupt an entry breaks one of those, so
// checking them is what makes it safe to repair digits at all.
func (r *Result) validate(pm *pagemap.Map, opt Options) []Problem {
	var probs []Problem
	add := func(ch string, sec int, format string, args ...any) {
		probs = append(probs, Problem{Chapter: ch, Section: sec,
			Detail: fmt.Sprintf(format, args...)})
	}
	// nopdf reports a contents entry that landed on no pdf page. There are two
	// ways to get there and they want different answers. A misread digit names
	// a page the volume never printed, and that is the contents being wrong.
	// A page the scan is short a leaf for is the contents being right about a
	// volume the file does not fully hold, and the map already knows which
	// pages those are, so it is asked rather than guessed at.
	//
	// what names the entry and is empty for a chapter or a §, which read
	// naturally without it. It is "no. 3" or "the exercises'" elsewhere.
	nopdf := func(ch string, sec int, what string, page int) {
		p := Problem{Chapter: ch, Section: sec}
		switch {
		case pm.MissingPage(ch, page):
			p.Soft = true
			p.Detail = fmt.Sprintf("%sprinted page %d is not in the scan, "+
				"which the page map records as a missing leaf", what, page)
		default:
			p.Detail = fmt.Sprintf("%sprinted page %d is on no pdf page", what, page)
		}
		probs = append(probs, p)
	}

	if len(r.Chapters) == 0 {
		add("", 0, "the contents yielded no chapters")
		return probs
	}

	want := map[string]pagemap.Span{}
	for _, sp := range pm.Chapters {
		want[sp.Chapter] = sp
	}
	// A fascicule is not a chapter, and two of the checks below have to know
	// which one they are looking at. The French Varietes is one scan holding
	// two fascicules de resultats: the manifest declares the restart at pdf 96,
	// the page map makes a nominal chapter of each run, and neither run is a
	// chapter of anything. The mark is the span opening on a declared restart,
	// which is what a fascicule is and what no chapter ever is.
	restart := map[int]bool{}
	for _, pdf := range opt.Restarts {
		restart[pdf] = true
	}
	fascicule := func(sp pagemap.Span) bool { return restart[sp.FirstPDF] }
	// A part the manifest has named is the other kind of fascicule: one bound
	// into the back of a volume rather than printed as a volume of its own. The
	// Summary of Results of Theory of Sets is the case. The printing sets no
	// chapter line over it, so the page map has no span for it and runs the
	// chapter in front of it straight through to the end of the file, and the
	// volume's chapter count stays the count of its chapters. What the checks
	// below want of it is everything the contents can answer on its own.
	part := map[string]bool{}
	for _, p := range opt.Parts {
		part[p.Numeral] = true
	}
	parts, last := 0, pagemap.Span{}
	for _, c := range r.Chapters {
		if part[c.Numeral] {
			parts++
		}
	}
	for _, sp := range pm.Chapters {
		if sp.LastPage > last.LastPage {
			last = sp
		}
	}
	// sections is how many § the chapters so far have listed, which is what a
	// fascicule's numbering carries on from. See the § check below.
	sections := 0
	if len(opt.Chapters) > 0 && len(r.Chapters)-parts != len(opt.Chapters) {
		add("", 0, "the contents lists %d chapters, the volume has %d",
			len(r.Chapters)-parts, len(opt.Chapters))
	}

	for _, c := range r.Chapters {
		sp, ok := want[c.Numeral]
		switch {
		case ok:
		case part[c.Numeral]:
			// The span a part is given is the one its own contents draws: it
			// opens where the contents says it opens and closes where the
			// volume does. That is not a second opinion to check the contents
			// against, and none is to be had, but it keeps the page checks
			// below asking a real question of the entries under it, which is
			// that they fall inside the part and inside the volume.
			sp = pagemap.Span{Chapter: c.Numeral, FirstPDF: c.PDFPage,
				FirstPage: c.Page, LastPDF: last.LastPDF, LastPage: last.LastPage}
		default:
			add(c.Numeral, 0, "the contents lists a chapter the page map never found")
			continue
		}
		// A span that opens on pdf page 1 is not evidence of where the chapter
		// opens. Nothing marks the end of the front matter, so the fit hands
		// every leaf before chapter II to chapter I, cover included, and the
		// span then starts on the first leaf of the file rather than on the
		// page the chapter is printed on.
		//
		// ac-i-iv-fr is the case. The page map runs chapter I from pdf 1 and
		// calls that printed page 3, and the contents says printed 13 at pdf
		// 11. The numbering is not in dispute: printed is pdf plus 2 on both
		// sides, and pdf 9, 10 and 12 carry that in a printed head. What is in
		// dispute is only where the chapter begins, and the contents is the one
		// that was told, while the map is filling in a boundary nothing gave
		// it. The map also reports 0 front matter for the volume, and every
		// other volume in the corpus reports between 7 and 29, which is the
		// same fact said another way.
		//
		// The test is on pdf page 1 rather than on the front matter count
		// because it needs no measurement to be right. A chapter of a bound
		// volume never opens on the first leaf of the file, that leaf is the
		// cover, so this gives up nothing anywhere it matters.
		//
		// The second test is the same fact about a volume that numbers its own
		// front matter inside chapter I. The French Theorie des ensembles does:
		// its note to the reader is printed E I.1 to E I.6 and its introduction
		// E I.7 to E I.13, so the page map reads real folios saying chapter I as
		// far back as pdf 9 and runs the span from pdf 2, one leaf after the
		// cover rather than on it. The numbering is not in dispute there either.
		// The contents says CHAPITRE I at E I.14 and lists the introduction
		// above it at E I.7, and both are right. Where the manifest has been
		// told which leaves are the note to the reader and the introduction, a
		// span reaching back into them is the map filling in a boundary nothing
		// gave it, exactly as a span reaching back to the cover is.
		//
		// The third is a fascicule, which carries its own front matter the way
		// a volume does. Fascicule 2 of the Varietes opens on printed page 6,
		// prints its notations and conventions on 7, and heads its first § on
		// 9, so the map and the contents are three pages apart and both are
		// right. A chapter cannot be in that position, because the leaves in
		// front of it belong to the chapter before; a fascicule's belong to the
		// fascicule they open. What is still asked of a fascicule is that its
		// contents does not start it before the fascicule itself starts, which
		// is a misread digit either way round.
		switch {
		case sp.FirstPDF == 1 || sp.FirstPDF <= opt.FrontMatterPDF:
		case fascicule(sp):
			if c.Page < sp.FirstPage {
				add(c.Numeral, 0, "the contents starts it at printed page %d, before the fascicule at %d",
					c.Page, sp.FirstPage)
			}
		case c.Page != sp.FirstPage:
			add(c.Numeral, 0, "the contents starts it at printed page %d, the page map at %d",
				c.Page, sp.FirstPage)
		}
		if c.PDFPage == 0 {
			nopdf(c.Numeral, 0, "", c.Page)
		}
		if c.Title == "" {
			add(c.Numeral, 0, "no title")
		}

		// A chapter that owns a run of nos outright owns nothing else at that
		// level. Chapter I of the English Integration prints three nos under
		// the chapter heading and never a §, and every other chapter of the
		// library opens a § first, so a chapter holding both is a contents line
		// misread as a no. and not a printing anybody has to support.
		if len(c.Subsections) > 0 && len(c.Sections) > 0 {
			add(c.Numeral, 0, "lists %d no. before its first § and then %d §, so a § line was read as a no.",
				len(c.Subsections), len(c.Sections))
		}
		// The chapter opens on its own no. 1, the way a § does, and for the same
		// reason: the two numbers are read off two different lines, so a gap
		// between them is the sharpest sign of a misread digit there is.
		if len(c.Subsections) > 0 && c.Subsections[0].Page-c.Page != 0 && c.Subsections[0].Page-c.Page != 1 {
			add(c.Numeral, 0, "no. 1 starts at printed page %d but the chapter starts at %d",
				c.Subsections[0].Page, c.Page)
		}
		barelast := 0
		for j, sub := range c.Subsections {
			if sub.Number != j+1 {
				add(c.Numeral, 0, "no. %d is listed %d%s, so a no. is missing or doubled",
					sub.Number, j+1, ordinal(j+1))
			}
			if sub.Page < barelast {
				add(c.Numeral, 0, "no. %d starts at printed page %d, before no. %d at %d",
					sub.Number, sub.Page, j, barelast)
			}
			if !inChapter(sp, sub.Page) {
				add(c.Numeral, 0, "no. %d starts at printed page %d, outside the chapter",
					sub.Number, sub.Page)
			}
			if sub.PDFPage == 0 {
				nopdf(c.Numeral, 0, fmt.Sprintf("no. %d ", sub.Number), sub.Page)
			}
			if sub.Title == "" {
				add(c.Numeral, 0, "no. %d has no title", sub.Number)
			}
			barelast = sub.Page
		}

		// § are counted on their own, because the appendices that close
		// chapters II, III and VIII are listed among them and carry their own
		// numbering, or none at all.
		//
		// base is where this chapter's § numbering begins, one less than its
		// first §. It is 0 everywhere but in a fascicule, which does not start
		// over: the Varietes runs § 1 to § 7 in the fascicule bound first and
		// § 8 to § 15 in the second, one numbering through the volume, and
		// asking the second to open at § 1 reported all eight of its § as
		// missing or doubled. What is asked of it instead is that it picks up
		// where the fascicule before it left off, which is the same check.
		base := 0
		if fascicule(sp) {
			base = sections
		}
		last, nsec, napp, exlast := 0, 0, 0, 0
		for _, s := range c.Sections {
			switch {
			case !s.Appendix:
				nsec++
				if napp > 0 {
					add(c.Numeral, s.Number, "is listed after an appendix")
				}
				if s.Number != base+nsec {
					add(c.Numeral, s.Number, "is listed where § %d should be, so a § is missing or doubled",
						base+nsec)
				}
			case s.Number > 0:
				napp++
				if s.Number != napp {
					add(c.Numeral, 0, "appendix %d is listed %d%s, so an appendix is missing or doubled",
						s.Number, napp, ordinal(napp))
				}
			default:
				napp++
			}
			if !inChapter(sp, s.Page) {
				add(c.Numeral, s.Number, "starts at printed page %d, outside the chapter's %d to %d",
					s.Page, sp.FirstPage, sp.LastPage)
			}
			if s.Page < last {
				add(c.Numeral, s.Number, "starts at printed page %d, before the previous § at %d",
					s.Page, last)
			}
			last = s.Page
			if s.PDFPage == 0 {
				nopdf(c.Numeral, s.Number, "", s.Page)
			}
			// An appendix is allowed to have no title. Chapter VII of the
			// English Integration 7 to 9 prints "Appendix I" and "Appendix II"
			// bare, in the contents and over the appendix itself, and a § is
			// the only thing that always carries one.
			if s.Title == "" && !s.Appendix {
				add(c.Numeral, s.Number, "no title")
			}

			// Bourbaki opens a § with its no. 1, on the same page or on the
			// next one, because a § heading that falls near the foot of a page
			// leaves its first no. to the page after: chapter VII § 2 of the
			// English Lie volume is headed at the foot of page 12 and its no. 1
			// begins on 13. Anything wider than that is a misread digit, and it
			// is the sharpest sign of one there is, because the two numbers come
			// from two different lines.
			if len(s.Subsections) > 0 && s.Subsections[0].Page-s.Page != 0 &&
				s.Subsections[0].Page-s.Page != 1 {
				add(c.Numeral, s.Number, "no. 1 starts at printed page %d but the § starts at %d",
					s.Subsections[0].Page, s.Page)
			}
			sublast := 0
			for j, sub := range s.Subsections {
				if sub.Number != j+1 {
					add(c.Numeral, s.Number, "no. %d is listed %d%s, so a no. is missing or doubled",
						sub.Number, j+1, ordinal(j+1))
				}
				if sub.Page < sublast {
					add(c.Numeral, s.Number, "no. %d starts at printed page %d, before no. %d at %d",
						sub.Number, sub.Page, j, sublast)
				}
				if !inChapter(sp, sub.Page) {
					add(c.Numeral, s.Number, "no. %d starts at printed page %d, outside the chapter",
						sub.Number, sub.Page)
				}
				if sub.PDFPage == 0 {
					nopdf(c.Numeral, s.Number, fmt.Sprintf("no. %d ", sub.Number), sub.Page)
				}
				sublast = sub.Page
			}
			if s.Exercises != nil {
				if !inChapter(sp, s.Exercises.Page) {
					add(c.Numeral, s.Number, "the exercises start at printed page %d, outside the chapter",
						s.Exercises.Page)
				}
				if s.Exercises.Page < s.Page {
					add(c.Numeral, s.Number, "the exercises start at printed page %d, before the § at %d",
						s.Exercises.Page, s.Page)
				}
				if s.Exercises.PDFPage == 0 {
					nopdf(c.Numeral, s.Number, "the exercises' ", s.Exercises.Page)
				}
				// The runs are printed in the order of the §§ they belong to,
				// whether they follow each § or are gathered at the end of the
				// chapter, so a run that goes backwards means a run has been
				// hung on the wrong §. It is the only sign there is when the
				// wrong § is a real one: the English Algebra sets the run for
				// chapter II § 10 as "Exercises for § I 0", and read as § 1 it
				// replaces the real run of § 1 with a page that is otherwise
				// perfectly in range.
				if s.Exercises.Page < exlast {
					add(c.Numeral, s.Number, "the exercises start at printed page %d, before the previous §'s at %d",
						s.Exercises.Page, exlast)
				}
				exlast = s.Exercises.Page
			}
		}
		if c.Exercises != nil {
			if !inChapter(sp, c.Exercises.Page) {
				add(c.Numeral, 0, "the chapter's exercises start at printed page %d, outside the chapter",
					c.Exercises.Page)
			}
			if c.Exercises.PDFPage == 0 {
				nopdf(c.Numeral, 0, "the chapter's exercises' ", c.Exercises.Page)
			}
		}
		if c.Historical != nil {
			if !inChapter(sp, c.Historical.Page) {
				add(c.Numeral, 0, "the historical note starts at printed page %d, outside the chapter",
					c.Historical.Page)
			}
			if c.Historical.PDFPage == 0 {
				nopdf(c.Numeral, 0, "the historical note's ", c.Historical.Page)
			}
		}
		sections = base + nsec
	}
	return probs
}

func inChapter(sp pagemap.Span, page int) bool {
	return page >= sp.FirstPage && page <= sp.LastPage
}

func ordinal(n int) string {
	if n%100 >= 11 && n%100 <= 13 {
		return "th"
	}
	switch n % 10 {
	case 1:
		return "st"
	case 2:
		return "nd"
	case 3:
		return "rd"
	}
	return "th"
}
