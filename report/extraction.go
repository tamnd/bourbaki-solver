package report

import (
	"fmt"
	"sort"
	"strings"
)

// What the extraction is worth, volume by volume, out of the page files
// themselves.
//
// The project has had two ways of asking this and neither answers it. ocr check
// runs the eight rules over one book and prints a line, which is the right
// question asked one volume at a time, and report usage says what the fleet
// spent, which is a fact about the transport and not about the text. Neither
// says how much of the library has been read at all, and that is the number
// somebody deciding what to run next actually needs.
//
// So the unit here is the volume and the denominator is the PDF. A book with
// 407 pages read out of 407 and a book with 57 read out of 222 both print
// "100.0 % accepted" from ocr check, because check only ever sees the pages
// that exist. Counting the pages that do not exist is most of the point.
//
// The other half is that the rules are not a verdict. A page that passes the
// eight rules can still be wrong, which is what ocr audit is for, and a page
// that fails one is often a real page with an unclosed dollar. The rejected
// column is a work list and it is printed as one.

// Volume is one book's extraction as it stands on disk.
type Volume struct {
	ID    string
	Title string
	Lang  string
	// TextLayer is what the PDF's own text is worth, and it is here because it
	// is the difference between a volume that can be extracted in a minute and
	// one that needs the fleet for a week. A volume at "none" has to be read
	// from images page by page.
	TextLayer string
	// Pages is what the PDF holds, from manifests/books.yaml, and Read is how
	// many page files there are. The gap between them is the work left.
	Pages int
	Read  int
	// Methods counts the page files by how they were read: native, ocr,
	// ocr-repair, blank, and anything else that turns up, which is how a page
	// left at ocr-failed gets counted rather than quietly dropped.
	Methods map[string]int
	// Failed is the pages committed at method ocr-failed. The spec says the
	// audit blocks on any of these and S06 is the rule that does it, so this
	// list is expected to be empty and is printed when it is not.
	Failed []int
	// Flagged is the pages the reading itself was not sure of. They are the
	// repair backlog, and unlike Rejected they are what the model or the
	// extractor said about its own work rather than what the rules said about
	// the result.
	Flagged []int
	// Manual is the pages somebody has repaired by hand. They are worth
	// counting because extraction must not write over them and because a
	// volume with many of them is a volume that was hard to read.
	Manual int
	// Checked is how many pages the rules ran over, which is the pages that are
	// not blank, and Rejected is which of them failed at least one rule.
	Checked  int
	Rejected []int
	// Rules counts the rejections by rule, one page counted once per rule it
	// fails.
	Rules map[string]int
	// NoPageMap says the running head and page label rules did not run for this
	// volume because there is no page map to compare against. Two of the eight
	// rules being skipped is worth saying next to the number they would have
	// changed.
	NoPageMap bool
}

// Unread is the pages of the PDF that have no page file at all.
func (v Volume) Unread() int {
	if v.Pages == 0 {
		return 0
	}
	return max(0, v.Pages-v.Read)
}

// Coverage is how much of the PDF has been read, as a percentage. A volume
// whose page count is unknown reports zero rather than dividing by it.
func (v Volume) Coverage() float64 { return percent(v.Read, v.Pages) }

// Accepted is how many of the checked pages pass every rule that ran.
func (v Volume) Accepted() float64 { return percent(v.Checked-len(v.Rejected), v.Checked) }

// AcceptedText is the acceptance for a table cell. A volume whose only page
// files are blanks has nothing for the rules to run over, and printing 0.0 %
// there says the pages failed when what happened is that there were none.
func (v Volume) AcceptedText() string {
	if v.Checked == 0 {
		return "none checked"
	}
	return fmt.Sprintf("%.1f %%", v.Accepted())
}

// Method is one count out of Methods, and zero for a method this volume has
// none of.
func (v Volume) Method(name string) int { return v.Methods[name] }

// Extraction is every volume and the totals.
type Extraction struct {
	Rows  []Volume
	Total Volume
}

// SummariseExtraction sorts the volumes and adds them up. The rows come in
// whatever order the books manifest lists them; they go out worst covered
// first, because the top of this table should be the work that is left rather
// than the work that is done.
func SummariseExtraction(rows []Volume) Extraction {
	out := Extraction{Rows: append([]Volume{}, rows...)}
	total := Volume{ID: "all", Methods: map[string]int{}, Rules: map[string]int{}}
	for _, row := range rows {
		total.Pages += row.Pages
		total.Read += row.Read
		total.Checked += row.Checked
		total.Manual += row.Manual
		total.Rejected = append(total.Rejected, row.Rejected...)
		total.Failed = append(total.Failed, row.Failed...)
		total.Flagged = append(total.Flagged, row.Flagged...)
		for method, n := range row.Methods {
			total.Methods[method] += n
		}
		for rule, n := range row.Rules {
			total.Rules[rule] += n
		}
	}
	sort.Slice(out.Rows, func(i, j int) bool {
		a, b := out.Rows[i], out.Rows[j]
		if ca, cb := a.Coverage(), b.Coverage(); ca != cb {
			return ca < cb
		}
		return a.ID < b.ID
	})
	out.Total = total
	return out
}

