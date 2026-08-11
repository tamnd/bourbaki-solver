package quality

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/mathtex"
)

// The mathematics rules are about the one thing a transcription of Bourbaki has
// to get right and the one thing that is hardest to see by reading: whether the
// formulae survived.
//
// They all work off the same split of a body into its math spans, because a
// rule that decides for itself where the mathematics is will disagree with the
// next rule that does, and two audits that disagree about the same file are
// worse than one audit that is wrong.

func init() {
	register(
		Check{ID: "M01", Group: Mathematics, Hard: true,
			Title: "every math span is closed", Run: m01},
		Check{ID: "M02", Group: Mathematics, Hard: true,
			Title: "the number sets are \\mathbf, as Bourbaki sets them", Run: m02},
		Check{ID: "M03", Group: Mathematics, Hard: true,
			Title: "no character stranded out of its TeX", Run: m03},
		Check{ID: "M04", Group: Mathematics, Hard: true,
			Title: "every math span parses", Run: m04, Need: needTeX},
		Check{ID: "M05", Group: Mathematics, Hard: true,
			Title: "no illegible marker is left in the corpus", Run: m05},
		Check{ID: "M06", Group: Mathematics, Hard: false,
			Title: "displays per page within three sigma of the book mean", Run: m06},
		Check{ID: "M07", Group: Mathematics, Hard: true,
			Title: "no bracket from the prose closes inside the mathematics", Run: m07},
		Check{ID: "M08", Group: Mathematics, Hard: false,
			Title: "no matrix is left flattened into a pair of scripts", Run: m08},
	)
}

// A Span is one stretch of mathematics in a body, without its delimiters. The
// splitter is in package mathtex, under both this package and extract, because
// the tool that writes the pages and the audit that reads them back have to
// agree about where the mathematics is.
type Span = mathtex.Span

// Math splits a normalised body into its math spans. See mathtex.Split.
func Math(body string) (spans []Span, unclosed *Span) { return mathtex.Split(body) }

// M01. Every math span is closed.
//
// This is the cheapest rule here and it has found the most. It has been down
// three times and each fall was a different fault behind it.
//
// The first was an exercise whose number was set inside the mathematics, so
// splitting the § at the number cut the span in half: see afterMarker in
// package assemble, which takes the marker's own dollar off with it. The second
// was a numbered display flattened into a line of prose with the delimiter that
// closed it left against the full stop, which is mathtex.DropStray, and it was
// eight pages.
//
// What is left is five files, and they are all the third fault: six pages of
// the volume where a matrix or a bracket arrived from the text layer in pieces
// and a display lost its opening delimiter rather than its closing one. Nothing
// mechanical repairs those. They are marked for the OCR repair pass, which
// reads the printed page, and the rest of this rule's findings follow them: a
// span left open runs to the end of the file, so every M rule downstream reads
// prose as mathematics until it closes.
func m01(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		_, open := Math(d.Body)
		if open == nil {
			continue
		}
		kind := "an inline span"
		if open.Display {
			kind = "a display"
		}
		out = append(out, Finding{File: d.Path, Line: d.BodyLine(open.Line),
			Msg: fmt.Sprintf("%s opens here and nothing closes it: %s", kind, ellipsis(open.Text, 60))})
	}
	return out, nil
}

func ellipsis(s string, n int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len([]rune(s)) <= n {
		return s
	}
	return string([]rune(s)[:n]) + "…"
}

// M02. The number sets are \mathbf.
//
// Bourbaki sets Z, Q, R, C and N in bold, not in blackboard bold, and a model
// asked to transcribe mathematics writes \mathbb because that is what almost
// every other book does. It is the single most likely thing for OCR to get
// consistently and invisibly wrong, so it is checked rather than trusted.
func m02(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		for i, line := range strings.Split(d.Body, "\n") {
			if j := strings.Index(line, `\mathbb`); j >= 0 {
				out = append(out, Finding{File: d.Path, Line: d.BodyLine(i + 1),
					Msg: `\mathbb where Bourbaki sets \mathbf: ` + ellipsis(line[maxInt(0, j-20):], 60)})
			}
		}
	}
	return out, nil
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}

