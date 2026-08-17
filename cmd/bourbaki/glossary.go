package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/glossary"
)

// The glossary commands: mine the English for terms, and say what the state of
// the contract is.
//
// Translating is the expensive half of this project and terminology is the half
// of that which cannot be fixed afterwards. A formula that came out wrong is
// one file to repair; a term rendered three ways across a chapter is every file
// that used it, and nobody notices until somebody searches.

const glossaryUsage = `usage: bourbaki glossary <command> [arguments]

The terminology contract between English and the three translations.

commands:
  extract     mine candidate terms out of the English corpus
  translate   ask the fleet for the rendering of the terms that have none
  set         correct renderings that are already there, by hand
  drop        take out a row that is right about the word and wrong about
              these books
  tidy        remove the rows the rules would not accept today
  status      what manifests/glossary.yaml holds, per language

Run bourbaki glossary <command> -h for the flags of a command.
`

func runGlossary(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, glossaryUsage)
		os.Exit(2)
	}
	switch args[0] {
	case "extract":
		return glossaryExtract(args[1:])
	case "translate":
		return glossaryTranslate(args[1:])
	case "set":
		return glossarySet(args[1:])
	case "drop":
		return glossaryDrop(args[1:])
	case "tidy":
		return glossaryTidy(args[1:])
	case "status":
		return glossaryStatus(args[1:])
	}
	fmt.Fprint(os.Stderr, glossaryUsage)
	os.Exit(2)
	return nil
}

const glossaryExtractUsage = `usage: bourbaki glossary extract [flags]

Mines candidate terms out of the English corpus and writes them to
manifests/glossary-candidates.yaml, for a person to curate into
manifests/glossary.yaml.

Four sources, and every candidate says which of them found it.

  title     the section and subsection titles, which are Bourbaki's own
            terminology and the best source here by a distance
  defined   the phrase after "is called", "is said to be" or "we call"
  prose     the recurring noun phrases of the running text
  kind      the words for the statement kinds, which are put in whether the
            mining found them or not

A Book with nothing assembled yet is mined from its table of contents, which
holds the same § and no. titles the section files would give. The glossary has
to be settled before the first translation and the first translation runs the
day the first chapter assembles, so a Book that waits for its own content waits
too long. Theory of Sets is the case: the glossary was mined from Algebra and
Lie and has no row for set, assembly, ordered set, well-ordered set, ordinal,
equipotent or species of structure, and its table of contents names all of them.
A Book that is assembled is not mined this way, because its titles are already
read off its section files and counting them twice would inflate the number a
curator judges a term by.

Spec 06 asks for the terms Bourbaki italicises at first use, which is the
convention the printed page uses and would be the best source of all. It is not
available: the whole English corpus carries seven runs of emphasis and none of
them is a defined term, because the volume's text layer did not keep the font.

flags:
  -corpus DIR   the checkout, default $BOURBAKI_CORPUS
  -min N        how often a phrase must occur in the prose to be offered,
                default 5. A title, a definition and a statement kind are
                offered whatever their count, because those three sources are
                curated and frequency is not what makes them worth having.
  -words N      the longest phrase mined, in words, default 4
  -limit N      keep only this many, commonest first, default all
  -o PATH       write somewhere else
`

func glossaryExtract(args []string) error {
	fs := flag.NewFlagSet("glossary extract", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, glossaryExtractUsage) }
	dir := fs.String("corpus", "", "the checkout")
	min := fs.Int("min", glossary.DefaultOptions().MinCount, "minimum prose count")
	words := fs.Int("words", glossary.DefaultOptions().MaxWords, "longest phrase in words")
	limit := fs.Int("limit", 0, "keep only this many")
	out := fs.String("o", "", "write somewhere else")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpusRoot(*dir)
	if err != nil {
		return err
	}

	docs, err := englishDocs(root)
	if err != nil {
		return err
	}
	unread, err := unassembledTOCDocs(root, docs)
	if err != nil {
		return err
	}
	docs = append(docs, unread...)
	if len(docs) == 0 {
		return fmt.Errorf("%s has no English content to mine", root)
	}

	terms := glossary.Extract(docs, glossary.Options{MinCount: *min, MaxWords: *words, Limit: *limit})
	path := *out
	if path == "" {
		path = glossary.CandidatesPath(root)
	}
	if err := glossary.WriteCandidates(path, glossary.Candidates{
		Version: 1, From: "content/en", Terms: terms,
	}); err != nil {
		return err
	}

	bySource := map[string]int{}
	for _, t := range terms {
		for _, s := range t.Sources {
			bySource[s]++
		}
	}
	fmt.Printf("glossary extract: %d candidates from %d English files\n", len(terms), len(docs))
	for _, s := range []string{glossary.SourceTitle, glossary.SourceDefined, glossary.SourceProse, glossary.SourceKind} {
		fmt.Printf("\t%-8s %5d\n", s, bySource[s])
	}
	fmt.Printf("\t%s\n", rel(root, path))
	return nil
}

