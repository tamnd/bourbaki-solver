package main

import (
	"flag"
	"fmt"
	iofs "io/fs"
	"os"
	"path/filepath"
	"slices"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/extract"
	"github.com/tamnd/bourbaki-solver/footnote"
	"github.com/tamnd/bourbaki-solver/mathtex"
	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/toc"
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
// A translation is the exception, and footnote is the one repair that takes it.
// No page makes content/vi and no assemble rewrites it, so a repair that stops
// at pages/ leaves the Vietnamese carrying what the English has stopped
// carrying, and the only other way to move it is to pay a model to do the
// section again.
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
  heading   set a numbered heading at the level the table of contents gives it
  footnote  take the printed mark off a footnote that already has a reference
  seal      write content_sha256 over a section body that was edited by hand

Run the first three in that order. Everything after an unclosed delimiter reads
as mathematics, so stray comes first and the other two will not touch a span
whose end they cannot see, and parens comes before math so that math reads the
spans as they will be rather than as they are. folio and heading touch no
mathematics and can be run at any point before assemble. seal works on content/
and not on pages/, and is the last thing run after a hand correction.

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

A reading that dropped the number is the other case, and -fill is for it. Page
32 of Theory of Sets prints 25 at the foot and the reading came back without it,
so there is nothing to move and the page has no folio at all. The page map has
one, read off that same foot when the map was built, so -fill writes it and
flags the page with where the number came from. It fills only a number the map
read off the page, never one the map worked out from the pages around it, since
that number is printed nowhere and a reader holding the book will not find it.
It is off by default because a number in the front matter of a page that does
not show one in its body is worth saying out loud rather than doing quietly to a
whole corpus.

flags:
  -book ID   only this volume, default every volume that prints its folio at the foot
  -check     say what would change and change nothing
  -fill      give a page whose reading dropped the number the one the page map
             holds, flagged as coming from there
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

It repairs a span the prose of the line has a bracket open against, whether the
bracket stands against the delimiter as in Tr($u)$ or a whole clause back as in
(cf. INT, VIII, §2, n$^o6)$. Where the prose holds nothing open the bracket in
the span is the mathematics' own and is left alone, so "$\alpha$)" at the head
of a list item and "$f(x$ and $y)$" both stay as they are, and so does a span
still holding a square bracket open, which is a half-open interval and not a
straddle. No more brackets come out of a span than the line has open, and the
page with every delimiter removed has to be the page it started as, so this
moves delimiters and never a character of prose or of mathematics.

Run bourbaki assemble afterwards, or the section files still hold the old text.

It runs over content/ as well as pages/. A translation has no page under it and
holds the fault all the same, having been written by a model copying the
mathematics of a source that had it, and the fleet answers too few sections an
hour for re-asking one over a bracket to be a trade worth making. A translation
is repaired the same way, and its source_content_sha256 is moved on only when it
recorded the source body as it stood before this repair. One that was already
stale stays stale, so L05 still means what it says.

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

const fixHeadingUsage = `usage: bourbaki fix heading [flags]

Sets a numbered heading at the level the table of contents gives it.

A § and a no. are printed the same way, a number and a title alone on a line,
and the only thing separating them on the page is the size of the type. The
reading decides by that and on Theory of Sets it decided wrong eight times, all
in the same direction: a no. written as a §. Chapter III, § 1 then carried
twelve no. where the contents lists thirteen, and the assembler stopped there.

manifests/toc.yaml is the authority. It gives every § and every no. with the
page it begins on, so the heading is looked up rather than guessed at. Both the
number and the title have to agree with it: a § and its first no. begin on the
same page in most §§ and both are numbered 1, so the number alone would make
the no. into a second §.

It changes the level and nothing else. The number, the title, the supplementary
star and the rest of the page are written back as they stand, and a heading the
contents does not put on that page is left alone and named, since a heading in
a place the contents does not know about is a disagreement worth reading rather
than a level worth changing.

Run bourbaki assemble afterwards, or the section files still hold the old text.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixFootnoteUsage = `usage: bourbaki fix footnote [flags]

Takes the mark a volume prints beside a footnote off the pages that kept it.

The volumes mark their notes with symbols, restarting on every page: an asterisk
for the first, a dagger for the second, two asterisks for the third. Markdown
numbers its notes itself and prints the number it chose, so a page that keeps
the printed symbol carries two marks for one note, "(*)[^1]" in the body and
"[^1]: (*)" at the foot, and both of them reach the reader.

The symbol is not thrown away. It is the only thing that says which note a
reference belongs to, and the pages this exists for include the ones where the
reading wrote the symbol and no reference at all. So the symbols are read off
the definitions of the page first, and one standing on its own becomes the
reference whose definition carries it. A symbol that two notes of the same file
share, or one whose note is already pointed at from somewhere else, is left
where it is and named: sending a reader to the wrong note is worse than leaving
the printing's mark on the page.

It runs over content/ as well as pages/, which no other repair here does. The
English files are rewritten by the next assemble whatever this does, but a
translation is not: it was made by a model, months of gateway time went into it,
and re-translating fourteen sections over a printer's asterisk is not a trade
anybody should make. A translation is repaired the same way, and its
source_content_sha256 is moved on only when it recorded the English body as it
stood before the repair. One that was already stale stays stale, so L05 still
means what it says.

Run bourbaki assemble afterwards. The pages are the source and the English
sections are made from them, and the two say different things until it runs.

flags:
  -book ID   only this volume, default every volume that has pages
  -check     say what would change and change nothing
`

const fixSealUsage = `usage: bourbaki fix seal [flags]

Writes content_sha256 over a section file whose body no longer hashes to it.

The hash is what tells a stale translation from a current one, so nothing may
write it without meaning to, and no command did: assemble writes a section from
its pages and seals it on the way out, and a correction made in content/ by hand
leaves the body one thing and the hash another. S08 then refuses the corpus and
says so, correctly, and there was nothing to run. This is that thing.

It is not a repair of the text and it does not look at the text. It reads what
the file says its body hashes to, hashes the body, and where the two differ it
writes the second. A file already sealed is not rewritten.

manifests/sections.yaml records the same hash a second time and is written with
it, since assemble -check compares the manifest it would write against the
committed one and a section sealed without its row fails that check with no way
to pass it. Only a row the manifest already has is touched.

Sealing an English section restales its translations, which is the point: the
English moved, so the Vietnamese was made from a body that is no longer there.
The translations that recorded the old hash are named, because a hand correction
is usually a comma and a stale translation over a comma is worth knowing about
before the next run spends an hour redoing the section.

Prefer the correction in pages/ where the page is what was misread, since
assemble overwrites the section from the page and the hand correction with it.
Use this where the fault is in the assembly and not in the page.

flags:
  -lang L    only this language, default every language
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
	case "heading":
		return fixHeading(args[1:])
	case "footnote":
		return fixFootnote(args[1:])
	case "seal":
		return fixSeal(args[1:])
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
	files, content, followed, err := parensContent(root, *check)
	if err != nil {
		return err
	}

	fmt.Printf("fix parens: %d pages read, %s %d spans in %d of them\n", pages, verb, spans, changed)
	verbed := "moved"
	if *check {
		verbed = "would move"
	}
	fmt.Printf("fix parens: %d content files read, %d of them changed, %d translations %s on\n",
		files, content, followed, verbed)
	if changed > 0 && !*check {
		fmt.Println("fix parens: run bourbaki fix math, then bourbaki assemble")
	}
	return nil
}

