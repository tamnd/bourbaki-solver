package toc

import (
	"fmt"
	"regexp"
	"slices"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/typography"
)

// A chapter opens under two headings and a § opens under one, and they are the
// largest type on their pages. Level is about the two smaller ones, the § and
// the no., which are printed alike and are told apart by the contents. These
// are not printed like anything else in the volume and the reading still loses
// them, in three ways: it keeps the words and drops the level, it keeps the
// words and misreads the number, or it drops the line altogether.
//
// The first two are repaired here and the third is not. What the two have in
// common is that the words are still on the page, so the contents has something
// to agree with and the repair is the same one Level makes: the page keeps its
// own words and the contents settles what level they are at. Where the line is
// gone there is nothing to agree with, and putting the heading back would mean
// writing onto a page a line the reading never saw. That is a re-reading of the
// page image and not a repair of the Markdown.

// ChapterWord is the word a printing sets over the number of a chapter, which
// is the one thing assemble looks for before a volume begins.
//
// It is here rather than taken from assemble's printings because a repair of
// pages/ cannot depend on the package that reads pages/ into content/, and
// because this is the whole of what a printing contributes: the number is the
// contents' and the title is the page's.
func ChapterWord(lang string) string {
	if lang == "fr" {
		return "CHAPITRE"
	}
	return "CHAPTER"
}

// ChapterOpening puts back the heading over a chapter whose page kept the title
// and lost everything above it.
//
// Page 22 of Theory of Sets opens the first chapter and came back as
//
//	Description
//	of Formal Mathematics
//
//	## 1. TERMS AND RELATIONS
//
// where the page prints CHAPTER I over a title set in the largest type in the
// volume. assemble finds a chapter by "## CHAPTER" and nothing else is one, so
// the volume does not begin and no part of it assembles.
//
// The title is what makes this safe. The contents gives the chapter a title and
// the page still carries it, in its own words and broken across the lines the
// press broke it across, so the two are put side by side and only a page whose
// opening lines say what the contents says is written to. The run has to start
// at the first line of the page, which is where a chapter title is and where
// nothing else is.
//
// The lines are joined with a space and set as one heading. The break between
// "Description" and "of Formal Mathematics" is the width of the measure and not
// part of the title, which is exactly what the contents is being asked. A line
// the join does not need is left where it stands: page 225 of Topology I to IV
// sets "Topological Groups" and "(Elementary Theory)" under it, the contents
// calls that chapter "Topological Groups", so the subtitle stays a line of the
// page rather than being taken into a heading the book does not give.
//
// The word may still be on the page. Pages 8, 56 and 141 of Groupes et algebres
// de Lie IV a VI open on "CHAPITRE IV", "CHAPITRE V" and "CHAPITRE VI" set as
// plain lines with the title under them, which is the same fault one line
// higher: the reading kept the words of both lines and the level of neither. So
// a first line that is the word and this chapter's own numeral and nothing else
// is read past and then written over, rather than left standing above the
// heading where it would be the only page in the volume carrying the word twice.
func ChapterOpening(body []string, lang, numeral, title string) ([]string, bool) {
	if numeral == "" {
		return body, false
	}
	at := blank(body, 0)
	i := at
	if i < len(body) && plainLine(body[i]) && normalize(body[i]) == normalize(ChapterWord(lang)+numeral) {
		i = blank(body, i+1)
	}
	j, head, ok := titleRun(body, i, title)
	if !ok {
		return body, false
	}
	out := make([]string, 0, len(body)+2)
	out = append(out, body[:at]...)
	out = append(out, "## "+ChapterWord(lang)+" "+numeral, "", "# "+head)
	out = append(out, body[j+1:]...)
	return out, true
}

// blank is the first line at or after i that has anything on it, or the end of
// the body where nothing does.
func blank(body []string, i int) int {
	for i < len(body) && strings.TrimSpace(body[i]) == "" {
		i++
	}
	return i
}

// titleRun is the run of plain lines starting at i that together say what the
// contents says the title is. It gives the line the run ends on and the title
// as one line.
//
// The lines are joined with a space. A press breaks a title at the measure and
// the reading keeps the break, so the title of a chapter is often two lines and
// neither of them says what the contents says while the two together do. The
// comparison is flattened on both sides, which is what lets a page that sets
// the title in capitals agree with a contents that sets it in mixed case.
//
// A line the join does not need is left where it stands. Page 225 of Topology I
// to IV sets a subtitle under the chapter title and the contents does not have
// it, so the run stops at the title and the subtitle stays a line of the page
// rather than being taken into a heading the book does not give.
func titleRun(body []string, i int, title string) (int, string, bool) {
	if flatten(title) == "" {
		return 0, "", false
	}
	var run []string
	for j := i; j < len(body) && strings.TrimSpace(body[j]) != "" && plainLine(body[j]); j++ {
		run = append(run, strings.TrimSpace(body[j]))
		if joined := strings.Join(run, " "); same(title, joined) {
			return j, joined, true
		}
	}
	return 0, "", false
}

