package pdfsrc

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// pdftohtml reports a run and not a word, and a run of prose is as long as the
// stretch of one font the line is set in. Page 177 of Algebra VIII hands back
// "Let P be a projective A-module, and let P be the A" as a single box 356
// pixels wide, and nothing in the layer says where inside it the second P is.
//
// That is enough for reading the text and not enough for placing a bar. TeX
// draws the bar of a closure over the letter it covers and nothing else, so the
// bar over that second P is ten pixels wide and falls in the middle of the box.
// bar.go can see the rule and can see the run, and has no way to say that the
// rule covers one letter of it. It gives the bar up, and the page goes out
// saying that a module is zero if and only if it is zero.
//
// pdftotext -bbox reports the same page one word at a time, with a box on each,
// and poppler already runs over every page. So the words are a second reading of
// the same text layer at a finer grain, and where the two agree about what is
// written they also agree about where each word of it stands.

// Word is one word of a page as pdftotext -bbox reports it.
//
// The box is in the same pixel units as Span, so a word and a run and a rule can
// be compared without any of them being converted first.
type Word struct {
	Left, Top, Width, Height int
	Text                     string
}

// Right is where the word ends horizontally.
func (w Word) Right() int { return w.Left + w.Width }

// Bottom is where the word ends vertically.
func (w Word) Bottom() int { return w.Top + w.Height }

// Words runs pdftotext -bbox over one page and returns the words of it, in the
// pixel units of a page that many pixels wide.
//
// One page per call, the way Rules works, so that a caller holding a page has
// one thing to ask and the page number it already has to ask it with.
func (s *Source) Words(ctx context.Context, page, width int) ([]Word, error) {
	out, err := s.Run.Run(ctx, "pdftotext", "-bbox", "-f", strconv.Itoa(page), "-l", strconv.Itoa(page), s.Path, "-")
	if err != nil {
		return nil, fmt.Errorf("pdftotext -bbox page %d: %w", page, err)
	}
	return ParseBBox(strings.NewReader(string(out)), width)
}

// WithWords fills in the words of every page of a layout.
//
// A second pass over the same file, for the reason WithRules gives: a caller
// that only wants the fonts of a volume should not pay for it.
func (s *Source) WithWords(ctx context.Context, l *Layout) error {
	for i := range l.Pages {
		words, err := s.Words(ctx, l.Pages[i].Number, l.Pages[i].Width)
		if err != nil {
			return err
		}
		l.Pages[i].Words = words
	}
	return nil
}

// ParseBBox reads the XHTML pdftotext -bbox writes and returns the words of the
// first page in it, scaled to a page width pixels wide.
//
// The boxes come in points and Span comes in the pixels pdftohtml chose, so
// every one is scaled by the ratio of the two widths. That is the same
// conversion ParseSVG makes and it holds for the same reason: both programs
// report the same page, so the ratio of the widths is the ratio of everything.
func ParseBBox(r io.Reader, width int) ([]Word, error) {
	raw, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("read pdftotext bbox: %w", err)
	}
	dec := xml.NewDecoder(strings.NewReader(scrub(string(raw))))
	dec.Strict = false
	dec.AutoClose = xml.HTMLAutoClose
	dec.Entity = xml.HTMLEntity
	scale := 0.0
	var out []Word
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse pdftotext bbox: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "page":
			if len(out) > 0 {
				// A second page in the same output. One call asks for one
				// page, so this is a caller that asked for a range, and the
				// pages after the first belong to nobody here.
				return out, nil
			}
			wpt := attrFloat(start, "width")
			if wpt > 0 && width > 0 {
				scale = float64(width) / wpt
			}
		case "word":
			if scale == 0 {
				continue
			}
			var text string
			if err := dec.DecodeElement(&text, &start); err != nil {
				return nil, fmt.Errorf("parse pdftotext bbox: %w", err)
			}
			x0 := attrFloat(start, "xMin") * scale
			y0 := attrFloat(start, "yMin") * scale
			x1 := attrFloat(start, "xMax") * scale
			y1 := attrFloat(start, "yMax") * scale
			if x1 <= x0 || text == "" {
				continue
			}
			out = append(out, Word{
				Left:   round(x0),
				Top:    round(y0),
				Width:  round(x1) - round(x0),
				Height: round(y1) - round(y0),
				Text:   text,
			})
		}
	}
	return out, nil
}

func attrFloat(e xml.StartElement, name string) float64 {
	f, _ := strconv.ParseFloat(attr(e, name), 64)
	return f
}

func round(f float64) int {
	if f < 0 {
		return int(f - 0.5)
	}
	return int(f + 0.5)
}

// scrub takes out the characters XML has no way of carrying.
//
// A glyph the font maps to nothing comes out of pdftotext as the code it has in
// the font, and a font is free to put a glyph at any code it likes. Algebra VIII
// puts one at U+000F, which XML 1.0 forbids outright and has no escape for, so
// the parser stops on the page rather than on the word. Dropping the character
// leaves the word spelling something the run does not, and a word that does not
// spell what the run says is one sunder leaves alone, so the page comes out the
// way it did before instead of not at all.
func scrub(s string) string {
	bad := func(c rune) bool {
		switch {
		case c == '\t' || c == '\n' || c == '\r':
			return false
		case c < 0x20, c >= 0x7f && c <= 0x9f, c == 0xfffe, c == 0xffff:
			return true
		}
		return false
	}
	if !strings.ContainsFunc(s, bad) {
		return s
	}
	return strings.Map(func(c rune) rune {
		if bad(c) {
			return -1
		}
		return c
	}, s)
}
