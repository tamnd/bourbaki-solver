package extract

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/mathtex"
)

// A page whose text layer flattened a matrix is flagged, and the flag is what
// render -flagged and ocr fill -flagged read to send the page to the model.
func TestAPageWithAStackedMatrixIsFlagged(t *testing.T) {
	p := &Page{Body: `Soit A $= (^{a b}_{c d})$ un élément.`}
	if mathtex.StackedMatrices(p.Body) == 0 {
		t.Fatal("the body was not read as carrying a matrix")
	}
	p.flag(FlagStackedMatrix)
	if !p.Flagged() {
		t.Error("the page was not flagged")
	}
}
