package report

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// A chapter of the Éléments printed twice is one chapter. The English Algebra
// VIII of 2023 and the French Algèbre VIII of 2012 set the same propositions in
// the same order under the same numbers, so a statement of one is a statement of
// the other, carries the same label and therefore the same permanent tag. That
// half of the rule is invariant T07, which tags verify already holds every
// printing to.
//
// This is the other half. Two printings that agree section by section on how
// many statements and how many exercises they hold are almost certainly being
// read correctly, and where they disagree one of two things is true: the
// printings really differ, or one of them is being read wrongly. Which of the
// two it is is a judgement, and the report does not make it. It says where to
// look, and it says it in the units somebody can check against the printed
// page: § 16 of the English holds 17 exercises and the French 14, so open § 16.
//
// It found nine such places on chapter VIII and eight of them were defects: a
// statement head the French sets in italic where the English sets it in small
// capitals, an exercise marked "$15)*$" that stopped the exercise reader dead,
// a pair of large parentheses that landed on the head above the formula they
// enclosed. The ninth is real, and the report exists so that the ninth can be
// told from the other eight: the 2023 English edition prints a twentieth
// exercise in § 2 that the 2012 French edition does not.

// A Pair is one chapter of one Book as two printings hold it.
//
// The pairing is by book and chapter and not by volume, because the volumes do
// not line up: Integration I to VI is one book in English and four in French.
// A chapter is the largest thing both printings agree on the boundaries of.
type Pair struct {
	Book      string // alg, top, int
	Chapter   string // VIII
	Left      string // the volume id holding it in the left language
	Right     string
	LeftLang  string
	RightLang string
}

// Pairs is every chapter both languages have assembled, in shelf order.
func Pairs(bm *corpus.BooksManifest, sm *corpus.SectionsManifest, left, right string) []Pair {
	type key struct{ book, chapter string }
	have := map[key]map[string]string{}
	for _, b := range sm.Books {
		vol, ok := bm.Get(b.ID)
		if !ok {
			continue
		}
		if vol.Lang != left && vol.Lang != right {
			continue
		}
		for _, c := range b.Chapters {
			if len(c.Sections) == 0 {
				continue
			}
			k := key{vol.Book, c.Chapter}
			if have[k] == nil {
				have[k] = map[string]string{}
			}
			have[k][vol.Lang] = b.ID
		}
	}
	var out []Pair
	for k, langs := range have {
		if langs[left] == "" || langs[right] == "" {
			continue
		}
		out = append(out, Pair{
			Book: k.book, Chapter: k.chapter,
			Left: langs[left], Right: langs[right],
			LeftLang: left, RightLang: right,
		})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Book != out[j].Book {
			return out[i].Book < out[j].Book
		}
		ci, _ := corpus.RomanOrder(out[i].Chapter)
		cj, _ := corpus.RomanOrder(out[j].Chapter)
		if ci != cj {
			return ci < cj
		}
		return out[i].Chapter < out[j].Chapter
	})
	return out
}

// A Printings is the two printings of one chapter set against each other.
type Printings struct {
	Pair
	Rows []PrintingRow
	// Statements and Exercises are the totals, left then right.
	Statements [2]int
	Exercises  [2]int
}

// A PrintingRow is one § as each printing holds it.
type PrintingRow struct {
	Label      string
	Title      string
	Statements [2]int
	Exercises  [2]int
}

// Agrees reports whether the two printings hold the same counts throughout.
func (r PrintingRow) Agrees() bool {
	return r.Statements[0] == r.Statements[1] && r.Exercises[0] == r.Exercises[1]
}

