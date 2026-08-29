package toc

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/pagemap"
)

// Every page in this file is a page of chapter X of Algebre commutative or of
// chapter VII of Groupes et algebres de Lie, cut down to the lines that decide
// something and otherwise left exactly as the reading wrote it. The running
// heads are the ones on the paper, damage included: page 3 of Algebre
// commutative really does head "PROFONDEUR., RÉGULARITÉ, DUALITÉ" with a full
// stop after the first word, and page 23 of Lie really does set its no. heading
// in bold. Those are the lines the reader has to get right, so those are the
// lines it is tested on.

// bodyMap is the page map of a fragment of one chapter, numbered straight
// through, which is what both of these volumes are.
func bodyMap(chapter string, firstPDF, lastPDF, firstPage int) *pagemap.Map {
	m := &pagemap.Map{Book: "test", Pagination: pagemap.Continuous,
		PDFPages: lastPDF,
		Chapters: []pagemap.Span{{Chapter: chapter, FirstPDF: firstPDF,
			LastPDF: lastPDF, FirstPage: firstPage,
			LastPage: firstPage + lastPDF - firstPDF}}}
	for pdf := firstPDF; pdf <= lastPDF; pdf++ {
		m.Entries = append(m.Entries, pagemap.Entry{PDFPage: pdf,
			Chapter: chapter, Page: firstPage + pdf - firstPDF,
			Confidence: pagemap.FromHead})
	}
	return m
}

// acPages is the opening of chapter X of Algebre commutative. Its § 1 heading
// and its no. 1 heading are both printed on page X.2, which is a leaf the scan
// does not have, so the first heading in the file is no. 2 on page X.4.
func acPages() []BodyPage {
	return []BodyPage{
		{PDFPage: 1, RunningHead: "CHAPITRE X", Body: "" +
			"Profondeur, régularité, dualité\n" +
			"\n" +
			"Dans ce chapitre, tous les anneaux sont supposés commutatifs, les " +
			"algèbres sont associatives, commutatives et unifères.\n"},
		{PDFPage: 2, RunningHead: "N° 1 PROFONDEUR", Body: "" +
			"PROPOSITION 1.— Soient $A$ un anneau, $J$ un idéal de $A$ et " +
			"$0 \\to M' \\to M \\to M'' \\to 0$ une suite exacte de $A$-modules.\n"},
		{PDFPage: 3, RunningHead: "PROFONDEUR., RÉGULARITÉ, DUALITÉ", Section: 1,
			Body: "" +
				"2. Profondeur et acyclicité\n" +
				"\n" +
				"Soit $A$ un anneau noethérien.\n"},
		{PDFPage: 4, RunningHead: "N° 3 PROFONDEUR", Body: "" +
			"3. Profondeur et complexe de Koszul\n" +
			"\n" +
			"Dans ce numéro, $A$ désigne un anneau.\n"},
	}
}

// acMap is the page map of that chapter, which records the leaf. Printed page 2
// is not in the file, so pdf page 2 prints page 3 and the offset steps there.
func acMap() *pagemap.Map {
	m := bodyMap("X", 1, 4, 1)
	for i := range m.Entries {
		if m.Entries[i].PDFPage > 1 {
			m.Entries[i].Page++
		}
	}
	m.Chapters[0].LastPage++
	m.Steps = []pagemap.Step{{AtPDFPage: 2, Chapter: "X", FromOffset: 0,
		ToOffset: -1, MissingPages: []int{2}}}
	return m
}

// The § whose heading is on the missing leaf is still opened, and it is still
// named, because the recto running heads carry its title on every page of it.
// It opens at the first page of itself the file has and not at the chapter's
// own first page, which is front matter and belongs before the §.
func TestBodyOpensTheSectionWhoseHeadingIsOnAMissingLeaf(t *testing.T) {
	res := FromBody(acPages(), acMap(), Options{Book: "ac-x-fr"})
	if len(res.Chapters) != 1 {
		t.Fatalf("chapters = %d, want 1", len(res.Chapters))
	}
	ch := res.Chapters[0]
	if ch.Title != "Profondeur, régularité, dualité" {
		t.Errorf("chapter title = %q, want what page 1 prints", ch.Title)
	}
	if len(ch.Sections) != 1 {
		t.Fatalf("sections = %d, want the one §", len(ch.Sections))
	}
	s := ch.Sections[0]
	if s.Number != 1 || s.PDFPage != 2 || s.Page != 3 {
		t.Errorf("§ = %d at printed %d pdf %d, want § 1 at pdf 2, printed 3",
			s.Number, s.Page, s.PDFPage)
	}
	if s.Title != "PROFONDEUR" {
		t.Errorf("§ 1 title = %q, want the recto head and not the chapter", s.Title)
	}
	if len(s.Subsections) != 2 || s.Subsections[0].Number != 2 {
		t.Fatalf("no. = %v, want 2 and 3", s.Subsections)
	}
	if s.Subsections[0].Page != 4 || s.Subsections[1].Page != 5 {
		t.Errorf("no. printed pages = %d and %d, want the page map's",
			s.Subsections[0].Page, s.Subsections[1].Page)
	}
}

