package glossary

import "testing"

// The Vietnamese here is what the corpus holds, lifted off the files rather
// than written for the test, because the question the function answers is
// whether real translated mathematics reads as translated and the only way to
// be wrong about that is to invent the mathematics.

// The sentence this was written for. It is the eleventh chunk of the appendix
// on the trace of an endomorphism, exactly as it came back, with the Vietnamese
// that surrounds it on the same line.
func TestARunOfEnglishInsideAVietnameseParagraphIsFound(t *testing.T) {
	const para = "det(1$_L+u'+v'+u'\\circ v') =$ det(1$_E+u+v+u\\circ v)$. " +
		"It therefore suffices to prove assertion b) when the A-module E is free. " +
		"There then exists a finitely generated free submodule F of E that contains " +
		"the image of $u$ and that of $v$. Đặt $w=u+v+u\\circ v$. Ảnh của $w$ được chứa trong F."
	run, words := Untranslated("vi", para)
	if words < 2 {
		t.Fatalf("the English sentences carry %d English words: %q", words, run)
	}
	if !WrittenIn("vi", para) {
		t.Fatal("the paragraph does not read as Vietnamese, so L07 would have caught this and L11 is not needed")
	}
}

// The mathematics is copied and not translated, so a display carries no
// Vietnamese at all and a rule that read length alone would report every one of
// them. What a display does not carry is the words that hold an English
// sentence together, which is the whole of why the floor is counted in those.
func TestADisplayIsNotAnUntranslatedSentence(t *testing.T) {
	for _, para := range []string{
		"det((1$_F+u_F)\\circ (1_F+v_F)) =$ det(1$_F+u_F)$ det(1$_F+v_F) =$ det(1$_E+u)$ det(1$_E+v)$.",
		"$$u=\\theta \\sum_{j\\in J}{}^tu(f_j^*)\\otimes f_j \\tag{2}$$",
		"#### Mệnh đề 2 {#alg-viii-a4-prop-2 .statement tag=00QT}",
		"Ta chứng minh a). Với mọi số nguyên $p\\geqslant 0$, A-môđun xạ ảnh $\\wedge^pF$ " +
			"có thể được đồng nhất với một môđun con của $\\wedge^pE$ (III, §7, No. 9, p. 520, Hệ quả).",
	} {
		if run, words := Untranslated("vi", para); words >= 2 {
			t.Errorf("%q was read as untranslated: %d words in %q", short(para), words, run)
		}
	}
}

// A language with no script test cannot be asked the question, and WrittenIn
// answers yes to every one of them so that nobody is failed for a test nobody
// has written.
func TestALanguageWithNoScriptTestIsNotAsked(t *testing.T) {
	if run, words := Untranslated("de", "It therefore suffices to prove assertion b) when the module is free"); words != 0 {
		t.Errorf("German was judged: %d words in %q", words, run)
	}
}

func short(s string) string {
	if len(s) <= 40 {
		return s
	}
	return s[:40] + "..."
}
