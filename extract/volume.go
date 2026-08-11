package extract

import (
	"sort"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// Volume is what reading one page needs to know about the volume around it.
// Both of its fields are measurements over the whole book, and neither can be
// worked out from the page in front of the reader.
type Volume struct {
	// Compounds are the words the volume writes with a hyphen inside them, so
	// that a hyphen at the end of a line can be told from a word broken across
	// it. See compound.go.
	Compounds Compounds
	// HeadBand is how far down the page the running head can be.
	HeadBand int
	// BodySize is the size the volume sets its text in. It is what a heading
	// is measured against: the printings do not agree on a size either, and a
	// heading is large or small relative to the page around it. Zero means the
	// volume was not measured, and heading falls back on the English size.
	BodySize int
}

// defaultHeadBand is where the band sits when the volume will not say. The 2023
// English printing sets its head at 56 and the first line of its body at 87, so
// anything between the two does, and 70 is what the reading of that volume was
// audited with.
const defaultHeadBand = 70

// Measure reads the volume for the head band.
//
// The band cannot be a constant, because the printings do not agree on it. The
// 2023 English printing sets the head at 56 and the body at 87. The 2012 French
// printing of the same chapter sets the head at 80 on a right hand page and 85
// on a left hand one, and opens the body at 125 to 131, so a band of 70 leaves
// every head of the volume standing at the top of the body, and the first
// paragraph of page 64 came out as "$N_o3$ QUELQUES OPÉRATIONS SUR LES MODULES
// A VIII.53".
//
// What the two printings do agree on is the shape: the head is the first line
// of the page, it is set well above the body, and it is high up. So the band is
// taken from the volume itself. Every page whose first line is detached from
// the second by more than the leading of the page and sits in the top quarter
// of it offers its two tops, and the band is put halfway between the middle of
// the heads and the middle of the bodies under them.
//
// A volume that does not print a running head has nothing here to measure and
// is left at the default, which is above every body this corpus has and strips
// nothing.
//
// Measured: 105 on the French chapter VIII, where the heads are at 80 and 85
// and the bodies at 125 to 131; 71 on the English one.
func Measure(l *pdfsrc.Layout) Volume {
	var heads, bodies []int
	sizes := map[int]int{}
	pages := 0
	for _, p := range l.Pages {
		lines, _ := LinesColumns(l, p)
		for _, ln := range lines {
			sizes[size(ln)] += len(ln.Runs)
		}
		if len(lines) < 4 {
			continue
		}
		pages++
		// A heading is not a head. A chapter opens on its title, which is set
		// with air under it exactly as a running head is, and page 12 of the
		// French chapter sets that title at 122, which is above the body of
		// every other page of the volume.
		if _, ok := heading(lines[0], 0); ok {
			continue
		}
		if lines[1].Top-lines[0].Top <= leading(lines)*3/2 {
			continue
		}
		if lines[0].Top*4 > p.Height {
			continue
		}
		heads = append(heads, lines[0].Top)
		bodies = append(bodies, lines[1].Top)
	}
	// Half the pages is the test that the volume runs a head at all. A page
	// that opens on a display, or on the last line of a paragraph with air
	// under it, has the shape of a head and is not one, and there are never
	// many of those.
	v := Volume{HeadBand: defaultHeadBand, BodySize: commonest(sizes)}
	if pages == 0 || len(heads)*2 < pages {
		return v
	}
	h, b := middle(heads), middle(bodies)
	if h+10 >= b {
		return v
	}
	v.HeadBand = (h + b) / 2
	return v
}

// commonest is the size most of the volume is set in, counted by run so that a
// page of small type does not weigh as much as a page of text. Measured: 15 on
// the English chapter VIII and 14 on the French one, which is the whole reason
// a heading cannot be told by a constant.
func commonest(sizes map[int]int) int {
	best, n := 0, 0
	for size, runs := range sizes {
		if runs > n || runs == n && size < best {
			best, n = size, runs
		}
	}
	return best
}

// middle is the median of a list, which is what is wanted everywhere here: a
// volume prints its head at two heights, one for each side of the spread, and
// the odd page that is not a head at all must not move the answer.
func middle(v []int) int {
	s := append([]int(nil), v...)
	sort.Ints(s)
	return s[len(s)/2]
}