// parensContent repairs the same fault in content/, which the pages pass cannot
// reach on its own.
//
// English and French are assembled from their pages, so repairing a page and
// assembling again carries the repair into the section. A translation has no
// page under it. It was written by a model copying the mathematics of a source
// that held the fault, so it holds the fault too, and the only way to it is to
// edit the file. Waiting for the section to be asked for again would work and it
// is not free: the fleet answers a few sections an hour on a good day, and there
// are seven hundred sections with no translation at all waiting behind them.
//
// This is fix footnote's arrangement and it is here for fix footnote's reason. A
// translation whose source is repaired goes stale by its hash, and re-asking it
// over a bracket that moved by a rule neither side had a choice about is not a
// trade anybody should make. So a source is walked first, its body hashed before
// and after, and a translation that recorded the first is moved on to the second.
// Only that translation: one that was already stale stays stale, so L05 still
// means what it says.
//
// A source here is a file that was not translated from anything, which is the
// English and the French alike, and it is not a test on the language. content/en-mt
// is English and it is a translation of the French, and moving it on with the
// French it was made from is the whole point of having the rule at all.
func parensContent(root string, check bool) (files, changed, followed int, err error) {
	// The source body each translation was made from, before and after, keyed by
	// the corpus-relative path the translation names.
	moved := map[string][2]string{}
	record := func(path, before, after string) {
		moved[filepath.ToSlash(rel(root, path))] = [2]string{
			corpus.ContentSHA256(before), corpus.ContentSHA256(after)}
	}
	follow := func(from, recorded string) (string, bool) {
		pair, ok := moved[from]
		if !ok || pair[0] != recorded || pair[0] == pair[1] {
			return recorded, false
		}
		return pair[1], true
	}

	var sources bool
	section := func(path string, f *corpus.File[corpus.SectionFrontMatter]) error {
		if (f.Meta.TranslatedFrom == "") != sources {
			return nil
		}
		files++
		body, n := mathtex.Unstraddle(f.Body)
		if sources {
			record(path, f.Body, body)
		} else if now, ok := follow(f.Meta.TranslatedFrom, f.Meta.SourceSHA256); ok {
			followed++
			if !check {
				f.Meta.SourceSHA256 = now
			}
			n++
		}
		if n == 0 {
			return nil
		}
		changed++
		if check {
			fmt.Printf("%s  %d spans\n", rel(root, path), n)
			return nil
		}
		f.Body = body
		return f.Write(path) // Write hashes the body again, so the seal follows
	}
	exercise := func(path string, f *corpus.File[corpus.ExerciseFrontMatter]) error {
		if (f.Meta.TranslatedFrom == "") != sources {
			return nil
		}
		files++
		body, n := mathtex.Unstraddle(f.Body)
		if sources {
			record(path, f.Body, body)
		} else if now, ok := follow(f.Meta.TranslatedFrom, f.Meta.SourceSHA256); ok {
			followed++
			if !check {
				f.Meta.SourceSHA256 = now
			}
			n++
		}
		if n == 0 {
			return nil
		}
		changed++
		if check {
			fmt.Printf("%s  %d spans\n", rel(root, path), n)
			return nil
		}
		f.Body = body
		return f.Write(path)
	}
	// The sources first and on their own, because a translation cannot be moved
	// on until the body it was made from has been repaired and hashed twice.
	for _, sources = range []bool{true, false} {
		if err := eachSection(root, "", section); err != nil {
			return files, changed, followed, err
		}
		if err := eachExercise(root, "", exercise); err != nil {
			return files, changed, followed, err
		}
	}
	return files, changed, followed, nil
}

