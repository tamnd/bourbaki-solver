package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/share"
)

const shareAuditUsage = `usage: bourbaki share audit [flags]

Holds every file of an import against the printed volume.

  -corpus DIR    the checkout, default $BOURBAKI_CORPUS
  -book ID       the volume in manifests/books.yaml, for example ens-i-iv
  -import NAME   the import tree under imports/, for example sets
  -v             print the soft findings as well as the hard ones

Three questions, and only three. Does the import carry every no. the printed
contents lists for that §, in the order the book runs them. Does it carry every
label the pages print, which in Theory of Sets means the criteria C, CF, CS, CST
and S as well as the propositions, corollaries, definitions, theorems and
lemmas. And is every page of the § somewhere in it, looked for by runs of eight
words of prose with the mathematics taken out, since two readings of one page
agree on the words and argue about the markup.

It does not judge which reading is better. Nothing here can, and an import that
passes this is still an import: it says the transcription is of the whole
section and not of most of it, which is the thing that cannot be seen by reading
the file on its own.

The volume and the import tree are both named because nothing can map one to the
other. imports/ is named the way the person who had the conversation named it and
manifests/books.yaml is named by volume and chapters.
`

// importFrontMatter is the head of an import file.
//
// The whole head and not only the three fields this command reads, because the
// corpus reader refuses a field it has not been told about, and it is right to:
// a head that has grown a field nobody reads is how a provenance line goes
// quietly missing. See share.Import.Markdown for what writes these.
type importFrontMatter struct {
	Book          string   `yaml:"book"`
	Chapter       int      `yaml:"chapter"`
	Section       int      `yaml:"section"`
	Intro         bool     `yaml:"intro"`
	Lang          string   `yaml:"lang"`
	Extraction    string   `yaml:"extraction"`
	ShareURL      string   `yaml:"share_url"`
	ShareTitle    string   `yaml:"share_title"`
	Asks          int      `yaml:"asks"`
	Answers       int      `yaml:"answers"`
	Models        []string `yaml:"models"`
	Joined        string   `yaml:"joined"`
	AnswersSHA256 string   `yaml:"answers_sha256"`
	ContentSHA256 string   `yaml:"content_sha256"`
}

func shareAudit(args []string) error {
	fs := flag.NewFlagSet("share audit", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, shareAuditUsage) }
	dir := fs.String("corpus", "", "the checkout")
	book := fs.String("book", "", "the volume the import is of")
	name := fs.String("import", "", "the import tree under imports/")
	verbose := fs.Bool("v", false, "print the soft findings too")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *book == "" || *name == "" {
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

	files, err := importFiles(filepath.Join(root, share.Dir, *name))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no import files under %s", filepath.Join(share.Dir, *name))
	}

	audited, failed, skipped := 0, 0, 0
	for _, path := range files {
		f, err := corpus.ReadFile[importFrontMatter](path)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(root, path)
		if f.Meta.Intro || f.Meta.Section == 0 {
			// The introduction is prose with no numbering in it, so there is
			// nothing here to hold it against. Said out loud rather than passed
			// over, because a silent skip reads as a pass.
			fmt.Printf("%-34s not audited: the introduction has no numbering and no § of its own\n", rel)
			skipped++
			continue
		}
		p, unread, err := printedSection(root, *book, bt, f.Meta.Chapter, f.Meta.Section)
		if err != nil {
			return fmt.Errorf("%s: %w", rel, err)
		}
		r := share.Audit(share.Target{Book: f.Meta.Book, Chapter: f.Meta.Chapter, Section: f.Meta.Section},
			f.Body, p)
		audited++
		if !r.OK() {
			failed++
		}
		printAudit(rel, r, unread, *verbose)
	}
	fmt.Printf("share audit: %d sections audited, %d with something missing, %d not audited\n",
		audited, failed, skipped)
	if failed > 0 {
		return fmt.Errorf("%d of %d sections are missing something the book prints", failed, audited)
	}
	return nil
}