// plainLine is a line with no markup at the front of it, which is what a
// heading the reading took for prose looks like. A line that is already a
// heading is not one of these and neither is a line that opens in mathematics
// or in bold, since both of those carry a decision the reading made and this
// is for the lines where it made none.
func plainLine(s string) bool {
	return !strings.HasPrefix(s, "#") && !strings.HasPrefix(s, "$") && !strings.HasPrefix(s, ">")
}

// lostSection is a § heading with no level on it: the number, a full stop and
// the title, with the section sign in front of it where the printing sets one
// and the bold the reading sometimes puts round the whole line.
//
// The number is taken as characters and not as a number, because the character
// is the thing that was misread. See sectionNumber.
//
// The space after the sign is optional because the printing does not always set
// one and the reading does not always keep one. Integration VII to IX has
// "§1. CONSTRUCTION OF A HAAR MEASURE" on page 7 and every one of its thirteen
// § openings is set that way, so requiring the space refused the lot of them and
// left the volume unassembled. What goes back is normalised, see sectionSign.
//
// The bold is allowed in two places because the reading puts it in two places.
// Page 103 of Topology I to IV has "**10. PROPER MAPPINGS**", the whole line in
// bold, and page 137 of Algebre commutative chapitres 8 et 9 has
// "3. **Existence et unicité des $ p $-anneaux**", the title alone. Only the
// outer pair was written down at first, and since the comparison against the
// contents throws away everything that is not a letter or a digit, the second
// shape matched anyway: the closing pair was taken off the end and the opening
// one stayed on the front of the title, and the heading that went back carried
// one half of a pair of asterisks. Both are stripped now.
//
// The single asterisk in front is not bold and is not stripped. It is the star
// Bourbaki opens a passage with that the reader may leave until later, it is
// printed on the paper, and it belongs to the no. rather than to the title.
// Page 222 of Espaces vectoriels topologiques I a V sets "*4. Cas des espaces de
// fonctions continues bornees" and its page image shows the star in the same
// bold as the number. The corpus already writes six of these at level, page 218
// of the English printing of that volume among them, and writes them "### \*4."
// with the star escaped so that Markdown does not read it as the start of an
// emphasis. See star for what goes back.
var lostSection = regexp.MustCompile(`^(?:(\*\*)|(\\?\*))?(§ *)?([0-9A-Za-z]{1,4})\. +(?:\*\*)?(.+?)(\*\*)?$`)

// star is what goes in front of the number of a heading the printing starred,
// given what the page had in front of it.
//
// The star goes back escaped whether or not the page escaped it, for the same
// reason the section sign goes back spaced: the assembler reads one shape, which
// is the shape the corpus already holds. See assemble.subsecRE.
func star(had string) string {
	if had == "" {
		return ""
	}
	return `\*`
}

// sectionSign is what goes in front of the number of a § heading, given what the
// page had in front of it.
//
// The page decides whether there is a sign at all, since Algebra I to III prints
// one and Topology I to IV does not, and the assembler reads either. It does not
// decide the spacing: the corpus sets "## § 1." in all 143 § headings that carry
// a sign, so a page that ran the sign into the number gets the space put in
// rather than carried through.
func sectionSign(had string) string {
	if had == "" {
		return ""
	}
	return "§ "
}

// SectionOpening puts back the heading over a § whose page kept the title and
// lost the level, the number, or both. It gives the run of lines it replaces,
// from and to inclusive, and the one line that replaces them, and false where
// the page has nothing the contents can agree with.
//
// Four shapes turn up and the contents settles all four the same way. Page 10
// of Algebra IV to VII has "§ 1. POLYNOMIALS", which is the heading with the
// hashes gone. Page 103 of Topology I to IV has "**10. PROPER MAPPINGS**",
// which is the heading in bold, and that page also carries "**1. PROPER
// MAPPINGS**" twelve lines down, the first no. of the same §, under the same
// title. Page 23 of the same volume has "I. OPEN SETS, NEIGHBOURHOODS, CLOSED
// SETS", which is § 1 with the digit read as a letter, and page 113 has "II.
// CONNECTEDNESS", which is § 11 read the same way twice. Page 267 has
//
//  5. INFINITE SUMS
//     IN COMMUTATIVE GROUPS
//
// which is one heading the press broke at the measure and the reading kept
// broken, so it is joined the way a chapter title is joined.
//
// The number and the title both have to agree with the contents, which is what
// tells the § from its own first no. on page 103 and what keeps this off the
// lines of "I. THEORY OF SETS" and "II. ALGEBRA" that page 16 of the same
// volume sets when it lists the Books of the Éléments. Those are on a page the
// contents does not open a § on, so they are never looked at.
//
// The number is written back as the contents gives it and the title as the page
// gives it. The sign is kept where the page has one: Algebra I to III and
// Algebra IV to VII print it, Theory of Sets and Topology I to IV set the
// number alone, and the assembler reads either.
func SectionOpening(body []string, number int, title string) (int, int, string, bool) {
	return opening(body, number, title, "## ", true, false)
}

