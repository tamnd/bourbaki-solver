package book

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

// Building the PDF is the whole point of this package, and not because anybody
// needs the PDF.
//
// The corpus has been checked one file at a time since M2. The audit reads a
// body, publish -check reads a math span, the anchor census reads an identifier.
// None of them can tell you that chapter III follows chapter II, that the § that
// was assembled out of forty pages is the § the printing has, or that the whole
// thing comes to about the length of the volume it was taken from. A typesetter
// answers all three at once and complains in a way that is hard to argue with:
// an overfull box is a line that does not fit, a missing character is a hole in
// the page, and a page count that is half the printing's is half a book.

// A Build is one run of the typesetter over one document.
type Build struct {
	Dir string // where the .tex, the .cls and the .pdf are
	PDF string
	Log string
	// Pages is what the typesetter made, which is the number the audit compares
	// against the printing's own page count. It will not agree exactly and is
	// not meant to. A build that comes out at three fifths of the printing is
	// three fifths of a book and that is the thing worth knowing.
	Pages int
	// Overfull and Underfull are boxes that did not fit. A few dozen in a volume
	// is ordinary typesetting; a few thousand is a document fighting its class.
	Overfull, Underfull int
	// MissingGlyphs is what XeTeX could not find a character for, by font. This
	// is the failure the whole of unicode.go exists to prevent, and it is
	// checked here rather than trusted, because the way it fails is silent.
	MissingGlyphs []string
	// Undefined is a control sequence the class does not define, which means the
	// writer and the class have got out of step.
	Undefined []string
	// Errors is every line the typesetter began with an exclamation mark, which
	// is how TeX marks an error and the only thing the different errors have in
	// common. Undefined control sequence used to be the only one of them this
	// read, and the two that got past it were Double superscript, out of a
	// primed base under a power, and Missing } inserted, out of a subscript
	// whose opening had been read twice. Both are as fatal to the output as an
	// undefined command and neither was reported.
	Errors []string
	// References that never resolved, which LaTeX prints as two question marks
	// and mentions once in the log.
	Unresolved int
}

// Tectonic is the typesetter. It is tectonic rather than a TeX Live
// installation because it fetches exactly the packages the document asks for
// and caches them, so the build is the same on a laptop with nothing installed
// as on one with four gigabytes of TeX Live, and because it runs XeTeX, which
// is what a document that has to set Vietnamese and Greek in the same paragraph
// needs.
const Tectonic = "tectonic"

// Options are the knobs on one run of the typesetter.
type Options struct {
	// Epoch is what SOURCE_DATE_EPOCH is pinned to. Two builds of the same
	// content have to produce the same bytes, because comparing this build's PDF
	// against the last one is how anybody would ever notice that a change to a
	// rendering rule moved four hundred pages.
	Epoch int64
	// Bundle is where tectonic looks for the TeX packages the class asks for,
	// and empty means its own default. It is a flag because the default is a
	// URL, and a machine that cannot reach it is not a rare thing: a corporate
	// resolver, a split-DNS setup, an air gap, or a build that wants to pin the
	// package versions rather than take whatever is current.
	Bundle string
	// Cached refuses to fetch anything, which is the setting for a build that
	// has to be reproducible against a cache somebody kept.
	Cached bool
}

