package main

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/publish"
)

func runPublish(args []string) error {
	fs := flag.NewFlagSet("publish", flag.ExitOnError)
	fs.Usage = func() {
		fmt.Fprint(os.Stderr, `usage: bourbaki publish [-out site] [-base /bourbaki] [-lang en,fr] [-clean]
                        [-check] [-drafts] [-max-broken n]

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
  -max-broken          the most refused formulae a build may carry before this
                       exits 1. It marks them rather than stopping at the first,
                       so one run names all of them. This is the pull request
                       gate while the chapters still carry the damage the text
                       layer did to them: a ceiling may only be lowered, never
                       raised, and the deploy does not use it. Use
                       -allow-broken-math to look at such a site.

                       A bare number is the ceiling for the whole corpus. A list
                       of volume=n, comma separated, is a ceiling apiece, and a
                       volume is named the way its files are: -max-broken
                       en/alg=17,en/lie=37,fr/alg=20. One number for everything
                       lets a volume that got worse hide behind a volume that
                       got better, which is the one thing this is here to catch,
                       so a list is what the workflow uses. A volume carrying a
                       refused formula and no ceiling of its own is an error,
                       since that is a volume nobody has measured yet.
`)
	}
	out := fs.String("out", "site", "directory to write")
	base := fs.String("base", "", "path the site is served under")
	lang := fs.String("lang", "", "build only these languages, comma separated")
	clean := fs.Bool("clean", false, "remove the output directory first")
	check := fs.Bool("check", false, "build in memory and write nothing")
	drafts := fs.Bool("drafts", false, "offer the languages under the coverage floor in the switcher")
	allowBroken := fs.Bool("allow-broken-math", false, "mark refused formulae instead of stopping")
	maxBroken := fs.String("max-broken", "", "the most refused formulae this build may carry")
	if _, err := parseFlags(fs, args); err != nil {
		return err
	}
	ceiling, err := parseCeilings(*maxBroken)
	if err != nil {
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
	// A ceiling is a promise about a count, and a count nobody finished taking
	// is not one. Stopping at the first refusal would give the number 1 on a
	// corpus that has two hundred of them, so the ceiling marks and counts.
	site.AllowBrokenMath = *allowBroken || ceiling.set()
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

	// Every page the build could not set, all of them at once. The build carries
	// on past one of these so that a run over twenty thousand files says
	// everything it found, rather than costing a ten minute pass per fault; this
	// is the other end of that, and it is what makes the run fail. It is
	// returned rather than printed alone so the exit code and the message agree.
	var unreadable error
	if n := len(site.Unreadable); n > 0 {
		var b strings.Builder
		fmt.Fprintf(&b, "publish: %d pages could not be built:", n)
		for _, u := range site.Unreadable {
			fmt.Fprintf(&b, "\n\t%s", u.Err)
		}
		unreadable = errors.New(b.String())
	}

	broken := map[string]int{}
	if n := len(site.Broken); n > 0 {
		// Stated and not judged. A formula marked broken is a fact about the
		// corpus that this run reports at every build; whether it is enough to
		// stop a build is the ceiling's to say, and saying "not publishable"
		// here contradicted the zero this exits with when every family is under
		// its ceiling.
		files := map[string]bool{}
		for _, ref := range site.Broken {
			file, _, _ := strings.Cut(ref.At, ":")
			files[file] = true
			broken[volumeOf(file)]++
		}
		fmt.Fprintf(os.Stderr, "publish: %d formulae are marked broken across %d files. "+
			"Run bourbaki audit -only P04 for the list.\n", n, len(files))
	}
	if ceiling.set() {
		// Said even at zero, because the run that clears the last one is the run
		// worth reading, and it is the run that says the ceiling can be dropped.
		for _, line := range ceiling.report(broken) {
			fmt.Printf("publish: %s\n", line)
		}
		if err := ceiling.check(broken); err != nil {
			return err
		}
	}
	if unreadable != nil {
		return unreadable
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
		case strings.HasPrefix(w, "reports/") && w != "reports/index.html":
			byKind["report pages"]++
		case w == "index.html", w == "tags/index.html", w == "search/index.html",
			w == "about/index.html", w == "reports/index.html", w == "style.css":
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

// ceilings is what -max-broken was given: how many formulae KaTeX may refuse
// before the build fails.
//
// One number for the whole corpus was enough while the corpus was one volume,
// and it stopped being enough the moment a second went in. A total lets a
// volume that got worse hide behind a volume that got better, and catching a
// formula that used to set and no longer does is the entire job of this gate,
// so the number that holds the line is one number per volume. The volumes are
// named the way the files are, en/alg and fr/alg and en/lie, because that is
// what a reader of the failure has in front of them.
//
// A ceiling of its own is also what lets a new volume in. The 204 the first
// chapter arrived with came down to 51 over five rounds of repair, and a volume
// that has had none of those rounds cannot enter under a number tuned by them.
// It enters at what it measures and comes down from there like the others did.
type ceilings struct {
	// total is the ceiling for everything at once, for -max-broken 51 and the
	// hand runs that still say it that way. -1 when the argument named volumes.
	total int
	each  map[string]int
}

func (c ceilings) set() bool { return c.total >= 0 || c.each != nil }

// parseCeilings reads the argument. A bare number is a total and anything else
// is a list of volume=n.
func parseCeilings(s string) (ceilings, error) {
	c := ceilings{total: -1}
	if s == "" {
		return c, nil
	}
	if n, err := strconv.Atoi(s); err == nil {
		if n < 0 {
			return c, fmt.Errorf("-max-broken %s: a ceiling is a count and counts are not negative", s)
		}
		c.total = n
		return c, nil
	}
	c.each = map[string]int{}
	for _, part := range strings.Split(s, ",") {
		name, count, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			return c, fmt.Errorf("-max-broken %s: %q is neither a number nor volume=n", s, part)
		}
		n, err := strconv.Atoi(count)
		if err != nil || n < 0 {
			return c, fmt.Errorf("-max-broken %s: %q does not name a count", s, part)
		}
		c.each[name] = n
	}
	return c, nil
}

// volumeOf names the volume a content file belongs to, which is its language
// and its book: content/en/lie/VIII/06_....md is en/lie. Anything shorter than
// that is not a content file and answers for itself, so a path this does not
// recognise still gets counted somewhere rather than quietly going missing.
func volumeOf(file string) string {
	parts := strings.Split(file, "/")
	if len(parts) >= 3 && parts[0] == "content" {
		return parts[1] + "/" + parts[2]
	}
	return file
}

// report is what the run prints about the ceilings it was given, one line per
// volume in the order a person would read them, and it is printed at zero as
// well: the run that clears the last refusal is the run that says a ceiling can
// come down.
func (c ceilings) report(broken map[string]int) []string {
	if c.each == nil {
		n := 0
		for _, v := range broken {
			n += v
		}
		return []string{fmt.Sprintf("%d formulae marked broken, ceiling %d", n, c.total)}
	}
	names := make([]string, 0, len(c.each))
	for name := range c.each {
		names = append(names, name)
	}
	for name := range broken {
		if _, ok := c.each[name]; !ok {
			names = append(names, name)
		}
	}
	sort.Strings(names)
	out := make([]string, 0, len(names))
	for _, name := range names {
		if limit, ok := c.each[name]; ok {
			out = append(out, fmt.Sprintf("%s\t%d formulae marked broken, ceiling %d", name, broken[name], limit))
		} else {
			out = append(out, fmt.Sprintf("%s\t%d formulae marked broken, no ceiling", name, broken[name]))
		}
	}
	return out
}

// check fails the build on a volume over its ceiling, and on a volume carrying
// a refused formula that has no ceiling at all. The second is not pedantry: a
// volume nobody has measured is a volume this gate is not watching, and it
// would go in silently under a rule whose whole promise is that nothing does.
func (c ceilings) check(broken map[string]int) error {
	if c.each == nil {
		n := 0
		for _, v := range broken {
			n += v
		}
		if n > c.total {
			return fmt.Errorf("%d formulae are marked broken, over the ceiling of %d. "+
				"A ceiling comes down as the pages are repaired and it does not go up", n, c.total)
		}
		return nil
	}
	names := make([]string, 0, len(broken))
	for name := range broken {
		names = append(names, name)
	}
	sort.Strings(names)
	var over []string
	for _, name := range names {
		limit, ok := c.each[name]
		switch {
		case !ok:
			over = append(over, fmt.Sprintf("%s has %d and no ceiling of its own, "+
				"so nothing here is watching it", name, broken[name]))
		case broken[name] > limit:
			over = append(over, fmt.Sprintf("%s has %d, over its ceiling of %d",
				name, broken[name], limit))
		}
	}
	if len(over) > 0 {
		return fmt.Errorf("formulae KaTeX refuses: %s. A ceiling comes down as the "+
			"pages are repaired and it does not go up", strings.Join(over, "; "))
	}
	return nil
}
