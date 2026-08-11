package extract

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// Every fixture in this file is real. The boxes and the font numbers were taken
// from what poppler prints for the 2023 volume, and the expected reading of each
// one was checked against the printed page at 150 dpi. A hand-made fixture would
// have agreed with whatever the code did on the day it was written, which is the
// one thing a test of this must not do.

// The display on page 115, which is the sum of the pairings. TeX sets the two
// limits of the sign so far above and below the line that their boxes clear the
// band of the formula altogether, and they arrive from poppler as lines of their
// own holding "n" and "i=1".
const displayXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="115" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="6" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="7" size="15" family="XJSZDJ+LMMathItalic10" color="#000000"/>
<fontspec id="8" size="10" family="CWSGRY+LMMathItalic7" color="#000000"/>
<fontspec id="9" size="15" family="XHUWIC+LMMathSymbols10" color="#000000"/>
<fontspec id="10" size="10" family="DCOIDG+LMRoman7" color="#000000"/>
<fontspec id="12" size="10" family="GORNHQ+LMMathSymbols7" color="#000000"/>
<fontspec id="13" size="15" family="XAEWAV+CMEX10" color="#000000"/>
<text top="478" left="280" width="7" height="9" font="8"><i>n</i></text>
<text top="489" left="273" width="22" height="7" font="13">X</text>
<text top="515" left="274" width="4" height="9" font="8"><i>i</i></text>
<text top="515" left="278" width="15" height="9" font="10">=1</text>
<text top="493" left="294" width="6" height="14" font="9">h</text>
<text top="494" left="300" width="24" height="13" font="7"><i>x, x</i></text>
<text top="490" left="324" width="6" height="10" font="12">&#8727;</text>
<text top="501" left="324" width="4" height="9" font="8"><i>i</i></text>
<text top="493" left="331" width="6" height="14" font="9">i</text>
<text top="494" left="337" width="9" height="13" font="7"><i>x</i></text>
<text top="501" left="345" width="4" height="9" font="8"><i>i</i></text>
<text top="494" left="354" width="12" height="13" font="6">=</text>
<text top="494" left="370" width="15" height="13" font="7"><i>x .</i></text>
</page>
</pdf2xml>
`

func TestDisplayLimits(t *testing.T) {
	lines := parse(t, displayXML)
	if len(lines) != 1 {
		for _, l := range lines {
			t.Logf("line [%d %d] %q", l.Top, l.Left, Render(l))
		}
		t.Fatalf("got %d lines, want the limits gathered onto the one line", len(lines))
	}
	const want = `$\sum_{i=1}^n\langle x, x^*_i\rangle x_i=x$.`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// A line of page 115 carrying a wide tilde. The accent is one CMEX glyph drawn
// over the P, and poppler reports the P in the middle of a run that runs on into
// the prose after it, so the accent has to cut the run rather than swallow it.
const accentXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="115" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="4" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="6" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="7" size="15" family="XJSZDJ+LMMathItalic10" color="#000000"/>
<fontspec id="9" size="15" family="XHUWIC+LMMathSymbols10" color="#000000"/>
<fontspec id="13" size="15" family="XAEWAV+CMEX10" color="#000000"/>
<text top="670" left="80" width="67" height="13" font="4">bimodules</text>
<text top="670" left="153" width="7" height="13" font="7"><i>&#950;</i></text>
<text top="670" left="166" width="21" height="13" font="6">: Q</text>
<text top="670" left="193" width="15" height="14" font="9">&#8594;</text>
<text top="676" left="214" width="8" height="7" font="13">e</text>
<text top="670" left="213" width="78" height="13" font="6">P such that</text>
</page>
</pdf2xml>
`

