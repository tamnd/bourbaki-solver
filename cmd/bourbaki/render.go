package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/extract"
	"github.com/tamnd/bourbaki-solver/render"
)

const renderUsage = `usage: bourbaki render -book <id> [flags]

Rasterise a scanned volume to the page images vision OCR reads.

flags:
  -book ID       book id from manifests/books.yaml
  -dpi N         resolution, default 300, use 600 on a retry
  -gray          render gray rather than colour (default true)
  -f N -l N      first and last pdf page, default the whole volume
  -flagged       only the pages the native extraction could not read, from
                 reports/extract-<book>.json, and the one way to render a
                 born-digital volume
  -batch N       pages per pdftoppm call, default 25
  -overwrite     re-render pages whose image is already on disk
  -blanks        write a method: blank page file for every blank page (default
                 true, and never under -flagged, where the page already has a
                 reading with its folio on it)
  -manifest      print the manifest that is already on disk and exit
  -json          print the manifest as JSON

The images are scratch. They live under images/<book>/ in the corpus checkout,
which is not committed: a page is a couple of hundred kilobytes, there are 1194
of them, and the Markdown is what replaces them.
`

func runRender(args []string) error {
	fs := flag.NewFlagSet("render", flag.ExitOnError)
	book := fs.String("book", "", "book id")
	dpi := fs.Int("dpi", render.DefaultDPI, "resolution")
	gray := fs.Bool("gray", true, "render gray")
	first := fs.Int("f", 0, "first pdf page")
	last := fs.Int("l", 0, "last pdf page")
	flagged := fs.Bool("flagged", false, "only the pages the extraction flagged")
	batch := fs.Int("batch", 25, "pages per pdftoppm call")
	overwrite := fs.Bool("overwrite", false, "re-render pages already on disk")
	blanks := fs.Bool("blanks", true, "write a page file for every blank page")
	show := fs.Bool("manifest", false, "print the manifest on disk and exit")
	asJSON := fs.Bool("json", false, "print the manifest as JSON")
	quiet := fs.Bool("quiet", false, "do not print progress")
	fs.Usage = func() { fmt.Fprint(os.Stderr, renderUsage) }
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
	entry, ok := books.Get(*book)
	if !ok {
		return fmt.Errorf("no book %q in %s", *book, corpus.BooksPath(root))
	}

	if *show {
		manifest, err := render.ReadManifest(root, entry.ID)
		if err != nil {
			return err
		}
		return printManifest(manifest, *asJSON)
	}

	// A born-digital volume has a text layer and goes through extract, not
	// here. Rendering one would work and would cost 151 seconds a page to read
	// back something poppler already has, so say no rather than let it run.
	//
	// The exception is the pages poppler could not read. Fifty six pages of
	// Algebra VIII carry a commutative diagram, a glyph the text layer dropped
	// or a delimiter it lost, and the only way to read those is to look at
	// them. That is what -flagged asks for and it is the whole door: no page
	// gets rendered out of a born-digital volume without the extraction having
	// said, in a report on disk, that it could not read it.
	var only []int
	if *flagged {
		pages, err := flaggedPages(root, entry.ID)
		if err != nil {
			return err
		}
		if len(pages) == 0 {
			fmt.Printf("%s: the extraction flagged nothing, nothing to render\n", entry.ID)
			return nil
		}
		only = pages
	}
	if entry.Extraction != "ocr" && !*flagged {
		return fmt.Errorf("%s is %s and extracts by %s: use bourbaki extract, or -flagged for the pages it could not read",
			entry.ID, entry.Nature, entry.Extraction)
	}

	options := render.Options{
		Book: entry.ID, PDF: filepath.Join(root, entry.PDF), Corpus: root,
		DPI: *dpi, SourceDPI: sourceDPI(entry), Gray: *gray,
		First: *first, Last: *last, Only: only, Batch: *batch,
		Overwrite: *overwrite, WriteBlanks: blankFiles(*blanks, *flagged),
	}
	if !*quiet {
		// 734 pages at a third of a second is four minutes of silence
		// otherwise, and the first thing anyone does with silence is kill it.
		start := time.Now()
		options.Logf = func(format string, args ...any) {
			fmt.Fprintf(os.Stderr, "["+time.Since(start).Round(time.Second).String()+"] "+format+"\n", args...)
		}
	}

	manifest, err := render.Render(context.Background(), options)
	if err != nil {
		return err
	}
	return printManifest(manifest, *asJSON)
}

// blankFiles says whether this render writes a page file for its blank pages.
//
// Writing one is for a volume that has no reading yet, where the file is how a
// page stays out of the OCR queue. Under -flagged every page already has a
// reading, out of the text layer, with the folio read off it and the flag that
// put it on this list, and a blank page file carries neither. Eighteen pages
// lost their page label to this before it was noticed: the extraction had
// already called them blank and had a label, and the render overwrote the file
// with a blank one that did not.
//
// The pages are still rendered and still go to the model. What the ink
// threshold says about a page of a born-digital volume is worth having and is
// not worth acting on: an empty page there is a page with no text layer, which
// is what a full page diagram looks like from here.
func blankFiles(blanks, flagged bool) bool { return blanks && !flagged }

// flaggedPages is the work list the extraction left behind: every page it could
// not read, in page order, empty ones included. An empty page in a born-digital
// volume is not a blank page, it is a page with no text layer at all, which is
// what a full page diagram looks like from here.
func flaggedPages(root, book string) ([]int, error) {
	path := corpus.ExtractReportPath(root, book)
	raw, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("%s has no extraction report, run bourbaki extract first", book)
		}
		return nil, err
	}
	var result extract.Result
	if err := json.Unmarshal(raw, &result); err != nil {
		return nil, fmt.Errorf("%s: %w", path, err)
	}
	pages := make([]int, 0, len(result.Flags))
	for _, page := range result.Flags {
		pages = append(pages, page.PDFPage)
	}
	sort.Ints(pages)
	return pages, nil
}

func printManifest(manifest render.Manifest, asJSON bool) error {
	if asJSON {
		raw, err := json.MarshalIndent(manifest, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(raw))
		return nil
	}
	fmt.Print(manifest.Summary())

	// The blank pages by number, because the first thing to check on a new
	// volume is whether the threshold called anything blank that is not.
	var blank []int
	for _, page := range manifest.Pages {
		if page.Blank {
			blank = append(blank, page.Page)
		}
	}
	sort.Ints(blank)
	if len(blank) > 0 {
		fmt.Printf("blank pages: %v\n", blank)
	}
	return nil
}

// sourceDPI is the resolution of the images inside a volume, or zero when the
// probe never got a figure out of it. Rendering above it is interpolation, and
// interpolation is bytes over rsync for nothing.
func sourceDPI(entry *corpus.Book) int {
	if entry.Scan == nil {
		return 0
	}
	return entry.Scan.DPI
}