// Whole, Part and Untouched are how many volumes are read through, how many are
// begun, and how many have no page file at all. The three of them are what the
// coverage percentage hides: 22 per cent of the library read could be every
// volume a fifth of the way through, and it is not, it is seven volumes done
// and twenty three never opened.
func (e Extraction) Whole() int { return e.count(func(v Volume) bool { return v.Read >= v.Pages }) }
func (e Extraction) Part() int {
	return e.count(func(v Volume) bool { return v.Read > 0 && v.Read < v.Pages })
}
func (e Extraction) Untouched() int { return e.count(func(v Volume) bool { return v.Read == 0 }) }

func (e Extraction) count(is func(Volume) bool) int {
	var n int
	for _, row := range e.Rows {
		if is(row) {
			n++
		}
	}
	return n
}

// methodsSeen is every method any volume used, in the order the pipeline writes
// them, with anything it does not know about after them in name order. Fixing
// the first four keeps the columns of the table steady from one run to the
// next, and keeping the rest means a method nobody planned for still shows up.
func (e Extraction) methodsSeen() []string {
	known := []string{"native", "ocr", "ocr-repair", "blank"}
	seen := map[string]bool{}
	for _, name := range known {
		seen[name] = true
	}
	var extra []string
	for name := range e.Total.Methods {
		if !seen[name] {
			extra = append(extra, name)
		}
	}
	sort.Strings(extra)
	return append(known, extra...)
}

// Table is the terminal form: one line a volume, worst covered first.
func (e Extraction) Table() string {
	if len(e.Rows) == 0 {
		return "no volume has a page file yet\n"
	}
	width := len("volume")
	for _, row := range e.Rows {
		width = max(width, len(row.ID))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "%-*s  %-4s  %6s  %6s  %6s  %8s  %7s  %12s\n",
		width, "volume", "lang", "pdf", "read", "unread", "coverage", "checked", "accepted")
	rows := append(append([]Volume{}, e.Rows...), e.Total)
	for _, row := range rows {
		fmt.Fprintf(&b, "%-*s  %-4s  %6d  %6d  %6d  %7.1f %%  %7d  %12s\n",
			width, row.ID, row.Lang, row.Pages, row.Read, row.Unread(),
			row.Coverage(), row.Checked, row.AcceptedText())
	}
	if len(e.Total.Failed) > 0 {
		fmt.Fprintf(&b, "\n%d page(s) are committed at ocr-failed and the audit blocks on every one\n", len(e.Total.Failed))
	}
	if len(e.Total.Rules) > 0 {
		b.WriteString("\nwhy pages are rejected, counted once per rule per page\n")
		for _, rule := range sorted(e.Total.Rules) {
			fmt.Fprintf(&b, "  %-12s %4d\n", rule, e.Total.Rules[rule])
		}
	}
	return b.String()
}

