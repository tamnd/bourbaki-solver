package share

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"unicode"
)

// Read builds the sheet a person uses to hold one import against the printed
// pages, and it asks the question the audit does not.
//
// The audit asks three questions and all three run from the book to the import:
// is every no. of the contents there, is every label the pages print there, is
// every page of the § somewhere in it. Those catch a transcription that stopped
// early or skipped a page. None of them can see the other direction. A sentence
// in the import that is on no page of the book passes all three, and that is the
// failure this material is most likely to have, because what wrote it is a
// language model reading a photograph: a model that cannot make out a line does
// not leave a gap, it writes the sentence the book would most likely have there.
// A gap is visible to anybody. A fluent invention is visible to nobody, which is
// exactly why it is worth the work of looking for.
//
// So this is not a fourth audit rule. It is a reading sheet, and the difference
// matters. The two sides are two transcriptions of the same type set by two
// different processes, and they disagree constantly about markup, about where a
// heading ends, about whether a running head is text. A rule that failed a
// section over that would fail every section, and a rule tuned until it passed
// them all would see nothing. What a person can do and a threshold cannot is
// read a sentence and say whether it is in the book. This puts the twenty
// sentences worth reading in front of them instead of the four hundred.
//
// Nothing here is a verdict. A passage on this sheet is a place to look.

// MinPassage is how much of a sentence's runs of eight words the other side has
// to carry before the sentence counts as present.
//
// Half, because the two sides break their lines differently and a sentence that
// straddles a page turn in one and not the other loses the shingles that cross
// the break. Half a sentence found is a sentence that is there in some form, and
// what this is looking for is a sentence that is not there at all, which scores
// nothing rather than a little.
const MinPassage = 0.5

// A Passage is one sentence of one side, with how much of it the other carries.
type Passage struct {
	// PDFPage is the page it is printed on, and zero for a passage of the
	// import, which sits in a file and not on a page.
	PDFPage int
	Text    string
	Found   float64
}

// A PageReading is one printed page held against the whole import.
type PageReading struct {
	PDFPage int
	// Found is the page's coverage, the same number share audit prints.
	Found float64
	// Missing is the sentences of the page the import does not carry.
	Missing []Passage
	// Short is the sentences too short to judge, counted rather than listed.
	// A run of fewer than eight words makes no shingle and there is nothing to
	// measure, so it is left out and said out loud rather than scored zero.
	Short int
}

// A Sheet is one import file read against the pages of its section.
type Sheet struct {
	Target Target
	Pages  []PageReading
	// Added are the sentences of the import that no page of the section
	// carries. This is the half the audit cannot see.
	Added []Passage
	// AddedShort is the same count for the import side.
	AddedShort int
}

// Missing is how many sentences of the printed pages are not in the import.
func (s *Sheet) Missing() int {
	n := 0
	for _, p := range s.Pages {
		n += len(p.Missing)
	}
	return n
}

// Read holds one import file against the printed pages of its section, both
// ways round.
func Read(t Target, body string, p Printed) *Sheet {
	sheet := &Sheet{Target: t}
	inImport := shingles(body)
	// Every page of the section at once, because the import is one file and a
	// sentence of it may be printed on any page of the §, most often on the two
	// either side of a page turn. The page the next § starts on goes in too,
	// since this § ends partway down it: without that page the closing
	// paragraph of every § reads as something the import made up.
	onPage := map[string]bool{}
	for _, pg := range append(append([]PrintedPage{}, p.Pages...), p.After...) {
		for s := range shingles(pg.Text) {
			onPage[s] = true
		}
	}
	for _, pg := range p.Pages {
		r := PageReading{PDFPage: pg.PDFPage}
		want := shingles(pg.Text)
		if len(want) > 0 {
			n := 0
			for s := range want {
				if inImport[s] {
					n++
				}
			}
			r.Found = float64(n) / float64(len(want))
		}
		for _, sentence := range sentences(pg.Text) {
			got, ok := carried(sentence, inImport)
			if !ok {
				r.Short++
				continue
			}
			if got < MinPassage {
				r.Missing = append(r.Missing, Passage{PDFPage: pg.PDFPage, Text: sentence, Found: got})
			}
		}
		sheet.Pages = append(sheet.Pages, r)
	}
	for _, sentence := range sentences(stripFront(body)) {
		got, ok := carried(sentence, onPage)
		if !ok {
			sheet.AddedShort++
			continue
		}
		if got < MinPassage {
			sheet.Added = append(sheet.Added, Passage{Text: sentence, Found: got})
		}
	}
	return sheet
}

