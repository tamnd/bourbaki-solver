package book

import (
	"os"
	"path/filepath"
	"testing"
)

// The heading of a § in the corpus is usually written with a bare number and no
// section sign, and a translation reads its title off that heading because its
// front matter was never translated. Leaving the number on printed it twice, on
// the page and in the contents: "SS 1. 1. CAC TAP MO, LAN CAN, CAC TAP DONG" on
// 175 lines across the eight Vietnamese volumes that had been built.
func TestSectionTitleTakesTheNumberOffEitherWay(t *testing.T) {
	for _, c := range []struct {
		head string
		n    int
		want string
	}{
		{"§ 5. GROUPS OPERATING ON A SET", 5, "GROUPS OPERATING ON A SET"},
		{"§5.GROUPS OPERATING ON A SET", 5, "GROUPS OPERATING ON A SET"},
		{"1. CÁC TẬP MỞ, LÂN CẬN, CÁC TẬP ĐÓNG", 1, "CÁC TẬP MỞ, LÂN CẬN, CÁC TẬP ĐÓNG"},
		{"10. MA TRẬN", 10, "MA TRẬN"},
		// The number has to be the number of this §. A title that opens on some
		// other number keeps it.
		{"2. QUOTIENT LAWS", 6, "2. QUOTIENT LAWS"},
		// Nothing that is not a § is touched, and those carry Number 0.
		{"1. Rings of fractions", 0, "1. Rings of fractions"},
		// No number at all is the common case and passes through.
		{"HISTORICAL NOTE", 0, "HISTORICAL NOTE"},
	} {
		if got := sectionTitle(c.head, c.n); got != c.want {
			t.Errorf("sectionTitle(%q, %d) = %q, want %q", c.head, c.n, got, c.want)
		}
	}
}

func TestPickTakesTheTreeThatHasTheFile(t *testing.T) {
	root := t.TempDir()
	en := filepath.Join(root, "content", "en")
	mt := filepath.Join(root, "content", "en-mt")
	// Springer translated Algebre I to VIII and not IX, so content/en/alg stops
	// at VIII and content/en-mt/alg picks up at IX. The split runs through the
	// Book, which is why this is decided per chapter and not per volume.
	for _, d := range []string{
		filepath.Join(en, "alg", "VIII"),
		filepath.Join(mt, "alg", "IX"),
	} {
		if err := os.MkdirAll(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	dirs := []string{en, mt}
	if got := pick(dirs, "alg", "VIII"); got != en {
		t.Errorf("chapter VIII came out of %s, want content/en", got)
	}
	if got := pick(dirs, "alg", "IX"); got != mt {
		t.Errorf("chapter IX came out of %s, want content/en-mt", got)
	}
	// A chapter that is in neither falls back to the first tree, so the caller
	// gets the same "not there" it got before there were two trees.
	if got := pick(dirs, "alg", "XI"); got != en {
		t.Errorf("a chapter in neither tree came out of %s, want content/en", got)
	}
	// One tree and the answer is that tree, whatever is asked for.
	if got := pick([]string{en}, "top", "IV"); got != en {
		t.Errorf("with one tree the answer was %s, want content/en", got)
	}
}

// Bourbaki numbers the appendices of a chapter from one alongside the §§, and
// corpus.Section holds both in the same field, so a printed contents keyed on
// the number alone loses one to the other. Chapter VIII of Algebra prints
// twenty one §§ and then four appendices, and appendix 1 overwrote § 1: the
// built book listed the subsections of "Algebras without Unit Element" under a
// § called "Artinian Modules and Noetherian Modules". Seventeen §§ in the
// library were wrong this way, four in each printing of Algebra VIII, two in
// the French Topologie IX and one in Lie VII.
func TestAnAppendixAndASectionOfTheSameNumberAreNotTheSameEntry(t *testing.T) {
	if printedKey("VIII", 1, false) == printedKey("VIII", 1, true) {
		t.Fatalf("§ 1 and appendix 1 of chapter VIII share the key %q",
			printedKey("VIII", 1, false))
	}
	// The § keeps the plain key it always had, so nothing that was right
	// before this moves.
	if got := printedKey("VIII", 1, false); got != "VIII/1" {
		t.Errorf("§ 1 of chapter VIII is keyed %q, want VIII/1", got)
	}
}
