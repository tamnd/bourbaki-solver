package corpus

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const sectionFile = `---
book: alg
book_title: Algebra
chapter: VIII
chapter_title: Semisimple Modules and Rings
section: 1
section_title: Simple Modules
lang: en
source: alg-viii
book_pages: 1-6
pdf_pages: 18-23
statements: 7
exercises: 12
content_sha256: 0000000000000000000000000000000000000000000000000000000000000000
---

Let A be a ring.
`

func TestParseFile(t *testing.T) {
	f, err := ParseFile[SectionFrontMatter]([]byte(sectionFile))
	if err != nil {
		t.Fatal(err)
	}
	if f.Meta.Chapter != "VIII" || f.Meta.Section != 1 || f.Meta.Statements != 7 {
		t.Errorf("meta = %+v", f.Meta)
	}
	if f.Body != "Let A be a ring.\n" {
		t.Errorf("body = %q", f.Body)
	}
}

// A field the schema does not know about is a mistake nobody would otherwise
// notice, so the decoder is strict.
func TestParseFileRejectsUnknownField(t *testing.T) {
	bad := strings.Replace(sectionFile, "statements: 7", "statements: 7\ndifficulty: 42", 1)
	if _, err := ParseFile[SectionFrontMatter]([]byte(bad)); err == nil {
		t.Fatal("an unknown front matter field was accepted")
	}
}

func TestParseFileNeedsFence(t *testing.T) {
	if _, err := ParseFile[SectionFrontMatter]([]byte("book: alg\n\nLet A be a ring.\n")); err == nil {
		t.Fatal("a file with no front matter was accepted")
	}
}

// The hash has to survive the things an editor does on its own, or every
// translation in the corpus goes stale the first time somebody opens a file.
func TestContentSHA256IgnoresWhitespace(t *testing.T) {
	want := ContentSHA256("Let A be a ring.\n")
	for _, body := range []string{
		"Let A be a ring.\r\n",
		"Let A be a ring.   \n",
		"Let A be a ring.\n\n\n",
		"Let A be a ring.",
	} {
		if got := ContentSHA256(body); got != want {
			t.Errorf("ContentSHA256(%q) = %s, want %s", body, got[:8], want[:8])
		}
	}
	if ContentSHA256("Let B be a ring.\n") == want {
		t.Error("two different bodies hash the same")
	}
}

func TestBytesRecomputesHash(t *testing.T) {
	f, err := ParseFile[SectionFrontMatter]([]byte(sectionFile))
	if err != nil {
		t.Fatal(err)
	}
	f.Body = "Let A be a ring and M an A-module.\n"
	b, err := f.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	back, err := ParseFile[SectionFrontMatter](b)
	if err != nil {
		t.Fatal(err)
	}
	if back.Meta.ContentSHA256 != ContentSHA256(f.Body) {
		t.Errorf("content_sha256 = %s, want %s", back.Meta.ContentSHA256, ContentSHA256(f.Body))
	}
	if back.Body != f.Body {
		t.Errorf("body round trip = %q", back.Body)
	}
}

func TestWriteAndRead(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "content", "en", "alg", "VIII", "01_s1_simple_modules.md")
	f, err := ParseFile[SectionFrontMatter]([]byte(sectionFile))
	if err != nil {
		t.Fatal(err)
	}
	if err := f.Write(path); err != nil {
		t.Fatal(err)
	}
	if got := SectionPath(dir, "en", f.Meta); got != path {
		t.Errorf("SectionPath = %s, want %s", got, path)
	}
	back, err := ReadFile[SectionFrontMatter](path)
	if err != nil {
		t.Fatal(err)
	}
	if back.Meta.SectionTitle != f.Meta.SectionTitle || back.Body != f.Body {
		t.Errorf("round trip = %+v %q", back.Meta, back.Body)
	}
	if _, err := os.Stat(path); err != nil {
		t.Fatal(err)
	}
}

func TestStale(t *testing.T) {
	english := ContentSHA256("Let A be a ring.\n")
	var m SectionFrontMatter
	if m.Stale(english) {
		t.Error("a file that is not a translation was called stale")
	}
	m.TranslatedFrom = "content/en/alg/VIII/01_s1_simple_modules.md"
	m.SourceSHA256 = english
	if m.Stale(english) {
		t.Error("a translation of the committed English was called stale")
	}
	if !m.Stale(ContentSHA256("Let A be a ring and M an A-module.\n")) {
		t.Error("a translation of an older English was not called stale")
	}
}

func TestSlug(t *testing.T) {
	tests := []struct{ in, want string }{
		{"Simple Modules", "simple_modules"},
		{"Laws of composition; associativity; commutativity", "laws_of_composition_associativity"},
		{"Applications: I. Rational integers", "applications_i_rational_integers"},
		{"  ", ""},
	}
	for _, tt := range tests {
		if got := Slug(tt.in, SlugLen); got != tt.want {
			t.Errorf("Slug(%q) = %q, want %q", tt.in, got, tt.want)
		}
		if len(Slug(tt.in, SlugLen)) > SlugLen {
			t.Errorf("Slug(%q) is longer than %d", tt.in, SlugLen)
		}
	}
}

func TestPaths(t *testing.T) {
	ex := ExerciseFrontMatter{Book: "alg", Chapter: "VIII", Section: 1, Exercise: 7}
	if got, want := ExercisePath("/c", "vi", ex), "/c/content/vi/alg/VIII/exercises/s1/07.md"; got != want {
		t.Errorf("ExercisePath = %s, want %s", got, want)
	}
	// Appendix 1 numbers its exercises from one as § 1 does, so the two sets
	// would land on each other if the directory did not say which is which.
	ex.Appendix = true
	if got, want := ExercisePath("/c", "vi", ex), "/c/content/vi/alg/VIII/exercises/a1/07.md"; got != want {
		t.Errorf("ExercisePath of an appendix = %s, want %s", got, want)
	}
	got, err := SolutionPath("/c", "en", "alg-viii-s1-ex-7")
	if err != nil {
		t.Fatal(err)
	}
	if want := "/c/content/solutions/en/alg/VIII/s1/07.md"; got != want {
		t.Errorf("SolutionPath = %s, want %s", got, want)
	}
	if _, err := SolutionPath("/c", "en", "not a label"); err == nil {
		t.Error("a bad label made a path")
	}
}

// A person who reads a solution writes down what they found, and what they
// found does not quietly become the verdict.
func TestWhatAReaderFoundIsKeptApartFromTheVerdict(t *testing.T) {
	f, err := ParseFile[SolutionFrontMatter]([]byte(`---
label: alg-viii-s1-ex-6
lang: en
status: partial
parts:
    - id: d
      status: unverified
      reason: it relies on numbered definitions not included in the supplied section
hand_read: "2026-08-13"
found:
    - part d is sound; the definitions it cites were trimmed out of the question,
      which is the context assembly and not the solution
---

The solution.
`))
	if err != nil {
		t.Fatal(err)
	}
	if f.Meta.Status != StatusPartial {
		t.Errorf("the reading moved the status to %q", f.Meta.Status)
	}
	if f.Meta.Parts[0].Status != StatusUnverified {
		t.Errorf("the reading moved part d to %q", f.Meta.Parts[0].Status)
	}
	if f.Meta.HandRead != "2026-08-13" {
		t.Errorf("hand_read read as %q", f.Meta.HandRead)
	}
	if len(f.Meta.Found) != 1 || !strings.Contains(f.Meta.Found[0], "trimmed out") {
		t.Errorf("found read as %v", f.Meta.Found)
	}
}
