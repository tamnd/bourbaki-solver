// Package render turns the scanned volumes into page images for vision OCR.
//
// Algebra I and Algebra II are scans. There is no text layer to read, so every
// page has to go to a model as a picture, and the pictures are the one part of
// this pipeline that is expensive to move: they go over rsync to a rented box
// and then up a browser upload control. Page weight is the throughput
// constraint, not page quality, which is why the default is 300 dpi gray rather
// than the 600 dpi bilevel the scans were made at.
//
// Nothing here is committed. The images are scratch under images/, the page
// files are scratch under work/, and what survives is the Markdown.
package render

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tamnd/bourbaki-solver/corpus"
	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// DefaultDPI is what a page is rendered at first.
//
// The scans are 3026 x 4713 at 600 dpi bilevel. Rendering at 600 gives a PNG of
// one to two megabytes; 300 gives about 1513 x 2357 and a couple of hundred
// kilobytes, which is comfortably legible for the 10 pt Times these volumes are
// set in. A retry escalates.
const DefaultDPI = 300

// RetryDPI is what a page is rendered at when the model's first reading of it
// failed validation. Doubling the resolution is the cheapest thing to try and
// it is the only knob on this side of the call.
const RetryDPI = 600

// BlankInk is the ink ratio below which a page is treated as blank.
//
// A truly empty page of a scan is not empty: it has scanner speckle, and often
// a grey edge where the platen ended. Measured on the front and back matter of
// both volumes, real blanks come in under a tenth of a percent and the lightest
// page with anything on it, a part title with four words, is well above a
// percent. Two tenths of a percent sits in that gap with room on both sides.
const BlankInk = 0.002

// darkAt is the gray level below which a pixel counts as ink, out of 65535 as
// Go's image package reports it. Half way is deliberately generous: JBIG2 scans
// rendered to gray have soft edges and counting only the black core would
// under-report a light page.
const darkAt = 1 << 15

// Page is one rendered page.
type Page struct {
	Page   int     `json:"page"`
	File   string  `json:"file"`
	SHA256 string  `json:"sha256"`
	Bytes  int64   `json:"bytes"`
	Width  int     `json:"width"`
	Height int     `json:"height"`
	DPI    int     `json:"dpi"`
	Ink    float64 `json:"ink"`
	Blank  bool    `json:"blank"`
}

// Manifest is what a render run produced. It is written next to the images and
// read by the OCR stage, which needs the hash of each image to name its job and
// the blank flag to know which pages not to pay for.
type Manifest struct {
	Book      string    `json:"book"`
	PDF       string    `json:"pdf"`
	PDFSHA256 string    `json:"pdf_sha256"`
	DPI       int       `json:"dpi"`
	Gray      bool      `json:"gray"`
	Generated time.Time `json:"generated"`
	Pages     []Page    `json:"pages"`
}

// Blanks is how many pages were found blank, which is the number of model calls
// this run saved.
func (m Manifest) Blanks() int {
	var count int
	for _, page := range m.Pages {
		if page.Blank {
			count++
		}
	}
	return count
}

// Find returns one page of the manifest.
func (m Manifest) Find(page int) (Page, bool) {
	for _, value := range m.Pages {
		if value.Page == page {
			return value, true
		}
	}
	return Page{}, false
}

// Options is one render run.
type Options struct {
	Book string
	// PDF is the source file. Corpus is the checkout it and the outputs live
	// under.
	PDF    string
	Corpus string
	DPI    int
	// SourceDPI is the resolution of the images inside the PDF, off the scan
	// block of books.yaml. Rendering above it invents nothing: pdftoppm
	// interpolates, the page gets no sharper and the file gets bigger, and the
	// bytes are what this pipeline pays for. Fonctions d'une variable reelle is
	// scanned at 150, so the flat 300 was doubling every one of its 329 pages
	// for nothing. Zero means unknown, and then the default stands.
	SourceDPI int
	// Gray is on by default for these volumes: the source is bilevel, so gray
	// throws nothing away and the files come out smaller.
	Gray bool
	// First and Last bound the run, one based, inclusive. Zero means the whole
	// volume.
	First, Last int
	// Only is an explicit list of pages, which wins over First and Last. It is
	// for the pages a native extraction could not read: fifty six of Algebra
	// VIII carry a commutative diagram, a dropped glyph or a delimiter poppler
	// lost, and they are scattered from page 41 to page 477. Rendering the span
	// between them would be four hundred images nobody wants.
	Only []int
	// Batch is how many pages one pdftoppm call renders. Bigger is faster
	// because the PDF is opened once per call, and smaller means less work
	// thrown away when somebody stops the run.
	Batch int
	// Overwrite re-renders pages whose image is already there. Off by default,
	// so a run that was interrupted picks up where it stopped.
	Overwrite bool
	// WriteBlanks writes a method: blank page file for every blank page, which
	// is what keeps those pages out of the OCR queue.
	WriteBlanks bool
	Run         pdfsrc.Runner
	Logf        func(string, ...any)
}

