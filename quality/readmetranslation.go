package quality

import (
	"fmt"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The coverage table says what has been read out of the books. This one says
// what has been carried into another language, which is the other half of the
// work and until now the half nobody could see a number for.
//
// The question it answers is the one that gets asked: how many exercises does
// each Book have, and how many of them are translated. That was only findable
// by running find over content/ and comparing two listings by hand, which is
// how the coverage numbers used to work before they were generated, and it went
// wrong the same way: quoted once in an issue, then repeated for weeks after it
// stopped being true.
//
// Two things about the counting are worth stating because both of them are
// choices and neither is obvious.
//
// The English is two trees and both of them count. content/en is the printed
// English translation, content/en-mt is this project's reading of the French
// where no English was ever printed. They are held apart on purpose, because
// one is Springer's translation and the other is a machine's and a reader has
// to be able to tell. But a Vietnamese section is translated from whichever of
// them exists, so for the question this table asks they are one source, and a
// figure that counted only content/en would call every section of Algebre IX
// and X, Algebre commutative VIII to X and Integration V to IX untranslatable
// rather than untranslated.
//
// The French is counted and not compared. A section file carries a slug of its
// own title, so 02_s2_relevement_des_ideaux_premiers.md and
// 02_s2_the_lift_of_prime_ideals.md are one section under two names, and
// comparing the French tree with the English by path calls every such pair a
// hole. Doing exactly that once reported 289 French sections with no English
// when the answer was 41. English against Vietnamese is safe, because a
// translation keeps the path of the file it was made from, and that is the only
// comparison this makes.

// A translationRow is one Book of the Éléments.
type translationRow struct {
	book      string
	bookTitle string
	order     int

	// sections and exercises are what the English holds for this Book, the two
	// English trees together and each file counted once.
	sections  int
	exercises int

	// done is, per target language, how many of those have a file in that
	// language: sections at 0 and exercises at 1.
	done map[string][2]int

	// machine is how many of the sections and exercises above are held only in
	// content/en-mt, so a translation of them is a translation of a translation.
	// machineDone is, per target language, how many of those have been made.
	machine     int
	machineDone map[string]int
}

// Translated renders the block. The name is not Translation because that is
// already the audit group of the fifteen rules that check a translation once it
// exists, and this counts whether one exists at all.
func Translated(c *Corpus) string {
	rows, langs := translationRows(c)

	var b strings.Builder
	b.WriteString("\n")

	b.WriteString("| Book | Sections | Exercises |")
	for _, l := range langs {
		name := LangName(l)
		fmt.Fprintf(&b, " %s sections | %s exercises | Done |", name, name)
	}
	b.WriteString(" From machine English |\n| --- | --- | --- |")
	for range langs {
		b.WriteString(" --- | --- | --- |")
	}
	b.WriteString(" --- |\n")

	var sections, exercises, machine int
	total := map[string][2]int{}
	machineTotal := map[string]int{}
	for _, r := range rows {
		fmt.Fprintf(&b, "| %s | %d | %d |", r.bookTitle, r.sections, r.exercises)
		for _, l := range langs {
			d := r.done[l]
			fmt.Fprintf(&b, " %d | %d | %s |", d[0], d[1], percent(d[0]+d[1], r.sections+r.exercises))
			t := total[l]
			t[0] += d[0]
			t[1] += d[1]
			total[l] = t
			machineTotal[l] += r.machineDone[l]
		}
		fmt.Fprintf(&b, " %s |\n", machineCell(r.machine, r.sections+r.exercises))
		sections += r.sections
		exercises += r.exercises
		machine += r.machine
	}
	fmt.Fprintf(&b, "| **All** | **%d** | **%d** |", sections, exercises)
	for _, l := range langs {
		t := total[l]
		fmt.Fprintf(&b, " **%d** | **%d** | **%s** |", t[0], t[1], percent(t[0]+t[1], sections+exercises))
	}
	fmt.Fprintf(&b, " **%d** |\n\n", machine)

	held := langHoldings(c)
	fmt.Fprintf(&b, "The source column is the English, which is %d sections and %d exercises: %d files in `content/en` where Springer printed an English translation and %d in `content/en-mt` where this project read the French instead. The French originals are %d sections and %d exercises in `content/fr`, and they are counted here rather than compared, because a file name carries a slug of its own title and matching the two trees by path calls every honestly translated title a missing section.\n",
		sections, exercises, held["en"][0]+held["en"][1], held["en-mt"][0]+held["en-mt"][1],
		held["fr"][0], held["fr"][1])
	if len(langs) > 0 {
		var parts []string
		for _, l := range langs {
			t := total[l]
			parts = append(parts, fmt.Sprintf("%s has %d of the %d sections and %d of the %d exercises",
				LangName(l), t[0], sections, t[1], exercises))
		}
		fmt.Fprintf(&b, "\n%s. Sections here means every file that is not an exercise, so the introductions, the notes to the reader and the historical notes are counted with the §§.\n",
			joinList(parts))

		var hops []string
		for _, l := range langs {
			if machineTotal[l] > 0 {
				hops = append(hops, fmt.Sprintf("%d of the %d files in %s", machineTotal[l], total[l][0]+total[l][1], LangName(l)))
			}
		}
		if len(hops) > 0 {
			fmt.Fprintf(&b, "\nThe last column is the part of a Book that was never printed in English, so the only English of it is this project's own reading of the French. A translation made from one of those is a translation of a translation, and that is %s. Where the column says all of it the whole Book is in that position, and a hundred per cent in the Done column for such a Book is not the same claim as a hundred per cent for one Springer translated.\n",
				joinList(hops))
		}
	}
	return b.String()
}

// machineCell says how much of a Book has no printed English behind it. The
// whole of a Book is worth saying in words rather than leaving a reader to
// notice that two numbers on the row happen to be equal.
func machineCell(machine, all int) string {
	switch {
	case machine == 0:
		return "0"
	case machine == all:
		return fmt.Sprintf("%d, all of it", machine)
	}
	return fmt.Sprintf("%d", machine)
}

// percent writes a share the way the tables around it do, with no decimal,
// because a tenth of a per cent of four thousand exercises is four exercises
// and nobody is making a decision on that.
func percent(have, want int) string {
	if want == 0 {
		return "n/a"
	}
	return fmt.Sprintf("%.0f%%", 100*float64(have)/float64(want))
}

// translationRows is one row per Book that the English holds anything for, in
// the order the Éléments shelve their Books, with the target languages found
// under content/.
func translationRows(c *Corpus) ([]translationRow, []string) {
	langs := targetLangs(c)

	// The English of a Book, keyed by the path under the language directory,
	// which is what a translation of it will be called. content/en wins over
	// content/en-mt where both hold the same path, but only for deciding which
	// Book the file belongs to: either way it is one unit of work.
	type unit struct {
		book     string
		exercise bool

		// machine is whether the only English of this file is content/en-mt.
		// The two trees hold no path in common, so this is settled by which of
		// them the file was found in, and the precedence below is a guard
		// against a day when they do rather than a rule that fires now.
		machine bool
	}
	source := map[string]unit{}
	for _, d := range c.Docs {
		lang, rel, ok := contentPath(d.Path)
		if !ok || (lang != "en" && lang != "en-mt") {
			continue
		}
		book, _, _ := strings.Cut(rel, "/")
		if book == "" {
			continue
		}
		if _, seen := source[rel]; seen && lang != "en" {
			continue
		}
		source[rel] = unit{book: book, exercise: isExercisePath(rel), machine: lang == "en-mt"}
	}

	have := map[string]map[string]bool{}
	for _, l := range langs {
		have[l] = map[string]bool{}
	}
	for _, d := range c.Docs {
		lang, rel, ok := contentPath(d.Path)
		if !ok {
			continue
		}
		if set, want := have[lang]; want {
			set[rel] = true
		}
	}

	byBook := map[string]*translationRow{}
	for rel, u := range source {
		r, ok := byBook[u.book]
		if !ok {
			r = &translationRow{
				book: u.book, bookTitle: corpus.BookTitle(u.book),
				order: bookShelfOrder(c, u.book), done: map[string][2]int{},
				machineDone: map[string]int{},
			}
			byBook[u.book] = r
		}
		if u.exercise {
			r.exercises++
		} else {
			r.sections++
		}
		if u.machine {
			r.machine++
		}
		for _, l := range langs {
			if !have[l][rel] {
				continue
			}
			d := r.done[l]
			if u.exercise {
				d[1]++
			} else {
				d[0]++
			}
			r.done[l] = d
			if u.machine {
				r.machineDone[l]++
			}
		}
	}

	out := make([]translationRow, 0, len(byBook))
	for _, r := range byBook {
		out = append(out, *r)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].order != out[j].order {
			return out[i].order < out[j].order
		}
		return out[i].book < out[j].book
	})
	return out, langs
}

