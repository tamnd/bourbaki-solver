// Package prompt holds the prompts the pipeline sends to a model.
//
// They are files rather than string literals so that a prompt can be read and
// edited as prose, and they are embedded rather than loaded from disk so that a
// binary carries the exact text it was built with. Every prompt has a hash, and
// that hash goes in the front matter of every page it produced: when the prompt
// changes, the pages it produced are detectably stale rather than silently
// mixed with pages produced by a different one.
package prompt

import (
	"crypto/sha256"
	_ "embed"
	"encoding/hex"
	"fmt"
	"strings"
)

//go:embed ocr_bourbaki.md
var ocrBourbaki string

// OCR is the prompt for reading a page of a scanned volume.
func OCR() string { return strings.TrimSpace(ocrBourbaki) + "\n" }

//go:embed ocr_ens.md
var ocrENS string

//go:embed ocr_foot.md
var ocrFoot string

// volumeNote is what a Book adds to the scanned prompt, by book id.
//
// The prompt above was written against Algebra and says so, and a Book that
// sets a notation of its own writes it here rather than there. A note is added
// and the prompt above is not touched, because the prompt hash goes in the
// front matter of every page it produced: a sentence about the sign for a
// theory, put in the shared prompt, would put every page of every other volume
// back in the queue for a rule that is about none of them.
//
// A Book with nothing to add gets the shared prompt byte for byte, so its pages
// do not go stale for a note about a volume it has no part in.
// A volume that prints its page number at the foot and has no text layer behind
// it gets the foot note, because those two together are what leave a volume with
// no page number anywhere. Six volumes are foot-number and four of them carry a
// text layer, so pdftotext reads the foot off that and the reading never has to.
// The two here are the ones where the reading is the only chance at it, and they
// are the two the page map cannot fit.
//
// ens-i-iv is foot-number as well and is deliberately not here. It has a text
// layer, and its own note already says where the folio sits, so adding this one
// would move its hash and put 416 read pages back in the queue for a rule that
// changes nothing about them.
var volumeNote = map[string]string{
	"ens-i-iv": ocrENS,
	"ac-i-vii": ocrFoot,
	"top-v-x":  ocrFoot,
}

// OCRFor is the prompt for reading a page of one scanned volume.
func OCRFor(book string) string {
	note, ok := volumeNote[book]
	if !ok {
		return OCR()
	}
	return OCR() + "\n" + strings.TrimSpace(note) + "\n"
}

// OCRForSHA256 is the hash of the prompt one volume is read with.
func OCRForSHA256(book string) string { return SHA256(OCRFor(book)) }

// OCRAnything is every prompt a page of one volume could have been read with,
// run together.
//
// For a caller holding a page and not the record of which prompt asked for it,
// which is what `ocr check` and the extraction report are: they read the files
// on disk and the front matter carries the hash of the prompt, not its text.
// Rule 3 only ever matches a whole line, so the extra prompts cost nothing and
// asking with the wrong one would miss a page that echoed a different one.
func OCRAnything(book string) string {
	return OCRFor(book) + "\n" + OCRNative() + "\n" + Contents()
}

//go:embed ocr_native.md
var ocrNative string

