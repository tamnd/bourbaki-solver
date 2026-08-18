package extract

import "testing"

// Page 289 of Topologie algébrique, the proof of lemma 5, written out of the
// runs the page hands back. The V measures 518 to 530, the 2 drawn over it 523
// to 529 and nine units above its top, and the prime 531 to 534 at the top of
// the V to the unit. The same line prints an ordinary V prime earlier on, which
// nothing here should touch.
const composedEntourage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="289" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="16" family="PYGVNS+LMRoman10" color="#000000"/>
<fontspec id="6" size="16" family="BMXWVH+LMRoman10" color="#000000"/>
<fontspec id="8" size="12" family="BXXFTB+LMMathSymbols8" color="#000000"/>
<fontspec id="9" size="12" family="GTNDLC+LMRoman8" color="#000000"/>
<fontspec id="10" size="16" family="BRGRHX+LMMathSymbols10" color="#000000"/>
<text top="378" left="81" width="95" height="21" font="2">un entourage</text>
<text top="378" left="183" width="12" height="21" font="6">V</text>
<text top="378" left="195" width="3" height="11" font="8">0</text>
<text top="378" left="206" width="305" height="21" font="2">de cette même structure uniforme tel que</text>
<text top="378" left="518" width="12" height="21" font="6">V</text>
<text top="369" left="523" width="6" height="11" font="9">2</text>
<text top="378" left="531" width="3" height="11" font="8">0</text>
<text top="381" left="542" width="13" height="15" font="10">⊂</text>
<text top="378" left="562" width="12" height="21" font="6">V</text>
<text top="378" left="574" width="5" height="21" font="2">.</text>
</page>
</pdf2xml>
`

// The same line with the 2 set where an exponent goes, which is what the rest
// of the library prints and what nothing here may touch. The 2 now begins where
// the V ends, so it is not drawn back across the letter.
const exponentEntourage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="289" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="2" size="16" family="PYGVNS+LMRoman10" color="#000000"/>
<fontspec id="6" size="16" family="BMXWVH+LMRoman10" color="#000000"/>
<fontspec id="9" size="12" family="GTNDLC+LMRoman8" color="#000000"/>
<fontspec id="10" size="16" family="BRGRHX+LMMathSymbols10" color="#000000"/>
<text top="378" left="81" width="61" height="21" font="2">tel que</text>
<text top="378" left="518" width="12" height="21" font="6">V</text>
<text top="369" left="530" width="6" height="11" font="9">2</text>
<text top="381" left="542" width="13" height="15" font="10">⊂</text>
<text top="378" left="562" width="12" height="21" font="6">V</text>
<text top="378" left="574" width="5" height="21" font="2">.</text>
</page>
</pdf2xml>
`

func TestAnExponentIsLeftAlone(t *testing.T) {
	lines := parse(t, exponentEntourage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `tel que $V^2\subset V$.`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

func TestAScriptDrawnOverASymbolAfterIt(t *testing.T) {
	lines := parse(t, composedEntourage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `un entourage $V'$ de cette même structure uniforme tel que $\overset{2}{V'}\subset V$.`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}
