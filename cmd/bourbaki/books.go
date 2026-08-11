package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

func runBooks(args []string) error {
	if len(args) == 0 {
		return fmt.Errorf("books: want one of add, list, verify")
	}
	switch args[0] {
	case "add":
		return booksAdd(args[1:])
	case "list":
		return booksList(args[1:])
	case "verify":
		return booksVerify(args[1:])
	default:
		return fmt.Errorf("books: unknown subcommand %q", args[0])
	}
}

func booksAdd(args []string) error {
	fs := flag.NewFlagSet("books add", flag.ExitOnError)
	id := fs.String("id", "", "book id, for example alg-viii")
	title := fs.String("title", "", "title, defaults to the file name")
	edition := fs.String("edition", "", "edition, for example \"2023, Springer Nature\"")
	chapters := fs.String("chapters", "", "comma-separated Roman chapters, for example I,II,III")
	book := fs.String("book", "alg", "book slug")
	lang := fs.String("lang", "en", "language of this printing, en or fr")
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, "usage: bourbaki books add <pdf> --id <id> [flags]\n\nProbes the PDF and records it in manifests/books.yaml.\n\n")
		fs.PrintDefaults()
	}
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || *id == "" {
		fs.Usage()
		os.Exit(2)
	}

	root, err := corpus.Root()
	if err != nil {
		return err
	}
	path := pos[0]
	rel := path
	if abs, err := filepath.Abs(path); err == nil {
		if r, err := filepath.Rel(root, abs); err == nil && !strings.HasPrefix(r, "..") {
			rel = filepath.ToSlash(r)
		}
	}

	src, err := pdfsrc.Open(path)
	if err != nil {
		return err
	}
	ctx := context.Background()

	info, err := src.Info(ctx)
	if err != nil {
		return err
	}
	sum, err := src.SHA256()
	if err != nil {
		return err
	}
	size, err := src.Size()
	if err != nil {
		return err
	}
	cls, err := src.Classify(ctx, 10)
	if err != nil {
		return err
	}
	sample, err := src.SampleText(ctx, 5)
	if err != nil {
		return err
	}

	b := corpus.Book{
		ID:         *id,
		Book:       *book,
		Lang:       *lang,
		Title:      *title,
		Edition:    *edition,
		PDF:        rel,
		PDFSHA256:  sum,
		PDFBytes:   size,
		Pages:      info.Pages,
		Nature:     string(cls.Nature),
		Producer:   info.Producer,
		PageWidth:  info.WidthPt,
		PageHeight: info.HeightPt,
		TextLayer:  textLayer(cls.Nature, sample),
	}
	if b.Title == "" {
		b.Title = strings.TrimSuffix(filepath.Base(path), ".pdf")
	}
	if *chapters != "" {
		for _, c := range strings.Split(*chapters, ",") {
			c = strings.ToUpper(strings.TrimSpace(c))
			if c == "" {
				continue
			}
			if _, err := corpus.RomanOrder(c); err != nil {
				return fmt.Errorf("chapter %q is not a Roman numeral", c)
			}
			b.Chapters = append(b.Chapters, c)
		}
	}
	if cls.Nature == pdfsrc.NatureScanned {
		b.Extraction = "ocr"
		img, ok := cls.BodyImage()
		if !ok {
			// Algèbre chapter 10 draws every page as two dozen strips, so no
			// single image describes it and the volume used to come out with no
			// scan block at all. render then has nothing to hold its resolution
			// down to and rasterises a 260 dpi scan at 300.
			img, ok = cls.TiledImage()
		}
		if ok {
			b.Scan = &corpus.Scan{
				Format: img.Enc, Width: img.Width, Height: img.Height,
				DPI: img.XPPI, BPC: img.BPC, Color: img.Color,
			}
		}
	} else {
		b.Extraction = "native"
	}

	m, err := corpus.LoadBooks(root)
	if err != nil {
		return err
	}
	if old, ok := m.Get(b.ID); ok {
		b = keepLearned(b, *old)
	}
	m.Upsert(b)
	if err := m.Save(root); err != nil {
		return err
	}

	fmt.Printf("%s  %s  %d pages  %s  %s\n", b.ID, b.Lang, b.Pages, b.Nature, b.PDFSHA256[:16])
	if b.Scan != nil {
		fmt.Printf("  scan %s %dx%d at %d dpi, %d bpc %s\n",
			b.Scan.Format, b.Scan.Width, b.Scan.Height, b.Scan.DPI, b.Scan.BPC, b.Scan.Color)
	}
	if b.Producer != "" {
		fmt.Printf("  producer %s\n", b.Producer)
	}
	fmt.Printf("  %d of %d sampled pages carry a full-page image, %d of %d fonts embedded\n",
		cls.PagesWithFull, cls.SampledPages, cls.FontsEmbedded, cls.Fonts)
	fmt.Printf("  text layer %s, %d characters a page over pages %v\n",
		b.TextLayer, sample.PerPage(), sample.At)
	fmt.Printf("  written to %s\n", corpus.BooksPath(root))
	return nil
}

