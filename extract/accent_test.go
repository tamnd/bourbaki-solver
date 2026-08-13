package extract

import "testing"

// The dual of a locally compact commutative group, out of Théories spectrales
// chapitres 1 et 2. The hat is drawn out of MSBM10 and the volume names no
// glyph, so poppler falls back on the code and hands back an opening bracket
// sitting where the hat was drawn. Page 240 shipped "G[/H" and page 217 shipped
// "\\circ", which is not mathematics at all but a line break inside a formula.
const wideHatPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="240" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="3" size="16" family="VxcftvXychsnNbbbxkJyhgbrLMRoman10" color="#131413"/>
<fontspec id="7" size="16" family="DmblwdPfyppxVvkxlkSwtntqMSBM10" color="#131413"/>
<text top="643" left="81" width="46" height="14" font="3">Soit </text>
<text top="643" left="194" width="13" height="14" font="3">G</text>
<text top="642" left="195" width="13" height="13" font="7">[</text>
</page>
</pdf2xml>
`

func TestAWideAccentDrawnInTheAMSFontIsAnAccent(t *testing.T) {
	lines := parse(t, wideHatPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `Soit $\widehat{G}$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// Page 420 of Théories spectrales chapitres 3 à 5, the remark that defines the
// reflected function. The check is drawn out of TeX-mathx10 and that subset
// names its narrow check "q", which the glyph list resolves to the letter q, so
// nothing is reported lost anywhere and the page shipped "la fonction $fq$ sur
// G", which reads as a product of two functions that is not there.
const checkAccentPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="420" position="absolute" top="0" left="0" height="999" width="659">
<fontspec id="3" size="16" family="KVZJDY+LMRoman10" color="#131413"/>
<fontspec id="4" size="16" family="OSYQLC+LMMathItalic10" color="#131413"/>
<fontspec id="9" size="16" family="LPADZX+TeX-mathx10" color="#131413"/>
<text top="574" left="81" width="80" height="14" font="3">la fonction </text>
<text top="574" left="168" width="7" height="15" font="4">f</text>
<text top="584" left="168" width="9" height="7" font="9">q</text>
<text top="574" left="181" width="30" height="14" font="3"> sur G</text>
</page>
</pdf2xml>
`

func TestACheckIsNotTheLetterQ(t *testing.T) {
	lines := parse(t, checkAccentPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `la fonction $\check{f}$ sur G`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// A 4 out of the first AMS symbol font is a preceding relation the same way a 6
// out of it is a slanted inequality, and the whole short row is written down.
func TestTheAMSRelationsAreNotDigits(t *testing.T) {
	for code, want := range map[rune]string{
		'4': `\preccurlyeq `, '6': `\leqslant `, '<': `\succcurlyeq `,
		'>': `\geqslant `, '{': `\complement `,
	} {
		if got := msam[code]; got != want {
			t.Errorf("msam[%q] = %q, want %q", code, got, want)
		}
	}
}

// The row is what the embedded subset says it is, and it is written down whole
// because the volume that draws only the hat is not the last volume.
func TestTheWideAccentsOfTheAMSFontAreTheOnesItCarries(t *testing.T) {
	for code, want := range map[rune]string{
		'[': `\widehat`, '\\': `\widehat`,
		']': `\widetilde`, '^': `\widetilde`,
	} {
		if got := msbm[code]; got != want {
			t.Errorf("msbm[%q] = %q, want %q", code, got, want)
		}
	}
	if len(msbm) != 4 {
		t.Errorf("the AMS symbol table has %d entries, want the 4 the font carries", len(msbm))
	}
}

// Page 209 of Lie 7 to 9, the spinor representation, which draws the tilde of
// its quadratic space out of the roman text face rather than out of a
// mathematics font. Nothing is lost and nothing is flagged: the accent arrives
// as itself and sits in the line where it was drawn, so the page shipped "from
// ˜V to" and "For $v\in$ ˜V, we have". One line carries both ways it arrives.
// The first tilde is welded to the end of the run "from " and the V is the run
// after it; the second is a run of its own drawn two units left of the V it
// stands over.
const spacingTildePage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="209" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="1" size="15" family="DGLKJH+CMR10" color="#000000"/>
<fontspec id="3" size="15" family="DGLMMN+CMMI10" color="#000000"/>
<fontspec id="5" size="15" family="DGLKKM+CMSY10" color="#000000"/>
<fontspec id="6" size="10" family="DGLOII+CMR7" color="#000000"/>
<text top="577" left="334" width="44" height="13" font="1">from ˜</text>
<text top="577" left="369" width="45" height="13" font="1">V to C</text>
<text top="575" left="414" width="9" height="9" font="6">+</text>
<text top="577" left="424" width="54" height="13" font="1">(Q). For </text>
<text top="577" left="482" width="7" height="13" font="3"><i>v</i></text>
<text top="574" left="494" width="10" height="19" font="5"><i>∈</i></text>
<text top="574" left="510" width="7" height="13" font="1">˜</text>
<text top="577" left="508" width="71" height="13" font="1">V, we have</text>
</page>
</pdf2xml>
`

func TestASpacingTildeGoesOverTheLetterItWasDrawnOver(t *testing.T) {
	lines := parse(t, spacingTildePage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `from $\widetilde{V}$ to $C^+(Q)$. For $v\in \widetilde{V}$, we have`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// The other half of the same volume, where a spacing accent stands over a
// letter of a word rather than over a letter of mathematics. All three lines
// break the run at the accent, which is what the first draft of this had
// assumed a word never does: page 259 hands back "Jordan-H¨" with "older" in
// the run after it, page 253 hands back "Mat. Obˇ" then "sˇ" then "c", and page
// 63 hands back "ZASSENHAUS, ¨" with "Uber" after it, a comma and a space in
// front of the accent leaving the U looking as alone as a variable does. Ten
// words of Lie 7 to 9 went out reading "Jordan-H$\ddot{o}$lder" before this was
// measured rather than reasoned about.
const wordAccentPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="259" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="0" size="13" family="DGLLON+CMR9" color="#000000"/>
<fontspec id="2" size="15" family="DGLKJH+CMR10" color="#000000"/>
<fontspec id="15" size="13" family="DGLLON+CMR9" color="#000000"/>
<fontspec id="16" size="13" family="DGNCHD+CMTI9" color="#000000"/>
<text top="695" left="195" width="141" height="13" font="2">) admits a Jordan-H¨</text>
<text top="695" left="328" width="149" height="13" font="2">older sequence.</text>
<text top="856" left="153" width="152" height="12" font="16"><i>Trudy Moskov. Mat. Obˇ</i></text>
<text top="856" left="299" width="12" height="12" font="16"><i>sˇ</i></text>
<text top="856" left="305" width="6" height="12" font="16"><i>c</i></text>
<text top="856" left="311" width="268" height="12" font="15">., Vol. 15</text>
<text top="926" left="94" width="250" height="12" font="0">For more details, cf. H. ZASSENHAUS, ¨</text>
<text top="926" left="336" width="244" height="12" font="0">Uber Liesche Ringe</text>
</page>
</pdf2xml>
`

func TestAnAccentInsideAWordIsALetterAndNotMathematics(t *testing.T) {
	lines := parse(t, wordAccentPage)
	want := []string{
		`) admits a Jordan-Hölder sequence.`,
		// The one the rule does not reach. The layer cuts this journal name
		// into "Obˇ", "sˇ" and "c", so the letter each caron covers is a token
		// of one character with no letter beside it in the token, and the test
		// for a word has nothing to read. It is what the corpus carries today
		// and it is a bibliography entry, so it is left where the other two
		// the corpus cannot spell are left.
		`Trudy Moskov. Mat. Ob$\check{s}\check{c}$., Vol. 15`,
		`For more details, cf. H. ZASSENHAUS, Über Liesche Ringe`,
	}
	if len(lines) != len(want) {
		t.Fatalf("got %d lines, want %d", len(lines), len(want))
	}
	for i, w := range want {
		if got := Render(lines[i]); got != w {
			t.Errorf("Render line %d:\n got %s\nwant %s", i, got, w)
		}
	}
}

// The citation of Chapter X of Algebra, which Lie 7 to 9 makes four times and
// which arrives in four runs: the "Alg" of the italic, a roman grave, the e out
// of the mathematics italic, and the "bre" of the italic again. TeX takes an
// accented letter of an italic word out of the mathematics italic, so the e is
// a run of one character between two runs of the word it belongs to, and the
// composed letter has to move into the word and take the ground it stood on
// with it or the page says "Alg$è$bre" or "Algè bre".
const italicWordPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="413" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="2" size="13" family="DGLLON+CMR9" color="#000000"/>
<fontspec id="3" size="13" family="DGMFAB+CMMI9" color="#000000"/>
<fontspec id="9" size="13" family="DGNCHD+CMTI9" color="#000000"/>
<text top="279" left="305" width="152" height="13" font="2">) by using Exerc. 23 of</text>
<text top="279" left="463" width="21" height="13" font="9"><i>Alg</i></text>
<text top="279" left="485" width="7" height="13" font="2">` + "`" + `</text>
<text top="279" left="484" width="7" height="13" font="3"><i>e</i></text>
<text top="279" left="491" width="19" height="13" font="9"><i>bre</i></text>
<text top="279" left="511" width="70" height="13" font="2">, Chap. X,</text>
</page>
</pdf2xml>
`

func TestAnAccentedLetterJoinsTheItalicWordItBelongsTo(t *testing.T) {
	lines := parse(t, italicWordPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	// The citation is set in italic and comes out plain, which is a separate
	// matter and not this one: what is being measured here is that the word is
	// one word, spelled, in the running text rather than a fragment with a
	// letter of mathematics wedged into it.
	const want = `) by using Exerc. 23 of Algèbre, Chap. X,`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}

// A large operator out of the extension font arrives under the code of an
// accent and is not one. Page 18 of Topologie algebrique sums a family of
// B-spaces over i in I and poppler hands the coprod back as a grave with the i
// of the subscript right after it, so the rule that folds a grave onto the
// letter that follows it wrote the sum as "X = $_{ì\in I}X_i$".
const operatorPage = `<?xml version="1.0" encoding="UTF-8"?>
<pdf2xml>
<page number="18" position="absolute" top="0" left="0" height="999" width="658">
<fontspec id="0" size="21" family="ABCDEF+NimbusRomNo9L" color="#231f20"/>
<fontspec id="2" size="21" family="ABCDEG+CMMI10" color="#231f20"/>
<fontspec id="3" size="11" family="ABCDEH+CMSY8" color="#231f20"/>
<fontspec id="5" size="11" family="ABCDEI+CMR8" color="#231f20"/>
<fontspec id="8" size="11" family="ABCDEJ+CMMI8" color="#231f20"/>
<fontspec id="11" size="21" family="ABCDEK+CMEX10" color="#231f20"/>
<text top="766" left="81" width="117" height="21" font="0">L’espace somme</text>
<text top="766" left="205" width="33" height="21" font="2">X =</text>
<text top="769" left="246" width="14" height="7" font="11">` + "`" + `</text>
<text top="777" left="260" width="4" height="11" font="8"><i>i</i></text>
<text top="777" left="265" width="8" height="11" font="3">∈</text>
<text top="777" left="273" width="5" height="11" font="5">I</text>
<text top="766" left="281" width="12" height="21" font="2">X</text>
<text top="777" left="293" width="4" height="11" font="8"><i>i</i></text>
</page>
</pdf2xml>
`

func TestALargeOperatorIsNotAnAccentWhateverCodeItArrivesUnder(t *testing.T) {
	lines := parse(t, operatorPage)
	if len(lines) != 1 {
		t.Fatalf("got %d lines, want 1", len(lines))
	}
	const want = `L’espace somme $X =\coprod_{i\in I}X_i$`
	if got := Render(lines[0]); got != want {
		t.Errorf("Render:\n got %s\nwant %s", got, want)
	}
}
