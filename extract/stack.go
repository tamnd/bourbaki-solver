package extract

import (
	"strings"
	"unicode"
)

// A text layer has no rows in it. TeX sets a superscript and a subscript at the
// same place on the line, one over the other, and pdftohtml reports the runs of
// a page left to right, so a stack comes back interleaved. The -1 of an inverse
// is set over the E of \theta_E, its minus starting a hair to the left of the E
// and its one ending a hair to the right, and the three runs arrive as minus,
// E, one. Written in that order they are ^-_E^1, which is not a formula: TeX
// gives a base one superscript and one subscript and refuses the second by
// name, Double superscript. The audit counts them in M09 and there were 189 in
// the Markdown when this was written.
//
// The pieces can be put back, because the page says which ones belong together.
// The two halves of the -1 touch, since they were one thing before the E was
// laid across them, and the E lies inside what they span, since it is what they
// were laid across. Neither is true of a matrix flattened the same way: the X
// and the 0 of a first row stand five pixels apart, which is the gap the page
// leaves between two columns, and the rows offset each other rather than one
// lying inside the other.

// restackGap is how far two pieces of one script may stand apart and still be
// read as one thing cut in two, in the pixel units of the page.
//
// Chapter VIII measures 0 or 1 across every inverse it prints, 2 at the widest
// where a piece of the script carries a script of its own, and 4 at the
// narrowest column gap of a matrix. Three would do as well and two is taken
// because nothing on the page needs it.
const restackGap = 2

// hoistGap is how far the limit of an operator may stop short of the sign, and
// how wide a break it may carry inside itself, in the pixel units of the page.
//
// A limit is centred on its sign, so the two meet by construction and the gap
// is only there for the rounding pdftohtml does on a box. Chapter VIII measures
// 3 at the widest, where the x of a product over x in H\G stops three units
// short of the sign, and 8 is taken because that is the reach text.go already
// uses for two pieces of one word.
const hoistGap = 8

// overhang is how far one script may stand outside what the other spans and
// still be read as lying across it, in the pixel units of the page.
//
// The two scripts of a stack are set at one size but out of different glyphs,
// and a glyph box is not the ink inside it. \varepsilon_M^{-1} of exercise 19
// of § 1 is the whole of the trouble: M is a wide letter and 1 is a narrow one,
// so the index starts five units inside the exponent laid across it and ends
// two units past the end of it, and strict containment refused a stack that is
// plainly a stack.
//
// Chapter VIII measures 2 at the widest across every cluster of both printings,
// and at 3 nothing more is taken, so 2 is a measurement rather than a licence.
// It is well inside the 4 of the narrowest matrix column, which is the gap
// touching already refuses on, so a flattened matrix comes back flattened.
//
// Three clusters a printing turn on this, and one of the three is not a stack
// at all: the derivation of exercise 23 of § 1 is the fraction \partial P over
// \partial T_i, and TeX sets the two halves of an inline fraction at the size
// and the height a superscript and a subscript are set at, so nothing in a text
// layer tells them apart. The rule of the fraction is a drawn path and there is
// no reading of the runs that would find it. That page is repaired by hand in
// both printings and carries manual: true.
const overhang = 2

