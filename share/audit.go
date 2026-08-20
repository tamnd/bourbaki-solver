package share

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

// Printed is one section of the volume as the corpus has it: what the printed
// contents says the section contains, and the pages themselves.
//
// The audit holds an import against this and against nothing else. It is not a
// comparison of two transcriptions, and it does not care which of the two reads
// a formula better, because there is no way to settle that here. It asks the one
// question a machine can settle: is anything the book numbered missing from the
// import, and is there a page of the book the import passes over in silence.
type Printed struct {
	Section int
	Title   string
	// Numbers is the no. of the section, off the printed contents rather than
	// off the pages, since the contents is the book's own statement of what it
	// contains and the pages are what is being checked.
	Numbers []Numbered
	Pages   []PrintedPage
	// After is the page the next § starts on, where that page has been read.
	//
	// A § almost never ends at the foot of a page. It ends partway down the
	// next one, with the heading of the § after it below, so the last paragraph
	// of this § is printed on a page that belongs to the next. Audit does not
	// look at this and must not: a page whose greater part is the next § cannot
	// be demanded of this import. Read does look at it, in one direction only.
	// Without it the closing paragraph of every § in the library reads as a
	// sentence the import invented, which is three of the five that § 2 of
	// chapter I turned up the first time this was run.
	After []PrintedPage
}

// Numbered is one no. of a section: the number the book prints and its title.
type Numbered struct {
	No    int
	Title string
}

// PrintedPage is one page of the volume, as the reading of it is committed.
type PrintedPage struct {
	PDFPage int
	Text    string
}

// Finding is one thing the audit has to say about an import.
//
// Hard is the difference between a fact about the book and a matter of taste. A
// missing no., a missing label and a page with nothing of it in the import are
// all facts: the import is not the section it claims to be. A title that does
// not match, or a page that is thinly covered rather than absent, is worth
// printing and is not worth failing over, because a person retyping a heading
// changes its case and a page of displayed formulae has very little prose on it
// to match against.
type Finding struct {
	Rule string
	Hard bool
	Text string
}

// Result is what the audit found in one import file.
type Result struct {
	Target Target
	// Numbers, Labels and Pages are what the printed section has, so that a
	// clean result says how much was checked rather than only that nothing was
	// wrong.
	Numbers int
	Labels  int
	Pages   int
	// Thin is the pages that are in the import but only barely.
	Thin     int
	Findings []Finding
}

// Hard is the count of findings that say the import is not the section.
func (r *Result) Hard() int {
	n := 0
	for _, f := range r.Findings {
		if f.Hard {
			n++
		}
	}
	return n
}

// OK is a section that can be read as the section it says it is.
func (r *Result) OK() bool { return r.Hard() == 0 }

// headRE is a numbered head at any level.
//
// Any level on purpose. The four Theory of Sets imports set the same kind of
// head at three different depths, and one of them changes depth in the middle
// of a file: no. 1 of § 1 arrives as h3 and no. 2 of the same § as h2. That is
// what a conversation transcribed by hand looks like and it is not worth
// failing an import over, so the depth is read and thrown away.
var headRE = regexp.MustCompile(`(?m)^#{1,6}\s+(?:\*\*)?(\d{1,3})\.\s+(.+?)\s*(?:\*\*)?$`)

// labelAlt is a label the volume gives to something it states.
//
// Two families, both measured over the 357 pages of Theory of Sets that are
// read. The criteria are printed as a bare label and a full stop: 61 C, 23 CST,
// 13 CF, 12 CS, 8 S and 4 A. The statements are printed in small caps and read
// back capitalised: 104 Proposition, 69 Corollary, 51 Definition, 16 Theorem
// and 9 Lemma. Remarks and examples are left out although the volume prints
// plenty of both, because they are numbered per no. rather than per section and
// a transcription that drops the number off a single Remark would fail an
// import for nothing.
const labelAlt = `(?:(C(?:ST|F|S)?\d{1,3}|S\d{1,3}|A\d{1,3})|(?i:(Proposition|Corollary|Definition|Theorem|Lemma)\s+(\d{1,3})))`

// A label is looked for in the two shapes it is written in, and the shape is
// not part of the question.
//
// The volume runs the label into the head of the paragraph it states: "C3. Let
// A be a theorem...". A transcription often lifts it into a heading of its own
// instead, and § 2 of chapter I arrives with "### C3" and the statement under
// it. Reading only the printed shape reported C2 to C5 missing from a section
// that states all four, which is the kind of finding that teaches a reader to
// ignore the audit.
// The parenthesis in the paragraph shape is the name a criterion is sometimes
// given: the volume prints "C1 (Syllogism). Let A and B be relations...", and
// four of the criteria in chapter I carry a name like that.
var (
	paraLabelRE = regexp.MustCompile(`(?m)^[*_>\s]{0,6}` + labelAlt + `(?:\s*\([^)\n]{1,40}\))?\.`)
	headLabelRE = regexp.MustCompile(`(?m)^#{1,6}\s+(?:\*\*)?` + labelAlt + `\b`)
)

