package clip

import (
	"fmt"
	"regexp"
	"strings"
	"unicode"
)

// A display that came apart is the one fault the text layer cannot be argued
// out of.
//
// TeX sets a built-up fraction as three things that have nothing to do with
// each other in the file: a numerator on one band, a rule that is drawn and not
// written, and a denominator on the band below. Nothing in the text layer says
// they belong together. extract/line.go joins the bands it can prove belong
// together, by the size of the type or by a large operator standing over its
// limit, and a fraction whose halves are both at body size answers to neither
// test. So the page comes out with the numerator stranded on a line of its own
// and the denominator in a display below it, and M03 reports the stranded line.
//
// The picture has no such problem. A reader sees the rule. Over the twelve
// pages this was measured on, the page image put back every one of the fifteen
// displays the text layer had dropped, and on the three pages that had already
// been rebuilt by hand from the run boxes it agreed with the hand reading.
//
// What it cannot be trusted with is the rest of the page. It reads Fraktur S as
// script S, because those are hard to tell apart by eye and the font name in
// the text layer settles it outright. It read the superscript minus one of
// f^{-1}(A) as a bar over the f and handed back \overline{f}^{-1}(A), which is
// plausible French mathematics and is not what page 190 prints. Both faults are
// invisible in the answer: it comes back fluent.
//
// So the whole page is never taken. The wreck is taken, which is the run of
// blocks around a stranded line, and it is taken only when the prose on either
// side of it is the same in both readings and the replacement says no word the
// wreck did not already say. Everything else on the page stays as the text
// layer read it, faces and accents and all.

// Change is one wreck put back.
type Change struct {
	// Line is the one based line of the page body the wreck starts at.
	Line int
	// Was is the blocks that were replaced, and Now is what replaced them.
	Was string
	Now string
}

// Refused is one wreck that was not put back, and why.
type Refused struct {
	Line   int
	Was    string
	Reason string
}

// Fix puts back the displays of a page that came apart, taking them from a
// reading of the page image and nothing else from it.
//
// It returns the repaired body, what it changed and what it refused. A page
// with no wreck on it comes back unchanged with no changes and no refusals,
// which is not an error: most pages are like that.
func Fix(body, model string) (string, []Change, []Refused) {
	ours, theirs := blocks(body), blocks(model)
	wrecks := wrecked(ours)
	if len(wrecks) == 0 {
		return body, nil, nil
	}
	var changes []Change
	var refused []Refused
	// Rebuilt back to front, so that an index into ours stays valid while the
	// blocks in front of it are being replaced.
	out := append([]block{}, ours...)
	for i := len(wrecks) - 1; i >= 0; i-- {
		w := wrecks[i]
		was := join(ours[w.first : w.last+1])
		now, why := lift(ours, theirs, w)
		if why != "" {
			refused = append(refused, Refused{Line: ours[w.first].line, Was: was, Reason: why})
			continue
		}
		out = append(out[:w.first], append([]block{{text: now}}, out[w.last+1:]...)...)
		changes = append(changes, Change{Line: ours[w.first].line, Was: was, Now: now})
	}
	// Front to back is the order a reader wants them reported in.
	for l, r := 0, len(changes)-1; l < r; l, r = l+1, r-1 {
		changes[l], changes[r] = changes[r], changes[l]
	}
	if len(changes) == 0 {
		return body, nil, refused
	}
	return join(out), changes, refused
}

// block is one paragraph of a page, which for this purpose is anything between
// two blank lines.
type block struct {
	text string
	// line is the one based line of the body the block starts at.
	line int
}

func blocks(body string) []block {
	var out []block
	lines := strings.Split(body, "\n")
	start, held := 0, []string(nil)
	flush := func() {
		if len(held) > 0 {
			out = append(out, block{text: strings.Join(held, "\n"), line: start + 1})
			held = nil
		}
	}
	for i, line := range lines {
		if strings.TrimSpace(line) == "" {
			flush()
			continue
		}
		if len(held) == 0 {
			start = i
		}
		held = append(held, line)
	}
	flush()
	return out
}

