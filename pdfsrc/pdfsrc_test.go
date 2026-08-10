package pdfsrc

import (
	"context"
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

// Note page 2 is missing: it is blank and carries no image.
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

const imagesAlgVIII = `page   num  type   width height color comp bpc  enc interp  object ID x-ppi y-ppi size ratio
--------------------------------------------------------------------------------------------
`

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
		"pdfinfo a.pdf":                    "Pages: 734\n",
		"pdfimages -list -f 1 -l 10 a.pdf": imagesAlgIIII,
		"pdffonts a.pdf":                   fontsAlgIIII,
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

func TestClassifyBornDigital(t *testing.T) {
	s := fake("b.pdf", map[string]string{
		"pdfinfo b.pdf":                    infoAlgVIII,
		"pdfimages -list -f 1 -l 10 b.pdf": imagesAlgVIII,
		"pdffonts b.pdf":                   fontsAlgVIII,
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

func TestUnregisteredCommandIsAnError(t *testing.T) {
	s := fake("a.pdf", map[string]string{})
	if _, err := s.Info(context.Background()); err == nil {
		t.Error("an unregistered command should fail loudly")
	}
}
