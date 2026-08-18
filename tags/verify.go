package tags

import (
	"fmt"
	"sort"
	"strings"
)

// The invariants of spec 01 §5.3. They are numbered here as they are numbered
// there, and the number is printed with the failure, so that a CI log says
// which part of the contract broke and not merely that something did.
const (
	T01 = "T01" // a tag is well formed and appears once in tags
	T02 = "T02" // a label appears at most once across tags and inactive
	T03 = "T03" // every statement in the corpus has exactly one tag
	T04 = "T04" // every tag in tags names a statement that is in the corpus
	T05 = "T05" // tags is only ever appended to
	T06 = "T06" // no tag is in both tags and inactive
	T07 = "T07" // a translation reuses the tag of the printing and never gets its own
	T09 = "T09" // every tag= in the Markdown is four of [0-9A-Z]; that it is also in tags is T03
	T10 = "T10" // the tags of a file climb, as they do on the run that assigned them
)

// T08 of spec 08 §2.2 is invariant T05 read off a diff rather than off the
// file, which is the only way it can be read, so AppendOnly answers to both
// names and reports the one the model spec gives it.

// A Failure is one broken invariant, said in one line.
type Failure struct {
	Rule string
	Msg  string
}

func (f Failure) String() string { return f.Rule + ": " + f.Msg }

// Verify checks everything that can be checked from the files alone, which is
// every invariant but T05. T05 is about the history of the file rather than its
// contents, so it is checked against git by the command.
//
// found is the corpus, per language, as Walk returns it. printings are the
// languages the Éléments is printed in and this corpus holds volumes of, which
// is where a statement is to be found and so what carries a tag; every other
// language in found is a translation and is held to T07. See BooksManifest.
// Printings, and spec 01 §5.3.
func Verify(s *Set, found map[string][]Item, printings []string) []Failure {
	var out []Failure
	out = append(out, s.check()...)
	out = append(out, checkCorpus(s, found, printings)...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].Rule != out[j].Rule {
			return out[i].Rule < out[j].Rule
		}
		return out[i].Msg < out[j].Msg
	})
	return out
}

// check is what the four files say about themselves.
func (s *Set) check() []Failure {
	var out []Failure
	seenTag := map[Tag]bool{}
	seenLabel := map[string]string{}
	for _, e := range append(append([]Entry(nil), s.Tags...), s.New...) {
		if _, err := Parse(string(e.Tag)); err != nil {
			out = append(out, Failure{T01, err.Error()})
		}
		if seenTag[e.Tag] {
			out = append(out, Failure{T01, fmt.Sprintf("the tag %s is on two lines", e.Tag)})
		}
		seenTag[e.Tag] = true
		if was, dup := seenLabel[e.Label]; dup {
			out = append(out, Failure{T02, fmt.Sprintf("the label %s is held by %s and by %s", e.Label, was, e.Tag)})
		}
		seenLabel[e.Label] = string(e.Tag)
	}
	for _, r := range s.Inactive {
		if seenTag[r.Tag] {
			out = append(out, Failure{T06, fmt.Sprintf("the tag %s is live and retired at once", r.Tag)})
		}
		if was, dup := seenLabel[r.Label]; dup {
			out = append(out, Failure{T02, fmt.Sprintf("the label %s is held by %s and retired as %s", r.Label, was, r.Tag)})
		}
		seenLabel[r.Label] = string(r.Tag)
	}
	return out
}

