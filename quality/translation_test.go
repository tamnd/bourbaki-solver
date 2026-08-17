package quality

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// As in math_test.go, none of the bodies here is Bourbaki. Each is the shape of
// a fault set in invented text, and the comment says which fault and where the
// number in the rule came from.

// pairDocs is an English file and a translation of it, linked the way the
// corpus links them, through translated_from and not through the path.
func pairDocs(en, vi string) []Doc {
	enDoc := Doc{
		Path: "content/en/alg/VIII/01_s1.md", Lang: "en", Kind: KindSection, Body: en, head: 1,
		Section: &corpus.SectionFrontMatter{},
	}
	viDoc := Doc{
		Path: "content/vi/alg/VIII/01_s1.md", Lang: "vi", Kind: KindSection, Body: vi, head: 1,
		Section: &corpus.SectionFrontMatter{
			TranslatedFrom: enDoc.Path,
			SourceSHA256:   corpus.ContentSHA256(en),
		},
	}
	return []Doc{enDoc, viDoc}
}

// L07 is about script and not about vocabulary, so the fixture only has to be
// Vietnamese in the one way the rule looks at: the diacritics.
func TestL07FindsAParagraphLeftInEnglish(t *testing.T) {
	docs := pairDocs(
		"Let A be a ring.\n\nEvery isotypical module is semisimple.",
		"Cho A là một vành.\n\nEvery isotypical module is semisimple.")
	got := run(t, l07, docs...)
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(got), got)
	}
	if got[0].Line != 3 {
		t.Errorf("the finding is on line %d, want 3", got[0].Line)
	}
	if !strings.Contains(got[0].Msg, "vi") {
		t.Errorf("the finding does not say what language is missing: %s", got[0].Msg)
	}
}

// The floor is a claim about whether a paragraph is English, not about how long
// it is. What is left of a display once the mathematics is taken out is not
// English and a translation leaves it standing, so a rule that reported it would
// fail the build on correct work.
func TestL07AsksWhetherTheParagraphIsEnglish(t *testing.T) {
	cases := []struct {
		name string
		para string
		want bool // is it reported
	}{
		// The residue, in the real shapes the corpus has. They are written out
		// here rather than copied, because the volume is under copyright and
		// what is being pinned is the shape and not the sentence.
		{"a numbered display gone", "(1) Card(J) Card(I .", false},
		{"two operators and a full stop", "Hom Hom .", false},
		{"the remains of an arrow", "// M", false},
		{"a bare roman numeral", "(II)", false},
		{"two lower case operators, which the word count read as prose", "long dim .", false},

		// English prose, which is what the rule is for.
		{"a short sentence", "Let A be a ring.", true},
		{"a fragment that runs into a display", "for every integer .", true},
		{"a numbered assertion", "(ii) The set is stable under addition.", true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := run(t, l07, pairDocs(c.para, c.para)...)
			if c.want && len(got) != 1 {
				t.Fatalf("got %d findings, want 1: %v", len(got), got)
			}
			if !c.want && len(got) != 0 {
				t.Fatalf("a paragraph with nothing to translate was reported: %v", got)
			}
		})
	}
}

// The margin, and the reason four obvious words are off the list. Every one of
// these is a Vietnamese word spelled the way an English one is, and the rule is
// hard, so this is the case that has to keep working.
func TestL07IsNotFooledByVietnameseWordsSpelledLikeEnglishOnes(t *testing.T) {
	docs := pairDocs(
		"Let A be a ring.\n\nThe degree of L over K is then finite.",
		"Cho A là một vành, do đó ta in ra một số to.\n\nKhi đó bậc của L trên K là hữu hạn.")
	if got := run(t, l07, docs...); len(got) != 0 {
		t.Errorf("a translated section was reported: %v", got)
	}
}