// A chapter whose page map records no missing leaf keeps its § 1 at its own
// first page. There the heading was on the page and the reading dropped it,
// which is the ordinary fault, and the page is the § 1 page.
func TestBodyOpensTheSectionAtTheChapterWhereNoLeafIsMissing(t *testing.T) {
	res := FromBody(acPages(), bodyMap("X", 1, 4, 1), Options{Book: "ac-x-fr"})
	s := res.Chapters[0].Sections[0]
	if s.Number != 1 || s.PDFPage != 1 || s.Page != 1 {
		t.Errorf("§ = %d at printed %d pdf %d, want § 1 at the chapter's own"+
			" first page", s.Number, s.Page, s.PDFPage)
	}
}

// And it says out loud that it is short, softly, because the reading is right
// and the file is what is incomplete.
func TestBodyReportsTheMissingNumberOneSoftly(t *testing.T) {
	res := FromBody(acPages(), acMap(), Options{Book: "ac-x-fr"})
	if len(res.Problems) != 2 {
		t.Fatalf("problems = %v, want the missing leaf and the missing no.",
			res.Problems)
	}
	for _, p := range res.Problems {
		if !p.Soft {
			t.Errorf("a missing heading was reported hard: %q", p.Detail)
		}
	}
	if !strings.Contains(res.Problems[0].Detail, "leaf the scan does not have") {
		t.Errorf("detail = %q, want it to name the leaf", res.Problems[0].Detail)
	}
	if !strings.Contains(res.Problems[1].Detail, "opens at no. 2") {
		t.Errorf("detail = %q, want it to name the no. the § opens at",
			res.Problems[1].Detail)
	}
}

// liePages is the shape of chapter VII of Lie: § headings the press set and the
// reading kept, one no. heading that came back in bold, and two appendices of
// which the reading kept only the first one's heading.
func liePages() []BodyPage {
	return []BodyPage{
		{PDFPage: 1, RunningHead: "CHAPITRE VII", Body: "" +
			"SOUS-ALGÈBRES DE CARTAN\n" +
			"ÉLÉMENTS RÉGULIERS\n" +
			"\n" +
			"Dans ce chapitre, k désigne un corps commutatif.\n" +
			"\n" +
			"§ 1. Décomposition primaire des représentations linéaires\n" +
			"\n" +
			"1. Décomposition d'une famille d'endomorphismes\n" +
			"\n" +
			"Soient V un espace vectoriel, S un ensemble.\n"},
		{PDFPage: 2, RunningHead: "n° 2 DÉCOMPOSITION PRIMAIRE", Body: "" +
			"2. Décomposition primaire d'une représentation\n" +
			"\n" +
			"Reprenons les notations du numéro précédent (§ 1, n° 1).\n"},
		{PDFPage: 3, RunningHead: "n° 3 THÉORÈMES DE CONJUGAISON 29", Body: "" +
			"**3. Applications de la conjugaison**\n" +
			"\n" +
			"Conservons les notations du lemme 2.\n"},
		{PDFPage: 4, RunningHead: "SOUS-ALGÈBRES DE CARTAN. ÉLÉMENTS RÉGULIERS",
			Body: "On a donc, d’après la prop. 3 de l’App. I, une partie ouverte" +
				" et dense pour la topologie de Zariski.\n"},
		{PDFPage: 5, RunningHead: "APPENDICE I", Body: "" +
			"Applications polynomiales et topologie de Zariski\n" +
			"\n" +
			"Dans cet appendice, k est supposé infini.\n" +
			"\n" +
			"1. Topologie de Zariski\n" +
			"\n" +
			"Soit V un espace vectoriel de dimension finie.\n"},
		{PDFPage: 6, RunningHead: "SOUS-ALGÈBRES DE CARTAN. ÉLÉMENTS RÉGULIERS",
			Body: "Les fonctions polynomiales sur V forment une algèbre graduée" +
				" dont la composante de degré 1 est le dual de V.\n"},
		{PDFPage: 7,
			RunningHead: "App. II APPLICATIONS POLYNOMIALES ET TOPOLOGIE DE ZARISKI",
			Body: "Soient $f : V \\to W$ une application polynomiale, et " +
				"$x_0 \\in V$. L’application $h \\mapsto f(x_0 + h)$ de $V$ dans" +
				" $W$ est polynomiale.\n"},
	}
}

