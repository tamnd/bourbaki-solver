package clip

import (
	"os"
	"path/filepath"
	"regexp"
	"testing"
	"time"

	"github.com/tamnd/bourbaki-solver/extract"
	"github.com/tamnd/bourbaki-solver/ocr"
	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// The numbers in this file are measured rather than invented. The page is 420
// of Théories spectrales, the units are what pdftohtml -xml reports for it, and
// the pixel figures are what pdftoppm was given when the crop that proved this
// route works was cut by hand.

// TestABoxBecomesThePixelsPdftoppmWasGiven is the whole geometry in one
// assertion. pdftohtml lays the page out at a zoom of 1.5, so a unit is
// dpi/72/1.5 pixels, which at 600 dpi is 5.555. The line whose box is below was
// cut by hand at -x 422 -y 2750 and came back legible, and that is the number
// this has to reproduce.
func TestABoxBecomesThePixelsPdftoppmWasGiven(t *testing.T) {
	box := Box{Left: 80, Top: 499, Right: 583, Bottom: 514}
	got := box.Pixels(600, Zoom, Pad)
	if got.Min.X != 422 || got.Min.Y != 2750 {
		t.Errorf("Pixels() top left = (%d,%d), want the (422,2750) the hand cut used", got.Min.X, got.Min.Y)
	}
	if got.Dx() < 2800 || got.Dx() > 2900 {
		t.Errorf("Pixels() width = %d, want about the 2833 the hand cut used", got.Dx())
	}
}

// TestTheEdgesOfAClipRoundOutwards records why the far edges have a 1 added to
// them. Truncating both ends of a box shaves a fraction of a pixel off the
// right and the bottom, and the bottom of a line of Bourbaki is where the
// indices are. A clip one pixel too wide costs nothing at all.
func TestTheEdgesOfAClipRoundOutwards(t *testing.T) {
	got := Box{Left: 100, Top: 100, Right: 101, Bottom: 101}.Pixels(600, Zoom, 0)
	if got.Dx() < 6 || got.Dy() < 6 {
		t.Errorf("Pixels() = %v, want a box at least as wide as the unit it came from", got)
	}
}

// TestAClipAtTheTopOfThePageIsNotCutAboveIt. A running head sits a few units
// below the top of the paper and the padding around it has nowhere to come
// from, so the box would start at a negative pixel and pdftoppm would refuse it.
func TestAClipAtTheTopOfThePageIsNotCutAboveIt(t *testing.T) {
	got := Box{Left: 2, Top: 1, Right: 400, Bottom: 20}.Pixels(600, Zoom, Pad)
	if got.Min.X != 0 || got.Min.Y != 0 {
		t.Errorf("Pixels() top left = (%d,%d), want it held at the corner of the page", got.Min.X, got.Min.Y)
	}
}

// TestTheZoomIsMeasuredRatherThanAssumed. Everything a clip does rests on the
// frame pdftohtml reports its boxes in being the frame pdftoppm crops in.
// Théories spectrales is 439.371 pt wide and poppler lays it out at 659 pixels,
// which is 1.4997 and not 1.5, and that is close enough to cut with and far
// enough to be worth knowing came from the page.
func TestTheZoomIsMeasuredRatherThanAssumed(t *testing.T) {
	got := ZoomOf(pdfsrc.Page{Width: 659, Height: 999}, 439.371)
	if got < 1.499 || got > 1.5 {
		t.Errorf("ZoomOf() = %v, want the 1.4997 the volume is really laid out at", got)
	}
	// A page poppler reported no size for falls back on the tool's constant,
	// which is what every command used before any of this was measured.
	if got := ZoomOf(pdfsrc.Page{}, 439.371); got != Zoom {
		t.Errorf("ZoomOf() on a page with no size = %v, want %v", got, Zoom)
	}
	if got := ZoomOf(pdfsrc.Page{Width: 659}, 0); got != Zoom {
		t.Errorf("ZoomOf() with no page size in points = %v, want %v", got, Zoom)
	}
}

// TestABoxIsTakenOverEveryRunAndNotTheBandOfTheType is the reason BoxOf exists.
// A line's own Top and Bottom are the band of the body type, because that is
// what says whether a run is an exponent, and a clip cut to that band is a clip
// of a formula with its exponents and its indices sliced off. The runs here are
// the shape of $u(\mathring{V})$: body type, then a mark above it, then an
// index below.
func TestABoxIsTakenOverEveryRunAndNotTheBandOfTheType(t *testing.T) {
	line := extract.Line{
		Top: 500, Bottom: 515, Left: 80, Right: 400,
		Runs: []extract.Run{
			{Span: pdfsrc.Span{Left: 80, Top: 500, Width: 320, Height: 15}},
			{Span: pdfsrc.Span{Left: 200, Top: 493, Width: 6, Height: 7}},   // the ring
			{Span: pdfsrc.Span{Left: 402, Top: 511, Width: 10, Height: 10}}, // the index
		},
	}
	box := BoxOf(line)
	if box.Top != 493 {
		t.Errorf("BoxOf().Top = %d, want the top of the accent at 493", box.Top)
	}
	if box.Bottom != 521 {
		t.Errorf("BoxOf().Bottom = %d, want the foot of the index at 521", box.Bottom)
	}
	if box.Right != 412 {
		t.Errorf("BoxOf().Right = %d, want the index counted at 412", box.Right)
	}
	if box.Left != 80 {
		t.Errorf("BoxOf().Left = %d, want 80", box.Left)
	}
}

// TestThePageIsCutToItsInkAndNotToItsPaper. Théories spectrales sets a text
// block of about 500 by 700 units on a page of 659 by 999, so a third of the
// paper is margin. Cutting it away is the one thing a page clip does that a
// render of the page does not, and it puts half again as many pixels on every
// letter for the same number of bytes going up the browser control.
func TestThePageIsCutToItsInkAndNotToItsPaper(t *testing.T) {
	page := pdfsrc.Page{Number: 112, Width: 659, Height: 999, Spans: []pdfsrc.Span{
		{Left: 81, Top: 60, Width: 60, Height: 10, Text: "TS III.98"},   // the running head
		{Left: 80, Top: 120, Width: 500, Height: 15, Text: "En rempla"}, // the first line
		{Left: 81, Top: 880, Width: 498, Height: 15, Text: "et contient"},
	}}
	box, ok := BlockOf(page)
	if !ok {
		t.Fatal("BlockOf() found no ink on a page that has some")
	}
	if box.Top != 60 || box.Bottom != 895 || box.Left != 80 || box.Right != 580 {
		t.Errorf("BlockOf() = %+v, want the ink and nothing outside it", box)
	}
	// The running head is inside the box and stays there. It is ink on the page,
	// and a box drawn to exclude it would have to know where it is, which is the
	// extractor's job and not a crop's.
	if box.Top != 60 {
		t.Errorf("BlockOf().Top = %d, want the running head kept at 60", box.Top)
	}
}

// TestAPageWithNoInkIsNotCut. A blank page is a real thing in these volumes,
// they fall between chapters, and cutting one produces a picture of nothing for
// a model to spend four minutes on.
func TestAPageWithNoInkIsNotCut(t *testing.T) {
	for _, page := range []pdfsrc.Page{
		{Number: 84, Width: 659, Height: 999},
		{Number: 84, Width: 659, Height: 999, Spans: []pdfsrc.Span{{Left: 80, Top: 100, Text: "  "}}},
	} {
		if _, ok := BlockOf(page); ok {
			t.Errorf("BlockOf() cut page %d, which has nothing on it", page.Number)
		}
	}
}

// TestAWholePageTargetSaysSoWithoutTheIndexBesideIt. The read has to pick a
// prompt and the audit has to pick a comparison, and both of them work from a
// target rather than from a flag somebody remembered to pass twice.
func TestAWholePageTargetSaysSoWithoutTheIndexBesideIt(t *testing.T) {
	if !(Target{Page: 112, Line: WholePage}).Whole() {
		t.Error("Whole() = false on a page target")
	}
	// Line zero is the first line of a page and not a page.
	if (Target{Page: 112, Line: 0}).Whole() {
		t.Error("Whole() = true on the first line of a page")
	}
	if got := PageName(112); got != "0112.png" {
		t.Errorf("PageName() = %q, want %q", got, "0112.png")
	}
}

// TestAQueryKeepsOnlyTheLinesItWasAskedFor. The match is against the line as
// the extractor renders it, which is the only form of the line that exists
// before the clip is cut.
func TestAQueryKeepsOnlyTheLinesItWasAskedFor(t *testing.T) {
	query := Query{Match: regexp.MustCompile(`[˚˘˙]`)}
	const hit = `contenu dans ˚$V\cup (E$ V). Comme ˚V et E V sont des parties ouvertes`
	const miss = `existe un voisinage équilibré W de 0 dans F tel que $u(V)\cap W\subset u(V)$.`
	if !query.Keep(hit, 0) {
		t.Error("Keep() refused the line carrying the loose ring")
	}
	if query.Keep(miss, 0) {
		t.Error("Keep() took a line with no accent on it")
	}
}

// TestSamplingSpreadsTheClipsOverTheVolume. A fault that occurs six hundred
// times is not audited by the first thirty, which are all in chapter one.
func TestSamplingSpreadsTheClipsOverTheVolume(t *testing.T) {
	query := Query{Every: 10}
	var kept int
	for seen := range 100 {
		if query.Keep("anything", seen) {
			kept++
		}
	}
	if kept != 10 {
		t.Errorf("Keep() took %d of 100 lines at one in ten, want 10", kept)
	}
	// Zero and one both mean take everything, and a caller that passes neither
	// should not silently get nothing.
	for _, every := range []int{0, 1} {
		if !(Query{Every: every}).Keep("anything", 7) {
			t.Errorf("Keep() with Every=%d dropped a line, want all of them", every)
		}
	}
}

// TestAPageListOutranksTheRange is how the pages a report named are revisited:
// the caller has a list and no interest in what lies between its ends.
func TestAPageListOutranksTheRange(t *testing.T) {
	query := Query{First: 1, Last: 500, Pages: map[int]bool{85: true, 111: true}}
	if !query.InRange(85) || !query.InRange(111) {
		t.Error("InRange() refused a page that was asked for by name")
	}
	if query.InRange(86) {
		t.Error("InRange() took a page that is in the range and not on the list")
	}
	// And a page on the list that falls outside the range is still out. The
	// range is a bound and the list is a choice within it.
	if (Query{First: 100, Pages: map[int]bool{85: true}}).InRange(85) {
		t.Error("InRange() took a named page from before the first page asked for")
	}
}

// TestAClipIsNamedAfterThePageAndTheLine so a directory of them sorts in
// reading order and a name says what it is without opening the index.
func TestAClipIsNamedAfterThePageAndTheLine(t *testing.T) {
	if got := Name(85, 17); got != "0085-017.png" {
		t.Errorf("Name() = %q, want %q", got, "0085-017.png")
	}
}

// TestAnIndexSurvivesBeingWrittenAndReadBack. The index is the whole memory of
// a cut: the read and the audit never open the PDF, and the reading it pins is
// the half of every comparison that is ours.
func TestAnIndexSurvivesBeingWrittenAndReadBack(t *testing.T) {
	path := filepath.Join(t.TempDir(), "clips.json")
	want := Index{
		Book: "ts-iii-v-fr", PDF: "work/prepared/ts-iii-v-fr-86558f9cf3ff-4257b2.pdf",
		DPI: 600, Zoom: 1.499704155477675, Pad: 4, Match: `[˚˘˙]`,
		Generated: time.Date(2026, 8, 13, 10, 51, 35, 0, time.UTC),
		Targets: []Target{{
			Page: 22, Line: 16, Name: "0022-016.png",
			Native: `de $E_{\sigma}$, il existe d’après le théorème de ˘Smulian (EVT, IV, p. 36, th. 2)`,
			Box:    Box{Left: 81, Top: 428, Right: 580, Bottom: 445},
		}},
	}
	if err := WriteIndex(path, want); err != nil {
		t.Fatalf("WriteIndex() = %v", err)
	}
	got, err := ReadIndex(path)
	if err != nil {
		t.Fatalf("ReadIndex() = %v", err)
	}
	if len(got.Targets) != 1 || got.Targets[0].Native != want.Targets[0].Native {
		t.Errorf("ReadIndex() targets = %v, want the reading pinned at cut time", got.Targets)
	}
	if got.Targets[0].Box != want.Targets[0].Box || got.Zoom != want.Zoom {
		t.Errorf("ReadIndex() = %+v, want the geometry back unchanged", got)
	}
}

// TestPendingIsWhatIsMissingAndNotWhatWasAsked. A run that was interrupted, or
// a host that dropped half a batch, should cost only what is missing, and the
// fleet time this saves is measured in hours.
func TestPendingIsWhatIsMissingAndNotWhatWasAsked(t *testing.T) {
	dir, dest := t.TempDir(), t.TempDir()
	index := Index{Targets: []Target{
		{Page: 85, Line: 14, Name: "0085-014.png"},
		{Page: 85, Line: 15, Name: "0085-015.png"},
	}}
	for _, target := range index.Targets {
		if err := os.WriteFile(filepath.Join(dir, target.Name), []byte("png"), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(dest, "0085-014.md"), []byte("a line"), 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Pending(index, dir, dest)
	if err != nil {
		t.Fatalf("Pending() = %v", err)
	}
	if len(got) != 1 || filepath.Base(got[0]) != "0085-015.png" {
		t.Errorf("Pending() = %v, want only the clip with no answer", got)
	}
}

// TestAnEmptyAnswerIsStillPending. A file of nothing is what a batch leaves
// behind when it died mid write, and treating it as read is how a clip is
// silently never asked about.
func TestAnEmptyAnswerIsStillPending(t *testing.T) {
	dir, dest := t.TempDir(), t.TempDir()
	index := Index{Targets: []Target{{Page: 85, Line: 14, Name: "0085-014.png"}}}
	if err := os.WriteFile(filepath.Join(dir, "0085-014.png"), []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dest, "0085-014.md"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	got, err := Pending(index, dir, dest)
	if err != nil {
		t.Fatalf("Pending() = %v", err)
	}
	if len(got) != 1 {
		t.Errorf("Pending() = %v, want the clip whose answer is an empty file", got)
	}
}

// TestTwoRunsOfOneVolumeDoNotShareARemoteDirectory. The first version of this
// named a batch <book>-clip-000 and the second run of Théories spectrales
// walked straight into the first run's answers: they were still in that
// directory on the box, the poll counted twelve of them, decided a batch of
// four had finished before it had started, and pulled back twelve readings of
// lines nobody had asked about while all seven pages came home missing.
func TestTwoRunsOfOneVolumeDoNotShareARemoteDirectory(t *testing.T) {
	lines := []string{"work/clips/ts-iii-v-fr/0022-016.png", "work/clips/ts-iii-v-fr/0085-014.png"}
	pages := []string{"work/clips/ts-iii-v-fr/0022.png", "work/clips/ts-iii-v-fr/0085.png"}
	first, second := batchID("ts-iii-v-fr-clip", 0, lines), batchID("ts-iii-v-fr-clip", 0, pages)
	if first == second {
		t.Errorf("batchID() = %q for two different batches of one volume", first)
	}
	// The same batch asked for twice is the same name, which is what makes a
	// resumed run land on the answers it already has rather than beside them.
	if again := batchID("ts-iii-v-fr-clip", 0, lines); again != first {
		t.Errorf("batchID() = %q then %q for one batch", first, again)
	}
	// And it is still a name a host will take as a directory.
	for _, id := range []string{first, second} {
		if err := ocr.ValidBatchID(id); err != nil {
			t.Errorf("ValidBatchID(%q) = %v", id, err)
		}
	}
}

// TestAClipThatWasNeverCutIsAnErrorAndNotASkip. An answer for a picture that
// does not exist is not a thing a run can wait for, and a read that quietly
// skipped it would report a batch of twelve as a batch of eleven and say
// nothing about the twelfth.
func TestAClipThatWasNeverCutIsAnErrorAndNotASkip(t *testing.T) {
	index := Index{Targets: []Target{{Page: 85, Line: 14, Name: "0085-014.png"}}}
	if _, err := Pending(index, t.TempDir(), t.TempDir()); err == nil {
		t.Error("Pending() = nil error, want a complaint about the clip that is not there")
	}
}