// hoist puts a large operator in front of the limit written across it.
//
// TeX centres the limit of an operator on the sign, so the first half of the
// limit is drawn to the left of it, and pdftohtml hands back a page in the
// order it draws it. Reading a line left to right therefore writes half the
// limit before the operator it belongs to, and the product of English page 320
// came out as "_{x\in}\prod_{H\backslash G}": every piece is there, in the order
// the page draws them, and none of it is where TeX wants it.
//
// A limit is told from an index by the band of the line. TeX sets an index
// against the term it hangs off, close enough that the two share a band, and it
// sets a limit far enough out that its box clears the band altogether, which is
// why a limit arrives as a line of its own and gather has to put it back at all.
// So the walk to the left stops at the first token that is on the band, and
// A_1\prod keeps its index while \prod_{x\in H\backslash G} gives up its limit.
//
// A token drawn across the sign is a limit whatever the band says, since a limit
// is centred on its sign and nothing else on a line is written across one. That
// is the reading for a sum set in the middle of a sentence, where the sign is
// small and the limit under it stands no lower than an index does, and it is
// what a band measured a unit or two wide of the type asks for as well: the
// symbol font of Lie 7 to 9 reports every ∈ 19 units high in a line whose roman
// is 13, so a line of that volume with an ∈ in it is 19 units tall and the
// limit of a sum on it reads as touching the band. Page 179 gives "x
// $\in_{\alpha}\sum_{\in B}\mathfrak{g}^{\alpha}$" three times over, with the
// sign standing in the middle of the limit it takes.
//
// The walk also stops where the tokens stop touching, since a limit is one
// thing and what stands beyond a break in it is something else the line was
// carrying at that height, and where it reaches material that is centred on
// another sign.
//
// Moving the sign out leaves the two halves of the limit side by side, which is
// what restack then reads as a single cluster.
func hoist(toks []token, top, bottom int) []token {
	var signs []token
	for _, t := range toks {
		if t.sign {
			signs = append(signs, t)
		}
	}
	if len(signs) == 0 {
		return toks
	}
	out := append([]token(nil), toks...)
	for i := range out {
		s := out[i]
		if !s.sign {
			continue
		}
		j, left := i, s.left
		edge := bound(out, i, s, signs)
		for j > 0 {
			t := out[j-1]
			if t.depth == 0 ||
				t.bottom > top && t.top < bottom && !astride(t, s) && t.left+overhang < edge {
				break
			}
			if t.right < left-hoistGap || !nearest(t, s, signs) {
				break
			}
			j, left = j-1, min(left, t.left)
		}
		if j == i {
			continue
		}
		// The sign now stands where the limit started, so it takes the reach the
		// limit had. What is on either side of a formula measures its distance
		// from the token at the end of it, and the sign of English page 320 is
		// drawn fourteen units past the equals sign that introduces it with the
		// x of its limit four units along. Left with its own edge the sign looked
		// a word away, the equals sign fell out of the formula, and the display
		// came back as "= $\prod...$".
		s.left = left
		copy(out[j+1:i+1], out[j:i])
		out[j] = s
	}
	return out
}

// astride reports whether a token is written across a sign rather than beside
// it, which is where a limit is and where an index of the term before the sign
// is not.
func astride(t, s token) bool { return lap(t, s) > 0 }

// lap is how much of a token is drawn over a sign.
func lap(t, s token) int { return min(t.right, s.right) - max(t.left, s.left) }

