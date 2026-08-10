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

// SHA256 hashes a prompt. This is what goes in a page's prompt_sha256.
func SHA256(text string) string {
	sum := sha256.Sum256([]byte(text))
	return hex.EncodeToString(sum[:])
}

// OCRSHA256 is the hash of the OCR prompt as embedded.
func OCRSHA256() string { return SHA256(OCR()) }
