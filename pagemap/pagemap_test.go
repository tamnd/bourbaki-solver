package pagemap

import (
	"strings"
	"testing"
)

// Every running head below was copied out of pdftotext -layout run over the
// volumes named. The mangled ones are not invented: the 2003 scan really does
// print "A.IV.3 8" for A.IV.38, and reading those correctly is worth 24 pages
// of the map.
func TestReadHeadLabelOnRealRunningHeads(t *testing.T) {
	tests := []struct {
		line    string
		prefix  string
		chapter string
		page    int
		ok      bool
	}{
		// Chapter 8, 2023, Springer Nature. Clean text layer, recto and verso.
		{"A VIII.466        TRACE OF AN ENDOMORPHISM OF FINITE RANK", "A", "VIII", 466, true},
		{"EXERCISES                                   A VIII.467", "A", "VIII", 467, true},
		{"A VIII.470                                HISTORICAL NOTE", "A", "VIII", 470, true},

		// Chapters 4 to 7, 2003, Springer. A scan, so the text layer is
		// somebody else's OCR and it splits the label in several ways.
		{"A.IV.82                    POLYNOMIALS AND RATIONAL FRACTIONS", "A", "IV", 82, true},
		{"A.IV.3 8                 POLYNOMIALS AND RATIONAL FRACTIONS       §4", "A", "IV", 38, true},
		{"A.IV. 5 6              POLYNOMIALS AND RATIONAL FRACTIONS         §5", "A", "IV", 56, true},
		{"No. 5     SYMMETRIC TENSORS AND POLYNOMIAL MAPPINGS       A.I V. 4 7", "A", "IV", 47, true},
		{"A .I V. 74              POLYNOMIALS AND RATIONAL FRACTIONS        §6", "A", "IV", 74, true},
		{"No. 2                  p-RADICAL EXTENSIONS OF HEIGHT = I     A. V.101", "A", "V", 101, true},
		{"No. 6                D                CRITERIA OF SEPARABIL.ITY   A. V .13 5", "A", "V", 135, true},
		{"A. V. 192.                        COMMUTATIVE FIELDS          § 16", "A", "V", 192, true},

		// Pages that carry no label. Bourbaki prints no running head on a
		// chapter opener, a section opener or a blank verso, which is where
		// every interpolated entry in the map comes from.
		{"CHAPTER VIII", "", "", 0, false},
		{"§ 2.   THE STRUCTURE OF MODULES OF FINITE", "", "", 0, false},
		{"HISTORICAL NOTE", "", "", 0, false},
		{"", "", "", 0, false},

		// The 1998 volume prints the chapter numeral alone on the verso. It
		// must not read as a label, because that volume's number is at the foot.
		{"I                              ALGEBRAIC STRUCTURES", "", "", 0, false},
		{"III                            TENSOR ALGEBRAS", "", "", 0, false},
	}
	for _, tt := range tests {
		pre, ch, p, ok := readHeadLabel(tt.line)
		if ok != tt.ok || pre != tt.prefix || ch != tt.chapter || p != tt.page {
			t.Errorf("readHeadLabel(%q) = %q, %q, %d, %v; want %q, %q, %d, %v",
				tt.line, pre, ch, p, ok, tt.prefix, tt.chapter, tt.page, tt.ok)
		}
	}
}

