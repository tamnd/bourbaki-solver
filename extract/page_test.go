package extract

import (
	"strings"
	"testing"

	"github.com/tamnd/bourbaki-solver/pdfsrc"
)

// The foot of page 42, which opens § 2. A page that opens a § carries no
// running head, because the title of the § stands where the head would, so the
// volume prints the folio at the foot instead. Twenty four pages of the volume
// are set this way and this is the first of them.
const footXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="42" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="8" size="15" family="XJSZDJ+LMMathItalic10" color="#000000"/>
<fontspec id="9" size="15" family="MJDJIW+LMRoman10" color="#000000"/>
<fontspec id="13" size="15" family="GSFMFK+LMRoman10" color="#000000"/>
<text top="885" left="80" width="6" height="13" font="8"><i>r</i></text>
<text top="885" left="91" width="434" height="13" font="9">is a two-sided ideal of A by the following assertions a) through d):</text>
<text top="930" left="567" width="12" height="13" font="13">25</text>
</page>
</pdf2xml>
`

func TestReadFoot(t *testing.T) {
	lay, err := pdfsrc.ParseXML(strings.NewReader(footXML))
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}
	p := ReadPage(lay, lay.Pages[0])
	if p.Foot != 25 {
		t.Errorf("Foot = %d, want 25", p.Foot)
	}
	if strings.Contains(p.Body, "25") {
		t.Errorf("the folio is still in the body:\n%s", p.Body)
	}
	if want := "two-sided ideal"; !strings.Contains(p.Body, want) {
		t.Errorf("body lost the line it should keep:\n%s", p.Body)
	}
}

// Every reading below is real. The first is what the accent used to do to page
// 115 before it learned to cut a run rather than swallow it, and catching that
// is what the test is for. The rest are lines the first version of the rule
// called English, and they are why it now asks for two words and a space rather
// than for a run of letters: a product of variables is not a word, however many
// letters it has.
//
// "D L" is the last line and it is not caught. It is a real loss, the backslash
// of a set difference that poppler dropped, and there is no reading of two
// letters and a space that separates it from an ordinary product. It is left to
// the audit rather than paid for with a flag on every page of algebra.
func TestWordInMath(t *testing.T) {
	for _, c := range []struct {
		body string
		want bool
	}{
		{`bimodules $\widetilde{P the dual Hom}_B(P,B)$`, true},
		{`we have $x=\sum_{i=1}^na_ix_i$`, false},
		{`the matrix $\mathbf{M}_n(D_i)$`, false},
		{`$xaxa$ and $nnnq$ and $xbca$`, false},
		{`$\varphi \sum a_nX^n=\sum(a_n)_M(X_M)^n$`, false},
		{`End$_A(P)$ is bijective`, false},
		{`$P\setminus S$ is the difference`, false},
		{`an element of $D L$`, false},
	} {
		if got := wordInMath(c.body); got != c.want {
			t.Errorf("wordInMath(%q) = %v, want %v", c.body, got, c.want)
		}
	}
}

// The head of § 3 of Chapter IX, which is too long for the measure and is set
// on two lines. TeX broke it at the hyphen the printing sets, so the hyphen
// stands at the end of the first line and the name carries on with no space
// after it. The corpus shipped "SEMI- SIMPLE" here, and read the same way in
// the table of contents, which is built off the heading.
const brokenHeadXML = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="303" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="1" size="18" family="LKSJDA+CMSY10" color="#000000"/>
<fontspec id="4" size="18" family="LKSJDB+CMBX10" color="#000000"/>
<fontspec id="6" size="15" family="LKSJDC+CMBX10" color="#000000"/>
<text top="75" left="83" width="8" height="23" font="1">§</text>
<text top="79" left="91" width="434" height="16" font="4"><b>3. COMPACT FORMS OF COMPLEX SEMI-</b></text>
<text top="103" left="83" width="241" height="16" font="4"><b>SIMPLE LIE ALGEBRAS</b></text>
<text top="147" left="83" width="137" height="13" font="6"><b>1. REAL FORMS</b></text>
</page>
</pdf2xml>
`

func TestATitleBrokenAtAHyphenCarriesOnWithNoSpace(t *testing.T) {
	lay, err := pdfsrc.ParseXML(strings.NewReader(brokenHeadXML))
	if err != nil {
		t.Fatalf("ParseXML: %v", err)
	}
	p := ReadPage(lay, lay.Pages[0])
	const want = "## § 3. COMPACT FORMS OF COMPLEX SEMI-SIMPLE LIE ALGEBRAS"
	if !strings.Contains(p.Body, want) {
		t.Errorf("body:\n%s\nwant a line reading\n%s", p.Body, want)
	}
}
