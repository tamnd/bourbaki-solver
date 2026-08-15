package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/extract"
	"github.com/tamnd/bourbaki-solver/mathtex"
	"github.com/tamnd/bourbaki-solver/pagemap"
)

// fix is the repairs that are a function of the Markdown alone: no PDF, no
// model, no network, and no judgement about what the book meant.
//
// It works on pages/ and not on content/. The section and exercise files are
// what assemble makes of the pages, so a repair written into content/ lasts
// until the next assemble and no longer, and the same repair written into a
// page survives it. The order is fix, then assemble, then audit, and the last
// of the three is what says whether the first worked.
//
// extract does the same repair as it writes each page, so a volume read in
// after this needs nothing done to it. This command is for the pages that were
// read before the repair existed.

const fixUsage = `usage: bourbaki fix <command> [arguments]

Repairs the committed Markdown in the ways that need no PDF and no model.

commands:
  stray     take out a delimiter that opens mathematics and closes nothing
  parens    put a bracket that belongs to the prose back outside the formula
  math      put the characters stranded outside their TeX back inside it
  folio     move the printed page number off the foot and into the front matter

Run the first three in that order. Everything after an unclosed delimiter reads
as mathematics, so stray comes first and the other two will not touch a span
whose end they cannot see, and parens comes before math so that math reads the
spans as they will be rather than as they are. folio touches no mathematics and
can be run at any point before assemble.

Run bourbaki fix <command> -h for the flags of a command.
`

const fixFolioUsage = `usage: bourbaki fix folio [flags]

Moves the printed page number off the foot of a page body and into the front
matter, on the volumes that print it there.

Five of the volumes print the number in the running head, where SplitHead files
it as the page is read. Theory of Sets and Algebra I to III print it at the foot
instead, so it comes back at the end of the body and stays there, and a section
assembled out of twenty such pages carries twenty bare numbers standing between
its paragraphs. The number is furniture in both printings and belongs in the
same place in both.

It runs after the reading and not during it, for two reasons. The reading is
faithful to the page and a page that prints a number has one. And a volume with
no text layer has its page map built out of these bodies, by reading the number
off the foot, so the number has to be there when pagemap build runs.

The page map is the check. It already says what number is printed on each PDF
page, and a page whose foot disagrees with it is left alone and named, since a
disagreement means one of the two is wrong and quietly believing either is how a
corpus ends up mispaginated. Where they agree the number is written to folio.

A page label is not built from it. A label such as "A VIII.13" counts pages
inside a chapter, and both volumes that print their number at the foot number
their pages straight through the book, so "E IV.289" would claim a page 289 of a
chapter that runs to sixty pages. The bare number is what those volumes print
and it is all that is recorded. A foot-number volume paginated per chapter would
get a label, and there is none in the corpus today.

flags:
  -book ID   only this volume, default every volume that prints its folio at the foot
  -check     say what would change and change nothing
`

const fixParensUsage = `usage: bourbaki fix parens [flags]

Moves a closing bracket that belongs to the prose back out of the mathematics
the text layer swept it into.

It is the function whose name Bourbaki sets upright. The name and its opening
bracket come through as prose and the closing one comes through inside the
formula, so the page reads Tr($u)$ where it should read Tr($u$). The two print
the same, which is why nobody catches it by reading, and they are not the same
text: the mathematics of the first is "u)". A translator asked to copy the
formulae hands back "u", correctly, and the audit refuses the section because a
translation may not alter mathematics.

It repairs only a bracket standing immediately before an opening delimiter with
nothing in between, which is the shape that comes from a name. A straddle with
a space in front of it is the sentence's own bracket, as in "(resp. $x$)", and
is left alone. No more brackets come out of a span than the line has open, and
the page with every delimiter removed has to be the page it started as, so this
moves delimiters and never a character of prose or of mathematics.

Run bourbaki assemble afterwards, or the section files still hold the old text.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixStrayUsage = `usage: bourbaki fix stray [flags]

Takes out a dollar sign that opens mathematics and never closes it, on the
pages where taking it out leaves the page balanced.

It is the numbered display that the text layer flattened into a line of prose,
leaving the display's own delimiter at the end against the full stop. From
there to the foot of the page reads as one long formula, which is why the page
is reported unbalanced and why fix math stops at that line.

