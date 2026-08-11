package glossary

import (
	"path/filepath"
	"testing"
)

func TestAMentionIsAWordAndNotARunOfLetters(t *testing.T) {
	cases := []struct {
		text, term string
		want       bool
	}{
		{"let a be a ring", "ring", true},
		{"the ring-theoretic argument", "ring", true},
		{"a ring, and nothing else", "ring", true},
		{"ring", "ring", true},
		{"the string is long", "ring", false},
		{"the ringed space", "ring", false},
		{"a semisimple ring", "semisimple ring", true},
		// The byte test called the second byte of đ a boundary, and then found
		// vành inside vànhđ. Vietnamese is the language this scanner is held
		// against, so a boundary test that is wrong on it is worth a case.
		{"một vành", "vành", true},
		{"một vànhđ", "vành", false},
		{"vành nửa đơn", "vành", true},
	}
	for _, c := range cases {
		if got := Mentions(c.text, c.term); got != c.want {
			t.Errorf("Mentions(%q, %q) = %v, want %v", c.text, c.term, got, c.want)
		}
	}
}

// The masking. One occurrence of the phrase is the phrase and not also the word
// inside it, which is the whole reason the scan goes longest first.
func TestThePhraseClaimsTheWordInsideIt(t *testing.T) {
	g := Glossary{Terms: []Term{
		{EN: "ring", VI: "vành"},
		{EN: "semisimple ring", VI: "vành nửa đơn"},
		{EN: "field", VI: "trường"},
	}}
	got := g.Mentioned("vi", "every semisimple ring over a field")
	if len(got) != 2 {
		t.Fatalf("got %d terms, want 2: %v", len(got), names(got))
	}
	seen := map[string]bool{}
	for _, t := range got {
		seen[t.EN] = true
	}
	if !seen["semisimple ring"] || !seen["field"] {
		t.Errorf("got %v, want semisimple ring and field", names(got))
	}
	if seen["ring"] {
		t.Error("the word inside the phrase was reported as a second term")
	}
	// The same word standing on its own is a mention of its own.
	if len(g.Mentioned("vi", "every semisimple ring is a ring")) != 2 {
		t.Error("a second, free standing occurrence was not claimed")
	}
}

// A term with no rendering in this language is not a term this language is held
// to. The Vietnamese pass runs first, so most of the glossary is in exactly
// that state for as long as Chinese and Japanese are unstarted.
func TestATermWithNoRenderingIsNotHeldTo(t *testing.T) {
	g := Glossary{Terms: []Term{{EN: "ring", VI: "vành"}}}
	if got := g.Mentioned("zh", "let a be a ring"); len(got) != 0 {
		t.Errorf("got %v, want nothing: the row has no Chinese", names(got))
	}
}

// Chinese and Japanese are written without spaces, so every neighbour of a term
// is a letter and the word boundary test would refuse a correct page.
func TestFollowsReadsChineseAsASubstring(t *testing.T) {
	if !Follows("zh", "交换环是一个环", "环") {
		t.Error("a Chinese rendering inside a compound was not found")
	}
	if Follows("vi", "một thể", "trường") {
		t.Error("a Vietnamese rendering that is not there was found")
	}
	if !Follows("vi", "một vành nửa đơn", "vành nửa đơn") {
		t.Error("a multi word Vietnamese rendering was not found")
	}
}

func names(ts []Term) []string {
	out := make([]string, len(ts))
	for i, t := range ts {
		out[i] = t.EN
	}
	return out
}

// The version is what makes a translated file stale, and it used to be a number
// somebody had to remember to raise. Save is what raises it, so these are the
// three cases it has to get right.
func TestSaveMovesTheVersionWhenTheTermsChange(t *testing.T) {
	path := filepath.Join(t.TempDir(), "manifests", "glossary.yaml")

	g := &Glossary{Terms: []Term{{EN: "ring", VI: "vành"}}}
	v, bumped, err := g.Save(path)
	if err != nil {
		t.Fatal(err)
	}
	if v != 1 || bumped {
		t.Fatalf("a new glossary saved as version %d, bumped %v, want 1 and false", v, bumped)
	}

	// A note is written for a person and never reaches a model, so editing one
	// must not restale the corpus.
	g.Terms[0].Note = "a ring is associative with a unit unless it says otherwise"
	if v, bumped, err = g.Save(path); err != nil {
		t.Fatal(err)
	}
	if v != 1 || bumped {
		t.Errorf("a note moved the version to %d, and a note changes no prompt", v)
	}

	// A rendering does reach a model.
	g.Terms[0].VI = "vòng"
	if v, bumped, err = g.Save(path); err != nil {
		t.Fatal(err)
	}
	if v != 2 || !bumped {
		t.Errorf("a changed rendering left the version at %d, want 2", v)
	}

	// So does a row going away, which is what glossary tidy does.
	g.Terms = nil
	g.Terms = append(g.Terms, Term{EN: "field", VI: "trường"})
	if v, _, err = g.Save(path); err != nil {
		t.Fatal(err)
	}
	if v != 3 {
		t.Errorf("a row swapped left the version at %d, want 3", v)
	}

	// And re-ordering does not, because the file order is nobody's business.
	g.Terms = append(g.Terms, Term{EN: "ring", VI: "vòng"})
	if v, _, err = g.Save(path); err != nil {
		t.Fatal(err)
	}
	g.Terms[0], g.Terms[1] = g.Terms[1], g.Terms[0]
	if v, bumped, err = g.Save(path); err != nil {
		t.Fatal(err)
	}
	if bumped {
		t.Errorf("re-ordering the file moved the version to %d", v)
	}
}
