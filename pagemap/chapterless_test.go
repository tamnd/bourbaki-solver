package pagemap

import "testing"

// Three volumes in the library print no chapters at all: the two printings of
// the Elements of the History of Mathematics, which are a flat run of numbered
// notes, and the French Varietes differentielles et analytiques, which is a
// fascicule de resultats running § 1 to § 7 with nothing over them. Their page
// maps have an empty chapter column on every row, so asking chapterSpans for
// the spans of no chapters gave back no spans, and toc then had nowhere to hang
// the § it read and reported that the contents yielded no chapters.
func TestAVolumeWithNoChaptersGetsOneSpanOverItsBody(t *testing.T) {
	entries := []Entry{
		{PDFPage: 1, Confidence: Unknown},
		{PDFPage: 2, Confidence: Unknown},
		{PDFPage: 3, Page: 3, Confidence: FromHead},
		{PDFPage: 4, Page: 4, Confidence: FromHead},
		{PDFPage: 5, Page: 5, Confidence: FromHead},
	}
	if spans := chapterSpans(entries, nil); len(spans) != 0 {
		t.Fatalf("chapterSpans over no chapters = %v, want none; the bug this "+
			"covers is that there was nothing else to ask", spans)
	}
	spans := spansFor(entries, nil)
	if len(spans) != 1 {
		t.Fatalf("spans = %v, want exactly one over the body", spans)
	}
	sp := spans[0]
	if sp.Chapter != WholeVolume {
		t.Errorf("the span is named %q, want %q", sp.Chapter, WholeVolume)
	}
	if sp.FirstPDF != 3 || sp.LastPDF != 5 {
		t.Errorf("the span runs pdf %d to %d, want 3 to 5", sp.FirstPDF, sp.LastPDF)
	}
	if sp.FirstPage != 3 || sp.LastPage != 5 {
		t.Errorf("the span runs printed %d to %d, want 3 to 5", sp.FirstPage, sp.LastPage)
	}
}

// The rows of the body are named and not just the span over them. PDFPageOf
// refuses a row whose chapter does not match the one it was asked for, so a
// span standing over rows that name nothing reports every page of the volume as
// being on no pdf page: 27 problems on the Elements of the History of
// Mathematics and 107 on the fascicule, all of them saying the same thing.
func TestTheBodyRowsOfAChapterlessVolumeAreNamedToo(t *testing.T) {
	entries := []Entry{
		{PDFPage: 1, Confidence: Unknown},
		{PDFPage: 2, Page: 3, Confidence: FromHead},
		{PDFPage: 3, Page: 4, Confidence: FromHead},
	}
	m := &Map{Book: "var-fr", Pagination: Continuous, PDFPages: 3, Entries: entries}
	m.Chapters = spansFor(m.Entries, nil)
	if got, ok := m.PDFPageOf(WholeVolume, 4); !ok || got != 3 {
		t.Errorf("PDFPageOf(%q, 4) = %d, %v; want 3, true", WholeVolume, got, ok)
	}
	if m.Entries[0].Chapter != "" {
		t.Errorf("a front matter row was named %q, want it left alone", m.Entries[0].Chapter)
	}
}

// A volume whose pages have not been read yet has rows but no printed numbers
// on any of them, and there is no body to stand a span over. Handing back a
// span with no pages in it would be worse than handing back none, because
// everything downstream would then believe the volume had been mapped.
func TestAMapWithNoNumberedRowsGetsNoSpan(t *testing.T) {
	entries := []Entry{
		{PDFPage: 1, Confidence: Unknown},
		{PDFPage: 2, Confidence: Unknown},
	}
	if spans := spansFor(entries, nil); len(spans) != 0 {
		t.Errorf("spans = %v, want none", spans)
	}
}

