package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/tamnd/bourbaki-solver/glossary"
)

const glossaryDropUsage = `usage: bourbaki glossary drop [flags] TERM [TERM ...]

Takes a row out of the glossary, by hand.

tidy removes what the rules can see is wrong. This is for what only a reader
can see: a row that is right about the word and wrong about these books.

"identity" is the case it was written for. It reads "đơn vị", the unit of a
ring, which is what the word means throughout Algebra and is why somebody wrote
it. Theory of Sets says "the identity" fifty eight times and means the identity
mapping every time, "ánh xạ đồng nhất", which is a different thing said with
the same English word. The row is put in front of every model asked to
translate a page that says it, and the row is wrong on that page. There is
already a row for "identity element" and one for "identity mapping", so what is
wanted is not a better rendering of the bare word but no bare word at all.

Removing is not the same as correcting and the difference matters here. A term
that means two things in two volumes has no one rendering to correct it to, and
a glossary that insists on one is a glossary that is wrong half the time it is
read.

The alternative is editing the YAML by hand, and that has gone wrong before. A
note carrying a colon and a space is not a plain scalar and the file stopped
parsing. This goes through the writer everything else goes through, and through
Save, so the version moves and every file that was shown the row reports stale
and is translated again.

A term that is not in the glossary is refused rather than passed over. Dropping
what is not there is a typo, and a typo that reports success is a typo nobody
finds.

flags:
  -corpus DIR    the checkout, default $BOURBAKI_CORPUS
  -write         write the file, which nothing does without it
`

func glossaryDrop(args []string) error {
	fs := flag.NewFlagSet("glossary drop", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, glossaryDropUsage) }
	dir := fs.String("corpus", "", "the checkout")
	write := fs.Bool("write", false, "write the file")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		fmt.Fprint(os.Stderr, glossaryDropUsage)
		os.Exit(2)
	}
	root, err := corpusRoot(*dir)
	if err != nil {
		return err
	}
	g, err := glossary.Load(root)
	if err != nil {
		return err
	}

	// Every term is looked up before anything goes, so a run that names one of
	// them wrong takes none of them out. Half an editorial decision applied is
	// worse than none: the file then holds part of it and no record of which
	// part.
	index := map[string]int{}
	for i, t := range g.Terms {
		index[glossary.Key(t.EN)] = i
	}
	going := map[int]bool{}
	for _, arg := range rest {
		en := strings.TrimSpace(arg)
		if en == "" {
			return fmt.Errorf("%q is not a term", arg)
		}
		at, found := index[glossary.Key(en)]
		if !found {
			return fmt.Errorf("no row for %q, so there is nothing to drop", en)
		}
		going[at] = true
	}

	kept := make([]glossary.Term, 0, len(g.Terms))
	for i, t := range g.Terms {
		if going[i] {
			fmt.Printf("\t%-34s %s\n", t.EN, renderings(t))
			continue
		}
		kept = append(kept, t)
	}
	before := len(g.Terms)
	fmt.Printf("glossary drop: %d terms, %d removals, %d terms left\n", before, len(going), len(kept))
	if !*write {
		fmt.Println("nothing was written, run again with -write")
		return nil
	}
	g.Terms = kept
	version, bumped, err := g.Save(glossary.Path(root))
	if err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", glossary.Path(root))
	if bumped {
		fmt.Printf("glossary drop: version %d, so every file that was shown one of these terms is stale\n", version)
	}
	return nil
}

// renderings is what a row said, for the line that reports it going. A person
// about to remove a row should see what is being removed in every language and
// not only in the one they were reading.
func renderings(t glossary.Term) string {
	var out []string
	for _, lang := range glossary.Langs {
		if r := t.In(lang); r != "" {
			out = append(out, lang+" "+r)
		}
	}
	if len(out) == 0 {
		return "(no renderings)"
	}
	return strings.Join(out, ", ")
}
