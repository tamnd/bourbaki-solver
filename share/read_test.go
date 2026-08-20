package share

import (
	"strings"
	"testing"
)

// The fixtures of audit_test are reused here on purpose. The two commands read
// the same material and a reading sheet built on pages the audit has never seen
// would prove nothing about the pages the audit passes.

func TestAnImportOfTheWholeSectionInventsNothingAndDropsNothing(t *testing.T) {
	s := Read(target(), whole(), section())
	if n := s.Missing(); n != 0 {
		t.Fatalf("want nothing missing from the pages, got %d: %v", n, s.Pages)
	}
	if len(s.Added) != 0 {
		t.Fatalf("want nothing invented, got %d: %v", len(s.Added), s.Added)
	}
	if len(s.Pages) != 2 {
		t.Fatalf("want both pages read, got %d", len(s.Pages))
	}
	for _, p := range s.Pages {
		if p.Found < 0.99 {
			t.Fatalf("pdf %d is wholly in the import and scored %.2f", p.PDFPage, p.Found)
		}
	}
}

func TestASentenceTheBookDoesNotPrintIsReported(t *testing.T) {
	invented := "It follows at once that every quasi-regular submodule of a finite tower is itself closed under the operation just defined."
	s := Read(target(), whole()+"\n\n"+invented+"\n", section())
	if len(s.Added) != 1 {
		t.Fatalf("want the one invented sentence, got %d: %v", len(s.Added), s.Added)
	}
	if s.Added[0].Text != invented {
		t.Fatalf("want the invented sentence back as it stands, got %q", s.Added[0].Text)
	}
	if s.Added[0].Found != 0 {
		t.Fatalf("want none of it found, got %.2f", s.Added[0].Found)
	}
	if s.Added[0].PDFPage != 0 {
		t.Fatalf("a sentence of the import is on no page and should carry no page number, got %d", s.Added[0].PDFPage)
	}
}

func TestASentenceOfThePageTheImportLacksIsReportedAgainstThatPage(t *testing.T) {
	// The import is the whole section with the last sentence of page 23 taken
	// out, which is what a transcription that stopped a line early leaves.
	full := whole()
	all := sentences(prose(23))
	last := all[len(all)-1]
	cut := strings.Replace(full, last, "", 1)
	if cut == full {
		t.Fatalf("the fixture did not change, so this test proves nothing")
	}
	s := Read(target(), cut, section())
	if n := s.Missing(); n != 1 {
		t.Fatalf("want the one dropped sentence, got %d: %v", n, s.Pages)
	}
	for _, p := range s.Pages {
		if p.PDFPage == 23 && len(p.Missing) == 1 && p.Missing[0].PDFPage == 23 {
			return
		}
	}
	t.Fatalf("want the sentence reported against pdf 23, got %v", s.Pages)
}

func TestTheClosingParagraphPrintedAboveTheNextHeadingIsNotAnInvention(t *testing.T) {
	// A § almost never ends at the foot of a page. Its last paragraph is printed
	// at the head of the page the next § starts on, and that page belongs to the
	// next §. Without it every § in the library ends in an invention.
	tail := "The theory so obtained is the one in which every assembly of the preceding kind may be written down in full."
	p := section()
	p.Pages = p.Pages[:1]
	body := "## 1. TERMS AND RELATIONS\n\n" + prose(22) + "\n\n" + tail + "\n"

	bare := Read(target(), body, p)
	if len(bare.Added) != 1 || bare.Added[0].Text != tail {
		t.Fatalf("without the boundary page the closing paragraph must read as invented, got %v", bare.Added)
	}

	p.After = []PrintedPage{{PDFPage: 23, Text: tail + "\n\n### 2. CRITERIA OF SUBSTITUTION\n\n" + prose(24)}}
	s := Read(target(), body, p)
	if len(s.Added) != 0 {
		t.Fatalf("with the boundary page it is printed and must not be reported, got %v", s.Added)
	}
	if len(s.Pages) != 1 {
		t.Fatalf("the boundary page is the next §'s and must not be read as this one's, got %d pages", len(s.Pages))
	}
}

func TestTheBoundaryPageIsNotDemandedOfTheImport(t *testing.T) {
	// The other direction of the same rule. Most of that page is the next §, so
	// an import that does not carry it is right and must not be marked short
	// for it.
	p := section()
	p.Pages = p.Pages[:1]
	p.After = []PrintedPage{{PDFPage: 23, Text: prose(23)}}
	s := Read(target(), "## 1. TERMS AND RELATIONS\n\n"+prose(22)+"\n", p)
	if n := s.Missing(); n != 0 {
		t.Fatalf("the boundary page belongs to the next § and must not be asked of this import, got %d missing", n)
	}
}

