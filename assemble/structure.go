package assemble

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// A section of Bourbaki has three levels under its own heading. It is divided
// into numbered no., it states Definitions, Propositions, Theorems, Lemmas,
// Corollaries, Remarks, Examples and one Scholium, and it ends in its
// exercises. Extraction writes the first and the third as headings, because
// they are set as headings on the page. The statements are not: the book runs
// them into the text, in bold, with an em dash after the number, and it is here
// that they become something a reader can link to.

// subsecRE is a no. heading as extraction writes it: "### 3. Simple Modules".
// The star marks a subsection the book sets as supplementary, and § 21 has one.
var subsecRE = regexp.MustCompile(`^### (?:\\\*)?(\d+)\. (.+)$`)

// subsections reads the no. headings of a piece.
//
// The number and the page are checked against the table of contents by Verify;
// the title is taken from here and not from there. Both are the same title, but
// the contents is set one line to an entry with the mathematics in it
// flattened, so it lists no. 6 of § 11 as "The Grothendieck Group RK (A)" where
// the page carries "The Grothendieck Group $R_K(A)$". Ten of the 146 in chapter
// VIII differ that way and the page is right in all ten.
func subsections(parts []part) []corpus.Subsection {
	var out []corpus.Subsection
	for _, p := range parts {
		for _, line := range strings.Split(p.body, "\n") {
			m := subsecRE.FindStringSubmatch(line)
			if m == nil {
				continue
			}
			n, _ := strconv.Atoi(m[1])
			s := corpus.Subsection{Number: n, Title: m[2], PDFPage: p.page}
			if l, ok := corpus.ParsePageLabel(p.label); ok {
				s.Page = l.Page
			}
			out = append(out, s)
		}
	}
	return out
}

// Verify checks a piece against the table of contents entry it was assembled
// from.
//
// This is the second half of the cross-check that begins with the section
// heading. The contents says § 11 has twelve no. and puts no. 7 on page 213; if
// the pages do not say the same then either the contents was read wrong or a
// page of the section is missing, and both are worth stopping for.
func (p Piece) Verify() error {
	if p.Front || p.Historical {
		return nil
	}
	want := p.Section.Subsections
	if len(p.Subsections) != len(want) {
		return fmt.Errorf("%s: the pages carry %d no., the table of contents lists %d",
			p.Name(), len(p.Subsections), len(want))
	}
	for i, s := range p.Subsections {
		if s.Number != want[i].Number {
			return fmt.Errorf("%s: no. %d on pdf page %d, the table of contents has no. %d there",
				p.Name(), s.Number, s.PDFPage, want[i].Number)
		}
		if s.PDFPage != want[i].PDFPage {
			return fmt.Errorf("%s: no. %d is on pdf page %d, the table of contents puts it on %d",
				p.Name(), s.Number, s.PDFPage, want[i].PDFPage)
		}
	}
	if p.Section.Exercises != nil && !p.HasExercise {
		return fmt.Errorf("%s: the table of contents gives exercises on page %d and the pages carry none",
			p.Name(), p.Section.Exercises.PDFPage)
	}
	if p.Section.Exercises == nil && p.HasExercise {
		return fmt.Errorf("%s: the pages carry exercises and the table of contents gives none", p.Name())
	}
	return nil
}