// L06 reads the glossary off disk, so it needs a root.
func glossaryRoot(t *testing.T, yaml string) string {
	t.Helper()
	root := t.TempDir()
	if err := os.MkdirAll(filepath.Join(root, "manifests"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "manifests", "glossary.yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	return root
}

const testGlossary = `version: 1
terms:
    - en: ring
      vi: vành
    - en: semisimple ring
      vi: vành nửa đơn
    - en: field
      vi: trường
`

func TestL06FindsATermRenderedSomeOtherWay(t *testing.T) {
	root := glossaryRoot(t, testGlossary)
	docs := pairDocs(
		"Let A be a ring and K a field.",
		"Cho A là một vành và K là một thể.")
	out, err := l06(&Corpus{Root: root, Docs: docs})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(out), out)
	}
	if !strings.Contains(out[0].Msg, "field") || !strings.Contains(out[0].Msg, "trường") {
		t.Errorf("the finding does not name the term and its rendering: %s", out[0].Msg)
	}
	if strings.Contains(out[0].Msg, "ring") {
		t.Errorf("a term the file does render was reported: %s", out[0].Msg)
	}
}

// The masking, which is the whole of Mentioned. "semisimple ring" contains
// "ring", and holding the translation to a rendering of "ring" inside a phrase
// that has no room for one would report every correct page.
func TestL06HoldsThePhraseAndNotTheWordInsideIt(t *testing.T) {
	root := glossaryRoot(t, testGlossary)
	docs := pairDocs(
		"Every semisimple ring is a ring.",
		"Mọi vành nửa đơn đều là một vành.")
	out, err := l06(&Corpus{Root: root, Docs: docs})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("a file that renders both terms was reported: %v", out)
	}
}

// A term that only occurs inside the mathematics is a term the translator was
// told to copy, so it is not a term the file has to render.
func TestL06IgnoresATermInsideAFormula(t *testing.T) {
	root := glossaryRoot(t, `version: 1
terms:
    - en: spec
      vi: phổ
`)
	docs := pairDocs(
		"The set $\\operatorname{spec}(A)$ is closed.",
		"Tập $\\operatorname{spec}(A)$ là đóng.")
	out, err := l06(&Corpus{Root: root, Docs: docs})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("a term inside a formula was held against the translation: %v", out)
	}
}

// The corpus shipped this one: "Theo Corollary da dan" in the appendix on the
// trace, where every other mention of the word in the same file is "hệ quả". L07
// passes it because the sentence is Vietnamese and L06 passes it because the
// rendering is there, nine times.
func TestL10FindsATermLeftInEnglish(t *testing.T) {
	root := glossaryRoot(t, `version: 1
terms:
    - en: corollary
      vi: hệ quả
    - en: semisimple ring
      vi: vành nửa đơn
`)
	docs := pairDocs(
		"By the corollary quoted above, A is a semisimple ring.",
		"Theo corollary đã dẫn, A là một vành nửa đơn.")
	out, err := l10(&Corpus{Root: root, Docs: docs})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(out), out)
	}
	if !strings.Contains(out[0].Msg, "corollary") || !strings.Contains(out[0].Msg, "hệ quả") {
		t.Errorf("the finding does not name the term and its rendering: %s", out[0].Msg)
	}
	if out[0].Line != 1 {
		t.Errorf("the finding is on line %d, want the line the word is on", out[0].Line)
	}
}

// The English word inside a formula is the mathematics, which the translator
// was told to copy, so it is not a word left standing.
func TestL10IgnoresATermInsideAFormula(t *testing.T) {
	root := glossaryRoot(t, `version: 1
terms:
    - en: ring
      vi: vành
    - en: spec
      vi: phổ
`)
	docs := pairDocs(
		"The ring $\\operatorname{spec}(A)$ is closed.",
		"Vành $\\operatorname{spec}(A)$ là đóng.")
	out, err := l10(&Corpus{Root: root, Docs: docs})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("a term inside a formula was read as English left standing: %v", out)
	}
}

// A row whose rendering is the English word has nothing to leak. Vietnamese
// keeps some of Bourbaki's vocabulary as it stands, and reporting those would
// fail the build on the rendering the glossary itself asks for.
func TestL10SkipsARowThatRendersToItself(t *testing.T) {
	root := glossaryRoot(t, `version: 1
terms:
    - en: hausdorff
      vi: Hausdorff
`)
	docs := pairDocs("The space is hausdorff.", "Không gian là Hausdorff.")
	out, err := l10(&Corpus{Root: root, Docs: docs})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("a term whose rendering is the English word was reported: %v", out)
	}
}