// The § heading on the page is taken when there is one, and a no. heading the
// reading set in bold is still a no. heading.
func TestBodyReadsTheHeadingsThePagesCarry(t *testing.T) {
	res := FromBody(liePages(), bodyMap("VII", 1, 7, 7),
		Options{Book: "lie-vii-viii-fr"})
	if len(res.Chapters) != 1 {
		t.Fatalf("chapters = %d, want 1", len(res.Chapters))
	}
	ch := res.Chapters[0]
	if ch.Title != "SOUS-ALGÈBRES DE CARTAN ÉLÉMENTS RÉGULIERS" {
		t.Errorf("chapter title = %q, want both lines of it", ch.Title)
	}
	if len(ch.Sections) != 2 {
		t.Fatalf("sections = %d, want § 1 and the appendix", len(ch.Sections))
	}
	s := ch.Sections[0]
	if s.Title != "Décomposition primaire des représentations linéaires" {
		t.Errorf("§ 1 title = %q, want the heading on page 1", s.Title)
	}
	if len(s.Subsections) != 3 {
		t.Fatalf("no. = %v, want 1, 2 and the bold 3", s.Subsections)
	}
	if s.Subsections[2].Title != "Applications de la conjugaison" {
		t.Errorf("no. 3 title = %q, want it with the bold taken off",
			s.Subsections[2].Title)
	}
}

// The appendices of a chapter are numbered from 1 over again, the way the
// English printing of this chapter numbers its two, and an appendix carries the
// title its own first page prints and not the running head. Its no. are read
// like any other, because it sets the first of them under its title on the page
// that opens it.
func TestBodyOpensAnAppendix(t *testing.T) {
	res := FromBody(liePages(), bodyMap("VII", 1, 7, 7),
		Options{Book: "lie-vii-viii-fr"})
	s := res.Chapters[0].Sections[1]
	if !s.Appendix {
		t.Fatalf("section 2 = %+v, want the appendix", s)
	}
	if s.Number != 1 {
		t.Errorf("appendix number = %d, want the appendices numbered from 1",
			s.Number)
	}
	if s.Title != "Applications polynomiales et topologie de Zariski" {
		t.Errorf("appendix title = %q, want the title on page 5", s.Title)
	}
	if s.PDFPage != 5 || s.Page != 11 {
		t.Errorf("appendix at printed %d pdf %d, want printed 11 pdf 5",
			s.Page, s.PDFPage)
	}
	if len(s.Subsections) != 1 || s.Subsections[0].Title != "Topologie de Zariski" {
		t.Errorf("appendix no. = %v, want the one set under its title",
			s.Subsections)
	}
}

// The second appendix is on the paper, because the running heads name it, and it
// is not in the reading, because the page that opens it came back with no
// heading. That is said rather than guessed at.
func TestBodyReportsAnAppendixTheHeadsNameAndThePagesDoNotOpen(t *testing.T) {
	res := FromBody(liePages(), bodyMap("VII", 1, 7, 7),
		Options{Book: "lie-vii-viii-fr"})
	if len(res.Problems) != 1 {
		t.Fatalf("problems = %v, want the one missing appendix", res.Problems)
	}
	p := res.Problems[0]
	if !p.Soft {
		t.Errorf("the missing appendix was reported hard: %q", p.Detail)
	}
	if !strings.Contains(p.Detail, "name 2 appendices, I and II") {
		t.Errorf("detail = %q, want it to name both appendices", p.Detail)
	}
}

// A head written "App. 1" on one page and "App. II" on the next is one appendix
// and not two, because a scan of this age reads a roman one as an arabic one
// about as often as not.
func TestBodyDoesNotReportTheSameAppendixTwice(t *testing.T) {
	pages := liePages()[:6]
	pages = append(pages, BodyPage{PDFPage: 7,
		RunningHead: "Ch. VII, App. 1",
		Body:        "Les applications polynomiales de V dans W forment un espace vectoriel.\n"})
	res := FromBody(pages, bodyMap("VII", 1, 7, 7),
		Options{Book: "lie-vii-viii-fr"})
	if len(res.Problems) != 0 {
		t.Errorf("problems = %v, want none: App. 1 and App. I are the same one",
			res.Problems)
	}
}

