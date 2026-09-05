package quality

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The scan that nobody ran.
//
// M01 has found every unclosed math span in this corpus since it was written,
// and people went on finding them with publish -check instead, which walks the
// whole corpus for ten minutes and stops at the first fault. The reason was not
// the rule. It was that asking it cost a minute, almost all of it spent building
// a reference graph M01 does not read, so the answer arrived after the file it
// was about had been closed again.
func TestARunNarrowedToOneRuleDoesNotBuildAGraphThatRuleNeverReads(t *testing.T) {
	for _, c := range []struct {
		only []string
		want bool
	}{
		{nil, true},                          // a full audit, and CI
		{[]string{"M01"}, false},             // the scan this is for
		{[]string{"R01"}, true},              // one reference rule is enough
		{[]string{"references"}, true},       // and so is the group
		{[]string{"M01", "R03"}, true},       // asked for together, built once
		{[]string{"mathematics"}, false},     // the whole M group reads none of it
		{[]string{"hygiene", "tags"}, false}, //
		{[]string{"nonsense"}, true},         // bad flags are Run's to report
	} {
		if got := (Options{Only: c.only}).wantsRefs(); got != c.want {
			t.Errorf("-only %v builds the graph: %v, want %v", c.only, got, c.want)
		}
	}
}

// And the load honours it, so that the R rules are the only thing that pays for
// the graph. A run that did not build one leaves Refs nil, which is what the
// report's summary and its JSON already test for.
func TestTheGraphIsBuiltOnlyWhenAReferenceRuleIsInTheRun(t *testing.T) {
	root := t.TempDir()
	writeSection(t, root, "content/en/ens/III/00_frontmatter.md", corpus.SectionFrontMatter{
		Book: "ens", Lang: "en", Section: 1,
	}, "Let $E$ be a set, and see II, p. 4 for the rest.")

	narrow, err := Load(Options{Root: root, Only: []string{"M01"}})
	if err != nil {
		t.Fatal(err)
	}
	if narrow.Refs != nil {
		t.Error("a run of M01 alone built the reference graph anyway")
	}
	// The English is still loaded, because the rule that was asked for reads it.
	if len(narrow.Docs) != 1 {
		t.Fatalf("the run loaded %d documents, want the one file", len(narrow.Docs))
	}

	whole, err := Load(Options{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if whole.Refs == nil {
		t.Error("a full audit has no reference graph, so R01 to R03 cannot run")
	}
}
