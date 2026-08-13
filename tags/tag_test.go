package tags

import "testing"

func TestFromInt(t *testing.T) {
	cases := []struct {
		n    int
		want Tag
	}{
		{1, "0001"},
		{35, "000Z"},
		{36, "0010"},
		{36*36 - 1, "00ZZ"},
		{1679615, "ZZZZ"},
	}
	for _, c := range cases {
		got, err := FromInt(c.n)
		if err != nil {
			t.Fatalf("FromInt(%d): %v", c.n, err)
		}
		if got != c.want {
			t.Errorf("FromInt(%d) = %s, want %s", c.n, got, c.want)
		}
		if back := got.Int(); back != c.n {
			t.Errorf("%s.Int() = %d, want %d", got, back, c.n)
		}
	}
	if _, err := FromInt(0); err == nil {
		t.Error("0000 is reserved and FromInt handed it out")
	}
	if _, err := FromInt(1679616); err == nil {
		t.Error("a tag past the end of the space was made")
	}
}

// A tag is copied by hand between a file, a URL and a citation, so exactly one
// spelling of it is valid. Folding case here would make two strings that look
// like the same tag and do not compare equal anywhere else.
func TestParse(t *testing.T) {
	if _, err := Parse("0A3F"); err != nil {
		t.Errorf("a good tag was refused: %v", err)
	}
	for _, bad := range []string{"", "0A3", "0A3FF", "0a3f", "0A3-", "0000"} {
		if _, err := Parse(bad); err == nil {
			t.Errorf("Parse(%q) was accepted", bad)
		}
	}
}

// The prompts hand a model the shape of the tag line, and the tags they write
// in it are held back from the allocator so that copying the shape can never
// produce a citation to a real result.
func TestTheSampleTagsAreNeverAssigned(t *testing.T) {
	for _, s := range []Tag{Reserved, SampleA, SampleB} {
		if _, err := Parse(string(s)); err == nil {
			t.Errorf("%s parsed as a tag that could be on a statement", s)
		}
		if got, err := FromInt(s.Int()); err == nil {
			t.Errorf("the allocator handed out %s at number %d", got, s.Int())
		}
	}
	// And they are still the shape of a tag, which is what makes a copied
	// sample something the solve guard can see and complain about rather than
	// something that reads as no citation at all.
	for _, s := range []Tag{SampleA, SampleB} {
		if len(s) != Width {
			t.Errorf("%s is %d characters, and a sample that is not tag shaped "+
				"is a sample a model will not copy in the shape being taught", s, len(s))
		}
	}
}