// M03. No character stranded out of its TeX.
//
// Five faults, every one of them found in this corpus before it was written
// down as a rule:
//
// A literal Greek letter inside a math span. The volume had 585 of them, all
// capitals bar four, sitting alongside the TeX for the same letter in the same
// sentence: § 5 had "$\lambda \in Λ$" and "$e_{\Lambda}$" three lines apart.
// The cause is in the printing. Bourbaki sets its capital Greek upright, in the
// text face, so the run is prose as far as the extractor's font table is
// concerned and the letter comes through as itself.
//
// The same letters again at other code points, which is what a census turned up
// once the Greek block was clear: 303 micro signs, U+00B5, for the µ of a
// family, 42 ohm signs, U+2126, for Ω, and three dotless i. These are the worst
// of the five, because they print correctly and read correctly and the only way
// to see one is to count.
//
// A Unicode operator inside a math span. 38 of the increment sign, U+2206,
// standing in for \Delta.
//
// A replacement glyph, a private use character, or a spacing accent with no
// letter under it, anywhere. The first two mean a character was lost between
// the page and the file. The third means one came apart from another: the four
// U+02C6 in the volume are all the hat of a \widehat, left standing next to the
// symbol it belonged over.
//
// A line whose whole content is one capital letter. There is one, an H in § 16,
// and it is the remains of a commutative diagram that was flattened into three
// lines of prose.
//
// Greek in the prose outside the mathematics is left alone. It is inconsistent
// with the TeX in the same sentence, but it reads correctly and repairing it is
// a rewrite of the transcription rather than a repair of it.
//
// The first three of the five are mechanical, one glyph for the TeX that prints
// that glyph, and bourbaki fix math does them. See mathtex.Repair for what it
// will not touch and why.
func m03(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		spans, _ := Math(d.Body)
		for _, s := range spans {
			for _, r := range s.Text {
				if name, bad := strandedInMath(r); bad {
					out = append(out, Finding{File: d.Path, Line: d.BodyLine(s.Line),
						Msg: fmt.Sprintf("%s inside the mathematics: %s", name, ellipsis(s.Text, 50))})
					break
				}
			}
		}
		for i, line := range strings.Split(d.Body, "\n") {
			at := d.BodyLine(i + 1)
			for _, r := range line {
				if name, bad := strandedAnywhere(r); bad {
					out = append(out, Finding{File: d.Path, Line: at,
						Msg: fmt.Sprintf("%s: %s", name, ellipsis(line, 50))})
					break
				}
			}
			if strings.Contains(line, "7→") {
				out = append(out, Finding{File: d.Path, Line: at,
					Msg: "7→ is what pdftotext makes of ↦: " + ellipsis(line, 50)})
			}
			if t := strings.TrimSpace(line); len([]rune(t)) == 1 && unicode.IsUpper([]rune(t)[0]) {
				out = append(out, Finding{File: d.Path, Line: at,
					Msg: fmt.Sprintf("a line with nothing on it but %q, which is a display that came apart", t)})
			}
		}
	}
	return out, nil
}

func strandedInMath(r rune) (string, bool) {
	if name, bad := strandedAnywhere(r); bad {
		return name, true
	}
	switch {
	case r >= 0x0370 && r <= 0x03FF:
		return fmt.Sprintf("the letter %q where its TeX belongs", r), true
	// The compatibility characters, which are the same letters at other code
	// points and were missed by the block above until the volume was counted
	// character by character: 303 micro signs, U+00B5, standing for the µ of a
	// family, 42 ohm signs, U+2126, standing for Ω, and three dotless i. A
	// range is the wrong tool for these. They are scattered across three blocks
	// and their neighbours are not letters at all, so they are named.
	case r == 0x00B5, r == 0x2126, r == 0x0131:
		return fmt.Sprintf("the letter %q where its TeX belongs", r), true
	case r >= 0x2190 && r <= 0x21FF, r >= 0x2200 && r <= 0x22FF,
		r >= 0x27F0 && r <= 0x27FF, r >= 0x2A00 && r <= 0x2AFF:
		return fmt.Sprintf("the operator %q where its TeX belongs", r), true
	}
	return "", false
}

func strandedAnywhere(r rune) (string, bool) {
	switch {
	case r == 0xFFFD:
		return "a replacement glyph, so a character was lost", true
	case r >= 0xE000 && r <= 0xF8FF:
		return "a private use character, which is a font artefact and not text", true
	case r >= 0x02B0 && r <= 0x02FF:
		// A spacing accent stands on its own only where the letter it was drawn
		// over has come away from it. The volume has four, all U+02C6 and all
		// the hat of a \widehat that was flattened: "Gˆ" for \widehat{G} and
		// "ˆ$\tau" for \widehat{\tau}. They are not repaired mechanically,
		// because putting the accent back means deciding which symbol it was
		// over and how far the brace reaches, and that is reading the page.
		return fmt.Sprintf("the accent %q with no letter under it, which is a lost \\widehat", r), true
	}
	return "", false
}

// needTeX says why M04 did not run.
func needTeX(c *Corpus) string {
	if !c.Opt.ValidateTeX {
		return "not asked for, run with -validate-tex"
	}
	return ""
}

