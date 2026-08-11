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
func Glossary(language, terms string) string {
	text := strings.ReplaceAll(strings.TrimSpace(glossaryPrompt), "{{LANGUAGE}}", language)
	return strings.ReplaceAll(text, "{{TERMS}}", strings.TrimSpace(terms)) + "\n"
}

// GlossarySHA256 is the hash of the glossary prompt as embedded, with the
// language and the terms left standing. It identifies the instructions, which
// is the part that is the same for every batch and every language: two runs
// that carry the same hash were asked for the same thing in the same words.
func GlossarySHA256() string { return SHA256(strings.TrimSpace(glossaryPrompt) + "\n") }

//go:embed translate.md
var translateCommon string

//go:embed translate_vi.md
var translateVI string

//go:embed translate_zh.md
var translateZH string

//go:embed translate_ja.md
var translateJA string

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
func Translate(lang, glossary, note, body string) (string, error) {
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
	terms := "There is no glossary entry for anything in this section."
	if g := strings.TrimSpace(glossary); g != "" {
		terms = "The glossary. Left is the English as the book spells it, right is" +
			" what to write.\n\n" + g
	}
	text := strings.TrimSpace(translateCommon)
	text = strings.ReplaceAll(text, "{{LANGUAGE}}", Language(lang))
	text = strings.ReplaceAll(text, "{{RULES}}", strings.TrimSpace(rules))
	text = strings.ReplaceAll(text, "{{GLOSSARY}}", terms)
	text = strings.ReplaceAll(text, "{{NOTE}}", note)
	text = strings.ReplaceAll(text, "{{BODY}}", strings.TrimSpace(body))
	return text + "\n", nil
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
func TranslateSHA256(lang string) (string, error) {
	rules, ok := translateRules(lang)
	if !ok {
		return "", fmt.Errorf("no translation prompt for language %q", lang)
	}
	return SHA256(strings.TrimSpace(translateCommon) + "\n" + strings.TrimSpace(rules) + "\n"), nil
}

// SHA256 hashes a prompt. This is what goes in a page's prompt_sha256.
func SHA256(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// OCRSHA256 is the hash of the OCR prompt as embedded.
func OCRSHA256() string { return SHA256(OCR()) }