// Doc is the whole thing as one page of Markdown, for reports/. It says more
// than the table does, because a file nobody has to run a command to read is
// worth the extra paragraphs.
func (e Extraction) Doc() string {
	var b strings.Builder
	b.WriteString("# What the extraction is worth\n\n")
	b.WriteString("Generated by bourbaki report extraction -write. Do not edit.\n\n")
	b.WriteString("Every number here is counted off the page files in `pages/` and the volumes in `manifests/books.yaml`, so it is what the corpus holds today and not what anybody hopes it holds. A volume with no row in the coverage table has no page file at all.\n\n")
	b.WriteString("Read is how many pages of the PDF have a page file, blanks included, since a blank page that has been looked at is a page that is done. Checked is the pages the validation rules ran over, which is the read pages that are not blank, and accepted is how many of those pass every rule that ran. The rules are the same ones the extraction itself accepts a page on, so this is that decision re-run against what is on disk.\n\n")
	b.WriteString("Passing the rules is not the same as being right. A page can balance its dollars, carry a plausible running head and still read an interval as a set, which is what `ocr audit` is for. A rejected page is a place to look and most of them are one character.\n\n")

	b.WriteString("## Coverage, worst first\n\n")
	b.WriteString("| Volume | Lang | Text layer | PDF pages | Read | Unread | Coverage | Checked | Accepted |\n")
	b.WriteString("| --- | --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |\n")
	for _, row := range e.Rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d | %d | %.1f %% | %d | %s |\n",
			row.ID, row.Lang, textLayer(row.TextLayer), row.Pages, row.Read, row.Unread(),
			row.Coverage(), row.Checked, row.AcceptedText())
	}
	fmt.Fprintf(&b, "| **all** | | | **%d** | **%d** | **%d** | **%.1f %%** | **%d** | **%s** |\n\n",
		e.Total.Pages, e.Total.Read, e.Total.Unread(), e.Total.Coverage(),
		e.Total.Checked, e.Total.AcceptedText())

	fmt.Fprintf(&b, "Of %d volumes, %d are read through, %d are begun and %d have no page file at all. That is worth saying next to the percentage, because %.1f %% of the library read could mean every volume a fifth of the way through and it does not: it means a few volumes finished and most of them never opened.\n\n",
		len(e.Rows), e.Whole(), e.Part(), e.Untouched(), e.Total.Coverage())

	b.WriteString("## How the pages were read\n\n")
	methods := e.methodsSeen()
	b.WriteString("| Volume |")
	for _, method := range methods {
		fmt.Fprintf(&b, " %s |", method)
	}
	b.WriteString(" flagged | by hand |\n| --- |")
	for range methods {
		b.WriteString(" ---: |")
	}
	b.WriteString(" ---: | ---: |\n")
	for _, row := range e.Rows {
		// A volume with nothing read is a row of zeros here and it is already
		// in the table above, where the zero is the point. Repeating it once a
		// column buries the volumes that have been read.
		if row.Read == 0 {
			continue
		}
		fmt.Fprintf(&b, "| %s |", row.ID)
		for _, method := range methods {
			fmt.Fprintf(&b, " %d |", row.Method(method))
		}
		fmt.Fprintf(&b, " %d | %d |\n", len(row.Flagged), row.Manual)
	}
	b.WriteString("| **all** |")
	for _, method := range methods {
		fmt.Fprintf(&b, " **%d** |", e.Total.Method(method))
	}
	fmt.Fprintf(&b, " **%d** | **%d** |\n\n", len(e.Total.Flagged), e.Total.Manual)
	b.WriteString("Volumes with no page file are left out of this table and are in the one above, where the zero is the point. Native is the text layer of a born-digital PDF, ocr is a model reading a page image, ocr-repair is a model putting back a display the text layer lost, and blank is a page with nothing on it. Flagged is the pages the reading was not sure of and by hand is the pages somebody has since repaired themselves, which extraction is forbidden to write over.\n\n")

	b.WriteString("## Pages left at ocr-failed\n\n")
	if len(e.Total.Failed) == 0 {
		b.WriteString("None. The extraction rejects a page it cannot read rather than committing it, so a page that fails three times leaves no file behind and shows up above as an unread page. S06 in the audit is what would catch one if a future run wrote it.\n\n")
	} else {
		fmt.Fprintf(&b, "%d, and the audit blocks on every one of them under S06.\n\n", len(e.Total.Failed))
		for _, row := range e.Rows {
			if len(row.Failed) > 0 {
				fmt.Fprintf(&b, "- %s: %s\n", row.ID, pages(row.Failed))
			}
		}
		b.WriteString("\n")
	}

	b.WriteString("## What the rules reject\n\n")
	if len(e.Total.Rules) == 0 {
		b.WriteString("Nothing. Every page on disk passes every rule that ran.\n\n")
	} else {
		b.WriteString("| Rule | Pages |\n| --- | ---: |\n")
		for _, rule := range sorted(e.Total.Rules) {
			fmt.Fprintf(&b, "| %s | %d |\n", rule, e.Total.Rules[rule])
		}
		b.WriteString("\nOne page is counted once per rule it fails, so these add up to more than the rejected pages when a page fails two.\n\n")
		b.WriteString("| Volume | Rejected | Rules | Pages |\n| --- | ---: | --- | --- |\n")
		for _, row := range e.Rows {
			if len(row.Rejected) == 0 {
				continue
			}
			fmt.Fprintf(&b, "| %s | %d | %s | %s |\n",
				row.ID, len(row.Rejected), strings.Join(sorted(row.Rules), ", "), pages(row.Rejected))
		}
		b.WriteString("\n")
	}

	var skipped []string
	for _, row := range e.Rows {
		if row.NoPageMap {
			skipped = append(skipped, row.ID)
		}
	}
	if len(skipped) > 0 {
		b.WriteString("## Where two of the rules did not run\n\n")
		fmt.Fprintf(&b, "There is no page map for %s, so the running head rule and the page label rule were skipped over those pages. Six of the eight rules ran, and the acceptance figure there is worth that much less than the rest of the column.\n\n",
			strings.Join(skipped, ", "))
	}
	return strings.TrimRight(b.String(), "\n") + "\n"
}

// textLayer is what to print for a volume whose manifest does not say. An empty
// cell reads as a fact about the volume rather than as a gap in the record.
func textLayer(s string) string {
	if strings.TrimSpace(s) == "" {
		return "not recorded"
	}
	return s
}

// pages prints a page list, and cuts it off before it takes over the table. The
// whole list is in the JSON.
func pages(list []int) string {
	const show = 20
	sorted := append([]int{}, list...)
	sort.Ints(sorted)
	parts := make([]string, 0, show)
	for i, page := range sorted {
		if i == show {
			return strings.Join(parts, " ") + fmt.Sprintf(" and %d more", len(sorted)-show)
		}
		parts = append(parts, fmt.Sprint(page))
	}
	return strings.Join(parts, " ")
}

func percent(part, whole int) float64 {
	if whole <= 0 {
		return 0
	}
	return float64(part) * 100 / float64(whole)
}
