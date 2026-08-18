package pdfsrc

import (
	"context"
	"fmt"
	"strings"
	"testing"
)

// Every fixture below is real output, captured from poppler 25 against the
// three volumes in scope. Hand-written fixtures would have hidden both of the
// bugs these tests now pin down: the object ID column being two fields wide,
// and embedded fonts being a useless signal for what is a scan.

const infoAlgVIII = `Title:           Algebra
Producer:        Adobe PDF Library 15.0
Creator:         Adobe InDesign 16.3
Tagged:          no
Form:            AcroForm
Pages:           505
Encrypted:       no
Page size:       439.37 x 666.14 pts
Page rot:        0
File size:       7177367 bytes
PDF version:     1.6
`

const infoAlgIVVII = `Custom Metadata: no
Metadata Stream: no
Tagged:          no
Form:            AcroForm
Pages:           460
Encrypted:       no
Page size:       385.92 x 596.16 pts
Page rot:        0
File size:       7418389 bytes
PDF version:     1.6
`

// The front of the 1998 scan, still used for the column parsing test. Note page
// 2 is missing: it is blank and carries no image, and page 1 is the colour
// plate rather than a body page.
const imagesAlgIIII = `page   num  type   width height color comp bpc  enc interp  object ID x-ppi y-ppi size ratio
--------------------------------------------------------------------------------------------
   1     0 image    3026  4713  rgb     3   8  image  no      7007  0   600   600  123K 0.3%
   3     1 image    3026  4713  gray    1   1  jbig2  no         3  0   600   600 6872B 0.4%
   4     2 image    3026  4713  gray    1   1  jbig2  no         6  0   600   600 7554B 0.4%
   5     3 image    3026  4713  gray    1   1  jbig2  no         9  0   600   600 11.9K 0.7%
   6     4 image    3026  4713  gray    1   1  jbig2  no        13  0   600   600 19.3K 1.1%
   7     5 image    3026  4713  gray    1   1  jbig2  no        16  0   600   600 7432B 0.4%
   8     6 image    3026  4713  gray    1   1  jbig2  no        19  0   600   600   30B 0.0%
   9     7 image    3026  4713  gray    1   1  jbig2  no        22  0   600   600 11.8K 0.7%
  10     8 image    3026  4713  gray    1   1  jbig2  no        25  0   600   600 7550B 0.4%
`

// The body band of the 1998 scan, pages 183 to 192, which is where Classify
// looks. Page 190 is blank.
const imagesAlgIIIIBody = `page   num  type   width height color comp bpc  enc interp  object ID x-ppi y-ppi size ratio
--------------------------------------------------------------------------------------------
 183   180 image    3026  4713  gray    1   1  jbig2  no      1503  0   600   600 51.1K 3.0%
 184   181 image    3026  4713  gray    1   1  jbig2  no      1510  0   600   600 49.7K 2.9%
 185   182 image    3026  4713  gray    1   1  jbig2  no      1517  0   600   600 52.4K 3.1%
 186   183 image    3026  4713  gray    1   1  jbig2  no      1524  0   600   600 48.8K 2.9%
 187   184 image    3026  4713  gray    1   1  jbig2  no      1531  0   600   600 50.2K 3.0%
 188   185 image    3026  4713  gray    1   1  jbig2  no      1538  0   600   600 47.9K 2.8%
 189   186 image    3026  4713  gray    1   1  jbig2  no      1545  0   600   600 53.0K 3.1%
 191   187 image    3026  4713  gray    1   1  jbig2  no      1552  0   600   600 46.6K 2.7%
 192   188 image    3026  4713  gray    1   1  jbig2  no      1559  0   600   600 49.1K 2.9%
`