// OCRNative is the prompt for reading a flagged page of a born-digital volume.
//
// It is a second file and not a rule added to the one above, and the reason is
// what a prompt change costs. The prompt hash goes in the front matter of every
// page it produced, so editing the scanned prompt puts 1194 pages of Algebra I
// to VII back in the queue, which is days of fleet time bought for a sentence
// about a typeface those volumes rarely set.
//
// The question is different anyway. A scanned page is a photograph and the
// model is rebuilding a structure out of it. A flagged page of a born-digital
// volume is clean type and already has a reading out of the text layer; what is
// wanted is the part of it the layer could not carry, which is a narrow list.
//
// The rules that are here and not there were put here by the pilot on Lie 7 to
// 9. Two pages came back with the mathematics repaired and the faces flattened:
// the Lie algebra \mathfrak{g} was written as g, so \lambda_{\mathfrak{g}} came
// back as \lambda_g and said something else, and \mathscr{C} came back as
// \mathcal{C}. The second page turned "### 4. CENTRAL FUNCTIONS ON G AND
// FUNCTIONS ON T" into bold prose, which assembly does not see as a heading and
// which would have dropped a subsection out of the chapter.
//
// The second pass over the same volume, eleven pages, found the faces again and
// in both directions. The model wrote \lambda_g where the layer has
// \lambda_{\mathfrak{g}}, so a fraktur letter survives on its own and not in a
// subscript. It wrote \mathrm{SU}(2,\mathbf{C}) for a group the book sets in
// bold. And it bolded twelve ordinary italic capitals on page 139, turning the
// module Z(\lambda-\rho) into the ring of integers, which is the number set
// rule reaching letters it was never about. All three are named in the prompt
// now with the page that found them.
//
// What the text layer knows about a typeface it knows better than a picture
// does, and the pages that come back from here are worth checking against the
// reading they replaced rather than trusted. See bourbaki-solver#111.
func OCRNative() string { return strings.TrimSpace(ocrNative) + "\n" }

// OCRNativeSHA256 is the hash of the born-digital prompt as embedded.
func OCRNativeSHA256() string { return SHA256(OCRNative()) }

//go:embed ocr_contents.md
var ocrContents string

// Contents is the prompt for reading a page of a table of contents.
//
// It is a third prompt and not a rule added to either of the other two, and the
// reason is what those two do with a contents page. The scanned prompt asks for
// Markdown with the structure marked in hashes, and a model following it reads
// the contents of Theory of Sets as a list of headings with the leader dots and
// the page numbers thrown away as layout. That is the right answer for a page of
// prose and the wrong one here, where the page numbers are the whole content:
// the contents is the only place in a volume that says where every no. begins,
// and an entry without its page says nothing at all.
//
// So this asks for plain text laid out as the page lays it out, indentation and
// leaders and page labels kept, which is what the volumes with a working text
// layer hand the contents reader already. The reader downstream is then the same
// code for both, and what the model says is checked against the page map and
// against the order of the contents rather than believed.
//
// It exists because several volumes have no other source. The scan of Espaces
// vectoriels topologiques prints every entry of its contents with leaders and no
// page at all, and the scan of Groupes et algebres de Lie chapitre 9 keeps the
// label on the last few lines and drops it from the rest. Nothing can be read
// out of either.
func Contents() string { return strings.TrimSpace(ocrContents) + "\n" }

// ContentsSHA256 is the hash of the contents prompt as embedded.
func ContentsSHA256() string { return SHA256(Contents()) }

//go:embed clip_line.md
var clipLine string

// ClipLine is the prompt for reading a picture of one line of a page.
//
// It is not the OCR prompt with the page rules taken out. A page and a line are
// different questions: a page carries a running head, headings and paragraph
// breaks and the answer is a structure, and a line carries none of that and the
// answer is one line. Half of what this asks for is about the cut itself, which
// leaves the neighbouring lines sliced off at the edges, and none of that
// applies to a page.
func ClipLine() string { return strings.TrimSpace(clipLine) + "\n" }

// ClipLineSHA256 is the hash of the clip prompt as embedded.
func ClipLineSHA256() string { return SHA256(ClipLine()) }

//go:embed clip_page.md
var clipPage string

