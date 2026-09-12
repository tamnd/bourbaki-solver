package quality

import (
	"fmt"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/prompt"
)

// content/en-mt holds English written by a model out of the French printing.
// The name of the tree is how the file was made, so lang is en in it, the same
// as under content/en, and method and translated_from are what tell the two
// apart.
func TestTheMachineEnglishTreeHoldsEnglish(t *testing.T) {
	d := Doc{
		Path: "content/en-mt/ts/I/00_frontmatter.md", Lang: "en-mt", Kind: KindSection,
		Section: &corpus.SectionFrontMatter{
			Book: "ts", Chapter: "I", Lang: "en", ContentSHA256: "0",
			TranslatedFrom: "content/fr/ts/I/00_frontmatter.md",
		},
	}
	if got := requireSection(d); len(got) != 0 {
		t.Errorf("reported %+v, want nothing", got)
	}
}

// The tree is still read: a French file that has wandered into it is a file in
// the wrong place whatever the tree is called.
func TestAFrenchFileUnderTheMachineEnglishTreeIsReported(t *testing.T) {
	d := Doc{
		Path: "content/en-mt/ts/I/00_frontmatter.md", Lang: "en-mt", Kind: KindSection,
		Section: &corpus.SectionFrontMatter{
			Book: "ts", Chapter: "I", Lang: "fr", ContentSHA256: "0",
		},
	}
	if got := requireSection(d); len(got) != 1 {
		t.Errorf("reported %+v, want the one file in the wrong tree", got)
	}
}

// s12Corpus is one chapter of one book: two § files that the manifest names and
// one Vietnamese translation of the first, which it does not.
func s12Corpus(named []corpus.SectionRecord) *Corpus {
	body := "the body of a section\n"
	sec := func(path string, from string) Doc {
		return Doc{Path: path, Lang: "en", Kind: KindSection, Body: body, head: 1,
			Section: &corpus.SectionFrontMatter{Book: "ens", Chapter: "IV",
				ContentSHA256: corpus.ContentSHA256(body), TranslatedFrom: from}}
	}
	return &Corpus{
		Docs: []Doc{
			sec("content/en/ens/IV/01_s1.md", ""),
			sec("content/en/ens/IV/02_s2.md", ""),
			sec("content/vi/ens/IV/01_s1.md", "content/en/ens/IV/01_s1.md"),
		},
		Sections: &corpus.SectionsManifest{Books: []corpus.BookSections{{
			ID:       "ens",
			Chapters: []corpus.ChapterSections{{Chapter: "IV", Sections: named}},
		}}},
	}
}

func s12Record(path string) corpus.SectionRecord {
	return corpus.SectionRecord{Kind: corpus.KindSection, Path: path,
		ContentSHA256: corpus.ContentSHA256("the body of a section\n")}
}

func TestS12PassesWhenTheManifestNamesEveryFileAndDescribesIt(t *testing.T) {
	c := s12Corpus([]corpus.SectionRecord{
		s12Record("content/en/ens/IV/01_s1.md"),
		s12Record("content/en/ens/IV/02_s2.md"),
	})
	got, err := s12(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %+v, want nothing", got)
	}
}

// This is the fault it was written for. assemble -partial skipped a chapter
// that was not read through and took its entries out with it, and every file of
// that chapter kept its text while nothing that reads the manifest could reach
// it.
// The two indexes are in the manifest, in fields of their own after the
// chapters, and the rule did not read those fields. So every index of the
// corpus, 79 files, was reported as reachable by nothing while its entry sat in
// the manifest beside the chapter it follows.
func TestS12ReadsTheTwoIndexesTheManifestNamesAfterTheChapters(t *testing.T) {
	body := "the body of a section\n"
	index := func(path string) Doc {
		return Doc{Path: path, Lang: "en", Kind: KindSection, Body: body, head: 1,
			Section: &corpus.SectionFrontMatter{Book: "ens", ContentSHA256: corpus.ContentSHA256(body)}}
	}
	c := s12Corpus([]corpus.SectionRecord{
		s12Record("content/en/ens/IV/01_s1.md"),
		s12Record("content/en/ens/IV/02_s2.md"),
	})
	c.Docs = append(c.Docs,
		index("content/en/ens/index_of_notation_i_iv.md"),
		index("content/en/ens/index_of_terminology_i_iv.md"))
	notation := s12Record("content/en/ens/index_of_notation_i_iv.md")
	terminology := s12Record("content/en/ens/index_of_terminology_i_iv.md")
	c.Sections.Books[0].NotationIndex = &notation
	c.Sections.Books[0].TerminologyIndex = &terminology

	got, err := s12(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %+v, want nothing", got)
	}
}