// headRE is a statement as the book prints it and extraction writes it: the
// kind in bold, a number for most of them, a full stop inside the bold, and an
// em dash before the statement itself.
//
// The plurals are the book's own. It sets "Remarks. —" over a run of remarks
// and "Examples. —" over a run of examples, 51 times in chapter VIII, and each
// of those is one statement rather than the first of several.
//
// The second branch is the same head with the bold lost and an attribution in
// its place: "Theorem 1 (Wedderburn). —". Bourbaki names the author of 18
// statements in chapter VIII that way and extraction dropped the bold on every
// one of them, while dropping it on none of the 631 heads that carry no
// attribution, so the parenthesis is what threw it. The attribution is required
// rather than optional here for exactly that reason: an undecorated "Theorem 1.
// —" is a shape the pages never produce, and reading one as a head would put
// the whole grammar at the mercy of a sentence that happens to open on a kind.
// These 18 were the largest hole left in the reference graph after the page
// joins were fixed, 55 unresolved references between them, headed by the
// thirteen that ask for Theorem 1 of § 7.
//
// The third branch is the head of a statement set in small type. Bourbaki
// brackets a passage that leans on a Book the reader has not reached with a
// star at each end, and extraction writes the star as $*$ because it is set as
// a mathematical asterisk. Eight passages of chapter VIII open that way and two
// of them open on a statement. The star is put back at the front of the body
// rather than dropped, both because it is the book's own mark and because the
// one at the far end would otherwise be left without its pair.
var headRE = regexp.MustCompile(
	`^(?:\*\*(` + kindAlt + `)(?: (\d+))?\.\*\*|(` + kindAlt + `)(?: (\d+))? \([^)]*\)\.|` +
		smallType + `(` + kindAlt + `)(?: (\d+))?\.)\s*—\s*`)

// smallType is the mark that opens a passage set in small type.
const smallType = `\$\*\$`

const kindAlt = `Definitions?|Propositions?|Theorems?|Lemmas?|Corollary|Corollaries|Remarks?|Examples?|Scholium`

// A run of remarks or examples is set under one head and numbered inside it, so
// no. 7 of § 16 prints "Remarks. — 2)", then "3)" as a paragraph of its own,
// then a Definition, then "Remarks. — 4)". Only the head is in bold, so without
// reading the numbers the second and third members of a run would be the only
// statements of the chapter with nowhere to point. The members are found with
// exNumRE, the same reading the exercises get, because the book marks them the
// same way: § 1 no. 1 sets its third example as "$*3)$", the asterisk saying it
// draws on something the reader has not reached yet.
//
// The numbering carries on across whatever stands between, so a run left open
// at 5 picks up the paragraph opening "6)" three propositions later. What
// closes a run is a heading, which is what keeps the exercises, whose blocks
// open "1)" as well, out of it. In chapter VIII, 44 runs carry on past their
// head and give 85 of the 649 statements.

// statements turns every statement of a piece into a heading of its own and
// returns a reference to each, in the order they are printed.
//
// The book runs a statement into the text, which is right for reading and wrong
// for everything else: a statement is the unit this corpus tags, translates,
// cites and solves against, so it has to be addressable. What comes out is
//
//	#### Proposition 6 {#alg-viii-s1-prop-6 .statement}
//
// followed by the statement itself. The bold head and the em dash go, since the
// heading now says what they said, and the label is permanent: it names the
// section and the number, both of which are the book's own, and nothing about
// where the statement happens to sit in the file.
//
// An unnumbered statement is labelled by the no. it stands in and its place in
// that no., which is the rule of spec 01 §4.2 and the reason the no. headings
// are read here as well. Chapter VIII has 151 of them, mostly Corollaries and
// Remarks.
//
// A numbered statement is labelled by whatever its Kind is counted within, and
// that is not the same for every Kind: see Kind.Scope. The one that needs
// keeping track of here is the Corollary, which is numbered under the statement
// it hangs from, so the last numbered Definition, Proposition, Theorem, Lemma
// or Scholium is carried along as the run goes.
func statements(blocks []block, id corpus.Ref) ([]block, []corpus.Statement, error) {
	out := make([]block, 0, len(blocks)*2)
	var found []corpus.Statement
	seen := map[string]bool{}
	no := 0
	var parent corpus.Ref // the last statement a Corollary could be numbered under
	var run corpus.Ref    // the run of remarks or examples now open, if any
	next := 0             // the number the next member of that run would carry
	occ := map[corpus.Ref]int{}
	for _, b := range blocks {
		if m := subsecRE.FindStringSubmatch(b.text); m != nil {
			no, _ = strconv.Atoi(m[1])
			next = 0
			out = append(out, b)
			continue
		}
		if strings.HasPrefix(b.text, "#") {
			next = 0
			out = append(out, b)
			continue
		}
		r, body, ok, err := statementAt(b.text, id, no, parent, run, next, occ)
		if err != nil {
			return nil, nil, err
		}
		if !ok {
			out = append(out, b)
			continue
		}
		if r.Number > 0 {
			switch r.Kind.Scope() {
			case corpus.ScopeSection:
				parent = r
			case corpus.ScopeSubsec:
				run, next = r, r.Number+1
			}
		}
		label := r.Label()
		if seen[label] {
			return nil, nil, fmt.Errorf("two statements are labelled %s", label)
		}
		seen[label] = true
		s := corpus.Statement{Ref: r, PDFPage: b.page, Body: body}
		if l, ok := corpus.ParsePageLabel(b.label); ok {
			s.Page = l.Page
		}
		found = append(found, s)
		out = append(out, block{text: heading(r, label), page: b.page, last: b.last, label: b.label})
		if body != "" {
			out = append(out, block{text: body, page: b.page, last: b.last, label: b.label})
		}
	}
	return out, found, nil
}

