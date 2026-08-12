package main

import (
	"flag"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/refs"
)

func runRefs(args []string) error {
	fs := flag.NewFlagSet("refs", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: bourbaki refs build [-lang en] [-check] [-min 90]

Reads the cross-references out of the committed Markdown and writes the graph
to manifests/refs/, one file to a §, with four reports under reports/.

The prose is never rewritten. A citation stays the sentence Bourbaki wrote and
the link to what it points at lives in the manifest, so the text on the page
remains a transcription and nothing has to be undone to translate it.

Nothing here reads a PDF, so it runs in CI, where there are none.

  -lang    which language of the corpus to read, en by default
  -check   fail if the manifest or a report is out of date, and write nothing
  -min     fail if fewer than this per cent of references resolve
`)
	}
	lang := fs.String("lang", "en", "language to read")
	check := fs.Bool("check", false, "fail if anything is out of date")
	min := fs.Float64("min", 0, "fail below this resolution rate, in per cent")
	pos, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(pos) != 1 || pos[0] != "build" {
		fs.Usage()
		os.Exit(2)
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}

	res, err := refs.Build(root, *lang)
	if err != nil {
		return err
	}
	total := len(res.Edges) + len(res.Unresolved)
	rate := 0.0
	if total > 0 {
		rate = 100 * float64(len(res.Edges)) / float64(total)
	}

	if *check {
		out, err := res.Manifest().Stale(root)
		if err != nil {
			return err
		}
		reports, err := res.StaleReports(root)
		if err != nil {
			return err
		}
		out = append(out, reports...)
		if len(out) > 0 {
			return fmt.Errorf("%s out of date, run bourbaki refs build", strings.Join(out, ", "))
		}
		fmt.Printf("refs build: %d references, %d edges, %.1f%% resolved, manifest and reports up to date\n",
			total, len(res.Edges), rate)
	} else {
		if err := res.Manifest().Save(root); err != nil {
			return err
		}
		wrote, err := res.Reports(root)
		if err != nil {
			return err
		}
		fmt.Printf("refs build: %d references over %s, %d edges and %d unresolved, %.1f%% resolved\n",
			total, *lang, len(res.Edges), len(res.Unresolved), rate)
		fmt.Printf("\t%s/\n", refs.ManifestDir)
		for _, w := range wrote {
			fmt.Printf("\t%s\n", w)
		}
	}

	// The shape the rate was measured over, because one number hides the form
	// that goes wrong. A formula is counted in and never resolves, on purpose:
	// a numbered display carries no tag, and pretending otherwise would flatter
	// the rate by a hundred references.
	var hows []string
	for h := range res.Counts {
		hows = append(hows, h)
	}
	sort.Strings(hows)
	for _, h := range hows {
		fmt.Printf("\t%-14s %d\n", h, res.Counts[h])
	}

	if *min > 0 && rate < *min {
		return fmt.Errorf("%.1f%% of references resolve, below the %.0f%% this corpus is held to", rate, *min)
	}
	return nil
}