// bound is how far to the left of a sign its limit may reach, which is as far
// as the limit reaches to the right of it.
//
// TeX centres a limit on its sign, so the half drawn to the left of the sign
// and the half drawn to the right are of one width, and the right half is
// handed back whole because nothing was written across it. It therefore
// measures the left half, which was cut off from its sign and is what the walk
// is looking for.
//
// This is what a limit wider than the sign it is written under asks for. The
// walk stops at a token that reads as touching the band, and the tokens at the
// far end of such a limit are neither on the band by any margin nor written
// across the sign, so the band is all there is to go on and the band of this
// printing is a unit or two wide of the type. Page 90 of Lie 7 to 9 sets a sum
// over -q <= j <= p in the middle of a sentence: the minus is drawn from a
// symbol font at size 10 and reported 14 units high, which laps 2 units into a
// line whose roman ends at 429, and it was left standing in front of the sign
// as an index of nothing.
//
// The measurement is close rather than exact. That sign spans 281 to 297 and
// its limit ends at 312, so the limit begins at 266, and 266 is where the minus
// begins. The same sentence is set again nine lines down and comes out a unit
// wide, since a glyph box is not the ink inside it and the minus and the p at
// the two ends of the limit are boxed differently, so the edge is allowed the
// overhang the two scripts of a stack are allowed and for the same reason.
// Nothing is allowed past that: an index of the term before a sign lies
// further out than the limit does, since the limit fills the space between the
// two, and the walk reaches the index only by first crossing the whole limit
// and so only from beyond this edge.
//
// The scan to the right stops where the walk to the left does, and for the same
// reasons. A double sum sets the two signs a limit's width apart, so the bound
// of the second stands within a few units of the first and would measure it a
// half wider than it is; the |G|^{-1} in front of the double sum of French page
// 399 is 13 units clear of the sign, and the edge that came of reading the
// lambda of the second sum into the first reached back over the 1 and took it.
// Asking that the right half answer to this sign and not to another is what
// nearest already does for the left.
//
// The mirror holds only where the limit is written under the sign, so that is
// asked before anything is measured. A large operator in the middle of a
// sentence is set in the text style, where TeX puts the limits beside the sign
// rather than beneath it, and there is no left half to go and find. Page 444
// sets Card(G)^{-1} in front of such a sum: the sign spans 149 to 164, its
// limit C in C-script follows at 164 to 188, and mirroring that onto a sign
// nothing is drawn across puts the edge at 125, which is 24 units clear on the
// far side and reaches back over the -1 and takes it. What the test asks for
// is a script drawn over the sign, and it asks for more than a unit or two of
// one. The two sums of page 87 stand a limit's width apart and the w that
// opens the limit of the second begins at 142 where that sign ends at 143,
// which is the glyph box slack the edge itself is allowed rather than anything
// written across the sign, and reading it as a limit gave the first sum an
// edge that took the S-script out of its own.
func bound(toks []token, i int, s token, signs []token) int {
	under := false
	for _, t := range toks {
		if t.depth > 0 && lap(t, s) > overhang && nearest(t, s, signs) {
			under = true
			break
		}
	}
	if !under {
		return s.right
	}
	right := s.right
	for j := i + 1; j < len(toks); j++ {
		t := toks[j]
		if t.depth == 0 || t.left > right+hoistGap || !nearest(t, s, signs) {
			break
		}
		right = max(right, t.right)
	}
	return s.left + s.right - right
}

// nearest reports whether a sign is the one a token is centred on.
//
// A limit is centred on its sign, so where a line sets two signs the middle of
// the token says which of them it answers to. The signs of a double sum stand a
// limit's width apart and the bound of the first is written between them, so
// walking left from the second reaches it before it reaches anything that stops
// the walk. French page 399 sets a sum over G against a sum over the dual of G
// twice, and both times the bound of the first went behind both signs and the
// page said "\sum\sum_{g\in G\lambda\in\widehat{G}}".
//
// The measure is between midpoints, so the distances are doubled, which they
// are on both sides and so says nothing.
func nearest(t, s token, signs []token) bool {
	d := gap(t, s)
	for _, o := range signs {
		if gap(t, o) < d {
			return false
		}
	}
	return true
}

// gap is how far two tokens stand from each other, midpoint to midpoint and
// doubled.
func gap(a, b token) int {
	d := a.left + a.right - b.left - b.right
	if d < 0 {
		return -d
	}
	return d
}

