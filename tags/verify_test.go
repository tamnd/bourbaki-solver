package tags

import (
	"strings"
	"testing"
)

func rules(fs []Failure) string {
	var out []string
	for _, f := range fs {
		out = append(out, f.Rule)
	}
	return strings.Join(out, " ")
}

func TestVerifyGreen(t *testing.T) {
	s := &Set{Tags: []Entry{{Tag: "0001", Label: "alg-viii-s1-def-1"}, {Tag: "0002", Label: "alg-viii-s1-ex-1"}}}
	found := map[string][]Item{
		"en": {
			{Path: "content/en/alg/VIII/01.md", Line: 40, Label: "alg-viii-s1-def-1", Tag: "0001"},
			{Path: "content/en/alg/VIII/exercises/s1/01.md", Label: "alg-viii-s1-ex-1", Tag: "0002"},
		},
		"vi": {
			{Path: "content/vi/alg/VIII/01.md", Line: 44, Label: "alg-viii-s1-def-1", Tag: "0001"},
		},
	}
	if bad := Verify(s, found); len(bad) != 0 {
		t.Errorf("a corpus that holds every invariant failed: %v", bad)
	}
}

func TestVerifyCatchesEachInvariant(t *testing.T) {
	cases := []struct {
		name  string
		set   *Set
		found map[string][]Item
		want  string
	}{{
		name: "T1 the same tag on two lines",
		set:  &Set{Tags: []Entry{{Tag: "0001", Label: "a"}, {Tag: "0001", Label: "b"}}},
		want: "T1",
	}, {
		name: "T2 the same label twice",
		set:  &Set{Tags: []Entry{{Tag: "0001", Label: "a"}, {Tag: "0002", Label: "a"}}},
		want: "T2",
	}, {
		name:  "T3 a statement with no tag",
		set:   &Set{Tags: []Entry{{Tag: "0001", Label: "a"}}},
		found: map[string][]Item{"en": {{Path: "f.md", Line: 3, Label: "a"}}},
		want:  "T3",
	}, {
		name:  "T3 a statement whose tag is in no file",
		set:   &Set{Tags: []Entry{{Tag: "0001", Label: "a"}}},
		found: map[string][]Item{"en": {{Path: "f.md", Line: 3, Label: "a", Tag: "0009"}}},
		want:  "T3",
	}, {
		name:  "T4 a tag whose statement is gone",
		set:   &Set{Tags: []Entry{{Tag: "0001", Label: "a"}, {Tag: "0002", Label: "b"}}},
		found: map[string][]Item{"en": {{Path: "f.md", Line: 3, Label: "a", Tag: "0001"}}},
		want:  "T4",
	}, {
		name: "T6 live and retired at once",
		set: &Set{Tags: []Entry{{Tag: "0001", Label: "a"}},
			Inactive: []Retired{{Tag: "0001", Label: "a"}}},
		want: "T2 T6",
	}, {
		name: "T7 a translation with a tag of its own",
		set:  &Set{Tags: []Entry{{Tag: "0001", Label: "a"}}},
		found: map[string][]Item{
			"en": {{Path: "en.md", Line: 3, Label: "a", Tag: "0001"}},
			"vi": {{Path: "vi.md", Line: 3, Label: "a", Tag: "0002"}},
		},
		want: "T7",
	}, {
		name: "T7 a translation of nothing",
		set:  &Set{Tags: []Entry{{Tag: "0001", Label: "a"}}},
		found: map[string][]Item{
			"en": {{Path: "en.md", Line: 3, Label: "a", Tag: "0001"}},
			"vi": {{Path: "vi.md", Line: 3, Label: "b", Tag: "0001"}},
		},
		want: "T7",
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rules(Verify(c.set, c.found)); got != c.want {
				t.Errorf("Verify reported %q, want %q", got, c.want)
			}
		})
	}
}

// The append-only rule is about the history of the file and not its contents,
// so it is read off a diff. The one removal that is allowed is the label
// rewrite migrate does, and only when an alias says so.
func TestAppendOnly(t *testing.T) {
	add := "--- a/tags/tags\n+++ b/tags/tags\n+0003,alg-viii-s2-prop-1\n"
	if bad := AppendOnly(add, nil); len(bad) != 0 {
		t.Errorf("an append was reported: %v", bad)
	}
	del := add + "-0001,alg-viii-s1-def-1\n"
	if bad := AppendOnly(del, nil); len(bad) != 1 || bad[0].Rule != T5 {
		t.Errorf("a removal was not reported: %v", bad)
	}
	rename := "--- a/tags/tags\n+++ b/tags/tags\n-0001,alg-viii-s1-rem-1\n+0001,alg-viii-s1-n1-rem-1\n"
	aliases := []Alias{{Old: "alg-viii-s1-rem-1", New: "alg-viii-s1-n1-rem-1"}}
	if bad := AppendOnly(rename, aliases); len(bad) != 0 {
		t.Errorf("a migration an alias justifies was reported: %v", bad)
	}
	if bad := AppendOnly(rename, nil); len(bad) != 1 {
		t.Errorf("a label rewrite with no alias behind it was allowed: %v", bad)
	}
	// The tag has to be the same one. Moving a label to another tag is two
	// statements swapping identity, which is the thing all of this prevents.
	swap := "--- a/tags/tags\n+++ b/tags/tags\n-0001,alg-viii-s1-rem-1\n+0002,alg-viii-s1-n1-rem-1\n"
	if bad := AppendOnly(swap, aliases); len(bad) != 1 {
		t.Errorf("a label that moved to another tag was allowed: %v", bad)
	}
}

// The comment block at the top of the file is prose and not record. Rewording
// it takes no tag away from anything.
func TestAppendOnlyIgnoresTheHeader(t *testing.T) {
	diff := "--- a/tags/tags\n+++ b/tags/tags\n-# the old wording\n+# the new wording\n"
	if bad := AppendOnly(diff, nil); len(bad) != 0 {
		t.Errorf("a reworded header was reported: %v", bad)
	}
}
