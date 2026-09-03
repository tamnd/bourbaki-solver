package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/glossary"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/prompt"
	"github.com/tamnd/bourbaki-solver/queue"
	"github.com/tamnd/bourbaki-solver/textguard"
	"github.com/tamnd/bourbaki-solver/translate"
)

// Staleness is what decides whether a section is asked for again, so getting it
// wrong is expensive in both directions: too eager and a glossary edit buys a
// run of the whole corpus, too lazy and files go on claiming to be current
// against terminology they were never shown.
//
// The rule that shipped first was too eager. It compared glossary_version, which
// moves on any edit anywhere, so pinning "common zero", a phrase that occurs in
// one appendix of chapter VIII, marked all 27 sections stale. Measured on the
// real corpus, the version 2 to version 5 move changes what 14 of the 27 are
// shown, "common zero" reaches 1 of them and "algebraic over" reaches 3.
func TestAGlossaryRowOnlyStalesTheSectionsItReaches(t *testing.T) {
	root := t.TempDir()
	english := "Let A be a ring, and let M be a simple module over it."
	other := "Every finite division ring is a field."
	writeEnglish(t, root, 1, english)
	writeEnglish(t, root, 2, other)

	g := &glossary.Glossary{Version: 4, Terms: []glossary.Term{
		{EN: "ring", VI: "vành"},
		{EN: "simple module", VI: "môđun đơn"},
	}}
	writeVietnamese(t, root, g, 1, english)
	writeVietnamese(t, root, g, 2, other)

	// Nothing has moved, so nothing is stale.
	if jobs := stale(t, root, g); len(jobs) != 0 {
		t.Fatalf("a corpus nobody touched reported %d stale: %v", len(jobs), jobs)
	}

	// A row that reaches one section stales that one and not the other, even
	// though the version moves for both.
	g.Version = 5
	g.Terms[1].VI = "môđun giản đơn"
	jobs := stale(t, root, g)
	if len(jobs) != 1 || jobs[0].source != rel(root, englishPath(root, 1)) {
		t.Fatalf("got %v, want only the section that mentions the term", jobs)
	}
	if jobs[0].why != "the terminology it was shown has changed" {
		t.Errorf("the reason given is %q", jobs[0].why)
	}

	// A row in neither section stales neither, and it does move the version.
	g.Version = 6
	g.Terms = append(g.Terms, glossary.Term{EN: "quaternion algebra", VI: "đại số quaternion"})
	writeVietnamese(t, root, g, 1, english)
	if jobs := stale(t, root, g); len(jobs) != 0 {
		t.Fatalf("a row neither section mentions stale %d of them: %v", len(jobs), jobs)
	}
}

// A file written before the digest existed has nothing to compare, so it falls
// back to the version and is stale on any bump. That is where every file already
// was, and it is the only honest answer: the rows it was shown were not recorded
// and cannot be recovered from the file.
func TestAFileWithNoDigestFallsBackToTheVersion(t *testing.T) {
	root := t.TempDir()
	english := "Let A be a ring."
	writeEnglish(t, root, 1, english)
	g := &glossary.Glossary{Version: 4, Terms: []glossary.Term{{EN: "ring", VI: "vành"}}}
	writeVietnamese(t, root, g, 1, english)

	// Take the digest out, the way a file written a month ago has none.
	path := vietnamesePath(root, 1)
	f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
	if err != nil {
		t.Fatal(err)
	}
	f.Meta.GlossaryTerms = ""
	if err := f.Write(path); err != nil {
		t.Fatal(err)
	}

	if jobs := stale(t, root, g); len(jobs) != 0 {
		t.Fatalf("the same version was called stale: %v", jobs)
	}
	g.Version = 5
	jobs := stale(t, root, g)
	if len(jobs) != 1 {
		t.Fatalf("got %d stale, want 1", len(jobs))
	}
	if jobs[0].why != "it records no terminology and was made with glossary 4, which is now 5" {
		t.Errorf("the reason given is %q", jobs[0].why)
	}
}

// The other three reasons a file is stale, and each says which it is, because
// "stale" on its own does not tell you whether one section is going again or all
// of them are.
func TestEachKindOfStalenessSaysWhichItIs(t *testing.T) {
	g := &glossary.Glossary{Version: 4, Terms: []glossary.Term{{EN: "ring", VI: "vành"}}}
	english := "Let A be a ring."

	cases := []struct {
		name string
		bend func(root string, meta *corpus.SectionFrontMatter)
		want string
	}{
		{"no translation", nil, "there is no translation"},
		{"the English moved", func(_ string, m *corpus.SectionFrontMatter) {
			m.SourceSHA256 = "0000"
		}, "the English has changed since"},
		{"the instructions moved", func(_ string, m *corpus.SectionFrontMatter) {
			m.PromptSHA256 = "0000"
		}, "the instructions have changed since"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			root := t.TempDir()
			writeEnglish(t, root, 1, english)
			if c.bend != nil {
				writeVietnamese(t, root, g, 1, english)
				path := vietnamesePath(root, 1)
				f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
				if err != nil {
					t.Fatal(err)
				}
				c.bend(root, &f.Meta)
				if err := f.Write(path); err != nil {
					t.Fatal(err)
				}
			}
			jobs := stale(t, root, g)
			if len(jobs) != 1 {
				t.Fatalf("got %d stale, want 1", len(jobs))
			}
			if jobs[0].why != c.want {
				t.Errorf("the reason given is %q, want %q", jobs[0].why, c.want)
			}
		})
	}
}