// checkCorpus is what the files say about the Markdown, and the Markdown about
// the files.
//
// The printings are read first and together. A statement printed in both of them
// is one statement of the Éléments with one tag, and a statement printed in only
// one is still a statement of the Éléments: Théories spectrales and Topologie
// algébrique are in this corpus in French alone, and holding them to an English
// printing that does not exist would leave 1382 statements with no permanent
// name and no way to get one.
func checkCorpus(s *Set, found map[string][]Item, printings []string) []Failure {
	var out []Failure
	byLabel := s.Lookup()
	printed := map[string]Tag{}
	isPrinting := map[string]bool{}
	for _, lang := range printings {
		isPrinting[lang] = true
	}
	for _, lang := range printings {
		for _, it := range found[lang] {
			if _, seen := printed[it.Label]; !seen {
				printed[it.Label] = it.Tag
			}
			switch {
			case it.Bad != "":
				out = append(out, Failure{T09, fmt.Sprintf("%s is tagged %q, which is not four of 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					at(it), it.Bad)})
			case it.Tag == "":
				out = append(out, Failure{T03, fmt.Sprintf("%s has no tag", at(it))})
			case byLabel[it.Label] == "":
				out = append(out, Failure{T03, fmt.Sprintf("%s carries the tag %s, which is in no file of tags/", at(it), it.Tag)})
			case byLabel[it.Label] != it.Tag:
				out = append(out, Failure{T03, fmt.Sprintf("%s carries the tag %s and tags/ gives it %s",
					at(it), it.Tag, byLabel[it.Label])})
			}
		}
	}
	if len(printed) > 0 {
		for _, e := range s.Tags {
			if _, ok := printed[e.Label]; !ok {
				out = append(out, Failure{T04, fmt.Sprintf("the tag %s names %s, which is in no file of the corpus", e.Tag, e.Label)})
			}
		}
	}
	for lang, items := range found {
		if isPrinting[lang] {
			continue
		}
		for _, it := range items {
			if it.Bad != "" {
				out = append(out, Failure{T09, fmt.Sprintf("%s is tagged %q, which is not four of 0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ",
					at(it), it.Bad)})
				continue
			}
			want, ok := printed[it.Label]
			if !ok {
				out = append(out, Failure{T07, fmt.Sprintf("%s translates %s, which no printing in the corpus has", at(it), it.Label)})
				continue
			}
			if it.Tag != want {
				out = append(out, Failure{T07, fmt.Sprintf("%s carries the tag %s and the English it translates carries %s",
					at(it), it.Tag, want)})
			}
		}
	}
	return out
}

// Order is T10, and it is the one check here that does not fail a build.
//
// Tags are handed out in reading order, so on the run that assigns them the
// tags of a file climb from top to bottom, and a file where they do not is
// worth a look: a heading hand-edited with somebody else's tag, or two
// statements that swapped places without their tags going with them.
//
// It cannot be a hard rule, though, and it is worth being plain about why. Add
// a proposition to the middle of § 3 tomorrow and it takes the next free tag,
// which is a larger number than everything after it in the file. The corpus is
// then correct and T10 is broken, and a rule a correct edit breaks is not a
// rule. So it is reported and counted and left at that.
//
// The runs are what keep it worth reading. A tag climbs against the tags its
// own run assigned and against nothing else, because a later run reads a file
// that already has tags on most of it and fills in what is new wherever it
// stands. Comparing across runs is the correct edit above, and it was the whole
// of what T10 reported: 236 findings, every one of them a volume read in August
// against a statement added to it later. What is left is the thing the rule was
// written for, two statements out of order inside one reading.
func Order(items []Item, runs []Run) []Failure {
	var out []Failure
	type where struct {
		path string
		run  int
	}
	last := map[where]Item{}
	for _, it := range items {
		if it.Tag == "" {
			continue
		}
		key := where{it.Path, RunAt(runs, it.Tag)}
		if was, seen := last[key]; seen && it.Tag < was.Tag {
			out = append(out, Failure{T10, fmt.Sprintf("%s has %s after %s, which the same run assigned later",
				at(it), it.Tag, was.Tag)})
		}
		last[key] = it
	}
	return out
}

func at(it Item) string {
	if it.Line == 0 {
		return it.Path
	}
	return fmt.Sprintf("%s:%d", it.Path, it.Line)
}

// AppendOnly reads a diff of tags and reports the lines it takes away, which is
// invariant T05.
//
// The diff is asked for with no context, so every line that starts with a minus
// and is not the file header is a line the change removes. Two things explain a
// removal and nothing else does.
//
// The first is an alias: the same tag on the same line with the label the alias
// says it became. That is what migrate does.
//
// The second is a retirement: the tag stands in inactive under the label it is
// leaving. That is what retire does, and it is what happens when a statement
// leaves the corpus for good, as the twelfth exercise of § 2 of chapter VIII of
// Lie did when the volume turned out to print eleven. The rule was written
// before anything had ever been retired and named only migrate, so the first
// retirement the corpus made was reported as a tag going missing.
//
// The comment block at the top is prose and not record. Rewording it is not
// taking a tag away from anything, so it is not what this rule is about.
func AppendOnly(diff string, aliases []Alias, retired []Retired) []Failure {
	renamed := map[string]string{}
	for _, a := range aliases {
		renamed[a.Old] = a.New
	}
	burned := map[string]string{}
	for _, r := range retired {
		burned[string(r.Tag)] = r.Label
	}
	added := map[string]bool{}
	var removed []string
	for _, line := range strings.Split(diff, "\n") {
		switch {
		case strings.HasPrefix(line, "+++") || strings.HasPrefix(line, "---"):
		case strings.HasPrefix(line, "+"):
			added[strings.TrimPrefix(line, "+")] = true
		case strings.HasPrefix(line, "-"):
			if body := strings.TrimPrefix(line, "-"); !strings.HasPrefix(body, "#") && body != "" {
				removed = append(removed, body)
			}
		}
	}
	var out []Failure
	for _, line := range removed {
		tag, label, ok := strings.Cut(line, ",")
		if ok {
			if to, isRename := renamed[label]; isRename && added[tag+","+to] {
				continue
			}
			if burned[tag] == label {
				continue
			}
		}
		out = append(out, Failure{T05, fmt.Sprintf("the line %q was taken out of tags, and only migrate may do that", line)})
	}
	return out
}
