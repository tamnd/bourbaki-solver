package main

import (
	"flag"
	"fmt"
	"os"
	"sort"

	"github.com/tamnd/bourbaki-solver/glossary"
)

const glossaryTidyUsage = `usage: bourbaki glossary tidy [flags]

Applies the glossary rules to the rows already in manifests/glossary.yaml and
removes the ones that would not be accepted today.

  -corpus DIR    the checkout, default $BOURBAKI_CORPUS
  -write         write the file, otherwise print what would go and change nothing

Two things come out. A row whose English is one of Bourbaki's operators, which
is notation rather than vocabulary: "ker" was rendered "hạt nhân", which is the
right Vietnamese and the wrong thing to put in front of a model that is about
to meet $\operatorname{Ker} f$. And a rendering carrying a formula its term
does not carry, which is what a model does with a term the miner cut out of a
line that had lost its delimiters.

Nothing is corrected here. A row is removed or a rendering is cleared, and what
should be there instead is for a person to write.

This is worth running after a glossary translate and after any change to the
rules. It is not run automatically: a command that silently rewrote the
terminology file every time something loaded it would make the file depend on
which command ran last.
`

func glossaryTidy(args []string) error {
	fs := flag.NewFlagSet("glossary tidy", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, glossaryTidyUsage) }
	dir := fs.String("corpus", "", "the checkout")
	write := fs.Bool("write", false, "write the file")
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
	before := len(g.Terms)
	dropped := g.Tidy()
	if len(dropped) == 0 {
		fmt.Printf("glossary tidy: %d terms, nothing to remove\n", before)
		return nil
	}
	sort.Slice(dropped, func(i, j int) bool {
		if dropped[i].Reason != dropped[j].Reason {
			return dropped[i].Reason < dropped[j].Reason
		}
		return dropped[i].EN < dropped[j].EN
	})
	for _, d := range dropped {
		if d.Line != "" {
			fmt.Printf("\t%-40s %s\n\t%-40s %s\n", d.EN, d.Line, "", d.Reason)
			continue
		}
		fmt.Printf("\t%-40s %s\n", d.EN, d.Reason)
	}
	fmt.Printf("glossary tidy: %d terms, %d removals, %d terms left\n", before, len(dropped), len(g.Terms))
	if !*write {
		fmt.Println("nothing was written, run again with -write")
		return nil
	}
	version, bumped, err := g.Save(glossary.Path(root))
	if err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", glossary.Path(root))
	if bumped {
		fmt.Printf("glossary tidy: version %d, so every translated file is stale and bourbaki translate will do it again\n", version)
	}
	return nil
}
