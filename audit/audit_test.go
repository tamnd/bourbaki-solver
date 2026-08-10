package audit

import (
	"strings"
	"testing"
)

// The line as page 51 of Algebra I came back, and the line as the scan reads.
const (
	misread = `**Definition 5.** Let $E_1,\ldots,E_n$ and $F$ be sets and $u$ a mapping of $E_1\times\cdots\times E_n$ into $F$. Let $i\in\{1,n\}$. Suppose $E_i$ and $F$ are given the structures of magmas.`
	correct = `**Definition 5.** Let $E_1,\ldots,E_n$ and $F$ be sets and $u$ a mapping of $E_1\times\cdots\times E_n$ into $F$. Let $i\in[1,n]$. Suppose $E_i$ and $F$ are given the structures of magmas.`
)

func TestTheIntervalOnPage51IsFound(t *testing.T) {
	found := Scan("DISTRIBUTIVITY\n\n" + misread)
	if len(found) != 1 {
		t.Fatalf("found %d suspects, want the one on the page: %+v", len(found), found)
	}
	s := found[0]
	if s.Line != 3 {
		t.Errorf("the suspect is on line %d, want 3", s.Line)
	}
	if s.Span != `\{1,n\}` {
		t.Errorf("the span is %q, want the braces and what is between them", s.Span)
	}
	if !strings.Contains(s.Text, s.Span) || strings.Count(s.Text, s.Span) != 1 {
		t.Error("the span is not on its line exactly once, which is what the repair audit splits at")
	}
	if s.Why == "" {
		t.Error("the suspect does not say what is suspected")
	}
}

// Every one of these costs a follow up call when it fires. A detector that
// fires on ordinary mathematics spends the day asking about pages that are
// right.
func TestWhatTheIntervalDetectorLeavesAlone(t *testing.T) {
	quiet := []struct {
		name string
		line string
	}{
		{"the same line read correctly", correct},
		{"a set written out in full", `for all $i\in\{1,\ldots,n\}$ and $x$ in $E$`},
		{"a set of three", `let $j\in\{1,2,3\}$ be given`},
		{"an ordinary set of elements", `suppose $x\in\{a,b\}$ holds`},
		{"a set that is not the object of an $\\in$", `the set $\{1,n\}$ is finite`},
		{"prose with a comma in braces nowhere near it", `the mapping $u$ is distributive, as above`},
	}
	for _, c := range quiet {
		t.Run(c.name, func(t *testing.T) {
			if found := Scan(c.line); len(found) != 0 {
				t.Errorf("the detector fired on %s: %+v", c.name, found)
			}
		})
	}
}

// Two of them on one line have nothing to prove a correction against, since the
// audit splits the line at the span, so they are dropped rather than asked
// about.
func TestASpanTwiceOnALineIsNotASuspect(t *testing.T) {
	line := `both $i\in\{1,n\}$ and $j\in\{1,n\}$ hold`
	if found := Scan(line); len(found) != 0 {
		t.Errorf("a span that is on its line twice was reported: %+v", found)
	}
}

// One set of elements $x\in\{a,b\}$ is left alone above. This is the same
// shape with a digit in it, which is the one that is nearly always an
// interval, and it has to be asked about even though it may be a real set.
func TestTheDigitShapeIsAskedAboutEvenThoughItIsSometimesASet(t *testing.T) {
	if found := Scan(`suppose $i\in\{1,p\}$ and consider`); len(found) != 1 {
		t.Fatalf("found %d suspects, want 1", len(found))
	}
}