// Once the run of exercises the press gathers at the end of a chapter has begun,
// nothing on the pages after it is a heading. The numbered exercises on them read
// as no. headings otherwise, and they arrive in the right order to be taken for
// some.
func TestBodyStopsReadingHeadingsAtTheExercises(t *testing.T) {
	pages := []BodyPage{
		{PDFPage: 1, RunningHead: "CHAPITRE X", Body: "" +
			"Profondeur, régularité, dualité\n" +
			"\n" +
			"§ 1. Profondeur\n" +
			"\n" +
			"1. Profondeur d'un module\n" +
			"\n" +
			"Soit $A$ un anneau noethérien.\n"},
		{PDFPage: 2, RunningHead: "EXERCICES", Section: 1, Body: "" +
			"Exercices\n" +
			"\n" +
			"1) Soit $A$ un anneau local noethérien.\n"},
		// The reading kept the head on nine of the thirty one exercise pages of
		// chapter X and lost it on the other twenty two. This is one of those.
		{PDFPage: 3, RunningHead: "", Body: "" +
			"2. Soient $A$ un anneau, $J$ un idéal de $A$\n" +
			"\n" +
			"Montrer que la profondeur de $A/J$ est finie.\n"},
	}
	res := FromBody(pages, bodyMap("X", 1, 3, 1), Options{Book: "ac-x-fr"})
	s := res.Chapters[0].Sections[0]
	if len(s.Subsections) != 1 {
		t.Errorf("no. = %v, want only the one before the exercises", s.Subsections)
	}
	if s.Exercises == nil {
		t.Fatal("§ 1 has no exercises, want the run that opens on pdf 2")
	}
	if s.Exercises.PDFPage != 2 || s.Exercises.Page != 2 {
		t.Errorf("exercises at printed %d pdf %d, want printed 2 pdf 2",
			s.Exercises.Page, s.Exercises.PDFPage)
	}
}

// A run of exercises begins at the mark the printing separates it with and not
// at the first page of it the reading kept a head on. Page 49 of chapter VII of
// Lie opens the block and marks § 1, page 51 is the third page of the same run
// and is the first with a head, and a contents that opened the run at 51 cut off
// the eight exercises before it.
func TestBodyOpensARunAtItsMark(t *testing.T) {
	pages := []BodyPage{
		{PDFPage: 1, RunningHead: "CHAPITRE VII", Body: "" +
			"Sous-algèbres de Cartan\n" +
			"\n" +
			"§ 1. Décomposition primaire\n" +
			"\n" +
			"1. Décomposition d'une famille d'endomorphismes\n" +
			"\n" +
			"Soient V un espace vectoriel, S un ensemble.\n"},
		{PDFPage: 2, RunningHead: "", Body: "" +
			"Exercices\n" +
			"\n" +
			"Les algèbres de Lie sont supposées de dimension finie sur k.\n" +
			"\n" +
			"§ 1\n" +
			"\n" +
			"1) On suppose k de caractéristique $p > 0$.\n"},
		{PDFPage: 3, RunningHead: "EXERCICES", Section: 1, Body: "" +
			"d) On suppose k algébriquement clos.\n"},
	}
	res := FromBody(pages, bodyMap("VII", 1, 3, 7), Options{Book: "lie-vii-viii-fr"})
	x := res.Chapters[0].Sections[0].Exercises
	if x == nil {
		t.Fatal("§ 1 has no exercises, want the run that opens on pdf 2")
	}
	if x.PDFPage != 2 {
		t.Errorf("exercises on pdf %d, want the page carrying the mark",
			x.PDFPage)
	}
}

