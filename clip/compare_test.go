package clip

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The readings called ours in this file are real. Each is what the extractor
// pinned in the index of the first clip run over Théories spectrales, taken out
// of work/clips/ts-iii-v-fr/clips.json, and each defect they carry is a defect
// the corpus shipped.

// TestTwoWaysOfWritingOneFormulaAreOneReading is what makes the audit worth
// reading. The extractor writes what the font drew and a model writes what an
// author would type, and if every one of those counted as a disagreement the
// report would be a list of every line, all of them saying the same thing
// twice.
func TestTwoWaysOfWritingOneFormulaAreOneReading(t *testing.T) {
	for _, pair := range []struct{ ours, theirs string }{
		{`$u(V)\cap W\subset u(V)$`, `$u(V) \cap W \subset u(V)$`},
		{`$x\rightarrow y$`, `$x \to y$`},
		{`$E_{\sigma}$`, `$E_\sigma$`},
		{`$\left(\frac{a}{b}\right)$`, `$(\frac{a}{b})$`},
		{`$1_{P^*}$`, `$1_{P^{\ast}}$`},
		{`Sp$(a)$`, `$\mathrm{Sp}(a)$`},
		{`$f\ldots g$`, `$f \cdots g$`},
		{`$a\leqslant b$`, `$a \leqslant b$`},
		{`**Proposition 4.**`, `Proposition 4.`},
	} {
		if at := Diff(pair.ours, pair.theirs); at >= 0 {
			t.Errorf("Diff(%q, %q) = %d, want the two read as one reading:\n  %q\n  %q",
				pair.ours, pair.theirs, at, Normalize(pair.ours), Normalize(pair.theirs))
		}
	}
}

// TestALooseAccentSurvivesNormalisation is the other half, and it is the half
// that matters. Everything above is folded away so that this is not: the ring
// of an interior and the caron of a Russian name are printed over their letter
// and the extractor sets them down beside it, and no amount of folding LaTeX
// spellings together may make those two readings agree.
func TestALooseAccentSurvivesNormalisation(t *testing.T) {
	for _, pair := range []struct{ name, ours, theirs string }{
		{
			"the caron of Šmulian",
			`il existe d’après le théorème de ˘Smulian`,
			`il existe d’après le théorème de Šmulian`,
		},
		{
			"the ring of an interior",
			`Comme $u$(˚V) est un voisinage de 0`,
			`Comme $u(\mathring{V})$ est un voisinage de 0`,
		},
		{
			"the dot of a class",
			`On désignera par $x$˙ l’image canonique`,
			`On désignera par $\dot{x}$ l’image canonique`,
		},
	} {
		if at := Diff(pair.ours, pair.theirs); at < 0 {
			t.Errorf("Diff() folded %s away, want it reported", pair.name)
		}
	}
}

// TestWhatBourbakiMeansBySomethingIsNotFoldedAway. The line between a macro
// that is folded and one that is not is whether the book uses it to say
// something. It sets its rings in bold, so a model that writes blackboard bold
// has read the page wrong and the report has to say so.
func TestWhatBourbakiMeansBySomethingIsNotFoldedAway(t *testing.T) {
	for _, pair := range []struct{ name, ours, theirs string }{
		{"bold against blackboard bold", `$\mathbf{R}$`, `$\mathbb{R}$`},
		{"a narrow accent against a wide one", `$\widehat{G/H}$`, `$\hat{G}/H$`},
		{"a Lie algebra set in fraktur", `$\mathfrak{g}$`, `$g$`},
		{"an omitted symbol", `Comme ˚V et E V sont`, `Comme $\mathring{V}$ et $E - V$ sont`},
	} {
		if at := Diff(pair.ours, pair.theirs); at < 0 {
			t.Errorf("Diff() folded %s away, want it reported", pair.name)
		}
	}
}

