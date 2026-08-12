package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/tamnd/bourbaki-solver/glossary"
)

const glossarySetUsage = `usage: bourbaki glossary set -lang CODE [flags] TERM=RENDERING [TERM=RENDERING ...]

Corrects renderings that are already in the glossary, by hand.

Everything else that writes this file writes what a model said. This is for the
other case, which comes up often enough to want a door: a person has read a
rendering, knows it is wrong, and knows what it should be. Chinese answered
simple as 简单, uncomplicated, where these volumes mean a module with no proper
submodule, which Chinese writes 单. No rerun fixes that, because the run was not
mistaken about the word, it was mistaken about the book.

The alternative is editing the YAML, and that has already gone wrong once: a
note containing a colon and a space is not a plain scalar, and the file stopped
parsing. This goes through the same writer everything else uses, so the quoting
is not a thing anybody has to know, and through Save, so the version moves by
itself and every file written against the old rendering reports stale.

A term that is not already in the glossary is refused rather than added. Adding
is what extract and translate are for, and a set that adds is a typo that
becomes a row nobody ever looks at again.

flags:
  -lang CODE     vi, zh or ja, required
  -note TEXT     the same note on every row set here, which is where the reason
                 goes: a note is written for a person, never reaches a model,
                 and is what stops the next pass from undoing this by hand
  -corpus DIR    the checkout, default $BOURBAKI_CORPUS
  -write         write the file, which nothing does without it

Renderings that are already what they are asked to be are reported and not
counted, so running this twice is not a second version.
`

func glossarySet(args []string) error {
	fs := flag.NewFlagSet("glossary set", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, glossarySetUsage) }
	lang := fs.String("lang", "", "vi, zh or ja")
	note := fs.String("note", "", "the note to put on every row set here")
	dir := fs.String("corpus", "", "the checkout")
	write := fs.Bool("write", false, "write the file")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if !isLang(*lang) {
		return fmt.Errorf("-lang must be one of %s", strings.Join(glossary.Langs, ", "))
	}
	if len(rest) == 0 {
		fmt.Fprint(os.Stderr, glossarySetUsage)
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

	// The rows are found first and written afterwards, so a run that names one
	// term wrong changes nothing at all. A half applied editorial pass is worse
	// than none: the file then holds some of a decision and no record of which
	// part.
	type change struct {
		at           int
		en, was, now string
	}
	index := map[string]int{}
	for i, t := range g.Terms {
		index[glossary.Key(t.EN)] = i
	}
	changes := make([]change, 0, len(rest))
	same := 0
	for _, arg := range rest {
		en, rendering, ok := strings.Cut(arg, "=")
		en, rendering = strings.TrimSpace(en), strings.TrimSpace(rendering)
		if !ok || en == "" || rendering == "" {
			return fmt.Errorf("%q is not TERM=RENDERING", arg)
		}
		at, found := index[glossary.Key(en)]
		if !found {
			return fmt.Errorf("no row for %q, and set does not add rows", en)
		}
		was := g.Terms[at].In(*lang)
		if was == rendering {
			same++
			continue
		}
		changes = append(changes, change{at: at, en: g.Terms[at].EN, was: was, now: rendering})
	}

	for _, c := range changes {
		was := c.was
		if was == "" {
			was = "(none)"
		}
		fmt.Printf("\t%-34s %s -> %s\n", c.en, was, c.now)
	}
	if same > 0 {
		fmt.Printf("glossary set: %d already read that way\n", same)
	}
	if len(changes) == 0 {
		fmt.Println("glossary set: nothing to change")
		return nil
	}
	fmt.Printf("glossary set: %d renderings in %s\n", len(changes), *lang)
	if !*write {
		fmt.Println("nothing was written, run again with -write")
		return nil
	}

	for _, c := range changes {
		g.Terms[c.at].Set(*lang, c.now)
		if strings.TrimSpace(*note) != "" {
			g.Terms[c.at].Note = strings.TrimSpace(*note)
		}
	}
	version, bumped, err := g.Save(glossary.Path(root))
	if err != nil {
		return err
	}
	fmt.Printf("wrote %s\n", glossary.Path(root))
	if bumped {
		fmt.Printf("glossary set: version %d, so every file that mentions one of these terms is stale\n", version)
	}
	return nil
}

func isLang(lang string) bool {
	for _, value := range glossary.Langs {
		if value == lang {
			return true
		}
	}
	return false
}
