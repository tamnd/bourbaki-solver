package report

import (
	"strings"
	"testing"
)

// A library half read has to read as a library half read. The whole reason this
// report exists is that ocr check divides by the pages that are there, so a
// volume with 57 of 222 pages prints 100 per cent and looks done.
func volumes() []Volume {
	return []Volume{
		{ID: "alg-viii", Lang: "en", TextLayer: "native", Pages: 505, Read: 505,
			Methods: map[string]int{"native": 494, "blank": 11},
			Checked: 494, Rejected: []int{1, 2, 3}, Rules: map[string]int{"short": 3},
			Manual: 40, Flagged: []int{7, 9}},
		{ID: "alg-x-fr", Lang: "fr", TextLayer: "none", Pages: 222, Read: 57,
			Methods: map[string]int{"ocr": 57}, Checked: 57,
			Rules: map[string]int{}, NoPageMap: true},
		{ID: "top-i-iv", Lang: "en", TextLayer: "ocr", Pages: 443,
			Methods: map[string]int{}, Rules: map[string]int{}},
	}
}

func TestSummariseExtractionCountsThePagesThatAreNotThere(t *testing.T) {
	got := SummariseExtraction(volumes())
	if got.Total.Pages != 505+222+443 {
		t.Errorf("pdf pages %d", got.Total.Pages)
	}
	if got.Total.Read != 562 {
		t.Errorf("read %d, want 562", got.Total.Read)
	}
	if got.Total.Unread() != 608 {
		t.Errorf("unread %d, want 608", got.Total.Unread())
	}
	// A volume nobody has opened is not a volume that passed. 57 of 222 read
	// with every one of the 57 accepted is a quarter of the book, and ocr check
	// calls the same volume 100 per cent.
	half := got.Rows[1]
	if half.ID != "alg-x-fr" || half.AcceptedText() != "100.0 %" {
		t.Fatalf("row %+v", half)
	}
	if c := half.Coverage(); c < 25.6 || c > 25.7 {
		t.Errorf("coverage %.1f %%, want 25.7", c)
	}
	if got.Whole() != 1 || got.Part() != 1 || got.Untouched() != 1 {
		t.Errorf("whole %d, part %d, untouched %d", got.Whole(), got.Part(), got.Untouched())
	}
}

// Worst first, because the top of the table is the work that is left.
func TestSummariseExtractionOrdersWorstCoveredFirst(t *testing.T) {
	got := SummariseExtraction(volumes())
	want := []string{"top-i-iv", "alg-x-fr", "alg-viii"}
	for i, id := range want {
		if got.Rows[i].ID != id {
			t.Fatalf("row %d is %s, want %s", i, got.Rows[i].ID, id)
		}
	}
}

// A volume whose only page files are blanks has nothing for the rules to run
// over, and 0.0 per cent there would say the pages failed when there were none.
func TestAcceptedTextSaysWhenNothingWasChecked(t *testing.T) {
	if got := (Volume{}).AcceptedText(); got != "none checked" {
		t.Errorf("got %q", got)
	}
	v := Volume{Checked: 494, Rejected: []int{1, 2, 3}}
	if got := v.AcceptedText(); got != "99.4 %" {
		t.Errorf("got %q", got)
	}
}

func TestExtractionDocSaysWhatIsMissing(t *testing.T) {
	doc := SummariseExtraction(volumes()).Doc()
	for _, want := range []string{
		"# What the extraction is worth",
		"| top-i-iv | en | ocr | 443 | 0 | 443 |",
		"of 3 volumes, 1 are read through", // the sentence the percentage hides
		"no page map for alg-x-fr",
		"None. The extraction rejects a page it cannot read",
	} {
		if !strings.Contains(strings.ToLower(doc), strings.ToLower(want)) {
			t.Errorf("the report does not say %q", want)
		}
	}
	// A volume with nothing read is a row of zeros in the methods table and it
	// is already in the coverage table, where the zero is the point.
	methods := doc[strings.Index(doc, "## How the pages were read"):]
	if strings.Contains(methods[:strings.Index(methods, "## Pages left")], "top-i-iv") {
		t.Error("an unread volume is in the methods table")
	}
}

// The one thing the audit blocks on has to be named when it is there and named
// as absent when it is not. Silence would read as either.
func TestExtractionDocNamesTheFailedPages(t *testing.T) {
	rows := volumes()
	rows[1].Failed = []int{31, 32}
	rows[1].Methods["ocr-failed"] = 2
	doc := SummariseExtraction(rows).Doc()
	if !strings.Contains(doc, "the audit blocks on every one of them under S06") {
		t.Error("the report does not say the audit blocks")
	}
	if !strings.Contains(doc, "- alg-x-fr: 31 32") {
		t.Error("the report does not name the failed pages")
	}
	if !strings.Contains(doc, "| ocr-failed |") {
		t.Error("ocr-failed is not a column of the methods table")
	}
}

func TestExtractionTableIsEmptyWhenNothingIsRead(t *testing.T) {
	if got := SummariseExtraction(nil).Table(); !strings.Contains(got, "no volume") {
		t.Errorf("got %q", got)
	}
}