// dpi is what the pages are rendered at: what was asked for, or the default,
// held down to what the scan actually holds.
func (o Options) dpi() int {
	want := o.DPI
	if want <= 0 {
		want = DefaultDPI
	}
	return capDPI(want, o.SourceDPI)
}

// capDPI holds a request down to the source. The cap is generous by a tenth
// because a scanner's own figure is a rounded one: Commutative Algebra reports
// 301 and is meant to be 300, and it would be silly to render that volume at
// 300 and call it capped, or to refuse a 600 retry on a 590 dpi scan.
func capDPI(want, source int) int {
	if source <= 0 || want <= source+source/10 {
		return want
	}
	return source
}

func (o Options) batch() int {
	if o.Batch > 0 {
		return o.Batch
	}
	return 25
}

func (o Options) logf(format string, args ...any) {
	if o.Logf != nil {
		o.Logf(format, args...)
	}
}

// ImagesDir is where the page images of one volume go.
func ImagesDir(root, book string) string {
	return filepath.Join(root, "images", book)
}

// ImagePath is one page image. Four digits, so the directory sorts in reading
// order and matches the page file names exactly.
func ImagePath(root, book string, page int) string {
	return filepath.Join(ImagesDir(root, book), fmt.Sprintf("%04d.png", page))
}

// ManifestPath is where the render manifest goes.
func ManifestPath(root, book string) string {
	return filepath.Join(ImagesDir(root, book), "manifest.json")
}

// Render rasterises a volume and returns what it produced.
func Render(ctx context.Context, options Options) (Manifest, error) {
	source, err := pdfsrc.Open(options.PDF)
	if err != nil {
		return Manifest{}, err
	}
	if options.Run != nil {
		source.Run = options.Run
	}
	info, err := source.Info(ctx)
	if err != nil {
		return Manifest{}, err
	}
	first, last := options.First, options.Last
	if first <= 0 {
		first = 1
	}
	if last <= 0 || last > info.Pages {
		last = info.Pages
	}
	if first > last {
		return Manifest{}, fmt.Errorf("page range %d to %d is empty", first, last)
	}

	directory := ImagesDir(options.Corpus, options.Book)
	if err := os.MkdirAll(directory, 0o755); err != nil {
		return Manifest{}, err
	}
	sum, err := source.SHA256()
	if err != nil {
		return Manifest{}, err
	}

	manifest := Manifest{
		Book: options.Book, PDF: options.PDF, PDFSHA256: sum,
		DPI: options.dpi(), Gray: options.Gray, Generated: time.Now().UTC(),
	}

	for _, span := range spans(options, first, last, info.Pages) {
		if err := ctx.Err(); err != nil {
			return manifest, err
		}
		start, end := span[0], span[1]
		want := missing(options, start, end)
		if len(want) > 0 {
			if err := renderRange(ctx, source, options, want[0], want[len(want)-1]); err != nil {
				return manifest, err
			}
		}
		for page := start; page <= end; page++ {
			value, err := measure(options, page)
			if err != nil {
				return manifest, err
			}
			manifest.Pages = append(manifest.Pages, value)
		}
		options.logf("rendered %d to %d of %d", start, end, last)
	}

	manifest = merge(options, manifest)
	sort.Slice(manifest.Pages, func(i, j int) bool { return manifest.Pages[i].Page < manifest.Pages[j].Page })
	if err := WriteManifest(options.Corpus, options.Book, manifest); err != nil {
		return manifest, err
	}
	if options.WriteBlanks {
		if err := writeBlanks(options, manifest); err != nil {
			return manifest, err
		}
	}
	return manifest, nil
}

// spans is the work as contiguous stretches of pages, each one a pdftoppm call.
//
// A range is chunked by the batch size. An explicit page list is grouped into
// its own runs instead, because pdftoppm renders a span, and fifty six scattered
// pages asked for as one span is the whole volume.
func spans(options Options, first, last, pages int) [][2]int {
	if len(options.Only) == 0 {
		var out [][2]int
		for start := first; start <= last; start += options.batch() {
			out = append(out, [2]int{start, min(start+options.batch()-1, last)})
		}
		return out
	}

	wanted := make([]int, 0, len(options.Only))
	seen := map[int]bool{}
	for _, page := range options.Only {
		if page < 1 || page > pages || seen[page] {
			continue
		}
		seen[page] = true
		wanted = append(wanted, page)
	}
	sort.Ints(wanted)

	var out [][2]int
	for index := 0; index < len(wanted); {
		start, end := wanted[index], wanted[index]
		index++
		for index < len(wanted) && wanted[index] == end+1 && end-start+1 < options.batch() {
			end = wanted[index]
			index++
		}
		out = append(out, [2]int{start, end})
	}
	return out
}