// heading is the heading a statement is given.
//
// The word is the corpus's singular, not the one printed: a run of remarks is
// headed "Remarks. —" in the book and each of its members comes out as a Remark
// of its own here, because each of them is one statement and has one label.
func heading(r corpus.Ref, label string) string {
	head := r.Kind.Heading()
	if r.Number > 0 {
		head += fmt.Sprintf(" %d", r.Number)
	}
	return fmt.Sprintf("#### %s {#%s .statement}", head, label)
}

// statementAt reads one block as a statement, and returns false if it is not
// one. body is the statement with the head taken off.
func statementAt(text string, id corpus.Ref, no int, parent, run corpus.Ref, next int,
	occ map[corpus.Ref]int) (corpus.Ref, string, bool, error) {
	m := headRE.FindStringSubmatch(text)
	if m == nil {
		// A paragraph opening on the number the open run is up to is the next
		// member of that run.
		i := exNumRE.FindStringSubmatch(text)
		if next == 0 || i == nil || i[3] != strconv.Itoa(next) {
			return corpus.Ref{}, "", false, nil
		}
		r := run
		r.Number = next
		return r, strings.TrimSpace(text[len(i[0]):]), true, nil
	}
	// The head matched one of headRE's three branches and left the others empty.
	word, num := m[1]+m[3]+m[5], m[2]+m[4]+m[6]
	kind, ok := corpus.KindFromHeading(word)
	if !ok {
		return corpus.Ref{}, "", false, fmt.Errorf("nothing in the corpus is called a %q", word)
	}
	r := id
	r.Kind = kind
	rest := text[len(m[0]):]
	if m[5] != "" {
		rest = "$*$" + rest
	}
	if num == "" && kind.Scope() == corpus.ScopeSubsec {
		// A run is headed by its kind in the plural and numbered inside:
		// "Remarks. — 2)" is Remark 2 and the head carries no number of its own.
		if i := exNumRE.FindStringSubmatch(rest); i != nil {
			num, rest = i[3], rest[len(i[0]):]
		}
	}
	// An unnumbered statement has no number to be numbered under, so it is
	// named by where it stands: the no. and how many of its kind came before it
	// there. That holds for an unnumbered Corollary too. Naming it under its
	// parent instead was tried and dropped, because "the second unnumbered
	// corollary of Theorem 1" and "Corollary 2 of Theorem 1" come out as the
	// same string, and the 45 unnumbered corollaries of chapter VIII include a
	// pair in § 1 that collide that way.
	if num == "" {
		key := r
		key.Subsec = no
		occ[key]++
		r.Subsec, r.Occurrence = no, occ[key]
		return r, strings.TrimSpace(rest), true, nil
	}
	r.Number, _ = strconv.Atoi(num)
	switch kind.Scope() {
	case corpus.ScopeSubsec:
		r.Subsec = no
	case corpus.ScopeParent:
		if parent.Number == 0 {
			return corpus.Ref{}, "", false, fmt.Errorf(
				"%s %d stands under no statement it could be numbered under", kind.Heading(), r.Number)
		}
		r.ParentKind, r.ParentNumber = parent.Kind, parent.Number
	}
	return r, strings.TrimSpace(rest), true, nil
}