// englishDocs reads content/en for the miner: the body, and the titles out of
// the front matter.
//
// The exercises are read as well as the sections. They are a third of the
// corpus, they are where the unusual vocabulary is, and a glossary built from
// the statements alone would miss every word that only an exercise uses.
// unassembledTOCDocs is one document per chapter of every English Book that has
// nothing in content/en yet, carrying that chapter's titles and no body.
//
// The titles are the § and no. headings the volume prints, taken off
// manifests/toc.yaml, which is where the printed table of contents was read to.
// They are the same strings the section files would give once the volume is
// assembled, and they are the source the miner rates highest, so a Book can have
// its terminology settled while its pages are still being read.
//
// The bar is the Book and not the volume, because Algebra is three volumes and
// one Book and one directory under content/en. Assembling any of the three is
// enough to have the Book's titles read off its section files, and mining the
// other two volumes' tables of contents on top of that would count the same
// title twice.
//
// French volumes are left out. Their titles are French, and a French word in a
// list of English candidates is a rendering asked for in the wrong direction.
func unassembledTOCDocs(root string, have []glossary.Doc) ([]glossary.Doc, error) {
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return nil, err
	}
	toc, err := corpus.LoadTOC(root)
	if err != nil {
		return nil, err
	}
	assembled := map[string]bool{}
	for _, d := range have {
		if parts := strings.Split(d.Path, "/"); len(parts) > 2 {
			assembled[parts[2]] = true
		}
	}
	var out []glossary.Doc
	for _, b := range books.Books {
		if b.Lang != "en" || assembled[b.Book] {
			continue
		}
		bt, ok := toc.Get(b.ID)
		if !ok {
			continue
		}
		for _, ch := range bt.Chapters {
			d := glossary.Doc{
				Path:   "manifests/toc.yaml",
				Titles: []string{ch.Title},
			}
			for _, s := range ch.Sections {
				d.Titles = append(d.Titles, s.Title)
				for _, sub := range s.Subsections {
					d.Titles = append(d.Titles, sub.Title)
				}
			}
			out = append(out, d)
		}
	}
	return out, nil
}

func englishDocs(root string) ([]glossary.Doc, error) {
	var out []glossary.Doc
	dir := filepath.Join(root, "content", "en")
	err := filepath.WalkDir(dir, func(path string, e os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if e.IsDir() || filepath.Ext(path) != ".md" {
			return nil
		}
		relPath, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		d := glossary.Doc{Path: filepath.ToSlash(relPath)}
		if strings.Contains(d.Path, "/exercises/") {
			f, err := corpus.ReadFile[corpus.ExerciseFrontMatter](path)
			if err != nil {
				return err
			}
			d.Body = f.Body
		} else {
			f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
			if err != nil {
				return err
			}
			d.Body = f.Body
			d.Titles = append(d.Titles, f.Meta.SectionTitle, f.Meta.ChapterTitle)
			for _, s := range f.Meta.Subsections {
				d.Titles = append(d.Titles, s.Title)
			}
		}
		out = append(out, d)
		return nil
	})
	if os.IsNotExist(err) {
		return nil, nil
	}
	return out, err
}

const glossaryStatusUsage = `usage: bourbaki glossary status [flags]

What manifests/glossary.yaml holds: the version, the number of terms, and how
many of them are rendered in each language.

flags:
  -corpus DIR   the checkout, default $BOURBAKI_CORPUS
  -lang CODE    list the terms this language is still missing
`

func glossaryStatus(args []string) error {
	fs := flag.NewFlagSet("glossary status", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, glossaryStatusUsage) }
	dir := fs.String("corpus", "", "the checkout")
	lang := fs.String("lang", "", "list what this language is missing")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpusRoot(*dir)
	if err != nil {
		return err
	}
	g, err := glossary.Load(root)
	if err != nil {
		return err
	}
	if len(g.Terms) == 0 {
		fmt.Printf("glossary status: %s holds no terms yet\n", rel(root, glossary.Path(root)))
		return nil
	}
	fmt.Printf("glossary status: version %d, %d terms\n", g.Version, len(g.Terms))
	for _, l := range glossary.Langs {
		n := len(g.In(l))
		fmt.Printf("\t%-3s %5d of %d  %5.1f%%\n", l, n, len(g.Terms), 100*float64(n)/float64(len(g.Terms)))
	}
	if *lang == "" {
		return nil
	}
	var missing []string
	for _, t := range g.Terms {
		if t.In(*lang) == "" {
			missing = append(missing, t.EN)
		}
	}
	sort.Strings(missing)
	fmt.Printf("\n%s is missing %d:\n", *lang, len(missing))
	for _, m := range missing {
		fmt.Printf("\t%s\n", m)
	}
	return nil
}

// corpusRoot is the checkout a command works on: the flag when it is given and
// $BOURBAKI_CORPUS otherwise.
func corpusRoot(dir string) (string, error) {
	if dir != "" {
		return dir, nil
	}
	return corpus.Root()
}
