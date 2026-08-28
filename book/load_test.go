package book

import "testing"

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
