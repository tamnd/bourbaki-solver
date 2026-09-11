package translate

import (
	"strings"
	"unicode"

	"github.com/tamnd/bourbaki-solver/mathtex"
)

// Redollar puts the dollars back round a symbol the answer wrote into its prose
// without them, and leaves everything else exactly as the model wrote it.
//
// A model translating a Bourbaki paragraph into Vietnamese keeps the display
// formulas and the long spans intact and loses the short ones. It writes the
// letter, in the right place, spelled right, and does not put it in mathematics
// mode: the English "the subgroup $H$ of $E$" comes back as the Vietnamese for
// "the subgroup H of E", with H and E standing bare in the sentence. Nothing
// about the translation is wrong except the four characters that say those two
// letters are mathematics, and RuleMath refuses the answer for spans it cannot
// find.
//
// Refusing is the wrong end to fix it at, for the reason Respace and Readdress
// both give. Exercise 11 of Commutative Algebra III § 3 is the case this was
// written for: 1330 characters of English, thirty math spans, and the answer
// carries twenty of them. The ten it does not carry are H, p, p, H, E, L, p, E,
// p and L, all one letter each, every one of them standing bare in the
// Vietnamese at the point the English puts it. The exercise has never been
// translated. It was asked twice, the second time with the complaint in front of
// it, and the second answer lost the same ten letters as the first.
//
// Over the 19 869 archived Vietnamese answers that still have their question
// beside them, 3009 are refused as the rules stand today and 2037 of those are
// refused for the mathematics and nothing else. Those 2037 lose 5742 spans
// between them and 4543 of the 5742 are standing bare in the answer's own prose.
// 554 answers lose nothing else at all: put the dollars back and there is
// nothing left in them for any rule to find.
//
// It is a repair and not a normaliser, and every part of it is led by the
// English. Only a span the English has and the answer does not is ever put back;
// it is put back as the English writes it, character for character; and it is
// put back only where the English puts it, in the stretch of the answer between
// the two spans that surround it on both sides. Where anything does not line up
// this does nothing at all, and RuleMath reports what it was always going to.
func Redollar(en, tr string) string {
	want, unclosedEn := mathtex.Split(en)
	got, unclosedTr := mathtex.Split(tr)
	if unclosedEn != nil || unclosedTr != nil || len(got) >= len(want) {
		// Nothing was lost, so there is nothing here to put back. An answer
		// with more spans than the English, or with a delimiter left open, is
		// RuleMath's finding to report and not this function's to paper over.
		return tr
	}
	at, ok := align(want, got)
	if !ok {
		return tr
	}

	rs := []rune(tr)
	var b strings.Builder
	from, done := 0, 0
	for i, w := range want {
		if at[i] >= 0 {
			continue
		}
		// The gap is the stretch of the answer the English puts this span in:
		// after the last span that is still there before it, and before the
		// first that is still there after it. A letter written bare somewhere
		// else in the paragraph is a different letter.
		lo, hi := gap(rs, got, at, i)
		if lo < done {
			lo = done
		}
		start, found := bare(rs[lo:hi], w.Text)
		if !found {
			return tr
		}
		start += lo
		b.WriteString(string(rs[from:start]))
		b.WriteString("$" + w.Text + "$")
		from = start + len([]rune(w.Text))
		done = from
	}
	if from == 0 {
		return tr
	}
	b.WriteString(string(rs[from:]))
	return b.String()
}

