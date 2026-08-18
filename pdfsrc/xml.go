package pdfsrc

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"strconv"
	"strings"
)

// The 2023 volume was typeset in LaTeX and carries its whole font stack in the
// file: Latin Modern for the text, LMMathItalic for variables, CMEX for the
// large operators and delimiters, MSAM and MSBM for the AMS symbols, rsfs for
// the script letters, EUFM for the fraktur ones. pdftotext throws all of that
// away and leaves a page of letters in which a summation sign is the letter P
// on a line of its own.
//
// pdftohtml -xml keeps it. Every run of text comes out with its box and the
// font it was set in, which turns the two questions extraction has to answer
// into lookups rather than guesses: this run is mathematics because it is set
// in a mathematics font, and that stranded P is a summation sign because it is
// set in CMEX10. What is left to work out is the geometry, and the geometry is
// exact.

// Span is one run of text as pdftohtml reports it. The box is in the same
// pixel units as the page, with top and left measured from the top left corner.
type Span struct {
	Top, Left     int
	Width, Height int
	Font          int
	Text          string
	Italic        bool
	Bold          bool
	Link          bool // the run sits inside a cross reference
}

// Right is where the span ends horizontally.
func (s Span) Right() int { return s.Left + s.Width }

// Bottom is where the span ends vertically.
func (s Span) Bottom() int { return s.Top + s.Height }

// Mid is the vertical centre of the span, which is what lines are gathered on.
// A baseline would be better and pdftohtml does not report one.
func (s Span) Mid() float64 { return float64(s.Top) + float64(s.Height)/2 }

// FontSpec is one font as pdftohtml declares it. Family carries the subset tag
// the typesetter's toolchain generated, as in XAEWAV+CMEX10.
type FontSpec struct {
	ID     int
	Size   int
	Family string
	Color  string
}

// Base is the family without its subset tag, so that XAEWAV+CMEX10 and
// AQYXWT+CMEX7 can be told apart from each other and recognised as the same
// design at two sizes.
func (f FontSpec) Base() string {
	if _, rest, ok := strings.Cut(f.Family, "+"); ok {
		return rest
	}
	return f.Family
}

// Page is one page of spans, in the order pdftohtml emitted them, which is the
// order the glyphs were drawn and not the order they are read in.
type Page struct {
	Number        int
	Width, Height int
	Spans         []Span
	// Rules are the horizontal lines the page draws rather than sets in a
	// font. pdftohtml never reports them, so they are filled in by WithRules
	// from a second pass and are empty on a layout that did not ask for one.
	Rules []Rule
}

// Layout is a whole document, or the page range that was asked for.
type Layout struct {
	Fonts map[int]FontSpec
	Pages []Page
}

// Font returns the spec a span was set in.
func (l *Layout) Font(s Span) FontSpec { return l.Fonts[s.Font] }

// XML runs pdftohtml over a page range and parses what it prints. A range of 0
// to 0 means the whole document.
func (s *Source) XML(ctx context.Context, first, last int) (*Layout, error) {
	args := []string{"-xml"}
	if first > 0 {
		args = append(args, "-f", strconv.Itoa(first))
	}
	if last > 0 {
		args = append(args, "-l", strconv.Itoa(last))
	}
	args = append(args, s.Path, "-stdout")
	out, err := s.Run.Run(ctx, "pdftohtml", args...)
	if err != nil {
		return nil, err
	}
	return ParseXML(strings.NewReader(string(out)))
}

