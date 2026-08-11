package quality

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// The registered library is forty three volumes and only three of them have a
// table of contents, so a percentage taken over the rows of the table is a
// percentage of what somebody has already opened. The block has to say so, or
// the number reads as a fraction of the Éléments.
func TestCoverageSaysWhatItLeavesOut(t *testing.T) {
	c := &Corpus{
		Books: &corpus.BooksManifest{Books: []corpus.Book{
			{ID: "alg-viii", Book: "alg", Pages: 505},
			{ID: "ac-i-vii", Book: "ac", Pages: 642},
			{ID: "ts-i-ii-fr", Book: "ts", Pages: 346},
		}},
		TOC: &corpus.TOCManifest{Books: []corpus.BookTOC{{
			ID: "alg-viii", Chapters: []corpus.Chapter{{
				Numeral: "VIII", Sections: []corpus.Section{{}, {}},
			}},
		}}},
	}
	got := Coverage(c)
	if !strings.Contains(got, "2 further volumes and 988 pages") {
		t.Errorf("coverage block does not account for the unread volumes:\n%s", got)
	}
}

// With every registered volume in the table of contents there is nothing left
// out, and a sentence saying nothing is left out is noise.
func TestCoverageIsSilentWhenNothingIsLeftOut(t *testing.T) {
	c := &Corpus{
		Books: &corpus.BooksManifest{Books: []corpus.Book{{ID: "alg-viii", Book: "alg", Pages: 505}}},
		TOC: &corpus.TOCManifest{Books: []corpus.BookTOC{{
			ID: "alg-viii", Chapters: []corpus.Chapter{{Numeral: "VIII"}},
		}}},
	}
	if got := Coverage(c); strings.Contains(got, "further volumes") {
		t.Errorf("coverage block invented an omission:\n%s", got)
	}
}