// overset puts back the script Bourbaki draws over a symbol rather than after
// it.
//
// The inverse image of a set is written with the -1 centred above the symbol,
// and Topologie algébrique and Théories spectrales write it on nearly every
// page: 444 of them across the two, against 2 in Algebra VIII. TeX draws the -1
// first and the symbol across the middle of it, so the page hands back minus,
// letter, one, and reading that left to right gives ^-p^1, where the page
// printed the inverse image of p. Some of those are legal TeX and set as
// nonsense; the rest are a second superscript on the same base and KaTeX
// refuses them by name.
//
// restack cannot see it. It reads a cluster of scripts and this is not one: the
// letter in the middle is on the line, at no level and at no depth, and restack
// cuts the token list at every token that is.
//
// Where restack does see it, it reads it wrong and says nothing, which is worse.
// Page 318 of Topologie algébrique writes the inverse image of o sub B, and the
// -1 is wide enough to reach past the o, so the page draws the o first and then
// the minus, the B and the one. Those last three are a cluster, they interleave,
// and restack put them together as o^{-1}_B, which is good TeX for the inverse
// of a function where the page printed the inverse image of a set.
//
// The two are told apart by where the script is centred, which is the only
// thing that ever told them apart on paper either. An exponent is set after the
// symbol and begins where the symbol ends. A script drawn over the symbol is
// centred on it and spans the whole of it, the index included: page 318
// measures o at 517 to 525 and its B at 525 to 534, and the minus and the one
// span 518 to 534, which covers both. Page 481 of Algebra VIII measures theta
// at 468 to 475 and its E at 475 to 483, and its minus and one span 475 to 490,
// which covers neither. So the question asked is whether the symbol lies inside
// what the script spans, and the answer is a measurement rather than a guess.
//
// The pieces of the script have to touch each other across the break the symbol
// made in it, since they were one piece before it was drawn through them. That
// is what refuses a^2b^3, where the b lies between two superscripts and inside
// what they span and they stand a letter's width apart.
//
// A piece of the script has to be drawn across the symbol, give or take the
// overhang one glyph box carries past another, which is what refuses the
// exponent of the letter before. Page 38 sets the inverse image of f prime, and
// f is a narrow box with a wide reach: the minus lies over the whole of it and
// the one begins a single unit past its right edge, so the overhang is asked for
// here for the same reason within asks for it.
func overset(toks []token) []token {
	out := make([]token, 0, len(toks))
	for i := 0; i < len(toks); i++ {
		script, under, after, at, end := laid(toks, i)
		if end == 0 {
			out = append(out, toks[i])
			continue
		}
		out = append(out, drawn(script, toks[at], under, after))
		i = end - 1
	}
	return out
}