func TestAccentCutsRun(t *testing.T) {
	lines := parse(t, accentXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `bimodules $\zeta : Q\rightarrow \widetilde{P}$ such that`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Formula (14) of page 114 and the line of prose under it. The display reads
// Λ̃ = Θ ∘ (σ̃ ⊗ 1_P), and both of its accents were wrong before.
//
// The tilde over the sigma is at 257, which is too low to overlap the band of
// the display at 246 to 260 by half its height, so it arrives as a line of its
// own. It was then read as a piece of the prose at 268 and put over the y of
// "by relations", and the page went out saying "b~y relations". The tilde over
// the lambda is at 271, which is where the box of the bracket before the sigma
// ends, and it was put over the bracket.
const displayAccentXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="114" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="3" size="15" family="XJSZDJ+LMMathItalic10" color="#000000"/>
<fontspec id="4" size="15" family="XHUWIC+LMMathSymbols10" color="#000000"/>
<fontspec id="5" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="6" size="15" family="XAEWAV+CMEX10" color="#000000"/>
<fontspec id="7" size="10" family="DCOIDG+LMRoman7" color="#000000"/>
<fontspec id="10" size="10" family="GORNHQ+LMMathSymbols7" color="#000000"/>
<fontspec id="11" size="15" family="MJDJIW+LMRoman10" color="#0000ff"/>
<text top="252" left="271" width="8" height="7" font="6">e</text>
<text top="247" left="270" width="42" height="13" font="5">Λ = Θ</text>
<text top="246" left="315" width="7" height="14" font="4">◦</text>
<text top="247" left="326" width="6" height="13" font="5">(</text>
<text top="257" left="332" width="8" height="7" font="6">e</text>
<text top="247" left="332" width="9" height="13" font="3"><i>σ</i></text>
<text top="246" left="344" width="12" height="14" font="4">⊗</text>
<text top="247" left="359" width="7" height="13" font="5">1</text>
<text top="254" left="367" width="8" height="9" font="7">P</text>
<text top="247" left="375" width="6" height="13" font="5">)</text>
<text top="247" left="384" width="4" height="13" font="3"><i>.</i></text>
<text top="247" left="80" width="27" height="13" font="2">(14)</text>
<text top="268" left="110" width="22" height="13" font="2">For</text>
<text top="268" left="137" width="9" height="13" font="3"><i>x</i></text>
<text top="267" left="150" width="10" height="14" font="4">&#8712;</text>
<text top="268" left="164" width="14" height="13" font="5">P,</text>
<text top="268" left="184" width="9" height="13" font="3"><i>x</i></text>
<text top="265" left="193" width="6" height="10" font="10">&#8727;</text>
<text top="267" left="204" width="10" height="14" font="4">&#8712;</text>
<text top="268" left="218" width="10" height="13" font="5">P</text>
<text top="265" left="228" width="6" height="10" font="10">&#8727;</text>
<text top="268" left="235" width="34" height="13" font="2">, and</text>
<text top="268" left="274" width="7" height="13" font="3"><i>y</i></text>
<text top="267" left="286" width="10" height="14" font="4">&#8712;</text>
<text top="268" left="301" width="108" height="13" font="5">P, by relations (</text>
<text top="268" left="408" width="7" height="13" font="11">4</text>
<text top="268" left="416" width="21" height="13" font="2">), (</text>
<text top="268" left="437" width="7" height="13" font="11">8</text>
<text top="268" left="444" width="21" height="13" font="2">), (</text>
<text top="268" left="465" width="15" height="13" font="11">10</text>
<text top="268" left="480" width="50" height="13" font="2">), and (</text>
<text top="268" left="530" width="15" height="13" font="11">11</text>
<text top="268" left="545" width="32" height="13" font="2">), we</text>
</page>
</pdf2xml>
`

func TestAnAccentOfADisplayStaysOnTheDisplay(t *testing.T) {
	lines := parse(t, displayAccentXML)
	if len(lines) != 2 {
		for _, l := range lines {
			t.Logf("line [%d %d] %q", l.Top, l.Left, Render(l))
		}
		t.Fatalf("got %d lines, want the display and the prose under it", len(lines))
	}
	want := []string{
		`(14) $\widetilde{Λ} = Θ\circ (\widetilde{\sigma}\otimes 1_P)$.`,
		`For $x\in P,x^*\in P^*$, and $y\in P$, by relations (4), (8), (10), and (11), we`,
	}
	for i, w := range want {
		if got := Render(lines[i]); got != w {
			t.Errorf("Render line %d:\n got %s\nwant %s", i, got, w)
		}
	}
}

// The extent of every line of two pages of the volume, as poppler reports them:
// page 500, the first page of the index, which is set in two columns, and page
// 115, which is prose. Each line is the horizontal runs of ink on it, so the
// strip between the columns is what is missing from the first list and is not
// missing from the second.
//
// Page 500 is the hard case on purpose. Two of its entries run long enough to
// cross the strip, the title stands across the middle of the measure, and the
// two lines at the foot are a page number and a catch line. A test that asks for
// a strip with nothing in it at all passes on a page of columns that has none of
// those and fails on this one.
const indexPage = `183 212-446
377 80-181 355-402
395 80-157 368-435
413 368-422
431 80-94 368-403
449 80-204 355-429
468 93-160 368-410
486 93-160 355-477
504 80-303 355-485
525 80-185
539 80-127 355-368
560 93-229 355-471
578 93-254 355-463
596 93-288 355-451
614 80-236 355-408
632 80-150 368-446
650 80-123 368-407
668 93-240 355-418
686 93-169 355-483
704 93-167 355-510
724 93-171
740 93-157 355-369
760 93-175 355-490
778 93-154 355-462
796 93-176 355-562
814 93-170 355-396
833 93-179 368-435
851 93-180 368-440
869 93-157 355-437
887 93-155 355-410
930 81-281 561-579
943 81-401`

const prosePage = `56 80-139 183-515 560-578
88 80-577
128 80-209 216-579
149 79-580
171 78-336
194 110-473
215 110-578
234 80-223
259 110-134 141-453
280 79-304 311-578
303 79-458
326 110-578
346 79-354
371 110-580
390 80-578
411 80-578
431 80-128 144-577
443 121-140 152-156 186-190
457 80-146
478 280-287
489 273-346 354-385
501 324-328 345-349
515 274-293
540 80-580
561 80-578
583 80-230
605 110-550 557-578
626 80-577
649 80-580
670 80-291 298-580
691 80-581
713 80-578
734 80-579
757 80-128
797 80-209 216-579
819 79-366
842 110-334
860 110-316 323-333 341-371 378-434 442-455 462-486 494-580
885 79-249`

func TestGutter(t *testing.T) {
	at, ok := gutter(extents(t, indexPage))
	if !ok {
		t.Fatal("no gutter found on the index page")
	}
	// The last line of the left column ends at 303 and the first entry of the
	// right column starts at 355.
	if at < 303 || at > 355 {
		t.Errorf("gutter at %d, want it between the columns at 303 and 355", at)
	}
	if at, ok := gutter(extents(t, prosePage)); ok {
		t.Errorf("gutter found at %d on a page of prose", at)
	}
}

// parse reads a fixture the way a page of the volume is read.
func parse(t *testing.T, doc string) []Line {
	t.Helper()
	lay, err := pdfsrc.ParseXML(strings.NewReader(doc))
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}
	if len(lay.Pages) != 1 {
		t.Fatalf("got %d pages, want 1", len(lay.Pages))
	}
	return Lines(lay, lay.Pages[0])
}

// extents builds lines from the table above. Only the boxes matter to the test,
// so every stretch of ink is one run of body type.
func extents(t *testing.T, table string) []Line {
	t.Helper()
	var out []Line
	for _, row := range strings.Split(table, "\n") {
		f := strings.Fields(row)
		top := number(t, f[0])
		l := Line{Top: top, Bottom: top + 13}
		for i, seg := range f[1:] {
			from, to, _ := strings.Cut(seg, "-")
			r := Run{Spec: pdfsrc.FontSpec{Size: 15}, Class: ClassText}
			r.Top, r.Height = top, 13
			r.Left, r.Width = number(t, from), number(t, to)-number(t, from)
			l.Runs = append(l.Runs, r)
			if i == 0 {
				l.Left = r.Left
			}
			l.Right = r.Right()
		}
		out = append(out, l)
	}
	return out
}

func number(t *testing.T, s string) int {
	t.Helper()
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			t.Fatalf("not a number: %q", s)
		}
		n = n*10 + int(c-'0')
	}
	return n
}
