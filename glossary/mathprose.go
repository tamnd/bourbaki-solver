package glossary

import (
	"strings"

	"github.com/tamnd/bourbaki-solver/mathtex"
)

// Prose set inside the mathematics.
//
// A translation copies the mathematics through byte for byte. That rule was
// written against Algebra, where a formula holds symbols and nothing else, and
// it is wrong for one part of one kind of formula: the words a printing sets
// inside a \text, which are prose in the language of the printing and are put
// in a formula because TeX has no other way of writing a word in one.
//
// Theory of Sets is where it stops being a detail. Bourbaki names the assembly
// signs by the ordinary words for them and then writes the words in the
// displays, so chapter I states its criteria as
//
//	$((\text{not } A) \text{ or } B) \Rightarrow ((\text{not not } A) \text{ or } B)$
//
// and the English printing is itself a translation: the French writes non, ou
// and et where this writes not, or and and, which is as plain a demonstration
// as there could be that these words belong to the language a printing is in.
// Copied through untouched they leave a Vietnamese chapter I in which every
// formula is half English, and 526 spans of the corpus carry a run like it.
//
// So the rule becomes: the mathematics is copied through byte for byte, except
// the words inside a \text, which are translated like any other prose. That is
// two rules rather than one and both are here, in one place, because three
// callers ask the question and three copies would drift: the translator that
// refuses an answer as it comes back, the L rules that read the committed
// corpus, and the report that says which sections are worth doing again.

// SameMath says whether two spans of mathematics differ anywhere a translation
// is not allowed to change them.
//
// It answers yes to a span whose words were translated and no to a span where
// anything else moved, including a word that was never prose. Which is which is
// decided by the English side: a run holding an English word is prose and may
// be rewritten, and a run holding none is a name the printing sets upright,
// Card or resp. or a quotation mark, and has to come back as it went.
//
// Byte for byte means byte for byte, and a formula that was only re-spaced is
// refused here like any other. That is not because the spacing matters to what
// the formula says, TeX sets $A/\mathfrak{m}$ and $A / \mathfrak{m}$ the same,
// but because a model that has touched the mathematics at all is a model to be
// suspicious of, and because the corpus is worth more when the English and the
// Vietnamese hold the same bytes between the dollars. translate.Respace is
// where a re-spaced answer is put right rather than thrown away.
func SameMath(en, tr string) bool {
	if mathtex.MaskText(en) != mathtex.MaskText(tr) {
		return false
	}
	// The masks are equal, so the runs stand in the same places and there are
	// as many of one as of the other.
	e, t := mathtex.TextRuns(en), mathtex.TextRuns(tr)
	for i := range e {
		if EnglishWords(e[i].Text) == 0 && t[i].Text != e[i].Text {
			return false
		}
	}
	return true
}

// UntranslatedMathProse is every run of prose inside these spans that came back
// in English, written as it stands in the English.
//
// The test is that no word of the list is left in the answer, rather than that
// the answer is written in the script of the language. A Vietnamese word may
// carry no diacritic at all, cho for for and la for is, so asking for the
// script would refuse a translation that is perfectly good; asking that the
// English is gone refuses exactly the run that was copied through.
//
// A run the answer empties counts as untranslated as well. Nothing else would
// notice: the mask compares the braces and not what is between them, so a word
// deleted rather than translated leaves a formula that sets and says less than
// the book does.
func UntranslatedMathProse(en, tr string) []string {
	e, t := mathtex.TextRuns(en), mathtex.TextRuns(tr)
	if len(e) != len(t) {
		return nil // the spans are not the same shape, and SameMath says so
	}
	var out []string
	for i := range e {
		if EnglishWords(e[i].Text) == 0 {
			continue
		}
		if strings.TrimSpace(t[i].Text) == "" || EnglishWords(t[i].Text) > 0 {
			out = append(out, e[i].Macro+"{"+e[i].Text+"}")
		}
	}
	return out
}
