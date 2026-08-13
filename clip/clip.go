// Package clip cuts a page out of a printed volume and asks a model to read the
// picture of it.
//
// The extractor reads the born-digital volumes out of the text layer, which is
// exact where the font says what it draws and guesswork where it does not. Every
// defect found so far came from the same place: a glyph the font names in TeX's
// vocabulary rather than Adobe's, so poppler falls back on the code and hands
// back the character that code happens to stand for. A check accent arrives as
// the letter q, a musical flat as an opening bracket, a capital omega as the ohm
// sign. Each one is fixed by writing the font's encoding down in Go, one table
// at a time, and each table is a thing somebody had to decompress a font
// programme to learn.
//
// A picture does not need any of that. The model sees what a reader sees, so
// the question it answers is the question the tables are trying to answer, and
// it answers it without being told which font the page was set in. That makes it
// two things at once: a reading that can be better than ours where the text
// layer is misleading, and a second opinion that is genuinely independent of
// ours, which is what an audit needs.
//
// The unit is a page. This was built to cut single lines, on the reasoning that
// a line is the smallest thing that carries the defect and the cheapest thing to
// send, and both halves of that were wrong. A line is not cheaper: a model call
// is three or four minutes whether it is given six words or four hundred. And a
// line is not enough to read from. Twenty-four of them went out over Théories
// spectrales and the answers guessed at what the missing context would have
// settled, turning an interior into a closure, a rho into a q, and on one line
// dropping the bar of a not-equals so that the answer said a vector was zero
// where the page says it is not. Cutting single lines is still here because it
// is the sharper instrument when the question is about one glyph and the answer
// is going to be read by a person, and it is not what to reach for first.
//
// What it is not is a replacement for extraction, and the first seven pages
// audited say so plainly. The model fixed every loose accent on them, twenty
// seven of them, and it also read the interior of V as its closure seven times
// on one page, wrote the polar of a cone as its interior on another, and gave
// the spectral radius as varrho on one run of a page and as rho on the next.
// Ours are narrow and mechanical and its are broad and not repeatable, so what
// this produces is a list of places to look rather than a page to ship. It is
// also for the pages worth minutes: the ones carrying a glyph the extractor has
// just been taught to read, and the ones a rule flagged.
//
// Nothing here is committed but the report. The clips are scratch under work/,
// the same as the page images under images/, because the volumes are in
// copyright and a picture of a line of one is still a picture of it.
package clip

