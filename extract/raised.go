package extract

import (
	"strings"
	"unicode/utf8"
)

// A script is not always drawn before the symbol it is written over.
//
// overset reads the inverse image, where TeX draws the -1 first and the symbol
// across the middle of it, and the half of the script standing in front of the
// symbol is the whole of what tells it from an exponent. Topologie algébrique
// prints another one. The composite of an entourage with itself is written with
// the 2 over the letter, and page 289 writes it over a letter that carries a
// prime: the page draws V, then the 2, then the prime, and read left to right
// that is V^2', which is a second superscript on one base and KaTeX refuses it
// by name. Four of the five M09 findings left in the born-digital volumes are
// that one line of that one page.
//
// Nothing is drawn before the symbol here, so overset cannot ask its question.
// What the page says instead is how high the 2 is set. TeX sets a superscript
// inside the band of its base, so the prime of that line starts at the top of
// the V it hangs off, to the unit; the 2 starts nine units above it, which is
// no band a superscript is set in. And it lies inside what the V and its prime
// span rather than after them: the V measures 518 to 530 and the prime 531 to
// 534, and the 2 spans 523 to 529, centred on the pair. An exponent begins
// where its base ends and is never drawn back across it.
//
// So the question asked is whether the script stands clear above the base and
// inside what the base and what hangs off it span, and both halves of it are
// measurements off the page.

// raised writes back a script the page draws over a symbol after drawing the
// symbol itself.
func raised(toks []token) []token {
	out := make([]token, 0, len(toks))
	for i := 0; i < len(toks); i++ {
		script, tail, end := above(toks, i)
		if end == 0 {
			out = append(out, toks[i])
			continue
		}
		out = append(out, stackAfter(toks[i], script, tail))
		i = end - 1
	}
	return out
}

// above gathers the script drawn over the symbol at i and whatever else hangs
// off that symbol, and says where the whole of it ends. It gives an end of 0
// where the tokens after i are not such a script.
func above(toks []token, i int) (script, tail []token, end int) {
	b := toks[i]
	// A large operator carries limits, which hoist has already put in front of
	// it, and a symbol of no width is one of the arrows of a diagram rather than
	// a thing a script can be drawn across. Both are the reasons laid gives.
	if b.sign || b.right <= b.left {
		return nil, nil, 0
	}
	// The symbol is one symbol, on the line, and the two go together.
	//
	// A run of prose is not a thing a script is drawn over, and it is wide
	// enough that anything printed above the line lands inside it: the box of
	// "Soit A un anneau et soient E et F des A-modules" spans most of the page,
	// and the f of End^f gathered on to that line from the line above it read as
	// a script written over the sentence. Six lines of the two Algebra printings
	// came back with a paragraph inside an \overset before this was asked.
	//
	// A symbol that is itself a script is the interleaved cluster restack reads,
	// and reading it here instead gets it wrong: p^{i_1}_1 draws the 1 of the
	// exponent above the 1 of the index and across it, which is the shape this
	// asks for, and it is an exponent carrying an index rather than anything
	// written over anything.
	if b.depth != 0 || !symbol(b.text) {
		return nil, nil, 0
	}
	d := b.depth + 1
	j := i + 1
	for j < len(toks) && toks[j].depth == d && toks[j].level == Sup && clear(toks[j], b) {
		script = append(script, toks[j])
		j++
	}
	if len(script) == 0 {
		return nil, nil, 0
	}
	// The pieces of the script touch each other, the same way the two halves of
	// an inverse do, since they were one thing before the page cut them into
	// runs.
	for n := 1; n < len(script); n++ {
		if script[n].left-script[n-1].right > restackGap {
			return nil, nil, 0
		}
	}
	// What follows is the prime, the index, whatever the symbol carries, and it
	// goes under the script with the symbol because the page drew the script
	// across the pair. A prime is taken at the depth of the symbol as well as at
	// the depth of a script, since a prime sits on the line of what it marks and
	// text.go has already dropped it there.
	//
	// A script of a script is refused rather than flattened, which is the reason
	// laid gives: nothing in the volumes writes one under a script drawn over a
	// symbol, and rebuilding one would mean guessing at where its braces close.
	end = j
	for end < len(toks) {
		t := toks[end]
		if t.depth > d {
			return nil, nil, 0
		}
		if !(t.depth == d && !clear(t, b)) && !(t.depth == b.depth && tick(t)) {
			break
		}
		tail = append(tail, t)
		end++
	}
	// A raised piece beyond the tail is the far half of a script the symbol was
	// drawn through, which is the inverse image overset reads and not this at
	// all. The alpha of Topologie algébrique page 34 prints its inverse image as
	// alpha, minus, the index f, one; taking the minus alone as a script written
	// over the alpha leaves the one of the -1 standing outside it, and the page
	// came back saying \overset{-}{\alpha f}^1 where it had said \alpha^{-1}_f.
	if end < len(toks) && toks[end].depth == d && clear(toks[end], b) {
		return nil, nil, 0
	}
	// The script lies inside what the symbol itself spans, because it is drawn
	// across it. This is the question laid asks the other way round, and it is
	// the whole of what leaves an ordinary exponent alone: TeX sets an exponent
	// after its base and never draws it back over the base. Page 289 measures
	// the V at 518 to 530 and its 2 at 523 to 529, and page 481 of Algebra VIII
	// measures theta at 468 to 475 and its exponent at 475 to 490.
	if !within([2]int{script[0].left, script[len(script)-1].right}, [2]int{b.left, b.right}) {
		return nil, nil, 0
	}
	return script, tail, end
}

