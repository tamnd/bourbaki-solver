package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	bourbaki "github.com/tamnd/bourbaki-solver"
	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/quality"
)

// audit is every quality claim this project makes, run as a rule. The rules
// themselves are in package quality; this is the flags, the assembler it needs
// for S09, and the exit status.
//
// The exit status is the hard rules alone. A soft rule failing means somebody
// should look, and a build that goes red for it is a build people learn to
// ignore, which costs more than the rule is worth.

const auditUsage = `usage: bourbaki audit [flags]

Runs every rule in spec 08 over the committed corpus and says what is wrong and
where. Nothing here opens a PDF, calls a model or reaches the network, so it
runs the same in CI, where the PDFs are not and cannot be, as it does here.

A hard rule failing means the corpus is wrong, and the exit status is the count
of those. A soft rule failing means somebody should look. A rule with nothing to
look at is reported as not run rather than as a pass.

flags:
  -corpus DIR    the checkout to audit, default $BOURBAKI_CORPUS
  -only LIST     only these rules or groups, as S07,mathematics
  -skip LIST     every rule but these
  -lang LIST     only these languages, default every language the corpus has
  -base REV      the commit T05 reads the history of tags/tags against
  -validate-tex  turn on M04, which is off by default because it is the slow one
  -report PATH   write the Markdown report here, relative to the corpus root
  -json PATH     write the machine form here, or - for standard output
  -list          print the rules and exit
  -v             print the rules that passed as well as the ones that did not
`

func runAudit(args []string) error {
	fs := flag.NewFlagSet("audit", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, auditUsage) }
	dir := fs.String("corpus", "", "the checkout to audit")
	only := fs.String("only", "", "only these rules or groups")
	skip := fs.String("skip", "", "every rule but these")
	lang := fs.String("lang", "", "only these languages")
	base := fs.String("base", "", "the commit T05 reads tags/tags against")
	validateTeX := fs.Bool("validate-tex", false, "turn on M04")
	report := fs.String("report", "", "write the Markdown report here")
	asJSON := fs.String("json", "", "write the machine form here, or - for stdout")
	list := fs.Bool("list", false, "print the rules and exit")
	verbose := fs.Bool("v", false, "print the rules that passed too")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	if *list {
		return listChecks()
	}

	root := *dir
	if root == "" {
		var err error
		if root, err = corpus.Root(); err != nil {
			return err
		}
	}
	abs, err := filepath.Abs(root)
	if err != nil {
		return err
	}
	root = abs

	opt := quality.Options{
		Root:        root,
		Langs:       split(*lang),
		Only:        split(*only),
		Skip:        split(*skip),
		ValidateTeX: *validateTeX,
		Base:        *base,
	}
	// S09 needs the assembler's output, and the assembler's driver lives with
	// this command. It is run here rather than inside the rule so that the rules
	// stay pure functions of what is on disk, and so that a corpus the assembler
	// cannot run over at all reports S09 as not run rather than as a pass.
	if opt.Selected("S09") {
		opt.Assembled, opt.Stale, opt.AssembleErr = assembleAll(root)
	}

	res, err := quality.Run(opt)
	if err != nil {
		return err
	}

	if out := quality.Text(res, *verbose); out != "" {
		fmt.Print(out)
	}
	if *report != "" {
		path := *report
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(path, []byte(quality.Report(res)), 0o644); err != nil {
			return err
		}
		fmt.Printf("audit: wrote %s\n", rel(root, path))
	}
	if *asJSON != "" {
		b, err := quality.JSON(res, bourbaki.Version, headCommit(root))
		if err != nil {
			return err
		}
		b = append(b, '\n')
		if *asJSON == "-" {
			os.Stdout.Write(b)
		} else {
			path := *asJSON
			if !filepath.IsAbs(path) {
				path = filepath.Join(root, path)
			}
			if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
				return err
			}
			if err := os.WriteFile(path, b, 0o644); err != nil {
				return err
			}
		}
	}

	ran := len(res.Outcomes) - len(res.Skipped())
	fmt.Printf("audit: %d rules ran, %d could not, %d hard findings, %d soft\n",
		ran, len(res.Skipped()), res.Hard(), res.Soft())
	if hard := quality.Failures(res); len(hard) > 0 {
		return fmt.Errorf("audit: %d hard findings in %s", res.Hard(), strings.Join(hard, ", "))
	}
	return nil
}

// assembleAll runs the assembler over every book that has pages, and returns
// what it would write.
//
// A book with no pages is not a failure, it is a volume nobody has read in yet,
// and it is left out rather than reported as sixty missing files. A book with a
// few pages is the same thing half done: alg-i-iii has three pilot pages and no
// complete chapter, so the assembler stops on it, and stopping the whole audit
// there would take S09 away from alg-viii, which is the volume that is finished
// and the one the rule exists for. Those are skipped, and the skip is printed
// rather than swallowed, because a coverage gap nobody is told about reads
// exactly like a pass.
func assembleAll(root string) (map[string][]byte, []string, string) {
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return nil, nil, "the books manifest will not load: " + err.Error()
	}
	files := map[string][]byte{}
	var stale []string
	var ran int
	for _, b := range books.Books {
		names, err := filepath.Glob(filepath.Join(corpus.PagesDir(root, b.ID), "*.md"))
		if err != nil || len(names) == 0 {
			continue
		}
		f, s, _, err := assembleBook(root, b.ID, "en", false)
		if err != nil {
			fmt.Fprintf(os.Stderr, "audit: S09 does not cover %s, the assembler stopped on it: %v\n", b.ID, err)
			continue
		}
		for k, v := range f {
			files[k] = v
		}
		stale = append(stale, s...)
		ran++
	}
	if ran == 0 {
		return nil, nil, "no book assembles yet, so there is nothing to compare against"
	}
	return files, stale, ""
}

// listChecks prints the rules, which is what somebody reaches for before typing
// -only. The order is spec 08's, and not the order the rules happen to register
// in, which is alphabetical by file name and means nothing to anybody.
func listChecks() error {
	for _, g := range quality.GroupOrder() {
		fmt.Printf("\n%s\n", g)
		for _, c := range quality.Checks() {
			if c.Group != g {
				continue
			}
			strength := "soft"
			if c.Hard {
				strength = "hard"
			}
			fmt.Printf("  %-4s %-4s %s\n", c.ID, strength, c.Title)
		}
	}
	return nil
}

// headCommit is the corpus commit, for the JSON. It is not in the Markdown
// report, because that report is committed and checked by regenerating it, and
// a file cannot name the commit it is part of.
func headCommit(root string) string {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}

func split(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, p := range strings.Split(s, ",") {
		if p = strings.TrimSpace(p); p != "" {
			out = append(out, p)
		}
	}
	return out
}
