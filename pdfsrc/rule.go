package pdfsrc

import (
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strconv"
	"strings"
)

// Not everything on the page is set in a font. TeX draws a rule wherever it
// wants a line of a given width and height, and a rule is not a character: it
// carries no code point, belongs to no font, and appears in no encoding. The
// fraction bar is one. So is the bar Bourbaki sets between the two operands of
// a set difference, which is what sent this file to be written.
//
// pdftohtml is blind to all of it. It reports the text layer and a rule is not
// in the text layer, so a page that prints I minus J comes out of pdftohtml as
// the two letters with a hole between them and no word about what was in the
// hole. Chapter VIII prints that operator 70 times in the English and 69 in the
// French, and until this was written the corpus had 14 of them, the ones a
// reader happened to notice.
//
// pdftocairo does see it. Its SVG keeps the page as it is drawn: glyphs go into
// <defs> and are placed with <use>, and everything the page draws itself stays
// in the body as a path. So the rules are exactly the paths in the body, and
// reading them is reading an SVG rather than guessing at a gap.

// Rule is a horizontal line the page draws rather than sets in a font.
//
// The box is in the same pixel units as Span, so a rule and a run can be
// compared without either of them being converted first. Thickness and Length
// stay in points, because they are what says which rule this is: a fraction bar
// and a set difference sign are both horizontal lines and only their weight
// tells them apart, and weight is a property of the type and not of the
// resolution somebody rendered at.
type Rule struct {
	Top, Left, Width int
	Thickness        float64
	Length           float64
	// Size is the type size the rule was drawn at, in the units pdftohtml
	// reports a font size in, so that a rule and a font can be compared
	// without either being converted first.
	//
	// TeX draws a rule at a fixed fraction of the em of the size it is set in,
	// and Algebra VIII sets these in four sizes: the weights measured are 1.457
	// points at 10 point, 1.310 at 9, 0.943 at 7 and 0.850 at 6, which is
	// 0.1457, 0.1456, 0.1347 and 0.1417 of the em. So the weight of a rule says
	// what size it belongs to, to within a point, and that is what says which
	// runs on a line are its operands and which are a superscript over it.
	Size float64
}

// emRule is the fraction of the em TeX draws these rules at, as measured over
// the four sizes Algebra VIII uses.
const emRule = 0.145

// Right is where the rule ends horizontally.
func (r Rule) Right() int { return r.Left + r.Width }

// Mid is the horizontal centre of the rule.
func (r Rule) Mid() int { return r.Left + r.Width/2 }

// Rules runs pdftocairo over one page and returns the horizontal lines drawn on
// it, in the pixel units of a page that many pixels wide.
//
// One page per call, because pdftocairo -svg writes the first page of the range
// and stops. It costs about sixty milliseconds a page, which is half a minute
// over a volume, against the several minutes the reading itself takes.
func (s *Source) Rules(ctx context.Context, page, width int) ([]Rule, error) {
	out, err := s.Run.Run(ctx, "pdftocairo", "-svg", "-f", strconv.Itoa(page), "-l", strconv.Itoa(page), s.Path, "-")
	if err != nil {
		return nil, fmt.Errorf("pdftocairo page %d: %w", page, err)
	}
	return ParseSVG(strings.NewReader(string(out)), width)
}

// WithRules fills in the drawn rules of every page of a layout.
//
// It is a second pass over the same file rather than part of XML, because the
// two programs disagree about what a page range is and because a caller that
// only wants to know which fonts a volume uses should not pay for it.
func (s *Source) WithRules(ctx context.Context, l *Layout) error {
	for i := range l.Pages {
		rules, err := s.Rules(ctx, l.Pages[i].Number, l.Pages[i].Width)
		if err != nil {
			return err
		}
		l.Pages[i].Rules = rules
	}
	return nil
}

// ParseSVG reads the SVG pdftocairo writes for one page and returns the
// horizontal rules in it, scaled to a page width pixels wide.
//
// <defs> is skipped whole. It holds one <g> per glyph the page uses, and every
// glyph is a filled path, so a reader that did not skip it would come back with
// the outline of every letter on the page and call each one a rule.
func ParseSVG(r io.Reader, width int) ([]Rule, error) {
	dec := xml.NewDecoder(r)
	dec.Strict = false
	scale := 0.0
	var out []Rule
	for {
		tok, err := dec.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("parse pdftocairo svg: %w", err)
		}
		start, ok := tok.(xml.StartElement)
		if !ok {
			continue
		}
		switch start.Name.Local {
		case "svg":
			wpt := lengthPt(attr(start, "width"))
			if wpt > 0 && width > 0 {
				scale = float64(width) / wpt
			}
		case "defs":
			if err := dec.Skip(); err != nil {
				return nil, err
			}
		case "path":
			if scale == 0 {
				continue
			}
			if rule, ok := readPath(start, scale); ok {
				out = append(out, rule)
			}
		}
	}
	return out, nil
}

