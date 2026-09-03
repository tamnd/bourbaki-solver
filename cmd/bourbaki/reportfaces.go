package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

const facesUsage = `usage: bourbaki report faces [flags]

Every \mathcal and \mathfrak argument a volume's pages carry, with how many
times it appears, how many pages it is on, and which ones.

This exists because a reader can collapse distinct letters onto one command and
nothing downstream can tell. olmOCR emitted \mathcal{G} 237 times over 25 pages
of top-v-x-fr, and the printing has three different Fraktur capitals underneath
it: F for the filter in the chapter VII exercise on rearranging a series, G for
its partner in the Baire hierarchy of chapter IX, and S for the S-convergence
chapter X is about. The TeX compiles, the audit passes, and the book is simply
wrong about which set system it is talking about.

It is not a fault a substitution can repair, which is why this reports and does
not correct. Page 228 of that volume carries Fraktur S and Fraktur F in the same
paragraph and the reader mapped the first to \mathcal{G} and the second to
\mathcal{F}, so the mapping is not consistent inside one page and no rule of the
form "this book's \mathcal{G} means Fraktur S" comes out right. The same volume
has 9 unsubscripted \mathcal{F} on page 239 that really are script F sitting a
few lines from subscripted ones that are Fraktur. fvr-i-vii-fr has the mirror
image, \mathfrak{g} where the printing has Fraktur F, 72 times over 9 pages with
the interleaved pages read correctly.

What makes the shape findable is the ratio and the isolation: a command that
appears 237 times in one volume and in no other volume of the corpus is worth a
look on its own. So the books column is the one to read first. A face that is
heavy in one volume and absent everywhere else is either a letter that volume
alone uses or a letter the reader invented, and those are told apart by looking
at the pages, which is why they are listed.

flags:
  -book NAME     only this volume
  -min N         only faces appearing at least N times, 1 by default
  -alone         only faces no other volume uses, which is the shape above
  -pages N       list at most this many pages per face, 12 by default, 0 for all
  -json          print the report as JSON
`

// faceUse is one face command's argument as one volume writes it.
type faceUse struct {
	Book  string `json:"book"`
	Face  string `json:"face"`
	Arg   string `json:"arg"`
	Count int    `json:"count"`
	Pages []int  `json:"pages"`
	// Books is how many volumes of the corpus write this same face and
	// argument, this one included. One means nobody else does.
	Books int `json:"books"`
}

// faceArg matches \mathcal{G} and \mathcal G, and the fraktur form, taking the
// whole argument rather than a letter: \mathfrak{su} is a thing the books set
// and splitting it would report two letters the printing does not have.
var faceArg = regexp.MustCompile(`\\(mathcal|mathfrak)\s*(?:\{([^{}]*)\}|([A-Za-z]))`)

func reportFacesCmd(args []string) error {
	fs := flag.NewFlagSet("report faces", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, facesUsage) }
	book := fs.String("book", "", "only this volume")
	min := fs.Int("min", 1, "only faces appearing at least this many times")
	alone := fs.Bool("alone", false, "only faces no other volume uses")
	pages := fs.Int("pages", 12, "list at most this many pages per face")
	asJSON := fs.Bool("json", false, "print the report as JSON")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return err
	}

	if _, ok := books.Get(*book); *book != "" && !ok {
		return fmt.Errorf("no volume %q in %s", *book, corpus.BooksPath(root))
	}

	// Every volume is read even when one was asked for, because the books
	// column is the point of the report and it cannot be counted from one
	// volume. Asking for a book filters what is printed and not what is read.
	all := map[string][]faceUse{}
	for _, entry := range books.Books {
		uses, err := facesOf(root, entry.ID)
		if err != nil {
			return err
		}
		for _, u := range uses {
			all[u.Book] = append(all[u.Book], u)
		}
	}
	across := map[[2]string]int{}
	for _, uses := range all {
		for _, u := range uses {
			across[[2]string{u.Face, u.Arg}]++
		}
	}

	var out []faceUse
	for _, entry := range books.Books {
		if *book != "" && entry.ID != *book {
			continue
		}
		for _, u := range all[entry.ID] {
			u.Books = across[[2]string{u.Face, u.Arg}]
			if u.Count < *min || (*alone && u.Books > 1) {
				continue
			}
			out = append(out, u)
		}
	}
	// Heaviest first, because the ratio is what makes the shape findable.
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count != out[j].Count {
			return out[i].Count > out[j].Count
		}
		if out[i].Book != out[j].Book {
			return out[i].Book < out[j].Book
		}
		return out[i].Face+out[i].Arg < out[j].Face+out[j].Arg
	})
	if *asJSON {
		return printJSON(out)
	}
	printFaces(out, *pages)
	return nil
}

// facesOf reads one volume's pages and counts the face arguments on each.
func facesOf(root, id string) ([]faceUse, error) {
	paths, err := filepath.Glob(filepath.Join(corpus.PagesDir(root, id), "*.md"))
	if err != nil {
		return nil, err
	}
	sort.Strings(paths)
	count := map[[2]string]int{}
	on := map[[2]string][]int{}
	for _, path := range paths {
		file, err := corpus.ReadFile[corpus.PageFrontMatter](path)
		if err != nil {
			return nil, err
		}
		seen := map[[2]string]bool{}
		for _, m := range faceArg.FindAllStringSubmatch(file.Body, -1) {
			arg := m[2]
			if arg == "" {
				arg = m[3]
			}
			if arg == "" {
				continue
			}
			key := [2]string{m[1], arg}
			count[key]++
			if !seen[key] {
				seen[key] = true
				on[key] = append(on[key], file.Meta.PDFPage)
			}
		}
	}
	var out []faceUse
	for key, n := range count {
		out = append(out, faceUse{Book: id, Face: key[0], Arg: key[1], Count: n, Pages: on[key]})
	}
	return out, nil
}

func printFaces(uses []faceUse, most int) {
	if len(uses) == 0 {
		fmt.Println("no \\mathcal or \\mathfrak argument matches")
		return
	}
	fmt.Printf("%-16s %-22s %7s %7s %7s  %s\n",
		"book", "face", "times", "pages", "books", "on")
	for _, u := range uses {
		fmt.Printf("%-16s %-22s %7d %7d %7d  %s\n", u.Book,
			`\`+u.Face+"{"+u.Arg+"}", u.Count, len(u.Pages), u.Books,
			facePages(u.Pages, most))
	}
}

// facePages prints the pages a face is on, cut off where the list stops being
// something to read. A face on every page of a volume is not the shape this
// report is looking for and its page list says nothing.
func facePages(pages []int, most int) string {
	var parts []string
	for i, p := range pages {
		if most > 0 && i == most {
			parts = append(parts, fmt.Sprintf("and %d more", len(pages)-most))
			break
		}
		parts = append(parts, fmt.Sprintf("%04d", p))
	}
	return strings.Join(parts, " ")
}
