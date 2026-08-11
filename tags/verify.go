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
	T1 = "T1" // a tag is well formed and appears once in tags
	T2 = "T2" // a label appears at most once across tags and inactive
	T3 = "T3" // every statement in the corpus has exactly one tag
	T4 = "T4" // every tag in tags names a statement that is in the corpus
	T5 = "T5" // tags is only ever appended to
	T6 = "T6" // no tag is in both tags and inactive
	T7 = "T7" // a translation reuses the English tag and never gets its own
)

// A Failure is one broken invariant, said in one line.
type Failure struct {
	Rule string
	Msg  string
}

func (f Failure) String() string { return f.Rule + ": " + f.Msg }

// Verify checks everything that can be checked from the files alone, which is
// every invariant but T5. T5 is about the history of the file rather than its
// contents, so it is checked against git by the command.
//
// found is the corpus, per language, as Walk returns it. English is the corpus
// proper; every other language is a translation and is held to T7.
func Verify(s *Set, found map[string][]Item) []Failure {
	var out []Failure
	out = append(out, s.check()...)
	out = append(out, checkCorpus(s, found)...)
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
			out = append(out, Failure{T1, err.Error()})
		}
		if seenTag[e.Tag] {
			out = append(out, Failure{T1, fmt.Sprintf("the tag %s is on two lines", e.Tag)})
		}
		seenTag[e.Tag] = true
		if was, dup := seenLabel[e.Label]; dup {
			out = append(out, Failure{T2, fmt.Sprintf("the label %s is held by %s and by %s", e.Label, was, e.Tag)})
		}
		seenLabel[e.Label] = string(e.Tag)
	}
	for _, r := range s.Inactive {
		if seenTag[r.Tag] {
			out = append(out, Failure{T6, fmt.Sprintf("the tag %s is live and retired at once", r.Tag)})
		}
		if was, dup := seenLabel[r.Label]; dup {
			out = append(out, Failure{T2, fmt.Sprintf("the label %s is held by %s and retired as %s", r.Label, was, r.Tag)})
		}
		seenLabel[r.Label] = string(r.Tag)
	}
	return out
}

// checkCorpus is what the files say about the Markdown, and the Markdown about
// the files.
func checkCorpus(s *Set, found map[string][]Item) []Failure {
	var out []Failure
	byLabel := s.Lookup()
	english := map[string]Tag{}
	for _, it := range found["en"] {
		english[it.Label] = it.Tag
		switch {
		case it.Tag == "":
			out = append(out, Failure{T3, fmt.Sprintf("%s has no tag", at(it))})
		case byLabel[it.Label] == "":
			out = append(out, Failure{T3, fmt.Sprintf("%s carries the tag %s, which is in no file of tags/", at(it), it.Tag)})
		case byLabel[it.Label] != it.Tag:
			out = append(out, Failure{T3, fmt.Sprintf("%s carries the tag %s and tags/ gives it %s",
				at(it), it.Tag, byLabel[it.Label])})
		}
	}
	if len(found["en"]) > 0 {
		for _, e := range s.Tags {
			if _, ok := english[e.Label]; !ok {
				out = append(out, Failure{T4, fmt.Sprintf("the tag %s names %s, which is in no file of the corpus", e.Tag, e.Label)})
			}
		}
	}
	for lang, items := range found {
		if lang == "en" {
			continue
		}
		for _, it := range items {
			want, ok := english[it.Label]
			if !ok {
				out = append(out, Failure{T7, fmt.Sprintf("%s translates %s, which the English corpus does not have", at(it), it.Label)})
				continue
			}
			if it.Tag != want {
				out = append(out, Failure{T7, fmt.Sprintf("%s carries the tag %s and the English it translates carries %s",
					at(it), it.Tag, want)})
			}
		}
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
// invariant T5.
//
// The diff is asked for with no context, so every line that starts with a minus
// and is not the file header is a line the change removes. A removal is allowed
// only when an alias explains it: the same tag on the same line with the label
// the alias says it became. That is what migrate does and it is the one edit to
// tags that is not an append.
//
// The comment block at the top is prose and not record. Rewording it is not
// taking a tag away from anything, so it is not what this rule is about.
func AppendOnly(diff string, aliases []Alias) []Failure {
	renamed := map[string]string{}
	for _, a := range aliases {
		renamed[a.Old] = a.New
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
		}
		out = append(out, Failure{T5, fmt.Sprintf("the line %q was taken out of tags, and only migrate may do that", line)})
	}
	return out
}
