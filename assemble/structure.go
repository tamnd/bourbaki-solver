package assemble

import (
	"fmt"
	"regexp"
	"slices"
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
			// The page the book prints, from whichever of the two the volume
			// has. A labelled volume carries "A VIII.7" and a volume paginated
			// straight through carries a bare 7 at the foot.
			if l, ok := corpus.ParsePageLabel(p.label); ok {
				s.Page = l.Page
			} else {
				s.Page = p.folio
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
		// One page earlier than the contents gives is the printing and not a
		// fault. No. 5 of § 1 of chapter VI of Lie 4 to 6 is listed at page
		// 166, and its heading stands near the foot of 165 with one paragraph
		// under it and the rest of the no. over the page. The contents is
		// giving the page the no. is read on rather than the page its heading
		// is set on, which is a fair thing for an index to do.
		//
		// Later than the contents gives is still an error, and that is the
		// half of the check that has the teeth in it: a page of the section
		// missing pushes every heading after it down, never up.
		if d := want[i].PDFPage - s.PDFPage; d < 0 || d > 1 {
			return fmt.Errorf("%s: no. %d is on pdf page %d, the table of contents puts it on %d",
				p.Name(), s.Number, s.PDFPage, want[i].PDFPage)
		}
	}
	if p.Section.Exercises != nil && !p.HasExercise {
		return fmt.Errorf("%s: the table of contents gives exercises on page %d and the pages carry none",
			p.Name(), p.Section.Exercises.PDFPage)
	}
	// A run that begins and never gets to exercise 1 is a run whose first
	// marker was misread, and it reads as a long preamble and nothing else.
	// Without this it would go by quietly, since the heading is there and the
	// contents is satisfied by it.
	if p.Section.Exercises != nil && len(p.Exercises) == 0 {
		return fmt.Errorf("%s: the exercises begin on page %d and none of them was read",
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
// repeats are the statements a printing numbered twice, each one written as the
// label the two would share and the pdf pages the later of them is printed on.
//
// Two statements at one label is normally a fault and is meant to fail loudly:
// it caught the run of Remarques that Bourbaki restarts inside a no., and it
// caught the head grammar reading a French statement in capitals as prose. Both
// were real bugs that no audit rule would have found, so the check stays as it
// is, and a printing that really does repeat a number is written down here one
// entry at a time rather than being let through by a rule.
//
// Each page listed is a page somebody has looked at. The page is part of the
// test for the same reason: a reader that misreads a number produces the same
// collision, and pinning the exception to the page it was verified on means a
// re-reading that moves the statement fails loudly again instead of inheriting
// the excuse.
//
// The pages come as a list because a label names the book and not the volume, so
// the two printings of a book that this corpus holds in both languages arrive at
// the same label on different pages. Nothing else is loosened by that: a page
// still has to be one of the pages written down.
//
// lie-iii-s3-def-7 was the first entry. § 3 of chapter III of Groupes et
// algebres de Lie prints Definition 7 at no. 7 and DEFINITION 7 again at no. 12,
// and then Definition 8. See corpus.Ref.Repeated for the citations that settle
// which of the two the volume means when it says def. 7.
//
// evt-ii-s6-def-2 is the second. § 6 of chapter II of Espaces vectoriels
// topologiques prints Definition 2 twice, once at no. 2 on the page the running
// head calls TVS II.42, where it puts two vector spaces in duality by a bilinear
// form, and again at no. 3 on TVS II.44, where it defines the polar of a set.
// Both are on the paper in both printings, at pdf page 81 of the English and 82
// of the French.
//
// lie-vi-s1-lem-3 is the third, and only the French printing has it. § 1 of
// chapter VI of Groupes et algebres de Lie prints Lemme 3 at no. 7 and Lemme 3
// again at no. 11, on pdf pages 159 and 168. The English translation of the same
// § numbers the second of them Lemma 4, so the two printings disagree and the
// French page image settles what the French volume says.
// int-vi-s2-def-5 is the fourth and is the same shape as the third. § 2 of
// chapter VI of Integration prints Definition 5 at no. 5 on pdf page 43 and
// Definition 5 again at no. 8 on pdf page 50, both in the French, and the
// English translation numbers the second of them Definition 6.
//
// ac-iii-s3-def-1 is the fifth. § 3 of chapter III of Commutative Algebra
// prints Definition 1 at no. 1 on pdf page 216, where it says what it is for a
// filtration to be m-good, and Definition 1 again at no. 3 on pdf page 221,
// where it says what it is for an ideal to be a defining ideal of a topology.
// Both page images were read and both print the number as 1, so the § really
// does number two definitions the same and neither is a misreading.
var repeats = map[string][]int{
	"lie-iii-s3-def-7": {151},
	"evt-ii-s6-def-2":  {81, 82},
	"lie-vi-s1-lem-3":  {168},
	"int-vi-s2-def-5":  {50},
	"ac-iii-s3-def-1":  {221},
}

func statements(blocks []block, id corpus.Ref, pr printing) ([]block, []corpus.Statement, error) {
	taken, err := numbers(blocks, id, pr)
	if err != nil {
		return nil, nil, err
	}
	out := make([]block, 0, len(blocks)*2)
	var found []corpus.Statement
	seen := map[string]bool{}
	err = walk(blocks, id, pr, taken, func(b block, r corpus.Ref, name, body string, ok bool) error {
		if !ok {
			out = append(out, b)
			return nil
		}
		label := r.Label()
		if seen[label] && slices.Contains(repeats[label], b.page) {
			r.Repeated = true
			label = r.Label()
		}
		if seen[label] {
			return fmt.Errorf("two statements are labelled %s", label)
		}
		seen[label] = true
		s := corpus.Statement{Ref: r, PDFPage: b.page, Body: body}
		if l, ok := corpus.ParsePageLabel(b.label); ok {
			s.Page = l.Page
		}
		found = append(found, s)
		out = append(out, block{text: heading(r, label, name, pr), page: b.page, last: b.last, label: b.label})
		if body != "" {
			out = append(out, block{text: body, page: b.page, last: b.last, label: b.label})
		}
		return nil
	})
	if err != nil {
		return nil, nil, err
	}
	return out, found, nil
}

// walk reads the blocks of a piece in printed order and hands each one to f,
// with the statement it turned out to be, or ok false where it turned out not to
// be one.
//
// What makes this worth naming is the state a head is read against: the no. the
// reader stands in, the last statement a Corollary could be numbered under, and
// the run of remarks or examples now open. None of it is on the block in hand,
// all of it is built up from the top of the piece, and the piece is read twice.
// See numbers.
func walk(blocks []block, id corpus.Ref, pr printing, taken map[corpus.Ref]map[int]bool,
	f func(b block, r corpus.Ref, name, body string, ok bool) error) error {
	no := 0
	var parent corpus.Ref           // the last statement a Corollary could be numbered under
	var run corpus.Ref              // the run of remarks or examples now open, if any
	next := 0                       // the number the next member of that run would carry
	passed := map[corpus.Ref]bool{} // the numbered statements printed so far
	occ := map[corpus.Ref]int{}
	high := map[corpus.Ref]int{}          // the highest number a run of the kind has reached in this no.
	off := map[corpus.Ref]int{}           // what a restarted run adds to the number it prints. See restart
	runs, err := runCount(blocks, id, pr) // how many runs of each kind the no. prints
	if err != nil {
		return err
	}
	seen := map[corpus.Ref]int{} // how many of them have been passed
	queue := slices.Clone(blocks)
	for i := 0; i < len(queue); i++ {
		b := queue[i]
		if m := subsecRE.FindStringSubmatch(b.text); m != nil {
			no, _ = strconv.Atoi(m[1])
			next = 0
			if err := f(b, corpus.Ref{}, "", "", false); err != nil {
				return err
			}
			continue
		}
		if strings.HasPrefix(b.text, "#") {
			next = 0
			if err := f(b, corpus.Ref{}, "", "", false); err != nil {
				return err
			}
			continue
		}
		// A lead that carries none of the run it opens. It is a statement of
		// nothing, so it is passed through as prose and only the run bookkeeping
		// moves: the members number from 1 under it, and each of them becomes a
		// statement of its own the same way a member of any other run does.
		//
		// Where a no. prints two runs of one kind, the last one carries the
		// numbering and the earlier ones are left as the prose they are printed
		// as. Three no. of Theory of Sets print two: no. 1 of § 1 of chapter III
		// sets three examples on page 131 and four more on page 132, and the
		// second lot starts again at (1), so a label can hold one of the two
		// Example 3 and the volume has to say which it means. It says so three
		// times over. "no. 1, Example 3" is cited on pages 137, 142 and 145 and
		// every one of them is the set of mappings of subsets of E into F ordered
		// by extension, which is the third member of the second lot; the third of
		// the first lot is the opposite of an order relation and is not what any
		// of those sentences is about. "no. 1, Example 4", cited on page 137 and
		// in exercise 18, is the partitions of a set ordered by fineness, and the
		// first lot has no fourth member at all.
		//
		// So the numbering the volume cites by is the last run's, and that is the
		// run that gets the numbers. This is counted before the walk rather than
		// decided as each head is passed, because the first head cannot know
		// whether another one follows it in the same no.
		// A lead sets the same run the other way, with its first member on the
		// line, so the block is a statement and goes on to statementAt like any
		// other. All that is taken here is the same decision about which of two
		// runs is the one that carries the numbering, and an earlier one is passed
		// through as prose with its members left unnumbered under it, exactly as a
		// head's are. § 1 of chapter I of Topology I to IV is the case: no. 3
		// prints "Examples. 1)" twice on page 27, once for fundamental systems of
		// neighbourhoods and once for bases, and read without this both runs
		// number from 1 and the § has two statements called Example 1.
		if k, carries := runOpen(b.text, pr); k != nil {
			kind, ok := corpus.KindFromHeading(k[1])
			if !ok {
				return fmt.Errorf("nothing in the corpus is called a %q", k[1])
			}
			key := corpus.Ref{Book: id.Book, Chapter: id.Chapter, Section: id.Section,
				Kind: kind, Subsec: no}
			seen[key]++
			earlier := seen[key] < runs[key]
			if earlier {
				next = 0 // an earlier run of the kind, so its members are not numbered
			} else if !carries {
				run, next = key, 1
			}
			if earlier || !carries {
				if err := f(b, corpus.Ref{}, "", "", false); err != nil {
					return err
				}
				continue
			}
		}
		// The printing sets each member of a run as a paragraph of its own, and a
		// page read without the blank line between two of them arrives as one
		// block. Page 16 of Theory of Sets is read that way, with (2) on the line
		// under (1), and the second member would go into the body of the first.
		// Cut it off and put it back in the queue as the block it is printed as.
		if next > 0 {
			if head, rest, ok := cutRunMember(b.text, next); ok {
				b.text = head
				t := b
				t.text, t.label = rest, ""
				queue = slices.Insert(queue, i+1, t)
			}
		}
		r, name, body, ok, err := statementAt(b.text, id, no, parent, run, next, occ, high, off, taken, pr)
		if err != nil {
			return err
		}
		if ok && r.Number > 0 {
			switch r.Kind.Scope() {
			case corpus.ScopeSection:
				parent, passed[r] = r, true
			case corpus.ScopeSubsec:
				run, next = r, r.Number+1
			}
		}
		// A proof the printing broke off and takes up again hands the reader
		// back to the statement it is a proof of, and the corollaries printed
		// after it hang from that statement and not from the last lemma the
		// proof needed. See resumed.
		if !ok {
			if back, is := resumed(b.text, id, pr, passed); is {
				parent = back
			}
		}
		if err := f(b, r, name, body, ok); err != nil {
			return err
		}
	}
	return nil
}

// resumed is the statement a paragraph hands the reader back to, and false
// where the paragraph hands them back to nothing.
//
// A long proof is broken off, the lemmas it needs are stated and proved, and
// then the printing says it is taking the proof up again. What comes after that
// belongs to the statement being proved, and a Corollary printed there is a
// corollary of it. Read without this the corollary is numbered under the last
// lemma, which is the statement the reader passed most recently and not the one
// the page is about.
//
// No. 3 of § 6 of chapter III of Theory of Sets is the case that made this
// worth writing. It prints Theorem 2, then Lemma 1 and Lemma 2 with their
// proofs, then "¶ We come now to the proof of Theorem 2." on page 187, and then
// four corollaries on page 188. The volume says whose they are on that same
// page: Proposition 1 of no. 4 is proved by "The others follow from the
// Corollaries to Theorem 2, no. 3", and § 3 cites "§ 6, no. 3, Theorem 3,
// Corollary 4" for a fact that is Corollary 4 there. Read as the last lemma's
// they were called ens-iii-s6-lem-2-cor-1 to 4, which no sentence of the volume
// names.
//
// § 1 of chapter III of Théories spectrales does the same and says so in the
// corollary itself. Page TS III.13 prints "Terminons la démonstration du
// théorème 3.", the proof, and then "COROLLAIRE 1. — Conservons les hypothèses
// du théorème 3", followed by Corollaire 2, which is Lomonosov's theorem, and
// Corollaire 3. Those three were numbered under Lemme 1.
//
// Those two are the whole of it. Eight paragraphs of the corpus, four in each
// language, say the proof of a numbered statement is being taken up or ended,
// and in the other six what follows is a heading, an unnumbered Remark or an
// unnumbered Example, none of which is numbered under anything. So nothing else
// in either language moves.
//
// The statement has to have been printed already. A paragraph that promises a
// proof still to come is not handing the reader back to anything, and a number
// the § has not reached is a number this cannot name.
func resumed(text string, id corpus.Ref, pr printing, passed map[corpus.Ref]bool) (corpus.Ref, bool) {
	if pr.resume == nil {
		return corpus.Ref{}, false
	}
	m := pr.resume.FindStringSubmatch(text)
	if m == nil {
		return corpus.Ref{}, false
	}
	kind, ok := corpus.KindFromHeading(m[1])
	if !ok {
		return corpus.Ref{}, false
	}
	r := id
	r.Kind = kind
	r.Number, _ = strconv.Atoi(m[2])
	if !passed[r] {
		return corpus.Ref{}, false
	}
	return r, true
}

// runCount is how many runs of each kind each no. of the piece prints, read off
// the blocks before the walk that needs it. It is empty for every printing that
// has no head of this sort, which is all of them but Theory of Sets.
func runCount(blocks []block, id corpus.Ref, pr printing) (map[corpus.Ref]int, error) {
	out := map[corpus.Ref]int{}
	if pr.runHead == nil && pr.runLead == nil {
		return out, nil
	}
	no := 0
	for _, b := range blocks {
		if m := subsecRE.FindStringSubmatch(b.text); m != nil {
			no, _ = strconv.Atoi(m[1])
			continue
		}
		k, _ := runOpen(b.text, pr)
		if k == nil {
			continue
		}
		kind, ok := corpus.KindFromHeading(k[1])
		if !ok {
			return nil, fmt.Errorf("nothing in the corpus is called a %q", k[1])
		}
		out[corpus.Ref{Book: id.Book, Chapter: id.Chapter, Section: id.Section,
			Kind: kind, Subsec: no}]++
	}
	return out, nil
}

// runOpen is the match where a block opens a run of statements, by either of the
// two ways a printing sets one, and nil where it opens none. carries says which
// of the two it is: a head carries no member and a lead carries the first. The
// two cannot both match, since a head is the whole line.
func runOpen(text string, pr printing) (k []string, carries bool) {
	if pr.runHead != nil {
		if k := pr.runHead.FindStringSubmatch(text); k != nil {
			return k, false
		}
	}
	if pr.runLead != nil {
		if k := pr.runLead.FindStringSubmatch(text); k != nil {
			return k, true
		}
	}
	return nil, false
}

// numbers is every number the book gives a statement of the piece, by the kind
// and the no. it is counted in.
//
// An unnumbered statement is named by its place among the statements of its kind
// in its no., and that place cannot be one the book has already given a numbered
// one, whichever of the two is printed first. Reading forwards is enough while
// the numbered one comes first, and that is what took does. It does not come
// first everywhere: no. 3 of § 1 of chapter II of Théories spectrales prints an
// unnumbered Remarque on page 222 and opens a run of Remarques on the page
// after, so both would be called Remark 1 and the chapter would not assemble at
// all. So the piece is read once for the numbers before it is read for the
// statements, and an unnumbered statement steps over a place claimed later.
//
// The unnumbered one then stands second while it is printed first, which is the
// right way round: the number in the second is the book's own and the place of
// the first is this corpus's, so the one that can move is the one that moves.
func numbers(blocks []block, id corpus.Ref, pr printing) (map[corpus.Ref]map[int]bool, error) {
	taken := map[corpus.Ref]map[int]bool{}
	err := walk(blocks, id, pr, nil, func(b block, r corpus.Ref, name, body string, ok bool) error {
		if !ok || r.Number == 0 || r.Kind.Scope() != corpus.ScopeSubsec {
			return nil
		}
		key := r
		key.Number, key.Occurrence = 0, 0
		if taken[key] == nil {
			taken[key] = map[int]bool{}
		}
		taken[key][r.Number] = true
		return nil
	})
	if err != nil {
		return nil, err
	}
	return taken, nil
}

// heading is the heading a statement is given.
//
// The word is the corpus's singular, not the one printed: a run of remarks is
// headed "Remarks. —" in the book and each of its members comes out as a Remark
// of its own here, because each of them is one statement and has one label.
//
// name is the name the printing gives the result and is empty for the great
// majority that are given none. It is kept, and kept where the printing put it,
// because it is how a reader finds the thing: "Théorème 2 (« lemme de
// Nakayama »)" and "Theorem 1 (Wedderburn)" are what those results are called,
// and a heading reading Theorem 1 says nothing. It goes in the heading rather
// than in the body because it is part of the head, and 85 of them across the
// corpus, 23 in the English and 62 in the French, were being dropped on the
// floor with the rest of the head.
func heading(r corpus.Ref, label, name string, pr printing) string {
	head := r.Kind.HeadingIn(pr.lang)
	if r.Number > 0 {
		head += fmt.Sprintf(" %d", r.Number)
	}
	if name != "" {
		head += " (" + name + ")"
	}
	return fmt.Sprintf("#### %s {#%s .statement}", head, label)
}

// statementAt reads one block as a statement, and returns false if it is not
// one. body is the statement with the head taken off, and name is the name the
// head gave it, empty where it gave it none.
func statementAt(text string, id corpus.Ref, no int, parent, run corpus.Ref, next int,
	occ, high, off map[corpus.Ref]int, taken map[corpus.Ref]map[int]bool, pr printing,
) (corpus.Ref, string, string, bool, error) {
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
	// took records a number a statement of this kind carries in this no., so that
	// one standing later with no number of its own is not named as though the
	// numbered ones were not there. See the note on Occurrence.
	bucket := func(kind corpus.Kind) corpus.Ref {
		key := id
		key.Kind, key.Subsec = kind, no
		return key
	}
	took := func(r corpus.Ref) {
		if r.Kind.Scope() != corpus.ScopeSubsec {
			return
		}
		key := bucket(r.Kind)
		occ[key] = max(occ[key], r.Number)
		high[key] = max(high[key], r.Number)
	}
	text = pr.unswallow(text)
	m := pr.head.FindStringSubmatch(text)
	if m == nil {
		// A paragraph opening on the number the open run is up to is the next
		// member of that run.
		num, marker, tail, ok := runItem(text)
		if next == 0 || !ok || num != strconv.Itoa(next-off[bucket(run.Kind)]) {
			return corpus.Ref{}, "", "", false, nil
		}
		r := run
		r.Number = next
		took(r)
		return r, "", body(afterMarker(marker, tail)), true, nil
	}
	// The head matched one of the branches of pr.head and left the others empty,
	// and every branch pairs its kind with its number, so running the pairs
	// together leaves the one that matched. A branch can be added to a grammar
	// without anything here having to know how many there now are.
	var word, num string
	for i := 1; i+1 < len(m); i += 2 {
		word, num = word+m[i], num+m[i+1]
	}
	kind, ok := corpus.KindFromHeading(word)
	if !ok {
		return corpus.Ref{}, "", "", false, fmt.Errorf("nothing in the corpus is called a %q", word)
	}
	// The name the printing gives the result, where it gives one. It is taken
	// out of the head that was just matched and carried to the heading, and it
	// is the one part of a head that is not said again by the heading: the rest
	// of what is dropped here is the kind, the number and the dash. See heading.
	//
	// The divisibility mark comes out of the head first. It sits between the same
	// parentheses a name does and taking it for one would put DIV in the heading
	// and lose the real name of the two heads that carry both, "COROLLARY 2 (DIV)
	// (Euclid's lemma)" and "PROPOSITION 10 (DIV) (Gauss)".
	head := m[0]
	div := divMark.MatchString(head)
	if div {
		head = divMark.ReplaceAllString(head, "")
	}
	name := ""
	if n := headName.FindStringSubmatch(head); n != nil {
		name = n[1]
	}
	r := id
	r.Div = div
	r.Kind = kind
	rest := text[len(m[0]):]
	if num == "" && kind.Scope() == corpus.ScopeSubsec {
		// A run is headed by its kind in the plural and numbered inside:
		// "Remarks. — 2)" is Remark 2 and the head carries no number of its own.
		if i := exNumRE.FindStringSubmatch(rest); i != nil {
			num, rest = i[2], afterMarker(i[0], rest[markerLen(i):])
			num = restart(num, bucket(kind), high, off)
		}
	}
	// The mark that opens a passage in small type stands in front of the head and
	// is taken with it, so it is put back on the front of the body. It is put back
	// exactly as it was read rather than as a canonical "$*$", because the two
	// English volumes set it differently and the body is a transcription, and it
	// is put back after the run has been numbered rather than before, or the mark
	// would stand between the head and the number of the run's first member and
	// "*Remarks 1)" would come out as one Remark with no number at all.
	if star := smallTypeOpen.FindString(m[0]); star != "" {
		rest = star + rest
	}
	// An unnumbered statement has no number to be numbered under, so it is
	// named by where it stands: the no. and how many of its kind came before it
	// there. That holds for an unnumbered Corollary too. Naming it under its
	// parent instead was tried and dropped, because "the second unnumbered
	// corollary of Theorem 1" and "Corollary 2 of Theorem 1" come out as the
	// same string, and the 45 unnumbered corollaries of chapter VIII include a
	// pair in § 1 that collide that way.
	if num == "" {
		key := bucket(kind)
		occ[key]++
		// A place the book gives a numbered statement of the same kind later in
		// the same no. is not free, however far ahead it is printed. See numbers.
		for taken[key][occ[key]] {
			occ[key]++
		}
		r.Subsec, r.Occurrence = no, occ[key]
		return r, name, body(strings.TrimSpace(rest)), true, nil
	}
	r.Number, _ = strconv.Atoi(num)
	switch kind.Scope() {
	case corpus.ScopeSubsec:
		r.Subsec = no
		took(r)
	case corpus.ScopeParent:
		if parent.Number == 0 {
			return corpus.Ref{}, "", "", false, fmt.Errorf(
				"%s %d stands under no statement it could be numbered under", kind.Heading(), r.Number)
		}
		r.ParentKind, r.ParentNumber = parent.Kind, parent.Number
	}
	return r, name, body(strings.TrimSpace(rest)), true, nil
}

// restart is the number a run of remarks or examples carries in a label, which
// is the number it prints except where a second run of the same kind opens in
// the same no. and starts again at one.
//
// A no. can print two runs and Bourbaki numbers each of them from one. No. 3 of
// § 5 of the French Groupes et algebres de Lie chapitre 1 does it: Definition 3
// is followed by "Remarques. — 1)", "2)", "3)", then Lemme 1, Corollaire 1 and
// Corollaire 2, and then a second "Remarques. — 1)", "2)" on the page after. So
// the no. prints two Remarque 1 and two Remarque 2, and read literally the
// second pair takes the labels of the first pair and the chapter does not
// assemble. No. 4 of § 5 of Algebre commutative chapitres 8 et 9 prints two runs
// of Exemples the same way.
//
// The number in the second run belongs to the book and the label belongs to this
// corpus, so it is the label that gives way: the second run carries on from where
// the first left off, and its printed 1 and 2 are Remarque 4 and Remarque 5. A
// citation of "no. 3, remarque 2" then lands on the second member of the first
// run, which is what the volume means by it. Page 64 of that chapter says so
// itself, inside the third member of the first run: "s est contenu dans le
// radical r de g d'apres la remarque 2".
//
// The alternative was the rule Theory of Sets gets, where the last run carries
// the numbering and the earlier ones are left as the prose they are printed as.
// That rule was written on the evidence of three citations in that volume, all
// of which point at the second run, and it loses the members of the first. Here
// the one citation in the chapter points the other way, and nothing is lost by
// carrying on, so this is the rule for the printings that have no run head.
//
// The offset is kept rather than the shifted number alone, because the members
// after the lead are found by the number they print and not by the number they
// are labelled with. Without it the second run's "2)" would be looked for as a
// "5)" and the run would end at its first member.
func restart(num string, key corpus.Ref, high, off map[corpus.Ref]int) string {
	n, err := strconv.Atoi(num)
	if err != nil {
		return num
	}
	off[key] = 0
	if n <= high[key] {
		off[key] = high[key]
	}
	return strconv.Itoa(n + off[key])
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
// Lie 7 to 9 adds two more ways of writing the same marker, both on pages the
// model read rather than pages taken out of the type. It marks the number in
// bold, "**7)** Let E be a finite dimensional s-module", seven times; and it
// closes the pilcrow's own span before the number instead of after it, "$\P$
// 13)" against "$\P 12)$", three times. Ten markers is not many, but a marker
// that is not seen does not lose one exercise, it loses every exercise after it:
// the reader looks for one number and one only, so the unread "**7)**" of § 1 of
// chapter VIII kept exercises 7 to 18 inside exercise 6 and left twelve
// citations pointing at exercises the § appeared not to have. Neither shape
// occurs in any other volume.
//
// The pilcrow is the other half of the same thing. A page taken out of the type
// gives it as the control word "\P" inside the mathematics the number was set
// in, but a page the model read gives the mark itself, "¶ **9)**" and
// "**¶5)**", with no mathematics anywhere near it. So the mark is read on its
// own as well as inside a span, and the bold is allowed to open in front of it
// as well as after it. Four lines of Lie 7 to 9 are written this way and no line
// of any English volume is; the two of the French Topology that are left over
// are exercise 6 of I, p. 126 and a lettered part, which is not a number and
// does not open an item whichever way it is marked.
//
// The star that says an exercise draws on something the reader has not reached
// yet is set as a superscript five times in Lie 7 to 9, "$7)^*$Let $d_1, ...,
// d_l$ be the characteristic degrees", where the other volumes set it on the
// line. It closes the marker's span the same way an ordinary star does, so it is
// read the same way, and § 8 of chapter VIII stopped at its sixth exercise
// without it where the volume prints eighteen.
//
// The closing dollar hangs off the pilcrow rather than standing on its own,
// because on its own it would read "$$ 5) $$", the opening of a display, as a
// marker.
//
// A full stop closes the number as well as a parenthesis does, because Theory of
// Sets closes it that way and every other volume closes it the other way. It
// prints "1. Let $\mathscr{T}$ be a theory with no specific signs." and "¶ 4.
// Let $A$ be a term", where Algebra VIII prints "1)" and "$\P 4)$", and it
// letters the parts of an exercise "(a)" rather than "a)", so the two shapes do
// not meet. This is a difference between two printings of one language, like the
// numeral over an appendix, so the marker is read both ways rather than one way
// being chosen by the volume. Read one way only, § 1 of chapter I reported a
// preamble and no exercises at all, and so did every other § of the book.
//
// The cost of reading the full stop was measured before it was allowed rather
// than argued about: no paragraph of the assembled corpus, in either language,
// opens on a number and a full stop. What keeps it honest at all is the rule
// that already keeps the parenthesis honest, that a marker counts only when it
// carries the number the run is up to.
//
// The marks in front of the number are read as one run of marks rather than in
// a fixed order, because Theory of Sets prints them in both orders and outside
// the mathematics: "* 4." on page 214, "\* 9." on page 215 where the text layer
// escaped the star, and "¶ * 18." on page 218 with the pilcrow first. Which mark
// the exercise carries is then read off the run. § 1 of chapter III stopped at
// its third exercise where the volume prints twenty four.
//
// The section sign is here because a page read as a picture sometimes comes back
// with it where the pilcrow is printed. The two marks are close enough on the
// page that a model swaps them, and page 447 of Algebra VIII is the case: the
// text layer had given it as "$\P 22)$" and a re-reading of the page image gave
// "$ \S 22) $", so § 21 stopped at exercise 21 and the nine the volume prints
// after it were lost. The same swap is on page 137 of Topology I to IV, and
// those two are the whole of it in this corpus. It costs nothing elsewhere,
// because a section sign in front of a number is otherwise a citation, and a
// citation is a comma or a page away from its number rather than a bracket:
// 1209 lines write a section sign and a number and only these two put a bracket
// after it. The rule that a marker counts only when it carries the number the §
// is up to holds them down as well.
//
// The dagger is the same swap again and a bigger one. A model reading a page
// image gives the pilcrow as a dagger 64 times, across nine volumes in both
// languages, and page 95 of Algebra 4 to 7 is the case that found it: the
// printing opens exercises 1, 3 and 4 of § 1 with a pilcrow and the reading
// wrote "† 1)", so the § reported a preamble and no exercises at all and the
// whole of chapter IV stopped assembling on it. It costs nothing to read,
// because the dagger has one other use in this corpus and it is not this one:
// of the 130 lines that write a dagger anywhere, the 64 that put a number and
// a bracket after it all open the line with it, and not one of the other 66
// puts a dagger in front of a number. Those 66 are footnote marks, which sit
// inside a sentence or against the word they hang off.
var exNumRE = regexp.MustCompile(
	`^(?:\*\*)?((?:\$[ \t]*)?(?:(?:\\?\*|\\P|\\S|¶|†)[ \t]*)+(?:\$[ \t]*)?|(?:\$[ \t]*)?)` +
		`(?:\*\*)?(\d+)[.)](?:\*\*|\^?\*?\$|(\s|[a-z]\)))`)

// marks are the star and the pilcrow a book can set in front of an exercise
// number. The bold that extraction writes around the number of some of them is
// taken off first, so that it is not read as a star.
func marksOf(prefix string) (star, pilcrow string) {
	prefix = strings.ReplaceAll(prefix, "**", "")
	// The section sign and the dagger are misread pilcrows and mark what the
	// pilcrow marks. See exNumRE. The two control words are looked for before
	// the bare marks for the same reason: both of them carry a letter that
	// markOf would find in the other.
	for _, c := range []string{`\P`, `\S`} {
		if p := markOf(prefix, c); p != "" {
			return markOf(prefix, "*"), p
		}
	}
	if p := markOf(prefix, "†"); p != "" {
		return markOf(prefix, "*"), p
	}
	return markOf(prefix, "*"), markOf(prefix, "¶")
}

// runNumRE is the other way a volume numbers a member of a run: the number in
// parentheses, at the head of a paragraph of its own.
//
// Theory of Sets writes every run this way. Page 16 sets its examples under
// "Examples" in italic and then "(1) The assembly ∨1 is represented by ⇒." and
// "(2) The following symbols represent assemblies", each a paragraph of its own,
// and 30 of the volume's 34 runs open exactly like that. The other four open on
// the star that brackets a passage in small type, which is why the star is read
// here as well and put back with the rest of the body.
//
// This is only ever asked of a paragraph while a run is open and only ever
// accepted when the number is the one the run is up to, which is what keeps the
// enumerations out. Page 15 lists the signs of a theory as "(1) The logical
// signs", "(2) The letters", "(3) The specific signs", under no head and inside
// a sentence that runs into them, and no run is open there.
//
// The number has to be followed by the member on the same line, because that is
// the one thing that tells it from the number of a displayed formula. The French
// volumes set that number on a line of its own with the display under it, and
// page 41 of Algebre commutative chapitres 5 a 7 is where it cost something: the
// run of Remarques opened on page 40 with members 1) and 2) and stayed open, so
// the "(3)" over the display on page 41 was read as Remarque 3, and page 42,
// which prints "Remarques. - 3)", gave the same label to the remark that really
// is the third. The chapter did not assemble. 162 pages of the corpus set a
// formula number that way and every one of them is the same trap.
var runNumRE = regexp.MustCompile(`^(?:\\?\*\s*)?\((\d+)\)[^\S\n]+`)

// runItem reads the marker on a member of a run, either way a volume writes it,
// and says what the number is, how much of the block the marker takes, and what
// is left after it.
// cutRunMember cuts a block that opens on member now of an open run at the line
// where member now+1 begins, so that each member is the block the printing sets
// it as. It cuts on nothing else: a bracketed number that is not the one the run
// is up to is a reference to a formula or a case, not a member.
func cutRunMember(text string, now int) (head, rest string, ok bool) {
	if num, _, _, ok := runItem(text); !ok || num != strconv.Itoa(now) {
		return "", "", false
	}
	lines := strings.Split(text, "\n")
	for i := 1; i < len(lines); i++ {
		if num, _, _, ok := runItem(lines[i]); ok && num == strconv.Itoa(now+1) {
			return strings.Join(lines[:i], "\n"), strings.Join(lines[i:], "\n"), true
		}
	}
	return "", "", false
}

func runItem(text string) (num, marker, rest string, ok bool) {
	if i := exNumRE.FindStringSubmatch(text); i != nil {
		return i[2], i[0], text[markerLen(i):], true
	}
	if i := runNumRE.FindStringSubmatch(text); i != nil {
		// A trailing space before the line break is not the member. The line
		// has to carry something after the number, or this is the number of a
		// display and not a member of the run.
		if rest := text[len(i[0]):]; rest != "" && !strings.HasPrefix(rest, "\n") {
			return i[1], i[0], rest, true
		}
	}
	return "", "", "", false
}

// markerLen is how much of a block the marker takes, which is not always the
// whole of what matched: a lettered part is matched to prove the number opens
// an item and then left where it stands, since it is the first line of the item
// and not part of its number.
func markerLen(m []string) int {
	n := len(m[0])
	if len(m) > 3 && strings.TrimSpace(m[3]) != "" {
		n -= len(m[3])
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
//
// What stands between the heading and exercise 1 is the preamble, and it is not
// an exercise. Algebra VIII has none, and the run of a § there opens on its
// first exercise; Lie 7 to 9 has one over most of its runs, saying what the
// letters mean or what is assumed throughout, as § 4 of chapter VII does with
// "The notations and assumptions are those of nos. 1, 2, 3 of § 4." It is left
// where the book sets it, under the heading and above the exercises, which is
// where cutExercises keeps it: it is one statement about the whole run and
// copying it into each of the nineteen files of the run would say it nineteen
// times. A run whose exercises are all preamble is a run whose first marker was
// misread, and Verify is what catches that.
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
					break // the preamble, see preamble
				}
				out[len(out)-1].Body += "\n\n" + text
				onPage(&out[len(out)-1], b)
				break
			}
			if head := strings.TrimSpace(text[:i]); head != "" && len(out) > 0 {
				out[len(out)-1].Body += "\n\n" + head
				onPage(&out[len(out)-1], b)
			}
			e := corpus.Exercise{Pages: spanning(nil, b)}
			e.Meta.Exercise = len(out) + 1
			star, pilcrow := marksOf(m[1])
			e.Meta.Supplementary = star != ""
			e.Meta.Starred = pilcrow != ""
			e.Meta.PDFPage = b.page
			e.Meta.BookPage = bookPage(b)
			out = append(out, e)
			text = afterMarker(m[0], text[i+markerLen(m):])
		}
	}
	return out, nil
}

// bookPage is the page of the printed book an exercise starts on.
//
// A volume that prints a page label on the page is cited by that label, and the
// label is what goes here: "A VIII.229". Lie 7 to 9 prints no label. It is
// paginated straight through and puts the number in the running head, so the
// folio read off that head is the only thing that says which page of the book
// an exercise is printed on, and without this the 351 exercises of that volume
// say nothing about where they are, which is worse than the wrong label they
// used to carry.
//
// Bare, and not dressed up as a label. A § of the same volume already writes
// its range of pages bare for the same reason, and a label naming a chapter
// would be inventing a citation form the volume does not use.
func bookPage(b block) string {
	if l, ok := corpus.ParsePageLabel(b.label); ok {
		return l.String()
	}
	if b.folio > 0 {
		return strconv.Itoa(b.folio)
	}
	return ""
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
		if got, _ := strconv.Atoi(m[2]); got == n {
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
			// A pilcrow in front of the number is part of the marker and is
			// taken with it. Left where it stands it would be pushed onto the
			// end of the exercise before this one, and the exercise it marks
			// would come out unmarked. The asterisk is not backed up over,
			// because the one that turns up here is the one that closes the
			// passage before rather than the one that marks what follows.
			if l := pilcrowBefore.FindStringIndex(text[:at]); l != nil {
				at = l[0]
			}
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
				m = []string{raw, marks, strconv.Itoa(got), ""}
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

// pilcrowBefore is a pilcrow standing between the end of a sentence and the
// number of the exercise it marks, with whatever is left of the math span it
// was written in around it. § 1 of chapter VIII runs "it suffices that the
// eigenvalue 0. $\P 14)$ The notations are those of the preceding exercise",
// with the whole of exercise 14 opening in the paragraph that ended exercise 13.
var pilcrowBefore = regexp.MustCompile(`\$?\s*(?:\\P|¶)\s*\$?\s*$`)

// shortened is a full stop that closes a word the book writes short rather than
// a sentence. Every citation of a no. carries one, and the number after it is a
// number the § is very likely to be up to: "(Chapter II, § 6, no. 3)" in the
// second exercise of § 1 of chapter III of Theory of Sets was read as the start
// of the third, which cost that § the twenty exercises printed after it.
var shortened = regexp.MustCompile(`(?i)(?:^|[\s(\[])(?:no|nos|p|pp|cf|fig|chap|vol|resp|art)\.$`)

// sentenceEnd reports whether the text before a number is the end of a
// sentence, with the marks the book closes a passage with, and the mark it
// opens one with, taken off.
//
// A line break counts as space here and not as anything else. The preamble of a
// run and the first exercise under it land in the one block whenever the page
// was read without a blank line between them, and the French Lie chapter I is
// read that way: "Les conventions du § 4 restent valables, sauf mention
// contraire." and "1) Soient g une algebre de Lie nilpotente" are one block
// with a newline in the middle. Trimming only spaces left the full stop behind
// the newline, so the sentence did not read as ended, exercise 1 was never
// found, and every exercise after it was passed over too, since the reader
// looks for one number and one only. That § prints 27 exercises and the volume
// reported none of them.
func sentenceEnd(s string) bool {
	s = strings.TrimRight(pilcrowBefore.ReplaceAllString(s, ""), " \t\n$*")
	if shortened.MatchString(s) {
		return false
	}
	return strings.HasSuffix(s, ".") || strings.HasSuffix(s, ")")
}

func first(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