func join(bs []block) string {
	var parts []string
	for _, b := range bs {
		parts = append(parts, b.text)
	}
	return strings.Join(parts, "\n\n")
}

// span is a run of blocks around at least one stranded line.
type span struct{ first, last int }

// stranded is the M03 shape: a block that is one line and that line is a single
// capital. It is the numerator of a fraction, or the upper half of whatever
// else was built up, left standing on a band of its own.
func stranded(b block) bool {
	t := strings.TrimSpace(b.text)
	r := []rune(t)
	return len(r) == 1 && unicode.IsUpper(r[0])
}

// Anchor is how many words a block needs before a replacement can be pinned
// against it.
//
// Four, which is short, and the shortness is doing two jobs. It has to admit
// C'est un élément de, which is the whole of the paragraph after the wreck on
// page 42 of Théories spectrales III and the only thing on that page a repair
// could be anchored to. And every block that clears it is a block a wreck stops
// at, so raising the bar does not make the repair safer, it makes the wreck
// longer: a bar of six swallows that paragraph instead of stopping at it, and
// then the extractor's reading of it is replaced by the model's for no reason.
//
// Four words matched exactly, in both readings, and found in only one place is
// evidence enough. What stands behind it is not the length but lift's proof,
// which holds the replacement to the words of what it replaces.
const Anchor = 4

// prose is a block solid enough to anchor a replacement against. A display is
// not prose however long it is, and neither is a fragment, which is what
// everything around a wreck is.
func prose(b block) bool {
	if strings.HasPrefix(strings.TrimSpace(b.text), "$$") {
		return false
	}
	return len(said(b.text)) >= Anchor
}

// wrecked is the runs of blocks worth replacing.
//
// A wreck reaches out from a stranded line to the nearest solid prose on either
// side, and everything between is taken: the stranded line, the displays that
// hold the other halves, and the fragments of the line the display was set in
// the middle of. Two stranded lines with nothing but fragments between them are
// one wreck, because the halves interleave and a repair that took them
// separately would have to guess which half went with which.
func wrecked(bs []block) []span {
	var out []span
	for i := 0; i < len(bs); i++ {
		if !stranded(bs[i]) {
			continue
		}
		first := i
		for first > 0 && !prose(bs[first-1]) {
			first--
		}
		last := i
		for last < len(bs)-1 && !prose(bs[last+1]) {
			last++
		}
		// A wreck that runs to the top or the bottom of the page is kept and
		// refused later rather than dropped here. It has nothing to anchor to
		// and it cannot be repaired, and a run that passed over it in silence
		// would report the page as whole, which is the one thing a repair pass
		// must never do about a page that is not.
		if n := len(out); n > 0 && out[n-1].last >= first {
			out[n-1].last = last
			continue
		}
		out = append(out, span{first: first, last: last})
		i = last
	}
	return out
}

// lift is the replacement for one wreck, taken from the other reading, or the
// reason there is not one.
func lift(ours, theirs []block, w span) (string, string) {
	if w.first == 0 {
		return "", "it runs to the top of the page, so there is nothing in front of it to pin a repair against"
	}
	if w.last == len(ours)-1 {
		return "", "it runs to the foot of the page, so there is nothing after it to pin a repair against"
	}
	before, ok := anchorFrom(theirs, ours[w.first-1], 0)
	if !ok {
		return "", "the paragraph in front of it is not in the reading of the picture"
	}
	after, ok := anchorFrom(theirs, ours[w.last+1], before+1)
	if !ok {
		return "", "the paragraph after it is not in the reading of the picture"
	}
	if after <= before+1 {
		return "", "the reading of the picture has nothing between those two paragraphs"
	}
	got := theirs[before+1 : after]
	if len(got) > w.last-w.first+1 {
		return "", fmt.Sprintf("the reading of the picture puts %d blocks where the page has %d", len(got), w.last-w.first+1)
	}
	was, now := join(ours[w.first:w.last+1]), house(join(got))
	if extra := added(was, now); len(extra) > 0 {
		return "", "it says words the page does not: " + strings.Join(extra, ", ")
	}
	if gone := added(now, was); len(gone) > 0 {
		return "", "it drops words the page has: " + strings.Join(gone, ", ")
	}
	return now, ""
}