// The front of the 2007 French reprint of Algèbre chapters 1 to 3: a small
// colour cover, three pages of front matter Springer reset in type, and then
// the scan. Six full pages in ten is under the threshold, so reading the front
// of this file calls a 645-page scan born-digital.
const imagesAlgIIIIFrFront = `page   num  type   width height color comp bpc  enc interp  object ID x-ppi y-ppi size ratio
--------------------------------------------------------------------------------------------
   1     0 image     827  1252  rgb     3   8  jpeg   no         6  0   136   135 80.6K 2.7%
   5     1 image    1831  2775  gray    1   1  jbig2  no        56  0   300   300 24.3K 3.9%
   6     2 image    1831  2775  gray    1   1  jbig2  no        72  0   300   300 40.2K 6.5%
   7     3 image    1848  2786  gray    1   1  jbig2  no        84  0   300   300 27.5K 4.4%
   8     4 image    1831  2775  gray    1   1  jbig2  no        96  0   300   300 31.1K 5.0%
   9     5 image    1848  2783  gray    1   1  jbig2  no       108  0   300   300 28.4K 4.6%
  10     6 image    1831  2775  gray    1   1  jbig2  no       120  0   300   300 33.7K 5.4%
`

// The same file at page 161, a quarter of the way in, which is all scan.
const imagesAlgIIIIFrBody = `page   num  type   width height color comp bpc  enc interp  object ID x-ppi y-ppi size ratio
--------------------------------------------------------------------------------------------
 161     0 image    1831  2775  gray    1   1  jbig2  no      2801  0   300   300 39.9K 6.4%
 162     1 image    1840  2779  gray    1   1  jbig2  no      2820  0   300   300 41.7K 6.7%
 163     2 image    1831  2775  gray    1   1  jbig2  no      2839  0   300   300 39.9K 6.4%
 164     3 image    1848  2783  gray    1   1  jbig2  no      2858  0   300   300 35.1K 5.6%
 165     4 image    1831  2775  gray    1   1  jbig2  no      2877  0   300   300 38.2K 6.1%
 166     5 image    1840  2779  gray    1   1  jbig2  no      2896  0   300   300 40.4K 6.5%
 167     6 image    1831  2775  gray    1   1  jbig2  no      2915  0   300   300 37.6K 6.0%
 168     7 image    1848  2783  gray    1   1  jbig2  no      2934  0   300   300 36.3K 5.8%
 169     8 image    1831  2775  gray    1   1  jbig2  no      2953  0   300   300 39.1K 6.3%
 170     9 image    1840  2779  gray    1   1  jbig2  no      2972  0   300   300 41.0K 6.6%
`

// Ten body pages of the 2007 French reprint of Fonctions d'une variable
// réelle, scanned at 150 dpi.
const imagesFVRFr = `page   num  type   width height color comp bpc  enc interp  object ID x-ppi y-ppi size ratio
--------------------------------------------------------------------------------------------
  82     0 image     920  1390  gray    1   1  ccitt  no       604  0   150   150 19.9K  13%
  83     1 image     924  1392  gray    1   1  ccitt  no       607  0   150   150 18.5K  12%
  84     2 image     915  1388  gray    1   1  ccitt  no       610  0   150   150 21.3K  14%
  85     3 image     924  1390  gray    1   1  ccitt  no       613  0   150   150 23.9K  15%
  86     4 image     914  1386  gray    1   1  ccitt  no       616  0   150   150 27.4K  18%
  87     5 image     915  1388  gray    1   1  ccitt  no       619  0   150   150 29.2K  19%
  88     6 image     914  1386  gray    1   1  ccitt  no       622  0   150   150 25.7K  17%
  89     7 image     915  1388  gray    1   1  ccitt  no       625  0   150   150 16.9K  11%
  90     8 image     914  1386  gray    1   1  ccitt  no       628  0   150   150 14.5K 9.4%
  91     9 image     915  1388  gray    1   1  ccitt  no       631  0   150   150 16.7K  11%
`

const imagesAlgVIII = `page   num  type   width height color comp bpc  enc interp  object ID x-ppi y-ppi size ratio
--------------------------------------------------------------------------------------------
`

// Ten body pages of Algèbre chapter 10, each drawn as 24 ccitt stencils 2055 by
// 121 at 260 dpi. The real listing is 240 rows of that one geometry, so it is
// built here rather than pasted.
func imagesAlgXFr() string {
	b := "page   num  type   width height color comp bpc  enc interp  object ID x-ppi y-ppi size ratio\n" +
		"--------------------------------------------------------------------------------------------\n"
	num := 0
	for page := 55; page <= 64; page++ {
		for range 23 {
			b += fmt.Sprintf("%4d %5d stencil  2055   121  -       1   1  ccitt  no      %4d  0   260   260 3384B  11%%\n",
				page, num, 1300+num)
			num++
		}
		// The sliver at the foot of every page is stored raw, not ccitt.
		b += fmt.Sprintf("%4d %5d stencil  2055    51  -       1   1  image  no      %4d  0   260   260 13.0K 100%%\n",
			page, num, 1300+num)
		num++
	}
	return b
}