import (
	"context"
	"encoding/json"
	"fmt"
	"image"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"time"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// DefaultDPI is what a clip is cut at.
//
// Twice what a whole page goes to the model at, and it costs nothing here. A
// page at 600 dpi is a megabyte and a half and the fleet uploads it over a
// browser control; one line at 600 dpi is thirty kilobytes. The type is 10 pt,
// so 600 dpi puts about 80 pixels on a capital, and an accent drawn over a
// letter is the whole question a clip is asked, which is a detail two or three
// pixels across at 150.
const DefaultDPI = 600

// PageDPI is what a clip of a whole page is cut at.
//
// Half what a line goes at, and it is the same amount of type either way. A
// line at 600 dpi is thirty kilobytes and a page at 600 dpi is a megabyte and a
// half, which has to go up a browser control on a rented box; at 300 it is
// about two hundred kilobytes and the type is still 40 pixels on a capital,
// which is more than a scan of the same page carries. The margins are cut away
// as well, so the letters take a larger share of the pixels that are left.
const PageDPI = 300

// Pad is how much of the page is kept around the line, in the units pdftohtml
// reports.
//
// Some is needed: a box is the ink and the accents of Bourbaki's mathematics
// sit outside their letter's box, so a cut at the box loses the top of the very
// mark the clip was made to settle. Too much and the line above bleeds in.
// Measured on Théories spectrales, where the lines sit 22 units apart and carry
// 14 to 15 units of type, 4 keeps every accent and takes in the descenders of
// the line above and nothing more of it.
const Pad = 4

// Zoom is the scale pdftohtml -xml reports its boxes at.
//
// Poppler's XML is not in points. It applies a zoom of 1.5 and rounds to whole
// pixels, so a 439 by 666 pt page comes out 659 by 999. It is a constant of the
// tool rather than of a volume, and it is checked against the page rather than
// trusted: a poppler that changed it would otherwise cut every clip in the
// wrong place, quietly and by a third.
const Zoom = 1.5

// Box is a rectangle in the units pdftohtml reports, measured from the top left
// of the page.
type Box struct {
	Left   int `json:"left"`
	Top    int `json:"top"`
	Right  int `json:"right"`
	Bottom int `json:"bottom"`
}

// Pixels is the box as pdftoppm wants it: pixels of the page rendered at this
// dpi, with the padding added and the edges rounded outwards, since a rounded
// in edge is a shaved accent.
func (b Box) Pixels(dpi int, zoom float64, pad int) image.Rectangle {
	scale := float64(dpi) / 72 / zoom
	at := func(v int) int { return int(float64(v) * scale) }
	return image.Rect(
		max(0, at(b.Left-pad)), max(0, at(b.Top-pad)),
		at(b.Right+pad)+1, at(b.Bottom+pad)+1)
}

// Target is one line of one page, and everything needed to cut it out and to
// judge what comes back.
type Target struct {
	Page int    `json:"page"`
	Line int    `json:"line"`
	Name string `json:"name"`
	// Native is what the extractor reads on that line today. It is written down
	// when the clip is cut rather than read again when the answer arrives, so
	// that the two halves of a comparison are the same reading and a rebuild in
	// between cannot silently move one of them.
	Native string `json:"native"`
	// Head is the furniture of a page: the running head and the folio. The
	// extractor keeps both in the front matter rather than the body, because
	// they belong to the paper and not to the argument. A model has no front
	// matter to put them in and writes them across the top of the page, so an
	// audit that did not know what they say would report the head of every page
	// as four words the model read and we have not. Empty for a line.
	Head string `json:"head,omitempty"`
	Box  Box    `json:"box"`
}

// Index is what a cut produced. It sits beside the clips and is what the read
// and the audit work from, so neither of them has to open the PDF again.
type Index struct {
	Book      string    `json:"book"`
	PDF       string    `json:"pdf"`
	PDFSHA256 string    `json:"pdf_sha256"`
	DPI       int       `json:"dpi"`
	Zoom      float64   `json:"zoom"`
	Pad       int       `json:"pad"`
	Match     string    `json:"match,omitempty"`
	Generated time.Time `json:"generated"`
	Targets   []Target  `json:"targets"`
}

// Dir is where the clips of one volume go. Under work/, which is scratch and is
// not committed.
func Dir(root, book string) string { return filepath.Join(root, "work", "clips", book) }

// IndexPath is where the index of a cut goes.
func IndexPath(root, book string) string { return filepath.Join(Dir(root, book), "clips.json") }

// AnswersDir is where the Markdown the fleet returns is pulled back to.
func AnswersDir(root, book string) string { return filepath.Join(Dir(root, book), "answers") }

// Name is what a clip is called. The page and the line, four digits and three,
// so a directory of them sorts in reading order and the name says what it is
// without opening the index.
func Name(page, line int) string { return fmt.Sprintf("%04d-%03d.png", page, line) }

// PageName is what a clip of a whole page is called. The same four digits the
// corpus names its pages with, and no line number, because there is no line.
func PageName(page int) string { return fmt.Sprintf("%04d.png", page) }

// WholePage is the line number a target carries when it is a whole page rather
// than a line of one. It is negative so that it cannot be confused with the
// first line of a page, which is line zero.
const WholePage = -1

// Whole says whether a target is a page rather than a line.
func (t Target) Whole() bool { return t.Line < 0 }

// Options is one cut.
type Options struct {
	DPI  int
	Pad  int
	Zoom float64
	// Gray is off here, unlike a page render. A page goes to the model as gray
	// because the file is uploaded and the bytes are what the pipeline pays for.
	// A clip is thirty kilobytes either way, and these volumes are set in black
	// on white, so there is nothing to gain by throwing a channel away.
	Gray bool
	Logf func(string, ...any)
}

func (o Options) dpi() int {
	if o.DPI > 0 {
		return o.DPI
	}
	return DefaultDPI
}

func (o Options) pad() int {
	if o.Pad > 0 {
		return o.Pad
	}
	return Pad
}

func (o Options) zoom() float64 {
	if o.Zoom > 0 {
		return o.Zoom
	}
	return Zoom
}

func (o Options) logf(format string, args ...any) {
	if o.Logf != nil {
		o.Logf(format, args...)
	}
}

// ZoomOf is the zoom the XML of a page was written at, worked out from the page
// itself.
//
// pdftohtml's own figure is a constant, and this asks the page rather than
// trusting it, because everything a clip does rests on the two frames agreeing
// and a silent disagreement cuts every line in the wrong place. A page whose
// size poppler did not report gives the constant back, which is what the tool
// has always used.
func ZoomOf(page pdfsrc.Page, widthPt float64) float64 {
	if widthPt <= 0 || page.Width <= 0 {
		return Zoom
	}
	return float64(page.Width) / widthPt
}

// Cut writes one PNG per target and the index beside them.
func Cut(ctx context.Context, source *pdfsrc.Source, dir string, options Options, targets []Target) error {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	scratch, err := os.MkdirTemp(dir, ".cut-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(scratch)

	for index, target := range targets {
		if err := ctx.Err(); err != nil {
			return err
		}
		box := target.Box.Pixels(options.dpi(), options.zoom(), options.pad())
		prefix := filepath.Join(scratch, "c")
		if err := source.Crop(ctx, target.Page, options.dpi(), options.Gray, box, prefix); err != nil {
			return fmt.Errorf("cut page %d line %d: %w", target.Page, target.Line, err)
		}
		written, err := only(scratch)
		if err != nil {
			return fmt.Errorf("cut page %d line %d: %w", target.Page, target.Line, err)
		}
		if err := os.Rename(written, filepath.Join(dir, target.Name)); err != nil {
			return err
		}
		if (index+1)%25 == 0 {
			options.logf("cut %d of %d clips", index+1, len(targets))
		}
	}
	return nil
}

// only is the single PNG the last crop wrote.
//
// pdftoppm names its output after the page number and pads it to the width of
// the volume's page count, so the name is not predictable from the page alone:
// the same page is p-42.png in one book and p-042.png in another. The
// directory is emptied by this, so the file that is in it is the one just
// written, and finding two is a bug worth stopping on rather than a file to
// pick between.
func only(dir string) (string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	var found []string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".png") {
			found = append(found, filepath.Join(dir, entry.Name()))
		}
	}
	switch len(found) {
	case 1:
		return found[0], nil
	case 0:
		return "", fmt.Errorf("pdftoppm wrote no png")
	default:
		return "", fmt.Errorf("pdftoppm wrote %d pngs where one was asked for", len(found))
	}
}