// The volumes that are not Algebra print a Book prefix of two or three letters,
// and until this session the reader took one letter, so every one of these read
// as nothing at all. Each line below was copied out of pdftotext -layout run
// over the volume named beside it.
func TestReadHeadLabelOutsideAlgebra(t *testing.T) {
	tests := []struct {
		line    string
		prefix  string
		chapter string
		page    int
		ok      bool
	}{
		// Theories spectrales chapters 1 and 2, French, born digital.
		{"TS I.142         ENDOMORPHISMES DES ESPACES DE BANACH                   § 7", "TS", "I", 142, true},
		{"No 1                        SOUS-ALGEBRES                           TS I.143", "TS", "I", 143, true},
		// Theories spectrales chapters 3 to 5, same printing, chapter IV.
		{"TS IV.248                   OPERATEURS PARTIELS                       § 4", "TS", "IV", 248, true},
		{"No 7                SPECTRE ET RESOLVANTE                      TS IV.247", "TS", "IV", 247, true},
		// Topologie algebrique chapters 1 to 4, French, born digital. This one
		// also prints a bare number at the foot, so it is the one volume where
		// the two readers both find something and the label has to win.
		{"TA II.214                       COEGALISATEUR                          § 5", "TA", "II", 214, true},
		{"                             EXERCICES                                 TA II.217", "TA", "II", 217, true},
		// Integration, three letters and a chapter numeral of two.
		{"INT IV.43              MESURES SUR LES ESPACES TOPOLOGIQUES", "INT", "IV", 43, true},
		// Fonctions d'une variable reelle, French. The scan reads the Roman
		// numeral as ones, so a numeral that is all digits is ordinary here.
		{"FVR 111.10                    FONCTIONS ELEMENTAIRES", "FVR", "III", 10, true},
	}
	for _, tt := range tests {
		pre, ch, p, ok := readHeadLabel(tt.line)
		if ok != tt.ok || pre != tt.prefix || ch != tt.chapter || p != tt.page {
			t.Errorf("readHeadLabel(%q) = %q, %q, %d, %v; want %q, %q, %d, %v",
				tt.line, pre, ch, p, ok, tt.prefix, tt.chapter, tt.page, tt.ok)
		}
	}
}

// The English Lie groups and Lie algebras chapters 7 to 9 prints its number at
// the outer edge of the running head and its chapter at the inner edge, which
// is neither of the two grammars the package started with. Every line below is
// a real head from that volume, except the last two, which are heads from
// volumes printed the other two ways and must not be read as this one.
func TestReadHeadNumber(t *testing.T) {
	tests := []struct {
		line    string
		chapter string
		page    int
		alt     int
		ok      bool
	}{
		{"§13.             CLASSICAL SPLITTABLE SIMPLE LIE ALGEBRAS                    189", "", 189, 0, true},
		{"190                        SPLIT SEMI-SIMPLE LIE ALGEBRAS               Ch. VIII", "VIII", 190, 0, true},
		{"192                           SPLIT SEMI-SIMPLE LIE ALGEBRAS                         Ch. VIII", "VIII", 192, 0, true},
		// A chapter opener and a section opener carry no number, as in every
		// other volume.
		{"CHAPTER VIII", "", 0, 0, false},
		{"§ 2.   THE STRUCTURE OF MODULES OF FINITE", "", 0, 0, false},
		// The French printings write the locator every way but the one this
		// used to recognise, and the scan splits the numeral as often as not.
		{"10                MESURES SUR LES ESPACES TOPOLOGIQUES SEPARES       Ch. IX, § 1", "IX", 10, 0, true},
		{"74                    ALGEBRE COMMUTATIVE               chap. I I , § 2", "II", 74, 0, true},
		{"32     INTEGRATION DES MESURES            Chap. V , $ 1", "V", 32, 0, true},
		// A verso names its chapter at the outer edge and prints its page at
		// the inner one, so the number after the § is never the page. The scan
		// of page 10 gives "1O", which is not a number at all, and reading the
		// § instead is how three separate pages came to be page 1.
		{"1O                MESURES SUR LES ESPACES TOPOLOGIQUES SEPARES       Ch. IX, § 1", "", 0, 0, false},
		// The verso of Lie chapitres 4, 5 et 6 puts the § one space clear of the
		// title where the page number stands twelve clear of it, and the scanner
		// read the mark as a digit here too. The chapter is what says the trailing
		// 6 is a section and not a page.
		{"122                 GROUPES ENGENDRÉS PAR DES RÉFLEXIONS                   Ch. v,     6", "V", 122, 0, true},
		// A recto of the same volume whose § 3 the scan ran together as 83. Both
		// edges are then equally well formed and nothing on the line says which
		// is the page, so both come back and the fit decides. It is 129.
		{" 83                                                   EXERCICES                                   129", "", 83, 129, true},
		{"94                                           EXERCICES                                              133", "", 94, 133, true},
		// The exercise heads of the French printings set the section at the inner
		// edge and the page at the outer one, and the outer one is the one the
		// scan breaks. Page 109 of Integration chapitre 9 shipped as page 85 for
		// as long as the number had to be an unbroken run of digits.
		{" 85                                         EXERCICES                                            1 09", "", 85, 109, true},
		{"54                               EXERCICES                                24 1", "", 54, 241, true},
		// Four digits is as far as any volume in the library goes, and the spaces
		// the scan inserts are not a way round that.
		{"12 345                          EXERCICES", "", 0, 0, false},
		// Prose is not a running head. Every one of these was read as a page.
		{"Reimpression inchangee de l'edition originale de 1959", "", 0, 0, false},
		{"(no 7). Lorsque @ est bilineaire, Q, est l'image reciproque de 5", "", 0, 0, false},
		{"5 2. Relevement des ideaux premiers.", "", 0, 0, false},
		// The history volume heads its index in mixed case, so what says this
		// is a head is the gap and not the capitals.
		{"298     Index", "", 298, 0, true},
		{"                          Index     301", "", 301, 0, true},
	}
	for _, tt := range tests {
		ch, p, alt, ok := readHeadNumber(tt.line)
		if ok != tt.ok || ch != tt.chapter || p != tt.page || alt != tt.alt {
			t.Errorf("readHeadNumber(%q) = %q, %d, %d, %v; want %q, %d, %d, %v",
				tt.line, ch, p, alt, ok, tt.chapter, tt.page, tt.alt, tt.ok)
		}
	}
}

