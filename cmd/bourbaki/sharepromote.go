package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/share"
)

const sharePromoteUsage = `usage: bourbaki share promote [flags]

Moves an import into content/, where the audit can reach it, or says why not.

  -corpus DIR    the checkout, default $BOURBAKI_CORPUS
  -book ID       the volume in manifests/books.yaml, for example ens-i-iv
  -import NAME   the import tree under imports/, for example sets
  -n             say what would happen and write nothing
  -v             print the whole reason for every section, not the short one

Four things have to hold before a section moves. It has to be a § and not the
book's introduction. share audit has to pass over it. A person has to have read
it against the printed volume and recorded that in manifests/imports.yaml. And
content/ must not already hold a reading of that § made from the pages.

The last one refuses most of what is here today and is the point of the
command. Theory of Sets was read off the rendered pages long before anyone had
share links for it, so content/en/ens already holds every § of chapters I to
IV. Those files carry pdf_pages and can be taken back to the page files and to
the PDF. An import carries a link to a conversation. Promoting over the first
with the second would swap a reading that has provenance for one that has none,
and it would look like nothing had happened, because both are Markdown. So an
import is promoted into a gap and never over a page. Where the two overlap the
import is a second opinion, which is worth having and is not a promotion.

Every import file comes out of this either promoted or listed with the reason
it was not. Nothing is passed over in silence, because a section that appears
in neither list is how an import ends up cited as the book.

Promoted sections are tagged and audited before the command returns, so a
promotion that breaks a rule is a failure here rather than a surprise in CI.
`

func sharePromote(args []string) error {
	fs := flag.NewFlagSet("share promote", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, sharePromoteUsage) }
	dir := fs.String("corpus", "", "the checkout")
	book := fs.String("book", "", "the volume the import is of")
	name := fs.String("import", "", "the import tree under imports/")
	dry := fs.Bool("n", false, "write nothing")
	verbose := fs.Bool("v", false, "print the whole reason for every section")
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
		return fmt.Errorf("%s has no contents in %s, so an import of it cannot be placed (run bourbaki toc build first)",
			*book, corpus.TOCPath(root, *book))
	}
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return err
	}
	vol, ok := books.Get(*book)
	if !ok {
		return fmt.Errorf("%s is not in %s", *book, corpus.BooksPath(root))
	}
	reviews, err := share.LoadReviews(root)
	if err != nil {
		return err
	}

	files, err := importFiles(filepath.Join(root, share.Dir, *name))
	if err != nil {
		return err
	}
	if len(files) == 0 {
		return fmt.Errorf("no import files under %s", filepath.Join(share.Dir, *name))
	}

	var rep share.Report
	var written []string
	for _, path := range files {
		f, err := corpus.ReadFile[importFrontMatter](path)
		if err != nil {
			return err
		}
		c, m, err := promoteCandidate(root, *book, *name, bt, vol, f, reviews)
		if err != nil {
			return err
		}
		d := share.Decide(*name, c)
		rep.Decisions = append(rep.Decisions, d)
		printDecision(root, path, d, *verbose)
		if !d.Promote || *dry {
			continue
		}
		out, err := writePromoted(root, m, f.Body)
		if err != nil {
			return err
		}
		written = append(written, out)
	}

	fmt.Printf("share promote: %d of %d promoted", rep.Promoted(), len(rep.Decisions))
	if r := rep.Reasons(); len(r) > 0 {
		fmt.Printf(", %s", strings.Join(r, ", "))
	}
	if *dry {
		fmt.Printf(" (nothing written, -n)")
	}
	fmt.Println()

	if len(written) == 0 {
		return nil
	}
	// Tagged and audited here rather than left to the next person. A promotion
	// is the moment a file becomes subject to every rule the corpus has, and
	// the run that made the claim is the one that should have to stand behind
	// it.
	fmt.Println("share promote: assigning tags over the promoted sections")
	if err := runTagsAssign([]string{"-corpus", root}); err != nil {
		return fmt.Errorf("promoted %d sections and tagging them failed: %w", len(written), err)
	}
	fmt.Println("share promote: running the audit")
	if err := runAudit([]string{"-corpus", root}); err != nil {
		return fmt.Errorf("promoted %d sections and the audit rejects the corpus: %w", len(written), err)
	}
	return nil
}

