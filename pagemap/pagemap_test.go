package pagemap

import (
	"strings"
	"testing"
)

// Every running head below was copied out of pdftotext -layout run over the
// three volumes. The mangled ones are not invented: the 2003 scan really does
// print "A.IV.3 8" for A.IV.38, and reading those correctly is worth 24 pages
// of the map.
func TestReadHeadLabelOnRealRunningHeads(t *testing.T) {
	tests := []struct {
		line    string
		chapter string
		page    int
		ok      bool
	}{
		// Chapter 8, 2023, Springer Nature. Clean text layer, recto and verso.
		{"A VIII.466        TRACE OF AN ENDOMORPHISM OF FINITE RANK", "VIII", 466, true},
		{"EXERCISES                                   A VIII.467", "VIII", 467, true},
		{"A VIII.470                                HISTORICAL NOTE", "VIII", 470, true},

		// Chapters 4 to 7, 2003, Springer. A scan, so the text layer is
		// somebody else's OCR and it splits the label in several ways.
		{"A.IV.82                    POLYNOMIALS AND RATIONAL FRACTIONS", "IV", 82, true},
		{"A.IV.3 8                 POLYNOMIALS AND RATIONAL FRACTIONS       §4", "IV", 38, true},
		{"A.IV. 5 6              POLYNOMIALS AND RATIONAL FRACTIONS         §5", "IV", 56, true},
		{"No. 5     SYMMETRIC TENSORS AND POLYNOMIAL MAPPINGS       A.I V. 4 7", "IV", 47, true},
		{"A .I V. 74              POLYNOMIALS AND RATIONAL FRACTIONS        §6", "IV", 74, true},
		{"No. 2                  p-RADICAL EXTENSIONS OF HEIGHT = I     A. V.101", "V", 101, true},
		{"No. 6                D                CRITERIA OF SEPARABIL.ITY   A. V .13 5", "V", 135, true},
		{"A. V. 192.                        COMMUTATIVE FIELDS          § 16", "V", 192, true},

		// Pages that carry no label. Bourbaki prints no running head on a
		// chapter opener, a section opener or a blank verso, which is where
		// every interpolated entry in the map comes from.
		{"CHAPTER VIII", "", 0, false},
		{"§ 2.   THE STRUCTURE OF MODULES OF FINITE", "", 0, false},
		{"HISTORICAL NOTE", "", 0, false},
		{"", "", 0, false},

		// The 1998 volume prints the chapter numeral alone on the verso. It
		// must not read as a label, because that volume's number is at the foot.
		{"I                              ALGEBRAIC STRUCTURES", "", 0, false},
		{"III                            TENSOR ALGEBRAS", "", 0, false},
	}
	for _, tt := range tests {
		ch, p, ok := readHeadLabel(tt.line)
		if ok != tt.ok || ch != tt.chapter || p != tt.page {
			t.Errorf("readHeadLabel(%q) = %q, %d, %v; want %q, %d, %v",
				tt.line, ch, p, ok, tt.chapter, tt.page, tt.ok)
		}
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