// Two runs can begin on one page, where the press ends one and marks the next
// under it, and both are filed.
func TestBodyOpensTwoRunsOnOnePage(t *testing.T) {
	pages := []BodyPage{
		{PDFPage: 1, RunningHead: "CHAPITRE X", Body: "" +
			"Profondeur, régularité, dualité\n" +
			"\n" +
			"§ 1. Profondeur\n" +
			"\n" +
			"1. Profondeur d'un module\n"},
		{PDFPage: 2, RunningHead: "", Body: "" +
			"§ 2. Modules macaulayens\n" +
			"\n" +
			"1. Modules macaulayens\n"},
		{PDFPage: 3, RunningHead: "EXERCICES", Body: "" +
			"Exercices\n" +
			"\n" +
			"§ 1\n" +
			"\n" +
			"1) Soit $A$ un anneau local noethérien.\n" +
			"\n" +
			"§ 2\n" +
			"\n" +
			"1) Soit $A$ un anneau de Macaulay.\n"},
	}
	res := FromBody(pages, bodyMap("X", 1, 3, 1), Options{Book: "ac-x-fr"})
	for _, s := range res.Chapters[0].Sections {
		if s.Exercises == nil || s.Exercises.PDFPage != 3 {
			t.Errorf("§ %d exercises = %+v, want pdf 3", s.Number, s.Exercises)
		}
	}
}

// A § heading whose number is not the one that comes next is a citation, and
// these volumes cite each other on nearly every page.
func TestBodyRefusesASectionOutOfOrder(t *testing.T) {
	pages := []BodyPage{
		{PDFPage: 1, RunningHead: "CHAPITRE VII", Body: "" +
			"Sous-algèbres de Cartan\n" +
			"\n" +
			"§ 1. Décomposition primaire\n" +
			"\n" +
			"1. Décomposition d'une famille d'endomorphismes\n" +
			"\n" +
			"Soient V un espace vectoriel, S un ensemble.\n"},
		{PDFPage: 2, RunningHead: "n° 2 DÉCOMPOSITION PRIMAIRE", Body: "" +
			"§ 3. Théorèmes de conjugaison\n" +
			"\n" +
			"2. Décomposition primaire d'une représentation\n"},
	}
	res := FromBody(pages, bodyMap("VII", 1, 2, 7), Options{Book: "lie-vii-viii-fr"})
	ch := res.Chapters[0]
	if len(ch.Sections) != 1 {
		t.Fatalf("sections = %d, want only § 1: § 3 does not follow § 1",
			len(ch.Sections))
	}
	if len(ch.Sections[0].Subsections) != 2 {
		t.Errorf("no. = %v, want 1 and 2", ch.Sections[0].Subsections)
	}
}

// A contents line that leaked into the body carries the leaders the press sets
// between the title and the page, and it is not a heading.
func TestBodyRefusesAContentsLine(t *testing.T) {
	pages := []BodyPage{
		{PDFPage: 1, RunningHead: "CHAPITRE VII", Body: "" +
			"Sous-algèbres de Cartan\n" +
			"\n" +
			"§ 1. Décomposition primaire des représentations linéaires ...... 7\n" +
			"\n" +
			"2. Décomposition primaire d'une représentation ...... 12\n"},
	}
	res := FromBody(pages, bodyMap("VII", 1, 1, 7), Options{Book: "lie-vii-viii-fr"})
	if len(res.Chapters) != 0 {
		t.Errorf("chapters = %+v, want none: those lines are a contents",
			res.Chapters)
	}
}

// A § whose rectos all came back without a head is left with an empty title
// rather than borrowed the next §'s. An empty title is visible and a wrong one
// is not.
func TestBodyLeavesAnUnnamedSectionUnnamed(t *testing.T) {
	pages := []BodyPage{
		{PDFPage: 1, RunningHead: "CHAPITRE X", Body: "" +
			"Profondeur, régularité, dualité\n" +
			"\n" +
			"Dans ce chapitre, tous les anneaux sont supposés commutatifs.\n"},
		{PDFPage: 2, RunningHead: "", Body: "" +
			"1. Profondeur d'un module\n" +
			"\n" +
			"Soit $A$ un anneau noethérien.\n"},
		{PDFPage: 3, RunningHead: "PROFONDEUR., RÉGULARITÉ, DUALITÉ", Body: "" +
			"§ 2. Modules et anneaux macaulayens\n" +
			"\n" +
			"1. Modules macaulayens\n"},
		{PDFPage: 4, RunningHead: "N° 2 MODULES ET ANNEAUX MACAULAYENS", Body: "" +
			"2. Anneaux macaulayens\n"},
	}
	res := FromBody(pages, bodyMap("X", 1, 4, 1), Options{Book: "ac-x-fr"})
	ch := res.Chapters[0]
	if len(ch.Sections) != 2 {
		t.Fatalf("sections = %d, want § 1 and § 2", len(ch.Sections))
	}
	if ch.Sections[0].Title != "" {
		t.Errorf("§ 1 title = %q, want it empty rather than § 2's",
			ch.Sections[0].Title)
	}
	if ch.Sections[1].Title != "Modules et anneaux macaulayens" {
		t.Errorf("§ 2 title = %q, want the heading on page 3",
			ch.Sections[1].Title)
	}
}

