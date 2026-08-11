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

	if len(r.Chapters) == 0 {
		add("", 0, "the contents yielded no chapters")
		return probs
	}

	want := map[string]pagemap.Span{}
	for _, sp := range pm.Chapters {
		want[sp.Chapter] = sp
	}
	if len(opt.Chapters) > 0 && len(r.Chapters) != len(opt.Chapters) {
		add("", 0, "the contents lists %d chapters, the volume has %d",
			len(r.Chapters), len(opt.Chapters))
	}

	for _, c := range r.Chapters {
		sp, ok := want[c.Numeral]
		if !ok {
			add(c.Numeral, 0, "the contents lists a chapter the page map never found")
			continue
		}
		if c.Page != sp.FirstPage {
			add(c.Numeral, 0, "the contents starts it at printed page %d, the page map at %d",
				c.Page, sp.FirstPage)
		}
		if c.PDFPage == 0 {
			add(c.Numeral, 0, "printed page %d is on no pdf page", c.Page)
		}
		if c.Title == "" {
			add(c.Numeral, 0, "no title")
		}

		// § are counted on their own, because the appendices that close
		// chapters II, III and VIII are listed among them and carry their own
		// numbering, or none at all.
		last, nsec, napp, exlast := 0, 0, 0, 0
		for _, s := range c.Sections {
			switch {
			case !s.Appendix:
				nsec++
				if napp > 0 {
					add(c.Numeral, s.Number, "is listed after an appendix")
				}
				if s.Number != nsec {
					add(c.Numeral, s.Number, "is the %d%s § listed, so a § is missing or doubled",
						nsec, ordinal(nsec))
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
				add(c.Numeral, s.Number, "printed page %d is on no pdf page", s.Page)
			}
			if s.Title == "" {
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
					add(c.Numeral, s.Number, "no. %d printed page %d is on no pdf page",
						sub.Number, sub.Page)
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
					add(c.Numeral, s.Number, "the exercises' printed page %d is on no pdf page",
						s.Exercises.Page)
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
				add(c.Numeral, 0, "the chapter's exercises' printed page %d is on no pdf page",
					c.Exercises.Page)
			}
		}
		if c.Historical != nil {
			if !inChapter(sp, c.Historical.Page) {
				add(c.Numeral, 0, "the historical note starts at printed page %d, outside the chapter",
					c.Historical.Page)
			}
			if c.Historical.PDFPage == 0 {
				add(c.Numeral, 0, "the historical note's printed page %d is on no pdf page",
					c.Historical.Page)
			}
		}
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
