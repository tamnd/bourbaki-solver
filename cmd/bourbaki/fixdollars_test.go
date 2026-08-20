package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/textguard"
)

// dollarsCorpus writes one English section, one Vietnamese translation of it,
// and one solution, all three with a display set the other way round. The
// translation carries it because a model handed the delimiters straight back,
// and the solution carries it because until the seam in solve was closed nothing
// on that path put an answer into the corpus's typography.
func dollarsCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(rest, text string) {
		path := filepath.Join(root, filepath.FromSlash(rest))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	en := "\nFor every \\(x\\) in $E$ we have\n\\[ f(x) = 0 \\]\nand nothing else.\n"
	write("content/en/ens/II/05_s5.md", `---
book: ens
chapter: II
section: 5
kind: section
lang: en
content_sha256: `+corpus.ContentSHA256(en)+`
---
`+en)
	vi := "\nVoi moi \\(x\\) trong $E$ ta co\n\\[ f(x) = 0 \\]\nva khong gi khac.\n"
	write("content/vi/ens/II/05_s5.md", `---
book: ens
chapter: II
section: 5
kind: section
lang: vi
content_sha256: `+corpus.ContentSHA256(vi)+`
translated_from: content/en/ens/II/05_s5.md
source_content_sha256: `+corpus.ContentSHA256(en)+`
---
`+vi)
	write("content/solutions/en/ens/II/s5/01.md", `---
label: ens-ii-s5-ex-1
tag: 03IV
lang: en
status: verified
corrections: 1
---

By induction on $n$:
\[ \operatorname{Card}(A) = n \]
with a matrix $\begin{pmatrix} a & b \\[2pt] c & d \end{pmatrix}$ in it.
`)
	return root
}

func TestFixDollarsRepairsATranslationAndMovesItOn(t *testing.T) {
	root := dollarsCorpus(t)

	files, changed, followed, err := repairContent(root, false, "delimiters", textguard.Dollars)
	if err != nil {
		t.Fatal(err)
	}
	if files != 2 || changed != 2 || followed != 1 {
		t.Fatalf("read %d files, changed %d, moved %d on, want 2, 2, 1", files, changed, followed)
	}

	for _, rest := range []string{"content/en/ens/II/05_s5.md", "content/vi/ens/II/05_s5.md"} {
		raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rest)))
		if err != nil {
			t.Fatal(err)
		}
		if strings.Contains(string(raw), `\[`) || strings.Contains(string(raw), `\(`) {
			t.Errorf("%s still carries the other spelling:\n%s", rest, raw)
		}
		if !strings.Contains(string(raw), "$$ f(x) = 0 $$") {
			t.Errorf("%s does not carry the display as the corpus writes one:\n%s", rest, raw)
		}
	}

	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash("content/vi/ens/II/05_s5.md")))
	if err != nil {
		t.Fatal(err)
	}
	want := corpus.ContentSHA256("\nFor every $x$ in $E$ we have\n$$ f(x) = 0 $$\nand nothing else.\n")
	if !strings.Contains(string(raw), "source_content_sha256: "+want) {
		t.Errorf("the translation was not moved on to the English as it now stands:\n%s", raw)
	}
}

// The solutions are the tree the other repairs skip, and the one this fault is
// mostly in.
func TestFixDollarsWalksTheSolutions(t *testing.T) {
	root := dollarsCorpus(t)
	rest := "content/solutions/en/ens/II/s5/01.md"

	var seen, repaired int
	err := eachSolution(root, "", func(path string, f *corpus.File[corpus.SolutionFrontMatter]) error {
		seen++
		body, n := textguard.Dollars(f.Body)
		if n == 0 {
			return nil
		}
		repaired++
		f.Body = body
		return f.Write(path)
	})
	if err != nil {
		t.Fatal(err)
	}
	if seen != 1 || repaired != 1 {
		t.Fatalf("walked %d solutions and repaired %d, want 1 and 1", seen, repaired)
	}

	raw, err := os.ReadFile(filepath.Join(root, filepath.FromSlash(rest)))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `$$ \operatorname{Card}(A) = n $$`) {
		t.Errorf("the display was not turned round:\n%s", raw)
	}
	// The row break of the matrix beside it is not a display and is left as it
	// is, which is the whole of what separates the two.
	if !strings.Contains(string(raw), `\\[2pt]`) {
		t.Errorf("the row break of the matrix was eaten:\n%s", raw)
	}
	// The front matter is untouched, so a solution keeps its judgement.
	if !strings.Contains(string(raw), "status: verified") {
		t.Errorf("the front matter was rewritten:\n%s", raw)
	}
}

// A language filter, since the solutions are laid out by language the way the
// rest of content/ is.
func TestEachSolutionTakesOneLanguage(t *testing.T) {
	root := dollarsCorpus(t)
	var seen int
	count := func(path string, f *corpus.File[corpus.SolutionFrontMatter]) error { seen++; return nil }
	if err := eachSolution(root, "vi", count); err != nil {
		t.Fatal(err)
	}
	if seen != 0 {
		t.Errorf("walked %d Vietnamese solutions, and there are none", seen)
	}
	if err := eachSolution(root, "en", count); err != nil {
		t.Fatal(err)
	}
	if seen != 1 {
		t.Errorf("walked %d English solutions, want 1", seen)
	}
}

func TestFixDollarsCheckWritesNothing(t *testing.T) {
	root := dollarsCorpus(t)
	path := filepath.Join(root, filepath.FromSlash("content/vi/ens/II/05_s5.md"))
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, _, err := repairContent(root, true, "delimiters", textguard.Dollars); err != nil {
		t.Fatal(err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if string(before) != string(after) {
		t.Error("-check wrote to the file")
	}
}
