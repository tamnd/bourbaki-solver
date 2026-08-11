package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// A page written twice from the same reading is the same bytes.
//
// This is the property that makes a re-extraction reviewable. Before it, every
// page carried the wall clock of the run that wrote it, so re-reading a volume
// rewrote all 505 pages and the diff of a run that changed 7 of them was 505
// files deep. A reviewer cannot find 7 files in 505, so nobody looked, which is
// the opposite of what committing the pages is for.
func TestAPageWrittenTwiceIsTheSameBytes(t *testing.T) {
	root := t.TempDir()
	page := func() PageFile {
		return PageFile{
			Meta: PageFrontMatter{
				Book: "alg-viii", PDFPage: 450, Method: MethodNative,
				InputSHA256: "fb5fe525c0e40ca7d4e8f4a3392a3420b37448d3eb6378198d3a7eb22811002e",
				Lines:       33,
			},
			Body: "Soit A un anneau.\n",
		}
	}
	first := filepath.Join(root, "first.md")
	second := filepath.Join(root, "second.md")
	p := page()
	if err := p.Write(first); err != nil {
		t.Fatal(err)
	}
	q := page()
	if err := q.Write(second); err != nil {
		t.Fatal(err)
	}
	a, err := os.ReadFile(first)
	if err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(second)
	if err != nil {
		t.Fatal(err)
	}
	if string(a) != string(b) {
		t.Errorf("the same page written twice differs:\n%s\n---\n%s", a, b)
	}
	if strings.Contains(string(a), "generated") {
		t.Error("the page carries a generated stamp, which is what made re-extraction unreviewable")
	}
}
