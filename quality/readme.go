package quality

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The README is the only document in this project with an audience, and almost
// every sentence in it carries a number: how many volumes are held, how many
// pages, how many of them have a text layer worth reading, how many sections
// have been extracted, how many rules the audit runs. Every one of those is a
// fact about the corpus, and every one of them moves.
//
// They used to be typed in by hand and they went wrong the way hand-typed
// numbers always go wrong, quietly and all at once. The coverage table was the
// worst of them: it stood at 201 of 369 sections while the corpus held 362 of
// 475, so the one document anybody reads was understating the work by a third
// and naming forty chapters as empty that were not. Nobody noticed because
// nothing checked.
//
// So the numbers are generated. Each one sits in a block between two HTML
// comments, invisible in every renderer and obvious in the source, and each
// block is a pure function of the manifests and the committed Markdown. H06
// regenerates all of them and fails if anything moved, which makes a stale
// README a red build rather than a thing somebody has to remember.
//
// What is not generated is the prose. The paragraphs around these blocks say
// what the numbers mean and why the work is shaped the way it is, and that is
// not derivable from a manifest and should not be.

// A READMEBlock is one generated region of the README.
type READMEBlock struct {
	// Name is what the markers say, so LIBRARY sits between
	// <!-- BEGIN LIBRARY --> and <!-- END LIBRARY -->.
	Name   string
	Render func(*Corpus) string
}

// READMEBlocks are the generated regions, in the order the README carries them.
func READMEBlocks() []READMEBlock {
	return []READMEBlock{
		{Name: "LIBRARY", Render: Library},
		{Name: "TEXTLAYER", Render: TextLayer},
		{Name: "COVERAGE", Render: Coverage},
		{Name: "TRANSLATION", Render: Translated},
		{Name: "RULES", Render: Rules},
	}
}

// BeginMarker and EndMarker are what a block is delimited by.
func BeginMarker(name string) string { return "<!-- BEGIN " + name + " -->" }
func EndMarker(name string) string   { return "<!-- END " + name + " -->" }

// readmeBlock is what sits between one block's markers, and whether they are
// both there.
func readmeBlock(readme, name string) (string, bool) {
	begin, end := BeginMarker(name), EndMarker(name)
	i := strings.Index(readme, begin)
	if i < 0 {
		return "", false
	}
	j := strings.Index(readme[i:], end)
	if j < 0 {
		return "", false
	}
	return readme[i+len(begin) : i+j], true
}

// replaceBlock puts a block back between its markers.
func replaceBlock(readme, name, block string) (string, error) {
	begin, end := BeginMarker(name), EndMarker(name)
	i := strings.Index(readme, begin)
	if i < 0 {
		return "", fmt.Errorf("README.md has no %s marker", begin)
	}
	j := strings.Index(readme[i:], end)
	if j < 0 {
		return "", fmt.Errorf("README.md has no %s marker", end)
	}
	return readme[:i+len(begin)] + block + readme[i+j:], nil
}

// StaleREADME is the blocks whose text is not what the corpus says it should
// be, named so that the finding says which one to look at, plus the ones whose
// markers are missing altogether.
//
// A missing marker is reported rather than ignored. A block that is not in the
// file is a number that has quietly gone back to being typed in by hand, which
// is the state this whole mechanism exists to get out of.
func StaleREADME(c *Corpus, readme string) (stale, missing []string) {
	for _, b := range READMEBlocks() {
		have, ok := readmeBlock(readme, b.Name)
		if !ok {
			missing = append(missing, b.Name)
			continue
		}
		if strings.TrimSpace(have) != strings.TrimSpace(b.Render(c)) {
			stale = append(stale, b.Name)
		}
	}
	return stale, missing
}

// WriteREADME regenerates every block and reports which ones moved.
func WriteREADME(root string, c *Corpus) ([]string, error) {
	path := filepath.Join(root, "README.md")
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	readme := string(b)
	out := readme
	var changed []string
	for _, blk := range READMEBlocks() {
		next, err := replaceBlock(out, blk.Name, blk.Render(c))
		if err != nil {
			return nil, err
		}
		if next != out {
			changed = append(changed, blk.Name)
		}
		out = next
	}
	if out == readme {
		return nil, nil
	}
	return changed, os.WriteFile(path, []byte(out), 0o644)
}

