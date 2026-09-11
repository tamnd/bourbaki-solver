package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/quality"
)

const fixResealUsage = `usage: bourbaki fix reseal [flags] [path...]

Moves source_content_sha256 on to the hash its source has now, where the source
moved in a way the translation does not have to follow.

L05 is the rule that a translation records the hash of the English body it was
made from, and a translation whose record no longer matches is stale. That is
what makes translating four languages affordable, and it is also why a comma
repaired in the English costs an hour of the fleet's time: the hash moves, every
translation under it goes stale, and nothing but a re-translation puts it back.

Four repairs get this right already. fix prime, fix notin, fix label and fix
math go through repairContent, which hashes each source before and after and
moves the translations that recorded the first on to the second. Every other way
a source can move -- a hand edit, an assemble after a page repair, any repair
written without that machinery -- leaves the record behind with no command to
settle it. This is that command, and unlike repairContent it does not have to be
the thing that made the change.

It asks the same question after the fact, out of git. For each stale
translation it reads the history of the file named in translated_from, hashing
the body at each commit until it finds the one the translation recorded. That
body is the source as the translator saw it. The record is moved on only when

  quality.Prose(then) == quality.Prose(now)

that is, when the source says the same words and only the mathematics between
them has moved. The mathematics is not translated; a translator is told to copy
it, and the rules that check they did are L01 and L07, which read the files
themselves and are not touched by anything here.

Everything else is left alone and named:

  a source whose prose moved wants a translation and gets no seal from this
  a record no commit of its source ever hashed to is not recognised and is not
    moved, because a record this command cannot account for is exactly the one
    a sweep must not bless

So a file that was stale before the repair stays stale, which is what #164 and
L05 are for. This command can only ever remove staleness it can explain.

Named paths are the only ones considered. With none, every translation is read,
which is the whole corpus unless -lang narrows it.

flags:
  -lang L    only this language, default every language
  -depth N   how far back to read a source's history, default 500 commits
  -check     say what would move and move nothing
`

// resealable is one translation whose source_content_sha256 no longer matches
// its source, held with what is needed to decide and to write.
type resealable struct {
	path     string // absolute
	from     string // corpus-relative, the source it names
	recorded string // the hash it carries
	now      string // the hash its source hashes to now
	// write puts sha into source_content_sha256 and saves the file. It closes
	// over the parsed file so the second pass does not read it again, and
	// because a section and an exercise are two schemas with one field in
	// common and no type that holds both.
	write func(sha string) error
}

