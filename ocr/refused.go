package ocr

import (
	"os"
	"path/filepath"
	"strings"
)

// A refusal is the third thing that can happen to a page, and it took a volume
// to find it.
//
// The first two were known: a page comes back and is good, or a page comes back
// and is wrong. Reading Theory of Sets on this machine turned up a third. The
// Historical Note at the end of the volume is twenty five pages of continuous
// English prose with no mathematics in it, and the reader will not return it:
// the API answers 400 with a content filtering policy on an output that is a
// long verbatim stretch of a published book. Every model tier does it. Opus and
// Sonnet are stopped by the filter and Haiku declines in its own words, so this
// is a property of the transport and not of a model to be swapped.
//
// What matters for the queue is that this is not evidence about the page. The
// page is fine, the image is fine, and the same page read through the fleet,
// which is a different provider under a different policy, comes back. So a
// refusal must not spend an attempt: four of them and the queue gives up on a
// page for good, and the corpus would be missing the Historical Note with
// nothing in the record but "dead". It must also not be retried here, because
// the answer is the same every time and each retry is a minute.
//
// So the run hands the page back untouched, stops offering it to the host that
// refused it, and reports it under its own name. The pages stay pending, which
// is exactly what they are: work still to do, on a host that will do it.

// RefusedMark is what a reader prints when it will not return a page at all.
//
// One string in one place because two programs use it: ocr-batch prints it into
// the batch log, and it reads as a sentence there.
const RefusedMark = "the model will not return this page"

// RefusedName is the file a reader leaves beside the answers to say a page was
// refused, 0301.png becoming 0301.refused.
//
// A file rather than a line in the log, for two reasons. The log comes back as
// its last twenty five lines, so a batch of twelve pages that refused all of
// them could push the first refusals off the top and the run would read those
// pages as merely missing. And the answers are pulled back with rsync anyway,
// so a sidecar costs nothing to carry.
//
// The extension is not .md on purpose. The poll counts the Markdown files in
// the output directory to know how far a batch has got, so a refusal named
// 0301.md would be counted as a page that had been read.
func RefusedName(image string) string {
	return strings.TrimSuffix(image, filepath.Ext(image)) + ".refused"
}

// Refusal reads the sidecar for a page, and says whether there is one.
func Refusal(dest, image string) (string, bool) {
	raw, err := os.ReadFile(filepath.Join(dest, RefusedName(filepath.Base(image))))
	if err != nil {
		return "", false
	}
	said := strings.TrimSpace(string(raw))
	if said == "" {
		return RefusedMark, true
	}
	return said, true
}

// WriteRefusal leaves that sidecar.
func WriteRefusal(dir, image, reason string) error {
	return os.WriteFile(filepath.Join(dir, RefusedName(filepath.Base(image))), []byte(reason+"\n"), 0o644)
}