// The 1998 scan really does report a mix. This is why Classify ignores fonts.
const fontsAlgIIII = `name                                 type              encoding         emb sub uni object ID
------------------------------------ ----------------- ---------------- --- --- --- ---------
Times-Roman                          Type 1            WinAnsi          no  no  no    2842  0
Times-Bold                           Type 1            WinAnsi          no  no  no    2843  0
Helvetica-Oblique                    Type 1            WinAnsi          no  no  no    2845  0
AAAAAB+NimbusRomNo9L-Regu            Type 1            Custom           yes yes yes   2851  0
`

const fontsAlgVIII = `name                                 type              encoding         emb sub uni object ID
------------------------------------ ----------------- ---------------- --- --- --- ---------
KHMBRE+TimesNewRomanPSMT             TrueType          WinAnsi          yes yes yes    120  0
KHMBRF+TimesNewRomanPS-ItalicMT      TrueType          WinAnsi          yes yes yes    121  0
`

func fake(path string, out map[string]string) *Source {
	return &Source{Path: path, Run: &FakeRunner{Out: out}}
}

func TestInfo(t *testing.T) {
	s := fake("a.pdf", map[string]string{"pdfinfo a.pdf": infoAlgVIII})
	got, err := s.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Pages != 505 {
		t.Errorf("Pages = %d, want 505", got.Pages)
	}
	if got.Producer != "Adobe PDF Library 15.0" {
		t.Errorf("Producer = %q", got.Producer)
	}
	if got.Creator != "Adobe InDesign 16.3" {
		t.Errorf("Creator = %q", got.Creator)
	}
	if got.PDFVersion != "1.6" {
		t.Errorf("PDFVersion = %q", got.PDFVersion)
	}
	if got.Encrypted {
		t.Error("Encrypted should be false")
	}
	if got.WidthPt != 439.37 || got.HeightPt != 666.14 {
		t.Errorf("page size = %v x %v", got.WidthPt, got.HeightPt)
	}
}