// ClipPage is the prompt for reading a picture of a whole page.
//
// It is a paragraph where the OCR prompt is two pages, and the short one is the
// one to reach for first. The long prompt was written a rule at a time against
// the scanned volumes, where the model was being asked to rebuild a structure
// out of a photograph; a born-digital page cut to its own text block is a clean
// picture of clean type, and most of those rules are answering questions it
// does not raise.
//
// Every rule that is here was put here by a page that came back wrong. The
// first version said to write the rings in bold and the model bolded every
// capital on the page, so that a Banach space E, a cone C and a subspace F all
// came back as number sets; hence the sentence saying what bold is not for. It
// wrote \mathcal where Bourbaki sets script, \rho for the curly rho, \xi for
// the script l, a dot for the ring on an interior, and parentheses for the
// angle brackets of a duality pairing. And on three pages in a row it read
// \not= as =, which is the one defect that costs a reader the meaning of the
// sentence rather than the look of it, so it is named.
func ClipPage() string { return strings.TrimSpace(clipPage) + "\n" }

// ClipPageSHA256 is the hash of the page prompt as embedded.
func ClipPageSHA256() string { return SHA256(ClipPage()) }

//go:embed glossary.md
var glossaryPrompt string

// Glossary is the prompt that asks for the rendering of a list of terms.
//
// It takes the language spelled out rather than its code. "vi" is a thing a
// model has to guess at and "Vietnamese" is not, and the guess is not worth
// saving eight characters over.
//
// The terms come in already numbered and one to a line, because the numbering
// is what the answer is checked against and the checker has to be the thing
// that wrote it.
// note is anything true of this language and not of the other three, and it is
// empty for most of them. It goes above the terms and below the rules, in the
// place a reader meets last before the list.
func Glossary(language, note, terms string) string {
	if n := strings.TrimSpace(note); n != "" {
		note = n + "\n"
	} else {
		note = ""
	}
	text := strings.ReplaceAll(strings.TrimSpace(glossaryPrompt), "{{LANGUAGE}}", language)
	text = strings.ReplaceAll(text, "{{NOTE}}", note)
	return strings.ReplaceAll(text, "{{TERMS}}", strings.TrimSpace(terms)) + "\n"
}

// GlossarySHA256 is the hash of the glossary prompt as embedded, with the
// language and the terms left standing. It identifies the instructions, which
// is the part that is the same for every batch and every language: two runs
// that carry the same hash were asked for the same thing in the same words.
func GlossarySHA256() string { return SHA256(strings.TrimSpace(glossaryPrompt) + "\n") }

//go:embed glossary_aligned.md
var glossaryAlignedPrompt string

// GlossaryAligned is the question asked of a paragraph that exists in both
// printings.
//
// It is a different question from Glossary and a better one. Glossary asks what
// the French for a term is, which is a thing a model knows or invents. This puts
// the French paragraph in front of it and asks which words in that paragraph are
// the term, so the answer is in the passage or it is refused. Half the series
// was never translated into English and the other half was, and this is what the
// half that was is worth.
func GlossaryAligned(english, french, terms string) string {
	text := strings.ReplaceAll(strings.TrimSpace(glossaryAlignedPrompt), "{{ENGLISH}}", strings.TrimSpace(english))
	text = strings.ReplaceAll(text, "{{FRENCH}}", strings.TrimSpace(french))
	return strings.ReplaceAll(text, "{{TERMS}}", strings.TrimSpace(terms)) + "\n"
}

//go:embed translate.md
var translateCommon string

//go:embed translate_vi.md
var translateVI string

//go:embed translate_zh.md
var translateZH string

//go:embed translate_ja.md
var translateJA string

//go:embed translate_en.md
var translateEN string

// The translation prompt is two files and not one.
//
// The spec asks for prompt/translate_<lang>.md, and taken literally that is
// three files each carrying the whole thing. Most of the whole thing is the
// same in all three: the mathematics is copied, the tag block is copied, the
// structure holds, the answer is the body and nothing else. Those rules are the
// ones that matter and they are the ones that will get edited, and three copies
// of a rule is three chances to edit two of them. So translate.md carries what
// is common and translate_<lang>.md carries what is not: the words for
// Proposition and Lemma, what to do with a proper name, the house style, and
// three worked examples in the language, which cannot be shared because the
// answer half of an example is the language.
//
// The hash covers both files, so a change to either marks the pages stale.