// fix opening puts the chapter line back on the page it was lost from, so the
// same volume read a second time opens "## CHAPITRE VII" where it opened with
// the title before. Both readings have to name the chapter the same thing.
func TestBodyReadsARepairedPageTheSameWay(t *testing.T) {
	pages := liePages()
	pages[0].Body = "## CHAPITRE VII\n" +
		"\n" +
		"# SOUS-ALGÈBRES DE CARTAN ÉLÉMENTS RÉGULIERS\n" +
		"\n" +
		"Dans ce chapitre, k désigne un corps commutatif.\n" +
		"\n" +
		"## § 1. Décomposition primaire des représentations linéaires\n" +
		"\n" +
		"### 1. Décomposition d'une famille d'endomorphismes\n" +
		"\n" +
		"Soient V un espace vectoriel, S un ensemble.\n"
	// And the appendix, whose word the repair moves off the running head and
	// down into the body as a heading of its own.
	pages[4] = BodyPage{PDFPage: 5, RunningHead: "", Body: "" +
		"## APPENDICE 1\n" +
		"\n" +
		"# Applications polynomiales et topologie de Zariski\n" +
		"\n" +
		"Dans cet appendice, k est supposé infini.\n" +
		"\n" +
		"### 1. Topologie de Zariski\n" +
		"\n" +
		"Soit V un espace vectoriel de dimension finie.\n"}
	res := FromBody(pages, bodyMap("VII", 1, 7, 7),
		Options{Book: "lie-vii-viii-fr"})
	ch := res.Chapters[0]
	if ch.Title != "SOUS-ALGÈBRES DE CARTAN ÉLÉMENTS RÉGULIERS" {
		t.Errorf("chapter title = %q, want the title and not the chapter line",
			ch.Title)
	}
	if len(ch.Sections) != 2 || len(ch.Sections[0].Subsections) != 3 {
		t.Fatalf("sections = %+v, want the same reading as the unrepaired page",
			ch.Sections)
	}
	app := ch.Sections[1]
	if !app.Appendix || app.Number != 1 ||
		app.Title != "Applications polynomiales et topologie de Zariski" {
		t.Errorf("appendix = %+v, want the same one the unrepaired page gave", app)
	}
	// The repair took the running head that named appendix I off the page, so
	// the heads alone now name only II. The appendix the pages did open has to
	// be counted in or the volume stops saying it is short one.
	if len(res.Problems) != 1 ||
		!strings.Contains(res.Problems[0].Detail, "name 2 appendices") {
		t.Errorf("problems = %v, want the missing appendix still reported",
			res.Problems)
	}
}

func TestPlainHeading(t *testing.T) {
	tests := []struct{ raw, want string }{
		{"**3. Applications de la conjugaison**", "3. Applications de la conjugaison"},
		{"*2. Enveloppe scindable*", "2. Enveloppe scindable"},
		{"## § 4. Eléments réguliers", "§ 4. Eléments réguliers"},
		{"  1. Topologie de Zariski  ", "1. Topologie de Zariski"},
		{"**", "**"},
		{"", ""},
	}
	for _, tt := range tests {
		if got := plainHeading(tt.raw); got != tt.want {
			t.Errorf("plainHeading(%q) = %q, want %q", tt.raw, got, tt.want)
		}
	}
}

func TestHeadTitle(t *testing.T) {
	tests := []struct{ head, want string }{
		{"N° 1 PROFONDEUR", "PROFONDEUR"},
		{"n° 3 THÉORÈMES DE CONJUGAISON 29", "THÉORÈMES DE CONJUGAISON 29"},
		{"N° 10 COHOMOLOGIE LOCALE, DUALITÉ DE GROTHENDIECK",
			"COHOMOLOGIE LOCALE, DUALITÉ DE GROTHENDIECK"},
		{"PROFONDEUR., RÉGULARITÉ, DUALITÉ", "PROFONDEUR., RÉGULARITÉ, DUALITÉ"},
	}
	for _, tt := range tests {
		if got := headTitle(tt.head); got != tt.want {
			t.Errorf("headTitle(%q) = %q, want %q", tt.head, got, tt.want)
		}
	}
}
