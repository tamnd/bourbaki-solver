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
	"github.com/tamnd/bourbaki-solver/render"
)

const renderUsage = `usage: bourbaki render -book <id> [flags]

Rasterise a scanned volume to the page images vision OCR reads.

flags:
  -book ID       book id from manifests/books.yaml
  -dpi N         resolution, default 300, use 600 on a retry
  -gray          render gray rather than colour (default true)
  -f N -l N      first and last pdf page, default the whole volume
  -batch N       pages per pdftoppm call, default 25
  -overwrite     re-render pages whose image is already on disk
  -blanks        write a method: blank page file for every blank page (default true)
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
	if entry.Extraction != "ocr" {
		return fmt.Errorf("%s is %s and extracts by %s: use bourbaki extract", entry.ID, entry.Nature, entry.Extraction)
	}

	options := render.Options{
		Book: entry.ID, PDF: filepath.Join(root, entry.PDF), Corpus: root,
		DPI: *dpi, Gray: *gray, First: *first, Last: *last, Batch: *batch,
		Overwrite: *overwrite, WriteBlanks: *blanks,
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