// SectionOpeningFromHead puts back the heading over a § whose page kept no line
// of it, where the reading filed the heading as the page's running head. It
// gives the heading to write and says nothing about where, since the words are
// not in the body at all and there is nowhere in it they were taken from.
//
// A § opening page carries no running head. It is the first page of the § and
// the press sets the heading in display type at the top of it, so what stands
// at the top of that page is the heading and not a head. A reading that takes
// the top line of every page for the running head is right on every page but
// this one, and on this one it files the heading away where the assembler will
// never look for it. General Topology V to X is read that way throughout: page
// 18 has running_head "2. MEASUREMENT OF MAGNITUDES", which is § 2 of chapter V
// word for word as the contents gives it, and a body that opens on the first
// paragraph of the § with no heading over it.
//
// The number is what makes this safe, and it is the same number the body path
// already requires. A running head carries the title and nothing else, which is
// what turned page 177 of Topologie generale V a X away from ESPACES POLONAIS ;
// ESPACES SOUSLINIENS when WitnessSection read the head of that page. A head
// that carries a number as well is not a running head, and a head that carries
// the number the contents gives on the one page the contents opens that § on,
// under a title the contents agrees with, is that § heading and not another
// line of the book.
func SectionOpeningFromHead(running string, number int, title string) (string, bool) {
	_, _, head, ok := opening([]string{strings.TrimSpace(running)}, number, title, "## ", true, false)
	return head, ok
}

// SectionOpeningFromLocatedHead is SectionOpeningFromHead on a page that filed
// the number of the § somewhere else again.
//
// SectionOpeningFromHead wants the number in the running head, and says why: a
// running head carries the title alone, so a head that carries the number as
// well is not a running head but the § heading in the wrong field. Two pages of
// this corpus split the heading between two fields instead. Page 64 of
// Topologie generale I a IV has running_head "ESPACES SEPARES ET ESPACES
// REGULIERS", which is § 8 of chapter I word for word as the contents gives it,
// and locator.section 8 under it; page 39 of Algebre chapitre 9 is the same
// with § 2. Both bodies open on the first no. of the § with nothing over it.
//
// The number is on the page either way, and what it is doing is the same thing
// in both: saying which § the head belongs to. So the two fields are put back
// together and the ordinary test is run on the result. The title still has to
// agree with the contents exactly, and the locator still has to give the number
// the contents gives, on the one page the contents opens that § on.
func SectionOpeningFromLocatedHead(running string, number int, title string) (string, bool) {
	head := strings.TrimSpace(running)
	if head == "" {
		return "", false
	}
	_, _, out, ok := opening([]string{fmt.Sprintf("§ %d. %s", number, head)}, number, title, "## ", true, false)
	return out, ok
}

// NumberOpening puts back the heading over a no. whose page kept the title and
// lost the level. It is the same repair as SectionOpening one level down, and
// it is the same fault: page 32 of Theory of Sets sets PROOFS as its running
// head and "2. PROOFS" under it, and the reading kept one of the two.
//
// A no. is told from the § it belongs to by the sign. The printings that set a
// sign set it over the § and never over a no., so a line that carries one is
// refused here, and a printing that sets no sign at all leaves the number and
// the title to do the telling, which is what they do everywhere else in this
// file. Where the § has already been put back it is a heading by then and no
// heading is looked at twice.
//
// The comparison against the contents is the loose one and the § above it keeps
// the exact one. That is not an inconsistency, it is where the person is. A § a
// page and the contents spell differently is reported by name, both spellings
// given, and somebody settles it: see the differ count in fix.go. A no. has no
// such branch. It is reported as having no line the contents can read as its
// heading, which on all thirteen no. of Algebra I to III that it refused was not
// true, since every one of those headings was on its page and was turned away
// over a plural, a dropped article, a misread letter or a formula set two ways.
// A refusal that is a dead end has to be right about more than a refusal that is
// a question, and this one was not.
//
// Nothing is written that the page did not print either way. The heading goes
// back in the page's own words and the contents only settles the number and the
// level, which is the rule this whole file works by.
func NumberOpening(body []string, number int, title string) (int, int, string, bool) {
	return opening(body, number, title, "### ", false, true)
}

