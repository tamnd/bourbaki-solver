package render

import (
	"context"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// poppler stands in for pdfinfo and pdftoppm. It writes real PNG files, because
// the whole point of the measure step is that it reads the bytes back: a fake
// that returned canned numbers would test nothing.
type poppler struct {
	pages int
	// ink is the fraction of each page that is dark, keyed by page number.
	// Pages not listed come out clean white, which is what a blank scan page
	// looks like once the speckle is under the threshold.
	ink map[int]float64
	// pad is the zero padding pdftoppm uses on the page number. Poppler picks
	// it from the page count of the volume, so a 734 page book gives 3 and the
	// renamer has to cope.
	pad  int
	seen []string
}

func (p *poppler) Run(_ context.Context, name string, args ...string) ([]byte, error) {
	p.seen = append(p.seen, strings.Join(append([]string{name}, args...), " "))
	switch name {
	case "pdfinfo":
		return []byte(fmt.Sprintf("Title:  test\nPages:  %d\nPage size: 385.92 x 596.16 pts\nEncrypted: no\n", p.pages)), nil
	case "pdftoppm":
		return nil, p.render(args)
	}
	return nil, fmt.Errorf("unexpected command %q", name)
}

func (p *poppler) render(args []string) error {
	var first, last int
	var width int = 100
	prefix := args[len(args)-1]
	for i, arg := range args {
		switch arg {
		case "-f":
			first, _ = strconv.Atoi(args[i+1])
		case "-l":
			last, _ = strconv.Atoi(args[i+1])
		case "-r":
			dpi, _ := strconv.Atoi(args[i+1])
			// Keep the image small but let the test see that dpi reached the
			// pixels, which is what an escalated retry has to prove.
			width = dpi / 10
		}
	}
	for page := first; page <= last; page++ {
		img := image.NewGray(image.Rect(0, 0, width, 100))
		for i := range img.Pix {
			img.Pix[i] = 0xff
		}
		dark := int(p.ink[page] * float64(width*100))
		for i := 0; i < dark; i++ {
			// Offset by the page number so no two pages come out byte
			// identical. Two real scanned pages never do, and a fake that
			// produced the same bytes for every page would hide a hashing bug
			// rather than expose one.
			img.Set((i+page)%width, (i+page)/width, color.Gray{Y: 0x10})
		}
		path := fmt.Sprintf("%s-%0*d.png", prefix, p.pad, page)
		file, err := os.Create(path)
		if err != nil {
			return err
		}
		if err := png.Encode(file, img); err != nil {
			file.Close()
			return err
		}
		if err := file.Close(); err != nil {
			return err
		}
	}
	return nil
}

func options(t *testing.T, run *poppler) Options {
	t.Helper()
	root := t.TempDir()
	pdf := filepath.Join(root, "book.pdf")
	if err := os.WriteFile(pdf, []byte("%PDF-1.4\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	return Options{
		Book: "alg-i-iii", PDF: pdf, Corpus: root,
		DPI: DefaultDPI, Gray: true, Batch: 4, Run: run,
	}
}

func TestRenderWritesEveryPageAndAManifest(t *testing.T) {
	run := &poppler{pages: 10, pad: 2, ink: map[int]float64{
		1: 0.05, 2: 0.05, 3: 0.05, 4: 0.05, 5: 0.05,
		6: 0.05, 7: 0.05, 8: 0.05, 9: 0.05, 10: 0.05,
	}}
	opts := options(t, run)

	manifest, err := Render(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := len(manifest.Pages); got != 10 {
		t.Fatalf("manifest has %d pages, want 10", got)
	}
	for i, page := range manifest.Pages {
		if page.Page != i+1 {
			t.Fatalf("page %d of the manifest is pdf page %d, want them in order", i, page.Page)
		}
		// The four digit name is the contract the OCR stage and the page files
		// share. Poppler padded to two here, so this is the rename working.
		if page.File != fmt.Sprintf("%04d.png", i+1) {
			t.Fatalf("page %d is named %q, want %04d.png", page.Page, page.File, i+1)
		}
		if _, err := os.Stat(ImagePath(opts.Corpus, opts.Book, page.Page)); err != nil {
			t.Fatalf("page %d: %v", page.Page, err)
		}
		if page.SHA256 == "" || page.Bytes == 0 || page.Width == 0 || page.Height == 0 {
			t.Fatalf("page %d was not measured: %+v", page.Page, page)
		}
		if page.DPI != DefaultDPI {
			t.Fatalf("page %d says %d dpi, want %d", page.Page, page.DPI, DefaultDPI)
		}
	}

	read, err := ReadManifest(opts.Corpus, opts.Book)
	if err != nil {
		t.Fatal(err)
	}
	if len(read.Pages) != 10 || read.PDFSHA256 == "" {
		t.Fatalf("manifest did not survive the round trip: %+v", read)
	}
	// Every page hashes differently, otherwise the OCR job ids collide and one
	// page's reading is filed under another.
	seen := map[string]int{}
	for _, page := range read.Pages {
		if other, ok := seen[page.SHA256]; ok {
			t.Fatalf("pages %d and %d hash the same", other, page.Page)
		}
		seen[page.SHA256] = page.Page
	}
}

func TestBlankPagesAreFoundAndKeptOutOfTheQueue(t *testing.T) {
	// Front matter, a part title, then real text. The half title is empty, the
	// part title has four words on it and must not be called blank.
	run := &poppler{pages: 6, pad: 1, ink: map[int]float64{
		1: 0, 2: 0.0005, 3: 0.02, 4: 0.06, 5: 0.06, 6: 0,
	}}
	opts := options(t, run)
	opts.WriteBlanks = true

	manifest, err := Render(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if got := manifest.Blanks(); got != 3 {
		t.Fatalf("found %d blank pages, want 3: %+v", got, manifest.Pages)
	}
	for _, page := range []int{1, 2, 6} {
		value, _ := manifest.Find(page)
		if !value.Blank {
			t.Fatalf("page %d has ink %.4f and was not called blank", page, value.Ink)
		}
	}
	for _, page := range []int{3, 4, 5} {
		value, _ := manifest.Find(page)
		if value.Blank {
			t.Fatalf("page %d has ink %.4f and was called blank", page, value.Ink)
		}
	}

	// A blank page still gets a page file, because a gap in the sequence and a
	// page that was missed look identical on disk.
	for _, page := range []int{1, 2, 6} {
		raw, err := os.ReadFile(corpus.PagePath(opts.Corpus, opts.Book, page))
		if err != nil {
			t.Fatalf("page %d: %v", page, err)
		}
		if !strings.Contains(string(raw), "method: blank") {
			t.Fatalf("page %d does not say method: blank:\n%s", page, raw)
		}
	}
	if _, err := os.Stat(corpus.PagePath(opts.Corpus, opts.Book, 4)); !os.IsNotExist(err) {
		t.Fatalf("page 4 has text on it and should not have a page file yet: %v", err)
	}
}

// A second render must not touch the blank pages it already wrote. Nothing
// about a page with no ink on it can have changed, and a fresh generated stamp
// on five empty files is five files somebody has to read past in a pull
// request to find the pages that were actually read.
func TestRenderingAgainLeavesTheBlankPagesAlone(t *testing.T) {
	run := &poppler{pages: 4, pad: 1, ink: map[int]float64{1: 0, 2: 0.06, 3: 0.06, 4: 0}}
	opts := options(t, run)
	opts.WriteBlanks = true
	if _, err := Render(context.Background(), opts); err != nil {
		t.Fatal(err)
	}

	blank := corpus.PagePath(opts.Corpus, opts.Book, 1)
	first, err := os.ReadFile(blank)
	if err != nil {
		t.Fatal(err)
	}
	before, err := os.Stat(blank)
	if err != nil {
		t.Fatal(err)
	}

	opts.Overwrite = true
	if _, err := Render(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	second, err := os.ReadFile(blank)
	if err != nil {
		t.Fatal(err)
	}
	if string(first) != string(second) {
		t.Errorf("the blank page was rewritten:\n%s\nbecame\n%s", first, second)
	}
	after, err := os.Stat(blank)
	if err != nil {
		t.Fatal(err)
	}
	if !after.ModTime().Equal(before.ModTime()) {
		t.Errorf("the blank page was written again at %s, having been written at %s",
			after.ModTime(), before.ModTime())
	}
}

func TestInterruptedRunsResume(t *testing.T) {
	run := &poppler{pages: 8, pad: 1, ink: map[int]float64{
		1: 0.05, 2: 0.05, 3: 0.05, 4: 0.05, 5: 0.05, 6: 0.05, 7: 0.05, 8: 0.05,
	}}
	opts := options(t, run)
	if _, err := Render(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	before := len(run.seen)

	// Lose two pages, as a killed run would.
	for _, page := range []int{3, 7} {
		if err := os.Remove(ImagePath(opts.Corpus, opts.Book, page)); err != nil {
			t.Fatal(err)
		}
	}
	manifest, err := Render(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Pages) != 8 {
		t.Fatalf("second run produced %d pages, want 8", len(manifest.Pages))
	}
	// At a third of a second a page over 734 pages, re-rendering what is
	// already on disk is four minutes thrown away every time somebody presses
	// control C.
	var rendered int
	for _, command := range run.seen[before:] {
		if strings.HasPrefix(command, "pdftoppm") {
			rendered++
		}
	}
	if rendered != 2 {
		t.Fatalf("the second run called pdftoppm %d times, want 2, one per lost page", rendered)
	}
}

func TestOverwriteRendersEverythingAgain(t *testing.T) {
	run := &poppler{pages: 4, pad: 1, ink: map[int]float64{1: 0.05, 2: 0.05, 3: 0.05, 4: 0.05}}
	opts := options(t, run)
	if _, err := Render(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	before := len(run.seen)
	opts.Overwrite = true
	if _, err := Render(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	var rendered int
	for _, command := range run.seen[before:] {
		if strings.HasPrefix(command, "pdftoppm") {
			rendered++
		}
	}
	if rendered != 1 {
		t.Fatalf("overwrite called pdftoppm %d times, want 1 batch covering all 4 pages", rendered)
	}
}

func TestRetryDPIReachesThePixels(t *testing.T) {
	run := &poppler{pages: 2, pad: 1, ink: map[int]float64{1: 0.05, 2: 0.05}}
	opts := options(t, run)
	opts.DPI = RetryDPI

	manifest, err := Render(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	for _, page := range manifest.Pages {
		if page.DPI != RetryDPI {
			t.Fatalf("page %d says %d dpi, want %d", page.Page, page.DPI, RetryDPI)
		}
		if page.Width != RetryDPI/10 {
			t.Fatalf("page %d is %d wide, so the dpi flag did not reach pdftoppm", page.Page, page.Width)
		}
	}
	var found bool
	for _, command := range run.seen {
		if strings.Contains(command, "-r 600") && strings.Contains(command, "-gray") {
			found = true
		}
	}
	if !found {
		t.Fatalf("no pdftoppm call at 600 dpi gray: %v", run.seen)
	}
}

func TestPageRangeIsRespected(t *testing.T) {
	run := &poppler{pages: 100, pad: 3, ink: map[int]float64{}}
	for page := 1; page <= 100; page++ {
		run.ink[page] = 0.05
	}
	opts := options(t, run)
	opts.First, opts.Last = 40, 45

	manifest, err := Render(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Pages) != 6 {
		t.Fatalf("rendered %d pages, want 6", len(manifest.Pages))
	}
	if manifest.Pages[0].Page != 40 || manifest.Pages[5].Page != 45 {
		t.Fatalf("range came out as %d to %d", manifest.Pages[0].Page, manifest.Pages[5].Page)
	}
	if _, err := os.Stat(ImagePath(opts.Corpus, opts.Book, 39)); !os.IsNotExist(err) {
		t.Fatalf("page 39 was outside the range and got rendered anyway")
	}
}

// The OCR retry escalates a failed page to 600 dpi, one page at a time. That
// used to write a manifest with one page in it and the other 733 were gone.
// The pilot did exactly this to Algebra I: afterwards the manifest held a
// single entry for page 45, `ocr fill` answered "0 pages in range" for every
// range because it reads this file to know what exists, and the blank flags for
// the whole volume went with it.
func TestARetryAtSixHundredDoesNotEraseTheRestOfTheBook(t *testing.T) {
	run := &poppler{pages: 20, pad: 4, ink: map[int]float64{}}
	for page := 1; page <= 20; page++ {
		run.ink[page] = 0.05
	}
	run.ink[7] = 0.0 // a blank, so the merge can be checked to keep one

	opts := options(t, run)
	if _, err := Render(context.Background(), opts); err != nil {
		t.Fatal(err)
	}

	retry := opts
	retry.DPI, retry.First, retry.Last, retry.Batch, retry.Overwrite = RetryDPI, 12, 12, 1, true
	manifest, err := Render(context.Background(), retry)
	if err != nil {
		t.Fatal(err)
	}

	if len(manifest.Pages) != 20 {
		t.Fatalf("the manifest has %d pages after one page was re-rendered, want 20", len(manifest.Pages))
	}
	// It has to survive the round trip to disk, because that is the copy every
	// later command reads.
	onDisk, err := ReadManifest(opts.Corpus, opts.Book)
	if err != nil {
		t.Fatal(err)
	}
	if len(onDisk.Pages) != 20 {
		t.Fatalf("the manifest on disk has %d pages, want 20", len(onDisk.Pages))
	}
	page, ok := onDisk.Find(12)
	if !ok || page.DPI != RetryDPI {
		t.Errorf("page 12 came back as %+v, want the re-rendered one at %d dpi", page, RetryDPI)
	}
	if page, ok := onDisk.Find(3); !ok || page.DPI != DefaultDPI {
		t.Errorf("page 3 came back as %+v, want the original at %d dpi", page, DefaultDPI)
	}
	if onDisk.Blanks() != 1 {
		t.Errorf("%d blanks after the merge, want the 1 that was there before", onDisk.Blanks())
	}
	// The top level dpi describes the run that covered the book, not the one
	// page retry that came after it.
	if onDisk.DPI != DefaultDPI {
		t.Errorf("the manifest says %d dpi, want %d", onDisk.DPI, DefaultDPI)
	}
}

// A merge is only safe while the entries describe the same file. Replace the
// PDF and every page that was not re-rendered is a page that no longer exists.
func TestADifferentPDFReplacesTheManifestRatherThanMerging(t *testing.T) {
	run := &poppler{pages: 20, pad: 4, ink: map[int]float64{}}
	for page := 1; page <= 20; page++ {
		run.ink[page] = 0.05
	}
	opts := options(t, run)
	if _, err := Render(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(opts.PDF, []byte("%PDF-1.4\na different book\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	second := opts
	second.First, second.Last, second.Batch, second.Overwrite = 1, 5, 1, true
	manifest, err := Render(context.Background(), second)
	if err != nil {
		t.Fatal(err)
	}
	if len(manifest.Pages) != 5 {
		t.Fatalf("the manifest has %d pages, want only the 5 that were rendered from the new file", len(manifest.Pages))
	}
}

func TestEmptyRangeIsAnError(t *testing.T) {
	run := &poppler{pages: 10, pad: 2, ink: map[int]float64{}}
	opts := options(t, run)
	opts.First, opts.Last = 8, 3
	if _, err := Render(context.Background(), opts); err == nil {
		t.Fatal("a backwards page range should be an error, not an empty run")
	}
}

func TestInkCountsEveryPixel(t *testing.T) {
	// One dark row out of a hundred is one percent, well over the threshold. A
	// sampled count that skipped rows would call this blank and drop a section
	// title out of the corpus without saying so.
	img := image.NewGray(image.Rect(0, 0, 100, 100))
	for i := range img.Pix {
		img.Pix[i] = 0xff
	}
	for x := 0; x < 100; x++ {
		img.Set(x, 42, color.Gray{Y: 0})
	}
	if got := Ink(img); got < 0.009 || got > 0.011 {
		t.Fatalf("ink came out %.4f, want about 0.01", got)
	}
	if got := Ink(img); got < BlankInk {
		t.Fatalf("a page with a line of text on it reads as blank at %.4f", got)
	}

	// The generic path has to agree with the gray one, otherwise a colour scan
	// gets a different threshold than a gray one for no stated reason.
	rgba := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			rgba.Set(x, y, color.White)
		}
	}
	for x := 0; x < 100; x++ {
		rgba.Set(x, 42, color.Black)
	}
	if got := Ink(rgba); got < 0.009 || got > 0.011 {
		t.Fatalf("the colour path came out %.4f, want about 0.01", got)
	}
}

func TestEmptyImageIsNotADivideByZero(t *testing.T) {
	if got := Ink(image.NewGray(image.Rect(0, 0, 0, 0))); got != 0 {
		t.Fatalf("ink of an empty image is %v", got)
	}
}

func TestManifestIsWrittenAtomically(t *testing.T) {
	run := &poppler{pages: 3, pad: 1, ink: map[int]float64{1: 0.05, 2: 0.05, 3: 0.05}}
	opts := options(t, run)
	if _, err := Render(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	entries, err := os.ReadDir(ImagesDir(opts.Corpus, opts.Book))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".tmp") || strings.HasPrefix(entry.Name(), ".render-") {
			t.Fatalf("%s was left behind", entry.Name())
		}
	}
}

func TestCancelledRunStops(t *testing.T) {
	run := &poppler{pages: 100, pad: 3, ink: map[int]float64{}}
	for page := 1; page <= 100; page++ {
		run.ink[page] = 0.05
	}
	opts := options(t, run)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := Render(ctx, opts); err == nil {
		t.Fatal("a cancelled render should stop, not run to the end")
	}
}

func TestSummaryReportsWhatTheRunCost(t *testing.T) {
	manifest := Manifest{Book: "alg-i-iii", DPI: 300, Pages: []Page{
		{Page: 1, Bytes: 1000, Width: 1513, Height: 2357, Blank: true},
		{Page: 2, Bytes: 200000, Width: 1513, Height: 2357},
	}}
	got := manifest.Summary()
	if !strings.Contains(got, "1 blank, so 1 pages go to the model") {
		t.Fatalf("summary does not say what the blanks saved:\n%s", got)
	}
	var empty Manifest
	if empty.Summary() == "" {
		t.Fatal("an empty manifest should still say something")
	}
}

func TestARangeIsChunkedAndAListIsGrouped(t *testing.T) {
	for _, c := range []struct {
		name             string
		options          Options
		first, last, all int
		want             [][2]int
	}{
		{"a plain range", Options{Batch: 25}, 1, 60, 505, [][2]int{{1, 25}, {26, 50}, {51, 60}}},
		// The pages the extraction of Algebra VIII could not read, at the start
		// of the list: scattered singles, one pair, and one run of three. Asking
		// pdftoppm for 41 to 77 would render thirty five pages nobody wants.
		{"the flagged pages", Options{Batch: 25, Only: []int{76, 41, 77, 61, 310, 311, 312}}, 0, 0, 505,
			[][2]int{{41, 41}, {61, 61}, {76, 77}, {310, 312}}},
		{"a page that is not in the volume", Options{Only: []int{3, 900}}, 0, 0, 505, [][2]int{{3, 3}}},
		{"the same page twice", Options{Only: []int{7, 7}}, 0, 0, 505, [][2]int{{7, 7}}},
		// A long run still breaks at the batch size, so one pdftoppm call is
		// never the whole volume.
		{"a run longer than a batch", Options{Batch: 2, Only: []int{4, 5, 6, 7}}, 0, 0, 505,
			[][2]int{{4, 5}, {6, 7}}},
	} {
		got := spans(c.options, c.first, c.last, c.all)
		if len(got) != len(c.want) {
			t.Errorf("%s: spans = %v, want %v", c.name, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("%s: spans = %v, want %v", c.name, got, c.want)
				break
			}
		}
	}
}

func TestDPIDoesNotGoAboveTheScan(t *testing.T) {
	tests := []struct {
		want, source, dpi int
	}{
		// Fonctions d'une variable reelle is scanned at 150, so the default
		// asks for twice what is there and gets what is there.
		{300, 150, 150},
		// Commutative Algebra reports 301 and means 300. A cap that took that
		// literally would render the whole volume one dot short of the default
		// and call it a saving.
		{300, 301, 300},
		{600, 301, 301},
		{600, 600, 600},
		{300, 600, 300},
		// A volume the probe never got a figure out of, like the French
		// Algebra chapter 10, renders at what it is asked for.
		{300, 0, 300},
	}
	for _, test := range tests {
		if got := capDPI(test.want, test.source); got != test.dpi {
			t.Errorf("capDPI(%d, %d) = %d, want %d", test.want, test.source, got, test.dpi)
		}
	}
}

func TestARenderOfALowResolutionScanStaysAtTheScan(t *testing.T) {
	run := &poppler{pages: 2, pad: 1, ink: map[int]float64{1: 0.05, 2: 0.05}}
	opts := options(t, run)
	opts.SourceDPI = 150

	manifest, err := Render(context.Background(), opts)
	if err != nil {
		t.Fatal(err)
	}
	if manifest.DPI != 150 {
		t.Fatalf("the manifest says %d dpi, want 150", manifest.DPI)
	}
	for _, command := range run.seen {
		if strings.Contains(command, "-r 300") {
			t.Fatalf("pdftoppm was asked for 300 dpi on a 150 dpi scan: %v", run.seen)
		}
	}
}

// A render that named its pages writes blank page files for those pages and no
// others.
//
// The manifest writeBlanks walks is the merged one, so after a few windows it
// holds the whole volume. ocr run -window calls this once per window, and a
// blank page outside the window is not this render's business: its image is not
// even on disk any more, it was swept when its own window finished.
func TestARenderOfNamedPagesWritesBlanksForThosePagesOnly(t *testing.T) {
	run := &poppler{pages: 6, pad: 1, ink: map[int]float64{
		1: 0, 2: 0.06, 3: 0, 4: 0.06, 5: 0, 6: 0.06,
	}}
	opts := options(t, run)
	opts.WriteBlanks = true

	// The first window is pages 1 and 2, and only page 1 of them is blank.
	opts.Only = []int{1, 2}
	if _, err := Render(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(corpus.PagePath(opts.Corpus, opts.Book, 1)); err != nil {
		t.Errorf("the blank page of the window got no file: %v", err)
	}
	for _, page := range []int{3, 5} {
		if _, err := os.Stat(corpus.PagePath(opts.Corpus, opts.Book, page)); !os.IsNotExist(err) {
			t.Errorf("page %d is outside the window and was written anyway: %v", page, err)
		}
	}

	// The next window takes page 3, and the merged manifest still holding page
	// 1 does not put page 5 on disk.
	opts.Only = []int{3, 4}
	if _, err := Render(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(corpus.PagePath(opts.Corpus, opts.Book, 3)); err != nil {
		t.Errorf("the blank page of the second window got no file: %v", err)
	}
	if _, err := os.Stat(corpus.PagePath(opts.Corpus, opts.Book, 5)); !os.IsNotExist(err) {
		t.Errorf("page 5 was never in a window and was written anyway: %v", err)
	}

	// A render that names nothing is the whole volume, as it was.
	opts.Only = nil
	if _, err := Render(context.Background(), opts); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(corpus.PagePath(opts.Corpus, opts.Book, 5)); err != nil {
		t.Errorf("a render of the whole volume left a blank page without a file: %v", err)
	}
}