// The digest is of the rows the section is shown and not of the glossary, so two
// sections that mention the same terms carry the same digest and a term nobody
// mentions does not appear in either.
func TestTheDigestIsTheRowsAndNotTheGlossary(t *testing.T) {
	g := &glossary.Glossary{Version: 1, Terms: []glossary.Term{
		{EN: "ring", VI: "vành"},
		{EN: "quaternion algebra", VI: "đại số quaternion"},
	}}
	a := translate.GlossaryDigest(g, "en", "vi", "Let A be a ring.")
	b := translate.GlossaryDigest(g, "en", "vi", "Every ring has a unit.")
	if a != b {
		t.Error("two sections shown the same rows carry different digests")
	}
	if c := translate.GlossaryDigest(g, "en", "vi", "Let A be a quaternion algebra."); c == a {
		t.Error("two sections shown different rows carry the same digest")
	}
	// A language with no rendering is shown nothing, which is where Chinese and
	// Japanese are until the glossary is filled in for them.
	if translate.GlossaryDigest(g, "en", "zh", "Let A be a ring.") != translate.GlossaryDigest(g, "en", "zh", "") {
		t.Error("a language with no renderings was shown something")
	}
}

// stale is what a run would do, without asking anything.
func stale(t *testing.T, root string, g *glossary.Glossary) []job {
	t.Helper()
	jobs, _, err := translateJobs(root, g, "en", "vi", "vi", "", "", "", "prompt-hash", false, false)
	if err != nil {
		t.Fatal(err)
	}
	return jobs
}

func meta(n int) corpus.SectionFrontMatter {
	return corpus.SectionFrontMatter{
		Book: "alg", BookTitle: "Algebra", Chapter: "VIII", ChapterTitle: "Semisimple",
		Section: n, SectionTitle: "A Section", Lang: "en", Source: "alg-viii",
	}
}

func englishPath(root string, n int) string {
	return corpus.SectionPath(root, "en", meta(n))
}

func vietnamesePath(root string, n int) string {
	m := meta(n)
	m.Lang = "vi"
	return corpus.SectionPath(root, "vi", m)
}

func writeEnglish(t *testing.T, root string, n int, body string) {
	t.Helper()
	m := meta(n)
	m.ContentSHA256 = corpus.ContentSHA256(body)
	writeSection(t, corpus.SectionPath(root, "en", m), corpus.SectionFile{Meta: m, Body: body})
}

// writeVietnamese writes the file a run against this glossary would have left,
// which is the only way a test of staleness can be about staleness rather than
// about how the front matter is filled in.
func writeVietnamese(t *testing.T, root string, g *glossary.Glossary, n int, english string) {
	t.Helper()
	m := meta(n)
	m.Lang = "vi"
	m.TranslatedFrom = rel(root, englishPath(root, n))
	m.SourceSHA256 = corpus.ContentSHA256(english)
	m.ContentSHA256 = corpus.ContentSHA256("Cho A là một vành.")
	m.GlossaryVersion = g.Version
	m.GlossaryTerms = translate.GlossaryDigest(g, "en", "vi", english)
	m.PromptSHA256 = "prompt-hash"
	writeSection(t, corpus.SectionPath(root, "vi", m), corpus.SectionFile{Meta: m, Body: "Cho A là một vành."})
}

func writeSection(t *testing.T, path string, f corpus.SectionFile) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := f.Write(path); err != nil {
		t.Fatal(err)
	}
}

// A section is fifteen asks over fifteen minutes and nobody chooses the model,
// so the account can be moved down halfway through one. A file that names only
// the model of the first chunk is a file that says gpt-5-6 about a section half
// of which came back on the small one, and L08 reads that field.
func TestTheFileNamesEveryModelThatAnsweredIt(t *testing.T) {
	cases := []struct {
		name   string
		models []string
		want   string
	}{
		{"one model throughout", []string{"gpt-5-6", "gpt-5-6", "gpt-5-6"}, "gpt-5-6"},
		{"moved down halfway", []string{"gpt-5-6", "gpt-5-6-mini"}, "gpt-5-6, gpt-5-6-mini"},
		{"moved down and back", []string{"gpt-5-6", "gpt-5-6-mini", "gpt-5-6"}, "gpt-5-6, gpt-5-6-mini"},
		{"a chunk that did not say", []string{"", "gpt-5-6"}, "gpt-5-6"},
		{"nothing said at all", []string{"", ""}, ""},
	}
	for _, c := range cases {
		if got := modelsUsed(c.models); got != c.want {
			t.Errorf("%s: got %q, want %q", c.name, got, c.want)
		}
	}
}

