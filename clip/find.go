package clip

import (
	"strings"

	"github.com/tamnd/bourbaki-solver/extract"
	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// Find picks the lines of a volume a query asks for and returns them as targets.
//
// The reading it records is the extractor's, taken from the same layout the box
// is taken from, so a clip and the line it argues with are one act. Anything
// that reads the index later is comparing a model against a reading that is
// pinned rather than against whatever the extractor says by the time the answers
// come back, which on this pipeline is days.
func Find(layout *pdfsrc.Layout, query Query) []Target {
	var out []Target
	var seen int
	for _, page := range layout.Pages {
		if !query.InRange(page.Number) {
			continue
		}
		lines, _ := extract.LinesColumns(layout, page)
		for index, line := range lines {
			text := extract.Render(line)
			if query.Match != nil && !query.Match.MatchString(text) {
				continue
			}
			keep := query.Keep(text, seen)
			seen++
			if !keep {
				continue
			}
			out = append(out, Target{
				Page: page.Number, Line: index, Name: Name(page.Number, index),
				Native: text, Box: BoxOf(line),
			})
			if query.Limit > 0 && len(out) >= query.Limit {
				return out
			}
		}
	}
	return out
}

// FindPages picks whole pages rather than lines.
//
// A page is the better unit and the first run of this route is what says so. A
// line went to the model with nothing around it, and a model given six words of
// French and a formula fills the gap with something plausible: it read an
// interior as a closure, it turned a rho into a q, and on one line it dropped
// the bar of a not-equals and wrote that y0 was zero where the page says it is
// not. None of those are things a reader of the page could do. The page carries
// its own context, so the model is answering a question it can answer.
//
// The other half of it is the cost. A clip and a page are one model call each,
// which is three or four minutes either way, and a page is forty lines. Reading
// Théories spectrales a line at a time is a year of fleet time and reading it a
// page at a time is a week.
//
// native says what the extractor already has for a page, which for these
// volumes is the page in the corpus: the body of it, and the running head and
// folio the front matter keeps out of the body. A page it has nothing for is
// skipped rather than compared against the empty string.
func FindPages(layout *pdfsrc.Layout, query Query, native func(page int) (body, head string)) []Target {
	var out []Target
	var seen int
	for _, page := range layout.Pages {
		if !query.InRange(page.Number) {
			continue
		}
		text, head := native(page.Number)
		if strings.TrimSpace(text) == "" {
			continue
		}
		if query.Match != nil && !query.Match.MatchString(text) {
			continue
		}
		keep := query.Keep(text, seen)
		seen++
		if !keep {
			continue
		}
		box, ok := BlockOf(page)
		if !ok {
			continue
		}
		out = append(out, Target{
			Page: page.Number, Line: WholePage, Name: PageName(page.Number),
			Native: text, Head: head, Box: box,
		})
		if query.Limit > 0 && len(out) >= query.Limit {
			return out
		}
	}
	return out
}

// BlockOf is the ink of a whole page.
//
// The margins are cut away rather than rendered, which is the one thing a clip
// of a page does that a render of one does not. Théories spectrales sets a text
// block about two thirds of the height of its paper, so cutting to the ink puts
// half again as many pixels on every letter for the same number of bytes
// uploaded, and the type is what the model is being asked to read.
//
// The running head and the folio are inside this box and stay there. They are
// ink on the page, the prompt says what to do with them, and a box drawn to
// exclude them would have to know where they are, which is the extractor's job
// and not a crop's.
func BlockOf(page pdfsrc.Page) (Box, bool) {
	if len(page.Spans) == 0 {
		return Box{}, false
	}
	box := Box{Left: page.Width, Top: page.Height}
	for _, span := range page.Spans {
		if strings.TrimSpace(span.Text) == "" {
			continue
		}
		box.Left = min(box.Left, span.Left)
		box.Right = max(box.Right, span.Right())
		box.Top = min(box.Top, span.Top)
		box.Bottom = max(box.Bottom, span.Bottom())
	}
	if box.Right <= box.Left || box.Bottom <= box.Top {
		return Box{}, false
	}
	return box, true
}

// BoxOf is the ink of one line.
//
// The line's own top and bottom are the band of the body type, which is what
// says whether a run is an exponent and is not what a clip wants: the exponent
// is above that band and the index below it, and a clip cut to the band is a
// clip of a formula with its indices sliced off. So the box is taken over every
// run in the line, and the padding on top of it is what keeps the accents.
func BoxOf(line extract.Line) Box {
	box := Box{Left: line.Left, Right: line.Right, Top: line.Top, Bottom: line.Bottom}
	for _, run := range line.Runs {
		box.Left = min(box.Left, run.Left)
		box.Right = max(box.Right, run.Right())
		box.Top = min(box.Top, run.Top)
		box.Bottom = max(box.Bottom, run.Bottom())
	}
	return box
}

// Refresh takes an index and replaces the reading it pinned with what the
// extractor says today, for the whole pages in it.
//
// The pinning is deliberate and stays: a comparison is of two readings and the
// one the pictures were actually argued against is the one that was current
// when they were cut. But a cut is worth reading twice, once when the answers
// arrive and again after the extractor has been fixed, and the second reading
// is the one that says whether the disagreement still stands. Eight pages of
// Lie 7 to 9 were cut before the spacing accents landed and six of them
// disagreed; five of the six disagreed about "Poincar e-Birkhoff-Witt" and
// "Obˇsˇc", which no longer exist, and the audit had no way to say so.
//
// A page the reader has nothing for keeps what it had, because an empty body
// is a page that has not been extracted rather than a page that now reads
// nothing, and a line keeps what it had because a line number is an index into
// a layout and the layout moves.
func (i Index) Refresh(native func(page int) (body, head string)) Index {
	out := i
	out.Targets = make([]Target, len(i.Targets))
	copy(out.Targets, i.Targets)
	for at, target := range out.Targets {
		if !target.Whole() {
			continue
		}
		body, head := native(target.Page)
		if strings.TrimSpace(body) == "" {
			continue
		}
		out.Targets[at].Native, out.Targets[at].Head = body, head
	}
	return out
}