// carried is how much of a sentence the other side has, and whether the
// question could be asked at all. A sentence of fewer than eight words makes no
// shingle, and scoring it zero would put every short line on the sheet.
func carried(sentence string, other map[string]bool) (float64, bool) {
	w := words(sentence)
	if len(w) < shingleLen {
		return 0, false
	}
	total, found := 0, 0
	for i := 0; i+shingleLen <= len(w); i++ {
		total++
		if other[strings.Join(w[i:i+shingleLen], " ")] {
			found++
		}
	}
	if total == 0 {
		return 0, false
	}
	return float64(found) / float64(total), true
}

var (
	// sentenceEnd is a full stop, and it is looked for after the mathematics
	// has been masked, since a full stop inside a formula ends no sentence and
	// $f(x) = 1.$ would otherwise be two of them.
	sentenceEnd = regexp.MustCompile(`(?U)[.!?](\s+|$)`)
	spaceRun    = regexp.MustCompile(`\s+`)
	holeRE      = regexp.MustCompile("\x00[0-9]+\x00")
	holeAtEnd   = regexp.MustCompile("\x00([0-9]+)\x00$")
)

// stripFront takes the front matter off an import, which is provenance and not
// the book.
//
// frontRE rather than a second copy of it here. words already drops the head
// before it counts anything, so the two sides of this file would have gone on
// agreeing for a while and then quietly stopped the first time the head grew a
// field written in a way one pattern allowed and the other did not.
func stripFront(s string) string {
	return frontRE.ReplaceAllString(s, "")
}

// sentences cuts a text into the units a person reads.
//
// A paragraph is too coarse to be useful, since a paragraph of Bourbaki runs
// half a page and a sheet that says "this paragraph differs" has said nothing.
// A line is too fine and is an artefact of how each side wrapped its text. The
// sentence is the unit somebody can hold in their head while they look at the
// page.
//
// The mathematics comes out first and goes back after, because a full stop
// inside a formula ends no sentence. What comes back is the sentence as it
// stands in the file, formulae and all, since a person checking it against the
// page needs to see what it actually says. The whole text is masked at once and
// not one paragraph at a time, because a displayed formula is written with a
// blank line above and below it and masking per paragraph would cut it in half.
func sentences(text string) []string {
	masked, spans := maskMath(text)
	var out []string
	for _, para := range paragraphs(masked, spans) {
		for _, s := range cut(para) {
			s = strings.TrimSpace(unmaskMath(s, spans))
			if s != "" {
				out = append(out, spaceRun.ReplaceAllString(s, " "))
			}
		}
	}
	return out
}

// paragraphs is the masked text in paragraphs, with a displayed formula joined
// to the sentence it belongs to.
//
// Bourbaki writes a sentence, sets a formula on a line of its own in the middle
// of it, and finishes the sentence underneath. In Markdown that is three
// paragraphs, and cutting there leaves "is a theorem in T, and hence that" as a
// sentence of its own. Both sides do this and neither does it in the same
// place, so the fragments never match each other and every displayed formula in
// the section turns into two passages that look like differences and are not.
// Ten of the twenty-one on the first sheet of § 3 were this.
//
// So a paragraph that is nothing but a formula joins the one above it, and the
// paragraph after that one joins too when the sentence was left open. Narrow on
// purpose: joining on any paragraph that lacks a full stop would swallow every
// heading in the volume, since a heading ends in a word.
func paragraphs(s string, spans []string) []string {
	var out []string
	display := false
	for _, p := range blankLine.Split(s, -1) {
		p = strings.TrimSpace(p)
		if p == "" {
			continue
		}
		switch {
		case len(out) > 0 && onlyMath(p):
			out[len(out)-1] += " " + p
			display = true
		case len(out) > 0 && display && !endsSentence(out[len(out)-1], spans):
			out[len(out)-1] += " " + p
			display = false
		default:
			out = append(out, p)
			display = false
		}
	}
	return out
}

