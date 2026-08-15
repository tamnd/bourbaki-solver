package glossary

import (
	"strings"
	"testing"
)

// The bodies here are written for the test and none of them is Bourbaki. They
// are the shapes the volume comes out in, set with invented terms and invented
// claims, because the corpus this mines is under copyright.
//
// Every case is a fault the miner actually had, or a rule it actually has. The
// comments say which, and what it was measured against.

// find is one candidate by its English, and whether it is there at all.
func find(cands []Candidate, en string) (Candidate, bool) {
	for _, c := range cands {
		if Key(c.EN) == Key(en) {
			return c, true
		}
	}
	return Candidate{}, false
}

func mine(t *testing.T, docs []Doc, opt Options) []Candidate {
	t.Helper()
	return Extract(docs, opt)
}

// The fault that cost the first run of this 72 candidates. A single letter was
// dropped rather than treated as a break, so "GRUB, p. 31, Proposition 4" mined
// the phrase "grub proposition" 149 times over the real corpus, which is a
// citation and not a term.
func TestASingleLetterBreaksAPhrase(t *testing.T) {
	body := strings.Repeat("See ABC, p. 31, Proposition 4 for this.\n", 5)
	got := mine(t, []Doc{{Path: "a.md", Body: body}}, Options{MinCount: 2, MaxWords: 4})
	if c, ok := find(got, "abc proposition"); ok {
		t.Errorf("the citation was mined as a term %d times: %+v", c.Count, c)
	}
}

// The same for a chapter numeral, which is a word by every other test here and
// terminology by none.
func TestARomanNumeralIsNotATerm(t *testing.T) {
	body := strings.Repeat("This holds by VII, Theorem 2, and by VII again.\n", 5)
	got := mine(t, []Doc{{Path: "a.md", Body: body}}, Options{MinCount: 2, MaxWords: 4})
	for _, bad := range []string{"vii", "vii theorem"} {
		if _, ok := find(got, bad); ok {
			t.Errorf("%q was offered as a term", bad)
		}
	}
}

// A phrase that only ever occurs inside a longer one is a fragment of that
// phrase and not a term the corpus uses.
func TestAFragmentIsDropped(t *testing.T) {
	body := strings.Repeat("Every wobbly quiver is a quiver of finite kind.\n", 6)
	got := mine(t, []Doc{{Path: "a.md", Body: body}}, Options{MinCount: 3, MaxWords: 4})
	if _, ok := find(got, "wobbly"); ok {
		t.Errorf("wobbly never occurs alone and was offered on its own")
	}
	if _, ok := find(got, "wobbly quiver"); !ok {
		t.Errorf("the phrase itself was not offered: %v", ens(got))
	}
	// quiver does occur alone, twice out of eighteen, so it is a term.
	if _, ok := find(got, "quiver"); !ok {
		t.Errorf("quiver occurs on its own and was dropped as a fragment")
	}
}

// One term and one glossary row. A curator should not have to decide the same
// thing twice because the corpus writes it both ways.
func TestAPluralFoldsOntoItsSingular(t *testing.T) {
	body := strings.Repeat("A left quiver is flat. The left quivers are flat.\n", 4)
	got := mine(t, []Doc{{Path: "a.md", Body: body}}, Options{MinCount: 2, MaxWords: 4})
	if _, ok := find(got, "left quivers"); ok {
		t.Errorf("the plural was offered as its own term")
	}
	c, ok := find(got, "left quiver")
	if !ok {
		t.Fatalf("the singular is not there either: %v", ens(got))
	}
	if c.Count != 8 {
		t.Errorf("the merged count is %d, want 8: four of each", c.Count)
	}
}

// A plural with no singular in the corpus keeps its own spelling. This is what
// stops the fold from turning "basis" into "basi", and it is why the fold is
// conditioned on the singular being a mined term rather than on the ending.
func TestAWordEndingInSIsNotAPlural(t *testing.T) {
	body := strings.Repeat("The basis is orthogonal and the basis is finite.\n", 4)
	got := mine(t, []Doc{{Path: "a.md", Body: body}}, Options{MinCount: 2, MaxWords: 4})
	if _, ok := find(got, "basis"); !ok {
		t.Errorf("basis was folded away: %v", ens(got))
	}
}

// Mathematics is not translated and its letters are not words. Replacing a span
// with a break rather than with nothing is what stops "the module $M$ is flat"
// from mining the phrase "module flat", which was never written.
func TestMathematicsBreaksAPhrase(t *testing.T) {
	body := strings.Repeat("The quiver $Q$ is flat over the ring $A$.\n", 5)
	got := mine(t, []Doc{{Path: "a.md", Body: body}}, Options{MinCount: 2, MaxWords: 4})
	if _, ok := find(got, "quiver flat"); ok {
		t.Errorf("two words either side of a formula were mined as a phrase")
	}
}

// A title is Bourbaki's own terminology and goes in whatever its count, which
// is the one place here where frequency is not the test.
func TestATitleIsOfferedOnce(t *testing.T) {
	got := mine(t, []Doc{{Path: "a.md", Body: "nothing at all here\n",
		Titles: []string{"Wobbly Quivers over a Flat Ring"}}}, Options{MinCount: 99, MaxWords: 4})
	c, ok := find(got, "Wobbly Quivers over a Flat Ring")
	if !ok {
		t.Fatalf("the title was not offered: %v", ens(got))
	}
	if len(c.Sources) != 1 || c.Sources[0] != SourceTitle {
		t.Errorf("the title's sources are %v, want just title", c.Sources)
	}
}