func TestAFullStopInsideAFormulaDoesNotEndASentence(t *testing.T) {
	got := sentences("If the relation $f(x) = 1.5$ holds for every $x$ in $E$, then $E$ is said to be full.")
	if len(got) != 1 {
		t.Fatalf("want one sentence, got %d: %q", len(got), got)
	}
	if !strings.Contains(got[0], "$f(x) = 1.5$") {
		t.Fatalf("the formula must come back as it stands, got %q", got[0])
	}
}

func TestSentencesComeBackWithTheirMathematicsInPlace(t *testing.T) {
	got := sentences("Let $A$ be a relation. Then $\\mathscr{T}$ is a theory.")
	if len(got) != 2 {
		t.Fatalf("want two sentences, got %d: %q", len(got), got)
	}
	if got[0] != "Let $A$ be a relation." {
		t.Fatalf("got %q", got[0])
	}
	if got[1] != "Then $\\mathscr{T}$ is a theory." {
		t.Fatalf("got %q", got[1])
	}
}

func TestASentenceTooShortToMeasureIsCountedAndNotListed(t *testing.T) {
	p := section()
	p.Pages = []PrintedPage{{PDFPage: 22, Text: "Let $A$ be a relation.\n"}}
	s := Read(target(), "nothing of the sort", p)
	if len(s.Pages) != 1 {
		t.Fatalf("want the one page, got %d", len(s.Pages))
	}
	if len(s.Pages[0].Missing) != 0 {
		t.Fatalf("a sentence of five words cannot be judged and must not be listed, got %v", s.Pages[0].Missing)
	}
	if s.Pages[0].Short != 1 {
		t.Fatalf("want it counted once, got %d", s.Pages[0].Short)
	}
}

func TestTheFrontMatterOfAnImportIsNotProse(t *testing.T) {
	head := "---\nbook: sets\nchapter: 1\nsection: 1\nshare_title: A very long recorded title of the conversation this came out of\n---\n"
	s := Read(target(), head+whole(), section())
	for _, a := range s.Added {
		if strings.Contains(a.Text, "share_title") || strings.Contains(a.Text, "book: sets") {
			t.Fatalf("the head is provenance and not the book, and it reached the sheet: %q", a.Text)
		}
	}
	if len(s.Added) != 0 {
		t.Fatalf("want nothing invented, got %v", s.Added)
	}
}

func TestTheSheetSaysWhatItPassedOver(t *testing.T) {
	if got := shortNote(0); !strings.Contains(got, "Every sentence") {
		t.Fatalf("got %q", got)
	}
	if got := shortNote(1); !strings.Contains(got, "1 sentence under") {
		t.Fatalf("want the singular, got %q", got)
	}
	if got := shortNote(4); !strings.Contains(got, "4 sentences under") {
		t.Fatalf("want the plural, got %q", got)
	}
}

func TestTheSummaryCountsBothDirections(t *testing.T) {
	s := &Sheet{
		Target: target(),
		Pages: []PageReading{
			{PDFPage: 22, Missing: []Passage{{Text: "one"}, {Text: "two"}}},
			{PDFPage: 23},
		},
		Added: []Passage{{Text: "three"}},
	}
	got := s.Summary()
	for _, want := range []string{"2 printed pages", "2 sentences of the pages", "1 sentence of the import"} {
		if !strings.Contains(got, want) {
			t.Fatalf("want %q in the summary, got %q", want, got)
		}
	}
}

func TestThePagesAreReadInPrintedOrder(t *testing.T) {
	s := &Sheet{Pages: []PageReading{{PDFPage: 24}, {PDFPage: 22}, {PDFPage: 23}}}
	s.SortPages()
	for i, want := range []int{22, 23, 24} {
		if s.Pages[i].PDFPage != want {
			t.Fatalf("want %v, got %v", []int{22, 23, 24}, s.Pages)
		}
	}
}

func TestTheSheetNamesBothHalvesEvenWhenAHalfIsEmpty(t *testing.T) {
	// A sheet that prints nothing under a heading it dropped reads like a sheet
	// that had nothing to say about it, and the empty half here is the half the
	// audit cannot see.
	md := Read(target(), whole(), section()).Markdown("ens-i-iv")
	if !strings.Contains(md, "## In the import and on no page of the section") {
		t.Fatalf("the invented half must be named even when it is empty:\n%s", md)
	}
	if !strings.Contains(md, "## pdf page 22") || !strings.Contains(md, "## pdf page 23") {
		t.Fatalf("both pages must be named:\n%s", md)
	}
	if !strings.Contains(md, "read against ens-i-iv") {
		t.Fatalf("the sheet must say which volume it was held against:\n%s", md)
	}
}

