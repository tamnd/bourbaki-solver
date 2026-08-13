// Package tags allocates and keeps the permanent identifiers of the corpus.
//
// The idea is the Stacks Project's and we take it whole: every statement gets a
// short tag, the tag is assigned once, it never changes, and it is never reused
// even after the statement it named is gone. A label says where the book puts a
// statement and so moves when the book is renumbered; a tag says nothing at all
// and so cannot. That is the point of it. A citation, a translation, a solution
// and a URL all hang off the tag, and none of them break when § 5 gains a
// proposition and everything after it shifts by one.
//
// The files live in the corpus repository under tags/ and are plain text, one
// record to a line, because the whole contract rests on being able to read the
// history of that file and see that nothing was ever taken out of it.
package tags

import (
	"fmt"
	"strings"
)

// Alphabet is what a tag is written in, base 36 in the digits and the uppercase
// letters. It is the Stacks Project's alphabet.
const Alphabet = "0123456789ABCDEFGHIJKLMNOPQRSTUVWXYZ"

// Width is how many characters a tag has, which fixes how many there can ever
// be: 36^4 is 1679616. Chapter VIII alone wants about a thousand, the Éléments
// entire will not want a hundredth of the space, and a fixed width keeps the
// files sorting and aligning as text.
const Width = 4

// Tag is a permanent identifier. The zero Tag is not a tag.
//
// Nothing may be read out of a tag's value. It is allocated in sequence, so a
// low tag was assigned early, and that is the only thing its value knows. In
// particular the order of two tags says nothing about the order of the two
// statements in the book.
type Tag string

// Reserved is never assigned to anything, so that a tag parsed out of an empty
// or zeroed field is not silently a valid one.
const Reserved Tag = "0000"

// SampleA and SampleB stand in for a tag in the prompts that ask a model to
// write down what it used, and they are never assigned to anything either.
//
// They are reserved because of what the prompt used to say. It gave the shape of
// the tag line as USES: 00QM, 00QN, and 00QM and 00QN are Theorem 1 and
// Corollary 2 of Appendix 3, which is Hilbert's Nullstellensatz. Three of the
// five answers to exercise 1 of § 1, on Artinian modules over a principal ideal
// domain, came back with exactly that line under them, copied out of the
// instructions, and one of them is in the corpus. A sample citation that names
// a real result is a false citation waiting for a model with nothing better to
// write, so the sample now names two tags that no result can ever have.
const (
	SampleA Tag = "XXXX"
	SampleB Tag = "YYYY"
)

func held(t Tag) bool { return t == Reserved || t == SampleA || t == SampleB }

// Parse reads a tag and refuses anything that is not exactly Width characters
// of Alphabet. Lowercase is refused rather than folded: a tag is copied between
// files, URLs and citations by hand, and one spelling is what keeps those
// copies comparable as strings.
func Parse(s string) (Tag, error) {
	if len(s) != Width {
		return "", fmt.Errorf("tag %q is %d characters, want %d", s, len(s), Width)
	}
	for i := 0; i < len(s); i++ {
		if !strings.ContainsRune(Alphabet, rune(s[i])) {
			return "", fmt.Errorf("tag %q has %q in it, which is not in %s", s, s[i], Alphabet)
		}
	}
	if held(Tag(s)) {
		return "", fmt.Errorf("tag %q is reserved and is never assigned", s)
	}
	return Tag(s), nil
}

// FromInt is the nth tag in allocation order, counting Reserved as zero.
func FromInt(n int) (Tag, error) {
	if n <= 0 {
		return "", fmt.Errorf("tag number %d is not a tag: allocation starts at 1", n)
	}
	want, b := n, []byte(Reserved)
	for i := Width - 1; i >= 0; i-- {
		b[i] = Alphabet[n%len(Alphabet)]
		n /= len(Alphabet)
	}
	if n > 0 {
		return "", fmt.Errorf("the %d character tag space is full", Width)
	}
	if held(Tag(b)) {
		// Unreachable in any corpus this will ever hold, and here so that the
		// allocator cannot hand out something Parse refuses to read back.
		return "", fmt.Errorf("tag number %d is %q, which is reserved", want, b)
	}
	return Tag(b), nil
}

// Int is the inverse of FromInt. It is not exported as an ordering: it exists
// so that the allocator can count, and the allocator is the only caller.
func (t Tag) Int() int {
	n := 0
	for i := 0; i < len(t); i++ {
		n = n*len(Alphabet) + strings.IndexByte(Alphabet, t[i])
	}
	return n
}