// A word a title sets off on its own is a term, and the fragment rule does not
// get to take it away.
//
// The counts cannot settle this case. A title is counted once for the file it
// heads and the phrases inside it are counted zero, so a word that reaches a
// count of one by folding a plural onto it lands exactly level with the title
// that contains it, and equal counts is what the fragment rule reads as proof.
// Theory of Sets lost axiom, empty set, ordered pair, well-ordered set, inverse
// limit and quantified theories this way, every one of them a heading of the
// volume.
func TestAWordATitleSetsOffOnItsOwnSurvives(t *testing.T) {
	got := mine(t, []Doc{{Path: "a.md", Body: "nothing at all here\n",
		Titles: []string{"The gribble of extent", "Gribbles"}}}, Options{MinCount: 99, MaxWords: 4})
	if _, ok := find(got, "gribble"); !ok {
		t.Errorf("the word heads a title of its own and was dropped as a fragment: %v", ens(got))
	}
}

// The exemption above is the whole run and not any phrase a title contains.
//
// Without that limit the fragments come back with it: a long title yields three
// and four word cuts out of its middle, "connected commutative real lie" and
// the like, which are not terms and which a curator has to read and reject one
// by one. 91 of them over the real corpus.
func TestAPhraseCutOutOfTheMiddleOfATitleIsStillAFragment(t *testing.T) {
	body := strings.Repeat("Connected flabby wobbly quivers are rare.\n", 6)
	got := mine(t, []Doc{{Path: "a.md", Body: body,
		Titles: []string{"Connected flabby wobbly quivers"}}}, Options{MinCount: 3, MaxWords: 4})
	if _, ok := find(got, "flabby wobbly"); ok {
		t.Errorf("two words out of the middle of a title were offered as a term")
	}
	if _, ok := find(got, "connected flabby wobbly quivers"); !ok {
		t.Errorf("the run itself was not offered: %v", ens(got))
	}
}

// set is a noun. It was in the stop list because it is also a verb, and the
// English corpus says "the set" 855 times against "we set" 17, so the list was
// breaking phrases at the commonest noun in mathematics to catch one use in
// fifty. Theory of Sets cannot be translated at all without the row.
func TestSetIsATermAndNotAStopWord(t *testing.T) {
	body := strings.Repeat("The ordered set of quivers is flat and the set is small.\n", 6)
	got := mine(t, []Doc{{Path: "a.md", Body: body}}, Options{MinCount: 3, MaxWords: 4})
	c, ok := find(got, "set")
	if !ok {
		t.Fatalf("set was not offered: %v", ens(got))
	}
	if c.Count != 12 {
		t.Errorf("set is counted %d times, want 12: six sentences saying it twice", c.Count)
	}
	if _, ok := find(got, "ordered set"); !ok {
		t.Errorf("the phrase the word is half of was not offered either: %v", ens(got))
	}
}

// The sentence form of a definition, which is thin on chapter VIII but always
// right when it hits.
func TestADefinedTermIsFound(t *testing.T) {
	body := "A quiver is called flabby if it has no proper subquiver.\n"
	got := mine(t, []Doc{{Path: "a.md", Body: body}}, Options{MinCount: 99, MaxWords: 4})
	c, ok := find(got, "flabby")
	if !ok {
		t.Fatalf("the defined term was not found: %v", ens(got))
	}
	if c.Where != "a.md:1" {
		t.Errorf("the term points at %q, want a.md:1, the line a curator has to read", c.Where)
	}
}

// The statement kinds are put in whether the mining found them or not, because
// every file uses them and every language has to render them the same way.
func TestTheStatementKindsAreAlwaysThere(t *testing.T) {
	got := mine(t, []Doc{{Path: "a.md", Body: "nothing\n"}}, Options{MinCount: 99, MaxWords: 4})
	for _, k := range []string{"theorem", "proposition", "lemma", "corollary"} {
		if _, ok := find(got, k); !ok {
			t.Errorf("%q is not in the candidates", k)
		}
	}
}

// Bourbaki writes a term capitalised at the head of a sentence and lower case
// inside one, and those are one term with one spelling.
func TestTheLowerCaseSpellingWins(t *testing.T) {
	body := "Flabby quivers are flat.\n\nEvery flabby quiver is flat.\n"
	got := mine(t, []Doc{{Path: "a.md", Body: body}}, Options{MinCount: 1, MaxWords: 4})
	c, ok := find(got, "flabby quiver")
	if !ok {
		t.Fatalf("not found: %v", ens(got))
	}
	if c.EN != "flabby quiver" {
		t.Errorf("the term is written %q, want the lower case spelling", c.EN)
	}
}

func ens(cands []Candidate) []string {
	var out []string
	for _, c := range cands {
		out = append(out, c.EN)
	}
	return out
}

func TestKeyIsCaseAndSpaceInsensitive(t *testing.T) {
	if Key("  Artinian   Ring ") != "artinian ring" {
		t.Errorf("got %q", Key("  Artinian   Ring "))
	}
}

// Longest first, because a scan that met "ring" first would find it inside
// "semisimple ring" and report the longer term as missing from a translation
// that renders it perfectly well.
func TestSortedIsLongestFirst(t *testing.T) {
	g := Glossary{Terms: []Term{{EN: "ring"}, {EN: "semisimple ring"}, {EN: "module"}}}
	got := g.Sorted()
	if got[0].EN != "semisimple ring" {
		t.Errorf("the longest term is not first: %v", got)
	}
}

func TestADuplicatedTermIsAnError(t *testing.T) {
	g := Glossary{Version: 1, Terms: []Term{{EN: "ring", VI: "vành"}, {EN: "Ring", VI: "nhẫn"}}}
	if err := g.Validate(); err == nil {
		t.Error("two rows for one term were accepted, and only one of them can be in use")
	}
}
