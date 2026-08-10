package corpus

import "testing"

func TestKindFromHeading(t *testing.T) {
	cases := map[string]Kind{
		"Proposition": KindProposition,
		"PROPOSITION": KindProposition,
		"  Lemma  ":   KindLemma,
		"Corollary":   KindCorollary,
		"Remarks":     KindRemark,
		"Definitions": KindDefinition,
		"Scholium":    KindScholium,
	}
	for in, want := range cases {
		got, ok := KindFromHeading(in)
		if !ok || got != want {
			t.Errorf("KindFromHeading(%q) = %q, %v; want %q, true", in, got, ok, want)
		}
	}
	if _, ok := KindFromHeading("Chapter"); ok {
		t.Error(`KindFromHeading("Chapter") should not resolve`)
	}
}

func TestRomanOrder(t *testing.T) {
	cases := map[string]int{"I": 1, "II": 2, "III": 3, "IV": 4, "V": 5, "VI": 6, "VII": 7, "VIII": 8, "IX": 9, "X": 10}
	for in, want := range cases {
		got, err := RomanOrder(in)
		if err != nil || got != want {
			t.Errorf("RomanOrder(%q) = %d, %v; want %d", in, got, err, want)
		}
	}
	if _, err := RomanOrder("VIII2"); err == nil {
		t.Error("RomanOrder should reject a non-numeral")
	}
}

// The eight chapters in scope must sort in reading order, not lexically. This
// is the whole reason RomanOrder exists: "VIII" sorts before "II" as a string.
func TestRomanOrderSortsChaptersInScope(t *testing.T) {
	chapters := []string{"I", "II", "III", "IV", "V", "VI", "VII", "VIII"}
	prev := 0
	for _, ch := range chapters {
		n, err := RomanOrder(ch)
		if err != nil {
			t.Fatalf("RomanOrder(%q): %v", ch, err)
		}
		if n != prev+1 {
			t.Errorf("chapter %q ordered %d, want %d", ch, n, prev+1)
		}
		prev = n
	}
}

func TestLabelRoundTrip(t *testing.T) {
	cases := []struct {
		ref   Ref
		label string
	}{
		{Ref{Book: "alg", Chapter: "VIII", Section: 1, Kind: KindProposition, Number: 6}, "alg-viii-s1-prop-6"},
		{Ref{Book: "alg", Chapter: "VIII", Section: 1, Kind: KindDefinition, Number: 3}, "alg-viii-s1-def-3"},
		{Ref{Book: "alg", Chapter: "I", Section: 2, Kind: KindExercise, Number: 7}, "alg-i-s2-ex-7"},
		{Ref{Book: "alg", Chapter: "V", Section: 17, Kind: KindTheorem, Number: 1}, "alg-v-s17-thm-1"},
		{Ref{Book: "alg", Chapter: "VIII", Section: 1, Kind: KindCorollary, Subsec: 3, Occurrence: 1}, "alg-viii-s1-n3-cor-1"},
		{Ref{Book: "alg", Chapter: "IV", Section: 6, Kind: KindRemark, Subsec: 12, Occurrence: 2}, "alg-iv-s6-n12-rem-2"},
	}
	for _, c := range cases {
		if got := c.ref.Label(); got != c.label {
			t.Errorf("Label() = %q, want %q", got, c.label)
		}
		back, err := ParseLabel(c.label)
		if err != nil {
			t.Fatalf("ParseLabel(%q): %v", c.label, err)
		}
		if back != c.ref {
			t.Errorf("ParseLabel(%q) = %+v, want %+v", c.label, back, c.ref)
		}
	}
}

func TestParseLabelRejectsGarbage(t *testing.T) {
	for _, bad := range []string{"", "alg-viii-s1-prop", "alg-VIII-s1-prop-6", "alg-viii-1-prop-6", "alg-viii-s1-n3-cor"} {
		if _, err := ParseLabel(bad); err == nil {
			t.Errorf("ParseLabel(%q) should fail", bad)
		}
	}
}