// onlyMath says whether a paragraph is a formula set on its own, punctuation
// and all. The punctuation counts as part of it: a displayed formula is very
// often followed by a comma on the same line.
func onlyMath(p string) bool {
	rest := strings.TrimFunc(holeRE.ReplaceAllString(p, ""), func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r)
	})
	return rest == "" && holeRE.MatchString(p)
}

// endsSentence says whether a paragraph closed the sentence it was carrying.
//
// A paragraph that ends in a displayed formula is asked of the formula itself,
// because the full stop that closes the sentence is very often set inside the
// dollars. "The assembly (A => B) and (B => A) will be denoted by A <=> B." is
// written with the stop after the B and inside the display, and without looking
// there the sentence reads as open and swallows the criterion printed under it.
func endsSentence(p string, spans []string) bool {
	p = strings.TrimRightFunc(p, unicode.IsSpace)
	if p == "" {
		return true
	}
	if m := holeAtEnd.FindStringSubmatch(p); m != nil {
		i, err := strconv.Atoi(m[1])
		if err != nil || i >= len(spans) {
			return false
		}
		return endsSentence(strings.Trim(spans[i], "$ \t\n"), nil)
	}
	return strings.ContainsRune(".!?", rune(p[len(p)-1]))
}

// cut splits a masked paragraph at its sentence ends, keeping the punctuation
// on the sentence it belongs to.
func cut(s string) []string {
	var out []string
	prev := 0
	for _, m := range sentenceEnd.FindAllStringIndex(s, -1) {
		if !closes(s[:m[0]]) {
			continue
		}
		out = append(out, s[prev:m[1]])
		prev = m[1]
	}
	if prev < len(s) {
		out = append(out, s[prev:])
	}
	return out
}

// abbrev is the words this library puts a full stop after in the middle of a
// sentence. no. is the one that matters: Bourbaki refers to a numbered no. of a
// § on nearly every page, and cutting there turned "by C1 (§ 2, no. 2)" into a
// sentence ending in no. and a sentence beginning in 2).
var abbrev = map[string]bool{
	"no": true, "nos": true, "cf": true, "ch": true, "chap": true,
	"p": true, "pp": true, "vol": true, "fig": true, "art": true,
	"resp": true, "etc": true, "e": true, "g": true, "i": true,
}

// closes says whether the stop at the end of head really ends a sentence.
//
// A numeral standing at the head of its paragraph with nothing but markup in
// front of it is a numbering and not a sentence: the heading "### 1. SIGNS AND
// ASSEMBLIES" is one heading, and cut at the stop it is a sentence reading
// "### 1." and another reading the title. A numeral with words in front of it
// is an ordinary end of sentence, since half the sentences in this book close
// on the name of an axiom.
func closes(head string) bool {
	i := len(head)
	for i > 0 && (isLetter(head[i-1]) || isDigit(head[i-1])) {
		i--
	}
	tok, before := head[i:], head[:i]
	if tok == "" {
		return true
	}
	if abbrev[strings.ToLower(tok)] {
		return false
	}
	if !allDigits(tok) {
		return true
	}
	return strings.IndexFunc(before, unicode.IsLetter) >= 0
}

func isLetter(b byte) bool { return b|0x20 >= 'a' && b|0x20 <= 'z' }

func isDigit(b byte) bool { return b >= '0' && b <= '9' }

func allDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if !isDigit(s[i]) {
			return false
		}
	}
	return true
}

const mathHole = "\x00%d\x00"

// maskMath swaps every formula for a placeholder that carries no punctuation.
func maskMath(s string) (string, []string) {
	var spans []string
	out := mathRE.ReplaceAllStringFunc(s, func(m string) string {
		spans = append(spans, m)
		return fmt.Sprintf(mathHole, len(spans)-1)
	})
	return out, spans
}

