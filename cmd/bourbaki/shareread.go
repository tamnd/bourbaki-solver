package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/share"
)

const shareReadUsage = `usage: bourbaki share read [flags]

Writes the sheet a person uses to hold one imported section against the pages.

  -corpus DIR    the checkout, default $BOURBAKI_CORPUS
  -book ID       the volume in manifests/books.yaml, for example ens-i-iv
  -import NAME   the import tree under imports/, for example sets
  -file PATH     the one import file to read, relative to the checkout
  -out PATH      where to write the sheet, relative to the checkout
  -n             print the sheet instead of writing it

share audit asks three questions and all three run from the book to the import.
That catches a transcription that stopped early or skipped a page. It cannot see
the other direction: a sentence in the import that is on no page of the book
passes every one of the three, and a transcription made by a model reading a
photograph is likelier to invent a line it could not make out than to leave a
gap where the line was.

So this is not a fourth rule and it fails nothing. It puts the sentences of each
page that the import does not carry, and the sentences of the import that no
page carries, in front of a person in the order they would read them. The two
sides are two transcriptions of one piece of type and they argue about markup
without either being wrong about the book, which is why a threshold cannot
settle this and a reader can.
`

func shareRead(args []string) error {
	fs := flag.NewFlagSet("share read", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, shareReadUsage) }
	dir := fs.String("corpus", "", "the checkout")
	book := fs.String("book", "", "the volume the import is of")
	name := fs.String("import", "", "the import tree under imports/")
	file := fs.String("file", "", "the one import file to read")
	out := fs.String("out", "", "where to write the sheet")
	dry := fs.Bool("n", false, "print the sheet instead of writing it")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" || (*name == "" && *file == "") {
		fs.Usage()
		os.Exit(2)
	}
	root, err := corpusRoot(*dir)
	if err != nil {
		return err
	}
	man, err := corpus.LoadTOC(root)
	if err != nil {
		return err
	}
	bt, ok := man.Get(*book)
	if !ok {
		return fmt.Errorf("%s has no contents in %s, so there is nothing to hold the import against (run bourbaki toc build first)",
			*book, corpus.TOCPath(root, *book))
	}

	path := *file
	if path == "" {
		// One file at a time by design. A sheet is read by a person and a sheet
		// covering a whole import is a sheet nobody finishes, which is how a
		// check like this becomes a file in the repository that says it was
		// done.
		files, err := importFiles(filepath.Join(root, share.Dir, *name))
		if err != nil {
			return err
		}
		if len(files) == 0 {
			return fmt.Errorf("no import files under %s", filepath.Join(share.Dir, *name))
		}
		return fmt.Errorf("name the file to read with -file. The import holds %d of them: %s",
			len(files), shortList(root, files))
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(root, path)
	}
	f, err := corpus.ReadFile[importFrontMatter](path)
	if err != nil {
		return err
	}
	rel, _ := filepath.Rel(root, path)
	if f.Meta.Intro || f.Meta.Section == 0 {
		return fmt.Errorf("%s is the introduction, which has no § of its own and no pages to hold it against", rel)
	}
	p, unread, err := printedSection(root, *book, bt, f.Meta.Chapter, f.Meta.Section)
	if err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}
	if len(p.Pages) == 0 {
		return fmt.Errorf("%s: none of the pages of chapter %d § %d has been read, so there is nothing to hold it against",
			rel, f.Meta.Chapter, f.Meta.Section)
	}
	sheet := share.Read(share.Target{Book: f.Meta.Book, Chapter: f.Meta.Chapter, Section: f.Meta.Section}, f.Body, p)
	sheet.SortPages()
	text := sheet.Markdown(*book)
	if unread > 0 {
		text += fmt.Sprintf("\n%d %s of this section have not been read yet and are not on this sheet.\n",
			unread, plural(unread, "page", "pages"))
	}
	if *dry {
		fmt.Print(text)
		return nil
	}
	dest := *out
	if dest == "" {
		dest = filepath.Join("reports", fmt.Sprintf("share-read-%s-%d.%d.md",
			f.Meta.Book, f.Meta.Chapter, f.Meta.Section))
	}
	if !filepath.IsAbs(dest) {
		dest = filepath.Join(root, dest)
	}
	if err := os.MkdirAll(filepath.Dir(dest), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(dest, []byte(text), 0o644); err != nil {
		return err
	}
	short, _ := filepath.Rel(root, dest)
	fmt.Printf("%s: %s\n", rel, sheet.Summary())
	fmt.Printf("written to %s\n", short)
	return nil
}

// shortList names the files of an import in an error, so that somebody who left
// -file off is told what to put there rather than told to read the usage again.
func shortList(root string, files []string) string {
	out := ""
	for i, f := range files {
		rel, _ := filepath.Rel(root, f)
		if i > 0 {
			out += ", "
		}
		out += rel
	}
	return out
}