// Every one of these lines is a real running head or cross-reference pulled out
// of the three PDFs with pdftotext -layout. The three volumes print the page
// label three different ways and the parser has to take all of them.
func TestParsePageLabelOnRealRunningHeads(t *testing.T) {
	cases := []struct {
		line string
		want PageLabel
	}{
		{"No 4   POLYNOMIALS WITH COEFFICIENTS IN A NOETHERIAN RING       A VIII.13", PageLabel{"A", "VIII", 13}},
		{"                              EXERCISES                                 A VIII.43", PageLabel{"A", "VIII", 43}},
		{"No. 2                                   POLYNOMIALS                                       A.IV.3", PageLabel{"A", "IV", 3}},
		{"A. IV. 2                     POLYNOMIALS AND RATIONAL FRACTIONS                            §l", PageLabel{"A", "IV", 2}},
		{"A.V . 36                        COMMUTATIVE FIELDS                                     §7", PageLabel{"A", "V", 36}},
		{" A.V.186                                  ALGEBRA", PageLabel{"A", "V", 186}},
		{"                                     HISTORICAL NOTE                                A.VII.81", PageLabel{"A", "VII", 81}},
	}
	for _, c := range cases {
		got, ok := ParsePageLabel(c.line)
		if !ok {
			t.Errorf("ParsePageLabel(%q) found nothing", c.line)
			continue
		}
		if got != c.want {
			t.Errorf("ParsePageLabel(%q) = %v, want %v", c.line, got, c.want)
		}
		if !got.Valid() {
			t.Errorf("%v should be valid", got)
		}
	}
}

// A prose cross-reference separates the Book from the chapter with a comma.
// Those are a different grammar and must not be mistaken for a page label.
func TestParsePageLabelIgnoresProseReferences(t *testing.T) {
	for _, line := range []string{
		"Set Theory, III, § 4, No. 2, p. 167, Corollary 3",
		"TA, II, § 2, no 4, p. 158, corollaire de la proposition 1",
		"                                    TO THE READER                                 VII",
		"I                              ALGEBRAIC STRUCTURES",
	} {
		if got, ok := ParsePageLabel(line); ok {
			t.Errorf("ParsePageLabel(%q) = %v, want no match", line, got)
		}
	}
}

func TestPageLabelString(t *testing.T) {
	if got := (PageLabel{"A", "VIII", 13}).String(); got != "A VIII.13" {
		t.Errorf("String() = %q", got)
	}
}

func TestPageLabelValid(t *testing.T) {
	for _, bad := range []PageLabel{
		{"", "VIII", 13},
		{"A", "VIII", 0},
		{"A", "", 13},
		{"A", "VII2", 13},
	} {
		if bad.Valid() {
			t.Errorf("%+v should be invalid", bad)
		}
	}
}

// Chapters I to III of the 1998 printing carry no page label in the running
// head, only the chapter numeral on the verso and this locator on the recto, so
// the page map for that volume is anchored on these.
func TestParseSectionLocatorOnRealRunningHeads(t *testing.T) {
	cases := []struct {
		line string
		want SectionLocator
	}{
		{"                                        /J-GROUPS                                     §6.5", SectionLocator{6, 5}},
		{"pdf p.45:  APPLICATIONS: I. RATIONAL INTEGERS          § 2.5", SectionLocator{2, 5}},
		{"A.V . 36                        COMMUTATIVE FIELDS                                     §7", SectionLocator{7, 0}},
		{"§ 2.8   NOTATION", SectionLocator{2, 8}},
	}
	for _, c := range cases {
		got, ok := ParseSectionLocator(c.line)
		if !ok {
			t.Errorf("ParseSectionLocator(%q) found nothing", c.line)
			continue
		}
		if got != c.want {
			t.Errorf("ParseSectionLocator(%q) = %v, want %v", c.line, got, c.want)
		}
	}
	if _, ok := ParseSectionLocator("ALGEBRAIC STRUCTURES"); ok {
		t.Error("a head with no locator should not parse")
	}
}

func TestSectionLocatorString(t *testing.T) {
	if got := (SectionLocator{6, 5}).String(); got != "§6.5" {
		t.Errorf("String() = %q", got)
	}
	if got := (SectionLocator{7, 0}).String(); got != "§7" {
		t.Errorf("String() = %q", got)
	}
}
