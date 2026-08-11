package pdfglyph

import "sort"

// Comparing the two readings of a volume is not as simple as it looks, and the
// first pass at it called the rewrite broken on a page where the rewrite was
// right.
//
// Poppler does two different things with a glyph name it cannot resolve. Most
// of the time it prints nothing and the run arrives empty, which is what the
// whole package is about. Some of the time it falls back on the code the glyph
// sits at and prints the character that code stands for, and in these printings
// the code means nothing at all: the typesetter subset every font and packed
// the glyphs it kept at codes 2, 3, 4 and up. So page 442 of Algèbre chapitre 8
// carries a wide hat drawn as an exclamation mark, which reads as a sentence
// ending in "π(λ . . . !" and is nobody's idea of a flag.
//
// So a character is allowed to change as well as to appear, and the two have to
// be told apart and counted separately rather than one of them being called a
// failure. What is not allowed is a character going missing.

// Change is a character that read one way before the rewrite and another way
// after it.
type Change struct{ Old, New rune }

// Diff is what the two readings of one page have to say about each other.
type Diff struct {
	// Kept is characters that read the same either way.
	Kept int
	// Added is characters the rewrite recovered, counted by character.
	Added map[rune]int
	// Changed is characters that read as something else before, which is
	// poppler having fallen back on a meaningless code.
	Changed map[Change]int
	// Lost is characters the volume had and the prepared copy does not. Any of
	// these is a bug in the rewrite.
	Lost map[rune]int
	// Hard is set when the two readings are too far apart to align, which is a
	// bug in the rewrite of a different and larger kind.
	Hard bool
}

func newDiff() Diff {
	return Diff{Added: map[rune]int{}, Changed: map[Change]int{}, Lost: map[rune]int{}}
}

// Add folds one page's diff into a running total.
func (d *Diff) Add(o Diff) {
	d.Kept += o.Kept
	d.Hard = d.Hard || o.Hard
	for k, v := range o.Added {
		d.Added[k] += v
	}
	for k, v := range o.Changed {
		d.Changed[k] += v
	}
	for k, v := range o.Lost {
		d.Lost[k] += v
	}
}

// Total counts a map of characters.
func Total[K comparable](m map[K]int) int {
	n := 0
	for _, v := range m {
		n += v
	}
	return n
}

// ByCount lists the keys of a count, most frequent first, ties broken so that
// two runs of the same volume report the same thing.
func ByCount[K comparable](m map[K]int, less func(a, b K) bool) []K {
	keys := make([]K, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if m[keys[i]] != m[keys[j]] {
			return m[keys[i]] > m[keys[j]]
		}
		return less(keys[i], keys[j])
	})
	return keys
}

// maxEdits is where aligning two readings of a page is given up on. A page of
// this series runs to about two thousand characters and the rewrite recovers a
// few dozen of them, so a page needing two hundred edits is not a page the
// rewrite improved.
const maxEdits = 200

// Compare aligns two readings of one page and says what the second did to the
// first.
func Compare(old, now []rune) Diff {
	d := newDiff()
	script, ok := diff(old, now)
	if !ok {
		d.Hard = true
		return d
	}
	for i := 0; i < len(script); {
		if script[i].op == keep {
			d.Kept++
			i++
			continue
		}
		// A run of edits is one place the two readings part company. Inside it
		// the deletions and the insertions are paired off in order, because a
		// glyph that read as one character and now reads as another arrives as
		// one of each in the same place.
		var del, ins []rune
		for ; i < len(script) && script[i].op != keep; i++ {
			if script[i].op == remove {
				del = append(del, script[i].r)
			} else {
				ins = append(ins, script[i].r)
			}
		}
		n := min(len(del), len(ins))
		for j := 0; j < n; j++ {
			d.Changed[Change{del[j], ins[j]}]++
		}
		for _, r := range ins[n:] {
			d.Added[r]++
		}
		for _, r := range del[n:] {
			d.Lost[r]++
		}
	}
	return d
}

type op byte

const (
	keep   op = '='
	remove op = '-'
	insert op = '+'
)

type edit struct {
	op op
	r  rune
}

// diff is the Myers difference of two runs of characters. It is the greedy
// version with the whole trace kept, which is the short one to write and is
// fast here because the two readings are nearly the same: the work it does is
// proportional to how many characters differ, and on a page of two thousand
// that is a few dozen.
func diff(a, b []rune) ([]edit, bool) {
	trace, ok := trace(a, b)
	if !ok {
		return nil, false
	}
	n, m := len(a), len(b)
	off := edgeOffset(n, m)
	x, y := n, m
	var out []edit
	for d := len(trace) - 1; d >= 0; d-- {
		v := trace[d]
		k := x - y
		var prevK int
		if k == -d || (k != d && v[off+k-1] < v[off+k+1]) {
			prevK = k + 1
		} else {
			prevK = k - 1
		}
		prevX := v[off+prevK]
		prevY := prevX - prevK
		for x > prevX && y > prevY {
			out = append(out, edit{keep, a[x-1]})
			x, y = x-1, y-1
		}
		if d == 0 {
			break
		}
		if x == prevX {
			out = append(out, edit{insert, b[y-1]})
		} else {
			out = append(out, edit{remove, a[x-1]})
		}
		x, y = prevX, prevY
	}
	for i, j := 0, len(out)-1; i < j; i, j = i+1, j-1 {
		out[i], out[j] = out[j], out[i]
	}
	return out, true
}

// trace is one row of the Myers edit graph per number of edits, kept so the
// path can be walked back afterwards.
func trace(a, b []rune) ([][]int, bool) {
	n, m := len(a), len(b)
	off := edgeOffset(n, m)
	v := make([]int, 2*off+1)
	var out [][]int
	limit := min(n+m, maxEdits)
	for d := 0; d <= limit; d++ {
		out = append(out, append([]int(nil), v...))
		for k := -d; k <= d; k += 2 {
			var x int
			if k == -d || (k != d && v[off+k-1] < v[off+k+1]) {
				x = v[off+k+1]
			} else {
				x = v[off+k-1] + 1
			}
			y := x - k
			for x < n && y < m && a[x] == b[y] {
				x, y = x+1, y+1
			}
			v[off+k] = x
			if x >= n && y >= m {
				return out, true
			}
		}
	}
	return nil, false
}

// edgeOffset is where the diagonal numbered zero sits in the row of furthest
// reaching paths. It is one wider than the longest edit script can be, since
// the row is read one diagonal to either side of the one being worked on and a
// blank page is two readings of nothing at all.
func edgeOffset(n, m int) int { return n + m + 1 }