// A head that cites another Book must not become an anchor. The prefix is
// measured off the volume, so the minority prefix loses however plausible it
// looks on the line.
func TestAnchorsKeepTheDominantPrefixOnly(t *testing.T) {
	pages := []string{
		"TS I.142         ENDOMORPHISMES DES ESPACES DE BANACH\nbody\n",
		"No 1             SOUS-ALGEBRES                    TS I.143\nbody\n",
		"TS I.144         FONCTIONS CONTINUES\nbody\n",
		"cf. TG I.4 et la prop. 3                          A I.99\nbody\n",
	}
	as, prefix := readAnchorsPrefix(pages, HeadLabel, []string{"I"})
	if prefix != "TS" {
		t.Fatalf("prefix = %q, want TS", prefix)
	}
	if len(as) != 3 {
		t.Fatalf("got %d anchors, want 3: %v", len(as), as)
	}
}

func TestDominantBreaksTiesWithoutMapOrder(t *testing.T) {
	if got := dominant(map[string]int{"A": 2, "TS": 2}); got != "A" {
		t.Errorf("dominant = %q, want A", got)
	}
	if got := dominant(nil); got != "" {
		t.Errorf("dominant = %q, want empty", got)
	}
}

func TestSplitPages(t *testing.T) {
	// pdftotext ends the last page with a form feed too, so a plain split
	// leaves an empty tail that would be counted as a page.
	if got := SplitPages("one\f two\f"); len(got) != 2 {
		t.Errorf("got %d pages, want 2: %q", len(got), got)
	}
	if got := SplitPages(""); len(got) != 0 {
		t.Errorf("got %d pages, want 0", len(got))
	}
}

// Page 350 of the 1998 scan ends with the tail of a display formula, a bare
// "1", and only then the page number 326. Reading up from the bottom by more
// than one line reads the formula as the page number.
func TestFootLineStopsAtTheLastLine(t *testing.T) {
	page := "the point g =    L _!_ a is called (by an abuse of\n" +
		"                 t=l m\n" +
		"                           1\n" +
		"326\n"
	if got := strings.TrimSpace(footLine(page)); got != "326" {
		t.Errorf("footLine = %q, want \"326\"", got)
	}
}

