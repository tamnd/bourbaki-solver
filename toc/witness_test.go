package toc

import (
	"strings"
	"testing"
)

// Every page in this file is pdftotext run over the publisher's own scan, which
// is what the witness reads.
func TestWitnessSectionReadsASignTheScanMisread(t *testing.T) {
	for _, c := range []struct {
		page   string
		number int
		title  string
		want   string
	}{
		// The sign as the book prints it, and the four ways this corpus was
		// found with it misread.
		{"§ 7. EXTERIOR ALGEBRAS\n1. DEFINITION OF THE EXTERIOR ALGEBRA OF A MODULE\n", 7, "Exterior algebras", "EXTERIOR ALGEBRAS"},
		{"5 8. ANNEAUX\n1. Anneaux\nDEFINITION\n", 8, "Anneaux", "ANNEAUX"},
		{"$11. MODULES ET ANNEAUX GRADUÉS\nA partir du no 2 de ce paragraphe.\n", 11, "Modules et anneaux gradués", "MODULES ET ANNEAUX GRADUÉS"},
		{"$ 3 . ESPACES PROJECTIFS RÉELS\n1. Structure de variété.\n", 3, "Espaces projectifs réels", "ESPACES PROJECTIFS RÉELS"},
		{"5 2. Réflexions\n1. Pseudo-réflexions.\n", 2, "Réflexions", "Réflexions"},
	} {
		got, ok := WitnessSection(c.page, c.number, c.title)
		if !ok || got != c.want {
			t.Errorf("WitnessSection(%q, %d) = %q, %v, want %q", c.page, c.number, got, ok, c.want)
		}
	}
}

// The press broke a long title at the measure and the scan kept it broken, so
// the run of lines is what is compared and not the first of them. Both of these
// are real: § 6 of chapter IX of Topologie generale V a X and § 2 of chapter II
// of Algebre I a III, the second with the accented E read as a G.
func TestWitnessSectionJoinsATitleBrokenAtTheMeasure(t *testing.T) {
	page := "ESPACES POLONAIS ; ESPACES SOUSLINIENS\n" +
		"$6,N0 1\n" +
		"T G IX.57\n" +
		"\n" +
		"6. ESPACES POLONAIS; ESPACES SOUSLINIENS;\n" +
		"ENSEMBLES BORÉLIENS\n"
	got, ok := WitnessSection(page, 6, "Espaces polonais ; espaces sousliniens ; ensembles boréliens")
	if !ok {
		t.Fatal("the heading two lines below the running head was not found")
	}
	if want := "ESPACES POLONAIS; ESPACES SOUSLINIENS; ENSEMBLES BORÉLIENS"; got != want {
		t.Errorf("WitnessSection() = %q, want %q", got, want)
	}

	page = "5 2. MODULES D'APPLICATIONS LINGAIRES.\nDUALITÉ\n1. Propriétés de HomA(E,F).\n"
	if got, ok := WitnessSection(page, 2, "Modules d’applications linéaires. Dualité"); !ok {
		t.Errorf("WitnessSection() = %q, false, want the two lines joined", got)
	}
}

// The running head carries the title and no number, and that is the whole of
// what keeps this rule off it.
func TestWitnessSectionRefusesTheRunningHead(t *testing.T) {
	page := "186\n\nCh. VI\n\nROOT SYSTEMS\n\nFor a E R and k E Z, let La,k be the hyperplane.\n"
	if got, ok := WitnessSection(page, 2, "Root systems"); ok {
		t.Errorf("WitnessSection() = %q, true, want the running head refused", got)
	}
}

// The first no. of a § is set under the same words as the § on many pages, and
// it carries the number 1. Page 199 of Lie IV to VI is the case: the scan lost
// the § heading and kept "1. AFFINE WEYL GROUP", which is no. 1 of § 2.
func TestWitnessSectionRefusesTheFirstNumber(t *testing.T) {
	page := "186\n\nCh. VI\n\nROOT SYSTEMS\n\n1. AFFINE WEYL GROUP\n\nFor a E R and k E Z.\n"
	if got, ok := WitnessSection(page, 2, "Affine Weyl group"); ok {
		t.Errorf("WitnessSection() = %q, true, want no. 1 refused for § 2", got)
	}
}

