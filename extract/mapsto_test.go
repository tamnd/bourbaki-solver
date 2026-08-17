package extract

import "testing"

// Page 309 of Topologie algébrique, the note that composes two paths. The book
// prints "L'application (c, d) 7→ c ∗ d de", and poppler prints the hook of the
// mapsto as a run of its own: zero units wide, in the blue of a hyperlink, at
// the same left edge as the arrow that follows it in black. Read run by run the
// hook is a \mapstochar and the arrow a \rightarrow, and the line shipped
// "$(c, d)\mapstochar \rightarrow c*d$".
const splitMapsXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="309" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="12" size="15" family="IWFIMB+PygvnsBslxqcLMRoman10" color="#000000"/>
<fontspec id="13" size="15" family="IWFIMB+BmxwvhBwyvtgLMRoman10" color="#000000"/>
<fontspec id="14" size="15" family="AQKDGP+QwxclbXgkcwbLMMathItalic10" color="#000000"/>
<fontspec id="15" size="15" family="XAAOWT+BrgrhxHhwpyxLMMathSymbols10" color="#0000ff"/>
<fontspec id="16" size="15" family="XAAOWT+BrgrhxHhwpyxLMMathSymbols10" color="#000000"/>
<text top="496" left="117" width="86" height="19" font="12">L’application</text>
<text top="496" left="210" width="6" height="19" font="13">(</text>
<text top="499" left="216" width="21" height="13" font="14"><i>c, d</i></text>
<text top="496" left="237" width="6" height="19" font="13">)</text>
<text top="499" left="251" width="0" height="14" font="15">7</text>
<text top="499" left="251" width="15" height="14" font="16">→</text>
<text top="499" left="274" width="6" height="13" font="14"><i>c</i></text>
<text top="499" left="285" width="7" height="14" font="16">∗</text>
<text top="499" left="298" width="8" height="13" font="14"><i>d</i></text>
<text top="496" left="313" width="15" height="19" font="12">de</text>
</page>
</pdf2xml>
`

func TestAHookAndAnArrowInTwoRunsAreOneMapsto(t *testing.T) {
	lines := parse(t, splitMapsXML)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `L’application $(c, d)\mapsto c*d$ de`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}