// translateRules returns the language half of the prompt.
func translateRules(lang string) (string, bool) {
	switch lang {
	case "vi":
		return translateVI, true
	case "zh":
		return translateZH, true
	case "ja":
		return translateJA, true
	case "en":
		return translateEN, true
	}
	return "", false
}

// Translate is the prompt that asks for a section body in another language.
//
// glossary is the terminology block, already laid out one term to a line, and
// body is the English between the front matter fences and nothing else. Neither
// is escaped, because both go over as a file rather than on a command line and
// the prompt tells the model that everything between the two lines of equals
// signs is source.
//
// There are two lines and there used to be one, and the second one is worth a
// paragraph. With the source opened and never closed, the model translated the
// section and then carried on past the end, re-emitting the last sentence or
// two: five asks in a row on the same chunk of the Nullstellensatz appendix
// came back with a duplicated tail, and it is in the bytes of the stream rather
// than anything the transport did, so it is the model finding no place to stop.
// Closing the block and saying to stop there fixed it three asks out of three
// on the same chunk. The audit caught every one of the five, which is what it
// is for, but a rule that refuses good work because the prompt left a door open
// is a rule doing somebody else's job.
//
// note is anything the caller wants said about this particular ask, and it is
// empty on a first attempt. It goes above the line of equals signs and not
// below it. That is the whole reason it is a parameter here rather than
// something a caller appends: the first version of the retry appended its
// complaint to the finished prompt, which put it after the sentence saying
// everything below is source and none of it is an instruction. It happened to
// work and it was wrong, and a prompt that is wrong and works is a prompt that
// will stop working on a section that is longer.
// source is the language the passage is written in. It is a parameter rather
// than the constant English it used to be because the French only volumes have
// no English to translate from, and the rules that say what to copy and what to
// carry over have to name the language actually in front of the model.
func Translate(source, lang, glossary, note, body string) (string, error) {
	rules, ok := translateRules(lang)
	if !ok {
		return "", fmt.Errorf("no translation prompt for language %q", lang)
	}
	if n := strings.TrimSpace(note); n != "" {
		note = "\n" + n + "\n"
	} else {
		note = ""
	}
	// A chunk with no glossary terms in it gets a sentence saying so rather than
	// a heading over an empty list. Short chunks do happen, and a heading with
	// nothing under it reads to a model as a list it failed to receive.
	terms := "There is no glossary entry for anything in this passage."
	if g := strings.TrimSpace(glossary); g != "" {
		terms = "The glossary. Left is the " + Language(source) + " as the book spells it, right is" +
			" what to write.\n\n" + g
	}
	text := strings.TrimSpace(translateCommon)
	text = strings.ReplaceAll(text, "{{SOURCE}}", Language(source))
	text = strings.ReplaceAll(text, "{{LANGUAGE}}", Language(lang))
	text = strings.ReplaceAll(text, "{{RULES}}", strings.TrimSpace(rules))
	text = strings.ReplaceAll(text, "{{GLOSSARY}}", terms)
	text = strings.ReplaceAll(text, "{{NOTE}}", note)
	text = strings.ReplaceAll(text, "{{BODY}}", strings.TrimSpace(body))
	return text + "\n", nil
}

// passageFence is the line of equals signs translate.md puts either side of the
// passage. It is a whole line, newlines included, so that a run of equals signs
// inside a sentence cannot be mistaken for it.
const passageFence = "\n==========\n"