// Run writes the document and the class into dir and typesets them.
func Run(ctx context.Context, dir string, d *Document, opt Options) (*Build, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, err
	}
	tex := filepath.Join(dir, "book.tex")
	if err := os.WriteFile(tex, []byte(d.TeX), 0o644); err != nil {
		return nil, err
	}
	if err := os.WriteFile(filepath.Join(dir, "bourbaki.cls"), []byte(Class), 0o644); err != nil {
		return nil, err
	}
	b := &Build{Dir: dir, PDF: filepath.Join(dir, "book.pdf"), Log: filepath.Join(dir, "book.log")}

	// The last run's PDF goes before this one starts. A compile that stops on a
	// TeX error writes no PDF, and what it leaves behind is whatever was there
	// before, which is a document that has nothing to do with the book.tex next
	// to it. Two volumes sat in the book repo that way, four page title pages
	// beside half a megabyte of manuscript, and the audit measured the
	// manuscript and called them 20 of 22. Removing it turns a stale artefact
	// into a missing one, and a missing one is loud.
	if err := os.Remove(b.PDF); err != nil && !os.IsNotExist(err) {
		return nil, err
	}

	args := []string{"-X", "compile", "--keep-logs", "--keep-intermediates", "--outdir", dir}
	if opt.Bundle != "" {
		args = append(args, "--bundle", opt.Bundle)
	}
	if opt.Cached {
		args = append(args, "--only-cached")
	}
	cmd := exec.CommandContext(ctx, Tectonic, append(args, tex)...)
	cmd.Env = append(os.Environ(),
		"SOURCE_DATE_EPOCH="+strconv.FormatInt(opt.Epoch, 10),
		"FORCE_SOURCE_DATE=1")
	out, err := cmd.CombinedOutput()
	// The log is read whether or not the run succeeded, because a failed run's
	// log is the only thing that says why, and a succeeded run's log is where
	// every one of the quiet failures is recorded.
	if lerr := b.readLog(); lerr != nil && err == nil {
		err = lerr
	}
	if err != nil {
		return b, fmt.Errorf("%s: %w\n%s", Tectonic, err, tail(string(out), 40))
	}
	if err := b.countPages(); err != nil {
		return b, err
	}
	return b, nil
}

var (
	overfullRE  = regexp.MustCompile(`^Overfull \\[hv]box`)
	underfullRE = regexp.MustCompile(`^Underfull \\[hv]box`)
	// The trailing ! is optional because the font name is not always on the line
	// the message starts on. XeTeX names a font by its file and its whole feature
	// string, that runs past the log's line width, and the ! ends up on the next
	// line. Requiring it meant every dropped character under the OpenType fonts
	// went unreported: the history volume lost four characters off six places and
	// the audit said none. See cpRE for the other half of the same wrapping.
	missingRE = regexp.MustCompile(`^Missing character: There is no (.+?) in font (.+?)!?$`)
	// The codepoint TeX prints beside the character it could not set. The log is
	// wrapped by byte and not by character, so a character of three bytes can be
	// cut in half by the wrap and reach us as mojibake. The codepoint is ASCII and
	// survives, so it is what the character is rebuilt from.
	cpRE        = regexp.MustCompile(`^(.*)\(U\+([0-9A-Fa-f]+)\)$`)
	undefinedRE = regexp.MustCompile(`^! Undefined control sequence`)
	errorRE     = regexp.MustCompile(`^! `)
	csRE        = regexp.MustCompile(`\\([A-Za-z@]+) $`)
	unresolvRE  = regexp.MustCompile(`LaTeX Warning: There were undefined references`)
	outputRE    = regexp.MustCompile(`Output written on .* \((\d+) pages?`)
)

// glyphName is how a character TeX could not set is named in the report. TeX
// gives the character and then its codepoint, and the character is the half that
// the log's line wrapping can cut in two, so where there is a codepoint the
// character is taken from that instead of from the bytes as they arrived.
func glyphName(s string) string {
	m := cpRE.FindStringSubmatch(strings.TrimSpace(s))
	if m == nil {
		return strings.TrimSpace(s)
	}
	n, err := strconv.ParseInt(m[2], 16, 32)
	if err != nil || !utf8.ValidRune(rune(n)) {
		return strings.TrimSpace(s)
	}
	return fmt.Sprintf("%c (U+%04X)", rune(n), rune(n))
}

