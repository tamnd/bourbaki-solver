package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/tags"
)

// tags is the permanent identifier of every statement and exercise in the
// corpus, taken whole from the Stacks Project. A tag is assigned once, never
// changes and is never reused, so that a citation, a translation, a solution
// and a URL all survive the book being renumbered under them.
//
// Allocation is deliberate and is never done by a build. assign puts what it
// allocated in tags/new-tags for review; merge is the act that makes it
// permanent; verify is what CI runs.

const tagsUsage = `usage: bourbaki tags <command> [arguments]

commands:
  assign    give a tag to every statement and exercise that has none
  merge     move tags/new-tags into tags/tags, which makes them permanent
  retire    take a tag out of use for good, for a statement that has left
  migrate   rewrite a tag's label to the one tags/aliases says it became
  verify    check the invariants, which is what CI runs
  list      print the tags, or look one up

`

func runTags(args []string) error {
	if len(args) == 0 {
		fmt.Fprint(os.Stderr, tagsUsage)
		os.Exit(2)
	}
	switch args[0] {
	case "assign":
		return runTagsAssign(args[1:])
	case "merge":
		return runTagsMerge(args[1:])
	case "retire":
		return runTagsRetire(args[1:])
	case "migrate":
		return runTagsMigrate(args[1:])
	case "verify":
		return runTagsVerify(args[1:])
	case "list":
		return runTagsList(args[1:])
	case "help", "-h", "--help":
		fmt.Fprint(os.Stderr, tagsUsage)
		return nil
	}
	return fmt.Errorf("unknown tags command %q", args[0])
}

// runTagsAssign walks the printings in book order and hands a tag to everything
// that has none.
//
// A printing is a language the corpus holds volumes of, and the tag names the
// statement of the Éléments rather than the printing it was read out of, so the
// two printings of Algebra VIII share every tag between them and a statement
// printed in only one of them is tagged off that one. Théories spectrales and
// Topologie algébrique are here in French alone and are tagged off the French;
// if the English of either ever arrives it will carry the same labels and so
// take the same tags, which is the whole point of a label being permanent.
//
// The English is read first, and not because it is worth more. Tags are handed
// out in the order the corpus is walked and they never move, so the order has to
// be the order they were first handed out in. See BooksManifest.Printings.
//
// Two things are written: the allocations, which go to tags/new-tags and wait
// there for a person, and the files themselves, since the tag lives in the
// Markdown so that a reader of the raw text can cite it. Assembly writes the
// same bytes from the same lookup, which is what keeps assemble -check green
// after an assignment.
func runTagsAssign(args []string) error {
	fs := flag.NewFlagSet("tags assign", flag.ExitOnError)
	dry := fs.Bool("dry-run", false, "print what would be allocated and write nothing")
	quiet := fs.Bool("q", false, "print only the totals")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	set, err := tags.Load(root)
	if err != nil {
		return err
	}
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return err
	}
	langs := books.Printings()
	// A label the corpus has twice in one printing is a mistake in assembly, and
	// Assign stops on it. Across printings it is not a mistake but the point:
	// Proposition 6 of § 1 is one statement whichever printing it is read in, so
	// the second printing to name it adds nothing to the list.
	var labels []string
	seen := map[string]bool{}
	total := 0
	for _, lang := range langs {
		items, err := tags.Walk(root, lang)
		if err != nil {
			return err
		}
		total += len(items)
		for _, it := range items {
			if seen[it.Label] {
				continue
			}
			seen[it.Label] = true
			labels = append(labels, it.Label)
		}
	}
	if total == 0 {
		return fmt.Errorf("no statement in content/: run bourbaki assemble first")
	}
	made, err := set.Assign(labels)
	if err != nil {
		return err
	}
	if !*quiet {
		for _, e := range made {
			fmt.Printf("%s %s\n", e.Tag, e.Label)
		}
	}
	if *dry {
		fmt.Printf("tags assign -dry-run: %d statements and exercises over %s, %d labels, %d would be allocated\n",
			total, strings.Join(langs, " and "), len(labels), len(made))
		return nil
	}
	if err := set.Save(root); err != nil {
		return err
	}
	n := 0
	for _, lang := range langs {
		w, err := writeTags(root, lang, set.Lookup())
		if err != nil {
			return err
		}
		n += w
	}
	fmt.Printf("tags assign: %d statements and exercises over %s, %d allocated to tags/new-tags, %d files rewritten\n",
		total, strings.Join(langs, " and "), len(made), n)
	if len(made) > 0 {
		fmt.Println("review tags/new-tags, then run bourbaki tags merge")
	}
	return nil
}

