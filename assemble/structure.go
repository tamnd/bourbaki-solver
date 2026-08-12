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

// A statement as the book prints it and extraction writes it is the kind in
// bold, a number for most of them, a full stop inside the bold, and an em dash
// before the statement itself. The expression that reads one is per printing:
// see printing.head, and the note there on the head with no bold on it.
//
// The second branch is the same head with the bold lost and an attribution in
// its place: "Theorem 1 (Wedderburn). —". Bourbaki names the author of 18
// statements in chapter VIII that way and extraction dropped the bold on every
// one of them, while dropping it on none of the 631 heads that carry no
// attribution, so the parenthesis is what threw it. The French printing does
// the same, for the same 18 statements.
//
// The third branch is the head of a statement set in small type. Bourbaki
// brackets a passage that leans on a Book the reader has not reached with a
// star at each end, and extraction writes the star as $*$ because it is set as
// a mathematical asterisk. Eight passages of chapter VIII open that way and two
// of them open on a statement. The star is put back at the front of the body
// rather than dropped, both because it is the book's own mark and because the
// one at the far end would otherwise be left without its pair.

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
func statements(blocks []block, id corpus.Ref, pr printing) ([]block, []corpus.Statement, error) {
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
		r, body, ok, err := statementAt(b.text, id, no, parent, run, next, occ, pr)
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
		out = append(out, block{text: heading(r, label, pr), page: b.page, last: b.last, label: b.label})
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
func heading(r corpus.Ref, label string, pr printing) string {
	head := r.Kind.HeadingIn(pr.lang)
	if r.Number > 0 {
		head += fmt.Sprintf(" %d", r.Number)
	}
	return fmt.Sprintf("#### %s {#%s .statement}", head, label)
}

// statementAt reads one block as a statement, and returns false if it is not
// one. body is the statement with the head taken off.
func statementAt(text string, id corpus.Ref, no int, parent, run corpus.Ref, next int,
	occ map[corpus.Ref]int, pr printing) (corpus.Ref, string, bool, error) {
	// Extraction writes the dangerous bend at the head of the passage it marks,
	// and a marked passage often opens on a statement: the French printing marks
	// Remark 2 of § 7 no. 1, two of the Examples of § 9 no. 2 and the Remark of
	// § 16 no. 8 that way. The sign is a mark on the whole statement and not part
	// of its head, so it comes off before the head is read and goes back at the
	// front of the body, the same as the star.
	bent := false
	if s, ok := strings.CutPrefix(text, corpus.Bend); ok {
		bent, text = true, strings.TrimLeft(s, " ")
	}
	body := func(s string) string {
		if !bent {
			return s
		}
		return strings.TrimSpace(corpus.Bend + " " + s)
	}
	m := pr.head.FindStringSubmatch(text)
	if m == nil {
		// A paragraph opening on the number the open run is up to is the next
		// member of that run.
		i := exNumRE.FindStringSubmatch(text)
		if next == 0 || i == nil || i[3] != strconv.Itoa(next) {
			return corpus.Ref{}, "", false, nil
		}
		r := run
		r.Number = next
		return r, body(afterMarker(i[0], text[markerLen(i):])), true, nil
	}
	// The head matched one of the branches of pr.head and left the others empty.
	word, num := m[1]+m[3]+m[5], m[2]+m[4]+m[6]
	if len(m) > 7 {
		word, num = word+m[7], num+m[8]
	}
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
			num, rest = i[3], afterMarker(i[0], rest[markerLen(i):])
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
		return r, body(strings.TrimSpace(rest)), true, nil
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
	return r, body(strings.TrimSpace(rest)), true, nil
}

// anchorExercises gives the block of exercises an anchor, so that a reference
// to "VIII, p. 15, Exercise 9" has somewhere to point before the exercises are
// split out into files of their own.
func anchorExercises(blocks []block, id corpus.Ref, pr printing) ([]block, bool) {
	found := false
	for i, b := range blocks {
		if b.text != pr.exercises {
			continue
		}
		blocks[i].text = fmt.Sprintf("%s {#%s-exercises}", pr.exercises, id.SectionLabel())
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
//
// What follows the number is a space, or the a) of its first part run straight
// on: the French printing sets four of its exercises as "$\P 3)$a) Soient K un
// corps commutatif", with nothing between the marker and the part. The space is
// what keeps a number inside a sentence from opening an exercise, so it is not
// simply dropped, and a lettered part is the one other thing that can stand
// there. Without this, § 14 of the French chapter reported three exercises
// where the volume prints nineteen: the reader looks for one number and one
// only, so a marker it cannot see stops the thread for the rest of the §.
// An asterisk after the number is read too, and is not a mark on the exercise:
// it closes the passage the exercise before it ended in, and it falls inside
// the same math span as the number because the text layer opened the span at
// the number. § 16 sets its fifteenth as "$15)*$a)", and the reader, which
// looks for one number and one only, stopped there and lost 15, 16 and 17.
//
// The dollar that closes the marker's span is the third thing that can stand
// after the number, and it is as good a separator as a space: § 6 of the French
// chapter sets the second of its examples "$2)*$Soient A une $k$-algèbre", with
// the number, the asterisk and the closing dollar run straight into the word.
var exNumRE = regexp.MustCompile(`^(?:\$\s*(\*)?\s*(\\P)?\s*)?(\d+)\)(?:\*?\$|(\s|[a-z]\)))`)

// markerLen is how much of a block the marker takes, which is not always the
// whole of what matched: a lettered part is matched to prove the number opens
// an item and then left where it stands, since it is the first line of the item
// and not part of its number.
func markerLen(m []string) int {
	n := len(m[0])
	if len(m) > 4 && strings.TrimSpace(m[4]) != "" {
		n -= len(m[4])
	}
	return n
}

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
func exercises(blocks []block, pr printing) ([]corpus.Exercise, error) {
	var out []corpus.Exercise
	in := false
	for _, b := range blocks {
		if strings.HasPrefix(b.text, pr.exercises) {
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
			text = afterMarker(m[0], text[i+markerLen(m):])
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
				m = []string{raw, markOf(marks, "*"), markOf(marks, `\P`), strconv.Itoa(got), ""}
			}
			return at, m
		}
		off += loc[1]
	}
	return -1, nil
}

// afterMarker is what is left of a paragraph once the marker numbering it has
// been taken off.
//
// It is not a slice, because the marker can be half of a math span. The text
// layer sets the pilcrow and the number as mathematics and the run it puts them
// in does not always stop where the number does, so the dollar that closes it
// is a few letters into the prose:
//
//	$\P 18) A$ group G is called locally finite if every subgroup ...
//
// Take the marker off and what is left opens a math span that nothing closes,
// which M01 reports against the exercise file and which makes the rest of the
// paragraph read as a formula. So the dollar the marker opened is taken off
// with it, wherever in the remainder it turns up. Four exercises of chapter
// VIII are set this way, in § 5, § 9 and § 10.
//
// Nothing is guessed here. The count says the marker opened a span and did not
// close it, and a span that is open has exactly one dollar that closes it,
// which is the first one after it.
func afterMarker(marker, rest string) string {
	if strings.Count(marker, "$")%2 == 1 {
		if i := strings.IndexByte(rest, '$'); i >= 0 {
			rest = rest[:i] + rest[i+1:]
		}
	}
	return strings.TrimSpace(rest)
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
