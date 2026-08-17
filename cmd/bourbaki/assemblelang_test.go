package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// frenchCorpus is smallCorpus set in French, which is the printing the four
// French volumes of the corpus are read from.
func frenchCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	books := &corpus.BooksManifest{}
	books.Upsert(corpus.Book{
		ID: "alg-viii-fr", Book: "alg", Lang: "fr", Title: "Algèbre, Chapitre 8", Edition: "2012, Springer",
		Chapters: []string{"VIII"}, Pages: 4, Nature: "digital", Extraction: "native",
	})
	if err := books.Save(root); err != nil {
		t.Fatal(err)
	}

	toc := &corpus.TOCManifest{}
	toc.Upsert(corpus.BookTOC{ID: "alg-viii-fr", Grammar: "head-label", Chapters: []corpus.Chapter{{
		Book: "alg-viii-fr", Numeral: "VIII", Title: "Modules et Anneaux Semi-Simples", Page: 1, PDFPage: 18,
		Sections: []corpus.Section{{
			Number: 1, Title: "Modules Artiniens et Modules Noethériens", Page: 1, PDFPage: 18,
			Subsections: []corpus.Subsection{{Number: 1, Title: "Modules Artiniens", Page: 1, PDFPage: 18}},
			Exercises:   &corpus.Locator{Page: 3, PDFPage: 20},
		}},
	}}})
	if err := toc.Save(root); err != nil {
		t.Fatal(err)
	}

	bodies := map[int]string{
		18: "## CHAPITRE VIII MODULES ET ANNEAUX SEMI-SIMPLES\n\n" +
			"## § 1. MODULES ARTINIENS ET MODULES NOETHÉRIENS\n\n" +
			"### 1. Modules artiniens\n\n" +
			"**Définition 1.** — Un A-module M est dit artinien si tout ensemble non vide de sous-modules admet un élément minimal.",
		19: "**Proposition 1.** — Soit A un anneau. L’anneau A est artinien à gauche.",
		20: "## EXERCICES\n\n1) Soit A un anneau. Montrer que A est artinien à gauche.\n\n" +
			`$\P 2)$ Soient K un corps et V un K-espace vectoriel.`,
		21: "# BIBLIOGRAPHIE",
	}
	dir := corpus.PagesDir(root, "alg-viii-fr")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for n, body := range bodies {
		f := corpus.PageFile{Meta: corpus.PageFrontMatter{
			Book: "alg-viii-fr", PDFPage: n, Method: corpus.MethodNative,
		}, Body: body}
		if n < 21 {
			f.Meta.PageLabel = corpus.PageLabel{Book: "A", Chapter: "VIII", Page: n - 17}.String()
		}
		out, err := f.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(corpus.PagePath(root, "alg-viii-fr", n), out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

// A French volume assembles in French without being told to. The language is
// part of what a volume is and the manifest has said so all along, and -lang
// defaulting to English meant a French volume asked for by name stopped on its
// first page, since the assembler was looking for "## CHAPTER" where the page
// prints "## CHAPITRE".
//
// It cost more than a flag somebody forgot. The audit assembles every book with
// pages to run S09, the rule that says what is committed is what assembly
// writes, and it passed "en" for all of them, so the four French volumes were
// reported as books the assembler stopped on and left out of the rule. They were
// out of it long enough for content/fr/ta/I/exercises/s6/05.md and one section of
// Théories spectrales to go stale against pages that had been re-read under them.
func TestAssembleTakesTheLanguageFromTheBook(t *testing.T) {
	root := frenchCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	if err := runAssemble([]string{"-book", "alg-viii-fr", "-q"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "content", "fr", "alg", "VIII",
		"01_s1_modules_artiniens_et_modules_noetheriens.md")
	f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Meta.Lang != "fr" {
		t.Errorf("lang = %q", f.Meta.Lang)
	}
	if f.Meta.Statements != 2 || f.Meta.Exercises != 2 {
		t.Errorf("the § came out %d statements and %d exercises", f.Meta.Statements, f.Meta.Exercises)
	}
	if _, err := os.Stat(filepath.Join(root, "content", "en")); !os.IsNotExist(err) {
		t.Errorf("a French volume wrote into content/en: %v", err)
	}
	if err := runAssemble([]string{"-book", "alg-viii-fr", "-check", "-q"}); err != nil {
		t.Fatalf("a second run differs from the first: %v", err)
	}
}

// -lang is still a flag and still overrides, and asking for the wrong one still
// stops the run rather than writing a chapter it could not read.
func TestAssembleStopsWhenToldTheWrongLanguage(t *testing.T) {
	root := frenchCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	err := runAssemble([]string{"-book", "alg-viii-fr", "-lang", "en", "-q"})
	if err == nil {
		t.Fatal("a French volume assembled as English")
	}
	if !strings.Contains(err.Error(), "## CHAPTER") {
		t.Errorf("the run stopped with %v", err)
	}
}
