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
	if bad := Verify(s, found, []string{"en", "fr"}); len(bad) != 0 {
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
		name: "T01 the same tag on two lines",
		set:  &Set{Tags: []Entry{{Tag: "0001", Label: "a"}, {Tag: "0001", Label: "b"}}},
		want: "T01",
	}, {
		name: "T02 the same label twice",
		set:  &Set{Tags: []Entry{{Tag: "0001", Label: "a"}, {Tag: "0002", Label: "a"}}},
		want: "T02",
	}, {
		name:  "T03 a statement with no tag",
		set:   &Set{Tags: []Entry{{Tag: "0001", Label: "a"}}},
		found: map[string][]Item{"en": {{Path: "f.md", Line: 3, Label: "a"}}},
		want:  "T03",
	}, {
		name:  "T03 a statement whose tag is in no file",
		set:   &Set{Tags: []Entry{{Tag: "0001", Label: "a"}}},
		found: map[string][]Item{"en": {{Path: "f.md", Line: 3, Label: "a", Tag: "0009"}}},
		want:  "T03",
	}, {
		name:  "T04 a tag whose statement is gone",
		set:   &Set{Tags: []Entry{{Tag: "0001", Label: "a"}, {Tag: "0002", Label: "b"}}},
		found: map[string][]Item{"en": {{Path: "f.md", Line: 3, Label: "a", Tag: "0001"}}},
		want:  "T04",
	}, {
		name: "T06 live and retired at once",
		set: &Set{Tags: []Entry{{Tag: "0001", Label: "a"}},
			Inactive: []Retired{{Tag: "0001", Label: "a"}}},
		want: "T02 T06",
	}, {
		name: "T07 a translation with a tag of its own",
		set:  &Set{Tags: []Entry{{Tag: "0001", Label: "a"}}},
		found: map[string][]Item{
			"en": {{Path: "en.md", Line: 3, Label: "a", Tag: "0001"}},
			"vi": {{Path: "vi.md", Line: 3, Label: "a", Tag: "0002"}},
		},
		want: "T07",
	}, {
		name: "T07 a translation of nothing",
		set:  &Set{Tags: []Entry{{Tag: "0001", Label: "a"}}},
		found: map[string][]Item{
			"en": {{Path: "en.md", Line: 3, Label: "a", Tag: "0001"}},
			"vi": {{Path: "vi.md", Line: 3, Label: "b", Tag: "0001"}},
		},
		want: "T07",
	}, {
		// Théories spectrales and Topologie algébrique have never been printed
		// in English. Their statements are statements of the Éléments all the
		// same, so they are tagged off the only printing there is, and a rule
		// that asked them for an English original would leave a whole Book with
		// no permanent name.
		name: "a statement printed in French alone is tagged, not faulted",
		set:  &Set{Tags: []Entry{{Tag: "0001", Label: "ts-i-s1-prop-1"}}},
		found: map[string][]Item{
			"fr": {{Path: "fr.md", Line: 3, Label: "ts-i-s1-prop-1", Tag: "0001"}},
		},
		want: "",
	}, {
		// And a translation of it reuses that tag, exactly as a translation of
		// an English statement reuses the English one.
		name: "a translation reuses the tag of the printing it was made from",
		set:  &Set{Tags: []Entry{{Tag: "0001", Label: "ts-i-s1-prop-1"}}},
		found: map[string][]Item{
			"fr": {{Path: "fr.md", Line: 3, Label: "ts-i-s1-prop-1", Tag: "0001"}},
			"vi": {{Path: "vi.md", Line: 3, Label: "ts-i-s1-prop-1", Tag: "0002"}},
		},
		want: "T07",
	}, {
		name:  "T09 a hand-written tag in the wrong case",
		set:   &Set{Tags: []Entry{{Tag: "0001", Label: "a"}}},
		found: map[string][]Item{"en": {{Path: "f.md", Line: 3, Label: "a", Bad: "000a"}}},
		want:  "T09",
	}, {
		name: "T09 in a translation",
		set:  &Set{Tags: []Entry{{Tag: "0001", Label: "a"}}},
		found: map[string][]Item{
			"en": {{Path: "en.md", Line: 3, Label: "a", Tag: "0001"}},
			"vi": {{Path: "vi.md", Line: 3, Label: "a", Bad: "0O1"}},
		},
		want: "T09",
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := rules(Verify(c.set, c.found, []string{"en", "fr"})); got != c.want {
				t.Errorf("Verify reported %q, want %q", got, c.want)
			}
		})
	}
}

