package main

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/extract"
	"github.com/tamnd/bourbaki-solver/pagemap"
)

// What says a volume has a page label to build is the grammar, which is what it
// prints, and not the pagination, which is what the number in the label counts.
// The two come apart in Théories spectrales and Topologie algébrique: both
// print "TS I.31" and "TA I.139" at the head, and both number their pages
// straight through the volume rather than restarting at each chapter. Asking
// pagination dropped the label from every page of them that carries no head,
// which is every page that opens a chapter, a § or a set of exercises.
func TestAVolumePagedStraightThroughStillPrintsItsLabel(t *testing.T) {
	cases := []struct {
		name  string
		book  corpus.Book
		entry pagemap.Entry
		page  extract.Page
		want  string
	}{{
		name:  "head label and continuous, the head read",
		book:  corpus.Book{Book: "ts", Grammar: "head-label", Pagination: "continuous"},
		entry: pagemap.Entry{PDFPage: 44, Chapter: "I", Page: 31},
		page:  extract.Page{PDFPage: 44, Label: "TS I.31"},
		want:  "TS I.31",
	}, {
		name:  "head label and continuous, a page that opens a chapter",
		book:  corpus.Book{Book: "ta", Grammar: "head-label", Pagination: "continuous"},
		entry: pagemap.Entry{PDFPage: 155, Chapter: "I"},
		page:  extract.Page{PDFPage: 155, Foot: 139},
		want:  "TA I.139",
	}, {
		name:  "head label and per chapter",
		book:  corpus.Book{Book: "alg", Grammar: "head-label", Pagination: "per-chapter"},
		entry: pagemap.Entry{PDFPage: 20, Chapter: "VIII", Page: 13},
		page:  extract.Page{PDFPage: 20},
		want:  "A VIII.13",
	}, {
		// Theory of Sets prints a bare number at the foot and no label
		// anywhere, so "E IV.289" would be made up: it reads as page 289 of
		// chapter IV, and chapter IV is 60 pages long.
		name:  "foot number",
		book:  corpus.Book{Book: "ens", Grammar: "foot-number", Pagination: "continuous"},
		entry: pagemap.Entry{PDFPage: 300, Chapter: "IV", Page: 289},
		page:  extract.Page{PDFPage: 300, Foot: 289},
		want:  "",
	}, {
		// Lie 7 to 9 carries the number at the outer edge of the head.
		name:  "head number",
		book:  corpus.Book{Book: "lie", Grammar: "head-number", Pagination: "continuous"},
		entry: pagemap.Entry{PDFPage: 120, Chapter: "VIII", Page: 100},
		page:  extract.Page{PDFPage: 120, Foot: 100},
		want:  "",
	}}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			m := &pagemap.Map{Book: c.book.Book, Entries: make([]pagemap.Entry, c.entry.PDFPage)}
			for i := range m.Entries {
				m.Entries[i].PDFPage = i + 1
			}
			m.Entries[c.entry.PDFPage-1] = c.entry
			if got := pageLabel(m, &c.book, &c.page); got != c.want {
				t.Errorf("pageLabel = %q, want %q", got, c.want)
			}
		})
	}
}