// M04. Every math span parses.
//
// The spec asks for a KaTeX parse with latexmk behind it. Neither is wired,
// and the reason is worth writing down rather than leaving as a gap: CI here
// installs Go and nothing else, and a rule that only runs on the one machine
// with a TeX distribution on it is a rule that will be broken for months
// before anybody notices. Reimplementing KaTeX in Go to avoid that is not a
// trade anybody should make either.
//
// What is here instead is the structural half, which needs nothing installed
// and catches the faults that a lost character actually produces: a brace that
// does not close, a backslash with no command after it, a subscript or a
// superscript with nothing under it, and an empty span. The § 21 region that
// M01 reports fails this one too, from the other side.
//
// It stays behind -validate-tex, as the spec puts it, because it is the rule
// most likely to want tightening and the one whose false positives would cost
// the most.
func m04(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		spans, _ := Math(d.Body)
		for _, s := range spans {
			if why := parseTeX(s.Text); why != "" {
				out = append(out, Finding{File: d.Path, Line: d.BodyLine(s.Line),
					Msg: fmt.Sprintf("%s: %s", why, ellipsis(s.Text, 50))})
			}
		}
	}
	return out, nil
}

// parseTeX is the structural reading of one span, and returns why it will not
// parse or the empty string when nothing is wrong with its shape.
func parseTeX(s string) string {
	rs := []rune(s)
	if strings.TrimSpace(s) == "" {
		return "the span is empty"
	}
	depth := 0
	for i := 0; i < len(rs); i++ {
		switch rs[i] {
		case '\\':
			if i+1 >= len(rs) {
				return "a backslash at the end with no command after it"
			}
			if unicode.IsLetter(rs[i+1]) {
				for i+1 < len(rs) && unicode.IsLetter(rs[i+1]) {
					i++
				}
				continue
			}
			if unicode.IsSpace(rs[i+1]) {
				return "a backslash with white space after it, so the command name was lost"
			}
			i++ // an escaped character, which is a command in its own right
		case '{':
			depth++
		case '}':
			depth--
			if depth < 0 {
				return "a closing brace with nothing open"
			}
		case '^', '_':
			if i+1 >= len(rs) || unicode.IsSpace(rs[i+1]) {
				return fmt.Sprintf("a %q with nothing under it", rs[i])
			}
		}
	}
	if depth > 0 {
		return fmt.Sprintf("%d brace(s) left open", depth)
	}
	return ""
}

// M05. No illegible marker is left in the corpus.
//
// The OCR prompt tells the model to write ⟪illegible⟫ rather than guess, which
// is the right instruction and the reason the corpus can be trusted at all. It
// also means the marker is a piece of unfinished work sitting in the text, and
// it has to leave before the page is done.
func m05(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		for i, line := range strings.Split(d.Body, "\n") {
			if strings.Contains(line, "⟪illegible⟫") {
				out = append(out, Finding{File: d.Path, Line: d.BodyLine(i + 1),
					Msg: "an illegible marker is still here: " + ellipsis(line, 60)})
			}
		}
	}
	return out, nil
}

// M06. Displays per page within three sigma of the book mean.
//
// The fault this is aimed at is a page whose displayed equations were flattened
// into the prose, which leaves a section that reads almost right and has half
// the mathematics set inline.
//
// It is soft, and on chapter VIII it flags nothing, which is worth saying
// plainly rather than leaving to be discovered. The 27 files of the chapter run
// from no displays at all to 2.37 a page, a mean of 0.73 and a standard
// deviation of 0.70, so three sigma reaches 2.8 and nothing in the volume is
// near it. § 10 has nine pages and no display, which is the shape this rule is
// looking for, and at 1.05 sigma below the mean it is nowhere close to being
// caught. A rule with a wide enough spread stops being a rule, and the honest
// thing is to run it, publish the numbers it measured, and not pretend the
// green means anything yet. It will mean something across four volumes.
func m06(c *Corpus) ([]Finding, error) {
	type row struct {
		doc   Doc
		pages int
		rate  float64
	}
	var rows []row
	for _, d := range c.Docs {
		if d.Section == nil || d.Lang != "en" {
			continue
		}
		first, last, err := pdfRange(d.Section.PDFPages)
		if err != nil || last < first {
			continue
		}
		pages := last - first + 1
		n := 0
		spans, _ := Math(d.Body)
		for _, s := range spans {
			if s.Display {
				n++
			}
		}
		rows = append(rows, row{d, pages, float64(n) / float64(pages)})
	}
	if len(rows) < 3 {
		return nil, nil
	}
	mean := 0.0
	for _, r := range rows {
		mean += r.rate
	}
	mean /= float64(len(rows))
	varsum := 0.0
	for _, r := range rows {
		varsum += (r.rate - mean) * (r.rate - mean)
	}
	sd := math.Sqrt(varsum / float64(len(rows)-1))
	var out []Finding
	for _, r := range rows {
		if sd > 0 && math.Abs(r.rate-mean) > 3*sd {
			out = append(out, Finding{File: r.doc.Path, Line: 1,
				Msg: fmt.Sprintf("%.2f displays a page over %d pages, against a mean of %.2f and a sigma of %.2f",
					r.rate, r.pages, mean, sd)})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].File < out[j].File })
	return out, nil
}

