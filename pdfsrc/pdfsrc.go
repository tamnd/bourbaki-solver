package pdfsrc

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Source is one PDF on disk.
type Source struct {
	Path string
	Run  Runner
}

// Open checks the file is readable and returns a Source that shells out for
// real. Pass a Source literal with a FakeRunner to avoid poppler in tests.
func Open(path string) (*Source, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, fmt.Errorf("open pdf: %w", err)
	}
	return &Source{Path: path, Run: ExecRunner{}}, nil
}

// SHA256 hashes the file. This is what pins an edition: if someone swaps in a
// different scan, every artefact derived from it is invalidated and the audit
// says so rather than quietly mixing two printings.
func (s *Source) SHA256() (string, error) {
	f, err := os.Open(s.Path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := sha256.New()
	if _, err := io.Copy(h, f); err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Size is the file size in bytes.
func (s *Source) Size() (int64, error) {
	fi, err := os.Stat(s.Path)
	if err != nil {
		return 0, err
	}
	return fi.Size(), nil
}

// Info is what pdfinfo reports.
type Info struct {
	Pages      int
	Title      string
	Producer   string
	Creator    string
	PDFVersion string
	WidthPt    float64
	HeightPt   float64
	Encrypted  bool
}

// Info runs pdfinfo and parses its key/value output.
func (s *Source) Info(ctx context.Context) (Info, error) {
	out, err := s.Run.Run(ctx, "pdfinfo", s.Path)
	if err != nil {
		return Info{}, err
	}
	var info Info
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		key, val, ok := strings.Cut(sc.Text(), ":")
		if !ok {
			continue
		}
		val = strings.TrimSpace(val)
		switch strings.TrimSpace(key) {
		case "Pages":
			info.Pages, _ = strconv.Atoi(val)
		case "Title":
			info.Title = val
		case "Producer":
			info.Producer = val
		case "Creator":
			info.Creator = val
		case "PDF version":
			info.PDFVersion = val
		case "Encrypted":
			info.Encrypted = val != "no"
		case "Page size":
			// "385.92 x 596.16 pts" or "... pts (A4)"
			fields := strings.Fields(val)
			if len(fields) >= 3 && fields[1] == "x" {
				info.WidthPt, _ = strconv.ParseFloat(fields[0], 64)
				info.HeightPt, _ = strconv.ParseFloat(fields[2], 64)
			}
		}
	}
	if info.Pages == 0 {
		return info, fmt.Errorf("pdfinfo %s: no page count in output", filepath.Base(s.Path))
	}
	return info, sc.Err()
}

// Text extracts the native text layer for a page range, inclusive on both ends.
// Layout preserves the column positions, which is what the running head parsers
// depend on. A page range of 0 to 0 means the whole document.
func (s *Source) Text(ctx context.Context, first, last int, layout bool) (string, error) {
	args := []string{}
	if layout {
		args = append(args, "-layout")
	}
	if first > 0 {
		args = append(args, "-f", strconv.Itoa(first))
	}
	if last > 0 {
		args = append(args, "-l", strconv.Itoa(last))
	}
	args = append(args, s.Path, "-")
	out, err := s.Run.Run(ctx, "pdftotext", args...)
	return string(out), err
}

// Image is one row of pdfimages -list.
type Image struct {
	Page   int
	Num    int
	Type   string // "image"
	Width  int
	Height int
	Color  string // "gray", "rgb"
	Comp   int
	BPC    int
	Enc    string // "jbig2", "jpeg", "ccitt"
	XPPI   int
	YPPI   int
}

// Images lists the embedded images in a page range.
func (s *Source) Images(ctx context.Context, first, last int) ([]Image, error) {
	args := []string{"-list"}
	if first > 0 {
		args = append(args, "-f", strconv.Itoa(first))
	}
	if last > 0 {
		args = append(args, "-l", strconv.Itoa(last))
	}
	args = append(args, s.Path)
	out, err := s.Run.Run(ctx, "pdfimages", args...)
	if err != nil {
		return nil, err
	}
	var images []Image
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		f := strings.Fields(sc.Text())
		// page num type width height color comp bpc enc interp objID xppi yppi size ratio
		if len(f) < 13 {
			continue
		}
		page, err := strconv.Atoi(f[0])
		if err != nil {
			continue // header or separator row
		}
		img := Image{Page: page, Type: f[2], Color: f[5], Enc: f[8]}
		img.Num, _ = strconv.Atoi(f[1])
		img.Width, _ = strconv.Atoi(f[3])
		img.Height, _ = strconv.Atoi(f[4])
		img.Comp, _ = strconv.Atoi(f[6])
		img.BPC, _ = strconv.Atoi(f[7])
		// The object ID column is two fields wide, "14 0", so x-ppi and y-ppi
		// land at 12 and 13 rather than 11 and 12.
		img.XPPI, _ = strconv.Atoi(f[12])
		img.YPPI, _ = strconv.Atoi(f[13])
		images = append(images, img)
	}
	return images, sc.Err()
}