func TestFitOffsetsBelievesAStepAndRejectsAMisread(t *testing.T) {
	// Offset 17 through the body, then the file drops a blank leaf and the
	// offset becomes 16 for good. The single anchor at pdf 20 is a misread of
	// the sort the 2003 scan produces and must not open a segment.
	as := []anchor{
		{pdfPage: 18, page: 1}, {pdfPage: 19, page: 2},
		{pdfPage: 20, page: 30}, // misread
		{pdfPage: 21, page: 4}, {pdfPage: 22, page: 5},
		{pdfPage: 23, page: 7}, {pdfPage: 24, page: 8}, {pdfPage: 25, page: 9},
	}
	segs, outliers := fitOffsets(as, DefaultMinRun)
	if len(segs) != 2 {
		t.Fatalf("got %d segments, want 2: %+v", len(segs), segs)
	}
	if segs[0].offset != 17 || segs[1].offset != 16 {
		t.Errorf("offsets are %d and %d, want 17 and 16", segs[0].offset, segs[1].offset)
	}
	if len(outliers) != 1 || as[outliers[0]].pdfPage != 20 {
		t.Errorf("outliers = %v, want just the anchor at pdf 20", outliers)
	}
}

func TestFitOffsetsAcceptsAShortRunAtTheEnd(t *testing.T) {
	// The last stretch of chapter 8 is the terminology index, and there are
	// fewer anchors left in it than the run length. All of them agreeing is
	// enough.
	as := []anchor{
		{pdfPage: 1, page: 1}, {pdfPage: 2, page: 2}, {pdfPage: 3, page: 3},
		{pdfPage: 5, page: 4}, {pdfPage: 6, page: 5},
	}
	segs, outliers := fitOffsets(as, DefaultMinRun)
	if len(segs) != 2 || len(outliers) != 0 {
		t.Fatalf("got %d segments and %d outliers, want 2 and 0", len(segs), len(outliers))
	}
	if segs[1].offset != 1 {
		t.Errorf("second offset = %d, want 1", segs[1].offset)
	}
}

// head builds a page whose first line is the running head.
func head(line string) string { return line + "\n\nsome body text\n" }

// foot builds a page with a chapter numeral in the head and the number at the
// foot, which is how the 1998 volume is laid out.
func foot(headLine, footLine string) string {
	return headLine + "\n\nsome body text\n\n" + footLine + "\n"
}

// perChapterVolume is a miniature of the 2003 and 2023 layout: front matter
// with no numbers, a chapter opener with no head, a section opener with no
// head, one OCR misread, and a dropped blank leaf.
func perChapterVolume() []string {
	return []string{
		head("Contents"),                  // 1
		head("Contents"),                  // 2
		head("CHAPTER IV"),                // 3, printed IV.1
		head("A.IV.2   POLYNOMIALS"),      // 4
		head("A.IV.3   POLYNOMIALS"),      // 5
		head("A.IV.4   POLYNOMIALS"),      // 6
		head("§ 3.  SYMMETRIC FUNCTIONS"), // 7, printed IV.5, no head
		head("A.IV.6   POLYNOMIALS"),      // 8
		head("A.IV.7 0   POLYNOMIALS"),    // 9, misread of A.IV.7
		head("A.IV.8   POLYNOMIALS"),      // 10
		head("A.IV.10   EXERCISES"),       // 11, IV.9 is a dropped leaf
		head("A.IV.11   EXERCISES"),       // 12
		head("A.IV.12   HISTORICAL NOTE"), // 13
	}
}

func TestBuildPerChapter(t *testing.T) {
	m, err := Build(perChapterVolume(), Options{Book: "mini", Chapters: []string{"IV"}})
	if err != nil {
		t.Fatal(err)
	}
	if m.Grammar != HeadLabel || m.Pagination != PerChapter {
		t.Fatalf("detected %s %s", m.Grammar, m.Pagination)
	}
	want := []struct {
		pdf  int
		page int
		conf Confidence
	}{
		{1, 0, Unknown},
		{2, 0, Unknown},
		{3, 1, Interpolated}, // the chapter opener prints no head
		{4, 2, FromHead},
		{7, 5, Interpolated}, // the section opener prints no head
		{9, 7, Interpolated}, // the misread was overruled
		{11, 10, FromHead},   // after the dropped leaf
		{13, 12, FromHead},
	}
	for _, w := range want {
		e, ok := m.Lookup(w.pdf)
		if !ok {
			t.Fatalf("pdf %d is missing", w.pdf)
		}
		if e.Page != w.page || e.Confidence != w.conf {
			t.Errorf("pdf %d = page %d %s, want page %d %s",
				w.pdf, e.Page, e.Confidence, w.page, w.conf)
		}
	}
	if len(m.Chapters) != 1 || m.Chapters[0].FirstPDF != 3 || m.Chapters[0].LastPDF != 13 {
		t.Errorf("chapter span = %+v, want pdf 3 to 13", m.Chapters)
	}
	if len(m.Steps) != 1 || len(m.Steps[0].MissingPages) != 1 || m.Steps[0].MissingPages[0] != 9 {
		t.Errorf("steps = %+v, want printed page 9 missing", m.Steps)
	}
	if len(m.Conflicts) != 1 || m.Conflicts[0].PDFPage != 9 || m.Conflicts[0].Read != 70 {
		t.Errorf("conflicts = %+v, want the misread at pdf 9", m.Conflicts)
	}
	if probs := m.Validate(); len(probs) != 0 {
		t.Errorf("validate found %d problems: %v", len(probs), probs)
	}
}