// laid gathers the script drawn over the symbol that begins at i, the index the
// symbol carries under it, where the symbol itself is and where the whole of it
// ends. It gives an end of 0 where the tokens at i are not such a script.
func laid(toks []token, i int) (script, under, after []token, at, end int) {
	// Part of the script has to be drawn before the symbol. Nothing else on a
	// page is: an exponent is set after the base it belongs to and a text layer
	// reports a page in the order it is drawn, so a superscript standing in
	// front of a symbol on the line is a superscript that was cut in half by
	// something laid through it. That is the whole of what this reads, and it is
	// a fact about the drawing rather than a threshold. See the head of the file
	// for the shape it therefore leaves alone.
	d := toks[i].depth
	if d == 0 || toks[i].level != Sup {
		return nil, nil, nil, 0, 0
	}
	at = i
	for at < len(toks) && toks[at].depth == d && toks[at].level == Sup {
		at++
	}
	if at >= len(toks) {
		return nil, nil, nil, 0, 0
	}
	b := toks[at]
	// The symbol is one level out from the script written over it, wherever that
	// leaves the pair. An inverse image is written inside a subscript as readily
	// as on the line: page 77 sets the restriction of a section to the inverse
	// image of V as a subscript of f, so the u is at a level and its -1 is at a
	// level inside that. A symbol beside the script rather than under it is an
	// interleaved cluster, which is restack's.
	if b.depth != d-1 {
		return nil, nil, nil, 0, 0
	}
	// A large operator carries limits and not a script drawn over it, and hoist
	// has already put it in front of the limit written across it. Four pages of
	// chapter VIII print a sum under a sum and the two limits land at the same
	// place on the line, one read as above it and one as below, which is a sign
	// with superscripts on both sides of it and inside what they span. restack
	// leaves that alone on purpose, since merging the limits would write one of
	// them out twice and say nothing about the two lines that were run together,
	// and writing a script over the sign would be the same lie in another hand.
	if b.sign {
		return nil, nil, nil, 0, 0
	}
	for _, t := range toks[i:at] {
		if !overlaid(t, b) {
			return nil, nil, nil, 0, 0
		}
	}
	script = append(script, toks[i:at]...)
	// Everything the page draws after the symbol at the depth of the script
	// belongs to the symbol, since a symbol on the line is what ends a run of
	// scripts and begins the next, so the whole of the run is read and the
	// superscripts in it are the rest of the script. It is read whole and not
	// piece by piece because the far half of the script is drawn over the index
	// rather than over the letter: page 318 draws the one of its -1 clear of the
	// o and squarely over the B, and a walk that asked each piece whether it lay
	// over the letter stopped at it and left the page saying o^{-1}_B.
	//
	// A script of a script is refused rather than flattened. Nothing in the six
	// volumes writes one under an inverse image, and rebuilding one here would
	// mean guessing at where its braces close.
	var kept []token
	end = at + 1
	for end < len(toks) && toks[end].depth >= d {
		if toks[end].depth > d {
			return nil, nil, nil, 0, 0
		}
		if toks[end].level == Sup {
			script = append(script, toks[end])
		} else {
			kept = append(kept, toks[end])
		}
		end++
	}
	// The script was cut in two by the symbol drawn through it, so there is a
	// piece of it on each side. A script with nothing after the symbol is a
	// superscript that ended where the symbol began, which is where a superscript
	// is supposed to end.
	//
	// The symbol also has to occupy space, since nothing can be laid through a
	// thing that takes up none. The arrows of the commutative diagrams are drawn
	// out of the XY fonts and every one of them is reported a box of no width at
	// all, so an arrow standing after the label written over it read as a label
	// the arrow had been laid through, and three wrecked diagrams of chapter VIII
	// came back saying \overset{u}{/}.
	if len(script) == len(toks[i:at]) || b.right <= b.left {
		return nil, nil, nil, 0, 0
	}
	for n := 1; n < len(script); n++ {
		if script[n].left-script[n-1].right > restackGap {
			return nil, nil, nil, 0, 0
		}
	}
	// An index of the symbol is under the script or after it. Page 379 sets the
	// -1 of its inverse image across the whole of p sub n, and page 260 sets the
	// -1 of its inverse image of phi sub 1 across the phi alone, with the 1 six
	// units clear of the end of it. The first is one symbol with a script over
	// it and the second is a symbol with a script over it carrying an index, and
	// the page says which by where it drew the index.
	span := [2]int{script[0].left, script[len(script)-1].right}
	for _, t := range kept {
		if within([2]int{t.left, t.right}, span) {
			under = append(under, t)
		} else {
			after = append(after, t)
		}
	}
	body := [2]int{b.left, b.right}
	for _, t := range under {
		body = [2]int{min(body[0], t.left), max(body[1], t.right)}
	}
	if !within(body, span) {
		return nil, nil, nil, 0, 0
	}
	return script, under, after, at, end
}

// overlaid reports whether a token is drawn across the symbol it stands against
// rather than beside it, give or take the overhang one glyph box carries past
// the other.
//
// Across the page and not down it. Whether the script overlaps the band of the
// symbol tells nothing, which is the opposite of what it looks like it should
// tell: page 18 draws the minus of its inverse image three units into the band
// of the p and page 25 draws it two units clear of the band of the p, in the
// same volume and in the same face, and the difference is where the line put
// the letter rather than anything about the script.
func overlaid(t, b token) bool { return lap(t, b) > -overhang }

// abut writes a piece of a script on to the end of the one before it. The
// pieces are set side by side on the page and the text of them goes together
// the same way, except where the join would change what it joins: a control
// word runs on into a letter after it and stops being the word it was. The
// display on page 359 of Lie 7 to 9 writes 1 tensor Ch over its arrow, and the
// three pieces of that put end to end say \otimesCh, which is not a command
// any renderer has. TeX ends a control word at the first character that cannot
// be part of one, so a space is all that is wanted, and a space between two
// pieces of mathematics sets as nothing.
func abut(w *strings.Builder, text string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return
	}
	if s := w.String(); s != "" && unicode.IsLetter(rune(text[0])) {
		if i := strings.LastIndexByte(s, '\\'); i >= 0 && word(s[i+1:]) {
			w.WriteByte(' ')
		}
	}
	w.WriteString(text)
}