// mathRE is a span of mathematics, display first so that a $$ is never taken
// for two inline dollars.
var mathRE = regexp.MustCompile(`(?s)\$\$.*?\$\$|\$[^$]*\$`)

// anchorFrom is where a paragraph of ours sits in the other reading, and false
// when it is not there or is there twice.
//
// A long paragraph is matched on its first six words and its last six rather
// than on all of them, because the middle is where the two readings write the
// same mathematics differently and the ends are prose in both. A short one is
// matched whole, because six and six of a paragraph of seven words is the
// paragraph twice over and there is no middle to spare.
func anchorFrom(bs []block, want block, from int) (int, bool) {
	key := said(want.text)
	if len(key) < Anchor {
		return 0, false
	}
	at, found := 0, false
	for i := from; i < len(bs); i++ {
		if !alike(said(bs[i].text), key) {
			continue
		}
		if found {
			// Two paragraphs that begin and end alike is not a match, it is a
			// coin toss, and the page is left alone.
			return 0, false
		}
		at, found = i, true
	}
	return at, found
}

// ends is how much of each end of a long paragraph is compared.
const ends = 6

func alike(got, key []string) bool {
	if len(got) < Anchor {
		return false
	}
	if len(key) < 2*ends {
		return same(got, key)
	}
	return len(got) >= 2*ends && same(got[:ends], key[:ends]) && same(got[len(got)-ends:], key[len(key)-ends:])
}

