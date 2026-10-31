package main

import (
	"testing"

	"github.com/tamnd/bourbaki-solver/translate"
)

// A draft that moves a formula keeps it; a draft that changes one does not.
func TestReorderedFormulasAreNotRefused(t *testing.T) {
	src := "Let $x \\in E$ and the map $f$.\n"
	moved := "映射 $f$ 和 $x \\in E$。\n"
	if bad := hard(reordered(src, moved, translate.Audit("zh", src, moved))); len(bad) > 0 {
		t.Errorf("a reordered draft was refused: %v", bad)
	}
	changed := "映射 $g$ 和 $x \\in E$。\n"
	if bad := hard(reordered(src, changed, translate.Audit("zh", src, changed))); len(bad) == 0 {
		t.Error("a draft that changed a formula was accepted")
	}
}