// WriteIndex saves the index atomically.
func WriteIndex(path string, index Index) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	raw, err := json.MarshalIndent(index, "", "  ")
	if err != nil {
		return err
	}
	if err := os.WriteFile(path+".tmp", append(raw, '\n'), 0o644); err != nil {
		return err
	}
	return os.Rename(path+".tmp", path)
}

// ReadIndex loads what a cut produced.
func ReadIndex(path string) (Index, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return Index{}, err
	}
	var index Index
	if err := json.Unmarshal(raw, &index); err != nil {
		return Index{}, fmt.Errorf("decode clip index: %w", err)
	}
	return index, nil
}

// Query says which lines to cut.
type Query struct {
	// First and Last bound the pages, one based and inclusive. Zero means the
	// whole volume.
	First, Last int
	// Match is a regular expression against the line as the extractor renders
	// it. A line is cut when it matches.
	Match *regexp.Regexp
	// Pages, when set, is the only pages a line may come from, whatever the
	// range says. It is how the pages a report named are revisited.
	Pages map[int]bool
	// Limit stops the cut after this many lines. A clip is a model call of a
	// minute or more, so an unbounded match against a volume is a week of
	// fleet time, and every command that makes clips takes a limit.
	Limit int
	// Every is how many matching lines to skip between the ones kept. Zero and
	// one both mean keep all of them. It is for sampling a fault that occurs
	// six hundred times, where the useful thing is thirty of them spread over
	// the volume rather than the first thirty, which are all in chapter one.
	Every int
}

// Keep says whether a line matches, and it is where the sampling is done.
func (q Query) Keep(line string, seen int) bool {
	if q.Match != nil && !q.Match.MatchString(line) {
		return false
	}
	if q.Every > 1 && seen%q.Every != 0 {
		return false
	}
	return true
}

// InRange says whether a page is one the query asks about.
func (q Query) InRange(page int) bool {
	if q.Pages != nil && !q.Pages[page] {
		return false
	}
	if q.First > 0 && page < q.First {
		return false
	}
	if q.Last > 0 && page > q.Last {
		return false
	}
	return true
}