// A term the English never mentions is not a term this section was shown, so a
// word of the translation that happens to match one is not a leak. Mentioned is
// what decides, the same way L06 decides.
func TestL10OnlyAsksAboutTermsTheEnglishMentions(t *testing.T) {
	root := glossaryRoot(t, testGlossary)
	docs := pairDocs("Let A be a ring.", "Cho A là một vành field.")
	out, err := l10(&Corpus{Root: root, Docs: docs})
	if err != nil {
		t.Fatal(err)
	}
	if len(out) != 0 {
		t.Fatalf("a term the English does not mention was reported: %v", out)
	}
}

// H07 lives in the hygiene group and is tested here because the file it was
// written for is a translation. The corpus shipped one: a :::writing fence
// around a retranslated section that every other rule passed.
func TestH07FindsAProviderFenceInACommittedFile(t *testing.T) {
	docs := pairDocs(
		"Let A be a ring.",
		":::writing{variant=\"document\" id=\"58321\"}\nCho A là một vành.\n:::")
	got := run(t, h07, docs...)
	if len(got) != 2 {
		t.Fatalf("got %d findings, want 2, one per fence line: %v", len(got), got)
	}
	if got[0].Line != 1 {
		t.Errorf("the first finding is on line %d, want 1", got[0].Line)
	}
	if len(run(t, h07, pairDocs("Let A be a ring.", "Cho A là một vành.")...)) != 0 {
		t.Error("a clean file was reported")
	}
}

// The paragraph is the eleventh chunk of the appendix on the trace as it came
// back, with the Vietnamese that surrounds the English on the same line. L07
// reads the paragraph and passes it, because the paragraph is written in
// Vietnamese; L11 reads the run of words inside it.
func TestL11FindsASentenceLeftInEnglishInsideAVietnameseParagraph(t *testing.T) {
	const vi = "Hãy chứng minh b). " +
		"It therefore suffices to prove assertion b) when the A-module E is free. " +
		"There then exists a finitely generated free submodule F of E that contains " +
		"the image of it. Đặt $w=u+v$. Ảnh của $w$ được chứa trong F."
	const en = "Let us prove b). " +
		"It therefore suffices to prove assertion b) when the A-module E is free. " +
		"There then exists a finitely generated free submodule F of E that contains " +
		"the image of it. Put $w=u+v$. The image of $w$ is contained in F."
	docs := pairDocs(en, vi)
	if got := run(t, l07, docs...); len(got) != 0 {
		t.Fatalf("L07 already reports this, so L11 has nothing to add: %v", got)
	}
	got := run(t, l11, docs...)
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(got), got)
	}
	if !strings.Contains(got[0].Msg, "nothing of vi") {
		t.Errorf("the finding does not say what language is missing: %s", got[0].Msg)
	}
}

// A paragraph that came back wholly in English is L07's, and reporting it twice
// would put two findings on one line for one failure.
func TestL11LeavesAWhollyEnglishParagraphToL07(t *testing.T) {
	docs := pairDocs(
		"Let A be a ring.\n\nEvery isotypical module is semisimple.",
		"Cho A là một vành.\n\nEvery isotypical module is semisimple.")
	if got := run(t, l11, docs...); len(got) != 0 {
		t.Errorf("L11 reported a paragraph L07 already has: %v", got)
	}
	if got := run(t, l07, docs...); len(got) != 1 {
		t.Errorf("L07 did not report it: %v", got)
	}
}

// The mathematics is copied and not translated, so a display carries no
// Vietnamese and a rule that counted words alone would fail the build on
// correct work. These are the shapes the corpus actually holds.
func TestL11LeavesTheMathematicsAlone(t *testing.T) {
	for _, para := range []string{
		"det((1$_F+u_F)\\circ (1_F+v_F)) =$ det(1$_F+u_F)$ det(1$_F+v_F)$.",
		"$$u=\\theta \\sum_{j\\in J}f_j \\tag{2}$$",
		"Ta chứng minh a). Với mọi số nguyên $p\\geqslant 0$, A-môđun xạ ảnh $\\wedge^pF$ " +
			"có thể được đồng nhất với một môđun con của $\\wedge^pE$ (III, §7, No. 9, p. 520, Hệ quả).",
	} {
		if got := run(t, l11, pairDocs(para, para)...); len(got) != 0 {
			t.Errorf("%q was reported: %v", para, got)
		}
	}
}