// M07. No bracket from the prose closes inside the mathematics.
//
// Bourbaki sets the name of a function upright, in the text face, so the text
// layer hands the name and its opening bracket back as prose and sweeps the
// closing one into the formula with the argument. The page then reads
// Tr($u)$ where it should read Tr($u$).
//
// The two print the same, which is why this was not found by reading and was
// not found by any rule here either. It was found by a translation: the model
// copied the formula back as "u" with the bracket set as prose, correctly, and
// the audit of the translation refused the section because a translation may
// not alter mathematics. That cost a seventeen minute run of the appendix on
// the trace of an endomorphism, and it would have cost one for every section
// that carries the shape.
//
// So it is hard, and the argument for that is the same argument: a span whose
// text is not the mathematics on the page is a span nothing downstream can
// copy, compare, or translate. It was 138 spans in 66 pages of chapter VIII
// when it was found, and bourbaki fix parens repaired every one it could reach.
//
// Three are left and they are all in one exercise, VIII, A2, Exercise 4, and
// they are not this fault so much as the shadow of another one. The page they
// come from is p. 477, where a display arrived from the text layer without its
// opening delimiter, so from line 7 to the foot of the page there is one span
// that never closes and the repair cannot see where any of the mathematics
// ends. The page is already one of M01's five. Assembling it cuts the page at
// the exercise markers, which closes the run, and the straddles inside it
// become visible in the section files where they were invisible in the page.
// Repairing them means repairing p. 477 against the printed image, which is the
// OCR repair pass and not a table.
//
// The other case this will report is the one the repair refuses: a span closing
// more brackets than its line has open. Moving those out would leave the line
// with a bracket that closes nothing, which is a guess about what the page says
// rather than a repair, so they are reported for somebody to read the printed
// page.
func m07(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		for _, s := range mathtex.Straddles(d.Body) {
			out = append(out, Finding{File: d.Path, Line: d.BodyLine(s.Line),
				Msg: "a bracket the prose opened closes inside the mathematics: " + ellipsis(s.Text, 60)})
		}
	}
	return out, nil
}

// M08. No matrix is left flattened into a pair of scripts.
//
// A matrix is set as rows one above the other. The text layer of a born-digital
// volume has no rows in it, only runs with a height, so the top row comes back
// raised and the bottom row lowered, and a 2 by 2 arrives as a superscript
// holding one row and a subscript holding the other. The page prints
//
//	A = ( a  b )
//	    ( c  d )
//
// and the Markdown says $(^{a b}_{c d})$, which renders as a b with c d under
// it and is not a matrix at all. The larger ones fare worse: the product of
// three 2 by 2 matrices in VIII, A2, Exercise 3 arrived as the display
// (IR)((P0)((I0) with the second row of all three left below it as loose prose.
//
// This is reported and not repaired, and the reason is the same one the dropped
// glyph test ran into. What separates a matrix from x^2_i is that the rows of a
// matrix line up, which is a fact about the boxes, and pdftohtml works a box
// out by dividing an element up by character count. The error in that division
// is the width being measured. So the page goes back to the model with the
// printed image, which is extract.FlagStackedMatrix and the -flagged path.
//
// Soft, because a flattened matrix is a defect of the reading and not of the
// corpus's structure, and because there are 62 of them across the two printings
// of chapter VIII: making it hard would stop every build until a fleet with
// image quota reads 19 pages, and the rule would then be a schedule rather than
// a check. It goes hard when the count reaches zero.
func m08(c *Corpus) ([]Finding, error) {
	var out []Finding
	for _, d := range c.Docs {
		for i, line := range strings.Split(d.Body, "\n") {
			for _, m := range mathtex.StackedRows(line) {
				out = append(out, Finding{File: d.Path, Line: d.BodyLine(i + 1),
					Msg: "a matrix the text layer flattened into a pair of scripts: " + m})
			}
		}
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].File != out[j].File {
			return out[i].File < out[j].File
		}
		return out[i].Line < out[j].Line
	})
	return out, nil
}