// ParseXML reads the document pdftohtml -xml writes.
//
// The parts of the file this does not keep are the outline and the stray "link
// to page 19" text poppler drops between pages, neither of which is on the page
// as a reader sees it.
func ParseXML(r io.Reader) (*Layout, error) {
	l := &Layout{Fonts: map[int]FontSpec{}}
	dec := xml.NewDecoder(r)
	dec.Strict = false
	var page *Page
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse pdftohtml xml: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			if end, ok := tok.(xml.EndElement); ok && end.Name.Local == "page" && page != nil {
				l.Pages = append(l.Pages, *page)
				page = nil
			}
			continue
		}
		switch start.Name.Local {
		case "page":
			page = &Page{
				Number: attrInt(start, "number"),
				Width:  attrInt(start, "width"),
				Height: attrInt(start, "height"),
			}
		case "fontspec":
			f := FontSpec{
				ID:     attrInt(start, "id"),
				Size:   attrInt(start, "size"),
				Family: attr(start, "family"),
				Color:  attr(start, "color"),
			}
			l.Fonts[f.ID] = f
		case "text":
			if page == nil {
				if err := dec.Skip(); err != nil {
					return nil, err
				}
				continue
			}
			spans, err := readText(dec, start)
			if err != nil {
				return nil, err
			}
			page.Spans = append(page.Spans, spans...)
		case "outline", "item":
			if err := dec.Skip(); err != nil {
				return nil, err
			}
		}
	}
	if page != nil {
		l.Pages = append(l.Pages, *page)
	}
	return l, nil
}

// readText reads one text element and everything nested in it.
//
// The element is mixed content: a cross reference comes out as
// <text>(VIII, p. <a href="...">2</a>)</text> and an emphasised phrase as
// <text><i>Let</i></text>, sometimes both at once. Each stretch under a
// different set of tags becomes its own span, because "which of these words
// were italic" is the question that tells a statement from the prose around it.
// pdftohtml reports one box for the whole element and none for the parts, so the
// box is shared out between them by how many characters each holds.
func readText(dec *xml.Decoder, start xml.StartElement) ([]Span, error) {
	base := Span{
		Top:    attrInt(start, "top"),
		Left:   attrInt(start, "left"),
		Width:  attrInt(start, "width"),
		Height: attrInt(start, "height"),
		Font:   attrInt(start, "font"),
	}
	var (
		spans []Span
		cur   = base
		buf   strings.Builder
		depth int
	)
	flush := func() {
		if buf.Len() == 0 {
			return
		}
		cur.Text = buf.String()
		spans = append(spans, cur)
		buf.Reset()
	}
	for {
		tok, err := dec.Token()
		if err != nil {
			return nil, err
		}
		switch t := tok.(type) {
		case xml.CharData:
			buf.Write(t)
		case xml.StartElement:
			flush()
			depth++
			switch t.Name.Local {
			case "i":
				cur.Italic = true
			case "b":
				cur.Bold = true
			case "a":
				cur.Link = true
			}
		case xml.EndElement:
			if depth == 0 {
				flush()
				if len(spans) == 0 {
					// An element with no characters in it at all is a glyph
					// poppler drew and could not name, and the box is where it
					// stood. Lie chapters 7 to 9 names code 0x17 of its
					// mathematics italic pi1, which is varpi, and wrote every
					// one of them as <text ...><i></i></text>: dropping the
					// element here is what made that loss invisible to
					// everything downstream, since a page cannot be flagged
					// for a run it never sees.
					return []Span{base}, nil
				}
				return apportion(spans, base), nil
			}
			flush()
			depth--
			switch t.Name.Local {
			case "i":
				cur.Italic = false
			case "b":
				cur.Bold = false
			case "a":
				cur.Link = false
			}
		}
	}
}

// apportion shares one element's box out between the spans read from it.
//
// A cross reference splits an element in two and pdftohtml reports the box of
// the element and nothing about the halves, so both halves would start where the
// element starts and end where it ends. Everything downstream is geometry: lines
// are gathered by overlap, order is left to right, a wide gap is a space. Giving
// each part the width of the characters in it is an estimate, and an estimate in
// the right place beats the same box repeated.
func apportion(spans []Span, base Span) []Span {
	if len(spans) < 2 {
		return spans
	}
	total := 0
	for _, s := range spans {
		total += len([]rune(s.Text))
	}
	if total == 0 {
		return spans
	}
	seen := 0
	for i := range spans {
		n := len([]rune(spans[i].Text))
		spans[i].Left = base.Left + base.Width*seen/total
		spans[i].Width = base.Width * n / total
		seen += n
	}
	return spans
}

func attr(e xml.StartElement, name string) string {
	for _, a := range e.Attr {
		if a.Name.Local == name {
			return a.Value
		}
	}
	return ""
}

func attrInt(e xml.StartElement, name string) int {
	n, _ := strconv.Atoi(attr(e, name))
	return n
}