// TranslatePassage is the passage a finished ask carried, read back out of it.
//
// A question is archived beside its answer, and the archive is the only record
// of what a model was actually shown. Reading the passage back out is how a
// later run tells whether an answer it finds on disk was written about the text
// it is holding now, which is not the same question as whether the file is
// where that text's answer would be filed. See archivedAnswers: the archive is
// named by the section and the chunk number, so a section chunked differently
// than it was the day the answer was written files a different passage under
// the same name.
//
// The first fence opens and the last one closes, rather than the first two,
// because a passage that happens to contain a line of equals signs is then read
// whole instead of being cut at it. A stray fence above the passage, in the
// glossary or in a note, does the opposite and takes half the prompt with it,
// and that is the harmless direction: the text will not match what the caller
// is holding and the answer is passed over.
func TranslatePassage(ask string) (string, bool) {
	i := strings.Index(ask, passageFence)
	if i < 0 {
		return "", false
	}
	i += len(passageFence)
	j := strings.LastIndex(ask, passageFence)
	if j < i {
		return "", false
	}
	return strings.TrimSpace(ask[i:j]), true
}

// Language spells out a language code for a model to read.
func Language(lang string) string {
	switch lang {
	case "vi":
		return "Vietnamese"
	case "zh":
		return "Chinese, written in simplified characters"
	case "ja":
		return "Japanese"
	case "en", "en-mt":
		return "English"
	case "fr":
		return "French"
	}
	return lang
}

// TranslateSHA256 is the hash of the instructions for one language, with the
// glossary and the body left standing.
//
// It covers both halves of the prompt, so editing either the common rules or
// one language's rules changes the hash and marks the sections that prompt
// produced as stale. It does not cover the glossary: a glossary change is
// tracked by glossary_version, which is a different kind of staleness and moves
// on a different schedule.
// It hashes the rules as the model will read them, with the source language
// already substituted, rather than as they sit in the file. That is what keeps
// adding the source axis from marking every finished translation stale: an
// English source renders {{SOURCE}} back to the word the file used to spell
// out, so the text hashed is byte for byte what it was before the placeholder
// existed, and only a passage whose source is not English gets a new hash.
func TranslateSHA256(source, lang string) (string, error) {
	rules, ok := translateRules(lang)
	if !ok {
		return "", fmt.Errorf("no translation prompt for language %q", lang)
	}
	common := strings.ReplaceAll(strings.TrimSpace(translateCommon), "{{SOURCE}}", Language(source))
	return SHA256(common + "\n" + strings.TrimSpace(rules) + "\n"), nil
}

// SHA256 hashes a prompt. This is what goes in a page's prompt_sha256.
func SHA256(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// OCRSHA256 is the hash of the OCR prompt as embedded.
func OCRSHA256() string { return SHA256(OCR()) }

//go:embed roundtrip_back.md
var roundtripBackPrompt string

//go:embed roundtrip_judge.md
var roundtripJudgePrompt string

// RoundTripBack asks for a translation to be put back into English.
//
// The passage goes over on its own. The English it was made from is not in this
// question and must never be put in it: a model shown both would be copying
// rather than translating, and the whole measurement rests on the return trip
// being made by somebody who has not seen the outbound one.
func RoundTripBack(lang, body string) string {
	text := strings.ReplaceAll(strings.TrimSpace(roundtripBackPrompt), "{{LANGUAGE}}", Language(lang))
	return strings.ReplaceAll(text, "{{BODY}}", strings.TrimSpace(body)) + "\n"
}

// RoundTripJudge asks whether the English that came back says the same
// mathematics as the English that went out.
func RoundTripJudge(english, back string) string {
	text := strings.ReplaceAll(strings.TrimSpace(roundtripJudgePrompt), "{{ENGLISH}}", strings.TrimSpace(english))
	return strings.ReplaceAll(text, "{{BACK}}", strings.TrimSpace(back)) + "\n"
}

// RoundTripSHA256 is the hash of both halves of the loop, so that editing
// either marks the verdicts made under the old wording as made under the old
// wording.
func RoundTripSHA256() string {
	return SHA256(strings.TrimSpace(roundtripBackPrompt) + "\n" + strings.TrimSpace(roundtripJudgePrompt) + "\n")
}