// Compare sets the two printings of a chapter against each other, § by §.
//
// The § is what the two are matched on, and not the file, because the file is
// named for its title and the titles are in different languages: § 13 of
// Algebra VIII is 13_s13_absolutely_semisimple_algebras in one printing and
// 13_s13_algebres_absolument_semi_simples in the other. Both carry the label
// alg-viii-s13, which is what makes them the same §.
func Compare(sm *corpus.SectionsManifest, p Pair) (*Printings, error) {
	l, ok := chapterOf(sm, p.Left, p.Chapter)
	if !ok {
		return nil, fmt.Errorf("%s has no assembled chapter %s in manifests/sections.yaml", p.Left, p.Chapter)
	}
	r, ok := chapterOf(sm, p.Right, p.Chapter)
	if !ok {
		return nil, fmt.Errorf("%s has no assembled chapter %s in manifests/sections.yaml", p.Right, p.Chapter)
	}
	out := &Printings{Pair: p}
	rows := map[string]*PrintingRow{}
	take := func(c corpus.ChapterSections, side int) {
		for _, s := range c.Sections {
			// The frontmatter and the historical note carry no label, since
			// neither is a §, but both are files with counts in them and a
			// chapter has one of each. The kind names them well enough.
			key := s.Label
			if key == "" {
				key = s.Kind
			}
			row := rows[key]
			if row == nil {
				row = &PrintingRow{Label: key}
				rows[key] = row
			}
			if side == 0 || row.Title == "" {
				row.Title = s.Title
			}
			row.Statements[side] += s.Statements
			row.Exercises[side] += s.Exercises
			out.Statements[side] += s.Statements
			out.Exercises[side] += s.Exercises
		}
	}
	take(l, 0)
	take(r, 1)
	for _, row := range rows {
		out.Rows = append(out.Rows, *row)
	}
	sort.Slice(out.Rows, func(i, j int) bool { return labelLess(out.Rows[i].Label, out.Rows[j].Label) })
	return out, nil
}

// Disagreements is how many sections the two printings do not agree on.
func (p *Printings) Disagreements() int {
	n := 0
	for _, r := range p.Rows {
		if !r.Agrees() {
			n++
		}
	}
	return n
}

// Line is the one line a chapter that agrees throughout is worth.
func (p *Printings) Line() string {
	return fmt.Sprintf("%s %s: %d of %d sections agree, statements %d %s %d %s, exercises %d %s %d %s",
		p.Book, p.Chapter,
		len(p.Rows)-p.Disagreements(), len(p.Rows),
		p.Statements[0], p.LeftLang, p.Statements[1], p.RightLang,
		p.Exercises[0], p.LeftLang, p.Exercises[1], p.RightLang)
}

// Table writes the comparison as Markdown. all is every § against only the ones
// the printings disagree on, which is what somebody auditing a chapter wants
// and is usually a handful of lines.
func (p *Printings) Table(all bool) string {
	var b strings.Builder
	fmt.Fprintf(&b, "### %s chapter %s, %s against %s\n\n", p.Book, p.Chapter, p.Left, p.Right)
	fmt.Fprintf(&b, "| Section | Statements %s | Statements %s | Exercises %s | Exercises %s |\n",
		p.LeftLang, p.RightLang, p.LeftLang, p.RightLang)
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	shown := 0
	for _, r := range p.Rows {
		if !all && r.Agrees() {
			continue
		}
		shown++
		fmt.Fprintf(&b, "| %s | %d | %d | %d | %d |\n",
			r.Label, r.Statements[0], r.Statements[1], r.Exercises[0], r.Exercises[1])
	}
	if shown == 0 {
		b.WriteString("| none | | | | |\n")
	}
	fmt.Fprintf(&b, "\n%s.\n", p.Line())
	return b.String()
}

// chapterOf is one chapter of one volume.
func chapterOf(sm *corpus.SectionsManifest, id, chapter string) (corpus.ChapterSections, bool) {
	for _, b := range sm.Books {
		if b.ID != id {
			continue
		}
		for _, c := range b.Chapters {
			if c.Chapter == chapter {
				return c, true
			}
		}
	}
	return corpus.ChapterSections{}, false
}

// labelLess orders the parts of a chapter as the chapter prints them: the
// frontmatter, then the §§ in number order, then the appendices, then the
// historical note. Sorting the labels as strings would put § 10 before § 2.
func labelLess(a, b string) bool {
	ka, na := labelOrder(a)
	kb, nb := labelOrder(b)
	if ka != kb {
		return ka < kb
	}
	if na != nb {
		return na < nb
	}
	return a < b
}

func labelOrder(label string) (int, int) {
	switch label {
	case corpus.KindFront:
		return 0, 0
	case corpus.KindHistorical:
		return 4, 0
	}
	i := strings.LastIndexByte(label, '-')
	if i < 0 || i+1 >= len(label) {
		return 3, 0
	}
	rest := label[i+1:]
	kind := 3
	switch rest[0] {
	case 's':
		kind = 1
	case 'a':
		kind = 2
	default:
		return 3, 0
	}
	n := 0
	for _, c := range rest[1:] {
		if c < '0' || c > '9' {
			return 3, 0
		}
		n = n*10 + int(c-'0')
	}
	return kind, n
}
