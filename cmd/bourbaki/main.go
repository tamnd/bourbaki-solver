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
  books            add, list and verify the source PDFs in manifests/books.yaml
  pagemap          map PDF pages to the page numbers Bourbaki printed
  toc              read each volume's table of contents into manifests/toc.yaml
  extract          read the pages of a volume into Markdown
  label            parse a statement label, or a running head, and print what it means
  fleet            probe the hosts, and run the ssh tunnels that reach them
  doctor           check that at least one route can take work, for use in cron

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
	case "books":
		err = runBooks(args[1:])
	case "pagemap":
		err = runPagemap(args[1:])
	case "toc":
		err = runTOC(args[1:])
	case "extract":
		err = runExtract(args[1:])
	case "label":
		err = runLabel(args[1:])
	case "fleet":
		err = runFleet(args[1:])
	case "doctor":
		err = runDoctor(args[1:])
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

// parseFlags parses args allowing flags and positional arguments to be mixed,
// which the flag package does not do on its own: it stops at the first thing
// that is not a flag. Writing the PDF path before the flags is the natural way
// to type these commands, so the parser has to cope with it.
func parseFlags(fs *flag.FlagSet, args []string) ([]string, error) {
	var positional []string
	rest := args
	for {
		if err := fs.Parse(rest); err != nil {
			return nil, err
		}
		if fs.NArg() == 0 {
			return positional, nil
		}
		positional = append(positional, fs.Arg(0))
		rest = fs.Args()[1:]
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
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 {
		fs.Usage()
		os.Exit(2)
	}
	in := pos[0]

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