// Audit holds one import against one printed section.
func Audit(t Target, body string, p Printed) *Result {
	r := &Result{Target: t, Numbers: len(p.Numbers), Pages: len(p.Pages)}
	r.auditNumbers(body, p)
	r.auditLabels(body, p)
	r.auditPages(body, p)
	return r
}

// auditNumbers checks the no. of the section, in order.
func (r *Result) auditNumbers(body string, p Printed) {
	var got []Numbered
	for _, m := range headRE.FindAllStringSubmatch(body, -1) {
		no, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		title := strings.TrimSpace(m[2])
		// The § itself is a numbered head with the same shape as the no. under
		// it, and three of the four imports open with one. It is dropped where
		// both its number and its title are the section's, which is narrow
		// enough that a no. that happens to share a number with its § is only
		// dropped when it also shares its title, and there is no such no. in
		// this volume.
		if no == p.Section && sameTitle(title, p.Title) {
			continue
		}
		got = append(got, Numbered{No: no, Title: title})
	}

	want := map[int]Numbered{}
	for _, n := range p.Numbers {
		want[n.No] = n
	}
	have := map[int]Numbered{}
	for _, n := range got {
		have[n.No] = n
	}
	for _, n := range p.Numbers {
		g, ok := have[n.No]
		if !ok {
			r.add("numbering", true, fmt.Sprintf("no. %d, %s, is not in the import", n.No, n.Title))
			continue
		}
		if !sameTitle(g.Title, n.Title) {
			r.add("numbering", false, fmt.Sprintf("no. %d is titled %q in the import and %q in the contents",
				n.No, g.Title, n.Title))
		}
	}
	for _, g := range got {
		if _, ok := want[g.No]; !ok {
			r.add("numbering", true, fmt.Sprintf("the import has a no. %d, %s, which the contents does not list",
				g.No, g.Title))
		}
	}
	// Order is checked over the numbers that are on both sides, so that a
	// missing no. is reported once as missing rather than again as a break in
	// the order.
	var seq []int
	for _, g := range got {
		if _, ok := want[g.No]; ok {
			seq = append(seq, g.No)
		}
	}
	if !sorted(seq) {
		r.add("numbering", true, fmt.Sprintf("the no. of the import run %s, and the book runs them in order", join(seq)))
	}
}

// auditLabels checks the labels the volume prints against the ones the import
// carries.
func (r *Result) auditLabels(body string, p Printed) {
	want := map[string]int{}
	var order []string
	for _, pg := range p.Pages {
		for _, l := range labels(pg.Text) {
			if want[l] == 0 {
				order = append(order, l)
			}
			want[l]++
		}
	}
	r.Labels = len(order)
	have := map[string]int{}
	for _, l := range labels(body) {
		have[l]++
	}
	for _, l := range order {
		if have[l] == 0 {
			r.add("labels", true, fmt.Sprintf("%s is printed in the section and is not in the import", l))
		}
	}
	var extra []string
	for l := range have {
		if want[l] == 0 {
			extra = append(extra, l)
		}
	}
	sort.Strings(extra)
	for _, l := range extra {
		// Not hard. A label the pages do not carry is as likely to be a page
		// this reading of the volume misread as it is to be something the
		// import invented, and the audit cannot tell which from here.
		r.add("labels", false, fmt.Sprintf("%s is in the import and is not printed in the section as the pages have it", l))
	}
}

// auditPages looks for every page of the section in the import.
//
// A page is looked for by its prose and never by its formulae. Two readings of
// the same page agree on the words and disagree on the markup around them: one
// writes \mathscr{T} where the other writes \mathcal{T}, one sets a display and
// the other runs it into the line. Matching on the words is what makes the
// question answerable at all, and it is why the measure is shingles rather than
// a diff: a shingle that survives is a run of eight words that both readings
// have in the same order, which is not something a transcription produces by
// accident.
func (r *Result) auditPages(body string, p Printed) {
	seen := shingles(body)
	for _, pg := range p.Pages {
		want := shingles(pg.Text)
		if len(want) == 0 {
			// A page with no continuous prose on it, which in this volume means
			// a page that is all displayed formulae, or a blank. There is
			// nothing here to look for and saying so is better than scoring it
			// zero and failing the import.
			r.add("pages", false, fmt.Sprintf("pdf page %d has no prose to look for", pg.PDFPage))
			continue
		}
		n := 0
		for s := range want {
			if seen[s] {
				n++
			}
		}
		frac := float64(n) / float64(len(want))
		switch {
		case frac < MinPage:
			r.add("pages", true, fmt.Sprintf("pdf page %d is %.0f%% in the import, and a page of the section has to be in it",
				pg.PDFPage, frac*100))
		case frac < ThinPage:
			r.Thin++
			r.add("pages", false, fmt.Sprintf("pdf page %d is %.0f%% in the import", pg.PDFPage, frac*100))
		}
	}
}

