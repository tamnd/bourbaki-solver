package roundtrip

import (
	"fmt"
	"strings"

	"github.com/tamnd/bourbaki-solver/translate"
)

// JudgeChars is how much text goes in one question to the judge, counting both
// passages.
//
// The whole file will not fit. Measured over the corpus, the largest English
// section is the historical note of Theory of Sets IV at 158,762 characters and
// its Vietnamese is 207,694, and a judge asked for both at once is being asked
// for a third of a million characters. So the comparison is cut, and where it is
// cut matters more than how big the pieces are.
const JudgeChars = 20000

// A Segment is one question to the judge: a run of blocks of the English and
// the same run of what came back.
type Segment struct {
	Index, Of int
	English   string
	Back      string
}

// Segments pairs the English with the English that came back, block for block,
// and groups the pairs into questions.
//
// Block for block and not by cutting each text to a length. The two texts are
// different lengths and always will be, so any cut made by counting characters
// puts the end of one paragraph beside the beginning of another and the judge
// reports every seam as an omission. Blocks are the unit the whole translation
// path already agrees on: the chunker cuts on them, the audit counts them, and
// a translation whose block count differs from its English is refused before it
// is written. So block i of the English, block i of the translation and block i
// of the return trip are the same paragraph three times.
//
// A back translation whose block count differs from the English breaks that
// correspondence and there is nothing sound to do with it here. It is an error
// rather than a best effort: the model was asked for one paragraph per paragraph
// and did something else, and judging the misaligned text would produce a page
// of findings about paragraphs that were never compared with their own
// counterparts. The caller reports the file as unjudged and says why, which is
// an honest gap, where a page of invented differences is not.
func Segments(english, back string, budget int) ([]Segment, error) {
	if budget <= 0 {
		budget = JudgeChars
	}
	eb, bb := trimmed(translate.Blocks(english)), trimmed(translate.Blocks(back))
	if len(eb) != len(bb) {
		return nil, fmt.Errorf("the English has %d blocks and what came back has %d, so the paragraphs cannot be paired",
			len(eb), len(bb))
	}
	if len(eb) == 0 {
		return nil, fmt.Errorf("there is nothing to compare")
	}
	var out []Segment
	var e, b []string
	n := 0
	flush := func() {
		if len(e) == 0 {
			return
		}
		out = append(out, Segment{English: strings.Join(e, "\n\n"), Back: strings.Join(b, "\n\n")})
		e, b, n = nil, nil, 0
	}
	for i := range eb {
		size := len(eb[i]) + len(bb[i])
		// A single pair over the budget goes on its own, the way the chunker
		// lets a single long block go over on its own. Splitting inside a
		// paragraph to meet a number would give back the alignment this whole
		// function is for.
		if len(e) > 0 && n+size > budget {
			flush()
		}
		e, b = append(e, eb[i]), append(b, bb[i])
		n += size
	}
	flush()
	for i := range out {
		out[i].Index, out[i].Of = i+1, len(out)
	}
	return out, nil
}

// trimmed takes the blank line off the end of a block.
//
// The splitter normalises the body before it cuts, which leaves the last block
// carrying the file's trailing newline. That newline is nothing to a reader and
// it is a paragraph break to anything that joins blocks back together, so it
// comes off before the two sides are paired.
func trimmed(bs []string) []string {
	out := make([]string, 0, len(bs))
	for _, b := range bs {
		out = append(out, strings.TrimSpace(b))
	}
	return out
}