func unmaskMath(s string, spans []string) string {
	for i, m := range spans {
		s = strings.ReplaceAll(s, fmt.Sprintf(mathHole, i), m)
	}
	return s
}

// Markdown renders the sheet for a person to read.
//
// The pages come first and in order, because that is the order somebody with
// the book open reads in. The import's own sentences come last and together,
// since a person checking those has to search the printed § for each one and
// that is a different job done in a different way.
func (s *Sheet) Markdown(book string) string {
	var b strings.Builder
	fmt.Fprintf(&b, "# %s read against %s\n\n", s.Target.Path(), book)
	fmt.Fprintf(&b, "%s\n\n", s.Summary())
	fmt.Fprint(&b, "`share audit` has already held this import against the printed contents and the printed labels, and this sheet does not repeat that. What is here is the prose: the sentences of each printed page that the import does not carry, and the sentences of the import that no page of the section carries. A passage on this sheet is a place to look and not a fault. The two sides are two transcriptions of one piece of type and they disagree about markup, about headings and about running heads without either being wrong about the book.\n\n")
	for i, p := range s.Pages {
		fmt.Fprintf(&b, "## pdf page %d, %.0f%% of its prose in the import\n\n", p.PDFPage, p.Found*100)
		if i == 0 && len(p.Missing) > 0 {
			// The other end of the same boundary. This § opens partway down its
			// first page and the end of the § before it is printed above the
			// heading, so a passage here may belong to that § and be missing
			// from nothing. Said rather than filtered: which sentences are the
			// previous §'s cannot be known from this side, and a sheet that
			// quietly dropped the head of the page would hide a real gap.
			fmt.Fprint(&b, "The head of this page is the end of the previous §, printed above this §'s heading. A passage here may belong to that § and be missing from nothing.\n\n")
		}
		if len(p.Missing) == 0 {
			fmt.Fprintf(&b, "Nothing to look at. %s\n\n", shortNote(p.Short))
			continue
		}
		for _, m := range p.Missing {
			fmt.Fprintf(&b, "- %.0f%% found. %s\n", m.Found*100, m.Text)
		}
		if p.Short > 0 {
			fmt.Fprintf(&b, "\n%s\n", shortNote(p.Short))
		}
		fmt.Fprint(&b, "\n")
	}
	fmt.Fprint(&b, "## In the import and on no page of the section\n\n")
	fmt.Fprint(&b, "This is the half `share audit` cannot see. A transcription made by a model reading a photograph does not leave a gap where it could not read a line, it writes the sentence the book would most likely have there, and a fluent invention passes every check this project owns. Read each of these against the printed section before the import is promoted.\n\n")
	if len(s.Added) == 0 {
		fmt.Fprintf(&b, "Nothing. %s\n", shortNote(s.AddedShort))
		return b.String()
	}
	for _, a := range s.Added {
		fmt.Fprintf(&b, "- %.0f%% found. %s\n", a.Found*100, a.Text)
	}
	if s.AddedShort > 0 {
		fmt.Fprintf(&b, "\n%s\n", shortNote(s.AddedShort))
	}
	return b.String()
}

// Summary is the sheet in one line, for the command that writes it.
func (s *Sheet) Summary() string {
	return fmt.Sprintf("%d printed %s, %d %s of the pages not in the import, %d %s of the import on no page.",
		len(s.Pages), plural(len(s.Pages), "page", "pages"),
		s.Missing(), plural(s.Missing(), "sentence", "sentences"),
		len(s.Added), plural(len(s.Added), "sentence", "sentences"))
}

// shortNote says how many sentences were passed over, because a sheet that
// silently drops what it cannot measure reads like a sheet that measured
// everything.
func shortNote(n int) string {
	if n == 0 {
		return "Every sentence was long enough to measure."
	}
	return fmt.Sprintf("%d %s under eight words, too short to make a run and left out.",
		n, plural(n, "sentence", "sentences"))
}

func plural(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// SortPages puts the pages in printed order, which is the order a person reads
// them in and not the order they come out of a map.
func (s *Sheet) SortPages() {
	sort.Slice(s.Pages, func(i, j int) bool { return s.Pages[i].PDFPage < s.Pages[j].PDFPage })
}