func printAudit(rel string, r *share.Result, unread int, verbose bool) {
	fmt.Printf("%-34s %d no., %d labels, %d pages", rel, r.Numbers, r.Labels, r.Pages)
	if r.Thin > 0 {
		fmt.Printf(", %d thin", r.Thin)
	}
	if unread > 0 {
		fmt.Printf(", %d pages not read yet and not checked", unread)
	}
	if r.OK() {
		fmt.Printf("  ok\n")
	} else {
		fmt.Printf("  %d missing\n", r.Hard())
	}
	for _, f := range r.Findings {
		if !f.Hard && !verbose {
			continue
		}
		mark := "-"
		if f.Hard {
			mark = "!"
		}
		fmt.Printf("    %s %-9s %s\n", mark, f.Rule, f.Text)
	}
}

// printedSection gathers what the book itself says about one §, and the pages
// of it that are read.
//
// The page range is taken from the contents and not from the assembled section,
// because the assembly is downstream of a reading of the same pages and would
// hide a page the assembler dropped. The end of a § is the next thing the
// contents points at, whatever kind of thing that is: the next §, the appendix,
// the chapter's historical note, the run of exercises, or the next chapter. The
// exercises matter here in particular, since chapters I to III gather them at
// the back of the volume, and without them the last § of a chapter would run on
// through the exercises of all of them.
func printedSection(root, book string, bt *corpus.BookTOC, chapter, section int) (share.Printed, int, error) {
	var want *corpus.Section
	var bounds []int
	found := false
	for _, c := range bt.Chapters {
		n, err := corpus.RomanOrder(c.Numeral)
		if err != nil {
			return share.Printed{}, 0, err
		}
		if n != chapter {
			bounds = append(bounds, c.PDFPage)
			continue
		}
		found = true
		for i := range c.Sections {
			s := &c.Sections[i]
			bounds = append(bounds, s.PDFPage)
			if s.Exercises != nil {
				bounds = append(bounds, s.Exercises.PDFPage)
			}
			if !s.Appendix && s.Number == section {
				want = s
			}
		}
		if c.Historical != nil {
			bounds = append(bounds, c.Historical.PDFPage)
		}
		if c.Exercises != nil {
			bounds = append(bounds, c.Exercises.PDFPage)
		}
	}
	if !found {
		return share.Printed{}, 0, fmt.Errorf("%s has no chapter %d in its contents", book, chapter)
	}
	if want == nil {
		return share.Printed{}, 0, fmt.Errorf("%s chapter %d has no § %d in its contents", book, chapter, section)
	}

	last := 0
	for _, b := range bounds {
		if b > want.PDFPage && (last == 0 || b < last) {
			last = b
		}
	}
	if last == 0 {
		return share.Printed{}, 0, fmt.Errorf("%s chapter %d § %d is the last thing the contents points at, so its last page is unknown",
			book, chapter, section)
	}

	p := share.Printed{Section: want.Number, Title: want.Title}
	for _, sub := range want.Subsections {
		p.Numbers = append(p.Numbers, share.Numbered{No: sub.Number, Title: sub.Title})
	}
	unread := 0
	for pg := want.PDFPage; pg < last; pg++ {
		f, err := corpus.ReadFile[corpus.PageFrontMatter](corpus.PagePath(root, book, pg))
		if os.IsNotExist(err) {
			unread++
			continue
		}
		if err != nil {
			return share.Printed{}, 0, err
		}
		p.Pages = append(p.Pages, share.PrintedPage{PDFPage: pg, Text: f.Body})
	}
	// The page the next § starts on, where it has been read. A § ends partway
	// down it and the rest of that page is somebody else's, which is why it
	// goes in a field of its own rather than in Pages: share read wants it and
	// share audit must not have it.
	if f, err := corpus.ReadFile[corpus.PageFrontMatter](corpus.PagePath(root, book, last)); err == nil {
		p.After = append(p.After, share.PrintedPage{PDFPage: last, Text: f.Body})
	} else if !os.IsNotExist(err) {
		return share.Printed{}, 0, err
	}
	return p, unread, nil
}

// importFiles is every Markdown file of an import tree, in reading order.
func importFiles(dir string) ([]string, error) {
	var out []string
	err := filepath.Walk(dir, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() || !strings.HasSuffix(path, ".md") {
			return nil
		}
		out = append(out, path)
		return nil
	})
	if os.IsNotExist(err) {
		return nil, fmt.Errorf("%s is not an import tree", dir)
	}
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}