// TestAnAliasIsMatchedAsAWholeName is a bug this had before it ran. A table of
// string replacements folding \le into \leq turns \leq into \leqq, because the
// replacer matches the first three characters and carries on after what it
// wrote, and it turns \top into an arrow with a p stuck on the end. The macros
// are matched as whole control words for that reason and for no other.
func TestAnAliasIsMatchedAsAWholeName(t *testing.T) {
	for _, text := range []string{`$a\leq b$`, `$a\leqq b$`, `$\top$`, `$\ne$`, `$\nearrow$`} {
		got := Normalize(text)
		if strings.Contains(got, "leqq") && !strings.Contains(text, "leqq") {
			t.Errorf("Normalize(%q) = %q, want the name matched whole", text, got)
		}
		if strings.Contains(got, "rightarrowp") {
			t.Errorf("Normalize(%q) = %q, want \\top left alone", text, got)
		}
	}
	// \ne and \neq are one thing, and \nearrow is not either of them.
	if Diff(`$a\ne b$`, `$a\neq b$`) >= 0 {
		t.Error("Diff() read \\ne and \\neq as different")
	}
	if Diff(`$a\nearrow b$`, `$a\neq b$`) < 0 {
		t.Error("Diff() read an arrow as a relation because its name starts the same way")
	}
}

// TestAnEscapedBraceGoesWholeAndLeavesNoBackslash. The braces are taken out
// because most of them group and none of the grouping is a claim about the
// page, and a set written $\{x\}$ would otherwise come out as two stray
// backslashes standing where its braces were, which no other reading can match.
func TestAnEscapedBraceGoesWholeAndLeavesNoBackslash(t *testing.T) {
	if got := Normalize(`$\{x\}$`); got != "x" {
		t.Errorf("Normalize() = %q, want %q, with no backslash left where a brace was", got, "x")
	}
	// The other control words in a set keep theirs, which is the reason the
	// braces are taken out by name rather than by dropping every backslash.
	if got, want := Normalize(`$\{x\in E\mid x>0\}$`), `x\inE\midx>0`; got != want {
		t.Errorf("Normalize() = %q, want %q", got, want)
	}
	if at := Diff(`$\{x\}$`, `$\{ x \}$`); at >= 0 {
		t.Errorf("Diff() = %d on one set written two ways", at)
	}
}

// TestAFencedAnswerIsTheLineInsideIt. The prompt asks for the line and nothing
// else and most answers are the line and nothing else, but a line that is
// mostly LaTeX comes back fenced often enough that counting the fence as
// content would report a disagreement on every one of them.
func TestAFencedAnswerIsTheLineInsideIt(t *testing.T) {
	for _, fenced := range []string{
		"```latex\nde $E_{\\sigma}$, il existe\n```",
		"```\nde $E_{\\sigma}$, il existe\n```",
		"```markdown\nde $E_{\\sigma}$, il existe```",
	} {
		if got := Unfence(fenced); got != `de $E_{\sigma}$, il existe` {
			t.Errorf("Unfence(%q) = %q, want the line inside", fenced, got)
		}
	}
	// A line that has no fence keeps every character it has, backticks and all.
	const inline = "the ring is written `\\mathbf{Z}` here"
	if got := Unfence(inline); got != inline {
		t.Errorf("Unfence() = %q, want the line unchanged", got)
	}
}

// TestTheReportSaysWhereTwoReadingsPartCompany. A line of Bourbaki is a hundred
// characters and the disagreement is one of them, and a report that printed
// both readings and left the finding of it to the reader would be read once.
func TestTheReportSaysWhereTwoReadingsPartCompany(t *testing.T) {
	const ours = `il existe d’après le théorème de ˘Smulian (EVT, IV, p. 36, th. 2)`
	const theirs = `il existe d’après le théorème de Šmulian (EVT, IV, p. 36, th. 2)`
	at := Diff(ours, theirs)
	if at < 0 {
		t.Fatal("Diff() = -1, want the caron reported")
	}
	if window := Window(ours, at); !strings.Contains(window, "˘S") {
		t.Errorf("Window() = %q, want the loose caron in view", window)
	}
	if window := Window(theirs, at); !strings.Contains(window, "Š") {
		t.Errorf("Window() = %q, want the letter as printed in view", window)
	}
}

// TestOneReadingRunningOnPastTheOtherIsADisagreement, and it is reported at the
// point the shorter one stops. A model that transcribed the line above as well
// as the line is the failure this catches, and it is the one the tight cut was
// meant to make unlikely rather than impossible.
func TestOneReadingRunningOnPastTheOtherIsADisagreement(t *testing.T) {
	const ours = `contenu dans U. Comme $u$(V) est un voisinage de 0`
	at := Diff(ours, ours+` dans $u(V)$, il existe`)
	if at != len([]rune(Normalize(ours))) {
		t.Errorf("Diff() = %d, want the point our reading stops, %d", at, len([]rune(Normalize(ours))))
	}
}

