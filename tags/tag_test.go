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