// word reports whether what follows a backslash is a control word rather than
// a control symbol. Only a word runs on: \, and \{ end at the character they
// are made of and take no space after them.
func word(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsLetter(r) {
			return false
		}
	}
	return true
}

// drawn writes the symbol with the script over it, out of the pieces the page
// drew each of them in.
func drawn(script []token, b token, under, after []token) token {
	var s, u, a strings.Builder
	for _, t := range script {
		abut(&s, t.text)
	}
	// An index the script is drawn over goes under the script with the letter,
	// and one the script stops short of goes after it, because that is where the
	// page put them. Page 379 prints the inverse image of p sub n with the -1
	// spanning the pair exactly, and page 260 prints the inverse image of phi
	// sub 1 with the -1 over the phi and the 1 clear of it to the right.
	for _, t := range under {
		abut(&u, t.text)
	}
	for _, t := range after {
		abut(&a, t.text)
	}
	body := strings.TrimSpace(b.text)
	if u.Len() > 0 {
		body += `_{` + u.String() + `}`
	}
	// The whole of it is mathematics whatever the symbol was set in. The p of
	// the inverse image comes out of the mathematics italic and the bracket after
	// it out of the roman, and the roman is what a base level letter of a French
	// page is more often than not.
	b.text = `\overset{` + s.String() + `}{` + body + `}`
	if a.Len() > 0 {
		b.text += `_{` + a.String() + `}`
	}
	b.class, b.math = ClassMath, true
	// The cluster keeps the reach it had, for the reason levels keeps it: what
	// stands on either side measures its distance from the edge of the token,
	// and the script reaches further to both sides than the symbol does.
	for _, t := range append(append(append([]token(nil), script...), under...), after...) {
		b.left, b.right = min(b.left, t.left), max(b.right, t.right)
	}
	return b
}

// restack puts the clusters of a line back into their levels.
func restack(toks []token) []token {
	out := make([]token, 0, len(toks))
	for i := 0; i < len(toks); {
		if toks[i].depth == 0 {
			out = append(out, toks[i])
			i++
			continue
		}
		j := i
		for j < len(toks) && toks[j].depth > 0 {
			j++
		}
		out = append(out, levels(toks[i:j])...)
		i = j
	}
	return out
}

// unit is one script of a cluster with whatever hangs off it: the i of M_{i_1}
// and the 1 under it are one unit, since they go where the i goes.
type unit struct {
	lo, hi      int
	level       Level
	left, right int
}

// levels gathers a cluster into its two scripts, where the cluster is a stack
// that came apart, and leaves it alone where it is not.
func levels(c []token) []token {
	us := units(c)
	if !stacked(c, us) {
		return c
	}
	out := make([]token, 0, len(c))
	for _, want := range []Level{us[0].level, flip(us[0].level)} {
		for _, u := range us {
			if u.level == want {
				out = append(out, c[u.lo:u.hi]...)
			}
		}
	}
	// What is on either side of the cluster measures its distance from the
	// token at the end of it, and after the reordering that token is no longer
	// the one that reaches furthest. Page 140 read "$A^{(\mathfrak{a})}_s$ ,
	// hence" with a space before the comma, because the s the subscript now
	// ends on stands where the closing bracket used to and the comma looked a
	// word away from it. So the ends of the cluster keep the reach the cluster
	// had.
	left, right := c[0].left, c[0].right
	for _, t := range c {
		left, right = min(left, t.left), max(right, t.right)
	}
	out[0].left, out[len(out)-1].right = left, right
	return out
}