// candidate gathers what the rule needs to judge one import file, and the front
// matter the promoted file would carry, which is built here because it decides
// the content path the rule looks at.
func promoteCandidate(root, book, name string, bt *corpus.BookTOC, vol *corpus.Book,
	f corpus.File[importFrontMatter], reviews *share.Reviews) (share.Candidate, corpus.SectionFrontMatter, error) {

	t := share.Target{Book: f.Meta.Book, Chapter: f.Meta.Chapter, Section: f.Meta.Section,
		Intro: f.Meta.Intro || f.Meta.Section == 0}
	c := share.Candidate{Target: t, SHA256: corpus.ContentSHA256(f.Body)}
	if t.Intro {
		return c, corpus.SectionFrontMatter{}, nil
	}

	p, _, err := printedSection(root, book, bt, f.Meta.Chapter, f.Meta.Section)
	if err != nil {
		return c, corpus.SectionFrontMatter{}, err
	}
	c.Audit = share.Audit(t, f.Body, p)

	ch, err := chapterOf(bt, f.Meta.Chapter)
	if err != nil {
		return c, corpus.SectionFrontMatter{}, err
	}
	lang := f.Meta.Lang
	if lang == "" {
		lang = vol.Lang
	}
	m := corpus.SectionFrontMatter{
		Book:          vol.Book,
		BookTitle:     corpus.BookTitle(vol.Book),
		Chapter:       ch.Numeral,
		ChapterTitle:  ch.Title,
		Section:       p.Section,
		SectionTitle:  p.Title,
		Lang:          lang,
		Source:        vol.ID,
		SourceEdition: vol.Edition,
		Extraction:    "share",
		Subsections:   subsectionsOf(p),
		// The label count stands in for the statement count. An assembled
		// section gets its statements from the block parser, which needs the
		// page structure an import does not have, and the labels share audit
		// found are the same things counted a different way: every
		// proposition, corollary, definition, theorem, lemma and criterion the
		// § prints. It is the honest number available, and it is the number
		// the audit was already holding the import to.
		Statements: c.Audit.Labels,
	}
	c.ContentPath = rel(root, corpus.SectionPath(root, lang, m))
	c.Occupant = occupantOf(root, lang, m)
	c.Review = reviews.Find(name, f.Meta.Chapter, f.Meta.Section)
	return c, m, nil
}

// occupantOf is what content/ holds at a path already, or nil.
//
// A file that cannot be read as a section is treated as occupying the path all
// the same, and as coming from the pages, which is the cautious way round: a
// head this cannot parse is a reason to leave the file alone and look at it,
// not a reason to write over it.
func occupantOf(root, lang string, m corpus.SectionFrontMatter) *share.Occupant {
	path := corpus.SectionPath(root, lang, m)
	f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		return &share.Occupant{Extraction: "unreadable", PDFPages: err.Error()}
	}
	return &share.Occupant{Extraction: f.Meta.Extraction, PDFPages: f.Meta.PDFPages}
}

func subsectionsOf(p share.Printed) []corpus.Subsection {
	var out []corpus.Subsection
	for _, n := range p.Numbers {
		out = append(out, corpus.Subsection{Number: n.No, Title: n.Title})
	}
	return out
}

func chapterOf(bt *corpus.BookTOC, chapter int) (*corpus.Chapter, error) {
	for i := range bt.Chapters {
		n, err := corpus.RomanOrder(bt.Chapters[i].Numeral)
		if err != nil {
			return nil, err
		}
		if n == chapter {
			return &bt.Chapters[i], nil
		}
	}
	return nil, fmt.Errorf("the contents has no chapter %d", chapter)
}

func writePromoted(root string, m corpus.SectionFrontMatter, body string) (string, error) {
	path := corpus.SectionPath(root, m.Lang, m)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", err
	}
	m.ContentSHA256 = corpus.ContentSHA256(body)
	if err := (corpus.File[corpus.SectionFrontMatter]{Meta: m, Body: body}).Write(path); err != nil {
		return "", err
	}
	return rel(root, path), nil
}

func printDecision(root, path string, d share.Decision, verbose bool) {
	rel := rel(root, path)
	if d.Promote {
		fmt.Printf("%-34s promoted\n", rel)
	} else {
		fmt.Printf("%-34s not promoted, %s\n", rel, d.Refusal)
	}
	if verbose || !d.Promote {
		fmt.Printf("    %s\n", d.Why)
	}
}