// missing is the pages of a batch that are not on disk yet. An interrupted run
// of 734 pages should not start over, and at a third of a second a page that
// matters.
func missing(options Options, first, last int) []int {
	var out []int
	for page := first; page <= last; page++ {
		if !options.Overwrite {
			if info, err := os.Stat(ImagePath(options.Corpus, options.Book, page)); err == nil && info.Size() > 0 {
				continue
			}
		}
		out = append(out, page)
	}
	return out
}

// pdftoppm names its output <prefix>-N.png with N zero padded to the width of
// the volume's page count, so a 734 page book gives three digits. The contract
// is four, so every file is renamed after it is written.
var rendered = regexp.MustCompile(`-(\d+)\.png$`)

func renderRange(ctx context.Context, source *pdfsrc.Source, options Options, first, last int) error {
	scratch, err := os.MkdirTemp(ImagesDir(options.Corpus, options.Book), ".render-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)

	if err := source.Render(ctx, first, last, options.dpi(), options.Gray, filepath.Join(scratch, "p")); err != nil {
		return err
	}
	entries, err := os.ReadDir(scratch)
	if err != nil {
		return err
	}
	for _, entry := range entries {
		match := rendered.FindStringSubmatch(entry.Name())
		if match == nil {
			continue
		}
		page, err := strconv.Atoi(match[1])
		if err != nil {
			continue
		}
		from := filepath.Join(scratch, entry.Name())
		to := ImagePath(options.Corpus, options.Book, page)
		// Rename rather than copy: same directory tree, so it is atomic, and a
		// reader never sees a half written PNG.
		if err := os.Rename(from, to); err != nil {
			return err
		}
	}
	return nil
}

// measure reads a rendered page back and records what it is.
//
// Reading the PNG again rather than trusting pdftoppm is the point: the hash is
// what names the OCR job, the dimensions are what say the dpi took effect, and
// the ink ratio is what keeps forty blank pages of front and back matter out of
// the queue at 151 seconds each.
func measure(options Options, page int) (Page, error) {
	path := ImagePath(options.Corpus, options.Book, page)
	raw, err := os.ReadFile(path)
	if err != nil {
		return Page{}, err
	}
	// The whole file, not a stream: a 300 dpi gray page is a couple of hundred
	// kilobytes, the hash has to cover every byte of it because that hash is
	// what names the OCR job, and the decoder stops reading at the end of the
	// image data rather than the end of the file.
	sum := sha256.Sum256(raw)
	decoded, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return Page{}, fmt.Errorf("%s: %w", path, err)
	}

	ink := Ink(decoded)
	bounds := decoded.Bounds()
	return Page{
		Page: page, File: filepath.Base(path), SHA256: hex.EncodeToString(sum[:]),
		Bytes: int64(len(raw)), Width: bounds.Dx(), Height: bounds.Dy(), DPI: options.dpi(),
		Ink: ink, Blank: ink < BlankInk,
	}, nil
}

// Ink is the fraction of the page that is dark.
//
// Every pixel is looked at rather than a sample. A page of a Bourbaki volume
// can be one line of text, a section title, and sampling every fourth row would
// eventually call one of those blank and drop it silently from the corpus.
func Ink(img image.Image) float64 {
	bounds := img.Bounds()
	total := bounds.Dx() * bounds.Dy()
	if total == 0 {
		return 0
	}
	var dark int
	switch typed := img.(type) {
	case *image.Gray:
		// The common case for these volumes, and worth the special path: it is
		// a byte per pixel with no conversion, over about three and a half
		// million pixels a page and 1194 pages.
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			row := typed.Pix[(y-bounds.Min.Y)*typed.Stride:][:bounds.Dx()]
			for _, value := range row {
				if uint32(value)<<8 < darkAt {
					dark++
				}
			}
		}
	default:
		for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
			for x := bounds.Min.X; x < bounds.Max.X; x++ {
				red, green, blue, _ := img.At(x, y).RGBA()
				if (red*299+green*587+blue*114)/1000 < darkAt {
					dark++
				}
			}
		}
	}
	return float64(dark) / float64(total)
}