func TestS12ReportsAFileTheManifestDoesNotName(t *testing.T) {
	c := s12Corpus([]corpus.SectionRecord{s12Record("content/en/ens/IV/01_s1.md")})
	got, err := s12(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("reported %+v, want the one file that fell out", got)
	}
	if got[0].File != "content/en/ens/IV/02_s2.md" {
		t.Errorf("named %q", got[0].File)
	}
	if !strings.Contains(got[0].Msg, "no entry in manifests/sections/") {
		t.Errorf("said %q", got[0].Msg)
	}
}

func TestS12ReportsAManifestHashThatIsNotTheBody(t *testing.T) {
	stale := s12Record("content/en/ens/IV/02_s2.md")
	stale.ContentSHA256 = corpus.ContentSHA256("a body this file used to have\n")
	c := s12Corpus([]corpus.SectionRecord{s12Record("content/en/ens/IV/01_s1.md"), stale})
	got, err := s12(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("reported %+v, want the one stale hash", got)
	}
	if !strings.Contains(got[0].Msg, "holds content_sha256") {
		t.Errorf("said %q", got[0].Msg)
	}
}

func TestS12ReportsAnEntryWithNoFileUnderIt(t *testing.T) {
	c := s12Corpus([]corpus.SectionRecord{
		s12Record("content/en/ens/IV/01_s1.md"),
		s12Record("content/en/ens/IV/02_s2.md"),
		s12Record("content/en/ens/IV/03_s3.md"),
	})
	got, err := s12(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].File != "content/en/ens/IV/03_s3.md" {
		t.Fatalf("reported %+v, want the one entry with nothing under it", got)
	}
}

// A translation has no page under it, is not rewritten by an assemble, and is
// not the manifest's to name. The test is that it names a source, and not the
// language, since French is assembled too and is in the manifest.
func TestS12LeavesATranslationOutOfIt(t *testing.T) {
	c := s12Corpus([]corpus.SectionRecord{
		s12Record("content/en/ens/IV/01_s1.md"),
		s12Record("content/en/ens/IV/02_s2.md"),
	})
	got, err := s12(c)
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range got {
		if strings.HasPrefix(f.File, "content/vi/") {
			t.Errorf("the Vietnamese was reported: %+v", f)
		}
	}
}

// s13Corpus is one volume whose contents was read off two pages, pdf 8 and 9,
// with whatever reading and prompt the caller hands over for each.
func s13Corpus(pages ...corpus.PageFile) *Corpus {
	c := &Corpus{
		TOC: &corpus.TOCManifest{Books: []corpus.BookTOC{
			{ID: "ens", ContentsPDFPages: []int{8, 9}},
		}},
		Pages:     map[string][]corpus.PageFile{},
		PagePaths: map[string][]string{},
	}
	for i, p := range pages {
		p.Meta.Book = "ens"
		p.Meta.PDFPage = 8 + i
		c.Pages["ens"] = append(c.Pages["ens"], p)
		c.PagePaths["ens"] = append(c.PagePaths["ens"], fmt.Sprintf("pages/ens/%04d.md", 8+i))
	}
	return c
}

// contentsPage is a page read the way a contents page should be: every line an
// entry, every entry ending in the printed page number it points at.
func contentsPage(sha string) corpus.PageFile {
	return corpus.PageFile{
		Meta: corpus.PageFrontMatter{PromptSHA256: sha},
		Body: "§ 1. Sets ...... 11\n§ 2. Relations ...... 24\n§ 3. Functions ...... 37\n",
	}
}

func TestS13PassesWhenTheContentsWasReadWithTheContentsPrompt(t *testing.T) {
	c := s13Corpus(contentsPage(prompt.ContentsSHA256()), contentsPage(prompt.ContentsSHA256()))
	got, err := s13(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %+v, want nothing", got)
	}
}

// The 60 pages the corpus holds today. The prompt is the wrong one and the
// numbers came through anyway, and a page with its numbers is not damage.
func TestS13LeavesAWrongPromptThatKeptTheNumbersAlone(t *testing.T) {
	c := s13Corpus(contentsPage(prompt.OCRSHA256()), contentsPage(prompt.OCRSHA256()))
	got, err := s13(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %+v, want nothing", got)
	}
}

// This is the fault it was written for: int-i-iv-fr pdf 282 to 284, read as a
// contents and then read again as prose, the numbers gone and nothing saying so.
func TestS13ReportsAContentsPageReadAsProse(t *testing.T) {
	prose := corpus.PageFile{
		Meta: corpus.PageFrontMatter{PromptSHA256: prompt.OCRSHA256()},
		Body: "§ 1. Sets\n§ 2. Relations\n§ 3. Functions\n",
	}
	c := s13Corpus(contentsPage(prompt.ContentsSHA256()), prose)
	got, err := s13(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("reported %+v, want the one page that lost its numbers", got)
	}
	if got[0].File != "pages/ens/0009.md" {
		t.Errorf("named %q", got[0].File)
	}
	if !strings.Contains(got[0].Msg, "0 of its 3 lines") {
		t.Errorf("said %q", got[0].Msg)
	}
}