// TestASilentClipIsNeitherAgreementNorDisagreement. A fleet that dropped half a
// batch would otherwise look like a result, and the percentage under it would
// be computed over the half that came back and printed as though it were the
// run.
func TestASilentClipIsNeitherAgreementNorDisagreement(t *testing.T) {
	answers := t.TempDir()
	index := Index{Book: "ts-iii-v-fr", DPI: 600, Targets: []Target{
		{Page: 22, Line: 16, Name: "0022-016.png", Native: `le théorème de ˘Smulian`},
		{Page: 85, Line: 14, Name: "0085-014.png", Native: `est un voisinage de 0`},
		{Page: 85, Line: 15, Name: "0085-015.png", Native: `un voisinage équilibré W`},
	}}
	write := func(name, text string) {
		if err := os.WriteFile(filepath.Join(answers, name), []byte(text), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	write("0022-016.md", `le théorème de Šmulian`)
	write("0085-014.md", `est un voisinage de $0$`)
	// 0085-015 never came back at all.

	report, err := Compare(index, answers)
	if err != nil {
		t.Fatalf("Compare() = %v", err)
	}
	if report.Clips != 3 || report.Agreed != 1 || report.Differed != 1 || report.SilentN != 1 {
		t.Errorf("Compare() = %d clips, %d agree, %d differ, %d silent; want 3, 1, 1, 1",
			report.Clips, report.Agreed, report.Differed, report.SilentN)
	}
	if summary := report.Summary(); !strings.Contains(summary, "50%") {
		t.Errorf("Summary() = %q, want the rate over the two that came back", summary)
	}
	markdown := report.Markdown()
	if !strings.Contains(markdown, "page 22 line 16") {
		t.Errorf("Markdown() = %q, want the disagreement in it", markdown)
	}
	if strings.Contains(markdown, "page 85 line 14") {
		t.Error("Markdown() printed a clip that agreed, want the disagreements and nothing else")
	}
	if !strings.HasSuffix(markdown, "\n") || strings.HasSuffix(markdown, "\n\n") {
		t.Errorf("Markdown() ends %q, want one newline and no blank line after it", last(markdown, 12))
	}
}

// The corpus is where these reports are committed and its audit reads a file
// that ends with a blank line as a finding. Every entry here is written with
// the blank line that separates it from the next one, so the last entry ends
// with a separator and nothing to separate, and four reports were written that
// way before the pre-commit hook refused them.
func TestTheReportDoesNotEndWithABlankLine(t *testing.T) {
	for _, report := range []Report{
		{Book: "ts-iii-v-fr", Clips: 1, Agreed: 1},
		{Book: "ts-iii-v-fr", Clips: 1, Differed: 1, Rows: []Row{
			{Page: 22, Line: WholePage, Verdict: Differ, Lost: []string{"Šmulian"}},
		}},
		{Book: "ts-iii-v-fr", Clips: 1, Differed: 1, Rows: []Row{
			{Page: 22, Line: 16, Verdict: Differ, Native: "˘Smulian", Model: "Šmulian"},
		}},
	} {
		markdown := report.Markdown()
		if !strings.HasSuffix(markdown, "\n") || strings.HasSuffix(markdown, "\n\n") {
			t.Errorf("Markdown() ends %q, want one newline and no blank line after it",
				last(markdown, 12))
		}
	}
}

func last(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// answer0022 is the first answer that ever came back from a clip, exactly as
// the tool wrote it, header and all. Page 22 of Théories spectrales, read on
// server2 in 209 seconds.
const answer0022 = `---
source: /root/bourbaki-ocr/in/ts-iii-v-fr-clip-000/0022-016.png
model: gpt-5-6
fetched: 2026-08-13 12:55
elapsed: 208.7s
conversation: https://chatgpt.com/c/6a7da29c-fa28-83eb-9dcc-eed360b9ff88
profile: /root/.cache/chatgpt-profile-15
---

de $E_\sigma$, il existe d’après le théorème de Šmulian (EVT, IV, p. 36, th. 2)`

// TestTheToolsOwnHeaderIsNotPartOfTheAnswer. ocr-batch writes four lines of
// machinery above every answer, fenced in the same three dashes a front matter
// is, and the first run of this route came back with all of them. A comparison
// that read the header as the model's reading would report every clip in the
// run as a disagreement in its first character, and the one real finding in the
// batch would be somewhere in the middle of twenty-four of those.
func TestTheToolsOwnHeaderIsNotPartOfTheAnswer(t *testing.T) {
	answers := t.TempDir()
	if err := os.WriteFile(filepath.Join(answers, "0022-016.md"), []byte(answer0022), 0o644); err != nil {
		t.Fatal(err)
	}
	index := Index{Book: "ts-iii-v-fr", Targets: []Target{{
		Page: 22, Line: 16, Name: "0022-016.png",
		Native: `de $E_{\sigma}$, il existe d’après le théorème de ˘Smulian (EVT, IV, p. 36, th. 2)`,
	}}}
	report, err := Compare(index, answers)
	if err != nil {
		t.Fatalf("Compare() = %v", err)
	}
	row := report.Rows[0]
	if strings.Contains(row.Model, "bourbaki-ocr") || strings.Contains(row.Model, "elapsed") {
		t.Errorf("Compare() kept the tool's header in the reading: %q", row.Model)
	}
	if row.Verdict != Differ {
		t.Fatalf("Compare() = %s, want the caron reported as a disagreement", row.Verdict)
	}
	// And the one place they part is the accent, not the front of the line.
	if window := Window(row.Model, row.At); !strings.Contains(window, "Šmulian") {
		t.Errorf("Window() = %q, want the disagreement at the name and not at the first character", window)
	}
}

// TestAPageIsJudgedOnItsWordsAndNotOnItsCharacters. Two readings of three
// hundred words never agree character for character, and a page audit held to
// that would report every page in the run and rank none of them. What it
// reports instead is the words one reading has and the other has not, which is
// the same question the extractor is already audited with and the same answer
// shape a person can act on.
func TestAPageIsJudgedOnItsWordsAndNotOnItsCharacters(t *testing.T) {
	const ours = `On a $u(x)\preccurlyeq u(y)$ si $x\preccurlyeq y$. Puisque ˚C est non vide, l’espace
vectoriel C C engendré par C contient un voisinage de 0, donc le cône C est total.`
	// The same passage with the mathematics spelled differently and nothing
	// added or lost. Nobody would call this a disagreement about the page.
	const same = `On a $u(x) \preceq u(y)$ si $x \preceq y$. Puisque $\mathring{C}$ est non vide,
l'espace vectoriel $C - C$ engendré par $C$ contient un voisinage de $0$, donc le cône $C$ est total.`
	answers := t.TempDir()
	if err := os.WriteFile(filepath.Join(answers, "0112.md"), []byte(same), 0o644); err != nil {
		t.Fatal(err)
	}
	index := Index{Book: "ts-iii-v-fr", Targets: []Target{
		{Page: 112, Line: WholePage, Name: "0112.png", Native: ours},
	}}
	report, err := Compare(index, answers)
	if err != nil {
		t.Fatalf("Compare() = %v", err)
	}
	if report.Agreed != 1 {
		t.Errorf("Compare() = %+v, want the page counted as agreement", report.Rows[0])
	}
	if Diff(ours, same) < 0 {
		t.Error("Diff() = -1, want these two to differ as strings: that is why a page is not judged that way")
	}
}

// TestAWordThePageHasAndWeDoNotIsWhatAPageAuditReports. This is the finding the
// whole route exists for, and at page scale it comes out as a word rather than
// as a character offset.
func TestAWordThePageHasAndWeDoNotIsWhatAPageAuditReports(t *testing.T) {
	const ours = `Soit $\varrho$ une représentation linéaire continue de G dans un espace
localement convexe E. Le théorème de ˘Smulian ne s’applique pas ici.`
	const theirs = `Soit $\varrho$ une représentation linéaire continue de G dans un espace
localement convexe E. Le théorème de Šmulian ne s'applique pas ici.`
	answers := t.TempDir()
	if err := os.WriteFile(filepath.Join(answers, "0390.md"), []byte(theirs), 0o644); err != nil {
		t.Fatal(err)
	}
	index := Index{Targets: []Target{
		{Page: 390, Line: WholePage, Name: "0390.png", Native: ours},
	}}
	report, err := Compare(index, answers)
	if err != nil {
		t.Fatalf("Compare() = %v", err)
	}
	row := report.Rows[0]
	if row.Verdict != Differ {
		t.Fatalf("Compare() = %s, want the loose caron reported", row.Verdict)
	}
	if !slices.Contains(row.Lost, "Šmulian") {
		t.Errorf("Compare() lost = %v, want the name as the page spells it", row.Lost)
	}
	// And the other direction has our broken spelling, which is the half that
	// says where the defect is rather than what it should be.
	if !slices.Contains(row.Extra, "Smulian") {
		t.Errorf("Compare() extra = %v, want our own spelling of it", row.Extra)
	}
}

// TestTheRunningHeadIsNotAFinding. It is the first thing the model writes on
// every page and it is the one thing the extractor keeps out of the body, so
// the first page run of Théories spectrales reported ENDOMORPHISMES, ESPACES
// and BANACH against six pages of seven and reported almost nothing else. Where
// two readings file the furniture of a page is not a defect in either of them.
func TestTheRunningHeadIsNotAFinding(t *testing.T) {
	const ours = `Le projecteur spectral $e_H(u)$ est de rang fini.`
	const theirs = `TS III.84 ENDOMORPHISMES DES ESPACES DE BANACH § 6

Le projecteur spectral $e_{\mathrm{H}}(u)$ est de rang fini.`
	index := Index{Targets: []Target{{
		Page: 98, Line: WholePage, Name: "0098.png", Native: ours,
		Head: "TS III.84 ENDOMORPHISMES DES ESPACES DE BANACH",
	}}}
	report, err := compareOne(t, index, "0098.md", theirs)
	if err != nil {
		t.Fatal(err)
	}
	if report.Rows[0].Verdict != Agree {
		t.Errorf("Compare() = %s lost %v, want the head thrown away and the page counted as agreement",
			report.Rows[0].Verdict, report.Rows[0].Lost)
	}
}

// TestSmallCapitalsAndABrokenWordAreNotFindingsEither. Bourbaki sets the word
// that opens a statement in small capitals, and a model reading the picture
// writes what it sees, which is capitals. A word the compositor broke across a
// line keeps its hyphen in the model's reading and never had one in ours. Both
// came back as findings on the first run and neither is anything to fix.
func TestSmallCapitalsAndABrokenWordAreNotFindingsEither(t *testing.T) {
	const ours = `Corollaire 2. — L’ensemble des points extrémaux est compact.`
	const theirs = `COROLLAIRE 2. — L'en-semble des points extrémaux est compact.`
	index := Index{Targets: []Target{
		{Page: 22, Line: WholePage, Name: "0022.png", Native: ours},
	}}
	report, err := compareOne(t, index, "0022.md", theirs)
	if err != nil {
		t.Fatal(err)
	}
	if report.Rows[0].Verdict != Agree {
		t.Errorf("Compare() = %s lost %v extra %v, want the case and the hyphen folded",
			report.Rows[0].Verdict, report.Rows[0].Lost, report.Rows[0].Extra)
	}
}

// TestOneNamePairSetTwoWaysIsOneReading. The compositor sets Krein and Rutman
// with an en dash and the model types a hyphen, and the first run reported the
// pair as a finding on two pages because only the hyphen counted as a joiner.
func TestOneNamePairSetTwoWaysIsOneReading(t *testing.T) {
	const ours = `D’après le théorème de Krein–Rutman (th. 4), le nombre réel est une valeur propre.`
	const theirs = `D'après le théorème de Krein-Rutman (th. 4), le nombre réel est une valeur propre.`
	index := Index{Targets: []Target{
		{Page: 112, Line: WholePage, Name: "0112.png", Native: ours},
	}}
	report, err := compareOne(t, index, "0112.md", theirs)
	if err != nil {
		t.Fatal(err)
	}
	if report.Rows[0].Verdict != Agree {
		t.Errorf("Compare() = %s lost %v extra %v, want the en dash and the hyphen folded together",
			report.Rows[0].Verdict, report.Rows[0].Lost, report.Rows[0].Extra)
	}
}

// TestFoldingTheCaseDoesNotFoldTheAccents, which would throw the finding out
// with the noise: the two are one letter apart and only one of them is a defect.
func TestFoldingTheCaseDoesNotFoldTheAccents(t *testing.T) {
	const ours = `Le théorème de Smulian et le lemme de Krein.`
	const theirs = `Le THÉORÈME de Šmulian et le lemme de KREIN.`
	index := Index{Targets: []Target{
		{Page: 390, Line: WholePage, Name: "0390.png", Native: ours},
	}}
	report, err := compareOne(t, index, "0390.md", theirs)
	if err != nil {
		t.Fatal(err)
	}
	row := report.Rows[0]
	if !slices.Contains(row.Lost, "Šmulian") {
		t.Errorf("Compare() lost = %v, want the caron still reported", row.Lost)
	}
	for _, word := range append(append([]string{}, row.Lost...), row.Extra...) {
		if strings.EqualFold(word, "théorème") || strings.EqualFold(word, "krein") {
			t.Errorf("Compare() reported %q, want the small capitals folded away", word)
		}
	}
}

// TestTheServicesApologyIsSilenceAndNotADisagreement. It arrives in the answer
// file with nothing else in it, and a page audit that read it as a reading
// reported every word of the page as a word the extractor had invented.
func TestTheServicesApologyIsSilenceAndNotADisagreement(t *testing.T) {
	index := Index{Targets: []Target{{
		Page: 113, Line: WholePage, Name: "0113.png",
		Native: "Le sous-espace Ker($\\ell$) de F est stable par $v$.",
	}}}
	report, err := compareOne(t, index, "0113.md", "Hmm...something seems to have gone wrong.")
	if err != nil {
		t.Fatal(err)
	}
	if report.Rows[0].Verdict != Silent || report.SilentN != 1 {
		t.Errorf("Compare() = %s, want the apology counted as a clip that was not read", report.Rows[0].Verdict)
	}
}

// compareOne is one page written to a scratch directory and judged, which is
// three lines every one of these tests would otherwise spell out.
func compareOne(t *testing.T, index Index, name, answer string) (Report, error) {
	t.Helper()
	answers := t.TempDir()
	if err := os.WriteFile(filepath.Join(answers, name), []byte(answer), 0o644); err != nil {
		t.Fatal(err)
	}
	return Compare(index, answers)
}

// TestTheRowsAreInReadingOrder however the answers arrived. Two hosts finish
// their batches in whatever order they finish them, and a report a person opens
// is read from the front of the volume to the back.
func TestTheRowsAreInReadingOrder(t *testing.T) {
	index := Index{Targets: []Target{
		{Page: 111, Line: 9, Name: "0111-009.png"},
		{Page: 22, Line: 16, Name: "0022-016.png"},
		{Page: 85, Line: 15, Name: "0085-015.png"},
		{Page: 85, Line: 14, Name: "0085-014.png"},
	}}
	report, err := Compare(index, t.TempDir())
	if err != nil {
		t.Fatalf("Compare() = %v", err)
	}
	var got []int
	for _, row := range report.Rows {
		got = append(got, row.Page*1000+row.Line)
	}
	want := []int{22016, 85014, 85015, 111009}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Compare() rows = %v, want them in reading order %v", got, want)
		}
	}
}

// TestAnEmptyAnswerIsSilentAndNotAgreement. A model that answered with a blank
// file has said nothing, and a comparison that normalised it to the empty
// string would agree with any line that normalises to nothing and disagree with
// the rest for the wrong reason.
func TestAnEmptyAnswerIsSilentAndNotAgreement(t *testing.T) {
	answers := t.TempDir()
	if err := os.WriteFile(filepath.Join(answers, "0022-016.md"), []byte("   \n"), 0o644); err != nil {
		t.Fatal(err)
	}
	index := Index{Targets: []Target{{Page: 22, Line: 16, Name: "0022-016.png", Native: "anything"}}}
	report, err := Compare(index, answers)
	if err != nil {
		t.Fatalf("Compare() = %v", err)
	}
	if report.SilentN != 1 || report.Rows[0].Verdict != Silent {
		t.Errorf("Compare() = %+v, want the blank answer counted as silence", report.Rows[0])
	}
}