// exercisesHead is the heading extraction writes over the exercises of a §,
// which the book prints as EXERCISES.
const exercisesHead = "### Exercises"

// anchorExercises gives the block of exercises an anchor, so that a reference
// to "VIII, p. 15, Exercise 9" has somewhere to point before the exercises are
// split out into files of their own.
func anchorExercises(blocks []block, id corpus.Ref) ([]block, bool) {
	found := false
	for i, b := range blocks {
		if b.text != exercisesHead {
			continue
		}
		blocks[i].text = fmt.Sprintf("%s {#%s-exercises}", exercisesHead, id.SectionLabel())
		found = true
	}
	return blocks, found
}

// exNumRE is how an exercise opens: the number, a closing parenthesis, and the
// exercise.
//
// Bourbaki marks two things about an exercise before it numbers it. A pilcrow
// says it is one of the harder ones, and an asterisk says it draws on results
// the reader has not reached yet. Chapter VIII sets 84 markers this way: 75
// with a pilcrow, 8 with an asterisk, and one with both. All 76 of the pilcrows
// open an exercise; only 3 of the 9 asterisks do, the other 6 opening a member
// of a run of Examples or Remarks, which the book marks the same way.
// Extraction writes them as the mathematics they were set in, "$\P 15)$" and
// "$*19)$", because that is what the volume's text layer holds, and the
// exercise is not found at all if they are not read.
var exNumRE = regexp.MustCompile(`^(?:\$\s*(\*)?\s*(\\P)?\s*)?(\d+)\)\$?\s`)

// itemOpen says a paragraph opens on a numbered item rather than on prose.
func itemOpen(s string) bool { return exNumRE.MatchString(s) }

// exercises reads the exercises of a § out of the assembled blocks.
//
// The number is not enough on its own to say where one exercise ends and the
// next begins, because the parts of an exercise are lettered a), b), c) and its
// cases are numbered (i), (ii), and both of those open a line as well. What
// makes it work is that the numbering only ever goes up by one: a block opening
// "9) " after exercise 8 is exercise 9, and a block opening "2) " inside
// exercise 12 is part of exercise 12. Bourbaki numbers the exercises of a §
// from one straight through, which is what makes the rule true.
//
// Two of the 317 exercises of chapter VIII do not open a block of their own,
// because the page sets them straight after the end of the one before with no
// break the extractor can see: exercise 13 of § 11 and exercise 15 of § 16. So
// each block is searched through rather than only looked at, and the search is
// for one number and not for any number, which is what keeps the "13)" of a
// cross-reference out of it.
func exercises(blocks []block) ([]corpus.Exercise, error) {
	var out []corpus.Exercise
	in := false
	for _, b := range blocks {
		if strings.HasPrefix(b.text, exercisesHead) {
			in = true
			continue
		}
		if !in {
			continue
		}
		if strings.HasPrefix(b.text, "#") {
			// Nothing of a § comes after its exercises, so a heading here is
			// something the section boundaries got wrong.
			return nil, fmt.Errorf("the exercises are followed by the heading %q", b.text)
		}
		text := b.text
		for text != "" {
			i, m := itemStart(text, len(out)+1)
			if i < 0 {
				if len(out) == 0 {
					return nil, fmt.Errorf("the exercises open on %q, which is not exercise 1", first(text, 40))
				}
				out[len(out)-1].Body += "\n\n" + text
				onPage(&out[len(out)-1], b)
				break
			}
			if head := strings.TrimSpace(text[:i]); head != "" {
				if len(out) == 0 {
					return nil, fmt.Errorf("the exercises open on %q, which is not exercise 1", first(head, 40))
				}
				out[len(out)-1].Body += "\n\n" + head
				onPage(&out[len(out)-1], b)
			}
			e := corpus.Exercise{Pages: spanning(nil, b)}
			e.Meta.Exercise = len(out) + 1
			e.Meta.Supplementary = m[1] != ""
			e.Meta.Starred = m[2] != ""
			e.Meta.PDFPage = b.page
			if l, ok := corpus.ParsePageLabel(b.label); ok {
				e.Meta.BookPage = l.String()
			}
			out = append(out, e)
			text = strings.TrimSpace(text[i+len(m[0]):])
		}
	}
	return out, nil
}

