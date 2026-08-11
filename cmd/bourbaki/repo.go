package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// repo is the commands about the checkout rather than about the books in it.

const repoUsage = `usage: bourbaki repo <command> [arguments]

Commands about the corpus checkout itself.

commands:
  init-hooks   point git at the hooks the corpus keeps in .githooks

Run bourbaki repo <command> -h for the flags of a command.
`

const initHooksUsage = `usage: bourbaki repo init-hooks [flags]

Points the corpus checkout's git at .githooks, which holds the pre-commit hook
that runs the hygiene rules before a commit rather than after a push.

Git does not track .git/hooks and never has, so a hook committed to a repository
does nothing until every checkout is told where to look. That is one git config
away and nobody remembers it, which is what this is for.

It is safe to run twice, and it will not overwrite a hooks path somebody has
already set to something else.

flags:
  -corpus DIR   the checkout, default $BOURBAKI_CORPUS
  -force        set the hooks path even if it already points somewhere else
`

func runRepo(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, repoUsage)
		os.Exit(2)
	}
	switch args[0] {
	case "init-hooks":
		return repoInitHooks(args[1:])
	}
	fmt.Fprint(os.Stderr, repoUsage)
	os.Exit(2)
	return nil
}

// hooksDir is where the corpus keeps its hooks, relative to the checkout. The
// path is committed to git config as a relative one so that it holds wherever
// the checkout is.
const hooksDir = ".githooks"

func repoInitHooks(args []string) error {
	fs := flag.NewFlagSet("repo init-hooks", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, initHooksUsage) }
	dir := fs.String("corpus", "", "the checkout")
	force := fs.Bool("force", false, "set the hooks path even if it is already set")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root := *dir
	if root == "" {
		var err error
		if root, err = corpus.Root(); err != nil {
			return err
		}
	}
	if _, err := os.Stat(filepath.Join(root, ".git")); err != nil {
		return fmt.Errorf("%s is not a git checkout", root)
	}
	if _, err := os.Stat(filepath.Join(root, hooksDir)); err != nil {
		return fmt.Errorf("%s has no %s to point at", root, hooksDir)
	}

	if cur := gitConfig(root, "core.hooksPath"); cur != "" && cur != hooksDir && !*force {
		return fmt.Errorf("core.hooksPath is already %s; run with -force to change it", cur)
	} else if cur == hooksDir {
		fmt.Printf("repo init-hooks: core.hooksPath is already %s, nothing to do\n", hooksDir)
		return nil
	}

	cmd := exec.Command("git", "config", "core.hooksPath", hooksDir)
	cmd.Dir = root
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("git config core.hooksPath: %v: %s", err, strings.TrimSpace(string(out)))
	}
	// The mode is set here rather than trusted, because a hook that is not
	// executable is one git skips without a word, which looks exactly like a
	// hook that ran and found nothing.
	hook := filepath.Join(root, hooksDir, "pre-commit")
	if fi, err := os.Stat(hook); err == nil && fi.Mode()&0o111 == 0 {
		if err := os.Chmod(hook, fi.Mode()|0o755); err != nil {
			return err
		}
		fmt.Printf("repo init-hooks: made %s executable\n", rel(root, hook))
	}
	fmt.Printf("repo init-hooks: core.hooksPath is %s, %s runs before every commit\n",
		hooksDir, rel(root, hook))
	return nil
}

// gitConfig is one config value, or the empty string when it is not set.
func gitConfig(root, key string) string {
	cmd := exec.Command("git", "config", "--get", key)
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return strings.TrimSpace(string(out))
}
