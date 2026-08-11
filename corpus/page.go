package corpus

import (
	"fmt"
	"path/filepath"
)

// A page file is the output of extraction and the input to assembly. It is one
// page of one volume, read either from the text layer of a born-digital PDF or
// from an image by a model, and it is the same file either way. That is the
// point of the contract: assembly does not ask where a page came from, and the
// audit compares the two paths against each other because they produce the
// same shape.
//
// Page files are committed. See PagesDir for why they moved out of work/.

// PageMethod is how a page was read.
type PageMethod string

const (
	// MethodNative is the text layer of a born-digital volume.
	MethodNative PageMethod = "native"
	// MethodOCR is a model reading a page image.
	MethodOCR PageMethod = "ocr"
	// MethodOCRRepair is a model repairing a page the text layer lost.
	MethodOCRRepair PageMethod = "ocr-repair"
	// MethodBlank is a page with nothing on it.
	MethodBlank PageMethod = "blank"
)

// PageLocator is where in the book a page sits, as far as the page itself says.
// The running head of this volume carries the § on one side of the spread and
// the no. on the other, so one of the two is known per page and the other is
// filled in by assembly, which sees the whole section at once.
type PageLocator struct {
	Section int `yaml:"section,omitempty"`
	Subsec  int `yaml:"subsec,omitempty"`
}

// PageFrontMatter is the head of pages/<book>/NNNN.md.
type PageFrontMatter struct {
	Book        string       `yaml:"book"`
	PDFPage     int          `yaml:"pdf_page"`
	PageLabel   string       `yaml:"page_label,omitempty"`
	RunningHead string       `yaml:"running_head,omitempty"`
	Locator     *PageLocator `yaml:"locator,omitempty"`

	// Continues says the first line of this page carries on the last paragraph
	// of the page before it. It is written here because only the reader of the
	// PDF can see it, in the indent of the first line, and assembly, which reads
	// these files and never the PDF, has to have it.
	Continues bool `yaml:"continues,omitempty"`

	Method PageMethod `yaml:"method"`
	Model  string     `yaml:"model,omitempty"`

	// InputSHA256 is the hash of what was read: the text layer for a native
	// page, the image for an OCR one. It is what says whether a page has to be
	// read again after the source is replaced.
	InputSHA256  string `yaml:"input_sha256"`
	PromptSHA256 string `yaml:"prompt_sha256,omitempty"`
	Generated    string `yaml:"generated"`
	Tokens       int    `yaml:"tokens,omitempty"`

	// Lines is how many lines of the page were read, and Flags is why the page
	// cannot be trusted as it stands. A page with flags is what the repair
	// pass works through.
	Lines int      `yaml:"lines"`
	Flags []string `yaml:"flags,omitempty"`

	// Columns is how many columns the page was set in. It is written only when
	// it is not one, which in this volume means the index at the back.
	Columns int `yaml:"columns,omitempty"`
}

// PageFile is one page of one volume.
type PageFile = File[PageFrontMatter]

// PagesDir is where the pages of one volume are written.
//
// Committed, not scratch. These were under work/ at first, on the reasoning
// that the assembled chapters are the product and a per page file is an
// intermediate. That was wrong twice over. A page that cost three minutes of a
// rented box to read is not an intermediate, it is the expensive thing, and
// work/ is gitignored, so five hundred pages of Algebra VIII sat on one laptop
// and the repository looked empty to anyone who opened it. Assembly reads these
// and writes content/; both are worth keeping and only one of them can be
// rebuilt in an afternoon.
//
// This is Markdown, so it is fine to commit. The PDFs and the page images stay
// out, and .gitignore keeps them out.
func PagesDir(root, book string) string {
	return filepath.Join(root, "pages", book)
}

// PagePath is the file one page is written to. The name is the PDF page padded
// to four digits, so the directory sorts in reading order.
func PagePath(root, book string, pdfPage int) string {
	return filepath.Join(PagesDir(root, book), fmt.Sprintf("%04d.md", pdfPage))
}

// ExtractReportPath is where the count of a run is published, next to the pages
// it counts.
func ExtractReportPath(root, book string) string {
	return filepath.Join(root, "reports", "extract-"+book+".json")
}
