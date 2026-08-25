package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/tamnd/bourbaki-solver/book"
	"github.com/tamnd/bourbaki-solver/corpus"
)

func runBook(args []string) error {
	fs := flag.NewFlagSet("book", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: bourbaki book -book <id> [-lang en,vi] [-chapter I] [-out work/books]
                      [-no-pdf] [-no-epub] [-bundle url] [-cached] [-epoch n]
                      [-tolerance 0.20] [-max-overfull 200] [-no-cover-check]

Builds a printed volume back out of content/ and audits what came out.

Everything else in this repository reads in one direction: the PDFs went in and
the Markdown came out, and no gate has ever asked whether what came out is a
book. The audits read one file at a time and cannot see a chapter missing from
the middle, a section that lost half its subsections in the assembly, or a
volume that is a fifth of the length of the printing it was taken from. This
sets the whole volume as a book and looks at the result, which finds all three
at once.

Two files come out of one load and one masking of the same mathematics. The PDF
is set by tectonic in a class that carries the printing's own measured margins,
running heads and cover. The EPUB is XHTML with the formulae as MathML, which is
what a reading system on a phone will show, and it is the format a reader of a
volume that has never been printed in their language is actually going to open.

Nothing is committed. The output goes under work/, which the corpus ignores,
because a built book is a statement about one build and not about the corpus.

  -book             the volume to build, as manifests/books.yaml names it
  -lang             the languages to build it in, comma separated. The language
                    of the printing by default. A language builds whatever of
                    the volume has been translated so far, and the audit says
                    how much that is rather than refusing.
  -chapter          build one chapter only, by numeral, for a quick look
  -out              where to write, work/books under the corpus by default
  -no-pdf           write the .tex and skip the typesetter
  -no-epub          skip the EPUB
  -bundle           where tectonic fetches its TeX packages, empty for its own
                    default. A machine whose resolver cannot reach the default
                    needs this, and so does a build that wants the package
                    versions pinned.
  -cached           refuse to fetch anything, for a reproducible build against a
                    cache somebody kept
  -epoch            the timestamp everything is pinned to, so that two builds of
                    the same content come out as the same bytes
  -tolerance        how far the page count may sit from the printing's own
                    before that check fails, as a fraction
  -max-overfull     the most lines that may run past the measure
  -max-stray        the most TeX control sequences that may be loose in the
                    prose, and -max-wide the most arrays that had to be widened
                    to hold their own rows. Both are damage the text layer did
                    to the book rather than anything the build got wrong, both
                    are in the corpus today, and both are ceilings so that the
                    number can be ratcheted down as they are repaired. A ceiling
                    may be lowered and may never be raised.
  -max-wide         see -max-stray
  -no-cover-check   do not render the first page to look at the cover

Exits 1 if any check failed, so this can be a gate.
`)
	}
	id := fs.String("book", "", "the volume to build")
	lang := fs.String("lang", "", "the languages to build it in")
	chapter := fs.String("chapter", "", "build one chapter only")
	out := fs.String("out", "", "where to write")
	noPDF := fs.Bool("no-pdf", false, "write the .tex and skip the typesetter")
	noEPUB := fs.Bool("no-epub", false, "skip the EPUB")
	bundle := fs.String("bundle", os.Getenv("BOURBAKI_TEX_BUNDLE"), "where tectonic fetches its packages")
	cached := fs.Bool("cached", false, "refuse to fetch anything")
	epoch := fs.Int64("epoch", 1735689600, "the timestamp everything is pinned to")
	tolerance := fs.Float64("tolerance", 0.20, "how far the page count may sit from the printing's")
	maxOverfull := fs.Int("max-overfull", 200, "the most lines that may run past the measure")
	maxStray := fs.Int("max-stray", 0, "the most TeX control sequences loose in the prose")
	maxWide := fs.Int("max-wide", 0, "the most arrays widened to hold their own rows")
	noCover := fs.Bool("no-cover-check", false, "do not render the first page to look at the cover")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *id == "" {
		fs.Usage()
		return errors.New("book: -book is required")
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return err
	}
	meta, ok := books.Get(*id)
	if !ok {
		return fmt.Errorf("book: no volume %s in manifests/books.yaml", *id)
	}
	langs := []string{meta.Lang}
	if *lang != "" {
		langs = strings.Split(*lang, ",")
	}
	dir := *out
	if dir == "" {
		dir = filepath.Join(root, "work", "books")
	}

	opt := book.Options{Epoch: *epoch, Bundle: *bundle, Cached: *cached}
	aopt := book.AuditOptions{
		Tolerance: *tolerance, Overfull: *maxOverfull,
		Stray: *maxStray, Wide: *maxWide,
		Cover: !*noCover && !*noPDF,
	}

	failed := 0
	for _, l := range langs {
		l = strings.TrimSpace(l)
		if l == "" {
			continue
		}
		a, err := buildOne(root, dir, *id, l, *chapter, opt, aopt, *noPDF, *noEPUB)
		if err != nil {
			return fmt.Errorf("%s-%s: %w", *id, l, err)
		}
		fmt.Print(a.Report())
		failed += a.Failed()
	}
	if failed > 0 {
		return fmt.Errorf("book: %d checks failed", failed)
	}
	return nil
}

// buildOne is one volume in one language, from the corpus to the audit report.
func buildOne(root, out, id, lang, chapter string, opt book.Options, aopt book.AuditOptions, noPDF, noEPUB bool) (*book.Audit, error) {
	v, err := book.Load(root, id, lang)
	if err != nil {
		return nil, err
	}
	if chapter != "" {
		c, ok := v.Chapter(chapter)
		if !ok {
			return nil, fmt.Errorf("no chapter %s in this volume", chapter)
		}
		v.Chapters = []*book.Chapter{c}
		// One chapter is not the volume, so the two checks that compare against
		// the whole of it would fail on a build that is doing exactly what it was
		// asked to. The rest of the audit is about what is on the page and holds
		// for a chapter as well as for a book.
		aopt.Tolerance = 1e9
	}
	dir := filepath.Join(out, v.ID())
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}

	d, err := book.Write(v)
	if err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "book.tex"), []byte(d.TeX), 0o644); err != nil {
		return nil, err
	}

	var b *book.Build
	if !noPDF {
		b, err = book.Run(context.Background(), dir, d, opt)
		if err != nil {
			// A typesetter that stopped is a failure worth reporting rather than
			// worth hiding, and the audit still has everything the writer found,
			// so the error goes to the terminal and the audit runs on what there
			// is. A build that produced no PDF fails the checks that need one.
			fmt.Fprintf(os.Stderr, "%s: %v\n", v.ID(), err)
			if b != nil && b.Pages == 0 {
				b = nil
			}
		}
	}

	var e *book.EPUB
	if !noEPUB {
		e, err = book.WriteEPUB(filepath.Join(dir, "book.epub"), v, opt)
		if err != nil {
			return nil, err
		}
	}

	a := book.Inspect(root, v, d, b, e, aopt)
	if err := os.WriteFile(filepath.Join(dir, "audit.md"), []byte(a.Markdown()), 0o644); err != nil {
		return nil, err
	}
	return a, nil
}
