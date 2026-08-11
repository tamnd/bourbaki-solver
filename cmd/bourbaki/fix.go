package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sort"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/extract"
	"github.com/tamnd/bourbaki-solver/mathtex"
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
  math      put the characters stranded outside their TeX back inside it

Run stray before math: everything after an unclosed delimiter reads as
mathematics, and math will not touch a span whose end it cannot see.

Run bourbaki fix <command> -h for the flags of a command.
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