// The two names are real. The same section on the same host a half hour apart
// came back as gpt-5-6 and then gpt-5-6-mini.
func TestL08NamesTheFileASmallModelWrote(t *testing.T) {
	cases := map[string]bool{
		"gpt-5-6":      false,
		"gpt-5-6-mini": true,
		"gpt-5-6-nano": true,
		"":             false,
		// A model whose name merely contains the letters. The word has to be
		// the suffix, or a model called minimax would be refused for its name.
		"minimax-01": false,
		// A section of fifteen chunks can be answered by two models, because
		// the account can be moved down in the middle of one. The file names
		// every model that answered it and half a section on the small one is
		// as much worth doing again as all of it.
		"gpt-5-6, gpt-5-6-mini": true,
		"gpt-5-6, gpt-5-7":      false,
	}
	for model, want := range cases {
		t.Run(model, func(t *testing.T) {
			docs := pairDocs("Let A be a ring.", "Cho A là một vành.")
			docs[1].Section.TranslationModel = model
			out, err := l08(&Corpus{Docs: docs})
			if err != nil {
				t.Fatal(err)
			}
			if got := len(out) == 1; got != want {
				t.Errorf("l08 on %q reported %v, want %v: %v", model, got, want, out)
			}
		})
	}
}

// content/fr is read off the French volume, not translated from the English, so
// it names no translated_from and must not be reported as a translation with a
// missing source. The library is printed in two languages and only one of them
// is the language of record.
func TestFrenchExtractionIsNotATranslation(t *testing.T) {
	fr := Doc{
		Path: "content/fr/alg/VIII/01_s1.md", Lang: "fr", Kind: KindSection,
		Body: "Soit A un anneau.", head: 1,
		Section: &corpus.SectionFrontMatter{},
	}
	c := &Corpus{
		Docs:  []Doc{fr},
		Books: &corpus.BooksManifest{Books: []corpus.Book{{ID: "alg-viii-fr", Lang: "fr"}}},
	}
	if _, bad := c.pairs(); len(bad) != 0 {
		t.Errorf("the French extraction was taken for a translation: %v", bad)
	}
}

// With no French volume registered, a French file really is a translation
// somebody made, and a translation with no source is still a finding.
func TestATranslationWithNoSourceIsStillAFinding(t *testing.T) {
	vi := Doc{
		Path: "content/vi/alg/VIII/01_s1.md", Lang: "vi", Kind: KindSection,
		Body: "Cho A là một vành.", head: 1,
		Section: &corpus.SectionFrontMatter{},
	}
	c := &Corpus{Docs: []Doc{vi}, Books: &corpus.BooksManifest{}}
	if _, bad := c.pairs(); len(bad) != 1 {
		t.Errorf("want one finding for a translation with no source, got %v", bad)
	}
}

// L12 is the rule Theory of Sets needed. The formula is chapter I's C6 and the
// words in it are prose, so a translation that copies the mathematics through
// as it stands has left part of the section in English.
func TestL12FindsTheWordsOfAFormulaLeftInEnglish(t *testing.T) {
	const en = "Đây là quy tắc C6.\n\n" +
		`$((\text{not } A) \text{ or } B) \Rightarrow ((\text{not not } A) \text{ or } B)$`
	docs := pairDocs(
		"This is criterion C6.\n\n"+
			`$((\text{not } A) \text{ or } B) \Rightarrow ((\text{not not } A) \text{ or } B)$`,
		en)
	got := run(t, l12, docs...)
	if len(got) != 4 {
		t.Fatalf("got %d findings, want one per run: %v", len(got), got)
	}
	if !strings.Contains(got[0].Msg, `\text{not }`) {
		t.Errorf("the finding does not name the run: %s", got[0].Msg)
	}
}

// The same formula with its words translated is a translated formula, and L01
// has to let it through or the section can never be written at all.
func TestL01AndL12AcceptAFormulaWhoseWordsWereTranslated(t *testing.T) {
	docs := pairDocs(
		"This is criterion C6.\n\n"+
			`$(\text{not } A) \text{ or } B$`,
		"Đây là quy tắc C6.\n\n"+
			`$(\text{không } A) \text{ hoặc } B$`)
	if got := run(t, l01, docs...); len(got) != 0 {
		t.Fatalf("L01 refuses a translated formula: %v", got)
	}
	if got := run(t, l12, docs...); len(got) != 0 {
		t.Fatalf("L12 reports a translated formula: %v", got)
	}
}