// opening is the run of lines a heading was set on and the heading that goes
// back over them. level is the hashes the assembler reads it at, and sign is
// whether the printing is allowed to set § in front of the number.
//
// The run that scores best wins rather than the first run that agrees. An exact
// agreement scores 1 and nothing beats it, so a page that used to match still
// matches on the same line as before, and the search only carries on past a run
// that is merely close. Taking the first close run instead would let a one line
// run at 0.86 beat the two line run at 0.99 that is the actual heading, since
// the press broke long titles at the measure and the reading kept them broken.
func opening(body []string, number int, title, level string, sign, loose bool) (int, int, string, bool) {
	if flatten(title) == "" {
		return 0, 0, "", false
	}
	floor := 1.0
	if loose {
		floor = titleFloor
	}
	best, from, to, head := 0.0, 0, 0, ""
	for i, line := range body {
		if !plainLine(line) {
			continue
		}
		m := lostSection.FindStringSubmatch(line)
		if m == nil || !sectionNumber(m[4], number) {
			continue
		}
		if m[3] != "" && !sign {
			continue
		}
		run := []string{m[5]}
		for j := i; ; j++ {
			joined := strings.Join(run, " ")
			if s := titleScore(title, joined); s > best {
				best, from, to = s, i, j
				head = level + star(m[2]) + sectionSign(m[3]) + strconv.Itoa(number) + ". " + joined
			}
			if j+1 >= len(body) || strings.TrimSpace(body[j+1]) == "" || !plainLine(body[j+1]) {
				break
			}
			run = append(run, strings.TrimSpace(body[j+1]))
		}
	}
	if best < floor {
		return 0, 0, "", false
	}
	return from, to, head, true
}

// titleFloor is how close a title on a page has to be to the title in the
// contents before the two are taken for the same title.
//
// 0.85 is measured rather than chosen. The thirteen no. of Algebra I to III
// that fix opening refused are all of them headings that are on their pages and
// were refused for the wording, and under the rules below they score 0.88, 0.89,
// 0.96, 0.97, 0.97, 0.97 and 1.00 six times. The floor sits under the lowest of
// those with room to spare and well above what an unrelated title scores.
//
// What makes a low floor safe here is that it is not the only test. The line has
// already had to carry the number the contents gives, on a page the contents
// opens that no. on, and a § sign disqualifies a no. outright. A wrong match
// needs a differently titled line numbered the same on the one page the contents
// points at, and that is a page with two headings on it, not a near miss.
const titleFloor = 0.85

