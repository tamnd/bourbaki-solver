package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/extract"
	"github.com/tamnd/bourbaki-solver/pagemap"
	"github.com/tamnd/bourbaki-solver/pdfglyph"
	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// openPDF opens the volume the extractor should read.
//
// For the 2023 English printing that is the file itself. For the French
// printings it is a prepared copy with the TeX glyph names rewritten, because
// the file as it ships tells poppler that a character is called circleplustext
// and poppler, having never heard of it, prints nothing at all and hands back a
// run with a box, a font, a width and no text. See pdfglyph.
//
// The copy lives in work/ and is named after the digest of the volume it came
// from, so it is built once and rebuilt if the volume is ever replaced.
func openPDF(root string, b *corpus.Book) (*pdfsrc.Source, error) {
	path, _, err := pdfglyph.Prepare(root, b.ID, filepath.Join(root, b.PDF), b.PDFSHA256)
	if err != nil {
		return nil, err
	}
	return pdfsrc.Open(path)
}

func extractPrepare(args []string) error {
	fs := flag.NewFlagSet("extract prepare", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	verify := fs.Bool("verify", false, "read both copies and compare them run by run")
	first := fs.Int("f", 0, "first pdf page to verify")
	last := fs.Int("l", 0, "last pdf page to verify")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: bourbaki extract prepare -book <id> [-verify] [-f N] [-l N]\n\nRewrites the TeX glyph names of a printing to names poppler resolves.\n\n")
		fs.PrintDefaults()
	}
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" {
		fs.Usage()
		os.Exit(2)
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return err
	}
	b, ok := books.Get(*book)
	if !ok {
		return fmt.Errorf("no book %q in %s", *book, corpus.BooksPath(root))
	}
	orig := filepath.Join(root, b.PDF)
	path, res, err := pdfglyph.Prepare(root, b.ID, orig, b.PDFSHA256)
	if err != nil {
		return err
	}

	fmt.Printf("%s  %d names rewritten in %d encodings\n", b.ID, res.Total(), res.Encodings)
	if res.Total() == 0 {
		fmt.Printf("  nothing to do, the printing names no glyph poppler cannot read\n")
		return nil
	}
	fmt.Printf("  %s\n", path)
	for _, n := range res.Sorted() {
		fmt.Printf("  %4d  %s\n", res.Names[n], n)
	}
	if len(res.Unknown) > 0 {
		fmt.Printf("\n%d TeX names have no replacement. A name is in a font's encoding\n", len(res.Unknown))
		fmt.Printf("whether or not any page draws it, so run this with -verify: a name\n")
		fmt.Printf("that is drawn is a symbol the corpus will not carry.\n")
		unknown := make([]string, 0, len(res.Unknown))
		for k := range res.Unknown {
			unknown = append(unknown, k)
		}
		sort.Strings(unknown)
		for _, k := range unknown {
			fmt.Printf("  %4d  %s\n", res.Unknown[k], k)
		}
	}
	if !*verify {
		return nil
	}
	return verifyPrepared(orig, path, *first, *last)
}

