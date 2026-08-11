package tags

import (
	"fmt"
	"sort"
)

// Assign hands a tag to every label that does not have one, in the order the
// labels are given, and returns what it allocated.
//
// A label that already has a tag keeps it, and that is the whole contract: the
// second run over an unchanged corpus allocates nothing. What is allocated goes
// to new-tags rather than to tags, because a permanent identifier is not
// something a build should be able to mint unreviewed. merge is the deliberate
// act that makes it permanent.
// A label that turns up twice in the corpus stops the run rather than being
// quietly given one tag between the two. Two statements at one address is a
// mistake in assembly, and letting a permanent identifier be minted over the
// top of it would make the mistake permanent too.
func (s *Set) Assign(labels []string) ([]Entry, error) {
	seen := make(map[string]bool, len(labels))
	for _, label := range labels {
		if seen[label] {
			return nil, fmt.Errorf("the label %s is on two things in the corpus, and a tag names one", label)
		}
		seen[label] = true
	}
	known := s.Lookup()
	a := s.allocator()
	var out []Entry
	for _, label := range labels {
		if _, ok := known[label]; ok {
			continue
		}
		t, err := a.next()
		if err != nil {
			return nil, err
		}
		e := Entry{Tag: t, Label: label}
		known[label] = t
		s.New = append(s.New, e)
		out = append(out, e)
	}
	return out, nil
}

// allocator walks the tag space from the first tag upwards, handing out the
// ones nothing has taken.
//
// Everything in tags, new-tags and inactive is taken, inactive above all: a tag
// whose statement left the corpus is burned, because somewhere there is a
// citation or a bookmark pointing at it and handing it to a different statement
// would silently make that citation wrong. That is the one thing worse than a
// dead link.
func (s *Set) allocator() *alloc {
	used := map[int]bool{}
	for _, e := range s.Tags {
		used[e.Tag.Int()] = true
	}
	for _, e := range s.New {
		used[e.Tag.Int()] = true
	}
	for _, r := range s.Inactive {
		used[r.Tag.Int()] = true
	}
	return &alloc{used: used, at: 0}
}

type alloc struct {
	used map[int]bool
	at   int
}

func (a *alloc) next() (Tag, error) {
	for {
		a.at++
		if !a.used[a.at] {
			a.used[a.at] = true
			return FromInt(a.at)
		}
	}
}

// Merge moves new-tags into tags, which is the act that makes an allocation
// permanent.
func (s *Set) Merge() (int, error) {
	if len(s.New) == 0 {
		return 0, nil
	}
	seen := map[string]Tag{}
	for _, e := range s.Tags {
		seen[e.Label] = e.Tag
	}
	for _, e := range s.New {
		if t, dup := seen[e.Label]; dup {
			return 0, fmt.Errorf("new-tags gives %s to %s, which tags already holds as %s",
				e.Tag, e.Label, t)
		}
		seen[e.Label] = e.Tag
	}
	n := len(s.New)
	s.Tags = append(s.Tags, s.New...)
	sort.Slice(s.Tags, func(i, j int) bool { return s.Tags[i].Tag < s.Tags[j].Tag })
	s.New = nil
	return n, nil
}

// Retire moves a label's tag out of tags and into inactive, spending it for
// good. It is what happens when a statement leaves the corpus, and it is the
// only sanctioned way a line ever leaves tags.
func (s *Set) Retire(label, reason, date string) (Tag, error) {
	if reason == "" {
		return "", fmt.Errorf("retiring %s needs a reason: it is permanent", label)
	}
	if commas(reason) {
		return "", fmt.Errorf("the reason for retiring %s has a comma in it, and the file is comma separated", label)
	}
	for i, e := range s.Tags {
		if e.Label != label {
			continue
		}
		s.Tags = append(s.Tags[:i:i], s.Tags[i+1:]...)
		s.Inactive = append(s.Inactive, Retired{Tag: e.Tag, Label: label, Reason: reason, Date: date})
		sort.Slice(s.Inactive, func(i, j int) bool { return s.Inactive[i].Tag < s.Inactive[j].Tag })
		return e.Tag, nil
	}
	return "", fmt.Errorf("no tag holds the label %s", label)
}

// Migrate rewrites the label of a tags line to the label an alias says it
// became, keeping the tag. It is the one edit to tags that is not an append,
// and it exists because a statement that is renumbered is still the same
// statement and must keep its identifier.
func (s *Set) Migrate() []Alias {
	at := map[string]int{}
	for i, e := range s.Tags {
		at[e.Label] = i
	}
	var done []Alias
	for _, a := range s.Aliases {
		i, ok := at[a.Old]
		if !ok {
			continue
		}
		if _, taken := at[a.New]; taken {
			continue
		}
		s.Tags[i].Label = a.New
		delete(at, a.Old)
		at[a.New] = i
		done = append(done, a)
	}
	return done
}

func commas(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] == ',' {
			return true
		}
	}
	return false
}
