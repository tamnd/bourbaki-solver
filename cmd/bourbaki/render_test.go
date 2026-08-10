package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/extract"
)

// The flagged list is the work list for the repair pass, and it is also the one
// thing that lets a born-digital volume be rendered at all, so it has to come
// off the report on disk and nowhere else.
func TestTheFlaggedPagesComeOffTheExtractionReport(t *testing.T) {
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "reports"), 0o755); err != nil {
		t.Fatal(err)
	}
	result := extract.Result{Book: "alg-viii", Pages: 505, Flags: []extract.PageFlags{
		{PDFPage: 77, Flags: []string{"diagram"}},
		{PDFPage: 41, Flags: []string{"empty"}},
		{PDFPage: 76, Flags: []string{"diagram"}},
	}}
	raw, err := json.Marshal(result)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corpus.ExtractReportPath(root, "alg-viii"), raw, 0o644); err != nil {
		t.Fatal(err)
	}

	pages, err := flaggedPages(root, "alg-viii")
	if err != nil {
		t.Fatal(err)
	}
	want := []int{41, 76, 77}
	if len(pages) != len(want) {
		t.Fatalf("pages = %v, want %v", pages, want)
	}
	for i := range pages {
		if pages[i] != want[i] {
			t.Fatalf("pages = %v, want %v in page order", pages, want)
		}
	}
}

// Without a report there is no work list, and rendering the whole volume
// because a file was missing is exactly what the guard is there to stop.
func TestNoExtractionReportIsAnErrorAndNotAWholeVolume(t *testing.T) {
	pages, err := flaggedPages(t.TempDir(), "alg-viii")
	if err == nil {
		t.Fatalf("a missing report gave back %v and no error", pages)
	}
	if len(pages) != 0 {
		t.Errorf("pages = %v, want none", pages)
	}
}