// titleScore is how far a title a page prints agrees with the title the contents
// gives, from 0 to 1. Anything the old exact rule accepted scores 1.
//
// The old rule was flatten equality both ways, and every one of the thirteen
// refusals in Algebra I to III fails it for a reason that has nothing to do with
// the page being wrong. They fall in three groups.
//
// The page prints different words for the same thing. The contents has "Change
// of the ring of scalars" and page 245 prints CHANGE OF RING OF SCALARS; the
// contents has "Tensor product of vector spaces" and page 330 prints TENSOR
// PRODUCTS, plural. Neither is a defect anybody should fix, because a contents
// and a page really do word a title differently, and the reading of one of them
// is not more right than the reading of the other.
//
// One of the two readings has a typo in it. Page 594 prints FUNCTIORIAL for
// functorial, and the contents entry for page 453 reads Subaigebras, which is
// SUBALGEBRAS with the l read as an i. A single letter should not cost a
// heading.
//
// The title is mostly mathematics and the two sides set the mathematics
// differently. The contents entry for page 291 came off the contents pages as
// "HomB(E 0A F, G)-+ HomA(F, HomB(E, G))" and the page sets the same thing as
// LaTeX. Flattening cannot bring those together and no threshold on the whole
// string will either, which is what the prose test below is for: both sides say
// "The isomorphisms" before the mathematics starts, and that is the part either
// reading can be trusted on.
func titleScore(want, got string) float64 {
	if same(want, got) {
		return 1
	}
	w, g := mathless(want), mathless(got)
	// Everything below scores under 1 and never at it, so that an exact
	// agreement further down the page always wins. The press broke long titles
	// at the measure and the reading kept them broken, so the first line of a
	// two line heading is a strict prefix of the whole, and a rule that scored
	// it 1 would stop the search on half a title. That is what
	// "5. INFINITE SUMS" over "IN COMMUTATIVE GROUPS" is.
	//
	// A title that is a title plus a word or two. Page 521 prints DEFINITION OF
	// THE SYMMETRIC ALGEBRA OF A MODULE over a no. the contents lists as
	// "Symmetric algebra of a module", so one side contains the other whole.
	if len(w) >= titleEnough && len(g) >= titleEnough && (strings.Contains(w, g) || strings.Contains(g, w)) {
		return titleClose
	}
	// The prose in front of the mathematics, which is the part of a title full
	// of formulae that both readings get right.
	wp, gp := prose(want), prose(got)
	if len(wp) >= titleEnough && wp == gp {
		return titleClose
	}
	// The same test where only one side marks its mathematics up. The contents
	// of Algebra I to III came off the contents pages with the formulae as run
	// together letters and no dollars in them, so no. 1 of chapter II § 4 reads
	// "The isomorphisms HomB(E 0A F, G)-+ ..." there and the page sets the same
	// no. as THE ISOMORPHISMS and then LaTeX. Nothing above brings those
	// together: the page's key loses everything between the dollars and the
	// contents key keeps its own mangled version of it, so they share the prose
	// and disagree on all the rest.
	//
	// The prose is still the whole of what both readings can be held to, and on
	// the side that marks its mathematics it is a prefix of the title rather
	// than the title. So the test is prefix rather than equality, one way round
	// only, and against the other side's letters.
	if len(gp) >= titleEnough && strings.HasPrefix(w, gp) {
		return titleClose
	}
	if len(wp) >= titleEnough && strings.HasPrefix(g, wp) {
		return titleClose
	}
	// How alike the two are, read twice. Once with the formulae taken out, which
	// is the reading the tests above are built on, and once with the letters
	// inside the formulae left standing.
	//
	// The second reading is there for the pair where only one side marked its
	// formulae up at all. mathless takes a formula out of the side that marked
	// it and leaves the other side's letters where they are, so the two come out
	// unlike each other over a difference that is no part of either title. Page
	// III.16 of Espaces vectoriels topologiques I a V heads no. 3 "Relations
	// entre $ \mathcal{L}(E ; F) $ et $ \mathcal{L}(\hat{E} ; F) $" and the
	// contents of that same volume lists it "Relation entre L (E ; F) et
	// L (Ê ; F)", so the press is a letter apart from itself and everything else
	// the title says is in the formulae. Read as letters the two are two edits
	// apart. Read mathless they are six, and the six are the formulae the
	// contents did not mark.
	//
	// The better of the two wins because each is blind to something the other
	// sees, and the floor is what turns away the pairs that are neither.
	s := 0.0
	if len(w) >= titleEnough && len(g) >= titleEnough {
		s = ratio(w, g)
	}
	wf, gf := flatten(clean(want)), flatten(clean(got))
	if len(wf) >= titleEnough && len(gf) >= titleEnough {
		s = max(s, ratio(wf, gf))
	}
	return s
}

// titleClose is what the two tests that are about part of a title score. It is
// under 1 so an exact agreement beats it and over the floor so it stands on its
// own when there is no exact agreement to be had.
const titleClose = 0.95

// titleEnough is the shortest run of letters worth comparing loosely. Below it
// a single letter is a large part of the string and the comparison says more
// about the length than about the words, so those titles are held to the exact
// rule they were always held to.
const titleEnough = 8

// same says whether a title a page prints and a title the contents gives are
// the same title. This is the exact test, and everything titleScore does below
// it is a matter of degree that only a no. is held to.
//
// It used to be flatten equality, and two habits of the press defeated it. A
// footnote hangs off the end of a heading, and flatten throws away the dollars
// and the caret around the marker while keeping the digit, so the two sides come
// out differing by a 1 that is no part of either title. A French printing sets
// the preposition "à" as a bare capital A, and flatten throws an accented letter
// away outright rather than folding it, so the accented a of the contents entry
// vanished while the plain A the page prints in its place stayed. Three openings
// in this corpus were refused over one or the other and all three were on their
// pages and correct.
//
// Both are set aside here and neither is touched on the page. The marker is ink
// and the heading that goes back keeps it.
func same(want, got string) bool {
	return flatten(clean(want)) == flatten(clean(got))
}

// clean is a title with the two things a printing varies freely in one set
// aside. See the typography package for what they are and why they are there
// rather than here: the assembler compares a heading against the same contents
// entry, on a flattening of its own, and runs into both of them too.
func clean(s string) string {
	return typography.Accentless(typography.Footless(s))
}