func TestBuildContinuous(t *testing.T) {
	// The 1998 layout: the number is at the foot and runs across chapters, and
	// only the opener says where a chapter begins.
	pages := []string{
		head("Contents"),                        // 1
		head("Contents"),                        // 2
		foot("CHAPTER I", "1"),                  // 3
		foot("I     ALGEBRAIC STRUCTURES", "2"), // 4
		foot("§1.5", ""),                        // 5, no foot number
		foot("I     ALGEBRAIC STRUCTURES", "4"), // 6
		foot("§1.8", "5"),                       // 7
		foot("CHAPTER II", "6"),                 // 8
		foot("II    LINEAR ALGEBRA", "7"),       // 9
		foot("§2.1", "8"),                       // 10
	}
	m, err := Build(pages, Options{Book: "mini", Chapters: []string{"I", "II"}})
	if err != nil {
		t.Fatal(err)
	}
	if m.Grammar != FootNumber || m.Pagination != Continuous {
		t.Fatalf("detected %s %s", m.Grammar, m.Pagination)
	}
	if len(m.Chapters) != 2 {
		t.Fatalf("got %d chapters, want 2: %+v", len(m.Chapters), m.Chapters)
	}
	if c := m.Chapters[0]; c.Chapter != "I" || c.FirstPDF != 3 || c.LastPDF != 7 || c.FirstPage != 1 || c.LastPage != 5 {
		t.Errorf("chapter I = %+v, want pdf 3-7 printed 1-5", c)
	}
	if c := m.Chapters[1]; c.Chapter != "II" || c.FirstPDF != 8 || c.FirstPage != 6 {
		t.Errorf("chapter II = %+v, want pdf 8 printed 6", c)
	}
	// The page with no foot number sits inside the fitted stretch, so it is
	// filled in rather than left blank.
	e, _ := m.Lookup(5)
	if e.Page != 3 || e.Confidence != Interpolated || e.Chapter != "I" {
		t.Errorf("pdf 5 = %+v, want printed 3 in chapter I, interpolated", e)
	}
	if e, _ := m.Lookup(1); e.Confidence != Unknown {
		t.Errorf("front matter should not be mapped, got %+v", e)
	}
	if probs := m.Validate(); len(probs) != 0 {
		t.Errorf("validate found %d problems: %v", len(probs), probs)
	}
}

// A French volume of the head-number grammar, with the running head naming its
// chapter the way Integration and Lie do. The two pages between the last head
// of chapter I and the opener of chapter II are the end of chapter I: its last
// exercises and the blank leaf after them, neither of which carries a head.
func frenchHeadVolume(firstOpener, secondOpener string) []string {
	return []string{
		head("TABLE DES MATIÈRES"),                                     // 1
		head(firstOpener),                                              // 2, printed 1
		head("2      MESURES SUR LES ESPACES SÉPARÉS     Ch. I, § 1"),  // 3
		head("3      MESURES SUR LES ESPACES SÉPARÉS     Ch. I, § 1"),  // 4
		head("la fin des exercices"),                                   // 5, printed 4
		head("une page laissée blanche"),                               // 6, printed 5
		head(secondOpener),                                             // 7, printed 6
		head("7      INTÉGRATION DES MESURES             Ch. II, § 1"), // 8
		head("8      INTÉGRATION DES MESURES             Ch. II, § 1"), // 9
	}
}