// The born-digital volumes take their contents off the pdf's own text layer.
// Those pages carry no reading and no prompt hash, and asking them about a
// prompt would report every one of them.
func TestS13DoesNotAskAPageThatWasNeverRead(t *testing.T) {
	c := s13Corpus(contentsPage(prompt.ContentsSHA256()), corpus.PageFile{})
	got, err := s13(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %+v, want nothing", got)
	}
}

// A volume toc build has not run over since the field was added names no pages,
// and the rule has nothing to say about it rather than something wrong.
func TestS13SaysNothingAboutAVolumeWithNoRecordedPages(t *testing.T) {
	c := s13Corpus(contentsPage(prompt.OCRSHA256()))
	c.TOC.Books[0].ContentsPDFPages = nil
	got, err := s13(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %+v, want nothing", got)
	}
}

// S14 runs the ocr rules over what is committed, so a page short enough to be a
// truncated answer is reported and a page that reads like a page is not. The
// rules themselves are tested in package ocr; what is tested here is that the
// audit reaches them, walks every volume, and skips what a read-time run skips.
func TestS14(t *testing.T) {
	page := func(n int, method corpus.PageMethod, body string) corpus.PageFile {
		return corpus.PageFile{
			Meta: corpus.PageFrontMatter{Book: "alg-x-fr", PDFPage: n, Method: method},
			Body: body,
		}
	}
	// Four hundred characters is the thinnest page of either volume that
	// carries text, and ocr.MinChars is two hundred.
	full := strings.Repeat("Soit E un espace vectoriel topologique sur un corps value. ", 12)
	c := &Corpus{
		Root:  t.TempDir(),
		Books: &corpus.BooksManifest{Books: []corpus.Book{{ID: "alg-x-fr", Book: "alg", Pages: 3}}},
		Pages: map[string][]corpus.PageFile{"alg-x-fr": {
			page(40, corpus.MethodOCR, full),
			page(41, corpus.MethodOCR, "a line and nothing else\n"),
			page(42, corpus.MethodBlank, ""),
		}},
		PagePaths: map[string][]string{"alg-x-fr": {
			"pages/alg-x-fr/0001.md", "pages/alg-x-fr/0002.md", "pages/alg-x-fr/0003.md",
		}},
	}
	got, err := s14(c)
	if err != nil {
		t.Fatalf("the rule returned an error: %v", err)
	}
	if len(got) != 1 {
		t.Fatalf("gave %d findings, want 1: %v", len(got), got)
	}
	if got[0].File != "pages/alg-x-fr/0002.md" {
		t.Errorf("the finding is on %s, want pages/alg-x-fr/0002.md", got[0].File)
	}
	if !strings.Contains(got[0].Msg, "refused today") {
		t.Errorf("the finding does not say what it is: %s", got[0].Msg)
	}

	// A volume with no pages read yet is work not yet done and not a finding,
	// which is what every other rule in this group does too.
	c.Pages["alg-x-fr"] = nil
	if got, err := s14(c); err != nil || len(got) != 0 {
		t.Errorf("an unread volume was reported: %v %v", got, err)
	}
}

// s16Corpus is one volume whose pages carry the running heads handed over.
func s16Corpus(heads ...string) *Corpus {
	c := &Corpus{
		Books:     &corpus.BooksManifest{Books: []corpus.Book{{ID: "ens"}}},
		Pages:     map[string][]corpus.PageFile{},
		PagePaths: map[string][]string{},
	}
	for i, h := range heads {
		c.Pages["ens"] = append(c.Pages["ens"], corpus.PageFile{
			Meta: corpus.PageFrontMatter{Book: "ens", PDFPage: 8 + i, RunningHead: h}})
		c.PagePaths["ens"] = append(c.PagePaths["ens"], fmt.Sprintf("pages/ens/%04d.md", 8+i))
	}
	return c
}

// The longest head any of the 44 volumes prints, and the longest the corpus
// holds in each of the two languages. A rule that reported these would be
// saying something untrue about every page of Topology I.
func TestS16LeavesTheLongestHeadThePrintingsHave(t *testing.T) {
	c := s16Corpus(
		"6. TOPOLOGICAL GROUPS WITH OPERATORS; TOPOLOGICAL RINGS, DIVISION RINGS AND FIELDS",
		"INTEGRAL FORMULA FOR THE REMAINDER IN TAYLOR'S FORMULA; PRIMITIVES OF HIGHER ORDER",
		"n° 4 SOUS-ALGÈBRES DE CARTAN ET ÉLÉMENTS RÉGULIERS D’UNE ALGÈBRE DE LIE 25",
		"EXERCICES", "")
	got, err := s16(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %+v, want nothing", got)
	}
}