// dollars is a run of mathematics as the reading writes it.
var dollars = regexp.MustCompile(`\$[^$]*\$`)

// mathless is flatten with the mathematics taken out as well as the control
// words, so that a title comparison is over the words of the title.
func mathless(s string) string {
	return flatten(dollars.ReplaceAllString(s, " "))
}

// mathStart is where the mathematics of a title begins.
var mathStart = regexp.MustCompile(`\$|\\[a-zA-Z]`)

// prose is the words a title sets before its first formula.
func prose(s string) string {
	if i := mathStart.FindStringIndex(s); i != nil {
		s = s[:i[0]]
	}
	return normalize(s)
}

// ratio is how alike two strings are, 1 for the same and 0 for nothing in
// common, taken as the edit distance against the longer of the two.
func ratio(a, b string) float64 {
	x, y := []rune(a), []rune(b)
	n := max(len(x), len(y))
	if n == 0 {
		return 0
	}
	return 1 - float64(distance(x, y))/float64(n)
}

// distance is the Levenshtein distance, over two rows rather than the whole
// table because the titles are short and only the last row is wanted.
func distance(a, b []rune) int {
	prev := make([]int, len(b)+1)
	cur := make([]int, len(b)+1)
	for j := range prev {
		prev[j] = j
	}
	for i := 1; i <= len(a); i++ {
		cur[0] = i
		for j := 1; j <= len(b); j++ {
			sub := prev[j-1]
			if a[i-1] != b[j-1] {
				sub++
			}
			cur[j] = min(min(prev[j]+1, cur[j-1]+1), sub)
		}
		prev, cur = cur, prev
	}
	return prev[len(b)]
}

// appendixLine is the word a volume heads an appendix with, standing alone on a
// line, with the number after it where a chapter has more than one appendix and
// numbers them.
//
// The words are the four assemble reads, since the volume chooses. Integration
// VII to IX calls its appendix an ANNEX throughout and the French volumes set
// ANNEXE beside APPENDICE, and a repair that insisted on one word would leave
// the other five volumes unassembled.
//
// The bold is allowed because the reading puts it there, and the full stop
// because some printings set one. Nothing else is allowed on the line: the word
// alone is the whole of what a page prints over an appendix, and requiring that
// is what keeps this off the sentence in a preface that happens to mention one.
var appendixLine = regexp.MustCompile(`(?i)^ *(?:\*\*)? *(appendi[xc]e?|annexe?)\.? *(\d+|[ivxIVX]+)? *\.? *(?:\*\*)? *$`)

// AppendixOpening puts back the heading over an appendix whose page kept the
// word but lost the level, or whose reading filed the word as the running head.
// It gives the body with the heading in it and whether the running head is now
// spent and should be dropped.
//
// Thirty nine appendices are in the contents of this corpus and twenty four of
// them are marked. The other fifteen fail in two ways and this repairs both.
// Seven have the word standing as a plain line in the body, which is the same
// fault SectionOpening repairs one shape up: the reading kept the words and
// dropped the hashes, so the words are still there to agree with. Seven more
// have it only in the running head, which happens because the word is the whole
// of what the page prints over an appendix and a reading that files the top
// line of a page as its running head takes the heading with it. Page 402 of
// Algebra I to III is one: the front matter has APPENDIX and the body opens on
// the title of the appendix with nothing above it.
//
// The running head goes when it is used, and that is the part worth arguing
// for. A running head is what a page prints at the top of every page of a run,
// and a word printed once over the opening of an appendix is not that. Leaving
// it in both places would put the word on the page twice, once as the heading
// the assembler reads and once as a running head no other page of the appendix
// carries.
//
// The number has to agree with the contents where the contents gives one. Nine
// of the fifteen are the only appendix of their chapter and are unnumbered, and
// the assembler reads an unnumbered appendix by the word alone, so a number on
// the page where the contents gives none is a line this does not touch. Where
// there is a number the page may set it as a digit or as a roman numeral and
// both are read, since Topology V to X numbers its two appendices in one
// printing and Lie IX in another.
//
// An appendix whose word is nowhere on the page and nowhere in the front matter
// is not put back, for the reason the head of this file gives: there would be
// nothing for the contents to agree with, and writing the word in would mean
// putting a line on a page that no reading of it ever produced.
func AppendixOpening(body []string, running, title string, number int) ([]string, bool, bool) {
	for i, line := range body {
		if !plainLine(line) {
			continue
		}
		m := appendixLine.FindStringSubmatch(line)
		if m == nil || !appendixNumber(m[2], number) {
			continue
		}
		out := slices.Clone(body)
		out[i] = "## " + appendixHead(m[1], number)
		return under(out, i+1, title), false, true
	}
	m := appendixLine.FindStringSubmatch(strings.TrimSpace(running))
	if m == nil || !appendixNumber(m[2], number) {
		return body, false, false
	}
	i := blank(body, 0)
	out := make([]string, 0, len(body)+2)
	out = append(out, body[:i]...)
	out = append(out, "## "+appendixHead(m[1], number), "")
	out = append(out, body[i:]...)
	return under(out, i+2, title), true, true
}