func TestADisplayedFormulaDoesNotCutTheSentenceItSitsIn(t *testing.T) {
	// Bourbaki writes a sentence, sets a formula on a line of its own in the
	// middle of it, and finishes underneath. Cutting there leaves two fragments
	// that match nothing on the other side, since neither side displays a
	// formula in the same place.
	got := sentences("By S2, the relation\n\n$$ (\\operatorname{not} A) \\Rightarrow B $$\n\nis a theorem in the theory.\n")
	if len(got) != 1 {
		t.Fatalf("want the one sentence it is, got %d: %q", len(got), got)
	}
	if !strings.Contains(got[0], "By S2") || !strings.Contains(got[0], "is a theorem in the theory.") {
		t.Fatalf("both halves belong to the sentence, got %q", got[0])
	}
}

func TestAFullStopInsideADisplayStillEndsTheSentence(t *testing.T) {
	// The stop that closes the sentence is very often set inside the dollars.
	// Without looking there the sentence reads as open and swallows whatever is
	// printed under it, which in § 3 was the criterion CS7.
	got := sentences("The assembly will be denoted by\n\n$$ A \\Leftrightarrow B. $$\n\n**CS7.** Let $A$ and $B$ be assemblies of the theory.\n")
	if len(got) != 2 {
		t.Fatalf("want the sentence and the criterion under it, got %d: %q", len(got), got)
	}
	if !strings.HasPrefix(got[1], "**CS7.**") {
		t.Fatalf("the criterion is its own sentence, got %q", got[1])
	}
}

func TestAHeadingIsNotSwallowedByTheParagraphAboveIt(t *testing.T) {
	// A heading ends in a word and not in a full stop, so a rule that joined
	// every paragraph left open would take the heading of every no. in the
	// library into the prose above it.
	got := sentences("### 1. SIGNS AND ASSEMBLIES\n\nA sign of the theory is one of the letters written down in advance.\n")
	if len(got) != 2 || got[0] != "### 1. SIGNS AND ASSEMBLIES" {
		t.Fatalf("want the heading on its own, got %q", got)
	}
}

func TestProseSetInsideAFormulaIsReadAsProse(t *testing.T) {
	// The page sets "A or (not A)" with the or and the not as words between
	// three short formulae. The import writes one formula with \text{ or } and
	// \operatorname{not} inside it. Both render the same line of type, and if
	// the macros are dropped with the mathematics the two readings differ in
	// every connective they have.
	page := words(`then "$A$ or (not $A$)" is a theorem`)
	imported := words(`then "$A\text{ or }(\operatorname{not}A)$" is a theorem`)
	if strings.Join(page, " ") != strings.Join(imported, " ") {
		t.Fatalf("the same printed line read two ways: %q against %q", page, imported)
	}
	if !strings.Contains(strings.Join(imported, " "), "or") {
		t.Fatalf("the connective is printed as a word and must be read as one, got %q", imported)
	}
}

func TestTheMathematicsItselfIsStillNotProse(t *testing.T) {
	// The point of stripping the mathematics is that the two sides argue about
	// markup. Lifting the prose macros must not lift the macro names with them.
	got := strings.Join(words(`the spaces $\mathscr{L}_{\mathfrak{S}}(E; F)$ are complete`), " ")
	if strings.Contains(got, "mathscr") || strings.Contains(got, "mathfrak") {
		t.Fatalf("a macro name is not a word, got %q", got)
	}
	if got != "the spaces are complete" {
		t.Fatalf("got %q", got)
	}
}

func TestTheSheetWarnsThatTheFirstPageCarriesThePreviousSection(t *testing.T) {
	p := section()
	// Page 22 opens the § partway down, with the end of the § before it above
	// the heading. That prose is on the page and is nobody's business here.
	p.Pages[0].Text = "The closing sentence of the section before this one is printed above the heading of this one.\n\n" + p.Pages[0].Text
	md := Read(target(), whole(), p).Markdown("ens-i-iv")
	if !strings.Contains(md, "may belong to that § and be missing from nothing") {
		t.Fatalf("the sheet has to say why a passage at the head of the first page may be nobody's fault:\n%s", md)
	}
}

func TestANumberedHeadingIsOneHeadingAndNotTwoSentences(t *testing.T) {
	got := sentences("### 1. Signs and assemblies\n")
	if len(got) != 1 {
		t.Fatalf("want the one heading, got %d: %q", len(got), got)
	}
}

func TestTheAbbreviationBourbakiUsesOnEveryPageDoesNotEndASentence(t *testing.T) {
	got := sentences("This follows by C1 (§ 2, no. 2) applied to the relation just written down.")
	if len(got) != 1 {
		t.Fatalf("no. is a numero and not the end of anything, got %d: %q", len(got), got)
	}
}

func TestASentenceEndingOnANumeralStillEnds(t *testing.T) {
	// Half the sentences of this book close on the name of an axiom, so the
	// rule about numerals has to be about numerals standing alone under markup
	// and not about digits.
	got := sentences("The relation is an axiom by S2 and S1. The result follows from that at once.")
	if len(got) != 2 {
		t.Fatalf("want two sentences, got %d: %q", len(got), got)
	}
}