// keepLearned carries into a freshly probed entry the fields that no probe can
// know. The grammar and the pagination are read off the running heads by
// pagemap build, and adding a volume a second time to correct its chapter list
// or to give it a language is a normal thing to do, so re-probing must not
// silently undo a page map.
func keepLearned(fresh, old corpus.Book) corpus.Book {
	fresh.Grammar, fresh.Pagination = old.Grammar, old.Pagination
	return fresh
}

// textLayer names what a volume's own text is worth, which decides how much
// work it is before anything can be read off it at all.
//
// A born-digital volume has real text. A scan either carries somebody else's
// OCR, legible enough to read a running head off and useless for mathematics,
// or carries nothing, and then even the page map has to come out of vision OCR.
// The threshold is characters on a sampled body page: a full page of this
// series runs to about two thousand, and a page with nothing but a stray label
// on it to a few dozen.
func textLayer(n pdfsrc.Nature, t pdfsrc.TextSample) string {
	if n == pdfsrc.NatureBornDigital {
		return "native"
	}
	if t.PerPage() >= 200 {
		return "ocr"
	}
	return "none"
}

func booksList(args []string) error {
	fs := flag.NewFlagSet("books list", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	m, err := corpus.LoadBooks(root)
	if err != nil {
		return err
	}
	if len(m.Books) == 0 {
		return fmt.Errorf("no books registered in %s", corpus.BooksPath(root))
	}
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "ID\tLANG\tCHAPTERS\tPAGES\tNATURE\tTEXT\tEDITION")
	for _, b := range m.Books {
		fmt.Fprintf(w, "%s\t%s\t%s\t%d\t%s\t%s\t%s\n",
			b.ID, b.Lang, strings.Join(b.Chapters, ","), b.Pages, b.Nature, b.TextLayer, b.Edition)
	}
	fmt.Fprintf(w, "\t\t\t%d\t\t\t%d volumes\n", m.Pages(), len(m.Books))
	return w.Flush()
}

func booksVerify(args []string) error {
	fs := flag.NewFlagSet("books verify", flag.ExitOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	m, err := corpus.LoadBooks(root)
	if err != nil {
		return err
	}
	ctx := context.Background()
	bad := 0
	for _, b := range m.Books {
		path := filepath.Join(root, b.PDF)
		src, err := pdfsrc.Open(path)
		if err != nil {
			fmt.Printf("%-12s MISSING  %s\n", b.ID, b.PDF)
			bad++
			continue
		}
		sum, err := src.SHA256()
		if err != nil {
			return err
		}
		if sum != b.PDFSHA256 {
			fmt.Printf("%-12s SHA MISMATCH\n  manifest %s\n  on disk  %s\n", b.ID, b.PDFSHA256, sum)
			bad++
			continue
		}
		info, err := src.Info(ctx)
		if err != nil {
			return err
		}
		if info.Pages != b.Pages {
			fmt.Printf("%-12s PAGE COUNT  manifest %d, on disk %d\n", b.ID, b.Pages, info.Pages)
			bad++
			continue
		}
		fmt.Printf("%-12s ok  %d pages  %s\n", b.ID, b.Pages, sum[:16])
	}
	if bad > 0 {
		return fmt.Errorf("%d of %d volumes failed verification", bad, len(m.Books))
	}
	return nil
}