// targetLangs are the languages this project writes rather than reads. The
// source languages are the ones a registered volume is printed in, and en-mt is
// a source too for this table's purpose even though nothing was printed in it,
// because it is what a Vietnamese section of an unpublished chapter is made
// from.
func targetLangs(c *Corpus) []string {
	src := c.SourceLangs()
	var out []string
	for _, l := range c.Langs {
		if src[l] || l == "en-mt" {
			continue
		}
		out = append(out, l)
	}
	sort.Strings(out)
	return out
}

// langHoldings is how many sections and exercises each language tree holds
// outright, with no comparison to anything.
func langHoldings(c *Corpus) map[string][2]int {
	out := map[string][2]int{}
	for _, d := range c.Docs {
		lang, rel, ok := contentPath(d.Path)
		if !ok {
			continue
		}
		v := out[lang]
		if isExercisePath(rel) {
			v[1]++
		} else {
			v[0]++
		}
		out[lang] = v
	}
	return out
}

// bookShelfOrder is where a Book sits on the shelf, taken from the order
// manifests/books.yaml registers volumes in, which is the order the Éléments
// print them. A Book with content and no registered volume sorts last rather
// than first, so it shows up at the end of the table as the anomaly it is.
func bookShelfOrder(c *Corpus, book string) int {
	if c.Books == nil {
		return 1 << 20
	}
	n := 0
	seen := map[string]bool{}
	for _, b := range c.Books.Books {
		if seen[b.Book] {
			continue
		}
		seen[b.Book] = true
		if b.Book == book {
			return n
		}
		n++
	}
	return 1 << 20
}

