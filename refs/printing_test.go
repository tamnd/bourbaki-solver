package refs

import (
	"os"
	"path/filepath"
	"testing"
)

// twoPrintings is a chapter the corpus holds twice, in English and in French,
// with the two sets of files named as assembly names them: out of the volume
// each was read from, so the French file is not the English file with a
// different directory in front of it.
//
// That is the whole of the fault this fixture is here for. A translation is the
// English file with the language swapped in its path, and the index was built
// on that assumption for every language that is not English. The French is not
// a translation. It is a second printing, it has its own records in
// sections.yaml, and swapping the language into an English path names a file
// nobody wrote.
func twoPrintings(t *testing.T) string {
	t.Helper()
	root := t.TempDir()
	write := func(path, body string) {
		full := filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("manifests/sections.yaml", `books:
    - id: alg-viii
      chapters:
        - chapter: VIII
          sections:
            - kind: section
              section: 1
              path: content/en/alg/VIII/01_s1_artinian_modules.md
              label: alg-viii-s1
              book_pages: A VIII.1 - A VIII.23
    - id: alg-viii-fr
      chapters:
        - chapter: VIII
          sections:
            - kind: section
              section: 1
              path: content/fr/alg/VIII/01_s1_modules_artiniens.md
              label: alg-viii-s1
              book_pages: A VIII.1 - A VIII.23
`)
	write("manifests/exercises.json", `{"books":[{"id":"alg-viii","chapters":[{"chapter":"VIII",
	  "sections":[{"section":1,"label":"alg-viii-s1","dir":"s1","count":0,"first":0,"last":0}]}]}]}`)
	body := func(prose string) string {
		return `---
book: alg
chapter: VIII
section: 1
lang: en
subsections:
    - "no": 1
      page: 1
statements: 1
exercises: 0
---

### 1. A first no.

#### Proposition 1 {#alg-viii-s1-prop-1 .statement tag=0001}

` + prose + `
`
	}
	write("content/en/alg/VIII/01_s1_artinian_modules.md", body("Every ring is a ring."))
	write("content/fr/alg/VIII/01_s1_modules_artiniens.md", body("Tout anneau est un anneau."))
	return root
}

func TestASecondPrintingIsIndexedFromItsOwnFiles(t *testing.T) {
	root := twoPrintings(t)
	for _, lang := range []string{"en", "fr"} {
		ix, err := Load(root, lang)
		if err != nil {
			t.Fatalf("%s: %v", lang, err)
		}
		// One § and not two. Indexing both printings puts every page of the
		// chapter in two sections at once and every page citation then resolves
		// to nothing.
		if len(ix.Sections) != 1 {
			t.Errorf("%s indexed %d sections, want 1", lang, len(ix.Sections))
		}
		if st := ix.Statement("alg-viii-s1-prop-1"); st == nil {
			t.Errorf("%s lost the proposition", lang)
		} else if want := "content/" + lang + "/"; len(st.Path) < len(want) || st.Path[:len(want)] != want {
			t.Errorf("%s read the proposition out of %s", lang, st.Path)
		}
	}
}

// A language the corpus translates into rather than reprints has no records of
// its own, and the English records with the language swapped in are still what
// it wants.
func TestATranslationIsIndexedFromTheEnglishRecords(t *testing.T) {
	root := twoPrintings(t)
	full := filepath.Join(root, "content/vi/alg/VIII/01_s1_artinian_modules.md")
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(root, "content/en/alg/VIII/01_s1_artinian_modules.md"))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(full, b, 0o644); err != nil {
		t.Fatal(err)
	}

	ix, err := Load(root, "vi")
	if err != nil {
		t.Fatal(err)
	}
	if len(ix.Sections) != 1 {
		t.Fatalf("indexed %d sections, want 1", len(ix.Sections))
	}
	st := ix.Statement("alg-viii-s1-prop-1")
	if st == nil {
		t.Fatal("the translation lost the proposition")
	}
	if want := "content/vi/alg/VIII/01_s1_artinian_modules.md"; st.Path != want {
		t.Errorf("read the proposition out of %s, want %s", st.Path, want)
	}
}