func fixReseal(args []string) error {
	fs := flag.NewFlagSet("fix reseal", flag.ExitOnError)
	fs.Usage = func() { fmt.Fprint(os.Stderr, fixResealUsage) }
	lang := fs.String("lang", "", "only this language")
	depth := fs.Int("depth", 500, "how far back to read a source's history")
	check := fs.Bool("check", false, "change nothing")
	named, err := parseFlags(fs, args)
	if err != nil {
		return err
	}
	root, err := corpus.Root()
	if err != nil {
		return err
	}

	// The paths bind, for fix seal's reason: a command that rewrites the field
	// telling a stale translation from a current one is the last one that should
	// do more than it was asked.
	only := map[string]bool{}
	for _, a := range named {
		p, err := filepath.Abs(a)
		if err != nil {
			return err
		}
		if _, err := os.Stat(p); err != nil {
			return err
		}
		only[filepath.Clean(p)] = true
	}
	seen := map[string]bool{}

	// The current body of every source asked for, read once. A § has a hundred
	// exercises under it in some volumes and they all name the same file.
	bodies := map[string]string{}
	sourceBody := func(from string) (string, error) {
		if b, ok := bodies[from]; ok {
			return b, nil
		}
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(from)))
		if err != nil {
			return "", err
		}
		_, body, err := corpus.SplitFrontMatter(raw)
		if err != nil {
			return "", fmt.Errorf("%s: %w", from, err)
		}
		bodies[from] = string(body)
		return bodies[from], nil
	}

	var read int
	var stale []resealable
	consider := func(path, from, recorded string, write func(string) error) error {
		if len(only) > 0 {
			if !only[filepath.Clean(path)] {
				return nil
			}
			seen[filepath.Clean(path)] = true
		}
		read++
		if from == "" || recorded == "" {
			return nil
		}
		body, err := sourceBody(from)
		if err != nil {
			// L05 already reports a translated_from that names nothing, and it
			// reports it better than a command that stops on the first one.
			return nil
		}
		now := corpus.ContentSHA256(body)
		if recorded == now {
			return nil
		}
		stale = append(stale, resealable{path: path, from: from, recorded: recorded, now: now, write: write})
		return nil
	}

	err = eachSection(root, *lang, func(path string, f *corpus.File[corpus.SectionFrontMatter]) error {
		return consider(path, f.Meta.TranslatedFrom, f.Meta.SourceSHA256, func(sha string) error {
			f.Meta.SourceSHA256 = sha
			return f.Write(path)
		})
	})
	if err != nil {
		return err
	}
	err = eachExercise(root, *lang, func(path string, f *corpus.File[corpus.ExerciseFrontMatter]) error {
		return consider(path, f.Meta.TranslatedFrom, f.Meta.SourceSHA256, func(sha string) error {
			f.Meta.SourceSHA256 = sha
			return f.Write(path)
		})
	})
	if err != nil {
		return err
	}
	// A named path the walk never offered is worth a word and not an error. It
	// is how fix seal refused a mixed list outright, which meant one exercise in
	// the arguments sealed nothing at all.
	var missed []string
	for p := range only {
		if !seen[p] {
			missed = append(missed, rel(root, p))
		}
	}
	if len(missed) > 0 {
		sort.Strings(missed)
		for _, m := range missed {
			fmt.Printf("%s  is not a translation this command reads, passed over\n", m)
		}
	}

	// One history walk per source, for every hash any translation of it records.
	want := map[string]map[string]bool{}
	for _, s := range stale {
		if want[s.from] == nil {
			want[s.from] = map[string]bool{}
		}
		want[s.from][s.recorded] = true
	}
	froms := make([]string, 0, len(want))
	for from := range want {
		froms = append(froms, from)
	}
	sort.Strings(froms)
	then := map[string]map[string]string{} // source -> recorded hash -> body as it was
	for _, from := range froms {
		found, err := bodyHistory(root, from, want[from], *depth)
		if err != nil {
			return err
		}
		then[from] = found
	}

	var moved, wordsMoved, unknown int
	for _, s := range stale {
		was, ok := then[s.from][s.recorded]
		if !ok {
			unknown++
			fmt.Printf("%s  records %s and no commit of %s hashes to it, so it is not moved\n",
				rel(root, s.path), short(s.recorded), s.from)
			continue
		}
		nowBody, err := sourceBody(s.from)
		if err != nil {
			return err
		}
		if quality.Prose(was) != quality.Prose(nowBody) {
			wordsMoved++
			fmt.Printf("%s  the words of %s moved, so it wants a translation and not a seal\n",
				rel(root, s.path), s.from)
			continue
		}
		moved++
		fmt.Printf("%s  %s is now %s, the mathematics of %s moved and its words did not\n",
			rel(root, s.path), short(s.recorded), short(s.now), s.from)
		if *check {
			continue
		}
		if err := s.write(s.now); err != nil {
			return err
		}
	}

	verb := "moved"
	if *check {
		verb = "would move"
	}
	fmt.Printf("fix reseal: %d translations read, %d stale, %s %d of them, %d want a translation, %d record a hash no history has\n",
		read, len(stale), verb, moved, wordsMoved, unknown)
	return nil
}

// bodyHistory walks the commits that touched one file, newest first, hashing the
// body at each until it has found every hash asked for. It returns the body as
// it stood at the commit that hashes to each.
//
// Newest first because a translation is usually one or two repairs behind and
// not fifty, so the answer is nearly always within a few commits and the walk
// stops there. -depth is the guard for the other case: a file with a long
// history and a record from before it that would otherwise be read in full to
// prove a negative.
//
// --all and not HEAD, which is the difference between finding the body and not.
// The corpus squash-merges its pull requests, so the commits of a branch are
// reachable from a ref and are not ancestors of main, and every body the branch
// held between its first commit and its last is off main's history entirely. The
// six notation indices were exactly that: the Vietnamese was translated from the
// English as it stood on the branch that first wrote it, main has only the
// squashed result, and asking main alone said no commit of the source ever
// hashed to what the translation records. It did; the commit is just not an
// ancestor of HEAD.
func bodyHistory(root, rel string, want map[string]bool, depth int) (map[string]string, error) {
	found := map[string]string{}
	if len(want) == 0 {
		return found, nil
	}
	out, err := exec.Command("git", "-C", root, "rev-list",
		"-n", strconv.Itoa(depth), "--all", "--", rel).Output()
	if err != nil {
		// A corpus that is not a git checkout, or a path git does not know. The
		// command still has something to say about every other file, so this is
		// not the end of the run.
		return found, nil
	}
	for _, sha := range strings.Fields(string(out)) {
		b, err := exec.Command("git", "-C", root, "show", sha+":"+rel).Output()
		if err != nil {
			continue // the file was not at that path in that commit
		}
		_, body, err := corpus.SplitFrontMatter(b)
		if err != nil {
			continue
		}
		h := corpus.ContentSHA256(string(body))
		if want[h] && found[h] == "" {
			found[h] = string(body)
			if len(found) == len(want) {
				break
			}
		}
	}
	return found, nil
}
