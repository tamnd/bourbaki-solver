package extract

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// Reading a whole volume is reading its pages one after another and counting
// what came of it. The count is the point: the target for this volume is that
// nine pages in ten come out of the text layer with nothing flagged on them,
// and a run that does not say how it did against that is a run nobody can
// argue with.

// Result is what one run of extraction did.
type Result struct {
	Book    string         `json:"book"`
	Pages   int            `json:"pages"`
	Clean   int            `json:"clean"`
	Flagged int            `json:"flagged"`
	Empty   int            `json:"empty"`
	Reasons map[string]int `json:"reasons"`
	// Flags lists every flagged page and why, so the repair pass has its work
	// list and a reader can check any of it against the printed page.
	Flags []PageFlags `json:"flagged_pages"`
	// Columns lists the pages set in two columns, which assembly reads as two
	// columns and not as one.
	Columns []int `json:"two_column_pages,omitempty"`
	// Minus is how many drawn set difference signs were put back into the
	// text, and MinusLost how many the volume draws that could not be placed.
	// A run that does not say both is a run that cannot be argued with: the
	// sign is invisible in the text layer, so the only account of it is this.
	Minus     int `json:"set_difference_placed"`
	MinusLost int `json:"set_difference_unplaced"`
}

// PageFlags is one flagged page.
type PageFlags struct {
	PDFPage int      `json:"pdf_page"`
	Label   string   `json:"page_label,omitempty"`
	Flags   []string `json:"flags"`
}

// Add records one page.
func (r *Result) Add(p *Page) {
	r.Pages++
	r.Minus += p.Minus
	r.MinusLost += p.MinusLost
	if r.Reasons == nil {
		r.Reasons = map[string]int{}
	}
	switch {
	case len(p.Flags) == 0:
		r.Clean++
		return
	case slicesHas(p.Flags, FlagEmpty):
		r.Empty++
	default:
		r.Flagged++
	}
	f := PageFlags{PDFPage: p.PDFPage, Label: p.Label}
	for _, x := range p.Flags {
		r.Reasons[string(x)]++
		f.Flags = append(f.Flags, string(x))
	}
	r.Flags = append(r.Flags, f)
}

func slicesHas(fs []Flag, f Flag) bool {
	for _, x := range fs {
		if x == f {
			return true
		}
	}
	return false
}

// Rate is the share of pages that came out clean, counting the blank ones as
// clean: a page with nothing on it has nothing to repair.
func (r *Result) Rate() float64 {
	if r.Pages == 0 {
		return 0
	}
	return float64(r.Clean+r.Empty) / float64(r.Pages)
}

// String is the line a run prints when it finishes.
func (r *Result) String() string {
	var b strings.Builder
	fmt.Fprintf(&b, "%s  %d pages: %d clean, %d flagged, %d empty  (%.1f%% native)\n",
		r.Book, r.Pages, r.Clean, r.Flagged, r.Empty, 100*r.Rate())
	if r.Minus > 0 || r.MinusLost > 0 {
		fmt.Fprintf(&b, "  %d drawn set difference signs put back, %d could not be placed\n",
			r.Minus, r.MinusLost)
	}
	reasons := make([]string, 0, len(r.Reasons))
	for k := range r.Reasons {
		reasons = append(reasons, k)
	}
	sort.Slice(reasons, func(i, j int) bool { return r.Reasons[reasons[i]] > r.Reasons[reasons[j]] })
	for _, k := range reasons {
		fmt.Fprintf(&b, "  %-16s %4d\n", k, r.Reasons[k])
	}
	return b.String()
}

// InputSHA256 hashes what extraction reads off a page: every run with its box
// and the font it was set in, and every rule the page draws that carries an
// operator. That is what says whether the page has to be read again.
//
// The rules are in it because they are read, and only the ones that carry an
// operator because only those are. A fraction bar is drawn too, and hashing it
// would move the hash of the 300 pages of this volume that set a fraction
// without changing a character of what they say.
func InputSHA256(l *pdfsrc.Layout, p pdfsrc.Page) string {
	h := sha256.New()
	fmt.Fprintf(h, "page %d %dx%d\n", p.Number, p.Width, p.Height)
	for _, s := range p.Spans {
		fmt.Fprintf(h, "%d %d %d %d %s %q\n", s.Top, s.Left, s.Width, s.Height,
			l.Font(s).Base(), s.Text)
	}
	for _, r := range p.Rules {
		if minus(r) {
			fmt.Fprintf(h, "rule %d %d %d\n", r.Top, r.Left, r.Width)
		}
	}
	return hex.EncodeToString(h.Sum(nil))
}