// merge folds a partial render into the manifest that is already there.
//
// A render of one page used to write a manifest with one page in it, and the
// record of the other 733 was gone. That is not hypothetical: the OCR retry
// escalates a failed page to 600 dpi, the pilot escalated page 45, and
// afterwards the manifest of Algebra I held a single entry. Nothing downstream
// could see the book any more. `ocr fill` reads this file to know which pages
// exist and which are blank, so it answered "0 pages in range" for every range,
// and the blank detection for the whole volume was gone with it.
//
// Pages this run measured win, because it just measured them. Pages it did not
// touch are kept as they were. Each entry carries its own dpi, so a book at 300
// with one page at 600 is described exactly.
//
// A different PDF is not a merge. If the source file has been replaced then
// every old entry describes a page that no longer exists, and keeping them
// would be worse than losing them.
func merge(options Options, fresh Manifest) Manifest {
	old, err := ReadManifest(options.Corpus, options.Book)
	if err != nil || old.PDFSHA256 != fresh.PDFSHA256 {
		return fresh
	}
	rendered := make(map[int]bool, len(fresh.Pages))
	for _, page := range fresh.Pages {
		rendered[page.Page] = true
	}
	var kept int
	for _, page := range old.Pages {
		if !rendered[page.Page] {
			fresh.Pages = append(fresh.Pages, page)
			kept++
		}
	}
	if kept > 0 {
		// The top level dpi describes a run, and once two runs are in one file
		// it cannot describe both. The first one covered the book, so it is the
		// more useful answer, and the per page dpi is the exact one.
		fresh.DPI = old.DPI
		options.logf("kept %d pages of the previous manifest, replaced %d", kept, len(rendered))
	}
	return fresh
}

// WriteManifest saves the manifest atomically.
func WriteManifest(root, book string, manifest Manifest) error {
	path := ManifestPath(root, book)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(manifest, "", "  ")
	if err != nil {
		return err
	}
	temp := path + ".tmp"
	if err := os.WriteFile(temp, append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(temp, path)
}

// ReadManifest loads what a render run produced.
func ReadManifest(root, book string) (Manifest, error) {
	raw, err := os.ReadFile(ManifestPath(root, book))
	if err != nil {
		return Manifest{}, err
	}
	var manifest Manifest
	if err := json.Unmarshal(raw, &manifest); err != nil {
		return Manifest{}, fmt.Errorf("decode render manifest: %w", err)
	}
	return manifest, nil
}

// writeBlanks writes the page file for every blank page.
//
// A blank page still gets a file, because the page contract is one file per
// page and a gap in the sequence is indistinguishable from a page that was
// missed. It says method: blank and it costs nothing.
//
// One that is already on disk from the same image is left where it is. Nothing
// about it can have changed, and rewriting it only to move the generated stamp
// puts five files with no ink on them into a pull request somebody has to read.
func writeBlanks(options Options, manifest Manifest) error {
	for _, page := range manifest.Pages {
		if !page.Blank {
			continue
		}
		if blankAlreadyWritten(corpus.PagePath(options.Corpus, options.Book, page.Page), page.SHA256) {
			continue
		}
		file := corpus.PageFile{
			Meta: corpus.PageFrontMatter{
				Book: options.Book, PDFPage: page.Page, Method: corpus.MethodBlank,
				InputSHA256: page.SHA256,
			},
			Body: "",
		}
		path := corpus.PagePath(options.Corpus, options.Book, page.Page)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return err
		}
		if err := file.Write(path); err != nil {
			return err
		}
	}
	return nil
}

// blankAlreadyWritten says whether this page is on disk as a blank read from
// this same image. The image hash is what makes it safe: a page re-rendered at
// another dpi is a different image and is written again.
func blankAlreadyWritten(path, sha string) bool {
	old, err := corpus.ReadFile[corpus.PageFrontMatter](path)
	if err != nil {
		return false
	}
	return old.Meta.Method == corpus.MethodBlank && old.Meta.InputSHA256 == sha && sha != ""
}

// Summary is what a run prints when it finishes.
func (m Manifest) Summary() string {
	if len(m.Pages) == 0 {
		return "nothing rendered"
	}
	var bytes int64
	var width, height int
	for _, page := range m.Pages {
		bytes += page.Bytes
		width, height = max(width, page.Width), max(height, page.Height)
	}
	blanks := m.Blanks()
	var out strings.Builder
	fmt.Fprintf(&out, "%s: %d pages at %d dpi, %d x %d, %.1f MB, %.0f KB a page\n",
		m.Book, len(m.Pages), m.DPI, width, height,
		float64(bytes)/(1<<20), float64(bytes)/float64(len(m.Pages))/1024)
	fmt.Fprintf(&out, "%d blank, so %d pages go to the model\n", blanks, len(m.Pages)-blanks)
	return out.String()
}