// units cuts a cluster at every token written at the level of the cluster
// itself, so that what is written below that level stays with the token it
// hangs off.
func units(c []token) []unit {
	top := c[0].depth
	for _, t := range c {
		if t.depth < top {
			top = t.depth
		}
	}
	var us []unit
	for i, t := range c {
		if t.depth > top && len(us) > 0 {
			u := &us[len(us)-1]
			u.hi = i + 1
			u.left, u.right = min(u.left, t.left), max(u.right, t.right)
			continue
		}
		level := t.level
		// A prime sits on the line of whatever it is written against and takes
		// the level of the group it is in, which is what emit does with it too.
		if level == Base && len(us) > 0 {
			level = us[len(us)-1].level
		}
		us = append(us, unit{lo: i, hi: i + 1, level: level, left: t.left, right: t.right})
	}
	return us
}

// stacked reports whether a cluster is one superscript and one subscript that
// the page handed back interleaved.
//
// Three things are asked, and each of them is what tells the inverse from the
// flattened matrix that arrives looking like it.
//
// One script has to be written in more than one piece, since a cluster whose
// levels each arrive whole is not a stack that came apart and wants nothing
// done to it.
//
// The pieces of a script have to touch, since they were one thing before the
// other script was laid across them. This is what refuses the matrix: its
// columns stand apart by the space the page sets between them.
//
// One script has to lie inside what the other spans, since that is what being
// laid across something is. This is what refuses the pages where the line
// gathering has pulled in the limits of the display above, which touch and
// interleave but sit beside each other rather than one within the other.
//
// A cluster that carries the same box at both levels is refused whatever else
// it does. Four pages of chapter VIII print a sum under a sum and the two
// limits arrive at the same place on the line, one read as above it and one as
// below, and merging them would write the limit out twice and say nothing about
// the two lines that were run together to make it.
func stacked(c []token, us []unit) bool {
	if len(us) < 3 {
		return false
	}
	var seq []Level
	for _, u := range us {
		if u.level != Sup && u.level != Sub {
			return false
		}
		if len(seq) == 0 || seq[len(seq)-1] != u.level {
			seq = append(seq, u.level)
		}
	}
	if len(seq) < 3 {
		return false
	}
	for _, level := range []Level{Sup, Sub} {
		if !touching(us, level) {
			return false
		}
	}
	if !within(span(us, Sup), span(us, Sub)) && !within(span(us, Sub), span(us, Sup)) {
		return false
	}
	for i, a := range c {
		for _, b := range c[i+1:] {
			if a.level != b.level && a.left == b.left && a.right == b.right {
				return false
			}
		}
	}
	return true
}

// touching reports whether the pieces one level is written in stand close
// enough together to have been one piece before the other level cut them.
//
// The gap is only asked for across a break in the level, since two units
// written one after the other at the same level were never apart, and the
// space a page sets inside a script is wider than the space it leaves where it
// laid one script over another.
func touching(us []unit, level Level) bool {
	prev, right := -1, 0
	for i, u := range us {
		if u.level != level {
			continue
		}
		if prev >= 0 && i > prev+1 && u.left-right > restackGap {
			return false
		}
		if prev < 0 || u.right > right {
			right = u.right
		}
		prev = i
	}
	return true
}

// span is what one level of a cluster reaches across.
func span(us []unit, level Level) [2]int {
	out, first := [2]int{}, true
	for _, u := range us {
		if u.level != level {
			continue
		}
		if first {
			out, first = [2]int{u.left, u.right}, false
			continue
		}
		out = [2]int{min(out[0], u.left), max(out[1], u.right)}
	}
	return out
}

// within reports whether the first span lies inside the second, give or take
// the overhang one glyph box carries past the other.
func within(a, b [2]int) bool { return a[0] >= b[0]-overhang && a[1] <= b[1]+overhang }

func flip(l Level) Level {
	if l == Sup {
		return Sub
	}
	return Sup
}