// verifyPrepared reads both copies of a volume and compares what they say.
//
// The claim the rewrite makes is narrow, and it is worth checking against the
// real file rather than against a fixture: every character the volume already
// said is still there, and the only differences are characters that were not
// there before and characters that were there and were wrong. A character that
// went missing is the rewrite having touched a byte it had no business
// touching, and it fails here rather than in a page file three commands later.
//
// The comparison is by page and not by run. Poppler splits a line into runs by
// font and by where the boxes fall, so a run that was empty and now carries a
// character can be merged into its neighbour and shift every box on the line
// along. Pairing runs by position calls that a hundred runs lost and a hundred
// gained on a page where nothing at all went wrong. What does not move is the
// reading order of the page, so the two readings are aligned as text.
func verifyPrepared(orig, prepared string, first, last int) error {
	ctx := context.Background()
	a, err := readLayout(ctx, orig, first, last)
	if err != nil {
		return err
	}
	b, err := readLayout(ctx, prepared, first, last)
	if err != nil {
		return err
	}
	if len(a.Pages) != len(b.Pages) {
		return fmt.Errorf("the prepared copy has %d pages, the volume has %d", len(b.Pages), len(a.Pages))
	}

	total := pdfglyph.Diff{Added: map[rune]int{}, Changed: map[pdfglyph.Change]int{}, Lost: map[rune]int{}}
	var touched, hard, lost []int
	for i, pa := range a.Pages {
		d := pdfglyph.Compare(pageRunes(pa), pageRunes(b.Pages[i]))
		total.Add(d)
		if pdfglyph.Total(d.Added)+pdfglyph.Total(d.Changed) > 0 {
			touched = append(touched, pa.Number)
		}
		if d.Hard {
			hard = append(hard, pa.Number)
		}
		if len(d.Lost) > 0 {
			lost = append(lost, pa.Number)
		}
	}

	fmt.Printf("\nverified %d pages\n", len(a.Pages))
	fmt.Printf("  %6d characters read exactly as they did before\n", total.Kept)
	fmt.Printf("  %6d characters are new, and %d read as something else before,\n",
		pdfglyph.Total(total.Added), pdfglyph.Total(total.Changed))
	fmt.Printf("         on %d of the %d pages\n", len(touched), len(a.Pages))
	if len(total.Added) > 0 {
		fmt.Printf("\n  recovered, most frequent first:\n")
		for _, r := range pdfglyph.ByCount(total.Added, func(x, y rune) bool { return x < y }) {
			fmt.Printf("    %6d  %-4q %s\n", total.Added[r], string(r), codes(string(r)))
		}
	}
	if len(total.Changed) > 0 {
		fmt.Printf("\n  corrected, where poppler had fallen back on a meaningless code:\n")
		less := func(x, y pdfglyph.Change) bool {
			if x.Old != y.Old {
				return x.Old < y.Old
			}
			return x.New < y.New
		}
		for _, c := range pdfglyph.ByCount(total.Changed, less) {
			fmt.Printf("    %6d  %-4q became %-4q %s\n",
				total.Changed[c], string(c.Old), string(c.New), codes(string(c.New)))
		}
	}
	if len(total.Lost) > 0 || len(hard) > 0 {
		for _, r := range pdfglyph.ByCount(total.Lost, func(x, y rune) bool { return x < y }) {
			fmt.Printf("  LOST %6d  %q %s\n", total.Lost[r], string(r), codes(string(r)))
		}
		if len(lost) > 0 {
			fmt.Printf("  LOST on pages %v\n", head(lost, 20))
		}
		if len(hard) > 0 {
			fmt.Printf("  UNREADABLE pages %v\n", head(hard, 20))
		}
		return fmt.Errorf("the prepared copy does not read the same as the volume")
	}
	return nil
}

// head is the front of a list of pages, since a rewrite that went wrong went
// wrong on more pages than anybody wants printed.
func head(pages []int, n int) []int {
	if len(pages) <= n {
		return pages
	}
	return pages[:n]
}

// pageRunes is everything a page says, in reading order, with the spacing left
// out. Poppler moves a space when it merges two runs into one, and a space that
// moved is not a character the volume lost.
func pageRunes(p pdfsrc.Page) []rune {
	var out []rune
	for _, s := range p.Spans {
		for _, r := range s.Text {
			if r == ' ' || r == '\t' || r == '\n' {
				continue
			}
			out = append(out, r)
		}
	}
	return out
}

func readLayout(ctx context.Context, path string, first, last int) (*pdfsrc.Layout, error) {
	src, err := pdfsrc.Open(path)
	if err != nil {
		return nil, err
	}
	return src.XML(ctx, first, last)
}

// volumeText is the text layer of a volume, one string per page, as the head
// parsers and the contents reader want it.
//
// It goes through the prepared copy for the same reason extraction does: the
// contents of the French printing names a subsection "Polynômes à coefficients
// dans un anneau noethérien", and read off the file as it ships that title
// arrives with the glyphs poppler could not name missing from it. The ligatures
// are written out here too, since a title is what a tag is named after and what
// a reader searches for, and nothing anybody searches for spells it coeﬃcients.
func volumeText(ctx context.Context, root string, b *corpus.Book) ([]string, error) {
	src, err := openPDF(root, b)
	if err != nil {
		return nil, err
	}
	text, err := src.Text(ctx, 0, 0, true)
	if err != nil {
		return nil, err
	}
	pages := pagemap.SplitPages(text)
	for i, p := range pages {
		pages[i] = extract.Unligature(p)
	}
	return pages, nil
}
