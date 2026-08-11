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

// SHA256 hashes a prompt. This is what goes in a page's prompt_sha256.
func SHA256(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// OCRSHA256 is the hash of the OCR prompt as embedded.
func OCRSHA256() string { return SHA256(OCR()) }