// writeTags puts the tags into the corpus and returns how many files changed.
//
// A section carries its tags in the statement headings and an exercise in its
// front matter. Both go back through the same writer the rest of the corpus
// uses, so content_sha256 is recomputed and a translation made from the old
// text is correctly reported stale.
func writeTags(root, lang string, tagOf map[string]tags.Tag) (int, error) {
	paths, err := contentFiles(root, lang)
	if err != nil {
		return 0, err
	}
	n := 0
	for _, path := range paths {
		var out []byte
		if isExercise(path) {
			f, err := corpus.ReadFile[corpus.ExerciseFrontMatter](path)
			if err != nil {
				return 0, err
			}
			f.Meta.Tag = string(tagOf[f.Meta.Label])
			if out, err = f.Bytes(); err != nil {
				return 0, err
			}
		} else {
			f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
			if err != nil {
				return 0, err
			}
			f.Body = tags.Apply(f.Body, tagOf)
			if out, err = f.Bytes(); err != nil {
				return 0, err
			}
		}
		have, err := os.ReadFile(path)
		if err != nil {
			return 0, err
		}
		if string(have) == string(out) {
			continue
		}
		if err := os.WriteFile(path, out, 0o644); err != nil {
			return 0, err
		}
		n++
	}
	return n, nil
}