// The exercises are seven eighths of the files in Theory of Sets, and until this
// they were translated by nothing at all: translateJobs read every file under
// content/en as a section, an exercise head would not parse as one, and the walk
// went quietly on to the next. A volume with its §§ in Vietnamese and its
// exercises in English is not a volume anybody can work through, and the count
// at the end of a run said 26 files and meant 26 of 239.
func TestAnExerciseIsWorkForThisCommandToo(t *testing.T) {
	root := t.TempDir()
	g := &glossary.Glossary{Version: 1, Terms: []glossary.Term{{EN: "ring", VI: "vành"}}}
	english := "Show that every finite ring with no divisor of zero is a field."
	writeEnglishExercise(t, root, 3, english)

	jobs := stale(t, root, g)
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs, want the exercise: %v", len(jobs), jobs)
	}
	j := jobs[0]
	if j.ex == nil {
		t.Fatal("the exercise was read as a section")
	}
	if j.ex.Exercise != 3 || j.ex.Label != "alg-viii-s1-ex-3" {
		t.Errorf("the head was lost: %+v", j.ex)
	}
	if j.meta.Book != "alg" || j.meta.Chapter != "VIII" {
		t.Errorf("-book and -chapter cannot reach it: book %q chapter %q", j.meta.Book, j.meta.Chapter)
	}
	if j.why != "there is no translation" {
		t.Errorf("the reason given is %q", j.why)
	}
	// The glossary reaches an exercise the same way it reaches a §, or the two
	// halves of a volume are held to different terminology.
	if j.terms != translate.GlossaryDigest(g, "en", "vi", english) {
		t.Error("the exercise was not held to the rows its English mentions")
	}
}

// A translated exercise records the same five facts a translated section does,
// and is then current. Anything less and every run would ask for every exercise
// again, which on this book is 211 files a night.
func TestATranslatedExerciseIsCurrent(t *testing.T) {
	root := t.TempDir()
	g := &glossary.Glossary{Version: 1, Terms: []glossary.Term{{EN: "ring", VI: "vành"}}}
	english := "Show that every finite ring with no divisor of zero is a field."
	writeEnglishExercise(t, root, 3, english)
	j := stale(t, root, g)[0]

	path, err := writeTranslation(root, "en", "vi", "vi", "run-1", "prompt-hash", g.Version, j, "Chứng minh rằng mọi vành hữu hạn không có ước của không là một trường.", "gpt-5-6")
	if err != nil {
		t.Fatal(err)
	}
	if want := corpus.ExercisePath(root, "vi", *j.ex); path != want {
		t.Errorf("written to %s, want %s", path, want)
	}
	f, err := corpus.ReadFile[corpus.ExerciseFrontMatter](path)
	if err != nil {
		t.Fatal(err)
	}
	if f.Meta.Lang != "vi" || f.Meta.Tag != "00QM" || f.Meta.Exercise != 3 {
		t.Errorf("the head did not carry over: %+v", f.Meta)
	}
	if f.Meta.SourceSHA256 != corpus.ContentSHA256(english) {
		t.Error("the English it was made from was not recorded")
	}
	if jobs := stale(t, root, g); len(jobs) != 0 {
		t.Fatalf("a translation just written was called stale: %v", jobs)
	}

	// The English moving puts it back on the list, with the reason said out
	// loud, and so does a glossary row that reaches it.
	writeEnglishExercise(t, root, 3, english+" Deduce that it is commutative.")
	if jobs := stale(t, root, g); len(jobs) != 1 || jobs[0].why != "the English has changed since" {
		t.Errorf("the English changed and got %v", jobs)
	}
	writeEnglishExercise(t, root, 3, english)

	g.Version, g.Terms[0].VI = 2, "vòng"
	if jobs := stale(t, root, g); len(jobs) != 1 || jobs[0].why != "the terminology it was shown has changed" {
		t.Errorf("the terminology changed and got %v", jobs)
	}
}