// MinPage is how much of a page's prose has to be found before the page counts
// as present, and ThinPage is where it is worth remarking on.
//
// Measured over the three Theory of Sets sections that are imported, 21 pages.
// A page the import carries scores between 0.63 and 1.00; the two lowest are
// the two pages of § 3 that are mostly displayed formulae and have little prose
// on them to match. The same sections held against the eight pages of a § they
// are not on score between 0.000 and 0.011, the stray hundredth being a run of
// eight words of ordinary mathematical English that turns up twice in a
// chapter. So there is a gap of sixty points between present and absent and the
// threshold is not delicate. It sits low in the gap rather than in the middle
// because the two mistakes do not cost the same: a page wrongly called missing
// sends a person to look at a page that is fine, and a page wrongly called
// present is the one thing this audit exists to catch.
const (
	MinPage  = 0.35
	ThinPage = 0.70
)

func (r *Result) add(rule string, hard bool, text string) {
	r.Findings = append(r.Findings, Finding{Rule: rule, Hard: hard, Text: text})
}

// labels reads the labels a piece of text states, in the order it states them.
func labels(s string) []string {
	type at struct {
		pos   int
		label string
	}
	var found []at
	for _, re := range []*regexp.Regexp{paraLabelRE, headLabelRE} {
		for _, m := range re.FindAllStringSubmatchIndex(s, -1) {
			found = append(found, at{pos: m[0], label: labelOf(s, m)})
		}
	}
	sort.Slice(found, func(i, j int) bool { return found[i].pos < found[j].pos })
	out := make([]string, len(found))
	for i, f := range found {
		out[i] = f.label
	}
	return out
}

// labelOf names one match, with the statements written the one way so that
// PROPOSITION, Proposition and proposition are one label and not three.
func labelOf(s string, m []int) string {
	if m[2] >= 0 {
		return s[m[2]:m[3]]
	}
	kind := s[m[4]:m[5]]
	kind = strings.ToUpper(kind[:1]) + strings.ToLower(kind[1:])
	return kind + " " + s[m[6]:m[7]]
}

// shingleLen is how many words make a shingle.
//
// Eight. At four, ordinary mathematical English repeats itself often enough
// that a page matches a section it is not on; at sixteen, one word read
// differently takes sixteen shingles with it and a page that is plainly present
// scores badly. Eight was measured against both failures on the imported
// sections.
const shingleLen = 8

// shingles is the runs of words in a text, with the mathematics taken out.
func shingles(s string) map[string]bool {
	w := words(s)
	out := make(map[string]bool, len(w))
	for i := 0; i+shingleLen <= len(w); i++ {
		out[strings.Join(w[i:i+shingleLen], " ")] = true
	}
	return out
}

var (
	mathRE  = regexp.MustCompile(`(?s)\$\$.*?\$\$|\$[^$\n]*\$`)
	frontRE = regexp.MustCompile(`(?s)\A---\n.*?\n---\n`)
	wordRE  = regexp.MustCompile(`[a-z]+`)

	// proseRE is a macro whose whole purpose is to set prose inside a formula.
	// \text{ or } is the word or, printed as a word and read as a word by
	// anybody looking at the page.
	proseRE = regexp.MustCompile(`\\(?:text|textrm|textit|textup|mbox|operatorname\*?)\s*\{([^{}]*)\}`)
)

// words is the prose of a text, lowercased, with the front matter, the
// mathematics and everything that is not a letter removed.
//
// The prose set inside a formula stays. The two sides disagree about where a
// formula ends: the printed page of criterion C10 sets "A or (not A)" with the
// or and the not as words between three short formulae, and the import writes
// the whole thing as one formula with \text{ or } and \operatorname{not} inside
// it. Both render the same line of type and a reader cannot tell them apart.
// Dropping the argument of those macros along with the mathematics made the two
// readings differ in every connective, which on § 3 of chapter I was twelve
// sentences of one import reported as printed nowhere in the book. They are
// printed on the page, in words, and the macro names say so.
func words(s string) []string {
	s = frontRE.ReplaceAllString(s, "")
	// The prose comes out of the formula as the formula goes, and not before
	// it: a \text{ or } sits inside the dollars, so lifting it first and
	// stripping afterwards would take it away again with everything else.
	s = mathRE.ReplaceAllStringFunc(s, func(m string) string {
		out := " "
		for _, g := range proseRE.FindAllStringSubmatch(m, -1) {
			out += g[1] + " "
		}
		return out
	})
	s = proseRE.ReplaceAllString(s, " $1 ")
	return wordRE.FindAllString(strings.ToLower(s), -1)
}

// sameTitle compares two headings the way a person would: on their words.
//
// The volume sets its headings in small caps, the contents sets them in upper
// and lower case, and a transcription writes down whichever it saw. None of
// that is a difference in the book.
func sameTitle(a, b string) bool {
	return strings.Join(words(a), " ") == strings.Join(words(b), " ")
}

func sorted(n []int) bool {
	for i := 1; i < len(n); i++ {
		if n[i] < n[i-1] {
			return false
		}
	}
	return true
}

func join(n []int) string {
	s := make([]string, len(n))
	for i, x := range n {
		s[i] = strconv.Itoa(x)
	}
	return strings.Join(s, ", ")
}