// contentPath splits a corpus path into the language directory and the path
// under it, and says no for anything that is not under content/. The solutions
// are under content/ and are not a language, and they are excluded here by not
// being one of the languages any caller asks about.
func contentPath(path string) (lang, rel string, ok bool) {
	rest, ok := strings.CutPrefix(path, "content/")
	if !ok {
		return "", "", false
	}
	lang, rel, ok = strings.Cut(rest, "/")
	if !ok || lang == "" || rel == "" {
		return "", "", false
	}
	return lang, rel, true
}

// isExercisePath is whether a path under a language directory is an exercise.
//
// This reads the path and not the front matter on purpose. Front matter that
// will not parse is still a file somebody has to translate, and a file that is
// missing its kind should not quietly stop being counted as work.
func isExercisePath(rel string) bool {
	return strings.Contains(rel, "/exercises/")
}

// langNames is what to call a language in a sentence. A language not in this
// list is printed as its code, which is wrong looking enough that somebody will
// add it.
// en-mt is deliberately not in here. It is machine English and the pages say so
// where it appears, so a bare "English" would be the one thing a reader of
// those pages must not be told.
var langNames = map[string]string{
	"en": "English",
	"fr": "French",
	"vi": "Vietnamese",
	"zh": "Chinese",
	"ja": "Japanese",
}

// LangName is the printed name of a language code.
func LangName(code string) string {
	if name := langNames[code]; name != "" {
		return name
	}
	return code
}