// A volume bound from two fascicules restarts its numbering part way through,
// so its printed pages run 97, 98 and then 6, 7. That is two spans and not one,
// and calling it one would claim a span covering 97 down to 7, which validate
// then reports as -88 printed pages over 4 pdf pages. It would also make a
// printed page ambiguous, since each fascicule prints its own page 50 and
// PDFPageOf would answer with whichever row it reached first.
//
// The French Varietes is the case: it prints no chapters and so reaches this
// code, but pdf 96 is where paragraphes 8 a 15 begin and the count goes back to
// 6. Each fascicule is a separate publication with its own front matter and its
// own table of contents, so a span each is the volume read as what it is. The
// two printings of the History have no restart, run forward end to end and get
// the one span, which is why the first run keeps the name WholeVolume.
func TestABodyThatRestartsGetsASpanPerFascicule(t *testing.T) {
	entries := []Entry{
		{PDFPage: 1, Page: 97, Confidence: FromHead},
		{PDFPage: 2, Page: 98, Confidence: FromHead},
		{PDFPage: 3, Page: 6, Confidence: FromHead},
		{PDFPage: 4, Page: 7, Confidence: FromHead},
	}
	spans := spansFor(entries, nil)
	if len(spans) != 2 {
		t.Fatalf("spans = %v, want one per fascicule", spans)
	}
	// In the order they are bound, which is the order they are appended in and
	// which chapterOrder leaves alone, RomanOrder having nothing to say about an
	// arabic numeral.
	if spans[0].Chapter != WholeVolume || spans[1].Chapter != "2" {
		t.Fatalf("spans named %q and %q, want %q and 2",
			spans[0].Chapter, spans[1].Chapter, WholeVolume)
	}
	if spans[0].FirstPDF != 1 || spans[0].LastPDF != 2 ||
		spans[0].FirstPage != 97 || spans[0].LastPage != 98 {
		t.Errorf("fascicule 1 = %+v, want pdf 1 to 2 over printed 97 to 98", spans[0])
	}
	if spans[1].FirstPDF != 3 || spans[1].LastPDF != 4 ||
		spans[1].FirstPage != 6 || spans[1].LastPage != 7 {
		t.Errorf("fascicule 2 = %+v, want pdf 3 to 4 over printed 6 to 7", spans[1])
	}
	// Neither span runs backwards, which is the whole point of cutting at the
	// restart and is what validate used to report as a negative page count.
	for _, sp := range spans {
		if sp.LastPage < sp.FirstPage {
			t.Errorf("span %q runs printed %d to %d", sp.Chapter, sp.FirstPage, sp.LastPage)
		}
	}
	// Each fascicule prints its own page 6, and the map has to answer with the
	// one that was asked for rather than whichever row came first.
	m := &Map{Book: "var-fr", Pagination: Continuous, PDFPages: 4,
		Entries: entries, Chapters: spans}
	if got, ok := m.PDFPageOf("2", 6); !ok || got != 3 {
		t.Errorf("PDFPageOf(2, 6) = %d, %v; want 3, true", got, ok)
	}
	if _, ok := m.PDFPageOf(WholeVolume, 6); ok {
		t.Error("printed page 6 was found in fascicule 1, which starts at 97")
	}
}

// Every other volume in the library declares its chapters, and this must not
// change what any of them gets. The naming only happens when the list is empty.
func TestAVolumeThatDeclaresChaptersIsUnaffected(t *testing.T) {
	entries := []Entry{
		{PDFPage: 1, Confidence: Unknown},
		{PDFPage: 2, Chapter: "IV", Page: 1, Confidence: FromHead},
		{PDFPage: 3, Chapter: "IV", Page: 2, Confidence: FromHead},
	}
	spans := spansFor(entries, []string{"IV"})
	if len(spans) != 1 || spans[0].Chapter != "IV" {
		t.Fatalf("spans = %v, want one named IV", spans)
	}
	if entries[1].Chapter != "IV" {
		t.Errorf("a declared chapter was renamed to %q", entries[1].Chapter)
	}
}
