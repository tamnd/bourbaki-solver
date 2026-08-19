package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/tamnd/bourbaki-solver/share"
)

const shareUsage = `usage: bourbaki share <command> [arguments]

commands:
  import    read public ChatGPT share links into imports/
  audit     hold an import against the printed volume
  promote   move a checked import into content/, or say why not

Set BOURBAKI_CORPUS to the checkout of tamnd/bourbaki.
`

func runShare(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, shareUsage)
		os.Exit(2)
	}
	switch args[0] {
	case "import":
		return shareImport(args[1:])
	case "audit":
		return shareAudit(args[1:])
	case "promote":
		return sharePromote(args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(os.Stderr, shareUsage)
		return nil
	}
	return fmt.Errorf("unknown share command %q", args[0])
}

const shareImportUsage = `usage: bourbaki share import [flags] <url>...

Reads public ChatGPT share links into imports/<book>/chapter_<n>/<n>.<m>.md.

  -corpus DIR    the checkout, default $BOURBAKI_CORPUS
  -book NAME     the book the pages belong to, for example sets
  -n             say what would be written and write nothing

A link may be given on its own, in which case the conversation's own title says
where in the book it goes, or as <label>=<url> when the title does not say or
says the wrong thing. A label is intro, or a chapter and a section: 1.1.

This is the cheapest transcription this project has. The OCR path drives a
browser on a server, waits about 150 seconds a page and then argues with what
comes back. A share link is a conversation somebody already had: reading one is
an HTTP GET, it needs no account, no profile and no browser, and it cannot be
throttled because nothing is being asked for.

What it is not is verified. Nothing here has been checked against the printed
book, so it is written to a tree of its own, where the corpus audit does not
reach it and cannot pass it by accident. Promoting a file into content/ is a
separate job, done by a person who has read it.

An answer that refuses, narrates or arrives wrapped in the provider's own
markup abandons the whole import with the answer named. Half a section is worse
than none: none is obvious.
`

func shareImport(args []string) error {
	fs := flag.NewFlagSet("share import", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, shareImportUsage) }
	dir := fs.String("corpus", "", "the checkout")
	book := fs.String("book", "", "the book the pages belong to")
	dry := fs.Bool("n", false, "write nothing")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) == 0 {
		fs.Usage()
		os.Exit(2)
	}
	root, err := corpusRoot(*dir)
	if err != nil {
		return err
	}

	ctx := context.Background()
	var written, refused int
	for _, arg := range pos {
		label, url := labelled(arg)
		rel, err := importOne(ctx, root, *book, label, url, *dry)
		if err != nil {
			refused++
			fmt.Printf("\t%-60s %v\n", url, err)
			continue
		}
		written++
		fmt.Printf("\t%s\n", rel)
	}
	verb := "written"
	if *dry {
		verb = "would be written"
	}
	fmt.Printf("share import: %d %s, %d refused\n", written, verb, refused)
	if refused > 0 {
		return fmt.Errorf("%d of %d links were refused", refused, len(pos))
	}
	return nil
}

func importOne(ctx context.Context, root, book, label, url string, dry bool) (string, error) {
	html, err := share.Fetch(ctx, url)
	if err != nil {
		return "", err
	}
	conv, err := share.Parse(url, html)
	if err != nil {
		return "", err
	}
	var target share.Target
	if label == "" {
		target, err = share.TargetFromTitle(book, conv.Title)
	} else {
		target, err = share.ParseTarget(book, label)
	}
	if err != nil {
		return "", err
	}
	page, err := share.Markdown(conv)
	if err != nil {
		return "", err
	}
	im := &share.Import{Target: target, Page: page, Conv: conv, URL: url}
	if dry {
		return fmt.Sprintf("%s from %d answers, %d joins", target.Path(),
			len(conv.Turns), joinedCount(page)), nil
	}
	rel, err := im.Write(root)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s from %d answers, %d joins", rel, len(conv.Turns), joinedCount(page)), nil
}

func joinedCount(p *share.Page) int {
	n := 0
	for _, b := range p.Boundaries {
		if b.Joined {
			n++
		}
	}
	return n
}

// labelled reads a <label>=<url> argument. A bare URL has no label, and the equals
// signs inside a URL's query string are why this splits on the first one only
// and why a piece that looks like a URL is never a label.
func labelled(arg string) (string, string) {
	i := strings.Index(arg, "=")
	if i < 0 || strings.HasPrefix(arg, "http") {
		return "", arg
	}
	return arg[:i], arg[i+1:]
}