// pdfinfo prints no Title or Producer for the 2003 scan, and the parser must
// not mind.
func TestInfoWithoutTitleOrProducer(t *testing.T) {
	s := fake("b.pdf", map[string]string{"pdfinfo b.pdf": infoAlgIVVII})
	got, err := s.Info(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if got.Pages != 460 || got.Producer != "" || got.Title != "" {
		t.Errorf("got %+v", got)
	}
}

func TestInfoWithoutPagesIsAnError(t *testing.T) {
	s := fake("c.pdf", map[string]string{"pdfinfo c.pdf": "Encrypted: no\n"})
	if _, err := s.Info(context.Background()); err == nil {
		t.Error("a pdfinfo run with no page count should be an error")
	}
}

// The object ID column is two fields wide, so x-ppi and y-ppi sit one place
// further right than a naive count suggests. Reading them one column early gave
// every scan a dpi of 0.
func TestImagesReadsDPIFromTheRightColumn(t *testing.T) {
	s := fake("a.pdf", map[string]string{"pdfimages -list -f 1 -l 10 a.pdf": imagesAlgIIII})
	imgs, err := s.Images(context.Background(), 1, 10)
	if err != nil {
		t.Fatal(err)
	}
	if len(imgs) != 9 {
		t.Fatalf("got %d images, want 9", len(imgs))
	}
	body := imgs[1]
	want := Image{Page: 3, Num: 1, Type: "image", Width: 3026, Height: 4713,
		Color: "gray", Comp: 1, BPC: 1, Enc: "jbig2", XPPI: 600, YPPI: 600}
	if body != want {
		t.Errorf("got %+v\nwant %+v", body, want)
	}
	if imgs[0].Color != "rgb" || imgs[0].Enc != "image" {
		t.Errorf("cover = %+v, want the rgb plate", imgs[0])
	}
}

func TestFonts(t *testing.T) {
	s := fake("a.pdf", map[string]string{"pdffonts a.pdf": fontsAlgIIII})
	fonts, err := s.Fonts(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(fonts) != 4 {
		t.Fatalf("got %d fonts, want 4", len(fonts))
	}
	if fonts[0].Name != "Times-Roman" || fonts[0].Type != "Type 1" ||
		fonts[0].Encoding != "WinAnsi" || fonts[0].Embedded || fonts[0].Subset {
		t.Errorf("got %+v", fonts[0])
	}
	if !fonts[3].Embedded || !fonts[3].Subset {
		t.Errorf("the subset font should read as embedded: %+v", fonts[3])
	}
}

// A scan with embedded fonts must still classify as a scan. Getting this wrong
// sent 734 scanned pages down the native text path.
func TestClassifyScanWithEmbeddedFonts(t *testing.T) {
	s := fake("a.pdf", map[string]string{
		"pdfinfo a.pdf":                       "Pages: 734\nPage size: 363.12 x 565.56 pts\n",
		"pdfimages -list -f 183 -l 192 a.pdf": imagesAlgIIIIBody,
		"pdffonts a.pdf":                      fontsAlgIIII,
	})
	c, err := s.Classify(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if c.Nature != NatureScanned {
		t.Errorf("Nature = %q, want %q", c.Nature, NatureScanned)
	}
	if c.PagesWithFull != 9 || c.SampledPages != 10 {
		t.Errorf("evidence = %d of %d", c.PagesWithFull, c.SampledPages)
	}
	if c.FontsEmbedded != 1 {
		t.Errorf("FontsEmbedded = %d, want 1", c.FontsEmbedded)
	}
	img, ok := c.BodyImage()
	if !ok {
		t.Fatal("BodyImage found nothing")
	}
	if img.Enc != "jbig2" || img.XPPI != 600 || img.Page == 1 {
		t.Errorf("BodyImage = %+v, want a jbig2 body page, not the cover", img)
	}
}

// Springer reset the front matter of their French reprints in type and scanned
// only the body. Classifying on the first ten pages called this 645-page scan
// born-digital, which would have sent it down the native text path.
func TestClassifyReprintWithTypesetFrontMatter(t *testing.T) {
	cmds := map[string]string{
		"pdfinfo a.pdf":                       "Pages: 645\nPage size: 439.37 x 666.142 pts\n",
		"pdfimages -list -f 1 -l 10 a.pdf":    imagesAlgIIIIFrFront,
		"pdfimages -list -f 161 -l 170 a.pdf": imagesAlgIIIIFrBody,
		"pdffonts a.pdf":                      fontsAlgIIII,
	}
	c, err := fake("a.pdf", cmds).Classify(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if c.Nature != NatureScanned {
		t.Errorf("Nature = %q, want %q", c.Nature, NatureScanned)
	}
	if c.First != 161 || c.Last != 170 {
		t.Errorf("sampled pages %d to %d, want the band a quarter of the way in", c.First, c.Last)
	}
	if c.PagesWithFull != 10 {
		t.Errorf("PagesWithFull = %d, want all ten body pages", c.PagesWithFull)
	}
}

// Fonctions d'une variable réelle is scanned at 150 dpi, so a full page of it
// is 914 by 1386 pixels. A pixel threshold reads that as a figure and the
// volume as born-digital, and it is a scan carrying somebody else's OCR.
func TestClassifyLowResolutionScan(t *testing.T) {
	c, err := fake("a.pdf", map[string]string{
		"pdfinfo a.pdf":                     "Pages: 329\nPage size: 439.37 x 666.142 pts\n",
		"pdfimages -list -f 82 -l 91 a.pdf": imagesFVRFr,
		"pdffonts a.pdf":                    fontsAlgIIII,
	}).Classify(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if c.Nature != NatureScanned {
		t.Errorf("Nature = %q, want %q", c.Nature, NatureScanned)
	}
	img, ok := c.BodyImage()
	if !ok || img.XPPI != 150 {
		t.Errorf("BodyImage = %+v, %v, want the 150 dpi page", img, ok)
	}
}

// Algèbre chapter 10 draws each page as two dozen ccitt strips 2055 by 121.
// No one strip is a page and together they cover it, and no strip is the
// geometry of the volume, so nothing goes in the scan block.
func TestClassifyPagesTiledOutOfStrips(t *testing.T) {
	c, err := fake("a.pdf", map[string]string{
		"pdfinfo a.pdf":                     "Pages: 222\nPage size: 612 x 792 pts\n",
		"pdfimages -list -f 55 -l 64 a.pdf": imagesAlgXFr(),
		"pdffonts a.pdf":                    "",
	}).Classify(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if c.Nature != NatureScanned {
		t.Errorf("Nature = %q, want %q", c.Nature, NatureScanned)
	}
	if c.PagesWithFull != 10 {
		t.Errorf("PagesWithFull = %d, want all ten", c.PagesWithFull)
	}
	if img, ok := c.BodyImage(); ok {
		t.Errorf("BodyImage = %+v, want nothing, since no strip is a page", img)
	}
	// The strips of one page do describe the page between them, and that is
	// what the volume gets recorded as: 23 of them 2055 by 121 at 260 ppi and
	// a sliver at the foot.
	img, ok := c.TiledImage()
	if !ok {
		t.Fatal("TiledImage found nothing on a volume that is nothing but tiles")
	}
	if want := 23*121 + 51; img.Width != 2055 || img.Height != want || img.XPPI != 260 || img.Enc != "ccitt" {
		t.Errorf("TiledImage = %+v, want 2055 by %d at 260 ppi ccitt", img, want)
	}
}

// A page with a figure on it also has more than one image on it, and it is not
// a tiled page. Two images that disagree about their width describe a page with
// something on it, not a page cut into strips.
func TestTiledImageWantsStripsThatAgree(t *testing.T) {
	c := Classification{
		PageWidthPt: 612, PageHeightPt: 792,
		Images: []Image{
			{Page: 4, Width: 2500, Height: 3300, XPPI: 300, YPPI: 300, Enc: "jbig2"},
			{Page: 4, Width: 600, Height: 400, XPPI: 300, YPPI: 300, Enc: "jpeg"},
		},
	}
	if img, ok := c.TiledImage(); ok {
		t.Errorf("TiledImage = %+v, want nothing: a scan of a page with a photo on it is not a tiled page", img)
	}
}

func TestClassifyBornDigital(t *testing.T) {
	s := fake("b.pdf", map[string]string{
		"pdfinfo b.pdf":                       infoAlgVIII,
		"pdfimages -list -f 126 -l 135 b.pdf": imagesAlgVIII,
		"pdffonts b.pdf":                      fontsAlgVIII,
	})
	c, err := s.Classify(context.Background(), 10)
	if err != nil {
		t.Fatal(err)
	}
	if c.Nature != NatureBornDigital {
		t.Errorf("Nature = %q, want %q", c.Nature, NatureBornDigital)
	}
	if c.PagesWithFull != 0 {
		t.Errorf("PagesWithFull = %d, want 0", c.PagesWithFull)
	}
	if _, ok := c.BodyImage(); ok {
		t.Error("a born-digital volume has no body image")
	}
}

func TestTextBuildsTheRightCommand(t *testing.T) {
	f := &FakeRunner{Out: map[string]string{"pdftotext -layout -f 40 -l 40 a.pdf -": "A.IV.31\n"}}
	s := &Source{Path: "a.pdf", Run: f}
	got, err := s.Text(context.Background(), 40, 40, true)
	if err != nil {
		t.Fatal(err)
	}
	if got != "A.IV.31\n" {
		t.Errorf("got %q", got)
	}
	f2 := &FakeRunner{Out: map[string]string{"pdftotext a.pdf -": "x"}}
	s2 := &Source{Path: "a.pdf", Run: f2}
	if _, err := s2.Text(context.Background(), 0, 0, false); err != nil {
		t.Fatal(err)
	}
}

func TestSpreadPagesStaysInTheMiddleHalf(t *testing.T) {
	for _, c := range []struct {
		pages, n int
		want     []int
	}{
		{642, 5, []int{160, 240, 320, 400, 481}},
		{4, 5, []int{1, 2, 3, 4}},
		{5, 5, []int{1, 2, 3, 4, 5}},
		{6, 5, []int{1, 1, 2, 3, 4}},
		{100, 1, []int{25}},
		{0, 5, nil},
		{100, 0, nil},
	} {
		got := spreadPages(c.pages, c.n)
		if len(got) != len(c.want) {
			t.Errorf("spreadPages(%d, %d) = %v, want %v", c.pages, c.n, got, c.want)
			continue
		}
		for i := range got {
			if got[i] != c.want[i] {
				t.Errorf("spreadPages(%d, %d) = %v, want %v", c.pages, c.n, got, c.want)
				break
			}
		}
	}
}

// The three volumes with no text layer at all report zero characters on every
// body page, and a scan somebody has already read reports a full page of them.
// Sampling the front matter would say the same thing about both, since a half
// title page is nearly empty in every volume of the series.
func TestSampleTextReadsBodyPages(t *testing.T) {
	s := fake("a.pdf", map[string]string{
		"pdfinfo a.pdf":                   "Pages: 400\n",
		"pdftotext -f 100 -l 100 a.pdf -": "  Théorème 1  \n",
		"pdftotext -f 150 -l 150 a.pdf -": "",
		"pdftotext -f 200 -l 200 a.pdf -": "\f\n \n",
		"pdftotext -f 250 -l 250 a.pdf -": "ab",
		"pdftotext -f 300 -l 300 a.pdf -": "",
	})
	got, err := s.SampleText(context.Background(), 5)
	if err != nil {
		t.Fatal(err)
	}
	if got.Pages != 5 {
		t.Errorf("Pages = %d, want 5", got.Pages)
	}
	// "Théorème1" is 9 characters and "ab" is 2. The form feed and the spaces
	// are not text and must not be counted as any.
	if got.Chars != 11 {
		t.Errorf("Chars = %d, want 11", got.Chars)
	}
	if got.PerPage() != 2 {
		t.Errorf("PerPage() = %d, want 2", got.PerPage())
	}
	if (TextSample{}).PerPage() != 0 {
		t.Error("an empty sample should average 0 rather than divide by zero")
	}
}

func TestUnregisteredCommandIsAnError(t *testing.T) {
	s := fake("a.pdf", map[string]string{})
	if _, err := s.Info(context.Background()); err == nil {
		t.Error("an unregistered command should fail loudly")
	}
}

// A glyph poppler drew and could not name arrives as an element with no
// characters in it, and it has to survive the parse. Lie chapters 7 to 9 names
// code 0x17 of its mathematics italic pi1, which is varpi, and its ToUnicode
// CMap is that one code short: every fundamental weight of the volume came out
// of pdftohtml as <text ...><i></i></text>. Dropping the element here is what
// kept the loss out of the flags, off the reports, and out of the queue of
// pages the model reads.
func TestParseXMLKeepsAGlyphPopplerCouldNotName(t *testing.T) {
	const page = `<?xml version="1.0"?>
<pdf2xml>
<page number="141" width="612" height="792">
<fontspec id="2" size="15" family="DGLKJH+CMR10" color="#000000"/>
<text top="260" left="226" width="6" height="13" font="2">(</text>
<text top="260" left="232" width="12" height="13" font="13"><i></i></text>
<text top="266" left="244" width="8" height="9" font="14"><i>&#945;</i></text>
</page>
</pdf2xml>
`
	l, err := ParseXML(strings.NewReader(page))
	if err != nil {
		t.Fatal(err)
	}
	if len(l.Pages) != 1 {
		t.Fatalf("read %d pages, want 1", len(l.Pages))
	}
	spans := l.Pages[0].Spans
	if len(spans) != 3 {
		t.Fatalf("read %d spans, want 3: %+v", len(spans), spans)
	}
	lost := spans[1]
	if lost.Text != "" {
		t.Errorf("the lost glyph reads %q", lost.Text)
	}
	// The box is what says where it stood and how wide the letter was.
	if lost.Left != 232 || lost.Width != 12 || lost.Font != 13 {
		t.Errorf("the box of the lost glyph is %+v", lost)
	}
}