// T10 is the soft one. It has to hold on the run that assigns the tags, since
// they are handed out in reading order, and it stops holding the first time a
// statement is added to the middle of a §, which is why it reports and does
// not fail.
func TestOrder(t *testing.T) {
	climbing := []Item{
		{Path: "s1.md", Line: 10, Tag: "0001"},
		{Path: "s1.md", Line: 20, Tag: "0002"},
		{Path: "s2.md", Line: 10, Tag: "0003"},
	}
	if bad := Order(climbing); len(bad) != 0 {
		t.Errorf("a file whose tags climb was reported: %v", bad)
	}
	// A tag from an earlier run sitting below a later one, which is what a
	// heading edited by hand with somebody else's tag looks like.
	swapped := []Item{
		{Path: "s1.md", Line: 10, Tag: "0009"},
		{Path: "s1.md", Line: 20, Tag: "0002"},
	}
	if bad := Order(swapped); len(bad) != 1 || bad[0].Rule != T10 {
		t.Errorf("a file whose tags fall was reported as %v", bad)
	}
	// Two files are not compared with each other. Section 2 being assigned
	// before section 1 in some later run is nobody's business but the record's.
	across := []Item{
		{Path: "s1.md", Line: 10, Tag: "0009"},
		{Path: "s2.md", Line: 10, Tag: "0002"},
	}
	if bad := Order(across); len(bad) != 0 {
		t.Errorf("two different files were compared: %v", bad)
	}
	// A statement with no tag yet is not out of order, it is unassigned.
	none := []Item{{Path: "s1.md", Line: 10, Tag: "0009"}, {Path: "s1.md", Line: 20}}
	if bad := Order(none); len(bad) != 0 {
		t.Errorf("an unassigned statement was reported: %v", bad)
	}
}

// The append-only rule is about the history of the file and not its contents,
// so it is read off a diff. The one removal that is allowed is the label
// rewrite migrate does, and only when an alias says so.
func TestAppendOnly(t *testing.T) {
	add := "--- a/tags/tags\n+++ b/tags/tags\n+0003,alg-viii-s2-prop-1\n"
	if bad := AppendOnly(add, nil, nil); len(bad) != 0 {
		t.Errorf("an append was reported: %v", bad)
	}
	del := add + "-0001,alg-viii-s1-def-1\n"
	if bad := AppendOnly(del, nil, nil); len(bad) != 1 || bad[0].Rule != T05 {
		t.Errorf("a removal was not reported: %v", bad)
	}
	rename := "--- a/tags/tags\n+++ b/tags/tags\n-0001,alg-viii-s1-rem-1\n+0001,alg-viii-s1-n1-rem-1\n"
	aliases := []Alias{{Old: "alg-viii-s1-rem-1", New: "alg-viii-s1-n1-rem-1"}}
	if bad := AppendOnly(rename, aliases, nil); len(bad) != 0 {
		t.Errorf("a migration an alias justifies was reported: %v", bad)
	}
	if bad := AppendOnly(rename, nil, nil); len(bad) != 1 {
		t.Errorf("a label rewrite with no alias behind it was allowed: %v", bad)
	}
	// The tag has to be the same one. Moving a label to another tag is two
	// statements swapping identity, which is the thing all of this prevents.
	swap := "--- a/tags/tags\n+++ b/tags/tags\n-0001,alg-viii-s1-rem-1\n+0002,alg-viii-s1-n1-rem-1\n"
	if bad := AppendOnly(swap, aliases, nil); len(bad) != 1 {
		t.Errorf("a label that moved to another tag was allowed: %v", bad)
	}
}

// A retirement takes the line out of tags and puts the tag in inactive, which
// is what happens when a statement leaves the corpus. The rule named only
// migrate until the corpus retired its first tag.
func TestAppendOnlyAllowsARetirement(t *testing.T) {
	diff := "--- a/tags/tags\n+++ b/tags/tags\n-00ZN,lie-viii-s2-ex-12\n"
	gone := []Retired{{Tag: "00ZN", Label: "lie-viii-s2-ex-12",
		Reason: "the section prints eleven exercises", Date: "2026-08-16"}}
	if bad := AppendOnly(diff, nil, gone); len(bad) != 0 {
		t.Errorf("a retirement was reported: %v", bad)
	}
	// The tag has to be burned under the label it is leaving. A tag retired
	// somewhere else does not explain this line going.
	other := []Retired{{Tag: "00ZN", Label: "lie-viii-s3-ex-1", Reason: "x", Date: "2026-08-16"}}
	if bad := AppendOnly(diff, nil, other); len(bad) != 1 || bad[0].Rule != T05 {
		t.Errorf("a removal no retirement explains was allowed: %v", bad)
	}
}

// The comment block at the top of the file is prose and not record. Rewording
// it takes no tag away from anything.
func TestAppendOnlyIgnoresTheHeader(t *testing.T) {
	diff := "--- a/tags/tags\n+++ b/tags/tags\n-# the old wording\n+# the new wording\n"
	if bad := AppendOnly(diff, nil, nil); len(bad) != 0 {
		t.Errorf("a reworded header was reported: %v", bad)
	}
}