// A five in front of a number is a section sign the scan misread. A five that is
// the number is the number, and nothing in the sign class eats it.
func TestWitnessSectionKeepsAFiveThatIsTheNumber(t *testing.T) {
	got, ok := WitnessSection("5. COMMUTATION\n1. Le commutant.\n", 5, "Commutation")
	if !ok || got != "COMMUTATION" {
		t.Errorf("WitnessSection() = %q, %v, want COMMUTATION", got, ok)
	}
	if got, ok := WitnessSection("5. COMMUTATION\n", 8, "Commutation"); ok {
		t.Errorf("WitnessSection() = %q, true, want the five read as the number and refused for § 8", got)
	}
}

// The head of a historical note is two words on a line and the line under it
// belongs to the note. Both pages here are pdftotext over the publisher's scan
// of the page the contents opens the note on, and in both the reading that made
// the corpus dropped the head and kept everything under it.
func TestHistoricalNoteReadsTheHeadAndNotTheLineUnderIt(t *testing.T) {
	page := "HISTORICAL NOTE\n(Chapters II and III)\n\nThe notion of a group.\n"
	at, rest := HistoricalNote(strings.Split(page, "\n"))
	if at != 0 || rest != "" {
		t.Fatalf("HistoricalNote() = %d, %q, want 0 and nothing after the head", at, rest)
	}
	if _, ok := WitnessHistorical(page); !ok {
		t.Error("WitnessHistorical() = false, want the head found")
	}

	page = "NOTE HISTORIQUE\n(N.-B. - Les chiffres romains entre parenthèses renvoient à la bibliographie\n"
	if at, _ := HistoricalNote(strings.Split(page, "\n")); at != 0 {
		t.Errorf("HistoricalNote() = %d, want 0", at)
	}
}

// Four pages of this corpus have a text layer that is itself a scan, and the
// head comes back from it letterspaced, misread or run together with the
// chapters the note covers. Every line here is pdftotext over the page the
// contents opens the note on.
func TestHistoricalNoteReadsAHeadTheScanMangled(t *testing.T) {
	for _, c := range []struct {
		line, rest string
	}{
		// Algebre IX page 183, the letterspacing read as spaces.
		{"                       NOTE H I S T O R I Q U E", ""},
		// Integration VI page 100, the R read as a K.
		{"                        NOTE HISTOKIQUE", ""},
		// Integration IX page 111, the N read as a K.
		{"                                 KOTE HISTORIQUE", ""},
		// Lie IV a VI page 233, the chapters set on the head's own line.
		{"             NOTE HISTORIQUE (chapitres IV, V et VI).", "(chapitres IV, V et VI)"},
	} {
		rest, ok := WitnessHistorical(c.line + "\n")
		if !ok || rest != c.rest {
			t.Errorf("WitnessHistorical(%q) = %q, %v, want %q, true", c.line, rest, ok, c.rest)
		}
	}
}

// One letter is as far as it goes. Two wrong letters is a page to read again.
func TestHistoricalNoteRefusesAHeadTooFarGone(t *testing.T) {
	for _, line := range []string{
		"KOTE HISTOKIQUE",
		"NOTE",
		"HISTORIQUE",
		"NOTE HISTORIQUE DES CHAPITRES",
	} {
		if rest, ok := WitnessHistorical(line + "\n"); ok {
			t.Errorf("WitnessHistorical(%q) = %q, true, want refused", line, rest)
		}
	}
}

// A line that only begins with the words is a line of the note. This is what
// keeps the rule off "(Chapters II and III)" when the head above it is already
// there, and off a sentence in the note that names itself.
func TestHistoricalNoteRefusesALineThatOnlyBeginsWithTheWords(t *testing.T) {
	for _, page := range []string{
		"Historical note on the theory of sets, by N. Bourbaki.\n",
		"# HISTORICAL NOTE\n",
		"> NOTE HISTORIQUE\n",
		"(Chapters II and III)\n",
	} {
		if _, ok := WitnessHistorical(page); ok {
			t.Errorf("WitnessHistorical(%q) = true, want refused", page)
		}
	}
}

