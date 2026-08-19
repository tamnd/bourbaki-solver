// Package roundtrip is the sampled round trip over the translations.
//
// Everything else that looks at a translation proves it is the same text. The
// seven checks in package translate hold the mathematics span by span and in
// order, the tag block, the headings with their numbers, the citations, the
// count of blocks, and the alphabet the answer is written in, and L01 to L07
// hold the committed corpus to the same things afterwards. All of that is worth
// having and none of it reads the sentences. A fluent Vietnamese sentence that
// says the opposite of the English passes every check this project owns, and
// the translate command's own comment says so: that is what the sampled round
// trip and a reader are for. This is the first half of that sentence.
//
// The loop is: take a translated file, ask a model that has not seen the
// English to put it back into English, and then ask a judge whether the two
// English texts say the same mathematics. What comes back is not a proof. A
// difference the judge reports is a place to look, since the loss could equally
// have happened on the way back as on the way out, and the judge is itself an
// unmeasured instrument. solve eval exists to say what a solve verdict is worth
// and there is no equivalent here, so the count is a floor on the problems and
// not a rate of them. It is still the only automatic thing in the project that
// can see a sentence that means the wrong thing.
//
// The sample is drawn by hashing and not by a random number generator. A rate
// of five per cent published beside a number nobody else can reproduce is not a
// measurement, because a draw that can be taken again can be taken until it
// reads well, and neither the reader nor the person who ran it can tell
// afterwards which draw this was. Hashing the path makes the sample a function
// of the corpus: anyone with the tree recomputes the same set and can check
// that the files reported on are the files the rule picks.
package roundtrip

import (
	"crypto/sha256"
	"encoding/binary"
	"math"
	"sort"
)

// Rate is the share of the translated files that go round the loop, which the
// milestone puts at five per cent.
const Rate = 0.05

// An Item is one translated file offered to the sample.
type Item struct {
	// Path is the translation, relative to the corpus root.
	Path string
	// English is the file it was made from, the translated_from.
	English string
	Lang    string
	// Digest is the content hash of the translation's body as it stands now.
	// The draw does not look at it. It is what a verdict is stamped with, so
	// that a verdict can be told apart from a verdict about a text that has
	// since been translated again.
	Digest string
}

// Draw is the sample, in language and then path order.
//
// Membership is a function of the language and the path, and deliberately not
// of the body. Those are two different questions and they want two different
// answers. Which files are measured should hold still while the work goes on,
// or the population moves whenever a file is translated again and this run and
// the last one are reports about different corpora. Whether a verdict still
// stands should move with the body, because a verdict is about a text, and that
// is what Digest carries.
//
// The language goes into the hash, so the three languages draw independently.
// Sampling the same sections in all three would be one draw counted three
// times: an easy section picked for Vietnamese would be picked for Chinese and
// Japanese as well, and all three numbers would be flattered together. Three
// draws can be wrong in three different directions, which is what makes them
// worth averaging.
func Draw(items []Item, rate float64) []Item {
	if rate <= 0 || len(items) == 0 {
		return nil
	}
	byLang := map[string][]Item{}
	for _, it := range items {
		byLang[it.Lang] = append(byLang[it.Lang], it)
	}
	var out []Item
	for _, group := range byLang {
		out = append(out, drawOne(group, rate)...)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Lang != out[j].Lang {
			return out[i].Lang < out[j].Lang
		}
		return out[i].Path < out[j].Path
	})
	return out
}

// drawOne is the draw within one language, with the floor applied.
func drawOne(group []Item, rate float64) []Item {
	cut := uint64(math.MaxUint64)
	if rate < 1 {
		cut = uint64(rate * float64(math.MaxUint64))
	}
	var picked []Item
	low, lowAt := uint64(math.MaxUint64), -1
	for i, it := range group {
		s := Score(it.Lang, it.Path)
		if s < cut {
			picked = append(picked, it)
		}
		if s <= low {
			low, lowAt = s, i
		}
	}
	// The floor. Five per cent of the 39 files of one language is 1.95, and a
	// rate can round to nothing on a small tree: content/ja holds a handful of
	// files today and a run over it would report a sampling rate beside a sample
	// that measured nothing, which reads like a pass. One file is a weak
	// measurement and it is a measurement. The lowest hash is taken rather than
	// the first path, so the floor picks the same file every time and not
	// whatever happens to sort first.
	if len(picked) == 0 && lowAt >= 0 {
		picked = append(picked, group[lowAt])
	}
	return picked
}

// Score is where a file falls in the draw, low is picked first.
//
// Exported because a person looking at the published sample and asking why
// their file is not in it deserves an answer better than the word hash.
func Score(lang, path string) uint64 {
	sum := sha256.Sum256([]byte(lang + "\x00" + path))
	return binary.BigEndian.Uint64(sum[:8])
}