// Library is what the corpus is meant to hold: the volumes registered in
// manifests/books.yaml, one row per Book of the Éléments, with the chapters
// each printing covers.
//
// This is scope and not progress. A volume counts here the moment it is
// registered, whether or not a page of it has been read, because the question
// this table answers is what the project is for and the one below it is how far
// along the project is.
func Library(c *Corpus) string {
	type row struct {
		book    string
		title   string
		order   int
		en, fr  []string
		volumes int
		pages   int
	}
	byBook := map[string]*row{}
	var order []string
	// c.Books.Books is shelved in the order the Éléments print their Books, so
	// the order a Book is first seen in is the order it belongs in here. There
	// is no second place to write that order down and no second place for it to
	// drift out of step with.
	var volumes, pages int
	byLang := map[string]struct{ volumes, pages int }{}
	for _, b := range c.Books.Books {
		volumes++
		pages += b.Pages
		l := byLang[b.Lang]
		l.volumes++
		l.pages += b.Pages
		byLang[b.Lang] = l

		r, ok := byBook[b.Book]
		if !ok {
			r = &row{book: b.Book, title: corpus.BookTitle(b.Book), order: len(order)}
			byBook[b.Book] = r
			order = append(order, b.Book)
		}
		r.volumes++
		r.pages += b.Pages
		switch b.Lang {
		case "en":
			r.en = append(r.en, chapterCell(b))
		case "fr":
			r.fr = append(r.fr, chapterCell(b))
		}
	}
	rows := make([]*row, 0, len(order))
	for _, k := range order {
		rows = append(rows, byBook[k])
	}
	sort.SliceStable(rows, func(i, j int) bool { return rows[i].order < rows[j].order })

	var b strings.Builder
	b.WriteString("\n")
	fmt.Fprintf(&b, "All %s Books of the *Éléments*, in the English translation where one was printed and in the French original everywhere else. %d volumes, %d pages, of which %d volumes and %d pages are English and %d volumes and %d pages are French.\n\n",
		numberWord(len(rows)), volumes, pages,
		byLang["en"].volumes, byLang["en"].pages,
		byLang["fr"].volumes, byLang["fr"].pages)
	b.WriteString("| Book | English | French | Volumes | Pages |\n")
	b.WriteString("| --- | --- | --- | --- | --- |\n")
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %s | %s | %d | %d |\n",
			r.title, chapterRange(r.en), chapterRange(r.fr), r.volumes, r.pages)
	}
	return b.String()
}

// chapterCell is what one volume contributes to its Book's chapter range.
//
// Two volumes list no chapters at all, and they list none for opposite reasons.
// Éléments d'histoire des mathématiques is not divided into chapters, so it is
// the whole of its Book. Variétés différentielles et analytiques is divided
// into chapters and this is not one of them, it is the fascicule de résultats,
// which is the summary the Book was published as while the chapters themselves
// never appeared. The title is where that difference is written down, so it is
// where this reads it from rather than from a list somewhere in here.
func chapterCell(b corpus.Book) string {
	if len(b.Chapters) > 0 {
		return strings.Join(b.Chapters, " ")
	}
	if _, tail, ok := strings.Cut(b.Title, ", "); ok {
		return tail
	}
	return "whole"
}

// chapterRange folds the chapter cells of one Book's printings into one cell,
// as runs, so eight volumes of Algebra covering I to VIII read as that and a
// Book with a hole in it says where the hole is.
func chapterRange(cells []string) string {
	if len(cells) == 0 {
		return "none held"
	}
	seen := map[string]bool{}
	var nums []string
	var words []string
	for _, cell := range cells {
		for _, ch := range strings.Fields(cell) {
			if _, err := corpus.RomanOrder(ch); err != nil {
				continue
			}
			if !seen[ch] {
				seen[ch] = true
				nums = append(nums, ch)
			}
		}
		if f := strings.Fields(cell); len(f) > 0 {
			if _, err := corpus.RomanOrder(f[0]); err != nil && !seen[cell] {
				seen[cell] = true
				words = append(words, cell)
			}
		}
	}
	if len(nums) == 0 {
		return strings.Join(words, ", ")
	}
	sort.Slice(nums, func(i, j int) bool {
		x, _ := corpus.RomanOrder(nums[i])
		y, _ := corpus.RomanOrder(nums[j])
		return x < y
	})
	var runs []string
	for i := 0; i < len(nums); {
		j := i
		for j+1 < len(nums) {
			a, _ := corpus.RomanOrder(nums[j])
			b, _ := corpus.RomanOrder(nums[j+1])
			if b != a+1 {
				break
			}
			j++
		}
		if j == i {
			runs = append(runs, nums[i])
		} else {
			runs = append(runs, nums[i]+" to "+nums[j])
		}
		i = j + 1
	}
	return strings.Join(append(runs, words...), ", ")
}