// onPage records that an exercise runs on into another block, and so on to
// whatever pages that block covers.
func onPage(e *corpus.Exercise, b block) {
	e.Pages = spanning(e.Pages, b)
}

// inlineNumRE is the number of an exercise set inside a paragraph rather than
// at the head of one. The book's marks come along with it, and by the time it
// reaches here the asterisk that closes the passage before it has come along
// too: § 16 runs "are bijective.$*$ $15)*$a) Let A be a regular integral
// domain".
var inlineNumRE = regexp.MustCompile(`[\s$*]\$?\*?\s*(\d+)\)\*?\$?\s*`)

// itemStart is where exercise n begins in a block, and the marker that opens
// it, or -1 when the block does not begin it.
//
// A number inside a paragraph only counts when it is the number the § is up to
// and it comes after the end of a sentence. Both conditions are needed. Without
// the first, "(argue as in Exercise 11, c))" starts an exercise; without the
// second, so does "(VIII, p. 210, Exercise 13)".
func itemStart(text string, n int) (int, []string) {
	if m := exNumRE.FindStringSubmatch(text); m != nil {
		if got, _ := strconv.Atoi(m[3]); got == n {
			return 0, m
		}
	}
	for off := 0; off < len(text); {
		loc := inlineNumRE.FindStringSubmatchIndex(text[off:])
		if loc == nil {
			return -1, nil
		}
		at := off + loc[0] + 1 // the byte matched before the number is not part of it
		got, _ := strconv.Atoi(text[off+loc[2] : off+loc[3]])
		if got == n && sentenceEnd(text[:at]) {
			m := exNumRE.FindStringSubmatch(text[at:] + " ")
			if m == nil {
				// The marker is set in mathematics the head-of-block form does
				// not cover, "$15)*$", so take what the inline form matched and
				// read the two marks straight off it. Only what stands in front
				// of the number is a mark on this exercise: § 16 sets its
				// fifteenth as "$15)*$" and that asterisk closes the passage the
				// exercise before it ended in, which is why the book prints
				// "$*19)$" and never "$19)*$" when it means the mark.
				raw, marks := text[at:off+loc[1]], text[at:off+loc[2]]
				m = []string{raw, markOf(marks, "*"), markOf(marks, `\P`), strconv.Itoa(got)}
			}
			return at, m
		}
		off += loc[1]
	}
	return -1, nil
}

// markOf is c when the marker carries it.
func markOf(raw, c string) string {
	if strings.Contains(raw, c) {
		return c
	}
	return ""
}

// sentenceEnd reports whether the text before a number is the end of a
// sentence, with the marks the book closes a passage with taken off.
func sentenceEnd(s string) bool {
	s = strings.TrimRight(s, " $*")
	return strings.HasSuffix(s, ".") || strings.HasSuffix(s, ")")
}

func first(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
