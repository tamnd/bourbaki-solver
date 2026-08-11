package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// writeHead puts one page file on disk with the given front matter and hands
// back its path.
func writeHead(t *testing.T, head string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "0001.md")
	if err := os.WriteFile(path, []byte(head), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

// A first run has no pages at all, and every one of its pages is a page it may
// write.
func TestAPageThatIsNotThereMayBeWritten(t *testing.T) {
	keep, err := repairedByHand(filepath.Join(t.TempDir(), "0001.md"))
	if err != nil {
		t.Fatalf("a page that is not there is not an error: %v", err)
	}
	if keep {
		t.Error("a page that is not there was kept")
	}
}

// The ordinary case, in both directions: extraction owns the pages it wrote and
// nobody else's.
func TestOnlyAManualPageIsKept(t *testing.T) {
	const head = "---\nbook: alg-viii\npdf_page: 1\nmethod: native\n" +
		"input_sha256: abc\nlines: 4\n"
	for _, tc := range []struct {
		name string
		head string
		want bool
	}{
		{"machine", head + "---\n\nText.\n", false},
		{"by hand", head + "manual: true\n---\n\nText.\n", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			keep, err := repairedByHand(writeHead(t, tc.head))
			if err != nil {
				t.Fatal(err)
			}
			if keep != tc.want {
				t.Errorf("kept is %v, want %v", keep, tc.want)
			}
		})
	}
}

// This is the one that matters. Front matter is decoded with KnownFields set,
// so dropping a field from PageFrontMatter stops every page written before the
// change from parsing at all, and a page that will not parse is a page whose
// manual flag cannot be read. Answering false there answers "nobody repaired
// this" to a question that was never asked, and it cost seven pages of chapter
// VIII the day the generated stamp came out. They came back because they were
// committed, which is luck and not a plan.
func TestAPageThatWillNotParseIsNotOverwritten(t *testing.T) {
	path := writeHead(t, "---\nbook: alg-viii\npdf_page: 426\nmethod: native\n"+
		"input_sha256: abc\nlines: 12\ngenerated: \"2026-08-11T17:05:33Z\"\nmanual: true\n"+
		"---\n\nThe repair somebody read the printed page to make.\n")
	keep, err := repairedByHand(path)
	if err == nil {
		t.Fatal("a page that will not parse was passed through as readable")
	}
	if keep {
		t.Error("a page that will not parse was reported as kept, which hides the error")
	}
	if !strings.Contains(err.Error(), path) {
		t.Errorf("the error does not name the page: %v", err)
	}
}

// Reading the flag back off a page the writer wrote, so the test is not
// asserting a spelling of its own invention.
func TestTheFlagTheWriterWritesIsTheFlagTheGuardReads(t *testing.T) {
	root := t.TempDir()
	f := corpus.PageFile{
		Meta: corpus.PageFrontMatter{
			Book: "alg-viii", PDFPage: 106, Method: corpus.MethodNative,
			InputSHA256: "abc", Lines: 4, Manual: true,
		},
		Body: "Text.\n",
	}
	path := corpus.PagePath(root, "alg-viii", 106)
	if err := f.Write(path); err != nil {
		t.Fatal(err)
	}
	keep, err := repairedByHand(path)
	if err != nil {
		t.Fatal(err)
	}
	if !keep {
		t.Error("the page the writer marked by hand was not kept")
	}
}
