package main

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/assemble"
	"github.com/tamnd/bourbaki-solver/corpus"
)

// A corpus of one volume, one chapter, one §, three pages. It is the smallest
// thing the stage can be run against end to end, and it is run against the same
// three files a real run is: manifests/books.yaml, manifests/toc/ and
// pages/.
func smallCorpus(t *testing.T) string {
	t.Helper()
	root := t.TempDir()

	books := &corpus.BooksManifest{}
	books.Upsert(corpus.Book{
		ID: "alg-viii", Book: "alg", Lang: "en", Title: "Algebra, Chapter 8", Edition: "2023, Springer Nature",
		Chapters: []string{"VIII"}, Pages: 4, Nature: "digital", Extraction: "native",
	})
	if err := books.Save(root); err != nil {
		t.Fatal(err)
	}

	toc := &corpus.TOCManifest{}
	toc.Upsert(corpus.BookTOC{ID: "alg-viii", Grammar: "head-label", Chapters: []corpus.Chapter{{
		Book: "alg-viii", Numeral: "VIII", Title: "Semisimple Modules and Rings", Page: 1, PDFPage: 18,
		Sections: []corpus.Section{{
			Number: 1, Title: "Artinian Modules and Noetherian Modules", Page: 1, PDFPage: 18,
			Subsections: []corpus.Subsection{{Number: 1, Title: "Artinian Modules", Page: 1, PDFPage: 18}},
			Exercises:   &corpus.Locator{Page: 3, PDFPage: 20},
		}},
	}}})
	if err := toc.Save(root); err != nil {
		t.Fatal(err)
	}

	bodies := map[int]string{
		18: "## CHAPTER VIII SEMISIMPLE MODULES AND RINGS\n\n" +
			"## § 1. ARTINIAN MODULES AND NOETHERIAN MODULES\n\n" +
			"### 1. Artinian Modules\n\n" +
			"**Definition 1.** — An A-module M is said to be Artinian if every nonempty set of submodules has a minimal element.",
		19: "**Proposition 1.** — Let A be a ring. The ring A is left Artinian.",
		20: "### Exercises\n\n1) Let A be a ring. Show that A is left Artinian.\n\n" +
			`$\P 2)$ Let K be a field and V a K-vector space.`,
		21: "# BIBLIOGRAPHY",
	}
	dir := corpus.PagesDir(root, "alg-viii")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for n, body := range bodies {
		f := corpus.PageFile{Meta: corpus.PageFrontMatter{
			Book: "alg-viii", PDFPage: n, Method: corpus.MethodNative,
		}, Body: body}
		if n < 21 {
			f.Meta.PageLabel = corpus.PageLabel{Book: "A", Chapter: "VIII", Page: n - 17}.String()
		}
		out, err := f.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(corpus.PagePath(root, "alg-viii", n), out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	return root
}

func TestRunAssemble(t *testing.T) {
	root := smallCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	dir := filepath.Join(root, "content", "en", "alg", "VIII")
	names, err := filepath.Glob(filepath.Join(dir, "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("wrote %v, want the front matter and § 1", names)
	}
	sec, err := corpus.ReadFile[corpus.SectionFrontMatter](
		filepath.Join(dir, "01_s1_artinian_modules_and_noetherian_modules.md"))
	if err != nil {
		t.Fatal(err)
	}
	if sec.Meta.BookTitle != "Algebra" {
		t.Errorf("book_title = %q, want the name of the Book and not of the volume", sec.Meta.BookTitle)
	}
	if sec.Meta.Statements != 2 || sec.Meta.Exercises != 2 {
		t.Errorf("§ 1 reports %d statements and %d exercises", sec.Meta.Statements, sec.Meta.Exercises)
	}
	if !strings.Contains(sec.Body, "See the [exercises for § 1](exercises/s1/).") {
		t.Errorf("§ 1 does not say where its exercises went:\n%s", sec.Body)
	}
	if sec.Meta.PDFPages != "0018-0020" || sec.Meta.BookPages != "A VIII.1-A VIII.3" {
		t.Errorf("§ 1 covers %q, %q", sec.Meta.PDFPages, sec.Meta.BookPages)
	}
	if sec.Meta.ContentSHA256 != corpus.ContentSHA256(sec.Body) {
		t.Error("the file does not hash to what its front matter says")
	}

	m, err := corpus.LoadSections(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Books) != 1 || len(m.Books[0].Chapters) != 1 {
		t.Fatalf("the manifest is %+v", m)
	}
	recs := m.Books[0].Chapters[0].Sections
	if len(recs) != 2 || recs[0].Kind != corpus.KindFront || recs[1].Kind != corpus.KindSection {
		t.Fatalf("the manifest records %+v", recs)
	}
	if recs[1].Label != "alg-viii-s1" || recs[1].Path != "content/en/alg/VIII/01_s1_artinian_modules_and_noetherian_modules.md" {
		t.Errorf("§ 1 is recorded as %+v", recs[1])
	}

	ex, err := corpus.ReadFile[corpus.ExerciseFrontMatter](
		filepath.Join(dir, "exercises", "s1", "02.md"))
	if err != nil {
		t.Fatal(err)
	}
	if ex.Meta.Label != "alg-viii-s1-ex-2" || !ex.Meta.Starred || ex.Meta.PDFPage != 20 {
		t.Errorf("exercise 2 is %+v", ex.Meta)
	}
	if !strings.HasPrefix(ex.Body, "Let K be a field") {
		t.Errorf("exercise 2 reads %q", ex.Body)
	}

	xm, err := corpus.LoadExercises(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(xm.Books) != 1 || len(xm.Books[0].Chapters) != 1 {
		t.Fatalf("the exercise manifest is %+v", xm)
	}
	cx := xm.Books[0].Chapters[0]
	if cx.Total != 2 || len(cx.Section) != 1 {
		t.Fatalf("the chapter records %+v", cx)
	}
	if s := cx.Section[0]; s.Dir != "s1" || s.Count != 2 || s.First != 1 || s.Last != 2 ||
		len(s.Gaps) != 0 || s.Starred != 1 {
		t.Errorf("§ 1 is recorded as %+v", s)
	}

	// The whole stage is meant to be a pure function of the pages and the
	// contents, and -check is how that is known rather than hoped.
	if err := runAssemble([]string{"-book", "alg-viii", "-check", "-q"}); err != nil {
		t.Fatalf("a second run differs from the first: %v", err)
	}
}

// A volume being read a few dozen pages a day is assembled chapter by chapter as
// the chapters finish, and the ones still being read are skipped rather than
// stopping the run. Without that, Theory of Sets would hold chapter I back for a
// week waiting on chapter IV, and nothing downstream, no tag, no reference, no
// translation, could touch a chapter that was finished on the first day.
//
// The second chapter here is in the table of contents and has not one page read,
// which is what a volume looks like part way through.
func TestAssemblePartialSkipsTheChapterStillBeingRead(t *testing.T) {
	root := smallCorpus(t)
	addUnreadChapter(t, root)
	t.Setenv("BOURBAKI_CORPUS", root)

	// Without the flag the run stops, because a volume whose pages are all in
	// and which still will not assemble is a fault and not a state to carry on
	// from.
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err == nil {
		t.Fatal("the run passed over an unread chapter without being asked to")
	}

	if err := runAssemble([]string{"-book", "alg-viii", "-partial", "-q"}); err != nil {
		t.Fatal(err)
	}
	names, err := filepath.Glob(filepath.Join(root, "content", "en", "alg", "VIII", "*.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(names) != 2 {
		t.Fatalf("chapter VIII came out as %v, want the front matter and § 1", names)
	}
	if _, err := os.Stat(filepath.Join(root, "content", "en", "alg", "IX")); !os.IsNotExist(err) {
		t.Errorf("chapter IX was written from pages nobody has read: %v", err)
	}
	m, err := corpus.LoadSections(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Books) != 1 || len(m.Books[0].Chapters) != 1 || m.Books[0].Chapters[0].Chapter != "VIII" {
		t.Fatalf("the manifest records %+v", m.Books)
	}

	// -check is what CI runs, and it has to agree with what -partial wrote,
	// or every push on a volume in the middle of a read reports the corpus as
	// out of date.
	if err := runAssemble([]string{"-book", "alg-viii", "-partial", "-check", "-q"}); err != nil {
		t.Fatalf("checking what -partial wrote differs from it: %v", err)
	}
}

// A page missing out of the middle of a chapter holds that chapter back exactly
// as an unread chapter is held back. The page here is the last of § 1, so the
// chapter would otherwise assemble short by a page and nothing would say so.
func TestAssemblePartialHoldsAChapterWithAHoleInIt(t *testing.T) {
	root := smallCorpus(t)
	addUnreadChapter(t, root)
	if err := os.Remove(corpus.PagePath(root, "alg-viii", 20)); err != nil {
		t.Fatal(err)
	}
	t.Setenv("BOURBAKI_CORPUS", root)
	err := runAssemble([]string{"-book", "alg-viii", "-partial", "-q"})
	if err == nil {
		t.Fatal("a volume with no chapter read through assembled anyway")
	}
	if !strings.Contains(err.Error(), "no chapter") {
		t.Errorf("the run stopped on %v", err)
	}
}

// addUnreadChapter puts a second chapter in the table of contents and leaves its
// pages unwritten, so that the first chapter is read through and the volume is
// not.
func addUnreadChapter(t *testing.T, root string) {
	t.Helper()
	books, err := corpus.LoadBooks(root)
	if err != nil {
		t.Fatal(err)
	}
	b, _ := books.Get("alg-viii")
	nb := *b
	nb.Chapters = []string{"VIII", "IX"}
	nb.Pages = 25
	books.Upsert(nb)
	if err := books.Save(root); err != nil {
		t.Fatal(err)
	}

	toc, err := corpus.LoadTOC(root)
	if err != nil {
		t.Fatal(err)
	}
	bt, _ := toc.Get("alg-viii")
	nt := *bt
	nt.Chapters = append(append([]corpus.Chapter{}, bt.Chapters...), corpus.Chapter{
		Book: "alg-viii", Numeral: "IX", Title: "Sesquilinear and Quadratic Forms", Page: 5, PDFPage: 22,
		Sections: []corpus.Section{{
			Number: 1, Title: "Sesquilinear Forms", Page: 5, PDFPage: 22,
			Subsections: []corpus.Subsection{{Number: 1, Title: "Sesquilinear Forms", Page: 5, PDFPage: 22}},
		}},
	})
	toc.Upsert(nt)
	if err := toc.Save(root); err != nil {
		t.Fatal(err)
	}
}

// readChapterIX writes the four pages of the second chapter, so that a volume
// which addUnreadChapter left half read is read through and assembles whole.
func readChapterIX(t *testing.T, root string) {
	t.Helper()
	bodies := map[int]string{
		22: "## CHAPTER IX SESQUILINEAR AND QUADRATIC FORMS\n\n" +
			"## § 1. SESQUILINEAR FORMS\n\n" +
			"### 1. Sesquilinear Forms\n\n" +
			"**Definition 1.** — Let A be a ring with an involution.",
		23: "**Proposition 1.** — Every sesquilinear form is a linear form in its first argument.",
		24: "**Proposition 2.** — The set of sesquilinear forms is a module.",
		25: "# BIBLIOGRAPHY",
	}
	for n, body := range bodies {
		f := corpus.PageFile{Meta: corpus.PageFrontMatter{
			Book: "alg-viii", PDFPage: n, Method: corpus.MethodNative,
		}, Body: body}
		if n < 25 {
			f.Meta.PageLabel = corpus.PageLabel{Book: "A", Chapter: "IX", Page: n - 21}.String()
		}
		out, err := f.Bytes()
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(corpus.PagePath(root, "alg-viii", n), out, 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// The manifests are written a volume at a time, and for as long as they were
// written out of the chapters the run walked, a partial run took every chapter
// it skipped out of them. That is what happened to chapter IV of Theory of Sets:
// content/ was untouched and still right, the command reported the skip as
// ordinary progress, and the only sign was 31 references two rules downstream
// that no longer resolved.
//
// This is that run. Both chapters assemble, a page in the middle of the second
// goes missing the way a page goes missing when the table of contents grows, and
// the partial run that follows has to leave the second chapter's accounting
// exactly as it found it.
func TestAssemblePartialCarriesASkippedChapterThroughTheManifests(t *testing.T) {
	root := smallCorpus(t)
	addUnreadChapter(t, root)
	readChapterIX(t, root)
	t.Setenv("BOURBAKI_CORPUS", root)

	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	before, err := corpus.LoadSections(root)
	if err != nil {
		t.Fatal(err)
	}
	whole, ok := before.Get("alg-viii")
	if !ok || len(whole.Chapters) != 2 {
		t.Fatalf("the full run recorded %+v", before.Books)
	}
	beforex, err := corpus.LoadExercises(root)
	if err != nil {
		t.Fatal(err)
	}
	wholex, _ := beforex.Get("alg-viii")

	if err := os.Remove(corpus.PagePath(root, "alg-viii", 23)); err != nil {
		t.Fatal(err)
	}
	if err := runAssemble([]string{"-book", "alg-viii", "-partial", "-q"}); err != nil {
		t.Fatal(err)
	}

	after, err := corpus.LoadSections(root)
	if err != nil {
		t.Fatal(err)
	}
	part, _ := after.Get("alg-viii")
	if len(part.Chapters) != 2 {
		t.Fatalf("the partial run left %+v, want both chapters", part.Chapters)
	}
	// The order is the table of contents' order, so a volume assembled chapter
	// by chapter reads the same as one assembled at once.
	if part.Chapters[0].Chapter != "VIII" || part.Chapters[1].Chapter != "IX" {
		t.Fatalf("the chapters came back as %q and %q",
			part.Chapters[0].Chapter, part.Chapters[1].Chapter)
	}
	if !reflect.DeepEqual(part.Chapters[1], whole.Chapters[1]) {
		t.Errorf("chapter IX was rewritten by a run that did not assemble it:\ngot  %+v\nwant %+v",
			part.Chapters[1], whole.Chapters[1])
	}

	afterx, err := corpus.LoadExercises(root)
	if err != nil {
		t.Fatal(err)
	}
	partx, _ := afterx.Get("alg-viii")
	if !reflect.DeepEqual(partx, wholex) {
		t.Errorf("the exercise manifest lost the skipped chapter:\ngot  %+v\nwant %+v", partx, wholex)
	}

	// The files themselves were never the problem and have to stay that way: a
	// run that skips a chapter sweeps nothing out from under it.
	if _, err := os.Stat(filepath.Join(root, "content", "en", "alg", "IX",
		"01_s1_sesquilinear_forms.md")); err != nil {
		t.Errorf("the partial run swept the chapter it skipped: %v", err)
	}
}

// A chapter can be read through and still not assemble, and until this it took
// the whole volume down with it. Integration I to VI is the case that found it:
// chapter I and chapter II are whole, chapter III is short one no. of what the
// contents lists, and 487 pages of reading produced nothing at all because of
// one heading.
//
// Here chapter IX is read through and the contents lists a no. its pages do not
// carry. The full run has to stop on it, since a volume with all its pages in
// and a chapter that will not assemble is a fault to fix. The partial run has to
// write chapter VIII, name chapter IX with the reason, and leave chapter IX's
// accounting exactly as the run that did assemble it left it.
func TestAssemblePartialSkipsAChapterThatWillNotAssemble(t *testing.T) {
	root := smallCorpus(t)
	addUnreadChapter(t, root)
	readChapterIX(t, root)
	t.Setenv("BOURBAKI_CORPUS", root)

	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	before, err := corpus.LoadSections(root)
	if err != nil {
		t.Fatal(err)
	}
	whole, _ := before.Get("alg-viii")

	// The contents grows a no. 2 that no page opens, which is what a heading the
	// reading dropped looks like from here.
	toc, err := corpus.LoadTOC(root)
	if err != nil {
		t.Fatal(err)
	}
	bt, _ := toc.Get("alg-viii")
	nt := *bt
	nt.Chapters = append([]corpus.Chapter{}, bt.Chapters...)
	ix := nt.Chapters[1]
	ix.Sections = []corpus.Section{{
		Number: 1, Title: "Sesquilinear Forms", Page: 5, PDFPage: 22,
		Subsections: []corpus.Subsection{
			{Number: 1, Title: "Sesquilinear Forms", Page: 5, PDFPage: 22},
			{Number: 2, Title: "Quadratic Forms", Page: 6, PDFPage: 23},
		},
	}}
	nt.Chapters[1] = ix
	toc.Upsert(nt)
	if err := toc.Save(root); err != nil {
		t.Fatal(err)
	}

	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err == nil {
		t.Fatal("a full run walked past a chapter that does not assemble")
	}
	if err := runAssemble([]string{"-book", "alg-viii", "-partial", "-q"}); err != nil {
		t.Fatalf("one chapter that will not assemble stopped the whole volume: %v", err)
	}

	if _, err := os.Stat(filepath.Join(root, "content", "en", "alg", "VIII",
		"01_s1_artinian_modules_and_noetherian_modules.md")); err != nil {
		t.Errorf("chapter VIII was held back by chapter IX: %v", err)
	}
	after, err := corpus.LoadSections(root)
	if err != nil {
		t.Fatal(err)
	}
	part, _ := after.Get("alg-viii")
	if len(part.Chapters) != 2 {
		t.Fatalf("the partial run left %+v, want both chapters", part.Chapters)
	}
	if !reflect.DeepEqual(part.Chapters[1], whole.Chapters[1]) {
		t.Errorf("chapter IX was rewritten by a run that refused it:\ngot  %+v\nwant %+v",
			part.Chapters[1], whole.Chapters[1])
	}
	// The files of the refused chapter are not swept either. They are what the
	// last good run wrote and they are still right.
	if _, err := os.Stat(filepath.Join(root, "content", "en", "alg", "IX",
		"01_s1_sesquilinear_forms.md")); err != nil {
		t.Errorf("the partial run swept the chapter it refused: %v", err)
	}
}

// An erratum is written into a manifest and stamped into the assembled file, and
// it has to survive the assembler being run again. Written into content/ by hand
// it would not: content/ is a pure function of the pages and the next run wipes
// it, which is how the first one was found, by CI reporting the file as out of
// date on the push that added it.
func TestAnErratumSurvivesAReassembly(t *testing.T) {
	root := smallCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	m := &corpus.ErrataManifest{Entries: []corpus.LabelErrata{{
		Label: "alg-viii-s1-ex-2", Lang: "en",
		Errata: []corpus.Erratum{{Says: "a K-vector space", Read: "a K-algebra", Why: "V is multiplied"}},
	}}}
	b, err := m.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corpus.ErrataPath(root), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "content", "en", "alg", "VIII", "exercises", "s1", "02.md")
	ex, err := corpus.ReadFile[corpus.ExerciseFrontMatter](path)
	if err != nil {
		t.Fatal(err)
	}
	if len(ex.Meta.Errata) != 1 || ex.Meta.Errata[0].Read != "a K-algebra" {
		t.Fatalf("exercise 2 carries %+v", ex.Meta.Errata)
	}
	one, err := corpus.ReadFile[corpus.ExerciseFrontMatter](
		filepath.Join(root, "content", "en", "alg", "VIII", "exercises", "s1", "01.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(one.Meta.Errata) != 0 {
		t.Errorf("exercise 1 was given exercise 2's erratum: %+v", one.Meta.Errata)
	}
	// The printed words are what the corpus holds. The correction stands beside
	// them and does not touch them.
	if !strings.Contains(ex.Body, "a K-vector space") {
		t.Errorf("the printed text was edited: %q", ex.Body)
	}
	if err := runAssemble([]string{"-book", "alg-viii", "-check", "-q"}); err != nil {
		t.Fatalf("the erratum did not survive the second run: %v", err)
	}
}

// An erratum against a label nothing is called is a correction that will never
// be seen again, and it looks exactly like a correction that was applied.
func TestAnErratumAgainstNothingStopsTheRun(t *testing.T) {
	root := smallCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	m := &corpus.ErrataManifest{Entries: []corpus.LabelErrata{
		{Label: "alg-viii-s1-ex-9", Lang: "en",
			Errata: []corpus.Erratum{{Says: "a", Read: "b", Why: "c"}}},
	}}
	b, err := m.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corpus.ErrataPath(root), b, 0o644); err != nil {
		t.Fatal(err)
	}
	err = runAssemble([]string{"-book", "alg-viii", "-q"})
	if err == nil {
		t.Fatal("an erratum on an exercise that does not exist should stop the run")
	}
	if !strings.Contains(err.Error(), "alg-viii-s1-ex-9") {
		t.Errorf("the error does not say which: %v", err)
	}

	// A chapter this run did not touch is not this run's business. The corpus
	// is twenty-six chapters across eight volumes and each is assembled on its
	// own, so judging another volume's labels here would make every run fail
	// until every volume was in.
	m.Entries[0].Label = "alg-i-s1-ex-9"
	if b, err = m.Bytes(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corpus.ErrataPath(root), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Errorf("another chapter's erratum stopped this one: %v", err)
	}
}

// A misprint is as often in the prose between two statements as in a statement,
// so an erratum goes on the § as well as on an exercise. § 5 of chapter VIII is
// the one that made this necessary: it says "Chap. VII, §13, no. 1" in a
// paragraph belonging to no statement at all, and chapter VII has five sections.
func TestAnErratumGoesOnASectionToo(t *testing.T) {
	root := smallCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	m := &corpus.ErrataManifest{Entries: []corpus.LabelErrata{{
		Label: "alg-viii-s1", Lang: "en",
		Errata: []corpus.Erratum{{Says: "a minimal element", Read: "a maximal element",
			Why: "The § is about Artinian modules and prints the wrong end of the order."}},
	}}}
	b, err := m.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(corpus.ErrataPath(root), b, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "content", "en", "alg", "VIII",
		"01_s1_artinian_modules_and_noetherian_modules.md")
	sec, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
	if err != nil {
		t.Fatal(err)
	}
	if len(sec.Meta.Errata) != 1 || sec.Meta.Errata[0].Read != "a maximal element" {
		t.Fatalf("§ 1 carries %+v", sec.Meta.Errata)
	}
	if !strings.Contains(sec.Body, "a minimal element") {
		t.Errorf("the printed text was edited:\n%s", sec.Body)
	}
	if err := runAssemble([]string{"-book", "alg-viii", "-check", "-q"}); err != nil {
		t.Errorf("the erratum did not survive the second run: %v", err)
	}
}

// The label being right is half of it. An erratum quotes the words it is
// correcting, and a quotation the file does not have is the same quiet failure
// as a label nothing is called, and worse: the graph is read out of the
// corrected body, so the reference stays exactly as broken as it was and nothing
// says why.
func TestAnErratumThatQuotesNothingStopsTheRun(t *testing.T) {
	root := smallCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	for _, label := range []string{"alg-viii-s1", "alg-viii-s1-ex-2"} {
		t.Run(label, func(t *testing.T) {
			m := &corpus.ErrataManifest{Entries: []corpus.LabelErrata{{
				Label: label, Lang: "en",
				Errata: []corpus.Erratum{{Says: "a sentence the page does not have",
					Read: "something else", Why: "a quotation that matches nothing"}},
			}}}
			b, err := m.Bytes()
			if err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(corpus.ErrataPath(root), b, 0o644); err != nil {
				t.Fatal(err)
			}
			err = runAssemble([]string{"-book", "alg-viii", "-q"})
			if err == nil {
				t.Fatal("an erratum quoting nothing should stop the run")
			}
			if !strings.Contains(err.Error(), label) ||
				!strings.Contains(err.Error(), "a sentence the page does not have") {
				t.Errorf("the error does not say what went wrong: %v", err)
			}
		})
	}
}

// A section file is named for its title and an exercise file for its number, so
// correcting either renames or drops a file. The old one has to go, or it sits
// there for ever looking like part of the book.
func TestRunAssembleRemovesAFileItDidNotWrite(t *testing.T) {
	root := smallCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	dir := filepath.Join(root, "content", "en", "alg", "VIII")
	if err := os.MkdirAll(filepath.Join(dir, "exercises", "s1"), 0o755); err != nil {
		t.Fatal(err)
	}
	old := filepath.Join(dir, "01_s1_artinian_modules.md")
	if err := os.WriteFile(old, []byte("stale\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// § 1 has two exercises, so a third file is one this run did not write.
	ex := filepath.Join(dir, "exercises", "s1", "03.md")
	if err := os.WriteFile(ex, []byte("an exercise the book does not print\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(old); !os.IsNotExist(err) {
		t.Error("the file with the old name is still there")
	}
	if _, err := os.Stat(ex); !os.IsNotExist(err) {
		t.Error("the exercise file nothing wrote is still there")
	}
	if _, err := os.Stat(filepath.Join(dir, "exercises", "s1", "02.md")); err != nil {
		t.Errorf("exercise 2 was swept: %v", err)
	}
}

// -check writes nothing and says what differs. It is what CI runs, and CI has
// no PDFs and nothing to compare against but the repository.
func TestRunAssembleCheckReportsADifference(t *testing.T) {
	root := smallCorpus(t)
	t.Setenv("BOURBAKI_CORPUS", root)
	if err := runAssemble([]string{"-book", "alg-viii", "-q"}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "content", "en", "alg", "VIII",
		"01_s1_artinian_modules_and_noetherian_modules.md")
	before, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(before, []byte("\nan edit nobody made upstream\n")...), 0o644); err != nil {
		t.Fatal(err)
	}
	err = runAssemble([]string{"-book", "alg-viii", "-check", "-q"})
	if err == nil {
		t.Fatal("an edited file should be reported")
	}
	if !strings.Contains(err.Error(), "01_s1_artinian_modules_and_noetherian_modules.md") {
		t.Errorf("the error does not name the file: %v", err)
	}
	after, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(after) == len(before) {
		t.Error("-check wrote the file back")
	}
}

// Two page files claiming the same PDF page means one of them is a leftover,
// and assembling both of them would put the page in twice.
func TestReadPagesRefusesTwoFilesForOnePage(t *testing.T) {
	root := smallCorpus(t)
	f := corpus.PageFile{Meta: corpus.PageFrontMatter{
		Book: "alg-viii", PDFPage: 19, Method: corpus.MethodNative,
	}, Body: "a second reading of page 19"}
	out, err := f.Bytes()
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(corpus.PagesDir(root, "alg-viii"), "0019_old.md"), out, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := readPages(root, "alg-viii"); err == nil {
		t.Fatal("two files for one page should be an error")
	}
}

// Bourbaki numbers the exercises of a § from one straight through, so a gap is
// never something the book does. It is a page that never got read, and writing
// the corpus round it would leave it quietly short of an exercise.
func TestWriteExercisesRefusesAGap(t *testing.T) {
	p := assemble.Piece{Section: corpus.Section{Number: 1}}
	for _, n := range []int{1, 2, 4} {
		e := corpus.Exercise{Body: "a body"}
		e.Meta = corpus.ExerciseFrontMatter{Book: "alg", Chapter: "VIII", Section: 1, Exercise: n}
		p.Exercises = append(p.Exercises, e)
	}
	cx := corpus.ChapterExercises{Chapter: "VIII"}
	err := writeExercises(t.TempDir(), "en", p, map[string][]byte{}, &cx, nil, nil, nil)
	if err == nil {
		t.Fatal("a § missing exercise 3 should be an error")
	}
	if !strings.Contains(err.Error(), "[3]") {
		t.Errorf("the error does not say which is missing: %v", err)
	}
}

func TestPageRange(t *testing.T) {
	cases := []struct{ first, last, want string }{
		{"A VIII.1", "A VIII.23", "A VIII.1-A VIII.23"},
		{"A VIII.7", "A VIII.7", "A VIII.7"},
		{"", "A VIII.23", ""},
		{"A VIII.1", "", ""},
	}
	for _, c := range cases {
		if got := pageRange(c.first, c.last); got != c.want {
			t.Errorf("pageRange(%q, %q) = %q, want %q", c.first, c.last, got, c.want)
		}
	}
}

// A volume that labels its pages writes the label, and one paginated straight
// through writes the folio bare. Theory of Sets is the second kind: its pages
// carry a number at the foot and nothing else, so a § of it used to come out
// with an empty book_pages, no printed page in the reference index, and every
// one of the 223 references Algebra VIII makes to it by page went unresolved.
func TestBookPagesWritesTheFolioWhereThereIsNoPageLabel(t *testing.T) {
	cases := []struct {
		runs []assemble.Run
		want string
	}{
		{[]assemble.Run{{FirstLabel: "A VIII.1", LastLabel: "A VIII.23"}}, "A VIII.1-A VIII.23"},
		{[]assemble.Run{{FirstFolio: 15, LastFolio: 23}}, "15-23"},
		{[]assemble.Run{{FirstFolio: 15, LastFolio: 23}, {FirstFolio: 56, LastFolio: 56}}, "15-23, 56"},
		// A page with neither is a page nothing is known about, and a guess here
		// would put the § on a page it is not printed on.
		{[]assemble.Run{{First: 18, Last: 20}}, ""},
	}
	for _, c := range cases {
		if got := bookPages(c.runs); got != c.want {
			t.Errorf("bookPages(%+v) = %q, want %q", c.runs, got, c.want)
		}
	}
}

func TestKindOfAndLabelOf(t *testing.T) {
	cases := []struct {
		piece assemble.Piece
		kind  string
		label string
	}{
		{assemble.Piece{Front: true}, corpus.KindFront, ""},
		{assemble.Piece{Historical: true}, corpus.KindHistorical, ""},
		{assemble.Piece{Section: corpus.Section{Number: 3}}, corpus.KindSection, "alg-viii-s3"},
		{assemble.Piece{Section: corpus.Section{Number: 2, Appendix: true}}, corpus.KindAppendix, "alg-viii-a2"},
	}
	for _, c := range cases {
		if got := kindOf(c.piece); got != c.kind {
			t.Errorf("kindOf(%s) = %q, want %q", c.piece.Name(), got, c.kind)
		}
		if got := labelOf("alg", "VIII", c.piece); got != c.label {
			t.Errorf("labelOf(%s) = %q, want %q", c.piece.Name(), got, c.label)
		}
	}
}