// The reading puts bold round display type often enough that the head comes back
// wrapped in it, and that is the same head.
func TestHistoricalNoteReadsItThroughBold(t *testing.T) {
	if _, ok := WitnessHistorical("**HISTORICAL NOTE**\n"); !ok {
		t.Error("WitnessHistorical() = false, want the bold head found")
	}
}

// The mark that opens a § inside a block of gathered exercises is the sign and
// the number alone. Page 671 of Algebra I to III is the one page in this corpus
// whose text layer still has a mark the reading dropped, and it gives "§II",
// with no space and both 1s read as capital I.
func TestWitnessMarkReadsTheSignAndTheNumberAlone(t *testing.T) {
	for _, c := range []struct {
		page   string
		number int
	}{
		{"                                     §II\n", 11},
		{"EXERCISES\n\n§ 8\n\n1) Soit G = SU(2, C).\n", 8},
		{"$ 3.\n", 3},
		{"**§ 2**\n", 2},
		{"S 10\n", 10},
	} {
		if !WitnessMark(c.page, c.number) {
			t.Errorf("WitnessMark(%q, %d) = false, want the mark found", c.page, c.number)
		}
	}
}

// A block of exercises is full of short lines that are a number and nothing
// else, and none of them is a mark. The sign is what tells them apart, so a line
// with no sign on it is refused however well the number matches, and a five or a
// nine standing alone is a number and not a sign the scan misread.
func TestWitnessMarkRefusesALineWithNoSign(t *testing.T) {
	for _, c := range []struct {
		page   string
		number int
	}{
		{"11\n", 11},
		{"(3)\n", 3},
		{"5 2\n", 2},
		{"9 4\n", 4},
		{"§ 7\n", 8},
		{"§ 6. ESPACES POLONAIS\n", 6},
		{"see § 4 below\n", 4},
	} {
		if WitnessMark(c.page, c.number) {
			t.Errorf("WitnessMark(%q, %d) = true, want refused", c.page, c.number)
		}
	}
}

// A title the page does not carry is not witnessed by a number on its own.
func TestWitnessSectionRefusesAnotherTitle(t *testing.T) {
	if got, ok := WitnessSection("§ 7. TENSOR ALGEBRAS\n", 7, "Exterior algebras"); ok {
		t.Errorf("WitnessSection() = %q, true, want a different title refused", got)
	}
}

func TestAHistoricalNoteFiledAsTheRunningHeadIsRead(t *testing.T) {
	// All six chapters of General Topology V to X are in this state: the page
	// the contents opens the note on carries running_head "HISTORICAL NOTE" and
	// a body that opens on the note's first sentence. The parenthetical a head
	// is sometimes set with comes back rather than being swallowed.
	for _, c := range []struct {
		running string
		rest    string
	}{
		{"HISTORICAL NOTE", ""},
		{"  HISTORICAL NOTE  ", ""},
		{"NOTE HISTORIQUE (chapitres IV, V et VI).", "(chapitres IV, V et VI)"},
	} {
		rest, ok := HistoricalNoteFromHead(c.running)
		if !ok {
			t.Fatalf("%q reads as the head of a historical note", c.running)
		}
		if rest != c.rest {
			t.Errorf("got %q, want %q", rest, c.rest)
		}
	}
}

func TestAnOrdinaryRunningHeadIsNotAHistoricalNote(t *testing.T) {
	for _, running := range []string{
		"", "MEASUREMENT OF MAGNITUDES", "HISTORICAL NOTE ON THE THEORY OF SETS",
	} {
		if _, ok := HistoricalNoteFromHead(running); ok {
			t.Errorf("%q was read as the head of a historical note", running)
		}
	}
}