// symbol reports whether the text of a token is one symbol: a single character,
// or one control word out of the mathematics fonts. An arrow is a symbol and so
// is a letter, and a run of words is not.
func symbol(s string) bool {
	s = strings.TrimSpace(s)
	if s == "" {
		return false
	}
	if utf8.RuneCountInString(s) == 1 {
		return true
	}
	return s[0] == '\\' && word(s[1:])
}

// clear reports whether a token is drawn clear above the band its base is set
// in, which is to say that more than half of its box stands above the top of
// the base.
//
// A superscript is raised inside the band of its base and clears the top of it
// by an ascender at most: the i of x^i on page 309 of the French chapter VIII
// stands 2 units above the x against a box 8 units deep, and the 1 of a K
// theory exponent measures the same. A script written over the symbol is lifted
// off it altogether: the 2 of the composed entourage stands 9 above the V
// against a box of 11, and the f written over the arrow of page 126 stands 5
// above it against a box of 9. Half is where the two stop overlapping, and
// nothing in the six volumes is measured between 0.3 and 0.55 of its own depth.
func clear(t, b token) bool { return 2*(b.top-t.top) > t.bottom-t.top }

// stackAfter writes the symbol with the script over it. Everything the symbol
// carries goes under the script with it, since the page drew the script across
// the pair.
func stackAfter(b token, script, tail []token) token {
	var s, body strings.Builder
	for _, t := range script {
		abut(&s, t.text)
	}
	abut(&body, b.text)
	// What the symbol carries keeps the level the page set it at. The entourage
	// of page 288 is written V sub y with the 2 over the pair, and writing the
	// index beside the letter rather than under it says Vy, which is two letters
	// where the volume prints one.
	for i := 0; i < len(tail); {
		j := i
		for j < len(tail) && tail[j].level == tail[i].level {
			j++
		}
		var group strings.Builder
		for _, t := range tail[i:j] {
			abut(&group, t.text)
		}
		switch tail[i].level {
		case Sup, Sub:
			body.WriteString(mark(tail[i].level) + "{" + group.String() + "}")
		default:
			abut(&body, group.String())
		}
		i = j
	}
	b.text = `\overset{` + s.String() + `}{` + body.String() + `}`
	b.class, b.math = ClassMath, true
	for _, t := range append(append([]token(nil), script...), tail...) {
		b.left, b.right = min(b.left, t.left), max(b.right, t.right)
	}
	return b
}