// under puts the title of an appendix under the heading that was just written
// over it, where the page carries the title as a plain line and the contents
// agrees with it.
//
// An appendix is headed by its word alone and the title is set under the word
// in its own type, so the reading loses the level on two lines rather than one.
// The assembler reads the title from under the word and wants a heading there:
// page 402 of Algebra I to III has the word in the running head and
// PSEUDOMODULES as the first plain line of the body, and marking the word alone
// gets as far as the assembler saying the page titles the appendix nothing
// while the contents calls it Pseudomodules.
//
// Where there is no title to find the body comes back as it was, which is the
// case that has to keep working. Chapter IX of Algebre commutative chapitres 8
// et 9 closes on an appendix the contents gives no title at all: the page
// prints the word centred and alone and the next thing on it is the heading of
// its first no. Reading a title there would take that heading.
func under(body []string, i int, title string) []string {
	i = blank(body, i)
	j, head, ok := titleRun(body, i, title)
	if !ok {
		return body
	}
	out := make([]string, 0, len(body))
	out = append(out, body[:i]...)
	out = append(out, "# "+head)
	out = append(out, body[j+1:]...)
	return out
}

// appendixMark is the heading the assembler reads over an appendix, and it is
// laxer than appendixLine on purpose. The repair asks for the word standing
// alone because that is what an unmarked page prints and anything else on the
// line means the line is something other than a heading. This asks whether the
// page is already marked, and a page that is already marked has whatever the
// volume's own hand put there: Algebra VIII sets the word, the number and the
// title of the appendix all on one line, and four of its appendices came back
// as unmarked when this was written the strict way.
//
// The number is not checked here for the same reason the assembler does not
// check it: a chapter with one appendix has one heading over it and a heading
// with the word in it is that heading whatever follows the word.
var appendixMark = regexp.MustCompile(`(?i)^#{1,4} +(?:\*\*)? *` + appendixWord + `\b`)

// appendixWord is the four words a volume of this corpus heads an appendix
// with. It is the same list the assembler reads, since a repair that put back a
// word the assembler does not read would leave the volume unassembled.
const appendixWord = `(?:appendi[xc]e?|annexe?)`

// Appendix is whether the body already opens an appendix, which is the test
// that keeps the repair from writing a second heading onto a page that has one.
// Twenty four of the thirty nine appendices in this corpus are already marked
// and every one of them has to come through untouched.
func Appendix(body []string) bool {
	return slices.ContainsFunc(body, appendixMark.MatchString)
}

// appendixHead is the heading that goes back, which is the word the page prints
// and the number the contents gives.
//
// The word is upper cased because that is how the corpus sets the twenty four
// appendices that are already marked, and because the page prints it that way
// in every one of the fifteen this repairs. The number is the contents' own,
// written as a digit, since assemble reads either and one of the two has to be
// chosen for the corpus to be consistent with itself.
func appendixHead(word string, number int) string {
	head := strings.ToUpper(word)
	if number > 0 {
		head += " " + strconv.Itoa(number)
	}
	return head
}

// appendixNumber is whether what the page sets after the word is the number the
// contents gives the appendix.
//
// The empty case is the common one and it has to be exact in both directions. A
// chapter with one appendix does not number it and the contents gives it no
// number either, so nothing after the word is right and a number after it is a
// line that belongs to something else. A chapter with two numbers both and the
// page may set the number as a digit or as a roman numeral.
func appendixNumber(had string, number int) bool {
	if had == "" {
		return number == 0
	}
	if number == 0 {
		return false
	}
	if n, err := strconv.Atoi(had); err == nil {
		return n == number
	}
	return strings.EqualFold(had, roman(number))
}

// roman is a number written the way a printing sets the number of an appendix.
// Nothing in this corpus numbers more than two, so the table stops where the
// evidence does rather than where a general algorithm would.
func roman(n int) string {
	switch n {
	case 1:
		return "I"
	case 2:
		return "II"
	case 3:
		return "III"
	case 4:
		return "IV"
	}
	return ""
}

