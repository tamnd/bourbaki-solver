package refs

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// A reference belongs to the statement it is printed under, because that is
// almost always the proof of that statement. Getting this wrong would not fail
// anything loudly: the graph would still have the right number of edges, all of
// them hung off the wrong statements.
func TestEdgesHangOffTheStatementAbove(t *testing.T) {
	ix := testIndex()
	body := strings.Join([]string{
		"### 1. A first no.",
		"",
		"Before any statement, we recall Proposition 1.",
		"",
		"#### Proposition 2 {#alg-viii-s1-prop-2 .statement tag=0003}",
		"",
		"This follows from Proposition 1.",
		"",
		"#### Theorem 1 {#alg-viii-s1-thm-1 .statement tag=0006}",
		"",
		"By Proposition 2 and by VIII, p. 3.",
	}, "\n")

	res := &Result{Counts: map[string]int{}, Forms: map[Form]int{}, Index: ix}
	at := where{section: "alg-viii-s1"}
	lines := strings.Split(body, "\n")
	res.lines(ix, "s1.md", body, 1, func(i int) where {
		if m := statementRE.FindStringSubmatch(lines[i]); m != nil {
			at = where{section: "alg-viii-s1", label: m[1], tag: m[2]}
		}
		return at
	})

	want := []struct{ from, to, tail string }{
		// Before the first statement the § itself is the tail, and it has no tag.
		{"alg-viii-s1", "alg-viii-s1-prop-1", ""},
		{"alg-viii-s1-prop-2", "alg-viii-s1-prop-1", "0003"},
		{"alg-viii-s1-thm-1", "alg-viii-s1-prop-2", "0006"},
		{"alg-viii-s1-thm-1", "alg-viii-s1", "0006"},
	}
	if len(res.Edges) != len(want) {
		t.Fatalf("built %d edges, want %d: %+v", len(res.Edges), len(want), res.Edges)
	}
	for i, w := range want {
		e := res.Edges[i]
		if e.FromLabel != w.from || e.ToLabel != w.to || e.From != w.tail {
			t.Errorf("edge %d is %s (%q) -> %s, want %s (%q) -> %s",
				i, e.FromLabel, e.From, e.ToLabel, w.from, w.tail, w.to)
		}
	}
	// A heading is put in force from its own line, and reads as no citation
	// itself, so no statement cites itself here.
	for _, e := range res.Edges {
		if e.FromLabel == e.ToLabel {
			t.Errorf("%s cites itself", e.FromLabel)
		}
	}
}

// The printed words stay printed and the graph is read out of the correction.
// § 1 of the fixture prints one Theorem 1 and no Proposition 12, so the printed
// sentence has nowhere to go and the corrected one resolves.
func TestAReferenceIsReadOutOfTheCorrectedBody(t *testing.T) {
	ix := testIndex()
	const printed = "This follows from Proposition 12."
	errata := []corpus.Erratum{{
		Says: "Proposition 12", Read: "Theorem 1",
		Why: "§ 1 prints eight statements and no Proposition 12.",
	}}
	if got := corrected(printed, errata); got != "This follows from Theorem 1." {
		t.Fatalf("the corrected body is %q", got)
	}

	res := &Result{Counts: map[string]int{}, Forms: map[Form]int{}, Index: ix}
	at := where{section: "alg-viii-s1"}
	res.lines(ix, "s1.md", corrected(printed, errata), 1, func(int) where { return at })
	if len(res.Edges) != 1 {
		t.Fatalf("built %d edges: %+v", len(res.Edges), res.Edges)
	}
	if res.Edges[0].ToLabel != "alg-viii-s1-thm-1" {
		t.Errorf("the edge points at %q", res.Edges[0].ToLabel)
	}

	// Read as printed it is a Proposition the § does not have, which is the
	// finding the manifest of errata exists to answer.
	res = &Result{Counts: map[string]int{}, Forms: map[Form]int{}, Index: ix}
	res.lines(ix, "s1.md", printed, 1, func(int) where { return at })
	if len(res.Unresolved) != 1 {
		t.Fatalf("the printed sentence left %d unresolved: %+v", len(res.Unresolved), res.Unresolved)
	}
}

func TestHeadLines(t *testing.T) {
	file := "---\nbook: alg\n---\n\nthe body\nand more\n"
	body := "the body\nand more\n"
	// The front matter is four lines, so the body opens on line 5.
	if got := headLines([]byte(file), body); got != 5 {
		t.Errorf("the body starts at line %d, want 5", got)
	}
}

func TestIsExercise(t *testing.T) {
	if !isExercise("/x/content/en/alg/VIII/exercises/s1/01.md") {
		t.Error("an exercise was not recognised")
	}
	if isExercise("/x/content/en/alg/VIII/01_s1_artinian.md") {
		t.Error("a section was taken for an exercise")
	}
}

// A reference that names a § and no statement in it is not a citation of any
// statement, so it must not put one at the top of the most cited list.
func TestInDegreeCountsStatementsOnly(t *testing.T) {
	label := "0001"
	res := &Result{Edges: []Edge{
		{ToLabel: "alg-viii-s1-prop-1", To: &label, How: ByPage},
		{ToLabel: "alg-viii-s1-prop-1", To: &label, How: ByContext},
		{ToLabel: "alg-viii-s1", How: BySection},
		{Book: "E", How: OutOfCorpus},
	}}
	got := res.inDegree()
	if got["alg-viii-s1-prop-1"] != 2 {
		t.Errorf("the proposition is cited %d times, want 2", got["alg-viii-s1-prop-1"])
	}
	if _, ok := got["alg-viii-s1"]; ok {
		t.Errorf("a § was counted as a cited statement")
	}
	if len(got) != 1 {
		t.Errorf("in-degree came out as %+v", got)
	}
}