func fixFolio(args []string) error {
	fs := flag.NewFlagSet("fix folio", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixFolioUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	fill := fs.Bool("fill", false, "take the number from the page map when the body has none")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var pages, changed, labelled, disagreed, missing, filled int
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
				if f.Meta.Folio != 0 {
					// Already repaired by an earlier run, which is why the
					// body has no number left to take.
					return nil
				}
				// Most of the rest are a page the volume prints no number on:
				// the opener of a chapter, a plate, the blank facing one. They
				// are not worth a line each, only a count.
				missing++
				e, ok := pm.Lookup(f.Meta.PDFPage)
				if !*fill || !ok || e.Page == 0 || !e.Confidence.Printed() {
					return nil
				}
				filled++
				label := folioLabel(f.Meta.PageLabel, letter, e.Chapter, e.Page)
				if label != f.Meta.PageLabel {
					labelled++
				}
				if *check {
					fmt.Printf("%s  %d  %s  from the page map\n", rel(root, path), e.Page, label)
					return nil
				}
				f.Meta.Folio, f.Meta.PageLabel = e.Page, label
				f.Meta.Flags = withFlag(f.Meta.Flags, folioFromMap)
				return f.Write(path)
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
			label := folioLabel(f.Meta.PageLabel, letter, e.Chapter, folio)
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
	if filled > 0 {
		verb := "took"
		if *check {
			verb = "would take"
		}
		fmt.Printf("fix folio: of those %d, %s the number of %d from the page map\n", missing, verb, filled)
	}
	if labelled > 0 {
		fmt.Printf("fix folio: %d pages got a page label\n", labelled)
	}
	if changed+filled > 0 && !*check {
		fmt.Println("fix folio: run bourbaki assemble to carry this into the section files")
	}
	return nil
}

// folioLabel keeps the label a page already carries, and builds one for a
// volume that numbers inside the chapter and has none.
func folioLabel(has, letter, chapter string, folio int) string {
	if has != "" || letter == "" || chapter == "" {
		return has
	}
	return fmt.Sprintf("%s %s.%d", letter, chapter, folio)
}

// folioFromMap is what a filled page is flagged with, because a number that
// came from somewhere other than the page in front of the reader is worth
// saying so on the page.
//
// Only a number the page map read off the page is filled in, never one it
// worked out from the pages around it. The two look the same in the map and are
// not the same claim at all: the first says the volume prints this number and
// the reading dropped it, which is a repair, and the second says nothing on the
// page carries a number, which makes writing one an invention. A reader holding
// the printed book would go looking for it at the foot and find blank paper.
const folioFromMap = "folio from the page map, printed on the page and dropped by this reading"

func fixHeading(args []string) error {
	fs := flag.NewFlagSet("fix heading", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixHeadingUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}
	man, err := corpus.LoadTOC(root)
	if err != nil {
		return err
	}

	var pages, changed, unknown int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		bt, ok := man.Get(f.Meta.Book)
		if !ok {
			// A volume whose contents has not been read yet. There is nothing
			// to look a heading up in, which is not a fault of the page.
			return nil
		}
		lines := strings.Split(f.Body, "\n")
		moved := false
		for i, line := range lines {
			h, ok := toc.ParseHeading(line)
			if !ok {
				continue
			}
			level := toc.Level(*bt, f.Meta.PDFPage, h.Number, h.Title)
			switch {
			case level == 0:
				// The contents does not give this heading on this page. The
				// front pages of a volume are mostly this: the contents itself
				// is set as a list of numbered titles and reads as a page full
				// of headings. So it is counted and not printed one by one.
				unknown++
			case level != h.Level:
				lines[i] = h.Write(level)
				moved = true
				if *check {
					fmt.Printf("%s:%d  %s\n", rel(root, path), i+1, lines[i])
				}
			}
		}
		if !moved {
			return nil
		}
		changed++
		if *check {
			return nil
		}
		f.Body = strings.Join(lines, "\n")
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	verb := "moved a heading on"
	if *check {
		verb = "would move a heading on"
	}
	heading := "headings"
	if unknown == 1 {
		heading = "heading"
	}
	fmt.Printf("fix heading: %d pages read, %s %d of them, %d %s the contents does not have\n",
		pages, verb, changed, unknown, heading)
	if changed > 0 && !*check {
		fmt.Println("fix heading: run bourbaki assemble")
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

// fixSeal writes content_sha256 over the body it no longer describes.
//
// The two passes are one walk each and not one walk with a lookup, because the
// translations that go stale are named against the hash the English had before
// this run, and that is not known until the first walk is over.
func fixSeal(args []string) error {
	fs := flag.NewFlagSet("fix seal", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixSealUsage) }
	lang := fs.String("lang", "", "only this language")
	check := fs.Bool("check", false, "change nothing")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}

	// The hash each resealed file used to carry, against the file it was in, so
	// that a translation recording it can be named by what it was made from.
	broke := map[string]string{}
	// The new hash against the corpus-relative path, for the manifest.
	now := map[string]string{}
	var read, sealed int
	err = eachSection(root, *lang, func(path string, f *corpus.File[corpus.SectionFrontMatter]) error {
		read++
		want := corpus.ContentSHA256(f.Body)
		// Every file, and not only the ones sealed here. The manifest row can be
		// stale on its own: seal a section today and the row is written with it,
		// but a section sealed before this command existed left a row behind that
		// nothing has been through since.
		now[filepath.ToSlash(rel(root, path))] = want
		if f.Meta.ContentSHA256 == want {
			return nil
		}
		sealed++
		fmt.Printf("%s  %s is now %s\n", rel(root, path),
			short(f.Meta.ContentSHA256), short(want))
		if f.Meta.ContentSHA256 != "" {
			broke[f.Meta.ContentSHA256] = rel(root, path)
		}
		if *check {
			return nil
		}
		// Write recomputes the hash from the body, so the field is not set here.
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	// manifests/sections.yaml records the same hash a second time, and the two
	// have to move together: assemble -check compares the manifest it would
	// write against the committed one, and a section sealed here without the
	// manifest is a corpus that fails that check with no way to pass it. The
	// volumes this command is for are the ones assemble will not run on, so
	// rewriting the manifest from a fresh assembly is not open to us.
	rows, err := sealManifest(root, now, *check)
	if err != nil {
		return err
	}

	var stale int
	if len(broke) > 0 {
		err = eachSection(root, "", func(path string, f *corpus.File[corpus.SectionFrontMatter]) error {
			from, ok := broke[f.Meta.SourceSHA256]
			if !ok {
				return nil
			}
			stale++
			fmt.Printf("%s  was made from %s as it stood and is now stale\n",
				rel(root, path), from)
			return nil
		})
		if err != nil {
			return err
		}
	}

	verb := "sealed"
	if *check {
		verb = "would seal"
	}
	fmt.Printf("fix seal: %d sections read, %s %d of them and %d manifest rows, %d translations left stale\n",
		read, verb, sealed, rows, stale)
	return nil
}

// sealManifest writes the new hashes into manifests/sections.yaml and returns
// how many rows moved. A row whose hash already agrees is left as it is, and a
// path the manifest does not know is not added: the manifest is assembly's
// account of what it wrote, and a file assembly never wrote does not belong in
// it. Nothing is written when no row moved, so a corpus that is in order comes
// out of this with an unmodified manifest.
func sealManifest(root string, now map[string]string, check bool) (int, error) {
	if len(now) == 0 {
		return 0, nil
	}
	m, err := corpus.LoadSections(root)
	if err != nil {
		return 0, err
	}
	var rows int
	for i := range m.Books {
		for j := range m.Books[i].Chapters {
			for k := range m.Books[i].Chapters[j].Sections {
				r := &m.Books[i].Chapters[j].Sections[k]
				want, ok := now[filepath.ToSlash(r.Path)]
				if !ok || r.ContentSHA256 == want {
					continue
				}
				rows++
				r.ContentSHA256 = want
			}
		}
	}
	if rows == 0 || check {
		return rows, nil
	}
	return rows, m.Save(root)
}

// short is the head of a hash, which is all the report needs and all S08 prints.
func short(sha string) string {
	if len(sha) > 12 {
		return sha[:12]
	}
	if sha == "" {
		return "(nothing)"
	}
	return sha
}

// eachSection walks the section files of the corpus in path order.
//
// The exercises and the solutions are left out. They are files of another
// schema, they carry no content_sha256, and reading them here would only be a
// way of failing on front matter this command has no business parsing.
func eachSection(root, lang string, fn func(path string, f *corpus.File[corpus.SectionFrontMatter]) error) error {
	dir := filepath.Join(root, "content")
	var paths []string
	err := filepath.WalkDir(dir, func(path string, e iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			// content/solutions is a tree of its own schema, and every
			// exercises directory holds the exercises of one §.
			if e.Name() == "solutions" || e.Name() == "exercises" {
				return iofs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		rest, err := filepath.Rel(dir, path)
		if err != nil {
			return err
		}
		if lang != "" {
			l, _, _ := strings.Cut(filepath.ToSlash(rest), "/")
			if l != lang {
				return nil
			}
		}
		paths = append(paths, path)
		return nil
	})
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	sort.Strings(paths)
	for _, path := range paths {
		f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
		if err != nil {
			return err
		}
		if err := fn(path, &f); err != nil {
			return err
		}
	}
	return nil
}

func fixFootnote(args []string) error {
	fs := flag.NewFlagSet("fix footnote", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixFootnoteUsage) }
	book := fs.String("book", "", "only this volume")
	check := fs.Bool("check", false, "change nothing")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, books, err := corpusAndBooks()
	if err != nil {
		return err
	}

	var left int
	// took says how many marks a file gives up and prints the ones it will not.
	// A mark left alone is the interesting half of the report: it is a place
	// where the reading has to be looked at rather than repaired.
	took := func(path string, moves []footnote.Move) int {
		n := 0
		for _, m := range moves {
			if m.Kind == footnote.KindLeft {
				left++
				fmt.Fprintf(os.Stderr, "fix footnote: left alone, %s:%d prints %s and nothing there says which note it means\n",
					rel(root, path), m.Line, m.Mark)
				continue
			}
			n++
			if *check {
				fmt.Printf("%s  line %d  %s %s\n", rel(root, path), m.Line, m.Mark, m.Kind)
			}
		}
		return n
	}

	var pages, repaired int
	err = eachPage(root, books, *book, func(path string, f *corpus.PageFile) error {
		pages++
		body, moves := footnote.Normalize(f.Body)
		if took(path, moves) == 0 {
			return nil
		}
		repaired++
		if *check {
			return nil
		}
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		return err
	}

	// The English body each translation was made from, before and after, so
	// that a translation recording the first can be moved on to the second.
	// Keyed by the corpus-relative path the translation names.
	moved := map[string][2]string{}
	recordEnglish := func(path, before, after string) {
		moved[filepath.ToSlash(rel(root, path))] = [2]string{
			corpus.ContentSHA256(before), corpus.ContentSHA256(after)}
	}
	// follow moves a translation's record of its source on, and says whether it
	// did. It is deliberately narrow: only a translation that recorded the
	// English body as it stood before this repair, which is the only case where
	// the two are the same translation of the same words.
	follow := func(from, recorded string) (string, bool) {
		pair, ok := moved[from]
		if !ok || pair[0] != recorded || pair[0] == pair[1] {
			return recorded, false
		}
		return pair[1], true
	}

	// The English is walked first and on its own, because a translation cannot
	// be moved on until the body it was made from has been repaired and hashed
	// twice. english says which of the two passes is running.
	var english bool
	var files, content, followed int
	section := func(path string, f *corpus.File[corpus.SectionFrontMatter]) error {
		if (f.Meta.Lang == "en") != english {
			return nil
		}
		files++
		body, moves := footnote.Normalize(f.Body)
		n := took(path, moves)
		if f.Meta.Lang == "en" {
			recordEnglish(path, f.Body, body)
		} else if now, ok := follow(f.Meta.TranslatedFrom, f.Meta.SourceSHA256); ok {
			followed++
			if !*check {
				f.Meta.SourceSHA256 = now
			}
			n++
		}
		if n == 0 {
			return nil
		}
		content++
		if *check {
			return nil
		}
		f.Body = body
		return f.Write(path)
	}
	exercise := func(path string, f *corpus.File[corpus.ExerciseFrontMatter]) error {
		if (f.Meta.Lang == "en") != english {
			return nil
		}
		files++
		body, moves := footnote.Normalize(f.Body)
		n := took(path, moves)
		if f.Meta.Lang == "en" {
			recordEnglish(path, f.Body, body)
		} else if now, ok := follow(f.Meta.TranslatedFrom, f.Meta.SourceSHA256); ok {
			followed++
			if !*check {
				f.Meta.SourceSHA256 = now
			}
			n++
		}
		if n == 0 {
			return nil
		}
		content++
		if *check {
			return nil
		}
		f.Body = body
		return f.Write(path)
	}
	// The English first and on its own, because a translation cannot be moved
	// on until the body it was made from has been repaired and hashed twice.
	for _, english = range []bool{true, false} {
		if err := eachSection(root, "", section); err != nil {
			return err
		}
		if err := eachExercise(root, "", exercise); err != nil {
			return err
		}
	}

	verb, verbed := "took", "moved"
	if *check {
		verb, verbed = "would take", "would move"
	}
	fmt.Printf("fix footnote: %d pages read, %s the printed mark off %d of them, %d left alone\n",
		pages, verb, repaired, left)
	fmt.Printf("fix footnote: %d content files read, %d of them changed, %d translations %s on\n",
		files, content, followed, verbed)
	if repaired > 0 && !*check {
		fmt.Println("fix footnote: run bourbaki assemble")
	}
	return nil
}

// eachExercise walks the committed exercise files of one language, or of every
// language, in path order. It is eachSection for the tree eachSection skips.
func eachExercise(root, lang string, fn func(path string, f *corpus.File[corpus.ExerciseFrontMatter]) error) error {
	dir := filepath.Join(root, "content")
	var paths []string
	err := filepath.WalkDir(dir, func(path string, e iofs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() {
			if e.Name() == "solutions" {
				return iofs.SkipDir
			}
			return nil
		}
		if filepath.Ext(path) != ".md" {
			return nil
		}
		rest := filepath.ToSlash(mustRel(dir, path))
		if !strings.Contains(rest, "/exercises/") {
			return nil
		}
		if lang != "" {
			if l, _, _ := strings.Cut(rest, "/"); l != lang {
				return nil
			}
		}
		paths = append(paths, path)
		return nil
	})
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return err
	}
	sort.Strings(paths)
	for _, path := range paths {
		f, err := corpus.ReadFile[corpus.ExerciseFrontMatter](path)
		if err != nil {
			return err
		}
		if err := fn(path, &f); err != nil {
			return err
		}
	}
	return nil
}

// mustRel is filepath.Rel where the two paths are known to share a root,
// because the walk produced the second from the first.
func mustRel(base, path string) string {
	rest, err := filepath.Rel(base, path)
	if err != nil {
		return path
	}
	return rest
}