// RunningHeadOpening is the heading a reading filed as the running head of the
// page, for a page where the two carry the same words.
//
// A recto sets the title of the current no. at the head of the page and the
// heading of the no. right under it, so a page where a no. begins prints those
// words twice, once without the number and once with it. Page 32 of Theory of
// Sets heads the page PROOFS and opens no. 2 under "2. PROOFS", and page 69 of
// Algebra I to III does the same with PRODUCTS AND FIBRE PRODUCTS. The reading
// keeps one line of the two, files it as the running head, and the body loses
// its heading.
//
// What makes this a repair rather than a re-reading is that the line is still
// in the file. It is in the front matter instead of the body, and the number on
// it is the tell: a running head carries no number and a heading does, so a
// running head that reads as "2. Proofs" where the contents opens no. 2 under
// that title is the heading and not the running head. Both go back, the heading
// to the top of the page where the no. begins, and the running head to the
// words without the number, which is what the page prints over them.
//
// A page whose running head is anything else is left alone and reported. The
// chapter title, the title with no number on it, or a paragraph the reading
// swallowed whole are all cases where the heading is not in the file at all.
func RunningHeadOpening(runningHead string, number int, title string) (string, string, bool) {
	m := lostSection.FindStringSubmatch(strings.TrimSpace(runningHead))
	if m == nil || m[3] != "" || !sectionNumber(m[4], number) {
		return "", "", false
	}
	// The same comparison the body gets, and for the same reason: two of the
	// thirteen no. this repair was refusing had their heading in the running
	// head all along, and were turned away over the wording. See titleScore.
	if titleScore(title, m[5]) < titleFloor {
		return "", "", false
	}
	return "### " + strconv.Itoa(number) + ". " + m[5], m[5], true
}

// numbered is a heading that opens on a number, with whatever the printing
// sets between the hashes and the number.
var numbered = regexp.MustCompile(`^(#{2,4}) +(?:\*\*)?(?:\\?\*)?(?:§ *)?(\d+)\.`)

// Numbered is whether a page already carries a heading at this many hashes
// under this number, so that the repair leaves it alone.
//
// What comes between the hashes and the number is not part of either. § 21 no.
// 13 of Algebra VIII is a starred no., which the reading writes "### \*13.",
// and a heading the reading put bold round is "### **13.". Reading those as
// missing headings would send somebody to a page image over a heading that is
// on the page and correct.
func Numbered(body []string, hashes, number int) bool {
	for _, line := range body {
		m := numbered.FindStringSubmatch(line)
		if m == nil || len(m[1]) != hashes {
			continue
		}
		if n, err := strconv.Atoi(m[2]); err == nil && n == number {
			return true
		}
	}
	return false
}

// SectionTitle is what a page calls the § the contents numbers this way, for a
// page where the two do not agree.
//
// It is a report and not a repair. Page 36 of Algebra I to III heads § 2
// "IDENTITY ELEMENT; CANCELABLE ELEMENTS; INVERTIBLE ELEMENTS" and the contents
// spells the middle word with two l. One of the two is a misreading and the
// heading cannot be put back until somebody says which, so the two are printed
// side by side. That is a different piece of work from a heading the reading
// dropped, and telling them apart saves reading the page image for a fault that
// is not in the page image.
func SectionTitle(body []string, number int) (string, bool) {
	for _, line := range body {
		if !plainLine(line) {
			continue
		}
		if m := lostSection.FindStringSubmatch(line); m != nil && sectionNumber(m[4], number) {
			return m[5], true
		}
	}
	return "", false
}

// sectionNumber is whether what the page has in front of a § title is the
// number the contents gives it.
//
// A reading of a page image confuses the digit 1 with the letters I and l, and
// the digit 0 with the letter O, and it does it in the largest type as readily
// as anywhere else. § 1 of chapter I of Topology I to IV came back as "I." and
// § 11 of the same chapter as "II.", which is the same confusion twice in the
// same number. Reading those as roman numerals would make the second of them 2,
// so they are not roman numerals here: they are digits that were read as the
// letters they are shaped like, and turning the letters back is what the page
// means.
//
// Nothing is decided by this on its own. The title has to agree with the
// contents as well, and the page has to be the page the contents opens the §
// on, so a line that arrives at a number this way arrives at the right one.
func sectionNumber(s string, n int) bool {
	digits := strings.Map(func(r rune) rune {
		switch r {
		case 'I', 'l':
			return '1'
		case 'O':
			return '0'
		}
		return r
	}, s)
	got, err := strconv.Atoi(digits)
	return err == nil && got == n
}