// An opener says where a chapter begins and a head only bounds it from above,
// so the openers are used whenever there is one for every chapter. Reading the
// heads instead moved chapter VIII of Lie 7 to 9 back two pages, onto the last
// exercises of chapter VII.
func TestOpenersBeatHeadsWhenThereIsOneForEveryChapter(t *testing.T) {
	pages := frenchHeadVolume("CHAPITRE I", "CHAPITRE II")
	m, err := Build(pages, Options{Book: "mini", Chapters: []string{"I", "II"}})
	if err != nil {
		t.Fatal(err)
	}
	if m.Grammar != HeadNumber {
		t.Fatalf("detected %s", m.Grammar)
	}
	if len(m.Chapters) != 2 {
		t.Fatalf("got %d chapters, want 2: %+v", len(m.Chapters), m.Chapters)
	}
	if c := m.Chapters[1]; c.Chapter != "II" || c.FirstPDF != 7 || c.FirstPage != 6 {
		t.Errorf("chapter II = %+v, want pdf 7 printed 6", c)
	}
	if probs := m.Validate(); len(probs) != 0 {
		t.Errorf("validate found %d problems: %v", len(probs), probs)
	}
}

// Where the scan mangled the openers the heads are all there is, and they are
// worth having: a French head names its chapter as plainly as "INT IX.10" does.
// Without this, Lie chapitres 7 et 8 came out as one unnamed run.
func TestHeadsNameTheChaptersWhenNoOpenerWasRead(t *testing.T) {
	pages := frenchHeadVolume("le début du premier chapitre", "le début du deuxième")
	m, err := Build(pages, Options{Book: "mini", Chapters: []string{"I", "II"}})
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Chapters) != 2 {
		t.Fatalf("got %d chapters, want 2: %+v", len(m.Chapters), m.Chapters)
	}
	// The head bounds the chapter from above and nothing else does, so chapter
	// II reaches back over the pages that carry no head at all.
	if c := m.Chapters[1]; c.Chapter != "II" || c.FirstPDF != 5 {
		t.Errorf("chapter II = %+v, want pdf 5", c)
	}
	if probs := m.Validate(); len(probs) != 0 {
		t.Errorf("validate found %d problems: %v", len(probs), probs)
	}
}

// A recto of Lie chapitres 4, 5 et 6 sets the § one space clear of the title
// and the page number twelve clear of it, and the scan of "§ 3" came out as
// "83". Both edges then read as page numbers and nothing on the line says
// which, so the fit picks the one that keeps the offset and the head counts as
// read rather than as a page in conflict with itself.
func TestTheOtherNumberOnTheLineSettlesAMangledHead(t *testing.T) {
	pages := []string{
		head("CHAPTER I"),
		head("2                        EXERCICES"),
		head("3                        EXERCICES"),
		head("83                   EXERCICES                    4"),
		head("5                        EXERCICES"),
	}
	m, err := Build(pages, Options{Book: "mini", Chapters: []string{"I"}})
	if err != nil {
		t.Fatal(err)
	}
	e, ok := m.Lookup(4)
	if !ok || e.Page != 4 || e.Confidence != FromHead {
		t.Errorf("pdf 4 = %+v, want printed 4 read off the head", e)
	}
	if len(m.Conflicts) != 0 {
		t.Errorf("conflicts = %+v, want none", m.Conflicts)
	}
	if probs := m.Validate(); len(probs) != 0 {
		t.Errorf("validate found %d problems: %v", len(probs), probs)
	}
}

// The table of contents lists every chapter opener on one line with the title
// and a page number after it. Taking those as openers put chapter III inside
// the front matter.
func TestChapterOpenerIgnoresTheTableOfContents(t *testing.T) {
	pages := []string{
		"CONTENTS\n\nCHAPTER I. ALGEBRAIC STRUCTURES ................ 1\n" +
			"CHAPTER II. LINEAR ALGEBRA ..................... 191\n",
		head("CHAPTER I"),
	}
	starts := readChapterStarts(pages, []string{"I", "II"})
	if len(starts) != 1 || starts[2] != "I" {
		t.Errorf("chapter starts = %v, want only pdf 2 opening chapter I", starts)
	}
}