func same(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// facedRE and looseFaceRE are an upright or italic face put on a single letter,
// braced and unbraced. The braces are matched exactly one letter wide so that
// \mathrm{Homgr}, which is an operator's name and is set upright by everybody,
// is left where it is.
var (
	facedRE     = regexp.MustCompile(`\\(?:mathrm|mathit|textit)\{(\p{L})\}`)
	looseFaceRE = regexp.MustCompile(`\\(?:mathrm|mathit|textit) *(\p{L})(\P{L}|$)`)
	arrowRE     = regexp.MustCompile(`\\to(\P{L}|$)`)
	longMapsRE  = regexp.MustCompile(`\\longmapsto`)
)

// house puts a replacement into the spelling the rest of the corpus is written
// in, so that a display taken from the picture reads the same as the prose it
// was taken out of the middle of.
//
// Every rule here is a count over the pages and not a preference. \rightarrow
// against \to is 4894 to 272 and \mapsto against \longmapsto is 1773 to 7,
// because the extractor writes the arrow the font drew and a person types the
// short name.
//
// The face is the interesting one, and the model is not wrong about the page.
// Bourbaki sets M and P and S and I upright and sets a, q, x, n italic, and page
// 302 of Théories spectrales II prints exactly that: the model read it and
// wrote \mathrm P. The corpus does not mark it, because it is every capital in
// every one of these volumes, and a markup that is always on says nothing. So a
// capital is written bare and means the upright capital the compositor set. That
// is a convention and not a reading, which is why it can be applied to an answer
// without arguing with it, and \mathrm bears it out: it is on ten pages of the
// corpus and nine of them were read from an image.
//
// What would be shipped otherwise is the fault. The paragraph two lines above
// that display says P, the extractor read it, it is not being touched, and the
// repair would have put \mathrm P underneath it. Two spellings of one letter on
// one page, and the reader has to work out that they are the same letter.
//
// What is deliberately not here is \leqslant against \leq. Both are in the
// corpus, 2440 and 484, and they are two different signs on the page rather than
// two ways of writing one. The same goes for \cdots against \ldots.
func house(text string) string {
	text = facedRE.ReplaceAllString(text, "${1}")
	text = looseFaceRE.ReplaceAllString(text, "${1}${2}")
	text = arrowRE.ReplaceAllString(text, `\rightarrow${1}`)
	return longMapsRE.ReplaceAllString(text, `\mapsto`)
}

// controlRE is a TeX control word, which is not a word of the page.
var controlRE = regexp.MustCompile(`\\[A-Za-z]+`)

// wordRE is two or more letters together.
var wordRE = regexp.MustCompile(`\p{L}{2,}`)

// said is the running text of a block, lowercased, with the TeX taken out.
//
// The TeX has to go before the words are counted, because \frac and \left are
// letters and are not language, and a replacement that turns three broken
// blocks into one \frac would otherwise look like it had invented two words.
func said(text string) []string {
	text = controlRE.ReplaceAllString(text, " ")
	var out []string
	for _, w := range wordRE.FindAllString(strings.ToLower(text), -1) {
		if notLanguage[w] {
			continue
		}
		out = append(out, w)
	}
	return out
}

// notLanguage is the words that are not words, dropped from both readings so
// that neither is held to the other's way of writing them.
//
// The operator names are here because a text layer has no macros in it: the
// extractor reads the roman sup of a supremum as the letters s, u, p and writes
// them, and the model writes \sup, which controlRE has already taken out. Page
// 424 of Théories spectrales III turns on it. The paragraph before the wreck
// ends sur G égale sup_{x in G} u(x). Si p > 1, définissons, and the two
// readings differ in exactly one word of it, which is the one word that is not
// language, so the repair was refused over a difference nobody would call one.
//
// The numéro is the same shape from the other end. Bourbaki cites a subsection
// as n with a raised o, the extractor writes the superscript and the model types
// no, and the citation is in the middle of half the paragraphs in the book.
var notLanguage = map[string]bool{
	"no":  true,
	"sup": true, "inf": true, "lim": true, "max": true, "min": true,
	"log": true, "exp": true, "sin": true, "cos": true, "tan": true,
	"det": true, "dim": true, "ker": true, "card": true, "coker": true,
	"deg": true, "pgcd": true, "ppcm": true, "mod": true,
}

// added is the prose the replacement says and the wreck did not. Run both ways
// round by lift, it is the whole proof: the replacement says the words the
// wreck said, no more and no fewer, and everything else about it is free.
//
// A model handed a page image and asked for the page will give back the page,
// and it will also tidy a sentence, expand a citation and explain what it just
// transcribed, and none of that is visible in a diff nobody reads. That is the
// half this catches going out. Coming back it catches the other half, which is
// a model that reads three broken blocks, understands two of them and quietly
// drops the sentence it could not place.
//
// The mathematics is not counted, in either direction, and that is deliberate
// rather than a gap. The mathematics is the thing being rebuilt: the wreck's is
// broken by definition and holding the repair to it would be holding it to the
// defect. It is also not made of words. A text layer that has set two italic
// letters side by side gives qP and qq and ix and zu, which are not words the
// page says and not words the repair drops, and counting them refused five of
// the seven pages this was first run on for no reason anybody could act on.
//
// What bounds the mathematics instead is everything around it: the replacement
// has to sit between two paragraphs both readings write the same way, and it
// has to be no longer in blocks than what it replaces.
func added(was, now string) []string {
	had := map[string]bool{}
	for _, w := range said(mathRE.ReplaceAllString(was, " ")) {
		had[w] = true
	}
	seen := map[string]bool{}
	var out []string
	for _, w := range said(mathRE.ReplaceAllString(now, " ")) {
		if had[w] || seen[w] {
			continue
		}
		seen[w] = true
		out = append(out, w)
	}
	return out
}