// The instructions are the fourth test, and it is the one with teeth: a rewrite
// of prompt/translate.md has to reach the exercises as well as the §§, or half
// the book stays on rules that no longer hold.
func TestAChangeOfInstructionsReachesAnExercise(t *testing.T) {
	root := t.TempDir()
	g := &glossary.Glossary{Version: 1, Terms: []glossary.Term{{EN: "ring", VI: "vành"}}}
	english := "Show that every finite ring with no divisor of zero is a field."
	writeEnglishExercise(t, root, 3, english)
	j := stale(t, root, g)[0]
	if _, err := writeTranslation(root, "en", "vi", "vi", "run-1", "prompt-hash", g.Version, j, "Chứng minh.", "gpt-5-6"); err != nil {
		t.Fatal(err)
	}

	jobs, _, err := translateJobs(root, g, "en", "vi", "vi", "", "", "", "prompt-hash-2", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 || jobs[0].why != "the instructions have changed since" {
		t.Fatalf("got %v, want the exercise back on the list", jobs)
	}
}

func exerciseMeta(n int) corpus.ExerciseFrontMatter {
	return corpus.ExerciseFrontMatter{
		Book: "alg", Chapter: "VIII", Section: 1, Exercise: n,
		Label: fmt.Sprintf("alg-viii-s1-ex-%d", n), Tag: "00QM", Lang: "en",
	}
}

func writeEnglishExercise(t *testing.T, root string, n int, body string) {
	t.Helper()
	m := exerciseMeta(n)
	path := corpus.ExercisePath(root, "en", m)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := (corpus.ExerciseFile{Meta: m, Body: body}).Write(path); err != nil {
		t.Fatal(err)
	}
}

// staleUnder is stale with -redo-small on.
func staleUnder(t *testing.T, root string, g *glossary.Glossary, redoSmall bool) []job {
	t.Helper()
	jobs, _, err := translateJobs(root, g, "en", "vi", "vi", "", "", "", "prompt-hash", false, redoSmall)
	if err != nil {
		t.Fatal(err)
	}
	return jobs
}

// A file a cut down model wrote is not stale, and -redo-small reaches it anyway.
//
// Nothing about the question changed, so the four hashes are right to call the
// file current, and that is the whole difficulty: the run has no reason to look
// at it again and the only record that anything is wrong is L08 in the audit. A
// person who has read L08 says -redo-small and the file comes back on the list
// with the model as the reason.
func TestRedoSmallReachesAFileACutDownModelWrote(t *testing.T) {
	root := t.TempDir()
	g := &glossary.Glossary{Version: 1, Terms: []glossary.Term{{EN: "ring", VI: "vành"}}}
	const english = "Let A be a ring."
	writeEnglish(t, root, 1, english)
	writeVietnameseBy(t, root, g, 1, english, "laguna-s-2.1-free, hy3-free, gpt-5-6, gpt-5-6-mini")
	writeEnglish(t, root, 2, english)
	writeVietnameseBy(t, root, g, 2, english, "gpt-5-6")

	if got := staleUnder(t, root, g, false); len(got) != 0 {
		t.Fatalf("%d files are stale, and the four hashes hold for both", len(got))
	}
	got := staleUnder(t, root, g, true)
	if len(got) != 1 {
		t.Fatalf("-redo-small put %d files on the list, want the one with a cut down model in it", len(got))
	}
	if got[0].meta.Section != 1 {
		t.Errorf("it put § %d on the list, and § 1 is the one gpt-5-6-mini touched", got[0].meta.Section)
	}
	if !strings.Contains(got[0].why, "cut down model") {
		t.Errorf("the reason reads %q, and it should say what is wrong with the file", got[0].why)
	}
}

// writeVietnameseBy is writeVietnamese with the models named, which is the one
// fact -redo-small reads.
func writeVietnameseBy(t *testing.T, root string, g *glossary.Glossary, n int, english, model string) {
	t.Helper()
	writeVietnamese(t, root, g, n, english)
	path := vietnamesePath(root, n)
	f, err := corpus.ReadFile[corpus.SectionFrontMatter](path)
	if err != nil {
		t.Fatal(err)
	}
	f.Meta.TranslationModel = model
	writeSection(t, path, corpus.SectionFile(f))
}

// The cheap route asks first and the full one answers what it gets wrong, which
// is what the route table has said all along and what this makes true.
func TestOnlyTheCutDownRoutesAreHeldToAFirstAsk(t *testing.T) {
	hosts := []ocr.Host{
		{Name: "codex-mini", Model: "gpt-5.4-mini"},
		{Name: "codex", Model: "gpt-5.4"},
	}
	want := freshOnly(hosts)
	if _, ok := want["codex"]; ok {
		t.Error("the full model is held to a first ask, so a failed chunk has nowhere to go")
	}
	take, ok := want["codex-mini"]
	if !ok {
		t.Fatal("the cut down model may take anything, which is the loop this is here to stop")
	}
	if !take(queue.Job{Attempts: 0}) {
		t.Error("the cut down model will not take a chunk nobody has asked about")
	}
	if take(queue.Job{Attempts: 1}) {
		t.Error("the cut down model will take back a chunk it got wrong")
	}
}

// A run with nothing but cut down routes filters nothing. Holding work for a
// lane that is not in the run means translating none of it.
func TestACutDownRunOnItsOwnTakesEverything(t *testing.T) {
	hosts := []ocr.Host{{Name: "codex-mini", Model: "gpt-5.4-mini"}, {Name: "zen-deepseek", Model: "deepseek-chat-lite"}}
	if want := freshOnly(hosts); len(want) != 0 {
		t.Errorf("%d routes were held to a first ask with nothing to escalate to", len(want))
	}
}

// The complaint alone did not move the bibliography, so the note names the
// words the model kept translating.
//
// Chunk 3 of the historical note of chapter III came back four times with Vol.
// written Tap and and written va inside a numbered entry, on three different
// models, each time having been told that an entry stands as printed.
func TestTheRetryNoteSaysWhatABibliographyEntryIs(t *testing.T) {
	note := retryNote([]translate.Problem{
		{Rule: translate.RuleBibliography, Msg: `bibliography entry 1 is "...Tap I..." and the English has "...Vol. I..."`},
		{Rule: translate.RuleBibliography, Msg: "another entry of the same kind"},
	})
	if !strings.Contains(note, "copied out of the English character for") {
		t.Errorf("the note says what was wrong and not what to do:\n%s", note)
	}
	if n := strings.Count(note, "A numbered bibliography entry"); n != 1 {
		t.Errorf("the sentence is in the note %d times, want it once however often the rule fired", n)
	}
}

// Chunk 30 of the historical note of chapters I to IV wrote tr. 13 for p. 13,
// which is how a Vietnamese book writes a page number and is not where the
// citation points. It did it twice, with the general sentence about addresses
// in front of it, so the note names the abbreviation.
func TestTheRetryNoteSaysAPageNumberKeepsItsLetter(t *testing.T) {
	note := retryNote([]translate.Problem{
		{Rule: translate.RuleReference, Msg: "the English cites 2 things this answer does not: p.13 p.439"},
	})
	if !strings.Contains(note, "p. 13 stays p. 13") {
		t.Errorf("the note does not say what to do with the letter in front of the number:\n%s", note)
	}
}

// A pass reads what the pass before it was refused for, so the first ask of a
// chunk that has been asked before carries the complaint the last one ended on.
//
// This is chunk 4 of the historical note of chapter IV, which went round the
// same circle four times: the first ask of a pass, which carried no note, left
// the word Chapter standing in English, the second ask was told about it and
// broke a formula instead, and the pass after that began again at Chapter.
// asked is the question a run would have archived for this passage, which is
// what the run reads back to tell whether an answer on disk is about the text it
// is holding. A test that archives a made up question is a test of a case that
// does not happen: an answer is only ever written beside the question it
// answered. See archivedAnswers.
func asked(t *testing.T, body string) string {
	t.Helper()
	q, err := prompt.Translate("en", "vi", "", "", body)
	if err != nil {
		t.Fatal(err)
	}
	return q
}

func TestAPassReadsWhatTheOneBeforeItWasRefusedFor(t *testing.T) {
	root := t.TempDir()
	const en = "The square of $M\\cap N$ is a ring element."
	if err := archiveChunk(root, "vi", "content/en/ens/IV/historical_note.md",
		translate.Chunk{Index: 4, Of: 40, Body: en}, 2,
		asked(t, en), "The square của $M \\cup N$ là một phần tử vành.",
		"https://chatgpt.com/c/1"); err != nil {
		t.Fatal(err)
	}

	g := &glossary.Glossary{Version: 4, Terms: []glossary.Term{{EN: "square", VI: "bình phương"}}}
	j := job{source: "content/en/ens/IV/historical_note.md"}
	prior := refusedBefore(root, "vi", g, j, translate.Chunk{Index: 4, Of: 40, Body: en}, en)
	if len(prior) != 2 {
		t.Fatalf("%d complaints came back off disk, want the formula and the term: %v", len(prior), prior)
	}
	note := retryNote(prior)
	for _, want := range []string{"math span", "bình phương"} {
		if !strings.Contains(note, want) {
			t.Errorf("the first ask of the next pass does not say %q:\n%s", want, note)
		}
	}

	// What the repairs put right is not a complaint. An answer whose only fault
	// is that it wrote the page number the Vietnamese way is an answer the run
	// accepts, so the pass after it has nothing to say about it. See
	// translate.Readdress.
	const cited = "The ring is defined, p. 185."
	if err := archiveChunk(root, "vi", "content/en/ens/IV/historical_note.md",
		translate.Chunk{Index: 9, Of: 40, Body: cited}, 1,
		asked(t, cited), "Vành được định nghĩa, tr. 185.", ""); err != nil {
		t.Fatal(err)
	}
	if prior := refusedBefore(root, "vi", g, j, translate.Chunk{Index: 9, Of: 40, Body: cited}, cited); len(prior) != 0 {
		t.Errorf("an answer the run would have repaired carried %v", prior)
	}

	// A chunk nobody has asked for yet is asked without a note, which is every
	// chunk of every section the first time it goes over the fleet.
	if prior := refusedBefore(root, "vi", g, j, translate.Chunk{Index: 7, Of: 40, Body: en}, en); len(prior) != 0 {
		t.Errorf("a chunk with no answer on disk carried %v", prior)
	}
}

// Counting the formulas told the model there were three too few and not which
// three, and the three were the ones a translator stops seeing.
//
// Chunk 3 of exercise 17 of § 1 of chapter III came back six times, on two
// hosts and two models, with the same eleven formulas out of fourteen: every
// display and every $\mathfrak{P}(\mathrm{A})$ was there and the $x$ and the
// $y$ of "a relative complement of $x$ with respect to $y$" had become plain
// letters. The sentence names that case.
func TestTheRetryNoteSaysAOneLetterFormulaIsAFormula(t *testing.T) {
	note := retryNote([]translate.Problem{
		{Rule: translate.RuleMath, Msg: "has 11 math spans and the English has 14"},
		{Rule: translate.RuleMath, Msg: `math span 2 is "\\omega" and the English has "x"`},
	})
	if !strings.Contains(note, "the formula of a single letter") {
		t.Errorf("the note counts the formulas and does not say which ones go missing:\n%s", note)
	}
	if n := strings.Count(note, "Every formula the English sets"); n != 1 {
		t.Errorf("the sentence is in the note %d times, want it once however often the rule fired", n)
	}
}

// And a chunk that failed on something else does not carry advice about the
// bibliography. A note that says everything says nothing.
func TestTheRetryNoteOnlyAdvisesOnWhatFailed(t *testing.T) {
	note := retryNote([]translate.Problem{{Rule: translate.RuleMath, Msg: "math span 6 is not the English one"}})
	if strings.Contains(note, "bibliography") {
		t.Errorf("a chunk that lost a formula was told about citations:\n%s", note)
	}
}

// The note is built at ask time and is not part of what prompt_sha256 covers,
// which is the reason it is where it is. A sentence added to the prompt itself
// marks every translated file in the corpus stale and buys a run of the whole
// fleet; a sentence added here changes nothing that is already written.
func TestTheRetryNoteDoesNotMoveThePromptHash(t *testing.T) {
	before, err := prompt.TranslateSHA256("en", "vi")
	if err != nil {
		t.Fatal(err)
	}
	q, err := prompt.Translate("en", "vi", "", retryNote([]translate.Problem{
		{Rule: translate.RuleBibliography, Msg: "an entry was translated"}}), "Let A be a ring.")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(q, "A numbered bibliography entry") {
		t.Error("the advice did not reach the question")
	}
	after, err := prompt.TranslateSHA256("en", "vi")
	if err != nil {
		t.Fatal(err)
	}
	if before != after {
		t.Error("asking with a note moved the prompt hash, which marks every translated file stale")
	}
}

// Translation is the fourth thing in this repository that writes Markdown into
// the corpus, after the OCR, the mender and the solver, and it was the last one
// still writing whatever spelling a model happened to choose. The other three
// put their output into the corpus's typography first.
//
// content/vi/ens/III/exercises/s1/24.md is the page that showed it. The English
// ends a paragraph with the corpus star, written \*, the model handed back a
// bare asterisk, and that is a hard M11 finding sitting in the corpus over a
// repair that already existed and that nothing on this path called.
func TestTheTranslationIsPutIntoTheCorpusTypographyBeforeItIsWritten(t *testing.T) {
	// Every fault a model actually sends: a bare star for the corpus star, a
	// display set with brackets, blackboard bold written bare, a warning sign
	// from the wrong font, and a line with space hanging off the end.
	answer := "Cho $E$ la mot tap hop. \\(x\\) thuoc \\mathbb{Z}.   \n" +
		"\\[ f(x) = 0 \\]\n" +
		"Ket thuc doan nay. *\n"
	got := textguard.Normalise(answer)

	for _, bad := range []string{`\[`, `\]`, `\(`, `\)`} {
		if strings.Contains(got, bad) {
			t.Errorf("the delimiter %s survived: %q", bad, got)
		}
	}
	if !strings.Contains(got, "$$ f(x) = 0 $$") {
		t.Errorf("the display is not written as the corpus writes one: %q", got)
	}
	if !strings.Contains(got, `Ket thuc doan nay. \*`) {
		t.Errorf("the bare star was not put back as the corpus star: %q", got)
	}
	if strings.Contains(got, "   \n") {
		t.Errorf("the hanging space survived: %q", got)
	}
	if !strings.Contains(got, `\mathbf{Z}`) {
		t.Errorf("the blackboard bold was not written the corpus's way: %q", got)
	}
}

// The star repair works outside the math spans only, and a translation carries
// plenty of asterisks that are not the mark. K^* runs through these volumes in
// their thousands, and the emphasis a printing sets is written with asterisks
// against the letters. Neither may move.
func TestNormalisingATranslationLeavesTheAsterisksThatAreNotTheMark(t *testing.T) {
	for _, body := range []string{
		`Nhom $K^*$ la nhom cac don vi cua $K$.`,
		`Mot tap hop duoc goi la *phan nhanh* neu voi moi $x$.`,
		`Da duoc danh dau \* o cuoi.`,
		`Chuoi $A^{**}$ va $f * g$ trong $L^1$.`,
	} {
		if got := textguard.Normalise(body); got != body {
			t.Errorf("normalising changed a line it should not have:\n got %q\nwant %q", got, body)
		}
	}
}

// The second ask goes to the same host, and for every transport failure but one
// that is worth doing: measured on the usage log, the second ask was answered
// half the time after a Cloudflare interstitial and a third of the time after a
// timeout. After out of quota it was answered no times at all, 2,538 of them,
// which is 2,538 messages spent against a quota that was already gone.
func TestAHostThatSaysItIsOutOfQuotaIsNotAskedTwice(t *testing.T) {
	for _, s := range []string{
		// What the codex command on this machine says, which is 5,165 of the
		// 5,169 in the log, and what a gateway says instead.
		"question tr-vi-ea69fe-001-1 on codex: You've hit your usage limit. Upgrade to Pro (https://chatgpt.com/explore/pro), visit https://chatgpt.com/codex/settings/",
		"question tr-vi-373c9d-001-1 on zen-mimo: chat completions returned 429 Too Many Requests: Error from provider (Console): Rate limit exceeded. Please try again later.",
	} {
		if !outOfTurns(errors.New(s)) {
			t.Errorf("a host out of turns would be asked again: %q", s)
		}
	}
	// The three that are worth a second ask. Losing any of these to a wider
	// match costs the chunks that only ever came back on the retry.
	for _, s := range []string{
		"question tr-vi-014afd-002-1 on server3 stopped without writing an answer: browser: ERROR Cloudflare is holding this host on an interstitial. It is the IP, not the account",
		"question tr-vi-6c822f-010-1 on server2: no answer after 12m21s, giving up",
		"context canceled: node:events:487 throw er; // Unhandled 'error' event Error: write EPIPE at afterWriteDispatched",
	} {
		if outOfTurns(errors.New(s)) {
			t.Errorf("a transport failure worth retrying was read as an empty quota: %q", s)
		}
	}
}

// -raw is for filling the corpus and not for deciding the audit was wrong, so
// what it does is keep the text and write the complaints down. The measurement
// that put it there: the smallest section in Algebra II is two chunks and cost
// 114,524 tokens of the codex subscription, because one chunk was refused four
// times over terminology, and a full size section cost 349,108 and wrote
// nothing at all.
func TestTakeRawKeepsTheTextAndLogsWhatWasWrongWithIt(t *testing.T) {
	j := job{source: "content/en/alg/II/05_s5.md"}
	c := translate.Chunk{Index: 2, Of: 7}
	bad := []translate.Problem{
		{Rule: translate.RuleMath, Msg: "has 56 math spans and the English has 58"},
		{Rule: "terms", Msg: `leaves "left" in English`},
	}
	var said []string
	got := takeRaw(true, bad, j, c, func(format string, args ...any) {
		said = append(said, fmt.Sprintf(format, args...))
	})
	if len(got) != 0 {
		t.Fatalf("the chunk is still refused for %d things", len(got))
	}
	if len(said) != 2 {
		t.Fatalf("%d complaints were written down, want both of them: %q", len(said), said)
	}
	for _, line := range said {
		if !strings.Contains(line, "taken raw") || !strings.Contains(line, "chunk 2 of 7") {
			t.Errorf("the log line does not say which chunk was taken raw: %q", line)
		}
	}
}

// Transport is not an answer the audit disliked, it is no answer at all, so
// there is nothing for -raw to keep and the chunk is asked again.
func TestTakeRawLeavesAChunkThatNeverCameBack(t *testing.T) {
	j := job{source: "content/en/alg/II/05_s5.md"}
	c := translate.Chunk{Index: 1, Of: 7}
	bad := []translate.Problem{{Rule: "transport", Msg: "Cloudflare is holding this host"}}
	got := takeRaw(true, bad, j, c, func(string, ...any) {})
	if len(got) != 1 {
		t.Fatalf("a chunk that never came back was taken raw: %v", got)
	}
}

// A rate limit page is not an answer the audit disliked either. It arrives as a
// successful answer, so unless -raw leaves it alone it is written to the corpus
// under a full set of headers and a matching source hash, and every pass after
// it reads a finished translation. Eight sections were found in that state.
func TestTakeRawLeavesAMessageFromTheProvider(t *testing.T) {
	j := job{source: "content/en/ac/VII/exercises/s2/06.md"}
	c := translate.Chunk{Index: 1, Of: 1}
	bad := []translate.Problem{
		{Rule: translate.RuleRefusal, Msg: `gateway: "unusual activity has been detected from your device"`},
		{Rule: translate.RuleMath, Msg: "has 0 math spans and the English has 5"},
	}
	got := takeRaw(true, bad, j, c, func(string, ...any) {})
	if len(got) != 1 || got[0].Rule != translate.RuleRefusal {
		t.Fatalf("a provider message was taken raw: %v", got)
	}
}

// Off, it changes nothing, which is what every run that does not ask for it
// gets.
func TestTakeRawOffChangesNothing(t *testing.T) {
	bad := []translate.Problem{{Rule: translate.RuleMath, Msg: "spans"}}
	if got := takeRaw(false, bad, job{}, translate.Chunk{}, func(string, ...any) {}); len(got) != 1 {
		t.Fatalf("the complaint was dropped by a run that did not ask for -raw: %v", got)
	}
}

// The machine English of a French only volume is a source for Vietnamese.
//
// Springer translated 15 of the 43 volumes and the other 28 have no English at
// all, so a Vietnamese pass that walks content/en can never reach Topologie
// algebrique or Theories spectrales. The French of those volumes is read into
// content/en-mt first and this is the step that carries it on. The tree is held
// apart from content/en because a reader has to be able to tell Springer's
// English from a model's, and everything that asks what language the passage is
// written in has to be told English all the same.
func TestTheMachineEnglishIsASourceForVietnamese(t *testing.T) {
	root := t.TempDir()
	g := &glossary.Glossary{Version: 1, Terms: []glossary.Term{{EN: "sheaf", VI: "bó"}}}
	english := "A sheaf on a topological space is a space over it."
	m := meta(1)
	m.ContentSHA256 = corpus.ContentSHA256(english)
	writeSection(t, corpus.SectionPath(root, "en-mt", m), corpus.SectionFile{Meta: m, Body: english})

	jobs, _, err := translateJobs(root, g, "en-mt", "vi", "vi", "", "", "", "prompt-hash", false, false)
	if err != nil {
		t.Fatal(err)
	}
	if len(jobs) != 1 {
		t.Fatalf("got %d jobs out of content/en-mt, want the section: %v", len(jobs), jobs)
	}
	// The glossary is keyed by language and has no en-mt column, so a lookup
	// against the name of the tree returns nothing for every term and the volume
	// is translated with no terminology at all. That is the failure this guards.
	if jobs[0].terms != translate.GlossaryDigest(g, "en", "vi", english) {
		t.Error("the machine English was not held to the rows its text mentions")
	}
	if jobs[0].terms == translate.GlossaryDigest(g, "en-mt", "vi", english) {
		t.Error("the glossary was asked for an en-mt column, which no glossary has")
	}
}

// Where a translation says it came from. A file out of content/en is Springer
// and says nothing extra; one out of the French or out of the machine English
// says so, because it sits in the same tree as the first kind.
func TestATranslationRecordsWhichEnglishItCameFrom(t *testing.T) {
	for _, c := range []struct{ from, lang, method string }{
		{"en", "", ""},
		{"fr", "fr", "machine"},
		{"en-mt", "en-mt", "machine"},
	} {
		lang, method := provenance(c.from)
		if lang != c.lang || method != c.method {
			t.Errorf("from %q gives %q %q, want %q %q", c.from, lang, method, c.lang, c.method)
		}
	}
}

func TestTheMachineEnglishTreeIsEnglish(t *testing.T) {
	for _, c := range []struct{ from, want string }{
		{"en", "en"}, {"en-mt", "en"}, {"fr", "fr"},
	} {
		if got := sourceLang(c.from); got != c.want {
			t.Errorf("sourceLang(%q) is %q, want %q", c.from, got, c.want)
		}
	}
}

// The name of the front matter file is not written inside it, so readJob has to
// put it back from the path. Without it every volume-suffixed piece collapses
// onto the default name and only the last one translated survives: Algebre
// prints three notes to the reader and all three were writing to
// content/vi/alg/00_to_the_reader.md.
func TestReadJobKeepsTheNameOfTheFrontMatterFile(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "content", "en", "alg")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	names := []string{"00_to_the_reader_i_iii.md", "00_to_the_reader_iv_vii.md", "00_to_the_reader_viii.md"}
	seen := map[string]string{}
	for _, name := range names {
		meta := corpus.SectionFrontMatter{
			Book: "alg", BookTitle: "Algebra", Kind: corpus.KindReader,
			SectionTitle: "TO THE READER", Lang: "en", Source: "alg-viii",
		}
		f := corpus.SectionFile{Meta: meta, Body: "## TO THE READER\n\n" + name + "\n"}
		if err := f.Write(filepath.Join(dir, name)); err != nil {
			t.Fatal(err)
		}
		j, ok, err := readJob(root, filepath.Join(dir, name), "en")
		if err != nil || !ok {
			t.Fatalf("readJob(%s) = %v, %v", name, ok, err)
		}
		out := j.meta
		out.Lang = "vi"
		got := corpus.SectionPath(root, "vi", out)
		if filepath.Base(got) != name {
			t.Errorf("%s translates to %s, want the same name", name, filepath.Base(got))
		}
		if was, ok := seen[got]; ok {
			t.Errorf("%s and %s both translate to %s", was, name, got)
		}
		seen[got] = name
	}
}