func TestDetectPrefersTheGrammarThatReadsMore(t *testing.T) {
	if g := Detect(perChapterVolume(), []string{"IV"}); g != HeadLabel {
		t.Errorf("Detect = %s, want %s", g, HeadLabel)
	}
	footOnly := []string{foot("I  ALGEBRAIC STRUCTURES", "1"), foot("§1.2", "2"), foot("§1.3", "3")}
	if g := Detect(footOnly, []string{"I"}); g != FootNumber {
		t.Errorf("Detect = %s, want %s", g, FootNumber)
	}
}

func TestBuildRejectsAnEmptyDocument(t *testing.T) {
	if _, err := Build(nil, Options{Book: "mini"}); err == nil {
		t.Error("an empty document should be an error")
	}
}

func TestValidateCatchesAnUnrecordedJump(t *testing.T) {
	m := &Map{Book: "mini", Pagination: PerChapter, PDFPages: 3, Entries: []Entry{
		{PDFPage: 1, Chapter: "IV", Page: 1, Confidence: FromHead},
		{PDFPage: 2, Chapter: "IV", Page: 2, Confidence: FromHead},
		{PDFPage: 3, Chapter: "IV", Page: 9, Confidence: FromHead},
	}}
	m.Chapters = chapterSpans(m.Entries, []string{"IV"})
	probs := m.Validate()
	if len(probs) == 0 {
		t.Fatal("a jump from page 2 to page 9 with no step should be a problem")
	}
	found := false
	for _, p := range probs {
		if p.PDFPage == 3 && strings.Contains(p.Detail, "no step recorded") {
			found = true
		}
	}
	if !found {
		t.Errorf("problems = %v, want one about the jump at pdf 3", probs)
	}
}

func TestValidateCatchesAnUnconfirmedOverrule(t *testing.T) {
	// A reading overruled next to a page that was itself interpolated is not
	// safe: nothing independent confirms the number the fitter chose.
	m := &Map{Book: "mini", Pagination: PerChapter, PDFPages: 3, Entries: []Entry{
		{PDFPage: 1, Chapter: "IV", Page: 1, Confidence: FromHead},
		{PDFPage: 2, Chapter: "IV", Page: 2, Confidence: Interpolated},
		{PDFPage: 3, Chapter: "IV", Page: 3, Confidence: Interpolated},
	}}
	m.Chapters = chapterSpans(m.Entries, []string{"IV"})
	m.Conflicts = []Conflict{{PDFPage: 3, Read: 30, Chapter: "IV", Fitted: 3}}
	probs := m.Validate()
	if len(probs) != 1 || !strings.Contains(probs[0].Detail, "without both neighbours") {
		t.Errorf("problems = %v, want one about the unconfirmed overrule", probs)
	}
}

func TestValidateCatchesAMapThatMappedNothing(t *testing.T) {
	// Every page unknown is what a scan with no usable running head fits to.
	// None of the other checks has an entry to fail on, so without this one the
	// map is clean, gets written, and the volume counts as mapped from then on.
	m := &Map{Book: "mini", Pagination: PerChapter, PDFPages: 3, Entries: []Entry{
		{PDFPage: 1, Confidence: Unknown},
		{PDFPage: 2, Confidence: Unknown},
		{PDFPage: 3, Confidence: Unknown},
	}}
	probs := m.Validate()
	if len(probs) != 1 || !strings.Contains(probs[0].Detail, "the fit found nothing") {
		t.Errorf("problems = %v, want one saying nothing was mapped", probs)
	}
}

func TestValidateAcceptsAVolumeOnlyPartlyMapped(t *testing.T) {
	// A front matter of two pages and one body page is the shape of a volume
	// whose pages are still being read, and it is not the shape this catches.
	m := &Map{Book: "mini", Pagination: PerChapter, PDFPages: 3, Entries: []Entry{
		{PDFPage: 1, Confidence: Unknown},
		{PDFPage: 2, Confidence: Unknown},
		{PDFPage: 3, Chapter: "IV", Page: 1, Confidence: FromHead},
	}}
	m.Chapters = chapterSpans(m.Entries, []string{"IV"})
	if probs := m.Validate(); len(probs) != 0 {
		t.Errorf("problems = %v, want none", probs)
	}
}