func contentFiles(root, lang string) ([]string, error) {
	var out []string
	err := filepath.WalkDir(filepath.Join(root, "content", lang), func(path string, d os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(path, ".md") {
			out = append(out, path)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	sort.Strings(out)
	return out, nil
}

func isExercise(path string) bool {
	return strings.Contains(filepath.ToSlash(path), "/exercises/")
}

func runTagsMerge(args []string) error {
	fs := flag.NewFlagSet("tags merge", flag.ExitOnError)
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	set, err := tags.Load(root)
	if err != nil {
		return err
	}
	n, err := set.Merge()
	if err != nil {
		return err
	}
	if n == 0 {
		fmt.Println("tags merge: nothing in tags/new-tags")
		return nil
	}
	if err := set.Save(root); err != nil {
		return err
	}
	fmt.Printf("tags merge: %d tags are now permanent, %d in all\n", n, len(set.Tags))
	return nil
}

func runTagsRetire(args []string) error {
	fs := flag.NewFlagSet("tags retire", flag.ExitOnError)
	reason := fs.String("reason", "", "why the statement left the corpus, which is recorded for ever")
	date := fs.String("date", "", "the date to record, today by default")
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	if len(rest) != 1 {
		return fmt.Errorf("usage: bourbaki tags retire <label> -reason \"…\"")
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	set, err := tags.Load(root)
	if err != nil {
		return err
	}
	when := *date
	if when == "" {
		when = time.Now().UTC().Format("2006-01-02")
	}
	tag, err := set.Retire(rest[0], *reason, when)
	if err != nil {
		return err
	}
	if err := set.Save(root); err != nil {
		return err
	}
	if err := tags.Appendf(root, "%s retire %s %s: %s", when, tag, rest[0], *reason); err != nil {
		return err
	}
	fmt.Printf("tags retire: %s %s is burned and will never be assigned again\n", tag, rest[0])
	return nil
}

func runTagsMigrate(args []string) error {
	fs := flag.NewFlagSet("tags migrate", flag.ExitOnError)
	date := fs.String("date", "", "the date to record, today by default")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	set, err := tags.Load(root)
	if err != nil {
		return err
	}
	done := set.Migrate()
	if len(done) == 0 {
		fmt.Println("tags migrate: no alias names a label tags/tags still holds")
		return nil
	}
	if err := set.Save(root); err != nil {
		return err
	}
	when := *date
	if when == "" {
		when = time.Now().UTC().Format("2006-01-02")
	}
	for _, a := range done {
		if err := tags.Appendf(root, "%s migrate %s -> %s", when, a.Old, a.New); err != nil {
			return err
		}
		fmt.Printf("%s -> %s\n", a.Old, a.New)
	}
	fmt.Printf("tags migrate: %d labels rewritten, tags kept\n", len(done))
	return nil
}

// runTagsVerify is the audit gate. It reads the files, reads the corpus, and
// reads the history of tags/tags out of git, since being append-only is a fact
// about the history and not about the file.
func runTagsVerify(args []string) error {
	fs := flag.NewFlagSet("tags verify", flag.ExitOnError)
	base := fs.String("base", "origin/main", "the commit to check tags/tags was only appended to since")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	set, err := tags.Load(root)
	if err != nil {
		return err
	}
	found := map[string][]tags.Item{}
	langs, err := os.ReadDir(filepath.Join(root, "content"))
	if err != nil {
		return err
	}
	for _, l := range langs {
		if !l.IsDir() || l.Name() == "solutions" {
			continue
		}
		items, err := tags.Walk(root, l.Name())
		if err != nil {
			return err
		}
		found[l.Name()] = items
	}
	books, err := corpus.LoadBooks(root)
	if err != nil {
		return err
	}
	bad := tags.Verify(set, found, books.Printings())
	diff, gitErr := tagsDiff(root, *base)
	switch {
	case gitErr != nil:
		fmt.Printf("tags verify: T05 not checked, %v\n", gitErr)
	default:
		bad = append(bad, tags.AppendOnly(diff, set.Aliases)...)
	}
	if len(bad) > 0 {
		var lines []string
		for _, f := range bad {
			lines = append(lines, f.String())
		}
		return fmt.Errorf("tags verify: %d failures\n\t%s", len(bad), strings.Join(lines, "\n\t"))
	}
	n, seen := 0, make([]string, 0, len(found))
	var soft []tags.Failure
	for lang, items := range found {
		n += len(items)
		seen = append(seen, lang)
		soft = append(soft, tags.Order(items)...)
	}
	sort.Strings(seen)
	fmt.Printf("tags verify: %d tags over %d tagged units in %s, %d retired, all invariants hold\n",
		len(set.Tags)+len(set.New), n, strings.Join(seen, " "), len(set.Inactive))
	// T10 is a note and not a gate. A statement added to the middle of a § takes
	// the next free tag and its file stops climbing, which is correct and which
	// no build should be failed for.
	if len(soft) > 0 {
		where := "places"
		if len(soft) == 1 {
			where = "place"
		}
		fmt.Printf("tags verify: %s, %d %s where a file's tags do not climb\n", tags.T10, len(soft), where)
		for _, f := range soft[:min(len(soft), 5)] {
			fmt.Printf("\t%s\n", f.Msg)
		}
	}
	return nil
}

// tagsDiff asks git what this checkout did to tags/tags since base. A checkout
// with no such commit, which is what a fresh clone or a shallow CI checkout can
// be, is reported rather than passed silently.
func tagsDiff(root, base string) (string, error) {
	cmd := exec.Command("git", "diff", "--unified=0", base, "--", filepath.Join("tags", tags.TagsFile))
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("git diff %s: %s", base, strings.TrimSpace(string(out)))
	}
	return string(out), nil
}

func runTagsList(args []string) error {
	fs := flag.NewFlagSet("tags list", flag.ExitOnError)
	rest, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}
	set, err := tags.Load(root)
	if err != nil {
		return err
	}
	if len(rest) == 0 {
		for _, e := range set.Tags {
			fmt.Printf("%s %s\n", e.Tag, e.Label)
		}
		for _, e := range set.New {
			fmt.Printf("%s %s (new)\n", e.Tag, e.Label)
		}
		return nil
	}
	byTag := map[tags.Tag]string{}
	for _, e := range append(append([]tags.Entry(nil), set.Tags...), set.New...) {
		byTag[e.Tag] = e.Label
	}
	lookup := set.Lookup()
	for _, arg := range rest {
		if label, ok := byTag[tags.Tag(arg)]; ok {
			fmt.Printf("%s %s\n", arg, label)
			continue
		}
		if t, ok := lookup[arg]; ok {
			fmt.Printf("%s %s\n", t, arg)
			continue
		}
		for _, r := range set.Inactive {
			if string(r.Tag) == arg || r.Label == arg {
				fmt.Printf("%s %s retired %s: %s\n", r.Tag, r.Label, r.Date, r.Reason)
				return nil
			}
		}
		return fmt.Errorf("no tag and no label %q", arg)
	}
	return nil
}
