package refs

import (
	"strings"
	"testing"
)

// The reports are committed and CI checks them by regenerating and diffing, so
// a report that is not a function of the graph alone fails the check with
// nothing wrong. This one was: its rows come out of a map keyed by Book and
// chapter, and two rows of one Book with the same count had nothing left to
// separate them, so they came out in whatever order the map was walked in.
//
// It went unnoticed while chapter VIII was thought to cite two Books, because
// the counts of the eleven chapters were nearly all different. Reading the
// abbreviations took it to nine Books and thirty chapters, most of them cited
// once, and the report stopped being stable between two runs of the same build.
func TestTheOutOfCorpusReportIsTheSameOnEveryRun(t *testing.T) {
	edges := func(chapters ...string) []Edge {
		var out []Edge
		for _, c := range chapters {
			book, chapter, _ := strings.Cut(c, " ")
			out = append(out, Edge{How: OutOfCorpus, Book: book, Chapter: chapter})
		}
		return out
	}
	// Every chapter here is cited once, which is the shape that has nothing to
	// sort on but the chapter.
	want := (&Result{Edges: edges("TG I", "TG II", "TG III", "TG IV", "TG VIII", "TG X")}).outOfCorpus(nil)
	for i := 0; i < 20; i++ {
		got := (&Result{Edges: edges("TG X", "TG IV", "TG I", "TG VIII", "TG III", "TG II")}).outOfCorpus(nil)
		if got != want {
			t.Fatalf("run %d came out differently:\n%s\nwant\n%s", i, got, want)
		}
	}
	rows := []string{"| 1 | TG | I |", "| 1 | TG | II |", "| 1 | TG | III |",
		"| 1 | TG | IV |", "| 1 | TG | VIII |", "| 1 | TG | X |"}
	if !strings.Contains(want, strings.Join(rows, "\n")) {
		t.Errorf("the chapters are not in the order the Éléments number them:\n%s", want)
	}
}

// The more cited chapter still comes first. Ordering by the numeral is the last
// word and not the first, since the report is the ingestion order.
func TestTheOutOfCorpusReportPutsTheMostCitedChapterFirst(t *testing.T) {
	var edges []Edge
	for i := 0; i < 3; i++ {
		edges = append(edges, Edge{How: OutOfCorpus, Book: "TG", Chapter: "IV"})
	}
	edges = append(edges, Edge{How: OutOfCorpus, Book: "TG", Chapter: "I"})
	got := (&Result{Edges: edges}).outOfCorpus(nil)
	if !strings.Contains(got, "| 3 | TG | IV |\n| 1 | TG | I |") {
		t.Errorf("the rows are not in citation order:\n%s", got)
	}
}
