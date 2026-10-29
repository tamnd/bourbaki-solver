package publish

import (
	"strings"
	"testing"
)

// A heading the printer set in capitals reads as a sentence, with its numerals
// and its proper names kept.
func TestSentenceKeepsNumeralsAndNames(t *testing.T) {
	for _, c := range []struct{ lang, in, want string }{
		{"fr", "§ 1. LOIS DE COMPOSITION; ASSOCIATIVITÉ", "§ 1. Lois de composition; associativité"},
		{"en", "CHAPTER IV. LIE ALGEBRAS", "Chapter IV. Lie algebras"},
		{"vi", "CHƯƠNG I", "Chương I"},
		{"vi", "5. CÁC PHẦN TỬ HOÁN ĐỔI ĐƯỢC", "5. Các phần tử hoán đổi được"},
		{"vi", "CÁC PHẦN TỬ VI PHÂN", "Các phần tử vi phân"},
		{"en", "Already mixed Case", "Already mixed Case"},
	} {
		if got := sentenceIn(c.lang, c.in); got != c.want {
			t.Errorf("sentenceIn(%q, %q) = %q, want %q", c.lang, c.in, got, c.want)
		}
	}
}

// A Vietnamese page says its frame in Vietnamese, and that includes the lines
// that are sentences and not only the buttons: who wrote the text, what the
// exercises are, what cites what.
func TestAVietnamesePageIsNotFramedInEnglish(t *testing.T) {
	s := withDraft(t, testSite(t))
	got := s.mustPage(t, "vi/alg/VIII/s14/ex/1/index.html")
	if strings.Contains(got, ", permanent. Cite it as") {
		t.Error("the tag line of a Vietnamese exercise is in English")
	}
	for _, k := range []string{"p.mt", "p.unchecked", "x.nosol", "x.solution"} {
		if !strings.Contains(got, T("vi", k)) {
			t.Errorf("%s is not said in Vietnamese on an exercise page", k)
		}
		if strings.Contains(got, T("en", k)) && T("en", k) != T("vi", k) {
			t.Errorf("%s is said in English on a Vietnamese page", k)
		}
	}
	list := s.mustPage(t, "vi/alg/VIII/s14/ex/index.html")
	if strings.Contains(list, "Bourbaki sets these") || strings.Contains(list, "No solution has been written") {
		t.Errorf("the Vietnamese exercise list is in English:\n%s", list)
	}
}

// A translation that dropped the § and kept the number does not get the number
// twice, and a title that merely opens with a numeral keeps it.
func TestATitleSaysItsNumberOnce(t *testing.T) {
	for _, c := range []struct {
		in   string
		n    int
		want string
	}{
		{"1. Các tập mở, lân cận, các tập đóng", 1, "Các tập mở, lân cận, các tập đóng"},
		{"§ 6. 6. Bộ lọc", 6, "Bộ lọc"},
		{"PHỤ LỤC 1 ĐẠI SỐ KHÔNG CÓ PHẦN TỬ KHẢ NGHỊCH", 1, "ĐẠI SỐ KHÔNG CÓ PHẦN TỬ KHẢ NGHỊCH"},
		{"Annexe 2. Corps", 2, "Corps"},
		{"2. Groupes", 3, "2. Groupes"},
	} {
		if got := bareTitle(c.in, c.n); got != c.want {
			t.Errorf("bareTitle(%q, %d) = %q, want %q", c.in, c.n, got, c.want)
		}
	}
}