// Font is one row of pdffonts.
type Font struct {
	Name     string
	Type     string
	Encoding string
	Embedded bool
	Subset   bool
}

// Fonts lists the fonts used in the document. Whether they are embedded is the
// tell that separates a born-digital volume from a scan: a scan's text layer
// was produced by somebody else's OCR and references stock fonts it does not
// carry.
func (s *Source) Fonts(ctx context.Context) ([]Font, error) {
	out, err := s.Run.Run(ctx, "pdffonts", s.Path)
	if err != nil {
		return nil, err
	}
	var fonts []Font
	sc := bufio.NewScanner(strings.NewReader(string(out)))
	for sc.Scan() {
		line := sc.Text()
		if strings.HasPrefix(line, "name ") || strings.HasPrefix(line, "---") || strings.TrimSpace(line) == "" {
			continue
		}
		f := strings.Fields(line)
		if len(f) < 6 {
			continue
		}
		// The trailing columns are: emb sub uni objectID(2 fields).
		n := len(f)
		fonts = append(fonts, Font{
			Name:     f[0],
			Type:     strings.Join(f[1:n-6], " "),
			Encoding: f[n-6],
			Embedded: f[n-5] == "yes",
			Subset:   f[n-4] == "yes",
		})
	}
	return fonts, sc.Err()
}

// Nature is how a volume has to be extracted.
type Nature string

const (
	// NatureBornDigital means the text layer is real and pdftotext is enough.
	NatureBornDigital Nature = "born-digital"
	// NatureScanned means the pages are images and the text layer, if any, is
	// somebody else's OCR and cannot be trusted for mathematics.
	NatureScanned Nature = "scanned"
)

// Classification is what Classify decided and the evidence it decided on, so a
// wrong answer can be argued with instead of guessed at.
type Classification struct {
	Nature        Nature
	SampledPages  int
	PagesWithFull int // sampled pages carrying a full-page image
	Images        []Image
	Fonts         int
	FontsEmbedded int
}

// Classify decides which extraction path a volume takes, from a sample of pages
// rather than the whole file, since pdfimages over 700 pages is slow.
//
// The signal is page images, not fonts. A scan carries one image covering
// essentially every page; the born-digital volume carries none at all. Fonts
// look like a tempting second signal and are not: the 1998 scan of chapters I
// to III reports 3208 non-embedded fonts and 599 embedded ones, so "has an
// embedded font" would call it born-digital and send 734 scanned pages down the
// native text path.
//
// Blank pages have no image, so the threshold is a fraction rather than a
// requirement that every sampled page have one.
func (s *Source) Classify(ctx context.Context, samplePages int) (Classification, error) {
	if samplePages <= 0 {
		samplePages = 10
	}
	info, err := s.Info(ctx)
	if err != nil {
		return Classification{}, err
	}
	last := min(samplePages, info.Pages)
	images, err := s.Images(ctx, 1, last)
	if err != nil {
		return Classification{}, err
	}
	fonts, err := s.Fonts(ctx)
	if err != nil {
		return Classification{}, err
	}

	c := Classification{SampledPages: last, Images: images, Fonts: len(fonts)}
	for _, f := range fonts {
		if f.Embedded {
			c.FontsEmbedded++
		}
	}
	// A page image covers the page, so it is large in both directions. Anything
	// small is a figure or a publisher's logo and does not make a volume a scan.
	withFull := map[int]bool{}
	for _, im := range images {
		if im.Width >= 1000 && im.Height >= 1000 {
			withFull[im.Page] = true
		}
	}
	c.PagesWithFull = len(withFull)

	if last > 0 && float64(c.PagesWithFull)/float64(last) >= 0.8 {
		c.Nature = NatureScanned
	} else {
		c.Nature = NatureBornDigital
	}
	return c, nil
}

// BodyImage returns the largest page image away from page 1, which is the one
// that describes the body of a scan. Page 1 is excluded because in the 2003
// volume it is a 300 dpi colour plate and nothing like the pages behind it.
func (c Classification) BodyImage() (Image, bool) {
	var best Image
	for _, im := range c.Images {
		if im.Page > 1 && im.Width*im.Height > best.Width*best.Height {
			best = im
		}
	}
	return best, best.Width > 0
}

// Render rasterises a page range to PNG files named <prefix>-NNNN.png, which is
// the input to vision OCR. Gray at 300 dpi is the default for the bilevel scans
// in scope; a retry escalates to 600.
func (s *Source) Render(ctx context.Context, first, last, dpi int, gray bool, prefix string) error {
	if err := os.MkdirAll(filepath.Dir(prefix), 0o755); err != nil {
		return err
	}
	args := []string{"-png", "-r", strconv.Itoa(dpi)}
	if gray {
		args = append(args, "-gray")
	}
	args = append(args, "-f", strconv.Itoa(first), "-l", strconv.Itoa(last))
	// Four digits covers 734 pages with room to spare and sorts lexically.
	args = append(args, "-progress", s.Path, prefix)
	_, err := s.Run.Run(ctx, "pdftoppm", args...)
	return err
}