// TextLayer is what the library's own text is worth, which is what decides the
// order the volumes are read in and most of what the reading costs.
func TextLayer(c *Corpus) string {
	kinds := []struct{ name, means string }{
		{"native", "Born digital. `pdftotext -layout` gives real text and real mathematics."},
		{"ocr", "A scan somebody has already run OCR over. Good enough to read a running head off, useless for mathematics."},
		{"none", "A scan with no text at all. Even the page map has to come out of vision OCR."},
	}
	count := map[string]int{}
	byKind := map[string][]corpus.Book{}
	for _, b := range c.Books.Books {
		count[b.TextLayer]++
		byKind[b.TextLayer] = append(byKind[b.TextLayer], b)
	}

	var b strings.Builder
	b.WriteString("\n| Text layer | Volumes | What it means |\n")
	b.WriteString("| --- | --- | --- |\n")
	for _, k := range kinds {
		fmt.Fprintf(&b, "| %s | %d | %s |\n", k.name, count[k.name], k.means)
	}
	native, none, ordinary := count["native"], count["none"], count["ocr"]
	b.WriteString("\n")
	fmt.Fprintf(&b, "The %s native %s %s %s. They are cheap and they go first. The %s with no text at all %s %s, and %s the most expensive %s in the library. The other %s %s the ordinary case.\n",
		numberWord(native), agree(native, "volume", "volumes"), agree(native, "is", "are"), byLanguage(byKind["native"]),
		numberWord(none), agree(none, "is", "are"), withPages(byKind["none"]),
		agree(none, "it is", "they are"), agree(none, "volume", "volumes"),
		numberWord(ordinary), agree(ordinary, "is", "are"))
	return b.String()
}

// byLanguage names volumes, saying which printing each is, because "Algebra,
// Chapter 8" and "Algèbre, Chapitre 8" are two files and a reader has to be
// able to tell which of them is meant.
func byLanguage(books []corpus.Book) string {
	var en, fr []string
	for _, b := range books {
		switch b.Lang {
		case "en":
			en = append(en, "*"+b.Title+"*")
		default:
			fr = append(fr, "*"+b.Title+"*")
		}
	}
	switch {
	case len(en) == 0:
		return joinList(fr) + " in French"
	case len(fr) == 0:
		return joinList(en) + " in English"
	}
	return joinList(en) + " in English, and " + joinList(fr) + " in French"
}

// withPages names volumes with what each one costs, in pages.
func withPages(books []corpus.Book) string {
	out := make([]string, 0, len(books))
	for _, b := range books {
		out = append(out, fmt.Sprintf("*%s* at %d pages", b.Title, b.Pages))
	}
	return joinList(out)
}

// Rules is what the audit is, in rules and groups. The README says elsewhere
// that anything outside content/ passes every rule by default, and this is the
// number that gives that sentence its weight.
func Rules(c *Corpus) string {
	all := Checks()
	count := map[string]int{}
	var hard int
	for _, ck := range all {
		count[ck.Group]++
		if ck.Hard {
			hard++
		}
	}
	var parts []string
	var groups int
	for _, g := range GroupOrder() {
		if count[g] == 0 {
			continue
		}
		groups++
		parts = append(parts, fmt.Sprintf("%d %s", count[g], g))
	}
	var b strings.Builder
	b.WriteString("\n")
	fmt.Fprintf(&b, "The audit is %d rules in %s groups: %s. %d of them are hard, which means a finding fails the build, and %d are soft.\n",
		len(all), numberWord(groups), joinList(parts), hard, len(all)-hard)
	return b.String()
}

// joinList writes a list the way a sentence does, with "and" before the last
// one and no comma in front of it.
func joinList(items []string) string {
	switch len(items) {
	case 0:
		return ""
	case 1:
		return items[0]
	case 2:
		return items[0] + " and " + items[1]
	}
	return strings.Join(items[:len(items)-1], ", ") + " and " + items[len(items)-1]
}

// agree picks the form a count wants. The blocks are sentences and not table
// cells, and a sentence that says "the one with no text at all are" reads like
// a template with the seams showing, which is the whole thing generated prose
// has to avoid.
func agree(n int, one, many string) string {
	if n == 1 {
		return one
	}
	return many
}

// numberWord spells a small number out, because the prose around these blocks
// is written and not tabulated and "All 12 Books" reads like a spreadsheet.
// Above twenty the figure is easier to read than the word, which is where this
// stops.
func numberWord(n int) string {
	words := []string{
		"zero", "one", "two", "three", "four", "five", "six", "seven",
		"eight", "nine", "ten", "eleven", "twelve", "thirteen", "fourteen",
		"fifteen", "sixteen", "seventeen", "eighteen", "nineteen", "twenty",
	}
	if n >= 0 && n < len(words) {
		return words[n]
	}
	return fmt.Sprint(n)
}