// The fault: pages/ac-v-vii-fr/0076.md, which held the opening of Commutative
// Algebra V § 2 exercise 13 where its running head belongs, so that the
// exercise was in no content file at all.
func TestS16ReportsAParagraphKeptAsAHead(t *testing.T) {
	c := s16Corpus("EXERCICES", "¶ 13) a) Soient K un corps de caractéristique 0, "+
		"n l’idéal de l’anneau de polynômes K[X, Y, Z] engendré par Y² — X² — X³. "+
		"Montrer que n est premier, et que l’anneau intègre A = K[X, Y, Z]/n n’est "+
		"pas intégralement clos.")
	got, err := s16(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("reported %+v, want the one page that kept a paragraph", got)
	}
	if got[0].File != "pages/ens/0009.md" {
		t.Errorf("named %q", got[0].File)
	}
	if !strings.Contains(got[0].Msg, "219 characters long") {
		t.Errorf("said %q", got[0].Msg)
	}
}

// Length is counted in characters and not in bytes. A French head of 89 runes
// runs past 90 bytes on the accents alone, and counting bytes would report it.
func TestS16CountsCharactersAndNotBytes(t *testing.T) {
	head := strings.Repeat("é", 89)
	if len(head) <= headLimit {
		t.Fatalf("the fixture is %d bytes, which does not test what it says", len(head))
	}
	got, err := s16(s16Corpus(head))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %+v, want nothing", got)
	}
}

// A § of exercises, one Doc each, with the lengths given.
func s17Corpus(dir string, lengths ...int) *Corpus {
	c := &Corpus{}
	for i, n := range lengths {
		m := corpus.ExerciseFrontMatter{Book: "lie", Chapter: "III", Section: 10, Exercise: i + 1}
		c.Docs = append(c.Docs, Doc{
			Path:     fmt.Sprintf("%s/%02d.md", dir, i+1),
			Lang:     "en",
			Kind:     KindExercise,
			Exercise: &m,
			Body:     strings.Repeat("x", n),
		})
	}
	return c
}

// The fault: content/en/lie/III/exercises/s10/03.md, 64560 characters against
// two neighbours of ordinary length, holding the historical note of chapter III
// and its 42-entry bibliography. tamnd/bourbaki#411.
func TestS17ReportsAFileThatHoldsTheRunAfterIt(t *testing.T) {
	c := s17Corpus("content/en/lie/III/exercises/s10", 700, 900, 64560)
	got, err := s17(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 {
		t.Fatalf("reported %+v, want the one file that ran on", got)
	}
	if got[0].File != "content/en/lie/III/exercises/s10/03.md" {
		t.Errorf("named %q", got[0].File)
	}
	if !strings.Contains(got[0].Msg, "64560 characters and the median of the § is 900") {
		t.Errorf("said %q", got[0].Msg)
	}
}

// A long exercise is not a run-on. Commutative Algebra VII § 2 exercise 22 is
// 18626 characters and is one exercise, printed long; a § whose files are all
// of that order says nothing is wrong with any of them.
func TestS17LeavesASectionWhoseExercisesAreAllLong(t *testing.T) {
	got, err := s17(s17Corpus("content/en/ac/VII/exercises/s2", 9000, 11000, 18626, 8000, 12000))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %+v, want nothing", got)
	}
}

// The floor. A § whose median is 90 characters says nothing useful about a file
// of 900, and without a floor every short § in the corpus reports one.
func TestS17LeavesASmallMultipleOfASmallMedian(t *testing.T) {
	got, err := s17(s17Corpus("content/en/top/I/exercises/s1", 90, 95, 100, 900))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %+v, want nothing: 900 characters is an exercise", got)
	}
}

// Characters and not bytes, so that a French § is held to the same length as an
// English one rather than to a shorter one by its accents.
func TestS17CountsCharactersAndNotBytes(t *testing.T) {
	c := s17Corpus("content/fr/lie/III/exercises/s10", 700, 900, 4001)
	for i := range c.Docs {
		c.Docs[i].Lang = "fr"
		c.Docs[i].Body = strings.Repeat("é", len([]rune(c.Docs[i].Body)))
	}
	got, err := s17(c)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Errorf("reported %+v: 4001 runes is 8002 bytes and neither is six times 900", got)
	}
}