func TestPDFPageOf(t *testing.T) {
	m, err := Build(perChapterVolume(), Options{Book: "mini", Chapters: []string{"IV"}})
	if err != nil {
		t.Fatal(err)
	}
	if got, ok := m.PDFPageOf("IV", 10); !ok || got != 11 {
		t.Errorf("PDFPageOf(IV, 10) = %d, %v; want 11, true", got, ok)
	}
	if _, ok := m.PDFPageOf("IV", 9); ok {
		t.Error("printed page 9 is a dropped leaf and has no pdf page")
	}
}

// The 2015 printings changed what a page label means. Theories spectrales
// prints TS I.1 to TS I.197 and then TS II.200, so the numeral says which
// chapter the page is in while the number counts the volume, where Algebra
// chapter 8 numbers the chapter. Assuming the older convention put all 197
// pages of chapter I into chapter II.
func TestPaginationIsDetectedNotAssumed(t *testing.T) {
	perChapter := []anchor{
		{chapter: "IV", page: 2}, {chapter: "IV", page: 80},
		{chapter: "V", page: 3}, {chapter: "V", page: 101},
		{chapter: "VI", page: 2}, {chapter: "VI", page: 44},
	}
	if got := detectPagination(perChapter); got != PerChapter {
		t.Errorf("detectPagination = %q, want %q", got, PerChapter)
	}
	continuous := []anchor{
		{chapter: "I", page: 2}, {chapter: "I", page: 197},
		{chapter: "II", page: 200}, {chapter: "II", page: 334},
	}
	if got := detectPagination(continuous); got != Continuous {
		t.Errorf("detectPagination = %q, want %q", got, Continuous)
	}
	// One chapter says nothing either way, and nothing downstream depends on
	// the answer, so it keeps the older convention.
	if got := detectPagination([]anchor{{chapter: "VIII", page: 4}}); got != PerChapter {
		t.Errorf("detectPagination = %q, want %q", got, PerChapter)
	}
}

// Where a volume prints the chapter on every page, that is where the chapter
// boundary is, and the boundary falls on the opener rather than on the first
// page that prints a number, because Bourbaki prints no running head on an
// opener. Theories spectrales chapters 1 and 2 is the case in hand: pdf 210
// prints TS I.197, pdf 211 is the page that says "chapitre ii" and prints
// nothing else, pdf 212 prints TS II.200.
func TestChapterStartsComeFromTheLabels(t *testing.T) {
	as := []anchor{
		{pdfPage: 14, chapter: "I", page: 1},
		{pdfPage: 210, chapter: "I", page: 197},
		{pdfPage: 212, chapter: "II", page: 200},
		{pdfPage: 346, chapter: "II", page: 334},
	}
	starts := chapterStartsFromAnchors(as)
	want := map[int]string{1: "I", 211: "II"}
	if len(starts) != len(want) {
		t.Fatalf("starts = %v, want %v", starts, want)
	}
	for at, ch := range want {
		if starts[at] != ch {
			t.Errorf("starts[%d] = %q, want %q", at, starts[at], ch)
		}
	}
}

// A page that fell between two fitted stretches goes to the chapter before it,
// because that is where the exercises and the historical note sit. A chapter
// opener is the exception and has to go the other way.
func TestOpenerGoesToTheChapterItOpens(t *testing.T) {
	covers := []cover{
		{from: 14, to: 210, offset: 13, chapter: "I"},
		{from: 212, to: 346, offset: 12, chapter: "II"},
	}
	got := closeCracks(openersGoRight(covers, []int{1, 211}))
	if got[0].to != 210 {
		t.Errorf("chapter I ends at pdf %d, want 210", got[0].to)
	}
	if got[1].from != 211 {
		t.Errorf("chapter II starts at pdf %d, want 211", got[1].from)
	}
	// Printed 199 rather than 198: the opener is a recto, and every opener in
	// the six volumes that map cleanly carries an odd printed number.
	if p := got[1].from - got[1].offset; p != 199 {
		t.Errorf("the opener is printed page %d, want 199", p)
	}
}