// align matches the answer's spans onto the English's, and says so only when
// the match is the one obvious reading.
//
// The result is one entry per English span: the index of the answer span that
// is it, or -1 for a span the answer does not have. A span counts as the same
// one if it is the same text, or the same mathematics laid out differently,
// which is what Respace exists to put right afterwards and what this must not
// mistake for a loss.
//
// Matched from both ends inwards rather than left to right. A greedy pass from
// the left is enough to decide whether the answer's spans are a subsequence of
// the English's, and it is not enough to decide which subsequence: the spans
// that go missing here are single letters and a paragraph of Bourbaki writes
// the same letter a dozen times, so a greedy pass will happily match the
// answer's third p to the English's first and leave two losses in a gap that
// contains one letter. What is unambiguous is a run that agrees from the start
// and a run that agrees from the end, and this reports the two runs and gives
// up on anything in between that is not one contiguous block of losses.
func align(want, got []mathtex.Span) ([]int, bool) {
	at := make([]int, len(want))
	for i := range at {
		at[i] = -1
	}
	head := 0
	for head < len(got) && alike(want[head], got[head]) {
		at[head] = head
		head++
	}
	tail := 0
	for tail < len(got)-head && alike(want[len(want)-1-tail], got[len(got)-1-tail]) {
		at[len(want)-1-tail] = len(got) - 1 - tail
		tail++
	}
	// Everything between the two runs has to be loss and nothing else. If the
	// answer still holds a span in there, this cannot say which of the English's
	// it is, and a repair that guesses writes the wrong mathematics into the
	// corpus.
	return at, head+tail == len(got)
}

// alike says whether two spans are the same one, before Respace has been over
// them.
func alike(a, b mathtex.Span) bool {
	if a.Display != b.Display {
		return false
	}
	return a.Text == b.Text || sameButForSpace(a.Text, b.Text)
}

// gap is the stretch of the answer that a lost span belongs in, in runes.
//
// Bounded by the spans that survive on either side of it, and by the ends of
// the answer where there is no such span. The bounds are the delimiters' own
// positions and not the text inside them, so a letter written immediately
// against a span it neighbours is still inside the gap.
func gap(rs []rune, got []mathtex.Span, at []int, i int) (lo, hi int) {
	lo, hi = 0, len(rs)
	for j := i - 1; j >= 0; j-- {
		if at[j] >= 0 {
			lo = got[at[j]].End
			break
		}
	}
	for j := i + 1; j < len(at); j++ {
		if at[j] >= 0 {
			hi = got[at[j]].Start
			break
		}
	}
	return lo, hi
}

// bare finds the one place in a stretch of prose where a span is written out
// with nothing round it, and says so only when there is exactly one.
//
// A word boundary and not a substring: the answer is Vietnamese and the span is
// most often a single letter, so "E" is inside a dozen words of any paragraph
// and none of them is the E the English set in mathematics. Letters and digits
// on either side are what rules those out, in the Unicode sense rather than
// ASCII, because a Vietnamese letter is a letter and this would otherwise put
// dollars round the a in "và".
//
// Two occurrences is a refusal and not a choice. The gap is already as narrow
// as the surviving spans make it, and if the letter still stands twice in it
// then nothing here knows which one the English meant.
func bare(rs []rune, text string) (int, bool) {
	ts := []rune(text)
	at, found := -1, false
	for i := 0; i+len(ts) <= len(rs); i++ {
		if string(rs[i:i+len(ts)]) != text {
			continue
		}
		if i > 0 && (wordRune(rs[i-1]) || rs[i-1] == '$' || rs[i-1] == '\\') {
			continue
		}
		if end := i + len(ts); end < len(rs) && (wordRune(rs[end]) || rs[end] == '$') {
			continue
		}
		if inBraces(rs, i) {
			// An attribute block is the identifier a link resolves against and
			// a tag the corpus keeps, and neither is prose. Putting dollars
			// inside one would break the block to repair a formula that is not
			// in it.
			continue
		}
		if found {
			return 0, false
		}
		at, found = i, true
	}
	return at, found
}

func wordRune(r rune) bool {
	return r == '_' || unicode.IsLetter(r) || unicode.IsDigit(r)
}

// inBraces says whether a position sits inside an attribute block, which is the
// one thing in this corpus that is written in braces and is not prose.
func inBraces(rs []rune, i int) bool {
	for j := i - 1; j >= 0 && rs[j] != '\n'; j-- {
		if rs[j] == '}' {
			return false
		}
		if rs[j] == '{' {
			return true
		}
	}
	return false
}
