package refs

import (
	"os"
	"regexp"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// This file works out which page of the book each statement is printed on.
//
// The § is not enough. § 12 prints three statements called Corollary 1 and § 9
// prints six, and a citation to one of them says a page and nothing else:
// "Corollary 1 of VIII, p. 215". Narrowing by the no. the page falls in is what
// the resolver did before this, and it is the wrong tool twice over. A page
// that starts a no. carries the tail of the one before it, so p. 215 reads as
// no. 3 while the corollary meant is the last statement of no. 2, and where two
// statements of one number stand in one no., as § 5 does on p. 82 and p. 83,
// the no. cannot separate them at all. Between them those two shapes were ten
// of the fifteen references the audit could not follow.
//
// The page each statement stands on is not written anywhere in content/. It
// was known once: assembly read the pages and knew which one each statement
// came off, and then wrote the section and threw it away. Rather than change
// what assembly writes and reassemble the chapter, this reads it back out of
// pages/, which is committed, by lining the statements of a § up against the
// statement leads printed on its pages.
//
// The two sequences are the same statements in the same order, since one was
// built from the other, so lining them up is a longest common subsequence and
// nothing cleverer. What they differ by is small and known: the book sets a run
// of remarks under one lead, "Remarks. —", and the corpus gives each member of
// the run a statement and a tag of its own, and a handful of leads lost their
// bold in extraction. Measured on Algebra VIII this places 707 of the 709
// statements.

// lead is one statement lead printed on one page.
type lead struct {
	kind   corpus.Kind
	number int
	page   int
	// plural is a lead that stands for a run: "Remarks. —" heads what the
	// corpus holds as Remark 1, Remark 2 and Remark 3.
	plural bool
}

// printedLeadRE is a statement as the book sets it: the word, its number if it has
// one, the name in brackets if the statement is somebody's, and then the dash
// Bourbaki puts before the statement itself. The bold is optional because
// extraction loses it now and then, and the dash is not, because without it
// this matches any sentence that opens with the word Proposition.
var printedLeadRE = regexp.MustCompile(`^(?:\*\*)?([A-Z][a-zé]+)\s*(\d*)\s*(?:\([^)]*\))?\s*\.?(?:\*\*)?\s*(?:—|--)`)

// readLeads reads the statement leads off the pages a § is printed on, in
// reading order, each with the page the book prints it on.
func readLeads(root, book string, first, last int) []lead {
	var out []lead
	for n := first; n <= last; n++ {
		b, err := os.ReadFile(corpus.PagePath(root, book, n))
		if err != nil {
			continue // a page that was never read is a gap and not an error here
		}
		f, err := corpus.ParseFile[corpus.PageFrontMatter](b)
		if err != nil {
			continue
		}
		label, ok := corpus.ParsePageLabel(f.Meta.PageLabel)
		if !ok {
			continue // no printed page number, so nothing here can be pointed at
		}
		for _, line := range strings.Split(f.Body, "\n") {
			m := printedLeadRE.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			k, ok := corpus.KindFromHeading(m[1])
			if !ok || k == corpus.KindExercise {
				continue
			}
			number, _ := strconv.Atoi(m[2])
			out = append(out, lead{kind: k, number: number, page: label.Page,
				plural: !strings.EqualFold(m[1], k.Heading())})
		}
	}
	return out
}

// placePages writes the printed page onto each statement of a § it can find one
// for. A statement it cannot place keeps page 0, which the resolver reads as
// unknown rather than as a page.
//
// The headings are the § own statements as its Markdown heads them, in the same
// order as the statements themselves. They carry the number the book printed
// rather than the one in the label, and the two are not the same: a Remark the
// book set with no number at all is labelled rem-1, because a no. can hold
// several and they need addresses, and the lead on the page says only "Remark".
func placePages(s *Section, headings, leads []lead) {
	pairs := align(headings, leads)
	placed := map[int]bool{}
	for _, p := range pairs {
		if page := leads[p.lead].page; s.Holds(page) {
			s.Statements[p.stmt].Page = page
			placed[p.stmt] = true
		}
	}
	// A run set under one lead leaves its second and later members unmatched,
	// with the lead they belong to sitting in the gap. Every member of the run
	// is printed where the lead is, or near enough: the pages are what a
	// citation names, and a run does not carry citations to its members anyway.
	for i, st := range s.Statements {
		if placed[i] {
			continue
		}
		lo, hi := gap(pairs, i, len(leads))
		for j := lo; j < hi; j++ {
			l := leads[j]
			if l.plural && l.kind == headings[i].kind && s.Holds(l.page) {
				st.Page = l.page
				break
			}
		}
	}
}

// gap is the run of leads that no statement before or after this one was
// matched to, which is where the lead for an unmatched statement has to be if
// it is anywhere.
func gap(pairs []pair, i, n int) (int, int) {
	lo, hi := 0, n
	for _, p := range pairs {
		if p.stmt < i && p.lead >= lo {
			lo = p.lead + 1
		}
		if p.stmt > i && p.lead < hi {
			hi = p.lead
		}
	}
	return lo, hi
}

type pair struct{ stmt, lead int }

// align is the longest common subsequence of the statements of a § and the
// leads printed on its pages, matched on the kind and the number. Being a
// subsequence in both, it is in reading order on both sides by construction, so
// no placement it makes can put a statement on a page before the one it put the
// statement above it on.
func align(sts, leads []lead) []pair {
	n, m := len(sts), len(leads)
	// table[i][j] is the length of the longest common subsequence of the first
	// i statements and the first j leads.
	table := make([][]int, n+1)
	for i := range table {
		table[i] = make([]int, m+1)
	}
	for i := 1; i <= n; i++ {
		for j := 1; j <= m; j++ {
			if same(sts[i-1], leads[j-1]) {
				table[i][j] = table[i-1][j-1] + 1
				continue
			}
			table[i][j] = max(table[i-1][j], table[i][j-1])
		}
	}
	var out []pair
	for i, j := n, m; i > 0 && j > 0; {
		switch {
		case same(sts[i-1], leads[j-1]) && table[i][j] == table[i-1][j-1]+1:
			out = append(out, pair{i - 1, j - 1})
			i, j = i-1, j-1
		case table[i-1][j] >= table[i][j-1]:
			i--
		default:
			j--
		}
	}
	return out
}

func same(st, l lead) bool {
	return !l.plural && st.kind == l.kind && st.number == l.number
}