// readLog reads the four things worth knowing out of a TeX log.
//
// A TeX log is a hundred thousand lines of which about twenty matter, and every
// one of the twenty is a line that TeX prints and then carries on regardless.
// That is the reason for reading it at all: the failures this is looking for do
// not stop the build, they ship.
func (b *Build) readLog() error {
	f, err := os.Open(b.Log)
	if err != nil {
		return nil // a run that never got as far as a log has already failed louder
	}
	defer f.Close()
	seenGlyph := map[string]bool{}
	seenCS := map[string]bool{}
	seenErr := map[string]bool{}
	s := bufio.NewScanner(f)
	s.Buffer(make([]byte, 0, 1<<20), 1<<22)
	undefinedNext := false
	for s.Scan() {
		line := s.Text()
		// Before the switch, because the undefined case below takes the line
		// out of the loop and an undefined control sequence is an error like
		// the rest of them. Deduplicated on the whole line, since one bad macro
		// in a chapter that is set twice for the table of contents prints the
		// same error twice and that is one fault, not two.
		if errorRE.MatchString(line) && !seenErr[line] {
			seenErr[line] = true
			b.Errors = append(b.Errors, strings.TrimSpace(line))
		}
		switch {
		case overfullRE.MatchString(line):
			b.Overfull++
		case underfullRE.MatchString(line):
			b.Underfull++
		case undefinedRE.MatchString(line):
			undefinedNext = true
			continue
		case unresolvRE.MatchString(line):
			b.Unresolved++
		}
		if m := missingRE.FindStringSubmatch(line); m != nil {
			key := glyphName(m[1]) + " in " + m[2]
			if !seenGlyph[key] {
				seenGlyph[key] = true
				b.MissingGlyphs = append(b.MissingGlyphs, key)
			}
		}
		if m := outputRE.FindStringSubmatch(line); m != nil {
			b.Pages, _ = strconv.Atoi(m[1])
		}
		if undefinedNext {
			undefinedNext = false
			if m := csRE.FindStringSubmatch(line); m != nil && !seenCS[m[1]] {
				seenCS[m[1]] = true
				b.Undefined = append(b.Undefined, `\`+m[1])
			}
		}
	}
	return s.Err()
}

// countPages falls back to reading the PDF when the log did not say. tectonic
// writes the "Output written on" line, but a run that was rerun for the table of
// contents can bury it, and the page count is the number the whole audit turns
// on.
func (b *Build) countPages() error {
	if b.Pages > 0 {
		return nil
	}
	raw, err := os.ReadFile(b.PDF)
	if err != nil {
		return err
	}
	// /Type /Page but not /Type /Pages, which is the tree node rather than a
	// leaf. Counting the leaves is exactly the page count for every PDF a TeX
	// engine writes, since none of them use an object stream for the page tree.
	n := 0
	for i := 0; i+10 < len(raw); i++ {
		if raw[i] != '/' || string(raw[i:i+5]) != "/Type" {
			continue
		}
		j := i + 5
		for j < len(raw) && (raw[j] == ' ' || raw[j] == '\r' || raw[j] == '\n') {
			j++
		}
		if j+5 <= len(raw) && string(raw[j:j+5]) == "/Page" {
			if j+6 > len(raw) || raw[j+5] != 's' {
				n++
			}
		}
	}
	b.Pages = n
	return nil
}

// Summary is the build's own report, the counterpart of the document's.
func (b *Build) Summary(v *Volume) string {
	var s strings.Builder
	fmt.Fprintf(&s, "%s: %d pages typeset", v.ID(), b.Pages)
	if v.Meta.Pages > 0 {
		fmt.Fprintf(&s, ", the printing has %d (%.0f%%)", v.Meta.Pages,
			100*float64(b.Pages)/float64(v.Meta.Pages))
	}
	fmt.Fprintf(&s, "\n  %d overfull boxes, %d underfull\n", b.Overfull, b.Underfull)
	if len(b.MissingGlyphs) > 0 {
		fmt.Fprintf(&s, "  %d characters with no glyph in their font\n", len(b.MissingGlyphs))
		for i, g := range b.MissingGlyphs {
			if i >= 8 {
				fmt.Fprintf(&s, "    and %d more\n", len(b.MissingGlyphs)-8)
				break
			}
			fmt.Fprintf(&s, "    %s\n", g)
		}
	}
	if len(b.Undefined) > 0 {
		fmt.Fprintf(&s, "  %d undefined control sequences: %s\n",
			len(b.Undefined), strings.Join(b.Undefined, " "))
	}
	if b.Unresolved > 0 {
		s.WriteString("  the document has references that never resolved\n")
	}
	return s.String()
}

func tail(s string, n int) string {
	lines := strings.Split(strings.TrimRight(s, "\n"), "\n")
	if len(lines) <= n {
		return s
	}
	return strings.Join(lines[len(lines)-n:], "\n")
}
