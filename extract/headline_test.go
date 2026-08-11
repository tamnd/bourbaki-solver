package extract

import (
	"strings"
	"testing"
)

// A title too long for the measure is set on two lines and is still one title.
// The volume does it twice for a §, on pages 42 and 112, and over each of the
// four appendices, which print the number on one line and the name on the next.
// Read as two headings the § of page 42 came out as "§ 2. THE STRUCTURE OF
// MODULES OF FINITE" with a subsection called "LENGTH" under it.

// Page 42, the head of § 2 and the first subsection under it.
const twoLineHeadXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="42" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="0" size="15" family="GSFMFK+LMRoman10" color="#000000"/>
<text top="184" left="118" width="27" height="13" font="0"><b>§ 2.</b></text>
<text top="184" left="162" width="377" height="13" font="0"><b>THE STRUCTURE OF MODULES OF FINITE</b></text>
<text top="205" left="292" width="74" height="13" font="0"><b>LENGTH</b></text>
<text top="350" left="80" width="110" height="13" font="0"><b>1. Local Rings</b></text>
</page>
</pdf2xml>
`

// Page 480, the head of the fourth appendix and the first subsection under it.
// The number stands on a line of its own and the name is 29 units below it,
// where the second line of § 2 is 21.
const appendixHeadXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="480" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="0" size="15" family="GSFMFK+LMRoman10" color="#000000"/>
<text top="180" left="275" width="108" height="13" font="0"><b>APPENDIX 4</b></text>
<text top="209" left="107" width="443" height="13" font="0"><b>TRACE OF AN ENDOMORPHISM OF FINITE RANK</b></text>
<text top="372" left="80" width="264" height="13" font="0"><b>1. Linear Mappings of Finite Rank</b></text>
</page>
</pdf2xml>
`

// Page 18, where the chapter number and the chapter title are two headings and
// not one. They are 64 units apart and set in different sizes.
const chapterHeadXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="18" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="0" size="12" family="GSFMFK+LMRoman10" color="#000000"/>
<fontspec id="1" size="16" family="GSFMFK+LMRoman10" color="#000000"/>
<text top="89" left="270" width="118" height="12" font="0"><b>CHAPTER VIII</b></text>
<text top="153" left="191" width="276" height="16" font="1"><b>Semisimple Modules and Rings</b></text>
</page>
</pdf2xml>
`

func heads(t *testing.T, doc string) []string {
	t.Helper()
	var out []string
	for _, p := range strings.Split(blocks(parse(t, doc), Volume{}), "\n\n") {
		if strings.HasPrefix(p, "#") {
			out = append(out, p)
		}
	}
	return out
}

func TestASectionTitleSetOnTwoLinesIsOneTitle(t *testing.T) {
	got := heads(t, twoLineHeadXML)
	want := []string{"## § 2. THE STRUCTURE OF MODULES OF FINITE LENGTH", "### 1. Local Rings"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("headings = %q, want %q", got, want)
	}
}

func TestAnAppendixIsASectionAndKeepsItsName(t *testing.T) {
	got := heads(t, appendixHeadXML)
	want := []string{"## APPENDIX 4 TRACE OF AN ENDOMORPHISM OF FINITE RANK", "### 1. Linear Mappings of Finite Rank"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Errorf("headings = %q, want %q", got, want)
	}
}

func TestTheChapterTitleIsNotAContinuation(t *testing.T) {
	if got := heads(t, chapterHeadXML); len(got) != 2 {
		t.Errorf("headings = %q, want the number and the title apart", got)
	}
}
