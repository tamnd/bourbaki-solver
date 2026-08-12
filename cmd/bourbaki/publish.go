package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/publish"
)

func runPublish(args []string) error {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: bourbaki publish [-out site] [-base /bourbaki] [-lang en,fr] [-clean]
                        [-check] [-drafts]

Builds the static site out of the committed Markdown. Every page is written
from content/, tags/ and the reference manifests, and nothing else is opened:
no PDF, no work/ directory, no network. Anyone with a clone can run this and
get the same bytes, which is the point of it.

The mathematics is set by KaTeX here rather than in the browser, so a formula
KaTeX will not read stops the build and names the file, the line and the span.
Run bourbaki audit -only P04 for the whole list of them in one go.

  -out                 where to write the site, site/ by default
  -base                the path the site is served under, empty for a domain root
  -lang                build only these languages, comma separated. All of them
                       by default. A language the corpus does not hold is an
                       error rather than an empty build.
  -clean               remove -out first, so a page a rename orphaned does not survive
  -check               build and report, write nothing
  -drafts              offer the languages under the coverage floor in the language
                       switcher. They are built and reachable either way; without
                       this they are reached from the front page and not from the
                       pages themselves. Never set by the deploy.
  -allow-broken-math   mark the formulae KaTeX refuses instead of stopping, for
                       looking at the site locally. Never set by the deploy.
`)
	}
	out := fs.String("out", "site", "directory to write")
	base := fs.String("base", "", "path the site is served under")
	lang := fs.String("lang", "", "build only these languages, comma separated")
	clean := fs.Bool("clean", false, "remove the output directory first")
	check := fs.Bool("check", false, "build in memory and write nothing")
	drafts := fs.Bool("drafts", false, "offer the languages under the coverage floor in the switcher")
	allowBroken := fs.Bool("allow-broken-math", false, "mark refused formulae instead of stopping")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}

	var only []string
	if *lang != "" {
		only = strings.Split(*lang, ",")
	}
	site, err := publish.LoadOnly(root, only)
	if err != nil {
		return err
	}
	site.Base = *base
	site.AllowBrokenMath = *allowBroken
	site.Drafts = *drafts

	langs := map[string]int{}
	exs := map[string]int{}
	for _, sec := range site.Sections {
		langs[sec.Lang]++
	}
	for _, ex := range site.Exercises {
		exs[ex.Lang]++
	}
	fmt.Printf("publish: %d sections over %d languages, %d tagged statements\n",
		len(site.Sections), len(site.Langs), len(site.Statements))
	for _, lang := range site.Langs {
		note := ""
		if site.Draft[lang] {
			note = " (below the coverage floor, kept out of the switcher)"
			if *drafts {
				note = " (below the coverage floor, offered anyway by -drafts)"
			}
		}
		fmt.Printf("\t%s\t%d sections, %d exercises%s\n", lang, langs[lang], exs[lang], note)
	}

	dest := *out
	if *check {
		// A check still writes, into a directory that goes away, because the
		// only build worth checking is the one that produces the bytes.
		tmp, err := os.MkdirTemp("", "bourbaki-publish")
		if err != nil {
			return err
		}
		defer os.RemoveAll(tmp)
		dest = tmp
	} else if *clean {
		if err := os.RemoveAll(dest); err != nil {
			return err
		}
	}

	wrote, err := site.Build(dest)
	if err != nil {
		return err
	}
	fmt.Printf("publish: %d pages\n", len(wrote))
	if n := len(site.Broken); n > 0 {
		// On standard error and named a site that cannot be published, because
		// this is the one build whose output looks finished and is not.
		files := map[string]bool{}
		for _, ref := range site.Broken {
			file, _, _ := strings.Cut(ref.At, ":")
			files[file] = true
		}
		fmt.Fprintf(os.Stderr, "publish: %d formulae are marked broken across %d files, "+
			"so this site is not publishable. Run bourbaki audit -only P04 for the list.\n", n, len(files))
	}
	if *check {
		return nil
	}

	// The counts a reader of the log would otherwise have to grep the tree for.
	// A tag page is split by what the tag is on, since an exercise tag page is a
	// second copy of a page counted elsewhere and a total that hid that would
	// read as more coverage than there is.
	byKind := map[string]int{}
	for _, w := range wrote {
		switch {
		case strings.HasPrefix(w, "tag/"):
			if tag := strings.TrimSuffix(strings.TrimPrefix(w, "tag/"), "/index.html"); site.ExerciseTag(tag) != nil {
				byKind["exercise tag pages"]++
				continue
			}
			byKind["tag pages"]++
		case strings.HasPrefix(w, "katex/"):
			byKind["KaTeX stylesheet and fonts"]++
		case strings.HasSuffix(w, ".json"):
			byKind["search indexes"]++
		case w == "index.html", w == "tags/index.html", w == "search/index.html", w == "style.css":
			byKind["site pages"]++
		case strings.HasSuffix(w, "/ex/index.html"):
			byKind["exercise list pages"]++
		case strings.Contains(w, "/ex/"):
			byKind["exercise pages"]++
		default:
			byKind["section and chapter pages"]++
		}
	}
	var kinds []string
	for k := range byKind {
		kinds = append(kinds, k)
	}
	sort.Strings(kinds)
	for _, k := range kinds {
		fmt.Printf("\t%d %s\n", byKind[k], k)
	}
	fmt.Printf("\t%s/\n", dest)
	return nil
}
