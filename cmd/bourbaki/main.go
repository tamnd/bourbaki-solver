// Command bourbaki builds the Markdown corpus in tamnd/bourbaki from the
// source PDFs.
package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"runtime/debug"

	bourbaki "github.com/tamnd/bourbaki-solver"
	"github.com/tamnd/bourbaki-solver/corpus"
)

const usage = `bourbaki builds the Bourbaki Markdown corpus from the source PDFs.

usage: bourbaki <command> [arguments]

commands:
  version          print the version and exit
  label            parse a statement label, or a running head, and print what it means

Set BOURBAKI_CORPUS to the checkout of tamnd/bourbaki.
Run bourbaki <command> -h for the flags of a command.
`

func main() {
	flag.Usage = func() { fmt.Fprint(os.Stderr, usage) }
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		flag.Usage()
		os.Exit(2)
	}

	var err error
	switch args[0] {
	case "version":
		err = runVersion()
	case "label":
		err = runLabel(args[1:])
	case "help", "-h", "--help":
		flag.Usage()
	default:
		fmt.Fprintf(os.Stderr, "bourbaki: unknown command %q\n\n", args[0])
		flag.Usage()
		os.Exit(2)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "bourbaki:", err)
		os.Exit(1)
	}
}

func runVersion() error {
	v := bourbaki.Version
	if v == "dev" {
		// A go install of a tagged commit records the version in the build
		// info, so prefer that over the ldflags default.
		if bi, ok := debug.ReadBuildInfo(); ok && bi.Main.Version != "" && bi.Main.Version != "(devel)" {
			v = bi.Main.Version
		}
	}
	fmt.Printf("bourbaki %s %s/%s %s\n", v, runtime.GOOS, runtime.GOARCH, runtime.Version())
	return nil
}

func runLabel(args []string) error {
	fs := flag.NewFlagSet("label", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: bourbaki label <string>

Parses one of the three things that identify a place in the corpus:

  a statement label   alg-viii-s1-prop-6
  a page label        "A VIII.13", "A.IV.3", "A. IV. 2"
  a section locator   "§6.5"

Page labels and section locators are recognised anywhere in the string, so a
whole running head can be passed in as it came out of pdftotext.
`)
	}
	if err := fs.Parse(args); err != nil {
		return err
	}
	if fs.NArg() != 1 {
		fs.Usage()
		os.Exit(2)
	}
	in := fs.Arg(0)

	if ref, err := corpus.ParseLabel(in); err == nil {
		fmt.Printf("statement  book=%s chapter=%s section=%d kind=%s",
			ref.Book, ref.Chapter, ref.Section, ref.Kind.Heading())
		if ref.Number > 0 {
			fmt.Printf(" number=%d\n", ref.Number)
		} else {
			fmt.Printf(" no.=%d occurrence=%d (unnumbered)\n", ref.Subsec, ref.Occurrence)
		}
		return nil
	}

	found := false
	if pl, ok := corpus.ParsePageLabel(in); ok && pl.Valid() {
		n, _ := corpus.RomanOrder(pl.Chapter)
		fmt.Printf("page       %s (book=%s chapter=%s [%d] page=%d)\n", pl, pl.Book, pl.Chapter, n, pl.Page)
		found = true
	}
	if loc, ok := corpus.ParseSectionLocator(in); ok {
		fmt.Printf("locator    %s (section=%d", loc, loc.Section)
		if loc.Subsec > 0 {
			fmt.Printf(" no.=%d", loc.Subsec)
		}
		fmt.Println(")")
		found = true
	}
	if !found {
		return fmt.Errorf("nothing recognised in %q", in)
	}
	return nil
}