// Everything outside the words is still compared character for character, which
// is what L01 is for. A renamed variable inside a formula that also carries
// words has to be refused the same as one in a formula that does not.
func TestL01StillRefusesASymbolThatMovedInsideAFormulaWithWordsInIt(t *testing.T) {
	docs := pairDocs(
		"This is criterion C6.\n\n"+
			`$(\text{not } A) \text{ or } B$`,
		"Đây là quy tắc C6.\n\n"+
			`$(\text{không } A) \text{ hoặc } C$`)
	got := run(t, l01, docs...)
	if len(got) != 1 {
		t.Fatalf("got %d findings, want 1: %v", len(got), got)
	}
}

// A run over one translation still has to find the English beside it.
//
// bourbaki audit -lang vi loads the Vietnamese, because that is what the run is
// about, and for a while it loaded nothing else. Every translation rule reads a
// file and its source, so all forty nine translated files of Theory of Sets came
// back as translations whose source does not exist, in nine rules, four hundred
// and forty one findings against a corpus that had nothing wrong with it. The
// English is read into Sources: in hand for the comparison, out of scope for the
// rules that walk Docs.
func TestARunOverOneLanguageStillFindsTheEnglishItWasTranslatedFrom(t *testing.T) {
	root := t.TempDir()
	writeSection(t, root, "content/en/ens/III/00_frontmatter.md", corpus.SectionFrontMatter{
		Book: "ens", Lang: "en", Section: 1,
	}, "Let $E$ be a set.")
	writeSection(t, root, "content/vi/ens/III/00_frontmatter.md", corpus.SectionFrontMatter{
		Book: "ens", Lang: "vi", Section: 1,
		TranslatedFrom: "content/en/ens/III/00_frontmatter.md",
	}, "Cho $E$ là một tập hợp.")

	c := &Corpus{Root: root, Books: &corpus.BooksManifest{}, Langs: []string{"vi"}}
	docs, err := readDocs(root, c.Langs)
	if err != nil {
		t.Fatal(err)
	}
	c.Docs = docs
	if err := c.readSources(); err != nil {
		t.Fatal(err)
	}

	// Out of scope: the rules walk Docs and the English is not in it, so an
	// English file with something wrong with it is not this run's business.
	for _, d := range c.Docs {
		if d.Lang != "vi" {
			t.Errorf("a run over vi loaded %s for auditing", d.Path)
		}
	}
	ps, bad := c.pairs()
	if len(bad) != 0 {
		t.Fatalf("the pairing reported %v", bad)
	}
	if len(ps) != 1 {
		t.Fatalf("got %d pairs, want 1", len(ps))
	}
	if ps[0].en.Path != "content/en/ens/III/00_frontmatter.md" {
		t.Errorf("paired with %s", ps[0].en.Path)
	}
}

// Asking for every language reads each file once, and the pairing takes it from
// the list the rules walk rather than from a second copy of the same bytes.
func TestARunOverEveryLanguageReadsTheEnglishOnce(t *testing.T) {
	root := t.TempDir()
	writeSection(t, root, "content/en/ens/III/00_frontmatter.md", corpus.SectionFrontMatter{
		Book: "ens", Lang: "en", Section: 1,
	}, "Let $E$ be a set.")

	c := &Corpus{Root: root, Books: &corpus.BooksManifest{}, Langs: []string{"en", "vi"}}
	docs, err := readDocs(root, c.Langs)
	if err != nil {
		t.Fatal(err)
	}
	c.Docs = docs
	if err := c.readSources(); err != nil {
		t.Fatal(err)
	}
	if len(c.Sources) != 0 {
		t.Errorf("read %d files a second time: %v", len(c.Sources), c.Sources)
	}
}

func writeSection(t *testing.T, root, rel string, meta corpus.SectionFrontMatter, body string) {
	t.Helper()
	path := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (corpus.SectionFile{Meta: meta, Body: body + "\n"}).Write(path); err != nil {
		t.Fatal(err)
	}
}