A page that does not balance without the delimiter is damaged in some other
way and is left alone, so this repairs nothing it cannot check. A page it does
repair keeps the stray-delimiter flag, because the mathematics is then right
and the setting is still wrong: what was printed as a display is now prose,
and only the printed page says how it should be set.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixMathUsage = `usage: bourbaki fix math [flags]

Rewrites every page of the corpus, putting a character that is printed as TeX
everywhere else back into its TeX where the mathematics has it as a bare
glyph: Greek capitals mostly, which Bourbaki sets upright so the extractor
sees prose, and the increment sign standing in for \Delta.

It substitutes one glyph for the TeX that prints the same glyph and does
nothing else. Two characters are ambiguous, a capital sigma and a capital pi
carrying a subscript, since either is as likely to be a sum or a product as a
letter, and those are left alone and printed for somebody to read the page and
decide. M03 reports them too.

Run bourbaki assemble afterwards, or the section files still hold the old text.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

func runFix(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, fixUsage)
		os.Exit(2)
	}
	switch args[0] {
	case "math":
		return fixMath(args[1:])
	case "stray":
		return fixStray(args[1:])
	case "parens":
		return fixParens(args[1:])
	case "folio":
		return fixFolio(args[1:])
	}
	fmt.Fprint(os.Stderr, fixUsage)
	os.Exit(2)
	return nil
}

func fixStray(args []string) error {
	fs := flag.NewFlagSet("fix stray", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixStrayUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, left int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, ok := mathtex.DropStray(f.Body)
		// A page the extractor could not read is not repaired by counting
		// delimiters. See DropStray for the four pages that taught us that.
		if len(f.Meta.Flags) > 0 {
			ok = false
		}
		if !ok {
			// Either there is no unclosed delimiter, which is the usual case,
			// or there is one and this is not the fault it repairs. The second
			// is a page for the repair pass against the printed image, and it
			// is named here so the two are not confused in the summary.
			if _, un := mathtex.Split(f.Body); un != nil {
				left++
				fmt.Fprintf(os.Stderr, "fix stray: left alone, %s:%d is not a stray display delimiter\n",
					rel(root, path), un.Line)
			}
			return nil
		}
		changed++
		if *check {
			fmt.Printf("%s  line %d\n", rel(root, path), strayLine(f.Body))
			return nil
		}
		f.Body = body
		f.Meta.Flags = withFlag(f.Meta.Flags, string(extract.FlagStrayDelimiter))
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	verb := "took out"
	if *check {
		verb = "would take out"
	}
	fmt.Printf("fix stray: %d pages read, %s a delimiter on %d of them, %d left alone\n",
		pages, verb, changed, left)
	if changed > 0 && !*check {
		fmt.Println("fix stray: run bourbaki fix math, then bourbaki assemble")
	}
	return nil
}

// strayLine is the line the unclosed delimiter sits on, for the report.
func strayLine(body string) int {
	if _, un := mathtex.Split(body); un != nil {
		return un.Line
	}
	return 0
}

// withFlag adds a flag to a page's flags and keeps them sorted and unique. The
// unbalanced flag stays where it is: a page that balanced only after a
// delimiter was dropped is not a page that was always balanced.
func withFlag(flags []string, f string) []string {
	if slices.Contains(flags, f) {
		return flags
	}
	flags = append(flags, f)
	sort.Strings(flags)
	return flags
}

// eachPage walks the committed pages of one volume, or of every volume, in
// reading order.
func eachPage(root string, books *corpus.BooksManifest, book string, fn func(path string, f *corpus.PageFile) error) error {
	for _, b := range books.Books {
		if book != "" && b.ID != book {
			continue
		}
		names, err := filepath.Glob(filepath.Join(corpus.PagesDir(root, b.ID), "*.md"))
		if err != nil {
			return err
		}
		sort.Strings(names)
		for _, path := range names {
			f, err := corpus.ReadFile[corpus.PageFrontMatter](path)
			if err != nil {
				return err
			}
			if err := fn(path, &f); err != nil {
				return err
			}
		}
	}
	return nil
}

// corpusAndBooks is the two lookups every fix command opens with.
func corpusAndBooks() (string, *corpus.BooksManifest, error) {
	root, err := corpus.Root()
	if err != nil {
		return "", nil, err
	}
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return "", nil, err
	}
	return root, books, nil
}

func fixParens(args []string) error {
	fs := flag.NewFlagSet("fix parens", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixParensUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, spans int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, n := mathtex.Unstraddle(f.Body)
		if n == 0 || body == f.Body {
			return nil
		}
		changed++
		spans += n
		if *check {
			word := "spans"
			if n == 1 {
				word = "span"
			}
			fmt.Printf("%s  %d %s\n", rel(root, path), n, word)
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	verb := "repaired"
	if *check {
		verb = "would repair"
	}
	fmt.Printf("fix parens: %d pages read, %s %d spans in %d of them\n", pages, verb, spans, changed)
	if changed > 0 && !*check {
		fmt.Println("fix parens: run bourbaki fix math, then bourbaki assemble")
	}
	return nil
}

func fixFolio(args []string) error {
	fs := flag.NewFlagSet("fix folio", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixFolioUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, labelled, disagreed, missing int
	for _, b := range books.Books {
		if *book != "" && b.ID != *book {
			continue
		}
		if pagemap.Grammar(b.Grammar) != pagemap.FootNumber {
			continue
		}
		pm, err := pagemap.Load(root, b.ID)
		if err != nil {
			return fmt.Errorf("%s: %w: run bourbaki pagemap build -book %s first", b.ID, err, b.ID)
		}
		// A label is only built for a volume that numbers its pages inside the
		// chapter, because that is what a label says. Both foot-number volumes
		// in the corpus number straight through the book, so in practice this
		// writes the number and no label, and says so in the summary.
		letter := corpus.BookLetter(b.Book)
		if pagemap.Pagination(b.Pagination) != pagemap.PerChapter {
			letter = ""
		}
		err = eachPage(root, books, b.ID, func(path string, f *corpus.PageFile) error {
			pages++
			body, folio := corpus.CutFolio(f.Body)
			if folio == 0 {
				// Most of these are a page the volume prints no number on: the
				// opener of a chapter, a plate, the blank facing one. They are
				// not worth a line each, only a count.
				missing++
				return nil
			}
			e, ok := pm.Lookup(f.Meta.PDFPage)
			if !ok || e.Page != folio {
				want := 0
				if ok {
					want = e.Page
				}
				disagreed++
				fmt.Fprintf(os.Stderr, "fix folio: left alone, %s prints %d and the page map says %d\n",
					rel(root, path), folio, want)
				return nil
			}
			label := f.Meta.PageLabel
			if label == "" && letter != "" && e.Chapter != "" {
				label = fmt.Sprintf("%s %s.%d", letter, e.Chapter, folio)
			}
			changed++
			if label != f.Meta.PageLabel {
				labelled++
			}
			if *check {
				fmt.Printf("%s  %d  %s\n", rel(root, path), folio, label)
				return nil
			}
			f.Body, f.Meta.Folio, f.Meta.PageLabel = body, folio, label
			f.Meta.Lines = len(strings.Split(strings.TrimSpace(body), "\n"))
			return f.Write(path)
		})
		if err != nil {
			return err
		}
	}

	verb := "took"
	if *check {
		verb = "would take"
	}
	fmt.Printf("fix folio: %d pages read, %s the number off %d of them, %d print none, %d left alone\n",
		pages, verb, changed, missing, disagreed)
	if labelled > 0 {
		fmt.Printf("fix folio: %d pages got a page label\n", labelled)
	}
	if changed > 0 && !*check {
		fmt.Println("fix folio: run bourbaki assemble to carry this into the section files")
	}
	return nil
}

func fixMath(args []string) error {
	fs := flag.NewFlagSet("fix math", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixMathUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var refused []mathtex.Refusal
	var pages, changed, chars int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, n, ref := mathtex.Repair(f.Body)
		for i := range ref {
			ref[i].File = rel(root, path)
		}
		refused = append(refused, ref...)
		if n == 0 || body == f.Body {
			return nil
		}
		changed++
		chars += n
		if *check {
			fmt.Printf("%s  %d characters\n", rel(root, path), n)
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	for _, r := range refused {
		fmt.Fprintln(os.Stderr, "fix math: left alone, "+r.String())
	}
	verb := "repaired"
	if *check {
		verb = "would repair"
	}
	fmt.Printf("fix math: %d pages read, %s %d characters in %d of them, %d left alone\n",
		pages, verb, chars, changed, len(refused))
	if changed > 0 && !*check {
		fmt.Println("fix math: run bourbaki assemble to carry this into the section files")
	}
	return nil
}