// readPath turns one path element into a rule, where it is one.
//
// Two shapes arrive and they are the same line drawn two ways. Below about
// eight points cairo strokes it, as a two point segment with a stroke width and
// a transform; above that it fills it, as a closed rectangle with no transform
// at all. Which one a given bar gets is a decision inside cairo and nothing on
// the page, so both have to be read and both come back as the same Rule.
func readPath(start xml.StartElement, scale float64) (Rule, bool) {
	pts, ok := points(attr(start, "d"))
	if !ok || len(pts) < 2 {
		return Rule{}, false
	}
	if attr(start, "fill") == "none" {
		w, err := strconv.ParseFloat(attr(start, "stroke-width"), 64)
		if err != nil || w <= 0 || len(pts) != 2 {
			return Rule{}, false
		}
		a, d, e, f, ok := matrix(attr(start, "transform"))
		if !ok {
			return Rule{}, false
		}
		x0, x1 := a*pts[0].x+e, a*pts[1].x+e
		y0, y1 := d*pts[0].y+f, d*pts[1].y+f
		if math.Abs(y1-y0) > 0.05 {
			return Rule{}, false
		}
		// The stroke is centred on the segment, so the top of the line it
		// paints is half a width above the segment itself.
		th := w * math.Abs(a)
		return rule(math.Min(x0, x1), math.Max(x0, x1)-math.Min(x0, x1), y0-th/2, th, scale), true
	}
	if attr(start, "fill") == "" || len(pts) < 4 {
		return Rule{}, false
	}
	minx, maxx := pts[0].x, pts[0].x
	miny, maxy := pts[0].y, pts[0].y
	for _, p := range pts[1:] {
		minx, maxx = math.Min(minx, p.x), math.Max(maxx, p.x)
		miny, maxy = math.Min(miny, p.y), math.Max(maxy, p.y)
	}
	w, h := maxx-minx, maxy-miny
	if h <= 0 || w <= 2*h {
		return Rule{}, false
	}
	return rule(minx, w, miny, h, scale), true
}

func rule(x, length, top, thickness, scale float64) Rule {
	return Rule{
		Top:       int(math.Round(top * scale)),
		Left:      int(math.Round(x * scale)),
		Width:     int(math.Round(length * scale)),
		Thickness: thickness,
		Length:    length,
		Size:      thickness / emRule * scale,
	}
}

type point struct{ x, y float64 }

// points reads a path made of absolute moves, lines and closes, and refuses
// anything else. A curve in the data means the path is not a rule and the
// caller wants nothing to do with it, so refusing is the answer rather than an
// approximation of where the curve went.
func points(d string) ([]point, bool) {
	fields := strings.Fields(d)
	var out []point
	for i := 0; i < len(fields); {
		switch fields[i] {
		case "M", "L":
			if i+2 >= len(fields) {
				return nil, false
			}
			x, err1 := strconv.ParseFloat(fields[i+1], 64)
			y, err2 := strconv.ParseFloat(fields[i+2], 64)
			if err1 != nil || err2 != nil {
				return nil, false
			}
			out = append(out, point{x, y})
			i += 3
		case "Z":
			i++
		default:
			return nil, false
		}
	}
	return out, true
}

// matrix reads the transform cairo puts on a stroked path. Only the diagonal
// form is read: a rotated or sheared rule is not a horizontal line whatever its
// coordinates say, and there are none in these volumes.
func matrix(s string) (a, d, e, f float64, ok bool) {
	if s == "" {
		return 1, 1, 0, 0, true
	}
	inner, ok := strings.CutPrefix(strings.TrimSpace(s), "matrix(")
	if !ok {
		return 0, 0, 0, 0, false
	}
	inner, ok = strings.CutSuffix(strings.TrimSpace(inner), ")")
	if !ok {
		return 0, 0, 0, 0, false
	}
	var v [6]float64
	parts := strings.Split(inner, ",")
	if len(parts) != 6 {
		return 0, 0, 0, 0, false
	}
	for i, p := range parts {
		n, err := strconv.ParseFloat(strings.TrimSpace(p), 64)
		if err != nil {
			return 0, 0, 0, 0, false
		}
		v[i] = n
	}
	if v[1] != 0 || v[2] != 0 {
		return 0, 0, 0, 0, false
	}
	return v[0], v[3], v[4], v[5], true
}

// lengthPt reads an SVG length that is in points, as pdftocairo writes it.
func lengthPt(s string) float64 {
	n, err := strconv.ParseFloat(strings.TrimSuffix(s, "pt"), 64)
	if err != nil {
		return 0
	}
	return n
}
