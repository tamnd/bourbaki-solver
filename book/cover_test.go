package book

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/corpus"
)

// There has to be one cover. The class draws it and the EPUB used to draw it
// again in SVG from the same eight numbers written out a second time, which is
// two covers that agree today and need not agree tomorrow. Taking page one of
// the PDF the same run just set is the only arrangement where they cannot
// disagree.

func coverVolume() *Volume {
	return &Volume{Lang: "en", Title: "Algebra",
		Meta: corpus.Book{ID: "alg-i-iii", Lang: "en", Chapters: []string{"I", "II", "III"},
			PageWidth: 363.12, PageHeight: 565.56}}
}

func TestAnEPUBWithARasterCoverNamesThePNGEverywhereAndTheSVGNowhere(t *testing.T) {
	v := coverVolume()
	opf := packageOPF(v, nil, Options{}, true)
	if !strings.Contains(opf, `href="cover.png"`) {
		t.Errorf("the manifest does not name cover.png:\n%s", opf)
	}
	if strings.Contains(opf, "cover.svg") {
		t.Errorf("the manifest still names cover.svg, which is not in the container")
	}
	// properties="svg" says the document embeds SVG and a reading system is
	// entitled to act on that, so a cover page holding a PNG must not carry it.
	if strings.Contains(opf, `properties="svg"`) {
		t.Errorf("the cover document is declared as embedding SVG and holds a PNG")
	}
	doc := coverDoc(v, true)
	if !strings.Contains(doc, `<img src="cover.png"`) {
		t.Errorf("the cover page does not show the raster:\n%s", doc)
	}
}

func TestAnEPUBBuiltWithNoPDFStillDrawsItsOwnCover(t *testing.T) {
	// -no-pdf is a real way to build and a machine without poppler is a real
	// machine. An EPUB with the drawn cover is a book; an EPUB with no cover is
	// not, so the SVG stays as the fallback rather than being deleted.
	v := coverVolume()
	opf := packageOPF(v, nil, Options{}, false)
	if !strings.Contains(opf, `href="cover.svg"`) {
		t.Errorf("the manifest does not name cover.svg:\n%s", opf)
	}
	if strings.Contains(opf, "cover.png") {
		t.Errorf("the manifest names cover.png, which is not in the container")
	}
	if doc := coverDoc(v, false); !strings.Contains(doc, "cover.svg") {
		t.Errorf("the cover page does not show the drawn cover:\n%s", doc)
	}
}

func TestNoPDFToRasteriseIsNotAnError(t *testing.T) {
	// Three ways there is nothing to rasterise, and none of them is a reason to
	// refuse to write the EPUB. Nothing coming back means the caller should draw
	// it, and the audit says which of the two happened.
	for _, pdf := range []string{"", "/nonexistent/book.pdf"} {
		png, err := RasterCover(pdf)
		if err != nil {
			t.Errorf("RasterCover(%q) = %v, want no error", pdf, err)
		}
		if png != nil {
			t.Errorf("RasterCover(%q) returned %d bytes from nothing", pdf, len(png))
		}
	}
}
